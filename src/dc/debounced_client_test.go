package dc

import (
	"sync"
	"testing"
	"time"
)

func TestMessageEditDebouncer_CoalescesRapidEdits(t *testing.T) {
	var mu sync.Mutex
	calls := make([]Message, 0)

	execFn := func(channelID, messageID string, m Message) (*MessageRef, error) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, m)
		return &MessageRef{ID: messageID, ChannelID: channelID}, nil
	}

	debouncer := NewMessageEditDebouncer(50*time.Millisecond, execFn)

	// Send 5 rapid edits in a row
	for i := 1; i <= 5; i++ {
		_, err := debouncer.Edit("ch1", "msg1", Message{Content: "edit-" + string(rune('0'+i))})
		if err != nil {
			t.Fatalf("unexpected edit error: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Before window expires, no execution should have happened yet
	mu.Lock()
	if len(calls) != 0 {
		t.Fatalf("expected 0 calls before debounce settles, got %d", len(calls))
	}
	mu.Unlock()

	// Wait for debounce window to settle (50ms after last edit)
	time.Sleep(80 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 1 {
		t.Fatalf("expected exactly 1 coalesced call, got %d", len(calls))
	}
	if calls[0].Content != "edit-5" {
		t.Errorf("expected final content %q, got %q", "edit-5", calls[0].Content)
	}
}

func TestMessageEditDebouncer_IndependentMessages(t *testing.T) {
	var mu sync.Mutex
	calls := make(map[string]string)

	execFn := func(channelID, messageID string, m Message) (*MessageRef, error) {
		mu.Lock()
		defer mu.Unlock()
		calls[channelID+"/"+messageID] = m.Content
		return &MessageRef{ID: messageID, ChannelID: channelID}, nil
	}

	debouncer := NewMessageEditDebouncer(40*time.Millisecond, execFn)

	_, _ = debouncer.Edit("ch1", "msg1", Message{Content: "msg1-content"})
	_, _ = debouncer.Edit("ch1", "msg2", Message{Content: "msg2-content"})
	_, _ = debouncer.Edit("ch2", "msg1", Message{Content: "ch2-msg1-content"})

	time.Sleep(70 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 3 {
		t.Fatalf("expected 3 calls for 3 independent messages, got %d", len(calls))
	}
	if calls["ch1/msg1"] != "msg1-content" {
		t.Errorf("expected ch1/msg1 to be %q, got %q", "msg1-content", calls["ch1/msg1"])
	}
	if calls["ch1/msg2"] != "msg2-content" {
		t.Errorf("expected ch1/msg2 to be %q, got %q", "msg2-content", calls["ch1/msg2"])
	}
	if calls["ch2/msg1"] != "ch2-msg1-content" {
		t.Errorf("expected ch2/msg1 to be %q, got %q", "ch2-msg1-content", calls["ch2/msg1"])
	}
}

func TestMessageEditDebouncer_Cancel(t *testing.T) {
	var mu sync.Mutex
	calls := 0

	execFn := func(channelID, messageID string, m Message) (*MessageRef, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		return &MessageRef{ID: messageID, ChannelID: channelID}, nil
	}

	debouncer := NewMessageEditDebouncer(50*time.Millisecond, execFn)

	_, _ = debouncer.Edit("ch1", "msg1", Message{Content: "will be cancelled"})
	debouncer.Cancel("ch1", "msg1")

	time.Sleep(80 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if calls != 0 {
		t.Errorf("expected 0 calls after cancel, got %d", calls)
	}
}

func TestMessageEditDebouncer_Flush(t *testing.T) {
	var mu sync.Mutex
	calls := make(map[string]string)

	execFn := func(channelID, messageID string, m Message) (*MessageRef, error) {
		mu.Lock()
		defer mu.Unlock()
		calls[channelID+"/"+messageID] = m.Content
		return &MessageRef{ID: messageID, ChannelID: channelID}, nil
	}

	debouncer := NewMessageEditDebouncer(5*time.Second, execFn)

	_, _ = debouncer.Edit("ch1", "msg1", Message{Content: "flushed-1"})
	_, _ = debouncer.Edit("ch1", "msg2", Message{Content: "flushed-2"})

	// Immediate flush before the 5s window
	debouncer.Flush()

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 2 {
		t.Fatalf("expected 2 flushed calls, got %d", len(calls))
	}
	if calls["ch1/msg1"] != "flushed-1" || calls["ch1/msg2"] != "flushed-2" {
		t.Errorf("unexpected flushed content: %v", calls)
	}
}
