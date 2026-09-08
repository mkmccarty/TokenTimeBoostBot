package dctest

import (
	"errors"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// The fake is only useful if it is substitutable for the real client.
var _ dc.Client = (*FakeClient)(nil)

func TestLookupsAnswerFromTheRegisteredTables(t *testing.T) {
	client := New().
		WithGuild("g1", "Guild One").
		WithChannel("c1", "g1", "general").
		WithUser("u1", "someone", "Some One")
	client.WithMember("g1", "u1", "Nickname", 0x00cc00)

	guild, err := client.Guild("g1")
	if err != nil || guild.Name != "Guild One" {
		t.Fatalf("Guild = %+v, %v", guild, err)
	}

	channel, err := client.Channel("c1")
	if err != nil || channel.GuildID != "g1" {
		t.Fatalf("Channel = %+v, %v", channel, err)
	}

	member, color, err := client.GuildMemberWithColor("g1", "u1", "c1")
	if err != nil {
		t.Fatalf("GuildMemberWithColor: %v", err)
	}
	if member.Nick != "Nickname" || color != 0x00cc00 {
		t.Errorf("member = %+v, color = %#x", member, color)
	}
}

func TestUnregisteredIDsAreNotFound(t *testing.T) {
	client := New()

	if _, err := client.Guild("missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Guild error = %v, want ErrNotFound", err)
	}
	if _, err := client.Channel("missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Channel error = %v, want ErrNotFound", err)
	}
	if _, err := client.User("missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("User error = %v, want ErrNotFound", err)
	}
}

func TestCallsAreRecordedInOrder(t *testing.T) {
	client := New().WithChannel("c1", "g1", "general")

	if _, err := client.SendMessage("c1", dc.Message{Content: "hello"}); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if err := client.DeleteMessage("c1", "m1"); err != nil {
		t.Fatalf("DeleteMessage: %v", err)
	}

	if len(client.Calls) != 2 {
		t.Fatalf("recorded %d calls, want 2", len(client.Calls))
	}
	if client.Calls[0].Method != "SendMessage" || client.Calls[0].Args[1] != "hello" {
		t.Errorf("first call = %+v", client.Calls[0])
	}
	if !client.Called("DeleteMessage") {
		t.Error("DeleteMessage should be recorded")
	}
	if got := client.CallsTo("DeleteMessage"); len(got) != 1 || got[0].Args[1] != "m1" {
		t.Errorf("CallsTo(DeleteMessage) = %+v", got)
	}
}

func TestSendMessageErrorIsReturned(t *testing.T) {
	client := New()
	client.SendErr = errors.New("boom")

	if _, err := client.SendMessage("c1", dc.Message{Content: "hi"}); err == nil {
		t.Fatal("want the configured error")
	}
	if !client.Called("SendMessage") {
		t.Error("a failed send is still a call")
	}
}

func TestCommandEventReadsOptions(t *testing.T) {
	e := CommandEvent("remove-dm-message", StringOption{Name: "message", Value: "12345"})

	if e.CommandName() != "remove-dm-message" {
		t.Errorf("CommandName = %q", e.CommandName())
	}
	got, ok := e.OptString("message")
	if !ok || got != "12345" {
		t.Errorf("OptString = %q, %v", got, ok)
	}
	if _, ok := e.OptString("absent"); ok {
		t.Error("an option that was not supplied should read as absent")
	}
}
