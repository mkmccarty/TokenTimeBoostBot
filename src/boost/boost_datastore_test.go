package boost

import (
	"database/sql"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	_ "modernc.org/sqlite"
)

func TestRoleNamesSaveLoad(t *testing.T) {
	// Initialize a temporary in-memory db for testing
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	defer db.Close()

	// Execute DDL
	_, err = db.Exec(ddl)
	if err != nil {
		t.Fatalf("failed to execute DDL: %v", err)
	}

	// Backup original queries and dbConn
	origQueries := queries
	origDbConn := dbConn
	defer func() {
		queries = origQueries
		dbConn = origDbConn
	}()

	// Set them to our test db
	dbConn = db
	queries = New(db)

	testData := map[string][]string{
		"contract-1": {"Role A", "Role B"},
		"contract-2": {"Role C"},
	}

	// Test Save
	SaveRoleNames(testData)

	// Test Load
	loaded, err := LoadRoleNames()
	if err != nil {
		t.Fatalf("LoadRoleNames failed: %v", err)
	}

	if !reflect.DeepEqual(loaded, testData) {
		t.Errorf("loaded roles do not match saved roles: got %v, want %v", loaded, testData)
	}

	// Test non-pruning: saving for a new contract shouldn't delete old contracts' roles
	newData := map[string][]string{
		"contract-3": {"Role D"},
	}
	SaveRoleNames(newData)

	loadedAfterAdd, err := LoadRoleNames()
	if err != nil {
		t.Fatalf("LoadRoleNames failed: %v", err)
	}

	expectedMerged := map[string][]string{
		"contract-1": {"Role A", "Role B"},
		"contract-2": {"Role C"},
		"contract-3": {"Role D"},
	}
	if !reflect.DeepEqual(loadedAfterAdd, expectedMerged) {
		t.Errorf("expected merged roles: got %v, want %v", loadedAfterAdd, expectedMerged)
	}
}

func TestThematicComplaintsSaveLoad(t *testing.T) {
	// Initialize a temporary in-memory database for testing
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	defer db.Close()

	// Execute DDL
	_, err = db.Exec(ddl)
	if err != nil {
		t.Fatalf("failed to execute DDL: %v", err)
	}

	// Backup original queries, dbConn, and the complaints cache
	origQueries := queries
	origDbConn := dbConn
	origCache := thematicComplaintsMap
	defer func() {
		queries = origQueries
		dbConn = origDbConn
		thematicComplaintsMap = origCache
	}()

	// Clear cache and set db
	thematicComplaintsMap = nil
	dbConn = db
	queries = New(db)

	testData := map[string][]string{
		"contract-1": {"complaint A", "complaint B"},
		"contract-2": {"complaint C"},
	}

	// Test Save
	err = SaveThematicComplaints(testData)
	if err != nil {
		t.Fatalf("SaveThematicComplaints failed: %v", err)
	}

	// Test Load (should hit cache)
	loaded, err := LoadThematicComplaints()
	if err != nil {
		t.Fatalf("LoadThematicComplaints failed: %v", err)
	}
	if !reflect.DeepEqual(loaded, testData) {
		t.Errorf("loaded complaints (cache) do not match saved: got %v, want %v", loaded, testData)
	}

	// Clear cache to force DB load
	thematicComplaintsMu.Lock()
	thematicComplaintsMap = nil
	thematicComplaintsMu.Unlock()

	// Test Load (should hit DB)
	loadedDB, err := LoadThematicComplaints()
	if err != nil {
		t.Fatalf("LoadThematicComplaints (DB) failed: %v", err)
	}
	if !reflect.DeepEqual(loadedDB, testData) {
		t.Errorf("loaded complaints (DB) do not match saved: got %v, want %v", loadedDB, testData)
	}

	// Test non-pruning: saving for a new contract shouldn't delete old contracts' complaints
	newData := map[string][]string{
		"contract-3": {"complaint D"},
	}
	err = SaveThematicComplaints(newData)
	if err != nil {
		t.Fatalf("SaveThematicComplaints failed: %v", err)
	}

	// Load complaints (forcing DB read)
	thematicComplaintsMu.Lock()
	thematicComplaintsMap = nil
	thematicComplaintsMu.Unlock()

	loadedDBAfterAdd, err := LoadThematicComplaints()
	if err != nil {
		t.Fatalf("LoadThematicComplaints failed: %v", err)
	}

	expectedMerged := map[string][]string{
		"contract-1": {"complaint A", "complaint B"},
		"contract-2": {"complaint C"},
		"contract-3": {"complaint D"},
	}
	if !reflect.DeepEqual(loadedDBAfterAdd, expectedMerged) {
		t.Errorf("expected merged complaints: got %v, want %v", loadedDBAfterAdd, expectedMerged)
	}
}

func TestPerformTransitionFromJSON(t *testing.T) {
	// Initialize a temporary in-memory database for testing
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	defer db.Close()

	// Execute DDL
	_, err = db.Exec(ddl)
	if err != nil {
		t.Fatalf("failed to execute DDL: %v", err)
	}

	// Backup original queries and dbConn
	origQueries := queries
	origDbConn := dbConn
	defer func() {
		queries = origQueries
		dbConn = origDbConn
	}()

	queries = New(db)
	dbConn = db

	// Create dummy legacy JSON files
	rolesPath := "ttbb-data/ei-roles.json"
	complaintsPath := "ttbb-data/ei-complaints.json"

	// Make sure the directory exists
	_ = os.MkdirAll("ttbb-data", 0755)

	rolesData := map[string][]string{
		"legacy-c1": {"Role X", "Role Y"},
	}
	rolesBytes, _ := json.Marshal(rolesData)
	_ = os.WriteFile(rolesPath, rolesBytes, 0644)
	defer os.Remove(rolesPath)

	complaintsData := map[string][]string{
		"legacy-c1": {"Complaint X", "Complaint Y"},
	}
	complaintsBytes, _ := json.Marshal(complaintsData)
	_ = os.WriteFile(complaintsPath, complaintsBytes, 0644)
	defer os.Remove(complaintsPath)

	// Run transition
	performTransitionFromJSON(db)

	// Verify legacy files are deleted
	if _, err := os.Stat(rolesPath); !os.IsNotExist(err) {
		t.Errorf("expected legacy roles file to be deleted, but stat returned: %v", err)
	}
	if _, err := os.Stat(complaintsPath); !os.IsNotExist(err) {
		t.Errorf("expected legacy complaints file to be deleted, but stat returned: %v", err)
	}

	// Verify data has been imported into database
	loadedRoles, err := LoadRoleNames()
	if err != nil {
		t.Fatalf("LoadRoleNames failed: %v", err)
	}
	if !reflect.DeepEqual(loadedRoles, rolesData) {
		t.Errorf("imported roles do not match legacy data: got %v, want %v", loadedRoles, rolesData)
	}

	// Clear cache to force DB query
	thematicComplaintsMu.Lock()
	thematicComplaintsMap = nil
	thematicComplaintsMu.Unlock()

	loadedComplaints, err := LoadThematicComplaints()
	if err != nil {
		t.Fatalf("LoadThematicComplaints failed: %v", err)
	}
	if !reflect.DeepEqual(loadedComplaints, complaintsData) {
		t.Errorf("imported complaints do not match legacy data: got %v, want %v", loadedComplaints, complaintsData)
	}
}
