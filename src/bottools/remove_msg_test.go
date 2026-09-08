package bottools

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
)

func newRemoveMessageEvent(messageOpt string, includeOpt bool) *dc.CommandEvent {
	if !includeOpt {
		return dctest.CommandEvent("remove-dm-message")
	}
	return dctest.CommandEvent("remove-dm-message", dctest.StringOption{Name: "message", Value: messageOpt})
}

func TestParseRemoveMessageIDFromBareID(t *testing.T) {
	e := newRemoveMessageEvent("1276990861158256664", true)
	if got := parseRemoveMessageID(e); got != "1276990861158256664" {
		t.Fatalf("want bare id, got %q", got)
	}
}

func TestParseRemoveMessageIDFromLink(t *testing.T) {
	e := newRemoveMessageEvent("https://discord.com/channels/@me/1124490885204287610/1276990861158256664", true)
	if got := parseRemoveMessageID(e); got != "1276990861158256664" {
		t.Fatalf("want trailing id from link, got %q", got)
	}
}

func TestParseRemoveMessageIDTrimsWhitespace(t *testing.T) {
	e := newRemoveMessageEvent("  1276990861158256664  ", true)
	if got := parseRemoveMessageID(e); got != "1276990861158256664" {
		t.Fatalf("want trimmed id, got %q", got)
	}
}

func TestParseRemoveMessageIDMissingOption(t *testing.T) {
	e := newRemoveMessageEvent("", false)
	if got := parseRemoveMessageID(e); got != "" {
		t.Fatalf("want empty, got %q", got)
	}
}

func TestRemoveBotMessageDeletesOwnMessage(t *testing.T) {
	orig := config.DiscordAppID
	config.DiscordAppID = "bot-1"
	defer func() { config.DiscordAppID = orig }()

	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	client := &fakeDCClient{getMessageResp: &dc.MessageRef{
		ID:        "msg-1",
		Author:    &dc.User{ID: "bot-1"},
		Timestamp: ts,
	}}

	got := removeBotMessage(client, "chan-1", "msg-1")

	if client.getMessageChannelID != "chan-1" || client.getMessageID != "msg-1" {
		t.Fatalf("GetMessage not called with expected args: %q %q", client.getMessageChannelID, client.getMessageID)
	}
	if client.deletedChannelID != "chan-1" || client.deletedMessageID != "msg-1" {
		t.Fatalf("DeleteMessage not called with expected args: %q %q", client.deletedChannelID, client.deletedMessageID)
	}
	want := fmt.Sprintf("Removed message from <t:%d:f>.", ts.Unix())
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestRemoveBotMessageRefusesOtherAuthor(t *testing.T) {
	orig := config.DiscordAppID
	config.DiscordAppID = "bot-1"
	defer func() { config.DiscordAppID = orig }()

	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	client := &fakeDCClient{getMessageResp: &dc.MessageRef{
		ID:        "msg-1",
		Author:    &dc.User{ID: "someone-else"},
		Timestamp: ts,
	}}

	got := removeBotMessage(client, "chan-1", "msg-1")

	if client.deletedChannelID != "" {
		t.Fatalf("DeleteMessage should not have been called, got channel %q", client.deletedChannelID)
	}
	want := fmt.Sprintf("The BoostBot can only remove its own messages. Message from <t:%d:f> was not removed.", ts.Unix())
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestRemoveBotMessageGetMessageError(t *testing.T) {
	client := &fakeDCClient{getMessageErr: errors.New("not found")}

	got := removeBotMessage(client, "chan-1", "msg-missing")

	want := "Failed to remove message with ID: msg-missing"
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestRemoveBotMessageDeleteError(t *testing.T) {
	orig := config.DiscordAppID
	config.DiscordAppID = "bot-1"
	defer func() { config.DiscordAppID = orig }()

	client := &fakeDCClient{
		getMessageResp: &dc.MessageRef{ID: "msg-1", Author: &dc.User{ID: "bot-1"}},
		deleteErr:      errors.New("delete failed"),
	}

	got := removeBotMessage(client, "chan-1", "msg-1")

	want := "Failed to remove message with ID: msg-1"
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestGetSlashRemoveMessage(t *testing.T) {
	cmd := GetSlashRemoveMessage("remove-dm-message")

	if cmd.Name != "remove-dm-message" {
		t.Fatalf("name wrong: %q", cmd.Name)
	}
	if len(cmd.Contexts) != 2 {
		t.Fatalf("contexts wrong: %v", cmd.Contexts)
	}
	if len(cmd.Options) != 1 {
		t.Fatalf("options wrong: %+v", cmd.Options)
	}
	opt, ok := cmd.Options[0].(dc.StringOption)
	if !ok || opt.Name != "message" || !opt.Required {
		t.Fatalf("options wrong: %+v", cmd.Options)
	}
}
