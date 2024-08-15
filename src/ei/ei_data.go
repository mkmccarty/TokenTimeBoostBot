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
	ID   int64
}

// EggEmojiMap of egg emojis from the Egg Inc Discord
var EggEmojiMap = map[string]EggEmojiData{
	"EDIBLE":         {"egg_edible", 455467571613925418},
	"SUPERFOOD":      {"egg_superfood", 455468082635210752},
	"MEDICAL":        {"egg_medical", 455468241582817299},
	"ROCKET_FUEL":    {"egg_rocketfuel", 455468270661795850},
	"SUPER_MATERIAL": {"egg_supermaterial", 455468299480989696},
	"FUSION":         {"egg_fusion", 455468334859681803},
	"QUANTUM":        {"egg_quantum", 455468361099247617},
	"CRISPR":         {"egg_crispr", 1255673610845163641},
	"IMMORTALITY":    {"egg_crispr", 1255673610845163641},
	"TACHYON":        {"egg_tachyon", 455468421048696843},
	"GRAVITON":       {"egg_graviton", 455468444070969369},
	"DILITHIUM":      {"egg_dilithium", 455468464639967242},
	"PRODIGY":        {"egg_prodigy", 455468487641661461},
	"TERRAFORM":      {"egg_terraform", 455468509099458561},
	"ANTIMATTER":     {"egg_antimatter", 455468542171807744},
	"DARKMATTER":     {"egg_darkmatter", 455468555421483010},
	"AI":             {"egg_ai", 455468564590100490},
	"NEBULA":         {"egg_nebula", 455468583426981908},
	"UNIVERSE":       {"egg_universe", 567345439381389312},
	"ENLIGHTENMENT":  {"egg_enlightenment", 844620906248929341},
	"UNKNOWN":        {"egg_unknown", 455471603384582165},
	"WATERBALLOON":   {"egg_waterballoon", 460976773430116362},
	"FIREWORK":       {"egg_firework", 460976588893454337},
	"PUMPKIN":        {"egg_pumpkin", 503686019896573962},
	"CHOCOLATE":      {"egg_chocolate", 455470627663380480},
	"EASTER":         {"egg_easter", 455470644646379520},
	"CARBON-FIBER":   {"egg_carbonfiber", 1264977562720014470},
}

// FindEggComponentEmoji will find the token emoji for the given guild
func FindEggComponentEmoji(eggOrig string) (string, EggEmojiData) {
	var eggIconString string

	var eggEmojiData EggEmojiData

	eggIcon, ok := EggEmojiMap[strings.ToUpper(eggOrig)]
	if ok {
		eggEmojiData = eggIcon
		eggIconString = fmt.Sprintf("<:%s:%d>", eggEmojiData.Name, eggEmojiData.ID)
	} else {
		eggEmojiData = eggIcon
		eggIconString = fmt.Sprintf("<:%s:%d>", EggEmojiMap["UNKNOWN"].Name, EggEmojiMap["UNKNOWN"].ID)
	}
	return eggIconString, eggEmojiData
}
