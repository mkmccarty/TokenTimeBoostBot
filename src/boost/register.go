package boost

import (
	"encoding/base64"
	"fmt"

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
			str = "Your Egg Inc ID has been registered as " + newName + fmt.Sprintf(" (TE: %d).", te)
			if farmerName != "" && farmerName != newName {
				str += " (Previously: " + farmerName + ")"
			}
		}
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: str,
		Flags:   discordgo.MessageFlagsEphemeral,
	})
}
