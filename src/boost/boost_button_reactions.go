package boost

import (
	"fmt"
	"log"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

	"github.com/bwmarrin/discordgo"
	"github.com/mattn/go-runewidth"
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

	contract := FindContractByHash(contractHash)
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
		targetUserID := ""
		if len(reaction) >= 4 {
			targetUserID = reaction[2]
		}
		buttonReactionRanChicken(s, i, contract, userID, targetUserID)
	case "crping":
		buttonReactionCRPing(s, i, contract, userID)
	case "complain":
		buttonReactionComplain(s, contract, userID)
	}

	if redraw {
		refreshBoostListMessage(s, contract, false)
	}
}

func buttonReactionBoost(s *discordgo.Session, GuildID string, ChannelID string, contract *Contract, cUserID string) bool {
	// If Rocket reaction on Boost List, only that boosting user can apply a reaction
	redraw := false
	votingElection := false
	currentBoosterID := contract.currentBoosterID()
	if currentBoosterID == "" {
		return redraw
	}
	if contract.State != ContractStateFastrun {
		panic("The boost option is only available during fastrun contracts")
	}

	userID := cUserID
	if contract.Boosters[cUserID] != nil && len(contract.Boosters[cUserID].Alts) > 0 {
		// Find the most recent boost time among the user and their alts
		for _, altID := range contract.Boosters[cUserID].Alts {
			if altID == currentBoosterID {
				userID = altID
				break
			}
		}
	}

	if userID != currentBoosterID {
		b := contract.Boosters[currentBoosterID]
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

	if userID == currentBoosterID || votingElection || creatorOfContract(s, contract, cUserID) {
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
	} else if currentBoosterID := contract.currentBoosterID(); currentBoosterID != "" {
		b = contract.Boosters[currentBoosterID]
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
			/*
				if contract.Style&ContractFlagDynamicTokens != 0 {
					// Determine the dynamic tokens
					determineDynamicTokens(contract)
				}
			*/
		} else {
			contract.mutex.Lock()
			b.TokensReceived += count
			contract.TokenLog = append(contract.TokenLog, ei.TokenUnitLog{Time: time.Now(), Quantity: count, FromUserID: fromUserID, FromNick: contract.Boosters[fromUserID].Nick, ToUserID: fromUserID, ToNick: contract.Boosters[fromUserID].Nick, Serial: xid.New().String(), Boost: false})
			contract.mutex.Unlock()
			if contract.BoostOrder == ContractOrderTVal {
				reorderBoosters(contract)
			}
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
	currentID := contract.currentBoosterID()
	currentIdx := contract.currentBoosterOrderIndex()
	if currentID == "" || currentIdx < 0 {
		return false
	}
	if cUserID == currentID || creatorOfContract(s, contract, cUserID) {
		if (currentIdx + 1) < len(contract.Order) {
			_ = SkipBooster(s, GuildID, ChannelID, "")
			return true
		}
	}
	return false
}

func buttonReactionRunChickens(s *discordgo.Session, contract *Contract, cUserID string) (bool, string) {
	defer func() {
		if r := recover(); r != nil {
			contractHash := ""
			if contract != nil {
				contractHash = contract.ContractHash
			}
			log.Printf("panic recovered in buttonReactionRunChickens: panic=%v contractHash=%s userID=%s\n%s",
				r, contractHash, cUserID, string(debug.Stack()),
			)
		}
	}()

	if s == nil || contract == nil {
		log.Printf("buttonReactionRunChickens invalid input: sessionNil=%t contractNil=%t userID=%s",
			s == nil, contract == nil, cUserID,
		)
		return false, "Unable to process chicken run request right now."
	}

	userID := cUserID
	var str string

	if !userInContract(contract, cUserID) {
		return false, "You are not in this contract."
	}

	if contract.Boosters[cUserID] == nil {
		log.Printf("buttonReactionRunChickens missing booster entry for user: contractHash=%s userID=%s",
			contract.ContractHash, cUserID,
		)
		return false, "Unable to process chicken run request right now."
	}

	// Indicate that a farmer is ready for chicken runs
	if len(contract.Boosters[cUserID].Alts) > 0 {
		ids := append(contract.Boosters[cUserID].Alts, cUserID)
		for _, id := range contract.Order {
			if slices.Index(ids, id) != -1 {
				alt := contract.Boosters[id]
				if alt == nil {
					log.Printf("buttonReactionRunChickens missing alt/main booster during selection: contractHash=%s requestUserID=%s targetID=%s",
						contract.ContractHash, cUserID, id,
					)
					continue
				}
				userID = id
				if alt.BoostState == BoostStateBoosted && alt.RunChickensTime.IsZero() {
					break
				}
			}
		}
	}

	if contract.Boosters[userID] == nil {
		log.Printf("buttonReactionRunChickens missing selected booster entry: contractHash=%s requestUserID=%s selectedUserID=%s",
			contract.ContractHash, cUserID, userID,
		)
		return false, "Unable to process chicken run request right now."
	}

	if contract.Boosters[userID].BoostState == BoostStateBoosted && !contract.Boosters[userID].RunChickensTime.IsZero() {
		// Already asked for chicken runs
		return false, "You've already asked for Chicken Runs, if you have an alternate use `/link-alternate` to link them to your main account and then ask for chicken runs."
	}

	if contract.Boosters[userID].BoostState == BoostStateBoosted && contract.Boosters[userID].RunChickensTime.IsZero() {

		contract.Boosters[userID].RunChickensTime = time.Now()

		go func() {
			for _, location := range contract.Location {
				contract.mutex.Lock()
				components, _ := buildCRMessageComponents(contract, location.RoleMention)
				existingMsgID := contract.CRMessageIDs[location.ChannelID]
				contract.mutex.Unlock()

				if components == nil {
					continue
				}

				// Always post a new message to bump the CR requests
				var data discordgo.MessageSend
				data.Flags = discordgo.MessageFlagsIsComponentsV2
				data.Components = components
				// Ping everyone with the first
				data.AllowedMentions = &discordgo.MessageAllowedMentions{
					Parse: []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeRoles, discordgo.AllowedMentionTypeUsers},
				}
				newMsg, err := s.ChannelMessageSendComplex(location.ChannelID, &data)
				if err != nil {
					log.Printf("Error sending CR message: contractHash=%s channelID=%s userID=%s error=%v",
						contract.ContractHash, location.ChannelID, userID, err)
					continue
				}

				contract.mutex.Lock()
				setChickenRunMessageID(contract, location.ChannelID, newMsg.ID)
				contract.mutex.Unlock()

				if existingMsgID != "" {
					// Replace old message with a redirect to the new one
					newMsgLink := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", location.GuildID, location.ChannelID, newMsg.ID)
					movedComponents := []discordgo.MessageComponent{
						discordgo.TextDisplay{Content: fmt.Sprintf("-# Chicken Run request moved: [View updated message](%s)", newMsgLink)},
					}
					oldEdit := discordgo.NewMessageEdit(location.ChannelID, existingMsgID)
					oldEdit.Flags = discordgo.MessageFlagsIsComponentsV2
					oldEdit.AllowedMentions = &discordgo.MessageAllowedMentions{}
					oldEdit.Components = &movedComponents
					if _, err := s.ChannelMessageEditComplex(oldEdit); err != nil {
						log.Printf("Error editing old CR message: contractHash=%s channelID=%s messageID=%s error=%v",
							contract.ContractHash, location.ChannelID, existingMsgID, err)
					}
				}
			}
		}()
		str = "You've asked for Chicken Runs, now what...\n...\nMaybe.. check on your habs and gusset?\nI'm sure you've already forced a game sync so no need to remind about that."
		return true, str
	}
	return false, fmt.Sprintf("You cannot request chicken runs as **%s** hasen't boosted yet.", contract.Boosters[userID].Nick)
}

// buildChickenRunLists returns who has/hasn't run chickens for requesterUserID.
// Also returns the list of allowed mentions for message highlighting (only those who haven't run yet).
// Iterate Order (not Boosters) to keep button and mention order consistent across all requesters.
func buildChickenRunLists(contract *Contract, requesterUserID string) (alreadyRun, missing, allowedMentions []string) {
	alreadyRun = make([]string, 0, len(contract.Boosters))
	missing = make([]string, 0, len(contract.Boosters))
	allowedMentions = make([]string, 0, len(contract.Boosters))
	for _, id := range contract.Order {
		booster := contract.Boosters[id]
		if booster == nil || booster.UserID == requesterUserID {
			continue
		}

		if slices.Contains(booster.RanChickensOn, requesterUserID) {
			alreadyRun = append(alreadyRun, booster.Mention)
			continue
		}
		missing = append(missing, booster.Mention)
		// Only players who still need to run are added to AllowedMentions so their highlight is active
		notifyID := booster.UserID
		if booster.AltController != "" {
			notifyID = booster.AltController
		}
		if !slices.Contains(allowedMentions, notifyID) {
			if nb := contract.Boosters[notifyID]; nb != nil && nb.UserName != "" {
				allowedMentions = append(allowedMentions, notifyID)
			}
		}
	}
	return
}

// crMissingPct returns the percentage of boosters who haven't yet run for requesterUserID.
func crMissingPct(contract *Contract, requesterUserID string) float64 {
	ar, m, _ := buildChickenRunLists(contract, requesterUserID)
	total := len(ar) + len(m)
	if total == 0 {
		return 0
	}
	return float64(len(m)) / float64(total) * 100
}

// crColorFromPct converts a missing percentage to an accent color.
// green (0x00ff00) = no runs missing, yellow (0xffff00) = missing <= 33%, red (0xff0000) = missing > 33%
func crColorFromPct(pct float64) int {
	if pct > 33.5 {
		return 0xff0000
	}
	if pct > 0 {
		return 0xffff00
	}
	return 0x00ff00
}

// getChickenRunAccentColor returns the worst-case color across all active chicken run requesters.
func getChickenRunAccentColor(contract *Contract) int {
	worstPct := 0.0
	for _, b := range contract.Boosters {
		if b == nil || b.RunChickensTime.IsZero() {
			continue
		}
		if pct := crMissingPct(contract, b.UserID); pct > worstPct {
			worstPct = pct
		}
	}
	return crColorFromPct(worstPct)
}

// buildCRMessageComponents builds the components for the chicken run message.
// Also returns the combined set of user IDs that should receive mention highlights (players still missing runs).
func buildCRMessageComponents(contract *Contract, roleMention string) ([]discordgo.MessageComponent, []string) {
	// Collect active requesters; LB/FR use boost order, other styles use reverse order
	order := contract.Order
	if contract.PlayStyle != ContractPlaystyleLeaderboard && contract.PlayStyle != ContractPlaystyleFastrun {
		order = slices.Clone(contract.Order)
		slices.Reverse(order)
	}
	requesters := make([]string, 0, len(order))
	for _, id := range order {
		b := contract.Boosters[id]
		if b == nil || b.RunChickensTime.IsZero() {
			continue
		}
		if time.Since(b.RunChickensTime) > 10*time.Minute {
			continue
		}
		requesters = append(requesters, id)
	}
	if len(requesters) == 0 {
		return nil, nil
	}

	// Find the most recent requester by RunChickensTime
	var latestRequester *Booster
	for _, id := range requesters {
		b := contract.Boosters[id]
		if b == nil {
			continue
		}
		if latestRequester == nil || b.RunChickensTime.After(latestRequester.RunChickensTime) {
			latestRequester = b
		}
	}
	latestName := ""
	if latestRequester != nil {
		latestName = latestRequester.Nick
		if ign := farmerstate.GetMiscSettingString(latestRequester.UserID, "ei_ign"); ign != "" {
			latestName = ign
		}
	}

	pingHeader := fmt.Sprintf(
		"%s **%s** is requesting chicken runs!\n-# Check for trucks and incoming tokens before visiting.\n-# I'm sure you've already forced a game sync so no need to remind about that.",
		roleMention, latestName,
	)

	worstPct := 0.0
	containerComps := make([]discordgo.MessageComponent, 0, len(requesters)+1)
	var buttons []discordgo.MessageComponent
	var selectOptions []discordgo.SelectMenuOption
	var allAllowedMentions []string

	// Add a TextDisplay for a header
	containerComps = append(containerComps, discordgo.TextDisplay{Content: "### Active Chicken Run Requests"})

	buttonLabelCount := make(map[string]int)

	// Build a TextDisplay and button for each requester, up to 10 buttons total.
	for _, reqID := range requesters {
		booster := contract.Boosters[reqID]
		if booster == nil {
			continue
		}
		name := booster.Nick
		if ign := farmerstate.GetMiscSettingString(reqID, "ei_ign"); ign != "" {
			name = ign
		}

		alreadyRun, missing, mentionIDs := buildChickenRunLists(contract, reqID)
		for _, id := range mentionIDs {
			if !slices.Contains(allAllowedMentions, id) {
				allAllowedMentions = append(allAllowedMentions, id)
			}
		}
		total := len(alreadyRun) + len(missing)
		pct := 0.0
		if total > 0 {
			pct = float64(len(missing)) / float64(total) * 100
		}
		if pct > worstPct {
			worstPct = pct
		}

		// Drop fully-completed requesters from the message
		if len(missing) == 0 {
			continue
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "%s **%s (%d/%d)**", bottools.NumberToEmoji(len(missing)), name, len(alreadyRun), total)
		switch contract.PlayStyle {
		case ContractPlaystyleLeaderboard, ContractPlaystyleFastrun:
			if len(missing) < 11 {
				fmt.Fprintf(&sb, "\n-# _  _↳ Waiting: %s", strings.Join(missing, " "))
			}
		case ContractPlaystyleChill, ContractPlaystyleACOCooperative:
			if len(alreadyRun) >= 11 {
				fmt.Fprintf(&sb, "\n-# _  _↳ Ran: %s", bottools.NumberToEmoji(len(alreadyRun)))
			} else {
				fmt.Fprintf(&sb, "\n-# _  _↳ Ran: %s", strings.Join(alreadyRun, " "))
			}
		}
		containerComps = append(containerComps, discordgo.TextDisplay{Content: sb.String()})

		// One button per incomplete requester, max 10 total
		if len(buttons) < 10 {
			label := runewidth.Truncate(name, 4, "")
			buttonLabelCount[label]++
			// Truncating to 4 chars can produce duplicate labels, append a number to keep them distinct
			if buttonLabelCount[label] > 1 {
				label = fmt.Sprintf("%s%d", label, buttonLabelCount[label])
			}
			buttons = append(buttons, discordgo.Button{
				Emoji:    ei.GetBotComponentEmoji("icon_chicken_run"),
				Label:    label,
				Style:    discordgo.SecondaryButton,
				CustomID: fmt.Sprintf("rc_#RanChicken#%s#%s", reqID, contract.ContractHash),
			})
		}

		selectOptions = append(selectOptions, discordgo.SelectMenuOption{
			Label: name + " Runners",
			Value: reqID,
		})
	}

	// All requesters done, show a completion notice instead of an empty container
	if len(containerComps) == 1 {
		containerComps = []discordgo.MessageComponent{
			discordgo.TextDisplay{Content: "🟩 All chicken runs complete!"},
		}
	}

	accentColor := crColorFromPct(worstPct)
	components := []discordgo.MessageComponent{
		discordgo.TextDisplay{Content: pingHeader},
		discordgo.Container{
			AccentColor: &accentColor,
			Components:  containerComps,
		},
	}

	// Up to 2 rows of 5 buttons
	for i := 0; i < len(buttons); i += 5 {
		end := min(i+5, len(buttons))
		components = append(components, discordgo.ActionsRow{
			Components: buttons[i:end],
		})
	}

	// Select menu to ping remaining players for a specific requester
	if len(selectOptions) > 0 {
		components = append(components, discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					MenuType:    discordgo.StringSelectMenu,
					CustomID:    fmt.Sprintf("rc_#CRPing#%s", contract.ContractHash),
					Placeholder: "Ping remaining for...",
					Options:     selectOptions,
				},
			},
		})
	}

	return components, allAllowedMentions
}

func buttonReactionCRPing(s *discordgo.Session, i *discordgo.InteractionCreate, contract *Contract, cUserID string) {
	if contract == nil || i == nil || i.Message == nil {
		return
	}

	values := i.MessageComponentData().Values
	if len(values) == 0 {
		return
	}
	requesterUserID := values[0]

	contract.mutex.Lock()
	defer contract.mutex.Unlock()

	if contract.Boosters[requesterUserID] == nil {
		return
	}

	_, _, pingIDs := buildChickenRunLists(contract, requesterUserID)
	if len(pingIDs) == 0 {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "No players remaining.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	mentions := make([]string, 0, len(pingIDs))
	for _, id := range pingIDs {
		if nb := contract.Boosters[id]; nb != nil {
			mentions = append(mentions, nb.Mention)
		}
	}

	// Resolve requester name
	requesterName := contract.Boosters[requesterUserID].Nick
	if ign := farmerstate.GetMiscSettingString(requesterUserID, "ei_ign"); ign != "" {
		requesterName = ign
	}

	// Resolve presser (alt → controller)
	presserID := cUserID
	if pb := contract.Boosters[cUserID]; pb != nil && pb.AltController != "" {
		presserID = pb.AltController
	}
	presserMention := presserID
	if mb := contract.Boosters[presserID]; mb != nil {
		presserMention = mb.Mention
	}

	content := fmt.Sprintf(
		"Hey %s! **%s** is waiting on you to run chickens. Those chickens aren't going to run themselves! %s\n-# Ping requested by %s",
		strings.Join(mentions, " "), requesterName, ei.GetBotEmojiMarkdown("icon_chicken_run"), presserMention)
	if _, err := s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{
		Content:         content,
		AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers}},
	}); err != nil {
		log.Printf("buttonReactionCRPing send error: contractHash=%s channelID=%s requesterUserID=%s pingIDs=%v error=%v",
			contract.ContractHash, i.ChannelID, requesterUserID, pingIDs, err)
	}
}

func buttonReactionRanChicken(s *discordgo.Session, i *discordgo.InteractionCreate, contract *Contract, cUserID, requesterUserID string) {
	defer func() {
		if r := recover(); r != nil {
			msgID := ""
			channelID := ""
			if i != nil {
				channelID = i.ChannelID
				if i.Message != nil {
					msgID = i.Message.ID
				}
			}
			contractHash := ""
			if contract != nil {
				contractHash = contract.ContractHash
			}
			log.Printf("panic recovered in buttonReactionRanChicken: panic=%v contractHash=%s channelID=%s messageID=%s userID=%s\n%s",
				r, contractHash, channelID, msgID, cUserID, string(debug.Stack()),
			)
		}
	}()

	if contract == nil || i == nil || i.Message == nil {
		log.Printf("buttonReactionRanChicken invalid input: contractNil=%t interactionNil=%t messageNil=%t userID=%s",
			contract == nil, i == nil, i == nil || i.Message == nil, cUserID,
		)
		return
	}

	if requesterUserID == "" {
		log.Printf("buttonReactionRanChicken empty requesterUserID: contractHash=%s messageID=%s userID=%s",
			contract.ContractHash, i.Message.ID, cUserID)
		return
	}

	if !userInContract(contract, cUserID) {
		return
	}

	contract.mutex.Lock()
	defer contract.mutex.Unlock()

	userBooster := contract.Boosters[cUserID]
	if userBooster == nil {
		log.Printf("buttonReactionRanChicken missing booster entry for reacting user: contractHash=%s requesterUserID=%s reactingUserID=%s",
			contract.ContractHash, requesterUserID, cUserID)
		return
	}

	if contract.Boosters[requesterUserID] == nil {
		log.Printf("buttonReactionRanChicken missing booster entry for requester: contractHash=%s requesterUserID=%s reactingUserID=%s",
			contract.ContractHash, requesterUserID, cUserID)
		return
	}

	// Already ran for this requester?
	if slices.Contains(userBooster.RanChickensOn, requesterUserID) {
		return
	}

	oldColor := getChickenRunAccentColor(contract)

	// Mark the run for the current user and all their alts
	for _, id := range append([]string{cUserID}, userBooster.Alts...) {
		if id == requesterUserID {
			continue
		}
		targetBooster := contract.Boosters[id]
		if targetBooster == nil {
			log.Printf("buttonReactionRanChicken missing alt/main booster during update: contractHash=%s requesterUserID=%s reactingUserID=%s targetID=%s",
				contract.ContractHash, requesterUserID, cUserID, id)
			continue
		}
		targetBooster.RanChickensOn = append(targetBooster.RanChickensOn, requesterUserID)
	}

	newColor := getChickenRunAccentColor(contract)

	// Find role mention for this channel to rebuild the header
	var roleMention string
	for _, loc := range contract.Location {
		if loc.ChannelID == i.ChannelID {
			roleMention = loc.RoleMention
			break
		}
	}

	components, allowedMentions := buildCRMessageComponents(contract, roleMention)
	msgedit := discordgo.NewMessageEdit(i.ChannelID, i.Message.ID)
	msgedit.Flags = discordgo.MessageFlagsIsComponentsV2
	msgedit.AllowedMentions = &discordgo.MessageAllowedMentions{Users: allowedMentions}
	msgedit.Components = &components
	if _, err := s.ChannelMessageEditComplex(msgedit); err != nil {
		log.Printf("ChannelMessageEditComplex error: contractHash=%s messageID=%s error=%v",
			contract.ContractHash, i.Message.ID, err)
	}

	if newColor != oldColor {
		saveData(contract.ContractHash)
		refreshBoostListMessage(s, contract, false)
	}
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
	fmt.Fprintf(&outputStr, "Farmer status line, %s:Requested Run, %s:10B Est, %s: Full Hab Est.\n", ei.GetBotEmojiMarkdown("icon_chicken_run"), ei.GetBotEmojiMarkdown("trophy_diamond"), ei.GetBotEmojiMarkdown("fullhab"))
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

func buttonReactionComplain(s *discordgo.Session, contract *Contract, cUserID string) {
	if !userInContract(contract, cUserID) {
		return
	}

	complaint, err := ei.GetTokenComplaint(contract.Boosters[cUserID].Mention)
	if err != nil {
		log.Print(err)
		return
	}

	_, err = s.ChannelMessageSendComplex(contract.Location[0].ChannelID, &discordgo.MessageSend{
		Content: complaint,
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Parse: []discordgo.AllowedMentionType{
				discordgo.AllowedMentionTypeUsers,
			},
		},
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
		compVals["📢"] = CompMap{Emoji: "📢", Style: discordgo.SecondaryButton, CustomID: "rc_#complain#"}
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
			iconsRow[2] = iconsRow[2][:5] // Limit the number of icons to 5
			if len(iconsRow[3]) > 5 {
				iconsRow[4] = iconsRow[3][5:] // Grab overflow icons to new row
				iconsRow[3] = iconsRow[3][:5] // Limit the number of icons to 5
				iconsRow[4] = iconsRow[4][:5] // Limit the number of icons to 5
			}
		}
	}

	out := []discordgo.MessageComponent{}

	if contract.State != ContractStateSignup {

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

			} else {
				menuOptions = append(menuOptions, discordgo.SelectMenuOption{
					Label:       "Request a token",
					Description: fmt.Sprintf("%s can use to cancel request.", strings.Join(requestors, ", ")),
					Value:       "want:",
					Emoji:       ei.GetBotComponentEmoji("token"),
				})

			}

			currentIdx := contract.currentBoosterOrderIndex()
			if contract.State == ContractStateFastrun && currentIdx >= 0 && currentIdx < len(contract.Order)-1 {
				b := contract.currentBooster()
				if b != nil && b.TokensWanted <= b.TokensReceived {
					menuOptions = append(menuOptions, discordgo.SelectMenuOption{
						Label:       fmt.Sprintf("Send %s a token", contract.Boosters[contract.Order[currentIdx+1]].Nick),
						Description: fmt.Sprintf("Waiting on %s 🚀.", b.Nick),
						Value:       fmt.Sprintf("next:%s", contract.Order[currentIdx+1]),
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
			Label: "My Chicken Runs",
			Value: "mychickens",
			Emoji: ei.GetBotComponentEmoji("icon_chicken_run"),
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
		menuOptions = append(menuOptions, discordgo.SelectMenuOption{
			Label: "Admin Logs",
			Value: "adminlogs",
			Emoji: &discordgo.ComponentEmoji{Name: "📜"},
		})

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
	}

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

func addContractReactionsGather(contract *Contract, tokenStr string) ([]string, []string) {

	iconsRowA := []string{}
	iconsRowB := []string{} //mainly for alt icons

	switch contract.State {
	case ContractStateBanker:
		iconsRowA = append(iconsRowA, []string{tokenStr, "🐓", "💰", "📢"}...)
	case ContractStateFastrun:
		iconsRowA = append(iconsRowA, []string{boostIconReaction, tokenStr, "🔃", "⤵️", "🐓", "📢"}...)
	case ContractStateWaiting:
		sinkID := contract.Banker.CurrentBanker
		if sinkID != "" {
			iconsRowA = append(iconsRowA, tokenStr, "📢")
		}
		iconsRowA = append(iconsRowA, "🐓", "📢")

	case ContractStateCompleted:
		contract.Banker.CurrentBanker = contract.Banker.PostSinkUserID
		sinkID := contract.Banker.CurrentBanker
		if sinkID != "" {
			iconsRowA = append(iconsRowA, tokenStr)
		}
		iconsRowA = append(iconsRowA, "🐓")
	}

	gg, ugg, _ := ei.GetGenerousGiftEvent()
	if gg > 1.0 {
		if slices.Contains(iconsRowA, tokenStr) {
			idx := slices.Index(iconsRowA, tokenStr)
			iconsRowA = append(iconsRowA[:idx+1], append([]string{"GG"}, iconsRowA[idx+1:]...)...)
		}
	}
	if ugg > 1.0 {
		if slices.Contains(iconsRowA, tokenStr) {
			idx := slices.Index(iconsRowA, tokenStr)
			iconsRowA = append(iconsRowA[:idx+1], append([]string{"UG"}, iconsRowA[idx+1:]...)...)
		}
	}

	// Move any icons beyond 5 from iconsRowA to iconsRowB
	if len(iconsRowA) > 5 {
		iconsRowB = append(iconsRowA[5:], iconsRowB...)
		iconsRowA = iconsRowA[:5]
	}

	if len(iconsRowA) < 5 {
		iconsRowA = append(iconsRowA, "❓")
	} else {
		iconsRowB = append(iconsRowB, "❓")
	}
	return iconsRowA, iconsRowB
}
