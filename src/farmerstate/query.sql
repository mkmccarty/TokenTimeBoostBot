-- name: GetLegacyFarmerstate :one
SELECT * FROM farmer_state
WHERE id = ? AND key = 'legacy' LIMIT 1;

-- name: InsertLegacyFarmerstate :one
INSERT INTO farmer_state (id, key, value)
VALUES (?, 'legacy', ?)
RETURNING *;

-- name: UpdateLegacyFarmerstate :execrows
UPDATE farmer_state
SET value = ?
WHERE id = ? AND key = 'legacy';

-- name: GetAllLegacyFarmerstate :many
SELECT * FROM farmer_state
WHERE key = 'legacy';


-- name: DeleteLegacyFarmerstate :exec
DELETE FROM farmer_state
WHERE id = ? AND key = 'legacy';

-- name: DeleteFarmerRecord :exec
DELETE FROM farmer_state
WHERE id = ?;

-- name: DeleteFarmerLegacyRecords :exec
DELETE FROM farmer_state
WHERE id = ? AND key = 'legacy';

-- name: GetEiIgnsByMiscString :many
SELECT json_extract(value, '$.MiscSettingsString.ei_ign') AS ei_ign
FROM farmer_state
WHERE json_extract(value, '$.MiscSettingsString.' || ?) = ?
  AND json_extract(value, '$.MiscSettingsString.ei_ign') IS NOT NULL;

-- name: GetIdsByMiscString :many
SELECT id
FROM farmer_state
WHERE json_extract(value, '$.MiscSettingsString.' || ?) = ?;

-- name: GetUserIdFromEiIgn :one
SELECT
    id
    --json_extract(value, '$.MiscSettingsString.ei_ign') AS ei_ign
FROM
    farmer_state
WHERE
    -- Exclude records where the extracted value is NULL
    json_extract(value, '$.MiscSettingsString.ei_ign') = ? LIMIT 1;


-- name: ClearExtraLegacyRecords :exec
	DELETE FROM farmer_state
	WHERE rowid NOT IN (
	    SELECT MIN(rowid)
	    FROM farmer_state
	    GROUP BY id, key
    );

-- name: AddGuildMembership :execrows
INSERT OR IGNORE INTO farmer_guild_membership (user_id, guild_id) 
VALUES (?, ?);

-- name: RemoveGuildMembership :exec
DELETE FROM farmer_guild_membership 
WHERE user_id = ? AND guild_id = ?;

-- name: GetGuildMembers :many
SELECT user_id FROM farmer_guild_membership 
WHERE guild_id = ?;

-- name: GetUserGuilds :many
SELECT guild_id FROM farmer_guild_membership 
WHERE user_id = ?;

-- name: GetEiIgnsByGuild :many
SELECT json_extract(fs.value, '$.MiscSettingsString.ei_ign') AS ei_ign
FROM farmer_guild_membership fgm
JOIN farmer_state fs ON fs.id = fgm.user_id AND fs.key = 'legacy'
WHERE fgm.guild_id = ?
  AND json_extract(fs.value, '$.MiscSettingsString.ei_ign') IS NOT NULL;

-- name: UpsertCustomBanner :exec
INSERT INTO custom_banners (user_id, guild_id, image_data) 
VALUES (?, ?, ?)
ON CONFLICT(user_id, guild_id) DO UPDATE SET 
	image_data = excluded.image_data,
	updated_at = CURRENT_TIMESTAMP;

-- name: DeleteCustomBanner :exec
DELETE FROM custom_banners WHERE user_id = ? AND guild_id = ?;

-- name: GetCustomBanner :one
SELECT image_data, updated_at FROM custom_banners WHERE user_id = ? AND guild_id = ?;

-- name: GetTimers :many
SELECT id, user_id, channel_id, msg_id, reminder, message, duration, original_channel_id, original_msg_id, active FROM timers;

-- name: InsertTimer :exec
INSERT INTO timers (id, user_id, channel_id, msg_id, reminder, message, duration, original_channel_id, original_msg_id, active)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateTimerState :exec
UPDATE timers SET active = ? WHERE id = ?;

-- name: UpdateTimerMsg :exec
UPDATE timers SET channel_id = ?, msg_id = ? WHERE id = ?;

-- name: DeleteTimer :exec
DELETE FROM timers WHERE id = ?;

-- name: DeleteInactiveTimers :exec
DELETE FROM timers WHERE active = 0;

-- name: InsertSuspectMission :exec
INSERT OR IGNORE INTO suspect_missions (
    user_id, mission_id, ship, status, duration_type, mission_type,
    level, capacity, quality_bump, target_artifact, duration_seconds,
    start_time_derived, base_seconds, event_multiplier
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
);

-- name: GetSuspectMissions :many
SELECT * FROM suspect_missions WHERE user_id = ?;

-- name: GetLeaderboardOptInUsers :many
-- Returns all Discord user IDs who have at least one opt-in in any guild.
SELECT DISTINCT user_id FROM leaderboard_optin;

-- name: GetLeaderboardOptInsForUser :many
-- Returns all (guild_id, lb_type) pairs for a given user.
SELECT guild_id, lb_type FROM leaderboard_optin
WHERE user_id = ?;

-- name: GetLeaderboardOptInsForGuild :many
-- Returns all (user_id, lb_type) pairs for a given guild.
SELECT user_id, lb_type FROM leaderboard_optin
WHERE guild_id = ?;

-- name: UpsertLeaderboardOptIn :exec
INSERT INTO leaderboard_optin (guild_id, user_id, lb_type)
VALUES (?, ?, ?)
ON CONFLICT(guild_id, user_id, lb_type) DO NOTHING;

-- name: DeleteLeaderboardOptIn :exec
DELETE FROM leaderboard_optin
WHERE guild_id = ? AND user_id = ? AND lb_type = ?;

-- name: DeleteAllLeaderboardOptInsForUserInGuild :exec
DELETE FROM leaderboard_optin
WHERE guild_id = ? AND user_id = ?;

-- name: UpsertLeaderboardStat :exec
-- Inserts or replaces a leaderboard snapshot for (lb_type, player, snap_date).
INSERT INTO leaderboard_stats (lb_type, player, game_name, snap_date, value, details)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(lb_type, player, snap_date) DO UPDATE SET
    game_name = excluded.game_name,
    value     = excluded.value,
    details   = excluded.details;

-- name: GetLatestLeaderboardSnapDate :one
-- Returns the most recent snap_date recorded for a given lb_type.
SELECT snap_date FROM leaderboard_stats
WHERE lb_type = ?
ORDER BY snap_date DESC
LIMIT 1;

-- name: GetLeaderboardForSnapDate :many
-- Returns all rows for a given lb_type, guild_id, and snap_date, ordered by value descending.
SELECT DISTINCT s.player, s.game_name, s.value, s.details
FROM leaderboard_stats s
JOIN leaderboard_optin o ON s.player = o.user_id AND (o.lb_type = s.lb_type OR o.lb_type = 'all')
LEFT JOIN leaderboard_exclusion excl ON s.player = excl.user_id AND excl.guild_id = o.guild_id AND excl.lb_type = s.lb_type
WHERE s.lb_type = ? AND o.guild_id = ? AND s.snap_date = ?
  AND excl.user_id IS NULL
ORDER BY s.value DESC;

-- name: GetLeaderboardStatForPlayer :one
-- Returns the most recent stat row for a player + lb_type.
SELECT player, game_name, snap_date, value, details
FROM leaderboard_stats
WHERE lb_type = ? AND player = ?
ORDER BY snap_date DESC
LIMIT 1;

-- name: GetLeaderboardSnapDates :many
-- Returns all distinct snap_dates for a given lb_type, newest first.
SELECT DISTINCT snap_date FROM leaderboard_stats
WHERE lb_type = ?
ORDER BY snap_date DESC;

-- name: GetStatsForPlayer :many
-- Returns all leaderboard stats for a specific player across all types, newest first.
SELECT lb_type, player, game_name, snap_date, value, details
FROM leaderboard_stats
WHERE player = ?
ORDER BY lb_type ASC, snap_date DESC;

-- name: GetStatsForPlayerInGuild :many
-- Returns all leaderboard stats for a specific player in a specific guild.
SELECT DISTINCT s.lb_type, s.player, s.game_name, s.snap_date, s.value, s.details
FROM leaderboard_stats s
JOIN leaderboard_optin o ON s.player = o.user_id AND (o.lb_type = s.lb_type OR o.lb_type = 'all')
LEFT JOIN leaderboard_exclusion excl ON s.player = excl.user_id AND excl.guild_id = o.guild_id AND excl.lb_type = s.lb_type
WHERE s.player = ? AND o.guild_id = ?
  AND excl.user_id IS NULL
ORDER BY s.lb_type ASC, s.snap_date DESC;

-- name: DeleteLeaderboardStatsForPlayer :exec
DELETE FROM leaderboard_stats
WHERE player = ? AND lb_type = ?;

-- name: DeleteAllLeaderboardStatsForPlayerInGuild :exec
-- No-op since leaderboard_stats is now global.
SELECT 1;

-- name: GetLeaderboardExclusionsForUser :many
-- Returns all (guild_id, lb_type) exclusion pairs for a given user.
SELECT guild_id, lb_type FROM leaderboard_exclusion
WHERE user_id = ?;

-- name: UpsertLeaderboardExclusion :exec
INSERT INTO leaderboard_exclusion (guild_id, user_id, lb_type)
VALUES (?, ?, ?)
ON CONFLICT(guild_id, user_id, lb_type) DO NOTHING;

-- name: DeleteLeaderboardExclusion :exec
DELETE FROM leaderboard_exclusion
WHERE guild_id = ? AND user_id = ? AND lb_type = ?;

-- name: DeleteAllLeaderboardExclusionsForUserInGuild :exec
DELETE FROM leaderboard_exclusion
WHERE guild_id = ? AND user_id = ?;
