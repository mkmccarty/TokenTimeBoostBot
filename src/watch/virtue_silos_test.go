package watch

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

func TestBuildQuietOffsetOptions(t *testing.T) {
	loc := eggStandardLocation()
	pstRef := time.Date(2026, 11, 21, 21, 0, 0, 0, loc)
	opts := buildQuietOffsetOptions("5", pstRef)
	if len(opts) != 25 {
		t.Fatalf("expected 25 options, got %d", len(opts))
	}
	if opts[0].Label != "None" || opts[0].Value != "none" || opts[0].Default {
		t.Errorf("unexpected first option: %+v", opts[0])
	}
	if opts[1].Label != "+0" || opts[1].Value != "0" || opts[1].Description != "9:00 AM PST" {
		t.Errorf("unexpected +0 PST option: %+v", opts[1])
	}
	if opts[24].Label != "+23" || opts[24].Value != "23" || opts[24].Description != "8:00 AM PST" {
		t.Errorf("unexpected +23 PST option: %+v", opts[24])
	}
	if !opts[6].Default {
		t.Errorf("expected option +5 (index 6) to be default, got %+v", opts[6])
	}

	pdtRef := time.Date(2026, 7, 14, 12, 0, 0, 0, loc)
	pdtOpts := buildQuietOffsetOptions("none", pdtRef)
	if pdtOpts[1].Description != "9:00 AM PDT" {
		t.Errorf("expected 9:00 AM PDT during DST, got %q", pdtOpts[1].Description)
	}
}

func TestCalculateVirtueSiloNotifyAt(t *testing.T) {
	loc := eggStandardLocation()
	// 22:00 to 06:00 Egg Standard Time overnight quiet window.
	startTOD := int64(22 * 3600)
	endTOD := int64(6 * 3600)
	offsetSec := int64(30 * 60)

	tests := []struct {
		name         string
		siloExpiry   time.Time
		quietSet     bool
		startTOD     int64
		endTOD       int64
		wantNotifyAt time.Time
		wantAdjusted bool
	}{
		{
			name:         "inside overnight quiet window after midnight",
			siloExpiry:   time.Date(2026, 11, 22, 3, 0, 0, 0, loc),
			quietSet:     true,
			startTOD:     startTOD,
			endTOD:       endTOD,
			wantNotifyAt: time.Date(2026, 11, 21, 21, 45, 0, 0, loc),
			wantAdjusted: true,
		},
		{
			name:         "inside overnight quiet window before midnight",
			siloExpiry:   time.Date(2026, 11, 21, 23, 0, 0, 0, loc),
			quietSet:     true,
			startTOD:     startTOD,
			endTOD:       endTOD,
			wantNotifyAt: time.Date(2026, 11, 21, 21, 45, 0, 0, loc),
			wantAdjusted: true,
		},
		{
			name:         "exactly at quiet start is inside",
			siloExpiry:   time.Date(2026, 11, 21, 22, 30, 0, 0, loc),
			quietSet:     true,
			startTOD:     startTOD,
			endTOD:       endTOD,
			wantNotifyAt: time.Date(2026, 11, 21, 21, 45, 0, 0, loc),
			wantAdjusted: true,
		},
		{
			name:         "exactly at quiet end is outside",
			siloExpiry:   time.Date(2026, 11, 22, 6, 30, 0, 0, loc),
			quietSet:     true,
			startTOD:     startTOD,
			endTOD:       endTOD,
			wantNotifyAt: time.Date(2026, 11, 22, 6, 0, 0, 0, loc),
			wantAdjusted: false,
		},
		{
			name:         "outside overnight quiet window",
			siloExpiry:   time.Date(2026, 11, 22, 15, 0, 0, 0, loc),
			quietSet:     true,
			startTOD:     startTOD,
			endTOD:       endTOD,
			wantNotifyAt: time.Date(2026, 11, 22, 14, 30, 0, 0, loc),
			wantAdjusted: false,
		},
		{
			name:         "same-day EST quiet window 05:00-13:00",
			siloExpiry:   time.Date(2026, 11, 22, 8, 30, 0, 0, loc),
			quietSet:     true,
			startTOD:     5 * 3600,
			endTOD:       13 * 3600,
			wantNotifyAt: time.Date(2026, 11, 22, 4, 45, 0, 0, loc),
			wantAdjusted: true,
		},
		{
			name:         "quiet hours disabled",
			siloExpiry:   time.Date(2026, 11, 22, 3, 0, 0, 0, loc),
			quietSet:     false,
			startTOD:     startTOD,
			endTOD:       endTOD,
			wantNotifyAt: time.Date(2026, 11, 22, 2, 30, 0, 0, loc),
			wantAdjusted: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, _, adjusted := calculateVirtueSiloNotifyAt(tc.siloExpiry, offsetSec, tc.quietSet, tc.startTOD, tc.endTOD)
			if adjusted != tc.wantAdjusted {
				t.Errorf("adjusted = %v, want %v", adjusted, tc.wantAdjusted)
			}
			if !got.Equal(tc.wantNotifyAt) {
				t.Errorf("notifyAt = %v, want %v", got, tc.wantNotifyAt)
			}
		})
	}
}

func TestResolveQuietWindowFromOffsets(t *testing.T) {
	loc := eggStandardLocation()
	ref := time.Date(2026, 11, 21, 21, 0, 0, 0, loc)

	// +2 = 11:00 AM PST, +10 = 7:00 PM PST
	qStart, qEnd, startTOD, endTOD, valid := resolveQuietWindowFromOffsets("2", "10", ref)
	if !valid {
		t.Fatalf("expected valid quiet window")
	}
	wantStart := time.Date(2026, 11, 22, 11, 0, 0, 0, loc)
	wantEnd := time.Date(2026, 11, 22, 19, 0, 0, 0, loc)
	if !qStart.Equal(wantStart) || !qEnd.Equal(wantEnd) {
		t.Errorf("unexpected window %v - %v, want %v - %v", qStart, qEnd, wantStart, wantEnd)
	}
	if startTOD != 11*3600 || endTOD != 19*3600 {
		t.Errorf("unexpected TODs: %d, %d", startTOD, endTOD)
	}

	// +14 = 11:00 PM PST, +22 = 7:00 AM PST next day (8h overnight window)
	qStart, qEnd, startTOD, endTOD, valid = resolveQuietWindowFromOffsets("14", "22", ref)
	if !valid {
		t.Fatalf("expected valid wrapped quiet window")
	}
	if qEnd.Sub(qStart) != 8*time.Hour {
		t.Errorf("expected 8h wrapped window, got %v", qEnd.Sub(qStart))
	}
	if startTOD != 23*3600 || endTOD != 7*3600 {
		t.Errorf("unexpected wrapped TODs: %d, %d", startTOD, endTOD)
	}

	// Invalid / incomplete cases
	for _, pair := range [][2]string{{"none", "10"}, {"2", "unset"}, {"5", "5"}} {
		if _, _, _, _, ok := resolveQuietWindowFromOffsets(pair[0], pair[1], ref); ok {
			t.Errorf("expected invalid for %v", pair)
		}
	}
}

func TestVirtueSilosQuietHoursFlow(t *testing.T) {
	userID := "100000000000000073"
	removeVirtueSiloWatch(userID)
	defer removeVirtueSiloWatch(userID)

	// 1. Toggle opens ephemeral prompt with 2 select menus (25 options each) + Confirm/Cancel
	toggleEv := dctest.ComponentButtonEventWithUser("watch-vsilos-toggle#"+userID, userID)
	HandleVirtueSilosToggle(toggleEv)
	if toggleEv.LastResponse == nil || !toggleEv.LastResponse.Ephemeral {
		t.Fatalf("expected ephemeral response from HandleVirtueSilosToggle")
	}
	if len(toggleEv.LastResponse.Components) != 4 {
		t.Fatalf("expected 4 layout components, got %d", len(toggleEv.LastResponse.Components))
	}

	row1, ok := toggleEv.LastResponse.Components[1].(dc.ActionRow)
	if !ok || len(row1.Components) != 1 {
		t.Fatalf("expected start ActionRow with 1 select menu")
	}
	startMenu := row1.Components[0].(dc.SelectMenu)
	if len(startMenu.Options) != 25 {
		t.Fatalf("expected 25 options in start menu, got %d", len(startMenu.Options))
	}

	// 2. Select Start = +2
	startEv := dctest.ComponentSelectEventWithUser(startMenu.CustomID, userID, "2")
	HandleVirtueSilosQuietSelect(startEv)
	if startEv.LastResponse == nil {
		t.Fatalf("expected update response after selecting start")
	}
	row2 := startEv.LastResponse.Components[2].(dc.ActionRow)
	endMenu := row2.Components[0].(dc.SelectMenu)

	// 3. Select End = +10
	endEv := dctest.ComponentSelectEventWithUser(endMenu.CustomID, userID, "10")
	HandleVirtueSilosQuietSelect(endEv)
	if endEv.LastResponse == nil {
		t.Fatalf("expected update response after selecting end")
	}
	td := endEv.LastResponse.Components[0].(dc.TextDisplay)
	if !strings.Contains(td.Content, "<t:") {
		t.Errorf("expected Discord timestamps in quiet hours text, got: %s", td.Content)
	}

	btnRow := endEv.LastResponse.Components[3].(dc.ActionRow)
	confirmBtn := btnRow.Components[0].(dc.Button)
	if !strings.HasPrefix(confirmBtn.CustomID, fmt.Sprintf("watch-vsilos-confirm#%s#1800#", userID)) {
		t.Fatalf("unexpected confirm CustomID: %s", confirmBtn.CustomID)
	}
	if strings.HasSuffix(confirmBtn.CustomID, "#-1#-1") {
		t.Fatalf("expected quiet hours TOD in confirm CustomID, got: %s", confirmBtn.CustomID)
	}

	// 4. Confirm saves watch with QuietHoursSet = true
	confirmEv := dctest.ComponentButtonEventWithUser(confirmBtn.CustomID, userID)
	HandleVirtueSilosConfirm(nil, confirmEv)

	virtueSiloMu.Lock()
	var saved *virtueSiloWatch
	for _, item := range loadVirtueSiloWatchesLocked() {
		if item.UserID == userID {
			c := item
			saved = &c
			break
		}
	}
	virtueSiloMu.Unlock()

	if saved == nil || !saved.QuietHoursSet {
		t.Fatalf("expected saved watch with QuietHoursSet=true, got %+v", saved)
	}
}

func TestWatchSettingsPrepopulateAcrossCommands(t *testing.T) {
	setupContractSilosTestEnv(t)

	origCompFile := contractCompletionWatchesFile
	contractCompletionWatchesFile = filepath.Join(t.TempDir(), "contract_completion_watches.json")
	userID := "100000000000000199"
	removeVirtueSiloWatch(userID)
	t.Cleanup(func() {
		removeVirtueSiloWatch(userID)
		contractCompletionMu.Lock()
		for k, tm := range contractCompletionTimers {
			tm.Stop()
			delete(contractCompletionTimers, k)
		}
		contractCompletionMu.Unlock()
		contractCompletionWatchesFile = origCompFile
	})

	farmerstate.SetEggIncName(userID, "PrepopFarmer")
	t.Cleanup(func() {
		farmerstate.SetMiscSettingString(userID, "watch_quiet_start", "")
		farmerstate.SetMiscSettingString(userID, "watch_quiet_end", "")
		farmerstate.SetMiscSettingString(userID, "watch_csilos_offset", "")
		farmerstate.SetMiscSettingString(userID, "watch_ccomp_offset", "")
	})

	u32 := func(v uint32) *uint32 { return &v }
	f64 := func(v float64) *float64 { return &v }
	sptr := func(v string) *string { return &v }

	now := time.Now()
	fetchContractCoopStatus = func(contractID, coopID, eeidOverride string) (*ei.ContractCoopStatusResponse, time.Time, string, error) {
		return makeTestCoopStatus([]*ei.ContractCoopStatusResponse_ContributionInfo{
			{
				UserName: sptr("PrepopFarmer"),
				FarmInfo: &ei.PlayerFarmInfo{
					PermitLevel: u32(ei.StandardPermitLevel),
					SilosOwned:  u32(2),
					EpicResearch: []*ei.Backup_ResearchItem{
						{Id: sptr("silo_capacity"), Level: u32(20)},
					},
					Timestamp: f64(-3600),
				},
			},
		}, 24*3600, false), now, "", nil
	}

	hasDefault := func(opts []dc.SelectOption, wantVal string) bool {
		for _, o := range opts {
			if o.Value == wantVal && o.Default {
				return true
			}
		}
		return false
	}

	// 1. Confirm Virtue Silo Watch with quiet hours +3 to +11 -> saves quiet hours.
	vsStart := dctest.ComponentSelectEventWithUser(fmt.Sprintf("watch-vsilos-qstart#%s#unset#unset#%d", userID, now.Unix()), userID, "3")
	HandleVirtueSilosQuietSelect(vsStart)
	vsEndMenu := vsStart.LastResponse.Components[2].(dc.ActionRow).Components[0].(dc.SelectMenu)
	vsEnd := dctest.ComponentSelectEventWithUser(vsEndMenu.CustomID, userID, "11")
	HandleVirtueSilosQuietSelect(vsEnd)
	vsConfirmBtn := vsEnd.LastResponse.Components[3].(dc.ActionRow).Components[0].(dc.Button)
	HandleVirtueSilosConfirm(nil, dctest.ComponentButtonEventWithUser(vsConfirmBtn.CustomID, userID))

	// 2. Open Contract Silo Watch -> quiet hours +3 and +11 should be pre-populated.
	csToggle := dctest.ComponentButtonEventWithUser("watch-csilos-toggle#c1#coop1", userID)
	HandleContractSilosToggle(csToggle)
	csOffsetMenu := csToggle.LastResponse.Components[1].(dc.ActionRow).Components[0].(dc.SelectMenu)
	csStartMenu := csToggle.LastResponse.Components[2].(dc.ActionRow).Components[0].(dc.SelectMenu)
	csEndMenu := csToggle.LastResponse.Components[3].(dc.ActionRow).Components[0].(dc.SelectMenu)
	if !hasDefault(csStartMenu.Options, "3") || !hasDefault(csEndMenu.Options, "11") {
		t.Fatalf("expected contract silos prompt to pre-populate quiet hours +3/+11")
	}

	// Change Contract Silo Watch offset to 600 (10m) and confirm -> saves 600.
	csOffsetSel := dctest.ComponentSelectEventWithUser(csOffsetMenu.CustomID, userID, "600")
	HandleContractSilosSelect(csOffsetSel)
	csConfirmBtn := csOffsetSel.LastResponse.Components[4].(dc.ActionRow).Components[0].(dc.Button)
	HandleContractSilosConfirm(nil, dctest.ComponentButtonEventWithUser(csConfirmBtn.CustomID, userID))

	// Re-open Contract Silo Watch on another coop -> offset 600 and quiet hours +3/+11 pre-populated.
	csToggle2 := dctest.ComponentButtonEventWithUser("watch-csilos-toggle#c2#coop2", userID)
	HandleContractSilosToggle(csToggle2)
	csOffsetMenu2 := csToggle2.LastResponse.Components[1].(dc.ActionRow).Components[0].(dc.SelectMenu)
	if !hasDefault(csOffsetMenu2.Options, "600") {
		t.Fatalf("expected contract silos prompt to pre-populate saved offset 600")
	}

	// 3. Open Contract Completion Watch -> quiet hours +3/+11 pre-populated; change offset to 3600 (1h) and clear quiet hours.
	ccToggle := dctest.ComponentButtonEventWithUser("watch-ccomp-toggle#c1#coop1", userID)
	HandleContractCompletionToggle(ccToggle)
	ccOffsetMenu := ccToggle.LastResponse.Components[1].(dc.ActionRow).Components[0].(dc.SelectMenu)
	ccStartMenu := ccToggle.LastResponse.Components[2].(dc.ActionRow).Components[0].(dc.SelectMenu)
	ccEndMenu := ccToggle.LastResponse.Components[3].(dc.ActionRow).Components[0].(dc.SelectMenu)
	if !hasDefault(ccStartMenu.Options, "3") || !hasDefault(ccEndMenu.Options, "11") {
		t.Fatalf("expected contract completion prompt to pre-populate quiet hours +3/+11")
	}

	ccOffsetSel := dctest.ComponentSelectEventWithUser(ccOffsetMenu.CustomID, userID, "3600")
	HandleContractCompletionSelect(ccOffsetSel)
	ccStartSel := dctest.ComponentSelectEventWithUser(ccOffsetSel.LastResponse.Components[2].(dc.ActionRow).Components[0].(dc.SelectMenu).CustomID, userID, "none")
	HandleContractCompletionSelect(ccStartSel)
	ccConfirmBtn := ccStartSel.LastResponse.Components[4].(dc.ActionRow).Components[0].(dc.Button)
	HandleContractCompletionConfirm(nil, dctest.ComponentButtonEventWithUser(ccConfirmBtn.CustomID, userID))

	// Re-open Contract Completion Watch on another coop -> offset 3600 pre-populated, quiet hours cleared.
	ccToggle2 := dctest.ComponentButtonEventWithUser("watch-ccomp-toggle#c2#coop2", userID)
	HandleContractCompletionToggle(ccToggle2)
	ccOffsetMenu2 := ccToggle2.LastResponse.Components[1].(dc.ActionRow).Components[0].(dc.SelectMenu)
	ccStartMenu2 := ccToggle2.LastResponse.Components[2].(dc.ActionRow).Components[0].(dc.SelectMenu)
	if !hasDefault(ccOffsetMenu2.Options, "3600") {
		t.Fatalf("expected contract completion prompt to pre-populate saved offset 3600")
	}
	if hasDefault(ccStartMenu2.Options, "3") {
		t.Fatalf("expected quiet hours to be cleared after confirming without quiet hours")
	}

	// Re-open Virtue Silo Watch (after clearing existing watch) -> quiet hours cleared.
	removeVirtueSiloWatch(userID)
	vsToggle := dctest.ComponentButtonEventWithUser(fmt.Sprintf("watch-vsilos-toggle#%s", userID), userID)
	HandleVirtueSilosToggle(vsToggle)
	vsStartMenu := vsToggle.LastResponse.Components[1].(dc.ActionRow).Components[0].(dc.SelectMenu)
	if hasDefault(vsStartMenu.Options, "3") {
		t.Fatalf("expected virtue silos prompt quiet hours to be cleared")
	}
}

func TestVirtueSiloWatchNotificationIncludesAccountName(t *testing.T) {
	origWatchesFile := virtueSiloWatchesFile
	virtueSiloWatchesFile = filepath.Join(t.TempDir(), "virtue_silo_watches.json")
	origFetch := fetchVirtueBackup
	origResearches := ei.EggIncResearchesMap
	ei.EggIncResearchesMap = map[string]ei.EggResearches{
		"silo_capacity": {
			ID:             "silo_capacity",
			EffectType:     "additive",
			LevelsCompound: "additive",
			PerLevel:       6,
		},
	}

	t.Cleanup(func() {
		virtueSiloMu.Lock()
		for k, tm := range virtueSiloTimers {
			tm.Stop()
			delete(virtueSiloTimers, k)
		}
		virtueSiloClient = nil
		virtueSiloMu.Unlock()
		virtueSiloWatchesFile = origWatchesFile
		fetchVirtueBackup = origFetch
		ei.EggIncResearchesMap = origResearches
	})

	mainUserID := "100000000000000201"
	altUserID := "100000000000000202"
	farmerstate.SetMiscSettingString(altUserID, "encrypted_ei_id", "EI1234567890123456")
	t.Cleanup(func() {
		farmerstate.SetMiscSettingString(altUserID, "encrypted_ei_id", "")
		farmerstate.SetEggIncName(altUserID, "")
	})

	u32 := func(v uint32) *uint32 { return &v }
	f64 := func(v float64) *float64 { return &v }
	sptr := func(v string) *string { return &v }
	egg := ei.Egg_CURIOSITY

	// Last saved 5h50m ago with 6h silo capacity -> expires in 10m (within 30m offset).
	lastStep := float64(time.Now().Add(-5*time.Hour - 50*time.Minute).Unix())
	fetchVirtueBackup = func(encryptedID, userID string, force bool) (*ei.Backup, bool) {
		return &ei.Backup{
			UserName: sptr("VirtueAltFarmer"),
			Game: &ei.Backup_Game{
				EpicResearch: []*ei.Backup_ResearchItem{
					{Id: sptr("silo_capacity"), Level: u32(20)},
				},
			},
			Farms: []*ei.Backup_Simulation{
				{
					EggType:      &egg,
					SilosOwned:   u32(2),
					LastStepTime: f64(lastStep),
				},
			},
		}, false
	}

	client := dctest.New()
	virtueSiloMu.Lock()
	virtueSiloClient = client
	virtueSiloMu.Unlock()

	upsertVirtueSiloWatch(virtueSiloWatch{
		UserID:        altUserID,
		NotifyUserID:  mainUserID,
		ChannelID:     "chan-virtue",
		OffsetSeconds: 1800,
		NextCheckAt:   time.Now().Add(-time.Minute).Unix(),
	})

	checkVirtueSiloWatch(altUserID)

	if len(client.SentMessages) != 1 {
		t.Fatalf("expected 1 DM sent, got %d", len(client.SentMessages))
	}
	got := client.SentMessages[0].Content
	if !strings.Contains(got, "for **VirtueAltFarmer**") {
		t.Errorf("expected notification to include account name 'for **VirtueAltFarmer**', got: %s", got)
	}

	// Fallback when backup and farmerstate have no IGN for an alt account -> mentions altUserID.
	fetchVirtueBackup = func(encryptedID, userID string, force bool) (*ei.Backup, bool) {
		return &ei.Backup{
			Game: &ei.Backup_Game{
				EpicResearch: []*ei.Backup_ResearchItem{
					{Id: sptr("silo_capacity"), Level: u32(20)},
				},
			},
			Farms: []*ei.Backup_Simulation{
				{
					EggType:      &egg,
					SilosOwned:   u32(2),
					LastStepTime: f64(lastStep - 10),
				},
			},
		}, false
	}

	checkVirtueSiloWatch(altUserID)
	if len(client.SentMessages) != 2 {
		t.Fatalf("expected 2 DMs sent, got %d", len(client.SentMessages))
	}
	gotFallback := client.SentMessages[1].Content
	if !strings.Contains(gotFallback, fmt.Sprintf("for <@%s>", altUserID)) {
		t.Errorf("expected fallback notification to mention alt user ID, got: %s", gotFallback)
	}
}
