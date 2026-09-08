package dc

import "github.com/disgoorg/disgo/discord"

// InteractionContext is a place an application command may be invoked from.
type InteractionContext int

// Interaction contexts a command can be made available in.
const (
	ContextGuild InteractionContext = iota
	ContextBotDM
	ContextPrivateChannel
)

// IntegrationType is the installation scope a command is published under.
type IntegrationType int

// Integration types a command can be published under.
const (
	IntegrationGuildInstall IntegrationType = iota
	IntegrationUserInstall
)

// Option is one parameter of a slash command.
type Option interface {
	toDisgo() discord.ApplicationCommandOption
}

// Choice is one selectable value of a StringOption, IntOption or
// NumberOption. Value is limited to the types Discord accepts for choices.
type Choice[T string | int | float64] struct {
	Name  string
	Value T
}

// IntOption is an integer command parameter. MinValue and MaxValue are
// optional; nil means the limit is left to Discord. Choices and Autocomplete
// are mutually exclusive, matching Discord's own rule.
type IntOption struct {
	Name         string
	Description  string
	Required     bool
	Autocomplete bool
	MinValue     *int
	MaxValue     *int
	Choices      []Choice[int]
}

// StringOption is a string command parameter. MinLength and MaxLength are
// optional; nil means the limit is left to Discord. Choices and Autocomplete
// are mutually exclusive, matching Discord's own rule.
type StringOption struct {
	Name         string
	Description  string
	Required     bool
	Autocomplete bool
	Choices      []Choice[string]
	MinLength    *int
	MaxLength    *int
}

// NumberOption is a floating point command parameter. MinValue and MaxValue
// are optional; nil means the limit is left to Discord.
type NumberOption struct {
	Name        string
	Description string
	Required    bool
	MinValue    *float64
	MaxValue    *float64
	Choices     []Choice[float64]
}

// BoolOption is a boolean command parameter.
type BoolOption struct {
	Name        string
	Description string
	Required    bool
}

// UserOption is a command parameter that resolves to a guild member or user.
type UserOption struct {
	Name        string
	Description string
	Required    bool
}

// ChannelType is a kind of Discord channel. Only the kinds this bot restricts
// a ChannelOption to are modeled; add more as commands need them.
type ChannelType int

// Channel kinds a ChannelOption can be limited to.
const (
	ChannelText ChannelType = iota
	ChannelNews
	ChannelForum
)

// ChannelOption is a command parameter that resolves to a channel. An empty
// ChannelTypes lets the user pick any channel.
type ChannelOption struct {
	Name         string
	Description  string
	Required     bool
	ChannelTypes []ChannelType
}

// RoleOption is a command parameter that resolves to a role.
type RoleOption struct {
	Name        string
	Description string
	Required    bool
}

// AttachmentOption is a command parameter that resolves to an uploaded file.
type AttachmentOption struct {
	Name        string
	Description string
	Required    bool
}

// SubCommand is a named grouping of options nested one level under a Command
// or a SubCommandGroup.
type SubCommand struct {
	Name        string
	Description string
	Options     []Option
}

// SubCommandGroup nests SubCommands one level under a Command. Discord does
// not allow a group to contain another group, so Options is typed as
// []SubCommand rather than []Option: illegal nesting is a compile error
// instead of a runtime rejection.
type SubCommandGroup struct {
	Name        string
	Description string
	Options     []SubCommand
}

// Permission bits for Command.DefaultMemberPermissions, which restricts who
// Discord offers a command to by default. Named here so callers do not need
// to import discordgo.
const (
	// PermissionAdministrator is the administrator permission bit.
	PermissionAdministrator int64 = int64(discord.PermissionAdministrator)
	// PermissionManageGuild is the "Manage Server" permission bit.
	PermissionManageGuild int64 = int64(discord.PermissionManageGuild)
)

// Command is a slash command definition.
type Command struct {
	Name                     string
	Description              string
	Contexts                 []InteractionContext
	IntegrationTypes         []IntegrationType
	Options                  []Option
	DefaultMemberPermissions *int64

	// GuildID scopes the command to one guild instead of publishing it
	// globally. Empty means global, which is the usual case.
	GuildID string
}
