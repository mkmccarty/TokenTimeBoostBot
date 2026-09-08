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

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// GetEggStandardTime returns 9:00 AM Pacific Time (PT) for the given date, accounting for daylight savings.
func GetEggStandardTime(t time.Time) time.Time {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		loc = time.FixedZone("PST", -8*60*60) // Fallback if timezone data is missing
	}
	tInLoc := t.In(loc)
	return time.Date(tInLoc.Year(), tInLoc.Month(), tInLoc.Day(), 9, 0, 0, 0, loc)
}

// GetSlashContractCommand returns the slash command for creating a contract
func GetSlashContractCommand(cmd string) *dc.Command {
	command := guildOnlyCommand(cmd, "Create a contract boost list.")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:         "contract-id",
			Description:  "Contract ID",
			Required:     true,
			Autocomplete: true,
		},
		dc.StringOption{
			Name:         "coop-id",
			Description:  "Select or enter a new coop-id",
			Required:     true,
			Autocomplete: true,
		},
		dc.IntOption{
			Name:        "play-style",
			Description: "Contract Play Style, default is ACO Cooperative",
			Choices: []dc.Choice[int]{
				{Name: "🟦 Chill", Value: ContractPlaystyleChill},
				{Name: "🟩 ACO Cooperative", Value: ContractPlaystyleACOCooperative},
				{Name: "🟧 Fastrun", Value: ContractPlaystyleFastrun},
				{Name: "🟥 Leaderboard", Value: ContractPlaystyleLeaderboard},
			},
		},
		dc.StringOption{
			Name:         "boost-order",
			Description:  "Select the starting boost order",
			Autocomplete: true,
		},
		dc.StringOption{
			Name:        "progenitors",
			Description: "List of mentions to seed farmers for this contract.",
		},
		dc.StringOption{
			Name:        "start-offset",
			Description: "Start offset in hours relative to contract drop.",
		},
		dc.IntOption{
			Name:        "coop-size",
			Description: "Co-op Size. This will be pulled from EI Contract data if unset.",
		},
		dc.BoolOption{
			Name:        "make-thread",
			Description: "Create a thread for this contract? (default: true)",
		},
	}
	return &command
}

// HandleContractCommand will handle the /contract command
func HandleContractCommand(client dc.Client, e *dc.CommandEvent) {
	// Protection against DM use
	if e.GuildID() == "" {
		_ = e.Respond(dc.Message{
			Content:   "This command can only be run in a server.",
			Ephemeral: true,
		})
		return
	}

	// Initial response to the user
	_ = e.Defer(true)

	ch, err := client.Channel(e.ChannelID())
	if err != nil {
		_ = e.Followup(dc.Message{
			Content:   "No permissions to write to this channel.",
			Ephemeral: true,
		})
		return
	}

	var contractID = e.GuildID()
	var coopID = e.GuildID() // Default to the Guild ID
	var boostOrder = -1
	var coopSize = 0
	var ChannelID = e.ChannelID()
	var playStyle = ContractPlaystyleChill
	makeThread := true // Default is to always make a thread
	progenitors := []string{e.UserID()}
	plannedStartTime := time.Time{}

	if opt, ok := e.OptInt("play-style"); ok {
		playStyle = opt
	}
	if opt, ok := e.OptInt("coop-size"); ok {
		coopSize = opt
	}
	if opt, ok := e.OptString("boost-order"); ok {
		if val, err := strconv.Atoi(opt); err == nil {
			boostOrder = val
		}
	}
	if opt, ok := e.OptString("progenitors"); ok {
		re := regexp.MustCompile(`\d+`)
		userIDs := re.FindAllString(opt, -1)
		if len(userIDs) > 0 {
			var validProgenitors []string
			for _, userID := range userIDs {
				// Verify the user exists in the guild
				_, err := client.GuildMember(e.GuildID(), userID)
				if err == nil {
					validProgenitors = append(validProgenitors, userID)
				}
			}
			if len(validProgenitors) > 0 {
				progenitors = validProgenitors
			}
		}
	}
	if opt, ok := e.OptString("contract-id"); ok {
		contractID = strings.ReplaceAll(opt, " ", "")
	}
	if opt, ok := e.OptString("coop-id"); ok {
		coopID = strings.ReplaceAll(opt, " ", "")

		// if the coop-id contains the word "chill" at the start or end of the string, then we set the play style to chill
		coopLower := strings.ToLower(coopID)
		if strings.HasPrefix(coopLower, "chill") || strings.Contains(coopLower, "-chill") {
			playStyle = ContractPlaystyleChill
		}
	} else {
		var c, err = client.Channel(ChannelID)
		if err != nil && c != nil {
			coopID = c.Name
		}
	}

	validFrom := GetEggStandardTime(time.Now())

	// If the contract exists, use its actual drop date for the 9 AM PT anchor
	if contractInfo, ok := ei.EggIncContractsAll[contractID]; ok && !contractInfo.ValidFrom.IsZero() {
		if time.Since(contractInfo.ValidFrom) < 14*24*time.Hour {
			validFrom = GetEggStandardTime(contractInfo.ValidFrom)
		}
	}

	if opt, ok := e.OptString("start-offset"); ok {

		offset, err := strconv.ParseFloat(opt, 64)
		if err == nil {

			// Apply offset
			offsetDuration := time.Duration(offset * float64(time.Hour))
			resultTime := validFrom.Add(offsetDuration)

			// If the resulting time is in the past, push it forward day by day until it's in the future
			for resultTime.Before(time.Now()) {
				resultTime = resultTime.Add(24 * time.Hour)
			}

			startTime := resultTime.Unix()
			plannedStartTime = time.Unix(startTime, 0)
		}
	}

	if ch.IsThread {
		makeThread = false
	} else {
		// Is the bot allowed to create a thread?
		perms, err := client.UserChannelPermissions(config.DiscordAppID, e.ChannelID())
		if err == nil && perms.CreatePublicThreads() {
			if opt, ok := e.OptBool("make-thread"); ok {
				makeThread = opt
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
			_ = e.Followup(dc.Message{
				Content:   "Select a contract-id from the dropdown list.\nIf the contract-id list doesn't have your contract then supply a coop-size parameter.",
				Ephemeral: true,
			})
			return
		}
	}

	// Before we make a thread, make sure this isn't a duplicate contract
	if !isTBDCoopID(coopID) {
		ContractsMutex.RLock()
		for _, c := range Contracts {
			if c.ContractID == contractID && strings.EqualFold(c.CoopID, coopID) {
				ContractsMutex.RUnlock()
				_ = e.Followup(dc.Message{
					Content:   "A contract with this coop-id (" + c.CoopID + ") exists in " + c.Location[0].ChannelMention,
					Ephemeral: true,
				})
				return
			}
		}
		ContractsMutex.RUnlock()
	}

	contractInfo := ei.EggIncContractsAll[contractID]
	maxSize := contractInfo.MaxCoopSize
	if maxSize == 0 {
		maxSize = coopSize
	}
	if maxSize > 0 {
		// Trim the progenitor list to the max coop size
		if len(progenitors) > maxSize {
			progenitors = progenitors[:maxSize]
		}
		if !slices.Contains(progenitors, e.UserID()) && len(progenitors) < maxSize {
			progenitors = append([]string{e.UserID()}, progenitors...)
		}
	}

	// Create a new thread for this contract
	if makeThread {
		threadStyleIcons := []string{"", "🟦 ", "🟩 ", "🟧 ", "🟥 "}

		// Build the suffixes first to know how much space they consume
		var suffixBuilder strings.Builder
		if contractInfo.ID != "" {
			playStyleStr := fmt.Sprintf("%s ", contractPlaystyleNames[playStyle])
			if !contractInfo.Predicted {
				if len(progenitors) != contractInfo.MaxCoopSize {
					fmt.Fprintf(&suffixBuilder, "(%s%d/%d)", playStyleStr, len(progenitors), contractInfo.MaxCoopSize)
				} else {
					fmt.Fprint(&suffixBuilder, "(FULL)")
				}
			} else {
				fmt.Fprintf(&suffixBuilder, " (%s%d)", playStyleStr, len(progenitors))
			}
		}
		if !plannedStartTime.IsZero() {
			nyTime, err := time.LoadLocation("America/New_York")
			if err == nil {
				currentTime := plannedStartTime.In(nyTime)
				formattedTime := currentTime.Format("3:04pm MST")
				fmt.Fprint(&suffixBuilder, " "+formattedTime)
			}
		}
		suffixes := suffixBuilder.String()

		var builder strings.Builder
		if !contractInfo.Predicted {
			if isTBDCoopID(coopID) {
				collisionCount := 0
				ContractsMutex.RLock()
				for _, c := range Contracts {
					if c.ContractID == contractID && strings.EqualFold(c.CoopID, coopID) && c.State != ContractStateArchive {
						collisionCount++
					}
				}
				ContractsMutex.RUnlock()

				collisionStr := ""
				if collisionCount > 0 {
					collisionStr = fmt.Sprintf(" #%d", collisionCount+1)
				}

				icon := threadStyleIcons[playStyle]
				fixedLen := len(icon) + 1 + len(coopID) + len(collisionStr) + len(suffixes) + 2

				nameToUse := contractInfo.Name
				if fixedLen+len(nameToUse) > 100 {
					allowedNameLen := 100 - fixedLen
					if allowedNameLen > 3 {
						nameToUse = nameToUse[:allowedNameLen-3] + "..."
					} else if allowedNameLen > 0 {
						nameToUse = nameToUse[:allowedNameLen]
					} else {
						nameToUse = ""
					}
				}

				if collisionCount > 0 {
					fmt.Fprintf(&builder, "%s%s %s%s", icon, nameToUse, coopID, collisionStr)
				} else {
					fmt.Fprintf(&builder, "%s%s %s", icon, nameToUse, coopID)
				}
			} else {
				fmt.Fprintf(&builder, "%s%s", threadStyleIcons[playStyle], coopID)
			}
		} else {
			icon := threadStyleIcons[playStyle]
			fixedLen := len(icon) + 1 + len("Signup") + len(suffixes) + 1
			nameToUse := contractInfo.Name
			if fixedLen+len(nameToUse) > 100 {
				allowedNameLen := 100 - fixedLen
				if allowedNameLen > 3 {
					nameToUse = nameToUse[:allowedNameLen-3] + "..."
				} else if allowedNameLen > 0 {
					nameToUse = nameToUse[:allowedNameLen]
				} else {
					nameToUse = ""
				}
			}
			fmt.Fprintf(&builder, "%s%s %s", icon, nameToUse, "Signup")
		}

		builder.WriteString(suffixes)
		threadName := builder.String()
		if len(threadName) > 100 {
			threadName = threadName[:100]
		}

		thread, err := client.StartThread(ChannelID, threadName, 60*24)
		if err == nil {
			ChannelID = thread.ID
			_ = client.JoinThread(thread.ID)
		} else {
			log.Print(err)
		}
	}

	mutex.Lock()
	contract, err := CreateContract(client, contractID, coopID, playStyle, coopSize, boostOrder, e.GuildID(), ChannelID, progenitors, e.UserID(), plannedStartTime, validFrom)
	mutex.Unlock()

	if err != nil {
		if ferr := e.Followup(dc.Message{
			Content:   err.Error(),
			Ephemeral: true,
		}); ferr != nil {
			log.Print(ferr)
		}
		return
	}

	if len(contract.Location) == 1 {
		str, comp := getSignupContractSettings(contract.Location[0].ChannelID, contract.ContractHash, makeThread)

		if ChannelID != e.ChannelID() {
			str += "\nThis message can be moved into the contract thread via `/contract-settings` command in that thread."
		}
		// Take the str and make it a TextDisplay component and add it as the fist entry on the components
		var components []dc.LayoutComponent
		components = append(components, dc.TextDisplay{
			Content: str,
		})
		// Add the contract settings component
		components = append(components, comp...)

		err = e.Followup(dc.Message{
			Ephemeral:  true,
			Components: components,
		})
		if err != nil {
			log.Print(err)
		}
	} else {
		err = e.Followup(dc.Message{
			Content:   "This contract was initiated in <#" + contract.Location[0].ChannelID + ">. The coordinator will take care of the options, including `/boost-order`.",
			Ephemeral: true,
		})
		if err != nil {
			log.Print(err)
		}
	}

	var createMsg = DrawBoostList(contract)
	buttonComponents := getContractReactionsComponents(contract)
	if len(buttonComponents) > 0 {
		createMsg = append(createMsg, buttonComponents...)
	}
	msg, err := client.SendMessage(ChannelID, dc.Message{Components: createMsg})
	if err == nil {
		var components []dc.LayoutComponent
		SetListMessageID(contract, ChannelID, msg.ID)

		contentStr, comp := GetSignupComponents(contract)
		components = append(components, dc.TextDisplay{
			Content: contentStr,
		})
		components = append(components, comp...)

		reactionMsg, err := client.SendMessage(ChannelID, dc.Message{Components: components})

		if err != nil {
			log.Print(err)
		} else {
			SetReactionID(contract, msg.ChannelID, reactionMsg.ID)
			_ = client.PinMessage(msg.ChannelID, reactionMsg.ID)
		}
	} else {
		log.Print(err)
	}
}

// CreateContract creates a new contract or joins an existing contract if run from a different location
func CreateContract(client dc.Client, contractID string, coopID string, playStyle int, coopSize int, BoostOrder int, guildID string, channelID string, progenitors []string, userID string, plannedStartTime time.Time, validFrom time.Time) (*Contract, error) {
	// When creating contracts, we can make sure to clean up and archived ones
	// Just in case a contract was immediately recreated
	var archivedContracts []*Contract
	ContractsMutex.RLock()
	for _, c := range Contracts {
		if c.State == ContractStateArchive {
			if c.CalcOperations == 0 || time.Since(c.CalcOperationTime).Minutes() > 20 {
				archivedContracts = append(archivedContracts, c)
			}
		}
	}
	ContractsMutex.RUnlock()

	for _, archivedContract := range archivedContracts {
		FinishContract(client, archivedContract)
	}

	// Make sure this channel doesn't already have a contract
	existingContract := FindContract(channelID)
	if existingContract != nil {
		return nil, errors.New("this channel already has a contract named: " + existingContract.ContractID + "/" + existingContract.CoopID)
	}

	var contract *Contract
	// Does a coop already exist for this contract-id and coop-id
	if !isTBDCoopID(coopID) {
		ContractsMutex.RLock()
		for _, c := range Contracts {
			if c.ContractID == contractID && strings.EqualFold(c.CoopID, coopID) {
				// We have a coop, add this channel to the coop
				ContractsMutex.RUnlock()
				return nil, errors.New("a contract with this coop-id (" + c.CoopID + ") exists in " + c.Location[0].ChannelMention)
				//contract = c
			}
		}
		ContractsMutex.RUnlock()
	}

	// Lets find a ping role to use

	loc := new(LocationData)
	loc.GuildID = guildID
	loc.ChannelID = channelID
	var g, gerr = client.Guild(guildID)
	if gerr == nil {
		loc.GuildName = g.Name
	}

	var c, cerr = client.Channel(channelID)
	if cerr == nil {
		loc.ChannelMention = c.Mention()
	}
	loc.ListMsgID = ""
	loc.ReactionID = ""

	//if contract == nil {
	var ContractHash string

	// Try a number of random docker-style names first
	for attempt := 0; attempt < 64; attempt++ {
		cand := bottools.GetRandomName(0)
		ContractsMutex.RLock()
		isNil := Contracts[cand] == nil
		ContractsMutex.RUnlock()
		if isNil {
			ContractHash = cand
			break
		}
	}

	// Fallback: append a short random suffix to guarantee uniqueness
	if ContractHash == "" {
		base := bottools.GetRandomName(0)
		for {
			suffix := strconv.FormatUint(rand.Uint64(), 36)
			if len(suffix) > 6 {
				suffix = suffix[:6]
			}
			cand := fmt.Sprintf("%s-%s", base, suffix)
			ContractsMutex.RLock()
			isNil := Contracts[cand] == nil
			ContractsMutex.RUnlock()
			if isNil {
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
	contract.ValidFrom = validFrom
	//	contract.UseInteractionButtons = config.GetTestMode() // Feature under test
	err := getContractRole(client, guildID, contract)
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
	contract.ThresholdTokensX = 4
	contract.ThresholdTokensY = 5
	contract.ThresholdTokensA = 70

	//GlobalContracts[ContractHash] = append(GlobalContracts[ContractHash], loc)
	contract.Boosters = make(map[string]*Booster)
	contract.CRMessageIDs = make(map[string]string)
	contract.ContractID = contractID
	contract.CoopID = coopID
	contract.PlayStyle = playStyle
	if BoostOrder == -1 {
		contract.BoostOrder = ContractOrderSignup
		if playStyle == ContractPlaystyleLeaderboard {
			contract.BoostOrder = ContractOrderTEFuzzy
		}
	} else {
		contract.BoostOrder = BoostOrder
	}
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

	contract.StartTime = time.Now()

	contract.NewFeature = 1
	contract.RegisteredNum = 0
	contract.CoopSize = coopSize
	contract.Name = contractID
	updateContractWithEggIncData(client, contract)

	if contractInfo, ok := ei.EggIncContractsAll[contract.ContractID]; ok {
		bannerText := contract.Name
		if bannerText == "" {
			bannerText = contractInfo.Name
		}
		eggName := contract.EggName
		if eggName == "" {
			eggName = contractInfo.EggName
		}
		if bannerText != "" && eggName != "" && !contract.PredictionSignup {
			styleArray := []string{"", "c", "a", "f", "l"}
			style := ""
			if contract.PlayStyle >= 0 && contract.PlayStyle < len(styleArray) {
				style = styleArray[contract.PlayStyle]
			}
			guildID := ""
			if len(contract.Location) > 0 {
				guildID = contract.Location[0].GuildID
			}
			bottools.GenerateBanner(contract.ContractID, eggName, bannerText, userID, guildID, style)
		}
	}

	UpdateBannerURL(contract)

	// Long contracts default the sink to boosting last
	// Short contracts default the sink to boosting first
	contract.Banker.SinkBoostPosition = SinkBoostLast
	if contract.EstimatedDuration < 10*time.Hour {
		contract.Banker.SinkBoostPosition = SinkBoostFirst
	}

	contract.DynamicData = createDynamicTokenData(50)
	ContractsMutex.Lock()
	Contracts[ContractHash] = contract
	ContractsMutex.Unlock()

	// Apply potato-themed role override asynchronously once for the first configured user.
	potatoRoleScheduled := false
	schedulePotatoRoleOverride := func(candidateUserID string) {
		if potatoRoleScheduled || candidateUserID == "" {
			return
		}
		for _, loc := range contract.Location {
			if loc != nil && isPotatoPreferredUser(loc.GuildID, candidateUserID) {
				ensurePotatoTeamRoleForUserAsync(client, contract, candidateUserID)
				potatoRoleScheduled = true
				return
			}
		}
	}
	schedulePotatoRoleOverride(userID)
	for _, pid := range progenitors {
		schedulePotatoRoleOverride(pid)
	}

	// want to string ContractFlagCrt and ContractFlagSelfRun from Style
	contract.Style &^= (ContractFlagCrt + ContractFlagSelfRuns)

	// Override the contract style based on the play style, only for leaderboard play style
	if contract.PlayStyle == ContractPlaystyleLeaderboard {
		contract.Style = ContractFlagFastrun
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
		_, err = AddFarmerToContract(client, contract, guildID, channelID, pid, contract.BoostOrder, true, false)
		if err != nil {
			return nil, err
		}
	}

	return contract, nil
}

// HandleContractSettingsReactions handles all the button reactions for a contract settings
func HandleContractSettingsReactions(client dc.Client, e *dc.ComponentEvent) {
	redrawSignup := true
	redrawSettings := false

	// This is only coming from the caller of the contract

	// cs_#Name # cs_#ID # HASH
	reaction := strings.Split(e.CustomID(), "#")
	cmd := strings.ToLower(reaction[1])
	contractHash := reaction[len(reaction)-1]

	dataValues := e.Values()
	if cmd == "features" && slices.Contains(dataValues, "threshold") {
		// Only open threshold modal if threshold is the topmost boost token setting selected.
		// Higher options in featuresOptions: boost4, boost6, boost8, dynamic.
		hasHigherTokenOption := slices.Contains(dataValues, "boost4") ||
			slices.Contains(dataValues, "boost6") ||
			slices.Contains(dataValues, "boost8") ||
			slices.Contains(dataValues, "dynamic")

		if !hasHigherTokenOption {
			// If threshold was selected alongside other features like amqp, update AMQP flag before showing modal
			if contract := FindContractByHash(contractHash); contract != nil {
				if slices.Contains(dataValues, "amqp") {
					contract.Style |= ContractFlagAMQP
				} else {
					contract.Style &= ^ContractFlagAMQP
				}
			}
			SendThresholdModal(e, contractHash)
			return
		}
	}

	_ = e.DeferUpdate()

	contract := FindContractByHash(contractHash)
	if contract == nil {
		_ = e.Followup(dc.Message{
			Content:   "Unable to find this contract.",
			Ephemeral: true,
		})
		return
	}

	if cmd == "style" {
		values := dataValues

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
		values := dataValues

		// AMQP is independent of token quantity
		if slices.Contains(values, "amqp") {
			contract.Style |= ContractFlagAMQP
		} else {
			contract.Style &= ^ContractFlagAMQP
		}

		// Find token option if selected (mutually exclusive).
		// In the menu, the options are ordered: boost4, boost6, boost8, dynamic, threshold.
		// Within the settings allowing boost token selections, only accept the topmost value and clear the others.
		const tokenMask = ContractFlagDynamicTokens | ContractFlag6Tokens | ContractFlag8Tokens | ContractFlag4Tokens | ContractFlagThresholdTokens
		var selectedTokenFlag int64

		tokenOptions := []struct {
			name string
			flag int64
		}{
			{"boost4", ContractFlag4Tokens},
			{"boost6", ContractFlag6Tokens},
			{"boost8", ContractFlag8Tokens},
			{"dynamic", ContractFlagDynamicTokens},
		}

		for _, opt := range tokenOptions {
			if slices.Contains(values, opt.name) {
				selectedTokenFlag = opt.flag
				break
			}
		}

		// Clear all token flags first
		contract.Style &= ^tokenMask

		// Apply the selected token flag if any
		if selectedTokenFlag != 0 {
			contract.Style |= selectedTokenFlag
		}

		redrawSignup = true
		redrawSettings = true
	}

	if cmd == "order" {
		/*
			if contract.State != ContractStateSignup && dataValues[0] != "signup" {
				_, _ = s.FollowupMessageCreate(i.Interaction, true,
					&discordgo.WebhookParams{
						Content: "Once the contract has started, you may change to Sign-up Order to cancel the original order selection.",
						Flags:   discordgo.MessageFlagsEphemeral,
					})
				return
			}
		*/

		values := dataValues
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
		case "te", "fuzzyte":
			type userToRefresh struct {
				userID  string
				booster *Booster
			}
			if values[0] == "fuzzyte" {
				contract.BoostOrder = ContractOrderTEFuzzy
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
				updateContractFarmerTE(client, item.userID, item.booster, contract)
			}
		case "ihr", "fuzzyihr":
			type userToRefresh struct {
				userID  string
				booster *Booster
			}
			if values[0] == "fuzzyihr" {
				contract.BoostOrder = ContractOrderIHRFuzzy
			} else {
				contract.BoostOrder = ContractOrderIHR
			}
			usersToRefresh := make([]userToRefresh, 0, len(contract.Boosters))
			for userID, b := range contract.Boosters {
				// Recalculate IHR rate from DB for all boosters so pre-change values excluding deflector stones are updated
				rate, logStr := CalculateIHRRateFromDB(userID)
				b.IHRRate = rate
				b.IHRCalcLog = logStr

				if b.IHRRate == 0 {
					usersToRefresh = append(usersToRefresh, userToRefresh{userID: userID, booster: b})
				}
			}
			// Call updateContractFarmerTE for each collected user with 0 IHR
			for _, item := range usersToRefresh {
				updateContractFarmerTE(client, item.userID, item.booster, contract)
			}
		}
	}

	switch cmd {

	case "boostsink":
		sid := e.UserID()
		booster, ok := contract.Boosters[sid]
		if !ok || booster == nil {
			_ = e.Followup(dc.Message{
				Content:   "You're not in this contract, so you can't modify sink selections.",
				Ephemeral: true,
			})
			return
		}
		alts := []string{sid}
		alts = append(alts, booster.Alts...)
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
		} else if UserInContract(contract, sid) {
			contract.Banker.BoostingSinkUserID = sid
		}
		if contract.State == ContractStateBanker {
			contract.Banker.CurrentBanker = contract.Banker.BoostingSinkUserID
		}
	case "postsink":
		sid := e.UserID()
		booster, ok := contract.Boosters[sid]
		if !ok || booster == nil {
			_ = e.Followup(dc.Message{
				Content:   "You're not in this contract, so you can't modify sink selections.",
				Ephemeral: true,
			})
			return
		}
		alts := []string{sid}
		alts = append(alts, booster.Alts...)
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
		} else if UserInContract(contract, sid) {
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
		values := dataValues
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
		if contractInfo, ok := ei.EggIncContractsAll[contract.ContractID]; ok {
			bannerText := contract.Name
			if bannerText == "" {
				bannerText = contractInfo.Name
			}
			eggName := contract.EggName
			if eggName == "" {
				eggName = contractInfo.EggName
			}
			if bannerText != "" && eggName != "" && !contract.PredictionSignup {
				creator := ""
				if len(contract.CreatorID) > 0 {
					creator = contract.CreatorID[0]
				}
				styleArray := []string{"", "c", "a", "f", "l"}
				style := ""
				if contract.PlayStyle >= 0 && contract.PlayStyle < len(styleArray) {
					style = styleArray[contract.PlayStyle]
				}
				guildID := ""
				if len(contract.Location) > 0 {
					guildID = contract.Location[0].GuildID
				}
				bottools.GenerateBanner(contract.ContractID, eggName, bannerText, creator, guildID, style)
			}
		}

		// Need to rename the thread if it exists
		UpdateThreadName(client, contract)
		UpdateBannerURL(contract)
	}

	// With the changed settings values, we need to redraw the current Interaction message
	if redrawSettings {

		inThread := false
		ch, err := client.Channel(e.ChannelID())
		if err == nil && ch.IsThread {
			inThread = true
		}
		str, comp := getSignupContractSettings(contract.Location[0].ChannelID, contract.ContractHash, inThread)
		// Take the str and make it a TextDisplay component and add it as the fist entry on the components
		var components []dc.LayoutComponent
		components = append(components, dc.TextDisplay{
			Content: str,
		})
		// Add the contract settings component
		components = append(components, comp...)

		_ = e.EditFollowup(e.MessageID(), dc.Message{Components: components})

	}

	for _, loc := range contract.Location {
		var components []dc.LayoutComponent
		boostListComp := DrawBoostList(contract)
		components = append(components, boostListComp...)
		buttonComponents := getContractReactionsComponents(contract)
		if len(buttonComponents) > 0 {
			components = append(components, buttonComponents...)
		}

		msg, err := client.EditMessage(loc.ChannelID, loc.ListMsgID, dc.Message{Components: components})
		if err == nil {
			loc.ListMsgID = msg.ID
		} else {
			log.Print(err)
		}

		if redrawSignup {
			updateSignupReactionMessage(client, contract, loc)
		}
	}

	_ = e.Followup(dc.Message{})

}

// HandleContractSettingsCommand will handle the /contract-settings command
func HandleContractSettingsCommand(client dc.Client, e *dc.CommandEvent) {
	str := "Contract not found"
	_ = e.Defer(true)
	contract := FindContract(e.ChannelID())
	if contract != nil {
		inThread := false
		ch, err := client.Channel(e.ChannelID())
		if err == nil && ch.IsThread {
			inThread = true
		}
		str, comp := getSignupContractSettings(contract.Location[0].ChannelID, contract.ContractHash, inThread)
		// Take the str and make it a TextDisplay component and add it as the fist entry on the components
		var components []dc.LayoutComponent
		components = append(components, dc.TextDisplay{
			Content: str,
		})
		// Add the contract settings component
		components = append(components, comp...)
		err = e.Followup(dc.Message{Components: components})
		if err != nil {
			log.Println("Error sending contract settings:", err)
		}
		return

	}

	_ = e.Followup(dc.Message{
		Ephemeral: true,
		Content:   str,
	})
}

// PopulateThematicComplaintsForContractID updates active contracts for a contract ID with complaints.
func PopulateThematicComplaintsForContractID(contractID string, complaints []string) {
	mutex.Lock()
	defer mutex.Unlock()
	for _, contract := range Contracts {
		if contract.ContractID == contractID && len(contract.ThematicComplaints) == 0 && len(complaints) > 0 {
			contract.ThematicComplaints = append([]string(nil), complaints...)
			rand.Shuffle(len(contract.ThematicComplaints), func(i, j int) {
				contract.ThematicComplaints[i], contract.ThematicComplaints[j] = contract.ThematicComplaints[j], contract.ThematicComplaints[i]
			})
			saveData(contract.ContractHash)
		}
	}
}

// SendThresholdModal displays a modal to configure threshold tokens
func SendThresholdModal(e *dc.ComponentEvent, contractHash string) {
	contract := FindContractByHash(contractHash)
	if contract == nil {
		_ = e.Respond(dc.Message{
			Content:   "Unable to find this contract.",
			Ephemeral: true,
		})
		return
	}

	xVal := "4"
	yVal := "5"
	aVal := "70"
	if contract.ThresholdTokensX > 0 {
		xVal = strconv.Itoa(contract.ThresholdTokensX)
	}
	if contract.ThresholdTokensY > 0 {
		yVal = strconv.Itoa(contract.ThresholdTokensY)
	}
	if contract.ThresholdTokensA > 0 {
		aVal = strconv.Itoa(contract.ThresholdTokensA)
	}

	err := e.ShowModal(dc.Modal{
		CustomID: "m_threshold#" + contractHash,
		Title:    "Configure Threshold Boost Tokens",
		Inputs: []dc.TextInput{
			{
				CustomID:    "threshold-x",
				Label:       "Tokens Wanted if >= TE (X)",
				Style:       dc.TextInputStyleShort,
				Placeholder: "X tokens (default 4)",
				Value:       xVal,
				MaxLength:   2,
				Required:    true,
			},
			{
				CustomID:    "threshold-y",
				Label:       "Tokens Wanted if < TE (Y)",
				Style:       dc.TextInputStyleShort,
				Placeholder: "Y tokens (default 5)",
				Value:       yVal,
				MaxLength:   2,
				Required:    true,
			},
			{
				CustomID:    "threshold-a",
				Label:       "TE Threshold",
				Style:       dc.TextInputStyleShort,
				Placeholder: "TE threshold (default 70)",
				Value:       aVal,
				MaxLength:   3,
				Required:    true,
			},
		},
	})
	if err != nil {
		log.Println("Error sending threshold modal:", err)
	}
}

// HandleThresholdModalSubmit processes the submitted threshold tokens modal dialog
func HandleThresholdModalSubmit(client dc.Client, e *dc.ModalEvent) {
	parts := strings.Split(e.CustomID(), "#")
	contractHash := parts[1]

	contract := FindContractByHash(contractHash)
	if contract == nil {
		_ = e.Respond(dc.Message{
			Content:   "Contract not found.",
			Ephemeral: true,
		})
		return
	}

	x, errX := strconv.Atoi(strings.TrimSpace(e.TextValue("threshold-x")))
	y, errY := strconv.Atoi(strings.TrimSpace(e.TextValue("threshold-y")))
	a, errA := strconv.Atoi(strings.TrimSpace(e.TextValue("threshold-a")))

	if errX != nil || errY != nil || errA != nil || x < 0 || y < 0 || a < 0 {
		_ = e.Respond(dc.Message{
			Content:   "Invalid values entered. Please enter non-negative numbers.",
			Ephemeral: true,
		})
		return
	}

	contract.mutex.Lock()
	contract.ThresholdTokensX = x
	contract.ThresholdTokensY = y
	contract.ThresholdTokensA = a

	contract.Style &= ^ContractFlagDynamicTokens
	contract.Style &= ^ContractFlag6Tokens
	contract.Style &= ^ContractFlag8Tokens
	contract.Style &= ^ContractFlag4Tokens
	contract.Style |= ContractFlagThresholdTokens
	contract.mutex.Unlock()

	saveData(contract.ContractHash)

	_ = e.DeferUpdate()

	// Redraw/refresh settings page
	inThread := false
	ch, err := client.Channel(e.ChannelID())
	if err == nil && ch.IsThread {
		inThread = true
	}
	str, comp := getSignupContractSettings(contract.Location[0].ChannelID, contract.ContractHash, inThread)
	var components []dc.LayoutComponent
	components = append(components, dc.TextDisplay{
		Content: str,
	})
	components = append(components, comp...)

	_ = e.EditFollowup(e.MessageID(), dc.Message{Components: components})

	// Redraw/refresh booster list and signup reactions
	for _, loc := range contract.Location {
		var listComponents []dc.LayoutComponent
		boostListComp := DrawBoostList(contract)
		listComponents = append(listComponents, boostListComp...)
		buttonComponents := getContractReactionsComponents(contract)
		if len(buttonComponents) > 0 {
			listComponents = append(listComponents, buttonComponents...)
		}

		msg, err := client.EditMessage(loc.ChannelID, loc.ListMsgID, dc.Message{Components: listComponents})
		if err == nil {
			loc.ListMsgID = msg.ID
		} else {
			log.Print(err)
		}

		updateSignupReactionMessage(client, contract, loc)
	}
}
