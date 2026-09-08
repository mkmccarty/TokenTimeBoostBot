package boost

import (
	"fmt"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// GetSlashChangeCommand returns the /update slash command with main subcommand groups for farmer and contract
func GetSlashChangeCommand(cmd string) *dc.Command {
	command := guildOnlyCommand(cmd, "Update contract statistics")
	command.Options = []dc.Option{
		dc.SubCommandGroup{
			Name:        "contract",
			Description: "Update contract settings",
			Options: []dc.SubCommand{
				{
					Name:        "coop-id",
					Description: "Update contract coopID",
					Options: []dc.Option{
						dc.StringOption{
							Name:         "coop-id",
							Description:  "New coopID value",
							Required:     true,
							Autocomplete: true,
						},
					},
				},
				{
					Name:        "contract-id",
					Description: "Update contract contractID",
					Options: []dc.Option{
						dc.StringOption{
							Name:         "contract-id",
							Description:  "New contractID value",
							Required:     true,
							Autocomplete: true,
						},
					},
				},
				{
					Name:        "coordinator",
					Description: "Update contract coordinator (must be in contract)",
					Options: []dc.Option{
						dc.UserOption{
							Name:        "user",
							Description: "New coordinator user",
							Required:    true,
						},
					},
				},
			},
		},
		dc.SubCommand{
			Name:        "order",
			Description: "Update contract order",
			Options: []dc.Option{
				dc.StringOption{
					Name:        "boost-order",
					Description: "Provide new boost order. Example: 1,2,3,6,7,5,8-10",
				},
				dc.StringOption{
					Name:        "current-booster",
					Description: "Change the current booster. Example: @farmer",
				},
			},
		},
	}
	return &command
}

// HandleChangeCommand handles the /update slash command through the dc facade.
//
// It still takes a raw session because the contract mutators and the boost
// list redraw are not on the facade yet.
func HandleChangeCommand(client dc.Client, e *dc.CommandEvent) {
	_ = e.Defer(true)

	subcommandGroup := ""
	subcommand := ""
	coopIDValue := ""
	contractIDValue := ""
	currentBooster := ""
	boostOrder := ""

	// Get the subcommand group and subcommand from nested options
	if path := e.SubcommandPath(); len(path) > 0 {
		subcommandGroup = path[0]
		if len(path) > 1 {
			subcommand = path[1]
		}
	}

	// Extract values based on subcommand group
	switch subcommandGroup {

	case "contract":
		if opt, ok := e.OptString("contract-coop-id-coop-id"); ok {
			coopIDValue = strings.TrimSpace(opt)
		}
		if opt, ok := e.OptString("contract-contract-id-contract-id"); ok {
			contractIDValue = strings.TrimSpace(opt)
		}
		if opt, ok := e.OptUser("contract-coordinator-user"); ok {
			coopIDValue = opt.ID // Reuse coopIDValue for coordinator
		}
	case "order":
		if opt, ok := e.OptString("order-current-booster"); ok {
			currentBooster = strings.TrimSpace(opt)
		}
		if opt, ok := e.OptString("order-boost-order"); ok {
			boostOrder = strings.TrimSpace(opt)
		}
	}

	resultMsg := ""

	// Handle the specific subcommand
	switch subcommandGroup {

	case "contract":
		contract := FindContract(e.ChannelID())
		if contract == nil {
			resultMsg = "❌ Contract not found in this channel"
		} else {
			defer saveData(contract.ContractHash)

			switch subcommand {
			case "coop-id":
				_, err := ChangeContractIDs(client, e.GuildID(), e.ChannelID(), e.UserID(), "", coopIDValue, "")
				if err != nil {
					resultMsg = fmt.Sprintf("❌ %s", err.Error())
				} else {
					resultMsg = fmt.Sprintf("✅ Updated coopID to %s", coopIDValue)
					refreshBoostListMessage(client, contract, false)
				}

			case "contract-id":
				movedToWaitlist, err := ChangeContractIDs(client, e.GuildID(), e.ChannelID(), e.UserID(), contractIDValue, "", "")
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
				_, err := ChangeContractIDs(client, e.GuildID(), e.ChannelID(), e.UserID(), "", "", coordinatorID)
				if err != nil {
					resultMsg = fmt.Sprintf("❌ %s", err.Error())
				} else {
					resultMsg = fmt.Sprintf("✅ Updated coordinator to <@%s>", coordinatorID)
					refreshBoostListMessage(client, contract, false)
				}

			default:
				resultMsg = "Unknown contract subcommand"
			}
		}
	case "order":
		contract := FindContract(e.ChannelID())
		if contract == nil {
			resultMsg = "❌ Contract not found in this channel"
		} else {
			defer saveData(contract.ContractHash)
			if boostOrder != "" {
				resultStr, err := ChangeBoostOrder(client, e.GuildID(), e.ChannelID(), e.UserID(), boostOrder, currentBooster == "")
				if err != nil {
					resultMsg += fmt.Sprintf("❌ %s", err.Error())
				} else {
					resultMsg += fmt.Sprintf("✅ %s", resultStr)
					refreshBoostListMessage(client, contract, false)
				}
			}

			if currentBooster != "" {
				if resultMsg != "" {
					resultMsg += "\n"
				}
				err := ChangeCurrentBooster(client, e.GuildID(), e.ChannelID(), e.UserID(), currentBooster, true)
				if err != nil {
					resultMsg += fmt.Sprintf("❌ %s", err.Error())
				} else {
					resultMsg += fmt.Sprintf("✅ Current changed to %s.", currentBooster)
					refreshBoostListMessage(client, contract, false)
				}
			}
		}

	default:
		resultMsg = "Unknown subcommand group"
	}

	_ = e.Followup(dc.Message{
		Content:   resultMsg,
		Ephemeral: true,
	})
}
