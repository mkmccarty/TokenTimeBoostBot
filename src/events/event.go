package events

import (
	"fmt"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

// SlashEventHelperCommand returns the command for the /launch-helper command
func SlashEventHelperCommand(cmd string) *dc.Command {
	command := dc.Command{
		Name:        cmd,
		Description: "Display Last Event(s) and current Event(s) information.",
		Contexts: []dc.InteractionContext{
			dc.ContextGuild,
			dc.ContextBotDM,
			dc.ContextPrivateChannel,
		},
		IntegrationTypes: []dc.IntegrationType{
			dc.IntegrationGuildInstall,
			dc.IntegrationUserInstall,
		},
		Options: []dc.Option{
			dc.BoolOption{
				Name:        "ultra",
				Description: "Show ultra event info. Default is false. [Sticky]",
				Required:    false,
			},
			dc.BoolOption{
				Name:        "private",
				Description: "Private reply, default is false. [Sticky]",
				Required:    false,
			},
		},
	}
	return &command
}

// HandleEventHelperCommand handles the /launch-helper command through the dc facade.
func HandleEventHelperCommand(e *dc.CommandEvent) {
	ultraIcon, _, _ := ei.GetBotEmoji("ultra")

	userID := e.UserID()

	privateReply := false

	ultra := false
	if opt, ok := e.OptBool("ultra"); ok {
		ultra = opt
		farmerstate.SetMiscSettingFlag(userID, "ultra", ultra)
	} else {
		ultra = farmerstate.GetMiscSettingFlag(userID, "ultra")
	}
	if opt, ok := e.OptBool("private"); ok {
		privateReply = opt
		farmerstate.SetMiscSettingFlag(userID, "event-private", privateReply)
	} else {
		privateReply = farmerstate.GetMiscSettingFlag(userID, "event-private")
	}

	_ = e.Defer(privateReply)

	var field []dc.EmbedField
	var events strings.Builder

	ei.EventMutex.Lock()
	localLastEvent := make([]ei.EggEvent, len(ei.LastEvent))
	copy(localLastEvent, ei.LastEvent)

	localEggIncEvents := make([]ei.EggEvent, len(ei.EggIncEvents))
	copy(localEggIncEvents, ei.EggIncEvents)
	ei.EventMutex.Unlock()

	events.WriteString("## Current Events:\n")
	// Build list of current Events
	for _, e := range localEggIncEvents {
		ultraStr := ""
		if e.Ultra {
			ultraStr = ultraIcon
			if !ultra {
				continue
			}
		}
		hours := e.EndTime.Sub(e.StartTime).Hours()
		if hours < 1.0 {
			mins := e.EndTime.Sub(e.StartTime).Minutes()
			fmt.Fprintf(&events, "%s%s for %.2dm ends <t:%d:R>\n", ultraStr, e.Message, int(mins), e.EndTime.Unix())
		} else {
			fmt.Fprintf(&events, "%s%s for %.2dh ends <t:%d:R>\n", ultraStr, e.Message, int(hours), e.EndTime.Unix())
		}
	}

	var prevEvents strings.Builder
	var ultraEvents strings.Builder
	str := ""

	continuedStr := ""

	// Previous Non Ultra Events
	for _, e := range localLastEvent {
		ultraStr := ""
		if e.Ultra {
			ultraStr = ultraIcon
			if !ultra {
				continue
			}
		}
		hours := e.EndTime.Sub(e.StartTime).Hours()
		if hours < 1.0 {
			mins := e.EndTime.Sub(e.StartTime).Minutes()
			str = fmt.Sprintf("%s%s for %.2dm <t:%d:R>\n", ultraStr, e.Message, int(mins), e.StartTime.Unix())
		} else {
			str = fmt.Sprintf("%s%s for %.2dh <t:%d:R>\n", ultraStr, e.Message, int(hours), e.StartTime.Unix())
		}

		if e.Ultra {
			ultraEvents.WriteString(str)
		} else {
			prevEvents.WriteString(str)
		}
		/*
			if len(prevEvents.String()) > 900 {
				field = append(field, dc.EmbedField{
					Name:   "Event History",
					Value:  prevEvents.String(),
					Inline: false,
				})
				prevEvents.Reset()
				continuedStr = " (Continued)"
			}*/
	}

	field = append(field, dc.EmbedField{
		Name:   "Event History",
		Value:  prevEvents.String(),
		Inline: false,
	})

	if ultra {
		field = append(field, dc.EmbedField{
			Name:   "Ultra Event History" + continuedStr,
			Value:  ultraEvents.String(),
			Inline: false,
		})
	}

	if len(config.EventsURL) > 0 {
		events.WriteString("[Event Calendar](")
		events.WriteString(config.EventsURL)
		events.WriteString(")")
	}

	_ = e.Followup(dc.Message{
		Content: events.String() + "\n\n",
		Embeds: []dc.Embed{{
			Color:  0x0055FF,
			Fields: field,
		}},
	})

}
