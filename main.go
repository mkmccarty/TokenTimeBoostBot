package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/fsnotify/fsnotify"
	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/events"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/menno"
	"github.com/mkmccarty/TokenTimeBoostBot/src/notok"
	"github.com/mkmccarty/TokenTimeBoostBot/src/tasks"
	"github.com/mkmccarty/TokenTimeBoostBot/src/track"
	"github.com/mkmccarty/TokenTimeBoostBot/src/version"
	"github.com/natefinch/lumberjack/v3"
)

const configFileName = ".config.json"
const statusMessagesFileName = "ttbb-data/status-messages.json"

// Admin Slash Command Constants
// const boostBotHomeGuild string = "766330702689992720"
const slashAdminContractsList string = "admin-contract-list"
const slashAdminContractFinish string = "admin-contract-finish"

// const slashAdminBotSettings string = "admin-bot-settings"
const slashReloadContracts string = "admin-reload-contracts"
const slashAdminGetContractData string = "admin-get-contract-data"
const slashAdminListRoles string = "list-roles"

// Slash Command Constants
const slashContract string = "contract"
const slashSkip string = "skip"
const slashBoost string = "boost"
const slashChangeOneBooster string = "change-one-booster"
const slashChangePlannedStartCommand string = "change-start"
const slashChangeSpeedRunSink string = "change-speedrun-sink"
const slashChangeCommand string = "change"
const slashUpdateCommand string = "update"
const slashUnboost string = "unboost"
const slashPrune string = "prune"
const slashJoin string = "join-contract"
const slashSetEggIncName string = "seteggincname"
const slashBump string = "bump"
const slashToggleContractPings string = "toggle-contract-pings"
const slashContractSettings string = "contract-settings"
const slashHelp string = "help"
const slashSpeedrun string = "speedrun"
const slashCoopETA string = "coopeta"
const slashLaunchHelper string = "launch-helper"
const slashEventHelper string = "events"
const slashToken string = "token"

// const slashTokenRemove string = "token-remove"
const slashTokenEdit string = "token-edit"
const slashTokenEditTrack string = "token-edit-track"
const slashCalcContractTval string = "calc-contract-tval"
const slashCoopTval string = "coop-tval"
const slashVolunteerSink string = "volunteer-sink"
const slashVoluntellSink string = "voluntell-sink"
const slashLinkAlternate string = "link-alternate"
const slashTeamworkEval string = "teamwork"
const slashEstimateTime string = "estimate-contract-time"
const slashCsEstimate string = "cs-estimate"
const slashRenameThread string = "rename-thread"
const slashFun string = "fun"
const slashStones string = "stones"
const slashTimer string = "timer"
const slashArtifact string = "artifact"
const slashScoreExplorer string = "score-explorer"
const slashRemoveDMMessage string = "remove-dm-message"
const slashPrivacy string = "privacy"
const slashRerunEval string = "rerun-eval"
const slashContractReport string = "contract-report"
const slashVirtue string = "virtue"
const slashRegister string = "register"
const slashHunt string = "hunt"
const slashPredictions string = "predictions"

var integerZeroMinValue float64 = 0.0

// const slashSignup string = "signup"
var s *discordgo.Session

// Version is set by the build system
var Version = "development"

var debugLogging = true

func init() {
	// Read values from .env file
	err := config.ReadConfig(configFileName)
	if err != nil {
		log.Println(err.Error())
		return
	}

	// Only logg to a file when not using the dev bot
	if !config.IsDevBot() {

		l, _ := lumberjack.NewRoller(fmt.Sprintf("%s/BoostBot.log", "ttbb-data"),
			1*1024*1024, // 1 megabyte
			&lumberjack.Options{
				MaxBackups: 12,
				MaxAge:     28 * time.Hour * 24, // 28 days
				Compress:   false,
			})
		log.SetOutput(l)
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGHUP)

		go func() {
			for {
				<-c
				_ = l.Rotate()
			}
		}()
	}

	version.Version = Version
	log.Printf("Starting Discord Bot: %s (%s)\n", version.Release, Version)
	// For the daemon Log
	fmt.Printf("Starting Discord Bot: %s (%s) at %s\n", version.Release, Version, time.Now().Format(time.RFC3339))

	// Load status messages
	ei.LoadStatusMessages(statusMessagesFileName)

	// Read application parameters
	flag.Parse()

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
	// if ttbb-data directory doesn't exist, create it
	if _, err := os.Stat("ttbb-data"); os.IsNotExist(err) {
		err := os.Mkdir("ttbb-data", 0755)
		if err != nil {
			log.Print(err)
		}
	}
}

// Bot parameters to override .config.json parameters
var (
	GuildID         = flag.String("guild", "", "Test guild ID")
	BotToken        = flag.String("token", "", "Bot access token")
	AppID           = flag.String("app", "", "Application ID")
	RemoveCommands  = flag.Bool("rmcmd", false, "Remove all commands after shutdowning or not")
	adminPermission = int64(0)

	adminCommands = []*discordgo.ApplicationCommand{
		{
			Name:                     slashAdminContractsList,
			Description:              "List all running contracts",
			DefaultMemberPermissions: &adminPermission,
		},
		{
			Name:                     slashAdminContractFinish,
			Description:              "Mark a contract as finished",
			DefaultMemberPermissions: &adminPermission,
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
			Name:                     slashReloadContracts,
			Description:              "Manual check for new Egg Inc contract data.",
			DefaultMemberPermissions: &adminPermission,
		},
		boost.SlashAdminGetContractData(slashAdminGetContractData),
		boost.SlashAdminListRoles(slashAdminListRoles),
	}

	globalCommands = []*discordgo.ApplicationCommand{
		events.SlashLaunchHelperCommand(slashLaunchHelper),
		events.SlashEventHelperCommand(slashEventHelper),
		track.GetSlashTokenCommand(slashToken),
		track.GetSlashTokenEditTrackCommand(slashTokenEditTrack),
		boost.GetSlashReplayEvalCommand(slashRerunEval),
		boost.GetSlashVirtueCommand(slashVirtue),
		boost.GetSlashRegisterCommand(slashRegister),
		// This isn't ready yet
		menno.SlashHuntCommand(slashHunt),
	}

	commands = []*discordgo.ApplicationCommand{

		boost.GetSlashContractCommand(slashContract),
		boost.GetSlashSpeedrunCommand(slashSpeedrun),
		boost.GetSlashRenameThread(slashRenameThread),
		boost.SlashArtifactsCommand(slashArtifact),
		boost.GetSlashScoreExplorerCommand(slashScoreExplorer),
		boost.GetSlashChangeSpeedRunSinkCommand(slashChangeSpeedRunSink),
		boost.GetSlashUpdateCommand(slashUpdateCommand),
		boost.GetSlashChangeCommand(slashChangeCommand),
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
						/*
							{
								Name:  "Fair Ordering",
								Value: boost.ContractOrderFair,
							},
						*/
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
		boost.GetSlasLinkAlternateCommand(slashLinkAlternate),
		boost.GetSlashCalcContractTval(slashCalcContractTval),
		boost.GetSlashCoopTval(slashCoopTval),
		boost.GetSlashTeamworkEval(slashTeamworkEval),
		boost.GetSlashContractReportCommand(slashContractReport),
		boost.GetPredictionsCommand(slashPredictions),
		//boost.GetSlashSiabEval(slashSiabEval),
		boost.GetSlashStones(slashStones),
		boost.GetSlashTimer(slashTimer),
		boost.GetSlashEstimateTime(slashEstimateTime),
		boost.GetSlashCsEstimates(slashCsEstimate),
		boost.GetSlashChangeOneBoosterCommand(slashChangeOneBooster),
		boost.GetSlashChangePlannedStartCommand(slashChangePlannedStartCommand),
		bottools.GetSlashRemoveMessage(slashRemoveDMMessage),
		boost.GetSlashTokenEditCommand(slashTokenEdit),
		farmerstate.SlashSetEggIncNameCommand(slashSetEggIncName),
		farmerstate.GetSlashPrivacyCommand(slashPrivacy),
		{
			Name:        slashBump,
			Description: "Redraw the boost list to the timeline.",
		},
		{
			Name:        slashToggleContractPings,
			Description: "Toggle Boost Bot contract pings [sticky]",
		},
		boost.GetSlashHelpCommand(slashHelp),
		{
			Name:        slashContractSettings,
			Description: "Coordinator of contract can use this to show initial settings",
		},
	}

	autocompleteHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		// Admin Commands
		slashContract: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleContractAutoComplete(s, i)
		},
		slashChangeCommand: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleContractAutoComplete(s, i)
		},
		slashRerunEval: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleAllContractsAutoComplete(s, i)
		},
		slashAdminGetContractData: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleCoopAutoComplete(s, i)
		},
		slashAdminListRoles: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleContractAutoComplete(s, i)
		},
		slashToken: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleContractAutoComplete(s, i)
		},
		slashEstimateTime: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleAllContractsAutoComplete(s, i)
		},
		slashCsEstimate: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleAllContractsAutoComplete(s, i)
		},
		slashLinkAlternate: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleLinkAlternateAutoComplete(s, i)
		},
		slashCalcContractTval: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleAltsAutoComplete(s, i)
		},
		slashStones: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleAllContractsAutoComplete(s, i)
		},
		slashTeamworkEval: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleAllContractsAutoComplete(s, i)
		},
		slashContractReport: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleAllContractsAutoComplete(s, i)
		},
		slashScoreExplorer: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleAllContractsAutoComplete(s, i)
		},
		slashHunt: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			menno.HandleHuntAutoComplete(s, i)
		},
		slashTokenEditTrack: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			data := i.ApplicationCommandData()
			for _, opt := range data.Options {
				if opt.Name == "list" && opt.Focused {
					if i.GuildID == "" {
						str, choices := track.HandleTokenListAutoComplete(s, i)
						_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
							Type: discordgo.InteractionApplicationCommandAutocompleteResult,
							Data: &discordgo.InteractionResponseData{
								Content: str,
								Choices: choices,
							}})
					}
				}
				if opt.Name == "id" && opt.Focused {
					if i.GuildID == "" {
						str, choices := track.HandleTokenIDAutoComplete(s, i)
						_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
							Type: discordgo.InteractionApplicationCommandAutocompleteResult,
							Data: &discordgo.InteractionResponseData{
								Content: str,
								Choices: choices,
							}})
					}
				}
			}
		},
		slashTokenEdit: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			data := i.ApplicationCommandData()
			for _, opt := range data.Options {
				if opt.Name == "list" && opt.Focused {
					if i.GuildID == "" {
						str, choices := track.HandleTokenListAutoComplete(s, i)
						_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
							Type: discordgo.InteractionApplicationCommandAutocompleteResult,
							Data: &discordgo.InteractionResponseData{
								Content: str,
								Choices: choices,
							}})
					} else {
						str, choices := boost.HandleTokenListAutoComplete(s, i)
						_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
							Type: discordgo.InteractionApplicationCommandAutocompleteResult,
							Data: &discordgo.InteractionResponseData{
								Content: str,
								Choices: choices,
							}})
					}
				}
				if opt.Name == "id" && opt.Focused {
					if i.GuildID == "" {
						str, choices := track.HandleTokenIDAutoComplete(s, i)
						_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
							Type: discordgo.InteractionApplicationCommandAutocompleteResult,
							Data: &discordgo.InteractionResponseData{
								Content: str,
								Choices: choices,
							}})
					} else {
						str, choices := boost.HandleTokenIDAutoComplete(s, i)
						err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
							Type: discordgo.InteractionApplicationCommandAutocompleteResult,
							Data: &discordgo.InteractionResponseData{
								Content: str,
								Choices: choices,
							}})
						if err != nil {
							log.Println(err.Error())
						}
					}
				}
				if opt.Name == "new-receiver" && opt.Focused {
					if i.GuildID != "" {
						str, choices := boost.HandleTokenReceiverAutoComplete(s, i)
						err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
							Type: discordgo.InteractionApplicationCommandAutocompleteResult,
							Data: &discordgo.InteractionResponseData{
								Content: str,
								Choices: choices,
							}})
						if err != nil {
							log.Println(err.Error())
						}

					}
				}
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
		slashAdminGetContractData: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleAdminGetContractData(s, i)
		},
		slashAdminListRoles: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleAdminListRoles(s, i)
		},
		// Slash Commands
		slashArtifact: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleArtifactCommand(s, i)
		},
		slashScoreExplorer: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleScoreExplorerCommand(s, i)
		},
		// Normal Commands
		slashJoin: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleJoinCommand(s, i)
		},
		slashCoopETA: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleCoopETACommand(s, i)
		},
		slashLaunchHelper: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			events.HandleLaunchHelper(s, i)
		},
		slashEventHelper: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			events.HandleEventHelper(s, i)
		},
		slashRegister: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleRegister(s, i)
		},
		slashContractReport: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleContractReport(s, i)
		},
		slashRerunEval: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleReplayEval(s, i)
		},
		slashVirtue: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleVirtue(s, i)
		},
		slashToken: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleTokenCommand(s, i)
		},
		slashTokenEditTrack: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			var str string
			if i.GuildID == "" {
				str = track.HandleTokenEditTrackCommand(s, i)
			}
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: str,
					Flags:   discordgo.MessageFlagsEphemeral,
				}})
		},
		slashTokenEdit: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			var str string
			if i.GuildID != "" {
				str = boost.HandleTokenEditCommand(s, i)
			}
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: str,
					Flags:   discordgo.MessageFlagsEphemeral,
				}})
		},
		slashCalcContractTval: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleContractCalcContractTvalCommand(s, i)
		},
		slashCoopTval: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleCoopTvalCommand(s, i)
		},
		slashTeamworkEval: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleTeamworkEvalCommand(s, i)
		},
		slashStones: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleStonesCommand(s, i)
		},
		slashTimer: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleTimerCommand(s, i)
		},
		slashHunt: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			menno.HandleHuntCommand(s, i)
		},
		slashPredictions: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandlePredictionsCommand(s, i)
		},
		slashEstimateTime: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleEstimateTimeCommand(s, i)
		},
		slashCsEstimate: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleCsEstimatesCommand(s, i)
		},
		slashSpeedrun: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleSpeedrunCommand(s, i)
		},
		slashChangeSpeedRunSink: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleChangeSpeedrunSinkCommand(s, i)
		},
		slashUpdateCommand: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleUpdateCommand(s, i)
		},
		slashChangeCommand: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleChangeCommand(s, i)
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
		slashToggleContractPings: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleToggleContractPingsCommand(s, i)
		},
		slashContractSettings: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleContractSettingsCommand(s, i)
		},
		slashChangeOneBooster: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleChangeOneBoosterCommand(s, i)
		},
		slashChangePlannedStartCommand: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleChangePlannedStartCommand(s, i)
		},
		slashLinkAlternate: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleLinkAlternateCommand(s, i)
		},
		slashRenameThread: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleRenameThreadCommand(s, i)
		},
		slashRemoveDMMessage: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			bottools.HandleRemoveMessageCommand(s, i)
		},
		slashHelp: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleHelpCommand(s, i)
		},
		slashFun: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			notok.FunHandler(s, i)
		},
		slashPrivacy: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			farmerstate.HandlePrivacyCommand(s, i)
		},
		slashSetEggIncName: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			// Protection against DM use
			if i.GuildID == "" {
				_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content:    "This command can only be run in a server.",
						Flags:      discordgo.MessageFlagsEphemeral,
						Components: []discordgo.MessageComponent{}},
				})
				return
			}
			var eiName string
			var callerUserID = getIntentUserID(i)
			var userID = getIntentUserID(i)

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
			} else {
				// Is the user issuing the command a coordinator?
				if userID != callerUserID && !boost.IsUserCreatorOfAnyContract(s, callerUserID) {
					str = "This form of usage is restricted to contract coordinators and administrators."
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
			boost.AddBoostTokensInteraction(s, i, 5, 0)
		},
		"fd_tokens6": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.AddBoostTokensInteraction(s, i, 6, 0)
		},
		"fd_tokens8": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.AddBoostTokensInteraction(s, i, 8, 0)
		},
		"fd_tokens1": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.AddBoostTokensInteraction(s, i, 0, 1)
		},
		"fd_tokens_sub": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.AddBoostTokensInteraction(s, i, 0, -1)
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
		"fd_tokenRefresh": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			track.HandleTrackerRefresh(s, i)
		},
		"rc_": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleContractReactions(s, i)
		},
		"menu": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleMenuReactions(s, i)
		},
		"cs_": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleContractSettingsReactions(s, i)
		},
		"as_": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleArtifactReactions(s, i)
		},
		"fd_stones": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleStonesPage(s, i)
		},
		"fd_teamwork": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleTeamworkPage(s, i)
		},
		"fd_playground": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleScoreExplorerPage(s, i)
		},
		"fd_signups": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleSignupsPage(s, i)
		},
		"fd_signupStart": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredMessageUpdate,
				Data: &discordgo.InteractionResponseData{
					Content:    "",
					Flags:      discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{}},
			})
			err := boost.StartContractBoosting(s, i.GuildID, i.ChannelID, getIntentUserID(i))
			if err != nil {
				str := fmt.Sprint(err.Error())
				_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
					Content: str,
					Flags:   discordgo.MessageFlagsEphemeral,
				})
			} else {
				_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{})

				contract := boost.FindContract(i.ChannelID)
				// Rebuild the signup message to disable the start button
				msg := discordgo.NewMessageEdit(i.ChannelID, i.Message.ID)
				contentStr, comp := boost.GetSignupComponents(contract) // True to get a disabled start button
				msg.SetContent(contentStr)
				msg.Components = &comp
				_, _ = s.ChannelMessageEditComplex(msg)
			}
		},
		"fd_signupFarmer": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			joinContract(s, i, false)
		},
		"fd_signupBell": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			joinContract(s, i, true)
		},
		"m_eggid": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleEggIDModalSubmit(s, i)
		},
		"fd_signupLeave": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			str := "Removed from Contract"
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Processing...",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})

			var err = boost.RemoveFarmerByMention(s, i.GuildID, i.ChannelID, i.Member.User.Mention(), i.Member.User.Mention())
			if err != nil {
				str = err.Error()
			}

			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: str,
				Flags:   discordgo.MessageFlagsEphemeral,
			})

		},
		"csestimate": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleCsEstimateButtons(s, i)
		},
	}
)

func joinContract(s *discordgo.Session, i *discordgo.InteractionCreate, bell bool) {
	var str = "Adding to Contract..."

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
		Data: &discordgo.InteractionResponseData{
			Content:    str,
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})

	userID := getIntentUserID(i)

	err := boost.JoinContract(s, i.GuildID, i.ChannelID, userID, bell)
	if err != nil {
		str = err.Error()
		log.Print(str)
	}
	//else {
	//	str = fmt.Sprintf("Added <@%s> to contract", i.Member.User.ID)
	//}

	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
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
				userID := getIntentUserID(i)
				log.Println("Component: ", i.ModalSubmitData().CustomID, userID)
				h(s, i)
			}
		case discordgo.InteractionMessageComponent:
			// Handlers could include a parameter to help identify this uniquly
			handlerID := strings.Split(i.MessageComponentData().CustomID, "#")[0]

			if h, ok := componentHandlers[handlerID]; ok {
				userID := getIntentUserID(i)
				log.Println("Component: ", i.MessageComponentData().CustomID, userID)
				h(s, i)
			}
		}
	})

	// Components are part of interactions, so we register InteractionCreate handler
	s.AddHandler(func(s *discordgo.Session, m *discordgo.MessageReactionAdd) {
		if m.UserID != s.State.User.ID {
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
		if m.UserID != s.State.User.ID {
			go boost.ReactionRemove(s, m.MessageReaction)
		}
	})
}

func syncCommands(s *discordgo.Session, guildID string, desiredCommandList []*discordgo.ApplicationCommand) {
	existingCommands, err := s.ApplicationCommands(s.State.User.ID, guildID)
	if err != nil {
		log.Fatalf("Failed to fetch commands for guild %s: %v", guildID, err)
		return
	}
	bottools.UpdateCommandMap(existingCommands)

	desiredMap := make(map[string]*discordgo.ApplicationCommand)
	for _, cmd := range desiredCommandList {
		desiredMap[cmd.Name] = cmd
	}

	existingMap := make(map[string]*discordgo.ApplicationCommand)
	for _, cmd := range existingCommands {
		existingMap[cmd.Name] = cmd
	}

	// Delete commands not in the desired list
	for _, cmd := range existingCommands {
		if _, found := desiredMap[cmd.Name]; !found {
			err := s.ApplicationCommandDelete(s.State.User.ID, guildID, cmd.ID)
			if err != nil {
				log.Printf("Failed to delete command %s (%s) in guild %s: %v", cmd.Name, cmd.ID, guildID, err)
			} else {
				log.Printf("Successfully deleted command %s (%s) in guild %s", cmd.Name, cmd.ID, guildID)
			}
		}
	}

	cmds, err := s.ApplicationCommandBulkOverwrite(s.State.User.ID, guildID, desiredCommandList)
	if err != nil {
		log.Fatalf("Failed to bulk overwrite commands for guild %s: %v", guildID, err)
	} else {
		bottools.UpdateCommandMap(cmds)
	}

}

func main() {

	/*
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	*/
	// Init Mongodb
	//db.Open()
	//defer db.Close()
	//bottools.GenerateBanner("EDIBLE", "Race Fuel")
	// Load the config file

	// Start our CRON job to grab Egg Inc contract data from the Carpet github repository
	startHeartbeat("/tmp/tokentimeboost.heartbeat", 1*time.Minute)
	go tasks.ExecuteCronJob(s)

	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Ready message for: %v#%v  SessID:%v", s.State.User.Username, s.State.User.Discriminator, r.SessionID)
		//log.Printf("Ready Vers:%v  SessId:%v", r.Version, r.SessionID)
	})
	err := s.Open()
	if err != nil {
		log.Fatalf("Cannot open the session: %v", err)
	}

	bottools.LoadEmotes(s, false)
	boost.LaunchIndependentTimers(s)
	go menno.Startup()

	_ = s.UpdateStatusComplex(discordgo.UpdateStatusData{
		AFK: false,
		Activities: []*discordgo.Activity{
			{
				Name: fmt.Sprintf("Starting: %s", Version),
				Type: discordgo.ActivityTypeGame,
			},
		},
		Status: string(discordgo.StatusOnline),
	})

	if !slices.Contains(config.FeatureFlags, "NO_FUN") {
		commands = append(commands, notok.SlashFunCommand(slashFun))
	}

	commandSet := append(commands, globalCommands...)

	commandSet = append(commandSet, adminCommands...)

	syncCommands(s, config.DiscordGuildID, commandSet)

	defer func() {
		if err := s.Close(); err != nil {
			// Handle the error appropriately, e.g., logging or taking corrective actions
			log.Printf("Failed to close: %v", err)
		}
	}()

	// Add a config file watcher to pick up changes to the config file
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		if err := watcher.Close(); err != nil {
			// Handle the error appropriately, e.g., logging or taking corrective actions
			log.Printf("Failed to close: %v", err)
		}
	}()

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Println("event:", event)
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Rename) {
					switch event.Name {
					case configFileName:
						log.Println("modified file:", event.Name)
						_ = config.ReadConfig(event.Name)
						if event.Has(fsnotify.Rename) {
							_ = watcher.Add(event.Name)
						}
					case statusMessagesFileName:
						log.Println("modified file:", event.Name)
						ei.LoadStatusMessages(event.Name)
						if event.Has(fsnotify.Rename) {
							_ = watcher.Add(event.Name)
						}
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add(configFileName)
	if err != nil {
		log.Fatal(err)
	}

	err = watcher.Add(statusMessagesFileName)
	if err != nil {
		log.Printf("Warning: Could not watch status messages file: %v", err)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	log.Println("Press Ctrl+C to exit")

	<-stop

	boost.SaveAllData()
	track.SaveAllData()

	log.Println("Graceful shutdown")
}

func getIntentUserID(i *discordgo.InteractionCreate) string {
	if i.GuildID == "" {
		return i.User.ID
	}
	return i.Member.User.ID
}

// Heartbeat function to update the modification time of a file at regular intervals
func startHeartbeat(filepath string, interval time.Duration) {
	go func() {
		// Create the file if it doesn't exist
		if _, err := os.Stat(filepath); os.IsNotExist(err) {
			f, err := os.Create(filepath)
			if err != nil {
				log.Printf("Heartbeat error: Could not create file: %v", err)
				return
			}
			_ = f.Close()
		}

		counter := 0
		ticker := time.NewTicker(interval)
		for range ticker.C {
			currentTime := time.Now()
			// Equivalent to 'touch' - updates modification time
			err := os.Chtimes(filepath, currentTime, currentTime)
			if err != nil {
				log.Printf("Heartbeat error: %v", err)
			}
			counter++
			if counter%2 == 1 {
				// Get a random status message
				activityName, err := ei.GetRandomStatusMessage()
				if err != nil {
					log.Printf("Error getting status message: %v", err)
					activityName = "Egg, Inc."
				}

				err = s.UpdateStatusComplex(discordgo.UpdateStatusData{
					AFK: false,
					Activities: []*discordgo.Activity{
						{
							Name: activityName,
							Type: discordgo.ActivityTypeGame,
						},
					},
					Status: string(discordgo.StatusOnline),
				})
				if err != nil {
					log.Printf("Heartbeat error: %v", err)
					log.Printf("Restarting the bot")
					fmt.Printf("Restarting the bot due to error: %v", err)
					// At this point lets just exit the process and let something like systemd restart it, since the bot is likely in a bad state if we can't update the status
					boost.SaveAllData()
					track.SaveAllData()
					// Sleep 5 seconds to allow any in-flight interactions to complete and for the save functions to finish
					time.Sleep(5 * time.Second)
					os.Exit(1)
				}
			}
		}
	}()
}
