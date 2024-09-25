package boost

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
)

func getSignupContractSettings(channelID string, id string, thread bool) (string, []discordgo.MessageComponent) {
	minValues := 1

	// is this channelID a thread

	var builder strings.Builder
	fmt.Fprintf(&builder, "Contract created in <#%s>\n", channelID)
	builder.WriteString("Use the Contract button if you have to recycle it.\n")
	builder.WriteString("**Use the menus to set your contract style. These will work until the contract is started.**\n")
	builder.WriteString("If this contract isn't an immediate start use `/change-planned-start` to add the time to the sign-up message.\n")
	if thread {
		builder.WriteString("React with 🌊 on the boost list to automaticaly update the thread name (`/rename-thread`).")
	} else {
		builder.WriteString("This contract is in a channel and it cannot be renamed. Create it in a thread to permit renaming.")
	}

	contract := Contracts[id]
	tokenName := strings.Split(contract.TokenReactionStr, ":")[0]
	tokenID := strings.Split(contract.TokenReactionStr, ":")[1]

	elrName := "ELR"
	elrIcon := "1288152787494109216"

	if config.IsDevBot() {
		elrIcon = "1288152690001580072"
	}

	return builder.String(), []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "cs_#style#" + id,
					Placeholder: "Select contract styles",
					MinValues:   &minValues,
					MaxValues:   1,
					Options: []discordgo.SelectMenuOption{
						{
							Label:       "Boost List Style",
							Description: "Everyone sends tokens to the current booster",
							Value:       "boostlist",
							Default:     (contract.Style & ContractFlagFastrun) != 0,
							Emoji: &discordgo.ComponentEmoji{
								Name: "📜",
							},
						},
						{
							Label:       "Banker Style",
							Description: "Everyone sends tokens to a banker.",
							Value:       "banker",
							Default:     (contract.Style & ContractFlagBanker) != 0,
							Emoji: &discordgo.ComponentEmoji{
								Name: "💰",
							},
						},
					},
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "cs_#crt#" + id,
					Placeholder: "Choose your options for CRT",
					MinValues:   &minValues,
					MaxValues:   1,
					Options: []discordgo.SelectMenuOption{
						{
							Label:       "No-CRT",
							Description: "Standard vanilla option for this contract",
							Value:       "no_crt",
							Default:     (contract.Style & ContractFlagCrt) == 0,
							Emoji: &discordgo.ComponentEmoji{
								Name: "🍦",
							},
						},
						{
							Label:       "CRT",
							Description: "Chicken Run Tango",
							Value:       "crt",
							Default:     (contract.Style&ContractFlagCrt) != 0 && (contract.Style&ContractFlagSelfRuns) == 0,
							Emoji: &discordgo.ComponentEmoji{
								Name: "🔁",
							},
						},
						{
							Label:       "CRT+selfrun",
							Description: "Less Tango Legs ",
							Value:       "self_runs",
							Default:     (contract.Style&ContractFlagCrt) != 0 && (contract.Style&ContractFlagSelfRuns) != 0,
							Emoji: &discordgo.ComponentEmoji{
								Name: "🔂",
							},
						},
					},
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "cs_#order#" + id,
					Placeholder: "Select the boosting order for this contract",
					MinValues:   &minValues,
					MaxValues:   1,
					Options: []discordgo.SelectMenuOption{
						{
							Label:       "Sign-up Order",
							Description: "Boost list is in the order farmers sign up",
							Value:       "signup",
							Emoji: &discordgo.ComponentEmoji{
								Name: "😑",
							},
							Default: contract.BoostOrder == ContractOrderSignup,
						},
						{
							Label:       "Reverse Sign-up Order",
							Description: "Boost list is in the reverse order farmers sign up",
							Value:       "reverse",
							Emoji: &discordgo.ComponentEmoji{
								Name: "😬",
							},
							Default: contract.BoostOrder == ContractOrderReverse,
						},
						{
							Label:       "Fair Order",
							Description: "Boost order is based order history in last 5 contracts",
							Value:       "fair",
							Emoji: &discordgo.ComponentEmoji{
								Name: "😇",
							},
							Default: contract.BoostOrder == ContractOrderFair,
						},
						{
							Label:       "Random Order",
							Description: "Boost order is random",
							Value:       "random",
							Emoji: &discordgo.ComponentEmoji{
								Name: "🤪",
							},

							Default: contract.BoostOrder == ContractOrderRandom,
						},
						{
							Label:       "ELR Order",
							Description: "Highest Egg Lay Rate first",
							Value:       "elr",
							Emoji: &discordgo.ComponentEmoji{
								Name: elrName,
								ID:   elrIcon,
							},

							Default: contract.BoostOrder == ContractOrderELR,
						},
						{
							Label:       "Token Value Order",
							Description: "Highest token value boosts earlier",
							Value:       "tval",
							Emoji: &discordgo.ComponentEmoji{
								Name: tokenName,
								ID:   tokenID,
							},
							Default: contract.BoostOrder == ContractOrderTVal,
						},
					},
				},
			},
		},
	}

}

// GetSignupComponents returns the signup components for a contract
func GetSignupComponents(disableStartContract bool, contract *Contract) (string, []discordgo.MessageComponent) {
	var str = "Join the contract and indicate the number boost tokens you'd like."
	if contract.State == ContractStateSignup && contract.Style&ContractFlagBanker != 0 {
		str += "\nThe Sink boost position button cycles from First->Last->Follow Order."
	}
	startLabel := "Start Boost List"
	if contract != nil && contract.Style&ContractFlagCrt != 0 {
		startLabel = "Start CRT"
	} else if disableStartContract {
		startLabel = "Started"
	}

	if contract != nil {

		// There needs to be at least one booster to start the contract
		if len(contract.Boosters) == 0 {
			disableStartContract = false
		} else if contract.CreatorID[0] == config.DiscordAppID {
			// If the Bot is the creator, then don't allow the contract to be started
			disableStartContract = true
		} else {
			disableStartContract = false
		}
		// If Banker style then we need to have at least a banker sink
		if contract.Style&ContractFlagBanker != 0 && contract.Banker.BoostingSinkUserID == "" {
			disableStartContract = true
		}
		if contract.State != ContractStateSignup {
			disableStartContract = true
		}
		/*
			// If the contract is both Banker and CRT it needs crt, boost and post sink
			if contract.Style&ContractFlagBanker != 0 && contract.Style&ContractFlagCrt != 0 {
				if contract.Banker.CrtSinkUserID == "" || contract.Banker.BoostingSinkUserID == "" || contract.Banker.PostSinkUserID == "" {
					disableStartContract = true
				}
			}
		*/

	}

	// Build the return message
	var buttons []discordgo.MessageComponent
	// Add the buttons to join, leave, and start the contract
	buttons = append(buttons, discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{
				Emoji: &discordgo.ComponentEmoji{
					Name: "🧑‍🌾",
				},
				Label:    "Join",
				Style:    discordgo.PrimaryButton,
				CustomID: "fd_signupFarmer",
			},
			discordgo.Button{
				Emoji: &discordgo.ComponentEmoji{
					Name: "🔔",
				},
				Label:    "Join w/Ping",
				Style:    discordgo.PrimaryButton,
				CustomID: "fd_signupBell",
			},
			discordgo.Button{
				Emoji: &discordgo.ComponentEmoji{
					Name: "❌",
				},
				Label:    "Leave",
				Style:    discordgo.SecondaryButton,
				CustomID: "fd_signupLeave",
			},
			discordgo.Button{
				Emoji: &discordgo.ComponentEmoji{
					Name: "⏱️",
				},
				Label:    startLabel,
				Style:    discordgo.SuccessButton,
				CustomID: "fd_signupStart",
				Disabled: disableStartContract,
			},
			discordgo.Button{
				Emoji: &discordgo.ComponentEmoji{
					Name: "♻️",
				},
				Label:    "Contract",
				Style:    discordgo.DangerButton,
				Disabled: false,
				CustomID: "fd_delete",
			},
		},
	})

	// Add the buttons to adjust the numbers of tokens
	buttons = append(buttons, discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{
				Emoji: &discordgo.ComponentEmoji{
					Name: "5️⃣",
				},
				Label:    " Tokens",
				Style:    discordgo.SecondaryButton,
				CustomID: "fd_tokens5",
			},
			discordgo.Button{
				Emoji: &discordgo.ComponentEmoji{
					Name: "6️⃣",
				},
				Label:    " Tokens",
				Style:    discordgo.SecondaryButton,
				CustomID: "fd_tokens6",
			},
			discordgo.Button{
				Emoji: &discordgo.ComponentEmoji{
					Name: "8️⃣",
				},
				Label:    " Tokens",
				Style:    discordgo.SecondaryButton,
				CustomID: "fd_tokens8",
			},
			discordgo.Button{
				Label:    "+ Token",
				Style:    discordgo.SecondaryButton,
				CustomID: "fd_tokens1",
			},
			discordgo.Button{
				Label:    "- Token",
				Style:    discordgo.SecondaryButton,
				CustomID: "fd_tokens_sub",
			},
		},
	})

	if contract != nil {

		type SinkList struct {
			name   string
			userID string
			id     string
		}

		var sinkList []SinkList

		// Volunteer Sink is only for fastrun contracts without a CRT
		if contract.Style&ContractFlagFastrun != 0 && contract.Style&ContractFlagCrt == 0 {
			sinkList = append(sinkList, SinkList{"Post Contract Sink", contract.Banker.PostSinkUserID, "postsink"})
		} else {
			if contract.State == ContractStateSignup {
				if contract.Style&ContractFlagCrt != 0 && contract.SRData.Legs > 0 {
					sinkList = append(sinkList, SinkList{"CRT Sink", contract.Banker.CrtSinkUserID, "crtsink"})
				}
				if contract.Style&ContractFlagBanker != 0 {
					sinkList = append(sinkList, SinkList{"Boost Sink", contract.Banker.BoostingSinkUserID, "boostsink"})
				}
			}
			sinkList = append(sinkList, SinkList{"Post Contract Sink", contract.Banker.PostSinkUserID, "postsink"})
		}

		var mComp []discordgo.MessageComponent
		for _, sink := range sinkList {
			buttonStyle := discordgo.SecondaryButton
			if sink.userID == "" {
				buttonStyle = discordgo.PrimaryButton
			}
			mComp = append(mComp, discordgo.Button{
				Emoji: &discordgo.ComponentEmoji{
					Name: "🫂",
				},
				Label:    sink.name,
				Style:    buttonStyle,
				CustomID: "cs_#" + sink.id + "#" + contract.ContractHash,
			})
		}

		if contract.State == ContractStateSignup && contract.Style&ContractFlagBanker != 0 {
			name := ""
			switch contract.Banker.SinkBoostPosition {
			case SinkBoostFirst:
				name = "Sink is FIRST"
			case SinkBoostLast:
				name = "Sink is LAST"
			case SinkBoostFollowOrder:
				name = "Sink will follow order"
			}

			mComp = append(mComp, discordgo.Button{
				Label:    name,
				Style:    discordgo.SecondaryButton,
				CustomID: "cs_#sinkorder#" + contract.ContractHash,
			})

		}

		buttons = append(buttons, discordgo.ActionsRow{Components: mComp})
	}

	return str, buttons
}
