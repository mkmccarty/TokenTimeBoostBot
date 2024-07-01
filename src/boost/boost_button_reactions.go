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
		if contract.Speedrun && (contract.State == ContractStateCRT ||
			contract.SRData.SpeedrunStyle == SpeedrunStyleBanker && contract.SRData.SpeedrunState == SpeedrunStateBoosting ||
			contract.SRData.SpeedrunState == SpeedrunStatePost) {

			if contract.State == ContractStateCRT {
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

func addContractReactionsButtons(s *discordgo.Session, contract *Contract, channelID string, messageID string) {
	if contract.buttonComponents == nil {
		compVals := make(map[string]CompMap, 14)
		compVals[boostIconReaction] = CompMap{Emoji: boostIconReaction, Style: discordgo.SecondaryButton, CustomID: "rc_#Boost#"}
		compVals[contract.TokenStr] = CompMap{Emoji: strings.Split(contract.TokenReactionStr, ":")[0], ID: strings.Split(contract.TokenReactionStr, ":")[1], Style: discordgo.SecondaryButton, CustomID: "rc_#Token#"}
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
	iconsRow[0], iconsRow[1] = addContractReactionsGather(contract, contract.TokenStr)
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

func addReactionButtonsSinkPost(contract *Contract) ([]string, []string) {
	iconsRowA := []string{}
	iconsRowB := []string{} //mainly for alt icons
	iconsRowA = append(iconsRowA, []string{contract.TokenStr, "🐓"}...)
	iconsRowB = append(iconsRowB, contract.AltIcons...)
	return iconsRowA, iconsRowB
}

func addSpeedrunContractReactionsButtons(contract *Contract) ([]string, []string) {
	iconsRowA := []string{}
	iconsRowB := []string{} //mainly for alt icons

	if contract.State == ContractStateCRT {
		return addReactionButtonsCRT(contract)
	}
	if contract.SRData.SpeedrunState == SpeedrunStateBoosting {
		return addReactionButtonsBanker(contract)
	}
	if contract.SRData.SpeedrunState == SpeedrunStatePost {
		return addReactionButtonsSinkPost(contract)
	}
	return iconsRowA, iconsRowB
}

func addContractReactionsGather(contract *Contract, tokenStr string) ([]string, []string) {

	iconsRowA := []string{}
	iconsRowB := []string{} //mainly for alt icons

	if contract.Speedrun {
		switch contract.State {
		case ContractStateCRT:
			return addSpeedrunContractReactionsButtons(contract)
		default:

			switch contract.SRData.SpeedrunState {
			case SpeedrunStateBoosting:
				if contract.SRData.SpeedrunStyle == SpeedrunStyleBanker {
					return addSpeedrunContractReactionsButtons(contract)
				}
			case SpeedrunStatePost:
				return addSpeedrunContractReactionsButtons(contract)
			default:
				break
			}
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
