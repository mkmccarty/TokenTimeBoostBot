package boost

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"

	"github.com/bwmarrin/discordgo"
	"github.com/xhit/go-str2duration/v2"
)

// GetSlashCoopTval calculates the coop token value of a running contract
func GetSlashCoopTval(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Get token value summary of entire coop.",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "duration",
				Description: "Total duration of this contract. Example: 19h35m.",
				Required:    false,
			},
		},
	}
}

// HandleCoopTvalCommand will handle the /contract-token-tval command
func HandleCoopTvalCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// User interacting with bot, is this first time ?
	command := i.ApplicationCommandData().Name

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}
	invalidDuration := false
	channelID := i.ChannelID
	contract := FindContract(channelID)
	var duration time.Duration
	if contract != nil {
		duration = contract.EstimatedDuration
	}
	if opt, ok := optionMap["duration"]; ok {
		var err error
		// Timespan of the contract duration
		contractTimespan := bottools.SanitizeStringDuration(opt.StringValue())
		duration, err = str2duration.ParseDuration(contractTimespan)
		if err != nil {
			duration = 12 * time.Hour
			invalidDuration = true
		} else {
			contract.EstimatedDuration = duration
			contract.EstimatedEndTime = contract.StartTime.Add(duration)
		}
	} else {
		if contract != nil {
			if contract.EstimatedDuration == 0 {
				c := ei.EggIncContractsAll[contract.ContractID]
				if c.ID != "" {
					duration = c.EstimatedDuration
				}
			}
		}
	}
	var builder strings.Builder
	if contract == nil {
		fmt.Fprintf(&builder, "No contract found in this channel")
	} else {
		flag := discordgo.MessageFlagsEphemeral
		if contract.CoopTokenValueMsgID == "" {
			flag = 0
		}

		if command != "bump" {
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Processing request...",
					Flags:   flag,
				},
			})
		}
		BTA := duration.Minutes() / float64(contract.MinutesPerToken)
		targetTval := 3.0
		if BTA > 42.0 {
			targetTval = 0.07 * BTA
		}
		// Calculate the token value
		fmt.Fprintf(&builder, "## Coop token value based on contract reactions\n")
		fmt.Fprintf(&builder, "Contract started at: <t:%d:f> with a duration of %s\n", contract.StartTime.Unix(), duration.Round(time.Second))
		fmt.Fprintf(&builder, "Target token value: %6.3f\n\n", targetTval)
		builder.WriteString(calculateTokenValueCoopLog(contract, duration, targetTval))

		fmt.Fprintf(&builder, "\nTokens remaining are based on average of 6 tokens per hour plus timer token.")
		fmt.Fprintf(&builder, "\nUpdated <t:%d:R>, refresh with %s\n", time.Now().Unix(), bottools.GetFormattedCommand("coop-tval"))

	}
	if invalidDuration {
		if invalidDuration {
			fmt.Fprintf(&builder, "\n\n__Invalid duration used__\n")
			fmt.Fprintf(&builder, "**Defaulting to 12 hours**.\n")
			fmt.Fprintf(&builder, "Format should be entered like `19h35m` or `1d 2h 3m` or `1d2h3m` or `1d 2h")
		}
	}

	if contract.CoopTokenValueMsgID != "" {
		strURL := "https://discordapp.com/channels/@me/" + i.ChannelID + "/" + contract.CoopTokenValueMsgID
		if command != "bump" {
			_, _ = s.FollowupMessageCreate(i.Interaction, true,
				&discordgo.WebhookParams{
					Content: "Updated original response " + strURL,
				})
		}
		//if err == nil {
		//	_ = s.FollowupMessageDelete(i.Interaction, msg.ID)
		//}
		_, _ = s.ChannelMessageEdit(i.ChannelID, contract.CoopTokenValueMsgID, builder.String())
	} else {
		msg, err := s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content: builder.String(),
			})
		if err != nil {
			fmt.Println(err)
		}
		if err == nil {
			contract.CoopTokenValueMsgID = msg.ID
			err = s.ChannelMessagePin(i.ChannelID, msg.ID)
			if err != nil {
				fmt.Println(err)
			}
		}
	}
}

func calculateTokenValueCoopLog(contract *Contract, duration time.Duration, tval float64) string {
	tokenSent := make(map[string]int)
	tokensReceived := make(map[string]int)
	tokenValue := make(map[string]float64)
	tokenUser := make(map[string]bool)
	ultraUser := make(map[string]bool)

	gg, ugg, _ := ei.GetGenerousGiftEvent()
	// Define thresholds for determining if a Generous Gift (GG) event is active
	const ggThreshold = 1.0

	// Check if either GG or Ultra GG exceeds their respective thresholds
	isGG := gg > ggThreshold || ugg > ggThreshold

	const maxFutureTokenLogEntries = 100 // Maximum number of future token log entries to process
	const rateSecondPerTokens = 592      // Rate at which tokens are generated
	// 1 token = 591.6 seconds / 9.86 minutes

	var crtTime time.Duration
	if !contract.TimeCRT.IsZero() {
		crtTime = contract.TimeBoosting.Sub(contract.TimeCRT)
	}

	futureTokenLog, futureTokenLogGG :=
		bottools.CalculateFutureTokenLogs(maxFutureTokenLogEntries, contract.StartTime, crtTime, contract.MinutesPerToken, duration, rateSecondPerTokens)

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
		if t.Quantity == 2 && ugg > ggThreshold {
			// Assuming this is a GG token
			ultraUser[t.FromNick] = true
		}

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

	headerStr := "`%-12s %3s %3s %6s %3s`\n"
	formatStr := "`%-12s %3d %3d %6.2f %3s`%s\n"
	var builder strings.Builder
	if len(keys) == 0 {
		fmt.Fprintf(&builder, "No tokens sent or received in this contract.\n")
	} else {
		fmt.Fprintf(&builder, headerStr, "Name", "Snd", "Rcv", "TVal-∆", "🪙#")

		// Iterate through the sorted keys
		for _, key := range keys {
			name := key
			var valueLog []bottools.FutureToken
			// test if ultraUser[key] exists
			ultra := false
			if _, ok := ultraUser[key]; ok {
				ultra = true
			}
			if isGG && ((ultra && ugg > ggThreshold) || (gg > ggThreshold)) {
				valueLog = futureTokenLogGG
			} else {
				valueLog = futureTokenLog
			}

			if len(name) > 12 {
				name = name[:12]
			}
			tcount, ttime, _ := bottools.CalculateTcountTtime(tokenValue[key], tval, valueLog)

			fmt.Fprintf(&builder, formatStr, name, tokenSent[key], tokensReceived[key], tokenValue[key], tcount, ttime)
		}
	}
	return builder.String()
}
