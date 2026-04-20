package boost

import (
	"testing"
	"time"
)

func TestGetEggStandardTime(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("Failed to load America/Los_Angeles location: %v", err)
	}

	tests := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			name:     "Day before Spring Forward (PST)",
			input:    time.Date(2024, 3, 9, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 3, 9, 9, 0, 0, 0, loc),
		},
		{
			name:     "Day of Spring Forward (PDT)",
			input:    time.Date(2024, 3, 10, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 3, 10, 9, 0, 0, 0, loc),
		},
		{
			name:     "Day after Spring Forward (PDT)",
			input:    time.Date(2024, 3, 11, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 3, 11, 9, 0, 0, 0, loc),
		},
		{
			name:     "Day before Fall Back (PDT)",
			input:    time.Date(2024, 11, 2, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 11, 2, 9, 0, 0, 0, loc),
		},
		{
			name:     "Day of Fall Back (PST)",
			input:    time.Date(2024, 11, 3, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 11, 3, 9, 0, 0, 0, loc),
		},
		{
			name:     "Day after Fall Back (PST)",
			input:    time.Date(2024, 11, 4, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 11, 4, 9, 0, 0, 0, loc),
		},
		{
			name:     "Day before Spring Forward (9 AM PST)",
			input:    time.Date(2024, 3, 9, 9, 0, 0, 0, loc),
			expected: time.Date(2024, 3, 9, 9, 0, 0, 0, loc),
		},
		{
			name:     "Day of Spring Forward (9 AM PDT)",
			input:    time.Date(2024, 3, 10, 9, 0, 0, 0, loc),
			expected: time.Date(2024, 3, 10, 9, 0, 0, 0, loc),
		},
		{
			name:     "Day after Spring Forward (9 AM PDT)",
			input:    time.Date(2024, 3, 11, 9, 0, 0, 0, loc),
			expected: time.Date(2024, 3, 11, 9, 0, 0, 0, loc),
		},
		{
			name:     "Day before Fall Back (9 AM PDT)",
			input:    time.Date(2024, 11, 2, 9, 0, 0, 0, loc),
			expected: time.Date(2024, 11, 2, 9, 0, 0, 0, loc),
		},
		{
			name:     "Day of Fall Back (9 AM PST)",
			input:    time.Date(2024, 11, 3, 9, 0, 0, 0, loc),
			expected: time.Date(2024, 11, 3, 9, 0, 0, 0, loc),
		},
		{
			name:     "Day after Fall Back (9 AM PST)",
			input:    time.Date(2024, 11, 4, 9, 0, 0, 0, loc),
			expected: time.Date(2024, 11, 4, 9, 0, 0, 0, loc),
		},
		{
			name:     "Input from different timezone (Asia/Tokyo)",
			input:    time.Date(2024, 6, 15, 23, 0, 0, 0, time.FixedZone("JST", 9*3600)),
			expected: time.Date(2024, 6, 15, 9, 0, 0, 0, loc),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := GetEggStandardTime(tc.input)
			if !got.Equal(tc.expected) {
				t.Errorf("GetEggStandardTime(%v) = %v; want %v", tc.input, got, tc.expected)
			}
			if got.Hour() != 9 {
				t.Errorf("Expected hour to be 9, got %d", got.Hour())
			}
			if got.Location().String() != "America/Los_Angeles" {
				t.Errorf("Expected location to be America/Los_Angeles, got %s", got.Location().String())
			}
		})
	}
}
