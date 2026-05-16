package leaderboard

import (
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"
)

// defaultWorkerCount is used when no configuration is set.
const defaultWorkerCount = 5

// workerCount returns the configured parallel collection worker count.
func workerCount() int {
	val := guildstate.GetGuildSettingString("DEFAULT", "leaderboard_worker_count")
	if val == "" {
		return defaultWorkerCount
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < 1 {
		return defaultWorkerCount
	}
	return n
}

// scheduleWeeklyFriday launches a goroutine that fires task every Friday at
// hour:min America/Los_Angeles time.
func scheduleWeeklyFriday(hour, min int, task func()) {
	go func() {
		loc, err := time.LoadLocation("America/Los_Angeles")
		if err != nil {
			log.Printf("leaderboard scheduler: failed to load timezone: %v", err)
			return
		}
		for {
			now := time.Now().In(loc)
			daysUntilFriday := (int(time.Friday) - int(now.Weekday()) + 7) % 7
			if daysUntilFriday == 0 {
				target := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, loc)
				if now.After(target) {
					daysUntilFriday = 7
				}
			}
			next := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, loc).
				AddDate(0, 0, daysUntilFriday)
			log.Printf("leaderboard: next collection scheduled for %s", next.Format(time.RFC1123))
			time.Sleep(time.Until(next))
			task()
		}
	}()
}

// SnapDateNow returns today's ISO date string used as the snap_date primary key.
func SnapDateNow() string {
	return time.Now().Format("2006-01-02")
}

// RunLeaderboardCollection is the main weekly entry point. It fans out API
// calls through a bounded worker pool, saves results, then posts to Discord.
// Pass dryRun=true to skip the Discord post step.
func RunLeaderboardCollection(s *discordgo.Session, dryRun bool) {
	log.Println("leaderboard: starting weekly collection run")

	userIDs := GetAllOptInUserIDs()
	if len(userIDs) == 0 {
		log.Println("leaderboard: no opted-in players, skipping")
		return
	}

	snapDate := SnapDateNow()
	n := workerCount()
	log.Printf("leaderboard: collecting %d players with %d workers (snap_date %s, dry=%v)",
		len(userIDs), n, snapDate, dryRun)

	sem := make(chan struct{}, n)
	var wg sync.WaitGroup

	for _, uid := range userIDs {
		uid := uid
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			collectPlayer(s, uid, snapDate)
		}()
	}

	wg.Wait()
	log.Println("leaderboard: collection complete")

	if !dryRun {
		PostLeaderboards(s, snapDate)
	} else {
		log.Println("leaderboard: dry run — skipping Discord post")
	}
}

// collectPlayer fetches API data for one player and saves leaderboard entries.
func collectPlayer(s *discordgo.Session, userID, snapDate string) {
	optedIn := GetPlayerOptInTypes(userID)
	if len(optedIn) == 0 {
		return
	}

	// Determine which API calls are needed.
	needsFirstContact := false
	needsArchive := false
	for _, key := range optedIn {
		def, ok := LBDefByKey(key)
		if !ok {
			continue
		}
		switch def.Source {
		case SourceFirstContact:
			needsFirstContact = true
		case SourceContractArchive:
			needsArchive = true
		case SourceBoth:
			needsFirstContact = true
			needsArchive = true
		}
	}

	encryptedEIID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	if encryptedEIID == "" {
		log.Printf("leaderboard: no EI ID for user %s, skipping", userID)
		return
	}

	var backup *ei.Backup
	var archive []*ei.LocalContract

	if needsFirstContact {
		var cached bool
		backup, cached = ei.GetFirstContactFromAPI(s, encryptedEIID, userID, true)
		_ = cached
		if backup == nil {
			log.Printf("leaderboard: first-contact API failed for user %s", userID)
		}
	}

	if needsArchive {
		var cached bool
		archive, cached = ei.GetContractArchiveFromAPI(s, encryptedEIID, userID, false, true)
		_ = cached
		if archive == nil {
			log.Printf("leaderboard: contract archive API failed for user %s", userID)
		}
	}

	entries := RunCalculators(userID, backup, archive, optedIn, snapDate)
	for _, e := range entries {
		SaveLBEntry(e)
	}
	log.Printf("leaderboard: saved %d entries for user %s", len(entries), userID)
}

// ScheduleWeeklyCollection registers the Friday 15:00 PT collection cron job.
// Call this from tasks.ExecuteCronJob.
func ScheduleWeeklyCollection(s *discordgo.Session) {
	scheduleWeeklyFriday(15, 0, func() {
		RunLeaderboardCollection(s, false)
	})
}
