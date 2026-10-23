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

	"github.com/mattn/go-runewidth"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

const (
	// RerunLBModelLeaderboard evaluates rerun potential with max of Leggy Set / SIAB.
	RerunLBModelLeaderboard = "leaderboard"
	// RerunLBModelLeaderboardGG evaluates rerun potential with max of Leggy Set GG / SIAB GG.
	RerunLBModelLeaderboardGG = "leaderboard_gg"
	// RerunLBModelStandard evaluates rerun potential with standard scoring (c.Cxp / SIAB).
	RerunLBModelStandard = "standard"
	// RerunLBModelStandardGG evaluates rerun potential with standard scoring under generous gift (GG).
	RerunLBModelStandardGG = "standard_gg"

	// Deprecated aliases for backwards compatibility with any stored settings
	RerunLBModelLeaderboardDubcap = RerunLBModelLeaderboardGG
	RerunLBModelStandardDubcap    = RerunLBModelStandardGG

	rerunLBModelMiscKey = "rerunLeaderboardScoringModel"
)

func normalizeRerunLBModel(m string) string {
	switch strings.ToLower(strings.TrimSpace(m)) {
	case RerunLBModelLeaderboardGG, "leaderboard-gg", "lb_gg", "gg", "leaderboard_dubcap", "leaderboard-dubcap", "lb_dubcap", "dubcap":
		return RerunLBModelLeaderboardGG
	case RerunLBModelStandard, "std":
		return RerunLBModelStandard
	case RerunLBModelStandardGG, "standard-gg", "std_gg", "standard_dubcap", "standard-dubcap", "std_dubcap":
		return RerunLBModelStandardGG
	case RerunLBModelLeaderboard, "lb":
		fallthrough
	default:
		return RerunLBModelLeaderboard
	}
}

func rerunLBModelLabel(m string) string {
	switch normalizeRerunLBModel(m) {
	case RerunLBModelLeaderboardGG:
		return "Leaderboard (GG)"
	case RerunLBModelStandard:
		return "Standard"
	case RerunLBModelStandardGG:
		return "Standard (GG)"
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

	case RerunLBModelLeaderboardGG:
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
		lb := getContractPotentialScore(c, RerunLBModelLeaderboard)
		std := c.Cxp
		if std > lb && lb > 0 {
			std = lb
		}
		return std

	case RerunLBModelStandardGG:
		lbGG := getContractPotentialScore(c, RerunLBModelLeaderboardGG)
		if c.EstimatedDurationMax > 0 && c.EstimatedDurationMaxGG > 0 && c.EstimatedDurationLower > 0 &&
			len(c.Grade) > int(ei.Contract_GRADE_AAA) && len(c.Grade[ei.Contract_GRADE_AAA].TargetAmount) > 0 && len(c.TargetAmount) > 0 {
			ratio := float64(c.EstimatedDurationMaxGG) / float64(c.EstimatedDurationMax)
			durGG := time.Duration(float64(c.EstimatedDurationLower) * ratio)
			stdGG := float64(getContractScoreEstimateWithDuration(c, ei.Contract_GRADE_AAA,
				durGG,
				fairShare,
				100, 30,
				20, 0,
				c.ChickenRuns,
				100, 5))
			if stdGG > lbGG && lbGG > 0 {
				stdGG = lbGG
			}
			if stdGG > 0 {
				return stdGG
			}
		}
		std := getContractPotentialScore(c, RerunLBModelStandard)
		if std > lbGG && lbGG > 0 {
			std = lbGG
		}
		return std
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
	for _, score := range userScores {
		allTimeRow.current += score
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

	var extraSeasons []string
	for _, sID := range userSeasons {
		extraSeasons = append(extraSeasons, sID)
	}
	for sID := range contractsBySeason {
		extraSeasons = append(extraSeasons, sID)
	}

	rows := []rerunLBRow{allTimeRow}

	// Calculate rows for each season across all contracts
	for _, s := range leaderboardSeasons(extraSeasons...) {
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
		userHeader = fmt.Sprintf(" • User: **%s**", ei.NormalizePlayerNameForDisplay(ign))
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
				fmt.Fprintf(&builder, "**%s**\nCurrent: **%s** | Rerun ∆: **%s** | Sum: **%s**\n",
					r.seasonName, curStr, deltaStr, sumStr)
			} else {
				fmt.Fprintf(&builder, "\n**%s**\nCurrent: **%s** | Rerun ∆: **%s** | Sum: **%s**\n",
					r.seasonName, curStr, deltaStr, sumStr)
			}

			if builder.Len() > 1800 {
				components = append(components, dc.TextDisplay{Content: builder.String()})
				builder.Reset()
			}
		}
	} else {
		maxSeason := runewidth.StringWidth("Season")
		maxCurrent := runewidth.StringWidth("Current")
		maxDelta := runewidth.StringWidth("Rerun ∆")
		maxSum := runewidth.StringWidth("Sum")

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
			if w := runewidth.StringWidth(fr.season); w > maxSeason {
				maxSeason = w
			}
			if w := runewidth.StringWidth(fr.current); w > maxCurrent {
				maxCurrent = w
			}
			if w := runewidth.StringWidth(fr.delta); w > maxDelta {
				maxDelta = w
			}
			if w := runewidth.StringWidth(fr.sum); w > maxSum {
				maxSum = w
			}
			fRows = append(fRows, fr)
		}

		header := fmt.Sprintf("%s  %s  %s  %s",
			bottools.AlignString("Season", maxSeason, bottools.StringAlignLeft),
			bottools.AlignString("Current", maxCurrent, bottools.StringAlignRight),
			bottools.AlignString("Rerun ∆", maxDelta, bottools.StringAlignRight),
			bottools.AlignString("Sum", maxSum, bottools.StringAlignRight),
		)
		sep := strings.Repeat("—", runewidth.StringWidth(header))

		builder.WriteString("```\n")
		builder.WriteString(header)
		builder.WriteString("\n")
		builder.WriteString(sep)
		builder.WriteString("\n")

		for i, fr := range fRows {
			line := fmt.Sprintf("%s  %s  %s  %s",
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

	builder.WriteString("-# Past season scores are fixed on the in-game leaderboard ~3 weeks after the season ends\n")

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
				Label:   "Leaderboard (GG)",
				Value:   RerunLBModelLeaderboardGG,
				Default: normModel == RerunLBModelLeaderboardGG,
			},
			{
				Label:   "Standard",
				Value:   RerunLBModelStandard,
				Default: normModel == RerunLBModelStandard,
			},
			{
				Label:   "Standard (GG)",
				Value:   RerunLBModelStandardGG,
				Default: normModel == RerunLBModelStandardGG,
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
