package boost

import (
	"slices"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"
)

const (
	testGuildID   = "guild-123"
	testChannelID = "channel-coord-1"
)

// testContractChannels is every channel the contract tests create a contract
// in. The fake client answers Channel for these and returns not-found for
// anything else, which is what an unknown channel looks like to CreateContract.
var testContractChannels = []string{
	testChannelID,
	"channel-1",
	"channel-2",
	"channel-3",
	"channel-123-1",
	"channel-123-2",
	"channel-123-3",
	"channel-prog-1",
	"channel-restart-1",
}

// newTestClient is the dc.Client the contract tests run against: it knows the
// test guild and the channels above, and records every call.
func newTestClient() *dctest.FakeClient {
	client := dctest.New().WithGuild(testGuildID, "Test Guild")
	for _, channelID := range testContractChannels {
		client.WithChannel(channelID, testGuildID, channelID)
	}
	return client
}

func TestDynamicGuildCoordinator(t *testing.T) {
	// 1. Add a test coordinator to guildstate
	guildID := testGuildID
	coordinatorUserID := "user-coord-123"
	adminUserID := "admin-user"

	// Cleanup existing to ensure clean slate
	_ = guildstate.RemoveGuildCoordinator(guildID, coordinatorUserID)

	err := guildstate.AddGuildCoordinator(guildID, coordinatorUserID, adminUserID)
	if err != nil {
		t.Fatalf("Failed to add guild coordinator: %v", err)
	}
	defer func() {
		_ = guildstate.RemoveGuildCoordinator(guildID, coordinatorUserID)
	}()

	// Verify that IsGuildCoordinator works
	if !guildstate.IsGuildCoordinator(guildID, coordinatorUserID) {
		t.Fatalf("Expected user %s to be a coordinator for guild %s", coordinatorUserID, guildID)
	}

	// 2. Test CreateContract - check that the coordinator is NOT statically added to CreatorID
	contractID := "dynamic-coord-test-contract"
	coopID := "coop-dynamic-1"
	creatorUserID := "original-creator-456"

	client := newTestClient()

	contract, err := CreateContract(client, contractID, coopID, ContractPlaystyleChill, 10, ContractOrderSignup, guildID, testChannelID, []string{creatorUserID}, creatorUserID, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("Failed to create contract: %v", err)
	}
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract.ContractHash)
		ContractsMutex.Unlock()
	}()

	// Verify that original creator is in CreatorID
	if !slices.Contains(contract.CreatorID, creatorUserID) {
		t.Errorf("Expected original creator %s to be in contract.CreatorID", creatorUserID)
	}

	// Verify that the guild coordinator is NOT statically appended to CreatorID anymore
	if slices.Contains(contract.CreatorID, coordinatorUserID) {
		t.Errorf("Expected guild coordinator %s NOT to be statically appended to contract.CreatorID", coordinatorUserID)
	}

	// The contract picked up the guild and channel the client reported.
	if contract.Location[0].GuildName != "Test Guild" {
		t.Errorf("GuildName = %q, want %q", contract.Location[0].GuildName, "Test Guild")
	}

	// 3. Test creatorOfContract helper - it should dynamically authorize the guild coordinator
	// Even though coordinatorUserID is not in contract.CreatorID, creatorOfContract should return true
	isAuthorized := creatorOfContract(client, contract, coordinatorUserID)
	if !isAuthorized {
		t.Errorf("Expected creatorOfContract to dynamically return true for guild coordinator %s", coordinatorUserID)
	}

	// 4. Test global admins check
	// Save existing values and restore them on defer
	oldAdminUserID := config.AdminUserID
	oldAdminUsers := config.AdminUsers
	defer func() {
		config.AdminUserID = oldAdminUserID
		config.AdminUsers = oldAdminUsers
	}()

	config.AdminUserID = "mock-primary-admin"
	config.AdminUsers = []string{"mock-admin-2", "mock-admin-3"}

	// Verify that they are recognized as guild coordinators
	if !guildstate.IsGuildCoordinator(guildID, "mock-primary-admin") {
		t.Error("Expected primary admin to be recognized as guild coordinator")
	}
	if !guildstate.IsGuildCoordinator(guildID, "mock-admin-2") {
		t.Error("Expected secondary admin to be recognized as guild coordinator")
	}

	// Verify creatorOfContract recognizes them dynamically
	if !creatorOfContract(client, contract, "mock-primary-admin") {
		t.Error("Expected creatorOfContract to dynamically authorize primary admin")
	}
	if !creatorOfContract(client, contract, "mock-admin-2") {
		t.Error("Expected creatorOfContract to dynamically authorize secondary admin")
	}

	// Create another contract and verify admins are not statically copied into contract.CreatorID
	delete(Contracts, contract.ContractHash)
	contract2, err := CreateContract(client, contractID, "coop-dynamic-2", ContractPlaystyleChill, 10, ContractOrderSignup, guildID, testChannelID, []string{creatorUserID}, creatorUserID, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("Failed to create contract2: %v", err)
	}
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract2.ContractHash)
		ContractsMutex.Unlock()
	}()

	if slices.Contains(contract2.CreatorID, "mock-primary-admin") {
		t.Error("Expected mock-primary-admin NOT to be statically appended to contract.CreatorID")
	}
	if slices.Contains(contract2.CreatorID, "mock-admin-2") {
		t.Error("Expected mock-admin-2 NOT to be statically appended to contract.CreatorID")
	}
}
