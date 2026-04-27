package ei

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/xhit/go-str2duration/v2"
)

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

const missionJSON = `{"ships":[
	{"id": "MissionInfo_CHICKEN_ONE", "name": "Chicken One","art":"chicken1","duration":["20m","1h","2h"]},
	{"id": "MissionInfo_CHICKEN_NINE", "name": "Chicken Nine","art":"chicken9","duration":["30m","1h","3h"]},
	{"id": "MissionInfo_CHICKEN_HEAVY", "name": "Chicken Heavy","art":"chickenheavy","duration":["45m","1h30m","4h"]},
	{"id": "MissionInfo_BCR", "name": "BCR","art":"bcr","duration":["1h30m","4h","8h"]},
	{"id": "MissionInfo_QUINTILLION_CHICKEN", "name": "Quintillion Chicken","art":"milleniumchicken","duration":["3h","6h","12h"]},
	{"id": "MissionInfo_CORNISH_HEN_CORVETTE", "name": "Cornish-Hen Corvette","art":"corellihencorvette","duration":["4h","12h","1d"]},
	{"id": "MissionInfo_GALEGGTICA", "name": "Galeggtica","art":"galeggtica","duration":["6h","16h","1d6h"]},
	{"id": "MissionInfo_DEFIHENT", "name": "Defihent","art":"defihent","duration":["8h","1d","2d"]},
	{"id": "MissionInfo_VOYEGGER", "name": "Voyegger","art":"voyegger","duration":["12h","1d12h","3d"]},
	{"id": "MissionInfo_HENERPRISE", "name": "Henerprise","art":"henerprise","duration":["1d","2d","4d"]},
	{"id": "MissionInfo_ATREGGIES_HENLINER", "name": "Atreggies Henliner","art":"atreggies","duration":["2d","3d","4d"]}
	]}`

// ShipData holds data for each mission ship
type ShipData struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Art      string   `json:"art"`
	ArtDev   string   `json:"artDev"`
	Duration []string `json:"duration"`
}

type missionData struct {
	Ships []ShipData `json:"ships"`
}

// MissionArt holds the mission art and durations loaded from JSON
var MissionArt missionData

func init() {
	_ = json.Unmarshal([]byte(missionJSON), &MissionArt)
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

		if ship < 0 || ship >= len(MissionArt.Ships) || durType < 0 || durType >= len(MissionArt.Ships[ship].Duration) {
			continue
		}

		durationStr := MissionArt.Ships[ship].Duration[durType]
		baseDur, err := str2duration.ParseDuration(durationStr)
		if err != nil {
			continue
		}
		baseSeconds := baseDur.Seconds() * epicResearchMult

		actualSeconds := mission.GetDurationSeconds()
		if actualSeconds <= 0 {
			continue
		}

		// Check for if there was a fast event (0.25) applied to the mission time. If the actual duration is less than 25% of the base duration, it's likely an anomaly.
		eventMult := FindFasterMissionEvent(time.Unix(int64(mission.GetStartTimeDerived()), 0))
		eventMultiplier := 1.0
		if eventMult.EventType == "mission-duration" {
			eventMultiplier = eventMult.Multiplier
		}

		minValidSeconds := baseSeconds * eventMultiplier

		if actualSeconds < minValidSeconds {
			suspects = append(suspects, fmt.Sprintf(
				"[%s] User: %s | Ship: %s | DurType: %d | BaseSec: %.0f | ActualSec: %.0f | ID: %s",
				time.Now().UTC().Format(time.RFC3339), userID, MissionArt.Ships[ship].Name, durType, baseSeconds, actualSeconds, mission.GetIdentifier(),
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
