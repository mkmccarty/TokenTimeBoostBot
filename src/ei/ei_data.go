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

var currentGGEvent = 1.0
var currentUltraGGEvent = 1.0

var colleggtibleELR = 1.0
var colleggtibleShip = 1.0
var colleggtibleHab = 1.0

// GetColleggtibleValues will return the current values of the 3 collectibles
func GetColleggtibleValues() (float64, float64, float64) {
	return colleggtibleELR, colleggtibleShip, colleggtibleHab
}

// SetColleggtibleValues will set the values of the 3 collectibles based on CustomEggMap
func SetColleggtibleValues() {
	colELR := 1.0
	colShip := 1.0
	colHab := 1.0

	for _, eggValue := range CustomEggMap {
		switch eggValue.Dimension {
		case GameModifier_EGG_LAYING_RATE:
			colELR *= eggValue.DimensionValue[len(eggValue.DimensionValue)-1]
		case GameModifier_SHIPPING_CAPACITY:
			colShip *= eggValue.DimensionValue[len(eggValue.DimensionValue)-1]
		case GameModifier_HAB_CAPACITY:
			colHab *= eggValue.DimensionValue[len(eggValue.DimensionValue)-1]
		}
	}

	colleggtibleELR = colELR
	colleggtibleShip = colShip
	colleggtibleHab = colHab
}

// GetGenerousGiftEvent will return the current Generous Gift event multiplier
func GetGenerousGiftEvent() (float64, float64) {
	return currentGGEvent, currentUltraGGEvent
}

// SetGenerousGiftEvent will return the current Generous Gift event multiplier
func SetGenerousGiftEvent(gg float64, ugg float64) {
	currentGGEvent = gg
	currentUltraGGEvent = ugg
}

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
	TargetAmountq          []float64
	LengthInSeconds        int
	EstimatedDuration      time.Duration
	EstimatedDurationLower time.Duration
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

	BasePoints float64
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
	Ultra                     bool
	MaxCoopSize               int
	TargetAmount              []float64
	TargetAmountq             []float64
	ChickenRuns               int
	LengthInSeconds           int
	ChickenRunCooldownMinutes int
	MinutesPerToken           int
	ContractDurationInDays    int
	EstimatedDuration         time.Duration
	EstimatedDurationLower    time.Duration
	ModifierEarnings          float64
	ModifierIHR               float64
	ModifierELR               float64
	ModifierSR                float64
	ModifierHabCap            float64
	ModifierAwayEarnings      float64
	ModifierVehicleCost       float64
	ModifierResearchCost      float64
	ModifierHabCost           float64
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
	"D-T4L":        {Type: "Deflector", Quality: "T4L", ShipBuff: 1.0, LayBuff: 1.0, DeflBuff: 1.20, Stones: 2},
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
func GetContractGradeString(grade int) string {
	str := Contract_PlayerGrade_name[int32(grade)]
	parts := strings.Split(str, "_")
	if len(parts) > 1 {
		return parts[1]
	}
	return str
}

// Vehicles

type vehicleType struct {
	ID           uint32
	Name         string
	BaseCapacity float64 // Unupgraded shipping capacity per second.
}

var vehicleTypes = map[uint32]vehicleType{
	0: {
		ID:           0,
		Name:         "Trike",
		BaseCapacity: 5e3,
	},
	1: {
		ID:           1,
		Name:         "Transit Van",
		BaseCapacity: 15e3,
	},
	2: {
		ID:           2,
		Name:         "Pickup",
		BaseCapacity: 50e3,
	},
	3: {
		ID:           3,
		Name:         "10 Foot",
		BaseCapacity: 100e3,
	},
	4: {
		ID:           4,
		Name:         "24 Foot",
		BaseCapacity: 250e3,
	},
	5: {
		ID:           5,
		Name:         "Semi",
		BaseCapacity: 500e3,
	},
	6: {
		ID:           6,
		Name:         "Double Semi",
		BaseCapacity: 1e6,
	},
	7: {
		ID:           7,
		Name:         "Future Semi",
		BaseCapacity: 5e6,
	},
	8: {
		ID:           8,
		Name:         "Mega Semi",
		BaseCapacity: 15e6,
	},
	10: {
		ID:           10,
		Name:         "Quantum Transporter",
		BaseCapacity: 50e6,
	},
	11: {
		ID:           11,
		Name:         "Hyperloop Train",
		BaseCapacity: 50e6,
	},
}

func isHoverVehicle(vehicle vehicleType) bool {
	return vehicle.ID >= 9
}

func isHyperloop(vehicle vehicleType) bool {
	return vehicle.ID == 11
}

func GetVehiclesShippingCapacity(vehicles []uint32, trainLength []uint32, univMult float64, hoverOnlyMult float64, hyperOnlyMult float64) (float64, string) {
	userShippingCap := 0.0
	shippingNote := ""
	fullyUpgraded := true

	for i, v := range vehicles {
		if v == 0 {
			continue
		}
		vehicleType := vehicleTypes[v]
		capacity := vehicleType.BaseCapacity
		if vehicleType.ID != 11 && trainLength[i] != 10 {
			fullyUpgraded = false
		}
		if isHoverVehicle(vehicleType) {
			capacity *= hoverOnlyMult
		}
		capacity *= univMult
		if isHyperloop(vehicleType) {
			capacity *= hyperOnlyMult
			if trainLength[i] > 0 {
				lengthOfOneTrain := trainLength[i]
				capacity *= float64(lengthOfOneTrain)
			}
		}

		userShippingCap += capacity

	}
	if !fullyUpgraded {
		shippingNote = "Vehicles not fully upgraded"
	}

	return userShippingCap, shippingNote
}

// Hab structure for hab Data
type Hab struct {
	ID           int
	Name         string
	IconPath     string
	BaseCapacity float64
}

// Habs is a list of all habs in the game
var Habs = []Hab{
	{
		ID:           0,
		Name:         "Coop",
		IconPath:     "egginc/ei_hab_icon_coop.png",
		BaseCapacity: 250,
	},
	{
		ID:           1,
		Name:         "Shack",
		IconPath:     "egginc/ei_hab_icon_shack.png",
		BaseCapacity: 500,
	},
	{
		ID:           2,
		Name:         "Super Shack",
		IconPath:     "egginc/ei_hab_icon_super_shack.png",
		BaseCapacity: 1e3,
	},
	{
		ID:           3,
		Name:         "Short House",
		IconPath:     "egginc/ei_hab_icon_short_house.png",
		BaseCapacity: 2e3,
	},
	{
		ID:           4,
		Name:         "The Standard",
		IconPath:     "egginc/ei_hab_icon_the_standard.png",
		BaseCapacity: 5e3,
	},
	{
		ID:           5,
		Name:         "Long House",
		IconPath:     "egginc/ei_hab_icon_long_house.png",
		BaseCapacity: 1e4,
	},
	{
		ID:           6,
		Name:         "Double Decker",
		IconPath:     "egginc/ei_hab_icon_double_decker.png",
		BaseCapacity: 2e4,
	},
	{
		ID:           7,
		Name:         "Warehouse",
		IconPath:     "egginc/ei_hab_icon_warehouse.png",
		BaseCapacity: 5e4,
	},
	{
		ID:           8,
		Name:         "Center",
		IconPath:     "egginc/ei_hab_icon_center.png",
		BaseCapacity: 1e5,
	},
	{
		ID:           9,
		Name:         "Bunker",
		IconPath:     "egginc/ei_hab_icon_bunker.png",
		BaseCapacity: 2e5,
	},
	{
		ID:           10,
		Name:         "Eggkea",
		IconPath:     "egginc/ei_hab_icon_eggkea.png",
		BaseCapacity: 5e5,
	},
	{
		ID:           11,
		Name:         "HAB 1000",
		IconPath:     "egginc/ei_hab_icon_hab1k.png",
		BaseCapacity: 1e6,
	},
	{
		ID:           12,
		Name:         "Hangar",
		IconPath:     "egginc/ei_hab_icon_hanger.png",
		BaseCapacity: 2e6,
	},
	{
		ID:           13,
		Name:         "Tower",
		IconPath:     "egginc/ei_hab_icon_tower.png",
		BaseCapacity: 5e6,
	},
	{
		ID:           14,
		Name:         "HAB 10,000",
		IconPath:     "egginc/ei_hab_icon_hab10k.png",
		BaseCapacity: 1e7,
	},
	{
		ID:           15,
		Name:         "Eggtopia",
		IconPath:     "egginc/ei_hab_icon_eggtopia.png",
		BaseCapacity: 2.5e7,
	},
	{
		ID:           16,
		Name:         "Monolith",
		IconPath:     "egginc/ei_hab_icon_monolith.png",
		BaseCapacity: 5e7,
	},
	{
		ID:           17,
		Name:         "Planet Portal",
		IconPath:     "egginc/ei_hab_icon_portal.png",
		BaseCapacity: 1e8,
	},
	{
		ID:           18,
		Name:         "Chicken Universe",
		IconPath:     "egginc/ei_hab_icon_chicken_universe.png",
		BaseCapacity: 6e8,
	},
}

// IsPortalHab returns true if the hab is a portal hab
func IsPortalHab(hab Hab) bool {
	return hab.ID >= 17
}

// ShortArtifactName maps artifact IDs to their corresponding short names.
// The map uses int32 keys representing the artifact IDs and string values representing the artifact names.
var ShortArtifactName = map[int32]string{
	0:     "TOTEM_",
	3:     "MEDALLION_",
	4:     "BEAK_",
	5:     "LOE_",
	6:     "NECKLACE_",
	7:     "VIAL_",
	8:     "GUSSET_",
	9:     "CHALICE_",
	10:    "BOOK_",
	11:    "FEATHER_",
	12:    "ANKH_",
	21:    "BROOCH_",
	22:    "RAINSTICK_",
	23:    "CUBE_",
	24:    "METR_",
	25:    "SIAB_",
	26:    "DEFL_",
	27:    "COMP_",
	28:    "MONOCLE_",
	29:    "ACTUATOR_",
	30:    "LENS_",
	1:     "TACHYON_",
	31:    "DILITHIUM_",
	32:    "SHELL_",
	33:    "LUNAR_",
	34:    "SOUL_",
	39:    "PROPHECY_",
	36:    "QUANTUM_",
	37:    "TERRA_",
	38:    "LIFE_",
	40:    "CLARITY_",
	13:    "ALUMINUM_",
	14:    "TUNGSTEN_",
	15:    "ROCKS_",
	16:    "WOOD_",
	17:    "GOLD_",
	18:    "GEODE_",
	19:    "STEEL_",
	20:    "ERIDANI_",
	35:    "PARTS_",
	41:    "BRONZE_",
	42:    "HIDE_",
	43:    "TITANIUM_",
	2:     "TACHYON_STONE_FRAGMENT_",
	44:    "DILITHIUM_STONE_FRAGMENT_",
	45:    "SHELL_STONE_FRAGMENT_",
	46:    "LUNAR_STONE_FRAGMENT_",
	47:    "SOUL_STONE_FRAGMENT_",
	48:    "PROPHECY_STONE_FRAGMENT_",
	49:    "QUANTUM_STONE_FRAGMENT_",
	50:    "TERRA_STONE_FRAGMENT_",
	51:    "LIFE_STONE_FRAGMENT_",
	52:    "CLARITY_STONE_FRAGMENT_",
	10000: "UNKNOWN_",
}
