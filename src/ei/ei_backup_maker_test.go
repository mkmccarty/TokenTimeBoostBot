package ei

import (
	"testing"

	"google.golang.org/protobuf/proto"
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

func TestBackupMaker_VirtueIHR(t *testing.T) {
	// Only run this test if mockBackupFile is set to verify Virtue IHR calculations using mock data
	if mockBackupFile == "" {
		t.Skip("Skipping TestBackupMaker_VirtueIHR as mockBackupFile is not configured")
	}

	maker := NewBackupMaker("EI123456", "TestUser")
	backup := maker.GetBackup()

	// 1. Evaluate baseline IHR from the loaded mock data (should be 1.0 since no virtue artifacts are equipped)
	baselineArtifacts := GetActiveVirtueArtifacts(backup)
	baselineBuffs := GetArtifactBuffs(baselineArtifacts)
	if baselineBuffs.IHR != 1.0 {
		t.Errorf("Expected baseline virtue IHR to be 1.0, got %f", baselineBuffs.IHR)
	}

	// 2. Programmatically add a Metronome (T4C) to Virtue inventory and equip it, leaving Chalice empty
	itemID, _ := maker.AddArtifact(ArtifactSpec_QUANTUM_METRONOME, ArtifactSpec_GREATER, ArtifactSpec_COMMON, 1, true)

	virtueDB := backup.GetArtifactsDb().GetVirtueAfxDb()
	if virtueDB == nil {
		t.Fatalf("Expected VirtueAfxDb to not be nil")
	}

	virtueDB.ActiveArtifacts = &ArtifactsDB_ActiveArtifactSet{
		Slots: []*ActiveArtifactSlot{
			{
				Occupied: proto.Bool(true),
				ItemId:   proto.Uint64(itemID),
			},
		},
	}

	// 3. Evaluate the rates; ELR should be 1.25, while IHR must remain exactly 1.0
	activeArtifacts := GetActiveVirtueArtifacts(backup)
	buffs := GetArtifactBuffs(activeArtifacts)

	if buffs.ELR != 1.25 {
		t.Errorf("Expected ELR to be 1.25 from Metronome, got %f", buffs.ELR)
	}
	if buffs.IHR != 1.0 {
		t.Errorf("Expected IHR to remain 1.0 when Chalice is missing, got %f", buffs.IHR)
	}
}
