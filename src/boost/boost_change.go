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

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

// GetSlashAdminContractsListCommand returns the command definition for admin contract list
func GetSlashAdminContractsListCommand(cmd string) *dc.Command {
	command := adminGuildCommand(cmd, "List all running contracts")
	return &command
}

// GetSlashJoinContractCommand returns the command definition for joining a contract
func GetSlashJoinContractCommand(cmd string) *dc.Command {
	tokenMin, tokenMax := 0, 14
	command := guildOnlyCommand(cmd, "Add farmer or guest to contract.")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:        "farmer",
			Description: "User mention or guest name to add to existing contract",
			Required:    true,
		},
		dc.IntOption{
			Name:        "token-count",
			Description: "Set the number of boost tokens for this farmer. Default is 8.",
			MinValue:    &tokenMin,
			MaxValue:    &tokenMax,
		},
		dc.IntOption{
			Name:        "boost-order",
			Description: "Order farmer added to contract. Default is Signup order.",
			Choices: []dc.Choice[int]{
				{Name: "Sign-up Ordering", Value: ContractOrderSignup},
				{Name: "Time Based Ordering", Value: ContractOrderTimeBased},
				{Name: "Random Ordering", Value: ContractOrderRandom},
			},
		},
		dc.BoolOption{
			Name:        "already-boosted",
			Description: "Add farmer in an already boosted state.",
		},
	}
	return &command
}

// GetSlashBoostCommand returns the command definition for boosting
func GetSlashBoostCommand(cmd string) *dc.Command {
	command := guildOnlyCommand(cmd, "Spending tokens to boost!")
	return &command
}

// GetSlashSkipCommand returns the command definition for skipping a booster
func GetSlashSkipCommand(cmd string) *dc.Command {
	command := guildOnlyCommand(cmd, "Move current booster to last in boost order.")
	return &command
}

// GetSlashUnboostCommand returns the command definition for unboosting
func GetSlashUnboostCommand(cmd string) *dc.Command {
	command := guildOnlyCommand(cmd, "Change boost state to unboosted.")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:        "farmer",
			Description: "User Mention",
			Required:    true,
		},
	}
	return &command
}

// GetSlashPruneCommand returns the command definition for pruning a booster
func GetSlashPruneCommand(cmd string) *dc.Command {
	command := guildOnlyCommand(cmd, "Prune Booster")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:        "farmer",
			Description: "User Mention",
			Required:    true,
		},
	}
	return &command
}

// GetSlashBumpCommand returns the command definition for bumping a contract
func GetSlashBumpCommand(cmd string) *dc.Command {
	command := guildOnlyCommand(cmd, "Redraw the boost list to the timeline.")
	return &command
}

// GetSlashBumpCRCommand returns the command definition for bumping CR messages
func GetSlashBumpCRCommand(cmd string) *dc.Command {
	command := guildOnlyCommand(cmd, "Redraw the chicken run messages to the timeline.")
	return &command
}

// GetSlashToggleContractPingsCommand returns the command definition for toggling pings
func GetSlashToggleContractPingsCommand(cmd string) *dc.Command {
	command := guildOnlyCommand(cmd, "Toggle Boost Bot contract pings [sticky]")
	return &command
}

// GetSlashContractSettingsCommand returns the command definition for contract settings
func GetSlashContractSettingsCommand(cmd string) *dc.Command {
	command := guildOnlyCommand(cmd, "Coordinator of contract can use this to show initial settings")
	return &command
}

// GetSlashChangeOneBoosterCommand adjust aspects of a running contract
func GetSlashChangeOneBoosterCommand(cmd string) *dc.Command {
	positionMin := 1
	command := guildOnlyCommand(cmd, "Move booster to a new position. If current booster, will assign new booster")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:        "booster-name",
			Description: "Booster to move. Use an @mention or guest farmer name",
			Required:    true,
		},
		dc.IntOption{
			Name:        "new-position",
			Description: "Position to move the booster to",
			Required:    true,
			MinValue:    &positionMin,
		},
	}
	return &command
}

// GetSlashChangePlannedStartCommand adjust aspects of a running contract
func GetSlashChangePlannedStartCommand(cmd string) *dc.Command {
	command := guildOnlyCommand(cmd, "Change the planned start time of the contract")
	command.Options = []dc.Option{
		dc.SubCommand{
			Name:        "offset",
			Description: "Relative offset",
			Options: []dc.Option{
				dc.StringOption{
					Name:        "relative-time",
					Description: "Relative time offset from 9:00 AM. Example: +2.5 or -1.5",
					Required:    true,
				},
			},
		},
		dc.SubCommand{
			Name:        "timestamp",
			Description: "Discord Timestamp",
			Options: []dc.Option{
				dc.StringOption{
					Name:        "start-time",
					Description: "Discord Timestamp format. Example: <t:1716822000:f>",
					Required:    true,
				},
			},
		},
	}
	return &command
}

// GetSlashLinkAlternateCommand allows a player to associate an alt.
func GetSlashLinkAlternateCommand(cmd string) *dc.Command {
	command := guildOnlyCommand(cmd, "Add an alternate persona for this contract.")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:         "farmer-name",
			Description:  "Name of your alternate persona. This guest needs to be in the contract.",
			Required:     true,
			Autocomplete: true,
		},
	}
	return &command
}

func extractUserID(client dc.Client, boosterName string) (string, error) {
	if userID, isMention := parseMentionUserID(boosterName); isMention {
		u, err := client.User(userID)
		if err != nil {
			return "", err
		}
		return u.ID, nil
	}
	return normalizeUserIDInput(boosterName), nil
}

// HandleChangeOneBoosterCommand handles the /change-one-booster command
// through the dc facade.
//
// It still takes a raw session because the boost order mutators are not on the
// facade yet.
func HandleChangeOneBoosterCommand(client dc.Client, e *dc.CommandEvent) {
	// Protection against DM use as we need the channel ID to find the contract
	if e.GuildID() == "" {
		_ = e.Respond(dc.Message{
			Content:   "This command can only be run in a server.",
			Ephemeral: true,
		})
		return
	}

	var str = ""

	var err error
	contract := FindContract(e.ChannelID())
	if contract == nil {
		_ = e.Respond(dc.Message{
			Content:   errorNoContract,
			Ephemeral: true,
		})
		return
	}

	position := 0
	boosterName := ""
	newBooster := ""

	if opt, ok := e.OptInt("new-position"); ok {
		position = opt
		if position > len(contract.Order) {
			str = "Invalid position, must be between 1 and " + strconv.Itoa(len(contract.Order))
		}
	}

	if opt, ok := e.OptString("booster-name"); ok {
		// String in the form of mention
		boosterName = strings.TrimSpace(opt)
		boosterName, err = extractUserID(client, boosterName)
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
			} else if boosterName == contract.currentBoosterID() {
				// If this is current booster, we need to reassign this to the next booster
				newBoosterIndex := findNextBoosterAfterUser(contract, boosterName)
				if newBoosterIndex != -1 {
					newBooster = contract.Order[newBoosterIndex]
				}
			} else {
				// Is the new position the current booster?
				if position > 0 && position <= len(contract.Order) && contract.Order[position-1] == contract.currentBoosterID() {
					newBooster = boosterName
				}
			}
		}
	}

	_ = e.Defer(true)

	// Empty string means we are good to go
	if str == "" {

		err := MoveBooster(client, e.GuildID(), e.ChannelID(), e.UserID(), boosterName, position, newBooster == "")
		if err != nil {
			str += err.Error()
		} else {
			str += fmt.Sprintf("Moved %s to position %d.", contract.Boosters[boosterName].Mention, position)

			if newBooster != "" && contract.State != ContractStateSignup {
				err := ChangeCurrentBooster(client, e.GuildID(), e.ChannelID(), e.UserID(), newBooster, true)
				if err != nil {
					str += " " + strings.ToUpper(string(err.Error()[0])) + err.Error()[1:]
				} else {
					str += fmt.Sprintf(" Current booster changed to %s.", contract.Boosters[newBooster].Mention)
				}
			}
		}
	}

	_ = e.Followup(dc.Message{Content: str})

}

// HandleChangePlannedStartCommand handles the /change-planned-start command
// through the dc facade.
//
// It still takes a raw session because creatorOfContract and the boost list
// redraw are not on the facade yet.
func HandleChangePlannedStartCommand(client dc.Client, e *dc.CommandEvent) {
	// Protection against DM use as we need the channel ID to find the contract
	if e.GuildID() == "" {
		_ = e.Respond(dc.Message{
			Content:   "This command can only be run in a server.",
			Ephemeral: true,
		})
		return
	}

	_ = e.Defer(true)

	var str = ""

	contract := FindContract(e.ChannelID())
	if contract == nil {
		str = errorNoContract
	} else {
		if !creatorOfContract(client, contract, e.UserID()) {
			str = "only the contract creator can change the contract"
		}
	}

	// No error string means we are good to go
	if str == "" {
		// Default to @here when there is no parameter

		if opt, ok := e.OptString("offset-relative-time"); ok {
			offsetStr := opt
			if strings.EqualFold(offsetStr, "tbd") {
				contract.PlannedStartTime = time.Time{}
				str = "Planned start time set to TBD"
				refreshBoostListMessage(client, contract, false)
			} else {
				offset, err := strconv.ParseFloat(offsetStr, 64)
				if err != nil {
					str = "Invalid offset format. Use a number like +2.5 or -1.5 or TBD"
				} else {
					baseTime := GetEggStandardTime(time.Now())

					// Apply offset
					offsetDuration := time.Duration(offset * float64(time.Hour))
					resultTime := baseTime.Add(offsetDuration)

					// If the resulting time is in the past, push it forward day by day until it's in the future
					for resultTime.Before(time.Now()) {
						resultTime = resultTime.Add(24 * time.Hour)
					}

					startTime := resultTime.Unix()

					contract.PlannedStartTime = time.Unix(startTime, 0)
					str = "Planned start time changed to " + "<t:" + strconv.FormatInt(startTime, 10) + ":f>"
					refreshBoostListMessage(client, contract, false)
				}
			}
		}

		if opt, ok := e.OptString("timestamp-start-time"); ok {
			var startTime int64
			var err error
			startTimeStr := opt

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
				if startTime == 0 {
					contract.PlannedStartTime = time.Time{}
					str = "Planned start time cleared"
					refreshBoostListMessage(client, contract, false)
				} else {
					contract.PlannedStartTime = time.Unix(startTime, 0)
					if contract.PlannedStartTime.After(time.Now()) && contract.PlannedStartTime.Before(time.Now().AddDate(0, 0, 7)) {
						str = "Planned start time changed to " + "<t:" + strconv.FormatInt(startTime, 10) + ":f>"
						refreshBoostListMessage(client, contract, false)
					} else {
						str = "Planned start time must be within the next 7 days. Use timestamps from [Discord Timestamp](https://discordtimestamp.com)"
						contract.PlannedStartTime = time.Time{}
					}
				}
			}
		}
	}

	_ = e.Followup(dc.Message{Content: str})
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

func moveOverflowBoostersToWaitlist(contract *Contract) []string {
	if contract == nil || contract.CoopSize < 0 {
		return nil
	}

	if len(contract.Boosters) <= contract.CoopSize || len(contract.Order) <= contract.CoopSize {
		return nil
	}

	overflowUsers := make([]string, 0)
	seen := make(map[string]bool)
	for _, userID := range contract.Order[contract.CoopSize:] {
		if contract.Boosters[userID] == nil || seen[userID] {
			continue
		}
		overflowUsers = append(overflowUsers, userID)
		seen[userID] = true
	}

	if len(overflowUsers) == 0 {
		return nil
	}

	movedLabels := make([]string, 0, len(overflowUsers))
	movedUserIDs := make([]string, 0, len(overflowUsers))
	altRelationshipsChanged := false

	for _, userID := range overflowUsers {
		for {
			idx := slices.Index(contract.Order, userID)
			if idx == -1 {
				break
			}
			contract.Order = removeIndex(contract.Order, idx)
		}

		booster := contract.Boosters[userID]
		if booster == nil {
			continue
		}

		boosterLabel := userID
		if booster.Mention != "" {
			boosterLabel = booster.Mention
		}

		if booster.AltController != "" {
			mainUserID := booster.AltController
			if contract.Boosters[mainUserID] != nil {
				altIdx := slices.Index(contract.Boosters[mainUserID].Alts, userID)
				if altIdx != -1 {
					contract.Boosters[mainUserID].Alts = removeIndex(contract.Boosters[mainUserID].Alts, altIdx)
					altRelationshipsChanged = true
				}
			}
		} else if len(booster.Alts) > 0 {
			for _, altID := range booster.Alts {
				if contract.Boosters[altID] != nil {
					contract.Boosters[altID].AltController = ""
				}
			}
			booster.Alts = nil
			altRelationshipsChanged = true
		}

		movedUserIDs = append(movedUserIDs, userID)
		movedLabels = append(movedLabels, boosterLabel)

		if contract.Ultra {
			contract.UltraCount--
		} else if farmerstate.IsUltra(userID) {
			contract.UltraCount--
		}

		delete(contract.Boosters, userID)
	}

	// Put moved boosters first in waitlist order, then keep existing waitlist order.
	filteredExistingWaitlist := make([]string, 0, len(contract.WaitlistBoosters))
	for _, existingUserID := range contract.WaitlistBoosters {
		if !slices.Contains(movedUserIDs, existingUserID) {
			filteredExistingWaitlist = append(filteredExistingWaitlist, existingUserID)
		}
	}

	contract.WaitlistBoosters = append(movedUserIDs, filteredExistingWaitlist...)

	if altRelationshipsChanged {
		contract.buttonComponents = nil
	}

	contract.RegisteredNum = len(contract.Boosters)
	contract.OrderRevision++
	return movedLabels
}

// ChangeContractIDs will change the contractID and/or coopID
func ChangeContractIDs(client dc.Client, guildID string, channelID string, userID string, contractID string, coopID string, coordinatorID string) (int, error) {
	var contract = FindContract(channelID)
	if contract == nil {
		return 0, errors.New(errorNoContract)
	}

	// return an error if the userID isn't the contract creator
	if !creatorOfContract(client, contract, userID) {
		return 0, errors.New("only the contract creator can change the contract")
	}

	movedToWaitlist := 0

	log.Println("ChangeContractIDs", "ContractID: ", contractID, "CoopID: ", coopID, "GuildID: ", guildID, "ChannelID: ", channelID, "UserID: ", userID, "Order: ", "")

	if contractID != "" {
		contract.ContractID = contractID
		updateContractWithEggIncData(client, contract)
		movedLabels := moveOverflowBoostersToWaitlist(contract)
		movedToWaitlist = len(movedLabels)
		contract.EggEmoji = FindEggEmoji(contract.EggName)

		if contract.Name != "" && contract.EggName != "" && !contract.PredictionSignup {
			creator := userID
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
			bottools.GenerateBanner(contract.ContractID, contract.EggName, contract.Name, creator, guildID, style)
		}

		UpdateBannerURL(contract)

		if len(movedLabels) > 0 {
			channelMsg := fmt.Sprintf("⚠️ Contract changed to %s and coop size is now %d. Moved %d booster(s) to waitlist: %s",
				contract.ContractID,
				contract.CoopSize,
				len(movedLabels),
				strings.Join(movedLabels, ", "),
			)
			if _, err := client.SendMessage(channelID, dc.Message{Content: channelMsg}); err != nil {
				log.Println("Error sending waitlist movement message:", err)
			}
		}

		// Rename the contract role to match the new contract
		renameContractRole(client, contract)

		refreshBoostListMessage(client, contract, false)
	}
	if coopID != "" {
		contract.CoopID = coopID
		refreshBoostListMessage(client, contract, true)
	}
	if coordinatorID != "" {
		if slices.Index(contract.Order, coordinatorID) != -1 {
			contract.CreatorID[0] = coordinatorID
		} else {
			return 0, errors.New("the selected coordinator needs to be in the contract")
		}
	}
	if contractID != "" || coopID != "" {
		CheckAndPublishAMQPContractUpdate(contract)
	}
	return movedToWaitlist, nil
}

// renameContractRole renames the contract role to match the new contract ID
func renameContractRole(client dc.Client, contract *Contract) {
	// Rename the role in each guild where the contract exists
	for _, loc := range contract.Location {
		if loc.GuildContractRole.ID == "" {
			continue
		}

		// Get the new thematic role name for the updated contract in this specific guild
		newRoleName := getContractRoleName(client, loc.GuildID, contract.ContractID)

		updatedRole, err := client.EditGuildRole(loc.GuildID, loc.GuildContractRole.ID, dc.RoleParams{
			Name: newRoleName,
		})

		if err != nil {
			log.Println("Error renaming contract role:", err)
		} else {
			if updatedRole != nil {
				loc.GuildContractRole = guildRoleFromDiscord(updatedRole)
			}
			loc.RoleManagedByBot = true
			log.Println("Successfully renamed contract role to:", newRoleName)
		}
	}
}

// ChangeCurrentBooster will change the current booster to the specified userID
func ChangeCurrentBooster(client dc.Client, guildID string, channelID string, userID string, newBooster string, redraw bool) error {
	var contract = FindContract(channelID)
	if contract == nil {
		return errors.New(errorNoContract)
	}

	// return an error if the contract is in the signup state
	if contract.State == ContractStateSignup {
		return errors.New(errorContractNotStarted)
	}

	// return an error if the userID isn't the contract creator
	if !creatorOfContract(client, contract, userID) {
		return errors.New("only the contract creator can change the contract")
	}

	log.Println("ChangeCurrentBooster", "GuildID: ", guildID, "ChannelID: ", channelID, "UserID: ", userID, "NewBooster: ", newBooster)

	newBoosterUserID := normalizeUserIDInput(newBooster)

	if slices.Index(contract.Order, newBoosterUserID) == -1 {
		return errors.New("this booster not in contract")
	}

	switch contract.Boosters[newBoosterUserID].BoostState {
	case BoostStateUnboosted:
		contract.Boosters[newBoosterUserID].StartTime = time.Now()
		contract.setCurrentBoosterByUserID(newBoosterUserID)
		contract.enforceOnlyOneTokenTimeBooster()
	case BoostStateTokenTime:
		return errors.New("this booster is already currently receiving tokens")
	case BoostStateBoosted:
		return errors.New("this booster already boosted")
	}

	// Clear current booster boost state
	if redraw {
		sendNextNotification(client, contract, true)
	}
	return nil
}

// ChangeBoostOrder will change the order of the boosters in the contract
func ChangeBoostOrder(client dc.Client, guildID string, channelID string, userID string, boostOrder string, redraw bool) (string, error) {
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
	if !creatorOfContract(client, contract, userID) {
		return "", errors.New("only the contract creator can change the contract")
	}

	// get current booster boost state
	var currentBooster = ""
	if contract.State == ContractStateFastrun || contract.State == ContractStateBanker {
		currentBooster = contract.currentBoosterID()
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
		intElement, err := strconv.Atoi(strings.TrimSpace(element))
		if err != nil || intElement < 1 || intElement > len(contract.Order) {
			return "", errors.New("invalid boost order. Positions must be between 1 and " + strconv.Itoa(len(contract.Order)))
		}
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
				contract.setCurrentBoosterByIndex(i)
				break
			}
		}
		// Enforce that only current booster has BoostStateTokenTime
		contract.enforceOnlyOneTokenTimeBooster()
	}

	//sendNextNotification(client, contract, true)
	if redraw {
		refreshBoostListMessage(client, contract, false)
	}

	summaryStr := fmt.Sprintf("Boost order changed to %s.", boostOrder)
	if currentID := contract.currentBoosterID(); currentID != "" {
		summaryStr += fmt.Sprintf(" Current booster is %s. ", contract.Boosters[currentID].Mention)
	}

	return summaryStr, nil
}

// MoveBooster will move a booster to a new position in the contract
func MoveBooster(client dc.Client, guildID string, channelID string, userID string, boosterName string, boosterPosition int, redraw bool) error {
	var contract = FindContract(channelID)
	if contract == nil {
		return errors.New(errorNoContract)
	}

	// return an error if the userID isn't the contract creator
	if !creatorOfContract(client, contract, userID) {
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

	currentBooster := contract.currentBoosterID()

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
				contract.setCurrentBoosterByIndex(i)
				break
			}
		}
		// Enforce that only current booster has BoostStateTokenTime
		contract.enforceOnlyOneTokenTimeBooster()
	}
	if redraw {
		refreshBoostListMessage(client, contract, false)
	}

	return nil
}

// HandleLinkAlternateCommand handles the /link-alternate command through the
// dc facade.
//
// It still takes a raw session because the boost list redraw is not on the
// facade yet.
func HandleLinkAlternateCommand(client dc.Client, e *dc.CommandEvent) {
	// Protection against DM use as we need the channel ID to find the contract
	if e.GuildID() == "" {
		_ = e.Respond(dc.Message{
			Content:   "This command can only be run in a server.",
			Ephemeral: true,
		})
		return
	}

	_ = e.Defer(true)

	var str = ""

	userID := e.UserID()

	contract := FindContract(e.ChannelID())
	if contract == nil {
		str = errorNoContract
	}

	// Is this user in the contract?
	if !UserInContract(contract, userID) {
		str = "You need to be in this contract to link an alternate that is also in the contract."
	}

	// No error string means we are good to go
	if str == "" {
		// Default to @here when there is no parameter
		newAlt := ""

		if opt, ok := e.OptString("farmer-name"); ok {
			newAlt = strings.TrimSpace(opt)

			// Is this booster in the contract?
			if _, ok := contract.Boosters[newAlt]; !ok {
				str = "This farmer is not in the contract"
			} else {
				b := contract.Boosters[userID]

				// Save remember this alt's owner so we can auto link next time
				farmerstate.SetMiscSettingString(newAlt, "AltController", userID)

				b.Alts = append(b.Alts, newAlt)
				contract.Boosters[newAlt].AltController = userID
				str = "Associated your `" + newAlt + "` alt with <@" + userID + ">\n"
				str += "> Use the Signup sink buttons to select your alt for sinks, these cycle through alts so you may need to press them multiple times.\n"
				str += "> Use the " + boostIcon + " reaction to indicate when your main or alt(s) boost.\n"
				str += "> Use the normal token buttons to indicate when `" + newAlt + "` sends tokens."
				contract.buttonComponents = nil // reset button components
				defer saveData(contract.ContractHash)
				//if contract.State == ContractStateSignup {
				refreshBoostListMessage(client, contract, false)
				//} else {
				//	_ = RedrawBoostList(client, e.GuildID(), e.ChannelID())
				//}
			}
		}
	}

	_ = e.Followup(dc.Message{Content: str})
}

// HandleLinkAlternateAutoComplete will handle the /link-alternate autocomplete
// through the dc facade.
func HandleLinkAlternateAutoComplete(e *dc.AutocompleteEvent) {
	choices := make([]dc.Choice[string], 0)

	contract := FindContract(e.ChannelID())
	if contract != nil {
		for _, b := range contract.Boosters {
			if b.UserID != b.Name {
				continue
			}
			if b.AltController != "" {
				continue
			}

			choices = append(choices, dc.Choice[string]{
				Name:  b.Name,
				Value: b.Name,
			})
		}
	}

	_ = e.RespondChoices(choices)
}
