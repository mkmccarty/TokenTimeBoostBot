CREATE TABLE farmer_state (
    id    text NOT NULL,
    key   text NOT NULL,
    value text, -- Store JSON data as TEXT
    PRIMARY KEY (id, key)
);

DELETE FROM farmer_state
WHERE rowid NOT IN (
    SELECT MIN(rowid)
    FROM farmer_state
    GROUP BY id, key
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_farmer_state_id_key
ON farmer_state (id, key);

CREATE TABLE IF NOT EXISTS farmer_guild_membership (
    user_id  TEXT NOT NULL,
    guild_id TEXT NOT NULL,
    PRIMARY KEY (user_id, guild_id)
);

CREATE TABLE IF NOT EXISTS custom_banners (
    user_id TEXT NOT NULL,
    guild_id TEXT NOT NULL,
    image_data BLOB NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (user_id, guild_id)
);

CREATE TABLE IF NOT EXISTS timers (
    id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    msg_id TEXT NOT NULL,
    reminder TIMESTAMP NOT NULL,
    message TEXT NOT NULL,
    duration INTEGER NOT NULL,
    original_channel_id TEXT NOT NULL,
    original_msg_id TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS suspect_missions (
    user_id TEXT NOT NULL,
    mission_id TEXT NOT NULL,
    ship INTEGER NOT NULL,
    status INTEGER NOT NULL,
    duration_type INTEGER NOT NULL,
    mission_type INTEGER NOT NULL,
    level INTEGER NOT NULL,
    capacity INTEGER NOT NULL,
    quality_bump REAL NOT NULL,
    target_artifact INTEGER NOT NULL,
    duration_seconds REAL NOT NULL,
    start_time_derived REAL NOT NULL,
    base_seconds REAL NOT NULL,
    event_multiplier REAL NOT NULL,
    PRIMARY KEY (user_id, mission_id)
);
