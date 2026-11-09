package ei

import (
	"testing"
)

func TestGetBotEmojiURL(t *testing.T) {
	origMap := EmoteMap
	defer func() {
		EmoteMap = origMap
	}()

	EmoteMap = map[string]Emotes{
		"egg_truth": {
			Name:     "egg_truth",
			ID:       "1418358003333201990",
			Animated: false,
		},
		"animated_test": {
			Name:     "animated_test",
			ID:       "123456789",
			Animated: true,
		},
		"empty_id": {
			Name: "empty_id",
			ID:   "",
		},
	}

	// Standard static emoji
	got := GetBotEmojiURL("egg_truth", 128)
	want := "https://cdn.discordapp.com/emojis/1418358003333201990.webp?size=128"
	if got != want {
		t.Errorf("GetBotEmojiURL(egg_truth, 128) = %q; want %q", got, want)
	}

	// Default size when size <= 0
	gotDef := GetBotEmojiURL("egg_truth", 0)
	if gotDef != want {
		t.Errorf("GetBotEmojiURL(egg_truth, 0) = %q; want %q", gotDef, want)
	}

	// Animated emoji
	gotAnim := GetBotEmojiURL("animated_test", 64)
	wantAnim := "https://cdn.discordapp.com/emojis/123456789.gif?size=64"
	if gotAnim != wantAnim {
		t.Errorf("GetBotEmojiURL(animated_test, 64) = %q; want %q", gotAnim, wantAnim)
	}

	// Missing emoji
	if gotMissing := GetBotEmojiURL("non_existent", 128); gotMissing != "" {
		t.Errorf("GetBotEmojiURL(non_existent) = %q; want empty", gotMissing)
	}

	// Emoji with empty ID
	if gotEmpty := GetBotEmojiURL("empty_id", 128); gotEmpty != "" {
		t.Errorf("GetBotEmojiURL(empty_id) = %q; want empty", gotEmpty)
	}
}

func TestFindEggEmoji(t *testing.T) {
	origMap := EmoteMap
	defer func() {
		EmoteMap = origMap
		SetEmojiResolver(nil)
	}()

	EmoteMap = map[string]Emotes{
		"egg_copper": {
			Name: "egg_copper",
			ID:   "1001",
		},
		"wood": {
			Name: "egg_wood",
			ID:   "1002",
		},
		"egg_unknown": {
			Name: "egg_unknown",
			ID:   "9999",
		},
	}

	// Lookup by raw name
	got := FindEggEmoji("copper")
	if got != "<:egg_copper:1001>" {
		t.Errorf("FindEggEmoji(copper) = %q; want <:egg_copper:1001>", got)
	}

	// Lookup with egg_ prefix
	got = FindEggEmoji("egg_copper")
	if got != "<:egg_copper:1001>" {
		t.Errorf("FindEggEmoji(egg_copper) = %q; want <:egg_copper:1001>", got)
	}

	// Lookup uppercase
	got = FindEggEmoji("COPPER")
	if got != "<:egg_copper:1001>" {
		t.Errorf("FindEggEmoji(COPPER) = %q; want <:egg_copper:1001>", got)
	}

	// Lookup where only unprefixed key was mapped
	got = FindEggEmoji("wood")
	if got != "<:egg_wood:1002>" {
		t.Errorf("FindEggEmoji(wood) = %q; want <:egg_wood:1002>", got)
	}

	// Lookup missing emoji without resolver falls back to egg_unknown
	got = FindEggEmoji("nonexistent")
	if got != "<:egg_unknown:9999>" {
		t.Errorf("FindEggEmoji(nonexistent) = %q; want <:egg_unknown:9999>", got)
	}

	// Lookup missing emoji with resolver
	SetEmojiResolver(func(name string) (Emotes, bool) {
		if name == "egg_dynamicegg" || name == "dynamicegg" {
			return Emotes{Name: "egg_dynamicegg", ID: "2002"}, true
		}
		return Emotes{}, false
	})

	got = FindEggEmoji("dynamicegg")
	if got != "<:egg_dynamicegg:2002>" {
		t.Errorf("FindEggEmoji(dynamicegg) with resolver = %q; want <:egg_dynamicegg:2002>", got)
	}

	comp := FindEggComponentEmoji("dynamicegg")
	if comp == nil || comp.Name != "egg_dynamicegg" || comp.ID != "2002" {
		t.Errorf("FindEggComponentEmoji(dynamicegg) = %+v; want Name: egg_dynamicegg, ID: 2002", comp)
	}
}
