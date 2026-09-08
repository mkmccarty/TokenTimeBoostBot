package dc

import (
	"io"
	"time"
)

// MentionType is a category of mention that can be allowed through in an
// AllowedMentions.Parse list.
type MentionType string

// Mention types.
const (
	MentionUsers    MentionType = "users"
	MentionRoles    MentionType = "roles"
	MentionEveryone MentionType = "everyone"
)

// AllowedMentions controls which mentions in a message actually notify.
//
// A nil *AllowedMentions leaves Discord's default behavior in place. A
// non-nil AllowedMentions with an empty Parse list suppresses all mentions.
type AllowedMentions struct {
	Parse []MentionType
	Users []string
	Roles []string
}

// EmbedField is one name/value pair inside an Embed.
type EmbedField struct {
	Name   string
	Value  string
	Inline bool
}

// EmbedFooter is the small text shown along the bottom edge of an Embed.
type EmbedFooter struct {
	Text    string
	IconURL string
}

// EmbedAuthor credits an Embed to a name, optionally linked and iconed.
type EmbedAuthor struct {
	Name    string
	URL     string
	IconURL string
}

// Embed is a rich embed attached to a Message. Its Type is always "rich" on
// the wire — no call site in this repo sets anything else.
type Embed struct {
	Title       string
	Description string
	URL         string
	Color       int
	Fields      []EmbedField
	Footer      *EmbedFooter
	Author      *EmbedAuthor
	Thumbnail   string
	Image       string
	Timestamp   time.Time
}

// File is a file attachment carried alongside a Message.
type File struct {
	Name        string
	ContentType string
	Reader      io.Reader
}

// Message is an outgoing message payload.
//
// Content and Components are mutually exclusive: Discord's components v2
// messages carry all of their text inside components, so Content is dropped
// whenever Components is non-empty.
type Message struct {
	Content         string
	Components      []LayoutComponent
	Ephemeral       bool
	Embeds          []Embed
	Files           []File
	AllowedMentions *AllowedMentions
	SuppressEmbeds  bool

	// ComponentsV1 keeps the message on Discord's original component model,
	// where Content, Embeds and Files coexist with a row of buttons.
	// Components v2 forbids that mix and carries all text inside components,
	// so a message that needs the old shape must opt out explicitly.
	ComponentsV1 bool

	// ClearComponents strips every component off the message being edited.
	// Editing a message without components leaves the ones already on it in
	// place, so a handler that only takes its buttons away has to say so.
	// It has no effect on a message being sent.
	ClearComponents bool
}

// componentsV2 reports whether the message is sent under Discord's components
// v2 model, which is the default for any message carrying components.
func (m Message) componentsV2() bool {
	return len(m.Components) > 0 && !m.ComponentsV1
}

// content is empty for components v2 messages, which carry all of their text
// inside components.
func (m Message) content() string {
	if m.componentsV2() {
		return ""
	}
	return m.Content
}
