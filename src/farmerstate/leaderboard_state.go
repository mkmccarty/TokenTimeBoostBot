package farmerstate

import (
	"database/sql"
)

// leaderboard_state.go — Wrapper functions for the leaderboard_stats table.
// These are thin adapters over the sqlc-generated Queries methods so that the
// leaderboard package can call into farmerstate without accessing internal state.

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

// GetLeaderboardForSnapDate returns all rows for a lb_type and snap_date, ordered by value DESC.
func GetLeaderboardForSnapDate(lbType, snapDate string) ([]GetLeaderboardForSnapDateRow, error) {
	if queries == nil {
		return nil, nil
	}
	return queries.GetLeaderboardForSnapDate(ctx, GetLeaderboardForSnapDateParams{
		LbType:   lbType,
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

// GetLeaderboardOptInUsers returns all Discord user IDs with a non-empty leaderboard_optin.
func GetLeaderboardOptInUsers() ([]string, error) {
	if queries == nil {
		return nil, nil
	}
	return queries.GetLeaderboardOptInUsers(ctx)
}

// GetLeaderboardSnapDates returns all distinct snap_dates for a lb_type, newest first.
func GetLeaderboardSnapDates(lbType string) ([]string, error) {
	if queries == nil {
		return nil, nil
	}
	return queries.GetLeaderboardSnapDates(ctx, lbType)
}

// GetStatsForPlayer returns all leaderboard stats for a specific player across all types, newest first.
func GetStatsForPlayer(player string) ([]LeaderboardStat, error) {
	if queries == nil {
		return nil, nil
	}
	return queries.GetStatsForPlayer(ctx, player)
}
