package boost

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/xid"
)

var integeOneMinValue float64 = 1.0

// GetSlashTimer will return the discord command for calculating ideal stone set
func GetSlashTimer(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Set a DM reminder timer for a contract",
		/*
			Contexts: &[]discordgo.InteractionContextType{
				discordgo.InteractionContextGuild,
				discordgo.InteractionContextBotDM,
				discordgo.InteractionContextPrivateChannel,
			},
			IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
				discordgo.ApplicationIntegrationGuildInstall,
				discordgo.ApplicationIntegrationUserInstall,
			},
		*/
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "minutes",
				Description: "Minutes to set the timer for",
				MinValue:    &integeOneMinValue,
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

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			//Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	minutes := time.Duration(1 * time.Minute)
	message := fmt.Sprintf("Reminding you about your contract <#%s>", i.ChannelID)

	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["minutes"]; ok {
		min := opt.IntValue()
		minutes = time.Duration(min * int64(time.Minute))
	}
	if opt, ok := optionMap["message"]; ok {
		message = fmt.Sprintf("%s <#%s>", opt.StringValue(), i.ChannelID)
	}

	contract := FindContract(i.ChannelID)
	if contract == nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content: "No contract found in this channel. Please provide a contract-id and coop-id.",
			})
		return
	}

	userID := getInteractionUserID(i)

	t := ContractTimer{
		ID:       xid.New().String(),
		Reminder: time.Now().Add(minutes),
		Message:  message,
		UserID:   userID,
		timer:    time.NewTimer(minutes),
		Active:   true,
	}

	go func(t *ContractTimer) {
		<-t.timer.C
		u, _ := s.UserChannelCreate(t.UserID)
		_, _ = s.ChannelMessageSend(u.ID, t.Message)
		t.Active = false
	}(&t)

	// Save this timer for later
	contract.Timers = append(contract.Timers, t)

	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Content: "timer set",
		})

	saveData(Contracts)
}
