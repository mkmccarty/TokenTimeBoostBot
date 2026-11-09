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

	if sched4.TruckChecks < 1 {
		t.Errorf("expected at least 1 truck check for 4-token boost, got %d (pureCRDuration=%v)", sched4.TruckChecks, sched4.PureCRDuration)
	}
	expectedTruckWait := time.Duration(sched4.TruckChecks) * 30 * time.Second
	if sched4.TruckWaitDuration != expectedTruckWait {
		t.Errorf("expected TruckWaitDuration=%v, got %v", expectedTruckWait, sched4.TruckWaitDuration)
	}
	if sched4.CRDuration != sched4.PureCRDuration+sched4.TruckWaitDuration {
		t.Errorf("expected CRDuration=%v, got %v", sched4.PureCRDuration+sched4.TruckWaitDuration, sched4.CRDuration)
	}
	if sched4.PostCROfflineDuration < time.Minute {
		t.Errorf("expected PostCROfflineDuration >= 1m, got %v", sched4.PostCROfflineDuration)
	}
	if sched4.TotalBoostDuration != sched4.CRDuration+sched4.PostCROfflineDuration {
		t.Errorf("expected TotalBoostDuration=%v, got %v", sched4.CRDuration+sched4.PostCROfflineDuration, sched4.TotalBoostDuration)
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
		"b_tach1000x10m",
		"b_boost10x10m",
		"Reach CR Population",
		"Offline to Max Shipping",
	} {
		if !strings.Contains(formatted, expected) {
			t.Errorf("formatBoostSchedule output missing %q:\n%s", expected, formatted)
		}
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

func TestBoostSchedule_SilentRocketAndHiddenFireReaction(t *testing.T) {
	guildID := "100000000000000001"
	chanID := "200000000000000011"
	listMsgID := "300000000000000011"
	userA := "400000000000000011"
	userB := "400000000000000012"
	coordID := "400000000000000099"

	t.Run("hidden fire emoji reaction sends boost schedule for current active booster", func(t *testing.T) {
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

		if len(client.SentMessages) == 0 {
			t.Fatalf("expected SendMessage to be called for 🔥 reaction")
		}
		lastMsg := client.SentMessages[len(client.SentMessages)-1]
		if !lastMsg.Ephemeral {
			t.Errorf("expected 🔥 reaction message to have Ephemeral: true")
		}
		if !strings.Contains(lastMsg.Content, "Boost Schedule for <@"+userA+">") {
			t.Errorf("expected schedule for active booster %s, got:\n%s", userA, lastMsg.Content)
		}
		// Active booster must not be advanced by 🔥
		if contract.Boosters[userA].BoostState != BoostStateTokenTime {
			t.Errorf("expected userA to remain in BoostStateTokenTime after 🔥 reaction")
		}
	})

	t.Run("fire component reaction sends ephemeral followup for current active booster", func(t *testing.T) {
		hash := "test-fire-component-reaction"
		contract := setupBoostReactionTestContract(hash, chanID, listMsgID, ContractPlaystyleLeaderboard)
		defer func() {
			ContractsMutex.Lock()
			delete(Contracts, hash)
			ContractsMutex.Unlock()
		}()

		client := dctest.New()
		ev := dctest.ComponentButtonEventFull(guildID, chanID, listMsgID, userB, "rc_#fire#"+hash)
		HandleContractReactions(client, ev)

		if ev.LastFollowup == nil {
			t.Fatalf("expected ephemeral followup for fire reaction, got nil")
		}
		if !ev.LastFollowup.Ephemeral {
			t.Errorf("expected followup to be ephemeral")
		}
		if !strings.Contains(ev.LastFollowup.Content, "Boost Schedule for <@"+userA+">") {
			t.Errorf("expected Boost Schedule for active booster in followup, got:\n%s", ev.LastFollowup.Content)
		}
		if contract.Boosters[userA].BoostState != BoostStateTokenTime {
			t.Errorf("expected userA to remain in BoostStateTokenTime")
		}
	})

	t.Run("rocket reaction works silently as before while updating estimated timings", func(t *testing.T) {
		hash := "test-fastrun-silent-rocket"
		contract := setupBoostReactionTestContract(hash, chanID, listMsgID, ContractPlaystyleFastrun)
		defer func() {
			ContractsMutex.Lock()
			delete(Contracts, hash)
			ContractsMutex.Unlock()
		}()

		client := dctest.New()
		ev := dctest.ComponentButtonEventFull(guildID, chanID, listMsgID, userA, "rc_#boost#"+hash)
		HandleContractReactions(client, ev)

		if ev.LastFollowup != nil && ev.LastFollowup.Content != "" {
			t.Errorf("expected rocket button not to send schedule content, got %q", ev.LastFollowup.Content)
		}
		if contract.Boosters[userA].BoostState != BoostStateBoosted {
			t.Errorf("expected userA to advance to BoostStateBoosted, got %v", contract.Boosters[userA].BoostState)
		}
		if contract.Boosters[userA].EstDurationOfBoost <= 0 {
			t.Errorf("expected EstDurationOfBoost to be set silently, got %v", contract.Boosters[userA].EstDurationOfBoost)
		}
	})
}
