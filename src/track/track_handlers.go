package track

import (
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// getTokenValComponents returns the components for the token value
func getTokenValComponents(name string, linked bool) []discordgo.MessageComponent {
	greenButton := discordgo.SuccessButton
	redButton := discordgo.DangerButton
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Send a Token",
					Style:    greenButton,
					CustomID: "fd_tokenSent#1x!" + name,
					Disabled: linked,
				},
				discordgo.Button{
					Label:    "Receive a Token",
					Style:    redButton,
					CustomID: "fd_tokenReceived#1x!" + name,
				},
				discordgo.Button{
					Label:    "Details",
					Style:    discordgo.PrimaryButton,
					CustomID: "fd_tokenDetails#" + name,
				},
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "🕰️",
					},
					Label:    "Adjust",
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
					Label:    "Send 2 Tokens",
					Style:    greenButton,
					CustomID: "fd_tokenSent#2x!" + name,
					Disabled: linked,
				},
				discordgo.Button{
					Label:    "Receive 2 Tokens",
					Style:    redButton,
					CustomID: "fd_tokenReceived#2x!" + name,
				},
				discordgo.Button{
					Label:    "Send 6 Tokens",
					Style:    greenButton,
					CustomID: "fd_tokenSent#6x!" + name,
					Disabled: linked,
				},
				discordgo.Button{
					Label:    "Receive 6 Tokens",
					Style:    redButton,
					CustomID: "fd_tokenReceived#6x!" + name,
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
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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
			Title:    "Update Tracker for " + name,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "coop-id",
							Label:       "Coop ID",
							Style:       discordgo.TextInputShort,
							Placeholder: t.CoopID,
							MaxLength:   30,
							Required:    false,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "duration",
							Label:       "Total Duration",
							Style:       discordgo.TextInputShort,
							Placeholder: strings.Replace(t.DurationTime.Round(time.Minute).String(), "0s", "", -1),
							Required:    false,
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
							Placeholder: fmt.Sprintf("%s (ago), or use timestamp", strings.Replace(time.Since(t.StartTime).Round(time.Minute).String(), "0s", "", -1)),
							Required:    false,
							MaxLength:   30,
							MinLength:   2,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "start-timestamp",
							Label:       "Start Timestamp",
							Style:       discordgo.TextInputShort,
							Placeholder: fmt.Sprintf("<t:%d:t> or %d.  Discord timestamp", t.StartTime.Unix(), t.StartTime.Unix()),
							Required:    false,
							MaxLength:   30,
							MinLength:   2,
						},
					},
				},
			}}})
	if err != nil {
		log.Println(err)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: err.Error(),
			}})
	}
	if err == nil {
		return
	}

}

// HandleTokenSend will handle the token send button
func HandleTokenSend(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
		Data: &discordgo.InteractionResponseData{
			Content:    "",
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})
	if err != nil {
		log.Println(err)
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{})

	name := extractTokenName(i.MessageComponentData().CustomID)
	if name == "" {
		name = extractTokenNameOriginal(i.Message.Components[0])
	}

	// Extract the token count from before the name
	tokens := 1
	tokenname := strings.Split(name, "!")
	if len(tokenname) > 1 {
		if len(tokenname[0]) > 0 && tokenname[0][0] >= '0' && tokenname[0][0] <= '9' {
			tokens, _ = strconv.Atoi(tokenname[0][0:1])
		}
		name = tokenname[1]
	}

	embed, linked := tokenTrackingTrack(userID, name, tokens, 0)

	comp := getTokenValComponents(name, linked)
	m := discordgo.NewMessageEdit(i.ChannelID, i.Message.ID)
	m.Components = &comp
	m.SetEmbeds(embed.Embeds)
	m.SetContent("")
	_, _ = s.ChannelMessageEditComplex(m)

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

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
		Data: &discordgo.InteractionResponseData{
			Content:    "",
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})
	if err != nil {
		log.Println(err)
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{})

	name := extractTokenName(i.MessageComponentData().CustomID)
	if name == "" {
		name = extractTokenNameOriginal(i.Message.Components[0])
	}
	// Extract the token count from before the name
	tokens := 1
	tokenname := strings.Split(name, "!")
	if len(tokenname) > 1 {
		if len(tokenname[0]) > 0 && tokenname[0][0] >= '0' && tokenname[0][0] <= '9' {
			tokens, _ = strconv.Atoi(tokenname[0][0:1])
		}
		name = tokenname[1]
	}

	embed, linked := tokenTrackingTrack(userID, name, 0, tokens)

	comp := getTokenValComponents(name, linked)
	m := discordgo.NewMessageEdit(i.ChannelID, i.Message.ID)
	m.Components = &comp
	m.SetEmbeds(embed.Embeds)
	m.SetContent("")
	_, _ = s.ChannelMessageEditComplex(m)

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

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
		Data: &discordgo.InteractionResponseData{
			Content:    "",
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})
	if err != nil {
		log.Println(err)
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{})
	name := extractTokenName(i.MessageComponentData().CustomID)
	if name == "" {
		name = extractTokenNameOriginal(i.Message.Components[0])
	}
	SetTokenTrackingDetails(userID, name)
	embed, linked := tokenTrackingTrack(userID, name, 0, 0)

	comp := getTokenValComponents(name, linked)
	m := discordgo.NewMessageEdit(i.ChannelID, i.Message.ID)
	m.Components = &comp
	m.SetEmbeds(embed.Embeds)
	m.SetContent("")
	_, err = s.ChannelMessageEditComplex(m)
	if err != nil {
		log.Println(err)
	}
}

// HandleTokenComplete will close the token tracking
func HandleTokenComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
		Data: &discordgo.InteractionResponseData{
			Content:    "",
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})
	if err != nil {
		log.Println(err)
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{})

	name := extractTokenName(i.MessageComponentData().CustomID)
	if name == "" {
		name = extractTokenNameOriginal(i.Message.Components[0])
	}

	td, err := getTrack(userID, name)
	if err == nil {
		embed := getTokenTrackingEmbed(td, true)
		msg := discordgo.NewMessageEdit(i.ChannelID, i.Message.ID)
		msg.SetContent("")
		msg.Components = &[]discordgo.MessageComponent{}
		msg.SetEmbeds(embed.Embeds)
		_, _ = s.ChannelMessageEditComplex(msg)
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
		embed, linked := tokenTrackingTrack(userID, name, 0, 0) // No sent or received
		comp := getTokenValComponents(name, linked)
		m := discordgo.NewMessageEdit(r.ChannelID, r.MessageID)
		m.Components = &comp
		m.SetEmbeds(embed.Embeds)
		m.SetContent("")
		_, _ = s.ChannelMessageEditComplex(m)
		defer saveData(Tokens)
	}
}
