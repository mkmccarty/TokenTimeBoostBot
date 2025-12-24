package ei

import (
	"encoding/json"
	"log"
	"math"
	"os"
	"sort"
	"strings"
	sync "sync"
	"time"
)

var currentGGEvent = 1.0
var currentUltraGGEvent = 1.0
var currentEventEndsGG time.Time
var currentEarningsEvent = 1.0
var currentEarningsEventUltra = 1.0

var currentResearchDiscountEvent = 1.0

//var currentResearchDiscountEventUltra = 1.0

// GetGenerousGiftEvent will return the current Generous Gift event multiplier
func GetGenerousGiftEvent() (float64, float64, time.Time) {
	return currentGGEvent, currentUltraGGEvent, currentEventEndsGG
}

// SetGenerousGiftEvent will return the current Generous Gift event multiplier
func SetGenerousGiftEvent(gg float64, ugg float64, endtime time.Time) {
	currentGGEvent = gg
	currentUltraGGEvent = ugg
	currentEventEndsGG = endtime
}

// SetEarningsEvent will set the current earnings event multipliers
func SetEarningsEvent(earnings float64, ultraEarnings float64) {
	currentEarningsEvent = earnings
	currentEarningsEventUltra = ultraEarnings
}

// SetResearchDiscountEvent will set the current research discount event multipliers
func SetResearchDiscountEvent(discount float64) {
	currentResearchDiscountEvent = discount
	//currentResearchDiscountEventUltra = ultraDiscount
}

// EventMutex protects access to EggIncEvents and LastEvent
var EventMutex sync.Mutex

// EggIncEvents holds a list of all Events, newest is last
var EggIncEvents []EggEvent

// LastMissionEvent holds the most recent mission event
var LastMissionEvent []EggEvent

// LastEvent holds the most recent event of each type
var LastEvent []EggEvent

// AllEventMap holds all events by type
var AllEventMap map[string]EggEvent

func init() {
	AllEventMap = make(map[string]EggEvent)
}

// LoadEventData will load event data from a file
func LoadEventData(filename string) {

	var EggIncEventsLoaded []EggEvent
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
	allEventMap := make(map[string]EggEvent)
	var currentEggIncEvents []EggEvent

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
	SortAndSwapEvents(allEventMap, currentEggIncEvents)
}

// SortAndSwapEvents will sort and swap in the new event data
func SortAndSwapEvents(allEventMap map[string]EggEvent, currentEggIncEvents []EggEvent) {
	// Sort missionEventMap by StartTime, oldest first into LastMissionEvent
	var missionEvent []EggEvent

	// Spin through all events in the map and extract mission and all events for sorting
	var allEvent []EggEvent
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
	EventMutex.Lock()
	EggIncEvents = currentEggIncEvents
	LastMissionEvent = missionEvent
	LastEvent = allEvent
	AllEventMap = allEventMap
	EventMutex.Unlock()
}

// FindGiftEvent returns gift-boost events that occur during the specified time
func FindGiftEvent(eventTime time.Time) EggEvent {
	var event EggEvent
	EventMutex.Lock()
	defer EventMutex.Unlock()
	for _, e := range AllEventMap {
		if e.EventType == "gift-boost" && e.StartTime.Before(eventTime) && e.EndTime.After(eventTime) {
			return e
		}
	}
	return event
}
