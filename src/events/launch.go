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

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"

	"github.com/bwmarrin/discordgo"
)

var integerZeroMinValue float64 = 0.0

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
	defer func() {
		if err := file.Close(); err != nil {
			// Handle the error appropriately, e.g., logging or taking corrective actions
			log.Printf("Failed to close: %v", err)
		}
	}()
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
