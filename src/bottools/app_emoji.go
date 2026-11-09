package bottools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/go-github/v71/github"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"

	"golang.org/x/image/draw"
)

const emoteFilePath = "ttbb-data/Emotes.json"

var (
	emojiImportMu        sync.Mutex
	failedEmojiLookups   = make(map[string]time.Time)
	failedEmojiLookupsMu sync.Mutex
)

func isEmojiLookupCoolingDown(name string) bool {
	failedEmojiLookupsMu.Lock()
	defer failedEmojiLookupsMu.Unlock()
	t, ok := failedEmojiLookups[name]
	if !ok {
		return false
	}
	if time.Since(t) > 2*time.Minute {
		delete(failedEmojiLookups, name)
		return false
	}
	return true
}

func recordFailedEmojiLookup(name string) {
	failedEmojiLookupsMu.Lock()
	failedEmojiLookups[name] = time.Now()
	failedEmojiLookupsMu.Unlock()
}

func clearFailedEmojiLookup(names ...string) {
	failedEmojiLookupsMu.Lock()
	if len(names) == 0 {
		failedEmojiLookups = make(map[string]time.Time)
	} else {
		for _, n := range names {
			delete(failedEmojiLookups, n)
		}
	}
	failedEmojiLookupsMu.Unlock()
}

// fetchEmojisFromDiscord fetches emojis from the discord API
func fetchEmojisFromDiscord(client dc.Client) map[string]ei.Emotes {
	emotes := make(map[string]ei.Emotes)
	appEmoji, err := client.ApplicationEmojis(config.DiscordAppID)
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

// LoadEmotesWithClient will load all the emojis from the app using the dc facade client.
func LoadEmotesWithClient(client dc.Client, force bool) {
	ei.SetEmojiResolver(func(name string) (ei.Emotes, bool) {
		return EnsureEmojiFromLocalRepoWithClient(client, name)
	})

	var EmoteMapNew map[string]ei.Emotes

	fileInfo, err := os.Stat(emoteFilePath)
	if force || err != nil {
		EmoteMapNew = fetchEmojisFromDiscord(client)
		ei.EmoteMap = EmoteMapNew
		saveEmotesToFile(emoteFilePath, EmoteMapNew)
		return
	}

	EmoteMapNew, err = loadEmotesFromFile(emoteFilePath)
	if err != nil {
		log.Print(err)
	}
	ei.EmoteMap = EmoteMapNew

	// Refresh cache if file is older than 24 hours
	if len(EmoteMapNew) == 0 || time.Since(fileInfo.ModTime()) > 24*time.Hour {
		EmoteMapNew = fetchEmojisFromDiscord(client)
		if len(EmoteMapNew) != len(ei.EmoteMap) {
			ei.EmoteMap = EmoteMapNew
			saveEmotesToFile(emoteFilePath, EmoteMapNew)
		}
	}
}

// ReloadEmotes removes the cached emoji file, fetches all application emojis from Discord,
// updates in-memory mapping, and saves the new cache file.
func ReloadEmotes(client dc.Client) (int, error) {
	clearFailedEmojiLookup()
	_ = os.Remove(emoteFilePath)
	_ = os.Remove("ttbb-data/emoji.json")
	LoadEmotesWithClient(client, true)
	return len(ei.EmoteMap), nil
}

// EnsureEmojiFromLocalRepoWithClient tries to load a missing emoji from the local emoji directory,
// uploads it to Discord via the dc facade client, and refreshes the in-memory emoji map.
func EnsureEmojiFromLocalRepoWithClient(client dc.Client, name string) (ei.Emotes, bool) {
	emojiName := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(name, "-", "")))
	if emojiName == "" {
		return ei.Emotes{}, false
	}

	if emoji, ok := ei.EmoteMap[emojiName]; ok {
		return emoji, true
	}

	if isEmojiLookupCoolingDown(emojiName) {
		return ei.Emotes{}, false
	}

	emojiImportMu.Lock()
	defer emojiImportMu.Unlock()

	if emoji, ok := ei.EmoteMap[emojiName]; ok {
		return emoji, true
	}

	// On cache miss, first fetch fresh emoji list from Discord API to check if it's already uploaded
	refreshed := fetchEmojisFromDiscord(client)
	if len(refreshed) > 0 {
		ei.EmoteMap = refreshed
		saveEmotesToFile(emoteFilePath, ei.EmoteMap)
		if emoji, found := ei.EmoteMap[emojiName]; found {
			clearFailedEmojiLookup(emojiName, name)
			return emoji, true
		}
	}

	// If still missing after Discord fetch, look up image in local emoji/ directory
	emojiPath, ok := findLocalEmojiPath(emojiName)
	if !ok {
		// Attempt to download the missing image from git repository
		emojiPath, ok = downloadEmojiFromGit(name, emojiName)
		if !ok {
			recordFailedEmojiLookup(emojiName)
			log.Printf("Emoji %s not found locally or in git repository", emojiName)
			return ei.Emotes{}, false
		}
	}

	createdEmoji, err := importSingleEmojiFromPath(client, emojiName, emojiPath)
	if err != nil {
		recordFailedEmojiLookup(emojiName)
		log.Printf("Error importing missing local emoji %s: %v", emojiName, err)
		return ei.Emotes{}, false
	}

	// Refresh map to store Discord assigned ID
	refreshed = fetchEmojisFromDiscord(client)
	if len(refreshed) > 0 {
		ei.EmoteMap = refreshed
		saveEmotesToFile(emoteFilePath, ei.EmoteMap)
		if emoji, found := ei.EmoteMap[emojiName]; found {
			ei.EmoteMap[strings.ToLower(name)] = emoji
			clearFailedEmojiLookup(emojiName, name)
			return emoji, true
		}
	}

	ei.EmoteMap[emojiName] = createdEmoji
	ei.EmoteMap[strings.ToLower(name)] = createdEmoji
	clearFailedEmojiLookup(emojiName, name)
	saveEmotesToFile(emoteFilePath, ei.EmoteMap)
	return createdEmoji, true
}

func downloadEmojiFromGit(name, emojiName string) (string, bool) {
	if err := os.MkdirAll("emoji", 0755); err != nil {
		return "", false
	}

	rawBaseURL := "https://raw.githubusercontent.com/mkmccarty/TokenTimeBoostBot/main/emoji/"

	candidates := []string{
		emojiName + ".png",
		emojiName + ".gif",
		name + ".png",
		name + ".gif",
		strings.ToLower(name) + ".png",
		strings.ToLower(name) + ".gif",
		strings.ReplaceAll(strings.ToLower(name), "-", "_") + ".png",
		strings.ReplaceAll(strings.ToLower(name), "-", "_") + ".gif",
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	seen := make(map[string]bool)

	for _, cand := range candidates {
		if seen[cand] {
			continue
		}
		seen[cand] = true

		url := rawBaseURL + cand
		req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "TokenTimeBoostBot/1.0")

		resp, err := httpClient.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode == http.StatusOK {
			data, err := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if err != nil || len(data) == 0 {
				continue
			}

			localPath := filepath.Join("emoji", cand)
			if err := os.WriteFile(localPath, data, 0644); err != nil {
				log.Printf("Failed to write downloaded emoji %s to %s: %v", cand, localPath, err)
				return "", false
			}

			log.Printf("Downloaded missing emoji %s from git (%s)", emojiName, url)
			return localPath, true
		}
		_ = resp.Body.Close()
	}

	// Fallback: query GitHub API directory listing for emoji/
	client := github.NewClient(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, directoryContents, _, err := client.Repositories.GetContents(ctx, "mkmccarty", "TokenTimeBoostBot", "emoji", &github.RepositoryContentGetOptions{
		Ref: "main",
	})
	if err != nil {
		return "", false
	}

	cleanTarget := strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(name), "-", ""), "_", "")
	for _, content := range directoryContents {
		if content.GetType() != "file" || !isEmojiFile(content.GetName()) {
			continue
		}
		candidateBase := strings.TrimSuffix(content.GetName(), filepath.Ext(content.GetName()))
		cleanCandidate := strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(candidateBase), "-", ""), "_", "")
		if cleanCandidate == cleanTarget || strings.EqualFold(candidateBase, emojiName) || strings.EqualFold(candidateBase, name) {
			downloadURL := content.GetDownloadURL()
			if downloadURL == "" {
				continue
			}
			req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
			if err != nil {
				continue
			}
			resp, err := httpClient.Do(req)
			if err != nil {
				continue
			}
			if resp.StatusCode == http.StatusOK {
				data, err := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if err != nil || len(data) == 0 {
					continue
				}
				localPath := filepath.Join("emoji", content.GetName())
				if err := os.WriteFile(localPath, data, 0644); err != nil {
					return "", false
				}
				log.Printf("Downloaded missing emoji %s from GitHub API (%s)", emojiName, downloadURL)
				return localPath, true
			}
			_ = resp.Body.Close()
		}
	}

	return "", false
}

func findLocalEmojiPath(emojiName string) (string, bool) {
	files, err := os.ReadDir("emoji")
	if err != nil {
		return "", false
	}

	for _, file := range files {
		if file.IsDir() || !isEmojiFile(file.Name()) {
			continue
		}
		candidate := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
		if strings.EqualFold(candidate, emojiName) {
			return filepath.Join("emoji", file.Name()), true
		}
	}

	// Fallback: match after removing hyphens and underscores
	cleanTarget := strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(emojiName), "-", ""), "_", "")
	for _, file := range files {
		if file.IsDir() || !isEmojiFile(file.Name()) {
			continue
		}
		candidate := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
		cleanCandidate := strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(candidate), "-", ""), "_", "")
		if cleanCandidate == cleanTarget {
			return filepath.Join("emoji", file.Name()), true
		}
	}

	return "", false
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

// ImportEggImageWithClient will import an egg image into the discord app using the dc facade client.
func ImportEggImageWithClient(client dc.Client, eggID, IconURL string) (string, error) {

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

	// Write this image to the config.BannerPath directory for banner creation
	if _, err := os.Stat(config.BannerPath); os.IsNotExist(err) {
		_ = os.MkdirAll(config.BannerPath, 0755)
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

	// Also write to local emoji/ directory if it exists
	if _, err := os.Stat("emoji"); err == nil {
		_ = os.WriteFile(filepath.Join("emoji", fmt.Sprintf("egg_%s.png", cleanEggID)), buf.Bytes(), 0644)
	}

	// Automatically commit egg image to GitHub repository if not already added
	go func(filename string, data []byte) {
		if _, err := CommitEmojiToGitHub(filename, data); err != nil {
			log.Printf("Automatic GitHub commit for %s failed: %v", filename, err)
		}
	}(fmt.Sprintf("egg_%s.png", cleanEggID), buf.Bytes())

	base64Image := base64.StdEncoding.EncodeToString(buf.Bytes())

	var emojiData ei.Emotes
	newID, err := client.ApplicationEmojiCreate(config.DiscordAppID, dc.EmojiParams{
		Name:  fmt.Sprintf("egg_%s", cleanEggID),
		Image: "data:image/png;base64," + base64Image,
	})
	if err != nil {
		log.Printf("ApplicationEmojiCreate for egg_%s failed: %v; checking if already present on Discord", cleanEggID, err)
		refreshed := fetchEmojisFromDiscord(client)
		if len(refreshed) > 0 {
			ei.EmoteMap = refreshed
			saveEmotesToFile(emoteFilePath, ei.EmoteMap)
			targetName := strings.ToLower(fmt.Sprintf("egg_%s", cleanEggID))
			if em, found := ei.EmoteMap[targetName]; found {
				ei.EmoteMap[eggID] = em
				ei.EmoteMap[cleanEggID] = em
				saveEmotesToFile(emoteFilePath, ei.EmoteMap)
				clearFailedEmojiLookup(cleanEggID, targetName, eggID)
				return fmt.Sprintf(":%s:%s:", em.Name, em.ID), nil
			}
		}
		return "", err
	}

	emojiData = ei.Emotes{
		Name:     newID.Name,
		ID:       newID.ID,
		Animated: newID.Animated,
	}
	ei.EmoteMap[eggID] = emojiData
	ei.EmoteMap[strings.ToLower(newID.Name)] = emojiData
	ei.EmoteMap[fmt.Sprintf("egg_%s", cleanEggID)] = emojiData
	ei.EmoteMap[cleanEggID] = emojiData

	clearFailedEmojiLookup(cleanEggID, fmt.Sprintf("egg_%s", cleanEggID), eggID)
	saveEmotesToFile(emoteFilePath, ei.EmoteMap)
	return fmt.Sprintf(":%s:%s:", newID.Name, newID.ID), nil
}

// CommitEmojiToGitHub commits an emoji file to the GitHub repository under emoji/<filename>.
// It first checks if the file has already been added to avoid duplicate commits (e.g. between dev and prod).
// It returns (committed bool, err error).
func CommitEmojiToGitHub(filename string, content []byte) (bool, error) {
	token := config.GetGitHubToken()
	if token == "" {
		log.Printf("GitHub token not configured; skipping automatic commit of %s", filename)
		return false, nil
	}

	owner := "mkmccarty"
	repo := "TokenTimeBoostBot"
	filePath := fmt.Sprintf("emoji/%s", filename)

	client := github.NewClient(nil).WithAuthToken(token)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check if file already exists in repository to prevent duplicate commits
	fileContent, _, resp, err := client.Repositories.GetContents(ctx, owner, repo, filePath, &github.RepositoryContentGetOptions{
		Ref: "main",
	})
	if err == nil && fileContent != nil {
		log.Printf("Emoji %s already exists on GitHub repository, skipping commit", filePath)
		return false, nil
	}
	if resp != nil && resp.StatusCode != http.StatusNotFound {
		log.Printf("Error checking existence of %s on GitHub: %v", filePath, err)
		return false, fmt.Errorf("failed to check existence of %s on GitHub: %w", filePath, err)
	}

	opts := &github.RepositoryContentFileOptions{
		Message: github.Ptr(fmt.Sprintf("feat(emoji): add %s", filename)),
		Content: content,
		Branch:  github.Ptr("main"),
		Committer: &github.CommitAuthor{
			Name:  github.Ptr("TokenTimeBoostBot"),
			Email: github.Ptr("bot@tokentimeboostbot"),
		},
	}

	_, resp, err = client.Repositories.CreateFile(ctx, owner, repo, filePath, opts)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnprocessableEntity {
			log.Printf("Emoji %s was already committed concurrently to GitHub", filePath)
			return false, nil
		}
		log.Printf("Failed to commit %s to GitHub: %v", filePath, err)
		return false, err
	}

	log.Printf("Successfully committed %s to GitHub (%s/%s)", filePath, owner, repo)
	return true, nil
}

// SyncMissingEmojisToGitHub scans BannerPath and emoji/ directories for egg_*.png and commits any missing ones to GitHub.
func SyncMissingEmojisToGitHub() (int, int, error) {
	token := config.GetGitHubToken()
	if token == "" {
		return 0, 0, fmt.Errorf("GitHub token not configured (set GitHubToken in config or GITHUB_TOKEN environment variable)")
	}

	dirs := []string{"emoji"}
	if config.BannerPath != "" && config.BannerPath != "emoji" {
		dirs = append(dirs, config.BannerPath)
	}

	seen := make(map[string]string) // filename -> filepath
	for _, dir := range dirs {
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			name := f.Name()
			if strings.HasPrefix(name, "egg_") && strings.HasSuffix(name, ".png") {
				if _, exists := seen[name]; !exists {
					seen[name] = filepath.Join(dir, name)
				}
			}
		}
	}

	total := len(seen)
	committed := 0

	for filename, fullPath := range seen {
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		newlyCommitted, err := CommitEmojiToGitHub(filename, data)
		if err != nil {
			log.Printf("SyncMissingEmojisToGitHub: failed for %s: %v", filename, err)
			continue
		}
		if newlyCommitted {
			committed++
		}
	}

	return committed, total, nil
}

func isEmojiFile(fileName string) bool {
	lower := strings.ToLower(fileName)
	if strings.HasPrefix(lower, "banner") {
		return false
	}
	allowedSuffixes := []string{".png", ".gif"}
	for _, suffix := range allowedSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

// ImportNewEmojisWithClient will import new emojis from the emoji directory into the discord app using the dc facade client.
func ImportNewEmojisWithClient(client dc.Client) {
	if config.IsDevBot() {
		return
	}

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
	var importedCount int32

	// Loop through all files in the emoji directory
	for _, file := range files {
		// Check if the file is an emoji image
		if !isEmojiFile(file.Name()) {
			continue
		}

		emojiName := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
		// Check if the emoji already exists
		if existingEmojis[strings.ToLower(emojiName)] {
			continue
		}
		f := file
		wg.Go(func() {
			importSingleEmoji(client, emojiName, f)
			atomic.AddInt32(&importedCount, 1)
		})
	}

	// Wait for all goroutines to finish
	wg.Wait()

	if importedCount > 0 {
		refreshed := fetchEmojisFromDiscord(client)
		if len(refreshed) > 0 {
			ei.EmoteMap = refreshed
			saveEmotesToFile(emoteFilePath, ei.EmoteMap)
		}
	}
}

func importSingleEmoji(client dc.Client, emojiName string, file os.DirEntry) {
	emojiData, err := importSingleEmojiFromPath(client, emojiName, filepath.Join("emoji", file.Name()))
	if err != nil {
		log.Println("Error creating emoji in Discord:", err)
		return
	}
	ei.EmoteMap[strings.ToLower(emojiName)] = emojiData
}

func importSingleEmojiFromPath(client dc.Client, emojiName, emojiPath string) (ei.Emotes, error) {
	imageBytes, err := os.ReadFile(emojiPath)
	if err != nil {
		return ei.Emotes{}, err
	}

	fileSuffix := strings.TrimPrefix(strings.ToLower(filepath.Ext(emojiPath)), ".")
	base64Image := fmt.Sprintf("data:image/%s;base64,%s", fileSuffix, base64.StdEncoding.EncodeToString(imageBytes))

	newID, err := client.ApplicationEmojiCreate(config.DiscordAppID, dc.EmojiParams{
		Name:  emojiName,
		Image: base64Image,
	})
	if err != nil {
		return ei.Emotes{}, err
	}

	return ei.Emotes{Name: newID.Name, ID: newID.ID, Animated: newID.Animated}, nil
}
