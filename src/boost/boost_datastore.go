package boost

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/peterbourgon/diskv/v3"
)

var dataStore *diskv.Diskv

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

func saveData(c map[string]*Contract) error {
	//diskmutex.Lock()
	b, _ := json.Marshal(c)
	dataStore.Write("EggsBackup", b)

	//diskmutex.Unlock()
	return nil
}

func saveEndData(c *Contract) error {
	//diskmutex.Lock()
	var saveName = fmt.Sprintf("%s/%s", c.ContractID, c.CoopID)
	b, _ := json.Marshal(c)
	dataStore.Write(saveName, b)
	//diskmutex.Unlock()
	return nil
}

func loadData() (map[string]*Contract, error) {
	//diskmutex.Lock()
	var c map[string]*Contract
	b, err := dataStore.Read("EggsBackup")
	if err != nil {
		return c, err
	}
	json.Unmarshal(b, &c)
	//diskmutex.Unlock()

	return c, nil
}
