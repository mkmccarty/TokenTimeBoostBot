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

var timers []ContractTimer

func init() {
	loadTimerData()
}

// LaunchIndependentTimers will start all the timers that are active
func LaunchIndependentTimers(s *discordgo.Session) {

	for _, t := range timers {
		if t.Active {
			mextTimer := time.Until(t.Reminder)
			if mextTimer > 0 {
				t.timer = time.NewTimer(mextTimer)

				go func(t *ContractTimer) {
					<-t.timer.C
					u, _ := s.UserChannelCreate(t.UserID)
					_, _ = s.ChannelMessageSend(u.ID, t.Message)
					t.Active = false
				}(&t)
			}
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
				Name:        "timer",
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

	if opt, ok := optionMap["timer"]; ok {
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

	contract := FindContract(i.ChannelID)
	userID := getInteractionUserID(i)

	t := ContractTimer{
		ID:       xid.New().String(),
		Reminder: time.Now().Add(duration),
		Message:  message,
		UserID:   userID,
		timer:    time.NewTimer(duration),
		Active:   true,
	}

	go func(t *ContractTimer) {
		<-t.timer.C
		u, _ := s.UserChannelCreate(t.UserID)
		_, _ = s.ChannelMessageSend(u.ID, t.Message)
		t.Active = false
	}(&t)

	if contract != nil {
		// Save this timer for later
		contract.Timers = append(contract.Timers, t)
		saveData(Contracts)
	} else {
		timers = append(timers, t)
		var newTimers []ContractTimer
		for _, el := range timers {
			if el.Active {
				newTimers = append(newTimers, el)
			}
		}
		timers = newTimers
		saveTimerData()
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Content: "Timer set for " + message,
		})

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
