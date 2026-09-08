package dc

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

// This file renders the facade's message types into disgo's. It is the disgo
// half of message.go, which still renders the discordgo half until the REST
// client and the interaction events stop speaking discordgo.
//
// disgo needs two payload shapes where discordgo needed four: MessageCreate
// covers both sending to a channel and responding to an interaction, and
// MessageUpdate covers both editing a message and editing an interaction's
// original response.

func (t MentionType) disgoMentionType() discord.AllowedMentionType {
	switch t {
	case MentionRoles:
		return discord.AllowedMentionTypeRoles
	case MentionEveryone:
		return discord.AllowedMentionTypeEveryone
	default:
		return discord.AllowedMentionTypeUsers
	}
}

// disgoSnowflakes parses a list of IDs, dropping any that do not parse.
// Discord would reject a malformed ID anyway, and the facade's mention lists
// are built from IDs it already received from Discord.
func disgoSnowflakes(ids []string) []snowflake.ID {
	if len(ids) == 0 {
		return nil
	}
	out := make([]snowflake.ID, 0, len(ids))
	for _, id := range ids {
		parsed, err := snowflake.Parse(id)
		if err != nil {
			continue
		}
		out = append(out, parsed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// disgoAllowedMentions converts a to a *discord.AllowedMentions, returning nil
// when a is nil.
func (a *AllowedMentions) disgoAllowedMentions() *discord.AllowedMentions {
	if a == nil {
		return nil
	}
	mentions := &discord.AllowedMentions{
		Users: disgoSnowflakes(a.Users),
		Roles: disgoSnowflakes(a.Roles),
	}
	for _, parse := range a.Parse {
		mentions.Parse = append(mentions.Parse, parse.disgoMentionType())
	}
	return mentions
}

func (f EmbedField) disgoField() discord.EmbedField {
	field := discord.EmbedField{Name: f.Name, Value: f.Value}
	// disgo types Inline as a *bool. Setting it only when true keeps the
	// wire output identical to discordgo's `inline,omitempty` bool.
	if f.Inline {
		inline := true
		field.Inline = &inline
	}
	return field
}

func (f *EmbedFooter) disgoFooter() *discord.EmbedFooter {
	if f == nil {
		return nil
	}
	return &discord.EmbedFooter{Text: f.Text, IconURL: f.IconURL}
}

func (a *EmbedAuthor) disgoAuthor() *discord.EmbedAuthor {
	if a == nil {
		return nil
	}
	return &discord.EmbedAuthor{Name: a.Name, URL: a.URL, IconURL: a.IconURL}
}

func (e Embed) disgoEmbed() discord.Embed {
	embed := discord.Embed{
		Type:        discord.EmbedTypeRich,
		Title:       e.Title,
		Description: e.Description,
		URL:         e.URL,
		Color:       e.Color,
		Footer:      e.Footer.disgoFooter(),
		Author:      e.Author.disgoAuthor(),
	}
	if e.Thumbnail != "" {
		embed.Thumbnail = &discord.EmbedResource{URL: e.Thumbnail}
	}
	if e.Image != "" {
		embed.Image = &discord.EmbedResource{URL: e.Image}
	}
	if !e.Timestamp.IsZero() {
		// disgo marshals the timestamp itself, where discordgo took a
		// preformatted RFC 3339 string.
		timestamp := e.Timestamp
		embed.Timestamp = &timestamp
	}
	for _, field := range e.Fields {
		embed.Fields = append(embed.Fields, field.disgoField())
	}
	return embed
}

// disgoFile converts an attachment. disgo has no per-file content type: it
// writes every attachment as a plain multipart part and lets Discord infer the
// type from the filename, which is what Discord does with discordgo's header
// anyway.
func (f File) disgoFile() *discord.File {
	return &discord.File{Name: f.Name, Reader: f.Reader}
}

// disgoFlags is the flag set every outgoing payload shape shares. Components
// v2 messages must carry MessageFlagIsComponentsV2 or Discord rejects them.
func (m Message) disgoFlags() discord.MessageFlags {
	var flags discord.MessageFlags
	if m.Ephemeral {
		flags = flags.Add(discord.MessageFlagEphemeral)
	}
	if m.SuppressEmbeds {
		flags = flags.Add(discord.MessageFlagSuppressEmbeds)
	}
	if m.componentsV2() {
		flags = flags.Add(discord.MessageFlagIsComponentsV2)
	}
	return flags
}

// disgoComponentList drops any component Discord does not accept at the top
// level of a message, which disgoLayout reports by returning nil.
func (m Message) disgoComponentList() []discord.LayoutComponent {
	var components []discord.LayoutComponent
	for _, component := range m.Components {
		if converted := disgoLayout(component); converted != nil {
			components = append(components, converted)
		}
	}
	return components
}

func (m Message) disgoEmbedList() []discord.Embed {
	var embeds []discord.Embed
	for _, embed := range m.Embeds {
		embeds = append(embeds, embed.disgoEmbed())
	}
	return embeds
}

func (m Message) disgoFileList() []*discord.File {
	var files []*discord.File
	for _, file := range m.Files {
		files = append(files, file.disgoFile())
	}
	return files
}

// toMessageCreate renders the message as a send payload. disgo uses the same
// shape for a channel send and for an interaction response, so this replaces
// both of the discordgo renderings.
func (m Message) toMessageCreate() discord.MessageCreate {
	return discord.MessageCreate{
		Content:         m.content(),
		Flags:           m.disgoFlags(),
		Components:      m.disgoComponentList(),
		Embeds:          m.disgoEmbedList(),
		Files:           m.disgoFileList(),
		AllowedMentions: m.AllowedMentions.disgoAllowedMentions(),
	}
}

// toMessageUpdate renders the message as an edit payload, for an existing
// message or for an interaction's original response.
//
// Components are sent as an empty list rather than left unset whenever the
// message clears them, because an unset list leaves the components already on
// the message in place.
func (m Message) toMessageUpdate() discord.MessageUpdate {
	content := m.content()
	update := discord.MessageUpdate{
		Content:         &content,
		AllowedMentions: m.AllowedMentions.disgoAllowedMentions(),
	}
	components := m.disgoComponentList()
	if components == nil && m.ClearComponents {
		components = []discord.LayoutComponent{}
	}
	if components != nil {
		update.Components = &components
	}
	if embeds := m.disgoEmbedList(); embeds != nil {
		update.Embeds = &embeds
	}
	if files := m.disgoFileList(); files != nil {
		update.Files = files
	}
	if flags := m.disgoFlags(); flags != 0 {
		update.Flags = &flags
	}
	return update
}
