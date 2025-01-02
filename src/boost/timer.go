package boost

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
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
	go func(t *BotTimer) {
		<-t.timer.C
		u, err := s.UserChannelCreate(t.UserID)
		if err != nil {
			fmt.Printf("Error creating user channel: %v\n", err)
			return
		}
		msg, err := s.ChannelMessageSend(u.ID, t.Message)
		if err != nil {
			fmt.Printf("Error sending message: %v\n", err)
			return
		}
		timerSetActiveState(t.ID, false)
		if msg != nil {
			timerSetMsgID(t.ID, u.ID, msg.ID)
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
				err := s.ChannelMessageDelete(timers[i].ChannelID, timers[i].MsgID)
				if err != nil {
					fmt.Printf("Error deleting message: %v\n", err)
				}
				purgeIndexes = append(purgeIndexes, i)
			}
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
				Description: "When do you want the timer to remind you? Example: 4m or 1h30m5s",
				Required:    true,
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
	setTimer := false

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: processingRequestMessage,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	duration := time.Duration(1 * time.Minute)
	var message string

	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["message"]; ok {
		message = fmt.Sprintf("%s <#%s>", opt.StringValue(), i.ChannelID)
	} else {
		message = fmt.Sprintf("activity reminder in <#%s>", i.ChannelID)
	}

	if opt, ok := optionMap["duration"]; ok {
		timespan := opt.StringValue()
		timespan = strings.Replace(timespan, "min", "m", -1)
		timespan = strings.Replace(timespan, "hr", "h", -1)
		timespan = strings.Replace(timespan, "sec", "s", -1)
		timespan = strings.TrimSpace(timespan)
		dur, err := str2duration.ParseDuration(timespan)
		if err == nil {
			// Error during parsing means skip this duration
			duration = dur
			setTimer = true
		} else {
			message = fmt.Sprintf("Error parsing duration: %s", err.Error())
		}
	}

	if !setTimer {
		_, _ = s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content: message,
			})
		return
	}

	//contract := FindContract(i.ChannelID)
	userID := getInteractionUserID(i)

	t := BotTimer{
		ID:       xid.New().String(),
		Reminder: time.Now().Add(duration),
		Message:  message,
		UserID:   userID,
		timer:    time.NewTimer(duration),
		Active:   true,
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
				fmt.Fprintf(&builder, "\n> <t:%d:R> %s", timers[i].Reminder.Unix(), timers[i].Message)
			}
		} else {
			if timers[i].ChannelID != "" && timers[i].MsgID != "" {
				// Purge old timer messages when a new one is scheduled
				err := s.ChannelMessageDelete(timers[i].ChannelID, timers[i].MsgID)
				if err != nil {
					fmt.Fprintf(&builder, "\n> Error deleting message: %s", err.Error())
				}

			}
		}
	}
	timers = newTimers
	saveTimerData()

	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Content: builder.String(),
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
