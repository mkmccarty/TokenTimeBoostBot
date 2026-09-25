package ei

import (
	"math"
	"testing"
)

func TestGetEarningsBonus(t *testing.T) {
	maker := NewBackupMaker("EI1234567890123456", "TestUser")
	backup := maker.GetBackup()

	game := backup.GetGame()
	soulEggs := game.GetSoulEggsD()
	prophecyEggs := game.GetEggsOfProphecy()

	// Calculate total EoV earned from JSON: [20, 20, 21, 20, 20]
	eov := 20.0 + 20.0 + 21.0 + 20.0 + 20.0

	actualEB := GetEarningsBonus(backup, eov)

	// Re-calculate expected EB to assert validity of the implementation logic
	expectedEB := soulEggs * 1.5 * math.Pow(1.1, float64(prophecyEggs)) // 1.5 = 0.1 base + 140 level * 0.01; 1.1 = 1 + (0.05 base + 5 level * 0.01)
	expectedEB = expectedEB * math.Pow(1.01, eov) * 100

	// Allow a small margin of error for floating point calculations
	if margin := expectedEB * 1e-9; math.Abs(actualEB-expectedEB) > margin {
		t.Errorf("GetEarningsBonus() = %e, want %e", actualEB, expectedEB)
	}
}

func TestFarmerRoles(t *testing.T) {
	if len(FarmerRoles) != 52 {
		t.Fatalf("expected 52 farmer roles, got %d", len(FarmerRoles))
	}
	if FarmerRoles[0].Name != "Farmer" {
		t.Errorf("expected role 0 to be Farmer, got %s", FarmerRoles[0].Name)
	}
	if FarmerRoles[3].Name != "Kilofarmer" {
		t.Errorf("expected role 3 to be Kilofarmer, got %s", FarmerRoles[3].Name)
	}
	if FarmerRoles[6].Name != "Megafarmer" {
		t.Errorf("expected role 6 to be Megafarmer, got %s", FarmerRoles[6].Name)
	}
	if FarmerRoles[9].Name != "Gigafarmer" {
		t.Errorf("expected role 9 to be Gigafarmer, got %s", FarmerRoles[9].Name)
	}
	if FarmerRoles[18].Name != "Exafarmer" {
		t.Errorf("expected role 18 to be Exafarmer, got %s", FarmerRoles[18].Name)
	}
	if FarmerRoles[24].Name != "Yottafarmer" {
		t.Errorf("expected role 24 to be Yottafarmer, got %s", FarmerRoles[24].Name)
	}
	if FarmerRoles[51].Name != "Infinifarmer" {
		t.Errorf("expected role 51 to be Infinifarmer, got %s", FarmerRoles[51].Name)
	}

	tests := []struct {
		ratio    float64
		expected string
		oom      int
	}{
		{0.0, "Farmer", 0},
		{0.5, "Farmer", 0},
		{1.0, "Farmer", 0},
		{9.9, "Farmer", 0},
		{10.0, "Farmer II", 1},
		{100.0, "Farmer III", 2},
		{1_000.0, "Kilofarmer", 3},
		{1_000_000.0, "Megafarmer", 6},
		{1e18, "Exafarmer", 18},
		{1e24, "Yottafarmer", 24},
		{1e51, "Infinifarmer", 51},
		{1e55, "Infinifarmer", 55},
	}

	for _, tc := range tests {
		role := EarningBonusToFarmerRole(tc.ratio)
		if role.Name != tc.expected {
			t.Errorf("EarningBonusToFarmerRole(%e) = %s; want %s", tc.ratio, role.Name, tc.expected)
		}
		if role.OOM != tc.oom {
			t.Errorf("EarningBonusToFarmerRole(%e) OOM = %d; want %d", tc.ratio, role.OOM, tc.oom)
		}
	}

	// Test percent helper
	rolePct := EarningBonusPercentToFarmerRole(100_000.0) // 100k% = 1,000 ratio -> Kilofarmer
	if rolePct.Name != "Kilofarmer" {
		t.Errorf("EarningBonusPercentToFarmerRole(100_000) = %s, want Kilofarmer", rolePct.Name)
	}
}
