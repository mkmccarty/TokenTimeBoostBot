package ei

import (
	"fmt"
	"strings"
	"time"
)

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
}

// EggIncContracts holds a list of all contracts, newest is last
var EggIncContracts []EggIncContract

// EggIncContractsAll holds a list of all contracts, newest is last
var EggIncContractsAll map[string]EggIncContract

func init() {
	EggIncContractsAll = make(map[string]EggIncContract)
}

// EggEmojiData is a struct to hold the name and ID of an egg emoji
type EggEmojiData struct {
	Name string
	ID   string
}

// EggEmojiMap of egg emojis from the Egg Inc Discord
var EggEmojiMap = map[string]EggEmojiData{
	"EDIBLE":         {"egg_edible", "455467571613925418"},
	"SUPERFOOD":      {"egg_superfood", "455468082635210752"},
	"MEDICAL":        {"egg_medical", "455468241582817299"},
	"ROCKET_FUEL":    {"egg_rocketfuel", "455468270661795850"},
	"SUPER_MATERIAL": {"egg_supermaterial", "455468299480989696"},
	"FUSION":         {"egg_fusion", "455468334859681803"},
	"QUANTUM":        {"egg_quantum", "455468361099247617"},
	"CRISPR":         {"egg_crispr", "1255673610845163641"},
	"IMMORTALITY":    {"egg_crispr", "1255673610845163641"},
	"TACHYON":        {"egg_tachyon", "455468421048696843"},
	"GRAVITON":       {"egg_graviton", "455468444070969369"},
	"DILITHIUM":      {"egg_dilithium", "455468464639967242"},
	"PRODIGY":        {"egg_prodigy", "455468487641661461"},
	"TERRAFORM":      {"egg_terraform", "455468509099458561"},
	"ANTIMATTER":     {"egg_antimatter", "455468542171807744"},
	"DARKMATTER":     {"egg_darkmatter", "455468555421483010"},
	"AI":             {"egg_ai", "455468564590100490"},
	"NEBULA":         {"egg_nebula", "455468583426981908"},
	"UNIVERSE":       {"egg_universe", "567345439381389312"},
	"ENLIGHTENMENT":  {"egg_enlightenment", "844620906248929341"},
	"UNKNOWN":        {"egg_unknown", "455471603384582165"},
	"WATERBALLOON":   {"egg_waterballoon", "460976773430116362"},
	"FIREWORK":       {"egg_firework", "460976588893454337"},
	"PUMPKIN":        {"egg_pumpkin", "503686019896573962"},
	"CHOCOLATE":      {"egg_chocolate", "455470627663380480"},
	"EASTER":         {"egg_easter", "455470644646379520"},
	"CARBON-FIBER":   {"egg_carbonfiber", "1264977562720014470"},
}

// FindEggComponentEmoji will find the token emoji for the given guild
func FindEggComponentEmoji(eggOrig string) (string, EggEmojiData) {
	var eggIconString string

	var eggEmojiData EggEmojiData

	eggIcon, ok := EggEmojiMap[strings.ToUpper(eggOrig)]
	if ok {
		eggEmojiData = eggIcon
		eggIconString = fmt.Sprintf("<:%s:%s>", eggEmojiData.Name, eggEmojiData.ID)
	} else {
		eggEmojiData = eggIcon
		eggIconString = fmt.Sprintf("<:%s:%s>", EggEmojiMap["UNKNOWN"].Name, EggEmojiMap["UNKNOWN"].ID)
	}
	return eggIconString, eggEmojiData
}

// Artifact holds the data for each artifact
type Artifact struct {
	Type     string
	Quality  string
	ShipBuff float64
	LayBuff  float64
	DeflBuff float64
	Stones   int
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
	"CarbonFiber":  {Type: "Collegg", Quality: "5%", ShipBuff: 1.05, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0},
	"Pumpkin":      {Type: "Collegg", Quality: "5%", ShipBuff: 1.05, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0},
	"Firework":     {Type: "Collegg", Quality: "5%", ShipBuff: 1.00, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0},
	"Waterballoon": {Type: "Collegg", Quality: "95%", ShipBuff: 1.00, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0},
}
