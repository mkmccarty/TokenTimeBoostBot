package boost

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/divan/num2words"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/peterbourgon/diskv/v3"
	emutil "github.com/post04/discordgo-emoji-util"
)

//var usermutex sync.Mutex
//var diskmutex sync.Mutex

var DataStore *diskv.Diskv

//var TokenStr = "" //"<:token:778019329693450270>"

const errorNoContract string = "contract doesn't exist"
const errorNotStarted string = "contract not started"
const errorContractFull string = "contract is full"
const errorNoFarmer string = "farmer doesn't exist"
const errorUserInContract string = "farmer already in contract"
const errorUserNotInContract string = "farmer not in contract"
const errorBot string = "cannot be a bot"
const errorContractEmpty = "contract doesn't have farmers"
const errorContractNotStarted = "contract hasn't started"
const errorContractAlreadyStarted = "contract already started"
const errorAlreadyBoosted = "farmer boosted already"
const errorNotContractCreator = "restricted to contract creator"

const ContractOrderSignup = 0
const ContractOrderReverse = 1
const ContractOrderRandom = 2
const ContractOrderFair = 3
const ContractOrderTimeBased = 4

const ContractStateSignup = 0
const ContractStateStarted = 1
const ContractStateWaiting = 2
const ContractStateCompleted = 3

const BoostStateUnboosted = 0
const BoostStateTokenTime = 1
const BoostStateBoosted = 2

const BoostOrderTimeThreshold = 20

type EggFarmer struct {
	UserID      string // Discord User ID
	Username    string
	Unique      string
	Nick        string
	ChannelName string
	GuildID     string // Discord Guild where this User is From
	GuildName   string
	Reactions   int       // Number of times farmer reacted
	Ping        bool      // True/False
	Register    time.Time //o Time Farmer registered to boost
	//Cluck       []string  // Keep track of messages from each user
}

type Booster struct {
	UserID         string // Egg Farmer
	Name           string
	BoostState     int           // Indicates if current booster
	Mention        string        // String which mentions user
	TokensReceived int           // indicate number of boost tokens
	TokensWanted   int           // indicate number of boost tokens
	StartTime      time.Time     // Time Farmer started boost turn
	EndTime        time.Time     // Time Farmer ended boost turn
	Duration       time.Duration // Duration of boost
}

type LocationData struct {
	GuildID          string
	GuildName        string
	ChannelID        string // Contract Discord Channel
	ChannelName      string
	ChannelMention   string
	ChannelPing      string
	ListMsgID        string   // Message ID for the Last Boost Order message
	ReactionID       string   // Message ID for the reaction Order String
	MessageIDs       []string // Array of message IDs for any contract message
	TokenStr         string   // Emoji for Token
	TokenReactionStr string   // Emoji for Token Reaction
}
type Contract struct {
	ContractHash  string // ContractID-CoopID
	Location      []*LocationData
	CreatorID     []string // Slice of creators
	ContractID    string   // Contract ID
	CoopID        string   // CoopID
	CoopSize      int
	BoostOrder    int // How the contract is sorted
	BoostVoting   int
	BoostPosition int       // Starting Slot
	State         int       // Boost Completed
	StartTime     time.Time // When Contract is started
	EndTime       time.Time // When final booster ends
	EggFarmers    map[string]*EggFarmer
	RegisteredNum int
	Boosters      map[string]*Booster // Boosters Registered
	Order         []string
	OrderRevision int        // Incremented when Order is changed
	mutex         sync.Mutex // Keep this contract thread safe
}

var (
	// DiscordToken holds the API Token for discord.
	Contracts map[string]*Contract
)

func init() {
	Contracts = make(map[string]*Contract)

	// DataStore to initialize a new diskv store, rooted at "my-data-dir", with a 1MB cache.
	DataStore = diskv.New(diskv.Options{
		BasePath:          "ttbb-data",
		AdvancedTransform: AdvancedTransform,
		InverseTransform:  InverseTransform,
		CacheSizeMax:      1024 * 1024,
	})

	var c, err = loadData()
	if err == nil {
		Contracts = c
	}

}
func RemoveLocIndex(s []*LocationData, index int) []*LocationData {
	return append(s[:index], s[index+1:]...)
}

func DeleteContract(s *discordgo.Session, guildID string, channelID string) (string, error) {
	var contract = FindContract(guildID, channelID)
	if contract == nil {
		return "", errors.New(errorNoContract)
	}

	//contract.mutex.Lock()
	//defer contract.mutex.Unlock()
	var coop = contract.ContractHash
	saveEndData(contract) // Save for historical purposes

	for _, el := range contract.Location {
		s.ChannelMessageDelete(el.ChannelID, el.ListMsgID)
		s.ChannelMessageDelete(el.ChannelID, el.ReactionID)
	}
	delete(Contracts, coop)
	saveData(Contracts)

	return coop, nil
}

func FindTokenEmoji(s *discordgo.Session, guildID string) string {
	g, _ := s.State.Guild(guildID) // RAIYC Playground
	var e = emutil.FindEmoji(g.Emojis, "token", false)
	if e != nil {
		return e.MessageFormat()
	}
	e = emutil.FindEmoji(g.Emojis, "Token", false)
	if e != nil {
		return e.MessageFormat()
	}
	return "🐣"
}

func getBoostOrderString(contract *Contract) string {
	var thresholdStartTime = contract.StartTime.Add(time.Minute * time.Duration(BoostOrderTimeThreshold))
	if contract.State != ContractStateSignup {
		if contract.BoostOrder == ContractOrderFair || contract.BoostOrder == ContractOrderRandom {
			var timeSinceStart = time.Since(contract.StartTime)
			var minutesSinceStart = int(timeSinceStart.Minutes())
			if minutesSinceStart > BoostOrderTimeThreshold {
				contract.BoostOrder = ContractOrderSignup
			}
		}
	}

	switch contract.BoostOrder {
	case ContractOrderSignup:
		return "Sign-up"
	case ContractOrderReverse:
		return "Reverse"
	case ContractOrderRandom:
		if contract.StartTime.IsZero() {
			return "Random"
		}
		return fmt.Sprintf("Random -> Sign-up <t:%d:R> ", thresholdStartTime.Unix())
	case ContractOrderFair:
		if contract.StartTime.IsZero() {
			return "Fair"
		}
		return fmt.Sprintf("Fair -> Sign-up <t:%d:R> ", thresholdStartTime.Unix())
	case ContractOrderTimeBased:
		return "Time"
	}
	return "Unknown"
}

// CreateContract creates a new contract or joins an existing contract if run from a different location
func CreateContract(s *discordgo.Session, contractID string, coopID string, coopSize int, BoostOrder int, guildID string, channelID string, userID string, pingRole string) (*Contract, error) {
	var ContractHash = fmt.Sprintf("%s/%s", contractID, coopID)

	contract := Contracts[ContractHash]

	loc := new(LocationData)
	loc.GuildID = guildID
	loc.ChannelID = channelID
	var g, gerr = s.Guild(guildID)
	if gerr == nil {
		loc.GuildName = g.Name

	}
	var c, cerr = s.Channel(channelID)
	if cerr == nil {
		loc.ChannelName = c.Name
		loc.ChannelMention = c.Mention()
		loc.ChannelPing = pingRole
	}
	loc.ListMsgID = ""
	loc.ReactionID = ""

	if contract == nil {
		// We don't have this contract on this channel, it could exist in another channel
		contract = new(Contract)
		contract.Location = append(contract.Location, loc)
		contract.ContractHash = ContractHash

		//GlobalContracts[ContractHash] = append(GlobalContracts[ContractHash], loc)
		contract.EggFarmers = make(map[string]*EggFarmer)
		contract.Boosters = make(map[string]*Booster)
		contract.ContractID = contractID
		contract.CoopID = coopID
		contract.BoostOrder = BoostOrder
		contract.BoostVoting = 0
		contract.OrderRevision = 0
		contract.State = ContractStateSignup
		contract.CreatorID = append(contract.CreatorID, userID) // starting userid

		if slices.Index(contract.CreatorID, config.AdminUserID) == -1 {
			contract.CreatorID = append(contract.CreatorID, config.AdminUserID) // overall admin user
		}
		if slices.Index(contract.CreatorID, "393477262412087319") == -1 {
			contract.CreatorID = append(contract.CreatorID, "393477262412087319") // Tbone user id
		}
		if slices.Index(contract.CreatorID, "430186990260977665") == -1 {
			contract.CreatorID = append(contract.CreatorID, "430186990260977665") // Aggie user id
		}
		contract.RegisteredNum = 0
		contract.CoopSize = coopSize
		Contracts[ContractHash] = contract
	} else { //if !creatorOfContract(contract, userID) {
		//contract.mutex.Lock()
		contract.CreatorID = append(contract.CreatorID, userID) // starting userid
		contract.Location = append(contract.Location, loc)
		//contract.mutex.Unlock()
	}

	// Find our Token emoji
	for _, el := range contract.Location {
		el.TokenStr = FindTokenEmoji(s, el.GuildID)
		// set TokenReactionStr to the TokenStr without first 2 characters and last character
		el.TokenReactionStr = el.TokenStr[2 : len(el.TokenStr)-1]
	}

	return contract, nil
}

func AddBoostTokens(s *discordgo.Session, guildID string, channelID string, userID string, setCountWant int, countWantAdjust int, countReceivedAdjust int) (int, int, error) {
	// Find the contract
	var contract = FindContract(guildID, channelID)
	if contract == nil {
		return 0, 0, errors.New(errorNoContract)
	}
	// verify the user is in the contract
	if !userInContract(contract, userID) {
		return 0, 0, errors.New(errorUserNotInContract)
	}

	// Add the token count for the userID, ensure the count is not negative
	var b = contract.Boosters[userID]
	if b == nil {
		return 0, 0, errors.New(errorUserNotInContract)
	}

	if setCountWant > 0 {
		b.TokensWanted = setCountWant
	}

	b.TokensWanted += countWantAdjust
	if b.TokensWanted < 0 {
		b.TokensWanted = 0
	}

	farmerstate.SetTokens(b.UserID, b.TokensWanted)

	// Add received tokens to current booster
	if countReceivedAdjust > 0 {
		contract.Boosters[contract.Order[contract.BoostPosition]].TokensReceived += countReceivedAdjust
		//TODO: Maybe track who's sending tokens
	}

	refreshBoostListMessage(s, contract)

	return b.TokensWanted, b.TokensReceived, nil
}

func SetListMessageID(contract *Contract, channelID string, messageID string) {
	for _, element := range contract.Location {
		if element.ChannelID == channelID {
			element.ListMsgID = messageID
			if slices.Index(element.MessageIDs, messageID) == -1 {
				element.MessageIDs = append(element.MessageIDs, messageID)
			}
		}
	}
	saveData(Contracts)
}

func SetReactionID(contract *Contract, channelID string, reactionID string) {
	for _, element := range contract.Location {
		if element.ChannelID == channelID {
			element.ReactionID = reactionID
			if slices.Index(element.MessageIDs, reactionID) == -1 {
				element.MessageIDs = append(element.MessageIDs, reactionID)
			}
		}
	}
	saveData(Contracts)
}

func getTokenCountString(tokenStr string, tokensWanted int, tokensReceived int) (string, string) {
	countStr := ""
	signupCountStr := ""
	if tokensWanted > 0 {
		var tokens = tokensWanted - tokensReceived
		if tokens < 0 {
			tokens = 0
		}

		// make countStr string with tokens number of duplicates of tokenStr
		// Build the token string, countdown from 8 to 1 and then emoji
		if tokens == 0 {
			countStr = "🚀"
		} else {
			for i := 0; i < tokens; i++ {
				if i == 9 {
					countStr += "+"
					break
				}
				countStr += fmt.Sprintf(":%s:", num2words.Convert(i+1))
			}
		}
		countStr += tokenStr

		//signupCountStr = fmt.Sprintf(" :%s:", num2words.Convert(tokens))
		signupCountStr = fmt.Sprintf(" (%d)", tokens)
	}
	return countStr, signupCountStr
}

func DrawBoostList(s *discordgo.Session, contract *Contract, tokenStr string) string {
	var outputStr = ""
	saveData(Contracts)

	outputStr = fmt.Sprintf("### %s - 📋%s - %d/%d\n", contract.ContractHash, getBoostOrderString(contract), len(contract.Boosters), contract.CoopSize)
	outputStr += fmt.Sprintf("> Coordinator: <@%s>", contract.CreatorID[0])
	outputStr += "\n"

	if contract.State == ContractStateSignup {
		outputStr += "## Sign-up List\n"
	} else {
		outputStr += "## Boost List\n"
	}
	var prefix = " - "

	earlyList := ""
	lateList := ""

	offset := 1

	// Some actions result in an unboosted farmer with the contract state still unset

	if contract.State == ContractStateWaiting {
		//set unboosted to true if any boosters are unboosted
		for _, element := range contract.Order {
			var b, ok = contract.Boosters[element]
			if ok {
				if b.BoostState == BoostStateUnboosted || b.BoostState == BoostStateTokenTime {
					contract.State = ContractStateStarted
					break
				}
			}
		}
	}

	showBoostedNums := 2
	windowSize := 10
	orderSubset := contract.Order
	if contract.State != ContractStateSignup && len(contract.Order) > 16 {
		// extract 10 elements around the current booster
		var start = contract.BoostPosition - showBoostedNums
		var end = contract.BoostPosition + (windowSize - showBoostedNums)

		if start < 0 {
			// add the aboslute value of start to end
			end += -start
			start = 0
		}
		if end > len(contract.Order) {
			start -= end - len(contract.Order)
			end = len(contract.Order)
		}
		// populate earlyList with all elements from earlySubset
		for i, element := range contract.Order[0:start] {
			var b, ok = contract.Boosters[element]
			if ok {
				if b.BoostState == BoostStateBoosted {
					earlyList += fmt.Sprintf("~~%s~~ ", b.Mention)
				} else {
					earlyList += fmt.Sprintf("%s(%d) ", b.Mention, b.TokensWanted)
				}
				if i < start-1 {
					earlyList += ", "
				}
			}
		}
		if earlyList != "" {
			if start == 1 {
				earlyList = fmt.Sprintf("1: %s\n", earlyList)
			} else {
				earlyList = fmt.Sprintf("1-%d: %s\n", start, earlyList)
			}
		}

		for i, element := range contract.Order[end:len(contract.Order)] {
			var b, ok = contract.Boosters[element]
			if ok {
				if b.BoostState == BoostStateBoosted {
					lateList += fmt.Sprintf("~~%s%s~~ ", b.Mention, farmerstate.GetEggIncName(b.UserID))
				} else {
					lateList += fmt.Sprintf("%s%s(%d) ", b.Mention, farmerstate.GetEggIncName(b.UserID), b.TokensWanted)
				}
				if (end + i + 1) < len(contract.Boosters) {
					lateList += ", "
				}
			}
		}
		if lateList != "" {
			if (end + 1) == len(contract.Order) {
				lateList = fmt.Sprintf("%d: %s", end+1, lateList)
			} else {
				lateList = fmt.Sprintf("%d-%d: %s", end+1, len(contract.Order), lateList)
			}
		}

		orderSubset = contract.Order[start:end]
		offset = start + 1
	}

	outputStr += earlyList

	for i, element := range orderSubset {

		if contract.State != ContractStateSignup {
			prefix = fmt.Sprintf("%2d - ", i+offset)
		}
		var b, ok = contract.Boosters[element]
		if ok {
			var name = b.Mention
			var einame = farmerstate.GetEggIncName(b.UserID)
			if einame != "" {
				name += " " + einame
			}
			var server = ""
			var currentStartTime = fmt.Sprintf(" <t:%d:R> ", b.StartTime.Unix())
			if len(contract.Location) > 1 {
				server = fmt.Sprintf(" (%s) ", contract.EggFarmers[element].GuildName)
			}

			countStr, signupCountStr := getTokenCountString(tokenStr, b.TokensWanted, b.TokensReceived)

			switch b.BoostState {
			case BoostStateUnboosted:
				outputStr += fmt.Sprintf("%s %s%s%s\n", prefix, name, signupCountStr, server)
			case BoostStateTokenTime:
				outputStr += fmt.Sprintf("%s **%s** %s%s%s\n", prefix, name, countStr, currentStartTime, server)
			case BoostStateBoosted:
				outputStr += fmt.Sprintf("%s ~~%s~~  %s %s\n", prefix, name, contract.Boosters[element].Duration.Round(time.Second), server)
			}
		}
	}
	outputStr += lateList

	// Add reaction guidance to the bottom of this list
	if contract.State == ContractStateStarted {
		outputStr += "\n"
		outputStr += "> Active Booster: 🚀 when boosting.\n"
		outputStr += "> Anyone: " + tokenStr + " when sending tokens. ❓ Help.\n"
		if contract.CoopSize != len(contract.Order) {
			outputStr += "> Use pinned message to join this list and set boost " + tokenStr + " wanted.\n"
		}
		//outputStr += "```"
	} else if contract.State == ContractStateWaiting {
		outputStr += "\n"
		outputStr += "> Waiting for other(s) to join...\n"
		outputStr += "> Use pinned message to join this list and set boost " + tokenStr + " wanted.\n"
		outputStr += "```"
		outputStr += "React with 🏁 to end the contract."
		outputStr += "```"
	}
	return outputStr
}

func FindContract(guildID string, channelID string) *Contract {
	// Look for the contract
	for key, element := range Contracts {
		for _, el := range element.Location {
			if el.GuildID == guildID && el.ChannelID == channelID {
				// Found the location of the contract, which one is it?
				return Contracts[key]
			}
		}
	}
	return nil
}

func FindContractByMessageID(channelID string, messageID string) *Contract {
	// Given a
	for _, c := range Contracts {
		for _, loc := range c.Location {
			if slices.Index(loc.MessageIDs, messageID) != -1 {
				return c
			}
		}
	}
	return nil
}

func ChangePingRole(s *discordgo.Session, guildID string, channelID string, userID string, pingRole string) error {
	var contract = FindContract(guildID, channelID)
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

// write the function removeDuplicates which takes an array as an argument and returns the array with all duplicate elements removed.
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
func ChangeContractIDs(s *discordgo.Session, guildID string, channelID string, userID string, contractID string, coopID string) error {
	var contract = FindContract(guildID, channelID)
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
	}
	if coopID != "" {
		contract.CoopID = coopID
	}
	return nil
}

func MoveBooster(s *discordgo.Session, guildID string, channelID string, userID string, boosterName string, boosterPosition int, redraw bool) error {
	var contract = FindContract(guildID, channelID)
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

	fmt.Println("MoveBooster", "GuildID: ", guildID, "ChannelID: ", channelID, "UserID: ", userID, "BoosterName: ", boosterName, "BoosterPosition: ", boosterPosition)

	var boosterIndex = slices.Index(contract.Order, boosterName)
	if boosterIndex == -1 {
		return errors.New("this booster not in contract")
	}

	var newOrder []string
	copyOrder := RemoveIndex(contract.Order, boosterIndex)
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
	contract.Order = newOrder
	if redraw {
		refreshBoostListMessage(s, contract)
	}

	return nil
}

// ChangeCurrentBooster will change the current booster to the specified userID
func ChangeCurrentBooster(s *discordgo.Session, guildID string, channelID string, userID string, newBooster string, redraw bool) error {
	var contract = FindContract(guildID, channelID)
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

func ChangeBoostOrder(s *discordgo.Session, guildID string, channelID string, userID string, boostOrder string, redraw bool) error {
	var contract = FindContract(guildID, channelID)
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
	contract.Order = newOrder
	contract.OrderRevision += 1

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

func AddContractMember(s *discordgo.Session, guildID string, channelID string, operator string, mention string, guest string, order int) error {
	var contract = FindContract(guildID, channelID)
	if contract == nil {
		return errors.New(errorNoContract)
	}
	//contract.mutex.Lock()
	//defer contract.mutex.Unlock()

	if contract.CoopSize == min(len(contract.Order), len(contract.Boosters)) {
		return errors.New(errorContractFull)
	}

	re := regexp.MustCompile(`[\\<>@#&!]`)
	if mention != "" {
		var userID = re.ReplaceAllString(mention, "")

		for i := range contract.Order {
			if userID == contract.Order[i] {
				return errors.New(errorUserInContract)
			}
		}

		var u, err = s.User(userID)
		if err != nil {
			return errors.New(errorNoFarmer)
		}
		if u.Bot {
			return errors.New(errorBot)
		}
		var farmer, fe = AddFarmerToContract(s, contract, guildID, channelID, u.ID, order)
		if fe == nil {
			// Need to rest the farmer reaction count when added this way
			farmer.Reactions = 0
		}
		for _, loc := range contract.Location {
			var listStr = "Boost"
			if contract.State == ContractStateSignup {
				listStr = "Sign-up"
			}
			var str = fmt.Sprintf("%s was added to the %s List by %s", u.Mention(), listStr, operator)

			var data discordgo.MessageSend
			var am discordgo.MessageAllowedMentions
			data.Content = str
			data.AllowedMentions = &am

			s.ChannelMessageSendComplex(loc.ChannelID, &data)
		}
	}

	if guest != "" {
		for i := range contract.Order {
			if guest == contract.Order[i] {
				return errors.New(errorUserInContract)
			}
		}

		var farmer, fe = AddFarmerToContract(s, contract, guildID, channelID, guest, order)
		if fe == nil {
			// Need to rest the farmer reaction count when added this way
			farmer.Reactions = 0
		}
		for _, loc := range contract.Location {
			var listStr = "Boost"
			if contract.State == ContractStateSignup {
				listStr = "Sign-up"
			}
			var str = fmt.Sprintf("%s was added to the %s List by %s", guest, listStr, operator)
			s.ChannelMessageSend(loc.ChannelID, str)
		}
	}

	return nil
}

func AddFarmerToContract(s *discordgo.Session, contract *Contract, guildID string, channelID string, userID string, order int) (*EggFarmer, error) {
	fmt.Println("AddFarmerToContract", "GuildID: ", guildID, "ChannelID: ", channelID, "UserID: ", userID, "Order: ", order)
	var farmer = contract.EggFarmers[userID]
	if farmer == nil {
		// New Farmer
		farmer = new(EggFarmer)
		farmer.Register = time.Now()
		farmer.Ping = false
		farmer.Reactions = 0
		farmer.UserID = userID
		farmer.GuildID = guildID
		ch, errCh := s.Channel(channelID)
		if errCh != nil {
			fmt.Println(channelID, errCh)
			farmer.ChannelName = "Unknown"
		} else {
			farmer.ChannelName = ch.Name
		}

		g, errG := s.Guild(guildID)
		if errG != nil {
			fmt.Println(guildID, errG)
			farmer.GuildName = "Unknown"
		} else {
			farmer.GuildName = g.Name
		}

		gm, errGM := s.GuildMember(guildID, userID)
		if gm != nil {
			farmer.Username = gm.User.Username
			farmer.Nick = gm.Nick
			farmer.Unique = gm.User.String()
		} else if errGM != nil {
			fmt.Println(guildID, userID, errGM)
			farmer.Username = userID
			farmer.Nick = userID
			farmer.Unique = userID
		}
		contract.EggFarmers[userID] = farmer
	}

	var b = contract.Boosters[userID]
	if b == nil {
		// New Farmer - add them to boost list
		var b = new(Booster)
		b.UserID = farmer.UserID
		var user, err = s.User(userID)
		b.BoostState = BoostStateUnboosted
		b.TokensWanted = farmerstate.GetTokens(b.UserID)
		if b.TokensWanted <= 0 {
			b.TokensWanted = 8
		}
		if err == nil {
			b.Name = user.Username
			b.Mention = user.Mention()
		} else {
			b.Name = userID
			b.Mention = userID
		}
		var member, gmErr = s.GuildMember(guildID, userID)
		if gmErr == nil && member.Nick != "" {
			b.Name = member.Nick
			b.Mention = member.Mention()
		}

		// Check if within the start period of a contract
		if contract.State != ContractStateSignup {
			if order == ContractOrderTimeBased || order == ContractOrderFair || order == ContractOrderRandom {
				var timeSinceStart = time.Since(contract.StartTime)
				var minutesSinceStart = int(timeSinceStart.Minutes())
				if minutesSinceStart <= BoostOrderTimeThreshold {
					order = contract.BoostOrder
				} else {
					contract.BoostOrder = ContractOrderSignup
					order = ContractOrderSignup
				}
			}
		}

		if !userInContract(contract, farmer.UserID) {
			contract.Boosters[farmer.UserID] = b
			// If contract hasn't started add booster to the end
			// or if contract is on the last booster already
			if contract.State == ContractStateSignup || contract.State == ContractStateWaiting || order == ContractOrderSignup {
				contract.Order = append(contract.Order, farmer.UserID)
				if contract.State == ContractStateWaiting {
					contract.BoostPosition = len(contract.Order) - 1
				}
			} else {
				copyOrder := make([]string, len(contract.Order))
				copy(copyOrder, contract.Order)
				copyOrder = append(copyOrder, farmer.UserID)

				newOrder := farmerstate.GetOrderHistory(copyOrder, 5)

				// find index of farmer.UserID in newOrder
				var index = slices.Index(newOrder, farmer.UserID)
				if contract.BoostPosition >= index {
					index = contract.BoostPosition + 1
				}
				contract.Order = insert(contract.Order, index, farmer.UserID)
			}
			contract.OrderRevision += 1
		}
		contract.RegisteredNum = len(contract.Boosters)

		if contract.State == ContractStateWaiting {
			// Reactivate the contract
			// Set the newly added booster as boosting
			contract.State = ContractStateStarted
			b.StartTime = time.Now()
			b.BoostState = BoostStateTokenTime
			contract.BoostPosition = len(contract.Order) - 1
			// for all locations delete the signup list message and send the boost list message
			//for _, loc := range contract.Location {
			//	s.ChannelMessageDelete(loc.ChannelID, loc.ListMsgID)
			//}
			sendNextNotification(s, contract, true)
			return farmer, nil
		}
	}
	refreshBoostListMessage(s, contract)
	return farmer, nil
}

func creatorOfContract(c *Contract, u string) bool {
	for _, el := range c.CreatorID {
		if el == u {
			return true
		}
	}
	return false
}

func userInContract(c *Contract, u string) bool {

	if len(c.Boosters) != len(c.Order) {
		// Exists in Boosters/Order but not in other
		for k := range c.Boosters {
			if !slices.Contains(c.Order, k) {
				c.Order = append(c.Order, k)
			}
		}
	}

	for _, el := range c.Order {
		if el == u && c.Boosters[u] != nil {
			return true
		}
	}

	return false
}

func ReactionAdd(s *discordgo.Session, r *discordgo.MessageReaction) string {
	// Find the message
	var msg, err = s.ChannelMessage(r.ChannelID, r.MessageID)
	if err != nil {
		return ""
	}
	redraw := false
	emojiName := r.Emoji.Name

	//var contract = FindContract(r.GuildID, r.ChannelID)
	//if contract == nil {
	var contract = FindContractByMessageID(r.ChannelID, r.MessageID)
	if contract == nil {
		return ""
	}
	//}
	//contract.mutex.Lock()
	defer saveData(Contracts)
	if userInContract(contract, r.UserID) || creatorOfContract(contract, r.UserID) {

		// if contract state is waiting and the reaction is a 🏁 finish the contract
		if contract.State == ContractStateWaiting && r.Emoji.Name == "🏁" {
			contract.State = ContractStateCompleted
			contract.EndTime = time.Now()
			sendNextNotification(s, contract, true)
			return ""
		}

		if contract.State != ContractStateSignup && contract.BoostPosition < len(contract.Order) {

			// If Rocket reaction on Boost List, only that boosting user can apply a reaction
			if r.Emoji.Name == "🚀" && contract.State == ContractStateStarted {
				var votingElection = (msg.Reactions[0].Count - 1) >= 2

				if r.UserID == contract.Order[contract.BoostPosition] || votingElection || creatorOfContract(contract, r.UserID) {
					//contract.mutex.Unlock()
					Boosting(s, r.GuildID, r.ChannelID)
				}
				return ""
			}

			// Reaction for current booster to change places
			if r.UserID == contract.Order[contract.BoostPosition] || creatorOfContract(contract, r.UserID) {
				if (contract.BoostPosition + 1) < len(contract.Order) {
					if r.Emoji.Name == "🔃" {
						//contract.mutex.Unlock()
						SkipBooster(s, r.GuildID, r.ChannelID, "")
						return ""
					}
				}
			}

			{
				// Reaction to jump to end
				if r.Emoji.Name == "⤵️" {
					//contract.mutex.Unlock()
					var uid = r.UserID // using a variable here for debugging
					if contract.Boosters[uid].BoostState == BoostStateTokenTime {
						currentBoosterPosition := findNextBooster(contract)
						MoveBooster(s, r.GuildID, r.ChannelID, contract.CreatorID[0], uid, len(contract.Order), currentBoosterPosition == -1)
						if currentBoosterPosition != -1 {
							ChangeCurrentBooster(s, r.GuildID, r.ChannelID, contract.CreatorID[0], contract.Order[currentBoosterPosition], true)
							return ""
						}
					} else {
						MoveBooster(s, r.GuildID, r.ChannelID, contract.CreatorID[0], uid, len(contract.Order), true)
					}
				}
				// Reaction to indicate you need to go now
				if r.Emoji.Name == "🚽" {
					SkipBooster(s, r.GuildID, r.ChannelID, r.UserID)
					return ""
				}
			}

			if contract.State == ContractStateWaiting && r.Emoji.Name == "🔃" {
				contract.State = ContractStateCompleted
				contract.EndTime = time.Now()
				sendNextNotification(s, contract, true)
				return ""
			}
		}
		if r.Emoji.Name == "🚽" {
			SkipBooster(s, r.GuildID, r.ChannelID, r.UserID)
			return "" //"!gonow"
		}

		if strings.ToLower(r.Emoji.Name) == "token" {
			if contract.BoostPosition < len(contract.Order) {
				contract.Boosters[contract.Order[contract.BoostPosition]].TokensReceived += 1
				emojiName = r.Emoji.Name + ":" + r.Emoji.ID

				var b = contract.Boosters[contract.Order[contract.BoostPosition]]
				if b.TokensReceived == b.TokensWanted && b.UserID == b.Name {
					// Guest farmer auto boosts
					Boosting(s, r.GuildID, r.ChannelID)
				}

				redraw = true
			}
		}
	} else {
		// Custon token reaction from user not in contract
		if strings.ToLower(r.Emoji.Name) == "token" {
			emojiName = r.Emoji.Name + ":" + r.Emoji.ID
			redraw = true
		}
	}

	// Remove extra added emoji
	err = s.MessageReactionRemove(r.ChannelID, r.MessageID, emojiName, r.UserID)
	if err != nil {
		fmt.Println(err, emojiName)
	}

	/*
		// case insensitive compare for token emoji
		if r.Emoji.Name == "➕" && r.UserID == contract.Order[contract.BoostPosition] {
			// Add a token to the current booster
			contract.Boosters[contract.Order[contract.BoostPosition]].TokensWanted += 1
			redraw = true
		}
		if r.Emoji.Name == "➖" && r.UserID == contract.Order[contract.BoostPosition] {
			// Add a token to the current booster
			contract.Boosters[contract.Order[contract.BoostPosition]].TokensWanted -= 1
			redraw = true
		}
	*/

	if redraw {
		refreshBoostListMessage(s, contract)
	}

	if r.Emoji.Name == "❓" {
		for _, loc := range contract.Location {
			outputStr := "## Boost Bot Icon Meanings\n\n"
			outputStr += "See 📌 message to join the contract and select your number of boost tokens.\n"
			outputStr += "Active booster reaction of 🚀 to when spending tokens to boost. Multiple 🚀 votes by others in the contract will also indicate a boost.\n"
			outputStr += "Farmers react with " + loc.TokenStr + " when sending tokens.\n"
			//outputStr += "Active Booster can react with ➕ or ➖ to adjust number of tokens needed.\n"
			outputStr += "Active booster reaction of 🔃 to exchange position with the next booster.\n"
			outputStr += "Reaction of ⤵️ to move yourself to last in the current boost order.\n"
			outputStr += "Anyone can add a 🚽 reaction to express your urgency to boost next.\n"
			s.ChannelMessageSend(loc.ChannelID, outputStr)
		}
	}

	return ""
}

// findNextBooster returns the index of the next booster that needs to boost
func findNextBooster(contract *Contract) int {
	for i := 0; i < len(contract.Order); i++ {
		if contract.Boosters[contract.Order[i]].BoostState == BoostStateUnboosted || contract.Boosters[contract.Order[i]].BoostState == BoostStateTokenTime {
			return i
		}
	}
	return -1
}

func JoinContract(s *discordgo.Session, guildID string, channelID string, userID string, bell bool) error {
	var err error

	fmt.Println("JoinContract", "GuildID: ", guildID, "ChannelID: ", channelID, "UserID: ", userID, "Bell: ", bell)

	var contract = FindContract(guildID, channelID)
	if contract == nil {
		return errors.New(errorNoContract)
	}

	if contract.Boosters[userID] == nil {
		if contract.CoopSize == min(len(contract.Order), len(contract.Boosters)) {
			return errors.New(errorContractFull)
		}

		// Wait here until we get our lock
		contract.mutex.Lock()
		_, err = AddFarmerToContract(s, contract, guildID, channelID, userID, contract.BoostOrder)

		contract.mutex.Unlock()
		if err != nil {
			return err
		}
	}

	var farmer = contract.EggFarmers[userID]
	farmer.Ping = bell

	if bell {
		u, _ := s.UserChannelCreate(farmer.UserID)
		var str = fmt.Sprintf("Boost notifications will be sent for %s.", contract.ContractHash)
		_, err := s.ChannelMessageSend(u.ID, str)
		if err != nil {
			panic(err)
		}

	}

	return nil
}

func RemoveIndex(s []string, index int) []string {
	return append(s[:index], s[index+1:]...)
}

func removeContractBoosterByContract(s *discordgo.Session, contract *Contract, offset int) bool {
	if offset > len(contract.Boosters) {
		return false
	}
	var index = offset - 1 // Index is 0 based

	var currentBooster = ""

	// Save current booster position
	if contract.State != ContractStateWaiting {
		currentBooster = contract.Order[contract.BoostPosition]
	}

	var activeBooster, ok = contract.Boosters[contract.Order[index]]
	if ok && contract.State != ContractStateSignup {
		var activeBoosterState = activeBooster.BoostState
		var userID = contract.Order[index]
		contract.Order = RemoveIndex(contract.Order, index)
		contract.OrderRevision += 1
		delete(contract.Boosters, userID)

		// Make sure we retain our current booster
		for i := range contract.Order {
			if contract.Order[i] == currentBooster {
				contract.BoostPosition = i
				break
			}
		}

		// Active Booster is leaving contract.
		if contract.State == ContractStateWaiting {
			contract.BoostPosition = len(contract.Order)
			sendNextNotification(s, contract, true)
		} else if contract.State == ContractStateStarted && contract.BoostPosition == len(contract.Order) {
			// set contract to waiting
			contract.State = ContractStateWaiting
			sendNextNotification(s, contract, true)
		} else if (activeBoosterState == BoostStateTokenTime) && len(contract.Order) > index {
			contract.Boosters[contract.Order[index]].BoostState = BoostStateTokenTime
			contract.Boosters[contract.Order[index]].StartTime = time.Now()
			sendNextNotification(s, contract, true)
		}
	} else {
		delete(contract.Boosters, contract.Order[index])

		contract.Order = RemoveIndex(contract.Order, index)
		contract.OrderRevision += 1
		//remove userID from Boosters
		refreshBoostListMessage(s, contract)

	}
	return true
}

func Unboost(s *discordgo.Session, guildID string, channelID string, mention string) error {
	var contract = FindContract(guildID, channelID)
	if contract == nil {
		return errors.New(errorNoContract)
	}
	//contract.mutex.Lock()
	//defer contract.mutex.Unlock()

	if contract.CoopSize == 0 {
		return errors.New(errorContractEmpty)
	}

	re := regexp.MustCompile(`[\\<>@#&!]`)
	var userID = re.ReplaceAllString(mention, "")
	userID = strings.TrimSpace(userID)

	var u, _ = s.User(userID)
	if u != nil {
		if u.Bot {
			return errors.New(errorBot)
		}
	}

	if contract.Boosters[userID] == nil {
		return errors.New(errorUserNotInContract)
	}

	if contract.State == ContractStateWaiting {
		contract.Boosters[userID].BoostState = BoostStateTokenTime
		contract.State = ContractStateStarted
		// set BoostPosition to unboosted user
		for i := range contract.Order {
			if contract.Order[i] == userID {
				contract.BoostPosition = i
				break
			}
		}

		sendNextNotification(s, contract, true)
	} else {
		contract.Boosters[userID].BoostState = BoostStateUnboosted
		refreshBoostListMessage(s, contract)
	}
	return nil
}

func RemoveContractBoosterByMention(s *discordgo.Session, guildID string, channelID string, operator string, mention string) error {
	fmt.Println("RemoveContractBoosterByMention", "GuildID: ", guildID, "ChannelID: ", channelID, "Operator: ", operator, "Mention: ", mention)
	var contract = FindContract(guildID, channelID)
	if contract == nil {
		return errors.New(errorNoContract)
	}
	//contract.mutex.Lock()
	//defer contract.mutex.Unlock()

	if contract.CoopSize == 0 {
		return errors.New(errorContractEmpty)
	}

	re := regexp.MustCompile(`[\\<>@#&!]`)
	var userID = re.ReplaceAllString(mention, "")

	var u, _ = s.User(userID)
	if u != nil {
		if u.Bot {
			return errors.New(errorBot)
		}
	}

	for i := range contract.Order {
		if contract.Order[i] == userID {
			if removeContractBoosterByContract(s, contract, i+1) {
				contract.RegisteredNum = len(contract.Boosters)
			}
			break
		}
	}

	// Edit the boost List in place
	if contract.BoostPosition != len(contract.Order) {
		for _, loc := range contract.Location {
			outputStr := DrawBoostList(s, contract, loc.TokenStr)
			msg, err := s.ChannelMessageEdit(loc.ChannelID, loc.ListMsgID, outputStr)
			if err == nil {
				loc.ListMsgID = msg.ID
			} else {
				s.ChannelMessageDelete(loc.ChannelID, loc.ListMsgID)
				msg, _ := s.ChannelMessageSend(loc.ChannelID, outputStr)
				SetListMessageID(contract, loc.ChannelID, msg.ID)
			}
		}
	}

	return nil
}

func RemoveContractBooster(s *discordgo.Session, guildID string, channelID string, index int) error {
	var contract = FindContract(guildID, channelID)

	if contract == nil {
		return errors.New(errorNoContract)
	}

	//contract.mutex.Lock()
	//defer contract.mutex.Unlock()

	if len(contract.Order) == 0 {
		return errors.New(errorContractEmpty)
	}
	if removeContractBoosterByContract(s, contract, index) {
		contract.RegisteredNum = len(contract.Boosters)
	}

	// Remove the Boost List and thoen redisplay it
	refreshBoostListMessage(s, contract)
	return nil
}

func ReactionRemove(s *discordgo.Session, r *discordgo.MessageReaction) {
	var _, err = s.ChannelMessage(r.ChannelID, r.MessageID)
	if err != nil {
		return
	}

	var contract = FindContractByMessageID(r.ChannelID, r.MessageID)
	if contract == nil {
		return
	}

	//contract.mutex.Lock()
	//defer contract.mutex.Unlock()
	defer saveData(Contracts)

	var farmer = contract.EggFarmers[r.UserID]
	if farmer == nil {
		return
	}

	if !userInContract(contract, r.UserID) {
		return
	}
}

func StartContractBoosting(s *discordgo.Session, guildID string, channelID string, userID string) error {
	var contract = FindContract(guildID, channelID)
	if contract == nil {
		return errors.New(errorNoContract)
	}

	//contract.mutex.Lock()
	//defer contract.mutex.Unlock()

	if len(contract.Boosters) == 0 {
		return errors.New(errorContractEmpty)
	}

	if contract.State != ContractStateSignup {
		return errors.New(errorContractAlreadyStarted)
	}

	if !creatorOfContract(contract, userID) {
		return errors.New(errorNotContractCreator)
	}

	reorderBoosters(contract)

	contract.BoostPosition = 0
	contract.State = ContractStateStarted
	contract.StartTime = time.Now()

	contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = BoostStateTokenTime
	contract.Boosters[contract.Order[contract.BoostPosition]].StartTime = time.Now()

	sendNextNotification(s, contract, true)

	return nil
}

// RedrawBoostList will move the boost message to the bottom of the channel
func RedrawBoostList(s *discordgo.Session, guildID string, channelID string) error {
	var contract = FindContract(guildID, channelID)
	if contract == nil {
		return errors.New(errorNoContract)
	}

	if contract.State == ContractStateSignup {
		return errors.New(errorContractNotStarted)
	}

	// Edit the boost list in place
	for _, loc := range contract.Location {
		if loc.GuildID == guildID && loc.ChannelID == channelID {
			s.ChannelMessageDelete(loc.ChannelID, loc.ListMsgID)
			var data discordgo.MessageSend
			var am discordgo.MessageAllowedMentions
			data.Content = DrawBoostList(s, contract, loc.TokenStr)
			data.AllowedMentions = &am
			msg, err := s.ChannelMessageSendComplex(loc.ChannelID, &data)
			if err == nil {
				SetListMessageID(contract, loc.ChannelID, msg.ID)
			}
			addContractReactions(s, contract, loc.ChannelID, msg.ID, loc.TokenReactionStr)
		}
	}
	return nil
}

func refreshBoostListMessage(s *discordgo.Session, contract *Contract) {
	// Edit the boost list in place
	for _, loc := range contract.Location {
		msg, err := s.ChannelMessageEdit(loc.ChannelID, loc.ListMsgID, DrawBoostList(s, contract, loc.TokenStr))
		if err == nil {
			// This is an edit, it should be the same
			loc.ListMsgID = msg.ID
		}
	}
}

func addContractReactions(s *discordgo.Session, contract *Contract, channelID string, messageID string, tokenStr string) {
	if contract.State == ContractStateStarted {
		s.MessageReactionAdd(channelID, messageID, "🚀")             // Booster
		err := s.MessageReactionAdd(channelID, messageID, tokenStr) // Token Reaction
		if err != nil {
			fmt.Print(err.Error())
		}
		s.MessageReactionAdd(channelID, messageID, "🔃")  // Swap
		s.MessageReactionAdd(channelID, messageID, "⤵️") // Last
	}
	if contract.State == ContractStateWaiting {
		s.MessageReactionAdd(channelID, messageID, "🏁") // Finish
	}
	s.MessageReactionAdd(channelID, messageID, "❓") // Finish
}

func sendNextNotification(s *discordgo.Session, contract *Contract, pingUsers bool) {
	// Start boosting contract
	for _, loc := range contract.Location {
		var msg *discordgo.Message
		var err error

		if contract.State == ContractStateSignup {
			msg, err = s.ChannelMessageEdit(loc.ChannelID, loc.ListMsgID, DrawBoostList(s, contract, loc.TokenStr))
			if err != nil {
				fmt.Println("Unable to send this message")
			}
		} else {
			// Unpin message once the contract is completed
			if contract.State == ContractStateCompleted {
				s.ChannelMessageUnpin(loc.ChannelID, loc.ReactionID)
			}
			s.ChannelMessageDelete(loc.ChannelID, loc.ListMsgID)

			// Compose the message without a Ping
			var data discordgo.MessageSend
			var am discordgo.MessageAllowedMentions
			data.Content = DrawBoostList(s, contract, loc.TokenStr)
			data.AllowedMentions = &am
			msg, err = s.ChannelMessageSendComplex(loc.ChannelID, &data)
			if err == nil {
				SetListMessageID(contract, loc.ChannelID, msg.ID)
			}
		}
		if err != nil {
			fmt.Println("Unable to resend message.")
		}
		var str = ""

		if contract.State != ContractStateCompleted {
			addContractReactions(s, contract, loc.ChannelID, msg.ID, loc.TokenReactionStr)
			if pingUsers {
				if contract.State == ContractStateStarted {
					var einame = farmerstate.GetEggIncName(contract.Order[contract.BoostPosition])
					if einame != "" {
						einame += " " // Add a space to this
					}
					name := einame + contract.Boosters[contract.Order[contract.BoostPosition]].Mention
					str = fmt.Sprintf(loc.ChannelPing+" send tokens to %s", name)
				} else {
					str = fmt.Sprintf(loc.ChannelPing + " contract boosting complete. Hold your tokens for late joining farmers.")
				}
			}
		} else {
			t1 := contract.EndTime
			t2 := contract.StartTime
			duration := t1.Sub(t2)
			str = fmt.Sprintf(loc.ChannelPing+" contract boosting complete in %s ", duration.Round(time.Second))
		}

		// Sending the update message
		s.ChannelMessageSend(loc.ChannelID, str)
		//if err == nil {
		//SetListMessageID(contract, loc.ChannelID, msg.ID)
		//}

	}
	if pingUsers {
		notifyBellBoosters(s, contract)
	}
	if contract.State == ContractStateCompleted {
		FinishContract(s, contract)
	}

}

// BoostCommand will trigger a contract boost of a user
func BoostCommand(s *discordgo.Session, guildID string, channelID string, userID string) error {
	var contract = FindContract(guildID, channelID)

	if contract == nil {
		return errors.New(errorNoContract)
	}

	//contract.mutex.Lock()
	//defer contract.mutex.Unlock()

	if contract.State == ContractStateSignup {
		return errors.New(errorContractEmpty)
	}

	if userID == contract.Order[contract.BoostPosition] {
		// User is using /boost command instead of reaction
		Boosting(s, guildID, channelID)
	} else {
		for i := range contract.Order {
			if contract.Order[i] == userID {
				if contract.Boosters[contract.Order[i]].BoostState == BoostStateBoosted {
					return errors.New(errorAlreadyBoosted)
				}
				// Mark user as complete
				// Taking start time from current booster start time
				contract.Boosters[contract.Order[i]].BoostState = BoostStateBoosted
				if contract.Boosters[contract.Order[i]].StartTime.IsZero() {
					// Keep existing start time if they already boosted
					contract.Boosters[contract.Order[i]].StartTime = contract.Boosters[contract.Order[contract.BoostPosition-1]].StartTime
				}
				contract.Boosters[contract.Order[i]].EndTime = time.Now()
				contract.Boosters[contract.Order[i]].Duration = time.Since(contract.Boosters[contract.Order[i]].StartTime)
				sendNextNotification(s, contract, false)
				return nil
			}
		}
		return nil
	}

	return nil
}

// Boosting will mark a as boosted and advance to the next in the list
func Boosting(s *discordgo.Session, guildID string, channelID string) error {
	var contract = FindContract(guildID, channelID)
	if contract == nil {
		return errors.New(errorNoContract)
	}

	//contract.mutex.Lock()
	//defer contract.mutex.Unlock()

	if contract.State == ContractStateSignup {
		return errors.New(errorContractNotStarted)
	}
	contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = BoostStateBoosted
	contract.Boosters[contract.Order[contract.BoostPosition]].EndTime = time.Now()
	contract.Boosters[contract.Order[contract.BoostPosition]].Duration = time.Since(contract.Boosters[contract.Order[contract.BoostPosition]].StartTime)

	// Advance past any that have already boosted
	// Set boost order to last spot so end of contract handling can occur
	// if nobody left unboosted
	contract.BoostPosition = len(contract.Order)

	// loop through all contract.Order until we find a non-boosted user
	// Want to prevent two TokenTime boosters
	foundActiveBooster := false
	var firstUnboosted = -1
	for i := range contract.Order {
		if contract.Boosters[contract.Order[i]].BoostState == BoostStateTokenTime {
			contract.BoostPosition = i
			foundActiveBooster = true
		} else if foundActiveBooster && contract.Boosters[contract.Order[i]].BoostState == BoostStateTokenTime {
			contract.Boosters[contract.Order[i]].BoostState = BoostStateUnboosted
		}
		if firstUnboosted == -1 && contract.Boosters[contract.Order[i]].BoostState == BoostStateUnboosted {
			firstUnboosted = i
		}
	}

	if !foundActiveBooster && firstUnboosted != -1 {
		contract.BoostPosition = firstUnboosted
	}

	if contract.BoostPosition == contract.CoopSize {
		contract.State = ContractStateCompleted // Finished
		contract.EndTime = time.Now()
	} else if contract.BoostPosition == len(contract.Order) {
		contract.State = ContractStateWaiting // There could be more boosters joining later
	} else {
		contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = BoostStateTokenTime
		contract.Boosters[contract.Order[contract.BoostPosition]].StartTime = time.Now()
		contract.Boosters[contract.Order[contract.BoostPosition]].TokensReceived = 0 // reset these
	}

	sendNextNotification(s, contract, true)

	return nil
}

// 0 <= index <= len(a)
func insert(a []string, index int, value string) []string {
	if len(a) == index { // nil or empty slice or after last element
		return append(a, value)
	}
	a = append(a[:index+1], a[index:]...) // index < len(a)
	a[index] = value
	return a
}

func SkipBooster(s *discordgo.Session, guildID string, channelID string, userID string) error {
	var boosterSwap = false
	var contract = FindContract(guildID, channelID)
	if contract == nil {
		return errors.New(errorNoContract)
	}

	//contract.mutex.Lock()
	//defer contract.mutex.Unlock()

	if contract.State == ContractStateSignup {
		return errors.New(errorNotStarted)
	}

	var selectedUser = contract.BoostPosition

	if userID != "" {
		for i := range contract.Order {
			if contract.Order[i] == userID {
				selectedUser = i
				if contract.Boosters[contract.Order[i]].BoostState == BoostStateBoosted {
					return nil
				}
				break
			}
		}
	} else {
		boosterSwap = true
	}

	if selectedUser == contract.BoostPosition {
		contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = BoostStateUnboosted
		var skipped = contract.Order[contract.BoostPosition]

		if boosterSwap {
			contract.Order[contract.BoostPosition] = contract.Order[contract.BoostPosition+1]
			contract.Order[contract.BoostPosition+1] = skipped

		} else {
			contract.Order = RemoveIndex(contract.Order, contract.BoostPosition)
			contract.Order = append(contract.Order, skipped)
		}

		if contract.BoostPosition == contract.CoopSize {
			contract.State = ContractStateCompleted // Finished
			contract.EndTime = time.Now()
		} else if contract.BoostPosition == len(contract.Boosters) {
			contract.State = ContractStateWaiting
		} else {
			contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = BoostStateTokenTime
			contract.Boosters[contract.Order[contract.BoostPosition]].StartTime = time.Now()
		}
	} else {
		var skipped = contract.Order[selectedUser]
		contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = BoostStateUnboosted
		contract.Order = RemoveIndex(contract.Order, selectedUser)
		contract.Order = insert(contract.Order, contract.BoostPosition, skipped)
		contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = BoostStateTokenTime
		contract.Boosters[contract.Order[contract.BoostPosition]].StartTime = time.Now()
	}
	contract.OrderRevision += 1

	sendNextNotification(s, contract, true)

	return nil
}

func notifyBellBoosters(s *discordgo.Session, contract *Contract) {
	for i := range contract.Boosters {
		var farmer = contract.EggFarmers[contract.Boosters[i].UserID]
		if farmer.Ping {
			u, _ := s.UserChannelCreate(farmer.UserID)
			var str = ""
			if contract.State == ContractStateCompleted {
				t1 := contract.EndTime
				t2 := contract.StartTime
				duration := t1.Sub(t2)
				str = fmt.Sprintf("%s: Contract Boosting Completed in %s ", farmer.ChannelName, duration.Round(time.Second))
			} else if contract.State == ContractStateWaiting {
				t1 := contract.EndTime
				t2 := contract.StartTime
				duration := t1.Sub(t2)
				str = fmt.Sprintf("%s: Boosting Completed in %s. Still %d spots in the contract. ", farmer.ChannelName, duration.Round(time.Second), contract.CoopSize-len(contract.Boosters))
			} else {
				str = fmt.Sprintf("%s: Send Boost Tokens to %s", farmer.ChannelName, contract.Boosters[contract.Order[contract.BoostPosition]].Name)
			}
			_, err := s.ChannelMessageSend(u.ID, str)
			if err != nil {
				panic(err)
			}
		}
	}

}

// FinishContract is called only when the contract is complete
func FinishContract(s *discordgo.Session, contract *Contract) error {
	// Don't delete the final boost message
	for _, loc := range contract.Location {
		loc.ListMsgID = ""
	}
	farmerstate.SetOrderPercentileAll(contract.Order, contract.CoopSize)
	DeleteContract(s, contract.Location[0].GuildID, contract.Location[0].ChannelID)
	return nil
}

func reorderBoosters(contract *Contract) {
	switch contract.BoostOrder {
	case ContractOrderSignup:
		// Join Order
	case ContractOrderReverse:
		// Reverse Order
		for i, j := 0, len(contract.Order)-1; i < j; i, j = i+1, j-1 {
			contract.Order[i], contract.Order[j] = contract.Order[j], contract.Order[i] //reverse the slice
		}
	case ContractOrderRandom:
		rand.Shuffle(len(contract.Order), func(i, j int) {
			contract.Order[i], contract.Order[j] = contract.Order[j], contract.Order[i]
		})
	case ContractOrderFair:
		newOrder := farmerstate.GetOrderHistory(contract.Order, 5)
		contract.Order = newOrder
	}
}

// AdvancedTransform for storing KV pairs
func AdvancedTransform(key string) *diskv.PathKey {
	path := strings.Split(key, "/")
	last := len(path) - 1
	return &diskv.PathKey{
		Path:     path[:last],
		FileName: path[last] + ".json",
	}
}

// InverseTransform for storing KV pairs
func InverseTransform(pathKey *diskv.PathKey) (key string) {
	txt := pathKey.FileName[len(pathKey.FileName)-4:]
	if txt != ".json" {
		panic("Invalid file found in storage folder!")
	}
	return strings.Join(pathKey.Path, "/") + pathKey.FileName[:len(pathKey.FileName)-4]
}

func saveData(c map[string]*Contract) error {
	//diskmutex.Lock()
	b, _ := json.Marshal(c)
	DataStore.Write("EggsBackup", b)

	//diskmutex.Unlock()
	return nil
}

func saveEndData(c *Contract) error {
	//diskmutex.Lock()
	b, _ := json.Marshal(c)
	DataStore.Write(c.ContractHash, b)
	//diskmutex.Unlock()
	return nil
}

func loadData() (map[string]*Contract, error) {
	//diskmutex.Lock()
	var c map[string]*Contract
	b, err := DataStore.Read("EggsBackup")
	if err != nil {
		return c, err
	}
	json.Unmarshal(b, &c)
	//diskmutex.Unlock()

	return c, nil
}

/*
func Cluck(s *discordgo.Session, guildID string, channelID string, userID string, msg string) error {
	var contract = FindContract(guildID, channelID)
	if contract == nil {
		return errors.New(errorNoContract)
	}
	var booster = contract.Boosters[userID]
	if booster == nil {
		return errors.New(errorNoFarmer)
	}
	var farmer = contract.EggFarmers[userID]
	if farmer == nil {
		return errors.New(errorNoFarmer)
	}

	// Save every cross channel message
	append(farmer.Cluck, msg)

	for _, el := range contract.Location {

		s.ChannelMessageSend(el.ChannelID, fmt.Sprintf("%s clucks: %s", booster.Name, msg))
		s.ChannelMessageDelete(el.ChannelID, el.ListMsgID)
		s.ChannelMessageDelete(el.ChannelID, el.ReactionID)
	}
	return nil
}
*/
