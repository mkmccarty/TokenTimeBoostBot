package boost

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

// Make sure to use the testing package
func TestDetermineDynamicTokens(t *testing.T) {

	// Create a contract with 10 farmers
	c := new(Contract)
	c.Style = ContractFlagDynamicTokens
	c.Boosters = make(map[string]*Booster)
	c.StartTime = time.Now()
	c.CoopSize = 10
	c.BoostPosition = 2

	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("Farmer%d", i)

		b := new(Booster)
		b.Name = name
		b.UserID = name
		c.Boosters[name] = b
		// Add this user to the boost order
		c.Order = append(c.Order, name)
	}

	// Mark first booster as boosted
	c.Boosters[c.Order[0]].BoostState = BoostStateBoosted
	c.Boosters[c.Order[0]].StartTime = c.StartTime.Add(time.Minute * 10)
	c.Boosters[c.Order[0]].EndTime = c.Boosters[c.Order[0]].StartTime.Add(time.Minute * 10)

	c.Boosters[c.Order[1]].BoostState = BoostStateBoosted
	c.Boosters[c.Order[1]].StartTime = c.Boosters[c.Order[0]].EndTime // After previous booster finishes
	c.Boosters[c.Order[1]].EndTime = c.Boosters[c.Order[1]].StartTime.Add(time.Minute * 10)

	// Create a dummy token log to fake how many tokens we've received
	for i := 0; i < 20; i++ {
		c.TokenLog = append(c.TokenLog, ei.TokenUnitLog{})
	}

	determineDynamicTokens(c)
}

func TestDynamicTokens(t *testing.T) {

	dt := createDynamicTokenData(50)

	boostDur, crDur := getBoostTimeSeconds(dt, 6)
	if boostDur <= 0 || crDur <= 0 {
		t.Errorf("getBoostTimeSeconds() = (%v, %v), want > 0", boostDur, crDur)
	}

	// Initially assign 6 token boosts to everyone,
	// In reverse order start calculating using 8 token boosts
	// stop when the 120 minute delivered eggs amount is less than the previous amount
	result := calculateDynamicTokens(dt, 4, 0.39, 0)

	// result should be 4 integers with the the last one being an 8
	if len(result) != 4 {
		t.Errorf("calculateDynamicTokens() = %v, want %v", len(result), 4)
	}

	if result[0] != 6 {
		t.Errorf("calculateDynamicTokens() = %v, want %v", result[0], 6)
	}

	if result[1] != 6 {
		t.Errorf("calculateDynamicTokens() = %v, want %v", result[1], 6)
	}

	if result[2] != 6 {

		t.Errorf("calculateDynamicTokens() = %v, want %v", result[2], 6)
	}

	if result[3] != 8 {

		t.Errorf("calculateDynamicTokens() = %v, want %v", result[3], 8)
	}

}

func TestGetExpectedBoostIcons(t *testing.T) {
	tests := []struct {
		tokens       int
		wantContains []string
	}{
		{0, []string{"b_tach10x4h", "(4h)"}},
		{1, []string{"b_boost2x30m", "(30m)", "b_tach10x4h", "(4h)"}},
		{2, []string{"b_tach100x20m", "(20m)", "b_tach10x4h", "(4h)"}},
		{3, []string{"b_tach100x20m", "(20m)", "b_boost2x30m", "(30m)", "b_tach10x4h", "(4h)"}},
		{4, []string{"b_tach1000x10m", "(10m)", "b_tach10x4h", "(4h)"}},
		{5, []string{"b_tach1000x10m", "(10m)", "b_boost2x30m", "(30m)", "b_tach10x4h", "(4h)"}},
		{6, []string{"b_tach1000x10m", "(10m)", "b_boost2x30m", "(30m)", "b_tach10x4h", "(4h)"}},
		{7, []string{"b_tach1000x10m", "(10m)", "b_boost2x30m", "(30m)", "b_tach10x4h", "(4h)"}},
		{8, []string{"b_tach1000x10m", "(10m)", "b_boost10x10m", "(10m)", "b_tach10x4h", "(4h)"}},
		{9, []string{"b_tach1000x10m", "(10m)", "b_boost10x10m", "b_boost2x30m", "(30m)", "b_tach10x4h", "(4h)"}},
		{10, []string{"b_tach1000x10m", "(10m)", "b_boost10x10m", "b_boost2x30m", "(30m)", "b_tach10x4h", "(4h)"}},
		{11, []string{"b_tach1000x10m", "(10m)", "b_boost10x10m", "b_boost2x30m", "(30m)"}},
		{12, []string{"b_tach1000x10m", "(10m)", "b_boost50x10m", "(10m)", "b_tach10x4h", "(4h)"}},
	}

	for _, tc := range tests {
		got := getExpectedBoostIcons(tc.tokens)
		for _, sub := range tc.wantContains {
			if !strings.Contains(got, sub) {
				t.Errorf("getExpectedBoostIcons(%d) = %q, missing %q", tc.tokens, got, sub)
			}
		}
		if tc.tokens >= 4 {
			gotLong := getExpectedBoostIcons(tc.tokens, 11*time.Minute)
			if !strings.Contains(gotLong, "b_tach1000x1h") || !strings.Contains(gotLong, "(1h)") {
				t.Errorf("getExpectedBoostIcons(%d, 11m) = %q, expected b_tach1000x1h (1h)", tc.tokens, gotLong)
			}
			if strings.Contains(gotLong, "b_tach1000x10m") {
				t.Errorf("getExpectedBoostIcons(%d, 11m) = %q, should not contain b_tach1000x10m", tc.tokens, gotLong)
			}
		}
	}
}

func TestCalculateBoostSchedule_TruckChecksAndOfflineMinimum(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)

	contract := &Contract{
		ContractID: "test-sched-contract",
		CoopSize:   4,
		PlayStyle:  ContractPlaystyleFastrun,
	}

	// 4-token boost takes > 3m40s to reach CR population, triggering truck checks
	booster4 := &Booster{
		UserID:       "100000000000000001",
		TokensWanted: 4,
		TECount:      20,
	}
	sched4 := calculateBoostSchedule(contract, booster4, now)

	if sched4.CRDuration != sched4.PureCRDuration {
		t.Errorf("expected CRDuration=%v (pure offline boost time), got %v", sched4.PureCRDuration, sched4.CRDuration)
	}
	if sched4.PostCROfflineDuration < time.Minute {
		t.Errorf("expected PostCROfflineDuration >= 1m, got %v", sched4.PostCROfflineDuration)
	}
	if sched4.TotalBoostDuration != sched4.CRDuration+sched4.PostCROfflineDuration {
		t.Errorf("expected TotalBoostDuration=%v, got %v", sched4.CRDuration+sched4.PostCROfflineDuration, sched4.TotalBoostDuration)
	}
	if sched4.TotalBoostDuration > 10*time.Minute && !strings.Contains(sched4.BoostIcons, "b_tach1000x1h") {
		t.Errorf("expected >10m 4-token boost (%v) to select b_tach1000x1h, got %q", sched4.TotalBoostDuration, sched4.BoostIcons)
	}

	// High-token boost in a large coop where CRs + 1m offline exceed 30% of maxPop:
	// PostCROfflineDuration must still be at least 1 minute and CRRequestPopulation adjusted down
	largeCoop := &Contract{
		ContractID: "test-sched-large",
		CoopSize:   10,
		PlayStyle:  ContractPlaystyleLeaderboard,
	}
	booster8 := &Booster{
		UserID:       "100000000000000002",
		TokensWanted: 8,
		TECount:      50,
	}
	sched8 := calculateBoostSchedule(largeCoop, booster8, now)
	if sched8.PostCROfflineDuration < time.Minute {
		t.Errorf("expected PostCROfflineDuration >= 1m in large coop, got %v", sched8.PostCROfflineDuration)
	}
	// Verify that CRRequestPopulation accounts for the 1 minute of offline chickens gained after requesting CR
	populationForCR := sched8.MaxShippingPopulation * (DefaultLeggyChickenRunPercent / 100.0)
	crFromCoop := populationForCR * 0.05 * 9.0
	oneMinOfflineChickens := sched8.OfflineIHRPerMin * 1.0
	wantCRPop := sched8.MaxShippingPopulation - crFromCoop - oneMinOfflineChickens
	if sched8.CRRequestPopulation > populationForCR {
		t.Errorf("expected CRRequestPopulation <= %v, got %v", populationForCR, sched8.CRRequestPopulation)
	}
	if diff := sched8.CRRequestPopulation - wantCRPop; diff < -1.0 || diff > 1.0 {
		t.Errorf("expected CRRequestPopulation=%v (accounting for 1m offline chickens), got %v", wantCRPop, sched8.CRRequestPopulation)
	}

	formatted := formatBoostSchedule(largeCoop, booster8, sched8)
	for _, expected := range []string{
		"Boost Schedule",
		"Total Boost Time:",
		"Request Chicken Runs",
		"Boosts:",
		"Post-Boost Delivery Set:",
		"Projected Rates:",
		"Delivery:",
		"LR:",
		"SR:",
		"b_tach1000x10m",
		"b_boost10x10m",
		"Reach CR Population",
		"Offline to Max Shipping",
	} {
		if !strings.Contains(formatted, expected) {
			t.Errorf("formatBoostSchedule output missing %q:\n%s", expected, formatted)
		}
	}
	if sched8.TachyonStones+sched8.QuantumStones != 10 {
		t.Errorf("expected 10 total delivery stones for default leggy set, got %d T + %d Q", sched8.TachyonStones, sched8.QuantumStones)
	}
	if sched8.DeliveryRate <= 0 || sched8.LayRate <= 0 || sched8.ShipRate <= 0 {
		t.Errorf("expected positive projected rates, got Delivery=%v, LR=%v, SR=%v", sched8.DeliveryRate, sched8.LayRate, sched8.ShipRate)
	}
}

func TestCalculateBoostSchedule_UsesKnownArtifacts(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	contract := &Contract{
		ContractID: "test-sched-artifacts",
		CoopSize:   4,
		PlayStyle:  ContractPlaystyleFastrun,
	}

	boosterLeggy := &Booster{
		UserID:       "100000000000000003",
		TokensWanted: 6,
		ArtifactSet: ArtifactSet{
			Artifacts: []ei.Artifact{
				{Type: "Deflector", Quality: "T4L", DeflBuff: 1.20, Stones: 2},
				{Type: "Metronome", Quality: "T4L", LayBuff: 1.35, Stones: 3},
				{Type: "Compass", Quality: "T4L", ShipBuff: 1.50, Stones: 3},
				{Type: "Gusset", Quality: "T4L", LayBuff: 1.25, Stones: 3},
				{Type: "Chalice", Quality: "T4L", Stones: 3},
				{Type: "Monocle", Quality: "T4L", Stones: 3},
				{Type: "SIAB", Quality: "T4L", Stones: 2},
			},
		},
	}

	boosterLower := &Booster{
		UserID:       "100000000000000004",
		TokensWanted: 6,
		ArtifactSet: ArtifactSet{
			Artifacts: []ei.Artifact{
				{Type: "Deflector", Quality: "T3C", DeflBuff: 1.12, Stones: 0},
				{Type: "Metronome", Quality: "T3C", LayBuff: 1.10, Stones: 0},
				{Type: "Compass", Quality: "T3C", ShipBuff: 1.20, Stones: 0},
				{Type: "Gusset", Quality: "T2E", LayBuff: 1.12, Stones: 0},
				{Type: "Chalice", Quality: "T2C", Stones: 0},
				{Type: "Monocle", Quality: "T2C", Stones: 0},
			},
		},
	}

	schedLeggy := calculateBoostSchedule(contract, boosterLeggy, now)
	schedLower := calculateBoostSchedule(contract, boosterLower, now)

	if schedLeggy.OfflineIHRPerMin <= schedLower.OfflineIHRPerMin {
		t.Errorf("expected leggy IHR (%v) > lower artifact IHR (%v)", schedLeggy.OfflineIHRPerMin, schedLower.OfflineIHRPerMin)
	}
	if schedLower.TotalBoostDuration <= schedLeggy.TotalBoostDuration {
		t.Errorf("expected lower artifacts to take longer (%v) than leggy artifacts (%v)", schedLower.TotalBoostDuration, schedLeggy.TotalBoostDuration)
	}
	if schedLeggy.DeflectorIsActual {
		t.Errorf("expected DeflectorIsActual=false when other farmers' deflectors are unknown")
	}
	if schedLeggy.DeflectorPercent != 60 {
		t.Errorf("expected estimated DeflectorPercent=60 for 4-player coop, got %v", schedLeggy.DeflectorPercent)
	}
	estFormatted := formatBoostSchedule(contract, boosterLeggy, schedLeggy)
	if !strings.Contains(estFormatted, "`60%` (estimated)") {
		t.Errorf("expected `60%%` (estimated) in formatted output, got:\n%s", estFormatted)
	}

	// Now populate known deflectors for all 3 other farmers in a 4-player coop (20% + 19% + 15% = 54%)
	contractWithDefl := &Contract{
		ContractID: "test-sched-actual-defl",
		CoopSize:   4,
		PlayStyle:  ContractPlaystyleFastrun,
		Boosters: map[string]*Booster{
			boosterLeggy.UserID: boosterLeggy,
			"100000000000000005": {
				UserID: "100000000000000005",
				ArtifactSet: ArtifactSet{
					Artifacts: []ei.Artifact{{Type: "Deflector", Quality: "T4L", DeflBuff: 1.20}},
				},
			},
			"100000000000000006": {
				UserID: "100000000000000006",
				ArtifactSet: ArtifactSet{
					Artifacts: []ei.Artifact{{Type: "Deflector", Quality: "T4E", DeflBuff: 1.19}},
				},
			},
			"100000000000000007": {
				UserID: "100000000000000007",
				ArtifactSet: ArtifactSet{
					Artifacts: []ei.Artifact{{Type: "Deflector", Quality: "T4C", DeflBuff: 1.15}},
				},
			},
		},
	}
	schedActual := calculateBoostSchedule(contractWithDefl, boosterLeggy, now)
	if !schedActual.DeflectorIsActual {
		t.Errorf("expected DeflectorIsActual=true when all other farmers have known deflectors")
	}
	if schedActual.DeflectorPercent != 54 {
		t.Errorf("expected actual DeflectorPercent=54 (20+19+15), got %v", schedActual.DeflectorPercent)
	}
	actFormatted := formatBoostSchedule(contractWithDefl, boosterLeggy, schedActual)
	if !strings.Contains(actFormatted, "`54%` (actual)") {
		t.Errorf("expected `54%%` (actual) in formatted output, got:\n%s", actFormatted)
	}
	if strings.Contains(actFormatted, "Post-Boost Delivery SIAB Set:") {
		t.Errorf("did not expect SIAB set when CxpMaxSiab is not greater than CxpMax, got:\n%s", actFormatted)
	}

	// Contract where Leaderboard max score with SIAB is greater than without SIAB
	if ei.EggIncContractsAll == nil {
		ei.EggIncContractsAll = make(map[string]ei.EggIncContract)
	}
	siabContractID := "test-sched-siab-lb"
	ei.EggIncContractsAll[siabContractID] = ei.EggIncContract{
		ID:         siabContractID,
		CxpMax:     100000,
		CxpMaxSiab: 105000,
	}
	defer delete(ei.EggIncContractsAll, siabContractID)

	contractWithSIAB := &Contract{
		ContractID: siabContractID,
		CoopSize:   4,
		PlayStyle:  ContractPlaystyleLeaderboard,
		Boosters:   contractWithDefl.Boosters,
	}
	schedSIAB := calculateBoostSchedule(contractWithSIAB, boosterLeggy, now)
	if !schedSIAB.ShowSIABSet {
		t.Fatalf("expected ShowSIABSet=true when CxpMaxSiab > CxpMax")
	}
	if !strings.Contains(schedSIAB.SIABDeliveryArtifacts, "SIAB_T4L") || strings.Contains(schedSIAB.SIABDeliveryArtifacts, "GUSSET_") {
		t.Errorf("expected SIABDeliveryArtifacts to use SIAB_T4L instead of Gusset, got %q", schedSIAB.SIABDeliveryArtifacts)
	}
	if schedSIAB.SIABTachyonStones+schedSIAB.SIABQuantumStones != 10 {
		t.Errorf("expected 10 total SIAB delivery stones (2+3+3+2) for boosterLeggy, got %d T + %d Q",
			schedSIAB.SIABTachyonStones, schedSIAB.SIABQuantumStones)
	}
	siabFormatted := formatBoostSchedule(contractWithSIAB, boosterLeggy, schedSIAB)
	for _, want := range []string{
		"Post-Boost Delivery SIAB Set:",
		"Projected SIAB Rates:",
		"SIAB_T4L",
	} {
		if !strings.Contains(siabFormatted, want) {
			t.Errorf("expected %q in SIAB formatted output, got:\n%s", want, siabFormatted)
		}
	}
}

func TestUpdateCoopDeflectorsFromStatus(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	hash := "test-coop-status-deflectors"
	chanID := "200000000000000022"
	listMsgID := "300000000000000022"
	contract := setupBoostReactionTestContract(hash, chanID, listMsgID, ContractPlaystyleFastrun)
	contract.CoopSize = 3
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, hash)
		ContractsMutex.Unlock()
	}()

	origFetch := fetchCoopStatusForDeflectors
	defer func() {
		fetchCoopStatusForDeflectors = origFetch
	}()

	deflName := ei.ArtifactSpec_TACHYON_DEFLECTOR
	lvlT4 := ei.ArtifactSpec_GREATER
	rarLeg := ei.ArtifactSpec_LEGENDARY
	rarEpic := ei.ArtifactSpec_EPIC
	buffT4C := 1.15

	pA := "PlayerA"
	pB := "PlayerB"
	pC := "PlayerC"

	fetchCoopStatusForDeflectors = func(contractID, coopID, eeidOverride string) (*ei.ContractCoopStatusResponse, error) {
		return &ei.ContractCoopStatusResponse{
			Contributors: []*ei.ContractCoopStatusResponse_ContributionInfo{
				{
					UserName: &pA,
					FarmInfo: &ei.PlayerFarmInfo{
						EquippedArtifacts: []*ei.CompleteArtifact{
							{
								Spec: &ei.ArtifactSpec{
									Name:   &deflName,
									Level:  &lvlT4,
									Rarity: &rarLeg,
								},
							},
						},
					},
				},
				{
					UserName: &pB,
					FarmInfo: &ei.PlayerFarmInfo{
						EquippedArtifacts: []*ei.CompleteArtifact{
							{
								Spec: &ei.ArtifactSpec{
									Name:   &deflName,
									Level:  &lvlT4,
									Rarity: &rarEpic,
								},
							},
						},
					},
				},
				{
					UserName: &pC,
					BuffHistory: []*ei.CoopBuffState{
						{
							EggLayingRate: &buffT4C,
						},
					},
				},
			},
		}, nil
	}

	updateCoopDeflectorsFromStatus(contract)

	// PlayerB (2nd booster) should see PlayerA (20%) + PlayerC (15%) = 35% actual deflector bonus
	userB := "400000000000000012"
	schedB := calculateBoostSchedule(contract, contract.Boosters[userB], now)
	if !schedB.DeflectorIsActual {
		t.Errorf("expected DeflectorIsActual=true after coop_status update")
	}
	if schedB.DeflectorPercent != 35 {
		t.Errorf("expected DeflectorPercent=35 (20%% + 15%%), got %v", schedB.DeflectorPercent)
	}

	// Development scenario: coop does not exist in-game (COOP_NOT_FOUND) -> fall back to guessing
	devHash := "test-dev-no-coop"
	devContract := setupBoostReactionTestContract(devHash, chanID, listMsgID, ContractPlaystyleFastrun)
	devContract.CoopSize = 4
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, devHash)
		ContractsMutex.Unlock()
	}()

	noCoopStatus := ei.ContractCoopStatusResponse_COOP_NOT_FOUND
	fetchCoopStatusForDeflectors = func(contractID, coopID, eeidOverride string) (*ei.ContractCoopStatusResponse, error) {
		return &ei.ContractCoopStatusResponse{
			ResponseStatus: &noCoopStatus,
		}, nil
	}

	updateCoopDeflectorsFromStatus(devContract)

	schedDev := calculateBoostSchedule(devContract, devContract.Boosters[userB], now)
	if schedDev.DeflectorIsActual {
		t.Errorf("expected DeflectorIsActual=false when coop_status returns COOP_NOT_FOUND")
	}
	if schedDev.DeflectorPercent != 60 {
		t.Errorf("expected estimated DeflectorPercent=60 (3 * 20%%) when no in-game coop exists, got %v", schedDev.DeflectorPercent)
	}
}

func setupBoostReactionTestContract(hash, chanID, listMsgID string, playStyle int) *Contract {
	userA := "400000000000000011"
	userB := "400000000000000012"
	userC := "400000000000000013"
	coordID := "400000000000000099"

	c := &Contract{
		ContractHash: hash,
		ContractID:   "test-boost-sched",
		CoopID:       "test-coop",
		CoopSize:     4,
		State:        ContractStateFastrun,
		PlayStyle:    playStyle,
		CreatorID:    []string{coordID},
		Location: []*LocationData{
			{
				GuildID:    "100000000000000001",
				ChannelID:  chanID,
				ListMsgID:  listMsgID,
				MessageIDs: []string{listMsgID},
			},
		},
		Boosters: map[string]*Booster{
			userA: {
				UserID:       userA,
				Name:         "PlayerA",
				Nick:         "PlayerA",
				Mention:      "<@" + userA + ">",
				BoostState:   BoostStateTokenTime,
				TokensWanted: 6,
				TECount:      40,
				StartTime:    time.Now().Add(-2 * time.Minute),
			},
			userB: {
				UserID:       userB,
				Name:         "PlayerB",
				Nick:         "PlayerB",
				Mention:      "<@" + userB + ">",
				BoostState:   BoostStateUnboosted,
				TokensWanted: 8,
				TECount:      50,
			},
			userC: {
				UserID:       userC,
				Name:         "PlayerC",
				Nick:         "PlayerC",
				Mention:      "<@" + userC + ">",
				BoostState:   BoostStateUnboosted,
				TokensWanted: 6,
				TECount:      30,
			},
		},
		Order:                []string{userA, userB, userC},
		BoostPosition:        0,
		CurrentBoosterUserID: userA,
	}

	ContractsMutex.Lock()
	Contracts[hash] = c
	ContractsMutex.Unlock()
	return c
}

func TestBoostSchedule_RocketAndFireReactions(t *testing.T) {
	guildID := "100000000000000001"
	chanID := "200000000000000011"
	listMsgID := "300000000000000011"
	userA := "400000000000000011"
	userB := "400000000000000012"
	coordID := "400000000000000099"

	t.Run("fire emoji reaction sends boost schedule DM for current active booster", func(t *testing.T) {
		hash := "test-fire-emoji-reaction"
		contract := setupBoostReactionTestContract(hash, chanID, listMsgID, ContractPlaystyleFastrun)
		defer func() {
			ContractsMutex.Lock()
			delete(Contracts, hash)
			ContractsMutex.Unlock()
		}()

		client := dctest.New()
		rxEvent := dctest.ReactionEvent(guildID, chanID, listMsgID, coordID, "🔥")
		ReactionAdd(client, rxEvent)

		dmCalls := client.CallsTo("CreateUserChannel")
		if len(dmCalls) == 0 || dmCalls[0].Args[0] != coordID {
			t.Fatalf("expected CreateUserChannel for %s, got %v", coordID, dmCalls)
		}
		if len(client.SentMessages) == 0 {
			t.Fatalf("expected SendMessage to be called for 🔥 DM")
		}
		lastMsg := client.SentMessages[len(client.SentMessages)-1]
		if !strings.Contains(lastMsg.Content, "Boost Schedule for <@"+userA+">") {
			t.Errorf("expected schedule for active booster %s, got:\n%s", userA, lastMsg.Content)
		}
		// Active booster must not be advanced by 🔥
		if contract.Boosters[userA].BoostState != BoostStateTokenTime {
			t.Errorf("expected userA to remain in BoostStateTokenTime after 🔥 reaction")
		}
	})

	t.Run("fastrun active booster rocket button sends ephemeral schedule and updates on coop_status response", func(t *testing.T) {
		hash := "test-fastrun-rocket-update"
		contract := setupBoostReactionTestContract(hash, chanID, listMsgID, ContractPlaystyleFastrun)
		defer func() {
			ContractsMutex.Lock()
			delete(Contracts, hash)
			ContractsMutex.Unlock()
		}()

		origFetch := fetchCoopStatusForDeflectors
		defer func() {
			fetchCoopStatusForDeflectors = origFetch
		}()
		// Disable automatic fetch during initial click so we can verify initial estimated followup then coop_status update
		fetchCoopStatusForDeflectors = func(contractID, coopID, eeidOverride string) (*ei.ContractCoopStatusResponse, error) {
			return nil, nil
		}

		client := dctest.New()
		ev := dctest.ComponentButtonEventFull(guildID, chanID, listMsgID, userA, "rc_#boost#"+hash)
		HandleContractReactions(client, ev)

		if ev.LastFollowup == nil || !ev.LastFollowup.Ephemeral {
			t.Fatalf("expected ephemeral followup for active booster in fastrun contract, got %+v", ev.LastFollowup)
		}
		if !strings.Contains(ev.LastFollowup.Content, "Boost Schedule for <@"+userA+">") {
			t.Errorf("expected Boost Schedule for %s, got:\n%s", userA, ev.LastFollowup.Content)
		}
		if !strings.Contains(ev.LastFollowup.Content, "**Boosts:**") || !strings.Contains(ev.LastFollowup.Content, "Checking coop API in 1m") {
			t.Errorf("expected initial boost prompt with boosts to use and 1m check notice, got:\n%s", ev.LastFollowup.Content)
		}

		// Now simulate a coop_status API response arriving while userA is within their boost time
		okStatus := ei.ContractCoopStatusResponse_NO_ERROR
		deflName := ei.ArtifactSpec_TACHYON_DEFLECTOR
		lvlT4 := ei.ArtifactSpec_GREATER
		rarLeg := ei.ArtifactSpec_LEGENDARY
		rarEpic := ei.ArtifactSpec_EPIC
		pA := "PlayerA"
		pB := "PlayerB"
		pC := "PlayerC"
		pD := "PlayerD"
		boostTachOrange := "tachyon_prism_orange"
		boostBeaconBlue := "boost_beacon_blue"
		boostTachPurple := "tachyon_prism_purple"
		remSec := 540.0
		popA := 2_000_000_000.0
		contract.Boosters["400000000000000014"] = &Booster{
			UserID: "400000000000000014",
			Name:   pD,
			Nick:   pD,
		}
		contract.Order = append(contract.Order, "400000000000000014")

		coopStatus := &ei.ContractCoopStatusResponse{
			ResponseStatus: &okStatus,
			Contributors: []*ei.ContractCoopStatusResponse_ContributionInfo{
				{
					UserName: &pA,
					ProductionParams: &ei.FarmProductionParams{
						FarmPopulation: &popA,
					},
					FarmInfo: &ei.PlayerFarmInfo{
						ActiveBoosts: []*ei.Backup_ActiveBoost{
							{BoostId: &boostTachOrange, TimeRemaining: &remSec},
							{BoostId: &boostBeaconBlue, TimeRemaining: &remSec},
							{BoostId: &boostBeaconBlue, TimeRemaining: &remSec},
							{BoostId: &boostTachPurple, TimeRemaining: &remSec},
							{BoostId: &boostTachPurple, TimeRemaining: &remSec},
						},
					},
				},
				{
					UserName: &pB,
					FarmInfo: &ei.PlayerFarmInfo{
						EquippedArtifacts: []*ei.CompleteArtifact{
							{Spec: &ei.ArtifactSpec{Name: &deflName, Level: &lvlT4, Rarity: &rarLeg}},
						},
					},
				},
				{
					UserName: &pC,
					FarmInfo: &ei.PlayerFarmInfo{
						EquippedArtifacts: []*ei.CompleteArtifact{
							{Spec: &ei.ArtifactSpec{Name: &deflName, Level: &lvlT4, Rarity: &rarEpic}},
						},
					},
				},
				{
					UserName: &pD,
					FarmInfo: &ei.PlayerFarmInfo{
						EquippedArtifacts: []*ei.CompleteArtifact{
							{Spec: &ei.ArtifactSpec{Name: &deflName, Level: &lvlT4, Rarity: &rarEpic}},
						},
					},
				},
			},
		}

		if ei.OnCoopStatusCallback == nil {
			t.Fatalf("expected ei.OnCoopStatusCallback to be registered")
		}
		ei.OnCoopStatusCallback(contract.ContractID, contract.CoopID, coopStatus)

		edits := client.CallsTo("EditFollowupMessage")
		if len(edits) == 0 {
			t.Fatalf("expected EditFollowupMessage to be called when coop_status arrives during boost time")
		}
		editedText := edits[len(edits)-1].Args[3]
		if !strings.Contains(editedText, "`58%` (actual)") {
			t.Errorf("expected edited ephemeral message to show 58%% (actual) deflector bonus, got:\n%s", editedText)
		}
		if !strings.Contains(editedText, "**Current Population:** `2B` (verified)") {
			t.Errorf("expected edited ephemeral message to show verified 2B current population, got:\n%s", editedText)
		}
		if !strings.Contains(editedText, "(verified)") || !strings.Contains(editedText, "b_tach1000x10m") {
			t.Errorf("expected edited ephemeral message to show verified active boosts, got:\n%s", editedText)
		}
	})

	t.Run("coordinator or voter pressing rocket does not receive schedule ephemeral", func(t *testing.T) {
		hash := "test-coord-and-vote-silent"
		_ = setupBoostReactionTestContract(hash, chanID, listMsgID, ContractPlaystyleLeaderboard)
		defer func() {
			ContractsMutex.Lock()
			delete(Contracts, hash)
			ContractsMutex.Unlock()
		}()

		client := dctest.New()
		// Voter userB clicks rocket
		evVote := dctest.ComponentButtonEventFull(guildID, chanID, listMsgID, userB, "rc_#boost#"+hash)
		HandleContractReactions(client, evVote)
		if evVote.LastFollowup != nil && evVote.LastFollowup.Content != "" {
			t.Errorf("expected voter rocket click not to send schedule content, got %q", evVote.LastFollowup.Content)
		}

		// Coordinator clicks rocket to advance active booster
		evCoord := dctest.ComponentButtonEventFull(guildID, chanID, listMsgID, coordID, "rc_#boost#"+hash)
		HandleContractReactions(client, evCoord)
		if evCoord.LastFollowup != nil && evCoord.LastFollowup.Content != "" {
			t.Errorf("expected coordinator rocket click not to send schedule content, got %q", evCoord.LastFollowup.Content)
		}
	})

	t.Run("active booster with DisableBoostSchedule toggled on does not receive schedule ephemeral", func(t *testing.T) {
		hash := "test-disable-boost-sched"
		contract := setupBoostReactionTestContract(hash, chanID, listMsgID, ContractPlaystyleFastrun)
		contract.Boosters[userA].DisableBoostSchedule = true
		defer func() {
			ContractsMutex.Lock()
			delete(Contracts, hash)
			ContractsMutex.Unlock()
		}()

		client := dctest.New()
		evBoost := dctest.ComponentButtonEventFull(guildID, chanID, listMsgID, userA, "rc_#boost#"+hash)
		HandleContractReactions(client, evBoost)
		if evBoost.LastFollowup != nil && evBoost.LastFollowup.Content != "" {
			t.Errorf("expected disabled boost schedule not to send schedule content, got %q", evBoost.LastFollowup.Content)
		}
	})

	t.Run("banker style contract suppresses ephemeral schedule response", func(t *testing.T) {
		hash := "test-banker-style-suppresses-sched"
		contract := setupBoostReactionTestContract(hash, chanID, listMsgID, ContractPlaystyleLeaderboard)
		contract.Style = ContractFlagBanker
		contract.Banker.BoostingSinkUserID = userA
		defer func() {
			ContractsMutex.Lock()
			delete(Contracts, hash)
			ContractsMutex.Unlock()
		}()

		client := dctest.New()
		ev := dctest.ComponentButtonEventFull(guildID, chanID, listMsgID, userA, "rc_#boost#"+hash)
		HandleContractReactions(client, ev)
		if ev.LastFollowup != nil && ev.LastFollowup.Content != "" {
			t.Errorf("expected banker style contract to suppress ephemeral schedule response, got %q", ev.LastFollowup.Content)
		}

		contract.State = ContractStateBanker
		evBag := dctest.ComponentButtonEventFull(guildID, chanID, listMsgID, userA, "rc_#bag#"+hash)
		HandleContractReactions(client, evBag)
		if evBag.LastFollowup != nil && evBag.LastFollowup.Content != "" {
			t.Errorf("expected banker bag click to suppress ephemeral schedule response, got %q", evBag.LastFollowup.Content)
		}
	})
}
