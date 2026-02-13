package ei

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
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
	StatusMessages = messagesLoaded.StatusMessages
	statusMessagesMutex.Unlock()

	log.Printf("Loaded %d status messages", len(messagesLoaded.StatusMessages))
}

// GetRandomStatusMessage returns a random status message
func GetRandomStatusMessage() (string, error) {
	statusMessagesMutex.RLock()
	defer statusMessagesMutex.RUnlock()

	if len(StatusMessages) == 0 {
		return "", fmt.Errorf("StatusMessages is empty")
	}

	return StatusMessages[rand.Intn(len(StatusMessages))], nil
}
