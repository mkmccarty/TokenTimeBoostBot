package track

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/moby/moby/pkg/namesgenerator"
	"github.com/xhit/go-str2duration/v2"
)

var integerOneMinValue float64 = 1.0

// GetSlashTokenCommand returns the slash command for token tracking
func GetSlashTokenCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Start token value tracking for a contract",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
			discordgo.InteractionContextBotDM,
			discordgo.InteractionContextPrivateChannel,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
			discordgo.ApplicationIntegrationUserInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "name",
				Description: "Unique name for this tracking session. i.e. Use coop-id of the contract.",
				Required:    false,
				MaxLength:   32, // Keep this short
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "duration",
				Description: "Time remaining in this contract. Example: 19h35m. Linked contracts know this.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "linked",
				Description: "Link with contract channel reactions for sent tokens. Default is true.",
				Required:    false,
			},
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "contract-id",
				Description:  "Contract ID",
				Required:     false,
				Autocomplete: true,
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
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
			discordgo.InteractionContextBotDM,
			discordgo.InteractionContextPrivateChannel,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
			discordgo.ApplicationIntegrationUserInstall,
		},
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
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "alternate",
				Description:  "Select a linked alternate to show their token values",
				Required:     false,
				Autocomplete: true,
			},
		},
	}
}

// HandleTokenCommand will handle the /token command
func HandleTokenCommand(s *discordgo.Session, i *discordgo.InteractionCreate, contractID string, name string) {
	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}
	channelID := i.ChannelID
	linked := true
	linkReceived := true
	duration := 12 * time.Hour

	if contractID == "" {
		if opt, ok := optionMap["contract-id"]; ok {
			contractID = opt.StringValue()
		}
	}

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
	} else {
		d := ei.EggIncContractsAll[contractID]
		if d.ID != "" {
			duration = d.EstimatedDuration
		}
	}
	var trackingName = name
	if opt, ok := optionMap["name"]; ok {
		trackingName = strings.TrimSpace(opt.StringValue())
	} else if trackingName == "" {

		trackingName = fmt.Sprintln(namesgenerator.GetRandomName(0))

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

		//if channelID != i.ChannelID {
		// Is there a contract running in another channel?
		//linked = false
		//linkReceived = false
		//}
	}
	// Call into boost module to do that calculations
	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}

	str, embed, err := tokenTracking(s, channelID, userID, trackingName, contractID, duration, linked, linkReceived)

	if err != nil {
		str = err.Error()
	} else {
		var data discordgo.MessageSend
		data.Embeds = embed.Embeds
		data.Components = getTokenValComponents(trackingName) // Initial state

		u, _ := s.UserChannelCreate(userID)
		msg, _ := s.ChannelMessageSendComplex(u.ID, &data)
		Tokens[userID].Coop[trackingName].TokenMessageID = msg.ID
		Tokens[userID].Coop[trackingName].UserChannelID = u.ID

		str += "Interact with the bot on " + u.Mention() + " to track your token values."
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: str,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	},
	)
	saveData(Tokens)
}

// HandleTrackerEdit will handle the tracker edit button
func HandleTrackerEdit(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}
	name := extractTokenName(i.ModalSubmitData().CustomID)

	if Tokens[userID] == nil || Tokens[userID].Coop == nil || Tokens[userID].Coop[name] == nil {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Tracker not found.",
			},
		})
	}

	str := ""

	t := Tokens[userID].Coop[name]
	data := i.ModalSubmitData()
	for _, comp := range data.Components {
		input := comp.(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput)
		if input.CustomID == "coop-id" {
			coopID := strings.TrimSpace(input.Value)
			t.CoopID = coopID
		}
		if input.CustomID == "duration" {
			// Timespan of the contract duration
			contractTimespan := strings.TrimSpace(input.Value)
			contractTimespan = strings.Replace(contractTimespan, "day", "d", -1)
			contractTimespan = strings.Replace(contractTimespan, "hr", "h", -1)
			contractTimespan = strings.Replace(contractTimespan, "min", "m", -1)
			contractTimespan = strings.Replace(contractTimespan, "sec", "s", -1)
			duration, err := str2duration.ParseDuration(contractTimespan)
			if err == nil {
				t.DurationTime = duration
				t.EstimatedEndTime = t.StartTime.Add(duration)
				str = fmt.Sprintf("Duration updated to %s.", duration.String())
			} else {
				str = "Invalid duration format. Use something like 19h35m."
			}
		}
		if input.CustomID == "since-start" && input.Value != "" {
			var err error
			// Timespan of the contract duration
			contractTimespan := strings.TrimSpace(input.Value)
			var contracTime int64

			// If this is a discord timestamp than use that
			startTimeArray := strings.Split(contractTimespan, ":")
			if len(startTimeArray) == 1 {
				contracTime, err = strconv.ParseInt(startTimeArray[0], 10, 64)
			} else {
				contracTime, err = strconv.ParseInt(startTimeArray[1], 10, 64)
			}

			if err == nil {
				t.StartTime = time.Unix(contracTime, 0)
				t.EstimatedEndTime = t.StartTime.Add(t.DurationTime)
			} else {

				contractTimespan = strings.Replace(contractTimespan, "day", "d", -1)
				contractTimespan = strings.Replace(contractTimespan, "hr", "h", -1)
				contractTimespan = strings.Replace(contractTimespan, "min", "m", -1)
				contractTimespan = strings.Replace(contractTimespan, "sec", "s", -1)
				sinceStart, err := str2duration.ParseDuration(contractTimespan)
				if err == nil {
					// Invalid duration, just assigning a 12h
					t.StartTime = time.Now().Add(-sinceStart)
					t.EstimatedEndTime = t.StartTime.Add(t.DurationTime)
					str += fmt.Sprintf("\nUpdate start time to <t:%d:R>", t.StartTime.Unix())
				} else {
					str += "\nInvalid start time format. Enter how long ago the contract started. Use something like 1h35m."
					str += "Optionally enter a timestamp from [Discord Timestamp](https://discordtimestamp.com)"
				}
			}
		}
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: str,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	//newDuration := data.Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value
	//newStarTime := data.Components[1].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value

	saveData(Tokens)
	//embed := getTokenTrackingEmbed(t, false)
	embed := TokenTrackingAdjustTime(i.ChannelID, userID, name, 0, 0, 0, 0)
	comp := getTokenValComponents(t.Name)
	m := discordgo.NewMessageEdit(t.UserChannelID, t.TokenMessageID)
	m.Components = &comp
	m.SetEmbeds(embed.Embeds)
	m.SetContent("")
	_, _ = s.ChannelMessageEditComplex(m)
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
			if len(t.Sent) < tokenIndex {
				return fmt.Sprintf("Invalid token index. You have sent %d tokens.", len(t.Sent))
			}
			removeSentToken(userID, tokenList, tokenIndex)
		} else {
			if len(t.Received) < tokenIndex {
				return fmt.Sprintf("Invalid token index. You have received %d tokens.", len(t.Received))
			}
			removeReceivedToken(userID, tokenList, tokenIndex)
		}
		str = "Token removed from tracking on <#" + Tokens[userID].Coop[tokenList].UserChannelID + ">."

		defer saveData(Tokens)
		embed := tokenTrackingTrack(userID, tokenList, 0, 0) // No sent or received
		comp := getTokenValComponents(tokenList)
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
			}})
	*/
	return "Select tracker to adjust the token.", choices

}
