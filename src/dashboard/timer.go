package dashboard

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"uuid"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/xhit/go-str2duration/v2"
)

// BotTimer holds the data for each timer
type BotTimer struct {
	ID                string        `json:"id"`
	Reminder          time.Time     `json:"reminder"`
	timer             *time.Timer   `json:"-"`
	Message           string        `json:"message"`
	UserID            string        `json:"user_id"`
	ChannelID         string        `json:"channel_id"`
	MsgID             string        `json:"msg_id"`
	Duration          time.Duration `json:"duration"`
	OriginalChannelID string        `json:"original_channel_id"`
	OriginalMsgID     string        `json:"original_msg_id"`
	Active            bool          `json:"active"`
}

var timersMutex sync.Mutex
var timers []BotTimer

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

func startTimer(client dc.Client, t *BotTimer) {
	deleteDuration := getTimerMsgDuration(t.UserID)
	go func(t *BotTimer) {
		<-t.timer.C
		u, err := client.CreateUserChannel(t.UserID)
		if err != nil {
			log.Printf("Error creating user channel: %v\n", err)
			return
		}

		var components []dc.LayoutComponent
		var actionRowComponents []dc.InteractiveComponent

		// Repeat button
		actionRowComponents = append(actionRowComponents, dc.Button{
			Label:    fmt.Sprintf("Repeat %s Timer", bottools.FmtDuration(t.Duration)),
			Style:    dc.ButtonPrimary,
			CustomID: fmt.Sprintf("timer_btn#repeat#%s", t.ID),
		})

		// 3m40s button
		threeMin40s := 3*time.Minute + 40*time.Second
		if t.Duration > threeMin40s+30*time.Second || t.Duration < threeMin40s-30*time.Second {
			actionRowComponents = append(actionRowComponents, dc.Button{
				Label:    "New 3m40s Timer",
				Style:    dc.ButtonPrimary,
				CustomID: fmt.Sprintf("timer_btn#repeat_3m40s#%s", t.ID),
			})
		}

		// Close button
		actionRowComponents = append(actionRowComponents, dc.Button{
			Label:    "Close",
			Style:    dc.ButtonDanger,
			CustomID: fmt.Sprintf("timer_btn#close#%s", t.ID),
		})

		components = append(components, dc.ActionRow{Components: actionRowComponents})

		finalMessage := t.Message
		if t.OriginalChannelID != "" {
			finalMessage = fmt.Sprintf("%s in <#%s>", t.Message, t.OriginalChannelID)
		}
		if deleteDuration > 0 {
			finalMessage = fmt.Sprintf("%s\nReminder deleting <t:%d:R>", finalMessage, time.Now().Add(deleteDuration).Unix())
		}

		msg, err := client.SendMessage(u.ID, dc.Message{
			Content:      finalMessage,
			Components:   components,
			ComponentsV1: true,
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
					err := client.DeleteMessage(msg.ChannelID, msg.ID)
					if err != nil {
						log.Println(err)
					}
					timerDelete(t.ID)
				})
			}
		}
	}(t)
}

func purgeOldTimers(client dc.Client) {
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
				_ = client.DeleteMessage(timers[i].ChannelID, timers[i].MsgID)
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
func LaunchIndependentTimers(client dc.Client) {
	loadTimerData()

	now := time.Now()
	timersMutex.Lock()
	for i := range timers {
		if now.Before(timers[i].Reminder) {
			nextTimer := time.Until(timers[i].Reminder)
			if nextTimer >= 0 {
				timers[i].timer = time.NewTimer(nextTimer)
				startTimer(client, &timers[i])
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
func GetSlashTimer(cmd string) *dc.Command {
	command := dc.Command{
		Name:        cmd,
		Description: "Set a DM reminder timer for a contract",
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
			dc.StringOption{
				Name:        "duration",
				Description: "When do you want the timer to remind you? Example: 4m or 1h30m5s. [Sticky]",
				Required:    false,
			},
			dc.StringOption{
				Name:        "message",
				Description: "Message to display when the timer expires",
				Required:    false,
			},
			dc.StringOption{
				Name:        "dm-timeout",
				Description: "How long the message stays in DM (e.g. 30s, 5m). 0 to keep until closed. [Sticky]",
				Required:    false,
			},
		},
	}
	return &command
}

// HandleTimer handles the /timer command through the dc facade.
func HandleTimer(client dc.Client, e *dc.CommandEvent) {
	userID := e.UserID()

	_ = e.Defer(true)

	var message string
	var statusMessage string

	if opt, ok := e.OptString("message"); ok {
		message = opt
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

	if opt, ok := e.OptString("duration"); ok {
		timespan := bottools.SanitizeStringDuration(opt)
		dur, err := str2duration.ParseDuration(timespan)
		if err == nil {
			// Error during parsing means skip this duration
			duration = dur
			farmerstate.SetMiscSettingString(userID, "timer", opt)
			statusMessage = fmt.Sprintf("Sticky timer set to: %s", duration)
		} else {
			statusMessage = fmt.Sprintf("Could not parse duration '%s'. Using default/sticky.", opt)
		}
	}

	if opt, ok := e.OptString("dm-timeout"); ok {
		val := opt
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
			statusMessage += fmt.Sprintf("\nCould not parse dm-timeout '%s'.", opt)
		}
	}

	t := BotTimer{
		ID:                uuid.NewV7().String(),
		Reminder:          time.Now().Add(duration),
		Message:           message,
		UserID:            userID,
		timer:             time.NewTimer(duration),
		Active:            true,
		Duration:          duration,
		OriginalChannelID: e.ChannelID(),
	}
	startTimer(client, &t)

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
				_ = client.DeleteMessage(timers[i].ChannelID, timers[i].MsgID)
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

	_ = e.Followup(dc.Message{Content: builder.String(), Ephemeral: true})

	purgeOldTimers(client)
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

func HandleTimerInteraction(client dc.Client, e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	action := parts[1]
	timerID := parts[2]

	switch action {
	case "repeat":
		handleTimerRepeat(client, e, timerID, 0)
		timerDelete(timerID)
	case "repeat_3m40s":
		handleTimerRepeat(client, e, timerID, 3*time.Minute+40*time.Second)
		timerDelete(timerID)
	case "close":
		handleTimerClose(client, e, timerID)
	}
}

func handleTimerClose(client dc.Client, e *dc.ComponentEvent, timerID string) {
	if err := e.DeferUpdate(); err != nil {
		log.Printf("Error responding to timer close: %v", err)
	}
	_ = client.DeleteMessage(e.ChannelID(), e.MessageID())
	timerDelete(timerID)
}

func handleTimerRepeat(client dc.Client, e *dc.ComponentEvent, oldTimerID string, newDuration time.Duration) {
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
		_ = e.Respond(dc.Message{Content: "Could not find the original timer to repeat.", Ephemeral: true})
		return
	}

	duration := originalTimer.Duration
	if newDuration > 0 {
		duration = newDuration
	}

	userID := e.UserID()

	content := fmt.Sprintf("New timer set for %s.", duration)
	if originalTimer.OriginalChannelID != "" {
		content = fmt.Sprintf("New timer set for %s in <#%s>.", duration, originalTimer.OriginalChannelID)
	}

	// Acknowledge interaction
	if err := e.Update(dc.Message{Content: content, ComponentsV1: true}); err != nil {
		log.Printf("Error responding to timer repeat: %v", err)
	}

	messageID := e.MessageID()
	channelID := e.ChannelID()
	time.AfterFunc(10*time.Second, func() {
		if messageID != "" {
			_ = client.DeleteMessage(channelID, messageID)
		}
	})

	// Create and start new timer
	t := BotTimer{
		ID: uuid.NewV7().String(), Reminder: time.Now().Add(duration), Message: originalTimer.Message, UserID: userID, timer: time.NewTimer(duration), Active: true, Duration: duration, OriginalChannelID: originalTimer.OriginalChannelID,
	}
	startTimer(client, &t)

	timersMutex.Lock()
	timers = append(timers, t)
	timersMutex.Unlock()

	farmerstate.AddTimer(t.ID, t.UserID, t.ChannelID, t.MsgID, t.Reminder, t.Message, int64(t.Duration), t.OriginalChannelID, t.OriginalMsgID, t.Active)
}
