-- name: GetGuildState :one
SELECT * FROM guild_record
WHERE id = ? LIMIT 1;

-- name: InsertGuildState :one
INSERT INTO guild_record (id, value)
VALUES (?, ?)
RETURNING *;

-- name: UpdateGuildState :execrows
UPDATE guild_record
SET value = ?
WHERE id = ?;

-- name: GetAllGuildState :many
SELECT * FROM guild_record;

-- name: DeleteGuildState :exec
DELETE FROM guild_record
WHERE id = ?;

-- name: DeleteGuildRecords :exec
DELETE FROM guild_record
WHERE id = ?;

-- name: UpsertLeaderboardConfig :exec
-- Insert or update a guild leaderboard channel configuration.
INSERT INTO leaderboard_config (lb_type, guild_id, channel_id, message_ids)
VALUES (?, ?, ?, ?)
ON CONFLICT(lb_type, guild_id) DO UPDATE SET
    channel_id  = excluded.channel_id,
    message_ids = excluded.message_ids;

-- name: GetLeaderboardConfig :one
SELECT lb_type, guild_id, channel_id, message_ids
FROM leaderboard_config
WHERE lb_type = ? AND guild_id = ?;

-- name: GetAllLeaderboardConfigsForGuild :many
SELECT lb_type, guild_id, channel_id, message_ids
FROM leaderboard_config
WHERE guild_id = ?
ORDER BY lb_type;

-- name: GetAllLeaderboardConfigs :many
-- Returns every configured (lb_type, guild_id) pair - used by the weekly post task.
SELECT lb_type, guild_id, channel_id, message_ids
FROM leaderboard_config
ORDER BY lb_type, guild_id;

-- name: UpdateLeaderboardConfigMessageIDs :exec
-- Persist the message ID(s) written during the last post run.
UPDATE leaderboard_config
SET message_ids = ?
WHERE lb_type = ? AND guild_id = ?;

-- name: DeleteLeaderboardConfig :exec
DELETE FROM leaderboard_config
WHERE lb_type = ? AND guild_id = ?;
