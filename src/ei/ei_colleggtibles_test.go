package ei

import (
	"encoding/json"
	"os"
	"testing"
)

func setupCustomEggMap() {
	// Keep fixtures static for test determinism; list synced from ttbb-data/ei-customeggs.json.
	CustomEggMap = make(map[string]*EggIncCustomEgg)
	CustomEggMap["carbon-fiber"] = &EggIncCustomEgg{
		ID:             "carbon-fiber",
		Dimension:      GameModifier_SHIPPING_CAPACITY,
		DimensionValue: []float64{1.01, 1.02, 1.03, 1.05},
	}
	CustomEggMap["chocolate"] = &EggIncCustomEgg{
		ID:             "chocolate",
		Dimension:      GameModifier_AWAY_EARNINGS,
		DimensionValue: []float64{1.25, 1.5, 2.0, 3.0},
	}
	CustomEggMap["easter"] = &EggIncCustomEgg{
		ID:             "easter",
		Dimension:      GameModifier_INTERNAL_HATCHERY_RATE,
		DimensionValue: []float64{1.01, 1.02, 1.03, 1.05},
	}
	CustomEggMap["firework"] = &EggIncCustomEgg{
		ID:             "firework",
		Dimension:      GameModifier_EARNINGS,
		DimensionValue: []float64{1.01, 1.02, 1.03, 1.05},
	}
	CustomEggMap["flame-retardant"] = &EggIncCustomEgg{
		ID:             "flame-retardant",
		Dimension:      GameModifier_HAB_COST,
		DimensionValue: []float64{0.99, 0.95, 0.88, 0.75},
	}
	CustomEggMap["ice"] = &EggIncCustomEgg{
		ID:             "ice",
		Dimension:      GameModifier_RESEARCH_COST,
		DimensionValue: []float64{0.99, 0.98, 0.965, 0.95},
	}
	CustomEggMap["lithium"] = &EggIncCustomEgg{
		ID:             "lithium",
		Dimension:      GameModifier_VEHICLE_COST,
		DimensionValue: []float64{0.98, 0.96, 0.93, 0.9},
	}
	CustomEggMap["pegg"] = &EggIncCustomEgg{
		ID:             "pegg",
		Dimension:      GameModifier_HAB_CAPACITY,
		DimensionValue: []float64{1.01, 1.02, 1.03, 1.05},
	}
	CustomEggMap["pumpkin"] = &EggIncCustomEgg{
		ID:             "pumpkin",
		Dimension:      GameModifier_SHIPPING_CAPACITY,
		DimensionValue: []float64{1.01, 1.02, 1.03, 1.05},
	}
	CustomEggMap["silicon"] = &EggIncCustomEgg{
		ID:             "silicon",
		Dimension:      GameModifier_EGG_LAYING_RATE,
		DimensionValue: []float64{1.01, 1.02, 1.03, 1.05},
	}
	CustomEggMap["waterballoon"] = &EggIncCustomEgg{
		ID:             "waterballoon",
		Dimension:      GameModifier_RESEARCH_COST,
		DimensionValue: []float64{0.99, 0.98, 0.97, 0.95},
	}
	CustomEggMap["wood"] = &EggIncCustomEgg{
		ID:             "wood",
		Dimension:      GameModifier_AWAY_EARNINGS,
		DimensionValue: []float64{1.1, 1.25, 1.5, 2.0},
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
		expectedVehicle  float64
		expectedHabCost  float64
		expectedResearch float64
	}{
		{
			name:             "All buffs from JSON",
			contracts:        &contracts,
			expectedELR:      1.05, // No ELR colleggtible in test data
			expectedSR:       1.05, // Pumpkin with 1e10 farm size (tier 3)
			expectedIHR:      1.0,  // Easter with 1e10 farm size (tier 3)
			expectedHab:      1.0,  // CarbonFiber with 1e10 farm size (tier 3)
			expectedEarnings: 1.0,  // Firework with 1e10 farm size (tier 3)
			expectedAway:     6.0,  // Chocolate with 1e10 farm size (tier 3)
			expectedVehicle:  1.0,
			expectedHabCost:  1.0,
			expectedResearch: 1.0,
		},
		/*
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
		*/
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
			if buffs.VehicleCost != tc.expectedVehicle {
				t.Errorf("VehicleCost: got %v, want %v", buffs.VehicleCost, tc.expectedVehicle)
			}
			if buffs.HabCost != tc.expectedHabCost {
				t.Errorf("HabCost: got %v, want %v", buffs.HabCost, tc.expectedHabCost)
			}
			if buffs.ResearchDiscount != tc.expectedResearch {
				t.Errorf("ResearchDiscount: got %v, want %v", buffs.ResearchDiscount, tc.expectedResearch)
			}
		})
	}
}

func TestGetColleggtibleBuffsAllDimensions(t *testing.T) {
	setupCustomEggMap()

	makeContract := func(eggID string) *LocalContract {
		return &LocalContract{
			Contract: &Contract{CustomEggId: &eggID},
			MaxFarmSizeReached: func(v float64) *float64 {
				return &v
			}(1e10),
		}
	}

	contracts := &MyContracts{
		Contracts: []*LocalContract{
			makeContract("silicon"),
			makeContract("pumpkin"),
			makeContract("easter"),
			makeContract("pegg"),
			makeContract("firework"),
			makeContract("chocolate"),
			makeContract("ice"),
			makeContract("waterballoon"),
			makeContract("lithium"),
			makeContract("flame-retardant"),
		},
	}

	buffs := GetColleggtibleBuffs(contracts)

	if buffs.ELR != 1.05 {
		t.Errorf("ELR: got %v, want 1.05", buffs.ELR)
	}
	if buffs.SR != 1.05 {
		t.Errorf("SR: got %v, want 1.05", buffs.SR)
	}
	if buffs.IHR != 1.05 {
		t.Errorf("IHR: got %v, want 1.05", buffs.IHR)
	}
	if buffs.Hab != 1.05 {
		t.Errorf("Hab: got %v, want 1.05", buffs.Hab)
	}
	if buffs.Earnings != 1.05 {
		t.Errorf("Earnings: got %v, want 1.05", buffs.Earnings)
	}
	if buffs.AwayEarnings != 3.0 {
		t.Errorf("AwayEarnings: got %v, want 3", buffs.AwayEarnings)
	}
	if buffs.VehicleCost != 0.9 {
		t.Errorf("VehicleCost: got %v, want 0.9", buffs.VehicleCost)
	}
	if buffs.HabCost != 0.75 {
		t.Errorf("HabCost: got %v, want 0.75", buffs.HabCost)
	}
	if buffs.ResearchDiscount != 0.9025 {
		t.Errorf("ResearchDiscount: got %v, want 0.9025", buffs.ResearchDiscount)
	}
}
