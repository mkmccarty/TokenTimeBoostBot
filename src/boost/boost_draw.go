package boost

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

// DrawBoostList will draw the boost list for the contract
func DrawBoostList(s *discordgo.Session, contract *Contract, tokenStr string) string {
	var outputStr = ""
	var afterListStr = ""
	saveData(Contracts)
	if contract.EggEmoji == "" {
		contract.EggEmoji = FindEggEmoji(s, "485162044652388384", contract.EggName)
	}

	if len(contract.Boosters) == contract.CoopSize {
		// Full contract, don't need extra details
		outputStr = fmt.Sprintf("## %s %s (%s)\n", contract.EggEmoji, contract.Name, contract.CoopID)
	} else {
		outputStr = fmt.Sprintf("## %s %s (%s) - 📋%s - %d/%d\n", contract.EggEmoji, contract.Name, contract.CoopID, getBoostOrderString(contract), len(contract.Boosters), contract.CoopSize)
	}

	outputStr += fmt.Sprintf("> Coordinator: <@%s> \n> <%s/%s/%s>\n", contract.CreatorID[0], "https://eicoop-carpet.netlify.app", contract.ContractID, contract.CoopID)
	if !contract.Speedrun && contract.VolunteerSink != "" {
		outputStr += fmt.Sprintf("> Post Contract Sink: **%s**\n", contract.Boosters[contract.VolunteerSink].Mention)
	}
	if contract.Speedrun {
		switch contract.SRData.SpeedrunState {
		case SpeedrunStateSignup:
			outputStr += contract.SRData.StatusStr
		case SpeedrunStateCRT:
			//outputStr += fmt.Sprintf("> Send Tokens to <@%s>\n", contract.SRData.SpeedrunStarterUserID)
		case SpeedrunStateBoosting:
			if contract.SRData.SpeedrunStyle == SpeedrunStyleWonky {
				afterListStr += fmt.Sprintf("\n**Send all tokens to <@%s>**\n", contract.SRData.SinkUserID)
			}
		case SpeedrunStatePost:
			//outputStr += fmt.Sprintf("> Send Tokens to <@%s>\n", contract.SRData.SinkUserID)
		}
	}

	if contract.State == ContractStateSignup {
		if contract.Speedrun {
			outputStr += "## Speedrun Sign-up List\n"
		} else {
			outputStr += "## Sign-up List\n"
		}
	}

	if contract.Speedrun && contract.SRData.SpeedrunState == SpeedrunStateCRT {
		// Handle Speedrun CRT
		outputStr += drawSpeedrunCRT(contract, tokenStr)

		return outputStr
	}

	if contract.State == ContractStateStarted {
		if contract.Speedrun && contract.SRData.SpeedrunStyle == SpeedrunStyleWonky {
			outputStr += "## Wonky Speedrun Boost List\n"
		} else {
			outputStr += "## Boost List\n"
		}
	} else if contract.State >= ContractStateWaiting {
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
					contract.State = ContractStateStarted
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
				sinkIcon := ""
				if contract.Speedrun && contract.SRData.SpeedrunStyle == SpeedrunStyleWonky {
					if contract.SRData.SinkUserID == b.UserID {
						sinkIcon = fmt.Sprintf("%s[%d] %s", tokenStr, b.TokensReceived, "🫂")
					}
				}

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
				sinkIcon := ""
				if contract.Speedrun && contract.SRData.SpeedrunStyle == SpeedrunStyleWonky {
					if contract.SRData.SinkUserID == b.UserID {
						sinkIcon = fmt.Sprintf("%s[%d] %s", tokenStr, b.TokensReceived, "🫂")
					}
				}

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

			countStr, signupCountStr := getTokenCountString(tokenStr, b.TokensWanted, b.TokensReceived)

			if contract.Speedrun && contract.SRData.SpeedrunStyle == SpeedrunStyleWonky {
				sinkIcon := ""
				if contract.SRData.SinkUserID == b.UserID {
					sinkIcon = fmt.Sprintf("%s[%d] %s", tokenStr, b.TokensReceived, "🫂")
					signupCountStr = fmt.Sprintf("(%d)", b.TokensWanted)
				}
				switch b.BoostState {
				case BoostStateUnboosted:
					outputStr += fmt.Sprintf("%s %s%s%s%s\n", prefix, name, signupCountStr, sinkIcon, server)
				case BoostStateTokenTime:
					outputStr += fmt.Sprintf("%s **%s** %s%s%s%s\n", prefix, name, signupCountStr, currentStartTime, sinkIcon, server)
				case BoostStateBoosted:
					outputStr += fmt.Sprintf("%s ~~%s~~  %s %s%s\n", prefix, name, contract.Boosters[element].Duration.Round(time.Second), sinkIcon, server)
				}

			} else {

				switch b.BoostState {
				case BoostStateUnboosted:
					outputStr += fmt.Sprintf("%s %s%s%s\n", prefix, name, signupCountStr, server)
				case BoostStateTokenTime:
					outputStr += fmt.Sprintf("%s **%s** %s%s%s\n", prefix, name, countStr, currentStartTime, server)
				case BoostStateBoosted:
					outputStr += fmt.Sprintf("%s ~~%s~~  %s %s\n", prefix, name, contract.Boosters[element].Duration.Round(time.Second), server)
				}
			}
		}
	}
	outputStr += lateList

	outputStr += afterListStr

	// Add reaction guidance to the bottom of this list
	if contract.State == ContractStateStarted {
		outputStr += "\n"
		if contract.Speedrun && contract.SRData.SpeedrunStyle == SpeedrunStyleWonky {
			outputStr += "> " + tokenStr + " when sending tokens to the sink.\n"
			outputStr += "> 🐓 when you're ready for others to run chickens on your farm.\n"
			outputStr += "> 💰 is used by the Sink to send the requested number of tokens to the booster.\n"
			outputStr += "> -When active Booster is sent tokens by the sink they are marked as boosted.\n"
			outputStr += "> -Adjust the number of boost tokens you want by adding a 6️⃣ to 🔟 reaction to the boost list message.\n"

		} else {
			outputStr += "> Active Booster: " + boostIcon + " when boosting.\n"
			outputStr += "> Anyone: " + tokenStr + " when sending tokens. ❓ Help.\n"
		}
		if contract.CoopSize != len(contract.Order) {
			outputStr += "> Use pinned message or add 🧑‍🌾 reaction to join this list and set boost " + tokenStr + " wanted.\n"
		}
		//outputStr += "```"
	} else if contract.State == ContractStateWaiting {
		outputStr += "\n"
		outputStr += "> Waiting for other(s) to join...\n"
		outputStr += "> Use pinned message or add 🧑‍🌾 reaction to join this list and set boost " + tokenStr + " wanted.\n"
		outputStr += "```"
		outputStr += "React with 🏁 to end the contract."
		outputStr += "```"
	} else if contract.Speedrun && contract.SRData.SpeedrunState == SpeedrunStatePost {
		outputStr += "\n"
		outputStr += "Contract Boosting Completed!\n\n"
		outputStr += "> Send every " + tokenStr + " to our sink " + contract.Boosters[contract.SRData.SinkUserID].Mention + "\n"
		outputStr += "```"
		outputStr += "Coordinator can react with 🏁 to end the contract."
		outputStr += "```"

	}
	return outputStr
}
