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
