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


/*
-- The following SQL statements are used to identify and delete entries in the `farmer_state` table that have IDs matching the specified patterns.

SELECT id
FROM farmer_state 
WHERE 
    -- Isolate the 'left' part of the ID
    SUBSTR(id, 1, INSTR(id, '_') - 1) IN (
        'brisk', 'calm', 'clever', 'cosmic', 'crisp', 'daring', 'gentle', 'lucky', 'mellow', 'nimble',
        'quiet', 'rapid', 'savvy', 'steady', 'swift', 'vivid', 'witty', 'bold', 'bright', 'chill'
    )
AND 
    -- Isolate the 'right' part of the ID
    SUBSTR(id, INSTR(id, '_') + 1) IN (
        'acorn', 'anchor', 'aster', 'beacon', 'bison', 'comet', 'drifter', 'falcon', 'harbor', 'meadow',
        'nebula', 'otter', 'ranger', 'rocket', 'sailor', 'sprout', 'thunder', 'valley', 'voyager', 'zephyr'
    );

DELETE FROM farmer_state 
WHERE 
    SUBSTR(id, 1, INSTR(id, '_') - 1) IN (
        'brisk', 'calm', 'clever', 'cosmic', 'crisp', 'daring', 'gentle', 'lucky', 'mellow', 'nimble',
        'quiet', 'rapid', 'savvy', 'steady', 'swift', 'vivid', 'witty', 'bold', 'bright', 'chill'
    )
AND 
    SUBSTR(id, INSTR(id, '_') + 1) IN (
        'acorn', 'anchor', 'aster', 'beacon', 'bison', 'comet', 'drifter', 'falcon', 'harbor', 'meadow',
        'nebula', 'otter', 'ranger', 'rocket', 'sailor', 'sprout', 'thunder', 'valley', 'voyager', 'zephyr'
    );

*/