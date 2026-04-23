package boost

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
)

// GetSlashDashboardCommand returns the /dashboard slash command definition
func GetSlashDashboardCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Show your personal BoostBot dashboard (active contracts, timers)",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
			discordgo.InteractionContextBotDM,
			discordgo.InteractionContextPrivateChannel,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
			discordgo.ApplicationIntegrationUserInstall,
		},
	}
}

// HandleDashboardCommand handles the /dashboard command
func HandleDashboardCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := getInteractionUserID(i)

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	var builder strings.Builder
	builder.WriteString("# 📊 Your BoostBot Dashboard\n\n")

	// Active Contracts
	builder.WriteString("## 🚀 Active Contracts\n")
	contractCount := 0
	for _, c := range Contracts {
		if userInContract(c, userID) {
			contractCount++
			channelStr := "Unknown Channel"
			if len(c.Location) > 0 {
				channelStr = fmt.Sprintf("<#%s>", c.Location[0].ChannelID)
			}

			timeStr := "TBD"
			if !c.EstimatedEndTime.IsZero() {
				timeStr = fmt.Sprintf("<t:%d:R>", c.EstimatedEndTime.Unix())
			} else if c.State == ContractStateSignup {
				timeStr = "In Sign-up"
			}

			fmt.Fprintf(&builder, "> **%s/%s** in %s - Completion: %s\n", c.ContractID, c.CoopID, channelStr, timeStr)
		}
	}
	if contractCount == 0 {
		builder.WriteString("> You are not in any active contracts.\n")
	}

	// Active Timers
	var timerBuilder strings.Builder
	timerCount := 0
	timersMutex.Lock()
	now := time.Now()
	for _, t := range timers {
		if t.UserID == userID && now.Before(t.Reminder) {
			timerCount++
			displayMessage := t.Message
			if t.OriginalChannelID != "" {
				displayMessage = fmt.Sprintf("%s in <#%s>", displayMessage, t.OriginalChannelID)
			}
			fmt.Fprintf(&timerBuilder, "> <t:%d:R> %s\n", t.Reminder.Unix(), displayMessage)
		}
	}
	timersMutex.Unlock()

	if timerCount > 0 {
		builder.WriteString("\n## ⏱️ Active Timers\n")
		builder.WriteString(timerBuilder.String())
	}

	// Command Links
	builder.WriteString("\n## 🔗 Useful Commands\n")
	stonesCmd := bottools.GetFormattedCommand("stones")
	if stonesCmd == "" {
		stonesCmd = "`/stones`"
	}
	csEstimateCmd := bottools.GetFormattedCommand("cs-estimate")
	if csEstimateCmd == "" {
		csEstimateCmd = "`/cs-estimate`"
	}
	timerCmd := bottools.GetFormattedCommand("timer")
	if timerCmd == "" {
		timerCmd = "`/timer`"
	}

	fmt.Fprintf(&builder, "> %s - Optimal stone set for running contract\n", stonesCmd)
	fmt.Fprintf(&builder, "> %s - Contract Score estimates\n", csEstimateCmd)
	fmt.Fprintf(&builder, "> %s - Set a DM reminder timer\n", timerCmd)

	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: builder.String(),
		Flags:   discordgo.MessageFlagsEphemeral,
	})
}
