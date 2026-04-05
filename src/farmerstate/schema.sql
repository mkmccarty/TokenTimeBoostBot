CREATE TABLE farmer_state (
    id   text NOT NULL,
    key text    NOT NULL,
    value  text -- Store JSON data as TEXT
);

CREATE TABLE IF NOT EXISTS farmer_guild_membership (
    user_id  TEXT NOT NULL,
    guild_id TEXT NOT NULL,
    PRIMARY KEY (user_id, guild_id)
);
