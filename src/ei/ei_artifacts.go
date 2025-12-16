package ei

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/encoding/protojson"
)

// DimensionBuffs holds the various dimension buffs
type DimensionBuffs struct {
	ELR              float64
	SR               float64
	IHR              float64
	Hab              float64
	Earnings         float64
	AwayEarnings     float64
	ResearchDiscount float64
}

// ArtifactLevels are used to map the level enums to strings
var ArtifactLevels = []string{"T1", "T2", "T3", "T4", "T5"}

// ArtifactRarity are used to map the level and rarity enums to strings
var ArtifactRarity = []string{"C", "R", "E", "L"}

// GetArtifactBuffs calculates the total buffs from artifacts
func GetArtifactBuffs(artifacts []*CompleteArtifact) DimensionBuffs {
	artifactBuffs := DimensionBuffs{
		ELR:              1.0,
		SR:               1.0,
		IHR:              1.0,
		Hab:              1.0,
		Earnings:         1.0,
		AwayEarnings:     1.0,
		ResearchDiscount: 1.0,
	}

	chalice := map[string]float64{
		"T1C": 1.05,
		"T2C": 1.10, "T2E": 1.15,
		"T3C": 1.20, "T3R": 1.23, "T3E": 1.25,
		"T4C": 1.30, "T4E": 1.35, "T4L": 1.40,
	}
	metronome := map[string]float64{
		"T1C": 1.05,
		"T2C": 1.10, "T2R": 1.12,
		"T3C": 1.15, "T3R": 1.17, "T3E": 1.20,
		"T4C": 1.25, "T4R": 1.27, "T4E": 1.30, "T4L": 1.35,
	}
	compass := map[string]float64{
		"T1C": 1.05,
		"T2C": 1.10,
		"T3C": 1.20, "T3R": 1.22,
		"T4C": 1.30, "T4R": 1.35, "T4E": 1.40, "T4L": 1.50,
	}
	gussett := map[string]float64{
		"T1C": 1.05,
		"T2C": 1.10, "T2E": 1.12,
		"T3C": 1.15, "T3R": 1.16,
		"T4C": 1.20, "T4E": 1.22, "T4L": 1.25,
	}
	totem := map[string]float64{
		"T1C": 2.0,
		"T2C": 3.0, "T2R": 8.0,
		"T3C": 20.0, "T3R": 40.0,
		"T4C": 50.0, "T4R": 100.0, "T4E": 150.0, "T4L": 200.0,
	}
	necklace := map[string]float64{
		"T1C": 1.1,
		"T2C": 1.25, "T2R": 1.35,
		"T3C": 1.5, "T3R": 1.6, "T3E": 1.75,
		"T4C": 2.0, "T4R": 2.25, "T4E": 2.5, "T4L": 3.0,
	}
	ankh := map[string]float64{
		"T1C": 1.1,
		"T2C": 1.25, "T2R": 1.28,
		"T3C": 1.5, "T3R": 1.75, "T3L": 2.0,
		"T4C": 2.0, "T4R": 2.25, "T4L": 2.5,
	}
	cube := map[string]float64{
		"T1C": 0.95,
		"T2C": 0.90, "T2E": 0.85,
		"T3C": 0.80, "T3R": 0.78,
		"T4C": 0.50, "T4R": 0.47, "T4E": 0.45, "T4L": 0.40,
	}
	artifactStonePercentLevels := []float64{1.02, 1.04, 1.05}
	artifactShellStonePercentLevels := []float64{1.05, 1.08, 1.1}
	artifactLunarStonePercentLevels := []float64{1.2, 1.3, 1.4}

	for _, artifact := range artifacts {
		spec := artifact.GetSpec()
		strType := ArtifactLevels[spec.GetLevel()] + ArtifactRarity[spec.GetRarity()]
		switch spec.GetName() {
		case ArtifactSpec_QUANTUM_METRONOME:
			artifactBuffs.ELR *= metronome[strType]
		case ArtifactSpec_INTERSTELLAR_COMPASS:
			artifactBuffs.SR *= compass[strType]
		case ArtifactSpec_ORNATE_GUSSET:
			artifactBuffs.Hab *= gussett[strType]
		case ArtifactSpec_THE_CHALICE:
			artifactBuffs.IHR *= chalice[strType]
		case ArtifactSpec_LUNAR_TOTEM:
			artifactBuffs.AwayEarnings *= totem[strType]
		case ArtifactSpec_TUNGSTEN_ANKH:
			artifactBuffs.Earnings *= ankh[strType]
		case ArtifactSpec_DEMETERS_NECKLACE:
			artifactBuffs.Earnings *= necklace[strType]
		case ArtifactSpec_PUZZLE_CUBE:
			artifactBuffs.ResearchDiscount *= cube[strType]

		default:
		}

		for _, stone := range artifact.GetStones() {
			switch stone.GetName() {
			case ArtifactSpec_TACHYON_STONE:
				artifactBuffs.ELR *= artifactStonePercentLevels[stone.GetLevel()]
			case ArtifactSpec_QUANTUM_STONE:
				artifactBuffs.SR *= artifactStonePercentLevels[stone.GetLevel()]
			case ArtifactSpec_LIFE_STONE:
				artifactBuffs.IHR *= artifactStonePercentLevels[stone.GetLevel()]
			case ArtifactSpec_LUNAR_STONE:
				artifactBuffs.AwayEarnings *= artifactLunarStonePercentLevels[stone.GetLevel()]
			case ArtifactSpec_SHELL_STONE:
				artifactBuffs.Earnings *= artifactShellStonePercentLevels[stone.GetLevel()]
			default:

			}
		}
	}
	return artifactBuffs
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
	"CARBON FIBER": {Type: "Collegg", Quality: "5%", ShipBuff: 1.05, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0, Dimension: GameModifier_SHIPPING_CAPACITY},
	"CHOCOLATE":    {Type: "Collegg", Quality: "3x", ShipBuff: 1.0, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0, Dimension: GameModifier_AWAY_EARNINGS},
	"EASTER":       {Type: "Collegg", Quality: "5%", ShipBuff: 1.0, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0, Dimension: GameModifier_INTERNAL_HATCHERY_RATE},
	"FIREWORK":     {Type: "Collegg", Quality: "5%", ShipBuff: 1.0, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0, Dimension: GameModifier_EARNINGS},
	"LITHIUM":      {Type: "Collegg", Quality: "10%", ShipBuff: 1.0, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0, Dimension: GameModifier_VEHICLE_COST},
	"PUMPKIN":      {Type: "Collegg", Quality: "5%", ShipBuff: 1.05, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0, Dimension: GameModifier_SHIPPING_CAPACITY},
	"WATERBALLOON": {Type: "Collegg", Quality: "95%", ShipBuff: 1.0, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0, Dimension: GameModifier_RESEARCH_COST},
	"P.E.G.G.":     {Type: "Collegg", Quality: "5%", ShipBuff: 1.0, LayBuff: 1.05, DeflBuff: 1.0, Stones: 0, Dimension: GameModifier_HAB_CAPACITY},
	"SILICON":      {Type: "Collegg", Quality: "5%", ShipBuff: 1.0, LayBuff: 1.05, DeflBuff: 1.0, Stones: 0, Dimension: GameModifier_EGG_LAYING_RATE},
}

var data *Store
var artifactConfig *ArtifactsConfigurationResponse

// Store holds the entire artifacts data
type Store struct {
	Schema           string    `json:"$schema"`
	ArtifactFamilies []*Family `json:"artifact_families"`
}

// Family holds the data for each artifact family
type Family struct {
	CoreFamily

	Effect       string  `json:"effect"`
	EffectTarget string  `json:"effect_target"`
	Tiers        []*Tier `json:"tiers"`
}

// CoreFamily holds the core data for each artifact family
type CoreFamily struct {
	ID          string              `json:"id"`
	AfxID       ArtifactSpec_Name   `json:"afx_id"`
	Name        string              `json:"name"`
	AfxType     ArtifactSpec_Type   `json:"afx_type"`
	Type        string              `json:"type"`
	SortKey     uint32              `json:"sort_key"`
	ChildAfxIds []ArtifactSpec_Name `json:"child_afx_ids"`
}

// Tier holds the data for each artifact tier
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

// CoreTier holds the core data for each artifact tier
type CoreTier struct {
	ItemIdentifiers
	TierNumber   int               `json:"tier_number"`
	TierName     string            `json:"tier_name"`
	AfxType      ArtifactSpec_Type `json:"afx_type"`
	Type         string            `json:"type"`
	IconFilename string            `json:"icon_filename"`
}

// ItemIdentifiers holds the identifiers for each artifact item
type ItemIdentifiers struct {
	ID       string             `json:"id"`
	AfxID    ArtifactSpec_Name  `json:"afx_id"`
	AfxLevel ArtifactSpec_Level `json:"afx_level"`
	Name     string             `json:"name"`
}

// Effect holds the data for each artifact effect
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

// Recipe holds the crafting recipe for each artifact
type Recipe struct {
	Ingredients   []Ingredient  `json:"ingredients"`
	CraftingPrice CraftingPrice `json:"crafting_price"`
}

// Ingredient holds the data for each crafting ingredient
type Ingredient struct {
	CoreTier
	Count uint32 `json:"count"`
}

// CraftingPrice holds the crafting price data
type CraftingPrice struct {
	Base    float64 `json:"base"`
	Low     float64 `json:"low"`
	Domain  uint32  `json:"domain"`
	Curve   float64 `json:"curve"`
	Initial uint32  `json:"initial"`
	Minimum uint32  `json:"minimum"`
}

// LoadArtifactsConfig loads artifact data from a JSON file
func LoadArtifactsConfig(configFile string) error {

	// Read the dataFile from disk into _eiafxConfigJSON
	fileContent, err := os.ReadFile(configFile)
	if err != nil {
		return errors.Wrap(err, "error reading configFile")
	}
	_eiafxConfigJSON := []byte(strings.ReplaceAll(string(fileContent), "./data.schema.json", "./ttbb-data/data.schema.json"))

	artifactConfig = &ArtifactsConfigurationResponse{}
	err = protojson.Unmarshal(_eiafxConfigJSON, artifactConfig)
	if err != nil {
		return errors.Wrap(err, "error unmarshalling eiafx-config.json")
	}
	return nil
}

// LoadArtifactsData loads artifact data from a JSON file
func LoadArtifactsData(dataFile string) error {

	// Read the dataFile from disk into _eiafxConfigJSON
	fileContent, err := os.ReadFile(dataFile)
	if err != nil {
		return errors.Wrap(err, "error reading dataFile")
	}
	_eiafxDataJSON := []byte(strings.ReplaceAll(string(fileContent), "./data.schema.json", "./ttbb-data/data.schema.json"))

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
		return "Hab Capacity"
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
