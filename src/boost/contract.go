package boost

import (
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"

	"github.com/bwmarrin/discordgo"
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
				Name:        "play-style",
				Description: "Contract Play Style, default is ACO Cooperative",
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{
						Name:  "Chill",
						Value: ContractPlaystyleChill,
					},
					{
						Name:  "ACO Cooperative",
						Value: ContractPlaystyleACOCooperative,
					},
					{
						Name:  "Fastrun",
						Value: ContractPlaystyleFastrun,
					},
					{
						Name:  "Leaderboard",
						Value: ContractPlaystyleLeaderboard,
					},
				},
				Required: false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "progenitors",
				Description: "List of mentions to seed farmers for this contract.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "start-offset",
				Description: "Start offset in hours relative to contract drop.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "coop-size",
				Description: "Co-op Size. This will be pulled from EI Contract data if unset.",
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
	var playStyle = ContractPlaystyleChill
	makeThread := true // Default is to always make a thread
	progenitors := []string{i.Member.User.ID}
	plannedStartTime := time.Time{}

	optionMap := bottools.GetCommandOptionsMap(i)

	if opt, ok := optionMap["play-style"]; ok {
		playStyle = int(opt.IntValue())
	}
	if opt, ok := optionMap["coop-size"]; ok {
		coopSize = int(opt.IntValue())
	}
	if opt, ok := optionMap["boost-order"]; ok {
		boostOrder = int(opt.IntValue())
	}
	if opt, ok := optionMap["progenitors"]; ok {
		farmerList := opt.StringValue()
		re := regexp.MustCompile(`\d+`)
		userIDs := re.FindAllString(farmerList, -1)
		if len(userIDs) > 0 {
			var validProgenitors []string
			for _, userID := range userIDs {
				// Verify the user exists in the guild
				_, err := s.GuildMember(i.GuildID, userID)
				if err == nil {
					validProgenitors = append(validProgenitors, userID)
				}
			}
			if len(validProgenitors) > 0 {
				progenitors = validProgenitors
			}
		}
	}
	if opt, ok := optionMap["contract-id"]; ok {
		contractID = opt.StringValue()
		contractID = strings.ReplaceAll(contractID, " ", "")
	}
	if opt, ok := optionMap["coop-id"]; ok {
		coopID = opt.StringValue()
		coopID = strings.ReplaceAll(coopID, " ", "")

		// if the coop-id contains the word "chill" at the start or end of the string, then we set the play style to chill
		coopLower := strings.ToLower(coopID)
		if strings.HasPrefix(coopLower, "chill") || strings.Contains(coopLower, "-chill") {
			playStyle = ContractPlaystyleChill
		}
	} else {
		var c, err = s.Channel(ChannelID)
		if err != nil {
			coopID = c.Name
		}
	}
	if opt, ok := optionMap["start-offset"]; ok {

		offsetStr := opt.StringValue()
		offset, err := strconv.ParseFloat(offsetStr, 64)
		if err == nil {
			c := ei.EggIncContractsAll[contractID]
			if c.ValidFrom.IsZero() {
				// Get the ValidFrom time from the last contract of ei.EggIncContracts
				c2 := ei.EggIncContracts[len(ei.EggIncContracts)-1]
				c.ValidFrom = c2.ValidFrom
			}
			// Calculate time as 9:00 AM + offset hours using today's date
			now := time.Now()
			baseTime := c.ValidFrom

			// Create today's version of the base time (same hour/minute, but today's date)
			todayBaseTime := time.Date(now.Year(), now.Month(), now.Day(),
				baseTime.Hour(), baseTime.Minute(), baseTime.Second(),
				baseTime.Nanosecond(), now.Location())

			// Apply offset
			offsetDuration := time.Duration(offset * float64(time.Hour))
			resultTime := todayBaseTime.Add(offsetDuration)

			// If the resulting time is in the past, add 24 hours to make it tomorrow
			if resultTime.Before(now) {
				resultTime = resultTime.Add(24 * time.Hour)
			}

			startTime := resultTime.Unix()
			plannedStartTime = time.Unix(startTime, 0)
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

	contractInfo := ei.EggIncContractsAll[contractID]
	if contractInfo.ID != "" {
		// Trim the progenitor list to the max coop size
		if len(progenitors) > contractInfo.MaxCoopSize {
			progenitors = progenitors[:contractInfo.MaxCoopSize]
		}
		if !slices.Contains(progenitors, getInteractionUserID(i)) && len(progenitors) < contractInfo.MaxCoopSize {
			progenitors = append([]string{getInteractionUserID(i)}, progenitors...)
		}
	}

	// Create a new thread for this contract
	if makeThread {
		threadStyleIcons := []string{"", "🟦 ", "🟩 ", "🟧 ", "🟥 "}
		// Default to 1 day timeout
		var builder strings.Builder
		fmt.Fprintf(&builder, "%s %s", threadStyleIcons[playStyle], coopID)
		if contractInfo.ID != "" {
			playStyleStr := fmt.Sprintf("%s ", contractPlaystyleNames[playStyle])
			fmt.Fprintf(&builder, " (%s%d/%d)", playStyleStr, len(progenitors), contractInfo.MaxCoopSize)
		}
		if !plannedStartTime.IsZero() {
			nyTime, err := time.LoadLocation("America/New_York")
			if err == nil {
				currentTime := plannedStartTime.In(nyTime)
				formattedTime := currentTime.Format("3:04pm MST")
				fmt.Fprint(&builder, " "+formattedTime)
			}
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
	contract, err := CreateContract(s, contractID, coopID, playStyle, coopSize, boostOrder, i.GuildID, ChannelID, progenitors, getInteractionUserID(i), plannedStartTime)
	mutex.Unlock()

	if err != nil {
		_, err = s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content:    err.Error(),
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{},
			},
		)
		if err != nil {
			log.Print(err)
		}
		return
	}

	if len(contract.Location) == 1 {
		str, comp := getSignupContractSettings(contract.Location[0].ChannelID, contract.ContractHash, makeThread)

		if ChannelID != i.ChannelID {
			str += "\nThis message can be moved into the contract thread via `/contract-settings` command in that thread."
		}
		// Take the str and make it a TextDisplay component and add it as the fist entry on the components
		var components []discordgo.MessageComponent
		components = append(components, &discordgo.TextDisplay{
			Content: str,
		})
		// Add the contract settings component
		components = append(components, comp...)

		_, err = s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				//Content:    str,
				Flags:      discordgo.MessageFlagsEphemeral | discordgo.MessageFlagsIsComponentsV2,
				Components: components,
			},
		)
		if err != nil {
			log.Print(err)
		}
	} else {
		_, err = s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content: "This contract was initiated in <#" + contract.Location[0].ChannelID + ">. The coordinator will take care of the options.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		)
		if err != nil {
			log.Print(err)
		}
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

		contentStr, comp := GetSignupComponents(contract)
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
	} else {
		log.Print(err)
	}
}

// CreateContract creates a new contract or joins an existing contract if run from a different location
func CreateContract(s *discordgo.Session, contractID string, coopID string, playStyle int, coopSize int, BoostOrder int, guildID string, channelID string, progenitors []string, userID string, plannedStartTime time.Time) (*Contract, error) {
	// When creating contracts, we can make sure to clean up and archived ones
	// Just in case a contract was immediately recreated
	for _, c := range Contracts {
		if c.State == ContractStateArchive {
			if c.CalcOperations == 0 || time.Since(c.CalcOperationTime).Minutes() > 20 {
				FinishContract(s, c)
			}
		}
	}

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

	// Lets find a ping role to use

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
	}
	loc.ListMsgID = ""
	loc.ReactionID = ""

	//if contract == nil {
	var ContractHash string

	// Try a number of random docker-style names first
	for attempt := 0; attempt < 64; attempt++ {
		cand := namesgenerator.GetRandomName(0)
		if Contracts[cand] == nil {
			ContractHash = cand
			break
		}
	}

	// Fallback: append a short random suffix to guarantee uniqueness
	if ContractHash == "" {
		base := namesgenerator.GetRandomName(0)
		for {
			suffix := strconv.FormatUint(rand.Uint64(), 36)
			if len(suffix) > 6 {
				suffix = suffix[:6]
			}
			cand := fmt.Sprintf("%s-%s", base, suffix)
			if Contracts[cand] == nil {
				ContractHash = cand
				break
			}
		}
	}

	// We don't have this contract on this channel, it could exist in another channel
	contract = new(Contract)
	contract.Location = append(contract.Location, loc)
	contract.ContractHash = ContractHash
	contract.ContractID = contractID
	contract.CoopID = coopID

	contract.PlannedStartTime = plannedStartTime
	//	contract.UseInteractionButtons = config.GetTestMode() // Feature under test
	err := getContractRole(s, guildID, contract)
	for _, loc := range contract.Location {
		if loc.GuildID == guildID {
			if err == nil {
				loc.RoleMention = loc.GuildContractRole.Mention()
			} else {
				loc.RoleMention = "@here"
			}
		}
	}

	contract.Style = ContractStyleFastrun

	//GlobalContracts[ContractHash] = append(GlobalContracts[ContractHash], loc)
	contract.Boosters = make(map[string]*Booster)
	contract.CRMessageIDs = make(map[string]string)
	contract.ContractID = contractID
	contract.CoopID = coopID
	contract.PlayStyle = playStyle
	contract.BoostOrder = BoostOrder
	contract.BoostVoting = 0
	contract.OrderRevision = 0

	changeContractState(contract, ContractStateSignup)
	// When the calling userID isn't in the progenitors list, make the first a coordinator
	if !slices.Contains(progenitors, userID) {
		contract.CreatorID = append(contract.CreatorID, progenitors[0])
	}
	// Add starting user uniquely
	if !slices.Contains(contract.CreatorID, userID) {
		contract.CreatorID = append(contract.CreatorID, userID)
	}
	// Add global admins uniquely
	for _, admin := range config.AdminUsers {
		if !slices.Contains(contract.CreatorID, admin) {
			contract.CreatorID = append(contract.CreatorID, admin)
		}
	}
	// Add guild-specific admins uniquely, if any
	if config.GuildAdminUsers != nil {
		if guildAdmins, ok := config.GuildAdminUsers[guildID]; ok {
			for _, admin := range guildAdmins {
				if !slices.Contains(contract.CreatorID, admin) {
					contract.CreatorID = append(contract.CreatorID, admin)
				}
			}
		}
	}

	contract.StartTime = time.Now()

	contract.NewFeature = 1
	contract.RegisteredNum = 0
	contract.CoopSize = coopSize
	contract.Name = contractID
	updateContractWithEggIncData(contract)

	// Long contracts default the sink to boosting last
	// Short contracts default the sink to boosting first
	contract.Banker.SinkBoostPosition = SinkBoostLast
	if contract.EstimatedDuration < 10*time.Hour {
		contract.Banker.SinkBoostPosition = SinkBoostFirst
	}

	contract.DynamicData = createDynamicTokenData(50)
	Contracts[ContractHash] = contract

	// want to string ContractFlagCrt and ContractFlagSelfRun from Style
	contract.Style &^= (ContractFlagCrt + ContractFlagSelfRuns)

	// Override the contract style based on the play style, only for leaderboard play style
	if contract.PlayStyle == ContractPlaystyleLeaderboard {
		contract.Style = ContractFlagBanker
		contract.BoostOrder = ContractOrderTVal
	}
	/*
		} else { //if !creatorOfContract(contract, userID) {
			contract.CreatorID = append(contract.CreatorID, userID) // starting userid
			contract.Location = append(contract.Location, loc)
		}*/

	// Find our Token emoji
	contract.TokenStr, _, _ = ei.GetBotEmoji("token")

	// Add users into the contract
	for _, pid := range progenitors {
		_, err = AddFarmerToContract(s, contract, guildID, channelID, pid, contract.BoostOrder, true)
		if err != nil {
			return nil, err
		}
	}

	return contract, nil
}

// HandleContractSettingsReactions handles all the button reactions for a contract settings
func HandleContractSettingsReactions(s *discordgo.Session, i *discordgo.InteractionCreate) {
	redrawSignup := true
	redrawSettings := false

	// This is only coming from the caller of the contract

	// cs_#Name # cs_#ID # HASH
	reaction := strings.Split(i.MessageComponentData().CustomID, "#")
	cmd := strings.ToLower(reaction[1])
	contractHash := reaction[len(reaction)-1]

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	contract := FindContractByHash(contractHash)
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
		redrawSignup = true
		redrawSettings = true
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

	if cmd == "order" {
		/*
			if contract.State != ContractStateSignup && data.Values[0] != "signup" {
				_, _ = s.FollowupMessageCreate(i.Interaction, true,
					&discordgo.WebhookParams{
						Content: "Once the contract has started, you may change to Sign-up Order to cancel the original order selection.",
						Flags:   discordgo.MessageFlagsEphemeral,
					})
				return
			}
		*/

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
		case "ask":
			contract.BoostOrder = ContractOrderTokenAsk
		case "te", "teplus":
			type userToRefresh struct {
				userID  string
				booster *Booster
			}
			if values[0] == "teplus" {
				contract.BoostOrder = ContractOrderTEplus
			} else {
				contract.BoostOrder = ContractOrderTE
			}
			// Refresh the user's egg inc data, if they have 0 TE count
			// Collect userIDs and boosters with TECount == 0
			usersToRefresh := make([]userToRefresh, 0, len(contract.Boosters))
			for userID, b := range contract.Boosters {
				if b.TECount == 0 {
					usersToRefresh = append(usersToRefresh, userToRefresh{userID: userID, booster: b})
				}
			}
			// Call updateContractFarmerTE for each collected user
			for _, item := range usersToRefresh {
				updateContractFarmerTE(s, item.userID, item.booster, contract)
			}
		}
	}

	switch cmd {

	case "boostsink":
		sid := getInteractionUserID(i)
		alts := append([]string{sid}, contract.Boosters[sid].Alts...)
		altIdx := slices.Index(alts, contract.Banker.BoostingSinkUserID)
		if altIdx != -1 {
			if altIdx != len(alts)-1 {
				sid = alts[altIdx+1]
			} else {
				sid = alts[0] // Allow for the state to reset
			}
		}

		if contract.Banker.BoostingSinkUserID == sid {
			contract.Banker.BoostingSinkUserID = ""
		} else if userInContract(contract, sid) {
			contract.Banker.BoostingSinkUserID = sid
		}
		if contract.State == ContractStateBanker {
			contract.Banker.CurrentBanker = contract.Banker.BoostingSinkUserID
		}
	case "postsink":
		sid := getInteractionUserID(i)
		alts := append([]string{sid}, contract.Boosters[sid].Alts...)
		altIdx := slices.Index(alts, contract.Banker.PostSinkUserID)
		if altIdx != -1 {
			if altIdx != len(alts)-1 {
				sid = alts[altIdx+1]
			} else {
				sid = alts[0] // Allow for the state to reset
			}
		}
		if contract.Banker.PostSinkUserID == sid {
			contract.Banker.PostSinkUserID = ""
		} else if userInContract(contract, sid) {
			contract.Banker.PostSinkUserID = sid
		}
		if contract.State == ContractStateCompleted || contract.State == ContractStateWaiting {
			contract.Banker.CurrentBanker = contract.Banker.PostSinkUserID
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

	originalPlayStyle := contract.PlayStyle
	// Handle the play style flair
	if cmd == "play" {
		values := data.Values
		switch values[0] {
		case "chill":
			contract.PlayStyle = ContractPlaystyleChill
		case "aco":
			contract.PlayStyle = ContractPlaystyleACOCooperative
		case "fastrun":
			contract.PlayStyle = ContractPlaystyleFastrun
		case "leaderboard":
			contract.PlayStyle = ContractPlaystyleLeaderboard
		default:
			contract.PlayStyle = ContractPlaystyleUnset
		}
	}

	// If the play style changed to leaderboard, then change to use Banker style
	if contract.PlayStyle == ContractPlaystyleLeaderboard && originalPlayStyle != ContractPlaystyleLeaderboard {
		contract.Style &= ^ContractFlagFastrun
		contract.Style |= ContractFlagBanker
		contract.BoostOrder = ContractOrderTVal
		redrawSignup = true
		redrawSettings = true
	} else if originalPlayStyle == ContractPlaystyleLeaderboard && contract.PlayStyle != ContractPlaystyleLeaderboard {
		// If the play style changed from leaderboard to something else, then change to use Boost list style
		contract.Style &= ^ContractFlagBanker
		contract.Style |= ContractFlagFastrun
		redrawSignup = true
		redrawSettings = true
	}

	if originalPlayStyle != contract.PlayStyle {
		// Need to rename the thread if it exists
		UpdateThreadName(s, contract)
	}

	// With the changed settings values, we need to redraw the current Interaction message
	if redrawSettings {

		inThread := false
		ch, err := s.Channel(i.ChannelID)
		if err == nil && ch.IsThread() {
			inThread = true
		}
		str, comp := getSignupContractSettings(contract.Location[0].ChannelID, contract.ContractHash, inThread)
		// Take the str and make it a TextDisplay component and add it as the fist entry on the components
		var components []discordgo.MessageComponent
		components = append(components, &discordgo.TextDisplay{
			Content: str,
		})
		// Add the contract settings component
		components = append(components, comp...)

		edit := discordgo.WebhookEdit{
			Components: &components,
		}

		_, _ = s.FollowupMessageEdit(i.Interaction, i.Message.ID, &edit)

	}

	for _, loc := range contract.Location {
		var components []discordgo.MessageComponent
		msgedit := discordgo.NewMessageEdit(loc.ChannelID, loc.ListMsgID)
		msgedit.Flags = discordgo.MessageFlagsIsComponentsV2
		boostListComp := DrawBoostList(s, contract)
		components = append(components, boostListComp...)
		buttonComponents := getContractReactionsComponents(contract)
		if len(buttonComponents) > 0 {
			components = append(components, buttonComponents...)
		}
		msgedit.Components = &components

		msg, err := s.ChannelMessageEditComplex(msgedit)
		if err == nil {
			loc.ListMsgID = msg.ID
		} else {
			log.Print(err)
		}

		if redrawSignup {
			updateSignupReactionMessage(s, contract, loc)
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
		// Take the str and make it a TextDisplay component and add it as the fist entry on the components
		var components []discordgo.MessageComponent
		components = append(components, &discordgo.TextDisplay{
			Content: str,
		})
		// Add the contract settings component
		components = append(components, comp...)
		_, err = s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Flags:      discordgo.MessageFlagsIsComponentsV2,
				Components: components,
			},
		)
		if err != nil {
			log.Println("Error sending contract settings:", err)
		}
		return

	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Flags:   discordgo.MessageFlagsEphemeral,
			Content: str,
		})
}
