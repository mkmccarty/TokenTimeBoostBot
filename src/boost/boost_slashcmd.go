package boost

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/xhit/go-str2duration/v2"
)

// HandleContractCommand will handle the /contract command
func HandleContractCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Protection against DM use
	if i.GuildID == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    "This command can only be run in a server.",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
		return
	}
	var contractID = i.GuildID
	var coopID = i.GuildID // Default to the Guild ID
	var boostOrder = ContractOrderSignup
	var coopSize = 0
	var ChannelID = i.ChannelID
	var pingRole = "@here"

	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["coop-size"]; ok {
		coopSize = int(opt.IntValue())
	}
	if opt, ok := optionMap["ping-role"]; ok {
		role := opt.RoleValue(nil, "")
		pingRole = role.Mention()
	}
	if opt, ok := optionMap["boost-order"]; ok {
		boostOrder = int(opt.IntValue())
	}
	if opt, ok := optionMap["contract-id"]; ok {
		contractID = opt.StringValue()
		contractID = strings.Replace(contractID, " ", "", -1)
	}
	if opt, ok := optionMap["coop-id"]; ok {
		coopID = opt.StringValue()
		coopID = strings.Replace(coopID, " ", "", -1)
	} else {
		var c, err = s.Channel(i.ChannelID)
		if err != nil {
			coopID = c.Name
		}
	}
	mutex.Lock()
	contract, err := CreateContract(s, contractID, coopID, coopSize, boostOrder, i.GuildID, i.ChannelID, i.Member.User.ID, pingRole)
	mutex.Unlock()
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    err.Error(),
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
		return
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Boost Order Management",
			Flags:   discordgo.MessageFlagsEphemeral,
			// Buttons and other components are specified in Components field.
			Components: []discordgo.MessageComponent{
				// ActionRow is a container of all buttons within the same row.
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Delete Contract",
							Style:    discordgo.DangerButton,
							Disabled: false,
							CustomID: "fd_delete",
						},
					},
				},
			},
		},
	})
	if err != nil {
		log.Print(err)
	}

	var createMsg = DrawBoostList(s, contract, FindTokenEmoji(s, i.GuildID))
	msg, err := s.ChannelMessageSend(ChannelID, createMsg)
	if err == nil {
		SetListMessageID(contract, ChannelID, msg.ID)
		var data discordgo.MessageSend
		data.Content, data.Components = GetSignupComponents(false, contract.Speedrun)
		reactionMsg, err := s.ChannelMessageSendComplex(ChannelID, &data)

		if err != nil {
			log.Print(err)
		} else {
			SetReactionID(contract, msg.ChannelID, reactionMsg.ID)
			s.ChannelMessagePin(msg.ChannelID, reactionMsg.ID)
		}
	} else {
		log.Print(err)
	}
}

// HandleBoostCommand will handle the /boost command
func HandleBoostCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Protection against DM use
	if i.GuildID == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    "This command can only be run in a server.",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
		return
	}
	var str = "Boosting!!"
	var err = UserBoost(s, i.GuildID, i.ChannelID, i.Member.User.ID)
	if err != nil {
		str = err.Error()
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    str,
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})
}

// HandleUnboostCommand will handle the /unboost command
func HandleUnboostCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Protection against DM use
	if i.GuildID == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    "This command can only be run in a server.",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
		return
	}
	var str = ""
	var farmer = ""
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["farmer"]; ok {
		farmer = opt.StringValue()
	}
	var err = Unboost(s, i.GuildID, i.ChannelID, farmer)
	if err != nil {
		str = err.Error()
	} else {
		str = "Marked " + farmer + " as unboosted."
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    str,
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})

}

// HandleSkipCommand will handle the /skip command
func HandleSkipCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Protection against DM use
	if i.GuildID == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    "This command can only be run in a server.",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
		return
	}
	var str = "Skip to Next Booster"
	var err = SkipBooster(s, i.GuildID, i.ChannelID, "")
	if err != nil {
		str = err.Error()
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    str,
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})
}

// HandleJoinCommand will handle the /join command
func HandleJoinCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Protection against DM use
	if i.GuildID == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    "This command can only be run in a server.",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
		return
	}
	var guestName = ""
	var orderValue int = ContractOrderTimeBased // Default to Time Based
	var mention = ""
	var str = "Joining Member"

	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["farmer"]; ok {
		farmerName := opt.StringValue()
		if strings.HasPrefix(farmerName, "<@") {
			mention = farmerName
		} else {
			guestName = farmerName
		}
		str += " " + farmerName
	}
	if opt, ok := optionMap["token-count"]; ok {
		tokenWant := int(opt.IntValue())
		str += " with " + fmt.Sprintf("%d", tokenWant) + " boost order"
		if guestName != "" {
			farmerstate.SetTokens(guestName, tokenWant)
		} else {
			farmerstate.SetTokens(mention[2:len(mention)-1], tokenWant)
		}
	}
	if opt, ok := optionMap["boost-order"]; ok {
		orderValue = int(opt.IntValue())
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    str,
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})

	var err = AddContractMember(s, i.GuildID, i.ChannelID, i.Member.User.Mention(), mention, guestName, orderValue)
	if err != nil {
		log.Println(err.Error())
	}

}

// HandlePruneCommand will handle the /prune command
func HandlePruneCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Protection against DM use
	if i.GuildID == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    "This command can only be run in a server.",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
		return
	}
	var str = "Prune Booster"
	var farmer = ""

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["farmer"]; ok {
		farmer = opt.StringValue()
		str += " " + farmer
	}

	var err = RemoveContractBoosterByMention(s, i.GuildID, i.ChannelID, i.Member.User.Mention(), farmer)
	if err != nil {
		log.Println("/prune", err.Error())
		str = err.Error()
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    str,
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})
}

// HandleCoopETACommand will handle the /coopeta command
func HandleCoopETACommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Protection against DM use
	if i.GuildID == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    "This command can only be run in a server.",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
		return
	}
	var rate = ""
	var t = time.Now()
	var timespan = ""

	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["rate"]; ok {
		rate = opt.StringValue()
	}
	if opt, ok := optionMap["timespan"]; ok {
		timespan = opt.StringValue()
	}

	dur, _ := str2duration.ParseDuration(timespan)
	endTime := t.Add(dur)

	var str = fmt.Sprintf("With a production rate of %s/hr completion <t:%d:R> near <t:%d:f>", rate, endTime.Unix(), endTime.Unix())

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: str,
			//Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})

}

// HandleBumpCommand will handle the /bump command
func HandleBumpCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	str := "Redrawing the boost list"
	err := RedrawBoostList(s, i.GuildID, i.ChannelID)
	if err != nil {
		str = err.Error()
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    str,
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})
}

// HandleChangeCommand will handle the /change command
func HandleChangeCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Protection against DM use
	if i.GuildID == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    "This command can only be run in a server.",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
		return
	}
	var str = ""
	var contractID = ""
	var coopID = ""
	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["ping-role"]; ok {
		role := opt.RoleValue(nil, "")
		err := ChangePingRole(s, i.GuildID, i.ChannelID, i.Member.User.ID, role.Mention())
		if err != nil {
			str += err.Error()
		} else {
			str = "Changed ping role to " + role.Mention() + "\n"
		}
	}
	if opt, ok := optionMap["contract-id"]; ok {
		contractID = opt.StringValue()
		contractID = strings.Replace(contractID, " ", "", -1)
		str += "Contract ID changed to " + contractID
	}
	if opt, ok := optionMap["coop-id"]; ok {
		coopID = opt.StringValue()
		coopID = strings.Replace(coopID, " ", "", -1)
		str += "Coop ID changed to " + coopID
	}

	if contractID != "" || coopID != "" {
		err := ChangeContractIDs(s, i.GuildID, i.ChannelID, i.Member.User.ID, contractID, coopID)
		if err != nil {
			str += err.Error()
		}
	}

	currentBooster := ""
	boostOrder := ""
	oneBoosterName := ""
	oneBoosterPosition := 0
	if opt, ok := optionMap["current-booster"]; ok {
		currentBooster = strings.TrimSpace(opt.StringValue())
	}
	if opt, ok := optionMap["boost-order"]; ok {
		boostOrder = strings.TrimSpace(opt.StringValue())
	}
	if opt, ok := optionMap["one-boost-position"]; ok {
		// String in the form of mention
		boosterString := strings.TrimSpace(opt.StringValue())

		// split string into slice by space, comma or colon
		boosterSlice := strings.FieldsFunc(boosterString, func(r rune) bool {
			return r == ' ' || r == ',' || r == ':'
		})
		if len(boosterSlice) >= 2 {

			// booster name is boosterString without the last element of boosterSlice
			oneBoosterName = strings.TrimSuffix(boosterString, boosterSlice[len(boosterSlice)-1])
			oneBoosterName = strings.TrimSpace(oneBoosterName)
			// Trim last character from oneBoosterName
			oneBoosterName = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(oneBoosterName, ":"), ","))

			re := regexp.MustCompile(`[\\<>@#&!]`)
			oneBoosterName = re.ReplaceAllString(oneBoosterName, "")

			// convert string to int
			oneBoosterPosition, _ = strconv.Atoi(boosterSlice[1])
		} else {
			str = "The one-boost-position parameter needs to be in the form of @farmer <space> 4"
		}
	}

	// Either change a single booster or the whole list
	// Cannot change one booster's position and make them boost
	if oneBoosterName != "" && oneBoosterPosition != 0 {
		err := MoveBooster(s, i.GuildID, i.ChannelID, i.Member.User.ID, oneBoosterName, oneBoosterPosition, currentBooster == "")
		if err != nil {
			str += err.Error()
		} else {
			str += fmt.Sprintf("Move <@%s> to position %d.", oneBoosterName, oneBoosterPosition)
		}
	} else {
		if boostOrder != "" {
			err := ChangeBoostOrder(s, i.GuildID, i.ChannelID, i.Member.User.ID, boostOrder, currentBooster == "")
			if err != nil {
				str += err.Error()
			} else {
				str += fmt.Sprintf("Change Boost Order to %s.", boostOrder)
			}
		}
	}

	if currentBooster != "" {
		err := ChangeCurrentBooster(s, i.GuildID, i.ChannelID, i.Member.User.ID, currentBooster, true)
		if err != nil {
			str += err.Error()
		} else {
			str += fmt.Sprintf("Current changed to <@%s>.", currentBooster)
		}
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    str,
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})

}

// HandleTokenRemoveAutoComplete will handle the /token-remove autocomplete
func HandleTokenRemoveAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) (string, []*discordgo.ApplicationCommandOptionChoice) {
	// User interacting with bot, is this first time ?
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0)

	c := FindContract(i.ChannelID)

	choice := discordgo.ApplicationCommandOptionChoice{
		Name:  c.ContractID + "/" + c.CoopID,
		Value: c.CoopID,
	}
	choices = append(choices, &choice)

	return "Select tracker to adjust the token.", choices
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

	var tokenType int
	var tokenIndex int

	if opt, ok := optionMap["token-type"]; ok {
		tokenType = int(opt.IntValue())
	}
	if opt, ok := optionMap["token-index"]; ok {
		tokenIndex = int(opt.IntValue())
	}

	str := "No contract running here"
	c := FindContract(i.ChannelID)
	if c == nil {
		return "Contract not found."
	}
	if c.Boosters[userID] != nil {
		b := c.Boosters[userID]

		if tokenIndex >= len(b.Sent) {
			return fmt.Sprintf("There are only %d tokens to remove.", len(b.Sent))
		}
		tokenIndex--

		// Need to figure out which list to remove from
		if tokenType == 0 {
			b.Sent = append(b.Sent[:tokenIndex], b.Sent[tokenIndex+1:]...)
			//b.TokenSentTime = append(b.TokenSentTime[:tokenIndex], b.TokenSentTime[tokenIndex+1:]...)
			//b.TokenSentName = append(b.TokenSentName[:tokenIndex], b.TokenSentName[tokenIndex+1:]...)
		} else {
			b.Received = append(b.Received[:tokenIndex], b.Received[tokenIndex+1:]...)
			//b.TokenReceivedTime = append(b.TokenReceivedTime[:tokenIndex], b.TokenReceivedTime[tokenIndex+1:]...)
			//b.TokenReceivedName = append(b.TokenReceivedName[:tokenIndex], b.TokenReceivedName[tokenIndex+1:]...)
		}
		str = "Token removed from tracking on <#" + i.ChannelID + ">."
	}
	saveData(Contracts)
	return str
}
