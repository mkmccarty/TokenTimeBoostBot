package boost

import (
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// GetSlashTokenEditCommand returns the slash command for token tracking removal
func GetSlashTokenEditCommand(cmd string) *dc.Command {
	quantityMin, quantityMax := 1, 32
	command := guildOnlyCommand(cmd, "Edit a tracked token")
	command.Options = []dc.Option{
		dc.IntOption{
			Name:        "action",
			Description: "Select the auction to take",
			Required:    true,
			Choices: []dc.Choice[int]{
				{Name: "Move Token", Value: 0},
				{Name: "Delete Token", Value: 1},
				{Name: "Modify Token Count", Value: 2},
			},
		},
		dc.StringOption{
			Name:         "list",
			Description:  "The tracking list to remove the token from.",
			Required:     true,
			Autocomplete: true,
		},
		dc.IntOption{
			Name:         "id",
			Description:  "Select the token to modify (last 15)",
			Required:     true,
			Autocomplete: true,
		},
		dc.StringOption{
			Name:         "new-receiver",
			Description:  "Who received the token",
			Autocomplete: true,
		},
		dc.IntOption{
			Name:        "new-quantity",
			Description: "Update token quantity",
			MinValue:    &quantityMin,
			MaxValue:    &quantityMax,
		},
	}
	return &command
}

// HandleTokenEditInteraction handles the /token-edit command interaction
func HandleTokenEditInteraction(client dc.Client, e *dc.CommandEvent) {
	var str string
	if e.GuildID() != "" {
		str = HandleTokenEditCommand(client, e)
	}
	_ = e.Respond(dc.Message{
		Content:   str,
		Ephemeral: true,
	})
}
