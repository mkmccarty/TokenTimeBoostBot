package boost

import (
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"time"

	"github.com/bwmarrin/discordgo"
	emutil "github.com/post04/discordgo-emoji-util"
)

//var usermutex sync.Mutex

type EggFarmer struct {
	userID      string // Discord User ID
	channelName string
	guildID     string    // Discord Guild where this User is From
	reactions   int       // Number of times farmer reacted
	ping        bool      // True/False
	register    time.Time // Time Farmer registered to boost
}

type Booster struct {
	userID     string // Egg Farmer
	name       string
	boostState int       // Indicates if current booster
	mention    string    // String which mentions user
	priority   bool      // Requested Early Boost
	later      bool      // Requested to Boost Later
	startTime  time.Time // Time Farmer started boost turn
	endTime    time.Time // Time Farmer ended boost turn
}

type LocationData struct {
	guildID     string
	guildName   string
	channelID   string // Contract Discord Channel
	channelName string
	listMsgID   string // Message ID for the Last Boost Order message
	reactionID  string // Message ID for the reaction Order String
}
type Contract struct {
	contractHash  string // ContractID-CoopID
	location      []*LocationData
	userID        string // Farmer Name of Creator
	contractID    string // Contract ID
	coopID        string // CoopID
	coopSize      int
	boostOrder    int
	boostVoting   int
	boostPosition int       // Starting Slot
	boostState    int       // Boost Completed
	startTime     time.Time // When Contract is started
	endTime       time.Time // When final booster ends
	EggFarmers    map[string]*EggFarmer
	registeredNum int
	Boosters      map[string]*Booster // Boosters Registered
	order         []string
}

var (
	// DiscordToken holds the API Token for discord.
	Contracts map[string]*Contract
	//GlobalContracts map[string][]*LocationData // Holds channel id's for various contracts
)

func init() {
	Contracts = make(map[string]*Contract)
	//GlobalContracts = make(map[string][]*LocationData)
}
func RemoveLocIndex(s []*LocationData, index int) []*LocationData {
	return append(s[:index], s[index+1:]...)
}

func DeleteContract(s *discordgo.Session, guildID string, channelID string) string {
	var coop = ""
	for key, element := range Contracts {
		for i, el := range element.location {
			if el.guildID == guildID && el.channelID == channelID {
				s.ChannelMessageDelete(el.channelID, el.listMsgID)
				s.ChannelMessageDelete(el.channelID, el.reactionID)
				element.location = RemoveLocIndex(element.location, i)
				coop = element.contractHash
			}
		}
		if len(element.location) == 0 {
			delete(Contracts, key)
			return coop
		}
	}
	return coop
}

// interface
func CreateContract(s *discordgo.Session, contractID string, coopID string, coopSize int, boostOrder int, guildID string, channelID string, userID string) (*Contract, error) {
	var new_contract = false
	var contractHash = fmt.Sprintf("%s/%s", contractID, coopID)

	contract := Contracts[contractHash]

	if contract == nil {
		// We don't have this contract on this channel, it could exist in another channel
		contract = new(Contract)
		loc := new(LocationData)
		loc.guildID = guildID
		loc.channelID = channelID
		var g, gerr = s.Guild(guildID)
		if gerr == nil {
			loc.guildName = g.Name

		}
		var c, cerr = s.Channel(channelID)
		if cerr == nil {
			loc.channelName = c.Name
		}
		loc.listMsgID = ""
		loc.reactionID = ""
		contract.location = append(contract.location, loc)
		contract.contractHash = contractHash

		//GlobalContracts[contractHash] = append(GlobalContracts[contractHash], loc)
		contract.EggFarmers = make(map[string]*EggFarmer)
		contract.Boosters = make(map[string]*Booster)
		contract.contractID = contractID
		contract.coopID = coopID
		contract.boostOrder = boostOrder
		contract.boostVoting = 0
		contract.boostState = 0
		contract.userID = userID // starting userid
		contract.registeredNum = 0
		contract.coopSize = coopSize
		Contracts[contractHash] = contract
		new_contract = true
	} else {
		// TODO Multi server isn't working because the Session Object is
		// specific to one Server/Guild
		//
		if contract.location[0].guildID != guildID {
			return nil, errors.New("contracts across servers not currently supported")
		}
		// Existing contract, make sure we know what server we're on
		/*
			loc := new(LocationData)
			loc.guildID = guildID
			loc.channelID = channelID
			loc.messageID = ""
			loc.reactionID = ""
			contract.location = append(contract.location, loc)
		*/
		//GlobalContracts[contractHash] = append(GlobalContracts[contractHash], loc)
	}
	new_contract = false

	if new_contract {
		// Create a bunch of test data
		for i := contract.registeredNum + 1; i < contract.coopSize; i++ {
			var fake_user = fmt.Sprintf("Test-%02d", i)
			var farmer = new(EggFarmer)
			farmer.register = time.Now()
			farmer.ping = false
			farmer.reactions = 0
			farmer.userID = fake_user
			farmer.guildID = guildID
			contract.EggFarmers[farmer.userID] = farmer

			var b = new(Booster)
			b.userID = fake_user
			b.name = fake_user
			b.boostState = 0
			b.startTime = time.Now()
			b.mention = fake_user

			contract.Boosters[farmer.userID] = b
			contract.order = append(contract.order, fake_user)
			contract.registeredNum += 1
		}
	}

	return contract, nil
}

func SetMessageID(contract *Contract, channelID string, messageID string) {
	for _, element := range contract.location {
		if element.channelID == channelID {
			element.listMsgID = messageID
		}
	}
}

func SetReactionID(contract *Contract, channelID string, messageID string) {
	for _, element := range contract.location {
		if element.channelID == channelID {
			element.reactionID = messageID
		}
	}
}

func DrawBoostList(s *discordgo.Session, contract *Contract) string {
	var outputStr string
	var tokenStr = "<:token:778019329693450270>"
	g, _ := s.State.Guild(contract.location[0].guildID) // RAIYC Playground
	var e = emutil.FindEmoji(g.Emojis, "token", false)
	if e != nil {
		tokenStr = e.MessageFormat()
	}
	outputStr = fmt.Sprintf("### %s  %d/%d ###\n", contract.contractHash, len(contract.Boosters), contract.coopSize)

	if contract.boostState == 0 {
		outputStr += "## Signup List ###\n"
	} else {
		outputStr += "## Boost List ###\n"
	}
	var i = 1
	var prefix = " - "
	for _, element := range contract.order {
		if contract.boostState != 0 {
			prefix = fmt.Sprintf("%2d - ", i)
		}
		//for i := 1; i <= len(contract.Boosters); i++ {
		var b = contract.Boosters[element]
		switch b.boostState {
		case 0:
			outputStr += fmt.Sprintf("%s %s\n", prefix, b.name)
		case 1:
			outputStr += fmt.Sprintf("%s %s %s\n", prefix, b.name, tokenStr)
		case 2:
			t1 := contract.Boosters[element].endTime
			t2 := contract.Boosters[element].startTime
			duration := t1.Sub(t2)
			outputStr += fmt.Sprintf("%s ~~%s~~  %s\n", prefix, b.name, duration.Round(time.Second))
		}
		i += 1
	}

	// Only draw empty slots when contract is active
	if contract.boostState != 2 {
		for ; i <= contract.coopSize; i++ {
			outputStr += fmt.Sprintf("%s  open position\n", prefix)
		}
	}
	if contract.boostState == 1 {
		outputStr += "```"
		outputStr += "React with 🚀 when you spend tokens to boost. Multiple 🚀 votes by others in the contract will also indicate a boost.\n"
		if (contract.boostPosition + 1) < len(contract.order) {
			outputStr += "React with 🔃 to exchange position with the next booster.\nReact with ⤵️ to move to last."
		}
		outputStr += "```"
	}
	return outputStr
}

func FindContractByMessageID(channelID string, messageID string) (*Contract, int) {
	// Given a
	for _, c := range Contracts {
		for i := range c.location {
			if c.location[i].channelID == channelID && c.location[i].listMsgID == messageID {
				return c, i
			}
		}
	}
	return nil, 0
}

func FindContractByReactionID(channelID string, reactionID string) (*Contract, int) {
	// Given a
	for _, c := range Contracts {
		for i := range c.location {
			if c.location[i].channelID == channelID && c.location[i].reactionID == reactionID {
				return c, i
			}
		}
	}
	return nil, 0
}

func AddContractMember(s *discordgo.Session, guildID string, channelID string, mention string) error {
	var contract = FindContract(guildID, channelID)
	if contract == nil {
		return errors.New("unable to locate a contract")
	}

	if contract.coopSize == len(contract.order) {
		return errors.New("contract is full")
	}

	re := regexp.MustCompile(`[\\<>@#&!]`)
	var userID = re.ReplaceAllString(mention, "")

	for i := range contract.order {
		if userID == contract.order[i] {
			return errors.New("user alread in contract")
		}
	}

	var u, err = s.User(userID)
	if err != nil {
		return errors.New("user not found")
	}
	if u.Bot {
		return errors.New("cannot add a bot")
	}

	var farmer, fe = AddFarmerToContract(s, contract, guildID, channelID, u.ID)
	if fe == nil {
		// Need to rest the farmer reaction count when added this way
		farmer.reactions = 0
	}

	return nil
}

func AddFarmerToContract(s *discordgo.Session, contract *Contract, guildID string, channelID string, userID string) (*EggFarmer, error) {
	var err error
	var farmer = contract.EggFarmers[userID]
	if farmer == nil {
		// New Farmer
		farmer = new(EggFarmer)
		farmer.register = time.Now()
		farmer.ping = false
		farmer.reactions = 0
		farmer.userID = userID
		farmer.guildID = guildID
		var ch, _ = s.Channel(channelID)
		farmer.channelName = ch.Name

		contract.EggFarmers[userID] = farmer
	}
	farmer.reactions += 1
	var b = contract.Boosters[userID]
	if b == nil {
		// New Farmer - add them to boost list
		var b = new(Booster)
		b.userID = farmer.userID
		b.priority = false
		b.later = false
		var user, _ = s.User(userID)
		if err == nil {
			b.name = user.Username
			b.boostState = 0
			b.mention = user.Mention()
		}
		var member, err = s.GuildMember(guildID, userID)
		if err == nil && member.Nick != "" {
			b.name = member.Nick
			b.mention = member.Mention()
		}
		contract.Boosters[farmer.userID] = b
		contract.order = append(contract.order, farmer.userID)
		contract.registeredNum += 1

		// Remove the Boost List and then redisplay it
		//s.ChannelMessageDelete(r.ChannelID, contract.messageID)
		for i := range contract.location {

			msg, err := s.ChannelMessageEdit(contract.location[i].channelID, contract.location[i].listMsgID, DrawBoostList(s, contract))
			if err != nil {
				panic(err)
			}
			contract.location[i].listMsgID = msg.ID
		}

	}
	if contract.registeredNum == contract.coopSize {
		StartContractBoosting(s, contract.location[0].guildID, contract.location[0].channelID)
	}

	return farmer, nil
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

	if contract.boostState != 0 && contract.boostPosition < len(contract.order) {

		// If Rocket reaction on Boost List, only that boosting user can apply a reaction
		if r.Emoji.Name == "🚀" && contract.boostState == 1 || r.Emoji.Name == "🪨" && r.UserID == contract.userID {
			var votingElection = (msg.Reactions[0].Count - 1) >= 2
			if r.Emoji.Name == "🪨" && r.UserID == contract.userID {
				votingElection = true
			}
			//msg.Reactions[0],count

			if r.UserID == contract.order[contract.boostPosition] || votingElection {
				Boosting(s, r.GuildID, r.ChannelID)
			}
			return
		}

		// Reaction to change places
		if (contract.boostPosition + 1) < len(contract.order) {
			if r.Emoji.Name == "🔃" && r.UserID == contract.order[contract.boostPosition] {
				SkipBooster(s, r.GuildID, r.ChannelID, "")
				return
			}
			// Reaction to jump to end
			if r.Emoji.Name == "⤵️" && r.UserID == contract.order[contract.boostPosition] {
				SkipBooster(s, r.GuildID, r.ChannelID, r.UserID)
				return
			}
		}
	}

	// Remove extra added emoji
	if r.Emoji.Name != "🧑‍🌾" && r.Emoji.Name != "🔔" && r.Emoji.Name != "🎲" {
		s.MessageReactionRemove(r.ChannelID, r.MessageID, r.Emoji.Name, r.UserID)
		return
	}

	if r.Emoji.Name == "🎲" {
		contract.boostVoting += 1
		return
	}

	var farmer, e = AddFarmerToContract(s, contract, r.GuildID, r.ChannelID, r.UserID)
	if e == nil {
		if r.Emoji.Name == "🔔" {
			farmer.ping = true
			u, _ := s.UserChannelCreate(farmer.userID)
			var str = fmt.Sprintf("Boost notifications will be sent for %s.", contract.contractHash)
			_, err := s.ChannelMessageSend(u.ID, str)
			if err != nil {
				panic(err)
			}
		}
	}
	/*
		var farmer = contract.EggFarmers[r.UserID]
		if farmer == nil {
			// New Farmer
			farmer = new(EggFarmer)
			farmer.register = time.Now()
			farmer.ping = false
			farmer.reactions = 0
			farmer.userID = r.UserID
			farmer.guildID = r.GuildID
			var ch, _ = s.Channel(r.ChannelID)
			farmer.channelName = ch.Name

			contract.EggFarmers[r.UserID] = farmer
		}
		farmer.reactions += 1
		if farmer.reactions == 1 {
			// New Farmer - add them to boost list
			var b = new(Booster)
			b.userID = farmer.userID
			b.priority = false
			b.later = false
			var user, _ = s.User(r.UserID)
			if err == nil {
				b.name = user.Username
				b.boostState = 0
				b.mention = user.Mention()
			}
			var member, err = s.GuildMember(r.GuildID, r.UserID)
			if err == nil && member.Nick != "" {
				b.name = member.Nick
				b.mention = member.Mention()
			}
			contract.Boosters[farmer.userID] = b
			contract.order = append(contract.order, farmer.userID)
			contract.registeredNum += 1

			// Remove the Boost List and then redisplay it
			//s.ChannelMessageDelete(r.ChannelID, contract.messageID)
			for i := range contract.location {

				msg, err := s.ChannelMessageEdit(contract.location[i].channelID, contract.location[i].listMsgID, DrawBoostList(s, contract))
				if err != nil {
					panic(err)
				}
				contract.location[i].listMsgID = msg.ID
			}

		}
	*/
	/*
		if contract.registeredNum == contract.coopSize {
			StartContractBoosting(s, contract.location[0].guildID, contract.location[0].channelID)
		}
	*/
}

func RemoveIndex(s []string, index int) []string {
	return append(s[:index], s[index+1:]...)
}

func removeContractBoosterByContract(s *discordgo.Session, contract *Contract, offset int) bool {
	if offset > len(contract.Boosters) {
		return false
	}
	var index = offset - 1 // Index is 0 based

	var activeBooster = contract.Boosters[contract.order[index]].boostState
	delete(contract.Boosters, contract.order[index])
	contract.order = RemoveIndex(contract.order, index)

	// Active Booster is leaving contract.
	if (activeBooster == 1) && len(contract.order) > index {
		contract.Boosters[contract.order[index]].boostState = 2
		contract.Boosters[contract.order[index]].startTime = time.Now()

	}
	return true
}

func RemoveContractBooster(s *discordgo.Session, guildID string, channelID string, index int) error {
	var contract = FindContract(guildID, channelID)

	if contract == nil {
		return errors.New("unable to locate a contract")
	}

	if len(contract.order) == 0 {
		return errors.New("nobody signed up to boost")
	}
	if removeContractBoosterByContract(s, contract, index) {
		contract.registeredNum -= 1
	}

	// Remove the Boost List and then redisplay it
	msg, err := s.ChannelMessageEdit(contract.location[0].channelID, contract.location[0].listMsgID, DrawBoostList(s, contract))
	if err != nil {
		return err
	}

	contract.location[0].listMsgID = msg.ID
	return nil
}

func ReactionRemove(s *discordgo.Session, r *discordgo.MessageReaction) {
	var _, err = s.ChannelMessage(r.ChannelID, r.MessageID)
	if err != nil {
		return
	}
	var contract, loc = FindContractByReactionID(r.ChannelID, r.MessageID)
	if contract == nil {
		return
	}
	var farmer = contract.EggFarmers[r.UserID]
	if farmer == nil {
		return
	}

	if r.Emoji.Name == "🎲" {
		contract.boostVoting -= 1
		return
	}

	if r.Emoji.Name != "🧑‍🌾" && r.Emoji.Name != "🔔" && r.Emoji.Name != "🎲" {
		return
	}

	farmer.reactions -= 1

	if r.Emoji.Name == "🔔" {
		farmer.ping = false
	}

	if farmer.reactions == 0 {
		// Remove farmer from boost list
		for i := range contract.order {
			if contract.order[i] == r.UserID {
				removeContractBoosterByContract(s, contract, i+1)
				break
			}
		}
		contract.registeredNum -= 1
		// Remove the Boost List and then redisplay it
		msg, err := s.ChannelMessageEdit(r.ChannelID, contract.location[loc].listMsgID, DrawBoostList(s, contract))
		if err != nil {
			panic(err)
		}
		contract.location[loc].listMsgID = msg.ID

	}
}

func FindContract(guildID string, channelID string) *Contract {
	// Look for the contract
	for key, element := range Contracts {
		for _, el := range element.location {
			if el.guildID == guildID && el.channelID == channelID {
				return Contracts[key]
			}
		}
	}
	return nil
}

func StartContractBoosting(s *discordgo.Session, guildID string, channelID string) error {
	var contract = FindContract(guildID, channelID)
	if contract == nil {
		return errors.New("unable to locate a contract")
	}

	if len(contract.Boosters) == 0 {
		return errors.New("nobody signed up to boost")
	}

	if contract.boostState != 0 {
		return errors.New("Contract already started")
	}

	// Check Voting for Randomized order
	// Supermajority 2/3
	if contract.boostVoting > ((len(contract.Boosters) * 2) / 3) {
		contract.boostOrder = 2
	}
	reorderBoosters(contract)

	contract.boostPosition = 0
	contract.boostState = 1
	contract.startTime = time.Now()
	contract.Boosters[contract.order[contract.boostPosition]].boostState = 1
	contract.Boosters[contract.order[contract.boostPosition]].startTime = time.Now()

	sendNextNotification(s, contract, true)
	return nil
}

func sendNextNotification(s *discordgo.Session, contract *Contract, pingUsers bool) {
	// Start boosting contract
	for i := range contract.location {
		var msg *discordgo.Message
		var err error

		if contract.boostState == 0 {
			msg, err = s.ChannelMessageEdit(contract.location[i].channelID, contract.location[i].listMsgID, DrawBoostList(s, contract))
			if err != nil {
				fmt.Println("Unable to send this message")
			}
		} else {
			if contract.coopSize == len(contract.Boosters) {
				s.ChannelMessageUnpin(contract.location[i].channelID, contract.location[i].reactionID)
			}
			s.ChannelMessageDelete(contract.location[i].channelID, contract.location[i].listMsgID)
			msg, err = s.ChannelMessageSend(contract.location[i].channelID, DrawBoostList(s, contract))
			contract.location[i].listMsgID = msg.ID
			//s.ChannelMessagePin(contract.location[i].channelID, contract.location[i].messageID)
		}
		if err == nil {
			fmt.Println("Unable to resend message.")
		}
		var str string = ""

		if contract.boostState != 2 {
			s.MessageReactionAdd(contract.location[i].channelID, msg.ID, "🚀") // Booster
			if (contract.boostPosition + 1) < len(contract.order) {
				s.MessageReactionAdd(contract.location[i].channelID, msg.ID, "🔃")  // Swap
				s.MessageReactionAdd(contract.location[i].channelID, msg.ID, "⤵️") // Last
			}

			if pingUsers {
				str = fmt.Sprintf("Send Tokens to %s", contract.Boosters[contract.order[contract.boostPosition]].mention)
			}
		} else {
			t1 := contract.endTime
			t2 := contract.startTime
			duration := t1.Sub(t2)
			str = fmt.Sprintf("Contract Boosting Complete in %s ", duration.Round(time.Second))
		}
		contract.location[i].listMsgID = msg.ID
		s.ChannelMessageSend(contract.location[i].channelID, str)
	}
	if contract.boostState == 2 {
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
		return errors.New("unable to locate a contract")
	}

	if contract.boostState == 0 {
		return errors.New("contract not started")
	}

	if userID == contract.order[contract.boostPosition] {
		// User is using /boost command instead of reaction
		Boosting(s, guildID, channelID)
	} else {
		for i := range contract.order {
			if contract.order[i] == userID {
				if contract.Boosters[contract.order[i]].boostState == 2 {
					return errors.New("you have already boosted")
				}
				// Mark user as complete
				// Taking start time from current booster start time
				contract.Boosters[contract.order[i]].boostState = 2
				contract.Boosters[contract.order[i]].startTime = contract.Boosters[contract.order[contract.boostPosition]].startTime
				contract.Boosters[contract.order[i]].endTime = time.Now()
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
		return errors.New("unable to locate a contract")
	}

	if contract.boostState == 0 {
		return errors.New("contract not started")
	}
	contract.Boosters[contract.order[contract.boostPosition]].boostState = 2
	contract.Boosters[contract.order[contract.boostPosition]].endTime = time.Now()

	// Advance past any that have already boosted
	for contract.Boosters[contract.order[contract.boostPosition]].boostState == 2 {
		contract.boostPosition += 1
		if contract.boostPosition == len(contract.order) {
			break
		}
	}

	if contract.boostPosition == contract.coopSize || contract.boostPosition == len(contract.Boosters) {
		contract.boostState = 2 // Finished
		contract.endTime = time.Now()
	} else {
		contract.Boosters[contract.order[contract.boostPosition]].boostState = 1
		contract.Boosters[contract.order[contract.boostPosition]].startTime = time.Now()
	}

	sendNextNotification(s, contract, true)

	return nil
}

func SkipBooster(s *discordgo.Session, guildID string, channelID string, userID string) error {
	var boosterSwap = false
	var contract = FindContract(guildID, channelID)
	if contract == nil {
		return errors.New("unable to locate a contract")
	}

	if contract.boostState == 0 {
		return errors.New("contract not started")
	}

	var selectedUser = contract.boostPosition

	if userID != "" {
		for i := range contract.order {
			if contract.order[i] == userID {
				selectedUser = i
				if contract.Boosters[contract.order[i]].boostState == 2 {
					return nil
				}
				break
			}
		}
	} else {
		boosterSwap = true
	}

	if selectedUser == contract.boostPosition {
		contract.Boosters[contract.order[contract.boostPosition]].boostState = 0
		var skipped = contract.order[contract.boostPosition]

		if boosterSwap {
			contract.order[contract.boostPosition] = contract.order[contract.boostPosition+1]
			contract.order[contract.boostPosition+1] = skipped

		} else {
			contract.order = RemoveIndex(contract.order, contract.boostPosition)
			contract.order = append(contract.order, skipped)
		}

		if contract.boostPosition == contract.coopSize || contract.boostPosition == len(contract.Boosters) {
			contract.boostState = 2 // Finished
			contract.endTime = time.Now()
		} else {
			contract.Boosters[contract.order[contract.boostPosition]].boostState = 1
			contract.Boosters[contract.order[contract.boostPosition]].startTime = time.Now()
		}
	} else {
		var skipped = contract.order[selectedUser]
		contract.order = RemoveIndex(contract.order, selectedUser)
		contract.order = append(contract.order, skipped)
	}

	sendNextNotification(s, contract, true)

	return nil
}

func notifyBellBoosters(s *discordgo.Session, contract *Contract) {
	for i := range contract.Boosters {
		var farmer = contract.EggFarmers[contract.Boosters[i].userID]
		if farmer.ping {
			u, _ := s.UserChannelCreate(farmer.userID)
			var str = fmt.Sprintf("%s: Send Boost Tokens to %s", farmer.channelName, contract.Boosters[contract.order[contract.boostPosition]].name)
			_, err := s.ChannelMessageSend(u.ID, str)
			if err != nil {
				panic(err)
			}
		}
	}

}

func FinishContract(s *discordgo.Session, contract *Contract) error {
	// Don't delete the final boost message
	contract.location[0].listMsgID = ""
	DeleteContract(s, contract.location[0].guildID, contract.location[0].channelID)
	return nil
}

func reorderBoosters(contract *Contract) {
	switch contract.boostOrder {
	case 0:
		// Join Order
	case 1:
		// Reverse order
		for i, j := 0, len(contract.order)-1; i < j; i, j = i+1, j-1 {
			contract.order[i], contract.order[j] = contract.order[j], contract.order[i] //reverse the slice
		}
	case 2:
		rand.Shuffle(len(contract.order), func(i, j int) {
			contract.order[i], contract.order[j] = contract.order[j], contract.order[i]
		})

	}
}
