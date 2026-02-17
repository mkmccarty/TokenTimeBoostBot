package ei

import (
	"math"
	"testing"
)

func TestCountTruthEggTiersPassed(t *testing.T) {
	testCases := []struct {
		name      string
		delivered float64
		expected  uint32
	}{
		{"Zero eggs", 0, 0},
		{"Below first tier", 4e7, 0},
		{"Exactly first tier", 5e7, 1},
		{"Above first tier", 6e7, 1},
		{"Exactly second tier", 1e9, 2},
		{"Between tiers", 1.5e9, 2},
		{"Exactly last predefined tier", 5e16, 16},
		{"Above last predefined tier", 6e16, 16},
		{"First calculated tier", 1e17, 17},
		{"Between calculated tiers", 1.1e17, 17},
		{"A high calculated tier", 4.51e20, 98},
		{"Above all tiers", 1e22, 98},
		{"Very high eggs", 553902585144507.3, 11},
		{"Very high eggs 2", 2791545234935618, 12},
		{"Very high eggs 3", 753946034139.7086, 5},
		{"Very high eggs 4", 600725288881.8899, 5},
		{"Very high eggs 5", 659412273245466.6, 11},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := CountTruthEggTiersPassed(tc.delivered)
			if got != tc.expected {
				t.Errorf("CountTruthEggTiersPassed(%v) = %d; want %d", tc.delivered, got, tc.expected)
			}
		})
	}
}

func TestPendingTruthEggs(t *testing.T) {
	testCases := []struct {
		name      string
		delivered float64
		earnedTE  uint32
		expected  uint32
	}{
		{"Exactly on a tier, all earned", 5e7, 1, 0},
		{"Progress, more earned than passed", 1e9, 3, 0},
		{"No progress, no earned", 0, 0, 0},
		{"Progress, no earned", 1e9, 0, 2},
		{"Progress, some earned", 1e10, 1, 2},
		{"Progress, all earned", 1e10, 3, 0},
		{"High progress, some earned", 4.51e20, 90, 8},
		{"Exactly on a tier, some earned", 5e7, 0, 1},
		{"Just below a tier, none earned", 49999999, 0, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := PendingTruthEggs(tc.delivered, tc.earnedTE)
			if got != tc.expected {
				t.Errorf("PendingTruthEggs(%v, %d) = %d; want %d", tc.delivered, tc.earnedTE, got, tc.expected)
			}
		})
	}
}

func TestNextTruthEggThreshold(t *testing.T) {
	testCases := []struct {
		name        string
		delivered   float64
		eov         uint32
		expected    float64
		expectInf   bool
		description string
	}{
		{
			name:        "Zero eggs",
			delivered:   0,
			eov:         0,
			expected:    5e7,
			expectInf:   false,
			description: "Should return the first threshold for zero eggs delivered.",
		},
		{
			name:        "Below first tier",
			delivered:   4e7,
			eov:         0,
			expected:    5e7,
			expectInf:   false,
			description: "Should return the first threshold when below it.",
		},
		{
			name:        "Exactly first tier",
			delivered:   5e7,
			eov:         0,
			expected:    1e9,
			expectInf:   false,
			description: "Should return the next threshold when exactly on a tier.",
		},
		{
			name:        "Between tiers",
			delivered:   1.5e9,
			eov:         0,
			expected:    1e10,
			expectInf:   false,
			description: "Should return the next unpassed threshold.",
		},
		{
			name:        "Exactly last predefined tier",
			delivered:   5e16,
			eov:         0,
			expected:    1e17,
			expectInf:   false,
			description: "Should return the first calculated threshold.",
		},
		{
			name:        "Exactly a calculated tier",
			delivered:   1e17,
			eov:         0,
			expected:    1.5e17,
			expectInf:   false,
			description: "Should return the next calculated threshold.",
		},
		{
			name:        "Above all tiers",
			delivered:   1e22,
			eov:         0,
			expected:    0,
			expectInf:   true,
			description: "Should return infinity when all tiers are passed.",
		},
		{
			name:        "Tiers passed <= eov",
			delivered:   1e9,
			eov:         3,
			expected:    7e10,
			expectInf:   false,
			description: "Should return threshold for eov + 1 when tiers passed is <= eov.",
		},
		{
			name:        "Tiers passed == eov",
			delivered:   1e9,
			eov:         2,
			expected:    1e10,
			expectInf:   false,
			description: "Should return threshold for eov + 1 when tiers passed is == eov.",
		},
		{
			name:        "Tiers passed < eov",
			delivered:   753946034139,
			eov:         6,
			expected:    7000000000000,
			description: "Should return next unpassed threshold when tiers passed is < eov.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := NextTruthEggThreshold(tc.delivered, tc.eov)
			if tc.expectInf {
				if !math.IsInf(got, 1) {
					t.Errorf("NextTruthEggThreshold(%v, %d) = %v; want +Inf. Description: %s", tc.delivered, tc.eov, got, tc.description)
				}
			} else {
				if got != tc.expected {
					t.Errorf("NextTruthEggThreshold(%v, %d) = %v; want %v. Description: %s", tc.delivered, tc.eov, got, tc.expected, tc.description)
				}
			}
		})
	}
}
