package leaderboard

import (
	"database/sql"
	"slices"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

func TestGuildScopedSnapDates(t *testing.T) {
	guildA := "test_guild_alpha"
	guildB := "test_guild_beta"
	userA1 := "test_user_a1"
	userA2 := "test_user_a2"
	userB1 := "test_user_b1"
	userB2 := "test_user_b2"
	userShared := "test_user_shared"

	cleanup := func() {
		for _, u := range []string{userA1, userA2, userB1, userB2, userShared} {
			_ = farmerstate.DeleteLeaderboardStatsForPlayer(u, LBContractExp)
			_ = farmerstate.DeleteAllLeaderboardOptInsForUserInGuild(guildA, u)
			_ = farmerstate.DeleteAllLeaderboardOptInsForUserInGuild(guildB, u)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	// Setup opt-ins
	AddPlayerOptInTypes(guildA, userA1, []string{LBContractExp})
	AddPlayerOptInTypes(guildA, userA2, []string{LBContractExp})
	AddPlayerOptInTypes(guildA, userShared, []string{LBContractExp})

	AddPlayerOptInTypes(guildB, userB1, []string{LBContractExp})
	AddPlayerOptInTypes(guildB, userB2, []string{LBContractExp})
	AddPlayerOptInTypes(guildB, userShared, []string{LBContractExp})

	day1 := "2026-01-01"
	day2 := "2026-01-02"
	day3 := "2026-01-03"

	// Day 1: Both guilds collected
	_ = farmerstate.UpsertLeaderboardStat(LBContractExp, userA1, "UserA1", day1, 100, sql.NullString{})
	_ = farmerstate.UpsertLeaderboardStat(LBContractExp, userA2, "UserA2", day1, 100, sql.NullString{})
	_ = farmerstate.UpsertLeaderboardStat(LBContractExp, userB1, "UserB1", day1, 100, sql.NullString{})
	_ = farmerstate.UpsertLeaderboardStat(LBContractExp, userB2, "UserB2", day1, 100, sql.NullString{})
	_ = farmerstate.UpsertLeaderboardStat(LBContractExp, userShared, "UserShared", day1, 100, sql.NullString{})

	// Day 2: Only Guild A collected
	_ = farmerstate.UpsertLeaderboardStat(LBContractExp, userA1, "UserA1", day2, 150, sql.NullString{})
	_ = farmerstate.UpsertLeaderboardStat(LBContractExp, userA2, "UserA2", day2, 150, sql.NullString{})
	_ = farmerstate.UpsertLeaderboardStat(LBContractExp, userShared, "UserShared", day2, 150, sql.NullString{})

	// Guild A's latest snap date should be Day 2
	latestA := GetLatestSnapDateForGuild(guildA, LBContractExp)
	if latestA != day2 {
		t.Fatalf("expected Guild A latest snap date to be %s, got %s", day2, latestA)
	}

	// Guild B's latest snap date should still be Day 1 (ignoring Day 2 where only the 1 shared user was collected)
	latestB := GetLatestSnapDateForGuild(guildB, LBContractExp)
	if latestB != day1 {
		t.Fatalf("expected Guild B latest snap date to be %s, got %s", day1, latestB)
	}

	// Guild A's previous snap date before Day 2 should be Day 1
	prevA := GetPreviousSnapDateForGuild(guildA, LBContractExp, day2)
	if prevA != day1 {
		t.Fatalf("expected Guild A previous snap date before %s to be %s, got %s", day2, day1, prevA)
	}

	// Day 3: Guild B runs a full collection
	_ = farmerstate.UpsertLeaderboardStat(LBContractExp, userB1, "UserB1", day3, 200, sql.NullString{})
	_ = farmerstate.UpsertLeaderboardStat(LBContractExp, userB2, "UserB2", day3, 200, sql.NullString{})
	_ = farmerstate.UpsertLeaderboardStat(LBContractExp, userShared, "UserShared", day3, 200, sql.NullString{})

	latestBAfter := GetLatestSnapDateForGuild(guildB, LBContractExp)
	if latestBAfter != day3 {
		t.Fatalf("expected Guild B latest snap date to be %s, got %s", day3, latestBAfter)
	}

	// Guild B's previous snap date before Day 3 should be Day 1, skipping Day 2
	prevB := GetPreviousSnapDateForGuild(guildB, LBContractExp, day3)
	if prevB != day1 {
		t.Fatalf("expected Guild B previous snap date before %s to be %s, got %s", day3, day1, prevB)
	}
}

func TestRunLeaderboardCollection_PrunesDepartedMembers(t *testing.T) {
	guildID := "test_guild_prune"
	activeUser := "test_user_active"
	departedUser := "test_user_departed"

	cleanup := func() {
		for _, u := range []string{activeUser, departedUser} {
			_ = farmerstate.DeleteAllLeaderboardOptInsForUserInGuild(guildID, u)
			_ = farmerstate.DeleteAllLeaderboardExclusionsForUserInGuild(guildID, u)
			farmerstate.RemoveGuildMembership(u, guildID)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	AddPlayerOptInTypes(guildID, activeUser, []string{LBContractExp})
	AddPlayerOptInTypes(guildID, departedUser, []string{LBContractExp})
	farmerstate.AddGuildMembership(activeUser, guildID)
	farmerstate.AddGuildMembership(departedUser, guildID)

	fakeClient := dctest.New().WithMember(guildID, activeUser, "ActiveUser", 0)
	// departedUser is intentionally absent from fakeClient.Members, so GuildMember returns ErrNotFound (404)

	// Run collection for guildID (dryRun = true so it doesn't post to Discord)
	RunLeaderboardCollection(fakeClient, true, guildID, "", "update", nil)

	// departedUser should be pruned from opt-in
	departedOptIns := GetPlayerOptInTypes(guildID, departedUser)
	if len(departedOptIns) != 0 {
		t.Fatalf("expected departed user to be pruned from opt-in, got %v", departedOptIns)
	}

	// departedUser should be removed from guild membership
	members := farmerstate.GetGuildMembers(guildID)
	if slices.Contains(members, departedUser) {
		t.Fatalf("expected departed user to be removed from guild membership, members: %v", members)
	}

	// activeUser should still be opted in
	activeOptIns := GetPlayerOptInTypes(guildID, activeUser)
	if len(activeOptIns) == 0 {
		t.Fatalf("expected active user to remain opted in")
	}
	if !slices.Contains(members, activeUser) {
		t.Fatalf("expected active user to remain in guild membership")
	}
}

func TestRunLeaderboardCollection_MultiGuildUserLeavesOneGuild(t *testing.T) {
	guild1 := "test_guild_one"
	guild2 := "test_guild_two"
	userMulti := "test_user_multi"
	snapDate := "2026-02-01"

	cleanup := func() {
		_ = farmerstate.DeleteAllLeaderboardOptInsForUserInGuild(guild1, userMulti)
		_ = farmerstate.DeleteAllLeaderboardOptInsForUserInGuild(guild2, userMulti)
		_ = farmerstate.DeleteAllLeaderboardExclusionsForUserInGuild(guild1, userMulti)
		_ = farmerstate.DeleteAllLeaderboardExclusionsForUserInGuild(guild2, userMulti)
		_ = farmerstate.DeleteLeaderboardStatsForPlayer(userMulti, LBContractExp)
		farmerstate.RemoveGuildMembership(userMulti, guild1)
		farmerstate.RemoveGuildMembership(userMulti, guild2)
	}
	cleanup()
	t.Cleanup(cleanup)

	// User is opted into both guilds
	AddPlayerOptInTypes(guild1, userMulti, []string{LBContractExp})
	AddPlayerOptInTypes(guild2, userMulti, []string{LBContractExp})
	farmerstate.AddGuildMembership(userMulti, guild1)
	farmerstate.AddGuildMembership(userMulti, guild2)

	// User has stats recorded
	_ = farmerstate.UpsertLeaderboardStat(LBContractExp, userMulti, "MultiPlayer", snapDate, 500, sql.NullString{})

	// fakeClient: user is still a member of guild2, but departed from guild1
	fakeClient := dctest.New().WithMember(guild2, userMulti, "MultiPlayer", 0)

	// Run collection for guild1
	RunLeaderboardCollection(fakeClient, true, guild1, "", "update", nil)

	// 1. In guild1: opt-ins and memberships must be removed
	guild1OptIns := GetPlayerOptInTypes(guild1, userMulti)
	if len(guild1OptIns) != 0 {
		t.Fatalf("expected user to be pruned from guild1 opt-in, got %v", guild1OptIns)
	}
	guild1Members := farmerstate.GetGuildMembers(guild1)
	if slices.Contains(guild1Members, userMulti) {
		t.Fatalf("expected user to be removed from guild1 membership")
	}

	// 2. In guild2: opt-ins and memberships must REMAIN intact
	guild2OptIns := GetPlayerOptInTypes(guild2, userMulti)
	if len(guild2OptIns) == 0 {
		t.Fatalf("expected user to remain opted into guild2")
	}
	guild2Members := farmerstate.GetGuildMembers(guild2)
	if !slices.Contains(guild2Members, userMulti) {
		t.Fatalf("expected user to remain a member of guild2")
	}

	// 3. Stats in leaderboard_stats must STAY intact
	stat := GetPriorStatForPlayer(LBContractExp, userMulti)
	if stat == nil || stat.Value != 500 {
		t.Fatalf("expected user's stats to remain intact, got %v", stat)
	}

	// 4. Guild2 leaderboard still shows the user
	guild2Rows := GetLeaderboardRows(LBContractExp, snapDate, guild2)
	foundInGuild2 := false
	for _, r := range guild2Rows {
		if r.Player == userMulti {
			foundInGuild2 = true
			break
		}
	}
	if !foundInGuild2 {
		t.Fatalf("expected user to be present on guild2 leaderboard")
	}

	// 5. Guild1 leaderboard does NOT show the user
	guild1Rows := GetLeaderboardRows(LBContractExp, snapDate, guild1)
	for _, r := range guild1Rows {
		if r.Player == userMulti {
			t.Fatalf("user should not be present on guild1 leaderboard after leaving")
		}
	}
}
