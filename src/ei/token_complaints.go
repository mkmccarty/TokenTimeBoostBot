package ei

import (
	jsonv2 "encoding/json/v2"
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
	if err := jsonv2.UnmarshalRead(file, &complaintsLoaded); err != nil {
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

var defaultFallbackComplaintSamples = []string{
	"A little birdie told [player] Kev doesn't particularly like them.",
	"[player] is manifesting tokens... unsuccessfully.",
	"Trucks keep arriving. [player] keeps learning things.",
	"“Oops… I did it again,” said [player].",
	"[player] is speedrunning everything except token gifts.",
	"USPS must have lost [player]'s tokens.",
	"“Wake me up when tokens arrive,” pleaded [player].",
	"Golden eggs again. Somewhere, [player] sighed.",
	"The system never promised fairness, only randomness, and [player] is learning the difference one empty truck at a time.",
	"\"Tokens exist elsewhere.\" - [player]",
	"“This is fine,” said [player], staring at the cash.",
	"[player] realized that they don't need tokens to boost.",
}

// GetRandomComplaintSamples returns a specified number of unique random token complaints from the loaded complaints
// (or diverse defaults if none loaded).
func GetRandomComplaintSamples(count int) []string {
	tokenComplaintsMutex.RLock()
	var pool []string
	for _, c := range TokenComplaints {
		if c != "" && c != tokenComplaintsResortFlag && strings.Contains(c, playerToken) {
			pool = append(pool, c)
		}
	}
	tokenComplaintsMutex.RUnlock()

	if len(pool) == 0 {
		pool = append([]string(nil), defaultFallbackComplaintSamples...)
	}

	if count <= 0 {
		count = 5
	}
	if count > len(pool) {
		count = len(pool)
	}

	shuffled := append([]string(nil), pool...)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	return shuffled[:count]
}
