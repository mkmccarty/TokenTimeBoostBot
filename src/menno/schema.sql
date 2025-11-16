

CREATE TABLE data_timestamp (
    key       text NOT NULL,
    timestamp DATETIME NOT NULL
);  

CREATE TABLE data (
    ship_type_id INTEGER,
    ship_duration_type_id INTEGER,
    ship_level INTEGER,
    target_artifact_id INTEGER,
    artifact_type_id INTEGER,
    artifact_rarity_id INTEGER,
    artifact_tier INTEGER,
    total_drops INTEGER,
    mission_type INTEGER
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_data_unique ON data(ship_type_id, ship_duration_type_id, ship_level, target_artifact_id, artifact_type_id, artifact_rarity_id, artifact_tier, mission_type);