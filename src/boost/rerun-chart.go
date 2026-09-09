package boost

import (
	"fmt"
	"log"
	"math"
	"slices"
	"sort"
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

type chartRow struct {
	contractID  string
	cxp         float64
	maxCxp      float64
	gap         float64
	percent     float64
	validUntil  int64
	dayLabel    string
	hasSiab     bool
	maxCoopSize int
}

type chartSession struct {
	uuidStr        string
	userID         string
	rows           []chartRow
	archive        []*ei.LocalContract // retained for season navigation
	page           int
	sortBy         string
	percent        int
	seasonScope    string // non-empty when percent == -100 (season chart)
	expiresAt      time.Time
	hasDayMap      bool
	mobileFriendly bool
	siabOnly       bool
	generousGift   bool
}

var (
	chartSessions      = make(map[string]*chartSession)
	chartSessionsMutex sync.Mutex
)

const (
	rerunSortByMiscKey       = "rerunSortBy"
	predictionsSortByMiscKey = "predictionsSortBy"
)

func isValidChartSortBy(sortBy string) bool {
	switch sortBy {
	case "date", "date_asc", "gap", "gap_asc", "percent", "percent_desc", "cs", "cs_asc", "name", "name_desc", "id", "id_desc", "pred", "pred_desc":
		return true
	default:
		return false
	}
}

func cleanupChartSessions() {
	now := time.Now()
	chartSessionsMutex.Lock()
	defer chartSessionsMutex.Unlock()
	for k, v := range chartSessions {
		if now.After(v.expiresAt) {
			delete(chartSessions, k)
		}
	}
}

func printContractChart(userID string, archive []*ei.LocalContract, percent int, page int, contractIDList []string, contractDayMap map[string]string, mobileFriendly bool, seasonScope string) []dc.LayoutComponent {
	cleanupChartSessions()
	var rows []chartRow

	eiUserName := farmerstate.GetMiscSettingString(userID, "ei_ign")
	var sortBy string
	if percent == -200 {
		sortBy = farmerstate.GetMiscSettingString(userID, predictionsSortByMiscKey)
		if !isValidChartSortBy(sortBy) {
			sortBy = "pred"
		}
	} else {
		sortBy = farmerstate.GetMiscSettingString(userID, rerunSortByMiscKey)
		if !isValidChartSortBy(sortBy) {
			sortBy = "percent_desc"
		}
	}

	if archive == nil {
		log.Print("No archived contracts found in Egg Inc API response")
		return []dc.LayoutComponent{dc.TextDisplay{
			Content: "No archived contracts found in Egg Inc API response",
		}}
	}
	log.Printf("Downloaded %d archived contracts from Egg Inc API for %s\n", len(archive), eiUserName)

	var generousGift bool
	if val := farmerstate.GetMiscSettingString(userID, "rerunGenerousGift"); val != "" {
		generousGift, _ = strconv.ParseBool(val)
	}

	processedIDs := make(map[string]bool)

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

		evaluationCxp := evaluation.GetCxp()
		c, _ := ei.GetEggIncContract(contractID)

		if len(contractIDList) > 0 {
			if !slices.Contains(contractIDList, contractID) {
				continue
			}
		}

		processedIDs[contractID] = true

		if c.ContractVersion == 2 {
			maxCxp := c.CxpMax
			if generousGift {
				maxCxp = c.CxpMaxGG
			}
			hasSiab := false
			if generousGift {
				if c.CxpMaxSiabGG > c.CxpMaxGG {
					maxCxp = c.CxpMaxSiabGG
					if c.CxpMaxSiabGG > evaluationCxp {
						hasSiab = true
					}
				}
			} else {
				if c.CxpMaxSiab > c.CxpMax {
					maxCxp = c.CxpMaxSiab
					if c.CxpMaxSiab > evaluationCxp {
						hasSiab = true
					}
				}
			}

			evalPercent := 0.0
			if maxCxp > 0 {
				evalPercent = (evaluationCxp / maxCxp) * 100.0
			}

			dayLabel := ""
			if len(contractDayMap) > 0 {
				dayLabel = contractDayMap[contractID]
			}
			rows = append(rows, chartRow{
				contractID:  contractID,
				cxp:         evaluationCxp,
				maxCxp:      maxCxp,
				gap:         maxCxp - evaluationCxp,
				percent:     evalPercent,
				validUntil:  c.ValidUntil.Unix(),
				dayLabel:    dayLabel,
				hasSiab:     hasSiab,
				maxCoopSize: c.MaxCoopSize,
			})
		}
	}

	if percent == -200 {
		for _, contractID := range contractIDList {
			if processedIDs[contractID] {
				continue
			}
			c, ok := ei.GetEggIncContract(contractID)
			if !ok {
				continue
			}
			if c.ContractVersion == 2 {
				evaluationCxp := 0.0
				maxCxp := c.CxpMax
				if generousGift {
					maxCxp = c.CxpMaxGG
				}
				hasSiab := false
				if generousGift {
					if c.CxpMaxSiabGG > c.CxpMaxGG {
						maxCxp = c.CxpMaxSiabGG
						if c.CxpMaxSiabGG > evaluationCxp {
							hasSiab = true
						}
					}
				} else {
					if c.CxpMaxSiab > c.CxpMax {
						maxCxp = c.CxpMaxSiab
						if c.CxpMaxSiab > evaluationCxp {
							hasSiab = true
						}
					}
				}

				dayLabel := ""
				if len(contractDayMap) > 0 {
					dayLabel = contractDayMap[contractID]
				}
				rows = append(rows, chartRow{
					contractID:  contractID,
					cxp:         evaluationCxp,
					maxCxp:      maxCxp,
					gap:         maxCxp - evaluationCxp,
					percent:     0.0,
					validUntil:  c.ValidUntil.Unix(),
					dayLabel:    dayLabel,
					hasSiab:     hasSiab,
					maxCoopSize: c.MaxCoopSize,
				})
			}
		}
	}

	// For the season chart, add zero-score rows for season contracts the user
	// has not yet completed (same pattern as predictions).
	if percent == -100 {
		for _, contractID := range contractIDList {
			if processedIDs[contractID] {
				continue
			}
			c, ok := ei.GetEggIncContract(contractID)
			if !ok {
				continue
			}
			if c.ContractVersion == 2 {
				evaluationCxp := 0.0
				maxCxp := c.CxpMax
				if generousGift {
					maxCxp = c.CxpMaxGG
				}
				hasSiab := false
				if generousGift {
					if c.CxpMaxSiabGG > c.CxpMaxGG {
						maxCxp = c.CxpMaxSiabGG
						if c.CxpMaxSiabGG > evaluationCxp {
							hasSiab = true
						}
					}
				} else {
					if c.CxpMaxSiab > c.CxpMax {
						maxCxp = c.CxpMaxSiab
						if c.CxpMaxSiab > evaluationCxp {
							hasSiab = true
						}
					}
				}

				rows = append(rows, chartRow{
					contractID:  contractID,
					cxp:         evaluationCxp,
					maxCxp:      maxCxp,
					gap:         maxCxp - evaluationCxp,
					percent:     0.0,
					validUntil:  c.ValidUntil.Unix(),
					hasSiab:     hasSiab,
					maxCoopSize: c.MaxCoopSize,
				})
			}
		}
	}

	session := &chartSession{
		uuidStr:        uuid.NewV7().String(),
		userID:         userID,
		rows:           rows,
		archive:        archive,
		page:           page - 1, // Store as 0-indexed internally
		sortBy:         sortBy,
		percent:        percent,
		seasonScope:    seasonScope,
		expiresAt:      time.Now().Add(15 * time.Minute),
		hasDayMap:      len(contractDayMap) > 0,
		mobileFriendly: mobileFriendly,
		generousGift:   generousGift,
	}
	if session.page < 0 {
		session.page = 0
	}

	chartSessionsMutex.Lock()
	chartSessions[session.uuidStr] = session
	chartSessionsMutex.Unlock()
	return renderChartSession(session)
}

func renderChartSession(session *chartSession) []dc.LayoutComponent {
	var components []dc.LayoutComponent
	divider := true
	spacing := dc.SeparatorSpacingSmall
	builder := strings.Builder{}
	now := time.Now().Unix()

	// Filter rows based on session criteria
	var displayRows []chartRow
	for _, r := range session.rows {
		c, ok := ei.GetEggIncContract(r.contractID)
		if ok {
			maxCxp := c.CxpMax
			if session.generousGift {
				maxCxp = c.CxpMaxGG
			}
			hasSiab := false
			if session.generousGift {
				if c.CxpMaxSiabGG > c.CxpMaxGG {
					maxCxp = c.CxpMaxSiabGG
					if c.CxpMaxSiabGG > r.cxp {
						hasSiab = true
					}
				}
			} else {
				if c.CxpMaxSiab > c.CxpMax {
					maxCxp = c.CxpMaxSiab
					if c.CxpMaxSiab > r.cxp {
						hasSiab = true
					}
				}
			}

			r.maxCxp = maxCxp
			r.gap = maxCxp - r.cxp
			if maxCxp > 0 {
				r.percent = (r.cxp / maxCxp) * 100.0
			} else {
				r.percent = 0.0
			}
			r.hasSiab = hasSiab
		}

		switch session.percent {
		case -1: // Active contracts chart
			if r.validUntil > now {
				displayRows = append(displayRows, r)
			}
		case -100: // Season chart – show all contracts, no time or percent filter
			displayRows = append(displayRows, r)
		case -200: // Predictions chart
			displayRows = append(displayRows, r)
		default: // Threshold chart
			if r.percent < float64(100-session.percent) {
				displayRows = append(displayRows, r)
			}
		}
	}

	// Apply SIAB filter if enabled
	if session.siabOnly {
		var siabRows []chartRow
		for _, r := range displayRows {
			if r.hasSiab {
				siabRows = append(siabRows, r)
			}
		}
		displayRows = siabRows
	}

	contractPreds, _ := GetPredictedTimes()

	// Sort rows
	sort.SliceStable(displayRows, func(i, j int) bool {
		switch session.sortBy {
		case "pred":
			tI := contractPreds[displayRows[i].contractID]
			tJ := contractPreds[displayRows[j].contractID]
			if !tI.IsZero() && !tJ.IsZero() {
				if !tI.Equal(tJ) {
					return tI.Before(tJ)
				}
			} else if !tI.IsZero() {
				return true
			} else if !tJ.IsZero() {
				return false
			}
			return displayRows[i].validUntil > displayRows[j].validUntil
		case "pred_desc":
			tI := contractPreds[displayRows[i].contractID]
			tJ := contractPreds[displayRows[j].contractID]
			if !tI.IsZero() && !tJ.IsZero() {
				if !tI.Equal(tJ) {
					return tI.After(tJ)
				}
			} else if !tI.IsZero() {
				return true
			} else if !tJ.IsZero() {
				return false
			}
			return displayRows[i].validUntil > displayRows[j].validUntil
		case "gap":
			if displayRows[i].gap == displayRows[j].gap {
				return displayRows[i].validUntil > displayRows[j].validUntil
			}
			return displayRows[i].gap > displayRows[j].gap
		case "gap_asc":
			if displayRows[i].gap == displayRows[j].gap {
				return displayRows[i].validUntil > displayRows[j].validUntil
			}
			return displayRows[i].gap < displayRows[j].gap
		case "percent":
			if displayRows[i].percent == displayRows[j].percent {
				return displayRows[i].validUntil > displayRows[j].validUntil
			}
			return displayRows[i].percent < displayRows[j].percent
		case "percent_desc":
			if displayRows[i].percent == displayRows[j].percent {
				return displayRows[i].validUntil > displayRows[j].validUntil
			}
			return displayRows[i].percent > displayRows[j].percent
		case "cs":
			if displayRows[i].cxp == displayRows[j].cxp {
				return displayRows[i].validUntil > displayRows[j].validUntil
			}
			return displayRows[i].cxp > displayRows[j].cxp
		case "cs_asc":
			if displayRows[i].cxp == displayRows[j].cxp {
				return displayRows[i].validUntil > displayRows[j].validUntil
			}
			return displayRows[i].cxp < displayRows[j].cxp
		case "date_asc":
			return displayRows[i].validUntil < displayRows[j].validUntil
		case "name":
			cI, _ := ei.GetEggIncContract(displayRows[i].contractID)
			nameI := cI.Name
			if nameI == "" {
				nameI = displayRows[i].contractID
			}
			cJ, _ := ei.GetEggIncContract(displayRows[j].contractID)
			nameJ := cJ.Name
			if nameJ == "" {
				nameJ = displayRows[j].contractID
			}
			if nameI == nameJ {
				return displayRows[i].validUntil > displayRows[j].validUntil
			}
			return nameI < nameJ
		case "name_desc":
			cI, _ := ei.GetEggIncContract(displayRows[i].contractID)
			nameI := cI.Name
			if nameI == "" {
				nameI = displayRows[i].contractID
			}
			cJ, _ := ei.GetEggIncContract(displayRows[j].contractID)
			nameJ := cJ.Name
			if nameJ == "" {
				nameJ = displayRows[j].contractID
			}
			if nameI == nameJ {
				return displayRows[i].validUntil > displayRows[j].validUntil
			}
			return nameI > nameJ
		case "id":
			if displayRows[i].contractID == displayRows[j].contractID {
				return displayRows[i].validUntil > displayRows[j].validUntil
			}
			return displayRows[i].contractID < displayRows[j].contractID
		case "id_desc":
			if displayRows[i].contractID == displayRows[j].contractID {
				return displayRows[i].validUntil > displayRows[j].validUntil
			}
			return displayRows[i].contractID > displayRows[j].contractID
		case "date":
			fallthrough
		default:
			return displayRows[i].validUntil > displayRows[j].validUntil
		}
	})

	// Pagination bounds
	pageSize := 15
	totalPages := int(math.Ceil(float64(len(displayRows)) / float64(pageSize)))
	if totalPages == 0 {
		totalPages = 1
	}
	if session.page >= totalPages {
		session.page = totalPages - 1
	}
	if session.page < 0 {
		session.page = 0
	}

	startIdx := session.page * pageSize
	endIdx := min(startIdx+pageSize, len(displayRows))
	pageRows := displayRows[startIdx:endIdx]

	switch session.percent {
	case -1:
		builder.WriteString("## Contract CS eval of active contracts")
	case -100:
		seasonLabel := leaderboardSeasonName(session.seasonScope)
		fmt.Fprintf(&builder, "## Contract CS eval for %s season", seasonLabel)
	case -200:
		builder.WriteString("## Displaying contract scores for future predictions")
	default:
		fmt.Fprintf(&builder, "## Displaying contract scores less than %d%% of speedrun potential", session.percent)
	}
	if session.generousGift {
		builder.WriteString(" (Generous Gift)")
	}
	builder.WriteString(":\n")
	if session.siabOnly {
		builder.WriteString("### (Filtered to contracts where SIAB score is higher than Max)\n")
	}

	components = append(components, dc.TextDisplay{Content: builder.String()})
	components = append(components, dc.Separator{Divider: divider, Spacing: spacing})
	builder.Reset()

	if len(pageRows) == 0 {
		components = append(components, dc.TextDisplay{Content: "No contracts met this condition.\n"})
		components = append(components, buildChartControls(session, totalPages)...)
		return components
	}

	if !session.mobileFriendly {
		if session.hasDayMap {
			fmt.Fprintf(&builder, "`%12s %6s %6s %6s %6s %3s %3s`\n",
				bottools.AlignString("CONTRACT-ID", 25, bottools.StringAlignCenter),
				bottools.AlignString("CS", 6, bottools.StringAlignCenter),
				bottools.AlignString("HIGH", 6, bottools.StringAlignCenter),
				bottools.AlignString("GAP", 6, bottools.StringAlignRight),
				bottools.AlignString("%", 4, bottools.StringAlignCenter),
				bottools.AlignString("👤", 3, bottools.StringAlignCenter),
				bottools.AlignString("Day", 6, bottools.StringAlignCenter),
			)
		} else {
			fmt.Fprintf(&builder, "`%12s %6s %6s %6s %6s`\n",
				bottools.AlignString("CONTRACT-ID", 25, bottools.StringAlignCenter),
				bottools.AlignString("CS", 6, bottools.StringAlignCenter),
				bottools.AlignString("HIGH", 6, bottools.StringAlignCenter),
				bottools.AlignString("GAP", 6, bottools.StringAlignRight),
				bottools.AlignString("%", 4, bottools.StringAlignCenter),
			)
		}
	}

	for _, r := range pageRows {
		siabIcon := ""
		if r.hasSiab {
			siabIcon = " " + ei.GetBotEmojiMarkdown("SIAB_T4L")
		}

		if session.mobileFriendly {
			c := ei.EggIncContractsAll[r.contractID]
			name := c.Name
			if name == "" {
				name = r.contractID
			}
			eggEmoji := ei.FindEggEmoji(c.EggName)

			dayStr := ""
			if session.hasDayMap && r.dayLabel != "" {
				switch r.dayLabel {
				case "W":
					dayStr = " - **Wed**"
				case "F":
					dayStr = " - **Fri**"
				case "U":
					dayStr = " - **Fri**" + ei.GetBotEmojiMarkdown("ultra")
				default:
					dayStr = " - **" + r.dayLabel + "**"
				}
			}

			expireStr := ""
			if !session.hasDayMap && r.validUntil > 0 {
				expireStr = fmt.Sprintf(" <t:%d:R>", r.validUntil)
			}

			szStr := ""
			if session.hasDayMap {
				szStr = fmt.Sprintf("/ **%dp** ", r.maxCoopSize)
			}

			fmt.Fprintf(&builder, "%s **%s**%s%s%s%s\n",
				eggEmoji, name, szStr, siabIcon, dayStr, expireStr)

			fmt.Fprintf(&builder, "-# _       _ CS: **%d** / %d (%.1f%%) Gap: **%d**\n",
				int(math.Ceil(r.cxp)), int(math.Ceil(r.maxCxp)), r.percent, int(math.Ceil(r.gap)))
		} else {
			if session.hasDayMap {
				fmt.Fprintf(&builder, "`%12s %6s %6s %6s %6s %3s %3s`%s\n",
					bottools.AlignString(r.contractID, 25, bottools.StringAlignLeft),
					bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(r.cxp))), 6, bottools.StringAlignRight),
					bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(r.maxCxp))), 6, bottools.StringAlignRight),
					bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(r.gap))), 6, bottools.StringAlignRight),
					bottools.AlignString(fmt.Sprintf("%.1f", r.percent), 4, bottools.StringAlignCenter),
					bottools.AlignString(fmt.Sprintf("%d", r.maxCoopSize), 3, bottools.StringAlignCenter),
					bottools.AlignString(r.dayLabel, 6, bottools.StringAlignCenter),
					siabIcon)
			} else {
				fmt.Fprintf(&builder, "`%12s %6s %6s %6s %6s`%s <t:%d:R>\n",
					bottools.AlignString(r.contractID, 25, bottools.StringAlignLeft),
					bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(r.cxp))), 6, bottools.StringAlignRight),
					bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(r.maxCxp))), 6, bottools.StringAlignRight),
					bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(r.gap))), 6, bottools.StringAlignRight),
					bottools.AlignString(fmt.Sprintf("%.1f", r.percent), 4, bottools.StringAlignCenter),
					siabIcon,
					r.validUntil)
			}
		}
	}

	fmt.Fprintf(&builder, "\nShowing page %d of %d (%d total contracts).\n", session.page+1, totalPages, len(displayRows))
	if !session.mobileFriendly && session.hasDayMap {
		fmt.Fprintf(&builder, "-# Predicted contract days: W=Wednesday, F=Friday, U=Friday%s\n", ei.GetBotEmojiMarkdown("ultra"))
	}
	rateVal := "6"
	ggSuffix := ""
	if session.generousGift {
		rateVal = "12"
		ggSuffix = " (GG x2)"
	}
	leggyTokens, _ := calcLeggyBoost(DefaultLeggyTE)
	fmt.Fprintf(&builder, "-# Est duration/CS based on 1.0 fair share, %.0f%s boosts (w/%.0f%s TE), %s%s/hr%s rate and leggy artifacts.\n", leggyTokens, ei.GetBotEmojiMarkdown("token"), DefaultLeggyTE, ei.GetBotEmojiMarkdown("egg_truth"), rateVal, ei.GetBotEmojiMarkdown("token"), ggSuffix)

	components = append(components, dc.TextDisplay{Content: builder.String()})
	components = append(components, buildChartControls(session, totalPages)...)

	return components
}

// buildSeasonRows rebuilds the row data for a season chart when the user
// navigates to a different season. It mirrors the logic in printContractChart
// for the -100 (season) path.
func buildSeasonRows(session *chartSession, newScope string) []chartRow {
	var rows []chartRow
	processedIDs := make(map[string]bool)

	// Collect contract IDs for the new season.
	var contractIDList []string
	ei.EggIncContractsMutex.RLock()
	for _, c := range ei.EggIncContractsAll {
		if c.Predicted || !strings.EqualFold(c.SeasonID, newScope) {
			continue
		}
		if !slices.Contains(contractIDList, c.ID) {
			contractIDList = append(contractIDList, c.ID)
		}
	}
	ei.EggIncContractsMutex.RUnlock()

	// Build rows from the stored archive.
	for _, a := range session.archive {
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
		if !slices.Contains(contractIDList, contractID) {
			continue
		}
		evaluationCxp := evaluation.GetCxp()
		c, _ := ei.GetEggIncContract(contractID)
		processedIDs[contractID] = true
		if c.ContractVersion == 2 {
			maxCxp := c.CxpMax
			if session.generousGift {
				maxCxp = c.CxpMaxGG
			}
			hasSiab := false
			if session.generousGift {
				if c.CxpMaxSiabGG > c.CxpMaxGG {
					maxCxp = c.CxpMaxSiabGG
					if c.CxpMaxSiabGG > evaluationCxp {
						hasSiab = true
					}
				}
			} else {
				if c.CxpMaxSiab > c.CxpMax {
					maxCxp = c.CxpMaxSiab
					if c.CxpMaxSiab > evaluationCxp {
						hasSiab = true
					}
				}
			}
			evalPercent := 0.0
			if maxCxp > 0 {
				evalPercent = (evaluationCxp / maxCxp) * 100.0
			}
			rows = append(rows, chartRow{
				contractID:  contractID,
				cxp:         evaluationCxp,
				maxCxp:      maxCxp,
				gap:         maxCxp - evaluationCxp,
				percent:     evalPercent,
				validUntil:  c.ValidUntil.Unix(),
				hasSiab:     hasSiab,
				maxCoopSize: c.MaxCoopSize,
			})
		}
	}

	// Zero-score rows for un-played season contracts.
	for _, contractID := range contractIDList {
		if processedIDs[contractID] {
			continue
		}
		c, ok := ei.GetEggIncContract(contractID)
		if !ok || c.ContractVersion != 2 {
			continue
		}
		maxCxp := c.CxpMax
		if session.generousGift {
			maxCxp = c.CxpMaxGG
		}
		hasSiab := false
		if session.generousGift {
			if c.CxpMaxSiabGG > c.CxpMaxGG {
				maxCxp = c.CxpMaxSiabGG
				hasSiab = true
			}
		} else {
			if c.CxpMaxSiab > c.CxpMax {
				maxCxp = c.CxpMaxSiab
				hasSiab = true
			}
		}
		rows = append(rows, chartRow{
			contractID:  contractID,
			maxCxp:      maxCxp,
			gap:         maxCxp,
			percent:     0.0,
			validUntil:  c.ValidUntil.Unix(),
			hasSiab:     hasSiab,
			maxCoopSize: c.MaxCoopSize,
		})
	}
	return rows
}

// leaderboardNextSeason returns the season immediately after the given one.
func leaderboardNextSeason(name string, year int) (string, int, bool) {
	idx := leaderboardSeasonIndex(name)
	if idx < 0 {
		return "", 0, false
	}
	idx++
	if idx >= len(leaderboardSeasonOrder) {
		idx = 0
		year++
	}
	return leaderboardSeasonOrder[idx], year, true
}

// leaderboardSeasonExists reports whether a given season ID is present in the
// known season list (i.e., it is not before the start and not in the future).
func leaderboardSeasonExists(seasonID string) bool {
	for _, s := range leaderboardSeasons() {
		if s.value == seasonID {
			return true
		}
	}
	return false
}

// buildSeasonNavButtons returns an action row of up to 4 season navigation
// buttons: [same-season prev-year] [prev-season] [next-season] [same-season next-year].
// Buttons whose target season does not exist in the known list are omitted.
// Returns nil if this is not a season chart or no valid neighbors exist.
func buildSeasonNavButtons(session *chartSession) dc.ActionRow {
	name, year, ok := leaderboardParseSeasonID(session.seasonScope)
	if !ok {
		return dc.ActionRow{}
	}

	type candidate struct {
		scope string
		label string
		tag   string
	}

	var cands []candidate

	// Prev year: same season, year-1
	if prevYearScope := leaderboardSeasonID(name, year-1); leaderboardSeasonExists(prevYearScope) {
		cands = append(cands, candidate{prevYearScope, leaderboardSeasonLabel(prevYearScope), "prev_year"})
	}

	// Prev season
	if prevName, prevYear, ok := leaderboardPreviousSeason(name, year); ok {
		if prevScope := leaderboardSeasonID(prevName, prevYear); leaderboardSeasonExists(prevScope) {
			cands = append(cands, candidate{prevScope, leaderboardSeasonLabel(prevScope), "prev_season"})
		}
	}

	// Next season
	if nextName, nextYear, ok := leaderboardNextSeason(name, year); ok {
		if nextScope := leaderboardSeasonID(nextName, nextYear); leaderboardSeasonExists(nextScope) {
			cands = append(cands, candidate{nextScope, leaderboardSeasonLabel(nextScope), "next_season"})
		}
	}

	// Next year: same season, year+1
	if nextYearScope := leaderboardSeasonID(name, year+1); leaderboardSeasonExists(nextYearScope) {
		cands = append(cands, candidate{nextYearScope, leaderboardSeasonLabel(nextYearScope), "next_year"})
	}

	var buttons []dc.InteractiveComponent
	for _, c := range cands {
		buttons = append(buttons, dc.Button{
			Label:    c.label,
			Style:    dc.ButtonSecondary,
			CustomID: fmt.Sprintf("chart#seasonswitch#%s#%s#%s", session.uuidStr, c.scope, c.tag),
		})
	}

	// Current Season button — only when not already viewing the current season.
	if curName, curYear, ok := leaderboardMostRecentSeason(); ok {
		curScope := leaderboardSeasonID(curName, curYear)
		if curScope != session.seasonScope && leaderboardSeasonExists(curScope) {
			buttons = append(buttons, dc.Button{
				Label:    "Current Season",
				Style:    dc.ButtonPrimary,
				CustomID: fmt.Sprintf("chart#seasonswitch#%s#%s#current", session.uuidStr, curScope),
			})
		}
	}

	if len(buttons) == 0 {
		return dc.ActionRow{}
	}

	if len(buttons) > 5 {
		buttons = buttons[:5]
	}

	return dc.ActionRow{Components: buttons}
}

func buildChartControls(session *chartSession, totalPages int) []dc.LayoutComponent {

	var rows []dc.LayoutComponent
	minValues := 1

	// Threshold menu
	if session.percent >= 0 {
		thresholdOptions := []dc.SelectOption{}
		for p := 0; p <= 50; p += 5 {
			thresholdOptions = append(thresholdOptions, dc.SelectOption{
				Label:   fmt.Sprintf("Below %d%% of max CS", 100-p),
				Value:   fmt.Sprintf("%d", p),
				Default: session.percent == p,
			})
		}
		rows = append(rows, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.SelectMenu{
					CustomID:    fmt.Sprintf("chart#threshold#%s", session.uuidStr),
					Placeholder: "Select threshold...",
					Options:     thresholdOptions,
					MinValues:   &minValues,
					MaxValues:   1,
				},
			},
		})
	}

	// Season navigation buttons (season chart only)
	if session.percent == -100 {
		if navRow := buildSeasonNavButtons(session); len(navRow.Components) > 0 {
			rows = append(rows, navRow)
		}
	}

	sortOptions := []dc.SelectOption{

		{Label: "Sort by Date (Newest First)", Value: "date", Default: session.sortBy == "date"},
		{Label: "Sort by Date (Oldest First)", Value: "date_asc", Default: session.sortBy == "date_asc"},
		{Label: "Sort by Prediction (Soonest First)", Value: "pred", Default: session.sortBy == "pred"},
		{Label: "Sort by Prediction (Latest First)", Value: "pred_desc", Default: session.sortBy == "pred_desc"},
		{Label: "Sort by CS Gap (Highest First)", Value: "gap", Default: session.sortBy == "gap"},
		{Label: "Sort by CS Gap (Lowest First)", Value: "gap_asc", Default: session.sortBy == "gap_asc"},
		{Label: "Sort by % of Max (Lowest First)", Value: "percent", Default: session.sortBy == "percent"},
		{Label: "Sort by % of Max (Highest First)", Value: "percent_desc", Default: session.sortBy == "percent_desc"},
		{Label: "Sort by CS (Highest First)", Value: "cs", Default: session.sortBy == "cs"},
		{Label: "Sort by CS (Lowest First)", Value: "cs_asc", Default: session.sortBy == "cs_asc"},
		{Label: "Sort by Name (A-Z)", Value: "name", Default: session.sortBy == "name"},
		{Label: "Sort by Name (Z-A)", Value: "name_desc", Default: session.sortBy == "name_desc"},
		{Label: "Sort by ID (A-Z)", Value: "id", Default: session.sortBy == "id"},
		{Label: "Sort by ID (Z-A)", Value: "id_desc", Default: session.sortBy == "id_desc"},
	}

	rows = append(rows, dc.ActionRow{
		Components: []dc.InteractiveComponent{
			dc.SelectMenu{
				CustomID:    fmt.Sprintf("chart#sort#%s", session.uuidStr),
				Placeholder: "Sort order...",
				Options:     sortOptions,
				MinValues:   &minValues,
				MaxValues:   1,
			},
		},
	})

	var pageButtons []dc.InteractiveComponent
	if totalPages > 1 {
		if totalPages > 4 {
			pageButtons = append(pageButtons, dc.Button{
				Label:    "First",
				Style:    dc.ButtonSecondary,
				CustomID: fmt.Sprintf("chart#first#%s", session.uuidStr),
				Disabled: session.page <= 0,
			})
		}
		pageButtons = append(pageButtons, dc.Button{
			Label:    "Prev",
			Style:    dc.ButtonSecondary,
			CustomID: fmt.Sprintf("chart#prev#%s", session.uuidStr),
			Disabled: session.page <= 0,
		})
		pageButtons = append(pageButtons, dc.Button{
			Label:    "Next",
			Style:    dc.ButtonSecondary,
			CustomID: fmt.Sprintf("chart#next#%s", session.uuidStr),
			Disabled: session.page >= totalPages-1,
		})
		if totalPages > 4 {
			pageButtons = append(pageButtons, dc.Button{
				Label:    "Last",
				Style:    dc.ButtonSecondary,
				CustomID: fmt.Sprintf("chart#last#%s", session.uuidStr),
				Disabled: session.page >= totalPages-1,
			})
		}
	}

	if len(pageButtons) > 0 {
		rows = append(rows, dc.ActionRow{Components: pageButtons})
	}

	var actionButtons []dc.InteractiveComponent
	viewLabel := "Mobile View"
	if session.mobileFriendly {
		viewLabel = "Desktop View"
	}
	actionButtons = append(actionButtons, dc.Button{
		Label:    viewLabel,
		Style:    dc.ButtonPrimary,
		CustomID: fmt.Sprintf("chart#toggleview#%s", session.uuidStr),
	})
	siabLabel := "Show SIAB Only"
	siabStyle := dc.ButtonSecondary
	if session.siabOnly {
		siabLabel = "Show All Contracts"
		siabStyle = dc.ButtonPrimary
	}
	actionButtons = append(actionButtons, dc.Button{
		Label:    siabLabel,
		Style:    siabStyle,
		CustomID: fmt.Sprintf("chart#togglesiab#%s", session.uuidStr),
	})
	ggLabel := "Standard View"
	ggEmoji := ei.GetBotComponentEmoji("token")
	ggStyle := dc.ButtonSecondary
	if session.generousGift {
		ggLabel = "Generous Gift"
		ggEmoji = ei.GetBotComponentEmoji("std_gg")
		ggStyle = dc.ButtonSuccess
	}
	actionButtons = append(actionButtons, dc.Button{
		Label:    ggLabel,
		Emoji:    ggEmoji,
		Style:    ggStyle,
		CustomID: fmt.Sprintf("chart#togglegg#%s", session.uuidStr),
	})
	actionButtons = append(actionButtons, dc.Button{
		Label:    "Watch Filtered",
		Style:    dc.ButtonSuccess,
		CustomID: fmt.Sprintf("chart#watchfiltered#%s", session.uuidStr),
	})
	actionButtons = append(actionButtons, dc.Button{
		Label:    "Finish",
		Style:    dc.ButtonDanger,
		CustomID: fmt.Sprintf("chart#finish#%s", session.uuidStr),
	})
	rows = append(rows, dc.ActionRow{Components: actionButtons})

	return rows
}

// HandleChartReactions handles button and select menu interactions for the chart view.
func HandleChartReactions(e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 3 {
		return
	}

	action := parts[1]
	uuidPart := parts[2]
	userID := e.UserID()

	chartSessionsMutex.Lock()
	session, ok := chartSessions[uuidPart]
	chartSessionsMutex.Unlock()
	if !ok {
		_ = e.Respond(dc.Message{
			Content:   "This chart session has expired. Please run the command again.",
			Ephemeral: true,
		})
		return
	}

	if session.userID != userID {
		_ = e.Respond(dc.Message{
			Content:   "This is restricted to the user that originally ran the command.",
			Ephemeral: true,
		})
		return
	}

	session.expiresAt = time.Now().Add(15 * time.Minute)

	switch action {
	case "sort":
		values := e.Values()
		if len(values) > 0 {
			newSortBy := values[0]
			if isValidChartSortBy(newSortBy) {
				session.sortBy = newSortBy
				if session.percent == -200 {
					farmerstate.SetMiscSettingString(session.userID, predictionsSortByMiscKey, newSortBy)
				} else {
					farmerstate.SetMiscSettingString(session.userID, rerunSortByMiscKey, newSortBy)
				}
				session.page = 0 // Reset to first page on sort
			}
		}
	case "first":
		session.page = 0
	case "prev":
		session.page--
	case "next":
		session.page++
	case "last":
		session.page = 999999 // Let renderChartSession clamp this to the actual last page
	case "toggleview":
		session.mobileFriendly = !session.mobileFriendly
		farmerstate.SetMiscSettingString(session.userID, "rerunMobileFriendly", strconv.FormatBool(session.mobileFriendly))
	case "togglesiab":
		session.siabOnly = !session.siabOnly
		session.page = 0 // Reset to first page on filter change
	case "togglegg":
		session.generousGift = !session.generousGift
		farmerstate.SetMiscSettingString(session.userID, "rerunGenerousGift", strconv.FormatBool(session.generousGift))
		session.page = 0 // Reset to first page on filter change
	case "threshold":
		values := e.Values()
		if len(values) > 0 {
			newPercent, err := strconv.Atoi(values[0])
			if err == nil {
				session.percent = newPercent
				session.page = 0 // Reset to first page on filter change
			}
		}
	case "watchfiltered":
		now := time.Now().Unix()
		var displayRows []chartRow
		switch session.percent {
		case -1: // Active contracts chart
			for _, r := range session.rows {
				if r.validUntil > now {
					displayRows = append(displayRows, r)
				}
			}
		case -100: // Season chart – show all contracts, no time or percent filter
			displayRows = session.rows
		case -200: // Predictions chart
			displayRows = session.rows
		default: // Threshold chart
			for _, r := range session.rows {
				if r.percent < float64(100-session.percent) {
					displayRows = append(displayRows, r)
				}
			}
		}

		if session.siabOnly {
			var siabRows []chartRow
			for _, r := range displayRows {
				if r.hasSiab {
					siabRows = append(siabRows, r)
				}
			}
			displayRows = siabRows
		}

		// Add watches for all of these contracts (skipping currently active ones)
		count := 0
		activeContracts := make(map[string]bool)
		for _, c := range ei.EggIncContracts {
			if !c.Predicted {
				activeContracts[c.ID] = true
			}
		}

		for _, r := range displayRows {
			if activeContracts[r.contractID] {
				continue
			}
			farmerstate.AddWatch(userID, "contract", r.contractID)
			count++
		}

		_ = e.Respond(dc.Message{
			Content:   fmt.Sprintf("Success! Added watch for %d contracts meeting the current chart conditions.", count),
			Ephemeral: true,
		})
		return
	case "finish":
		// Remove interactive components, keeping the chart itself
		_ = e.Update(dc.Message{Components: e.MessageComponentsWithoutActionRows()})
		chartSessionsMutex.Lock()
		delete(chartSessions, uuidPart) // Clean up session
		chartSessionsMutex.Unlock()
		return
	case "seasonswitch":
		// parts: chart # seasonswitch # uuid # newSeasonScope [# tag]
		if len(parts) < 4 {
			log.Printf("[rerun-chart] seasonswitch invalid customID: %s", e.CustomID())
			_ = e.Respond(dc.Message{
				Content:   "Invalid season navigation request.",
				Ephemeral: true,
			})
			return
		}
		newScope := parts[3]
		if !leaderboardSeasonExists(newScope) {
			log.Printf("[rerun-chart] seasonswitch unknown season scope: %s", newScope)
			_ = e.Respond(dc.Message{
				Content:   fmt.Sprintf("Season %s is not recognized.", newScope),
				Ephemeral: true,
			})
			return
		}
		session.rows = buildSeasonRows(session, newScope)
		session.seasonScope = newScope
		session.page = 0
	}

	components := renderChartSession(session)

	if err := e.Update(dc.Message{
		Ephemeral:  e.MessageIsEphemeral(),
		Components: components,
	}); err != nil {
		log.Printf("[rerun-chart] error updating chart message: %v", err)
	}
}
