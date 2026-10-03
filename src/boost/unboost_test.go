package boost

import (
	"slices"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
)

func TestUnboost_ActiveContractAccidentalRocket(t *testing.T) {
	client := dctest.New().
		WithGuild("guild1", "Guild 1").
		WithChannel("channel1", "guild1", "contract-channel").
		WithUser("100000000000000001", "farmer1", "Farmer One").
		WithUser("100000000000000002", "farmer2", "Farmer Two").
		WithUser("100000000000000003", "farmer3", "Farmer Three")

	oldEnd := time.Now().Add(-5 * time.Minute)
	contract := &Contract{
		ContractHash:         "test-hash-unboost-accidental",
		ContractID:           "test-contract",
		CoopID:               "test-coop",
		CoopSize:             3,
		State:                ContractStateFastrun,
		Style:                ContractFlagFastrun,
		BoostOrder:           ContractOrderSignup,
		CreatorID:            []string{"100000000000000001"},
		CurrentBoosterUserID: "100000000000000002",
		BoostPosition:        1,
		Order:                []string{"100000000000000001", "100000000000000002", "100000000000000003"},
		BoostedOrder:         []string{"100000000000000001"},
		Boosters: map[string]*Booster{
			"100000000000000001": {UserID: "100000000000000001", Name: "Farmer One", Nick: "farmer1", Mention: "<@100000000000000001>", BoostState: BoostStateBoosted, EndTime: oldEnd, Duration: 5 * time.Minute},
			"100000000000000002": {UserID: "100000000000000002", Name: "Farmer Two", Nick: "farmer2", Mention: "<@100000000000000002>", BoostState: BoostStateTokenTime},
			"100000000000000003": {UserID: "100000000000000003", Name: "Farmer Three", Nick: "farmer3", Mention: "<@100000000000000003>", BoostState: BoostStateUnboosted},
		},
		Location: []*LocationData{
			{GuildID: "guild1", ChannelID: "channel1", ListMsgID: "msg-list-1"},
		},
	}
	ContractsMutex.Lock()
	Contracts[contract.ContractHash] = contract
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract.ContractHash)
		ContractsMutex.Unlock()
	}()

	err := Unboost(client, "guild1", "channel1", "<@100000000000000001>")
	if err != nil {
		t.Fatalf("unexpected error running Unboost: %v", err)
	}

	// Farmer One should now be the current active booster
	if got := contract.currentBoosterID(); got != "100000000000000001" {
		t.Errorf("currentBoosterID = %q, want %q", got, "100000000000000001")
	}
	if b1 := contract.Boosters["100000000000000001"]; b1.BoostState != BoostStateTokenTime {
		t.Errorf("farmer1 BoostState = %v, want BoostStateTokenTime", b1.BoostState)
	} else if !b1.EndTime.IsZero() {
		t.Errorf("farmer1 EndTime = %v, want zero time", b1.EndTime)
	} else if b1.Duration != 0 {
		t.Errorf("farmer1 Duration = %v, want 0", b1.Duration)
	}

	// Farmer Two should be reverted to BoostStateUnboosted
	if b2 := contract.Boosters["100000000000000002"]; b2.BoostState != BoostStateUnboosted {
		t.Errorf("farmer2 BoostState = %v, want BoostStateUnboosted", b2.BoostState)
	}

	// BoostPosition should be pointing to farmer 1 (index 0)
	if contract.BoostPosition != 0 {
		t.Errorf("BoostPosition = %d, want 0", contract.BoostPosition)
	}

	// Farmer One should be removed from BoostedOrder
	if slices.Contains(contract.BoostedOrder, "100000000000000001") {
		t.Errorf("expected farmer1 to be removed from BoostedOrder")
	}
}

func TestUnboost_ContractWaiting(t *testing.T) {
	client := dctest.New().
		WithGuild("guild1", "Guild 1").
		WithChannel("channel1", "guild1", "contract-channel").
		WithUser("100000000000000001", "farmer1", "Farmer One").
		WithUser("100000000000000002", "farmer2", "Farmer Two")

	contract := &Contract{
		ContractHash:         "test-hash-unboost-waiting",
		ContractID:           "test-contract",
		CoopID:               "test-coop",
		CoopSize:             3,
		State:                ContractStateWaiting,
		Style:                ContractFlagFastrun,
		BoostOrder:           ContractOrderSignup,
		CreatorID:            []string{"100000000000000001"},
		CurrentBoosterUserID: "",
		BoostPosition:        2,
		Order:                []string{"100000000000000001", "100000000000000002"},
		BoostedOrder:         []string{"100000000000000001", "100000000000000002"},
		Boosters: map[string]*Booster{
			"100000000000000001": {UserID: "100000000000000001", Name: "Farmer One", Nick: "farmer1", Mention: "<@100000000000000001>", BoostState: BoostStateBoosted},
			"100000000000000002": {UserID: "100000000000000002", Name: "Farmer Two", Nick: "farmer2", Mention: "<@100000000000000002>", BoostState: BoostStateBoosted},
		},
		Location: []*LocationData{
			{GuildID: "guild1", ChannelID: "channel1", ListMsgID: "msg-list-1"},
		},
	}
	ContractsMutex.Lock()
	Contracts[contract.ContractHash] = contract
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract.ContractHash)
		ContractsMutex.Unlock()
	}()

	err := Unboost(client, "guild1", "channel1", "<@100000000000000002>")
	if err != nil {
		t.Fatalf("unexpected error running Unboost: %v", err)
	}

	if contract.State != ContractStateFastrun {
		t.Errorf("contract.State = %v, want ContractStateFastrun", contract.State)
	}
	if got := contract.currentBoosterID(); got != "100000000000000002" {
		t.Errorf("currentBoosterID = %q, want %q", got, "100000000000000002")
	}
	if b2 := contract.Boosters["100000000000000002"]; b2.BoostState != BoostStateTokenTime {
		t.Errorf("farmer2 BoostState = %v, want BoostStateTokenTime", b2.BoostState)
	}
	if slices.Contains(contract.BoostedOrder, "100000000000000002") {
		t.Errorf("expected farmer2 to be removed from BoostedOrder")
	}
}

func TestUnboost_ContractCompleted(t *testing.T) {
	client := dctest.New().
		WithGuild("guild1", "Guild 1").
		WithChannel("channel1", "guild1", "contract-channel").
		WithUser("100000000000000001", "farmer1", "Farmer One").
		WithUser("100000000000000002", "farmer2", "Farmer Two")

	contract := &Contract{
		ContractHash:         "test-hash-unboost-completed",
		ContractID:           "test-contract",
		CoopID:               "test-coop",
		CoopSize:             2,
		State:                ContractStateCompleted,
		Style:                ContractFlagFastrun,
		EndTime:              time.Now(),
		BoostOrder:           ContractOrderSignup,
		CreatorID:            []string{"100000000000000001"},
		CurrentBoosterUserID: "",
		BoostPosition:        2,
		Order:                []string{"100000000000000001", "100000000000000002"},
		BoostedOrder:         []string{"100000000000000001", "100000000000000002"},
		Boosters: map[string]*Booster{
			"100000000000000001": {UserID: "100000000000000001", Name: "Farmer One", Nick: "farmer1", Mention: "<@100000000000000001>", BoostState: BoostStateBoosted},
			"100000000000000002": {UserID: "100000000000000002", Name: "Farmer Two", Nick: "farmer2", Mention: "<@100000000000000002>", BoostState: BoostStateBoosted},
		},
		Location: []*LocationData{
			{GuildID: "guild1", ChannelID: "channel1", ListMsgID: "msg-list-1"},
		},
	}
	ContractsMutex.Lock()
	Contracts[contract.ContractHash] = contract
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract.ContractHash)
		ContractsMutex.Unlock()
	}()

	err := Unboost(client, "guild1", "channel1", "<@100000000000000002>")
	if err != nil {
		t.Fatalf("unexpected error running Unboost: %v", err)
	}

	if contract.State != ContractStateFastrun {
		t.Errorf("contract.State = %v, want ContractStateFastrun", contract.State)
	}
	if !contract.EndTime.IsZero() {
		t.Errorf("contract.EndTime = %v, want zero time", contract.EndTime)
	}
	if got := contract.currentBoosterID(); got != "100000000000000002" {
		t.Errorf("currentBoosterID = %q, want %q", got, "100000000000000002")
	}
	if b2 := contract.Boosters["100000000000000002"]; b2.BoostState != BoostStateTokenTime {
		t.Errorf("farmer2 BoostState = %v, want BoostStateTokenTime", b2.BoostState)
	}
	if slices.Contains(contract.BoostedOrder, "100000000000000002") {
		t.Errorf("expected farmer2 to be removed from BoostedOrder")
	}
}

func TestUnboost_ContractSignupError(t *testing.T) {
	client := dctest.New().
		WithGuild("guild1", "Guild 1").
		WithChannel("channel1", "guild1", "contract-channel").
		WithUser("100000000000000001", "farmer1", "Farmer One")

	contract := &Contract{
		ContractHash: "test-hash-unboost-signup",
		ContractID:   "test-contract",
		CoopID:       "test-coop",
		CoopSize:     2,
		State:        ContractStateSignup,
		Style:        ContractFlagFastrun,
		CreatorID:    []string{"100000000000000001"},
		Order:        []string{"100000000000000001"},
		Boosters: map[string]*Booster{
			"100000000000000001": {UserID: "100000000000000001", Name: "Farmer One", Nick: "farmer1", Mention: "<@100000000000000001>", BoostState: BoostStateUnboosted},
		},
		Location: []*LocationData{
			{GuildID: "guild1", ChannelID: "channel1", ListMsgID: "msg-list-1"},
		},
	}
	ContractsMutex.Lock()
	Contracts[contract.ContractHash] = contract
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract.ContractHash)
		ContractsMutex.Unlock()
	}()

	err := Unboost(client, "guild1", "channel1", "<@100000000000000001>")
	if err == nil || err.Error() != errorContractNotStarted {
		t.Fatalf("expected error %q, got %v", errorContractNotStarted, err)
	}
}
