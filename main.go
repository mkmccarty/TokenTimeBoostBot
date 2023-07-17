package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/akyoto/cache"
	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
)

// Bot parameters to override .config.json parameters
var (
	GuildID  = flag.String("guild", "", "Test guild ID")
	BotToken = flag.String("token", "", "Bot access token")
	AppID    = flag.String("app", "", "Application ID")
)

// Mutex
var mutex sync.Mutex

// Storage

var contractcache = cache.New(24 * time.Hour * 7)

var s *discordgo.Session

var pingOff = "Pings OFF"

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
		"fd_init": func(s *discordgo.Session, i *discordgo.InteractionCreate) {

		},
		"fd_boost": func(s *discordgo.Session, i *discordgo.InteractionCreate) {

		},
		"fd_next": func(s *discordgo.Session, i *discordgo.InteractionCreate) {

		},
		"fd_last": func(s *discordgo.Session, i *discordgo.InteractionCreate) {

		},
		"fd_ping": func(s *discordgo.Session, i *discordgo.InteractionCreate) {

			mutex.Lock()
			//boolString := [...]string{"On", "Off"}
			//contract, cfound := contractcache.Get(i.ChannelID)
			/*
				user := usermap[i.Member.User.ID]
				if user != nil {
					user.ping = !user.ping
				}
				pingStr := fmt.Sprintf("Persoanl Ping: %t", user.ping)
				mutex.Unlock()

				if user.ping {
					u, _ := s.UserChannelCreate(i.Member.User.ID)
					_, err := s.ChannelMessageSend(u.ID, "Boost messages will be sent.")
					if err != nil {
						panic(err)
					}
				}
			*/

			m := discordgo.NewMessageEdit(i.ChannelID, i.Message.ID)

			fmt.Print(m)

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content:    "XXX",
					Flags:      discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{}},
			})

		},
		"fd_list": func(s *discordgo.Session, i *discordgo.InteractionCreate) {

		},
		"fd_spare": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Huh. I see, maybe some of these resources might help you?",
					Flags:   discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.Button{
									Emoji: discordgo.ComponentEmoji{
										Name: "📜",
									},
									Label: "Documentation",
									Style: discordgo.LinkButton,
									URL:   "https://discord.com/developers/docs/interactions/message-components#buttons",
								},
								discordgo.Button{
									Emoji: discordgo.ComponentEmoji{
										Name: "🔧",
									},
									Label: "Discord developers",
									Style: discordgo.LinkButton,
									URL:   "https://discord.gg/discord-developers",
								},
								discordgo.Button{
									Emoji: discordgo.ComponentEmoji{
										Name: "🦫",
									},
									Label: "Discord Gophers",
									Style: discordgo.LinkButton,
									URL:   "https://discord.gg/7RuRrVHyXF",
								},
							},
						},
					},
				},
			})
			if err != nil {
				panic(err)
			}
		},
		"fd_yes": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Great! If you wanna know more or just have questions, feel free to visit Discord Devs and Discord Gophers server. " +
						"But now, when you know how buttons work, let's move onto select menus (execute `/selects single`)",
					Flags: discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.Button{
									Emoji: discordgo.ComponentEmoji{
										Name: "🔧",
									},
									Label: "Discord developers",
									Style: discordgo.LinkButton,
									URL:   "https://discord.gg/discord-developers",
								},
								discordgo.Button{
									Emoji: discordgo.ComponentEmoji{
										Name: "🦫",
									},
									Label: "Discord Gophers",
									Style: discordgo.LinkButton,
									URL:   "https://discord.gg/7RuRrVHyXF",
								},
							},
						},
					},
				},
			})
			if err != nil {
				panic(err)
			}
		},
	}
	commandsHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"boost": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			var contractID = i.GuildID
			var coopID = i.GuildID // Default to the Guild ID
			var boostOrder = 1
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
			}
			if opt, ok := optionMap["coop-id"]; ok {
				coopID = opt.StringValue()
			} else {
				var c, err = s.Channel(i.ChannelID)
				if err != nil {
					coopID = c.Name
				}
			}
			if opt, ok := optionMap["boost-order"]; ok {
				boostOrder = int(opt.IntValue())
			}
			mutex.Lock()

			contract, err := boost.StartContract(contractID, coopID, coopSize, boostOrder, i.GuildID, i.ChannelID, i.Member.User.ID)
			if err != nil {
				fmt.Print("Error Calling StartContract()")
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
									// Label is what the user will see on the button.
									Label: "Boosted",
									// Style provides coloring of the button. There are not so many styles tho.
									Style: discordgo.PrimaryButton,
									// Disabled allows bot to disable some buttons for users.
									Disabled: false,
									// CustomID is a thing telling Discord which data to send when this button will be pressed.
									CustomID: "fd_init",
								},
								discordgo.Button{
									Label:    "🚀",
									Style:    discordgo.SuccessButton,
									Disabled: false,
									CustomID: "fd_boost",
								},
								discordgo.Button{
									Label:    "▶️",
									Style:    discordgo.DangerButton,
									Disabled: false,
									CustomID: "fd_next",
								},
								discordgo.Button{
									Label:    "⏭️",
									Style:    discordgo.DangerButton,
									Disabled: false,
									CustomID: "fd_last",
								},
								discordgo.Button{
									Label:    "deprecatedr",
									Style:    discordgo.SecondaryButton,
									Disabled: false,
									CustomID: "fd_ping",
								},
							},
						},
						// The message may have multiple actions rows.
						/*
							discordgo.ActionsRow{
								Components: []discordgo.MessageComponent{
									discordgo.Button{
										Label:    "Discord Developers server",
										Style:    discordgo.LinkButton,
										Disabled: false,
										URL:      "https://discord.gg/discord-developers",
									},
								},
							},
						*/
					},
				},
			})
			if err != nil {
				panic(err)
			}

			msg, err := s.ChannelMessageSend(i.ChannelID, boost.DrawBoostList(contract))
			if err != nil {
				panic(err)
			}
			boost.SetMessageID(contract, msg.ID)

			msg, err = s.ChannelMessageSend(i.ChannelID, "`React with 🚀 or 🔔 to boost. 🔔 will DM Updates`")
			if err != nil {
				panic(err)
			}
			boost.SetReactionID(contract, msg.ID)
			s.MessageReactionAdd(msg.ChannelID, msg.ID, "🚀") // Booster
			s.MessageReactionAdd(msg.ChannelID, msg.ID, "🔔") // Ping
			//s.ChannelMessagePin(msg.ChannelID, msg.ID)

			//s.ChannelMessageDelete(i.ChannelID, msg.ID)

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
		Name:        "boost",
		Description: "Contract Boosting Elections",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "coop-size",
				Description: "Co-op Size",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "boost-order",
				Description: "Boost Ordering",
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{
						Name:  "Join Order",
						Value: 0,
					},
					{
						Name:  "Random Order",
						Value: 1,
					},
					{
						Name:  "Fair",
						Value: 2,
					},
				},
				Required: false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "contract-id",
				Description: "Contract ID",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "coop-id",
				Description: "Coop ID",
				Required:    false,
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
