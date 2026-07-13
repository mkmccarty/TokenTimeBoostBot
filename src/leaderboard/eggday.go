package leaderboard

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/events"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

// GetStatForPlayerAndSnapDate returns the stored stat for a player + lbType + snapDate.
func GetStatForPlayerAndSnapDate(lbType, playerID, snapDate string) *LBEntry {
	row, err := farmerstate.GetLeaderboardStatForPlayerAndSnapDate(lbType, playerID, snapDate)
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

// StartEggDayScheduler schedules the Egg Day start and end collections.
func StartEggDayScheduler(s *discordgo.Session) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		log.Printf("eggday: error loading timezone America/Los_Angeles: %v", err)
		return
	}

	go func() {
		for {
			now := time.Now().In(loc)
			year := now.Year()

			// Egg Day is July 14th.
			// Target start time: July 14th of the current year at 8:55 AM PT.
			startTime := time.Date(year, time.July, 14, 8, 55, 0, 0, loc)

			// If it's already past startTime for this year, schedule for next year.
			if now.After(startTime) {
				startTime = time.Date(year+1, time.July, 14, 8, 55, 0, 0, loc)
			}

			sleepDuration := time.Until(startTime)
			log.Printf("eggday: scheduler waiting until %s (in %v)", startTime.Format(time.RFC3339), sleepDuration)
			time.Sleep(sleepDuration)

			// Re-evaluate year when we wake up
			runYear := time.Now().In(loc).Year()
			log.Printf("eggday: starting Egg Day collection for year %d", runYear)

			// 1. Run start collection
			CollectEggDayStart(s, runYear)

			// 2. Wait 10 minutes (until 9:05 AM PT) to let periodicals update, then determine event end time
			time.Sleep(10 * time.Minute)

			endTime := determineEggDayEndTime(s, runYear, loc)
			log.Printf("eggday: determined end time: %s", endTime.Format(time.RFC3339))

			// Sleep until end time (or slightly after, e.g. 10 minutes after)
			sleepUntilEnd := time.Until(endTime.Add(10 * time.Minute))
			if sleepUntilEnd > 0 {
				log.Printf("eggday: sleeping until 10 minutes after end time: %s", endTime.Add(10*time.Minute).Format(time.RFC3339))
				time.Sleep(sleepUntilEnd)
			}

			// 3. Run end collection, calculate gains/pct, and post leaderboards
			CollectEggDayEndAndCalculate(s, runYear, false)
		}
	}()
}

// determineEggDayEndTime polls the active events to find the end time of Egg Day events.
func determineEggDayEndTime(s *discordgo.Session, year int, loc *time.Location) time.Time {
	// Default to 24 hours later (July 15th 9:00 AM PT)
	defaultEndTime := time.Date(year, time.July, 15, 9, 0, 0, 0, loc)

	// Poll up to 6 times, once every 5 minutes
	for attempt := 0; attempt < 6; attempt++ {
		// Active call to download the new periodicals from the API to guarantee updated events
		_ = events.GetPeriodicalsFromAPI(s)

		ei.EventMutex.Lock()
		eventsCopy := make([]ei.EggEvent, len(ei.EggIncEvents))
		copy(eventsCopy, ei.EggIncEvents)
		ei.EventMutex.Unlock()

		var maxEndTime time.Time
		foundEggDayEvent := false

		targetStartLower := time.Date(year, time.July, 14, 8, 50, 0, 0, loc)
		targetStartUpper := time.Date(year, time.July, 14, 9, 10, 0, 0, loc)

		for _, ev := range eventsCopy {
			if ev.EventType == "prestige-boost" && ev.StartTime.After(targetStartLower) && ev.StartTime.Before(targetStartUpper) {
				foundEggDayEvent = true
				if ev.EndTime.After(maxEndTime) {
					maxEndTime = ev.EndTime
				}
			}
		}

		if foundEggDayEvent && !maxEndTime.IsZero() {
			log.Printf("eggday: found Egg Day event(s), max end time is %s", maxEndTime.In(loc).Format(time.RFC3339))
			return maxEndTime.In(loc)
		}

		log.Printf("eggday: Egg Day events not found in periodicals yet (attempt %d/6), waiting...", attempt+1)
		time.Sleep(5 * time.Minute)
	}

	log.Printf("eggday: falling back to default end time: %s", defaultEndTime.Format(time.RFC3339))
	return defaultEndTime
}

// CollectEggDayManual runs the manual collection flow. It determines whether to perform starting collection
// or ending collection/calculation based on current date & database presence.
func CollectEggDayManual(s *discordgo.Session, target string, dryRun bool, onProgress func(string)) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		if onProgress != nil {
			onProgress("❌ Error loading timezone America/Los_Angeles")
		}
		return
	}

	now := time.Now().In(loc)
	year := now.Year()
	yearStr := fmt.Sprintf("%d", year)

	// Check if any player already has a start stat for this year
	hasStart := false
	userIDs := GetAllOptInUserIDs()
	for _, u := range userIDs {
		if GetStatForPlayerAndSnapDate("egg_day_se_start", u, yearStr) != nil {
			hasStart = true
			break
		}
	}

	// We collect start stats if we are on July 14th (or earlier) and don't have start stats yet.
	// Otherwise, we collect end stats and calculate.
	isStartPhase := !hasStart
	if now.Month() == time.July && now.Day() == 14 && now.Hour() < 12 {
		isStartPhase = true
	}

	if isStartPhase {
		if onProgress != nil {
			onProgress("🚀 Running Egg Day START collection (saving baseline SE counts)...")
		}
		CollectEggDayStart(s, year)
		if onProgress != nil {
			onProgress("✅ Egg Day START collection complete. Baseline SE counts saved.")
		}
	} else {
		if onProgress != nil {
			onProgress("🏁 Running Egg Day END collection & calculating gains...")
		}
		CollectEggDayEndAndCalculate(s, year, dryRun)
		if onProgress != nil {
			onProgress("✅ Egg Day END collection and calculations complete.")
		}
	}
}

// CollectEggDayStart records the initial SE count for all opted-in players.
func CollectEggDayStart(s *discordgo.Session, year int) {
	log.Printf("eggday: collecting start stats for year %d", year)
	userIDs := GetAllOptInUserIDs()
	if len(userIDs) == 0 {
		log.Println("eggday: no opted-in users found")
		return
	}

	yearStr := fmt.Sprintf("%d", year)

	for _, userID := range userIDs {
		enc := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
		if enc == "" {
			continue
		}
		dec := ei.DecryptEID(enc)
		if dec == "" {
			continue
		}

		backup, _ := ei.GetFirstContactFromAPI(s, enc, userID, true)
		if backup == nil {
			log.Printf("eggday: failed to fetch backup for user %s during start collection", userID)
			continue
		}

		game := backup.GetGame()
		if game == nil {
			continue
		}

		se := game.GetSoulEggsD()
		gameName := ei.NormalizePlayerNameForDisplay(backup.GetUserName())
		if game.GetPermitLevel() != 1 {
			gameName += " (SP)"
		}

		err := farmerstate.UpsertLeaderboardStat("egg_day_se_start", userID, gameName, yearStr, se, sql.NullString{})
		if err != nil {
			log.Printf("eggday: failed to save start stat for user %s: %v", userID, err)
		} else {
			log.Printf("eggday: saved start stat for user %s (%s): %f SE", userID, gameName, se)
		}
	}
}

// CollectEggDayEndAndCalculate records the ending SE count, calculates gains, and posts leaderboards.
func CollectEggDayEndAndCalculate(s *discordgo.Session, year int, dryRun bool) {
	log.Printf("eggday: collecting end stats for year %d", year)
	userIDs := GetAllOptInUserIDs()
	if len(userIDs) == 0 {
		log.Println("eggday: no opted-in users found")
		return
	}

	yearStr := fmt.Sprintf("%d", year)

	for _, userID := range userIDs {
		enc := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
		if enc == "" {
			continue
		}
		dec := ei.DecryptEID(enc)
		if dec == "" {
			continue
		}

		backup, _ := ei.GetFirstContactFromAPI(s, enc, userID, true)
		if backup == nil {
			log.Printf("eggday: failed to fetch backup for user %s during end collection", userID)
			continue
		}

		game := backup.GetGame()
		if game == nil {
			continue
		}

		seEnd := game.GetSoulEggsD()
		gameName := ei.NormalizePlayerNameForDisplay(backup.GetUserName())
		if game.GetPermitLevel() != 1 {
			gameName += " (SP)"
		}

		err := farmerstate.UpsertLeaderboardStat("egg_day_se_end", userID, gameName, yearStr, seEnd, sql.NullString{})
		if err != nil {
			log.Printf("eggday: failed to save end stat for user %s: %v", userID, err)
			continue
		}
		log.Printf("eggday: saved end stat for user %s (%s): %f SE", userID, gameName, seEnd)

		// Calculate gains if start stat is available
		startStat := GetStatForPlayerAndSnapDate("egg_day_se_start", userID, yearStr)
		if startStat != nil {
			seStart := startStat.Value
			gain := seEnd - seStart
			if gain < 0 {
				gain = 0
			}

			var pct float64
			if seStart > 0 {
				pct = (gain / seStart) * 100.0
			}

			// Save LBEggDaySEGain
			err = farmerstate.UpsertLeaderboardStat(LBEggDaySEGain, userID, gameName, yearStr, gain, sql.NullString{})
			if err != nil {
				log.Printf("eggday: failed to save se_gain for user %s: %v", userID, err)
			}

			// Save LBEggDaySEPct
			err = farmerstate.UpsertLeaderboardStat(LBEggDaySEPct, userID, gameName, yearStr, pct, sql.NullString{})
			if err != nil {
				log.Printf("eggday: failed to save se_pct for user %s: %v", userID, err)
			}

			log.Printf("eggday: calculated stats for user %s - Gain: %f SE, Improvement: %.2f%%", userID, gain, pct)
		} else {
			log.Printf("eggday: starting stat not found for user %s, skipping calculations", userID)
		}
	}

	// Post the new leaderboards if not a dry run
	if !dryRun {
		log.Printf("eggday: posting Egg Day leaderboards for year %s", yearStr)
		PostLeaderboards(s, yearStr, "", "group_egg_day", "update", nil)
	} else {
		log.Printf("eggday: dry run — skipping Discord post for Egg Day leaderboards")
	}
}
