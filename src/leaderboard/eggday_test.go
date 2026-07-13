package leaderboard

import (
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

func TestEggDayLeaderboardsRegistry(t *testing.T) {
	// Verify that the new keys are registered
	if _, ok := LBDefByKey(LBEggDaySEGain); !ok {
		t.Fatalf("expected key %s to be registered", LBEggDaySEGain)
	}
	if _, ok := LBDefByKey(LBEggDaySEPct); !ok {
		t.Fatalf("expected key %s to be registered", LBEggDaySEPct)
	}

	// Verify group membership
	foundGain := false
	foundPct := false
	g, ok := GroupByKey("group_egg_day")
	if !ok {
		t.Fatalf("expected group group_egg_day to be registered")
	}
	for _, m := range g.Members {
		if m == LBEggDaySEGain {
			foundGain = true
		}
		if m == LBEggDaySEPct {
			foundPct = true
		}
	}
	if !foundGain || !foundPct {
		t.Fatalf("expected group_egg_day to contain both gain and pct leaderboards")
	}
}

func TestFormatEggDayPct(t *testing.T) {
	formatted := FormatLBValue("pct", 12.3456)
	expected := "12.35%"
	if formatted != expected {
		t.Fatalf("expected percent format to be %q, got %q", expected, formatted)
	}
}

func TestDetermineEggDayEndTime(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("failed to load timezone America/Los_Angeles: %v", err)
	}

	// Backup original events
	ei.EventMutex.Lock()
	origEvents := ei.EggIncEvents
	ei.EventMutex.Unlock()

	defer func() {
		ei.EventMutex.Lock()
		ei.EggIncEvents = origEvents
		ei.EventMutex.Unlock()
	}()

	year := 2026
	targetStart := time.Date(year, time.July, 14, 9, 0, 0, 0, loc)
	expectedEndTime := time.Date(year, time.July, 15, 9, 0, 0, 0, loc)

	// Create test events: one non-prestige-boost event (should be ignored), and one prestige-boost event
	testEvents := []ei.EggEvent{
		{
			EventType: "earnings-boost",
			StartTime: targetStart,
			EndTime:   expectedEndTime.Add(1 * time.Hour),
		},
		{
			EventType: "prestige-boost",
			StartTime: targetStart,
			EndTime:   expectedEndTime,
		},
	}

	ei.EventMutex.Lock()
	ei.EggIncEvents = testEvents
	ei.EventMutex.Unlock()

	// Calling determineEggDayEndTime should only detect the prestige-boost event
	gotEndTime := determineEggDayEndTime(nil, year, loc)
	if !gotEndTime.Equal(expectedEndTime) {
		t.Errorf("expected end time %s, got %s", expectedEndTime.Format(time.RFC3339), gotEndTime.Format(time.RFC3339))
	}
}
