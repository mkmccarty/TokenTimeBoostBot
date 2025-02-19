package boost

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func buttonReactionCheck(s *discordgo.Session, ChannelID string, contract *Contract, cUserID string) bool {

	if !userInContract(contract, cUserID) {
		return false
	}
	contract.mutex.Lock()
	defer contract.mutex.Unlock()
	keepReaction := true
	if contract.SRData.ChickenRunCheckMsgID == "" {
		// Empty list, build a new one
		boosterNames := make([]string, 0, len(contract.Boosters))
		for _, booster := range contract.Boosters {
			// Saving the CRT sink from having to react to run chickens
			boosterNames = append(boosterNames, booster.Mention)
			//}
		}
		slices.Sort(boosterNames)
		contract.SRData.NeedToRunChickens = boosterNames
	}

	index := slices.Index(contract.SRData.NeedToRunChickens, contract.Boosters[cUserID].Mention)
	if index != -1 {
		contract.SRData.NeedToRunChickens = remove(contract.SRData.NeedToRunChickens, index)
		if len(contract.Boosters[cUserID].Alts) > 0 {
			// This user has an alt, clear the reaction so they can select it again
			keepReaction = false
		}
	} else if len(contract.Boosters[cUserID].Alts) > 0 {
		// Check for alts and remove them one by one
		for _, altID := range contract.Boosters[cUserID].Alts {
			index = slices.Index(contract.SRData.NeedToRunChickens, contract.Boosters[altID].Mention)
			if index != -1 {
				// only remove one name for each press of the button
				contract.SRData.NeedToRunChickens = remove(contract.SRData.NeedToRunChickens, index)
				if index == (len(contract.Boosters[cUserID].Alts) - 1) {
					keepReaction = false
				}
				break
			}
		}
	}

	if len(contract.SRData.NeedToRunChickens) > 0 {
		var str string
		if len(contract.SRData.NeedToRunChickens) <= 3 {
			str = fmt.Sprintf("Waiting on CRT chicken run ✅ from: **%s**", strings.Join(contract.SRData.NeedToRunChickens, ","))
		} else {
			str = fmt.Sprintf("Waiting on CRT chicken run ✅ from **%d/%d**", len(contract.SRData.NeedToRunChickens), contract.CoopSize)
		}

		if contract.SRData.ChickenRunCheckMsgID == "" {
			var data discordgo.MessageSend
			data.Content = str
			data.Flags = discordgo.MessageFlagsSuppressNotifications
			msg, err := s.ChannelMessageSendComplex(ChannelID, &data)
			if err == nil {
				contract.SRData.ChickenRunCheckMsgID = msg.ID
			}
		} else {
			msg := discordgo.NewMessageEdit(ChannelID, contract.SRData.ChickenRunCheckMsgID)
			msg.SetContent(str)
			_, _ = s.ChannelMessageEditComplex(msg)
		}
	}

	if len(contract.SRData.NeedToRunChickens) == 0 {
		_ = s.ChannelMessageDelete(ChannelID, contract.SRData.ChickenRunCheckMsgID)
		contract.SRData.ChickenRunCheckMsgID = ""

		str := fmt.Sprintf("All players have run chickens. **%s** should now react with 🦵 then start to kick all farmers.", contract.Boosters[contract.Banker.CurrentBanker].Mention)
		_, _ = s.ChannelMessageSend(ChannelID, str)
	}
	// Indicate to remove the reaction
	return keepReaction
}

func buttonReactionTruck(s *discordgo.Session, contract *Contract, cUserID string) bool {
	if !userInContract(contract, cUserID) {
		return false
	}

	// Indicate that the farmer has a truck incoming
	str := fmt.Sprintf("Truck arriving for **%s**. The sink may or may not pause kicks.", contract.Boosters[cUserID].Mention)
	for _, location := range contract.Location {
		_, _ = s.ChannelMessageSendComplex(location.ChannelID, &discordgo.MessageSend{
			Content: str,
			AllowedMentions: &discordgo.MessageAllowedMentions{
				Parse: []discordgo.AllowedMentionType{},
			},
		})
	}
	return false
}

func buttonReactionLeg(s *discordgo.Session, contract *Contract, cUserID string) bool {
	if (cUserID == contract.Banker.CurrentBanker || creatorOfContract(s, contract, cUserID)) && contract.SRData.LegReactionMessageID == "" {
		// Indicate that the Sink is starting to kick users
		str := fmt.Sprintf("**Starting to kick %d farmers.** Swap shiny artifacts if you need to force a server sync.\n", contract.SRData.Tango[0]-1)
		str += contract.Boosters[contract.Banker.CurrentBanker].Mention + " will react here with 💃 after kicks to advance the tango."
		for _, location := range contract.Location {
			var data discordgo.MessageSend
			data.Content = str
			data.Components = []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Emoji: &discordgo.ComponentEmoji{
								Name: "💃",
							},
							Style:    discordgo.SecondaryButton,
							CustomID: fmt.Sprintf("rc_#tango#%s", contract.ContractHash),
						},
					},
				},
			}
			msg, err := s.ChannelMessageSendComplex(location.ChannelID, &data)
			if err == nil {
				contract.SRData.LegReactionMessageID = msg.ID
			}
		}
		return true
	}
	return false
}

func buttonReactionTango(s *discordgo.Session, contract *Contract, cUserID string) bool {
	// Indicate that this Tango Leg is complete
	if cUserID == contract.Banker.CurrentBanker || creatorOfContract(s, contract, cUserID) {
		str := "Kicks completed."
		contract.SRData.CurrentLeg++ // Move to the next leg
		contract.SRData.LegReactionMessageID = ""
		contract.SRData.ChickenRunCheckMsgID = ""
		contract.SRData.NeedToRunChickens = nil
		if contract.SRData.CurrentLeg >= contract.SRData.Legs {
			if contract.Style&ContractFlagBanker != 0 {
				changeContractState(contract, ContractStateBanker)
			} else if contract.Style&ContractFlagFastrun != 0 {
				changeContractState(contract, ContractStateFastrun)
			} else {
				panic("Invalid contract style")
			}

			if contract.Style&ContractFlagBanker != 0 {
				changeContractState(contract, ContractStateBanker)
			} else if contract.Style&ContractFlagFastrun != 0 {
				changeContractState(contract, ContractStateFastrun)
			} else {
				panic("Invalid contract style")
			}

			str += " This was the final kick. Build up your farm as you would for boosting.\n"
		}
		contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = BoostStateTokenTime
		contract.Boosters[contract.Order[contract.BoostPosition]].StartTime = time.Now()

		for _, location := range contract.Location {
			_, _ = s.ChannelMessageSend(location.ChannelID, str)
		}
		sendNextNotification(s, contract, true)
		return true
	}
	return false
}

func reactionCRT(s *discordgo.Session, r *discordgo.MessageReaction, contract *Contract) {
	keepReaction := false
	redraw := false
	userID := r.UserID

	if r.Emoji.Name == "✅" {
		keepReaction = buttonReactionCheck(s, r.ChannelID, contract, r.UserID)
	}

	if r.Emoji.Name == "🚚" {
		keepReaction = buttonReactionTruck(s, contract, r.UserID)
	}

	idx := slices.Index(contract.Boosters[r.UserID].Alts, contract.Banker.CurrentBanker)
	if idx != -1 {
		// This is an alternate
		userID = contract.Boosters[r.UserID].Alts[idx]
	}

	if userID == contract.Banker.CurrentBanker || creatorOfContract(s, contract, r.UserID) {
		if r.Emoji.Name == "🦵" {
			redraw = buttonReactionLeg(s, contract, r.UserID)
		}

		if r.Emoji.Name == "💃" {
			keepReaction = buttonReactionTango(s, contract, r.UserID)
		}
	}

	// Remove extra added emoji
	if !keepReaction {
		go RemoveAddedReaction(s, r)
	}

	if redraw {
		refreshBoostListMessage(s, contract)
	}
}

func drawSpeedrunCRT(contract *Contract) string {
	var builder strings.Builder
	if contract.State == ContractStateCRT {
		fmt.Fprintf(&builder, "# Chicken Run Tango - Leg %d of %d\n", contract.SRData.CurrentLeg+1, contract.SRData.Legs)
		fmt.Fprintf(&builder, "### Tips\n")
		fmt.Fprintf(&builder, "- Don't use any boosts\n")
		//fmt.Fprintf(&builder, "- Equip coop artifacts: Deflector and SIAB\n")
		fmt.Fprintf(&builder, "- A chicken run on %s can be saved for the boost phase.\n", contract.Boosters[contract.Banker.CurrentBanker].Mention)
		fmt.Fprintf(&builder, "- :truck: reaction will indicate truck arriving and request a later kick. Send tokens through the boost menu if doing this.\n")
		fmt.Fprintf(&builder, "- Sink will react with 🦵 when starting to kick.\n")
		if contract.SRData.CurrentLeg == contract.SRData.Legs-1 {
			fmt.Fprintf(&builder, "### Final Kick Leg\n")
			fmt.Fprintf(&builder, "- After this kick you can build up your farm as you would for boosting\n")
		}
		taskNum := 1
		fmt.Fprintf(&builder, "## Tasks\n")
		fmt.Fprintf(&builder, "%d. Upgrade habs\n", taskNum)
		taskNum++
		fmt.Fprintf(&builder, "%d. Build up your farm to at least 20 chickens\n", taskNum)
		taskNum++
		fmt.Fprintf(&builder, "%d. Equip shiny artifact to force a server sync\n", taskNum)
		taskNum++
		if contract.Style&ContractFlagSelfRuns != 0 {
			if contract.SRData.Legs != contract.SRData.NoSelfRunLegs {
				fmt.Fprintf(&builder, "%d. **Run chickens on your own farm**\n", taskNum)
				taskNum++
			}
		}
		fmt.Fprintf(&builder, "%d. Run chickens on all the other farms and react with :white_check_mark: after all runs\n", taskNum)

		if contract.SRData.ChickenRunCheckMsgID != "" {
			link := fmt.Sprintf("https://discordapp.com/channels/%s/%s/%s", contract.Location[0].GuildID, contract.Location[0].ChannelID, contract.SRData.ChickenRunCheckMsgID)
			fmt.Fprintf(&builder, "\n[link to Chicken Run Check Status](%s)\n", link)
		}

	}
	fmt.Fprintf(&builder, "\n**Send %s to %s** [%d]\n", contract.TokenStr, contract.Boosters[contract.Banker.CurrentBanker].Mention, contract.Boosters[contract.Banker.CurrentBanker].TokensReceived)

	return builder.String()
}
