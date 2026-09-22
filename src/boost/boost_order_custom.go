package boost

import (
	"bytes"
	"fmt"
	"math"
	"math/rand/v2"
	"regexp"
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

const (
	customOrderHandlerPrefix = "bo_custom"
	customOrderSessionTTL    = 15 * time.Minute
)

type customOrderSession struct {
	uuidStr      string
	contractHash string
	channelID    string
	userID       string
	lines        []string
	expiresAt    time.Time
}

var (
	customOrderSessions      = make(map[string]*customOrderSession)
	customOrderSessionsMutex sync.Mutex
)

func getOrCreateCustomOrderSession(userID string, contractHash string, lines []string) *customOrderSession {
	customOrderSessionsMutex.Lock()
	defer customOrderSessionsMutex.Unlock()

	now := time.Now()
	for k, s := range customOrderSessions {
		if s.expiresAt.Before(now) {
			delete(customOrderSessions, k)
		}
	}

	for _, s := range customOrderSessions {
		if s.userID == userID && s.contractHash == contractHash {
			s.expiresAt = now.Add(customOrderSessionTTL)
			if len(lines) > 0 {
				s.lines = append([]string(nil), lines...)
			}
			return s
		}
	}

	session := &customOrderSession{
		uuidStr:      uuid.NewV7().String(),
		contractHash: contractHash,
		userID:       userID,
		lines:        append([]string(nil), lines...),
		expiresAt:    now.Add(customOrderSessionTTL),
	}
	customOrderSessions[session.uuidStr] = session
	return session
}

func getCustomOrderSession(uuidStr string) *customOrderSession {
	customOrderSessionsMutex.Lock()
	defer customOrderSessionsMutex.Unlock()
	return customOrderSessions[uuidStr]
}

func clearCustomOrderSession(uuidStr string) {
	customOrderSessionsMutex.Lock()
	defer customOrderSessionsMutex.Unlock()
	delete(customOrderSessions, uuidStr)
}

// CustomCriterionType enumerates the metric types in custom boost order rules.
type CustomCriterionType int

const (
	CritDeflEffort CustomCriterionType = iota
	CritCraftDefl
	CritIHR
	CritELR
	CritTE
	CritTokens
	CritTVal
	CritDefl
	CritDeflSlot
	CritDeliv
	CritSignup
	CritReverse
	CritRandom
	CritUnknown
)

type customCriterion struct {
	raw       string
	critType  CustomCriterionType
	ascending bool // true = ascending (>), false = descending (<)
	effortN   int  // equivalence craft count for DEFL_EFFORT (default 50)
	fuzzyPct  float64
	fuzzySqrt bool
}

var (
	reDeflEffort = regexp.MustCompile(`(?i)DEFL_EFFORT(?:\[(\d+)\])?`)
	reCraftDefl  = regexp.MustCompile(`(?i)(?:CRAFT_DEFL|CRAFT\(T4_DEFL\))`)
	reFuzzyPct   = regexp.MustCompile(`\[(\d+(?:\.\d+)?)%\]`)
	reFuzzySqrt  = regexp.MustCompile(`(?i)\[(?:sqrt|~)\]`)
)

func parseCustomCriterion(s string) customCriterion {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return customCriterion{critType: CritUnknown}
	}

	crit := customCriterion{
		raw:       trimmed,
		ascending: false, // Default: descending (<), higher is better
		effortN:   50,    // Default 50 craft attempts
	}

	cur := trimmed
	// Strip outer brackets if given e.g. <DEFL_EFFORT[50]>
	if strings.HasPrefix(cur, "<") && strings.HasSuffix(cur, ">") && !strings.Contains(cur[1:len(cur)-1], "<") && !strings.Contains(cur[1:len(cur)-1], ">") {
		cur = cur[1 : len(cur)-1]
	}

	cur = strings.TrimSpace(cur)
	if strings.HasPrefix(cur, ">") || strings.HasPrefix(cur, "+") {
		crit.ascending = true
		cur = strings.TrimSpace(cur[1:])
	} else if strings.HasPrefix(cur, "<") || strings.HasPrefix(cur, "-") {
		crit.ascending = false
		cur = strings.TrimSpace(cur[1:])
	}

	// Check fuzzy modifiers
	if m := reFuzzyPct.FindStringSubmatch(cur); len(m) > 1 {
		if pct, err := strconv.ParseFloat(m[1], 64); err == nil {
			crit.fuzzyPct = pct / 100.0
		}
	}
	if reFuzzySqrt.MatchString(cur) {
		crit.fuzzySqrt = true
	}

	upper := strings.ToUpper(cur)

	switch {
	case reDeflEffort.MatchString(upper):
		crit.critType = CritDeflEffort
		m := reDeflEffort.FindStringSubmatch(upper)
		if len(m) > 1 && m[1] != "" {
			if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
				crit.effortN = n
			}
		}
	case reCraftDefl.MatchString(upper):
		crit.critType = CritCraftDefl
	case strings.HasPrefix(upper, "IHR"):
		crit.critType = CritIHR
	case strings.HasPrefix(upper, "ELR"):
		crit.critType = CritELR
	case strings.HasPrefix(upper, "TE"):
		crit.critType = CritTE
	case strings.HasPrefix(upper, "TOKEN") || strings.HasPrefix(upper, "TASK"):
		crit.critType = CritTokens
		// For tokens wanted, ascending order is naturally preferred (fewer tokens boost earlier)
		// unless explicitly prefixed with <
		if !strings.HasPrefix(trimmed, "<") && !strings.HasPrefix(trimmed, "-") {
			crit.ascending = true
		}
	case strings.HasPrefix(upper, "TVAL"):
		crit.critType = CritTVal
	case strings.HasPrefix(upper, "DEFL_SLOT"):
		crit.critType = CritDeflSlot
	case strings.HasPrefix(upper, "DEFL"):
		crit.critType = CritDefl
	case strings.HasPrefix(upper, "DELIV") || strings.HasPrefix(upper, "DEL"):
		crit.critType = CritDeliv
	case strings.HasPrefix(upper, "SIGNUP") || strings.HasPrefix(upper, "JOIN"):
		crit.critType = CritSignup
		if !strings.HasPrefix(trimmed, "<") && !strings.HasPrefix(trimmed, "-") {
			crit.ascending = true
		}
	case strings.HasPrefix(upper, "REVERSE"):
		crit.critType = CritReverse
	case strings.HasPrefix(upper, "RANDOM"):
		crit.critType = CritRandom
	default:
		crit.critType = CritUnknown
	}

	return crit
}

// getBoosterT4DeflectorCraftCount retrieves how many T4 Deflectors the booster has crafted.
func getBoosterT4DeflectorCraftCount(userID string) int {
	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	if eiID == "" && !isDiscordSnowflake(userID) {
		if discordID, err := farmerstate.GetDiscordUserIDFromEiIgnExact(userID); err == nil && discordID != "" {
			eiID = farmerstate.GetMiscSettingString(discordID, "encrypted_ei_id")
		}
	}
	if eiID == "" {
		return 0
	}
	backup, _ := ei.GetFirstContactFromAPI(eiID, userID, true)
	if backup == nil || backup.GetArtifactsDb() == nil {
		return 0
	}
	for _, a := range backup.GetArtifactsDb().GetArtifactStatus() {
		if a != nil && a.Spec != nil &&
			a.Spec.GetName() == ei.ArtifactSpec_TACHYON_DEFLECTOR &&
			a.Spec.GetLevel() == ei.ArtifactSpec_GREATER {
			return int(a.GetCount())
		}
	}
	return 0
}

// hasBoosterT4LDeflector checks if the booster possesses a T4L Deflector.
func hasBoosterT4LDeflector(b *Booster) bool {
	if b == nil {
		return false
	}
	for _, a := range b.ArtifactSet.Artifacts {
		if (a.Type == "Deflector" || a.Type == "IHR Deflector") && strings.EqualFold(a.Quality, "T4L") {
			return true
		}
	}
	quality := strings.ToUpper(strings.TrimSpace(farmerstate.GetMiscSettingString(b.UserID, "defl")))
	if strings.Contains(quality, "T4L") {
		return true
	}
	qualityIHR := strings.ToUpper(strings.TrimSpace(farmerstate.GetMiscSettingString(b.UserID, "defl-ihr")))
	if strings.Contains(qualityIHR, "T4L") {
		return true
	}
	return false
}

// getBoosterDeflectorQualityString returns display string of booster's deflector.
func getBoosterDeflectorQualityString(b *Booster) string {
	if b == nil {
		return "-"
	}
	for _, a := range b.ArtifactSet.Artifacts {
		if a.Type == "Deflector" || a.Type == "IHR Deflector" {
			if a.Quality != "" && a.Quality != "NONE" {
				return a.Quality
			}
		}
	}
	defl := farmerstate.GetMiscSettingString(b.UserID, "defl")
	if defl != "" && defl != "NONE" {
		return defl
	}
	deflIHR := farmerstate.GetMiscSettingString(b.UserID, "defl-ihr")
	if deflIHR != "" && deflIHR != "NONE" {
		return deflIHR
	}
	return "-"
}

// calculateBoosterDeflectorEffort calculates the balanced effort score for DEFL_EFFORT[N]:
// - If player owns T4L Deflector: score = N
// - If player does not own T4L: score = min(N, craft_attempts)
func calculateBoosterDeflectorEffort(b *Booster, n int) (score int, craftCount int, hasT4L bool) {
	if b == nil {
		return 0, 0, false
	}
	hasT4L = hasBoosterT4LDeflector(b)
	craftCount = getBoosterT4DeflectorCraftCount(b.UserID)
	if hasT4L {
		score = n
	} else {
		score = min(n, craftCount)
	}
	return score, craftCount, hasT4L
}

type boosterEvalData struct {
	userID        string
	signupIndex   int
	hasT4L        bool
	t4Crafts      int
	deflQuality   string
	ihrBase       float64
	ihrSort       float64
	elr           float64
	teBase        int
	teSort        float64
	tokensWanted  int
	deflScore     int
	deflSlotScore int
	delivScore    int
	rowScores     [4]float64
	effortScores  [4]int
}

func evaluateBoosterForCustom(contract *Contract, userID string, signupIdx int, criteria [4]customCriterion, applyFuzzy bool) boosterEvalData {
	b := contract.Boosters[userID]
	data := boosterEvalData{
		userID:      userID,
		signupIndex: signupIdx,
	}

	if b != nil {
		data.hasT4L = hasBoosterT4LDeflector(b)
		data.t4Crafts = getBoosterT4DeflectorCraftCount(b.UserID)
		data.deflQuality = getBoosterDeflectorQualityString(b)
		data.ihrBase = b.IHRRate
		data.ihrSort = b.IHRRate
		data.elr = b.ArtifactSet.LayRate
		data.teBase = b.TECount
		data.teSort = float64(max(b.TECount, 0))
		data.tokensWanted = b.TokensWanted
		data.deflScore = getArtifactQualityScore(b, "Deflector")
		data.deflSlotScore = getESCDeflectorScore(b, false)
		data.delivScore = getArtifactQualityScore(b, "Metronome") + getArtifactQualityScore(b, "Compass") + getArtifactQualityScore(b, "Gusset")
	}

	for rowIdx, crit := range criteria {
		score := 0.0
		switch crit.critType {
		case CritDeflEffort:
			effScore, _, _ := calculateBoosterDeflectorEffort(b, crit.effortN)
			data.effortScores[rowIdx] = effScore
			score = float64(effScore)
		case CritCraftDefl:
			score = float64(data.t4Crafts)
		case CritIHR:
			val := data.ihrBase
			if applyFuzzy && crit.fuzzyPct > 0 {
				bonusMax := val * crit.fuzzyPct
				offset := (rand.Float64()*2 - 1) * bonusMax
				val += offset
				data.ihrSort = val
				if b != nil {
					b.FuzzyOffset = offset
				}
			}
			score = val
		case CritELR:
			score = data.elr
		case CritTE:
			val := float64(data.teBase)
			if applyFuzzy {
				if crit.fuzzySqrt {
					baseTE := float64(max(data.teBase, 0))
					bonusMax := math.Max(baseTE*0.06, math.Sqrt(baseTE+25))
					offset := (rand.Float64()*2 - 1) * bonusMax
					val = baseTE + offset
					data.teSort = val
					if b != nil {
						b.FuzzyOffset = offset
					}
				} else if crit.fuzzyPct > 0 {
					bonusMax := val * crit.fuzzyPct
					offset := (rand.Float64()*2 - 1) * bonusMax
					val += offset
					data.teSort = val
					if b != nil {
						b.FuzzyOffset = offset
					}
				}
			}
			score = val
		case CritTokens:
			score = float64(data.tokensWanted)
		case CritTVal:
			_, _, tvalByUser, _ := buildTokenTotalsFromLog(contract)
			score = tvalByUser[data.userID]
		case CritDefl:
			score = float64(data.deflScore)
		case CritDeflSlot:
			score = float64(data.deflSlotScore)
		case CritDeliv:
			score = float64(data.delivScore)
		case CritSignup:
			score = float64(data.signupIndex)
		case CritReverse:
			score = -float64(data.signupIndex)
		case CritRandom:
			score = rand.Float64()
		default:
			score = 0.0
		}
		data.rowScores[rowIdx] = score
	}

	return data
}

// sortCustomRemaining sorts boosters using the custom condition strings.
func sortCustomRemaining(contract *Contract, unselected []string, lines []string, applyFuzzy ...bool) []string {
	if len(unselected) <= 1 {
		return append([]string(nil), unselected...)
	}

	doFuzzy := len(applyFuzzy) > 0 && applyFuzzy[0]

	var criteria [4]customCriterion
	for i := 0; i < 4; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		criteria[i] = parseCustomCriterion(line)
	}

	signupIndexMap := make(map[string]int, len(contract.Order))
	for idx, id := range contract.Order {
		signupIndexMap[id] = idx
	}

	items := make([]boosterEvalData, len(unselected))
	for i, userID := range unselected {
		signupIdx := i
		if pos, ok := signupIndexMap[userID]; ok {
			signupIdx = pos
		}
		items[i] = evaluateBoosterForCustom(contract, userID, signupIdx, criteria, doFuzzy)
	}

	sort.SliceStable(items, func(i, j int) bool {
		for r := 0; r < 4; r++ {
			if criteria[r].critType == CritUnknown {
				continue
			}
			valI := items[i].rowScores[r]
			valJ := items[j].rowScores[r]
			if math.Abs(valI-valJ) > 1e-6 {
				if criteria[r].ascending {
					return valI < valJ
				}
				return valI > valJ
			}
		}
		// Fallback tiebreaker: signup order then userID
		if items[i].signupIndex != items[j].signupIndex {
			return items[i].signupIndex < items[j].signupIndex
		}
		return items[i].userID < items[j].userID
	})

	sorted := make([]string, len(items))
	for i, item := range items {
		sorted[i] = item.userID
	}
	return sorted
}

// RenderCustomOrderTableImage renders a PNG table image of the custom boost order.
func RenderCustomOrderTableImage(contract *Contract, lines []string) ([]byte, error) {
	if contract == nil {
		return nil, fmt.Errorf("contract is nil")
	}

	unselected := append([]string(nil), contract.Order...)
	sortedIDs := sortCustomRemaining(contract, unselected, lines, false)

	var criteria [4]customCriterion
	for i := 0; i < 4; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		criteria[i] = parseCustomCriterion(line)
	}

	effortN := 50
	for _, c := range criteria {
		if c.critType == CritDeflEffort {
			effortN = c.effortN
			break
		}
	}

	cols := []TableImageColumn{
		{Label: "#", Align: bottools.StringAlignRight},
		{Label: "Player", Align: bottools.StringAlignLeft},
		{Label: "Deflector", Align: bottools.StringAlignCenter},
		{Label: "T4 Crafts", Align: bottools.StringAlignRight},
		{Label: fmt.Sprintf("Effort [N=%d]", effortN), Align: bottools.StringAlignCenter},
		{Label: "ELR", Align: bottools.StringAlignRight},
		{Label: "IHR", Align: bottools.StringAlignRight},
		{Label: "Tokens", Align: bottools.StringAlignRight},
		{Label: "TE", Align: bottools.StringAlignRight},
	}

	var tableRows []TableImageRow
	for idx, userID := range sortedIDs {
		b := contract.Boosters[userID]
		if b == nil {
			continue
		}
		name := b.Nick
		if name == "" {
			name = b.UserName
		}
		if name == "" {
			name = b.GlobalName
		}
		if name == "" {
			name = userID
		}

		effScore, craftCount, hasT4L := calculateBoosterDeflectorEffort(b, effortN)
		deflQuality := getBoosterDeflectorQualityString(b)

		deflColor := ""
		if hasT4L {
			deflColor = "green"
		} else if strings.HasPrefix(deflQuality, "T4") {
			deflColor = "blue"
		}

		effortDisplay := fmt.Sprintf("%d", effScore)
		effortColor := ""
		if hasT4L {
			effortDisplay = fmt.Sprintf("%d (T4L)", effScore)
			effortColor = "green"
		} else if craftCount >= effortN {
			effortDisplay = fmt.Sprintf("%d (Capped)", effScore)
			effortColor = "green"
		} else if effScore > 0 {
			effortColor = "blue"
		}

		craftsDisplay := fmt.Sprintf("%d", craftCount)
		craftsColor := ""
		if craftCount >= effortN {
			craftsColor = "green"
		}

		ihrMult := fmt.Sprintf("%0.2fx", b.IHRRate/DefaultLeggyIHR)
		elrStr := fmt.Sprintf("%0.2f", b.ArtifactSet.LayRate)
		tokensStr := fmt.Sprintf("%d", b.TokensWanted)
		teStr := fmt.Sprintf("%d", b.TECount)

		cells := []TableImageCell{
			{Text: fmt.Sprintf("%d", idx+1), Color: ""},
			{Text: name, Color: ""},
			{Text: deflQuality, Color: deflColor},
			{Text: craftsDisplay, Color: craftsColor},
			{Text: effortDisplay, Color: effortColor},
			{Text: elrStr, Color: ""},
			{Text: ihrMult, Color: ""},
			{Text: tokensStr, Color: ""},
			{Text: teStr, Color: ""},
		}
		tableRows = append(tableRows, TableImageRow{Cells: cells})
	}

	return RenderTableImage(cols, tableRows)
}

// BuildCustomOrderMessage formats the preview catalyst message with table image and action buttons.
func BuildCustomOrderMessage(contract *Contract, session *customOrderSession, status string) dc.Message {
	if contract == nil || session == nil {
		return dc.Message{Content: "Contract or session not found.", Ephemeral: true}
	}

	var headerSb strings.Builder
	fmt.Fprintf(&headerSb, "## ⚙️ Custom Boost Order Preview\n")
	fmt.Fprintf(&headerSb, "**Contract:** `%s` | **Coop:** `%s`\n", contract.ContractID, contract.CoopID)

	var ruleSb strings.Builder
	for i := 0; i < 4; i++ {
		line := "-"
		if i < len(session.lines) && strings.TrimSpace(session.lines[i]) != "" {
			line = session.lines[i]
		}
		fmt.Fprintf(&ruleSb, "**Row %d:** `%s`\n", i+1, line)
	}

	actionRow1 := dc.ActionRow{
		Components: []dc.InteractiveComponent{
			dc.Button{
				Label:    "EVAL",
				Style:    dc.ButtonPrimary,
				CustomID: fmt.Sprintf("%s#%s#eval#%s", customOrderHandlerPrefix, session.uuidStr, contract.ContractHash),
			},
			dc.Button{
				Label:    "EDIT",
				Style:    dc.ButtonSecondary,
				CustomID: fmt.Sprintf("%s#%s#edit#%s", customOrderHandlerPrefix, session.uuidStr, contract.ContractHash),
			},
			dc.Button{
				Label:    "SAVE PRESET",
				Style:    dc.ButtonSecondary,
				CustomID: fmt.Sprintf("%s#%s#save#%s", customOrderHandlerPrefix, session.uuidStr, contract.ContractHash),
			},
			dc.Button{
				Label:    "LOAD PRESET",
				Style:    dc.ButtonSecondary,
				CustomID: fmt.Sprintf("%s#%s#load#%s", customOrderHandlerPrefix, session.uuidStr, contract.ContractHash),
			},
		},
	}

	actionRow2 := dc.ActionRow{
		Components: []dc.InteractiveComponent{
			dc.Button{
				Label:    "SAVE & EXIT",
				Style:    dc.ButtonSuccess,
				CustomID: fmt.Sprintf("%s#%s#apply#%s", customOrderHandlerPrefix, session.uuidStr, contract.ContractHash),
			},
			dc.Button{
				Label:    "EXIT",
				Style:    dc.ButtonDanger,
				CustomID: fmt.Sprintf("%s#%s#exit#%s", customOrderHandlerPrefix, session.uuidStr, contract.ContractHash),
			},
		},
	}

	imgBytes, err := RenderCustomOrderTableImage(contract, session.lines)
	components := []dc.LayoutComponent{
		dc.TextDisplay{Content: headerSb.String()},
		dc.TextDisplay{Content: ruleSb.String()},
	}

	var files []dc.File
	if err == nil && len(imgBytes) > 0 {
		components = append(components, dc.MediaGallery{
			Items: []dc.MediaItem{{URL: "attachment://custom_order_preview.png"}},
		})
		files = []dc.File{{
			Name:        "custom_order_preview.png",
			ContentType: "image/png",
			Reader:      bytes.NewReader(imgBytes),
		}}
	}

	if status != "" {
		components = append(components, dc.TextDisplay{Content: status})
	}
	components = append(components, actionRow1, actionRow2)

	return dc.Message{
		Components: components,
		Files:      files,
		Ephemeral:  true,
	}
}

// HandleCustomOrderReactions processes button clicks on the Custom Boost Order preview catalyst.
func HandleCustomOrderReactions(client dc.Client, e *dc.ComponentEvent) {
	reaction := strings.Split(e.CustomID(), "#")
	if len(reaction) < 4 {
		_ = e.Update(dc.Message{Content: "Invalid catalyst interaction path. Please rerun.", Ephemeral: true})
		return
	}

	uuidStr := reaction[1]
	action := reaction[2]
	contractHash := reaction[3]

	session := getCustomOrderSession(uuidStr)
	if session == nil {
		_ = e.Update(dc.Message{Content: "This catalyst session expired. Please reselect Custom Boost Order.", Ephemeral: true})
		return
	}

	contract := FindContractByHash(contractHash)
	if contract == nil {
		_ = e.Update(dc.Message{Content: "Unable to find this contract.", Ephemeral: true})
		return
	}

	switch action {
	case "eval":
		_ = e.DeferUpdate()
		msg := BuildCustomOrderMessage(contract, session, "✓ Evaluated and refreshed boost order preview.")
		_ = e.Update(msg)

	case "edit":
		SendCustomBoostOrderModal(e, contractHash)

	case "save":
		_ = e.DeferUpdate()
		savedStr := strings.Join(session.lines, "\n")
		farmerstate.SetMiscSettingString(session.userID, "custom_boost_order", savedStr)
		msg := BuildCustomOrderMessage(contract, session, "✓ Saved conditions to your personal custom boost order preset!")
		_ = e.Update(msg)

	case "load":
		_ = e.DeferUpdate()
		saved := farmerstate.GetMiscSettingString(session.userID, "custom_boost_order")
		if saved == "" {
			msg := BuildCustomOrderMessage(contract, session, "⚠️ No saved custom boost order preset found in your profile.")
			_ = e.Update(msg)
			return
		}
		parts := strings.Split(saved, "\n")
		session.lines = parts
		msg := BuildCustomOrderMessage(contract, session, "✓ Loaded custom boost order preset from your profile!")
		_ = e.Update(msg)

	case "apply":
		_ = e.DeferUpdate()
		contract.mutex.Lock()
		contract.CustomOrderLines = append([]string(nil), session.lines...)
		contract.BoostOrder = ContractOrderCustom
		unselected := append([]string(nil), contract.Order...)
		contract.Order = sortCustomRemaining(contract, unselected, contract.CustomOrderLines, false)
		contract.mutex.Unlock()

		saveData(contract.ContractHash)
		refreshBoostListMessage(client, contract, false)
		clearCustomOrderSession(session.uuidStr)

		_ = e.Update(dc.Message{
			Content:   "✅ Custom Boost Order applied to contract and boost list updated.",
			Ephemeral: true,
		})

	case "exit":
		clearCustomOrderSession(session.uuidStr)
		_ = e.Update(dc.Message{
			Content:   "Exited without saving custom boost order changes.",
			Ephemeral: true,
		})

	default:
		_ = e.Update(dc.Message{Content: "Unknown action.", Ephemeral: true})
	}
}
