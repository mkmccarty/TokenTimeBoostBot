package dc

import (
	"testing"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

func TestCommandEventIdentity(t *testing.T) {
	e := commandEventFrom(t, commandPayload("contract", ""))

	if e.CommandName() != "contract" {
		t.Errorf("command name = %q", e.CommandName())
	}
	if e.UserID() != "400" {
		t.Errorf("user id = %q", e.UserID())
	}
	if e.ChannelID() != "300" {
		t.Errorf("channel id = %q", e.ChannelID())
	}
	if e.GuildID() != "900" {
		t.Errorf("guild id = %q", e.GuildID())
	}
	// A command interaction is not attached to a message.
	if e.MessageID() != "" || e.MessageContent() != "" || e.FromComponent() {
		t.Error("a command event should carry no source message")
	}
}

// In a DM Discord sends a user and no member or guild, and the bot still has
// to know who ran the command.
func TestCommandEventInDM(t *testing.T) {
	e := commandEventFrom(t, commandPayloadDM("contract"))

	if e.UserID() != "400" {
		t.Errorf("user id = %q", e.UserID())
	}
	if e.GuildID() != "" {
		t.Errorf("guild id = %q, want empty in a DM", e.GuildID())
	}
	if e.Member() != nil {
		t.Errorf("member = %+v, want nil in a DM", e.Member())
	}
	if user := e.User(); user == nil || user.Username != "tester" {
		t.Errorf("user = %+v", user)
	}
}

func TestCommandEventMember(t *testing.T) {
	member := commandEventFrom(t, commandPayload("contract", "")).Member()
	if member == nil {
		t.Fatal("member is nil in a guild interaction")
	}
	if member.UserID != "400" || member.Nick != "Tester" {
		t.Errorf("member = %+v", member)
	}
	if len(member.Roles) != 1 || member.Roles[0] != "500" {
		t.Errorf("roles = %v", member.Roles)
	}
	if member.User == nil || member.User.Username != "tester" {
		t.Errorf("member user = %+v", member.User)
	}
}

func TestCommandEventOptions(t *testing.T) {
	e := commandEventFrom(t, commandPayload("contract", `[
		{"name": "contract-id", "type": 3, "value": "farmers-market"},
		{"name": "coop-size", "type": 4, "value": 11},
		{"name": "make-thread", "type": 5, "value": true}
	]`))

	if got, ok := e.OptString("contract-id"); !ok || got != "farmers-market" {
		t.Errorf("OptString = %q, %v", got, ok)
	}
	if got, ok := e.OptInt("coop-size"); !ok || got != 11 {
		t.Errorf("OptInt = %d, %v", got, ok)
	}
	if got, ok := e.OptBool("make-thread"); !ok || got != true {
		t.Errorf("OptBool = %v, %v", got, ok)
	}
	if _, ok := e.OptString("absent"); ok {
		t.Error("an option the user left out reported as present")
	}
}

// HasOption separates "the user left this out" from "the value did not come
// through", which the typed readers collapse into one false.
func TestHasOptionSeparatesAbsentFromEmpty(t *testing.T) {
	e := commandEventFrom(t, commandPayload("contract", `[{"name": "note", "type": 3, "value": ""}]`))

	if !e.HasOption("note") {
		t.Error("an option supplied empty reported as absent")
	}
	if e.HasOption("never-supplied") {
		t.Error("an option never supplied reported as present")
	}
}

// The bot addresses an option nested under a subcommand by its path joined
// with "-". disgo flattens the tree while decoding, so the prefix has to be
// stripped back off.
func TestOptionLookupTraversesSubcommands(t *testing.T) {
	e := commandEventFrom(t, commandPayload("admin", `[
		{"name": "contract", "type": 2, "options": [
			{"name": "reload", "type": 1, "options": [
				{"name": "force", "type": 5, "value": true}
			]}
		]}
	]`))

	if path := e.SubcommandPath(); len(path) != 2 || path[0] != "contract" || path[1] != "reload" {
		t.Fatalf("subcommand path = %v, want [contract reload]", path)
	}
	if name, ok := e.Subcommand(); !ok || name != "contract" {
		t.Errorf("Subcommand = %q, %v", name, ok)
	}
	if got, ok := e.OptBool("contract-reload-force"); !ok || !got {
		t.Errorf("path-joined lookup failed: %v, %v", got, ok)
	}
}

func TestCommandEventWithoutSubcommand(t *testing.T) {
	e := commandEventFrom(t, commandPayload("contract", `[{"name": "id", "type": 3, "value": "x"}]`))
	if path := e.SubcommandPath(); path != nil {
		t.Errorf("subcommand path = %v, want nil", path)
	}
	if _, ok := e.Subcommand(); ok {
		t.Error("a command with no subcommand reported one")
	}
}

func TestOptUserResolves(t *testing.T) {
	e := commandEventFrom(t, `{
		"id": "100", "application_id": "200", "type": 2, "token": "tok", "version": 1,
		`+testChannel+`, `+testMember+`, `+testGuild+`,
		"data": {"id": "1", "name": "voluntell", "type": 1,
			"options": [{"name": "farmer", "type": 6, "value": "777"}],
			"resolved": {"users": {"777": {"id": "777", "username": "sink", "discriminator": "0"}}}}
	}`)

	user, ok := e.OptUser("farmer")
	if !ok {
		t.Fatal("user option did not resolve")
	}
	if user.ID != "777" || user.Username != "sink" {
		t.Errorf("user = %+v", user)
	}
	if _, ok := e.OptUser("absent"); ok {
		t.Error("an absent user option resolved")
	}
}

func TestOptAttachmentResolves(t *testing.T) {
	e := commandEventFrom(t, `{
		"id": "100", "application_id": "200", "type": 2, "token": "tok", "version": 1,
		`+testChannel+`, `+testMember+`, `+testGuild+`,
		"data": {"id": "1", "name": "mint", "type": 1,
			"options": [{"name": "csv", "type": 11, "value": "888"}],
			"resolved": {"attachments": {"888": {"id": "888", "filename": "mint.csv", "size": 42,
				"url": "https://cdn.example/mint.csv", "proxy_url": "https://proxy.example/mint.csv",
				"content_type": "text/csv"}}}}
	}`)

	attachment, ok := e.OptAttachment("csv")
	if !ok {
		t.Fatal("attachment option did not resolve")
	}
	if attachment.Filename != "mint.csv" || attachment.ContentType != "text/csv" || attachment.Size != 42 {
		t.Errorf("attachment = %+v", attachment)
	}
}

func TestOptionSummaryRendersEachOptionType(t *testing.T) {
	e := commandEventFrom(t, `{
		"id": "100", "application_id": "200", "type": 2, "token": "tok", "version": 1,
		`+testChannel+`, `+testMember+`, `+testGuild+`,
		"data": {"id": "1", "name": "mixed", "type": 1, "options": [
			{"name": "text", "type": 3, "value": "hello"},
			{"name": "count", "type": 4, "value": 7},
			{"name": "flag", "type": 5, "value": false},
			{"name": "who", "type": 6, "value": "777"},
			{"name": "where", "type": 7, "value": "300"}
		], "resolved": {"users": {"777": {"id": "777", "username": "sink", "discriminator": "0"}}}}
	}`)

	summary := e.OptionSummary()
	want := map[string]string{
		"text":  "hello",
		"count": "7",
		"flag":  "false",
		"who":   "sink",
		"where": "Unknown",
	}
	for name, value := range want {
		if summary[name] != value {
			t.Errorf("summary[%q] = %q, want %q", name, summary[name], value)
		}
	}
}

func TestComponentEventIdentity(t *testing.T) {
	e := componentEventFrom(t, componentPayload("fd_boost#hash", "2",
		`{"id": "700", "channel_id": "300", "content": "Boost list", "timestamp": "2026-01-01T00:00:00Z",
		  "author": {"id": "1", "username": "bot", "discriminator": "0"}}`))

	if e.CustomID() != "fd_boost#hash" {
		t.Errorf("custom id = %q", e.CustomID())
	}
	if e.MessageID() != "700" {
		t.Errorf("message id = %q", e.MessageID())
	}
	if e.MessageContent() != "Boost list" {
		t.Errorf("message content = %q", e.MessageContent())
	}
	if !e.FromComponent() {
		t.Error("a component event must report itself as coming from a component")
	}
	if e.ChannelID() != "300" || e.GuildID() != "900" || e.UserID() != "400" {
		t.Errorf("identity wrong: channel=%q guild=%q user=%q", e.ChannelID(), e.GuildID(), e.UserID())
	}
}

func TestComponentEventSelectValues(t *testing.T) {
	e := componentEventFrom(t, `{
		"id": "100", "application_id": "200", "type": 3, "token": "tok", "version": 1,
		`+testChannel+`, `+testMember+`, `+testGuild+`,
		"message": {"id": "700", "channel_id": "300", "content": "", "timestamp": "2026-01-01T00:00:00Z",
			"author": {"id": "1", "username": "bot", "discriminator": "0"}},
		"data": {"custom_id": "fd_menu#hash", "component_type": 3, "values": ["rancoop", "leave"]}
	}`)

	values := e.Values()
	if len(values) != 2 || values[0] != "rancoop" || values[1] != "leave" {
		t.Errorf("values = %v", values)
	}
}

// A button press carries no values, and asking for them must not panic on the
// type assertion.
func TestComponentEventButtonHasNoValues(t *testing.T) {
	e := componentEventFrom(t, componentPayload("fd_boost#hash", "2", ""))
	if values := e.Values(); values != nil {
		t.Errorf("values = %v, want none for a button", values)
	}
}

// A handler replacing an ephemeral message has to carry the flag forward,
// since Discord will not infer it from the original.
func TestComponentEventMessageIsEphemeral(t *testing.T) {
	ephemeral := componentEventFrom(t, componentPayload("x#h", "2",
		`{"id": "700", "channel_id": "300", "content": "", "flags": 64, "timestamp": "2026-01-01T00:00:00Z",
		  "author": {"id": "1", "username": "bot", "discriminator": "0"}}`))
	if !ephemeral.MessageIsEphemeral() {
		t.Error("an ephemeral message did not report itself as one")
	}

	normal := componentEventFrom(t, componentPayload("x#h", "2", ""))
	if normal.MessageIsEphemeral() {
		t.Error("a normal message reported itself as ephemeral")
	}
}

// Stripping the controls off a message means re-sending everything that is not
// an action row.
func TestMessageComponentsWithoutActionRows(t *testing.T) {
	e := componentEventFrom(t, componentPayload("x#h", "2", `{
		"id": "700", "channel_id": "300", "content": "", "timestamp": "2026-01-01T00:00:00Z",
		"author": {"id": "1", "username": "bot", "discriminator": "0"},
		"components": [
			{"type": 10, "content": "kept"},
			{"type": 1, "components": [{"type": 2, "style": 1, "label": "Join", "custom_id": "j"}]},
			{"type": 14, "spacing": 1}
		]
	}`))

	kept := e.MessageComponentsWithoutActionRows()
	if len(kept) != 2 {
		t.Fatalf("kept %d components, want 2", len(kept))
	}
	for _, component := range kept {
		if disgoLayout(component).Type() == discord.ComponentTypeActionRow {
			t.Error("an action row survived the filter")
		}
	}
}

func TestModalEventIdentity(t *testing.T) {
	e := modalEventFrom(t, modalPayload("m_eggid#register", "egginc-id", "EI0000000000000001"))

	if e.CustomID() != "m_eggid#register" {
		t.Errorf("custom id = %q", e.CustomID())
	}
	if e.TextValue("egginc-id") != "EI0000000000000001" {
		t.Errorf("text value = %q", e.TextValue("egginc-id"))
	}
	if e.TextValue("not-a-field") != "" {
		t.Errorf("an absent input read as %q", e.TextValue("not-a-field"))
	}
	if e.UserID() != "400" || e.ChannelID() != "300" || e.GuildID() != "900" {
		t.Errorf("identity wrong: user=%q channel=%q guild=%q", e.UserID(), e.ChannelID(), e.GuildID())
	}
	// This modal came from a command, so there is no message behind it and the
	// handler must answer with a followup rather than an edit.
	if e.FromComponent() {
		t.Error("a modal opened from a command reported a source message")
	}
}

// Opening a modal from a modal submission is not something Discord allows.
func TestModalEventCannotOpenAnotherModal(t *testing.T) {
	e := modalEventFrom(t, modalPayload("m_eggid#register", "egginc-id", "EI1"))
	if err := e.ShowModal(Modal{CustomID: "again"}); err != ErrWrongInteractionResponse {
		t.Errorf("ShowModal returned %v", err)
	}
}

func TestAutocompleteEvent(t *testing.T) {
	e := autocompleteEventFrom(t, autocompletePayload("contract", `[
		{"name": "contract-id", "type": 3, "value": "farm", "focused": true},
		{"name": "coop-id", "type": 3, "value": "existing"}
	]`))

	if e.CommandName() != "contract" {
		t.Errorf("command name = %q", e.CommandName())
	}
	name, value := e.FocusedOption()
	if name != "contract-id" || value != "farm" {
		t.Errorf("focused option = %q/%q", name, value)
	}
	// One option's suggestions can depend on another already filled in.
	if got, ok := e.OptString("coop-id"); !ok || got != "existing" {
		t.Errorf("OptString = %q, %v", got, ok)
	}
	if e.ChannelID() != "300" || e.GuildID() != "900" || e.UserID() != "400" {
		t.Errorf("identity wrong: channel=%q guild=%q user=%q", e.ChannelID(), e.GuildID(), e.UserID())
	}
}

func TestAutocompleteSubcommand(t *testing.T) {
	e := autocompleteEventFrom(t, autocompletePayload("admin", `[
		{"name": "guild", "type": 1, "options": [
			{"name": "setting", "type": 3, "value": "ho", "focused": true}
		]}
	]`))

	name, ok := e.Subcommand()
	if !ok || name != "guild" {
		t.Errorf("subcommand = %q, %v", name, ok)
	}
	if focused, value := e.FocusedOption(); focused != "setting" || value != "ho" {
		t.Errorf("focused option = %q/%q", focused, value)
	}
}

func TestMessageEvent(t *testing.T) {
	message := decode[discord.Message](t, `{
		"id": "700", "channel_id": "300", "guild_id": "900", "content": "hello",
		"timestamp": "2026-01-01T00:00:00Z",
		"author": {"id": "400", "username": "tester", "discriminator": "0"},
		"attachments": [{"id": "888", "filename": "mint.csv", "size": 42,
			"url": "https://cdn.example/mint.csv", "proxy_url": "https://proxy.example/mint.csv"}]
	}`)
	e := &MessageEvent{message: message}

	if e.MessageID() != "700" || e.ChannelID() != "300" || e.GuildID() != "900" {
		t.Errorf("identity wrong: %+v", e)
	}
	if e.Content() != "hello" || e.AuthorID() != "400" {
		t.Errorf("content/author wrong: %q %q", e.Content(), e.AuthorID())
	}
	if e.AuthorIsBot() {
		t.Error("a human author reported as a bot")
	}
	attachments := e.Attachments()
	if len(attachments) != 1 || attachments[0].Filename != "mint.csv" {
		t.Errorf("attachments = %+v", attachments)
	}
}

// The message handler answers only humans, so the bot must not answer itself.
func TestMessageEventBotAuthor(t *testing.T) {
	message := decode[discord.Message](t, `{
		"id": "700", "channel_id": "300", "content": "beep", "timestamp": "2026-01-01T00:00:00Z",
		"author": {"id": "1", "username": "bot", "discriminator": "0", "bot": true}
	}`)
	if !(&MessageEvent{message: message}).AuthorIsBot() {
		t.Error("a bot author was not reported as one")
	}
}

// The REST API identifies an emoji as "name:id" for a custom one and the bare
// name for a unicode one. Client.RemoveMessageReaction takes that form.
func TestReactionEventEmojiRef(t *testing.T) {
	unicode := reactionEventFrom(&events.GenericReaction{
		MessageID: 700,
		ChannelID: 300,
		UserID:    400,
		Emoji:     discord.PartialEmoji{Name: stringPtr("🚀")},
	})
	if unicode.EmojiRef() != "🚀" {
		t.Errorf("unicode emoji ref = %q", unicode.EmojiRef())
	}
	if unicode.EmojiID() != "" {
		t.Errorf("unicode emoji carried an id: %q", unicode.EmojiID())
	}
	if unicode.MessageID() != "700" || unicode.ChannelID() != "300" || unicode.UserID() != "400" {
		t.Errorf("identity wrong: %+v", unicode)
	}

	customID := discord.PartialEmoji{Name: stringPtr("token"), ID: idPtr(999)}
	custom := reactionEventFrom(&events.GenericReaction{Emoji: customID})
	if custom.EmojiRef() != "token:999" {
		t.Errorf("custom emoji ref = %q", custom.EmojiRef())
	}
}

func TestSnowflakeTimestamp(t *testing.T) {
	// Discord's epoch is 2015-01-01; a snowflake of 0 sits exactly on it.
	got, err := SnowflakeTimestamp("0")
	if err != nil {
		t.Fatalf("parsing a snowflake: %v", err)
	}
	if got.UTC().Year() != 2015 {
		t.Errorf("timestamp = %v, want Discord's 2015 epoch", got.UTC())
	}
	if _, err := SnowflakeTimestamp("not-a-snowflake"); err == nil {
		t.Error("a malformed snowflake parsed without error")
	}
}

// A helper that renders the same view for a command and for a button click on
// that view takes the shared interface rather than one concrete event.
func TestEventsSatisfyInteractionEvent(t *testing.T) {
	var _ InteractionEvent = (*CommandEvent)(nil)
	var _ InteractionEvent = (*ComponentEvent)(nil)
	var _ InteractionEvent = (*ModalEvent)(nil)
}

func stringPtr(s string) *string { return &s }

func idPtr(id snowflake.ID) *snowflake.ID { return &id }

func TestComponentEventEditFollowupNilClient(t *testing.T) {
	e := componentEventFrom(t, componentPayload("fd_boost#hash", "2",
		`{"id": "700", "channel_id": "300", "content": "Boost list", "timestamp": "2026-01-01T00:00:00Z",
		  "author": {"id": "1", "username": "bot", "discriminator": "0"}}`))

	// With nil client, all of these should safely return nil without panicking
	if err := e.EditFollowup(e.MessageID(), Message{Content: "test"}); err != nil {
		t.Errorf("EditFollowup with MessageID returned err: %v", err)
	}
	if err := e.EditFollowup("@original", Message{Content: "test"}); err != nil {
		t.Errorf("EditFollowup with @original returned err: %v", err)
	}
	if err := e.EditFollowup("", Message{Content: "test"}); err != nil {
		t.Errorf("EditFollowup with empty returned err: %v", err)
	}
	if err := e.EditFollowup("800", Message{Content: "test"}); err != nil {
		t.Errorf("EditFollowup with different ID returned err: %v", err)
	}
}

func TestModalEventEditFollowupNilClient(t *testing.T) {
	e := modalEventFrom(t, `{
		"id": "100", "application_id": "200", "type": 5, "token": "tok", "version": 1,
		`+testChannel+`, `+testMember+`, `+testGuild+`,
		"message": {"id": "700", "channel_id": "300", "content": "hello", "timestamp": "2026-01-01T00:00:00Z"},
		"data": {"custom_id": "md_settings", "components": []}
	}`)

	if err := e.EditFollowup(e.MessageID(), Message{Content: "test"}); err != nil {
		t.Errorf("EditFollowup with MessageID returned err: %v", err)
	}
	if err := e.EditFollowup("@original", Message{Content: "test"}); err != nil {
		t.Errorf("EditFollowup with @original returned err: %v", err)
	}
}
