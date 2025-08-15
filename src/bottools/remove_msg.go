package bottools

import (
	"fmt"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"

	"github.com/bwmarrin/discordgo"
)

// GetSlashRemoveMessage returns the slash command for removing a bot message from a DM channel.
func GetSlashRemoveMessage(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Remove BoostBot message from this DM channel.",
		Contexts: &[]discordgo.InteractionContextType{
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
				Name:        "message",
				Description: "Message Link or Message ID to remove.",
				Required:    true,
			},
		},
	}
}

// HandleRemoveMessageCommand handles the remove message command.
func HandleRemoveMessageCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	var messageID string
	var responseStr string
	if opt, ok := optionMap["message"]; ok {
		// Timespan of the contract duration
		// https://discord.com/channels/@me/1124490885204287610/1276990861158256664
		// 1276990861158256664
		message := strings.TrimSpace(opt.StringValue())
		split := strings.Split(message, "/")
		messageID = split[len(split)-1]
	}

	responseStr = "Failed to remove message with ID: " + messageID
	msg, err := s.ChannelMessage(i.ChannelID, messageID)
	if err == nil {
		time := msg.Timestamp
		if msg.Author.ID == config.DiscordAppID {
			err := s.ChannelMessageDelete(i.ChannelID, msg.ID)
			if err == nil {
				responseStr = fmt.Sprintf("Removed message from <t:%d:f>.", time.Unix())
			}
		} else {
			responseStr = fmt.Sprintf("The BoostBot can only remove its own messages. Message from <t:%d:f> was not removed.", time.Unix())
		}
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Content: responseStr,
		})
}
