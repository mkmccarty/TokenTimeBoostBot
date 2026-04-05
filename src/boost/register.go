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
			str = "Your Egg Inc ID has been registered as " + newName + fmt.Sprintf(" (TE: %d).", te)
			if farmerName != "" && farmerName != newName {
				str += " (Previously: " + farmerName + ")"
			}
			// Respond to register command
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: str,
				Flags:   discordgo.MessageFlagsEphemeral,
			})
			// Spawn /artifacts command response
			artStr, artComponents := getArtifactsComponents(userID, "", false)
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
