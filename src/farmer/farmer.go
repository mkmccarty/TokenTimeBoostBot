package farmer

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
	farmers   map[string]*Farmer
	dataStore *diskv.Diskv
)

func init() {
	farmers = make(map[string]*Farmer)
	//Glob
	// DataStore to initialize a new diskv store, rooted at "my-data-dir", with a 1MB cache.
	dataStore = diskv.New(diskv.Options{
		BasePath:          "ttbb-data",
		AdvancedTransform: AdvancedTransform,
		InverseTransform:  InverseTransform,
		CacheSizeMax:      512 * 512,
	})

	var f, err = loadData()
	if err == nil {
		farmers = f
	}
}

// NewFarmer creates a new Farmer
func newFarmer(userID string) {
	farmers[userID] = &Farmer{
		userID: userID,
		ping:   false,
		tokens: 0,
	}
}

func DeleteFarmer(userID string) {
	delete(farmers, userID)
}

func GetTokens(userID string) int {
	if farmer, ok := farmers[userID]; ok {
		return farmer.tokens
	}
	return 0
}

func SetTokens(userID string, tokens int) {
	if farmers[userID] == nil {
		newFarmer(userID)
	}

	farmers[userID].tokens = tokens
	saveData(farmers)
}

func SetPing(userID string, ping bool) {
	if farmers[userID] == nil {
		newFarmer(userID)
	}

	farmers[userID].ping = ping
	saveData(farmers)
}

func GetPing(userID string) bool {
	if farmers[userID] == nil {
		newFarmer(userID)
	}

	if farmer, ok := farmers[userID]; ok {
		return farmer.ping
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
