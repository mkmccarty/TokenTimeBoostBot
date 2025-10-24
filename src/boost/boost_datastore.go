package boost

import (
	"context"
	"database/sql"
	_ "embed" // This is used to embed the schema.sql file
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/peterbourgon/diskv/v3"
)

var dataStore *diskv.Diskv

var ctx = context.Background()

//go:embed schema.sql
var ddl string
var queries *Queries

func sqliteInit() {
	db, _ := sql.Open("sqlite", "ttbb-data/ContractData.sqlite")

	result, err := db.ExecContext(ctx, ddl)
	if err != nil {
		log.Printf("We have an error: %v", err)
	} else {
		fmt.Print(result)
	}
	queries = New(db)
}

// SaveAllData will remove a token from the Contracts
func SaveAllData() {
	log.Print("Saving contact data")
	saveData(Contracts)
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

func saveData(c map[string]*Contract) {
	b, _ := json.Marshal(c)
	_ = dataStore.Write("EggsBackup", b)
}

func saveEndData(c *Contract) error {
	//diskmutex.Lock()
	var saveName = fmt.Sprintf("%s/%s", c.ContractID, c.CoopID)
	b, _ := json.Marshal(c)
	_ = dataStore.Write(saveName, b)
	//diskmutex.Unlock()
	return nil
}

func loadData() (map[string]*Contract, error) {
	var c map[string]*Contract
	b, err := dataStore.Read("EggsBackup")
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	if err != nil {
		return c, err
	}
	if queries == nil {
		sqliteInit()
	}

	for _, v := range c {
		saveSqliteData(v)
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
	var updateErr error
	rows, updateErr = queries.UpdateContract(ctx, UpdateContractParams{
		Channelid:  contract.Location[0].ChannelID,
		Contractid: contract.ContractID,
		Coopid:     contract.CoopID,
		Value:      sql.NullString{String: string(contractJSON), Valid: true},
	})
	if rows != 1 {
		log.Printf("Error updating contract data: %v", updateErr)
		// Record exists, update instead
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
