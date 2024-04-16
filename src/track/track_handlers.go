package track

import (
	"log"
	"slices"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xhit/go-str2duration/v2"
)

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
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
	})

	name, _ := extractTokenName(i.Message.Components[0])
	isEditing := tokenTrackingEditing(userID, name, true)
	str := tokenTrackingTrack(userID, name, 0, 0)

	comp := getTokenValComponents(isEditing, name)
	m := discordgo.NewMessageEdit(i.ChannelID, i.Message.ID)
	m.Components = &comp
	m.SetContent(str)
	s.ChannelMessageEditComplex(m)

}

// HandleTokenSend will handle the token send button
func HandleTokenSend(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
	str := tokenTrackingTrack(userID, name, 1, 0)

	comp := getTokenValComponents(tokenTrackingEditing(userID, name, false), name)
	m := discordgo.NewMessageEdit(i.ChannelID, i.Message.ID)
	m.Components = &comp
	m.SetContent(str)
	s.ChannelMessageEditComplex(m)

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

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
	})

	name, _ := extractTokenName(i.Message.Components[0])
	str := tokenTrackingTrack(userID, name, 0, 1)

	comp := getTokenValComponents(tokenTrackingEditing(userID, name, false), name)
	m := discordgo.NewMessageEdit(i.ChannelID, i.Message.ID)
	m.Components = &comp
	m.SetContent(str)
	s.ChannelMessageEditComplex(m)

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

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
	})

	name, _ := extractTokenName(i.Message.Components[0])
	SetTokenTrackingDetails(userID, name)
	str := tokenTrackingTrack(userID, name, 0, 0)

	comp := getTokenValComponents(tokenTrackingEditing(userID, name, false), name)
	m := discordgo.NewMessageEdit(i.ChannelID, i.Message.ID)
	m.Components = &comp
	m.SetContent(str)
	s.ChannelMessageEditComplex(m)
}

// HandleTokenCommand will handle the /token command
func HandleTokenCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}
	channelID := i.ChannelID
	linked := true
	linkReceived := true
	var duration time.Duration
	if opt, ok := optionMap["duration"]; ok {
		var err error
		// Timespan of the contract duration
		contractTimespan := strings.TrimSpace(opt.StringValue())
		contractTimespan = strings.Replace(contractTimespan, "day", "d", -1)
		contractTimespan = strings.Replace(contractTimespan, "hr", "h", -1)
		contractTimespan = strings.Replace(contractTimespan, "min", "m", -1)
		contractTimespan = strings.Replace(contractTimespan, "sec", "s", -1)
		duration, err = str2duration.ParseDuration(contractTimespan)
		if err != nil {
			// Invalid duration, just assigning a 12h
			duration = 12 * time.Hour
		}
	}
	var trackingName = ""
	if opt, ok := optionMap["name"]; ok {
		trackingName = strings.TrimSpace(opt.StringValue())
	}
	if opt, ok := optionMap["linked"]; ok {
		linked = opt.BoolValue()
	}
	/*
		if opt, ok := optionMap["link-received"]; ok {
			linkReceived = opt.BoolValue()
		}
	*/
	if opt, ok := optionMap["contract-channel"]; ok {
		input := strings.TrimSpace(opt.StringValue())
		s := strings.Split(input, "/")
		if len(s) > 0 {
			// set channelID to last entry in the slice
			channelID = s[len(s)-1]
		}
		linked = false
		linkReceived = false
	}
	// Call into boost module to do that calculations
	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}

	str, err := tokenTracking(s, channelID, userID, trackingName, duration, linked, linkReceived)

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

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
	})

	name, _ := extractTokenName(i.Message.Components[0])
	s.ChannelMessageDelete(i.ChannelID, i.Message.ID)

	td, err := getTrack(userID, name)
	if err == nil {
		str := getTokenTrackingString(td, true)
		s.ChannelMessageSend(i.ChannelID, str)
	}

	if Tokens[userID] != nil {
		if Tokens[userID].Coop != nil && Tokens[userID].Coop[name] != nil {
			delete(Tokens[userID].Coop, name)
		}
	}

	saveData(Tokens)
}

// ReactionAdd is called when a reaction is added to a message
func ReactionAdd(s *discordgo.Session, r *discordgo.MessageReaction) {
	// Find the message
	var msg, err = s.ChannelMessage(r.ChannelID, r.MessageID)

	if err != nil {
		return
	}
	name, _ := extractTokenName(msg.Components[0])
	emojiName := r.Emoji.Name
	userID := r.UserID
	var numberSlice = []string{"1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣", "🔟"}
	if slices.Contains(numberSlice, emojiName) {
		log.Printf("Token-Reaction: %s id:%s user:%s", emojiName, name, userID)
		var receivedIndex = slices.Index(numberSlice, emojiName)
		removeReceivedToken(userID, name, receivedIndex)
		str := tokenTrackingTrack(userID, name, 0, 0) // No sent or received
		comp := getTokenValComponents(tokenTrackingEditing(userID, name, false), name)
		m := discordgo.NewMessageEdit(r.ChannelID, r.MessageID)
		m.Components = &comp
		m.SetContent(str)
		s.ChannelMessageEditComplex(m)
		defer saveData(Tokens)
	}

	/*
		err = s.MessageReactionRemove(r.ChannelID, r.MessageID, emojiName, r.UserID)
		if err != nil {
			fmt.Println(err, emojiName)
		}
	*/

}
