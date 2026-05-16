package guildstate

import (
	"context"
	"database/sql"
	"log"
)

// leaderboard_state.go — Wrapper functions for the leaderboard_config table.
// These are thin adapters over the sqlc-generated Queries methods so that the
// leaderboard package can call into guildstate without accessing internal state.

// UpsertLeaderboardConfig inserts or updates a guild leaderboard config row.
func UpsertLeaderboardConfig(lbType, guildID, channelID, messageIDsJSON string) error {
	if queries == nil {
		sqliteInit()
	}
	return queries.UpsertLeaderboardConfig(context.Background(), UpsertLeaderboardConfigParams{
		LbType:     lbType,
		GuildID:    guildID,
		ChannelID:  channelID,
		MessageIds: sql.NullString{String: messageIDsJSON, Valid: messageIDsJSON != ""},
	})
}

// GetLeaderboardConfig retrieves a single leaderboard config row.
func GetLeaderboardConfig(lbType, guildID string) (LeaderboardConfig, error) {
	if queries == nil {
		sqliteInit()
	}
	return queries.GetLeaderboardConfig(context.Background(), GetLeaderboardConfigParams{
		LbType:  lbType,
		GuildID: guildID,
	})
}

// GetAllLeaderboardConfigsForGuild returns all leaderboard configs for a guild.
func GetAllLeaderboardConfigsForGuild(guildID string) ([]LeaderboardConfig, error) {
	if queries == nil {
		sqliteInit()
	}
	return queries.GetAllLeaderboardConfigsForGuild(context.Background(), guildID)
}

// GetAllLeaderboardConfigs returns every configured (lb_type, guild_id) pair.
func GetAllLeaderboardConfigs() ([]LeaderboardConfig, error) {
	if queries == nil {
		sqliteInit()
	}
	return queries.GetAllLeaderboardConfigs(context.Background())
}

// UpdateLeaderboardConfigMessageIDs persists message IDs after a post run.
func UpdateLeaderboardConfigMessageIDs(messageIDsJSON, lbType, guildID string) error {
	if queries == nil {
		sqliteInit()
	}
	return queries.UpdateLeaderboardConfigMessageIDs(context.Background(), UpdateLeaderboardConfigMessageIDsParams{
		MessageIds: sql.NullString{String: messageIDsJSON, Valid: messageIDsJSON != ""},
		LbType:     lbType,
		GuildID:    guildID,
	})
}

// DeleteLeaderboardConfig removes a leaderboard config row.
func DeleteLeaderboardConfig(lbType, guildID string) error {
	if queries == nil {
		sqliteInit()
	}
	if err := queries.DeleteLeaderboardConfig(context.Background(), DeleteLeaderboardConfigParams{
		LbType:  lbType,
		GuildID: guildID,
	}); err != nil {
		log.Printf("guildstate: DeleteLeaderboardConfig %s/%s: %v", lbType, guildID, err)
		return err
	}
	return nil
}
