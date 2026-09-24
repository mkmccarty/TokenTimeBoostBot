package boost

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// manualAddThreadMemberDelay is the duration to wait before adding a manually added farmer to a thread.
var manualAddThreadMemberDelay = 1 * time.Minute

// AddThreadMemberDelayed waits for manualAddThreadMemberDelay (or the specified delay)
// and checks if the farmer has already joined the thread. If not, it adds them.
func AddThreadMemberDelayed(client dc.Client, contractHash string, channelID, userID string, delay time.Duration) {
	if client == nil || channelID == "" || !dc.IsSnowflake(userID) {
		return
	}
	if delay <= 0 {
		delay = manualAddThreadMemberDelay
	}

	time.AfterFunc(delay, func() {
		// Verify contract still exists and user is still in the contract
		contract := FindContractByHash(contractHash)
		if contract == nil {
			return
		}
		contract.mutex.Lock()
		if contract.State == ContractStateSignup || !UserInContract(contract, userID) {
			contract.mutex.Unlock()
			return
		}
		contract.mutex.Unlock()

		// Check if user is already in the thread
		member, err := client.ThreadMember(channelID, userID)
		if err == nil && member != nil {
			return // Already in the thread
		}

		_ = client.AddThreadMember(channelID, userID)
	})
}

// AddBoostersToThread adds all contract boosters and creators to the contract thread(s)
// when the contract is not in signup mode.
func AddBoostersToThread(client dc.Client, contract *Contract) {
	if client == nil || contract == nil || contract.State == ContractStateSignup {
		return
	}
	for _, el := range contract.Location {
		if el == nil || el.ChannelID == "" {
			continue
		}
		for userID := range contract.Boosters {
			if dc.IsSnowflake(userID) {
				_ = client.AddThreadMember(el.ChannelID, userID)
			}
		}
		for _, creatorID := range contract.CreatorID {
			if dc.IsSnowflake(creatorID) && creatorID != config.DiscordAppID {
				_ = client.AddThreadMember(el.ChannelID, creatorID)
			}
		}
	}
}

// GetSlashRenameThread is the definition of the slash command
func GetSlashRenameThread(cmd string) *dc.Command {
	command := guildOnlyCommand(cmd, "Rename Boost Bot created contract thread.")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:        "thread-name",
			Description: "The name of the thread. Enter `help` for more information.",
			Required:    true,
		},
	}
	return &command
}

// HandleRenameThreadCommand will handle the thread rename command
func HandleRenameThreadCommand(client dc.Client, e *dc.CommandEvent) {
	var threadName string
	var builder strings.Builder

	if opt, ok := e.OptString("thread-name"); ok {
		threadName = opt
	}

	setName := true

	// Command will only work in a thread
	ch, err := client.Channel(e.ChannelID())
	if err != nil || !ch.IsThread {
		fmt.Fprint(&builder, "This command can only be used in a thread, ")
		setName = false
	}
	// Requires a contract
	c := FindContract(e.ChannelID())
	if c == nil {
		fmt.Fprint(&builder, "There is no contract in this thread.")
		setName = false
	} else {
		userID := e.UserID()
		// if member is not the contract owner or in the contract, then return
		if !creatorOfContract(client, c, userID) && slices.Index(c.Order, userID) == -1 {
			fmt.Fprint(&builder, "This command can only be used by the contract owner or a member of the contract.")
			setName = false
		}
	}

	if setName {

		if strings.HasPrefix(threadName, "help") {
			// Provide help on the command
			fmt.Fprint(&builder, "You can use the following variables in the thread name:\n")
			fmt.Fprint(&builder, "$ID - The ContractID of the contract\n")
			fmt.Fprint(&builder, "$NAME, $N - The CoopID of the contract\n")
			fmt.Fprint(&builder, "$COUNT, $C  - Signup count of the contract\n")
			fmt.Fprint(&builder, "$STYLE, $S - The style of the contract\n")
			fmt.Fprint(&builder, "$TIME, $T - The start time of the contract, If time not set will be TBD\n")
			fmt.Fprint(&builder, "\n")
			fmt.Fprint(&builder, "clear - Clear the thread name and use the default\n")
		} else if strings.HasPrefix(threadName, "clear") {
			c.ThreadName = ""
			c.ThreadRenameFinalized = false
			fmt.Fprint(&builder, "The thread name has been cleared and will use the default\n")
		} else {
			c.ThreadName = threadName
			c.ThreadRenameFinalized = false
			fmt.Fprintf(&builder, "The thread will use your string:\n> %s\n", threadName)
			fmt.Fprintf(&builder, "> %s", generateThreadName(c))
			fmt.Fprint(&builder, "\nUse the 🌊 reaction to rename the thread.")
		}

		if c.ThreadName != "" {
			fmt.Fprintf(&builder, "\nThe thread name is currently set to:\n> %s", c.ThreadName)
		}

		if time.Since(c.ThreadRenameTime) < ThreadRenameCooldown {
			fmt.Fprintf(&builder, "\n\n⚠️ Thread renaming is on cooldown until <t:%d:R>.", c.ThreadRenameTime.Add(ThreadRenameCooldown).Unix())
		}
	}

	_ = e.Respond(dc.Message{
		Content:   builder.String(),
		Ephemeral: true,
	})
}

func generateThreadName(c *Contract) string {
	var threadName = c.ThreadName
	threadStyleIcons := []string{"", "🟦 ", "🟩 ", "🟧 ", "🟥 "}
	if threadName == "" {
		threadName = "$N $C"
		// Special case for Eggscape
		if c.Location[0].GuildID == "1457709786597560515" {
			threadName = "$ID/$N $C"
		}
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
		if !c.PredictionSignup {
			if len(c.Boosters) != c.CoopSize {
				statusStr = fmt.Sprintf("(%s%d/%d)", playStyleStr, len(c.Boosters), c.CoopSize)
			} else {
				statusStr = "(FULL)"
			}
		} else {
			statusStr = fmt.Sprintf("(%s%d)", playStyleStr, len(c.Boosters))
		}
	}

	// Check if we need to trim the CoopID to keep total length under 90
	const maxLength = 90
	coopID := c.CoopID

	var nReplacement string
	var idReplacement string

	if !c.PredictionSignup {
		nReplacement = coopID
		idReplacement = c.ContractID
	} else {
		nReplacement = fmt.Sprintf("%s %s", c.Name, "Signup")
		idReplacement = ""
	}

	// Create a temporary version with all replacements to check length
	tempName := strings.ReplaceAll(threadName, "$NAME", nReplacement)
	tempName = strings.ReplaceAll(tempName, "$ID", idReplacement)
	tempName = strings.ReplaceAll(tempName, "$N", nReplacement)
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
		if !c.PredictionSignup {
			nReplacement = coopID
		}
	}

	threadName = strings.ReplaceAll(threadName, "$NAME", nReplacement)
	threadName = strings.ReplaceAll(threadName, "$N", nReplacement)
	threadName = strings.ReplaceAll(threadName, "$ID", idReplacement)
	threadName = strings.ReplaceAll(threadName, "$COUNT", statusStr)
	threadName = strings.ReplaceAll(threadName, "$C", statusStr)

	return threadColor + threadName
}
