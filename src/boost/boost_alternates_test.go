package boost

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

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

func TestAddFarmerToContract_AutoLinkAlts_GuestAndActAs(t *testing.T) {
	client := dctest.New().
		WithGuild("guild-alt-1", "Guild 1").
		WithChannel("chan-alt-1", "guild-alt-1", "contract-channel").
		WithUser("main-u1", "mainUser", "Main User").
		WithUser("alt-u1", "altUser", "Alt User").
		WithUser("stranger-u1", "strangerUser", "Stranger User")

	mainID := "main-u1"
	altID := "alt-u1"
	strangerID := "stranger-u1"
	guestAlt := "GuestAltOnly"

	// Guest alt has AltController only (no act_as_links).
	farmerstate.SetMiscSettingString(guestAlt, "AltController", mainID)
	// Discord alt with AltController only (no act_as_links) should also auto-link.
	farmerstate.SetMiscSettingString(strangerID, "AltController", mainID)
	// Discord alt with act_as_links only (no AltController) must NOT auto-link because
	// /act-as and AltController are separate concepts.
	_ = farmerstate.AddActAsLink(mainID, altID)
	defer func() {
		farmerstate.SetMiscSettingString(guestAlt, "AltController", "")
		farmerstate.SetMiscSettingString(strangerID, "AltController", "")
		_ = farmerstate.RemoveActAsLink(mainID, altID)
	}()

	contract := &Contract{
		ContractHash: "test-hash-autolink-guest-actas",
		ContractID:   "test-contract",
		CoopID:       "test-coop",
		CoopSize:     10,
		State:        ContractStateSignup,
		CreatorID:    []string{mainID},
		Order:        make([]string, 0),
		Boosters:     make(map[string]*Booster),
		Location:     []*LocationData{{GuildID: "guild-alt-1", ChannelID: "chan-alt-1"}},
	}
	ContractsMutex.Lock()
	Contracts[contract.ContractHash] = contract
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract.ContractHash)
		ContractsMutex.Unlock()
	}()

	// Add alts BEFORE main joins to test reverse-direction auto-linking too.
	guestB, err := AddFarmerToContract(client, contract, "guild-alt-1", "chan-alt-1", guestAlt, ContractOrderSignup, false, false)
	if err != nil || guestB == nil {
		t.Fatalf("failed to add guest alt: %v", err)
	}
	altB, err := AddFarmerToContract(client, contract, "guild-alt-1", "chan-alt-1", altID, ContractOrderSignup, false, false)
	if err != nil || altB == nil {
		t.Fatalf("failed to add discord alt: %v", err)
	}
	strangerB, err := AddFarmerToContract(client, contract, "guild-alt-1", "chan-alt-1", strangerID, ContractOrderSignup, false, false)
	if err != nil || strangerB == nil {
		t.Fatalf("failed to add stranger: %v", err)
	}

	mainB, err := AddFarmerToContract(client, contract, "guild-alt-1", "chan-alt-1", mainID, ContractOrderSignup, true, false)
	if err != nil || mainB == nil {
		t.Fatalf("failed to add main: %v", err)
	}

	if guestB.AltController != mainID || !slices.Contains(mainB.Alts, guestAlt) {
		t.Errorf("guest alt not auto-linked: AltController=%q, main.Alts=%v", guestB.AltController, mainB.Alts)
	}
	if altB.AltController != "" || slices.Contains(mainB.Alts, altID) {
		t.Errorf("act-as-only discord user should NOT be auto-linked as AltController: AltController=%q, main.Alts=%v", altB.AltController, mainB.Alts)
	}
	if strangerB.AltController != mainID || !slices.Contains(mainB.Alts, strangerID) {
		t.Errorf("discord alt with AltController not auto-linked: AltController=%q, main.Alts=%v", strangerB.AltController, mainB.Alts)
	}
}

func TestAddFarmerToContract_DiscordNameDoesNotCollideWithGuestAltController(t *testing.T) {
	mainID := "100000000000000301"
	discordOtherID := "100000000000000302"
	guestName := "SharedFarmName"

	client := dctest.New().
		WithGuild("guild-col-1", "Guild Col").
		WithChannel("chan-col-1", "guild-col-1", "contract-channel").
		WithUser(mainID, "mainUser", "MainUser").
		WithUser(discordOtherID, guestName, guestName)

	farmerstate.SetMiscSettingString(guestName, "AltController", mainID)
	defer farmerstate.SetMiscSettingString(guestName, "AltController", "")

	contract := &Contract{
		ContractHash: "test-hash-name-collision",
		ContractID:   "test-contract",
		CoopID:       "test-coop",
		CoopSize:     10,
		State:        ContractStateSignup,
		CreatorID:    []string{mainID},
		Order:        make([]string, 0),
		Boosters:     make(map[string]*Booster),
		Location:     []*LocationData{{GuildID: "guild-col-1", ChannelID: "chan-col-1"}},
	}
	ContractsMutex.Lock()
	Contracts[contract.ContractHash] = contract
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract.ContractHash)
		ContractsMutex.Unlock()
	}()

	mainB, err := AddFarmerToContract(client, contract, "guild-col-1", "chan-col-1", mainID, ContractOrderSignup, true, false)
	if err != nil || mainB == nil {
		t.Fatalf("failed to add main: %v", err)
	}
	otherB, err := AddFarmerToContract(client, contract, "guild-col-1", "chan-col-1", discordOtherID, ContractOrderSignup, false, false)
	if err != nil || otherB == nil {
		t.Fatalf("failed to add other discord user: %v", err)
	}

	if otherB.AltController != "" || slices.Contains(mainB.Alts, discordOtherID) {
		t.Errorf("Discord user %s should not auto-link via guest name collision: AltController=%q, main.Alts=%v", discordOtherID, otherB.AltController, mainB.Alts)
	}
}

func TestLinkedAltBidirectionalInteractions(t *testing.T) {
	mainID := "100000000000000401"
	altID := "100000000000000402"
	alt2ID := "100000000000000403"
	otherID := "100000000000000404"
	guildID := "900000000000000401"
	channelID := "900000000000000402"

	makeContract := func() *Contract {
		return &Contract{
			ContractHash: "test-hash-bidir",
			ContractID:   "test-contract",
			CoopID:       "test-coop",
			CoopSize:     4,
			State:        ContractStateFastrun,
			CreatorID:    []string{otherID},
			Location:     []*LocationData{{GuildID: guildID, ChannelID: channelID, ListMsgID: "msg-list"}},
			Order:        []string{mainID, altID, alt2ID, otherID},
			Boosters: map[string]*Booster{
				mainID:  {UserID: mainID, Name: "Main", Nick: "Main", Mention: "<@" + mainID + ">", Alts: []string{altID, alt2ID}},
				altID:   {UserID: altID, Name: "AltOne", Nick: "AltOne", Mention: "<@" + altID + ">", AltController: mainID},
				alt2ID:  {UserID: alt2ID, Name: "AltTwo", Nick: "AltTwo", Mention: "<@" + alt2ID + ">", AltController: mainID},
				otherID: {UserID: otherID, Name: "Other", Nick: "Other", Mention: "<@" + otherID + ">"},
			},
			CRMessageIDs: map[string]string{channelID: "900000000000000499"},
		}
	}

	// 1. resolveBoostClickUserID works for main->alt, alt->main, and alt1->alt2
	c := makeContract()
	if got := resolveBoostClickUserID(c, mainID, altID); got != altID {
		t.Errorf("resolveBoostClickUserID(main, alt) = %q, want %q", got, altID)
	}
	if got := resolveBoostClickUserID(c, altID, mainID); got != mainID {
		t.Errorf("resolveBoostClickUserID(alt, main) = %q, want %q", got, mainID)
	}
	if got := resolveBoostClickUserID(c, altID, alt2ID); got != alt2ID {
		t.Errorf("resolveBoostClickUserID(alt1, alt2) = %q, want %q", got, alt2ID)
	}
	if got := resolveBoostClickUserID(c, altID, otherID); got != altID {
		t.Errorf("resolveBoostClickUserID(alt, other) = %q, want %q", got, altID)
	}

	// 2. buttonReactionRunChickens works when alt clicks and main is boosted
	client := dctest.New().
		WithGuild(guildID, "Test Guild").
		WithChannel(channelID, guildID, "contract-channel")
	c2 := makeContract()
	c2.Boosters[mainID].BoostState = BoostStateBoosted
	c2.Boosters[altID].BoostState = BoostStateUnboosted
	ok, _ := buttonReactionRunChickens(client, c2, altID)
	if !ok {
		t.Fatalf("expected buttonReactionRunChickens from altID to succeed for boosted mainID")
	}
	if c2.Boosters[mainID].RunChickensTime.IsZero() {
		t.Errorf("expected mainID.RunChickensTime to be set when altID clicked CR")
	}

	// 3. buttonReactionRanChicken from altID marks mainID, altID, and alt2ID
	c3 := makeContract()
	c3.Boosters[otherID].BoostState = BoostStateBoosted
	c3.Boosters[otherID].RunChickensTime = time.Now()
	crPayload := fmt.Sprintf(`{
		"id": "201",
		"application_id": "200",
		"type": 3,
		"token": "tok",
		"version": 1,
		"guild_id": %q,
		"channel": {"id": %q, "type": 0},
		"message": {"id": "900000000000000499", "channel_id": %q},
		"member": {"user": {"id": %q, "username": "alt", "discriminator": "0"}},
		"user": {"id": %q, "username": "alt", "discriminator": "0"},
		"data": {"component_type": 2, "custom_id": "rc_#RanChicken#%s#%s"}
	}`, guildID, channelID, channelID, altID, altID, otherID, c3.ContractHash)
	crEvent, err := dc.NewComponentEventFromPayload([]byte(crPayload))
	if err != nil {
		t.Fatalf("failed to create component event: %v", err)
	}
	buttonReactionRanChicken(client, crEvent, c3, altID, otherID)
	for _, id := range []string{mainID, altID, alt2ID} {
		if !slices.Contains(c3.Boosters[id].RanChickensOn, otherID) {
			t.Errorf("expected %s to have %s in RanChickensOn after alt clicked RanChicken, got %v", id, otherID, c3.Boosters[id].RanChickensOn)
		}
	}

	// 4. buttonReactionRanCoop from altID marks all linked accounts, and second press from mainID clears all
	c4 := makeContract()
	buttonReactionRanCoop(client, crEvent, c4, altID)
	for _, id := range []string{mainID, altID, alt2ID} {
		if !slices.Contains(c4.Boosters[id].RanChickensOn, otherID) {
			t.Errorf("expected %s to have %s in RanChickensOn after alt clicked RanCoop", id, otherID)
		}
		if slices.Contains(c4.Boosters[id].RanChickensOn, mainID) || slices.Contains(c4.Boosters[id].RanChickensOn, altID) {
			t.Errorf("linked member %s should not mark self/group in RanCoop: %v", id, c4.Boosters[id].RanChickensOn)
		}
	}
	buttonReactionRanCoop(client, crEvent, c4, mainID)
	for _, id := range []string{mainID, altID, alt2ID} {
		if len(c4.Boosters[id].RanChickensOn) != 0 {
			t.Errorf("expected %s RanChickensOn to be cleared on second RanCoop press from main, got %v", id, c4.Boosters[id].RanChickensOn)
		}
	}
}

func newLinkAlternateCommandEvent(guildID, channelID, userID, farmerName string) (*dc.CommandEvent, error) {
	payload := fmt.Sprintf(`{
		"id": "101",
		"application_id": "200",
		"type": 2,
		"token": "tok",
		"version": 1,
		"guild_id": %q,
		"channel": {"id": %q, "type": 0},
		"member": {"user": {"id": %q, "username": "user", "discriminator": "0"}},
		"user": {"id": %q, "username": "user", "discriminator": "0"},
		"data": {"id": "1", "name": "link-alternate", "type": 1, "options": [{"name": "farmer-name", "type": 3, "value": %q}]}
	}`, guildID, channelID, userID, userID, farmerName)
	return dc.NewCommandEventFromPayload([]byte(payload))
}

func newLinkAlternateAutocompleteEvent(guildID, channelID, userID string) (*dc.AutocompleteEvent, error) {
	payload := fmt.Sprintf(`{
		"id": "102",
		"application_id": "200",
		"type": 4,
		"token": "tok",
		"version": 1,
		"guild_id": %q,
		"channel": {"id": %q, "type": 0},
		"member": {"user": {"id": %q, "username": "user", "discriminator": "0"}},
		"user": {"id": %q, "username": "user", "discriminator": "0"},
		"data": {"id": "1", "name": "link-alternate", "type": 1, "options": [{"name": "farmer-name", "type": 3, "value": "", "focused": true}]}
	}`, guildID, channelID, userID, userID)
	return dc.NewAutocompleteEventFromPayload([]byte(payload))
}

func TestHandleLinkAlternate_DiscordAndGuestAlts(t *testing.T) {
	guildID := "900000000000000001"
	channelID := "900000000000000002"
	mainID := "100000000000000201"
	linkedAltID := "100000000000000202"
	unlinkedID := "100000000000000203"
	guestAlt := "GuestTwo"

	client := dctest.New().
		WithGuild(guildID, "Test Guild").
		WithChannel(channelID, guildID, "contract-channel")

	_ = farmerstate.AddActAsLink(mainID, linkedAltID)
	defer func() {
		_ = farmerstate.RemoveActAsLink(mainID, linkedAltID)
		farmerstate.SetMiscSettingString(linkedAltID, "AltController", "")
		farmerstate.SetMiscSettingString(guestAlt, "AltController", "")
	}()

	contract := &Contract{
		ContractHash: "test-hash-link-alternate",
		ContractID:   "test-contract",
		CoopID:       "test-coop",
		CoopSize:     10,
		State:        ContractStateSignup,
		CreatorID:    []string{mainID},
		Location:     []*LocationData{{GuildID: guildID, ChannelID: channelID}},
		Order:        []string{mainID, linkedAltID, unlinkedID, guestAlt},
		Boosters: map[string]*Booster{
			mainID:      {UserID: mainID, Name: "MainName", Nick: "MainName", Mention: "<@" + mainID + ">"},
			linkedAltID: {UserID: linkedAltID, Name: "LinkedAltName", Nick: "LinkedAltName", Mention: "<@" + linkedAltID + ">"},
			unlinkedID:  {UserID: unlinkedID, Name: "OtherPlayer", Nick: "OtherPlayer", Mention: "<@" + unlinkedID + ">"},
			guestAlt:    {UserID: guestAlt, Name: guestAlt, Nick: guestAlt, Mention: guestAlt},
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

	// 1. Autocomplete should include all other unlinked boosters (Discord and guest), but NOT self.
	acEvent, err := newLinkAlternateAutocompleteEvent(guildID, channelID, mainID)
	if err != nil {
		t.Fatalf("failed to create autocomplete event: %v", err)
	}
	HandleLinkAlternateAutoComplete(acEvent)
	choices := acEvent.LastChoices
	var choiceValues []string
	for _, c := range choices {
		choiceValues = append(choiceValues, fmt.Sprint(c.Value))
	}
	if !slices.Contains(choiceValues, linkedAltID) {
		t.Errorf("autocomplete choices %v missing linked Discord alt %s", choiceValues, linkedAltID)
	}
	if !slices.Contains(choiceValues, unlinkedID) {
		t.Errorf("autocomplete choices %v missing Discord booster %s", choiceValues, unlinkedID)
	}
	if !slices.Contains(choiceValues, guestAlt) {
		t.Errorf("autocomplete choices %v missing guest alt %s", choiceValues, guestAlt)
	}
	if slices.Contains(choiceValues, mainID) {
		t.Errorf("autocomplete choices %v should not include self %s", choiceValues, mainID)
	}

	// 2. Linking self must be rejected.
	eSelf, _ := newLinkAlternateCommandEvent(guildID, channelID, mainID, "MainName")
	HandleLinkAlternateCommand(client, eSelf)
	if contract.Boosters[mainID].AltController != "" {
		t.Errorf("self should not be linked, got AltController=%q", contract.Boosters[mainID].AltController)
	}

	// 3. Linking a Discord alt without /act-as by display name must succeed.
	eUnlinked, _ := newLinkAlternateCommandEvent(guildID, channelID, mainID, "OtherPlayer")
	HandleLinkAlternateCommand(client, eUnlinked)
	if contract.Boosters[unlinkedID].AltController != mainID {
		t.Errorf("Discord alt without act-as AltController = %q, want %q", contract.Boosters[unlinkedID].AltController, mainID)
	}
	if !slices.Contains(contract.Boosters[mainID].Alts, unlinkedID) {
		t.Errorf("main Alts %v missing %s", contract.Boosters[mainID].Alts, unlinkedID)
	}
	farmerstate.SetMiscSettingString(unlinkedID, "AltController", "")

	// 3. Linking a linked Discord alt by display name must succeed.
	eLinked, _ := newLinkAlternateCommandEvent(guildID, channelID, mainID, "LinkedAltName")
	HandleLinkAlternateCommand(client, eLinked)
	if contract.Boosters[linkedAltID].AltController != mainID {
		t.Errorf("linked Discord alt AltController = %q, want %q", contract.Boosters[linkedAltID].AltController, mainID)
	}
	if !slices.Contains(contract.Boosters[mainID].Alts, linkedAltID) {
		t.Errorf("main Alts %v missing %s", contract.Boosters[mainID].Alts, linkedAltID)
	}

	// 4. Linking a guest alt by name must succeed.
	eGuest, _ := newLinkAlternateCommandEvent(guildID, channelID, mainID, guestAlt)
	HandleLinkAlternateCommand(client, eGuest)
	if contract.Boosters[guestAlt].AltController != mainID {
		t.Errorf("guest alt AltController = %q, want %q", contract.Boosters[guestAlt].AltController, mainID)
	}
	if !slices.Contains(contract.Boosters[mainID].Alts, guestAlt) {
		t.Errorf("main Alts %v missing %s", contract.Boosters[mainID].Alts, guestAlt)
	}
}
