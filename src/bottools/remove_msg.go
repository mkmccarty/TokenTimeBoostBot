package bottools

import (
	"fmt"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// GetSlashRemoveMessage returns the slash command for removing a bot message from a DM channel.
func GetSlashRemoveMessage(cmd string) *dc.Command {
	command := dc.Command{
		Name:             cmd,
		Description:      "Remove BoostBot message from this DM channel.",
		Contexts:         []dc.InteractionContext{dc.ContextBotDM, dc.ContextPrivateChannel},
		IntegrationTypes: []dc.IntegrationType{dc.IntegrationGuildInstall, dc.IntegrationUserInstall},
		Options: []dc.Option{
			dc.StringOption{
				Name:        "message",
				Description: "Message Link or Message ID to remove.",
				Required:    true,
			},
		},
	}
	return &command
}

// HandleRemoveMessage handles the remove message command against the dc facade.
func HandleRemoveMessage(client dc.Client, e *dc.CommandEvent) {
	messageID := parseRemoveMessageID(e)

	_ = e.Defer(true)

	responseStr := removeBotMessage(client, e.ChannelID(), messageID)

	_ = e.Followup(dc.Message{Content: responseStr})
}

// parseRemoveMessageID extracts the message ID from the command's "message"
// option, which accepts either a bare message ID or a full message link.
func parseRemoveMessageID(e *dc.CommandEvent) string {
	v, ok := e.OptString("message")
	if !ok {
		return ""
	}
	// Timespan of the contract duration
	// https://discord.com/channels/@me/1124490885204287610/1276990861158256664
	// 1276990861158256664
	message := strings.TrimSpace(v)
	split := strings.Split(message, "/")
	return split[len(split)-1]
}

// removeBotMessage deletes the bot's own message identified by messageID in
// channelID and returns the human-readable outcome to report back to the user.
func removeBotMessage(client dc.Client, channelID, messageID string) string {
	responseStr := "Failed to remove message with ID: " + messageID
	msg, err := client.GetMessage(channelID, messageID)
	if err == nil && msg != nil {
		if msg.Author != nil && msg.Author.ID == config.DiscordAppID {
			if err := client.DeleteMessage(channelID, msg.ID); err == nil {
				responseStr = fmt.Sprintf("Removed message from <t:%d:f>.", msg.Timestamp.Unix())
			}
		} else {
			responseStr = fmt.Sprintf("The BoostBot can only remove its own messages. Message from <t:%d:f> was not removed.", msg.Timestamp.Unix())
		}
	}
	return responseStr
}
