package boost

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

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
	var header strings.Builder
	var currentTval float64
	//var outputStr string
	var afterListStr strings.Builder
	tokenStr := contract.TokenStr
	divider := true
	spacing := discordgo.SeparatorSpacingSizeSmall

	targetTval := GetTargetTval(contract.SeasonalScoring, contract.EstimatedDuration.Minutes(), float64(contract.MinutesPerToken))

	var bannerItem discordgo.MediaGalleryItem

	styleArray := []string{"", "c", "a", "f", "l"}

	if contract.Description == "" {
		header.WriteString("# Contract Interest List\n")
	} else {
		bannerItem.Media.URL = fmt.Sprintf("%sb%s-%s.png", config.BannerURL, styleArray[contract.PlayStyle], contract.ContractID)
		components = append(components, &discordgo.MediaGallery{
			Items: []discordgo.MediaGalleryItem{
				bannerItem,
			},
		},
		)
	}

	contract.LastInteractionTime = time.Now()

	saveData(contract.ContractHash)
	if contract.EggEmoji == "" {
		contract.EggEmoji = FindEggEmoji(contract.EggName)
	}

	if contract.Description != "" {
		fmt.Fprintf(&header, "## CoopID: [%s](%s/%s/%s)", contract.CoopID, "https://eicoop-carpet.netlify.app", contract.ContractID, contract.CoopID)
	} else {
		header.WriteString("## ")
	}
	if len(contract.Boosters) != contract.CoopSize {
		fmt.Fprintf(&header, " - %d/%d\n", len(contract.Boosters), contract.CoopSize)
	}
	header.WriteString("\n")

	if contract.State == ContractStateSignup && contract.PlannedStartTime.After(time.Now()) && contract.PlannedStartTime.Before(time.Now().Add(7*24*time.Hour)) {
		fmt.Fprintf(&header, "## Planned Start Time: <t:%d:f>\n", contract.PlannedStartTime.Unix())
	}

	if contract.Description != "" {
		if len(contract.Boosters) != contract.CoopSize || contract.State == ContractStateSignup {
			fmt.Fprintf(&header, "### Boost ordering is %s\n", getBoostOrderString(contract))
			if contract.Style&ContractFlag6Tokens != 0 {
				fmt.Fprintf(&header, ">  6️⃣%s boosting for everyone!\n", contract.TokenStr)
			} else if contract.Style&ContractFlag8Tokens != 0 {
				fmt.Fprintf(&header, ">  8️⃣%s boosting for everyone!\n", contract.TokenStr)
			}
		}
	}
	fmt.Fprintf(&header, "> Coordinator: <@%s>\n", contract.CreatorID[0])
	if contract.Description != "" {
		if contract.Location[0].GuildContractRole.ID != "" {
			fmt.Fprintf(&header, "> Team Role: %s\n", contract.Location[0].RoleMention)
		}
	}
	if contract.State == ContractStateSignup {
		if contract.Style&ContractFlagBanker != 0 {
			if contract.Banker.BoostingSinkUserID != "" {
				fmt.Fprintf(&header, "> * During boosting send all tokens to **%s**\n", contract.Boosters[contract.Banker.BoostingSinkUserID].Mention)
				switch contract.Banker.SinkBoostPosition {
				case SinkBoostFirst:
					fmt.Fprint(&header, ">  * Banker boosts **First**\n")
				case SinkBoostLast:
					fmt.Fprint(&header, ">  * Banker boosts **Last**\n")
				default:
					fmt.Fprint(&header, ">  * Banker follows normal boost order\n")
				}

			} else {
				fmt.Fprintf(&header, "> * **Contract cannot start**. Banker required for boosting phase.\n")
			}
		}
		if contract.Banker.PostSinkUserID != "" {
			fmt.Fprintf(&header, "> * After contract boosting send all tokens to **%s**\n", contract.Boosters[contract.Banker.PostSinkUserID].Mention)
		}
	}
	if contract.Style&ContractStyleFastrun != 0 && contract.Banker.PostSinkUserID != "" {
		if contract.State != ContractStateSignup && contract.Boosters[contract.Banker.PostSinkUserID] != nil {
			fmt.Fprintf(&header, "> Post Contract Sink: **%s**\n", contract.Boosters[contract.Banker.PostSinkUserID].Mention)
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
		contract.TokensPerMinute = float64(singleTokenEntries) / max(1.0, time.Since(contract.StartTime).Minutes()) / float64(max(1, len(contract.Boosters)))
		fmt.Fprintf(&header, "> %s/min/player: %2.3f %s\n", contract.TokenStr, contract.TokensPerMinute, ggicon)
	}

	// Current tval
	if contract.State != ContractStateSignup {
		if contract.EstimatedDuration == 0 {
			c := ei.EggIncContractsAll[contract.ContractID]
			if c.ID != "" {
				contract.EstimatedDuration = c.EstimatedDuration
			}
		}
		if targetTval != 0.0 {
			currentTval = bottools.GetTokenValue(time.Since(contract.StartTime).Seconds(), contract.EstimatedDuration.Seconds())
			fmt.Fprintf(&header, "> TVal: 🎯%2.2f 📉%2.2f\n", targetTval, currentTval)
		}
	}

	if !contract.EstimateUpdateTime.IsZero() {
		fmt.Fprintf(&header, "> **Completion Time: <t:%d:f>**\n", contract.StartTime.Add(contract.EstimatedDuration).Unix())
		fmt.Fprintf(&header, "> Duration: %v\n", contract.EstimatedDuration)
	}

	components = append(components, &discordgo.TextDisplay{
		Content: header.String(),
	})

	var builder strings.Builder

	switch contract.State {
	case ContractStateBanker:
		if contract.Banker.CurrentBanker == "" {
			if contract.Banker.PostSinkUserID != "" {
				contract.Banker.CurrentBanker = contract.Banker.PostSinkUserID
			}
		}
		if contract.Banker.CurrentBanker != "" {
			fmt.Fprintf(&afterListStr, "\n## Send all tokens to %s\n", contract.Boosters[contract.Banker.CurrentBanker].Mention)
		}

	default:
	}

	// Add chicken run status links for red and yellow messages
	if len(contract.CRMessageIDs) > 0 {
		var redLinks []string
		var yellowLinks []string

		for messageID, requesterUserID := range contract.CRMessageIDs {
			// Check if the chicken run request is within the last 5 minutes
			msgIDInt, _ := strconv.ParseInt(messageID, 10, 64)
			// Discord snowflake ID: timestamp is in the top 42 bits, milliseconds since epoch
			msgTimestamp := time.UnixMilli((msgIDInt >> 22) + 1420070400000)
			if time.Since(msgTimestamp) > 5*time.Minute {
				// Skip requests older than 5 minutes
				continue
			}

			var alreadyRun, missing []string

			// Count completed and missing runs
			for _, booster := range contract.Boosters {
				if booster.UserID == requesterUserID {
					continue
				}
				if slices.Contains(booster.RanChickensOn, requesterUserID) {
					alreadyRun = append(alreadyRun, booster.Mention)
				} else {
					missing = append(missing, booster.Mention)
				}
			}

			totalBoosters := len(alreadyRun) + len(missing)
			if totalBoosters > 0 {
				missingPercent := float64(len(missing)) / float64(totalBoosters) * 100
				channelID := contract.Location[0].ChannelID
				messageLink := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", contract.Location[0].GuildID, channelID, messageID)

				if missingPercent > 33.5 {
					// Red status
					requesterIndex := slices.Index(contract.Order, requesterUserID) + 1
					requesterName := contract.Boosters[requesterUserID].Nick
					redLinks = append(redLinks, fmt.Sprintf("[**#%d**](%s) %s", requesterIndex, messageLink, requesterName))
				} else if missingPercent > 0 {
					// Yellow status
					requesterIndex := slices.Index(contract.Order, requesterUserID) + 1
					requesterName := contract.Boosters[requesterUserID].Nick
					yellowLinks = append(yellowLinks, fmt.Sprintf("[**#%d**](%s) %s", requesterIndex, messageLink, requesterName))
				}
			}
		}

		if len(redLinks) > 0 || len(yellowLinks) > 0 {
			fmt.Fprintf(&afterListStr, "\n%s Chicken Runs: ", ei.GetBotEmojiMarkdown("icon_chicken_run"))
			if len(yellowLinks) > 0 {
				afterListStr.WriteString("🟨 ")
				afterListStr.WriteString(strings.Join(yellowLinks, ", "))
				if len(redLinks) > 0 {
					afterListStr.WriteString(" ")
				}
			}
			if len(redLinks) > 0 {
				afterListStr.WriteString("🟥 ")
				afterListStr.WriteString(strings.Join(redLinks, ", "))
			}
			afterListStr.WriteString("\n")
		}
	}

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
			fmt.Fprintf(&builder, "\n%s  %s\n", name, sinkIcon)
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
				// add the absolute value of start to end
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
					if contract.State == ContractStateSignup && contract.BoostOrder == ContractOrderTokenAsk {
						sortRate = fmt.Sprintf(" **Ask:%d** ", b.TokensWanted)
					}
					if contract.State == ContractStateSignup &&
						(contract.BoostOrder == ContractOrderTE || contract.BoostOrder == ContractOrderTEplus) {
						if b.TECount == -1 {
							if bottools.IsValidDiscordID(b.UserID) {
								sortRate = " **TE:🛜** "
							} else {
								sortRate = " **TE:0** "
							}
						} else {
							sortRate = fmt.Sprintf(" **TE:%d** ", b.TECount)
						}
					}
					if (contract.State == ContractStateBanker || contract.State == ContractStateFastrun) && contract.PlayStyle != ContractPlaystyleChill {
						//if (contract.State == ContractStateBanker || contract.State == ContractStateFastrun) && contract.BoostOrder == ContractOrderTVal {
						sortRate = fmt.Sprintf(" *∆:%2.3f* ", b.TokenValue)
					}

					sinkIcon := getSinkIcon(contract, b)

					if b.BoostState == BoostStateBoosted {
						fmt.Fprintf(&earlyList, "~~%s~~%s%s ", b.Mention, sortRate, sinkIcon)
					} else {
						fmt.Fprintf(&earlyList, "%s(%d)%s%s ", b.Mention, b.TokensWanted, sortRate, sinkIcon)
					}
					if i < start-1 {
						earlyList.WriteString(", ")
					}
				}
			}
			if earlyList.Len() > 0 {
				temp := earlyList.String()
				earlyList.Reset()
				if start == 1 {
					fmt.Fprintf(&earlyList, "1: %s\n", temp)
				} else {
					fmt.Fprintf(&earlyList, "1-%d: %s\n", start, temp)
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
					if contract.State == ContractStateSignup &&
						(contract.BoostOrder == ContractOrderTE || contract.BoostOrder == ContractOrderTEplus) {
						if b.TECount == -1 {
							if bottools.IsValidDiscordID(b.UserID) {
								sortRate = " **TE:🛜** "
							} else {
								sortRate = " **TE:0** "
							}
						} else {
							sortRate = fmt.Sprintf(" **TE:%d** ", b.TECount)
						}
					}
					if (contract.State == ContractStateBanker || contract.State == ContractStateFastrun) && contract.PlayStyle != ContractPlaystyleChill {
						//if (contract.State == ContractStateBanker || contract.State == ContractStateFastrun) && contract.BoostOrder == ContractOrderTVal {
						sortRate = fmt.Sprintf(" *∆:%2.3f* ", b.TokenValue)
					}

					sinkIcon := getSinkIcon(contract, b)

					if b.BoostState == BoostStateBoosted {
						fmt.Fprintf(&lateList, "~~%s~~%s%s ", b.Mention, sortRate, sinkIcon)
					} else {
						fmt.Fprintf(&lateList, "%s(%d)%s%s ", b.Mention, b.TokensWanted, sortRate, sinkIcon)
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
					fmt.Fprintf(&lateList, "%d: %s", end+1, temp)
				} else {
					temp := lateList.String()
					lateList.Reset()
					fmt.Fprintf(&lateList, "%d-%d: %s", end+1, len(contract.Order), temp)
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
				if contract.State == ContractStateSignup &&
					(contract.BoostOrder == ContractOrderTE || contract.BoostOrder == ContractOrderTEplus) {
					if b.TECount == -1 {
						if bottools.IsValidDiscordID(b.UserID) {
							sortRate = " **TE:🛜** "
						} else {
							sortRate = " **TE:0** "
						}
					} else {
						sortRate = fmt.Sprintf(" **TE:%d** ", b.TECount)
					}
				}
				if (contract.State == ContractStateBanker || contract.State == ContractStateFastrun) && contract.PlayStyle != ContractPlaystyleChill {
					//if (contract.State == ContractStateBanker || contract.State == ContractStateFastrun) && contract.BoostOrder == ContractOrderTVal {
					sortRate = fmt.Sprintf(" *∆:%2.3f* ", b.TokenValue)
				}

				sinkIcon := getSinkIcon(contract, b)
				if contract.State == ContractStateBanker {

					switch b.BoostState {
					case BoostStateUnboosted:
						fmt.Fprintf(&builder, "%s %s%s%s%s%s\n", prefix, name, signupCountStr, sortRate, sinkIcon, server)
					case BoostStateTokenTime:
						fmt.Fprintf(&builder, "%s ➡️ **%s** %s%s%s%s%s\n", prefix, name, signupCountStr, sortRate, currentStartTime, sinkIcon, server)
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
						fmt.Fprintf(&builder, "%s ~~%s~~  %s %s%s%s%s%s\n", prefix, name, sortRate, contract.Boosters[element].Duration.Round(time.Second), sinkIcon, boostingString, chickenStr, server)
					}

				} else {

					switch b.BoostState {
					case BoostStateUnboosted:
						fmt.Fprintf(&builder, "%s %s%s%s%s\n", prefix, name, signupCountStr, sortRate, server)
					case BoostStateTokenTime:
						if b.UserID == b.Name && b.AltController == "" && contract.State != ContractStateBanker {
							// Add a rocket for auto boosting
							fmt.Fprintf(&builder, "%s ➡️ **%s** 🚀%s%s%s%s\n", prefix, name, countStr, sortRate, currentStartTime, server)
						} else {
							if !b.BoostingTokenTimestamp.IsZero() {
								currentStartTime = fmt.Sprintf(" <t:%d:R> since ️T-0️⃣ / votes:%d", b.BoostingTokenTimestamp.Unix(), len(b.VotingList))
							}
							fmt.Fprintf(&builder, "%s ➡️ **%s** %s%s%s%s\n", prefix, name, countStr, sortRate, currentStartTime, server)
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
						fmt.Fprintf(&builder, "%s ~~%s~~  %s %s%s%s%s%s\n", prefix, name, sortRate, contract.Boosters[element].Duration.Round(time.Second), sinkIcon, boostingString, chickenStr, server)
					}
				}
			}
		}

		if contract.State == ContractStateSignup && len(contract.WaitlistBoosters) > 0 {
			// Loop through the waitlist and list waitlist folks
			builder.WriteString("\n### Backups\n")
			for _, userID := range contract.WaitlistBoosters {
				if bottools.IsValidDiscordID(userID) {
					builder.WriteString("<@" + userID + "> ")
				} else {
					builder.WriteString(userID + " ")
				}
			}
			components = append(components, &discordgo.TextDisplay{
				Content: builder.String(),
			})
			builder.Reset()

			components = append(components, &discordgo.Separator{
				Divider: &divider,
				Spacing: &spacing,
			})

		}

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
		guidanceStr.WriteString("> " + tokenStr + " when sending tokens to the Banker")
		if len(contract.AltIcons) > 0 {
			guidanceStr.WriteString(", alts use 🇦-🇿")
		}
		runReady, _, _ := ei.GetBotEmoji("runready")

		guidanceStr.WriteString(".\n")
		guidanceStr.WriteString("> " + runReady + " when you're ready for others to run chickens on your farm.\n")
		guidanceStr.WriteString("> 💰 is used by the Banker to send the requested number of tokens to the booster.\n")
		guidanceStr.WriteString("> -When active Booster is sent tokens by the Banker they are marked as boosted.\n")
		guidanceStr.WriteString("> -Adjust the number of boost tokens you want by adding a 6️⃣ to 🔟 reaction to the boost list message.\n")
		if contract.CoopSize != len(contract.Order) {
			guidanceStr.WriteString("> Use pinned message or add 🧑‍🌾 reaction to join this list and set boost " + tokenStr + " wanted.\n")
		}
		// Sum the Content lengths of the components for this length test
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
		fmt.Fprintf(&builder, "Contract boosting complete in %s with a rate of %2.3g %s/min\n", duration.Round(time.Second), contract.TokensPerMinute, contract.TokenStr)

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
