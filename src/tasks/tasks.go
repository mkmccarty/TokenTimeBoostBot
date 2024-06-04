package tasks

import (
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"slices"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jasonlvhit/gocron"
	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/launch"
	"github.com/rs/xid"
)

const eggIncContractsURL string = "https://raw.githubusercontent.com/carpetsage/egg/main/periodicals/data/contracts.json"
const eggIncContractsFile string = "ttbb-data/ei-contracts.json"

const eggIncEventsURL string = "https://raw.githubusercontent.com/carpetsage/egg/main/periodicals/data/events.json"
const eggIncEventsFile string = "ttbb-data/ei-events.json"

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
	if slices.Index(boost.AdminUsers, userID) == -1 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    "You are not authorized to use this command.",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
		return
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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

	// if lastContractUpdate or lastEventUpdate was updated within the last 1 minute
	// then we have new data
	if time.Since(lastContractUpdate) < 1*time.Minute || time.Since(lastEventUpdate) < 1*time.Minute {
		str = "Updated Egg Inc contract data:.\n"
		str += fmt.Sprintf("> Contracts: %s\n", lastContractUpdate.Format(time.RFC1123))
		str += fmt.Sprintf("> Events: %s\n", lastEventUpdate.Format(time.RFC1123))
	} else {
		str += fmt.Sprintf("> Contracts: %s\n", lastContractUpdate.Format(time.RFC1123))
		str += fmt.Sprintf("> Events: %s\n", lastEventUpdate.Format(time.RFC1123))
	}

	s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Content: str},
	)

}

func isNewEggIncDataAvailable(url string, finalname string) bool {
	if _, err := os.Stat(finalname); err == nil {
		// Get the current file size
		fileInfo, err := os.Stat(finalname)
		if err != nil {
			log.Print(err)
			return true
		}

		switch finalname {
		case eggIncContractsFile:
			if time.Since(lastContractUpdate) < 2*time.Hour {
				return false
			}
		case eggIncEventsFile:
			if time.Since(lastEventUpdate) < 2*time.Hour {
				return false
			}
		}

		req, err := http.NewRequest("GET", url+"?token="+xid.New().String(), nil)
		if err != nil {
			log.Print(err)
			return false
		}

		EvalWidth := int64(256 - rand.IntN(128)/2 + 1)

		fileSize := fileInfo.Size()
		rangeStart := fileSize - EvalWidth
		rangeHeader := fmt.Sprintf("bytes=%d-", rangeStart)
		req.Header.Add("Range", rangeHeader)
		req.Header.Add("Cache-Control", "no-cache, no-store, must-revalidate")
		req.Header.Add("Pragma", "no-cache")
		req.Header.Add("Expires", "0")
		req.Header.Add("Clear-Site-Data", "*")
		//		log.Print("EI-Contracts: Requested Range", rangeHeader)
		var client http.Client
		resp, err := client.Do(req)
		if err != nil {
			return false
		}
		//log.Print("EI-Contracts: Response Status:", resp.Status, resp.Header)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return false
		}
		str := string(body)
		log.Print("EI-Data: Downloaded ", len(body), " bytes")
		log.Print("EI-Data: Downloaded Bytes:", str)

		if len(body) > 0 {
			// Test if the end of the the file eggIncContractsFile is the same as the body
			file, err := os.Open(finalname)
			if err != nil {
				log.Print(err)
				return false
			}
			defer file.Close()
			// Read the last 1024 bytes from the file
			file.Seek(-EvalWidth, io.SeekEnd)
			fileBytes := make([]byte, EvalWidth)
			_, err = file.Read(fileBytes)
			if err != nil {
				log.Print(err)
				return false
			}
			//log.Print("EI-Contracts: Saved File Bytes:", string(fileBytes))
			//log.Print("EI-Contracts: Compare ", bytes.Equal(fileBytes, body), " Len:", len(fileBytes), len(body))

			// Compare the last 1024 bytes of the file with the body
			if string(fileBytes) == string(body) && len(fileBytes) == len(body) {
				switch finalname {
				case eggIncContractsFile:
					if lastContractUpdate.IsZero() {
						lastContractUpdate = time.Now()
					}
				case eggIncEventsFile:
					if lastEventUpdate.IsZero() {
						lastEventUpdate = time.Now()
					}
				}
				return false
			}

			return true
		}
		return false
	}
	return true
}

func crondownloadEggIncData() {
	downloadEggIncData(eggIncContractsURL, eggIncContractsFile)
	downloadEggIncData(eggIncEventsURL, eggIncEventsFile)
}

func downloadEggIncData(url string, filename string) bool {
	// Download the latest data from this URL https://raw.githubusercontent.com/carpetsage/egg/main/periodicals/data/contracts.json
	// save it to disk and put it into an array of structs
	// If data has been read within the last 70 minutes then skip it.
	// This wil handle daylight savings time changes
	//if !force && time.Since(lastContractUpdate) < 10*time.Minute {
	//	log.Print("EI-Contracts. New data was updated ", lastContractUpdate)
	//	return false
	//}
	if !isNewEggIncDataAvailable(url, filename) {
		log.Print("EI-Data. No new data available for ", filename)
		return false
	}
	req, err := http.NewRequest("GET", url+"?token="+xid.New().String(), nil)
	if err != nil {
		log.Print(err)
		return false
	}

	req.Header.Add("Cache-Control", "no-cache, no-store, must-revalidate")
	req.Header.Add("Pragma", "no-cache")
	req.Header.Add("Expires", "0")
	req.Header.Add("Clear-Site-Data", "*")

	var client http.Client
	resp, err := client.Do(req)
	if err != nil {
		return false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
	}
	defer resp.Body.Close()

	// Check if the file already exists
	_, err = os.Stat(filename)
	if err == nil {
		err = os.Remove(filename)
		if err != nil {
			log.Print("Error Deleting EI-Contracts File ", err.Error())
		}
	}

	// Save to disk
	err = os.WriteFile(filename, body, 0644)
	if err != nil {
		log.Print(err)
		return false
	}

	// Notify bot of out new data
	if filename == eggIncContractsFile {
		boost.LoadContractData(filename)
		lastContractUpdate = time.Now()
		log.Print("EI-Contracts. New data loaded, length: ", int64(len(body)))
	} else if filename == eggIncEventsFile {
		launch.LoadEventData(filename)
		lastEventUpdate = time.Now()
		log.Print("EI-Events. New data loaded, length: ", int64(len(body)))

	}
	return true
}

// ExecuteCronJob runs the cron jobs for the bot
func ExecuteCronJob(s *discordgo.Session) {
	if !downloadEggIncData(eggIncContractsURL, eggIncContractsFile) {
		boost.LoadContractData(eggIncContractsFile)
	}
	if !downloadEggIncData(eggIncEventsURL, eggIncEventsFile) {
		launch.LoadEventData(eggIncEventsFile)
	}
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

	minuteTimes := []string{":00:15", ":00:30", ":00:45", ":01:00", ":01:30", ":02:00", ":03:00", ":05:00"}
	var checkTimes []string

	for _, t := range minuteTimes {
		checkTimes = append(checkTimes, fmt.Sprintf("%d%s", 16+offset, t)) // Handle daylight savings time
		checkTimes = append(checkTimes, fmt.Sprintf("%d%s", 17+offset, t)) // Handle standard time
	}

	for _, t := range checkTimes {
		gocron.Every(1).Day().At(t).Do(crondownloadEggIncData)
	}

	gocron.Every(1).Day().Do(boost.ArchiveContracts, s)

	<-gocron.Start()
	log.Print("Exiting cron job")
}
