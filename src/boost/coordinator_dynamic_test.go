package boost

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/ewohltman/discordgo-mock/mockchannel"
	"github.com/ewohltman/discordgo-mock/mockconstants"
	"github.com/ewohltman/discordgo-mock/mockguild"
	"github.com/ewohltman/discordgo-mock/mockmember"
	"github.com/ewohltman/discordgo-mock/mockrest"
	"github.com/ewohltman/discordgo-mock/mockrole"
	"github.com/ewohltman/discordgo-mock/mocksession"
	"github.com/ewohltman/discordgo-mock/mockstate"
	"github.com/ewohltman/discordgo-mock/mockuser"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"
)

func createMockSession() (*discordgo.Session, error) {
	role := mockrole.New(
		mockrole.WithID(mockconstants.TestRole),
		mockrole.WithName(mockconstants.TestRole),
		mockrole.WithPermissions(discordgo.PermissionViewChannel),
	)

	botUser := mockuser.New(
		mockuser.WithID(mockconstants.TestUser+"Bot"),
		mockuser.WithUsername(mockconstants.TestUser+"Bot"),
		mockuser.WithBotFlag(true),
	)

	botMember := mockmember.New(
		mockmember.WithUser(botUser),
		mockmember.WithGuildID(mockconstants.TestGuild),
		mockmember.WithRoles(role),
	)

	userMember := mockmember.New(
		mockmember.WithUser(mockuser.New(
			mockuser.WithID(mockconstants.TestUser),
			mockuser.WithUsername(mockconstants.TestUser),
		)),
		mockmember.WithGuildID(mockconstants.TestGuild),
		mockmember.WithRoles(role),
	)

	channel := mockchannel.New(
		mockchannel.WithID(mockconstants.TestChannel),
		mockchannel.WithGuildID(mockconstants.TestGuild),
		mockchannel.WithName(mockconstants.TestChannel),
		mockchannel.WithType(discordgo.ChannelTypeGuildText),
	)

	state, err := mockstate.New(
		mockstate.WithUser(botUser),
		mockstate.WithGuilds(
			mockguild.New(
				mockguild.WithID(mockconstants.TestGuild),
				mockguild.WithName(mockconstants.TestGuild),
				mockguild.WithRoles(role),
				mockguild.WithChannels(channel),
				mockguild.WithMembers(botMember, userMember),
			),
		),
	)
	if err != nil {
		return nil, err
	}

	session, err := mocksession.New(
		mocksession.WithState(state),
		mocksession.WithClient(&http.Client{
			Transport: mockrest.NewTransport(state),
		}),
	)
	return session, err
}

func TestDynamicGuildCoordinator(t *testing.T) {
	// 1. Add a test coordinator to guildstate
	guildID := mockconstants.TestGuild
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

	s, err := createMockSession()
	if err != nil {
		t.Fatalf("Failed to create mock session: %v", err)
	}

	contract, err := CreateContract(s, contractID, coopID, ContractPlaystyleChill, 10, ContractOrderSignup, guildID, mockconstants.TestChannel, []string{creatorUserID}, creatorUserID, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("Failed to create contract: %v", err)
	}

	// Verify that original creator is in CreatorID
	if !slices.Contains(contract.CreatorID, creatorUserID) {
		t.Errorf("Expected original creator %s to be in contract.CreatorID", creatorUserID)
	}

	// Verify that the guild coordinator is NOT statically appended to CreatorID anymore
	if slices.Contains(contract.CreatorID, coordinatorUserID) {
		t.Errorf("Expected guild coordinator %s NOT to be statically appended to contract.CreatorID", coordinatorUserID)
	}

	// 3. Test creatorOfContract helper - it should dynamically authorize the guild coordinator
	// Even though coordinatorUserID is not in contract.CreatorID, creatorOfContract should return true
	isAuthorized := creatorOfContract(s, contract, coordinatorUserID)
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
	if !creatorOfContract(s, contract, "mock-primary-admin") {
		t.Error("Expected creatorOfContract to dynamically authorize primary admin")
	}
	if !creatorOfContract(s, contract, "mock-admin-2") {
		t.Error("Expected creatorOfContract to dynamically authorize secondary admin")
	}

	// Create another contract and verify admins are not statically copied into contract.CreatorID
	delete(Contracts, contract.ContractHash)
	contract2, err := CreateContract(s, contractID, "coop-dynamic-2", ContractPlaystyleChill, 10, ContractOrderSignup, guildID, mockconstants.TestChannel, []string{creatorUserID}, creatorUserID, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("Failed to create contract2: %v", err)
	}

	if slices.Contains(contract2.CreatorID, "mock-primary-admin") {
		t.Error("Expected mock-primary-admin NOT to be statically appended to contract.CreatorID")
	}
	if slices.Contains(contract2.CreatorID, "mock-admin-2") {
		t.Error("Expected mock-admin-2 NOT to be statically appended to contract.CreatorID")
	}
}
