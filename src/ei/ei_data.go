package ei

import (
	"fmt"
	"strconv"
	"strings"
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
	SeasonUnknownID = "season_unknown"
	SeasonUnknown   = "Unknown Season"
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
	ParadeChickenRuns         int
	LengthInSeconds           int
	LengthInDays              int
	ChickenRunCooldownMinutes int
	MinutesPerToken           int
	EstimatedDuration         time.Duration
	EstimatedDurationLower    time.Duration
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
	CxpBuffOnly     float64 // Minimum score with only CR/TVal
	CxpRunDelta     float64 // Individual chicken run addition
	Cxp             float64 // CXP value for the contract
	SeasonalScoring int     // 0 = old (0.2.0), true = 1 (0.2.0+ seasonal change for AA+AAA)
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

func init() {
	EggIncContractsAll = make(map[string]EggIncContract)
	CustomEggMap = make(map[string]*EggIncCustomEgg)
	EmoteMap = make(map[string]Emotes)
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

// Emotes is a struct to hold the name and ID of an egg emoji
type Emotes struct {
	Name     string
	ID       string
	Animated bool
	URL      string
}

// EmoteMap of egg emojis from the Egg Inc Discord
var EmoteMap map[string]Emotes

// GetBotComponentEmoji will return a ComponentEmoji for the given name
func GetBotComponentEmoji(name string) *discordgo.ComponentEmoji {
	compEmoji := new(discordgo.ComponentEmoji)
	var emojiName string
	var emojiID string

	name = strings.ReplaceAll(name, "-", "")

	emoji, ok := EmoteMap[strings.ToLower(name)]
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

	emoji, ok := EmoteMap[strings.ToLower(name)]
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

	emoji, ok := EmoteMap[strings.ToLower(name)]
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
