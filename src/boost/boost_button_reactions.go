package boost

import (
	"fmt"
	"log"
	"math/rand"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"time"
	"uuid"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"

	"github.com/mattn/go-runewidth"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// HandleContractReactions handles all the button reactions for a contract.
//
// It still takes a raw session because the boost order helpers and the boost
// list redraw are not on the facade yet.
func HandleContractReactions(client dc.Client, e *dc.ComponentEvent) {
	_ = e.DeferUpdate()
	userID := e.UserID()

	// rc_Name # rc_ID # HASH
	reaction := strings.Split(e.CustomID(), "#")
	cmd := strings.ToLower(reaction[1])
	contractHash := dc.SplitContractHash(e.CustomID())

	if cmd == "dismiss" {
		_ = client.DeleteMessage(e.ChannelID(), e.MessageID())
		return
	}

	contract := FindContractByHash(contractHash)
	if contract == nil {
		_ = e.Followup(dc.Message{
			Content:   "Unable to find this contract.",
			Ephemeral: true,
		})
		return
	}

	if cmd == "help" {
		_ = e.Followup(dc.Message{})
		buttonReactionHelp(client, e, contract)
		return
	}

	// Restring commands to those within the contract
	if !UserInContract(contract, userID) && !creatorOfContract(client, contract, userID) {
		_ = e.Followup(dc.Message{
			Content:   "User isn't in this contract.",
			Ephemeral: true,
		})
		return
	}
	// Ack the message for every other command
	if cmd != "cr" {
		_ = e.Followup(dc.Message{})
	}

	redraw := false

	switch cmd {
	case "boost":
		if e.MessageID() == contract.Location[0].ListMsgID {
			redraw = buttonReactionBoost(client, e.GuildID(), e.ChannelID(), contract, userID)
		}
	case "bag":
		_, redraw = buttonReactionBag(client, e.GuildID(), e.ChannelID(), contract, userID)
	case "token":
		_, redraw = buttonReactionToken(client, e.GuildID(), e.ChannelID(), contract, userID, 1, "")
		sendOrUpdateUserReactionSummary(e, contract, userID)
	case "2token":
		_, redraw = buttonReactionToken(client, e.GuildID(), e.ChannelID(), contract, userID, 2, "")
		sendOrUpdateUserReactionSummary(e, contract, userID)
	case "swap":
		redraw = buttonReactionSwap(client, e.GuildID(), e.ChannelID(), contract, userID)
	case "last":
		_, redraw = buttonReactionLast(client, e.GuildID(), e.ChannelID(), contract, userID)
	case "cr":
		var str string
		redraw, str = buttonReactionRunChickens(client, contract, userID)
		// Ack the message for every other command
		_ = e.Followup(dc.Message{
			Content:   str,
			Ephemeral: true,
		})
	case "ranchicken":
		targetUserID := ""
		if len(reaction) >= 4 {
			targetUserID = reaction[2]
		}
		buttonReactionRanChicken(client, e, contract, userID, targetUserID)
	case "rancoop":
		buttonReactionRanCoop(client, e, contract, userID)
	case "crping":
		buttonReactionCRPing(client, e, contract, userID)
	case "complain":
		buttonReactionComplain(client, contract, userID)
	case "notoken":
		buttonReactionNonToken(client, e, contract, userID)
	case "predmenu":
		values := e.Values()
		if b := contract.Boosters[userID]; b != nil {
			b.Availability.Contract = values
			for _, altID := range b.Alts {
				if altBooster := contract.Boosters[altID]; altBooster != nil {
					altBooster.Availability.Contract = values
				}
			}
			saveData(contract.ContractHash)
			_ = e.EditResponse(dc.Message{
				Components: GetAvailabilityComponents(client, contract, userID),
			})
			redraw = true
		}
	case "predtime":
		values := e.Values()
		if b := contract.Boosters[userID]; b != nil {
			b.Availability.Timeslots = values
			for _, altID := range b.Alts {
				if altBooster := contract.Boosters[altID]; altBooster != nil {
					altBooster.Availability.Timeslots = values
				}
			}
			saveData(contract.ContractHash)
			_ = e.EditResponse(dc.Message{
				Components: GetAvailabilityComponents(client, contract, userID),
			})
			redraw = true

		}
	}

	if redraw {
		refreshBoostListMessage(client, contract, false)
	}
}

func buttonReactionBoost(client dc.Client, GuildID string, ChannelID string, contract *Contract, cUserID string) bool {
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

	if userID == currentBoosterID || votingElection || creatorOfContract(client, contract, cUserID) {
		_ = Boosting(client, GuildID, ChannelID)
		scheduleCoopStatusPoll(contract)
		return true
	}
	return redraw
}

func buttonReactionToken(client dc.Client, GuildID string, ChannelID string, contract *Contract, fromUserID string, count int, alternateBooster string) (bool, bool) {
	if !UserInContract(contract, fromUserID) {
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
			tokenSerial := uuid.NewV7().String()
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
			contract.TokenLog = append(contract.TokenLog, ei.TokenUnitLog{Time: time.Now(), Quantity: count, FromUserID: fromUserID, FromNick: contract.Boosters[fromUserID].Nick, ToUserID: fromUserID, ToNick: contract.Boosters[fromUserID].Nick, Serial: uuid.NewV7().String(), Boost: false})
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
			_ = Boosting(client, GuildID, ChannelID)
			return true, false
		}
		if bankerID != "" && fromUserID == bankerID {
			scheduleCoopStatusPoll(contract)
		}
		return false, true

	}
	return false, false
}

func buttonReactionLast(client dc.Client, GuildID string, ChannelID string, contract *Contract, cUserID string) (bool, bool) {
	var uid = cUserID
	// make sure uid is in the contract
	if !UserInContract(contract, uid) {
		return false, false
	}

	switch contract.Boosters[uid].BoostState {
	case BoostStateTokenTime:
		currentBoosterPosition := findNextBooster(contract)
		err := MoveBooster(client, GuildID, ChannelID, contract.CreatorID[0], uid, len(contract.Order), currentBoosterPosition == -1)
		if err == nil && currentBoosterPosition != -1 {
			_ = ChangeCurrentBooster(client, GuildID, ChannelID, contract.CreatorID[0], contract.Order[currentBoosterPosition], true)
			return true, false
		}
	case BoostStateUnboosted:
		_ = MoveBooster(client, GuildID, ChannelID, contract.CreatorID[0], uid, len(contract.Order), true)
	}

	return false, false
}

func buttonReactionSwap(client dc.Client, GuildID string, ChannelID string, contract *Contract, cUserID string) bool {
	// Reaction for current booster to change places
	currentID := contract.currentBoosterID()
	currentIdx := contract.currentBoosterOrderIndex()
	if currentID == "" || currentIdx < 0 {
		return false
	}
	if cUserID == currentID || creatorOfContract(client, contract, cUserID) {
		if (currentIdx + 1) < len(contract.Order) {
			_ = SkipBooster(client, GuildID, ChannelID, "")
			return true
		}
	}
	return false
}

func buttonReactionRunChickens(client dc.Client, contract *Contract, cUserID string) (bool, string) {
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

	if client == nil || contract == nil {
		log.Printf("buttonReactionRunChickens invalid input: clientNil=%t contractNil=%t userID=%s",
			client == nil, contract == nil, cUserID,
		)
		return false, "Unable to process chicken run request right now."
	}

	userID := cUserID
	var str string

	if !UserInContract(contract, cUserID) {
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
		var fallbackID string
		for _, id := range contract.Order {
			if slices.Index(ids, id) != -1 {
				alt := contract.Boosters[id]
				if alt == nil {
					log.Printf("buttonReactionRunChickens missing alt/main booster during selection: contractHash=%s requestUserID=%s targetID=%s",
						contract.ContractHash, cUserID, id,
					)
					continue
				}
				if fallbackID == "" && alt.RunChickensTime.IsZero() {
					fallbackID = id
				}
				if alt.BoostState == BoostStateBoosted && alt.RunChickensTime.IsZero() {
					userID = id
					break
				}
			}
		}
		if userID == cUserID && fallbackID != "" {
			userID = fallbackID
		}
	}

	if contract.Boosters[userID] == nil {
		log.Printf("buttonReactionRunChickens missing selected booster entry: contractHash=%s requestUserID=%s selectedUserID=%s",
			contract.ContractHash, cUserID, userID,
		)
		return false, "Unable to process chicken run request right now."
	}

	if !contract.Boosters[userID].RunChickensTime.IsZero() {
		// Already asked for chicken runs
		return false, "You've already asked for Chicken Runs, if you have an alternate use `/link-alternate` to link them to your main account and then ask for chicken runs."
	}

	canRequest := false
	if contract.Boosters[userID].BoostState == BoostStateBoosted {
		canRequest = true
	} else {
		now := time.Now()
		if !contract.Boosters[userID].LastCRAttempt.IsZero() && now.Sub(contract.Boosters[userID].LastCRAttempt) < 5*time.Second {
			canRequest = true
		} else {
			contract.Boosters[userID].LastCRAttempt = now
			return false, fmt.Sprintf("You cannot request chicken runs as **%s** hasn't boosted yet. (Click the button again within 5 seconds to request anyway)", contract.Boosters[userID].Nick)
		}
	}

	if canRequest {
		contract.Boosters[userID].RunChickensTime = time.Now()

		go func() {
			client := client
			for _, location := range contract.Location {
				contract.mutex.Lock()
				components, _ := buildCRMessageComponents(contract, location.RoleMention)
				existingMsgID := contract.CRMessageIDs[location.ChannelID]
				contract.mutex.Unlock()

				if components == nil {
					continue
				}

				// Always post a new message to bump the CR requests
				newMsg, err := client.SendMessage(location.ChannelID, dc.Message{
					Components: components,
					// Ping everyone with the first
					AllowedMentions: &dc.AllowedMentions{
						Parse: []dc.MentionType{dc.MentionRoles, dc.MentionUsers},
					},
				})
				if err != nil {
					log.Printf("Error sending CR message: contractHash=%s channelID=%s userID=%s error=%v",
						contract.ContractHash, location.ChannelID, userID, err)
					continue
				}

				contract.mutex.Lock()
				setChickenRunMessageID(contract, location.ChannelID, newMsg.ID)
				contract.CRNoticeCount++
				contract.mutex.Unlock()

				if existingMsgID != "" {
					// Replace old message with a redirect to the new one
					newMsgLink := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", location.GuildID, location.ChannelID, newMsg.ID)
					movedComponents := []dc.LayoutComponent{
						dc.TextDisplay{Content: fmt.Sprintf("-# Chicken Run request moved: [View updated message](%s)", newMsgLink)},
					}
					if _, err := client.EditMessage(location.ChannelID, existingMsgID, dc.Message{
						Components:      movedComponents,
						AllowedMentions: &dc.AllowedMentions{},
					}); err != nil {
						log.Printf("Error editing old CR message: contractHash=%s channelID=%s messageID=%s error=%v",
							contract.ContractHash, location.ChannelID, existingMsgID, err)
					}
				}
			}
		}()
		str = "You've asked for Chicken Runs, now what...\n...\nMaybe.. check on your habs and gusset?\nI'm sure you've already forced a game sync so no need to remind about that."
		return true, str
	}
	return false, fmt.Sprintf("You cannot request chicken runs as **%s** hasn't boosted yet.", contract.Boosters[userID].Nick)
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
func buildCRMessageComponents(contract *Contract, roleMention string) ([]dc.LayoutComponent, []string) {
	// Collect requesters within the last 10 minutes, sorted by RunChickensTime (oldest request first).
	requesters := make([]string, 0, len(contract.Boosters))
	for id, b := range contract.Boosters {
		if b == nil || b.RunChickensTime.IsZero() {
			continue
		}
		if time.Since(b.RunChickensTime) > 10*time.Minute {
			continue
		}
		requesters = append(requesters, id)
	}
	slices.SortFunc(requesters, func(a, b string) int {
		return contract.Boosters[a].RunChickensTime.Compare(contract.Boosters[b].RunChickensTime)
	})
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

	worstPct := 0.0
	containerComps := make([]dc.ContainerSubComponent, 0, len(requesters)+1)
	var buttons []dc.InteractiveComponent
	var selectOptions []dc.SelectOption
	var allAllowedMentions []string

	// Add a TextDisplay for a header
	containerComps = append(containerComps, dc.TextDisplay{Content: "### Active Chicken Run Requests"})

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
		containerComps = append(containerComps, dc.TextDisplay{Content: sb.String()})

		// One button per incomplete requester, max 10 total (up to 2 rows of 5)
		if len(buttons) < 10 {
			label := runewidth.Truncate(name, 16, "")
			buttonLabelCount[label]++
			// Truncating to 16 chars can produce duplicate labels, append a number to keep them distinct
			if buttonLabelCount[label] > 1 {
				label = fmt.Sprintf("%s%d", label, buttonLabelCount[label])
			}
			buttons = append(buttons, dc.Button{
				Emoji:    ei.GetBotComponentEmoji("icon_chicken_run"),
				Label:    label,
				Style:    dc.ButtonSecondary,
				CustomID: fmt.Sprintf("rc_#RanChicken#%s#%s", reqID, contract.ContractHash),
			})
		}

		selectOptions = append(selectOptions, dc.SelectOption{
			Label: name + " Runners",
			Value: reqID,
		})
	}

	var tipText string
	if contract.CRNoticeCount < 2 {
		tipText = "\n-# 💡 Use the Boost Menu to indicate you aren't holding runs."
	}

	pingHeader := fmt.Sprintf(
		"%s **%s** is requesting chicken runs!\n-# Check for trucks and incoming tokens before visiting.%s\n",
		roleMention, latestName, tipText,
	)
	accentColor := crColorFromPct(worstPct)
	var components []dc.LayoutComponent

	// All requesters done, collapse into a single container with completion notice
	if len(containerComps) == 1 {
		prefix := ""
		if roleMention != "" {
			prefix = roleMention + " "
		}
		name := latestName
		if name == "" {
			name = "Contract"
		}
		completionMsg := fmt.Sprintf("-# %s**%s**'s chicken run request is complete!", prefix, name)
		components = []dc.LayoutComponent{
			dc.Container{
				AccentColor: accentColor,
				Components:  []dc.ContainerSubComponent{dc.TextDisplay{Content: completionMsg}},
			},
		}
	} else {
		components = []dc.LayoutComponent{
			dc.TextDisplay{Content: pingHeader},
			dc.Container{
				AccentColor: accentColor,
				Components:  containerComps,
			},
		}
	}

	// Up to 2 rows of 5 buttons
	for i := 0; i < len(buttons); i += 5 {
		end := min(i+5, len(buttons))
		components = append(components, dc.ActionRow{
			Components: buttons[i:end],
		})
	}

	// Select menu to ping remaining players for a specific requester
	if len(selectOptions) > 0 {
		components = append(components, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.SelectMenu{
					CustomID:    fmt.Sprintf("rc_#CRPing#%s", contract.ContractHash),
					Placeholder: "Ping remaining for...",
					Options:     selectOptions,
				},
			},
		})
	}

	return components, allAllowedMentions
}

func buttonReactionCRPing(client dc.Client, e *dc.ComponentEvent, contract *Contract, cUserID string) {
	if contract == nil || e == nil || e.MessageID() == "" {
		return
	}

	values := e.Values()
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
		_ = e.Followup(dc.Message{
			Content:   "No players remaining.",
			Ephemeral: true,
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

	msgLink := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", e.GuildID(), e.ChannelID(), e.MessageID())
	content := fmt.Sprintf(
		"Hey %s! **%s** is waiting on you to run chickens. Those chickens aren't going to run themselves! %s\n Link: %s\n-# Ping requested by %s ",
		strings.Join(mentions, " "), requesterName, ei.GetBotEmojiMarkdown("icon_chicken_run"), msgLink, presserMention)
	if _, err := client.SendMessage(e.ChannelID(), dc.Message{
		Content:         content,
		AllowedMentions: &dc.AllowedMentions{Parse: []dc.MentionType{dc.MentionUsers}},
	}); err != nil {
		log.Printf("buttonReactionCRPing send error: contractHash=%s channelID=%s requesterUserID=%s pingIDs=%v error=%v",
			contract.ContractHash, e.ChannelID(), requesterUserID, pingIDs, err)
	}
}

func buttonReactionRanChicken(client dc.Client, e *dc.ComponentEvent, contract *Contract, cUserID, requesterUserID string) {
	defer func() {
		if r := recover(); r != nil {
			msgID := ""
			channelID := ""
			if e != nil {
				channelID = e.ChannelID()
				if e.MessageID() != "" {
					msgID = e.MessageID()
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

	if contract == nil || e == nil || e.MessageID() == "" {
		log.Printf("buttonReactionRanChicken invalid input: contractNil=%t interactionNil=%t messageNil=%t userID=%s",
			contract == nil, e == nil, e == nil || e.MessageID() == "", cUserID,
		)
		return
	}

	if requesterUserID == "" {
		log.Printf("buttonReactionRanChicken empty requesterUserID: contractHash=%s messageID=%s userID=%s",
			contract.ContractHash, e.MessageID(), cUserID)
		return
	}

	if !UserInContract(contract, cUserID) {
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
		if !slices.Contains(targetBooster.RanChickensOn, requesterUserID) {
			targetBooster.RanChickensOn = append(targetBooster.RanChickensOn, requesterUserID)
		}
	}

	newColor := getChickenRunAccentColor(contract)

	// Find role mention for this channel to rebuild the header
	var roleMention string
	for _, loc := range contract.Location {
		if loc.ChannelID == e.ChannelID() {
			roleMention = loc.RoleMention
			break
		}
	}

	components, allowedMentions := buildCRMessageComponents(contract, roleMention)
	if _, err := client.EditMessage(e.ChannelID(), e.MessageID(), dc.Message{
		Components:      components,
		AllowedMentions: &dc.AllowedMentions{Users: allowedMentions},
	}); err != nil {
		log.Printf("ChannelMessageEditComplex error: contractHash=%s messageID=%s error=%v",
			contract.ContractHash, e.MessageID(), err)
	}

	if newColor != oldColor {
		saveData(contract.ContractHash)
		refreshBoostListMessage(client, contract, false)
	}
}

func buttonReactionRanCoop(client dc.Client, e *dc.ComponentEvent, contract *Contract, cUserID string) {
	defer func() {
		if r := recover(); r != nil {
			msgID := ""
			channelID := ""
			if e != nil {
				channelID = e.ChannelID()
				if e.MessageID() != "" {
					msgID = e.MessageID()
				}
			}
			contractHash := ""
			if contract != nil {
				contractHash = contract.ContractHash
			}
			log.Printf("panic recovered in buttonReactionRanCoop: panic=%v contractHash=%s channelID=%s messageID=%s userID=%s\n%s",
				r, contractHash, channelID, msgID, cUserID, string(debug.Stack()),
			)
		}
	}()

	if contract == nil || e == nil || e.MessageID() == "" {
		log.Printf("buttonReactionRanCoop invalid input: contractNil=%t interactionNil=%t messageNil=%t userID=%s",
			contract == nil, e == nil, e == nil || e.MessageID() == "", cUserID,
		)
		return
	}

	if !UserInContract(contract, cUserID) {
		return
	}

	contract.mutex.Lock()
	defer contract.mutex.Unlock()

	userBooster := contract.Boosters[cUserID]
	if userBooster == nil {
		log.Printf("buttonReactionRanCoop missing booster entry for reacting user: contractHash=%s reactingUserID=%s",
			contract.ContractHash, cUserID)
		return
	}

	oldColor := getChickenRunAccentColor(contract)

	// User IDs for reacting user and their alts
	reactingUserIDs := append([]string{cUserID}, userBooster.Alts...)

	// Mark all other boosters in the coop as having chickens run on them
	for _, id := range reactingUserIDs {
		targetBooster := contract.Boosters[id]
		if targetBooster == nil {
			continue
		}
		for _, coopMemberID := range contract.Order {
			if slices.Contains(reactingUserIDs, coopMemberID) {
				continue
			}
			if !slices.Contains(targetBooster.RanChickensOn, coopMemberID) {
				targetBooster.RanChickensOn = append(targetBooster.RanChickensOn, coopMemberID)
			}
		}
	}

	newColor := getChickenRunAccentColor(contract)

	// Determine the CR message ID to update for this channel
	targetMsgID := contract.CRMessageIDs[e.ChannelID()]
	if targetMsgID == "" {
		targetMsgID = e.MessageID()
	}

	if targetMsgID != "" {
		// Find role mention for this channel to rebuild the header
		var roleMention string
		for _, loc := range contract.Location {
			if loc.ChannelID == e.ChannelID() {
				roleMention = loc.RoleMention
				break
			}
		}

		components, allowedMentions := buildCRMessageComponents(contract, roleMention)
		if _, err := client.EditMessage(e.ChannelID(), targetMsgID, dc.Message{
			Components:      components,
			AllowedMentions: &dc.AllowedMentions{Users: allowedMentions},
		}); err != nil {
			log.Printf("EditMessage error: contractHash=%s messageID=%s error=%v",
				contract.ContractHash, targetMsgID, err)
		}
	}

	saveData(contract.ContractHash)
	if newColor != oldColor {
		refreshBoostListMessage(client, contract, false)
	}
}

func buttonReactionHelp(client dc.Client, e dc.InteractionEvent, contract *Contract) {
	contract.HelpGuidanceUntil = time.Now().Add(10 * time.Minute)
	saveData(contract.ContractHash)
	refreshBoostListMessage(client, contract, false)

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
	outputStr.WriteString("Active booster reaction of ")
	outputStr.WriteString(boostIcon)
	outputStr.WriteString(" to when spending tokens to boost. Multiple ")
	outputStr.WriteString(boostIcon)
	outputStr.WriteString(" votes by others in the contract will also indicate a boost.\n")
	outputStr.WriteString("Use ")
	outputStr.WriteString(contract.TokenStr)
	outputStr.WriteString(" when sending tokens. ")
	outputStr.WriteString("During GG use ")
	outputStr.WriteString(ei.GetBotEmojiMarkdown("std_gg"))
	outputStr.WriteString("/")
	outputStr.WriteString(ei.GetBotEmojiMarkdown("ultra_gg"))
	outputStr.WriteString(" to send 2 tokens.\n")
	fmt.Fprintf(&outputStr, "Farmer status line, %s:Requested Run, %s:10B Est, %s: Full Hab Est.\n", ei.GetBotEmojiMarkdown("icon_chicken_run"), ei.GetBotEmojiMarkdown("trophy_diamond"), ei.GetBotEmojiMarkdown("hab_full"))
	//outputStr.WriteString("Active Booster can react with ➕ or ➖ to adjust number of tokens needed.\n")
	outputStr.WriteString("Active booster reaction of 🔃 to exchange position with the next booster.\n")
	outputStr.WriteString("Reaction of ⤵️ to move yourself to last in the current boost order.\n")
	outputStr.WriteString("Reaction of ")
	outputStr.WriteString(chickMention)
	outputStr.WriteString(" when you're ready for others to run chickens on your farm.\n")
	outputStr.WriteString("Anyone can add a 🚽 reaction to express your urgency to boost next.\n")
	outputStr.WriteString("Additional help through the **/help** command.\n")

	err := e.Followup(dc.Message{
		Content:   outputStr.String(),
		Ephemeral: true,
	})
	if err != nil {
		log.Print(err)
	}
}

func buttonReactionComplain(client dc.Client, contract *Contract, cUserID string) {
	if !UserInContract(contract, cUserID) {
		return
	}

	// Backfill thematic complaints from cache if empty
	if len(contract.ThematicComplaints) == 0 {
		if complaints, err := LoadThematicComplaints(); err == nil {
			if themed, ok := complaints[contract.ContractID]; ok && len(themed) > 0 {
				contract.ThematicComplaints = append([]string(nil), themed...)
				r := rand.New(rand.NewSource(time.Now().UnixNano()))
				r.Shuffle(len(contract.ThematicComplaints), func(i, j int) {
					contract.ThematicComplaints[i], contract.ThematicComplaints[j] = contract.ThematicComplaints[j], contract.ThematicComplaints[i]
				})
				saveData(contract.ContractHash)
			}
		}
	}

	var complaint string
	var err error

	if len(contract.ThematicComplaints) > 0 && rand.Float64() < 0.60 {
		template := contract.ThematicComplaints[0]
		contract.ThematicComplaints = append(contract.ThematicComplaints[1:], template)
		complaint = fmt.Sprintf(":loudspeaker: %s", strings.ReplaceAll(template, "[player]", contract.Boosters[cUserID].Mention))
		saveData(contract.ContractHash)
	} else {
		complaint, err = ei.GetTokenComplaint(contract.Boosters[cUserID].Mention)
		if err != nil {
			log.Print(err)
			return
		}
	}

	_, err = client.SendMessage(contract.Location[0].ChannelID, dc.Message{
		Content: complaint,
		AllowedMentions: &dc.AllowedMentions{
			Parse: []dc.MentionType{dc.MentionUsers},
		},
	})
	if err != nil {
		log.Print(err)
	}
}

func getContractReactionsComponents(contract *Contract) []dc.LayoutComponent {
	compVals := contract.buttonComponents
	if compVals == nil {
		compVals = make(map[string]CompMap, 14)
		compVals[boostIconReaction] = CompMap{Emoji: boostIconReaction, Style: dc.ButtonSecondary, CustomID: "rc_#Boost#"}
		compVals[contract.TokenStr] = CompMap{ComponentEmoji: ei.GetBotComponentEmoji("token"), Style: dc.ButtonSecondary, CustomID: "rc_#token#"}
		compVals["GG"] = CompMap{ComponentEmoji: ei.GetBotComponentEmoji("std_gg"), Style: dc.ButtonSecondary, CustomID: "rc_#2token#gg#"}
		compVals["UG"] = CompMap{ComponentEmoji: ei.GetBotComponentEmoji("ultra_gg"), Style: dc.ButtonSecondary, CustomID: "rc_#2token#ug#"}
		compVals["💰"] = CompMap{Emoji: "💰", Style: dc.ButtonSecondary, CustomID: "rc_#bag#"}
		compVals["🚚"] = CompMap{Emoji: "🚚", Style: dc.ButtonSecondary, CustomID: "rc_#truck#"}
		compVals["💃"] = CompMap{Emoji: "💃", Style: dc.ButtonSecondary, CustomID: "rc_#tango#"}
		compVals["🦵"] = CompMap{Emoji: "🦵", Style: dc.ButtonSecondary, CustomID: "rc_#leg#"}
		compVals["🔃"] = CompMap{Emoji: "🔃", Style: dc.ButtonSecondary, CustomID: "rc_#swap#"}
		compVals["⤵️"] = CompMap{Emoji: "⤵️", Style: dc.ButtonSecondary, CustomID: "rc_#last#"}
		compVals["🐓"] = CompMap{ComponentEmoji: ei.GetBotComponentEmoji("runready"), Style: dc.ButtonSecondary, CustomID: "rc_#cr#"}
		compVals["✅"] = CompMap{Emoji: "✅", Style: dc.ButtonSecondary, CustomID: "rc_#check#"}
		compVals["❓"] = CompMap{Emoji: "❓", Style: dc.ButtonSecondary, CustomID: "rc_#help#"}
		compVals["📢"] = CompMap{Emoji: "📢", Style: dc.ButtonSecondary, CustomID: "rc_#complain#"}
		compVals["🚫"] = CompMap{ComponentEmoji: ei.GetBotComponentEmoji("notoken"), Style: dc.ButtonSecondary, CustomID: "rc_#notoken#"}
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

	out := []dc.LayoutComponent{}

	if contract.State != ContractStateSignup {

		menuOptions := []dc.SelectOption{}
		/*
			menuOptions = append(menuOptions, dc.SelectOption{
				Label:       "Send 2 Tokens",
				Description: "Sent 2 tokens to the current booster.",
				Value:       "send2",
				Emoji:       ei.GetBotComponentEmoji("token"),
			})*/
		if contract.State == ContractStateCompleted {
			menuOptions = append(menuOptions, dc.SelectOption{
				Label:       "Sync w/EI",
				Description: "Add completion timestamp.",
				Value:       "time",
				Emoji:       &dc.Emoji{Name: "⏱️"},
			})
		}

		if contract.State != ContractStateSignup {
			requestors := make([]string, 0, len(contract.Boosters))
			for _, booster := range contract.Boosters {
				if booster.TokenRequestFlag {
					requestors = append(requestors, booster.Nick)
					menuOptions = append(menuOptions, dc.SelectOption{
						Label: fmt.Sprintf("Send %s a token", booster.Nick),
						Value: fmt.Sprintf("send:%s", booster.UserID),
						Emoji: ei.GetBotComponentEmoji("token"),
					})
				}
			}

			if len(requestors) == 0 {
				menuOptions = append(menuOptions, dc.SelectOption{
					Label: "Request a token",
					Value: "want:",
					Emoji: ei.GetBotComponentEmoji("token"),
				})

			} else {
				menuOptions = append(menuOptions, dc.SelectOption{
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
					nextBoosterNick := contract.Boosters[contract.Order[currentIdx+1]].Nick
					menuOptions = append(menuOptions, dc.SelectOption{
						Label:       fmt.Sprintf("Send %s a token", nextBoosterNick),
						Description: fmt.Sprintf("Waiting on %s 🚀.", b.Nick),
						Value:       fmt.Sprintf("next:%s", contract.Order[currentIdx+1]),
						Emoji:       ei.GetBotComponentEmoji("token"),
					})

					gg, ugg, _ := ei.GetGenerousGiftEvent()
					if gg > 1.0 || ugg > 1.0 {
						ggEmojiName := "std_gg"
						if ugg > 1.0 && gg <= 1.0 {
							ggEmojiName = "ultra_gg"
						}
						menuOptions = append(menuOptions, dc.SelectOption{
							Label:       fmt.Sprintf("Send %s 2 tokens", nextBoosterNick),
							Description: fmt.Sprintf("Waiting on %s 🚀.", b.Nick),
							Value:       fmt.Sprintf("next2:%s", contract.Order[currentIdx+1]),
							Emoji:       ei.GetBotComponentEmoji(ggEmojiName),
						})
					}
				}
			}

		}

		if contract.State == ContractStateFastrun {
			menuOptions = append(menuOptions, dc.SelectOption{
				Label: "Move to Last",
				Value: "last",
				Emoji: &dc.Emoji{Name: "⤵️"},
			})
			menuOptions = append(menuOptions, dc.SelectOption{
				Label: "Swap with Next",
				Value: "swap",
				Emoji: &dc.Emoji{Name: "🔃"},
			})
		}
		menuOptions = append(menuOptions, dc.SelectOption{
			Label: "Help",
			Value: "help",
			Emoji: &dc.Emoji{Name: "❓"},
		})
		menuOptions = append(menuOptions, dc.SelectOption{
			Label: "Token Log",
			Value: "tlog",
			Emoji: ei.GetBotComponentEmoji("token"),
		})
		menuOptions = append(menuOptions, dc.SelectOption{
			Label:       "Toggle Reaction Log",
			Description: "Toggle token log details on or off",
			Value:       "togglerxlog",
			Emoji:       &dc.Emoji{Name: "📊"},
		})
		menuOptions = append(menuOptions, dc.SelectOption{
			Label: "My Chicken Runs",
			Value: "mychickens",
			Emoji: ei.GetBotComponentEmoji("icon_chicken_run"),
		})
		menuOptions = append(menuOptions, dc.SelectOption{
			Label: "Ran chickens early on all farms",
			Value: "rancoop",
			Emoji: ei.GetBotComponentEmoji("icon_chicken_run"),
		})
		menuOptions = append(menuOptions, dc.SelectOption{
			Label: "Coop Tools",
			Value: "tools",
			Emoji: &dc.Emoji{Name: "🧰"},
		})
		menuOptions = append(menuOptions, dc.SelectOption{
			Label: "SR Sandbox Link",
			Value: "sandbox",
			Emoji: &dc.Emoji{Name: "🌌"},
		})
		menuOptions = append(menuOptions, dc.SelectOption{
			Label: "X-Post Template",
			Value: "xpost",
			Emoji: &dc.Emoji{Name: "🖇️"},
		})
		menuOptions = append(menuOptions, dc.SelectOption{
			Label: fmt.Sprintf("%s Grange", contract.Location[0].GuildContractRole.Name),
			Value: "grange",
			Emoji: &dc.Emoji{Name: "🧑‍🧑‍🧒‍🧒"},
		})
		if contract.BoostOrder == ContractOrderIHR || contract.BoostOrder == ContractOrderIHRFuzzy {
			menuOptions = append(menuOptions, dc.SelectOption{
				Label:       "IHR Calculation Details",
				Description: "View IHR calculations for contract boosters",
				Value:       "ihrlog",
				Emoji:       ei.GetBotComponentEmoji("chalice_T4L"),
			})
		}
		menuOptions = append(menuOptions, dc.SelectOption{
			Label: "Admin Logs",
			Value: "adminlogs",
			Emoji: &dc.Emoji{Name: "📜"},
		})

		// An explicit 0 is what lets the menu be deselected.
		minValues := 0
		out = append(out, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.SelectMenu{
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
		var mComp []dc.InteractiveComponent
		for _, el := range row {
			// if compVals[el] is not found, it will panic
			if _, ok := compVals[el]; !ok {
				log.Printf("Warning: Missing component for %s in contract %s", el, contract.ContractHash)
				continue
			}
			if compVals[el].Emoji == "" {
				mComp = append(mComp, dc.Button{
					//Label: "Send a Token",
					Emoji:    compVals[el].ComponentEmoji,
					Style:    compVals[el].Style,
					CustomID: compVals[el].CustomID + contract.ContractHash,
				})

			} else {
				mComp = append(mComp, dc.Button{
					Label: compVals[el].Name,
					Emoji: &dc.Emoji{
						Name: compVals[el].Emoji,
						ID:   compVals[el].ID,
					},
					Style:    compVals[el].Style,
					CustomID: compVals[el].CustomID + contract.ContractHash,
				})
			}
		}

		if len(mComp) > 0 {
			out = append(out, dc.ActionRow{Components: mComp})
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
		iconsRowA = append(iconsRowA, []string{boostIconReaction, tokenStr, "🐓", "📢"}...)
	case ContractStateWaiting:
		sinkID := contract.Banker.CurrentBanker
		if sinkID != "" {
			iconsRowA = append(iconsRowA, tokenStr)
		}
		iconsRowA = append(iconsRowA, "🐓", "📢")

	case ContractStateCompleted:
		contract.Banker.CurrentBanker = contract.Banker.PostSinkUserID
		sinkID := contract.Banker.CurrentBanker
		if sinkID != "" {
			iconsRowA = append(iconsRowA, tokenStr)
		}
		iconsRowA = append(iconsRowA, "🐓", "📢")
	}

	gg, ugg, _ := ei.GetGenerousGiftEvent()
	if gg > 1.0 {
		if slices.Contains(iconsRowA, tokenStr) {
			idx := slices.Index(iconsRowA, tokenStr)
			iconsRowA = append(iconsRowA[:idx+1], append([]string{"GG"}, iconsRowA[idx+1:]...)...)
		}
	}
	if ugg > 1.0 && gg <= 1.0 {
		if slices.Contains(iconsRowA, tokenStr) {
			idx := slices.Index(iconsRowA, tokenStr)
			iconsRowA = append(iconsRowA[:idx+1], append([]string{"UG"}, iconsRowA[idx+1:]...)...)
		}
	}
	if contract.Style&ContractFlagAMQP != 0 && contract.State != ContractStateSignup {
		iconsRowA = append(iconsRowA, "🚫")
	}

	// Move any icons beyond 5 from iconsRowA to iconsRowB, if we have < 5 add help icon
	if len(iconsRowA) > 5 {
		iconsRowB = append(iconsRowA[5:], iconsRowB...)
		iconsRowA = iconsRowA[:5]
	} else if len(iconsRowA) < 5 {
		iconsRowA = append(iconsRowA, "❓")
	}
	return iconsRowA, iconsRowB
}

func pollCoopStatus(contract *Contract) {
	if contract.PlayStyle != ContractPlaystyleLeaderboard {
		return
	}
	if contract.State == ContractStateSignup || contract.State == ContractStateCompleted || contract.State == ContractStateArchive {
		return
	}
	log.Printf("pollCoopStatus: polling coop status for contract %s", contract.ContractHash)
	coopStatus, _, _, err := ei.GetCoopStatusUncached(contract.ContractID, contract.CoopID, "")
	if err != nil {
		log.Printf("pollCoopStatus error: %v", err)
		return
	}
	log.Printf("pollCoopStatus: successfully retrieved status for %s, response: %s", contract.ContractHash, coopStatus.GetResponseStatus().String())
}

func scheduleCoopStatusPoll(contract *Contract) {
	if contract.PlayStyle != ContractPlaystyleLeaderboard {
		return
	}
	if contract.State == ContractStateSignup || contract.State == ContractStateCompleted || contract.State == ContractStateArchive {
		return
	}
	log.Printf("scheduleCoopStatusPoll: scheduled coop status poll in 1 minute for contract %s", contract.ContractHash)
	time.AfterFunc(1*time.Minute, func() {
		pollCoopStatus(contract)
	})
}

func sendOrUpdateUserReactionSummary(e *dc.ComponentEvent, contract *Contract, userID string) {
	if contract == nil {
		return
	}

	contract.mutex.Lock()
	booster := contract.Boosters[userID]
	if booster == nil || booster.DisableEphemeralLog {
		contract.mutex.Unlock()
		return
	}
	tokenEmoji := contract.TokenStr
	if tokenEmoji == "" {
		tokenEmoji = "🪙"
	}

	var logLines []string
	tokenCount := 0
	for _, entry := range contract.TokenLog {
		if entry.FromUserID == userID {
			tokenCount += entry.Quantity
			tokenIcons := strings.Repeat(tokenEmoji, entry.Quantity)
			logLines = append(logLines, fmt.Sprintf("-# <t:%d:T> %s to **%s**", entry.Time.Unix(), tokenIcons, entry.ToNick))
		}
	}

	isAMQP := (contract.Style & ContractFlagAMQP) != 0
	if isAMQP {
		for _, t := range booster.NonTokenReactionTimes {
			logLines = append(logLines, fmt.Sprintf("-# <t:%d:T> 🚫 Sent no-token reaction", t.Unix()))
		}
	}
	noTokenCount := len(booster.NonTokenReactionTimes)
	existingMsgID := booster.NonTokenMsgID
	contract.mutex.Unlock()

	// Sort logs chronologically
	sort.Slice(logLines, func(i, j int) bool {
		return logLines[i] < logLines[j]
	})

	// Limit to the last 10 log entries while keeping overall stats intact
	if len(logLines) > 10 {
		logLines = logLines[len(logLines)-10:]
	}

	var sb strings.Builder
	if len(logLines) > 0 {
		if isAMQP {
			fmt.Fprintf(&sb, "-# **Tokens:** %d | **Other:** %d\n", tokenCount, noTokenCount)
		} else {
			fmt.Fprintf(&sb, "-# **Tokens:** %d\n", tokenCount)
		}
		for _, line := range logLines {
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	} else {
		if isAMQP {
			fmt.Fprintf(&sb, "-# **Tokens:** %d | **Other:** %d\n", tokenCount, noTokenCount)
		} else {
			fmt.Fprintf(&sb, "-# **Tokens:** %d\n", tokenCount)
		}
		sb.WriteString("-# *No recorded reactions yet.*")
	}

	content := sb.String()
	edited := false

	if existingMsgID != "" {
		if err := e.EditFollowup(existingMsgID, dc.Message{Content: content}); err == nil {
			edited = true
		} else {
			contract.mutex.Lock()
			if b := contract.Boosters[userID]; b != nil {
				b.NonTokenMsgID = ""
			}
			contract.mutex.Unlock()
		}
	}

	if !edited {
		msg, err := e.FollowupMessage(dc.Message{
			Content:   content,
			Ephemeral: true,
		})
		if err == nil && msg != nil {
			contract.mutex.Lock()
			if b := contract.Boosters[userID]; b != nil {
				b.NonTokenMsgID = msg.ID
			}
			contract.mutex.Unlock()
		}
	}
}

func buttonReactionNonToken(client dc.Client, e *dc.ComponentEvent, contract *Contract, userID string) {
	if !UserInContract(contract, userID) {
		return
	}

	now := time.Now()
	contract.mutex.Lock()
	if booster, ok := contract.Boosters[userID]; ok {
		booster.NonTokenReactionTimes = append(booster.NonTokenReactionTimes, now)
	}
	contract.mutex.Unlock()

	guildID := e.GuildID()
	if guildID == "" && len(contract.Location) > 0 {
		guildID = contract.Location[0].GuildID
	}

	if guildID != "" {
		amqpURL := guildstate.GetGuildSettingString(guildID, "amqp_url")
		if amqpURL != "" {
			PublishAMQPNonToken(guildID, amqpURL, contract.ContractID, contract.CoopID, AMQPNonTokenMessage{
				Event:      "non_token",
				ContractID: contract.ContractID,
				CoopID:     contract.CoopID,
				UserID:     userID,
				Nick:       contract.Boosters[userID].Nick,
				Time:       now,
			})
		}
	}

	sendOrUpdateUserReactionSummary(e, contract, userID)
}
