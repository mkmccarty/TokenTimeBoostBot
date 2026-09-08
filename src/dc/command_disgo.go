package dc

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/omit"
)

// This file renders the facade's command types into disgo's. It is the disgo
// half of command.go, which still renders the discordgo half until the REST
// client stops speaking discordgo.

func (c InteractionContext) toDisgo() discord.InteractionContextType {
	switch c {
	case ContextBotDM:
		return discord.InteractionContextTypeBotDM
	case ContextPrivateChannel:
		return discord.InteractionContextTypePrivateChannel
	default:
		return discord.InteractionContextTypeGuild
	}
}

func (t IntegrationType) toDisgo() discord.ApplicationIntegrationType {
	if t == IntegrationUserInstall {
		return discord.ApplicationIntegrationTypeUserInstall
	}
	return discord.ApplicationIntegrationTypeGuildInstall
}

func (t ChannelType) toDisgo() discord.ChannelType {
	switch t {
	case ChannelNews:
		return discord.ChannelTypeGuildNews
	case ChannelForum:
		return discord.ChannelTypeGuildForum
	default:
		return discord.ChannelTypeGuildText
	}
}

// disgo splits choices into one struct per value type, so the single generic
// helper the discordgo side uses does not carry over.

func stringChoicesToDisgo(choices []Choice[string]) []discord.ApplicationCommandOptionChoiceString {
	if len(choices) == 0 {
		return nil
	}
	out := make([]discord.ApplicationCommandOptionChoiceString, 0, len(choices))
	for _, c := range choices {
		out = append(out, discord.ApplicationCommandOptionChoiceString{Name: c.Name, Value: c.Value})
	}
	return out
}

func intChoicesToDisgo(choices []Choice[int]) []discord.ApplicationCommandOptionChoiceInt {
	if len(choices) == 0 {
		return nil
	}
	out := make([]discord.ApplicationCommandOptionChoiceInt, 0, len(choices))
	for _, c := range choices {
		out = append(out, discord.ApplicationCommandOptionChoiceInt{Name: c.Name, Value: c.Value})
	}
	return out
}

func floatChoicesToDisgo(choices []Choice[float64]) []discord.ApplicationCommandOptionChoiceFloat {
	if len(choices) == 0 {
		return nil
	}
	out := make([]discord.ApplicationCommandOptionChoiceFloat, 0, len(choices))
	for _, c := range choices {
		out = append(out, discord.ApplicationCommandOptionChoiceFloat{Name: c.Name, Value: c.Value})
	}
	return out
}

// copyInt and copyFloat hand disgo its own pointer rather than aliasing the
// caller's, so a command definition cannot be mutated through the rendered
// payload.

func copyInt(v *int) *int {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}

func copyFloat(v *float64) *float64 {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}

func (o IntOption) toDisgo() discord.ApplicationCommandOption {
	return discord.ApplicationCommandOptionInt{
		Name:         o.Name,
		Description:  o.Description,
		Required:     o.Required,
		Autocomplete: o.Autocomplete,
		MinValue:     copyInt(o.MinValue),
		MaxValue:     copyInt(o.MaxValue),
		Choices:      intChoicesToDisgo(o.Choices),
	}
}

func (o StringOption) toDisgo() discord.ApplicationCommandOption {
	return discord.ApplicationCommandOptionString{
		Name:         o.Name,
		Description:  o.Description,
		Required:     o.Required,
		Autocomplete: o.Autocomplete,
		MinLength:    copyInt(o.MinLength),
		MaxLength:    copyInt(o.MaxLength),
		Choices:      stringChoicesToDisgo(o.Choices),
	}
}

func (o NumberOption) toDisgo() discord.ApplicationCommandOption {
	return discord.ApplicationCommandOptionFloat{
		Name:        o.Name,
		Description: o.Description,
		Required:    o.Required,
		MinValue:    copyFloat(o.MinValue),
		MaxValue:    copyFloat(o.MaxValue),
		Choices:     floatChoicesToDisgo(o.Choices),
	}
}

func (o BoolOption) toDisgo() discord.ApplicationCommandOption {
	return discord.ApplicationCommandOptionBool{
		Name:        o.Name,
		Description: o.Description,
		Required:    o.Required,
	}
}

func (o UserOption) toDisgo() discord.ApplicationCommandOption {
	return discord.ApplicationCommandOptionUser{
		Name:        o.Name,
		Description: o.Description,
		Required:    o.Required,
	}
}

func (o ChannelOption) toDisgo() discord.ApplicationCommandOption {
	opt := discord.ApplicationCommandOptionChannel{
		Name:        o.Name,
		Description: o.Description,
		Required:    o.Required,
	}
	for _, t := range o.ChannelTypes {
		opt.ChannelTypes = append(opt.ChannelTypes, t.toDisgo())
	}
	return opt
}

func (o RoleOption) toDisgo() discord.ApplicationCommandOption {
	return discord.ApplicationCommandOptionRole{
		Name:        o.Name,
		Description: o.Description,
		Required:    o.Required,
	}
}

func (o AttachmentOption) toDisgo() discord.ApplicationCommandOption {
	return discord.ApplicationCommandOptionAttachment{
		Name:        o.Name,
		Description: o.Description,
		Required:    o.Required,
	}
}

// toDisgoSubCommand exists because disgo types a group's Options as the
// concrete []ApplicationCommandOptionSubCommand, not the option interface.
func (s SubCommand) toDisgoSubCommand() discord.ApplicationCommandOptionSubCommand {
	opt := discord.ApplicationCommandOptionSubCommand{
		Name:        s.Name,
		Description: s.Description,
	}
	for _, o := range s.Options {
		opt.Options = append(opt.Options, o.toDisgo())
	}
	return opt
}

func (s SubCommand) toDisgo() discord.ApplicationCommandOption {
	return s.toDisgoSubCommand()
}

func (g SubCommandGroup) toDisgo() discord.ApplicationCommandOption {
	opt := discord.ApplicationCommandOptionSubCommandGroup{
		Name:        g.Name,
		Description: g.Description,
	}
	for _, s := range g.Options {
		opt.Options = append(opt.Options, s.toDisgoSubCommand())
	}
	return opt
}

// ToDisgo renders the command as the payload disgo publishes. GuildID has no
// counterpart in the payload: disgo scopes a command by which REST call
// registers it, so the caller reads Command.GuildID itself.
func (c Command) ToDisgo() discord.ApplicationCommandCreate {
	cmd := discord.SlashCommandCreate{
		Name:        c.Name,
		Description: c.Description,
	}
	if c.DefaultMemberPermissions != nil {
		cmd.DefaultMemberPermissions = omit.NewPtr(discord.Permissions(*c.DefaultMemberPermissions))
	}
	for _, ctx := range c.Contexts {
		cmd.Contexts = append(cmd.Contexts, ctx.toDisgo())
	}
	for _, t := range c.IntegrationTypes {
		cmd.IntegrationTypes = append(cmd.IntegrationTypes, t.toDisgo())
	}
	for _, opt := range c.Options {
		cmd.Options = append(cmd.Options, opt.toDisgo())
	}
	return cmd
}
