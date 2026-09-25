CREATE TABLE IF NOT EXISTS contract_data (
    channelID   text PRIMARY KEY NOT NULL,
    contractID  text NOT NULL,
    coopID      text NOT NULL,
    value       text -- Store JSON data as TEXT
);

CREATE TABLE IF NOT EXISTS contract_roles (
    contractID  text NOT NULL,
    role_name   text NOT NULL,
    PRIMARY KEY (contractID, role_name)
);

CREATE TABLE IF NOT EXISTS contract_complaints (
    contractID  text NOT NULL,
    complaint   text NOT NULL,
    PRIMARY KEY (contractID, complaint)
);

CREATE TABLE IF NOT EXISTS missing_contracts (
    contractID text PRIMARY KEY NOT NULL,
    timestamp  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS custom_order_sessions (
    uuid          text PRIMARY KEY NOT NULL,
    contract_hash text NOT NULL,
    channel_id    text NOT NULL,
    user_id       text NOT NULL,
    lines         text NOT NULL,
    expires_at    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS define_order_sessions (
    uuid          text PRIMARY KEY NOT NULL,
    contract_hash text NOT NULL,
    channel_id    text NOT NULL,
    user_id       text NOT NULL,
    template_json text NOT NULL,
    is_saved      INTEGER NOT NULL,
    expires_at    INTEGER NOT NULL
);

