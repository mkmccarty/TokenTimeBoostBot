package boost

import (
	"fmt"
	"log"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

func getSignupContractSettings(channelID string, hashID string, thread bool) (string, []dc.LayoutComponent) {
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
		builder.WriteString("React with 🌊 on the boost list to automaticaly update the thread name (`/rename-thread`).\n")
	} else {
		builder.WriteString("This contract is in a channel and it cannot be renamed. Create it in a thread to permit renaming.\n")
	}
	builder.WriteString("React with ⏱️ after the boosting is completed to update the duration from the EI API.")

	contract := FindContractByHash(hashID)
	if contract == nil {
		return "", nil
	}

	// Dynamic Boost List Styles
	runStyleOptions := []dc.SelectOption{}
	runStyleOptions = append(runStyleOptions, dc.SelectOption{
		Label:       "Boost List Style",
		Description: "Everyone sends tokens to the current booster",
		Value:       "boostlist",
		Default:     (contract.Style & ContractFlagFastrun) != 0,
		Emoji: &dc.Emoji{
			Name: "📜",
		},
	})

	runStyleOptions = append(runStyleOptions, dc.SelectOption{
		Label:       "Banker Style",
		Description: "Everyone sends tokens to a banker.",
		Value:       "banker",
		Default:     (contract.Style & ContractFlagBanker) != 0,
		Emoji: &dc.Emoji{
			Name: "💰",
		},
	})
	playstyleOptions := []dc.SelectOption{}

	playstyleOptions = append(playstyleOptions, dc.SelectOption{
		Label:       "Chill play style",
		Description: "Everyone fills habs and uses correct artifacts",
		Value:       "chill",
		Default:     (contract.PlayStyle == ContractPlaystyleChill),
		Emoji:       ei.GetBotComponentEmoji("chill"),
	})
	playstyleOptions = append(playstyleOptions, dc.SelectOption{
		Label:       "ACO Cooperative play style",
		Description: "Chill + Everyone checks in on time",
		Value:       "aco",
		Default:     (contract.PlayStyle == ContractPlaystyleACOCooperative),
		Emoji:       ei.GetBotComponentEmoji("aco"),
	})
	playstyleOptions = append(playstyleOptions, dc.SelectOption{
		Label:       "Fastrun",
		Description: "ACO + Get TVal and CR from your coop size or act as sink",
		Value:       "fastrun",
		Default:     (contract.PlayStyle == ContractPlaystyleFastrun),
		Emoji:       ei.GetBotComponentEmoji("fastrun"),
	})
	playstyleOptions = append(playstyleOptions, dc.SelectOption{
		Label:       "Leaderboard",
		Description: "Banker Run + TBD",
		Value:       "leaderboard",
		Default:     (contract.PlayStyle == ContractPlaystyleLeaderboard),
		Emoji:       ei.GetBotComponentEmoji("leaderboard"),
	})

	featuresOptions := []dc.SelectOption{
		{
			Label:       "4 token boosts",
			Description: "Everyone joins wanting 4 token boosts",
			Value:       "boost4",
			Default:     (contract.Style & ContractFlag4Tokens) != 0,
			Emoji: &dc.Emoji{
				Name: "4️⃣",
			},
		},
		{
			Label:       "6 token boosts",
			Description: "Everyone joins wanting 6 token boosts",
			Value:       "boost6",
			Default:     (contract.Style & ContractFlag6Tokens) != 0,
			Emoji: &dc.Emoji{
				Name: "6️⃣",
			},
		},
		{
			Label:       "8 token boosts",
			Description: "Everyone joins wanting 8 token boosts",
			Value:       "boost8",
			Default:     (contract.Style & ContractFlag8Tokens) != 0,
			Emoji: &dc.Emoji{
				Name: "8️⃣",
			},
		},
		{
			Label:       "Dynamic Boost Tokens",
			Description: "Based on highest 120min delivery rate",
			Value:       "dynamic",
			Default:     (contract.Style & ContractFlagDynamicTokens) != 0,
			Emoji: &dc.Emoji{
				Name: "🤖",
			},
		},
		{
			Label:       "Threshold Boost Tokens",
			Description: "X tokens for >= TE, Y tokens < TE",
			Value:       "threshold",
			Default:     (contract.Style & ContractFlagThresholdTokens) != 0,
			Emoji: &dc.Emoji{
				Name: "📊",
			},
		},
	}

	hasAMQP := false
	if len(contract.Location) > 0 {
		guildID := contract.Location[0].GuildID
		if guildID != "" && guildstate.GetGuildSettingString(guildID, "amqp_url") != "" {
			hasAMQP = true
		}
	}

	if hasAMQP {
		featuresOptions = append(featuresOptions, dc.SelectOption{
			Label:       "AMQP Publish",
			Description: "Send token logs and boost status to AMQP queue",
			Value:       "amqp",
			Default:     (contract.Style & ContractFlagAMQP) != 0,
			Emoji: &dc.Emoji{
				Name: "📣",
			},
		})
	}

	return builder.String(), []dc.LayoutComponent{
		dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.SelectMenu{
					CustomID:    "cs_#style#" + hashID,
					Placeholder: "Select contract styles",
					MinValues:   &minValues,
					MaxValues:   1,
					Options:     runStyleOptions,
				},
			},
		},
		dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.SelectMenu{
					CustomID:    "cs_#order#" + hashID,
					Placeholder: "Select the boosting order for this contract",
					MinValues:   &minValues,
					MaxValues:   1,
					Options: []dc.SelectOption{
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
						{
							Label:       "Token Ask Order",
							Description: "Those asking for less tokens boost earlier",
							Value:       "ask",
							Emoji:       ei.GetBotComponentEmoji("ask"),
							Default:     contract.BoostOrder == ContractOrderTokenAsk,
						},
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
						{
							Label:       "TE Order",
							Description: "Highest Truth Egg count first",
							Value:       "te",
							Emoji:       ei.GetBotComponentEmoji("egg_truth"),
							Default:     contract.BoostOrder == ContractOrderTE,
						},
						{
							Label:       "Fuzzy TE Order",
							Description: "Highest Truth Egg count first with randomization",
							Value:       "fuzzyte",
							Emoji:       ei.GetBotComponentEmoji("egg_truth"),
							Default:     contract.BoostOrder == ContractOrderTEFuzzy,
						},
						{
							Label:       "Boosting IHR Order",
							Description: "Highest Boosting IHR first",
							Value:       "ihr",
							Emoji:       ei.GetBotComponentEmoji("chalice_T4L"),
							Default:     contract.BoostOrder == ContractOrderIHR,
						},
						{
							Label:       "Fuzzy Boosting IHR Order",
							Description: "Highest Boosting IHR first with randomization",
							Value:       "fuzzyihr",
							Emoji:       ei.GetBotComponentEmoji("chalice_T4L"),
							Default:     contract.BoostOrder == ContractOrderIHRFuzzy,
						},
					},
				},
			},
		},
		dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.SelectMenu{
					CustomID:    "cs_#play#" + hashID,
					Placeholder: "Choose your play style",
					MinValues:   &minValues,
					MaxValues:   1,
					Options:     playstyleOptions,
				},
			},
		},
		dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.SelectMenu{
					CustomID:    "cs_#features#" + hashID,
					Placeholder: "Optional Features",
					MinValues:   &minZeroValues,
					MaxValues:   1,
					Options:     featuresOptions,
				},
			},
		},
	}

}

// GetSignupComponents returns the signup components for a contract
func GetSignupComponents(contract *Contract) (string, []dc.LayoutComponent) {
	if contract == nil {
		return "", []dc.LayoutComponent{}
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
	if isTBDCoopID(contract.CoopID) {
		disableStartContract = true
	}

	joinMsg := "Join"
	if len(contract.Boosters) == contract.CoopSize {
		joinMsg = "Join (Backup)"
	}

	// Build the return message
	var buttons []dc.LayoutComponent
	// Add the buttons to join, leave, and start the contract
	buttons = append(buttons, dc.ActionRow{
		Components: []dc.InteractiveComponent{
			dc.Button{
				Emoji:    ei.GetBotComponentEmoji("clucker"),
				Label:    joinMsg,
				Style:    dc.ButtonPrimary,
				CustomID: "fd_signupFarmer",
			},
			/*
				dc.Button{
					Emoji: &dc.Emoji{
						Name: "🔔",
					},
					Label:    "Join w/Ping",
					Style:    dc.ButtonPrimary,
					CustomID: "fd_signupBell",
				},
			*/
			dc.Button{
				Emoji: &dc.Emoji{
					Name: "❌",
				},
				Label:    "Leave",
				Style:    dc.ButtonSecondary,
				CustomID: "fd_signupLeave",
			},
			dc.Button{
				Emoji: &dc.Emoji{
					Name: "⏱️",
				},
				Label:    startLabel,
				Style:    dc.ButtonSuccess,
				CustomID: "fd_signupStart",
				Disabled: disableStartContract,
			},
			dc.Button{
				Emoji: &dc.Emoji{
					Name: "♻️",
				},
				Label:    "Contract",
				Style:    dc.ButtonDanger,
				Disabled: false,
				CustomID: "fd_delete",
			},
			dc.Button{
				Emoji: &dc.Emoji{
					Name: "🔄",
				},
				Label:    "Restart",
				Style:    dc.ButtonDanger,
				Disabled: false,
				CustomID: "fd_restart",
			},
		},
	})

	// Add the buttons to adjust the numbers of tokens
	buttons = append(buttons, dc.ActionRow{
		Components: []dc.InteractiveComponent{
			dc.Button{
				Emoji: &dc.Emoji{
					Name: "4️⃣",
				},
				Label:    " Tokens",
				Style:    dc.ButtonSecondary,
				CustomID: "fd_tokens4",
			},
			dc.Button{
				Emoji: &dc.Emoji{
					Name: "5️⃣",
				},
				Label:    " Tokens",
				Style:    dc.ButtonSecondary,
				CustomID: "fd_tokens5",
			},
			dc.Button{
				Emoji: &dc.Emoji{
					Name: "6️⃣",
				},
				Label:    " Tokens",
				Style:    dc.ButtonSecondary,
				CustomID: "fd_tokens6",
			},
			dc.Button{
				Label:    "+ Token",
				Style:    dc.ButtonSecondary,
				CustomID: "fd_tokens1",
			},
			dc.Button{
				Label:    "- Token",
				Style:    dc.ButtonSecondary,
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
	if (contract.State == ContractStateSignup || contract.State == ContractStateBanker) && contract.Style&ContractFlagBanker != 0 {
		sinkList = append(sinkList, SinkList{"Banker", "🏦", contract.Banker.BoostingSinkUserID, "boostsink"})
	}
	if contract.SeasonalScoring == ei.SeasonalScoringStandard {
		// New contracts fom 9/22/2025 on don't have token value requirements
		sinkList = append(sinkList, SinkList{"Sink", "🏁", contract.Banker.PostSinkUserID, "postsink"})
	}

	var mComp []dc.InteractiveComponent
	for _, sink := range sinkList {
		buttonStyle := dc.ButtonSecondary
		if sink.userID == "" {
			buttonStyle = dc.ButtonPrimary
		}
		mComp = append(mComp, dc.Button{
			Emoji: &dc.Emoji{
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

		mComp = append(mComp, dc.Button{
			Label:    name,
			Style:    dc.ButtonSecondary,
			CustomID: "cs_#sinkorder#" + contract.ContractHash,
		})
	}

	if len(mComp) > 0 {
		buttons = append(buttons, dc.ActionRow{Components: mComp})
	}

	return str, buttons
}

func joinContract(client dc.Client, e *dc.ComponentEvent, bell bool) {
	_ = e.DeferUpdate()

	userID := e.UserID()

	if err := JoinContract(client, e.GuildID(), e.ChannelID(), userID, bell); err != nil {
		log.Print(err.Error())
	}

	_ = e.Followup(dc.Message{})
}

// HandleSignupStart handles the interaction for starting the signup process.
//
// It still takes a raw session because StartContractBoosting and the signup
// message rebuild are not on the facade yet.
func HandleSignupStart(client dc.Client, e *dc.ComponentEvent) {
	_ = e.DeferUpdate()
	err := StartContractBoosting(client, e.GuildID(), e.ChannelID(), e.UserID())
	if err != nil {
		str := fmt.Sprint(err.Error())
		_ = e.Followup(dc.Message{
			Content:   str,
			Ephemeral: true,
		})
	} else {
		_ = e.Followup(dc.Message{})

		contract := FindContract(e.ChannelID())
		// Rebuild the signup message to disable the start button
		contentStr, comp := GetSignupComponents(contract) // True to get a disabled start button
		_, _ = client.EditMessage(e.ChannelID(), e.MessageID(), dc.Message{
			Components: append([]dc.LayoutComponent{dc.TextDisplay{Content: contentStr}}, comp...),
		})
	}
}

// HandleSignupFarmer handles the interaction for joining a contract as a farmer
func HandleSignupFarmer(client dc.Client, e *dc.ComponentEvent) {
	joinContract(client, e, false)
}

// HandleSignupBell handles the interaction for joining a contract with a bell
func HandleSignupBell(client dc.Client, e *dc.ComponentEvent) {
	joinContract(client, e, true)
}

// HandleSignupLeave handles the interaction for leaving a contract.
//
// It still takes a raw session because RemoveFarmerByMention is not on the
// facade yet.
func HandleSignupLeave(client dc.Client, e *dc.ComponentEvent) {
	str := "Removed from Contract"
	_ = e.Defer(true)

	mention := "<@" + e.UserID() + ">"
	var err = RemoveFarmerByMention(client, e.GuildID(), e.ChannelID(), mention, mention)
	if err != nil {
		str = err.Error()
	}

	_ = e.Followup(dc.Message{
		Content:   str,
		Ephemeral: true,
	})
}
