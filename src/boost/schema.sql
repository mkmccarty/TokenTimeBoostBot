CREATE TABLE IF NOT EXISTS contract_data (
    channelID   text PRIMARY KEY NOT NULL,
    contractID  text NOT NULL,
    coopID      text NOT NULL,
    value       text -- Store JSON data as TEXT
);
