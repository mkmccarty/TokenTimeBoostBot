package track

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xhit/go-str2duration/v2"
)

var integerOneMinValue float64 = 1.0

// GetSlashTokenCommand returns the slash command for token tracking
func GetSlashTokenCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Start token value tracking for a contract",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "name",
				Description: "Unique name for this tracking session. i.e. Use coop-id of the contract.",
				Required:    true,
				MaxLength:   32, // Keep this short
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "duration",
				Description: "Time remaining in this contract. Example: 19h35m.",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "linked",
				Description: "Link with contract channel reactions for sent tokens. Default is true.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "contract-channel",
				Description: "ChannelID or URL to Channel/Thread on Non-BootBot Server. Default is current channel.",
				Required:    false,
			},
		},
	}
}

// GetSlashTokenRemoveCommand returns the slash command for token tracking removal
func GetSlashTokenRemoveCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Remove a sent or received token from tracking",
		Options: []*discordgo.ApplicationCommandOption{
			{

				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "token-list",
				Description:  "The tracking list to remove the token from.",
				Required:     true,
				Autocomplete: true,
			},
			{
				Name:        "token-type",
				Description: "Select the type of token to remove from tracking.",
				Required:    true,
				Type:        discordgo.ApplicationCommandOptionInteger,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{
						Name:  "Sent Token",
						Value: 0,
					},
					{
						Name:  "Received Token",
						Value: 1,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "token-index",
				Description: "Token index number to remove from tracking.",
				MinValue:    &integerOneMinValue,
				Required:    true,
			},
		},
	}
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
	if opt, ok := optionMap["contract-channel"]; ok {
		input := strings.TrimSpace(opt.StringValue())
		s := strings.Split(input, "/")
		if len(s) > 0 {
			// set channelID to last entry in the slice
			channelID = s[len(s)-1]
		}
		if channelID != i.ChannelID {
			linked = false
			linkReceived = false
		}
	}
	// Call into boost module to do that calculations
	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}

	str, embed, err := tokenTracking(s, channelID, userID, trackingName, duration, linked, linkReceived)

	if err != nil {
		str = err.Error()
	} else {
		var data discordgo.MessageSend
		data.Embeds = embed.Embeds
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

// HandleTokenRemoveCommand will handle the /token-remove command
func HandleTokenRemoveCommand(s *discordgo.Session, i *discordgo.InteractionCreate) string {
	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}
	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}

	var tokenList string
	var tokenType int
	var tokenIndex int

	if opt, ok := optionMap["token-list"]; ok {
		tokenList = opt.StringValue()
	}
	if opt, ok := optionMap["token-type"]; ok {
		tokenType = int(opt.IntValue())
	}
	if opt, ok := optionMap["token-index"]; ok {
		tokenIndex = int(opt.IntValue())
	}
	var str = "Token tracker not found in tracking list."

	if Tokens[userID] != nil && Tokens[userID].Coop[tokenList] != nil {
		t := Tokens[userID].Coop[tokenList]
		// Need to figure out which list to remove from
		if tokenType == 0 {
			if len(t.TokenSentTime) <= tokenIndex {
				return fmt.Sprintf("Invalid token index. You have sent %d tokens.", len(t.TokenSentTime))
			}
			removeSentToken(userID, tokenList, tokenIndex)
		} else {
			if len(t.TokenReceivedTime) <= tokenIndex {
				return fmt.Sprintf("Invalid token index. You have received %d tokens.", len(t.TokenReceivedTime))
			}
			removeReceivedToken(userID, tokenList, tokenIndex)
		}
		str = "Token removed from tracking on <#" + Tokens[userID].Coop[tokenList].UserChannelID + ">."

		defer saveData(Tokens)
		embed := tokenTrackingTrack(userID, tokenList, 0, 0) // No sent or received
		comp := getTokenValComponents(tokenTrackingEditing(userID, tokenList, false), tokenList)
		m := discordgo.NewMessageEdit(Tokens[userID].Coop[tokenList].UserChannelID, Tokens[userID].Coop[tokenList].TokenMessageID)
		m.Components = &comp
		m.SetEmbeds(embed.Embeds)
		m.SetContent("")
		_, err := s.ChannelMessageEditComplex(m)
		if err != nil {
			str = "Error updating the token message. " + err.Error()
		}
	}

	saveData(Tokens)
	return str
}

// HandleTokenRemoveAutoComplete will handle the /token-remove autocomplete
func HandleTokenRemoveAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) (string, []*discordgo.ApplicationCommandOptionChoice) {
	// User interacting with bot, is this first time ?
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0)

	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}

	if _, ok := Tokens[userID]; !ok {
		return "", nil
	}
	for _, c := range Tokens[userID].Coop {
		choice := discordgo.ApplicationCommandOptionChoice{
			Name:  c.Name,
			Value: c.Name,
		}
		choices = append(choices, &choice)
	}
	/*
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{
				Content: "Select tracker to adjust the token.",
				Choices: choices,
			}})*/
	return "Select tracker to adjust the token.", choices

}
