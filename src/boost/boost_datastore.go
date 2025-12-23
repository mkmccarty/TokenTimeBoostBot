package boost

import (
	"context"
	"database/sql"
	_ "embed" // This is used to embed the schema.sql file
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/peterbourgon/diskv/v3"
)

var dataStore *diskv.Diskv

var ctx = context.Background()

//go:embed schema.sql
var ddl string
var queries *Queries

func sqliteInit() {
	db, _ := sql.Open("sqlite", "ttbb-data/ContractData.sqlite?_busy_timeout=5000")

	result, err := db.ExecContext(ctx, ddl)
	if err == nil {
		fmt.Print(result)
	}
	queries = New(db)
}

// SaveAllData will save all contract data to disk
func SaveAllData() {
	log.Print("Saving contract data")
	saveData("")
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

		if contract.State == ContractStateSignup {
			if time.Since(contract.LastSaveTime) < 2*time.Minute && contract.CoopSize < len(contract.Boosters) {
				// Only save signup contracts every 2 minutes during signup
				return
			}
		} else {
			if time.Since(contract.LastSaveTime) < 15*time.Second {
				// Only save non-signup contracts every 15 seconds
				return
			}
		}

		saveSqliteData(contract)
		return
	}

	for _, c := range Contracts {
		saveSqliteData(c)
	}

	// Legacy disk store backup
	//b, _ := json.Marshal(Contracts)
	//_ = dataStore.Write("EggsBackup", b)
}

/*
func saveEndData(c *Contract) error {
	//diskmutex.Lock()
	var saveName = fmt.Sprintf("%s/%s", c.ContractID, c.CoopID)
	b, _ := json.Marshal(c)
	_ = dataStore.Write(saveName, b)
	//diskmutex.Unlock()
	return nil
}
*/

func loadData() (map[string]*Contract, error) {
	// Ensure SQLite is initialized before use
	if queries == nil {
		sqliteInit()
	}
	c := make(map[string]*Contract)

	//if the file ttbb-data/EggsBackup.json exists
	if dataStore.Has("EggsBackup") {

		b, err := dataStore.Read("EggsBackup")
		if err != nil {
			return c, err
		}
		if err := json.Unmarshal(b, &c); err != nil {
			return c, err
		}

		for _, v := range c {
			saveSqliteData(v)
		}

		_ = dataStore.Erase("EggsBackup")
	}

	// Try to load from SQLite first
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
	// Save the contract data to SQLite
	contractJSON, err := json.Marshal(contract)
	if err != nil {
		log.Printf("Error marshaling contract data: %v", err)
		return
	}
	var rows int64
	if queries == nil {
		sqliteInit()
	}

	/*
		_ = queries.DeleteContract(ctx, DeleteContractParams{
			Channelid:  contract.Location[0].ChannelID,
			Contractid: contract.ContractID,
			Coopid:     contract.CoopID,
		})
	*/
	rows, _ = queries.UpdateContract(ctx, UpdateContractParams{
		Channelid:  contract.Location[0].ChannelID,
		Contractid: contract.ContractID,
		Coopid:     contract.CoopID,
		Value:      sql.NullString{String: string(contractJSON), Valid: true},
	})
	if rows == 0 {
		// Record doesn't exist, insert it
		err = queries.InsertContract(ctx, InsertContractParams{
			Channelid:  contract.Location[0].ChannelID,
			Contractid: contract.ContractID,
			Coopid:     contract.CoopID,
			Value:      sql.NullString{String: string(contractJSON), Valid: true},
		})
		if err != nil {
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
