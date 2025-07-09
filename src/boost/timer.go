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
			fmt.Printf("Error creating user channel: %v\n", err)
			return
		}
		msg, err := s.ChannelMessageSend(u.ID, fmt.Sprintf("%s\nReminder deleting <t:%d:R>", t.Message, time.Now().Add(deleteDuration).Unix()))
		if err != nil {
			fmt.Printf("Error sending message: %v\n", err)
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
	setTimer := false
	userID := getInteractionUserID(i)

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: processingRequestMessage,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

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

	duration := time.Duration(3*time.Minute + 30*time.Second) // Default to 3m30s
	setTimer = true

	stickyTimer := farmerstate.GetMiscSettingString(userID, "timer")
	if stickyTimer != "" {
		timespan := bottools.SanitizeStringDuration(stickyTimer)
		dur, err := str2duration.ParseDuration(timespan)
		if err == nil {
			// Error during parsing means skip this duration
			duration = dur
			setTimer = true
			message = fmt.Sprintf("Sticky timer set to %s", stickyTimer)
		}
	}

	if opt, ok := optionMap["duration"]; ok {
		timespan := bottools.SanitizeStringDuration(opt.StringValue())
		dur, err := str2duration.ParseDuration(timespan)
		if err == nil {
			// Error during parsing means skip this duration
			duration = dur
			setTimer = true
			farmerstate.SetMiscSettingString(userID, "timer", opt.StringValue())
		}
	}

	if !setTimer {
		_, _ = s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content: message,
			})
		return
	}

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
				_ = s.ChannelMessageDelete(timers[i].ChannelID, timers[i].MsgID)
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
