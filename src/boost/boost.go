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
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
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

const ContractOrderSignup = 0
const ContractOrderLast = 0
const ContractOrderReverse = 1
const ContractOrderRandom = 2
const ContractOrderFair = 3

const ContractStateSignup = 0
const ContractStateStarted = 1
const ContractStateWaiting = 2
const ContractStateCompleted = 3

const BoostStateUnboosted = 0
const BoostStateTokenTime = 1
const BoostStateBoosted = 2

type Farmer struct {
	UserID    string // Discord User ID
	Username  string
	Unique    string
	Nick      string
	GameName  string
	GuildID   string // Discord Guild where this User is From
	GuildName string
	Ping      bool      // True/False
	Register  time.Time // Time Farmer registered
	//Cluck       []string  // Keep track of messages from each user
}
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
	UserID     string // Egg Farmer
	Name       string
	BoostState int       // Indicates if current booster
	Mention    string    // String which mentions user
	TokenCount int       // indicate number of boost tokens
	StartTime  time.Time // Time Farmer started boost turn
	EndTime    time.Time // Time Farmer ended boost turn
}

type LocationData struct {
	GuildID        string
	GuildName      string
	ChannelID      string // Contract Discord Channel
	ChannelName    string
	ChannelMention string
	ChannelPing    string
	ListMsgID      string // Message ID for the Last Boost Order message
	ReactionID     string // Message ID for the reaction Order String
	TokenStr       string // Emoji for Token
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
	OrderRevision int // Incremented when Order is changed
	//mutex         sync.Mutex // Keep this contract thread safe
}

var (
	// DiscordToken holds the API Token for discord.
	Contracts map[string]*Contract
	Farmers   map[string]*Farmer
)

func init() {
	Contracts = make(map[string]*Contract)
	Farmers = make(map[string]*Farmer)
	//GlobalContracts = make(map[string][]*LocationData)

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

func DeleteContract(s *discordgo.Session, guildID string, channelID string) string {
	var contract = FindContract(guildID, channelID)

	if contract != nil {
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
		return coop
	}
	return ""
}

func FindTokenEmoji(s *discordgo.Session, guildID string) string {
	g, _ := s.State.Guild(guildID) // RAIYC Playground
	var e = emutil.FindEmoji(g.Emojis, "token", false)
	if e != nil {
		return e.MessageFormat()
	}
	return "🐣"
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
		contract.CreatorID = append(contract.CreatorID, userID)               // starting userid
		contract.CreatorID = append(contract.CreatorID, config.AdminUserID)   // overall admin user
		contract.CreatorID = append(contract.CreatorID, "393477262412087319") // Tbone user id
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
	}

	return contract, nil
}

func SetMessageID(contract *Contract, channelID string, messageID string) {
	for _, element := range contract.Location {
		if element.ChannelID == channelID {
			element.ListMsgID = messageID
		}
	}
}

func SetReactionID(contract *Contract, channelID string, messageID string) {
	for _, element := range contract.Location {
		if element.ChannelID == channelID {
			element.ReactionID = messageID
		}
	}
}

func DrawBoostList(s *discordgo.Session, contract *Contract, tokenStr string) string {
	var outputStr string

	saveData(Contracts)

	outputStr = fmt.Sprintf("### %s  %d/%d ###\n", contract.ContractHash, len(contract.Boosters), contract.CoopSize)

	if contract.State == ContractStateSignup {
		outputStr += "## Sign-up List ###\n"
	} else {
		outputStr += "## Boost List ###\n"
	}
	var i = 1
	var prefix = " - "
	for _, element := range contract.Order {
		if contract.State != ContractStateSignup {
			prefix = fmt.Sprintf("%2d - ", i)
		}
		var b, ok = contract.Boosters[element]
		if ok {
			var name = b.Mention
			var server = ""
			var currentStartTime = fmt.Sprintf(" <t:%d:R> ", b.StartTime.Unix())
			if len(contract.Location) > 1 {
				server = fmt.Sprintf(" (%s) ", contract.EggFarmers[element].GuildName)
			}

			switch b.BoostState {
			case BoostStateUnboosted:
				outputStr += fmt.Sprintf("%s %s%s\n", prefix, name, server)
			case BoostStateTokenTime:
				countStr := ""
				//if b.TokenCount > 0 {
				//	countStr = ":" + num2words.Convert(b.TokenCount) + ":"
				//}
				outputStr += fmt.Sprintf("%s %s %s%s%s\n", prefix, name, countStr+tokenStr, currentStartTime, server)
			case BoostStateBoosted:
				t1 := contract.Boosters[element].EndTime
				t2 := contract.Boosters[element].StartTime
				duration := t1.Sub(t2)
				outputStr += fmt.Sprintf("%s ~~%s~~  %s %s\n", prefix, name, duration.Round(time.Second), server)
			}
			i += 1
		}
	}

	if contract.State == ContractStateStarted {
		outputStr += "```"
		outputStr += "React with 🚀 when you spend tokens to boost. Multiple 🚀 votes by others in the contract will also indicate a boost.\n"
		if (contract.BoostPosition + 1) < len(contract.Order) {
			outputStr += "React with 🔃 to exchange position with the next booster.\nReact with ⤵️ to move to last. "
			outputStr += "\nAdd a 🚽 (toilet) reaction to express your urgency to go now."
		}
		outputStr += "```"
	} else if contract.State == ContractStateWaiting {
		outputStr += "Waiting for other(s) to join..."
		outputStr += "```"
		outputStr += "React with 🏁 to end the contract."
		outputStr += "```"

	}
	return outputStr
}

func FindContractByMessageID(channelID string, messageID string) (*Contract, int) {
	// Given a
	for _, c := range Contracts {
		for i, loc := range c.Location {
			if loc.ChannelID == channelID && loc.ListMsgID == messageID {
				return c, i
			}
		}
	}
	return nil, 0
}

func FindContractByReactionID(channelID string, ReactionID string) (*Contract, int) {
	// Given a
	for _, c := range Contracts {
		for i, loc := range c.Location {
			if loc.ChannelID == channelID && loc.ReactionID == ReactionID {
				return c, i
			}
		}
	}
	return nil, 0
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

// write the function removeBoostOrderIndex which takes an array and an index as arguments and returns the array with the element at the given index removed.
func removeBoostOrderIndex(s []string, index int) []string {
	return append(s[:index], s[index+1:]...)
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

func ChangeBoostOrder(s *discordgo.Session, guildID string, channelID string, userID string, boostOrder string) error {
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

	// split the boostOrder string into an array by commas
	re := regexp.MustCompile(`[\\<>@#&!]`)
	if boostOrder != "" {
		boostOrderClean = re.ReplaceAllString(boostOrder, "")
	}

	var boostOrderArray = strings.Split(boostOrderClean, ",")
	// expand hyphenated values into a range, incrementing or decrementing as appropriate and append them to the boostOrderArray
	for i, element := range boostOrderArray {
		var hyphenArray = strings.Split(element, "-")
		if len(hyphenArray) == 2 {
			var start, _ = strconv.Atoi(hyphenArray[0])
			var end, _ = strconv.Atoi(hyphenArray[1])
			if start > end {
				for j := start; j >= end; j-- {
					boostOrderArray = append(boostOrderArray, strconv.Itoa(j))
				}
			} else {
				for j := start; j <= end; j++ {
					boostOrderArray = append(boostOrderArray, strconv.Itoa(j))
				}
			}
			boostOrderArray = removeBoostOrderIndex(boostOrderArray, i)
		}
	}

	// Remove duplicates from boostOrderArray calling removeDuplicates function
	boostOrderArray = removeDuplicates(boostOrderArray)

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
	if contract.State == ContractStateStarted {
		contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = BoostStateUnboosted
	}

	// set contract.BoostOrder to the index of the element contract.Boosters[element].BoostState == BoostStateTokenTime
	contract.Order = newOrder
	contract.OrderRevision += 1

	for i, el := range newOrder {
		if contract.Boosters[el].BoostState == BoostStateUnboosted {
			contract.BoostPosition = i
			contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = BoostStateTokenTime
			contract.Boosters[contract.Order[contract.BoostPosition]].StartTime = time.Now()
			break
		}
	}

	sendNextNotification(s, contract, true)
	/*
		// Draw the boost List in place
		for _, loc := range contract.Location {
			msg, err := s.ChannelMessageEdit(loc.ChannelID, loc.ListMsgID, DrawBoostList(s, contract, loc.TokenStr))
			if err == nil {
				loc.ListMsgID = msg.ID
			}
		}
	*/
	return nil
}

func AddContractMember(s *discordgo.Session, guildID string, channelID string, operator string, mention string, guest string, order int64) error {
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
			var str = fmt.Sprintf("%s, was added to the %s List by %s", u.Mention(), listStr, operator)

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

		var farmer, fe = AddFarmerToContract(s, contract, guildID, channelID, guest, int64(contract.BoostOrder))
		if fe == nil {
			// Need to rest the farmer reaction count when added this way
			farmer.Reactions = 0
		}
		for _, loc := range contract.Location {
			var listStr = "Boost"
			if contract.State == ContractStateSignup {
				listStr = "Sign-up"
			}
			var str = fmt.Sprintf("%s, was added to the %s List by %s", guest, listStr, operator)
			s.ChannelMessageSend(loc.ChannelID, str)
		}
	}

	return nil
}

func AddFarmerToContract(s *discordgo.Session, contract *Contract, guildID string, channelID string, userID string, order int64) (*EggFarmer, error) {
	var farmer = contract.EggFarmers[userID]
	if farmer == nil {
		// New Farmer
		farmer = new(EggFarmer)
		farmer.Register = time.Now()
		farmer.Ping = false
		farmer.Reactions = 0
		farmer.UserID = userID
		farmer.GuildID = guildID
		var ch, _ = s.Channel(channelID)
		farmer.ChannelName = ch.Name
		var g, _ = s.Guild(guildID)
		farmer.GuildName = g.Name
		var gm, _ = s.GuildMember(guildID, userID)
		if gm != nil {
			farmer.Username = gm.User.Username
			farmer.Nick = gm.Nick
			farmer.Unique = gm.User.String()
		} else {
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
		if err == nil {
			b.Name = user.Username
			b.BoostState = BoostStateUnboosted
			b.Mention = user.Mention()
		} else {
			b.Name = userID
			b.BoostState = BoostStateUnboosted
			b.Mention = userID
		}
		var member, gmErr = s.GuildMember(guildID, userID)
		if gmErr == nil && member.Nick != "" {
			b.Name = member.Nick
			b.Mention = member.Mention()
		}

		if !userInContract(contract, farmer.UserID) {
			contract.Boosters[farmer.UserID] = b
			// If contract hasn't started add booster to the end
			// or if contract is on the last booster already
			if contract.State == ContractStateSignup || contract.State == ContractStateWaiting || order == ContractOrderLast {
				contract.Order = append(contract.Order, farmer.UserID)
			} else {
				// Insert booster randomly into non-boosting order
				var remainingBoosters = len(contract.Boosters) - contract.BoostPosition - 1
				if remainingBoosters == 0 {
					contract.Order = append(contract.Order, farmer.UserID)
				} else {
					var insertPosition = contract.BoostPosition + 1 + rand.Intn(remainingBoosters)
					contract.Order = insert(contract.Order, insertPosition, farmer.UserID)

				}
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
		}
		if contract.State != ContractStateSignup {
			sendNextNotification(s, contract, false)
		} else {
			// Edit the boost list in place
			for _, loc := range contract.Location {
				msg, err := s.ChannelMessageEdit(loc.ChannelID, loc.ListMsgID, DrawBoostList(s, contract, loc.TokenStr))
				if err == nil {
					//panic(err)
					loc.ListMsgID = msg.ID
				}
			}
		}
	}

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

	//var contract = FindContract(r.GuildID, r.ChannelID)
	//if contract == nil {
	var contract, _ = FindContractByReactionID(r.ChannelID, r.MessageID)
	if contract == nil {
		contract, _ = FindContractByMessageID(r.ChannelID, r.MessageID)
		if contract == nil {
			return ""
		}
	}
	//}
	//contract.mutex.Lock()
	defer saveData(Contracts)
	// If we get a stopwatch reaction from the contract creator, start the contract
	if r.Emoji.Name == "⏱️" && contract.State == ContractStateSignup && creatorOfContract(contract, r.UserID) {
		//contract.mutex.Unlock()
		StartContractBoosting(s, r.GuildID, r.ChannelID)
		return ""
	}

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
					// Reaction to jump to end
					if r.Emoji.Name == "⤵️" {
						//contract.mutex.Unlock()
						SkipBooster(s, r.GuildID, r.ChannelID, contract.Order[contract.BoostPosition])
						return ""
					}
				}
			} else {
				// Reaction to indicate you need to go now
				if r.Emoji.Name == "🚽" {
					SkipBooster(s, r.GuildID, r.ChannelID, r.UserID)
					return "!gonow"
				}
			}

			if contract.State == ContractStateWaiting && r.Emoji.Name == "🔃" {
				contract.State = ContractStateCompleted
				contract.EndTime = time.Now()
				sendNextNotification(s, contract, true)
				return ""
			}
		}
	}
	if r.Emoji.Name == "🚽" {
		SkipBooster(s, r.GuildID, r.ChannelID, r.UserID)
		return "!gonow"
	}

	//defer contract.mutex.Unlock()
	// Remove extra added emoji
	if r.Emoji.Name != "🧑‍🌾" && r.Emoji.Name != "🔔" {
		s.MessageReactionRemove(r.ChannelID, r.MessageID, r.Emoji.Name, r.UserID)
		return ""
	}

	if len(contract.Order) < contract.CoopSize {
		var farmer, e = AddFarmerToContract(s, contract, r.GuildID, r.ChannelID, r.UserID, int64(contract.BoostOrder))
		if e == nil {
			farmer.Reactions = getReactions(s, r.ChannelID, r.MessageID, r.UserID)
			if r.Emoji.Name == "🔔" {
				farmer.Ping = true
				u, _ := s.UserChannelCreate(farmer.UserID)
				var str = fmt.Sprintf("Boost notifications will be sent for %s.", contract.ContractHash)
				_, err := s.ChannelMessageSend(u.ID, str)
				if err != nil {
					panic(err)
				}
			}
		}
	}
	return ""
}

func RemoveIndex(s []string, index int) []string {
	return append(s[:index], s[index+1:]...)
}

func removeContractBoosterByContract(s *discordgo.Session, contract *Contract, offset int) bool {
	if offset > len(contract.Boosters) {
		return false
	}
	var index = offset - 1 // Index is 0 based

	var activeBooster, ok = contract.Boosters[contract.Order[index]]
	if ok {
		var activeBoosterState = activeBooster.BoostState
		var userID = contract.Order[index]
		contract.Order = RemoveIndex(contract.Order, index)
		contract.OrderRevision += 1
		delete(contract.Boosters, userID)

		// Active Booster is leaving contract.
		if contract.BoostPosition == len(contract.Order) {
			// set contract to waiting
			contract.State = ContractStateWaiting
			sendNextNotification(s, contract, true)
		} else if (activeBoosterState == BoostStateUnboosted) && len(contract.Order) > index {
			contract.Boosters[contract.Order[index]].BoostState = BoostStateTokenTime
			contract.Boosters[contract.Order[index]].StartTime = time.Now()
			sendNextNotification(s, contract, true)
		}
	} else {
		contract.Order = RemoveIndex(contract.Order, index)
		contract.OrderRevision += 1
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

		sendNextNotification(s, contract, true)
	} else {
		contract.Boosters[userID].BoostState = BoostStateUnboosted

		// Edit the boost List in place
		for _, loc := range contract.Location {
			msg, err := s.ChannelMessageEdit(loc.ChannelID, loc.ListMsgID, DrawBoostList(s, contract, loc.TokenStr))
			if err == nil {
				loc.ListMsgID = msg.ID
			}
		}
	}
	return nil
}

func RemoveContractBoosterByMention(s *discordgo.Session, guildID string, channelID string, operator string, mention string) error {
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

	var found = false
	for i := range contract.Order {
		if contract.Order[i] == userID {
			found = true
			if removeContractBoosterByContract(s, contract, i+1) {
				contract.RegisteredNum = len(contract.Boosters)
			}
			break
		}
	}
	if !found {
		return errors.New(errorUserNotInContract)
	}

	// Edit the boost List in place
	for _, loc := range contract.Location {
		msg, err := s.ChannelMessageEdit(loc.ChannelID, loc.ListMsgID, DrawBoostList(s, contract, loc.TokenStr))
		if err == nil {
			loc.ListMsgID = msg.ID
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
	for _, loc := range contract.Location {

		msg, err := s.ChannelMessageEdit(loc.ChannelID, loc.ListMsgID, DrawBoostList(s, contract, loc.TokenStr))
		if err == nil {
			loc.ListMsgID = msg.ID
		}
	}
	return nil
}

func getReactions(s *discordgo.Session, channelID string, messageID string, userID string) int {
	var reaction = 0
	var emoji = [2]string{"🧑‍🌾", "🔔"}
	for _, e := range emoji {
		var m, me = s.MessageReactions(channelID, messageID, e, 50, "", "")
		if me == nil {
			for _, r := range m {
				if r.ID == userID {
					reaction += 1
				}
			}
		}
	}
	return reaction
}

func ReactionRemove(s *discordgo.Session, r *discordgo.MessageReaction) {
	var _, err = s.ChannelMessage(r.ChannelID, r.MessageID)
	if err != nil {
		return
	}

	//var contract = FindContract(r.GuildID, r.ChannelID)
	//if contract == nil {
	var contract, _ = FindContractByReactionID(r.ChannelID, r.MessageID)
	if contract == nil {
		contract, _ = FindContractByMessageID(r.ChannelID, r.MessageID)
		if contract == nil {
			return
		}
	}
	//}

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

	if r.Emoji.Name != "🧑‍🌾" && r.Emoji.Name != "🔔" {
		return
	}

	farmer.Reactions = getReactions(s, r.ChannelID, r.MessageID, r.UserID)

	if r.Emoji.Name == "🔔" {
		farmer.Ping = false
	}

	if farmer.Reactions == 0 {
		// Remove farmer from boost list
		for i := range contract.Order {
			if contract.Order[i] == r.UserID {
				removeContractBoosterByContract(s, contract, i+1)
				break
			}
		}
		contract.RegisteredNum = len(contract.Boosters)

		for _, loc := range contract.Location {
			msg, err := s.ChannelMessageEdit(loc.ChannelID, loc.ListMsgID, DrawBoostList(s, contract, loc.TokenStr))
			if err == nil {
				loc.ListMsgID = msg.ID
			}
		}
	}
}

func FindContract(guildID string, channelID string) *Contract {
	// Look for the contract
	for key, element := range Contracts {
		for _, el := range element.Location {
			if el.GuildID == guildID && el.ChannelID == channelID {
				return Contracts[key]
			}
		}
	}
	return nil
}

func StartContractBoosting(s *discordgo.Session, guildID string, channelID string) error {
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

	// Check Voting for eeeeeeized Order
	// Supermajority 2/3
	/*
		if contract.BoostVoting > 1 {
			var votingStr = "Random boost order supermajority vote "
			if contract.BoostVoting < ((len(contract.Boosters) * 2) / 3) {
				votingStr += "failed"
			} else {
				votingStr += "passed"
				contract.BoostOrder = 2
			}
			votingStr = fmt.Sprintf("%s %d/%d.", votingStr, contract.BoostVoting, len(contract.Boosters))
			for _, el := range contract.Location {
				s.ChannelMessageSend(el.ChannelID, votingStr)
			}
		}
	*/
	// Contracts are always random order
	contract.BoostOrder = ContractOrderRandom
	reorderBoosters(contract)

	contract.BoostPosition = 0
	contract.State = ContractStateStarted
	contract.StartTime = time.Now()
	contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = BoostStateTokenTime
	contract.Boosters[contract.Order[contract.BoostPosition]].StartTime = time.Now()

	sendNextNotification(s, contract, true)

	return nil
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
			if contract.CoopSize == len(contract.Boosters) {
				s.ChannelMessageUnpin(loc.ChannelID, loc.ReactionID)
			}
			s.ChannelMessageDelete(loc.ChannelID, loc.ListMsgID)

			// Compose the message without a Ping
			var data discordgo.MessageSend
			var am discordgo.MessageAllowedMentions
			data.Content = DrawBoostList(s, contract, loc.TokenStr)
			data.AllowedMentions = &am
			msg, err = s.ChannelMessageSendComplex(loc.ChannelID, &data)

			loc.ListMsgID = msg.ID
		}
		if err != nil {
			fmt.Println("Unable to resend message.")
		}
		var str = ""

		if contract.State != ContractStateCompleted {
			if contract.State == ContractStateStarted {
				s.MessageReactionAdd(loc.ChannelID, msg.ID, "🚀") // Booster
			}
			if (contract.BoostPosition + 1) < len(contract.Order) {
				s.MessageReactionAdd(loc.ChannelID, msg.ID, "🔃")  // Swap
				s.MessageReactionAdd(loc.ChannelID, msg.ID, "⤵️") // Last
			}
			if contract.State == ContractStateWaiting {
				s.MessageReactionAdd(loc.ChannelID, msg.ID, "🏁") // Finish
			}

			if pingUsers {
				if contract.State == ContractStateStarted {
					str = fmt.Sprintf(loc.ChannelPing+" send tokens to %s", contract.Boosters[contract.Order[contract.BoostPosition]].Mention)
				} else {
					str = fmt.Sprintf(loc.ChannelPing + " contract boosting complete hold your tokens for late joining farmers.")
				}
			}
		} else {
			t1 := contract.EndTime
			t2 := contract.StartTime
			duration := t1.Sub(t2)
			str = fmt.Sprintf(loc.ChannelPing+" contract boosting complete in %s ", duration.Round(time.Second))
		}
		loc.ListMsgID = msg.ID
		s.ChannelMessageSend(loc.ChannelID, str)
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

	// Advance past any that have already boosted
	for contract.Boosters[contract.Order[contract.BoostPosition]].BoostState == BoostStateBoosted {
		contract.BoostPosition += 1
		// loop through contract.Order until we find a non-boosted user
		for i := range contract.Order {
			if contract.Boosters[contract.Order[i]].BoostState == BoostStateUnboosted {
				contract.BoostPosition = i
				break
			}
		}
	}

	if contract.BoostPosition == contract.CoopSize {
		contract.State = ContractStateCompleted // Finished
		contract.EndTime = time.Now()
	} else if contract.BoostPosition == len(contract.Boosters) {
		contract.State = ContractStateWaiting // There could be more boosters joining later
	} else {
		contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = BoostStateTokenTime
		contract.Boosters[contract.Order[contract.BoostPosition]].StartTime = time.Now()
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

	}
}

// AdvancedTransform for storing KV pairs
func AdvancedTransform(key string) *diskv.PathKey {
	path := strings.Split(key, "/")
	last := len(path) - 1
	return &diskv.PathKey{
		Path:     path[:last],
		FileName: path[last] + ".txt",
	}
}

// InverseTransform for storing KV pairs
func InverseTransform(pathKey *diskv.PathKey) (key string) {
	txt := pathKey.FileName[len(pathKey.FileName)-4:]
	if txt != ".txt" {
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

func StartSignup(s *discordgo.Session, i *discordgo.InteractionCreate, number int, contractID string, coopID string, coopSize int, threshold int, threadChannel *discordgo.Channel) {

}
