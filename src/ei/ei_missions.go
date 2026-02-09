package ei

import (
	"encoding/json"
	"log"
	"os"
	"sort"
	"strconv"
	"time"
)

const missionJSON = `{"ships":[
	{"name": "Chicken One","art":"chicken1","duration":["20m","1h","2h"]},
	{"name": "Chicken Nine","art":"chicken9","duration":["30m","1h","3h"]},
	{"name": "Chicken Heavy","art":"chickenheavy","duration":["45m","1h30m","4h"]},
	{"name": "BCR","art":"bcr","duration":["1h30m","4h","8h"]},
	{"name": "Quintillion Chicken","art":"milleniumchicken","duration":["3h","6h","12h"]},
	{"name": "Cornish-Hen Corvette","art":"corellihencorvette","duration":["4h","12h","1d"]},
	{"name": "Galeggtica","art":"galeggtica","duration":["6h","16h","1d6h"]},
	{"name": "Defihent","art":"defihent","duration":["8h","1d","2d"]},
	{"name": "Voyegger","art":"voyegger","duration":["12h","1d12h","3d"]},
	{"name": "Henerprise","art":"henerprise","duration":["1d","2d","4d"]},
	{"name": "Atreggies Henliner","art":"atreggies","duration":["2d","3d","4d"]}
	]}`

// MissionConfigPath is the on-disk config used for mission and artifact data.
const MissionConfigPath = "ttbb-data/ei-afx-config.json"

// ShipData holds data for each mission ship
type ShipData struct {
	Name     string   `json:"Name"`
	Art      string   `json:"Art"`
	Duration []string `json:"Duration"`
}

type missionData struct {
	Ships []ShipData `json:"ships"`
}

type missionConfig struct {
	MissionParameters  []missionParameter  `json:"missionParameters"`
	ArtifactParameters []artifactParameter `json:"artifactParameters"`
	CraftingLevelInfos []craftingLevelInfo `json:"craftingLevelInfos"`
}

type craftingLevelInfo struct {
	XpRequired int     `json:"xpRequired"`
	RarityMult float64 `json:"rarityMult"`
}

type artifactParameter struct {
	Spec                artifactSpec `json:"spec"`
	BaseQuality         float64      `json:"baseQuality"`
	Value               float64      `json:"value"`
	OddsMultiplier      float64      `json:"oddsMultiplier"`
	CraftingPrice       float64      `json:"craftingPrice"`
	CraftingPriceLow    float64      `json:"craftingPriceLow"`
	CraftingPriceDomain float64      `json:"craftingPriceDomain"`
	CraftingPriceCurve  float64      `json:"craftingPriceCurve"`
	CraftingXp          float64      `json:"craftingXp"`
}

type artifactSpec struct {
	Name   string `json:"name"`
	Level  string `json:"level"`
	Rarity string `json:"rarity"`
}

type missionParameter struct {
	Ship      string            `json:"ship"`
	Durations []missionDuration `json:"durations"`
}

type missionDuration struct {
	DurationType string  `json:"durationType"`
	Seconds      int     `json:"seconds"`
	Quality      float64 `json:"quality"`
	MinQuality   float64 `json:"minQuality"`
	MaxQuality   float64 `json:"maxQuality"`
	Capacity     int     `json:"capacity"`
}

// MissionArt holds the mission art and durations loaded from JSON
var MissionArt missionData

// ArtifactParameters holds artifact parameters loaded from JSON
var ArtifactParameters []artifactParameter

// CraftingLevelInfos holds crafting level info loaded from JSON
var CraftingLevelInfos []craftingLevelInfo

var missionShipInfo = map[string]ShipData{
	"CHICKEN_ONE":         {Name: "Chicken One", Art: "chicken1"},
	"CHICKEN_NINE":        {Name: "Chicken Nine", Art: "chicken9"},
	"CHICKEN_HEAVY":       {Name: "Chicken Heavy", Art: "chickenheavy"},
	"BCR":                 {Name: "BCR", Art: "bcr"},
	"MILLENIUM_CHICKEN":   {Name: "Quintillion Chicken", Art: "milleniumchicken"},
	"CORELLIHEN_CORVETTE": {Name: "Cornish-Hen Corvette", Art: "corellihencorvette"},
	"GALEGGTICA":          {Name: "Galeggtica", Art: "galeggtica"},
	"CHICKFIANT":          {Name: "Defihent", Art: "defihent"},
	"VOYEGGER":            {Name: "Voyegger", Art: "voyegger"},
	"HENERPRISE":          {Name: "Henerprise", Art: "henerprise"},
	"ATREGGIES":           {Name: "Atreggies Henliner", Art: "atreggies"},
}

func init() {
	if !loadMissionDataFromConfig(MissionConfigPath) {
		_ = json.Unmarshal([]byte(missionJSON), &MissionArt)
	}
}

// ReloadMissionConfig reloads mission, artifact, and crafting data from disk.
func ReloadMissionConfig() bool {
	return loadMissionDataFromConfig(MissionConfigPath)
}

func loadMissionDataFromConfig(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("Mission config read failed: %v", err)
		return false
	}

	var cfg missionConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("Mission config parse failed: %v", err)
		return false
	}

	ArtifactParameters = cfg.ArtifactParameters
	CraftingLevelInfos = cfg.CraftingLevelInfos

	var md missionData
	for _, param := range cfg.MissionParameters {
		info, ok := missionShipInfo[param.Ship]
		if !ok {
			continue
		}
		info.Duration = pickMissionDurations(param.Durations)
		md.Ships = append(md.Ships, info)
	}

	if len(md.Ships) == 0 {
		return false
	}

	MissionArt = md
	return true
}

func pickMissionDurations(durations []missionDuration) []string {
	preferred := []string{"SHORT", "LONG", "EPIC"}
	byType := make(map[string]int, len(durations))
	for _, d := range durations {
		if d.Seconds <= 0 {
			continue
		}
		byType[d.DurationType] = d.Seconds
	}

	var result []string
	for _, key := range preferred {
		if seconds, ok := byType[key]; ok {
			result = append(result, formatMissionDuration(seconds))
		}
	}

	if len(result) > 0 {
		return result
	}

	sort.Slice(durations, func(i, j int) bool {
		return durations[i].Seconds < durations[j].Seconds
	})
	for _, d := range durations {
		if d.Seconds <= 0 {
			continue
		}
		result = append(result, formatMissionDuration(d.Seconds))
		if len(result) == 3 {
			break
		}
	}

	return result
}

func formatMissionDuration(seconds int) string {
	d := time.Duration(seconds) * time.Second
	if d <= 0 {
		return "0m"
	}

	days := d / (24 * time.Hour)
	d -= days * 24 * time.Hour
	hours := d / time.Hour
	d -= hours * time.Hour
	minutes := d / time.Minute

	parts := ""
	if days > 0 {
		parts += formatDurationPart(int(days), "d")
	}
	if hours > 0 {
		parts += formatDurationPart(int(hours), "h")
	}
	if minutes > 0 || parts == "" {
		parts += formatDurationPart(int(minutes), "m")
	}

	return parts
}

func formatDurationPart(value int, unit string) string {
	return strconv.Itoa(value) + unit
}
