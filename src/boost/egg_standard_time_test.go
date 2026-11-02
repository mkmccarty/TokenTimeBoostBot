package boost

import (
	"strings"
	"testing"
	"time"
)

func TestFormatEggTimeOffset(t *testing.T) {
	tests := []struct {
		name     string
		diff     time.Duration
		expected string
	}{
		{
			name:     "Zero offset",
			diff:     0,
			expected: "+0 (+0m)",
		},
		{
			name:     "Positive whole hours",
			diff:     2 * time.Hour,
			expected: "+2 (+2h)",
		},
		{
			name:     "Positive fractional hours",
			diff:     2*time.Hour + 30*time.Minute,
			expected: "+2.5 (+2h 30m)",
		},
		{
			name:     "Negative whole hours",
			diff:     -1 * time.Hour,
			expected: "-1 (-1h)",
		},
		{
			name:     "Negative fractional hours",
			diff:     -(1*time.Hour + 15*time.Minute),
			expected: "-1.25 (-1h 15m)",
		},
		{
			name:     "Negative minutes only",
			diff:     -45 * time.Minute,
			expected: "-0.75 (-45m)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatEggTimeOffset(tc.diff)
			if got != tc.expected {
				t.Errorf("FormatEggTimeOffset(%v) = %q, want %q", tc.diff, got, tc.expected)
			}
		})
	}
}

func TestBuildEggStandardTimeString(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("Failed to load America/Los_Angeles location: %v", err)
	}

	// 11:30 AM PT on Nov 2, 2026 -> +2.5 hours from 9:00 AM PST (Standard Time)
	now := time.Date(2026, 11, 2, 11, 30, 0, 0, loc)
	msg := BuildEggStandardTimeString(now)

	if !strings.Contains(msg, "**Egg Standard Time:**") {
		t.Errorf("expected msg to contain '**Egg Standard Time:**', got: %s", msg)
	}
	if !strings.Contains(msg, "**Kev's Time:** 11:30 AM PST") {
		t.Errorf("expected msg to contain '**Kev's Time:** 11:30 AM PST', got: %s", msg)
	}
	if !strings.Contains(msg, "**Current Time:** <t:") || !strings.Contains(msg, ":R>") {
		t.Errorf("expected msg to contain Discord relative time for Current Time, got: %s", msg)
	}
	if !strings.Contains(msg, "**Current Offset Time:**") {
		t.Errorf("expected msg to contain '**Current Offset Time:**', got: %s", msg)
	}
	if !strings.Contains(msg, "+2.5 (+2h 30m)") {
		t.Errorf("expected msg to contain '+2.5 (+2h 30m)', got: %s", msg)
	}

	// Also verify PDT (Daylight Saving Time)
	nowPDT := time.Date(2026, 6, 15, 8, 0, 0, 0, loc)
	msgPDT := BuildEggStandardTimeString(nowPDT)
	if !strings.Contains(msgPDT, "**Kev's Time:** 8:00 AM PDT") {
		t.Errorf("expected msgPDT to contain '**Kev's Time:** 8:00 AM PDT', got: %s", msgPDT)
	}
	if !strings.Contains(msgPDT, "**Egg Standard Time:** 9:00 AM PDT") {
		t.Errorf("expected msgPDT to contain '**Egg Standard Time:** 9:00 AM PDT', got: %s", msgPDT)
	}
	if !strings.Contains(msgPDT, "**Current Offset Time:** `-1 (-1h)`") {
		t.Errorf("expected msgPDT to contain '**Current Offset Time:** `-1 (-1h)`', got: %s", msgPDT)
	}
}
