package boost

import "github.com/mkmccarty/TokenTimeBoostBot/src/dc"

// guildOnlyCommand is the shape nearly every boost slash command shares:
// usable in a guild, installed to the guild. Callers fill in Options and any
// other field they need.
func guildOnlyCommand(name, description string) dc.Command {
	return dc.Command{
		Name:             name,
		Description:      description,
		Contexts:         []dc.InteractionContext{dc.ContextGuild},
		IntegrationTypes: []dc.IntegrationType{dc.IntegrationGuildInstall},
	}
}

// anywhereCommand is the shape used by commands a user can run outside the
// server they installed the bot into: any context, either installation.
func anywhereCommand(name, description string) dc.Command {
	return dc.Command{
		Name:             name,
		Description:      description,
		Contexts:         []dc.InteractionContext{dc.ContextGuild, dc.ContextBotDM, dc.ContextPrivateChannel},
		IntegrationTypes: []dc.IntegrationType{dc.IntegrationGuildInstall, dc.IntegrationUserInstall},
	}
}

// adminGuildCommand is guildOnlyCommand with DefaultMemberPermissions of zero,
// which is how this bot hides a command from everyone but server admins.
func adminGuildCommand(name, description string) dc.Command {
	var adminPermission int64
	command := guildOnlyCommand(name, description)
	command.DefaultMemberPermissions = &adminPermission
	return command
}
