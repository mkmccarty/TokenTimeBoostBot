package tasks

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRefreshTokenComplaints(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "token-complaints.json")

	validPayload := `{
		"token_complaints": [
			"[player] complaint 1",
			"[player] complaint 2"
		]
	}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", "test-complaints-etag")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(validPayload))
	}))
	defer ts.Close()

	origURL := eggIncTokenComplaintsURL
	origFile := eggIncTokenComplaintsFile
	eggIncTokenComplaintsURL = ts.URL
	eggIncTokenComplaintsFile = testFile
	defer func() {
		eggIncTokenComplaintsURL = origURL
		eggIncTokenComplaintsFile = origFile
	}()

	count, err := RefreshTokenComplaints()
	if err != nil {
		t.Fatalf("RefreshTokenComplaints failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count = 2, got %d", count)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if len(content) == 0 {
		t.Errorf("written file is empty")
	}
}

func TestRefreshTokenComplaints_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "token-complaints.json")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not valid json"))
	}))
	defer ts.Close()

	origURL := eggIncTokenComplaintsURL
	origFile := eggIncTokenComplaintsFile
	eggIncTokenComplaintsURL = ts.URL
	eggIncTokenComplaintsFile = testFile
	defer func() {
		eggIncTokenComplaintsURL = origURL
		eggIncTokenComplaintsFile = origFile
	}()

	_, err := RefreshTokenComplaints()
	if err == nil {
		t.Fatalf("expected error for invalid JSON, got nil")
	}

	// Ensure bad payload was not written to disk
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Errorf("file should not exist after failed validation")
	}
}

func TestRefreshStatusMessages(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "status-messages.json")

	validPayload := `{
		"status_messages": [
			"Message 1",
			"Message 2",
			"Message 3"
		]
	}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", "test-status-etag")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(validPayload))
	}))
	defer ts.Close()

	origURL := eggIncStatusMessagesURL
	origFile := eggIncStatusMessagesFile
	eggIncStatusMessagesURL = ts.URL
	eggIncStatusMessagesFile = testFile
	defer func() {
		eggIncStatusMessagesURL = origURL
		eggIncStatusMessagesFile = origFile
	}()

	count, err := RefreshStatusMessages()
	if err != nil {
		t.Fatalf("RefreshStatusMessages failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected count = 3, got %d", count)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if len(content) == 0 {
		t.Errorf("written file is empty")
	}
}

func TestRefreshStatusMessages_HTTPError(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "status-messages.json")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	origURL := eggIncStatusMessagesURL
	origFile := eggIncStatusMessagesFile
	eggIncStatusMessagesURL = ts.URL
	eggIncStatusMessagesFile = testFile
	defer func() {
		eggIncStatusMessagesURL = origURL
		eggIncStatusMessagesFile = origFile
	}()

	_, err := RefreshStatusMessages()
	if err == nil {
		t.Fatalf("expected error on HTTP 500, got nil")
	}
}
