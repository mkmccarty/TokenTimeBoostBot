package boost

import (
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/xhit/go-str2duration/v2"
)

// GetSlashCoopTval calculates the coop token value of a running contract
func GetSlashCoopTval(cmd string) *dc.Command {
	command := guildOnlyCommand(cmd, "Get token value summary of entire coop.")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:        "duration",
			Description: "Total duration of this contract. Example: 19h35m.",
		},
	}
	return &command
}

// HandleCoopTvalCommand will handle the /contract-token-tval command. It is
// also called from /bump, which has already answered its own interaction, so
// every response here is skipped when the command name is "bump".
func HandleCoopTvalCommand(client dc.Client, e *dc.CommandEvent) {
	command := e.CommandName()

	channelID := e.ChannelID()
	contract := FindContract(channelID)
	if contract == nil {
		if command != "bump" {
			_ = e.Respond(dc.Message{
				Content:   "No contract found in this channel",
				Ephemeral: true,
			})
		}
		return
	}

	invalidDuration := false
	duration := contract.EstimatedDuration
	if opt, ok := e.OptString("duration"); ok {
		var err error
		// Timespan of the contract duration
		contractTimespan := bottools.SanitizeStringDuration(opt)
		duration, err = str2duration.ParseDuration(contractTimespan)
		if err != nil {
			duration = 12 * time.Hour
			invalidDuration = true
		} else {
			contract.EstimatedDuration = duration
			contract.EstimatedEndTime = contract.StartTime.Add(duration)
		}
	} else if contract.EstimatedDuration == 0 {
		c := ei.EggIncContractsAll[contract.ContractID]
		if c.ID != "" {
			duration = c.EstimatedDuration
		}
	}

	ephemeral := contract.CoopTokenValueMsgID != ""
	if command != "bump" {
		_ = e.Defer(ephemeral)
	}

	var builder strings.Builder
	BTA := math.Floor(duration.Minutes() / float64(contract.MinutesPerToken))
	targetTval := 3.0
	if BTA > 42.0 {
		targetTval = 0.07 * BTA
	}
	// Calculate the token value
	fmt.Fprintf(&builder, "## Coop token value based on contract reactions\n")
	fmt.Fprintf(&builder, "Contract started at: <t:%d:f> with a duration of %s\n", contract.StartTime.Unix(), duration.Round(time.Second))
	fmt.Fprintf(&builder, "Target token value: %6.3f\n\n", targetTval)
	builder.WriteString(calculateTokenValueCoopLog(contract, duration))

	fmt.Fprintf(&builder, "\nUpdated <t:%d:R>, refresh with %s\n", time.Now().Unix(), bottools.GetFormattedCommand("coop-tval"))

	if invalidDuration {
		fmt.Fprintf(&builder, "\n\n__Invalid duration used__\n")
		fmt.Fprintf(&builder, "**Defaulting to 12 hours**.\n")
		fmt.Fprintf(&builder, "Format should be entered like `19h35m` or `1d 2h 3m` or `1d2h3m` or `1d 2h")
	}

	if contract.CoopTokenValueMsgID != "" {
		strURL := "https://discordapp.com/channels/@me/" + channelID + "/" + contract.CoopTokenValueMsgID
		if command != "bump" {
			_ = e.Followup(dc.Message{
				Content: "Updated original response " + strURL,
			})
		}
		_, _ = client.EditMessage(channelID, contract.CoopTokenValueMsgID, dc.Message{Content: builder.String()})
		return
	}

	msg, err := e.FollowupMessage(dc.Message{Content: builder.String()})
	if err != nil {
		log.Println(err)
		return
	}
	contract.CoopTokenValueMsgID = msg.ID
	if err := client.PinMessage(channelID, msg.ID); err != nil {
		log.Println(err)
	}
}

func calculateTokenValueCoopLog(contract *Contract, duration time.Duration) string {
	tokenSent := make(map[string]int)
	tokensReceived := make(map[string]int)
	tokenValue := make(map[string]float64)
	tokenUser := make(map[string]bool)

	// Now we have a sorted list of future token logs
	for _, t := range contract.TokenLog {
		if t.FromUserID == t.ToUserID {
			// Farmed token, ignore
			continue
		}
		t.Value = bottools.GetTokenValue(t.Time.Sub(contract.StartTime).Seconds(), duration.Seconds())
		// Received tokens
		tokensReceived[t.ToNick] += t.Quantity
		tokenValue[t.ToNick] -= t.Value * float64(t.Quantity)
		// Sent tokens
		tokenSent[t.FromNick] += t.Quantity
		tokenValue[t.FromNick] += t.Value * float64(t.Quantity)

		tokenUser[t.ToNick] = true
		tokenUser[t.FromNick] = true
	}

	// Create a sorted list of keys from tokenCount
	keys := make([]string, 0, len(tokenUser))
	for key := range tokenUser {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return strings.ToLower(keys[i]) < strings.ToLower(keys[j])
	})

	headerStr := "`%-12s %3s %3s %6s`\n"
	formatStr := "`%s %3d %3d %6.2f`\n"
	var builder strings.Builder
	if len(keys) == 0 {
		fmt.Fprintf(&builder, "No tokens sent or received in this contract.\n")
	} else {
		fmt.Fprintf(&builder, headerStr, "Name", "Snd", "Rcv", "TVal-∆")

		// Iterate through the sorted keys
		for _, key := range keys {
			name := key

			fmt.Fprintf(&builder, formatStr, bottools.FitString(name, 12, bottools.StringAlignLeft), tokenSent[key], tokensReceived[key], tokenValue[key])
		}
	}
	return builder.String()
}
