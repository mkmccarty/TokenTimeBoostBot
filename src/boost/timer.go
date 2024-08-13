package boost

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/xid"
	"github.com/xhit/go-str2duration/v2"
)

var timers []BotTimer

func init() {
	loadTimerData()
}

// LaunchIndependentTimers will start all the timers that are active
func LaunchIndependentTimers(s *discordgo.Session) {

	now := time.Now()
	for _, t := range timers {
		if now.Before(t.Reminder) {
			nextTimer := time.Until(t.Reminder)
			if nextTimer >= 0 {
				t.timer = time.NewTimer(nextTimer)

				go func(t *BotTimer) {
					<-t.timer.C
					u, _ := s.UserChannelCreate(t.UserID)
					_, _ = s.ChannelMessageSend(u.ID, t.Message)
					t.Active = false
				}(&t)
			}
		} else {
			t.Active = false
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
			Content: "Processing request...",
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

	go func(t *BotTimer) {
		<-t.timer.C
		u, _ := s.UserChannelCreate(t.UserID)
		_, _ = s.ChannelMessageSend(u.ID, t.Message)
		timerSetActiveState(t.ID, false)
	}(&t)

	timers = append(timers, t)
	var newTimers []BotTimer
	now := time.Now()
	for _, el := range timers {
		if now.Before(el.Reminder) {
			// Only move over new timers
			newTimers = append(newTimers, el)
		}
	}
	timers = newTimers
	saveTimerData()

	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Content: "Timer set for " + message,
		})
}

func timerSetActiveState(id string, active bool) {
	for i, t := range timers {
		if t.ID == id {
			timers[i].Active = active
			break
		}
	}
}

func saveTimerData() {
	b, _ := json.Marshal(timers)
	_ = dataStore.Write("Timers", b)
}

func loadTimerData() {
	b, err := dataStore.Read("Timers")
	if err == nil {
		_ = json.Unmarshal(b, &timers)
	}
}
