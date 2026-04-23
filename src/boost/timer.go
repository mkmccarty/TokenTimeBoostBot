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

func init() {
	loadTimerData()
}

func startTimer(s *discordgo.Session, t *BotTimer) {
	deleteDuration := 30 * time.Second
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
		finalMessage = fmt.Sprintf("%s\nReminder deleting <t:%d:R>", finalMessage, time.Now().Add(deleteDuration).Unix())

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
			time.AfterFunc(deleteDuration, func() {
				err := s.ChannelMessageDelete(msg.ChannelID, msg.ID)
				if err != nil {
					log.Println(err)
				}
				timerSetMsgID(t.ID, "", "")
				saveTimerData()
			})
			saveTimerData()
		}
	}(t)
}

func purgeOldTimers(s *discordgo.Session) {
	timersMutex.Lock()
	defer timersMutex.Unlock()
	var purgeIndexes []int
	now := time.Now()
	for i := range timers {
		if now.After(timers[i].Reminder.Add(5 * time.Minute)) {
			if timers[i].ChannelID != "" && timers[i].MsgID != "" {
				_ = s.ChannelMessageDelete(timers[i].ChannelID, timers[i].MsgID)
			}
			purgeIndexes = append(purgeIndexes, i)
		}
	}
	for _, i := range purgeIndexes {
		timers = append(timers[:i], timers[i+1:]...)
	}
}

// LaunchIndependentTimers will start all the timers that are active
func LaunchIndependentTimers(s *discordgo.Session) {
	now := time.Now()
	for i := range timers {
		if now.Before(timers[i].Reminder) {
			nextTimer := time.Until(timers[i].Reminder)
			if nextTimer >= 0 {
				timers[i].timer = time.NewTimer(nextTimer)
				startTimer(s, &timers[i])
			}
		} else {
			timers[i].Active = false
		}
	}
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

	if statusMessage != "" {
		_, _ = s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content: statusMessage,
				Flags:   discordgo.MessageFlagsEphemeral,
			})
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
	builder.WriteString("Timer set. Existing timers:")

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
		}
	}
	timers = newTimers
	saveTimerData()

	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Content: builder.String(),
			Flags:   discordgo.MessageFlagsEphemeral,
		})

	purgeOldTimers(s)
}

func timerSetActiveState(id string, active bool) {
	timersMutex.Lock()
	defer timersMutex.Unlock()
	for i := range timers {
		if timers[i].ID == id {
			timers[i].Active = active
			break
		}
	}
}

func timerSetMsgID(id string, channelID string, msgID string) {
	timersMutex.Lock()
	defer timersMutex.Unlock()

	for i := range timers {
		if timers[i].ID == id {
			timers[i].ChannelID = channelID
			timers[i].MsgID = msgID
			break
		}
	}
}

func saveTimerData() {
	timersMutex.Lock()
	defer timersMutex.Unlock()

	b, _ := json.Marshal(timers)
	_ = dataStore.Write(timerDataKey, b)
}

func loadTimerData() {
	timersMutex.Lock()
	defer timersMutex.Unlock()

	b, err := dataStore.Read(timerDataKey)
	if err == nil {
		_ = json.Unmarshal(b, &timers)
	}

}

// HandleTimerInteraction handles button interactions from timer DMs.
func HandleTimerInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	parts := strings.Split(i.MessageComponentData().CustomID, "#")
	action := parts[1]
	timerID := parts[2]

	switch action {
	case "repeat":
		handleTimerRepeat(s, i, timerID, 0)
	case "repeat_3m40s":
		handleTimerRepeat(s, i, timerID, 3*time.Minute+40*time.Second)
	case "close":
		handleTimerClose(s, i)
	}
}

func handleTimerClose(s *discordgo.Session, i *discordgo.InteractionCreate) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
	if err != nil {
		log.Printf("Error responding to timer close: %v", err)
	}
	_ = s.ChannelMessageDelete(i.ChannelID, i.Message.ID)
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
	saveTimerData()
}
