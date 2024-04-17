package track

import (
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xhit/go-str2duration/v2"
)

// GetSlashTokenCommand returns the slash command for token tracking
func GetSlashTokenCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Display contract completion estimate.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "name",
				Description: "Unique name for this tracking session. i.e. Use coop-id of the contract.",
				Required:    true,
				MaxLength:   16, // Keep this short
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
