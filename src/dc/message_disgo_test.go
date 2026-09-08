package dc

import (
	"strings"
	"testing"
	"time"

	"github.com/disgoorg/disgo/discord"
)

// These cover the message-level behaviors the parity test cannot reach:
// attachments and the edit-only fields, which discordgo splits across two
// payload types and disgo carries on one.

func TestDisgoMessageCreateFlags(t *testing.T) {
	componentsV2 := Message{Components: []LayoutComponent{TextDisplay{Content: "x"}}}.toMessageCreate()
	if !componentsV2.Flags.Has(discord.MessageFlagIsComponentsV2) {
		t.Error("a components v2 message must carry MessageFlagIsComponentsV2")
	}

	// Opting out of components v2 leaves the flag off, which is what lets a
	// message mix content, embeds and a row of buttons.
	v1 := Message{Content: "hi", ComponentsV1: true, Components: []LayoutComponent{
		ActionRow{Components: []InteractiveComponent{Button{Label: "x", CustomID: "x"}}},
	}}.toMessageCreate()
	if v1.Flags.Has(discord.MessageFlagIsComponentsV2) {
		t.Error("a components v1 message must not carry MessageFlagIsComponentsV2")
	}
	if v1.Content != "hi" {
		t.Errorf("components v1 dropped its content: %q", v1.Content)
	}

	both := Message{Content: "x", Ephemeral: true, SuppressEmbeds: true}.toMessageCreate()
	if !both.Flags.Has(discord.MessageFlagEphemeral) || !both.Flags.Has(discord.MessageFlagSuppressEmbeds) {
		t.Errorf("flags = %d, want ephemeral and suppress embeds", both.Flags)
	}
}

func TestDisgoMessageCreateFiles(t *testing.T) {
	create := Message{Files: []File{
		{Name: "contract.csv", ContentType: "text/csv", Reader: strings.NewReader("a,b\n")},
	}}.toMessageCreate()

	if len(create.Files) != 1 {
		t.Fatalf("want 1 file, got %d", len(create.Files))
	}
	if create.Files[0].Name != "contract.csv" {
		t.Errorf("file name = %q, want contract.csv", create.Files[0].Name)
	}
	if create.Files[0].Reader == nil {
		t.Error("file reader was dropped")
	}
}

// Editing a message without components leaves the components already on it in
// place, so a handler that only takes its buttons away has to send an empty
// list rather than none at all.
func TestDisgoMessageUpdateClearComponents(t *testing.T) {
	cleared := Message{Content: "done", ClearComponents: true}.toMessageUpdate()
	if cleared.Components == nil {
		t.Fatal("ClearComponents must send a components list")
	}
	if len(*cleared.Components) != 0 {
		t.Errorf("want an empty components list, got %#v", *cleared.Components)
	}

	untouched := Message{Content: "done"}.toMessageUpdate()
	if untouched.Components != nil {
		t.Errorf("an edit with no components should leave them alone, got %#v", *untouched.Components)
	}
}

func TestDisgoMessageUpdateAlwaysSendsContent(t *testing.T) {
	// A components v2 edit blanks the content, and that blanking has to reach
	// Discord or the old text stays under the new components.
	update := Message{
		Content:    "dropped by components v2",
		Components: []LayoutComponent{TextDisplay{Content: "new"}},
	}.toMessageUpdate()

	if update.Content == nil {
		t.Fatal("an edit must always send content")
	}
	if *update.Content != "" {
		t.Errorf("content = %q, want empty for a components v2 edit", *update.Content)
	}
}

func TestDisgoEmbedTimestamp(t *testing.T) {
	when := time.Date(2026, 9, 7, 14, 25, 0, 0, time.UTC)
	withTimestamp := Embed{Title: "t", Timestamp: when}.disgoEmbed()
	if withTimestamp.Timestamp == nil || !withTimestamp.Timestamp.Equal(when) {
		t.Errorf("timestamp = %v, want %v", withTimestamp.Timestamp, when)
	}

	// The facade uses the zero time as "unset", so it must not reach Discord
	// as a real timestamp.
	without := Embed{Title: "t"}.disgoEmbed()
	if without.Timestamp != nil {
		t.Errorf("an unset timestamp should be omitted, got %v", without.Timestamp)
	}
}

func TestDisgoAllowedMentionsDropsMalformedIDs(t *testing.T) {
	mentions := (&AllowedMentions{
		Users: []string{"502524949551120387", "not-a-snowflake"},
		Roles: []string{"also-not-a-snowflake"},
	}).disgoAllowedMentions()

	if len(mentions.Users) != 1 || mentions.Users[0].String() != "502524949551120387" {
		t.Errorf("users = %v, want just the parseable id", mentions.Users)
	}
	if mentions.Roles != nil {
		t.Errorf("roles = %v, want none once the unparseable id is dropped", mentions.Roles)
	}

	var absent *AllowedMentions
	if absent.disgoAllowedMentions() != nil {
		t.Error("nil AllowedMentions must stay nil, or it would suppress every mention")
	}
}
