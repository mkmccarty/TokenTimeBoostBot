package launch

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strings"
	"time"
)

var integerZeroMinValue float64 = 0.0

type shipData struct {
	Name     string   `json:"Name"`
	Art      string   `json:"Art"`
	Duration []string `json:"Duration"`
}

type missionData struct {
	Ships []shipData
}

const missionJSON = `{"ships":[
	{"name": "Atreggies Henliner","art":"","duration":["2d","3d","4d"]},
	{"name": "Henerprise","art":"","duration":["1d","2d","4d"]},
	{"name": "Voyegger","art":"","duration":["12h","1d12h","3d"]},
	{"name": "Defihent","art":"","duration":["8h","1d","2d"]},
	{"name": "Galeggtica","art":"","duration":["6h","16h","1d6h"]},
	{"name": "Cornish-Hen Corvette","art":"","duration":["4h","12h","1d"]},
	{"name": "Quintillion Chicken","art":"","duration":["3h","6h","12h"]},
	{"name": "BCR","art":"","duration":["1h30m","4h","8h"]},
	{"name": "Chicken Heavy","art":"","duration":["45m","1h30m","4h"]},
	{"name": "Chicken Nine","art":"","duration":["30m","1h","3h"]},
	{"name": "Chicken One","art":"","duration":["20m","1h","2h"]}
	]}`

var mis missionData

func init() {
	json.Unmarshal([]byte(missionJSON), &mis)
}

func fmtDuration(d time.Duration) string {
	str := ""
	d = d.Round(time.Minute)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d = h / 24
	h -= d * 24

	if d > 0 {
		str = fmt.Sprintf("%dd%dh%dm", d, h, m)
	} else {
		str = fmt.Sprintf("%dh%dm", h, m)
	}
	return strings.Replace(str, "0h0m", "", -1)
}

// EggIncEvent is a raw event data for Egg Inc
type EggIncEvent struct {
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

// EggIncEvents holds a list of all Events, newest is last
var EggIncEvents []EggIncEvent

// LastMissionEvent holds the most recent mission event
var LastMissionEvent []EggIncEvent

func getEventMultiplier(event string) *EggIncEvent {
	// loop through EggIncEvents and if there is a matching event return it
	for _, e := range EggIncEvents {
		if e.EventType == event {
			return &EggIncEvent{
				Message:    e.Message,
				Multiplier: e.Multiplier,
				EventType:  e.EventType,
				Ultra:      e.Ultra,
				StartTime:  e.StartTime,
				EndTime:    e.EndTime,
			}
		}
	}
	return nil
}

// LoadEventData will load event data from a file
func LoadEventData(filename string) {

	var EggIncEventsLoaded []EggIncEvent
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&EggIncEventsLoaded)
	if err != nil {
		log.Fatal(err)
	}

	eventmap := make(map[string]EggIncEvent)

	var newEggIncEvents []EggIncEvent
	for _, e := range EggIncEventsLoaded {
		endTimestampRaw := int64(math.Round(e.EndTimestamp))
		e.EndTime = time.Unix(endTimestampRaw, 0)

		StartTimestampRaw := int64(math.Round(e.StartTimestamp))
		e.StartTime = time.Unix(StartTimestampRaw, 0)

		if e.StartTime.Before(time.Now().UTC()) && e.EndTime.After(time.Now().UTC()) {
			newEggIncEvents = append(newEggIncEvents, e)
			continue
		}
		// Continue above retains the previous event
		if strings.HasPrefix(e.EventType, "mission-") {
			name := e.EventType
			if e.Ultra {
				name += "-ultra"
			}
			eventmap[name] = e
		}
	}

	EggIncEvents = newEggIncEvents

	// Sort eventmap by StartTime, oldest first into LstMissionEvent
	var missionEvent []EggIncEvent
	for _, event := range eventmap {
		missionEvent = append(missionEvent, event)
	}
	sort.Slice(missionEvent, func(i, j int) bool {
		return missionEvent[i].StartTime.Before(missionEvent[j].StartTime)
	})
	LastMissionEvent = missionEvent

}
