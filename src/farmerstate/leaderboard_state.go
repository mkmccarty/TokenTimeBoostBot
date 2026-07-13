package farmerstate

import (
	"database/sql"
)

// leaderboard_state.go — Wrapper functions for the leaderboard_stats and leaderboard_optin tables.
// These are thin adapters over the sqlc-generated Queries methods so that the
// leaderboard package can call into farmerstate without accessing internal state.

// ─── Opt-In Management ───────────────────────────────────────────────────────

// GetLeaderboardOptInUsers returns all Discord user IDs who have at least one opt-in in any guild.
func GetLeaderboardOptInUsers() ([]string, error) {
	if queries == nil {
		return nil, nil
	}
	return queries.GetLeaderboardOptInUsers(ctx)
}

// GetLeaderboardOptInsForUser returns all (guild_id, lb_type) pairs for a given user.
func GetLeaderboardOptInsForUser(userID string) ([]GetLeaderboardOptInsForUserRow, error) {
	if queries == nil {
		return nil, nil
	}
	return queries.GetLeaderboardOptInsForUser(ctx, userID)
}

// GetLeaderboardOptInsForGuild returns all (user_id, lb_type) pairs for a given guild.
func GetLeaderboardOptInsForGuild(guildID string) ([]GetLeaderboardOptInsForGuildRow, error) {
	if queries == nil {
		return nil, nil
	}
	return queries.GetLeaderboardOptInsForGuild(ctx, guildID)
}

// UpsertLeaderboardOptIn adds a leaderboard opt-in record for a user in a guild.
func UpsertLeaderboardOptIn(guildID, userID, lbType string) error {
	if queries == nil {
		return nil
	}
	return queries.UpsertLeaderboardOptIn(ctx, UpsertLeaderboardOptInParams{
		GuildID: guildID,
		UserID:  userID,
		LbType:  lbType,
	})
}

// DeleteLeaderboardOptIn removes a specific leaderboard opt-in record.
func DeleteLeaderboardOptIn(guildID, userID, lbType string) error {
	if queries == nil {
		return nil
	}
	return queries.DeleteLeaderboardOptIn(ctx, DeleteLeaderboardOptInParams{
		GuildID: guildID,
		UserID:  userID,
		LbType:  lbType,
	})
}

// DeleteAllLeaderboardOptInsForUserInGuild removes all opt-ins for a user in a specific guild.
func DeleteAllLeaderboardOptInsForUserInGuild(guildID, userID string) error {
	if queries == nil {
		return nil
	}
	return queries.DeleteAllLeaderboardOptInsForUserInGuild(ctx, DeleteAllLeaderboardOptInsForUserInGuildParams{
		GuildID: guildID,
		UserID:  userID,
	})
}

// ─── Exclusion Management ────────────────────────────────────────────────────

// GetLeaderboardExclusionsForUser returns all (guild_id, lb_type) exclusion pairs for a given user.
func GetLeaderboardExclusionsForUser(userID string) ([]GetLeaderboardExclusionsForUserRow, error) {
	if queries == nil {
		return nil, nil
	}
	return queries.GetLeaderboardExclusionsForUser(ctx, userID)
}

// UpsertLeaderboardExclusion adds a leaderboard exclusion record for a user in a guild.
func UpsertLeaderboardExclusion(guildID, userID, lbType string) error {
	if queries == nil {
		return nil
	}
	return queries.UpsertLeaderboardExclusion(ctx, UpsertLeaderboardExclusionParams{
		GuildID: guildID,
		UserID:  userID,
		LbType:  lbType,
	})
}

// DeleteLeaderboardExclusion removes a specific leaderboard exclusion record.
func DeleteLeaderboardExclusion(guildID, userID, lbType string) error {
	if queries == nil {
		return nil
	}
	return queries.DeleteLeaderboardExclusion(ctx, DeleteLeaderboardExclusionParams{
		GuildID: guildID,
		UserID:  userID,
		LbType:  lbType,
	})
}

// DeleteAllLeaderboardExclusionsForUserInGuild removes all exclusions for a user in a specific guild.
func DeleteAllLeaderboardExclusionsForUserInGuild(guildID, userID string) error {
	if queries == nil {
		return nil
	}
	return queries.DeleteAllLeaderboardExclusionsForUserInGuild(ctx, DeleteAllLeaderboardExclusionsForUserInGuildParams{
		GuildID: guildID,
		UserID:  userID,
	})
}

// ─── Stats Management ────────────────────────────────────────────────────────

// UpsertLeaderboardStat saves one leaderboard snapshot row (upsert on primary key).
func UpsertLeaderboardStat(lbType, player, gameName, snapDate string, value float64, details sql.NullString) error {
	if queries == nil {
		return nil
	}
	return queries.UpsertLeaderboardStat(ctx, UpsertLeaderboardStatParams{
		LbType:   lbType,
		Player:   player,
		GameName: gameName,
		SnapDate: snapDate,
		Value:    value,
		Details:  details,
	})
}

// GetLatestLeaderboardSnapDate returns the most recent snap_date for a lb_type.
func GetLatestLeaderboardSnapDate(lbType string) (string, error) {
	if queries == nil {
		return "", nil
	}
	return queries.GetLatestLeaderboardSnapDate(ctx, lbType)
}

// GetLeaderboardForSnapDate returns all rows for a lb_type, guild_id and snap_date, ordered by value DESC.
func GetLeaderboardForSnapDate(lbType, guildID, snapDate string) ([]GetLeaderboardForSnapDateRow, error) {
	if queries == nil {
		return nil, nil
	}
	return queries.GetLeaderboardForSnapDate(ctx, GetLeaderboardForSnapDateParams{
		LbType:   lbType,
		GuildID:  guildID,
		SnapDate: snapDate,
	})
}

// GetLeaderboardStatForPlayer returns the most recent stat for a player + lb_type.
func GetLeaderboardStatForPlayer(lbType, player string) (GetLeaderboardStatForPlayerRow, error) {
	if queries == nil {
		return GetLeaderboardStatForPlayerRow{}, nil
	}
	return queries.GetLeaderboardStatForPlayer(ctx, GetLeaderboardStatForPlayerParams{
		LbType: lbType,
		Player: player,
	})
}

// GetLeaderboardStatForPlayerAndSnapDate returns the stat for a player + lb_type + snap_date.
func GetLeaderboardStatForPlayerAndSnapDate(lbType, player, snapDate string) (GetLeaderboardStatForPlayerRow, error) {
	if queries == nil {
		return GetLeaderboardStatForPlayerRow{}, nil
	}
	// We can use the generated struct from sqlc: GetLeaderboardStatForPlayerAndSnapDateParams
	row, err := queries.GetLeaderboardStatForPlayerAndSnapDate(ctx, GetLeaderboardStatForPlayerAndSnapDateParams{
		LbType:   lbType,
		Player:   player,
		SnapDate: snapDate,
	})
	if err != nil {
		return GetLeaderboardStatForPlayerRow{}, err
	}
	return GetLeaderboardStatForPlayerRow(row), nil
}

// GetLeaderboardSnapDates returns all distinct snap_dates for a lb_type, newest first.
func GetLeaderboardSnapDates(lbType string) ([]string, error) {
	if queries == nil {
		return nil, nil
	}
	return queries.GetLeaderboardSnapDates(ctx, lbType)
}

// GetStatsForPlayer returns all leaderboard stats for a specific player across all types.
func GetStatsForPlayer(player string) ([]LeaderboardStat, error) {
	if queries == nil {
		return nil, nil
	}
	return queries.GetStatsForPlayer(ctx, player)
}

// GetStatsForPlayerInGuild returns all leaderboard stats for a player in a specific guild.
func GetStatsForPlayerInGuild(player, guildID string) ([]LeaderboardStat, error) {
	if queries == nil {
		return nil, nil
	}
	return queries.GetStatsForPlayerInGuild(ctx, GetStatsForPlayerInGuildParams{
		Player:  player,
		GuildID: guildID,
	})
}

// DeleteLeaderboardStatsForPlayer removes snapshots for a specific player and type.
func DeleteLeaderboardStatsForPlayer(player, lbType string) error {
	if queries == nil {
		return nil
	}
	return queries.DeleteLeaderboardStatsForPlayer(ctx, DeleteLeaderboardStatsForPlayerParams{
		Player: player,
		LbType: lbType,
	})
}

// DeleteAllLeaderboardStatsForPlayerInGuild is a no-op since leaderboard_stats is now global.
func DeleteAllLeaderboardStatsForPlayerInGuild(player, guildID string) error {
	if queries == nil {
		return nil
	}
	return queries.DeleteAllLeaderboardStatsForPlayerInGuild(ctx)
}

// DeleteAllLeaderboardStatsForPlayer removes every leaderboard stat for a player globally.
func DeleteAllLeaderboardStatsForPlayer(player string) error {
	if queries == nil {
		return nil
	}
	_, err := queries.db.ExecContext(ctx, "DELETE FROM leaderboard_stats WHERE player = ?", player)
	return err
}

// PruneOlderLeaderboardStatsForPlayer removes older leaderboard stats for a specific player and type.
func PruneOlderLeaderboardStatsForPlayer(lbType, player, keepSnapDate string) error {
	if queries == nil {
		return nil
	}
	return queries.PruneOlderLeaderboardStatsForPlayer(ctx, PruneOlderLeaderboardStatsForPlayerParams{
		LbType:   lbType,
		Player:   player,
		SnapDate: keepSnapDate,
	})
}
