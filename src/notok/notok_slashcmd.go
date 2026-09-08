package notok

import (
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// SlashFunCommand returns the command for the /fun command
func SlashFunCommand(cmd string) *dc.Command {
	command := dc.Command{
		Name:        cmd,
		Description: "LLM Fun",
		Contexts: []dc.InteractionContext{
			dc.ContextGuild,
			dc.ContextBotDM,
			dc.ContextPrivateChannel,
		},
		IntegrationTypes: []dc.IntegrationType{dc.IntegrationGuildInstall},
		Options: []dc.Option{
			dc.IntOption{
				Name:        "action",
				Description: "What interaction?",
				Required:    true,
				Choices: []dc.Choice[int]{
					{Name: "Wish for a token", Value: 1},
					{Name: "Compose letter asking for a token", Value: 5},
					/*
						{Name: "Let Me Out!", Value: 2},
						{Name: "Go Now!", Value: 3},
						{Name: "Generate image. Use prompt.", Value: 4},
					*/
				},
			},
			dc.StringOption{
				Name:        "prompt",
				Description: "Optional prompt to fine tune the original query.",
				Required:    false,
			},
		},
	}
	return &command
}

// HandleFun handles the /fun command
func HandleFun(client dc.Client, e *dc.CommandEvent) {
	// Protection against DM use
	if e.GuildID() == "" {
		_ = e.Respond(dc.Message{
			Content:   "This command can only be run in a server.",
			Ephemeral: true,
		})
		return
	}

	gptOption, _ := e.OptInt("action")
	gptText, _ := e.OptString("prompt")

	_ = e.Defer(true)

	_ = Notok(client, e, int64(gptOption), gptText)
}
