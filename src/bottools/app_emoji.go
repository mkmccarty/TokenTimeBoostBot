package bottools

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"golang.org/x/image/draw"
)

const emoteFilePath = "ttbb-data/Emotes.json"

// fetchEmojis fetches emojis from the discord API
func fetchEmojis(s *discordgo.Session) map[string]ei.Emotes {
	emotes := make(map[string]ei.Emotes)
	appEmoji, err := s.ApplicationEmojis(config.DiscordAppID)
	if err != nil {
		return emotes
	}
	for _, e := range appEmoji {
		emotes[strings.ToLower(e.Name)] = ei.Emotes{
			Name:     e.Name,
			ID:       e.ID,
			Animated: e.Animated,
		}
	}
	return emotes
}

// LoadEmotes will load all the emojis from the app
func LoadEmotes(s *discordgo.Session, force bool) {
	EmoteMapNew := make(map[string]ei.Emotes)

	// Attempt to load the file
	fileInfo, err := os.Stat(emoteFilePath)
	if force || err != nil {
		if force || os.IsNotExist(err) {
			// File didn't eist load fresh data
			EmoteMapNew = fetchEmojis(s)
			ei.EmoteMap = EmoteMapNew
			saveEmotesToFile(emoteFilePath, EmoteMapNew)
			ei.EmoteMap = EmoteMapNew
			return
		}
	} else {
		// File exists, load the data
		EmoteMapNew, err = loadEmotesFromFile(emoteFilePath)
		if err != nil {
			log.Print(err)
		}
		ei.EmoteMap = EmoteMapNew
	}

	// If data is empty or the file is older than 1 day, fetch new data
	if len(EmoteMapNew) == 0 || time.Since(fileInfo.ModTime()) > 24*time.Hour {
		EmoteMapNew = fetchEmojis(s)
		if len(EmoteMapNew) != len(ei.EmoteMap) {
			ei.EmoteMap = EmoteMapNew
			saveEmotesToFile(emoteFilePath, EmoteMapNew)
		}
	}
}

// saveEmotesToFile saves the emotes to a file
func saveEmotesToFile(emoteFilePath string, emotes map[string]ei.Emotes) {
	file, err := os.Create(emoteFilePath)
	if err != nil {
		log.Print(err)
		return
	}

	defer func() {
		if err := file.Close(); err != nil {
			// Handle the error appropriately, e.g., logging or taking corrective actions
			log.Printf("Failed to close: %v", err)
		}
	}()

	encoder := json.NewEncoder(file)
	err = encoder.Encode(emotes)
	if err != nil {
		log.Print(err)
		return
	}
}

// loadEmotesFromFile loads the emotes from a file
func loadEmotesFromFile(emoteFilePath string) (map[string]ei.Emotes, error) {
	file, err := os.Open(emoteFilePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			// Handle the error appropriately, e.g., logging or taking corrective actions
			log.Printf("Failed to close: %v", err)
		}
	}()
	var emotes map[string]ei.Emotes
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&emotes)
	if err != nil {
		return nil, err
	}
	return emotes, nil
}

// ImportEggImage will import an egg image into the discord app
func ImportEggImage(s *discordgo.Session, eggID, IconURL string) (string, error) {

	// Read the icon URL into memory
	resp, err := http.Get(IconURL)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Handle the error appropriately, e.g., logging or taking corrective actions
			log.Printf("Failed to close: %v", err)
		}
	}()

	iconData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var src image.Image
	src, _, err = image.Decode(strings.NewReader(string(iconData)))
	if err != nil {
		return "", err
	}

	// Create a new variable for egg.ID, but remove spaces and hyphens from its name and make it lowercase
	cleanEggID := strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(eggID), " ", ""), "-", "")
	// Do something with iconData if needed
	destinationImage := image.NewRGBA(image.Rect(0, 0, src.Bounds().Max.X/4, src.Bounds().Max.Y/4))
	draw.NearestNeighbor.Scale(destinationImage, destinationImage.Rect, src, src.Bounds(), draw.Over, nil)
	//output, _ := os.Create(fmt.Sprintf("egg_%s.png", cleanEggID))
	//defer output.Close()
	//png.Encode(output, destinationImage)

	var buf bytes.Buffer
	err = png.Encode(&buf, destinationImage)
	if err != nil {
		return "", err
	}
	base64Image := base64.StdEncoding.EncodeToString(buf.Bytes())

	//
	const DiscordURIPng = "data:image/png;base64,"
	//const DiscordURIJpg = "data:image/jpeg;base64,"
	//const DiscordURIGif = "data:image/gif;base64,"

	data := discordgo.EmojiParams{
		Name:  fmt.Sprintf("egg_%s", cleanEggID),
		Image: DiscordURIPng + base64Image,
	}

	newID, err := s.ApplicationEmojiCreate(config.DiscordAppID, &data)
	if err != nil {
		log.Print(err)
		return "", err
	}
	emojiData := ei.Emotes{
		Name: newID.Name,
		ID:   newID.ID,
	}
	ei.EmoteMap[eggID] = emojiData
	saveEmotesToFile(emoteFilePath, ei.EmoteMap)
	return fmt.Sprintf(":%s:%s:", newID.Name, newID.ID), nil
}

// ImportNewEmojis will import new emojis from the emoji directory into the discord app
func ImportNewEmojis(s *discordgo.Session) {
	// Get a list of all files in the emoji directory
	files, err := os.ReadDir("emoji")
	if err != nil {
		// Likely just running in production
		return
	}

	// Create a map of existing emoji names for quick lookup
	existingEmojis := make(map[string]bool)
	for name := range ei.EmoteMap {
		existingEmojis[name] = true
	}

	// Create a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Loop through all files in the emoji directory
	for _, file := range files {
		// Check if the file is a PNG image
		if !strings.HasSuffix(file.Name(), ".png") && !strings.HasSuffix(file.Name(), ".gif") {
			continue
		}

		fileType := strings.ToLower(file.Name()[len(file.Name())-3:]) // Extract the emoji name from the file name
		emojiName := strings.ToLower(strings.TrimSuffix(strings.TrimSuffix(file.Name(), ".png"), ".gif"))

		// Check if the emoji already exists
		if existingEmojis[emojiName] {
			continue
		}

		// Add to the wait group
		wg.Add(1)

		// Launch a goroutine to import the emoji
		go func(emojiName string) {
			defer wg.Done()

			// Open the emoji file
			emojiFile, err := os.Open("emoji/" + emojiName + "." + fileType)
			if err != nil {
				log.Println("Error opening emoji file:", err)
				return
			}
			defer func() {
				if err := emojiFile.Close(); err != nil {
					// Handle the error appropriately, e.g., logging or taking corrective actions
					log.Printf("Failed to close: %v", err)
				}
			}()

			// Read the emoji file into memory
			reader := bufio.NewReader(emojiFile)
			var buf bytes.Buffer
			_, err = io.Copy(&buf, reader)
			if err != nil {
				log.Println("Error reading emoji file:", err)
				return
			}
			// Encode the emoji image
			fileSuffix := strings.ToLower(file.Name()[len(file.Name())-3:])
			base64Image := fmt.Sprintf("data:image/%s;base64,%s", fileSuffix, base64.StdEncoding.EncodeToString(buf.Bytes()))

			// Create the emoji in Discord
			data := discordgo.EmojiParams{
				Name:  emojiName,
				Image: base64Image,
			}
			newID, err := s.ApplicationEmojiCreate(config.DiscordAppID, &data)
			if err != nil {
				log.Println("Error creating emoji in Discord:", err)
				return
			}
			ei.EmoteMap[emojiName] = ei.Emotes{Name: newID.Name, ID: newID.ID}
		}(emojiName)
	}

	// Wait for all goroutines to finish
	wg.Wait()
	saveEmotesToFile(emoteFilePath, ei.EmoteMap)
}
