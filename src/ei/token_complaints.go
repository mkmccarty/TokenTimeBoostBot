package ei

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"strings"
	"sync"
	"time"
)

// TokenComplaintsFile is a struct to hold the token complaints
type TokenComplaintsFile struct {
	TokenComplaints []string `json:"token_complaints"`
}

const playerToken = "[player]"

// TokenComplaints is a list of token complaints
var TokenComplaints []string
var tokenComplaintsMutex sync.RWMutex
var loadedTokenComplaintsPath string
var loadedTokenComplaintsModTime time.Time
var loadedTokenComplaintsSize int64

const tokenComplaintsResortFlag = "__TTBB_TOKEN_COMPLAINTS_RESORT__"

// LoadTokenComplaints loads token complaints from a JSON file
func LoadTokenComplaints(filename string) {
	fileInfo, err := os.Stat(filename)
	if err != nil {
		log.Printf("Failed to stat token complaints file: %v", err)
		return
	}

	tokenComplaintsMutex.Lock()
	if loadedTokenComplaintsPath == filename &&
		loadedTokenComplaintsModTime.Equal(fileInfo.ModTime()) &&
		loadedTokenComplaintsSize == fileInfo.Size() {
		tokenComplaintsMutex.Unlock()
		return
	}
	tokenComplaintsMutex.Unlock()

	var complaintsLoaded TokenComplaintsFile

	file, err := os.Open(filename)
	if err != nil {
		log.Printf("Failed to open token complaints file: %v", err)
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Failed to close: %v", err)
		}
	}()
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&complaintsLoaded); err != nil {
		log.Printf("Failed to decode token complaints: %v", err)
		return
	}

	tokenComplaintsMutex.Lock()
	TokenComplaints = append([]string(nil), complaintsLoaded.TokenComplaints...)
	rand.Shuffle(len(TokenComplaints), func(i, j int) {
		TokenComplaints[i], TokenComplaints[j] = TokenComplaints[j], TokenComplaints[i]
	})
	if len(TokenComplaints) > 0 {
		TokenComplaints = append(TokenComplaints, tokenComplaintsResortFlag)
	}
	loadedTokenComplaintsPath = filename
	loadedTokenComplaintsModTime = fileInfo.ModTime()
	loadedTokenComplaintsSize = fileInfo.Size()
	tokenComplaintsMutex.Unlock()

	log.Printf("Loaded %d token complaints", len(complaintsLoaded.TokenComplaints))
}

// GetTokenComplaint returns the next complaint string from a shuffled queue for the given userName.
//
// Complaints are shuffled on load, then rotated in order so each complaint is used
// once before repeating. When a resort flag reaches the front of the queue,
// the queue is reshuffled.
func GetTokenComplaint(userName string) (string, error) {
	tokenComplaintsMutex.Lock()
	defer tokenComplaintsMutex.Unlock()

	if len(TokenComplaints) == 0 {
		return "", fmt.Errorf("TokenComplaints is empty")
	}

	if TokenComplaints[0] == tokenComplaintsResortFlag {
		TokenComplaints = TokenComplaints[1:]
		if len(TokenComplaints) == 0 {
			return "", fmt.Errorf("TokenComplaints is empty")
		}

		rand.Shuffle(len(TokenComplaints), func(i, j int) {
			TokenComplaints[i], TokenComplaints[j] = TokenComplaints[j], TokenComplaints[i]
		})
		TokenComplaints = append(TokenComplaints, tokenComplaintsResortFlag)
	}

	template := TokenComplaints[0]
	TokenComplaints = append(TokenComplaints[1:], template)

	return fmt.Sprintf(":loudspeaker: %s", strings.ReplaceAll(template, playerToken, userName)), nil
}
