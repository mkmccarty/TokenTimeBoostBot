package boost

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

	"github.com/bwmarrin/discordgo"
)

// GetSlashRegisterCommand returns the /register command
func GetSlashRegisterCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Register your Egg Inc ID with Boost Bot.",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
			discordgo.InteractionContextBotDM,
			discordgo.InteractionContextPrivateChannel,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
			discordgo.ApplicationIntegrationUserInstall,
		},
	}
}

// GetSlashRegisterAltCommand returns the /register-alt command
func GetSlashRegisterAltCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Register an alternate Egg Inc ID with Boost Bot.",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
			discordgo.InteractionContextBotDM,
			discordgo.InteractionContextPrivateChannel,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
			discordgo.ApplicationIntegrationUserInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "name",
				Description:  "The name of the alternate account or 'new' for a new one.",
				Required:     true,
				Autocomplete: true,
			},
		},
	}
}

// HandleRegisterAlt handles the /register-alt command
func HandleRegisterAlt(s *discordgo.Session, i *discordgo.InteractionCreate) {
	optionMap := bottools.GetCommandOptionsMap(i)
	targetAlt := ""
	if opt, ok := optionMap["name"]; ok {
		targetAlt = opt.StringValue()
	}
	RequestEggIncIDModal(s, i, "register-alt#"+targetAlt, optionMap)
}

// HandleRegisterAltAutocomplete handles the autocomplete for the /register-alt command
func HandleRegisterAltAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := bottools.GetInteractionUserID(i)
	alts := farmerstate.GetAltControllerByMiscString("AltController", userID)
	choices := []*discordgo.ApplicationCommandOptionChoice{
		{Name: "New Alternate", Value: "new"},
	}
	for _, alt := range alts {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  ei.NormalizePlayerNameForDisplay(alt),
			Value: alt,
		})
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	})
}

// HandleRegister handles the /register command
func HandleRegister(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := bottools.GetInteractionUserID(i)
	optionMap := bottools.GetCommandOptionsMap(i)
	if opt, ok := optionMap["reset"]; ok {
		if opt.BoolValue() {
			farmerstate.SetMiscSettingString(userID, "encrypted_ei_id", "")
		}
	}
	RequestEggIncIDModal(s, i, "register", optionMap)
}

// Register processes the register modal submission, saves the EI ID, and pulls a backup to update the player's IGN.
func Register(s *discordgo.Session, i *discordgo.InteractionCreate, encryptedID string, okayToSave bool) {
	userID := bottools.GetInteractionUserID(i)

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	eggIncID := ""
	encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
	if err == nil {
		decodedData, err := base64.StdEncoding.DecodeString(encryptedID)
		if err == nil {
			decryptedData, err := config.DecryptCombined(encryptionKey, decodedData)
			if err == nil {
				eggIncID = string(decryptedData)
			}
		}
	}

	var str string
	if eggIncID == "" {
		str = "Your Egg Inc ID could not be saved."
	} else {
		backup, _ := ei.GetFirstContactFromAPI(s, eggIncID, userID, okayToSave)
		if backup == nil {
			str = "Your Egg Inc ID was saved but the backup could not be retrieved from EI."
		} else {
			farmerName := farmerstate.GetMiscSettingString(userID, "ei_ign")
			newName := backup.GetUserName()
			displayName := ei.NormalizePlayerNameForDisplay(newName)
			farmerstate.SetMiscSettingString(userID, "ei_ign", newName)
			te := ei.GetCurrentTruthEggs(backup)
			farmerstate.SetMiscSettingString(userID, "TE", fmt.Sprintf("%d", te))
			artifacts := ei.GetBestCoopArtifactsFromInventory(backup.GetArtifactsDb().GetInventoryItems())
			for key, val := range artifacts {
				farmerstate.SetMiscSettingString(userID, key, val)
			}
			// Set all collectibles to "owned" by default
			if farmerstate.GetMiscSettingString(userID, "collegg") == "" {
				var eggNames []string
				for _, egg := range ei.CustomEggMap {
					eggNames = append(eggNames, egg.Name)
				}
				farmerstate.SetMiscSettingString(userID, "collegg", strings.Join(eggNames, ","))
			}
			str = "Your Egg Inc ID has been registered as " + displayName + fmt.Sprintf(" (TE: %d).", te)
			if farmerName != "" && farmerName != newName {
				str += " (Previously: " + ei.NormalizePlayerNameForDisplay(farmerName) + ")"
			}
			// Respond to register command
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: str,
				Flags:   discordgo.MessageFlagsEphemeral,
			})
			// Spawn /artifacts command response
			artStr, artComponents := getArtifactsComponents(userID, "", false, "delivery", "")
			artStr = "## Check your collectibles\n" + artStr
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content:    artStr,
				Components: artComponents,
				Flags:      discordgo.MessageFlagsEphemeral,
			})
			return
		}
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: str,
		Flags:   discordgo.MessageFlagsEphemeral,
	})
}

// RegisterAlt processes the register-alt modal submission.
func RegisterAlt(s *discordgo.Session, i *discordgo.InteractionCreate, targetAlt string, encryptedID string) {
	parentUserID := bottools.GetInteractionUserID(i)

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	var str string
	if targetAlt != "new" && encryptedID == "" {
		// Clear existing alt
		farmerstate.SetMiscSettingString(targetAlt, "encrypted_ei_id", "")
		str = fmt.Sprintf("Egg Inc ID for alternate %s has been cleared.", targetAlt)
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: str,
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	if encryptedID == "" {
		str = "You must provide a valid Egg Inc ID for a new alternate."
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: str,
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	eggIncID := ""
	encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
	if err == nil {
		decodedData, err := base64.StdEncoding.DecodeString(encryptedID)
		if err == nil {
			decryptedData, err := config.DecryptCombined(encryptionKey, decodedData)
			if err == nil {
				eggIncID = string(decryptedData)
			}
		}
	}

	if eggIncID == "" {
		str = "Your Egg Inc ID could not be saved."
	} else {
		backup, _ := ei.GetFirstContactFromAPI(s, eggIncID, parentUserID, true)
		if backup == nil {
			str = "Your Egg Inc ID was saved but the backup could not be retrieved from EI."
		} else {
			newName := backup.GetUserName()
			displayName := ei.NormalizePlayerNameForDisplay(newName)
			altID := targetAlt
			if altID == "new" {
				altID = newName
			}
			// Use altID as the ID for the alt record
			farmerstate.SetMiscSettingString(altID, "ei_ign", newName)
			farmerstate.SetMiscSettingString(altID, "encrypted_ei_id", encryptedID)
			farmerstate.SetMiscSettingString(altID, "AltController", parentUserID)

			te := ei.GetCurrentTruthEggs(backup)
			farmerstate.SetMiscSettingString(altID, "TE", fmt.Sprintf("%d", te))
			artifacts := ei.GetBestCoopArtifactsFromInventory(backup.GetArtifactsDb().GetInventoryItems())
			for key, val := range artifacts {
				farmerstate.SetMiscSettingString(altID, key, val)
			}
			str = "Your alternate Egg Inc ID has been registered as " + displayName + fmt.Sprintf(" (TE: %d).", te)
		}
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: str,
		Flags:   discordgo.MessageFlagsEphemeral,
	})
}
