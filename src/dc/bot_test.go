package dc

import (
	"testing"

	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// Routing is the part of the Bot worth pinning: commands and autocomplete
// route on the command name, components and modals on the segment of the
// CustomID before the first "#". Every already-posted message carries a
// CustomID in that shape, so the rule cannot change.

func TestCommandDispatchRoutesOnName(t *testing.T) {
	bot := NewBot()
	var got string
	bot.OnCommand("contract", func(e *CommandEvent) { got = e.CommandName() })
	bot.OnCommand("other", func(*CommandEvent) { t.Error("the wrong handler ran") })

	bot.onCommand(&events.ApplicationCommandInteractionCreate{
		ApplicationCommandInteraction: decode[discord.ApplicationCommandInteraction](t, commandPayload("contract", "")),
	})

	if got != "contract" {
		t.Errorf("handler saw %q, want contract", got)
	}
}

func TestComponentDispatchRoutesOnPrefix(t *testing.T) {
	bot := NewBot()
	var got string
	bot.OnComponent("fd_boost", func(e *ComponentEvent) { got = e.CustomID() })
	bot.OnComponent("fd_other", func(*ComponentEvent) { t.Error("the wrong handler ran") })

	bot.onComponent(&events.ComponentInteractionCreate{
		ComponentInteraction: decode[discord.ComponentInteraction](t, componentPayload("fd_boost#arg#hash", "2", "")),
	})

	if got != "fd_boost#arg#hash" {
		t.Errorf("handler saw %q", got)
	}
}

func TestModalDispatchRoutesOnPrefix(t *testing.T) {
	bot := NewBot()
	var got string
	bot.OnModal("m_eggid", func(e *ModalEvent) { got = e.CustomID() })

	bot.onModal(&events.ModalSubmitInteractionCreate{
		ModalSubmitInteraction: decode[discord.ModalSubmitInteraction](t, modalPayload("m_eggid#register", "egginc-id", "EI1")),
	})

	if got != "m_eggid#register" {
		t.Errorf("handler saw %q", got)
	}
}

func TestAutocompleteDispatchRoutesOnName(t *testing.T) {
	bot := NewBot()
	var got string
	bot.OnAutocomplete("contract", func(e *AutocompleteEvent) { got = e.CommandName() })

	bot.onAutocomplete(&events.AutocompleteInteractionCreate{
		AutocompleteInteraction: decode[discord.AutocompleteInteraction](t, autocompletePayload("contract", `[{"name": "contract-id", "type": 3, "value": "far", "focused": true}]`)),
	})

	if got != "contract" {
		t.Errorf("handler saw %q", got)
	}
}

// A button on a message older than the handler that made it has to be answered
// rather than left hanging with "this application did not respond".
func TestUnknownRouteReachesTheFallback(t *testing.T) {
	bot := NewBot()
	var meta InteractionMeta
	bot.OnUnknown(func(e *UnknownEvent) { meta = e.Meta() })
	bot.OnComponent("known", func(*ComponentEvent) { t.Error("the wrong handler ran") })

	bot.onComponent(&events.ComponentInteractionCreate{
		ComponentInteraction: decode[discord.ComponentInteraction](t, componentPayload("stale#hash", "2", "")),
	})

	if meta.Kind != KindComponent {
		t.Errorf("kind = %q, want %q", meta.Kind, KindComponent)
	}
	if meta.CustomID != "stale#hash" {
		t.Errorf("custom id = %q", meta.CustomID)
	}
	if meta.ChannelID != "300" || meta.GuildID != "900" || meta.UserID != "400" {
		t.Errorf("meta identity wrong: %+v", meta)
	}
}

func TestUnknownRouteWithoutFallbackIsIgnored(t *testing.T) {
	bot := NewBot()
	// No OnUnknown registered: the interaction is dropped rather than panicking.
	bot.onCommand(&events.ApplicationCommandInteractionCreate{
		ApplicationCommandInteraction: decode[discord.ApplicationCommandInteraction](t, commandPayload("nobody-handles-this", "")),
	})
}

// An unrouted autocomplete can only be answered with choices, and an unrouted
// command only with a message. Answering the wrong way is refused rather than
// sent to Discord to be rejected.
func TestUnknownEventAnswersOnlyItsOwnWay(t *testing.T) {
	bot := NewBot()
	var event *UnknownEvent
	bot.OnUnknown(func(e *UnknownEvent) { event = e })

	bot.onAutocomplete(&events.AutocompleteInteractionCreate{
		AutocompleteInteraction: decode[discord.AutocompleteInteraction](t, autocompletePayload("gone", `[{"name": "q", "type": 3, "value": "", "focused": true}]`)),
	})
	if event == nil {
		t.Fatal("the fallback did not run")
	}
	if err := event.Respond(Message{Content: "no"}); err != ErrWrongInteractionResponse {
		t.Errorf("responding to an autocomplete with a message returned %v", err)
	}

	bot.onModal(&events.ModalSubmitInteractionCreate{
		ModalSubmitInteraction: decode[discord.ModalSubmitInteraction](t, modalPayload("gone#hash", "x", "y")),
	})
	if err := event.RespondChoices(nil); err != ErrWrongInteractionResponse {
		t.Errorf("answering a modal with choices returned %v", err)
	}
}

// A panicking handler must reach the recovery hook rather than take the
// process down, and the hook needs enough to log what failed.
func TestPanicInHandlerReachesTheRecoverHook(t *testing.T) {
	bot := NewBot()
	var recovered any
	var meta InteractionMeta
	bot.OnPanic(func(r any, m InteractionMeta) { recovered, meta = r, m })
	bot.OnCommand("boom", func(*CommandEvent) { panic("handler exploded") })

	bot.onCommand(&events.ApplicationCommandInteractionCreate{
		ApplicationCommandInteraction: decode[discord.ApplicationCommandInteraction](t, commandPayload("boom", "")),
	})

	if recovered != "handler exploded" {
		t.Errorf("recovered %v", recovered)
	}
	if meta.Kind != KindCommand || meta.Name != "boom" {
		t.Errorf("meta wrong: %+v", meta)
	}
}

func TestRegisteredNamesAreReportedForCommandSync(t *testing.T) {
	bot := NewBot()
	bot.OnCommand("zebra", func(*CommandEvent) {})
	bot.OnCommand("apple", func(*CommandEvent) {})

	names := bot.CommandNames()
	if len(names) != 2 || names[0] != "apple" || names[1] != "zebra" {
		t.Errorf("names = %v, want sorted [apple zebra]", names)
	}
}

// Client is nil until Connect succeeds. Returning the typed nil pointer
// directly would hand back a non-nil interface, which every "client == nil"
// check misses — the crash reporter's among them.
func TestClientIsNilBeforeConnect(t *testing.T) {
	if client := NewBot().Client(); client != nil {
		t.Errorf("Client() = %v, want nil before Connect", client)
	}
}

func TestIntentsMapToDisgo(t *testing.T) {
	// The bot identifies with exactly these five, and the values are
	// Discord's own bits rather than the facade's ordering.
	got := (IntentGuilds | IntentGuildMessages | IntentDirectMessages |
		IntentGuildMessageReactions | IntentDirectMessageReactions).toDisgo()

	for _, want := range []struct {
		name string
		bit  int
	}{
		{"guilds", 1 << 0},
		{"guild messages", 1 << 9},
		{"guild message reactions", 1 << 10},
		{"direct messages", 1 << 12},
		{"direct message reactions", 1 << 13},
	} {
		if int(got)&want.bit == 0 {
			t.Errorf("the %s intent is missing from %d", want.name, got)
		}
	}
}

func TestRoutePrefixIsTheSegmentBeforeTheFirstHash(t *testing.T) {
	cases := map[string]string{
		"fd_boost#arg#hash": "fd_boost",
		"fd_boost":          "fd_boost",
		"#leading":          "",
	}
	for customID, want := range cases {
		if got := routePrefix(customID); got != want {
			t.Errorf("routePrefix(%q) = %q, want %q", customID, got, want)
		}
	}
}

// disgo caches nothing unless told to, and three Client methods answer from
// the cache rather than over REST. Losing any of these flags would make
// UserChannelPermissions fail on every call and GuildMemberWithColor always
// report no colour, neither of which shows up as a compile error.
func TestCacheFlagsCoverWhatTheClientReads(t *testing.T) {
	required := map[string]cache.Flags{
		"guilds":   cache.FlagGuilds,
		"channels": cache.FlagChannels,
		"roles":    cache.FlagRoles,
		"members":  cache.FlagMembers,
	}
	// Read back off a real Connect rather than off the constant, because the
	// bug this guards against was Connect never passing the options at all.
	// The token only has to parse: Connect builds the client without touching
	// the network, and the gateway is opened separately.
	b := NewBot()
	if err := b.Connect("MTU0NjUwMjM0NDE4OTkzOTg0Mw.fake.token", IntentGuilds); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	flags := b.gateway.Caches.CacheFlags()
	for name, flag := range required {
		if !flags.Has(flag) {
			t.Errorf("the %s cache is not enabled (flags=%d)", name, flags)
		}
	}
}
