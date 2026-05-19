-- name: InsertContract :exec
INSERT INTO contract_data (channelID, contractID, coopID, value)
VALUES (?, ?, ?, ?);

-- name: UpdateContract :execrows
UPDATE contract_data
SET value = ?
WHERE channelID = ? AND contractID = ? AND coopID = ?;

-- name: DeleteContractByChannel :exec
DELETE FROM contract_data
WHERE channelID = ? ;

-- name: CountContractsByChannel :one
SELECT COUNT(*) FROM contract_data
WHERE channelID = ? ;

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

-- name: GetContractRoles :many
SELECT contractID, role_name FROM contract_roles;

-- name: InsertContractRole :exec
INSERT INTO contract_roles (contractID, role_name) VALUES (?, ?)
ON CONFLICT(contractID, role_name) DO NOTHING;

-- name: DeleteAllContractRoles :exec
DELETE FROM contract_roles;

-- name: DeleteContractRoles :exec
DELETE FROM contract_roles WHERE contractID = ?;

-- name: GetContractComplaints :many
SELECT contractID, complaint FROM contract_complaints;

-- name: InsertContractComplaint :exec
INSERT INTO contract_complaints (contractID, complaint) VALUES (?, ?)
ON CONFLICT(contractID, complaint) DO NOTHING;

-- name: DeleteAllContractComplaints :exec
DELETE FROM contract_complaints;

-- name: DeleteContractComplaints :exec
DELETE FROM contract_complaints WHERE contractID = ?;

