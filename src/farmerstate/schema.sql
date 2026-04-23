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
