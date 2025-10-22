package farmerstate

import (
	"context"
	"database/sql"
	_ "embed" // This is used to embed the schema.sql file
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite" // Want this here
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
)

var ctx = context.Background()

//go:embed schema.sql
var ddl string
var queries *Queries

func sqliteInit() {
	//db, _ := sql.Open("sqlite", ":memory:")
	db, _ := sql.Open("sqlite", "ttbb-data/Farmers.sqlite")

	result, err := db.ExecContext(ctx, ddl)
	if err != nil {
		log.Printf("We have an error: %v", err)
	} else {
		fmt.Print(result)
	}
	queries = New(db)
}

func init() {
	sqliteInit()

	// Initialized farmerstate map, will be populated on demand
	farmerstate = make(map[string]*Farmer)
}

// saveSqliteData saves a single piece of farmer data to SQLite (for legacy support)
func saveSqliteData(userID string, farmer *Farmer) {
	// Save the farmer data to SQLite
	farmer.LastUpdated = time.Now()
	farmerJSON, err := json.Marshal(farmer)
	if err != nil {
		log.Printf("Error marshaling farmer data: %v", err)
		return
	}
	_, err = queries.UpdateLegacyFarmerstate(ctx, UpdateLegacyFarmerstateParams{
		Value: sql.NullString{String: string(farmerJSON), Valid: true},
		ID:    userID,
	})
	if err != nil {
		// Record exists, update instead
		_, err = queries.InsertLegacyFarmerstate(ctx, InsertLegacyFarmerstateParams{
			ID:    userID,
			Value: sql.NullString{String: string(farmerJSON), Valid: true},
		})
		if err != nil {
			log.Printf("Error saving farmer data to SQLite: %v", err)
		}
	}
}

/*
func loadAllSqliteData() (map[string]Farmer, error) {
	farmers := make(map[string]Farmer)
	rows, err := queries.GetAllLegacyFarmerstate(ctx)
	if err != nil {
		return farmers, err
	}
	for _, row := range rows {
		var farmer Farmer
		if row.Value.Valid {
			err = json.Unmarshal([]byte(row.Value.String), &farmer)
			if err != nil {
				log.Printf("Error unmarshaling farmer data for user %s: %v", row.ID, err)
				continue
			}
			farmers[row.ID] = farmer
		}
	}

	return farmers, nil
}
*/

// NewFarmer creates a new Farmer
func newFarmer(userID string) {
	// Check if farmer already exists in SQLite
	sqliteFarmer, err := queries.GetLegacyFarmerstate(ctx, userID)
	if err == nil && sqliteFarmer.Value.Valid {
		var farmer Farmer
		err = json.Unmarshal([]byte(sqliteFarmer.Value.String), &farmer)
		if err == nil {
			farmerstate[userID] = &farmer
			return
		}
	}

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
		farmerstate[userID].EggIncName = eggIncName
		SetMiscSettingString(userID, "EggIncRawName", eggIncName)
		saveSqliteData(userID, farmerstate[userID])
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
		farmerstate[userID].LaunchChain = setting
		saveSqliteData(userID, farmerstate[userID])
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
		farmerstate[userID].MissionShipPrimary = setting
		saveSqliteData(userID, farmerstate[userID])
	}
}

// GetMissionShipSecondary returns a Farmer Mission Ship Secondary
func GetMissionShipSecondary(userID string) int {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}
	f := farmerstate[userID]
	return f.MissionShipSecondary
}

// SetMissionShipSecondary sets a Farmer Mission Ship Secondary
func SetMissionShipSecondary(userID string, setting int) {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}
	if !farmerstate[userID].DataPrivacy {
		farmerstate[userID].MissionShipSecondary = setting
		saveSqliteData(userID, farmerstate[userID])
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
		farmerstate[userID].Tokens = tokens
		saveSqliteData(userID, farmerstate[userID])
	}
}

// SetPing sets a Farmer's ping preference
func SetPing(userID string, ping bool) {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}

	if !farmerstate[userID].DataPrivacy {
		farmerstate[userID].Ping = ping
		saveSqliteData(userID, farmerstate[userID])
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

// SetMiscSettingFlag sets a key-value sticky setting
func SetMiscSettingFlag(userID string, key string, value bool) {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}

	if farmerstate[userID].MiscSettingsFlag == nil {
		farmerstate[userID].MiscSettingsFlag = make(map[string]bool)
	}
	if !farmerstate[userID].DataPrivacy {
		farmerstate[userID].MiscSettingsFlag[key] = value
		saveSqliteData(userID, farmerstate[userID])
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
		if value == "" {
			delete(farmerstate[userID].MiscSettingsString, key)
			saveSqliteData(userID, farmerstate[userID])
		} else {
			if farmerstate[userID].MiscSettingsString[key] != value {
				farmerstate[userID].MiscSettingsString[key] = value
				saveSqliteData(userID, farmerstate[userID])
			}
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
		saveSqliteData(userID, farmerstate[userID])
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
		saveSqliteData(userID, farmerstate[userID])
	}
}

// GetDiscordUserIDFromEiIgn retrieves the Discord user ID based on the provided ei_ign
func GetDiscordUserIDFromEiIgn(eiIgn string) (string, error) {
	id, err := queries.GetUserIdFromEiIgn(ctx, sql.NullString{String: eiIgn, Valid: true})
	if err != nil {
		return "", err
	}
	return id, nil
}
