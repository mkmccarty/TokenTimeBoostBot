package track

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/moby/moby/pkg/namesgenerator"
	"github.com/rs/xid"
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
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "contract-id",
				Description:  "Contract ID (ie. spring-2025)",
				Required:     false,
				Autocomplete: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "coop-id",
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
func HandleTokenCommand(s *discordgo.Session, i *discordgo.InteractionCreate, contractID string, coopID string, contractStartTime time.Time, pastTokens *[]ei.TokenUnitLog, linked bool) {
	optionMap := bottools.GetCommandOptionsMap(i)
	channelID := i.ChannelID
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
		contractTimespan := bottools.SanitizeStringDuration(opt.StringValue())
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
	var trackingName = coopID
	if opt, ok := optionMap["coop-id"]; ok {
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
	startTime := time.Now()
	if pastTokens != nil {
		startTime = contractStartTime
	}

	str, embed, err := tokenTracking(s, channelID, userID, trackingName, contractID, duration, linked, linkReceived, startTime, pastTokens)

	if err != nil {
		str = err.Error()
	} else {
		var data discordgo.MessageSend
		data.Embeds = embed.Embeds
		data.Components = getTokenValComponents(trackingName, linked) // Initial state

		u, err := s.UserChannelCreate(userID)
		if err == nil {
			msg, err := s.ChannelMessageSendComplex(u.ID, &data)
			if err == nil {
				Tokens[userID].Coop[trackingName].TokenMessageID = msg.ID
				Tokens[userID].Coop[trackingName].UserChannelID = u.ID

				str += "Interact with the bot on " + u.Mention() + " to track your token values."
				farmerstate.SetLink(userID, "Tracking:"+trackingName, i.GuildID, i.ChannelID, "")
				farmerstate.SetLink(userID, "Tracker:"+trackingName, "", u.ID, msg.ID)
			} else {
				str = "Error writing to channel. " + err.Error()
			}
		} else {
			str = fmt.Sprintf("Error creating user channel. %s", err.Error())
			log.Println(str)
		}
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
		if input.CustomID == "contract-id" && input.Value != "" {
			contractID := strings.TrimSpace(input.Value)
			t.ContractID = contractID
			t.TimeFromCoopStatus = time.Time{}
			str = fmt.Sprintf("Contract ID updated to %s.", contractID)
		}
		if input.CustomID == "coop-id" && input.Value != "" {
			coopID := strings.TrimSpace(input.Value)
			t.CoopID = coopID
			t.TimeFromCoopStatus = time.Time{}
			str = fmt.Sprintf("Coop ID updated to %s.", coopID)
		}
		if input.CustomID == "duration" && input.Value != "" {
			// Timespan of the contract duration
			contractTimespan := bottools.SanitizeStringDuration(input.Value)
			duration, err := str2duration.ParseDuration(contractTimespan)
			if err == nil {
				t.DurationTime = duration
				t.EstimatedEndTime = t.StartTime.Add(duration)
				str = fmt.Sprintf("Duration updated to %s.", duration.String())
			} else {
				str = "Invalid duration format. Use something like 19h35m."
			}
		}
		if input.CustomID == "start-timestamp" && input.Value != "" {
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
				str += fmt.Sprintf("\nUpdate start time to <t:%d:R>", t.StartTime.Unix())
			} else {
				str = "Optionally enter a timestamp from [Discord Timestamp](https://discordtimestamp.com)"
			}
		}
		if input.CustomID == "since-start" && input.Value != "" {
			var err error
			// Timespan of the contract duration
			contractTimespan := bottools.SanitizeStringDuration(input.Value)
			sinceStart, err := str2duration.ParseDuration(contractTimespan)
			if err == nil {
				// Invalid duration, just assigning a 12h
				t.StartTime = time.Now().Add(-sinceStart)
				t.EstimatedEndTime = t.StartTime.Add(t.DurationTime)
				str += fmt.Sprintf("\nUpdate start time to <t:%d:R>", t.StartTime.Unix())
			} else {
				str += "\nInvalid start time format. Enter how long ago the contract started. Use something like 1h35m."
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
	embed := TokenTrackingAdjustTime(i.ChannelID, userID, name, 0, 0, 0, 0, time.Time{}, 0)
	comp := getTokenValComponents(t.Name, t.Linked && !t.LinkedCompleted)
	m := discordgo.NewMessageEdit(t.UserChannelID, t.TokenMessageID)
	m.Components = &comp
	m.SetEmbeds(embed.Embeds)
	m.SetContent("")
	_, _ = s.ChannelMessageEditComplex(m)
}

// HandleTokenEditTrackCommand will handle the /token-edit command
func HandleTokenEditTrackCommand(s *discordgo.Session, i *discordgo.InteractionCreate) string {
	optionMap := bottools.GetCommandOptionsMap(i)

	var userID string

	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}

	var action int   // 0:Move, 1: Delete, 2 Modify Count
	var typeList int // 1: Sent, 2: Received
	var tokenCoop string
	var tokenIndex int32
	var tokenCount int64
	if opt, ok := optionMap["action"]; ok {
		action = int(opt.IntValue())
	}
	if opt, ok := optionMap["type"]; ok {
		typeList = int(opt.IntValue())
	}

	if opt, ok := optionMap["list"]; ok {
		tokenCoop = opt.StringValue()
	}

	if opt, ok := optionMap["id"]; ok {
		tokenIndex = int32(opt.IntValue())
	}
	if opt, ok := optionMap["new-quantity"]; ok {
		tokenCount = opt.IntValue()
	}

	tracker := Tokens[userID].Coop[tokenCoop]
	if tracker == nil {
		return "Contract not found."
	}

	str := "Token not found"
	if action == 1 { // Delete
		if typeList == 1 { // Sent
			for i, t := range tracker.Sent {
				xid, _ := xid.FromString(t.Serial)
				if xid.Counter() == tokenIndex {
					tracker.Sent = append(tracker.Sent[:i], tracker.Sent[i+1:]...)
					str = "Token deleted"
					break
				}
			}
		} else { // Received
			for i, t := range tracker.Received {
				xid, _ := xid.FromString(t.Serial)
				if xid.Counter() == tokenIndex {
					tracker.Received = append(tracker.Received[:i], tracker.Received[i+1:]...)
					str = "Token deleted"
					break
				}
			}
		}
	} else if action == 2 { // Modify Count
		if typeList == 1 { // Sent
			for i, t := range tracker.Sent {
				xid, _ := xid.FromString(t.Serial)
				if xid.Counter() == tokenIndex {
					tracker.Sent[i].Quantity = int(tokenCount)
					tracker.Sent[i].Value = getTokenValue(tracker.Sent[i].Time.Sub(tracker.StartTime).Seconds(), float64(tracker.DurationTime.Seconds())) * float64(tracker.Sent[i].Quantity)
					str = "Token count modified"
					break
				}
			}
		} else { // Received
			for i, t := range tracker.Received {
				xid, _ := xid.FromString(t.Serial)
				if xid.Counter() == tokenIndex {
					tracker.Received[i].Quantity = int(tokenCount)
					tracker.Received[i].Value = getTokenValue(tracker.Received[i].Time.Sub(tracker.StartTime).Seconds(), float64(tracker.DurationTime.Seconds())) * float64(tracker.Received[i].Quantity)
					str = "Token count modified"
					break
				}
			}
		}
	}

	// Recalculate the token values
	tracker.SumValueSent = 0
	tracker.SumValueReceived = 0
	tracker.SentCount = 0
	tracker.ReceivedCount = 0
	for i, t := range tracker.Sent {
		//tracker.Sent[i].Value = getTokenValue(t.Time.Sub(tracker.StartTime).Seconds(), float64(tracker.DurationTime.Seconds()))
		tracker.SentCount += t.Quantity
		tracker.SumValueSent += tracker.Sent[i].Value
	}
	for i, t := range tracker.Received {
		//tracker.Received[i].Value = getTokenValue(t.Time.Sub(tracker.StartTime).Seconds(), float64(tracker.DurationTime.Seconds()))
		tracker.ReceivedCount += t.Quantity
		tracker.SumValueReceived += tracker.Received[i].Value
	}

	saveData(Tokens)
	embed, linked := tokenTrackingTrack(userID, tokenCoop, 0, 0) // No sent or received
	comp := getTokenValComponents(tokenCoop, linked)
	m := discordgo.NewMessageEdit(Tokens[userID].Coop[tokenCoop].UserChannelID, Tokens[userID].Coop[tokenCoop].TokenMessageID)
	m.Components = &comp
	m.SetEmbeds(embed.Embeds)
	m.SetContent("")
	_, err := s.ChannelMessageEditComplex(m)
	if err != nil {
		str = "Error updating the token message. " + err.Error()
	}

	return str
}

// HandleTokenListAutoComplete will handle the /token-remove autocomplete
func HandleTokenListAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) (string, []*discordgo.ApplicationCommandOptionChoice) {
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0)

	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}

	if _, ok := Tokens[userID]; !ok {
		return "No trackers found", nil
	}
	for _, c := range Tokens[userID].Coop {
		choice := discordgo.ApplicationCommandOptionChoice{
			Name:  c.Name,
			Value: c.Name,
		}
		choices = append(choices, &choice)
	}
	return "Select tracker to adjust the token.", choices

}

// GetSlashTokenEditTrackCommand returns the slash command for token tracking removal
func GetSlashTokenEditTrackCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Edit a tracked token",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextBotDM,
			discordgo.InteractionContextPrivateChannel,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
			discordgo.ApplicationIntegrationUserInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "action",
				Description: "Select the auction to take",
				Type:        discordgo.ApplicationCommandOptionInteger,
				Required:    true,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{
						Name:  "Delete Token",
						Value: 1,
					},
					{
						Name:  "Modify Token Count",
						Value: 2,
					},
				},
			},
			{
				Name:        "type",
				Description: "Which type of token to modify",
				Type:        discordgo.ApplicationCommandOptionInteger,
				Required:    true,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{
						Name:  "Sent",
						Value: 1,
					},
					{
						Name:  "Received",
						Value: 2,
					},
				},
			},
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "list",
				Description:  "The tracking list to remove the token from.",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:         discordgo.ApplicationCommandOptionInteger,
				Name:         "id",
				Description:  "Select the token to modify (last 15)",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "new-quantity",
				Description: "Update token count",
				Required:    false,
				MinValue:    &integerOneMinValue,
				MaxValue:    32,
			},
		},
	}
}

// HandleTokenIDAutoComplete will handle the /token-remove autocomplete
func HandleTokenIDAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) (string, []*discordgo.ApplicationCommandOptionChoice) {
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0)
	optionMap := bottools.GetCommandOptionsMap(i)
	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}

	var tokenlist string

	if opt, ok := optionMap["list"]; ok {
		tokenlist = opt.StringValue()
	} else {
		return "Select token list first", choices
	}

	if _, ok := Tokens[userID]; !ok {
		return "", nil
	}

	// Create a list of choices from the last 7 sent and received tokens (14 total)
	c := Tokens[userID].Coop[tokenlist]
	if c == nil {
		return "Tracker not found", nil
	}

	listType := 1 // Sent

	// Which list ?
	if opt, ok := optionMap["type"]; ok {
		listType = int(opt.IntValue())
	}

	if listType == 1 {
		// Last 7 sent tokens from c.Sent and last 7 received tokens from c.Received
		for i := len(c.Sent) - 1; i >= 0 && len(choices) < 10; i-- {
			t := c.Sent[i]
			x, _ := xid.FromString(t.Serial)
			choice := &discordgo.ApplicationCommandOptionChoice{
				Name:  fmt.Sprintf("%ds ago %s - %d @ %2.3f", int(time.Since(t.Time).Seconds()), t.UserID, t.Quantity, t.Value),
				Value: x.Counter(),
			}
			choices = append(choices, choice)
		}
	} else {
		for i := len(c.Received) - 1; i >= 0 && len(choices) < 10; i-- {
			t := c.Received[i]
			x, _ := xid.FromString(t.Serial)
			choice := &discordgo.ApplicationCommandOptionChoice{
				Name:  fmt.Sprintf("%ds ago %s - %d @ %2.3f", int(time.Since(t.Time).Seconds()), t.UserID, t.Quantity, t.Value),
				Value: x.Counter(),
			}
			choices = append(choices, choice)
		}
	}

	return "Select token to modify", choices

}
