package watch

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
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

type contractCompletionWatch struct {
	UserID        string `json:"user_id"`
	NotifyUserID  string `json:"notify_user_id"`
	ChannelID     string `json:"channel_id"`
	ContractID    string `json:"contract_id"`
	CoopID        string `json:"coop_id"`
	OffsetSeconds int64  `json:"offset_seconds"`
	NextCheckAt   int64  `json:"next_check_at"`
	QuietHoursSet bool   `json:"quiet_hours_set,omitempty"`
	QuietStartTOD int64  `json:"quiet_start_tod,omitempty"`
	QuietEndTOD   int64  `json:"quiet_end_tod,omitempty"`
}

type contractCompletionPromptState struct {
	UserID       string
	NotifyUserID string
	ContractID   string
	CoopID       string
	OffsetSec    int64
	StartVal     string
	EndVal       string
	RefTime      time.Time
	EstEndTime   time.Time
	CreatedAt    time.Time
}

const (
	defaultContractCompletionOffsetSec = 30 * 60
	contractCompletionToleranceSec     = 60
	contractCompletionPromptTTL        = 30 * time.Minute
)

var (
	contractCompletionMu          sync.Mutex
	contractCompletionTimers      = make(map[string]*time.Timer)
	contractCompletionClient      dc.Client
	contractCompletionWatchesFile = filepath.Join("ttbb-data", "contract_completion_watches.json")

	contractCompletionPromptMu sync.Mutex
	contractCompletionPrompts  = make(map[string]*contractCompletionPromptState)
)

func init() {
	ei.AddCoopStatusCallback(UpdateContractCompletionWatchesFromCoopStatus)
}

func contractCompletionKey(userID, contractID, coopID string) string {
	return fmt.Sprintf("%s#%s#%s", userID, strings.ToLower(contractID), strings.ToLower(coopID))
}

func loadContractCompletionWatchesLocked() []contractCompletionWatch {
	data, err := os.ReadFile(contractCompletionWatchesFile)
	if err != nil {
		return nil
	}
	var list []contractCompletionWatch
	if err := json.Unmarshal(data, &list); err != nil {
		return nil
	}
	return list
}

func saveContractCompletionWatchesLocked(list []contractCompletionWatch) {
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(contractCompletionWatchesFile), 0755)
	_ = os.WriteFile(contractCompletionWatchesFile, data, 0644)
}

func upsertContractCompletionWatch(rec contractCompletionWatch) {
	contractCompletionMu.Lock()
	defer contractCompletionMu.Unlock()

	key := contractCompletionKey(rec.UserID, rec.ContractID, rec.CoopID)
	list := loadContractCompletionWatchesLocked()
	filtered := make([]contractCompletionWatch, 0, len(list)+1)
	for _, item := range list {
		if contractCompletionKey(item.UserID, item.ContractID, item.CoopID) != key {
			filtered = append(filtered, item)
		}
	}
	filtered = append(filtered, rec)
	saveContractCompletionWatchesLocked(filtered)
}

func removeContractCompletionWatch(userID, contractID, coopID string) {
	contractCompletionMu.Lock()
	defer contractCompletionMu.Unlock()

	key := contractCompletionKey(userID, contractID, coopID)
	if t, ok := contractCompletionTimers[key]; ok {
		t.Stop()
		delete(contractCompletionTimers, key)
	}
	list := loadContractCompletionWatchesLocked()
	filtered := make([]contractCompletionWatch, 0, len(list))
	for _, item := range list {
		if contractCompletionKey(item.UserID, item.ContractID, item.CoopID) != key {
			filtered = append(filtered, item)
		}
	}
	saveContractCompletionWatchesLocked(filtered)
}

func removeContractCompletionWatchesForCoop(contractID, coopID string) {
	contractCompletionMu.Lock()
	defer contractCompletionMu.Unlock()

	list := loadContractCompletionWatchesLocked()
	if len(list) == 0 {
		return
	}
	filtered := make([]contractCompletionWatch, 0, len(list))
	changed := false
	for _, item := range list {
		if strings.EqualFold(item.ContractID, contractID) && strings.EqualFold(item.CoopID, coopID) {
			key := contractCompletionKey(item.UserID, item.ContractID, item.CoopID)
			if t, ok := contractCompletionTimers[key]; ok {
				t.Stop()
				delete(contractCompletionTimers, key)
			}
			changed = true
			continue
		}
		filtered = append(filtered, item)
	}
	if changed {
		saveContractCompletionWatchesLocked(filtered)
	}
}

func scheduleContractCompletionTimer(rec contractCompletionWatch) {
	upsertContractCompletionWatch(rec)

	key := contractCompletionKey(rec.UserID, rec.ContractID, rec.CoopID)
	contractCompletionMu.Lock()
	if old, ok := contractCompletionTimers[key]; ok {
		old.Stop()
	}
	delay := max(time.Until(time.Unix(rec.NextCheckAt, 0)), time.Second)
	contractCompletionTimers[key] = time.AfterFunc(delay, func() {
		checkContractCompletionWatch(key)
	})
	contractCompletionMu.Unlock()
}

// LoadContractCompletionWatches restores persisted contract completion watches on startup.
func LoadContractCompletionWatches(client dc.Client) {
	contractCompletionMu.Lock()
	contractCompletionClient = client
	list := loadContractCompletionWatchesLocked()
	contractCompletionMu.Unlock()

	for _, rec := range list {
		r := rec
		key := contractCompletionKey(r.UserID, r.ContractID, r.CoopID)
		contractCompletionMu.Lock()
		if old, ok := contractCompletionTimers[key]; ok {
			old.Stop()
		}
		delay := max(time.Until(time.Unix(r.NextCheckAt, 0)), time.Second)
		contractCompletionTimers[key] = time.AfterFunc(delay, func() {
			checkContractCompletionWatch(key)
		})
		contractCompletionMu.Unlock()
	}
}

// UpdateContractCompletionWatchesFromCoopStatus updates or cancels active contract
// completion watches whenever any command fetches a coop_status response.
func UpdateContractCompletionWatchesFromCoopStatus(contractID, coopID string, coopStatus *ei.ContractCoopStatusResponse) {
	if contractID == "" || coopID == "" || coopStatus == nil {
		return
	}
	if isCoopCompleted(coopStatus) {
		removeContractCompletionWatchesForCoop(contractID, coopID)
		return
	}

	contractCompletionMu.Lock()
	list := loadContractCompletionWatchesLocked()
	var matching []contractCompletionWatch
	for _, item := range list {
		if strings.EqualFold(item.ContractID, contractID) && strings.EqualFold(item.CoopID, coopID) {
			matching = append(matching, item)
		}
	}
	contractCompletionMu.Unlock()

	if len(matching) == 0 {
		return
	}

	now := time.Now()
	estEndTime := estimateCoopCompletionTime(contractID, coopStatus, now)
	if estEndTime.IsZero() {
		return
	}

	for _, rec := range matching {
		notifyAt, _, _ := calculateVirtueSiloNotifyAt(estEndTime, rec.OffsetSeconds, rec.QuietHoursSet, rec.QuietStartTOD, rec.QuietEndTOD)
		if now.Before(notifyAt) && math.Abs(float64(notifyAt.Unix()-rec.NextCheckAt)) > contractCompletionToleranceSec {
			rec.NextCheckAt = notifyAt.Unix()
			scheduleContractCompletionTimer(rec)
		}
	}
}

func sendContractCompletionDM(client dc.Client, rec contractCompletionWatch, msgContent string) {
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
			log.Printf("contract completion watch notify error (%s/%s): %v", rec.ContractID, rec.CoopID, err)
		}
	}
}

func checkContractCompletionWatch(key string) {
	contractCompletionMu.Lock()
	client := contractCompletionClient
	var rec *contractCompletionWatch
	for _, item := range loadContractCompletionWatchesLocked() {
		if contractCompletionKey(item.UserID, item.ContractID, item.CoopID) == key {
			c := item
			rec = &c
			break
		}
	}
	contractCompletionMu.Unlock()

	if rec == nil || client == nil {
		return
	}

	eiID := farmerstate.GetMiscSettingString(rec.UserID, "encrypted_ei_id")
	coopStatus, nowTime, _, err := fetchContractCoopStatusUncached(rec.ContractID, rec.CoopID, eiID)
	if err != nil || coopStatus == nil {
		rec.NextCheckAt = time.Now().Add(5 * time.Minute).Unix()
		scheduleContractCompletionTimer(*rec)
		return
	}

	if coopStatus.GetResponseStatus() != ei.ContractCoopStatusResponse_NO_ERROR {
		switch coopStatus.GetResponseStatus() {
		case ei.ContractCoopStatusResponse_COOP_NOT_FOUND, ei.ContractCoopStatusResponse_CONTRACT_NOT_FOUND:
			removeContractCompletionWatchesForCoop(rec.ContractID, rec.CoopID)
		default:
			rec.NextCheckAt = time.Now().Add(5 * time.Minute).Unix()
			scheduleContractCompletionTimer(*rec)
		}
		return
	}

	if isCoopCompleted(coopStatus) {
		msgContent := fmt.Sprintf("🏁 Your contract **%s/%s** has completed!", rec.ContractID, rec.CoopID)
		sendContractCompletionDM(client, *rec, msgContent)
		removeContractCompletionWatch(rec.UserID, rec.ContractID, rec.CoopID)
		return
	}

	if nowTime.IsZero() {
		nowTime = time.Now()
	}

	estEndTime := estimateCoopCompletionTime(rec.ContractID, coopStatus, nowTime)
	if estEndTime.IsZero() {
		rec.NextCheckAt = time.Now().Add(5 * time.Minute).Unix()
		scheduleContractCompletionTimer(*rec)
		return
	}

	notifyAt, _, _ := calculateVirtueSiloNotifyAt(estEndTime, rec.OffsetSeconds, rec.QuietHoursSet, rec.QuietStartTOD, rec.QuietEndTOD)
	if time.Now().Before(notifyAt) {
		rec.NextCheckAt = notifyAt.Unix()
		scheduleContractCompletionTimer(*rec)
		return
	}

	msgContent := fmt.Sprintf("🏁 Your contract **%s/%s** is estimated to complete <t:%d:R> (<t:%d:t>).",
		rec.ContractID, rec.CoopID, estEndTime.Unix(), estEndTime.Unix())
	sendContractCompletionDM(client, *rec, msgContent)
	removeContractCompletionWatch(rec.UserID, rec.ContractID, rec.CoopID)
}

func saveContractCompletionPrompt(state *contractCompletionPromptState) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)

	now := time.Now()
	state.CreatedAt = now

	contractCompletionPromptMu.Lock()
	for k, v := range contractCompletionPrompts {
		if now.Sub(v.CreatedAt) > contractCompletionPromptTTL {
			delete(contractCompletionPrompts, k)
		}
	}
	contractCompletionPrompts[token] = state
	contractCompletionPromptMu.Unlock()
	return token
}

func getContractCompletionPrompt(token string) (*contractCompletionPromptState, bool) {
	contractCompletionPromptMu.Lock()
	defer contractCompletionPromptMu.Unlock()
	st, ok := contractCompletionPrompts[token]
	if !ok {
		return nil, false
	}
	if time.Since(st.CreatedAt) > contractCompletionPromptTTL {
		delete(contractCompletionPrompts, token)
		return nil, false
	}
	return st, true
}

func deleteContractCompletionPrompt(token string) {
	contractCompletionPromptMu.Lock()
	delete(contractCompletionPrompts, token)
	contractCompletionPromptMu.Unlock()
}

func buildContractCompletionOffsetOptions(selectedSec int64) []dc.SelectOption {
	intervals := []struct {
		label string
		sec   int64
	}{
		{"5 minutes", 5 * 60},
		{"10 minutes", 10 * 60},
		{"15 minutes", 15 * 60},
		{"30 minutes", 30 * 60},
		{"1 hour", 60 * 60},
	}
	opts := make([]dc.SelectOption, 0, len(intervals))
	for _, iv := range intervals {
		opts = append(opts, dc.SelectOption{
			Label:       iv.label,
			Value:       strconv.FormatInt(iv.sec, 10),
			Description: fmt.Sprintf("Notify %s before contract completion", bottools.FmtDuration(time.Duration(iv.sec)*time.Second)),
			Default:     selectedSec == iv.sec,
		})
	}
	return opts
}

func buildContractCompletionPrompt(token string, st *contractCompletionPromptState) []dc.LayoutComponent {
	offsetSec := st.OffsetSec
	if offsetSec <= 0 {
		offsetSec = defaultContractCompletionOffsetSec
	}
	offsetLabel := bottools.FmtDuration(time.Duration(offsetSec) * time.Second)

	qStart, qEnd, startTOD, endTOD, quietValid := resolveQuietWindowFromOffsets(st.StartVal, st.EndVal, st.RefTime)

	quietText := "\n\n**Quiet Hours:** None (optional — select Start and End offsets below to avoid notifications during quiet hours)."
	if quietValid {
		quietText = fmt.Sprintf("\n\n**Quiet Hours:** <t:%d:t> (<t:%d:R>) to <t:%d:t> (<t:%d:R>)",
			qStart.Unix(), qStart.Unix(), qEnd.Unix(), qEnd.Unix())
	} else {
		sh, sok := parseQuietOffset(st.StartVal)
		eh, eok := parseQuietOffset(st.EndVal)
		switch {
		case sok && eok && sh == eh:
			quietText = "\n\n**Quiet Hours:** None (Start and End offsets are the same)."
		case sok && st.EndVal == "unset":
			qs := nextEggStandardHour(st.RefTime, sh)
			quietText = fmt.Sprintf("\n\n**Quiet Hours:** Start <t:%d:t> (<t:%d:R>) — select an End offset to set quiet hours.", qs.Unix(), qs.Unix())
		case eok && st.StartVal == "unset":
			qe := nextEggStandardHour(st.RefTime, eh)
			quietText = fmt.Sprintf("\n\n**Quiet Hours:** End <t:%d:t> (<t:%d:R>) — select a Start offset to set quiet hours.", qe.Unix(), qe.Unix())
		}
	}

	exampleText := ""
	if !st.EstEndTime.IsZero() {
		notifyAt, normalNotifyAt, adjusted := calculateVirtueSiloNotifyAt(st.EstEndTime, offsetSec, quietValid, startTOD, endTOD)
		if adjusted {
			exampleText = fmt.Sprintf("\n\nBased on current coop status, **%s/%s** is estimated to complete <t:%d:R> (<t:%d:t>). Since a normal **%s** watch (<t:%d:t>) falls within your quiet hours, we would notify you 15m before quiet hours at <t:%d:t> (<t:%d:R>).",
				st.ContractID, st.CoopID, st.EstEndTime.Unix(), st.EstEndTime.Unix(), offsetLabel, normalNotifyAt.Unix(), notifyAt.Unix(), notifyAt.Unix())
		} else {
			exampleText = fmt.Sprintf("\n\nBased on current coop status, **%s/%s** is estimated to complete <t:%d:R> (<t:%d:t>), so you would be notified around <t:%d:t> (<t:%d:R>).",
				st.ContractID, st.CoopID, st.EstEndTime.Unix(), st.EstEndTime.Unix(), notifyAt.Unix(), notifyAt.Unix())
		}
	}

	minValues := 1
	return []dc.LayoutComponent{
		dc.TextDisplay{
			Content: fmt.Sprintf("Monitoring **%s/%s** completion checks the coop status periodically and adjusts if players boost or sync, then DMs you **%s** before completion. Select your notification offset and optional quiet hours below:",
				st.ContractID, st.CoopID, offsetLabel) + exampleText + quietText,
		},
		dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.SelectMenu{
					CustomID:    fmt.Sprintf("watch-ccomp-offset#%s", token),
					Placeholder: "Notification Offset",
					MinValues:   &minValues,
					MaxValues:   1,
					Options:     buildContractCompletionOffsetOptions(offsetSec),
				},
			},
		},
		dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.SelectMenu{
					CustomID:    fmt.Sprintf("watch-ccomp-qstart#%s", token),
					Placeholder: "Quiet Start Offset (Egg Standard Time)",
					MinValues:   &minValues,
					MaxValues:   1,
					Options:     buildQuietOffsetOptions(st.StartVal, st.RefTime),
				},
			},
		},
		dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.SelectMenu{
					CustomID:    fmt.Sprintf("watch-ccomp-qend#%s", token),
					Placeholder: "Quiet End Offset (Egg Standard Time)",
					MinValues:   &minValues,
					MaxValues:   1,
					Options:     buildQuietOffsetOptions(st.EndVal, st.RefTime),
				},
			},
		},
		dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{
					Label:    "Confirm",
					Style:    dc.ButtonSuccess,
					CustomID: fmt.Sprintf("watch-ccomp-confirm#%s", token),
				},
				dc.Button{
					Label:    "Cancel",
					Style:    dc.ButtonSecondary,
					CustomID: fmt.Sprintf("watch-ccomp-cancel#%s", token),
				},
			},
		},
	}
}

// HandleContractCompletionToggle handles the Completion Watch button on /cs-estimate.
func HandleContractCompletionToggle(e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 3 {
		return
	}
	contractID := parts[1]
	coopID := parts[2]
	targetUserID := farmerstate.GetEffectiveUserID(e.UserID(), e.ChannelID())

	key := contractCompletionKey(targetUserID, contractID, coopID)
	var existing *contractCompletionWatch
	contractCompletionMu.Lock()
	for _, item := range loadContractCompletionWatchesLocked() {
		if contractCompletionKey(item.UserID, item.ContractID, item.CoopID) == key {
			c := item
			existing = &c
			break
		}
	}
	contractCompletionMu.Unlock()

	if existing != nil {
		quietInfo := ""
		if existing.QuietHoursSet {
			qStart, qEnd := nextQuietWindow(time.Now(), existing.QuietStartTOD, existing.QuietEndTOD)
			quietInfo = fmt.Sprintf("\nQuiet hours: <t:%d:t> to <t:%d:t>.", qStart.Unix(), qEnd.Unix())
		}
		_ = e.Respond(dc.Message{
			Ephemeral: true,
			Components: []dc.LayoutComponent{
				dc.TextDisplay{
					Content: fmt.Sprintf("🏁 You currently have an active completion watch for **%s/%s** (**%s** before completion), scheduled to check/notify <t:%d:R> (<t:%d:t>).%s Would you like to clear it?",
						contractID, coopID, bottools.FmtDuration(time.Duration(existing.OffsetSeconds)*time.Second), existing.NextCheckAt, existing.NextCheckAt, quietInfo),
				},
				dc.ActionRow{
					Components: []dc.InteractiveComponent{
						dc.Button{
							Label:    "Clear Watch",
							Style:    dc.ButtonDanger,
							CustomID: fmt.Sprintf("watch-ccomp-clear#%s#%s", contractID, coopID),
						},
						dc.Button{
							Label:    "Cancel",
							Style:    dc.ButtonSecondary,
							CustomID: "watch-ccomp-cancel#clear",
						},
					},
				},
			},
		})
		return
	}

	_ = e.Respond(dc.Message{
		Ephemeral: true,
		Components: []dc.LayoutComponent{
			dc.TextDisplay{
				Content: fmt.Sprintf("🏁 Checking coop status for **%s/%s**...", contractID, coopID),
			},
		},
	})

	eiID := farmerstate.GetMiscSettingString(targetUserID, "encrypted_ei_id")
	coopStatus, nowTime, _, err := fetchContractCoopStatus(contractID, coopID, eiID)
	if err != nil || coopStatus == nil || coopStatus.GetResponseStatus() != ei.ContractCoopStatusResponse_NO_ERROR {
		_ = e.EditResponse(dc.Message{
			Ephemeral: true,
			Components: []dc.LayoutComponent{
				dc.TextDisplay{
					Content: fmt.Sprintf("Unable to fetch coop status for **%s/%s**. Please try again shortly.", contractID, coopID),
				},
			},
		})
		return
	}

	if isCoopCompleted(coopStatus) {
		removeContractCompletionWatchesForCoop(contractID, coopID)
		_ = e.EditResponse(dc.Message{
			Ephemeral: true,
			Components: []dc.LayoutComponent{
				dc.TextDisplay{
					Content: fmt.Sprintf("Contract **%s/%s** is already completed, so no Completion Watch is needed.", contractID, coopID),
				},
			},
		})
		return
	}

	if nowTime.IsZero() {
		nowTime = time.Now()
	}

	estEndTime := estimateCoopCompletionTime(contractID, coopStatus, nowTime)
	if estEndTime.IsZero() {
		_ = e.EditResponse(dc.Message{
			Ephemeral: true,
			Components: []dc.LayoutComponent{
				dc.TextDisplay{
					Content: fmt.Sprintf("Unable to estimate completion time for **%s/%s** yet. Please try again once production has started.", contractID, coopID),
				},
			},
		})
		return
	}

	state := &contractCompletionPromptState{
		UserID:       targetUserID,
		NotifyUserID: e.UserID(),
		ContractID:   contractID,
		CoopID:       coopID,
		OffsetSec:    defaultContractCompletionOffsetSec,
		StartVal:     "unset",
		EndVal:       "unset",
		RefTime:      nowTime,
		EstEndTime:   estEndTime,
	}
	token := saveContractCompletionPrompt(state)

	_ = e.EditResponse(dc.Message{
		Ephemeral:  true,
		Components: buildContractCompletionPrompt(token, state),
	})
}

// HandleContractCompletionSelect handles offset and quiet hours dropdown selections for contract Completion Watch.
func HandleContractCompletionSelect(e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 2 {
		return
	}
	action := parts[0]
	token := parts[1]

	st, ok := getContractCompletionPrompt(token)
	if !ok {
		_ = e.Update(dc.Message{
			Components: []dc.LayoutComponent{
				dc.TextDisplay{Content: "This Completion Watch prompt has expired. Please click **Completion Watch** again."},
			},
		})
		return
	}

	if vals := e.Values(); len(vals) > 0 {
		switch action {
		case "watch-ccomp-offset":
			if sec, err := strconv.ParseInt(vals[0], 10, 64); err == nil && sec > 0 {
				st.OffsetSec = sec
			}
		case "watch-ccomp-qstart":
			st.StartVal = vals[0]
		case "watch-ccomp-qend":
			st.EndVal = vals[0]
		}
	}

	_ = e.Update(dc.Message{
		Components: buildContractCompletionPrompt(token, st),
	})
}

// HandleContractCompletionConfirm handles the Confirm button for contract Completion Watch.
func HandleContractCompletionConfirm(client dc.Client, e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 2 {
		return
	}
	token := parts[1]

	st, ok := getContractCompletionPrompt(token)
	if !ok {
		_ = e.Update(dc.Message{
			Components: []dc.LayoutComponent{
				dc.TextDisplay{Content: "This Completion Watch prompt has expired. Please click **Completion Watch** again."},
			},
		})
		return
	}
	deleteContractCompletionPrompt(token)

	contractCompletionMu.Lock()
	contractCompletionClient = client
	contractCompletionMu.Unlock()

	offsetSec := st.OffsetSec
	if offsetSec <= 0 {
		offsetSec = defaultContractCompletionOffsetSec
	}

	_, _, startTOD, endTOD, quietValid := resolveQuietWindowFromOffsets(st.StartVal, st.EndVal, st.RefTime)

	nextCheckAt := time.Now().Add(time.Second).Unix()
	if !st.EstEndTime.IsZero() {
		notifyAt, _, _ := calculateVirtueSiloNotifyAt(st.EstEndTime, offsetSec, quietValid, startTOD, endTOD)
		if time.Now().Before(notifyAt) {
			nextCheckAt = notifyAt.Unix()
		}
	}

	rec := contractCompletionWatch{
		UserID:        st.UserID,
		NotifyUserID:  e.UserID(),
		ChannelID:     e.ChannelID(),
		ContractID:    st.ContractID,
		CoopID:        st.CoopID,
		OffsetSeconds: offsetSec,
		NextCheckAt:   nextCheckAt,
	}
	if quietValid {
		rec.QuietHoursSet = true
		rec.QuietStartTOD = startTOD
		rec.QuietEndTOD = endTOD
	}
	scheduleContractCompletionTimer(rec)

	confirmMsg := fmt.Sprintf("✅ **Confirmed:** 🏁 Contract completion watch enabled for **%s/%s** (**%s** before completion).",
		st.ContractID, st.CoopID, bottools.FmtDuration(time.Duration(offsetSec)*time.Second))
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

// HandleContractCompletionClear handles the Clear Watch button for contract Completion Watch.
func HandleContractCompletionClear(e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 3 {
		return
	}
	contractID := parts[1]
	coopID := parts[2]
	targetUserID := farmerstate.GetEffectiveUserID(e.UserID(), e.ChannelID())
	removeContractCompletionWatch(targetUserID, contractID, coopID)

	kept := e.MessageComponentsWithoutActionRows()
	kept = append(kept, dc.TextDisplay{
		Content: fmt.Sprintf("✅ **Cleared:** 🏁 Contract completion watch for **%s/%s** has been removed.", contractID, coopID),
	})
	_ = e.Update(dc.Message{
		Components: kept,
	})
}

// HandleContractCompletionCancel handles the Cancel button for contract Completion Watch.
func HandleContractCompletionCancel(e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) >= 2 && parts[1] != "clear" {
		deleteContractCompletionPrompt(parts[1])
	}
	kept := e.MessageComponentsWithoutActionRows()
	kept = append(kept, dc.TextDisplay{
		Content: "❌ **Cancelled:** Contract completion watch was not changed.",
	})
	_ = e.Update(dc.Message{
		Components: kept,
	})
}
