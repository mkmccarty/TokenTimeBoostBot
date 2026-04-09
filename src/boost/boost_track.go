package boost

import (
	"github.com/bwmarrin/discordgo"
)

// GetSlashTokenEditCommand returns the slash command for token tracking removal
func GetSlashTokenEditCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Edit a tracked token",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "action",
				Description: "Select the auction to take",
				Type:        discordgo.ApplicationCommandOptionInteger,
				Required:    true,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{
						Name:  "Move Token",
						Value: 0,
					},
					{
						Name:  "Delete Token",
						Value: 1,
					},
					{
						Name:  "Modify Token Count",
						Value: 2,
					},
				},
			},
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "list",
				Description:  "The tracking list to remove the token from.",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:         discordgo.ApplicationCommandOptionInteger,
				Name:         "id",
				Description:  "Select the token to modify (last 15)",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "new-receiver",
				Description:  "Who received the token",
				Autocomplete: true,
				Required:     false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "new-quantity",
				Description: "Update token quantity",
				Required:    false,
				MinValue:    &integerOneMinValue,
				MaxValue:    32,
			},
		},
	}
}
