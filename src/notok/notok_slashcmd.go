package notok

import (
	"github.com/bwmarrin/discordgo"
)

var integerFunMinValue float64 = 20.0

// SlashFunCommand returns the command for the /fun command
func SlashFunCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "OpenAI Fun",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
			discordgo.InteractionContextBotDM,
			discordgo.InteractionContextPrivateChannel,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
		},

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
				MinValue:    &integerFunMinValue,
				MaxValue:    250.0,
				Required:    false,
			},
		},
	}
}

// FunHandler handles the /fun command
func FunHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
	var gptOption = int64(0)
	var gptText = ""

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

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    "",
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	},
	)

	var _ = Notok(s, i, gptOption, gptText)
}
