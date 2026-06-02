package boost_test

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"google.golang.org/protobuf/proto"
)

func TestLoadContractDataHistory(t *testing.T) {
	gradeSpecs := []*ei.Contract_GradeSpec{
		{
			Grade: ei.Contract_GRADE_AAA.Enum(),
			Goals: []*ei.Contract_Goal{
				{
					TargetAmount: proto.Float64(100000),
					RewardType:   ei.RewardType_EGGS_OF_PROPHECY.Enum(),
				},
			},
			LengthSeconds: proto.Float64(86400),
		},
	}

	// Create mock contracts
	c1Proto := &ei.Contract{
		Identifier:     proto.String("test-history-contract"),
		Name:           proto.String("Test History V1"),
		ExpirationTime: proto.Float64(float64(time.Now().Add(10 * time.Hour).Unix())),
		StartTime:      proto.Float64(float64(time.Now().Unix())),
		GradeSpecs:     gradeSpecs,
	}
	c1Bin, err := proto.Marshal(c1Proto)
	if err != nil {
		t.Fatal(err)
	}

	// V2 is later (on the next day) to ensure ValidFrom is different after GetEggStandardTime
	c2Proto := &ei.Contract{
		Identifier:     proto.String("test-history-contract"),
		Name:           proto.String("Test History V2"),
		ExpirationTime: proto.Float64(float64(time.Now().Add(48 * time.Hour).Unix())),
		StartTime:      proto.Float64(float64(time.Now().Add(24 * time.Hour).Unix())),
		GradeSpecs:     gradeSpecs,
	}
	c2Bin, err := proto.Marshal(c2Proto)
	if err != nil {
		t.Fatal(err)
	}

	loadedContracts := []ei.EggIncContract{
		{
			ID:    "test-history-contract",
			Proto: base64.StdEncoding.EncodeToString(c1Bin),
		},
		{
			ID:    "test-history-contract",
			Proto: base64.StdEncoding.EncodeToString(c2Bin),
		},
	}

	// Write to a temporary file
	tmpFile, err := os.CreateTemp("", "contracts_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	defer func() { _ = tmpFile.Close() }()

	encoder := json.NewEncoder(tmpFile)
	if err := encoder.Encode(loadedContracts); err != nil {
		t.Fatal(err)
	}
	_ = tmpFile.Close()

	// Clear global maps to avoid interference
	ei.EggIncContractsAll = make(map[string]ei.EggIncContract)

	// Load the contracts
	boost.LoadContractData(tmpFile.Name())

	// Verify the result
	contract, exists := ei.EggIncContractsAll["test-history-contract"]
	if !exists {
		t.Fatalf("expected contract to exist in ei.EggIncContractsAll")
	}

	// The current version should be V2
	if contract.Name != "Test History V2" {
		t.Errorf("expected current name to be 'Test History V2', got '%s'", contract.Name)
	}

	// The history should have V1
	if len(contract.History) != 1 {
		t.Fatalf("expected history length to be 1, got %d", len(contract.History))
	}

	v1 := contract.History[0]
	if v1.Name != "Test History V1" {
		t.Errorf("expected history[0] name to be 'Test History V1', got '%s'", v1.Name)
	}

	if len(v1.History) != 0 {
		t.Errorf("expected history[0].History to be empty, got %d elements", len(v1.History))
	}

	// Test GetEggIncContractByStartTime
	// V1 start time is roughly now.
	// V2 start time is 24 hours later.
	v1StartTime := contract.History[0].ValidFrom
	v2StartTime := contract.ValidFrom

	// 1. Query at V2 start time -> should get V2
	cRes, found := ei.GetEggIncContractByStartTime("test-history-contract", v2StartTime.Add(1*time.Hour))
	if !found || cRes.Name != "Test History V2" {
		t.Errorf("expected Test History V2 for time after V2 start, got name=%s (found=%t)", cRes.Name, found)
	}

	// 2. Query between V1 and V2 start times -> should get V1
	cRes, found = ei.GetEggIncContractByStartTime("test-history-contract", v1StartTime.Add(5*time.Hour))
	if !found || cRes.Name != "Test History V1" {
		t.Errorf("expected Test History V1 for time between V1 and V2, got name=%s (found=%t)", cRes.Name, found)
	}

	// 3. Query before V1 start time -> should fallback to V1 (the oldest)
	cRes, found = ei.GetEggIncContractByStartTime("test-history-contract", v1StartTime.Add(-5*time.Hour))
	if !found || cRes.Name != "Test History V1" {
		t.Errorf("expected Test History V1 fallback for time before V1, got name=%s (found=%t)", cRes.Name, found)
	}
}
