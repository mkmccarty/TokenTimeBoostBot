package boost

import (
	"fmt"
	"slices"
	"testing"

	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

func newHelperCommandEvent(guildID, channelID, userID, subcommand, farmersOpt string, hasFarmers bool) (*dc.CommandEvent, error) {
	return newHelperCommandEventFull(guildID, channelID, userID, subcommand, farmersOpt, hasFarmers, false, false, "")
}

func newHelperCommandEventFull(guildID, channelID, userID, subcommand, farmersOpt string, hasFarmers bool, sticky bool, hasSticky bool, role string) (*dc.CommandEvent, error) {
	var opts []string
	if hasFarmers {
		opts = append(opts, fmt.Sprintf(`{"name": "farmers", "type": 3, "value": %q}`, farmersOpt))
	}
	if hasSticky {
		opts = append(opts, fmt.Sprintf(`{"name": "sticky", "type": 5, "value": %t}`, sticky))
	}
	if role != "" {
		opts = append(opts, fmt.Sprintf(`{"name": "role", "type": 3, "value": %q}`, role))
	}
	var subOptions string
	if len(opts) > 0 {
		subOptions = fmt.Sprintf(`"options": [%s]`, strings.Join(opts, ", "))
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

func TestGetSlashBoostOrderHelpersCommand_StickyAndRoleOptions(t *testing.T) {
	cmd := GetSlashBoostOrderHelpersCommand("boost-order-helpers")
	if cmd == nil {
		t.Fatal("expected command definition")
	}

	var setSub, clearSub *dc.SubCommand
	for _, opt := range cmd.Options {
		if sub, ok := opt.(dc.SubCommand); ok {
			switch sub.Name {
			case "set":
				setCopy := sub
				setSub = &setCopy
			case "clear":
				clearCopy := sub
				clearSub = &clearCopy
			}
		}
	}

	if setSub == nil {
		t.Fatal("expected 'set' subcommand in /boost-order-helpers")
	}
	if clearSub == nil {
		t.Fatal("expected 'clear' subcommand in /boost-order-helpers")
	}

	// Verify sticky option in set
	var setHasSticky, setHasRole bool
	for _, opt := range setSub.Options {
		if bOpt, ok := opt.(dc.BoolOption); ok && bOpt.Name == "sticky" {
			setHasSticky = true
		}
		if sOpt, ok := opt.(dc.StringOption); ok && sOpt.Name == "role" {
			setHasRole = true
		}
	}
	if !setHasSticky {
		t.Error("expected 'sticky' bool option in 'set' subcommand")
	}
	if !setHasRole {
		t.Error("expected 'role' string option in 'set' subcommand")
	}

	// Verify sticky option in clear
	var clearHasSticky bool
	for _, opt := range clearSub.Options {
		if bOpt, ok := opt.(dc.BoolOption); ok && bOpt.Name == "sticky" {
			clearHasSticky = true
		}
	}
	if !clearHasSticky {
		t.Error("expected 'sticky' bool option in 'clear' subcommand")
	}
}

func TestHandleBoostOrderHelpers_StickyOption(t *testing.T) {
	guildID := "9001"
	channelID := "8001"
	client := dctest.New().WithGuild(guildID, "Test Guild")
	client.WithChannel(channelID, guildID, "contract-channel")

	contract := &Contract{
		ContractHash: "helper-sticky-test-1",
		CreatorID:    []string{"1001"},
		Location:     []*LocationData{{GuildID: guildID, ChannelID: channelID}},
		Order:        []string{"1001", "1002", "1003", "1004"},
		Boosters: map[string]*Booster{
			"1001": {UserID: "1001", Name: "Coord"},
			"1002": {UserID: "1002", Name: "Main", Alts: []string{"1003"}},
			"1003": {UserID: "1003", Name: "Alt", AltController: "1002"},
			"1004": {UserID: "1004", Name: "Other"},
		},
		CoopSize:   10,
		State:      ContractStateSignup,
		BoostOrder: ContractOrderSignup,
	}
	ContractsMutex.Lock()
	Contracts[contract.ContractHash] = contract
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract.ContractHash)
		ContractsMutex.Unlock()
		farmerstate.SetOrderHelper("1002", false)
		farmerstate.SetOrderHelper("1003", false)
		farmerstate.SetOrderHelper("1004", false)
	}()

	// 1. User 1002 sets self as helper with sticky=false: marked in contract, but NOT in farmerstate
	e1, _ := newHelperCommandEventFull(guildID, channelID, "1002", "set", "", false, false, true, "")
	HandleBoostOrderHelpersCommand(client, e1)

	if !contract.Boosters["1002"].IsHelper {
		t.Error("expected 1002 to be helper in contract")
	}
	if farmerstate.GetOrderHelper("1002") {
		t.Error("expected 1002 to NOT have sticky helper in farmerstate when sticky=false")
	}

	// 2. User 1002 sets self as helper with sticky=true: marked in contract AND in farmerstate
	e2, _ := newHelperCommandEventFull(guildID, channelID, "1002", "set", "", false, true, true, "")
	HandleBoostOrderHelpersCommand(client, e2)

	if !contract.Boosters["1002"].IsHelper {
		t.Error("expected 1002 to be helper in contract")
	}
	if !farmerstate.GetOrderHelper("1002") {
		t.Error("expected 1002 to have sticky helper in farmerstate when sticky=true")
	}

	// 3. User 1002 clears self with sticky=true: cleared in contract AND removed from farmerstate
	e3, _ := newHelperCommandEventFull(guildID, channelID, "1002", "clear", "1002", true, true, true, "")
	HandleBoostOrderHelpersCommand(client, e3)

	if contract.Boosters["1002"].IsHelper {
		t.Error("expected 1002 helper status to be cleared in contract")
	}
	if farmerstate.GetOrderHelper("1002") {
		t.Error("expected 1002 sticky helper to be removed from farmerstate after clear sticky=true")
	}

	// 4. User 1002 sets self as helper with sticky=true, then sets role: Main with sticky=true
	e4, _ := newHelperCommandEventFull(guildID, channelID, "1002", "set", "", false, true, true, "Helper")
	HandleBoostOrderHelpersCommand(client, e4)
	if !farmerstate.GetOrderHelper("1002") {
		t.Fatal("expected 1002 to have sticky helper before testing role:Main")
	}

	e5, _ := newHelperCommandEventFull(guildID, channelID, "1002", "set", "", false, true, true, "Main")
	HandleBoostOrderHelpersCommand(client, e5)

	if contract.Boosters["1002"].IsHelper {
		t.Error("expected 1002 to have IsHelper=false after role:Main")
	}
	if farmerstate.GetOrderHelper("1002") {
		t.Error("expected 1002 sticky helper to be removed from farmerstate after role:Main sticky=true")
	}

	// 5. Non-coordinator tries to set another user with sticky=true -> rejected, target farmerstate untouched
	e6, _ := newHelperCommandEventFull(guildID, channelID, "1002", "set", "1004", true, true, true, "Helper")
	HandleBoostOrderHelpersCommand(client, e6)

	if contract.Boosters["1004"].IsHelper {
		t.Error("expected 1004 to NOT be helper when set by non-coordinator")
	}
	if farmerstate.GetOrderHelper("1004") {
		t.Error("expected 1004 sticky helper to NOT be set when non-coordinator call rejected")
	}

	// 6. Coordinator sets another user with sticky=true -> succeeds
	e7, _ := newHelperCommandEventFull(guildID, channelID, "1001", "set", "1004", true, true, true, "Helper")
	HandleBoostOrderHelpersCommand(client, e7)

	if !contract.Boosters["1004"].IsHelper {
		t.Error("expected 1004 to be helper when set by coordinator")
	}
	if !farmerstate.GetOrderHelper("1004") {
		t.Error("expected 1004 sticky helper in farmerstate when set by coordinator with sticky=true")
	}
}

func TestStickyHelper_AppliesOnFutureContractJoins(t *testing.T) {
	guildID := "9001"
	channelID1 := "8010"
	channelID2 := "8011"
	channelID3 := "8012"
	client := dctest.New().WithGuild(guildID, "Test Guild")
	client.WithChannel(channelID1, guildID, "contract-1")
	client.WithChannel(channelID2, guildID, "contract-2")
	client.WithChannel(channelID3, guildID, "contract-3")
	client.WithUser("1002", "User1002", "User1002")
	client.WithMember(guildID, "1002", "User1002", 0)

	contract1 := &Contract{
		ContractHash: "sticky-join-test-1",
		CreatorID:    []string{"1001"},
		Location:     []*LocationData{{GuildID: guildID, ChannelID: channelID1}},
		Order:        []string{"1001", "1002"},
		Boosters: map[string]*Booster{
			"1001": {UserID: "1001", Name: "Coord"},
			"1002": {UserID: "1002", Name: "MainUser"},
		},
		CoopSize:   10,
		State:      ContractStateSignup,
		BoostOrder: ContractOrderSignup,
	}
	ContractsMutex.Lock()
	Contracts[contract1.ContractHash] = contract1
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract1.ContractHash)
		ContractsMutex.Unlock()
		farmerstate.SetOrderHelper("1002", false)
	}()

	// In contract 1, user 1002 runs /boost-order-helpers set with sticky=true
	e1, _ := newHelperCommandEventFull(guildID, channelID1, "1002", "set", "", false, true, true, "")
	HandleBoostOrderHelpersCommand(client, e1)

	if !contract1.Boosters["1002"].IsHelper {
		t.Fatal("expected 1002 to be helper in contract 1")
	}
	if !farmerstate.GetOrderHelper("1002") {
		t.Fatal("expected 1002 to have sticky helper in farmerstate")
	}

	// Contract 2 is created. User 1002 joins without role parameter via /join-contract.
	contract2 := &Contract{
		ContractHash: "sticky-join-test-2",
		CreatorID:    []string{"1001"},
		Location:     []*LocationData{{GuildID: guildID, ChannelID: channelID2}},
		Order:        []string{"1001"},
		Boosters: map[string]*Booster{
			"1001": {UserID: "1001", Name: "Coord"},
		},
		CoopSize:   10,
		State:      ContractStateSignup,
		BoostOrder: ContractOrderSignup,
	}
	ContractsMutex.Lock()
	Contracts[contract2.ContractHash] = contract2
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract2.ContractHash)
		ContractsMutex.Unlock()
	}()

	e2, _ := newJoinCommandEvent(guildID, channelID2, "1002", "", "")
	HandleJoinCommand(client, e2)

	b2 := contract2.Boosters["1002"]
	if b2 == nil {
		t.Fatal("expected 1002 to be added to contract 2")
	}
	if !b2.IsHelper {
		t.Errorf("expected 1002 to have IsHelper=true on joining new contract due to sticky helper, got %v", b2.IsHelper)
	}

	// Contract 3 is created. User 1002 joins via JoinContract (emoji reaction).
	contract3 := &Contract{
		ContractHash: "sticky-join-test-3",
		CreatorID:    []string{"1001"},
		Location:     []*LocationData{{GuildID: guildID, ChannelID: channelID3}},
		Order:        []string{"1001"},
		Boosters: map[string]*Booster{
			"1001": {UserID: "1001", Name: "Coord"},
		},
		CoopSize:   10,
		State:      ContractStateSignup,
		BoostOrder: ContractOrderSignup,
	}
	ContractsMutex.Lock()
	Contracts[contract3.ContractHash] = contract3
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract3.ContractHash)
		ContractsMutex.Unlock()
	}()

	err := JoinContract(client, guildID, channelID3, "1002", false)
	if err != nil {
		t.Fatalf("JoinContract failed: %v", err)
	}

	b3 := contract3.Boosters["1002"]
	if b3 == nil {
		t.Fatal("expected 1002 to be added to contract 3")
	}
	if !b3.IsHelper {
		t.Errorf("expected 1002 to have IsHelper=true on JoinContract (reaction) due to sticky helper, got %v", b3.IsHelper)
	}

	// Now clear sticky helper for 1002 in contract 3
	e3, _ := newHelperCommandEventFull(guildID, channelID3, "1002", "clear", "", false, true, true, "")
	HandleBoostOrderHelpersCommand(client, e3)

	if b3.IsHelper {
		t.Error("expected 1002 to be cleared in contract 3")
	}
	if farmerstate.GetOrderHelper("1002") {
		t.Error("expected 1002 sticky helper to be removed from farmerstate")
	}

	// Contract 4 is created. User 1002 joins without role parameter; should now default to Main.
	channelID4 := "8013"
	client.WithChannel(channelID4, guildID, "contract-4")
	contract4 := &Contract{
		ContractHash: "sticky-join-test-4",
		CreatorID:    []string{"1001"},
		Location:     []*LocationData{{GuildID: guildID, ChannelID: channelID4}},
		Order:        []string{"1001"},
		Boosters: map[string]*Booster{
			"1001": {UserID: "1001", Name: "Coord"},
		},
		CoopSize:   10,
		State:      ContractStateSignup,
		BoostOrder: ContractOrderSignup,
	}
	ContractsMutex.Lock()
	Contracts[contract4.ContractHash] = contract4
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract4.ContractHash)
		ContractsMutex.Unlock()
	}()

	e4, _ := newJoinCommandEvent(guildID, channelID4, "1002", "", "")
	HandleJoinCommand(client, e4)

	b4 := contract4.Boosters["1002"]
	if b4 == nil {
		t.Fatal("expected 1002 to be added to contract 4")
	}
	if b4.IsHelper {
		t.Errorf("expected 1002 to default to Main (IsHelper=false) after sticky was removed, got %v", b4.IsHelper)
	}
}
