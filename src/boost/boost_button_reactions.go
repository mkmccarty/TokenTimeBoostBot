package boost

import (
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/track"
	"github.com/rs/xid"
)

// HandleContractReactions handles all the button reactions for a contract
func HandleContractReactions(s *discordgo.Session, i *discordgo.InteractionCreate) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
		Data: &discordgo.InteractionResponseData{
			Content:    "",
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})
	userID := getInteractionUserID(i)

	// rc_Name # rc_ID # HASH
	reaction := strings.Split(i.MessageComponentData().CustomID, "#")
	cmd := strings.ToLower(reaction[1])
	contractHash := reaction[len(reaction)-1]

	contract := Contracts[contractHash]
	if contract == nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Unable to find this contract.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	if cmd == "help" {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{})
		buttonReactionHelp(s, i, contract)
		return
	}

	// Restring commands to those within the contract
	if !userInContract(contract, userID) && !creatorOfContract(s, contract, userID) {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "User isn't in this contract.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}
	// Ack the message for every other command
	if cmd != "cr" {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{})
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
		_, redraw = buttonReactionToken(s, i.GuildID, i.ChannelID, contract, userID, 1, "")
	case "2token":
		_, redraw = buttonReactionToken(s, i.GuildID, i.ChannelID, contract, userID, 2, "")
	case "swap":
		redraw = buttonReactionSwap(s, i.GuildID, i.ChannelID, contract, userID)
	case "last":
		_, redraw = buttonReactionLast(s, i.GuildID, i.ChannelID, contract, userID)
	case "cr":
		var str string
		redraw, str = buttonReactionRunChickens(s, contract, userID)
		// Ack the message for every other command
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: str,
			Flags:   discordgo.MessageFlagsEphemeral,
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
	votingElection := false
	if contract.State != ContractStateFastrun {
		panic("The boost option is only available during fastrun contracts")
	}

	userID := cUserID
	if contract.Boosters[cUserID] != nil && len(contract.Boosters[cUserID].Alts) > 0 {
		// Find the most recent boost time among the user and their alts
		for _, altID := range contract.Boosters[cUserID].Alts {
			if altID == contract.Order[contract.BoostPosition] {
				userID = altID
				break
			}
		}
	}

	if userID != contract.Order[contract.BoostPosition] {
		b := contract.Boosters[contract.Order[contract.BoostPosition]]
		// TODO: This is currently not a unique list of userID's, maybe needs to be a unqiue insert,
		// but it's not a big deal in practice.
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
		_ = Boosting(s, GuildID, ChannelID)
		return true
	}
	return redraw
}

func buttonReactionToken(s *discordgo.Session, GuildID string, ChannelID string, contract *Contract, fromUserID string, count int, alternateBooster string) (bool, bool) {
	if !userInContract(contract, fromUserID) {
		return false, false
	}

	// See if we have a banker for this token
	bankerID := contract.Banker.CurrentBanker

	// Identify the recipient of this token
	var b *Booster
	if bankerID != "" {
		b = contract.Boosters[bankerID]
	} else if contract.BoostPosition < len(contract.Order) {
		b = contract.Boosters[contract.Order[contract.BoostPosition]]
		// When not using a banker, adjust the boost countdown variable
	}
	if alternateBooster != "" {
		b = contract.Boosters[alternateBooster]
	}

	if b != nil {
		if fromUserID != b.UserID {
			// Record the Tokens as received
			tokenSerial := xid.New().String()
			now := time.Now()

			track.ContractTokenMessage(s, ChannelID, b.UserID, track.TokenReceived, count, contract.Boosters[fromUserID].Nick, tokenSerial, now)

			// Record who sent the token
			track.ContractTokenMessage(s, ChannelID, fromUserID, track.TokenSent, count, b.Nick, tokenSerial, now)
			contract.mutex.Lock()
			b.TokensReceived += count
			contract.TokenLog = append(contract.TokenLog, ei.TokenUnitLog{Time: now, Quantity: count, FromUserID: fromUserID, FromNick: contract.Boosters[fromUserID].Nick, ToUserID: b.UserID, ToNick: b.Nick, Serial: tokenSerial, Boost: false})
			contract.mutex.Unlock()
			tval := bottools.GetTokenValue(time.Since(contract.StartTime).Seconds(), contract.EstimatedDuration.Seconds())
			contract.mutex.Lock()
			contract.Boosters[fromUserID].TokenValue += tval * float64(count)
			contract.Boosters[b.UserID].TokenValue -= tval * float64(count)
			contract.mutex.Unlock()
			if contract.BoostOrder == ContractOrderTVal {
				reorderBoosters(contract)
			}
			if contract.Style&ContractFlagDynamicTokens != 0 {
				// Determine the dynamic tokens
				determineDynamicTokens(contract)
			}
		} else {
			track.FarmedToken(s, ChannelID, fromUserID, count)
			contract.mutex.Lock()
			b.TokensReceived += count
			contract.TokenLog = append(contract.TokenLog, ei.TokenUnitLog{Time: time.Now(), Quantity: count, FromUserID: fromUserID, FromNick: contract.Boosters[fromUserID].Nick, ToUserID: fromUserID, ToNick: contract.Boosters[fromUserID].Nick, Serial: xid.New().String(), Boost: false})
			contract.mutex.Unlock()
			reorderBoosters(contract)
		}
		if b.TokensReceived == b.TokensWanted {
			b.BoostingTokenTimestamp = time.Now()
		}

		// Only auto boost Fastrun guest farmers
		if contract.State == ContractStateFastrun &&
			b.Name == b.UserID &&
			b.TokensReceived >= b.TokensWanted &&
			b.AltController == "" {
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
	// make sure uid is in the contract
	if !userInContract(contract, uid) {
		return false, false
	}

	switch contract.Boosters[uid].BoostState {
	case BoostStateTokenTime:
		currentBoosterPosition := findNextBooster(contract)
		err := MoveBooster(s, GuildID, ChannelID, contract.CreatorID[0], uid, len(contract.Order), currentBoosterPosition == -1)
		if err == nil && currentBoosterPosition != -1 {
			_ = ChangeCurrentBooster(s, GuildID, ChannelID, contract.CreatorID[0], contract.Order[currentBoosterPosition], true)
			return true, false
		}
	case BoostStateUnboosted:
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

func buttonReactionRunChickens(s *discordgo.Session, contract *Contract, cUserID string) (bool, string) {
	userID := cUserID
	var str string

	if !userInContract(contract, cUserID) {
		return false, "You are not in this contract."
	}

	// Indicate that a farmer is ready for chicken runs
	if len(contract.Boosters[cUserID].Alts) > 0 {
		ids := append(contract.Boosters[cUserID].Alts, cUserID)
		for _, id := range contract.Order {
			if slices.Index(ids, id) != -1 {
				alt := contract.Boosters[id]
				userID = id
				if alt.BoostState == BoostStateBoosted && alt.RunChickensTime.IsZero() {
					break
				}
			}
		}
	}

	if contract.Boosters[userID].BoostState == BoostStateBoosted && !contract.Boosters[userID].RunChickensTime.IsZero() {
		// Already asked for chicken runs
		return false, "You've already asked for Chicken Runs, if you have an alternate use `/link-alternate` to link them to your main account and then ask for chicken runs."
	}

	if contract.Boosters[userID].BoostState == BoostStateBoosted && contract.Boosters[userID].RunChickensTime.IsZero() {

		contract.Boosters[userID].RunChickensTime = time.Now()

		var name = contract.Boosters[userID].Nick
		var einame = farmerstate.GetEggIncName(userID)
		if einame != "" {
			name += " " + einame
		}
		color := contract.Boosters[userID].Color
		if color == 0 {
			color = 0x00cc00
		}
		go func() {
			for _, location := range contract.Location {
				str := fmt.Sprintf("%s **%s** is ready for chicken runs, check for incoming trucks before visiting.", location.RoleMention, contract.Boosters[userID].Mention)
				var data discordgo.MessageSend
				data.Content = str
				data.Embeds = []*discordgo.MessageEmbed{
					{
						Title:       fmt.Sprintf("%s Runners", name),
						Description: "",
						Color:       color,
					},
				}
				data.Components = []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.Button{
								Emoji:    ei.GetBotComponentEmoji("icon_chicken_run"),
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
		str = "You've asked for Chicken Runs, now what...\n...\nMaybe.. check on your habs and gusset?  \nI'm sure you've already forced a game sync so no need to remind about that.\nMaybe self-runs?"
		return true, str
	}
	return false, fmt.Sprintf("You cannot request chicken runs as **%s** hasen't boosted yet.", contract.Boosters[userID].Nick)
}

func buttonReactionRanChicken(s *discordgo.Session, i *discordgo.InteractionCreate, contract *Contract, cUserID string) {
	if !userInContract(contract, cUserID) {
		// Ignore if the user isn't in the contract
		return
	}
	contract.mutex.Lock()
	defer contract.mutex.Unlock()

	//log.Print("Ran Chicken")
	msgedit := discordgo.NewMessageEdit(i.ChannelID, i.Message.ID)

	str := i.Message.Embeds[0].Description

	userMention := contract.Boosters[cUserID].Mention
	repost := false

	if !strings.Contains(str, userMention) {
		str += " " + contract.Boosters[cUserID].Mention
		repost = true
	} else if len(contract.Boosters[cUserID].Alts) > 0 {
		for _, altID := range contract.Boosters[cUserID].Alts {
			if !strings.Contains(str, contract.Boosters[altID].Mention) {
				str += " " + contract.Boosters[altID].Mention
				repost = true
				break
			}
		}
	}
	if repost {
		//msgedit.SetContent(str)
		embeds := []*discordgo.MessageEmbed{
			{
				Title:       i.Message.Embeds[0].Title,
				Description: str,
				Color:       i.Message.Embeds[0].Color,
			},
		}
		msgedit.SetEmbeds(embeds)
		msgedit.Flags = discordgo.MessageFlagsSuppressNotifications
		_, _ = s.ChannelMessageEditComplex(msgedit)
	}
}

func remove(s []string, i int) []string {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

func buttonReactionHelp(s *discordgo.Session, i *discordgo.InteractionCreate, contract *Contract) {
	chickMention, _, _ := ei.GetBotEmoji("runready")
	var outputStr strings.Builder
	// Each of the contract play styles has a link that descibes them, Lets print that
	//	if i.GuildID == "485162044652388384" {
	switch contract.PlayStyle {
	case ContractPlaystyleChill:
		outputStr.WriteString("## [Chill Playstyle](https://discord.com/channels/485162044652388384/1386391295869849681/1386598237380804661)\n")
	case ContractPlaystyleACOCooperative:
		outputStr.WriteString("## [ACO Cooperative Playstyle](https://discord.com/channels/485162044652388384/1386391295869849681/1386598298907050067)\n")
	case ContractPlaystyleFastrun:
		outputStr.WriteString("## [Fastrun Playstyle](https://discord.com/channels/485162044652388384/1386391295869849681/1386598380855365784)\n")
	case ContractPlaystyleLeaderboard:
		outputStr.WriteString("## [Leaderboard Playstyle](https://discord.com/channels/485162044652388384/1386391295869849681/1386598461184544818)\n")
	case ContractPlaystyleUnset:
		// No playstyle set, so no link
	}
	//	}
	outputStr.WriteString("## Boost Bot Icon Meanings\n\n")
	outputStr.WriteString("See 📌 message to join the contract.\nSet your number of boost tokens there or ")
	outputStr.WriteString("add a 4️⃣ to 🔟 reaction to the boost list message.\n")
	outputStr.WriteString("Active booster reaction of " + boostIcon + " to when spending tokens to boost. Multiple " + boostIcon + " votes by others in the contract will also indicate a boost.\n")
	outputStr.WriteString("Use " + contract.TokenStr + " when sending tokens. ")
	outputStr.WriteString("During GG use " + ei.GetBotEmojiMarkdown("std_gg") + "/" + ei.GetBotEmojiMarkdown("ultra_gg") + " to send 2 tokens.\n")
	outputStr.WriteString(fmt.Sprintf("Farmer status line, %s:Requested Run, %s:10B Est, %s: Full Hab Est.\n", ei.GetBotEmojiMarkdown("icon_chicken_run"), ei.GetBotEmojiMarkdown("trophy_diamond"), ei.GetBotEmojiMarkdown("fullhab")))
	//outputStr.WriteString("Active Booster can react with ➕ or ➖ to adjust number of tokens needed.\n")
	outputStr.WriteString("Active booster reaction of 🔃 to exchange position with the next booster.\n")
	outputStr.WriteString("Reaction of ⤵️ to move yourself to last in the current boost order.\n")
	outputStr.WriteString("Reaction of " + chickMention + " when you're ready for others to run chickens on your farm.\n")
	outputStr.WriteString("Anyone can add a 🚽 reaction to express your urgency to boost next.\n")
	outputStr.WriteString("Additional help through the **/help** command.\n")

	_, err := s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Content: outputStr.String(),
			Flags:   discordgo.MessageFlagsEphemeral,
		})
	if err != nil {
		log.Print(err)
	}
}

func getContractReactionsComponents(contract *Contract) []discordgo.MessageComponent {
	compVals := contract.buttonComponents
	if compVals == nil {
		compVals = make(map[string]CompMap, 14)
		compVals[boostIconReaction] = CompMap{Emoji: boostIconReaction, Style: discordgo.SecondaryButton, CustomID: "rc_#Boost#"}
		compVals[contract.TokenStr] = CompMap{ComponentEmoji: ei.GetBotComponentEmoji("token"), Style: discordgo.SecondaryButton, CustomID: "rc_#token#"}
		compVals["GG"] = CompMap{ComponentEmoji: ei.GetBotComponentEmoji("std_gg"), Style: discordgo.SecondaryButton, CustomID: "rc_#2token#"}
		compVals["UG"] = CompMap{ComponentEmoji: ei.GetBotComponentEmoji("ultra_gg"), Style: discordgo.SecondaryButton, CustomID: "rc_#2token#"}
		compVals["💰"] = CompMap{Emoji: "💰", Style: discordgo.SecondaryButton, CustomID: "rc_#bag#"}
		compVals["🚚"] = CompMap{Emoji: "🚚", Style: discordgo.SecondaryButton, CustomID: "rc_#truck#"}
		compVals["💃"] = CompMap{Emoji: "💃", Style: discordgo.SecondaryButton, CustomID: "rc_#tango#"}
		compVals["🦵"] = CompMap{Emoji: "🦵", Style: discordgo.SecondaryButton, CustomID: "rc_#leg#"}
		compVals["🔃"] = CompMap{Emoji: "🔃", Style: discordgo.SecondaryButton, CustomID: "rc_#swap#"}
		compVals["⤵️"] = CompMap{Emoji: "⤵️", Style: discordgo.SecondaryButton, CustomID: "rc_#last#"}
		compVals["🐓"] = CompMap{ComponentEmoji: ei.GetBotComponentEmoji("runready"), Style: discordgo.SecondaryButton, CustomID: "rc_#cr#"}
		compVals["✅"] = CompMap{Emoji: "✅", Style: discordgo.SecondaryButton, CustomID: "rc_#check#"}
		compVals["❓"] = CompMap{Emoji: "❓", Style: discordgo.SecondaryButton, CustomID: "rc_#help#"}
		for i, el := range contract.AltIcons {
			name := ""
			for _, booster := range contract.Boosters {
				idx := slices.Index(booster.AltsIcons, el)
				if idx != -1 {
					name = booster.Alts[idx]
				}
			}
			compVals[el] = CompMap{Emoji: el, Name: name, Style: discordgo.SecondaryButton, CustomID: fmt.Sprintf("rc_#alt-%d#", i)}
		}
		contract.buttonComponents = compVals
	}

	iconsRow := make([][]string, 5)
	iconsRow[0], iconsRow[1] = addContractReactionsGather(contract, contract.TokenStr)
	if len(iconsRow[0]) > 5 {
		iconsRow[1] = append([]string{iconsRow[0][len(iconsRow[0])-1]}, iconsRow[1]...)
		iconsRow[0] = iconsRow[0][:len(iconsRow[0])-1]
	}
	if len(iconsRow[1]) > 5 {
		iconsRow[2] = iconsRow[1][5:] // Grab overflow icons to new row
		iconsRow[1] = iconsRow[1][:5] // Limit this row to 5 icons
		if len(iconsRow[2]) > 5 {
			iconsRow[3] = iconsRow[2][5:] // Grab overflow icons to new row
			iconsRow[2] = iconsRow[2][:5] // Limit the number of alt icons to 5
			iconsRow[3] = iconsRow[3][:5] // Limit the number of alt icons to 5
		}
	}

	// Alt icons can go on a second action row
	//icons = append(icons, contract.AltIcons...)
	out := []discordgo.MessageComponent{}

	menuOptions := []discordgo.SelectMenuOption{}
	/*
		menuOptions = append(menuOptions, discordgo.SelectMenuOption{
			Label:       "Send 2 Tokens",
			Description: "Sent 2 tokens to the current booster.",
			Value:       "send2",
			Emoji:       ei.GetBotComponentEmoji("token"),
		})*/
	if contract.State == ContractStateCompleted {
		menuOptions = append(menuOptions, discordgo.SelectMenuOption{
			Label:       "Sync w/EI",
			Description: "Add completion timestamp.",
			Value:       "time",
			Emoji:       &discordgo.ComponentEmoji{Name: "⏱️"},
		})
	}

	if contract.State != ContractStateSignup {
		requestors := make([]string, 0, len(contract.Boosters))
		for _, booster := range contract.Boosters {
			if booster.TokenRequestFlag {
				requestors = append(requestors, booster.Nick)
				menuOptions = append(menuOptions, discordgo.SelectMenuOption{
					Label: fmt.Sprintf("Send %s a token", booster.Nick),
					Value: fmt.Sprintf("send:%s", booster.UserID),
					Emoji: ei.GetBotComponentEmoji("token"),
				})
			}
		}

		if len(requestors) == 0 {
			menuOptions = append(menuOptions, discordgo.SelectMenuOption{
				Label: "Request a token",
				Value: "want:",
				Emoji: ei.GetBotComponentEmoji("token"),
			})
			/*
				if contract.AltIcons != nil && len(contract.AltIcons) > 0 {
					menuOptions = append(menuOptions, discordgo.SelectMenuOption{
						Label:       "Request a token from an alternate",
						Description: "Select an alternate to request a token from.",
						Value:       "want-alt",
						Emoji:       ei.GetBotComponentEmoji("token"),
					})
				}
			*/
		} else {
			menuOptions = append(menuOptions, discordgo.SelectMenuOption{
				Label:       "Request a token",
				Description: fmt.Sprintf("%s can use to cancel request.", strings.Join(requestors, ", ")),
				Value:       "want:",
				Emoji:       ei.GetBotComponentEmoji("token"),
			})

		}

		if contract.State == ContractStateFastrun && contract.BoostPosition < len(contract.Order)-1 {
			b := contract.Boosters[contract.Order[contract.BoostPosition]]
			if b.TokensWanted <= b.TokensReceived {
				menuOptions = append(menuOptions, discordgo.SelectMenuOption{
					Label:       fmt.Sprintf("Send %s a token", contract.Boosters[contract.Order[contract.BoostPosition+1]].Nick),
					Description: fmt.Sprintf("Waiting on %s 🚀.", b.Nick),
					Value:       fmt.Sprintf("next:%s", contract.Order[contract.BoostPosition+1]),
					Emoji:       ei.GetBotComponentEmoji("token"),
				})
			}
		}

	}

	menuOptions = append(menuOptions, discordgo.SelectMenuOption{
		Label: "Token Log",
		Value: "tlog",
		Emoji: ei.GetBotComponentEmoji("token"),
	})
	menuOptions = append(menuOptions, discordgo.SelectMenuOption{
		Label: "Coop Tools",
		Value: "tools",
		Emoji: &discordgo.ComponentEmoji{Name: "🧰"},
	})
	menuOptions = append(menuOptions, discordgo.SelectMenuOption{
		Label: "X-Post Template",
		Value: "xpost",
		Emoji: &discordgo.ComponentEmoji{Name: "🖇️"},
	})
	menuOptions = append(menuOptions, discordgo.SelectMenuOption{
		Label: fmt.Sprintf("%s Grange", contract.Location[0].GuildContractRole.Name),
		Value: "grange",
		Emoji: &discordgo.ComponentEmoji{Name: "🧑‍🧑‍🧒‍🧒"},
	})

	/*
		menuOptions = append(menuOptions, discordgo.SelectMenuOption{
			Label:       "Lousy Breaks Thing",
			Description: "I didn't do it",
			Value:       "lousyt",
			Emoji:       &discordgo.ComponentEmoji{Name: "🔨"},
		})
		menuOptions = append(menuOptions, discordgo.SelectMenuOption{
			Label:       "Tbone something something",
			Description: "Confusion and Delay",
			Value:       "tbone",
			Emoji:       &discordgo.ComponentEmoji{Name: "🥔"},
		})
		menuOptions = append(menuOptions, discordgo.SelectMenuOption{
			Label:       "Send Sara's Tokens to the Void",
			Description: "Not never gonna show up",
			Value:       "sara",
			Emoji:       &discordgo.ComponentEmoji{Name: "🕳️"},
		})
	*/
	minValues := 0
	out = append(out, discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{
				CustomID:    "menu#" + contract.ContractHash,
				Placeholder: "Boost Menu",
				MinValues:   &minValues,
				MaxValues:   1,
				Options:     menuOptions,
			},
		},
	})

	for _, row := range iconsRow {
		var mComp []discordgo.MessageComponent
		for _, el := range row {
			// if compVals[el] is not found, it will panic
			if _, ok := compVals[el]; !ok {
				log.Printf("Warning: Missing component for %s in contract %s", el, contract.ContractHash)
				continue
			}
			if compVals[el].Emoji == "" {
				mComp = append(mComp, discordgo.Button{
					//Label: "Send a Token",
					Emoji:    compVals[el].ComponentEmoji,
					Style:    compVals[el].Style,
					CustomID: compVals[el].CustomID + contract.ContractHash,
				})

			} else {
				mComp = append(mComp, discordgo.Button{
					Label: compVals[el].Name,
					Emoji: &discordgo.ComponentEmoji{
						Name: compVals[el].Emoji,
						ID:   compVals[el].ID,
					},
					Style:    compVals[el].Style,
					CustomID: compVals[el].CustomID + contract.ContractHash,
				})
			}
		}

		actionRow := discordgo.ActionsRow{Components: mComp}
		if len(actionRow.Components) > 0 {
			out = append(out, actionRow)
		}
	}

	return out
}

/*
func addContractReactionsButtons(s *discordgo.Session, contract *Contract, channelID string, messageID string) {
	if contract.buttonComponents == nil {
		compVals := make(map[string]CompMap, 14)
		compVals[boostIconReaction] = CompMap{Emoji: boostIconReaction, Style: discordgo.SecondaryButton, CustomID: "rc_#Boost#"}
		compVals[contract.TokenStr] = CompMap{ComponentEmoji: ei.GetBotComponentEmoji("token"), Style: discordgo.SecondaryButton, CustomID: "rc_#Token#"}
		compVals["💰"] = CompMap{Emoji: "💰", Style: discordgo.SecondaryButton, CustomID: "rc_#bag#"}
		compVals["🚚"] = CompMap{Emoji: "🚚", Style: discordgo.SecondaryButton, CustomID: "rc_#truck#"}
		compVals["💃"] = CompMap{Emoji: "💃", Style: discordgo.SecondaryButton, CustomID: "rc_#tango#"}
		compVals["🦵"] = CompMap{Emoji: "🦵", Style: discordgo.SecondaryButton, CustomID: "rc_#leg#"}
		compVals["🔃"] = CompMap{Emoji: "🔃", Style: discordgo.SecondaryButton, CustomID: "rc_#swap#"}
		compVals["⤵️"] = CompMap{Emoji: "⤵️", Style: discordgo.SecondaryButton, CustomID: "rc_#last#"}
		compVals["🐓"] = CompMap{ComponentEmoji: ei.GetBotComponentEmoji("runready"), Style: discordgo.SecondaryButton, CustomID: "rc_#cr#"}
		compVals["✅"] = CompMap{Emoji: "✅", Style: discordgo.SecondaryButton, CustomID: "rc_#check#"}
		compVals["❓"] = CompMap{Emoji: "❓", Style: discordgo.SecondaryButton, CustomID: "rc_#help#"}
		for i, el := range contract.AltIcons {
			name := ""
			for _, booster := range contract.Boosters {
				idx := slices.Index(booster.AltsIcons, el)
				if idx != -1 {
					name = booster.Alts[idx]
				}
			}
			compVals[el] = CompMap{Emoji: el, Name: name, Style: discordgo.SecondaryButton, CustomID: fmt.Sprintf("rc_#alt-%d#", i)}
		}
		contract.buttonComponents = compVals
	}

	compVals := contract.buttonComponents

	iconsRow := make([][]string, 5)
	iconsRow[0], iconsRow[1] = addContractReactionsGather(contract, contract.TokenStr)
	if len(iconsRow[1]) > 5 {
		iconsRow[2] = iconsRow[1][5:] // Grab overflow icons to new row
		iconsRow[1] = iconsRow[1][:5] // Limit this row to 5 icons
		if len(iconsRow[2]) > 5 {
			iconsRow[3] = iconsRow[2][5:] // Grab overflow icons to new row
			iconsRow[2] = iconsRow[2][:5] // Limit the number of alt icons to 5
			iconsRow[3] = iconsRow[3][:5] // Limit the number of alt icons to 5
		}
	}

	// Alt icons can go on a second action row
	//icons = append(icons, contract.AltIcons...)
	out := []discordgo.MessageComponent{}
	for _, row := range iconsRow {
		var mComp []discordgo.MessageComponent
		for _, el := range row {
			if compVals[el].Emoji == "" {
				mComp = append(mComp, discordgo.Button{
					//Label: "Send a Token",
					Emoji:    compVals[el].ComponentEmoji,
					Style:    compVals[el].Style,
					CustomID: compVals[el].CustomID + contract.ContractHash,
				})

			} else {
				mComp = append(mComp, discordgo.Button{
					Label: compVals[el].Name,
					Emoji: &discordgo.ComponentEmoji{
						Name: compVals[el].Emoji,
						ID:   compVals[el].ID,
					},
					Style:    compVals[el].Style,
					CustomID: compVals[el].CustomID + contract.ContractHash,
				})
			}
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
*/

func addContractReactionsGather(contract *Contract, tokenStr string) ([]string, []string) {

	iconsRowA := []string{}
	iconsRowB := []string{} //mainly for alt icons

	switch contract.State {
	case ContractStateCRT:
		iconsRowA = append(iconsRowA, []string{tokenStr, "✅", "🚚", "🦵", "💰"}...)
		iconsRowB = append(iconsRowB, contract.AltIcons...)
	case ContractStateBanker:
		iconsRowA = append(iconsRowA, []string{tokenStr, "🐓", "💰"}...)
		iconsRowB = append(iconsRowB, contract.AltIcons...)
	case ContractStateFastrun:
		iconsRowA = append(iconsRowA, []string{boostIconReaction, tokenStr, "🔃", "⤵️", "🐓"}...)
		iconsRowB = append(iconsRowB, contract.AltIcons...)
	case ContractStateWaiting:
		sinkID := contract.Banker.CurrentBanker
		if sinkID != "" {
			iconsRowA = append(iconsRowA, tokenStr)
			iconsRowB = append(iconsRowB, contract.AltIcons...)
		}
		iconsRowA = append(iconsRowA, "🐓")

	case ContractStateCompleted:
		sinkID := contract.Banker.CurrentBanker
		if sinkID != "" {
			iconsRowA = append(iconsRowA, tokenStr)
			iconsRowB = append(iconsRowB, contract.AltIcons...)
		}
		iconsRowA = append(iconsRowA, "🐓")

	}

	gg, ugg, _ := ei.GetGenerousGiftEvent()
	if gg > 1.0 {
		if slices.Contains(iconsRowA, tokenStr) {
			idx := slices.Index(iconsRowA, tokenStr)
			iconsRowA = append(iconsRowA[:idx+1], append([]string{"GG"}, iconsRowA[idx+1:]...)...)
		} //else {
		//	iconsRowA = append(iconsRowA, "GG")
		//}
	}
	if ugg > 1.0 {
		if slices.Contains(iconsRowA, tokenStr) {
			idx := slices.Index(iconsRowA, tokenStr)
			iconsRowA = append(iconsRowA[:idx+1], append([]string{"UG"}, iconsRowA[idx+1:]...)...)
		} //else {
		//	iconsRowA = append(iconsRowA, "UG")
		//}
	}

	if len(iconsRowA) < 5 {
		iconsRowA = append(iconsRowA, "❓")
	} else {
		iconsRowB = append(iconsRowB, "❓")
	}
	return iconsRowA, iconsRowB
}
