package boost

import (
	"testing"
)

// Write a test for ReadConfig
// Make sure to use the testing package

func TestDynamicTokens(t *testing.T) {

	dt := createDynamicTokenData()

	// Initially assign 6 token boosts to everyone,
	// In reverse order start calculating using 8 token boosts
	// stop when the 120 minute delivered eggs amount is less than the previous amount
	result := calculateDynamicTokens(dt, 4, 0.39, 0)

	// result should be 4 integers with the the last one being an 8
	if len(result) != 4 {
		t.Errorf("calculateDynamicTokens() = %v, want %v", len(result), 4)
	}

	if result[0] != 6 {
		t.Errorf("calculateDynamicTokens() = %v, want %v", result[0], 6)
	}

	if result[1] != 6 {
		t.Errorf("calculateDynamicTokens() = %v, want %v", result[1], 6)
	}

	if result[2] != 6 {

		t.Errorf("calculateDynamicTokens() = %v, want %v", result[2], 6)
	}

	if result[3] != 8 {

		t.Errorf("calculateDynamicTokens() = %v, want %v", result[3], 8)
	}

}
