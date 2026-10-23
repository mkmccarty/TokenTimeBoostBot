package boost

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

const (
	// RerunLBModelLeaderboard evaluates rerun potential with max of Leggy Set / SIAB.
	RerunLBModelLeaderboard = "leaderboard"
	// RerunLBModelLeaderboardDubcap evaluates rerun potential with max of Leggy Set GG / SIAB GG.
	RerunLBModelLeaderboardDubcap = "leaderboard_dubcap"
	// RerunLBModelStandard evaluates rerun potential with standard scoring (c.Cxp / SIAB).
	RerunLBModelStandard = "standard"
	// RerunLBModelStandardDubcap evaluates rerun potential with standard scoring under dubcap (GG).
	RerunLBModelStandardDubcap = "standard_dubcap"

	rerunLBModelMiscKey = "rerunLeaderboardScoringModel"
)

func normalizeRerunLBModel(m string) string {
	switch strings.ToLower(strings.TrimSpace(m)) {
	case RerunLBModelLeaderboardDubcap, "leaderboard-dubcap", "lb_dubcap", "dubcap", "gg":
		return RerunLBModelLeaderboardDubcap
	case RerunLBModelStandard, "std":
		return RerunLBModelStandard
	case RerunLBModelStandardDubcap, "standard-dubcap", "std_dubcap":
		return RerunLBModelStandardDubcap
	case RerunLBModelLeaderboard, "lb":
		fallthrough
	default:
		return RerunLBModelLeaderboard
	}
}

func rerunLBModelLabel(m string) string {
	switch normalizeRerunLBModel(m) {
	case RerunLBModelLeaderboardDubcap:
		return "Leaderboard (Dubcap)"
	case RerunLBModelStandard:
		return "Standard"
	case RerunLBModelStandardDubcap:
		return "Standard (Dubcap)"
	default:
		return "Leaderboard"
	}
}

// getContractPotentialScore returns the potential CXP score for a contract based on the scoring model.
// For all models, the assumption is that the score used is the max of the score or SIAB score.
func getContractPotentialScore(c ei.EggIncContract, model string) float64 {
	if c.ContractVersion != 2 {
		return 0
	}
	normModel := normalizeRerunLBModel(model)

	fairShare := 1.00
	if c.ID == "quant-blitz" || (c.LengthInSeconds > 0 && float64(c.LengthInSeconds) < 45*60) {
		fairShare = 3.85
	}
	if c.MaxCoopSize == 1 {
		fairShare = 1.0
	}

	switch normModel {
	case RerunLBModelLeaderboard:
		maxCxp := c.CxpMax
		if c.CxpMaxSiab > maxCxp {
			maxCxp = c.CxpMaxSiab
		}
		return maxCxp

	case RerunLBModelLeaderboardDubcap:
		maxCxp := c.CxpMaxGG
		if c.CxpMaxSiabGG > maxCxp {
			maxCxp = c.CxpMaxSiabGG
		}
		if maxCxp <= 0 {
			maxCxp = c.CxpMax
			if c.CxpMaxSiab > maxCxp {
				maxCxp = c.CxpMaxSiab
			}
		}
		return maxCxp

	case RerunLBModelStandard:
		stdCxp := c.Cxp
		if c.EstimatedDurationLower > 0 {
			siabScore := float64(getContractScoreEstimateWithDuration(c, ei.Contract_GRADE_AAA,
				c.EstimatedDurationLower,
				fairShare,
				100, int(c.EstimatedDurationLower.Minutes()),
				20, 0,
				c.ChickenRuns,
				100, 5))
			if siabScore > stdCxp {
				stdCxp = siabScore
			}
		}
		return stdCxp

	case RerunLBModelStandardDubcap:
		if c.EstimatedDurationMax > 0 && c.EstimatedDurationMaxGG > 0 && c.EstimatedDurationLower > 0 {
			ratio := float64(c.EstimatedDurationMaxGG) / float64(c.EstimatedDurationMax)
			durGG := time.Duration(float64(c.EstimatedDurationLower) * ratio)
			stdGG := float64(getContractScoreEstimateWithDuration(c, ei.Contract_GRADE_AAA,
				durGG,
				fairShare,
				100, 30,
				20, 0,
				c.ChickenRuns,
				100, 5))
			stdGGSiab := float64(getContractScoreEstimateWithDuration(c, ei.Contract_GRADE_AAA,
				durGG,
				fairShare,
				100, int(durGG.Minutes()),
				20, 0,
				c.ChickenRuns,
				100, 5))
			score := max(stdGG, stdGGSiab)
			if score > 0 {
				return score
			}
		}
		return getContractPotentialScore(c, RerunLBModelStandard)
	}

	return c.CxpMax
}

type rerunLBRow struct {
	seasonName string
	seasonID   string
	current    float64
	delta      float64
	sum        float64
}

type lbRerunSession struct {
	uuidStr        string
	userID         string
	archive        []*ei.LocalContract
	model          string
	mobileFriendly bool
	expiresAt      time.Time
}

var (
	lbRerunSessions      = make(map[string]*lbRerunSession)
	lbRerunSessionsMutex sync.Mutex
)

func cleanupLBRerunSessions() {
	now := time.Now()
	lbRerunSessionsMutex.Lock()
	defer lbRerunSessionsMutex.Unlock()
	for k, v := range lbRerunSessions {
		if now.After(v.expiresAt) {
			delete(lbRerunSessions, k)
		}
	}
}

func printLeaderboardRerun(userID string, archive []*ei.LocalContract, model string, mobileFriendly bool) []dc.LayoutComponent {
	cleanupLBRerunSessions()

	if archive == nil {
		log.Print("No archived contracts found in Egg Inc API response")
		return []dc.LayoutComponent{dc.TextDisplay{
			Content: "No archived contracts found in Egg Inc API response",
		}}
	}

	session := &lbRerunSession{
		uuidStr:        uuid.NewV7().String(),
		userID:         userID,
		archive:        archive,
		model:          normalizeRerunLBModel(model),
		mobileFriendly: mobileFriendly,
		expiresAt:      time.Now().Add(15 * time.Minute),
	}

	lbRerunSessionsMutex.Lock()
	lbRerunSessions[session.uuidStr] = session
	lbRerunSessionsMutex.Unlock()

	return renderLeaderboardRerunSession(session)
}

func calculateLeaderboardRerunRows(archive []*ei.LocalContract, model string) []rerunLBRow {
	userScores := make(map[string]float64)
	userSeasons := make(map[string]string)

	for _, a := range archive {
		contractID := a.GetContractIdentifier()
		evaluation := a.GetEvaluation()
		if contractID == "" && a.GetContract() != nil {
			contractID = a.GetContract().GetIdentifier()
		} else if contractID == "" && evaluation != nil {
			contractID = evaluation.GetContractIdentifier()
		}
		if contractID == "first-contract" || contractID == "" {
			continue
		}

		cxp := 0.0
		if evaluation != nil {
			cxp = evaluation.GetCxp()
		}
		if cxp > userScores[contractID] {
			userScores[contractID] = cxp
		}

		sID := ""
		if evaluation != nil {
			sID = strings.ToLower(strings.TrimSpace(evaluation.GetSeasonId()))
		}
		if sID == "" {
			if c, ok := ei.GetEggIncContract(contractID); ok {
				sID = strings.ToLower(strings.TrimSpace(c.SeasonID))
			}
		}
		if sID != "" {
			userSeasons[contractID] = sID
		}
	}

	contractsBySeason := make(map[string][]ei.EggIncContract)
	allV2Contracts := make(map[string]ei.EggIncContract)

	ei.EggIncContractsMutex.RLock()
	for _, c := range ei.EggIncContractsAll {
		if c.ContractVersion != 2 || c.Predicted {
			continue
		}
		allV2Contracts[c.ID] = c
		sID := strings.ToLower(strings.TrimSpace(c.SeasonID))
		if sID != "" {
			contractsBySeason[sID] = append(contractsBySeason[sID], c)
		}
	}
	ei.EggIncContractsMutex.RUnlock()

	// Ensure any contract in archive is tracked in allV2Contracts
	for id := range userScores {
		if _, exists := allV2Contracts[id]; !exists {
			if c, ok := ei.GetEggIncContract(id); ok && c.ContractVersion == 2 {
				allV2Contracts[id] = c
				sID := userSeasons[id]
				if sID == "" {
					sID = strings.ToLower(strings.TrimSpace(c.SeasonID))
				}
				if sID != "" {
					contractsBySeason[sID] = append(contractsBySeason[sID], c)
				}
			}
		}
	}

	// Calculate All Time row across all contracts
	allTimeRow := rerunLBRow{
		seasonName: "All Time",
		seasonID:   leaderboardAllTimeScope,
	}
	for id, score := range userScores {
		if c, ok := allV2Contracts[id]; ok && c.ContractVersion == 2 {
			allTimeRow.current += score
		}
	}
	for _, c := range allV2Contracts {
		pot := getContractPotentialScore(c, model)
		cur := userScores[c.ID]
		// If player already scored higher than rerun score, don't consider negative values
		if pot > cur {
			allTimeRow.delta += (pot - cur)
		}
	}
	allTimeRow.sum = allTimeRow.current + allTimeRow.delta

	rows := []rerunLBRow{allTimeRow}

	// Calculate rows for each season across all contracts
	for _, s := range leaderboardSeasons() {
		if s.value == leaderboardAllTimeScope {
			continue
		}
		sID := strings.ToLower(strings.TrimSpace(s.value))
		row := rerunLBRow{
			seasonName: s.name,
			seasonID:   s.value,
		}

		for id, score := range userScores {
			if strings.EqualFold(userSeasons[id], sID) {
				row.current += score
			}
		}

		seenContract := make(map[string]bool)
		for _, c := range contractsBySeason[sID] {
			if seenContract[c.ID] {
				continue
			}
			seenContract[c.ID] = true

			pot := getContractPotentialScore(c, model)
			cur := userScores[c.ID]
			// If player already scored higher than rerun score, don't consider negative values
			if pot > cur {
				row.delta += (pot - cur)
			}
		}
		row.sum = row.current + row.delta

		rows = append(rows, row)
	}

	return rows
}

func formatDelta(delta float64) string {
	dRound := int(math.Round(delta))
	if dRound <= 0 {
		return "+0"
	}
	return fmt.Sprintf("+%s", bottools.FormatIntWithCommas(dRound))
}

func renderLeaderboardRerunSession(session *lbRerunSession) []dc.LayoutComponent {
	rows := calculateLeaderboardRerunRows(session.archive, session.model)
	ign := farmerstate.GetMiscSettingString(session.userID, "ei_ign")
	userHeader := ""
	if ign != "" {
		userHeader = fmt.Sprintf(" • User: **%s**", ign)
	}

	var components []dc.LayoutComponent
	var builder strings.Builder

	fmt.Fprintf(&builder, "## Potential Leaderboard Rerun\n-# Model: **%s**%s\n",
		rerunLBModelLabel(session.model), userHeader)

	if session.mobileFriendly {
		for i, r := range rows {
			curStr := bottools.FormatIntWithCommas(int(math.Round(r.current)))
			deltaStr := formatDelta(r.delta)
			sumStr := bottools.FormatIntWithCommas(int(math.Round(r.sum)))

			if i == 0 {
				fmt.Fprintf(&builder, "**%s**\nCurrent: **%s** | Delta: **%s** | Sum: **%s**\n",
					r.seasonName, curStr, deltaStr, sumStr)
			} else {
				fmt.Fprintf(&builder, "\n**%s**\nCurrent: **%s** | Delta: **%s** | Sum: **%s**\n",
					r.seasonName, curStr, deltaStr, sumStr)
			}

			if builder.Len() > 1800 {
				components = append(components, dc.TextDisplay{Content: builder.String()})
				builder.Reset()
			}
		}
	} else {
		maxSeason := len("Season")
		maxCurrent := len("Current")
		maxDelta := len("Delta")
		maxSum := len("Sum")

		type formattedRow struct {
			season  string
			current string
			delta   string
			sum     string
		}

		fRows := make([]formattedRow, 0, len(rows))
		for _, r := range rows {
			fr := formattedRow{
				season:  r.seasonName,
				current: bottools.FormatIntWithCommas(int(math.Round(r.current))),
				delta:   formatDelta(r.delta),
				sum:     bottools.FormatIntWithCommas(int(math.Round(r.sum))),
			}
			if len(fr.season) > maxSeason {
				maxSeason = len(fr.season)
			}
			if len(fr.current) > maxCurrent {
				maxCurrent = len(fr.current)
			}
			if len(fr.delta) > maxDelta {
				maxDelta = len(fr.delta)
			}
			if len(fr.sum) > maxSum {
				maxSum = len(fr.sum)
			}
			fRows = append(fRows, fr)
		}

		header := fmt.Sprintf("%s | %s | %s | %s",
			bottools.AlignString("Season", maxSeason, bottools.StringAlignLeft),
			bottools.AlignString("Current", maxCurrent, bottools.StringAlignRight),
			bottools.AlignString("Delta", maxDelta, bottools.StringAlignRight),
			bottools.AlignString("Sum", maxSum, bottools.StringAlignRight),
		)
		sep := strings.Repeat("—", len(header))

		builder.WriteString("```\n")
		builder.WriteString(header)
		builder.WriteString("\n")
		builder.WriteString(sep)
		builder.WriteString("\n")

		for i, fr := range fRows {
			line := fmt.Sprintf("%s | %s | %s | %s",
				bottools.AlignString(fr.season, maxSeason, bottools.StringAlignLeft),
				bottools.AlignString(fr.current, maxCurrent, bottools.StringAlignRight),
				bottools.AlignString(fr.delta, maxDelta, bottools.StringAlignRight),
				bottools.AlignString(fr.sum, maxSum, bottools.StringAlignRight),
			)
			builder.WriteString(line)
			builder.WriteString("\n")
			if i == 0 {
				builder.WriteString(sep)
				builder.WriteString("\n")
			}
		}
		builder.WriteString("```\n")
	}

	if builder.Len() > 0 {
		components = append(components, dc.TextDisplay{Content: builder.String()})
	}

	components = append(components, buildLeaderboardRerunControls(session)...)
	return components
}

func buildLeaderboardRerunControls(session *lbRerunSession) []dc.LayoutComponent {
	minVal := 1
	normModel := normalizeRerunLBModel(session.model)

	modelMenu := dc.SelectMenu{
		CustomID:    fmt.Sprintf("rerun_lb#model#%s", session.uuidStr),
		Placeholder: "Select Scoring Model",
		MinValues:   &minVal,
		MaxValues:   1,
		Options: []dc.SelectOption{
			{
				Label:   "Leaderboard",
				Value:   RerunLBModelLeaderboard,
				Default: normModel == RerunLBModelLeaderboard,
			},
			{
				Label:   "Leaderboard (Dubcap)",
				Value:   RerunLBModelLeaderboardDubcap,
				Default: normModel == RerunLBModelLeaderboardDubcap,
			},
			{
				Label:   "Standard",
				Value:   RerunLBModelStandard,
				Default: normModel == RerunLBModelStandard,
			},
			{
				Label:   "Standard (Dubcap)",
				Value:   RerunLBModelStandardDubcap,
				Default: normModel == RerunLBModelStandardDubcap,
			},
		},
	}

	toggleViewLabel := "Mobile View"
	if session.mobileFriendly {
		toggleViewLabel = "Desktop View"
	}

	btnRow := dc.ActionRow{
		Components: []dc.InteractiveComponent{
			dc.Button{
				Label:    toggleViewLabel,
				Style:    dc.ButtonSecondary,
				CustomID: fmt.Sprintf("rerun_lb#toggleview#%s", session.uuidStr),
			},
			dc.Button{
				Label:    "Close",
				Style:    dc.ButtonDanger,
				CustomID: fmt.Sprintf("rerun_lb#close#%s", session.uuidStr),
			},
		},
	}

	return []dc.LayoutComponent{
		dc.ActionRow{
			Components: []dc.InteractiveComponent{modelMenu},
		},
		btnRow,
	}
}

// HandleLeaderboardRerunReactions dispatches component interactions for /rerun leaderboard.
func HandleLeaderboardRerunReactions(e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 2 {
		return
	}
	action := parts[1]
	uuidStr := ""
	if len(parts) >= 3 {
		uuidStr = parts[2]
	}

	if action == "close" {
		lbRerunSessionsMutex.Lock()
		delete(lbRerunSessions, uuidStr)
		lbRerunSessionsMutex.Unlock()

		_ = e.Update(dc.Message{
			Components: e.MessageComponentsWithoutActionRows(),
		})
		return
	}

	lbRerunSessionsMutex.Lock()
	session, exists := lbRerunSessions[uuidStr]
	lbRerunSessionsMutex.Unlock()
	if !exists {
		_ = e.Respond(dc.Message{
			Content:   "This rerun leaderboard session has expired. Please run the command again.",
			Ephemeral: true,
		})
		return
	}

	effectiveUser := farmerstate.GetEffectiveUserID(e.UserID(), e.ChannelID())
	if effectiveUser != session.userID && e.UserID() != session.userID {
		_ = e.Respond(dc.Message{
			Content:   "Only the user who initiated this command can interact with it.",
			Ephemeral: true,
		})
		return
	}

	switch action {
	case "model":
		vals := e.Values()
		if len(vals) > 0 {
			session.model = normalizeRerunLBModel(vals[0])
			farmerstate.SetMiscSettingString(session.userID, rerunLBModelMiscKey, session.model)
		}
	case "toggleview":
		session.mobileFriendly = !session.mobileFriendly
		farmerstate.SetMiscSettingString(session.userID, "rerunMobileFriendly", strconv.FormatBool(session.mobileFriendly))
	}

	components := renderLeaderboardRerunSession(session)
	_ = e.Update(dc.Message{
		Components: components,
	})
}
