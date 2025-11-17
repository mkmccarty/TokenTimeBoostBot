

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

