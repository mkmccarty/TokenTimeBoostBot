package boost

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/xhit/go-str2duration/v2"
)

// UpdateThreadName will update a threads name to the current contract state
func UpdateThreadName(s *discordgo.Session, contract *Contract) {
	if contract == nil {
		return
	}
	var builder strings.Builder
	builder.WriteString(contract.CoopID + " ")
	if len(contract.Order) != contract.CoopSize {
		fmt.Fprintf(&builder, "(%d/%d)", len(contract.Order), contract.CoopSize)
	} else {
		builder.WriteString(" (full)")
	}
	if !contract.PlannedStartTime.IsZero() && contract.State == ContractStateSignup {
		nyTime, err := time.LoadLocation("America/New_York")
		if err != nil {
			log.Println("Failed to load New York time zone:", err)
			return
		}
		currentTime := contract.PlannedStartTime.In(nyTime)

		// Format the current time as a string
		formattedTime := currentTime.Format("15:04pm MST")

		// Append the formatted time to the thread name
		builder.WriteString(" " + formattedTime)

	}
	/*
		if contract.Speedrun {
			builder.WriteString(" " + speedrunStateNames[contract.SRData.SpeedrunState])
		} else {
			builder.WriteString(" " + contractStateNames[contract.State])

			// Get the current time in New York
		}
	*/

	for _, loc := range contract.Location {
		ch, err := s.Channel(loc.ChannelID)
		if err == nil {

			if ch.IsThread() {
				_, _ = s.ChannelEdit(loc.ChannelID, &discordgo.ChannelEdit{
					Name: builder.String(),
				})
			}
		}
	}
}

// HandleContractCommand will handle the /contract command
func HandleContractCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Protection against DM use
	if i.GuildID == "" {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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
	makeThread := false

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
		var c, err = s.Channel(ChannelID)
		if err != nil {
			coopID = c.Name
		}
	}
	if opt, ok := optionMap["make-thread"]; ok {
		makeThread = opt.BoolValue()
	}

	if coopSize == 0 {
		found := false
		for _, x := range ei.EggIncContracts {
			if x.ID == contractID {
				found = true
			}
		}
		if !found {
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content:    "Select a contract-id from the dropdown list.\nIf the contract-id list doesn't have your contract then supply a coop-size parameter.",
					Flags:      discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{}},
			})
			return
		}
	}

	// Create a new thread for this contract
	if makeThread {
		ch, err := s.Channel(ChannelID)
		if err == nil {
			if !ch.IsThread() {
				// Default to 1 day timeout
				var builder strings.Builder
				builder.WriteString(coopID)
				info := ei.EggIncContractsAll[contractID]
				if info.ID != "" {
					fmt.Fprintf(&builder, " (0/%d)", info.MaxCoopSize)
				}

				thread, err := s.ThreadStart(ChannelID, builder.String(), discordgo.ChannelTypeGuildPublicThread, 60*24)
				if err == nil {
					ChannelID = thread.ID
					_ = s.ThreadJoin(getInteractionUserID(i))
				} else {
					log.Print(err)
				}
			}
		}
	}

	mutex.Lock()
	contract, err := CreateContract(s, contractID, coopID, coopSize, boostOrder, i.GuildID, ChannelID, getInteractionUserID(i), pingRole)
	mutex.Unlock()
	if err != nil {

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    err.Error(),
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
		return
	}

	var builder strings.Builder
	builder.WriteString("Contract created. Use the Contract button if you have to recycle it.\n")
	builder.WriteString("This fastrun contract can be converted to a `/speedrun` anytime during the sign-up list.\n")
	builder.WriteString("If this contract isn't an immediate start use `/change-planned-start` to add the time to the sign-up message.\n")
	builder.WriteString("React with 🌊 to automaticaly update the thread name.")

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: builder.String(),
			Flags:   discordgo.MessageFlagsEphemeral,
			// Buttons and other components are specified in Components field.
		},
	})
	if err != nil {
		log.Print(err)
	}

	var createMsg = DrawBoostList(s, contract)
	var data discordgo.MessageSend
	data.Content = createMsg
	data.Flags = discordgo.MessageFlagsSuppressEmbeds
	msg, err := s.ChannelMessageSendComplex(ChannelID, &data)
	if err == nil {
		SetListMessageID(contract, ChannelID, msg.ID)
		var data discordgo.MessageSend
		data.Content, data.Components = GetSignupComponents(false, contract.Speedrun)
		reactionMsg, err := s.ChannelMessageSendComplex(ChannelID, &data)

		if err != nil {
			log.Print(err)
		} else {
			SetReactionID(contract, msg.ChannelID, reactionMsg.ID)
			_ = s.ChannelMessagePin(msg.ChannelID, reactionMsg.ID)
		}
	} else {
		log.Print(err)
	}
}

// HandleBoostCommand will handle the /boost command
func HandleBoostCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Protection against DM use
	if i.GuildID == "" {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "This command can only be run in a server.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}
	var str = "Boosting!!"
	var err = UserBoost(s, i.GuildID, i.ChannelID, i.Member.User.ID)
	if err != nil {
		str = err.Error()
	}
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: str,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

// HandleUnboostCommand will handle the /unboost command
func HandleUnboostCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Protection against DM use
	if i.GuildID == "" {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    "This command can only be run in a server.",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
		return
	}
	var str string
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

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Working on it...",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	var err = AddContractMember(s, i.GuildID, i.ChannelID, i.Member.User.Mention(), mention, guestName, orderValue)
	if err != nil {
		str = err.Error()
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Content: str,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	)
}

// HandlePruneCommand will handle the /prune command
func HandlePruneCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Protection against DM use
	if i.GuildID == "" {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    "Working on it...",
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})

	var err = RemoveFarmerByMention(s, i.GuildID, i.ChannelID, i.Member.User.Mention(), farmer)
	if err != nil {
		log.Println("/prune", err.Error())
		str = err.Error()
	}
	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Content: str},
	)
}

// HandleCoopETACommand will handle the /coopeta command
func HandleCoopETACommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Protection against DM use
	if i.GuildID == "" {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: str,
			//Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})

}

// HandleBumpCommand will handle the /bump command
func HandleBumpCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	str := "Contract not found"
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	contract := FindContract(i.ChannelID)
	if contract != nil {

		//	if contract.Speedrun && contract.SRData.SpeedrunState == SpeedrunStateCRT && contract.UseInteractionButtons == 0 {
		//	str = "Speedrun CRT is in progress, cannot bump as the message will lose reactions."
		//} else
		{
			str = "Boost list moved."
			err := RedrawBoostList(s, i.GuildID, i.ChannelID)
			if err != nil {
				str = err.Error()
			}
		}
	}

	msg, _ := s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Flags:   discordgo.MessageFlagsEphemeral,
			Content: str,
		})
	_ = s.FollowupMessageDelete(i.Interaction, msg.ID)
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
	if opt, ok := optionMap["alternate"]; ok {
		userID = opt.StringValue()
	}

	str := "No contract running here"
	c := FindContract(i.ChannelID)
	if c == nil {
		return "Contract not found."
	}
	if c.Boosters[userID] != nil {
		b := c.Boosters[userID]

		if tokenIndex > len(b.Sent) {
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

// HandleContractDelete facilitates the deletion of a channel contract
func HandleContractDelete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Delete coop
	var str = "Contract not found."
	// if user is contract coordinator
	contract := FindContract(i.ChannelID)

	if contract != nil {

		if creatorOfContract(s, contract, i.Member.User.ID) {

			coopName, err := DeleteContract(s, i.GuildID, i.ChannelID)
			if err == nil {
				str = fmt.Sprintf("Contract %s recycled.", coopName)
			}
			for _, loc := range contract.Location {
				_ = s.ChannelMessageUnpin(loc.ChannelID, loc.ReactionID)
			}
			_ = s.ChannelMessageDelete(i.ChannelID, i.Message.ID)
		} else {
			str = "Only the coordinator can recycle this contract."
		}
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: str,
			Flags:   discordgo.MessageFlagsEphemeral,
		}})
}
