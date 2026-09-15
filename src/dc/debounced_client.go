package dc

import (
	"sync"
	"time"
)

// DefaultEditDebounceWindow is the duration rapid edits to the same message are debounced.
const DefaultEditDebounceWindow = 500 * time.Millisecond

type pendingEdit struct {
	channelID string
	messageID string
	msg       Message
	timer     *time.Timer
}

// MessageEditDebouncer coalesces and delays rapid consecutive edits to the same
// message, executing only the latest payload when the debounce window settles.
type MessageEditDebouncer struct {
	mu        sync.Mutex
	window    time.Duration
	pending   map[string]*pendingEdit
	executeFn func(channelID, messageID string, m Message) (*MessageRef, error)
}

// NewMessageEditDebouncer creates a new debouncer with the given window and execution function.
func NewMessageEditDebouncer(window time.Duration, executeFn func(channelID, messageID string, m Message) (*MessageRef, error)) *MessageEditDebouncer {
	if window <= 0 {
		window = DefaultEditDebounceWindow
	}
	return &MessageEditDebouncer{
		window:    window,
		pending:   make(map[string]*pendingEdit),
		executeFn: executeFn,
	}
}

// Edit schedules or updates a debounced message edit for the given channelID and messageID.
func (d *MessageEditDebouncer) Edit(channelID, messageID string, m Message) (*MessageRef, error) {
	key := channelID + "/" + messageID

	d.mu.Lock()
	defer d.mu.Unlock()

	if entry, exists := d.pending[key]; exists {
		entry.msg = m
		if entry.timer != nil {
			entry.timer.Stop()
		}
		entry.timer = time.AfterFunc(d.window, func() {
			d.flushKey(key)
		})
	} else {
		entry := &pendingEdit{
			channelID: channelID,
			messageID: messageID,
			msg:       m,
		}
		entry.timer = time.AfterFunc(d.window, func() {
			d.flushKey(key)
		})
		d.pending[key] = entry
	}

	return &MessageRef{
		ID:        messageID,
		ChannelID: channelID,
	}, nil
}

// Cancel removes and stops any pending debounced edit for the given message.
func (d *MessageEditDebouncer) Cancel(channelID, messageID string) {
	key := channelID + "/" + messageID

	d.mu.Lock()
	defer d.mu.Unlock()

	if entry, exists := d.pending[key]; exists {
		if entry.timer != nil {
			entry.timer.Stop()
		}
		delete(d.pending, key)
	}
}

// flushKey executes a single pending edit for a specific message key.
func (d *MessageEditDebouncer) flushKey(key string) {
	d.mu.Lock()
	entry, exists := d.pending[key]
	if !exists {
		d.mu.Unlock()
		return
	}
	delete(d.pending, key)
	d.mu.Unlock()

	if entry != nil && d.executeFn != nil {
		_, _ = d.executeFn(entry.channelID, entry.messageID, entry.msg)
	}
}

// Flush immediately dispatches all currently pending message edits.
func (d *MessageEditDebouncer) Flush() {
	d.mu.Lock()
	entries := make([]*pendingEdit, 0, len(d.pending))
	for key, entry := range d.pending {
		if entry.timer != nil {
			entry.timer.Stop()
		}
		entries = append(entries, entry)
		delete(d.pending, key)
	}
	d.mu.Unlock()

	if d.executeFn != nil {
		for _, entry := range entries {
			_, _ = d.executeFn(entry.channelID, entry.messageID, entry.msg)
		}
	}
}
