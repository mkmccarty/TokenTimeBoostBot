CREATE TABLE IF NOT EXISTS guild_record (
    id TEXT PRIMARY KEY,
    value TEXT
);

CREATE TABLE IF NOT EXISTS leaderboard_config (
    lb_type     TEXT NOT NULL,
    guild_id    TEXT NOT NULL,
    channel_id  TEXT NOT NULL,
    message_ids TEXT,  -- JSON array of Discord message IDs (retained for previous week)
    PRIMARY KEY (lb_type, guild_id)
);
