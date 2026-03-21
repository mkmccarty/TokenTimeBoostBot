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
