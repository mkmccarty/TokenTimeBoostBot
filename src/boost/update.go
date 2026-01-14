package boost

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

// GetSlashUpdateCommand returns the /update slash command with subcommands for boost-tokens and TE
func GetSlashUpdateCommand(cmd string) *discordgo.ApplicationCommand {
	intMin0 := float64(0)
	intMax12 := float64(12)
	intMax490 := float64(490)

	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Update farmer statistics",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "boost-tokens",
				Description: "Update boost tokens (0-12)",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "farmername",
						Description: "Farmer name to update",
						Required:    true,
					},
					{
						Type:        discordgo.ApplicationCommandOptionInteger,
						Name:        "value",
						Description: "Number of boost tokens (0-12)",
						Required:    true,
						MinValue:    &intMin0,
						MaxValue:    intMax12,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "te",
				Description: "Update TE value (0-490)",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "farmername",
						Description: "Farmer name to update",
						Required:    true,
					},
					{
						Type:        discordgo.ApplicationCommandOptionInteger,
						Name:        "value",
						Description: "TE value (0-490)",
						Required:    true,
						MinValue:    &intMin0,
						MaxValue:    intMax490,
					},
				},
			},
		},
	}
}

// HandleUpdateCommand handles the /update slash command
func HandleUpdateCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	optionMap := bottools.GetCommandOptionsMap(i)
	subcommand := ""
	farmername := ""
	value := int64(0)

	if opt, ok := optionMap["te-farmername"]; ok {
		farmername = strings.TrimSpace(opt.StringValue())
	}
	if opt, ok := optionMap["te-value"]; ok {
		value = opt.IntValue()
	}
	if opt, ok := optionMap["boost-tokens-farmername"]; ok {
		farmername = strings.TrimSpace(opt.StringValue())
	}
	if opt, ok := optionMap["boost-tokens-value"]; ok {
		value = opt.IntValue()
	}

	// Get the subcommand name
	if len(i.ApplicationCommandData().Options) > 0 {
		subcommand = i.ApplicationCommandData().Options[0].Name
	}

	userID := farmername
	resultMsg := ""

	// Try to find the user by farmername or discord mention
	if strings.HasPrefix(farmername, "<@") {
		// Extract user ID from mention
		mention := farmername[2 : len(farmername)-1]
		if mention[0] == '!' {
			mention = mention[1:]
		}
		userID = mention
	}

	// Handle the specific subcommand
	switch subcommand {
	case "boost-tokens":
		farmerstate.SetTokens(userID, int(value))
		resultMsg = fmt.Sprintf("✅ Updated %s's boost tokens to %d", farmername, value)

	case "te":
		farmerstate.SetMiscSettingString(userID, "TE", fmt.Sprintf("%d", value))
		resultMsg = fmt.Sprintf("✅ Updated %s's TE to %d", farmername, value)

	default:
		resultMsg = "Unknown subcommand"
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: resultMsg,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	// If this farmer is in any contracts, update their contract.Boosters data with the changed value
	// only update the value if it was changed by this command, don't overwrite other data
	updateFarmerInContracts(s, userID, subcommand, value)

}

// updateFarmerInContracts updates a farmer's data in all contracts they're part of
func updateFarmerInContracts(s *discordgo.Session, userID string, subcommand string, value int64) {
	for _, contract := range Contracts {
		if booster, exists := contract.Boosters[userID]; exists {
			// Update the specific field based on which subcommand was used
			switch subcommand {
			case "boost-tokens":
				booster.TokensWanted = int(value)
			case "te":
				booster.TECount = int(value)
			}

			// Redraw the boost list message to reflect the updated data
			for _, loc := range contract.Location {
				_ = loc
				refreshBoostListMessage(s, contract, false)
				break // Only need to refresh once
			}
		}
	}
}
