package boost

import (
	"fmt"
	"log"
	"slices"
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
	log.Print(cmd, contractHash)

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
			compVals[el] = CompMap{Emoji: el, Style: discordgo.SecondaryButton, CustomID: fmt.Sprintf("rc_#Alt-%d#", i)}

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
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
	})

	redraw := false

	switch cmd {
	case "boost":
		redraw = buttonReactionBoost(s, i, contract, userID)

	case "token":
		// Token also needs to handle the alt icons
		/*
			userID := r.UserID
			// Special handling for alt icons representing token reactions
			if slices.Index(contract.AltIcons, r.Emoji.Name) != -1 {
				idx := slices.Index(contract.Boosters[r.UserID].AltsIcons, r.Emoji.Name)
				if idx != -1 {
					userID = contract.Boosters[r.UserID].Alts[idx]
					tokenReactionStr = r.Emoji.Name
				}
			}
		*/

		// Help button
		redraw = buttonReactionToken(s, i, contract, userID)

	case "swap":
		redraw = buttonReactionSwap(s, i, contract, userID)
	case "last":
		redraw = buttonReactionLast(s, i, contract, userID)
	case "cr":
		redraw = buttonReactionRunChickens(s, contract, userID)
	case "ranchicken":
		buttonReactionRanChicken(s, i, contract, userID)
	}

	if redraw {
		refreshBoostListMessage(s, contract)
	}
}

func buttonReactionBoost(s *discordgo.Session, i *discordgo.InteractionCreate, contract *Contract, cUserID string) bool {
	if contract.State != ContractStateSignup && contract.BoostPosition < len(contract.Order) {
		// If Rocket reaction on Boost List, only that boosting user can apply a reaction
		if contract.State == ContractStateStarted {
			//var votingElection = (msg.Reactions[0].Count - 1) >= 2
			var votingElection = false

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

			if userID == contract.Order[contract.BoostPosition] || votingElection || creatorOfContract(s, contract, userID) {
				_ = Boosting(s, i.GuildID, i.ChannelID)
				return false
			}
		}

	}

	return false
}

func buttonReactionToken(s *discordgo.Session, i *discordgo.InteractionCreate, contract *Contract, fromUserID string) bool {
	if contract.State == ContractStateWaiting || contract.State == ContractStateCompleted {
		if contract.VolunteerSink != "" {
			rSerial := xid.New().String()
			sSerial := xid.New().String()

			sink := contract.Boosters[contract.VolunteerSink]
			sink.Received = append(sink.Received, TokenUnit{Time: time.Now(), Value: 0.0, UserID: contract.Boosters[fromUserID].Nick, Serial: rSerial})
			track.ContractTokenMessage(s, i.ChannelID, sink.UserID, track.TokenReceived, 1, fromUserID, rSerial)
			// Record who sent the token
			contract.Boosters[fromUserID].Sent = append(contract.Boosters[fromUserID].Sent, TokenUnit{Time: time.Now(), Value: 0.0, UserID: contract.Boosters[sink.UserID].Nick, Serial: sSerial})
			track.ContractTokenMessage(s, i.ChannelID, fromUserID, track.TokenSent, 1, sink.UserID, sSerial)
		}
	} else if contract.BoostPosition < len(contract.Order) {
		var b = contract.Boosters[contract.Order[contract.BoostPosition]]

		b.TokensReceived++
		if fromUserID != b.UserID {
			// Record the Tokens as received
			rSerial := xid.New().String()
			sSerial := xid.New().String()
			b.Received = append(b.Received, TokenUnit{Time: time.Now(), Value: 0.0, UserID: contract.Boosters[fromUserID].Nick, Serial: rSerial})
			track.ContractTokenMessage(s, i.ChannelID, b.UserID, track.TokenReceived, 1, contract.Boosters[fromUserID].Nick, rSerial)

			// Record who sent the token
			if contract.Boosters[fromUserID] != nil {
				// Make sure this isn't an admin user who's sending on behalf of an alt
				contract.Boosters[fromUserID].Sent = append(contract.Boosters[fromUserID].Sent, TokenUnit{Time: time.Now(), Value: 0.0, UserID: b.Nick, Serial: sSerial})
			}
			track.ContractTokenMessage(s, i.ChannelID, fromUserID, track.TokenSent, 1, b.Nick, sSerial)
		} else {
			b.TokensFarmedTime = append(b.TokensFarmedTime, time.Now())
			track.FarmedToken(s, i.ChannelID, fromUserID)
		}

		if b.TokensReceived >= b.TokensWanted && fromUserID == b.Name && b.AltController == "" {
			// Guest farmer auto boosts
			_ = Boosting(s, i.GuildID, i.ChannelID)
			return false
		}
		return true
	}

	return false
}

func buttonReactionLast(s *discordgo.Session, i *discordgo.InteractionCreate, contract *Contract, cUserID string) bool {
	var uid = cUserID
	if contract.Boosters[uid].BoostState == BoostStateTokenTime {
		currentBoosterPosition := findNextBooster(contract)
		err := MoveBooster(s, i.GuildID, i.ChannelID, contract.CreatorID[0], uid, len(contract.Order), currentBoosterPosition == -1)
		if err == nil && currentBoosterPosition != -1 {
			_ = ChangeCurrentBooster(s, i.GuildID, i.ChannelID, contract.CreatorID[0], contract.Order[currentBoosterPosition], true)
			return false
		}
	} else if contract.Boosters[uid].BoostState == BoostStateUnboosted {
		_ = MoveBooster(s, i.GuildID, i.ChannelID, contract.CreatorID[0], uid, len(contract.Order), true)
	}

	return false
}

func buttonReactionSwap(s *discordgo.Session, i *discordgo.InteractionCreate, contract *Contract, cUserID string) bool {
	// Reaction for current booster to change places
	if contract.State == ContractStateStarted && contract.Boosters[cUserID].BoostState == BoostStateTokenTime {
		if (contract.BoostPosition + 1) < len(contract.Order) {
			_ = SkipBooster(s, i.GuildID, i.ChannelID, "")
		}
	}
	/*
		if contract.State == ContractStateWaiting {
			contract.State = ContractStateCompleted
			contract.EndTime = time.Now()
			sendNextNotification(s, contract, true)
		}
	*/
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

	if !strings.Contains(strings.Split(str, "\n")[1], userMention) {
		str += " " + contract.Boosters[cUserID].Mention
		msgedit.SetContent(str)
		msgedit.Flags = discordgo.MessageFlagsSuppressNotifications
		_, _ = s.ChannelMessageEditComplex(msgedit)
	}
}

func buttonReactionHelp(s *discordgo.Session, i *discordgo.InteractionCreate, contract *Contract) {

	outputStr := "## Boost Bot Icon Meanings\n\n"
	outputStr += "See 📌 message to join the contract.\nSet your number of boost tokens there or "
	outputStr += "add a 4️⃣ to 🔟 reaction to the boost list message.\n"
	outputStr += "Active booster reaction of " + boostIcon + " to when spending tokens to boost. Multiple " + boostIcon + " votes by others in the contract will also indicate a boost.\n"
	outputStr += "Farmers react with " + contract.Location[0].TokenStr + " when sending tokens.\n"
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

	iconsRow := make([][]string, 2)
	iconsRow[0], iconsRow[1] = addContractReactionsGather(contract, tokenStr)
	type CompMap struct {
		Emoji    string
		ID       string
		Style    discordgo.ButtonStyle
		CustomID string
	}

	compVals := make(map[string]CompMap, 12)
	compVals[boostIconReaction] = CompMap{Emoji: boostIconReaction, Style: discordgo.SecondaryButton, CustomID: "rc_#Boost#"}
	compVals[tokenStr] = CompMap{Emoji: strings.Split(tokenStr, ":")[0], ID: strings.Split(tokenStr, ":")[1], Style: discordgo.SecondaryButton, CustomID: "rc_#Token#"}
	compVals["💰"] = CompMap{Emoji: "💰", Style: discordgo.SecondaryButton, CustomID: "rc_#Bag#"}
	compVals["🚚"] = CompMap{Emoji: "🚚", Style: discordgo.SecondaryButton, CustomID: "rc_#Truck#"}
	compVals["💃"] = CompMap{Emoji: "💃", Style: discordgo.SecondaryButton, CustomID: "rc_#Tango#"}
	compVals["🦵"] = CompMap{Emoji: "🦵", Style: discordgo.SecondaryButton, CustomID: "rc_#Leg#"}
	compVals["🔃"] = CompMap{Emoji: "🔃", Style: discordgo.SecondaryButton, CustomID: "rc_#Swap#"}
	compVals["⤵️"] = CompMap{Emoji: "⤵️", Style: discordgo.SecondaryButton, CustomID: "rc_#Last#"}
	compVals["🐓"] = CompMap{Emoji: "🐓", Style: discordgo.SecondaryButton, CustomID: "rc_#CR#"}
	compVals["✅"] = CompMap{Emoji: "✅", Style: discordgo.SecondaryButton, CustomID: "rc_#Check#"}
	compVals["❓"] = CompMap{Emoji: "❓", Style: discordgo.SecondaryButton, CustomID: "rc_#Help#"}
	for i, el := range contract.AltIcons {
		compVals[el] = CompMap{Emoji: el, Style: discordgo.SecondaryButton, CustomID: fmt.Sprintf("rc_#Alt-%d#", i)}
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
