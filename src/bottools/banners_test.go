package bottools

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
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

func TestRemoveGeneratedBannersForUserAndGuild(t *testing.T) {
	tempDir := t.TempDir()
	origOut := config.BannerOutputPath
	config.BannerOutputPath = tempDir
	t.Cleanup(func() { config.BannerOutputPath = origOut })

	// Create test files
	userBanner := filepath.Join(tempDir, "contract1-ba-user1_guild1.png")
	userBannerOther := filepath.Join(tempDir, "contract1-ba-user2_guild1.png")
	guildBanner := filepath.Join(tempDir, "contract1-ba-guild1.png")
	guildBannerOther := filepath.Join(tempDir, "contract1-ba-guild2.png")
	seasonBanner := filepath.Join(tempDir, "contract1-ba.png")

	for _, f := range []string{userBanner, userBannerOther, guildBanner, guildBannerOther, seasonBanner} {
		if err := os.WriteFile(f, []byte("fake"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}

	// Test user banner removal
	RemoveGeneratedBannersForUser("user1", "guild1")
	if _, err := os.Stat(userBanner); !os.IsNotExist(err) {
		t.Errorf("expected userBanner to be removed")
	}
	if _, err := os.Stat(userBannerOther); err != nil {
		t.Errorf("expected userBannerOther to remain")
	}

	// Test guild banner removal
	RemoveGeneratedBannersForGuild("guild1")
	if _, err := os.Stat(guildBanner); !os.IsNotExist(err) {
		t.Errorf("expected guildBanner to be removed")
	}
	if _, err := os.Stat(guildBannerOther); err != nil {
		t.Errorf("expected guildBannerOther to remain")
	}
	if _, err := os.Stat(seasonBanner); err != nil {
		t.Errorf("expected seasonBanner to remain")
	}
}
