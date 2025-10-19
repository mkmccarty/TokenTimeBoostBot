package farmerstate

import "github.com/bwmarrin/discordgo"

// SlashSetEggIncNameCommand creates a new slash command for setting Egg, Inc name
func SlashSetEggIncNameCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Set Egg, Inc game name.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "ei-ign",
				Description: "Egg Inc IGN",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "discord-name",
				Description: "Discord name for this IGN assignment. Used by coordinator or admin to set another farmers IGN",
				Required:    false,
			},
		},
	}
}
