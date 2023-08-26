package boost

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/peterbourgon/diskv/v3"
	emutil "github.com/post04/discordgo-emoji-util"
)

//var usermutex sync.Mutex
//var diskmutex sync.Mutex

var DataStore *diskv.Diskv

var TokenStr = "" //"<:token:778019329693450270>"

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
	Priority   bool      // Requested Early Boost
	Later      bool      // Requested to Boost Later
	StartTime  time.Time // Time Farmer started boost turn
	EndTime    time.Time // Time Farmer ended boost turn
}

type LocationData struct {
	GuildID        string
	GuildName      string
	ChannelID      string // Contract Discord Channel
	ChannelName    string
	ChannelMention string
	ListMsgID      string // Message ID for the Last Boost Order message
	ReactionID     string // Message ID for the reaction Order String
}
type Contract struct {
	ContractHash  string // ContractID-CoopID
	Location      []*LocationData
	UserID        string // Farmer Name of Creator
	ContractID    string // Contract ID
	CoopID        string // CoopID
	CoopSize      int
	BoostOrder    int
	BoostVoting   int
	BoostPosition int       // Starting Slot
	BoostState    int       // Boost Completed
	StartTime     time.Time // When Contract is started
	EndTime       time.Time // When final booster ends
	EggFarmers    map[string]*EggFarmer
	RegisteredNum int
	Boosters      map[string]*Booster // Boosters Registered
	Order         []string
}

var (
	// DiscordToken holds the API Token for discord.
	Contracts map[string]*Contract
)

func init() {
	Contracts = make(map[string]*Contract)
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

// interface
func CreateContract(s *discordgo.Session, contractID string, coopID string, coopSize int, BoostOrder int, guildID string, channelID string, userID string) (*Contract, error) {
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
		contract.BoostState = 0
		contract.UserID = userID // starting userid
		contract.RegisteredNum = 0
		contract.CoopSize = coopSize
		Contracts[ContractHash] = contract
	} else {
		contract.Location = append(contract.Location, loc)
	}

	// Find our Token emoji
	for _, el := range contract.Location {
		g, _ := s.State.Guild(el.GuildID) // RAIYC Playground
		var e = emutil.FindEmoji(g.Emojis, "token", false)
		if e != nil {
			TokenStr = e.MessageFormat()
		}
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

func DrawBoostList(s *discordgo.Session, contract *Contract) string {
	var outputStr string

	saveData(Contracts)

	outputStr = fmt.Sprintf("### %s  %d/%d ###\n", contract.ContractHash, len(contract.Boosters), contract.CoopSize)

	if contract.BoostState == 0 {
		outputStr += "## Sign-up List ###\n"
	} else {
		outputStr += "## Boost List ###\n"
	}
	var i = 1
	var prefix = " - "
	for _, element := range contract.Order {
		if contract.BoostState != 0 {
			prefix = fmt.Sprintf("%2d - ", i)
		}
		//for i := 1; i <= len(contract.Boosters); i++ {
		var b = contract.Boosters[element]
		var name = b.Name
		var server = ""
		var currentStartTime = fmt.Sprintf(" <t:%d:R> ", b.StartTime.Unix())
		if len(contract.Location) > 1 {
			server = fmt.Sprintf(" (%s) ", contract.EggFarmers[element].GuildName)
		}

		switch b.BoostState {
		case 0:
			outputStr += fmt.Sprintf("%s %s%s\n", prefix, name, server)
		case 1:
			outputStr += fmt.Sprintf("%s %s %s%s%s\n", prefix, name, TokenStr, currentStartTime, server)
		case 2:
			t1 := contract.Boosters[element].EndTime
			t2 := contract.Boosters[element].StartTime
			duration := t1.Sub(t2)
			outputStr += fmt.Sprintf("%s ~~%s~~  %s %s\n", prefix, name, duration.Round(time.Second), server)
		}
		i += 1
	}

	// Only draw empty slots when contract is active
	//if contract.BoostState != 2 {
	//	for ; i <= contract.CoopSize; i++ {
	//		outputStr += fmt.Sprintf("%2d -   open position\n", i)
	//	}
	//}
	if contract.BoostState == 1 {
		outputStr += "```"
		outputStr += "React with 🚀 when you spend tokens to boost. Multiple 🚀 votes by others in the contract will also indicate a boost.\n"
		if (contract.BoostPosition + 1) < len(contract.Order) {
			outputStr += "React with 🔃 to exchange position with the next booster.\nReact with ⤵️ to move to last."
		}
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

func AddContractMember(s *discordgo.Session, guildID string, channelID string, operator string, mention string) error {
	var contract = FindContract(guildID, channelID)
	if contract == nil {
		return errors.New(errorNoContract)
	}

	if contract.CoopSize == len(contract.Order) {
		return errors.New(errorContractFull)
	}

	re := regexp.MustCompile(`[\\<>@#&!]`)
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

	var farmer, fe = AddFarmerToContract(s, contract, guildID, channelID, u.ID)
	if fe == nil {
		// Need to rest the farmer reaction count when added this way
		farmer.Reactions = 0
	}

	for _, loc := range contract.Location {
		var listStr = "Boost"
		if contract.BoostState == 0 {
			listStr = "Sign-up"
		}
		var str = fmt.Sprintf("%s, was added to the %s List by %s", u.Mention(), listStr, operator)
		s.ChannelMessageSend(loc.ChannelID, str)
	}

	return nil
}

func AddFarmerToContract(s *discordgo.Session, contract *Contract, guildID string, channelID string, userID string) (*EggFarmer, error) {
	var err error
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
		farmer.Username = gm.User.Username
		farmer.Nick = gm.Nick
		farmer.Unique = gm.User.String()

		contract.EggFarmers[userID] = farmer
	}
	farmer.Reactions += 1
	var b = contract.Boosters[userID]
	if b == nil {
		// New Farmer - add them to boost list
		var b = new(Booster)
		b.UserID = farmer.UserID
		b.Priority = false
		b.Later = false
		var user, _ = s.User(userID)
		if err == nil {
			b.Name = user.Username
			b.BoostState = 0
			b.Mention = user.Mention()
		}
		var member, err = s.GuildMember(guildID, userID)
		if err == nil && member.Nick != "" {
			b.Name = member.Nick
			b.Mention = member.Mention()
		}
		contract.Boosters[farmer.UserID] = b
		contract.Order = append(contract.Order, farmer.UserID)
		contract.RegisteredNum += 1

		// Edit the boost list in place
		for _, loc := range contract.Location {
			msg, err := s.ChannelMessageEdit(loc.ChannelID, loc.ListMsgID, DrawBoostList(s, contract))
			if err != nil {
				panic(err)
			}
			loc.ListMsgID = msg.ID
		}

	}

	return farmer, nil
}

func userInContract(c *Contract, u string) bool {
	for _, el := range c.Order {
		if el == u {
			return true
		}
	}

	return false
}

func ReactionAdd(s *discordgo.Session, r *discordgo.MessageReaction) {
	// Find the message
	var msg, err = s.ChannelMessage(r.ChannelID, r.MessageID)
	if err != nil {
		return
	}

	var contract, _ = FindContractByReactionID(r.ChannelID, r.MessageID)
	if contract == nil {
		contract, _ = FindContractByMessageID(r.ChannelID, r.MessageID)
		if contract == nil {
			return
		}
	}

	// If we get a stopwatch reaction from the contract creator, start the contract
	if r.Emoji.Name == "⏱️" && contract.BoostState == 0 && r.UserID == contract.UserID {
		StartContractBoosting(s, r.GuildID, r.ChannelID)
		return
	}

	if userInContract(contract, r.UserID) {

		if contract.BoostState != 0 && contract.BoostPosition < len(contract.Order) {

			// If Rocket reaction on Boost List, only that boosting user can apply a reaction
			if r.Emoji.Name == "🚀" && contract.BoostState == 1 {
				var votingElection = (msg.Reactions[0].Count - 1) >= 2

				if r.UserID == contract.Order[contract.BoostPosition] || votingElection || r.UserID == contract.UserID {
					Boosting(s, r.GuildID, r.ChannelID)
				}
				return
			}

			// Reaction to change places
			if r.UserID == contract.Order[contract.BoostPosition] || r.UserID == contract.UserID {
				if (contract.BoostPosition + 1) < len(contract.Order) {
					if r.Emoji.Name == "🔃" {
						SkipBooster(s, r.GuildID, r.ChannelID, "")
						return
					}
					// Reaction to jump to end
					if r.Emoji.Name == "⤵️" {
						SkipBooster(s, r.GuildID, r.ChannelID, contract.Order[contract.BoostPosition])
						return
					}
				}
			}
		}
	}

	// Remove extra added emoji
	if r.Emoji.Name != "🧑‍🌾" && r.Emoji.Name != "🔔" && r.Emoji.Name != "🎲" {
		s.MessageReactionRemove(r.ChannelID, r.MessageID, r.Emoji.Name, r.UserID)
		return
	}

	if r.Emoji.Name == "🎲" {
		contract.BoostVoting += 1
		return
	}

	if len(contract.Order) < contract.CoopSize {
		var farmer, e = AddFarmerToContract(s, contract, r.GuildID, r.ChannelID, r.UserID)
		if e == nil {
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
}

func RemoveIndex(s []string, index int) []string {
	return append(s[:index], s[index+1:]...)
}

func removeContractBoosterByContract(s *discordgo.Session, contract *Contract, offset int) bool {
	if offset > len(contract.Boosters) {
		return false
	}
	var index = offset - 1 // Index is 0 based

	var activeBooster = contract.Boosters[contract.Order[index]].BoostState
	var userID = contract.Order[index]
	contract.Order = RemoveIndex(contract.Order, index)
	delete(contract.Boosters, userID)

	// Active Booster is leaving contract.
	if (activeBooster == 1) && len(contract.Order) > index {
		contract.Boosters[contract.Order[index]].BoostState = 2
		contract.Boosters[contract.Order[index]].StartTime = time.Now()

	}
	return true
}

func RemoveContractBoosterByMention(s *discordgo.Session, guildID string, channelID string, operator string, mention string) error {
	var contract = FindContract(guildID, channelID)
	if contract == nil {
		return errors.New(errorNoContract)
	}

	if contract.CoopSize == 0 {
		return errors.New(errorContractEmpty)
	}

	re := regexp.MustCompile(`[\\<>@#&!]`)
	var userID = re.ReplaceAllString(mention, "")

	var u, err = s.User(userID)
	if err != nil {
		return errors.New(errorNoFarmer)
	}
	if u.Bot {
		return errors.New(errorBot)
	}

	var found = false
	for i := range contract.Order {
		if contract.Order[i] == userID {
			found = true
			if removeContractBoosterByContract(s, contract, i+1) {
				contract.RegisteredNum -= 1
			}
			break
		}
	}
	if !found {
		return errors.New(errorUserNotInContract)
	}

	// Edit the boost List in place
	for _, loc := range contract.Location {
		msg, err := s.ChannelMessageEdit(loc.ChannelID, loc.ListMsgID, DrawBoostList(s, contract))
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

	if len(contract.Order) == 0 {
		return errors.New(errorContractEmpty)
	}
	if removeContractBoosterByContract(s, contract, index) {
		contract.RegisteredNum -= 1
	}

	// Remove the Boost List and thoen redisplay it
	for _, loc := range contract.Location {

		msg, err := s.ChannelMessageEdit(loc.ChannelID, loc.ListMsgID, DrawBoostList(s, contract))
		if err == nil {
			loc.ListMsgID = msg.ID
		}
	}
	return nil
}

func ReactionRemove(s *discordgo.Session, r *discordgo.MessageReaction) {
	var _, err = s.ChannelMessage(r.ChannelID, r.MessageID)
	if err != nil {
		return
	}
	var contract, _ = FindContractByReactionID(r.ChannelID, r.MessageID)
	if contract == nil {
		return
	}
	var farmer = contract.EggFarmers[r.UserID]
	if farmer == nil {
		return
	}

	if !userInContract(contract, r.UserID) {
		return
	}

	if r.Emoji.Name == "🎲" {
		contract.BoostVoting -= 1
		return
	}

	if r.Emoji.Name != "🧑‍🌾" && r.Emoji.Name != "🔔" && r.Emoji.Name != "🎲" {
		return
	}

	farmer.Reactions -= 1

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
		contract.RegisteredNum -= 1

		for _, loc := range contract.Location {
			msg, err := s.ChannelMessageEdit(loc.ChannelID, loc.ListMsgID, DrawBoostList(s, contract))
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

	if len(contract.Boosters) == 0 {
		return errors.New(errorContractEmpty)
	}

	if contract.BoostState != 0 {
		return errors.New(errorContractAlreadyStarted)
	}

	// Check Voting for Randomized Order
	// Supermajority 2/3
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

	reorderBoosters(contract)

	contract.BoostPosition = 0
	contract.BoostState = 1
	contract.StartTime = time.Now()
	contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = 1
	contract.Boosters[contract.Order[contract.BoostPosition]].StartTime = time.Now()

	sendNextNotification(s, contract, true)

	return nil
}

func sendNextNotification(s *discordgo.Session, contract *Contract, pingUsers bool) {
	// Start boosting contract
	for _, loc := range contract.Location {
		var msg *discordgo.Message
		var err error

		if contract.BoostState == 0 {
			msg, err = s.ChannelMessageEdit(loc.ChannelID, loc.ListMsgID, DrawBoostList(s, contract))
			if err != nil {
				fmt.Println("Unable to send this message")
			}
		} else {
			if contract.CoopSize == len(contract.Boosters) {
				s.ChannelMessageUnpin(loc.ChannelID, loc.ReactionID)
			}
			s.ChannelMessageDelete(loc.ChannelID, loc.ListMsgID)
			msg, err = s.ChannelMessageSend(loc.ChannelID, DrawBoostList(s, contract))
			loc.ListMsgID = msg.ID
		}
		if err == nil {
			fmt.Println("Unable to resend message.")
		}
		var str string = ""

		if contract.BoostState != 2 {
			s.MessageReactionAdd(loc.ChannelID, msg.ID, "🚀") // Booster
			if (contract.BoostPosition + 1) < len(contract.Order) {
				s.MessageReactionAdd(loc.ChannelID, msg.ID, "🔃")  // Swap
				s.MessageReactionAdd(loc.ChannelID, msg.ID, "⤵️") // Last
			}

			if pingUsers {
				str = fmt.Sprintf("Send Tokens to %s", contract.Boosters[contract.Order[contract.BoostPosition]].Mention)
			}
		} else {
			t1 := contract.EndTime
			t2 := contract.StartTime
			duration := t1.Sub(t2)
			str = fmt.Sprintf("Contract Boosting Complete in %s ", duration.Round(time.Second))
		}
		loc.ListMsgID = msg.ID
		s.ChannelMessageSend(loc.ChannelID, str)
	}
	if contract.BoostState == 2 {
		FinishContract(s, contract)
	} else {
		if pingUsers {
			notifyBellBoosters(s, contract)
		}
	}
}

// If
func BoostCommand(s *discordgo.Session, guildID string, channelID string, userID string) error {
	var contract = FindContract(guildID, channelID)

	if contract == nil {
		return errors.New(errorNoContract)
	}

	if contract.BoostState == 0 {
		return errors.New(errorContractEmpty)
	}

	if userID == contract.Order[contract.BoostPosition] {
		// User is using /boost command instead of reaction
		Boosting(s, guildID, channelID)
	} else {
		for i := range contract.Order {
			if contract.Order[i] == userID {
				if contract.Boosters[contract.Order[i]].BoostState == 2 {
					return errors.New(errorAlreadyBoosted)
				}
				// Mark user as complete
				// Taking start time from current booster start time
				contract.Boosters[contract.Order[i]].BoostState = 2
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

// Player has boosted
func Boosting(s *discordgo.Session, guildID string, channelID string) error {
	var contract = FindContract(guildID, channelID)
	if contract == nil {
		return errors.New(errorNoContract)
	}

	if contract.BoostState == 0 {
		return errors.New(errorContractNotStarted)
	}
	contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = 2
	contract.Boosters[contract.Order[contract.BoostPosition]].EndTime = time.Now()

	// Advance past any that have already boosted
	for contract.Boosters[contract.Order[contract.BoostPosition]].BoostState == 2 {
		contract.BoostPosition += 1
		if contract.BoostPosition == len(contract.Order) {
			break
		}
	}

	if contract.BoostPosition == contract.CoopSize || contract.BoostPosition == len(contract.Boosters) {
		contract.BoostState = 2 // Finished
		contract.EndTime = time.Now()
	} else {
		contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = 1
		contract.Boosters[contract.Order[contract.BoostPosition]].StartTime = time.Now()
	}

	sendNextNotification(s, contract, true)

	return nil
}

func SkipBooster(s *discordgo.Session, guildID string, channelID string, userID string) error {
	var boosterSwap = false
	var contract = FindContract(guildID, channelID)
	if contract == nil {
		return errors.New(errorNoContract)
	}

	if contract.BoostState == 0 {
		return errors.New(errorNotStarted)
	}

	var selectedUser = contract.BoostPosition

	if userID != "" {
		for i := range contract.Order {
			if contract.Order[i] == userID {
				selectedUser = i
				if contract.Boosters[contract.Order[i]].BoostState == 2 {
					return nil
				}
				break
			}
		}
	} else {
		boosterSwap = true
	}

	if selectedUser == contract.BoostPosition {
		contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = 0
		var skipped = contract.Order[contract.BoostPosition]

		if boosterSwap {
			contract.Order[contract.BoostPosition] = contract.Order[contract.BoostPosition+1]
			contract.Order[contract.BoostPosition+1] = skipped

		} else {
			contract.Order = RemoveIndex(contract.Order, contract.BoostPosition)
			contract.Order = append(contract.Order, skipped)
		}

		if contract.BoostPosition == contract.CoopSize || contract.BoostPosition == len(contract.Boosters) {
			contract.BoostState = 2 // Finished
			contract.EndTime = time.Now()
		} else {
			contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = 1
			contract.Boosters[contract.Order[contract.BoostPosition]].StartTime = time.Now()
		}
	} else {
		var skipped = contract.Order[selectedUser]
		contract.Order = RemoveIndex(contract.Order, selectedUser)
		contract.Order = append(contract.Order, skipped)
	}

	sendNextNotification(s, contract, true)

	return nil
}

func notifyBellBoosters(s *discordgo.Session, contract *Contract) {
	for i := range contract.Boosters {
		var farmer = contract.EggFarmers[contract.Boosters[i].UserID]
		if farmer.Ping {
			u, _ := s.UserChannelCreate(farmer.UserID)
			var str = fmt.Sprintf("%s: Send Boost Tokens to %s", farmer.ChannelName, contract.Boosters[contract.Order[contract.BoostPosition]].Name)
			_, err := s.ChannelMessageSend(u.ID, str)
			if err != nil {
				panic(err)
			}
		}
	}

}

// Called only when the contract is complete
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
	case 0:
		// Join Order
	case 1:
		// Reverse Order
		for i, j := 0, len(contract.Order)-1; i < j; i, j = i+1, j-1 {
			contract.Order[i], contract.Order[j] = contract.Order[j], contract.Order[i] //reverse the slice
		}
	case 2:
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
