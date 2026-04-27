package ei

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	Ship      string               `json:"ship"`
	Durations []AfxMissionDuration `json:"durations"`
}

// AfxMissionDuration holds duration parameter data from the AFX config.
type AfxMissionDuration struct {
	DurationType string  `json:"durationType"`
	Seconds      float64 `json:"seconds"`
	Capacity     uint32  `json:"capacity"`
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

// MissionArt holds the mission art and durations loaded from JSON
var MissionArt missionData

// AfxConfig holds the AFX configuration data loaded from JSON
var AfxConfig AfxConfigData

// MissionDurations maps ship and duration type to expected duration in seconds
var MissionDurations = make(map[int]map[int]float64)

func loadAfxConfig() {
	const url = "https://raw.githubusercontent.com/carpetsage/egg/main/wasmegg/_common/eiafx/eiafx-config.json"
	const filename = "ttbb-data/ei-afx-config.json"

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		log.Printf("Downloading %s...", filename)
		resp, err := http.Get(url)
		if err == nil {
			defer resp.Body.Close()
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

		if MissionDurations[shipInt] == nil {
			MissionDurations[shipInt] = make(map[int]float64)
		}
		for _, d := range mp.Durations {
			durInt := -1
			for k, v := range MissionInfo_DurationType_name {
				if v == d.DurationType {
					durInt = int(k)
					break
				}
			}
			if durInt != -1 {
				MissionDurations[shipInt][durInt] = d.Seconds
			}
		}
	}
}

func init() {
	_ = json.Unmarshal([]byte(missionJSON), &MissionArt)
	loadAfxConfig()
}

// MissionValidation evaluates mission data from a Backup to detect duration anomalies.
// It logs suspect missions to a log file.
func MissionValidation(backup *Backup) {
	if backup == nil || backup.GetArtifactsDb() == nil {
		return
	}

	userID := backup.GetEiUserId()
	if userID == "" {
		userID = "UNKNOWN"
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

		actualSeconds := mission.GetDurationSeconds()
		if actualSeconds <= 0 {
			continue
		}

		baseSeconds *= epicResearchMult

		// Check for if there was a fast event (0.25) applied to the mission time. If the actual duration is less than 25% of the base duration, it's likely an anomaly.
		eventMult := FindFasterMissionEvent(time.Unix(int64(mission.GetStartTimeDerived()), 0))
		eventMultiplier := 1.0
		if eventMult.EventType == "mission-duration" {
			eventMultiplier = eventMult.Multiplier
		}

		// If this wasn't a valid event multiplier then we need to figure out our own minimum valid seconds based on the base seconds and the event multiplier. If the actual seconds is less than the minimum valid seconds, then we log it as a suspect mission.
		calculatedEventMultiplier := float64(baseSeconds) / float64(actualSeconds)
		if eventMultiplier != calculatedEventMultiplier {
			eventMultiplier = calculatedEventMultiplier
		}

		minValidSeconds := baseSeconds * eventMultiplier

		if actualSeconds < minValidSeconds {
			shipName := ""
			if ship >= 0 && ship < len(MissionArt.Ships) {
				shipName = MissionArt.Ships[ship].Name
			} else {
				shipName = fmt.Sprintf("Ship%d", ship)
			}

			suspects = append(suspects, fmt.Sprintf(
				"[%s] User: %s | Ship: %s | DurType: %d | BaseSec: %.0f | ActualSec: %.0f | ID: %s",
				time.Now().UTC().Format(time.RFC3339), userID, shipName, durType, baseSeconds, actualSeconds, mission.GetIdentifier(),
			))
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
