package events

import (
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

var integerZeroMinValue float64 = 0.0

func getEventMultiplier(event string) *ei.EggEvent {
	// loop through EggIncEvents and if there is a matching event return it
	for _, e := range ei.EggIncEvents {
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
