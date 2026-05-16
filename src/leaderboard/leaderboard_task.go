package leaderboard

import (
	"fmt"
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

type playerGroup struct {
	eiUserID      string
	encryptedEIID string
	discordIDs    []string
	optedInTypes  []string // Union of all individual lb_type keys for these Discord IDs
}

// RunLeaderboardCollection is the main weekly entry point. It fans out API
// calls through a bounded worker pool, saves results, then posts to Discord.
// Pass dryRun=true to skip the Discord post step.
func RunLeaderboardCollection(s *discordgo.Session, dryRun bool, onProgress func(string)) {
	if onProgress != nil {
		onProgress("🔍 Starting weekly collection run...")
	}
	log.Println("leaderboard: starting weekly collection run")

	userIDs := GetAllOptInUserIDs()
	if len(userIDs) == 0 {
		log.Println("leaderboard: no opted-in players, skipping")
		return
	}

	// Group the opted-in Discord users by their Egg Inc User ID to minimize duplicate API requests.
	groups := make(map[string]*playerGroup)
	for _, uid := range userIDs {
		enc := farmerstate.GetMiscSettingString(uid, "encrypted_ei_id")
		if enc == "" {
			continue
		}
		dec := ei.DecryptEID(enc)
		if dec == "" {
			continue
		}

		// Get all individual lb_types this user has opted into.
		optRows, err := farmerstate.GetLeaderboardOptInsForUser(uid)
		if err != nil || len(optRows) == 0 {
			continue
		}

		var keys []string
		for _, o := range optRows {
			if o.LbType == OptInAll {
				for _, def := range AllLeaderboards {
					keys = append(keys, def.Key)
				}
			} else {
				keys = append(keys, ExpandConfigKey(o.LbType)...)
			}
		}

		if len(keys) == 0 {
			continue
		}

		g, ok := groups[dec]
		if !ok {
			g = &playerGroup{
				eiUserID:      dec,
				encryptedEIID: enc,
			}
			groups[dec] = g
		}
		g.discordIDs = append(g.discordIDs, uid)
		g.optedInTypes = append(g.optedInTypes, keys...)
	}

	var playerGroups []*playerGroup
	for _, g := range groups {
		// Unique the opted-in types in the group
		keysSet := make(map[string]struct{})
		for _, k := range g.optedInTypes {
			keysSet[k] = struct{}{}
		}
		var uniqueKeys []string
		for k := range keysSet {
			uniqueKeys = append(uniqueKeys, k)
		}
		g.optedInTypes = uniqueKeys
		playerGroups = append(playerGroups, g)
	}

	if len(playerGroups) == 0 {
		log.Println("leaderboard: no players with valid Egg Inc IDs, skipping")
		if onProgress != nil {
			onProgress("❌ No players with valid Egg Inc IDs found.")
		}
		return
	}

	snapDate := SnapDateNow()
	n := workerCount()
	status := fmt.Sprintf("📡 Collecting %d unique players (from %d opted-in Discord accounts) with %d workers...", len(playerGroups), len(userIDs), n)
	log.Printf("leaderboard: %s (snap_date %s, dry=%v)", status, snapDate, dryRun)
	if onProgress != nil {
		onProgress(status)
	}

	sem := make(chan struct{}, n)
	var wg sync.WaitGroup

	var completed int
	var mu sync.Mutex
	for _, g := range playerGroups {
		g := g
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			collectPlayerGroup(s, g, snapDate)

			mu.Lock()
			completed++
			c := completed
			mu.Unlock()

			if onProgress != nil && c%5 == 0 {
				onProgress(fmt.Sprintf("📡 Collecting players... (%d/%d)", c, len(playerGroups)))
			}
		}()
	}

	wg.Wait()
	log.Println("leaderboard: collection complete")
	if onProgress != nil {
		onProgress("✅ Collection complete. Preparing posts...")
	}

	if !dryRun {
		PostLeaderboards(s, snapDate, onProgress)
	} else {
		log.Println("leaderboard: dry run — skipping Discord post")
		if onProgress != nil {
			onProgress("🏁 Dry run complete. Data saved but not posted.")
		}
	}
}

// collectPlayerGroup fetches API data for one Egg Inc account and saves leaderboard entries for all associated Discord users.
func collectPlayerGroup(s *discordgo.Session, g *playerGroup, snapDate string) {
	// Determine which API calls are needed based on the union of all opted-in types.
	needsFirstContact := false
	needsArchive := false
	for _, k := range g.optedInTypes {
		if def, ok := LBDefByKey(k); ok {
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
	}

	var backup *ei.Backup
	var archive []*ei.LocalContract

	if needsFirstContact {
		var cached bool
		backup, cached = ei.GetFirstContactFromAPI(s, g.encryptedEIID, g.discordIDs[0], true)
		_ = cached
		if backup == nil {
			log.Printf("leaderboard: first-contact API failed for Egg Inc ID %s (Discord IDs: %v)", g.eiUserID, g.discordIDs)
		}
	}

	if needsArchive {
		var cached bool
		archive, cached = ei.GetContractArchiveFromAPI(s, g.encryptedEIID, g.discordIDs[0], false, true)
		_ = cached
		if archive == nil {
			log.Printf("leaderboard: contract archive API failed for Egg Inc ID %s (Discord IDs: %v)", g.eiUserID, g.discordIDs)
		}
	}

	if backup == nil && archive == nil {
		return
	}

	// Calculate and save entries for each individual Discord user mapped to this Egg Inc account.
	for _, userID := range g.discordIDs {
		// Get all individual lb_types this specific Discord user has opted into.
		optRows, err := farmerstate.GetLeaderboardOptInsForUser(userID)
		if err != nil || len(optRows) == 0 {
			continue
		}

		userKeysSet := make(map[string]struct{})
		for _, o := range optRows {
			if o.LbType == OptInAll {
				for _, def := range AllLeaderboards {
					userKeysSet[def.Key] = struct{}{}
				}
			} else {
				for _, k := range ExpandConfigKey(o.LbType) {
					userKeysSet[k] = struct{}{}
				}
			}
		}

		var userKeys []string
		for k := range userKeysSet {
			userKeys = append(userKeys, k)
		}

		// For CXPWeeklyDelta, we need a prior total.
		priorCXPTotal := 0.0
		if prior := GetPriorStatForPlayer(LBCXPWeeklyDelta, userID); prior != nil {
			_, _ = fmt.Sscanf(prior.Details, "total:%f", &priorCXPTotal)
		}

		// Run calculators specifically for this user's opted-in keys.
		allEntries := RunCalculators(userID, backup, archive, userKeys, snapDate, priorCXPTotal)

		// Save global entries.
		for _, e := range allEntries {
			SaveLBEntry(e)
		}
		log.Printf("leaderboard: saved %d entries globally for user %s (Egg Inc: %s)", len(allEntries), userID, g.eiUserID)
	}
}

// ScheduleWeeklyCollection registers the Friday 15:00 PT collection cron job.
// Call this from tasks.ExecuteCronJob.
func ScheduleWeeklyCollection(s *discordgo.Session) {
	scheduleWeeklyFriday(15, 0, func() {
		RunLeaderboardCollection(s, false, nil)
	})
}
