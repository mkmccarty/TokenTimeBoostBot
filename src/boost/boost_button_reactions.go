package boost

import (
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/track"
	"github.com/rs/xid"
)

// HandleContractReactions handles all the button reactions for a contract
func HandleContractReactions(s *discordgo.Session, i *discordgo.InteractionCreate) {

	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}

	// rc_Name # rc_ID # HASH
	reaction := strings.Split(i.MessageComponentData().CustomID, "#")
	cmd := strings.ToLower(reaction[1])
	contractHash := reaction[len(reaction)-1]

	contract := Contracts[contractHash]
	if contract == nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content: "Unable to find this contract.",
				Flags:   discordgo.MessageFlagsEphemeral,
			})
	}

	if cmd == "help" {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
			},
		})
		buttonReactionHelp(s, i, contract)
		return
	}
	/*
		compVals[boostIconReaction] = CompMap{Emoji: boostIconReaction, Style: discordgo.SecondaryButton, CustomID: "rc_#Boost#"}
		compVals[tokenStr] = CompMap{Emoji: strings.Split(tokenStr, ":")[0], ID: strings.Split(tokenStr, ":")[1], Style: discordgo.SecondaryButton, CustomID: "rc_#Token#"}
		compVals["💰"] = CompMap{Emoji: "💰", Style: discordgo.SecondaryButton, CustomID: "rc_#Bag#"}
		compVals["🚚"] = CompMap{Emoji: "🚚", Style: discordgo.SecondaryButton, CustomID: "rc_#Truck#"}
		compVals["💃"] = CompMap{Emoji: "💃", Style: discordgo.SecondaryButton, CustomID: "rc_#Tango#"}
		compVals["🦵"] = CompMap{Emoji: "🦵", Style: discordgo.SecondaryButton, CustomID: "rc_#Leg#"}
		compVals["✅"] = CompMap{Emoji: "✅", Style: discordgo.SecondaryButton, CustomID: "rc_#Check#"}
		for i, el := range contract.AltIcons {
			compVals[el] = CompMap{Emoji: el, Style: discordgo.SecondaryButton, CustomID: fmt.Sprintf("rc_#Token-%d#", i)}

	*/

	// Restring commands to those within the contract
	if !userInContract(contract, userID) && !creatorOfContract(s, contract, userID) {
		_, _ = s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content: "Unable to find this contract.",
				Flags:   discordgo.MessageFlagsEphemeral,
			})
		return
	}
	// Ack the message for every other command
	if cmd != "cr" {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
		})
	}

	redraw := false

	// Handle the alt icons and mapping to the correct alt user
	if strings.Contains(cmd, "alt-") {
		// Special handling for alt icons representing token reactions
		idx, _ := strconv.Atoi(strings.Split(cmd, "-")[1])
		if idx < len(contract.AltIcons) {
			idx := slices.Index(contract.Boosters[userID].AltsIcons, contract.AltIcons[idx])
			if idx != -1 {
				userID = contract.Boosters[userID].Alts[idx]
				cmd = "token"
			}
		}
	}

	switch cmd {
	case "boost":
		if i.Message.ID == contract.Location[0].ListMsgID {
			redraw = buttonReactionBoost(s, i.GuildID, i.ChannelID, contract, userID)
		}
	case "bag":
		_, redraw = buttonReactionBag(s, i.GuildID, i.ChannelID, contract, userID)
	case "token":
		_, redraw = buttonReactionToken(s, i.GuildID, i.ChannelID, contract, userID)
	case "swap":
		redraw = buttonReactionSwap(s, i.GuildID, i.ChannelID, contract, userID)
	case "last":
		_, redraw = buttonReactionLast(s, i.GuildID, i.ChannelID, contract, userID)
	case "cr":
		redraw = buttonReactionRunChickens(s, contract, userID)
		// Ack the message for every other command
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You've asked for Chicken Runs, now what...\n...\nMaybe.. check on your habs and gusset?  \nI'm sure you've already forced a game sync so no need to remind about that.\nMaybe self-runs?",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	case "ranchicken":
		buttonReactionRanChicken(s, i, contract, userID)
	case "truck":
		redraw = buttonReactionTruck(s, contract, userID)
	case "leg":
		redraw = buttonReactionLeg(s, contract, userID)
	case "tango":
		redraw = buttonReactionTango(s, contract, userID)
	case "check":
		redraw = buttonReactionCheck(s, i.ChannelID, contract, userID)
	}

	if redraw {
		refreshBoostListMessage(s, contract)
	}
}

func buttonReactionBoost(s *discordgo.Session, GuildID string, ChannelID string, contract *Contract, cUserID string) bool {
	// If Rocket reaction on Boost List, only that boosting user can apply a reaction
	redraw := false
	if contract.State == ContractStateStarted {
		votingElection := false

		userID := cUserID
		if contract.Speedrun {
			if contract.Boosters[cUserID] != nil && len(contract.Boosters[cUserID].Alts) > 0 {
				// Find the most recent boost time among the user and their alts
				for _, altID := range contract.Boosters[cUserID].Alts {
					if altID == contract.Order[contract.BoostPosition] {
						userID = altID
						break
					}
				}
			}
		}

		if userID != contract.Order[contract.BoostPosition] {
			b := contract.Boosters[contract.Order[contract.BoostPosition]]
			b.VotingList = append(b.VotingList, userID)
			votesNeeded := 2
			if len(b.VotingList) >= votesNeeded {
				votingElection = true
			} else {
				redraw = true
			}
			log.Printf("Vote for %s to boost from %s - vote count %d or %d\n", b.UserID, userID, len(b.VotingList), votesNeeded)
		}

		if userID == contract.Order[contract.BoostPosition] || votingElection || creatorOfContract(s, contract, cUserID) {
			//contract.mutex.Unlock()
			_ = Boosting(s, GuildID, ChannelID)
			return true
		}
	}
	return redraw
}

func buttonReactionBag(s *discordgo.Session, GuildID string, ChannelID string, contract *Contract, cUserID string) (bool, bool) {
	redraw := false
	if cUserID == contract.SRData.BoostingSinkUserID {
		var b, sink *Booster
		b = contract.Boosters[contract.Order[contract.BoostPosition]]
		sink = contract.Boosters[contract.SRData.BoostingSinkUserID]

		if cUserID == b.UserID {
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
				b.Received = append(b.Received, TokenUnit{Time: time.Now(), Value: 0.0, UserID: contract.Boosters[cUserID].Nick, Serial: rSerial})
				contract.Boosters[cUserID].Sent = append(contract.Boosters[cUserID].Sent, TokenUnit{Time: time.Now(), Value: 0.0, UserID: contract.Boosters[b.UserID].Nick, Serial: sSerial})
			}
			track.ContractTokenMessage(s, ChannelID, b.UserID, track.TokenReceived, b.TokensReceived, contract.Boosters[cUserID].Nick, rSerial)
			track.ContractTokenMessage(s, ChannelID, cUserID, track.TokenSent, b.TokensReceived, contract.Boosters[b.UserID].Nick, sSerial)
		}

		str := fmt.Sprintf("**%s** ", contract.Boosters[b.UserID].Mention)
		if contract.Boosters[b.UserID].AltController != "" {
			str = fmt.Sprintf("%s **(%s)** ", contract.Boosters[contract.Boosters[b.UserID].AltController].Mention, b.UserID)
		}
		str += fmt.Sprintf("you've been sent %d tokens to boost with!", b.TokensWanted)

		_, _ = s.ChannelMessageSend(contract.Location[0].ChannelID, str)

		_ = Boosting(s, GuildID, ChannelID)

		return false, redraw
	}
	return false, redraw
}

func buttonReactionToken(s *discordgo.Session, GuildID string, ChannelID string, contract *Contract, fromUserID string) (bool, bool) {
	if !contract.Speedrun && (contract.State == ContractStateWaiting || contract.State == ContractStateCompleted) {
		// Without a volunteer sink in the condition there is nobody to assign the tokens to...
		// The icon for this
		if contract.VolunteerSink != "" {
			sink := contract.Boosters[contract.VolunteerSink]
			// Record who received the token
			rSerial := xid.New().String()
			sink.Received = append(sink.Received, TokenUnit{Time: time.Now(), Value: 0.0, UserID: contract.Boosters[fromUserID].Nick, Serial: rSerial})
			track.ContractTokenMessage(s, ChannelID, sink.UserID, track.TokenReceived, 1, contract.Boosters[fromUserID].Nick, rSerial)
			// Record who sent the token
			sSerial := xid.New().String()
			contract.Boosters[fromUserID].Sent = append(contract.Boosters[fromUserID].Sent, TokenUnit{Time: time.Now(), Value: 0.0, UserID: contract.Boosters[sink.UserID].Nick, Serial: sSerial})
			track.ContractTokenMessage(s, ChannelID, fromUserID, track.TokenSent, 1, contract.Boosters[sink.UserID].Nick, sSerial)
		}
	} else if contract.Speedrun || contract.BoostPosition < len(contract.Order) {
		var b *Booster
		// Speedrun will use the sink booster instead
		if contract.Speedrun && (contract.SRData.SpeedrunState == SpeedrunStateCRT ||
			contract.SRData.SpeedrunStyle == SpeedrunStyleWonky && contract.SRData.SpeedrunState == SpeedrunStateBoosting ||
			contract.SRData.SpeedrunState == SpeedrunStatePost) {

			if contract.SRData.SpeedrunState == SpeedrunStateCRT {
				b = contract.Boosters[contract.SRData.CrtSinkUserID]
			} else if contract.SRData.SpeedrunState == SpeedrunStateBoosting {
				b = contract.Boosters[contract.SRData.BoostingSinkUserID]
			} else if contract.SRData.SpeedrunState == SpeedrunStatePost {
				b = contract.Boosters[contract.SRData.PostSinkUserID]
			}
		} else {
			b = contract.Boosters[contract.Order[contract.BoostPosition]]
		}

		b.TokensReceived++
		if fromUserID != b.UserID {
			// Record the Tokens as received
			rSerial := xid.New().String()
			b.Received = append(b.Received, TokenUnit{Time: time.Now(), Value: 0.0, UserID: contract.Boosters[fromUserID].Nick, Serial: rSerial})
			track.ContractTokenMessage(s, ChannelID, b.UserID, track.TokenReceived, 1, contract.Boosters[fromUserID].Nick, rSerial)

			// Record who sent the token
			sSerial := xid.New().String()
			if contract.Boosters[fromUserID] != nil {
				// Make sure this isn't an admin user who's sending on behalf of an alt
				contract.Boosters[fromUserID].Sent = append(contract.Boosters[fromUserID].Sent, TokenUnit{Time: time.Now(), Value: 0.0, UserID: b.Nick, Serial: sSerial})
			}
			track.ContractTokenMessage(s, ChannelID, fromUserID, track.TokenSent, 1, b.Nick, sSerial)
		} else {
			track.FarmedToken(s, ChannelID, fromUserID)
			b.TokensFarmedTime = append(b.TokensFarmedTime, time.Now())
		}
		if b.TokensReceived == b.TokensWanted {
			b.BoostingTokenTimestamp = time.Now()
		}

		if !contract.Speedrun && b.Name == b.UserID && b.TokensReceived >= b.TokensWanted && b.AltController == "" {
			// Guest farmer auto boosts
			_ = Boosting(s, GuildID, ChannelID)
			return true, false
		}
		return false, true
	}

	return false, false
}

func buttonReactionLast(s *discordgo.Session, GuildID string, ChannelID string, contract *Contract, cUserID string) (bool, bool) {
	var uid = cUserID
	if contract.Boosters[uid].BoostState == BoostStateTokenTime {
		currentBoosterPosition := findNextBooster(contract)
		err := MoveBooster(s, GuildID, ChannelID, contract.CreatorID[0], uid, len(contract.Order), currentBoosterPosition == -1)
		if err == nil && currentBoosterPosition != -1 {
			_ = ChangeCurrentBooster(s, GuildID, ChannelID, contract.CreatorID[0], contract.Order[currentBoosterPosition], true)
			return true, false
		}
	} else if contract.Boosters[uid].BoostState == BoostStateUnboosted {
		_ = MoveBooster(s, GuildID, ChannelID, contract.CreatorID[0], uid, len(contract.Order), true)
	}

	return false, false
}

func buttonReactionSwap(s *discordgo.Session, GuildID string, ChannelID string, contract *Contract, cUserID string) bool {
	// Reaction for current booster to change places
	if cUserID == contract.Order[contract.BoostPosition] || creatorOfContract(s, contract, cUserID) {
		if (contract.BoostPosition + 1) < len(contract.Order) {
			_ = SkipBooster(s, GuildID, ChannelID, "")
			return true
		}
	}
	return false
}

func buttonReactionRunChickens(s *discordgo.Session, contract *Contract, cUserID string) bool {
	userID := cUserID

	// Indicate that a farmer is ready for chicken runs
	if len(contract.Boosters[cUserID].Alts) > 0 {
		ids := append(contract.Boosters[cUserID].Alts, cUserID)
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

	if contract.Boosters[userID].BoostState == BoostStateBoosted && contract.Boosters[userID].RunChickensTime.IsZero() {

		contract.Boosters[userID].RunChickensTime = time.Now()
		go func() {
			for _, location := range contract.Location {
				str := fmt.Sprintf("%s **%s** is ready for chicken runs, check for incoming trucks before visiting.\nRunners:", location.ChannelPing, contract.Boosters[userID].Mention)
				var data discordgo.MessageSend
				data.Content = str
				data.Components = []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.Button{
								Emoji: &discordgo.ComponentEmoji{
									Name: strings.Split(contract.ChickenRunEmoji, ":")[1],
									ID:   strings.Split(contract.ChickenRunEmoji, ":")[2],
								},
								Style:    discordgo.SecondaryButton,
								CustomID: fmt.Sprintf("rc_#RanChicken#%s", contract.ContractHash),
							},
						},
					},
				}
				msg, err := s.ChannelMessageSendComplex(location.ChannelID, &data)
				if err == nil {
					setChickenRunMessageID(contract, msg.ID)
				}
			}
		}()
		return true
	}
	return false
}

func buttonReactionRanChicken(s *discordgo.Session, i *discordgo.InteractionCreate, contract *Contract, cUserID string) {

	log.Print("Ran Chicken")
	msgedit := discordgo.NewMessageEdit(i.ChannelID, i.Message.ID)

	str := i.Message.Content

	userMention := contract.Boosters[cUserID].Mention
	repost := false

	if !strings.Contains(strings.Split(str, "\n")[1], userMention) {
		str += " " + contract.Boosters[cUserID].Mention
		repost = true
	} else if len(contract.Boosters[cUserID].Alts) > 0 {
		for _, altID := range contract.Boosters[cUserID].Alts {
			if !strings.Contains(strings.Split(str, "\n")[1], contract.Boosters[altID].Mention) {
				str += " " + contract.Boosters[altID].Mention
				repost = true
				break
			}
		}
	}
	if repost {
		msgedit.SetContent(str)
		msgedit.Flags = discordgo.MessageFlagsSuppressNotifications
		_, _ = s.ChannelMessageEditComplex(msgedit)
	}
}

func remove(s []string, i int) []string {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

func buttonReactionCheck(s *discordgo.Session, ChannelID string, contract *Contract, cUserID string) bool {
	keepReaction := true
	if contract.SRData.ChickenRunCheckMsgID == "" {
		// Empty list, build a new one
		boosterNames := make([]string, 0, len(contract.Boosters))
		for _, booster := range contract.Boosters {
			// Saving the CRT sink from having to react to run chickens
			//if contract.SRData.CrtSinkUserID != booster.UserID {
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

		str := fmt.Sprintf("All players have run chickens. **%s** should now react with 🦵 then start to kick all farmers.", contract.Boosters[contract.SRData.CrtSinkUserID].Mention)
		_, _ = s.ChannelMessageSend(ChannelID, str)
	}
	// Indicate to remove the reaction
	return keepReaction
}

func buttonReactionTruck(s *discordgo.Session, contract *Contract, cUserID string) bool {
	// Indicate that the farmer has a truck incoming
	str := fmt.Sprintf("Truck arriving for **%s**. The sink may or may not pause kicks.", contract.Boosters[cUserID].Mention)
	for _, location := range contract.Location {
		_, _ = s.ChannelMessageSend(location.ChannelID, str)
	}
	return false
}

func buttonReactionLeg(s *discordgo.Session, contract *Contract, cUserID string) bool {
	if (cUserID == contract.SRData.CrtSinkUserID || creatorOfContract(s, contract, cUserID)) && contract.SRData.LegReactionMessageID == "" {
		// Indicate that the Sink is starting to kick users
		str := "**Starting to kick users.** Swap shiny artifacts if you need to force a server sync.\n"
		str += contract.Boosters[contract.SRData.CrtSinkUserID].Mention + " will react here with 💃 after kicks to advance the tango."
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
	if cUserID == contract.SRData.CrtSinkUserID || creatorOfContract(s, contract, cUserID) {
		str := "Kicks completed."
		contract.SRData.CurrentLeg++ // Move to the next leg
		contract.SRData.LegReactionMessageID = ""
		contract.SRData.ChickenRunCheckMsgID = ""
		contract.SRData.NeedToRunChickens = nil
		if contract.SRData.CurrentLeg >= contract.SRData.Legs {
			contract.SRData.SpeedrunState = SpeedrunStateBoosting
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

func buttonReactionHelp(s *discordgo.Session, i *discordgo.InteractionCreate, contract *Contract) {

	outputStr := "## Boost Bot Icon Meanings\n\n"
	outputStr += "See 📌 message to join the contract.\nSet your number of boost tokens there or "
	outputStr += "add a 4️⃣ to 🔟 reaction to the boost list message.\n"
	outputStr += "Active booster reaction of " + boostIcon + " to when spending tokens to boost. Multiple " + boostIcon + " votes by others in the contract will also indicate a boost.\n"
	outputStr += "Farmers react with " + contract.TokenStr + " when sending tokens.\n"
	//outputStr += "Active Booster can react with ➕ or ➖ to adjust number of tokens needed.\n"
	outputStr += "Active booster reaction of 🔃 to exchange position with the next booster.\n"
	outputStr += "Reaction of ⤵️ to move yourself to last in the current boost order.\n"
	outputStr += "Reaction of 🐓 when you're ready for others to run chickens on your farm.\n"
	outputStr += "Anyone can add a 🚽 reaction to express your urgency to boost next.\n"
	outputStr += "Additional help through the **/help** command.\n"

	_, err := s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Content: outputStr,
		})
	if err != nil {
		log.Print(err)
	}
}

func addContractReactionsButtons(s *discordgo.Session, contract *Contract, channelID string, messageID string, tokenStr string) {
	if contract.buttonComponents == nil {
		compVals := make(map[string]CompMap, 14)
		compVals[boostIconReaction] = CompMap{Emoji: boostIconReaction, Style: discordgo.SecondaryButton, CustomID: "rc_#Boost#"}
		compVals[tokenStr] = CompMap{Emoji: strings.Split(tokenStr, ":")[0], ID: strings.Split(tokenStr, ":")[1], Style: discordgo.SecondaryButton, CustomID: "rc_#Token#"}
		compVals["💰"] = CompMap{Emoji: "💰", Style: discordgo.SecondaryButton, CustomID: "rc_#bag#"}
		compVals["🚚"] = CompMap{Emoji: "🚚", Style: discordgo.SecondaryButton, CustomID: "rc_#truck#"}
		compVals["💃"] = CompMap{Emoji: "💃", Style: discordgo.SecondaryButton, CustomID: "rc_#tango#"}
		compVals["🦵"] = CompMap{Emoji: "🦵", Style: discordgo.SecondaryButton, CustomID: "rc_#leg#"}
		compVals["🔃"] = CompMap{Emoji: "🔃", Style: discordgo.SecondaryButton, CustomID: "rc_#swap#"}
		compVals["⤵️"] = CompMap{Emoji: "⤵️", Style: discordgo.SecondaryButton, CustomID: "rc_#last#"}
		compVals["🐓"] = CompMap{Emoji: "🐓", Style: discordgo.SecondaryButton, CustomID: "rc_#cr#"}
		compVals["✅"] = CompMap{Emoji: "✅", Style: discordgo.SecondaryButton, CustomID: "rc_#check#"}
		compVals["❓"] = CompMap{Emoji: "❓", Style: discordgo.SecondaryButton, CustomID: "rc_#help#"}
		for i, el := range contract.AltIcons {
			compVals[el] = CompMap{Emoji: el, Style: discordgo.SecondaryButton, CustomID: fmt.Sprintf("rc_#alt-%d#", i)}
		}
		contract.buttonComponents = compVals
	}

	compVals := contract.buttonComponents

	iconsRow := make([][]string, 2)
	iconsRow[0], iconsRow[1] = addContractReactionsGather(contract, tokenStr)
	if len(iconsRow[1]) > 5 {
		iconsRow[2] = iconsRow[1][5:] // Grab overflow icons to new row
		iconsRow[1] = iconsRow[1][:5] // Limit this row to 5 icons
		iconsRow[2] = iconsRow[2][:5] // Limit the number of alt icons to 5
	}

	// Alt icons can go on a second action row
	//icons = append(icons, contract.AltIcons...)
	out := []discordgo.MessageComponent{}
	for _, row := range iconsRow {
		var mComp []discordgo.MessageComponent
		for _, el := range row {
			mComp = append(mComp, discordgo.Button{
				//Label: "Send a Token",
				Emoji: &discordgo.ComponentEmoji{
					Name: compVals[el].Emoji,
					ID:   compVals[el].ID,
				},
				Style:    compVals[el].Style,
				CustomID: compVals[el].CustomID + contract.ContractHash,
			})
		}

		actionRow := discordgo.ActionsRow{Components: mComp}
		if len(actionRow.Components) > 0 {
			out = append(out, actionRow)
		}
	}

	msgedit := discordgo.NewMessageEdit(channelID, messageID)
	msgedit.Components = &out

	_, err := s.ChannelMessageEditComplex(msgedit)
	if err != nil {
		log.Println(err)
	}
}

func addSpeedrunContractReactionsButtons(contract *Contract, tokenStr string) ([]string, []string) {
	iconsRowA := []string{}
	iconsRowB := []string{} //mainly for alt icons

	if contract.SRData.SpeedrunState == SpeedrunStateCRT {
		iconsRowA = append(iconsRowA, []string{tokenStr, "✅", "🚚", "🦵"}...)
		iconsRowB = append(iconsRowB, contract.AltIcons...)
	}
	if contract.SRData.SpeedrunState == SpeedrunStateBoosting {
		iconsRowA = append(iconsRowA, []string{tokenStr, "🐓", "💰"}...)
		iconsRowB = append(iconsRowB, contract.AltIcons...)
	}
	if contract.SRData.SpeedrunState == SpeedrunStatePost {
		iconsRowA = append(iconsRowA, []string{tokenStr, "🐓"}...)
		iconsRowB = append(iconsRowB, contract.AltIcons...)
	}
	return iconsRowA, iconsRowB
}

func addContractReactionsGather(contract *Contract, tokenStr string) ([]string, []string) {

	iconsRowA := []string{}
	iconsRowB := []string{} //mainly for alt icons

	if contract.Speedrun {
		switch contract.SRData.SpeedrunState {
		case SpeedrunStateCRT:
			return addSpeedrunContractReactionsButtons(contract, tokenStr)
		case SpeedrunStateBoosting:
			if contract.SRData.SpeedrunStyle == SpeedrunStyleWonky {
				return addSpeedrunContractReactionsButtons(contract, tokenStr)
			}
		case SpeedrunStatePost:
			return addSpeedrunContractReactionsButtons(contract, tokenStr)
		default:
			break
		}
	}

	if contract.State == ContractStateStarted {
		iconsRowA = append(iconsRowA, []string{boostIconReaction, tokenStr, "🔃", "⤵️", "🐓"}...)
		iconsRowB = append(iconsRowB, contract.AltIcons...)
	}
	if contract.State == ContractStateWaiting || contract.State == ContractStateCompleted {
		if contract.VolunteerSink != "" {
			iconsRowA = append(iconsRowA, tokenStr)
			iconsRowB = append(iconsRowB, contract.AltIcons...)
		}
		iconsRowA = append(iconsRowA, "🐓")
	}

	if len(iconsRowA) < 5 {
		iconsRowA = append(iconsRowA, "❓")
	} else {
		iconsRowB = append(iconsRowB, "❓")
	}
	return iconsRowA, iconsRowB
}
