package menno

import (
	"context"
	"database/sql"
	_ "embed" // This is used to embed the schema.sql file
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	_ "modernc.org/sqlite" // Want this here
)

// From https://github.com/menno-egginc/eggincdatacollection-docs/blob/main/DataEndpoints.md

var ctx = context.Background()

//go:embed schema.sql
var ddl string
var queries *Queries

func parseNullInt64(s string) sql.NullInt64 {
	if s == "" {
		return sql.NullInt64{Valid: false}
	}
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: val, Valid: true}
}

func sqliteInit() {
	db, _ := sql.Open("sqlite", "ttbb-data/menno.sqlite?_busy_timeout=5000")

	_, _ = db.ExecContext(ctx, ddl)
	queries = New(db)
}

// Startup initializes the Menno module.
func Startup() {
	sqliteInit()

	// first time?
	timestamp, err := queries.GetTimestamp(ctx)
	if err != nil {
		populateData(true, time.Now())
	} else {
		if true {
			if timestamp.AddDate(0, 0, 14).Before(time.Now()) {
				populateData(false, time.Now())
			}
		} else {
			// This is a temporary measure to force an update until the data source is more reliable.
			if timestamp.AddDate(0, 0, 0).Before(time.Now()) {
				populateData(false, timestamp)
			}
		}
	}
}

func retrieveMennoData(csvPath, url string) (*os.File, error) {

	// Ensure file exists (download if not).
	if _, err := os.Stat(csvPath); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll("ttbb-data", 0755); err != nil {
			log.Printf("mkdir error: %v", err)
			return nil, err
		}
		resp, err := http.Get(url)
		if err != nil {
			log.Printf("download error: %v", err)
			return nil, err
		}

		defer func() {
			if err := resp.Body.Close(); err != nil {
				// Handle the error appropriately, e.g., logging or taking corrective actions
				log.Printf("Failed to close: %v", err)
			}
		}()
		if resp.StatusCode != http.StatusOK {
			return nil, err
		}
		out, err := os.Create(csvPath)
		if err != nil {
			return nil, err
		}
		if _, err = io.Copy(out, resp.Body); err != nil {
			if err := out.Close(); err != nil {
				log.Printf("close error: %v", err)
			}
			return nil, err
		}
		if err := out.Close(); err != nil {
			log.Printf("close error: %v", err)
		}
	}

	// Open CSV.
	f, err := os.Open(csvPath)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// populateData loads menno.csv into the data table, downloading if missing.
func populateData(newData bool, timestamp time.Time) {
	const csvPathTemplate = "ttbb-data/menno-%s.csv"
	const url = "https://eggincdatacollection.azurewebsites.net/api/GetAllDataCsvCompact"

	if newData {
		// Clear out existing data.
		_ = queries.DeleteData(ctx)
	}
	// Construct the csvPath so it includes the current date (YYYYMMDD).
	currentDate := timestamp.Format("20060102")
	csvPath := fmt.Sprintf(csvPathTemplate, currentDate)

	rowCount := 0
	f, err := retrieveMennoData(csvPath, url)
	if err != nil {
		log.Printf("retrieveMennoData error: %v", err)
		return
	}

	defer func() {
		if err := f.Close(); err != nil {
			// Handle the error appropriately, e.g., logging or taking corrective actions
			log.Printf("Failed to close: %v", err)
		}
	}()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		log.Printf("read header error: %v", err)
		return
	}
	if len(header) == 0 {
		log.Printf("empty header")
		return
	}

	tx, err := queries.db.(*sql.DB).BeginTx(ctx, nil)
	if err != nil {
		log.Printf("begin tx error: %v", err)
		return
	}
	qtx := queries.WithTx(tx)
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Printf("tx rollback error: %v", rbErr)
			}
			return
		}
		if cmErr := tx.Commit(); cmErr != nil {
			log.Printf("tx commit error: %v", cmErr)
		}
	}()

	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("read record error: %v", err)
			return
		}

		if newData {
			err = qtx.InsertData(ctx, InsertDataParams{
				ShipTypeID:         parseNullInt64(rec[0]),
				ShipDurationTypeID: parseNullInt64(rec[1]),
				ShipLevel:          parseNullInt64(rec[2]),
				TargetArtifactID:   parseNullInt64(rec[3]),
				ArtifactTypeID:     parseNullInt64(rec[4]),
				ArtifactRarityID:   parseNullInt64(rec[5]),
				ArtifactTier:       parseNullInt64(rec[6]),
				TotalDrops:         parseNullInt64(rec[7]),
				MissionType:        parseNullInt64(rec[8]),
			})
			if err != nil {
				log.Printf("insert error: %v", err)
				//return
			}
		} else {
			err = qtx.UpdateData(ctx, UpdateDataParams{
				ShipTypeID:         parseNullInt64(rec[0]),
				ShipDurationTypeID: parseNullInt64(rec[1]),
				ShipLevel:          parseNullInt64(rec[2]),
				TargetArtifactID:   parseNullInt64(rec[3]),
				ArtifactTypeID:     parseNullInt64(rec[4]),
				ArtifactRarityID:   parseNullInt64(rec[5]),
				ArtifactTier:       parseNullInt64(rec[6]),
				TotalDrops:         parseNullInt64(rec[7]),
				MissionType:        parseNullInt64(rec[8]),
			})
			if err != nil {
				log.Printf("update error: %v", err)
				//return
			}
		}

		rowCount++
	}

	if newData {
		_ = queries.CreateTimestamp(ctx)
	} else {
		_ = queries.UpdateTimestamp(ctx)
	}

	if !config.IsDevBot() {
		// Remove the CSV file after processing.
		_ = os.Remove(csvPath)
	}

	//PrintDropData(ei.MissionInfo_VOYEGGER, ei.MissionInfo_SHORT, 2, ei.ArtifactSpec_INTERSTELLAR_COMPASS)

	fmt.Printf("populateData: %d rows loaded", rowCount)
}

// GetShipDropData retrieves and logs drop data for a specific ship configuration.
func GetShipDropData(shipType ei.MissionInfo_Spaceship, duration ei.MissionInfo_DurationType, level int, artifactType ei.ArtifactSpec_Name) []GetDropsRow {
	rows, err := queries.GetDrops(ctx, GetDropsParams{
		ShipTypeID:         sql.NullInt64{Int64: int64(shipType), Valid: true},
		ShipDurationTypeID: sql.NullInt64{Int64: int64(duration), Valid: true},
		ShipLevel:          sql.NullInt64{Int64: int64(level), Valid: true},
		ArtifactTypeID:     sql.NullInt64{Int64: int64(artifactType), Valid: true},
	})
	if err != nil {
		log.Printf("GetShipDropData GetDrops error: %v", err)
		return nil
	}

	return rows
}

func asFloat64(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int64:
		return float64(t)
	case int32:
		return float64(t)
	case int:
		return float64(t)
	case uint64:
		return float64(t)
	case uint32:
		return float64(t)
	case uint:
		return float64(t)
	case []byte:
		f, _ := strconv.ParseFloat(string(t), 64)
		return f
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	case sql.NullFloat64:
		if t.Valid {
			return t.Float64
		}
	case sql.NullInt64:
		if t.Valid {
			return float64(t.Int64)
		}
	}
	return 0
}

// PrintDropData retrieves and logs drop data for a specific ship configuration.
func PrintDropData(ship ei.MissionInfo_Spaceship, duration ei.MissionInfo_DurationType, stars int, target ei.ArtifactSpec_Name) string {
	var output strings.Builder
	var rows []GetDropsRow
	// Cap the ship stars to the max for the ship
	stars = min(stars, int(ei.ShipMaxStars[int32(ship)]))

	artifactName := ei.ArtifactTypeName[int32(target)]
	// If this contains Stone then I want the ID's for both Stone and Fragment
	if strings.Contains(artifactName, " Stone") {
		stoneID := int32(-1)
		fragmentID := int32(-1)
		for id, name := range ei.ArtifactTypeName {
			if name == artifactName {
				stoneID = id
			}
			if name == artifactName+" Fragment" {
				fragmentID = id
			}
		}
		rows = append(rows, GetShipDropData(ship, duration, stars, ei.ArtifactSpec_Name(stoneID))...)
		rows = append(rows, GetShipDropData(ship, duration, stars, ei.ArtifactSpec_Name(fragmentID))...)
		sort.Slice(rows, func(i, j int) bool {
			return asFloat64(rows[i].DropRate) > asFloat64(rows[j].DropRate)
		})

	} else {
		rows = GetShipDropData(ship, duration, stars, target)
	}

	// Does the  contain " Stone"?
	// If so then I want the Stone and Fragment ID's

	// if ei.ArtifactTypeName[int32(target)]

	var tier1 strings.Builder
	var tier2 strings.Builder
	var tier3 strings.Builder
	var tier4 strings.Builder
	tierRarityCounts := make(map[string]int) // key: tier|rarityID

	for _, row := range rows {
		allDropsValue := row.AllDropsValue.(int64)
		ratio := float64(row.TotalDrops.Int64) / float64(allDropsValue)

		targetArtifact := ei.ArtifactTypeName[int32(row.TargetArtifactID.Int64)]
		//returnArtifact := ei.ArtifactSpec_Name_name[int32(row.ArtifactTypeID.Int64)]
		rf := ei.ArtifactSpec_Rarity_name[int32(row.ArtifactRarityID.Int64)]

		rarity := ""
		if len(rf) > 0 {
			rarity = rf[:1]
		}

		tier := row.ArtifactTier.Int64 + 1
		rarityID := row.ArtifactRarityID.Int64
		key := fmt.Sprintf("%d|%d", tier, rarityID)

		// Limit to 4 entries per (tier, rarity)
		if tierRarityCounts[key] >= 4 {
			continue
		}
		tierRarityCounts[key]++

		var tierOutput *strings.Builder
		switch tier {
		case 1:
			tierOutput = &tier1
		case 2:
			tierOutput = &tier2
		case 3:
			tierOutput = &tier3
		case 4:
			tierOutput = &tier4
		default:
			continue
		}

		fmt.Fprintf(tierOutput, "Target: %s: %f - T%d%s\n", targetArtifact, ratio, tier, rarity)
	}

	shipName := ei.ShipTypeName[int32(ship)]
	targetName := ei.ArtifactTypeName[int32(target)]

	starsStr := strings.Repeat("⭐️", stars)
	fmt.Fprintf(&output, "%s %s %s hunting for %s\n\n", ei.DurationTypeName[int32(duration)], shipName, starsStr, targetName)
	if len(tier4.String()) != 0 {
		fmt.Fprintf(&output, "=== Tier 4 ===\n%s\n", tier4.String())
	}
	if len(tier3.String()) != 0 {
		fmt.Fprintf(&output, "=== Tier 3 ===\n%s\n", tier3.String())
	}
	if len(tier2.String()) != 0 {
		fmt.Fprintf(&output, "=== Tier 2 ===\n%s\n", tier2.String())
	}
	if len(tier1.String()) != 0 {
		fmt.Fprintf(&output, "=== Tier 1 ===\n%s\n", tier1.String())
	}

	fmt.Fprintf(&output, "-# This tool is made for RAIYC and is still under development. Data may be incomplete or inaccurate.\n")
	return output.String()
}
