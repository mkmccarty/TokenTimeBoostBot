package boost

import (
	"fmt"
	"math"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

func getSinkIcon(contract *Contract, b *Booster) string {
	var sinkIcon = ""
	if contract.Banker.CurrentBanker == b.UserID {
		sinkIcon = fmt.Sprintf("%s[%d] %s", contract.TokenStr, b.TokensReceived, "🫂")
	}

	return sinkIcon
}

// DrawBoostList will draw the boost list for the contract
func DrawBoostList(s *discordgo.Session, contract *Contract) string {
	var outputStr string
	var afterListStr = ""
	tokenStr := contract.TokenStr

	contract.LastInteractionTime = time.Now()

	saveData(Contracts)
	if contract.EggEmoji == "" {
		contract.EggEmoji = FindEggEmoji(contract.EggName)
	}

	outputStr = fmt.Sprintf("## %s %s : [%s](%s/%s/%s)", contract.EggEmoji, contract.Name, contract.CoopID, "https://eicoop-carpet.netlify.app", contract.ContractID, contract.CoopID)
	if len(contract.Boosters) != contract.CoopSize {
		outputStr += fmt.Sprintf(" - %d/%d\n", len(contract.Boosters), contract.CoopSize)
	}
	outputStr += "\n"

	if contract.State == ContractStateSignup && contract.PlannedStartTime.After(time.Now()) && contract.PlannedStartTime.Before(time.Now().Add(7*24*time.Hour)) {
		outputStr += fmt.Sprintf("## Planned Start Time: <t:%d:f>\n", contract.PlannedStartTime.Unix())
	}

	if len(contract.Boosters) != contract.CoopSize || contract.State == ContractStateSignup {
		outputStr += fmt.Sprintf("### Boost ordering is %s\n", getBoostOrderString(contract))

		if contract.Style&ContractFlag6Tokens != 0 {
			outputStr += fmt.Sprintf(">  6️⃣%s boosting for everyone!\n", contract.TokenStr)
		} else if contract.Style&ContractFlag8Tokens != 0 {
			outputStr += fmt.Sprintf(">  8️⃣%s boosting for everyone!\n", contract.TokenStr)
		} else if contract.Style&ContractFlagDynamicTokens != 0 {
			outputStr += "> 🤖 Dynamic tokens (coming soon)\n"
		}
	}

	outputStr += fmt.Sprintf("> Coordinator: <@%s>\n", contract.CreatorID[0])
	if contract.Style&ContractStyleFastrun != 0 && contract.Banker.PostSinkUserID != "" {
		if contract.State != ContractStateSignup && contract.Boosters[contract.Banker.PostSinkUserID] != nil {
			outputStr += fmt.Sprintf("> Post Contract Sink: **%s**\n", contract.Boosters[contract.Banker.PostSinkUserID].Mention)
		}
	}

	if contract.State != ContractStateSignup && contract.State != ContractStateCompleted {
		/*
			(0.101332 * GG + 1/TokenTimer) * AllPlayers
			TokenTimer: Token Timer in minutes
			GG: 2 for all Generous Gifts, 1 + (UltraPlayers/AllPlayers) for Ultra GG, 1 for normal day
			UltraPlayers: Number of players in coop with ultra
			AllPlayers: size of coop

			0.144803 is rate with epic rainstick and habs full
		*/
		gg, ugg := ei.GetGenerousGiftEvent()
		ggicon := ""
		if gg > 1.0 {
			ggicon = " " + ei.GetBotEmojiMarkdown("std_gg")
		}
		if ugg > 1.0 && contract.UltraCount > 0 {
			// farmers with ultra
			gg = ugg + (float64(contract.UltraCount) / float64(contract.CoopSize))
			ggicon = " " + ei.GetBotEmojiMarkdown("ultra_gg")
		}

		timerTokensSinceStart := math.Floor(float64(time.Since(contract.StartTime).Minutes()) / float64(float64(contract.MinutesPerToken)))

		estTPM := (float64(0.101332)*gg + timerTokensSinceStart/time.Since(contract.StartTime).Minutes()) * float64(contract.CoopSize)
		// How many single token entries are in the log, excludes banker sent tokens
		singleTokenEntries := 0
		for _, logEntry := range contract.TokenLog {
			if logEntry.Quantity == 1 {
				singleTokenEntries++
			}
		}
		// Save this into the contract
		contract.TokensPerMinute = float64(singleTokenEntries) / time.Since(contract.StartTime).Minutes()
		outputStr += fmt.Sprintf("> %s/min: %2.2f   Expected %1.2f%s\n", contract.TokenStr, contract.TokensPerMinute, estTPM, ggicon)
	}

	// Current tval
	if contract.State != ContractStateSignup {
		if contract.EstimatedDuration == 0 {
			c := ei.EggIncContractsAll[contract.ContractID]
			if c.ID != "" {
				contract.EstimatedDuration = c.EstimatedDuration
			}
		}
		tval := getTokenValue(time.Since(contract.StartTime).Seconds(), contract.EstimatedDuration.Seconds())
		outputStr += fmt.Sprintf("> Current TVal: %2.3g\n", tval)
	}

	switch contract.State {
	case ContractStateSignup:
		outputStr += contract.SRData.StatusStr

	case ContractStateCRT:
		//outputStr += fmt.Sprintf("> Send Tokens to <@%s>\n", contract.SRData.SpeedrunStarterUserID)

	case ContractStateBanker:
		if contract.Banker.CurrentBanker == "" {
			if contract.Banker.PostSinkUserID != "" {
				contract.Banker.CurrentBanker = contract.Banker.PostSinkUserID
			}
		}
		if contract.Banker.CurrentBanker != "" {
			afterListStr += fmt.Sprintf("\n**Send all tokens to %s**\n", contract.Boosters[contract.Banker.CurrentBanker].Mention)
		}

	default:
	}

	if contract.State == ContractStateCRT {
		// Handle Speedrun CRT
		outputStr += drawSpeedrunCRT(contract)

		return outputStr
	}

	/*
		if contract.State == ContractStateSignup {
			outputStr += "## Sign-up List\n"
		} else {
			outputStr += "## Boost List\n"
		}
	*/
	var prefix = " - "

	earlyList := ""
	lateList := ""

	offset := 1

	// Some actions result in an unboosted farmer with the contract state still unset

	if contract.State == ContractStateWaiting {
		//set unboosted to true if any boosters are unboosted
		for _, element := range contract.Order {
			var b, ok = contract.Boosters[element]
			if ok {
				if b.BoostState == BoostStateUnboosted || b.BoostState == BoostStateTokenTime {
					if contract.Style&ContractFlagBanker != 0 {
						changeContractState(contract, ContractStateBanker)
					} else if contract.Style&ContractFlagFastrun != 0 {
						changeContractState(contract, ContractStateFastrun)
					} else {
						panic("Invalid contract style")
					}
					break
				}
			}
		}
	}

	showBoostedNums := 6 // Try to show at least 6 previously boosted
	windowSize := 10     // Number lines to show a single booster

	// If the contract has been completed for 15 minutes then just show the sink without the entire list
	if contract.State == ContractStateCompleted && time.Since(contract.EndTime) > 15*time.Minute {
		//outputStr += "## Boost\n"
		if contract.Banker.CurrentBanker == "" {
			outputStr += "\nNo volunteer sink for this contract, hold your tokens.\n"
		} else {
			b := contract.Boosters[contract.Banker.CurrentBanker]
			var name = b.Mention
			var einame = farmerstate.GetEggIncName(b.UserID)
			if einame != "" {
				name += " " + einame
			}

			sinkIcon := getSinkIcon(contract, b)
			outputStr += fmt.Sprintf("\n%s  %s\n", name, sinkIcon)
		}
	} else {
		if contract.State == ContractStateSignup {
			outputStr += "## Sign-up List\n"
		} else {
			outputStr += "## Boost List\n"
		}

		orderSubset := contract.Order
		if contract.State != ContractStateSignup && len(contract.Order) >= (windowSize+2) {
			// extract 10 elements around the current booster
			var start = contract.BoostPosition - showBoostedNums
			var end = contract.BoostPosition + (windowSize - showBoostedNums)

			if start < 0 {
				// add the aboslute value of start to end
				end += -start
				start = 0
			}
			if end > len(contract.Order) {
				start -= end - len(contract.Order)
				end = len(contract.Order)
			}
			// populate earlyList with all elements from earlySubset
			for i, element := range contract.Order[0:start] {
				var b, ok = contract.Boosters[element]
				if ok {
					sinkIcon := getSinkIcon(contract, b)

					if b.BoostState == BoostStateBoosted {
						earlyList += fmt.Sprintf("~~%s~~%s ", b.Mention, sinkIcon)
					} else {
						earlyList += fmt.Sprintf("%s(%d)%s ", b.Mention, b.TokensWanted, sinkIcon)
					}
					if i < start-1 {
						earlyList += ", "
					}
				}
			}
			if earlyList != "" {
				if start == 1 {
					earlyList = fmt.Sprintf("1: %s\n", earlyList)
				} else {
					earlyList = fmt.Sprintf("1-%d: %s\n", start, earlyList)
				}
			}

			for i, element := range contract.Order[end:len(contract.Order)] {
				var b, ok = contract.Boosters[element]
				if ok {
					sinkIcon := getSinkIcon(contract, b)

					if b.BoostState == BoostStateBoosted {
						lateList += fmt.Sprintf("~~%s~~%s ", b.Mention, sinkIcon)
					} else {
						lateList += fmt.Sprintf("%s(%d)%s ", b.Mention, b.TokensWanted, sinkIcon)
					}
					if (end + i + 1) < len(contract.Boosters) {
						lateList += ", "
					}
				}
			}
			if lateList != "" {
				if (end + 1) == len(contract.Order) {
					lateList = fmt.Sprintf("%d: %s", end+1, lateList)
				} else {
					lateList = fmt.Sprintf("%d-%d: %s", end+1, len(contract.Order), lateList)
				}
			}

			orderSubset = contract.Order[start:end]
			offset = start + 1
		}

		outputStr += earlyList

		for i, element := range orderSubset {

			if contract.State != ContractStateSignup {
				prefix = fmt.Sprintf("%2d - ", i+offset)
			}
			var b, ok = contract.Boosters[element]
			if ok {
				var name = b.Mention
				var einame = farmerstate.GetEggIncName(b.UserID)
				if einame != "" {
					name += " " + einame
				}
				var server = ""
				var currentStartTime = fmt.Sprintf(" <t:%d:R> ", b.StartTime.Unix())
				if len(contract.Location) > 1 {
					server = fmt.Sprintf(" (%s) ", b.GuildName)
				}
				var chickenStr = ""
				//if time.Since(b.RunChickensTime) < 10*time.Minute {
				if !b.RunChickensTime.IsZero() {
					chickenStr = fmt.Sprintf(" - <t:%d:R>%s", b.RunChickensTime.Unix(), ei.GetBotEmojiMarkdown("icon_chicken_run"))
				}

				countStr, signupCountStr := getTokenCountString(tokenStr, b.TokensWanted, b.TokensReceived)
				if b.UserID == contract.Banker.CurrentBanker && b.BoostState == BoostStateUnboosted {
					countStr, signupCountStr = getTokenCountString(tokenStr, b.TokensWanted, 0)
				}

				// Additions for contract state value display
				sortRate := ""
				if contract.State == ContractStateSignup && contract.BoostOrder == ContractOrderELR {
					sortRate = fmt.Sprintf(" **ELR:%2.3f** ", min(b.ArtifactSet.LayRate, b.ArtifactSet.ShipRate))
				}
				if (contract.State == ContractStateBanker || contract.State == ContractStateFastrun) && contract.BoostOrder == ContractOrderTVal {
					sortRate = fmt.Sprintf(" *∆:%2.3f* ", b.TokenValue)
				}

				sinkIcon := getSinkIcon(contract, b)
				if contract.State == ContractStateBanker {

					switch b.BoostState {
					case BoostStateUnboosted:
						outputStr += fmt.Sprintf("%s %s%s%s%s%s\n", prefix, name, signupCountStr, sortRate, sinkIcon, server)
					case BoostStateTokenTime:
						outputStr += fmt.Sprintf("%s ➡️ **%s** %s%s%s%s%s\n", prefix, name, signupCountStr, sortRate, currentStartTime, sinkIcon, server)
					case BoostStateBoosted:
						boostingString := ""
						if time.Now().Before(b.EstEndOfBoost) {
							diamond, _, _ := ei.GetBotEmoji("trophy_diamond")
							habFull, _, _ := ei.GetBotEmoji("hab_full")
							if b.RunChickensTime.IsZero() {
								boostingString = fmt.Sprintf(" %s<t:%d:R> / ", diamond, b.EstRequestChickenRuns.Unix())
							}
							boostingString += fmt.Sprintf(" %s<t:%d:R>", habFull, b.EstEndOfBoost.Unix())
						}
						outputStr += fmt.Sprintf("%s ~~%s~~  %s %s%s%s%s\n", prefix, name, contract.Boosters[element].Duration.Round(time.Second), sinkIcon, boostingString, chickenStr, server)
					}

				} else {

					switch b.BoostState {
					case BoostStateUnboosted:
						outputStr += fmt.Sprintf("%s %s%s%s%s\n", prefix, name, signupCountStr, sortRate, server)
					case BoostStateTokenTime:
						if b.UserID == b.Name && b.AltController == "" && contract.State != ContractStateBanker {
							// Add a rocket for auto boosting
							outputStr += fmt.Sprintf("%s ➡️ **%s** 🚀%s%s%s%s\n", prefix, name, countStr, sortRate, currentStartTime, server)
						} else {
							if !b.BoostingTokenTimestamp.IsZero() {
								currentStartTime = fmt.Sprintf(" <t:%d:R> since ️T-0️⃣ / votes:%d", b.BoostingTokenTimestamp.Unix(), len(b.VotingList))
							}
							outputStr += fmt.Sprintf("%s ➡️ **%s** %s%s%s%s\n", prefix, name, countStr, sortRate, currentStartTime, server)
						}
					case BoostStateBoosted:
						boostingString := ""
						if time.Now().Before(b.EstEndOfBoost) {
							diamond, _, _ := ei.GetBotEmoji("trophy_diamond")
							habFull, _, _ := ei.GetBotEmoji("hab_full")
							if b.RunChickensTime.IsZero() {
								boostingString = fmt.Sprintf(" %s<t:%d:R> / ", diamond, b.EstRequestChickenRuns.Unix())
							}
							boostingString += fmt.Sprintf(" %s<t:%d:R>", habFull, b.EstEndOfBoost.Unix())
						}
						outputStr += fmt.Sprintf("%s ~~%s~~  %s %s%s%s%s\n", prefix, name, contract.Boosters[element].Duration.Round(time.Second), sinkIcon, boostingString, chickenStr, server)
					}
				}
			}
		}
		outputStr += lateList

		outputStr += afterListStr
	}

	guidanceStr := ""
	// Add reaction guidance to the bottom of this list
	switch contract.State {
	case ContractStateFastrun:
		guidanceStr += "\n"
		guidanceStr += "> Active Booster: " + boostIcon + " when boosting. \n"
		guidanceStr += "> Anyone: " + tokenStr + " when sending tokens "
		if len(contract.AltIcons) > 0 {
			guidanceStr += ", alts use 🇦-🇿"
		}
		guidanceStr += ". ❓ Help.\n"
		if contract.CoopSize != len(contract.Order) {
			guidanceStr += "> Use pinned message or add 🧑‍🌾 reaction to join this list and set boost " + tokenStr + " wanted.\n"
		}
		if len(outputStr)+len(guidanceStr) < 1900 {
			outputStr += guidanceStr
		}

	case ContractStateBanker:
		guidanceStr += "\n"
		guidanceStr += "> " + tokenStr + " when sending tokens to the sink"
		if len(contract.AltIcons) > 0 {
			guidanceStr += ", alts use 🇦-🇿"
		}
		runReady, _, _ := ei.GetBotEmoji("runready")

		guidanceStr += ".\n"
		guidanceStr += "> " + runReady + " when you're ready for others to run chickens on your farm.\n"
		guidanceStr += "> 💰 is used by the Sink to send the requested number of tokens to the booster.\n"
		guidanceStr += "> -When active Booster is sent tokens by the sink they are marked as boosted.\n"
		guidanceStr += "> -Adjust the number of boost tokens you want by adding a 6️⃣ to 🔟 reaction to the boost list message.\n"
		if contract.CoopSize != len(contract.Order) {
			guidanceStr += "> Use pinned message or add 🧑‍🌾 reaction to join this list and set boost " + tokenStr + " wanted.\n"
		}
		if len(outputStr)+len(guidanceStr) < 1900 {
			outputStr += guidanceStr
		}

	case ContractStateWaiting:
		guidanceStr += "\n"
		guidanceStr += "> Waiting for other(s) to join...\n"
		guidanceStr += "> Use pinned message or add 🧑‍🌾 reaction to join this list and set boost " + tokenStr + " wanted.\n"
		if len(outputStr)+len(guidanceStr) < 1900 {
			outputStr += guidanceStr
		}

	case ContractStateCompleted:
		if time.Since(contract.EndTime) > 15*time.Minute {
			outputStr += "\n## Post Boost Tools\n"
			outputStr += fmt.Sprintf("> **Boost Bot:** %s %s %s\n", bottools.GetFormattedCommand("stones"), bottools.GetFormattedCommand("calc-contract-tval"), bottools.GetFormattedCommand("coop-tval"))
			outputStr += "> **Wonky:** </auditcoop:1231383614701174814> </optimizestones:1235003878886342707> </srtracker:1158969351702069328>\n"
			outputStr += fmt.Sprintf("> **Web:** \n> * [%s](%s)\n> * [%s](%s)\n",
				"Staabmia Stone Calc", "https://srsandbox-staabmia.netlify.app/stone-calc",
				"Kaylier Coop Laying Assistant", "https://ei-coop-assistant.netlify.app/laying-set")
		}

		t1 := contract.EndTime
		t2 := contract.StartTime
		duration := t1.Sub(t2)
		outputStr += "\n"
		outputStr += fmt.Sprintf("Contract boosting complete in %s with a rate of %2.3g %s/min\n", duration.Round(time.Second), contract.TokensPerMinute, contract.TokenStr)

		sinkID := contract.Banker.CurrentBanker
		if sinkID != "" {
			outputStr += "> Send every " + tokenStr + " to our sink " + contract.Boosters[sinkID].Mention + "\n"
		}
	}

	return outputStr
}
