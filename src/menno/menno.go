package menno

import (
	"context"
	"database/sql"
	_ "embed" // This is used to embed the schema.sql file
	"encoding/csv"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "modernc.org/sqlite" // Want this here
)

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
		populateData(true)
	} else {
		//if timestamp.AddDate(0, 1, 0).Before(time.Now()) {
		if timestamp.AddDate(0, 0, 0).Before(time.Now()) {
			populateData(false)
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
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, err
		}
		out, err := os.Create(csvPath)
		if err != nil {
			return nil, err
		}
		if _, err = io.Copy(out, resp.Body); err != nil {
			out.Close()
			return nil, err
		}
		out.Close()
	}

	// Open CSV.
	f, err := os.Open(csvPath)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// populateData loads menno.csv into the data table, downloading if missing.
func populateData(newData bool) {
	const csvPath = "ttbb-data/menno.csv"
	const url = "https://eggincdatacollection.azurewebsites.net/api/GetAllDataCsvCompact"

	rowCount := 0
	f, err := retrieveMennoData(csvPath, url)
	if err != nil {
		log.Printf("retrieveMennoData error: %v", err)
		return
	}
	defer f.Close()

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
			_, err = queries.InsertData(ctx, InsertDataParams{
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
			_, err = queries.UpdateData(ctx, UpdateDataParams{
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
		queries.CreateTimestamp(ctx)
	} else {
		queries.UpdateTimestamp(ctx)
	}
	// Remove the CSV file after processing.
	_ = os.Remove(csvPath)

	log.Printf("populateData: %d rows loaded", rowCount)
}
