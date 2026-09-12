package notok

import (
	"slices"
	"strings"
	"testing"
)

func TestParseComplaintArrayFallback(t *testing.T) {
	raw := `["[player] complaint one", "[player] complaint two", "[player] complaint three"]`
	got := parseComplaintArrayFallback(raw)
	if len(got) != 3 {
		t.Fatalf("expected 3 items, got %d", len(got))
	}
	expected := []string{"[player] complaint one", "[player] complaint two", "[player] complaint three"}
	if !slices.Equal(got, expected) {
		t.Fatalf("got %v, want %v", got, expected)
	}
}

func TestBracketCorrection(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{
			input:    `player] lost their tokens to a bad crop.`,
			expected: `[player] lost their tokens to a bad crop.`,
		},
		{
			input:    `The vendor told [player that tokens are out of stock.`,
			expected: `The vendor told [player] that tokens are out of stock.`,
		},
		{
			input:    `[player] already had correct brackets.`,
			expected: `[player] already had correct brackets.`,
		},
	}

	for _, tc := range testCases {
		c := tc.input
		if strings.Contains(c, "player]") && !strings.Contains(c, "[player]") {
			c = strings.ReplaceAll(c, "player]", "[player]")
		}
		if strings.Contains(c, "[player") && !strings.Contains(c, "[player]") {
			c = strings.ReplaceAll(c, "[player", "[player]")
		}
		if c != tc.expected {
			t.Errorf("input %q => got %q, want %q", tc.input, c, tc.expected)
		}
	}
}
