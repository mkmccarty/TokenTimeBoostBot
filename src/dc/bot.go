package dc

import (
	"context"
	"sort"
	"strings"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
)

// InteractionKind names the four kinds of interaction the bot handles.
type InteractionKind string

// Interaction kinds.
const (
	KindCommand      InteractionKind = "command"
	KindComponent    InteractionKind = "component"
	KindModal        InteractionKind = "modal"
	KindAutocomplete InteractionKind = "autocomplete"
)

// InteractionMeta identifies one interaction without exposing the underlying
// client's types. It is what the unknown-route and panic hooks receive, so
// callers can log or answer without unwrapping the event.
type InteractionMeta struct {
	Kind      InteractionKind
	Name      string
	CustomID  string
	ChannelID string
	GuildID   string
	UserID    string
}

// Intents is the gateway event subscription set.
type Intents int

// Intents the bot subscribes to.
const (
	IntentGuilds Intents = 1 << iota
	IntentGuildMessages
	IntentDirectMessages
	IntentGuildMessageReactions
	IntentDirectMessageReactions
)

func (i Intents) toDisgo() gateway.Intents {
	var intents gateway.Intents
	if i&IntentGuilds != 0 {
		intents |= gateway.IntentGuilds
	}
	if i&IntentGuildMessages != 0 {
		intents |= gateway.IntentGuildMessages
	}
	if i&IntentDirectMessages != 0 {
		intents |= gateway.IntentDirectMessages
	}
	if i&IntentGuildMessageReactions != 0 {
		intents |= gateway.IntentGuildMessageReactions
	}
	if i&IntentDirectMessageReactions != 0 {
		intents |= gateway.IntentDirectMessageReactions
	}
	return intents
}

// cacheFlags is what the gateway keeps a local copy of. disgo caches nothing
// by default, and three Client methods answer from the cache rather than over
// REST: Channel reads channels, UserChannelPermissions needs channels and
// roles to compute an overwrite, and GuildMemberWithColor needs roles to find
// a member's highest coloured one. Members are cached too, so a permission
// check on someone the bot has already seen costs no request.
const cacheFlags = cache.FlagGuilds | cache.FlagChannels | cache.FlagRoles | cache.FlagMembers

// cacheConfigOpts is what Connect hands disgo. It is a function rather than an
// inline argument so a test can build a cache from the same options and read
// back the flags that actually reach disgo — asserting the constant alone
// would not catch Connect forgetting to pass it.
func cacheConfigOpts() []cache.ConfigOpt {
	return []cache.ConfigOpt{cache.WithCaches(cacheFlags)}
}

// Bot is the gateway connection plus the interaction routing table.
//
// Routing matches what the bot did before the facade existed: commands and
// autocomplete route on the command name, components and modals route on the
// segment of the CustomID before the first "#". disgo ships a handler.Router
// that would do the same job, but its paths are "/"-delimited, which would
// invalidate the "#"-delimited CustomIDs already sitting on posted messages.
type Bot struct {
	gateway *bot.Client
	client  *disgoClient

	commands      map[string]func(*CommandEvent)
	components    map[string]func(*ComponentEvent)
	modals        map[string]func(*ModalEvent)
	autocompletes map[string]func(*AutocompleteEvent)

	unknown func(*UnknownEvent)
	panics  func(recovered any, meta InteractionMeta)
	ready   func()

	messages        func(*MessageEvent)
	reactionAdds    func(*ReactionEvent)
	reactionRemoves func(*ReactionEvent)
}

// NewBot returns a Bot with an empty routing table and no gateway session.
// Use Connect to attach one.
func NewBot() *Bot {
	return &Bot{
		commands:      map[string]func(*CommandEvent){},
		components:    map[string]func(*ComponentEvent){},
		modals:        map[string]func(*ModalEvent){},
		autocompletes: map[string]func(*AutocompleteEvent){},
	}
}

// Connect builds the gateway client for a bot token and wires interaction
// dispatch to the routing table. It does not open the connection; call Open.
//
// The gateway listeners are fixed at build time, so every On* hook has to be
// registered before this runs.
func (b *Bot) Connect(token string, intents Intents) error {
	opts := []bot.ConfigOpt{
		bot.WithDefaultGateway(),
		bot.WithGatewayConfigOpts(gateway.WithIntents(intents.toDisgo())),
		bot.WithCacheConfigOpts(cacheConfigOpts()...),
		bot.WithEventListenerFunc(b.onCommand),
		bot.WithEventListenerFunc(b.onAutocomplete),
		bot.WithEventListenerFunc(b.onComponent),
		bot.WithEventListenerFunc(b.onModal),
	}
	if b.ready != nil {
		opts = append(opts, bot.WithEventListenerFunc(func(_ *events.Ready) { b.ready() }))
	}
	if b.messages != nil {
		opts = append(opts, bot.WithEventListenerFunc(func(e *events.MessageCreate) {
			b.messages(&MessageEvent{message: e.Message})
		}))
	}
	if b.reactionAdds != nil {
		opts = append(opts, bot.WithEventListenerFunc(func(e *events.MessageReactionAdd) {
			b.reactionAdds(reactionEventFrom(e.GenericReaction))
		}))
	}
	if b.reactionRemoves != nil {
		opts = append(opts, bot.WithEventListenerFunc(func(e *events.MessageReactionRemove) {
			b.reactionRemoves(reactionEventFrom(e.GenericReaction))
		}))
	}

	client, err := disgo.New(token, opts...)
	if err != nil {
		return err
	}
	b.gateway = client
	b.client = newDisgoClient(client)
	return nil
}

// Client is the bot's REST surface. It is nil until Connect succeeds.
func (b *Bot) Client() Client {
	if b.client == nil {
		// Returning b.client directly would hand back a non-nil interface
		// holding a nil pointer, which every "client == nil" check misses.
		return nil
	}
	return b.client
}

// Open connects to the gateway.
func (b *Bot) Open() error {
	return b.gateway.OpenGateway(context.Background())
}

// Close disconnects from the gateway. disgo's shutdown reports no error, so
// the error return exists only to keep the facade's shape.
func (b *Bot) Close() error {
	b.gateway.Close(context.Background())
	return nil
}

// UserID is the bot's own account ID, available once the gateway is open.
func (b *Bot) UserID() string {
	if b.gateway == nil {
		return ""
	}
	id := b.gateway.ID()
	if id == 0 {
		return ""
	}
	return id.String()
}

// SetPresence sets the bot's activity to "Playing <activity>".
func (b *Bot) SetPresence(activity string) error {
	return b.gateway.SetPresence(context.Background(), gateway.WithPlayingActivity(activity))
}

// OnCommand registers the handler for a slash command name.
func (b *Bot) OnCommand(name string, handler func(*CommandEvent)) {
	b.commands[name] = handler
}

// OnAutocomplete registers the autocomplete handler for a slash command name.
func (b *Bot) OnAutocomplete(name string, handler func(*AutocompleteEvent)) {
	b.autocompletes[name] = handler
}

// OnComponent registers the handler for every component CustomID starting with
// prefix, where prefix is the segment before the first "#".
func (b *Bot) OnComponent(prefix string, handler func(*ComponentEvent)) {
	b.components[prefix] = handler
}

// OnModal registers the handler for every modal CustomID starting with prefix.
func (b *Bot) OnModal(prefix string, handler func(*ModalEvent)) {
	b.modals[prefix] = handler
}

// OnMessage registers a handler for every message the bot can see. Call it
// before Connect.
func (b *Bot) OnMessage(handler func(*MessageEvent)) {
	b.messages = handler
}

// OnReactionAdd registers a handler for reactions added to any message the bot
// can see, including its own. Call it before Connect.
func (b *Bot) OnReactionAdd(handler func(*ReactionEvent)) {
	b.reactionAdds = handler
}

// OnReactionRemove registers a handler for reactions taken back off a message.
// Call it before Connect.
func (b *Bot) OnReactionRemove(handler func(*ReactionEvent)) {
	b.reactionRemoves = handler
}

// OnReady registers a callback run once the gateway reports ready. Call it
// before Connect.
func (b *Bot) OnReady(handler func()) {
	b.ready = handler
}

// OnUnknown registers the fallback for interactions with no registered route,
// which is how the bot answers stale buttons on old messages.
func (b *Bot) OnUnknown(handler func(*UnknownEvent)) {
	b.unknown = handler
}

// OnPanic registers the recovery hook for panics raised inside handlers.
// Without one, a panicking handler takes the process down.
func (b *Bot) OnPanic(handler func(recovered any, meta InteractionMeta)) {
	b.panics = handler
}

// CommandNames lists every registered command name, sorted, so command sync
// produces a stable order.
func (b *Bot) CommandNames() []string {
	names := make([]string, 0, len(b.commands))
	for name := range b.commands {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// routePrefix is the segment of a CustomID before the first "#".
func routePrefix(customID string) string {
	return strings.Split(customID, "#")[0]
}

// recoverInteraction hands a panic raised inside a handler to the OnPanic
// hook. Without a hook it re-panics, which takes the process down the way an
// unhandled panic in any goroutine would.
func (b *Bot) recoverInteraction(meta InteractionMeta) {
	recovered := recover()
	if recovered == nil {
		return
	}
	if b.panics == nil {
		panic(recovered)
	}
	b.panics(recovered, meta)
}

func (b *Bot) onCommand(e *events.ApplicationCommandInteractionCreate) {
	meta := commandMeta(e)
	defer b.recoverInteraction(meta)

	if handler, ok := b.commands[meta.Name]; ok {
		handler(&CommandEvent{event: e, client: b.gateway})
		return
	}
	b.routeUnknown(meta, func(m Message) error {
		return e.CreateMessage(m.toMessageCreate())
	}, nil)
}

func (b *Bot) onAutocomplete(e *events.AutocompleteInteractionCreate) {
	meta := autocompleteMeta(e)
	defer b.recoverInteraction(meta)

	if handler, ok := b.autocompletes[meta.Name]; ok {
		handler(&AutocompleteEvent{event: e})
		return
	}
	b.routeUnknown(meta, nil, func(choices []Choice[string]) error {
		return e.AutocompleteResult(stringChoicesToDisgoAutocomplete(choices))
	})
}

func (b *Bot) onComponent(e *events.ComponentInteractionCreate) {
	meta := componentMeta(e)
	defer b.recoverInteraction(meta)

	if handler, ok := b.components[routePrefix(meta.CustomID)]; ok {
		handler(&ComponentEvent{event: e, client: b.gateway})
		return
	}
	b.routeUnknown(meta, func(m Message) error {
		return e.CreateMessage(m.toMessageCreate())
	}, nil)
}

func (b *Bot) onModal(e *events.ModalSubmitInteractionCreate) {
	meta := modalMeta(e)
	defer b.recoverInteraction(meta)

	if handler, ok := b.modals[routePrefix(meta.CustomID)]; ok {
		handler(&ModalEvent{event: e, client: b.gateway})
		return
	}
	b.routeUnknown(meta, func(m Message) error {
		return e.CreateMessage(m.toMessageCreate())
	}, nil)
}

func (b *Bot) routeUnknown(meta InteractionMeta, respond func(Message) error, respondChoices func([]Choice[string]) error) {
	if b.unknown == nil {
		return
	}
	b.unknown(&UnknownEvent{meta: meta, respond: respond, respondChoices: respondChoices})
}

// interactionMeta fills in the fields every interaction has, whatever its
// kind. disgo types the guild ID as a pointer because a DM interaction has
// none, where the facade uses the empty string.
func interactionMeta(i discord.Interaction) InteractionMeta {
	meta := InteractionMeta{
		ChannelID: idString(i.Channel().ID()),
		UserID:    idString(i.User().ID),
	}
	if guildID := i.GuildID(); guildID != nil {
		meta.GuildID = guildID.String()
	}
	return meta
}

func commandMeta(e *events.ApplicationCommandInteractionCreate) InteractionMeta {
	meta := interactionMeta(e.ApplicationCommandInteraction)
	meta.Kind = KindCommand
	meta.Name = e.Data.CommandName()
	return meta
}

func autocompleteMeta(e *events.AutocompleteInteractionCreate) InteractionMeta {
	meta := interactionMeta(e.AutocompleteInteraction)
	meta.Kind = KindAutocomplete
	meta.Name = e.Data.CommandName
	return meta
}

func componentMeta(e *events.ComponentInteractionCreate) InteractionMeta {
	meta := interactionMeta(e.ComponentInteraction)
	meta.Kind = KindComponent
	meta.CustomID = e.Data.CustomID()
	return meta
}

func modalMeta(e *events.ModalSubmitInteractionCreate) InteractionMeta {
	meta := interactionMeta(e.ModalSubmitInteraction)
	meta.Kind = KindModal
	meta.CustomID = e.Data.CustomID
	return meta
}

// UnknownEvent is an interaction that matched no registered route: a button
// on a message older than the handler that made it, a command the bot no
// longer publishes. It carries enough to answer the user so the interaction
// does not hang with "this application did not respond".
//
// It holds the two ways to answer as closures rather than holding the
// interaction itself, because the four interaction kinds are four unrelated
// types and only one of them answers with choices.
type UnknownEvent struct {
	meta           InteractionMeta
	respond        func(Message) error
	respondChoices func([]Choice[string]) error
}

// Meta identifies the interaction that went unrouted.
func (e *UnknownEvent) Meta() InteractionMeta {
	return e.meta
}

// Respond answers with a message. Discord rejects this for an autocomplete
// interaction, which has no message response; use RespondChoices there.
func (e *UnknownEvent) Respond(m Message) error {
	if e.respond == nil {
		return ErrWrongInteractionResponse
	}
	return e.respond(m)
}

// RespondChoices answers an unrouted autocomplete, which only accepts a
// choice list. An empty list is the way to say "no suggestions".
func (e *UnknownEvent) RespondChoices(choices []Choice[string]) error {
	if e.respondChoices == nil {
		return ErrWrongInteractionResponse
	}
	return e.respondChoices(choices)
}
