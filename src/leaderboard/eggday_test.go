package leaderboard

import "testing"

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
