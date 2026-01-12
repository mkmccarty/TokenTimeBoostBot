package ei

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
)

type TokenComplaintsFile struct {
	TokenComplaints []string `json:"token_complaints"`
}

const playerToken = "[player]"

var TokenComplaints []string

// LoadTokenComplaints loads token complaints from a JSON file
func LoadTokenComplaints(filename string) {
	var complaintsLoaded TokenComplaintsFile

	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Failed to close: %v", err)
		}
	}()
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&complaintsLoaded); err != nil {
		log.Print(err)
		return
	}

	TokenComplaints = complaintsLoaded.TokenComplaints
}

// GetTokenComplaint returns a random complaint string for the given userName
func GetTokenComplaint(userName string) (string, error) {
	if len(TokenComplaints) == 0 {
		return "", fmt.Errorf("TokenComplaints is empty")
	}
	/*
		log.Printf("TokenComplaints debug dump (%d total):", len(TokenComplaints))
		for idx, template := range TokenComplaints {
			log.Printf("  [%d] %s", idx, strings.ReplaceAll(template, playerToken, userName))
		}
	*/
	template := TokenComplaints[rand.Intn(len(TokenComplaints))]
	return fmt.Sprintf(":loudspeaker: %s", strings.ReplaceAll(template, playerToken, userName)), nil
}
