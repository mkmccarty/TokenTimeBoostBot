package boost

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
)

// GetSlashRenameThread is the definition of the slash command
func GetSlashRenameThread(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Rename Boost Bot created contract thread.",
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
				Name:        "thread-name",
				Description: "The name of the thread. Enter `help` for more information.",
				Required:    true,
			},
		},
	}
}

// HandleRenameThreadCommand will handle the thread rename command
func HandleRenameThreadCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	//var builder strings.Builder
	optionMap := bottools.GetCommandOptionsMap(i)

	var threadName string
	var builder strings.Builder

	if opt, ok := optionMap["thread-name"]; ok {
		threadName = opt.StringValue()
	}

	setName := true

	// Command will only work in a thread
	ch, err := s.Channel(i.ChannelID)
	if err != nil || !ch.IsThread() {
		fmt.Fprint(&builder, "This command can only be used in a thread, ")
		setName = false
	}
	// Requires a contract
	c := FindContract(i.ChannelID)
	if c == nil {
		fmt.Fprint(&builder, "There is no contract in this thread.")
		setName = false
	} else {
		userID := getInteractionUserID(i)
		// if member is not the contract owner or in the contract, then return
		if !creatorOfContract(s, c, userID) && slices.Index(c.Order, userID) == -1 {
			fmt.Fprint(&builder, "This command can only be used by the contract owner or a member of the contract.")
			setName = false
		}
	}

	if setName {

		if strings.HasPrefix(threadName, "help") {
			// Provide help on the command
			fmt.Fprint(&builder, "You can use the following variables in the thread name:\n")
			fmt.Fprint(&builder, "$NAME, $N - The name of the contract\n")
			fmt.Fprint(&builder, "$COUNT, $C  - Signup count of the contract\n")
			fmt.Fprint(&builder, "$STYLE, $S - The style of the contract\n")
			fmt.Fprint(&builder, "$TIME, $T - The start time of the contract, If time not set will be TBD\n")
			fmt.Fprint(&builder, "\n")
			fmt.Fprint(&builder, "clear - Clear the thread name and use the default\n")
		} else if strings.HasPrefix(threadName, "clear") {
			c.ThreadName = ""
			fmt.Fprint(&builder, "The thread name has been cleared and will use the default\n")
		} else {
			c.ThreadName = threadName
			fmt.Fprintf(&builder, "The thread will use your string:\n> %s\n", threadName)
			fmt.Fprintf(&builder, "> %s", generateThreadName(c))
			fmt.Fprint(&builder, "\nUse the 🌊 reaction to rename the thread.")
		}

		if c.ThreadName != "" {
			fmt.Fprintf(&builder, "\nThe thread name is currently set to:\n> %s", c.ThreadName)
		}
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    builder.String(),
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})
}

func generateThreadName(c *Contract) string {
	var threadName = c.ThreadName
	threadStyleIcons := []string{"", "🟦 ", "🟩 ", "🟧 ", "🟥 "}
	if threadName == "" {
		threadName = "$N $C"
		if !c.PlannedStartTime.IsZero() && c.State == ContractStateSignup {
			threadName += " $T"
		}
	}
	threadColor := threadStyleIcons[c.PlayStyle]
	if strings.Contains(threadName, "$STYLE") || strings.Contains(threadName, "$S") {
		var styleStr string
		if c.Style&ContractFlagBanker != 0 {
			styleStr += "Banker"
		} else {
			styleStr += "Fastrun"
		}
		threadName = strings.ReplaceAll(threadName, "$STYLE", styleStr)
		threadName = strings.ReplaceAll(threadName, "$S", styleStr)
	}

	if strings.Contains(threadName, "$TIME") || strings.Contains(threadName, "$T") {
		if !c.PlannedStartTime.IsZero() && c.State == ContractStateSignup {
			nyTime, err := time.LoadLocation("America/New_York")
			if err == nil {
				currentTime := c.PlannedStartTime.In(nyTime)

				// Format the current time as a string
				formattedTime := currentTime.Format("3:04pm MST")

				// Append the formatted time to the thread name
				threadName = strings.ReplaceAll(threadName, "$TIME", formattedTime)
				threadName = strings.ReplaceAll(threadName, "$T", formattedTime)
			}
		} else if c.State == ContractStateSignup {
			threadName = strings.ReplaceAll(threadName, "$TIME", "TBD")
			threadName = strings.ReplaceAll(threadName, "$T", "TBD")
		}
	}
	// Calculate COUNT replacement first to determine accurate length
	var statusStr string
	if strings.Contains(threadName, "$COUNT") || strings.Contains(threadName, "$C") {
		playStyleStr := ""
		if c.PlayStyle != ContractPlaystyleUnset && c.PlayStyle < len(contractPlaystyleNames) {
			playStyleStr = fmt.Sprintf("%s ", contractPlaystyleNames[c.PlayStyle])
		}
		if len(c.Order) != c.CoopSize {
			statusStr = fmt.Sprintf("(%s%d/%d)", playStyleStr, len(c.Order), c.CoopSize)
		} else {
			statusStr = "(FULL)"
		}
	}

	// Check if we need to trim the CoopID to keep total length under 90
	const maxLength = 90
	coopID := c.CoopID

	// Create a temporary version with all replacements to check length
	tempName := strings.ReplaceAll(threadName, "$NAME", coopID)
	tempName = strings.ReplaceAll(tempName, "$N", coopID)
	tempName = strings.ReplaceAll(tempName, "$COUNT", statusStr)
	tempName = strings.ReplaceAll(tempName, "$C", statusStr)

	fullLength := len(threadColor) + len(tempName)
	if fullLength >= maxLength {
		// Need to trim the CoopID
		excess := fullLength - maxLength + 1 // +1 for safety margin
		if len(coopID) > excess+3 {          // +3 for ellipsis
			coopID = coopID[:len(coopID)-excess-3] + "..."
		} else if len(coopID) > 3 {
			coopID = "..."
		}
	}

	threadName = strings.ReplaceAll(threadName, "$NAME", coopID)
	threadName = strings.ReplaceAll(threadName, "$N", coopID)
	threadName = strings.ReplaceAll(threadName, "$COUNT", statusStr)
	threadName = strings.ReplaceAll(threadName, "$C", statusStr)

	return threadColor + threadName
}
