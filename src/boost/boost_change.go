package boost

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

var integerOneMinValue float64 = 1.0

// GetSlashChangeOneBoosterCommand adjust aspects of a running contract
func GetSlashChangeOneBoosterCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Move booster to a new position. If current booster, will assign new booster",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "booster-name",
				Description: "Booster to move. Use an @mention or guest farmer name",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "new-position",
				Description: "Position to move the booster to",
				Required:    true,
				MinValue:    &integerOneMinValue,
			},
		},
	}
}

// GetSlashChangePlannedStartCommand adjust aspects of a running contract
func GetSlashChangePlannedStartCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name: cmd,
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
		},
		Description: "Change the planned start time of the contract",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "offset",
				Description: "Relative offset",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "relative-time",
						Description: "Relative time offset from 9:00 AM. Example: +2.5 or -1.5",
						Required:    true,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "timestamp",
				Description: "Discord Timestamp",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "start-time",
						Description: "Discord Timestamp format. Example: <t:1716822000:f>",
						Required:    true,
					},
				},
			},
		},
	}
}

// GetSlasLinkAlternateCommand allows a player to associate an alt
func GetSlasLinkAlternateCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Add an alternate persona for this contract.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "farmer-name",
				Description:  "Name of your alternate persona. This guest needs to be in the contract.",
				Required:     true,
				Autocomplete: true,
			},
		},
	}
}

func extractUserID(s *discordgo.Session, boosterName string) (string, error) {
	if userID, isMention := parseMentionUserID(boosterName); isMention {
		u, err := s.User(userID)
		if err != nil {
			return "", err
		}
		return u.ID, nil
	}
	return normalizeUserIDInput(boosterName), nil
}

// HandleChangeOneBoosterCommand will handle the /change-one-booster command
func HandleChangeOneBoosterCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Protection against DM use as we need the channel ID to find the contract
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

	var str = ""
	optionMap := bottools.GetCommandOptionsMap(i)

	var err error
	contract := FindContract(i.ChannelID)
	if contract == nil {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    errorNoContract,
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
		return
	}

	position := 0
	boosterName := ""
	newBooster := ""

	if opt, ok := optionMap["new-position"]; ok {
		position = int(opt.IntValue())
		if position > len(contract.Order) {
			str = "Invalid position, must be between 1 and " + strconv.Itoa(len(contract.Order))
		}
	}

	if opt, ok := optionMap["booster-name"]; ok {
		// String in the form of mention
		boosterName = strings.TrimSpace(opt.StringValue())
		boosterName, err = extractUserID(s, boosterName)
		if err != nil {
			str = err.Error()
		}

		// Is this booster in the contract?
		if _, ok := contract.Boosters[boosterName]; !ok {
			str = "This farmer is not in the contract"
		} else {
			// If this booster has alread boosted then we can't move them
			if contract.Boosters[boosterName].BoostState == BoostStateBoosted {
				str = "This farmer has already boosted, no need to move them."
			} else if boosterName == contract.Order[contract.BoostPosition] {
				// If this is current booster, we need to reassign this to the next booster
				newBoosterIndex := findNextBoosterAfterUser(contract, boosterName)
				if newBoosterIndex != -1 {
					newBooster = contract.Order[newBoosterIndex]
				}
			} else {
				// Is the new position the current booster?
				if contract.Order[position-1] == contract.Order[contract.BoostPosition] {
					newBooster = boosterName
				}
			}
		}
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing...",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	// Empty string means we are good to go
	if str == "" {

		err := MoveBooster(s, i.GuildID, i.ChannelID, i.Member.User.ID, boosterName, position, newBooster == "")
		if err != nil {
			str += err.Error()
		} else {
			str += fmt.Sprintf("Moved %s to position %d.", contract.Boosters[boosterName].Mention, position)

			if newBooster != "" {
				err := ChangeCurrentBooster(s, i.GuildID, i.ChannelID, i.Member.User.ID, newBooster, true)
				if err != nil {
					str += " " + strings.ToUpper(string(err.Error()[0])) + err.Error()[1:]
				} else {
					str += fmt.Sprintf(" Current booster changed to %s.", contract.Boosters[newBooster].Mention)
				}
			}
		}
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Content: str},
	)

}

// HandleChangePlannedStartCommand will handle the /change--planned-start command
func HandleChangePlannedStartCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Protection against DM use as we need the channel ID to find the contract
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

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing...",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	var str = ""

	contract := FindContract(i.ChannelID)
	if contract == nil {
		str = errorNoContract
	} else {
		if !creatorOfContract(s, contract, i.Member.User.ID) {
			str = "only the contract creator can change the contract"
		}
	}

	// No error string means we are good to go
	if str == "" {
		// Default to @here when there is no parameter

		optionMap := bottools.GetCommandOptionsMap(i)

		if opt, ok := optionMap["offset-relative-time"]; ok {
			var startTime int64
			var err error
			offsetStr := opt.StringValue()
			offset, err := strconv.ParseFloat(offsetStr, 64)
			if err != nil {
				str = "Invalid offset format. Use a number like +2.5 or -1.5"
			} else {
				c := ei.EggIncContractsAll[contract.ContractID]
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

				startTime = resultTime.Unix()

				contract.PlannedStartTime = time.Unix(startTime, 0)
				str = "Planned start time changed to " + "<t:" + strconv.FormatInt(startTime, 10) + ":f>"
				refreshBoostListMessage(s, contract, false)
			}
		}

		if opt, ok := optionMap["timestamp-start-time"]; ok {
			var startTime int64
			var err error
			startTimeStr := opt.StringValue()

			// Split string by colons to get the timestamp
			startTimeArry := strings.Split(startTimeStr, ":")
			if len(startTimeArry) == 1 {
				startTime, err = strconv.ParseInt(startTimeArry[0], 10, 64)
			} else {
				startTime, err = strconv.ParseInt(startTimeArry[1], 10, 64)
			}

			if err != nil {
				str = "Invalid start time format. Use timestamps from [Discord Timestamp](https://discordtimestamp.com)"
			} else {
				contract.PlannedStartTime = time.Unix(startTime, 0)
				if startTime == 0 {
					str = "Planned start time cleared"
					refreshBoostListMessage(s, contract, false)
				} else if contract.PlannedStartTime.After(time.Now()) && contract.PlannedStartTime.Before(time.Now().AddDate(0, 0, 7)) {
					str = "Planned start time changed to " + "<t:" + strconv.FormatInt(startTime, 10) + ":f>"
					refreshBoostListMessage(s, contract, false)
				} else {
					str = "Planned start time must be within the next 7 days. Use timestamps from [Discord Timestamp](https://discordtimestamp.com)"
					contract.PlannedStartTime = time.Unix(0, 0)
				}
			}
		}
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Content: str},
	)
}

// removeDuplicates takes a slice as an argument and returns the array with all duplicate elements removed.
func removeDuplicates(s []string) []string {
	var result []string
	for i := range s {
		if !slices.Contains(result, s[i]) {
			result = append(result, s[i])
		}
	}
	return result
}

// ChangeContractIDs will change the contractID and/or coopID
func ChangeContractIDs(s *discordgo.Session, guildID string, channelID string, userID string, contractID string, coopID string, coordinatorID string) error {
	var contract = FindContract(channelID)
	if contract == nil {
		return errors.New(errorNoContract)
	}

	// return an error if the userID isn't the contract creator
	if !creatorOfContract(s, contract, userID) {
		return errors.New("only the contract creator can change the contract")
	}

	log.Println("ChangeContractIDs", "ContractID: ", contractID, "CoopID: ", coopID, "GuildID: ", guildID, "ChannelID: ", channelID, "UserID: ", userID, "Order: ", "")

	if contractID != "" {
		contract.ContractID = contractID
		updateContractWithEggIncData(contract)
		contract.EggEmoji = FindEggEmoji(contract.EggName)

		// Rename the contract role to match the new contract
		renameContractRole(s, contract)

		refreshBoostListMessage(s, contract, false)
	}
	if coopID != "" {
		contract.CoopID = coopID
	}
	if coordinatorID != "" {
		if slices.Index(contract.Order, coordinatorID) != -1 {
			contract.CreatorID[0] = coordinatorID
		} else {
			return errors.New("the selected coordinator needs to be in the contract")
		}
	}
	return nil
}

// renameContractRole renames the contract role to match the new contract ID
func renameContractRole(s *discordgo.Session, contract *Contract) {
	// Get the new thematic role name for the updated contract
	newRoleName := getContractRoleName(contract.ContractID)

	// Rename the role in each guild where the contract exists
	for _, loc := range contract.Location {
		if loc.GuildContractRole.ID == "" {
			continue
		}

		_, err := s.GuildRoleEdit(loc.GuildID, loc.GuildContractRole.ID, &discordgo.RoleParams{
			Name: newRoleName,
		})

		if err != nil {
			log.Println("Error renaming contract role:", err)
		} else {
			log.Println("Successfully renamed contract role to:", newRoleName)
		}
	}
}

// ChangeCurrentBooster will change the current booster to the specified userID
func ChangeCurrentBooster(s *discordgo.Session, guildID string, channelID string, userID string, newBooster string, redraw bool) error {
	var contract = FindContract(channelID)
	if contract == nil {
		return errors.New(errorNoContract)
	}

	// return an error if the contract is in the signup state
	if contract.State == ContractStateSignup {
		return errors.New(errorContractNotStarted)
	}

	// return an error if the userID isn't the contract creator
	if !creatorOfContract(s, contract, userID) {
		return errors.New("only the contract creator can change the contract")
	}

	log.Println("ChangeCurrentBooster", "GuildID: ", guildID, "ChannelID: ", channelID, "UserID: ", userID, "NewBooster: ", newBooster)

	newBoosterUserID := normalizeUserIDInput(newBooster)

	newBoosterIndex := slices.Index(contract.Order, newBoosterUserID)
	if newBoosterIndex == -1 {
		return errors.New("this booster not in contract")
	}

	switch contract.Boosters[newBoosterUserID].BoostState {
	case BoostStateUnboosted:
		// Clear current booster status
		currentBooster := contract.Order[contract.BoostPosition]
		if contract.Boosters[currentBooster].BoostState == BoostStateTokenTime {
			contract.Boosters[currentBooster].BoostState = BoostStateUnboosted
		}
		contract.Boosters[newBoosterUserID].BoostState = BoostStateTokenTime
		contract.Boosters[newBoosterUserID].StartTime = time.Now()
		contract.BoostPosition = newBoosterIndex

		// Make sure there's only a single booster
		for _, element := range contract.Order {
			if element != newBoosterUserID && contract.Boosters[element].BoostState == BoostStateTokenTime {
				contract.Boosters[element].BoostState = BoostStateUnboosted
			}
		}
	case BoostStateTokenTime:
		return errors.New("this booster is already currently receiving tokens")
	case BoostStateBoosted:
		return errors.New("this booster already boosted")
	}

	// Clear current booster boost state
	if redraw {
		sendNextNotification(s, contract, true)
	}
	return nil
}

// ChangeBoostOrder will change the order of the boosters in the contract
func ChangeBoostOrder(s *discordgo.Session, guildID string, channelID string, userID string, boostOrder string, redraw bool) (string, error) {
	var contract = FindContract(channelID)
	var boostOrderClean = ""
	if contract == nil {
		return "", errors.New(errorNoContract)
	}

	// if contract is in signup state return error
	if contract.State == ContractStateSignup {
		return "", errors.New(errorContractNotStarted)
	}

	// return an error if the userID isn't the contract creator
	if !creatorOfContract(s, contract, userID) {
		return "", errors.New("only the contract creator can change the contract")
	}

	// get current booster boost state
	var currentBooster = ""
	if contract.State == ContractStateFastrun || contract.State == ContractStateBanker {
		currentBooster = contract.Order[contract.BoostPosition]
	}

	log.Println("ChangeBoostOrder", "GuildID: ", guildID, "ChannelID: ", channelID, "UserID: ", userID, "BoostOrder: ", boostOrder)

	// split the boostOrder string into an array by commas
	re := regexp.MustCompile(`[\\<>@#&!]`)
	if boostOrder != "" {
		boostOrderClean = re.ReplaceAllString(normalizeMentionSyntax(boostOrder), "")
	}

	var boostOrderArray = strings.Split(boostOrderClean, ",")
	var boostOrderExpanded []string
	// expand hyphenated values into a range, incrementing or decrementing as appropriate and append them to the boostOrderArray
	for _, element := range boostOrderArray {
		var hyphenArray = strings.Split(element, "-")
		if len(hyphenArray) == 2 {
			var start, _ = strconv.Atoi(hyphenArray[0])
			var end, _ = strconv.Atoi(hyphenArray[1])
			if start > end {
				for j := start; j >= end; j-- {

					boostOrderExpanded = append(boostOrderExpanded, strconv.Itoa(j))
				}
			} else {
				for j := start; j <= end; j++ {
					boostOrderExpanded = append(boostOrderExpanded, strconv.Itoa(j))
				}
			}
			//boostOrderExpanded = removeBoostOrderIndex(boostOrderExpanded, i)
		} else {
			boostOrderExpanded = append(boostOrderExpanded, element)
		}

	}

	// Remove duplicates from boostOrderArray calling removeDuplicates function
	boostOrderArray = removeDuplicates(boostOrderExpanded)

	// if length of boostorderarray doesn't mach length of contract.Order then return error
	if len(boostOrderArray) != len(contract.Order) {
		return "", errors.New("invalid boost order. Every position needs to be specified")
	}

	// convert boostOrderArray to an array of ints
	var boostOrderIntArray []int
	for _, element := range boostOrderArray {
		var intElement, _ = strconv.Atoi(element)
		boostOrderIntArray = append(boostOrderIntArray, intElement)
	}

	// reorder data in contract.Order using the idnex order specified in boostOrderIntArray
	var newOrder []string
	for _, element := range boostOrderIntArray {
		newOrder = append(newOrder, contract.Order[element-1])
	}

	// Clear current booster boost state
	//if contract.State == ContractStateStarted {
	//	contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = BoostStateUnboosted
	//}

	// set contract.BoostOrder to the index of the element contract.Boosters[element].BoostState == BoostStateTokenTime
	contract.Order = removeDuplicates(newOrder)
	contract.OrderRevision++

	if contract.State == ContractStateFastrun || contract.State == ContractStateBanker {
		for i, el := range newOrder {
			if el == currentBooster {
				contract.BoostPosition = i
			}
		}
	}

	//sendNextNotification(s, contract, true)
	if redraw {
		refreshBoostListMessage(s, contract, false)
	}

	summaryStr := fmt.Sprintf("Boost order changed to %s.", boostOrder)
	if contract.BoostPosition < len(contract.Order) {
		summaryStr += fmt.Sprintf(" Current booster is %s. ", contract.Boosters[contract.Order[contract.BoostPosition]].Mention)
	}

	return summaryStr, nil
}

// MoveBooster will move a booster to a new position in the contract
func MoveBooster(s *discordgo.Session, guildID string, channelID string, userID string, boosterName string, boosterPosition int, redraw bool) error {
	var contract = FindContract(channelID)
	if contract == nil {
		return errors.New(errorNoContract)
	}

	// return an error if the contract is in the signup state
	if contract.State == ContractStateSignup {
		return errors.New(errorContractNotStarted)
	}

	// return an error if the userID isn't the contract creator
	if !creatorOfContract(s, contract, userID) {
		return errors.New("only the contract creator can change the contract")
	}

	if boosterPosition > len(contract.Order) {
		return errors.New("invalid position")
	}

	log.Println("MoveBooster", "GuildID: ", guildID, "ChannelID: ", channelID, "UserID: ", userID, "BoosterName: ", boosterName, "BoosterPosition: ", boosterPosition)

	var boosterIndex = slices.Index(contract.Order, boosterName)
	if boosterIndex == -1 {
		return errors.New("this booster not in contract")
	}

	if (boosterIndex + 1) == boosterPosition {
		return errors.New("booster already in this position")
	}

	currentBooster := contract.Order[contract.BoostPosition]

	var newOrder []string
	copyOrder := removeIndex(contract.Order, boosterIndex)
	if len(copyOrder) == 0 {
		newOrder = append(newOrder, boosterName)
	} else if boosterPosition > len(copyOrder) {
		// Booster at end of list
		newOrder = append(copyOrder, boosterName)
	} else {
		// loop through copyOrder
		for i, element := range copyOrder {
			if i == boosterPosition-1 {
				newOrder = append(newOrder, boosterName)
				newOrder = append(newOrder, element)
			} else {
				newOrder = append(newOrder, element)
			}
		}
	}

	// Swap in the new order and redraw the list
	contract.Order = removeDuplicates(newOrder)
	contract.OrderRevision++

	if contract.State == ContractStateFastrun || contract.State == ContractStateBanker {
		for i, el := range newOrder {
			if el == currentBooster {
				contract.BoostPosition = i
			}
		}
	}
	if redraw {
		refreshBoostListMessage(s, contract, false)
	}

	return nil
}

// HandleLinkAlternateCommand will handle the /link-alternate command
func HandleLinkAlternateCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Protection against DM use as we need the channel ID to find the contract
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

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing...",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	var str = ""

	contract := FindContract(i.ChannelID)
	if contract == nil {
		str = errorNoContract
	}

	// Is this user in the contract?
	if !userInContract(contract, i.Member.User.ID) {
		str = "You need to be in this contract to link an alternate that is also in the contract."
	}

	// No error string means we are good to go
	if str == "" {
		// Default to @here when there is no parameter
		newAlt := ""

		optionMap := bottools.GetCommandOptionsMap(i)

		if opt, ok := optionMap["farmer-name"]; ok {
			newAlt = strings.TrimSpace(opt.StringValue())

			// Is this booster in the contract?
			if _, ok := contract.Boosters[newAlt]; !ok {
				str = "This farmer is not in the contract"
			} else {
				b := contract.Boosters[i.Member.User.ID]

				newAltIcon := findAltIcon(newAlt, contract.AltIcons)

				// Save remember this alt's owner so we can auto link next time
				farmerstate.SetMiscSettingString(newAlt, "AltController", i.Member.User.ID)

				b.Alts = append(b.Alts, newAlt)
				b.AltsIcons = append(b.AltsIcons, newAltIcon)
				contract.AltIcons = append(contract.AltIcons, newAltIcon)
				contract.Boosters[newAlt].AltController = i.Member.User.ID
				rebuildAltList(contract)
				str = "Associated your `" + newAlt + "` alt with " + i.Member.User.Mention() + "\n"
				str += "> Use the Signup sink buttons to select your alt for sinks, these cycle through alts so you may need to press them multiple times.\n"
				str += "> Use the " + boostIcon + " reaction to indicate when your main or alt(s) boost.\n"
				str += "> Use the normal token buttons to indicate when `" + newAlt + "` sends tokens."
				contract.buttonComponents = nil // reset button components
				defer saveData(contract.ContractHash)
				//if contract.State == ContractStateSignup {
				refreshBoostListMessage(s, contract, false)
				//} else {
				//	_ = RedrawBoostList(s, i.GuildID, i.ChannelID)
				//}
			}
		}
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Content: str},
	)
}

func findAltIcon(newAlt string, altIcons []string) string {
	altIcon := ""
	// Create an alphabet slice of 🇦 to 🇿
	alphabet := make([]string, 0)
	for i := 'A'; i <= 'Z'; i++ {
		alphabet = append(alphabet, string('🇦'+(i-'A')))
	}
	for _, char := range strings.ToLower(newAlt) {
		// Only want alpha digits
		if char < 'a' || char > 'z' {
			continue
		}

		altIcon = alphabet[char-'a']
		if slices.Index(altIcons, altIcon) == -1 {
			break
		}
	}
	return altIcon
}

func rebuildAltList(contract *Contract) {
	contract.AltIcons = make([]string, 0)
	for _, b := range contract.Boosters {
		if len(b.AltsIcons) != 0 {
			contract.AltIcons = append(contract.AltIcons, b.AltsIcons...)
		}
	}

}

// HandleLinkAlternateAutoComplete will handle the /link-alternate autocomplete
func HandleLinkAlternateAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0)

	contract := FindContract(i.ChannelID)
	if contract != nil {
		for _, b := range contract.Boosters {
			if b.UserID != b.Name {
				continue
			}
			if b.AltController != "" {
				continue
			}

			choice := discordgo.ApplicationCommandOptionChoice{
				Name:  b.Name,
				Value: b.Name,
			}
			choices = append(choices, &choice)
		}
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Content: "Contract ID",
			Choices: choices,
		}})
}
