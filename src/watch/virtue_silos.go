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
	QuietHoursSet        bool   `json:"quiet_hours_set,omitempty"`
	QuietStartTOD        int64  `json:"quiet_start_tod,omitempty"`
	QuietEndTOD          int64  `json:"quiet_end_tod,omitempty"`
}

const virtueSiloQuietLeadTime = 15 * time.Minute

func eggStandardLocation() *time.Location {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		return time.FixedZone("PST", -8*60*60)
	}
	return loc
}

func timeOfDaySeconds(t time.Time) int64 {
	return int64(t.Hour()*3600 + t.Minute()*60 + t.Second())
}

// findContainingQuietStart checks whether t falls inside the daily Egg Standard Time
// quiet window [startTOD, endTOD) and returns the start timestamp of that window.
func findContainingQuietStart(t time.Time, startTOD, endTOD int64) (time.Time, bool) {
	if startTOD < 0 || endTOD < 0 || startTOD == endTOD {
		return time.Time{}, false
	}
	loc := eggStandardLocation()
	est := t.In(loc)
	midnight := time.Date(est.Year(), est.Month(), est.Day(), 0, 0, 0, 0, loc)
	tTOD := timeOfDaySeconds(est)

	if startTOD < endTOD {
		if tTOD >= startTOD && tTOD < endTOD {
			return midnight.Add(time.Duration(startTOD) * time.Second), true
		}
		return time.Time{}, false
	}

	// Overnight window crossing 00:00 Egg Standard Time.
	if tTOD >= startTOD {
		return midnight.Add(time.Duration(startTOD) * time.Second), true
	}
	if tTOD < endTOD {
		return midnight.AddDate(0, 0, -1).Add(time.Duration(startTOD) * time.Second), true
	}
	return time.Time{}, false
}

func nextQuietWindow(now time.Time, startTOD, endTOD int64) (time.Time, time.Time) {
	loc := eggStandardLocation()
	est := now.In(loc)
	midnight := time.Date(est.Year(), est.Month(), est.Day(), 0, 0, 0, 0, loc)
	qStart := midnight.Add(time.Duration(startTOD) * time.Second)
	qEnd := midnight.Add(time.Duration(endTOD) * time.Second)
	if endTOD <= startTOD {
		qEnd = qEnd.AddDate(0, 0, 1)
	}
	if !est.Before(qEnd) {
		qStart = qStart.AddDate(0, 0, 1)
		qEnd = qEnd.AddDate(0, 0, 1)
	}
	return qStart, qEnd
}

// calculateVirtueSiloNotifyAt returns the scheduled notification time, shifting
// to 15 minutes before quiet hours start if the normal watch falls inside them.
func calculateVirtueSiloNotifyAt(siloExpiry time.Time, offsetSeconds int64, quietHoursSet bool, quietStartTOD, quietEndTOD int64) (notifyAt, normalNotifyAt time.Time, adjusted bool) {
	normalNotifyAt = siloExpiry.Add(-time.Duration(offsetSeconds) * time.Second)
	if !quietHoursSet || quietStartTOD == quietEndTOD {
		return normalNotifyAt, normalNotifyAt, false
	}
	if qStart, inQuiet := findContainingQuietStart(normalNotifyAt, quietStartTOD, quietEndTOD); inQuiet {
		return qStart.Add(-virtueSiloQuietLeadTime), normalNotifyAt, true
	}
	return normalNotifyAt, normalNotifyAt, false
}

var (
	virtueSiloMu          sync.Mutex
	virtueSiloTimers      = make(map[string]*time.Timer)
	virtueSiloClient      dc.Client
	virtueSiloWatchesFile = filepath.Join("ttbb-data", "virtue_silo_watches.json")
	fetchVirtueBackup     = ei.GetFirstContactFromAPINoCache
)

func virtueSiloWatchesPath() string {
	return virtueSiloWatchesFile
}

// siloWatchAccountLabel returns a " for **IGN**" suffix (or " for <@userID>"
// when no IGN is available for an alternate account) so multi-account users
// can tell which account triggered a silo watch notification.
func siloWatchAccountLabel(userID, notifyUserID string, candidateNames ...string) string {
	for _, raw := range candidateNames {
		if name := strings.TrimSpace(raw); name != "" {
			if normalized := strings.TrimSpace(ei.NormalizePlayerNameForDisplay(name)); normalized != "" {
				return fmt.Sprintf(" for **%s**", normalized)
			}
		}
	}
	if userID != "" {
		if ign := strings.TrimSpace(farmerstate.GetEggIncName(userID)); ign != "" {
			if normalized := strings.TrimSpace(ei.NormalizePlayerNameForDisplay(ign)); normalized != "" {
				return fmt.Sprintf(" for **%s**", normalized)
			}
		}
		if userID != notifyUserID {
			return fmt.Sprintf(" for <@%s>", userID)
		}
	}
	return ""
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
	notifyAt, _, _ := calculateVirtueSiloNotifyAt(siloExpiry, rec.OffsetSeconds, rec.QuietHoursSet, rec.QuietStartTOD, rec.QuietEndTOD)

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
	backup, _ := fetchVirtueBackup(encryptedID, rec.UserID, true)
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
	notifyAt, _, _ := calculateVirtueSiloNotifyAt(siloExpiry, rec.OffsetSeconds, rec.QuietHoursSet, rec.QuietStartTOD, rec.QuietEndTOD)

	if time.Now().Before(notifyAt) {
		rec.NextCheckAt = notifyAt.Unix()
		scheduleVirtueSiloTimer(*rec)
		return
	}

	// Within offset (or already expired): ping once per sync and keep the watch active
	lastStep := int64(farm.GetLastStepTime())
	if rec.LastNotifiedStepTime != lastStep {
		rec.LastNotifiedStepTime = lastStep
		accountLabel := siloWatchAccountLabel(rec.UserID, rec.NotifyUserID, backup.GetUserName())
		msgContent := fmt.Sprintf("%s Your virtue farm silos%s will run out <t:%d:R> (<t:%d:t>). Please check and sync your farm!",
			ei.GetBotEmojiMarkdown("silo"), accountLabel, siloExpiry.Unix(), siloExpiry.Unix())

		if err := sendWatchNotificationDM(client, rec.NotifyUserID, rec.ChannelID, msgContent); err != nil {
			log.Printf("virtue silo watch notify error (user=%s): %v", rec.NotifyUserID, err)
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
	if len(parts) >= 5 {
		startTOD, errS := strconv.ParseInt(parts[3], 10, 64)
		endTOD, errE := strconv.ParseInt(parts[4], 10, 64)
		if errS == nil && errE == nil && startTOD >= 0 && startTOD < 86400 && endTOD >= 0 && endTOD < 86400 && startTOD != endTOD {
			rec.QuietHoursSet = true
			rec.QuietStartTOD = startTOD
			rec.QuietEndTOD = endTOD
		}
	}
	if rec.QuietHoursSet {
		saveQuietOffsets(targetUserID, quietOffsetFromTOD(rec.QuietStartTOD), quietOffsetFromTOD(rec.QuietEndTOD))
	} else {
		saveQuietOffsets(targetUserID, "unset", "unset")
	}
	scheduleVirtueSiloTimer(rec)

	accountLabel := siloWatchAccountLabel(targetUserID, e.UserID(), "")
	confirmMsg := fmt.Sprintf("✅ **Confirmed:** %s Virtue silo watch%s enabled for **%s** before your silos run out.",
		ei.GetBotEmojiMarkdown("silo"), accountLabel, bottools.FmtDuration(time.Duration(offsetSec)*time.Second))
	if rec.QuietHoursSet {
		qStart, qEnd := nextQuietWindow(time.Now(), rec.QuietStartTOD, rec.QuietEndTOD)
		confirmMsg += fmt.Sprintf(" Quiet hours set to <t:%d:t>–<t:%d:t> (will notify 15m before quiet hours if a watch falls inside).",
			qStart.Unix(), qEnd.Unix())
	}

	kept := e.MessageComponentsWithoutActionRows()
	kept = append(kept, dc.TextDisplay{
		Content: confirmMsg,
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
		quietInfo := ""
		if existing.QuietHoursSet {
			qStart, qEnd := nextQuietWindow(time.Now(), existing.QuietStartTOD, existing.QuietEndTOD)
			quietInfo = fmt.Sprintf("\nQuiet hours: <t:%d:t> to <t:%d:t>.", qStart.Unix(), qEnd.Unix())
		}
		accountLabel := siloWatchAccountLabel(targetUserID, e.UserID(), "")
		_ = e.Respond(dc.Message{
			Ephemeral: true,
			Components: []dc.LayoutComponent{
				dc.TextDisplay{
					Content: fmt.Sprintf("%s You currently have an active virtue silo watch%s scheduled to check/notify <t:%d:R> (<t:%d:t>).%s Would you like to clear it?",
						ei.GetBotEmojiMarkdown("silo"), accountLabel, existing.NextCheckAt, existing.NextCheckAt, quietInfo),
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

	startVal, endVal := loadSavedQuietOffsets(targetUserID)
	_ = e.Respond(dc.Message{
		Ephemeral:  true,
		Components: buildVirtueSilosPrompt(targetUserID, startVal, endVal, time.Now()),
	})
}

// HandleVirtueSilosQuietSelect handles quiet start/end offset dropdown selections.
func HandleVirtueSilosQuietSelect(e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 5 {
		return
	}
	action := parts[0]
	targetUserID := parts[1]
	startVal := parts[2]
	endVal := parts[3]
	refUnix, err := strconv.ParseInt(parts[4], 10, 64)
	refTime := time.Now()
	if err == nil && refUnix > 0 {
		refTime = time.Unix(refUnix, 0)
	}

	if vals := e.Values(); len(vals) > 0 {
		switch action {
		case "watch-vsilos-qstart":
			startVal = vals[0]
		case "watch-vsilos-qend":
			endVal = vals[0]
		}
	}

	_ = e.Update(dc.Message{
		Components: buildVirtueSilosPrompt(targetUserID, startVal, endVal, refTime),
	})
}

func parseQuietOffset(val string) (int, bool) {
	if val == "" || val == "none" || val == "unset" {
		return 0, false
	}
	h, err := strconv.Atoi(val)
	if err != nil || h < 0 || h > 23 {
		return 0, false
	}
	return h, true
}

func quietOffsetFromTOD(tod int64) string {
	if tod < 0 || tod >= 86400 || tod%3600 != 0 {
		return "unset"
	}
	hour := int(tod / 3600)
	offset := (hour - virtueSiloEpochHour + 24) % 24
	return strconv.Itoa(offset)
}

func loadSavedQuietOffsets(userID string) (string, string) {
	startVal := farmerstate.GetMiscSettingString(userID, "watch_quiet_start")
	endVal := farmerstate.GetMiscSettingString(userID, "watch_quiet_end")
	if _, sok := parseQuietOffset(startVal); sok {
		if _, eok := parseQuietOffset(endVal); eok && startVal != endVal {
			return startVal, endVal
		}
	}
	return "unset", "unset"
}

func saveQuietOffsets(userID, startVal, endVal string) {
	if _, sok := parseQuietOffset(startVal); sok {
		if _, eok := parseQuietOffset(endVal); eok && startVal != endVal {
			farmerstate.SetMiscSettingString(userID, "watch_quiet_start", startVal)
			farmerstate.SetMiscSettingString(userID, "watch_quiet_end", endVal)
			return
		}
	}
	farmerstate.SetMiscSettingString(userID, "watch_quiet_start", "")
	farmerstate.SetMiscSettingString(userID, "watch_quiet_end", "")
}

func loadSavedWatchOffset(userID, key string, defaultSec int64, validOptions []dc.SelectOption) int64 {
	raw := farmerstate.GetMiscSettingString(userID, key)
	if raw == "" {
		return defaultSec
	}
	sec, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || sec <= 0 {
		return defaultSec
	}
	for _, opt := range validOptions {
		if opt.Value == raw {
			return sec
		}
	}
	return defaultSec
}

const virtueSiloEpochHour = 9 // Egg Standard Time +0 is 9:00 AM America/Los_Angeles

func eggStandardHour(offset int) int {
	return (virtueSiloEpochHour + offset) % 24
}

func resolveQuietWindowFromOffsets(startVal, endVal string, refTime time.Time) (qStart, qEnd time.Time, startTOD, endTOD int64, valid bool) {
	sh, sok := parseQuietOffset(startVal)
	eh, eok := parseQuietOffset(endVal)
	if !sok || !eok || sh == eh {
		return time.Time{}, time.Time{}, -1, -1, false
	}
	startTOD = int64(eggStandardHour(sh) * 3600)
	endTOD = int64(eggStandardHour(eh) * 3600)
	qStart, qEnd = nextQuietWindow(refTime, startTOD, endTOD)
	return qStart, qEnd, startTOD, endTOD, true
}

func nextEggStandardHour(refTime time.Time, offset int) time.Time {
	loc := eggStandardLocation()
	est := refTime.In(loc)
	t := time.Date(est.Year(), est.Month(), est.Day(), eggStandardHour(offset), 0, 0, 0, loc)
	if !est.Before(t) {
		t = t.AddDate(0, 0, 1)
	}
	return t
}

func buildQuietOffsetOptions(selectedVal string, refTime time.Time) []dc.SelectOption {
	loc := eggStandardLocation()
	est := refTime.In(loc)
	options := make([]dc.SelectOption, 0, 25)
	options = append(options, dc.SelectOption{
		Label:       "None",
		Value:       "none",
		Description: "No quiet hours",
		Default:     selectedVal == "none",
	})
	for h := 0; h <= 23; h++ {
		val := strconv.Itoa(h)
		t := time.Date(est.Year(), est.Month(), est.Day(), eggStandardHour(h), 0, 0, 0, loc)
		options = append(options, dc.SelectOption{
			Label:       fmt.Sprintf("+%d", h),
			Value:       val,
			Description: t.Format("3:04 PM MST"),
			Default:     selectedVal == val,
		})
	}
	return options
}

func buildVirtueSilosPrompt(targetUserID, startVal, endVal string, refTime time.Time) []dc.LayoutComponent {
	const offsetSec = 30 * 60

	qStart, qEnd, startTOD, endTOD, quietValid := resolveQuietWindowFromOffsets(startVal, endVal, refTime)

	quietText := "\n\n**Quiet Hours:** None (optional — select Start and End offsets below to avoid notifications during quiet hours)."
	if quietValid {
		quietText = fmt.Sprintf("\n\n**Quiet Hours:** <t:%d:t> (<t:%d:R>) to <t:%d:t> (<t:%d:R>)",
			qStart.Unix(), qStart.Unix(), qEnd.Unix(), qEnd.Unix())
	} else {
		sh, sok := parseQuietOffset(startVal)
		eh, eok := parseQuietOffset(endVal)
		switch {
		case sok && eok && sh == eh:
			quietText = "\n\n**Quiet Hours:** None (Start and End offsets are the same)."
		case sok && endVal == "unset":
			qs := nextEggStandardHour(refTime, sh)
			quietText = fmt.Sprintf("\n\n**Quiet Hours:** Start <t:%d:t> (<t:%d:R>) — select an End offset to set quiet hours.", qs.Unix(), qs.Unix())
		case eok && startVal == "unset":
			qe := nextEggStandardHour(refTime, eh)
			quietText = fmt.Sprintf("\n\n**Quiet Hours:** End <t:%d:t> (<t:%d:R>) — select a Start offset to set quiet hours.", qe.Unix(), qe.Unix())
		}
	}

	exampleText := ""
	if encryptedID := farmerstate.GetMiscSettingString(targetUserID, "encrypted_ei_id"); encryptedID != "" {
		if backup, _ := ei.GetFirstContactFromAPI(encryptedID, targetUserID, false); backup != nil && len(backup.GetFarms()) > 0 {
			farm := backup.GetFarms()[0]
			silosOwned := ei.GetEffectiveSilos(backup, farm.GetSilosOwned())
			siloMinutes := ei.GetSiloMinutes(silosOwned, backup.GetGame().GetEpicResearch())
			lastSaved := time.Unix(int64(farm.GetLastStepTime()), 0)
			siloExpiry := lastSaved.Add(time.Duration(siloMinutes) * time.Minute)
			notifyAt, normalNotifyAt, adjusted := calculateVirtueSiloNotifyAt(siloExpiry, offsetSec, quietValid, startTOD, endTOD)
			if adjusted {
				exampleText = fmt.Sprintf("\n\nBased on your last sync, your silos expire <t:%d:R> (<t:%d:t>). Since a normal **30m** watch (<t:%d:t>) falls within your quiet hours, we would set your silo watch 15m before quiet hours at <t:%d:t> (<t:%d:R>).",
					siloExpiry.Unix(), siloExpiry.Unix(), normalNotifyAt.Unix(), notifyAt.Unix(), notifyAt.Unix())
			} else {
				exampleText = fmt.Sprintf("\n\nBased on your last sync, your silos expire <t:%d:R> (<t:%d:t>), so you would be notified around <t:%d:t> (<t:%d:R>).",
					siloExpiry.Unix(), siloExpiry.Unix(), notifyAt.Unix(), notifyAt.Unix())
			}
		}
	}

	confirmCustomID := fmt.Sprintf("watch-vsilos-confirm#%s#%d#-1#-1", targetUserID, offsetSec)
	if quietValid {
		confirmCustomID = fmt.Sprintf("watch-vsilos-confirm#%s#%d#%d#%d", targetUserID, offsetSec, startTOD, endTOD)
	}

	minValues := 1
	return []dc.LayoutComponent{
		dc.TextDisplay{
			Content: "Monitoring your virtue farm silos requires periodically polling your Egg Inc game backup. Whenever you sync your farm (detected by other bot commands or when the timer checks your backup), the watch automatically extends to the new silo expiration time. Are you okay with scheduling this to notify you **30m** before your silos run out?" + exampleText + quietText,
		},
		dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.SelectMenu{
					CustomID:    fmt.Sprintf("watch-vsilos-qstart#%s#%s#%s#%d", targetUserID, startVal, endVal, refTime.Unix()),
					Placeholder: "Quiet Start Offset (Egg Standard Time)",
					MinValues:   &minValues,
					MaxValues:   1,
					Options:     buildQuietOffsetOptions(startVal, refTime),
				},
			},
		},
		dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.SelectMenu{
					CustomID:    fmt.Sprintf("watch-vsilos-qend#%s#%s#%s#%d", targetUserID, startVal, endVal, refTime.Unix()),
					Placeholder: "Quiet End Offset (Egg Standard Time)",
					MinValues:   &minValues,
					MaxValues:   1,
					Options:     buildQuietOffsetOptions(endVal, refTime),
				},
			},
		},
		dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{
					Label:    "Confirm",
					Style:    dc.ButtonSuccess,
					CustomID: confirmCustomID,
				},
				dc.Button{
					Label:    "Cancel",
					Style:    dc.ButtonSecondary,
					CustomID: fmt.Sprintf("watch-vsilos-cancel#%s", targetUserID),
				},
			},
		},
	}
}
