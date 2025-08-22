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

// fetchEmojisFromDiscord fetches emojis from the discord API
func fetchEmojisFromDiscord(s *discordgo.Session) map[string]ei.Emotes {
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

	// Attempt to load the cached file
	fileInfo, err := os.Stat(emoteFilePath)
	if force || err != nil {
		if force || os.IsNotExist(err) {
			// File didn't eist load fresh data
			EmoteMapNew = fetchEmojisFromDiscord(s)
			ei.EmoteMap = EmoteMapNew
			saveEmotesToFile(emoteFilePath, EmoteMapNew)
			ei.EmoteMap = EmoteMapNew
			// ImportNewEmojis is intended to import new emojis to the Discord API
			ImportNewEmojis(s)
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
		EmoteMapNew = fetchEmojisFromDiscord(s)
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
		// Check if the URL is missing a protocol scheme and try to fix it
		if !strings.HasPrefix(IconURL, "https://") && !strings.HasPrefix(IconURL, "http://") {
			// Try fixing the URL by adding https:// prefix
			// Extract the part starting from www.auxbrain.com
			var urlToTry string
			if idx := strings.Index(IconURL, "www.auxbrain.com"); idx != -1 {
				urlToTry = IconURL[idx:]
			} else {
				urlToTry = IconURL
			}
			fixedURL := "https://" + urlToTry
			log.Printf("Attempting to fix URL from %s to %s", IconURL, fixedURL)
			resp, err = http.Get(fixedURL)
			if err != nil {
				log.Printf("Failed to fetch icon from fixed URL %s: %v", fixedURL, err)
				return "", fmt.Errorf("invalid URL: %s", IconURL)
			}
		} else {
			log.Printf("Failed to fetch icon from URL %s: %v", IconURL, err)
			return "", err
		}
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
	// The image should be resized to 128x128 pixels
	destinationImage := image.NewRGBA(image.Rect(0, 0, 128, 128))
	draw.NearestNeighbor.Scale(destinationImage, destinationImage.Rect, src, src.Bounds(), draw.Over, nil)

	var buf bytes.Buffer
	err = png.Encode(&buf, destinationImage)
	if err != nil {
		return "", err
	}

	// Save the image to a file
	filePath := fmt.Sprintf("%s/egg_%s.png", config.BannerPath, cleanEggID)
	file, err := os.Create(filePath)
	if err != nil {
		log.Print(err)
		return "", err
	}
	defer func() {
		if err := file.Close(); err != nil {
			// Handle the error appropriately, e.g., logging or taking corrective actions
			log.Printf("Failed to close: %v", err)
		}
	}()
	_, err = file.Write(buf.Bytes())
	if err != nil {
		log.Print(err)
		return "", err
	}
	base64Image := base64.StdEncoding.EncodeToString(buf.Bytes())

	data := discordgo.EmojiParams{
		Name:  fmt.Sprintf("egg_%s", cleanEggID),
		Image: "data:image/png;base64," + base64Image,
	}

	newID, err := s.ApplicationEmojiCreate(config.DiscordAppID, &data)
	if err != nil {
		log.Print(err)
		return "", err
	}
	emojiData := ei.Emotes{
		Name:     newID.Name,
		ID:       newID.ID,
		Animated: newID.Animated,
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
		emojiName := strings.TrimSuffix(strings.TrimSuffix(file.Name(), ".png"), ".gif")

		// Check if the emoji already exists
		if existingEmojis[strings.ToLower(emojiName)] {
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
			ei.EmoteMap[strings.ToLower(emojiName)] = ei.Emotes{Name: newID.Name, ID: newID.ID, Animated: newID.Animated}
		}(emojiName)
	}

	// Wait for all goroutines to finish
	wg.Wait()
	saveEmotesToFile(emoteFilePath, ei.EmoteMap)
}
