package ei

import (
	"testing"
)

func TestGetBotEmojiURL(t *testing.T) {
	origMap := EmoteMap
	defer func() {
		EmoteMap = origMap
	}()

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
