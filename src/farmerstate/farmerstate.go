package farmerstate

import (
	"context"
	"database/sql"
	_ "embed" // This is used to embed the schema.sql file
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"

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
	LaunchChain          bool      // Launch History chain option
	MissionShipPrimary   int       // Launch Helper Ship Selection - Primary
	MissionShipSecondary int       // Launch Helper Ship Selection - Secondary
	UltraContract        time.Time // Date last Ultra contract was detected
	MiscSettingsFlag     map[string]bool
	MiscSettingsString   map[string]string
	Links                []Link // Array of Links
	LastUpdated          time.Time
	LastSeen             time.Time // Last time farmer was added to a contract
	DataPrivacy          bool      // User data privacy setting
}

var (
	farmerstate  map[string]*Farmer
	pendingSaves = make(map[string]string)
	saveMutex    sync.Mutex
	stateMutex   sync.RWMutex
)

var ctx = context.Background()

//go:embed schema.sql
var ddl string
var queries *Queries

func sqliteInit() {
	//db, _ := sql.Open("sqlite", ":memory:")
	db, _ := sql.Open("sqlite", "ttbb-data/Farmers.sqlite?_busy_timeout=5000")
	db.SetMaxOpenConns(1)

	// Drop old leaderboard_stats table if it has guild_id column to migrate to new global schema
	var hasGuildID bool
	rows, err := db.QueryContext(ctx, "PRAGMA table_info(leaderboard_stats)")
	if err == nil {
		for rows.Next() {
			var cid int
			var name string
			var type_ string
			var notnull int
			var dfltVal interface{}
			var pk int
			if err := rows.Scan(&cid, &name, &type_, &notnull, &dfltVal, &pk); err == nil {
				if name == "guild_id" {
					hasGuildID = true
				}
			}
		}
		_ = rows.Close()
	}
	if hasGuildID {
		log.Println("farmerstate: dropping old guild-specific leaderboard_stats to migrate to global schema")
		_, _ = db.ExecContext(ctx, "DROP TABLE leaderboard_stats;")
	}

	// Execute each statement in the DDL to set up the database schema
	for stmt := range strings.SplitSeq(ddl, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt != "" {
			_, _ = db.ExecContext(ctx, stmt)
		}
	}
	queries = New(db)
	go dbFlusher()
}

func init() {
	sqliteInit()

	// Initialized farmerstate map, will be populated on demand
	farmerstate = make(map[string]*Farmer)
}

// saveSqliteData saves a single piece of farmer data to SQLite (for legacy support)
func saveSqliteData(userID string, farmer *Farmer) {
	if farmer == nil || bottools.IsRandomName(userID) {
		return
	}

	// Save the farmer data to SQLite
	farmer.LastUpdated = time.Now()
	farmerJSON, err := json.Marshal(farmer)
	if err != nil {
		log.Printf("Error marshaling farmer data: %v", err)
		return
	}

	saveMutex.Lock()
	pendingSaves[userID] = string(farmerJSON)
	saveMutex.Unlock()
}

func dbFlusher() {
	ticker := time.NewTicker(2 * time.Second)
	for range ticker.C {
		FlushPendingSaves()
	}
}

// FlushPendingSaves forces an immediate write of all pending database updates.
func FlushPendingSaves() {
	saveMutex.Lock()
	if len(pendingSaves) == 0 {
		saveMutex.Unlock()
		return
	}
	toSave := pendingSaves
	pendingSaves = make(map[string]string)
	saveMutex.Unlock()

	if queries == nil {
		return
	}

	for userID, farmerJSON := range toSave {
		rows, err := queries.UpdateLegacyFarmerstate(ctx, UpdateLegacyFarmerstateParams{
			Value: sql.NullString{String: farmerJSON, Valid: true},
			ID:    userID,
		})
		if err == nil && rows == 0 {
			_, err = queries.InsertLegacyFarmerstate(ctx, InsertLegacyFarmerstateParams{
				ID:    userID,
				Value: sql.NullString{String: farmerJSON, Valid: true},
			})
		}
		if err != nil {
			log.Printf("Error saving farmer data to SQLite: %v", err)
		}
	}
}

// getFarmer returns a Farmer from the map, creating it if it doesn't exist
// This function is thread-safe.
func getFarmer(userID string) *Farmer {
	stateMutex.RLock()
	f, ok := farmerstate[userID]
	stateMutex.RUnlock()
	if ok {
		return f
	}

	stateMutex.Lock()
	defer stateMutex.Unlock()

	// Check again in case another goroutine created it while we were waiting for the lock
	if f, ok = farmerstate[userID]; ok {
		return f
	}

	// Check if farmer already exists in SQLite
	// We do this while holding the lock to ensure we don't have multiple
	// goroutines trying to load/create the same farmer.
	sqliteFarmer, err := queries.GetLegacyFarmerstate(ctx, userID)
	if err == nil && sqliteFarmer.Value.Valid {
		var farmer Farmer
		err = json.Unmarshal([]byte(sqliteFarmer.Value.String), &farmer)
		if err == nil {
			farmerstate[userID] = &farmer
			return &farmer
		}
	}

	f = &Farmer{
		UserID:               userID,
		Ping:                 false,
		Tokens:               0,
		DataPrivacy:          false,
		LaunchChain:          false,
		MissionShipPrimary:   0,
		MissionShipSecondary: 1,
		LastSeen:             time.Now(),
	}
	farmerstate[userID] = f
	return f
}

// DeleteFarmer deletes a Farmer from the map
func DeleteFarmer(userID string) {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	delete(farmerstate, userID)
}

// GetEggIncName returns a Farmer Egg Inc name
func GetEggIncName(userID string) string {
	f := getFarmer(userID)
	stateMutex.RLock()
	defer stateMutex.RUnlock()
	return f.EggIncName
}

// SetEggIncName sets a Farmer Egg Inc name
func SetEggIncName(userID string, eggIncName string) {
	f := getFarmer(userID)
	stateMutex.Lock()
	defer stateMutex.Unlock()
	if !f.DataPrivacy {
		f.EggIncName = eggIncName
		// Release lock before calling another exported function that takes the lock
		stateMutex.Unlock()
		SetMiscSettingString(userID, "EggIncRawName", eggIncName)
		stateMutex.Lock()
	}
}

// GetLaunchHistory returns a Farmer Launch History
func GetLaunchHistory(userID string) bool {
	f := getFarmer(userID)
	stateMutex.RLock()
	defer stateMutex.RUnlock()
	return f.LaunchChain
}

// SetLaunchHistory sets a Farmer Launch History
func SetLaunchHistory(userID string, setting bool) {
	f := getFarmer(userID)
	stateMutex.Lock()
	defer stateMutex.Unlock()
	if !f.DataPrivacy {
		f.LaunchChain = setting
		saveSqliteData(userID, f)
	}
}

// GetMissionShipPrimary returns a Farmer Mission Ship Primary
func GetMissionShipPrimary(userID string) int {
	f := getFarmer(userID)
	stateMutex.RLock()
	defer stateMutex.RUnlock()
	return f.MissionShipPrimary
}

// SetMissionShipPrimary sets a Farmer Mission Ship Primary
func SetMissionShipPrimary(userID string, setting int) {
	f := getFarmer(userID)
	stateMutex.Lock()
	defer stateMutex.Unlock()
	if !f.DataPrivacy {
		f.MissionShipPrimary = setting
		saveSqliteData(userID, f)
	}
}

// GetMissionShipSecondary returns a Farmer Mission Ship Secondary
func GetMissionShipSecondary(userID string) int {
	f := getFarmer(userID)
	stateMutex.RLock()
	defer stateMutex.RUnlock()
	return f.MissionShipSecondary
}

// SetMissionShipSecondary sets a Farmer Mission Ship Secondary
func SetMissionShipSecondary(userID string, setting int) {
	f := getFarmer(userID)
	stateMutex.Lock()
	defer stateMutex.Unlock()
	if !f.DataPrivacy {
		f.MissionShipSecondary = setting
		saveSqliteData(userID, f)
	}
}

// GetTokens returns a Farmer's tokens
func GetTokens(userID string) int {
	f := getFarmer(userID)
	stateMutex.RLock()
	defer stateMutex.RUnlock()
	return f.Tokens
}

// SetTokens sets a Farmer's tokens
func SetTokens(userID string, tokens int) {
	f := getFarmer(userID)
	stateMutex.Lock()
	defer stateMutex.Unlock()
	if !f.DataPrivacy {
		f.Tokens = tokens
		saveSqliteData(userID, f)
	}
}

// SetPing sets a Farmer's ping preference
func SetPing(userID string, ping bool) {
	f := getFarmer(userID)
	stateMutex.Lock()
	defer stateMutex.Unlock()

	if !f.DataPrivacy {
		f.Ping = ping
		saveSqliteData(userID, f)
	}
}

// SetLastSeen updates the timestamp of the last time a farmer was added to a contract
func SetLastSeen(userID string) {
	f := getFarmer(userID)
	stateMutex.Lock()
	defer stateMutex.Unlock()
	f.LastSeen = time.Now()
	saveSqliteData(userID, f)
}

// GetLastSeen returns the last time a farmer was added to a contract
func GetLastSeen(userID string) time.Time {
	f := getFarmer(userID)
	stateMutex.RLock()
	defer stateMutex.RUnlock()
	return f.LastSeen
}

// GetPing returns a Farmer's ping preference
func GetPing(userID string) bool {
	f := getFarmer(userID)
	stateMutex.RLock()
	defer stateMutex.RUnlock()
	return f.Ping
}

// SetMiscSettingFlag sets a key-value sticky setting
func SetMiscSettingFlag(userID string, key string, value bool) {
	f := getFarmer(userID)
	stateMutex.Lock()
	defer stateMutex.Unlock()

	if f.MiscSettingsFlag == nil {
		f.MiscSettingsFlag = make(map[string]bool)
	}
	if !f.DataPrivacy {
		f.MiscSettingsFlag[key] = value
		saveSqliteData(userID, f)
	}
}

// GetMiscSettingFlag returns a Farmer sticky setting
func GetMiscSettingFlag(userID string, key string) bool {
	f := getFarmer(userID)
	stateMutex.Lock() // Using Lock because we might initialize the map
	defer stateMutex.Unlock()

	if f.MiscSettingsFlag == nil {
		f.MiscSettingsFlag = make(map[string]bool)
	}
	return f.MiscSettingsFlag[key]
}

// SetMiscSettingString sets a key-value sticky setting
func SetMiscSettingString(userID string, key string, value string) {
	f := getFarmer(userID)
	stateMutex.Lock()
	defer stateMutex.Unlock()

	if f.MiscSettingsString == nil {
		f.MiscSettingsString = make(map[string]string)
	}

	if !f.DataPrivacy {
		if value == "" {
			delete(f.MiscSettingsString, key)
			saveSqliteData(userID, f)
		} else {
			if f.MiscSettingsString[key] != value {
				f.MiscSettingsString[key] = value
				saveSqliteData(userID, f)
			}
		}
	}
}

// GetMiscSettingString returns a Farmer sticky setting
func GetMiscSettingString(userID string, key string) string {
	f := getFarmer(userID)
	stateMutex.Lock() // Using Lock because we might initialize the map
	defer stateMutex.Unlock()

	if f.MiscSettingsString == nil {
		f.MiscSettingsString = make(map[string]string)
	}
	return f.MiscSettingsString[key]
}

// GetLinks will return a slice of bookmark links
func GetLinks(userID string) []string {
	f := getFarmer(userID)
	stateMutex.RLock()
	defer stateMutex.RUnlock()

	var retLinks []string
	// Collect all Links.link into a slice
	for _, link := range f.Links {
		retLinks = append(retLinks, link.Link)
	}

	return retLinks
}

// SetLink will store a link for a user
func SetLink(userID string, description string, guildID string, channelID string, messageID string) {
	f := getFarmer(userID)
	stateMutex.Lock()
	defer stateMutex.Unlock()
	if !f.DataPrivacy {
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

		newList := append(f.Links, link)
		f.Links = nil
		// Prune farmerstate links older than 2 days
		for _, el := range newList {
			if el.Timestamp.Before(time.Now().Add(-48 * time.Hour)) {
				f.Links = append(f.Links, el)
			}
		}
		saveSqliteData(userID, f)
	}
}

// IsUltra will return if a player has joined an ultra contract in last 60 days
func IsUltra(userID string) bool {
	f := getFarmer(userID)
	stateMutex.RLock()
	defer stateMutex.RUnlock()

	return time.Since(f.UltraContract) <= 60*24*time.Hour
}

// SetUltra sets a player to have joined an ultra contract
func SetUltra(userID string) {
	f := getFarmer(userID)
	stateMutex.Lock()
	defer stateMutex.Unlock()
	if !f.DataPrivacy {
		f.UltraContract = time.Now()
		saveSqliteData(userID, f)
	}
}

// GetEiIgnsByMiscString returns all ei_ign values for farmers where MiscSettingsString[key] == value.
func GetEiIgnsByMiscString(key, value string) []string {
	FlushPendingSaves()
	results, err := queries.GetEiIgnsByMiscString(ctx, GetEiIgnsByMiscStringParams{
		Column1: sql.NullString{String: key, Valid: true},
		Value:   sql.NullString{String: value, Valid: true},
	})
	if err != nil {
		log.Println("GetEiIgnsByMiscString:", err)
		return nil
	}
	eiIgns := make([]string, 0, len(results))
	for _, r := range results {
		if s, ok := r.(string); ok && s != "" {
			eiIgns = append(eiIgns, s)
		}
	}
	return eiIgns
}

// GetAltControllerByMiscString returns all IDs for farmers where MiscSettingsString[key] == value.
func GetAltControllerByMiscString(key, value string) []string {
	FlushPendingSaves()
	results, err := queries.GetIdsByMiscString(ctx, GetIdsByMiscStringParams{
		Column1: sql.NullString{String: key, Valid: true},
		Value:   sql.NullString{String: value, Valid: true},
	})
	if err != nil {
		log.Println("GetIdsByMiscString:", err)
		return nil
	}
	return results
}

// FarmerExists returns true if a record for the given userID exists in farmer_state.
func FarmerExists(userID string) bool {
	FlushPendingSaves()
	_, err := queries.GetLegacyFarmerstate(ctx, userID)
	return err == nil
}

// AddGuildMembership adds a user to a guild. Returns true if added, false if already a member.
func AddGuildMembership(userID, guildID string) bool {
	FlushPendingSaves()
	n, err := queries.AddGuildMembership(ctx, AddGuildMembershipParams{UserID: userID, GuildID: guildID})
	if err != nil {
		log.Println("AddGuildMembership:", err)
	}
	return n > 0
}

// RemoveGuildMembership removes a user from a guild.
func RemoveGuildMembership(userID, guildID string) {
	FlushPendingSaves()
	err := queries.RemoveGuildMembership(ctx, RemoveGuildMembershipParams{UserID: userID, GuildID: guildID})
	if err != nil {
		log.Println("RemoveGuildMembership:", err)
	}
}

// GetGuildMembers returns all user IDs that are members of the given guild.
func GetGuildMembers(guildID string) []string {
	FlushPendingSaves()
	members, err := queries.GetGuildMembers(ctx, guildID)
	if err != nil {
		log.Println("GetGuildMembers:", err)
		return nil
	}
	return members
}

// GetUserGuilds returns all guild IDs the user belongs to.
func GetUserGuilds(userID string) []string {
	FlushPendingSaves()
	guilds, err := queries.GetUserGuilds(ctx, userID)
	if err != nil {
		log.Println("GetUserGuilds:", err)
		return nil
	}
	return guilds
}

// GetEiIgnsByGuild returns all ei_ign values for farmers who are members of the given guild.
func GetEiIgnsByGuild(guildID string) []string {
	FlushPendingSaves()
	results, err := queries.GetEiIgnsByGuild(ctx, guildID)
	if err != nil {
		log.Println("GetEiIgnsByGuild:", err)
		return nil
	}
	eiIgns := make([]string, 0, len(results))
	for _, r := range results {
		if s, ok := r.(string); ok && s != "" {
			eiIgns = append(eiIgns, s)
		}
	}
	return eiIgns
}

// GetDiscordUserIDFromEiIgn retrieves the Discord user ID based on the provided ei_ign.
// It also checks if the account is an alternate and returns the parent's Discord ID if so.
func GetDiscordUserIDFromEiIgn(eiIgn string) (string, error) {
	FlushPendingSaves()
	id, err := queries.GetUserIdFromEiIgn(ctx, sql.NullString{String: eiIgn, Valid: true})
	if err != nil {
		return "", err
	}
	// Check if this ID is an alternate of another user
	parentID := GetMiscSettingString(id, "AltController")
	if parentID != "" {
		return parentID, nil
	}
	return id, nil
}

// SetCustomBanner saves a user's custom banner PNG bytes into the database.
func SetCustomBanner(userID string, guildID string, imageData []byte) error {
	FlushPendingSaves()
	err := queries.UpsertCustomBanner(ctx, UpsertCustomBannerParams{
		UserID:    userID,
		GuildID:   guildID,
		ImageData: imageData,
	})
	if err != nil {
		log.Printf("Error saving custom banner for %s in guild %s: %v", userID, guildID, err)
		return err
	}
	return nil
}

// SyncCustomBanner checks if a custom banner exists in the database and is newer than the file on disk.
// If it is, it writes the image data from the database to the specified path.
func SyncCustomBanner(userID string, guildID string, destPath string) bool {
	FlushPendingSaves()
	banner, err := queries.GetCustomBanner(ctx, GetCustomBannerParams{UserID: userID, GuildID: guildID})
	if err != nil {
		_ = os.Remove(destPath) // Cleanup any lingering orphaned files
		return false
	}

	info, err := os.Stat(destPath)
	if err == nil {
		if !info.ModTime().Before(banner.UpdatedAt) {
			return true // File is up to date
		}
	}

	if err := os.WriteFile(destPath, banner.ImageData, 0644); err != nil {
		log.Printf("Failed to write custom banner to disk for %s: %v", userID, err)
		return false
	}

	_ = os.Chtimes(destPath, time.Now(), banner.UpdatedAt)
	return true
}

// RemoveCustomBanner deletes a user's custom banner from the database.
func RemoveCustomBanner(userID string, guildID string) error {
	FlushPendingSaves()
	err := queries.DeleteCustomBanner(ctx, DeleteCustomBannerParams{UserID: userID, GuildID: guildID})
	if err != nil {
		log.Printf("Error removing custom banner for %s in guild %s: %v", userID, guildID, err)
		return err
	}
	return nil
}

// InsertSuspectMission records a suspect mission anomaly to the database.
func InsertSuspectMission(arg InsertSuspectMissionParams) error {
	return queries.InsertSuspectMission(ctx, arg)
}
