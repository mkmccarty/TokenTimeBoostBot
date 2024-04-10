package main

import (
	"flag"
	"fmt"
	"log"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/launch"
	"github.com/mkmccarty/TokenTimeBoostBot/src/notok"
	"github.com/mkmccarty/TokenTimeBoostBot/src/track"
	"github.com/mkmccarty/TokenTimeBoostBot/version"
	"github.com/xhit/go-str2duration/v2"
)

// Admin Slash Command Constants
const boostBotHomeGuild string = "766330702689992720"
const slashAdminContractsList string = "contract-list"
const slashAdminContractFinish string = "contract-finish"
const slashAdminBotSettings string = "bot-settings"

// Slash Command Constants
const slashContract string = "contract"
const slashSkip string = "skip"
const slashBoost string = "boost"
const slashChange string = "change"
const slashUnboost string = "unboost"
const slashPrune string = "prune"
const slashJoin string = "join"
const slashSetEggIncName string = "seteggincname"
const slashBump string = "bump"
const slashHelp string = "help"
const slashSpeedrun string = "speedrun"
const slashCoopETA string = "coopeta"
const slashLaunchHelper string = "launch-helper"
const slashToken string = "token"
const slashFun string = "fun"

var integerZeroMinValue float64 = 0.0

// const slashSignup string = "signup"
// Mutex
var mutex sync.Mutex
var s *discordgo.Session

// Version is set by the build system
var Version = "development"

var debugLogging = true

func init() {
	fmt.Printf("Starting Discord Bot: %s (%s)\n", version.Release, Version)

	// Read application parameters
	flag.Parse()

	// Read values from .env file
	err := config.ReadConfig("./.config.json")

	if err != nil {
		fmt.Println(err.Error())
		return
	}

	if *BotToken == "" {
		BotToken = &config.DiscordToken
	}

	if *AppID == "" {
		AppID = &config.DiscordAppID
	}

	if *GuildID == "" {
		GuildID = &config.DiscordGuildID
	}
}

func init() {
	var err error

	s, err = discordgo.New("Bot " + *BotToken)
	if err != nil {
		log.Fatalf("Invalid bot parameters: %v", err)
	}
}

// Bot parameters to override .config.json parameters
var (
	GuildID        = flag.String("guild", "", "Test guild ID")
	BotToken       = flag.String("token", "", "Bot access token")
	AppID          = flag.String("app", "", "Application ID")
	RemoveCommands = flag.Bool("rmcmd", false, "Remove all commands after shutdowning or not")

	adminCommands = []*discordgo.ApplicationCommand{
		{
			Name:        slashAdminContractsList,
			Description: "List all running contracts",
		},
		{
			Name:        slashAdminContractFinish,
			Description: "Mark a contract as finished",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "contract-hash",
					Description: "Hash of the contract to finish",
					Required:    true,
				},
			},
		},
		{
			Name:        slashAdminBotSettings,
			Description: "Set various bot settings",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "debug-logging",
					Description: "Enable or disable debug logging",
					Required:    true,
				},
			},
		},
	}

	commands = []*discordgo.ApplicationCommand{
		{
			Name:        slashContract,
			Description: "Contract Boosting Elections",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "contract-id",
					Description: "Contract ID",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "coop-id",
					Description: "Coop ID",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "coop-size",
					Description: "Co-op Size",
					Required:    true,
				},
				{
					Name:        "boost-order",
					Description: "Select how boost list is ordered. Default is Sign-up order.",
					Required:    false,
					Type:        discordgo.ApplicationCommandOptionInteger,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "Sign-up Ordering",
							Value: boost.ContractOrderSignup,
						},
						{
							Name:  "Fair Ordering",
							Value: boost.ContractOrderFair,
						},
						{
							Name:  "Random Ordering",
							Value: boost.ContractOrderRandom,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionRole,
					Name:        "ping-role",
					Description: "Role to use to ping for this contract. Default is @here.",
					Required:    false,
				},
			},
		},
		{
			Name:        slashSpeedrun,
			Description: "Add speedrun features to a contract.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "contract-starter",
					Description: "User who starts the EI contract.",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "sink",
					Description: "Token Sink",
					Required:    false,
				},
				{
					Name:        "sink-position",
					Description: "Default is First Booster",
					Required:    false,
					Type:        discordgo.ApplicationCommandOptionInteger,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "First",
							Value: boost.SinkBoostFirst,
						},
						{
							Name:  "Last",
							Value: boost.SinkBoostLast,
						},
					},
				},
				{
					Name:        "style",
					Description: "Style of speedrun. Default is Wonky",
					Required:    false,
					Type:        discordgo.ApplicationCommandOptionInteger,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "Wonky",
							Value: boost.SpeedrunStyleWonky,
						},
						{
							Name:  "Boost List",
							Value: boost.SpeedrunStyleFastrun,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "chicken-runs",
					Description: "Number of chicken runs for this contract.",
					MinValue:    &integerZeroMinValue,
					MaxValue:    20,
					Required:    false,
				},
			},
		},
		{
			Name:        slashJoin,
			Description: "Add farmer or guest to contract.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "farmer",
					Description: "User mention or guest name to add to existing contract",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "token-count",
					Description: "Set the number of boost tokens for this farmer. Default is 8.",
					MinValue:    &integerZeroMinValue,
					MaxValue:    14,
					Required:    false,
				},
				{
					Name:        "boost-order",
					Description: "Order farmer added to contract. Default is Signup order.",
					Required:    false,
					Type:        discordgo.ApplicationCommandOptionInteger,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "Sign-up Ordering",
							Value: boost.ContractOrderSignup,
						},
						{
							Name:  "Fair Ordering",
							Value: boost.ContractOrderFair,
						},
						{
							Name:  "Time Based Ordering",
							Value: boost.ContractOrderTimeBased,
						},
						{
							Name:  "Random Ordering",
							Value: boost.ContractOrderRandom,
						},
					},
				},
			},
		},
		{
			Name:        slashFun,
			Description: "OpenAI Fun",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "action",
					Description: "What interaction?",
					Required:    true,
					Type:        discordgo.ApplicationCommandOptionInteger,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "Wish for a token",
							Value: 1,
						},
						{
							Name:  "Compose letter asking for a token",
							Value: 5,
						},
						{
							Name:  "Let Me Out!",
							Value: 2,
						},
						{
							Name:  "Go Now!",
							Value: 3,
						},
						{
							Name:  "Generate image. Use prompt.",
							Value: 4,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "prompt",
					Description: "Optional prompt to fine tune the original query. For images it is used to describe the image.",
					Required:    false,
				},
			},
		},
		{
			Name:        slashBoost,
			Description: "Spending tokens to boost!",
			Options:     []*discordgo.ApplicationCommandOption{},
		},
		{
			Name:        slashSkip,
			Description: "Move current booster to last in boost order.",
			Options:     []*discordgo.ApplicationCommandOption{},
		},
		{
			Name:        slashUnboost,
			Description: "Change boost state to unboosted.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "farmer",
					Description: "User Mention",
					Required:    true,
				},
			},
		},
		{
			Name:        slashPrune,
			Description: "Prune Booster",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "farmer",
					Description: "User Mention",
					Required:    true,
				},
			},
		},
		{
			Name:        slashCoopETA,
			Description: "Display contract completion estimate.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "rate",
					Description: "Hourly production rate (i.e. 15.7q)",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "timespan",
					Description: "Time remaining in this contract. Example: 0d7h27m.",
					Required:    true,
				},
			},
		},
		{
			Name:        slashLaunchHelper,
			Description: "Display timestamp table for next mission.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "mission-duration",
					Description: "Time remaining for next mission(s). Example: 8h15m or \"8h15m, 10h5m, 1d2m\"",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "primary-ship",
					Description: "Select the primary ship to display. Default is Atreggies Henliner. [Sticky]",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "Atreggies Henliner",
							Value: 0,
						},
						{
							Name:  "Henerprise",
							Value: 1,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "secondary-ship",
					Description: "Select a secondary ship to display. Default is Henerprise. [Sticky]",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "None",
							Value: -1,
						},
						{
							Name:  "All Stars Club",
							Value: -2,
						},
						{
							Name:  "Starfleet Commander",
							Value: -3,
						},
						{
							Name:  "Henerprise",
							Value: 1,
						},
						{
							Name:  "Voyegger",
							Value: 2,
						},
						{
							Name:  "Defihent",
							Value: 3,
						},
						{
							Name:  "Galeggtica",
							Value: 4,
						},
						{
							Name:  "Cornish-Hen Corvette",
							Value: 5,
						},
						{
							Name:  "Quintillion Chicken",
							Value: 6,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "chain",
					Description: "Show return time for a chained Henliner extended mission. [Sticky]",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "dubcap-time",
					Description: "Time remaining for double capacity event. Examples: `43:16:22` or `43h16m22s`",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "ftl",
					Description: "FTL Drive Upgrades level. Default is 60.",
					MinValue:    &integerZeroMinValue,
					MaxValue:    60,
					Required:    false,
				},
			},
		},
		{
			Name:        slashToken,
			Description: "Display contract completion estimate.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "Unique name for this tracking session. i.e. Use coop-id of the contract.",
					Required:    true,
					MaxLength:   16, // Keep this short
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "duration",
					Description: "Time remaining in this contract. Example: 19h35m.",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "linked",
					Description: "Link with contract channel reactions for sent tokens. Default is true.",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "link-received",
					Description: "Link with contract channel reactions for received tokens. Default is false.",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "contract-channel",
					Description: "ChannelID or URL to Channel/Thread on Non-BootBot Server. Default is current channel.",
					Required:    false,
				},
			},
		},
		{
			Name:        slashChange,
			Description: "Change aspects of a running contract",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "coop-id",
					Description: "Change the coop-id",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "contract-id",
					Description: "Change the contract-id",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionRole,
					Name:        "ping-role",
					Description: "Change the contract ping role.",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "one-boost-position",
					Description: "Move a booster to a specific order position.  Example: @farmer 4",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "boost-order",
					Description: "Provide new boost order. Example: 1,2,3,6,7,5,8-10",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "current-booster",
					Description: "Change the current booster. Example: @farmer",
					Required:    false,
				},
			},
		},
		{
			Name:        slashSetEggIncName,
			Description: "Set Egg, Inc game name.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "ei-ign",
					Description: "Egg Inc IGN",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "discord-name",
					Description: "Discord name for this IGN assignment. Used by coordinator or admin to set another farmers IGN",
					Required:    false,
				},
			},
		},
		{
			Name:        slashBump,
			Description: "Redraw the boost list to the timeline.",
		},
		{
			Name:        slashHelp,
			Description: "Help with Boost Bot commands.",
		},
	}

	commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		// Admin Commands
		slashAdminContractsList: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleAdminContractList(s, i)
		},
		slashAdminContractFinish: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleAdminContractFinish(s, i)
		},
		// Normal Commands
		slashJoin: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			// Protection against DM use
			if i.GuildID == "" {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content:    "This command can only be run in a server.",
						Flags:      discordgo.MessageFlagsEphemeral,
						Components: []discordgo.MessageComponent{}},
				})
				return
			}
			var guestName = ""
			var orderValue int = boost.ContractOrderTimeBased // Default to Time Based
			var mention = ""
			var str = "Joining Member"

			// User interacting with bot, is this first time ?
			options := i.ApplicationCommandData().Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			if opt, ok := optionMap["farmer"]; ok {
				farmerName := opt.StringValue()
				if strings.HasPrefix(farmerName, "<@") {
					mention = farmerName
				} else {
					guestName = farmerName
				}
				str += " " + farmerName
			}
			if opt, ok := optionMap["token-count"]; ok {
				tokenWant := int(opt.IntValue())
				str += " with " + fmt.Sprintf("%d", tokenWant) + " boost order"
				if guestName != "" {
					farmerstate.SetTokens(guestName, tokenWant)
				} else {
					farmerstate.SetTokens(mention[2:len(mention)-1], tokenWant)
				}
			}
			if opt, ok := optionMap["boost-order"]; ok {
				orderValue = int(opt.IntValue())
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content:    str,
					Flags:      discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{}},
			})

			var err = boost.AddContractMember(s, i.GuildID, i.ChannelID, i.Member.Mention(), mention, guestName, orderValue)
			if err != nil {
				fmt.Println(err.Error())
			}
		},
		slashCoopETA: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			// Protection against DM use
			if i.GuildID == "" {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content:    "This command can only be run in a server.",
						Flags:      discordgo.MessageFlagsEphemeral,
						Components: []discordgo.MessageComponent{}},
				})
				return
			}
			var rate = ""
			var t = time.Now()
			var timespan = ""

			// User interacting with bot, is this first time ?
			options := i.ApplicationCommandData().Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			if opt, ok := optionMap["rate"]; ok {
				rate = opt.StringValue()
			}
			if opt, ok := optionMap["timespan"]; ok {
				timespan = opt.StringValue()
			}

			dur, _ := str2duration.ParseDuration(timespan)
			endTime := t.Add(dur)

			var str = fmt.Sprintf("With a production rate of %s/hr completion <t:%d:R> near <t:%d:f>", rate, endTime.Unix(), endTime.Unix())

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: str,
					//Flags:      discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{}},
			})
		},
		slashLaunchHelper: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			launch.HandleLaunchHelper(s, i)
		},
		slashToken: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			track.HandleTokenCommand(s, i)
		},
		slashSpeedrun: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleSpeedrunCommand(s, i)
		},
		slashContract: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			// Protection against DM use
			if i.GuildID == "" {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content:    "This command can only be run in a server.",
						Flags:      discordgo.MessageFlagsEphemeral,
						Components: []discordgo.MessageComponent{}},
				})
				return
			}
			var contractID = i.GuildID
			var coopID = i.GuildID // Default to the Guild ID
			var boostOrder = boost.ContractOrderSignup
			var coopSize = 2
			var ChannelID = i.ChannelID
			var pingRole = "@here"

			// User interacting with bot, is this first time ?
			options := i.ApplicationCommandData().Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			if opt, ok := optionMap["coop-size"]; ok {
				coopSize = int(opt.IntValue())
			}
			if opt, ok := optionMap["ping-role"]; ok {
				role := opt.RoleValue(nil, "")
				pingRole = role.Mention()
			}
			if opt, ok := optionMap["boost-order"]; ok {
				boostOrder = int(opt.IntValue())
			}
			if opt, ok := optionMap["contract-id"]; ok {
				contractID = opt.StringValue()
				contractID = strings.Replace(contractID, " ", "", -1)
			}
			if opt, ok := optionMap["coop-id"]; ok {
				coopID = opt.StringValue()
				coopID = strings.Replace(coopID, " ", "", -1)
			} else {
				var c, err = s.Channel(i.ChannelID)
				if err != nil {
					coopID = c.Name
				}
			}
			mutex.Lock()
			contract, err := boost.CreateContract(s, contractID, coopID, coopSize, boostOrder, i.GuildID, i.ChannelID, i.Member.User.ID, pingRole)
			mutex.Unlock()
			if err != nil {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content:    err.Error(),
						Flags:      discordgo.MessageFlagsEphemeral,
						Components: []discordgo.MessageComponent{}},
				})
				return
			}

			err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Boost Order Management",
					Flags:   discordgo.MessageFlagsEphemeral,
					// Buttons and other components are specified in Components field.
					Components: []discordgo.MessageComponent{
						// ActionRow is a container of all buttons within the same row.
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.Button{
									Label:    "Delete Contract",
									Style:    discordgo.DangerButton,
									Disabled: false,
									CustomID: "fd_delete",
								},
							},
						},
					},
				},
			})
			if err != nil {
				print(err)
			}

			var createMsg = boost.DrawBoostList(s, contract, boost.FindTokenEmoji(s, i.GuildID))
			msg, err := s.ChannelMessageSend(ChannelID, createMsg)
			if err == nil {
				boost.SetListMessageID(contract, ChannelID, msg.ID)
				var data discordgo.MessageSend
				data.Content, data.Components = boost.GetSignupComponents(false, contract.Speedrun)
				reactionMsg, err := s.ChannelMessageSendComplex(ChannelID, &data)

				if err != nil {
					print(err)
				} else if err == nil {
					boost.SetReactionID(contract, msg.ChannelID, reactionMsg.ID)
					s.ChannelMessagePin(msg.ChannelID, reactionMsg.ID)
				}
			} else {
				print(err)
			}

		},

		slashBoost: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			// Protection against DM use
			if i.GuildID == "" {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content:    "This command can only be run in a server.",
						Flags:      discordgo.MessageFlagsEphemeral,
						Components: []discordgo.MessageComponent{}},
				})
				return
			}
			var str = "Boosting!!"
			var err = boost.UserBoost(s, i.GuildID, i.ChannelID, i.Member.User.ID)
			if err != nil {
				str = err.Error()
			}
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content:    str,
					Flags:      discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{}},
			})

		},

		slashSkip: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			// Protection against DM use
			if i.GuildID == "" {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content:    "This command can only be run in a server.",
						Flags:      discordgo.MessageFlagsEphemeral,
						Components: []discordgo.MessageComponent{}},
				})
				return
			}
			var str = "Skip to Next Booster"
			var err = boost.SkipBooster(s, i.GuildID, i.ChannelID, "")
			if err != nil {
				str = err.Error()
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content:    str,
					Flags:      discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{}},
			})

		},
		slashUnboost: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			// Protection against DM use
			if i.GuildID == "" {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content:    "This command can only be run in a server.",
						Flags:      discordgo.MessageFlagsEphemeral,
						Components: []discordgo.MessageComponent{}},
				})
				return
			}
			var str = ""
			var farmer = ""
			options := i.ApplicationCommandData().Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			if opt, ok := optionMap["farmer"]; ok {
				farmer = opt.StringValue()
			}
			var err = boost.Unboost(s, i.GuildID, i.ChannelID, farmer)
			if err != nil {
				str = err.Error()
			} else {
				str = "Marked " + farmer + " as unboosted."
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content:    str,
					Flags:      discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{}},
			})

		},

		slashPrune: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			// Protection against DM use
			if i.GuildID == "" {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content:    "This command can only be run in a server.",
						Flags:      discordgo.MessageFlagsEphemeral,
						Components: []discordgo.MessageComponent{}},
				})
				return
			}
			var str = "Prune Booster"
			var farmer = ""

			options := i.ApplicationCommandData().Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			if opt, ok := optionMap["farmer"]; ok {
				farmer = opt.StringValue()
				str += " " + farmer
			}

			var err = boost.RemoveContractBoosterByMention(s, i.GuildID, i.ChannelID, i.Member.Mention(), farmer)
			if err != nil {
				fmt.Println("/prune", err.Error())
				str = err.Error()
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content:    str,
					Flags:      discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{}},
			})

		},
		slashSetEggIncName: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			// Protection against DM use
			if i.GuildID == "" {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content:    "This command can only be run in a server.",
						Flags:      discordgo.MessageFlagsEphemeral,
						Components: []discordgo.MessageComponent{}},
				})
				return
			}
			var eiName = ""
			var userID = i.Member.User.ID

			options := i.ApplicationCommandData().Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			if opt, ok := optionMap["discord-name"]; ok {
				farmerMention := opt.UserValue(s).Mention()
				re := regexp.MustCompile(`[\\<>@#&!]`)
				userID = re.ReplaceAllString(farmerMention, "")
			}

			var str = "Setting Egg, IGN for <@" + userID + "> to "

			if opt, ok := optionMap["ei-ign"]; ok {
				eiName = strings.TrimSpace(opt.StringValue())
				str += eiName
			}

			// if eiName matches this regex ^EI[1-9]*$ then it an invalid name
			re := regexp.MustCompile(`^EI[1-9]*$`)
			if re.MatchString(eiName) {
				str = "Don't use your Egg, Inc. EI number."
				eiName = ""
			} else {
				// Is the user issuing the command a coordinator?
				if userID != i.Member.User.ID && !boost.IsUserCreatorOfAnyContract(i.Member.User.ID) {
					str = "This form of usage is restricted to contract coordinators and administrators."
					eiName = ""
				} else {
					farmerstate.SetEggIncName(userID, eiName)
				}
			}

			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content:    str,
					Flags:      discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{}},
			})
			if err != nil {
				fmt.Println(err.Error())
			}

		},
		slashBump: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			str := "Redrawing the boost list"
			err := boost.RedrawBoostList(s, i.GuildID, i.ChannelID)
			if err != nil {
				str = err.Error()
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content:    str,
					Flags:      discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{}},
			})
		},
		slashChange: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			// Protection against DM use
			if i.GuildID == "" {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content:    "This command can only be run in a server.",
						Flags:      discordgo.MessageFlagsEphemeral,
						Components: []discordgo.MessageComponent{}},
				})
				return
			}
			var str = ""
			var contractID = ""
			var coopID = ""
			// User interacting with bot, is this first time ?
			options := i.ApplicationCommandData().Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			if opt, ok := optionMap["ping-role"]; ok {
				role := opt.RoleValue(nil, "")
				err := boost.ChangePingRole(s, i.GuildID, i.ChannelID, i.Member.User.ID, role.Mention())
				if err != nil {
					str += err.Error()
				} else {
					str = "Changed ping role to " + role.Mention() + "\n"
				}
			}
			if opt, ok := optionMap["contract-id"]; ok {
				contractID = opt.StringValue()
				contractID = strings.Replace(contractID, " ", "", -1)
				str += "Contract ID changed to " + contractID
			}
			if opt, ok := optionMap["coop-id"]; ok {
				coopID = opt.StringValue()
				coopID = strings.Replace(coopID, " ", "", -1)
				str += "Coop ID changed to " + coopID
			}

			if contractID != "" || coopID != "" {
				err := boost.ChangeContractIDs(s, i.GuildID, i.ChannelID, i.Member.User.ID, contractID, coopID)
				if err != nil {
					str += err.Error()
				}
			}

			currentBooster := ""
			boostOrder := ""
			oneBoosterName := ""
			oneBoosterPosition := 0
			if opt, ok := optionMap["current-booster"]; ok {
				currentBooster = strings.TrimSpace(opt.StringValue())
			}
			if opt, ok := optionMap["boost-order"]; ok {
				boostOrder = strings.TrimSpace(opt.StringValue())
			}
			if opt, ok := optionMap["one-boost-position"]; ok {
				// String in the form of mention
				boosterString := strings.TrimSpace(opt.StringValue())

				// split string into slice by space, comma or colon
				boosterSlice := strings.FieldsFunc(boosterString, func(r rune) bool {
					return r == ' ' || r == ',' || r == ':'
				})
				if len(boosterSlice) >= 2 {

					// booster name is boosterString without the last element of boosterSlice
					oneBoosterName = strings.TrimSuffix(boosterString, boosterSlice[len(boosterSlice)-1])
					oneBoosterName = strings.TrimSpace(oneBoosterName)
					// Trim last character from oneBoosterName
					oneBoosterName = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(oneBoosterName, ":"), ","))

					re := regexp.MustCompile(`[\\<>@#&!]`)
					oneBoosterName = re.ReplaceAllString(oneBoosterName, "")

					// convert string to int
					oneBoosterPosition, _ = strconv.Atoi(boosterSlice[1])
				} else {
					str = "The one-boost-position parameter needs to be in the form of @farmer <space> 4"
				}
			}

			// Either change a single booster or the whole list
			// Cannot change one booster's position and make them boost
			if oneBoosterName != "" && oneBoosterPosition != 0 {
				err := boost.MoveBooster(s, i.GuildID, i.ChannelID, i.Member.User.ID, oneBoosterName, oneBoosterPosition, currentBooster == "")
				if err != nil {
					str += err.Error()
				}
			} else {
				if boostOrder != "" {
					err := boost.ChangeBoostOrder(s, i.GuildID, i.ChannelID, i.Member.User.ID, boostOrder, currentBooster == "")
					if err != nil {
						str += err.Error()
					}
				}
			}

			if currentBooster != "" {
				err := boost.ChangeCurrentBooster(s, i.GuildID, i.ChannelID, i.Member.User.ID, currentBooster, true)
				if err != nil {
					str += err.Error()
				}
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content:    str,
					Flags:      discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{}},
			})
		},
		slashHelp: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleHelpCommand(s, i)
		},
		slashFun: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			notok.FunHandler(s, i)
		},
	}

	componentHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"fd_delete": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			// Delete coop
			var str = "Contract not found."
			coopName, err := boost.DeleteContract(s, i.GuildID, i.ChannelID)
			if err == nil {
				str = fmt.Sprintf("Contract %s deleted.", coopName)
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content:    str,
					Flags:      discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{}},
			})
			s.ChannelMessageDelete(i.ChannelID, i.Message.ID)
		},
		"fd_tokens5": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			addBoostTokens(s, i, 5, 0)
		},
		"fd_tokens6": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			addBoostTokens(s, i, 6, 0)
		},
		"fd_tokens8": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			addBoostTokens(s, i, 8, 0)
		},
		"fd_tokens1": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			addBoostTokens(s, i, 0, 1)
		},
		"fd_tokens_sub": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			addBoostTokens(s, i, 0, -1)
		},
		"fd_tokenStartHourPlus": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			track.TokenAdjustTimestamp(s, i, 1, 0, 0, 0)
		},
		"fd_tokenStartHourMinus": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			track.TokenAdjustTimestamp(s, i, -1, 0, 0, 0)
		},
		"fd_tokenStartMinutePlusFive": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			track.TokenAdjustTimestamp(s, i, 0, 5, 0, 0)
		},
		"fd_tokenStartMinutePlusOne": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			track.TokenAdjustTimestamp(s, i, 0, 1, 0, 0)
		},
		"fd_tokenDurationHourPlus": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			track.TokenAdjustTimestamp(s, i, 0, 0, 1, 0)
		},
		"fd_tokenDurationHourMinus": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			track.TokenAdjustTimestamp(s, i, 0, 0, -1, 0)
		},
		"fd_tokenDurationMinutePlusFive": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			track.TokenAdjustTimestamp(s, i, 0, 0, 0, 5)
		},
		"fd_tokenDurationMinutePlusOne": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			track.TokenAdjustTimestamp(s, i, 0, 0, 0, 1)
		},
		"fd_tokenEdit": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			track.HandleTokenEdit(s, i)
		},
		"fd_tokenSent": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			track.HandleTokenSend(s, i)
		},
		"fd_tokenReceived": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			track.HandleTokenReceived(s, i)
		},
		"fd_tokenDetails": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			track.HandleTokenDetails(s, i)
		},
		"fd_tokenComplete": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			track.HandleTokenComplete(s, i)
		},

		"fd_signupStart": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredMessageUpdate,
				Data: &discordgo.InteractionResponseData{
					Content:    "",
					Flags:      discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{}},
			})
			err := boost.StartContractBoosting(s, i.GuildID, i.ChannelID, i.Member.User.ID)
			if err != nil {
				str := fmt.Sprint(err.Error())
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content:    str,
						Flags:      discordgo.MessageFlagsEphemeral,
						Components: []discordgo.MessageComponent{}},
				})
			} else {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{})

				// Rebuild the signup message to disable the start button
				msg := discordgo.NewMessageEdit(i.ChannelID, i.Message.ID)
				contentStr, comp := boost.GetSignupComponents(true, false) // True to get a disabled start button
				msg.SetContent(contentStr)
				msg.Components = &comp
				s.ChannelMessageEditComplex(msg)
			}
		},
		"fd_signupFarmer": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			joinContract(s, i, false)
		},
		"fd_signupBell": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			joinContract(s, i, true)
		},
		"fd_signupLeave": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			str := "Removed from Contract"
			var err = boost.RemoveContractBoosterByMention(s, i.GuildID, i.ChannelID, i.Member.Mention(), i.Member.Mention())
			if err != nil {
				str = err.Error()
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content:    str,
					Flags:      discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{}},
			})
		},
	}
)

func joinContract(s *discordgo.Session, i *discordgo.InteractionCreate, bell bool) {
	var str = "Adding to Contract..."

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
		Data: &discordgo.InteractionResponseData{
			Content:    str,
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})

	err := boost.JoinContract(s, i.GuildID, i.ChannelID, i.Member.User.ID, bell)
	if err != nil {
		str = err.Error()
		fmt.Print(str)
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    str,
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})
}

// main init to call other init functions in sequence
func init() {
	s.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		switch i.Type {
		case discordgo.InteractionApplicationCommand:
			if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
				if debugLogging {
					options := i.ApplicationCommandData().Options
					optionMap := make(map[string]string, len(options))
					for _, opt := range options {
						switch opt.Type {
						case discordgo.ApplicationCommandOptionString:
							optionMap[opt.Name] = opt.StringValue()
						case discordgo.ApplicationCommandOptionInteger:
							optionMap[opt.Name] = strconv.Itoa(int(opt.IntValue()))
						case discordgo.ApplicationCommandOptionBoolean:
							optionMap[opt.Name] = strconv.FormatBool(opt.BoolValue())
						case discordgo.ApplicationCommandOptionUser:
							optionMap[opt.Name] = opt.UserValue(s).Username
						default:
							optionMap[opt.Name] = "Unknown"
						}
					}
					if i.GuildID != "" {
						log.Println("Command:", i.ApplicationCommandData().Name, optionMap, i.ChannelID, i.Member.User.ID)
					}
				}
				h(s, i)
			}
		case discordgo.InteractionMessageComponent:

			if h, ok := componentHandlers[i.MessageComponentData().CustomID]; ok {
				userID := ""
				if i.GuildID == "" {
					userID = i.User.ID
				} else {
					userID = i.Member.User.ID
				}
				log.Println("Component: ", i.MessageComponentData().CustomID, userID)
				h(s, i)
			}
		}
	})

	// Components are part of interactions, so we register InteractionCreate handler
	s.AddHandler(func(s *discordgo.Session, m *discordgo.MessageReactionAdd) {
		if m.MessageReaction.UserID != s.State.User.ID {
			if m.GuildID != "" {
				var str = boost.ReactionAdd(s, m.MessageReaction)
				if str == "!gonow" {
					notok.DoGoNow(s, m.ChannelID)
				}
			} else {
				track.ReactionAdd(s, m.MessageReaction)
			}
		}
	})
	s.AddHandler(func(s *discordgo.Session, m *discordgo.MessageReactionRemove) {
		if m.MessageReaction.UserID != s.State.User.ID {
			boost.ReactionRemove(s, m.MessageReaction)
		}
	})

}

func addBoostTokens(s *discordgo.Session, i *discordgo.InteractionCreate, valueSet int, valueAdj int) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
	})

	boost.AddBoostTokens(s, i.GuildID, i.ChannelID, i.Member.User.ID, valueSet, valueAdj, 0)
}

func main() {

	/*
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	*/
	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
	})
	err := s.Open()
	if err != nil {
		log.Fatalf("Cannot open the session: %v", err)
	}
	if *RemoveCommands {
		// Delete Guild Specific commands
		cmds, err := s.ApplicationCommands(config.DiscordAppID, config.DiscordGuildID)
		if (err == nil) && (len(cmds) > 0) {
			// loop through all cmds
			for _, cmd := range cmds {
				// delete each cmd
				s.ApplicationCommandDelete(config.DiscordAppID, config.DiscordGuildID, cmd.ID)
			}
		}

		// Delete global commands
		if config.DiscordGuildID != "" {
			cmds, err = s.ApplicationCommands(config.DiscordAppID, "")
			if (err == nil) && (len(cmds) > 0) {
				// loop through all cmds
				for _, cmd := range cmds {
					// delete each cmd
					s.ApplicationCommandDelete(config.DiscordAppID, "", cmd.ID)
				}
			}
		}
	}

	log.Println("Adding commands...")
	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands)+len(adminCommands))
	for i, v := range commands {
		cmd, err := s.ApplicationCommandCreate(s.State.User.ID, config.DiscordGuildID, v)
		if err != nil {
			log.Printf("Cannot create '%v' command: %v", v.Name, err)
		}
		registeredCommands[i] = cmd
	}
	// Admin Commands exist only for the BoostBot Home Guild
	for i, v := range adminCommands {
		cmd, err := s.ApplicationCommandCreate(s.State.User.ID, boostBotHomeGuild, v)
		if err != nil {
			log.Printf("Cannot create '%v' command: %v", v.Name, err)
		}
		registeredCommands[len(commands)+i] = cmd
	}

	defer s.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	log.Println("Press Ctrl+C to exit")
	<-stop

	if *RemoveCommands {
		log.Println("Removing commands...")

		//registeredCommands, err := s.ApplicationCommands(s.State.User.ID, *GuildID)
		if err == nil {
			for _, v := range registeredCommands {
				err := s.ApplicationCommandDelete(s.State.User.ID, config.DiscordGuildID, v.ID)
				log.Printf("Delete command '%v' command.", v.Name)
				if err != nil {
					log.Printf("Cannot delete '%v' command: %v\n", v.Name, err)
				}
			}
		}
	}

	log.Println("Graceful shutdown")
}
