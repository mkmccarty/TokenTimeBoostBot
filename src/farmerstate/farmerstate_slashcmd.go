package farmerstate

import (
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// SlashSetEggIncNameCommand creates a new slash command for setting Egg, Inc name
func SlashSetEggIncNameCommand(cmd string) *dc.Command {
	command := dc.Command{
		Name:        cmd,
		Description: "Set Egg, Inc game name.",
		Contexts: []dc.InteractionContext{
			dc.ContextGuild,
			dc.ContextBotDM,
			dc.ContextPrivateChannel,
		},
		IntegrationTypes: []dc.IntegrationType{
			dc.IntegrationGuildInstall,
			dc.IntegrationUserInstall,
		},
		Options: []dc.Option{
			dc.StringOption{
				Name:        "ei-ign",
				Description: "Egg Inc IGN",
				Required:    false,
			},
			dc.UserOption{
				Name:        "discord-name",
				Description: "Discord name for this IGN assignment. Used by coordinator or admin to set another farmers IGN",
				Required:    false,
			},
		},
	}
	return &command
}
