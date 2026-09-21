package bottools

import (
	"testing"
	"time"
)

func TestGetCelestialSeasonBoundaries(t *testing.T) {
	tests := []struct {
		name string
		date time.Time
		want string
	}{
		{
			name: "early june remains spring",
			date: time.Date(2026, time.June, 1, 12, 0, 0, 0, time.UTC),
			want: "spring",
		},
		{
			name: "before spring equinox 2026",
			date: time.Date(2026, time.March, 20, 14, 44, 59, 0, time.UTC),
			want: "winter",
		},
		{
			name: "at spring equinox 2026 (14:45 UTC)",
			date: time.Date(2026, time.March, 20, 14, 45, 0, 0, time.UTC),
			want: "spring",
		},
		{
			name: "just before summer solstice 2026",
			date: time.Date(2026, time.June, 21, 8, 23, 59, 0, time.UTC),
			want: "spring",
		},
		{
			name: "at summer solstice 2026 (08:24 UTC)",
			date: time.Date(2026, time.June, 21, 8, 24, 0, 0, time.UTC),
			want: "summer",
		},
		{
			name: "just before fall equinox 2026",
			date: time.Date(2026, time.September, 23, 0, 4, 59, 0, time.UTC),
			want: "summer",
		},
		{
			name: "at fall equinox 2026 (00:05 UTC on Sep 23)",
			date: time.Date(2026, time.September, 23, 0, 5, 0, 0, time.UTC),
			want: "fall",
		},
		{
			name: "just before winter solstice 2026",
			date: time.Date(2026, time.December, 21, 20, 49, 59, 0, time.UTC),
			want: "fall",
		},
		{
			name: "at winter solstice 2026 (20:50 UTC)",
			date: time.Date(2026, time.December, 21, 20, 50, 0, 0, time.UTC),
			want: "winter",
		},
		{
			name: "works across time zones (local time converted to UTC)",
			// 2026-03-20 07:45 PDT (-7) == 2026-03-20 14:45 UTC
			date: time.Date(2026, time.March, 20, 7, 45, 0, 0, time.FixedZone("PDT", -7*3600)),
			want: "spring",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getCelestialSeason(tt.date)
			if got != tt.want {
				t.Fatalf("getCelestialSeason(%s) = %q, want %q", tt.date.Format(time.RFC3339), got, tt.want)
			}
		})
	}
}
