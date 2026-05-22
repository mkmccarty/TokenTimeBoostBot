package boost

import (
	"context"
	"database/sql"
	_ "embed" // This is used to embed the schema.sql file
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

var ctx = context.Background()

type dbSaveRequest struct {
	contractID string
	coopID     string
	jsonStr    string
}

var (
	pendingSaves = make(map[string]dbSaveRequest)
	saveMutex    sync.Mutex
)

const contractDataDedupeSQL = `
DELETE FROM contract_data
WHERE rowid NOT IN (
	SELECT MAX(rowid)
	FROM contract_data
	GROUP BY channelID
);
`

const contractDataUniqueIndexSQL = `
CREATE UNIQUE INDEX IF NOT EXISTS idx_contract_data_channelid
ON contract_data(channelID);
`

const contractDataUpsertSQL = `
INSERT INTO contract_data (channelID, contractID, coopID, value)
VALUES (?, ?, ?, ?)
ON CONFLICT(channelID) DO UPDATE SET
	contractID = excluded.contractID,
	coopID = excluded.coopID,
	value = excluded.value;
`

const contractDataMigrationSQL = `
CREATE TABLE IF NOT EXISTS contract_data_new (
	channelID  text PRIMARY KEY NOT NULL,
	contractID text NOT NULL,
	coopID     text NOT NULL,
	value      text
);

INSERT INTO contract_data_new (channelID, contractID, coopID, value)
SELECT cd.channelID, cd.contractID, cd.coopID, cd.value
FROM contract_data cd
INNER JOIN (
	SELECT channelID, MAX(rowid) AS max_rowid
	FROM contract_data
	GROUP BY channelID
) dedupe ON dedupe.max_rowid = cd.rowid;

DROP TABLE contract_data;
ALTER TABLE contract_data_new RENAME TO contract_data;
`

//go:embed schema.sql
var ddl string
var queries *Queries
var dbConn *sql.DB

func sqliteInit() {
	db, _ := sql.Open("sqlite", "ttbb-data/ContractData.sqlite?_busy_timeout=5000")
	db.SetMaxOpenConns(1)
	dbConn = db

	result, err := db.ExecContext(ctx, ddl)
	if err != nil {
		log.Printf("Error initializing SQLite schema: %v", err)
	} else {
		fmt.Print(result)
	}

	if needsMigration, err := contractDataNeedsMigration(db); err != nil {
		log.Printf("Error checking contract_data schema migration status: %v", err)
	} else if needsMigration {
		if err := migrateContractDataSchema(db); err != nil {
			log.Printf("Error migrating contract_data schema: %v", err)
		}
	}

	if err := ensureContractDataUniqueness(db); err != nil {
		log.Printf("Error enforcing contract_data channel uniqueness: %v", err)
	}
	queries = New(db)
	performTransitionFromJSON(db)
	go dbFlusher()
}

func contractDataNeedsMigration(db *sql.DB) (bool, error) {
	rows, err := db.QueryContext(ctx, "PRAGMA table_info(contract_data)")
	if err != nil {
		return false, err
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			log.Printf("Error closing PRAGMA table_info rows: %v", cerr)
		}
	}()

	type pragmaColumn struct {
		cid       int
		name      string
		typeName  string
		notNull   int
		defaultV  sql.NullString
		pkOrdinal int
	}

	hasRows := false
	for rows.Next() {
		hasRows = true
		var c pragmaColumn
		if err := rows.Scan(&c.cid, &c.name, &c.typeName, &c.notNull, &c.defaultV, &c.pkOrdinal); err != nil {
			return false, err
		}
		if strings.EqualFold(c.name, "channelID") && c.pkOrdinal > 0 {
			return false, nil
		}
	}

	if err := rows.Err(); err != nil {
		return false, err
	}

	// If the table exists but channelID is not the PK, migration is needed.
	return hasRows, nil
}

func migrateContractDataSchema(db *sql.DB) error {
	// BEGIN IMMEDIATE prevents concurrent writers during table replacement.
	if _, err := db.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, contractDataMigrationSQL); err != nil {
		_, _ = db.ExecContext(ctx, "ROLLBACK")
		return err
	}

	if _, err := db.ExecContext(ctx, "COMMIT"); err != nil {
		_, _ = db.ExecContext(ctx, "ROLLBACK")
		return err
	}

	return nil
}

func ensureContractDataUniqueness(db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	if _, err = tx.ExecContext(ctx, contractDataDedupeSQL); err != nil {
		_ = tx.Rollback()
		return err
	}

	if _, err = tx.ExecContext(ctx, contractDataUniqueIndexSQL); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

// SaveAllData will save all contract data to disk
func SaveAllData() {
	log.Print("Saving contract data")
	saveData("")
	flushPendingSaves()
	farmerstate.FlushPendingSaves()
}

func saveData(contractHash string) {
	if contractHash != "" {
		contract := FindContractByHash(contractHash)
		if contract == nil {
			return
		}

		contract.LastSaveTime = time.Now()
		saveSqliteData(contract)
		return
	}

	byChannel := make(map[string]*Contract)
	ContractsMutex.RLock()
	for _, c := range Contracts {
		if c == nil || len(c.Location) == 0 {
			continue
		}

		channelID := c.Location[0].ChannelID
		existing := byChannel[channelID]
		if shouldReplaceChannelSaveCandidate(existing, c) {
			byChannel[channelID] = c
		}
	}
	ContractsMutex.RUnlock()

	now := time.Now()
	for _, c := range byChannel {
		c.LastSaveTime = now
		saveSqliteData(c)
	}
}

// shouldReplaceChannelSaveCandidate picks the best contract to persist for a
// channel when multiple contracts reference the same channel.
func shouldReplaceChannelSaveCandidate(current *Contract, candidate *Contract) bool {
	if candidate == nil {
		return false
	}
	if current == nil {
		return true
	}

	currentArchived := current.State == ContractStateArchive
	candidateArchived := candidate.State == ContractStateArchive

	// Prefer active contracts over archived contracts for the same channel.
	if currentArchived != candidateArchived {
		return !candidateArchived
	}

	if candidate.StartTime.After(current.StartTime) {
		return true
	}
	if current.StartTime.After(candidate.StartTime) {
		return false
	}

	if candidate.LastSaveTime.After(current.LastSaveTime) {
		return true
	}
	if current.LastSaveTime.After(candidate.LastSaveTime) {
		return false
	}

	// Final stable tie-breaker to avoid map-iteration nondeterminism.
	if candidate.ContractHash != current.ContractHash {
		return candidate.ContractHash < current.ContractHash
	}

	if candidate.ContractID != current.ContractID {
		return candidate.ContractID < current.ContractID
	}

	return candidate.CoopID < current.CoopID
}

func loadData() (map[string]*Contract, error) {
	// Ensure SQLite is initialized before use
	if queries == nil {
		sqliteInit()
	}
	c := make(map[string]*Contract)

	rows, err := queries.GetActiveContracts(ctx)
	if err == nil {
		for _, r := range rows {
			if r.Value.Valid {
				var contract Contract
				if err := json.Unmarshal([]byte(r.Value.String), &contract); err != nil {
					log.Printf("Error unmarshaling contract data from SQLite: %v", err)
					continue
				}

				backfilledRoleManagedByBot := false
				for _, loc := range contract.Location {
					if loc == nil || loc.RoleManagedByBot || strings.TrimSpace(loc.GuildContractRole.Name) == "" {
						continue
					}
					if IsRoleCreatedByBot(loc.GuildContractRole.Name) {
						loc.RoleManagedByBot = true
						backfilledRoleManagedByBot = true
					}
				}
				if backfilledRoleManagedByBot {
					saveSqliteData(&contract)
				}

				switch v := r.Contracthash.(type) {
				case string:
					c[v] = &contract
				case []byte:
					c[string(v)] = &contract
				default:
					log.Printf("Unsupported type for Contracthash: %T", v)
					continue
				}
			}
		}
		if len(c) > 0 {
			return c, nil
		}
	} else {
		log.Printf("Error reading active contracts from SQLite: %v", err)
	}

	return c, nil
}

/*
func readSqliteData(channelID string) (*Contract, error) {
	var contract ContractDatum
	var err error
	contract, err = queries.GetContractByChannelID(ctx, channelID)
	if err != nil {
		log.Printf("Error reading contract data from SQLite: %v", err)
		return nil, err
	}
	if contract.Value.Valid {
		var c Contract
		err = json.Unmarshal([]byte(contract.Value.String), &c)
		if err != nil {
			log.Printf("Error unmarshaling contract data from SQLite: %v", err)
			return nil, err
		}
		return &c, nil
	}
	return nil, nil
}
*/

// saveSqliteData saves a single piece of contract data to SQLite (for legacy support)
func saveSqliteData(contract *Contract) {
	if contract == nil || len(contract.Location) == 0 {
		return
	}

	// Save the contract data to SQLite
	contractJSON, err := json.Marshal(contract)
	if err != nil {
		log.Printf("Error marshaling contract data: %v", err)
		return
	}
	if queries == nil {
		sqliteInit()
	}

	channelID := contract.Location[0].ChannelID

	saveMutex.Lock()
	pendingSaves[channelID] = dbSaveRequest{
		contractID: contract.ContractID,
		coopID:     contract.CoopID,
		jsonStr:    string(contractJSON),
	}
	saveMutex.Unlock()
}

func dbFlusher() {
	ticker := time.NewTicker(2 * time.Second)
	for range ticker.C {
		flushPendingSaves()
	}
}

func flushPendingSaves() {
	saveMutex.Lock()
	if len(pendingSaves) == 0 {
		saveMutex.Unlock()
		return
	}
	toSave := pendingSaves
	pendingSaves = make(map[string]dbSaveRequest)
	saveMutex.Unlock()

	if queries == nil || queries.db == nil {
		return
	}

	for channelID, req := range toSave {
		if _, err := queries.db.ExecContext(ctx, contractDataUpsertSQL, channelID, req.contractID, req.coopID, sql.NullString{String: req.jsonStr, Valid: true}); err != nil {
			log.Printf("Error saving contract data to SQLite: %v", err)
		}
	}
}

/*
DELETE FROM contract_data
WHERE ROWID IN (
    -- 1. Use a CTE to rank the duplicate rows
    WITH RankedContracts AS (
        SELECT
            -- Select the implicit ROWID, which is unique for every row and indicates insertion order
            ROWID as row_id,
            channelID,
            contractID,
            coopID,
            -- Assign a rank to each entry within its unique group
            ROW_NUMBER() OVER (
                PARTITION BY
                    channelID,
                    contractID,
                    coopID
                -- Ordering by ROWID DESC ensures the most recently inserted row gets rank 1
                ORDER BY
                    ROWID DESC
            ) as row_rank
        FROM
            contract_data
    )
    -- 2. Select the ROWIDs of the rows to delete (those that are not the "last copy," i.e., rank > 1)
    SELECT row_id
    FROM RankedContracts
    WHERE row_rank > 1
);

*/

/*
-- Select to view the current state values for all contracts
SELECT
    channelID,
    contractID,
    coopID,
    json_extract(value, '$.State') AS state_value
FROM
    contract_data;

*/

func performTransitionFromJSON(db *sql.DB) {
	// Transition for roles
	rolesPath := "ttbb-data/ei-roles.json"
	if _, err := os.Stat(rolesPath); err == nil {
		log.Printf("Found legacy roles JSON file %s, importing to database...", rolesPath)
		data, err := os.ReadFile(rolesPath)
		if err == nil {
			var rolesMap map[string][]string
			if err := json.Unmarshal(data, &rolesMap); err == nil {
				tx, err := db.BeginTx(ctx, nil)
				if err == nil {
					txQueries := queries.WithTx(tx)
					for contractID, roles := range rolesMap {
						for _, rName := range roles {
							_ = txQueries.InsertContractRole(ctx, InsertContractRoleParams{
								Contractid: contractID,
								RoleName:   rName,
							})
						}
					}
					if err := tx.Commit(); err != nil {
						log.Printf("Failed to commit imported roles: %v", err)
					} else {
						log.Printf("Successfully imported roles for %d contracts", len(rolesMap))
						_ = os.Remove(rolesPath)
					}
				}
			} else {
				log.Printf("Failed to unmarshal legacy roles JSON: %v", err)
			}
		} else {
			log.Printf("Failed to read legacy roles JSON file: %v", err)
		}
	}

	// Transition for complaints
	complaintsPath := "ttbb-data/ei-complaints.json"
	if _, err := os.Stat(complaintsPath); err == nil {
		log.Printf("Found legacy complaints JSON file %s, importing to database...", complaintsPath)
		data, err := os.ReadFile(complaintsPath)
		if err == nil {
			var complaintsMap map[string][]string
			if err := json.Unmarshal(data, &complaintsMap); err == nil {
				tx, err := db.BeginTx(ctx, nil)
				if err == nil {
					txQueries := queries.WithTx(tx)
					for contractID, complaints := range complaintsMap {
						for _, complaint := range complaints {
							_ = txQueries.InsertContractComplaint(ctx, InsertContractComplaintParams{
								Contractid: contractID,
								Complaint:  complaint,
							})
						}
					}
					if err := tx.Commit(); err != nil {
						log.Printf("Failed to commit imported complaints: %v", err)
					} else {
						log.Printf("Successfully imported complaints for %d contracts", len(complaintsMap))
						_ = os.Remove(complaintsPath)
					}
				}
			} else {
				log.Printf("Failed to unmarshal legacy complaints JSON: %v", err)
			}
		} else {
			log.Printf("Failed to read legacy complaints JSON file: %v", err)
		}
	}
}

func splitPackedRoleNames(raw string) []string {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return nil
	}

	var parsed []string
	if strings.HasPrefix(cleaned, "[") && strings.HasSuffix(cleaned, "]") {
		if err := json.Unmarshal([]byte(cleaned), &parsed); err == nil && len(parsed) > 1 {
			return parsed
		}
	}

	if strings.Contains(cleaned, "\",\"") || strings.Contains(cleaned, "\", \"") {
		cleaned = strings.TrimPrefix(cleaned, "[")
		cleaned = strings.TrimSuffix(cleaned, "]")
		parts := strings.Split(cleaned, "\",\"")
		if len(parts) == 1 {
			parts = strings.Split(cleaned, "\", \"")
		}
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			p = strings.TrimPrefix(p, "\"")
			p = strings.TrimSuffix(p, "\"")
			p = strings.Trim(p, ", ")
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		if len(out) > 1 {
			return out
		}
	}

	if strings.Count(cleaned, ",") >= 2 {
		parts := strings.Split(cleaned, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			p = strings.TrimPrefix(p, "\"")
			p = strings.TrimSuffix(p, "\"")
			if p != "" {
				out = append(out, p)
			}
		}
		if len(out) > 1 {
			return out
		}
	}

	if strings.Contains(cleaned, "\n") {
		lines := strings.Split(cleaned, "\n")
		out := make([]string, 0, len(lines))
		for _, l := range lines {
			l = strings.TrimSpace(l)
			l = strings.TrimLeft(l, "0123456789.-*• ")
			l = strings.TrimSpace(l)
			if l != "" {
				out = append(out, l)
			}
		}
		if len(out) > 1 {
			return out
		}
	}

	return nil
}

// LoadRoleNames loads all contract roles from the database
func LoadRoleNames() (map[string][]string, error) {
	if queries == nil {
		sqliteInit()
	}
	rows, err := queries.GetContractRoles(ctx)
	if err != nil {
		return nil, err
	}
	res := make(map[string][]string)
	repairedContracts := make(map[string]bool)
	for _, row := range rows {
		if repaired := splitPackedRoleNames(row.RoleName); len(repaired) > 1 {
			res[row.Contractid] = append(res[row.Contractid], repaired...)
			repairedContracts[row.Contractid] = true
			log.Printf("LoadRoleNames: repaired packed role row for %s into %d role names", row.Contractid, len(repaired))
			continue
		}
		res[row.Contractid] = append(res[row.Contractid], row.RoleName)
	}

	if len(repairedContracts) > 0 {
		tx, txErr := dbConn.BeginTx(ctx, nil)
		if txErr != nil {
			log.Printf("LoadRoleNames: failed to begin repair transaction: %v", txErr)
		} else {
			txQueries := queries.WithTx(tx)
			repairFailed := false
			for contractID := range repairedContracts {
				if err := txQueries.DeleteContractRoles(ctx, contractID); err != nil {
					log.Printf("LoadRoleNames: failed to clear roles for %s during repair: %v", contractID, err)
					repairFailed = true
					break
				}
				for _, roleName := range res[contractID] {
					if err := txQueries.InsertContractRole(ctx, InsertContractRoleParams{
						Contractid: contractID,
						RoleName:   roleName,
					}); err != nil {
						log.Printf("LoadRoleNames: failed to write repaired role for %s: %v", contractID, err)
						repairFailed = true
						break
					}
				}
				if repairFailed {
					break
				}
			}

			if repairFailed {
				_ = tx.Rollback()
			} else if err := tx.Commit(); err != nil {
				log.Printf("LoadRoleNames: failed to commit repair transaction: %v", err)
			} else {
				log.Printf("LoadRoleNames: repaired %d contract role record sets", len(repairedContracts))
			}
		}
	}

	return res, nil
}

// SaveRoleNames saves contract roles to the database.
// It deletes existing roles for the given contract IDs and inserts the new ones in a transaction.
func SaveRoleNames(data map[string][]string) {
	if queries == nil {
		sqliteInit()
	}
	tx, err := dbConn.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("SaveRoleNames: failed to begin transaction: %v", err)
		return
	}
	txQueries := queries.WithTx(tx)
	for contractID, roles := range data {
		if err := txQueries.DeleteContractRoles(ctx, contractID); err != nil {
			_ = tx.Rollback()
			log.Printf("SaveRoleNames: failed to delete roles for contract %s: %v", contractID, err)
			return
		}
		for _, rName := range roles {
			err := txQueries.InsertContractRole(ctx, InsertContractRoleParams{
				Contractid: contractID,
				RoleName:   rName,
			})
			if err != nil {
				log.Printf("SaveRoleNames: failed to insert role %s for %s: %v", rName, contractID, err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		log.Printf("SaveRoleNames: failed to commit transaction: %v", err)
	}
}

var (
	thematicComplaintsMap map[string][]string
	thematicComplaintsMu  sync.RWMutex
)

func splitPackedComplaintList(raw string) []string {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return nil
	}

	if !strings.Contains(cleaned, "\",\"") && strings.Count(cleaned, "[player]") < 2 {
		return nil
	}

	cleaned = strings.TrimPrefix(cleaned, "[")
	cleaned = strings.TrimSuffix(cleaned, "]")

	parts := strings.Split(cleaned, "\",\"")
	if len(parts) == 1 {
		parts = strings.Split(cleaned, "\", \"")
	}
	if len(parts) <= 1 {
		return nil
	}

	out := make([]string, 0, len(parts))
	playerCount := 0
	for _, part := range parts {
		part = strings.TrimSpace(part)
		part = strings.TrimPrefix(part, "\"")
		part = strings.TrimSuffix(part, "\"")
		part = strings.Trim(part, ", ")
		part = strings.ReplaceAll(part, "\\\"", "\"")
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "[player]") {
			playerCount++
		}
		out = append(out, part)
	}

	if len(out) <= 1 || playerCount < 2 {
		return nil
	}

	return out
}

// LoadThematicComplaints loads all contract complaints from the database,
// using an in-memory cache to match previous behavior.
func LoadThematicComplaints() (map[string][]string, error) {
	thematicComplaintsMu.RLock()
	if thematicComplaintsMap != nil {
		defer thematicComplaintsMu.RUnlock()
		return thematicComplaintsMap, nil
	}
	thematicComplaintsMu.RUnlock()

	thematicComplaintsMu.Lock()
	defer thematicComplaintsMu.Unlock()
	// Check again in case it was loaded in between
	if thematicComplaintsMap != nil {
		return thematicComplaintsMap, nil
	}

	if queries == nil {
		sqliteInit()
	}

	rows, err := queries.GetContractComplaints(ctx)
	if err != nil {
		return nil, err
	}

	res := make(map[string][]string)
	repairedContracts := make(map[string]bool)
	for _, row := range rows {
		if repaired := splitPackedComplaintList(row.Complaint); len(repaired) > 1 {
			res[row.Contractid] = append(res[row.Contractid], repaired...)
			repairedContracts[row.Contractid] = true
			log.Printf("LoadThematicComplaints: repaired packed complaint row for %s into %d complaints", row.Contractid, len(repaired))
			continue
		}

		res[row.Contractid] = append(res[row.Contractid], row.Complaint)
	}

	if len(repairedContracts) > 0 {
		tx, txErr := dbConn.BeginTx(ctx, nil)
		if txErr != nil {
			log.Printf("LoadThematicComplaints: failed to begin repair transaction: %v", txErr)
		} else {
			txQueries := queries.WithTx(tx)
			repairFailed := false
			for contractID := range repairedContracts {
				if err := txQueries.DeleteContractComplaints(ctx, contractID); err != nil {
					log.Printf("LoadThematicComplaints: failed to clear complaints for %s during repair: %v", contractID, err)
					repairFailed = true
					break
				}
				for _, complaint := range res[contractID] {
					if err := txQueries.InsertContractComplaint(ctx, InsertContractComplaintParams{
						Contractid: contractID,
						Complaint:  complaint,
					}); err != nil {
						log.Printf("LoadThematicComplaints: failed to write repaired complaint for %s: %v", contractID, err)
						repairFailed = true
						break
					}
				}
				if repairFailed {
					break
				}
			}

			if repairFailed {
				_ = tx.Rollback()
			} else if err := tx.Commit(); err != nil {
				log.Printf("LoadThematicComplaints: failed to commit repair transaction: %v", err)
			} else {
				log.Printf("LoadThematicComplaints: repaired %d contract complaint record sets", len(repairedContracts))
			}
		}
	}
	thematicComplaintsMap = res
	return res, nil
}

// SaveThematicComplaints saves contract complaints to the database.
// It deletes existing complaints for the given contract IDs and inserts the new ones in a transaction.
func SaveThematicComplaints(data map[string][]string) error {
	thematicComplaintsMu.Lock()
	thematicComplaintsMap = nil // Invalidate cache so that next Load loads all complaints from DB
	thematicComplaintsMu.Unlock()

	if queries == nil {
		sqliteInit()
	}

	tx, err := dbConn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	txQueries := queries.WithTx(tx)

	for contractID, complaints := range data {
		if err := txQueries.DeleteContractComplaints(ctx, contractID); err != nil {
			_ = tx.Rollback()
			return err
		}
		for _, complaint := range complaints {
			err := txQueries.InsertContractComplaint(ctx, InsertContractComplaintParams{
				Contractid: contractID,
				Complaint:  complaint,
			})
			if err != nil {
				log.Printf("SaveThematicComplaints: failed to insert complaint %s for %s: %v", complaint, contractID, err)
			}
		}
	}

	return tx.Commit()
}
