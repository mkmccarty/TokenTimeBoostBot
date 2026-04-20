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
	_ "time/tzdata"

	"github.com/bwmarrin/discordgo"
	"github.com/fsnotify/fsnotify"
	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/events"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/menno"
	"github.com/mkmccarty/TokenTimeBoostBot/src/notok"
	"github.com/mkmccarty/TokenTimeBoostBot/src/tasks"
	"github.com/mkmccarty/TokenTimeBoostBot/src/version"
	"github.com/natefinch/lumberjack/v3"
)

const (
	configFileName         = ".config.json"
	statusMessagesFileName = "ttbb-data/status-messages.json"
)

const (
	unknownCommandPathMessage   = "Unknown command path. Please run the command again, or use /help if this persists."
	unknownModalPathMessage     = "Unknown modal action. Please rerun the related command to open a fresh dialog."
	unknownComponentPathMessage = "Unknown interaction action. Use the latest message buttons/menus, or rerun the command."
)

// Admin Slash Command Constants
const slashAdminContractsList string = "admin-contract-list"

// const slashAdminBotSettings string = "admin-bot-settings"
const slashReloadContracts string = "admin-reload-contracts"
const slashAdminGetContractData string = "admin-get-contract-data"
const slashAdminListRoles string = "list-roles"
const slashAdminSetGuildSetting string = "admin-set-guild-setting"
const slashAdminGetGuildSettings string = "admin-get-guild-settings"
const slashAdminSetGuildFlag string = "admin-set-guild-flag"
const slashAdminGetGuildFlag string = "admin-get-guild-flag"
const slashAdminGuildstate string = "admin-guildstate"
const slashAdminMembers string = "admin-members"
const slashActiveContracts string = "active-contracts"
const slashStatusMessage string = "status-message"

// Slash Command Constants
const slashContract string = "contract"
const slashSkip string = "skip"
const slashBoost string = "boost"
const slashBoostOrder string = "boost-order"
const slashCatalyst string = "catalyst"
const slashChangeOneBooster string = "change-one-booster"
const slashChangePlannedStartCommand string = "change-start"
const slashChangeSpeedRunSink string = "change-speedrun-sink"
const slashChangeCommand string = "change"
const slashUpdateCommand string = "update"
const slashUnboost string = "unboost"
const slashPrune string = "prune"
const slashJoinContract string = "join-contract"
const slashSetEggIncName string = "seteggincname"
const slashBump string = "bump"
const slashToggleContractPings string = "toggle-contract-pings"
const slashContractSettings string = "contract-settings"
const slashHelp string = "help"
const slashSpeedrun string = "speedrun"
const slashCoopETA string = "coopeta"
const slashLaunchHelper string = "launch-helper"
const slashEventHelper string = "events"

// const slashTokenRemove string = "token-remove"
const slashTokenEdit string = "token-edit"
const slashCalcContractTval string = "calc-contract-tval"
const slashCoopTval string = "coop-tval"
const slashVolunteerSink string = "volunteer-sink"
const slashVoluntellSink string = "voluntell-sink"
const slashLinkAlternate string = "link-alternate"
const slashTeamworkEval string = "teamwork"
const slashEstimateTime string = "estimate-contract-time"
const slashCsEstimate string = "cs-estimate"
const slashLobby string = "lobby"
const slashRenameThread string = "rename-thread"
const slashFun string = "fun"
const slashStones string = "stones"
const slashLeaderboard string = "leaderboard"
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
const slashMint string = "mint"

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

	// Wire the coop status fix check to avoid import cycle in ei package
	ei.CoopStatusFixEnabled = func() bool {
		return guildstate.GetGuildSettingString("DEFAULT", "coop_status_fix") == "1"
	}

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
	s.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsGuildMessageReactions |
		discordgo.IntentsDirectMessageReactions
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
	GuildID        = flag.String("guild", "", "Test guild ID")
	BotToken       = flag.String("token", "", "Bot access token")
	AppID          = flag.String("app", "", "Application ID")
	RemoveCommands = flag.Bool("rmcmd", false, "Remove all commands after shutdowning or not")

	adminCommands = []*discordgo.ApplicationCommand{
		boost.GetSlashAdminContractsListCommand(slashAdminContractsList),
		tasks.GetSlashReloadContractsCommand(slashReloadContracts),
		boost.SlashAdminGetContractData(slashAdminGetContractData),
		boost.SlashAdminListRoles(slashAdminListRoles),
		boost.SlashAdminGuildStateCommand(slashAdminGuildstate),
		boost.SlashAdminMembers(slashAdminMembers),
		boost.SlashAdminCurrentContracts(slashActiveContracts),
		boost.SlashAdminStatusMessageCommand(slashStatusMessage),
		guildstate.SlashSetGuildSettingCommand(slashAdminSetGuildSetting),
		guildstate.SlashGetGuildSettingsCommand(slashAdminGetGuildSettings),
		guildstate.SlashSetGuildFlagCommand(slashAdminSetGuildFlag),
		guildstate.SlashGetGuildFlagCommand(slashAdminGetGuildFlag),
	}

	globalCommands = []*discordgo.ApplicationCommand{
		events.SlashLaunchHelperCommand(slashLaunchHelper),
		events.SlashEventHelperCommand(slashEventHelper),
		boost.GetSlashRerunEvalCommand(slashRerunEval),
		boost.GetSlashVirtueCommand(slashVirtue),
		boost.GetSlashRegisterCommand(slashRegister),
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
		boost.GetSlashJoinContractCommand(slashJoinContract),
		boost.GetSlashBoostCommand(slashBoost),
		boost.GetSlashBoostOrderCommand(slashBoostOrder),
		boost.GetSlashBoostOrderCommand(slashCatalyst),
		boost.GetSlashSkipCommand(slashSkip),
		boost.GetSlashUnboostCommand(slashUnboost),
		boost.GetSlashPruneCommand(slashPrune),
		boost.GetSlashCoopETACommand(slashCoopETA),
		boost.GetSlashVolunteerSink(slashVolunteerSink),
		boost.GetSlashVoluntellSink(slashVoluntellSink),
		boost.GetSlasLinkAlternateCommand(slashLinkAlternate),
		boost.GetSlashCalcContractTval(slashCalcContractTval),
		boost.GetSlashCoopTval(slashCoopTval),
		boost.GetSlashTeamworkEval(slashTeamworkEval),
		boost.GetSlashContractReportCommand(slashContractReport),
		boost.GetPredictionsCommand(slashPredictions),
		boost.GetSlashStones(slashStones),
		boost.GetSlashLeaderboard(slashLeaderboard),
		boost.GetSlashTimer(slashTimer),
		boost.GetSlashEstimateTime(slashEstimateTime),
		boost.GetSlashCsEstimates(slashCsEstimate),
		boost.GetSlashLobbyCommand(slashLobby),
		boost.GetSlashChangeOneBoosterCommand(slashChangeOneBooster),
		boost.GetSlashChangePlannedStartCommand(slashChangePlannedStartCommand),
		bottools.GetSlashRemoveMessage(slashRemoveDMMessage),
		boost.GetSlashTokenEditCommand(slashTokenEdit),
		farmerstate.SlashSetEggIncNameCommand(slashSetEggIncName),
		farmerstate.GetSlashPrivacyCommand(slashPrivacy),
		boost.GetSlashBumpCommand(slashBump),
		boost.GetSlashToggleContractPingsCommand(slashToggleContractPings),
		boost.GetSlashHelpCommand(slashHelp),
		boost.GetSlashContractSettingsCommand(slashContractSettings),
	}

	autocompleteHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		slashContract:             boost.HandleContractAutoComplete,
		slashChangeCommand:        boost.HandleContractAutoComplete,
		slashRerunEval:            boost.HandleAllContractsAutoComplete,
		slashAdminGetContractData: boost.HandleCoopAutoComplete,
		slashAdminListRoles:       boost.HandleContractAutoComplete,
		slashAdminGuildstate:      boost.HandleAdminGuildStateAutoComplete,
		slashStatusMessage:        boost.HandleAdminStatusMessageAutoComplete,
		slashEstimateTime:         boost.HandleAllContractsAutoComplete,
		slashCsEstimate:           boost.HandleAllContractsAutoComplete,
		slashLobby:                boost.HandleAllContractsAutoComplete,
		slashLinkAlternate:        boost.HandleLinkAlternateAutoComplete,
		slashCalcContractTval:     boost.HandleAltsAutoComplete,
		slashStones:               boost.HandleAllContractsAutoComplete,
		slashTeamworkEval:         boost.HandleAllContractsAutoComplete,
		slashContractReport:       boost.HandleAllContractsAutoComplete,
		slashScoreExplorer:        boost.HandleAllContractsAutoComplete,
		slashHunt:                 menno.HandleHuntAutoComplete,
		slashTokenEdit:            boost.HandleTokenEditAutoComplete,
	}

	commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		slashAdminContractsList:        boost.HandleAdminContractList,
		slashReloadContracts:           tasks.HandleReloadContractsCommand,
		slashAdminGetContractData:      boost.HandleAdminGetContractData,
		slashAdminListRoles:            boost.HandleAdminListRoles,
		slashAdminGuildstate:           boost.HandleAdminGuildStateCommand,
		slashAdminMembers:              boost.HandleAdminMembers,
		slashActiveContracts:           boost.HandleAdminCurrentContracts,
		slashStatusMessage:             boost.HandleAdminStatusMessageCommand,
		slashAdminSetGuildSetting:      guildstate.SetGuildSetting,
		slashAdminGetGuildSettings:     guildstate.GetGuildSettings,
		slashAdminSetGuildFlag:         guildstate.SetGuildFlag,
		slashAdminGetGuildFlag:         guildstate.GetGuildFlag,
		slashArtifact:                  boost.HandleArtifactCommand,
		slashScoreExplorer:             boost.HandleScoreExplorerCommand,
		slashJoinContract:              boost.HandleJoinCommand,
		slashCoopETA:                   boost.HandleCoopETACommand,
		slashLaunchHelper:              events.HandleLaunchHelper,
		slashEventHelper:               events.HandleEventHelper,
		slashRegister:                  boost.HandleRegister,
		slashContractReport:            boost.HandleContractReport,
		slashRerunEval:                 boost.HandleReplayEval,
		slashVirtue:                    boost.HandleVirtue,
		slashTokenEdit:                 handleTokenEditCommand,
		slashCalcContractTval:          boost.HandleContractCalcContractTvalCommand,
		slashCoopTval:                  boost.HandleCoopTvalCommand,
		slashTeamworkEval:              boost.HandleTeamworkEvalCommand,
		slashStones:                    boost.HandleStonesCommand,
		slashLeaderboard:               boost.HandleLeaderboard,
		slashTimer:                     boost.HandleTimerCommand,
		slashHunt:                      menno.HandleHuntCommand,
		slashPredictions:               boost.HandlePredictionsCommand,
		slashMint:                      boost.HandleMintCommand,
		slashEstimateTime:              boost.HandleEstimateTimeCommand,
		slashCsEstimate:                boost.HandleCsEstimatesCommand,
		slashLobby:                     boost.HandleLobbyCommand,
		slashSpeedrun:                  boost.HandleSpeedrunCommand,
		slashChangeSpeedRunSink:        boost.HandleChangeSpeedrunSinkCommand,
		slashUpdateCommand:             boost.HandleUpdateCommand,
		slashChangeCommand:             boost.HandleChangeCommand,
		slashVolunteerSink:             boost.HandleSlashVolunteerSinkCommand,
		slashVoluntellSink:             boost.HandleSlashVoluntellSinkCommand,
		slashContract:                  boost.HandleContractCommand,
		slashBoost:                     boost.HandleBoostCommand,
		slashBoostOrder:                boost.HandleBoostOrderCommand,
		slashCatalyst:                  boost.HandleBoostOrderCommand,
		slashSkip:                      boost.HandleSkipCommand,
		slashUnboost:                   boost.HandleUnboostCommand,
		slashPrune:                     boost.HandlePruneCommand,
		slashBump:                      boost.HandleBumpCommand,
		slashToggleContractPings:       boost.HandleToggleContractPingsCommand,
		slashContractSettings:          boost.HandleContractSettingsCommand,
		slashChangeOneBooster:          boost.HandleChangeOneBoosterCommand,
		slashChangePlannedStartCommand: boost.HandleChangePlannedStartCommand,
		slashLinkAlternate:             boost.HandleLinkAlternateCommand,
		slashRenameThread:              boost.HandleRenameThreadCommand,
		slashRemoveDMMessage:           bottools.HandleRemoveMessageCommand,
		slashHelp:                      boost.HandleHelpCommand,
		slashFun:                       notok.FunHandler,
		slashPrivacy:                   farmerstate.HandlePrivacyCommand,
		slashSetEggIncName:             handleSetEggIncName,
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
		"bo_order": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleBoostOrderReactions(s, i)
		},
		"predictions": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandlePredictionsPage(s, i)
		},
		"leaderboard": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleLeaderboardPage(s, i)
		},
		"active-contracts": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleActiveContractsPage(s, i)
		},
		"admin-contract-list": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleAdminContractListComponent(s, i)
		},
		"fd_signupStart":  handleSignupStart,
		"fd_signupFarmer": handleSignupFarmer,
		"fd_signupBell":   handleSignupBell,
		"m_eggid": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleEggIDModalSubmit(s, i)
		},
		"fd_signupLeave": handleSignupLeave,
		"csestimate": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleCsEstimateButtons(s, i)
		},
		"lobby": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleLobbyButtons(s, i)
		},
		"coop_status": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleCoopStatusPermissionButton(s, i)
		},
		"leaderboard_perm": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleLeaderboardPermissionButton(s, i)
		},
		"mint_preview": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleMintPreviewComponent(s, i)
		},
		"chart": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			boost.HandleChartReactions(s, i)
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

func handleTokenEditCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
}

func handleSignupStart(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
}

func handleSignupFarmer(s *discordgo.Session, i *discordgo.InteractionCreate) {
	joinContract(s, i, false)
}

func handleSignupBell(s *discordgo.Session, i *discordgo.InteractionCreate) {
	joinContract(s, i, true)
}

func handleSignupLeave(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
}

func handleSetEggIncName(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
}

func respondUnknownInteractionPath(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func respondUnknownAutocompletePath(s *discordgo.Session, i *discordgo.InteractionCreate) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: []*discordgo.ApplicationCommandOptionChoice{},
		},
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
			} else {
				log.Printf("Unknown command handler: %s", i.ApplicationCommandData().Name)
				respondUnknownInteractionPath(s, i, unknownCommandPathMessage)
			}
		case discordgo.InteractionApplicationCommandAutocomplete:
			if h, ok := autocompleteHandlers[i.ApplicationCommandData().Name]; ok {
				h(s, i)
			} else {
				log.Printf("Unknown autocomplete handler: %s", i.ApplicationCommandData().Name)
				respondUnknownAutocompletePath(s, i)
			}

		case discordgo.InteractionModalSubmit:
			// Handlers could include a parameter to help identify this uniquly
			handlerID := strings.Split(i.ModalSubmitData().CustomID, "#")[0]
			if h, ok := componentHandlers[handlerID]; ok {
				userID := getIntentUserID(i)
				log.Println("Component: ", i.ModalSubmitData().CustomID, userID)
				h(s, i)
			} else {
				log.Printf("Unknown modal handler: %s", i.ModalSubmitData().CustomID)
				respondUnknownInteractionPath(s, i, unknownModalPathMessage)
			}
		case discordgo.InteractionMessageComponent:
			// Handlers could include a parameter to help identify this uniquly
			handlerID := strings.Split(i.MessageComponentData().CustomID, "#")[0]

			if h, ok := componentHandlers[handlerID]; ok {
				userID := getIntentUserID(i)
				log.Println("Component: ", i.MessageComponentData().CustomID, userID)
				h(s, i)
			} else {
				log.Printf("Unknown component handler: %s", i.MessageComponentData().CustomID)
				respondUnknownInteractionPath(s, i, unknownComponentPathMessage)
			}
		}
	})

	// Components are part of interactions, so we register InteractionCreate handler
	s.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		boost.HandleMintCSVUploadMessage(s, m)
	})

	s.AddHandler(func(s *discordgo.Session, m *discordgo.MessageReactionAdd) {
		if m.UserID != s.State.User.ID {
			if m.GuildID != "" {
				boost.ReactionAdd(s, m.MessageReaction)
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

func connectWithRetry(dg *discordgo.Session) error {
	var err error
	backoff := time.Second * 2

	for i := 0; i < 5; i++ {
		err = dg.Open()
		if err == nil {
			return nil
		}

		log.Printf("Conn failed: %v. Retrying in %v...", err, backoff)
		time.Sleep(backoff)
		backoff *= 2 // Exponentially increase wait time
	}
	return fmt.Errorf("could not connect after multiple attempts: %w", err)
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

	err := connectWithRetry(s)
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

	//if config.IsDevBot() {
	commands = append(commands, boost.GetSlashMintCommand(slashMint))
	//}

	commandSet := append(commands, globalCommands...)

	commandSet = append(commandSet, adminCommands...)

	// Restrict some commands to a specific guild if the home_guild setting is set, to allow for faster iteration during development without affecting the global command set that everyone uses
	homeGuild := guildstate.GetGuildSettingString("DEFAULT", "home_guild")
	if homeGuild == "" {
		homeGuild = "DISABLED"
	}

	var homeGuildCommandSet []*discordgo.ApplicationCommand
	var filteredCommandSet []*discordgo.ApplicationCommand
	for _, cmd := range commandSet {
		if homeGuild != "" && cmd.GuildID == homeGuild {
			homeGuildCommandSet = append(homeGuildCommandSet, cmd)
		} else {
			filteredCommandSet = append(filteredCommandSet, cmd)
		}
	}
	commandSet = filteredCommandSet

	if homeGuild != "DISABLED" {
		syncCommands(s, homeGuild, homeGuildCommandSet)
	}
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
					// At this point, trigger a graceful shutdown so deferred cleanups (including s.Close()) run.
					boost.SaveAllData()
					// Sleep 5 seconds to allow any in-flight interactions to complete and for the save functions to finish
					time.Sleep(5 * time.Second)
					// Signal the current process to initiate the normal shutdown path instead of calling os.Exit directly.
					if serr := syscall.Kill(os.Getpid(), syscall.SIGTERM); serr != nil {
						log.Printf("Heartbeat error: could not signal shutdown (forcing exit): %v", serr)
						os.Exit(1)
					}
					return
				}
			}
		}
	}()
}
