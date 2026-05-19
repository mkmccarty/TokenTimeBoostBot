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

