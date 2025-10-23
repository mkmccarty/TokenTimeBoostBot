package ei

import (
	"encoding/json"
	"os"
	"testing"
)

func setupCustomEggMap() {
	// Manually populate CustomEggMap for testing purposes
	// In a real scenario, this would be populated from an API or config file.
	CustomEggMap = make(map[string]*EggIncCustomEgg)
	CustomEggMap["Chocolate"] = &EggIncCustomEgg{
		ID:             "Chocolate",
		Dimension:      GameModifier_AWAY_EARNINGS,
		DimensionValue: []float64{1.1, 1.25, 1.5, 3},
	}
	CustomEggMap["Easter"] = &EggIncCustomEgg{
		ID:             "Easter",
		Dimension:      GameModifier_INTERNAL_HATCHERY_RATE,
		DimensionValue: []float64{1.01, 1.02, 1.03, 1.05},
	}
	CustomEggMap["Waterballoon"] = &EggIncCustomEgg{
		ID:             "Waterballoon",
		Dimension:      GameModifier_RESEARCH_COST, // Not used in GetColleggtibleBuffs
		DimensionValue: []float64{0.99, 0.98, 0.97, 0.95},
	}
	CustomEggMap["Firework"] = &EggIncCustomEgg{
		ID:             "Firework",
		Dimension:      GameModifier_EARNINGS,
		DimensionValue: []float64{1.01, 1.02, 1.03, 1.05},
	}
	CustomEggMap["Pumpkin"] = &EggIncCustomEgg{
		ID:             "Pumpkin",
		Dimension:      GameModifier_SHIPPING_CAPACITY,
		DimensionValue: []float64{1.01, 1.02, 1.03, 1.05},
	}
	CustomEggMap["CarbonFiber"] = &EggIncCustomEgg{
		ID:             "CarbonFiber",
		Dimension:      GameModifier_HAB_CAPACITY,
		DimensionValue: []float64{1.01, 1.02, 1.03, 1.05},
	}
}

func TestGetColleggtibleBuffs(t *testing.T) {
	setupCustomEggMap()

	// Print current working directory for debugging
	cwd, err := os.Getwd()
	if err != nil {
		t.Logf("Warning: Could not get current directory: %v", err)
	} else {
		t.Logf("Current working directory: %s", cwd)
	}

	// Load test data from JSON file
	data, err := os.ReadFile("testdata1.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}

	var contracts MyContracts
	err = json.Unmarshal(data, &contracts)
	if err != nil {
		t.Fatalf("Failed to unmarshal test data: %v", err)
	}

	testCases := []struct {
		name             string
		contracts        *MyContracts
		expectedELR      float64
		expectedSR       float64
		expectedIHR      float64
		expectedHab      float64
		expectedEarnings float64
		expectedAway     float64
	}{
		{
			name:             "All buffs from JSON",
			contracts:        &contracts,
			expectedELR:      1.0,  // No ELR colleggtible in test data
			expectedSR:       1.05, // Pumpkin with 1e10 farm size (tier 3)
			expectedIHR:      1.05, // Easter with 1e10 farm size (tier 3)
			expectedHab:      1.05, // CarbonFiber with 1e10 farm size (tier 3)
			expectedEarnings: 1.05, // Firework with 1e10 farm size (tier 3)
			expectedAway:     3.0,  // Chocolate with 1e10 farm size (tier 3)
		},
		{
			name:             "No contracts in archive",
			contracts:        &MyContracts{Archive: []*LocalContract{}},
			expectedELR:      1.0,
			expectedSR:       1.0,
			expectedIHR:      1.0,
			expectedHab:      1.0,
			expectedEarnings: 1.0,
			expectedAway:     1.0,
		},
		{
			name:             "Nil contracts",
			contracts:        nil,
			expectedELR:      1.0,
			expectedSR:       1.0,
			expectedIHR:      1.0,
			expectedHab:      1.0,
			expectedEarnings: 1.0,
			expectedAway:     1.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buffs := GetColleggtibleBuffs(tc.contracts)

			if buffs.ELR != tc.expectedELR {
				t.Errorf("ELR: got %v, want %v", buffs.ELR, tc.expectedELR)
			}
			if buffs.SR != tc.expectedSR {
				t.Errorf("SR: got %v, want %v", buffs.SR, tc.expectedSR)
			}
			if buffs.IHR != tc.expectedIHR {
				t.Errorf("IHR: got %v, want %v", buffs.IHR, tc.expectedIHR)
			}
			if buffs.Hab != tc.expectedHab {
				t.Errorf("Hab: got %v, want %v", buffs.Hab, tc.expectedHab)
			}
			if buffs.Earnings != tc.expectedEarnings {
				t.Errorf("Earnings: got %v, want %v", buffs.Earnings, tc.expectedEarnings)
			}
			if buffs.AwayEarnings != tc.expectedAway {
				t.Errorf("AwayEarnings: got %v, want %v", buffs.AwayEarnings, tc.expectedAway)
			}
		})
	}
}
