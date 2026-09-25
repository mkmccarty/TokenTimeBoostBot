package boost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
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

func saveCustomOrderSessionDB(s *customOrderSession) {
	if dbConn == nil || s == nil {
		return
	}
	linesJSON, _ := json.Marshal(s.lines)
	_, _ = dbConn.ExecContext(ctx,
		`INSERT INTO custom_order_sessions (uuid, contract_hash, channel_id, user_id, lines, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(uuid) DO UPDATE SET
		   contract_hash = excluded.contract_hash,
		   channel_id = excluded.channel_id,
		   user_id = excluded.user_id,
		   lines = excluded.lines,
		   expires_at = excluded.expires_at`,
		s.uuidStr, s.contractHash, s.channelID, s.userID, string(linesJSON), s.expiresAt.Unix(),
	)
}

func loadCustomOrderSessionDB(uuidStr string) *customOrderSession {
	if dbConn == nil {
		return nil
	}
	var contractHash, channelID, userID, linesJSON string
	var expiresAtSec int64
	err := dbConn.QueryRowContext(ctx,
		`SELECT contract_hash, channel_id, user_id, lines, expires_at FROM custom_order_sessions WHERE uuid = ?`,
		uuidStr,
	).Scan(&contractHash, &channelID, &userID, &linesJSON, &expiresAtSec)
	if err != nil {
		return nil
	}
	expiresAt := time.Unix(expiresAtSec, 0)
	if expiresAt.Before(time.Now()) {
		_, _ = dbConn.ExecContext(ctx, `DELETE FROM custom_order_sessions WHERE uuid = ?`, uuidStr)
		return nil
	}
	var lines []string
	if err := json.Unmarshal([]byte(linesJSON), &lines); err != nil {
		lines = strings.Split(linesJSON, "\n")
	}
	return &customOrderSession{
		uuidStr:      uuidStr,
		contractHash: contractHash,
		channelID:    channelID,
		userID:       userID,
		lines:        lines,
		expiresAt:    expiresAt,
	}
}

func deleteCustomOrderSessionDB(uuidStr string) {
	if dbConn == nil {
		return
	}
	_, _ = dbConn.ExecContext(ctx, `DELETE FROM custom_order_sessions WHERE uuid = ?`, uuidStr)
}

func cleanupExpiredCustomOrderSessionsDB() {
	if dbConn == nil {
		return
	}
	_, _ = dbConn.ExecContext(ctx, `DELETE FROM custom_order_sessions WHERE expires_at < ?`, time.Now().Unix())
}

func findCustomOrderSession(userID string, contractHash string) *customOrderSession {
	customOrderSessionsMutex.Lock()
	defer customOrderSessionsMutex.Unlock()

	now := time.Now()
	for _, s := range customOrderSessions {
		if s.userID == userID && s.contractHash == contractHash && !s.expiresAt.Before(now) {
			return s
		}
	}

	if dbConn != nil {
		var uuidStr, channelID, linesJSON string
		var expiresAtSec int64
		err := dbConn.QueryRowContext(ctx,
			`SELECT uuid, channel_id, lines, expires_at FROM custom_order_sessions
			 WHERE user_id = ? AND contract_hash = ? AND expires_at > ?
			 ORDER BY expires_at DESC LIMIT 1`,
			userID, contractHash, now.Unix(),
		).Scan(&uuidStr, &channelID, &linesJSON, &expiresAtSec)
		if err == nil {
			var lines []string
			if err := json.Unmarshal([]byte(linesJSON), &lines); err != nil {
				lines = strings.Split(linesJSON, "\n")
			}
			s := &customOrderSession{
				uuidStr:      uuidStr,
				contractHash: contractHash,
				channelID:    channelID,
				userID:       userID,
				lines:        lines,
				expiresAt:    time.Unix(expiresAtSec, 0),
			}
			customOrderSessions[s.uuidStr] = s
			return s
		}
	}
	return nil
}

func getOrCreateCustomOrderSession(userID string, contractHash string, lines []string) *customOrderSession {
	customOrderSessionsMutex.Lock()
	defer customOrderSessionsMutex.Unlock()

	now := time.Now()
	for k, s := range customOrderSessions {
		if s.expiresAt.Before(now) {
			delete(customOrderSessions, k)
		}
	}
	cleanupExpiredCustomOrderSessionsDB()

	for _, s := range customOrderSessions {
		if s.userID == userID && s.contractHash == contractHash {
			s.expiresAt = now.Add(customOrderSessionTTL)
			if len(lines) > 0 {
				s.lines = append([]string(nil), lines...)
			}
			saveCustomOrderSessionDB(s)
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
	saveCustomOrderSessionDB(session)
	return session
}

func getCustomOrderSession(uuidStr string) *customOrderSession {
	customOrderSessionsMutex.Lock()
	defer customOrderSessionsMutex.Unlock()

	if s, ok := customOrderSessions[uuidStr]; ok {
		if s.expiresAt.Before(time.Now()) {
			delete(customOrderSessions, uuidStr)
			deleteCustomOrderSessionDB(uuidStr)
			return nil
		}
		return s
	}

	if s := loadCustomOrderSessionDB(uuidStr); s != nil {
		customOrderSessions[s.uuidStr] = s
		return s
	}

	return nil
}

func clearCustomOrderSession(uuidStr string) {
	customOrderSessionsMutex.Lock()
	defer customOrderSessionsMutex.Unlock()
	delete(customOrderSessions, uuidStr)
	deleteCustomOrderSessionDB(uuidStr)
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
	CritArtifactHas
	CritArtifactScore
	CritCraftingXP
	CritBoostCount
	CritGE
	CritEB
	CritSE
	CritCTE
	CritPrestige
	CritDrone
	CritEliteDrone
	CritManual
	CritFair
	CritTime
	CritUnknown
)

type customRoleCondition int

const (
	roleConditionNone customRoleCondition = iota
	roleConditionMain
	roleConditionHelper
)

const (
	RolePriorityMain   = 1
	RolePriorityHelper = 2
)

type customCriterion struct {
	raw           string
	critType      CustomCriterionType
	ascending     bool // true = ascending (>), false = descending (<)
	effortN       int  // equivalence craft count for DEFL_EFFORT (default 50)
	fuzzyPct      float64
	fuzzySqrt     bool
	fuzzyStr      string
	isConditional bool
	targetRole    customRoleCondition
	thenCrit      *customCriterion
	hasElse       bool
	elseCrit      *customCriterion

	artName    ei.ArtifactSpec_Name
	artLevel   int // 0..3 for T1..T4, -1 for any
	artRarity  int // 0..3 for C..L, -1 for any
	artLabel   string
	boostID    string
	boostLabel string
}

var (
	reDeflEffort  = regexp.MustCompile(`(?i)^(?:(?:DEFL|DEFLECTOR)_)?EFFORT(?:_?\[\s*(\d+)\s*\])?$|(?i)^DEFL_EFFORT(?:_?\[\s*(\d+)\s*\])?$`)
	reFuzzyPct    = regexp.MustCompile(`\[\s*(\d+(?:\.\d+)?)\s*%\s*\]`)
	reFuzzySqrt   = regexp.MustCompile(`(?i)\[\s*(?:sqrt|~|√)\s*\]`)
	reConditional = regexp.MustCompile(`(?i)^\s*IF\s+(?:(NOT)\s+)?(?:\(?\s*ROLE\s*(==|=|!=|<>|IS\s+NOT|IS|NOT)?\s*)?([A-Za-z"']+)\)?(?:\s*(?:THEN|:))?\s+(.+?)(?:\s+ELSE(?::)?\s+(.+))?$`)
	reCraftExpr   = regexp.MustCompile(`(?i)^(?:CRAFTS?[\(\[]\s*([A-Za-z0-9_ -]+)\s*[\)\]]|CRAFTS?[:_ ]\s*([A-Za-z0-9_ -]+)|([A-Za-z0-9_ -]+)_CRAFTS?)$`)
	reCountExpr   = regexp.MustCompile(`(?i)^(?:COUNTS?|QTY|QUANTITY)(?:[\(\[]\s*([A-Za-z0-9_ -]+)\s*[\)\]]|[:_ ]\s*([A-Za-z0-9_ -]+))$`)
	reHasExpr     = regexp.MustCompile(`(?i)^(?:HAS|OWN|OWNS)(?:[\(\[]\s*([A-Za-z0-9_ -]+)\s*[\)\]]|[:_ ]\s*([A-Za-z0-9_ -]+))$`)
	reBoostExpr   = regexp.MustCompile(`(?i)^(?:BOOSTS?|BOOST_COUNT)(?:[\(\[]\s*([A-Za-z0-9_ -]+)\s*[\)\]]|[:_ ]\s*([A-Za-z0-9_ -]+))$`)
	reArtWrapper  = regexp.MustCompile(`(?i)^(?:ARTIFACT|ART)[\(\[]\s*([A-Za-z0-9_ -]+)\s*[\)\]]$`)
	reTierRarity  = regexp.MustCompile(`(?i)\bT([1-4])([CREL])?\b`)
	reRarityWord  = regexp.MustCompile(`(?i)\b(LEGENDARY|LEGGY|EPIC|RARE|COMMON)\b`)
	reWhitespace  = regexp.MustCompile(`[\s\-]+`)
)

var knownRawBoostIDs = map[string]string{
	"soul_mirror_blue":         "Soul Mirror (10m)",
	"soul_mirror_purple":       "Soul Mirror (1h)",
	"soul_mirror_orange":       "Soul Mirror (1d)",
	"tachyon_prism_blue":       "Tachyon Prism",
	"tachyon_prism_blue_big":   "Large Tachyon Prism",
	"tachyon_prism_purple":     "Powerful Tachyon Prism",
	"tachyon_prism_purple_big": "Epic Tachyon Prism",
	"tachyon_prism_purple_v2":  "Tachyon Prism",
	"tachyon_prism_orange":     "Legendary Tachyon Prism",
	"tachyon_prism_orange_big": "Supreme Tachyon Prism",
	"boost_beacon_blue":        "Boost Beacon",
	"boost_beacon_blue_big":    "Large Boost Beacon",
	"boost_beacon_purple":      "Epic Boost Beacon",
	"boost_beacon_orange":      "Legendary Boost Beacon",
	"soul_beacon_blue":         "Soul Beacon",
	"soul_beacon_purple":       "Epic Soul Beacon",
	"soul_beacon_orange":       "Legendary Soul Beacon",
	"jimbos_blue":              "Jimbo's Best Bird Feed (20m)",
	"jimbos_blue_big":          "Jimbo's Best Bird Feed (2h)",
	"jimbos_purple":            "Jimbo's Best Bird Feed (2h 10x)",
	"jimbos_purple_big":        "Jimbo's Best Bird Feed (8h 10x)",
	"jimbos_orange":            "Jimbo's Best Bird Feed (10m 50x)",
	"jimbos_orange_big":        "Jimbo's Best Bird Feed (1h 50x)",
	"dilithium_bulb":           "Quantum Warming Bulb",
	"money_printer":            "Money Printer",
	"blank_check":              "Blank Check",
}

var boostFriendlyToRawID = map[string]string{
	"soul_mirror":             "soul_mirror_blue",
	"soul_mirror_10m":         "soul_mirror_blue",
	"soul_mirror_1h":          "soul_mirror_purple",
	"soul_mirror_1d":          "soul_mirror_orange",
	"epic_soul_mirror":        "soul_mirror_purple",
	"legendary_soul_mirror":   "soul_mirror_orange",
	"tachyon_prism":           "tachyon_prism_purple_v2",
	"tachyon_prism_10m_10x":   "tachyon_prism_blue",
	"large_tachyon_prism":     "tachyon_prism_blue_big",
	"powerful_tachyon_prism":  "tachyon_prism_purple",
	"epic_tachyon_prism":      "tachyon_prism_purple_big",
	"legendary_tachyon_prism": "tachyon_prism_orange",
	"supreme_tachyon_prism":   "tachyon_prism_orange_big",
	"boost_beacon":            "boost_beacon_blue",
	"large_boost_beacon":      "boost_beacon_blue_big",
	"epic_boost_beacon":       "boost_beacon_purple",
	"legendary_boost_beacon":  "boost_beacon_orange",
	"soul_beacon":             "soul_beacon_blue",
	"epic_soul_beacon":        "soul_beacon_purple",
	"legendary_soul_beacon":   "soul_beacon_orange",
	"quantum_warming_bulb":    "dilithium_bulb",
	"warming_bulb":            "dilithium_bulb",
}

func isKnownBoostID(s string) bool {
	rawID := strings.ToLower(strings.Trim(reWhitespace.ReplaceAllString(s, "_"), "_"))
	if _, ok := knownRawBoostIDs[rawID]; ok {
		return true
	}
	if _, ok := boostFriendlyToRawID[rawID]; ok {
		return true
	}
	return strings.HasPrefix(rawID, "soul_mirror_") ||
		strings.HasPrefix(rawID, "tachyon_prism_") ||
		strings.HasPrefix(rawID, "boost_beacon_") ||
		strings.HasPrefix(rawID, "soul_beacon_") ||
		strings.HasPrefix(rawID, "jimbo_") ||
		strings.HasPrefix(rawID, "jimbos_")
}

func resolveBoostTarget(raw string) (string, string) {
	rawID := strings.ToLower(strings.Trim(reWhitespace.ReplaceAllString(raw, "_"), "_"))
	if targetID, ok := boostFriendlyToRawID[rawID]; ok {
		rawID = targetID
	}
	if label, found := knownRawBoostIDs[rawID]; found {
		return rawID, label
	}

	labelWords := strings.Split(rawID, "_")
	for i, w := range labelWords {
		if len(w) > 0 {
			labelWords[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	label := strings.Join(labelWords, " ")
	return rawID, label
}

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
	"QUANT":                ei.ArtifactSpec_INTERSTELLAR_COMPASS,
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
	} else if m := reHasExpr.FindStringSubmatch(clean); len(m) > 1 {
		clean = strings.ToUpper(strings.TrimSpace(m[1]))
		if clean == "" && len(m) > 2 {
			clean = strings.ToUpper(strings.TrimSpace(m[2]))
		}
	} else if m := reCountExpr.FindStringSubmatch(clean); len(m) > 1 {
		clean = strings.ToUpper(strings.TrimSpace(m[1]))
		if clean == "" && len(m) > 2 {
			clean = strings.ToUpper(strings.TrimSpace(m[2]))
		}
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

func getBoosterDeflectorSlotScore(b *Booster) int {
	if b == nil {
		return 0
	}
	quality := ""
	for _, a := range b.ArtifactSet.Artifacts {
		if a.Type == "Deflector" || a.Type == "IHR Deflector" {
			quality = a.Quality
			break
		}
	}
	if quality == "" {
		quality = farmerstate.GetMiscSettingString(b.UserID, "defl")
	}
	if quality == "" {
		quality = farmerstate.GetMiscSettingString(b.UserID, "defl-ihr")
	}

	quality = strings.ToUpper(strings.TrimSpace(quality))
	quality = strings.TrimSuffix(quality, "_L")

	// 2-slot (T4L, T4E) > 1-slot (T4R) > T3R > 0-slot / Other
	switch quality {
	case "T4L", "T4E":
		return 3
	case "T4R":
		return 2
	case "T3R":
		return 1
	default:
		return 0
	}
}

func getBoosterRolePriority(b *Booster) int {
	if isBoosterHelper(b) {
		return RolePriorityHelper
	}
	return RolePriorityMain
}

func getBoosterRoleString(b *Booster) (string, string) {
	if isBoosterHelper(b) {
		return "Helper", "red"
	}
	return "Main", "green"
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
	if m := reConditional.FindStringSubmatch(trimmed); len(m) > 4 {
		notPrefix := strings.TrimSpace(m[1])
		op := strings.TrimSpace(m[2])
		roleStr := strings.ToUpper(strings.Trim(strings.TrimSpace(m[3]), `"'`))
		isNot := (notPrefix != "" || op == "!=" || op == "<>" || strings.Contains(strings.ToUpper(op), "NOT"))

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
			thenStr := strings.TrimSpace(m[4])
			thenCrit := parseCustomCriterion(thenStr)
			crit := customCriterion{
				raw:           trimmed,
				isConditional: true,
				targetRole:    targetRole,
				thenCrit:      &thenCrit,
			}
			if len(m) > 5 && strings.TrimSpace(m[5]) != "" {
				elseStr := strings.TrimSpace(m[5])
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
		crit.fuzzyStr = fmt.Sprintf("[%s%%]", m[1])
		cur = strings.TrimSpace(reFuzzyPct.ReplaceAllString(cur, ""))
	}
	if reFuzzySqrt.MatchString(cur) {
		crit.fuzzySqrt = true
		crit.fuzzyStr = "[√]"
		cur = strings.TrimSpace(reFuzzySqrt.ReplaceAllString(cur, ""))
	}

	upper := strings.ToUpper(cur)
	norm := strings.Trim(reWhitespace.ReplaceAllString(upper, "_"), "_")

	switch {
	case reDeflEffort.MatchString(norm):
		crit.critType = CritDeflEffort
		m := reDeflEffort.FindStringSubmatch(norm)
		digitStr := ""
		for idx := 1; idx < len(m); idx++ {
			if m[idx] != "" {
				digitStr = m[idx]
				break
			}
		}
		if digitStr != "" {
			if n, err := strconv.Atoi(digitStr); err == nil && n > 0 {
				crit.effortN = n
			}
		}
	case strings.HasPrefix(norm, "DEFL_SLOT") || strings.HasPrefix(norm, "DEFLECTOR_SLOT") || norm == "SLOT" || norm == "SLOTS":
		crit.critType = CritDeflSlot
	case norm == "DEFL" || norm == "DEFLECTOR" || norm == "DEFLECTORS":
		crit.critType = CritDefl
	case strings.HasPrefix(norm, "IHR") || norm == "INTERNAL_HATCHERY" || norm == "INTERNAL_HATCHERY_RATE":
		crit.critType = CritIHR
	case strings.HasPrefix(norm, "ELR") || norm == "EGG_LAYING_RATE" || norm == "LAYING_RATE":
		crit.critType = CritELR
	case strings.HasPrefix(norm, "TE") || norm == "TRUTH_EGGS" || norm == "TRUTH_EGG" || norm == "EOFT":
		crit.critType = CritTE
	case strings.HasPrefix(norm, "TOKEN") || strings.HasPrefix(norm, "TASK"):
		crit.critType = CritTokens
		// For tokens wanted, ascending order is naturally preferred (fewer tokens boost earlier)
		// unless explicitly prefixed with <
		if !strings.HasPrefix(trimmed, "<") && !strings.HasPrefix(trimmed, "-") {
			crit.ascending = true
		}
	case strings.HasPrefix(norm, "TVAL") || strings.HasPrefix(norm, "TOKEN_VALUE") || strings.HasPrefix(norm, "TOKENVALUE"):
		crit.critType = CritTVal
	case strings.HasPrefix(norm, "DELIV") || strings.HasPrefix(norm, "DEL") || strings.HasPrefix(norm, "SHIPPING") || strings.HasPrefix(norm, "SHIP"):
		crit.critType = CritDeliv
	case strings.HasPrefix(norm, "SIGNUP") || strings.HasPrefix(norm, "JOIN") || strings.HasPrefix(norm, "ORDER"):
		crit.critType = CritSignup
		if !strings.HasPrefix(trimmed, "<") && !strings.HasPrefix(trimmed, "-") {
			crit.ascending = true
		}
	case strings.HasPrefix(norm, "REVERSE") || norm == "REV":
		crit.critType = CritReverse
	case strings.HasPrefix(norm, "RANDOM") || norm == "RND" || norm == "SHUFFLE":
		crit.critType = CritRandom
	case strings.HasPrefix(norm, "ROLE"):
		crit.critType = CritRole
	case strings.HasPrefix(norm, "MANUAL"):
		crit.critType = CritManual
		crit.ascending = true
	case strings.HasPrefix(norm, "FAIR"):
		crit.critType = CritFair
		crit.ascending = true
	case strings.HasPrefix(norm, "TIME"):
		crit.critType = CritTime
		crit.ascending = true
	case norm == "MAIN" || norm == "MAINS":
		crit.critType = CritRole
		crit.ascending = false
	case norm == "HELPER" || norm == "HELPERS" || norm == "ALT" || norm == "ALTS":
		crit.critType = CritRole
		crit.ascending = true
	case norm == "CRAFT_DEFL" || norm == "DEFL_CRAFT" || norm == "CRAFTS" || norm == "CRAFT_DEFLECTOR" || norm == "DEFLECTOR_CRAFT" || norm == "DEFLECTOR_CRAFTS" || norm == "DEFL_CRAFTS":
		crit.critType = CritCraftDefl
	case norm == "ARTIFACT_SCORE" || norm == "ART_SCORE" || norm == "SCORE" || norm == "ARTIFACTS_SCORE" || norm == "ARTIFACTSCORE":
		crit.critType = CritArtifactScore
	case norm == "CRAFTING_XP" || norm == "CRAFT_XP" || norm == "CXP" || norm == "CRAFTINGXP" || norm == "CRAFTXP" || norm == "CRAFT_EXPERIENCE" || norm == "CRAFTING_EXPERIENCE":
		crit.critType = CritCraftingXP
	case norm == "GE" || norm == "GOLDEN_EGGS" || norm == "GOLDEN_EGG":
		crit.critType = CritGE
	case norm == "EB" || norm == "EARNINGS_BONUS" || norm == "EARNING_BONUS":
		crit.critType = CritEB
	case norm == "SE" || norm == "SOUL_EGGS" || norm == "SOUL_EGG":
		crit.critType = CritSE
	case norm == "CTE" || norm == "CLOTHED_TRUTH_EGGS" || norm == "CLOTHED_TRUTH_EGG" || norm == "CLOTHED_TE":
		crit.critType = CritCTE
	case norm == "PRESTIGE" || norm == "PRESTIGES" || norm == "PRESTIGE_COUNT":
		crit.critType = CritPrestige
	case norm == "DRONE" || norm == "DRONES" || norm == "DRONE_COUNT" || norm == "DRONE_TAKEDOWNS" || norm == "REGULAR_DRONE" || norm == "REGULAR_DRONES":
		crit.critType = CritDrone
	case norm == "ELITE_DRONE" || norm == "ELITE_DRONES" || norm == "ELITE_DRONE_COUNT" || norm == "ELITE_DRONE_TAKEDOWNS" || norm == "ELITE" || norm == "ELITES":
		crit.critType = CritEliteDrone
	case isKnownBoostID(norm):
		boostID, label := resolveBoostTarget(norm)
		crit.critType = CritBoostCount
		crit.boostID = boostID
		crit.boostLabel = label
	case reBoostExpr.MatchString(cur):
		m := reBoostExpr.FindStringSubmatch(cur)
		inner := m[1]
		if inner == "" && len(m) > 2 {
			inner = m[2]
		}
		boostID, label := resolveBoostTarget(inner)
		crit.critType = CritBoostCount
		crit.boostID = boostID
		crit.boostLabel = label
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
	case reCountExpr.MatchString(cur):
		m := reCountExpr.FindStringSubmatch(cur)
		inner := m[1]
		if inner == "" && len(m) > 2 {
			inner = m[2]
		}
		if artName, level, rarity, label, ok := parseArtifactSpecTokens(inner, false); ok {
			crit.critType = CritArtifactCount
			crit.artName = artName
			crit.artLevel = level
			crit.artRarity = rarity
			crit.artLabel = fmt.Sprintf("%s Count", label)
		} else {
			crit.critType = CritUnknown
		}
	default:
		if artName, level, rarity, label, ok := parseArtifactSpecTokens(cur, false); ok {
			crit.critType = CritArtifactHas
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

func getBoosterArtifactScore(backup *ei.Backup) float64 {
	if backup == nil || backup.GetArtifacts() == nil {
		return 0
	}
	return backup.GetArtifacts().GetInventoryScore()
}

func getBoosterCraftingXP(backup *ei.Backup) float64 {
	if backup == nil || backup.GetArtifacts() == nil {
		return 0
	}
	return backup.GetArtifacts().GetCraftingXp()
}

func getBoosterBoostCount(backup *ei.Backup, boostID string) float64 {
	if backup == nil || backup.GetGame() == nil {
		return 0
	}
	for _, b := range backup.GetGame().GetBoosts() {
		if b != nil && strings.EqualFold(b.GetBoostId(), boostID) {
			return float64(b.GetCount())
		}
	}
	return 0
}

func getBoosterGE(backup *ei.Backup) float64 {
	if backup == nil || backup.GetGame() == nil {
		return 0
	}
	earned := backup.GetGame().GetGoldenEggsEarned()
	spent := backup.GetGame().GetGoldenEggsSpent()
	if earned > spent {
		return float64(earned - spent)
	}
	return 0
}

func getBoosterEB(backup *ei.Backup) float64 {
	if backup == nil {
		return 0
	}
	te := float64(ei.GetCurrentTruthEggs(backup))
	return ei.GetEarningsBonus(backup, te)
}

func getBoosterSE(backup *ei.Backup) float64 {
	if backup == nil || backup.GetGame() == nil {
		return 0
	}
	return backup.GetGame().GetSoulEggsD()
}

func getBoosterCTE(backup *ei.Backup) float64 {
	if backup == nil {
		return 0
	}
	res := ei.CalculateMaxClothedTE(backup)
	return res.ClothedTE
}

func getBoosterPrestiges(backup *ei.Backup) float64 {
	if backup == nil || backup.GetStats() == nil {
		return 0
	}
	return float64(backup.GetStats().GetNumPrestiges())
}

func getBoosterDrones(backup *ei.Backup) float64 {
	if backup == nil || backup.GetStats() == nil {
		return 0
	}
	return float64(backup.GetStats().GetDroneTakedowns())
}

func getBoosterEliteDrones(backup *ei.Backup) float64 {
	if backup == nil || backup.GetStats() == nil {
		return 0
	}
	return float64(backup.GetStats().GetDroneTakedownsElite())
}

func getBoosterArtifactScoreVal(b *Booster) float64 {
	if b == nil {
		return 0
	}
	return getBoosterArtifactScore(getBoosterBackup(b.UserID))
}

func getBoosterCraftingXPVal(b *Booster) float64 {
	if b == nil {
		return 0
	}
	return getBoosterCraftingXP(getBoosterBackup(b.UserID))
}

func getBoosterBoostCountVal(b *Booster, boostID string) float64 {
	if b == nil {
		return 0
	}
	return getBoosterBoostCount(getBoosterBackup(b.UserID), boostID)
}

func getBoosterGEVal(b *Booster) float64 {
	if b == nil {
		return 0
	}
	return getBoosterGE(getBoosterBackup(b.UserID))
}

func getBoosterEBVal(b *Booster) float64 {
	if b == nil {
		return 0
	}
	return getBoosterEB(getBoosterBackup(b.UserID))
}

func getBoosterSEVal(b *Booster) float64 {
	if b == nil {
		return 0
	}
	return getBoosterSE(getBoosterBackup(b.UserID))
}

func getBoosterCTEVal(b *Booster) float64 {
	if b == nil {
		return 0
	}
	return getBoosterCTE(getBoosterBackup(b.UserID))
}

func getBoosterPrestigeVal(b *Booster) float64 {
	if b == nil {
		return 0
	}
	return getBoosterPrestiges(getBoosterBackup(b.UserID))
}

func getBoosterDroneVal(b *Booster) float64 {
	if b == nil {
		return 0
	}
	return getBoosterDrones(getBoosterBackup(b.UserID))
}

func getBoosterEliteDroneVal(b *Booster) float64 {
	if b == nil {
		return 0
	}
	return getBoosterEliteDrones(getBoosterBackup(b.UserID))
}

type boosterEvalData struct {
	userID          string
	signupIndex     int
	isMain          bool
	isHelper        bool
	hasT4L          bool
	t4Crafts        int
	deflQuality     string
	ihrBase         float64
	ihrSort         float64
	elr             float64
	teBase          int
	teSort          float64
	tokensWanted    int
	deflScore       int
	deflSlotScore   int
	delivRate       float64
	roleScore       float64
	randomScore     float64
	artifactScore   float64
	craftingXP      float64
	ge              float64
	eb              float64
	se              float64
	cte             float64
	prestigeCount   float64
	droneCount      float64
	eliteDroneCount float64
	boostCounts     map[string]float64
	rowScores       [4]float64
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
		boostCounts: make(map[string]float64),
	}

	if b != nil {
		prio := getBoosterRolePriority(b)
		data.isHelper = prio == RolePriorityHelper
		data.isMain = !data.isHelper
		if data.isMain {
			data.roleScore = 1.0
		} else {
			data.roleScore = 0.0
		}
		data.hasT4L = hasBoosterT4LDeflector(b)
		data.t4Crafts = getBoosterT4DeflectorCraftCount(b.UserID)
		data.deflQuality = getBoosterDeflectorQualityString(b)
		if b.IHRRate <= DefaultLeggyIHR {
			rate, logStr := CalculateIHRRateFromDB(userID)
			if rate > DefaultLeggyIHR {
				b.IHRRate = rate
				b.IHRCalcLog = logStr
			}
		}
		if b.TECount <= 0 {
			teStr := farmerstate.GetMiscSettingString(userID, "TE")
			if teStr != "" {
				if n, err := strconv.Atoi(teStr); err == nil && n > 0 {
					b.TECount = n
				}
			}
		}
		if b.ArtifactSet.LayRate == 0 && b.ArtifactSet.ShipRate == 0 && len(b.ArtifactSet.Artifacts) == 0 {
			b.ArtifactSet = getUserArtifacts(userID, nil)
		}
		data.ihrBase = b.IHRRate
		data.ihrSort = b.IHRRate
		data.elr = b.ArtifactSet.LayRate
		data.teBase = b.TECount
		data.teSort = float64(max(b.TECount, 0))
		data.tokensWanted = b.TokensWanted
		data.deflScore = getArtifactQualityScore(b, "Deflector")
		data.deflSlotScore = getBoosterDeflectorSlotScore(b)
		data.delivRate = getBoosterDeliveryRate(b)
	}

	needsBackup := false
	var neededBoostIDs []string
	var checkNeedsBackup func(c customCriterion)
	checkNeedsBackup = func(c customCriterion) {
		if c.isConditional {
			if c.thenCrit != nil {
				checkNeedsBackup(*c.thenCrit)
			}
			if c.elseCrit != nil {
				checkNeedsBackup(*c.elseCrit)
			}
			return
		}
		switch c.critType {
		case CritArtifactScore, CritCraftingXP, CritGE, CritEB, CritSE, CritCTE, CritPrestige, CritDrone, CritEliteDrone:
			needsBackup = true
		case CritBoostCount:
			needsBackup = true
			if c.boostID != "" {
				neededBoostIDs = append(neededBoostIDs, c.boostID)
			}
		}
	}
	for _, c := range criteria {
		checkNeedsBackup(c)
	}

	if needsBackup {
		backup := getBoosterBackup(userID)
		if backup != nil {
			data.artifactScore = getBoosterArtifactScore(backup)
			data.craftingXP = getBoosterCraftingXP(backup)
			data.ge = getBoosterGE(backup)
			data.eb = getBoosterEB(backup)
			data.se = getBoosterSE(backup)
			data.cte = getBoosterCTE(backup)
			data.prestigeCount = getBoosterPrestiges(backup)
			data.droneCount = getBoosterDrones(backup)
			data.eliteDroneCount = getBoosterEliteDrones(backup)
			for _, boostID := range neededBoostIDs {
				data.boostCounts[boostID] = getBoosterBoostCount(backup, boostID)
			}
		}
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
		return item.delivRate
	case CritSignup, CritManual, CritFair, CritTime:
		return float64(item.signupIndex)
	case CritReverse:
		return float64(item.signupIndex)
	case CritRandom:
		return item.randomScore
	case CritRole:
		return item.roleScore
	case CritArtifactHas:
		b := contract.Boosters[item.userID]
		if getBoosterArtifactCount(b, crit.artName, crit.artLevel, crit.artRarity) > 0 {
			return 1.0
		}
		return 0.0
	case CritArtifactCount:
		b := contract.Boosters[item.userID]
		return float64(getBoosterArtifactCount(b, crit.artName, crit.artLevel, crit.artRarity))
	case CritArtifactCraft:
		b := contract.Boosters[item.userID]
		return float64(getBoosterArtifactCraftCount(b, crit.artName, crit.artLevel))
	case CritArtifactScore:
		return item.artifactScore
	case CritCraftingXP:
		return item.craftingXP
	case CritBoostCount:
		if item.boostCounts != nil {
			return item.boostCounts[crit.boostID]
		}
		return 0
	case CritGE:
		return item.ge
	case CritEB:
		return item.eb
	case CritSE:
		return item.se
	case CritCTE:
		return item.cte
	case CritPrestige:
		return item.prestigeCount
	case CritDrone:
		return item.droneCount
	case CritEliteDrone:
		return item.eliteDroneCount
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

func getFuzzyLabelSuffix(c customCriterion) string {
	if c.fuzzySqrt {
		return "[√]"
	}
	if c.fuzzyStr != "" {
		return c.fuzzyStr
	}
	if c.fuzzyPct > 0 {
		return fmt.Sprintf("[%g%%]", c.fuzzyPct*100)
	}
	return ""
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
						roleStr, roleColor := getBoosterRoleString(b)
						return TableImageCell{Text: roleStr, Color: roleColor}
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
						roleStr, roleColor := getBoosterRoleString(b)
						return TableImageCell{Text: roleStr, Color: roleColor}
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
					col: TableImageColumn{Label: fmt.Sprintf("Effort[N=%d]", effortN), Align: bottools.StringAlignCenter},
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
						slots := getBoosterDeflectorSlotScore(b)
						text := fmt.Sprintf("%s (%d)", quality, slots)
						if quality == "-" {
							text = "-"
						}
						return TableImageCell{Text: text, Color: ""}
					},
				})
			}
		case CritIHR:
			suffix := getFuzzyLabelSuffix(c)
			id := "ihr" + suffix
			if !seenColIDs[id] {
				seenColIDs[id] = true
				activeCols = append(activeCols, customTableColDef{
					id:  id,
					col: TableImageColumn{Label: "IHR" + suffix, Align: bottools.StringAlignRight},
					evalCell: func(b *Booster, _ int) TableImageCell {
						ihrMult := fmt.Sprintf("%0.2fx", b.IHRRate/DefaultLeggyIHR)
						return TableImageCell{Text: ihrMult, Color: ""}
					},
				})
			}
		case CritELR:
			suffix := getFuzzyLabelSuffix(c)
			id := "elr" + suffix
			if !seenColIDs[id] {
				seenColIDs[id] = true
				activeCols = append(activeCols, customTableColDef{
					id:  id,
					col: TableImageColumn{Label: "ELR" + suffix, Align: bottools.StringAlignRight},
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
			suffix := getFuzzyLabelSuffix(c)
			id := "te" + suffix
			if !seenColIDs[id] {
				seenColIDs[id] = true
				activeCols = append(activeCols, customTableColDef{
					id:  id,
					col: TableImageColumn{Label: "TE" + suffix, Align: bottools.StringAlignRight},
					evalCell: func(b *Booster, _ int) TableImageCell {
						teStr := fmt.Sprintf("%d", b.TECount)
						return TableImageCell{Text: teStr, Color: ""}
					},
				})
			}
		case CritTVal:
			if !seenColIDs["tval"] {
				seenColIDs["tval"] = true
				if tvalByUser == nil && contract != nil {
					_, _, tvalByUser, _ = buildTokenTotalsFromLog(contract)
				}
				activeCols = append(activeCols, customTableColDef{
					id:  "tval",
					col: TableImageColumn{Label: "TVal", Align: bottools.StringAlignRight},
					evalCell: func(b *Booster, _ int) TableImageCell {
						val := 0.0
						if tvalByUser != nil {
							val = tvalByUser[b.UserID]
						}
						if val == 0 && b != nil && b.TokenValue != 0 {
							val = b.TokenValue
						}
						return TableImageCell{Text: fmt.Sprintf("%0.1f", val), Color: ""}
					},
				})
			}
		case CritDeliv:
			suffix := getFuzzyLabelSuffix(c)
			id := "deliv" + suffix
			if !seenColIDs[id] {
				seenColIDs[id] = true
				activeCols = append(activeCols, customTableColDef{
					id:  id,
					col: TableImageColumn{Label: "Delivery" + suffix, Align: bottools.StringAlignRight},
					evalCell: func(b *Booster, _ int) TableImageCell {
						rate := getBoosterDeliveryRate(b)
						return TableImageCell{Text: fmt.Sprintf("%0.2f", rate), Color: ""}
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
						return TableImageCell{Text: fmt.Sprintf("%d", signupIdx+1), Color: ""}
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
						return TableImageCell{Text: fmt.Sprintf("%d", signupIdx+1), Color: ""}
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
						return TableImageCell{Text: "RND", Color: ""}
					},
				})
			}
		case CritManual:
			if !seenColIDs["manual"] {
				seenColIDs["manual"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "manual",
					col: TableImageColumn{Label: "Order", Align: bottools.StringAlignRight},
					evalCell: func(_ *Booster, signupIdx int) TableImageCell {
						return TableImageCell{Text: fmt.Sprintf("%d", signupIdx+1), Color: ""}
					},
				})
			}
		case CritFair:
			if !seenColIDs["fair"] {
				seenColIDs["fair"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "fair",
					col: TableImageColumn{Label: "Fair", Align: bottools.StringAlignRight},
					evalCell: func(_ *Booster, signupIdx int) TableImageCell {
						return TableImageCell{Text: fmt.Sprintf("%d", signupIdx+1), Color: ""}
					},
				})
			}
		case CritTime:
			if !seenColIDs["time"] {
				seenColIDs["time"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "time",
					col: TableImageColumn{Label: "Time", Align: bottools.StringAlignRight},
					evalCell: func(_ *Booster, signupIdx int) TableImageCell {
						return TableImageCell{Text: fmt.Sprintf("%d", signupIdx+1), Color: ""}
					},
				})
			}
		case CritArtifactHas:
			id := fmt.Sprintf("art_has_%d_%d_%d", c.artName, c.artLevel, c.artRarity)
			if !seenColIDs[id] {
				seenColIDs[id] = true
				name := c.artName
				level := c.artLevel
				rarity := c.artRarity
				activeCols = append(activeCols, customTableColDef{
					id:  id,
					col: TableImageColumn{Label: c.artLabel, Align: bottools.StringAlignCenter},
					evalCell: func(b *Booster, _ int) TableImageCell {
						cnt := getBoosterArtifactCount(b, name, level, rarity)
						if cnt > 0 {
							return TableImageCell{Text: "Yes", Color: "green"}
						}
						return TableImageCell{Text: "-", Color: ""}
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
		case CritArtifactScore:
			if !seenColIDs["art_score"] {
				seenColIDs["art_score"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "art_score",
					col: TableImageColumn{Label: "Art Score", Align: bottools.StringAlignRight},
					evalCell: func(b *Booster, _ int) TableImageCell {
						score := getBoosterArtifactScoreVal(b)
						return TableImageCell{Text: ei.FormatEIValue(score, map[string]any{"decimals": 1, "trim": true}), Color: ""}
					},
				})
			}
		case CritCraftingXP:
			if !seenColIDs["crafting_xp"] {
				seenColIDs["crafting_xp"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "crafting_xp",
					col: TableImageColumn{Label: "Craft XP", Align: bottools.StringAlignRight},
					evalCell: func(b *Booster, _ int) TableImageCell {
						xp := getBoosterCraftingXPVal(b)
						lvl := ei.GetCraftingLevel(xp)
						return TableImageCell{Text: fmt.Sprintf("Lvl %d", lvl), Color: ""}
					},
				})
			}
		case CritBoostCount:
			id := "boost_" + c.boostID
			if !seenColIDs[id] {
				seenColIDs[id] = true
				boostID := c.boostID
				label := c.boostLabel
				if label == "" {
					label = "Boosts"
				}
				activeCols = append(activeCols, customTableColDef{
					id:  id,
					col: TableImageColumn{Label: label, Align: bottools.StringAlignRight},
					evalCell: func(b *Booster, _ int) TableImageCell {
						cnt := getBoosterBoostCountVal(b, boostID)
						return TableImageCell{Text: fmt.Sprintf("%.0f", cnt), Color: ""}
					},
				})
			}
		case CritGE:
			if !seenColIDs["ge"] {
				seenColIDs["ge"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "ge",
					col: TableImageColumn{Label: "GE", Align: bottools.StringAlignRight},
					evalCell: func(b *Booster, _ int) TableImageCell {
						ge := getBoosterGEVal(b)
						return TableImageCell{Text: ei.FormatEIValue(ge, map[string]any{"decimals": 2, "trim": true}), Color: ""}
					},
				})
			}
		case CritEB:
			if !seenColIDs["eb"] {
				seenColIDs["eb"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "eb",
					col: TableImageColumn{Label: "EB", Align: bottools.StringAlignRight},
					evalCell: func(b *Booster, _ int) TableImageCell {
						eb := getBoosterEBVal(b)
						return TableImageCell{Text: ei.FormatEIValue(eb, map[string]any{"decimals": 2, "trim": true}) + "%", Color: ""}
					},
				})
			}
		case CritSE:
			if !seenColIDs["se"] {
				seenColIDs["se"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "se",
					col: TableImageColumn{Label: "SE", Align: bottools.StringAlignRight},
					evalCell: func(b *Booster, _ int) TableImageCell {
						se := getBoosterSEVal(b)
						return TableImageCell{Text: ei.FormatEIValue(se, map[string]any{"decimals": 2, "trim": true}), Color: ""}
					},
				})
			}
		case CritCTE:
			if !seenColIDs["cte"] {
				seenColIDs["cte"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "cte",
					col: TableImageColumn{Label: "CTE", Align: bottools.StringAlignRight},
					evalCell: func(b *Booster, _ int) TableImageCell {
						cte := getBoosterCTEVal(b)
						return TableImageCell{Text: fmt.Sprintf("%.0f", cte), Color: ""}
					},
				})
			}
		case CritPrestige:
			if !seenColIDs["prestige"] {
				seenColIDs["prestige"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "prestige",
					col: TableImageColumn{Label: "Prestiges", Align: bottools.StringAlignRight},
					evalCell: func(b *Booster, _ int) TableImageCell {
						p := getBoosterPrestigeVal(b)
						return TableImageCell{Text: fmt.Sprintf("%.0f", p), Color: ""}
					},
				})
			}
		case CritDrone:
			if !seenColIDs["drone"] {
				seenColIDs["drone"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "drone",
					col: TableImageColumn{Label: "Drones", Align: bottools.StringAlignRight},
					evalCell: func(b *Booster, _ int) TableImageCell {
						d := getBoosterDroneVal(b)
						return TableImageCell{Text: fmt.Sprintf("%.0f", d), Color: ""}
					},
				})
			}
		case CritEliteDrone:
			if !seenColIDs["elite_drone"] {
				seenColIDs["elite_drone"] = true
				activeCols = append(activeCols, customTableColDef{
					id:  "elite_drone",
					col: TableImageColumn{Label: "Elite Drones", Align: bottools.StringAlignRight},
					evalCell: func(b *Booster, _ int) TableImageCell {
						ed := getBoosterEliteDroneVal(b)
						return TableImageCell{Text: fmt.Sprintf("%.0f", ed), Color: ""}
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

	for userID, b := range contract.Boosters {
		if b.IHRRate <= DefaultLeggyIHR {
			rate, logStr := CalculateIHRRateFromDB(userID)
			if rate > DefaultLeggyIHR {
				b.IHRRate = rate
				b.IHRCalcLog = logStr
			}
		}
		if b.TECount <= 0 {
			teStr := farmerstate.GetMiscSettingString(userID, "TE")
			if teStr != "" {
				if n, err := strconv.Atoi(teStr); err == nil && n > 0 {
					b.TECount = n
				}
			}
		}
		if b.ArtifactSet.LayRate == 0 && b.ArtifactSet.ShipRate == 0 && len(b.ArtifactSet.Artifacts) == 0 {
			b.ArtifactSet = getUserArtifacts(userID, nil)
		}
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

	contract := FindContractByHash(contractHash)
	if contract == nil {
		_ = e.Update(dc.Message{Content: "Unable to find this contract.", Ephemeral: true})
		return
	}

	session := getCustomOrderSession(uuidStr)
	if session == nil {
		session = findCustomOrderSession(e.UserID(), contractHash)
	}
	if session == nil {
		lines := contract.CustomOrderLines
		if len(lines) == 0 {
			saved := farmerstate.GetMiscSettingString(e.UserID(), "custom_boost_order")
			if saved != "" {
				lines = strings.Split(saved, "\n")
			}
		}
		session = getOrCreateCustomOrderSession(e.UserID(), contractHash, lines)
	}

	switch action {
	case "eval":
		_ = e.DeferUpdate()
		refreshCustomBoosters(client, contract)
		msg := BuildCustomOrderMessage(contract, session, "✓ Evaluated and refreshed boost order preview.")
		if err := e.EditResponse(msg); err != nil {
			log.Printf("HandleCustomOrderReactions eval EditResponse error: %v", err)
		}

	case "edit":
		SendCustomBoostOrderModal(e, contractHash)

	case "save":
		_ = e.DeferUpdate()
		savedStr := strings.Join(session.lines, "\n")
		farmerstate.SetMiscSettingString(session.userID, "custom_boost_order", savedStr)
		saveCustomOrderSessionDB(session)
		msg := BuildCustomOrderMessage(contract, session, "✓ Saved conditions to your personal custom boost order preset!")
		if err := e.EditResponse(msg); err != nil {
			log.Printf("HandleCustomOrderReactions save EditResponse error: %v", err)
		}

	case "load":
		_ = e.DeferUpdate()
		saved := farmerstate.GetMiscSettingString(session.userID, "custom_boost_order")
		if saved == "" {
			msg := BuildCustomOrderMessage(contract, session, "⚠️ No saved custom boost order preset found in your profile.")
			if err := e.EditResponse(msg); err != nil {
				log.Printf("HandleCustomOrderReactions load EditResponse error: %v", err)
			}
			return
		}
		parts := strings.Split(saved, "\n")
		session.lines = parts
		saveCustomOrderSessionDB(session)
		msg := BuildCustomOrderMessage(contract, session, "✓ Loaded custom boost order preset from your profile!")
		if err := e.EditResponse(msg); err != nil {
			log.Printf("HandleCustomOrderReactions load EditResponse error: %v", err)
		}

	case "apply":
		_ = e.DeferUpdate()
		refreshCustomBoosters(client, contract)
		contract.mutex.Lock()
		contract.CustomOrderLines = append([]string(nil), session.lines...)
		contract.CustomOrderName = SuggestCustomOrderName(session.lines)
		contract.BoostOrder = ContractOrderCustom
		unselected := append([]string(nil), contract.Order...)
		contract.Order = sortCustomRemaining(contract, unselected, contract.CustomOrderLines, false)
		contract.mutex.Unlock()

		saveData(contract.ContractHash)
		refreshBoostListMessage(client, contract, false)
		clearCustomOrderSession(session.uuidStr)

		channelID := e.ChannelID()
		if len(contract.Location) > 0 {
			channelID = contract.Location[0].ChannelID
		}
		inThread := false
		ch, err := client.Channel(e.ChannelID())
		if err == nil && ch.IsThread {
			inThread = true
		}
		str, comp := getSignupContractSettings(channelID, contract.ContractHash, inThread)
		var components []dc.LayoutComponent
		components = append(components, dc.TextDisplay{
			Content: str,
		})
		components = append(components, comp...)

		if err := e.EditResponse(dc.Message{Components: components}); err != nil {
			log.Printf("HandleCustomOrderReactions apply EditResponse error: %v", err)
		}

	case "exit":
		_ = e.DeferUpdate()
		clearCustomOrderSession(session.uuidStr)

		channelID := e.ChannelID()
		if len(contract.Location) > 0 {
			channelID = contract.Location[0].ChannelID
		}
		inThread := false
		ch, err := client.Channel(e.ChannelID())
		if err == nil && ch.IsThread {
			inThread = true
		}
		str, comp := getSignupContractSettings(channelID, contract.ContractHash, inThread)
		var components []dc.LayoutComponent
		components = append(components, dc.TextDisplay{
			Content: str,
		})
		components = append(components, comp...)

		if err := e.EditResponse(dc.Message{Components: components}); err != nil {
			log.Printf("HandleCustomOrderReactions exit EditResponse error: %v", err)
		}

	default:
		_ = e.Update(dc.Message{Content: "Unknown action.", Ephemeral: true})
	}
}

// GetContractBoostOrderLines returns the condition lines representing a contract's current boost order.
func GetContractBoostOrderLines(contract *Contract) []string {
	if contract == nil {
		return []string{"SIGNUP"}
	}
	if contract.BoostOrder == ContractOrderCustom && len(contract.CustomOrderLines) > 0 {
		return contract.CustomOrderLines
	}
	switch contract.BoostOrder {
	case ContractOrderSignup:
		return []string{"SIGNUP"}
	case ContractOrderReverse:
		return []string{"REVERSE"}
	case ContractOrderRandom:
		return []string{"RANDOM", "SIGNUP"}
	case ContractOrderIHR:
		return []string{"<IHR", "<DEFL", "<DELIV", "<TE"}
	case ContractOrderIHRFuzzy:
		return []string{"<IHR[6%]", "<DEFL", "<DELIV", "<TE"}
	case ContractOrderELR:
		return []string{"<ELR", "SIGNUP"}
	case ContractOrderTokenAsk:
		return []string{">TOKENS", "SIGNUP"}
	case ContractOrderTE:
		return []string{"<TE", "SIGNUP"}
	case ContractOrderTEFuzzy:
		return []string{"<TE[sqrt]", "SIGNUP"}
	case ContractOrderTVal:
		return []string{"<TVAL", ">TOKENS", "SIGNUP"}
	case ContractManualOrder:
		return []string{"MANUAL"}
	case ContractOrderFair:
		return []string{"FAIR"}
	case ContractOrderTimeBased:
		return []string{"TIME"}
	default:
		return []string{"SIGNUP"}
	}
}

// GetContractCustomOrderName returns the human-readable name of a contract's custom boost order.
func GetContractCustomOrderName(contract *Contract) string {
	if contract == nil {
		return "Custom Boost Order"
	}
	if contract.CustomOrderName != "" {
		return contract.CustomOrderName
	}

	sig := strings.Join(contract.CustomOrderLines, "\n")
	if strings.TrimSpace(sig) != "" {
		for _, g := range GetGlobalCustomOrders() {
			if strings.Join(g.Lines, "\n") == sig {
				contract.CustomOrderName = g.Name
				return g.Name
			}
		}
		if len(contract.CreatorID) > 0 {
			for _, u := range GetUserCustomOrders(contract.CreatorID[0]) {
				if strings.Join(u.Lines, "\n") == sig {
					contract.CustomOrderName = u.Name
					return u.Name
				}
			}
		}
		if suggested := SuggestCustomOrderName(contract.CustomOrderLines); suggested != "" {
			contract.CustomOrderName = suggested
			return suggested
		}
	}
	return "Custom Boost Order"
}

// BuildBoostOrderPreviewMessage creates the boost order image preview message with Keep and Dismiss buttons.
func BuildBoostOrderPreviewMessage(contract *Contract) (dc.Message, error) {
	if contract == nil {
		return dc.Message{}, fmt.Errorf("contract is nil")
	}

	lines := GetContractBoostOrderLines(contract)
	orderName := "Custom Boost Order"
	if contract.BoostOrder != ContractOrderCustom {
		if int(contract.BoostOrder) < len(contractOrderNames) {
			orderName = contractOrderNames[contract.BoostOrder]
		}
	} else {
		orderName = strings.TrimPrefix(GetContractCustomOrderName(contract), "CBO: ")
	}

	var headerSb strings.Builder
	fmt.Fprintf(&headerSb, "## ⚙️ Boost Order: **%s**\n", orderName)
	fmt.Fprintf(&headerSb, "**Contract:** `%s` | **Coop:** `%s`\n", contract.ContractID, contract.CoopID)

	if len(lines) > 0 {
		headerSb.WriteString("**Hierarchy:**\n")
		for i, l := range lines {
			if strings.TrimSpace(l) != "" {
				fmt.Fprintf(&headerSb, "-# **(%d)** `%s`\n", i+1, l)
			}
		}
	}

	imgBytes, err := RenderCustomOrderTableImage(contract, lines)
	if err != nil {
		return dc.Message{}, err
	}

	components := []dc.LayoutComponent{
		dc.TextDisplay{Content: headerSb.String()},
	}

	var files []dc.File
	if len(imgBytes) > 0 {
		components = append(components, dc.MediaGallery{
			Items: []dc.MediaItem{{URL: "attachment://boost_order.png"}},
		})
		files = []dc.File{{
			Name:        "boost_order.png",
			ContentType: "image/png",
			Reader:      bytes.NewReader(imgBytes),
		}}
	}

	components = append(components, dc.ActionRow{
		Components: []dc.InteractiveComponent{
			dc.Button{
				Label:    "Keep",
				Style:    dc.ButtonSecondary,
				CustomID: "rc_#keep#" + contract.ContractHash,
			},
			dc.Button{
				Label:    "Dismiss",
				Style:    dc.ButtonDanger,
				CustomID: "rc_#dismiss#" + contract.ContractHash,
			},
		},
	})

	return dc.Message{
		Components: components,
		Files:      files,
	}, nil
}

// criterionShortName returns a concise, human-readable label for a single criterion.
func criterionShortName(c customCriterion) string {
	if c.isConditional {
		thenName := ""
		if c.thenCrit != nil {
			thenName = criterionShortName(*c.thenCrit)
			if c.thenCrit.critType == CritTE {
				thenName = "TE"
			}
		}
		elseName := ""
		if c.elseCrit != nil {
			elseName = criterionShortName(*c.elseCrit)
			if c.elseCrit.critType == CritTE {
				elseName = "TE"
			}
		}

		target := "Role"
		switch c.targetRole {
		case roleConditionMain:
			target = "Main"
		case roleConditionHelper:
			target = "Helper"
		}

		if thenName != "" && elseName != "" {
			if target == "Main" {
				return fmt.Sprintf("Role (%s/%s)", thenName, elseName)
			}
			return fmt.Sprintf("%s (%s/%s)", target, thenName, elseName)
		} else if thenName != "" {
			if target == "Main" {
				return fmt.Sprintf("Role (%s)", thenName)
			}
			return fmt.Sprintf("%s (%s)", target, thenName)
		}
		return target
	}

	switch c.critType {
	case CritDeflEffort:
		if c.effortN != 50 && c.effortN > 0 {
			return fmt.Sprintf("Defl Effort (%d)", c.effortN)
		}
		return "Deflector Effort"
	case CritCraftDefl:
		return "Deflector Crafts"
	case CritIHR:
		if c.fuzzyPct > 0 {
			return "Fuzzy IHR"
		}
		return "IHR"
	case CritELR:
		return "ELR"
	case CritTE:
		if c.fuzzyPct > 0 || c.fuzzySqrt {
			return "Fuzzy TE"
		}
		return "Truth Eggs"
	case CritTokens:
		if !c.ascending {
			return "Most Tokens"
		}
		return "Tokens"
	case CritTVal:
		return "Token Value"
	case CritDefl:
		return "Deflector"
	case CritDeflSlot:
		return "Deflector Slot"
	case CritDeliv:
		return "Delivery Rate"
	case CritSignup:
		if !c.ascending {
			return "Reverse Signup"
		}
		return "Signup"
	case CritReverse:
		return "Reverse Signup"
	case CritRandom:
		return "Random"
	case CritRole:
		if c.ascending {
			return "Helpers First"
		}
		return "Role"
	case CritArtifactCraft:
		if c.artLabel != "" {
			return c.artLabel
		}
		return "Artifact Crafts"
	case CritArtifactCount:
		if c.artLabel != "" {
			return c.artLabel
		}
		return "Artifact Count"
	case CritArtifactHas:
		if c.artLabel != "" {
			return c.artLabel
		}
		return "Artifact"
	case CritArtifactScore:
		if c.ascending {
			return "Lowest Art Score"
		}
		return "Artifact Score"
	case CritCraftingXP:
		if c.ascending {
			return "Lowest Craft XP"
		}
		return "Crafting XP"
	case CritBoostCount:
		label := c.boostLabel
		if label == "" {
			label = "Boosts"
		}
		if c.ascending {
			return "Fewest " + label
		}
		return label
	case CritGE:
		if c.ascending {
			return "Fewest GE"
		}
		return "Golden Eggs"
	case CritEB:
		if c.ascending {
			return "Lowest EB"
		}
		return "EB"
	case CritSE:
		if c.ascending {
			return "Lowest SE"
		}
		return "Soul Eggs"
	case CritCTE:
		if c.ascending {
			return "Lowest CTE"
		}
		return "CTE"
	case CritPrestige:
		if c.ascending {
			return "Fewest Prestiges"
		}
		return "Prestiges"
	case CritDrone:
		if c.ascending {
			return "Fewest Drones"
		}
		return "Drones"
	case CritEliteDrone:
		if c.ascending {
			return "Fewest Elite Drones"
		}
		return "Elite Drones"
	case CritUnknown:
		raw := strings.TrimSpace(c.raw)
		if len(raw) > 20 {
			raw = raw[:20]
		}
		return raw
	}
	return ""
}

func formatCustomOrderNameList(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return fmt.Sprintf("%s & %s", items[0], items[1])
	default:
		return fmt.Sprintf("%s & %s", strings.Join(items[:len(items)-1], ", "), items[len(items)-1])
	}
}

// SuggestCustomOrderName analyzes custom boost order rule lines and generates a concise, readable preset name.
func SuggestCustomOrderName(lines []string) string {
	var items []string
	seen := make(map[string]bool)
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" || trimmed == "-" {
			continue
		}
		crit := parseCustomCriterion(trimmed)
		name := criterionShortName(crit)
		if name != "" && !seen[name] {
			seen[name] = true
			items = append(items, name)
		}
	}

	if len(items) == 0 {
		return ""
	}

	// Try fitting as many criteria as possible within the 50-character modal limit
	for k := len(items); k >= 1; k-- {
		candidate := formatCustomOrderNameList(items[:k])
		if len(candidate) <= 50 {
			return candidate
		}
	}

	first := items[0]
	if len(first) > 50 {
		return first[:50]
	}
	return first
}
