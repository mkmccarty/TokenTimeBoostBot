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
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/launch"
	"github.com/mkmccarty/TokenTimeBoostBot/src/notok"
	"github.com/mkmccarty/TokenTimeBoostBot/src/tasks"
	"github.com/mkmccarty/TokenTimeBoostBot/src/track"
	"github.com/mkmccarty/TokenTimeBoostBot/src/version"
)

// Admin Slash Command Constants
const boostBotHomeGuild string = "766330702689992720"
const slashAdminContractsList string = "contract-list"
const slashAdminContractFinish string = "contract-finish"
const slashAdminBotSettings string = "bot-settings"
const slashReloadContracts string = "reload-contracts"

// Slash Command Constants
const slashContract string = "contract"
const slashSkip string = "skip"
const slashBoost string = "boost"
const slashChange string = "change"
const slashChangeOneBooster string = "change-one-booster"
const slashChangePingRole string = "change-ping-role"
const slashUnboost string = "unboost"
const slashPrune string = "prune"
const slashJoin string = "join-contract"
const slashSetEggIncName string = "seteggincname"
const slashBump string = "bump"
const slashHelp string = "help"
const slashSpeedrun string = "speedrun"
const slashCoopETA string = "coopeta"
const slashLaunchHelper string = "launch-helper"
const slashToken string = "token"
const slashTokenRemove string = "token-remove"
const slashCalcContractTval string = "calc-contract-tval"
const slashVolunteerSink string = "volunteer-sink"
const slashVoluntellSink string = "voluntell-sink"
const slashFun string = "fun"

var integerZeroMinValue float64 = 0.0

// const slashSignup string = "signup"
var s *discordgo.Session

// Version is set by the build system
var Version = "development"

var debugLogging = true

func init() {
	log.Printf("Starting Discord Bot: %s (%s)\n", version.Release, Version)

	// Read application parameters
	flag.Parse()

	// Read values from .env file
	err := config.ReadConfig("./.config.json")

	if err != nil {
		log.Println(err.Error())
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
		{
			Name:        slashReloadContracts,
			Description: "Manual check for new Egg Inc contract data.",
		},
	}

	globalCommands = []*discordgo.ApplicationCommand{
		launch.SlashLaunchHelperCommand(slashLaunchHelper),
		track.GetSlashTokenCommand(slashToken),
		track.GetSlashTokenRemoveCommand(slashTokenRemove),
	}

	commands = []*discordgo.ApplicationCommand{
		{
			Name:        slashContract,
			Description: "Contract Boosting Elections",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "contract-id",
					Description:  "Contract ID",
					Required:     true,
					Autocomplete: true,
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
					Description: "Co-op Size. This will be pulled from EI Contract data if unset.",
					Required:    false,
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
					Description: "Number of chicken runs for this contract. Optional if contract-id was selected via auto fill.",
					MinValue:    &integerZeroMinValue,
					MaxValue:    20,
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "self-runs",
					Description: "Self Runs during CRT",
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
		notok.SlashFunCommand(slashFun),
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
		boost.GetSlashVolunteerSink(slashVolunteerSink),
		boost.GetSlashVoluntellSink(slashVoluntellSink),
		boost.GetSlashCalcContractTval(slashCalcContractTval),
		boost.GetSlashChangeCommand(slashChange),
		boost.GetSlashChangeOneBoosterCommand(slashChangeOneBooster),
		boost.GetSlashChangePingRoleCommand(slashChangePingRole),
		farmerstate.SlashSetEggIncNameCommand(slashSetEggIncName),
		{
			Name:        slashBump,
			Description: "Redraw the boost list to the timeline.",
		},
		{
			Name:        slashHelp,
			Description: "Help with Boost Bot commands.",
		},
	}

	autocompleteHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		// Admin Commands
		slashContract: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleContractAutoComplete(s, i)
		},
		slashChange: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleContractAutoComplete(s, i)
		},
		slashTokenRemove: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			if i.GuildID == "" {
				str, choices := track.HandleTokenRemoveAutoComplete(s, i)
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionApplicationCommandAutocompleteResult,
					Data: &discordgo.InteractionResponseData{
						Content: str,
						Choices: choices,
					}})

			} else {
				str, choices := boost.HandleTokenRemoveAutoComplete(s, i)
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionApplicationCommandAutocompleteResult,
					Data: &discordgo.InteractionResponseData{
						Content: str,
						Choices: choices,
					}})
			}
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
		slashReloadContracts: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			tasks.HandleReloadContractsCommand(s, i)
		},
		// Normal Commands
		slashJoin: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleJoinCommand(s, i)
		},
		slashCoopETA: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleCoopETACommand(s, i)
		},
		slashLaunchHelper: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			launch.HandleLaunchHelper(s, i)
		},
		slashToken: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			track.HandleTokenCommand(s, i)
		},
		slashTokenRemove: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			var str = "Token not found."
			if i.GuildID == "" {
				str = track.HandleTokenRemoveCommand(s, i)
			} else {
				str = boost.HandleTokenRemoveCommand(s, i)
			}
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: str,
					Flags:   discordgo.MessageFlagsEphemeral,
				}})
		},
		slashCalcContractTval: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleContractCalcContractTvalCommand(s, i)
		},
		slashSpeedrun: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleSpeedrunCommand(s, i)
		},
		slashVolunteerSink: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleSlashVolunteerSinkCommand(s, i)
		},
		slashVoluntellSink: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleSlashVoluntellSinkCommand(s, i)
		},
		slashContract: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleContractCommand(s, i)
		},
		slashBoost: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleBoostCommand(s, i)
		},
		slashSkip: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleSkipCommand(s, i)
		},
		slashUnboost: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleUnboostCommand(s, i)
		},
		slashPrune: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandlePruneCommand(s, i)
		},
		slashBump: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleBumpCommand(s, i)
		},
		slashChange: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleChangeCommand(s, i)
		},
		slashChangeOneBooster: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleChangeOneBoosterCommand(s, i)
		},
		slashChangePingRole: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleChangePingRoleCommand(s, i)
		},
		slashHelp: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleHelpCommand(s, i)
		},
		slashFun: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			notok.FunHandler(s, i)
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
				log.Println(err.Error())
			}

		},
	}

	componentHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"fd_delete": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleContractDelete(s, i)
		},
		"fd_tokens5": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.AddBoostTokens(s, i, 5, 0)
		},
		"fd_tokens6": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.AddBoostTokens(s, i, 6, 0)
		},
		"fd_tokens8": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.AddBoostTokens(s, i, 8, 0)
		},
		"fd_tokens1": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.AddBoostTokens(s, i, 0, 1)
		},
		"fd_tokens_sub": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.AddBoostTokens(s, i, 0, -1)
		},
		"fd_tokenEdit": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			track.HandleTokenEdit(s, i)
		},
		"fd_trackerEdit": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			track.HandleTrackerEdit(s, i)
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
				s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
					Content: str,
				})
			} else {
				s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{})

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
			var err = boost.RemoveContractBoosterByMention(s, i.GuildID, i.ChannelID, i.Member.User.Mention(), i.Member.User.Mention())
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
		log.Print(str)
	}
	//else {
	//	str = fmt.Sprintf("Added <@%s> to contract", i.Member.User.ID)
	//}

	s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		//		Content: str,
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
					} else {
						log.Println("Command-DM:", i.ApplicationCommandData().Name, optionMap, i.ChannelID, i.User.ID)
					}
				}
				h(s, i)
			}
		case discordgo.InteractionApplicationCommandAutocomplete:
			if h, ok := autocompleteHandlers[i.ApplicationCommandData().Name]; ok {
				h(s, i)
			}

		case discordgo.InteractionModalSubmit:
			// Handlers could include a parameter to help identify this uniquly
			handlerID := strings.Split(i.ModalSubmitData().CustomID, "#")[0]
			if h, ok := componentHandlers[handlerID]; ok {
				userID := ""
				if i.GuildID == "" {
					userID = i.User.ID
				} else {
					userID = i.Member.User.ID
				}
				log.Println("Component: ", i.ModalSubmitData().CustomID, userID)
				h(s, i)
			}
		case discordgo.InteractionMessageComponent:
			// Handlers could include a parameter to help identify this uniquly
			handlerID := strings.Split(i.MessageComponentData().CustomID, "#")[0]

			if h, ok := componentHandlers[handlerID]; ok {
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

func main() {
	/*
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	*/

	// Start our CRON job to grab Egg Inc contract data from the Carpet github repository
	go tasks.ExecuteCronJob()

	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Ready message for: %v#%v", s.State.User.Username, s.State.User.Discriminator)
		log.Printf("Ready Vers:%v  SessId:%v", r.Version, r.SessionID)
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
				log.Printf("Delete command '%v' command.", cmd.Name)
				s.ApplicationCommandDelete(config.DiscordAppID, config.DiscordGuildID, cmd.ID)
			}
		}

		// Delete global commands
		if config.DiscordGuildID != `` {
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
		gid := config.DiscordGuildID
		cmd, err := s.ApplicationCommandCreate(s.State.User.ID, gid, v)
		if err != nil {
			log.Printf("Cannot create '%v' command: %v", v.Name, err)
		}
		registeredCommands[i] = cmd
	}
	// Global Commands exist only for the BoostBot Home Guild
	for i, v := range globalCommands {
		cmd, err := s.ApplicationCommandCreate(s.State.User.ID, "", v)
		if err != nil {
			log.Printf("Cannot create '%v' command: %v", v.Name, err)
		}
		registeredCommands[len(commands)+i] = cmd
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

	boost.SaveAllData()
	track.SaveAllData()

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
