package boost

import (
	"errors"
	"fmt"
	"log"
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
	emutil "github.com/post04/discordgo-emoji-util"
	"github.com/rs/xid"
)

//var usermutex sync.Mutex
//var diskmutex sync.Mutex

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

// Constnts for the contract
const (
	ContractOrderSignup    = 0 // Signup order
	ContractOrderReverse   = 1 // Reverse order
	ContractOrderRandom    = 2 // Randomized when the contract starts. After 20 minutes the order changes to Sign-up.
	ContractOrderFair      = 3 // Fair based on position percentile of each farmers last 5 contracts. Those with no history use 50th percentile
	ContractOrderTimeBased = 4 // Time based order

	ContractStateSignup    = 0 // Contract is in signup phase
	ContractStateStarted   = 1 // Contract is started
	ContractStateWaiting   = 2 // Waiting for other(s) to join
	ContractStateCompleted = 3 // Contract is completed
	ContractStateArchive   = 4 // Contract is ready to archive

	BoostStateUnboosted = 0 // Unboosted
	BoostStateTokenTime = 1 // TokenTime or turn to receive tokens
	BoostStateBoosted   = 2 // Boosted

	BoostOrderTimeThreshold = 20 // minutes to switch from random or fair to signup

	SpeedrunStateNone     = 0 // No speedrun
	SpeedrunStateSignup   = 1 // Signup Speedrun
	SpeedrunStateCRT      = 2 // CRT Speedrun
	SpeedrunStateBoosting = 3 // Boosting Speedrun
	SpeedrunStatePost     = 4 // Post Speedrun
	SpeedrunStateComplete = 5 // Speedrun Complete

	SpeedrunFirstLeg   = 0
	SpeedrunMiddleLegs = 1
	SpeedrunFinalLeg   = 2

	SpeedrunStyleWonky   = 0
	SpeedrunStyleFastrun = 1

	SinkBoostFirst = 0 // First position
	SinkBoostLast  = 1 // Last position

)

// EggFarmer was intended to be more for across contract tracking of users
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

// Booster holds the data for each booster within a Contract
type Booster struct {
	UserID            string // Egg Farmer
	Name              string
	BoostState        int           // Indicates if current booster
	Mention           string        // String which mentions user
	TokenSentTime     []time.Time   // time of each token sent
	TokenSentName     []string      // recipient of each token
	TokenReceivedTime []time.Time   // time of each received token
	TokenReceivedName []string      // name of sender of received tokens
	TokensReceived    int           // indicate number of boost tokens
	TokensWanted      int           // indicate number of boost tokens
	TokensFarmedTime  []time.Time   // When each token was farmed
	StartTime         time.Time     // Time Farmer started boost turn
	EndTime           time.Time     // Time Farmer ended boost turn
	Duration          time.Duration // Duration of boost
}

// LocationData holds server specific Data for a contract
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

// Contract is the main struct for each contract
type Contract struct {
	ContractHash string // ContractID-CoopID
	Location     []*LocationData
	CreatorID    []string // Slice of creators
	//SignupMsgID    map[string]string // Message ID for the Signup Message
	ContractID     string // Contract ID
	CoopID         string // CoopID
	CoopSize       int
	BoostOrder     int // How the contract is sorted
	BoostVoting    int
	BoostPosition  int       // Starting Slot
	State          int       // Boost Completed
	StartTime      time.Time // When Contract is started
	EndTime        time.Time // When final booster ends
	EggFarmers     map[string]*EggFarmer
	RegisteredNum  int
	Boosters       map[string]*Booster // Boosters Registered
	Order          []string
	OrderRevision  int  // Incremented when Order is changed
	Speedrun       bool // Speedrun mode
	SRData         SpeedrunData
	lastWishPrompt string     // saved prompt for this contract
	mutex          sync.Mutex // Keep this contract thread safe
}

// SpeedrunData holds the data for a speedrun
type SpeedrunData struct {
	SpeedrunState         int    // Speedrun state
	SpeedrunStarterUserID string // Sink CRT User ID
	SinkUserID            string // Sink End of Contract User ID
	SinkBoostPosition     int    // Sink Boost Position
	SpeedrunStyle         int    // Speedrun Style
	ChickenRuns           int    // Number of Chicken Runs for this contract
	Legs                  int    // Number of legs for this Tango
	Tango                 [3]int // The Tango itself (First, Middle, Last)
	CurrentLeg            int    // Current Leg
	StatusStr             string // Status string for the speedrun
}

var (
	// Contracts is a map of contracts and is saved to disk
	Contracts map[string]*Contract
)

func init() {
	Contracts = make(map[string]*Contract)

	initDataStore()

	var c, err = loadData()
	if err == nil {
		Contracts = c
	}
}

func removeLocIndex(s []*LocationData, index int) []*LocationData {
	return append(s[:index], s[index+1:]...)
}

// DeleteContract will delete the contract
func DeleteContract(s *discordgo.Session, guildID string, channelID string) (string, error) {
	var contract = FindContract(guildID, channelID)
	if contract == nil {
		return "", errors.New(errorNoContract)
	}

	var coopHash = contract.ContractHash
	var coopName = contract.ContractID + "/" + contract.CoopID
	saveEndData(contract) // Save for historical purposes

	for _, el := range contract.Location {
		if s != nil {
			s.ChannelMessageDelete(el.ChannelID, el.ListMsgID)
			s.ChannelMessageDelete(el.ChannelID, el.ReactionID)
		}
	}
	delete(Contracts, coopHash)
	saveData(Contracts)

	return coopName, nil
}

// FindTokenEmoji will find the token emoji for the given guild
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
	var ContractHash = xid.New().String()

	// Make sure this channel doesn't already have a contract
	existingContract := FindContract(guildID, channelID)
	if existingContract != nil {
		return nil, errors.New("this channel already has a contract named: " + existingContract.ContractID + "/" + existingContract.CoopID)
	}

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
		contract.Speedrun = false
		contract.SRData.SpeedrunState = SpeedrunStateNone

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

	//	if !saveStaleContracts(s) {
	// Didn't prune any contracts so we should save this
	//	saveData(Contracts)
	//	}

	return contract, nil
}

// AddBoostTokens will add tokens to the current booster and adjust the count of the booster
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

// SetListMessageID will save the list messageID for the contract
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

// SetReactionID will save the reactionID for the contract
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

// FindContract will find the contract by the guildID and channelID
func FindContract(guildID string, channelID string) *Contract {
	// Look for the contract
	for key, element := range Contracts {
		for _, el := range element.Location {
			// ChannelIDs are unique globally
			if el.ChannelID == channelID {
				// Found the location of the contract, which one is it?
				return Contracts[key]
			}
		}
	}
	return nil
}

// FindContractByMessageID will find the contract by the messageID
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

// ChangePingRole will change the ping role for the contract
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

// MoveBooster will move a booster to a new position in the contract
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
	contract.Order = newOrder
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

// ChangeBoostOrder will change the order of the boosters in the contract
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

// AddContractMember adds a member to a contract
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

// AddFarmerToContract adds a farmer to a contract
func AddFarmerToContract(s *discordgo.Session, contract *Contract, guildID string, channelID string, userID string, order int) (*EggFarmer, error) {
	log.Println("AddFarmerToContract", "GuildID: ", guildID, "ChannelID: ", channelID, "UserID: ", userID, "Order: ", order)
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

		// Determine if userID is a snowflake (17-19 character string of numbers) using regex
		re := regexp.MustCompile(`[0-9]{17,19}`)
		if re.MatchString(userID) {
			// userID is a snowflake
			gm, errGM := s.GuildMember(guildID, userID)
			if errGM == nil {
				farmer.Username = gm.User.Username
				farmer.Nick = gm.Nick
				farmer.Unique = gm.User.String()
			}
		}

		// Guest Farmer or unknown guild member will have a blank username
		if farmer.Username == "" {
			// userID is not a snowflake
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
			contract.OrderRevision++
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

// IsUserCreatorOfAnyContract will return true if the user is the creator of any contract
func IsUserCreatorOfAnyContract(userID string) bool {
	for _, c := range Contracts {
		if creatorOfContract(c, userID) {
			return true
		}
	}
	return false
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

// findNextBooster returns the index of the next booster that needs to boost
func findNextBooster(contract *Contract) int {
	for i := 0; i < len(contract.Order); i++ {
		if contract.Boosters[contract.Order[i]].BoostState == BoostStateUnboosted || contract.Boosters[contract.Order[i]].BoostState == BoostStateTokenTime {
			return i
		}
	}
	return -1
}

// JoinContract will add a user to the contract
func JoinContract(s *discordgo.Session, guildID string, channelID string, userID string, bell bool) error {
	var err error

	log.Println("JoinContract", "GuildID: ", guildID, "ChannelID: ", channelID, "UserID: ", userID, "Bell: ", bell)

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
		var str = fmt.Sprintf("Boost notifications will be sent for %s/%s.", contract.ContractID, contract.CoopID)
		_, err := s.ChannelMessageSend(u.ID, str)
		if err != nil {
			panic(err)
		}

	}

	return nil
}

func removeIndex(s []string, index int) []string {
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
		contract.Order = removeIndex(contract.Order, index)
		contract.OrderRevision++
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

		contract.Order = removeIndex(contract.Order, index)
		contract.OrderRevision++
		//remove userID from Boosters
		refreshBoostListMessage(s, contract)

	}
	return true
}

// Unboost will mark a user as unboosted
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

// RemoveContractBoosterByMention will remove a booster from the contract by mention
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
	userID := mention

	if strings.HasPrefix(userID, "<@") {
		var u, _ = s.User(userID)
		if u != nil {
			if u.Bot {
				return errors.New(errorBot)
			}
		}
		userID = mention[2 : len(mention)-1]
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
			// Need to disable the speedrun start button if the contract is no longer full
			if contract.Speedrun && contract.State == ContractStateSignup {
				if (contract.CoopSize - 1) == len(contract.Order) {
					msgID := loc.ReactionID
					msg := discordgo.NewMessageEdit(loc.ChannelID, msgID)
					// Full contract for speedrun
					contentStr, comp := GetSignupComponents(true, contract.Speedrun) // True to get a disabled start button
					msg.SetContent(contentStr)
					msg.Components = &comp
					s.ChannelMessageEditComplex(msg)
				}
			}

		}
	}

	return nil
}

// RemoveContractBooster will remove a booster from the contract
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

// StartContractBoosting will start the contract
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

	// Only need to do speedruns if we have more than one leg
	if contract.Speedrun && contract.SRData.Legs > 1 {
		contract.SRData.SpeedrunState = SpeedrunStateCRT
		// Do not mark the token sink as boosting at this point
		// This will happen when the CRT completes
	} else {
		// Start at the top of the boost list
		contract.SRData.SpeedrunState = SpeedrunStateBoosting
		contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = BoostStateTokenTime
		contract.Boosters[contract.Order[contract.BoostPosition]].StartTime = time.Now()
	}

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
		if contract.Speedrun && contract.State == ContractStateSignup {
			if contract.CoopSize == len(contract.Order) {
				msgID := loc.ReactionID
				msg := discordgo.NewMessageEdit(loc.ChannelID, msgID)

				// Full contract for speedrun
				contentStr, comp := GetSignupComponents(false, contract.Speedrun) // True to get a disabled start button
				msg.SetContent(contentStr)
				msg.Components = &comp
				s.ChannelMessageEditComplex(msg)
			}
		}

	}
}

func addContractReactions(s *discordgo.Session, contract *Contract, channelID string, messageID string, tokenStr string) {
	if contract.Speedrun {
		switch contract.SRData.SpeedrunState {
		case SpeedrunStateCRT:
			addSpeedrunContractReactions(s, contract, channelID, messageID, tokenStr)
			return
		case SpeedrunStateBoosting:
			if contract.SRData.SpeedrunStyle == SpeedrunStyleWonky {
				addSpeedrunContractReactions(s, contract, channelID, messageID, tokenStr)
				return
			}
		case SpeedrunStatePost:
			addSpeedrunContractReactions(s, contract, channelID, messageID, tokenStr)
			return
		default:
			break
		}
	}

	if contract.State == ContractStateStarted {
		s.MessageReactionAdd(channelID, messageID, "🚀")             // Booster
		err := s.MessageReactionAdd(channelID, messageID, tokenStr) // Token Reaction
		if err != nil {
			fmt.Print(err.Error())
		}
		s.MessageReactionAdd(channelID, messageID, "🔃")  // Swap
		s.MessageReactionAdd(channelID, messageID, "⤵️") // Last
		s.MessageReactionAdd(channelID, messageID, "🐓")  // Want Chicken Run
	}
	if contract.State == ContractStateWaiting {
		s.MessageReactionAdd(channelID, messageID, "🐓") // Want Chicken Run
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
			if contract.State == ContractStateArchive {
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

		if contract.State == ContractStateStarted || contract.State == ContractStateWaiting {
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
		} else if contract.State >= ContractStateCompleted {
			t1 := contract.EndTime
			t2 := contract.StartTime
			duration := t1.Sub(t2)
			str = fmt.Sprintf(loc.ChannelPing+" contract boosting complete in %s ", duration.Round(time.Second))
		}

		// Sending the update message
		if !contract.Speedrun {
			s.ChannelMessageSend(loc.ChannelID, str)
		} else {
			RedrawBoostList(s, loc.GuildID, loc.ChannelID)
		}
		//if err == nil {
		//SetListMessageID(contract, loc.ChannelID, msg.ID)
		//}

	}
	if pingUsers {
		notifyBellBoosters(s, contract)
	}
	if !contract.Speedrun && contract.State == ContractStateCompleted {
		FinishContract(s, contract)
	} else if contract.Speedrun && contract.SRData.SpeedrunState == SpeedrunStateComplete {
		FinishContract(s, contract)
	}

}

// UserBoost will trigger a contract boost of a user
func UserBoost(s *discordgo.Session, guildID string, channelID string, userID string) error {
	var contract = FindContract(guildID, channelID)

	if contract == nil {
		return errors.New(errorNoContract)
	}

	if contract.State == ContractStateSignup {
		return errors.New(errorContractEmpty)
	}

	if contract.BoostPosition != -1 &&
		contract.BoostPosition < len(contract.Order) &&
		userID == contract.Order[contract.BoostPosition] {
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
		if contract.Speedrun {
			contract.SRData.SpeedrunState = SpeedrunStatePost
		}
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

// SkipBooster will skip the current booster and move to the next
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
			contract.Order = removeIndex(contract.Order, contract.BoostPosition)
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
		contract.Order = removeIndex(contract.Order, selectedUser)
		contract.Order = insert(contract.Order, contract.BoostPosition, skipped)
		contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = BoostStateTokenTime
		contract.Boosters[contract.Order[contract.BoostPosition]].StartTime = time.Now()
	}
	contract.OrderRevision++

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
	farmerstate.SetOrderPercentileAll(contract.Order, len(contract.Order))
	DeleteContract(s, contract.Location[0].GuildID, contract.Location[0].ChannelID)
	return nil
}

func reorderBoosters(contract *Contract) {
	if contract.Speedrun {
		reorderSpeedrunBoosters(contract)
	} else {
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
