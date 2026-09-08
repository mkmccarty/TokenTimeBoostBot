package dc

import "github.com/disgoorg/disgo/discord"

// OptionValues is a snapshot of the options a command was invoked with.
//
// It exists for handlers that have to interrupt themselves and resume on a
// later interaction — ask for a modal, then finish the original work once the
// modal comes back. The second interaction carries none of the first one's
// options, so the handler stashes an OptionValues and reads from it instead.
//
// Names follow the same path-joined convention CommandEvent uses: an option
// nested under a subcommand is addressed as "group-subcommand-option".
type OptionValues struct {
	data discord.SlashCommandInteractionData
}

// Options snapshots the options this command was invoked with.
func (e *CommandEvent) Options() OptionValues {
	return OptionValues{data: e.data()}
}

// option resolves a path-joined name onto disgo's flat option map, the same
// way CommandEvent does.
func (o OptionValues) option(name string) (discord.SlashCommandOption, bool) {
	for _, prefix := range o.SubcommandPath() {
		name = trimNamePrefix(name, prefix)
	}
	opt, ok := o.data.Options[name]
	return opt, ok
}

func trimNamePrefix(name, prefix string) string {
	if len(name) > len(prefix)+1 && name[:len(prefix)+1] == prefix+"-" {
		return name[len(prefix)+1:]
	}
	return name
}

// Subcommand is the name of the subcommand that was invoked, if any.
func (o OptionValues) Subcommand() (string, bool) {
	path := o.SubcommandPath()
	if len(path) == 0 {
		return "", false
	}
	return path[0], true
}

// SubcommandPath is the full subcommand path, outermost first.
func (o OptionValues) SubcommandPath() []string {
	var path []string
	if o.data.SubCommandGroupName != nil {
		path = append(path, *o.data.SubCommandGroupName)
	}
	if o.data.SubCommandName != nil {
		path = append(path, *o.data.SubCommandName)
	}
	return path
}

// String reads a string option.
func (o OptionValues) String(name string) (string, bool) {
	opt, ok := o.option(name)
	if !ok || opt.Type != discord.ApplicationCommandOptionTypeString {
		return "", false
	}
	return opt.String(), true
}

// Int reads an integer option.
func (o OptionValues) Int(name string) (int, bool) {
	opt, ok := o.option(name)
	if !ok || opt.Type != discord.ApplicationCommandOptionTypeInt {
		return 0, false
	}
	return opt.Int(), true
}

// Uint reads an integer option as an unsigned value, for the callers that
// carry counts Discord sends as plain integers. A negative value reads as
// absent rather than wrapping around.
func (o OptionValues) Uint(name string) (uint64, bool) {
	value, ok := o.Int(name)
	if !ok || value < 0 {
		return 0, false
	}
	return uint64(value), true
}

// Float reads a number option.
func (o OptionValues) Float(name string) (float64, bool) {
	opt, ok := o.option(name)
	if !ok || opt.Type != discord.ApplicationCommandOptionTypeFloat {
		return 0, false
	}
	return opt.Float(), true
}

// Bool reads a boolean option.
func (o OptionValues) Bool(name string) (bool, bool) {
	opt, ok := o.option(name)
	if !ok || opt.Type != discord.ApplicationCommandOptionTypeBool {
		return false, false
	}
	return opt.Bool(), true
}

// User reads a user option, resolved against the data Discord sent with the
// original interaction.
func (o OptionValues) User(name string) (*User, bool) {
	opt, ok := o.option(name)
	if !ok || opt.Type != discord.ApplicationCommandOptionTypeUser {
		return nil, false
	}
	user, ok := o.data.Resolved.Users[opt.Snowflake()]
	if !ok {
		return nil, false
	}
	return userFrom(&user), true
}

// Attachment reads an attachment option, resolved against the data Discord
// sent with the original interaction.
func (o OptionValues) Attachment(name string) (*Attachment, bool) {
	opt, ok := o.option(name)
	if !ok || opt.Type != discord.ApplicationCommandOptionTypeAttachment {
		return nil, false
	}
	attachment, ok := o.data.Resolved.Attachments[opt.Snowflake()]
	if !ok {
		return nil, false
	}
	return attachmentFrom(&attachment), true
}
