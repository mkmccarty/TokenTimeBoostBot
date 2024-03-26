package track

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/peterbourgon/diskv/v3"
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
func TokenTrackingAdjustTime(channelID string, userID string, name string, startHour int, startMinute int, endHour int, endMinute int) string {
	td, err := getTrack(userID, name)
	if err != nil {
		return ""
	}
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

	return getTokenTrackingString(td, false)
}

func getTokenTrackingString(td *tokenValue, finalDisplay bool) string {
	var builder strings.Builder
	ts := td.DurationTime.Round(time.Minute).String()
	fmt.Fprintf(&builder, "Token tracking for **%s** with duration **%s**\n", td.Name, ts[:len(ts)-2])
	fmt.Fprint(&builder, "Contract channel: ", td.ChannelMention, "\n")
	fmt.Fprintf(&builder, "Contract Start time <t:%d:t>\n", td.StartTime.Unix())

	if !finalDisplay {
		offsetTime := time.Since(td.StartTime).Seconds()
		fmt.Fprintf(&builder, "> Current token value: %f\n", getTokenValue(offsetTime, td.DurationTime.Seconds()))
		fmt.Fprintf(&builder, "> Token value in 30 minutes: %f\n", getTokenValue(offsetTime+(30*60), td.DurationTime.Seconds()))
		fmt.Fprintf(&builder, "> Token value in one hour: %f\n\n", getTokenValue(offsetTime+(60*60), td.DurationTime.Seconds()))
	}

	if (len(td.TokenSentTime) + len(td.TokenReceivedTime)) > 0 {
		fmt.Fprintf(&builder, "Sent: **%d**  (%4.3f)\n", len(td.TokenSentTime), td.TokenValueSent)
		if td.Details {
			for i, t := range td.TokenSentTime {
				if !finalDisplay {
					fmt.Fprintf(&builder, "> %d: <t:%d:R> %6.3f\n", i+1, t.Unix(), td.TokenSentValues[i])
				} else {
					fmt.Fprintf(&builder, "> %d: %s  %6.3f\n", i+1, t.Sub(td.StartTime).Round(time.Second), td.TokenSentValues[i])
				}
				if builder.Len() > 1750 {
					fmt.Fprint(&builder, "> ...\n")
					break
				}
			}
		}
		fmt.Fprintf(&builder, "Received: **%d**  (%4.3f)\n", len(td.TokenReceivedTime), td.TokenValueReceived)
		if td.Details {
			for i, t := range td.TokenReceivedTime {
				if !finalDisplay {
					fmt.Fprintf(&builder, "> %d: <t:%d:R> %6.3f\n", i+1, t.Unix(), td.TokenReceivedValues[i])
				} else {
					fmt.Fprintf(&builder, "> %d: %s  %6.3f\n", i+1, t.Sub(td.StartTime).Round(time.Second), td.TokenReceivedValues[i])
				}
				if builder.Len() > 1750 {
					fmt.Fprint(&builder, "> ...\n")
					break
				}
			}
		}
		fmt.Fprintf(&builder, "**Current △ TVal %4.3f**\n", td.TokenDelta)
	}

	return builder.String()
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
func tokenTracking(s *discordgo.Session, channelID string, userID string, name string, duration time.Duration) (string, error) {
	var builder strings.Builder

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
		return "", err
	}

	td.ChannelID = channelID // Last channel gets responses
	td.ChannelMention = fmt.Sprintf("<#%s>", channelID)

	// Set the duration
	td.DurationTime = duration
	td.EstimatedEndTime = time.Now().Add(duration)

	builder.WriteString(getTokenTrackingString(td, false))

	return builder.String(), nil
}

// tokenTrackingTrack is called to track tokens sent and received
func tokenTrackingTrack(userID string, name string, tokenSent int, tokenReceived int) string {
	td, err := getTrack(userID, name)
	if err != nil {
		return ""
	}
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

	return getTokenTrackingString(td, false)
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
	saveData(Tokens)
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
	saveData(Tokens)
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

	str, err := tokenTracking(s, i.ChannelID, userID, trackingName, duration)

	if err != nil {
		str = err.Error()
	} else {
		var data discordgo.MessageSend
		data.Content = str
		data.Components = getTokenValComponents(false, trackingName) // Initial state

		u, _ := s.UserChannelCreate(userID)
		msg, _ := s.ChannelMessageSendComplex(u.ID, &data)
		Tokens[userID].Coop[trackingName].TokenMessageID = msg.ID
		Tokens[userID].Coop[trackingName].UserChannelID = u.ID

		str += "Interact with the bot on " + u.Mention() + " to track your token values."
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: str,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	},
	)
	saveData(Tokens)
}

// HandleTokenComplete will close the token tracking
func HandleTokenComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}
	name, _ := extractTokenName(i.Message.Components[0])
	s.ChannelMessageDelete(i.ChannelID, i.Message.ID)

	td, err := getTrack(userID, name)
	if err == nil {
		str := getTokenTrackingString(td, true)
		s.ChannelMessageSend(i.ChannelID, str)
	}

	if Tokens[userID] != nil {
		if Tokens[userID].Coop != nil && Tokens[userID].Coop[name] != nil {
			Tokens[userID].Coop[name] = nil
		}
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
	})
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

// ContractTokenSent will track the token received from the contract Token reaction
func ContractTokenSent(s *discordgo.Session, channelID string, userID string) {
	if Tokens[userID] == nil {
		return
	}

	for _, v := range Tokens[userID].Coop {
		if v.ChannelID == channelID {
			now := time.Now()
			offsetTime := now.Sub(v.StartTime).Seconds()
			tokenValue := getTokenValue(offsetTime, v.DurationTime.Seconds())
			v.TokenSentTime = append(v.TokenSentTime, now)
			v.TokenSentValues = append(v.TokenSentValues, tokenValue)
			v.TokenValueSent += tokenValue
			v.TokenDelta = v.TokenValueSent - v.TokenValueReceived
			saveData(Tokens)
			str := getTokenTrackingString(v, false)
			m := discordgo.NewMessageEdit(v.UserChannelID, v.TokenMessageID)
			m.Components = getTokenValComponents(tokenTrackingEditing(userID, v.Name, false), v.Name)
			m.SetContent(str)
			s.ChannelMessageEditComplex(m)
		}
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
