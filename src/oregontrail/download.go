package oregontrail

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// OregonBasURL is the raw source URL for the original 1975 Oregon Trail BASIC source.
	OregonBasURL = "https://raw.githubusercontent.com/clintmoyer/oregon-trail/master/oregon.bas"
	// OregonBasLocalPath is the data file path in ttbb-data.
	OregonBasLocalPath = "ttbb-data/oregon.bas"
)

var (
	progMu     sync.RWMutex
	cachedProg *Program
)

// resolveOregonBasPath returns the local path to oregon.bas.
// It checks standard locations (e.g. current directory or repo root when running in package tests).
func resolveOregonBasPath() string {
	candidates := []string{
		OregonBasLocalPath,
		filepath.Join("..", "..", OregonBasLocalPath),
		filepath.Join("..", OregonBasLocalPath),
	}
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Size() > 0 {
			return p
		}
	}
	if _, err := os.Stat(filepath.Join("..", "..", "go.mod")); err == nil {
		return filepath.Join("..", "..", OregonBasLocalPath)
	}
	return OregonBasLocalPath
}

// DownloadSource downloads the raw BASIC source during launch.
func DownloadSource() error {
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, OregonBasURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request for oregon.bas: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download oregon.bas: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading oregon.bas returned status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read oregon.bas body: %w", err)
	}

	if len(body) == 0 {
		return fmt.Errorf("downloaded oregon.bas is empty")
	}

	localPath := resolveOregonBasPath()
	// Ensure destination directory exists
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for oregon.bas: %w", err)
	}

	if err := os.WriteFile(localPath, body, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", localPath, err)
	}

	log.Printf("oregontrail: successfully downloaded %s (%d bytes)", localPath, len(body))

	// Invalidate cached program so it reloads fresh
	progMu.Lock()
	cachedProg = nil
	progMu.Unlock()

	return nil
}

// GetProgramSource returns the source code of oregon.bas from ttbb-data/oregon.bas,
// downloading it first if it does not yet exist.
func GetProgramSource() (string, error) {
	localPath := resolveOregonBasPath()
	data, err := os.ReadFile(localPath)
	if err == nil && len(data) > 0 {
		return string(data), nil
	}

	// If missing, attempt download
	if err := DownloadSource(); err != nil {
		return "", fmt.Errorf("oregon.bas not found and download failed: %w", err)
	}

	localPath = resolveOregonBasPath()
	data, err = os.ReadFile(localPath)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", localPath, err)
	}
	return string(data), nil
}

// GetProgram returns the parsed Program AST, caching it for fast session starts.
func GetProgram() (*Program, error) {
	progMu.RLock()
	if cachedProg != nil {
		p := cachedProg
		progMu.RUnlock()
		return p, nil
	}
	progMu.RUnlock()

	src, err := GetProgramSource()
	if err != nil {
		return nil, err
	}

	prog, err := ParseProgram(src)
	if err != nil {
		return nil, fmt.Errorf("failed to parse oregon.bas: %w", err)
	}

	progMu.Lock()
	cachedProg = prog
	progMu.Unlock()

	return prog, nil
}
