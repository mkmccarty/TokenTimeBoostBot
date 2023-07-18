package boost

import (
	"errors"
	"fmt"
	"math/rand"
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
	startTime  time.Time // Time Farmer started boost turn
	endTime    time.Time // Time Farmer ended boost turn
}

type LocationData struct {
	guildID    string
	channelID  string // Contract Discord Channel
	messageID  string // Message ID for the Last Boost Order message
	reactionID string // Message ID for the reaction Order String
}
type Contract struct {
	contractHash string // ContractID-CoopID
	location     []*LocationData
	//guildID      string
	//channelID     string // Contract Discord Channel
	userID        string // Farmer Name of Creator
	contractID    string // Contract ID
	coopID        string // CoopID
	coopSize      int
	boostOrder    int
	boostVoting   int
	boostPosition int // Starting Slot
	boostState    int // Boost Completed
	//messageID     string // Message ID for the Last Boost Order message
	//reactionID    string // Message ID for the reaction Order String
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
				s.ChannelMessageDelete(el.channelID, el.messageID)
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
func StartContract(contractID string, coopID string, coopSize int, boostOrder int, guildID string, channelID string, userID string) (*Contract, error) {
	var new_contract = false
	var contractHash = fmt.Sprintf("%s-%s", contractID, coopID)

	contract := Contracts[contractHash]

	if contract == nil {
		// We don't have this contract on this channel, it could exist in another channel
		contract = new(Contract)
		loc := new(LocationData)
		loc.guildID = guildID
		loc.channelID = channelID
		loc.messageID = ""
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
			element.messageID = messageID
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

	outputStr = "# Boost List #\n"
	outputStr += fmt.Sprintf("### ID %s  Size %d ###\n", contract.contractHash, contract.coopSize)
	var i = 1
	for _, element := range contract.order {
		//for i := 1; i <= len(contract.Boosters); i++ {
		var b = contract.Boosters[element]
		switch b.boostState {
		case 0:
			outputStr += fmt.Sprintf("%2d -  %s\n", i, b.name)
		case 1:
			outputStr += fmt.Sprintf("%2d -  %s %s\n", i, b.name, tokenStr)
		case 2:
			t1 := contract.Boosters[element].endTime
			t2 := contract.Boosters[element].startTime
			diff := t1.Sub(t2)

			outputStr += fmt.Sprintf("%2d -  ~~%s~~  %s\n", i, b.name, fmt.Sprint(diff))
		}
		i += 1
	}
	for ; i <= contract.coopSize; i++ {
		outputStr += fmt.Sprintf("%2d -  \n", i)
	}
	outputStr += "\n"
	return outputStr
}

func FindContractByMessageID(channelID string, messageID string) (*Contract, int) {
	// Given a
	for _, c := range Contracts {
		for i := range c.location {
			if c.location[i].channelID == channelID && c.location[i].messageID == messageID {
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

func ReactionAdd(s *discordgo.Session, r *discordgo.MessageReaction) {
	// Find the message
	var _, err = s.ChannelMessage(r.ChannelID, r.MessageID)
	if err != nil {
		return
	}

	var contract, _ = FindContractByReactionID(r.ChannelID, r.MessageID)
	if contract == nil {
		return
	}

	// Remove extra added emoji
	if r.Emoji.Name != "🚀" && r.Emoji.Name != "🔔" && r.Emoji.Name != "🎲" {
		s.MessageReactionRemove(r.ChannelID, r.MessageID, r.Emoji.Name, r.UserID)
		return
	}

	if r.Emoji.Name == "🎲" {
		contract.boostVoting += 1
		return
	}

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

			msg, err := s.ChannelMessageEdit(contract.location[i].channelID, contract.location[i].messageID, DrawBoostList(s, contract))
			if err != nil {
				panic(err)
			}
			contract.location[i].messageID = msg.ID
		}

	}

	if r.Emoji.Name == "🔔" {
		farmer.ping = true
		u, _ := s.UserChannelCreate(farmer.userID)
		var str = fmt.Sprintf("Boost notifications will be sent for %s.", farmer.channelName)
		_, err := s.ChannelMessageSend(u.ID, str)
		if err != nil {
			panic(err)
		}
	}

	if contract.registeredNum == contract.coopSize {
		StartBoosting(s, contract.location[0].guildID, contract.location[0].channelID)
	}
}

func RemoveIndex(s []string, index int) []string {
	return append(s[:index], s[index+1:]...)
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

	farmer.reactions -= 1

	if r.Emoji.Name == "🔔" {
		farmer.ping = false
	}

	if farmer.reactions == 0 {
		// Remove farmer from boost list
		for i := range contract.order {
			if contract.order[i] == r.UserID {
				contract.order = RemoveIndex(contract.order, i)
				break
			}
		}

		delete(contract.Boosters, r.UserID)

		contract.registeredNum -= 1
		// Remove the Boost List and then redisplay it
		//s.ChannelMessageDelete(r.ChannelID, contract.messageID)
		msg, err := s.ChannelMessageEdit(r.ChannelID, contract.location[loc].messageID, DrawBoostList(s, contract))
		if err != nil {
			panic(err)
		}
		contract.location[loc].messageID = msg.ID

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

func StartBoosting(s *discordgo.Session, guildID string, channelID string) error {
	var contract = FindContract(guildID, channelID)
	if contract == nil {
		return errors.New("unable to locate a contract")
	}

	if len(contract.Boosters) == 0 {
		return errors.New("nobody signed up to boost")
	}

	// Check Voting for Randomized order
	if (contract.boostVoting * 2) > len(contract.Boosters) {
		contract.boostOrder = 2
	}
	reorderBoosters(contract)

	contract.boostPosition = 0
	contract.boostState = 1
	contract.Boosters[contract.order[contract.boostPosition]].boostState = 1
	contract.Boosters[contract.order[contract.boostPosition]].startTime = time.Now()

	sendNextNotification(s, contract)
	return nil
}

func sendNextNotification(s *discordgo.Session, contract *Contract) {
	// Start boosting contract
	for i := range contract.location {
		var msg *discordgo.Message
		var err error

		if contract.coopSize != len(contract.Boosters) {
			msg, err = s.ChannelMessageEdit(contract.location[i].channelID, contract.location[i].messageID, DrawBoostList(s, contract))

		} else {
			s.ChannelMessageUnpin(contract.location[i].channelID, contract.location[i].reactionID)
			s.ChannelMessageDelete(contract.location[i].channelID, contract.location[i].messageID)
			msg, err = s.ChannelMessageSend(contract.location[i].channelID, DrawBoostList(s, contract))
		}
		if err == nil {
			fmt.Println("Unable to resend message.")
		}
		var str string = ""

		if contract.boostState != 2 {
			str = fmt.Sprintf("Send Tokens to %s", contract.Boosters[contract.order[contract.boostPosition]].mention)
		} else {
			t1 := time.Now()
			t2 := contract.Boosters[contract.order[0]].startTime
			diff := t1.Sub(t2)
			str = fmt.Sprintf("Contract Boosting Complete in %s ", fmt.Sprint(diff))
		}
		contract.location[i].messageID = msg.ID
		s.ChannelMessageSend(contract.location[i].channelID, str)
	}
	if contract.boostState == 2 {
		FinishContract(s, contract)
	} else {
		notifyBellBoosters(s, contract)
	}
}

func NextBooster(s *discordgo.Session, guildID string, channelID string) error {
	var contract = FindContract(guildID, channelID)
	if contract == nil {
		return errors.New("unable to locate a contract")
	}

	if contract.boostState == 0 {
		return errors.New("contract not started")
	}
	contract.Boosters[contract.order[contract.boostPosition]].boostState = 2
	contract.Boosters[contract.order[contract.boostPosition]].endTime = time.Now()
	contract.boostPosition += 1
	if contract.boostPosition == contract.coopSize || contract.boostPosition == len(contract.Boosters) {
		contract.boostState = 2 // Finished
	} else {
		contract.Boosters[contract.order[contract.boostPosition]].boostState = 1
		contract.Boosters[contract.order[contract.boostPosition]].startTime = time.Now()
	}

	sendNextNotification(s, contract)
	// Start boosting contract

	return nil
}

func SkipBooster(s *discordgo.Session, guildID string, channelID string) error {
	var contract = FindContract(guildID, channelID)
	if contract == nil {
		return errors.New("unable to locate a contract")
	}

	if contract.boostState == 0 {
		return errors.New("contract not started")
	}

	contract.Boosters[contract.order[contract.boostPosition]].boostState = 0
	var skipped = contract.order[contract.boostPosition]
	contract.order = RemoveIndex(contract.order, contract.boostPosition)
	contract.order = append(contract.order, skipped)

	if contract.boostPosition == contract.coopSize || contract.boostPosition == len(contract.Boosters) {
		contract.boostState = 2 // Finished
	} else {
		contract.Boosters[contract.order[contract.boostPosition]].boostState = 1
		contract.Boosters[contract.order[contract.boostPosition]].startTime = time.Now()
	}

	// Start boosting contract
	sendNextNotification(s, contract)

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
	contract.location[0].messageID = ""
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
