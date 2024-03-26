package track

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xhit/go-str2duration/v2"
)

type tokenValue struct {
	UserID              string        // The user ID that is tracking the token value
	Name                string        // Tracking name for this contract
	ChannelID           string        // The channel ID that is tracking the token value
	ChannelMention      string        // The channel mention
	StartTime           time.Time     // When Token Value time started
	EstimatedEndTime    time.Time     // Time of Token Value time plus Duration
	DurationTime        time.Duration // Duration of Token Value time
	TokenSentTime       []time.Time   // time of each token sent
	TokenSentValues     []float64     // time of each token sent
	TokenReceivedTime   []time.Time   // time of each received token
	TokenReceivedValues []float64     // time of each token sent
	TokenValueSent      float64       // sum of all token values
	TokenValueReceived  float64       // sum of all token values
	TokenDelta          float64       // difference between sent and received
	TokenMessageID      string        // Message ID for the Last Token Value message
	Details             bool          // Show details of each token sent
	Edit                bool          // Editing is enabled
}

type tokenValues struct {
	coop map[string]*tokenValue
}

var (
	// Contracts is a map of contracts and is saved to disk
	tokens map[string]*tokenValues // map is UserID
)

func init() {
	tokens = make(map[string]*tokenValues)
}

func resetTokenTracking(tv *tokenValue) {
	tv.StartTime = time.Now()
	tv.TokenSentTime = nil
	tv.TokenReceivedTime = nil
	tv.TokenSentValues = nil
	tv.TokenReceivedValues = nil
	tv.TokenValueSent = 0.0
	tv.TokenValueReceived = 0.0
	tv.TokenDelta = 0.0
	tv.Details = false
	tv.Edit = false
}

// SetTokenTrackingDetails will toggle the details for token tracking
func SetTokenTrackingDetails(userID string, name string) {
	tokens[userID].coop[name].Details = !tokens[userID].coop[name].Details
}

// tokenTrackingEditing will toggle the edit for token tracking
func tokenTrackingEditing(userID string, name string, editSelected bool) bool {
	if editSelected {
		tokens[userID].coop[name].Edit = !tokens[userID].coop[name].Edit
	}
	return tokens[userID].coop[name].Edit
}

// TokenTrackingAdjustTime will adjust the time values for a contract
func TokenTrackingAdjustTime(channelID string, userID string, name string, startHour int, startMinute int, endHour int, endMinute int) string {
	td := tokens[userID].coop[name]

	td.StartTime = td.StartTime.Add(time.Duration(startHour) * time.Hour)
	td.StartTime = td.StartTime.Add(time.Duration(startMinute) * time.Minute)

	td.DurationTime = max(0, td.DurationTime+(time.Duration(endHour)*time.Hour))
	td.DurationTime = max(0, td.DurationTime+(time.Duration(endMinute)*time.Minute))

	td.EstimatedEndTime = td.StartTime.Add(td.DurationTime)

	// Changed duration needs a recalculation
	td.TokenValueSent = 0.0
	for i, t := range td.TokenSentTime {
		now := t
		offsetTime := now.Sub(td.StartTime).Seconds()
		td.TokenSentValues[i] = getTokenValue(offsetTime, td.DurationTime.Seconds())
		td.TokenValueSent += td.TokenSentValues[i]
	}
	td.TokenValueReceived = 0.0
	for i, t := range td.TokenReceivedTime {
		now := t
		offsetTime := now.Sub(td.StartTime).Seconds()
		td.TokenReceivedValues[i] = getTokenValue(offsetTime, td.DurationTime.Seconds())
		td.TokenValueReceived += td.TokenReceivedValues[i]

	}
	td.TokenDelta = td.TokenValueSent - td.TokenValueReceived

	return getTokenTrackingString(td)
}

func getTokenTrackingString(td *tokenValue) string {

	var builder strings.Builder
	fmt.Fprintf(&builder, "Token tracking for **%s** with duration **%s**\n", td.Name, td.DurationTime.Round(time.Minute).String())
	fmt.Fprint(&builder, "Contract channel: ", td.ChannelMention, "\n")
	fmt.Fprintf(&builder, "Contract Start time <t:%d:f>\n", td.StartTime.Unix())

	offsetTime := time.Since(td.StartTime).Seconds()
	fmt.Fprintf(&builder, "> Current token value: %f\n", getTokenValue(offsetTime, td.DurationTime.Seconds()))
	fmt.Fprintf(&builder, "> Token value in 30 minutes: %f\n", getTokenValue(offsetTime+(30*60), td.DurationTime.Seconds()))
	fmt.Fprintf(&builder, "> Token value in one hour: %f\n\n", getTokenValue(offsetTime+(60*60), td.DurationTime.Seconds()))

	if (len(td.TokenSentTime) + len(td.TokenReceivedTime)) > 0 {
		fmt.Fprintf(&builder, "Sent: **%d**  (%4.3f)\n", len(td.TokenSentTime), td.TokenValueSent)
		if td.Details {
			for i, t := range td.TokenSentTime {
				fmt.Fprintf(&builder, "> %d: <t:%d:R> %6.3f\n", i+1, t.Unix(), td.TokenSentValues[i])
			}
		}
		fmt.Fprintf(&builder, "Received: **%d**  (%4.3f)\n", len(td.TokenReceivedTime), td.TokenValueReceived)
		if td.Details {
			for i, t := range td.TokenReceivedTime {
				fmt.Fprintf(&builder, "> %d: <t:%d:R> %6.3f\n", i+1, t.Unix(), td.TokenReceivedValues[i])
			}
		}
		fmt.Fprintf(&builder, "**Current △ TVal %4.3f**\n", td.TokenDelta)
	}

	return builder.String()
}

// TokenTracking is called as a starting point for token tracking
func tokenTracking(channelID string, userID string, name string, duration time.Duration) (string, error) {
	var builder strings.Builder

	if tokens[userID] == nil {
		tokens[userID] = new(tokenValues)
		tokens[userID].coop = make(map[string]*tokenValue)
		tokens[userID].coop[name] = new(tokenValue)
		tokens[userID].coop[name].UserID = userID
		resetTokenTracking(tokens[userID].coop[name])
		tokens[userID].coop[name].Name = name
	}

	td := tokens[userID].coop[name]

	td.ChannelID = channelID // Last channel gets responses
	td.ChannelMention = fmt.Sprintf("<#%s>", channelID)

	// Set the duration
	td.DurationTime = duration
	td.EstimatedEndTime = time.Now().Add(duration)

	builder.WriteString(getTokenTrackingString(td))

	return builder.String(), nil
}

// tokenTrackingTrack is called to track tokens sent and received
func tokenTrackingTrack(userID string, name string, tokenSent int, tokenReceived int) string {

	if tokens[userID] == nil {
		return "Token Tracking not started."
	}

	td := tokens[userID].coop[name]
	now := time.Now()
	offsetTime := now.Sub(td.StartTime).Seconds()
	tokenValue := getTokenValue(offsetTime, td.DurationTime.Seconds())

	if tokenSent > 0 {
		td.TokenSentTime = append(td.TokenSentTime, now)
		td.TokenSentValues = append(td.TokenSentValues, tokenValue)
		td.TokenValueSent += tokenValue
	}
	if tokenReceived > 0 {
		td.TokenReceivedTime = append(td.TokenReceivedTime, now)
		td.TokenReceivedValues = append(td.TokenReceivedValues, tokenValue)
		td.TokenValueReceived += tokenValue
	}
	td.TokenDelta = td.TokenValueSent - td.TokenValueReceived

	return getTokenTrackingString(td)
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
	str := TokenTrackingAdjustTime(i.ChannelID, userID, name, startHour, startMinute, endHour, endMinute)

	m := discordgo.NewMessageEdit(i.ChannelID, i.Message.ID)
	m.Components = getTokenValComponents(tokenTrackingEditing(userID, name, false), name)
	m.SetContent(str)
	s.ChannelMessageEditComplex(m)

}

// getTokenValComponents returns the components for the token value
func getTokenValComponents(timeAdjust bool, name string) []discordgo.MessageComponent {

	if !timeAdjust {
		return []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Send a Token",
						Style:    discordgo.SuccessButton,
						CustomID: "fd_tokenSent",
					},
					discordgo.Button{
						Label:    "Receive a Token",
						Style:    discordgo.DangerButton,
						CustomID: "fd_tokenReceived",
					},
					discordgo.Button{
						Label:    "Details",
						Style:    discordgo.PrimaryButton,
						CustomID: "fd_tokenDetails",
					},
					discordgo.Button{
						Emoji: &discordgo.ComponentEmoji{
							Name: "📝",
						},
						Label:    name,
						Style:    discordgo.SecondaryButton,
						CustomID: "fd_tokenEdit",
					},
					discordgo.Button{
						Label:    "Finish",
						Style:    discordgo.SecondaryButton,
						CustomID: "fd_tokenComplete",
					},
				},
			},
		}
	}
	// Add Start time adjustment
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Send a Token",
					Style:    discordgo.SuccessButton,
					CustomID: "fd_tokenSent",
				},
				discordgo.Button{
					Label:    "Receive a Token",
					Style:    discordgo.DangerButton,
					CustomID: "fd_tokenReceived",
				},
				discordgo.Button{
					Label:    "Details",
					Style:    discordgo.PrimaryButton,
					CustomID: "fd_tokenDetails",
				},
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "💾",
					},
					Label:    name,
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_tokenEdit",
				},
				discordgo.Button{
					Label:    "Finish",
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_tokenComplete",
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Start Hour +1",
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_tokenStartHourPlus",
				},
				discordgo.Button{
					Label:    "Start Minute +5",
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_tokenStartMinutePlusFive",
				},
				discordgo.Button{
					Label:    "Start Minute +1",
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_tokenStartMinutePlusOne",
				},
				discordgo.Button{
					Label:    "Start Hour -1",
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_tokenStartHourMinus",
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Duration Hour +1",
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_tokenDurationHourPlus",
				},
				discordgo.Button{
					Label:    "Duration Minute +5",
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_tokenDurationMinutePlusFive",
				},
				discordgo.Button{
					Label:    "Duration Minute +1",
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_tokenDurationMinutePlusOne",
				},
				discordgo.Button{
					Label:    "Duration Hour -1",
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_tokenDurationHourMinus",
				},
			},
		},
	}
}

// HandleTokenEdit will handle the token edit button
func HandleTokenEdit(s *discordgo.Session, i *discordgo.InteractionCreate) {

	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}

	name, _ := extractTokenName(i.Message.Components[0])

	isEditing := tokenTrackingEditing(userID, name, true)

	str := tokenTrackingTrack(userID, name, 0, 0)

	m := discordgo.NewMessageEdit(i.ChannelID, i.Message.ID)
	m.Components = getTokenValComponents(isEditing, name)
	m.SetContent(str)
	s.ChannelMessageEditComplex(m)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
	})
}

// HandleTokenSend will handle the token send button
func HandleTokenSend(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}

	name, _ := extractTokenName(i.Message.Components[0])
	str := tokenTrackingTrack(userID, name, 1, 0)

	m := discordgo.NewMessageEdit(i.ChannelID, i.Message.ID)
	m.Components = getTokenValComponents(tokenTrackingEditing(userID, name, false), name)
	m.SetContent(str)
	s.ChannelMessageEditComplex(m)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
	})
}

// HandleTokenReceived will handle the token received button
func HandleTokenReceived(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}
	name, _ := extractTokenName(i.Message.Components[0])
	str := tokenTrackingTrack(userID, name, 0, 1)

	m := discordgo.NewMessageEdit(i.ChannelID, i.Message.ID)
	m.Components = getTokenValComponents(tokenTrackingEditing(userID, name, false), name)
	m.SetContent(str)
	s.ChannelMessageEditComplex(m)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
	})
}

// HandleTokenDetails will handle the token sent button
func HandleTokenDetails(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}

	name, _ := extractTokenName(i.Message.Components[0])

	SetTokenTrackingDetails(userID, name)
	str := tokenTrackingTrack(userID, name, 0, 0)

	m := discordgo.NewMessageEdit(i.ChannelID, i.Message.ID)
	m.Components = getTokenValComponents(tokenTrackingEditing(userID, name, false), name)
	m.SetContent(str)
	s.ChannelMessageEditComplex(m)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
	})
}

// HandleTokenCommand will handle the /token command
func HandleTokenCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}
	var duration time.Duration
	if opt, ok := optionMap["duration"]; ok {
		// Timespan of the contract duration
		contractTimespan := strings.TrimSpace(opt.StringValue())
		contractTimespan = strings.Replace(contractTimespan, "day", "d", -1)
		contractTimespan = strings.Replace(contractTimespan, "hr", "h", -1)
		contractTimespan = strings.Replace(contractTimespan, "min", "m", -1)
		contractTimespan = strings.Replace(contractTimespan, "sec", "s", -1)
		duration, _ = str2duration.ParseDuration(contractTimespan)
	}
	var trackingName = ""
	if opt, ok := optionMap["name"]; ok {
		trackingName = opt.StringValue()
	}

	// Call into boost module to do that calculations
	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}

	str, err := tokenTracking(i.ChannelID, userID, trackingName, duration)

	if err != nil {
		str = err.Error()
	}

	var data discordgo.MessageSend
	data.Content = str
	data.Components = getTokenValComponents(false, trackingName) // Initial state

	u, _ := s.UserChannelCreate(userID)
	s.ChannelMessageSendComplex(u.ID, &data)

	str += "Interact with the bot on " + u.Mention() + " to track your token values."

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: str,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	},
	)
}
