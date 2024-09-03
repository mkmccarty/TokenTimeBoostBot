package ei

import (
	"fmt"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
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
	"SOUL":           {"egg_soul", "1279201265628348490", "1280382995282526208"},
	"PROPHECY":       {"egg_prophecy", "1279201195872878652", "1280382926470643754"},
}

// FindEggComponentEmoji will find the token emoji for the given guild
func FindEggComponentEmoji(eggOrig string) (string, EggEmojiData) {
	var eggIconString string

	var eggEmojiData EggEmojiData

	eggIcon, ok := EggEmojiMap[strings.ToUpper(eggOrig)]
	if config.DiscordAppID == "1187298713903829042" { // Dev
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
	if config.DiscordAppID == "1187298713903829042" { // Dev

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
	"Chocolate":    {Type: "Collegg", Quality: "100%", ShipBuff: 1.0, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0},
	"Easter":       {Type: "Collegg", Quality: "5%", ShipBuff: 1.0, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0},
	"Firework":     {Type: "Collegg", Quality: "5%", ShipBuff: 1.0, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0},
	"Pumpkin":      {Type: "Collegg", Quality: "5%", ShipBuff: 1.05, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0},
	"Waterballoon": {Type: "Collegg", Quality: "95%", ShipBuff: 1.0, LayBuff: 1.0, DeflBuff: 1.0, Stones: 0},
}
