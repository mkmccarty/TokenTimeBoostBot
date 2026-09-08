package boost

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/xhit/go-str2duration/v2"
)

// GetSlashCalcContractTval calculates the callers token value of a running contract
func GetSlashCalcContractTval(cmd string) *dc.Command {
	command := guildOnlyCommand(cmd, "Calculate token values of current running contract")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:        "duration",
			Description: "Total duration of this contract. Example: 19h35m.",
		},
		dc.BoolOption{
			Name:        "details",
			Description: "Show individual token values. Default is false. (sticky)",
		},
		dc.StringOption{
			Name:         "alternate",
			Description:  "Select a linked alternate to show their token values",
			Autocomplete: true,
		},
	}
	return &command
}

// HandleAltsAutoComplete will populate with linked alternate names
func HandleAltsAutoComplete(e *dc.AutocompleteEvent) {
	choices := make([]dc.Choice[string], 0)
	userID := e.UserID()

	contract := FindContract(e.ChannelID())
	if contract != nil && contract.Boosters[userID] != nil {
		for _, name := range contract.Boosters[userID].Alts {
			choices = append(choices, dc.Choice[string]{
				Name:  name,
				Value: name,
			})
		}
	}

	_ = e.RespondChoices(choices)
}

// HandleContractCalcContractTvalCommand will handle the /contract-token-tval command
func HandleContractCalcContractTvalCommand(e *dc.CommandEvent) {
	_ = e.Defer(true)

	// Call into boost module to do that calculations
	userID := e.UserID()

	str := ""
	invalidDuration := false
	channelID := e.ChannelID()
	contract := FindContract(channelID)
	var duration time.Duration
	if contract != nil {
		duration = contract.EstimatedDuration
	}
	details := false
	if opt, ok := e.OptString("duration"); ok {
		var err error
		// Timespan of the contract duration
		contractTimespan := bottools.SanitizeStringDuration(opt)
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
	if opt, ok := e.OptBool("details"); ok {
		details = opt
		farmerstate.SetMiscSettingFlag(userID, "calc-details", details)
	} else {
		details = farmerstate.GetMiscSettingFlag(userID, "calc-details")
	}

	if opt, ok := e.OptString("alternate"); ok {
		userID = opt
	}
	var embed *dc.Embed
	if contract == nil {
		str = "No contract found in this channel"
	} else if !UserInContract(contract, userID) {
		str = "You are not part of this contract"
	} else {
		targetTval := GetTargetTval(contract.SeasonalScoring, duration.Minutes(), float64(contract.MinutesPerToken))
		// Calculate the token value
		if targetTval == 0.0 {
			str += "This contract does not have a target token value requirement.\n"
		} else {
			embed = calculateTokenValueFromLog(contract, duration, details, targetTval, userID)
		}

	}
	if invalidDuration {
		str += "\n\n__Invalid duration used__\n"
		str += "**Defaulting to 12 hours**.\n"
		str += "Format should be entered like `19h35m` or `1d 2h 3m` or `1d2h3m` or `1d 2h"
	}

	message := dc.Message{Content: str}
	if embed != nil {
		message.Embeds = []dc.Embed{*embed}
	}
	_ = e.Followup(message)
}

func calculateTokenValueFromLog(contract *Contract, duration time.Duration, details bool, targetTval float64, userID string) *dc.Embed {
	var description strings.Builder

	var totalHeader string
	var finalTotal string

	var TokensFarmed []ei.TokenUnitLog // When each token was farmed
	var TokensSent []ei.TokenUnitLog
	var TokensReceived []ei.TokenUnitLog

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
			tokens.Value = bottools.GetTokenValue(tokens.Time.Sub(contract.StartTime).Seconds(), duration.Seconds())
			tokens.Value *= float64(tokens.Quantity)
			TokensSent = append(TokensSent, tokens)
			SentValue += tokens.Value
			SentCount += tokens.Quantity
			tokenCountTo[tokens.ToNick] += tokens.Quantity
			tokenValueTo[tokens.ToNick] += tokens.Value
		} else if tokens.ToUserID == userID {
			tokens.Value = bottools.GetTokenValue(tokens.Time.Sub(contract.StartTime).Seconds(), duration.Seconds())
			tokens.Value *= float64(tokens.Quantity)
			TokensReceived = append(TokensReceived, tokens)
			ReceivedValue += tokens.Value
			ReceivedCount += tokens.Quantity
			tokenCountFrom[tokens.FromNick] += tokens.Quantity
			tokenValueFrom[tokens.FromNick] += tokens.Value
		}
	}

	var field []dc.EmbedField

	URL := fmt.Sprintf("[%s](%s/%s/%s)", contract.CoopID, "https://eicoop-carpet.netlify.app", contract.ContractID, contract.CoopID)

	ts := duration.Round(time.Minute).String()
	fmt.Fprintf(&description, "Token tracking for **%s**\n", URL)
	fmt.Fprintf(&description, "Start time: <t:%d:t>\n", contract.StartTime.Unix())
	fmt.Fprintf(&description, "Duration  : **%s**\n", ts[:len(ts)-2])

	offsetTime := time.Since(contract.StartTime).Seconds()

	field = append(field, dc.EmbedField{
		Name:   fmt.Sprintf("Value <t:%d:R>", time.Now().Unix()),
		Value:  fmt.Sprintf("%1.3f\n", bottools.GetTokenValue(offsetTime, duration.Seconds())),
		Inline: true,
	})
	field = append(field, dc.EmbedField{
		Name:   fmt.Sprintf("<t:%d:R>", time.Now().Add(30*time.Minute).Unix()),
		Value:  fmt.Sprintf("%1.3f\n", bottools.GetTokenValue(offsetTime+(30*60), duration.Seconds())),
		Inline: true,
	})
	field = append(field, dc.EmbedField{
		Name:   fmt.Sprintf("<t:%d:R>", time.Now().Add(60*time.Minute).Unix()),
		Value:  fmt.Sprintf("%1.3f\n", bottools.GetTokenValue(offsetTime+(60*60), duration.Seconds())),
		Inline: true,
	})

	if len(TokensFarmed) > 0 {
		var fbuilder strings.Builder
		farmedCount := 0
		for _, t := range TokensFarmed {
			farmedCount += t.Quantity
		}

		fmt.Fprintf(&fbuilder, "%d", farmedCount)
		field = append(field, dc.EmbedField{
			Name:   "Farmed Tokens",
			Value:  fbuilder.String(),
			Inline: false,
		})
	}

	if len(TokensSent) > 0 {
		var sbuilder strings.Builder
		sentStr := "Sent Tokens"
		sentOffset := 0

		if len(TokensSent) > 15 {

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

			field = append(field, dc.EmbedField{
				Name:   "Sent Summary",
				Value:  sbuilder.String(),
				Inline: false,
			})
			sbuilder.Reset()

			// Trim tokens Sent to last 5
			sentStr = "Last 5 Sent Tokens"
			sentOffset = len(TokensSent) - 5
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
				fmt.Fprintf(&sbuilder, "> %d%s: <t:%d:R> %6.3f %s\n", i+1+sentOffset, quant, t.Time.Unix(), t.Value, id)

				if i > 0 && (i+1)%25 == 0 {
					field = append(field, dc.EmbedField{
						Name:   "Sent Tokens",
						Value:  sbuilder.String(),
						Inline: false,
					})
					sbuilder.Reset()
					sbuilder.WriteString("> \n")
				}
			}
		}
		field = append(field, dc.EmbedField{
			Name:   sentStr,
			Value:  sbuilder.String(),
			Inline: false,
		})
	}

	if len(TokensReceived) > 0 {
		var rbuilder strings.Builder
		recvStr := "Received Tokens"
		recvOffset := 0

		if len(TokensReceived) > 15 {
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

			field = append(field, dc.EmbedField{
				Name:   "Received Summary",
				Value:  rbuilder.String(),
				Inline: false,
			})
			rbuilder.Reset()

			// Trim tokens Sent to last 5
			recvStr = "Last 5 Received Tokens"
			recvOffset = len(TokensReceived) - 5
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
				fmt.Fprintf(&rbuilder, "> %d%s: <t:%d:R> %6.3f %s\n", i+1+recvOffset, quant, t.Time.Unix(), t.Value, id)
				if i > 0 && (i+1)%25 == 0 {
					field = append(field, dc.EmbedField{
						Name:   "Received Tokens",
						Value:  rbuilder.String(),
						Inline: false,
					})
					rbuilder.Reset()
					rbuilder.WriteString("> \n")
				}
			}
		}

		field = append(field, dc.EmbedField{
			Name:   recvStr,
			Value:  rbuilder.String(),
			Inline: false,
		})
	}

	totalHeader = "Current △ TVal"
	finalTotal = fmt.Sprintf("%4.3f", SentValue-ReceivedValue)
	contract.Boosters[userID].TokenValue = SentValue - ReceivedValue
	field = append(field, dc.EmbedField{
		Name:   totalHeader,
		Value:  finalTotal,
		Inline: true,
	})

	field = append(field, dc.EmbedField{
		Name:   "Target TVal",
		Value:  fmt.Sprintf("%4.3f", targetTval),
		Inline: true,
	})

	// Show Token Teamwork score vs max.
	myTeamwork := calculateTokenTeamwork(contract.EstimatedDuration.Seconds(), contract.MinutesPerToken, SentValue, ReceivedValue)
	maxTeamwork := calculateTokenTeamwork(contract.EstimatedDuration.Seconds(), contract.MinutesPerToken, 1000, 8)
	field = append(field, dc.EmbedField{
		Name:   "Teamwork Value",
		Value:  fmt.Sprintf("%4.3g / %4.3g", myTeamwork, maxTeamwork),
		Inline: true,
	})

	footerStr := "For the most accurate values make sure total contract time is accurate."

	embed := &dc.Embed{
		Title:       "Token Tracking",
		Description: description.String(),
		Color:       0xeedd00,
		Fields:      field,
		Footer: &dc.EmbedFooter{
			Text: footerStr,
		},
	}

	return embed
}
