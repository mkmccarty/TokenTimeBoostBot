package boost

import (
	"strings"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
)

func TestBuildSeasonNavButtons_UniqueCustomIDs(t *testing.T) {
	testScopes := []string{"spring_2026", "summer_2026", "summer_2025", "winter_2026", "fall_2024", "spring_2023"}

	for _, scope := range testScopes {
		session := &chartSession{
			uuidStr:     "test-uuid-1234",
			seasonScope: scope,
			percent:     -100,
		}

		navRow := buildSeasonNavButtons(session)
		if len(navRow.Components) != 5 {
			t.Errorf("scope %s: expected exactly 5 buttons, got %d", scope, len(navRow.Components))
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

func TestBuildSeasonNavButtons_CurrentSeasonDisabledOnCurrent(t *testing.T) {
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
	foundCurrent := false
	for _, comp := range navRow.Components {
		btn := comp.(dc.Button)
		if btn.Label == "Current Season" {
			foundCurrent = true
			if !btn.Disabled {
				t.Errorf("expected 'Current Season' button to be disabled when viewing current season %s", curScope)
			}
		}
	}
	if !foundCurrent {
		t.Errorf("expected 'Current Season' button to be present on current season %s", curScope)
	}
}

func TestBuildSeasonNavButtons_CurrentSeasonEnabledOnPrior(t *testing.T) {
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
			if btn.Disabled {
				t.Errorf("expected 'Current Season' button to be enabled when viewing prior season %s", prevScope)
			}
			if !strings.HasSuffix(btn.CustomID, "#current") {
				t.Errorf("expected Current Season button CustomID to have #current tag, got %s", btn.CustomID)
			}
		}
	}

	if !foundCurrent {
		t.Errorf("expected 'Current Season' button to be present when viewing prior season %s", prevScope)
	}
}

func TestBuildSeasonNavButtons_InvalidSeasonsDisabled(t *testing.T) {
	curName, curYear, ok := leaderboardMostRecentSeason()
	if !ok {
		t.Skip("no most recent season known")
	}
	curScope := leaderboardSeasonID(curName, curYear)

	session := &chartSession{
		uuidStr:     "test-uuid-curr",
		seasonScope: curScope,
		percent:     -100,
	}

	navRow := buildSeasonNavButtons(session)
	// Next season and next year on the current season should be disabled because they are future seasons
	var nextSeasonBtn, nextYearBtn dc.Button
	for _, comp := range navRow.Components {
		btn := comp.(dc.Button)
		if strings.HasSuffix(btn.CustomID, "#next_season") {
			nextSeasonBtn = btn
		} else if strings.HasSuffix(btn.CustomID, "#next_year") {
			nextYearBtn = btn
		}
	}

	if !nextSeasonBtn.Disabled {
		t.Errorf("expected next season button to be disabled on current season, got label %s disabled=%v", nextSeasonBtn.Label, nextSeasonBtn.Disabled)
	}
	if !nextYearBtn.Disabled {
		t.Errorf("expected next year button to be disabled on current season, got label %s disabled=%v", nextYearBtn.Label, nextYearBtn.Disabled)
	}
}

func TestBuildSeasonNavButtons_Positions(t *testing.T) {
	session := &chartSession{
		uuidStr:     "test-uuid-pos",
		seasonScope: "spring_2026",
		percent:     -100,
	}

	navRow := buildSeasonNavButtons(session)
	if len(navRow.Components) != 5 {
		t.Fatalf("expected 5 buttons, got %d", len(navRow.Components))
	}

	expectedTags := []string{"#prev_year", "#prev_season", "#next_season", "#next_year", "#current"}
	for i, tag := range expectedTags {
		btn := navRow.Components[i].(dc.Button)
		if !strings.HasSuffix(btn.CustomID, tag) {
			t.Errorf("button index %d: expected suffix %s, got CustomID %s", i, tag, btn.CustomID)
		}
	}
}

func TestHandleChartReactions_Finish(t *testing.T) {
	// 1. Expired session - Finish should not panic or fail, should succeed in removing components
	expiredEv := dctest.ComponentButtonEvent("chart#finish#expired-uuid-1234")
	HandleChartReactions(expiredEv)

	// 2. Active session with matching user - Finish should delete session
	chartSessionsMutex.Lock()
	chartSessions["active-uuid-5678"] = &chartSession{
		uuidStr: "active-uuid-5678",
		userID:  "4", // Matches user in dctest.ComponentButtonEvent
	}
	chartSessionsMutex.Unlock()

	activeEv := dctest.ComponentButtonEvent("chart#finish#active-uuid-5678")
	HandleChartReactions(activeEv)

	chartSessionsMutex.Lock()
	_, exists := chartSessions["active-uuid-5678"]
	chartSessionsMutex.Unlock()
	if exists {
		t.Errorf("expected active session to be deleted after finish")
	}

	// 3. Active session with different user - Finish should not delete session
	chartSessionsMutex.Lock()
	chartSessions["other-uuid-9999"] = &chartSession{
		uuidStr: "other-uuid-9999",
		userID:  "other-user",
	}
	chartSessionsMutex.Unlock()

	otherEv := dctest.ComponentButtonEvent("chart#finish#other-uuid-9999")
	HandleChartReactions(otherEv)

	chartSessionsMutex.Lock()
	_, stillExists := chartSessions["other-uuid-9999"]
	chartSessionsMutex.Unlock()
	if !stillExists {
		t.Errorf("expected session with different user to remain")
	}
}
