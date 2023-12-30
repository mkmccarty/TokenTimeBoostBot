package farmerstate

import (
	"encoding/json"
	"strings"

	"github.com/peterbourgon/diskv/v3"
)

type Farmer struct {
	userID string // Discord User ID
	ping   bool   // True/False
	tokens int    // Number of tokens this user wants
	//order_history []string // List of order IDs this user has made
}

var (
	farmerstate map[string]*Farmer
	dataStore   *diskv.Diskv
)

func init() {
	farmerstate = make(map[string]*Farmer)
	//Glob
	// DataStore to initialize a new diskv store, rooted at "my-data-dir", with a 1MB cache.
	dataStore = diskv.New(diskv.Options{
		BasePath:          "../../ttbb-data",
		AdvancedTransform: AdvancedTransform,
		InverseTransform:  InverseTransform,
		CacheSizeMax:      512 * 512,
	})

	var f, err = loadData()
	if err == nil {
		farmerstate = f
	}
}

// NewFarmer creates a new Farmer
func newFarmer(userID string) {
	farmerstate[userID] = &Farmer{
		userID: userID,
		ping:   false,
		tokens: 0,
	}
}

func DeleteFarmer(userID string) {
	delete(farmerstate, userID)
}

func GetTokens(userID string) int {
	if farmerstate, ok := farmerstate[userID]; ok {
		return farmerstate.tokens
	}
	return 0
}

func SetTokens(userID string, tokens int) {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}

	farmerstate[userID].tokens = tokens
	saveData(farmerstate)
}

func SetPing(userID string, ping bool) {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}

	farmerstate[userID].ping = ping
	saveData(farmerstate)
}

func GetPing(userID string) bool {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}

	if farmerstate, ok := farmerstate[userID]; ok {
		return farmerstate.ping
	}
	return false
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

func saveData(c map[string]*Farmer) error {
	//diskmutex.Lock()
	b, _ := json.Marshal(c)
	dataStore.Write("Farmers", b)

	//diskmutex.Unlock()
	return nil
}

func loadData() (map[string]*Farmer, error) {
	//diskmutex.Lock()
	var c map[string]*Farmer
	b, err := dataStore.Read("Farmers")
	if err != nil {
		return c, err
	}
	json.Unmarshal(b, &c)
	//diskmutex.Unlock()

	return c, nil
}
