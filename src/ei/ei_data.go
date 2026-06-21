package ei

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// TokenUnitLog is a full log of all passed tokens
type TokenUnitLog struct {
	Time       time.Time // Time token was received
	Quantity   int       // Number of tokens
	Value      float64   // Last calculated value of the token
	FromUserID string    // Who sent the token
	FromNick   string    // Who sent the token
	ToUserID   string    // Who received the token
	ToNick     string    // Who received the token
	Serial     string    // Serial number of the token
	Boost      bool      // Whether the token part of a boost
}

// EggEvent is a raw event data for Egg Inc
type EggEvent struct {
	EndTimestamp   float64 `json:"endTimestamp"`
	ID             string  `json:"id"`
	Message        string  `json:"message"`
	Multiplier     float64 `json:"multiplier"`
	StartTimestamp float64 `json:"startTimestamp"`
	EventType      string  `json:"type"`
	Ultra          bool    `json:"ultra"`

	StartTime time.Time
	EndTime   time.Time
}

// EggIncCustomEgg is custom egg data for Egg Inc
type EggIncCustomEgg struct {
	ID                   string `json:"id"`
	Proto                string `json:"proto"`
	Name                 string
	Value                float64
	IconName             string
	IconURL              string
	IconWidth            int
	IconHeight           int
	Dimension            GameModifier_GameDimension
	DimensionName        string
	DimensionValue       []float64
	DimensionValueString []string
	Description          string
}

// GradeMultiplier is a list of multipliers for each grade
var GradeMultiplier = map[string]float64{
	"GRADE_UNSET": 0.0,
	"GRADE_C":     1.0,
	"GRADE_B":     2.0,
	"GRADE_A":     3.5,
	"GRADE_AA":    5.0,
	"GRADE_AAA":   7.0,
}

// ContractGrade is a raw contract data for a Grade in Egg Inc
type ContractGrade struct {
	TargetAmount           []float64
	LengthInSeconds        int
	EstimatedDuration      time.Duration
	EstimatedDurationLower time.Duration
	TargetTval             float64
	TargetTvalLower        float64
	//	EstimatedDurationShip      time.Duration

	ModifierEarnings     float64
	ModifierIHR          float64
	ModifierELR          float64
	ModifierSR           float64
	ModifierHabCap       float64
	ModifierAwayEarnings float64
	ModifierVehicleCost  float64
	ModifierResearchCost float64
	ModifierHabCost      float64
	BasePoints           float64
}

const (
	// SeasonUnknownID is the ID for an unknown season
	SeasonUnknownID = "season_unknown"
	// SeasonUnknown is the name for an unknown season
	SeasonUnknown = "Unknown Season"
)

// EggIncSeason is a raw contract season data for Egg Inc
type EggIncSeason struct {
	ID        string  `protobuf:"bytes,1,opt,name=id" json:"id,omitempty"`
	Name      string  `protobuf:"bytes,3,opt,name=name" json:"name,omitempty"`
	StartTime float64 `protobuf:"fixed64,4,opt,name=start_time,json=startTime" json:"start_time,omitempty"`
	//GradeGoals []*ContractSeasonInfo_GoalSet `protobuf:"bytes,2,rep,name=grade_goals,json=gradeGoals" json:"grade_goals,omitempty"`
}

// Want an enum const for the SeasonalScoring field
const (
	SeasonalScoringStandard = 0
	SeasonalScoringNerfed   = 1
)

// EggIncContract is a raw contract data for Egg Inc
type EggIncContract struct {
	ID                        string `json:"id"`
	Proto                     string `json:"proto"`
	PeriodicalAPI             bool
	Predicted                 bool
	Name                      string
	Description               string
	Egg                       int32
	EggName                   string
	CoopAllowed               bool
	Ultra                     bool
	SeasonID                  string
	MaxCoopSize               int
	TargetAmount              []float64
	ChickenRuns               int
	LengthInSeconds           int
	LengthInDays              int
	ChickenRunCooldownMinutes int
	MinutesPerToken           int
	EstimatedDuration         time.Duration
	EstimatedDurationLower    time.Duration
	EstimatedDurationMax      time.Duration
	EstimatedDurationSIAB     time.Duration
	EstimatedDurationMaxGG    time.Duration
	EstimatedDurationSIABGG   time.Duration
	TargetTval                float64
	TargetTvalLower           float64
	ModifierEarnings          float64
	ModifierIHR               float64
	ModifierELR               float64
	ModifierSR                float64
	ModifierHabCap            float64
	ModifierAwayEarnings      float64
	ModifierVehicleCost       float64
	ModifierResearchCost      float64
	ModifierHabCost           float64
	ValidFrom                 time.Time
	ValidUntil                time.Time
	HasPE                     bool
	ContractVersion           int // 1 = old, 2 = new
	Grade                     []ContractGrade
	TeamNames                 []string // Names of the teams in the contract
	// Contract Scoring Values
	CxpBuffOnly     float64          // Minimum score with only CR/TVal
	CxpRunDelta     float64          // Individual chicken run addition
	Cxp             float64          // CXP value for the contract
	CxpMax          float64          // Maximum CXP value based on EstimatedDurationMax
	CxpMaxGG        float64          // Maximum CXP value based on EstimatedDurationMaxGG
	CxpMaxSiab      float64          // Maximum CXP value based on SIAB usage
	CxpMaxSiabGG    float64          // CXP value for the contract based on SIAB usage
	SeasonalScoring int              // 0 = old (0.2.0), true = 1 (0.2.0+ seasonal change for AA+AAA)
	PredictionsList []string         // List of predictions for this contract
	History         []EggIncContract `json:"history,omitempty"` // History of replaced versions of this contract
}

// EggIncContracts holds a list of all contracts, newest is last
var EggIncContracts []EggIncContract

// EggIncContractsAll holds a list of all contracts, newest is last
var EggIncContractsAll map[string]EggIncContract

// CustomEggMap maps custom egg ID to EggIncCustomEgg
var CustomEggMap map[string]*EggIncCustomEgg

// EggIncCurrentSeason holds the current season contract, init with unknown values
var EggIncCurrentSeason = EggIncSeason{
	ID:   SeasonUnknownID,
	Name: SeasonUnknown,
}

// applePrivateUseEmojiMap covers a broad collection of the legacy
// SoftBank PUA runes mapped directly to standard UTF-8 Go runes.
var applePrivateUseEmojiMap = map[rune]rune{
	// Smileys & Emotion
	'\ue056': '😊', '\ue057': '😃', '\ue058': '😗', '\ue059': '😘',
	'\ue105': '😜', '\ue106': '😍', '\ue107': '😱', '\ue108': '😜',
	'\ue401': '😥', '\ue402': '😏', '\ue403': '😔', '\ue404': '😚',
	'\ue405': '😜', '\ue406': '😡', '\ue407': '😰', '\ue408': '👋',
	'\ue409': '😭', '\ue40a': '😭', '\ue40b': '😲', '\ue40c': '😷',
	'\ue40d': '🚀', '\ue40e': '😁', '\ue40f': '😒', '\ue410': '😅',
	'\ue411': '🏃', '\ue412': '😤', '\ue413': '😜', '\ue414': '😄',
	'\ue415': '😄', '\ue416': '🤝', '\ue417': '🤤', '\ue418': '😋',

	// Creatures / Sci-Fi
	'\ue10c': '👽', '\ue10b': '👻', '\ue428': '🐻', '\ue429': '🐱',

	// Clothing / Accessories
	'\ue31d': '👑', '\ue31e': '👟', '\ue31f': '👠', '\ue320': '👜',
	'\ue321': '💄', '\ue322': '👗', '\ue323': '👕', '\ue324': '👖',

	// Hand Gestures & People
	'\ue00e': '👍', '\ue00f': '👎', '\ue010': '👊', '\ue011': '☝',
	'\ue012': '🙌', '\ue22e': '👄', '\ue22f': '👁', '\ue230': '👂',
	'\ue231': '👃', '\ue41f': '👀', '\ue420': '👄', '\ue421': '💅',

	// Weather & Nature
	'\ue04a': '☀', '\ue04b': '☁', '\ue04c': '🌧', '\ue04d': '❄',
	'\ue04e': '🌪', '\ue04f': '⚡', '\ue050': '🌙', '\ue051': '🏞',
	'\ue303': '🌸', '\ue304': '🌹', '\ue305': '🍀', '\ue306': '🍁',
	'\ue307': '🍂', '\ue308': '🍃', '\ue310': '🌴', '\ue311': '🌵',

	// Travel & Places
	'\ue01b': '🚗', '\ue01c': '🚌', '\ue01d': '🚊', '\ue01e': '🚂',
	'\ue01f': '✈', '\ue020': '🚢', '\ue124': '⚓', '\ue125': '🚀',
	'\ue14c': '🚲', '\ue436': '🏠', '\ue437': '🏢', '\ue43a': '🏥',

	// Objects & Tech
	'\ue00a': '✉', '\ue00b': '📱', '\ue00c': '💻',
	'\ue022': '☕', '\ue023': '🍻', '\ue043': '🎥', '\ue044': '📷',
	'\ue11c': '🎙', '\ue11d': '🎸', '\ue11e': '🎹', '\ue314': '💎',
}

func init() {
	EggIncContractsAll = make(map[string]EggIncContract)
	CustomEggMap = make(map[string]*EggIncCustomEgg)
	EmoteMap = make(map[string]Emotes)
}

// NormalizePlayerNameForDisplay replaces known Apple private-use emoji with standard Unicode equivalents.
func NormalizePlayerNameForDisplay(name string) string {
	if name == "" {
		return ""
	}

	return strings.Map(func(r rune) rune {
		if replacement, ok := applePrivateUseEmojiMap[r]; ok {
			return replacement
		}
		return r
	}, name)
}

const eggUnknownName = "egg_unknown"

// FindEggEmoji will find the token emoji
func FindEggEmoji(eggOrig string) string {
	var eggIconString string

	eggOrig = strings.ReplaceAll(eggOrig, " ", "")
	eggOrig = strings.ReplaceAll(eggOrig, "-", "")
	eggOrig = strings.ReplaceAll(eggOrig, "_", "")

	if !strings.HasPrefix(eggOrig, "egg_") {
		eggOrig = "egg_" + eggOrig
	}

	var eggEmojiData Emotes
	eggIcon, ok := EmoteMap[strings.ToLower(eggOrig)]
	if ok {
		eggEmojiData = eggIcon
		eggIconString = fmt.Sprintf("<:%s:%s>", eggEmojiData.Name, eggEmojiData.ID)
	} else {
		eggEmojiData = eggIcon
		eggIconString = fmt.Sprintf("<:%s:%s>", EmoteMap[eggUnknownName].Name, EmoteMap[eggUnknownName].ID)
	}

	return eggIconString
}

// FindEggComponentEmoji will find the token emoji and return it as a ComponentEmoji
func FindEggComponentEmoji(eggOrig string) *discordgo.ComponentEmoji {
	eggOrig = strings.ReplaceAll(eggOrig, " ", "")
	eggOrig = strings.ReplaceAll(eggOrig, "-", "")
	eggOrig = strings.ReplaceAll(eggOrig, "_", "")

	if !strings.HasPrefix(eggOrig, "egg_") {
		eggOrig = "egg_" + eggOrig
	}

	if eggIcon, ok := EmoteMap[strings.ToLower(eggOrig)]; ok {
		return &discordgo.ComponentEmoji{
			Name: eggIcon.Name,
			ID:   eggIcon.ID,
		}
	}
	return &discordgo.ComponentEmoji{
		Name: EmoteMap[eggUnknownName].Name,
		ID:   EmoteMap[eggUnknownName].ID,
	}
}

// Emotes is a struct to hold the name and ID of an egg emoji
type Emotes struct {
	Name     string
	ID       string
	Animated bool
	URL      string
}

// EmoteMap of egg emojis from the Egg Inc Discord
var EmoteMap map[string]Emotes

var (
	emojiResolverMu sync.RWMutex
	emojiResolver   func(name string) (Emotes, bool)
)

// SetEmojiResolver sets a callback used to resolve missing bot emojis at runtime.
func SetEmojiResolver(resolver func(name string) (Emotes, bool)) {
	emojiResolverMu.Lock()
	emojiResolver = resolver
	emojiResolverMu.Unlock()
}

func getBotEmojiData(name string) (Emotes, bool) {
	key := strings.ToLower(name)
	if emoji, ok := EmoteMap[key]; ok {
		return emoji, true
	}

	emojiResolverMu.RLock()
	resolver := emojiResolver
	emojiResolverMu.RUnlock()
	if resolver == nil {
		return Emotes{}, false
	}

	emoji, ok := resolver(key)
	if !ok {
		return Emotes{}, false
	}
	EmoteMap[key] = emoji
	return emoji, true
}

// GetBotComponentEmoji will return a ComponentEmoji for the given name
func GetBotComponentEmoji(name string) *discordgo.ComponentEmoji {
	compEmoji := new(discordgo.ComponentEmoji)
	var emojiName string
	var emojiID string

	name = strings.ReplaceAll(name, "-", "")

	emoji, ok := getBotEmojiData(name)
	if ok {
		emojiName = emoji.Name
		emojiID = emoji.ID
	} else {
		emojiName = EmoteMap["unknown"].Name
		emojiID = EmoteMap["unknown"].ID
	}
	compEmoji.Name = emojiName
	compEmoji.ID = emojiID

	return compEmoji
}

// GetBotEmoji will return the token name and id for the given token
func GetBotEmoji(name string) (string, string, string) {
	var emojiName string
	var emojiID string
	var markdown string

	emoji, ok := getBotEmojiData(name)
	if ok {
		emojiName = emoji.Name
		emojiID = emoji.ID
	} else {
		emojiName = EmoteMap["unknown"].Name
		emojiID = EmoteMap["unknown"].ID
	}
	markdown = fmt.Sprintf("<:%s:%s>", emojiName, emojiID)

	return markdown, emojiName, emojiID
}

// GetBotEmojiMarkdown will return the token name and id for the given token
func GetBotEmojiMarkdown(name string) string {
	var emojiName string
	var emojiID string
	var markdown string
	animated := ""

	emoji, ok := getBotEmojiData(name)
	if ok {
		emojiName = emoji.Name
		emojiID = emoji.ID
		if emoji.Animated {
			animated = "a"
		}
	} else {
		emojiName = EmoteMap["unknown"].Name
		emojiID = EmoteMap["unknown"].ID
	}
	markdown = fmt.Sprintf("<%s:%s:%s>", animated, emojiName, emojiID)

	return markdown
}

// GetContractGradeString returns the string representation of the Contract_PlayerGrade
func GetContractGradeString(grade int) string {
	str := Contract_PlayerGrade_name[int32(grade)]
	parts := strings.Split(str, "_")
	if len(parts) > 1 {
		return parts[1]
	}
	return str
}

// SetEggIncCurrentSeason sets the current season
func SetEggIncCurrentSeason(seasonID, seasonName string, seasonStartTime float64) {
	EggIncCurrentSeason.ID = seasonID
	EggIncCurrentSeason.Name = seasonName
	EggIncCurrentSeason.StartTime = seasonStartTime
}

// GetEggIncCurrentSeason returns the current season name and year
// returns (name, year, seasonStartTime)
func GetEggIncCurrentSeason() (string, int, float64) {
	// Id like fall_2025, split by "_"
	parts := strings.Split(EggIncCurrentSeason.ID, "_")
	if len(parts) == 2 {
		y, err := strconv.Atoi(parts[1])
		if err == nil {
			return parts[0], y, EggIncCurrentSeason.StartTime
		}
	}
	return "", 0, 0.0
}

// GetCurrentWeekNumber computes the current week number in the current season.
// returns 1 on failure
func GetCurrentWeekNumber(locationTZ *time.Location) int {
	season := EggIncCurrentSeason
	if season.StartTime == 0 || season.ID == SeasonUnknownID {
		return 1
	}

	now := time.Now().In(locationTZ)
	seasonStart := time.Unix(int64(season.StartTime), 0).In(locationTZ)

	if now.Before(seasonStart) {
		return 1
	}

	days := int(now.Sub(seasonStart).Hours() / 24)
	return days/7 + 1
}

// EggIncContractsMutex protects EggIncContracts and EggIncContractsAll maps/slices from concurrent access.
var EggIncContractsMutex sync.RWMutex

// SetContractTeamNames updates the team names for a contract thread-safely
func SetContractTeamNames(contractID string, teamNames []string) {
	EggIncContractsMutex.Lock()
	defer EggIncContractsMutex.Unlock()

	// Update in EggIncContractsAll
	if c, ok := EggIncContractsAll[contractID]; ok {
		c.TeamNames = teamNames
		EggIncContractsAll[contractID] = c
	}

	// Update in EggIncContracts slice
	for i, c := range EggIncContracts {
		if c.ID == contractID {
			EggIncContracts[i].TeamNames = teamNames
			break
		}
	}
}

// GetContractTeamNames returns the team names for a contract thread-safely
func GetContractTeamNames(contractID string) []string {
	EggIncContractsMutex.RLock()
	defer EggIncContractsMutex.RUnlock()

	if c, ok := EggIncContractsAll[contractID]; ok {
		if len(c.TeamNames) > 0 {
			// Return a copy to prevent race conditions on slice elements
			return append([]string(nil), c.TeamNames...)
		}
	}
	return nil
}

// GetEggIncContract returns a copy of the EggIncContract for the given ID thread-safely
func GetEggIncContract(contractID string) (EggIncContract, bool) {
	EggIncContractsMutex.RLock()
	defer EggIncContractsMutex.RUnlock()
	c, ok := EggIncContractsAll[contractID]
	return c, ok
}

// GetEggIncContractByStartTime returns the version of the contract that was active at the given start time thread-safely.
// It iterates through the history of the contract to find the one active at startTime.
func GetEggIncContractByStartTime(contractID string, startTime time.Time) (EggIncContract, bool) {
	EggIncContractsMutex.RLock()
	defer EggIncContractsMutex.RUnlock()

	c, ok := EggIncContractsAll[contractID]
	if !ok {
		return EggIncContract{}, false
	}

	if !startTime.Before(c.ValidFrom) {
		return c, true
	}

	for _, h := range c.History {
		if !startTime.Before(h.ValidFrom) {
			return h, true
		}
	}

	// Fallback to the oldest version if the start time is before any ValidFrom
	if len(c.History) > 0 {
		return c.History[len(c.History)-1], true
	}
	return c, true
}

// GetEggIncContractsSlice returns a copy of the EggIncContracts slice thread-safely
func GetEggIncContractsSlice() []EggIncContract {
	EggIncContractsMutex.RLock()
	defer EggIncContractsMutex.RUnlock()
	contracts := make([]EggIncContract, len(EggIncContracts))
	copy(contracts, EggIncContracts)
	return contracts
}
