package boost

import (
	"fmt"
	"math"
	"testing"
)

// Helper function for floating-point comparisons
func almostEqual(a, b float64) bool {
	return math.Abs(a-b) <= 0.05
}

func TestCalculateBoostTime(t *testing.T) {
	tokenRate := 13.0
	sixTokBoostTime := 20.0

	tests := []struct {
		name     string
		te       int
		tokens   int
		expected float64
	}{
		// User's initial test cases
		{"TE 0, 6tok", 0, 6, 97.61},
		{"TE 490, 0tok", 490, 0, 12.21},

		// Additional specific column checks
		{"TE 1, 8tok", 1, 8, 111.92},
		{"TE 10, 3tok", 10, 3, 317.55},
		{"TE 199, 2tok", 199, 2, 104.89},

		// Crossover point: TE 108 (5tok and 4tok are tied)
		{"TE 108, 5tok", 108, 5, 78.26},
		{"TE 108, 4tok", 108, 4, 78.26},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CalculateBoostTime(tc.te, tc.tokens, tokenRate, sixTokBoostTime)
			if !almostEqual(got, tc.expected) {
				t.Errorf("CalculateBoostTime(%d, %d, %.1f, %.1f) = %.2f; want %.2f",
					tc.te, tc.tokens, tokenRate, sixTokBoostTime, got, tc.expected)
			}
		})
	}
}

func TestFindFastestBoost(t *testing.T) {
	tokenRate := 13.0
	sixTokBoostTime := 20.0

	tests := []struct {
		name             string
		te               int
		expectedStrategy string
		expectedTime     float64
	}{
		// User's initial test case
		{"TE 40 (Optimal Strategy Shift to 5tok)", 40, "5tok", 91.08},

		// Range 1: 6tok (TE 0 to 39)
		{"TE 0 (Lower bound 6tok)", 0, "6tok", 97.61},
		{"TE 39 (Upper bound 6tok)", 39, "6tok", 91.30},

		// Range 2: 5tok (TE 40 to 108)
		{"TE 108 (Upper bound 5tok)", 108, "5tok", 78.26},

		// Range 3: 4tok (TE 109 to 289)
		{"TE 109 (Lower bound 4tok)", 109, "4tok", 78.00},
		{"TE 199 (Mid 4tok)", 199, "4tok", 62.62},

		// Range End: 0tok (TE 386 to 490)
		{"TE 490 (Upper bound 0tok)", 490, "0tok", 12.21},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FindFastestBoost(tc.te, tokenRate, sixTokBoostTime)

			if got.Strategy != tc.expectedStrategy {
				t.Errorf("FindFastestBoost(%d, ...).Strategy = %s; want %s",
					tc.te, got.Strategy, tc.expectedStrategy)
			}

			var expectedTokInt int
			_, _ = fmt.Sscanf(tc.expectedStrategy, "%dtok", &expectedTokInt)
			if got.Tokens != expectedTokInt {
				t.Errorf("FindFastestBoost(%d, ...).Tokens = %d; want %d",
					tc.te, got.Tokens, expectedTokInt)
			}

			expectedMult := calcBoostMulti(float64(expectedTokInt))
			if got.Multiplier != expectedMult {
				t.Errorf("FindFastestBoost(%d, ...).Multiplier = %.1f; want %.1f",
					tc.te, got.Multiplier, expectedMult)
			}

			if !almostEqual(got.TotalTime, tc.expectedTime) {
				t.Errorf("FindFastestBoost(%d, ...).TotalTime = %.2f; want %.2f",
					tc.te, got.TotalTime, tc.expectedTime)
			}
		})
	}

	// Tests with full artifact multipliers derived from log data
	// Log 1: TE=149, Collegg=1.05, Chalice=1.40, Monocle=1.30, Stones=1.4802 => artifactMult = 1.05 * 1.40 * 1.30 * 1.4802 = 2.82866
	// Log 2: TE=101, Collegg=1.05, Chalice=1.40, Monocle=1.30, Stones=9 (1.4233) => artifactMult = 1.05 * 1.40 * 1.30 * 1.4233 = 2.71992
	artifactTests := []struct {
		name             string
		te               int
		artifactMult     float64
		expectedTokens   int
		expectedStrategy string
	}{
		{
			name:             "TE 149 with artifacts (Collegg 1.05, T4L Chalice 1.4, T4L Monocle 1.3, 10 Stones 1.4802)",
			te:               149,
			artifactMult:     1.05 * 1.40 * 1.30 * 1.4802, // 2.82866
			expectedTokens:   4,
			expectedStrategy: "4tok",
		},
		{
			name:             "TE 101 with artifacts (Collegg 1.05, T4L Chalice 1.4, T4L Monocle 1.3, 9 Stones 1.4233)",
			te:               101,
			artifactMult:     1.05 * 1.40 * 1.30 * 1.4233, // 2.71992
			expectedTokens:   4,
			expectedStrategy: "4tok",
		},
	}

	for _, tc := range artifactTests {
		t.Run(tc.name, func(t *testing.T) {
			got := FindFastestBoost(tc.te, tokenRate, sixTokBoostTime, tc.artifactMult)
			if got.Tokens != tc.expectedTokens {
				t.Errorf("FindFastestBoost(%d, ..., %.4f).Tokens = %d (%s); want %d (%s)",
					tc.te, tc.artifactMult, got.Tokens, got.Strategy, tc.expectedTokens, tc.expectedStrategy)
			}
		})
	}
}

func TestGetPlayerBoostConfig(t *testing.T) {
	tests := []struct {
		name               string
		te                 float64
		expectedTokens     float64
		expectedMultiplier float64
	}{
		{"Low TE (1.01^0 = 1 <= 2) -> 6 tokens", 0.0, 6.0, 4080.0},
		{"Mid TE (1.01^70 = 2.006 > 2) -> 5 tokens", 70.0, 5.0, 2060.0},
		{"High TE (1.01^140 = 4.028 > 4) -> 4 tokens", 140.0, 4.0, 1040.0},
		{"Very High TE (TE 490 -> 0tok)", 490.0, 0.0, 50.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, mult := GetPlayerBoostConfig(tc.te)
			if tokens != tc.expectedTokens {
				t.Errorf("GetPlayerBoostConfig(%.1f) tokens = %.1f; want %.1f", tc.te, tokens, tc.expectedTokens)
			}
			if mult != tc.expectedMultiplier {
				t.Errorf("GetPlayerBoostConfig(%.1f) multiplier = %.1f; want %.1f", tc.te, mult, tc.expectedMultiplier)
			}
		})
	}
}

func TestGetBoostMultiplierForTokens(t *testing.T) {
	tests := []struct {
		tokens   float64
		expected float64
	}{
		{1.0, 80.0},
		{2.0, 140.0},
		{3.0, 260.0},
		{4.0, 1040.0},
		{5.0, 2060.0},
		{6.0, 4080.0},
		{8.0, 10300.0},
	}

	for _, tc := range tests {
		got := GetBoostMultiplierForTokens(tc.tokens)
		if got != tc.expected {
			t.Errorf("GetBoostMultiplierForTokens(%.1f) = %.1f; want %.1f", tc.tokens, got, tc.expected)
		}
	}
}
