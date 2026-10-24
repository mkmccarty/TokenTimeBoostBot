package boost

import (
	"fmt"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
)

func TestIsOldBoostListMessage(t *testing.T) {
	contract := &Contract{
		ContractHash: "test-stale-hash",
		Location: []*LocationData{
			{
				ChannelID:  "200000000000000001",
				ListMsgID:  "300000000000000100",
				ReactionID: "300000000000000001",
				MessageIDs: []string{"300000000000000001", "300000000000000010", "300000000000000020", "300000000000000100"},
			},
		},
		CRMessageIDs: map[string]string{
			"200000000000000001": "300000000000000050",
		},
	}

	// Active list message is not old
	if isOldBoostListMessage(contract, "200000000000000001", "300000000000000100") {
		t.Errorf("expected isOldBoostListMessage to be false for active ListMsgID")
	}

	// Signup message is not an old boost list
	if isOldBoostListMessage(contract, "200000000000000001", "300000000000000001") {
		t.Errorf("expected isOldBoostListMessage to be false for ReactionID")
	}

	// CR message is not an old boost list
	if isOldBoostListMessage(contract, "200000000000000001", "300000000000000050") {
		t.Errorf("expected isOldBoostListMessage to be false for CR message")
	}

	// Previous list messages should be identified as old
	if !isOldBoostListMessage(contract, "200000000000000001", "300000000000000010") {
		t.Errorf("expected isOldBoostListMessage to be true for previous list message")
	}
	if !isOldBoostListMessage(contract, "200000000000000001", "300000000000000020") {
		t.Errorf("expected isOldBoostListMessage to be true for previous list message")
	}

	// Unrelated message ID should return false
	if isOldBoostListMessage(contract, "200000000000000001", "300000000000000999") {
		t.Errorf("expected isOldBoostListMessage to be false for unknown message")
	}

	// Unrelated channel should return false
	if isOldBoostListMessage(contract, "200000000000000999", "300000000000000010") {
		t.Errorf("expected isOldBoostListMessage to be false for unknown channel")
	}
}

func setupStaleTestContract(contractHash, channelID, currentListMsgID, oldListMsgID, signupMsgID string) *Contract {
	const userAID = "400000000000000001"
	const userBID = "400000000000000002"

	c := &Contract{
		ContractHash:         contractHash,
		ContractID:           "test-contract",
		CoopID:               "test-coop",
		StartTime:            time.Now(),
		EstimatedDuration:    2 * time.Hour,
		Style:                ContractFlagFastrun,
		State:                ContractStateFastrun,
		CurrentBoosterUserID: userAID,
		CreatorID:            []string{"400000000000000000"},
		Order:                []string{userAID, userBID},
		Boosters:             make(map[string]*Booster),
		Location: []*LocationData{
			{
				GuildID:    "100000000000000001",
				ChannelID:  channelID,
				ListMsgID:  currentListMsgID,
				ReactionID: signupMsgID,
				MessageIDs: []string{signupMsgID, oldListMsgID, currentListMsgID},
			},
		},
	}

	c.Boosters[userAID] = &Booster{
		UserID:         userAID,
		Name:           "Farmer A",
		Nick:           "Farmer A",
		BoostState:     BoostStateTokenTime,
		TokensWanted:   6,
		TokensReceived: 0,
		StartTime:      time.Now(),
	}
	c.Boosters[userBID] = &Booster{
		UserID:         userBID,
		Name:           "Farmer B",
		Nick:           "Farmer B",
		BoostState:     BoostStateUnboosted,
		TokensWanted:   6,
		TokensReceived: 0,
	}

	ContractsMutex.Lock()
	Contracts[contractHash] = c
	ContractsMutex.Unlock()

	return c
}

func TestStaleBoostList_TokenButtonClickCleansUpOldList(t *testing.T) {
	contractHash := "test-race-token-btn"
	chanID := "200000000000000001"
	activeListID := "300000000000000100"
	oldListID := "300000000000000090"
	signupID := "300000000000000001"
	userAID := "400000000000000001"
	userBID := "400000000000000002"

	contract := setupStaleTestContract(contractHash, chanID, activeListID, oldListID, signupID)
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contractHash)
		ContractsMutex.Unlock()
	}()

	client := dctest.New()

	// User B clicks token button on the old message (oldListID)
	event := dctest.ComponentButtonEventFull("100000000000000001", chanID, oldListID, userBID, "rc_#token#"+contractHash)

	HandleContractReactions(client, event)

	// 1. Verify old message was deleted
	deleteCalls := client.CallsTo("DeleteMessage")
	foundDelete := false
	for _, call := range deleteCalls {
		if len(call.Args) >= 2 && call.Args[0] == chanID && call.Args[1] == oldListID {
			foundDelete = true
			break
		}
	}
	if !foundDelete {
		t.Errorf("expected DeleteMessage to be called for stale message %s, calls: %+v", oldListID, deleteCalls)
	}

	// 2. Verify token was credited to the active booster (userA)
	contract.mutex.Lock()
	tokensRecv := contract.Boosters[userAID].TokensReceived
	contract.mutex.Unlock()
	if tokensRecv != 1 {
		t.Errorf("expected booster userA to receive 1 token despite clicking on old message, got %d", tokensRecv)
	}

	// 3. Verify active boost list message was refreshed via EditMessage
	editCalls := client.CallsTo("EditMessage")
	foundEditActive := false
	for _, call := range editCalls {
		if len(call.Args) >= 2 && call.Args[0] == chanID && call.Args[1] == activeListID {
			foundEditActive = true
			break
		}
	}
	if !foundEditActive {
		t.Errorf("expected EditMessage on active list %s, got calls: %+v", activeListID, editCalls)
	}
}

func TestStaleBoostList_RocketButtonClickCleansUpWithoutAdvancing(t *testing.T) {
	contractHash := "test-race-rocket-btn"
	chanID := "200000000000000002"
	activeListID := "300000000000000200"
	oldListID := "300000000000000190"
	signupID := "300000000000000002"
	userAID := "400000000000000001"
	userBID := "400000000000000002"

	contract := setupStaleTestContract(contractHash, chanID, activeListID, oldListID, signupID)
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contractHash)
		ContractsMutex.Unlock()
	}()

	client := dctest.New()

	// Active booster userA clicks rocket on stale message oldListID
	event := dctest.ComponentButtonEventFull("100000000000000001", chanID, oldListID, userAID, "rc_#boost#"+contractHash)

	HandleContractReactions(client, event)

	// 1. Verify stale message was deleted
	deleteCalls := client.CallsTo("DeleteMessage")
	foundDelete := false
	for _, call := range deleteCalls {
		if len(call.Args) >= 2 && call.Args[0] == chanID && call.Args[1] == oldListID {
			foundDelete = true
			break
		}
	}
	if !foundDelete {
		t.Errorf("expected DeleteMessage to be called for stale message %s, calls: %+v", oldListID, deleteCalls)
	}

	// 2. Verify contract did NOT advance again (userA is still active booster, not userB)
	if contract.Boosters[userAID].BoostState != BoostStateTokenTime {
		t.Errorf("expected userA to remain in BoostStateTokenTime, got %v", contract.Boosters[userAID].BoostState)
	}
	if contract.Boosters[userBID].BoostState != BoostStateUnboosted {
		t.Errorf("expected userB to remain unboosted, got %v", contract.Boosters[userBID].BoostState)
	}
}

func TestStaleBoostList_TokenEmojiReactionCleansUpOldList(t *testing.T) {
	contractHash := "test-race-token-emoji"
	chanID := "200000000000000003"
	activeListID := "300000000000000300"
	oldListID := "300000000000000290"
	signupID := "300000000000000003"
	userAID := "400000000000000001"
	userBID := "400000000000000002"

	contract := setupStaleTestContract(contractHash, chanID, activeListID, oldListID, signupID)
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contractHash)
		ContractsMutex.Unlock()
	}()

	client := dctest.New()

	// User B reacts with "token" emoji on the old message (oldListID)
	rxEvent := dctest.ReactionEvent("100000000000000001", chanID, oldListID, userBID, "token")

	ReactionAdd(client, rxEvent)

	// 1. Verify old message was deleted
	deleteCalls := client.CallsTo("DeleteMessage")
	foundDelete := false
	for _, call := range deleteCalls {
		if len(call.Args) >= 2 && call.Args[0] == chanID && call.Args[1] == oldListID {
			foundDelete = true
			break
		}
	}
	if !foundDelete {
		t.Errorf("expected DeleteMessage to be called for stale message %s, calls: %+v", oldListID, deleteCalls)
	}

	// 2. Verify token was credited to booster userA
	contract.mutex.Lock()
	tokensRecv := contract.Boosters[userAID].TokensReceived
	contract.mutex.Unlock()
	if tokensRecv != 1 {
		t.Errorf("expected booster userA to receive 1 token from emoji reaction on old message, got %d", tokensRecv)
	}
}

func TestStaleBoostList_RocketEmojiReactionCleansUpWithoutAdvancing(t *testing.T) {
	contractHash := "test-race-rocket-emoji"
	chanID := "200000000000000004"
	activeListID := "300000000000000400"
	oldListID := "300000000000000390"
	signupID := "300000000000000004"
	userAID := "400000000000000001"
	userBID := "400000000000000002"

	contract := setupStaleTestContract(contractHash, chanID, activeListID, oldListID, signupID)
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contractHash)
		ContractsMutex.Unlock()
	}()

	client := dctest.New()

	// User A reacts with rocket emoji on stale message oldListID
	rxEvent := dctest.ReactionEvent("100000000000000001", chanID, oldListID, userAID, boostIconName)

	ReactionAdd(client, rxEvent)

	// 1. Verify old message was deleted
	deleteCalls := client.CallsTo("DeleteMessage")
	foundDelete := false
	for _, call := range deleteCalls {
		if len(call.Args) >= 2 && call.Args[0] == chanID && call.Args[1] == oldListID {
			foundDelete = true
			break
		}
	}
	if !foundDelete {
		t.Errorf("expected DeleteMessage to be called for stale message %s, calls: %+v", oldListID, deleteCalls)
	}

	// 2. Verify userA is still the active booster and was not advanced again
	if contract.Boosters[userAID].BoostState != BoostStateTokenTime {
		t.Errorf("expected userA to remain in BoostStateTokenTime, got %v", contract.Boosters[userAID].BoostState)
	}
	if contract.Boosters[userBID].BoostState != BoostStateUnboosted {
		t.Errorf("expected userB to remain unboosted, got %v", contract.Boosters[userBID].BoostState)
	}
}

func TestChickenRunMessage_NotDeletedOnReactionOrButton(t *testing.T) {
	contractHash := "test-cr-msg-safety"
	chanID := "200000000000000001"
	activeListID := "300000000000000100"
	oldListID := "300000000000000090"
	signupID := "300000000000000001"
	crMsgID := "300000000000000999"
	userAID := "400000000000000001"
	userBID := "400000000000000002"

	contract := setupStaleTestContract(contractHash, chanID, activeListID, oldListID, signupID)
	contract.CRMessageIDs = map[string]string{
		chanID: crMsgID,
	}
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contractHash)
		ContractsMutex.Unlock()
	}()

	client := dctest.New()

	// 1. User B clicks RanChicken button on crMsgID
	crButtonEvent := dctest.ComponentButtonEventFull("100000000000000001", chanID, crMsgID, userBID, fmt.Sprintf("rc_#RanChicken#%s#%s", userAID, contractHash))
	HandleContractReactions(client, crButtonEvent)

	// Verify DeleteMessage was NEVER called for the CR message
	for _, call := range client.CallsTo("DeleteMessage") {
		if len(call.Args) >= 2 && call.Args[1] == crMsgID {
			t.Fatalf("DeleteMessage was unexpectedly called for CR message %s", crMsgID)
		}
	}

	// 2. User B clicks CRPing on crMsgID
	crPingEvent := dctest.ComponentButtonEventFull("100000000000000001", chanID, crMsgID, userBID, fmt.Sprintf("rc_#CRPing#%s", contractHash))
	HandleContractReactions(client, crPingEvent)

	for _, call := range client.CallsTo("DeleteMessage") {
		if len(call.Args) >= 2 && call.Args[1] == crMsgID {
			t.Fatalf("DeleteMessage was unexpectedly called for CR message %s after CRPing", crMsgID)
		}
	}

	// 3. User B reacts with an emoji on crMsgID
	rxEvent := dctest.ReactionEvent("100000000000000001", chanID, crMsgID, userBID, "👍")
	ReactionAdd(client, rxEvent)

	for _, call := range client.CallsTo("DeleteMessage") {
		if len(call.Args) >= 2 && call.Args[1] == crMsgID {
			t.Fatalf("DeleteMessage was unexpectedly called for CR message %s after reaction", crMsgID)
		}
	}
}
