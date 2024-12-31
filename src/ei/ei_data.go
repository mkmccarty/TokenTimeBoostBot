package ei

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/encoding/protojson"

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

// EggIncContract is a raw contract data for Egg Inc
type ContractGrade struct {
	TargetAmount          []float64
	TargetAmountq         []float64
	LengthInSeconds       int
	EstimatedDuration     time.Duration
	EstimatedDurationShip time.Duration

	ModifierIHR    float64
	ModifierELR    float64
	ModifierSR     float64
	ModifierHabCap float64
}
type EggIncContract struct {
	ID                        string `json:"id"`
	Proto                     string `json:"proto"`
	PeriodicalAPI             bool
	Name                      string
	Description               string
	Egg                       int32
	EggName                   string
	CoopAllowed               bool
	MaxCoopSize               int
	TargetAmount              []float64
	TargetAmountq             []float64
	ChickenRuns               int
	LengthInSeconds           int
	ChickenRunCooldownMinutes int
	MinutesPerToken           int
	ContractDurationInDays    int
	EstimatedDuration         time.Duration
	EstimatedDurationShip     time.Duration
	ModifierIHR               float64
	ModifierELR               float64
	ModifierSR                float64
	ModifierHabCap            float64
	StartTime                 time.Time
	ExpirationTime            time.Time
	ContractVersion           int // 1 = old, 2 = new
	Grade                     []ContractGrade
}

// EggIncContracts holds a list of all contracts, newest is last
var EggIncContracts []EggIncContract

// EggIncContractsAll holds a list of all contracts, newest is last
var EggIncContractsAll map[string]EggIncContract

var CustomEggMap map[string]*EggIncCustomEgg

func init() {
	EggIncContractsAll = make(map[string]EggIncContract)
	CustomEggMap = make(map[string]*EggIncCustomEgg)
	EmoteMap = make(map[string]Emotes)
}

const eggUnknownName = "egg_unknown"

// FindEggEmoji will find the token emoji
func FindEggEmoji(eggOrig string) string {
	var eggIconString string

	if !strings.HasPrefix(eggOrig, "egg_") {
		eggOrig = "egg_" + eggOrig
	}

	eggOrig = strings.ReplaceAll(eggOrig, " ", "")
	eggOrig = strings.ReplaceAll(eggOrig, "-", "")
	eggOrig = strings.ReplaceAll(eggOrig, "_", "")

	var eggEmojiData Emotes
	eggIcon, ok := EmoteMap[strings.ToUpper(eggOrig)]
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
	Name string
	ID   string
	URL  string
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

	emoji, ok := EmoteMap[strings.ToLower(name)]
	if ok {
		emojiName = emoji.Name
		emojiID = emoji.ID
	} else {
		emojiName = EmoteMap["unknown"].Name
		emojiID = EmoteMap["unknown"].ID
	}
	markdown = fmt.Sprintf("<:%s:%s>", emojiName, emojiID)

	return markdown
}

// Artifact holds the data for each artifact
type Artifact struct {
	Type      string
	Quality   string
	ShipBuff  float64
	LayBuff   float64
	DeflBuff  float64
	Stones    int
	Dimension GameModifier_GameDimension
}

// ArtifactMap of higher level coop artifacts in the game
var ArtifactMap = map[string]*Artifact{
	"D-T4L":        {Type: "Deflector", Quality: "T4L", ShipBuff: 1.0, LayBuff: 1.0, DeflBuff: 1.20, Stones: 3},
	"D-T4E":        {Type: "Deflector", Quality: "T4E", ShipBuff: 1.0, LayBuff: 1.0, DeflBuff: 1.19, Stones: 2},
	"D-T4R":        {Type: "Deflector", Quality: "T4R", ShipBuff: 1.0, LayBuff: 1.0, DeflBuff: 1.17, Stones: 1},
	"D-T4C":        {Type: "Deflector", Quality: "T4C", ShipBuff: 1.0, LayBuff: 1.0, DeflBuff: 1.15, Stones: 0},
	"D-T3R":        {Type: "Deflector", Quality: "T3R", ShipBuff: 1.0, LayBuff: 1.0, DeflBuff: 1.13, Stones: 1},
	"D-T3C":        {Type: "Deflector", Quality: "T3C", ShipBuff: 1.0, LayBuff: 1.0, DeflBuff: 1.12, Stones: 0},
	"D-NONE":       {Type: "Deflector", Quality: "NONE", ShipBuff: 1.0, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0},
	"M-T4L":        {Type: "Metronome", Quality: "T4L", ShipBuff: 1.0, LayBuff: 1.35, DeflBuff: 1.0, Stones: 3},
	"M-T4E":        {Type: "Metronome", Quality: "T4E", ShipBuff: 1.0, LayBuff: 1.30, DeflBuff: 1.0, Stones: 2},
	"M-T4R":        {Type: "Metronome", Quality: "T4R", ShipBuff: 1.0, LayBuff: 1.25, DeflBuff: 1.0, Stones: 1},
	"M-T4C":        {Type: "Metronome", Quality: "T4C", ShipBuff: 1.0, LayBuff: 1.20, DeflBuff: 1.0, Stones: 0},
	"M-T3R":        {Type: "Metronome", Quality: "T3R", ShipBuff: 1.0, LayBuff: 1.15, DeflBuff: 1.0, Stones: 1},
	"M-T3C":        {Type: "Metronome", Quality: "T3C", ShipBuff: 1.0, LayBuff: 1.10, DeflBuff: 1.0, Stones: 0},
	"M-NONE":       {Type: "Metronome", Quality: "NONE", ShipBuff: 1.0, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0},
	"C-T4L":        {Type: "Compass", Quality: "T4L", ShipBuff: 1.50, LayBuff: 1.0, DeflBuff: 1.0, Stones: 3},
	"C-T4E":        {Type: "Compass", Quality: "T4E", ShipBuff: 1.40, LayBuff: 1.0, DeflBuff: 1.0, Stones: 2},
	"C-T4R":        {Type: "Compass", Quality: "T4R", ShipBuff: 1.35, LayBuff: 1.0, DeflBuff: 1.0, Stones: 1},
	"C-T4C":        {Type: "Compass", Quality: "T4C", ShipBuff: 1.30, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0},
	"C-T3R":        {Type: "Compass", Quality: "T3R", ShipBuff: 1.22, LayBuff: 1.0, DeflBuff: 1.0, Stones: 1},
	"C-T3C":        {Type: "Compass", Quality: "T3C", ShipBuff: 1.20, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0},
	"C-NONE":       {Type: "Compass", Quality: "NONE", ShipBuff: 1.00, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0},
	"G-T4L":        {Type: "Gusset", Quality: "T4L", ShipBuff: 1.0, LayBuff: 1.25, DeflBuff: 1.0, Stones: 3},
	"G-T4E":        {Type: "Gusset", Quality: "T4E", ShipBuff: 1.0, LayBuff: 1.22, DeflBuff: 1.0, Stones: 2},
	"G-T4C":        {Type: "Gusset", Quality: "T4C", ShipBuff: 1.0, LayBuff: 1.20, DeflBuff: 1.0, Stones: 0},
	"G-T3R":        {Type: "Gusset", Quality: "T3R", ShipBuff: 1.0, LayBuff: 1.19, DeflBuff: 1.0, Stones: 1},
	"G-T3C":        {Type: "Gusset", Quality: "T3C", ShipBuff: 1.0, LayBuff: 1.16, DeflBuff: 1.0, Stones: 0},
	"G-T2E":        {Type: "Gusset", Quality: "T2E", ShipBuff: 1.0, LayBuff: 1.12, DeflBuff: 1.0, Stones: 0},
	"G-NONE":       {Type: "Gusset", Quality: "NONE", ShipBuff: 1.0, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0},
	"NONE":         {Type: "Collegg", Quality: "NONE", ShipBuff: 1.0, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0},
	"CarbonFiber":  {Type: "Collegg", Quality: "5%", ShipBuff: 1.05, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0, Dimension: GameModifier_SHIPPING_CAPACITY},
	"Chocolate":    {Type: "Collegg", Quality: "3x", ShipBuff: 1.0, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0, Dimension: GameModifier_AWAY_EARNINGS},
	"Easter":       {Type: "Collegg", Quality: "5%", ShipBuff: 1.0, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0, Dimension: GameModifier_INTERNAL_HATCHERY_RATE},
	"Firework":     {Type: "Collegg", Quality: "5%", ShipBuff: 1.0, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0, Dimension: GameModifier_EARNINGS},
	"Lithium":      {Type: "Collegg", Quality: "10%", ShipBuff: 1.0, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0, Dimension: GameModifier_VEHICLE_COST},
	"Pumpkin":      {Type: "Collegg", Quality: "5%", ShipBuff: 1.05, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0, Dimension: GameModifier_SHIPPING_CAPACITY},
	"Waterballoon": {Type: "Collegg", Quality: "95%", ShipBuff: 1.0, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0, Dimension: GameModifier_RESEARCH_COST},
}

var data *Store
var artifactConfig *ArtifactsConfigurationResponse

type Store struct {
	Schema           string    `json:"$schema"`
	ArtifactFamilies []*Family `json:"artifact_families"`
}

type Family struct {
	CoreFamily

	Effect       string  `json:"effect"`
	EffectTarget string  `json:"effect_target"`
	Tiers        []*Tier `json:"tiers"`
}

type CoreFamily struct {
	ID          string              `json:"id"`
	AfxID       ArtifactSpec_Name   `json:"afx_id"`
	Name        string              `json:"name"`
	AfxType     ArtifactSpec_Type   `json:"afx_type"`
	Type        string              `json:"type"`
	SortKey     uint32              `json:"sort_key"`
	ChildAfxIds []ArtifactSpec_Name `json:"child_afx_ids"`
}

type Tier struct {
	Family *CoreFamily `json:"family"`

	CoreTier

	Quality               float64               `json:"quality"`
	Craftable             bool                  `json:"craftable"`
	BaseCraftingPrices    []float64             `json:"base_crafting_prices"`
	HasRarities           bool                  `json:"has_rarities"`
	PossibleAfxRarities   []ArtifactSpec_Rarity `json:"possible_afx_rarities"`
	HasEffects            bool                  `json:"has_effects"`
	AvailableFromMissions bool                  `json:"available_from_missions"`

	Effects []*Effect `json:"effects"`
	Recipe  *Recipe   `json:"recipe"`

	IngredientsAvailableFromMissions bool         `json:"ingredients_available_from_missions"`
	HardDependencies                 []Ingredient `json:"hard_dependencies"`
	OddsMultiplier                   float64      `json:"odds_multiplier"`
}

type CoreTier struct {
	ItemIdentifiers
	TierNumber   int               `json:"tier_number"`
	TierName     string            `json:"tier_name"`
	AfxType      ArtifactSpec_Type `json:"afx_type"`
	Type         string            `json:"type"`
	IconFilename string            `json:"icon_filename"`
}

type ItemIdentifiers struct {
	ID       string             `json:"id"`
	AfxID    ArtifactSpec_Name  `json:"afx_id"`
	AfxLevel ArtifactSpec_Level `json:"afx_level"`
	Name     string             `json:"name"`
}

type Effect struct {
	AfxRarity    ArtifactSpec_Rarity `json:"afx_rarity"`
	Rarity       string              `json:"rarity"`
	Effect       string              `json:"effect"`
	EffectTarget string              `json:"effect_target"`
	EffectSize   string              `json:"effect_size"`
	EffectDelta  float64             `json:"effect_delta"`
	FamilyEffect string              `json:"family_effect"`
	// May be null (for stones).
	Slots          *uint32 `json:"slots"`
	OddsMultiplier float64 `json:"odds_multiplier"`
}

type Recipe struct {
	Ingredients   []Ingredient  `json:"ingredients"`
	CraftingPrice CraftingPrice `json:"crafting_price"`
}

type Ingredient struct {
	CoreTier
	Count uint32 `json:"count"`
}

type CraftingPrice struct {
	Base    float64 `json:"base"`
	Low     float64 `json:"low"`
	Domain  uint32  `json:"domain"`
	Curve   float64 `json:"curve"`
	Initial uint32  `json:"initial"`
	Minimum uint32  `json:"minimum"`
}

func LoadConfig(configFile string) error {

	// Read the dataFile from disk into _eiafxConfigJSON
	fileContent, err := os.ReadFile(configFile)
	if err != nil {
		return errors.Wrap(err, "error reading configFile")
	}
	_eiafxConfigJSON := []byte(strings.Replace(string(fileContent), "./data.schema.json", "./ttbb-data/data.schema.json", -1))

	artifactConfig = &ArtifactsConfigurationResponse{}
	err = protojson.Unmarshal(_eiafxConfigJSON, artifactConfig)
	if err != nil {
		return errors.Wrap(err, "error unmarshalling eiafx-config.json")
	}
	return nil
}

func LoadData(dataFile string) error {

	// Read the dataFile from disk into _eiafxConfigJSON
	fileContent, err := os.ReadFile(dataFile)
	if err != nil {
		return errors.Wrap(err, "error reading dataFile")
	}
	_eiafxDataJSON := []byte(strings.Replace(string(fileContent), "./data.schema.json", "./ttbb-data/data.schema.json", -1))

	data = &Store{}
	err = json.Unmarshal(_eiafxDataJSON, data)
	if err != nil {
		return errors.Wrap(err, "error unmarshalling eiafx-data.json")
	}

	return nil
}

// GetStones returns the number of stones for the given artifact
func GetStones(afxName ArtifactSpec_Name, afxLevel ArtifactSpec_Level, afxRarity ArtifactSpec_Rarity) (int, error) {
	//afxID := fmt.Sprintf("%s-%d", spec.Name, spec.GetLevel())
	//familyAfxID := spec.Name
	for _, f := range data.ArtifactFamilies {
		if f.AfxID != afxName {
			continue
		}
		//fmt.Print(f.AfxID)
		tier := f.Tiers[afxLevel]
		for _, e := range tier.Effects {
			if e.AfxRarity == afxRarity {
				return int(*e.Slots), nil
			}
		}
		/*
			if f.AfxId == familyAfxID {
				for _, t := range f.Tiers {
					if t.AfxId == afxID && t.AfxLevel == afxLevel {
						return t, nil
					}
				}
				break
			}
		*/
	}
	return 0, errors.Errorf("artifact (%s, %s) not found in data.json", afxName, afxLevel)
}

// GetGameDimensionString returns the string representation of the GameModifier_GameDimension
func GetGameDimensionString(d GameModifier_GameDimension) string {
	switch d {
	case GameModifier_INVALID:
		return "Invalid"
	case GameModifier_EARNINGS:
		return "Earnings"
	case GameModifier_AWAY_EARNINGS:
		return "Away Earnings"
	case GameModifier_INTERNAL_HATCHERY_RATE:
		return "Internal Hatchery Rate"
	case GameModifier_EGG_LAYING_RATE:
		return "Egg Laying Rate"
	case GameModifier_SHIPPING_CAPACITY:
		return "Shipping Capacity"
	case GameModifier_HAB_CAPACITY:
		return "HAB_CAPACITY"
	case GameModifier_VEHICLE_COST:
		return "Vehicle Cost"
	case GameModifier_HAB_COST:
		return "Hab Cost"
	case GameModifier_RESEARCH_COST:
		return "Research Cost"
	default:
		return "Unknown"
	}
}

// GetContractGradeString returns the string representation of the Contract_PlayerGrade
func GetContractGradeString(grade int32) string {
	str := Contract_PlayerGrade_name[grade]
	parts := strings.Split(str, "_")
	if len(parts) > 1 {
		return parts[1]
	}
	return str
}
