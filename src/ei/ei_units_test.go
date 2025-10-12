package ei_test

import (
	"math"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

func TestFormatEIValue(t *testing.T) {
	// Initialize units if needed (assuming ei.init() handles this)
	// ei.init() // Or however initialization is done if not automatic

	testCases := []struct {
		name     string
		value    float64
		options  map[string]interface{}
		expected string
	}{
		{"Zero", 0, map[string]interface{}{"decimals": 3}, "0"},
		{"Small Number", 123, map[string]interface{}{"decimals": 3}, "123"},
		{"Negative Small Number", -456, map[string]interface{}{"decimals": 3}, "-456"},
		{"Kilo", 1500, map[string]interface{}{"decimals": 3}, "1.500K"},
		{"Mega", 2.5e6, map[string]interface{}{"decimals": 3}, "2.500M"},
		{"Billion", 9.87e9, map[string]interface{}{"decimals": 3}, "9.870B"},
		{"Trillion", 1.234e12, map[string]interface{}{"decimals": 3}, "1.234T"},
		{"Quadrillion (q)", 5.67e15, map[string]interface{}{"decimals": 3}, "5.670q"},
		{"Quadrillion (q)", 9.99999999e15, map[string]interface{}{"decimals": 3}, "9.999q"},
		{"Quintillion (Q)", 1e18, map[string]interface{}{"decimals": 3}, "1.000Q"},
		{"Sextillion (s)", 3.14e21, map[string]interface{}{"decimals": 3}, "3.140s"},
		{"Septillion (S)", 1e24, map[string]interface{}{"decimals": 3}, "1.000S"},
		{"Octillion (o)", 2.718e27, map[string]interface{}{"decimals": 3}, "2.718o"},
		{"Nonillion (N)", 1e30, map[string]interface{}{"decimals": 3}, "1.000N"},
		{"Decillion (d)", 4.5e33, map[string]interface{}{"decimals": 3}, "4.500d"},
		{"Vigintillion (V)", 1e63, map[string]interface{}{"decimals": 3}, "1.000V"},
		{"Tre-trigintillion (tT)", 1e93, map[string]interface{}{"decimals": 3}, "1.000tT"},
		{"Max OOM", 1e95, map[string]interface{}{"decimals": 3}, "100.000tT"},
		{"Trimmed", 12345, map[string]interface{}{"decimals": 3, "trim": true}, "12.345K"},
		{"Trimmed Zero Decimal", 5e6, map[string]interface{}{"decimals": 3, "trim": true}, "5M"},
		{"Trimmed Some Decimal", 5.6e6, map[string]interface{}{"decimals": 3, "trim": true}, "5.6M"},
		{"Scientific", 1.23e10, map[string]interface{}{"decimals": 3, "scientific": true}, "12.300×10^9"}, // Note: oomFloor logic
		{"NaN", math.NaN(), map[string]interface{}{"decimals": 3}, "NaN"},
		{"Infinity", math.Inf(1), map[string]interface{}{"decimals": 3}, "infinity"},
		{"Negative Infinity", math.Inf(-1), map[string]interface{}{"decimals": 3}, "-infinity"},
		{"Precision Option", 1.23456e12, map[string]interface{}{"precision": 5}, "1.2346T"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := ei.FormatEIValue(tc.value, tc.options)
			if got != tc.expected {
				t.Errorf("FormatEIValue(%v, %v) = %q; want %q", tc.value, tc.options, got, tc.expected)
			}
		})
	}
}

func TestFmtApprox(t *testing.T) {
	testCases := []struct {
		name     string
		value    float64
		expected string
	}{
		{"Zero", 0, "0"},
		{"Small", 123, "123"},
		{"Kilo", 1567, "1.567K"},
		{"Mega", 2.5e6, "2.500M"},
		{"Billion", 9.87654e9, "9.877B"}, // Check rounding
		{"Trillion", 1.23e12, "1.230T"},
		{"Large", 1e93, "1.000tT"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := ei.FmtApprox(tc.value)
			if got != tc.expected {
				t.Errorf("FmtApprox(%v) = %q; want %q", tc.value, got, tc.expected)
			}
		})
	}
}

func TestParseValueWithUnit(t *testing.T) {
	testCases := []struct {
		name         string
		input        string
		unitRequired bool
		expectedVal  float64
		expectError  bool
	}{
		{"Integer No Unit Optional", "123", false, 123, false},
		{"Decimal No Unit Optional", "123.45", false, 123.45, false},
		{"Integer With K Optional", "10K", false, 10000, false},
		{"Decimal With M Optional", "1.5M", false, 1500000, false},
		{"Decimal With q Optional", "2.345q", false, 2.345e15, false},
		{"Decimal With tT Optional", "1.1tT", false, 1.1e93, false},
		{"Integer No Unit Required", "123", true, 0, true},    // Error expected
		{"Decimal No Unit Required", "123.45", true, 0, true}, // Error expected
		{"Integer With K Required", "10K", true, 10000, false},
		{"Decimal With M Required", "1.5M", true, 1500000, false},
		{"Space Before Unit", "10 K", false, 10000, false},
		{"No Space Before Unit", "10K", false, 10000, false},
		{"Invalid Unit", "10X", false, 0, true},
		{"Invalid Format", "abc", false, 0, true},
		{"Invalid Format With Unit", "abcK", false, 0, true},
		{"Leading Space", " 10K", false, 0, true},  // Regex matches start ^
		{"Trailing Space", "10K ", false, 0, true}, // Regex matches end $
		{"Unit Only", "K", false, 0, true},
		{"Negative Value", "-10K", false, 0, true}, // Current regex doesn't support negative
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotVal, err := ei.ParseValueWithUnit(tc.input, tc.unitRequired)

			if tc.expectError {
				if err == nil {
					t.Errorf("ParseValueWithUnit(%q, %v) expected an error, but got nil", tc.input, tc.unitRequired)
				}
			} else {
				if err != nil {
					t.Errorf("ParseValueWithUnit(%q, %v) returned an unexpected error: %v", tc.input, tc.unitRequired, err)
				}
				// Use tolerance for float comparison
				if math.Abs(gotVal-tc.expectedVal) > 1e-9 {
					t.Errorf("ParseValueWithUnit(%q, %v) = %f; want %f", tc.input, tc.unitRequired, gotVal, tc.expectedVal)
				}
			}
		})
	}
}
