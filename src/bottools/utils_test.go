package bottools

import (
	"testing"
	"time"
)

func TestFmtDuration(t *testing.T) {
	now := time.Now()

	later := now.Add(1 * time.Hour)
	if FmtDuration(later.Sub(now)) != "1h" {
		t.Errorf("FmtDuration() = %v, want %v", FmtDuration(later.Sub(now)), "1h")
	}

	later = now.Add(1 * time.Hour).Add(15 * time.Minute)
	if FmtDuration(later.Sub(now)) != "1h15m" {
		t.Errorf("FmtDuration() = %v, want %v", FmtDuration(later.Sub(now)), "1h15m")
	}

	later = now.Add(72520 * time.Minute)
	if FmtDuration(later.Sub(now)) != "50d8h40m" {
		t.Errorf("FmtDuration() = %v, want %v", FmtDuration(later.Sub(now)), "50d8h40m")
	}
}

func TestSanitizeStringDuration(t *testing.T) {

	if SanitizeStringDuration("1h30m15s") != "1h30m15s" {
		t.Errorf("SanitizeStringDuration() = %v, want %v", SanitizeStringDuration("1h30m15s"), "1h30m15s")
	}

	if SanitizeStringDuration("1h 30m 15s") != "1h30m15s" {
		t.Errorf("SanitizeStringDuration() = %v, want %v", SanitizeStringDuration("1h 30m 15s"), "1h30m15s")
	}

	if SanitizeStringDuration("1h 15s 30m") != "1h30m15s" {
		t.Errorf("SanitizeStringDuration() = %v, want %v", SanitizeStringDuration("1h 15s 30m"), "1h30m15s")
	}

	if SanitizeStringDuration("1h30m15s, 2h34s") != "1h30m15s" {
		t.Errorf("SanitizeStringDuration() = %v, want %v", SanitizeStringDuration("1h30m15s, 2h34s"), "1h30m15s")
	}

	if SanitizeStringDuration("5d1h30m15s, 2h34s") != "5d1h30m15s" {
		t.Errorf("SanitizeStringDuration() = %v, want %v", SanitizeStringDuration("5d1h30m15s, 2h34s"), "5d1h30m15s")
	}
}

func FuzzSanitizeStringDuration(f *testing.F) {
	f.Add("1h30m15s")
	f.Add("1h 30m 15s")
	f.Add("1h 15s 30m")
	f.Add("1h30m15s, 2h34s")
	f.Add("5d1h30m15s, 2h34s")
	f.Fuzz(func(t *testing.T, s string) {
		result := SanitizeStringDuration(s)
		if len(result) > 0 {
			t.Logf("SanitizeStringDuration(%s) = %s", s, result)
		}
	})

}

func TestCleanContractTeamNames(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "JSON array format",
			input:    `["Galaxy Strikers", "Solar Flares", "Nebula Nomads"]`,
			expected: []string{"Galaxy Strikers", "Solar Flares", "Nebula Nomads"},
		},
		{
			name: "Conversational preamble with newline and comma-separated list",
			input: `Here are 30 team names:

Galaxy Strikers, Solar Flares, Nebula Nomads`,
			expected: []string{"Galaxy Strikers", "Solar Flares", "Nebula Nomads"},
		},
		{
			name: "Numbered list format",
			input: `1. Galaxy Strikers
2. Solar Flares
3. Nebula Nomads`,
			expected: []string{"Galaxy Strikers", "Solar Flares", "Nebula Nomads"},
		},
		{
			name: "Bulleted list format with conversational outro",
			input: `Sure! I can help with that.
- Galaxy Strikers
- Solar Flares
- Nebula Nomads

Let me know if you need more!`,
			expected: []string{"Galaxy Strikers", "Solar Flares", "Nebula Nomads"},
		},
		{
			name:     "Single line comma separated list",
			input:    `Galaxy Strikers, Solar Flares, Nebula Nomads`,
			expected: []string{"Galaxy Strikers", "Solar Flares", "Nebula Nomads"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CleanContractTeamNames(tc.input)
			if len(got) != len(tc.expected) {
				t.Fatalf("expected %d elements, got %d. Got: %v", len(tc.expected), len(got), got)
			}
			for i, v := range got {
				if v != tc.expected[i] {
					t.Errorf("at index %d: expected %q, got %q", i, tc.expected[i], v)
				}
			}
		})
	}
}
