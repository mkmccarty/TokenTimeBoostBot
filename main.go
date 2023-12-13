package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/notok"
	"github.com/xhit/go-str2duration/v2"
)

// Slash Command Constants
const slashContract string = "contract"
const slashSkip string = "skip"
const slashBoost string = "boost"

// const slashcluck string = "cluck"
const slashLast string = "last"
const slashPrune string = "prune"
const slashJoin string = "join"

// const slashSignup string = "signup"
const slashCoopETA string = "coopeta"

const slashGPT string = "fun"

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
			var str = "Contract not found."
			var coopName = boost.DeleteContract(s, i.GuildID, i.ChannelID)
			if coopName != "" {
				str = fmt.Sprintf("Contract %s Deleted", coopName)
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
	}

	commandsHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		slashJoin: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			var farmerName *discordgo.User = nil
			var guestName = ""
			var str = "Joining Member"
			var mention = ""

			// User interacting with bot, is this first time ?
			options := i.ApplicationCommandData().Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			if opt, ok := optionMap["farmer"]; ok {
				farmerName = opt.UserValue(s)
				mention = farmerName.Mention()
			}
			if opt, ok := optionMap["guest"]; ok {
				guestName = opt.StringValue()
			}

			var err = boost.AddContractMember(s, i.GuildID, i.ChannelID, i.Member.Mention(), mention, guestName)
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
		slashCoopETA: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
		/*
			slashSignup: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
				var contractID = i.GuildID
				var coopID = i.GuildID // Default to the Guild ID
				var number = 0
				var coopSize = 0
				var threshold = 0
				var threadChannel *discordgo.Channel = nil

				// User interacting with bot, is this first time ?
				options := i.ApplicationCommandData().Options
				optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
				for _, opt := range options {
					optionMap[opt.Name] = opt
				}
				if opt, ok := optionMap["number"]; ok {
					number = int(opt.IntValue())
				}
				if opt, ok := optionMap["contract-id"]; ok {
					contractID = opt.StringValue()
					contractID = strings.Replace(contractID, " ", "", -1)
				}
				if opt, ok := optionMap["coop-id"]; ok {
					coopID = opt.StringValue()
					coopID = strings.Replace(coopID, " ", "", -1)
				}
				if opt, ok := optionMap["coop-size"]; ok {
					coopSize = int(opt.IntValue())
				}
				if opt, ok := optionMap["threshold"]; ok {
					threshold = int(opt.IntValue())
				}
				if opt, ok := optionMap["thread-channel"]; ok {
					threadChannel = opt.ChannelValue(s)
				}

				boost.StartSignup(s, i, number, contractID, coopID, coopSize, threshold, threadChannel)
				var embeds []*discordgo.MessageEmbed

				embed := &discordgo.MessageEmbed{
					Author: &discordgo.MessageEmbedAuthor{
						Name: "RAIYC#0",
					},
					Type:        discordgo.EmbedTypeRich,
					Color:       0x00ff00, // Green
					Description: "Test embed",
					Fields: []*discordgo.MessageEmbedField{
						&discordgo.MessageEmbedField{
							Name:   "I am a field",
							Value:  "I am a value",
							Inline: true,
						},
						&discordgo.MessageEmbedField{
							Name:   "I am a secondfield",
							Value:  "I am a value",
							Inline: true,
						},
					},
					Timestamp: time.Now().Format(time.RFC3339),
					Title:     "I am an Embed",
				}

				embeds = append(embeds, embed)

				var actionRows []discordgo.MessageComponent
				var comp []discordgo.MessageComponent

				for i := 0; i < number; i++ {
					comp = append(comp,
						discordgo.Button{
							Label:    fmt.Sprintf("%d", i),
							Style:    discordgo.PrimaryButton,
							Disabled: false,
							CustomID: fmt.Sprintf("fd_signup%d", i),
						})
				}
				comp = append(comp,
					discordgo.Button{
						Label:    "Delete",
						Style:    discordgo.DangerButton,
						Disabled: false,
						CustomID: "fd_delete",
					})
				// Build Action Rows
				x := int(len(comp) / 5)
				for i := 0; i <= x; i++ {
					var c []discordgo.MessageComponent

					for j := 0; j < 5; j++ {
						if len(comp) == (i*5 + j) {
							break
						}
						c = append(c, comp[i*5+j])
					}
					actionRows = append(actionRows,
						discordgo.ActionsRow{
							Components: c,
						})
				}
				//embed := NewEmbed().
				err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Eggent Signup",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				if err != nil {
					fmt.Print(err.Error())
					//panic(err)
				}
				_, err = s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{
					Content:    "Test",
					Embeds:     embeds,
					Components: actionRows,
				})
				if err != nil {
					fmt.Print(err.Error())
					//panic(err)
				}

			},
		*/
		slashContract: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			var contractID = i.GuildID
			var coopID = i.GuildID // Default to the Guild ID
			var boostOrder = boost.CONTRACT_ORDER_RANDOM
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
				print(err)
			}

			msg, err := s.ChannelMessageSend(i.ChannelID, boost.DrawBoostList(s, contract))
			if err != nil {
				print(err)
			}
			boost.SetMessageID(contract, i.ChannelID, msg.ID)

			reactionMsg, err := s.ChannelMessageSend(i.ChannelID, "`React with 🧑‍🌾 or 🔔 to signup. 🔔 will DM Updates. Select :six: or :eight: to indicate your boost. Contract Creator can start the contract with ⏱️.`")
			if err != nil {
				print(err)
			}
			boost.SetReactionID(contract, i.ChannelID, reactionMsg.ID)
			s.MessageReactionAdd(msg.ChannelID, reactionMsg.ID, "🧑‍🌾")     // Booster
			s.MessageReactionAdd(msg.ChannelID, reactionMsg.ID, "🔔")       // Ping
			s.MessageReactionAdd(msg.ChannelID, reactionMsg.ID, ":six:")   // Six token
			s.MessageReactionAdd(msg.ChannelID, reactionMsg.ID, ":eight:") // Eight Token
			s.MessageReactionAdd(msg.ChannelID, reactionMsg.ID, "+")       // Additional Token
			s.MessageReactionAdd(msg.ChannelID, reactionMsg.ID, "⏱️")      // Creator Start Contract

			s.ChannelMessagePin(msg.ChannelID, reactionMsg.ID)

		},
		/*
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
		*/
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
		slashGPT: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			var gptOption = int64(0)
			var gptText = ""
			//var str = ""

			// User interacting with bot, is this first time ?
			options := i.ApplicationCommandData().Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			if opt, ok := optionMap["action"]; ok {
				gptOption = opt.IntValue()
			}
			if opt, ok := optionMap["prompt"]; ok {
				gptText = opt.StringValue()
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				//Data: &discordgo.InteractionResponseData{
				//	Content:    "",
				//	Flags:      discordgo.MessageFlagsEphemeral,
				//	Components: []discordgo.MessageComponent{}},
			},
			)

			var _ = notok.Notok(s, i, gptOption, gptText)
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
			var str = boost.ReactionAdd(s, m.MessageReaction)
			if str == "!gonow" {
				notok.DoGoNow(s, m.ChannelID)
			}
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
	///signup number contract-id coop-base coop-size threshhold #contract_threads
	/*
		_, err = s.ApplicationCommandCreate(*AppID, *GuildID, &discordgo.ApplicationCommand{
			Name:        slashSignup,
			Description: "Contract Boosting Elections",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "number",
					Description: "Number of Sign-ups",
					Required:    true,
				},
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
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "threshold",
					Description: "Spawn at threshold",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionChannel,
					Name:        "thread-channel",
					Description: "Thread Channel",
					Required:    true,
				},
			},
		})
		if err != nil {
			fmt.Println(err)
		}
	*/
	_, err = s.ApplicationCommandCreate(*AppID, *GuildID, &discordgo.ApplicationCommand{
		Name:        slashJoin,
		Description: "Add farmer or guest to contract.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "farmer",
				Description: "User Mention to add to existing contract",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "guest",
				Description: "Guest Farmer to add to existing contract",
				Required:    false,
			},
			/*
				{
					Name:        "Order",
					Description: "How farmer should be added to contract. Default is Random.",
					Required:    false,
					Type:        discordgo.ApplicationCommandOptionInteger,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "Random",
							Value: 1,
						},
						{
							Name:  "Last",
							Value: 2,
						},
					},
				},
			*/
		},
	})
	if err != nil {
		fmt.Println(err)
	}
	_, err = s.ApplicationCommandCreate(*AppID, *GuildID, &discordgo.ApplicationCommand{
		Name:        slashGPT,
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
						Name:  "Draw a fancy picture using the prompt text.",
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
	})
	if err != nil {
		fmt.Println(err)
	}
	/*
		s.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
			notok.Notok(s, m)
		})
		if err != nil {
			fmt.Println(err)
		}
	*/
	/*
		_, err = s.ApplicationCommandCreate(*AppID, *GuildID, &discordgo.ApplicationCommand{
			Name:        slashStart,
			Description: "Start Contract Boost",
			Options:     []*discordgo.ApplicationCommandOption{},
		})
		if err != nil {
			fmt.Println(err)
		}
	*/

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

	_, err = s.ApplicationCommandCreate(*AppID, *GuildID, &discordgo.ApplicationCommand{
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
	})
	if err != nil {
		fmt.Println(err)
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
