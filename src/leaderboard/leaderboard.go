// Package leaderboard manages weekly farm stat leaderboards collected from the
// Egg Inc first-contact API (and contract archive for CXP data). Adding a new
// leaderboard type requires only:
//  1. Define a new key constant below.
//  2. Append a LBDef entry to AllLeaderboards.
//  3. Write the calculator function in leaderboard_calculators.go.
package leaderboard

import (
	"database/sql"
	"encoding/json"
	"log"
	"sort"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"
)

// ─── Leaderboard type keys ────────────────────────────────────────────────────
// Each key must be unique and stable across deployments (it is the DB primary key).
const (
	LBVirtueShifts = "virtue_shifts"

	LBTECuriosity  = "te_curiosity"
	LBTEIntegrity  = "te_integrity"
	LBTEHumility   = "te_humility"
	LBTEResilience = "te_resilience"
	LBTEKindness   = "te_kindness"
	LBTETotal      = "te_total"

	LBEggsCuriosity  = "eggs_curiosity"
	LBEggsIntegrity  = "eggs_integrity"
	LBEggsHumility   = "eggs_humility"
	LBEggsResilience = "eggs_resilience"
	LBEggsKindness   = "eggs_kindness"

	// VIRTUE missions only (MissionInfo_VIRTUE)
	LBShipChicken1           = "ship_chicken1"
	LBShipChicken9           = "ship_chicken9"
	LBShipChickenHeavy       = "ship_chickenheavy"
	LBShipBCR                = "ship_bcr"
	LBShipMilleniumChicken   = "ship_milleniumchicken"
	LBShipCorellihenCorvette = "ship_corellihencorvette"
	LBShipGaleggtica         = "ship_galeggtica"
	LBShipDefihent           = "ship_defihent"
	LBShipVoyegger           = "ship_voyegger"
	LBShipHenerprise         = "ship_henerprise"
	LBShipAtreggies          = "ship_atreggies"

	// Non-VIRTUE missions (all other mission types)
	LBShipStdChicken1           = "std_ship_chicken1"
	LBShipStdChicken9           = "std_ship_chicken9"
	LBShipStdChickenHeavy       = "std_ship_chickenheavy"
	LBShipStdBCR                = "std_ship_bcr"
	LBShipStdMilleniumChicken   = "std_ship_milleniumchicken"
	LBShipStdCorellihenCorvette = "std_ship_corellihencorvette"
	LBShipStdGaleggtica         = "std_ship_galeggtica"
	LBShipStdDefihent           = "std_ship_defihent"
	LBShipStdVoyegger           = "std_ship_voyegger"
	LBShipStdHenerprise         = "std_ship_henerprise"
	LBShipStdAtreggies          = "std_ship_atreggies"

	LBCXPWeeklyDelta = "cxp_weekly_delta"

	LBSoulEggs      = "soul_eggs"
	LBEarningsBonus = "earnings_bonus"
	LBDrones        = "drones"
	LBEliteDrones   = "elite_drones"
	LBPrestiges     = "prestiges"
	LBSoulMirrors   = "soul_mirrors"
)

// OptInAll is the sentinel value stored in farmerstate meaning "all leaderboards".
const OptInAll = "ALL"

// ─── Leaderboard definition ───────────────────────────────────────────────────

// DataSource indicates which Egg Inc API calls a leaderboard type requires.
type DataSource uint8

const (
	// SourceFirstContact requires only the first-contact backup.
	SourceFirstContact DataSource = iota
	// SourceContractArchive requires only the contract archive.
	SourceContractArchive
	// SourceBoth requires both calls.
	SourceBoth
)

// LBDef describes a single leaderboard type in the registry.
// To add a new type, append a LBDef to AllLeaderboards and add its calculator
// to leaderboard_calculators.go.
type LBDef struct {
	// Key is the stable DB identifier (one of the LB* constants above).
	Key string
	// DisplayName is shown in Discord messages and slash-command choices.
	DisplayName string
	// Description is shown in the /bock-leaderboard player optin choice list.
	Description string
	// Source controls which API calls are made during the weekly collection task.
	Source DataSource
	// HigherIsBetter controls sort order (true = DESC, false = ASC).
	HigherIsBetter bool
	// ValueFmt controls how the value column is formatted in leaderboard tables.
	// Options: "int", "float", "ei" (Egg Inc large number format), "cxp"
	ValueFmt string
}

// AllLeaderboards is the authoritative registry of all leaderboard types.
//
// ─── HOW TO ADD A NEW LEADERBOARD ────────────────────────────────────────────
//  1. Add a new key constant in the const block above (e.g. LBMyNewStat = "my_new_stat").
//  2. Append a LBDef{} entry to this slice.
//  3. Add a case for your key in the RunCalculators switch in leaderboard_calculators.go.
//
// ─────────────────────────────────────────────────────────────────────────────
var AllLeaderboards = []LBDef{
	// ── Virtue farm stats (first-contact only) ────────────────────────────────
	{LBVirtueShifts, "Virtue Shifts", "Total number of virtue egg shifts completed", SourceFirstContact, true, "int"},

	// ── Truth Eggs by virtue egg (integer tier count) ─────────────────────────
	{LBTECuriosity, "Curiosity Truth Eggs", "Truth Eggs earned from Curiosity egg", SourceFirstContact, true, "int"},
	{LBTEIntegrity, "Integrity Truth Eggs", "Truth Eggs earned from Integrity egg", SourceFirstContact, true, "int"},
	{LBTEHumility, "Humility Truth Eggs", "Truth Eggs earned from Humility egg", SourceFirstContact, true, "int"},
	{LBTEResilience, "Resilience Truth Eggs", "Truth Eggs earned from Resilience egg", SourceFirstContact, true, "int"},
	{LBTEKindness, "Kindness Truth Eggs", "Truth Eggs earned from Kindness egg", SourceFirstContact, true, "int"},
	{LBTETotal, "Total Truth Eggs", "Sum of Truth Eggs across all five virtue eggs", SourceFirstContact, true, "int"},

	// ── Raw eggs delivered per virtue egg (EI large-number format) ───────────
	{LBEggsCuriosity, "Curiosity Eggs Delivered", "Raw eggs delivered to Curiosity egg (detail: TE count)", SourceFirstContact, true, "ei"},
	{LBEggsIntegrity, "Integrity Eggs Delivered", "Raw eggs delivered to Integrity egg (detail: TE count)", SourceFirstContact, true, "ei"},
	{LBEggsHumility, "Humility Eggs Delivered", "Raw eggs delivered to Humility egg (detail: TE count)", SourceFirstContact, true, "ei"},
	{LBEggsResilience, "Resilience Eggs Delivered", "Raw eggs delivered to Resilience egg (detail: TE count)", SourceFirstContact, true, "ei"},
	{LBEggsKindness, "Kindness Eggs Delivered", "Raw eggs delivered to Kindness egg (detail: TE count)", SourceFirstContact, true, "ei"},

	// ── Ship launches — VIRTUE missions only ─────────────────────────────────
	{LBShipChicken1, "Chicken One Launches (Virtue)", "VIRTUE mission launches: Chicken One", SourceFirstContact, true, "int"},
	{LBShipChicken9, "Chicken Nine Launches (Virtue)", "VIRTUE mission launches: Chicken Nine", SourceFirstContact, true, "int"},
	{LBShipChickenHeavy, "Chicken Heavy Launches (Virtue)", "VIRTUE mission launches: Chicken Heavy", SourceFirstContact, true, "int"},
	{LBShipBCR, "BCR Launches (Virtue)", "VIRTUE mission launches: BCR", SourceFirstContact, true, "int"},
	{LBShipMilleniumChicken, "Quintillion Chicken Launches (Virtue)", "VIRTUE mission launches: Quintillion Chicken", SourceFirstContact, true, "int"},
	{LBShipCorellihenCorvette, "Cornish-Hen Corvette Launches (Virtue)", "VIRTUE mission launches: Cornish-Hen Corvette", SourceFirstContact, true, "int"},
	{LBShipGaleggtica, "Galeggtica Launches (Virtue)", "VIRTUE mission launches: Galeggtica", SourceFirstContact, true, "int"},
	{LBShipDefihent, "Defihent Launches (Virtue)", "VIRTUE mission launches: Defihent", SourceFirstContact, true, "int"},
	{LBShipVoyegger, "Voyegger Launches (Virtue)", "VIRTUE mission launches: Voyegger", SourceFirstContact, true, "int"},
	{LBShipHenerprise, "Henerprise Launches (Virtue)", "VIRTUE mission launches: Henerprise", SourceFirstContact, true, "int"},
	{LBShipAtreggies, "Atreggies Henliner Launches (Virtue)", "VIRTUE mission launches: Atreggies Henliner", SourceFirstContact, true, "int"},

	// ── Ship launches — standard (non-VIRTUE) missions ────────────────────────
	{LBShipStdChicken1, "Chicken One Launches (Standard)", "Standard mission launches: Chicken One", SourceFirstContact, true, "int"},
	{LBShipStdChicken9, "Chicken Nine Launches (Standard)", "Standard mission launches: Chicken Nine", SourceFirstContact, true, "int"},
	{LBShipStdChickenHeavy, "Chicken Heavy Launches (Standard)", "Standard mission launches: Chicken Heavy", SourceFirstContact, true, "int"},
	{LBShipStdBCR, "BCR Launches (Standard)", "Standard mission launches: BCR", SourceFirstContact, true, "int"},
	{LBShipStdMilleniumChicken, "Quintillion Chicken Launches (Standard)", "Standard mission launches: Quintillion Chicken", SourceFirstContact, true, "int"},
	{LBShipStdCorellihenCorvette, "Cornish-Hen Corvette Launches (Standard)", "Standard mission launches: Cornish-Hen Corvette", SourceFirstContact, true, "int"},
	{LBShipStdGaleggtica, "Galeggtica Launches (Standard)", "Standard mission launches: Galeggtica", SourceFirstContact, true, "int"},
	{LBShipStdDefihent, "Defihent Launches (Standard)", "Standard mission launches: Defihent", SourceFirstContact, true, "int"},
	{LBShipStdVoyegger, "Voyegger Launches (Standard)", "Standard mission launches: Voyegger", SourceFirstContact, true, "int"},
	{LBShipStdHenerprise, "Henerprise Launches (Standard)", "Standard mission launches: Henerprise", SourceFirstContact, true, "int"},
	{LBShipStdAtreggies, "Atreggies Henliner Launches (Standard)", "Standard mission launches: Atreggies Henliner", SourceFirstContact, true, "int"},

	// ── Contract score (requires contract archive) ────────────────────────────
	{LBCXPWeeklyDelta, "Weekly CXP Change", "Change in accumulated Contract Score (CXP) since last week", SourceContractArchive, true, "cxp"},

	// ── Prestige and Drone stats ──────────────────────────────────────────────
	{LBSoulEggs, "Soul Eggs", "Total Soul Egg count", SourceFirstContact, true, "ei"},
	{LBEarningsBonus, "Earnings Bonus", "Calculated Earnings Bonus (including TE bonus)", SourceFirstContact, true, "eb"},
	{LBDrones, "Drone Takedowns", "Total drones taken down", SourceFirstContact, true, "int"},
	{LBEliteDrones, "Elite Drone Takedowns", "Total elite drones taken down", SourceFirstContact, true, "int"},
	{LBPrestiges, "Prestige Count", "Total number of prestiges", SourceFirstContact, true, "int"},
	{LBSoulMirrors, "Soul Mirrors Score", "Weighted Soul Mirror count (1x Blue, 2x Purple, 3x Orange). Lower is better.", SourceFirstContact, false, "int"},
}

// LBDefByKey returns the LBDef for the given key, or false if not found.
func LBDefByKey(key string) (LBDef, bool) {
	for _, def := range AllLeaderboards {
		if def.Key == key {
			return def, true
		}
	}
	return LBDef{}, false
}

// ─── Leaderboard groups ───────────────────────────────────────────────────────
// Groups bundle related leaderboard types under a single channel config key.
// Admins configure channels using group keys (or individual keys).
// Player opt-ins always use individual type keys.
//
// To add a new group: append a LBGroup entry to AllGroups.

// LBGroup maps a single channel-config key to a set of individual LBDef keys.
type LBGroup struct {
	Key         string   // stable DB key (prefixed "group_")
	DisplayName string   // shown in autocomplete and /admin list
	Members     []string // individual LB type keys in display order
}

// AllGroups is the registry of channel-config groups.
var AllGroups = []LBGroup{
	{
		Key:         "group_te_virtue",
		DisplayName: "Truth Eggs per Virtue Egg (all 5 eggs)",
		Members: []string{
			LBTECuriosity, LBTEIntegrity, LBTEHumility,
			LBTEResilience, LBTEKindness,
		},
	},
	{
		Key:         "group_eggs_virtue",
		DisplayName: "Virtue Egg Deliveries (all 5 eggs)",
		Members: []string{
			LBEggsCuriosity, LBEggsIntegrity, LBEggsHumility,
			LBEggsResilience, LBEggsKindness,
		},
	},
	{
		Key:         "group_ships_virtue",
		DisplayName: "Virtue Ship Launches (all ships)",
		Members: []string{
			LBShipChicken1, LBShipChicken9, LBShipChickenHeavy,
			LBShipBCR, LBShipMilleniumChicken, LBShipCorellihenCorvette,
			LBShipGaleggtica, LBShipDefihent, LBShipVoyegger,
			LBShipHenerprise, LBShipAtreggies,
		},
	},
	{
		Key:         "group_ships_std",
		DisplayName: "Standard Ship Launches (all ships)",
		Members: []string{
			LBShipStdChicken1, LBShipStdChicken9, LBShipStdChickenHeavy,
			LBShipStdBCR, LBShipStdMilleniumChicken, LBShipStdCorellihenCorvette,
			LBShipStdGaleggtica, LBShipStdDefihent, LBShipStdVoyegger,
			LBShipStdHenerprise, LBShipStdAtreggies,
		},
	},
	{
		Key:         "group_prestige_stats",
		DisplayName: "Prestige & Drone Stats",
		Members: []string{
			LBSoulEggs, LBEarningsBonus, LBDrones, LBEliteDrones, LBPrestiges,
		},
	},
}

// GroupByKey returns the LBGroup for the given key, or false if not found.
func GroupByKey(key string) (LBGroup, bool) {
	for _, g := range AllGroups {
		if g.Key == key {
			return g, true
		}
	}
	return LBGroup{}, false
}

// ExpandConfigKey returns the individual lb_type keys that a channel config key
// resolves to. For group keys this is the group's Members slice; for individual
// type keys it's a single-element slice containing that key.
func ExpandConfigKey(key string) []string {
	if g, ok := GroupByKey(key); ok {
		return g.Members
	}
	return []string{key}
}

// IsValidConfigKey returns true if key is either an individual LBDef key or a
// group key — i.e. valid for use in /bock-leaderboard admin set-channel.
func IsValidConfigKey(key string) bool {
	if _, ok := LBDefByKey(key); ok {
		return true
	}
	_, ok := GroupByKey(key)
	return ok
}

// DisplayNameForConfigKey returns a human-readable name for any config key
// (individual or group).
func DisplayNameForConfigKey(key string) string {
	if def, ok := LBDefByKey(key); ok {
		return def.DisplayName
	}
	if g, ok := GroupByKey(key); ok {
		return g.DisplayName
	}
	return key
}

// ─── Player opt-in helpers ────────────────────────────────────────────────────

const optInKey = "leaderboard_optin"

// PlayerIsOptedIn returns true if the user is opted into the given lb_type.
func PlayerIsOptedIn(userID, lbType string) bool {
	val := farmerstate.GetMiscSettingString(userID, optInKey)
	if val == "" {
		return false
	}
	if val == OptInAll {
		return true
	}
	for _, t := range strings.Split(val, ",") {
		if strings.TrimSpace(t) == lbType {
			return true
		}
	}
	return false
}

// GetPlayerOptInTypes returns the slice of lb_type keys the user is opted into.
// Returns all keys when the stored value is OptInAll.
func GetPlayerOptInTypes(userID string) []string {
	val := farmerstate.GetMiscSettingString(userID, optInKey)
	if val == "" {
		return nil
	}
	if val == OptInAll {
		keys := make([]string, 0, len(AllLeaderboards))
		for _, def := range AllLeaderboards {
			keys = append(keys, def.Key)
		}
		return keys
	}
	parts := strings.Split(val, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// SetPlayerOptInTypes persists the opt-in list. Pass []string{OptInAll} for all.
func SetPlayerOptInTypes(userID string, types []string) {
	if len(types) == 0 {
		farmerstate.SetMiscSettingString(userID, optInKey, "")
		return
	}
	if len(types) == 1 && types[0] == OptInAll {
		farmerstate.SetMiscSettingString(userID, optInKey, OptInAll)
		return
	}
	farmerstate.SetMiscSettingString(userID, optInKey, strings.Join(types, ","))
}

// AddPlayerOptInTypes adds the given types to the player's opt-in list without
// removing existing ones.
func AddPlayerOptInTypes(userID string, types []string) {
	existing := GetPlayerOptInTypes(userID)
	existingVal := farmerstate.GetMiscSettingString(userID, optInKey)
	if existingVal == OptInAll {
		return // already in everything
	}
	seen := make(map[string]struct{}, len(existing))
	for _, t := range existing {
		seen[t] = struct{}{}
	}
	for _, t := range types {
		if t == OptInAll {
			farmerstate.SetMiscSettingString(userID, optInKey, OptInAll)
			return
		}
		seen[t] = struct{}{}
	}
	merged := make([]string, 0, len(seen))
	for t := range seen {
		merged = append(merged, t)
	}
	SetPlayerOptInTypes(userID, merged)
}

// RemovePlayerOptInTypes removes the given types from the player's opt-in list.
// Passing []string{OptInAll} clears the entire opt-in.
func RemovePlayerOptInTypes(userID string, types []string) {
	if len(types) == 1 && types[0] == OptInAll {
		SetPlayerOptInTypes(userID, nil)
		return
	}
	existing := GetPlayerOptInTypes(userID)
	remove := make(map[string]struct{}, len(types))
	for _, t := range types {
		remove[t] = struct{}{}
	}
	var kept []string
	for _, t := range existing {
		if _, ok := remove[t]; !ok {
			kept = append(kept, t)
		}
	}
	SetPlayerOptInTypes(userID, kept)
}

// ─── Guild config helpers ─────────────────────────────────────────────────────

// LBConfig holds one row from the leaderboard_config table.
type LBConfig struct {
	LBType     string
	GuildID    string
	ChannelID  string
	MessageIDs []string // JSON-decoded message ID list
}

// GuildQueries returns the guildstate sqlc Queries object via the package-level
// accessor (avoids re-exporting the internal db handle).
// We delegate to guildstate package functions to keep DB access encapsulated.

// UpsertGuildLBConfig saves or updates a leaderboard config row.
func UpsertGuildLBConfig(cfg LBConfig) error {
	msgJSON := "[]"
	if len(cfg.MessageIDs) > 0 {
		b, err := json.Marshal(cfg.MessageIDs)
		if err == nil {
			msgJSON = string(b)
		}
	}
	return guildstate.UpsertLeaderboardConfig(cfg.LBType, cfg.GuildID, cfg.ChannelID, msgJSON)
}

// GetGuildLBConfig retrieves a single config row.
func GetGuildLBConfig(guildID, lbType string) (*LBConfig, error) {
	row, err := guildstate.GetLeaderboardConfig(lbType, guildID)
	if err != nil {
		return nil, err
	}
	cfg := &LBConfig{
		LBType:    row.LbType,
		GuildID:   row.GuildID,
		ChannelID: row.ChannelID,
	}
	if row.MessageIds.Valid && row.MessageIds.String != "" {
		_ = json.Unmarshal([]byte(row.MessageIds.String), &cfg.MessageIDs)
	}
	return cfg, nil
}

// GetAllConfigs retrieves every configured leaderboard in the system.
func GetAllConfigs() ([]LBConfig, error) {
	rows, err := guildstate.GetAllLeaderboardConfigs()
	if err != nil {
		return nil, err
	}
	cfgs := make([]LBConfig, 0, len(rows))
	for _, row := range rows {
		cfg := LBConfig{
			LBType:    row.LbType,
			GuildID:   row.GuildID,
			ChannelID: row.ChannelID,
		}
		if row.MessageIds.Valid && row.MessageIds.String != "" {
			_ = json.Unmarshal([]byte(row.MessageIds.String), &cfg.MessageIDs)
		}
		cfgs = append(cfgs, cfg)
	}
	return cfgs, nil
}

// GetAllGuildLBConfigs retrieves all leaderboard configs for a guild.
func GetAllGuildLBConfigs(guildID string) ([]LBConfig, error) {
	rows, err := guildstate.GetAllLeaderboardConfigsForGuild(guildID)
	if err != nil {
		return nil, err
	}
	cfgs := make([]LBConfig, 0, len(rows))
	for _, row := range rows {
		cfg := LBConfig{
			LBType:    row.LbType,
			GuildID:   row.GuildID,
			ChannelID: row.ChannelID,
		}
		if row.MessageIds.Valid && row.MessageIds.String != "" {
			_ = json.Unmarshal([]byte(row.MessageIds.String), &cfg.MessageIDs)
		}
		cfgs = append(cfgs, cfg)
	}
	return cfgs, nil
}

// GetAllLBConfigs retrieves every leaderboard config (used by the weekly task).
func GetAllLBConfigs() ([]LBConfig, error) {
	rows, err := guildstate.GetAllLeaderboardConfigs()
	if err != nil {
		return nil, err
	}
	cfgs := make([]LBConfig, 0, len(rows))
	for _, row := range rows {
		cfg := LBConfig{
			LBType:    row.LbType,
			GuildID:   row.GuildID,
			ChannelID: row.ChannelID,
		}
		if row.MessageIds.Valid && row.MessageIds.String != "" {
			_ = json.Unmarshal([]byte(row.MessageIds.String), &cfg.MessageIDs)
		}
		cfgs = append(cfgs, cfg)
	}
	return cfgs, nil
}

// UpdateGuildLBConfigMessageIDs persists message IDs after a post run.
func UpdateGuildLBConfigMessageIDs(guildID, lbType string, messageIDs []string) {
	b, err := json.Marshal(messageIDs)
	if err != nil {
		log.Printf("leaderboard: failed to marshal message IDs: %v", err)
		return
	}
	if err := guildstate.UpdateLeaderboardConfigMessageIDs(string(b), lbType, guildID); err != nil {
		log.Printf("leaderboard: failed to update message IDs for %s/%s: %v", guildID, lbType, err)
	}
}

// DeleteGuildLBConfig removes a leaderboard config row.
func DeleteGuildLBConfig(guildID, lbType string) error {
	return guildstate.DeleteLeaderboardConfig(lbType, guildID)
}

// ─── Leaderboard stat helpers (farmerstate) ───────────────────────────────────

// LBEntry is a collected data row for one player on one leaderboard.
type LBEntry struct {
	LBType   string
	Player   string // Discord user ID
	GameName string // Egg Inc in-game name
	SnapDate string // ISO date "YYYY-MM-DD"
	Value    float64
	Details  string // human-readable extra info
}

// SaveLBEntry persists one leaderboard stat row.
func SaveLBEntry(e LBEntry) {
	if err := farmerstate.UpsertLeaderboardStat(e.LBType, e.Player, e.GameName, e.SnapDate, e.Value, sql.NullString{String: e.Details, Valid: e.Details != ""}); err != nil {
		log.Printf("leaderboard: save stat %s/%s: %v", e.LBType, e.Player, err)
	}
}

// GetLatestSnapDate returns the most recent snap_date for a lb_type, or "".
func GetLatestSnapDate(lbType string) string {
	date, err := farmerstate.GetLatestLeaderboardSnapDate(lbType)
	if err != nil {
		return ""
	}
	return date
}

// GetPriorStatForPlayer returns the most recent stored stat for a player+lbType.
// Returns nil if none found.
func GetPriorStatForPlayer(lbType, playerID string) *LBEntry {
	row, err := farmerstate.GetLeaderboardStatForPlayer(lbType, playerID)
	if err != nil {
		return nil
	}
	e := &LBEntry{
		LBType:   lbType,
		Player:   row.Player,
		GameName: row.GameName,
		SnapDate: row.SnapDate,
		Value:    row.Value,
	}
	if row.Details.Valid {
		e.Details = row.Details.String
	}
	return e
}

// GetLeaderboardRows returns all rows for a lb_type on a given snap_date, ranked by value.
func GetLeaderboardRows(lbType, snapDate string) []LBEntry {
	rows, err := farmerstate.GetLeaderboardForSnapDate(lbType, snapDate)
	if err != nil {
		log.Printf("leaderboard: GetLeaderboardRows %s/%s: %v", lbType, snapDate, err)
		return nil
	}
	out := make([]LBEntry, 0, len(rows))
	for _, r := range rows {
		e := LBEntry{
			LBType:   lbType,
			Player:   r.Player,
			GameName: r.GameName,
			SnapDate: snapDate,
			Value:    r.Value,
		}
		if r.Details.Valid {
			e.Details = r.Details.String
		}
		out = append(out, e)
	}

	// Sort based on definition (DB usually returns DESC, but we might want ASC).
	def, ok := LBDefByKey(lbType)
	if ok && !def.HigherIsBetter {
		sort.Slice(out, func(i, j int) bool {
			return out[i].Value < out[j].Value
		})
	}

	return out
}

// GetAllOptInUserIDs returns all Discord user IDs with any leaderboard opt-in.
func GetAllOptInUserIDs() []string {
	ids, err := farmerstate.GetLeaderboardOptInUsers()
	if err != nil {
		log.Printf("leaderboard: GetAllOptInUserIDs: %v", err)
		return nil
	}
	return ids
}

// GetPreviousSnapDate returns the snap_date immediately preceding the given date for a lb_type.
func GetPreviousSnapDate(lbType, snapDate string) string {
	dates, err := farmerstate.GetLeaderboardSnapDates(lbType)
	if err != nil {
		return ""
	}
	// dates are newest first.
	for i, d := range dates {
		if d == snapDate && i+1 < len(dates) {
			return dates[i+1]
		}
	}
	return ""
}
// PlayerStat holds a single metric's latest value and its previous week's value for comparison.
type PlayerStat struct {
	Def     LBDef
	Current LBEntry
	HasPrev bool
	PrevVal float64
	Rank    int
}

// GetPlayerStats retrieves the latest data for every leaderboard type for a given player.
func GetPlayerStats(playerID string) []PlayerStat {
	rows, err := farmerstate.GetStatsForPlayer(playerID)
	if err != nil {
		return nil
	}

	// Group rows by lb_type. Rows are ordered by lb_type ASC, snap_date DESC.
	type lbGroup struct {
		current  *LBEntry
		previous *LBEntry
	}
	groups := make(map[string]*lbGroup)

	for _, r := range rows {
		g, ok := groups[r.LbType]
		if !ok {
			g = &lbGroup{}
			groups[r.LbType] = g
		}

		entry := &LBEntry{
			LBType:   r.LbType,
			Player:   r.Player,
			GameName: r.GameName,
			SnapDate: r.SnapDate,
			Value:    r.Value,
		}
		if r.Details.Valid {
			entry.Details = r.Details.String
		}

		if g.current == nil {
			g.current = entry
		} else if g.previous == nil {
			g.previous = entry
		}
	}

	var out []PlayerStat
	// Build output list using the registry order (AllLeaderboards).
	for _, def := range AllLeaderboards {
		if g, ok := groups[def.Key]; ok && g.current != nil {
			stat := PlayerStat{
				Def:     def,
				Current: *g.current,
			}
			if g.previous != nil {
				stat.HasPrev = true
				stat.PrevVal = g.previous.Value
			}

			// Calculate rank
			rows := GetLeaderboardRows(def.Key, g.current.SnapDate)
			for i, r := range rows {
				if r.Player == playerID {
					stat.Rank = i + 1
					break
				}
			}

			out = append(out, stat)
		}
	}
	return out
}
