package ei

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
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
	ID             string `json:"id"`
	Proto          string `json:"proto"`
	Name           string
	Value          float64
	IconName       string
	IconURL        string
	IconWidth      int
	IconHeight     int
	Dimension      GameModifier_GameDimension
	DimensionValue []float64
}

// EggIncContract is a raw contract data for Egg Inc
type EggIncContract struct {
	ID                        string `json:"id"`
	Proto                     string `json:"proto"`
	Name                      string
	Description               string
	Egg                       int32
	EggName                   string
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
}

// EggIncContracts holds a list of all contracts, newest is last
var EggIncContracts []EggIncContract

// EggIncContractsAll holds a list of all contracts, newest is last
var EggIncContractsAll map[string]EggIncContract

var CustomEggMap map[string]*EggIncCustomEgg

func init() {
	EggIncContractsAll = make(map[string]EggIncContract)
	CustomEggMap = make(map[string]*EggIncCustomEgg)
}

// EggEmojiData is a struct to hold the name and ID of an egg emoji
type EggEmojiData struct {
	Name  string
	ID    string
	DevID string
}

// EggEmojiMap of egg emojis from the Egg Inc Discord
var EggEmojiMap = map[string]EggEmojiData{
	"EDIBLE":         {"egg_edible", "1279201065983672331", "1280382273165725707"},
	"SUPERFOOD":      {"egg_superfood", "1279201277498101871", "1280383008901173400"},
	"MEDICAL":        {"egg_medical", "1279201164264607845", "1280382360554180661"},
	"ROCKET_FUEL":    {"egg_rocketfuel", "1279201250310881301", "1280382980753457162"},
	"SUPER_MATERIAL": {"egg_supermaterial", "1279201294040432690", "1280383030858350624"},
	"FUSION":         {"egg_fusion", "1279201108119519254", "1280382311166382113"},
	"QUANTUM":        {"egg_quantum", "1279201237106954260", "1280382968069886045"},
	"CRISPR":         {"egg_crispr", "1279202262941696032", "1280385130363490324"},
	"IMMORTALITY":    {"egg_crispr", "1279201128675934263", "1280382335577227355"},
	"TACHYON":        {"egg_tachyon", "1279201308494135316", "1280383045710381066"},
	"GRAVITON":       {"egg_graviton", "1279201119066783765", "1280382323652821124"},
	"DILITHIUM":      {"egg_dilithium", "1279201030747459717", "1280382245189713984"},
	"PRODIGY":        {"egg_prodigy", "1279201210473123954", "1280382940383285290"},
	"TERRAFORM":      {"egg_terraform", "1279201322671014042", "1280383061007269910"},
	"ANTIMATTER":     {"egg_antimatter", "1279200966423347311", "1280382205084041287"},
	"DARKMATTER":     {"egg_darkmatter", "1279201008471380019", "1280382231361093706"},
	"AI":             {"egg_ai", "1279200905081782313", "1280382185257566278"},
	"NEBULA":         {"egg_nebula", "1279201366396633231", "1280383107236626482"},
	"UNIVERSE":       {"egg_universe", "1279201333081145364", "1280383077415125086"},
	"ENLIGHTENMENT":  {"egg_enlightenment", "1279201086531702895", "1280382287980269618"},
	"UNKNOWN":        {"egg_unknown", "1279201352408633425", "1280383094448328717"},
	"WATERBALLOON":   {"egg_waterballoon", "1279201379227009076", "1280383119618473984"},
	"FIREWORK":       {"egg_firework", "1279201097348812830", "1280382299141050399"},
	"PUMPKIN":        {"egg_pumpkin", "1279201221235703900", "1280382955239247965"},
	"CHOCOLATE":      {"egg_chocolate", "1279200983523524659", "1280382217822146560"},
	"EASTER":         {"egg_easter", "1279201048845881414", "1280382259630964767"},
	"CARBON-FIBER":   {"egg_carbonfiber", "1279202173904752802", "1280385218213187640"},
	"LITHIUM":        {"egg_lithium", "1305429259048718387", "1305429887749853185"},
	"SOUL":           {"egg_soul", "1279201265628348490", "1280382995282526208"},
	"PROPHECY":       {"egg_prophecy", "1279201195872878652", "1280382926470643754"},
}

// FindEggComponentEmoji will find the token emoji for the given guild
func FindEggComponentEmoji(eggOrig string) (string, EggEmojiData) {
	var eggIconString string

	var eggEmojiData EggEmojiData

	eggIcon, ok := EggEmojiMap[strings.ToUpper(eggOrig)]
	if config.IsDevBot() {
		if ok {
			eggEmojiData = eggIcon
			eggIconString = fmt.Sprintf("<:%s:%s>", eggEmojiData.Name, eggEmojiData.DevID)
		} else {
			eggEmojiData = eggIcon
			eggIconString = fmt.Sprintf("<:%s:%s>", EggEmojiMap["UNKNOWN"].Name, EggEmojiMap["UNKNOWN"].DevID)
		}

	} else {
		if ok {
			eggEmojiData = eggIcon
			eggIconString = fmt.Sprintf("<:%s:%s>", eggEmojiData.Name, eggEmojiData.ID)
		} else {
			eggEmojiData = eggIcon
			eggIconString = fmt.Sprintf("<:%s:%s>", EggEmojiMap["UNKNOWN"].Name, EggEmojiMap["UNKNOWN"].ID)
		}
	}
	return eggIconString, eggEmojiData
}

// FindEggEmoji will find the token emoji for the given guild
func FindEggEmoji(eggOrig string) string {
	var eggIconString string

	var eggEmojiData EggEmojiData
	if config.IsDevBot() {
		eggIcon, ok := EggEmojiMap[strings.ToUpper(eggOrig)]
		if ok {
			eggEmojiData = eggIcon
			eggIconString = fmt.Sprintf("<:%s:%s>", eggEmojiData.Name, eggEmojiData.DevID)
		} else {
			eggEmojiData = eggIcon
			eggIconString = fmt.Sprintf("<:%s:%s>", EggEmojiMap["UNKNOWN"].Name, EggEmojiMap["UNKNOWN"].DevID)
		}
	} else {
		eggIcon, ok := EggEmojiMap[strings.ToUpper(eggOrig)]
		if ok {
			eggEmojiData = eggIcon
			eggIconString = fmt.Sprintf("<:%s:%s>", eggEmojiData.Name, eggEmojiData.ID)
		} else {
			eggEmojiData = eggIcon
			eggIconString = fmt.Sprintf("<:%s:%s>", EggEmojiMap["UNKNOWN"].Name, EggEmojiMap["UNKNOWN"].ID)
		}

	}

	return eggIconString
}

// BotEmojiData is a struct to hold the name and ID of an egg emoji
type BotEmojiData struct {
	Name  string
	ID    string
	DevID string
}

// BotEmojiMap of all bot emojis for production and development environments
var BotEmojiMap = map[string]BotEmojiData{
	"runready": {"runchicken", "1288641440074698845", "1288641283010728037"},
	"unknown":  {"unknown", "1288638240886095872", "1288638345970454570"},
	"signup":   {"signup", "1288391738758795284", "1288391860431360093"},
	"reverse":  {"reverse", "1288577307451326575", "1288577133995753635"},
	"random":   {"random", "1288394311305658452", "1288394382516424725"},
	"fair":     {"fair", "1288579576435445792", "1288579727598026857"},
	"elr":      {"ELR", "1288152787494109216", "1288152690001580072"},
	"sharing":  {"sharing", "1288581074124804106", "1288580958794158090"},
	"ultra":    {"ultra", "1286890801963470848", "1286890849719812147"},
	"token":    {"token", "1279216492927385652", "1279216759131476123"},
}

// GetBotComponentEmoji will return a ComponentEmoji for the given name
func GetBotComponentEmoji(name string) *discordgo.ComponentEmoji {
	compEmoji := new(discordgo.ComponentEmoji)
	var emojiName string
	var emojiID string

	emoji, ok := BotEmojiMap[strings.ToLower(name)]
	if ok {
		emojiName = emoji.Name
		emojiID = emoji.ID
		if config.IsDevBot() {
			emojiID = emoji.DevID
		}
	} else {
		emojiName = BotEmojiMap["unknown"].Name
		emojiID = BotEmojiMap["unknown"].ID
		if config.IsDevBot() {
			emojiID = BotEmojiMap["unknown"].DevID
		}
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

	emoji, ok := BotEmojiMap[strings.ToLower(name)]
	if ok {
		emojiName = emoji.Name
		emojiID = emoji.ID
		if config.IsDevBot() {
			emojiID = emoji.DevID
		}
	} else {
		emojiName = BotEmojiMap["unknown"].Name
		emojiID = BotEmojiMap["unknown"].ID
		if config.IsDevBot() {
			emojiID = BotEmojiMap["unknown"].DevID
		}
	}
	markdown = fmt.Sprintf("<:%s:%s>", emojiName, emojiID)

	return markdown, emojiName, emojiID
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
		return "INVALID"
	case GameModifier_EARNINGS:
		return "EARNINGS"
	case GameModifier_AWAY_EARNINGS:
		return "AWAY_EARNINGS"
	case GameModifier_INTERNAL_HATCHERY_RATE:
		return "INTERNAL_HATCHERY_RATE"
	case GameModifier_EGG_LAYING_RATE:
		return "EGG_LAYING_RATE"
	case GameModifier_SHIPPING_CAPACITY:
		return "SHIPPING_CAPACITY"
	case GameModifier_HAB_CAPACITY:
		return "HAB_CAPACITY"
	case GameModifier_VEHICLE_COST:
		return "VEHICLE_COST"
	case GameModifier_HAB_COST:
		return "HAB_COST"
	case GameModifier_RESEARCH_COST:
		return "RESEARCH_COST"
	default:
		return "UNKNOWN"
	}
}
