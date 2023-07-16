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
)

type EggFarmer struct {
	userId   string    // Discord User ID
	ping     bool      // True/False
	register time.Time // Time Farmer registered to boost
	start    time.Time // Time Farmer started boost turn
	end      time.Time // Time Farmer ended boost turn
}

type Booster struct {
	userId  string // Egg Farmer
	mention string // String which mentions user
}

type Contract struct {
	ID        string       // Contract Discord Channel
	Name      string       // Farmer Name
	position  int          // Starting Slot
	completed bool         // Boost Completed
	messageID string       // Message ID for the Last Boost Order message
	order     [100]Booster // Booster
}

// Bot parameters
var (
	GuildID  = flag.String("guild", "", "Test guild ID")
	BotToken = flag.String("token", "", "Bot access token")
	AppID    = flag.String("app", "", "Application ID")
)

// Mutex
var mutex sync.Mutex
var usermutex sync.Mutex

// Storage

var contractcache = cache.New(24 * time.Hour * 7)

var usermap map[string]*EggFarmer = make(map[string]*EggFarmer)

var s *discordgo.Session

var pingOff = "Pings OFF"

func init() { flag.Parse() }

func init() {
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

			m := discordgo.NewMessageEdit(i.ChannelID, i.Message.ID)

			fmt.Print(m)

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content:    pingStr,
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
			// User interacting with bot, is this first time ?

			mutex.Lock()
			//contract, cfound := contractcache.Get(i.ChannelID)
			user := usermap[i.Member.User.ID]
			if user == nil {
				user = new(EggFarmer)

				user.register = time.Now()
				user.ping = false
				user.userId = i.Member.User.ID
				// Cache doesn't exist, create one
				usermap[i.Member.User.ID] = user
				// New Channel, set some defaults
			}
			mutex.Unlock()

			var pingStr = "🔕"
			if user.ping {
				pingStr = "🔔"
			}
			fmt.Print(pingStr)

			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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
									Label:    pingStr,
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
			msg, err := s.ChannelMessageSend(i.ChannelID, "BoostList")
			if err != nil {
				panic(err)
			}
			s.ChannelMessageDelete(i.ChannelID, msg.ID)

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
	_, err := s.ApplicationCommandCreate(*AppID, *GuildID, &discordgo.ApplicationCommand{
		Name:        "boost",
		Description: "Contract Boosting Elections",
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
