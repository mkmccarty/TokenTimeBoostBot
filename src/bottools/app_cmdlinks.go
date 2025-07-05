package bottools

import (
	"github.com/bwmarrin/discordgo"
)

var commandMap = make(map[string]string)

// UpdateCommandMap updates the command map
func UpdateCommandMap(commands []*discordgo.ApplicationCommand) {
	for _, cmd := range commands {
		commandMap[cmd.Name] = cmd.ID
		// Handle subcommands
		if len(cmd.Options) > 0 {
			for _, option := range cmd.Options {
				if option.Type == discordgo.ApplicationCommandOptionSubCommand {
					commandMap[cmd.Name+" "+option.Name] = cmd.ID
				}
			}
		}
	}
}

// GetFormattedCommand returns the formatted command string
func GetFormattedCommand(command string) string {
	if id, exists := commandMap[command]; exists {
		return "</" + command + ":" + id + ">"
	}
	return ""
}
