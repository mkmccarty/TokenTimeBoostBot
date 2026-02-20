package ei

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"sync"
)

// StatusMessagesFile is a struct to hold the status messages
type StatusMessagesFile struct {
	StatusMessages []string `json:"status_messages"`
}

// StatusMessages is a list of status messages
var StatusMessages []string
var statusMessagesMutex sync.RWMutex

// LoadStatusMessages loads status messages from a JSON file
func LoadStatusMessages(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		log.Printf("Failed to open status messages file: %v", err)
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Failed to close: %v", err)
		}
	}()

	var messagesLoaded StatusMessagesFile
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&messagesLoaded); err != nil {
		log.Printf("Failed to decode status messages: %v", err)
		return
	}

	statusMessagesMutex.Lock()
	StatusMessages = append([]string(nil), messagesLoaded.StatusMessages...)
	rand.Shuffle(len(StatusMessages), func(i, j int) {
		StatusMessages[i], StatusMessages[j] = StatusMessages[j], StatusMessages[i]
	})
	statusMessagesMutex.Unlock()

	log.Printf("Loaded %d status messages", len(messagesLoaded.StatusMessages))
}

// GetRandomStatusMessage returns the next status message from a shuffled queue.
//
// Messages are shuffled on load, then rotated in order so each message is used
// once before repeating.
func GetRandomStatusMessage() (string, error) {
	statusMessagesMutex.Lock()
	defer statusMessagesMutex.Unlock()

	if len(StatusMessages) == 0 {
		return "", fmt.Errorf("StatusMessages is empty")
	}

	activity := StatusMessages[0]
	StatusMessages = append(StatusMessages[1:], activity)

	return activity, nil
}
