package tasks

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
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

func TestExtractContractIDFromBannerFileName(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"2021-year-hangover-ba.png", "2021-year-hangover"},
		{"ac-2019-ba-1457709786597560515.png", "ac-2019"},
		{"catch-up-work-2026-ba-user1_guild1.png", "catch-up-work-2026"},
		{"moon-base-2023-ba.png", "moon-base-2023"},
		{".CustomCostumes6p-ba.png", ".customcostumes6p"},
		{"broken-bones-2022-bf-space.png", "broken-bones-2022"},
		{"b-old-contract.png", "old-contract"},
		{"not-a-banner.png", ""},
		{"banner.png", ""},
	}

	for _, tt := range tests {
		got := extractContractIDFromBannerFileName(tt.filename)
		if got != tt.want {
			t.Errorf("extractContractIDFromBannerFileName(%q) = %q, want %q", tt.filename, got, tt.want)
		}
	}
}

func TestPruneOldGeneratedBanners(t *testing.T) {
	tmpDir := t.TempDir()
	origOut := config.BannerOutputPath
	config.BannerOutputPath = tmpDir
	defer func() { config.BannerOutputPath = origOut }()

	// Mock periodicals contracts in ei
	ei.EggIncContractsMutex.Lock()
	savedEiContracts := ei.EggIncContracts
	savedEiAll := ei.EggIncContractsAll
	ei.EggIncContracts = []ei.EggIncContract{
		{
			ID:         "contract-unexpired",
			ValidUntil: time.Now().UTC().Add(48 * time.Hour),
		},
		{
			ID:         "contract-expired-periodicals",
			ValidUntil: time.Now().UTC().Add(-48 * time.Hour),
		},
	}
	ei.EggIncContractsAll = map[string]ei.EggIncContract{
		"contract-unexpired": {
			ID:         "contract-unexpired",
			ValidUntil: time.Now().UTC().Add(48 * time.Hour),
		},
		"contract-expired-periodicals": {
			ID:         "contract-expired-periodicals",
			ValidUntil: time.Now().UTC().Add(-48 * time.Hour),
		},
	}
	ei.EggIncContractsMutex.Unlock()
	defer func() {
		ei.EggIncContractsMutex.Lock()
		ei.EggIncContracts = savedEiContracts
		ei.EggIncContractsAll = savedEiAll
		ei.EggIncContractsMutex.Unlock()
	}()

	// Mock Boost Bot contracts
	boost.ContractsMutex.Lock()
	savedContracts := boost.Contracts
	boost.Contracts = map[string]*boost.Contract{
		"hash-1": {
			ContractID: "contract-active-bb",
			State:      boost.ContractStateFastrun,
		},
		"hash-2": {
			ContractID: "contract-expired-periodicals",
			State:      boost.ContractStateArchive, // Archived in BB
		},
	}
	boost.ContractsMutex.Unlock()
	defer func() {
		boost.ContractsMutex.Lock()
		boost.Contracts = savedContracts
		boost.ContractsMutex.Unlock()
	}()

	// Create test files
	keptFiles := []string{
		filepath.Join(tmpDir, "contract-unexpired-ba.png"),
		filepath.Join(tmpDir, "contract-unexpired-bc-guild1.png"),
		filepath.Join(tmpDir, "contract-active-bb-ba.png"),
		filepath.Join(tmpDir, "contract-active-bb-bf-user1_guild1.png"),
	}
	prunedFiles := []string{
		filepath.Join(tmpDir, "contract-expired-periodicals-ba.png"),
		filepath.Join(tmpDir, "contract-expired-periodicals-bc-guild1.png"),
		filepath.Join(tmpDir, "b-contract-expired-periodicals.png"),
	}
	ignoredFiles := []string{
		filepath.Join(tmpDir, "unrelated.txt"),
	}

	for _, f := range append(append(keptFiles, prunedFiles...), ignoredFiles...) {
		if err := os.WriteFile(f, []byte("data"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}
	// Create a subdirectory that should be ignored
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	prunedCount := PruneOldGeneratedBanners()
	if prunedCount != len(prunedFiles) {
		t.Fatalf("expected pruned count = %d, got %d", len(prunedFiles), prunedCount)
	}

	for _, f := range keptFiles {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("expected kept file %s to exist, but stat returned: %v", f, err)
		}
	}

	for _, f := range prunedFiles {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("expected pruned file %s to be deleted, but it exists", f)
		}
	}

	for _, f := range ignoredFiles {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("expected ignored file %s to exist, but stat returned: %v", f, err)
		}
	}

	if _, err := os.Stat(subDir); err != nil {
		t.Errorf("expected subDir to exist, but stat returned: %v", err)
	}
}
