package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
)

// Slash Command Constants
const slashContract string = "contract"
const slashSkip string = "skip"
const slashBoost string = "boost"
const slashStart string = "start"
const slashLast string = "last"
const slashPrune string = "prune"
const slashJoin string = "join"

// Bot parameters to override .config.json parameters
var (
	GuildID  = flag.String("guild", "", "Test guild ID")
	BotToken = flag.String("token", "", "Bot access token")
	AppID    = flag.String("app", "", "Application ID")
)

// Mutex
var mutex sync.Mutex

// Storage

//var contractcache = cache.New(24 * time.Hour * 7)

var s *discordgo.Session

// main init to call other init functions in sequence
func init() {
	initLaunchParameters()
	initDiscordBot()
}

func initLaunchParameters() {
	// Read application parameters
	flag.Parse()

	// Read values from .env file
	err := config.ReadConfig()

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

func initDiscordBot() {
	var err error

	s, err = discordgo.New("Bot " + *BotToken)
	if err != nil {
		log.Fatalf("Invalid bot parameters: %v", err)
	}

}

// Important note: call every command in order it's placed in the example.

var (
	componentsHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"fd_delete": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			// Delete coop
			var str = fmt.Sprintf("Coop %s Deleted", boost.DeleteContract(s, i.GuildID, i.ChannelID))

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content:    str,
					Flags:      discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{}},
			})
			s.ChannelMessageDelete(i.ChannelID, i.Message.ID)

		},
	}

	commandsHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		slashJoin: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			var farmerName = ""
			var str = "Joining Member"

			// User interacting with bot, is this first time ?
			options := i.ApplicationCommandData().Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			if opt, ok := optionMap["farmer"]; ok {
				farmerName = opt.StringValue()
			}

			var err = boost.AddContractMember(s, i.GuildID, i.ChannelID, i.Member.Mention(), farmerName)
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
		slashContract: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			var contractID = i.GuildID
			var coopID = i.GuildID // Default to the Guild ID
			var boostOrder = 0
			var coopSize = 2

			// User interacting with bot, is this first time ?
			options := i.ApplicationCommandData().Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			if opt, ok := optionMap["coop-size"]; ok {
				coopSize = int(opt.IntValue())
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

			contract, err := boost.CreateContract(s, contractID, coopID, coopSize, boostOrder, i.GuildID, i.ChannelID, i.Member.User.ID)
			if err != nil {
				fmt.Print("Contract already exists")
			}
			mutex.Unlock()

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
				panic(err)
			}

			msg, err := s.ChannelMessageSend(i.ChannelID, boost.DrawBoostList(s, contract))
			if err != nil {
				panic(err)
			}
			boost.SetMessageID(contract, i.ChannelID, msg.ID)

			reactionMsg, err := s.ChannelMessageSend(i.ChannelID, "`React with 🧑‍🌾 or 🔔 to signup. 🔔 will DM Updates, 🎲 is vote for random boost order, requires 2/3 supermajoriy to pass.`")
			if err != nil {
				panic(err)
			}
			boost.SetReactionID(contract, i.ChannelID, reactionMsg.ID)
			s.MessageReactionAdd(msg.ChannelID, reactionMsg.ID, "🧑‍🌾") // Booster
			s.MessageReactionAdd(msg.ChannelID, reactionMsg.ID, "🔔")   // Ping
			s.MessageReactionAdd(msg.ChannelID, reactionMsg.ID, "🎲")   // Boost Order

			s.ChannelMessagePin(msg.ChannelID, reactionMsg.ID)

		},
		slashStart: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			var str = "Sorted, Starting boosting!"
			var err = boost.StartContractBoosting(s, i.GuildID, i.ChannelID)
			if err != nil {
				str = err.Error()
			}
			fmt.Print(str)

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content:    str,
					Flags:      discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{}},
			})

		},
		slashBoost: func(s *discordgo.Session, i *discordgo.InteractionCreate) {

			var str = "Boosting!!"
			var err = boost.BoostCommand(s, i.GuildID, i.ChannelID, i.Member.User.ID)
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
		slashLast: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			var str = "Move yourself to end of boost list."
			var err = boost.SkipBooster(s, i.GuildID, i.ChannelID, i.Member.User.ID)
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

		slashPrune: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			var str = "Prune Booster"
			var farmer = ""

			options := i.ApplicationCommandData().Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			if opt, ok := optionMap["farmer"]; ok {
				farmer = opt.StringValue()
			}

			var err = boost.RemoveContractBoosterByMention(s, i.GuildID, i.ChannelID, i.Member.Mention(), farmer)
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

func main() {

	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Println("Bot is up!")
	})
	// Components are part of interactions, so we register InteractionCreate handler
	s.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		switch i.Type {
		case discordgo.InteractionApplicationCommand:
			if h, ok := commandsHandlers[i.ApplicationCommandData().Name]; ok {
				h(s, i)
			}
		case discordgo.InteractionMessageComponent:

			if h, ok := componentsHandlers[i.MessageComponentData().CustomID]; ok {
				h(s, i)
			}
		}
	})
	s.AddHandler(func(s *discordgo.Session, m *discordgo.MessageReactionAdd) {
		if m.MessageReaction.UserID != s.State.User.ID {
			boost.ReactionAdd(s, m.MessageReaction)
		}
	})
	s.AddHandler(func(s *discordgo.Session, m *discordgo.MessageReactionRemove) {
		if m.MessageReaction.UserID != s.State.User.ID {
			boost.ReactionRemove(s, m.MessageReaction)
		}
	})
	_, err := s.ApplicationCommandCreate(*AppID, *GuildID, &discordgo.ApplicationCommand{
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
		},
	})
	if err != nil {
		fmt.Println(err)
	}
	_, err = s.ApplicationCommandCreate(*AppID, *GuildID, &discordgo.ApplicationCommand{
		Name:        slashJoin,
		Description: "Contract Boosting Elections",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "farmer",
				Description: "User Mention to add to existing contract",
				Required:    true,
			},
		},
	})
	if err != nil {
		fmt.Println(err)
	}

	_, err = s.ApplicationCommandCreate(*AppID, *GuildID, &discordgo.ApplicationCommand{
		Name:        slashStart,
		Description: "Start Contract Boost",
		Options:     []*discordgo.ApplicationCommandOption{},
	})
	if err != nil {
		fmt.Println(err)
	}

	_, err = s.ApplicationCommandCreate(*AppID, *GuildID, &discordgo.ApplicationCommand{
		Name:        slashBoost,
		Description: "Spending tokens to boost!",
		Options:     []*discordgo.ApplicationCommandOption{},
	})
	if err != nil {
		log.Fatalf("Cannot create slash command: %v", err)
	}

	_, err = s.ApplicationCommandCreate(*AppID, *GuildID, &discordgo.ApplicationCommand{
		Name:        slashSkip,
		Description: "Move current booster to last in boost order.",
		Options:     []*discordgo.ApplicationCommandOption{},
	})
	if err != nil {
		log.Fatalf("Cannot create slash command: %v", err)
	}

	_, err = s.ApplicationCommandCreate(*AppID, *GuildID, &discordgo.ApplicationCommand{
		Name:        slashLast,
		Description: "Move yourself to last in boost order.",
		Options:     []*discordgo.ApplicationCommandOption{},
	})
	if err != nil {
		log.Fatalf("Cannot create slash command: %v", err)
	}

	_, err = s.ApplicationCommandCreate(*AppID, *GuildID, &discordgo.ApplicationCommand{
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
	})
	if err != nil {
		log.Fatalf("Cannot create slash command: %v", err)
	}

	err = s.Open()
	if err != nil {
		log.Fatalf("Cannot open the session: %v", err)
	}
	defer s.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	log.Println("Graceful shutdown")
}
