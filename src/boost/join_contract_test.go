package boost

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
)

func newJoinCommandEvent(guildID, channelID, callerID, farmerOpt, roleOpt string) (*dc.CommandEvent, error) {
	var opts []string
	if farmerOpt != "" {
		opts = append(opts, fmt.Sprintf(`{"name": "farmer", "type": 3, "value": %q}`, farmerOpt))
	}
	if roleOpt != "" {
		opts = append(opts, fmt.Sprintf(`{"name": "role", "type": 3, "value": %q}`, roleOpt))
	}

	optionsJSON := "[" + strings.Join(opts, ", ") + "]"

	payload := fmt.Sprintf(`{
		"id": "100",
		"application_id": "200",
		"type": 2,
		"token": "tok",
		"version": 1,
		"guild_id": %q,
		"channel": {"id": %q, "type": 0},
		"member": {"user": {"id": %q, "username": "caller", "discriminator": "0"}},
		"user": {"id": %q, "username": "caller", "discriminator": "0"},
		"data": {"id": "1", "name": "join-contract", "type": 1, "options": %s}
	}`, guildID, channelID, callerID, callerID, optionsJSON)

	return dc.NewCommandEventFromPayload([]byte(payload))
}

func TestGetSlashJoinContractCommand_RoleOption(t *testing.T) {
	cmd := GetSlashJoinContractCommand("join-contract")
	if cmd == nil {
		t.Fatal("expected command definition, got nil")
	}

	var roleOpt *dc.StringOption
	for _, opt := range cmd.Options {
		if sOpt, ok := opt.(dc.StringOption); ok && sOpt.Name == "role" {
			roleOpt = &sOpt
			break
		}
	}

	if roleOpt == nil {
		t.Fatal("expected 'role' option in /join-contract command")
	}

	if roleOpt.Required {
		t.Error("expected 'role' option to be optional (Required == false)")
	}

	if len(roleOpt.Choices) != 2 {
		t.Fatalf("expected 2 choices for role option, got %d", len(roleOpt.Choices))
	}

	choiceMap := make(map[string]string)
	for _, c := range roleOpt.Choices {
		choiceMap[c.Name] = c.Value
	}

	if val, ok := choiceMap["Main"]; !ok || val != "Main" {
		t.Errorf("expected Choice Main -> Main, got %v", choiceMap["Main"])
	}
	if val, ok := choiceMap["Helper"]; !ok || val != "Helper" {
		t.Errorf("expected Choice Helper -> Helper, got %v", choiceMap["Helper"])
	}
}

func TestHandleJoinCommand_RoleDefaultsToMain(t *testing.T) {
	guildID := "9001"
	channelID := "8001"
	client := dctest.New().WithGuild(guildID, "Test Guild")
	client.WithChannel(channelID, guildID, "contract-channel")

	contract := &Contract{
		ContractHash: "join-test-hash-default",
		CreatorID:    []string{"1001"},
		Location:     []*LocationData{{GuildID: guildID, ChannelID: channelID}},
		Order:        []string{},
		Boosters:     make(map[string]*Booster),
		CoopSize:     10,
		State:        ContractStateSignup,
		BoostOrder:   ContractOrderSignup,
	}
	ContractsMutex.Lock()
	Contracts[contract.ContractHash] = contract
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract.ContractHash)
		ContractsMutex.Unlock()
	}()

	// Join guest without role parameter
	e, err := newJoinCommandEvent(guildID, channelID, "1001", "GuestOne", "")
	if err != nil {
		t.Fatalf("failed to create join command event: %v", err)
	}
	HandleJoinCommand(client, e)

	b := contract.Boosters["GuestOne"]
	if b == nil {
		t.Fatal("expected GuestOne to be added to contract")
	}
	if b.IsHelper {
		t.Errorf("expected GuestOne role to default to Main (IsHelper=false), got IsHelper=%v", b.IsHelper)
	}
	if b.IsAlt {
		t.Errorf("expected GuestOne IsAlt to be false, got IsAlt=%v", b.IsAlt)
	}
}

func TestHandleJoinCommand_RoleHelper(t *testing.T) {
	guildID := "9001"
	channelID := "8002"
	client := dctest.New().WithGuild(guildID, "Test Guild")
	client.WithChannel(channelID, guildID, "contract-channel")

	contract := &Contract{
		ContractHash: "join-test-hash-helper",
		CreatorID:    []string{"1001"},
		Location:     []*LocationData{{GuildID: guildID, ChannelID: channelID}},
		Order:        []string{},
		Boosters:     make(map[string]*Booster),
		CoopSize:     10,
		State:        ContractStateSignup,
		BoostOrder:   ContractOrderSignup,
	}
	ContractsMutex.Lock()
	Contracts[contract.ContractHash] = contract
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract.ContractHash)
		ContractsMutex.Unlock()
	}()

	// Join guest with role=Helper
	e, err := newJoinCommandEvent(guildID, channelID, "1001", "HelperGuest", "Helper")
	if err != nil {
		t.Fatalf("failed to create join command event: %v", err)
	}
	HandleJoinCommand(client, e)

	b := contract.Boosters["HelperGuest"]
	if b == nil {
		t.Fatal("expected HelperGuest to be added to contract")
	}
	if !b.IsHelper {
		t.Errorf("expected HelperGuest to have IsHelper=true, got IsHelper=%v", b.IsHelper)
	}
	// Just because someone is a helper doesn't mean it's an Alt
	if b.IsAlt {
		t.Errorf("expected HelperGuest to NOT be marked as an Alt (IsAlt=false), got IsAlt=%v", b.IsAlt)
	}
	if !isBoosterHelper(b) {
		t.Error("expected isBoosterHelper(b) to be true for helper")
	}
}

func TestHandleJoinCommand_RoleMainExplicit(t *testing.T) {
	guildID := "9001"
	channelID := "8003"
	client := dctest.New().WithGuild(guildID, "Test Guild")
	client.WithChannel(channelID, guildID, "contract-channel")

	contract := &Contract{
		ContractHash: "join-test-hash-main-explicit",
		CreatorID:    []string{"1001"},
		Location:     []*LocationData{{GuildID: guildID, ChannelID: channelID}},
		Order:        []string{},
		Boosters:     make(map[string]*Booster),
		CoopSize:     10,
		State:        ContractStateSignup,
		BoostOrder:   ContractOrderSignup,
	}
	ContractsMutex.Lock()
	Contracts[contract.ContractHash] = contract
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract.ContractHash)
		ContractsMutex.Unlock()
	}()

	// Join guest with role=Main
	e, err := newJoinCommandEvent(guildID, channelID, "1001", "MainGuest", "Main")
	if err != nil {
		t.Fatalf("failed to create join command event: %v", err)
	}
	HandleJoinCommand(client, e)

	b := contract.Boosters["MainGuest"]
	if b == nil {
		t.Fatal("expected MainGuest to be added to contract")
	}
	if b.IsHelper {
		t.Errorf("expected MainGuest to have IsHelper=false, got IsHelper=%v", b.IsHelper)
	}
	if b.IsAlt {
		t.Errorf("expected MainGuest to have IsAlt=false, got IsAlt=%v", b.IsAlt)
	}
}

func TestHandleJoinCommand_HelperSortingAndNonSticky(t *testing.T) {
	guildID := "9001"
	channelID := "8004"
	client := dctest.New().WithGuild(guildID, "Test Guild")
	client.WithChannel(channelID, guildID, "contract-channel")

	contract1 := &Contract{
		ContractHash: "join-test-hash-order-1",
		CreatorID:    []string{"1001"},
		Location:     []*LocationData{{GuildID: guildID, ChannelID: channelID}},
		Order:        []string{},
		Boosters:     make(map[string]*Booster),
		CoopSize:     10,
		State:        ContractStateSignup,
		BoostOrder:   ContractOrderSignup,
	}
	ContractsMutex.Lock()
	Contracts[contract1.ContractHash] = contract1
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract1.ContractHash)
		ContractsMutex.Unlock()
	}()

	// 1. Join GuestMain1 as Main
	e1, _ := newJoinCommandEvent(guildID, channelID, "1001", "GuestMain1", "Main")
	HandleJoinCommand(client, e1)

	// 2. Join GuestHelper1 as Helper
	e2, _ := newJoinCommandEvent(guildID, channelID, "1001", "GuestHelper1", "Helper")
	HandleJoinCommand(client, e2)

	// 3. Join GuestMain2 with no role (defaults to Main)
	e3, _ := newJoinCommandEvent(guildID, channelID, "1001", "GuestMain2", "")
	HandleJoinCommand(client, e3)

	// Expected order in ContractOrderSignup: Mains first (GuestMain1, GuestMain2), then Helpers (GuestHelper1)
	expectedOrder := []string{"GuestMain1", "GuestMain2", "GuestHelper1"}
	if !slices.Equal(contract1.Order, expectedOrder) {
		t.Fatalf("expected order %v, got %v", expectedOrder, contract1.Order)
	}

	// 4. Test non-sticky: The user "GuestHelper1" joining a second contract without role defaults to Main
	channelID2 := "8005"
	client.WithChannel(channelID2, guildID, "contract-channel-2")
	contract2 := &Contract{
		ContractHash: "join-test-hash-order-2",
		CreatorID:    []string{"1001"},
		Location:     []*LocationData{{GuildID: guildID, ChannelID: channelID2}},
		Order:        []string{},
		Boosters:     make(map[string]*Booster),
		CoopSize:     10,
		State:        ContractStateSignup,
		BoostOrder:   ContractOrderSignup,
	}
	ContractsMutex.Lock()
	Contracts[contract2.ContractHash] = contract2
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract2.ContractHash)
		ContractsMutex.Unlock()
	}()

	e4, _ := newJoinCommandEvent(guildID, channelID2, "1001", "GuestHelper1", "")
	HandleJoinCommand(client, e4)

	b2 := contract2.Boosters["GuestHelper1"]
	if b2 == nil {
		t.Fatal("expected GuestHelper1 to be added to second contract")
	}
	if b2.IsHelper {
		t.Errorf("expected GuestHelper1 in new contract to default to Main (IsHelper=false), but got IsHelper=true (was sticky)")
	}
	if b2.IsAlt {
		t.Errorf("expected GuestHelper1 in new contract to have IsAlt=false, got IsAlt=%v", b2.IsAlt)
	}
}

func TestHandleJoinCommand_MentionWithRole(t *testing.T) {
	guildID := "9001"
	channelID := "8006"
	client := dctest.New().WithGuild(guildID, "Test Guild")
	client.WithChannel(channelID, guildID, "contract-channel")
	client.WithUser("5001", "MentionUser", "MentionUser")
	client.WithMember(guildID, "5001", "MentionUser", 0)

	contract := &Contract{
		ContractHash: "join-test-hash-mention-role",
		CreatorID:    []string{"1001"},
		Location:     []*LocationData{{GuildID: guildID, ChannelID: channelID}},
		Order:        []string{},
		Boosters:     make(map[string]*Booster),
		CoopSize:     10,
		State:        ContractStateSignup,
		BoostOrder:   ContractOrderSignup,
	}
	ContractsMutex.Lock()
	Contracts[contract.ContractHash] = contract
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract.ContractHash)
		ContractsMutex.Unlock()
	}()

	// Mention user with Helper role
	e, err := newJoinCommandEvent(guildID, channelID, "1001", "<@5001>", "Helper")
	if err != nil {
		t.Fatalf("failed to create join command event: %v", err)
	}
	HandleJoinCommand(client, e)

	b := contract.Boosters["5001"]
	if b == nil {
		t.Fatal("expected 5001 to be added to contract")
	}
	if !b.IsHelper {
		t.Errorf("expected mention user to have IsHelper=true for Helper role, got IsHelper=%v", b.IsHelper)
	}
	// Just because someone is a helper doesn't mean it's an Alt
	if b.IsAlt {
		t.Errorf("expected mention user to NOT have IsAlt=true, got IsAlt=%v", b.IsAlt)
	}
	if !isBoosterHelper(b) {
		t.Error("expected isBoosterHelper(b) to be true for mention helper")
	}
}
