package watch

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

func setupContractSilosTestEnv(t *testing.T) {
	t.Helper()

	origFile := contractSiloWatchesFile
	contractSiloWatchesFile = filepath.Join(t.TempDir(), "contract_silo_watches.json")

	origResearches := ei.EggIncResearchesMap
	ei.EggIncResearchesMap = map[string]ei.EggResearches{
		"silo_capacity": {
			ID:             "silo_capacity",
			EffectType:     "additive",
			LevelsCompound: "additive",
			PerLevel:       6,
		},
	}

	origFetch := fetchContractCoopStatus
	origFetchUncached := fetchContractCoopStatusUncached

	t.Cleanup(func() {
		contractSiloMu.Lock()
		for k, tm := range contractSiloTimers {
			tm.Stop()
			delete(contractSiloTimers, k)
		}
		contractSiloMu.Unlock()

		contractSiloWatchesFile = origFile
		ei.EggIncResearchesMap = origResearches
		fetchContractCoopStatus = origFetch
		fetchContractCoopStatusUncached = origFetchUncached
	})
}

func makeTestCoopStatus(contributors []*ei.ContractCoopStatusResponse_ContributionInfo, secRem float64, allGoals bool) *ei.ContractCoopStatusResponse {
	status := ei.ContractCoopStatusResponse_NO_ERROR
	return &ei.ContractCoopStatusResponse{
		ResponseStatus:   &status,
		Contributors:     contributors,
		SecondsRemaining: &secRem,
		AllGoalsAchieved: &allGoals,
	}
}

func TestContractSilosPromptFlowAndClear(t *testing.T) {
	setupContractSilosTestEnv(t)

	userID := "100000000000000101"
	farmerstate.SetEggIncName(userID, "StdFarmerTest")

	u32 := func(v uint32) *uint32 { return &v }
	f64 := func(v float64) *float64 { return &v }
	sptr := func(v string) *string { return &v }

	now := time.Now()
	toggleEv := dctest.ComponentButtonEventWithUser("watch-csilos-toggle#test-contract#test-coop", userID)
	ackedBeforeFetch := false
	fetchContractCoopStatus = func(contractID, coopID, eeidOverride string) (*ei.ContractCoopStatusResponse, time.Time, string, error) {
		if toggleEv.LastResponse != nil && toggleEv.LastResponse.Ephemeral {
			ackedBeforeFetch = true
		}
		return makeTestCoopStatus([]*ei.ContractCoopStatusResponse_ContributionInfo{
			{
				UserName: sptr("StdFarmerTest"),
				FarmInfo: &ei.PlayerFarmInfo{
					PermitLevel: u32(ei.StandardPermitLevel),
					SilosOwned:  u32(2),
					EpicResearch: []*ei.Backup_ResearchItem{
						{Id: sptr("silo_capacity"), Level: u32(20)}, // 6h total
					},
					Timestamp: f64(-3600), // synced 1h ago -> expires in 5h
				},
			},
		}, 24*3600, false), now, "", nil
	}

	// 1. Toggle immediately acks ephemerally, then updates with offset menu (4 options) + 2 quiet menus + Confirm/Cancel
	HandleContractSilosToggle(toggleEv)
	if !ackedBeforeFetch {
		t.Fatalf("expected immediate ephemeral ack before fetchContractCoopStatus was called")
	}
	if toggleEv.LastResponse == nil || !toggleEv.LastResponse.Ephemeral {
		t.Fatalf("expected ephemeral response from HandleContractSilosToggle")
	}
	if len(toggleEv.LastResponse.Components) != 5 {
		t.Fatalf("expected 5 layout components, got %d", len(toggleEv.LastResponse.Components))
	}

	offsetRow := toggleEv.LastResponse.Components[1].(dc.ActionRow)
	offsetMenu := offsetRow.Components[0].(dc.SelectMenu)
	if len(offsetMenu.Options) != 4 {
		t.Fatalf("expected 4 offset options (5m, 10m, 15m, 30m), got %d", len(offsetMenu.Options))
	}
	wantVals := []string{"300", "600", "900", "1800"}
	for i, w := range wantVals {
		if offsetMenu.Options[i].Value != w {
			t.Errorf("offset option %d value = %q, want %q", i, offsetMenu.Options[i].Value, w)
		}
	}

	// 2. Select 10m offset (600s)
	offsetEv := dctest.ComponentSelectEventWithUser(offsetMenu.CustomID, userID, "600")
	HandleContractSilosSelect(offsetEv)
	if offsetEv.LastResponse == nil {
		t.Fatalf("expected update after selecting offset")
	}
	td := offsetEv.LastResponse.Components[0].(dc.TextDisplay)
	if !strings.Contains(td.Content, "10m") {
		t.Errorf("expected prompt text to reflect 10m offset, got: %s", td.Content)
	}

	// 3. Select Quiet Start = +2 and Quiet End = +10
	startMenu := offsetEv.LastResponse.Components[2].(dc.ActionRow).Components[0].(dc.SelectMenu)
	startEv := dctest.ComponentSelectEventWithUser(startMenu.CustomID, userID, "2")
	HandleContractSilosSelect(startEv)

	endMenu := startEv.LastResponse.Components[3].(dc.ActionRow).Components[0].(dc.SelectMenu)
	endEv := dctest.ComponentSelectEventWithUser(endMenu.CustomID, userID, "10")
	HandleContractSilosSelect(endEv)

	// 4. Confirm saves watch with OffsetSeconds = 600 and QuietHoursSet = true
	confirmBtn := endEv.LastResponse.Components[4].(dc.ActionRow).Components[0].(dc.Button)
	confirmEv := dctest.ComponentButtonEventWithUser(confirmBtn.CustomID, userID)
	HandleContractSilosConfirm(nil, confirmEv)

	contractSiloMu.Lock()
	list := loadContractSiloWatchesLocked()
	contractSiloMu.Unlock()
	if len(list) != 1 {
		t.Fatalf("expected 1 saved watch, got %d", len(list))
	}
	if list[0].OffsetSeconds != 600 || !list[0].QuietHoursSet || list[0].ContractID != "test-contract" || list[0].CoopID != "test-coop" {
		t.Fatalf("unexpected saved watch: %+v", list[0])
	}

	// 5. Clicking Silo Watch again prompts to clear the existing watch
	toggleAgainEv := dctest.ComponentButtonEventWithUser("watch-csilos-toggle#test-contract#test-coop", userID)
	HandleContractSilosToggle(toggleAgainEv)
	clearRow := toggleAgainEv.LastResponse.Components[1].(dc.ActionRow)
	clearBtn := clearRow.Components[0].(dc.Button)
	if clearBtn.Label != "Clear Watch" {
		t.Fatalf("expected Clear Watch button, got %+v", clearBtn)
	}

	clearEv := dctest.ComponentButtonEventWithUser(clearBtn.CustomID, userID)
	HandleContractSilosClear(clearEv)

	contractSiloMu.Lock()
	list = loadContractSiloWatchesLocked()
	contractSiloMu.Unlock()
	if len(list) != 0 {
		t.Fatalf("expected 0 watches after clear, got %d", len(list))
	}
}

func TestContractSilosPlayerDoesNotNeedWatch(t *testing.T) {
	setupContractSilosTestEnv(t)

	userID := "100000000000000102"
	farmerstate.SetEggIncName(userID, "ProFarmerTest")

	u32 := func(v uint32) *uint32 { return &v }
	f64 := func(v float64) *float64 { return &v }
	sptr := func(v string) *string { return &v }

	now := time.Now()
	fetchContractCoopStatus = func(contractID, coopID, eeidOverride string) (*ei.ContractCoopStatusResponse, time.Time, string, error) {
		return makeTestCoopStatus([]*ei.ContractCoopStatusResponse_ContributionInfo{
			{
				UserName: sptr("ProFarmerTest"),
				FarmInfo: &ei.PlayerFarmInfo{
					PermitLevel: u32(ei.ProPermitLevel),
					SilosOwned:  u32(10),
					EpicResearch: []*ei.Backup_ResearchItem{
						{Id: sptr("silo_capacity"), Level: u32(20)}, // 30h total
					},
					Timestamp: f64(-3600), // synced 1h ago -> expires in 29h
				},
			},
		}, 10*3600, false), now, "", nil // contract ends in 10h
	}

	toggleEv := dctest.ComponentButtonEventWithUser("watch-csilos-toggle#short-contract#pro-coop", userID)
	HandleContractSilosToggle(toggleEv)
	if toggleEv.LastResponse == nil || !toggleEv.LastResponse.Ephemeral {
		t.Fatalf("expected ephemeral response")
	}
	if len(toggleEv.LastResponse.Components) != 1 {
		t.Fatalf("expected 1 informational TextDisplay component, got %d", len(toggleEv.LastResponse.Components))
	}
	td := toggleEv.LastResponse.Components[0].(dc.TextDisplay)
	if !strings.Contains(td.Content, "don't need a Silo Watch") || !strings.Contains(td.Content, "Pro Permit") || !strings.Contains(td.Content, "10") {
		t.Errorf("expected explanation of why silos aren't needed, got: %s", td.Content)
	}
}

func TestContractSilosAutoCancelOnCompletion(t *testing.T) {
	setupContractSilosTestEnv(t)

	scheduleContractSiloTimer(contractSiloWatch{
		UserID:        "test-user-a",
		NotifyUserID:  "test-user-a",
		ContractID:    "contract-alpha",
		CoopID:        "coop-1",
		OffsetSeconds: 900,
		NextCheckAt:   time.Now().Add(time.Hour).Unix(),
	})
	scheduleContractSiloTimer(contractSiloWatch{
		UserID:        "test-user-b",
		NotifyUserID:  "test-user-b",
		ContractID:    "contract-alpha",
		CoopID:        "coop-1",
		OffsetSeconds: 1800,
		NextCheckAt:   time.Now().Add(time.Hour).Unix(),
	})
	scheduleContractSiloTimer(contractSiloWatch{
		UserID:        "test-user-c",
		NotifyUserID:  "test-user-c",
		ContractID:    "contract-beta",
		CoopID:        "coop-2",
		OffsetSeconds: 900,
		NextCheckAt:   time.Now().Add(time.Hour).Unix(),
	})

	// Completed coop status for contract-alpha/coop-1 should cancel both watches for that coop only.
	completedStatus := makeTestCoopStatus(nil, 0, true)
	UpdateContractSiloWatchesFromCoopStatus("contract-alpha", "coop-1", completedStatus)

	contractSiloMu.Lock()
	remaining := loadContractSiloWatchesLocked()
	contractSiloMu.Unlock()

	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining watch (for other coop), got %d: %+v", len(remaining), remaining)
	}
	if remaining[0].ContractID != "contract-beta" || remaining[0].CoopID != "coop-2" {
		t.Errorf("unexpected remaining watch: %+v", remaining[0])
	}
}

func TestContractSiloWatchNotificationIncludesAccountName(t *testing.T) {
	setupContractSilosTestEnv(t)

	mainUserID := "100000000000000203"
	altUserID := "100000000000000204"
	u32 := func(v uint32) *uint32 { return &v }
	f64 := func(v float64) *float64 { return &v }
	sptr := func(v string) *string { return &v }

	now := time.Now()
	fetchContractCoopStatusUncached = func(contractID, coopID, eeidOverride string) (*ei.ContractCoopStatusResponse, time.Time, string, error) {
		return makeTestCoopStatus([]*ei.ContractCoopStatusResponse_ContributionInfo{
			{
				UserName: sptr("ContractAltFarmer"),
				FarmInfo: &ei.PlayerFarmInfo{
					PermitLevel: u32(ei.StandardPermitLevel),
					SilosOwned:  u32(2),
					EpicResearch: []*ei.Backup_ResearchItem{
						{Id: sptr("silo_capacity"), Level: u32(20)}, // 6h total
					},
					Timestamp: f64(-5*3600 - 50*60), // expires in 10m
				},
			},
		}, 24*3600, false), now, "", nil
	}

	client := dctest.New()
	contractSiloMu.Lock()
	contractSiloClient = client
	contractSiloMu.Unlock()
	t.Cleanup(func() {
		contractSiloMu.Lock()
		contractSiloClient = nil
		contractSiloMu.Unlock()
	})

	rec := contractSiloWatch{
		UserID:        altUserID,
		NotifyUserID:  mainUserID,
		ChannelID:     "chan-contract",
		ContractID:    "contract-gamma",
		CoopID:        "coop-3",
		EggIncName:    "ContractAltFarmer",
		OffsetSeconds: 1800,
		NextCheckAt:   now.Add(-time.Minute).Unix(),
	}
	upsertContractSiloWatch(rec)

	checkContractSiloWatch(contractSiloKey(altUserID, rec.ContractID, rec.CoopID))

	if len(client.SentMessages) != 1 {
		t.Fatalf("expected 1 DM sent, got %d", len(client.SentMessages))
	}
	got := client.SentMessages[0].Content
	if !strings.Contains(got, "for **ContractAltFarmer**") {
		t.Errorf("expected notification to include account name 'for **ContractAltFarmer**', got: %s", got)
	}
}
