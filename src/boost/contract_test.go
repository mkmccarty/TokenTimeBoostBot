package boost

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

func TestGetEggStandardTime(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("Failed to load America/Los_Angeles location: %v", err)
	}

	tests := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			name:     "Day before Spring Forward (PST)",
			input:    time.Date(2024, 3, 9, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 3, 9, 9, 0, 0, 0, loc),
		},
		{
			name:     "Day of Spring Forward (PDT)",
			input:    time.Date(2024, 3, 10, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 3, 10, 9, 0, 0, 0, loc),
		},
		{
			name:     "Day after Spring Forward (PDT)",
			input:    time.Date(2024, 3, 11, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 3, 11, 9, 0, 0, 0, loc),
		},
		{
			name:     "Day before Fall Back (PDT)",
			input:    time.Date(2024, 11, 2, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 11, 2, 9, 0, 0, 0, loc),
		},
		{
			name:     "Day of Fall Back (PST)",
			input:    time.Date(2024, 11, 3, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 11, 3, 9, 0, 0, 0, loc),
		},
		{
			name:     "Day after Fall Back (PST)",
			input:    time.Date(2024, 11, 4, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 11, 4, 9, 0, 0, 0, loc),
		},
		{
			name:     "Day before Spring Forward (9 AM PST)",
			input:    time.Date(2024, 3, 9, 9, 0, 0, 0, loc),
			expected: time.Date(2024, 3, 9, 9, 0, 0, 0, loc),
		},
		{
			name:     "Day of Spring Forward (9 AM PDT)",
			input:    time.Date(2024, 3, 10, 9, 0, 0, 0, loc),
			expected: time.Date(2024, 3, 10, 9, 0, 0, 0, loc),
		},
		{
			name:     "Day after Spring Forward (9 AM PDT)",
			input:    time.Date(2024, 3, 11, 9, 0, 0, 0, loc),
			expected: time.Date(2024, 3, 11, 9, 0, 0, 0, loc),
		},
		{
			name:     "Day before Fall Back (9 AM PDT)",
			input:    time.Date(2024, 11, 2, 9, 0, 0, 0, loc),
			expected: time.Date(2024, 11, 2, 9, 0, 0, 0, loc),
		},
		{
			name:     "Day of Fall Back (9 AM PST)",
			input:    time.Date(2024, 11, 3, 9, 0, 0, 0, loc),
			expected: time.Date(2024, 11, 3, 9, 0, 0, 0, loc),
		},
		{
			name:     "Day after Fall Back (9 AM PST)",
			input:    time.Date(2024, 11, 4, 9, 0, 0, 0, loc),
			expected: time.Date(2024, 11, 4, 9, 0, 0, 0, loc),
		},
		{
			name:     "Input from different timezone (Asia/Tokyo)",
			input:    time.Date(2024, 6, 15, 23, 0, 0, 0, time.FixedZone("JST", 9*3600)),
			expected: time.Date(2024, 6, 15, 9, 0, 0, 0, loc),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := GetEggStandardTime(tc.input)
			if !got.Equal(tc.expected) {
				t.Errorf("GetEggStandardTime(%v) = %v; want %v", tc.input, got, tc.expected)
			}
			if got.Hour() != 9 {
				t.Errorf("Expected hour to be 9, got %d", got.Hour())
			}
			if got.Location().String() != "America/Los_Angeles" {
				t.Errorf("Expected location to be America/Los_Angeles, got %s", got.Location().String())
			}
		})
	}
}

func TestNextWeekdayDateBeforeEggStandardTimeStaysToday(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("Failed to load America/Los_Angeles location: %v", err)
	}

	tests := []struct {
		name     string
		now      time.Time
		weekday  time.Weekday
		expected time.Time
	}{
		{
			name:     "Monday before 9 AM",
			now:      time.Date(2026, 5, 11, 7, 0, 0, 0, loc),
			weekday:  time.Monday,
			expected: time.Date(2026, 5, 11, 9, 0, 0, 0, loc),
		},
		{
			name:     "Wednesday before 9 AM",
			now:      time.Date(2026, 5, 13, 7, 0, 0, 0, loc),
			weekday:  time.Wednesday,
			expected: time.Date(2026, 5, 13, 9, 0, 0, 0, loc),
		},
		{
			name:     "Friday before 9 AM",
			now:      time.Date(2026, 5, 15, 7, 0, 0, 0, loc),
			weekday:  time.Friday,
			expected: time.Date(2026, 5, 15, 9, 0, 0, 0, loc),
		},
		{
			name:     "Wednesday after 9 AM rolls to next week",
			now:      time.Date(2026, 5, 13, 10, 0, 0, 0, loc),
			weekday:  time.Wednesday,
			expected: time.Date(2026, 5, 20, 9, 0, 0, 0, loc),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := nextWeekdayDate(tc.now, tc.weekday)
			if !got.Equal(tc.expected) {
				t.Fatalf("nextWeekdayDate(%v, %v) = %v; want %v", tc.now, tc.weekday, got, tc.expected)
			}
		})
	}
}

func TestChangePlannedStartOffsetCalculation(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("Failed to load America/Los_Angeles location: %v", err)
	}

	// Simulated current time: Sept 5, 2026 at 8:10 AM PT
	now := time.Date(2026, 9, 5, 8, 10, 0, 0, loc)
	baseTime := GetEggStandardTime(now) // Sept 5, 2026 at 9:00 AM PT

	tests := []struct {
		name     string
		offset   float64
		expected time.Time
	}{
		{
			name:     "Offset +24 from 8:10 AM should be tomorrow Sept 6 at 9:00 AM PT",
			offset:   24,
			expected: time.Date(2026, 9, 6, 9, 0, 0, 0, loc),
		},
		{
			name:     "Offset +48 from 8:10 AM should be Sept 7 at 9:00 AM PT",
			offset:   48,
			expected: time.Date(2026, 9, 7, 9, 0, 0, 0, loc),
		},
		{
			name:     "Offset +2.5 from 8:10 AM should be today Sept 5 at 11:30 AM PT",
			offset:   2.5,
			expected: time.Date(2026, 9, 5, 11, 30, 0, 0, loc),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			offsetDuration := time.Duration(tc.offset * float64(time.Hour))
			resultTime := baseTime.Add(offsetDuration)
			for resultTime.Before(now) {
				resultTime = resultTime.Add(24 * time.Hour)
			}
			if !resultTime.Equal(tc.expected) {
				t.Fatalf("resultTime = %v, want %v", resultTime, tc.expected)
			}
		})
	}
}

func TestPopulateThematicComplaintsForContractID(t *testing.T) {
	// Create a dummy contract and put it in Contracts map
	cID := "test-contract-123"
	cHash := "test-hash-456"
	contract := &Contract{
		ContractID:   cID,
		ContractHash: cHash,
	}

	mutex.Lock()
	Contracts[cHash] = contract
	mutex.Unlock()

	defer func() {
		mutex.Lock()
		delete(Contracts, cHash)
		mutex.Unlock()
	}()

	complaints := []string{"complaint 1", "complaint 2"}
	PopulateThematicComplaintsForContractID(cID, complaints)

	mutex.Lock()
	tc := contract.ThematicComplaints
	mutex.Unlock()

	if len(tc) != 2 {
		t.Fatalf("expected 2 complaints, got %d", len(tc))
	}
	found1 := false
	found2 := false
	for _, c := range tc {
		if c == "complaint 1" {
			found1 = true
		}
		if c == "complaint 2" {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Errorf("complaints not populated correctly: %v", tc)
	}
}

func TestThresholdTokensCalculation(t *testing.T) {
	contract := &Contract{
		Style:            ContractFlagThresholdTokens,
		ThresholdTokensX: 4,
		ThresholdTokensY: 5,
		ThresholdTokensA: 70,
		Boosters:         make(map[string]*Booster),
	}

	b1 := &Booster{UserID: "user1", TECount: 75}
	b2 := &Booster{UserID: "user2", TECount: 65}
	b3 := &Booster{UserID: "user3", TECount: 70}

	contract.Boosters["user1"] = b1
	contract.Boosters["user2"] = b2
	contract.Boosters["user3"] = b3

	// Simulate contract start logic for setting tokens
	for i := range contract.Boosters {
		if contract.Style&ContractFlagThresholdTokens != 0 {
			x := contract.ThresholdTokensX
			y := contract.ThresholdTokensY
			a := contract.ThresholdTokensA
			if contract.Boosters[i].TECount >= a {
				contract.Boosters[i].TokensWanted = x
			} else {
				contract.Boosters[i].TokensWanted = y
			}
		}
	}

	if b1.TokensWanted != 4 {
		t.Errorf("expected user1 to want 4 tokens (TE 75 >= 70), got %d", b1.TokensWanted)
	}
	if b2.TokensWanted != 5 {
		t.Errorf("expected user2 to want 5 tokens (TE 65 < 70), got %d", b2.TokensWanted)
	}
	if b3.TokensWanted != 4 {
		t.Errorf("expected user3 to want 4 tokens (TE 70 >= 70), got %d", b3.TokensWanted)
	}
}

func TestCreateContractPlaystyleBoostOrder(t *testing.T) {
	client := newTestClient()

	contractID := "playstyle-boost-order-test"
	guildID := "guild-123"
	creatorUserID := "user-456"

	// Case 1: boostOrder is -1 (not specified) and PlayStyle is Leaderboard.
	// It should default to ContractOrderTEFuzzy.
	channelID1 := "channel-123-1"
	contract1, err := CreateContract(client, contractID, "coop-order-test-1", ContractPlaystyleLeaderboard, 10, -1, guildID, channelID1, []string{creatorUserID}, creatorUserID, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("Failed to create contract: %v", err)
	}
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract1.ContractHash)
		ContractsMutex.Unlock()
	}()
	if contract1.BoostOrder != ContractOrderTEFuzzy {
		t.Errorf("expected default boost order for Leaderboard playstyle to be ContractOrderTEFuzzy (%d), got %d", ContractOrderTEFuzzy, contract1.BoostOrder)
	}

	// Case 2: boostOrder is explicitly ContractOrderRandom (2) and PlayStyle is Leaderboard.
	// It should override the default playstyle boost order and remain ContractOrderRandom.
	channelID2 := "channel-123-2"
	contract2, err := CreateContract(client, contractID, "coop-order-test-2", ContractPlaystyleLeaderboard, 10, ContractOrderRandom, guildID, channelID2, []string{creatorUserID}, creatorUserID, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("Failed to create contract: %v", err)
	}
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract2.ContractHash)
		ContractsMutex.Unlock()
	}()
	if contract2.BoostOrder != ContractOrderRandom {
		t.Errorf("expected explicit boost order ContractOrderRandom (%d) to override Leaderboard playstyle, got %d", ContractOrderRandom, contract2.BoostOrder)
	}

	// Case 3: boostOrder is -1 (not specified) and PlayStyle is Chill.
	// It should default to ContractOrderSignup (0).
	channelID3 := "channel-123-3"
	contract3, err := CreateContract(client, contractID, "coop-order-test-3", ContractPlaystyleChill, 10, -1, guildID, channelID3, []string{creatorUserID}, creatorUserID, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("Failed to create contract: %v", err)
	}
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract3.ContractHash)
		ContractsMutex.Unlock()
	}()
	if contract3.BoostOrder != ContractOrderSignup {
		t.Errorf("expected default boost order for Chill playstyle to be ContractOrderSignup (%d), got %d", ContractOrderSignup, contract3.BoostOrder)
	}
}

func TestMultipleTBDContracts(t *testing.T) {
	client := newTestClient()
	contractID := "tbd-test-contract"
	guildID := "guild-123"
	creatorUserID := "user-456"

	// Create first TBD contract in channel-1
	channelID1 := "channel-1"
	contract1, err := CreateContract(client, contractID, "tbd", ContractPlaystyleChill, 10, -1, guildID, channelID1, []string{creatorUserID}, creatorUserID, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("Failed to create first TBD contract: %v", err)
	}
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract1.ContractHash)
		ContractsMutex.Unlock()
	}()

	// Create second TBD contract in channel-2 (with a variation "tbd+3")
	channelID2 := "channel-2"
	contract2, err := CreateContract(client, contractID, "tbd+3", ContractPlaystyleChill, 10, -1, guildID, channelID2, []string{creatorUserID}, creatorUserID, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("Failed to create second TBD contract (tbd+3): %v", err)
	}
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract2.ContractHash)
		ContractsMutex.Unlock()
	}()

	// Create third TBD contract in channel-3 (with same coopID "tbd" to test duplicate bypass)
	channelID3 := "channel-3"
	contract3, err := CreateContract(client, contractID, "tbd", ContractPlaystyleChill, 10, -1, guildID, channelID3, []string{creatorUserID}, creatorUserID, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("Failed to create third TBD contract (duplicate tbd): %v", err)
	}
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract3.ContractHash)
		ContractsMutex.Unlock()
	}()

	// Verify lookups by coopID and channelID
	found1 := FindContractByIDs(channelID1, contractID, "tbd")
	if found1 == nil || found1.ContractHash != contract1.ContractHash {
		t.Errorf("Lookup for channel-1 / tbd returned %v, expected hash %s", found1, contract1.ContractHash)
	}

	found2 := FindContractByIDs(channelID2, contractID, "tbd+3")
	if found2 == nil || found2.ContractHash != contract2.ContractHash {
		t.Errorf("Lookup for channel-2 / tbd+3 returned %v, expected hash %s", found2, contract2.ContractHash)
	}

	found3 := FindContractByIDs(channelID3, contractID, "tbd")
	if found3 == nil || found3.ContractHash != contract3.ContractHash {
		t.Errorf("Lookup for channel-3 / tbd returned %v, expected hash %s", found3, contract3.ContractHash)
	}

	// Lookup without channel context should fallback to returning one of them
	foundFallback := FindContractByIDs("", contractID, "tbd")
	if foundFallback == nil {
		t.Errorf("Lookup without channel context returned nil")
	}

	// Verify DrawBoostList adds guidance text
	boostComponents := DrawBoostList(contract1)
	foundWarning := false
	for _, comp := range boostComponents {
		if td, ok := comp.(dc.TextDisplay); ok {
			if strings.Contains(td.Content, "Coop ID is set to TBD") {
				foundWarning = true
				break
			}
		}
	}
	if !foundWarning {
		t.Errorf("expected DrawBoostList output to contain TBD warning")
	}

	// Verify GetSignupComponents disables start button
	str, components := GetSignupComponents(contract1)
	if strings.Contains(str, "Coop ID is set to TBD") {
		t.Errorf("expected join dialog guidance text NOT to contain TBD warning, got: %s", str)
	}

	foundStartBtn := false
	for _, row := range components {
		if actionRow, ok := row.(dc.ActionRow); ok {
			for _, comp := range actionRow.Components {
				if btn, ok := comp.(dc.Button); ok && btn.CustomID == "fd_signupStart" {
					foundStartBtn = true
					if !btn.Disabled {
						t.Errorf("expected start button to be disabled for TBD contract")
					}
				}
			}
		}
	}
	if !foundStartBtn {
		t.Errorf("start button not found in signup components")
	}
}

func TestRenderContractReportImage(t *testing.T) {
	p := contractReportParameters{
		contractID:  "test-contract",
		coopID:      "test-coop",
		thresholds:  thresholds{buffTimeValue: 100, chickenRuns: 10, teamwork: 25},
		metricPeaks: metricPeaks{cxp: 1000, teamwork: 30, contributionRatio: 5.0, buffTimeValue: 150},
		playerEvalsMetrics: []evalMetrics{
			{
				player:            "VeryLongPlayerNameForTesting",
				cxp:               1000,
				contributionRatio: 5.0,
				teamwork:          30,
				chickenRunsSent:   10,
				buffTimeValue:     150,
			},
			{
				player:            "ShortName",
				cxp:               800,
				contributionRatio: 3.5,
				teamwork:          20,
				chickenRunsSent:   5,
				buffTimeValue:     90,
			},
			{
				player:            "EmojiFarmer🔥Δ",
				cxp:               950,
				contributionRatio: 4.2,
				teamwork:          28,
				chickenRunsSent:   8,
				buffTimeValue:     120,
			},
			{
				player:            "\ue40dPUAFarmer",
				cxp:               910,
				contributionRatio: 4.0,
				teamwork:          26,
				chickenRunsSent:   7,
				buffTimeValue:     110,
			},
			{
				player:            "👽Chipmunk",
				cxp:               77654,
				contributionRatio: 0.949,
				teamwork:          0.658,
				chickenRunsSent:   4,
				buffTimeValue:     55794,
			},
		},
	}

	t.Run("Default", func(t *testing.T) {
		imgBytes, err := renderContractReportImage(&p, false)
		if err != nil {
			t.Fatalf("renderContractReportImage failed: %v", err)
		}
		if len(imgBytes) == 0 {
			t.Fatal("expected non-empty PNG image bytes")
		}
	})

	t.Run("Show token details", func(t *testing.T) {
		imgBytes, err := renderContractReportImage(&p, true)
		if err != nil {
			t.Fatalf("renderContractReportImage failed: %v", err)
		}
		if len(imgBytes) == 0 {
			t.Fatal("expected non-empty PNG image bytes")
		}
	})
}

func TestProgenitorsCreatorNotProgenitor(t *testing.T) {
	client := newTestClient()

	contractID := "progenitor-test-contract"
	guildID := "guild-123"
	creatorUserID := "coordinator-user"
	progenitorID1 := "progenitor-1"
	progenitorID2 := "progenitor-2"
	progenitors := []string{progenitorID1, progenitorID2}

	contract, err := CreateContract(client, contractID, "coop-prog-test", ContractPlaystyleChill, 10, -1, guildID, "channel-prog-1", progenitors, creatorUserID, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("Failed to create contract: %v", err)
	}
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract.ContractHash)
		ContractsMutex.Unlock()
	}()

	// Creator (who is not in progenitors) should be in CreatorID (coordinator list)
	if !slices.Contains(contract.CreatorID, creatorUserID) {
		t.Errorf("Expected creatorUserID %s to be in contract.CreatorID, got %v", creatorUserID, contract.CreatorID)
	}
	// The first progenitor should also be in CreatorID
	if !slices.Contains(contract.CreatorID, progenitorID1) {
		t.Errorf("Expected first progenitor %s to be in contract.CreatorID, got %v", progenitorID1, contract.CreatorID)
	}
	// Creator should NOT be in progenitors/order list
	if slices.Contains(contract.Order, creatorUserID) {
		t.Errorf("Expected creatorUserID %s NOT to be in contract.Order, got %v", creatorUserID, contract.Order)
	}
}

func TestRestartContractRestoresState(t *testing.T) {
	client := newTestClient()

	contractID := "restart-state-contract"
	guildID := "guild-123"
	channelID := "channel-restart-1"
	creatorUserID := "coordinator-user"
	progenitors := []string{"farmer-1", "farmer-2", "farmer-3"}

	contract, err := CreateContract(client, contractID, "coop-restart-test", ContractPlaystyleChill, 10, ContractOrderFair, guildID, channelID, progenitors, creatorUserID, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("Failed to create contract: %v", err)
	}

	// Customize style flags, threshold tokens, and planned start time
	contract.Style |= ContractFlag4Tokens | ContractFlagThresholdTokens
	contract.ThresholdTokensX = 6
	contract.ThresholdTokensY = 8
	contract.ThresholdTokensA = 80
	contract.BoostOrder = ContractOrderFair
	contract.PlannedStartTime = time.Now().Add(2 * time.Hour)

	// Simulate contract starting state
	contract.State = ContractStateFastrun
	contract.OriginalOrder = append([]string{}, progenitors...)

	// Capture state as done in HandleRestartContract
	savedStyle := contract.Style
	savedProgenitors := contract.Order
	if contract.State != ContractStateSignup && len(contract.OriginalOrder) > 0 {
		savedProgenitors = contract.OriginalOrder
	}
	savedThresholdX := contract.ThresholdTokensX
	savedThresholdY := contract.ThresholdTokensY
	savedThresholdA := contract.ThresholdTokensA
	origCreatorID := contract.CreatorID[0]

	_, err = DeleteContract(client, guildID, channelID)
	if err != nil {
		t.Fatalf("Failed to delete contract: %v", err)
	}

	newContract, err := CreateContract(client, contractID, "coop-restart-test", ContractPlaystyleChill, 10, ContractOrderSignup, guildID, channelID, savedProgenitors, origCreatorID, time.Time{}, time.Now())
	if err != nil {
		t.Fatalf("Failed to recreate contract on restart: %v", err)
	}
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, newContract.ContractHash)
		ContractsMutex.Unlock()
	}()

	newContract.Style = savedStyle
	newContract.BoostOrder = ContractOrderSignup
	newContract.ThresholdTokensX = savedThresholdX
	newContract.ThresholdTokensY = savedThresholdY
	newContract.ThresholdTokensA = savedThresholdA
	reorderBoosters(newContract)

	if newContract.BoostOrder != ContractOrderSignup {
		t.Errorf("Expected BoostOrder to be %d, got %d", ContractOrderSignup, newContract.BoostOrder)
	}
	if !newContract.PlannedStartTime.IsZero() {
		t.Errorf("Expected PlannedStartTime to be zero, got %v", newContract.PlannedStartTime)
	}
	if slices.Compare(newContract.Order, savedProgenitors) != 0 {
		t.Errorf("Expected Order to be preserved as %v, got %v", savedProgenitors, newContract.Order)
	}
	if newContract.Style != savedStyle {
		t.Errorf("Expected Style to be %d, got %d", savedStyle, newContract.Style)
	}
	if newContract.ThresholdTokensX != 6 || newContract.ThresholdTokensY != 8 || newContract.ThresholdTokensA != 80 {
		t.Errorf("Expected threshold tokens (6, 8, 80), got (%d, %d, %d)", newContract.ThresholdTokensX, newContract.ThresholdTokensY, newContract.ThresholdTokensA)
	}
}

func TestHandleContractSettingsReactionsFeatures(t *testing.T) {
	client := newTestClient()
	contractID := "features-contract"
	guildID := "guild-123"
	channelID := "channel-restart-1"
	creatorUserID := "coordinator-user"
	progenitors := []string{"farmer-1"}

	contract, err := CreateContract(client, contractID, "coop-features-test", ContractPlaystyleChill, 10, ContractOrderSignup, guildID, channelID, progenitors, creatorUserID, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("Failed to create contract: %v", err)
	}
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract.ContractHash)
		ContractsMutex.Unlock()
	}()

	customID := "cs_#features#" + contract.ContractHash

	// 1. Select both boost6 and amqp concurrently
	ev1 := dctest.ComponentSelectEvent(customID, "boost6", "amqp")
	HandleContractSettingsReactions(client, ev1)

	if contract.Style&ContractFlag6Tokens == 0 {
		t.Errorf("Expected ContractFlag6Tokens to be set, style: %x", contract.Style)
	}
	if contract.Style&ContractFlagAMQP == 0 {
		t.Errorf("Expected ContractFlagAMQP to be set, style: %x", contract.Style)
	}

	// 2. Select boost8 and amqp concurrently; boost6 should be replaced by boost8
	ev2 := dctest.ComponentSelectEvent(customID, "boost8", "amqp")
	HandleContractSettingsReactions(client, ev2)

	if contract.Style&ContractFlag8Tokens == 0 {
		t.Errorf("Expected ContractFlag8Tokens to be set, style: %x", contract.Style)
	}
	if contract.Style&ContractFlag6Tokens != 0 {
		t.Errorf("Expected ContractFlag6Tokens to be cleared, style: %x", contract.Style)
	}
	if contract.Style&ContractFlagAMQP == 0 {
		t.Errorf("Expected ContractFlagAMQP to remain set, style: %x", contract.Style)
	}

	// 3. Select only dynamic; AMQP should be cleared and dynamic set
	ev3 := dctest.ComponentSelectEvent(customID, "dynamic")
	HandleContractSettingsReactions(client, ev3)

	if contract.Style&ContractFlagDynamicTokens == 0 {
		t.Errorf("Expected ContractFlagDynamicTokens to be set, style: %x", contract.Style)
	}
	if contract.Style&ContractFlag8Tokens != 0 {
		t.Errorf("Expected ContractFlag8Tokens to be cleared, style: %x", contract.Style)
	}
	if contract.Style&ContractFlagAMQP != 0 {
		t.Errorf("Expected ContractFlagAMQP to be cleared, style: %x", contract.Style)
	}

	// 4. Threshold + AMQP: should set AMQP and trigger threshold modal flow
	ev4 := dctest.ComponentSelectEvent(customID, "threshold", "amqp")
	HandleContractSettingsReactions(client, ev4)

	if contract.Style&ContractFlagAMQP == 0 {
		t.Errorf("Expected ContractFlagAMQP to be set when threshold is selected with amqp, style: %x", contract.Style)
	}

	// 5. Multiple boost token options selected at once (e.g. boost4, boost6, boost8, dynamic):
	// Only the topmost one (boost4) should be accepted, and the others cleared.
	ev5 := dctest.ComponentSelectEvent(customID, "boost8", "boost4", "dynamic", "boost6")
	HandleContractSettingsReactions(client, ev5)

	if contract.Style&ContractFlag4Tokens == 0 {
		t.Errorf("Expected topmost option ContractFlag4Tokens to be set, style: %x", contract.Style)
	}
	if contract.Style&ContractFlag6Tokens != 0 {
		t.Errorf("Expected ContractFlag6Tokens to be cleared, style: %x", contract.Style)
	}
	if contract.Style&ContractFlag8Tokens != 0 {
		t.Errorf("Expected ContractFlag8Tokens to be cleared, style: %x", contract.Style)
	}
	if contract.Style&ContractFlagDynamicTokens != 0 {
		t.Errorf("Expected ContractFlagDynamicTokens to be cleared, style: %x", contract.Style)
	}

	// 6. Multiple options including a higher token option (boost6) and threshold:
	// boost6 is higher than threshold, so boost6 should win, threshold modal not triggered, and others cleared.
	ev6 := dctest.ComponentSelectEvent(customID, "threshold", "boost6", "amqp")
	HandleContractSettingsReactions(client, ev6)

	if contract.Style&ContractFlag6Tokens == 0 {
		t.Errorf("Expected topmost option ContractFlag6Tokens to be set, style: %x", contract.Style)
	}
	if contract.Style&ContractFlag4Tokens != 0 {
		t.Errorf("Expected ContractFlag4Tokens to be cleared, style: %x", contract.Style)
	}
	if contract.Style&ContractFlagThresholdTokens != 0 {
		t.Errorf("Expected ContractFlagThresholdTokens to be cleared, style: %x", contract.Style)
	}
	if contract.Style&ContractFlagAMQP == 0 {
		t.Errorf("Expected ContractFlagAMQP to remain set, style: %x", contract.Style)
	}
}

func TestBoostMenuNextBoosterTokens(t *testing.T) {
	// Restore GG event state when done
	origGG, origUGG, origEnd := ei.GetGenerousGiftEvent()
	defer ei.SetGenerousGiftEvent(origGG, origUGG, origEnd)

	contract := &Contract{
		ContractHash: "test-next-tokens",
		State:        ContractStateFastrun,
		Location: []*LocationData{
			{GuildContractRole: GuildRole{Name: "TestRole"}},
		},
		Order: []string{"user1", "user2", "user3"},
		Boosters: map[string]*Booster{
			"user1": {UserID: "user1", Nick: "Alice", TokensWanted: 6, TokensReceived: 6},
			"user2": {UserID: "user2", Nick: "Bob", TokensWanted: 6, TokensReceived: 0},
			"user3": {UserID: "user3", Nick: "Charlie", TokensWanted: 6, TokensReceived: 0},
		},
	}
	contract.setCurrentBoosterByUserID("user1")

	findNextOptions := func(comps []dc.LayoutComponent) (hasNext1 bool, hasNext2 bool, next2EmojiName string) {
		for _, row := range comps {
			if ar, ok := row.(dc.ActionRow); ok {
				for _, c := range ar.Components {
					if sm, ok := c.(dc.SelectMenu); ok && strings.HasPrefix(sm.CustomID, "menu#") {
						for _, opt := range sm.Options {
							if opt.Value == "next:user2" {
								hasNext1 = true
							}
							if opt.Value == "next2:user2" {
								hasNext2 = true
								if opt.Emoji != nil {
									next2EmojiName = opt.Emoji.Name
								}
							}
						}
					}
				}
			}
		}
		return
	}

	// 1. No GG Event (gg=1.0, ugg=1.0) -> only next:user2 should be present
	ei.SetGenerousGiftEvent(1.0, 1.0, time.Time{})
	comps := getContractReactionsComponents(contract)
	h1, h2, _ := findNextOptions(comps)
	if !h1 {
		t.Errorf("Expected next:user2 option to be present when current booster has needed tokens")
	}
	if h2 {
		t.Errorf("Did not expect next2:user2 option to be present when no GG event is active")
	}

	// 2. Standard GG Event (gg=2.0, ugg=1.0) -> next2:user2 should be present with std_gg emoji
	ei.SetGenerousGiftEvent(2.0, 1.0, time.Time{})
	comps = getContractReactionsComponents(contract)
	h1, h2, emoji := findNextOptions(comps)
	if !h1 || !h2 {
		t.Errorf("Expected both next:user2 and next2:user2 options during GG event, got h1=%v, h2=%v", h1, h2)
	}
	stdGGEmoji := ei.GetBotComponentEmoji("std_gg")
	if emoji != stdGGEmoji.Name {
		t.Errorf("Expected next2:user2 emoji to match std_gg name %q, got %q", stdGGEmoji.Name, emoji)
	}

	// 3. Ultra GG Event (gg=1.0, ugg=3.0) -> next2:user2 should be present with ultra_gg emoji
	ei.SetGenerousGiftEvent(1.0, 3.0, time.Time{})
	comps = getContractReactionsComponents(contract)
	h1, h2, emoji = findNextOptions(comps)
	if !h1 || !h2 {
		t.Errorf("Expected both next:user2 and next2:user2 options during Ultra GG event, got h1=%v, h2=%v", h1, h2)
	}
	ultraGGEmoji := ei.GetBotComponentEmoji("ultra_gg")
	if emoji != ultraGGEmoji.Name {
		t.Errorf("Expected next2:user2 emoji to match ultra_gg name %q, got %q", ultraGGEmoji.Name, emoji)
	}
}

func TestAddFarmerToContract_AddsThreadMember(t *testing.T) {
	client := dctest.New().
		WithGuild("guild1", "Guild 1").
		WithChannel("thread1", "guild1", "contract-thread").
		WithUser("123456789012345678", "farmer1", "Farmer One")

	contract := &Contract{
		ContractHash: "test-hash-thread",
		ContractID:   "test-contract",
		CoopID:       "test-coop",
		CoopSize:     10,
		State:        ContractStateFastrun,
		CreatorID:    []string{"creator1"},
		Order:        make([]string, 0),
		Boosters:     make(map[string]*Booster),
		Location:     []*LocationData{{GuildID: "guild1", ChannelID: "thread1"}},
	}
	Contracts[contract.ContractHash] = contract
	defer delete(Contracts, contract.ContractHash)

	b, err := AddFarmerToContract(client, contract, "guild1", "thread1", "123456789012345678", ContractOrderSignup, false, false)
	if err != nil {
		t.Fatalf("unexpected error adding farmer: %v", err)
	}
	if b == nil {
		t.Fatalf("expected booster to be created, got nil")
	}

	// Verify AddThreadMember call was recorded for snowflake user
	found := false
	for _, call := range client.Calls {
		if call.Method == "AddThreadMember" {
			if len(call.Args) >= 2 && call.Args[0] == "thread1" && call.Args[1] == "123456789012345678" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Errorf("expected AddThreadMember(thread1, 123456789012345678) to be called, recorded calls: %v", client.Calls)
	}

	// Adding a non-snowflake guest should NOT call AddThreadMember
	client.Calls = nil
	_, err = AddFarmerToContract(client, contract, "guild1", "thread1", "guest-farmer", ContractOrderSignup, false, false)
	if err != nil {
		t.Fatalf("unexpected error adding guest farmer: %v", err)
	}
	for _, call := range client.Calls {
		if call.Method == "AddThreadMember" {
			t.Errorf("unexpected AddThreadMember call for guest: %v", call)
		}
	}
}

func TestJoinRunningContract_AddsThreadMember(t *testing.T) {
	client := dctest.New().
		WithGuild("guild1", "Guild 1").
		WithChannel("thread2", "guild1", "running-contract-thread").
		WithUser("234567890123456789", "farmer2", "Farmer Two")

	contract := &Contract{
		ContractHash: "test-hash-running-join",
		ContractID:   "test-contract-running",
		CoopID:       "test-coop-running",
		CoopSize:     5,
		State:        ContractStateFastrun,
		CreatorID:    []string{"creator1"},
		Order:        []string{"creator1"},
		Boosters: map[string]*Booster{
			"creator1": {UserID: "creator1", Name: "Creator", Nick: "Creator"},
		},
		Location: []*LocationData{{GuildID: "guild1", ChannelID: "thread2"}},
	}
	Contracts[contract.ContractHash] = contract
	defer delete(Contracts, contract.ContractHash)

	err := JoinContract(client, "guild1", "thread2", "234567890123456789", false)
	if err != nil {
		t.Fatalf("unexpected error joining contract: %v", err)
	}

	// Verify AddThreadMember call was recorded
	found := false
	for _, call := range client.Calls {
		if call.Method == "AddThreadMember" {
			if len(call.Args) >= 2 && call.Args[0] == "thread2" && call.Args[1] == "234567890123456789" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Errorf("expected AddThreadMember(thread2, 234567890123456789) to be called on join, recorded calls: %v", client.Calls)
	}
}
