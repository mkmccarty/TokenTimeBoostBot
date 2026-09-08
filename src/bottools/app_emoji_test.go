package bottools

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

func TestFetchEmojisFromDiscord(t *testing.T) {
	client := &fakeDCClient{emojisResp: []dc.Emoji{
		{ID: "1", Name: "Token", Animated: false},
		{ID: "2", Name: "ULTRA_GG", Animated: true},
	}}

	got := fetchEmojisFromDiscord(client)

	if len(got) != 2 {
		t.Fatalf("want 2 emotes, got %d: %+v", len(got), got)
	}
	if e, ok := got["token"]; !ok || e.ID != "1" || e.Animated {
		t.Fatalf("token entry wrong: %+v ok=%v", e, ok)
	}
	if e, ok := got["ultra_gg"]; !ok || e.ID != "2" || !e.Animated {
		t.Fatalf("ultra_gg entry wrong: %+v ok=%v", e, ok)
	}
}

func TestFetchEmojisFromDiscordError(t *testing.T) {
	client := &fakeDCClient{emojisErr: errors.New("boom")}

	got := fetchEmojisFromDiscord(client)

	if len(got) != 0 {
		t.Fatalf("want empty map on error, got %+v", got)
	}
}

func TestImportSingleEmojiFromPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "my-emoji.png")
	content := []byte("fake-png-bytes")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	client := &fakeDCClient{createResp: &dc.Emoji{ID: "9", Name: "my-emoji", Animated: false}}

	got, err := importSingleEmojiFromPath(client, "my-emoji", path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantImage := "data:image/png;base64," + base64.StdEncoding.EncodeToString(content)
	if client.createParams.Name != "my-emoji" {
		t.Fatalf("name wrong: %q", client.createParams.Name)
	}
	if client.createParams.Image != wantImage {
		t.Fatalf("image wrong: got %q want %q", client.createParams.Image, wantImage)
	}
	want := ei.Emotes{Name: "my-emoji", ID: "9", Animated: false}
	if got != want {
		t.Fatalf("returned emote wrong: %+v want %+v", got, want)
	}
}

func TestImportSingleEmojiFromPathMissingFile(t *testing.T) {
	client := &fakeDCClient{}

	_, err := importSingleEmojiFromPath(client, "missing", filepath.Join(t.TempDir(), "nope.png"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if client.createCalled {
		t.Fatal("ApplicationEmojiCreate should not have been called")
	}
}

func TestImportSingleEmojiFromPathCreateError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "boom.gif")
	if err := os.WriteFile(path, []byte("gif-bytes"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	client := &fakeDCClient{createErr: errors.New("upload failed")}

	_, err := importSingleEmojiFromPath(client, "boom", path)
	if err == nil {
		t.Fatal("expected error from client")
	}
}
