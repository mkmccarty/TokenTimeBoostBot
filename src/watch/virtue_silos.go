package watch

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

type virtueSiloWatch struct {
	UserID               string `json:"user_id"`
	NotifyUserID         string `json:"notify_user_id"`
	ChannelID            string `json:"channel_id"`
	OffsetSeconds        int64  `json:"offset_seconds"`
	NextCheckAt          int64  `json:"next_check_at"`
	LastNotifiedStepTime int64  `json:"last_notified_step_time,omitempty"`
}

var (
	virtueSiloMu     sync.Mutex
	virtueSiloTimers = make(map[string]*time.Timer)
	virtueSiloClient dc.Client
)

func virtueSiloWatchesPath() string {
	return filepath.Join("ttbb-data", "virtue_silo_watches.json")
}

func loadVirtueSiloWatchesLocked() []virtueSiloWatch {
	data, err := os.ReadFile(virtueSiloWatchesPath())
	if err != nil {
		return nil
	}
	var list []virtueSiloWatch
	if err := json.Unmarshal(data, &list); err != nil {
		return nil
	}
	return list
}

func saveVirtueSiloWatchesLocked(list []virtueSiloWatch) {
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(virtueSiloWatchesPath()), 0755)
	_ = os.WriteFile(virtueSiloWatchesPath(), data, 0644)
}

func upsertVirtueSiloWatch(rec virtueSiloWatch) {
	virtueSiloMu.Lock()
	defer virtueSiloMu.Unlock()

	list := loadVirtueSiloWatchesLocked()
	filtered := make([]virtueSiloWatch, 0, len(list)+1)
	for _, item := range list {
		if item.UserID != rec.UserID {
			filtered = append(filtered, item)
		}
	}
	filtered = append(filtered, rec)
	saveVirtueSiloWatchesLocked(filtered)
}

func removeVirtueSiloWatch(userID string) {
	virtueSiloMu.Lock()
	defer virtueSiloMu.Unlock()

	if t, ok := virtueSiloTimers[userID]; ok {
		t.Stop()
		delete(virtueSiloTimers, userID)
	}
	list := loadVirtueSiloWatchesLocked()
	filtered := make([]virtueSiloWatch, 0, len(list))
	for _, item := range list {
		if item.UserID != userID {
			filtered = append(filtered, item)
		}
	}
	saveVirtueSiloWatchesLocked(filtered)
}

func scheduleVirtueSiloTimer(rec virtueSiloWatch) {
	upsertVirtueSiloWatch(rec)

	virtueSiloMu.Lock()
	if old, ok := virtueSiloTimers[rec.UserID]; ok {
		old.Stop()
	}
	delay := max(time.Until(time.Unix(rec.NextCheckAt, 0)), time.Second)
	virtueSiloTimers[rec.UserID] = time.AfterFunc(delay, func() {
		checkVirtueSiloWatch(rec.UserID)
	})
	virtueSiloMu.Unlock()
}

func init() {
	ei.OnBackupFetched = UpdateVirtueSiloWatchFromBackup
}

// UpdateVirtueSiloWatchFromBackup pushes out an active virtue silo watch when
// another action fetches a fresh backup for the user.
func UpdateVirtueSiloWatchFromBackup(userID string, backup *ei.Backup) {
	if userID == "" || backup == nil || len(backup.GetFarms()) == 0 {
		return
	}
	virtueSiloMu.Lock()
	var rec *virtueSiloWatch
	for _, item := range loadVirtueSiloWatchesLocked() {
		if item.UserID == userID {
			c := item
			rec = &c
			break
		}
	}
	virtueSiloMu.Unlock()
	if rec == nil {
		return
	}

	farm := backup.GetFarms()[0]
	eggType := farm.GetEggType()
	if eggType < ei.Egg_CURIOSITY || eggType > ei.Egg_KINDNESS {
		removeVirtueSiloWatch(userID)
		return
	}

	silosOwned := ei.GetEffectiveSilos(backup, farm.GetSilosOwned())
	siloMinutes := ei.GetSiloMinutes(silosOwned, backup.GetGame().GetEpicResearch())
	lastSaved := time.Unix(int64(farm.GetLastStepTime()), 0)
	siloExpiry := lastSaved.Add(time.Duration(siloMinutes) * time.Minute)
	notifyAt := siloExpiry.Add(-time.Duration(rec.OffsetSeconds) * time.Second)

	if time.Now().Before(notifyAt) && notifyAt.Unix() > rec.NextCheckAt {
		rec.NextCheckAt = notifyAt.Unix()
		scheduleVirtueSiloTimer(*rec)
	}
}

// LoadVirtueSiloWatches restores persisted virtue silo watches on startup.
func LoadVirtueSiloWatches(client dc.Client) {
	virtueSiloMu.Lock()
	virtueSiloClient = client
	list := loadVirtueSiloWatchesLocked()
	virtueSiloMu.Unlock()

	for _, rec := range list {
		r := rec
		virtueSiloMu.Lock()
		if old, ok := virtueSiloTimers[r.UserID]; ok {
			old.Stop()
		}
		delay := max(time.Until(time.Unix(r.NextCheckAt, 0)), time.Second)
		virtueSiloTimers[r.UserID] = time.AfterFunc(delay, func() {
			checkVirtueSiloWatch(r.UserID)
		})
		virtueSiloMu.Unlock()
	}
}

func checkVirtueSiloWatch(userID string) {
	virtueSiloMu.Lock()
	client := virtueSiloClient
	var rec *virtueSiloWatch
	for _, item := range loadVirtueSiloWatchesLocked() {
		if item.UserID == userID {
			c := item
			rec = &c
			break
		}
	}
	virtueSiloMu.Unlock()

	if rec == nil || client == nil {
		return
	}

	encryptedID := farmerstate.GetMiscSettingString(rec.UserID, "encrypted_ei_id")
	if encryptedID == "" {
		removeVirtueSiloWatch(rec.UserID)
		return
	}
	backup, _ := ei.GetFirstContactFromAPINoCache(encryptedID, rec.UserID, true)
	if backup == nil || len(backup.GetFarms()) == 0 {
		// Retry in 5 minutes if API temporarily failed
		rec.NextCheckAt = time.Now().Add(5 * time.Minute).Unix()
		scheduleVirtueSiloTimer(*rec)
		return
	}

	farm := backup.GetFarms()[0]
	eggType := farm.GetEggType()
	if eggType < ei.Egg_CURIOSITY || eggType > ei.Egg_KINDNESS {
		// User is no longer on a virtue farm; stop polling
		removeVirtueSiloWatch(rec.UserID)
		return
	}

	silosOwned := ei.GetEffectiveSilos(backup, farm.GetSilosOwned())
	siloMinutes := ei.GetSiloMinutes(silosOwned, backup.GetGame().GetEpicResearch())
	lastSaved := time.Unix(int64(farm.GetLastStepTime()), 0)
	siloExpiry := lastSaved.Add(time.Duration(siloMinutes) * time.Minute)
	notifyAt := siloExpiry.Add(-time.Duration(rec.OffsetSeconds) * time.Second)

	if time.Now().Before(notifyAt) {
		rec.NextCheckAt = notifyAt.Unix()
		scheduleVirtueSiloTimer(*rec)
		return
	}

	// Within offset (or already expired): ping once per sync and keep the watch active
	lastStep := int64(farm.GetLastStepTime())
	if rec.LastNotifiedStepTime != lastStep {
		rec.LastNotifiedStepTime = lastStep
		msgContent := fmt.Sprintf("%s Your virtue farm silos will run out <t:%d:R> (<t:%d:t>). Please check and sync your farm!",
			ei.GetBotEmojiMarkdown("silo"), siloExpiry.Unix(), siloExpiry.Unix())

		sent := false
		if ch, err := client.CreateUserChannel(rec.NotifyUserID); err == nil && ch != nil {
			if _, err := client.SendMessage(ch.ID, dc.Message{Content: msgContent}); err == nil {
				sent = true
			}
		}
		if !sent && rec.ChannelID != "" {
			if _, err := client.SendMessage(rec.ChannelID, dc.Message{
				Content: fmt.Sprintf("<@%s> %s", rec.NotifyUserID, msgContent),
			}); err != nil {
				log.Printf("virtue silo watch notify error (user=%s): %v", rec.NotifyUserID, err)
			}
		}
	}

	// Renew timer so the watch stays active and extends when the player syncs
	rec.NextCheckAt = time.Now().Add(time.Duration(rec.OffsetSeconds) * time.Second).Unix()
	scheduleVirtueSiloTimer(*rec)
}

// HandleVirtueSilosConfirm handles the confirmation button for /watch virtue-silos.
func HandleVirtueSilosConfirm(client dc.Client, e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 3 {
		return
	}
	targetUserID := parts[1]
	offsetSec, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || offsetSec <= 0 {
		offsetSec = 3600
	}

	virtueSiloMu.Lock()
	virtueSiloClient = client
	virtueSiloMu.Unlock()

	rec := virtueSiloWatch{
		UserID:        targetUserID,
		NotifyUserID:  e.UserID(),
		ChannelID:     e.ChannelID(),
		OffsetSeconds: offsetSec,
		NextCheckAt:   time.Now().Add(time.Second).Unix(),
	}
	scheduleVirtueSiloTimer(rec)

	kept := e.MessageComponentsWithoutActionRows()
	kept = append(kept, dc.TextDisplay{
		Content: fmt.Sprintf("✅ **Confirmed:** %s Virtue silo watch enabled for **%s** before your silos run out.",
			ei.GetBotEmojiMarkdown("silo"), bottools.FmtDuration(time.Duration(offsetSec)*time.Second)),
	})
	_ = e.Update(dc.Message{
		Components: kept,
	})
}

// HandleVirtueSilosCancel handles the cancel button for /watch virtue-silos.
func HandleVirtueSilosCancel(e *dc.ComponentEvent) {
	kept := e.MessageComponentsWithoutActionRows()
	kept = append(kept, dc.TextDisplay{
		Content: "❌ **Cancelled:** Virtue silo watch was not enabled.",
	})
	_ = e.Update(dc.Message{
		Components: kept,
	})
}

// HasVirtueSiloWatch reports whether userID currently has an active virtue silo watch.
func HasVirtueSiloWatch(userID string) bool {
	virtueSiloMu.Lock()
	defer virtueSiloMu.Unlock()
	for _, item := range loadVirtueSiloWatchesLocked() {
		if item.UserID == userID {
			return true
		}
	}
	return false
}

// HandleVirtueSilosClear handles the clear confirmation button for /virtue Silo Watch.
func HandleVirtueSilosClear(e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 2 {
		return
	}
	targetUserID := parts[1]
	removeVirtueSiloWatch(targetUserID)

	kept := e.MessageComponentsWithoutActionRows()
	kept = append(kept, dc.TextDisplay{
		Content: fmt.Sprintf("✅ **Cleared:** %s Virtue silo watch has been removed.", ei.GetBotEmojiMarkdown("silo")),
	})
	_ = e.Update(dc.Message{
		Components: kept,
	})
}

// HandleVirtueSilosToggle handles the Silo Watch button on /virtue.
func HandleVirtueSilosToggle(e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 2 {
		return
	}
	targetUserID := parts[1]

	var existing *virtueSiloWatch
	virtueSiloMu.Lock()
	for _, item := range loadVirtueSiloWatchesLocked() {
		if item.UserID == targetUserID {
			c := item
			existing = &c
			break
		}
	}
	virtueSiloMu.Unlock()

	if existing != nil {
		_ = e.Respond(dc.Message{
			Ephemeral: true,
			Components: []dc.LayoutComponent{
				dc.TextDisplay{
					Content: fmt.Sprintf("%s You currently have an active virtue silo watch scheduled to check/notify <t:%d:R> (<t:%d:t>). Would you like to clear it?",
						ei.GetBotEmojiMarkdown("silo"), existing.NextCheckAt, existing.NextCheckAt),
				},
				dc.ActionRow{
					Components: []dc.InteractiveComponent{
						dc.Button{
							Label:    "Clear Watch",
							Style:    dc.ButtonDanger,
							CustomID: fmt.Sprintf("watch-vsilos-clear#%s", targetUserID),
						},
						dc.Button{
							Label:    "Cancel",
							Style:    dc.ButtonSecondary,
							CustomID: fmt.Sprintf("watch-vsilos-cancel#%s", targetUserID),
						},
					},
				},
			},
		})
		return
	}

	const offsetSec = 30 * 60
	exampleText := ""
	if encryptedID := farmerstate.GetMiscSettingString(targetUserID, "encrypted_ei_id"); encryptedID != "" {
		if backup, _ := ei.GetFirstContactFromAPI(encryptedID, targetUserID, false); backup != nil && len(backup.GetFarms()) > 0 {
			farm := backup.GetFarms()[0]
			silosOwned := ei.GetEffectiveSilos(backup, farm.GetSilosOwned())
			siloMinutes := ei.GetSiloMinutes(silosOwned, backup.GetGame().GetEpicResearch())
			lastSaved := time.Unix(int64(farm.GetLastStepTime()), 0)
			siloExpiry := lastSaved.Add(time.Duration(siloMinutes) * time.Minute)
			notifyAt := siloExpiry.Add(-time.Duration(offsetSec) * time.Second)
			exampleText = fmt.Sprintf("\n\nBased on your last sync, your silos expire <t:%d:R> (<t:%d:t>), so you would be notified around <t:%d:t> (<t:%d:R>).",
				siloExpiry.Unix(), siloExpiry.Unix(), notifyAt.Unix(), notifyAt.Unix())
		}
	}

	_ = e.Respond(dc.Message{
		Ephemeral: true,
		Components: []dc.LayoutComponent{
			dc.TextDisplay{
				Content: "Monitoring your virtue farm silos requires periodically polling your Egg Inc game backup. Whenever you sync your farm (detected by other bot commands or when the timer checks your backup), the watch automatically extends to the new silo expiration time. Are you okay with scheduling this to notify you **30m** before your silos run out?" + exampleText,
			},
			dc.ActionRow{
				Components: []dc.InteractiveComponent{
					dc.Button{
						Label:    "Confirm",
						Style:    dc.ButtonSuccess,
						CustomID: fmt.Sprintf("watch-vsilos-confirm#%s#%d", targetUserID, offsetSec),
					},
					dc.Button{
						Label:    "Cancel",
						Style:    dc.ButtonSecondary,
						CustomID: fmt.Sprintf("watch-vsilos-cancel#%s", targetUserID),
					},
				},
			},
		},
	})
}
