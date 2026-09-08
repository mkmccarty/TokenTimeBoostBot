package bottools

import (
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

var commandMap = make(map[string]string)

// UpdateDashboardDisplays is a callback used to notify the dashboard of data changes
var UpdateDashboardDisplays func(client dc.Client, userID string)

// UpdateCommandMap records the ID Discord assigned each published command so
// GetFormattedCommand can render a "</name:id>" mention link.
//
// The IDs come from what Discord returned; the subcommand paths come from the
// definitions the bot published, because a CommandRef carries only a name and
// an ID.
func UpdateCommandMap(definitions []*dc.Command, published []dc.CommandRef) {
	ids := make(map[string]string, len(published))
	for _, cmd := range published {
		ids[cmd.Name] = cmd.ID
	}

	for _, def := range definitions {
		id, ok := ids[def.Name]
		if !ok {
			continue
		}
		commandMap[def.Name] = id

		// Discord mentions a subcommand by the parent's ID with the path in
		// the name, so every reachable path gets the same ID.
		for _, option := range def.Options {
			switch opt := option.(type) {
			case dc.SubCommand:
				commandMap[def.Name+" "+opt.Name] = id
			case dc.SubCommandGroup:
				for _, sub := range opt.Options {
					commandMap[def.Name+" "+opt.Name+" "+sub.Name] = id
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
