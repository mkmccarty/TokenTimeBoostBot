package track

import (
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/bwmarrin/discordgo"
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
						CustomID: "fd_tokenSent#" + name,
					},
					discordgo.Button{
						Label:    "Receive a Token",
						Style:    discordgo.DangerButton,
						CustomID: "fd_tokenReceived#" + name,
					},
					discordgo.Button{
						Label:    "Details",
						Style:    discordgo.PrimaryButton,
						CustomID: "fd_tokenDetails#" + name,
					},
					discordgo.Button{
						Emoji: &discordgo.ComponentEmoji{
							Name: "📝",
						},
						Label:    "Edit",
						Style:    discordgo.SecondaryButton,
						CustomID: "fd_tokenEdit#" + name,
					},
					discordgo.Button{
						Label:    "Finish",
						Style:    discordgo.SecondaryButton,
						CustomID: "fd_tokenComplete#" + name,
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
					CustomID: "fd_tokenSent#" + name,
				},
				discordgo.Button{
					Label:    "Receive a Token",
					Style:    discordgo.DangerButton,
					CustomID: "fd_tokenReceived#" + name,
				},
				discordgo.Button{
					Label:    "Details",
					Style:    discordgo.PrimaryButton,
					CustomID: "fd_tokenDetails#" + name,
				},
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "💾",
					},
					Label:    "Save",
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_tokenEdit#" + name,
				},
				discordgo.Button{
					Label:    "Finish",
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_tokenComplete#" + name,
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Start Hour +1",
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_tokenStartHourPlus#" + name,
				},
				discordgo.Button{
					Label:    "Start Minute +5",
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_tokenStartMinutePlusFive#" + name,
				},
				discordgo.Button{
					Label:    "Start Minute +1",
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_tokenStartMinutePlusOne#" + name,
				},
				discordgo.Button{
					Label:    "Start Hour -1",
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_tokenStartHourMinus#" + name,
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Duration Hour +1",
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_tokenDurationHourPlus#" + name,
				},
				discordgo.Button{
					Label:    "Duration Minute +5",
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_tokenDurationMinutePlusFive#" + name,
				},
				discordgo.Button{
					Label:    "Duration Minute +1",
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_tokenDurationMinutePlusOne#" + name,
				},
				discordgo.Button{
					Label:    "Duration Hour -1",
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_tokenDurationHourMinus#" + name,
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

	name := extractTokenName(i.MessageComponentData().CustomID)
	if name == "" {
		name = extractTokenNameOriginal(i.Message.Components[0])
	}

	if Tokens[userID] == nil || Tokens[userID].Coop == nil || Tokens[userID].Coop[name] == nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Tracker not found.",
			},
		})
	}

	t := Tokens[userID].Coop[name]

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: "fd_trackerEdit#" + name,
			Title:    "Update Tracker Details for " + name,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "duration",
							Label:       "Total Duration",
							Style:       discordgo.TextInputShort,
							Placeholder: fmt.Sprintf("%s", t.DurationTime.String()),
							Required:    true,
							MaxLength:   10,
							MinLength:   2,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "since-start",
							Label:       "How long ago did this start?",
							Style:       discordgo.TextInputShort,
							Placeholder: fmt.Sprintf("%s ago. Example: 1h30m", time.Since(t.StartTime).Round(time.Minute).String()),
							Required:    false,
							MaxLength:   10,
							MinLength:   2,
						},
					},
				},
			}}})
	if err == nil {
		return
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
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

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
	})

	name := extractTokenName(i.MessageComponentData().CustomID)
	if name == "" {
		name = extractTokenNameOriginal(i.Message.Components[0])
	}
	embed := tokenTrackingTrack(userID, name, 1, 0)

	comp := getTokenValComponents(tokenTrackingEditing(userID, name, false), name)
	m := discordgo.NewMessageEdit(i.ChannelID, i.Message.ID)
	m.Components = &comp
	m.SetEmbeds(embed.Embeds)
	m.SetContent("")
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

	name := extractTokenName(i.MessageComponentData().CustomID)
	if name == "" {
		name = extractTokenNameOriginal(i.Message.Components[0])
	}
	embed := tokenTrackingTrack(userID, name, 0, 1)

	comp := getTokenValComponents(tokenTrackingEditing(userID, name, false), name)
	m := discordgo.NewMessageEdit(i.ChannelID, i.Message.ID)
	m.Components = &comp
	m.SetEmbeds(embed.Embeds)
	m.SetContent("")
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

	name := extractTokenName(i.MessageComponentData().CustomID)
	if name == "" {
		name = extractTokenNameOriginal(i.Message.Components[0])
	}
	SetTokenTrackingDetails(userID, name)
	embed := tokenTrackingTrack(userID, name, 0, 0)

	comp := getTokenValComponents(tokenTrackingEditing(userID, name, false), name)
	m := discordgo.NewMessageEdit(i.ChannelID, i.Message.ID)
	m.Components = &comp
	m.SetEmbeds(embed.Embeds)
	m.SetContent("")
	s.ChannelMessageEditComplex(m)
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

	name := extractTokenName(i.MessageComponentData().CustomID)
	if name == "" {
		name = extractTokenNameOriginal(i.Message.Components[0])
	}
	s.ChannelMessageDelete(i.ChannelID, i.Message.ID)

	td, err := getTrack(userID, name)
	if err == nil {
		/*
			str := getTokenTrackingString(td, true)
			s.ChannelMessageSend(i.ChannelID, str)
		*/
		embed := getTokenTrackingEmbed(td, true)
		s.ChannelMessageSendComplex(i.ChannelID, embed)

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

	if Tokens[r.UserID] == nil || Tokens[r.UserID].Coop == nil {
		return
	}
	name := ""

	// Find the name of this tracker from the messageID
	for _, v := range Tokens[r.UserID].Coop {
		if v.TokenMessageID == r.MessageID {
			name = v.Name
			break
			// This is the tracker
		}
	}
	if name == "" {
		return
	}

	emojiName := r.Emoji.Name
	userID := r.UserID
	var numberSlice = []string{"1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣", "🔟"}
	if slices.Contains(numberSlice, emojiName) {
		log.Printf("Token-Reaction: %s id:%s user:%s", emojiName, name, userID)
		var receivedIndex = slices.Index(numberSlice, emojiName)
		removeReceivedToken(userID, name, receivedIndex)
		embed := tokenTrackingTrack(userID, name, 0, 0) // No sent or received
		comp := getTokenValComponents(tokenTrackingEditing(userID, name, false), name)
		m := discordgo.NewMessageEdit(r.ChannelID, r.MessageID)
		m.Components = &comp
		m.SetEmbeds(embed.Embeds)
		m.SetContent("")
		s.ChannelMessageEditComplex(m)
		defer saveData(Tokens)
	}
}
