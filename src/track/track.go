package track

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/peterbourgon/diskv/v3"
	"github.com/rs/xid"
)

// Constants for token tracking
const (
	TokenSent     = 0 // TokenSent is a token sent
	TokenReceived = 1 // TokenReceived is a token received
)

// TokenUnit holds everything we need to know about a token
type TokenUnit struct {
	Time     time.Time // Time the token was sent or received
	Value    float64   // Calculated value of the token * Quantity
	UserID   string    // Who sent or received the token
	Serial   string    // Serial number of the token
	Quantity int       // Quantity of the tokens
}

type tokenValue struct {
	UserID             string        // The user ID that is tracking the token value
	Username           string        // The username that is tracking the token value
	Name               string        // Tracking name for this contract
	ChannelID          string        // The channel ID that is tracking the token value
	ContractID         string        // The contract ID
	CoopID             string        // The coop ID
	Linked             bool          // If the tracker is linked to channel contract
	LinkedCompleted    bool          // If the linked tracker has a completed contract
	LinkReceived       bool          // If linked, log the received tokens
	ChannelMention     string        // The channel mention
	StartTime          time.Time     // When Token Value time started
	EstimatedEndTime   time.Time     // Time of Token Value time plus Duration
	DurationTime       time.Duration // Duration of Token Value time
	TimeFromCoopStatus time.Time     // If the time was set from the coop status
	Sent               []TokenUnit
	SentCount          int
	Received           []TokenUnit
	ReceivedCount      int
	FarmedTokenTime    []time.Time // time a self farmed token was received
	SumValueSent       float64     // sum of all token values sent
	SumValueReceived   float64     // sum of all token values received
	TokenDelta         float64     // difference between sent and received
	TokenMessageID     string      // Message ID for the Last Token Value message
	UserChannelID      string      // User Channel ID for the Last Token Value message
	Details            bool        // Show details of each token sent
	MinutesPerToken    int         // Used for token value calculation
}

type tokenValues struct {
	Coop map[string]*tokenValue
}

var (
	// Tokens is a map of contracts and is saved to disk
	Tokens    map[string]*tokenValues // map is UserID
	dataStore *diskv.Diskv
)

func init() {
	dataStore = diskv.New(diskv.Options{
		BasePath:          "ttbb-data",
		AdvancedTransform: advancedTransform,
		InverseTransform:  inverseTransform,
		CacheSizeMax:      512 * 512,
	})
	Tokens = make(map[string]*tokenValues)

	var t, err = loadData()
	if err == nil {
		Tokens = t
	}
}

func resetTokenTracking(tv *tokenValue) {
	tv.StartTime = time.Now()
	tv.Linked = true
	tv.LinkedCompleted = false
	tv.LinkReceived = true
	tv.Sent = nil
	tv.Received = nil
	tv.SumValueSent = 0.0
	tv.SumValueReceived = 0.0
	tv.TokenDelta = 0.0
	tv.Details = false
}

// SetTokenTrackingDetails will toggle the details for token tracking
func SetTokenTrackingDetails(userID string, name string) {
	td, err := getTrack(userID, name)
	if err != nil {
		return
	}
	td.Details = !td.Details
}

// TokenTrackingAdjustTime will adjust the time values for a contract
func TokenTrackingAdjustTime(channelID string, userID string, name string, startHour int, startMinute int, endHour int, endMinute int, startTime time.Time, duration float64) *discordgo.MessageSend {
	td, err := getTrack(userID, name)
	if err != nil {
		return nil
	}
	if startTime != (time.Time{}) && duration != 0 {
		td.StartTime = startTime
		td.DurationTime = time.Duration(duration) * time.Second
		td.EstimatedEndTime = startTime.Add(td.DurationTime)
	} else {
		td.StartTime = td.StartTime.Add(time.Duration(startHour) * time.Hour)
		td.StartTime = td.StartTime.Add(time.Duration(startMinute) * time.Minute)

		td.DurationTime = max(0, td.DurationTime+(time.Duration(endHour)*time.Hour))
		td.DurationTime = max(0, td.DurationTime+(time.Duration(endMinute)*time.Minute))

		td.EstimatedEndTime = td.StartTime.Add(td.DurationTime)
	}

	// Changed duration needs a recalculation
	td.SumValueSent = 0.0
	for i, t := range td.Sent {
		now := t.Time
		offsetTime := now.Sub(td.StartTime).Seconds()
		td.Sent[i].Value = getTokenValue(offsetTime, td.DurationTime.Seconds()) * float64(t.Quantity)
		td.SumValueSent += td.Sent[i].Value
	}
	td.SumValueReceived = 0.0
	for i, t := range td.Received {
		now := t.Time
		offsetTime := now.Sub(td.StartTime).Seconds()
		td.Received[i].Value = getTokenValue(offsetTime, td.DurationTime.Seconds()) * float64(t.Quantity)
		td.SumValueReceived += td.Received[i].Value
	}
	td.TokenDelta = td.SumValueSent - td.SumValueReceived

	return getTokenTrackingEmbed(td, false)
}

func getTokenTrackingString(td *tokenValue, finalDisplay bool) string {
	var builder strings.Builder
	ts := td.DurationTime.Round(time.Minute).String()
	if finalDisplay {
		fmt.Fprintf(&builder, "# Final Token tracking for %s\n", td.Name)
	} else {
		fmt.Fprintf(&builder, "# Token tracking for %s\n", td.Name)
	}
	if td.Linked {
		fmt.Fprint(&builder, "Linked Contract: ", td.ChannelMention, "\n")
	} else {
		fmt.Fprint(&builder, "Contract Channel: ", td.ChannelMention, "\n")
	}
	fmt.Fprintf(&builder, "Start time: <t:%d:t>\n", td.StartTime.Unix())
	if td.TimeFromCoopStatus.IsZero() {
		fmt.Fprintf(&builder, "Duration  : **%s** Estimate\n", ts[:len(ts)-2])
	} else {
		fmt.Fprintf(&builder, "Duration  : **%s**  retrieved <t:%d:r>\n", ts[:len(ts)-2], td.TimeFromCoopStatus.Unix())
	}

	return builder.String()
}

func getTokenTrackingEmbed(td *tokenValue, finalDisplay bool) *discordgo.MessageSend {
	var description strings.Builder
	var linkedHdr strings.Builder

	var totalHeader string
	var finalTotal string

	var field []*discordgo.MessageEmbedField

	URL := fmt.Sprintf("[%s](%s/%s/%s)", td.CoopID, "https://eicoop-carpet.netlify.app", td.ContractID, td.CoopID)

	ts := td.DurationTime.Round(time.Minute).String()
	if finalDisplay {
		fmt.Fprintf(&description, "Final Token tracking for **%s**\n", URL)
	} else {
		fmt.Fprintf(&description, "Token tracking for **%s**\n", URL)
	}
	fmt.Fprintf(&description, "Start time: <t:%d:t>\n", td.StartTime.Unix())
	if td.TimeFromCoopStatus.IsZero() {
		fmt.Fprintf(&description, "Duration (Estimate): **%s** \n", ts[:len(ts)-2])
	} else {
		fmt.Fprintf(&description, "Duration (<t:%d:R>): **%s**\n", td.TimeFromCoopStatus.Unix(), ts[:len(ts)-2])
	}

	if td.Linked {
		fmt.Fprint(&linkedHdr, "Linked Contract")
	} else {
		fmt.Fprint(&linkedHdr, "Contract Channel")
	}
	field = append(field, &discordgo.MessageEmbedField{
		Name:   linkedHdr.String(),
		Value:  td.ChannelMention,
		Inline: false,
	})

	if !finalDisplay {
		//var tokenValues strings.Builder

		offsetTime := time.Since(td.StartTime).Seconds()

		field = append(field, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("Value <t:%d:R>", time.Now().Unix()),
			Value:  fmt.Sprintf("%1.3f\n", getTokenValue(offsetTime, td.DurationTime.Seconds())),
			Inline: true,
		})
		field = append(field, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("<t:%d:R>", time.Now().Add(30*time.Minute).Unix()),
			Value:  fmt.Sprintf("%1.3f\n", getTokenValue(offsetTime+(30*60), td.DurationTime.Seconds())),
			Inline: true,
		})
		field = append(field, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("<t:%d:R>", time.Now().Add(60*time.Minute).Unix()),
			Value:  fmt.Sprintf("%1.3f\n", getTokenValue(offsetTime+(60*60), td.DurationTime.Seconds())),
			Inline: true,
		})

	}

	if len(td.FarmedTokenTime) > 0 {
		var fbuilder strings.Builder
		/*
			if td.Details {
				for i := range td.FarmedTokenTime {
					if !finalDisplay {
						fmt.Fprintf(&fbuilder, "%d: <t:%d:R>\n", i+1, td.FarmedTokenTime[i].Unix())
					} else {
						fmt.Fprintf(&fbuilder, "%d: %s\n", i+1, td.FarmedTokenTime[i].Sub(td.StartTime).Round(time.Second))
					}
				}
			} else {
				fmt.Fprintf(&fbuilder, "%d", len(td.FarmedTokenTime))
			}
		*/
		fmt.Fprintf(&fbuilder, "%d", len(td.FarmedTokenTime))
		field = append(field, &discordgo.MessageEmbedField{
			Name:   "Farmed Tokens",
			Value:  fbuilder.String(),
			Inline: false,
		})
	}

	if len(td.Sent) > 0 {
		var sbuilder strings.Builder
		brief := false
		if len(td.Received) > 20 {
			brief = true
		}
		if len(td.Sent) != td.SentCount {
			// Indicates that this was a banker user
			brief = false
		}

		fmt.Fprintf(&sbuilder, "%d valued at %4.3f\n", td.SentCount, td.SumValueSent)
		if td.Details {
			for i, t := range td.Sent {
				id := td.Sent[i].UserID
				quant := ""
				if t.Quantity > 1 {
					quant = fmt.Sprintf("x%d", t.Quantity)
				}

				if !brief || (len(td.Sent) != td.SentCount && t.Quantity > 1) {
					if !finalDisplay {
						fmt.Fprintf(&sbuilder, "> %d%s: <t:%d:R> %6.3f %s\n", i+1, quant, t.Time.Unix(), t.Value, id)
					} else {
						fmt.Fprintf(&sbuilder, "> %d%s: %s  %6.3f %s\n", i+1, quant, t.Time.Sub(td.StartTime).Round(time.Second), t.Value, id)
					}
				} else {
					if !finalDisplay {
						fmt.Fprintf(&sbuilder, "> %d%s: %6.3f\n", i+1, quant, t.Value)
					} else {
						fmt.Fprintf(&sbuilder, "> %d%s: %6.3f\n", i+1, quant, t.Value)
					}
				}
				if i > 0 && (i+1)%25 == 0 {
					field = append(field, &discordgo.MessageEmbedField{
						Name:   "Sent Tokens",
						Value:  sbuilder.String(),
						Inline: brief,
					})
					sbuilder.Reset()
					sbuilder.WriteString("> \n")
				}
			}
		}
		field = append(field, &discordgo.MessageEmbedField{
			Name:   "Sent Tokens",
			Value:  sbuilder.String(),
			Inline: brief,
		})
	}

	if len(td.Received) > 0 {
		var rbuilder strings.Builder
		brief := false
		if len(td.Received) > 20 {
			brief = true
		}

		fmt.Fprintf(&rbuilder, "%d valued at %4.3f\n", td.ReceivedCount, td.SumValueReceived)
		if td.Details {
			for i, t := range td.Received {
				id := t.UserID
				quant := ""
				if t.Quantity > 1 {
					quant = fmt.Sprintf("x%d", t.Quantity)
				}

				if !brief || (len(td.Received) != td.ReceivedCount && t.Quantity > 1) {
					if !finalDisplay {
						fmt.Fprintf(&rbuilder, "> %d%s: <t:%d:R> %6.3f %s\n", i+1, quant, t.Time.Unix(), t.Value, id)
					} else {
						fmt.Fprintf(&rbuilder, "> %d%s: %s  %6.3f %s\n", i+1, quant, t.Time.Sub(td.StartTime).Round(time.Second), t.Value, id)
					}
				} else {
					if !finalDisplay {
						fmt.Fprintf(&rbuilder, "> %d%s: %6.3f\n", i+1, quant, t.Value)
					} else {
						fmt.Fprintf(&rbuilder, "> %d%s: %6.3f\n", i+1, quant, t.Value)
					}
				}
				if i > 0 && (i+1)%25 == 0 {
					field = append(field, &discordgo.MessageEmbedField{
						Name:   "Received Tokens",
						Value:  rbuilder.String(),
						Inline: brief,
					})
					rbuilder.Reset()
					rbuilder.WriteString("> \n")
				}
			}

			if td.LinkReceived && !finalDisplay {
				fmt.Fprint(&rbuilder, "\nReact with 1️⃣..🔟 to remove errant received tokens at that index. The bot cannot remove your DM reactions.\n")
			}
		}

		field = append(field, &discordgo.MessageEmbedField{
			Name:   "Received Tokens",
			Value:  rbuilder.String(),
			Inline: brief,
		})

	}
	totalHeader = "Current △ TVal"
	if finalDisplay {
		totalHeader = "Final △ TVal"
	}
	finalTotal = fmt.Sprintf("%4.3f", td.TokenDelta)
	field = append(field, &discordgo.MessageEmbedField{
		Name:   totalHeader,
		Value:  finalTotal,
		Inline: true,
	})

	if td.MinutesPerToken != 0 {
		BTA := td.DurationTime.Minutes() / float64(td.MinutesPerToken)
		targetTval := 3.0
		if BTA > 42.0 {
			targetTval = 0.07 * BTA
		}
		field = append(field, &discordgo.MessageEmbedField{
			Name:   "Target TVal",
			Value:  fmt.Sprintf("%4.3f", targetTval),
			Inline: true,
		})
	}

	footerStr := "For the most accurate values make sure the start time and total contract time is accurate."
	if td.Linked {
		footerStr += " Linked contracts will automatically track tokens sent and received through discord message reactions. "
	}
	footerStr += " After 4 days any tracker not marked as finished will be purged."

	embed := &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{{
			Type:        discordgo.EmbedTypeRich,
			Title:       "Token Tracking",
			Description: description.String(),
			Color:       0xffaa00,
			Fields:      field,
			Footer: &discordgo.MessageEmbedFooter{
				Text: footerStr,
			},
		},
		},
	}

	return embed
}

func getTrack(userID string, name string) (*tokenValue, error) {
	if Tokens[userID] == nil {
		Tokens[userID] = new(tokenValues)
	}
	if Tokens[userID].Coop == nil || Tokens[userID].Coop[name] == nil {
		Tokens[userID].Coop = make(map[string]*tokenValue)
		Tokens[userID].Coop[name] = new(tokenValue)
		Tokens[userID].Coop[name].UserID = userID
		resetTokenTracking(Tokens[userID].Coop[name])
		Tokens[userID].Coop[name].Name = name
		Tokens[userID].Coop[name].CoopID = name
	}
	return Tokens[userID].Coop[name], nil
}

// TokenTracking is called as a starting point for token tracking
func tokenTracking(s *discordgo.Session, channelID string, userID string, name string, contractID string, duration time.Duration, linked bool, linkReceived bool, startTime time.Time, pastTokens *[]ei.TokenUnitLog) (string, *discordgo.MessageSend, error) {
	if Tokens[userID] == nil {
		Tokens[userID] = new(tokenValues)
	}
	if Tokens[userID].Coop == nil {
		Tokens[userID].Coop = make(map[string]*tokenValue)
	}
	if Tokens[userID].Coop[name] == nil {
		Tokens[userID].Coop[name] = new(tokenValue)
		Tokens[userID].Coop[name].UserID = userID
		u, err := s.User(userID)
		if err != nil {
			Tokens[userID].Coop[name].Username = "<@" + userID + ">"
		} else {
			Tokens[userID].Coop[name].Username = u.GlobalName
		}

		resetTokenTracking(Tokens[userID].Coop[name])
		Tokens[userID].Coop[name].Name = name
	} else {
		// Existing contract, reset the values
		_ = s.ChannelMessageDelete(Tokens[userID].Coop[name].UserChannelID, Tokens[userID].Coop[name].TokenMessageID)
		resetTokenTracking(Tokens[userID].Coop[name])
		Tokens[userID].Coop[name].Name = name
	}

	td, err := getTrack(userID, name)
	if err != nil {
		return "", nil, err
	}

	td.StartTime = startTime

	td.ChannelID = channelID // Last channel gets responses
	td.ChannelMention = fmt.Sprintf("<#%s>", channelID)

	// Set the duration
	td.DurationTime = duration
	td.EstimatedEndTime = time.Now().Add(duration)
	td.Linked = linked
	td.LinkReceived = linkReceived
	td.ContractID = contractID
	td.CoopID = name
	td.MinutesPerToken = 0
	td.SentCount = 0
	td.ReceivedCount = 0

	if contractID != "" {
		td.ContractID = contractID
		c := ei.EggIncContractsAll[contractID]
		if c.ID != "" {
			td.MinutesPerToken = c.MinutesPerToken
		}
	}

	if pastTokens != nil {
		for _, t := range *pastTokens {
			if t.FromUserID == userID && t.ToUserID == userID {
				td.FarmedTokenTime = append(td.FarmedTokenTime, t.Time)
				continue
			}

			if t.FromUserID == userID {
				value := getTokenValue(t.Time.Sub(td.StartTime).Seconds(), td.DurationTime.Seconds()) * float64(t.Quantity)
				td.Sent = append(td.Sent, TokenUnit{Time: t.Time, Value: value, UserID: t.ToNick, Serial: t.Serial, Quantity: t.Quantity})
				td.SumValueSent += value
				td.SentCount += t.Quantity
			} else if t.ToUserID == userID {
				value := getTokenValue(t.Time.Sub(td.StartTime).Seconds(), td.DurationTime.Seconds()) * float64(t.Quantity)
				td.Received = append(td.Received, TokenUnit{Time: t.Time, Value: value, UserID: t.FromNick, Serial: t.Serial, Quantity: t.Quantity})
				td.SumValueReceived += value
				td.ReceivedCount += t.Quantity
			}
		}
		td.TokenDelta = td.SumValueSent - td.SumValueReceived
	}

	// Recalculate the token values just in case there was a previous /token command
	// and for some reason it's run a multiple times.
	td.SumValueSent = 0
	td.SumValueReceived = 0
	td.SentCount = 0
	td.ReceivedCount = 0
	for i, t := range td.Sent {
		td.SentCount += t.Quantity
		td.SumValueSent += td.Sent[i].Value
	}
	for i, t := range td.Received {
		td.ReceivedCount += t.Quantity
		td.SumValueReceived += td.Received[i].Value
	}

	return getTokenTrackingString(td, false), getTokenTrackingEmbed(td, false), nil
}

// tokenTrackingTrack is called to track tokens sent and received
func tokenTrackingTrack(userID string, name string, tokenSent int, tokenReceived int) (*discordgo.MessageSend, bool) {
	td, err := getTrack(userID, name)
	if err != nil {
		return nil, false
	}
	now := time.Now()
	offsetTime := now.Sub(td.StartTime).Seconds()
	tokenValue := getTokenValue(offsetTime, td.DurationTime.Seconds())

	if tokenSent > 0 {
		td.Sent = append(td.Sent, TokenUnit{Time: now, Value: tokenValue * float64(tokenSent), UserID: td.Username, Quantity: tokenSent, Serial: xid.New().String()})
		td.SentCount += tokenSent
		td.SumValueSent += tokenValue * float64(tokenSent)
	}
	if tokenReceived > 0 {
		td.Received = append(td.Received, TokenUnit{Time: now, Value: tokenValue * float64(tokenReceived), UserID: td.Username, Quantity: tokenReceived, Serial: xid.New().String()})
		td.ReceivedCount += tokenReceived
		td.SumValueReceived += tokenValue * float64(tokenReceived)
	}
	td.TokenDelta = td.SumValueSent - td.SumValueReceived

	return getTokenTrackingEmbed(td, false), td.Linked && !td.LinkedCompleted
}

func getTokenValue(seconds float64, durationSeconds float64) float64 {
	currentval := max(0.03, math.Pow(1-0.9*(min(seconds, durationSeconds)/durationSeconds), 4))

	return math.Round(currentval*1000) / 1000
}

// extractTokenName will extract the token name from the message component
func extractTokenNameOriginal(comp discordgo.MessageComponent) string {
	jsonBlob, _ := discordgo.Marshal(comp)
	stage1 := string(jsonBlob[:])
	stage2 := strings.Split(stage1, "{")[5]
	stage3 := strings.Split(stage2, ",")[0]
	stage4 := strings.Split(stage3, ":")[1]
	// extract string from test2 until the backslash
	return stage4[1 : len(stage4)-1]
}

func extractTokenName(customID string) string {
	// name is the part of the CustomID after the first #
	name := strings.Split(customID, "#")
	if len(name) == 1 {
		return ""
	}
	return name[1]
}

/*
func syncTokenTracking(name string, startTime time.Time, duration time.Duration) {
	for _, v := range Tokens {
		if v.Coop[name] != nil {
			v.Coop[name].StartTime = startTime
			v.Coop[name].DurationTime = duration
			v.Coop[name].EstimatedEndTime = startTime.Add(duration)
		}
	}
}
*/

// UnlinkTokenTracking will unlink the token tracking from the channel
func UnlinkTokenTracking(s *discordgo.Session, channelID string) {

	for userID, t := range Tokens {
		for _, v := range t.Coop {
			if v != nil && v.ChannelID == channelID && v.Linked {
				Tokens[userID].Coop[v.Name].LinkedCompleted = true
				saveData(Tokens)
				embed := getTokenTrackingEmbed(v, false)
				comp := getTokenValComponents(v.Name, false)
				m := discordgo.NewMessageEdit(v.UserChannelID, v.TokenMessageID)
				m.Components = &comp
				m.SetEmbeds(embed.Embeds)
				m.SetContent("")
				_, _ = s.ChannelMessageEditComplex(m)
			}
		}
	}
}

// FarmedToken will track the token sent from the contract Token reaction
func FarmedToken(s *discordgo.Session, channelID string, userID string) {
	if Tokens[userID] == nil {
		return
	}

	for _, v := range Tokens[userID].Coop {
		if v != nil && v.ChannelID == channelID && v.Linked {
			v.FarmedTokenTime = append(v.FarmedTokenTime, time.Now())
			saveData(Tokens)
			embed := getTokenTrackingEmbed(v, false)
			comp := getTokenValComponents(v.Name, v.Linked && !v.LinkedCompleted)
			m := discordgo.NewMessageEdit(v.UserChannelID, v.TokenMessageID)
			m.Components = &comp
			m.SetEmbeds(embed.Embeds)
			m.SetContent("")
			_, _ = s.ChannelMessageEditComplex(m)
		}
	}
}

// ContractTokenUpdate will remove a token from the tracking list
func ContractTokenUpdate(s *discordgo.Session, channelID string, modifyToken *ei.TokenUnitLog) {
	redraw := false

	var track *tokenValue

	// FromUserID - Modify Token
	if Tokens[modifyToken.FromUserID] != nil {
		for _, v := range Tokens[modifyToken.FromUserID].Coop {
			if v != nil && v.ChannelID == channelID && v.Linked {
				for i, t := range v.Sent {
					if t.Serial == modifyToken.Serial {
						track = v
						v.SumValueSent -= t.Value
						v.SentCount -= t.Quantity
						Tokens[modifyToken.FromUserID].Coop[v.Name].Sent[i] = TokenUnit{Time: t.Time, Value: t.Value, UserID: modifyToken.ToNick, Serial: modifyToken.Serial, Quantity: modifyToken.Quantity}
						Tokens[modifyToken.FromUserID].Coop[v.Name].Sent[i].Value = getTokenValue(t.Time.Sub(v.StartTime).Seconds(), v.DurationTime.Seconds()) * float64(modifyToken.Quantity)
						v.SumValueSent += Tokens[modifyToken.FromUserID].Coop[v.Name].Sent[i].Value
						v.SentCount += t.Quantity
						redraw = true
						break
					}
				}
			}
		}
	} else if Tokens[modifyToken.ToUserID] != nil {
		for _, v := range Tokens[modifyToken.ToUserID].Coop {
			if v != nil && v.ChannelID == channelID && v.Linked {
				for i, t := range v.Received {
					if t.Serial == modifyToken.Serial {
						track = v
						v.SumValueReceived -= t.Value
						v.ReceivedCount -= t.Quantity
						Tokens[modifyToken.FromUserID].Coop[v.Name].Received[i] = TokenUnit{Time: t.Time, Value: t.Value, UserID: modifyToken.FromNick, Serial: modifyToken.Serial, Quantity: modifyToken.Quantity}
						Tokens[modifyToken.FromUserID].Coop[v.Name].Received[i].Value = getTokenValue(t.Time.Sub(v.StartTime).Seconds(), v.DurationTime.Seconds()) * float64(modifyToken.Quantity)
						v.SumValueReceived += Tokens[modifyToken.FromUserID].Coop[v.Name].Received[i].Value
						v.ReceivedCount += t.Quantity
						redraw = true
						break
					}
				}

			}
		}
	}

	if redraw {
		track.TokenDelta = track.SumValueSent - track.SumValueReceived
		saveData(Tokens)
		embed := getTokenTrackingEmbed(track, false)
		comp := getTokenValComponents(track.Name, track.Linked && !track.LinkedCompleted)
		m := discordgo.NewMessageEdit(track.UserChannelID, track.TokenMessageID)
		m.Components = &comp
		m.SetEmbeds(embed.Embeds)
		m.SetContent("")
		_, _ = s.ChannelMessageEditComplex(m)
	}

}

// ContractTokenMessage will track the token sent from the contract Token reaction
func ContractTokenMessage(s *discordgo.Session, channelID string, userID string, kind int, count int, actorUserID string, serialID string) {
	if Tokens[userID] == nil {
		return
	}
	log.Printf("ContractTokenMessage: %s %d %d %s\n", userID, kind, count, actorUserID)
	redraw := false
	for _, v := range Tokens[userID].Coop {
		if v != nil && v.ChannelID == channelID && v.Linked {
			now := time.Now()
			offsetTime := now.Sub(v.StartTime).Seconds()
			tokenValue := getTokenValue(offsetTime, v.DurationTime.Seconds())
			tokenValue *= float64(count)
			if kind == TokenSent {
				v.Sent = append(v.Sent, TokenUnit{Time: now, Value: tokenValue, UserID: actorUserID, Serial: serialID, Quantity: count})
				v.SumValueSent += tokenValue
				v.SentCount += count
				redraw = true
			} else if v.LinkReceived && kind == TokenReceived {
				v.Received = append(v.Received, TokenUnit{Time: now, Value: tokenValue, UserID: actorUserID, Serial: serialID, Quantity: count})
				v.SumValueReceived += tokenValue
				v.ReceivedCount += count
				redraw = true
			}
			if redraw {
				v.TokenDelta = v.SumValueSent - v.SumValueReceived
				saveData(Tokens)
				embed := getTokenTrackingEmbed(v, false)
				comp := getTokenValComponents(v.Name, v.Linked && !v.LinkedCompleted)
				m := discordgo.NewMessageEdit(v.UserChannelID, v.TokenMessageID)
				m.Components = &comp
				m.SetEmbeds(embed.Embeds)
				m.SetContent("")
				_, _ = s.ChannelMessageEditComplex(m)
			}
		}
	}
}

func removeReceivedToken(userID string, name string, index int) {
	if Tokens[userID] == nil {
		return
	}

	index--
	for _, v := range Tokens[userID].Coop {
		if v != nil && v.Name == name {
			if index <= len(v.Received) {
				v.SumValueReceived -= v.Received[index].Value
				v.Received = append(v.Received[:index], v.Received[index+1:]...)
				v.TokenDelta = v.SumValueSent - v.SumValueReceived
				saveData(Tokens)
			}
		}
	}
}

// SaveAllData will remove a token from the tracking list
func SaveAllData() {
	log.Print("Saving all token data")
	saveData(Tokens)
}

// AdvancedTransform for storing KV pairs
func advancedTransform(key string) *diskv.PathKey {
	path := strings.Split(key, "/")
	last := len(path) - 1
	return &diskv.PathKey{
		Path:     path[:last],
		FileName: path[last] + ".json",
	}
}

// InverseTransform for storing KV pairs
func inverseTransform(pathKey *diskv.PathKey) (key string) {
	txt := pathKey.FileName[len(pathKey.FileName)-4:]
	if txt != ".json" {
		panic("Invalid file found in storage folder!")
	}
	return strings.Join(pathKey.Path, "/") + pathKey.FileName[:len(pathKey.FileName)-4]
}

func saveData(c map[string]*tokenValues) {
	b, _ := json.Marshal(c)
	_ = dataStore.Write("Tokens", b)
}

func loadData() (map[string]*tokenValues, error) {
	var t map[string]*tokenValues
	b, err := dataStore.Read("Tokens")
	if err != nil {
		return t, err
	}
	err = json.Unmarshal(b, &t)
	if err != nil {
		return t, err
	}

	return t, nil
}

// ArchiveTrackerData purges stale tracker data after 4 days
func ArchiveTrackerData(s *discordgo.Session) {
	if Tokens == nil {
		return
	}
	// For each user, check if the token tracking is older than 3 days
	// If it is, Finish the data
	for k, v := range Tokens {
		for name, tv := range v.Coop {
			if tv.StartTime.Before(time.Now().Add(-72 * time.Hour)) {
				log.Print("Purging Tracker ", tv.Name, " for ", tv.Username)
				msg := discordgo.NewMessageEdit(tv.UserChannelID, tv.TokenMessageID)
				msg.SetContent("")
				msg.Components = &[]discordgo.MessageComponent{}
				embed := getTokenTrackingEmbed(tv, true)
				msg.SetEmbeds(embed.Embeds)
				_, _ = s.ChannelMessageEditComplex(msg)
				delete(Tokens[k].Coop, name)
			}
		}
	}
	saveData(Tokens)
}
