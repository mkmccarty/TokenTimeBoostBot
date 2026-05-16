package leaderboard

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sort"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"
)

// ─── Leaderboard Registry ───────────────────────────────────────────────────

const (
	LBEarningsBonus  = "eb"
	LBSoulEggs       = "soul_eggs"
	LBProphecyEggs   = "prophecy_eggs"
	LBTerrorEvents   = "terror_events"
	LBEggsDelivered  = "eggs_delivered"
	LBContractExp    = "contract_exp"
	LBEggsTotal      = "eggs_total"
	LBStandardPermit = "std_permit"
	LBDrones         = "drones"
	LBEliteDrones    = "elite_drones"
	LBPrestiges      = "prestiges"
	LBSoulMirrors    = "soul_mirrors"
	LBVirtueShifts   = "virtue_shifts"
	LBTETotal        = "te_total"
	LBCXPWeeklyDelta = "cxp_weekly"

	LBTECuriosity  = "te_curiosity"
	LBTEIntegrity  = "te_integrity"
	LBTEHumility   = "te_humility"
	LBTEResilience = "te_resilience"
	LBTEKindness   = "te_kindness"

	LBEggsCuriosity  = "egg_curiosity"
	LBEggsIntegrity  = "egg_integrity"
	LBEggsHumility   = "egg_humility"
	LBEggsResilience = "egg_resilience"
	LBEggsKindness   = "egg_kindness"

	LBShipChicken1           = "ship_chicken1"
	LBShipChicken9           = "ship_chicken9"
	LBShipChickenHeavy       = "ship_chicken_heavy"
	LBShipBCR                = "ship_bcr"
	LBShipMilleniumChicken   = "ship_millenium_chicken"
	LBShipCorellihenCorvette = "ship_corvette"
	LBShipGaleggtica         = "ship_galeggtica"
	LBShipDefihent           = "ship_defihent"
	LBShipVoyegger           = "ship_voyegger"
	LBShipHenerprise         = "ship_henerprise"
	LBShipAtreggies          = "ship_atreggies"

	LBShipStdChicken1           = "std_ship_chicken1"
	LBShipStdChicken9           = "std_ship_chicken9"
	LBShipStdChickenHeavy       = "std_ship_chicken_heavy"
	LBShipStdBCR                = "std_ship_bcr"
	LBShipStdMilleniumChicken   = "std_ship_millenium_chicken"
	LBShipStdCorellihenCorvette = "std_ship_corvette"
	LBShipStdGaleggtica         = "std_ship_galeggtica"
	LBShipStdDefihent           = "std_ship_defihent"
	LBShipStdVoyegger           = "std_ship_voyegger"
	LBShipStdHenerprise         = "std_ship_henerprise"
	LBShipStdAtreggies          = "std_ship_atreggies"
)

// LBDef defines a single leaderboard metric.
type LBDef struct {
	Key            string
	DisplayName    string
	Description    string
	ValueFmt       string // "int", "float", "ei", "eb", "cxp"
	HigherIsBetter bool
	Source         LBSource
}

type LBSource int

const (
	SourceFirstContact LBSource = iota
	SourceContractArchive
	SourceBoth
)

// LBGroup defines a collection of leaderboard metrics that are posted together.
type LBGroup struct {
	Key         string
	DisplayName string
	Members     []string // Slice of LBDef.Key
}

// AllLeaderboards is the registry of all available individual leaderboard types.
var AllLeaderboards = []LBDef{
	{Key: LBSoulEggs, DisplayName: "Soul Eggs", Description: "Total soul eggs collected.", ValueFmt: "ei", HigherIsBetter: true, Source: SourceFirstContact},
	{Key: LBProphecyEggs, DisplayName: "Prophecy Eggs", Description: "Total eggs of prophecy collected.", ValueFmt: "int", HigherIsBetter: true, Source: SourceFirstContact},
	{Key: LBEarningsBonus, DisplayName: "Earnings Bonus", Description: "Nekkid and Dressed earnings bonus.", ValueFmt: "eb", HigherIsBetter: true, Source: SourceFirstContact},
	{Key: LBContractExp, DisplayName: "Contract XP", Description: "Total experience earned from contracts.", ValueFmt: "cxp", HigherIsBetter: true, Source: SourceBoth},
	{Key: LBVirtueShifts, DisplayName: "Virtue Shifts", Description: "Total virtue shifts completed.", ValueFmt: "int", HigherIsBetter: true, Source: SourceFirstContact},
	{Key: LBTETotal, DisplayName: "Total Truth Eggs", Description: "Sum of truth eggs across all virtues.", ValueFmt: "int", HigherIsBetter: true, Source: SourceFirstContact},
	{Key: LBDrones, DisplayName: "Drones", Description: "Total drones taken down.", ValueFmt: "int", HigherIsBetter: true, Source: SourceFirstContact},
	{Key: LBEliteDrones, DisplayName: "Elite Drones", Description: "Total elite drones taken down.", ValueFmt: "int", HigherIsBetter: true, Source: SourceFirstContact},
	{Key: LBPrestiges, DisplayName: "Prestiges", Description: "Total number of prestiges.", ValueFmt: "int", HigherIsBetter: true, Source: SourceFirstContact},
	{Key: LBSoulMirrors, DisplayName: "Soul Mirrors", Description: "Score based on soul mirror inventory (1, 2, 3 points).", ValueFmt: "int", HigherIsBetter: false, Source: SourceFirstContact},
	{Key: LBCXPWeeklyDelta, DisplayName: "Weekly CXP", Description: "CXP earned since last collection.", ValueFmt: "cxp", HigherIsBetter: true, Source: SourceContractArchive},
}

// AllGroups defines logical groupings for the UI and posting tasks.
var AllGroups = []LBGroup{
	{
		Key:         "group_core",
		DisplayName: "Core Stats",
		Members:     []string{LBSoulEggs, LBProphecyEggs, LBTETotal, LBEarningsBonus, LBVirtueShifts, LBContractExp, LBCXPWeeklyDelta},
	},
	{
		Key:         "group_misc",
		DisplayName: "Miscellaneous Stats",
		Members:     []string{LBPrestiges, LBDrones, LBEliteDrones, LBSoulMirrors},
	},
}

// Add ships leaderboards dynamically.
func init() {
	// Virtue eggs.
	virtueEggs := []struct {
		key          string
		name         string
		teKey        string
		deliveredKey string
	}{
		{"curiosity", "Curiosity", LBTECuriosity, LBEggsCuriosity},
		{"integrity", "Integrity", LBTEIntegrity, LBEggsIntegrity},
		{"humility", "Humility", LBTEHumility, LBEggsHumility},
		{"resilience", "Resilience", LBTEResilience, LBEggsResilience},
		{"kindness", "Kindness", LBTEKindness, LBEggsKindness},
	}

	teGroupMembers := []string{}
	deliveredGroupMembers := []string{}

	for _, e := range virtueEggs {
		AllLeaderboards = append(AllLeaderboards, LBDef{
			Key:            e.teKey,
			DisplayName:    e.name + " Truth Eggs",
			Description:    fmt.Sprintf("Truth eggs earned for %s virtue contracts.", e.name),
			ValueFmt:       "int",
			HigherIsBetter: true,
			Source:         SourceFirstContact,
		})
		teGroupMembers = append(teGroupMembers, e.teKey)

		AllLeaderboards = append(AllLeaderboards, LBDef{
			Key:            e.deliveredKey,
			DisplayName:    e.name + " Eggs Delivered",
			Description:    fmt.Sprintf("Raw eggs delivered for %s virtue contracts.", e.name),
			ValueFmt:       "ei",
			HigherIsBetter: true,
			Source:         SourceFirstContact,
		})
		deliveredGroupMembers = append(deliveredGroupMembers, e.deliveredKey)
	}

	AllGroups = append(AllGroups, LBGroup{
		Key:         "group_te",
		DisplayName: "Virtue Truth Eggs",
		Members:     teGroupMembers,
	}, LBGroup{
		Key:         "group_eggs_virtue",
		DisplayName: "Virtue Egg Delivery",
		Members:     deliveredGroupMembers,
	})

	// Add ship leaderboards using the static map in ei package.
	standardGroupMembers := []string{}
	virtueGroupShipsMembers := []string{}

	// Keys are int32 (ship ID), values are string (ship name)
	// We'll iterate in order 1-10 (skipping tutorial ship 0)
	for i := int32(1); i <= 10; i++ {
		name, ok := ei.ShipTypeName[i]
		if !ok {
			continue
		}

		stdKey := fmt.Sprintf("std_ship_%d", i)
		virtueKey := fmt.Sprintf("ship_%d", i)

		// Map specific IDs to our constants for compatibility
		switch i {
		case 0: // Skip tutorial
		case 1:
			stdKey = LBShipStdChicken1
			virtueKey = LBShipChicken1
		case 2:
			stdKey = LBShipStdChicken9
			virtueKey = LBShipChicken9
		case 3:
			stdKey = LBShipStdChickenHeavy
			virtueKey = LBShipChickenHeavy
		case 4:
			stdKey = LBShipStdBCR
			virtueKey = LBShipBCR
		case 5:
			stdKey = LBShipStdMilleniumChicken
			virtueKey = LBShipMilleniumChicken
		case 6:
			stdKey = LBShipStdCorellihenCorvette
			virtueKey = LBShipCorellihenCorvette
		case 7:
			stdKey = LBShipStdGaleggtica
			virtueKey = LBShipGaleggtica
		case 8:
			stdKey = LBShipStdDefihent
			virtueKey = LBShipDefihent
		case 9:
			stdKey = LBShipStdVoyegger
			virtueKey = LBShipStdVoyegger
		case 10:
			stdKey = LBShipStdHenerprise
			virtueKey = LBShipHenerprise
		case 11:
			stdKey = LBShipStdAtreggies
			virtueKey = LBShipAtreggies
		}

		AllLeaderboards = append(AllLeaderboards, LBDef{
			Key:            stdKey,
			DisplayName:    name + " Launches",
			Description:    fmt.Sprintf("Total launches for the %s.", name),
			ValueFmt:       "int",
			HigherIsBetter: true,
			Source:         SourceFirstContact,
		})
		standardGroupMembers = append(standardGroupMembers, stdKey)

		AllLeaderboards = append(AllLeaderboards, LBDef{
			Key:            virtueKey,
			DisplayName:    name + " Virtue Launches",
			Description:    fmt.Sprintf("Total virtue launches for the %s.", name),
			ValueFmt:       "int",
			HigherIsBetter: true,
			Source:         SourceFirstContact,
		})
		virtueGroupShipsMembers = append(virtueGroupShipsMembers, virtueKey)
	}

	AllGroups = append(AllGroups, LBGroup{
		Key:         "group_ships_std",
		DisplayName: "Standard Ship Launches",
		Members:     standardGroupMembers,
	}, LBGroup{
		Key:         "group_ships_virtue",
		DisplayName: "Virtue Ship Launches",
		Members:     virtueGroupShipsMembers,
	})
}

// legacyKeyAliases maps old/renamed config keys to their current registered equivalents.
var legacyKeyAliases = map[string]string{
	"cxp_weekly_delta":     LBCXPWeeklyDelta,
	"group_prestige_stats": "group_misc",
}

func resolveAlias(key string) string {
	if alias, ok := legacyKeyAliases[key]; ok {
		return alias
	}
	return key
}

// LBDefByKey looks up a definition by its unique key.
func LBDefByKey(key string) (LBDef, bool) {
	key = resolveAlias(key)
	for _, def := range AllLeaderboards {
		if def.Key == key {
			return def, true
		}
	}
	return LBDef{}, false
}

// GroupByKey looks up a group by its unique key.
func GroupByKey(key string) (LBGroup, bool) {
	key = resolveAlias(key)
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
	key = resolveAlias(key)
	if key == OptInAll {
		keys := make([]string, 0, len(AllLeaderboards))
		for _, def := range AllLeaderboards {
			keys = append(keys, def.Key)
		}
		return keys
	}
	if g, ok := GroupByKey(key); ok {
		return g.Members
	}
	return []string{key}
}

// IsValidConfigKey returns true if key is either an individual LBDef key or a
// group key — i.e. valid for use in /admin-lb admin set-channel.
func IsValidConfigKey(key string) bool {
	key = resolveAlias(key)
	if _, ok := LBDefByKey(key); ok {
		return true
	}
	_, ok := GroupByKey(key)
	return ok
}

// DisplayNameForConfigKey returns a human-readable name for any config key
// (individual or group).
func DisplayNameForConfigKey(key string) string {
	key = resolveAlias(key)
	if def, ok := LBDefByKey(key); ok {
		return def.DisplayName
	}
	if g, ok := GroupByKey(key); ok {
		return g.DisplayName
	}
	return key
}

// ─── Player opt-in helpers ────────────────────────────────────────────────────

const OptInAll = "all"

// PlayerIsOptedIn returns true if the user is opted into the given lb_type in the given guild.
func PlayerIsOptedIn(guildID, userID, lbType string) bool {
	if guildID == "" {
		return false
	}
	optins, err := farmerstate.GetLeaderboardOptInsForGuild(guildID)
	if err != nil {
		return false
	}
	for _, o := range optins {
		if o.UserID == userID && (resolveAlias(o.LbType) == resolveAlias(lbType) || o.LbType == OptInAll) {
			return true
		}
	}
	return false
}

// GetPlayerOptInTypes returns the slice of lb_type keys the user is opted into for a guild.
func GetPlayerOptInTypes(guildID, userID string) []string {
	if guildID == "" {
		return nil
	}
	optins, err := farmerstate.GetLeaderboardOptInsForUser(userID)
	if err != nil {
		return nil
	}
	var out []string
	isOptInAll := false
	for _, o := range optins {
		if o.GuildID == guildID {
			if o.LbType == OptInAll {
				isOptInAll = true
				break
			}
			out = append(out, o.LbType)
		}
	}
	if isOptInAll {
		keys := make([]string, 0, len(AllLeaderboards))
		for _, def := range AllLeaderboards {
			keys = append(keys, def.Key)
		}
		return keys
	}
	return out
}

// GetAllOptInUserIDs returns all user IDs who have opted into leaderboards in ANY guild.
func GetAllOptInUserIDs() []string {
	users, _ := farmerstate.GetLeaderboardOptInUsers()
	return users
}

// AddPlayerOptInTypes adds the given types to the player's opt-in list for a guild.
func AddPlayerOptInTypes(guildID, userID string, types []string) {
	for _, t := range types {
		_ = farmerstate.UpsertLeaderboardOptIn(guildID, userID, t)
	}
}

// RemovePlayerOptInTypes removes the given types from the player's opt-in list for a guild.
func RemovePlayerOptInTypes(guildID, userID string, types []string) {
	if len(types) == 1 && types[0] == OptInAll {
		_ = farmerstate.DeleteAllLeaderboardOptInsForUserInGuild(guildID, userID)
		return
	}
	for _, t := range types {
		_ = farmerstate.DeleteLeaderboardOptIn(guildID, userID, t)
	}
}

// GetUserOptInGuilds returns all guild IDs where the user has at least one opt-in.
func GetUserOptInGuilds(userID string) []string {
	optins, err := farmerstate.GetLeaderboardOptInsForUser(userID)
	if err != nil {
		return nil
	}
	guilds := make(map[string]struct{})
	for _, o := range optins {
		guilds[o.GuildID] = struct{}{}
	}
	var out []string
	for g := range guilds {
		out = append(out, g)
	}
	return out
}

// ─── Guild config helpers ─────────────────────────────────────────────────────

// LBConfig holds one row from the leaderboard_config table.
type LBConfig struct {
	LBType     string
	GuildID    string
	ChannelID  string
	MessageIDs []string // JSON-decoded message ID list
}

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

// GetLeaderboardRows returns all rows for a lb_type on a given snap_date for a guild, ranked by value.
func GetLeaderboardRows(lbType, snapDate, guildID string) []LBEntry {
	rows, err := farmerstate.GetLeaderboardForSnapDate(lbType, guildID, snapDate)
	if err != nil {
		log.Printf("leaderboard: GetLeaderboardRows %s/%s/%s: %v", lbType, guildID, snapDate, err)
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

// GetPreviousSnapDate returns the snap_date immediately before the given one.
func GetPreviousSnapDate(lbType, snapDate string) string {
	dates, err := farmerstate.GetLeaderboardSnapDates(lbType)
	if err != nil {
		return ""
	}
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

// GetPlayerStats retrieves the latest data for every leaderboard type for a given player in a guild.
func GetPlayerStats(guildID, playerID string) []PlayerStat {
	rows, err := farmerstate.GetStatsForPlayerInGuild(playerID, guildID)
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
				Rank:    -1, // Will be filled in by rankings logic
			}
			if g.previous != nil {
				stat.HasPrev = true
				stat.PrevVal = g.previous.Value
			}
			out = append(out, stat)
		}
	}
	return out
}
