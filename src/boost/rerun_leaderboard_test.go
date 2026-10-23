package boost

import (
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

func TestLeaderboardSeasonLabel_YearFirst(t *testing.T) {
	cases := map[string]string{
		"summer_2026": "2026 Summer",
		"spring_2026": "2026 Spring",
		"winter_2025": "2025 Winter",
		"fall_2024":   "2024 Fall",
	}
	for id, want := range cases {
		got := leaderboardSeasonLabel(id)
		if got != want {
			t.Errorf("leaderboardSeasonLabel(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestGetContractPotentialScore(t *testing.T) {
	c := ei.EggIncContract{
		ID:              "test-contract",
		ContractVersion: 2,
		Cxp:             90000.0,
		CxpMax:          100000.0,
		CxpMaxSiab:      105000.0,
		CxpMaxGG:        110000.0,
		CxpMaxSiabGG:    115000.0,
	}

	// 1. Leaderboard -> max(CxpMax, CxpMaxSiab) = 105000
	scoreLB := getContractPotentialScore(c, RerunLBModelLeaderboard)
	if scoreLB != 105000.0 {
		t.Errorf("expected Leaderboard score 105000, got %f", scoreLB)
	}

	// If CxpMax > CxpMaxSiab
	c2 := c
	c2.CxpMax = 120000.0
	c2.CxpMaxSiab = 105000.0
	if got := getContractPotentialScore(c2, RerunLBModelLeaderboard); got != 120000.0 {
		t.Errorf("expected Leaderboard score 120000, got %f", got)
	}

	// 2. Leaderboard Dubcap -> max(CxpMaxGG, CxpMaxSiabGG) = 115000
	scoreLBDubcap := getContractPotentialScore(c, RerunLBModelLeaderboardDubcap)
	if scoreLBDubcap != 115000.0 {
		t.Errorf("expected Leaderboard Dubcap score 115000, got %f", scoreLBDubcap)
	}

	// 3. Standard -> at least c.Cxp
	scoreStd := getContractPotentialScore(c, RerunLBModelStandard)
	if scoreStd < 90000.0 {
		t.Errorf("expected Standard score at least 90000, got %f", scoreStd)
	}

	// 4. ContractVersion 1 returns 0
	cV1 := c
	cV1.ContractVersion = 1
	if got := getContractPotentialScore(cV1, RerunLBModelLeaderboard); got != 0 {
		t.Errorf("expected V1 contract to return 0, got %f", got)
	}
}

func TestCalculateLeaderboardRerunRows(t *testing.T) {
	ei.EggIncContractsMutex.Lock()
	origAll := ei.EggIncContractsAll
	ei.EggIncContractsAll = make(map[string]ei.EggIncContract)

	now := time.Now()
	seasonA := "summer_2026"
	seasonB := "spring_2026"

	id1 := "contract-1"
	id2 := "contract-2"
	idBetter := "contract-better-score"

	ei.EggIncContractsAll[id1] = ei.EggIncContract{
		ID:              id1,
		Name:            "Contract 1",
		SeasonID:        seasonA,
		ContractVersion: 2,
		CxpMax:          100000.0,
		ValidUntil:      now.Add(24 * time.Hour),
	}
	ei.EggIncContractsAll[id2] = ei.EggIncContract{
		ID:              id2,
		Name:            "Contract 2",
		SeasonID:        seasonB,
		ContractVersion: 2,
		CxpMax:          90000.0,
		ValidUntil:      now.Add(-24 * time.Hour),
	}
	ei.EggIncContractsAll[idBetter] = ei.EggIncContract{
		ID:              idBetter,
		Name:            "Already Better Contract",
		SeasonID:        seasonB,
		ContractVersion: 2,
		CxpMax:          80000.0,
		ValidUntil:      now.Add(-24 * time.Hour),
	}
	ei.EggIncContractsMutex.Unlock()

	defer func() {
		ei.EggIncContractsMutex.Lock()
		ei.EggIncContractsAll = origAll
		ei.EggIncContractsMutex.Unlock()
	}()

	cur1 := 60000.0
	cur2 := 70000.0
	curBetter := 95000.0 // Player scored 95000, higher than max potential 80000

	archive := []*ei.LocalContract{
		{
			ContractIdentifier: &id1,
			Evaluation: &ei.ContractEvaluation{
				ContractIdentifier: &id1,
				SeasonId:           &seasonA,
				Cxp:                &cur1,
			},
		},
		{
			ContractIdentifier: &id2,
			Evaluation: &ei.ContractEvaluation{
				ContractIdentifier: &id2,
				SeasonId:           &seasonB,
				Cxp:                &cur2,
			},
		},
		{
			ContractIdentifier: &idBetter,
			Evaluation: &ei.ContractEvaluation{
				ContractIdentifier: &idBetter,
				SeasonId:           &seasonB,
				Cxp:                &curBetter,
			},
		},
	}

	rows := calculateLeaderboardRerunRows(archive, RerunLBModelLeaderboard)
	if len(rows) == 0 {
		t.Fatalf("expected rows to be non-empty")
	}

	// First row must be All Time
	allTime := rows[0]
	if allTime.seasonName != "All Time" {
		t.Errorf("expected first row to be 'All Time', got %s", allTime.seasonName)
	}

	// Current All Time score = 60000 + 70000 + 95000 = 225000
	if allTime.current != 225000.0 {
		t.Errorf("expected All Time current 225000, got %f", allTime.current)
	}

	// id1 potential 100000, cur 60000 -> gain = 40000
	// id2 potential 90000, cur 70000 -> gain = 20000
	// idBetter potential 80000 < cur 95000 -> gain = 0 (no negative values)
	// Total All Time delta = 40000 + 20000 = 60000
	if allTime.delta != 60000.0 {
		t.Errorf("expected All Time delta 60000, got %f", allTime.delta)
	}
	if allTime.sum != 285000.0 {
		t.Errorf("expected All Time sum 285000 (225000 + 60000), got %f", allTime.sum)
	}

	// Find seasonB row
	var seasonBRow *rerunLBRow
	for i := range rows {
		if rows[i].seasonID == seasonB {
			seasonBRow = &rows[i]
			break
		}
	}
	if seasonBRow == nil {
		t.Fatalf("expected seasonB row to exist")
	}
	// Season B has id2 (70000) and idBetter (95000) -> current = 165000
	if seasonBRow.current != 165000.0 {
		t.Errorf("expected seasonB current 165000, got %f", seasonBRow.current)
	}
	// id2 delta is 20000, idBetter delta is 0 -> delta = 20000
	if seasonBRow.delta != 20000.0 {
		t.Errorf("expected seasonB delta 20000, got %f", seasonBRow.delta)
	}
	if seasonBRow.sum != 185000.0 {
		t.Errorf("expected seasonB sum 185000, got %f", seasonBRow.sum)
	}
}

func TestCalculateLeaderboardRerunRows_ExcludesFirstContract(t *testing.T) {
	origAll := ei.EggIncContractsAll
	ei.EggIncContractsMutex.Lock()
	ei.EggIncContractsAll = make(map[string]ei.EggIncContract)
	id1 := "test-contract-1"
	seasonA := "summer_2024"
	now := time.Now()
	ei.EggIncContractsAll[id1] = ei.EggIncContract{
		ID:              id1,
		Name:            "Contract 1",
		SeasonID:        seasonA,
		ContractVersion: 2,
		CxpMax:          100000.0,
		ValidUntil:      now.Add(-24 * time.Hour),
	}
	ei.EggIncContractsMutex.Unlock()

	defer func() {
		ei.EggIncContractsMutex.Lock()
		ei.EggIncContractsAll = origAll
		ei.EggIncContractsMutex.Unlock()
	}()

	cur1 := 80000.0
	curFirst := 15289.0
	idFirst := "first-contract"

	archive := []*ei.LocalContract{
		{
			ContractIdentifier: &id1,
			Evaluation: &ei.ContractEvaluation{
				ContractIdentifier: &id1,
				SeasonId:           &seasonA,
				Cxp:                &cur1,
			},
		},
		{
			ContractIdentifier: &idFirst,
			Evaluation: &ei.ContractEvaluation{
				ContractIdentifier: &idFirst,
				Cxp:                &curFirst,
			},
		},
	}

	rows := calculateLeaderboardRerunRows(archive, RerunLBModelLeaderboard)
	if len(rows) == 0 {
		t.Fatalf("expected rows to be non-empty")
	}

	allTime := rows[0]
	// first-contract is a special intro contract that cannot be replayed and is excluded from rerun leaderboard.
	expectedCurrent := 80000.0
	if allTime.current != expectedCurrent {
		t.Errorf("expected All Time current %f, got %f", expectedCurrent, allTime.current)
	}
	// test-contract-1 delta is 100000 - 80000 = 20000
	if allTime.delta != 20000.0 {
		t.Errorf("expected All Time delta 20000, got %f", allTime.delta)
	}
	if allTime.sum != expectedCurrent+20000.0 {
		t.Errorf("expected All Time sum %f, got %f", expectedCurrent+20000.0, allTime.sum)
	}
}


func TestRenderLeaderboardRerunSession(t *testing.T) {
	dummyUser := "100000000000000001"
	now := time.Now()
	id := "test-render-contract"
	season := "summer_2026"
	score := 50000.0

	ei.EggIncContractsMutex.Lock()
	origAll := ei.EggIncContractsAll
	ei.EggIncContractsAll = map[string]ei.EggIncContract{
		id: {
			ID:              id,
			Name:            "Render Contract",
			SeasonID:        season,
			ContractVersion: 2,
			CxpMax:          100000.0,
			ValidUntil:      now.Add(24 * time.Hour),
		},
	}
	ei.EggIncContractsMutex.Unlock()

	defer func() {
		ei.EggIncContractsMutex.Lock()
		ei.EggIncContractsAll = origAll
		ei.EggIncContractsMutex.Unlock()
	}()

	archive := []*ei.LocalContract{
		{
			ContractIdentifier: &id,
			Evaluation: &ei.ContractEvaluation{
				ContractIdentifier: &id,
				SeasonId:           &season,
				Cxp:                &score,
			},
		},
	}

	session := &lbRerunSession{
		uuidStr:        "test-session-uuid",
		userID:         dummyUser,
		archive:        archive,
		model:          RerunLBModelLeaderboard,
		mobileFriendly: false,
		expiresAt:      now.Add(15 * time.Minute),
	}
	farmerstate.SetMiscSettingString(dummyUser, "ei_ign", "TestPlayer \ue10c")

	comps := renderLeaderboardRerunSession(session)
	if len(comps) == 0 {
		t.Fatalf("expected components to be non-empty")
	}

	var textBuilder strings.Builder
	for _, comp := range comps {
		if td, ok := comp.(dc.TextDisplay); ok {
			textBuilder.WriteString(td.Content)
			textBuilder.WriteString("\n")
		}
	}
	out := textBuilder.String()

	if !strings.Contains(out, "Potential Leaderboard Rerun") {
		t.Errorf("expected title in output, got: %s", out)
	}
	if !strings.Contains(out, "All Time") {
		t.Errorf("expected 'All Time' row in output, got: %s", out)
	}
	if !strings.Contains(out, "Current") {
		t.Errorf("expected 'Current' column in output, got: %s", out)
	}
	if !strings.Contains(out, "Rerun ∆") {
		t.Errorf("expected 'Rerun ∆' column in output, got: %s", out)
	}
	if !strings.Contains(out, "Sum") {
		t.Errorf("expected 'Sum' column in output, got: %s", out)
	}
	if strings.Contains(out, "Season |") || strings.Contains(out, "| Current") {
		t.Errorf("expected table without pipe column separators, got: %s", out)
	}
	if !strings.Contains(out, "TestPlayer 👽") {
		t.Errorf("expected normalized alien emoji in user header, got: %s", out)
	}
	if !strings.Contains(out, "Past season scores are fixed on the in-game leaderboard") {
		t.Errorf("expected past seasons footnote in table output, got: %s", out)
	}

	// Test Mobile Friendly rendering
	session.mobileFriendly = true
	compsMobile := renderLeaderboardRerunSession(session)
	textBuilder.Reset()
	for _, comp := range compsMobile {
		if td, ok := comp.(dc.TextDisplay); ok {
			textBuilder.WriteString(td.Content)
			textBuilder.WriteString("\n")
		}
	}
	outMobile := textBuilder.String()
	if !strings.Contains(outMobile, "**All Time**") || !strings.Contains(outMobile, "Rerun ∆:") || !strings.Contains(outMobile, "Sum:") {
		t.Errorf("expected mobile format with Rerun ∆ and Sum, got: %s", outMobile)
	}
	if !strings.Contains(outMobile, "Past season scores are fixed on the in-game leaderboard") {
		t.Errorf("expected past seasons footnote in mobile output, got: %s", outMobile)
	}
}

func TestHandleLeaderboardRerunReactions(t *testing.T) {
	dummyUser := "100000000000000001"
	sessionID := "test-react-session"

	session := &lbRerunSession{
		uuidStr:        sessionID,
		userID:         dummyUser,
		archive:        nil,
		model:          RerunLBModelLeaderboard,
		mobileFriendly: false,
		expiresAt:      time.Now().Add(15 * time.Minute),
	}

	lbRerunSessionsMutex.Lock()
	lbRerunSessions[sessionID] = session
	lbRerunSessionsMutex.Unlock()

	// 1. Toggle view
	toggleEv := dctest.ComponentButtonEventWithUser("rerun_lb#toggleview#"+sessionID, dummyUser)
	HandleLeaderboardRerunReactions(toggleEv)

	if !session.mobileFriendly {
		t.Errorf("expected mobileFriendly to be true after toggle")
	}
	stickyMobile := farmerstate.GetMiscSettingString(dummyUser, "rerunMobileFriendly")
	if stickyMobile != "true" {
		t.Errorf("expected sticky rerunMobileFriendly to be true, got %s", stickyMobile)
	}

	// 2. Model change
	modelEv := dctest.ComponentSelectEventWithUser("rerun_lb#model#"+sessionID, dummyUser, RerunLBModelStandardDubcap)
	HandleLeaderboardRerunReactions(modelEv)

	if session.model != RerunLBModelStandardDubcap {
		t.Errorf("expected model %s, got %s", RerunLBModelStandardDubcap, session.model)
	}
	stickyModel := farmerstate.GetMiscSettingString(dummyUser, rerunLBModelMiscKey)
	if stickyModel != RerunLBModelStandardDubcap {
		t.Errorf("expected sticky model %s, got %s", RerunLBModelStandardDubcap, stickyModel)
	}

	// 3. Unauthorized user
	unauthEv := dctest.ComponentButtonEventWithUser("rerun_lb#toggleview#"+sessionID, "100000000000000002")
	HandleLeaderboardRerunReactions(unauthEv)
	// mobileFriendly should remain true (unchanged)
	if !session.mobileFriendly {
		t.Errorf("expected mobileFriendly to remain true after unauthorized attempt")
	}

	// 4. Close
	closeEv := dctest.ComponentButtonEventWithUser("rerun_lb#close#"+sessionID, dummyUser)
	HandleLeaderboardRerunReactions(closeEv)

	lbRerunSessionsMutex.Lock()
	_, exists := lbRerunSessions[sessionID]
	lbRerunSessionsMutex.Unlock()
	if exists {
		t.Errorf("expected session to be deleted after close")
	}
}

func TestLeaderboardSeasons_DynamicNewSeasons(t *testing.T) {
	// 1. Test when periodicals arrives with a brand new season not in fallback
	origSeason := ei.EggIncCurrentSeason
	defer func() {
		ei.EggIncCurrentSeason = origSeason
	}()

	ei.SetEggIncCurrentSeason("fall_2026", "Fall 2026", 1780000000)

	seasons := leaderboardSeasons()
	if len(seasons) < 2 {
		t.Fatalf("expected at least 2 seasons, got %d", len(seasons))
	}
	// Season after "All Time" must be "2026 Fall"
	if seasons[1].value != "fall_2026" || seasons[1].name != "2026 Fall" {
		t.Errorf("expected 2026 Fall at seasons[1], got %+v", seasons[1])
	}

	// 2. Test when a new contract arrives with an even newer future season (e.g. winter_2027)
	futureContract := ei.EggIncContract{
		ID:              "future-contract-2027",
		ContractVersion: 2,
		SeasonID:        "winter_2027",
	}
	ei.EggIncContractsMutex.Lock()
	origAll := ei.EggIncContractsAll
	ei.EggIncContractsAll = map[string]ei.EggIncContract{
		futureContract.ID: futureContract,
	}
	ei.EggIncContractsMutex.Unlock()
	defer func() {
		ei.EggIncContractsMutex.Lock()
		ei.EggIncContractsAll = origAll
		ei.EggIncContractsMutex.Unlock()
	}()

	seasons2 := leaderboardSeasons()
	if len(seasons2) < 2 {
		t.Fatalf("expected at least 2 seasons with future contract, got %d", len(seasons2))
	}
	if seasons2[1].value != "winter_2027" || seasons2[1].name != "2027 Winter" {
		t.Errorf("expected 2027 Winter at seasons2[1], got %+v", seasons2[1])
	}

	// 3. Test extra seasons from user archive (e.g. spring_2027)
	seasons3 := leaderboardSeasons("spring_2027")
	if len(seasons3) < 2 {
		t.Fatalf("expected at least 2 seasons with extra season, got %d", len(seasons3))
	}
	if seasons3[1].value != "spring_2027" || seasons3[1].name != "2027 Spring" {
		t.Errorf("expected 2027 Spring at seasons3[1], got %+v", seasons3[1])
	}

	// 4. Test dynamic earliest and latest season bounds derived solely from contracts
	ei.SetEggIncCurrentSeason("", "", 0)
	cEarly := ei.EggIncContract{
		ID:              "early-contract-2024",
		ContractVersion: 2,
		SeasonID:        "summer_2024",
	}
	cLate := ei.EggIncContract{
		ID:              "late-contract-2025",
		ContractVersion: 2,
		SeasonID:        "winter_2025",
	}
	ei.EggIncContractsMutex.Lock()
	ei.EggIncContractsAll = map[string]ei.EggIncContract{
		cEarly.ID: cEarly,
		cLate.ID:  cLate,
	}
	ei.EggIncContractsMutex.Unlock()

	earlyName, earlyYear := leaderboardEarliestSeason()
	if earlyName != "summer" || earlyYear != 2024 {
		t.Errorf("expected earliest season summer 2024, got %s %d", earlyName, earlyYear)
	}

	boundedSeasons := leaderboardFallbackSeasons()
	// Should have "All Time", "2025 Winter", "2024 Fall", "2024 Summer" = 4 entries
	if len(boundedSeasons) != 4 {
		t.Errorf("expected 4 bounded seasons, got %d: %+v", len(boundedSeasons), boundedSeasons)
	}
	if len(boundedSeasons) > 0 && boundedSeasons[len(boundedSeasons)-1].value != "summer_2024" {
		t.Errorf("expected oldest season to be summer_2024, got %s", boundedSeasons[len(boundedSeasons)-1].value)
	}
}

func TestContractPotentialScore_ModelHierarchy(t *testing.T) {
	makeGrade := func() []ei.ContractGrade {
		g := make([]ei.ContractGrade, int(ei.Contract_GRADE_AAA)+1)
		g[ei.Contract_GRADE_AAA] = ei.ContractGrade{
			TargetAmount:    []float64{1e15},
			LengthInSeconds: 86400,
		}
		return g
	}

	cases := []struct {
		name string
		c    ei.EggIncContract
	}{
		{
			name: "higher SIAB than CxpMax",
			c: ei.EggIncContract{
				ID:                     "test-hierarchy-1",
				ContractVersion:        2,
				MaxCoopSize:            5,
				TargetAmount:           []float64{1e15},
				Grade:                  makeGrade(),
				Cxp:                    85000,
				CxpMax:                 95000,
				CxpMaxSiab:             100000,
				CxpMaxGG:               102000,
				CxpMaxSiabGG:           108000,
				EstimatedDurationMax:   10 * time.Hour,
				EstimatedDurationMaxGG: 9 * time.Hour,
				EstimatedDurationLower: 12 * time.Hour,
			},
		},
		{
			name: "higher CxpMax than SIAB",
			c: ei.EggIncContract{
				ID:                     "test-hierarchy-2",
				ContractVersion:        2,
				MaxCoopSize:            5,
				TargetAmount:           []float64{1e15},
				Grade:                  makeGrade(),
				Cxp:                    90000,
				CxpMax:                 105000,
				CxpMaxSiab:             100000,
				CxpMaxGG:               115000,
				CxpMaxSiabGG:           110000,
				EstimatedDurationMax:   8 * time.Hour,
				EstimatedDurationMaxGG: 7 * time.Hour,
				EstimatedDurationLower: 10 * time.Hour,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			std := getContractPotentialScore(tc.c, RerunLBModelStandard)
			stdDubcap := getContractPotentialScore(tc.c, RerunLBModelStandardDubcap)
			lb := getContractPotentialScore(tc.c, RerunLBModelLeaderboard)
			lbDubcap := getContractPotentialScore(tc.c, RerunLBModelLeaderboardDubcap)

			if std > lb {
				t.Errorf("expected Standard (%.0f) <= Leaderboard (%.0f)", std, lb)
			}
			if stdDubcap > lbDubcap {
				t.Errorf("expected Standard Dubcap (%.0f) <= Leaderboard Dubcap (%.0f)", stdDubcap, lbDubcap)
			}
			if std > stdDubcap {
				t.Errorf("expected Standard (%.0f) <= Standard Dubcap (%.0f)", std, stdDubcap)
			}
			if lb > lbDubcap {
				t.Errorf("expected Leaderboard (%.0f) <= Leaderboard Dubcap (%.0f)", lb, lbDubcap)
			}
		})
	}
}
