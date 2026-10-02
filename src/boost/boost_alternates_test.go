package boost

import (
	"fmt"
	"slices"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
)

func newHelperCommandEvent(guildID, channelID, userID, subcommand, farmersOpt string, hasFarmers bool) (*dc.CommandEvent, error) {
	var subOptions string
	if hasFarmers {
		subOptions = fmt.Sprintf(`"options": [{"name": "farmers", "type": 3, "value": %q}]`, farmersOpt)
	}
	var dataOptions string
	if subOptions != "" {
		dataOptions = fmt.Sprintf(`[{"name": %q, "type": 1, %s}]`, subcommand, subOptions)
	} else {
		dataOptions = fmt.Sprintf(`[{"name": %q, "type": 1}]`, subcommand)
	}

	payload := fmt.Sprintf(`{
		"id": "100",
		"application_id": "200",
		"type": 2,
		"token": "tok",
		"version": 1,
		"guild_id": %q,
		"channel": {"id": %q, "type": 0},
		"member": {"user": {"id": %q, "username": "testuser", "discriminator": "0"}},
		"user": {"id": %q, "username": "testuser", "discriminator": "0"},
		"data": {"id": "1", "name": "boost-order-helpers", "type": 1, "options": %s}
	}`, guildID, channelID, userID, userID, dataOptions)

	return dc.NewCommandEventFromPayload([]byte(payload))
}

func TestCanManageBooster(t *testing.T) {
	client := dctest.New()
	contract := &Contract{
		ContractHash: "test-hash",
		CreatorID:    []string{"1001"},
		Location:     []*LocationData{{GuildID: "9001", ChannelID: "8001"}},
		Boosters: map[string]*Booster{
			"1001": {UserID: "1001", Name: "Coord"},
			"1002": {UserID: "1002", Name: "Main", Alts: []string{"1003"}},
			"1003": {UserID: "1003", Name: "Alt", AltController: "1002"},
			"1004": {UserID: "1004", Name: "Other"},
		},
	}

	// Coordinator can manage anyone
	if !canManageBooster(client, contract, "1001", "1004") {
		t.Error("expected coordinator to be able to manage other booster")
	}

	// User can manage themselves
	if !canManageBooster(client, contract, "1002", "1002") {
		t.Error("expected user to be able to manage self")
	}

	// User can manage their linked alt (via AltController & Alts list)
	if !canManageBooster(client, contract, "1002", "1003") {
		t.Error("expected user to be able to manage their alt")
	}

	// User cannot manage someone else
	if canManageBooster(client, contract, "1002", "1004") {
		t.Error("expected user to NOT be able to manage another farmer")
	}
}

func TestHandleBoostOrderHelpers_NonCoordinatorSelfSetAndClear(t *testing.T) {
	guildID := "9001"
	channelID := "8001"
	client := dctest.New().WithGuild(guildID, "Test Guild")
	client.WithChannel(channelID, guildID, "contract-channel")

	contract := &Contract{
		ContractHash: "helper-test-hash",
		CreatorID:    []string{"1001"},
		Location:     []*LocationData{{GuildID: guildID, ChannelID: channelID}},
		Order:        []string{"1002", "1004", "1003"},
		Boosters: map[string]*Booster{
			"1001": {UserID: "1001", Name: "Coordinator"},
			"1002": {UserID: "1002", Name: "MainUser"},
			"1004": {UserID: "1004", Name: "OtherUser"},
			"1003": {UserID: "1003", Name: "AltUser", AltController: "1002"},
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

	// 1. Non-coordinator runs /boost-order-helpers set with no parameters -> marks self as helper
	e1, err := newHelperCommandEvent(guildID, channelID, "1002", "set", "", false)
	if err != nil {
		t.Fatalf("failed to create command event: %v", err)
	}
	HandleBoostOrderHelpersCommand(client, e1)

	if !contract.Boosters["1002"].IsAlt {
		t.Error("expected 1002 to be marked as helper when set is called without arguments")
	}

	// 2. Non-coordinator runs /boost-order-helpers set targeting another user -> rejected, other user remains not alt
	e2, err := newHelperCommandEvent(guildID, channelID, "1002", "set", "1004", true)
	if err != nil {
		t.Fatalf("failed to create command event: %v", err)
	}
	HandleBoostOrderHelpersCommand(client, e2)

	if contract.Boosters["1004"].IsAlt {
		t.Error("expected 1004 to NOT be marked as helper when set by non-coordinator")
	}

	// 3. Non-coordinator runs /boost-order-helpers set targeting their own alt -> succeeds
	e3, err := newHelperCommandEvent(guildID, channelID, "1002", "set", "1003", true)
	if err != nil {
		t.Fatalf("failed to create command event: %v", err)
	}
	HandleBoostOrderHelpersCommand(client, e3)

	if !contract.Boosters["1003"].IsAlt {
		t.Error("expected 1003 to be marked as helper when set by alt controller")
	}

	// 4. Non-coordinator runs /boost-order-helpers clear targeting another user -> rejected
	e4, err := newHelperCommandEvent(guildID, channelID, "1002", "clear", "1004", true)
	if err != nil {
		t.Fatalf("failed to create command event: %v", err)
	}
	HandleBoostOrderHelpersCommand(client, e4)

	// 5. Non-coordinator runs /boost-order-helpers clear targeting their own alt -> clears alt
	e5, err := newHelperCommandEvent(guildID, channelID, "1002", "clear", "1003", true)
	if err != nil {
		t.Fatalf("failed to create command event: %v", err)
	}
	HandleBoostOrderHelpersCommand(client, e5)

	if contract.Boosters["1003"].IsAlt {
		t.Error("expected 1003 helper status to be cleared when targeted by alt controller")
	}
	if !contract.Boosters["1002"].IsAlt {
		t.Error("expected 1002 to still be alt")
	}

	// 6. Mark alt as helper again so we can test blank clear
	contract.Boosters["1003"].IsAlt = true

	// 7. Non-coordinator runs /boost-order-helpers clear with no arguments -> clears both self and linked alts
	e6, err := newHelperCommandEvent(guildID, channelID, "1002", "clear", "", false)
	if err != nil {
		t.Fatalf("failed to create command event: %v", err)
	}
	HandleBoostOrderHelpersCommand(client, e6)

	if contract.Boosters["1002"].IsAlt {
		t.Error("expected 1002 helper status to be cleared by clear with no arguments")
	}
	if contract.Boosters["1003"].IsAlt {
		t.Error("expected linked alt 1003 helper status to be cleared by clear with no arguments")
	}

	// 8. Non-coordinator runs /boost-order-helpers list -> succeeds without error
	e7, err := newHelperCommandEvent(guildID, channelID, "1002", "list", "", false)
	if err != nil {
		t.Fatalf("failed to create command event: %v", err)
	}
	HandleBoostOrderHelpersCommand(client, e7)

	// 9. Coordinator can set another user and clear all
	contract.Boosters["1004"].IsAlt = true
	e8, err := newHelperCommandEvent(guildID, channelID, "1001", "clear", "all", true)
	if err != nil {
		t.Fatalf("failed to create command event: %v", err)
	}
	HandleBoostOrderHelpersCommand(client, e8)

	if contract.Boosters["1004"].IsAlt {
		t.Error("expected 1004 helper status to be cleared by coordinator clear all")
	}
}

func TestHandleBoostOrderHelpers_NonContractParticipant(t *testing.T) {
	guildID := "9001"
	channelID := "8001"
	client := dctest.New().WithGuild(guildID, "Test Guild")
	client.WithChannel(channelID, guildID, "contract-channel")

	contract := &Contract{
		ContractHash: "helper-test-hash-2",
		CreatorID:    []string{"1001"},
		Location:     []*LocationData{{GuildID: guildID, ChannelID: channelID}},
		Order:        []string{"1002"},
		Boosters: map[string]*Booster{
			"1002": {UserID: "1002", Name: "MainUser"},
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

	// User not in contract calling set with no params should not modify contract
	e, err := newHelperCommandEvent(guildID, channelID, "1099", "set", "", false)
	if err != nil {
		t.Fatalf("failed to create command event: %v", err)
	}
	HandleBoostOrderHelpersCommand(client, e)

	if slices.Contains(contract.Order, "1099") {
		t.Error("1099 should not be in contract")
	}
}
