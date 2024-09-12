package boost

import (
	"fmt"
	"math"
	"slices"
	"sort"
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

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
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
	if contract != nil {
		duration = contract.EstimatedDuration
	}
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
		} else if contract != nil {
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
	if opt, ok := optionMap["details"]; ok {
		details = opt.BoolValue()
		farmerstate.SetMiscSettingFlag(userID, "calc-details", details)
	} else {
		details = farmerstate.GetMiscSettingFlag(userID, "calc-details")
	}

	if opt, ok := optionMap["alternate"]; ok {
		userID = opt.StringValue()
	}
	var embed *discordgo.MessageSend
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
		if contract.NewFeature == 1 {
			embed = calculateTokenValueFromLog(contract, duration, details, targetTval, userID)
		} else {
			str = calculateTokenValue(contract.StartTime, duration, details, contract.Boosters[userID], targetTval)
			if len(str) > 2000 {
				str = calculateTokenValue(contract.StartTime, duration, false, contract.Boosters[userID], targetTval)
				str += "> **Message too long, details disabled**\n"
			}
		}

	}
	if invalidDuration {
		str += "\n\n__Invalid duration used__\n"
		str += "**Defaulting to 12 hours**.\n"
		str += "Format should be entered like `19h35m` or `1d 2h 3m` or `1d2h3m` or `1d 2h"
	}

	if contract.NewFeature == 1 {
		_, _ = s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content: str,
				Embeds:  embed.Embeds,
			})
	} else {
		_, _ = s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content: str,
			})
	}
}

func calculateTokenValueFromLog(contract *Contract, duration time.Duration, details bool, targetTval float64, userID string) *discordgo.MessageSend {
	var description strings.Builder

	var totalHeader string
	var finalTotal string

	var TokensFarmed []TokenUnitLog // When each token was farmed
	var TokensSent []TokenUnitLog
	var TokensReceived []TokenUnitLog

	var SentValue float64
	var SentCount int
	var ReceivedValue float64
	var ReceivedCount int

	tokenCountTo := make(map[string]int)
	tokenValueTo := make(map[string]float64)
	tokenCountFrom := make(map[string]int)
	tokenValueFrom := make(map[string]float64)

	// Split the contract.TokenLog into Farmed, Sent and Received for this userID
	for _, tokens := range contract.TokenLog {
		if tokens.FromUserID == userID && tokens.ToUserID == userID {
			TokensFarmed = append(TokensFarmed, tokens)
		} else if tokens.FromUserID == userID {
			tokens.Value = getTokenValue(tokens.Time.Sub(contract.StartTime).Seconds(), duration.Seconds())
			tokens.Value *= float64(tokens.Quantity)
			TokensSent = append(TokensSent, tokens)
			SentValue += tokens.Value
			SentCount += tokens.Quantity
			tokenCountTo[tokens.ToNick] += tokens.Quantity
			tokenValueTo[tokens.ToNick] += tokens.Value
		} else if tokens.ToUserID == userID {
			tokens.Value = getTokenValue(tokens.Time.Sub(contract.StartTime).Seconds(), duration.Seconds())
			tokens.Value *= float64(tokens.Quantity)
			TokensReceived = append(TokensReceived, tokens)
			ReceivedValue += tokens.Value
			ReceivedCount += tokens.Quantity
			tokenCountFrom[tokens.FromNick] += tokens.Quantity
			tokenValueFrom[tokens.FromNick] += tokens.Value
		}
	}

	var field []*discordgo.MessageEmbedField

	URL := fmt.Sprintf("[%s](%s/%s/%s)", contract.CoopID, "https://eicoop-carpet.netlify.app", contract.ContractID, contract.CoopID)

	ts := duration.Round(time.Minute).String()
	fmt.Fprintf(&description, "Token tracking for **%s**\n", URL)
	fmt.Fprintf(&description, "Start time: <t:%d:t>\n", contract.StartTime.Unix())
	fmt.Fprintf(&description, "Duration  : **%s**\n", ts[:len(ts)-2])

	offsetTime := time.Since(contract.StartTime).Seconds()

	field = append(field, &discordgo.MessageEmbedField{
		Name:   fmt.Sprintf("Value <t:%d:R>", time.Now().Unix()),
		Value:  fmt.Sprintf("%1.3f\n", getTokenValue(offsetTime, duration.Seconds())),
		Inline: true,
	})
	field = append(field, &discordgo.MessageEmbedField{
		Name:   fmt.Sprintf("<t:%d:R>", time.Now().Add(30*time.Minute).Unix()),
		Value:  fmt.Sprintf("%1.3f\n", getTokenValue(offsetTime+(30*60), duration.Seconds())),
		Inline: true,
	})
	field = append(field, &discordgo.MessageEmbedField{
		Name:   fmt.Sprintf("<t:%d:R>", time.Now().Add(60*time.Minute).Unix()),
		Value:  fmt.Sprintf("%1.3f\n", getTokenValue(offsetTime+(60*60), duration.Seconds())),
		Inline: true,
	})

	if len(TokensFarmed) > 0 {
		var fbuilder strings.Builder
		fmt.Fprintf(&fbuilder, "%d", len(TokensFarmed))
		field = append(field, &discordgo.MessageEmbedField{
			Name:   "Farmed Tokens",
			Value:  fbuilder.String(),
			Inline: false,
		})
	}

	if len(TokensSent) > 0 {
		var sbuilder strings.Builder
		sentStr := "Sent Tokens"

		if len(TokensSent) > 20 {

			// Create a sorted list of keys from tokenCount
			keys := make([]string, 0, len(tokenCountTo))
			for key := range tokenCountTo {
				keys = append(keys, key)
			}
			sort.Strings(keys)

			// Iterate through the sorted keys
			for _, key := range keys {
				name := key
				if len(name) > 12 {
					name = name[:12]
				}
				fmt.Fprintf(&sbuilder, "> %s: %d %2.3f\n", name, tokenCountTo[key], tokenValueTo[key])
				if len(sbuilder.String()) > 1500 {
					break
				}
			}

			field = append(field, &discordgo.MessageEmbedField{
				Name:   "Sent Summary",
				Value:  sbuilder.String(),
				Inline: false,
			})
			sbuilder.Reset()

			// Trim tokens Sent to last 5
			sentStr = "Last 5 Sent Tokens"
			TokensSent = TokensSent[len(TokensSent)-5:]
		}

		fmt.Fprintf(&sbuilder, "%d valued at %4.3f\n", SentCount, SentValue)
		if details {
			for i, t := range TokensSent {
				id := t.ToNick
				quant := ""
				if t.Quantity > 1 {
					quant = fmt.Sprintf("x%d", t.Quantity)
				}
				fmt.Fprintf(&sbuilder, "> %d%s: <t:%d:R> %6.3f %s\n", i+1, quant, t.Time.Unix(), t.Value, id)

				if i > 0 && (i+1)%25 == 0 {
					field = append(field, &discordgo.MessageEmbedField{
						Name:   "Sent Tokens",
						Value:  sbuilder.String(),
						Inline: false,
					})
					sbuilder.Reset()
					sbuilder.WriteString("> \n")
				}
			}
		}
		field = append(field, &discordgo.MessageEmbedField{
			Name:   sentStr,
			Value:  sbuilder.String(),
			Inline: false,
		})
	}

	if len(TokensReceived) > 0 {
		var rbuilder strings.Builder
		recvStr := "Received Tokens"

		if len(TokensReceived) > 20 {
			// Create a sorted list of keys from tokenCount
			keys := make([]string, 0, len(tokenCountFrom))
			for key := range tokenCountFrom {
				keys = append(keys, key)
			}
			sort.Strings(keys)

			// Iterate through the sorted keys
			for _, key := range keys {
				name := key
				if len(name) > 12 {
					name = name[:12]
				}
				fmt.Fprintf(&rbuilder, "> %s: %d -%2.3f\n", name, tokenCountFrom[key], tokenValueFrom[key])
				if len(rbuilder.String()) > 1500 {
					break
				}
			}

			field = append(field, &discordgo.MessageEmbedField{
				Name:   "Received Summary",
				Value:  rbuilder.String(),
				Inline: false,
			})
			rbuilder.Reset()

			// Trim tokens Sent to last 5
			recvStr = "Last 5 Received Tokens"
			TokensReceived = TokensReceived[len(TokensReceived)-5:]
		}
		fmt.Fprintf(&rbuilder, "%d valued at %4.3f\n", ReceivedCount, ReceivedValue)
		if details {
			for i, t := range TokensReceived {
				id := t.FromNick
				quant := ""
				if t.Quantity > 1 {
					quant = fmt.Sprintf("x%d", t.Quantity)
				}
				fmt.Fprintf(&rbuilder, "> %d%s: <t:%d:R> %6.3f %s\n", i+1, quant, t.Time.Unix(), t.Value, id)
				if i > 0 && (i+1)%25 == 0 {
					field = append(field, &discordgo.MessageEmbedField{
						Name:   "Received Tokens",
						Value:  rbuilder.String(),
						Inline: false,
					})
					rbuilder.Reset()
					rbuilder.WriteString("> \n")
				}
			}
		}

		field = append(field, &discordgo.MessageEmbedField{
			Name:   recvStr,
			Value:  rbuilder.String(),
			Inline: false,
		})
	}

	totalHeader = "Current △ TVal"
	finalTotal = fmt.Sprintf("%4.3f", SentValue-ReceivedValue)
	field = append(field, &discordgo.MessageEmbedField{
		Name:   totalHeader,
		Value:  finalTotal,
		Inline: true,
	})

	field = append(field, &discordgo.MessageEmbedField{
		Name:   "Target TVal",
		Value:  fmt.Sprintf("%4.3f", targetTval),
		Inline: true,
	})

	footerStr := "For the most accurate values make sure total contract time is accurate."

	embed := &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{{
			Type:        discordgo.EmbedTypeRich,
			Title:       "Token Tracking",
			Description: description.String(),
			Color:       0xeedd00,
			Fields:      field,
			Footer: &discordgo.MessageEmbedFooter{
				Text: footerStr,
			},
		},
		},
	}

	return embed
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
				name := booster.Sent[i].UserID
				if len(name) > 12 {
					name = name[:12] + "…"
				}
				table.Append([]string{fmt.Sprintf("%d", i+1), booster.Sent[i].Time.Sub(startTime).Round(time.Second).String(), fmt.Sprintf("%6.3f", booster.Sent[i].Value), name})

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
				name := booster.Received[i].UserID
				if len(name) > 12 {
					name = name[:12] + "…"
				}
				//fmt.Fprintf(&builder, "> %d: %s  %6.3f %16s\n", i+1, booster.Received[i].Time.Sub(startTime).Round(time.Second), booster.Received[i].Value, booster.Received[i].UserID)
				table.Append([]string{fmt.Sprintf("%d", i+1), booster.Received[i].Time.Sub(startTime).Round(time.Second).String(), fmt.Sprintf("%6.3f", booster.Received[i].Value), name})
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
