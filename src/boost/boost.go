package boost

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/divan/num2words"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/track"
	"google.golang.org/protobuf/proto"
)

var mutex sync.Mutex

//const boostBotHomeGuild string = "766330702689992720"

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

//const errorContractNotWaiting = "contract not in waiting state"

// Create a slice with the names of the ContractState const names
var contractStateNames = []string{
	"ContractStateSignup",
	"ContractStateFastrun",
	"ContractStateWaiting",
	"ContractStateCompleted",
	"ContractStateArchive",
	"ContractStateCRT",
	"ContractStateBanker",
}

// Constnts for the contract
const (
	ContractOrderSignup    = 0 // Signup order
	ContractOrderReverse   = 1 // Reverse order
	ContractOrderRandom    = 2 // Randomized when the contract starts. After 20 minutes the order changes to Sign-up.
	ContractOrderFair      = 3 // Fair based on position percentile of each farmers last 5 contracts. Those with no history use 50th percentile
	ContractOrderTimeBased = 4 // Time based order
	ContractOrderELR       = 5 // ELR based order
	ContractOrderTVal      = 6 // Token Value based order

	ContractStateSignup    = 0 // Contract is in signup phase
	ContractStateFastrun   = 1 // Contract in Boosting as fastrun
	ContractStateWaiting   = 2 // Waiting for other(s) to join
	ContractStateCompleted = 3 // Contract is completed
	ContractStateArchive   = 4 // Contract is ready to archive
	ContractStateCRT       = 5 // Contract is doing CRT
	ContractStateBanker    = 6 // Contract is Boosting with Banker

	BoostStateUnboosted = 0 // Unboosted
	BoostStateTokenTime = 1 // TokenTime or turn to receive tokens
	BoostStateBoosted   = 2 // Boosted

	BoostOrderTimeThreshold = 20 // minutes to switch from random or fair to signup

	// These are used for the /speedrun command
	SpeedrunStyleBanker  = 0
	SpeedrunStyleFastrun = 1

	SinkBoostFollowOrder = -1 // Follow the order
	SinkBoostFirst       = 0  // First position
	SinkBoostLast        = 1  // Last position

	// These are an int64 flaglist to construct the style of the contract
	ContractFlagNone          = 0x0000
	ContractFlagCrt           = 0x0001
	ContractFlagSelfRuns      = 0x0002
	ContractFlag6Tokens       = 0x0100
	ContractFlag8Tokens       = 0x0200
	ContractFlagDynamicTokens = 0x0400
	ContractFlagFastrun       = 0x4000
	ContractFlagBanker        = 0x8000

	ContractStyleFastrun           = ContractFlagFastrun
	ContractStyleFastrunBanker     = ContractFlagBanker
	ContractStyleSpeedrunBoostList = ContractFlagCrt | ContractFlagFastrun
	ContractStyleSpeedrunBanker    = ContractFlagCrt | ContractFlagBanker
)

const defaultFamerTokens = 6

var boostIconName = "🚀"     // For Reaction tests
var boostIconReaction = "🚀" // For displaying
var boostIcon = "🚀"         // For displaying

// CompMap is a cached set of components for this contract
type CompMap struct {
	Emoji          string
	ID             string
	Style          discordgo.ButtonStyle
	CustomID       string
	ComponentEmoji *discordgo.ComponentEmoji
}

// TokenUnit holds the data for each token
type TokenUnit struct {
	Time   time.Time // Time token was received
	Value  float64   // Last calculated value of the token
	UserID string    // Who sent or received the token
	Serial string    // Serial number of the token
}

// BotTimer holds the data for each timer
type BotTimer struct {
	ID        string // Unique ID for this timer
	Reminder  time.Time
	timer     *time.Timer
	Message   string
	UserID    string
	ChannelID string
	MsgID     string
	Active    bool
}

// ArtifactSet holds the data for each set of artifacts
type ArtifactSet struct {
	Artifacts []ei.Artifact
	LayRate   float64
	ShipRate  float64
}

// Booster holds the data for each booster within a Contract
type Booster struct {
	UserID      string // Egg Farmer
	GlobalName  string
	ChannelName string
	GuildID     string // Discord Guild where this User is From
	GuildName   string
	Ping        bool      // True/False
	Register    time.Time //o Time Farmer registered to boost

	Name          string
	Unique        string
	Nick          string
	Mention       string   // String which mentions user
	Alts          []string // Array of alternate ids for the user
	AltsIcons     []string // Array of alternate icons for the user
	AltController string   // User ID of the controller of this alternate

	BoostState             int           // Indicates if current booster
	TokensReceived         int           // indicate number of boost tokens
	TokensWanted           int           // indicate number of boost tokens
	TokenValue             float64       // Current Token Value
	StartTime              time.Time     // Time Farmer started boost turn
	EndTime                time.Time     // Time Farmer ended boost turn
	Duration               time.Duration // Duration of boost
	BoostingTokenTimestamp time.Time     // When the boosting token was last received
	VotingList             []string      // Record list of those that voted to boost
	RunChickensTime        time.Time     // Time Farmer triggered chicken run reaction
	RanChickensOn          []string      // Array of users that the farmer ran chickens on
	BoostTriggerTime       time.Time     // Used for time remaining in boost
	Hint                   []string      // Used to track which hints have been given
	ArtifactSet            ArtifactSet   // Set of artifacts for this booster
	EstDurationOfBoost     time.Duration // Estimated duration of the boost
	EstEndOfBoost          time.Time     // Estimated end of the boost
	EstRequestChickenRuns  time.Time     // Estimated time to request chicken runs
}

// LocationData holds server specific Data for a contract
type LocationData struct {
	GuildID           string
	GuildName         string
	ChannelID         string // Contract Discord Channel
	ChannelMention    string
	ChannelPing       string
	ListMsgID         string   // Message ID for the Last Boost Order message
	ReactionID        string   // Message ID for the reaction Order String
	MessageIDs        []string // Array of message IDs for any contract message
	TokenXStr         string   // Emoji for Token
	TokenXReactionStr string   // Emoji for Token Reaction
}

// BankerInfo holds information about contract Banker
type BankerInfo struct {
	CurrentBanker      string // Current Banker
	CrtSinkUserID      string // Sink CRT User ID
	BoostingSinkUserID string // Sink CRT User ID
	PostSinkUserID     string // Sink End of Contract User ID
	SinkBoostPosition  int    // Sink Boost Position
}

// Contract is the main struct for each contract
type Contract struct {
	ContractHash string // ContractID-CoopID
	Location     []*LocationData
	CreatorID    []string // Slice of creators
	//SignupMsgID    map[string]string // Message ID for the Signup Message
	ContractID string // Contract ID
	CoopID     string // CoopID

	Name                      string
	Description               string
	Egg                       int32
	EggName                   string
	EggEmoji                  string
	TokenStr                  string // Emoji for Token
	TargetAmount              []float64
	QTargetAmount             []float64
	ChickenRunCooldownMinutes int
	MinutesPerToken           int
	EstimatedDuration         time.Duration
	CompletionDuration        time.Duration
	EstimatedEndTime          time.Time
	EstimatedDurationValid    bool
	ThreadName                string

	CRMessageIDs []string // Array of message IDs for chicken run messages

	CoopSize            int
	Style               int64 // Mask for the Contract Style
	LengthInSeconds     int
	BoostOrder          int // How the contract is sorted
	BoostVoting         int
	BoostPosition       int       // Starting Slot
	State               int       // Boost Completed
	StartTime           time.Time // When Contract is started
	EndTime             time.Time // When final booster ends
	PlannedStartTime    time.Time // Parameter start time
	ActualStartTime     time.Time // Actual start time for token tracking
	RegisteredNum       int
	Boosters            map[string]*Booster // Boosters Registered
	AltIcons            []string            // Array of alternate icons for the Boosters
	Order               []string
	BoostedOrder        []string // Actual order of boosting
	OrderRevision       int      // Incremented when Order is changed
	Speedrun            bool     // Speedrun mode
	SRData              SpeedrunData
	Banker              BankerInfo // Banker for the contract
	TokenLog            []ei.TokenUnitLog
	CalcOperations      int
	CalcOperationTime   time.Time
	CoopTokenValueMsgID string
	LastWishPrompt      string    // saved prompt for this contract
	LastInteractionTime time.Time // last time the contract was drawn
	//UseInteractionButtons bool               // Use buttons for interaction
	buttonComponents map[string]CompMap // Cached components for this contract
	SavedStats       bool               // Saved stats for this contract
	NewFeature       int                // Used to slide in new features
	DynamicData      *DynamicTokenData

	mutex sync.Mutex // Keep this contract thread safe
}

// SpeedrunData holds the data for a speedrun
type SpeedrunData struct {
	ChickenRuns          int      // Number of Chicken Runs for this contract
	Legs                 int      // Number of legs for this Tango
	Tango                [3]int   // The Tango itself (First, Middle, Last)
	CurrentLeg           int      // Current Leg
	LegReactionMessageID string   // Message ID for the Leg Reaction Message
	ChickenRunCheckMsgID string   // Message ID for the Chicken Run Check Message
	NeedToRunChickens    []string // Array of users that the farmer ran chickens on
	StatusStr            string   // Status string for the speedrun
}

type eiData struct {
	ID                  string
	timestamp           time.Time
	expirationTimestamp time.Time
	contractID          string
	coopID              string
	protoData           string
}

var (
	// Contracts is a map of contracts and is saved to disk
	Contracts map[string]*Contract
	eiDatas   map[string]*eiData
)

func init() {
	Contracts = make(map[string]*Contract)
	eiDatas = make(map[string]*eiData)

	initDataStore()

	var c, err = loadData()
	if err == nil {
		Contracts = c
	}
}

func changeContractState(contract *Contract, newstate int) {
	contract.State = newstate

	// Set the banker to a common sink variable
	// This will avoid adding this logic in multiple places
	switch contract.State {
	case ContractStateCRT:
		contract.Banker.CurrentBanker = contract.Banker.CrtSinkUserID
	case ContractStateBanker:
		contract.Banker.CurrentBanker = contract.Banker.BoostingSinkUserID
	case ContractStateWaiting:
		if contract.Style&ContractFlagBanker != 0 {
			contract.Banker.CurrentBanker = contract.Banker.BoostingSinkUserID
		} else {
			contract.Banker.CurrentBanker = contract.Banker.PostSinkUserID
		}
	case ContractStateCompleted:
		contract.Banker.CurrentBanker = contract.Banker.PostSinkUserID
		if contract.SavedStats {
			if len(contract.BoostedOrder) != len(contract.Order) {
				contract.BoostedOrder = contract.Order
			}
			farmerstate.SetOrderPercentileAll(contract.BoostedOrder, len(contract.Order))
			contract.SavedStats = true
		}
	default:
		contract.Banker.CurrentBanker = ""
	}

}

// DeleteContract will delete the contract
func DeleteContract(s *discordgo.Session, guildID string, channelID string) (string, error) {
	var contract = FindContract(channelID)
	if contract == nil {
		return "", errors.New(errorNoContract)
	}

	var coopHash = contract.ContractHash
	var coopName = contract.ContractID + "/" + contract.CoopID
	_ = saveEndData(contract) // Save for historical purposes

	for _, el := range contract.Location {
		if s != nil {
			_ = s.ChannelMessageDelete(el.ChannelID, el.ListMsgID)
			_ = s.ChannelMessageDelete(el.ChannelID, el.ReactionID)
		}
	}
	delete(Contracts, coopHash)
	saveData(Contracts)

	return coopName, nil
}

// FindEggEmoji will find the token emoji for the given guild
func FindEggEmoji(eggOrig string) string {
	// remove _ from egg
	//egg := strings.Replace(eggOrig, "_", "", -1)
	//egg = strings.Replace(egg, "-", "", -1) // carbon fibre egg

	var e = ei.FindEggEmoji(strings.ToUpper(eggOrig))
	if e != "" {
		return e
	}

	e = ei.FindEggEmoji("UNKNOWN")
	return e
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
		return "in Sign-up order"
	case ContractOrderReverse:
		if contract.StartTime.IsZero() || contract.State == ContractStateSignup {
			return "in Reverse Sign-up order"
		}
		return fmt.Sprintf("Reverse -> Sign-up <t:%d:R> ", thresholdStartTime.Unix())
	case ContractOrderRandom:
		if contract.StartTime.IsZero() || contract.State == ContractStateSignup {
			return "Random order"
		}
		return fmt.Sprintf("Random -> Sign-up <t:%d:R> ", thresholdStartTime.Unix())
	case ContractOrderFair:
		if contract.StartTime.IsZero() || contract.State == ContractStateSignup {
			return "Fair order"
		}
		return fmt.Sprintf("Fair -> Sign-up <t:%d:R> ", thresholdStartTime.Unix())
	case ContractOrderTimeBased:
		return "Time"
	case ContractOrderELR:
		return "Egg Lay Rate order"
	case ContractOrderTVal:
		return "Token Value order"
	}
	return "Unknown"
}

// AddBoostTokensInteraction handles the interactions responses for AddBoostTokens
func AddBoostTokensInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, setCountWant int, countWantAdjust int) {
	var str string
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	tSent, tRecv, err := AddBoostTokens(s, i, setCountWant, countWantAdjust)
	if err != nil {
		str = err.Error()
	} else {
		str = fmt.Sprintf("Adjusted. Tokens Wanted %d, Received %d", tSent, tRecv)
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Content: str,
		})
}

// AddBoostTokens will add tokens to the current booster and adjust the count of the booster
func AddBoostTokens(s *discordgo.Session, i *discordgo.InteractionCreate, setCountWant int, countWantAdjust int) (int, int, error) {
	// Find the contract
	var contract = FindContract(i.ChannelID)
	if contract == nil {
		return 0, 0, errors.New(errorNoContract)
	}
	// verify the user is in the contract
	if !userInContract(contract, getInteractionUserID(i)) {
		return 0, 0, errors.New(errorUserNotInContract)
	}

	// Add the token count for the userID, ensure the count is not negative
	var b = contract.Boosters[getInteractionUserID(i)]
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

	if (ContractFlagDynamicTokens+ContractFlag8Tokens+ContractFlag6Tokens)&contract.Style == 0 {
		// Only set this if the contract isn't controlling the wanted tokens
		farmerstate.SetTokens(b.UserID, b.TokensWanted)
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

func setChickenRunMessageID(contract *Contract, messageID string) {
	if slices.Index(contract.CRMessageIDs, messageID) == -1 {
		contract.CRMessageIDs = append(contract.CRMessageIDs, messageID)
	}
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
			countStr = boostIcon
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
func FindContract(channelID string) *Contract {
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
			//if loc.ChannelID == channelID {
			if slices.Index(loc.MessageIDs, messageID) != -1 {
				return c
			}
		}
	}
	return nil
}

// AddContractMember adds a member to a contract
func AddContractMember(s *discordgo.Session, guildID string, channelID string, operator string, mention string, guest string, order int) error {
	var contract = FindContract(channelID)
	if contract == nil {
		return errors.New(errorNoContract)
	}

	re := regexp.MustCompile(`[\\<>@#&!]`)
	if mention != "" {
		var userID = re.ReplaceAllString(mention, "")
		idx := slices.Index(contract.Order, userID)
		if idx != -1 {
			return errors.New(errorUserInContract)
		}

		var u, err = s.User(userID)
		if err != nil {
			return errors.New(errorNoFarmer)
		}
		if u.Bot {
			return errors.New(errorBot)
		}
		_, err = AddFarmerToContract(s, contract, guildID, channelID, u.ID, order)
		if err != nil {
			return err
		}

	}

	if guest != "" {
		idx := slices.Index(contract.Order, guest)
		if idx != -1 {
			b := contract.Boosters[guest]
			wantedTokens := farmerstate.GetTokens(b.UserID)
			if contract.State != ContractStateSignup {
				if contract.Style&ContractFlag6Tokens != 0 {
					wantedTokens = 6
				} else if contract.Style&ContractFlag8Tokens != 0 {
					wantedTokens = 8
				}
			}

			if b.TokensWanted != wantedTokens {
				b.TokensWanted = wantedTokens
				return fmt.Errorf("token count for **%s** adjusted to %d. This will show on the next contract refresh", guest, wantedTokens)
			}
			return errors.New(errorUserInContract)
		}

		_, err := AddFarmerToContract(s, contract, guildID, channelID, guest, order)
		if err != nil {
			return err
		}
		for _, loc := range contract.Location {
			var listStr = "Boost"
			if contract.State == ContractStateSignup {
				listStr = "Sign-up"
			}
			var str = fmt.Sprintf("%s was added to the %s List by %s", guest, listStr, operator)
			_, _ = s.ChannelMessageSend(loc.ChannelID, str)
		}
	}

	return nil
}

func getUserArtifacts(userID string, inSet *ArtifactSet) ArtifactSet {
	var mySet ArtifactSet
	//mySet.Artifacts = make([]ei.Artifact, 5)
	// Pull in any saved artifacts
	if inSet == nil {
		prefix := []string{"", "D-", "M-", "C-", "G-"}
		for i, el := range []string{"collegg", "defl", "metr", "comp", "guss"} {
			art := farmerstate.GetMiscSettingString(userID, el)
			if art == "" {
				continue
			}
			if i == 0 {
				// Colleggtible
				for _, a := range strings.Split(art, ",") {
					if a != "" {
						colleg := ei.ArtifactMap[a]
						if colleg != nil {
							mySet.Artifacts = append(mySet.Artifacts, *colleg)
						}
					}
				}
			} else {
				if art != "" {
					a := ei.ArtifactMap[prefix[i]+art]
					if a != nil {
						fmt.Print(prefix[i]+art, a)
						mySet.Artifacts = append(mySet.Artifacts, *a)
					}
				}
			}
		}
	} else {
		mySet = *inSet
	}

	baseLaying := 3.772
	baseShipping := 7.148
	layRate := baseLaying
	shipRate := baseShipping
	for _, a := range mySet.Artifacts {
		layRate *= a.LayBuff * math.Pow(1.05, float64(a.Stones))
		shipRate *= a.ShipBuff * math.Pow(1.05, float64(a.Stones))
	}
	mySet.LayRate = layRate
	mySet.ShipRate = shipRate
	return mySet
}

// AddFarmerToContract adds a farmer to a contract
func AddFarmerToContract(s *discordgo.Session, contract *Contract, guildID string, channelID string, userID string, order int) (*Booster, error) {
	log.Println("AddFarmerToContract", "GuildID: ", guildID, "ChannelID: ", channelID, "UserID: ", userID, "Order: ", order)

	if contract.CoopSize == min(len(contract.Order), len(contract.Boosters)) {
		return nil, errors.New(errorContractFull)
	}

	var b = contract.Boosters[userID]
	if b == nil {
		// New Booster - add them to boost list
		var b = new(Booster)
		b.Register = time.Now()
		b.UserID = userID

		var user, err = s.User(userID)
		if err != nil {
			b.GlobalName = userID
			b.Name = userID
			b.Nick = userID
			b.Unique = userID
			b.Mention = userID
		} else {
			b.GlobalName = user.GlobalName
			b.Name = user.Username
			b.Mention = user.Mention()
			gm, errGM := s.GuildMember(guildID, userID)
			if errGM == nil {
				if gm.Nick != "" {
					b.Nick = gm.Nick
					b.Name = gm.Nick
				}
				b.Unique = gm.User.String()
			}
			if b.Nick == "" {
				b.Nick = b.Name
			}
		}

		b.GuildID = guildID
		// Get Guild Name
		g, errG := s.Guild(guildID)
		if errG != nil {
			b.GuildName = "Unknown"
		} else {
			b.GuildName = g.Name
		}
		// Get Channel Name
		ch, errCh := s.Channel(channelID)
		if errCh != nil {
			b.ChannelName = "Unknown"
		} else {
			b.ChannelName = ch.Name
		}

		b.Ping = false
		b.BoostState = BoostStateUnboosted
		b.TokensWanted = farmerstate.GetTokens(b.UserID)
		if b.TokensWanted <= 0 {
			b.TokensWanted = defaultFamerTokens // Default to 6
		}
		if contract.State != ContractStateSignup {
			if contract.Style&ContractFlag6Tokens != 0 {
				b.TokensWanted = 6
			} else if contract.Style&ContractFlag8Tokens != 0 {
				b.TokensWanted = 8
			}
		}
		b.ArtifactSet = getUserArtifacts(userID, nil)

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

		if !userInContract(contract, b.UserID) {
			contract.Boosters[b.UserID] = b
			// If contract hasn't started add booster to the end
			// or if contract is on the last booster already
			if contract.State == ContractStateSignup || contract.State == ContractStateWaiting || order == ContractOrderSignup {
				contract.Order = append(contract.Order, b.UserID)
				if contract.State == ContractStateWaiting {
					contract.BoostPosition = len(contract.Order) - 1
				}
			} else {
				copyOrder := make([]string, len(contract.Order))
				copy(copyOrder, contract.Order)
				copyOrder = append(copyOrder, b.UserID)

				newOrder := farmerstate.GetOrderHistory(copyOrder, 5)

				// find index of farmer.UserID in newOrder
				var index = slices.Index(newOrder, b.UserID)
				if contract.BoostPosition >= index {
					index = contract.BoostPosition + 1
				}
				contract.Order = insert(contract.Order, index, b.UserID)
			}
			contract.Order = removeDuplicates(contract.Order)
			contract.OrderRevision++
		}
		contract.RegisteredNum = len(contract.Boosters)

		// If the BoostBot is the creator, the first person joining becomes
		// the coordinator
		if contract.CreatorID[0] == config.DiscordAppID {
			contract.CreatorID[0] = userID
		}

		// Disabling this for now, leave the signup open
		if contract.State == ContractStateSignup && contract.Style&ContractFlagCrt != 0 {
			/*
				if len(contract.Order) == 1 {
					// Reset contract sink to the first booster to signup
					contract.Banker.CrtSinkUserID = contract.Order[0]
					contract.Banker.BoostingSinkUserID = contract.Order[0]
					contract.Banker.PostSinkUserID = contract.Order[0]
				}
			*/

			calculateTangoLegs(contract, true)
		}

		if contract.State == ContractStateWaiting {
			// Reactivate the contract
			// Set the newly added booster as boosting
			if contract.Style&ContractFlagBanker != 0 {
				changeContractState(contract, ContractStateBanker)
			} else if contract.Style&ContractFlagFastrun != 0 {
				changeContractState(contract, ContractStateFastrun)
			} else {
				panic("Invalid contract style")
			}
			b.StartTime = time.Now()
			b.BoostState = BoostStateTokenTime
			contract.BoostPosition = len(contract.Order) - 1
			// for all locations delete the signup list message and send the boost list message
			//for _, loc := range contract.Location {
			//	s.ChannelMessageDelete(loc.ChannelID, loc.ListMsgID)
			//}
			sendNextNotification(s, contract, true)
			return b, nil
		}
	}
	refreshBoostListMessage(s, contract)
	return b, nil
}

// IsUserCreatorOfAnyContract will return true if the user is the creator of any contract
func IsUserCreatorOfAnyContract(s *discordgo.Session, userID string) bool {
	for _, c := range Contracts {
		if creatorOfContract(s, c, userID) {
			return true
		}
	}
	return false
}

func creatorOfContract(s *discordgo.Session, c *Contract, u string) bool {
	if c != nil {
		for _, el := range c.CreatorID {
			if el == u {
				return true
			}
		}
		for _, el := range c.Location {
			perms, err := s.UserChannelPermissions(u, el.ChannelID)
			if err != nil {
				log.Println(err)
			}
			if perms&discordgo.PermissionAdministrator != 0 {
				return true
			}
		}
	}

	return false
}

func userInContract(c *Contract, u string) bool {
	if c != nil {

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

func findNextBoosterAfterUser(contract *Contract, userID string) int {
	for i := 0; i < len(contract.Order); i++ {
		if contract.Boosters[contract.Order[i]].BoostState == BoostStateUnboosted && contract.Order[i] != userID {
			return i
		}
	}
	return -1
}

// JoinContract will add a user to the contract
func JoinContract(s *discordgo.Session, guildID string, channelID string, userID string, bell bool) error {
	var err error

	log.Println("JoinContract", "GuildID: ", guildID, "ChannelID: ", channelID, "UserID: ", userID, "Bell: ", bell)

	var contract = FindContract(channelID)
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

	// test if userID in Boosters
	if contract.Boosters[userID] != nil {
		contract.Boosters[userID].Ping = bell
	}

	if bell {
		u, _ := s.UserChannelCreate(userID)
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

// RemoveFarmerByMention will remove a booster from the contract by mention
func RemoveFarmerByMention(s *discordgo.Session, guildID string, channelID string, operator string, mention string) error {
	fmt.Println("RemoveContractBoosterByMention", "GuildID: ", guildID, "ChannelID: ", channelID, "Operator: ", operator, "Mention: ", mention)
	var contract = FindContract(channelID)
	redraw := false
	if contract == nil {
		return errors.New(errorNoContract)
	}

	if len(contract.Boosters) == 0 {
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
		offset := 2
		if strings.HasPrefix(userID, "<@!") {
			offset = 3
		}
		userID = mention[offset : len(mention)-1]
	}

	removalIndex := slices.Index(contract.Order, userID)
	if removalIndex == -1 {
		return errors.New(errorUserNotInContract)
	}
	currentBooster := ""

	// Save current booster name
	if contract.BoostPosition != -1 {
		if contract.State != ContractStateWaiting && contract.BoostPosition != len(contract.Order) && userID != contract.Order[contract.BoostPosition] {
			currentBooster = contract.Order[contract.BoostPosition]
		}
	}

	// Remove the booster from the contract
	if userID == contract.Banker.PostSinkUserID {
		contract.Banker.PostSinkUserID = ""
		changeContractState(contract, contract.State)
	}

	// If this is an alt, remove its entries from main
	if contract.Boosters[userID].AltController != "" {
		mainUserID := contract.Boosters[userID].AltController
		if contract.Banker.CrtSinkUserID == userID {
			contract.Banker.CrtSinkUserID = mainUserID
		}
		if contract.Banker.BoostingSinkUserID == userID {
			contract.Banker.BoostingSinkUserID = mainUserID
		}
		if contract.Banker.PostSinkUserID == userID {
			contract.Banker.PostSinkUserID = mainUserID
		}
		contract.SRData.StatusStr = getSpeedrunStatusStr(contract)

		altIdx := slices.Index(contract.Boosters[mainUserID].Alts, userID)
		contract.Boosters[mainUserID].Alts = removeIndex(contract.Boosters[mainUserID].Alts, altIdx)
		contract.Boosters[mainUserID].AltsIcons = removeIndex(contract.Boosters[mainUserID].AltsIcons, altIdx)
		rebuildAltList(contract)
		contract.buttonComponents = nil // reset button components
		redraw = true
	} else if len(contract.Boosters[userID].Alts) > 0 {
		// If this is a main with alts, clear the alts
		for _, alt := range contract.Boosters[userID].Alts {
			contract.Boosters[alt].AltController = ""
		}
		contract.Boosters[userID].Alts = nil
		contract.Boosters[userID].AltsIcons = nil

		rebuildAltList(contract)
		redraw = true
	}
	contract.Order = removeIndex(contract.Order, removalIndex)
	contract.OrderRevision++
	delete(contract.Boosters, userID)
	contract.RegisteredNum = len(contract.Boosters)

	if userID == contract.CreatorID[0] {
		// Reassign CreatorID to the Bot, then if there's a non-guest, make them the coordinator
		contract.CreatorID[0] = config.DiscordAppID
		for _, el := range contract.Order {
			if contract.Boosters[el] != nil {
				if contract.Boosters[el].UserID != contract.Boosters[el].Mention {
					contract.CreatorID[0] = contract.Boosters[el].UserID
					break
				}
			}
		}
	}

	if contract.State != ContractStateSignup {
		if currentBooster != "" {
			contract.BoostPosition = slices.Index(contract.Order, currentBooster)
		} else {
			// Active Booster is leaving contract.
			if contract.State == ContractStateCompleted || contract.State == ContractStateArchive || contract.State == ContractStateWaiting {
				changeContractState(contract, ContractStateWaiting)
				contract.BoostPosition = len(contract.Order)
				sendNextNotification(s, contract, true)
			} else if (contract.State == ContractStateFastrun || contract.State == ContractStateBanker) && contract.BoostPosition == len(contract.Order) {
				// set contract to waiting
				changeContractState(contract, ContractStateWaiting)
				sendNextNotification(s, contract, true)
			} else {
				contract.BoostPosition = findNextBooster(contract)
				if contract.BoostPosition != -1 {
					contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = BoostStateTokenTime
					contract.Boosters[contract.Order[contract.BoostPosition]].StartTime = time.Now()
					sendNextNotification(s, contract, true)
					// Returning here since we're actively boosting and will send a new message
					return nil
				}
			}
		}
	}

	// Edit the boost List in place
	if contract.BoostPosition != len(contract.Order) {
		for _, loc := range contract.Location {
			if redraw {
				_ = RedrawBoostList(s, loc.GuildID, loc.ChannelID)
				continue
			}
			if contract.State == ContractStateSignup && contract.Style&ContractFlagCrt != 0 {
				if len(contract.Order) == 0 {
					// Need to clear all the contract sinks
					contract.Banker.CrtSinkUserID = ""
					contract.Banker.BoostingSinkUserID = ""
					contract.Banker.PostSinkUserID = ""
				}
				calculateTangoLegs(contract, true)
			}
			msgedit := discordgo.NewMessageEdit(loc.ChannelID, loc.ListMsgID)
			contentStr := DrawBoostList(s, contract)
			msgedit.SetContent(contentStr)
			msgedit.Flags = discordgo.MessageFlagsSuppressEmbeds
			msg, err := s.ChannelMessageEditComplex(msgedit)
			if err == nil {
				loc.ListMsgID = msg.ID
			}
			// Need to disable the speedrun start button if the contract is no longer full
			if contract.Style&ContractFlagCrt != 0 && contract.State == ContractStateSignup {
				enableButton := false
				if len(contract.Order) == 0 {
					enableButton = true
				}
				msgID := loc.ReactionID
				msg := discordgo.NewMessageEdit(loc.ChannelID, msgID)
				// Full contract for speedrun
				contentStr, comp := GetSignupComponents(enableButton, contract) // True to get a disabled start button
				msg.SetContent(contentStr)
				msg.Flags = discordgo.MessageFlagsSuppressEmbeds
				msg.Components = &comp
				_, _ = s.ChannelMessageEditComplex(msg)
			}
		}
	}

	return nil
}

// StartContractBoosting will start the contract
func StartContractBoosting(s *discordgo.Session, guildID string, channelID string, userID string) error {
	var contract = FindContract(channelID)
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

	if !creatorOfContract(s, contract, userID) && contract.CreatorID[0] != config.DiscordAppID {
		if !(contract.Style&ContractFlagCrt != 0 && contract.Banker.CrtSinkUserID == userID) {
			return errors.New(errorNotContractCreator)
		}
	}

	reorderBoosters(contract)
	if contract.State == ContractStateSignup && contract.Style&ContractFlagCrt != 0 {
		calculateTangoLegs(contract, true)
	}

	// Set tokens...
	for i := range contract.Boosters {
		if contract.Style&ContractFlag6Tokens != 0 {
			contract.Boosters[i].TokensWanted = 6
		} else if contract.Style&ContractFlag8Tokens != 0 {
			contract.Boosters[i].TokensWanted = 8
		}
	}

	contract.BoostPosition = 0
	contract.StartTime = time.Now()
	if contract.Style&ContractFlagCrt != 0 && contract.SRData.Legs != 0 && contract.Banker.CrtSinkUserID != "" {
		changeContractState(contract, ContractStateCRT)
		// Do not mark the token sink as boosting at this point
		// This will happen when the CRT completes
	} else if contract.Style&ContractFlagBanker != 0 {
		changeContractState(contract, ContractStateBanker)
		contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = BoostStateTokenTime
		contract.Boosters[contract.Order[contract.BoostPosition]].StartTime = time.Now()
	} else if contract.Style&ContractFlagFastrun != 0 {
		changeContractState(contract, ContractStateFastrun)
		contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = BoostStateTokenTime
		contract.Boosters[contract.Order[contract.BoostPosition]].StartTime = time.Now()
	} else {
		panic("Invalid contract style")
	}

	sendNextNotification(s, contract, true)

	return nil
}

// RedrawBoostList will move the boost message to the bottom of the channel
func RedrawBoostList(s *discordgo.Session, guildID string, channelID string) error {
	var contract = FindContract(channelID)
	if contract == nil {
		return errors.New(errorNoContract)
	}

	if contract.State == ContractStateSignup {
		return errors.New(errorContractNotStarted)
	}

	// Edit the boost list in place
	for _, loc := range contract.Location {
		if loc.GuildID == guildID && loc.ChannelID == channelID {
			_ = s.ChannelMessageDelete(loc.ChannelID, loc.ListMsgID)
			var data discordgo.MessageSend
			var am discordgo.MessageAllowedMentions
			data.Content = DrawBoostList(s, contract)
			data.AllowedMentions = &am
			data.Flags = discordgo.MessageFlagsSuppressEmbeds
			msg, err := s.ChannelMessageSendComplex(loc.ChannelID, &data)
			if err == nil {
				SetListMessageID(contract, loc.ChannelID, msg.ID)
			}
			addContractReactionsButtons(s, contract, loc.ChannelID, msg.ID)
		}
	}
	return nil
}

func refreshBoostListMessage(s *discordgo.Session, contract *Contract) {
	// Edit the boost list in place
	for _, loc := range contract.Location {
		msgedit := discordgo.NewMessageEdit(loc.ChannelID, loc.ListMsgID)
		// Full contract for speedrun
		contentStr := DrawBoostList(s, contract)
		msgedit.SetContent(contentStr)
		msgedit.Flags = discordgo.MessageFlagsSuppressEmbeds
		msg, err := s.ChannelMessageEditComplex(msgedit)
		if err == nil {
			// This is an edit, it should be the same
			loc.ListMsgID = msg.ID
		}
		if contract.Style&ContractFlagCrt != 0 && contract.State == ContractStateSignup {
			msgID := loc.ReactionID
			msg := discordgo.NewMessageEdit(loc.ChannelID, msgID)

			// Full contract for speedrun
			contentStr, comp := GetSignupComponents(len(contract.Order) < 1, contract) // True to get a disabled start button
			msg.SetContent(contentStr)
			msg.Components = &comp
			_, _ = s.ChannelMessageEditComplex(msg)
		}

	}
}

func sendNextNotification(s *discordgo.Session, contract *Contract, pingUsers bool) {
	// Start boosting contract
	drawn := false
	for _, loc := range contract.Location {
		var msg *discordgo.Message
		var err error

		if contract.State == ContractStateSignup {
			msgedit := discordgo.NewMessageEdit(loc.ChannelID, loc.ListMsgID)
			// Full contract for speedrun
			contentStr := DrawBoostList(s, contract)
			msgedit.SetContent(contentStr)
			msgedit.Flags = discordgo.MessageFlagsSuppressEmbeds
			_, err := s.ChannelMessageEditComplex(msgedit)
			if err != nil {
				fmt.Println("Unable to send this message." + err.Error())
			}
		} else {
			// Unpin message once the contract is completed
			if contract.State == ContractStateArchive {
				_ = s.ChannelMessageUnpin(loc.ChannelID, loc.ReactionID)
			}
			_ = s.ChannelMessageDelete(loc.ChannelID, loc.ListMsgID)

			// Compose the message without a Ping
			var data discordgo.MessageSend
			var am discordgo.MessageAllowedMentions
			data.Content = DrawBoostList(s, contract)
			data.AllowedMentions = &am
			data.Flags = discordgo.MessageFlagsSuppressEmbeds
			msg, err = s.ChannelMessageSendComplex(loc.ChannelID, &data)
			if err == nil {
				SetListMessageID(contract, loc.ChannelID, msg.ID)
			}
			drawn = true
		}
		if err != nil {
			fmt.Println("Unable to resend message." + err.Error())
		}
		var str = ""
		if msg == nil {
			// Maybe this location is broken
			continue
		}

		switch contract.State {
		case ContractStateWaiting, ContractStateCRT, ContractStateBanker, ContractStateFastrun:
			addContractReactionsButtons(s, contract, loc.ChannelID, msg.ID)
			if pingUsers {
				if contract.State == ContractStateFastrun || contract.State == ContractStateBanker {
					var name string
					var einame = farmerstate.GetEggIncName(contract.Order[contract.BoostPosition])
					if einame != "" {
						einame += " " // Add a space to this
					}

					if contract.Banker.CurrentBanker != "" {
						name = einame + contract.Boosters[contract.Banker.CurrentBanker].Mention
					} else {
						name = einame + contract.Boosters[contract.Order[contract.BoostPosition]].Mention
					}

					str = fmt.Sprintf(loc.ChannelPing+" send tokens to %s", name)
				} else {
					if contract.Banker.CurrentBanker == "" {
						str = loc.ChannelPing + " contract boosting complete. Hold your tokens for late joining farmers."
					} else {
						str = "Contract boosting complete. There may late joining farmers. "
						if contract.State == ContractStateCompleted || contract.State == ContractStateWaiting {
							var einame = farmerstate.GetEggIncName(contract.Banker.CurrentBanker)
							if einame != "" {
								einame += " " // Add a space to this
							}
							name := einame + contract.Boosters[contract.Banker.CurrentBanker].Mention
							str = fmt.Sprintf(loc.ChannelPing+" send tokens to our volunteer sink **%s**", name)
						}
					}
				}
			}
		case ContractStateCompleted:
			addContractReactionsButtons(s, contract, loc.ChannelID, msg.ID)
			t1 := contract.EndTime
			t2 := contract.StartTime
			duration := t1.Sub(t2)
			str = fmt.Sprintf(loc.ChannelPing+" contract boosting complete in %s ", duration.Round(time.Second))
			if contract.Banker.CurrentBanker != "" {
				var einame = farmerstate.GetEggIncName(contract.Banker.CurrentBanker)
				if einame != "" {
					einame += " " // Add a space to this
				}
				if contract.State != ContractStateArchive {
					name := einame + contract.Boosters[contract.Banker.CurrentBanker].Mention
					str += fmt.Sprintf("\nSend tokens to our volunteer sink **%s**", name)
				}
			}
		default:
		}

		// Sending the update message
		// TODO: Need to figure out just what message to send for banker/fastrun/crt
		if !contract.Speedrun {
			_, _ = s.ChannelMessageSend(loc.ChannelID, str)
		} else if !drawn {
			_ = RedrawBoostList(s, loc.GuildID, loc.ChannelID)
		}
	}
	if pingUsers {
		notifyBellBoosters(s, contract)
	}
	/*
		if contract.State == ContractStateArchive {
			// Only purge the contract from memory if /calc isn't being used
			if contract.CalcOperations == 0 || time.Since(contract.CalcOperationTime).Minutes() > 20 {
				FinishContract(s, contract)
			}
		}
	*/
}

// UserBoost will trigger a contract boost of a user
func UserBoost(s *discordgo.Session, guildID string, channelID string, userID string) error {
	var contract = FindContract(channelID)

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
		_ = Boosting(s, guildID, channelID)
	} else {
		for i := range contract.Order {
			if contract.Order[i] == userID {
				if contract.Boosters[contract.Order[i]].BoostState == BoostStateBoosted {
					return errors.New(errorAlreadyBoosted)
				}
				// Mark user as complete
				// Taking start time from current booster start time
				contract.Boosters[contract.Order[i]].BoostState = BoostStateBoosted
				// Unique insert into contract.BoostedOrder
				contract.BoostedOrder = append(contract.BoostedOrder, contract.Order[i])
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
	var contract = FindContract(channelID)
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

	// These fields are for the dynamic token assignment
	if contract.DynamicData != nil {
		wiggleRoom := time.Duration(30 * time.Second) // Add 30 seconds to the end of the boost
		boostDuration, chickenRunDuration := getBoostTimeSeconds(contract.DynamicData, contract.Boosters[contract.Order[contract.BoostPosition]].TokensWanted)
		contract.Boosters[contract.Order[contract.BoostPosition]].EstDurationOfBoost = boostDuration
		contract.Boosters[contract.Order[contract.BoostPosition]].EstEndOfBoost = time.Now().Add(boostDuration).Add(wiggleRoom)
		contract.Boosters[contract.Order[contract.BoostPosition]].EstRequestChickenRuns = time.Now().Add(chickenRunDuration).Add(wiggleRoom)
	}

	contract.BoostedOrder = append(contract.BoostedOrder, contract.Order[contract.BoostPosition])

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
		changeContractState(contract, ContractStateCompleted) // Waiting for sink
		contract.EndTime = time.Now()
	} else if contract.BoostPosition == len(contract.Order) {
		changeContractState(contract, ContractStateWaiting) // There could be more boosters joining later
	} else {
		contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = BoostStateTokenTime
		contract.Boosters[contract.Order[contract.BoostPosition]].StartTime = time.Now()
		if contract.Order[contract.BoostPosition] == contract.Banker.BoostingSinkUserID {
			contract.Boosters[contract.Order[contract.BoostPosition]].TokensReceived = 0 // reset these
		}
	}

	sendNextNotification(s, contract, true)

	return nil
}

// Unboost will mark a user as unboosted
func Unboost(s *discordgo.Session, guildID string, channelID string, mention string) error {
	var contract = FindContract(channelID)
	if contract == nil {
		return errors.New(errorNoContract)
	}
	//contract.mutex.Lock()
	//defer contract.mutex.Unlock()

	if contract.CoopSize == 0 {
		return errors.New(errorContractEmpty)
	}
	/*
		if contract.Speedrun {
			return errors.New(errorSpeedrunContract)
		}
	*/
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
		if contract.Style&ContractFlagBanker != 0 {
			changeContractState(contract, ContractStateBanker)
		} else if contract.Style&ContractFlagFastrun != 0 {
			changeContractState(contract, ContractStateFastrun)
		} else {
			panic("Invalid contract style")
		}
		// Remove user from contract.BootedOrder
		contract.BoostedOrder = removeIndex(contract.BoostedOrder, slices.Index(contract.BoostedOrder, userID))
		contract.Boosters[userID].BoostState = BoostStateTokenTime
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
		contract.BoostedOrder = removeIndex(contract.BoostedOrder, slices.Index(contract.BoostedOrder, userID))
		refreshBoostListMessage(s, contract)
	}
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
	var contract = FindContract(channelID)
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
			changeContractState(contract, ContractStateCompleted) // Finished
			contract.EndTime = time.Now()
		} else if contract.BoostPosition == len(contract.Boosters) {
			changeContractState(contract, ContractStateWaiting)
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
	contract.Order = removeDuplicates(contract.Order)
	contract.OrderRevision++

	sendNextNotification(s, contract, true)

	return nil
}

func notifyBellBoosters(s *discordgo.Session, contract *Contract) {
	for i, b := range contract.Boosters {
		if contract.Boosters[i].Ping {
			u, _ := s.UserChannelCreate(b.UserID)
			var str string
			if contract.State == ContractStateCompleted || contract.State == ContractStateArchive {
				t1 := contract.EndTime
				t2 := contract.StartTime
				duration := t1.Sub(t2)
				str = fmt.Sprintf("%s: Contract Boosting Completed in %s ", b.ChannelName, duration.Round(time.Second))
			} else if contract.State == ContractStateWaiting {
				t1 := time.Now()
				t2 := contract.StartTime
				duration := t1.Sub(t2)
				str = fmt.Sprintf("%s: Boosting Completed in %s. Still %d spots in the contract. ", b.ChannelName, duration.Round(time.Second), contract.CoopSize-len(contract.Boosters))
			} else {
				str = fmt.Sprintf("%s: Send Boost Tokens to %s", b.ChannelName, contract.Boosters[contract.Order[contract.BoostPosition]].Name)
			}
			_, err := s.ChannelMessageSend(u.ID, str)
			if err != nil {
				log.Println(err)
			}
		}
	}

}

// FinishContract is called only when the contract is complete
func FinishContract(s *discordgo.Session, contract *Contract) {
	// Don't delete the final boost message
	for _, loc := range contract.Location {
		loc.ListMsgID = ""
	}
	// Location[0] for this since the original contract is on the first location
	if !contract.SavedStats {
		if len(contract.BoostedOrder) != len(contract.Order) {
			contract.BoostedOrder = contract.Order
		}
		farmerstate.SetOrderPercentileAll(contract.BoostedOrder, len(contract.Order))
		contract.SavedStats = true
	}
	_, _ = DeleteContract(s, contract.Location[0].GuildID, contract.Location[0].ChannelID)
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
		contract.Order = removeDuplicates(newOrder)
	case ContractOrderELR:
		type ELRPair struct {
			Name string
			ELR  float64
		}

		var elrPairs []ELRPair
		for _, el := range contract.Order {
			elrPairs = append(elrPairs, ELRPair{
				Name: el,
				ELR:  min(contract.Boosters[el].ArtifactSet.LayRate, contract.Boosters[el].ArtifactSet.ShipRate),
			})
		}

		sort.Slice(elrPairs, func(i, j int) bool {
			return elrPairs[i].ELR > elrPairs[j].ELR
		})

		var orderedNames []string
		for _, pair := range elrPairs {
			orderedNames = append(orderedNames, pair.Name)
		}
		contract.Order = orderedNames
		// Reset this to Signup after the initial ELR sort
		contract.BoostOrder = ContractOrderSignup
	case ContractOrderTVal:
		type TValPair struct {
			name string
			val  float64
		}
		var orderedNames []string
		var lastOrderNames []string
		var tvalPairs []TValPair
		if contract.BoostPosition == contract.CoopSize {
			log.Print("TVal Boosting complete", contract.BoostPosition, len(contract.Order))
			contract.BoostOrder = ContractOrderSignup
			return
		} else if contract.BoostPosition == len(contract.Order) {
			return
		}
		lastBoostTime := time.Now()
		contract.mutex.Lock()
		for i, el := range contract.Order {
			if contract.Banker.SinkBoostPosition == SinkBoostFirst {
				// Sink boosting last or in order will end up with negative tval
				// and drop lowest in the list anyway
				if el == contract.Banker.BoostingSinkUserID {
					// Put the banker at the front of orderedNames
					orderedNames = append([]string{el}, orderedNames...)
					if contract.Boosters[el].BoostState == BoostStateBoosted {
						lastBoostTime = contract.Boosters[el].EndTime
					}
					continue
				}
			} else if contract.Banker.SinkBoostPosition == SinkBoostLast {
				if el == contract.Banker.BoostingSinkUserID {
					// Hold the banker until the end
					lastOrderNames = append(lastOrderNames, el)
					if i == contract.BoostPosition && contract.Boosters[el].BoostState == BoostStateTokenTime {
						// If this is the current booster, reset it to unboosed
						contract.Boosters[el].BoostState = BoostStateUnboosted
					}
					continue
				}
			}

			if contract.Boosters[el].BoostState == BoostStateBoosted {
				orderedNames = append(orderedNames, el)
				lastBoostTime = contract.Boosters[el].EndTime
			} else if contract.Style&ContractFlagFastrun != 0 && contract.Boosters[el].BoostState == BoostStateTokenTime {
				// Fastrun style keeps current booster in place
				orderedNames = append(orderedNames, el)
			} else {
				tvalPairs = append(tvalPairs, TValPair{
					name: el,
					val:  contract.Boosters[el].TokenValue,
				})
			}
		}

		sort.Slice(tvalPairs, func(i, j int) bool {
			return tvalPairs[i].val > tvalPairs[j].val
		})

		newBoostPosition := len(orderedNames)
		if contract.Banker.SinkBoostPosition == SinkBoostFirst && contract.BoostPosition == 0 ||
			contract.Boosters[contract.Order[contract.BoostPosition]].BoostState == BoostStateTokenTime {
			newBoostPosition = contract.BoostPosition
		}

		// These boosters are all dymanic, any of them could be the next booster
		for _, pair := range tvalPairs {
			contract.Boosters[pair.name].BoostState = BoostStateUnboosted
			contract.Boosters[pair.name].StartTime = lastBoostTime
			orderedNames = append(orderedNames, pair.name)
		}
		if contract.Banker.SinkBoostPosition == SinkBoostLast {
			orderedNames = append(orderedNames, lastOrderNames...)
		}

		contract.Order = orderedNames
		contract.BoostPosition = newBoostPosition
		contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = BoostStateTokenTime
		contract.mutex.Unlock()

	}
	//
	if contract.Style&ContractFlagBanker != 0 && contract.Banker.BoostingSinkUserID != "" {
		repositionSinkBoostPosition(contract)
	}
}

// ArchiveContracts will set a contract state to Archive if it is older than 5 days
func ArchiveContracts(s *discordgo.Session) {

	var finishHash []string
	currentTime := time.Now()
	for _, contract := range Contracts {
		contractInfo, ok := ei.EggIncContractsAll[contract.ContractID]
		if ok {
			// If the contract is still in the signup phase and hasn't expired, don't archive it
			if contract.State == ContractStateSignup && !currentTime.After(contractInfo.ExpirationTime) {
				continue
			}
		}

		// It's been 3 days since the contract.StartTime and at least 36 hours since the ListInteractionTime
		if currentTime.After(contract.StartTime.Add(3 * 24 * time.Hour)) {
			if currentTime.After(contract.LastInteractionTime.Add(36 * time.Hour)) {
				log.Println("Archiving contract: ", contract.ContractID, " / ", contract.CoopID)
				changeContractState(contract, ContractStateArchive)
				finishHash = append(finishHash, contract.ContractHash)
			}
		}
	}
	for _, hash := range finishHash {
		_ = finishContractByHash(hash)
	}

	// clear finishHash
	finishHash = nil
	for _, d := range eiDatas {
		if d != nil {
			if time.Now().After(d.expirationTimestamp) {
				finishHash = append(finishHash, d.ID)
			}
		}
	}
	for _, hash := range finishHash {
		eiDatas[hash] = nil
	}

	track.ArchiveTrackerData(s)
}

// LoadContractData will load contract data from a file
func LoadContractData(filename string) {

	var EggIncContractsLoaded []ei.EggIncContract
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&EggIncContractsLoaded)
	if err != nil {
		log.Print(err)
		//return
	}

	var EggIncContractsNew []ei.EggIncContract
	//EggIncContractsAllNew := make(map[string]ei.EggIncContract, 800)
	contractProtoBuf := &ei.Contract{}
	for _, c := range EggIncContractsLoaded {
		rawDecodedText, _ := base64.StdEncoding.DecodeString(c.Proto)
		err = proto.Unmarshal(rawDecodedText, contractProtoBuf)
		if err != nil {
			log.Fatalln("Failed to parse address book:", err)
		}
		expirationTime := int64(math.Round(contractProtoBuf.GetExpirationTime()))
		contractTime := time.Unix(expirationTime, 0)

		contract := PopulateContractFromProto(contractProtoBuf)

		if contract.CoopAllowed && contractTime.After(time.Now().UTC()) {
			EggIncContractsNew = append(EggIncContractsNew, contract)
		}

		// Only add completely new contracts to this list
		if existingContract, exists := ei.EggIncContractsAll[c.ID]; !exists || contract.StartTime.After(existingContract.StartTime) {
			ei.EggIncContractsAll[c.ID] = contract
		}

	}
	ei.EggIncContracts = EggIncContractsNew

	/*
		// Call the function to write the estimated durations to a CSV file
		err = WriteEstimatedDurationsToCSV("estimated_durations.csv")
		if err != nil {
			log.Fatal(err)
		}
	*/
}

const originalContractValidDuration = 21 * 86400
const legacyContractValidDuration = 7 * 86400

// PopulateContractFromProto will populate a contract from a protobuf
func PopulateContractFromProto(contractProtoBuf *ei.Contract) ei.EggIncContract {
	var c ei.EggIncContract
	c.ID = contractProtoBuf.GetIdentifier()

	// Create a protobuf for the contract
	//contractBin, _ := proto.Marshal(contractProtoBuf)
	//c.Proto = base64.StdEncoding.EncodeToString(contractBin)

	expirationTime := int64(math.Round(contractProtoBuf.GetExpirationTime()))
	contractTime := time.Unix(expirationTime, 0)

	c.PeriodicalAPI = false // Default this to false
	c.MaxCoopSize = int(contractProtoBuf.GetMaxCoopSize())
	c.ChickenRunCooldownMinutes = int(contractProtoBuf.GetChickenRunCooldownMinutes())
	c.Name = contractProtoBuf.GetName()
	c.Description = contractProtoBuf.GetDescription()
	c.Egg = int32(contractProtoBuf.GetEgg())

	c.LengthInSeconds = int(contractProtoBuf.GetLengthSeconds())
	c.ModifierIHR = 1.0
	c.ModifierELR = 1.0
	c.ModifierSR = 1.0
	c.ModifierHabCap = 1.0
	c.ContractVersion = 2

	if contractProtoBuf.GetStartTime() == 0 {

		if contractProtoBuf.Leggacy == nil || contractProtoBuf.GetLeggacy() {
			c.StartTime = contractTime.Add(-time.Duration(c.LengthInSeconds-legacyContractValidDuration) * time.Second)
		} else {
			c.StartTime = contractTime.Add(-time.Duration(c.LengthInSeconds-originalContractValidDuration) * time.Second)
		}

	} else {
		c.StartTime = time.Unix(int64(contractProtoBuf.GetStartTime()), 0)
	}
	c.ExpirationTime = contractTime
	c.CoopAllowed = contractProtoBuf.GetCoopAllowed()

	if c.Egg == int32(ei.Egg_CUSTOM_EGG) {
		c.EggName = contractProtoBuf.GetCustomEggId()
	} else {
		c.EggName = ei.Egg_name[c.Egg]
	}

	c.MinutesPerToken = int(contractProtoBuf.GetMinutesPerToken())
	c.Grade = make([]ei.ContractGrade, 6)
	for _, s := range contractProtoBuf.GetGradeSpecs() {
		grade := int(s.GetGrade())

		//		if grade == ei.Contract_GRADE_AAA {
		for _, g := range s.GetGoals() {
			c.TargetAmount = append(c.TargetAmount, g.GetTargetAmount())
			c.TargetAmountq = append(c.TargetAmountq, g.GetTargetAmount()/1e15)
			c.LengthInSeconds = int(s.GetLengthSeconds())
		}
		for _, mod := range s.GetModifiers() {
			switch mod.GetDimension() {
			case ei.GameModifier_INTERNAL_HATCHERY_RATE:
				c.ModifierIHR = mod.GetValue()
			case ei.GameModifier_EGG_LAYING_RATE:
				c.ModifierELR = mod.GetValue()
			case ei.GameModifier_SHIPPING_CAPACITY:
				c.ModifierSR = mod.GetValue()
			case ei.GameModifier_HAB_CAPACITY:
				c.ModifierHabCap = mod.GetValue()
			}
		}
		//		}
		c.Grade[grade].TargetAmount = c.TargetAmount
		c.Grade[grade].TargetAmountq = c.TargetAmountq
		c.Grade[grade].ModifierIHR = c.ModifierIHR
		c.Grade[grade].ModifierELR = c.ModifierELR
		c.Grade[grade].ModifierSR = c.ModifierSR
		c.Grade[grade].ModifierHabCap = c.ModifierHabCap
		c.Grade[grade].LengthInSeconds = c.LengthInSeconds

		c.Grade[grade].EstimatedDuration, c.Grade[grade].EstimatedDurationLower = getContractDurationEstimate(c.TargetAmountq[len(c.TargetAmountq)-1], float64(c.MaxCoopSize), c.LengthInSeconds)
	}
	if c.TargetAmount == nil {
		for _, g := range contractProtoBuf.GetGoals() {
			c.ContractVersion = 1
			c.TargetAmount = append(c.TargetAmount, g.GetTargetAmount())
			c.TargetAmountq = append(c.TargetAmountq, g.GetTargetAmount()/1e15)
		}
		//log.Print("No target amount found for contract ", c.ID)
	}
	if c.LengthInSeconds > 0 {
		d := time.Duration(c.LengthInSeconds) * time.Second
		days := d.Hours() / 24.0 // 2 days
		c.ContractDurationInDays = int(days)
		c.ChickenRuns = int(min(20.0, math.Ceil((days*float64(c.MaxCoopSize))/2.0)))
	}
	// Duration estimate
	if len(c.TargetAmount) != 0 {
		c.EstimatedDuration, c.EstimatedDurationLower = getContractDurationEstimate(c.TargetAmountq[len(c.TargetAmountq)-1], float64(c.MaxCoopSize), c.LengthInSeconds)
	}
	return c
}

func updateContractWithEggIncData(contract *Contract) {
	for _, cc := range ei.EggIncContracts {
		if cc.ID == contract.ContractID {
			contract.CoopSize = cc.MaxCoopSize
			contract.LengthInSeconds = cc.LengthInSeconds
			contract.SRData.ChickenRuns = cc.ChickenRuns
			contract.EstimatedDuration = cc.EstimatedDuration
			contract.Name = cc.Name
			contract.Description = cc.Description
			contract.EggName = cc.EggName
			contract.TargetAmount = cc.TargetAmount
			contract.QTargetAmount = cc.TargetAmountq
			contract.ChickenRunCooldownMinutes = cc.ChickenRunCooldownMinutes
			contract.MinutesPerToken = cc.MinutesPerToken
			break
		}
	}
}

// HandleContractAutoComplete will handle the contract auto complete of contract-id's
func HandleContractAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// User interacting with bot, is this first time ?
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0)
	for _, c := range ei.EggIncContracts {
		choice := discordgo.ApplicationCommandOptionChoice{
			Name:  fmt.Sprintf("%s (%s)", c.Name, c.ID),
			Value: c.ID,
		}
		choices = append(choices, &choice)
	}
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Content: "Contract ID",
			Choices: choices,
		}})
}

// HandleAllContractsAutoComplete will handle the contract auto complete of contract-id's
// default to new contracts but allow searching all contracts
func HandleAllContractsAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	searchString := ""

	if opt, ok := optionMap["contract-id"]; ok {
		searchString = opt.StringValue()
	}
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0)

	if searchString == "" {
		for _, c := range ei.EggIncContracts {
			choice := discordgo.ApplicationCommandOptionChoice{
				Name:  fmt.Sprintf("%s (%s)", c.Name, c.ID),
				Value: c.ID,
			}
			choices = append(choices, &choice)
		}
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{
				Content: "Contract ID",
				Choices: choices,
			}})
		return
	}

	for _, c := range ei.EggIncContractsAll {
		if strings.Contains(c.ID, searchString) {

			choice := discordgo.ApplicationCommandOptionChoice{
				Name:  fmt.Sprintf("%s (%s)", c.Name, c.ID),
				Value: c.ID,
			}
			choices = append(choices, &choice)
			if len(choices) >= 10 {
				break
			}
		}
	}
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Content: "Contract ID",
			Choices: choices,
		}})

}
