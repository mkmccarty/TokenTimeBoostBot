-- name: InsertContract :exec
INSERT INTO contract_data (channelID, contractID, coopID, value)
VALUES (?, ?, ?, ?);

-- name: UpdateContract :execrows
UPDATE contract_data
SET value = ?
WHERE channelID = ? AND contractID = ? AND coopID = ?;

-- name: DeleteContract :exec
DELETE FROM contract_data
WHERE channelID = ? AND contractID = ? AND coopID = ?;

-- name: UpdateContractCoopID :exec
UPDATE contract_data
SET coopID = ?
WHERE channelID = ? AND contractID = ? AND coopID = ?;

-- name: GetContractByChannelID :one
SELECT * FROM contract_data
WHERE channelID = ?;

-- name: GetActiveContracts :many
SELECT value->>'ContractHash' AS ContractHash,value FROM contract_data WHERE value->>'State' != 4;

-- name: UpdateContractState :exec
UPDATE contract_data
SET value = json_replace(value, '$.State', ?)
WHERE channelID = ? AND contractID = ? AND coopID = ?;