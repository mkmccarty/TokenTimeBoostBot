package boost

import (
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// A handler that reads an option its own schema never declares can never see a
// value: Discord only sends what the published command defines. Four commands
// had drifted that way, and one of them — /virtue's help screen — was
// unreachable because of it.
//
// The pairs below are the options whose absence was a real defect. This is not
// a substitute for reading the handler, but it does mean the schema cannot
// quietly lose one again.
var optionsHandlersRead = map[string][]string{
	"GetSlashContractReportCommand": {"reset", "token-details", "missing-players", "as-image", "contract-id"},
	"GetSlashRegisterCommand":       {"reset"},
	"GetSlashRerunEvalCommand":      {"reset", "refresh", "mobile-friendly", "season"},
	"GetSlashEstimateTime":          {"contract-id", "include-leggy", "show-gg", "te-override"},
	"GetSlashHelpCommand":           {"attributions"},
	"GetSlashCsEstimates":           {"contract-id", "coop-id", "sr-mode", "as-image", "private-reply"},
}

// declaresOption reports whether a command declares an option by this name,
// at the top level or nested under any subcommand. The bot addresses a nested
// option by its path, but the name itself is what a handler reads.
func declaresOption(options []dc.Option, name string) bool {
	for _, option := range options {
		switch opt := option.(type) {
		case dc.SubCommand:
			if declaresOption(opt.Options, name) {
				return true
			}
		case dc.SubCommandGroup:
			for _, sub := range opt.Options {
				if declaresOption(sub.Options, name) {
					return true
				}
			}
		case dc.BoolOption:
			if opt.Name == name {
				return true
			}
		case dc.StringOption:
			if opt.Name == name {
				return true
			}
		case dc.IntOption:
			if opt.Name == name {
				return true
			}
		case dc.NumberOption:
			if opt.Name == name {
				return true
			}
		case dc.UserOption:
			if opt.Name == name {
				return true
			}
		case dc.ChannelOption:
			if opt.Name == name {
				return true
			}
		case dc.RoleOption:
			if opt.Name == name {
				return true
			}
		case dc.AttachmentOption:
			if opt.Name == name {
				return true
			}
		}
	}
	return false
}

func TestCommandsDeclareTheOptionsTheyRead(t *testing.T) {
	for constructor, names := range optionsHandlersRead {
		build, ok := commandDefinitions[constructor]
		if !ok {
			t.Fatalf("%s is not in commandDefinitions; the test is out of date", constructor)
		}
		command := build("cmd-" + constructor)
		for _, name := range names {
			if !declaresOption(command.Options, name) {
				t.Errorf("%s: handler reads option %q that the schema does not declare", constructor, name)
			}
		}
	}
}

// Discord rejects a command that carries top-level options alongside
// subcommands, which is why /rerun-eval declares its reset flag on each
// subcommand rather than once at the top.
func TestSubcommandCommandsHaveNoTopLevelOptions(t *testing.T) {
	for constructor, build := range commandDefinitions {
		command := build("cmd-" + constructor)

		var hasSubcommands, hasPlainOptions bool
		for _, option := range command.Options {
			switch option.(type) {
			case dc.SubCommand, dc.SubCommandGroup:
				hasSubcommands = true
			default:
				hasPlainOptions = true
			}
		}
		if hasSubcommands && hasPlainOptions {
			t.Errorf("%s mixes subcommands with top-level options, which Discord rejects", constructor)
		}
	}
}

func checkOptionLimits(t *testing.T, cmdName string, option dc.Option) {
	check := func(name, desc string) {
		if len(name) < 1 || len(name) > 32 {
			t.Errorf("%s option %q: name length %d must be 1-32", cmdName, name, len(name))
		}
		if len(desc) < 1 || len(desc) > 100 {
			t.Errorf("%s option %q: description length %d must be 1-100 (%q)", cmdName, name, len(desc), desc)
		}
	}

	switch opt := option.(type) {
	case dc.SubCommand:
		check(opt.Name, opt.Description)
		for _, sub := range opt.Options {
			checkOptionLimits(t, cmdName+" "+opt.Name, sub)
		}
	case dc.SubCommandGroup:
		check(opt.Name, opt.Description)
		for _, sub := range opt.Options {
			checkOptionLimits(t, cmdName+" "+opt.Name, sub)
		}
	case dc.BoolOption:
		check(opt.Name, opt.Description)
	case dc.StringOption:
		check(opt.Name, opt.Description)
	case dc.IntOption:
		check(opt.Name, opt.Description)
	case dc.NumberOption:
		check(opt.Name, opt.Description)
	case dc.UserOption:
		check(opt.Name, opt.Description)
	case dc.ChannelOption:
		check(opt.Name, opt.Description)
	case dc.RoleOption:
		check(opt.Name, opt.Description)
	case dc.AttachmentOption:
		check(opt.Name, opt.Description)
	}
}

func TestDiscordCommandLengthLimits(t *testing.T) {
	for constructor, build := range commandDefinitions {
		command := build("cmd-" + constructor)
		if len(command.Description) < 1 || len(command.Description) > 100 {
			t.Errorf("%s: command description length %d must be 1-100 (%q)", constructor, len(command.Description), command.Description)
		}
		for _, option := range command.Options {
			checkOptionLimits(t, constructor, option)
		}
	}
}
