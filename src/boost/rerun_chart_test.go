package boost

import (
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
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

func TestPrintContractChart_ThresholdIncludesUnplayedContracts(t *testing.T) {
	ei.EggIncContractsMutex.Lock()
	origAll := ei.EggIncContractsAll
	ei.EggIncContractsAll = make(map[string]ei.EggIncContract)

	now := time.Now()
	idRun := "test-contract-run"
	idUnplayed := "test-contract-unplayed"
	idPerfect := "test-contract-perfect"
	idPred := "test-contract-pred"
	idV1 := "test-contract-v1"

	ei.EggIncContractsAll[idRun] = ei.EggIncContract{
		ID:              idRun,
		Name:            "Played Contract",
		ContractVersion: 2,
		CxpMax:          100000.0,
		ValidUntil:      now.Add(24 * time.Hour),
	}
	ei.EggIncContractsAll[idUnplayed] = ei.EggIncContract{
		ID:              idUnplayed,
		Name:            "Unplayed Contract",
		ContractVersion: 2,
		CxpMax:          80000.0,
		ValidUntil:      now.Add(24 * time.Hour),
	}
	ei.EggIncContractsAll[idPerfect] = ei.EggIncContract{
		ID:              idPerfect,
		Name:            "Perfect Contract",
		ContractVersion: 2,
		CxpMax:          100000.0,
		ValidUntil:      now.Add(24 * time.Hour),
	}
	ei.EggIncContractsAll[idPred] = ei.EggIncContract{
		ID:              idPred,
		Name:            "Predicted Contract",
		ContractVersion: 2,
		Predicted:       true,
		CxpMax:          90000.0,
		ValidUntil:      now.Add(24 * time.Hour),
	}
	ei.EggIncContractsAll[idV1] = ei.EggIncContract{
		ID:              idV1,
		Name:            "V1 Contract",
		ContractVersion: 1,
		CxpMax:          0.0,
		ValidUntil:      now.Add(24 * time.Hour),
	}
	ei.EggIncContractsMutex.Unlock()

	defer func() {
		ei.EggIncContractsMutex.Lock()
		ei.EggIncContractsAll = origAll
		ei.EggIncContractsMutex.Unlock()
	}()

	runCxp := 70000.0
	perfectCxp := 100000.0

	archive := []*ei.LocalContract{
		{
			ContractIdentifier: &idRun,
			Evaluation: &ei.ContractEvaluation{
				ContractIdentifier: &idRun,
				Cxp:                &runCxp,
			},
		},
		{
			ContractIdentifier: &idPerfect,
			Evaluation: &ei.ContractEvaluation{
				ContractIdentifier: &idPerfect,
				Cxp:                &perfectCxp,
			},
		},
	}

	// percent = 10 -> filters to < 90% (100 - 10) of speedrun potential
	components := printContractChart("user1", archive, 10, 1, nil, nil, false, "")
	if len(components) == 0 {
		t.Fatalf("expected non-empty components")
	}

	var allText strings.Builder
	for _, comp := range components {
		if td, ok := comp.(dc.TextDisplay); ok {
			allText.WriteString(td.Content)
			allText.WriteString("\n")
		}
	}
	fullOutput := allText.String()

	// Should include header with 90% (100 - 10)
	if !strings.Contains(fullOutput, "less than 90% of speedrun potential") {
		t.Errorf("expected header to contain 'less than 90%%%% of speedrun potential', got: %s", fullOutput)
	}

	// idRun had 70% < 90% -> should be included
	if !strings.Contains(fullOutput, idRun) {
		t.Errorf("expected played contract %s with 70%% score to be included", idRun)
	}

	// idUnplayed was NOT in archive -> should be included with 0% score (< 90%)
	if !strings.Contains(fullOutput, idUnplayed) {
		t.Errorf("expected unplayed contract %s to be included in threshold chart", idUnplayed)
	}

	// idPerfect had 100% >= 90% -> should NOT be included
	if strings.Contains(fullOutput, idPerfect) {
		t.Errorf("expected perfect score contract %s to be excluded", idPerfect)
	}

	// idPred was predicted -> should NOT be included
	if strings.Contains(fullOutput, idPred) {
		t.Errorf("expected predicted contract %s to be excluded", idPred)
	}

	// idV1 was version 1 -> should NOT be included
	if strings.Contains(fullOutput, idV1) {
		t.Errorf("expected v1 contract %s to be excluded", idV1)
	}
}
