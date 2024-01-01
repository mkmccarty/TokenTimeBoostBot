package farmerstate

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/peterbourgon/diskv/v3"
)

type Farmer struct {
	UserID       string // Discord User ID
	Ping         bool   // True/False
	Tokens       int    // Number of tokens this user wants
	OrderHistory []int  // list of contract order percentiles
}

type OrderHistory struct {
	Order [][]string `json:"Order"`
}

var (
	farmerstate map[string]*Farmer
	dataStore   *diskv.Diskv
)

func init() {

	if flag.Lookup("test.v") == nil {
		fmt.Println("normal run")
	} else {
		fmt.Println("run under go test")
	}

	farmerstate = make(map[string]*Farmer)
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
		farmerstate = f
	}

	file, fileerr := os.ReadFile("ttbb-data/boost_data_history.json")
	if fileerr == nil {
		data := OrderHistory{}
		json.Unmarshal([]byte(file), &data)

		// create Farmer OrderHistory from read data
		for _, boostHistory := range data.Order {
			SetOrderPercentileAll(boostHistory, len(boostHistory))
		}
		// delete the file
		os.Remove("ttbb-data/boost_data_history.json")
	}

}

// NewFarmer creates a new Farmer
func newFarmer(userID string) {
	farmerstate[userID] = &Farmer{
		UserID: userID,
		Ping:   false,
		Tokens: 0,
	}
}

func DeleteFarmer(userID string) {
	delete(farmerstate, userID)
}

func GetTokens(userID string) int {
	if farmerstate, ok := farmerstate[userID]; ok {
		return farmerstate.Tokens
	}
	return 0
}

func SetTokens(userID string, tokens int) {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}

	farmerstate[userID].Tokens = tokens
	saveData(farmerstate)
}

func SetPing(userID string, ping bool) {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}

	farmerstate[userID].Ping = ping
	saveData(farmerstate)
}

func GetPing(userID string) bool {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}

	if farmerstate, ok := farmerstate[userID]; ok {
		return farmerstate.Ping
	}
	return false
}

func SetOrderPercentileOne(userID string, boostNumber int, coopSize int) {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}

	var percentile = boostNumber * 100 / coopSize

	farmerstate[userID].OrderHistory = append(farmerstate[userID].OrderHistory, percentile)
	saveData(farmerstate)
}

func SetOrderPercentileAll(userIDs []string, coopSize int) {
	for i, userID := range userIDs {
		if farmerstate[userID] == nil {
			newFarmer(userID)
		}
		farmerstate[userID].OrderHistory = append(farmerstate[userID].OrderHistory, (i+1)*100/coopSize)
	}
	saveData(farmerstate)
}

func GetOrderHistory(userIDs []string, number int) []string {
	var orderHistory = make(map[string]int)

	for _, userID := range userIDs {

		if farmerstate[userID] == nil {
			orderHistory[userID] = 1 // Unknown user gets to be last
		} else {
			count := len(farmerstate[userID].OrderHistory)
			if count > number {
				count = number
			}
			// get an average of the last count percentiles for this user
			var sum int
			for i := 0; i < count; i++ {
				sum += farmerstate[userID].OrderHistory[len(farmerstate[userID].OrderHistory)-i-1]
			}
			orderHistory[userID] = sum / count
		}
	}
	fmt.Println(orderHistory)
	// iterate over orderHistory and return a slice of userIDs sorted by percentile
	var sortedOrderHistory []string
	for i := 0; i < len(userIDs); i++ {
		var max int
		var maxUserID string
		for userID, percentile := range orderHistory {
			if percentile > max {
				max = percentile
				maxUserID = userID
			}
		}
		sortedOrderHistory = append(sortedOrderHistory, maxUserID)
		delete(orderHistory, maxUserID)
	}
	return sortedOrderHistory
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
	if testing.Testing() {
		return nil
	}

	return nil
}

func loadData() (map[string]*Farmer, error) {

	var c map[string]*Farmer
	b, err := dataStore.Read("Farmers")
	if err != nil {
		return c, err
	}
	json.Unmarshal(b, &c)

	return c, nil
}
