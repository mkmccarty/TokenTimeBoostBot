package events

import (
	"encoding/json"
	"log"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

var integerZeroMinValue float64 = 0.0

type shipData struct {
	Name     string   `json:"Name"`
	Art      string   `json:"Art"`
	ArtDev   string   `json:"ArtDev"`
	Duration []string `json:"Duration"`
}

type missionData struct {
	Ships []shipData
}

const missionJSON = `{"ships":[
	{"name": "Atreggies Henliner","art":"<:atreggies:1280038674398183464>","artDev":"<:atreggies:1280389911509340240>","duration":["2d","3d","4d"]},
	{"name": "Henerprise","art":"<:henerprise:1280038539328749609>","artDev":"<:henerprise:1280390026487664704>","duration":["1d","2d","4d"]},
	{"name": "Voyegger","art":"<:voyegger:1280041822416273420>","artDev":"<:voyegger:1280390114094354472>","duration":["12h","1d12h","3d"]},
	{"name": "Defihent","art":"<:defihent:1280044758001258577>","artDev":"<:defihent:1280390249943666739>","duration":["8h","1d","2d"]},
	{"name": "Galeggtica","art":"<:galeggtica:1280045010917527593>","artDev":"<:galeggtica:1280390347825872916>","duration":["6h","16h","1d6h"]},
	{"name": "Cornish-Hen Corvette","art":"<:corellihencorvette:1280045137518657536>","artDev":"<:corellihencorvette:1280390458983452742>","duration":["4h","12h","1d"]},
	{"name": "Quintillion Chicken","art":"<:milleniumchicken:1280045411444326400>","artDev":"<:milleniumchicken:1280390575178383386>","duration":["3h","6h","12h"]},
	{"name": "BCR","art":"<:bcr:1280045542495228008>","artDev":"<:bcr:1280390686461661275>","duration":["1h30m","4h","8h"]},
	{"name": "Chicken Heavy","art":"<:chickenheavy:1280045643922018315>","artDev":"<:chickenheavy:1280390782783590473>","duration":["45m","1h30m","4h"]},
	{"name": "Chicken Nine","art":"<:chicken9:1280045842442616902>","artDev":"<:chicken9:1280390884575154226>","duration":["30m","1h","3h"]},
	{"name": "Chicken One","art":"<:chicken1:1280045945974951949>","artDev":"<:chicken1:1280390988824576061>","duration":["20m","1h","2h"]}
	]}`

var mis missionData

func init() {
	_ = json.Unmarshal([]byte(missionJSON), &mis)
}

var eventMutex sync.Mutex

// EggIncEvents holds a list of all Events, newest is last
var EggIncEvents []ei.EggEvent

// LastMissionEvent holds the most recent mission event
var LastMissionEvent []ei.EggEvent

// LastEvent holds the most recent event of each type
var LastEvent []ei.EggEvent

// AllEventMap holds all events by type
var AllEventMap map[string]ei.EggEvent

func init() {
	AllEventMap = make(map[string]ei.EggEvent)
}

func getInteractionUserID(i *discordgo.InteractionCreate) string {
	if i.GuildID == "" {
		return i.User.ID
	}
	return i.Member.User.ID
}

func getEventMultiplier(event string) *ei.EggEvent {
	// loop through EggIncEvents and if there is a matching event return it
	for _, e := range EggIncEvents {
		if e.EventType == event {
			return &ei.EggEvent{
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

	var EggIncEventsLoaded []ei.EggEvent
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&EggIncEventsLoaded)
	if err != nil {
		log.Print(err)
		return
	}

	// allEventMap is a map of event types
	allEventMap := make(map[string]ei.EggEvent)
	var currentEggIncEvents []ei.EggEvent

	for _, e := range EggIncEventsLoaded {
		endTimestampRaw := int64(math.Round(e.EndTimestamp))
		e.EndTime = time.Unix(endTimestampRaw, 0)

		StartTimestampRaw := int64(math.Round(e.StartTimestamp))
		e.StartTime = time.Unix(StartTimestampRaw, 0)

		if e.StartTime.Before(time.Now().UTC()) && e.EndTime.After(time.Now().UTC()) {
			currentEggIncEvents = append(currentEggIncEvents, e)
			//continue
		}

		name := e.EventType
		if e.Ultra {
			name += "-ultra"
		}
		allEventMap[name] = e

	}
	sortAndSwapEvents(allEventMap, currentEggIncEvents)
}

func sortAndSwapEvents(allEventMap map[string]ei.EggEvent, currentEggIncEvents []ei.EggEvent) {
	// Sort missionEventMap by StartTime, oldest first into LastMissionEvent
	var missionEvent []ei.EggEvent

	// Spin through all events in the map and extract mission and all events for sorting
	var allEvent []ei.EggEvent
	for _, event := range allEventMap {
		allEvent = append(allEvent, event)

		if strings.HasPrefix(event.EventType, "mission-") {
			missionEvent = append(missionEvent, event)
		}
	}

	sort.Slice(missionEvent, func(i, j int) bool {
		return missionEvent[i].StartTime.Before(missionEvent[j].StartTime)
	})

	sort.Slice(allEvent, func(i, j int) bool {
		return allEvent[i].StartTime.Before(allEvent[j].StartTime)
	})

	// Swap in our new data
	eventMutex.Lock()
	EggIncEvents = currentEggIncEvents
	LastMissionEvent = missionEvent
	LastEvent = allEvent
	AllEventMap = allEventMap
	eventMutex.Unlock()
}
