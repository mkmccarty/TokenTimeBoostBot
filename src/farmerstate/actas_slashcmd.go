package farmerstate

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

// GetSlashActAsCommand returns the slash command definition for /act-as.
func GetSlashActAsCommand(cmd string) *dc.Command {
	command := dc.Command{
		Name:        cmd,
		Description: "Manage and switch between linked alternate accounts.",
		Contexts: []dc.InteractionContext{
			dc.ContextGuild,
			dc.ContextBotDM,
			dc.ContextPrivateChannel,
		},
		IntegrationTypes: []dc.IntegrationType{
			dc.IntegrationGuildInstall,
			dc.IntegrationUserInstall,
		},
		Options: []dc.Option{
			dc.SubCommand{
				Name:        "register",
				Description: "Link an alternate Discord account for /act-as permissions.",
				Options: []dc.Option{
					dc.UserOption{
						Name:        "user",
						Description: "Alternate Discord account to link",
						Required:    true,
					},
				},
			},
			dc.SubCommand{
				Name:        "switch",
				Description: "Switch to acting as a linked alternate account.",
				Options: []dc.Option{
					dc.StringOption{
						Name:         "account",
						Description:  "The alternate account to act as",
						Required:     true,
						Autocomplete: true,
					},
					dc.StringOption{
						Name:        "duration",
						Description: "How long this switch will remain active before expiring",
						Required:    true,
						Choices: []dc.Choice[string]{
							{Name: "15 minutes", Value: "15m"},
							{Name: "30 minutes", Value: "30m"},
							{Name: "1 hour", Value: "1h"},
							{Name: "2 hours", Value: "2h"},
							{Name: "4 hours", Value: "4h"},
							{Name: "8 hours", Value: "8h"},
							{Name: "12 hours", Value: "12h"},
							{Name: "24 hours", Value: "24h"},
							{Name: "Forever (until reset)", Value: "forever"},
							{Name: "Switch back to Main account", Value: "0m"},
						},
					},
				},
			},
			dc.SubCommand{
				Name:        "revoke",
				Description: "Revoke /act-as linking with an alternate account.",
				Options: []dc.Option{
					dc.StringOption{
						Name:         "account",
						Description:  "The linked account to revoke (or leave empty to revoke)",
						Required:     false,
						Autocomplete: true,
					},
				},
			},
			dc.SubCommand{
				Name:        "status",
				Description: "Report the current /act-as state for this channel.",
			},
		},
	}
	return &command
}

// HandleActAsCommand routes subcommands for /act-as.
// Note: /act-as always resolves e.UserID() as the real user and never applies act-as mapping.
func HandleActAsCommand(client dc.Client, e *dc.CommandEvent) {
	subcommand := ""
	if path := e.SubcommandPath(); len(path) > 0 {
		subcommand = path[0]
	}

	switch subcommand {
	case "register":
		handleActAsRegister(client, e)
	case "switch":
		handleActAsSwitch(e)
	case "revoke":
		handleActAsRevoke(e)
	case "status":
		handleActAsStatus(e)
	default:
		_ = e.Respond(dc.Message{
			Content:   "Unknown subcommand for /act-as.",
			Ephemeral: true,
		})
	}
}

func handleActAsRegister(client dc.Client, e *dc.CommandEvent) {
	u, ok := e.OptUser("user")
	if !ok || u == nil {
		_ = e.Respond(dc.Message{
			Content:   "Please specify a valid Discord user to link.",
			Ephemeral: true,
		})
		return
	}

	mainUserID := e.UserID()
	targetUserID := u.ID

	if targetUserID == mainUserID {
		_ = e.Respond(dc.Message{
			Content:   "You cannot register your own account as an alternate.",
			Ephemeral: true,
		})
		return
	}

	if u.Bot {
		_ = e.Respond(dc.Message{
			Content:   "You cannot register a bot account as an alternate.",
			Ephemeral: true,
		})
		return
	}

	if client == nil {
		_ = e.Respond(dc.Message{
			Content:   "Bot client is not available. Please try again later.",
			Ephemeral: true,
		})
		return
	}

	// Generate 8-digit verification code
	codeInt, err := rand.Int(rand.Reader, big.NewInt(90000000))
	if err != nil {
		_ = e.Respond(dc.Message{
			Content:   "Failed to generate verification code. Please try again.",
			Ephemeral: true,
		})
		return
	}
	code := fmt.Sprintf("%08d", codeInt.Int64()+10000000)

	dmChan, err := client.CreateUserChannel(targetUserID)
	if err != nil || dmChan == nil {
		_ = e.Respond(dc.Message{
			Content:   fmt.Sprintf("Unable to open a direct message with <@%s>. Please make sure they have direct messages enabled from server members and try again.", targetUserID),
			Ephemeral: true,
		})
		return
	}

	dmContent := fmt.Sprintf(
		"⚠️ **BoostBot Account Link Request** ⚠️\n\n"+
			"<@%s> has requested to link your Discord account for `/act-as` permissions in Boost Bot.\n\n"+
			"> **WARNING:** This should have been an expected message from BoostBot. If you did NOT request or expect this, please **ignore this message**.\n"+
			"> Never share verification codes with anyone you do not trust.\n\n"+
			"Your 8-digit verification code is: **`%s`**\n*(This code expires in 10 minutes)*\n\n"+
			"💡 **Revoking Access:** If you approve this link, you can revoke access at any time by running `/act-as revoke` from this account.",
		mainUserID,
		code,
	)

	_, err = client.SendMessage(dmChan.ID, dc.Message{Content: dmContent})
	if err != nil {
		_ = e.Respond(dc.Message{
			Content:   fmt.Sprintf("Failed to send verification DM to <@%s>. Please ensure they allow direct messages from server members and try again.", targetUserID),
			Ephemeral: true,
		})
		return
	}

	StorePendingActAsRegistration(mainUserID, targetUserID, code, 10*time.Minute)

	err = e.ShowModal(dc.Modal{
		CustomID: "m_actas_reg#" + targetUserID,
		Title:    "Link Alternate Account",
		Inputs: []dc.TextInput{
			{
				CustomID:    "code",
				Label:       "8-Digit Verification Code",
				Style:       dc.TextInputStyleShort,
				Placeholder: "12345678",
				MinLength:   8,
				MaxLength:   8,
				Required:    true,
			},
		},
	})
	if err != nil {
		log.Println("ShowModal error:", err)
	}
}

// HandleActAsRegisterModalSubmit processes the submitted verification code modal.
func HandleActAsRegisterModalSubmit(client dc.Client, e *dc.ModalEvent) {
	mainUserID := e.UserID()
	code := strings.TrimSpace(e.TextValue("code"))
	targetUserID := ""
	if parts := strings.Split(e.CustomID(), "#"); len(parts) > 1 {
		targetUserID = parts[1]
	}

	pending, ok := GetPendingActAsRegistration(mainUserID)
	if !ok {
		_ = e.Respond(dc.Message{
			Content:   "Verification session expired or not found. Please run `/act-as register` again.",
			Ephemeral: true,
		})
		return
	}

	if targetUserID != "" && pending.AltUserID != targetUserID {
		_ = e.Respond(dc.Message{
			Content:   "Registration session mismatch. Please run `/act-as register` again.",
			Ephemeral: true,
		})
		return
	}

	if code != pending.Code {
		_ = e.Respond(dc.Message{
			Content:   "Incorrect verification code. Please check the code sent to your alternate account and try again.",
			Ephemeral: true,
		})
		return
	}

	ClearPendingActAsRegistration(mainUserID)

	err := AddActAsLink(mainUserID, pending.AltUserID)
	if err != nil {
		_ = e.Respond(dc.Message{
			Content:   "Failed to save account link. Please try again.",
			Ephemeral: true,
		})
		return
	}

	if client != nil {
		dmChan, dmErr := client.CreateUserChannel(pending.AltUserID)
		if dmErr == nil && dmChan != nil {
			linkedDM := fmt.Sprintf(
				"✅ **BoostBot Account Link Successful**\n\n"+
					"Your Discord account has been linked as an alternate account by <@%s> for `/act-as` permissions in Boost Bot.\n\n"+
					"They can now act on your behalf when running contracts, boost list interactions, and commands.\n\n"+
					"💡 **Revoking Access:** You can revoke access at any time by running `/act-as revoke` from this account.",
				mainUserID,
			)
			_, _ = client.SendMessage(dmChan.ID, dc.Message{Content: linkedDM})
		}
	}

	_ = e.Respond(dc.Message{
		Content:   fmt.Sprintf("✅ Successfully linked <@%s> as an alternate account! You can now use `/act-as switch` to act as this account.", pending.AltUserID),
		Ephemeral: true,
	})
}

func handleActAsSwitch(e *dc.CommandEvent) {
	mainUserID := e.UserID()
	channelID := e.ChannelID()
	account, _ := e.OptString("account")
	account = strings.TrimSpace(account)
	durationStr, _ := e.OptString("duration")
	durationStr = strings.TrimSpace(durationStr)

	if account == "main" || account == "reset" || durationStr == "0m" || durationStr == "0" {
		_ = ClearActAsSwitch(mainUserID, channelID)
		_ = e.Respond(dc.Message{
			Content:   "Switched back to your main account. You are now acting as yourself in this channel.",
			Ephemeral: true,
		})
		return
	}

	// Validate account is linked to mainUserID
	if !IsActAsLinked(mainUserID, account) {
		_ = e.Respond(dc.Message{
			Content:   "You can only switch to accounts linked to your main account via `/act-as register`.",
			Ephemeral: true,
		})
		return
	}

	var expiresAt time.Time
	if durationStr == "forever" {
		expiresAt = ActAsForeverExpiry
	} else {
		dur, err := time.ParseDuration(durationStr)
		if err != nil || dur <= 0 || dur > 7*24*time.Hour {
			_ = e.Respond(dc.Message{
				Content:   "Invalid duration specified. Please choose a duration up to 7 days.",
				Ephemeral: true,
			})
			return
		}
		expiresAt = time.Now().Add(dur)
	}

	err := SetActAsSwitch(mainUserID, channelID, account, expiresAt)
	if err != nil {
		_ = e.Respond(dc.Message{
			Content:   "Failed to switch account. Please try again.",
			Ephemeral: true,
		})
		return
	}

	if IsActAsForever(expiresAt) {
		_ = e.Respond(dc.Message{
			Content:   fmt.Sprintf("Switched to acting as <@%s> in this channel forever (until reset).", account),
			Ephemeral: true,
		})
		return
	}

	_ = e.Respond(dc.Message{
		Content:   fmt.Sprintf("Switched to acting as <@%s> in this channel. This switch will expire <t:%d:R> (<t:%d:t>).", account, expiresAt.Unix(), expiresAt.Unix()),
		Ephemeral: true,
	})
}

func handleActAsRevoke(e *dc.CommandEvent) {
	callerID := e.UserID()
	account, _ := e.OptString("account")
	account = strings.TrimSpace(account)

	linkedUsers := GetAllActAsLinkedUsers(callerID)
	if len(linkedUsers) == 0 {
		_ = e.Respond(dc.Message{
			Content:   "You have no linked accounts.",
			Ephemeral: true,
		})
		return
	}

	if account == "all" {
		_ = DeleteAllActAsLinks(callerID)
		_ = ClearAllActAsSwitches(callerID)
		_ = e.Respond(dc.Message{
			Content:   "Revoked all linked accounts.",
			Ephemeral: true,
		})
		return
	}

	if account == "" {
		if len(linkedUsers) == 1 {
			targetID := linkedUsers[0]
			_ = RemoveActAsLink(callerID, targetID)
			_ = e.Respond(dc.Message{
				Content:   fmt.Sprintf("Revoked `/act-as` link with <@%s>.", targetID),
				Ephemeral: true,
			})
			return
		}
		_ = e.Respond(dc.Message{
			Content:   "You have multiple linked accounts. Please specify an account to revoke or select 'All Linked Accounts'.",
			Ephemeral: true,
		})
		return
	}

	if !IsActAsLinkedEither(callerID, account) {
		_ = e.Respond(dc.Message{
			Content:   "That account is not linked.",
			Ephemeral: true,
		})
		return
	}

	_ = RemoveActAsLink(callerID, account)
	_ = e.Respond(dc.Message{
		Content:   fmt.Sprintf("Revoked `/act-as` link with <@%s>.", account),
		Ephemeral: true,
	})
}

func handleActAsStatus(e *dc.CommandEvent) {
	mainUserID := e.UserID()
	channelID := e.ChannelID()

	altID, expiresAt, active := GetActAsSwitch(mainUserID, channelID)
	if !active || altID == "" {
		_ = e.Respond(dc.Message{
			Content:   fmt.Sprintf("You are currently acting as **yourself** (<@%s>) in this channel.", mainUserID),
			Ephemeral: true,
		})
		return
	}

	if IsActAsForever(expiresAt) {
		_ = e.Respond(dc.Message{
			Content:   fmt.Sprintf("You are currently acting as <@%s> in this channel forever (until reset).", altID),
			Ephemeral: true,
		})
		return
	}

	_ = e.Respond(dc.Message{
		Content:   fmt.Sprintf("You are currently acting as <@%s> in this channel. This switch will expire <t:%d:R> (<t:%d:t>).", altID, expiresAt.Unix(), expiresAt.Unix()),
		Ephemeral: true,
	})
}

// HandleActAsAutocomplete provides autocomplete suggestions for /act-as subcommands.
func HandleActAsAutocomplete(e *dc.AutocompleteEvent) {
	subcommand, _ := e.Subcommand()
	optName, optVal := e.FocusedOption()
	optVal = strings.ToLower(strings.TrimSpace(optVal))

	if optName != "account" && !strings.HasSuffix(optName, "account") {
		_ = e.RespondChoices(nil)
		return
	}

	mainUserID := e.UserID()
	links := GetActAsLinks(mainUserID)

	switch subcommand {
	case "switch":
		if len(links) == 0 {
			_ = e.RespondChoices([]dc.Choice[string]{
				{Name: "No linked accounts (use /act-as register)", Value: ""},
			})
			return
		}

		var choices []dc.Choice[string]
		if _, _, active := GetActAsSwitch(mainUserID, e.ChannelID()); active {
			choices = append(choices, dc.Choice[string]{
				Name:  "Main Account (Switch back to yourself)",
				Value: "main",
			})
		}

		for _, altID := range links {
			ign := strings.TrimSpace(GetMiscSettingString(altID, "ei_ign"))
			name := altID
			if ign != "" {
				name = fmt.Sprintf("%s (%s)", ei.NormalizePlayerNameForDisplay(ign), altID)
			}
			if optVal != "" && !strings.Contains(strings.ToLower(name), optVal) && !strings.Contains(strings.ToLower(altID), optVal) {
				continue
			}
			choices = append(choices, dc.Choice[string]{
				Name:  name,
				Value: altID,
			})
			if len(choices) >= 25 {
				break
			}
		}
		_ = e.RespondChoices(choices)

	case "revoke":
		callerID := e.UserID()
		allLinks := GetAllActAsLinkedUsers(callerID)
		if len(allLinks) == 0 {
			_ = e.RespondChoices([]dc.Choice[string]{
				{Name: "No linked accounts to revoke", Value: ""},
			})
			return
		}

		var choices []dc.Choice[string]
		if len(allLinks) > 1 {
			choices = append(choices, dc.Choice[string]{
				Name:  "All Linked Accounts",
				Value: "all",
			})
		}

		for _, linkedID := range allLinks {
			ign := strings.TrimSpace(GetMiscSettingString(linkedID, "ei_ign"))
			name := linkedID
			if ign != "" {
				name = fmt.Sprintf("%s (%s)", ei.NormalizePlayerNameForDisplay(ign), linkedID)
			}
			if optVal != "" && !strings.Contains(strings.ToLower(name), optVal) && !strings.Contains(strings.ToLower(linkedID), optVal) {
				continue
			}
			choices = append(choices, dc.Choice[string]{
				Name:  name,
				Value: linkedID,
			})
			if len(choices) >= 25 {
				break
			}
		}
		_ = e.RespondChoices(choices)

	default:
		_ = e.RespondChoices(nil)
	}
}
