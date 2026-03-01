package boost

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/track"

	"github.com/bwmarrin/discordgo"
	"github.com/divan/num2words"
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
	"ContractStateDeprecated",
	"ContractStateBanker",
}

var contractPlaystyleNames = []string{
	"Unset",
	"Chill",
	"ACO",
	"Fastrun",
	"Leaderboard",
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
	ContractOrderTokenAsk  = 7 // Token Ask order, less tokens boosts earlier
	ContractOrderTE        = 8 // Truth Egg based order
	ContractOrderTEplus    = 9 // Truth Egg + randomization

	ContractStateSignup    = 0 // Contract is in signup phase
	ContractStateFastrun   = 1 // Contract in Boosting as fastrun
	ContractStateWaiting   = 2 // Waiting for other(s) to join
	ContractStateCompleted = 3 // Contract is completed
	ContractStateArchive   = 4 // Contract is ready to archive
	//ContractStateDeprecated       = 5 // Deprecated
	ContractStateBanker = 6 // Contract is Boosting with Banker

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

	ContractStyleFastrun       = ContractFlagFastrun
	ContractStyleFastrunBanker = ContractFlagBanker
	//ContractStyleSpeedrunBoostList = ContractFlagCrt | ContractFlagFastrun
	//ContractStyleSpeedrunBanker    = ContractFlagCrt | ContractFlagBanker

	ContractPlaystyleUnset          = 0 // Unset
	ContractPlaystyleChill          = 1 // Chill
	ContractPlaystyleACOCooperative = 2 // ACO Cooperative
	ContractPlaystyleFastrun        = 3 // Fastrun
	ContractPlaystyleLeaderboard    = 4 // Leaderboard
)

const defaultFamerTokens = 6

var boostIconName = "🚀"     // For Reaction tests
var boostIconReaction = "🚀" // For displaying
var boostIcon = "🚀"         // For displaying

// CompMap is a cached set of components for this contract
type CompMap struct {
	Emoji          string
	ID             string
	Name           string
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
	UserName    string
	ChannelName string
	GuildID     string // Discord Guild where this User is From
	GuildName   string
	Ping        bool      // True/False
	Register    time.Time //o Time Farmer registered to boost

	Name          string
	Unique        string
	Nick          string
	Mention       string // String which mentions user
	Color         int
	Alts          []string // Array of alternate ids for the user
	AltsIcons     []string // Array of alternate icons for the user
	AltController string   // User ID of the controller of this alternate

	BoostState             int           // Indicates if current booster
	TokensReceived         int           // indicate number of boost tokens
	TokensWanted           int           // indicate number of boost tokens
	TokenValue             float64       // Current Token Value
	TokenRequestFlag       bool          // Flag to indicate if the token request is active
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
	Ultra                  bool          // Does this player have Ultra
	TECount                int           // Truth Egg Count
}

// LocationData holds server specific Data for a contract
type LocationData struct {
	GuildID           string
	GuildName         string
	ChannelID         string // Contract Discord ThreadID
	ChannelMention    string // Mention string for the thread
	GuildContractRole discordgo.Role
	RoleMention       string
	ListMsgID         string   // Message ID for the Last Boost Order message
	ReactionID        string   // Message ID for the reaction Order String
	MessageIDs        []string // Array of message IDs for any contract message
	TokenXStr         string   // Emoji for Token
	TokenXReactionStr string   // Emoji for Token Reaction
}

// BankerInfo holds information about contract Banker
type BankerInfo struct {
	CurrentBanker      string // Current Banker
	BoostingSinkUserID string // Boosting Sink User ID
	PostSinkUserID     string // Sink End of Contract User ID
	SinkBoostPosition  int    // Sink Boost Position
}

// Contract is the main struct for each contract
type Contract struct {
	ContractHash string // ContractID-CoopID
	Location     []*LocationData
	CreatorID    []string // Slice of creators
	//SignupMsgID    map[string]string // Message ID for the Signup Message
	ContractID                string // Contract ID
	CoopID                    string // CoopID
	SeasonalScoring           int    // 1 = new scoring
	Name                      string
	Description               string
	Egg                       int32
	EggName                   string
	EggEmoji                  string
	TokenStr                  string // Emoji for Token
	ChickenRuns               int    // Number of Chicken Runs for this contract
	ChickenRunCooldownMinutes int
	MinutesPerToken           int
	EstimatedDuration         time.Duration
	CompletionDuration        time.Duration
	EstimatedEndTime          time.Time
	EstimatedDurationValid    bool
	ThreadName                string
	ThreadRenameTime          time.Time
	EstimateUpdateTime        time.Time
	TimeBoosting              time.Time // When the contract boost started

	CoopSize            int
	Ultra               bool
	UltraCount          int
	Style               int64 // Mask for the Contract Style
	PlayStyle           int   // Playstyle of the contract
	NewToPlayStyle      bool  // Someone in the contract is new to this playstyle
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
	CRMessageIDs        map[string]string   // CR reqest messageIDs
	WaitlistBoosters    []string            // Waitlist of UserID's
	AltIcons            []string            // Array of alternate icons for the Boosters
	Order               []string
	BoostedOrder        []string   // Actual order of boosting
	OrderRevision       int        // Incremented when Order is changed
	Banker              BankerInfo // Banker for the contract
	TokenLog            []ei.TokenUnitLog
	TokensPerMinute     float64
	CalcOperations      int
	CalcOperationTime   time.Time
	CoopTokenValueMsgID string
	LastWishPrompt      string             // saved prompt for this contract
	LastInteractionTime time.Time          // last time the contract was drawn
	buttonComponents    map[string]CompMap // Cached components for this contract
	SavedStats          bool               // Saved stats for this contract
	NewFeature          int                // Used to slide in new features
	DynamicData         *DynamicTokenData
	LastSaveTime        time.Time // The last time the contract was saved

	mutex sync.Mutex // Keep this contract thread safe
}

// UnmarshalJSON handles backward compatibility for CRMessageIDs
// which used to be stored as an array but is now a map[string]string
func (c *Contract) UnmarshalJSON(data []byte) error {
	type Alias Contract
	aux := &struct {
		CRMessageIDs interface{} `json:"CRMessageIDs"`
		*Alias
	}{
		Alias: (*Alias)(c),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Handle CRMessageIDs backward compatibility
	if aux.CRMessageIDs != nil {
		switch v := aux.CRMessageIDs.(type) {
		case map[string]interface{}:
			// Already a map, convert to map[string]string
			c.CRMessageIDs = make(map[string]string)
			for k, val := range v {
				if str, ok := val.(string); ok {
					c.CRMessageIDs[k] = str
				}
			}
		case []interface{}:
			// Old array format; leave initialization to the common handler below
		}
	}

	// Ensure CRMessageIDs is always initialized
	if c.CRMessageIDs == nil {
		c.CRMessageIDs = make(map[string]string)
	}
	return nil
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

func changeContractState(contract *Contract, newstate int) {
	contract.State = newstate

	// Set the banker to a common sink variable
	// This will avoid adding this logic in multiple places
	switch contract.State {
	case ContractStateBanker:
		contract.TimeBoosting = time.Now()
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
	//_ = saveEndData(contract) // Save for historical purposes

	for _, el := range contract.Location {
		if s != nil {
			_ = s.ChannelMessageDelete(el.ChannelID, el.ListMsgID)
			_ = s.ChannelMessageDelete(el.ChannelID, el.ReactionID)

			if IsRoleCreatedByBot(el.GuildContractRole.Name) {
				err := s.GuildRoleDelete(el.GuildID, el.GuildContractRole.ID)
				if err != nil {
					log.Printf("Failed to delete role %s: %v", el.GuildContractRole.Name, err)
				}
			}
		}
	}
	contract.State = ContractStateArchive
	saveData(contract.ContractHash)
	delete(Contracts, coopHash)

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
		return fmt.Sprintf("Egg Lay Rate order (%s)", bottools.GetFormattedCommand("artifact"))
	case ContractOrderTVal:
		return "Token Value order"
	case ContractOrderTokenAsk:
		return "Token Ask order."
	case ContractOrderTE:
		return "TE order"
	case ContractOrderTEplus:
		return "TE+ order"
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

	refreshBoostListMessage(s, contract, false)

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
	saveData(contract.ContractHash)
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
	saveData(contract.ContractHash)
}

func setChickenRunMessageID(contract *Contract, messageID, userID string) {
	if contract.CRMessageIDs == nil {
		contract.CRMessageIDs = make(map[string]string)
	}
	contract.CRMessageIDs[messageID] = userID
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

// FindContractByHash will find a contract by its hash
func FindContractByHash(hash string) *Contract {
	contract, exists := Contracts[hash]
	if exists {
		return contract
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

// FindContractByIDs will find the contract by the contractID and coopID
func FindContractByIDs(contractID string, coopID string) *Contract {
	// Look for the contract
	for key, element := range Contracts {
		if strings.EqualFold(element.ContractID, contractID) && strings.EqualFold(element.CoopID, coopID) {
			// Found the contract matching the given ContractID and CoopID
			return Contracts[key]
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

	if mention != "" {
		userID := normalizeUserIDInput(mention)
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
		_, err = AddFarmerToContract(s, contract, guildID, channelID, u.ID, order, false)
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

		previousBoosters := len(contract.Boosters)

		_, err := AddFarmerToContract(s, contract, guildID, channelID, guest, order, false)
		if err != nil {
			return err
		}
		for _, loc := range contract.Location {
			var listStr = "Boost"
			if contract.State == ContractStateSignup {
				listStr = "Sign-up"
			}
			var str = fmt.Sprintf("%s was added to the %s List by %s", guest, listStr, operator)
			msg, err := s.ChannelMessageSend(loc.ChannelID, str)
			if err == nil && msg != nil {
				time.AfterFunc(10*time.Second, func() {
					if err := s.ChannelMessageDelete(msg.ChannelID, msg.ID); err != nil {
						log.Println(err)
					}
				})
			}

			if contract.State == ContractStateSignup {
				if previousBoosters != len(contract.Boosters) && previousBoosters == contract.CoopSize {
					updateSignupReactionMessage(s, contract, loc)
				}
			}
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
					art = strings.ToUpper(a)
					art = strings.ReplaceAll(art, "CARBONFIBER", "CARBON FIBER")
					art = strings.ReplaceAll(art, "FLAMERETARDANT", "FLAME RETARDANT")

					if a != "" {
						colleg := ei.ArtifactMap[art]
						if colleg != nil {
							mySet.Artifacts = append(mySet.Artifacts, *colleg)
						}
					}
				}
			} else {
				if art != "" {
					a := ei.ArtifactMap[prefix[i]+art]
					if a != nil {
						//fmt.Print(prefix[i]+art, a)
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
func AddFarmerToContract(s *discordgo.Session, contract *Contract, guildID string, channelID string, userID string, order int, progenitor bool) (*Booster, error) {
	log.Println("AddFarmerToContract", "GuildID: ", guildID, "ChannelID: ", channelID, "UserID: ", userID, "Order: ", order)

	if contract.CoopSize == min(len(contract.Order), len(contract.Boosters)) {
		// Only add to waitlist if user isn't already in it
		if !slices.Contains(contract.WaitlistBoosters, userID) {
			contract.WaitlistBoosters = append(contract.WaitlistBoosters, userID)
			refreshBoostListMessage(s, contract, false)

			return nil, nil
		}
		return nil, errors.New("contract is full")
	}

	var b = contract.Boosters[userID]
	if b == nil {
		// New Booster - add them to boost list
		var b = new(Booster)
		b.Register = time.Now()
		b.UserID = userID
		b.Color = 0x00cc00

		var user, err = s.User(userID)
		if err != nil {
			b.GlobalName = userID
			b.Name = userID
			b.Nick = userID
			b.Unique = userID
			b.Mention = userID
		} else {
			b.GlobalName = user.GlobalName
			b.UserName = user.Username
			b.Mention = user.Mention()
			gm, errGM := s.GuildMember(guildID, userID)
			if errGM == nil {
				if gm.Nick != "" {
					b.Name = gm.Nick
					b.Nick = gm.Nick
				} else {
					b.Name = user.GlobalName
					b.Nick = user.GlobalName
				}
				b.Unique = gm.User.String()
				// See if we can find a color
				if s.State.MemberAdd(gm) == nil {
					b.Color = s.State.UserColor(userID, channelID)
				}
			}

			if b.Nick == "" {
				if b.GlobalName != "" {
					b.Nick = b.GlobalName
				} else {
					b.Nick = b.UserName
				}
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
		// Get TE count from Farmerstate
		te := farmerstate.GetMiscSettingString(userID, "TE")
		if te != "" {
			b.TECount, _ = strconv.Atoi(te)
		}
		b.ArtifactSet = getUserArtifacts(userID, nil)
		if contract.Ultra {
			farmerstate.SetUltra(userID)
		}

		if farmerstate.IsUltra(userID) {
			contract.UltraCount++
		}

		if contract.BoostOrder == ContractOrderTE || contract.BoostOrder == ContractOrderTEplus {
			updateContractFarmerTE(s, userID, b, contract)
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

		if !userInContract(contract, b.UserID) {
			contract.Boosters[b.UserID] = b
			if contract.Ultra {
				contract.UltraCount++
			}
			// If contract hasn't started add booster to the end
			// or if contract is on the last booster already
			if contract.State == ContractStateSignup || contract.State == ContractStateWaiting || order == ContractOrderSignup {
				contract.Order = append(contract.Order, b.UserID)
				if contract.State == ContractStateWaiting {
					contract.BoostPosition = len(contract.Order) - 1
				}
			} else {
				contract.Order = append(contract.Order, b.UserID)
			}
			for _, el := range contract.Location {
				if el.GuildID == guildID && b.UserID != b.Name && el.GuildContractRole.ID != "" {
					_ = s.GuildMemberRoleAdd(guildID, b.UserID, el.GuildContractRole.ID)
				}
			}

			contract.Order = removeDuplicates(contract.Order)
			contract.OrderRevision++
		}
		contract.RegisteredNum = len(contract.Boosters)

		altController := farmerstate.GetMiscSettingString(userID, "AltController")
		if altController != "" {
			if contract.Boosters[altController] != nil {
				contract.mutex.Lock()
				// We have an alt we can auto link
				newAltIcon := findAltIcon(userID, contract.AltIcons)
				contract.Boosters[altController].Alts = append(contract.Boosters[altController].Alts, userID)
				contract.Boosters[altController].AltsIcons = append(contract.Boosters[altController].AltsIcons, newAltIcon)
				contract.AltIcons = append(contract.AltIcons, newAltIcon)
				contract.Boosters[userID].AltController = altController
				rebuildAltList(contract)
				/*
					str := "Associated your `" + userID + "` alt with " + contract.Boosters[altController].Mention + "\n"
					str += "> Use the Signup sink buttons to select your alt for sinks, these cycle through alts so you may need to press them multiple times.\n"
					str += "> Use the " + boostIcon + " reaction to indicate when your main or alt(s) boost.\n"
					str += "> Use the " + newAltIcon + " reaction to indicate when `" + userID + "` sends tokens."
				*/
				contract.buttonComponents = nil // reset button components
				contract.mutex.Unlock()
			}
		}

		// If the BoostBot is the creator, the first person joining becomes
		// the coordinator
		if contract.CreatorID[0] == config.DiscordAppID {
			contract.CreatorID[0] = userID
		}

		if !progenitor {
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
	}
	if !progenitor {
		refreshBoostListMessage(s, contract, contract.RegisteredNum == contract.CoopSize)
	}
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

func updateContractFarmerTE(s *discordgo.Session, userID string, b *Booster, contract *Contract) {
	// Get user EI from the db and set any relevant fields
	eggIncID := ""
	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
	if err == nil {
		decodedData, err := base64.StdEncoding.DecodeString(eiID)
		if err == nil {
			decryptedData, err := config.DecryptCombined(encryptionKey, decodedData)
			if err == nil {
				eggIncID = string(decryptedData)
			}
		}
	}
	b.TECount = -1 // Indicate we are fetching TE Count
	if len(eggIncID) == 18 && strings.HasPrefix(eggIncID, "EI") {
		go func(eggIncID, userID string, b *Booster) {
			backup, _ := ei.GetFirstContactFromAPI(s, eggIncID, userID, true)
			if backup == nil {
				log.Printf("Received nil backup for user %s", userID)
				return
			}
			virtue := backup.GetVirtue()
			if virtue == nil {
				log.Printf("Received nil virtue for user %s", userID)
				return
			}

			var allEov uint32

			// virtueEggs := []string{"CURIOSITY", "INTEGRITY", "HUMILITY", "RESILIENCE", "KINDNESS"}
			for i := range 5 {
				eov := virtue.GetEovEarned()[i]
				delivered := virtue.GetEggsDelivered()[i]

				eovEarned := ei.CountTruthEggTiersPassed(delivered)
				eovPending := ei.PendingTruthEggs(delivered, eov)

				allEov += max(eovEarned-eovPending, 0)
			}

			contract.mutex.Lock()
			b.TECount = int(allEov)
			contract.mutex.Unlock()
			refreshBoostListMessage(s, contract, false)

		}(eggIncID, userID, b)
	} else {
		te := farmerstate.GetMiscSettingString(userID, "TE")
		if te != "" {
			b.TECount, _ = strconv.Atoi(te)
		}
	}
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
			if contract.State != ContractStateSignup {
				return errors.New(errorContractFull)
				// Only add to waitlist if user isn't already in it
			}

		}

		// Wait here until we get our lock
		contract.mutex.Lock()
		_, err = AddFarmerToContract(s, contract, guildID, channelID, userID, contract.BoostOrder, false)

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
			log.Println("Error sending DM to user: ", err)
		}

	}

	return nil
}

func removeIndex(s []string, index int) []string {
	return append(s[:index], s[index+1:]...)
}

// RemoveFarmerByMention will remove a booster from the contract by mention
func RemoveFarmerByMention(s *discordgo.Session, guildID string, channelID string, operator string, mention string) error {
	log.Println("RemoveContractBoosterByMention", "GuildID: ", guildID, "ChannelID: ", channelID, "Operator: ", operator, "Mention: ", mention)
	var contract = FindContract(channelID)
	redraw := false
	redrawSignup := false
	if contract == nil {
		return errors.New(errorNoContract)
	}

	previousBoosters := len(contract.Boosters)

	if len(contract.Boosters) == 0 {
		return errors.New(errorContractEmpty)
	}
	userID := normalizeUserIDInput(mention)

	if _, isMention := parseMentionUserID(mention); isMention {
		u, _ := s.User(userID)
		if u != nil && u.Bot {
			return errors.New(errorBot)
		}
	}

	// If the farmer is on the waitlist, remove them from it
	removalIndex := slices.Index(contract.WaitlistBoosters, userID)
	if removalIndex != -1 {
		contract.mutex.Lock()
		contract.WaitlistBoosters = removeIndex(contract.WaitlistBoosters, removalIndex)
		contract.mutex.Unlock()
	} else {

		removalIndex = slices.Index(contract.Order, userID)
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

		// Remove the user from the role
		for _, el := range contract.Location {
			if el.GuildID == guildID && el.GuildContractRole.ID != "" && contract.Boosters[userID].Name != userID {
				_ = s.GuildMemberRoleRemove(guildID, userID, el.GuildContractRole.ID)
			}
		}

		sinkChanged := false

		// Remove the booster from the contract
		if userID == contract.Banker.BoostingSinkUserID {
			sinkChanged = true
			contract.Banker.BoostingSinkUserID = ""
		}
		if userID == contract.Banker.PostSinkUserID {
			sinkChanged = true
			contract.Banker.PostSinkUserID = ""
		}
		if sinkChanged {
			changeContractState(contract, contract.State)
		}

		// If this is an alt, remove its entries from main
		if contract.Boosters[userID].AltController != "" {
			mainUserID := contract.Boosters[userID].AltController
			if contract.Banker.BoostingSinkUserID == userID {
				contract.Banker.BoostingSinkUserID = mainUserID
			}
			if contract.Banker.PostSinkUserID == userID {
				contract.Banker.PostSinkUserID = mainUserID
			}

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
			contract.buttonComponents = nil
			redraw = true
		}
		contract.Order = removeIndex(contract.Order, removalIndex)
		contract.OrderRevision++
		delete(contract.Boosters, userID)
		contract.RegisteredNum = len(contract.Boosters)
		if contract.Ultra {
			contract.UltraCount--
		} else {
			if farmerstate.IsUltra(userID) {
				contract.UltraCount--
			}
		}

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
		} else {
			if len(contract.WaitlistBoosters) > 0 {
				// Remove the first person from the want list
				firstWaitlistUser := contract.WaitlistBoosters[0]
				contract.WaitlistBoosters = contract.WaitlistBoosters[1:]
				_, _ = AddFarmerToContract(s, contract, guildID, channelID, firstWaitlistUser, contract.BoostOrder, false)
			}
		}
	}

	// Edit the boost List in place
	//if contract.BoostPosition != len(contract.Order) {
	for _, loc := range contract.Location {
		if redraw {
			if contract.State == ContractStateSignup && previousBoosters == contract.CoopSize {
				redrawSignup = true
			}
			refreshBoostListMessage(s, contract, redrawSignup)
			continue
		}
		if contract.State == ContractStateSignup && contract.Style&ContractFlagCrt != 0 {
			if len(contract.Order) == 0 {
				// Need to clear all the contract sinks
				contract.Banker.BoostingSinkUserID = ""
				contract.Banker.PostSinkUserID = ""
			}
		}
		msgedit := discordgo.NewMessageEdit(loc.ChannelID, loc.ListMsgID)
		components := DrawBoostList(s, contract)
		msgedit.Components = &components
		msgedit.Flags = discordgo.MessageFlagsIsComponentsV2
		msg, err := s.ChannelMessageEditComplex(msgedit)
		if err == nil {
			loc.ListMsgID = msg.ID
		}
		// Need to disable the speedrun start button if the contract is no longer full
		if previousBoosters != len(contract.Boosters) && previousBoosters == contract.CoopSize {
			if contract.State == ContractStateSignup {
				updateSignupReactionMessage(s, contract, loc)
			}
		}
	}
	//}

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
		//if contract.Style&ContractFlagCrt == 0 || contract.Banker.CrtSinkUserID != userID {
		return errors.New(errorNotContractCreator)
		//}
	}

	reorderBoosters(contract)

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

	if contract.Style&ContractFlagBanker != 0 {
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
					if contract.BoostPosition > 0 && contract.BoostPosition <= len(contract.Order) {
						contract.Boosters[contract.Order[i]].StartTime = contract.Boosters[contract.Order[contract.BoostPosition-1]].StartTime
					} else {
						contract.Boosters[contract.Order[i]].StartTime = time.Now()
					}
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

	booster := contract.Boosters[contract.Order[contract.BoostPosition]]
	booster.BoostState = BoostStateBoosted
	booster.EndTime = time.Now()
	booster.Duration = time.Since(booster.StartTime)

	// Estimate timings for remaining boosters
	setEstimatedBoostTimings(booster)

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
		for _, loc := range contract.Location {
			track.UnlinkTokenTracking(s, loc.ChannelID)
		}
	} else if contract.BoostPosition == len(contract.Order) {
		changeContractState(contract, ContractStateWaiting) // There could be more boosters joining later
	} else {
		nextBooster := contract.Boosters[contract.Order[contract.BoostPosition]]
		nextBooster.BoostState = BoostStateTokenTime
		nextBooster.StartTime = time.Now()
		if contract.Order[contract.BoostPosition] == contract.Banker.BoostingSinkUserID {
			nextBooster.TokensReceived = 0 // reset these
		}
		if contract.BoostOrder == ContractOrderTVal {
			reorderBoosters(contract)
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
	userID := normalizeUserIDInput(mention)

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
		boostedIdx := slices.Index(contract.BoostedOrder, userID)
		if boostedIdx != -1 {
			contract.BoostedOrder = removeIndex(contract.BoostedOrder, boostedIdx)
		} else {
			log.Printf("Unboost warning: user not found in BoostedOrder while in waiting state; contractHash=%s channelID=%s userID=%s", contract.ContractHash, channelID, userID)
		}
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
		boostedIdx := slices.Index(contract.BoostedOrder, userID)
		if boostedIdx != -1 {
			contract.BoostedOrder = removeIndex(contract.BoostedOrder, boostedIdx)
		} else {
			log.Printf("Unboost warning: user not found in BoostedOrder; contractHash=%s channelID=%s userID=%s state=%d", contract.ContractHash, channelID, userID, contract.State)
		}
		refreshBoostListMessage(s, contract, false)
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
			for _, loc := range contract.Location {
				track.UnlinkTokenTracking(s, loc.ChannelID)
			}
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
			switch contract.State {
			case ContractStateCompleted, ContractStateArchive:
				t1 := contract.EndTime
				t2 := contract.StartTime
				duration := t1.Sub(t2)
				str = fmt.Sprintf("%s: Contract Boosting Completed in %s ", b.ChannelName, duration.Round(time.Second))
			case ContractStateWaiting:
				t1 := time.Now()
				t2 := contract.StartTime
				duration := t1.Sub(t2)
				str = fmt.Sprintf("%s: Boosting Completed in %s. Still %d spots in the contract. ", b.ChannelName, duration.Round(time.Second), contract.CoopSize-len(contract.Boosters))
			default:
				var name = contract.Boosters[contract.Order[contract.BoostPosition]].Nick
				var einame = farmerstate.GetEggIncName(contract.Order[contract.BoostPosition])
				if einame != "" && einame != name {
					name += " (" + einame + ")"
				}
				str = fmt.Sprintf("%s: Send Boost Tokens to %s", b.ChannelName, name)
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
		contract.SavedStats = true
	}
	_, _ = DeleteContract(s, contract.Location[0].GuildID, contract.Location[0].ChannelID)
}

func reorderBoosters(contract *Contract) {

	type teOrderPair struct {
		name string
		te   int
	}

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
		/*
			case ContractOrderFair:
				newOrder := farmerstate.GetOrderHistory(contract.Order, 5)
				contract.Order = removeDuplicates(newOrder)
		*/
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
	case ContractOrderTokenAsk:
		type TokenPair struct {
			Name string
			Ask  int
		}

		var tokenPairs []TokenPair
		for _, el := range contract.Order {
			tokenPairs = append(tokenPairs, TokenPair{
				Name: el,
				Ask:  contract.Boosters[el].TokensWanted,
			})
		}

		sort.Slice(tokenPairs, func(i, j int) bool {
			return tokenPairs[i].Ask < tokenPairs[j].Ask
		})

		var orderedNames []string
		for _, pair := range tokenPairs {
			orderedNames = append(orderedNames, pair.Name)
		}
		contract.Order = orderedNames
		// Reset this to Signup after the initial Ask sort
		// contract.BoostOrder = ContractOrderSignup

	case ContractOrderTVal:
		type TValPair struct {
			name     string
			position int
			val      float64
			tokenAsk int
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

		for _, el := range contract.Order {
			if contract.Boosters[el].BoostState == BoostStateBoosted {
				orderedNames = append(orderedNames, el)
				lastBoostTime = contract.Boosters[el].EndTime
			} else if contract.Style&ContractFlagFastrun != 0 && contract.Boosters[el].BoostState == BoostStateTokenTime {
				// Fastrun style keeps current booster in place
				orderedNames = append(orderedNames, el)
			} else {
				pos := SinkBoostFollowOrder
				if el == contract.Banker.BoostingSinkUserID {
					pos = contract.Banker.SinkBoostPosition
				}
				tvalPairs = append(tvalPairs, TValPair{
					name:     el,
					position: pos,
					val:      contract.Boosters[el].TokenValue,
					tokenAsk: contract.Boosters[el].TokensWanted,
				})
			}
		}
		sort.SliceStable(tvalPairs, func(i, j int) bool {
			// Keep Sink First Boost at the front of the list
			if tvalPairs[i].position == SinkBoostFirst {
				return true
			} else if tvalPairs[j].position == SinkBoostFirst {
				return false
			}
			// Keep Sink Last Boost at the end of the list
			if tvalPairs[i].position == SinkBoostLast {
				return false
			} else if tvalPairs[j].position == SinkBoostLast {
				return true
			}

			//if tvalPairs[i].tokenWant != tvalPairs[j].tokenWant {
			//	return tvalPairs[i].tokenWant > tvalPairs[j].tokenWant
			//}

			return tvalPairs[i].val > tvalPairs[j].val
		})

		newBoostPosition := len(orderedNames)
		if contract.Style&ContractFlagFastrun != 0 {
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

	case ContractOrderTE, ContractOrderTEplus:
		pairs := make([]teOrderPair, len(contract.Order))

		for i, name := range contract.Order {
			te := contract.Boosters[name].TECount

			if contract.BoostOrder == ContractOrderTEplus {
				// Add randomization factor of up to 10% of TE count
				te = max(te, 0)
				te += rand.IntN(te/10 + 1)
			}

			pairs[i] = teOrderPair{name: name, te: te}
		}

		sort.Slice(pairs, func(i, j int) bool {
			return pairs[i].te > pairs[j].te
		})

		for i := range pairs {
			contract.Order[i] = pairs[i].name
		}

	}

	if contract.BoostOrder != ContractOrderTVal {
		if contract.Style&ContractFlagBanker != 0 && contract.Banker.BoostingSinkUserID != "" {
			repositionSinkBoostPosition(contract)
		}
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
			if contract.State == ContractStateSignup && !currentTime.After(contractInfo.ValidUntil) {
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
		_ = finishContractByHash(s, hash)
	}

	// clear finishHash
	ei.ClearCoopStatusCachedData()

	track.ArchiveTrackerData(s)
}

// UpdateContractTime will update the contract start time and estimated duration
func UpdateContractTime(contractID string, coopID string, startTime, endTime time.Time, contractDurationSeconds float64) {
	// Update the contract start time and estimated duration
	contract := FindContractByIDs(contractID, coopID)
	if contract == nil || (contract.State != ContractStateCompleted && contract.State != ContractStateWaiting) {
		return
	}

	// Only update if startTime or EstimatedDuration are different
	newDuration := time.Duration(contractDurationSeconds) * time.Second
	if !contract.StartTime.Equal(startTime) || contract.EstimatedDuration != newDuration {
		contract.StartTime = startTime
		contract.EstimatedDuration = newDuration
		contract.EstimatedEndTime = endTime
		contract.EstimateUpdateTime = time.Now()
	}
}
