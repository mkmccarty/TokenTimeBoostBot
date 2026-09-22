package ei

import (
	"os"
	"strings"
	"testing"
)

func TestGetRandomComplaintSamples(t *testing.T) {
	// When TokenComplaints is empty, returns fallback samples
	tokenComplaintsMutex.Lock()
	original := TokenComplaints
	TokenComplaints = nil
	tokenComplaintsMutex.Unlock()

	samples := GetRandomComplaintSamples(4)
	if len(samples) != 4 {
		t.Fatalf("expected 4 samples, got %d", len(samples))
	}
	for _, s := range samples {
		if !strings.Contains(s, playerToken) {
			t.Errorf("sample %q does not contain player token", s)
		}
	}

	// Restore and load mock complaints
	tokenComplaintsMutex.Lock()
	TokenComplaints = []string{
		"[player] complaint 1",
		"[player] complaint 2",
		"[player] complaint 3",
		"[player] complaint 4",
		"[player] complaint 5",
		tokenComplaintsResortFlag,
	}
	tokenComplaintsMutex.Unlock()

	defer func() {
		tokenComplaintsMutex.Lock()
		TokenComplaints = original
		tokenComplaintsMutex.Unlock()
	}()

	samples2 := GetRandomComplaintSamples(3)
	if len(samples2) != 3 {
		t.Fatalf("expected 3 samples, got %d", len(samples2))
	}
	for _, s := range samples2 {
		if s == tokenComplaintsResortFlag {
			t.Errorf("sample contained resort flag: %q", s)
		}
		if !strings.Contains(s, playerToken) {
			t.Errorf("sample %q does not contain player token", s)
		}
	}
}

func TestForceLoadTokenComplaints(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := tmpDir + "/complaints.json"

	data := `{"token_complaints": ["[player] wants tokens", "[player] is waiting"]}`
	if err := os.WriteFile(filePath, []byte(data), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	count, err := ForceLoadTokenComplaints(filePath)
	if err != nil {
		t.Fatalf("ForceLoadTokenComplaints failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count = 2, got %d", count)
	}

	// Test non-existent file
	_, err = ForceLoadTokenComplaints(tmpDir + "/does-not-exist.json")
	if err == nil {
		t.Errorf("expected error for non-existent file, got nil")
	}
}
