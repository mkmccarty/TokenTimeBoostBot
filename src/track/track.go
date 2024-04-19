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
)

// Constants for token tracking
const (
	TokenSent     = 0 // TokenSent is a token sent
	TokenReceived = 1 // TokenReceived is a token received
)

type tokenValue struct {
	UserID              string        // The user ID that is tracking the token value
	Name                string        // Tracking name for this contract
	ChannelID           string        // The channel ID that is tracking the token value
	Linked              bool          // If the tracker is linked to channel contract
	LinkRecieved        bool          // If linked, log the received tokens
	ChannelMention      string        // The channel mention
	StartTime           time.Time     // When Token Value time started
	EstimatedEndTime    time.Time     // Time of Token Value time plus Duration
	DurationTime        time.Duration // Duration of Token Value time
	TokenSentTime       []time.Time   // time of each token sent
	TokenSentValues     []float64     // time of each token sent
	TokenSentUserID     []string      // User ID of where each token was sent to
	TokenReceivedTime   []time.Time   // time of each received token
	TokenReceivedValues []float64     // time of each token sent
	TokenReceivedUserID []string      // User ID of where each token came from
	FarmedTokenTime     []time.Time   // time a self farmed token was received
	SumValueSent        float64       // sum of all token values sent
	SumValueReceived    float64       // sum of all token values received
	TokenDelta          float64       // difference between sent and received
	TokenMessageID      string        // Message ID for the Last Token Value message
	UserChannelID       string        // User Channel ID for the Last Token Value message
	Details             bool          // Show details of each token sent
	Edit                bool          // Editing is enabled
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
	tv.TokenSentTime = nil
	tv.TokenReceivedTime = nil
	tv.TokenReceivedUserID = nil
	tv.TokenSentUserID = nil
	tv.TokenSentValues = nil
	tv.TokenReceivedValues = nil
	tv.SumValueSent = 0.0
	tv.SumValueReceived = 0.0
	tv.TokenDelta = 0.0
	tv.Details = false
	tv.Edit = false
}

// SetTokenTrackingDetails will toggle the details for token tracking
func SetTokenTrackingDetails(userID string, name string) {
	td, err := getTrack(userID, name)
	if err != nil {
		return
	}
	td.Details = !td.Details
}

// tokenTrackingEditing will toggle the edit for token tracking
func tokenTrackingEditing(userID string, name string, editSelected bool) bool {
	td, err := getTrack(userID, name)
	if err != nil {
		return false
	}

	if editSelected {
		td.Edit = !td.Edit
	}
	return td.Edit
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
	for i, t := range td.TokenSentTime {
		now := t
		offsetTime := now.Sub(td.StartTime).Seconds()
		td.TokenSentValues[i] = getTokenValue(offsetTime, td.DurationTime.Seconds())
		td.SumValueSent += td.TokenSentValues[i]
	}
	td.SumValueReceived = 0.0
	for i, t := range td.TokenReceivedTime {
		now := t
		offsetTime := now.Sub(td.StartTime).Seconds()
		td.TokenReceivedValues[i] = getTokenValue(offsetTime, td.DurationTime.Seconds())
		td.SumValueReceived += td.TokenReceivedValues[i]

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
	/*
		if !finalDisplay {
			offsetTime := time.Since(td.StartTime).Seconds()
			fmt.Fprintf(&builder, "> Current token value: %1.3f\n", getTokenValue(offsetTime, td.DurationTime.Seconds()))
			fmt.Fprintf(&builder, "> Token value in 30 minutes: %1.3f\n", getTokenValue(offsetTime+(30*60), td.DurationTime.Seconds()))
			fmt.Fprintf(&builder, "> Token value in one hour: %1.3f\n\n", getTokenValue(offsetTime+(60*60), td.DurationTime.Seconds()))
		}

		if len(td.FarmedTokenTime) > 0 {
			fmt.Fprintf(&builder, "Farmed: **%d**\n", len(td.FarmedTokenTime))
			if td.Details {
				for i, t := range td.FarmedTokenTime {
					if !finalDisplay {
						fmt.Fprintf(&builder, "> %d: <t:%d:R>\n", i+1, t.Unix())
					} else {
						fmt.Fprintf(&builder, "> %d: %s\n", i+1, t.Sub(td.StartTime).Round(time.Second))
					}
					if builder.Len() > 1750 {
						fmt.Fprint(&builder, "> ...\n")
						break
					}
				}
			}
		}

		if (len(td.TokenSentTime) + len(td.TokenReceivedTime)) > 0 {
			fmt.Fprintf(&builder, "Sent: **%d**  (%4.3f)\n", len(td.TokenSentTime), td.SumValueSent)
			if td.Details {
				id := "1124449428267343992"
				for i, t := range td.TokenSentTime {
					if len(td.TokenSentUserID) == len(td.TokenSentValues) {
						// Transitional code to fix missing user ID
						id = td.TokenSentUserID[i]
					}
					if !finalDisplay {
						fmt.Fprintf(&builder, "> %d: <t:%d:R> %6.3f <@%s>\n", i+1, t.Unix(), td.TokenSentValues[i], id)
					} else {
						fmt.Fprintf(&builder, "> %d: %s  %6.3f <@%s>\n", i+1, t.Sub(td.StartTime).Round(time.Second), td.TokenSentValues[i], id)
					}
					if builder.Len() > 1750 {
						fmt.Fprint(&builder, "> ...\n")
						break
					}
				}
			}
			fmt.Fprintf(&builder, "Received: **%d**  (%4.3f)\n", len(td.TokenReceivedTime), td.SumValueReceived)
			if td.Details {
				id := "1124449428267343992" // Boost Bot ID
				for i, t := range td.TokenReceivedTime {
					if len(td.TokenReceivedUserID) == len(td.TokenReceivedValues) {
						// Transitional code to fix missing user ID
						id = td.TokenReceivedUserID[i]
					}
					if !finalDisplay {
						fmt.Fprintf(&builder, "> %d: <t:%d:R> %6.3f <@%s>\n", i+1, t.Unix(), td.TokenReceivedValues[i], id)
					} else {
						fmt.Fprintf(&builder, "> %d: %s  %6.3f <@%s>\n", i+1, t.Sub(td.StartTime).Round(time.Second), td.TokenReceivedValues[i], id)
					}
					if builder.Len() > 1750 {
						fmt.Fprint(&builder, "> ...\n")
						break
					}
				}
				if td.LinkRecieved && !finalDisplay {
					fmt.Fprint(&builder, "React with 1️⃣..🔟 to remove errant received tokens at that index. The bot cannot remove your DM reactions.\n")
				}
			}
			if !finalDisplay {
				builder.WriteString("**Current")
			} else {
				builder.WriteString("**Final")
			}

			fmt.Fprintf(&builder, " △ TVal %4.3f**\n", td.TokenDelta)
		}
	*/
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
		//fmt.Fprintf(&tokenValues, "Value now: %1.3f\n", getTokenValue(offsetTime, td.DurationTime.Seconds()))
		//fmt.Fprintf(&tokenValues, "Value in 30 minutes: %1.3f\n", getTokenValue(offsetTime+(30*60), td.DurationTime.Seconds()))
		//fmt.Fprintf(&tokenValues, "Value in one hour: %1.3f\n\n", getTokenValue(offsetTime+(60*60), td.DurationTime.Seconds()))
		field = append(field, &discordgo.MessageEmbedField{
			Name:   "Value Now",
			Value:  fmt.Sprintf("%1.3f\n", getTokenValue(offsetTime, td.DurationTime.Seconds())),
			Inline: true,
		})
		field = append(field, &discordgo.MessageEmbedField{
			Name:   "in 30 Minutes",
			Value:  fmt.Sprintf("%1.3f\n", getTokenValue(offsetTime+(30*60), td.DurationTime.Seconds())),
			Inline: true,
		})
		field = append(field, &discordgo.MessageEmbedField{
			Name:   "in 1 Hour",
			Value:  fmt.Sprintf("%1.3f\n", getTokenValue(offsetTime+(60*60), td.DurationTime.Seconds())),
			Inline: true,
		})

	}

	if len(td.FarmedTokenTime) > 0 {
		var fbuilder strings.Builder

		if td.Details {
			for i, t := range td.FarmedTokenTime {
				if !finalDisplay {
					fmt.Fprintf(&fbuilder, "%d: <t:%d:R>\n", i+1, t.Unix())
				} else {
					fmt.Fprintf(&fbuilder, "%d: %s\n", i+1, t.Sub(td.StartTime).Round(time.Second))
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

	if len(td.TokenSentTime) > 0 {
		var sbuilder strings.Builder
		brief := false
		if len(td.TokenReceivedTime) > 30 {
			brief = true
		}

		fmt.Fprintf(&sbuilder, "%d valued at %4.3f\n", len(td.TokenSentTime), td.SumValueSent)
		if td.Details {
			for i, t := range td.TokenSentTime {
				id := td.TokenSentUserID[i]
				if !brief {
					if !finalDisplay {
						fmt.Fprintf(&sbuilder, "> %d: <t:%d:R> %6.3f <@%s>\n", i+1, t.Unix(), td.TokenSentValues[i], id)
					} else {
						fmt.Fprintf(&sbuilder, "> %d: %s  %6.3f <@%s>\n", i+1, t.Sub(td.StartTime).Round(time.Second), td.TokenSentValues[i], id)
					}
				} else {
					if !finalDisplay {
						fmt.Fprintf(&sbuilder, "> %d: %6.3f\n", i+1, td.TokenSentValues[i])
					} else {
						fmt.Fprintf(&sbuilder, "> %d: %6.3f\n", i+1, td.TokenSentValues[i])
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

	if len(td.TokenReceivedTime) > 0 {
		var rbuilder strings.Builder
		brief := false
		if len(td.TokenReceivedTime) > 30 {
			brief = true
		}

		fmt.Fprintf(&rbuilder, "%d valued at %4.3f\n", len(td.TokenReceivedTime), td.SumValueReceived)
		if td.Details {
			for i, t := range td.TokenReceivedTime {
				id := td.TokenReceivedUserID[i]
				if !brief {
					if !finalDisplay {
						fmt.Fprintf(&rbuilder, "> %d: <t:%d:R> %6.3f <@%s>\n", i+1, t.Unix(), td.TokenReceivedValues[i], id)
					} else {
						fmt.Fprintf(&rbuilder, "> %d: %s  %6.3f <@%s>\n", i+1, t.Sub(td.StartTime).Round(time.Second), td.TokenReceivedValues[i], id)
					}
				} else {
					if !finalDisplay {
						fmt.Fprintf(&rbuilder, "> %d: %6.3f\n", i+1, td.TokenReceivedValues[i])
					} else {
						fmt.Fprintf(&rbuilder, "> %d: %6.3f\n", i+1, td.TokenReceivedValues[i])
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

	embed := &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{{
			Type:        discordgo.EmbedTypeRich,
			Title:       "Token Tracking",
			Description: description.String(),
			Color:       0xffaa00,
			Fields:      field,
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
		td.TokenSentTime = append(td.TokenSentTime, now)
		td.TokenSentValues = append(td.TokenSentValues, tokenValue)
		td.TokenSentUserID = append(td.TokenSentUserID, userID) // Self reported
		td.SumValueSent += tokenValue
	}
	if tokenReceived > 0 {
		td.TokenReceivedTime = append(td.TokenReceivedTime, now)
		td.TokenReceivedValues = append(td.TokenReceivedValues, tokenValue)
		td.TokenReceivedUserID = append(td.TokenReceivedUserID, userID) // Self reported
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
func extractTokenName(comp discordgo.MessageComponent) (string, error) {
	jsonBlob, _ := discordgo.Marshal(comp)
	stage1 := string(jsonBlob[:])
	stage2 := strings.Split(stage1, "{")[5]
	stage3 := strings.Split(stage2, ",")[0]
	stage4 := strings.Split(stage3, ":")[1]

	// extract string from test2 until the backslash
	return stage4[1 : len(stage4)-1], nil
}

func syncTokenTracking(name string, startTime time.Time, duration time.Duration) {
	for _, v := range Tokens {
		if v.Coop[name] != nil && !v.Coop[name].Edit {
			v.Coop[name].StartTime = startTime
			v.Coop[name].DurationTime = duration
			v.Coop[name].EstimatedEndTime = startTime.Add(duration)
		}
	}
}

// TokenAdjustTimestamp will adjust the timestamp for the token
func TokenAdjustTimestamp(s *discordgo.Session, i *discordgo.InteractionCreate, startHour int, startMinute int, endHour int, endMinute int) {
	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
	})

	name, _ := extractTokenName(i.Message.Components[0])
	embed := TokenTrackingAdjustTime(i.ChannelID, userID, name, startHour, startMinute, endHour, endMinute)

	comp := getTokenValComponents(tokenTrackingEditing(userID, name, false), name)
	m := discordgo.NewMessageEdit(i.ChannelID, i.Message.ID)
	m.Components = &comp
	m.SetEmbeds(embed.Embeds)
	m.SetContent("")
	s.ChannelMessageEditComplex(m)
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
			comp := getTokenValComponents(tokenTrackingEditing(userID, v.Name, false), v.Name)
			m := discordgo.NewMessageEdit(v.UserChannelID, v.TokenMessageID)
			m.Components = &comp
			m.SetEmbeds(embed.Embeds)
			m.SetContent("")
			s.ChannelMessageEditComplex(m)
		}
	}

}

// ContractTokenMessage will track the token sent from the contract Token reaction
func ContractTokenMessage(s *discordgo.Session, channelID string, userID string, kind int, count int, actorUserID string) {
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
					v.TokenSentTime = append(v.TokenSentTime, now)
					v.TokenSentValues = append(v.TokenSentValues, tokenValue)
					v.TokenSentUserID = append(v.TokenSentUserID, actorUserID) // Token sent to...
					v.SumValueSent += tokenValue
				}
				redraw = true
			} else if v.LinkRecieved && kind == TokenReceived {
				for i := 0; i < count; i++ {
					v.TokenReceivedTime = append(v.TokenReceivedTime, now)
					v.TokenReceivedValues = append(v.TokenReceivedValues, tokenValue)
					v.TokenReceivedUserID = append(v.TokenReceivedUserID, actorUserID) // Token received from...
					v.SumValueReceived += tokenValue
				}
				redraw = true
			}
			if redraw {
				v.TokenDelta = v.SumValueSent - v.SumValueReceived
				saveData(Tokens)
				embed := getTokenTrackingEmbed(v, false)
				comp := getTokenValComponents(tokenTrackingEditing(userID, v.Name, false), v.Name)
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
			if index < len(v.TokenReceivedTime) {
				v.SumValueReceived -= v.TokenReceivedValues[index]
				v.TokenReceivedTime = append(v.TokenReceivedTime[:index], v.TokenReceivedTime[index+1:]...)
				v.TokenReceivedValues = append(v.TokenReceivedValues[:index], v.TokenReceivedValues[index+1:]...)
				v.TokenReceivedUserID = append(v.TokenReceivedUserID[:index], v.TokenReceivedUserID[index+1:]...)
				v.TokenDelta = v.SumValueSent - v.SumValueReceived
				saveData(Tokens)
			}
		}
	}
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
