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
	"github.com/jasonlvhit/gocron"
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

	/*
		Here's the exact cron config for the cloudflare worker that triggers the github action that updates contracts.
		Normal contract time is either 16 or 17 utc depending on US daylight savings.
		at utc 16, 17, and 18 it checks on the hour and every minute for the first 9 minutes after and then every 5 minutes the rest of the hour. The rest of the time it checks every 30 minutes. It happens rarely but sometimes contracts get released late.

		TLDR yes it checks right at contract release time and also fairly frequently for the next hour or two after contract release time and then every 30 minutes
	*/

	// Contracts always start at 9:00 AM Pacific Time
	// 9:00 AM Pacific Time is 16:00 UTC
	// Get current system timezone

	currentTime := time.Now()
	currentTimeZone, offset := currentTime.Zone()
	offset = offset / 3600
	log.Println("The Current time zone is:", currentTimeZone, " Offset:", offset)

	/*
		minuteTimes := []string{":00:30", ":00:45", ":01:00", ":01:30", ":02:00", ":03:00", ":05:00", ":10:00"}
		var checkTimes []string

			// Hit the server so the cache is hit 3 minutes earlier
			checkTimes = append(checkTimes, fmt.Sprintf("%d:57:00", 15+offset))
			checkTimes = append(checkTimes, fmt.Sprintf("%d:57:00", 16+offset))

			for _, t := range minuteTimes {
				checkTimes = append(checkTimes, fmt.Sprintf("%d%s", 16+offset, t)) // Handle daylight savings time
				checkTimes = append(checkTimes, fmt.Sprintf("%d%s", 17+offset, t)) // Handle standard time
			}

			for _, t := range checkTimes {
				err := gocron.Every(1).Day().At(t).Do(crondownloadEggIncData)
				if err != nil {
					log.Print(err)
				}
			}
	*/

	err = gocron.Every(8).Hours().Do(boost.ArchiveContracts, s)
	if err != nil {
		log.Print(err)
	}

	// Want to check Egg Inc data once a day day minutes
	err = gocron.Every(1).Day().At("00:00:05").Do(crondownloadEggIncData)
	if err != nil {
		log.Print(err)
	}

	// Prune generated banner files older than one month.
	err = gocron.Every(1).Day().At("00:30:05").Do(cronPruneOldGeneratedBanners)
	if err != nil {
		log.Print(err)
	}

	// Check for new periodicals once at 9 PDT
	err = gocron.Every(1).Day().At(fmt.Sprintf("%d:00:05", 16+offset)).Do(events.GetPeriodicalsFromAPI, s)
	if err != nil {
		log.Print(err)
	}

	// Check for new periodicals once at 9 PST
	err = gocron.Every(1).Day().At(fmt.Sprintf("%d:00:05", 17+offset)).Do(events.GetPeriodicalsFromAPI, s)
	if err != nil {
		log.Print(err)
	}

	//events.GetPeriodicalsFromAPI(s)

	<-gocron.Start()
	log.Print("Exiting cron job")
}
