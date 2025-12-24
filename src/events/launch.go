package events

import (
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"

	"github.com/bwmarrin/discordgo"
)

var integerZeroMinValue float64 = 0.0

func getInteractionUserID(i *discordgo.InteractionCreate) string {
	if i.GuildID == "" {
		return i.User.ID
	}
	return i.Member.User.ID
}

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
