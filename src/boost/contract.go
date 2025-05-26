package boost

import (
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/moby/moby/pkg/namesgenerator"
)

// GetSlashContractCommand returns the slash command for creating a contract
func GetSlashContractCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Create a contract boost list.",
		/*
			Contexts: &[]discordgo.InteractionContextType{
				discordgo.InteractionContextGuild,
				discordgo.InteractionContextBotDM,
				discordgo.InteractionContextPrivateChannel,
			},
			IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
				discordgo.ApplicationIntegrationGuildInstall,
				discordgo.ApplicationIntegrationUserInstall,
			},
		*/
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "contract-id",
				Description:  "Contract ID",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "coop-id",
				Description: "Coop ID",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "coop-size",
				Description: "Co-op Size. This will be pulled from EI Contract data if unset.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionRole,
				Name:        "ping-role",
				Description: "Role to use to ping for this contract. Default is @here.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "make-thread",
				Description: "Create a thread for this contract? (default: true)",
				Required:    false,
			},
		},
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

	// Initial response to the user
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    "Processing...",
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})

	ch, err := s.Channel(i.ChannelID)
	if err != nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content:    "No permissions to write to this channel.",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{},
			},
		)
		return
	}

	var contractID = i.GuildID
	var coopID = i.GuildID // Default to the Guild ID
	var boostOrder = ContractOrderSignup
	var coopSize = 0
	var ChannelID = i.ChannelID
	var pingRole = "@here"
	makeThread := true // Default is to always make a thread

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
		contractID = strings.ReplaceAll(contractID, " ", "")
	}
	if opt, ok := optionMap["coop-id"]; ok {
		coopID = opt.StringValue()
		coopID = strings.ReplaceAll(coopID, " ", "")
	} else {
		var c, err = s.Channel(ChannelID)
		if err != nil {
			coopID = c.Name
		}
	}

	if ch.IsThread() {
		makeThread = false
	} else {
		// Is the bot allowed to create a thread?
		perms, err := s.UserChannelPermissions(config.DiscordAppID, i.ChannelID)
		if err == nil && perms&discordgo.PermissionCreatePublicThreads != 0 {
			if opt, ok := optionMap["make-thread"]; ok {
				makeThread = opt.BoolValue()
			}
		} else {
			makeThread = false
		}
	}

	if coopSize == 0 {
		found := false
		for _, x := range ei.EggIncContracts {
			if x.ID == contractID {
				found = true
			}
		}
		if !found {
			_, _ = s.FollowupMessageCreate(i.Interaction, true,
				&discordgo.WebhookParams{
					Content:    "Select a contract-id from the dropdown list.\nIf the contract-id list doesn't have your contract then supply a coop-size parameter.",
					Flags:      discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{},
				},
			)
			return
		}
	}

	// Before we make a thread, make sure this isn't a duplicate contract
	for _, c := range Contracts {
		if c.ContractID == contractID && c.CoopID == coopID {
			_, _ = s.FollowupMessageCreate(i.Interaction, true,
				&discordgo.WebhookParams{
					Content:    "A contract with this coop-id (" + c.CoopID + ") exists in " + c.Location[0].ChannelMention,
					Flags:      discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{},
				},
			)
			return
		}
	}

	// Create a new thread for this contract
	if makeThread {
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

	mutex.Lock()
	contract, err := CreateContract(s, contractID, coopID, coopSize, boostOrder, i.GuildID, ChannelID, getInteractionUserID(i), pingRole)
	mutex.Unlock()

	if err != nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content:    err.Error(),
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{},
			},
		)
		return
	}

	if len(contract.Location) == 1 {
		str, comp := getSignupContractSettings(contract.Location[0].ChannelID, contract.ContractHash, makeThread)

		if ChannelID != i.ChannelID {
			str += "\nThis message can be moved into the contract thread via `/contract-settings` command in that thread."
		}

		_, _ = s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content:    str,
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: comp,
			},
		)
	} else {
		_, _ = s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content: "This contract was initiated in <#" + contract.Location[0].ChannelID + ">. The coordinator will take care of the options.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		)
	}

	var createMsg = DrawBoostList(s, contract)
	var data discordgo.MessageSend
	data.Components = createMsg
	data.Flags = discordgo.MessageFlagsIsComponentsV2
	msg, err := s.ChannelMessageSendComplex(ChannelID, &data)
	if err == nil {
		var components []discordgo.MessageComponent
		SetListMessageID(contract, ChannelID, msg.ID)
		var data discordgo.MessageSend
		data.Flags = discordgo.MessageFlagsIsComponentsV2

		contentStr, comp := GetSignupComponents(false, contract)
		components = append(components, &discordgo.TextDisplay{
			Content: contentStr,
		})
		components = append(components, comp...)
		data.Components = components

		reactionMsg, err := s.ChannelMessageSendComplex(ChannelID, &data)

		if err != nil {
			log.Print(err)
		} else {
			SetReactionID(contract, msg.ChannelID, reactionMsg.ID)
			_ = s.ChannelMessagePin(msg.ChannelID, reactionMsg.ID)
		}
		// Auto join the caller into this contract
		// TODO: This will end up causing a double draw of the contract list
		_ = JoinContract(s, i.GuildID, ChannelID, getInteractionUserID(i), false)

	} else {
		log.Print(err)
	}
}

// CreateContract creates a new contract or joins an existing contract if run from a different location
func CreateContract(s *discordgo.Session, contractID string, coopID string, coopSize int, BoostOrder int, guildID string, channelID string, userID string, pingRole string) (*Contract, error) {
	// When creating contracts, we can make sure to clean up and archived ones
	// Just in case a contract was immediately recreated
	for _, c := range Contracts {
		if c.State == ContractStateArchive {
			if c.CalcOperations == 0 || time.Since(c.CalcOperationTime).Minutes() > 20 {
				FinishContract(s, c)
			}
		}
	}
	/*
		if boostIcon == "🚀" {
			boostIconName = "chickenboost"
			boostIconReaction = findBoostBotGuildEmoji(s, boostIconName, true)
			boostIcon = boostIconReaction + ">"
		}
	*/

	// Make sure this channel doesn't already have a contract
	existingContract := FindContract(channelID)
	if existingContract != nil {
		return nil, errors.New("this channel already has a contract named: " + existingContract.ContractID + "/" + existingContract.CoopID)
	}

	var contract *Contract
	// Does a coop already exist for this contract-id and coop-id
	for _, c := range Contracts {
		if c.ContractID == contractID && c.CoopID == coopID {
			// We have a coop, add this channel to the coop
			return nil, errors.New("a contract with this coop-id (" + c.CoopID + ") exists in " + c.Location[0].ChannelMention)
			//contract = c
		}
	}

	loc := new(LocationData)
	loc.GuildID = guildID
	loc.ChannelID = channelID
	var g, gerr = s.Guild(guildID)
	if gerr == nil {
		loc.GuildName = g.Name

	}
	var c, cerr = s.Channel(channelID)
	if cerr == nil {
		loc.ChannelMention = c.Mention()
		loc.ChannelPing = pingRole
	}
	loc.ListMsgID = ""
	loc.ReactionID = ""

	//if contract == nil {
	var ContractHash = namesgenerator.GetRandomName(0)
	for Contracts[ContractHash] != nil {
		ContractHash = namesgenerator.GetRandomName(0)
	}

	// We don't have this contract on this channel, it could exist in another channel
	contract = new(Contract)
	contract.Location = append(contract.Location, loc)
	contract.ContractHash = ContractHash
	//	contract.UseInteractionButtons = config.GetTestMode() // Feature under test

	contract.Style = ContractStyleFastrun

	//GlobalContracts[ContractHash] = append(GlobalContracts[ContractHash], loc)
	contract.Boosters = make(map[string]*Booster)
	contract.ContractID = contractID
	contract.CoopID = coopID
	contract.BoostOrder = BoostOrder
	contract.BoostVoting = 0
	contract.OrderRevision = 0
	changeContractState(contract, ContractStateSignup)
	contract.CreatorID = append(contract.CreatorID, userID)               // starting userid
	contract.CreatorID = append(contract.CreatorID, config.AdminUsers...) // Admins
	contract.Speedrun = false
	contract.Banker.SinkBoostPosition = SinkBoostFollowOrder
	contract.StartTime = time.Now()

	contract.NewFeature = 1
	contract.RegisteredNum = 0
	contract.CoopSize = coopSize
	contract.Name = contractID
	updateContractWithEggIncData(contract)

	contract.DynamicData = createDynamicTokenData()
	Contracts[ContractHash] = contract

	/*
		} else { //if !creatorOfContract(contract, userID) {
			contract.CreatorID = append(contract.CreatorID, userID) // starting userid
			contract.Location = append(contract.Location, loc)
		}*/

	// Find our Token emoji
	contract.TokenStr, _, _ = ei.GetBotEmoji("token")

	return contract, nil
}

// HandleContractSettingsReactions handles all the button reactions for a contract settings
func HandleContractSettingsReactions(s *discordgo.Session, i *discordgo.InteractionCreate) {
	redrawSignup := true
	// This is only coming from the caller of the contract

	// cs_#Name # cs_#ID # HASH
	reaction := strings.Split(i.MessageComponentData().CustomID, "#")
	cmd := strings.ToLower(reaction[1])
	contractHash := reaction[len(reaction)-1]
	refreshBoostListComponents := false

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	contract := Contracts[contractHash]
	if contract == nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content: "Unable to find this contract.",
				Flags:   discordgo.MessageFlagsEphemeral,
			})
	}

	data := i.MessageComponentData()

	if cmd == "style" {
		values := data.Values

		contract.Style &= ^(ContractFlagFastrun + ContractFlagBanker)
		switch values[0] {
		case "boostlist":
			contract.Style |= ContractFlagFastrun
		case "banker":
			contract.Style |= ContractFlagBanker
		}
	}

	if cmd == "features" {
		values := data.Values
		if len(values) == 0 {
			contract.Style &= ^ContractFlagDynamicTokens
			contract.Style &= ^ContractFlag6Tokens
			contract.Style &= ^ContractFlag8Tokens
		} else {
			switch values[0] {
			case "boost6":
				contract.Style &= ^ContractFlagDynamicTokens
				contract.Style &= ^ContractFlag8Tokens
				if contract.Style&ContractFlag6Tokens != 0 {
					contract.Style &= ^ContractFlag6Tokens
				} else {
					contract.Style |= ContractFlag6Tokens
				}
			case "boost8":
				contract.Style &= ^ContractFlagDynamicTokens
				contract.Style &= ^ContractFlag6Tokens
				if contract.Style&ContractFlag8Tokens != 0 {
					contract.Style &= ^ContractFlag8Tokens
				} else {
					contract.Style |= ContractFlag8Tokens
				}
			case "dynamic":
				contract.Style &= ^ContractFlag6Tokens
				contract.Style &= ^ContractFlag8Tokens
				if contract.Style&ContractFlagDynamicTokens != 0 {
					contract.Style &= ^ContractFlagDynamicTokens
				} else {
					contract.Style |= ContractFlagDynamicTokens
				}
			}
		}
	}

	if cmd == "crt" {
		contract.Style &= ^(ContractFlagCrt + ContractFlagSelfRuns)
		values := data.Values
		switch values[0] {
		case "no_crt":
			if contract.State == ContractStateSignup {
				contract.Style |= ContractFlagNone
				contract.Speedrun = false
			}
		case "crt":
			contract.Style |= ContractFlagCrt
			contract.Speedrun = true
			contract.SRData.Legs = contract.SRData.NoSelfRunLegs

		case "self_runs":
			contract.Style |= (ContractFlagCrt + ContractFlagSelfRuns)
			contract.Speedrun = true
			// Update the contract to change style
			contract.SRData.Legs = contract.SRData.SelfRunLegs
		}
	}

	if cmd == "order" {
		if contract.State != ContractStateSignup && data.Values[0] != "signup" {
			_, _ = s.FollowupMessageCreate(i.Interaction, true,
				&discordgo.WebhookParams{
					Content: "Once the contract has started, you may change to Sign-up Order to cancel the original order selection.",
					Flags:   discordgo.MessageFlagsEphemeral,
				})
			return
		}

		values := data.Values
		switch values[0] {
		case "signup":
			contract.BoostOrder = ContractOrderSignup
		case "reverse":
			contract.BoostOrder = ContractOrderReverse
		case "fair":
			contract.BoostOrder = ContractOrderFair
		case "random":
			contract.BoostOrder = ContractOrderRandom
		case "elr":
			contract.BoostOrder = ContractOrderELR
			for _, b := range contract.Boosters {
				// Refresh the user's artifact set
				contract.Boosters[b.UserID].ArtifactSet = getUserArtifacts(b.UserID, nil)
			}
		case "tval":
			contract.BoostOrder = ContractOrderTVal
		}
	}

	switch cmd {
	case "crtsink":
		sid := getInteractionUserID(i)
		alts := append([]string{sid}, contract.Boosters[sid].Alts...)
		altIdx := slices.Index(alts, contract.Banker.CrtSinkUserID)
		if altIdx != -1 {
			if altIdx != len(alts)-1 {
				sid = alts[altIdx+1]
			} else {
				sid = alts[altIdx] // Allow for the state to reset
			}
		}

		if contract.Banker.CrtSinkUserID == sid {
			contract.Banker.CrtSinkUserID = ""
		} else if userInContract(contract, sid) {
			contract.Banker.CrtSinkUserID = sid
		}
	case "boostsink":
		sid := getInteractionUserID(i)
		alts := append([]string{sid}, contract.Boosters[sid].Alts...)
		altIdx := slices.Index(alts, contract.Banker.BoostingSinkUserID)
		if altIdx != -1 {
			if altIdx != len(alts)-1 {
				sid = alts[altIdx+1]
			} else {
				sid = alts[altIdx] // Allow for the state to reset
			}
		}

		if contract.Banker.BoostingSinkUserID == sid {
			contract.Banker.BoostingSinkUserID = ""
		} else if userInContract(contract, sid) {
			contract.Banker.BoostingSinkUserID = sid
		}
	case "postsink":
		sid := getInteractionUserID(i)
		alts := append([]string{sid}, contract.Boosters[sid].Alts...)
		altIdx := slices.Index(alts, contract.Banker.PostSinkUserID)
		if altIdx != -1 {
			if altIdx != len(alts)-1 {
				sid = alts[altIdx+1]
			} else {
				sid = alts[altIdx] // Allow for the state to reset
			}
		}
		if contract.Banker.PostSinkUserID == sid {
			contract.Banker.PostSinkUserID = ""
		} else if userInContract(contract, sid) {
			contract.Banker.PostSinkUserID = sid
		}
		if contract.State == ContractStateCompleted || contract.State == ContractStateWaiting {
			contract.Banker.CurrentBanker = contract.Banker.PostSinkUserID
			refreshBoostListComponents = true
		}
	case "sinkorder":
		// toggle the sink order
		switch contract.Banker.SinkBoostPosition {
		case SinkBoostFirst:
			contract.Banker.SinkBoostPosition = SinkBoostLast
		case SinkBoostLast:
			contract.Banker.SinkBoostPosition = SinkBoostFollowOrder
		case SinkBoostFollowOrder:
			contract.Banker.SinkBoostPosition = SinkBoostFirst
		}
	}

	calculateTangoLegs(contract, true)

	for _, loc := range contract.Location {
		var components []discordgo.MessageComponent
		msgedit := discordgo.NewMessageEdit(loc.ChannelID, loc.ListMsgID)
		boostListComp := DrawBoostList(s, contract)
		components = append(components, boostListComp...)

		msgedit.Flags = discordgo.MessageFlagsIsComponentsV2
		if refreshBoostListComponents {
			comp := getContractReactionsComponents(contract)
			components = append(components, comp...)
		}
		msgedit.Components = &components

		msg, err := s.ChannelMessageEditComplex(msgedit)
		if err == nil {
			loc.ListMsgID = msg.ID
		}
		//if refreshBoostListComponents {
		//	addContractReactionsButtons(s, contract, loc.ChannelID, msg.ID)
		//}
		if redrawSignup {
			// Rebuild the signup message to disable the start button
			var components []discordgo.MessageComponent
			msgID := loc.ReactionID
			msg := discordgo.NewMessageEdit(loc.ChannelID, msgID)

			contentStr, comp := GetSignupComponents(contract.State != ContractStateSignup, contract) // True to get a disabled start button
			components = append(components, &discordgo.TextDisplay{
				Content: contentStr,
			})
			components = append(components, comp...)
			msg.Components = &components
			msg.Flags = discordgo.MessageFlagsIsComponentsV2
			_, _ = s.ChannelMessageEditComplex(msg)
		}
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{})

}

// HandleContractSettingsCommand will handle the /contract-settings command
func HandleContractSettingsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
		inThread := false
		ch, err := s.Channel(i.ChannelID)
		if err == nil && ch.IsThread() {
			inThread = true
		}
		str, comp := getSignupContractSettings(contract.Location[0].ChannelID, contract.ContractHash, inThread)
		_, _ = s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content:    str,
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: comp,
			},
		)
		return

	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Flags:   discordgo.MessageFlagsEphemeral,
			Content: str,
		})
}
