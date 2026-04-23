package tasks

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/go-github/v33/github"
	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/events"
	"github.com/mkmccarty/TokenTimeBoostBot/src/version"
)

const eggIncContractsURL string = "https://raw.githubusercontent.com/carpetsage/egg/main/periodicals/data/contracts.json"
const eggIncContractsFile string = "ttbb-data/ei-contracts.json"

const eggIncEventsURL string = "https://raw.githubusercontent.com/carpetsage/egg/main/periodicals/data/events.json"
const eggIncEventsFile string = "ttbb-data/ei-events.json"

const eggIncCustomEggsURL string = "https://raw.githubusercontent.com/carpetsage/egg/main/periodicals/data/customeggs.json"
const eggIncCustomEggsFile string = "ttbb-data/ei-customeggs.json"

const eggIncDataSchemaURL string = "https://raw.githubusercontent.com/carpetsage/egg/main/lib/artifacts/data.schema.json"
const eggIncDataSchemaFile string = "ttbb-data/ei-data.schema.json"

const eggIncEiAfxDataURL string = "https://raw.githubusercontent.com/carpetsage/egg/main/wasmegg/_common/eiafx/eiafx-data.json"
const eggIncEiAfxDataFile string = "ttbb-data/ei-afx-data.json"

const eggIncEiAfxConfigURL string = "https://raw.githubusercontent.com/carpetsage/egg/main/wasmegg/_common/eiafx/eiafx-config.json"
const eggIncEiAfxConfigFile string = "ttbb-data/ei-afx-config.json"

const eggIncEiResearchesURL string = "https://raw.githubusercontent.com/carpetsage/egg/refs/heads/main/lib/researches.json"
const eggIncEiResearchesFile string = "ttbb-data/ei-researches.json"

const eggIncTokenComplaintsURL string = "https://raw.githubusercontent.com/mkmccarty/TokenTimeBoostBot/refs/heads/main/data/token-complaints.json"
const eggIncTokenComplaintsFile string = "ttbb-data/token-complaints.json"

const eggIncStatusMessagesURL string = "https://raw.githubusercontent.com/mkmccarty/TokenTimeBoostBot/refs/heads/main/data/status-messages.json"
const eggIncStatusMessagesFile string = "ttbb-data/status-messages.json"

var lastContractUpdate time.Time
var lastEventUpdate time.Time

// GetSlashReloadContractsCommand returns the command definition for reloading contracts
func GetSlashReloadContractsCommand(cmd string) *discordgo.ApplicationCommand {
	var adminPermission int64 = 0
	return &discordgo.ApplicationCommand{
		Name:                     cmd,
		Description:              "Manual check for new Egg Inc contract data.",
		DefaultMemberPermissions: &adminPermission,
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
		},
	}
}

// HandleReloadContractsCommand will handle the /reload command
func HandleReloadContractsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	str := "No updated Egg Inc contract data available.\n"

	userID := ""
	if i.GuildID == "" {
		userID = i.User.ID
	} else {
		userID = i.Member.User.ID
	}

	// Only allow command if users is in the admin list
	perms, err := s.UserChannelPermissions(userID, i.ChannelID)
	if err != nil {
		log.Println(err)
	}
	if perms&discordgo.PermissionAdministrator == 0 && userID != config.AdminUserID {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    "You are not authorized to use this command.",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
		return
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Working on it...",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	lastContractUpdate = time.Time{}
	lastEventUpdate = time.Time{}
	downloadEggIncData(eggIncContractsURL, eggIncContractsFile)
	downloadEggIncData(eggIncEventsURL, eggIncEventsFile)
	bottools.LoadEmotes(s, true)

	events.GetPeriodicalsFromAPI(s)

	// if lastContractUpdate or lastEventUpdate was updated within the last 1 minute
	// then we have new data
	if time.Since(lastContractUpdate) < 1*time.Minute || time.Since(lastEventUpdate) < 1*time.Minute {
		str = "Updated Egg Inc contract data:.\n"
		str += fmt.Sprintf("> Boost Bot version:  %s\n", version.Version)
		str += fmt.Sprintf("> Contracts: %s\n", lastContractUpdate.Format(time.RFC1123))
		str += fmt.Sprintf("> Events: %s\n", lastEventUpdate.Format(time.RFC1123))
		str += fmt.Sprintf("> Collegeggtibles: %d\n", len(ei.CustomEggMap))
	} else {
		str += fmt.Sprintf("> Boost Bot version:  %s\n", version.Version)
		str += fmt.Sprintf("> Contracts: %s\n", lastContractUpdate.Format(time.RFC1123))
		str += fmt.Sprintf("> Events: %s\n", lastEventUpdate.Format(time.RFC1123))
		str += fmt.Sprintf("> Collegeggtibles: %d\n", len(ei.CustomEggMap))
	}
	if config.IsDevBot() {
		ei.GetConfigFromAPI(s)
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Content: str},
	)

	// I want to go through all the active contracts and update their EggName and EggEmoji field.
	// Use the original Contract data for that info
	for _, contract := range boost.Contracts {
		if contract == nil || len(contract.Location) == 0 {
			continue
		}
		originalContract := ei.EggIncContractsAll[contract.ContractID]
		contract.Egg = originalContract.Egg
		contract.EggName = originalContract.EggName
		contract.EggEmoji = boost.FindEggEmoji(originalContract.EggName)
	}

}

func parseGithubRawURL(urlStr string) (owner, repo, branch, path string, err error) {
	urlStr = strings.TrimPrefix(urlStr, "https://raw.githubusercontent.com/")
	parts := strings.Split(urlStr, "/")
	if len(parts) < 4 {
		return "", "", "", "", fmt.Errorf("invalid github raw url")
	}
	owner = parts[0]
	repo = parts[1]

	if parts[2] == "refs" && len(parts) >= 6 && parts[3] == "heads" {
		branch = parts[4]
		path = strings.Join(parts[5:], "/")
	} else {
		branch = parts[2]
		path = strings.Join(parts[3:], "/")
	}
	return owner, repo, branch, path, nil
}

func getGitBlobSHA(filename string) (string, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	hasher := sha1.New()
	if _, err := fmt.Fprintf(hasher, "blob %d\x00", len(content)); err != nil {
		return "", err
	}
	if _, err := hasher.Write(content); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func crondownloadEggIncData() {
	downloadEggIncData(eggIncContractsURL, eggIncContractsFile)
	downloadEggIncData(eggIncEventsURL, eggIncEventsFile)
	downloadEggIncData(eggIncCustomEggsURL, eggIncCustomEggsFile)
}

func cronPruneOldGeneratedBanners() {
	outputPath := config.BannerOutputPath
	if outputPath == "" {
		return
	}

	entries, err := os.ReadDir(outputPath)
	if err != nil {
		log.Printf("Banner prune skipped, unable to read %s: %v", outputPath, err)
		return
	}

	cutoff := time.Now().AddDate(0, -1, 0)
	pruned := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".png") {
			continue
		}
		if !strings.HasPrefix(name, "b") || !strings.Contains(name, "-") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			log.Printf("Banner prune skipped file %s: %v", name, err)
			continue
		}

		if info.ModTime().After(cutoff) {
			continue
		}

		fullPath := outputPath + "/" + name
		if err := os.Remove(fullPath); err != nil {
			log.Printf("Banner prune failed for %s: %v", fullPath, err)
			continue
		}
		pruned++
	}

	if pruned > 0 {
		log.Printf("Banner prune removed %d old generated images", pruned)
	}
}

func downloadEggIncData(urlStr string, filename string) bool {
	owner, repo, branch, path, err := parseGithubRawURL(urlStr)
	if err != nil {
		log.Printf("Failed to parse URL %s: %v", urlStr, err)
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client := github.NewClient(nil)
	content, _, _, err := client.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{Ref: branch})
	if err != nil {
		log.Printf("Failed to resolve %s in repository: %v", path, err)
		return false
	}

	newSHA := content.GetSHA()
	localSHA, err := getGitBlobSHA(filename)
	if err == nil && localSHA == newSHA {
		// The file hasn't changed
		return false
	}

	downloadURL := content.GetDownloadURL()
	if downloadURL == "" {
		log.Printf("Download URL not found in repository for %s", path)
		return false
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		log.Print(err)
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Print(err)
		return false
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close: %v", err)
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		log.Printf("Download failed with status %s", resp.Status)
		return false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
		return false
	}

	// Check if the file already exists
	_, err = os.Stat(filename)
	if err == nil {
		err = os.Remove(filename)
		if err != nil {
			log.Println("Error Deleting file ", err.Error())
		}
	}

	// Save to disk
	err = os.WriteFile(filename, body, 0644)
	if err != nil {
		log.Print(err)
		return false
	}

	// Notify bot of out new data
	switch filename {
	case eggIncContractsFile:
		boost.LoadContractData(filename)
		lastContractUpdate = time.Now()
		log.Printf("EI-Contracts. New data loaded, length: %d\n", int64(len(body)))
	case eggIncEventsFile:
		ei.LoadEventData(filename)
		lastEventUpdate = time.Now()
		log.Printf("EI-Events. New data loaded, length: %d\n", int64(len(body)))
	case eggIncEiAfxDataFile:
		err := ei.LoadArtifactsData(filename)
		if err != nil {
			log.Print(err)
		} else {
			log.Printf("EI-AFX-Data. New data loaded, length: %d\n", int64(len(body)))
		}
	case eggIncEiResearchesFile:
		ei.LoadResearchData(filename)
	case eggIncTokenComplaintsFile:
		ei.LoadTokenComplaints(filename)
	case eggIncStatusMessagesFile:
		ei.LoadStatusMessages(filename)
	}
	return true
}

// scheduleDaily triggers a task every day at the specified local time
func scheduleDaily(hour, min, sec int, task func()) {
	go func() {
		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day(), hour, min, sec, 0, now.Location())
			if !now.Before(next) {
				next = next.AddDate(0, 0, 1)
			}
			time.Sleep(time.Until(next))
			task()
		}
	}()
}

// schedulePeriodicals natively handles Pacific Time (PST/PDT) and triggers polling
/*
	Here's the exact cron config for the cloudflare worker that triggers the github action that updates contracts.
	Normal contract time is either 16 or 17 utc depending on US daylight savings.
	at utc 16, 17, and 18 it checks on the hour and every minute for the first 9 minutes after and then every 5 minutes the rest of the hour. The rest of the time it checks every 30 minutes. It happens rarely but sometimes contracts get released late.

	TLDR yes it checks right at contract release time and also fairly frequently for the next hour or two after contract release time and then every 30 minutes
*/
func schedulePeriodicals(s *discordgo.Session) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		log.Printf("Error loading timezone America/Los_Angeles: %v", err)
		return
	}

	// Check if we missed today's load time upon startup
	now := time.Now().In(loc)
	todayLoadTime := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, loc)
	wd := now.Weekday()
	isPollDay := wd == time.Monday || wd == time.Wednesday || wd == time.Friday

	if now.After(todayLoadTime) {
		needsReload := true

		if !lastContractUpdate.IsZero() && lastContractUpdate.After(todayLoadTime) {
			needsReload = false
		} else if !lastEventUpdate.IsZero() && lastEventUpdate.After(todayLoadTime) {
			needsReload = false
		} else if fileInfo, err := os.Stat(eggIncContractsFile); err == nil && fileInfo.ModTime().In(loc).After(todayLoadTime) {
			needsReload = false
		}

		if needsReload {
			if isPollDay {
				log.Println("Startup check: Periodicals data hasn't been updated since today's load time. Triggering reload loop.")
				go pollPeriodicalsUntilUpdated(s)
			} else {
				log.Println("Startup check: Periodicals data hasn't been updated since today's load time. Triggering single reload.")
				go events.GetPeriodicalsFromAPI(s)
			}
		}
	}

	for {
		now = time.Now().In(loc)
		// Set target time to 9:00:00 AM PT today
		next := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, loc)

		// If it's already past 9:00:00 AM PT, schedule for tomorrow
		if !now.Before(next) {
			next = next.AddDate(0, 0, 1)
		}

		// Sleep until the next 9:00:00 AM PT
		time.Sleep(time.Until(next))

		// Run retry loop on Mon, Wed, Fri; otherwise, run once
		wd = next.Weekday()
		if wd == time.Monday || wd == time.Wednesday || wd == time.Friday {
			go pollPeriodicalsUntilUpdated(s)
		} else {
			go events.GetPeriodicalsFromAPI(s)
		}
	}
}

func pollPeriodicalsUntilUpdated(s *discordgo.Session) {
	log.Println("Starting periodic checks for Egg Inc updates...")
	// Poll every minute for the first 9 minutes, then every 5 minutes for roughly 2 hours
	maxRetries := 32 // 10 attempts in the first 9 mins + 22 attempts every 5 mins
	for i := 0; i < maxRetries; i++ {
		events.GetPeriodicalsFromAPI(s)

		// Check if a manual reload successfully updated the contracts or events
		recentContract := !lastContractUpdate.IsZero() && time.Since(lastContractUpdate) < 5*time.Minute
		recentEvent := !lastEventUpdate.IsZero() && time.Since(lastEventUpdate) < 5*time.Minute

		if recentContract || recentEvent {
			log.Println("Periodicals successfully updated via manual reload.")
			break
		}

		var waitTime time.Duration
		if i < 9 {
			waitTime = 1 * time.Minute
		} else {
			waitTime = 5 * time.Minute
		}
		log.Printf("Update not yet detected, waiting %v before retrying...", waitTime)
		time.Sleep(waitTime)
	}
}

// scheduleImageDownloads natively handles Pacific Time (PST/PDT) and triggers an image pre-fetch at 8:55 AM.
func scheduleImageDownloads() {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		log.Printf("Error loading timezone America/Los_Angeles for images: %v", err)
		return
	}

	for {
		now := time.Now().In(loc)
		// Set target time to 8:55:00 AM PT today
		next := time.Date(now.Year(), now.Month(), now.Day(), 8, 55, 0, 0, loc)

		// If it's already past 8:55:00 AM PT, schedule for tomorrow
		if !now.Before(next) {
			next = next.AddDate(0, 0, 1)
		}

		// Sleep until the next 8:55:00 AM PT
		time.Sleep(time.Until(next))

		log.Println("Pre-fetching latest egg/banner images...")
		if err := bottools.DownloadLatestEggImages(config.BannerPath); err != nil {
			log.Printf("Error pre-fetching images: %v", err)
		}
	}
}

// ExecuteCronJob runs the cron jobs for the bot
func ExecuteCronJob(s *discordgo.Session) {
	// Look for new Custom Eggs

	var err error
	ei.CustomEggMap, err = events.LoadCustomEggData()
	if err != nil {
		ei.CustomEggMap = make(map[string]*ei.EggIncCustomEgg)
		events.GetPeriodicalsFromAPI(s)
	}
	ei.SetColleggtibleValues()

	if !downloadEggIncData(eggIncContractsURL, eggIncContractsFile) {
		boost.LoadContractData(eggIncContractsFile)
	}
	if !downloadEggIncData(eggIncEventsURL, eggIncEventsFile) {
		ei.LoadEventData(eggIncEventsFile)
	}
	downloadEggIncData(eggIncDataSchemaURL, eggIncDataSchemaFile)
	downloadEggIncData(eggIncEiAfxConfigURL, eggIncEiAfxConfigFile)

	if !downloadEggIncData(eggIncEiAfxDataURL, eggIncEiAfxDataFile) {
		err := ei.LoadArtifactsData(eggIncEiAfxDataFile)
		if err != nil {
			log.Print(err)
		}
	}

	if !downloadEggIncData(eggIncEiResearchesURL, eggIncEiResearchesFile) {
		ei.LoadResearchData(eggIncEiResearchesFile)
	}

	if !downloadEggIncData(eggIncTokenComplaintsURL, eggIncTokenComplaintsFile) {
		ei.LoadTokenComplaints(eggIncTokenComplaintsFile)
	}

	if !downloadEggIncData(eggIncStatusMessagesURL, eggIncStatusMessagesFile) {
		ei.LoadStatusMessages(eggIncStatusMessagesFile)
	}

	events.GetPeriodicalsFromAPI(s)

	// Archive contracts every 8 hours
	go func() {
		ticker := time.NewTicker(8 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			boost.ArchiveContracts(s)
		}
	}()

	// Want to check Egg Inc data once a day day minutes
	scheduleDaily(0, 0, 5, crondownloadEggIncData)

	// Prune generated banner files older than one month.
	scheduleDaily(0, 30, 5, cronPruneOldGeneratedBanners)

	// Start timezone-aware loop to poll Periodicals on Mon, Wed, Fri at 9 AM PT
	go schedulePeriodicals(s)

	// Start timezone-aware loop to pre-fetch images at 8:55 AM PT daily
	go scheduleImageDownloads()

	log.Print("Cron jobs scheduled")
}
