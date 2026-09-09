package boost

import (
	"strings"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

func TestBuildSeasonNavButtons_UniqueCustomIDs(t *testing.T) {
	testScopes := []string{"spring_2026", "summer_2026", "summer_2025", "winter_2026", "fall_2024"}

	for _, scope := range testScopes {
		session := &chartSession{
			uuidStr:     "test-uuid-1234",
			seasonScope: scope,
			percent:     -100,
		}

		navRow := buildSeasonNavButtons(session)
		if len(navRow.Components) == 0 {
			continue
		}

		if len(navRow.Components) > 5 {
			t.Errorf("scope %s: expected <= 5 buttons, got %d", scope, len(navRow.Components))
		}

		seenIDs := make(map[string]bool)
		for _, comp := range navRow.Components {
			btn, ok := comp.(dc.Button)
			if !ok {
				t.Fatalf("scope %s: component is not dc.Button: %#v", scope, comp)
			}

			if btn.CustomID == "" {
				t.Errorf("scope %s: empty CustomID on button %q", scope, btn.Label)
			}

			if seenIDs[btn.CustomID] {
				t.Errorf("scope %s: duplicate CustomID %q found on button %q", scope, btn.CustomID, btn.Label)
			}
			seenIDs[btn.CustomID] = true

			// Validate prefix
			if !strings.HasPrefix(btn.CustomID, "chart#seasonswitch#test-uuid-1234#") {
				t.Errorf("scope %s: unexpected CustomID prefix on %q: %s", scope, btn.Label, btn.CustomID)
			}
		}
	}
}

func TestBuildSeasonNavButtons_CurrentSeasonOmittedOnCurrent(t *testing.T) {
	curName, curYear, ok := leaderboardMostRecentSeason()
	if !ok {
		t.Skip("no most recent season known")
	}
	curScope := leaderboardSeasonID(curName, curYear)

	session := &chartSession{
		uuidStr:     "test-uuid-5678",
		seasonScope: curScope,
		percent:     -100,
	}

	navRow := buildSeasonNavButtons(session)
	for _, comp := range navRow.Components {
		btn := comp.(dc.Button)
		if btn.Label == "Current Season" {
			t.Errorf("expected 'Current Season' button to be omitted when already viewing current season %s", curScope)
		}
	}
}

func TestBuildSeasonNavButtons_CurrentSeasonPresentOnPrior(t *testing.T) {
	curName, curYear, ok := leaderboardMostRecentSeason()
	if !ok {
		t.Skip("no most recent season known")
	}
	prevName, prevYear, ok := leaderboardPreviousSeason(curName, curYear)
	if !ok {
		t.Skip("no previous season known")
	}
	prevScope := leaderboardSeasonID(prevName, prevYear)

	session := &chartSession{
		uuidStr:     "test-uuid-9999",
		seasonScope: prevScope,
		percent:     -100,
	}

	navRow := buildSeasonNavButtons(session)
	foundCurrent := false
	for _, comp := range navRow.Components {
		btn := comp.(dc.Button)
		if btn.Label == "Current Season" {
			foundCurrent = true
			if !strings.HasSuffix(btn.CustomID, "#current") {
				t.Errorf("expected Current Season button CustomID to have #current tag, got %s", btn.CustomID)
			}
		}
	}

	if !foundCurrent {
		t.Errorf("expected 'Current Season' button to be present when viewing prior season %s", prevScope)
	}
}
