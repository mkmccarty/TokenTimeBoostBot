package ei

import (
	"testing"
)

func TestBackupMaker_Colleggtibles(t *testing.T) {
	// Ensure the CustomEggMap is populated with test fixture data
	setupCustomEggMap()

	maker := NewBackupMaker("EI1234567890123456", "TestUser")

	// Add the max tier of every custom egg present in the map
	for id := range CustomEggMap {
		maker.AddMaxColleggtibleContract(id)
	}

	backup := maker.GetBackup()
	contracts := backup.GetContracts()

	if contracts == nil {
		t.Fatalf("Expected contracts to be initialized")
	}

	archive := contracts.GetArchive()
	if len(archive) != len(CustomEggMap) {
		t.Errorf("Expected %d contracts in archive, got %d", len(CustomEggMap), len(archive))
	}

	// Verify that every egg ID from the map is present in the backup data
	foundEggs := make(map[string]bool)
	for _, localContract := range archive {
		if c := localContract.GetContract(); c != nil {
			eggID := c.GetCustomEggId()
			if eggID != "" {
				foundEggs[eggID] = true
			}
			if c.GetEgg() != Egg_CUSTOM_EGG {
				t.Errorf("Expected Egg type to be Egg_CUSTOM_EGG, got %v", c.GetEgg())
			}
		}
	}

	for id := range CustomEggMap {
		if !foundEggs[id] {
			t.Errorf("Expected custom egg %s to be in the backup", id)
		}
	}

	// Calculate buffs and ensure they reflect the additions using ei_colleggtibles logic
	buffs := GetColleggtibleBuffs(contracts)

	if buffs.AwayEarnings <= 1.0 {
		t.Errorf("Expected AwayEarnings buff to be > 1.0, got %f", buffs.AwayEarnings)
	}
	if buffs.ELR <= 1.0 {
		t.Errorf("Expected ELR buff to be > 1.0, got %f", buffs.ELR)
	}
	if buffs.SR <= 1.0 {
		t.Errorf("Expected SR buff to be > 1.0, got %f", buffs.SR)
	}
}
