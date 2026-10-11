package ei_test

import (
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

func TestPermitLimits(t *testing.T) {
	// Standard permit constants
	stdLimits := ei.StandardPermitLimits
	if stdLimits.PermitLevel != 0 {
		t.Errorf("Standard permit level: expected 0, got %d", stdLimits.PermitLevel)
	}
	if stdLimits.MaxArtifactSlots != 2 {
		t.Errorf("Standard permit max artifact slots: expected 2, got %d", stdLimits.MaxArtifactSlots)
	}
	if stdLimits.MaxActiveBoosts != 2 {
		t.Errorf("Standard permit max active boosts: expected 2, got %d", stdLimits.MaxActiveBoosts)
	}
	if stdLimits.MaxSilos != 2 {
		t.Errorf("Standard permit max silos: expected 2, got %d", stdLimits.MaxSilos)
	}
	if stdLimits.MaxFuelTankLevel != 0 {
		t.Errorf("Standard permit max fuel tank level: expected 0, got %d", stdLimits.MaxFuelTankLevel)
	}
	if stdLimits.MaxFuelTankCapacity != 2e9 {
		t.Errorf("Standard permit max fuel tank capacity: expected 2B (2e9), got %e", stdLimits.MaxFuelTankCapacity)
	}

	// Pro permit constants
	proLimits := ei.ProPermitLimits
	if proLimits.PermitLevel != 1 {
		t.Errorf("Pro permit level: expected 1, got %d", proLimits.PermitLevel)
	}
	if proLimits.MaxArtifactSlots != 4 {
		t.Errorf("Pro permit max artifact slots: expected 4, got %d", proLimits.MaxArtifactSlots)
	}
	if proLimits.MaxActiveBoosts != 5 {
		t.Errorf("Pro permit max active boosts: expected 5, got %d", proLimits.MaxActiveBoosts)
	}
	if proLimits.MaxSilos != 10 {
		t.Errorf("Pro permit max silos: expected 10, got %d", proLimits.MaxSilos)
	}
	if proLimits.MaxFuelTankLevel != 7 {
		t.Errorf("Pro permit max fuel tank level: expected 7, got %d", proLimits.MaxFuelTankLevel)
	}
	if proLimits.MaxFuelTankCapacity != 500e12 {
		t.Errorf("Pro permit max fuel tank capacity: expected 500T (500e12), got %e", proLimits.MaxFuelTankCapacity)
	}

	// Test GetPermitLimitsByLevel
	if got := ei.GetPermitLimitsByLevel(0); got.MaxArtifactSlots != 2 || got.MaxSilos != 2 {
		t.Errorf("GetPermitLimitsByLevel(0): unexpected limits %#v", got)
	}
	if got := ei.GetPermitLimitsByLevel(1); got.MaxArtifactSlots != 4 || got.MaxSilos != 10 {
		t.Errorf("GetPermitLimitsByLevel(1): unexpected limits %#v", got)
	}
}

func TestPermitHelpersWithBackups(t *testing.T) {
	makerPro := ei.NewBackupMaker("EI1234567890123456", "proUser")
	makerPro.SetPermitLevel(1)
	backupPro := makerPro.GetBackup()

	if !ei.HasProPermit(backupPro) {
		t.Errorf("Expected HasProPermit true for pro backup")
	}
	if got := ei.GetMaxEquippedArtifacts(backupPro); got != 4 {
		t.Errorf("Pro permit max artifacts: expected 4, got %d", got)
	}
	if got := ei.GetMaxActiveBoosts(backupPro); got != 5 {
		t.Errorf("Pro permit max active boosts: expected 5, got %d", got)
	}
	if got := ei.GetMaxSilos(backupPro); got != 10 {
		t.Errorf("Pro permit max silos: expected 10, got %d", got)
	}
	if got := ei.GetEffectiveSilos(backupPro, 8); got != 8 {
		t.Errorf("Pro permit effective silos (8 owned): expected 8, got %d", got)
	}
	if got := ei.GetEffectiveSilos(backupPro, 12); got != 10 {
		t.Errorf("Pro permit effective silos clamped: expected 10, got %d", got)
	}

	makerStd := ei.NewBackupMaker("EI1234567890123456", "stdUser")
	makerStd.SetPermitLevel(0)
	backupStd := makerStd.GetBackup()

	if ei.HasProPermit(backupStd) {
		t.Errorf("Expected HasProPermit false for standard backup")
	}
	if got := ei.GetMaxEquippedArtifacts(backupStd); got != 2 {
		t.Errorf("Standard permit max artifacts: expected 2, got %d", got)
	}
	if got := ei.GetMaxActiveBoosts(backupStd); got != 2 {
		t.Errorf("Standard permit max active boosts: expected 2, got %d", got)
	}
	if got := ei.GetMaxSilos(backupStd); got != 2 {
		t.Errorf("Standard permit max silos: expected 2, got %d", got)
	}
	if got := ei.GetEffectiveSilos(backupStd, 1); got != 1 {
		t.Errorf("Standard permit effective silos (1 owned): expected 1, got %d", got)
	}
	if got := ei.GetEffectiveSilos(backupStd, 8); got != 2 {
		t.Errorf("Standard permit effective silos clamped: expected 2, got %d", got)
	}
}
