package ei

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// StatusMessagesFile is a struct to hold the status messages
type StatusMessagesFile struct {
	StatusMessages []string `json:"status_messages"`
}

// StatusMessages is a list of status messages
var StatusMessages []string
var statusMessagesMutex sync.RWMutex
var loadedStatusMessagesPath string
var loadedStatusMessagesModTime time.Time
var loadedStatusMessagesSize int64
var nextStatusMessageOverride string

const statusMessageResortFlag = "__TTBB_STATUS_MESSAGES_RESORT__"

func loadStatusMessages(filename string, force bool) error {
	fileInfo, err := os.Stat(filename)
	if err != nil {
		return fmt.Errorf("failed to stat status messages file: %w", err)
	}

	statusMessagesMutex.Lock()
	if !force && loadedStatusMessagesPath == filename &&
		loadedStatusMessagesModTime.Equal(fileInfo.ModTime()) &&
		loadedStatusMessagesSize == fileInfo.Size() {
		statusMessagesMutex.Unlock()
		return nil
	}
	statusMessagesMutex.Unlock()

	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open status messages file: %w", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			log.Printf("Failed to close: %v", cerr)
		}
	}()

	var messagesLoaded StatusMessagesFile
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&messagesLoaded); err != nil {
		return fmt.Errorf("failed to decode status messages: %w", err)
	}

	statusMessagesMutex.Lock()
	StatusMessages = append([]string(nil), messagesLoaded.StatusMessages...)
	rand.Shuffle(len(StatusMessages), func(i, j int) {
		StatusMessages[i], StatusMessages[j] = StatusMessages[j], StatusMessages[i]
	})
	if len(StatusMessages) > 0 {
		StatusMessages = append(StatusMessages, statusMessageResortFlag)
	}
	loadedStatusMessagesPath = filename
	loadedStatusMessagesModTime = fileInfo.ModTime()
	loadedStatusMessagesSize = fileInfo.Size()
	statusMessagesMutex.Unlock()

	log.Printf("Loaded %d status messages", len(messagesLoaded.StatusMessages))
	return nil
}

// LoadStatusMessages loads status messages from a JSON file
func LoadStatusMessages(filename string) {
	if err := loadStatusMessages(filename, false); err != nil {
		log.Printf("%v", err)
	}
}

// GetRandomStatusMessage returns the next status message from a shuffled queue.
//
// Messages are shuffled on load, then rotated in order so each message is used
// once before repeating. When a resort flag reaches the front of the queue,
// the queue is reshuffled.
func GetRandomStatusMessage() (string, error) {
	statusMessagesMutex.Lock()

	if nextStatusMessageOverride != "" {
		override := nextStatusMessageOverride
		nextStatusMessageOverride = ""
		statusMessagesMutex.Unlock()
		return override, nil
	}

	if len(StatusMessages) == 0 {
		statusMessagesMutex.Unlock()
		return "", fmt.Errorf("StatusMessages is empty")
	}

	if StatusMessages[0] == statusMessageResortFlag {
		reloadPath := loadedStatusMessagesPath
		statusMessagesMutex.Unlock()

		if reloadPath != "" {
			if err := loadStatusMessages(reloadPath, true); err != nil {
				log.Printf("%v", err)
			}
		}

		statusMessagesMutex.Lock()
		if len(StatusMessages) == 0 {
			statusMessagesMutex.Unlock()
			return "", fmt.Errorf("StatusMessages is empty")
		}
		if StatusMessages[0] == statusMessageResortFlag {
			StatusMessages = StatusMessages[1:]
			if len(StatusMessages) == 0 {
				statusMessagesMutex.Unlock()
				return "", fmt.Errorf("StatusMessages is empty")
			}
			rand.Shuffle(len(StatusMessages), func(i, j int) {
				StatusMessages[i], StatusMessages[j] = StatusMessages[j], StatusMessages[i]
			})
			StatusMessages = append(StatusMessages, statusMessageResortFlag)
		}
	}

	activity := StatusMessages[0]
	StatusMessages = append(StatusMessages[1:], activity)
	statusMessagesMutex.Unlock()

	return activity, nil
}

// SetNextStatusMessageOverride sets a one-time status message that will be used
// on the next status update tick.
func SetNextStatusMessageOverride(message string) error {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return fmt.Errorf("status message cannot be empty")
	}

	statusMessagesMutex.Lock()
	nextStatusMessageOverride = trimmed
	statusMessagesMutex.Unlock()

	return nil
}

// GetStatusMessageChoices returns up to limit unique status messages filtered by
// a case-insensitive substring match.
func GetStatusMessageChoices(search string, limit int) []string {
	statusMessagesMutex.RLock()
	queueCopy := append([]string(nil), StatusMessages...)
	statusMessagesMutex.RUnlock()

	if limit <= 0 {
		limit = 25
	}

	searchLower := strings.ToLower(strings.TrimSpace(search))
	unique := make(map[string]struct{}, len(queueCopy))
	choices := make([]string, 0, limit)

	for _, msg := range queueCopy {
		if msg == "" || msg == statusMessageResortFlag {
			continue
		}
		if _, exists := unique[msg]; exists {
			continue
		}
		unique[msg] = struct{}{}

		if searchLower != "" && !strings.Contains(strings.ToLower(msg), searchLower) {
			continue
		}

		choices = append(choices, msg)
	}

	sort.Slice(choices, func(i, j int) bool {
		return strings.ToLower(choices[i]) < strings.ToLower(choices[j])
	})

	if len(choices) > limit {
		choices = choices[:limit]
	}

	return choices
}
