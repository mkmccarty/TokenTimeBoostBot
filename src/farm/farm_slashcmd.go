package farm

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

func init() {
	boost.RegisterEggIDModalAction("farm", func(e *dc.ModalEvent, options dc.OptionValues, encryptedID string, okayToSave bool) {
		HandleFarmModal(e, options, encryptedID, okayToSave)
	})
}

func anywhereCommand(name, description string) dc.Command {
	return dc.Command{
		Name:        name,
		Description: description,
		Contexts: []dc.InteractionContext{
			dc.ContextGuild,
			dc.ContextBotDM,
			dc.ContextPrivateChannel,
		},
		IntegrationTypes: []dc.IntegrationType{
			dc.IntegrationGuildInstall,
			dc.IntegrationUserInstall,
		},
	}
}

// GetSlashFarmCommand returns the command definition for the /farm command with delivery, earnings, and drone-farming subcommands.
func GetSlashFarmCommand(cmd string) *dc.Command {
	command := anywhereCommand(cmd, "Evaluate active farm and provide optimal artifact and stone setups.")
	command.Options = []dc.Option{
		dc.SubCommand{
			Name:        "delivery",
			Description: "Optimal artifact and stone setup for egg delivery rate.",
			Options: []dc.Option{
				dc.StringOption{
					Name:         "alt",
					Description:  "Select an alternate account to evaluate",
					Required:     false,
					Autocomplete: true,
				},
				dc.BoolOption{
					Name:        "help",
					Description: "Explain what this command reports",
				},
			},
		},
		dc.SubCommand{
			Name:        "earnings",
			Description: "Optimal artifact and stone setup for farm earnings.",
			Options: []dc.Option{
				dc.StringOption{
					Name:         "alt",
					Description:  "Select an alternate account to evaluate",
					Required:     false,
					Autocomplete: true,
				},
				dc.BoolOption{
					Name:        "help",
					Description: "Explain what this command reports",
				},
			},
		},
		dc.SubCommand{
			Name:        "drone-farming",
			Description: "Optimal artifact and stone setup for drone Golden Egg farming.",
			Options: []dc.Option{
				dc.StringOption{
					Name:         "alt",
					Description:  "Select an alternate account to evaluate",
					Required:     false,
					Autocomplete: true,
				},
				dc.BoolOption{
					Name:        "help",
					Description: "Explain what this command reports",
				},
			},
		},
	}
	return &command
}

// HandleFarmAutocomplete handles autocomplete events for the /farm command.
func HandleFarmAutocomplete(e *dc.AutocompleteEvent) {
	name, value := e.FocusedOption()
	if name != "alt" {
		return
	}

	alts := farmerstate.GetUserAltsWithSavedEID(farmerstate.GetEffectiveUserID(e.UserID(), e.ChannelID()))
	value = strings.ToLower(strings.TrimSpace(value))

	var choices []dc.Choice[string]
	for _, alt := range alts {
		if value != "" {
			matchID := strings.Contains(strings.ToLower(alt.ID), value)
			matchIGN := strings.Contains(strings.ToLower(alt.IGN), value)
			matchDisplay := strings.Contains(strings.ToLower(alt.DisplayName), value)
			if !matchID && !matchIGN && !matchDisplay {
				continue
			}
		}
		displayName := alt.DisplayName
		if len(displayName) > 100 {
			displayName = displayName[:100]
		}
		choices = append(choices, dc.Choice[string]{
			Name:  displayName,
			Value: alt.ID,
		})
		if len(choices) == 25 {
			break
		}
	}

	_ = e.RespondChoices(choices)
}

// decryptEggIncID decrypts an encrypted Egg Inc ID string using config.Key.
func decryptEggIncID(eiID string) string {
	if eiID == "" {
		return ""
	}
	encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
	if err != nil {
		return ""
	}
	decodedData, err := base64.StdEncoding.DecodeString(eiID)
	if err != nil {
		return ""
	}
	decryptedData, err := config.DecryptCombined(encryptionKey, decodedData)
	if err != nil {
		return ""
	}
	id := string(decryptedData)
	if len(id) == 18 && id[:2] == "EI" {
		return id
	}
	return ""
}

// getEggIncID retrieves and decrypts the user's stored Egg Inc ID.
func getEggIncID(userID string) string {
	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	return decryptEggIncID(eiID)
}

// HandleFarm handles the /farm command.
func HandleFarm(e *dc.CommandEvent) {
	effectiveUserID, expiresAt, isActAs := farmerstate.GetEffectiveUserIDAndExpiry(e.UserID(), e.ChannelID())
	userID := effectiveUserID

	if opt, ok := e.OptBool("help"); ok && opt {
		_ = e.Respond(dc.Message{Content: farmHelpText(), Ephemeral: true})
		return
	}

	altParam, _ := e.OptString("alt")
	targetID, altEggIncID, notice := farmerstate.ResolveAltSelection(userID, altParam, "Farm")

	var actAsNotice string
	if isActAs && altParam == "" {
		if farmerstate.IsActAsForever(expiresAt) {
			actAsNotice = fmt.Sprintf("Acting as <@%s> via /act-as", effectiveUserID)
		} else {
			actAsNotice = fmt.Sprintf("Acting as <@%s> via /act-as (expires <t:%d:R>)", effectiveUserID, expiresAt.Unix())
		}
	}

	eggIncID := altEggIncID
	if eggIncID == "" {
		eggIncID = getEggIncID(targetID)
	}

	if eggIncID == "" {
		boost.RequestEggIncIDModal(e, "farm", e.Options())
		return
	}

	ExecuteFarmTarget(e, e.Options(), eggIncID, true, targetID, notice, actAsNotice)
}

// HandleFarmModal handles the modal submission when a user provides their Egg Inc ID.
func HandleFarmModal(e *dc.ModalEvent, options dc.OptionValues, encryptedID string, okayToSave bool) {
	effectiveUserID, expiresAt, isActAs := farmerstate.GetEffectiveUserIDAndExpiry(e.UserID(), e.ChannelID())
	userID := effectiveUserID

	altParam, _ := options.String("alt")
	targetID, altEggIncID, notice := farmerstate.ResolveAltSelection(userID, altParam, "Farm")

	var actAsNotice string
	if isActAs && altParam == "" {
		if farmerstate.IsActAsForever(expiresAt) {
			actAsNotice = fmt.Sprintf("Acting as <@%s> via /act-as", effectiveUserID)
		} else {
			actAsNotice = fmt.Sprintf("Acting as <@%s> via /act-as (expires <t:%d:R>)", effectiveUserID, expiresAt.Unix())
		}
	}

	eggIncID := altEggIncID
	if eggIncID == "" {
		eggIncID = decryptEggIncID(encryptedID)
	}
	if eggIncID == "" {
		_ = e.Respond(dc.Message{
			Content:   "Invalid Egg Inc ID received.",
			Ephemeral: true,
		})
		return
	}

	ExecuteFarmTarget(e, options, eggIncID, okayToSave, targetID, notice, actAsNotice)
}

func farmHelpText() string {
	var builder strings.Builder
	builder.WriteString("# /farm Help\n\n")
	builder.WriteString("The `/farm` command evaluates your current active farm (Virtue, Home, or Enlightenment farm) and recommends optimal artifact loadouts and stone distributions from your inventory.\n\n")
	builder.WriteString("### Subcommands\n")
	builder.WriteString("- `/farm delivery`: Recommends optimal artifacts and stone split (Tachyon vs Quantum) to maximize your egg delivery rate (eggs/hour).\n")
	builder.WriteString("- `/farm earnings`: Recommends optimal artifacts and Lunar stone distribution to maximize your offline away earnings multiplier.\n")
	builder.WriteString("- `/farm drone-farming`: Recommends optimal artifacts (Brooch, Medallion, Beak) and Clarity stone distribution on the Enlightenment farm to maximize drone Golden Egg yield.\n\n")
	builder.WriteString("### Options\n")
	builder.WriteString("- `alt` (string): Select an alternate account linked to your Discord profile to evaluate.\n")
	builder.WriteString("- `help` (boolean): Show this explanation.\n\n")
	builder.WriteString("### Notes\n")
	builder.WriteString("- `/farm` uses your saved Egg Inc ID. If you haven't set one yet, you will be prompted with a secure modal.\n")
	builder.WriteString("- On Virtue farms, items are selected from your Virtue artifact inventory. On standard farms, items are selected from your standard artifact inventory.\n")
	builder.WriteString("- Drone farming is configured for the Enlightenment Farm to take advantage of Clarity Stones amplifying artifact effects.\n")
	builder.WriteString("- If your current chicken population exceeds base hab capacity without a gusset, the optimizer ensures a qualifying gusset is retained to prevent hab lockout.\n")
	return builder.String()
}

