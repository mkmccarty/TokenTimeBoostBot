package dc

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/disgoorg/disgo/discord"
)

// asSlashCommand asserts the concrete type behind ToDisgo's interface return.
// Every test below reads fields off it, so the assertion lives in one place.
func asSlashCommand(t *testing.T, cmd Command) discord.SlashCommandCreate {
	t.Helper()
	rendered := cmd.ToDisgo()
	got, ok := rendered.(discord.SlashCommandCreate)
	if !ok {
		t.Fatalf("ToDisgo returned %T, want discord.SlashCommandCreate", rendered)
	}
	return got
}

func TestCommandToDisgo(t *testing.T) {
	min1, max1000 := 1, 1000
	got := asSlashCommand(t, Command{
		Name:             "chill",
		Description:      "Roll some dice.",
		Contexts:         []InteractionContext{ContextGuild},
		IntegrationTypes: []IntegrationType{IntegrationGuildInstall},
		Options: []Option{
			IntOption{Name: "die", Description: "sides", MinValue: &min1, MaxValue: &max1000},
		},
	})

	if got.Name != "chill" || got.Description != "Roll some dice." {
		t.Fatalf("name/description not carried: %+v", got)
	}
	// disgo types these as plain slices, where discordgo used pointers to slices.
	if len(got.Contexts) != 1 || got.Contexts[0] != discord.InteractionContextTypeGuild {
		t.Fatalf("contexts wrong: %+v", got.Contexts)
	}
	if len(got.IntegrationTypes) != 1 || got.IntegrationTypes[0] != discord.ApplicationIntegrationTypeGuildInstall {
		t.Fatalf("integration types wrong: %+v", got.IntegrationTypes)
	}
	if len(got.Options) != 1 {
		t.Fatalf("want 1 option, got %d", len(got.Options))
	}
	opt, ok := got.Options[0].(discord.ApplicationCommandOptionInt)
	if !ok {
		t.Fatalf("option is %T, want ApplicationCommandOptionInt", got.Options[0])
	}
	if opt.Name != "die" || opt.Description != "sides" {
		t.Fatalf("option wrong: %+v", opt)
	}
	if opt.MinValue == nil || *opt.MinValue != 1 {
		t.Fatalf("min value wrong: %v", opt.MinValue)
	}
	// discordgo took MaxValue as a bare float64; disgo takes a *int, so an
	// unset maximum is nil rather than 0.
	if opt.MaxValue == nil || *opt.MaxValue != 1000 {
		t.Fatalf("max value wrong: %v", opt.MaxValue)
	}
	if opt.Required {
		t.Fatal("option should not be required")
	}
}

func TestIntOptionToDisgoUnboundedLimits(t *testing.T) {
	opt := IntOption{Name: "count", Description: "n"}.toDisgo().(discord.ApplicationCommandOptionInt)
	if opt.MinValue != nil {
		t.Fatalf("min should be unset, got %v", *opt.MinValue)
	}
	if opt.MaxValue != nil {
		t.Fatalf("max should be unset, got %v", *opt.MaxValue)
	}
}

func TestIntOptionToDisgoAutocompleteAndChoices(t *testing.T) {
	opt := IntOption{
		Name:         "size",
		Description:  "coop size",
		Required:     true,
		Autocomplete: true,
		Choices:      []Choice[int]{{Name: "small", Value: 4}, {Name: "large", Value: 10}},
	}.toDisgo().(discord.ApplicationCommandOptionInt)

	if !opt.Required || !opt.Autocomplete {
		t.Fatalf("required/autocomplete not carried: %+v", opt)
	}
	if len(opt.Choices) != 2 {
		t.Fatalf("want 2 choices, got %d", len(opt.Choices))
	}
	if opt.Choices[0].Name != "small" || opt.Choices[0].Value != 4 {
		t.Fatalf("first choice wrong: %+v", opt.Choices[0])
	}
	if opt.Choices[1].Value != 10 {
		t.Fatalf("second choice wrong: %+v", opt.Choices[1])
	}
}

func TestStringOptionToDisgoChoicesAndLengths(t *testing.T) {
	min2, max20 := 2, 20
	opt := StringOption{
		Name:        "name",
		Description: "a name",
		MinLength:   &min2,
		MaxLength:   &max20,
		Choices:     []Choice[string]{{Name: "First", Value: "first"}},
	}.toDisgo().(discord.ApplicationCommandOptionString)

	if opt.MinLength == nil || *opt.MinLength != 2 {
		t.Fatalf("min length wrong: %v", opt.MinLength)
	}
	// discordgo took MaxLength as a bare int; disgo takes a *int.
	if opt.MaxLength == nil || *opt.MaxLength != 20 {
		t.Fatalf("max length wrong: %v", opt.MaxLength)
	}
	if len(opt.Choices) != 1 || opt.Choices[0].Value != "first" {
		t.Fatalf("choices wrong: %+v", opt.Choices)
	}
}

func TestStringOptionToDisgoUnboundedLengths(t *testing.T) {
	opt := StringOption{Name: "text", Description: "free text"}.toDisgo().(discord.ApplicationCommandOptionString)
	if opt.MinLength != nil || opt.MaxLength != nil {
		t.Fatalf("lengths should be unset, got min=%v max=%v", opt.MinLength, opt.MaxLength)
	}
	if opt.Choices != nil {
		t.Fatalf("choices should be nil, got %+v", opt.Choices)
	}
}

func TestNumberOptionToDisgo(t *testing.T) {
	minValue, maxValue := 0.5, 9.5
	opt := NumberOption{
		Name:        "rate",
		Description: "a rate",
		MinValue:    &minValue,
		MaxValue:    &maxValue,
		Choices:     []Choice[float64]{{Name: "half", Value: 0.5}},
	}.toDisgo().(discord.ApplicationCommandOptionFloat)

	if opt.MinValue == nil || *opt.MinValue != 0.5 {
		t.Fatalf("min wrong: %v", opt.MinValue)
	}
	if opt.MaxValue == nil || *opt.MaxValue != 9.5 {
		t.Fatalf("max wrong: %v", opt.MaxValue)
	}
	if len(opt.Choices) != 1 || opt.Choices[0].Value != 0.5 {
		t.Fatalf("choices wrong: %+v", opt.Choices)
	}

	unbounded := NumberOption{Name: "n", Description: "n"}.toDisgo().(discord.ApplicationCommandOptionFloat)
	if unbounded.MinValue != nil || unbounded.MaxValue != nil {
		t.Fatalf("limits should be unset, got min=%v max=%v", unbounded.MinValue, unbounded.MaxValue)
	}
}

func TestSimpleOptionsToDisgo(t *testing.T) {
	cases := []struct {
		name string
		opt  Option
		want discord.ApplicationCommandOptionType
	}{
		{name: "bool", opt: BoolOption{Name: "flag", Description: "d", Required: true}, want: discord.ApplicationCommandOptionTypeBool},
		{name: "user", opt: UserOption{Name: "who", Description: "d", Required: true}, want: discord.ApplicationCommandOptionTypeUser},
		{name: "role", opt: RoleOption{Name: "role", Description: "d", Required: true}, want: discord.ApplicationCommandOptionTypeRole},
		{name: "attachment", opt: AttachmentOption{Name: "file", Description: "d", Required: true}, want: discord.ApplicationCommandOptionTypeAttachment},
		{name: "channel", opt: ChannelOption{Name: "channel", Description: "d", Required: true}, want: discord.ApplicationCommandOptionTypeChannel},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.opt.toDisgo()
			if got.Type() != tc.want {
				t.Fatalf("type = %v, want %v", got.Type(), tc.want)
			}
			if got.OptionDescription() != "d" {
				t.Fatalf("description = %q, want %q", got.OptionDescription(), "d")
			}
		})
	}
}

func TestChannelOptionToDisgoTypes(t *testing.T) {
	opt := ChannelOption{
		Name:         "channel",
		Description:  "where to look",
		ChannelTypes: []ChannelType{ChannelText, ChannelForum, ChannelNews},
	}.toDisgo().(discord.ApplicationCommandOptionChannel)

	want := []discord.ChannelType{
		discord.ChannelTypeGuildText,
		discord.ChannelTypeGuildForum,
		discord.ChannelTypeGuildNews,
	}
	if len(opt.ChannelTypes) != len(want) {
		t.Fatalf("want %d channel types, got %d", len(want), len(opt.ChannelTypes))
	}
	for i, ct := range want {
		if opt.ChannelTypes[i] != ct {
			t.Fatalf("channel type %d: want %v, got %v", i, ct, opt.ChannelTypes[i])
		}
	}

	unrestricted := ChannelOption{Name: "channel", Description: "anywhere"}.toDisgo().(discord.ApplicationCommandOptionChannel)
	if unrestricted.ChannelTypes != nil {
		t.Fatalf("expected no channel restriction, got %v", unrestricted.ChannelTypes)
	}
}

func TestSubCommandNestingToDisgo(t *testing.T) {
	cmd := asSlashCommand(t, Command{
		Name:        "admin",
		Description: "admin things",
		Options: []Option{
			SubCommandGroup{
				Name:        "contract",
				Description: "contract admin",
				Options: []SubCommand{{
					Name:        "reload",
					Description: "reload contracts",
					Options:     []Option{BoolOption{Name: "force", Description: "force"}},
				}},
			},
		},
	})

	group, ok := cmd.Options[0].(discord.ApplicationCommandOptionSubCommandGroup)
	if !ok {
		t.Fatalf("want a subcommand group, got %T", cmd.Options[0])
	}
	if len(group.Options) != 1 || group.Options[0].Name != "reload" {
		t.Fatalf("want subcommand reload, got %+v", group.Options)
	}
	if group.Options[0].Options[0].Type() != discord.ApplicationCommandOptionTypeBool {
		t.Fatalf("want a bool option, got %v", group.Options[0].Options[0].Type())
	}
}

func TestSubCommandWithoutGroupToDisgo(t *testing.T) {
	cmd := asSlashCommand(t, Command{
		Name:        "contract",
		Description: "contract things",
		Options: []Option{
			SubCommand{
				Name:        "create",
				Description: "create a contract",
				Options:     []Option{StringOption{Name: "id", Description: "contract id"}},
			},
		},
	})

	sub, ok := cmd.Options[0].(discord.ApplicationCommandOptionSubCommand)
	if !ok {
		t.Fatalf("want a subcommand, got %T", cmd.Options[0])
	}
	if sub.Name != "create" || len(sub.Options) != 1 {
		t.Fatalf("subcommand wrong: %+v", sub)
	}
	if sub.Options[0].Type() != discord.ApplicationCommandOptionTypeString {
		t.Fatalf("want a string option nested, got %v", sub.Options[0].Type())
	}
}

func TestDefaultMemberPermissionsToDisgo(t *testing.T) {
	perms := PermissionAdministrator
	cmd := asSlashCommand(t, Command{Name: "a", Description: "b", DefaultMemberPermissions: &perms})
	if !cmd.DefaultMemberPermissions.OK {
		t.Fatal("permissions should be set")
	}
	if got := cmd.DefaultMemberPermissions.Value; got == nil || int64(*got) != perms {
		t.Fatalf("permissions not carried: %v", got)
	}

	unset := asSlashCommand(t, Command{Name: "a", Description: "b"})
	if !unset.DefaultMemberPermissions.IsZero() {
		t.Fatalf("permissions should be omitted, got %v", unset.DefaultMemberPermissions)
	}
}

// A zero DefaultMemberPermissions is how this bot hides an admin command from
// everyone but server admins, so it has to survive as a literal 0 rather than
// being dropped as an empty value.
func TestZeroDefaultMemberPermissionsToDisgoIsSent(t *testing.T) {
	var none int64
	cmd := asSlashCommand(t, Command{Name: "admin", Description: "admin only", DefaultMemberPermissions: &none})

	encoded, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(encoded), `"default_member_permissions":"0"`) {
		t.Fatalf("zero permissions not sent: %s", encoded)
	}

	unset, err := json.Marshal(asSlashCommand(t, Command{Name: "open", Description: "anyone"}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(unset), "default_member_permissions") {
		t.Fatalf("unset permissions should be omitted: %s", unset)
	}
}

func TestPermissionConstantsMatchDisgo(t *testing.T) {
	if PermissionAdministrator != int64(discord.PermissionAdministrator) {
		t.Fatalf("PermissionAdministrator = %d, want %d", PermissionAdministrator, discord.PermissionAdministrator)
	}
	if PermissionManageGuild != int64(discord.PermissionManageGuild) {
		t.Fatalf("PermissionManageGuild = %d, want %d", PermissionManageGuild, discord.PermissionManageGuild)
	}
}

// disgo has no GuildID on the payload — a command is scoped by which REST call
// publishes it — so ToDisgo must leave Command.GuildID for the caller to read.
func TestCommandGuildIDIsNotInDisgoPayload(t *testing.T) {
	cmd := Command{Name: "admin-force-download", Description: "Force re-download", GuildID: "guild-1"}
	encoded, err := json.Marshal(asSlashCommand(t, cmd))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), "guild-1") {
		t.Fatalf("guild id leaked into the payload: %s", encoded)
	}
	if cmd.GuildID != "guild-1" {
		t.Fatalf("guild id lost from the definition: %q", cmd.GuildID)
	}
}

// The rendered payload must not alias the definition's pointers, or editing a
// published command would reach back into the command table.
func TestToDisgoCopiesLimitPointers(t *testing.T) {
	minValue, maxValue := 1, 10
	opt := IntOption{Name: "n", Description: "n", MinValue: &minValue, MaxValue: &maxValue}.
		toDisgo().(discord.ApplicationCommandOptionInt)

	if opt.MinValue == &minValue || opt.MaxValue == &maxValue {
		t.Fatal("rendered option aliases the definition's pointers")
	}

	minValue, maxValue = 99, 99
	if *opt.MinValue != 1 || *opt.MaxValue != 10 {
		t.Fatalf("rendered option tracked the definition: min=%d max=%d", *opt.MinValue, *opt.MaxValue)
	}
}
