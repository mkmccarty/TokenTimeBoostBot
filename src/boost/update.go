package boost

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

// GetSlashUpdateCommand returns the /update slash command with main subcommand groups for farmer and contract
func GetSlashUpdateCommand(cmd string) *discordgo.ApplicationCommand {
	intMin0 := float64(0)
	intMax12 := float64(12)
	intMax490 := float64(490)

	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Update farmer or contract statistics",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
				Name:        "farmer",
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
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
				Name:        "contract",
				Description: "Update contract settings",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionSubCommand,
						Name:        "coop-id",
						Description: "Update contract coopID",
						Options: []*discordgo.ApplicationCommandOption{
							{
								Type:        discordgo.ApplicationCommandOptionString,
								Name:        "coop-id",
								Description: "New coopID value",
								Required:    true,
							},
						},
					},
					{
						Type:        discordgo.ApplicationCommandOptionSubCommand,
						Name:        "contract-id",
						Description: "Update contract contractID",
						Options: []*discordgo.ApplicationCommandOption{
							{
								Type:         discordgo.ApplicationCommandOptionString,
								Name:         "contract-id",
								Description:  "New contractID value",
								Required:     true,
								Autocomplete: true,
							},
						},
					},
					{
						Type:        discordgo.ApplicationCommandOptionSubCommand,
						Name:        "coordinator",
						Description: "Update contract coordinator (must be in contract)",
						Options: []*discordgo.ApplicationCommandOption{
							{
								Type:        discordgo.ApplicationCommandOptionUser,
								Name:        "user",
								Description: "New coordinator user",
								Required:    true,
							},
						},
					},
				},
			},
		},
	}
}

// HandleUpdateCommand handles the /update slash command
func HandleUpdateCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	optionMap := bottools.GetCommandOptionsMap(i)
	subcommandGroup := ""
	subcommand := ""
	farmername := ""
	value := int64(0)
	coopIDValue := ""
	contractIDValue := ""

	// Get the subcommand group and subcommand from nested options
	if len(i.ApplicationCommandData().Options) > 0 {
		subcommandGroup = i.ApplicationCommandData().Options[0].Name
		if len(i.ApplicationCommandData().Options[0].Options) > 0 {
			subcommand = i.ApplicationCommandData().Options[0].Options[0].Name
		}
	}

	// Extract values based on subcommand group
	switch subcommandGroup {
	case "farmer":
		if opt, ok := optionMap["farmer-boost-tokens-farmername"]; ok {
			farmername = strings.TrimSpace(opt.StringValue())
		}
		if opt, ok := optionMap["farmer-boost-tokens-value"]; ok {
			value = opt.IntValue()
		}
		if opt, ok := optionMap["farmer-te-farmername"]; ok {
			farmername = strings.TrimSpace(opt.StringValue())
		}
		if opt, ok := optionMap["farmer-te-value"]; ok {
			value = opt.IntValue()
		}

	case "contract":
		if opt, ok := optionMap["contract-coop-id-coop-id"]; ok {
			coopIDValue = strings.TrimSpace(opt.StringValue())
		}
		if opt, ok := optionMap["contract-contract-id-contract-id"]; ok {
			contractIDValue = strings.TrimSpace(opt.StringValue())
		}
		if opt, ok := optionMap["contract-coordinator-user"]; ok {
			coopIDValue = opt.UserValue(s).ID // Reuse coopIDValue for coordinator
		}
	}

	userID := farmername
	resultMsg := ""

	// Try to find the user by farmername or discord mention
	if farmername != "" && strings.HasPrefix(farmername, "<@") {
		// Extract user ID from mention
		mention := farmername[2 : len(farmername)-1]
		if mention[0] == '!' {
			mention = mention[1:]
		}
		userID = mention
	}

	// Handle the specific subcommand
	switch subcommandGroup {
	case "farmer":
		switch subcommand {
		case "boost-tokens":
			farmerstate.SetTokens(userID, int(value))
			resultMsg = fmt.Sprintf("✅ Updated %s's boost tokens to %d", farmername, value)

		case "te":
			farmerstate.SetMiscSettingString(userID, "TE", fmt.Sprintf("%d", value))
			resultMsg = fmt.Sprintf("✅ Updated %s's TE to %d", farmername, value)

		default:
			resultMsg = "Unknown farmer subcommand"
		}

		// If this farmer is in any contracts, update their contract.Boosters data with the changed value
		updateFarmerInContracts(s, userID, subcommand, value)

	case "contract":
		contract := FindContract(i.ChannelID)
		if contract == nil {
			resultMsg = "❌ Contract not found in this channel"
		} else {
			defer saveData(contract.ContractHash)

			switch subcommand {
			case "coop-id":
				err := ChangeContractIDs(s, i.GuildID, i.ChannelID, i.Member.User.ID, "", coopIDValue, "")
				if err != nil {
					resultMsg = fmt.Sprintf("❌ %s", err.Error())
				} else {
					resultMsg = fmt.Sprintf("✅ Updated coopID to %s", coopIDValue)
					refreshBoostListMessage(s, contract, false)
				}

			case "contract-id":
				err := ChangeContractIDs(s, i.GuildID, i.ChannelID, i.Member.User.ID, contractIDValue, "", "")
				if err != nil {
					resultMsg = fmt.Sprintf("❌ %s", err.Error())
				} else {
					resultMsg = fmt.Sprintf("✅ Updated contractID to %s", contractIDValue)
					refreshBoostListMessage(s, contract, false)
				}

			case "coordinator":
				coordinatorID := coopIDValue // Reused variable from above (already extracted as user ID)
				err := ChangeContractIDs(s, i.GuildID, i.ChannelID, i.Member.User.ID, "", "", coordinatorID)
				if err != nil {
					resultMsg = fmt.Sprintf("❌ %s", err.Error())
				} else {
					resultMsg = fmt.Sprintf("✅ Updated coordinator to <@%s>", coordinatorID)
					refreshBoostListMessage(s, contract, false)
				}

			default:
				resultMsg = "Unknown contract subcommand"
			}
		}

	default:
		resultMsg = "Unknown subcommand group"
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: resultMsg,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
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
