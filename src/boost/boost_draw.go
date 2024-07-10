package boost

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
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
		contract.EggEmoji = FindEggEmoji(s, "485162044652388384", contract.EggName)
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
	}

	outputStr += fmt.Sprintf("> Coordinator: <@%s>\n", contract.CreatorID[0])
	if contract.Style&ContractStyleFastrun != 0 && contract.Banker.PostSinkUserID != "" {
		if contract.State != ContractStateSignup && contract.Boosters[contract.Banker.PostSinkUserID] != nil {
			outputStr += fmt.Sprintf("> Post Contract Sink: **%s**\n", contract.Boosters[contract.Banker.PostSinkUserID].Mention)
		}
	}

	switch contract.State {
	case ContractStateSignup:
		outputStr += contract.SRData.StatusStr

	case ContractStateCRT:
		//outputStr += fmt.Sprintf("> Send Tokens to <@%s>\n", contract.SRData.SpeedrunStarterUserID)

	case ContractStateBanker:
		afterListStr += fmt.Sprintf("\n**Send all tokens to %s**\n", contract.Boosters[contract.Banker.CurrentBanker].Mention)

	default:
	}

	if contract.State == ContractStateCRT {
		// Handle Speedrun CRT
		outputStr += drawSpeedrunCRT(contract)

		return outputStr
	}

	if contract.State == ContractStateSignup {
		outputStr += "## Sign-up List\n"
	} else {
		outputStr += "## Boost List\n"
	}

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

	showBoostedNums := 2 // Try to show at least 2 previously boosted
	windowSize := 10     // Number lines to show a single booster
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
			if time.Since(b.RunChickensTime) < 5*time.Minute {
				chickenStr = fmt.Sprintf(" - <t:%d:R>%s>", b.RunChickensTime.Unix(), contract.ChickenRunEmoji)
			}

			countStr, signupCountStr := getTokenCountString(tokenStr, b.TokensWanted, b.TokensReceived)
			if b.UserID == contract.Banker.CurrentBanker && b.BoostState == BoostStateUnboosted {
				countStr, signupCountStr = getTokenCountString(tokenStr, b.TokensWanted, 0)
			}

			sinkIcon := getSinkIcon(contract, b)
			if contract.State == ContractStateBanker {

				switch b.BoostState {
				case BoostStateUnboosted:
					outputStr += fmt.Sprintf("%s %s%s%s%s\n", prefix, name, signupCountStr, sinkIcon, server)
				case BoostStateTokenTime:
					outputStr += fmt.Sprintf("%s ➡️ **%s** %s%s%s%s\n", prefix, name, signupCountStr, currentStartTime, sinkIcon, server)
				case BoostStateBoosted:
					outputStr += fmt.Sprintf("%s ~~%s~~  %s %s%s%s\n", prefix, name, contract.Boosters[element].Duration.Round(time.Second), sinkIcon, chickenStr, server)
				}

			} else {

				switch b.BoostState {
				case BoostStateUnboosted:
					outputStr += fmt.Sprintf("%s %s%s%s\n", prefix, name, signupCountStr, server)
				case BoostStateTokenTime:
					if b.UserID == b.Name && b.AltController == "" && contract.State != ContractStateBanker {
						// Add a rocket for auto boosting
						outputStr += fmt.Sprintf("%s ➡️ **%s** 🚀%s%s%s\n", prefix, name, countStr, currentStartTime, server)
					} else {
						if !b.BoostingTokenTimestamp.IsZero() {
							currentStartTime = fmt.Sprintf(" <t:%d:R> since ️T-0️⃣ / votes:%d", b.BoostingTokenTimestamp.Unix(), len(b.VotingList))
						}
						outputStr += fmt.Sprintf("%s ➡️ **%s** %s%s%s\n", prefix, name, countStr, currentStartTime, server)
					}
				case BoostStateBoosted:
					outputStr += fmt.Sprintf("%s ~~%s~~  %s %s%s%s\n", prefix, name, contract.Boosters[element].Duration.Round(time.Second), sinkIcon, chickenStr, server)
				}
			}
		}
	}
	outputStr += lateList

	outputStr += afterListStr

	// Add reaction guidance to the bottom of this list
	switch contract.State {
	case ContractStateFastrun:
		outputStr += "\n"
		outputStr += "> Active Booster: " + boostIcon + " when boosting. \n"
		outputStr += "> Anyone: " + tokenStr + " when sending tokens "
		if len(contract.AltIcons) > 0 {
			outputStr += ", alts use 🇦-🇿"
		}
		outputStr += ". ❓ Help.\n"
		if contract.CoopSize != len(contract.Order) {
			outputStr += "> Use pinned message or add 🧑‍🌾 reaction to join this list and set boost " + tokenStr + " wanted.\n"
		}

	case ContractStateBanker:
		outputStr += "\n"
		outputStr += "> " + tokenStr + " when sending tokens to the sink"
		if len(contract.AltIcons) > 0 {
			outputStr += ", alts use 🇦-🇿"
		}
		outputStr += ".\n"
		outputStr += "> 🐓 when you're ready for others to run chickens on your farm.\n"
		outputStr += "> 💰 is used by the Sink to send the requested number of tokens to the booster.\n"
		outputStr += "> -When active Booster is sent tokens by the sink they are marked as boosted.\n"
		outputStr += "> -Adjust the number of boost tokens you want by adding a 6️⃣ to 🔟 reaction to the boost list message.\n"
		if contract.CoopSize != len(contract.Order) {
			outputStr += "> Use pinned message or add 🧑‍🌾 reaction to join this list and set boost " + tokenStr + " wanted.\n"
		}

	case ContractStateWaiting:
		outputStr += "\n"
		outputStr += "> Waiting for other(s) to join...\n"
		outputStr += "> Use pinned message or add 🧑‍🌾 reaction to join this list and set boost " + tokenStr + " wanted.\n"

	case ContractStateCompleted:
		t1 := contract.EndTime
		t2 := contract.StartTime
		duration := t1.Sub(t2)
		outputStr += "\n"
		outputStr += fmt.Sprintf("Contract boosting complete in %s!!\n", duration.Round(time.Second))

		sinkID := contract.Banker.CurrentBanker
		if sinkID != "" {
			outputStr += "> Send every " + tokenStr + " to our sink " + contract.Boosters[sinkID].Mention + "\n"
		}
	}

	return outputStr
}
