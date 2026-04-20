package boost

import (
	"context"
	"database/sql"
	_ "embed" // This is used to embed the schema.sql file
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/peterbourgon/diskv/v3"
)

var dataStore *diskv.Diskv

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

func sqliteInit() {
	db, _ := sql.Open("sqlite", "ttbb-data/ContractData.sqlite?_busy_timeout=5000")
	db.SetMaxOpenConns(1)

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

func initDataStore() {

	// dataStore to initialize a new diskv store, rooted at "my-data-dir", with a 1MB cache.
	dataStore = diskv.New(diskv.Options{
		BasePath:          "ttbb-data",
		AdvancedTransform: AdvancedTransform,
		InverseTransform:  InverseTransform,
		CacheSizeMax:      1024 * 1024,
	})
}

// AdvancedTransform for storing KV pairs
func AdvancedTransform(key string) *diskv.PathKey {
	path := strings.Split(key, "/")
	last := len(path) - 1
	return &diskv.PathKey{
		Path:     path[:last],
		FileName: path[last] + ".json",
	}
}

// InverseTransform for storing KV pairs
func InverseTransform(pathKey *diskv.PathKey) (key string) {
	txt := pathKey.FileName[len(pathKey.FileName)-4:]
	if txt != ".json" {
		panic("Invalid file found in storage folder!")
	}
	return strings.Join(pathKey.Path, "/") + pathKey.FileName[:len(pathKey.FileName)-4]
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
