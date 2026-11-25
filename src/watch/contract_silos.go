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

type contractSiloWatch struct {
	UserID               string `json:"user_id"`
	NotifyUserID         string `json:"notify_user_id"`
	ChannelID            string `json:"channel_id"`
	ContractID           string `json:"contract_id"`
	CoopID               string `json:"coop_id"`
	EggIncName           string `json:"egg_inc_name,omitempty"`
	OffsetSeconds        int64  `json:"offset_seconds"`
	NextCheckAt          int64  `json:"next_check_at"`
	LastNotifiedSyncTime int64  `json:"last_notified_sync_time,omitempty"`
	QuietHoursSet        bool   `json:"quiet_hours_set,omitempty"`
	QuietStartTOD        int64  `json:"quiet_start_tod,omitempty"`
	QuietEndTOD          int64  `json:"quiet_end_tod,omitempty"`
}

type contractSiloPromptState struct {
	UserID       string
	NotifyUserID string
	ContractID   string
	CoopID       string
	EggIncName   string
	OffsetSec    int64
	StartVal     string
	EndVal       string
	RefTime      time.Time
	SiloExpiry   time.Time
	SiloDuration time.Duration
	EstEndTime   time.Time
	CreatedAt    time.Time
}

const (
	defaultContractSiloOffsetSec = 30 * 60
	contractSiloSyncToleranceSec = 60
	contractSiloPromptTTL        = 30 * time.Minute
)

var (
	contractSiloMu          sync.Mutex
	contractSiloTimers      = make(map[string]*time.Timer)
	contractSiloClient      dc.Client
	contractSiloWatchesFile = filepath.Join("ttbb-data", "contract_silo_watches.json")

	contractSiloPromptMu sync.Mutex
	contractSiloPrompts  = make(map[string]*contractSiloPromptState)

	fetchContractCoopStatus         = ei.GetCoopStatus
	fetchContractCoopStatusUncached = ei.GetCoopStatusUncached
)

func init() {
	ei.AddCoopStatusCallback(UpdateContractSiloWatchesFromCoopStatus)
}

func contractSiloKey(userID, contractID, coopID string) string {
	return fmt.Sprintf("%s#%s#%s", userID, strings.ToLower(contractID), strings.ToLower(coopID))
}

func contractSiloWatchesPath() string {
	return contractSiloWatchesFile
}

func loadContractSiloWatchesLocked() []contractSiloWatch {
	data, err := os.ReadFile(contractSiloWatchesPath())
	if err != nil {
		return nil
	}
	var list []contractSiloWatch
	if err := json.Unmarshal(data, &list); err != nil {
		return nil
	}
	return list
}

func saveContractSiloWatchesLocked(list []contractSiloWatch) {
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(contractSiloWatchesPath()), 0755)
	_ = os.WriteFile(contractSiloWatchesPath(), data, 0644)
}

func upsertContractSiloWatch(rec contractSiloWatch) {
	contractSiloMu.Lock()
	defer contractSiloMu.Unlock()

	key := contractSiloKey(rec.UserID, rec.ContractID, rec.CoopID)
	list := loadContractSiloWatchesLocked()
	filtered := make([]contractSiloWatch, 0, len(list)+1)
	for _, item := range list {
		if contractSiloKey(item.UserID, item.ContractID, item.CoopID) != key {
			filtered = append(filtered, item)
		}
	}
	filtered = append(filtered, rec)
	saveContractSiloWatchesLocked(filtered)
}

func removeContractSiloWatch(userID, contractID, coopID string) {
	contractSiloMu.Lock()
	defer contractSiloMu.Unlock()

	key := contractSiloKey(userID, contractID, coopID)
	if t, ok := contractSiloTimers[key]; ok {
		t.Stop()
		delete(contractSiloTimers, key)
	}
	list := loadContractSiloWatchesLocked()
	filtered := make([]contractSiloWatch, 0, len(list))
	for _, item := range list {
		if contractSiloKey(item.UserID, item.ContractID, item.CoopID) != key {
			filtered = append(filtered, item)
		}
	}
	saveContractSiloWatchesLocked(filtered)
}

func removeContractSiloWatchesForCoop(contractID, coopID string) {
	contractSiloMu.Lock()
	defer contractSiloMu.Unlock()

	list := loadContractSiloWatchesLocked()
	if len(list) == 0 {
		return
	}
	filtered := make([]contractSiloWatch, 0, len(list))
	changed := false
	for _, item := range list {
		if strings.EqualFold(item.ContractID, contractID) && strings.EqualFold(item.CoopID, coopID) {
			key := contractSiloKey(item.UserID, item.ContractID, item.CoopID)
			if t, ok := contractSiloTimers[key]; ok {
				t.Stop()
				delete(contractSiloTimers, key)
			}
			changed = true
			continue
		}
		filtered = append(filtered, item)
	}
	if changed {
		saveContractSiloWatchesLocked(filtered)
	}
}

func scheduleContractSiloTimer(rec contractSiloWatch) {
	upsertContractSiloWatch(rec)

	key := contractSiloKey(rec.UserID, rec.ContractID, rec.CoopID)
	contractSiloMu.Lock()
	if old, ok := contractSiloTimers[key]; ok {
		old.Stop()
	}
	delay := max(time.Until(time.Unix(rec.NextCheckAt, 0)), time.Second)
	contractSiloTimers[key] = time.AfterFunc(delay, func() {
		checkContractSiloWatch(key)
	})
	contractSiloMu.Unlock()
}

// LoadContractSiloWatches restores persisted contract silo watches on startup.
func LoadContractSiloWatches(client dc.Client) {
	contractSiloMu.Lock()
	contractSiloClient = client
	list := loadContractSiloWatchesLocked()
	contractSiloMu.Unlock()

	for _, rec := range list {
		r := rec
		key := contractSiloKey(r.UserID, r.ContractID, r.CoopID)
		contractSiloMu.Lock()
		if old, ok := contractSiloTimers[key]; ok {
			old.Stop()
		}
		delay := max(time.Until(time.Unix(r.NextCheckAt, 0)), time.Second)
		contractSiloTimers[key] = time.AfterFunc(delay, func() {
			checkContractSiloWatch(key)
		})
		contractSiloMu.Unlock()
	}
}

func isCoopCompleted(coopStatus *ei.ContractCoopStatusResponse) bool {
	if coopStatus == nil {
		return false
	}
	return coopStatus.GetAllGoalsAchieved() ||
		coopStatus.GetSecondsSinceAllGoalsAchieved() > 0 ||
		coopStatus.GetSecondsRemaining() <= 0
}

func calculateContributorSiloTimes(fi *ei.PlayerFarmInfo, now time.Time) (lastSync, siloExpiry time.Time, siloDuration time.Duration, silosOwned uint32) {
	if fi == nil {
		return time.Time{}, time.Time{}, 0, 0
	}
	permitLevel := fi.GetPermitLevel()
	silosOwned = min(fi.GetSilosOwned(), uint32(ei.GetMaxSilosByPermitLevel(permitLevel)))
	siloMinutes := ei.GetSiloMinutes(silosOwned, fi.GetEpicResearch())
	siloDuration = time.Duration(siloMinutes) * time.Minute

	var offlineSec float64
	ts := fi.GetTimestamp()
	if ts < 0 {
		offlineSec = -ts
	} else if ts > 1_000_000_000 {
		offlineSec = max(0, now.Sub(time.Unix(int64(ts), 0)).Seconds())
	}
	lastSync = now.Add(-time.Duration(offlineSec * float64(time.Second)))
	siloExpiry = lastSync.Add(siloDuration)
	return lastSync, siloExpiry, siloDuration, silosOwned
}

func estimateCoopCompletionTime(contractID string, coopStatus *ei.ContractCoopStatusResponse, now time.Time) time.Time {
	if coopStatus == nil || isCoopCompleted(coopStatus) {
		return time.Time{}
	}

	var totalContributions float64
	var contributionRatePerSecond float64
	for _, c := range coopStatus.GetContributors() {
		totalContributions += c.GetContributionAmount()
		if fi := c.GetFarmInfo(); fi != nil && fi.GetTimestamp() < 0 {
			totalContributions += -(c.GetContributionRate() * fi.GetTimestamp())
		}
		contributionRatePerSecond += c.GetContributionRate()
	}

	grade := int(coopStatus.GetGrade())
	if eiContract, ok := ei.EggIncContractsAll[contractID]; ok &&
		eiContract.ID != "" &&
		grade > 0 && grade < len(eiContract.Grade) &&
		len(eiContract.Grade[grade].TargetAmount) > 0 &&
		contributionRatePerSecond > 0 {
		totalRequired := eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1]
		remSec := (totalRequired - totalContributions) / contributionRatePerSecond
		if remSec > 0 && !math.IsInf(remSec, 0) && !math.IsNaN(remSec) {
			return now.Add(time.Duration(remSec * float64(time.Second)))
		}
	}

	if rem := coopStatus.GetSecondsRemaining(); rem > 0 {
		return now.Add(time.Duration(rem * float64(time.Second)))
	}
	return time.Time{}
}

func findContributorForUser(coopStatus *ei.ContractCoopStatusResponse, userID, savedIGN string) *ei.ContractCoopStatusResponse_ContributionInfo {
	if coopStatus == nil {
		return nil
	}
	ign := strings.TrimSpace(savedIGN)
	if ign == "" && userID != "" {
		ign = strings.TrimSpace(farmerstate.GetEggIncName(userID))
	}

	if ign != "" {
		for _, c := range coopStatus.GetContributors() {
			if strings.EqualFold(strings.TrimSpace(c.GetUserName()), ign) {
				return c
			}
		}
	}

	if userID != "" {
		for _, c := range coopStatus.GetContributors() {
			rawName := strings.TrimSpace(c.GetUserName())
			if rawName == "" || rawName == "[departed]" {
				continue
			}
			if dID, err := farmerstate.GetDiscordUserIDFromEiIgnExact(rawName); err == nil && dID == userID {
				return c
			}
		}
		for _, c := range coopStatus.GetContributors() {
			rawName := strings.TrimSpace(c.GetUserName())
			if rawName == "" || rawName == "[departed]" {
				continue
			}
			if dID, err := farmerstate.GetDiscordUserIDFromEiIgn(rawName); err == nil && dID == userID {
				return c
			}
		}
	}
	return nil
}

// UpdateContractSiloWatchesFromCoopStatus updates or cancels active contract silo
// watches whenever any command fetches a coop_status response.
func UpdateContractSiloWatchesFromCoopStatus(contractID, coopID string, coopStatus *ei.ContractCoopStatusResponse) {
	if contractID == "" || coopID == "" || coopStatus == nil {
		return
	}
	if isCoopCompleted(coopStatus) {
		removeContractSiloWatchesForCoop(contractID, coopID)
		return
	}

	contractSiloMu.Lock()
	list := loadContractSiloWatchesLocked()
	var matching []contractSiloWatch
	for _, item := range list {
		if strings.EqualFold(item.ContractID, contractID) && strings.EqualFold(item.CoopID, coopID) {
			matching = append(matching, item)
		}
	}
	contractSiloMu.Unlock()

	if len(matching) == 0 {
		return
	}

	now := time.Now()
	for _, rec := range matching {
		contrib := findContributorForUser(coopStatus, rec.UserID, rec.EggIncName)
		if contrib == nil || contrib.GetFarmInfo() == nil {
			continue
		}
		_, siloExpiry, siloDuration, _ := calculateContributorSiloTimes(contrib.GetFarmInfo(), now)
		if siloDuration <= 0 {
			continue
		}
		notifyAt, _, _ := calculateVirtueSiloNotifyAt(siloExpiry, rec.OffsetSeconds, rec.QuietHoursSet, rec.QuietStartTOD, rec.QuietEndTOD)
		if now.Before(notifyAt) && notifyAt.Unix() > rec.NextCheckAt+contractSiloSyncToleranceSec {
			rec.NextCheckAt = notifyAt.Unix()
			scheduleContractSiloTimer(rec)
		}
	}
}

func checkContractSiloWatch(key string) {
	contractSiloMu.Lock()
	client := contractSiloClient
	var rec *contractSiloWatch
	for _, item := range loadContractSiloWatchesLocked() {
		if contractSiloKey(item.UserID, item.ContractID, item.CoopID) == key {
			c := item
			rec = &c
			break
		}
	}
	contractSiloMu.Unlock()

	if rec == nil || client == nil {
		return
	}

	eiID := farmerstate.GetMiscSettingString(rec.UserID, "encrypted_ei_id")
	coopStatus, nowTime, _, err := fetchContractCoopStatusUncached(rec.ContractID, rec.CoopID, eiID)
	if err != nil || coopStatus == nil {
		rec.NextCheckAt = time.Now().Add(5 * time.Minute).Unix()
		scheduleContractSiloTimer(*rec)
		return
	}

	if coopStatus.GetResponseStatus() != ei.ContractCoopStatusResponse_NO_ERROR {
		switch coopStatus.GetResponseStatus() {
		case ei.ContractCoopStatusResponse_COOP_NOT_FOUND, ei.ContractCoopStatusResponse_CONTRACT_NOT_FOUND:
			removeContractSiloWatchesForCoop(rec.ContractID, rec.CoopID)
		default:
			rec.NextCheckAt = time.Now().Add(5 * time.Minute).Unix()
			scheduleContractSiloTimer(*rec)
		}
		return
	}

	if isCoopCompleted(coopStatus) {
		removeContractSiloWatchesForCoop(rec.ContractID, rec.CoopID)
		return
	}

	if nowTime.IsZero() {
		nowTime = time.Now()
	}

	contrib := findContributorForUser(coopStatus, rec.UserID, rec.EggIncName)
	if contrib == nil || contrib.GetFarmInfo() == nil {
		removeContractSiloWatch(rec.UserID, rec.ContractID, rec.CoopID)
		return
	}

	lastSync, siloExpiry, siloDuration, _ := calculateContributorSiloTimes(contrib.GetFarmInfo(), nowTime)
	if siloDuration <= 0 {
		rec.NextCheckAt = time.Now().Add(5 * time.Minute).Unix()
		scheduleContractSiloTimer(*rec)
		return
	}

	notifyAt, _, _ := calculateVirtueSiloNotifyAt(siloExpiry, rec.OffsetSeconds, rec.QuietHoursSet, rec.QuietStartTOD, rec.QuietEndTOD)
	if time.Now().Before(notifyAt) {
		rec.NextCheckAt = notifyAt.Unix()
		scheduleContractSiloTimer(*rec)
		return
	}

	lastSyncUnix := lastSync.Unix()
	if rec.LastNotifiedSyncTime == 0 || math.Abs(float64(lastSyncUnix-rec.LastNotifiedSyncTime)) > contractSiloSyncToleranceSec {
		rec.LastNotifiedSyncTime = lastSyncUnix
		msgContent := fmt.Sprintf("%s Your contract **%s/%s** silos will run out <t:%d:R> (<t:%d:t>). Please check in and sync your farm!",
			ei.GetBotEmojiMarkdown("silo"), rec.ContractID, rec.CoopID, siloExpiry.Unix(), siloExpiry.Unix())

		if err := sendWatchNotificationDM(client, rec.NotifyUserID, rec.ChannelID, msgContent); err != nil {
			log.Printf("contract silo watch notify error (%s/%s): %v", rec.ContractID, rec.CoopID, err)
		}
	}

	rec.NextCheckAt = time.Now().Add(time.Duration(rec.OffsetSeconds) * time.Second).Unix()
	scheduleContractSiloTimer(*rec)
}

func saveContractSiloPrompt(state *contractSiloPromptState) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)

	now := time.Now()
	state.CreatedAt = now

	contractSiloPromptMu.Lock()
	for k, v := range contractSiloPrompts {
		if now.Sub(v.CreatedAt) > contractSiloPromptTTL {
			delete(contractSiloPrompts, k)
		}
	}
	contractSiloPrompts[token] = state
	contractSiloPromptMu.Unlock()
	return token
}

func getContractSiloPrompt(token string) (*contractSiloPromptState, bool) {
	contractSiloPromptMu.Lock()
	defer contractSiloPromptMu.Unlock()
	st, ok := contractSiloPrompts[token]
	if !ok {
		return nil, false
	}
	if time.Since(st.CreatedAt) > contractSiloPromptTTL {
		delete(contractSiloPrompts, token)
		return nil, false
	}
	return st, true
}

func deleteContractSiloPrompt(token string) {
	contractSiloPromptMu.Lock()
	delete(contractSiloPrompts, token)
	contractSiloPromptMu.Unlock()
}

func buildContractSiloOffsetOptions(selectedSec int64) []dc.SelectOption {
	intervals := []struct {
		label string
		sec   int64
	}{
		{"5 minutes", 5 * 60},
		{"10 minutes", 10 * 60},
		{"15 minutes", 15 * 60},
		{"30 minutes", 30 * 60},
	}
	opts := make([]dc.SelectOption, 0, len(intervals))
	for _, iv := range intervals {
		opts = append(opts, dc.SelectOption{
			Label:       iv.label,
			Value:       strconv.FormatInt(iv.sec, 10),
			Description: fmt.Sprintf("Notify %s before silos run out", bottools.FmtDuration(time.Duration(iv.sec)*time.Second)),
			Default:     selectedSec == iv.sec,
		})
	}
	return opts
}

func buildContractSilosPrompt(token string, st *contractSiloPromptState) []dc.LayoutComponent {
	offsetSec := st.OffsetSec
	if offsetSec <= 0 {
		offsetSec = defaultContractSiloOffsetSec
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
	if !st.SiloExpiry.IsZero() {
		notifyAt, normalNotifyAt, adjusted := calculateVirtueSiloNotifyAt(st.SiloExpiry, offsetSec, quietValid, startTOD, endTOD)
		if adjusted {
			exampleText = fmt.Sprintf("\n\nBased on your last sync, your silos expire <t:%d:R> (<t:%d:t>). Since a normal **%s** watch (<t:%d:t>) falls within your quiet hours, we would set your silo watch 15m before quiet hours at <t:%d:t> (<t:%d:R>).",
				st.SiloExpiry.Unix(), st.SiloExpiry.Unix(), offsetLabel, normalNotifyAt.Unix(), notifyAt.Unix(), notifyAt.Unix())
		} else {
			exampleText = fmt.Sprintf("\n\nBased on your last sync, your silos expire <t:%d:R> (<t:%d:t>), so you would be notified around <t:%d:t> (<t:%d:R>).",
				st.SiloExpiry.Unix(), st.SiloExpiry.Unix(), notifyAt.Unix(), notifyAt.Unix())
		}
	}

	minValues := 1
	return []dc.LayoutComponent{
		dc.TextDisplay{
			Content: fmt.Sprintf("Monitoring your **%s/%s** contract silos checks the coop status periodically and automatically extends when you sync, then cancels once the contract completes. Select your notification offset (**%s**) and optional quiet hours below:",
				st.ContractID, st.CoopID, offsetLabel) + exampleText + quietText,
		},
		dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.SelectMenu{
					CustomID:    fmt.Sprintf("watch-csilos-offset#%s", token),
					Placeholder: "Notification Offset",
					MinValues:   &minValues,
					MaxValues:   1,
					Options:     buildContractSiloOffsetOptions(offsetSec),
				},
			},
		},
		dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.SelectMenu{
					CustomID:    fmt.Sprintf("watch-csilos-qstart#%s", token),
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
					CustomID:    fmt.Sprintf("watch-csilos-qend#%s", token),
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
					CustomID: fmt.Sprintf("watch-csilos-confirm#%s", token),
				},
				dc.Button{
					Label:    "Cancel",
					Style:    dc.ButtonSecondary,
					CustomID: fmt.Sprintf("watch-csilos-cancel#%s", token),
				},
			},
		},
	}
}

// HandleContractSilosToggle handles the Silo Watch button on /cs-estimate.
func HandleContractSilosToggle(e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 3 {
		return
	}
	contractID := parts[1]
	coopID := parts[2]
	targetUserID := farmerstate.GetEffectiveUserID(e.UserID(), e.ChannelID())

	// Check for an existing watch for this user + contract + coop.
	key := contractSiloKey(targetUserID, contractID, coopID)
	var existing *contractSiloWatch
	contractSiloMu.Lock()
	for _, item := range loadContractSiloWatchesLocked() {
		if contractSiloKey(item.UserID, item.ContractID, item.CoopID) == key {
			c := item
			existing = &c
			break
		}
	}
	contractSiloMu.Unlock()

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
					Content: fmt.Sprintf("%s You currently have an active contract silo watch for **%s/%s** (**%s** before silos run out), scheduled to check/notify <t:%d:R> (<t:%d:t>).%s Would you like to clear it?",
						ei.GetBotEmojiMarkdown("silo"), contractID, coopID, bottools.FmtDuration(time.Duration(existing.OffsetSeconds)*time.Second), existing.NextCheckAt, existing.NextCheckAt, quietInfo),
				},
				dc.ActionRow{
					Components: []dc.InteractiveComponent{
						dc.Button{
							Label:    "Clear Watch",
							Style:    dc.ButtonDanger,
							CustomID: fmt.Sprintf("watch-csilos-clear#%s#%s", contractID, coopID),
						},
						dc.Button{
							Label:    "Cancel",
							Style:    dc.ButtonSecondary,
							CustomID: "watch-csilos-cancel#clear",
						},
					},
				},
			},
		})
		return
	}

	// Acknowledge the button press immediately with an ephemeral response before fetching coop_status.
	_ = e.Respond(dc.Message{
		Ephemeral: true,
		Components: []dc.LayoutComponent{
			dc.TextDisplay{
				Content: fmt.Sprintf("%s Checking coop status for **%s/%s**...", ei.GetBotEmojiMarkdown("silo"), contractID, coopID),
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
		removeContractSiloWatchesForCoop(contractID, coopID)
		_ = e.EditResponse(dc.Message{
			Ephemeral: true,
			Components: []dc.LayoutComponent{
				dc.TextDisplay{
					Content: fmt.Sprintf("Contract **%s/%s** is already completed, so no Silo Watch is needed.", contractID, coopID),
				},
			},
		})
		return
	}

	if nowTime.IsZero() {
		nowTime = time.Now()
	}

	contrib := findContributorForUser(coopStatus, targetUserID, "")
	if contrib == nil {
		_ = e.EditResponse(dc.Message{
			Ephemeral: true,
			Components: []dc.LayoutComponent{
				dc.TextDisplay{
					Content: fmt.Sprintf("Could not find you in coop **%s/%s**. Make sure you are in this coop and your Egg Inc IGN is linked to Boost Bot.", contractID, coopID),
				},
			},
		})
		return
	}

	fi := contrib.GetFarmInfo()
	if fi == nil {
		_ = e.EditResponse(dc.Message{
			Ephemeral: true,
			Components: []dc.LayoutComponent{
				dc.TextDisplay{
					Content: fmt.Sprintf("Your farm info is not yet available in coop status for **%s/%s**. Please sync your farm and try again.", contractID, coopID),
				},
			},
		})
		return
	}

	_, siloExpiry, siloDuration, silosOwned := calculateContributorSiloTimes(fi, nowTime)
	estEndTime := estimateCoopCompletionTime(contractID, coopStatus, nowTime)

	// If the player's silos already extend past the estimated contract completion, explain why they don't need Silo Watch.
	if !estEndTime.IsZero() && siloExpiry.After(estEndTime) {
		permitName := "Standard Permit"
		if fi.GetPermitLevel() == ei.ProPermitLevel {
			permitName = "Pro Permit"
		}
		_ = e.EditResponse(dc.Message{
			Ephemeral: true,
			Components: []dc.LayoutComponent{
				dc.TextDisplay{
					Content: fmt.Sprintf("%s You don't need a Silo Watch for **%s/%s**!\n\nWith your **%s** and **%d** purchased silos, your silo away time is **%s**. Based on your last sync, your silos won't run out until <t:%d:R> (<t:%d:t>), which extends past the estimated contract completion <t:%d:R> (<t:%d:t>).",
						ei.GetBotEmojiMarkdown("silo"),
						contractID, coopID,
						permitName, silosOwned,
						bottools.FmtDuration(siloDuration),
						siloExpiry.Unix(), siloExpiry.Unix(),
						estEndTime.Unix(), estEndTime.Unix(),
					),
				},
			},
		})
		return
	}

	startVal, endVal := loadSavedQuietOffsets(targetUserID)
	offsetSec := loadSavedWatchOffset(targetUserID, "watch_csilos_offset", defaultContractSiloOffsetSec, buildContractSiloOffsetOptions(0))
	state := &contractSiloPromptState{
		UserID:       targetUserID,
		NotifyUserID: e.UserID(),
		ContractID:   contractID,
		CoopID:       coopID,
		EggIncName:   strings.TrimSpace(contrib.GetUserName()),
		OffsetSec:    offsetSec,
		StartVal:     startVal,
		EndVal:       endVal,
		RefTime:      nowTime,
		SiloExpiry:   siloExpiry,
		SiloDuration: siloDuration,
		EstEndTime:   estEndTime,
	}
	token := saveContractSiloPrompt(state)

	_ = e.EditResponse(dc.Message{
		Ephemeral:  true,
		Components: buildContractSilosPrompt(token, state),
	})
}

// HandleContractSilosSelect handles offset and quiet hours dropdown selections for contract Silo Watch.
func HandleContractSilosSelect(e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 2 {
		return
	}
	action := parts[0]
	token := parts[1]

	st, ok := getContractSiloPrompt(token)
	if !ok {
		_ = e.Update(dc.Message{
			Components: []dc.LayoutComponent{
				dc.TextDisplay{Content: "This Silo Watch prompt has expired. Please click **Silo Watch** again."},
			},
		})
		return
	}

	if vals := e.Values(); len(vals) > 0 {
		switch action {
		case "watch-csilos-offset":
			if sec, err := strconv.ParseInt(vals[0], 10, 64); err == nil && sec > 0 {
				st.OffsetSec = sec
			}
		case "watch-csilos-qstart":
			st.StartVal = vals[0]
		case "watch-csilos-qend":
			st.EndVal = vals[0]
		}
	}

	_ = e.Update(dc.Message{
		Components: buildContractSilosPrompt(token, st),
	})
}

// HandleContractSilosConfirm handles the Confirm button for contract Silo Watch.
func HandleContractSilosConfirm(client dc.Client, e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 2 {
		return
	}
	token := parts[1]

	st, ok := getContractSiloPrompt(token)
	if !ok {
		_ = e.Update(dc.Message{
			Components: []dc.LayoutComponent{
				dc.TextDisplay{Content: "This Silo Watch prompt has expired. Please click **Silo Watch** again."},
			},
		})
		return
	}
	deleteContractSiloPrompt(token)

	contractSiloMu.Lock()
	contractSiloClient = client
	contractSiloMu.Unlock()

	offsetSec := st.OffsetSec
	if offsetSec <= 0 {
		offsetSec = defaultContractSiloOffsetSec
	}

	_, _, startTOD, endTOD, quietValid := resolveQuietWindowFromOffsets(st.StartVal, st.EndVal, st.RefTime)

	nextCheckAt := time.Now().Add(time.Second).Unix()
	if !st.SiloExpiry.IsZero() {
		notifyAt, _, _ := calculateVirtueSiloNotifyAt(st.SiloExpiry, offsetSec, quietValid, startTOD, endTOD)
		if time.Now().Before(notifyAt) {
			nextCheckAt = notifyAt.Unix()
		}
	}

	rec := contractSiloWatch{
		UserID:        st.UserID,
		NotifyUserID:  e.UserID(),
		ChannelID:     e.ChannelID(),
		ContractID:    st.ContractID,
		CoopID:        st.CoopID,
		EggIncName:    st.EggIncName,
		OffsetSeconds: offsetSec,
		NextCheckAt:   nextCheckAt,
	}
	if quietValid {
		rec.QuietHoursSet = true
		rec.QuietStartTOD = startTOD
		rec.QuietEndTOD = endTOD
	}
	farmerstate.SetMiscSettingString(st.UserID, "watch_csilos_offset", strconv.FormatInt(offsetSec, 10))
	if quietValid {
		saveQuietOffsets(st.UserID, st.StartVal, st.EndVal)
	} else {
		saveQuietOffsets(st.UserID, "unset", "unset")
	}
	scheduleContractSiloTimer(rec)

	confirmMsg := fmt.Sprintf("✅ **Confirmed:** %s Contract silo watch enabled for **%s/%s** (**%s** before your silos run out).",
		ei.GetBotEmojiMarkdown("silo"), st.ContractID, st.CoopID, bottools.FmtDuration(time.Duration(offsetSec)*time.Second))
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

// HandleContractSilosClear handles the Clear Watch button for contract Silo Watch.
func HandleContractSilosClear(e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 3 {
		return
	}
	contractID := parts[1]
	coopID := parts[2]
	targetUserID := farmerstate.GetEffectiveUserID(e.UserID(), e.ChannelID())
	removeContractSiloWatch(targetUserID, contractID, coopID)

	kept := e.MessageComponentsWithoutActionRows()
	kept = append(kept, dc.TextDisplay{
		Content: fmt.Sprintf("✅ **Cleared:** %s Contract silo watch for **%s/%s** has been removed.", ei.GetBotEmojiMarkdown("silo"), contractID, coopID),
	})
	_ = e.Update(dc.Message{
		Components: kept,
	})
}

// HandleContractSilosCancel handles the Cancel button for contract Silo Watch.
func HandleContractSilosCancel(e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) >= 2 && parts[1] != "clear" {
		deleteContractSiloPrompt(parts[1])
	}
	kept := e.MessageComponentsWithoutActionRows()
	kept = append(kept, dc.TextDisplay{
		Content: "❌ **Cancelled:** Contract silo watch was not changed.",
	})
	_ = e.Update(dc.Message{
		Components: kept,
	})
}
