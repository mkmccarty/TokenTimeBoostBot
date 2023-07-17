package boost

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	emutil "github.com/post04/discordgo-emoji-util"
)

//var usermutex sync.Mutex

type EggFarmer struct {
	userID    string    // Discord User ID
	guildID   string    // Discord Guild where this User is From
	reactions int       // Number of times farmer reacted
	ping      bool      // True/False
	register  time.Time // Time Farmer registered to boost
	start     time.Time // Time Farmer started boost turn
	end       time.Time // Time Farmer ended boost turn
}

type Booster struct {
	userID   string // Egg Farmer
	name     string
	order    int    // Position in Boost Order
	boosting bool   // Indicates if current booster
	mention  string // String which mentions user
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
	guildID      string
	channelID    string // Contract Discord Channel
	userID       string // Farmer Name of Creator
	contractID   string // Contract ID
	coopID       string // CoopID
	coopSize     int
	boostOrder   int
	position     int  // Starting Slot
	completed    bool // Boost Completed
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

type configStruct struct {
	DiscordToken   string `json:"DiscordToken"`
	DiscordAppID   string `json:"DiscordAppID"`
	DiscordGuildID string `json:"DiscordGuildID"`
}

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
		contract.guildID = guildID // starting guild
		contract.userID = userID   // starting userid
		contract.channelID = channelID
		contract.registeredNum = 0
		contract.coopSize = coopSize
		Contracts[contractHash] = contract
		new_contract = true

	} else {
		// Existing contract, make sure we know what server we're on
		loc := new(LocationData)
		loc.guildID = guildID
		loc.channelID = channelID
		loc.messageID = ""
		loc.reactionID = ""
		contract.location = append(contract.location, loc)
		//GlobalContracts[contractHash] = append(GlobalContracts[contractHash], loc)
	}

	/*
		farmer := contract.EggFarmers[userID]
		if farmer == nil {
			farmer = new(EggFarmer)
			farmer.register = time.Now()
			farmer.ping = false
			farmer.reactions = 0
			farmer.userID = userID
			farmer.guildID = guildID
			contract.EggFarmers[userID] = farmer
		}
	*/
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
			b.boosting = false
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
	g, _ := s.State.Guild(contract.guildID) // RAIYC Playground
	var e = emutil.FindEmoji(g.Emojis, "token", false)
	if e != nil {
		tokenStr = e.MessageFormat()
	}

	outputStr = "# Boost List #\n"
	outputStr += fmt.Sprintf("## %s ##\n", contract.contractHash)
	outputStr += fmt.Sprintf("### Contract Size: %d ###\n", contract.coopSize)
	var i = 1
	for _, element := range contract.order {
		//for i := 1; i <= len(contract.Boosters); i++ {
		var b = contract.Boosters[element]
		outputStr += fmt.Sprintf("%2d -  %s", i, b.name)
		if b.boosting {
			outputStr += fmt.Sprintf(" %s\n", tokenStr)
		} else {
			outputStr += "\n"
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
	var msg, err = s.ChannelMessage(r.ChannelID, r.MessageID)
	if err != nil {
		return
	}

	// Remove extra added emoji
	if r.Emoji.Name != "🚀" && r.Emoji.Name != "🔔" {
		s.MessageReactionRemove(r.ChannelID, r.MessageID, r.Emoji.Name, r.UserID)
		return
	}

	var contract, _ = FindContractByReactionID(r.ChannelID, r.MessageID)
	if contract == nil {
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
		contract.EggFarmers[r.UserID] = farmer
	}
	farmer.reactions += 1
	if farmer.reactions == 1 {
		// New Farmer - add them to boost list
		var b = new(Booster)
		b.userID = farmer.userID
		var user, err = s.User(r.UserID)
		if err == nil {
			b.name = user.Username
			b.boosting = false
			b.mention = user.Mention()
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

	//var mr = msg.Reactions
	fmt.Print(msg)

	if r.Emoji.Name == "🔔" {
		farmer.ping = true

		var ch, _ = s.Channel(contract.channelID)
		u, _ := s.UserChannelCreate(farmer.userID)
		var str = fmt.Sprintf("Boost notifications will be sent for %s.", ch.Name)
		_, err := s.ChannelMessageSend(u.ID, str)
		if err != nil {
			panic(err)
		}

	}

	// Verify that if the user reacted once or twice

}

func RemoveIndex(s []string, index int) []string {
	return append(s[:index], s[index+1:]...)
}

func ReactionRemove(s *discordgo.Session, r *discordgo.MessageReaction) {
	var msg, err = s.ChannelMessage(r.ChannelID, r.MessageID)
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

	farmer.reactions -= 1

	//var mr = msg.Reactions
	fmt.Print(msg)

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
