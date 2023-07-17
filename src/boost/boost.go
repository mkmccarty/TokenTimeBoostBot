package boost

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
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
	userID  string // Egg Farmer
	name    string
	order   int    // Position in Boost Order
	mention string // String which mentions user
}

type Contract struct {
	guildID    string
	channelID  string // Contract Discord Channel
	userID     string // Farmer Name of Creator
	contractID string // Contract ID
	coopID     string // CoopID
	coopSize   int
	boostOrder int
	position   int    // Starting Slot
	completed  bool   // Boost Completed
	messageID  string // Message ID for the Last Boost Order message
	reactionID string // Message ID for the reaction Order String
	EggFarmers map[string]*EggFarmer

	registeredNum int
	Boosters      map[string]*Booster // Boosters Registered
}

var (
	// DiscordToken holds the API Token for discord.
	Contracts map[string]*Contract
)

type configStruct struct {
	DiscordToken   string `json:"DiscordToken"`
	DiscordAppID   string `json:"DiscordAppID"`
	DiscordGuildID string `json:"DiscordGuildID"`
}

func init() {
	Contracts = make(map[string]*Contract)
}

// interface
func StartContract(contractID string, coopID string, coopSize int, boostOrder int, guildID string, channelID string, userID string) (*Contract, error) {

	contractHash := guildID + "_" + channelID
	contract := Contracts[contractHash]
	if contract == nil {
		contract = new(Contract)
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
	}

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
	return contract, nil
}

func SetMessageID(contract *Contract, messageID string) {
	contract.messageID = messageID
}

func SetReactionID(contract *Contract, messageID string) {
	contract.reactionID = messageID
}

func DrawBoostList(contract *Contract) string {
	var outputStr string
	//var tokenStr = "<:token:778019329693450270>"
	outputStr = "# Boost List #\n"
	outputStr += fmt.Sprintf("### Contract Size: %d ###\n", contract.coopSize)
	var i = 1
	for _, element := range contract.Boosters {
		//for i := 1; i <= len(contract.Boosters); i++ {
		outputStr += fmt.Sprintf("%2d -  %s\n", i, element.name)
		i += 1
	}
	for ; i <= contract.coopSize; i++ {
		outputStr += fmt.Sprintf("%2d -  \n", i)
	}
	outputStr += "\n"
	return outputStr
}

func ReactionAdd(s *discordgo.Session, r *discordgo.MessageReaction) {
	// Find the message
	var msg, err = s.ChannelMessage(r.ChannelID, r.MessageID)
	if err != nil {
		return
	}
	contractHash := r.GuildID + "_" + r.ChannelID
	var contract = Contracts[contractHash]
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
			b.mention = user.Mention()
		}
		contract.Boosters[farmer.userID] = b
		contract.registeredNum += 1

		// Remove the Boost List and then redisplay it
		//s.ChannelMessageDelete(r.ChannelID, contract.messageID)
		msg, err := s.ChannelMessageEdit(r.ChannelID, contract.messageID, DrawBoostList(contract))
		if err != nil {
			panic(err)
		}
		contract.messageID = msg.ID

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

func ReactionRemove(s *discordgo.Session, r *discordgo.MessageReaction) {
	var msg, err = s.ChannelMessage(r.ChannelID, r.MessageID)
	if err != nil {
		return
	}
	contractHash := r.GuildID + "_" + r.ChannelID
	var contract = Contracts[contractHash]
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
		delete(contract.Boosters, r.UserID)
		contract.registeredNum -= 1
		// Remove the Boost List and then redisplay it
		//s.ChannelMessageDelete(r.ChannelID, contract.messageID)
		msg, err := s.ChannelMessageEdit(r.ChannelID, contract.messageID, DrawBoostList(contract))
		if err != nil {
			panic(err)
		}
		contract.messageID = msg.ID

	}
}
