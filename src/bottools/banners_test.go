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
			name: "summer starts june 21",
			date: time.Date(2026, time.June, 21, 0, 0, 0, 0, time.UTC),
			want: "summer",
		},
		{
			name: "fall starts september 22",
			date: time.Date(2026, time.September, 22, 0, 0, 0, 0, time.UTC),
			want: "fall",
		},
		{
			name: "winter starts december 21",
			date: time.Date(2026, time.December, 21, 0, 0, 0, 0, time.UTC),
			want: "winter",
		},
		{
			name: "winter before spring cutover",
			date: time.Date(2026, time.March, 19, 23, 59, 59, 0, time.UTC),
			want: "winter",
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
