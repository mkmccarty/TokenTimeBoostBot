package ei

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
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

// ShipData holds data for each mission ship
type ShipData struct {
	Name     string   `json:"Name"`
	Art      string   `json:"Art"`
	ArtDev   string   `json:"ArtDev"`
	Duration []string `json:"Duration"`
}

type missionData struct {
	Ships []ShipData `json:"ships"`
}

// AfxMissionParam holds mission parameter data from the AFX config.
type AfxMissionParam struct {
	Ship                     string               `json:"ship"`
	Durations                []AfxMissionDuration `json:"durations"`
	LevelMissionRequirements []float64            `json:"levelMissionRequirements"`
}

// AfxMissionDuration holds duration parameter data from the AFX config.
type AfxMissionDuration struct {
	DurationType      string  `json:"durationType"`
	Seconds           float64 `json:"seconds"`
	Quality           float64 `json:"quality"`
	MinQuality        float64 `json:"minQuality"`
	MaxQuality        float64 `json:"maxQuality"`
	Capacity          uint32  `json:"capacity"`
	LevelCapacityBump uint32  `json:"levelCapacityBump"`
	LevelQualityBump  float64 `json:"levelQualityBump"`
}

// AfxArtifactParam holds artifact parameter data from the AFX config.
type AfxArtifactParam struct {
	Spec struct {
		Name   string `json:"name"`
		Level  string `json:"level"`
		Rarity string `json:"rarity"`
	} `json:"spec"`
	BaseQuality         float64 `json:"baseQuality"`
	Value               float64 `json:"value"`
	CraftingPrice       float64 `json:"craftingPrice"`
	CraftingPriceLow    float64 `json:"craftingPriceLow"`
	CraftingPriceDomain uint32  `json:"craftingPriceDomain"`
	CraftingPriceCurve  float64 `json:"craftingPriceCurve"`
	CraftingXp          uint64  `json:"craftingXp"`
}

// AfxCraftingLevel holds crafting level data from the AFX config.
type AfxCraftingLevel struct {
	XpRequired float64 `json:"xpRequired"`
	RarityMult float32 `json:"rarityMult"`
}

// AfxConfigData holds the entire AFX config parsed.
type AfxConfigData struct {
	MissionParameters  []AfxMissionParam  `json:"missionParameters"`
	ArtifactParameters []AfxArtifactParam `json:"artifactParameters"`
	CraftingLevelInfos []AfxCraftingLevel `json:"craftingLevelInfos"`
}

// GetCraftingLevel calculates the crafting level based on the provided total crafting XP.
func GetCraftingLevel(xp float64) int {
	level := 1
	cumulativeXP := 0.0
	for _, info := range AfxConfig.CraftingLevelInfos {
		if info.XpRequired <= 1 {
			// Ignore dummy max level entries.
			break
		}
		cumulativeXP += info.XpRequired
		if xp >= cumulativeXP {
			level++
		} else {
			break
		}
	}
	return level
}

// MissionArt holds the mission art and durations loaded from JSON
var MissionArt missionData

// AfxConfig holds the AFX configuration data loaded from JSON
var AfxConfig AfxConfigData

// MissionDurations maps ship and duration type to expected duration in seconds
var MissionDurations = make(map[int]map[int]float64)

// MissionDurationParams maps spaceship and duration type to its AfxMissionDuration config
var MissionDurationParams = make(map[MissionInfo_Spaceship]map[MissionInfo_DurationType]AfxMissionDuration)

// MissionLevelReqs maps spaceship to its slice of required launch points per level
var MissionLevelReqs = make(map[MissionInfo_Spaceship][]float64)

// SuspectMissionHandler is a callback to record suspect missions to the database without importing farmerstate
var SuspectMissionHandler func(discordID string, mission *MissionInfo, baseSeconds, actualSeconds, eventMultiplier float64)

func loadAfxConfig() {
	const url = "https://raw.githubusercontent.com/carpetsage/egg/main/wasmegg/_common/eiafx/eiafx-config.json"
	const filename = "ttbb-data/ei-afx-config.json"

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		log.Printf("Downloading %s...", filename)
		resp, err := http.Get(url)
		if err == nil {
			defer func() {
				if cerr := resp.Body.Close(); cerr != nil {
					log.Printf("Failed to close suspect mission log: %v", cerr)
				}
			}()
			body, _ := io.ReadAll(resp.Body)
			_ = os.MkdirAll("ttbb-data", 0755)
			_ = os.WriteFile(filename, body, 0644)
		} else {
			log.Printf("Failed to download afx config: %v", err)
		}
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return
	}
	err = json.Unmarshal(data, &AfxConfig)
	if err != nil {
		log.Printf("Failed to unmarshal afx config: %v", err)
		return
	}

	for _, mp := range AfxConfig.MissionParameters {
		shipInt := -1
		for k, v := range MissionInfo_Spaceship_name {
			if v == mp.Ship {
				shipInt = int(k)
				break
			}
		}
		if shipInt == -1 {
			continue
		}

		shipEnum := MissionInfo_Spaceship(shipInt)
		if MissionDurations[shipInt] == nil {
			MissionDurations[shipInt] = make(map[int]float64)
		}
		if MissionDurationParams[shipEnum] == nil {
			MissionDurationParams[shipEnum] = make(map[MissionInfo_DurationType]AfxMissionDuration)
		}
		MissionLevelReqs[shipEnum] = mp.LevelMissionRequirements

		for _, d := range mp.Durations {
			durInt := -1
			for k, v := range MissionInfo_DurationType_name {
				if v == d.DurationType {
					durInt = int(k)
					break
				}
			}
			if durInt != -1 {
				durEnum := MissionInfo_DurationType(durInt)
				MissionDurations[shipInt][durInt] = d.Seconds
				MissionDurationParams[shipEnum][durEnum] = d
			}
		}
	}
}

// FuelRequirement represents a fuel requirement for a mission.
type FuelRequirement struct {
	Egg    Egg
	Amount float64
}

// StandardMissionFuels maps ship and duration type to standard egg fuels.
var StandardMissionFuels = map[MissionInfo_Spaceship]map[MissionInfo_DurationType][]FuelRequirement{
	MissionInfo_CHICKEN_ONE: {
		MissionInfo_TUTORIAL: {{Egg: Egg_ROCKET_FUEL, Amount: 1e5}},
		MissionInfo_SHORT:    {{Egg: Egg_ROCKET_FUEL, Amount: 2e6}},
		MissionInfo_LONG:     {{Egg: Egg_ROCKET_FUEL, Amount: 3e6}},
		MissionInfo_EPIC:     {{Egg: Egg_ROCKET_FUEL, Amount: 10e6}},
	},
	MissionInfo_CHICKEN_NINE: {
		MissionInfo_SHORT: {{Egg: Egg_ROCKET_FUEL, Amount: 10e6}},
		MissionInfo_LONG:  {{Egg: Egg_ROCKET_FUEL, Amount: 15e6}},
		MissionInfo_EPIC:  {{Egg: Egg_ROCKET_FUEL, Amount: 25e6}},
	},
	MissionInfo_CHICKEN_HEAVY: {
		MissionInfo_SHORT: {{Egg: Egg_ROCKET_FUEL, Amount: 100e6}},
		MissionInfo_LONG:  {{Egg: Egg_ROCKET_FUEL, Amount: 50e6}, {Egg: Egg_FUSION, Amount: 5e6}},
		MissionInfo_EPIC:  {{Egg: Egg_ROCKET_FUEL, Amount: 75e6}, {Egg: Egg_FUSION, Amount: 25e6}},
	},
	MissionInfo_BCR: {
		MissionInfo_SHORT: {{Egg: Egg_ROCKET_FUEL, Amount: 250e6}, {Egg: Egg_FUSION, Amount: 50e6}},
		MissionInfo_LONG:  {{Egg: Egg_ROCKET_FUEL, Amount: 400e6}, {Egg: Egg_FUSION, Amount: 75e6}},
		MissionInfo_EPIC:  {{Egg: Egg_SUPERFOOD, Amount: 5e6}, {Egg: Egg_ROCKET_FUEL, Amount: 300e6}, {Egg: Egg_FUSION, Amount: 100e6}},
	},
	MissionInfo_MILLENIUM_CHICKEN: {
		MissionInfo_SHORT: {{Egg: Egg_FUSION, Amount: 5e9}, {Egg: Egg_GRAVITON, Amount: 1e9}},
		MissionInfo_LONG:  {{Egg: Egg_FUSION, Amount: 7e9}, {Egg: Egg_GRAVITON, Amount: 5e9}},
		MissionInfo_EPIC:  {{Egg: Egg_SUPERFOOD, Amount: 10e6}, {Egg: Egg_FUSION, Amount: 10e9}, {Egg: Egg_GRAVITON, Amount: 15e9}},
	},
	MissionInfo_CORELLIHEN_CORVETTE: {
		MissionInfo_SHORT: {{Egg: Egg_FUSION, Amount: 15e9}, {Egg: Egg_GRAVITON, Amount: 2e9}},
		MissionInfo_LONG:  {{Egg: Egg_FUSION, Amount: 20e9}, {Egg: Egg_GRAVITON, Amount: 3e9}},
		MissionInfo_EPIC:  {{Egg: Egg_SUPERFOOD, Amount: 500e6}, {Egg: Egg_FUSION, Amount: 25e9}, {Egg: Egg_GRAVITON, Amount: 5e9}},
	},
	MissionInfo_GALEGGTICA: {
		MissionInfo_SHORT: {{Egg: Egg_FUSION, Amount: 50e9}, {Egg: Egg_GRAVITON, Amount: 10e9}},
		MissionInfo_LONG:  {{Egg: Egg_FUSION, Amount: 75e9}, {Egg: Egg_GRAVITON, Amount: 25e9}},
		MissionInfo_EPIC:  {{Egg: Egg_FUSION, Amount: 100e9}, {Egg: Egg_GRAVITON, Amount: 50e9}, {Egg: Egg_ANTIMATTER, Amount: 1e9}},
	},
	MissionInfo_CHICKFIANT: {
		MissionInfo_SHORT: {{Egg: Egg_DILITHIUM, Amount: 200e9}, {Egg: Egg_ANTIMATTER, Amount: 50e9}},
		MissionInfo_LONG:  {{Egg: Egg_DILITHIUM, Amount: 250e9}, {Egg: Egg_ANTIMATTER, Amount: 150e9}},
		MissionInfo_EPIC:  {{Egg: Egg_TACHYON, Amount: 25e9}, {Egg: Egg_DILITHIUM, Amount: 250e9}, {Egg: Egg_ANTIMATTER, Amount: 250e9}},
	},
	MissionInfo_VOYEGGER: {
		MissionInfo_SHORT: {{Egg: Egg_DILITHIUM, Amount: 1e12}, {Egg: Egg_ANTIMATTER, Amount: 1e12}},
		MissionInfo_LONG:  {{Egg: Egg_DILITHIUM, Amount: 1.5e12}, {Egg: Egg_ANTIMATTER, Amount: 1.5e12}},
		MissionInfo_EPIC:  {{Egg: Egg_TACHYON, Amount: 100e9}, {Egg: Egg_DILITHIUM, Amount: 2e12}, {Egg: Egg_ANTIMATTER, Amount: 2e12}},
	},
	MissionInfo_HENERPRISE: {
		MissionInfo_SHORT: {{Egg: Egg_DILITHIUM, Amount: 2e12}, {Egg: Egg_ANTIMATTER, Amount: 2e12}},
		MissionInfo_LONG:  {{Egg: Egg_DILITHIUM, Amount: 3e12}, {Egg: Egg_ANTIMATTER, Amount: 3e12}, {Egg: Egg_DARK_MATTER, Amount: 3e12}},
		MissionInfo_EPIC:  {{Egg: Egg_TACHYON, Amount: 1e12}, {Egg: Egg_DILITHIUM, Amount: 3e12}, {Egg: Egg_ANTIMATTER, Amount: 3e12}, {Egg: Egg_DARK_MATTER, Amount: 3e12}},
	},
	MissionInfo_ATREGGIES: {
		MissionInfo_SHORT: {{Egg: Egg_DILITHIUM, Amount: 4e12}, {Egg: Egg_ANTIMATTER, Amount: 4e12}, {Egg: Egg_DARK_MATTER, Amount: 3e12}},
		MissionInfo_LONG:  {{Egg: Egg_DILITHIUM, Amount: 6e12}, {Egg: Egg_ANTIMATTER, Amount: 6e12}, {Egg: Egg_DARK_MATTER, Amount: 4e12}},
		MissionInfo_EPIC:  {{Egg: Egg_TACHYON, Amount: 2e12}, {Egg: Egg_DILITHIUM, Amount: 6e12}, {Egg: Egg_ANTIMATTER, Amount: 6e12}, {Egg: Egg_DARK_MATTER, Amount: 6e12}},
	},
}

// VirtueMissionFuels maps ship and duration type to virtue egg fuels.
var VirtueMissionFuels = map[MissionInfo_Spaceship]map[MissionInfo_DurationType][]FuelRequirement{
	MissionInfo_CHICKEN_ONE: {
		MissionInfo_SHORT: {{Egg: Egg_HUMILITY, Amount: 5e6}},
		MissionInfo_LONG:  {{Egg: Egg_HUMILITY, Amount: 10e6}},
		MissionInfo_EPIC:  {{Egg: Egg_HUMILITY, Amount: 20e6}},
	},
	MissionInfo_CHICKEN_NINE: {
		MissionInfo_SHORT: {{Egg: Egg_HUMILITY, Amount: 10e6}},
		MissionInfo_LONG:  {{Egg: Egg_HUMILITY, Amount: 20e6}},
		MissionInfo_EPIC:  {{Egg: Egg_HUMILITY, Amount: 50e6}},
	},
	MissionInfo_CHICKEN_HEAVY: {
		MissionInfo_SHORT: {{Egg: Egg_HUMILITY, Amount: 50e6}},
		MissionInfo_LONG:  {{Egg: Egg_HUMILITY, Amount: 100e6}},
		MissionInfo_EPIC:  {{Egg: Egg_HUMILITY, Amount: 150e6}},
	},
	MissionInfo_BCR: {
		MissionInfo_SHORT: {{Egg: Egg_HUMILITY, Amount: 100e6}, {Egg: Egg_INTEGRITY, Amount: 10e6}},
		MissionInfo_LONG:  {{Egg: Egg_HUMILITY, Amount: 150e6}, {Egg: Egg_INTEGRITY, Amount: 20e6}},
		MissionInfo_EPIC:  {{Egg: Egg_HUMILITY, Amount: 200e6}, {Egg: Egg_INTEGRITY, Amount: 30e6}},
	},
	MissionInfo_MILLENIUM_CHICKEN: {
		MissionInfo_SHORT: {{Egg: Egg_HUMILITY, Amount: 10e9}, {Egg: Egg_INTEGRITY, Amount: 10e9}},
		MissionInfo_LONG:  {{Egg: Egg_HUMILITY, Amount: 20e9}, {Egg: Egg_INTEGRITY, Amount: 20e9}},
		MissionInfo_EPIC:  {{Egg: Egg_HUMILITY, Amount: 50e9}, {Egg: Egg_INTEGRITY, Amount: 50e9}},
	},
	MissionInfo_CORELLIHEN_CORVETTE: {
		MissionInfo_SHORT: {{Egg: Egg_HUMILITY, Amount: 20e9}, {Egg: Egg_INTEGRITY, Amount: 5e9}},
		MissionInfo_LONG:  {{Egg: Egg_HUMILITY, Amount: 40e9}, {Egg: Egg_INTEGRITY, Amount: 8e9}},
		MissionInfo_EPIC:  {{Egg: Egg_HUMILITY, Amount: 70e9}, {Egg: Egg_INTEGRITY, Amount: 10e9}},
	},
	MissionInfo_GALEGGTICA: {
		MissionInfo_SHORT: {{Egg: Egg_HUMILITY, Amount: 200e9}, {Egg: Egg_INTEGRITY, Amount: 200e9}, {Egg: Egg_CURIOSITY, Amount: 200e9}},
		MissionInfo_LONG:  {{Egg: Egg_HUMILITY, Amount: 400e9}, {Egg: Egg_INTEGRITY, Amount: 400e9}, {Egg: Egg_CURIOSITY, Amount: 400e9}},
		MissionInfo_EPIC:  {{Egg: Egg_HUMILITY, Amount: 600e9}, {Egg: Egg_INTEGRITY, Amount: 600e9}, {Egg: Egg_CURIOSITY, Amount: 600e9}},
	},
	MissionInfo_CHICKFIANT: {
		MissionInfo_SHORT: {{Egg: Egg_HUMILITY, Amount: 1e12}, {Egg: Egg_CURIOSITY, Amount: 1e12}, {Egg: Egg_KINDNESS, Amount: 1e12}},
		MissionInfo_LONG:  {{Egg: Egg_HUMILITY, Amount: 2e12}, {Egg: Egg_CURIOSITY, Amount: 2e12}, {Egg: Egg_KINDNESS, Amount: 2e12}},
		MissionInfo_EPIC:  {{Egg: Egg_HUMILITY, Amount: 3e12}, {Egg: Egg_CURIOSITY, Amount: 3e12}, {Egg: Egg_KINDNESS, Amount: 3e12}},
	},
	MissionInfo_VOYEGGER: {
		MissionInfo_SHORT: {{Egg: Egg_HUMILITY, Amount: 5e12}, {Egg: Egg_CURIOSITY, Amount: 10e12}, {Egg: Egg_KINDNESS, Amount: 5e12}},
		MissionInfo_LONG:  {{Egg: Egg_HUMILITY, Amount: 10e12}, {Egg: Egg_CURIOSITY, Amount: 20e12}, {Egg: Egg_KINDNESS, Amount: 10e12}},
		MissionInfo_EPIC:  {{Egg: Egg_HUMILITY, Amount: 15e12}, {Egg: Egg_CURIOSITY, Amount: 25e12}, {Egg: Egg_KINDNESS, Amount: 15e12}},
	},
	MissionInfo_HENERPRISE: {
		MissionInfo_SHORT: {{Egg: Egg_HUMILITY, Amount: 10e12}, {Egg: Egg_CURIOSITY, Amount: 15e12}, {Egg: Egg_KINDNESS, Amount: 10e12}},
		MissionInfo_LONG:  {{Egg: Egg_HUMILITY, Amount: 15e12}, {Egg: Egg_CURIOSITY, Amount: 20e12}, {Egg: Egg_KINDNESS, Amount: 15e12}, {Egg: Egg_RESILIENCE, Amount: 10e12}},
		MissionInfo_EPIC:  {{Egg: Egg_HUMILITY, Amount: 25e12}, {Egg: Egg_CURIOSITY, Amount: 25e12}, {Egg: Egg_KINDNESS, Amount: 25e12}, {Egg: Egg_RESILIENCE, Amount: 20e12}},
	},
	MissionInfo_ATREGGIES: {
		MissionInfo_SHORT: {{Egg: Egg_HUMILITY, Amount: 20e12}, {Egg: Egg_CURIOSITY, Amount: 25e12}, {Egg: Egg_KINDNESS, Amount: 20e12}},
		MissionInfo_LONG:  {{Egg: Egg_HUMILITY, Amount: 30e12}, {Egg: Egg_CURIOSITY, Amount: 40e12}, {Egg: Egg_KINDNESS, Amount: 30e12}, {Egg: Egg_RESILIENCE, Amount: 20e12}},
		MissionInfo_EPIC:  {{Egg: Egg_HUMILITY, Amount: 75e12}, {Egg: Egg_CURIOSITY, Amount: 50e12}, {Egg: Egg_KINDNESS, Amount: 75e12}, {Egg: Egg_RESILIENCE, Amount: 40e12}},
	},
}

// GetMissionFuels returns the fuel requirements for a ship and duration type.
func GetMissionFuels(ship MissionInfo_Spaceship, dt MissionInfo_DurationType, virtue bool) []FuelRequirement {
	if virtue {
		if m, ok := VirtueMissionFuels[ship]; ok {
			return m[dt]
		}
		return nil
	}
	if m, ok := StandardMissionFuels[ship]; ok {
		return m[dt]
	}
	return nil
}

// GetShipMissionParams returns duration and quality parameters from afx config.
func GetShipMissionParams(ship MissionInfo_Spaceship, dt MissionInfo_DurationType) (AfxMissionDuration, bool) {
	if m, ok := MissionDurationParams[ship]; ok {
		if p, ok2 := m[dt]; ok2 {
			return p, true
		}
	}
	return AfxMissionDuration{}, false
}

// GetShipLevelRequirements returns the launch points requirements per level for a ship.
func GetShipLevelRequirements(ship MissionInfo_Spaceship) []float64 {
	return MissionLevelReqs[ship]
}

// CalculateShipLevel calculates stars (level), points into current level, points needed to next level, and max level.
func CalculateShipLevel(ship MissionInfo_Spaceship, totalLP float64) (level int, curLP float64, neededLP float64, maxLevel int) {
	reqs := GetShipLevelRequirements(ship)
	maxLevel = len(reqs)
	accum := 0.0
	for i, req := range reqs {
		if totalLP >= accum+req {
			level = i + 1
			accum += req
		} else {
			curLP = totalLP - accum
			neededLP = req - curLP
			return level, curLP, neededLP, maxLevel
		}
	}
	curLP = totalLP - accum
	neededLP = 0
	return level, curLP, neededLP, maxLevel
}

func init() {
	_ = json.Unmarshal([]byte(missionJSON), &MissionArt)
	loadAfxConfig()
}

// GetEpicResearchMissionCapacity calculates the mission capacity multiplier from the epic research items.
func GetEpicResearchMissionCapacity(epicResearch []*Backup_ResearchItem) float64 {
	missionCapacity := 1.0

	ids := []string{
		"afx_mission_capacity",
	}
	result := GetResearchGeneric(epicResearch, ids, missionCapacity)
	return result
}

// GetEpicResearchMissionTime calculates the mission time multiplier from the epic research items.
func GetEpicResearchMissionTime(epicResearch []*Backup_ResearchItem) float64 {
	missionTime := 1.0

	ids := []string{
		"afx_mission_time",
	}
	result := GetResearchGeneric(epicResearch, ids, missionTime)
	return result
}

// MissionValidation evaluates mission data from a Backup to detect duration anomalies.
// It logs suspect missions to a log file.
func MissionValidation(backup *Backup, discordID string) {
	if backup == nil || backup.GetArtifactsDb() == nil {
		return
	}

	db := backup.GetArtifactsDb()
	var allMissions []*MissionInfo
	allMissions = append(allMissions, db.GetMissionArchive()...)

	var suspects []string

	epicResearchMult := 1.0
	if backup.GetGame() != nil {
		epicResearchMult = GetEpicResearchMissionTime(backup.GetGame().GetEpicResearch())
	}

	for _, mission := range allMissions {
		ship := int(mission.GetShip())
		durType := int(mission.GetDurationType())

		// Skip tutorial missions
		if durType == 3 {
			continue
		}

		shipDurs, ok := MissionDurations[ship]
		if !ok {
			continue
		}
		baseSeconds, ok := shipDurs[durType]
		if !ok {
			continue
		}

		actualSeconds := math.Round(mission.GetDurationSeconds())
		if actualSeconds <= 0 {
			continue
		}

		//baseSeconds *= epicResearchMult

		// Check for if there was a fast event (0.25) applied to the mission time. If the actual duration is less than 25% of the base duration, it's likely an anomaly.
		eventMult := FindFasterMissionEvent(time.Unix(int64(mission.GetStartTimeDerived()), 0))
		eventMultiplier := 1.0
		if eventMult.EventType == "mission-duration" {
			eventMultiplier = eventMult.Multiplier
		}

		/*
			// If this wasn't a valid event multiplier then we need to figure out our own minimum valid seconds based on the base seconds and the event multiplier. If the actual seconds is less than the minimum valid seconds, then we log it as a suspect mission.
			calculatedEventMultiplier := float64(baseSeconds) / float64(actualSeconds)
			if eventMultiplier != calculatedEventMultiplier {
				eventMultiplier = 1.0 / calculatedEventMultiplier
			}
		*/

		// Bypass anomaly: allow a 10x mission speedup for a specific ship type and date range
		missionTime := time.Unix(int64(mission.GetStartTimeDerived()), 0)
		targetShip := int(MissionInfo_ATREGGIES)
		startDate := time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2024, time.September, 24, 0, 0, 0, 0, time.UTC)
		if ship == targetShip && missionTime.After(startDate) && missionTime.Before(endDate) {
			eventMultiplier *= 0.1
		}

		if actualSeconds < (math.Round(baseSeconds*eventMultiplier*epicResearchMult) - 1.0) {
			shipName := ""
			if ship >= 0 && ship < len(MissionArt.Ships) {
				shipName = MissionArt.Ships[ship].Name
			} else {
				shipName = fmt.Sprintf("Ship%d", ship)
			}

			suspects = append(suspects, fmt.Sprintf(
				"Date: %s | Ship: %s | DurType: %d | BaseSec: %.0f | ActualSec: %.0f | EventMult: %.2f",
				time.Unix(int64(mission.GetStartTimeDerived()), 0).UTC().Format("2006-01-02"),
				shipName, durType, baseSeconds, actualSeconds, eventMultiplier,
			))

			if discordID != "" && SuspectMissionHandler != nil {
				SuspectMissionHandler(discordID, mission, baseSeconds, actualSeconds, eventMultiplier)
			}
		}
	}

	if len(suspects) > 0 {
		logSuspectMissions(suspects)
	}
}

func logSuspectMissions(suspects []string) {
	logDir := "ttbb-data"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("Failed to create log directory: %v", err)
		return
	}

	logPath := filepath.Join(logDir, "suspect_missions.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Failed to open suspect mission log: %v", err)
		return
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			log.Printf("Failed to close suspect mission log: %v", cerr)
		}
	}()

	for _, s := range suspects {
		if _, err := f.WriteString(s + "\n"); err != nil {
			log.Printf("Failed to write to suspect mission log: %v", err)
		}
	}
}
