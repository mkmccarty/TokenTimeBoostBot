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
const slashChange string = "change"

// const slashcluck string = "cluck"
const slashUnboost string = "unboost"
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

func addBoostTokens(s *discordgo.Session, i *discordgo.InteractionCreate, value int) {
	var str = "Contract not found."
	tokenCount, _, err := boost.AddBoostTokens(s, i.GuildID, i.ChannelID, i.Member.User.ID, value, 0)
	if (err == nil) && (tokenCount >= 0) {
		nick := i.Member.Nick
		if nick == "" {
			nick = i.Member.User.Username
		}

		str = fmt.Sprintf("Tokens wanted by %s set to %d after adding %d", nick, tokenCount, value)
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    str,
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})
}

func JoinContract(s *discordgo.Session, i *discordgo.InteractionCreate, bell bool) {
	var str = "Added to Contract"
	err := boost.JoinContract(s, i.GuildID, i.ChannelID, i.Member.User.ID, bell)
	if err != nil {
		str = err.Error()
		fmt.Print(str)
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		/*
			Data: &discordgo.InteractionResponseData{
				//	Content:    str,
				//	Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},*/
	})

}

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
		"fd_tokens1": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			addBoostTokens(s, i, 1)
		},
		"fd_tokens5": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			addBoostTokens(s, i, 5)
		},
		"fd_tokens6": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			addBoostTokens(s, i, 6)
		},
		"fd_tokens8": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			addBoostTokens(s, i, 8)
		},
		"fd_tokens_sub": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			addBoostTokens(s, i, -1)
		},
		"fd_signupStart": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
			}
		},
		"fd_signupFarmer": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			JoinContract(s, i, false)
		},
		"fd_signupBell": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			JoinContract(s, i, true)
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

	commandsHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		slashJoin: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			var farmerName *discordgo.User = nil
			var guestName = ""
			var orderValue int64 = 2 // Default to Random
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
			if opt, ok := optionMap["order"]; ok {
				orderValue = opt.IntValue()
			}

			var err = boost.AddContractMember(s, i.GuildID, i.ChannelID, i.Member.Mention(), mention, guestName, orderValue)
			if err != nil {
				str = err.Error()
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content:    str,
						Flags:      discordgo.MessageFlagsEphemeral,
						Components: []discordgo.MessageComponent{}},
				})
			} else {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					/*
						Data: &discordgo.InteractionResponseData{
							Content:    str,
							Flags:      discordgo.MessageFlagsEphemeral,
							Components: []discordgo.MessageComponent{}},
					*/
				})
			}

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
			var boostOrder = boost.ContractOrderRandom
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

			var createMsg = boost.DrawBoostList(s, contract, boost.FindTokenEmoji(s, i.GuildID))
			msg, err := s.ChannelMessageSend(ChannelID, createMsg)
			if err == nil {
				boost.SetMessageID(contract, ChannelID, msg.ID)
				//var tokenEmoji = boost.FindTokenEmoji(s, i.GuildID)
				// extract from tokenEmjoi the string between :'s
				//tokenEmoji = tokenEmoji[strings.Index(tokenEmoji, ":")+1 : strings.LastIndex(tokenEmoji, ":")]

				var data discordgo.MessageSend
				data.Content = "" //`React with 🧑‍🌾 or 🔔 to signup. 🔔 will DM Updates. Contract Creator can start the contract with ⏱️.`"
				data.Components = []discordgo.MessageComponent{
					// add buttons to the action row
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.Button{
								Emoji: discordgo.ComponentEmoji{
									Name: "🧑‍🌾",
								},
								Label:    "Join",
								Style:    discordgo.PrimaryButton,
								CustomID: "fd_signupFarmer",
							},
							discordgo.Button{
								Emoji: discordgo.ComponentEmoji{
									Name: "🔔",
								},
								Label:    "Join w/Ping",
								Style:    discordgo.PrimaryButton,
								CustomID: "fd_signupBell",
							},
							discordgo.Button{
								Emoji: discordgo.ComponentEmoji{
									Name: "❌",
								},
								Label:    "Leave",
								Style:    discordgo.SecondaryButton,
								CustomID: "fd_signupLeave",
							},

							discordgo.Button{
								Emoji: discordgo.ComponentEmoji{
									Name: "⏱️",
								},
								Label:    "Start Boost List",
								Style:    discordgo.SuccessButton,
								CustomID: "fd_signupStart",
							},
						},
					},
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.Button{
								Label:    "+1",
								Style:    discordgo.SecondaryButton,
								CustomID: "fd_tokens1",
							},
							discordgo.Button{
								Emoji: discordgo.ComponentEmoji{
									Name: "5️⃣",
								},
								//Label:    "5",
								Style:    discordgo.SecondaryButton,
								CustomID: "fd_tokens5",
							},
							discordgo.Button{
								Emoji: discordgo.ComponentEmoji{
									Name: "6️⃣",
								},
								//Label:    "6",
								Style:    discordgo.SecondaryButton,
								CustomID: "fd_tokens6",
							},
							discordgo.Button{
								Emoji: discordgo.ComponentEmoji{
									Name: "8️⃣",
								},
								//Label:    "8",
								Style:    discordgo.SecondaryButton,
								CustomID: "fd_tokens8",
							},
							discordgo.Button{
								Label:    "-1",
								Style:    discordgo.SecondaryButton,
								CustomID: "fd_tokens_sub",
							},
						},
					},
				}
				reactionMsg, err := s.ChannelMessageSendComplex(ChannelID, &data)

				if err != nil {
					print(err)
				}
				boost.SetReactionID(contract, msg.ChannelID, reactionMsg.ID)
				//s.MessageReactionAdd(msg.ChannelID, reactionMsg.ID, "🧑‍🌾") // Booster
				//s.MessageReactionAdd(msg.ChannelID, reactionMsg.ID, "🔔")   // Ping
				//s.MessageReactionAdd(msg.ChannelID, reactionMsg.ID, "⏱️")  // Creator Start Contract

				s.ChannelMessagePin(msg.ChannelID, reactionMsg.ID)
			} else {
				print(err)
			}

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
		slashUnboost: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
		slashChange: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			var str = ""
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
				}
			}
			/*
				if opt, ok := optionMap["contract-id"]; ok {
					contractID := opt.StringValue()
				}
				if opt, ok := optionMap["coop-id"]; ok {
					coopID := opt.StringValue()
				}
			*/
			if opt, ok := optionMap["boost-order"]; ok {
				boostOrder := opt.StringValue()
				err := boost.ChangeBoostOrder(s, i.GuildID, i.ChannelID, i.Member.User.ID, boostOrder)
				if err != nil {
					str += err.Error()
				}
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: str,
					//Flags:      discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{}},
			})
		},
	}
)

func main() {

	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Println("Boost Bot is up!")
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
			{
				Type:        discordgo.ApplicationCommandOptionRole,
				Name:        "ping-role",
				Description: "Role to use to ping for this contract. Default is @here.",
				Required:    false,
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
			{
				Name:        "order",
				Description: "How farmer should be added to contract. Default is Random.",
				Required:    false,
				Type:        discordgo.ApplicationCommandOptionInteger,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{
						Name:  "Random",
						Value: 2,
					},
					{
						Name:  "Last",
						Value: 0,
					},
				},
			},
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
	_, err = s.ApplicationCommandCreate(*AppID, *GuildID, &discordgo.ApplicationCommand{
		Name:        slashChange,
		Description: "Change aspects of a running contract",
		Options: []*discordgo.ApplicationCommandOption{
			/*{
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
			},*/
			{
				Type:        discordgo.ApplicationCommandOptionRole,
				Name:        "ping-role",
				Description: "Change the contract ping role.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "boost-order",
				Description: "Provide new boost order. Example: 1,2,3,6,7,5,7-10",
				Required:    false,
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
