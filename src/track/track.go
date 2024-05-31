package track

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
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
	Time   time.Time // Time the token was sent or received
	Value  float64   // Calculated value of the token
	UserID string    // Who sent or received the token
	Serial string    // Serial number of the token
}

type tokenValue struct {
	UserID           string        // The user ID that is tracking the token value
	Username         string        // The username that is tracking the token value
	Name             string        // Tracking name for this contract
	ChannelID        string        // The channel ID that is tracking the token value
	Linked           bool          // If the tracker is linked to channel contract
	LinkRecieved     bool          // If linked, log the received tokens
	ChannelMention   string        // The channel mention
	StartTime        time.Time     // When Token Value time started
	EstimatedEndTime time.Time     // Time of Token Value time plus Duration
	DurationTime     time.Duration // Duration of Token Value time
	Sent             []TokenUnit
	Received         []TokenUnit
	FarmedTokenTime  []time.Time // time a self farmed token was received
	SumValueSent     float64     // sum of all token values sent
	SumValueReceived float64     // sum of all token values received
	TokenDelta       float64     // difference between sent and received
	TokenMessageID   string      // Message ID for the Last Token Value message
	UserChannelID    string      // User Channel ID for the Last Token Value message
	Details          bool        // Show details of each token sent
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
	tv.LinkRecieved = true
	tv.Sent = nil
	tv.Received = nil
	//tv.TokenSentTime = nil
	//tv.TokenReceivedTime = nil
	//tv.TokenReceivedUserID = nil
	//tv.TokenSentUserID = nil
	//tv.TokenSentValues = nil
	//tv.TokenReceivedValues = nil
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
func TokenTrackingAdjustTime(channelID string, userID string, name string, startHour int, startMinute int, endHour int, endMinute int) *discordgo.MessageSend {
	td, err := getTrack(userID, name)
	if err != nil {
		return nil
	}
	td.StartTime = td.StartTime.Add(time.Duration(startHour) * time.Hour)
	td.StartTime = td.StartTime.Add(time.Duration(startMinute) * time.Minute)

	td.DurationTime = max(0, td.DurationTime+(time.Duration(endHour)*time.Hour))
	td.DurationTime = max(0, td.DurationTime+(time.Duration(endMinute)*time.Minute))

	td.EstimatedEndTime = td.StartTime.Add(td.DurationTime)

	// Changed duration needs a recalculation
	td.SumValueSent = 0.0
	for i, t := range td.Sent {
		now := t.Time
		offsetTime := now.Sub(td.StartTime).Seconds()
		td.Sent[i].Value = getTokenValue(offsetTime, td.DurationTime.Seconds())
		td.SumValueSent += td.Sent[i].Value
	}
	td.SumValueReceived = 0.0
	for i, t := range td.Received {
		now := t.Time
		offsetTime := now.Sub(td.StartTime).Seconds()
		td.Received[i].Value = getTokenValue(offsetTime, td.DurationTime.Seconds())
		td.SumValueReceived += td.Received[i].Value

	}
	td.TokenDelta = td.SumValueSent - td.SumValueReceived

	return getTokenTrackingEmbed(td, false)
}

func getTokenTrackingString(td *tokenValue, finalDisplay bool) string {
	var builder strings.Builder
	ts := td.DurationTime.Round(time.Minute).String()
	if finalDisplay {
		fmt.Fprintf(&builder, "# Final Token tracking for **%s**\n", td.Name)
	} else {
		fmt.Fprintf(&builder, "# Token tracking for **%s**\n", td.Name)
	}
	if td.Linked {
		fmt.Fprint(&builder, "Linked Contract: ", td.ChannelMention, "\n")
	} else {
		fmt.Fprint(&builder, "Contract Channel: ", td.ChannelMention, "\n")
	}
	fmt.Fprintf(&builder, "Start time: <t:%d:t>\n", td.StartTime.Unix())
	fmt.Fprintf(&builder, "Duration  : **%s**\n", ts[:len(ts)-2])

	return builder.String()
}

func getTokenTrackingEmbed(td *tokenValue, finalDisplay bool) *discordgo.MessageSend {
	var description strings.Builder
	var linkedHdr strings.Builder

	var totalHeader string
	var finalTotal string

	var field []*discordgo.MessageEmbedField

	ts := td.DurationTime.Round(time.Minute).String()
	if finalDisplay {
		fmt.Fprintf(&description, "Final Token tracking for **%s**\n", td.Name)
	} else {
		fmt.Fprintf(&description, "Token tracking for **%s**\n", td.Name)
	}
	fmt.Fprintf(&description, "Start time: <t:%d:t>\n", td.StartTime.Unix())
	fmt.Fprintf(&description, "Duration  : **%s**\n", ts[:len(ts)-2])

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
		field = append(field, &discordgo.MessageEmbedField{
			Name:   "Farmed Tokens",
			Value:  fbuilder.String(),
			Inline: false,
		})
	}

	if len(td.Sent) > 0 {
		var sbuilder strings.Builder
		brief := false
		if len(td.Received) > 30 {
			brief = true
		}

		fmt.Fprintf(&sbuilder, "%d valued at %4.3f\n", len(td.Sent), td.SumValueSent)
		if td.Details {
			for i, t := range td.Sent {
				id := td.Sent[i].UserID
				if !brief {
					if !finalDisplay {
						fmt.Fprintf(&sbuilder, "> %d: <t:%d:R> %6.3f %s\n", i+1, t.Time.Unix(), t.Value, id)
					} else {
						fmt.Fprintf(&sbuilder, "> %d: %s  %6.3f %s\n", i+1, t.Time.Sub(td.StartTime).Round(time.Second), t.Value, id)
					}
				} else {
					if !finalDisplay {
						fmt.Fprintf(&sbuilder, "> %d: %6.3f\n", i+1, t.Value)
					} else {
						fmt.Fprintf(&sbuilder, "> %d: %6.3f\n", i+1, t.Value)
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
		if len(td.Received) > 30 {
			brief = true
		}

		fmt.Fprintf(&rbuilder, "%d valued at %4.3f\n", len(td.Received), td.SumValueReceived)
		if td.Details {
			for i, t := range td.Received {
				id := t.UserID
				if !brief {
					if !finalDisplay {
						fmt.Fprintf(&rbuilder, "> %d: <t:%d:R> %6.3f %s\n", i+1, t.Time.Unix(), t.Value, id)
					} else {
						fmt.Fprintf(&rbuilder, "> %d: %s  %6.3f %s\n", i+1, t.Time.Sub(td.StartTime).Round(time.Second), t.Value, id)
					}
				} else {
					if !finalDisplay {
						fmt.Fprintf(&rbuilder, "> %d: %6.3f\n", i+1, t.Value)
					} else {
						fmt.Fprintf(&rbuilder, "> %d: %6.3f\n", i+1, t.Value)
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

			if td.LinkRecieved && !finalDisplay {
				fmt.Fprint(&rbuilder, "\nReact with 1️⃣..🔟 to remove errant received tokens at that index. The bot cannot remove your DM reactions.\n")
			}
		}

		field = append(field, &discordgo.MessageEmbedField{
			Name:   "Received Tokens",
			Value:  rbuilder.String(),
			Inline: brief,
		})
		totalHeader = "Current △ TVal"
		if finalDisplay {
			totalHeader = "Final △ TVal"
		}
		finalTotal = fmt.Sprintf("%4.3f", td.TokenDelta)
		field = append(field, &discordgo.MessageEmbedField{
			Name:   totalHeader,
			Value:  finalTotal,
			Inline: false,
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
	}
	return Tokens[userID].Coop[name], nil
}

// TokenTracking is called as a starting point for token tracking
func tokenTracking(s *discordgo.Session, channelID string, userID string, name string, duration time.Duration, linked bool, linkReceived bool) (string, *discordgo.MessageSend, error) {
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
		s.ChannelMessageDelete(Tokens[userID].Coop[name].UserChannelID, Tokens[userID].Coop[name].TokenMessageID)
		resetTokenTracking(Tokens[userID].Coop[name])
		Tokens[userID].Coop[name].Name = name
	}

	td, err := getTrack(userID, name)
	if err != nil {
		return "", nil, err
	}

	td.ChannelID = channelID // Last channel gets responses
	td.ChannelMention = fmt.Sprintf("<#%s>", channelID)

	// Set the duration
	td.DurationTime = duration
	td.EstimatedEndTime = time.Now().Add(duration)
	td.Linked = linked
	td.LinkRecieved = linkReceived

	return getTokenTrackingString(td, false), getTokenTrackingEmbed(td, false), nil
}

// tokenTrackingTrack is called to track tokens sent and received
func tokenTrackingTrack(userID string, name string, tokenSent int, tokenReceived int) *discordgo.MessageSend {
	td, err := getTrack(userID, name)
	if err != nil {
		return nil
	}
	now := time.Now()
	offsetTime := now.Sub(td.StartTime).Seconds()
	tokenValue := getTokenValue(offsetTime, td.DurationTime.Seconds())

	if tokenSent > 0 {
		td.Sent = append(td.Sent, TokenUnit{Time: now, Value: tokenValue, UserID: td.Username, Serial: xid.New().String()})
		td.SumValueSent += tokenValue
	}
	if tokenReceived > 0 {
		td.Received = append(td.Received, TokenUnit{Time: now, Value: tokenValue, UserID: td.Username, Serial: xid.New().String()})
		td.SumValueReceived += tokenValue
	}
	td.TokenDelta = td.SumValueSent - td.SumValueReceived

	return getTokenTrackingEmbed(td, false)
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

func syncTokenTracking(name string, startTime time.Time, duration time.Duration) {
	for _, v := range Tokens {
		if v.Coop[name] != nil {
			v.Coop[name].StartTime = startTime
			v.Coop[name].DurationTime = duration
			v.Coop[name].EstimatedEndTime = startTime.Add(duration)
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
			comp := getTokenValComponents(v.Name)
			m := discordgo.NewMessageEdit(v.UserChannelID, v.TokenMessageID)
			m.Components = &comp
			m.SetEmbeds(embed.Embeds)
			m.SetContent("")
			s.ChannelMessageEditComplex(m)
		}
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
			if kind == TokenSent {
				for i := 0; i < count; i++ {
					v.Sent = append(v.Sent, TokenUnit{Time: now, Value: tokenValue, UserID: actorUserID, Serial: serialID})
					v.SumValueSent += tokenValue
				}
				redraw = true
			} else if v.LinkRecieved && kind == TokenReceived {
				for i := 0; i < count; i++ {
					v.Received = append(v.Received, TokenUnit{Time: now, Value: tokenValue, UserID: actorUserID, Serial: serialID})
					v.SumValueReceived += tokenValue
				}
				redraw = true
			}
			if redraw {
				v.TokenDelta = v.SumValueSent - v.SumValueReceived
				saveData(Tokens)
				embed := getTokenTrackingEmbed(v, false)
				comp := getTokenValComponents(v.Name)
				m := discordgo.NewMessageEdit(v.UserChannelID, v.TokenMessageID)
				m.Components = &comp
				m.SetEmbeds(embed.Embeds)
				m.SetContent("")
				s.ChannelMessageEditComplex(m)
			}
		}
	}
}

func removeReceivedToken(userID string, name string, index int) {
	if Tokens[userID] == nil {
		return
	}

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

func removeSentToken(userID string, name string, index int) {
	if Tokens[userID] == nil {
		return
	}
	index--
	for _, v := range Tokens[userID].Coop {
		if v != nil && v.Name == name {
			if index < len(v.Sent) {
				v.SumValueSent -= v.Sent[index].Value
				// Rewrite the following 3 lines to use TokenUnit
				v.Sent = append(v.Sent[:index], v.Sent[index+1:]...)
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

func saveData(c map[string]*tokenValues) error {
	b, _ := json.Marshal(c)
	dataStore.Write("Tokens", b)
	return nil
}

func loadData() (map[string]*tokenValues, error) {
	var t map[string]*tokenValues
	b, err := dataStore.Read("Tokens")
	if err != nil {
		return t, err
	}
	json.Unmarshal(b, &t)
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
				s.ChannelMessageDelete(tv.UserChannelID, tv.TokenMessageID)
				embed := getTokenTrackingEmbed(tv, true)
				s.ChannelMessageSendComplex(tv.UserChannelID, embed)
				delete(Tokens[k].Coop, name)
			}
		}
	}
	saveData(Tokens)
}
