package boost

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
)

// GetSlashChangeCommand returns the /update slash command with main subcommand groups for farmer and contract
func GetSlashChangeCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name: cmd,
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
		},
		Description: "Update contract statistics",
		Options: []*discordgo.ApplicationCommandOption{
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
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "order",
				Description: "Update contract order",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "boost-order",
						Description: "Provide new boost order. Example: 1,2,3,6,7,5,8-10",
						Required:    false,
					},
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "current-booster",
						Description: "Change the current booster. Example: @farmer",
						Required:    false,
					},
				},
			}},
	}
}

// HandleChangeCommand handles the /update slash command
func HandleChangeCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	optionMap := bottools.GetCommandOptionsMap(i)
	subcommandGroup := ""
	subcommand := ""
	coopIDValue := ""
	contractIDValue := ""
	currentBooster := ""
	boostOrder := ""

	// Get the subcommand group and subcommand from nested options
	if len(i.ApplicationCommandData().Options) > 0 {
		subcommandGroup = i.ApplicationCommandData().Options[0].Name
		if len(i.ApplicationCommandData().Options[0].Options) > 0 {
			subcommand = i.ApplicationCommandData().Options[0].Options[0].Name
		}
	}

	// Extract values based on subcommand group
	switch subcommandGroup {

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
	case "order":
		if opt, ok := optionMap["order-current-booster"]; ok {
			currentBooster = strings.TrimSpace(opt.StringValue())
		}
		if opt, ok := optionMap["order-boost-order"]; ok {
			boostOrder = strings.TrimSpace(opt.StringValue())
		}
	}

	resultMsg := ""

	// Handle the specific subcommand
	switch subcommandGroup {

	case "contract":
		contract := FindContract(i.ChannelID)
		if contract == nil {
			resultMsg = "❌ Contract not found in this channel"
		} else {
			defer saveData(contract.ContractHash)

			switch subcommand {
			case "coop-id":
				_, err := ChangeContractIDs(s, i.GuildID, i.ChannelID, i.Member.User.ID, "", coopIDValue, "")
				if err != nil {
					resultMsg = fmt.Sprintf("❌ %s", err.Error())
				} else {
					resultMsg = fmt.Sprintf("✅ Updated coopID to %s", coopIDValue)
					refreshBoostListMessage(s, contract, false)
				}

			case "contract-id":
				movedToWaitlist, err := ChangeContractIDs(s, i.GuildID, i.ChannelID, i.Member.User.ID, contractIDValue, "", "")
				if err != nil {
					resultMsg = fmt.Sprintf("❌ %s", err.Error())
				} else {
					resultMsg = fmt.Sprintf("✅ Updated contractID to %s and updated the role to %s", contractIDValue, contract.Location[0].RoleMention)
					if movedToWaitlist > 0 {
						resultMsg += fmt.Sprintf(". Moved %d booster(s) to waitlist.", movedToWaitlist)
					}
				}

			case "coordinator":
				coordinatorID := coopIDValue // Reused variable from above (already extracted as user ID)
				_, err := ChangeContractIDs(s, i.GuildID, i.ChannelID, i.Member.User.ID, "", "", coordinatorID)
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
	case "order":
		contract := FindContract(i.ChannelID)
		if contract == nil {
			resultMsg = "❌ Contract not found in this channel"
		} else {
			defer saveData(contract.ContractHash)
			if boostOrder != "" {
				resultStr, err := ChangeBoostOrder(s, i.GuildID, i.ChannelID, i.Member.User.ID, boostOrder, currentBooster == "")
				if err != nil {
					resultMsg += fmt.Sprintf("❌ %s", err.Error())
				} else {
					resultMsg += fmt.Sprintf("✅ %s", resultStr)
					refreshBoostListMessage(s, contract, false)
				}
			}

			if currentBooster != "" {
				if resultMsg != "" {
					resultMsg += "\n"
				}
				err := ChangeCurrentBooster(s, i.GuildID, i.ChannelID, i.Member.User.ID, currentBooster, true)
				if err != nil {
					resultMsg += fmt.Sprintf("❌ %s", err.Error())
				} else {
					resultMsg += fmt.Sprintf("✅ Current changed to %s.", currentBooster)
					refreshBoostListMessage(s, contract, false)
				}
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
