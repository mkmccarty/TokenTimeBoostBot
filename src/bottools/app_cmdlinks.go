package bottools

import (
	"github.com/bwmarrin/discordgo"
)

var commandMap = make(map[string]string)

// UpdateDashboardDisplays is a callback used to notify the dashboard of data changes
var UpdateDashboardDisplays func(s *discordgo.Session, userID string)

// UpdateCommandMap updates the command map
func UpdateCommandMap(commands []*discordgo.ApplicationCommand) {
	for _, cmd := range commands {
		commandMap[cmd.Name] = cmd.ID
		// Handle subcommands and subcommand groups
		if len(cmd.Options) > 0 {
			for _, option := range cmd.Options {
				switch option.Type {
				case discordgo.ApplicationCommandOptionSubCommand:
					commandMap[cmd.Name+" "+option.Name] = cmd.ID
				case discordgo.ApplicationCommandOptionSubCommandGroup:
					for _, subOption := range option.Options {
						if subOption.Type == discordgo.ApplicationCommandOptionSubCommand {
							commandMap[cmd.Name+" "+option.Name+" "+subOption.Name] = cmd.ID
						}
					}
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
