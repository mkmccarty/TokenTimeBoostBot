package ei_test

import (
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

func TestFuelTankLevels(t *testing.T) {
	if len(ei.FuelTankLevels) != 8 {
		t.Fatalf("Expected 8 fuel tank levels, got %d", len(ei.FuelTankLevels))
	}

	expectedCapacities := []float64{2e9, 200e9, 10e12, 100e12, 200e12, 300e12, 400e12, 500e12}
	for i, exp := range expectedCapacities {
		info := ei.GetFuelTankLevelInfo(i)
		if info.Level != i {
			t.Errorf("Level %d: expected level %d, got %d", i, i, info.Level)
		}
		if info.Capacity != exp {
			t.Errorf("Level %d: expected capacity %e, got %e", i, exp, info.Capacity)
		}
		capVal := ei.GetFuelTankCapacity(i)
		if capVal != exp {
			t.Errorf("GetFuelTankCapacity(%d): expected %e, got %e", i, exp, capVal)
		}
	}

	// Clamping tests
	if got := ei.GetFuelTankCapacity(-1); got != 2e9 {
		t.Errorf("Expected clamped level -1 to return 2e9, got %e", got)
	}
	if got := ei.GetFuelTankCapacity(100); got != 500e12 {
		t.Errorf("Expected clamped level 100 to return 500e12, got %e", got)
	}

	// Output format checks
	for i, c := range expectedCapacities {
		formatted := ei.FormatEIValue(c, map[string]any{"decimals": 0, "trim": true})
		switch i {
		case 0:
			if formatted != "2B" {
				t.Errorf("Level 0 formatted: expected 2B, got %s", formatted)
			}
		case 1:
			if formatted != "200B" {
				t.Errorf("Level 1 formatted: expected 200B, got %s", formatted)
			}
		case 2:
			if formatted != "10T" {
				t.Errorf("Level 2 formatted: expected 10T, got %s", formatted)
			}
		case 3:
			if formatted != "100T" {
				t.Errorf("Level 3 formatted: expected 100T, got %s", formatted)
			}
		case 4:
			if formatted != "200T" {
				t.Errorf("Level 4 formatted: expected 200T, got %s", formatted)
			}
		case 5:
			if formatted != "300T" {
				t.Errorf("Level 5 formatted: expected 300T, got %s", formatted)
			}
		case 6:
			if formatted != "400T" {
				t.Errorf("Level 6 formatted: expected 400T, got %s", formatted)
			}
		case 7:
			if formatted != "500T" {
				t.Errorf("Level 7 formatted: expected 500T, got %s", formatted)
			}
		}
	}
}
