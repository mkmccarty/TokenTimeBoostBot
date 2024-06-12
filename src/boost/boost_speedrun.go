package boost

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/track"
	"github.com/rs/xid"
)

// HandleSpeedrunCommand handles the speedrun command
func HandleSpeedrunCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Protection against DM use
	if i.GuildID == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    "This command can only be run in a server.",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
		return
	}

	chickenRuns := 0
	contractStarter := ""
	sink := ""
	sinkPosition := SinkBoostFirst
	speedrunStyle := 0
	selfRuns := false

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["contract-starter"]; ok {
		contractStarter = opt.UserValue(s).Mention()
		contractStarter = contractStarter[2 : len(contractStarter)-1]
		sink = contractStarter
	}
	if opt, ok := optionMap["sink"]; ok {
		sink = strings.TrimSpace(opt.StringValue())
		reMention := regexp.MustCompile(`<@!?(\d+)>`)
		if reMention.MatchString(sink) {
			sink = sink[2 : len(sink)-1]
		}
	}
	if opt, ok := optionMap["style"]; ok {
		speedrunStyle = int(opt.IntValue())
	}
	if opt, ok := optionMap["chicken-runs"]; ok {
		chickenRuns = int(opt.IntValue())
	}
	if opt, ok := optionMap["self-runs"]; ok {
		selfRuns = opt.BoolValue()
	}
	if opt, ok := optionMap["sink-position"]; ok {
		sinkPosition = int(opt.IntValue())
	}

	str, err := setSpeedrunOptions(s, i.ChannelID, contractStarter, sink, sinkPosition, chickenRuns, speedrunStyle, selfRuns)
	if err != nil {
		str = err.Error()
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: str,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func getSpeedrunStatusStr(contract *Contract) string {
	var b strings.Builder
	fmt.Fprint(&b, "> Speedrun can be started once the contract is full.\n\n")
	if contract.SRData.SelfRuns {
		fmt.Fprintf(&b, "> --> **Self run of chickens is required** <--\n")
	}
	if contract.SRData.Tango[0] != 1 {
		fmt.Fprintf(&b, "> **%d** Chicken Run Legs to reach **%d** total chicken runs.\n", contract.SRData.Legs, contract.SRData.ChickenRuns)
	} else {
		fmt.Fprintf(&b, "> It's not possible to reach **%d** total chicken runs with only **%d** farmers.\n", contract.SRData.ChickenRuns, contract.CoopSize)
	}
	if contract.SRData.SpeedrunStyle == SpeedrunStyleWonky {
		fmt.Fprint(&b, "> **Wonky** style speed run:\n")
		fmt.Fprintf(&b, "> * Send all tokens to **%s**\n", contract.Boosters[contract.SRData.SpeedrunStarterUserID].Mention)
		fmt.Fprintf(&b, "> The sink will send you a full set of boost tokens.\n")
		if contract.SRData.SpeedrunStarterUserID != contract.SRData.SinkUserID {
			fmt.Fprintf(&b, "> * After contract boosting send all tokens to: %s (This is unusual)\n", contract.Boosters[contract.SRData.SinkUserID].Mention)
		}
	} else {
		fmt.Fprint(&b, "> **Boost List** style speed run:\n")
		fmt.Fprintf(&b, "> * During CRT send tokens to %s\n", contract.Boosters[contract.SRData.SpeedrunStarterUserID].Mention)
		fmt.Fprint(&b, "> * Follow the Boost List for Token Passing.\n")
		fmt.Fprintf(&b, "> * After contract boosting send all tokens to %s\n", contract.Boosters[contract.SRData.SinkUserID].Mention)
	}
	return b.String()
}

func setSpeedrunOptions(s *discordgo.Session, channelID string, contractStarter string, sink string, sinkPosition int, chickenRuns int, speedrunStyle int, selfRuns bool) (string, error) {
	var contract = FindContract(channelID)
	if contract == nil {
		return "", errors.New(errorNoContract)
	}

	if contract.State != ContractStateSignup {
		return "", errors.New("contract must be in the Sign-up state to set speedrun options")
	}

	// is contractStarter and sink in the contract
	if _, ok := contract.Boosters[contractStarter]; !ok {
		return "", errors.New("contract starter not in the contract")
	}
	if _, ok := contract.Boosters[sink]; !ok {
		return "", errors.New("sink not in the contract")
	}

	if speedrunStyle == SpeedrunStyleWonky {
		// Verify that the sink is a snowflake id
		if _, err := s.User(sink); err != nil {
			return "", errors.New("sink must user mention for Wonky style boost lists")
		}
	}

	contract.Speedrun = true
	contract.SRData.SpeedrunStarterUserID = contractStarter
	contract.SRData.SinkUserID = sink
	contract.SRData.SinkBoostPosition = sinkPosition
	contract.SRData.SelfRuns = selfRuns
	contract.SRData.SpeedrunStyle = speedrunStyle
	contract.SRData.SpeedrunState = SpeedrunStateSignup
	contract.BoostOrder = ContractOrderFair

	// Chicken Runs Calc
	// Info from https://egg-inc.fandom.com/wiki/Contracts
	if chickenRuns != 0 {
		contract.SRData.ChickenRuns = chickenRuns
	}

	// Set up the details for the Chicken Run Tango
	// first lap is CoopSize -1, every following lap is CoopSize -2
	// unless self runs
	selfRunMod := 1
	if selfRuns {
		selfRunMod = 0
	}

	contract.SRData.Tango[0] = max(0, contract.CoopSize-selfRunMod) // First Leg
	contract.SRData.Tango[1] = max(0, contract.SRData.Tango[0]-1)   // Middle Legs
	contract.SRData.Tango[2] = 0                                    // Last Leg

	runs := contract.SRData.ChickenRuns
	contract.SRData.Legs = 0
	for runs > 0 {
		if contract.SRData.Legs == 0 {
			runs -= contract.SRData.Tango[0]
			if runs <= 0 {
				break
			}
		} else if contract.SRData.Tango[1] == 0 {
			// Not possible to do any CRT
			break
		} else if runs > contract.SRData.Tango[1] {
			runs -= contract.SRData.Tango[1]
		} else {
			contract.SRData.Tango[2] = runs
			runs = 0
			break // No more runs to do, skips the Legs++ below
		}
		contract.SRData.Legs++
	}

	contract.SRData.StatusStr = getSpeedrunStatusStr(contract)

	var builder strings.Builder
	fmt.Fprintf(&builder, "Speedrun options set for %s/%s\n", contract.ContractID, contract.CoopID)
	fmt.Fprintf(&builder, "Contract Starter: %s\n", contract.Boosters[contract.SRData.SpeedrunStarterUserID].Mention)
	fmt.Fprintf(&builder, "Sink CRT: %s\n", contract.Boosters[contract.SRData.SinkUserID].Mention)

	disableButton := false
	if contract.Speedrun && contract.CoopSize != len(contract.Boosters) {
		disableButton = true
	}
	if contract.State != ContractStateSignup {
		disableButton = true
	}

	// For each contract location, update the signup message
	refreshBoostListMessage(s, contract)

	for _, loc := range contract.Location {
		// Rebuild the signup message to disable the start button
		msgID := loc.ReactionID
		msg := discordgo.NewMessageEdit(loc.ChannelID, msgID)

		contentStr, comp := GetSignupComponents(disableButton, contract.Speedrun) // True to get a disabled start button
		msg.SetContent(contentStr)
		msg.Components = &comp
		s.ChannelMessageEditComplex(msg)
	}

	return builder.String(), nil
}

func reorderSpeedrunBoosters(contract *Contract) {
	// Speedrun contracts are always fair ordering over last 3 contracts
	newOrder := farmerstate.GetOrderHistory(contract.Order, 3)

	index := slices.Index(newOrder, contract.SRData.SpeedrunStarterUserID)
	// Remove the speedrun starter from the list
	newOrder = append(newOrder[:index], newOrder[index+1:]...)

	if contract.SRData.SinkBoostPosition == SinkBoostFirst {
		newOrder = append([]string{contract.SRData.SpeedrunStarterUserID}, newOrder...)
	} else {
		newOrder = append(newOrder, contract.SRData.SpeedrunStarterUserID)
	}
	contract.Order = removeDuplicates(newOrder)
}

func drawSpeedrunCRT(contract *Contract, tokenStr string) string {
	var builder strings.Builder
	if contract.SRData.SpeedrunState == SpeedrunStateCRT {
		fmt.Fprintf(&builder, "# Chicken Run Tango - Leg %d of %d\n", contract.SRData.CurrentLeg+1, contract.SRData.Legs)
		fmt.Fprintf(&builder, "### Tips\n")
		fmt.Fprintf(&builder, "- Don't use any boosts\n")
		//fmt.Fprintf(&builder, "- Equip coop artifacts: Deflector and SIAB\n")
		fmt.Fprintf(&builder, "- A chicken run on %s can be saved for the boost phase.\n", contract.Boosters[contract.SRData.SpeedrunStarterUserID].Mention)
		fmt.Fprintf(&builder, "- :truck: reaction will indicate truck arriving and request a later kick. Send tokens through the boost menu if doing this.\n")
		fmt.Fprintf(&builder, "- Sink will react with 🦵 when starting to kick.\n")
		if contract.SRData.CurrentLeg == contract.SRData.Legs-1 {
			fmt.Fprintf(&builder, "### Final Kick Leg\n")
			fmt.Fprintf(&builder, "- After this kick you can build up your farm as you would for boosting\n")
		}
		fmt.Fprintf(&builder, "## Tasks\n")
		fmt.Fprintf(&builder, "1. Upgrade habs\n")
		fmt.Fprintf(&builder, "2. Build up your farm to at least 20 chickens\n")
		fmt.Fprintf(&builder, "3. Equip shiny artifact to force a server sync\n")
		fmt.Fprintf(&builder, "4. Run chickens on all the other farms and react with :white_check_mark: after all runs\n")
		if contract.SRData.SelfRuns {
			fmt.Fprintf(&builder, "5. **Run chickens on your own farm**\n")
		}

	}
	fmt.Fprintf(&builder, "\n**Send %s to %s**\n", tokenStr, contract.Boosters[contract.SRData.SpeedrunStarterUserID].Mention)

	return builder.String()
}

func addSpeedrunContractReactions(s *discordgo.Session, contract *Contract, channelID string, messageID string, tokenStr string) {
	if contract.SRData.SpeedrunState == SpeedrunStateCRT {
		s.MessageReactionAdd(channelID, messageID, tokenStr) // Token Reaction
		for _, el := range contract.AltIcons {
			s.MessageReactionAdd(channelID, messageID, el)
		}
		s.MessageReactionAdd(channelID, messageID, "✅") // Run Reaction
		s.MessageReactionAdd(channelID, messageID, "🚚") // Truck Reaction
		s.MessageReactionAdd(channelID, messageID, "🦵") // Kick Reaction
	}
	if contract.SRData.SpeedrunState == SpeedrunStateBoosting {
		s.MessageReactionAdd(channelID, messageID, tokenStr) // Send token to Sink
		for _, el := range contract.AltIcons {
			s.MessageReactionAdd(channelID, messageID, el)
		}
		s.MessageReactionAdd(channelID, messageID, "🐓") // Want Chicken Run
		s.MessageReactionAdd(channelID, messageID, "💰") // Sink sent requested number of tokens to booster
	}
	if contract.SRData.SpeedrunState == SpeedrunStatePost {
		s.MessageReactionAdd(channelID, messageID, tokenStr) // Send token to Sink
		for _, el := range contract.AltIcons {
			s.MessageReactionAdd(channelID, messageID, el)
		}
		s.MessageReactionAdd(channelID, messageID, "🐓") // Want Chicken Run
		//s.MessageReactionAdd(channelID, messageID, "🏁") // Run Reaction
	}
}

func speedrunReactions(s *discordgo.Session, r *discordgo.MessageReaction, contract *Contract) string {
	returnVal := ""
	keepReaction := false
	redraw := false
	emojiName := r.Emoji.Name

	// Token reaction handling
	tokenReactionStr := "token"
	userID := r.UserID
	// Special handling for alt icons representing token reactions
	if slices.Index(contract.AltIcons, r.Emoji.Name) != -1 {
		idx := slices.Index(contract.Boosters[r.UserID].AltsIcons, r.Emoji.Name)
		if idx != -1 {
			userID = contract.Boosters[r.UserID].Alts[idx]
			tokenReactionStr = r.Emoji.Name
		}
	}

	if strings.ToLower(r.Emoji.Name) == tokenReactionStr {
		var b *Booster
		if contract.SRData.SpeedrunState == SpeedrunStateCRT {
			b = contract.Boosters[contract.SRData.SpeedrunStarterUserID]
		} else {
			b = contract.Boosters[contract.SRData.SinkUserID]
		}

		b.TokensReceived++
		if r.Emoji.ID != "" {
			emojiName = r.Emoji.Name + ":" + r.Emoji.ID
		}
		if userID != b.UserID {

			// Record the Tokens as received
			rSerial := xid.New().String()
			b.Received = append(b.Received, TokenUnit{Time: time.Now(), Value: 0.0, UserID: contract.Boosters[userID].Nick, Serial: rSerial})
			track.ContractTokenMessage(s, r.ChannelID, b.UserID, track.TokenReceived, 1, contract.Boosters[userID].Nick, rSerial)

			// Record who sent the token
			sSerial := xid.New().String()
			track.ContractTokenMessage(s, r.ChannelID, userID, track.TokenSent, 1, b.Nick, sSerial)
			contract.Boosters[r.UserID].Sent = append(contract.Boosters[userID].Sent, TokenUnit{Time: time.Now(), Value: 0.0, UserID: b.Nick, Serial: sSerial})
		} else {
			track.FarmedToken(s, r.ChannelID, userID)
			b.TokensFarmedTime = append(b.TokensFarmedTime, time.Now())
		}
		redraw = true
	}

	if contract.SRData.SpeedrunState == SpeedrunStateCRT {

		if r.Emoji.Name == "✅" {
			keepReaction = true
			var msg, err = s.ChannelMessage(r.ChannelID, r.MessageID)
			if err == nil {
				// Display the list of users who have not run chickens yet
				uids, _ := s.MessageReactions(r.ChannelID, r.MessageID, r.Emoji.Name, 100, "", "")
				copyOrder := make([]string, len(contract.Order))
				copy(copyOrder, contract.Order)
				for _, uid := range uids {
					// get the index of uid.ID within copyOrder
					index := slices.Index(copyOrder, uid.ID)
					if index != -1 {
						copyOrder = append(copyOrder[:index], copyOrder[index+1:]...)
					}
				}
				// create a string of the remaining users and post it to the channel
				remainingUsers := ""
				for _, uid := range copyOrder {
					remainingUsers += contract.Boosters[uid].Mention
				}

				if len(copyOrder) <= 3 {
					str := fmt.Sprintf("Waiting on CRT chicken runs from: %s", remainingUsers)

					if contract.SRData.ChickenRunCheckMsgID == "" {
						msg, _ := s.ChannelMessageSend(r.ChannelID, str)
						contract.SRData.ChickenRunCheckMsgID = msg.ID
					} else {
						msg := discordgo.NewMessageEdit(r.ChannelID, contract.SRData.ChickenRunCheckMsgID)
						msg.SetContent(str)
						s.ChannelMessageEditComplex(msg)
					}
				}

				if msg.Reactions[1].Count > contract.CoopSize {
					s.ChannelMessageDelete(r.ChannelID, contract.SRData.ChickenRunCheckMsgID)
					contract.SRData.ChickenRunCheckMsgID = ""

					str := fmt.Sprintf("All players have run chickens. **%s** should now react with 🦵 then start to kick all farmers.", contract.Boosters[contract.SRData.SinkUserID].Mention)
					s.ChannelMessageSend(r.ChannelID, str)
				}
			}
			// Indicate that the farmer has completed running chickens
		}

		if r.Emoji.Name == "🚚" {
			keepReaction = true
			// Indicate that the farmer has a truck incoming
			str := fmt.Sprintf("Truck arriving for **%s**. The sink may or may not pause kicks.", contract.Boosters[r.UserID].Mention)
			s.ChannelMessageSend(contract.Location[0].ChannelID, str)
		}

		idx := slices.Index(contract.Boosters[r.UserID].Alts, contract.SRData.SpeedrunStarterUserID)
		if idx != -1 {
			// This is an alternate
			userID = contract.Boosters[r.UserID].Alts[idx]
		}

		if userID == contract.SRData.SpeedrunStarterUserID || creatorOfContract(contract, r.UserID) {
			if r.Emoji.Name == "🦵" {
				keepReaction = true
				// Indicate that the Sink is starting to kick users
				str := "**Starting to kick users.** Swap shiny artifacts if you need to force a server sync.\n"
				str += contract.Boosters[contract.SRData.SpeedrunStarterUserID].Mention + " will react here with 💃 after kicks to advance the tango."
				msg, _ := s.ChannelMessageSend(contract.Location[0].ChannelID, str)
				s.MessageReactionAdd(contract.Location[0].ChannelID, msg.ID, "💃") // Tango Reaction
				SetReactionID(contract, contract.Location[0].ChannelID, msg.ID)
				contract.SRData.LegReactionMessageID = msg.ID
				if contract.SRData.ChickenRunCheckMsgID != "" {
					s.ChannelMessageDelete(r.ChannelID, contract.SRData.ChickenRunCheckMsgID)
					contract.SRData.ChickenRunCheckMsgID = ""
				}
			}

			if r.Emoji.Name == "💃" {
				keepReaction = true

				// Indicate that this Tango Leg is complete
				str := "Kicks completed."
				contract.SRData.CurrentLeg++ // Move to the next leg
				if contract.SRData.CurrentLeg == contract.SRData.Legs {
					contract.SRData.SpeedrunState = SpeedrunStateBoosting
					str += " This was the final kick. Build up your farm as you would for boosting.\n"
				}
				//if contract.SRData.SpeedrunStyle == SpeedrunStyleFastrun {
				contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = BoostStateTokenTime
				contract.Boosters[contract.Order[contract.BoostPosition]].StartTime = time.Now()
				//}
				s.ChannelMessageSend(contract.Location[0].ChannelID, str)
				sendNextNotification(s, contract, true)
			}
		}
	}

	if contract.SRData.SpeedrunState == SpeedrunStateBoosting {
		idx := slices.Index(contract.Boosters[r.UserID].Alts, contract.SRData.SinkUserID)
		if idx != -1 {
			// This is an alternate
			userID = contract.Boosters[r.UserID].Alts[idx]
		}
		if userID == contract.SRData.SinkUserID {
			if r.Emoji.Name == "💰" {
				var b, sink *Booster
				b = contract.Boosters[contract.Order[contract.BoostPosition]]
				sink = contract.Boosters[contract.SRData.SinkUserID]

				if userID == b.UserID {
					// Current booster subtract number of tokens wanted
					log.Printf("Sink indicating they are boosting with %d tokens.\n", b.TokensWanted)
					sink.TokensReceived -= b.TokensWanted
					sink.TokensReceived = max(0, sink.TokensReceived) // Avoid missing self farmed tokens
				} else {
					log.Printf("Sink sent %d tokens to booster\n", b.TokensWanted)
					// Current booster number of tokens wanted
					// How many tokens does booster want?  Check to see if sink has that many
					tokensToSend := b.TokensWanted // If Sink is pressing 💰 they are assumed to be sending that many
					b.TokensReceived += tokensToSend
					sink.TokensReceived -= tokensToSend
					sink.TokensReceived = max(0, sink.TokensReceived) // Avoid missing self farmed tokens
					// Record the Tokens as received
					rSerial := xid.New().String()
					sSerial := xid.New().String()
					for i := 0; i < tokensToSend; i++ {
						b.Received = append(b.Received, TokenUnit{Time: time.Now(), Value: 0.0, UserID: contract.Boosters[userID].Nick, Serial: rSerial})
						contract.Boosters[userID].Sent = append(contract.Boosters[userID].Sent, TokenUnit{Time: time.Now(), Value: 0.0, UserID: contract.Boosters[b.UserID].Nick, Serial: sSerial})
					}
					track.ContractTokenMessage(s, r.ChannelID, b.UserID, track.TokenReceived, b.TokensReceived, contract.Boosters[userID].Nick, rSerial)
					track.ContractTokenMessage(s, r.ChannelID, userID, track.TokenSent, b.TokensReceived, contract.Boosters[b.UserID].Nick, sSerial)
				}

				Boosting(s, r.GuildID, r.ChannelID)

				str := fmt.Sprintf("**%s** ", contract.Boosters[b.UserID].Mention)
				if contract.Boosters[b.UserID].AltController != "" {
					str = fmt.Sprintf("%s **(%s)** ", contract.Boosters[contract.Boosters[b.UserID].AltController].Mention, b.UserID)
				}
				str += fmt.Sprintf("you've been sent %d tokens to boost with!", b.TokensWanted)

				s.ChannelMessageSend(contract.Location[0].ChannelID, str)

				redraw = false
			}
		}
	}

	if contract.SRData.SpeedrunState == SpeedrunStateBoosting || contract.SRData.SpeedrunState == SpeedrunStatePost {
		if r.Emoji.Name == "🐓" {
			// Indicate that a farmer is ready for chicken runs

			userID := r.UserID
			if len(contract.Boosters[r.UserID].Alts) > 0 {
				ids := append(contract.Boosters[r.UserID].Alts, r.UserID)
				for _, id := range contract.Order {
					if slices.Index(ids, id) != -1 {
						alt := contract.Boosters[id]
						if alt.BoostState == BoostStateBoosted && alt.RunChickensTime.IsZero() {
							userID = id
							break
						}
					}
				}
			}
			contract.Boosters[userID].RunChickensTime = time.Now()
			str := fmt.Sprintf("%s **%s** is ready for chicken runs, check for incoming trucks before visiting.", contract.Location[0].ChannelPing, contract.Boosters[userID].Mention)
			var data discordgo.MessageSend
			data.Content = str
			msg, _ := s.ChannelMessageSendComplex(contract.Location[0].ChannelID, &data)
			s.MessageReactionAdd(msg.ChannelID, msg.ID, contract.ChickenRunEmoji) // Indicate Chicken Run
			keepReaction = true
			redraw = true
		}
	}

	/*
		if contract.SRData.SpeedrunState == SpeedrunStatePost && creatorOfContract(contract, r.UserID) {
			// Coordinator can end the contract
			if r.Emoji.Name == "🏁" {
				contract.State = ContractStateArchive
				contract.SRData.SpeedrunState = SpeedrunStateComplete
				sendNextNotification(s, contract, true)
				return returnVal
			}
		}
	*/

	// Remove extra added emoji
	if !keepReaction {
		s.MessageReactionRemove(r.ChannelID, r.MessageID, emojiName, r.UserID)
	}

	if redraw {
		refreshBoostListMessage(s, contract)
	}

	return returnVal
}
