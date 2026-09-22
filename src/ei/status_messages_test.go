package ei

import (
	"os"
	"testing"
)

func TestForceLoadStatusMessages(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := tmpDir + "/status.json"

	data := `{"status_messages": ["Status one", "Status two", "Status three"]}`
	if err := os.WriteFile(filePath, []byte(data), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	count, err := ForceLoadStatusMessages(filePath)
	if err != nil {
		t.Fatalf("ForceLoadStatusMessages failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected count = 3, got %d", count)
	}

	// Test non-existent file
	_, err = ForceLoadStatusMessages(tmpDir + "/does-not-exist.json")
	if err == nil {
		t.Errorf("expected error for non-existent file, got nil")
	}
}
