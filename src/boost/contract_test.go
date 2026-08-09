package boost

import (
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
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
	s, err := createMockSession()
	if err != nil {
		t.Fatalf("Failed to create mock session: %v", err)
	}

	contractID := "playstyle-boost-order-test"
	guildID := "guild-123"
	creatorUserID := "user-456"

	// Case 1: boostOrder is -1 (not specified) and PlayStyle is Leaderboard.
	// It should default to ContractOrderTEFuzzy.
	channelID1 := "channel-123-1"
	contract1, err := CreateContract(s, contractID, "coop-order-test-1", ContractPlaystyleLeaderboard, 10, -1, guildID, channelID1, []string{creatorUserID}, creatorUserID, time.Now(), time.Now())
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
	contract2, err := CreateContract(s, contractID, "coop-order-test-2", ContractPlaystyleLeaderboard, 10, ContractOrderRandom, guildID, channelID2, []string{creatorUserID}, creatorUserID, time.Now(), time.Now())
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
	contract3, err := CreateContract(s, contractID, "coop-order-test-3", ContractPlaystyleChill, 10, -1, guildID, channelID3, []string{creatorUserID}, creatorUserID, time.Now(), time.Now())
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
	s, err := createMockSession()
	if err != nil {
		t.Fatalf("Failed to create mock session: %v", err)
	}
	contractID := "tbd-test-contract"
	guildID := "guild-123"
	creatorUserID := "user-456"

	// Create first TBD contract in channel-1
	channelID1 := "channel-1"
	contract1, err := CreateContract(s, contractID, "tbd", ContractPlaystyleChill, 10, -1, guildID, channelID1, []string{creatorUserID}, creatorUserID, time.Now(), time.Now())
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
	contract2, err := CreateContract(s, contractID, "tbd+3", ContractPlaystyleChill, 10, -1, guildID, channelID2, []string{creatorUserID}, creatorUserID, time.Now(), time.Now())
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
	contract3, err := CreateContract(s, contractID, "tbd", ContractPlaystyleChill, 10, -1, guildID, channelID3, []string{creatorUserID}, creatorUserID, time.Now(), time.Now())
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
	boostComponents := DrawBoostList(s, contract1)
	foundWarning := false
	for _, comp := range boostComponents {
		if td, ok := comp.(*discordgo.TextDisplay); ok {
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
		if actionRow, ok := row.(discordgo.ActionsRow); ok {
			for _, comp := range actionRow.Components {
				if btn, ok := comp.(discordgo.Button); ok && btn.CustomID == "fd_signupStart" {
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

func TestMakeShortNameMap(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected map[string]string
	}{
		{
			name:  "No common prefixes",
			input: []string{"Alice", "Bob", "Charlie"},
			expected: map[string]string{
				"Alice":   "Alice",
				"Bob":     "Bob",
				"Charlie": "Charlie",
			},
		},
		{
			name:  "Common prefix of length < 3",
			input: []string{"Al", "An", "Bob"},
			expected: map[string]string{
				"Al":  "Al",
				"An":  "An",
				"Bob": "Bob",
			},
		},
		{
			name:  "Common prefix of length >= 3",
			input: []string{"EggscapeABC", "EggscapeDEF", "EggscapeGHI", "OtherPlayer"},
			expected: map[string]string{
				"EggscapeABC": "~ABC",
				"EggscapeDEF": "~DEF",
				"EggscapeGHI": "~GHI",
				"OtherPlayer": "OtherPlayer",
			},
		},
		{
			name:  "Multiple different common prefixes",
			input: []string{"EggscapeABC", "EggscapeDEF", "Cluck123", "Cluck456", "Solo"},
			expected: map[string]string{
				"EggscapeABC": "~ABC",
				"EggscapeDEF": "~DEF",
				"Cluck123":    "~123",
				"Cluck456":    "~456",
				"Solo":        "Solo",
			},
		},
		{
			name:  "One name is exactly the prefix of another",
			input: []string{"Eggscape", "EggscapeABC"},
			expected: map[string]string{
				"Eggscape":    "Eggscape",
				"EggscapeABC": "~ABC",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := makeShortNameMap(tc.input)
			if tc.expected == nil && got == nil {
				return
			}
			if len(got) != len(tc.expected) {
				t.Fatalf("expected map length %d, got %d", len(tc.expected), len(got))
			}
			for k, v := range tc.expected {
				if got[k] != v {
					t.Errorf("for key %q: expected %q, got %q", k, v, got[k])
				}
			}
		})
	}
}
