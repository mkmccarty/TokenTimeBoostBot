package boost

import (
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
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
	command := i.ApplicationCommandData().Name

	optionMap := bottools.GetCommandOptionsMap(i)

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
			log.Println(err)
		}
		if err == nil {
			contract.CoopTokenValueMsgID = msg.ID
			err = s.ChannelMessagePin(i.ChannelID, msg.ID)
			if err != nil {
				log.Println(err)
			}
		}
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
