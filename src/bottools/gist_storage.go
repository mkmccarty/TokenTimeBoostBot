package bottools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"config"
)

/*


	err1 := bottools.PutGistData("boostbot.json", "{\"test\":\"test\"}")
	if err1 != nil {
		log.Println(err1.Error())
	}

	str, err2 := bottools.GetGistData("boostbot.json")
	if err2 != nil {
		log.Println(err2.Error())
	}
	log.Print(str)
*/

// GetGistData retrieves data from a Gist and unmarshals it into the target interface.
func GetGistData(filename string) (string, error) {
	resp, err := http.Get(fmt.Sprintf("https://api.github.com/gists/%s", config.GistID))
	if err != nil {
		return "", fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Handle the error appropriately, e.g., logging or taking corrective actions
			log.Printf("Failed to close: %v", err)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received non-200 response code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var gistData map[string]interface{}
	if err := json.Unmarshal(body, &gistData); err != nil {
		return "", fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	files, ok := gistData["files"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("failed to parse files from gist data")
	}

	file, ok := files[filename].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("failed to parse file from gist data")
	}

	content, ok := file["content"].(string)
	if !ok {
		return "", fmt.Errorf("failed to parse content from gist data")
	}

	body = []byte(content)

	return string(body), nil
}

// PutGistData uploads data to a Gist.
func PutGistData(filename string, data string) error {
	reqBody := map[string]interface{}{
		"files": map[string]interface{}{
			filename: map[string]string{
				"content": data,
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	req, err := http.NewRequest(http.MethodPatch, fmt.Sprintf("https://api.github.com/gists/%s", config.GistID), bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.GistToken))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Handle the error appropriately, e.g., logging or taking corrective actions
			log.Printf("Failed to close: %v", err)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-200 response code: %d", resp.StatusCode)
	}

	return nil
}
