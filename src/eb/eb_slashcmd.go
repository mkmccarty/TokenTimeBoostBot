package eb

import (
	"fmt"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

const (
	FarmHome                  = "Home"
	FarmVirtue                = "Virtue"
	FarmHomeAndVirtue         = "Home & Virtue"
	DefaultFarmChoice         = FarmHomeAndVirtue
	StickySettingEbFarm       = "eb-farm"
	StickySettingEbShowAvatar = "eb-show-avatar"
)

func init() {
	boost.RegisterEggIDModalAction("eb", func(e *dc.ModalEvent, options dc.OptionValues, encryptedID string, okayToSave bool) {
		HandleEbModal(e, options, encryptedID, okayToSave)
	})
}

// GetSlashEbCommand returns the slash command definition for /eb.
func GetSlashEbCommand(cmd string) *dc.Command {
	command := dc.Command{
		Name:        cmd,
		Description: "Show player's Earnings Bonus, roles, and farm details.",
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
			dc.StringOption{
				Name:        "farm",
				Description: "Farm view: Home, Virtue, or Home & Virtue. Default is Home & Virtue. (sticky)",
				Required:    false,
				Choices: []dc.Choice[string]{
					{Name: FarmHomeAndVirtue, Value: FarmHomeAndVirtue},
					{Name: FarmHome, Value: FarmHome},
					{Name: FarmVirtue, Value: FarmVirtue},
				},
			},
			dc.BoolOption{
				Name:        "show-avatar",
				Description: "Show player's avatar image. Default is false. (sticky)",
				Required:    false,
			},
			dc.StringOption{
				Name:         "alt",
				Description:  "Show EB for a registered alternate account.",
				Required:     false,
				Autocomplete: true,
			},
		},
	}
	return &command
}

// AltAccount represents an alternate account with a saved Egg Inc ID.
type AltAccount = farmerstate.AltAccount

// findAltDiscordID returns the registered Discord user ID for an alt, if any.
func findAltDiscordID(altID string) string {
	return farmerstate.FindAltDiscordID(altID)
}

// getDiscordAvatarURL looks up the avatar image URL for a given Discord user ID.
func getDiscordAvatarURL(client dc.Client, guildID string, discordID string) string {
	return bottools.GetDiscordAvatarURL(client, guildID, discordID)
}

// GetUserAltsWithSavedEID returns all registered alternate accounts for userID that have a saved Egg Inc ID.
func GetUserAltsWithSavedEID(userID string) []AltAccount {
	return farmerstate.GetUserAltsWithSavedEID(userID)
}

// resolveAltSelection determines which user ID and Egg Inc ID to display, along with any informational message.
func resolveAltSelection(userID string, altParam string) (targetID string, eggIncID string, notice string) {
	return farmerstate.ResolveAltSelection(userID, altParam, "EB")
}

// HandleEbAutocomplete handles autocomplete events for the /eb command.
func HandleEbAutocomplete(e *dc.AutocompleteEvent) {
	name, value := e.FocusedOption()
	if name != "alt" {
		_ = e.RespondChoices(nil)
		return
	}

	alts := GetUserAltsWithSavedEID(farmerstate.GetEffectiveUserID(e.UserID(), e.ChannelID()))
	if len(alts) == 0 {
		_ = e.RespondChoices(nil)
		return
	}

	value = strings.ToLower(strings.TrimSpace(value))
	choices := make([]dc.Choice[string], 0, len(alts))
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

// NormalizeFarmChoice cleans and standardizes a user-supplied farm choice.
func NormalizeFarmChoice(choice string) string {
	switch strings.ToLower(strings.TrimSpace(choice)) {
	case "home":
		return FarmHome
	case "virtue":
		return FarmVirtue
	case "home & virtue", "home and virtue", "both", "all":
		return FarmHomeAndVirtue
	default:
		return DefaultFarmChoice
	}
}

// HandleEb handles the /eb command.
func HandleEb(e *dc.CommandEvent) {
	effectiveUserID, expiresAt, isActAs := farmerstate.GetEffectiveUserIDAndExpiry(e.UserID(), e.ChannelID())
	userID := effectiveUserID

	farmChoice := DefaultFarmChoice
	if opt, ok := e.OptString("farm"); ok && strings.TrimSpace(opt) != "" {
		farmChoice = NormalizeFarmChoice(opt)
		farmerstate.SetMiscSettingString(userID, StickySettingEbFarm, farmChoice)
	} else {
		saved := farmerstate.GetMiscSettingString(userID, StickySettingEbFarm)
		if saved != "" {
			farmChoice = NormalizeFarmChoice(saved)
		}
	}

	showAvatar := false
	if opt, ok := e.OptBool("show-avatar"); ok {
		showAvatar = opt
		farmerstate.SetMiscSettingFlag(userID, StickySettingEbShowAvatar, showAvatar)
	} else {
		showAvatar = farmerstate.GetMiscSettingFlag(userID, StickySettingEbShowAvatar)
	}

	altParam, _ := e.OptString("alt")
	targetID, eggIncID, notice := resolveAltSelection(userID, altParam)

	if isActAs && altParam == "" {
		var actAsNotice string
		if farmerstate.IsActAsForever(expiresAt) {
			actAsNotice = fmt.Sprintf("Acting as <@%s> via /act-as", effectiveUserID)
		} else {
			actAsNotice = fmt.Sprintf("Acting as <@%s> via /act-as (expires <t:%d:R>)", effectiveUserID, expiresAt.Unix())
		}
		if notice != "" {
			notice = actAsNotice + "\n" + notice
		} else {
			notice = actAsNotice
		}
	}

	if eggIncID == "" {
		eggIncID = getEggIncID(targetID)
	}

	if eggIncID == "" {
		boost.RequestEggIncIDModal(e, "eb", e.Options())
		return
	}

	ExecuteEbTarget(e, farmChoice, eggIncID, showAvatar, true, targetID, notice)
}

// HandleEbModal handles the modal submission when a user provides their Egg Inc ID.
func HandleEbModal(e *dc.ModalEvent, options dc.OptionValues, encryptedID string, okayToSave bool) {
	effectiveUserID, expiresAt, isActAs := farmerstate.GetEffectiveUserIDAndExpiry(e.UserID(), e.ChannelID())
	userID := effectiveUserID

	farmChoice := DefaultFarmChoice
	if opt, ok := options.String("farm"); ok && strings.TrimSpace(opt) != "" {
		farmChoice = NormalizeFarmChoice(opt)
		farmerstate.SetMiscSettingString(userID, StickySettingEbFarm, farmChoice)
	} else {
		saved := farmerstate.GetMiscSettingString(userID, StickySettingEbFarm)
		if saved != "" {
			farmChoice = NormalizeFarmChoice(saved)
		}
	}

	showAvatar := false
	if opt, ok := options.Bool("show-avatar"); ok {
		showAvatar = opt
		farmerstate.SetMiscSettingFlag(userID, StickySettingEbShowAvatar, showAvatar)
	} else {
		showAvatar = farmerstate.GetMiscSettingFlag(userID, StickySettingEbShowAvatar)
	}

	altParam, _ := options.String("alt")
	targetID, altEggIncID, notice := resolveAltSelection(userID, altParam)

	if isActAs && altParam == "" {
		var actAsNotice string
		if farmerstate.IsActAsForever(expiresAt) {
			actAsNotice = fmt.Sprintf("Acting as <@%s> via /act-as", effectiveUserID)
		} else {
			actAsNotice = fmt.Sprintf("Acting as <@%s> via /act-as (expires <t:%d:R>)", effectiveUserID, expiresAt.Unix())
		}
		if notice != "" {
			notice = actAsNotice + "\n" + notice
		} else {
			notice = actAsNotice
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

	ExecuteEbTarget(e, farmChoice, eggIncID, showAvatar, okayToSave, targetID, notice)
}
