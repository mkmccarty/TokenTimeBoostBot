package boost

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/rs/xid"
	"github.com/xhit/go-str2duration/v2"
)

var timersMutex sync.Mutex
var timers []BotTimer

const (
	timerDataKey             = "Timers"
	processingRequestMessage = "Processing request..."
)

func timerDelete(id string) {
	timersMutex.Lock()
	for i, t := range timers {
		if t.ID == id {
			timers = append(timers[:i], timers[i+1:]...)
			break
		}
	}
	timersMutex.Unlock()
	farmerstate.DeleteTimer(id)
}

func getTimerMsgDuration(userID string) time.Duration {
	stickyMsgDur := farmerstate.GetMiscSettingString(userID, "timer_dm_timeout")
	if stickyMsgDur == "" {
		stickyMsgDur = farmerstate.GetMiscSettingString(userID, "timer_msg_duration")
	}
	if stickyMsgDur != "" {
		if stickyMsgDur == "0" || stickyMsgDur == "0s" {
			return 0
		}
		if dur, err := str2duration.ParseDuration(bottools.SanitizeStringDuration(stickyMsgDur)); err == nil {
			return dur
		}
	}
	return 30 * time.Second
}

func startTimer(s *discordgo.Session, t *BotTimer) {
	deleteDuration := getTimerMsgDuration(t.UserID)
	go func(t *BotTimer) {
		<-t.timer.C
		u, err := s.UserChannelCreate(t.UserID)
		if err != nil {
			log.Printf("Error creating user channel: %v\n", err)
			return
		}

		var components []discordgo.MessageComponent
		var actionRowComponents []discordgo.MessageComponent

		// Repeat button
		actionRowComponents = append(actionRowComponents, discordgo.Button{
			Label:    fmt.Sprintf("Repeat %s Timer", bottools.FmtDuration(t.Duration)),
			Style:    discordgo.PrimaryButton,
			CustomID: fmt.Sprintf("timer_btn#repeat#%s", t.ID),
		})

		// 3m40s button
		threeMin40s := 3*time.Minute + 40*time.Second
		if t.Duration > threeMin40s+30*time.Second || t.Duration < threeMin40s-30*time.Second {
			actionRowComponents = append(actionRowComponents, discordgo.Button{
				Label:    "New 3m40s Timer",
				Style:    discordgo.PrimaryButton,
				CustomID: fmt.Sprintf("timer_btn#repeat_3m40s#%s", t.ID),
			})
		}

		// Close button
		actionRowComponents = append(actionRowComponents, discordgo.Button{
			Label:    "Close",
			Style:    discordgo.DangerButton,
			CustomID: fmt.Sprintf("timer_btn#close#%s", t.ID),
		})

		components = append(components, discordgo.ActionsRow{Components: actionRowComponents})

		finalMessage := t.Message
		if t.OriginalChannelID != "" {
			finalMessage = fmt.Sprintf("%s in <#%s>", t.Message, t.OriginalChannelID)
		}
		if deleteDuration > 0 {
			finalMessage = fmt.Sprintf("%s\nReminder deleting <t:%d:R>", finalMessage, time.Now().Add(deleteDuration).Unix())
		}

		msg, err := s.ChannelMessageSendComplex(u.ID, &discordgo.MessageSend{
			Content:    finalMessage,
			Components: components,
		})
		if err != nil {
			log.Printf("Error sending message: %v\n", err)
			return
		}

		timerSetActiveState(t.ID, false)
		if msg != nil {
			timerSetMsgID(t.ID, u.ID, msg.ID)
			if deleteDuration > 0 {
				time.AfterFunc(deleteDuration, func() {
					err := s.ChannelMessageDelete(msg.ChannelID, msg.ID)
					if err != nil {
						log.Println(err)
					}
					timerDelete(t.ID)
				})
			}
		}
	}(t)
}

func purgeOldTimers(s *discordgo.Session) {
	timersMutex.Lock()
	var purgeIndexes []int
	var purgedIDs []string
	now := time.Now()
	for i := range timers {
		deleteDuration := getTimerMsgDuration(timers[i].UserID)
		if deleteDuration == 0 {
			continue
		}
		if now.After(timers[i].Reminder.Add(deleteDuration).Add(time.Minute)) {
			if timers[i].ChannelID != "" && timers[i].MsgID != "" {
				_ = s.ChannelMessageDelete(timers[i].ChannelID, timers[i].MsgID)
			}
			purgeIndexes = append(purgeIndexes, i)
			purgedIDs = append(purgedIDs, timers[i].ID)
		}
	}
	for i := len(purgeIndexes) - 1; i >= 0; i-- {
		idx := purgeIndexes[i]
		timers = append(timers[:idx], timers[idx+1:]...)
	}
	timersMutex.Unlock()

	for _, id := range purgedIDs {
		farmerstate.DeleteTimer(id)
	}
}

// LaunchIndependentTimers will start all the timers that are active
func LaunchIndependentTimers(s *discordgo.Session) {
	loadTimerData()

	now := time.Now()
	timersMutex.Lock()
	for i := range timers {
		if now.Before(timers[i].Reminder) {
			nextTimer := time.Until(timers[i].Reminder)
			if nextTimer >= 0 {
				timers[i].timer = time.NewTimer(nextTimer)
				startTimer(s, &timers[i])
			}
		} else {
			timers[i].Active = false
			farmerstate.UpdateTimerState(timers[i].ID, false)
		}
	}
	timersMutex.Unlock()

	farmerstate.DeleteInactiveTimers()
}

// GetSlashTimer will return the discord command for calculating ideal stone set
func GetSlashTimer(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Set a DM reminder timer for a contract",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
			discordgo.InteractionContextBotDM,
			discordgo.InteractionContextPrivateChannel,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
			discordgo.ApplicationIntegrationUserInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "duration",
				Description: "When do you want the timer to remind you? Example: 4m or 1h30m5s. [Sticky]",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "message",
				Description: "Message to display when the timer expires",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "dm-timeout",
				Description: "How long the message stays in DM (e.g. 30s, 5m). 0 to keep until closed. [Sticky]",
				Required:    false,
			},
		},
	}
}

// HandleTimerCommand will handle the /stones command
func HandleTimerCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := getInteractionUserID(i)

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: processingRequestMessage,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	var message string
	var statusMessage string

	optionMap := bottools.GetCommandOptionsMap(i)

	if opt, ok := optionMap["message"]; ok {
		message = opt.StringValue()
	} else {
		message = "Activity reminder"
	}

	duration := time.Duration(3*time.Minute + 30*time.Second) // Default to 3m30s

	stickyTimer := farmerstate.GetMiscSettingString(userID, "timer")
	if stickyTimer != "" {
		timespan := bottools.SanitizeStringDuration(stickyTimer)
		dur, err := str2duration.ParseDuration(timespan)
		if err == nil {
			duration = dur
			statusMessage = fmt.Sprintf("Using saved timer duration: %s", duration)
		}
	}

	if opt, ok := optionMap["duration"]; ok {
		timespan := bottools.SanitizeStringDuration(opt.StringValue())
		dur, err := str2duration.ParseDuration(timespan)
		if err == nil {
			// Error during parsing means skip this duration
			duration = dur
			farmerstate.SetMiscSettingString(userID, "timer", opt.StringValue())
			statusMessage = fmt.Sprintf("Sticky timer set to: %s", duration)
		} else {
			statusMessage = fmt.Sprintf("Could not parse duration '%s'. Using default/sticky.", opt.StringValue())
		}
	}

	if opt, ok := optionMap["dm-timeout"]; ok {
		val := opt.StringValue()
		if val == "0" {
			val = "0s"
		}
		timespan := bottools.SanitizeStringDuration(val)
		dur, err := str2duration.ParseDuration(timespan)
		if err == nil {
			farmerstate.SetMiscSettingString(userID, "timer_dm_timeout", val)
			if statusMessage != "" {
				statusMessage += "\n"
			}
			if dur == 0 {
				statusMessage += "Sticky message duration set to: Keep until closed"
			} else {
				statusMessage += fmt.Sprintf("Sticky message duration set to: %s", dur)
			}
		} else {
			statusMessage += fmt.Sprintf("\nCould not parse dm-timeout '%s'.", opt.StringValue())
		}
	}

	t := BotTimer{
		ID:                xid.New().String(),
		Reminder:          time.Now().Add(duration),
		Message:           message,
		UserID:            userID,
		timer:             time.NewTimer(duration),
		Active:            true,
		Duration:          duration,
		OriginalChannelID: i.ChannelID,
	}
	startTimer(s, &t)

	var builder strings.Builder
	if statusMessage != "" {
		builder.WriteString(statusMessage + "\n")
	}
	builder.WriteString("Timer set. Existing timers:")

	var purgedIDs []string
	timersMutex.Lock()
	timers = append(timers, t)
	var newTimers []BotTimer
	now := time.Now()
	for i := range timers {
		if now.Before(timers[i].Reminder) {
			// Only move over new timers
			newTimers = append(newTimers, timers[i])
			if timers[i].UserID == userID {
				displayMessage := timers[i].Message
				if timers[i].OriginalChannelID != "" {
					displayMessage = fmt.Sprintf("%s in <#%s>", displayMessage, timers[i].OriginalChannelID)
				}
				fmt.Fprintf(&builder, "\n> <t:%d:R> %s", timers[i].Reminder.Unix(), displayMessage)
			}
		} else {
			if timers[i].ChannelID != "" && timers[i].MsgID != "" {
				// Purge old timer messages when a new one is scheduled
				_ = s.ChannelMessageDelete(timers[i].ChannelID, timers[i].MsgID)
			}
			purgedIDs = append(purgedIDs, timers[i].ID)
		}
	}
	timers = newTimers
	timersMutex.Unlock()

	farmerstate.AddTimer(t.ID, t.UserID, t.ChannelID, t.MsgID, t.Reminder, t.Message, int64(t.Duration), t.OriginalChannelID, t.OriginalMsgID, t.Active)
	for _, id := range purgedIDs {
		farmerstate.DeleteTimer(id)
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Content: builder.String(),
			Flags:   discordgo.MessageFlagsEphemeral,
		})

	purgeOldTimers(s)
}

func timerSetActiveState(id string, active bool) {
	timersMutex.Lock()
	for i := range timers {
		if timers[i].ID == id {
			timers[i].Active = active
			break
		}
	}
	timersMutex.Unlock()
	farmerstate.UpdateTimerState(id, active)
}

func timerSetMsgID(id string, channelID string, msgID string) {
	timersMutex.Lock()

	for i := range timers {
		if timers[i].ID == id {
			timers[i].ChannelID = channelID
			timers[i].MsgID = msgID
			break
		}
	}
	timersMutex.Unlock()
	farmerstate.UpdateTimerMsg(id, channelID, msgID)
}

func loadTimerData() {
	b, err := dataStore.Read(timerDataKey)
	if err == nil && len(b) > 0 {
		var legacyTimers []BotTimer
		if err := json.Unmarshal(b, &legacyTimers); err == nil {
			for _, t := range legacyTimers {
				farmerstate.AddTimer(t.ID, t.UserID, t.ChannelID, t.MsgID, t.Reminder, t.Message, int64(t.Duration), t.OriginalChannelID, t.OriginalMsgID, t.Active)
			}
		}
		_ = dataStore.Erase(timerDataKey)
	}

	dbTimers := farmerstate.GetAllTimers()

	timersMutex.Lock()
	timers = make([]BotTimer, 0, len(dbTimers))
	for _, dt := range dbTimers {
		timers = append(timers, BotTimer{
			ID:                dt.ID,
			UserID:            dt.UserID,
			ChannelID:         dt.ChannelID,
			MsgID:             dt.MsgID,
			Reminder:          dt.Reminder,
			Message:           dt.Message,
			Duration:          time.Duration(dt.Duration),
			OriginalChannelID: dt.OriginalChannelID,
			OriginalMsgID:     dt.OriginalMsgID,
			Active:            dt.Active,
		})
	}
	timersMutex.Unlock()
}

// HandleTimerInteraction handles button interactions from timer DMs.
func HandleTimerInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	parts := strings.Split(i.MessageComponentData().CustomID, "#")
	action := parts[1]
	timerID := parts[2]

	switch action {
	case "repeat":
		handleTimerRepeat(s, i, timerID, 0)
		timerDelete(timerID)
	case "repeat_3m40s":
		handleTimerRepeat(s, i, timerID, 3*time.Minute+40*time.Second)
		timerDelete(timerID)
	case "close":
		handleTimerClose(s, i, timerID)
	}
}

func handleTimerClose(s *discordgo.Session, i *discordgo.InteractionCreate, timerID string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
	if err != nil {
		log.Printf("Error responding to timer close: %v", err)
	}
	_ = s.ChannelMessageDelete(i.ChannelID, i.Message.ID)
	timerDelete(timerID)
}

func handleTimerRepeat(s *discordgo.Session, i *discordgo.InteractionCreate, oldTimerID string, newDuration time.Duration) {
	timersMutex.Lock()
	var originalTimer BotTimer
	found := false
	for _, t := range timers {
		if t.ID == oldTimerID {
			originalTimer = t
			found = true
			break
		}
	}
	timersMutex.Unlock()

	if !found {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Could not find the original timer to repeat.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	duration := originalTimer.Duration
	if newDuration > 0 {
		duration = newDuration
	}

	userID := getInteractionUserID(i)

	// Acknowledge interaction
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    fmt.Sprintf("New timer set for %s.", duration),
			Components: []discordgo.MessageComponent{},
		},
	})
	if err != nil {
		log.Printf("Error responding to timer repeat: %v", err)
	}

	// Create and start new timer
	t := BotTimer{
		ID: xid.New().String(), Reminder: time.Now().Add(duration), Message: originalTimer.Message, UserID: userID, timer: time.NewTimer(duration), Active: true, Duration: duration, OriginalChannelID: originalTimer.OriginalChannelID,
	}
	startTimer(s, &t)

	timersMutex.Lock()
	timers = append(timers, t)
	timersMutex.Unlock()

	farmerstate.AddTimer(t.ID, t.UserID, t.ChannelID, t.MsgID, t.Reminder, t.Message, int64(t.Duration), t.OriginalChannelID, t.OriginalMsgID, t.Active)
}
