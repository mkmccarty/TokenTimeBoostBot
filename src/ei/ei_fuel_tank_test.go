package ei_test

import (
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

func TestFuelTankLevels(t *testing.T) {
	if len(ei.FuelTankLevels) != 8 {
		t.Fatalf("Expected 8 fuel tank levels, got %d", len(ei.FuelTankLevels))
	}

	expectedCapacities := []float64{2e9, 200e9, 10e12, 100e12, 200e12, 300e12, 400e12, 500e12}
	for i, exp := range expectedCapacities {
		info := ei.GetFuelTankLevelInfo(i)
		if info.Level != i {
			t.Errorf("Level %d: expected level %d, got %d", i, i, info.Level)
		}
		if info.Capacity != exp {
			t.Errorf("Level %d: expected capacity %e, got %e", i, exp, info.Capacity)
		}
		capVal := ei.GetFuelTankCapacity(i)
		if capVal != exp {
			t.Errorf("GetFuelTankCapacity(%d): expected %e, got %e", i, exp, capVal)
		}
	}

	// Clamping tests
	if got := ei.GetFuelTankCapacity(-1); got != 2e9 {
		t.Errorf("Expected clamped level -1 to return 2e9, got %e", got)
	}
	if got := ei.GetFuelTankCapacity(100); got != 500e12 {
		t.Errorf("Expected clamped level 100 to return 500e12, got %e", got)
	}

	// Output format checks
	for i, c := range expectedCapacities {
		formatted := ei.FormatEIValue(c, map[string]any{"decimals": 0, "trim": true})
		switch i {
		case 0:
			if formatted != "2B" {
				t.Errorf("Level 0 formatted: expected 2B, got %s", formatted)
			}
		case 1:
			if formatted != "200B" {
				t.Errorf("Level 1 formatted: expected 200B, got %s", formatted)
			}
		case 2:
			if formatted != "10T" {
				t.Errorf("Level 2 formatted: expected 10T, got %s", formatted)
			}
		case 3:
			if formatted != "100T" {
				t.Errorf("Level 3 formatted: expected 100T, got %s", formatted)
			}
		case 4:
			if formatted != "200T" {
				t.Errorf("Level 4 formatted: expected 200T, got %s", formatted)
			}
		case 5:
			if formatted != "300T" {
				t.Errorf("Level 5 formatted: expected 300T, got %s", formatted)
			}
		case 6:
			if formatted != "400T" {
				t.Errorf("Level 6 formatted: expected 400T, got %s", formatted)
			}
		case 7:
			if formatted != "500T" {
				t.Errorf("Level 7 formatted: expected 500T, got %s", formatted)
			}
		}
	}
}

func TestStandardPermitFuelTank(t *testing.T) {
	// Pro permit with tank level 7
	makerPro := ei.NewBackupMaker("EI1234567890123456", "proUser")
	makerPro.SetPermitLevel(1)
	backupPro := makerPro.GetBackup()
	lvl7 := uint32(7)
	backupPro.Artifacts.TankLevel = &lvl7

	if !ei.HasProPermit(backupPro) {
		t.Errorf("Expected HasProPermit true for permit level 1")
	}
	if got := ei.GetPlayerFuelTankLevel(backupPro); got != 7 {
		t.Errorf("Pro permit: expected tank level 7, got %d", got)
	}
	if got := ei.GetPlayerFuelTankCapacity(backupPro); got != 500e12 {
		t.Errorf("Pro permit: expected tank capacity 500T (500e12), got %e", got)
	}

	// Standard permit (permit level 0) with tank level 7 set in artifacts
	makerStd := ei.NewBackupMaker("EI1234567890123456", "stdUser")
	makerStd.SetPermitLevel(0)
	backupStd := makerStd.GetBackup()
	backupStd.Artifacts.TankLevel = &lvl7

	if ei.HasProPermit(backupStd) {
		t.Errorf("Expected HasProPermit false for permit level 0")
	}
	// Standard permit players must be limited to level 0 (2B capacity)
	if got := ei.GetPlayerFuelTankLevel(backupStd); got != 0 {
		t.Errorf("Standard permit: expected tank level 0, got %d", got)
	}
	if got := ei.GetPlayerFuelTankCapacity(backupStd); got != 2e9 {
		t.Errorf("Standard permit: expected tank capacity 2B (2e9), got %e", got)
	}
}
