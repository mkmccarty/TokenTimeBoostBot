package leaderboard

import (
	"strings"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

func TestRunCalculators_EarningsBonusPendingVsPrestiged(t *testing.T) {
	maker := ei.NewBackupMaker("EI1234567890123456", "TestFarmer")
	backup := maker.GetBackup()

	// 1. With pending TE (eggs delivered passed tiers, but EoV earned = 0)
	backup.Virtue = &ei.Backup_Virtue{
		EovEarned:     []uint32{0},
		EggsDelivered: []float64{1e18},
	}

	entries := RunCalculators(
		"user-test-1",
		backup,
		nil,
		[]string{LBEarningsBonus, LBFutureEarningsBonus},
		"2026-10-12",
		0,
	)

	var entryEB, entryFutureEB *LBEntry
	for i := range entries {
		if entries[i].LBType == LBEarningsBonus {
			entryEB = &entries[i]
		}
		if entries[i].LBType == LBFutureEarningsBonus {
			entryFutureEB = &entries[i]
		}
	}

	if entryEB == nil {
		t.Fatalf("expected %s entry to be emitted", LBEarningsBonus)
	}
	if entryFutureEB == nil {
		t.Fatalf("expected %s entry to be emitted", LBFutureEarningsBonus)
	}

	if entryFutureEB.Value <= entryEB.Value {
		t.Errorf("expected Future Earnings Bonus (%f) to be strictly greater than Earnings Bonus (%f) when pending TE > 0",
			entryFutureEB.Value, entryEB.Value)
	}

	// 2. Without pending TE (earned TE matches tiers passed)
	tiers := ei.CountTruthEggTiersPassed(1e18)
	backup.Virtue = &ei.Backup_Virtue{
		EovEarned:     []uint32{tiers},
		EggsDelivered: []float64{1e18},
	}

	entriesNoPending := RunCalculators(
		"user-test-1",
		backup,
		nil,
		[]string{LBEarningsBonus, LBFutureEarningsBonus},
		"2026-10-12",
		0,
	)

	var entryEB2, entryFutureEB2 *LBEntry
	for i := range entriesNoPending {
		if entriesNoPending[i].LBType == LBEarningsBonus {
			entryEB2 = &entriesNoPending[i]
		}
		if entriesNoPending[i].LBType == LBFutureEarningsBonus {
			entryFutureEB2 = &entriesNoPending[i]
		}
	}

	if entryEB2 == nil || entryFutureEB2 == nil {
		t.Fatalf("expected both entries to be emitted when no pending TE")
	}

	if entryEB2.Value != entryFutureEB2.Value {
		t.Errorf("expected Earnings Bonus (%f) to equal Future Earnings Bonus (%f) when pending TE == 0",
			entryEB2.Value, entryFutureEB2.Value)
	}
}

func TestEarningsBonusRegistryDefinitions(t *testing.T) {
	defEB, okEB := LBDefByKey(LBEarningsBonus)
	if !okEB {
		t.Fatalf("expected definition for %s", LBEarningsBonus)
	}
	if defEB.DisplayName != "Earnings Bonus" {
		t.Errorf("expected %s display name to be 'Earnings Bonus', got %q", LBEarningsBonus, defEB.DisplayName)
	}
	if !strings.Contains(defEB.Description, "excludes pending TE") {
		t.Errorf("expected %s description to mention excluding pending TE, got %q", LBEarningsBonus, defEB.Description)
	}

	defFutureEB, okFutureEB := LBDefByKey(LBFutureEarningsBonus)
	if !okFutureEB {
		t.Fatalf("expected definition for %s", LBFutureEarningsBonus)
	}
	if defFutureEB.DisplayName != "Future Earnings Bonus" {
		t.Errorf("expected %s display name to be 'Future Earnings Bonus', got %q", LBFutureEarningsBonus, defFutureEB.DisplayName)
	}
	if !strings.Contains(defFutureEB.Description, "includes pending TE") {
		t.Errorf("expected %s description to mention including pending TE, got %q", LBFutureEarningsBonus, defFutureEB.Description)
	}

	// Verify aliases
	if resolveAlias("prestiged_eb") != LBEarningsBonus {
		t.Errorf("expected alias prestiged_eb to resolve to %s, got %s", LBEarningsBonus, resolveAlias("prestiged_eb"))
	}
	if resolveAlias("eb_prestiged") != LBEarningsBonus {
		t.Errorf("expected alias eb_prestiged to resolve to %s, got %s", LBEarningsBonus, resolveAlias("eb_prestiged"))
	}
	if resolveAlias("pending_eb") != LBFutureEarningsBonus {
		t.Errorf("expected alias pending_eb to resolve to %s, got %s", LBFutureEarningsBonus, resolveAlias("pending_eb"))
	}
	if resolveAlias("eb_pending") != LBFutureEarningsBonus {
		t.Errorf("expected alias eb_pending to resolve to %s, got %s", LBFutureEarningsBonus, resolveAlias("eb_pending"))
	}
	if resolveAlias("eb_future") != LBFutureEarningsBonus {
		t.Errorf("expected alias eb_future to resolve to %s, got %s", LBFutureEarningsBonus, resolveAlias("eb_future"))
	}

	// Verify group_core contains both
	groupCoreMembers := ExpandConfigKey("group_core")
	foundEB, foundFutureEB := false, false
	for _, m := range groupCoreMembers {
		if m == LBEarningsBonus {
			foundEB = true
		}
		if m == LBFutureEarningsBonus {
			foundFutureEB = true
		}
	}
	if !foundEB {
		t.Errorf("expected %s in group_core members", LBEarningsBonus)
	}
	if !foundFutureEB {
		t.Errorf("expected %s in group_core members", LBFutureEarningsBonus)
	}
}
