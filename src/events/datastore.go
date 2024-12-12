package events

import (
	"encoding/json"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/peterbourgon/diskv/v3"
)

var dataStore *diskv.Diskv

func init() {

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

func saveCustomEggData(c map[string]*ei.EggIncCustomEgg) {
	b, _ := json.Marshal(c)
	_ = dataStore.Write("ei-customeggs", b)
}

func loadCustomEggData() (map[string]*ei.EggIncCustomEgg, error) {
	var c map[string]*ei.EggIncCustomEgg
	b, err := dataStore.Read("ei-customeggs")
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	if err != nil {
		return c, err
	}

	return c, nil
}
