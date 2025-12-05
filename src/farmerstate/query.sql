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