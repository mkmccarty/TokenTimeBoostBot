package boost

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xhit/go-str2duration/v2"
)

// GetSlashCalcContractTval calculates the callers token value of a running contract
func GetSlashCalcContractTval(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Calculate token values of current running contract",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "duration",
				Description: "Total duration of this contract. Example: 19h35m.",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "details",
				Description: "Show individual token values. Default is false.",
				Required:    false,
			},
		},
	}
}

// HandleContractCalcContractTvalCommand will handle the /contract-token-tval command
func HandleContractCalcContractTvalCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}
	channelID := i.ChannelID
	var duration time.Duration
	details := false
	if opt, ok := optionMap["duration"]; ok {
		var err error
		// Timespan of the contract duration
		contractTimespan := strings.TrimSpace(opt.StringValue())
		contractTimespan = strings.Replace(contractTimespan, "day", "d", -1)
		contractTimespan = strings.Replace(contractTimespan, "hr", "h", -1)
		contractTimespan = strings.Replace(contractTimespan, "min", "m", -1)
		contractTimespan = strings.Replace(contractTimespan, "sec", "s", -1)
		// replace all spaces with nothing
		contractTimespan = strings.Replace(contractTimespan, " ", "", -1)
		duration, err = str2duration.ParseDuration(contractTimespan)
		if err != nil {
			// Invalid duration, just assigning a 12h
			duration = 12 * time.Hour
		}
	}
	if opt, ok := optionMap["details"]; ok {
		details = opt.BoolValue()
	}

	// Call into boost module to do that calculations
	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}

	str := ""
	// Find the contract
	contract := FindContract(channelID)
	if contract == nil {
		str = "No contract found in this channel"
	} else {
		// Is user in this contract ?
		if !userInContract(contract, userID) {
			str = "You are not part of this contract"
		} else {
			// Calculate the token value
			str = calculateTokenValue(contract.StartTime, duration, details, contract.Boosters[userID])
		}

	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: str,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	},
	)
}

func calculateTokenValue(startTime time.Time, duration time.Duration, details bool, booster *Booster) string {
	// Calculate the token value
	var builder strings.Builder

	sentValue := 0.0
	receivedValue := 0.0

	fmt.Fprint(&builder, "## Calculating Token Value for Contract based on your contract reactions\n")
	fmt.Fprintf(&builder, "### Contract started at: <t:%d:f> with a duration of %s\n", startTime.Unix(), duration.Round(time.Second))
	offsetTime := time.Since(startTime).Seconds()
	fmt.Fprintf(&builder, "> **Current token value: %1.3f**\n", getTokenValue(offsetTime, duration.Seconds()))
	fmt.Fprintf(&builder, "> Token value in 30 minutes: %1.3f\n", getTokenValue(offsetTime+(30*60), duration.Seconds()))
	fmt.Fprintf(&builder, "> Token value in one hour: %1.3f\n\n", getTokenValue(offsetTime+(60*60), duration.Seconds()))

	// for each Token Sent, calculate the value
	if len(booster.TokensFarmedTime) != 0 {
		fmt.Fprintf(&builder, "**Tokens Farmed: %d**\n", len(booster.TokensFarmedTime))
		if details {
			for i, token := range booster.TokensFarmedTime {
				fmt.Fprintf(&builder, "> %d: %s\n", i+1, token.Sub(startTime).Round(time.Second))
			}
		}
	}
	if len(booster.TokenSentTime) != 0 {
		sval := make([]float64, len(booster.TokenSentTime))
		for i, token := range booster.TokenSentTime {
			sval[i] = getTokenValue(token.Sub(startTime).Seconds(), duration.Seconds())
			sentValue += sval[i]
		}
		fmt.Fprintf(&builder, "**Tokens Sent: %d for %4.3f**\n", len(booster.TokenSentTime), sentValue)
		if details {
			for i, token := range booster.TokenSentTime {
				fmt.Fprintf(&builder, "> %d: %s  %6.3f\n", i+1, token.Sub(startTime).Round(time.Second), sval[i])
			}
		}
	}
	if len(booster.TokenReceivedTime) != 0 {
		rval := make([]float64, len(booster.TokenReceivedTime))

		for i, token := range booster.TokenReceivedTime {
			rval[i] = getTokenValue(token.Sub(startTime).Seconds(), duration.Seconds())
			receivedValue += rval[i]
		}
		fmt.Fprintf(&builder, "**Token Received: %d for %4.3f**\n", len(booster.TokenReceivedTime), receivedValue)
		if details {
			for i, token := range booster.TokenReceivedTime {
				fmt.Fprintf(&builder, "> %d: %s  %6.3f\n", i+1, token.Sub(startTime).Round(time.Second), rval[i])
			}
		}
	}
	fmt.Fprintf(&builder, "\n** △ TVal %4.3f**\n", sentValue-receivedValue)

	fmt.Fprintf(&builder, "ᵀʳᵃᶜᵏᵉʳ ᵘˢᵉˢ ᵈᶦˢᶜᵒʳᵈ ᶦⁿᵗᵉʳᵃᶜᵗᶦᵒⁿˢ ᵃⁿᵈ ʳᵉᵃᶜᵗᶦᵒⁿˢ ᵗᵒ ᵗʳᵃᶜᵏ ᵗᵒᵏᵉⁿˢ. ᶠᵒʳ ᵗʰᵉ ᵐᵒˢᵗ ᵃᶜᶜᵘʳᵃᵗᵉ ᵛᵃˡᵘᵉˢ ᵐᵃᵏᵉ ˢᵘʳᵉ ᵗʰᵉ ˢᵗᵃʳᵗ ᵗᶦᵐᵉ ᵃⁿᵈ ᵗᵒᵗᵃˡ ᶜᵒⁿᵗʳᵃᶜᵗ ᵗᶦᵐᵉ ᶦˢ ᵃᶜᶜᵘʳᵃᵗᵉ")
	return builder.String()
}

func getTokenValue(seconds float64, durationSeconds float64) float64 {
	currentval := max(0.03, math.Pow(1-0.9*(min(seconds, durationSeconds)/durationSeconds), 4))

	return math.Round(currentval*1000) / 1000
}
