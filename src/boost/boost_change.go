package boost

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// GetSlashChangeCommand adjust aspects of a running contract
func GetSlashChangeCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Change aspects of a running contract",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "coop-id",
				Description: "Change the coop-id",
				Required:    false,
			},
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "contract-id",
				Description:  "Change the contract-id",
				Required:     false,
				Autocomplete: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionRole,
				Name:        "ping-role",
				Description: "Change the contract ping role.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "one-boost-position",
				Description: "Move a booster to a specific order position.  Example: @farmer 4",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "boost-order",
				Description: "Provide new boost order. Example: 1,2,3,6,7,5,8-10",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "current-booster",
				Description: "Change the current booster. Example: @farmer",
				Required:    false,
			},
		},
	}
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

// ChangePingRole will change the ping role for the contract
func ChangePingRole(s *discordgo.Session, guildID string, channelID string, userID string, pingRole string) error {
	var contract = FindContract(channelID)
	if contract == nil {
		return errors.New(errorNoContract)
	}

	// return an error if the contract is in the signup state
	if contract.State == ContractStateSignup {
		return errors.New(errorContractNotStarted)
	}

	// return an error if the userID isn't the contract creator
	if !creatorOfContract(contract, userID) {
		return errors.New("only the contract creator can change the contract")
	}

	for _, loc := range contract.Location {
		if loc.ChannelID == channelID {
			loc.ChannelPing = pingRole
			return nil
		}
	}
	return errors.New(errorNoContract)
}

// ChangeContractIDs will change the contractID and/or coopID
func ChangeContractIDs(s *discordgo.Session, guildID string, channelID string, userID string, contractID string, coopID string) error {
	var contract = FindContract(channelID)
	if contract == nil {
		return errors.New(errorNoContract)
	}

	// return an error if the userID isn't the contract creator
	if !creatorOfContract(contract, userID) {
		return errors.New("only the contract creator can change the contract")
	}

	fmt.Println("ChangeContractIDs", "ContractID: ", contractID, "CoopID: ", coopID, "GuildID: ", guildID, "ChannelID: ", channelID, "UserID: ", userID, "Order: ", "")

	if contractID != "" {
		contract.ContractID = contractID
		updateContractWithEggIncData(contract)
	}
	if coopID != "" {
		contract.CoopID = coopID
	}
	return nil
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
	if !creatorOfContract(contract, userID) {
		return errors.New("only the contract creator can change the contract")
	}

	fmt.Println("ChangeCurrentBooster", "GuildID: ", guildID, "ChannelID: ", channelID, "UserID: ", userID, "NewBooster: ", newBooster)

	re := regexp.MustCompile(`[\\<>@#&!]`)
	var newBoosterUserID = re.ReplaceAllString(newBooster, "")

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
func ChangeBoostOrder(s *discordgo.Session, guildID string, channelID string, userID string, boostOrder string, redraw bool) error {
	var contract = FindContract(channelID)
	var boostOrderClean = ""
	if contract == nil {
		return errors.New(errorNoContract)
	}

	// if contract is in signup state return error
	if contract.State == ContractStateSignup {
		return errors.New(errorContractNotStarted)
	}

	// return an error if the userID isn't the contract creator
	if !creatorOfContract(contract, userID) {
		return errors.New("only the contract creator can change the contract")
	}

	// get current booster boost state
	var currentBooster = ""
	if contract.State == ContractStateStarted {
		currentBooster = contract.Order[contract.BoostPosition]
	}

	fmt.Println("ChangeBoostOrder", "GuildID: ", guildID, "ChannelID: ", channelID, "UserID: ", userID, "BoostOrder: ", boostOrder)

	// split the boostOrder string into an array by commas
	re := regexp.MustCompile(`[\\<>@#&!]`)
	if boostOrder != "" {
		boostOrderClean = re.ReplaceAllString(boostOrder, "")
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
		return errors.New("invalid boost order. Every position needs to be specified")
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

	if contract.State == ContractStateStarted {
		for i, el := range newOrder {
			if el == currentBooster {
				contract.BoostPosition = i
			}
		}
	}

	//sendNextNotification(s, contract, true)
	if redraw {
		refreshBoostListMessage(s, contract)
	}
	return nil
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
	if !creatorOfContract(contract, userID) {
		return errors.New("only the contract creator can change the contract")
	}

	if boosterPosition > len(contract.Order) {
		return errors.New("invalid position")
	}

	fmt.Println("MoveBooster", "GuildID: ", guildID, "ChannelID: ", channelID, "UserID: ", userID, "BoosterName: ", boosterName, "BoosterPosition: ", boosterPosition)

	var boosterIndex = slices.Index(contract.Order, boosterName)
	if boosterIndex == -1 {
		return errors.New("this booster not in contract")
	}

	if (boosterIndex + 1) == boosterPosition {
		return errors.New("booster already in this position")
	}

	if boosterIndex < contract.BoostPosition {
		boosterPosition--
	}

	currentBooster := contract.Order[contract.BoostPosition]

	var newOrder []string
	copyOrder := removeIndex(contract.Order, boosterIndex)
	if len(copyOrder) == 0 {
		newOrder = append(newOrder, boosterName)
	} else if boosterPosition >= len(copyOrder) {
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

	if contract.State == ContractStateStarted {
		for i, el := range newOrder {
			if el == currentBooster {
				contract.BoostPosition = i
			}
		}
	}
	if redraw {
		refreshBoostListMessage(s, contract)
	}

	return nil
}
