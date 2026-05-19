package ei

import (
	"os"
	"reflect"
	"testing"
)

func TestThematicComplaintsSaveLoad(t *testing.T) {
	// Backup original file if it exists
	const filepath = "ttbb-data/ei-complaints.json"
	var backupData []byte
	hasBackup := false
	if _, err := os.Stat(filepath); err == nil {
		var rerr error
		backupData, rerr = os.ReadFile(filepath)
		if rerr != nil {
			t.Fatalf("failed to read original file for backup: %v", rerr)
		}
		hasBackup = true
		// Delete it to start fresh
		_ = os.Remove(filepath)
	}
	defer func() {
		if hasBackup {
			_ = os.WriteFile(filepath, backupData, 0644)
		} else {
			_ = os.Remove(filepath)
		}
		// Reset in-memory cache
		thematicComplaintsMutex.Lock()
		ThematicComplaintsMap = nil
		thematicComplaintsMutex.Unlock()
	}()

	// Clear in-memory cache for test
	thematicComplaintsMutex.Lock()
	ThematicComplaintsMap = nil
	thematicComplaintsMutex.Unlock()

	testData := map[string][]string{
		"contract-1": {"[player] got squished by a space egg.", "[player] wants tokens, got none."},
		"contract-2": {"Is [player] even playing?", "[player] is crying in the corner."},
	}

	// Test Save
	err := SaveThematicComplaints(testData)
	if err != nil {
		t.Fatalf("SaveThematicComplaints failed: %v", err)
	}

	// Test Load (should hit in-memory cache)
	loaded, err := LoadThematicComplaints()
	if err != nil {
		t.Fatalf("LoadThematicComplaints failed: %v", err)
	}
	if !reflect.DeepEqual(loaded, testData) {
		t.Errorf("loaded data does not match saved data: got %v, want %v", loaded, testData)
	}

	// Clear in-memory cache to force disk read
	thematicComplaintsMutex.Lock()
	ThematicComplaintsMap = nil
	thematicComplaintsMutex.Unlock()

	// Test Load (should hit disk)
	loadedFromDisk, err := LoadThematicComplaints()
	if err != nil {
		t.Fatalf("LoadThematicComplaints from disk failed: %v", err)
	}
	if !reflect.DeepEqual(loadedFromDisk, testData) {
		t.Errorf("loaded from disk data does not match saved data: got %v, want %v", loadedFromDisk, testData)
	}
}
