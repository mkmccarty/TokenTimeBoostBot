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
	CritRole
	CritArtifactCount
	CritArtifactCraft
	CritUnknown
)

type customRoleCondition int

const (
	roleConditionNone customRoleCondition = iota
	roleConditionMain
	roleConditionHelper
)

type customCriterion struct {
	raw           string
	critType      CustomCriterionType
	ascending     bool // true = ascending (>), false = descending (<)
	effortN       int  // equivalence craft count for DEFL_EFFORT (default 50)
	fuzzyPct      float64
	fuzzySqrt     bool
	isConditional bool
	targetRole    customRoleCondition
	thenCrit      *customCriterion
	hasElse       bool
	elseCrit      *customCriterion

	artName   ei.ArtifactSpec_Name
	artLevel  int // 0..3 for T1..T4, -1 for any
	artRarity int // 0..3 for C..L, -1 for any
	artLabel  string
}

var (
	reDeflEffort  = regexp.MustCompile(`(?i)DEFL_EFFORT(?:\[(\d+)\])?`)
	reFuzzyPct    = regexp.MustCompile(`\[(\d+(?:\.\d+)?)%\]`)
	reFuzzySqrt   = regexp.MustCompile(`(?i)\[(?:sqrt|~)\]`)
	reConditional = regexp.MustCompile(`(?i)^\s*IF\s+(?:\(?\s*ROLE\s*(==|=|!=|<>)?\s*)?([A-Za-z]+)\)?(?:\s+THEN)?\s+(.+?)(?:\s+ELSE\s+(.+))?$`)
	reCraftExpr   = regexp.MustCompile(`(?i)^(?:CRAFTS?[\(\[]\s*([A-Za-z0-9_ -]+)\s*[\)\]]|CRAFT_([A-Za-z0-9_ -]+)|([A-Za-z0-9_ -]+)_CRAFTS?)$`)
	reArtWrapper  = regexp.MustCompile(`(?i)^(?:ARTIFACT|ART|COUNT)[\(\[]\s*([A-Za-z0-9_ -]+)\s*[\)\]]$`)
	reTierRarity  = regexp.MustCompile(`(?i)\bT([1-4])([CREL])?\b`)
	reRarityWord  = regexp.MustCompile(`(?i)\b(LEGENDARY|LEGGY|EPIC|RARE|COMMON)\b`)
	reWhitespace  = regexp.MustCompile(`[\s\-]+`)
)

var artifactAliasMap = map[string]ei.ArtifactSpec_Name{
	"ACTUATOR":             ei.ArtifactSpec_TITANIUM_ACTUATOR,
	"TITANIUM_ACTUATOR":    ei.ArtifactSpec_TITANIUM_ACTUATOR,
	"DEFLECTOR":            ei.ArtifactSpec_TACHYON_DEFLECTOR,
	"TACHYON_DEFLECTOR":    ei.ArtifactSpec_TACHYON_DEFLECTOR,
	"DEFL":                 ei.ArtifactSpec_TACHYON_DEFLECTOR,
	"METRONOME":            ei.ArtifactSpec_QUANTUM_METRONOME,
	"QUANTUM_METRONOME":    ei.ArtifactSpec_QUANTUM_METRONOME,
	"METR":                 ei.ArtifactSpec_QUANTUM_METRONOME,
	"COMPASS":              ei.ArtifactSpec_INTERSTELLAR_COMPASS,
	"INTERSTELLAR_COMPASS": ei.ArtifactSpec_INTERSTELLAR_COMPASS,
	"COMP":                 ei.ArtifactSpec_INTERSTELLAR_COMPASS,
	"GUSSET":               ei.ArtifactSpec_ORNATE_GUSSET,
	"ORNATE_GUSSET":        ei.ArtifactSpec_ORNATE_GUSSET,
	"GUSS":                 ei.ArtifactSpec_ORNATE_GUSSET,
	"CHALICE":              ei.ArtifactSpec_THE_CHALICE,
	"THE_CHALICE":          ei.ArtifactSpec_THE_CHALICE,
	"BOOK":                 ei.ArtifactSpec_BOOK_OF_BASAN,
	"BOOK_OF_BASAN":        ei.ArtifactSpec_BOOK_OF_BASAN,
	"BOB":                  ei.ArtifactSpec_BOOK_OF_BASAN,
	"FEATHER":              ei.ArtifactSpec_PHOENIX_FEATHER,
	"PHOENIX_FEATHER":      ei.ArtifactSpec_PHOENIX_FEATHER,
	"ANKH":                 ei.ArtifactSpec_TUNGSTEN_ANKH,
	"TUNGSTEN_ANKH":        ei.ArtifactSpec_TUNGSTEN_ANKH,
	"BROOCH":               ei.ArtifactSpec_AURELIAN_BROOCH,
	"AURELIAN_BROOCH":      ei.ArtifactSpec_AURELIAN_BROOCH,
	"RAINSTICK":            ei.ArtifactSpec_CARVED_RAINSTICK,
	"CARVED_RAINSTICK":     ei.ArtifactSpec_CARVED_RAINSTICK,
	"CUBE":                 ei.ArtifactSpec_PUZZLE_CUBE,
	"PUZZLE_CUBE":          ei.ArtifactSpec_PUZZLE_CUBE,
	"SIAB":                 ei.ArtifactSpec_SHIP_IN_A_BOTTLE,
	"SHIP":                 ei.ArtifactSpec_SHIP_IN_A_BOTTLE,
	"SHIP_IN_A_BOTTLE":     ei.ArtifactSpec_SHIP_IN_A_BOTTLE,
	"MONOCLE":              ei.ArtifactSpec_DILITHIUM_MONOCLE,
	"DILITHIUM_MONOCLE":    ei.ArtifactSpec_DILITHIUM_MONOCLE,
	"LENS":                 ei.ArtifactSpec_MERCURYS_LENS,
	"MERCURYS_LENS":        ei.ArtifactSpec_MERCURYS_LENS,
	"TOTEM":                ei.ArtifactSpec_LUNAR_TOTEM,
	"LUNAR_TOTEM":          ei.ArtifactSpec_LUNAR_TOTEM,
	"MEDALLION":            ei.ArtifactSpec_NEODYMIUM_MEDALLION,
	"NEODYMIUM_MEDALLION":  ei.ArtifactSpec_NEODYMIUM_MEDALLION,
	"NEO_MEDALLION":        ei.ArtifactSpec_NEODYMIUM_MEDALLION,
	"BEAK":                 ei.ArtifactSpec_BEAK_OF_MIDAS,
	"BEAK_OF_MIDAS":        ei.ArtifactSpec_BEAK_OF_MIDAS,
	"LIGHT":                ei.ArtifactSpec_LIGHT_OF_EGGENDIL,
	"LIGHT_OF_EGGENDIL":    ei.ArtifactSpec_LIGHT_OF_EGGENDIL,
	"LOE":                  ei.ArtifactSpec_LIGHT_OF_EGGENDIL,
	"NECKLACE":             ei.ArtifactSpec_DEMETERS_NECKLACE,
	"DEMETERS_NECKLACE":    ei.ArtifactSpec_DEMETERS_NECKLACE,
	"VIAL":                 ei.ArtifactSpec_VIAL_MARTIAN_DUST,
	"VIAL_MARTIAN_DUST":    ei.ArtifactSpec_VIAL_MARTIAN_DUST,
	"DUST":                 ei.ArtifactSpec_VIAL_MARTIAN_DUST,
	"TACHYON_STONE":        ei.ArtifactSpec_TACHYON_STONE,
	"DILITHIUM_STONE":      ei.ArtifactSpec_DILITHIUM_STONE,
	"SHELL_STONE":          ei.ArtifactSpec_SHELL_STONE,
	"LUNAR_STONE":          ei.ArtifactSpec_LUNAR_STONE,
	"SOUL_STONE":           ei.ArtifactSpec_SOUL_STONE,
	"PROPHECY_STONE":       ei.ArtifactSpec_PROPHECY_STONE,
	"PROP_STONE":           ei.ArtifactSpec_PROPHECY_STONE,
	"QUANTUM_STONE":        ei.ArtifactSpec_QUANTUM_STONE,
	"TERRA_STONE":          ei.ArtifactSpec_TERRA_STONE,
	"LIFE_STONE":           ei.ArtifactSpec_LIFE_STONE,
	"CLARITY_STONE":        ei.ArtifactSpec_CLARITY_STONE,
}

func artifactDisplayName(name ei.ArtifactSpec_Name) string {
	switch name {
	case ei.ArtifactSpec_TITANIUM_ACTUATOR:
		return "Actuator"
	case ei.ArtifactSpec_TACHYON_DEFLECTOR:
		return "Deflector"
	case ei.ArtifactSpec_QUANTUM_METRONOME:
		return "Metronome"
	case ei.ArtifactSpec_INTERSTELLAR_COMPASS:
		return "Compass"
	case ei.ArtifactSpec_ORNATE_GUSSET:
		return "Gusset"
	case ei.ArtifactSpec_THE_CHALICE:
		return "Chalice"
	case ei.ArtifactSpec_BOOK_OF_BASAN:
		return "Book of Basan"
	case ei.ArtifactSpec_PHOENIX_FEATHER:
		return "Feather"
	case ei.ArtifactSpec_TUNGSTEN_ANKH:
		return "Ankh"
	case ei.ArtifactSpec_AURELIAN_BROOCH:
		return "Brooch"
	case ei.ArtifactSpec_CARVED_RAINSTICK:
		return "Rainstick"
	case ei.ArtifactSpec_PUZZLE_CUBE:
		return "Puzzle Cube"
	case ei.ArtifactSpec_SHIP_IN_A_BOTTLE:
		return "SIAB"
	case ei.ArtifactSpec_DILITHIUM_MONOCLE:
		return "Monocle"
	case ei.ArtifactSpec_MERCURYS_LENS:
		return "Lens"
	case ei.ArtifactSpec_LUNAR_TOTEM:
		return "Totem"
	case ei.ArtifactSpec_NEODYMIUM_MEDALLION:
		return "Medallion"
	case ei.ArtifactSpec_BEAK_OF_MIDAS:
		return "Beak"
	case ei.ArtifactSpec_LIGHT_OF_EGGENDIL:
		return "Light"
	case ei.ArtifactSpec_DEMETERS_NECKLACE:
		return "Necklace"
	case ei.ArtifactSpec_VIAL_MARTIAN_DUST:
		return "Vial"
	default:
		return name.String()
	}
}

func formatArtifactTierRarity(level int, rarity int) string {
	rarityLetters := []string{"C", "R", "E", "L"}
	if level >= 0 && rarity >= 0 && rarity < len(rarityLetters) {
		return fmt.Sprintf("T%d%s", level+1, rarityLetters[rarity])
	}
	if level >= 0 {
		return fmt.Sprintf("T%d", level+1)
	}
	return ""
}

func formatArtifactCountLabel(level int, rarity int, name ei.ArtifactSpec_Name) string {
	disp := artifactDisplayName(name)
	rarityLetters := []string{"C", "R", "E", "L"}
	if level >= 0 && rarity >= 0 && rarity < len(rarityLetters) {
		return fmt.Sprintf("T%d%s %s", level+1, rarityLetters[rarity], disp)
	}
	if level >= 0 {
		return fmt.Sprintf("T%d %s", level+1, disp)
	}
	if rarity >= 0 && rarity < len(rarityLetters) {
		rNames := []string{"Common", "Rare", "Epic", "Legendary"}
		return fmt.Sprintf("%s %s", rNames[rarity], disp)
	}
	return disp
}

func parseArtifactSpecTokens(s string, isCraft bool) (artName ei.ArtifactSpec_Name, level int, rarity int, label string, ok bool) {
	clean := strings.ToUpper(strings.TrimSpace(s))
	if m := reArtWrapper.FindStringSubmatch(clean); len(m) > 1 {
		clean = strings.ToUpper(strings.TrimSpace(m[1]))
	}

	level = -1
	rarity = -1

	// Normalize separators like underscores or hyphens so \b word boundaries match tier/rarity
	clean = strings.ReplaceAll(clean, "_", " ")
	clean = strings.ReplaceAll(clean, "-", " ")
	clean = strings.TrimSpace(reWhitespace.ReplaceAllString(clean, " "))

	// Check tier and rarity e.g. T4L, T4, T3R
	if m := reTierRarity.FindStringSubmatch(clean); len(m) > 0 {
		level = int(m[1][0] - '1')
		if len(m) > 2 && m[2] != "" {
			switch strings.ToUpper(m[2]) {
			case "C":
				rarity = 0
			case "R":
				rarity = 1
			case "E":
				rarity = 2
			case "L":
				rarity = 3
			}
		}
		clean = strings.TrimSpace(reTierRarity.ReplaceAllString(clean, " "))
	} else if m := reRarityWord.FindStringSubmatch(clean); len(m) > 0 {
		switch strings.ToUpper(m[1]) {
		case "COMMON":
			rarity = 0
		case "RARE":
			rarity = 1
		case "EPIC":
			rarity = 2
		case "LEGENDARY", "LEGGY":
			rarity = 3
		}
		clean = strings.TrimSpace(reRarityWord.ReplaceAllString(clean, " "))
	}

	if isCraft && level < 0 {
		level = 3 // default T4 for crafts
	}

	normKey := strings.ToUpper(strings.Trim(reWhitespace.ReplaceAllString(clean, "_"), "_"))
	if name, found := artifactAliasMap[normKey]; found {
		disp := artifactDisplayName(name)
		if isCraft {
			label = fmt.Sprintf("T%d %s Crafts", level+1, disp)
		} else {
			label = formatArtifactCountLabel(level, rarity, name)
		}
		return name, level, rarity, label, true
	}

	return 0, -1, -1, "", false
}

func isBoosterHelper(b *Booster) bool {
	if b == nil {
		return false
	}
	return b.IsAlt || b.AltController != ""
}

func isBoosterMain(b *Booster) bool {
	return !isBoosterHelper(b)
}

func (c customCriterion) matchesRole(b *Booster) bool {
	switch c.targetRole {
	case roleConditionMain:
		return isBoosterMain(b)
	case roleConditionHelper:
		return isBoosterHelper(b)
	default:
		return true
	}
}

func parseCustomCriterion(s string) customCriterion {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return customCriterion{critType: CritUnknown}
	}

	// Check if this is a conditional IF ... [ELSE ...] rule
	if m := reConditional.FindStringSubmatch(trimmed); len(m) > 3 {
		op := strings.TrimSpace(m[1])
		roleStr := strings.ToUpper(strings.Trim(strings.TrimSpace(m[2]), `"'`))
		isNot := (op == "!=" || op == "<>")

		var targetRole customRoleCondition
		switch roleStr {
		case "MAIN", "MAINS":
			if isNot {
				targetRole = roleConditionHelper
			} else {
				targetRole = roleConditionMain
			}
		case "HELPER", "HELPERS", "ALT", "ALTS":
			if isNot {
				targetRole = roleConditionMain
			} else {
				targetRole = roleConditionHelper
			}
		default:
			targetRole = roleConditionNone
		}

		if targetRole != roleConditionNone {
			thenStr := strings.TrimSpace(m[3])
			thenCrit := parseCustomCriterion(thenStr)
			crit := customCriterion{
				raw:           trimmed,
				isConditional: true,
				targetRole:    targetRole,
				thenCrit:      &thenCrit,
			}
			if len(m) > 4 && strings.TrimSpace(m[4]) != "" {
				elseStr := strings.TrimSpace(m[4])
				elseCrit := parseCustomCriterion(elseStr)
				crit.hasElse = true
				crit.elseCrit = &elseCrit
			}
			return crit
		}
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
	case strings.HasPrefix(upper, "DEFL_SLOT"):
		crit.critType = CritDeflSlot
	case upper == "DEFL" || upper == "DEFLECTOR":
		crit.critType = CritDefl
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
	case strings.HasPrefix(upper, "ROLE"):
		crit.critType = CritRole
		if strings.Contains(upper, "HELP") || strings.Contains(upper, "ALT") {
			crit.ascending = true
		}
	case upper == "MAIN" || upper == "MAINS":
		crit.critType = CritRole
		crit.ascending = false
	case upper == "HELPER" || upper == "HELPERS" || upper == "ALT" || upper == "ALTS":
		crit.critType = CritRole
		crit.ascending = true
	case upper == "CRAFT_DEFL" || upper == "DEFL_CRAFT" || upper == "CRAFTS":
		crit.critType = CritCraftDefl
	case reCraftExpr.MatchString(cur):
		m := reCraftExpr.FindStringSubmatch(cur)
		inner := m[1]
		if inner == "" {
			inner = m[2]
		}
		if inner == "" {
			inner = m[3]
		}
		if artName, level, rarity, label, ok := parseArtifactSpecTokens(inner, true); ok {
			crit.critType = CritArtifactCraft
			crit.artName = artName
			crit.artLevel = level
			crit.artRarity = rarity
			crit.artLabel = label
		} else {
			crit.critType = CritUnknown
		}
	default:
		if artName, level, rarity, label, ok := parseArtifactSpecTokens(cur, false); ok {
			crit.critType = CritArtifactCount
			crit.artName = artName
			crit.artLevel = level
			crit.artRarity = rarity
			crit.artLabel = label
		} else {
			crit.critType = CritUnknown
		}
	}

	return crit
}

func getBoosterBackup(userID string) *ei.Backup {
	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	if eiID == "" && !isDiscordSnowflake(userID) {
		if discordID, err := farmerstate.GetDiscordUserIDFromEiIgnExact(userID); err == nil && discordID != "" {
			eiID = farmerstate.GetMiscSettingString(discordID, "encrypted_ei_id")
		}
	}
	if eiID == "" {
		return nil
	}
	backup, _ := ei.GetFirstContactFromAPI(eiID, userID, true)
	return backup
}

func getBoosterArtifactCraftCount(b *Booster, name ei.ArtifactSpec_Name, level int) int {
	if b == nil {
		return 0
	}
	overrideKey := fmt.Sprintf("crafts_%d_%d", name, level)
	if s := farmerstate.GetMiscSettingString(b.UserID, overrideKey); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			return v
		}
	}
	overrideKeyNamed := fmt.Sprintf("crafts_%s_%d", name.String(), level)
	if s := farmerstate.GetMiscSettingString(b.UserID, overrideKeyNamed); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			return v
		}
	}
	if name == ei.ArtifactSpec_TACHYON_DEFLECTOR && (level < 0 || level == int(ei.ArtifactSpec_GREATER)) {
		if s := farmerstate.GetMiscSettingString(b.UserID, "t4_crafts"); s != "" {
			if v, err := strconv.Atoi(s); err == nil {
				return v
			}
		}
	}

	backup := getBoosterBackup(b.UserID)
	if backup != nil && backup.GetArtifactsDb() != nil {
		total := 0
		for _, a := range backup.GetArtifactsDb().GetArtifactStatus() {
			if a != nil && a.Spec != nil && a.Spec.GetName() == name {
				if level < 0 || int(a.Spec.GetLevel()) == level {
					total += int(a.GetCount())
				}
			}
		}
		return total
	}
	return 0
}

func getBoosterArtifactCount(b *Booster, name ei.ArtifactSpec_Name, level int, rarity int) int {
	if b == nil {
		return 0
	}
	overrideKey := fmt.Sprintf("art_count_%d_%d_%d", name, level, rarity)
	if s := farmerstate.GetMiscSettingString(b.UserID, overrideKey); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			return v
		}
	}
	overrideKeyNamed := fmt.Sprintf("art_count_%s_%d_%d", name.String(), level, rarity)
	if s := farmerstate.GetMiscSettingString(b.UserID, overrideKeyNamed); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			return v
		}
	}

	backup := getBoosterBackup(b.UserID)
	if backup != nil && backup.GetArtifactsDb() != nil {
		db := backup.GetArtifactsDb()
		total := 0
		countItems := func(items []*ei.ArtifactInventoryItem) {
			for _, item := range items {
				art := item.GetArtifact()
				if art == nil {
					continue
				}
				spec := art.GetSpec()
				if spec == nil {
					continue
				}
				if spec.GetName() != name {
					continue
				}
				if level >= 0 && int(spec.GetLevel()) != level {
					continue
				}
				if rarity >= 0 && int(spec.GetRarity()) != rarity {
					continue
				}
				qty := int(item.GetQuantity())
				if qty == 0 {
					qty = 1
				}
				total += qty
			}
		}
		countItems(db.GetInventoryItems())
		if virtueDB := db.GetVirtueAfxDb(); virtueDB != nil {
			countItems(virtueDB.GetInventoryItems())
		}
		if total > 0 {
			return total
		}
	}

	// Fallback to equipped artifacts if backup is unavailable or didn't report
	total := 0
	artDisp := artifactDisplayName(name)
	for _, a := range b.ArtifactSet.Artifacts {
		if strings.EqualFold(a.Type, artDisp) || strings.EqualFold(a.Type, name.String()) {
			expQuality := formatArtifactTierRarity(level, rarity)
			if expQuality == "" || strings.EqualFold(a.Quality, expQuality) {
				total++
			}
		}
	}
	if name == ei.ArtifactSpec_TACHYON_DEFLECTOR && (level < 0 || level == int(ei.ArtifactSpec_GREATER)) && (rarity < 0 || rarity == int(ei.ArtifactSpec_LEGENDARY)) {
		if total == 0 && hasBoosterT4LDeflector(b) {
			total = 1
		}
	}
	return total
}

// getBoosterT4DeflectorCraftCount retrieves how many T4 Deflectors the booster has crafted.
func getBoosterT4DeflectorCraftCount(userID string) int {
	return getBoosterArtifactCraftCount(&Booster{UserID: userID}, ei.ArtifactSpec_TACHYON_DEFLECTOR, int(ei.ArtifactSpec_GREATER))
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
	return strings.Contains(qualityIHR, "T4L")
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
	isMain        bool
	isHelper      bool
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
	roleScore     float64
	randomScore   float64
	rowScores     [4]float64
}

func applyFuzzyModifiers(b *Booster, data *boosterEvalData, crit customCriterion, applyFuzzy bool) {
	if !applyFuzzy || b == nil {
		return
	}
	if crit.isConditional {
		if crit.thenCrit != nil {
			applyFuzzyModifiers(b, data, *crit.thenCrit, applyFuzzy)
		}
		if crit.elseCrit != nil {
			applyFuzzyModifiers(b, data, *crit.elseCrit, applyFuzzy)
		}
		return
	}
	if crit.critType == CritIHR && crit.fuzzyPct > 0 {
		bonusMax := data.ihrBase * crit.fuzzyPct
		offset := (rand.Float64()*2 - 1) * bonusMax
		data.ihrSort = data.ihrBase + offset
		b.FuzzyOffset = offset
	} else if crit.critType == CritTE {
		if crit.fuzzySqrt {
			baseTE := float64(max(data.teBase, 0))
			bonusMax := math.Max(baseTE*0.06, math.Sqrt(baseTE+25))
			offset := (rand.Float64()*2 - 1) * bonusMax
			data.teSort = baseTE + offset
			b.FuzzyOffset = offset
		} else if crit.fuzzyPct > 0 {
			bonusMax := float64(data.teBase) * crit.fuzzyPct
			offset := (rand.Float64()*2 - 1) * bonusMax
			data.teSort = float64(data.teBase) + offset
			b.FuzzyOffset = offset
		}
	}
}

func evaluateBoosterForCustom(contract *Contract, userID string, signupIdx int, criteria [4]customCriterion, applyFuzzy bool) boosterEvalData {
	b := contract.Boosters[userID]
	data := boosterEvalData{
		userID:      userID,
		signupIndex: signupIdx,
		randomScore: rand.Float64(),
	}

	if b != nil {
		data.isHelper = isBoosterHelper(b)
		data.isMain = !data.isHelper
		if data.isMain {
			data.roleScore = 2.0
		} else {
			data.roleScore = 1.0
		}
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
		applyFuzzyModifiers(b, &data, crit, applyFuzzy)
		data.rowScores[rowIdx] = getBoosterCriterionValue(contract, &data, crit)
	}

	return data
}

func getBoosterCriterionValue(contract *Contract, item *boosterEvalData, crit customCriterion) float64 {
	switch crit.critType {
	case CritDeflEffort:
		b := contract.Boosters[item.userID]
		effScore, _, _ := calculateBoosterDeflectorEffort(b, crit.effortN)
		return float64(effScore)
	case CritCraftDefl:
		return float64(item.t4Crafts)
	case CritIHR:
		return item.ihrSort
	case CritELR:
		return item.elr
	case CritTE:
		return item.teSort
	case CritTokens:
		return float64(item.tokensWanted)
	case CritTVal:
		_, _, tvalByUser, _ := buildTokenTotalsFromLog(contract)
		return tvalByUser[item.userID]
	case CritDefl:
		return float64(item.deflScore)
	case CritDeflSlot:
		return float64(item.deflSlotScore)
	case CritDeliv:
		return float64(item.delivScore)
	case CritSignup:
		return float64(item.signupIndex)
	case CritReverse:
		return -float64(item.signupIndex)
	case CritRandom:
		return item.randomScore
	case CritRole:
		return item.roleScore
	case CritArtifactCount:
		b := contract.Boosters[item.userID]
		return float64(getBoosterArtifactCount(b, crit.artName, crit.artLevel, crit.artRarity))
	case CritArtifactCraft:
		b := contract.Boosters[item.userID]
		return float64(getBoosterArtifactCraftCount(b, crit.artName, crit.artLevel))
	default:
		return 0.0
	}
}

func compareBoosterCriterion(contract *Contract, itemI, itemJ *boosterEvalData, crit customCriterion) int {
	if crit.isConditional {
		bI := contract.Boosters[itemI.userID]
		bJ := contract.Boosters[itemJ.userID]

		matchI := crit.matchesRole(bI)
		matchJ := crit.matchesRole(bJ)

		if matchI && matchJ {
			if crit.thenCrit != nil {
				return compareBoosterCriterion(contract, itemI, itemJ, *crit.thenCrit)
			}
			return 0
		}
		if !matchI && !matchJ {
			if crit.hasElse && crit.elseCrit != nil {
				return compareBoosterCriterion(contract, itemI, itemJ, *crit.elseCrit)
			}
			// If ELSE is missing then that sort doesn't apply: both tie on this level
			return 0
		}
		if matchI && !matchJ {
			return -1
		}
		return 1
	}

	if crit.critType == CritUnknown {
		return 0
	}

	valI := getBoosterCriterionValue(contract, itemI, crit)
	valJ := getBoosterCriterionValue(contract, itemJ, crit)

	if math.Abs(valI-valJ) > 1e-6 {
		if crit.ascending {
			if valI < valJ {
				return -1
			}
			return 1
		}
		if valI > valJ {
			return -1
		}
		return 1
	}
	return 0
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
			if !criteria[r].isConditional && criteria[r].critType == CritUnknown {
				continue
			}
			cmp := compareBoosterCriterion(contract, &items[i], &items[j], criteria[r])
			if cmp < 0 {
				return true
			} else if cmp > 0 {
				return false
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

type customTableColDef struct {
	id       string
	col      TableImageColumn
	evalCell func(b *Booster, signupIdx int) TableImageCell
}

func buildCustomOrderTableColDefs(contract *Contract, criteria [4]customCriterion) ([]customTableColDef, []TableImageColumn) {
	var activeCols []customTableColDef
	seenColIDs := make(map[string]bool)
	var tvalByUser map[string]float64

	var addCritCol func(c customCriterion)
	addCritCol = func(c customCriterion) {
		if c.isConditional {
			if !seenColIDs["role"] {
				seenColIDs["role"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "role",
					col: TableImageColumn{Label: "Role", Align: bottools.StringAlignCenter},
					evalCell: func(b *Booster, _ int) TableImageCell {
						if isBoosterHelper(b) {
							return TableImageCell{Text: "Helper", Color: "red"}
						}
						return TableImageCell{Text: "Main", Color: "green"}
					},
				})
			}
			if c.thenCrit != nil {
				addCritCol(*c.thenCrit)
			}
			if c.elseCrit != nil {
				addCritCol(*c.elseCrit)
			}
			return
		}

		switch c.critType {
		case CritRole:
			if !seenColIDs["role"] {
				seenColIDs["role"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "role",
					col: TableImageColumn{Label: "Role", Align: bottools.StringAlignCenter},
					evalCell: func(b *Booster, _ int) TableImageCell {
						if isBoosterHelper(b) {
							return TableImageCell{Text: "Helper", Color: "red"}
						}
						return TableImageCell{Text: "Main", Color: "green"}
					},
				})
			}
		case CritDeflEffort:
			id := fmt.Sprintf("defl_effort_%d", c.effortN)
			if !seenColIDs[id] {
				seenColIDs[id] = true
				effortN := c.effortN
				activeCols = append(activeCols, customTableColDef{
					id:  id,
					col: TableImageColumn{Label: fmt.Sprintf("Effort [N=%d]", effortN), Align: bottools.StringAlignCenter},
					evalCell: func(b *Booster, _ int) TableImageCell {
						effScore, craftCount, hasT4L := calculateBoosterDeflectorEffort(b, effortN)
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
						return TableImageCell{Text: effortDisplay, Color: effortColor}
					},
				})
			}
		case CritCraftDefl:
			if !seenColIDs["craft_defl"] {
				seenColIDs["craft_defl"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "craft_defl",
					col: TableImageColumn{Label: "T4 Crafts", Align: bottools.StringAlignRight},
					evalCell: func(b *Booster, _ int) TableImageCell {
						craftCount := getBoosterT4DeflectorCraftCount(b.UserID)
						craftsDisplay := fmt.Sprintf("%d", craftCount)
						craftsColor := ""
						if craftCount >= 50 {
							craftsColor = "green"
						}
						return TableImageCell{Text: craftsDisplay, Color: craftsColor}
					},
				})
			}
		case CritDefl:
			if !seenColIDs["defl"] {
				seenColIDs["defl"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "defl",
					col: TableImageColumn{Label: "Deflector", Align: bottools.StringAlignCenter},
					evalCell: func(b *Booster, _ int) TableImageCell {
						quality := getBoosterDeflectorQualityString(b)
						deflColor := ""
						if hasBoosterT4LDeflector(b) {
							deflColor = "green"
						} else if strings.HasPrefix(quality, "T4") {
							deflColor = "blue"
						}
						return TableImageCell{Text: quality, Color: deflColor}
					},
				})
			}
		case CritDeflSlot:
			if !seenColIDs["defl_slot"] {
				seenColIDs["defl_slot"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "defl_slot",
					col: TableImageColumn{Label: "Defl Slot", Align: bottools.StringAlignCenter},
					evalCell: func(b *Booster, _ int) TableImageCell {
						quality := getBoosterDeflectorQualityString(b)
						slots := getESCDeflectorScore(b, false)
						text := fmt.Sprintf("%s (%d)", quality, slots)
						if quality == "-" {
							text = "-"
						}
						return TableImageCell{Text: text, Color: ""}
					},
				})
			}
		case CritIHR:
			if !seenColIDs["ihr"] {
				seenColIDs["ihr"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "ihr",
					col: TableImageColumn{Label: "IHR", Align: bottools.StringAlignRight},
					evalCell: func(b *Booster, _ int) TableImageCell {
						ihrMult := fmt.Sprintf("%0.2fx", b.IHRRate/DefaultLeggyIHR)
						return TableImageCell{Text: ihrMult, Color: ""}
					},
				})
			}
		case CritELR:
			if !seenColIDs["elr"] {
				seenColIDs["elr"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "elr",
					col: TableImageColumn{Label: "ELR", Align: bottools.StringAlignRight},
					evalCell: func(b *Booster, _ int) TableImageCell {
						elrStr := fmt.Sprintf("%0.2f", b.ArtifactSet.LayRate)
						return TableImageCell{Text: elrStr, Color: ""}
					},
				})
			}
		case CritTokens:
			if !seenColIDs["tokens"] {
				seenColIDs["tokens"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "tokens",
					col: TableImageColumn{Label: "Tokens", Align: bottools.StringAlignRight},
					evalCell: func(b *Booster, _ int) TableImageCell {
						tokensStr := fmt.Sprintf("%d", b.TokensWanted)
						return TableImageCell{Text: tokensStr, Color: ""}
					},
				})
			}
		case CritTE:
			if !seenColIDs["te"] {
				seenColIDs["te"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "te",
					col: TableImageColumn{Label: "TE", Align: bottools.StringAlignRight},
					evalCell: func(b *Booster, _ int) TableImageCell {
						teStr := fmt.Sprintf("%d", b.TECount)
						return TableImageCell{Text: teStr, Color: ""}
					},
				})
			}
		case CritTVal:
			if !seenColIDs["tval"] {
				seenColIDs["tval"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "tval",
					col: TableImageColumn{Label: "TVal", Align: bottools.StringAlignRight},
					evalCell: func(b *Booster, _ int) TableImageCell {
						val := 0.0
						if tvalByUser != nil {
							val = tvalByUser[b.UserID]
						}
						return TableImageCell{Text: fmt.Sprintf("%0.1f", val), Color: ""}
					},
				})
			}
		case CritDeliv:
			if !seenColIDs["deliv"] {
				seenColIDs["deliv"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "deliv",
					col: TableImageColumn{Label: "Delivery", Align: bottools.StringAlignRight},
					evalCell: func(b *Booster, _ int) TableImageCell {
						delivScore := getArtifactQualityScore(b, "Metronome") + getArtifactQualityScore(b, "Compass") + getArtifactQualityScore(b, "Gusset")
						return TableImageCell{Text: fmt.Sprintf("%d", delivScore), Color: ""}
					},
				})
			}
		case CritSignup:
			if !seenColIDs["signup"] {
				seenColIDs["signup"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "signup",
					col: TableImageColumn{Label: "Signup", Align: bottools.StringAlignRight},
					evalCell: func(_ *Booster, signupIdx int) TableImageCell {
						return TableImageCell{Text: fmt.Sprintf("#%d", signupIdx+1), Color: ""}
					},
				})
			}
		case CritReverse:
			if !seenColIDs["reverse"] {
				seenColIDs["reverse"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "reverse",
					col: TableImageColumn{Label: "Reverse", Align: bottools.StringAlignRight},
					evalCell: func(_ *Booster, signupIdx int) TableImageCell {
						return TableImageCell{Text: fmt.Sprintf("#%d", signupIdx+1), Color: ""}
					},
				})
			}
		case CritRandom:
			if !seenColIDs["random"] {
				seenColIDs["random"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "random",
					col: TableImageColumn{Label: "Random", Align: bottools.StringAlignCenter},
					evalCell: func(_ *Booster, _ int) TableImageCell {
						return TableImageCell{Text: "🎲", Color: ""}
					},
				})
			}
		case CritArtifactCount:
			id := fmt.Sprintf("art_count_%d_%d_%d", c.artName, c.artLevel, c.artRarity)
			if !seenColIDs[id] {
				seenColIDs[id] = true
				name := c.artName
				level := c.artLevel
				rarity := c.artRarity
				activeCols = append(activeCols, customTableColDef{
					id:  id,
					col: TableImageColumn{Label: c.artLabel, Align: bottools.StringAlignRight},
					evalCell: func(b *Booster, _ int) TableImageCell {
						cnt := getBoosterArtifactCount(b, name, level, rarity)
						color := ""
						if cnt > 0 {
							color = "green"
						}
						return TableImageCell{Text: fmt.Sprintf("%d", cnt), Color: color}
					},
				})
			}
		case CritArtifactCraft:
			id := fmt.Sprintf("art_craft_%d_%d", c.artName, c.artLevel)
			if !seenColIDs[id] {
				seenColIDs[id] = true
				name := c.artName
				level := c.artLevel
				activeCols = append(activeCols, customTableColDef{
					id:  id,
					col: TableImageColumn{Label: c.artLabel, Align: bottools.StringAlignRight},
					evalCell: func(b *Booster, _ int) TableImageCell {
						cnt := getBoosterArtifactCraftCount(b, name, level)
						color := ""
						if cnt > 0 {
							color = "green"
						}
						return TableImageCell{Text: fmt.Sprintf("%d", cnt), Color: color}
					},
				})
			}
		}
	}

	for _, c := range criteria {
		addCritCol(c)
	}

	if seenColIDs["tval"] && contract != nil {
		_, _, tvalByUser, _ = buildTokenTotalsFromLog(contract)
	}

	cols := []TableImageColumn{
		{Label: "#", Align: bottools.StringAlignRight},
		{Label: "Player", Align: bottools.StringAlignLeft},
	}
	for _, ac := range activeCols {
		cols = append(cols, ac.col)
	}

	return activeCols, cols
}

// getCustomOrderTableColumns returns the columns that would be displayed for the given custom boost order lines.
func getCustomOrderTableColumns(contract *Contract, lines []string) []TableImageColumn {
	var criteria [4]customCriterion
	for i := 0; i < 4; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		criteria[i] = parseCustomCriterion(line)
	}
	_, cols := buildCustomOrderTableColDefs(contract, criteria)
	return cols
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

	activeCols, cols := buildCustomOrderTableColDefs(contract, criteria)

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

		signupIdx := 0
		for i, id := range contract.Order {
			if id == userID {
				signupIdx = i
				break
			}
		}

		cells := []TableImageCell{
			{Text: fmt.Sprintf("%d", idx+1), Color: ""},
			{Text: name, Color: ""},
		}

		for _, ac := range activeCols {
			cells = append(cells, ac.evalCell(b, signupIdx))
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
