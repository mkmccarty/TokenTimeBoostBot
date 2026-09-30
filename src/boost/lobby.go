package boost

import (
	"fmt"
	"log"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

type lobbyParams struct {
	contractID string
	coopID     string
	ephemeral  bool
}

// GetSlashLobbyCommand returns the slash command for showing the current coop lobby.
func GetSlashLobbyCommand(cmd string) *dc.Command {
	command := anywhereCommand(cmd, "Show the current contract lobby roster")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:         "contract-id",
			Description:  "Select a contract-id",
			Autocomplete: true,
		},
		dc.StringOption{
			Name:        "coop-id",
			Description: "Your coop-id",
		},
		dc.BoolOption{
			Name:        "private-reply",
			Description: "Response visibility, default is public",
		},
	}
	return &command
}

func parseLobbyParams(e *dc.CommandEvent) lobbyParams {
	var (
		contractID string
		coopID     string
		ephemeral  bool
	)

	if opt, ok := e.OptString("contract-id"); ok {
		contractID = strings.ToLower(strings.ReplaceAll(opt, " ", ""))
	}
	if opt, ok := e.OptString("coop-id"); ok {
		coopID = strings.ToLower(strings.ReplaceAll(opt, " ", ""))
	}
	if opt, ok := e.OptBool("private-reply"); ok && opt {
		ephemeral = true
	}

	return lobbyParams{
		contractID: contractID,
		coopID:     coopID,
		ephemeral:  ephemeral,
	}
}

// HandleLobbyCommand handles the /lobby slash command.
func HandleLobbyCommand(e *dc.CommandEvent) {
	if !CheckCoopStatusPermission(e, ei.CoopStatusFixEnabled != nil && ei.CoopStatusFixEnabled()) {
		return
	}

	p := parseLobbyParams(e)

	_ = e.Defer(p.ephemeral)

	contractID, coopID, errMsg := resolveLobbyRequest(e.ChannelID(), p.contractID, p.coopID)
	if errMsg != "" {
		_ = e.Followup(dc.Message{
			Ephemeral: true,
			Components: []dc.LayoutComponent{
				dc.TextDisplay{Content: errMsg},
			},
		})
		return
	}

	components := buildLobbyComponents(e.ChannelID(), contractID, coopID, e.UserID(), false, true)
	if sendErr := e.Followup(dc.Message{
		Ephemeral:  p.ephemeral,
		Components: components,
	}); sendErr != nil {
		log.Println("lobby FollowupMessageCreate:", sendErr)
		_ = e.Followup(dc.Message{
			Ephemeral: true,
			Content:   "Unable to display lobby. Please try running `/lobby` again.",
		})
	}
}

// HandleLobbyButtons handles refresh, ping, and close button interactions for /lobby.
func HandleLobbyButtons(client dc.Client, e *dc.ComponentEvent) {
	respondUsage := func(msg string) {
		_ = e.Respond(dc.Message{
			Content:   msg,
			Ephemeral: true,
		})
	}

	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 2 {
		respondUsage("Invalid lobby action. Use the Refresh, Ping, or Close buttons from a /lobby response.")
		return
	}

	action := parts[1]
	ephemeral := e.MessageIsEphemeral()

	switch action {
	case "close":
		_ = e.Update(dc.Message{
			Ephemeral:  ephemeral,
			Components: e.MessageComponentsWithoutActionRows(),
		})

	case "refresh":
		if len(parts) < 4 {
			respondUsage("Invalid refresh action. Use the Refresh button from a /lobby response.")
			return
		}
		contractID := parts[2]
		coopID := parts[3]

		_ = e.DeferUpdate()

		components := buildLobbyComponents(e.ChannelID(), contractID, coopID, e.UserID(), true, true)
		if err := e.EditResponse(dc.Message{Components: components}); err != nil {
			if apiErr, ok := dc.AsAPIError(err); ok && (apiErr.Code == dc.ErrCodeMissingAccess || apiErr.Code == dc.ErrCodeMissingPermissions) {
				log.Printf("lobby: unable to edit message %s in channel %s (missing access/permissions): %v", e.MessageID(), e.ChannelID(), err)
				fallback := append([]dc.LayoutComponent{
					dc.TextDisplay{Content: "_⚠️ Unable to update the original message (missing permissions in this channel). Here is the refreshed lobby:_"},
				}, components...)
				_ = e.Followup(dc.Message{Ephemeral: true, Components: fallback})
			} else {
				log.Printf("lobby FollowupMessageEdit failed (channel: %s, message: %s): %v", e.ChannelID(), e.MessageID(), err)
				_ = e.Followup(dc.Message{
					Ephemeral: true,
					Content:   "Unable to refresh lobby message. Please try running `/lobby` again.",
				})
			}
		}

	case "ping":
		if len(parts) < 4 {
			respondUsage("Invalid ping action. Use the Ping button from a /lobby response.")
			return
		}
		contractID := parts[2]
		coopID := parts[3]

		_ = e.DeferUpdate()

		handleLobbyPing(client, e, contractID, coopID)

	default:
		respondUsage("Unknown lobby action. Use the Refresh, Ping, or Close buttons from a /lobby response.")
	}
}

var getLobbyCoopStatus = func(contractID, coopID, eiID string) (*ei.ContractCoopStatusResponse, error) {
	coopStatus, _, _, err := ei.GetCoopStatusUncached(contractID, coopID, eiID)
	if err != nil {
		coopStatus, _, _, err = ei.GetCoopStatus(contractID, coopID, eiID)
	}
	return coopStatus, err
}

func handleLobbyPing(client dc.Client, e *dc.ComponentEvent, contractID string, coopID string) {
	contract := FindContractByIDs(e.ChannelID(), contractID, coopID)
	if contract == nil {
		contract = FindContract(e.ChannelID())
	}
	if contract == nil {
		_ = e.Followup(dc.Message{
			Ephemeral: true,
			Content:   "No active contract found in this channel to determine who is missing from the lobby.",
		})
		return
	}

	userID := e.UserID()
	isMember := UserInContract(contract, userID)
	if !isMember {
		contract.mutex.Lock()
		for _, b := range contract.Boosters {
			if b.AltController == userID {
				isMember = true
				break
			}
		}
		contract.mutex.Unlock()
	}
	if !isMember && !creatorOfContract(client, contract, userID) {
		_ = e.Followup(dc.Message{
			Ephemeral: true,
			Content:   "Only contract members or coordinators can ping players.",
		})
		return
	}

	contract.mutex.Lock()
	if !contract.LastLobbyPingTime.IsZero() && time.Since(contract.LastLobbyPingTime) < 1*time.Minute {
		cooldownExpiry := contract.LastLobbyPingTime.Add(1 * time.Minute).Unix()
		contract.mutex.Unlock()
		_ = e.Followup(dc.Message{
			Ephemeral: true,
			Content:   fmt.Sprintf("A lobby ping was already sent recently. Please wait <t:%d:R> before pinging again.", cooldownExpiry),
		})
		return
	}
	contract.mutex.Unlock()

	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	coopStatus, err := getLobbyCoopStatus(contractID, coopID, eiID)
	if err != nil {
		_ = e.Followup(dc.Message{
			Ephemeral: true,
			Content:   fmt.Sprintf("Unable to fetch coop status: %v", err),
		})
		return
	}
	if coopStatus.GetResponseStatus() != ei.ContractCoopStatusResponse_NO_ERROR {
		_ = e.Followup(dc.Message{
			Ephemeral: true,
			Content:   fmt.Sprintf("Coop status error: %s", ei.ContractCoopStatusResponse_ResponseStatus_name[int32(coopStatus.GetResponseStatus())]),
		})
		return
	}

	mismatch := computeLobbyMismatch(coopStatus.GetContributors(), contract)
	if len(mismatch.contractNotInCoop) == 0 {
		_ = e.Followup(dc.Message{
			Ephemeral: true,
			Content:   "All contract members are already in the lobby!",
		})
		return
	}

	pingItems := make([]string, 0, len(mismatch.contractNotInCoop))
	var pingIDs []string
	for _, s := range mismatch.contractNotInCoop {
		var item string
		var targetID string
		if s.altController != "" {
			targetID = s.altController
			altName := s.nick
			if altName == "" {
				altName = s.name
			}
			if altName == "" {
				altName = s.userName
			}
			if altName != "" {
				item = fmt.Sprintf("<@%s> (%s)", s.altController, altName)
			} else {
				item = fmt.Sprintf("<@%s>", s.altController)
			}
		} else if strings.HasPrefix(s.mention, "<@") {
			item = s.mention
			raw := strings.TrimPrefix(s.mention, "<@")
			raw = strings.TrimPrefix(raw, "!")
			targetID = strings.TrimSuffix(raw, ">")
		} else if s.discordID != "" {
			item = fmt.Sprintf("<@%s>", s.discordID)
			targetID = s.discordID
		} else if s.nick != "" {
			item = s.nick
		} else {
			item = s.mention
		}
		if item != "" {
			pingItems = append(pingItems, item)
		}
		if targetID != "" && !slices.Contains(pingIDs, targetID) {
			pingIDs = append(pingIDs, targetID)
		}
	}
	sort.Strings(pingItems)

	contractName := contract.Name
	if contractName == "" {
		contractName = contract.ContractID
	}
	coopCode := coopStatus.GetCoopIdentifier()
	if coopCode == "" {
		coopCode = coopID
	}
	requesterMention := "<@" + userID + ">"
	contract.mutex.Lock()
	if b := contract.Boosters[userID]; b != nil && b.Mention != "" {
		requesterMention = b.Mention
	}
	contract.mutex.Unlock()

	content := fmt.Sprintf(
		"Hey %s! Please join the coop for **%s**!\nCoop Code: `%s`\n-# Ping requested by %s",
		strings.Join(pingItems, ", "),
		contractName,
		coopCode,
		requesterMention,
	)

	allowedMentions := &dc.AllowedMentions{
		Users: pingIDs,
	}
	if len(pingIDs) == 0 {
		allowedMentions.Parse = []dc.MentionType{dc.MentionUsers}
	}

	msg := dc.Message{
		Content:         content,
		AllowedMentions: allowedMentions,
	}

	var sendErr error
	if client != nil {
		_, sendErr = client.SendMessage(e.ChannelID(), msg)
	} else {
		sendErr = e.Followup(msg)
	}

	if sendErr != nil {
		log.Printf("lobby ping error (channel: %s): %v", e.ChannelID(), sendErr)
		_ = e.Followup(dc.Message{
			Ephemeral: true,
			Content:   "Unable to send ping message. Please check bot permissions.",
		})
		return
	}

	contract.mutex.Lock()
	contract.LastLobbyPingTime = time.Now()
	contract.mutex.Unlock()
}

func resolveLobbyRequest(channelID string, contractID string, coopID string) (string, string, string) {
	if contractID == "" || coopID == "" {
		contract := FindContract(channelID)
		if contract == nil {
			commandLink := bottools.GetFormattedCommand("lobby")
			if commandLink == "" {
				commandLink = "/lobby"
			}
			return "", "", fmt.Sprintf("No contract found in this channel. Run %s in a channel with an active contract, or provide a `contract-id` and `coop-id`.", commandLink)
		}
		contractID = contract.ContractID
		coopID = strings.ToLower(contract.CoopID)
	}

	return contractID, coopID, ""
}

func buildLobbyComponents(channelID string, contractID string, coopID string, userID string, bypassCache bool, includeButtons bool) []dc.LayoutComponent {
	content, err := buildLobbyContent(channelID, contractID, coopID, userID, bypassCache)
	if err != nil {
		components := []dc.LayoutComponent{
			dc.TextDisplay{Content: err.Error()},
		}
		if includeButtons {
			components = append(components, lobbyButtons(contractID, coopID))
		}
		return components
	}

	components := []dc.LayoutComponent{
		dc.TextDisplay{Content: content.header},
		dc.TextDisplay{Content: content.lobby},
	}
	if content.mismatch != "" {
		components = append(components, dc.TextDisplay{Content: content.mismatch})
	}
	if includeButtons {
		components = append(components, lobbyButtons(contractID, coopID))
	}

	return components
}

type lobbyContent struct {
	header   string
	lobby    string
	mismatch string
}

func buildLobbyContent(channelID string, contractID string, coopID string, userID string, bypassCache bool) (lobbyContent, error) {
	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	eiContract, ok := ei.GetEggIncContract(contractID)
	if !ok || eiContract.ID == "" {
		return lobbyContent{}, fmt.Errorf("invalid contract ID %q", contractID)
	}

	var (
		coopStatus       *ei.ContractCoopStatusResponse
		dataTimestampStr string
		err              error
	)

	if bypassCache {
		coopStatus, _, dataTimestampStr, err = ei.GetCoopStatusUncached(contractID, coopID, eiID)
	} else {
		coopStatus, _, dataTimestampStr, err = ei.GetCoopStatus(contractID, coopID, eiID)
	}
	if err != nil {
		return lobbyContent{}, err
	}
	if coopStatus.GetResponseStatus() != ei.ContractCoopStatusResponse_NO_ERROR {
		return lobbyContent{}, fmt.Errorf("%s", ei.ContractCoopStatusResponse_ResponseStatus_name[int32(coopStatus.GetResponseStatus())])
	}
	if coopStatus.GetGrade() == ei.Contract_GRADE_UNSET {
		return lobbyContent{}, fmt.Errorf("no grade found for contract %s/%s", contractID, coopID)
	}

	grade := int(coopStatus.GetGrade())
	if grade < 0 || grade >= len(eiContract.Grade) {
		return lobbyContent{}, fmt.Errorf("invalid grade found for contract %s/%s", contractID, coopID)
	}

	coopID = coopStatus.GetCoopIdentifier()

	memberNames := make([]string, 0, len(coopStatus.GetContributors()))
	for _, contributor := range coopStatus.GetContributors() {
		name := contributor.GetUserName()
		if name == "" {
			name = contributor.GetUserId()
		}
		if name == "" {
			name = "Unknown"
		}
		memberNames = append(memberNames, ei.NormalizePlayerNameForDisplay(name))
	}
	sort.Strings(memberNames)

	carpetLink := fmt.Sprintf("%s/%s/%s", "https://eicoop-carpet.netlify.app", contractID, coopID)
	var header strings.Builder
	fmt.Fprintf(&header, "Lobby: %d/%d\n%s contract %s/[**%s**](%s)\n",
		len(memberNames), eiContract.MaxCoopSize,
		ei.GetBotEmojiMarkdown("contract_grade_"+ei.GetContractGradeString(grade)),
		coopStatus.GetContractIdentifier(), coopID, carpetLink)
	header.WriteString(dataTimestampStr)

	var lobby strings.Builder
	if len(memberNames) == 0 {
		lobby.WriteString("No lobby members found.")
	} else {
		for i, name := range memberNames {
			if i == 0 {
				fmt.Fprintf(&lobby, "1. %s\n", name)
			} else {
				fmt.Fprintf(&lobby, "- %s\n", name)
			}
		}
	}

	mismatch := buildLobbyMismatchSection(coopStatus.GetContributors(), FindContractByIDs(channelID, contractID, coopID))

	return lobbyContent{header: header.String(), lobby: lobby.String(), mismatch: mismatch}, nil
}

func boosterDisplayName(mention, nick, discordID string) string {
	if strings.HasPrefix(mention, "<@") {
		return mention
	}
	if nick != "" {
		return "`" + ei.NormalizePlayerNameForDisplay(nick) + "`"
	}
	if mention != "" {
		return "`" + ei.NormalizePlayerNameForDisplay(mention) + "`"
	}
	if discordID != "" {
		return "<@" + discordID + ">"
	}
	return "`Unknown`"
}

type boosterSnapshot struct {
	discordID     string
	eiIgn         string
	eggIncName    string
	nick          string
	userName      string
	globalName    string
	name          string
	mention       string
	altController string
}

type guessEntry struct {
	coopName        string
	contractDisplay string
}

type lobbyMismatchResult struct {
	bestFitGuesses    []guessEntry
	coopNotInContract []string
	contractNotInCoop []boosterSnapshot
}

func computeLobbyMismatch(contributors []*ei.ContractCoopStatusResponse_ContributionInfo, contract *Contract) lobbyMismatchResult {
	if contract == nil {
		return lobbyMismatchResult{}
	}

	contract.mutex.Lock()
	snapshots := make([]boosterSnapshot, 0, len(contract.Boosters))
	for id, b := range contract.Boosters {
		snapshots = append(snapshots, boosterSnapshot{
			discordID:     id,
			nick:          b.Nick,
			userName:      b.UserName,
			globalName:    b.GlobalName,
			name:          b.Name,
			mention:       b.Mention,
			altController: b.AltController,
		})
	}
	contract.mutex.Unlock()

	// Fetch ei_ign and eggincname for each booster outside the contract lock.
	for i := range snapshots {
		snapshots[i].eiIgn = farmerstate.GetMiscSettingString(snapshots[i].discordID, "ei_ign")
		snapshots[i].eggIncName = farmerstate.GetEggIncName(snapshots[i].discordID)
	}

	matchedBoosterIDs := make(map[string]bool)
	var bestFitGuesses []guessEntry
	var coopNotInContract []string

	for _, contributor := range contributors {
		coopName := contributor.GetUserName()
		if coopName == "" {
			coopName = contributor.GetUserId()
		}
		if coopName == "" {
			continue
		}

		coopTrimmed := strings.TrimSpace(coopName)
		coopLower := strings.ToLower(coopTrimmed)
		coopNorm := strings.ToLower(strings.TrimSpace(ei.NormalizePlayerNameForDisplay(coopName)))

		matched := false

		// 1. Exact database lookup by ei_ign. Try exact match first, then collapse alts.
		if discordID, err := farmerstate.GetDiscordUserIDFromEiIgnExact(coopTrimmed); err == nil && discordID != "" {
			contract.mutex.Lock()
			_, inContract := contract.Boosters[discordID]
			contract.mutex.Unlock()
			if inContract && !matchedBoosterIDs[discordID] {
				matchedBoosterIDs[discordID] = true
				matched = true
			}
		}
		if !matched {
			if discordID, err := farmerstate.GetDiscordUserIDFromEiIgn(coopTrimmed); err == nil && discordID != "" {
				contract.mutex.Lock()
				_, inContract := contract.Boosters[discordID]
				contract.mutex.Unlock()
				if inContract && !matchedBoosterIDs[discordID] {
					matchedBoosterIDs[discordID] = true
					matched = true
				}
			}
		}

		// 1b. Exact database lookup by eggincname.
		if !matched {
			if discordID, err := farmerstate.GetDiscordUserIDFromEggIncName(coopTrimmed); err == nil && discordID != "" {
				contract.mutex.Lock()
				_, inContract := contract.Boosters[discordID]
				contract.mutex.Unlock()
				if inContract && !matchedBoosterIDs[discordID] {
					matchedBoosterIDs[discordID] = true
					matched = true
				}
			}
		}

		// 2a. Case-insensitive exact match on configured Egg Inc names (ei_ign or eggincname).
		if !matched {
			for _, s := range snapshots {
				if matchedBoosterIDs[s.discordID] {
					continue
				}
				ignLower := strings.ToLower(strings.TrimSpace(s.eiIgn))
				eggLower := strings.ToLower(strings.TrimSpace(s.eggIncName))
				ignNorm := strings.ToLower(strings.TrimSpace(ei.NormalizePlayerNameForDisplay(s.eiIgn)))
				eggNorm := strings.ToLower(strings.TrimSpace(ei.NormalizePlayerNameForDisplay(s.eggIncName)))

				if (ignLower != "" && (ignLower == coopLower || ignLower == coopNorm || ignNorm == coopNorm)) ||
					(eggLower != "" && (eggLower == coopLower || eggLower == coopNorm || eggNorm == coopNorm)) {
					matchedBoosterIDs[s.discordID] = true
					matched = true
					break
				}
			}
		}

		// 2b. Case-insensitive exact match on Discord names (nick, username, global name, or name).
		if !matched {
			for _, s := range snapshots {
				if matchedBoosterIDs[s.discordID] {
					continue
				}
				nickLower := strings.ToLower(strings.TrimSpace(s.nick))
				userLower := strings.ToLower(strings.TrimSpace(s.userName))
				globalLower := strings.ToLower(strings.TrimSpace(s.globalName))
				nameLower := strings.ToLower(strings.TrimSpace(s.name))
				nickNorm := strings.ToLower(strings.TrimSpace(ei.NormalizePlayerNameForDisplay(s.nick)))
				nameNorm := strings.ToLower(strings.TrimSpace(ei.NormalizePlayerNameForDisplay(s.name)))

				if (nickLower != "" && (nickLower == coopLower || nickLower == coopNorm || nickNorm == coopNorm)) ||
					(userLower != "" && (userLower == coopLower || userLower == coopNorm)) ||
					(globalLower != "" && (globalLower == coopLower || globalLower == coopNorm)) ||
					(nameLower != "" && (nameLower == coopLower || nameLower == coopNorm || nameNorm == coopNorm)) {
					matchedBoosterIDs[s.discordID] = true
					matched = true
					break
				}
			}
		}

		// 3. Best-fit: substring match against ei_ign, eggincname, or Discord names (requiring >= 3 characters).
		if !matched {
			if len([]rune(coopLower)) >= 3 {
				for _, s := range snapshots {
					if matchedBoosterIDs[s.discordID] {
						continue
					}
					checkSub := func(candidate string) bool {
						c := strings.ToLower(strings.TrimSpace(candidate))
						if len([]rune(c)) < 3 {
							return false
						}
						cNorm := strings.ToLower(strings.TrimSpace(ei.NormalizePlayerNameForDisplay(candidate)))
						return strings.Contains(c, coopLower) || strings.Contains(coopLower, c) ||
							strings.Contains(cNorm, coopNorm) || strings.Contains(coopNorm, cNorm)
					}

					if checkSub(s.eiIgn) || checkSub(s.eggIncName) || checkSub(s.nick) ||
						checkSub(s.userName) || checkSub(s.globalName) || checkSub(s.name) {
						matchedBoosterIDs[s.discordID] = true
						display := boosterDisplayName(s.mention, s.nick, s.discordID)
						bestFitGuesses = append(bestFitGuesses, guessEntry{coopName: ei.NormalizePlayerNameForDisplay(coopName), contractDisplay: display})
						matched = true
						break
					}
				}
			}
		}

		if !matched {
			coopNotInContract = append(coopNotInContract, ei.NormalizePlayerNameForDisplay(coopName))
		}
	}

	var contractNotInCoop []boosterSnapshot
	for _, s := range snapshots {
		if !matchedBoosterIDs[s.discordID] {
			contractNotInCoop = append(contractNotInCoop, s)
		}
	}

	return lobbyMismatchResult{
		bestFitGuesses:    bestFitGuesses,
		coopNotInContract: coopNotInContract,
		contractNotInCoop: contractNotInCoop,
	}
}

func buildLobbyMismatchSection(contributors []*ei.ContractCoopStatusResponse_ContributionInfo, contract *Contract) string {
	if contract == nil {
		return ""
	}

	mismatch := computeLobbyMismatch(contributors, contract)
	if len(mismatch.bestFitGuesses) == 0 && len(mismatch.coopNotInContract) == 0 && len(mismatch.contractNotInCoop) == 0 {
		return ""
	}

	contractNotInCoopDisplays := make([]string, 0, len(mismatch.contractNotInCoop))
	for _, s := range mismatch.contractNotInCoop {
		contractNotInCoopDisplays = append(contractNotInCoopDisplays, boosterDisplayName(s.mention, s.nick, s.discordID))
	}

	sort.Strings(mismatch.coopNotInContract)
	sort.Strings(contractNotInCoopDisplays)

	var sb strings.Builder
	sb.WriteString("**Roster Mismatches**\n")

	if len(mismatch.bestFitGuesses) > 0 {
		sb.WriteString("Possible matches (unverified):\n")
		for _, g := range mismatch.bestFitGuesses {
			fmt.Fprintf(&sb, "- `%s` (coop) ≈ %s (contract)\n", g.coopName, g.contractDisplay)
		}
	}
	if len(mismatch.coopNotInContract) > 0 {
		sb.WriteString("In coop but not in bot contract:\n")
		for _, name := range mismatch.coopNotInContract {
			fmt.Fprintf(&sb, "- `%s`\n", name)
		}
	}
	if len(contractNotInCoopDisplays) > 0 {
		sb.WriteString("In bot contract but not in coop:\n")
		for _, display := range contractNotInCoopDisplays {
			fmt.Fprintf(&sb, "- %s\n", display)
		}
	}

	return sb.String()
}

func lobbyButtons(contractID string, coopID string) dc.ActionRow {
	return dc.ActionRow{
		Components: []dc.InteractiveComponent{
			dc.Button{
				Label:    "Refresh",
				Style:    dc.ButtonSecondary,
				CustomID: fmt.Sprintf("lobby#refresh#%s#%s", contractID, coopID),
				Emoji:    &dc.Emoji{Name: "🔄"},
			},
			dc.Button{
				Label:    "Ping",
				Style:    dc.ButtonPrimary,
				CustomID: fmt.Sprintf("lobby#ping#%s#%s", contractID, coopID),
				Emoji:    &dc.Emoji{Name: "🔔"},
			},
			dc.Button{
				Label:    "Close",
				Style:    dc.ButtonDanger,
				CustomID: fmt.Sprintf("lobby#close#%s#%s", contractID, coopID),
			},
		},
	}
}
