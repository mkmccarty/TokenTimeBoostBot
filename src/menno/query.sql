-- name: CreateTimestamp :exec
INSERT INTO data_timestamp (key, timestamp) VALUES ('menno', CURRENT_TIMESTAMP);

-- name: UpdateTimestamp :exec
UPDATE data_timestamp
SET timestamp = CURRENT_TIMESTAMP
WHERE key = 'menno';

-- name: GetTimestamp :one
SELECT timestamp FROM data_timestamp
WHERE key = 'menno' LIMIT 1;

-- name: InsertData :exec
INSERT INTO data (
    ship_type_id,
    ship_duration_type_id,
    ship_level,
    target_artifact_id,
    artifact_type_id,
    artifact_rarity_id,
    artifact_tier,
    total_drops,
    mission_type
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateData :exec
UPDATE data
SET total_drops = ?
WHERE
    ship_type_id = ? AND
    ship_duration_type_id = ? AND
    ship_level = ? AND
    target_artifact_id = ? AND
    artifact_type_id = ? AND
    artifact_rarity_id = ? AND
    artifact_tier = ? AND
    mission_type = ?;

-- name: DeleteData :execrows
DELETE FROM data
WHERE   
    ship_type_id = ? AND
    ship_duration_type_id = ? AND
    ship_level = ? AND
    target_artifact_id = ? AND
    artifact_type_id = ? AND
    artifact_rarity_id = ? AND
    artifact_tier = ? AND
    mission_type = ?;

-- name: GetDrops :many
SELECT
    d.total_drops AS total_drops,
    (
        SELECT COALESCE(SUM(d2.total_drops), 0)
        FROM data d2
        WHERE
            d2.ship_type_id = d.ship_type_id AND
            d2.ship_duration_type_id = d.ship_duration_type_id AND
            d2.ship_level = d.ship_level AND
            d2.target_artifact_id = d.target_artifact_id
    ) AS all_drops_value,
    d.*
FROM data d
WHERE
    d.ship_type_id = ? AND
    d.ship_duration_type_id = ? AND
    d.ship_level = ? AND
    d.artifact_type_id = ?
ORDER BY total_drops DESC;