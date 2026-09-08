package dc

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

// InteractionEvent is the behavior a CommandEvent and a ComponentEvent share:
// both can send a followup and both can edit the response already sent. A
// helper that renders the same view for a slash command and for a button
// click on that view takes this rather than one concrete event type.
type InteractionEvent interface {
	UserID() string
	ChannelID() string
	GuildID() string
	Member() *Member
	User() *User
	Respond(m Message) error
	Defer(ephemeral bool) error
	Followup(m Message) error
	FollowupMessage(m Message) (*MessageRef, error)
	EditResponse(m Message) error
	EditFollowup(messageID string, m Message) error

	// MessageID is the message the interaction came from. It is empty for a
	// command interaction, which has no source message.
	MessageID() string

	// MessageContent is the text of the message the interaction came from,
	// empty for a command interaction.
	MessageContent() string

	// FromComponent reports whether the interaction came from a component.
	// A component interaction already has a message on screen to edit; a
	// command interaction does not, and answers with a followup instead.
	FromComponent() bool
}

// idString renders a snowflake the way the facade passes IDs around, with the
// zero value becoming the empty string rather than "0".
func idString(id snowflake.ID) string {
	if id == 0 {
		return ""
	}
	return id.String()
}

// optionalIDString renders disgo's optional IDs, which are pointers because
// the field is absent for a DM.
func optionalIDString(id *snowflake.ID) string {
	if id == nil {
		return ""
	}
	return idString(*id)
}

// CommandEvent is one slash command invocation: the interaction plus the
// means to answer it. Handlers take this instead of a client and an event.
type CommandEvent struct {
	event  *events.ApplicationCommandInteractionCreate
	client *bot.Client
}

// data is the slash command payload. Every command this bot publishes is a
// slash command, so the assertion inside disgo's accessor always holds.
func (e *CommandEvent) data() discord.SlashCommandInteractionData {
	return e.event.SlashCommandInteractionData()
}

// CommandName is the name the command was invoked under. A single handler can
// back several names, so this is how it tells them apart.
func (e *CommandEvent) CommandName() string {
	return e.event.Data.CommandName()
}

// MessageID is empty for a command interaction, which has no source message.
func (e *CommandEvent) MessageID() string { return "" }

// MessageContent is always empty for a command interaction, which is not
// attached to a message.
func (e *CommandEvent) MessageContent() string { return "" }

// UserID is the ID of the user who ran the command, in a guild or a DM.
func (e *CommandEvent) UserID() string {
	return idString(e.event.User().ID)
}

// ChannelID is the channel the command was run in.
func (e *CommandEvent) ChannelID() string {
	return idString(e.event.Channel().ID())
}

// GuildID is the guild the command was run in, empty in a DM.
func (e *CommandEvent) GuildID() string {
	return optionalIDString(e.event.GuildID())
}

// Member is the guild-scoped identity of the user who ran the command, nil in
// a DM where there is no membership.
func (e *CommandEvent) Member() *Member {
	return resolvedMemberFrom(e.event.Member())
}

// User is the account that ran the command.
func (e *CommandEvent) User() *User {
	user := e.event.User()
	return userFrom(&user)
}

// Subcommand is the name of the subcommand the user invoked, for commands
// that group their behavior that way. The second return is false when the
// command was invoked with no subcommand.
//
// disgo lifts the subcommand out of the option tree while decoding, so the
// outermost name is the group when there is one and the subcommand otherwise.
func (e *CommandEvent) Subcommand() (string, bool) {
	path := e.SubcommandPath()
	if len(path) == 0 {
		return "", false
	}
	return path[0], true
}

// SubcommandPath is the full path of the invoked subcommand, outermost first.
// A command grouped as `/update contract coop-id` returns
// ["contract", "coop-id"]; a command with a single level of subcommands
// returns one element; a command invoked with no subcommand returns nil.
func (e *CommandEvent) SubcommandPath() []string {
	data := e.data()
	var path []string
	if data.SubCommandGroupName != nil {
		path = append(path, *data.SubCommandGroupName)
	}
	if data.SubCommandName != nil {
		path = append(path, *data.SubCommandName)
	}
	return path
}

// optionName resolves the facade's option naming onto disgo's flat option map.
//
// The bot addresses an option nested under a subcommand by its path joined
// with "-", so a "force" option on the "reload" subcommand of the "contract"
// group is "contract-reload-force". disgo flattens the tree while decoding and
// keys the leaves by their bare names, so the path prefix is stripped back off
// here. The bare name is accepted too, since after flattening there is nothing
// left for it to collide with.
func (e *CommandEvent) optionName(name string) string {
	for _, prefix := range e.SubcommandPath() {
		name = strings.TrimPrefix(name, prefix+"-")
	}
	return name
}

// HasOption reports whether the user supplied an option, whatever its value.
// It separates "the user left this out" from "the value did not come through",
// which the typed readers collapse into one false.
func (e *CommandEvent) HasOption(name string) bool {
	_, ok := e.data().Options[e.optionName(name)]
	return ok
}

// OptAttachment reads an attachment option, resolving it against the
// interaction's attachment table. The second return is false when the user did
// not supply the option or Discord sent no matching attachment.
func (e *CommandEvent) OptAttachment(name string) (*Attachment, bool) {
	attachment, ok := e.data().OptAttachment(e.optionName(name))
	if !ok {
		return nil, false
	}
	return attachmentFrom(&attachment), true
}

// OptUser reads a user option, resolved from the data Discord sends with the
// interaction. The second return is false when the user did not supply the
// option or Discord did not resolve it.
func (e *CommandEvent) OptUser(name string) (*User, bool) {
	user, ok := e.data().OptUser(e.optionName(name))
	if !ok {
		return nil, false
	}
	return userFrom(&user), true
}

// OptInt reads an integer option. The second return is false when the user
// did not supply the option.
func (e *CommandEvent) OptInt(name string) (int, bool) {
	return e.data().OptInt(e.optionName(name))
}

// OptString reads a string option.
func (e *CommandEvent) OptString(name string) (string, bool) {
	return e.data().OptString(e.optionName(name))
}

// OptBool reads a boolean option.
func (e *CommandEvent) OptBool(name string) (bool, bool) {
	return e.data().OptBool(e.optionName(name))
}

// OptionSummary is a flat name-to-text view of the options the user filled
// in, for logging. A user option renders as the username, and an option type
// it does not know becomes "Unknown".
func (e *CommandEvent) OptionSummary() map[string]string {
	data := e.data()
	summary := make(map[string]string, len(data.Options))
	for name, option := range data.Options {
		switch option.Type {
		case discord.ApplicationCommandOptionTypeString:
			summary[name] = option.String()
		case discord.ApplicationCommandOptionTypeInt:
			summary[name] = strconv.Itoa(option.Int())
		case discord.ApplicationCommandOptionTypeBool:
			summary[name] = strconv.FormatBool(option.Bool())
		case discord.ApplicationCommandOptionTypeUser:
			id := option.Snowflake()
			if user, ok := data.Resolved.Users[id]; ok {
				summary[name] = user.Username
			} else {
				summary[name] = idString(id)
			}
		default:
			summary[name] = "Unknown"
		}
	}
	return summary
}

// OptChannel reads a channel option and returns the channel's ID. The channel
// is identified by ID alone because that is what every caller passes on to
// Discord; the resolved channel object is not carried through the facade. The
// second return is false when the user did not supply the option.
func (e *CommandEvent) OptChannel(name string) (string, bool) {
	channel, ok := e.data().OptChannel(e.optionName(name))
	if !ok {
		return "", false
	}
	return idString(channel.ID), true
}

// Respond answers the interaction with a new message.
func (e *CommandEvent) Respond(m Message) error {
	return e.event.CreateMessage(m.toMessageCreate())
}

// Defer acknowledges the interaction without answering it, which buys past
// Discord's three second deadline. Answer afterwards with Followup.
func (e *CommandEvent) Defer(ephemeral bool) error {
	return e.event.DeferCreateMessage(ephemeral)
}

// Followup sends a message after the interaction has been deferred or already
// answered.
func (e *CommandEvent) Followup(m Message) error {
	_, err := e.FollowupMessage(m)
	return err
}

// FollowupMessage sends a followup and returns the message it created, for a
// caller that needs to edit or delete it later.
func (e *CommandEvent) FollowupMessage(m Message) (*MessageRef, error) {
	return followupMessage(e.client, e.event.ApplicationID(), e.event.Token(), m)
}

// EditFollowup edits a followup message this interaction already sent.
func (e *CommandEvent) EditFollowup(messageID string, m Message) error {
	return editFollowup(e.client, e.event.ApplicationID(), e.event.Token(), messageID, m)
}

// DeleteFollowup deletes a followup message this interaction already sent.
func (e *CommandEvent) DeleteFollowup(messageID string) error {
	return deleteFollowup(e.client, e.event.ApplicationID(), e.event.Token(), messageID)
}

// EditResponse replaces the interaction's original response.
func (e *CommandEvent) EditResponse(m Message) error {
	return editResponse(e.client, e.event.ApplicationID(), e.event.Token(), m)
}

// ShowModal answers the interaction by opening a modal form.
func (e *CommandEvent) ShowModal(modal Modal) error {
	return e.event.Modal(modal.toDisgo())
}

// FromComponent is false: a command interaction has no message on screen.
func (e *CommandEvent) FromComponent() bool { return false }

// ComponentEvent is one button press or select menu choice.
type ComponentEvent struct {
	event  *events.ComponentInteractionCreate
	client *bot.Client
}

// CustomID is the component's identifier, which carries this bot's
// "prefix#arg#...#contractHash" routing data.
func (e *ComponentEvent) CustomID() string {
	return e.event.Data.CustomID()
}

// Values are the options chosen from a select menu, empty for buttons.
func (e *ComponentEvent) Values() []string {
	data, ok := e.event.Data.(discord.StringSelectMenuInteractionData)
	if !ok {
		return nil
	}
	return data.Values
}

// MessageID is the ID of the message the interacted component is attached to.
func (e *ComponentEvent) MessageID() string {
	return idString(e.event.Message.ID)
}

// MessageContent is the text of the message the component lives on. A handler
// that only strips the buttons re-sends this so the text survives the update.
func (e *ComponentEvent) MessageContent() string {
	return e.event.Message.Content
}

// MessageComponentsWithoutActionRows is the message's top-level components
// with every ActionRow dropped, which is how a handler strips the controls off
// its own message without rebuilding the text that sits above them.
//
// The components come back opaque. They can be sent again unchanged, but the
// facade cannot inspect or edit them, because reading a component off Discord
// is not the same shape as the one that built it.
func (e *ComponentEvent) MessageComponentsWithoutActionRows() []LayoutComponent {
	var kept []LayoutComponent
	for _, component := range e.event.Message.Components {
		if component == nil || component.Type() == discord.ComponentTypeActionRow {
			continue
		}
		kept = append(kept, rawComponent{component: component})
	}
	return kept
}

// MessageIsEphemeral reports whether the message the component lives on is
// ephemeral. A handler replacing that message has to carry the flag forward,
// since Discord will not infer it from the original.
func (e *ComponentEvent) MessageIsEphemeral() bool {
	return e.event.Message.Flags.Has(discord.MessageFlagEphemeral)
}

// ChannelID is the channel the interaction was sent from.
func (e *ComponentEvent) ChannelID() string {
	return idString(e.event.Channel().ID())
}

// GuildID is the guild the interaction was sent from, empty in a DM.
func (e *ComponentEvent) GuildID() string {
	return optionalIDString(e.event.GuildID())
}

// UserID is the ID of the user who used the component.
func (e *ComponentEvent) UserID() string {
	return idString(e.event.User().ID)
}

// Member is the guild-scoped identity of the user who used the component, nil
// in a DM.
func (e *ComponentEvent) Member() *Member {
	return resolvedMemberFrom(e.event.Member())
}

// User is the account that used the component.
func (e *ComponentEvent) User() *User {
	user := e.event.User()
	return userFrom(&user)
}

// Respond answers with a new message rather than touching the one the
// component sits on.
func (e *ComponentEvent) Respond(m Message) error {
	return e.event.CreateMessage(m.toMessageCreate())
}

// Update replaces the message the component sits on.
func (e *ComponentEvent) Update(m Message) error {
	return e.event.UpdateMessage(m.toMessageUpdate())
}

// DeferUpdate acknowledges the interaction and leaves the message as it is,
// which is how a handler takes its time before redrawing.
func (e *ComponentEvent) DeferUpdate() error {
	return e.event.DeferUpdateMessage()
}

// Defer acknowledges the interaction and promises a new message rather than an
// edit of the existing one.
func (e *ComponentEvent) Defer(ephemeral bool) error {
	return e.event.DeferCreateMessage(ephemeral)
}

// Followup sends a message after the interaction has been deferred or already
// answered.
func (e *ComponentEvent) Followup(m Message) error {
	_, err := e.FollowupMessage(m)
	return err
}

// EditResponse replaces the interaction's original response.
func (e *ComponentEvent) EditResponse(m Message) error {
	return editResponse(e.client, e.event.ApplicationID(), e.event.Token(), m)
}

// DeleteResponse deletes the interaction's original response.
func (e *ComponentEvent) DeleteResponse() error {
	return e.client.Rest.DeleteInteractionResponse(e.event.ApplicationID(), e.event.Token())
}

// FollowupMessage sends a followup and returns the message it created.
func (e *ComponentEvent) FollowupMessage(m Message) (*MessageRef, error) {
	return followupMessage(e.client, e.event.ApplicationID(), e.event.Token(), m)
}

// EditFollowup edits a followup message this interaction already sent.
func (e *ComponentEvent) EditFollowup(messageID string, m Message) error {
	return editFollowup(e.client, e.event.ApplicationID(), e.event.Token(), messageID, m)
}

// FromComponent is true: a component interaction has a message on screen.
func (e *ComponentEvent) FromComponent() bool { return true }

// ShowModal answers the interaction by opening a modal form.
func (e *ComponentEvent) ShowModal(modal Modal) error {
	return e.event.Modal(modal.toDisgo())
}

// ModalEvent is one submitted modal form.
type ModalEvent struct {
	event  *events.ModalSubmitInteractionCreate
	client *bot.Client
}

// CustomID is the modal's identifier, carrying the same routing data as a
// component's.
func (e *ModalEvent) CustomID() string {
	return e.event.Data.CustomID
}

// UserID is the ID of the user who submitted the modal.
func (e *ModalEvent) UserID() string {
	return idString(e.event.User().ID)
}

// ChannelID is the channel the modal was opened from.
func (e *ModalEvent) ChannelID() string {
	return idString(e.event.Channel().ID())
}

// GuildID is the guild the modal was opened from, empty in a DM.
func (e *ModalEvent) GuildID() string {
	return optionalIDString(e.event.GuildID())
}

// Member is the guild-scoped identity of the submitting user, nil in a DM.
func (e *ModalEvent) Member() *Member {
	return resolvedMemberFrom(e.event.Member())
}

// User is the account that submitted the modal.
func (e *ModalEvent) User() *User {
	user := e.event.User()
	return userFrom(&user)
}

// MessageID is the message the modal was opened from, empty when the modal
// came from a command rather than a component.
func (e *ModalEvent) MessageID() string {
	if e.event.Message == nil {
		return ""
	}
	return idString(e.event.Message.ID)
}

// MessageContent is the text of the message the modal was opened from, empty
// when the modal came from a command.
func (e *ModalEvent) MessageContent() string {
	if e.event.Message == nil {
		return ""
	}
	return e.event.Message.Content
}

// FromComponent reports whether the modal was opened from a component, which
// is what decides between editing the message and sending a followup.
func (e *ModalEvent) FromComponent() bool {
	return e.event.Message != nil
}

// FollowupMessage sends a followup and returns the message it created.
func (e *ModalEvent) FollowupMessage(m Message) (*MessageRef, error) {
	return followupMessage(e.client, e.event.ApplicationID(), e.event.Token(), m)
}

// EditResponse replaces the interaction's original response.
func (e *ModalEvent) EditResponse(m Message) error {
	return editResponse(e.client, e.event.ApplicationID(), e.event.Token(), m)
}

// EditFollowup edits a followup message this interaction already sent.
func (e *ModalEvent) EditFollowup(messageID string, m Message) error {
	return editFollowup(e.client, e.event.ApplicationID(), e.event.Token(), messageID, m)
}

// TextValue reads the submitted value of one text input on the modal,
// identified by its own CustomID. It returns the empty string if no such
// input is present.
func (e *ModalEvent) TextValue(customID string) string {
	return e.event.Data.Text(customID)
}

// Respond answers the interaction with a new message.
func (e *ModalEvent) Respond(m Message) error {
	return e.event.CreateMessage(m.toMessageCreate())
}

// Update replaces the message the modal was opened from.
func (e *ModalEvent) Update(m Message) error {
	return e.event.UpdateMessage(m.toMessageUpdate())
}

// DeferUpdate acknowledges the submission and leaves the message as it is.
func (e *ModalEvent) DeferUpdate() error {
	return e.event.DeferUpdateMessage()
}

// Defer acknowledges the submission and promises a new message.
func (e *ModalEvent) Defer(ephemeral bool) error {
	return e.event.DeferCreateMessage(ephemeral)
}

// Followup sends a message after the submission has been deferred or already
// answered.
func (e *ModalEvent) Followup(m Message) error {
	_, err := e.FollowupMessage(m)
	return err
}

// ShowModal answers the submission by opening another modal.
func (e *ModalEvent) ShowModal(modal Modal) error {
	return ErrWrongInteractionResponse
}

// ReactionEvent is one reaction added to or removed from a message.
type ReactionEvent struct {
	messageID string
	channelID string
	guildID   string
	userID    string
	emojiName string
	emojiID   string
}

// reactionEventFrom converts the reaction payload both the add and the remove
// event carry.
func reactionEventFrom(r *events.GenericReaction) *ReactionEvent {
	event := &ReactionEvent{
		messageID: idString(r.MessageID),
		channelID: idString(r.ChannelID),
		userID:    idString(r.UserID),
		emojiName: "",
		emojiID:   "",
	}
	if r.GuildID != nil {
		event.guildID = idString(*r.GuildID)
	}
	if r.Emoji.Name != nil {
		event.emojiName = *r.Emoji.Name
	}
	if r.Emoji.ID != nil {
		event.emojiID = idString(*r.Emoji.ID)
	}
	return event
}

// MessageID is the message the reaction is on.
func (e *ReactionEvent) MessageID() string { return e.messageID }

// ChannelID is the channel the reacted-to message is in.
func (e *ReactionEvent) ChannelID() string { return e.channelID }

// GuildID is the guild the reaction happened in, empty in a DM.
func (e *ReactionEvent) GuildID() string { return e.guildID }

// UserID is the user who reacted.
func (e *ReactionEvent) UserID() string { return e.userID }

// EmojiName is the emoji's name: the character itself for a unicode emoji,
// the bare name for a custom one.
func (e *ReactionEvent) EmojiName() string { return e.emojiName }

// EmojiID is the snowflake of a custom emoji, empty for a unicode one.
func (e *ReactionEvent) EmojiID() string { return e.emojiID }

// EmojiRef identifies the emoji the way the REST API wants it: "name:id" for
// a custom emoji, the bare name for a unicode one. Client.RemoveMessageReaction
// takes this form.
func (e *ReactionEvent) EmojiRef() string {
	if e.emojiID == "" {
		return e.emojiName
	}
	return e.emojiName + ":" + e.emojiID
}

// MessageEvent is one message the bot can see.
type MessageEvent struct {
	message discord.Message
}

// MessageID is the message's own ID.
func (e *MessageEvent) MessageID() string {
	return idString(e.message.ID)
}

// ChannelID is the channel the message was posted in.
func (e *MessageEvent) ChannelID() string {
	return idString(e.message.ChannelID)
}

// GuildID is the guild the message was posted in, empty in a DM.
func (e *MessageEvent) GuildID() string {
	return optionalIDString(e.message.GuildID)
}

// Content is the message's text.
func (e *MessageEvent) Content() string {
	return e.message.Content
}

// AuthorID is the ID of the account that posted the message.
func (e *MessageEvent) AuthorID() string {
	return idString(e.message.Author.ID)
}

// AuthorIsBot reports whether the message came from a bot, which is how a
// handler avoids answering itself.
func (e *MessageEvent) AuthorIsBot() bool {
	return e.message.Author.Bot
}

// Attachments are the files uploaded with the message.
func (e *MessageEvent) Attachments() []Attachment {
	attachments := make([]Attachment, 0, len(e.message.Attachments))
	for i := range e.message.Attachments {
		if a := attachmentFrom(&e.message.Attachments[i]); a != nil {
			attachments = append(attachments, *a)
		}
	}
	return attachments
}

// AutocompleteEvent is one autocomplete request for a slash command option:
// the interaction plus the means to answer it.
type AutocompleteEvent struct {
	event *events.AutocompleteInteractionCreate
}

// CommandName is the command the option being completed belongs to.
func (e *AutocompleteEvent) CommandName() string {
	return e.event.Data.CommandName
}

// GuildID is the guild the command is being typed in, empty in a DM.
func (e *AutocompleteEvent) GuildID() string {
	return optionalIDString(e.event.GuildID())
}

// ChannelID is the channel the command is being typed in.
func (e *AutocompleteEvent) ChannelID() string {
	return idString(e.event.Channel().ID())
}

// UserID is the user typing the command.
func (e *AutocompleteEvent) UserID() string {
	return idString(e.event.User().ID)
}

// Subcommand is the subcommand being completed under, false when there is
// none.
func (e *AutocompleteEvent) Subcommand() (string, bool) {
	data := e.event.Data
	if data.SubCommandGroupName != nil {
		return *data.SubCommandGroupName, true
	}
	if data.SubCommandName != nil {
		return *data.SubCommandName, true
	}
	return "", false
}

// OptString reads a string option already filled in on the command being
// completed, which is how one option's suggestions depend on another.
func (e *AutocompleteEvent) OptString(name string) (string, bool) {
	return e.event.Data.OptString(name)
}

// FocusedOption is the option the user is currently typing into, which is the
// one the suggestions are for.
func (e *AutocompleteEvent) FocusedOption() (name string, value string) {
	focused := e.event.Data.Focused()
	if focused.Name == "" {
		return "", ""
	}
	return focused.Name, strings.Trim(string(focused.Value), `"`)
}

// RespondChoices answers with the suggestion list. An empty list is the way
// to say "no suggestions".
func (e *AutocompleteEvent) RespondChoices(choices []Choice[string]) error {
	return e.event.AutocompleteResult(stringChoicesToDisgoAutocomplete(choices))
}

// RespondChoicesInt answers an integer option's autocomplete.
func (e *AutocompleteEvent) RespondChoicesInt(choices []Choice[int]) error {
	return e.event.AutocompleteResult(intChoicesToDisgoAutocomplete(choices))
}

func stringChoicesToDisgoAutocomplete(choices []Choice[string]) []discord.AutocompleteChoice {
	out := make([]discord.AutocompleteChoice, 0, len(choices))
	for _, choice := range choices {
		out = append(out, discord.AutocompleteChoiceString{Name: choice.Name, Value: choice.Value})
	}
	return out
}

func intChoicesToDisgoAutocomplete(choices []Choice[int]) []discord.AutocompleteChoice {
	out := make([]discord.AutocompleteChoice, 0, len(choices))
	for _, choice := range choices {
		out = append(out, discord.AutocompleteChoiceInt{Name: choice.Name, Value: choice.Value})
	}
	return out
}

// The interaction webhook calls every event shares. They take the application
// ID and token rather than the event, because the four interaction kinds are
// four unrelated types with the same two fields.

func followupMessage(client *bot.Client, appID snowflake.ID, token string, m Message) (*MessageRef, error) {
	message, err := client.Rest.CreateFollowupMessage(appID, token, m.toMessageCreate())
	if err != nil {
		return nil, wrapAPIError(err)
	}
	return messageRefFrom(message), nil
}

func editFollowup(client *bot.Client, appID snowflake.ID, token string, messageID string, m Message) error {
	id, err := snowflake.Parse(messageID)
	if err != nil {
		return err
	}
	_, err = client.Rest.UpdateFollowupMessage(appID, token, id, m.toMessageUpdate())
	return wrapAPIError(err)
}

func deleteFollowup(client *bot.Client, appID snowflake.ID, token string, messageID string) error {
	id, err := snowflake.Parse(messageID)
	if err != nil {
		return err
	}
	return wrapAPIError(client.Rest.DeleteFollowupMessage(appID, token, id))
}

func editResponse(client *bot.Client, appID snowflake.ID, token string, m Message) error {
	_, err := client.Rest.UpdateInteractionResponse(appID, token, m.toMessageUpdate())
	return wrapAPIError(err)
}

// SnowflakeTimestamp is the creation time encoded in a Discord ID.
func SnowflakeTimestamp(id string) (time.Time, error) {
	parsed, err := snowflake.Parse(id)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.Time(), nil
}

// NewCommandEventFromPayload builds a CommandEvent from Discord's own
// interaction JSON.
//
// disgo keeps an interaction's identity fields unexported and fills them in
// while decoding, so the wire payload is the only way to construct one. Tests
// use this to exercise option parsing without a gateway; it is also what an
// HTTP-interaction transport would need.
func NewCommandEventFromPayload(payload []byte) (*CommandEvent, error) {
	var interaction discord.ApplicationCommandInteraction
	if err := json.Unmarshal(payload, &interaction); err != nil {
		return nil, err
	}
	return &CommandEvent{
		event: &events.ApplicationCommandInteractionCreate{
			ApplicationCommandInteraction: interaction,
		},
	}, nil
}
