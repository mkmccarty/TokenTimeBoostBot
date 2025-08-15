package boost

import (
	"fmt"
	"strings"
	"time"

	"bottools"
	"config"
	"ei"
	"farmerstate"

	"github.com/bwmarrin/discordgo"
)

// Return the number of tokens received by the user from the token log
// This includes farmed tokens
func getTokensReceivedFromLog(contract *Contract, userID string) int {
	var tokensReceived = 0
	for _, logEntry := range contract.TokenLog {
		if logEntry.ToUserID == userID {
			tokensReceived += logEntry.Quantity
			if logEntry.FromUserID == logEntry.ToUserID && logEntry.Boost {
				// Banker boosted tokens are not counted as received
				tokensReceived -= logEntry.Quantity
			}
		}
	}
	return tokensReceived
}

func getTokensSentFromLog(contract *Contract, userID string) int {
	var tokensSent = 0
	for _, logEntry := range contract.TokenLog {
		if logEntry.FromUserID == userID && logEntry.FromUserID != logEntry.ToUserID {
			tokensSent += logEntry.Quantity
		}
	}
	return tokensSent
}

func getSinkIcon(contract *Contract, b *Booster) string {
	var sinkIcon = ""
	if contract.Banker.CurrentBanker == b.UserID {
		b.TokensReceived = getTokensReceivedFromLog(contract, b.UserID) - getTokensSentFromLog(contract, b.UserID)
		if b.BoostState == BoostStateBoosted {
			b.TokensReceived -= b.TokensWanted
		}
		sinkIcon = fmt.Sprintf("%s[%d] %s", contract.TokenStr, b.TokensReceived, ei.GetBotEmojiMarkdown("tvalrip"))
	}

	return sinkIcon
}

// DrawBoostList will draw the boost list for the contract
func DrawBoostList(s *discordgo.Session, contract *Contract) []discordgo.MessageComponent {
	var components []discordgo.MessageComponent
	var builder strings.Builder
	var currentTval float64
	//var outputStr string
	var afterListStr strings.Builder
	tokenStr := contract.TokenStr
	divider := true
	spacing := discordgo.SeparatorSpacingSizeSmall
	targetTval := 3.0
	BTA := contract.EstimatedDuration.Minutes() / float64(contract.MinutesPerToken)
	if BTA > 42.0 {
		targetTval = 0.07 * BTA
	}

	var bannerItem discordgo.MediaGalleryItem

	styleArray := []string{"", "c", "a", "f", "l"}

	bannerItem.Media.URL = fmt.Sprintf("%sb%s-%s.png", config.BannerURL, styleArray[contract.PlayStyle], contract.ContractID)
	components = append(components, &discordgo.MediaGallery{
		Items: []discordgo.MediaGalleryItem{
			bannerItem,
		},
	},
	)

	/*
		components = append(components, &discordgo.TextDisplay{
			Content: header.String() + "\n" + builder.String(),
		})

		components = append(components, &discordgo.Separator{
			Divider: &divider,
			Spacing: &spacing,
		})
	*/

	contract.LastInteractionTime = time.Now()

	saveData(Contracts)
	if contract.EggEmoji == "" {
		contract.EggEmoji = FindEggEmoji(contract.EggName)
	}

	builder.WriteString(fmt.Sprintf("## CoopID: [%s](%s/%s/%s)", contract.CoopID, "https://eicoop-carpet.netlify.app", contract.ContractID, contract.CoopID))
	//builder.WriteString(fmt.Sprintf("## %s %s : [%s](%s/%s/%s)", contract.EggEmoji, contract.Name, contract.CoopID, "https://eicoop-carpet.netlify.app", contract.ContractID, contract.CoopID))
	if len(contract.Boosters) != contract.CoopSize {
		builder.WriteString(fmt.Sprintf(" - %d/%d\n", len(contract.Boosters), contract.CoopSize))
	}
	builder.WriteString("\n")

	if contract.State == ContractStateSignup && contract.PlannedStartTime.After(time.Now()) && contract.PlannedStartTime.Before(time.Now().Add(7*24*time.Hour)) {
		builder.WriteString(fmt.Sprintf("## Planned Start Time: <t:%d:f>\n", contract.PlannedStartTime.Unix()))
	}

	if len(contract.Boosters) != contract.CoopSize || contract.State == ContractStateSignup {
		builder.WriteString(fmt.Sprintf("### Boost ordering is %s\n", getBoostOrderString(contract)))

		if contract.Style&ContractFlag6Tokens != 0 {
			builder.WriteString(fmt.Sprintf(">  6️⃣%s boosting for everyone!\n", contract.TokenStr))
		} else if contract.Style&ContractFlag8Tokens != 0 {
			builder.WriteString(fmt.Sprintf(">  8️⃣%s boosting for everyone!\n", contract.TokenStr))
		} else if contract.Style&ContractFlagDynamicTokens != 0 {
			builder.WriteString("> 🤖 Dynamic tokens (coming soon)\n")
		}
	}

	builder.WriteString(fmt.Sprintf("> Coordinator: <@%s>\n", contract.CreatorID[0]))
	if contract.Location[0].GuildContractRole.ID != "" {
		builder.WriteString(fmt.Sprintf("> Team Role: %s\n", contract.Location[0].RoleMention))
	}
	if contract.Style&ContractStyleFastrun != 0 && contract.Banker.PostSinkUserID != "" {
		if contract.State != ContractStateSignup && contract.Boosters[contract.Banker.PostSinkUserID] != nil {
			builder.WriteString(fmt.Sprintf("> Post Contract Sink: **%s**\n", contract.Boosters[contract.Banker.PostSinkUserID].Mention))
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
		gg, ugg, _ := ei.GetGenerousGiftEvent()
		ggicon := ""
		if gg > 1.0 {
			ggicon = " " + ei.GetBotEmojiMarkdown("std_gg")
		}
		if ugg > 1.0 && contract.UltraCount > 0 {
			// farmers with ultra
			//gg = ugg + (float64(contract.UltraCount) / float64(contract.CoopSize))
			ggicon = " " + ei.GetBotEmojiMarkdown("ultra_gg")
		}

		//timerTokensSinceStart := math.Floor(float64(time.Since(contract.StartTime).Minutes()) / float64(float64(contract.MinutesPerToken)))
		//estTPM := (float64(0.101332)*gg + timerTokensSinceStart/time.Since(contract.StartTime).Minutes()) * float64(contract.CoopSize)

		// How many single token entries are in the log, excludes banker sent tokens
		singleTokenEntries := 0
		for _, logEntry := range contract.TokenLog {
			if logEntry.Quantity == 1 || logEntry.Quantity == 2 {
				singleTokenEntries++
			}
		}
		// Save this into the contract
		contract.TokensPerMinute = float64(singleTokenEntries) / time.Since(contract.StartTime).Minutes()
		// Commented out estimate for now, it's currently confusing and not very useful
		// builder.WriteString(fmt.Sprintf("> %s/min: %2.2f   Expected %1.2f%s\n", contract.TokenStr, contract.TokensPerMinute, estTPM, ggicon))
		builder.WriteString(fmt.Sprintf("> %s/min: %2.2f %s\n", contract.TokenStr, contract.TokensPerMinute, ggicon)) //   Expected %1.2f%s\n", contract.TokenStr, contract.TokensPerMinute, estTPM, ggicon)
	}

	// Current tval
	if contract.State != ContractStateSignup {
		if contract.EstimatedDuration == 0 {
			c := ei.EggIncContractsAll[contract.ContractID]
			if c.ID != "" {
				contract.EstimatedDuration = c.EstimatedDuration
			}
		}
		currentTval = bottools.GetTokenValue(time.Since(contract.StartTime).Seconds(), contract.EstimatedDuration.Seconds())
		builder.WriteString(fmt.Sprintf("> TVal: 🎯%2.2f 📉%2.2f\n", targetTval, currentTval))
	}

	if !contract.EstimateUpdateTime.IsZero() {
		builder.WriteString(fmt.Sprintf("> **Completion Time: <t:%d:f>**\n", contract.StartTime.Add(contract.EstimatedDuration).Unix()))
		builder.WriteString(fmt.Sprintf("> Duration: %v\n", contract.EstimatedDuration))
	}

	components = append(components, &discordgo.TextDisplay{
		Content: builder.String(),
	})
	builder.Reset()

	switch contract.State {
	case ContractStateSignup:
		builder.WriteString(contract.SRData.StatusStr)

	case ContractStateCRT:
		//builder.WriteString(fmt.Sprintf("> Send Tokens to <@%s>\n", contract.SRData.SpeedrunStarterUserID))

	case ContractStateBanker:
		if contract.Banker.CurrentBanker == "" {
			if contract.Banker.PostSinkUserID != "" {
				contract.Banker.CurrentBanker = contract.Banker.PostSinkUserID
			}
		}
		if contract.Banker.CurrentBanker != "" {
			afterListStr.WriteString(fmt.Sprintf("\n## Send all tokens to %s\n", contract.Boosters[contract.Banker.CurrentBanker].Mention))
		}

	default:
	}

	if contract.State == ContractStateCRT {
		// Handle Speedrun CRT
		builder.WriteString(drawSpeedrunCRT(contract))

		components = append(components, &discordgo.TextDisplay{
			Content: builder.String(),
		})

		components = append(components, &discordgo.Separator{
			Divider: &divider,
			Spacing: &spacing,
		})

		return components
	}

	/*
		if contract.State == ContractStateSignup {
			builder.WriteString("## Sign-up List\n")
		} else {
			builder.WriteString("## Boost List\n")
		}
	*/
	var prefix = " - "

	var earlyList strings.Builder
	var lateList strings.Builder

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
		//builder.WriteString("## Boost\n")
		if contract.Banker.CurrentBanker == "" {
			builder.WriteString("\nNo volunteer sink for this contract, hold your tokens.\n")
		} else {
			b := contract.Boosters[contract.Banker.CurrentBanker]
			var name = b.Mention
			var einame = farmerstate.GetEggIncName(b.UserID)
			if einame != "" {
				name += " " + einame
			}

			sinkIcon := getSinkIcon(contract, b)
			builder.WriteString(fmt.Sprintf("\n%s  %s\n", name, sinkIcon))
		}
	} else {
		if contract.State == ContractStateSignup {
			builder.WriteString("## Sign-up List\n")
		} else {
			builder.WriteString("## Boost List\n")
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
					// Additions for contract state value display
					sortRate := ""
					if contract.State == ContractStateSignup && contract.BoostOrder == ContractOrderELR {
						sortRate = fmt.Sprintf(" **ELR:%2.3f** ", min(b.ArtifactSet.LayRate, b.ArtifactSet.ShipRate))
					}
					if (contract.State == ContractStateBanker || contract.State == ContractStateFastrun) && contract.PlayStyle != ContractPlaystyleChill {
						//if (contract.State == ContractStateBanker || contract.State == ContractStateFastrun) && contract.BoostOrder == ContractOrderTVal {
						sortRate = fmt.Sprintf(" *∆:%2.3f* ", b.TokenValue)
					}

					sinkIcon := getSinkIcon(contract, b)

					if b.BoostState == BoostStateBoosted {
						earlyList.WriteString(fmt.Sprintf("~~%s~~%s%s ", b.Mention, sortRate, sinkIcon))
					} else {
						earlyList.WriteString(fmt.Sprintf("%s(%d)%s%s ", b.Mention, b.TokensWanted, sortRate, sinkIcon))
					}
					if i < start-1 {
						earlyList.WriteString(", ")
					}
				}
			}
			if earlyList.Len() > 0 {
				if start == 1 {
					earlyList.Reset()
					earlyList.WriteString(fmt.Sprintf("1: %s\n", earlyList.String()))
				} else {
					temp := earlyList.String()
					earlyList.Reset()
					earlyList.WriteString(fmt.Sprintf("1-%d: %s\n", start, temp))
				}
			}

			for i, element := range contract.Order[end:len(contract.Order)] {
				var b, ok = contract.Boosters[element]
				if ok {
					// Additions for contract state value display
					sortRate := ""
					if contract.State == ContractStateSignup && contract.BoostOrder == ContractOrderELR {
						sortRate = fmt.Sprintf(" **ELR:%2.3f** ", min(b.ArtifactSet.LayRate, b.ArtifactSet.ShipRate))
					}
					if (contract.State == ContractStateBanker || contract.State == ContractStateFastrun) && contract.PlayStyle != ContractPlaystyleChill {
						//if (contract.State == ContractStateBanker || contract.State == ContractStateFastrun) && contract.BoostOrder == ContractOrderTVal {
						sortRate = fmt.Sprintf(" *∆:%2.3f* ", b.TokenValue)
					}

					sinkIcon := getSinkIcon(contract, b)

					if b.BoostState == BoostStateBoosted {
						lateList.WriteString(fmt.Sprintf("~~%s~~%s%s ", b.Mention, sortRate, sinkIcon))
					} else {
						lateList.WriteString(fmt.Sprintf("%s(%d)%s%s ", b.Mention, b.TokensWanted, sortRate, sinkIcon))
					}
					if (end + i + 1) < len(contract.Boosters) {
						lateList.WriteString(", ")
					}
				}
			}
			if lateList.Len() > 0 {
				if (end + 1) == len(contract.Order) {
					temp := lateList.String()
					lateList.Reset()
					lateList.WriteString(fmt.Sprintf("%d: %s", end+1, temp))
				} else {
					temp := lateList.String()
					lateList.Reset()
					lateList.WriteString(fmt.Sprintf("%d-%d: %s", end+1, len(contract.Order), temp))
				}
			}

			orderSubset = contract.Order[start:end]
			offset = start + 1
		}

		if earlyList.Len() > 0 {
			components = append(components, &discordgo.TextDisplay{
				Content: earlyList.String(),
			})
		}

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

				b.TokensReceived = getTokensReceivedFromLog(contract, b.UserID) // - getTokensSentFromLog(contract, b.UserID)

				countStr, signupCountStr := getTokenCountString(tokenStr, b.TokensWanted, b.TokensReceived)
				if b.UserID == contract.Banker.CurrentBanker {
					b.TokensReceived = getTokensReceivedFromLog(contract, b.UserID) - getTokensSentFromLog(contract, b.UserID)
					if b.BoostState != BoostStateBoosted {
						countStr, signupCountStr = getTokenCountString(tokenStr, b.TokensWanted, 0)
					}
				}

				// Additions for contract state value display
				sortRate := ""
				if contract.State == ContractStateSignup && contract.BoostOrder == ContractOrderELR {
					sortRate = fmt.Sprintf(" **ELR:%2.3f** ", min(b.ArtifactSet.LayRate, b.ArtifactSet.ShipRate))
				}
				if (contract.State == ContractStateBanker || contract.State == ContractStateFastrun) && contract.PlayStyle != ContractPlaystyleChill {
					//if (contract.State == ContractStateBanker || contract.State == ContractStateFastrun) && contract.BoostOrder == ContractOrderTVal {
					sortRate = fmt.Sprintf(" *∆:%2.3f* ", b.TokenValue)
				}

				sinkIcon := getSinkIcon(contract, b)
				if contract.State == ContractStateBanker {

					switch b.BoostState {
					case BoostStateUnboosted:
						builder.WriteString(fmt.Sprintf("%s %s%s%s%s%s\n", prefix, name, signupCountStr, sortRate, sinkIcon, server))
					case BoostStateTokenTime:
						builder.WriteString(fmt.Sprintf("%s ➡️ **%s** %s%s%s%s%s\n", prefix, name, signupCountStr, sortRate, currentStartTime, sinkIcon, server))
					case BoostStateBoosted:
						boostingString := ""
						if time.Now().Before(b.EstEndOfBoost) {
							diamond, _, _ := ei.GetBotEmoji("trophy_diamond")
							habFull, _, _ := ei.GetBotEmoji("hab_full")
							if b.RunChickensTime.IsZero() {
								boostingString = fmt.Sprintf(" %s<t:%d:R> / ", diamond, b.EstRequestChickenRuns.Unix())
							} else {
								boostingString = fmt.Sprintf(" %s<t:%d:R>", habFull, b.EstEndOfBoost.Unix())
							}
						}
						builder.WriteString(fmt.Sprintf("%s ~~%s~~  %s %s%s%s%s%s\n", prefix, name, sortRate, contract.Boosters[element].Duration.Round(time.Second), sinkIcon, boostingString, chickenStr, server))
					}

				} else {

					switch b.BoostState {
					case BoostStateUnboosted:
						builder.WriteString(fmt.Sprintf("%s %s%s%s%s\n", prefix, name, signupCountStr, sortRate, server))
					case BoostStateTokenTime:
						if b.UserID == b.Name && b.AltController == "" && contract.State != ContractStateBanker {
							// Add a rocket for auto boosting
							builder.WriteString(fmt.Sprintf("%s ➡️ **%s** 🚀%s%s%s%s\n", prefix, name, countStr, sortRate, currentStartTime, server))
						} else {
							if !b.BoostingTokenTimestamp.IsZero() {
								currentStartTime = fmt.Sprintf(" <t:%d:R> since ️T-0️⃣ / votes:%d", b.BoostingTokenTimestamp.Unix(), len(b.VotingList))
							}
							builder.WriteString(fmt.Sprintf("%s ➡️ **%s** %s%s%s%s\n", prefix, name, countStr, sortRate, currentStartTime, server))
						}
					case BoostStateBoosted:
						boostingString := ""
						if time.Now().Before(b.EstEndOfBoost) {
							diamond, _, _ := ei.GetBotEmoji("trophy_diamond")
							habFull, _, _ := ei.GetBotEmoji("hab_full")
							if b.RunChickensTime.IsZero() {
								boostingString = fmt.Sprintf(" %s<t:%d:R> / ", diamond, b.EstRequestChickenRuns.Unix())
							} else {
								boostingString = fmt.Sprintf(" %s<t:%d:R>", habFull, b.EstEndOfBoost.Unix())
							}
						}
						builder.WriteString(fmt.Sprintf("%s ~~%s~~  %s %s%s%s%s%s\n", prefix, name, sortRate, contract.Boosters[element].Duration.Round(time.Second), sinkIcon, boostingString, chickenStr, server))
					}
				}
			}
		}
		//		builder.WriteString(lateList.String())
		if builder.Len() != 0 {
			components = append(components, &discordgo.TextDisplay{
				Content: builder.String(),
			})
			builder.Reset()
		}
		if lateList.Len() > 0 {
			components = append(components, &discordgo.TextDisplay{
				Content: lateList.String(),
			})
		}
		components = append(components, &discordgo.Separator{
			Divider: &divider,
			Spacing: &spacing,
		})

		if afterListStr.Len() != 0 {
			components = append(components, &discordgo.TextDisplay{
				Content: afterListStr.String(),
			})
			components = append(components, &discordgo.Separator{
				Divider: &divider,
				Spacing: &spacing,
			})
		}
	}

	var guidanceStr strings.Builder
	// Add reaction guidance to the bottom of this list
	switch contract.State {
	case ContractStateFastrun:
		guidanceStr.WriteString("\n")
		guidanceStr.WriteString("> Active Booster: " + boostIcon + " when boosting. \n")
		guidanceStr.WriteString("> Anyone: " + tokenStr + " when sending tokens ")
		if len(contract.AltIcons) > 0 {
			guidanceStr.WriteString(", alts use 🇦-🇿")
		}
		guidanceStr.WriteString(". ❓ Help.\n")
		if contract.CoopSize != len(contract.Order) {
			guidanceStr.WriteString("> Use pinned message or add 🧑‍🌾 reaction to join this list and set boost " + tokenStr + " wanted.\n")
		}
		totalContentLength := guidanceStr.Len()
		for _, component := range components {
			if textDisplay, ok := component.(*discordgo.TextDisplay); ok {
				totalContentLength += len(textDisplay.Content)
			}
		}
		if totalContentLength < 3000 {
			builder.WriteString(guidanceStr.String())
		}

	case ContractStateBanker:
		guidanceStr.WriteString("\n")
		guidanceStr.WriteString("> " + tokenStr + " when sending tokens to the sink")
		if len(contract.AltIcons) > 0 {
			guidanceStr.WriteString(", alts use 🇦-🇿")
		}
		runReady, _, _ := ei.GetBotEmoji("runready")

		guidanceStr.WriteString(".\n")
		guidanceStr.WriteString("> " + runReady + " when you're ready for others to run chickens on your farm.\n")
		guidanceStr.WriteString("> 💰 is used by the Sink to send the requested number of tokens to the booster.\n")
		guidanceStr.WriteString("> -When active Booster is sent tokens by the sink they are marked as boosted.\n")
		guidanceStr.WriteString("> -Adjust the number of boost tokens you want by adding a 6️⃣ to 🔟 reaction to the boost list message.\n")
		if contract.CoopSize != len(contract.Order) {
			guidanceStr.WriteString("> Use pinned message or add 🧑‍🌾 reaction to join this list and set boost " + tokenStr + " wanted.\n")
		}
		// Sum the Content lenghts of the components for this length test
		// If the length of the builder is less than 1900 characters, add the guidanceStr
		// to the builder
		totalContentLength := guidanceStr.Len()
		for _, component := range components {
			if textDisplay, ok := component.(*discordgo.TextDisplay); ok {
				totalContentLength += len(textDisplay.Content)
			}
		}
		if totalContentLength < 3000 {
			builder.WriteString(guidanceStr.String())
		}

	case ContractStateWaiting:
		coopTvalStr := calculateTokenValueCoopLog(contract, contract.EstimatedDuration, targetTval)
		components = append(components, &discordgo.TextDisplay{
			Content: coopTvalStr,
		})
		components = append(components, &discordgo.Separator{
			Divider: &divider,
			Spacing: &spacing,
		})
		guidanceStr.WriteString("> Waiting for other(s) to join...\n")
		guidanceStr.WriteString("> Use pinned message or add 🧑‍🌾 reaction to join this list and set boost " + tokenStr + " wanted.\n")
		totalContentLength := guidanceStr.Len()
		for _, component := range components {
			if textDisplay, ok := component.(*discordgo.TextDisplay); ok {
				totalContentLength += len(textDisplay.Content)
			}
		}
		if totalContentLength < 3000 {
			builder.WriteString(guidanceStr.String())
		}

	case ContractStateCompleted:
		coopTvalStr := calculateTokenValueCoopLog(contract, contract.EstimatedDuration, targetTval)
		components = append(components, &discordgo.TextDisplay{
			Content: coopTvalStr,
		})
		components = append(components, &discordgo.Separator{
			Divider: &divider,
			Spacing: &spacing,
		})
		t1 := contract.EndTime
		t2 := contract.StartTime
		duration := t1.Sub(t2)
		builder.WriteString("\n")
		builder.WriteString(fmt.Sprintf("Contract boosting complete in %s with a rate of %2.3g %s/min\n", duration.Round(time.Second), contract.TokensPerMinute, contract.TokenStr))

		sinkID := contract.Banker.CurrentBanker
		if sinkID != "" {
			builder.WriteString("##  Send every " + tokenStr + " to our sink " + contract.Boosters[sinkID].Mention + "\n")
		}
	}

	if builder.Len() != 0 {
		components = append(components, &discordgo.TextDisplay{
			Content: builder.String(),
		})
		builder.Reset()
	}

	return components
}
