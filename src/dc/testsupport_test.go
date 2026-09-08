package dc

import (
	"encoding/json"
	"testing"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// Interactions are built from Discord's own JSON here rather than from struct
// literals. disgo keeps an interaction's identity fields unexported and fills
// them in while decoding, so the wire payload is the only way to make one —
// which has the side benefit that these tests exercise the same decoding path
// the gateway does.

func decode[T any](t *testing.T, payload string) T {
	t.Helper()
	var out T
	if err := json.Unmarshal([]byte(payload), &out); err != nil {
		t.Fatalf("decoding interaction: %v\n%s", err, payload)
	}
	return out
}

func commandEventFrom(t *testing.T, payload string) *CommandEvent {
	t.Helper()
	return &CommandEvent{event: &events.ApplicationCommandInteractionCreate{
		ApplicationCommandInteraction: decode[discord.ApplicationCommandInteraction](t, payload),
	}}
}

func componentEventFrom(t *testing.T, payload string) *ComponentEvent {
	t.Helper()
	return &ComponentEvent{event: &events.ComponentInteractionCreate{
		ComponentInteraction: decode[discord.ComponentInteraction](t, payload),
	}}
}

func modalEventFrom(t *testing.T, payload string) *ModalEvent {
	t.Helper()
	return &ModalEvent{event: &events.ModalSubmitInteractionCreate{
		ModalSubmitInteraction: decode[discord.ModalSubmitInteraction](t, payload),
	}}
}

func autocompleteEventFrom(t *testing.T, payload string) *AutocompleteEvent {
	t.Helper()
	return &AutocompleteEvent{event: &events.AutocompleteInteractionCreate{
		AutocompleteInteraction: decode[discord.AutocompleteInteraction](t, payload),
	}}
}

// The payload fragments below are the parts every interaction carries. They
// are kept separate so a test names only the part it cares about.

const (
	testChannel = `"channel": {"id": "300", "type": 0}`
	testUser    = `"user": {"id": "400", "username": "tester", "discriminator": "0"}`
	testMember  = `"member": {"user": {"id": "400", "username": "tester", "discriminator": "0"}, "roles": ["500"], "joined_at": "2026-01-01T00:00:00Z", "nick": "Tester"}`
	testGuild   = `"guild_id": "900"`
)

// commandPayload builds a slash command interaction run in a guild.
func commandPayload(name, options string) string {
	if options == "" {
		options = "[]"
	}
	return `{
		"id": "100", "application_id": "200", "type": 2, "token": "tok", "version": 1,
		` + testChannel + `, ` + testMember + `, ` + testGuild + `,
		"data": {"id": "1", "name": "` + name + `", "type": 1, "options": ` + options + `}
	}`
}

// commandPayloadDM builds the same interaction sent in a DM, where Discord
// sends a user and no member or guild.
func commandPayloadDM(name string) string {
	return `{
		"id": "100", "application_id": "200", "type": 2, "token": "tok", "version": 1,
		` + testChannel + `, ` + testUser + `,
		"data": {"id": "1", "name": "` + name + `", "type": 1, "options": []}
	}`
}

// componentPayload builds a component interaction on a message.
func componentPayload(customID, componentType, message string) string {
	if message == "" {
		message = `{"id": "700", "channel_id": "300", "content": "", "timestamp": "2026-01-01T00:00:00Z", "author": {"id": "1", "username": "bot", "discriminator": "0"}}`
	}
	return `{
		"id": "100", "application_id": "200", "type": 3, "token": "tok", "version": 1,
		` + testChannel + `, ` + testMember + `, ` + testGuild + `,
		"message": ` + message + `,
		"data": {"custom_id": "` + customID + `", "component_type": ` + componentType + `}
	}`
}

// modalPayload builds a modal submission carrying one text input, in the label
// shape Discord returns for a modal built the way the facade builds them.
func modalPayload(customID, inputID, value string) string {
	return `{
		"id": "100", "application_id": "200", "type": 5, "token": "tok", "version": 1,
		` + testChannel + `, ` + testMember + `, ` + testGuild + `,
		"data": {"custom_id": "` + customID + `", "components": [
			{"type": 18, "label": "Field", "component": {"type": 4, "custom_id": "` + inputID + `", "value": "` + value + `"}}
		]}
	}`
}

// autocompletePayload builds an autocomplete request whose options say which
// one the user is typing into.
func autocompletePayload(name, options string) string {
	return `{
		"id": "100", "application_id": "200", "type": 4, "token": "tok", "version": 1,
		` + testChannel + `, ` + testMember + `, ` + testGuild + `,
		"data": {"id": "1", "name": "` + name + `", "type": 1, "options": ` + options + `}
	}`
}
