package boost

import (
	"fmt"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"

	"github.com/bwmarrin/discordgo"
)

func getSignupContractSettings(channelID string, id string, thread bool) (string, []discordgo.MessageComponent) {
	minValues := 1
	minZeroValues := 0

	var builder strings.Builder
	fmt.Fprintf(&builder, "Contract created in <#%s>\n", channelID)
	builder.WriteString("Use the Contract button if you have to recycle it.\n")
	builder.WriteString("**Use the menus to set your contract style. These will work until the contract is started.**\n")
	fmt.Fprintf(&builder, "If this contract isn't an immediate start use %s or %s to add the time to the sign-up message.\n",
		bottools.GetFormattedCommand("change-start offset"),
		bottools.GetFormattedCommand("change-start timestamp"))
	if thread {
		builder.WriteString("React with 🌊 on the bost list to automaticaly update the thread name (`/rename-thread`).\n")
	} else {
		builder.WriteString("This contract is in a channel and it cannot be renamed. Create it in a thread to permit renaming.\n")
	}
	builder.WriteString("React with ⏱️ after the boosting is completed to update the duration from the EI API.")

	contract := Contracts[id]

	// Dynamic Boost List Styles
	runStyleOptions := []discordgo.SelectMenuOption{}
	runStyleOptions = append(runStyleOptions, discordgo.SelectMenuOption{
		Label:       "Boost List Style",
		Description: "Everyone sends tokens to the current booster",
		Value:       "boostlist",
		Default:     (contract.Style & ContractFlagFastrun) != 0,
		Emoji: &discordgo.ComponentEmoji{
			Name: "📜",
		},
	})

	runStyleOptions = append(runStyleOptions, discordgo.SelectMenuOption{
		Label:       "Banker Style",
		Description: "Everyone sends tokens to a banker.",
		Value:       "banker",
		Default:     (contract.Style & ContractFlagBanker) != 0,
		Emoji: &discordgo.ComponentEmoji{
			Name: "💰",
		},
	})
	playstyleOptions := []discordgo.SelectMenuOption{}

	playstyleOptions = append(playstyleOptions, discordgo.SelectMenuOption{
		Label:       "Chill play style",
		Description: "Everyone fills habs and uses correct artifacts",
		Value:       "chill",
		Default:     (contract.PlayStyle == ContractPlaystyleChill),
		Emoji:       ei.GetBotComponentEmoji("chill"),
	})
	playstyleOptions = append(playstyleOptions, discordgo.SelectMenuOption{
		Label:       "ACO Cooperative play style",
		Description: "Chill + Everyone checks in on time",
		Value:       "aco",
		Default:     (contract.PlayStyle == ContractPlaystyleACOCooperative),
		Emoji:       ei.GetBotComponentEmoji("aco"),
	})
	playstyleOptions = append(playstyleOptions, discordgo.SelectMenuOption{
		Label:       "Fastrun",
		Description: "ACO + Get TVal and CR from your coop size or act as sink",
		Value:       "fastrun",
		Default:     (contract.PlayStyle == ContractPlaystyleFastrun),
		Emoji:       ei.GetBotComponentEmoji("fastrun"),
	})
	playstyleOptions = append(playstyleOptions, discordgo.SelectMenuOption{
		Label:       "Leaderboard",
		Description: "Banker Run + TBD",
		Value:       "leaderboard",
		Default:     (contract.PlayStyle == ContractPlaystyleLeaderboard),
		Emoji:       ei.GetBotComponentEmoji("leaderboard"),
	})

	return builder.String(), []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "cs_#style#" + id,
					Placeholder: "Select contract styles",
					MinValues:   &minValues,
					MaxValues:   1,
					Options:     runStyleOptions,
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
							Emoji:       ei.GetBotComponentEmoji("signup"),
							Default:     contract.BoostOrder == ContractOrderSignup,
						},
						{
							Label:       "Token Value Order",
							Description: "Highest token value boosts earlier",
							Value:       "tval",
							Emoji:       ei.GetBotComponentEmoji("sharing"),
							Default:     contract.BoostOrder == ContractOrderTVal,
						},
						{
							Label:       "ELR Order",
							Description: "Highest Egg Lay Rate first",
							Value:       "elr",
							Emoji:       ei.GetBotComponentEmoji("elr"),
							Default:     contract.BoostOrder == ContractOrderELR,
						},
						/*
							{
								Label:       "Token Ask Order",
								Description: "Those asking for less tokens boost earlier",
								Value:       "ask",
								Emoji:       ei.GetBotComponentEmoji("ask"),
								Default:     contract.BoostOrder == ContractOrderTokenAsk,
							},
						*/
						{
							Label:       "Random Order",
							Description: "Boost order is random",
							Value:       "random",
							Emoji:       ei.GetBotComponentEmoji("random"),
							Default:     contract.BoostOrder == ContractOrderRandom,
						},
						{
							Label:       "Reverse Sign-up Order",
							Description: "Boost list is in the reverse order farmers sign up",
							Value:       "reverse",
							Emoji:       ei.GetBotComponentEmoji("reverse"),
							Default:     contract.BoostOrder == ContractOrderReverse,
						},
					},
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "cs_#play#" + id,
					Placeholder: "Choose your play style",
					MinValues:   &minValues,
					MaxValues:   1,
					Options:     playstyleOptions,
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "cs_#features#" + id,
					Placeholder: "Optional Features",
					MinValues:   &minZeroValues,
					MaxValues:   1,
					Options: []discordgo.SelectMenuOption{
						{
							Label:       "6 token boosts",
							Description: "Everyone joins wanting 6 token boosts",
							Value:       "boost6",
							Default:     (contract.Style & ContractFlag6Tokens) != 0,
							Emoji: &discordgo.ComponentEmoji{
								Name: "6️⃣",
							},
						},
						{
							Label:       "8 token boosts",
							Description: "Everyone joins wanting 8 token boosts",
							Value:       "boost8",
							Default:     (contract.Style & ContractFlag8Tokens) != 0,
							Emoji: &discordgo.ComponentEmoji{
								Name: "8️⃣",
							},
						},
						{
							Label:       "Dynamic Boost Tokens",
							Description: "Based on highest 120min delivery rate",
							Value:       "dynamic",
							Default:     (contract.Style & ContractFlagDynamicTokens) != 0,
							Emoji: &discordgo.ComponentEmoji{
								Name: "🤖",
							},
						},
					},
				},
			},
		},
	}

}

// GetSignupComponents returns the signup components for a contract
func GetSignupComponents(contract *Contract) (string, []discordgo.MessageComponent) {
	if contract == nil {
		return "", []discordgo.MessageComponent{}
	}

	disableStartContract := false
	var str = "Join the contract and indicate the number boost tokens you'd like."
	if contract.State == ContractStateSignup && contract.Style&ContractFlagBanker != 0 {
		str += "\nThe Sink boost position button cycles from First->Last->Follow Order."
	}
	startLabel := "Start Boost List"
	if contract.State != ContractStateSignup {
		startLabel = "Started"
	}

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
	bankerStyle := (contract.Style & ContractFlagBanker) != 0
	if bankerStyle && contract.Banker.BoostingSinkUserID == "" {
		disableStartContract = true
	}
	if contract.State != ContractStateSignup {
		disableStartContract = true
	}

	joinMsg := "Join"
	if len(contract.Boosters) == contract.CoopSize {
		joinMsg = "Join (Backup)"
	}

	// Build the return message
	var buttons []discordgo.MessageComponent
	// Add the buttons to join, leave, and start the contract
	buttons = append(buttons, discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{
				Emoji:    ei.GetBotComponentEmoji("clucker"),
				Label:    joinMsg,
				Style:    discordgo.PrimaryButton,
				CustomID: "fd_signupFarmer",
			},
			/*
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "🔔",
					},
					Label:    "Join w/Ping",
					Style:    discordgo.PrimaryButton,
					CustomID: "fd_signupBell",
				},
			*/
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

	type SinkList struct {
		name   string
		emote  string
		userID string
		id     string
	}

	var sinkList []SinkList
	if config.IsDevBot() && contract.PlayStyle == ContractPlaystyleLeaderboard {
		sinkList = append(sinkList, SinkList{"Parade Banker", "🎪", contract.Banker.ParadeSinkUserID, "paradesink"})
		sinkList = append(sinkList, SinkList{"Parade Host", "🤹", "", "paradehost"})
	}
	if (contract.State == ContractStateSignup || contract.State == ContractStateBanker) && contract.Style&ContractFlagBanker != 0 {
		sinkList = append(sinkList, SinkList{"Banker", "🏦", contract.Banker.BoostingSinkUserID, "boostsink"})
	}
	if contract.SeasonalScoring != 1 {
		// New contracts fom 9/22/2025 on don't have token value requirements
		sinkList = append(sinkList, SinkList{"Post Contract Sink", "🏁", contract.Banker.PostSinkUserID, "postsink"})
	}

	var mComp []discordgo.MessageComponent
	for _, sink := range sinkList {
		buttonStyle := discordgo.SecondaryButton
		if sink.userID == "" {
			buttonStyle = discordgo.PrimaryButton
		}
		mComp = append(mComp, discordgo.Button{
			Emoji: &discordgo.ComponentEmoji{
				Name: sink.emote,
			},
			Label:    sink.name,
			Style:    buttonStyle,
			CustomID: "cs_#" + sink.id + "#" + contract.ContractHash,
		})
	}

	if (contract.State == ContractStateSignup || contract.State == ContractStateBanker) && contract.Style&ContractFlagBanker != 0 {
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

		if config.IsDevBot() && contract.PlayStyle == ContractPlaystyleLeaderboard {
			runsNeeded := contract.ChickenRuns - (contract.CoopSize - 1)
			// If any of the contract Boosters are Parade Kind then show the parade join button
			if contract.Banker.ParadeSinkUserID != "" {
				mComp = append(mComp, discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "🤡",
					},
					Label:    fmt.Sprintf("Need %d Parade Alts", runsNeeded),
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_paradeJoin",
				})
			}
		}
	}

	if len(mComp) > 0 {
		buttons = append(buttons, discordgo.ActionsRow{Components: mComp})
	}

	return str, buttons
}
