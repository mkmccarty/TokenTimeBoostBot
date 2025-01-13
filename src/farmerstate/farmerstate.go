package farmerstate

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/peterbourgon/diskv/v3"
)

// Link will hold bookmark links
type Link struct {
	Link      string    `json:"Link"`
	Timestamp time.Time `json:"Timestamp"`
}

// Farmer struct to store user data
type Farmer struct {
	UserID               string    // Discord User ID
	EggIncName           string    // User's Egg Inc name
	Ping                 bool      // True/False
	Tokens               int       // Number of tokens this user wants
	OrderHistory         []int     // list of contract order percentiles
	LaunchChain          bool      // Launch History chain option
	MissionShipPrimary   int       // Launch Helper Ship Selection - Primary
	MissionShipSecondary int       // Launch Helper Ship Selection - Secondary
	UltraContract        time.Time // Date last Ultra contract was detected
	MiscSettingsFlag     map[string]bool
	MiscSettingsString   map[string]string
	Links                []Link // Array of Links
	LastUpdated          time.Time
	DataPrivacy          bool // User data privacy setting
}

// OrderHistory struct to store order history data
type OrderHistory struct {
	Order [][]string `json:"Order"`
}

var (
	farmerstate map[string]*Farmer
	dataStore   *diskv.Diskv
)

func init() {
	/*
		if flag.Lookup("test.v") == nil {
			fmt.Println("normal run")
		} else {
			fmt.Println("run under go test")
		}
	*/
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

	// Keep only the last 10 order percentiles
	for _, farmer := range farmerstate {
		if len(farmerstate[farmer.UserID].OrderHistory) > 10 {
			farmerstate[farmer.UserID].OrderHistory = farmerstate[farmer.UserID].OrderHistory[len(farmerstate[farmer.UserID].OrderHistory)-10:]
		}
	}
}

// NewFarmer creates a new Farmer
func newFarmer(userID string) {
	farmerstate[userID] = &Farmer{
		UserID:               userID,
		Ping:                 false,
		Tokens:               0,
		DataPrivacy:          false,
		LaunchChain:          false,
		MissionShipPrimary:   0,
		MissionShipSecondary: 1,
	}
}

// DeleteFarmer deletes a Farmer from the map
func DeleteFarmer(userID string) {
	delete(farmerstate, userID)
}

// GetEggIncName returns a Farmer Egg Inc name
func GetEggIncName(userID string) string {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}
	return farmerstate[userID].EggIncName
}

// SetEggIncName sets a Farmer Egg Inc name
func SetEggIncName(userID string, eggIncName string) {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}
	if !farmerstate[userID].DataPrivacy {
		farmerstate[userID].LastUpdated = time.Now()
		farmerstate[userID].EggIncName = eggIncName
		SetMiscSettingString(userID, "EggIncRawName", eggIncName)
		saveData(farmerstate)
	}
}

// GetLaunchHistory returns a Farmer Launch History
func GetLaunchHistory(userID string) bool {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}
	return farmerstate[userID].LaunchChain
}

// SetLaunchHistory sets a Farmer Launch History
func SetLaunchHistory(userID string, setting bool) {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}
	if !farmerstate[userID].DataPrivacy {
		farmerstate[userID].LastUpdated = time.Now()
		farmerstate[userID].LaunchChain = setting
		saveData(farmerstate)
	}
}

// GetMissionShipPrimary returns a Farmer Mission Ship Primary
func GetMissionShipPrimary(userID string) int {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}
	return farmerstate[userID].MissionShipPrimary
}

// SetMissionShipPrimary sets a Farmer Mission Ship Primary
func SetMissionShipPrimary(userID string, setting int) {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}
	if !farmerstate[userID].DataPrivacy {
		farmerstate[userID].LastUpdated = time.Now()
		farmerstate[userID].MissionShipPrimary = setting
		saveData(farmerstate)
	}
}

// GetMissionShipSecondary returns a Farmer Mission Ship Secondary
func GetMissionShipSecondary(userID string) int {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}
	return farmerstate[userID].MissionShipSecondary
}

// SetMissionShipSecondary sets a Farmer Mission Ship Secondary
func SetMissionShipSecondary(userID string, setting int) {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}
	if !farmerstate[userID].DataPrivacy {
		farmerstate[userID].LastUpdated = time.Now()
		farmerstate[userID].MissionShipSecondary = setting
		saveData(farmerstate)
	}
}

// GetTokens returns a Farmer's tokens
func GetTokens(userID string) int {
	if farmerstate, ok := farmerstate[userID]; ok {
		return farmerstate.Tokens
	}
	return 0
}

// SetTokens sets a Farmer's tokens
func SetTokens(userID string, tokens int) {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}
	if !farmerstate[userID].DataPrivacy {
		farmerstate[userID].LastUpdated = time.Now()
		farmerstate[userID].Tokens = tokens
		saveData(farmerstate)
	}
}

// SetPing sets a Farmer's ping preference
func SetPing(userID string, ping bool) {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}

	if !farmerstate[userID].DataPrivacy {
		farmerstate[userID].LastUpdated = time.Now()
		farmerstate[userID].Ping = ping
		saveData(farmerstate)
	}
}

// GetPing returns a Farmer's ping preference
func GetPing(userID string) bool {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}

	if farmerstate, ok := farmerstate[userID]; ok {
		return farmerstate.Ping
	}
	return false
}

// SetOrderPercentileOne sets a Farmer's order percentile
func SetOrderPercentileOne(userID string, boostNumber int, coopSize int) {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}
	if !farmerstate[userID].DataPrivacy {
		farmerstate[userID].LastUpdated = time.Now()
		var percentile = boostNumber * 100 / coopSize

		farmerstate[userID].OrderHistory = append(farmerstate[userID].OrderHistory, percentile)
		if len(farmerstate[userID].OrderHistory) > 10 {
			farmerstate[userID].OrderHistory = farmerstate[userID].OrderHistory[len(farmerstate[userID].OrderHistory)-10:]
		}
		saveData(farmerstate)
	}
}

// SetOrderPercentileAll sets a Farmer's order percentiles from a slice
func SetOrderPercentileAll(userIDs []string, coopSize int) {
	for i, userID := range userIDs {
		if farmerstate[userID] == nil {
			newFarmer(userID)
		}
		if !farmerstate[userID].DataPrivacy {
			farmerstate[userID].LastUpdated = time.Now()
			farmerstate[userID].OrderHistory = append(farmerstate[userID].OrderHistory, (i+1)*100/coopSize)
			if len(farmerstate[userID].OrderHistory) > 10 {
				farmerstate[userID].OrderHistory = farmerstate[userID].OrderHistory[len(farmerstate[userID].OrderHistory)-10:]
			}
		}
	}
	saveData(farmerstate)
}

// GetOrderHistory returns a Farmer's order history
func GetOrderHistory(userIDs []string, number int) []string {
	var orderHistory = make(map[string]int)

	for _, userID := range userIDs {

		if farmerstate[userID] == nil {
			newFarmer(userID)
			farmerstate[userID].OrderHistory = append(farmerstate[userID].OrderHistory, 50)
		} else {
			count := len(farmerstate[userID].OrderHistory)
			if count > 0 {
				if count > number {
					count = number
				}

				// get an average of the last count percentiles for this user
				var sum int
				for i := 0; i < count; i++ {
					sum += farmerstate[userID].OrderHistory[len(farmerstate[userID].OrderHistory)-i-1]
				}
				orderHistory[userID] = sum / count
			} else {
				orderHistory[userID] = 50
			}
		}
	}
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
	log.Println("getOrderHistory", sortedOrderHistory)
	return sortedOrderHistory
}

// SetMiscSettingFlag sets a key-value sticky setting
func SetMiscSettingFlag(userID string, key string, value bool) {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}

	if farmerstate[userID].MiscSettingsFlag == nil {
		farmerstate[userID].MiscSettingsFlag = make(map[string]bool)
	}
	if !farmerstate[userID].DataPrivacy {
		farmerstate[userID].LastUpdated = time.Now()
		farmerstate[userID].MiscSettingsFlag[key] = value
		saveData(farmerstate)
	}
}

// GetMiscSettingFlag returns a Farmer sticky setting
func GetMiscSettingFlag(userID string, key string) bool {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}

	if farmer, ok := farmerstate[userID]; ok {
		if farmer.MiscSettingsFlag == nil {
			farmer.MiscSettingsFlag = make(map[string]bool)
		}
		return farmer.MiscSettingsFlag[key]
	}
	return false
}

// SetMiscSettingString sets a key-value sticky setting
func SetMiscSettingString(userID string, key string, value string) {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}

	if farmerstate[userID].MiscSettingsString == nil {
		farmerstate[userID].MiscSettingsString = make(map[string]string)
	}

	if !farmerstate[userID].DataPrivacy {
		farmerstate[userID].LastUpdated = time.Now()
		if farmerstate[userID].MiscSettingsString[key] != value {
			farmerstate[userID].MiscSettingsString[key] = value
			saveData(farmerstate)
		}
	}
}

// GetMiscSettingString returns a Farmer sticky setting
func GetMiscSettingString(userID string, key string) string {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}

	if farmer, ok := farmerstate[userID]; ok {
		if farmer.MiscSettingsString == nil {
			farmer.MiscSettingsString = make(map[string]string)
		}
		return farmer.MiscSettingsString[key]
	}
	return ""
}

// GetLinks will return a slice of bookmark links
func GetLinks(userID string) []string {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}

	var retLinks []string
	// Collect all Links.link into a slice
	for _, link := range farmerstate[userID].Links {
		retLinks = append(retLinks, link.Link)
	}

	return retLinks
}

// SetLink will store a link for a user
func SetLink(userID string, description string, guildID string, channelID string, messageID string) {
	//	link := fmt.Sprintf("https://discordapp.com/channels/%s/%s/%s", contract.Location[0].GuildID, contract.Location[0].ChannelID, contract.SRData.ChickenRunCheckMsgID)
	//
	// fmt.Fprintf(&builder, "\n[link to Chicken Run Check Status](%s)\n", link)
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}
	if !farmerstate[userID].DataPrivacy {
		farmerstate[userID].LastUpdated = time.Now()
		var link Link
		var strURL string
		link.Timestamp = time.Now()
		if messageID == "" {
			link.Link = description + " <#" + channelID + ">"
		} else if guildID != "" {
			strURL = "https://discordapp.com/channels/" + guildID + "/" + channelID + "/" + messageID
			link.Link = fmt.Sprintf("%s %s", description, strURL)
		} else {
			// https://discord.com/channels/@me/1124490885204287610/1264231460244815913
			strURL = "https://discordapp.com/channels/@me/" + channelID + "/" + messageID
			link.Link = fmt.Sprintf("%s %s", description, strURL)
		}

		newList := append(farmerstate[userID].Links, link)
		farmerstate[userID].Links = nil
		// Prune farmerstate links older than 2 days
		for _, el := range newList {
			if el.Timestamp.Before(time.Now().Add(-48 * time.Hour)) {
				farmerstate[userID].Links = append(farmerstate[userID].Links, el)
			}
		}
		saveData(farmerstate)
	}
}

// IsUltra will return if a player has joined an ultra contract in last 60 days
func IsUltra(userID string) bool {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}

	return time.Since(farmerstate[userID].UltraContract) <= 60*24*time.Hour
}

// SetUltra sets a player to have joined an ultra contract
func SetUltra(userID string) {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}
	if !farmerstate[userID].DataPrivacy {
		farmerstate[userID].UltraContract = time.Now()
		saveData(farmerstate)
	}
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

func saveData(c map[string]*Farmer) {
	//diskmutex.Lock()
	b, _ := json.Marshal(c)
	_ = dataStore.Write("Farmers", b)
}

func loadData() (map[string]*Farmer, error) {

	var c map[string]*Farmer
	b, err := dataStore.Read("Farmers")
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	if err != nil {
		return c, err
	}

	return c, nil
}
