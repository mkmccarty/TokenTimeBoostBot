package guildstate

import (
	"context"
	"database/sql"
	_ "embed" // Required for go:embed.
	"encoding/json"
	"log"
	"sort"
	"time"

	_ "modernc.org/sqlite" // SQLite driver registration.
)

// GuildState stores guild-scoped bot settings.
type GuildState struct {
	GuildID            string
	MiscSettingsFlag   map[string]bool
	MiscSettingsString map[string]string
	LastUpdated        time.Time
}

var (
	ctx        = context.Background()
	queries    *Queries
	guildstate map[string]*GuildState
)

//go:embed schema.sql
var ddl string

func sqliteInit() {
	if queries != nil {
		return
	}

	db, _ := sql.Open("sqlite", "ttbb-data/Guildstate.sqlite?_busy_timeout=5000")
	_, _ = db.ExecContext(ctx, ddl)
	queries = New(db)
}

func init() {
	sqliteInit()
	guildstate = make(map[string]*GuildState)

}

func newGuild(guildID string) {
	sqliteGuild, err := queries.GetGuildState(ctx, guildID)
	if err == nil && sqliteGuild.Value.Valid {
		var guild GuildState
		err = json.Unmarshal([]byte(sqliteGuild.Value.String), &guild)
		if err == nil {
			guildstate[guildID] = &guild
			return
		}
	}

	guildstate[guildID] = &GuildState{GuildID: guildID}
}

func saveGuildSqliteData(guildID string, guild *GuildState) error {
	guild.LastUpdated = time.Now()
	guildJSON, err := json.Marshal(guild)
	if err != nil {
		return err
	}

	rows, err := queries.UpdateGuildState(ctx, UpdateGuildStateParams{
		Value: sql.NullString{String: string(guildJSON), Valid: true},
		ID:    guildID,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		_, err = queries.InsertGuildState(ctx, InsertGuildStateParams{
			ID:    guildID,
			Value: sql.NullString{String: string(guildJSON), Valid: true},
		})
	}
	return err
}

// GetGuildState returns persisted settings for a guild.
func GetGuildState(guildID string) (*GuildState, error) {
	row, err := queries.GetGuildState(ctx, guildID)
	if err != nil {
		return nil, err
	}

	guild := &GuildState{GuildID: guildID}
	if row.Value.Valid && row.Value.String != "" {
		if err := json.Unmarshal([]byte(row.Value.String), guild); err != nil {
			return nil, err
		}
		if guild.GuildID == "" {
			guild.GuildID = guildID
		}
	}
	return guild, nil
}

// UpsertGuildState inserts or updates guild settings by GuildID.
func UpsertGuildState(guild *GuildState) error {
	if guild == nil {
		return nil
	}
	return saveGuildSqliteData(guild.GuildID, guild)
}

// GetAllGuildState returns all persisted guild settings.
func GetAllGuildState() ([]GuildState, error) {
	rows, err := queries.GetAllGuildState(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]GuildState, 0, len(rows))
	for _, row := range rows {
		guild := GuildState{GuildID: row.ID}
		if row.Value.Valid && row.Value.String != "" {
			if err := json.Unmarshal([]byte(row.Value.String), &guild); err != nil {
				return nil, err
			}
			if guild.GuildID == "" {
				guild.GuildID = row.ID
			}
		}
		items = append(items, guild)
	}
	return items, nil
}

// GetAllGuildIDs returns all persisted guild IDs sorted ascending.
func GetAllGuildIDs() ([]string, error) {
	items, err := GetAllGuildState()
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.GuildID == "" {
			continue
		}
		if _, ok := seen[item.GuildID]; ok {
			continue
		}
		seen[item.GuildID] = struct{}{}
		ids = append(ids, item.GuildID)
	}

	sort.Strings(ids)
	return ids, nil
}

// SetGuildSettingFlag sets a boolean guild setting and persists it.
func SetGuildSettingFlag(guildID string, key string, value bool) {
	if guildstate[guildID] == nil {
		newGuild(guildID)
	}

	if guildstate[guildID].MiscSettingsFlag == nil {
		guildstate[guildID].MiscSettingsFlag = make(map[string]bool)
	}
	guildstate[guildID].MiscSettingsFlag[key] = value
	if err := saveGuildSqliteData(guildID, guildstate[guildID]); err != nil {
		log.Printf("error saving guild data: %v", err)
	}
}

// GetGuildSettingFlag gets a boolean guild setting.
func GetGuildSettingFlag(guildID string, key string) bool {
	if guildstate[guildID] == nil {
		newGuild(guildID)
	}

	if guild, ok := guildstate[guildID]; ok {
		if guild.MiscSettingsFlag == nil {
			guild.MiscSettingsFlag = make(map[string]bool)
		}
		return guild.MiscSettingsFlag[key]
	}
	return false
}

// SetGuildSettingString sets a string guild setting and persists it.
func SetGuildSettingString(guildID string, key string, value string) {
	if guildstate[guildID] == nil {
		newGuild(guildID)
	}

	if guildstate[guildID].MiscSettingsString == nil {
		guildstate[guildID].MiscSettingsString = make(map[string]string)
	}

	if value == "" {
		delete(guildstate[guildID].MiscSettingsString, key)
	} else if guildstate[guildID].MiscSettingsString[key] != value {
		guildstate[guildID].MiscSettingsString[key] = value
	}

	if err := saveGuildSqliteData(guildID, guildstate[guildID]); err != nil {
		log.Printf("error saving guild data: %v", err)
	}
}

// GetGuildSettingString gets a string guild setting.
func GetGuildSettingString(guildID string, key string) string {
	if guildstate[guildID] == nil {
		newGuild(guildID)
	}

	if guild, ok := guildstate[guildID]; ok {
		if guild.MiscSettingsString == nil {
			guild.MiscSettingsString = make(map[string]string)
		}
		return guild.MiscSettingsString[key]
	}
	return ""
}

// DeleteGuildState deletes persisted state for a guild.
func DeleteGuildState(guildID string) error {
	delete(guildstate, guildID)
	return queries.DeleteGuildState(ctx, guildID)
}

// DeleteGuildRecords deletes guild records for a guild ID.
func DeleteGuildRecords(guildID string) error {
	delete(guildstate, guildID)
	return queries.DeleteGuildRecords(ctx, guildID)
}
