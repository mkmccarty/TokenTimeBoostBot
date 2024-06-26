package boost

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/olekukonko/tablewriter"
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
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "details",
				Description: "Show individual token values. Default is false. (sticky)",
				Required:    false,
			},
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "alternate",
				Description:  "Select a linked alternate to show their token values",
				Required:     false,
				Autocomplete: true,
			},
		},
	}
}

// HandleAltsAutoComplete will populate with linked alternate names
func HandleAltsAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0)
	userID := getInteractionUserID(i)

	contract := FindContract(i.ChannelID)
	if contract != nil && contract.Boosters[userID] != nil {
		for _, name := range contract.Boosters[userID].Alts {
			choice := discordgo.ApplicationCommandOptionChoice{
				Name:  name,
				Value: name,
			}
			choices = append(choices, &choice)
		}
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Content: "Contract ID",
			Choices: choices,
		}})
}

// HandleContractCalcContractTvalCommand will handle the /contract-token-tval command
func HandleContractCalcContractTvalCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	// Call into boost module to do that calculations
	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}

	str := ""
	invalidDuration := false
	channelID := i.ChannelID
	contract := FindContract(channelID)
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
			duration = 12 * time.Hour
			invalidDuration = true
		}
	} else {
		if contract != nil {
			c := ei.EggIncContractsAll[contract.ContractID]
			if c.ID != "" {
				duration = c.EstimatedDuration
			}
		}
	}
	if opt, ok := optionMap["details"]; ok {
		details = opt.BoolValue()
		farmerstate.SetMiscSettingFlag(userID, "calc-details", details)
	} else {
		details = farmerstate.GetMiscSettingFlag(userID, "calc-details")
	}

	if opt, ok := optionMap["alternate"]; ok {
		userID = opt.StringValue()
	}

	if contract == nil {
		str = "No contract found in this channel"
	} else if !userInContract(contract, userID) {
		str = "You are not part of this contract"
	} else {
		BTA := duration.Minutes() / float64(contract.MinutesPerToken)
		targetTval := 3.0
		if BTA > 42.0 {
			targetTval = 0.07 * BTA
		}
		// Calculate the token value
		str = calculateTokenValue(contract.StartTime, duration, details, contract.Boosters[userID], targetTval)
		contract.CalcOperations++
		contract.CalcOperationTime = time.Now()
	}
	if invalidDuration {
		str += "\n\n__Invalid duration used__\n"
		str += "**Defaulting to 12 hours**.\n"
		str += "Format should be entered like `19h35m` or `1d 2h 3m` or `1d2h3m` or `1d 2h"
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: str,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	},
	)
}

func calculateTokenValue(startTime time.Time, duration time.Duration, details bool, booster *Booster, targetTval float64) string {
	// Calculate the token value
	var builder strings.Builder

	sentValue := 0.0
	receivedValue := 0.0

	fmt.Fprintf(&builder, "## %s token value for contract based on contract reactions\n", booster.Name)
	fmt.Fprintf(&builder, "### Contract started at: <t:%d:f> with a duration of %s\n", startTime.Unix(), duration.Round(time.Second))
	offsetTime := time.Since(startTime).Seconds()
	fmt.Fprintf(&builder, "> **Token value <t:%d:R> %1.3f**\n", time.Now().Unix(), getTokenValue(offsetTime, duration.Seconds()))
	fmt.Fprintf(&builder, "> Token value <t:%d:R>  %1.3f\n", time.Now().Add(30*time.Minute).Unix(), getTokenValue(offsetTime+(30*60), duration.Seconds()))
	fmt.Fprintf(&builder, "> Token value <t:%d:R>  %1.3f\n\n", time.Now().Add(60*time.Minute).Unix(), getTokenValue(offsetTime+(60*60), duration.Seconds()))

	if (len(booster.Sent) + len(booster.Received)) > 30 {
		details = false
	}
	// for each Token Sent, calculate the value
	if len(booster.TokensFarmedTime) != 0 {
		fmt.Fprintf(&builder, "**Tokens Farmed: %d**\n", len(booster.TokensFarmedTime))
		if details {
			for i, token := range booster.TokensFarmedTime {
				fmt.Fprintf(&builder, "> %d: %s\n", i+1, token.Sub(startTime).Round(time.Second))
			}
		}
	}
	if len(booster.Sent) != 0 {
		for i := range booster.Sent {
			booster.Sent[i].Value = getTokenValue(booster.Sent[i].Time.Sub(startTime).Seconds(), duration.Seconds())
			sentValue += booster.Sent[i].Value
		}
		fmt.Fprintf(&builder, "**Tokens Sent: %d for %4.3f**\n", len(booster.Sent), sentValue)
		if details {
			table := tablewriter.NewWriter(&builder)
			table.SetHeader([]string{"", "Time", "Value", "Recipient"})
			table.SetBorder(false)
			//table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
			//table.SetAlignment(tablewriter.ALIGN_LEFT)
			table.SetCenterSeparator("")
			table.SetColumnSeparator("")
			table.SetRowSeparator("")
			table.SetHeaderLine(false)
			table.SetTablePadding("\t") // pad with tabs
			table.SetNoWhiteSpace(true)
			fmt.Fprint(&builder, "```")
			for i := range booster.Sent {
				table.Append([]string{fmt.Sprintf("%d", i+1), booster.Sent[i].Time.Sub(startTime).Round(time.Second).String(), fmt.Sprintf("%6.3f", booster.Sent[i].Value), booster.Sent[i].UserID})

				//fmt.Fprintf(&builder, "> %d: %s  %6.3f %16s\n", i+1, booster.Sent[i].Time.Sub(startTime).Round(time.Second), booster.Sent[i].Value, booster.Sent[i].UserID)
			}
			table.Render()
			fmt.Fprint(&builder, "```")
		}
	}
	if len(booster.Received) != 0 {
		for i := range booster.Received {
			booster.Received[i].Value = getTokenValue(booster.Received[i].Time.Sub(startTime).Seconds(), duration.Seconds())
			receivedValue += booster.Received[i].Value
		}
		fmt.Fprintf(&builder, "**Token Received: %d for %4.3f**\n", len(booster.Received), receivedValue)
		if details {
			table := tablewriter.NewWriter(&builder)
			table.SetHeader([]string{"", "Time", "Value", "Sender"})
			table.SetBorder(false)
			//table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
			//table.SetAlignment(tablewriter.ALIGN_LEFT)
			table.SetCenterSeparator("")
			table.SetColumnSeparator("")
			table.SetRowSeparator("")
			table.SetHeaderLine(false)
			table.SetTablePadding("\t") // pad with tabs
			table.SetNoWhiteSpace(true)

			fmt.Fprint(&builder, "```")
			for i := range booster.Received {
				//fmt.Fprintf(&builder, "> %d: %s  %6.3f %16s\n", i+1, booster.Received[i].Time.Sub(startTime).Round(time.Second), booster.Received[i].Value, booster.Received[i].UserID)
				table.Append([]string{fmt.Sprintf("%d", i+1), booster.Received[i].Time.Sub(startTime).Round(time.Second).String(), fmt.Sprintf("%6.3f", booster.Received[i].Value), booster.Received[i].UserID})
			}
			table.Render()
			fmt.Fprint(&builder, "```")
		}
	}
	fmt.Fprintf(&builder, "\n**Current △ TVal %4.3f**  need %4.3f\n", sentValue-receivedValue, targetTval)

	if slices.Index(booster.Hint, "TokenRemove") == -1 {
		fmt.Fprintf(&builder, "\nThe `/token-remove` command can be used to adjust sent/received tokens.\n")
		booster.Hint = append(booster.Hint, "TokenRemove")
	}
	return builder.String()
}

func getTokenValue(seconds float64, durationSeconds float64) float64 {
	currentval := max(0.03, math.Pow(1-0.9*(min(seconds, durationSeconds)/durationSeconds), 4))

	return math.Round(currentval*1000) / 1000
}
