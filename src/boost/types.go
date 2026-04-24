package boost

import (
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

var mutex sync.Mutex

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

const defaultFamerTokens = 6
const signupThreadBackstopDuration = 7 * 24 * time.Hour

var boostIconName = "🚀"     // For Reaction tests
var boostIconReaction = "🚀" // For displaying
var boostIcon = "🚀"         // For displaying

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
	ContractOrderSignup    = 0  // Signup order
	ContractOrderReverse   = 1  // Reverse order
	ContractOrderRandom    = 2  // Randomized when the contract starts. After 20 minutes the order changes to Sign-up.
	ContractOrderFair      = 3  // Fair based on position percentile of each farmers last 5 contracts. Those with no history use 50th percentile
	ContractOrderTimeBased = 4  // Time based order
	ContractOrderELR       = 5  // ELR based order
	ContractOrderTVal      = 6  // Token Value based order
	ContractOrderTokenAsk  = 7  // Token Ask order, less tokens boosts earlier
	ContractOrderTE        = 8  // Truth Egg based order
	ContractOrderTEplus    = 9  // Truth Egg + randomization
	ContractManualOrder    = 10 // Manual order set by contract creator

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
	ContractHash              string // ContractID-CoopID
	Location                  []*LocationData
	CreatorID                 []string // Slice of creators
	BannerURL                 string   // URL for the contract banner
	ContractID                string   // Contract ID
	CoopID                    string   // CoopID
	PredictionSignup          bool     // True if this contract is/was a prediction
	SeasonalScoring           int      // 1 = new scoring
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

	CoopSize             int
	Ultra                bool
	UltraCount           int
	Style                int64 // Mask for the Contract Style
	PlayStyle            int   // Playstyle of the contract
	NewToPlayStyle       bool  // Someone in the contract is new to this playstyle
	LengthInSeconds      int
	BoostOrder           int // How the contract is sorted
	BoostVoting          int
	CurrentBoosterUserID string    // Current booster UserID (source of truth)
	BoostPosition        int       // Starting Slot
	State                int       // Boost Completed
	StartTime            time.Time // When Contract is started
	EndTime              time.Time // When final booster ends
	PlannedStartTime     time.Time // Parameter start time
	ActualStartTime      time.Time // Actual start time for token tracking
	ValidFrom            time.Time // Base time used for offsets (9 AM PT of creation day)
	RegisteredNum        int
	Boosters             map[string]*Booster // Boosters Registered
	CRMessageIDs         map[string]string   // CR reqest messageIDs
	WaitlistBoosters     []string            // Waitlist of UserID's
	AltIcons             []string            // Array of alternate icons for the Boosters
	Order                []string
	BoostedOrder         []string   // Actual order of boosting
	OrderRevision        int        // Incremented when Order is changed
	Banker               BankerInfo // Banker for the contract
	TokenLog             []ei.TokenUnitLog
	TokensPerMinute      float64
	CalcOperations       int
	CalcOperationTime    time.Time
	CoopTokenValueMsgID  string
	LastWishPrompt       string             // saved prompt for this contract
	LastInteractionTime  time.Time          // last time the contract was drawn
	buttonComponents     map[string]CompMap // Cached components for this contract
	NewFeature           int                // Used to slide in new features
	DynamicData          *DynamicTokenData
	LastSaveTime         time.Time // The last time the contract was saved

	mutex sync.Mutex // Keep this contract thread safe
}
