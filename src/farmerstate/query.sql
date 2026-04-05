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