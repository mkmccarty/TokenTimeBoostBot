package boost

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/track"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/xid"
	"github.com/xhit/go-str2duration/v2"
)

// UpdateThreadName will update a threads name to the current contract state
func UpdateThreadName(s *discordgo.Session, contract *Contract) {
	if contract == nil {
		return
	}

	contract.ThreadRenameTime = time.Now()

	var builder strings.Builder
	builder.WriteString(generateThreadName(contract))
	contract.ThreadRenameTime = time.Now()

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
	optionMap := bottools.GetCommandOptionsMap(i)

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
	var orderValue = ContractOrderTimeBased // Default to Time Based
	var mention = ""
	var str = "Joining Member"
	var tokenWant = 0

	optionMap := bottools.GetCommandOptionsMap(i)

	if opt, ok := optionMap["farmer"]; ok {
		farmerName := opt.StringValue()
		if _, isMention := parseMentionUserID(farmerName); isMention {
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
			farmerstate.SetTokens(normalizeUserIDInput(mention), tokenWant)
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

	optionMap := bottools.GetCommandOptionsMap(i)

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

	optionMap := bottools.GetCommandOptionsMap(i)
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

// HandleToggleContractPingsCommand will handle the /toggle-contract-pings command
func HandleToggleContractPingsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	str := "Contract not found"
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	userID := getInteractionUserID(i)
	contract := FindContract(i.ChannelID)

	userInContract := userInContract(contract, userID)
	if contract != nil && userInContract {
		value := farmerstate.GetMiscSettingFlag(userID, "SuppressContractPings")
		value = !value
		farmerstate.SetMiscSettingFlag(userID, "SuppressContractPings", value)
		for _, loc := range contract.Location {
			if loc.GuildID == i.GuildID {
				if value {
					str = fmt.Sprintf("Suppressing contract pings.\nRemoving you from this contract's %s role.", loc.GuildContractRole.Name)
					_ = s.GuildMemberRoleRemove(i.GuildID, userID, loc.GuildContractRole.ID)
				} else {
					str = fmt.Sprintf("Enabling contract pings.\nAdding you to this contract's %s role.", loc.GuildContractRole.Name)
					_ = s.GuildMemberRoleAdd(i.GuildID, userID, loc.GuildContractRole.ID)
				}
			}
		}
	} else {
		str = "You are not in this contract."
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Flags:   discordgo.MessageFlagsEphemeral,
			Content: str,
		})

}

// HandleTokenListAutoComplete will handle the /token-remove autocomplete
func HandleTokenListAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) (string, []*discordgo.ApplicationCommandOptionChoice) {
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0)

	c := FindContract(i.ChannelID)

	if c == nil {
		return "Contract not found.", choices
	}

	choice := discordgo.ApplicationCommandOptionChoice{
		Name:  c.ContractID + "/" + c.CoopID,
		Value: c.CoopID,
	}
	choices = append(choices, &choice)

	return "Select tracker to adjust the token.", choices
}

// HandleTokenIDAutoComplete will handle the /token-edit token-id autocomplete
func HandleTokenIDAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) (string, []*discordgo.ApplicationCommandOptionChoice) {
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0)

	//optionMap := bottools.GetCommandOptionsMap(i)
	c := FindContract(i.ChannelID)

	if c == nil {
		return "Contract not found.", choices
	}

	var myTokes []ei.TokenUnitLog
	for _, t := range c.TokenLog {
		if t.FromUserID == i.Member.User.ID {
			t.Value = bottools.GetTokenValue(t.Time.Sub(c.StartTime).Seconds(), c.EstimatedDuration.Seconds()) * float64(t.Quantity)
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

	optionMap := bottools.GetCommandOptionsMap(i)

	c := FindContract(i.ChannelID)
	if c == nil {
		return "Contract not found.", choices
	}
	searchString := ""

	if opt, ok := optionMap["new-receiver"]; ok {
		searchString = opt.StringValue()
	}

	// Want a set of sorted keys from c.Boosters
	// Sort by Nick
	keys := make([]string, 0, len(c.Boosters))
	for k := range c.Boosters {

		if searchString == "" || strings.Contains(strings.ToLower(c.Boosters[k].Nick), strings.ToLower(searchString)) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	for _, b := range keys {
		choice := discordgo.ApplicationCommandOptionChoice{
			Name:  c.Boosters[b].Nick,
			Value: b,
		}
		choices = append(choices, &choice)
		if len(choices) > 15 {
			break
		}
	}
	/* else {
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
	optionMap := bottools.GetCommandOptionsMap(i)

	userID := getInteractionUserID(i)
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
	var boosterIndex string
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
		boosterIndex = opt.StringValue()
	}
	if opt, ok := optionMap["new-quantity"]; ok {
		tokenCount = opt.IntValue()
	}

	modifiedTokenLog := ei.TokenUnitLog{}

	str := "Token not found"
	c.mutex.Lock()
	if action == 0 { // Move
		for i, t := range c.TokenLog {
			xid, _ := xid.FromString(t.Serial)
			if xid.Counter() == tokenIndex {
				c.TokenLog[i].ToUserID = c.Boosters[boosterIndex].UserID
				c.TokenLog[i].ToNick = c.Boosters[boosterIndex].Nick
				modifiedTokenLog = c.TokenLog[i]
				str = fmt.Sprintf("Token moved to %s", c.TokenLog[i].ToNick)
				break
			}
		}
	} else if action == 1 { // Delete str = "Token not found"
		for i, t := range c.TokenLog {
			xid, _ := xid.FromString(t.Serial)
			if xid.Counter() == tokenIndex {
				modifiedTokenLog = c.TokenLog[i]
				modifiedTokenLog.Quantity = 0
				modifiedTokenLog.ToNick = "Deleted"
				modifiedTokenLog.ToUserID = "Deleted"
				modifiedTokenLog.FromNick = "Deleted"
				modifiedTokenLog.FromUserID = "Deleted"
				c.TokenLog = append(c.TokenLog[:i], c.TokenLog[i+1:]...)
				str = "Token deleted"
				break
			}
		}
	} else if action == 2 { // Modify Count
		for i, t := range c.TokenLog {
			xid, _ := xid.FromString(t.Serial)
			if xid.Counter() == tokenIndex {
				c.TokenLog[i].Quantity = int(tokenCount)
				c.TokenLog[i].Value = bottools.GetTokenValue(c.TokenLog[i].Time.Sub(c.StartTime).Seconds(), c.EstimatedDuration.Seconds()) * float64(c.TokenLog[i].Quantity)
				modifiedTokenLog = c.TokenLog[i]
				str = "Token count modified"
				break
			}
		}
	}
	// Recalculate token values after the change
	targetTval := GetTargetTval(c.SeasonalScoring, c.EstimatedDuration.Minutes(), float64(c.MinutesPerToken))
	calculateTokenValueCoopLog(c, c.EstimatedDuration, targetTval)

	c.mutex.Unlock()
	track.ContractTokenUpdate(s, i.ChannelID, &modifiedTokenLog)
	saveData(c.ContractHash)
	refreshBoostListMessage(s, c, false)
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
