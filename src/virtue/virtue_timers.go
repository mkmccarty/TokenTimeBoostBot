package virtue

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

type virtueTimerRecord struct {
	ChannelID    string `json:"channel_id"`
	MessageID    string `json:"message_id"`
	TargetUserID string `json:"target_user_id"`
	EggIncID     string `json:"egg_inc_id"`
	SimulatedEgg int    `json:"simulated_egg"`
	TargetTE     uint64 `json:"target_te"`
	Compact      bool   `json:"compact"`
	AvatarURL    string `json:"avatar_url"`
	ActAsText    string `json:"act_as_text"`
	ExpiresAt    int64  `json:"expires_at"`
}

var (
	virtueClientFn func() dc.Client
	virtueTimersMu sync.Mutex
)

func virtueTimersPath() string {
	return filepath.Join("ttbb-data", "virtue_timers.json")
}

func loadVirtueTimersLocked() []virtueTimerRecord {
	data, err := os.ReadFile(virtueTimersPath())
	if err != nil {
		return nil
	}
	var records []virtueTimerRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil
	}
	return records
}

func saveVirtueTimersLocked(records []virtueTimerRecord) {
	_ = os.MkdirAll("ttbb-data", 0o755)
	data, err := json.Marshal(records)
	if err != nil {
		return
	}
	_ = os.WriteFile(virtueTimersPath(), data, 0o644)
}

// StartButtonCleanup loads any pending virtue button-removal timers from disk
// and schedules them to fire at their expiration time.
func StartButtonCleanup(clientFn func() dc.Client) {
	virtueTimersMu.Lock()
	virtueClientFn = clientFn
	records := loadVirtueTimersLocked()
	virtueTimersMu.Unlock()

	for _, rec := range records {
		delay := max(time.Until(time.Unix(rec.ExpiresAt, 0)), 0)
		r := rec
		time.AfterFunc(delay, func() {
			expireVirtueButtons(r)
		})
	}
}

func scheduleVirtueButtonCleanup(rec virtueTimerRecord) {
	virtueTimersMu.Lock()
	records := loadVirtueTimersLocked()
	records = append(records, rec)
	saveVirtueTimersLocked(records)
	virtueTimersMu.Unlock()

	delay := max(time.Until(time.Unix(rec.ExpiresAt, 0)), 0)
	time.AfterFunc(delay, func() {
		expireVirtueButtons(rec)
	})
}

func removeVirtueTimer(channelID, messageID string) {
	virtueTimersMu.Lock()
	defer virtueTimersMu.Unlock()
	records := loadVirtueTimersLocked()
	filtered := records[:0]
	for _, r := range records {
		if r.ChannelID == channelID && r.MessageID == messageID {
			continue
		}
		filtered = append(filtered, r)
	}
	saveVirtueTimersLocked(filtered)
}

func expireVirtueButtons(rec virtueTimerRecord) {
	removeVirtueTimer(rec.ChannelID, rec.MessageID)

	virtueTimersMu.Lock()
	clientFn := virtueClientFn
	virtueTimersMu.Unlock()
	if clientFn == nil {
		return
	}
	client := clientFn()
	if client == nil {
		return
	}

	msg, err := client.GetMessage(rec.ChannelID, rec.MessageID)
	if err != nil || msg == nil {
		if err != nil {
			log.Printf("virtue button cleanup GetMessage error (channel=%s message=%s): %v", rec.ChannelID, rec.MessageID, err)
		}
		return
	}

	kept := make([]dc.LayoutComponent, 0, len(msg.Components))
	for _, c := range msg.Components {
		if _, isRow := c.(dc.ActionRow); isRow {
			continue
		}
		kept = append(kept, c)
	}

	if _, err := client.EditMessage(rec.ChannelID, rec.MessageID, dc.Message{
		Components: kept,
	}); err != nil {
		log.Printf("virtue button cleanup EditMessage error (channel=%s message=%s): %v", rec.ChannelID, rec.MessageID, err)
	}
}
