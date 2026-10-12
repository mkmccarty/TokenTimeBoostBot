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
		[]string{LBEarningsBonus, LBPrestigedEarningsBonus},
		"2026-10-12",
		0,
	)

	var entryEB, entryPrestigedEB *LBEntry
	for i := range entries {
		if entries[i].LBType == LBEarningsBonus {
			entryEB = &entries[i]
		}
		if entries[i].LBType == LBPrestigedEarningsBonus {
			entryPrestigedEB = &entries[i]
		}
	}

	if entryEB == nil {
		t.Fatalf("expected %s entry to be emitted", LBEarningsBonus)
	}
	if entryPrestigedEB == nil {
		t.Fatalf("expected %s entry to be emitted", LBPrestigedEarningsBonus)
	}

	if entryEB.Value <= entryPrestigedEB.Value {
		t.Errorf("expected Earnings Bonus (%f) to be strictly greater than Prestiged Earnings Bonus (%f) when pending TE > 0",
			entryEB.Value, entryPrestigedEB.Value)
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
		[]string{LBEarningsBonus, LBPrestigedEarningsBonus},
		"2026-10-12",
		0,
	)

	var entryEB2, entryPrestigedEB2 *LBEntry
	for i := range entriesNoPending {
		if entriesNoPending[i].LBType == LBEarningsBonus {
			entryEB2 = &entriesNoPending[i]
		}
		if entriesNoPending[i].LBType == LBPrestigedEarningsBonus {
			entryPrestigedEB2 = &entriesNoPending[i]
		}
	}

	if entryEB2 == nil || entryPrestigedEB2 == nil {
		t.Fatalf("expected both entries to be emitted when no pending TE")
	}

	if entryEB2.Value != entryPrestigedEB2.Value {
		t.Errorf("expected Earnings Bonus (%f) to equal Prestiged Earnings Bonus (%f) when pending TE == 0",
			entryEB2.Value, entryPrestigedEB2.Value)
	}
}

func TestEarningsBonusRegistryDefinitions(t *testing.T) {
	defEB, okEB := LBDefByKey(LBEarningsBonus)
	if !okEB {
		t.Fatalf("expected definition for %s", LBEarningsBonus)
	}
	if !strings.Contains(defEB.Description, "pending TE") {
		t.Errorf("expected %s description to mention pending TE, got %q", LBEarningsBonus, defEB.Description)
	}

	defPrestigedEB, okPrestigedEB := LBDefByKey(LBPrestigedEarningsBonus)
	if !okPrestigedEB {
		t.Fatalf("expected definition for %s", LBPrestigedEarningsBonus)
	}
	if !strings.Contains(defPrestigedEB.Description, "excludes pending TE") {
		t.Errorf("expected %s description to mention excluding pending TE, got %q", LBPrestigedEarningsBonus, defPrestigedEB.Description)
	}

	// Verify alias
	if resolveAlias("eb_prestiged") != LBPrestigedEarningsBonus {
		t.Errorf("expected alias eb_prestiged to resolve to %s, got %s", LBPrestigedEarningsBonus, resolveAlias("eb_prestiged"))
	}

	// Verify group_core contains both
	groupCoreMembers := ExpandConfigKey("group_core")
	foundEB, foundPrestigedEB := false, false
	for _, m := range groupCoreMembers {
		if m == LBEarningsBonus {
			foundEB = true
		}
		if m == LBPrestigedEarningsBonus {
			foundPrestigedEB = true
		}
	}
	if !foundEB {
		t.Errorf("expected %s in group_core members", LBEarningsBonus)
	}
	if !foundPrestigedEB {
		t.Errorf("expected %s in group_core members", LBPrestigedEarningsBonus)
	}
}
