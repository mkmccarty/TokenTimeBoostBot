package boost

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/rs/xid"
	"github.com/xhit/go-str2duration/v2"
)

// UpdateThreadName will update a threads name to the current contract state
func UpdateThreadName(s *discordgo.Session, contract *Contract) {
	if contract == nil {
		return
	}
	var builder strings.Builder
	builder.WriteString(generateThreadName(contract))

	for _, loc := range contract.Location {
		ch, err := s.Channel(loc.ChannelID)
		if err == nil {

			if ch.IsThread() {
				_, err := s.ChannelEdit(loc.ChannelID, &discordgo.ChannelEdit{
					Name: builder.String(),
				})
				if err != nil {
					log.Println("Error updating thread name", err)
				}
			}
		}
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
	var guestNames = ""
	var orderValue int = ContractOrderTimeBased // Default to Time Based
	var mention = ""
	var str = "Joining Member"
	var tokenWant = 0

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
			guestNames = farmerName
		}
		str += " " + farmerName
	}

	// TODO make this handle multiple farmers with tokens
	if opt, ok := optionMap["token-count"]; ok {
		tokenWant = int(opt.IntValue())
		str += " with " + fmt.Sprintf("%d", tokenWant) + " boost tokens"
		if guestNames == "" {
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

	if guestNames != "" {
		guestNames := strings.Split(guestNames, ",")
		for _, guestNameRaw := range guestNames {
			guestName := strings.TrimSpace(guestNameRaw)
			if tokenWant != 0 {
				farmerstate.SetTokens(guestName, tokenWant)
			}
			var err = AddContractMember(s, i.GuildID, i.ChannelID, i.Member.User.Mention(), "", guestName, orderValue)
			if err != nil {
				str = err.Error()
			}
		}
	} else {
		var err = AddContractMember(s, i.GuildID, i.ChannelID, i.Member.User.Mention(), mention, "", orderValue)
		if err != nil {
			str = err.Error()
		}
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
		if contract.CoopTokenValueMsgID != "" {
			HandleCoopTvalCommand(s, i)
		}

	}

	msg, _ := s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Flags:   discordgo.MessageFlagsEphemeral,
			Content: str,
		})
	_ = s.FollowupMessageDelete(i.Interaction, msg.ID)

}

// HandleTokenListAutoComplete will handle the /token-remove autocomplete
func HandleTokenListAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) (string, []*discordgo.ApplicationCommandOptionChoice) {
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

// HandleTokenIDAutoComplete will handle the /token-edit token-id autocomplete
func HandleTokenIDAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) (string, []*discordgo.ApplicationCommandOptionChoice) {
	// User interacting with bot, is this first time ?
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0)

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}
	c := FindContract(i.ChannelID)

	var myTokes []ei.TokenUnitLog
	for _, t := range c.TokenLog {
		if t.FromUserID == i.Member.User.ID && t.ToUserID != i.Member.User.ID {
			t.Value = getTokenValue(t.Time.Sub(c.StartTime).Seconds(), c.EstimatedDuration.Seconds()) * float64(t.Quantity)
			myTokes = append(myTokes, t)
		}
	}
	// Trim myTokes to last 10
	if len(myTokes) > 15 {
		myTokes = myTokes[len(myTokes)-15:]
	}

	for _, t := range myTokes {
		x, _ := xid.FromString(t.Serial)
		choice := discordgo.ApplicationCommandOptionChoice{
			Name:  fmt.Sprintf("%ds ago %s - %d @ %2.3f", int(time.Since(t.Time).Seconds()), t.ToNick, t.Quantity, t.Value),
			Value: x.Counter(),
		}
		choices = append(choices, &choice)
	}

	return "Select token to modify", choices
}

// HandleTokenReceiverAutoComplete will handle the /token-edit new-receiver autocomplete
func HandleTokenReceiverAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) (string, []*discordgo.ApplicationCommandOptionChoice) {
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0)

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}
	c := FindContract(i.ChannelID)
	if c == nil {
		return "Contract not found.", choices
	}
	searchString := ""

	/*
		if opt, ok := optionMap["new-receiver"]; ok {
			searchString = opt.IntValue()()
		}
	*/
	if searchString == "" {
		for i, o := range c.Order {
			b := c.Boosters[o]

			choice := discordgo.ApplicationCommandOptionChoice{
				Name:  b.Nick,
				Value: i,
			}
			choices = append(choices, &choice)
			if len(choices) > 15 {
				break
			}
		}
	} /* else {
		for i, o := range c.Order {
			b := c.Boosters[o]

			if strings.Contains(strings.ToLower(b.Nick), strings.ToLower(searchString)) {
				choice := discordgo.ApplicationCommandOptionChoice{
					Name:  b.Nick,
					Value: i,
				}
				choices = append(choices, &choice)
				if len(choices) > 15 {
					break
				}
			}
		}
	}
	*/
	return "Select new recipient", choices
}

// HandleTokenEditCommand will handle the /token-edit command
func HandleTokenEditCommand(s *discordgo.Session, i *discordgo.InteractionCreate) string {
	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}
	userID := getInteractionUserID(i)
	str := "No contract running here"
	c := FindContract(i.ChannelID)
	if c == nil {
		return "Contract not found."
	}
	if !userInContract(c, userID) {
		return "You are not in this contract."
	}
	var action int // 0:Move, 1: Delete, 2 Modify Count
	//var tokenCoop string
	var tokenIndex int32
	var boostIndex int64
	var tokenCount int64
	if opt, ok := optionMap["action"]; ok {
		action = int(opt.IntValue())
	}
	/*
		if opt, ok := optionMap["list"]; ok {
			tokenCoop = opt.StringValue()
		}
	*/
	if opt, ok := optionMap["id"]; ok {
		tokenIndex = int32(opt.IntValue())
	}
	if opt, ok := optionMap["new-receiver"]; ok {
		boostIndex = opt.IntValue()
	}
	if opt, ok := optionMap["new-count"]; ok {
		tokenCount = opt.IntValue()
	}

	if action == 0 { // Move
		str = "Token not found"
		for i, t := range c.TokenLog {
			xid, _ := xid.FromString(t.Serial)
			if xid.Counter() == tokenIndex {
				c.TokenLog[i].ToUserID = c.Order[boostIndex]
				c.TokenLog[i].ToNick = c.Boosters[c.Order[boostIndex]].Nick
				str = fmt.Sprintf("Token moved to %s", c.TokenLog[i].ToNick)
				break
			}
		}
	} else if action == 1 { // Delete str = "Token not found"
		for i, t := range c.TokenLog {
			xid, _ := xid.FromString(t.Serial)
			if xid.Counter() == tokenIndex {
				c.TokenLog = append(c.TokenLog[:i], c.TokenLog[i+1:]...)
				str = "Token deleted"
				break
			}
		}
	} else if action == 2 { // Modify Count
		str = "Token not found"
		for i, t := range c.TokenLog {
			xid, _ := xid.FromString(t.Serial)
			if xid.Counter() == tokenIndex {
				c.TokenLog[i].Quantity = int(tokenCount)
				c.TokenLog[i].Value = getTokenValue(c.TokenLog[i].Time.Sub(c.StartTime).Seconds(), c.EstimatedDuration.Seconds()) * float64(c.TokenLog[i].Quantity)
				str = "Token count modified"
				break
			}
		}
	}
	saveData(Contracts)
	return str
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
		} else {
			b.Received = append(b.Received[:tokenIndex], b.Received[tokenIndex+1:]...)
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
