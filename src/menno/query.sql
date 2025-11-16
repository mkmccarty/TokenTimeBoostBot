-- name: CreateTimestamp :exec
INSERT INTO data_timestamp (key, timestamp) VALUES ('menno', CURRENT_TIMESTAMP);

-- name: UpdateTimestamp :exec
UPDATE data_timestamp
SET timestamp = CURRENT_TIMESTAMP
WHERE key = 'menno';

-- name: GetTimestamp :one
SELECT timestamp FROM data_timestamp
WHERE key = 'menno' LIMIT 1;

-- name: InsertData :execrows
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

-- name: UpdateData :execrows
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