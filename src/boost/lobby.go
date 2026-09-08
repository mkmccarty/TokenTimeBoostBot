package boost

import (
	"fmt"
	"log"
	"sort"
	"strings"

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
	}
}

// HandleLobbyButtons handles refresh and close button interactions for /lobby.
func HandleLobbyButtons(e *dc.ComponentEvent) {
	respondUsage := func(msg string) {
		_ = e.Respond(dc.Message{
			Content:   msg,
			Ephemeral: true,
		})
	}

	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 2 {
		respondUsage("Invalid lobby action. Use the Refresh or Close buttons from a /lobby response.")
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
		if err := e.EditFollowup(e.MessageID(), dc.Message{Components: components}); err != nil {
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

	default:
		respondUsage("Unknown lobby action. Use the Refresh or Close buttons from a /lobby response.")
	}
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
	eiContract := ei.EggIncContractsAll[contractID]
	if eiContract.ID == "" {
		return lobbyContent{}, fmt.Errorf("invalid contract ID")
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

func buildLobbyMismatchSection(contributors []*ei.ContractCoopStatusResponse_ContributionInfo, contract *Contract) string {
	if contract == nil {
		return ""
	}

	// Snapshot booster info to avoid holding the contract mutex across farmerstate calls.
	type boosterSnapshot struct {
		discordID  string
		eiIgn      string
		eggIncName string
		nick       string
		mention    string
	}
	contract.mutex.Lock()
	snapshots := make([]boosterSnapshot, 0, len(contract.Boosters))
	for id, b := range contract.Boosters {
		snapshots = append(snapshots, boosterSnapshot{discordID: id, nick: b.Nick, mention: b.Mention})
	}
	contract.mutex.Unlock()

	// Fetch ei_ign and eggincname for each booster outside the contract lock.
	for i := range snapshots {
		snapshots[i].eiIgn = farmerstate.GetMiscSettingString(snapshots[i].discordID, "ei_ign")
		snapshots[i].eggIncName = farmerstate.GetEggIncName(snapshots[i].discordID)
	}

	// Build case-insensitive lookup map: lowercase ei_ign or eggincname -> discordID.
	ignToID := make(map[string]string, len(snapshots))
	for _, s := range snapshots {
		if s.eiIgn != "" {
			ignToID[strings.ToLower(s.eiIgn)] = s.discordID
		}
		if s.eggIncName != "" {
			ignToID[strings.ToLower(s.eggIncName)] = s.discordID
		}
	}

	matchedBoosterIDs := make(map[string]bool)

	type guessEntry struct {
		coopName        string
		contractDisplay string
	}
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

		matched := false

		// 1. Exact database lookup by ei_ign.
		if discordID, err := farmerstate.GetDiscordUserIDFromEiIgn(coopName); err == nil && discordID != "" {
			contract.mutex.Lock()
			_, inContract := contract.Boosters[discordID]
			contract.mutex.Unlock()
			if inContract {
				matchedBoosterIDs[discordID] = true
				matched = true
			}
		}

		// 1b. Exact database lookup by eggincname.
		if !matched {
			if discordID, err := farmerstate.GetDiscordUserIDFromEggIncName(coopName); err == nil && discordID != "" {
				contract.mutex.Lock()
				_, inContract := contract.Boosters[discordID]
				contract.mutex.Unlock()
				if inContract {
					matchedBoosterIDs[discordID] = true
					matched = true
				}
			}
		}

		// 2. Case-insensitive match on ei_ign or eggincname.
		if !matched {
			coopLower := strings.ToLower(coopName)
			if id, ok := ignToID[coopLower]; ok {
				matchedBoosterIDs[id] = true
				matched = true
			}
		}

		// 3. Best-fit: substring match against ei_ign, eggincname, or Discord nick.
		if !matched {
			coopLower := strings.ToLower(coopName)
			for _, s := range snapshots {
				ignLower := strings.ToLower(s.eiIgn)
				eggLower := strings.ToLower(s.eggIncName)
				nickLower := strings.ToLower(s.nick)
				if (ignLower != "" && (strings.Contains(ignLower, coopLower) || strings.Contains(coopLower, ignLower))) ||
					(eggLower != "" && (strings.Contains(eggLower, coopLower) || strings.Contains(coopLower, eggLower))) ||
					(nickLower != "" && (strings.Contains(nickLower, coopLower) || strings.Contains(coopLower, nickLower))) {
					matchedBoosterIDs[s.discordID] = true
					display := s.mention
					if !strings.HasPrefix(display, "<@") {
						display = "`" + ei.NormalizePlayerNameForDisplay(s.nick) + "`"
					}
					bestFitGuesses = append(bestFitGuesses, guessEntry{coopName: ei.NormalizePlayerNameForDisplay(coopName), contractDisplay: display})
					matched = true
					break
				}
			}
		}

		if !matched {
			coopNotInContract = append(coopNotInContract, ei.NormalizePlayerNameForDisplay(coopName))
		}
	}

	// Find contract members with no matching coop contributor.
	var contractNotInCoop []string
	for _, s := range snapshots {
		if !matchedBoosterIDs[s.discordID] {
			display := s.mention
			if !strings.HasPrefix(display, "<@") {
				display = "`" + ei.NormalizePlayerNameForDisplay(s.nick) + "`"
			}
			contractNotInCoop = append(contractNotInCoop, display)
		}
	}

	if len(bestFitGuesses) == 0 && len(coopNotInContract) == 0 && len(contractNotInCoop) == 0 {
		return ""
	}

	sort.Strings(coopNotInContract)
	sort.Strings(contractNotInCoop)

	var sb strings.Builder
	sb.WriteString("**Roster Mismatches**\n")

	if len(bestFitGuesses) > 0 {
		sb.WriteString("Possible matches (unverified):\n")
		for _, g := range bestFitGuesses {
			fmt.Fprintf(&sb, "- `%s` (coop) ≈ %s (contract)\n", g.coopName, g.contractDisplay)
		}
	}
	if len(coopNotInContract) > 0 {
		sb.WriteString("In coop but not in bot contract:\n")
		for _, name := range coopNotInContract {
			fmt.Fprintf(&sb, "- `%s`\n", name)
		}
	}
	if len(contractNotInCoop) > 0 {
		sb.WriteString("In bot contract but not in coop:\n")
		for _, display := range contractNotInCoop {
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
				Label:    "Close",
				Style:    dc.ButtonDanger,
				CustomID: fmt.Sprintf("lobby#close#%s#%s", contractID, coopID),
			},
		},
	}
}
