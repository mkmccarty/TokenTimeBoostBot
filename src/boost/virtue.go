package boost

import (
	"encoding/base64"
	"fmt"
	"log"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

	"github.com/bwmarrin/discordgo"
)

// GetSlashVirtueCommand returns the command for the /launch-helper command
func GetSlashVirtueCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Evaluate contract history and provide replay guidance.",
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
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "reset",
				Description: "Reset stored EI number",
				Required:    false,
			},
		},
	}
}

// HandleVirtue handles the /virtue command
func HandleVirtue(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := bottools.GetInteractionUserID(i)
	percent := -1

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["reset"]; ok {
		if opt.BoolValue() {
			farmerstate.SetMiscSettingString(userID, "encrypted_ei_id", "")
		}
	}

	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")

	Virtue(s, i, percent, eiID, true)
}

// Virtue processes the virtue command
func Virtue(s *discordgo.Session, i *discordgo.InteractionCreate, percent int, eiID string, okayToSave bool) {
	// Get the Egg Inc ID from the stored settings
	eggIncID := ""
	encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
	if err == nil {
		decodedData, err := base64.StdEncoding.DecodeString(eiID)
		if err == nil {
			decryptedData, err := config.DecryptCombined(encryptionKey, decodedData)
			if err == nil {
				eggIncID = string(decryptedData)
			}
		}
	}
	if eggIncID == "" || len(eggIncID) != 18 || eggIncID[:2] != "EI" {
		RequestEggIncIDModal(s, i, fmt.Sprintf("virtue#%d", 0))
		return
	}

	// Quick reply to buy us some time
	flags := discordgo.MessageFlagsIsComponentsV2
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			Flags:   flags,
		},
	})

	userID := bottools.GetInteractionUserID(i)

	backup, _ := ei.GetFirstContactFromAPI(s, eggIncID, userID, okayToSave)

	virtue := backup.GetVirtue()
	str := printVirtue(virtue)
	if str == "" {
		str = "No archived contracts found in Egg Inc API response"
	}
	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Flags: flags,
		Components: []discordgo.MessageComponent{
			discordgo.TextDisplay{Content: str},
		},
	})

}

func printVirtue(virtue *ei.Backup_Virtue) string {
	builder := strings.Builder{}
	if virtue == nil {
		log.Print("No virtue backup data found in Egg Inc API response")
		return builder.String()
	}

	fmt.Fprintf(&builder, "# Eggs Of Virtue\n")
	fmt.Fprintf(&builder, "Shift Count: %d\n", virtue.GetShiftCount())
	fmt.Fprintf(&builder, "Resets: %d\n", virtue.GetResets())
	fmt.Fprintf(&builder, "Inventory Score %.0f\n", virtue.GetAfx().GetInventoryScore())

	virtueEggs := []string{"CURIOSITY", "INTEGRITY", "HUMILITY", "RESILIENCE", "KINDNESS"}

	for i, egg := range virtueEggs {
		eov := virtue.GetEovEarned()[i] // Assuming Eggs is the correct field for accessing egg virtues
		delivered := virtue.GetEggsDelivered()[i]
		fmt.Fprintf(&builder, "%s %d: %.0f\n", ei.GetBotEmojiMarkdown("egg_"+strings.ToLower(egg)), eov, delivered)
	}

	return builder.String()
}
