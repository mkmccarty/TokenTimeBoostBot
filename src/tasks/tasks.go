package tasks

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jasonlvhit/gocron"
	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
)

const eggIncContractsURL string = "https://raw.githubusercontent.com/carpetsage/egg/main/periodicals/data/contracts.json"
const eggIncContractsFile string = "ttbb-data/ei-contracts.json"

var lastContractUpdate time.Time

// HandleReloadContractsCommand will handle the /reload command
func HandleReloadContractsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	str := "No updated Egg Inc contract data available"

	result := downloadEggIncContracts()
	if result {
		str += "New contract data loaded"
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    str,
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})
}

func isNewEggIncContractDataAvailable() bool {
	req, err := http.NewRequest("GET", eggIncContractsURL, nil)
	if err != nil {
		log.Print(err)
		return false
	}

	if _, err = os.Stat(eggIncContractsFile); err == nil {
		// Get the current file size
		fileInfo, err := os.Stat(eggIncContractsFile)
		if err != nil {
			log.Print(err)
			return false
		}

		const EvalWidth = 256

		fileSize := fileInfo.Size()
		rangeStart := fileSize - EvalWidth
		rangeHeader := fmt.Sprintf("bytes=%d-", rangeStart)
		req.Header.Add("Range", rangeHeader)
		log.Print("EI-Contracts: Requested Range", rangeHeader)
		var client http.Client
		resp, err := client.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return false
		}
		str := string(body)
		log.Print("EI-Contracts: Downloaded ", len(body), " bytes")
		log.Print("EI-Contracts: Downloaded Bytes:", str)

		if len(body) > 0 {
			// Test if the end of the the file eggIncContractsFile is the same as the body
			file, err := os.Open(eggIncContractsFile)
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
			log.Print("EI-Contracts: Bytes ", string(fileBytes))
			log.Print("EI-Contracts: Compare ", bytes.Equal(fileBytes, body), " Len:", len(fileBytes), len(body))

			// Compare the last 1024 bytes of the file with the body
			if string(fileBytes) == string(body) && len(fileBytes) == len(body) {
				return false
			}

			return true
		}
		return false
	}
	return true
}

func downloadEggIncContracts() bool {
	// Download the latest data from this URL https://raw.githubusercontent.com/carpetsage/egg/main/periodicals/data/contracts.json
	// save it to disk and put it into an array of structs
	if time.Since(lastContractUpdate) < 10*time.Minute {
		log.Print("EI-Contracts. New data was updated ", lastContractUpdate)

		return false
	}
	if !isNewEggIncContractDataAvailable() {
		log.Print("EI-Contracts. No new data available")
		return false
	}

	resp, err := http.Get(eggIncContractsURL)
	if err != nil {
		log.Print(err)
		return false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
		return false
	}
	defer resp.Body.Close()

	// Check if the file already exists
	_, err = os.Stat(eggIncContractsFile)
	if err == nil {
		// Delete the file
		os.Remove(eggIncContractsFile)
	}

	// Save to disk
	err = os.WriteFile(eggIncContractsFile, body, 0644)
	if err != nil {
		log.Print(err)
		return false
	}

	// Notify bot of out new data
	boost.LoadContractData(eggIncContractsFile)
	lastContractUpdate = time.Now()
	log.Print("EI-Contracts. New data loaded, length: ", int64(len(body)))
	return true
}

// ExecuteCronJob runs the cron jobs for the bot
func ExecuteCronJob() {
	if !downloadEggIncContracts() {
		boost.LoadContractData(eggIncContractsFile)
	}
	/*
		Here's the exact cron config for the cloudflare worker that triggers the github action that updates contracts.
		Normal contract time is either 16 or 17 utc depending on US daylight savings.
		at utc 16, 17, and 18 it checks on the hour and every minute for the first 9 minutes after and then every 5 minutes the rest of the hour. The rest of the time it checks every 30 minutes. It happens rarely but sometimes contracts get released late.

		TLDR yes it checks right at contract release time and also fairly frequently for the next hour or two after contract release time and then every 30 minutes
	*/
	gocron.Every(1).Day().At("16:00:05").Do(downloadEggIncContracts)
	gocron.Every(1).Day().At("16:00:15").Do(downloadEggIncContracts)
	gocron.Every(1).Day().At("16:01:15").Do(downloadEggIncContracts)
	gocron.Every(1).Day().At("16:02:15").Do(downloadEggIncContracts)
	gocron.Every(1).Day().At("16:03:15").Do(downloadEggIncContracts)

	gocron.Every(1).Day().Do(boost.ArchiveContracts)

	<-gocron.Start()
	log.Print("Exiting cron job")
}
