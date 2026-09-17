package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
)

var (
	// DiscordToken holds the API Token for discord.
	DiscordToken string
	// DiscordAppID holds the application ID for the bot.
	DiscordAppID string
	// DiscordGuildID holds a GuildID to restring the bot to a single guild.
	DiscordGuildID string
	// GoogleAPIKey holds the API key for Google.
	GoogleAPIKey string
	// AdminUserID holds primary admin's DiscordID
	AdminUserID string
	// EIUserID holds the DiscordID used for EI API calls requiring Ultra (Periodicals)
	EIUserID string
	// EIUserIDBasic holds the DiscordID used for non-Ultra responses
	EIUserIDBasic string
	// AdminUsers holds a list of DiscordIDs for other admins
	AdminUsers []string
	// TestMode is true if the bot is running in test mode.
	TestMode bool
	// DevBotAppID is the application ID for the development bot.
	DevBotAppID string
	// FeatureFlags is a list of feature flags for the bot.
	FeatureFlags []string
	// EventsURL is the URL for the carpet-wasmegg events page.
	EventsURL string
	// RocketsURL is the URL for the carpet-wasmegg rockets page].
	RocketsURL string
	// GistToken is the token used to access the gist.
	GistToken string
	// GistID is the ID of the gist used for storage.
	GistID string
	// BannerPath is the path to the banner image and font files.
	BannerPath string
	// BannerOutputPath is the path where the compsited banner image will be saved.
	BannerOutputPath string
	// BannerURL is the URL to the banner image.
	BannerURL string
	// DevelopmentStaff is a list of user IDs for development staff.
	DevelopmentStaff []string
	// Key is the encryption key used for encrypting sensitive data.
	Key string

	config *configStruct
)

type configStruct struct {
	DiscordToken     string   `json:"DiscordToken"`
	DiscordAppID     string   `json:"DiscordAppID"`
	DiscordGuildID   string   `json:"DiscordGuildID"`
	GoogleAPIKey     string   `json:"GoogleAPIKey"`
	AdminUserID      string   `json:"AdminUserId"`
	EIUserIDBasic    string   `json:"EIUserIDBasic"`
	EIUserID         string   `json:"EIUserId"`
	AdminUsers       []string `json:"AdminUsers"`
	TestMode         bool     `json:"TestMode"`
	DevBotAppID      string   `json:"DevBotAppID"`
	FeatureFlags     []string `json:"FeatureFlags"`
	RocketsURL       string   `json:"RocketsURL"`
	EventsURL        string   `json:"EventsURL"`
	GistToken        string   `json:"GistToken"`
	GistID           string   `json:"GistID"`
	BannerPath       string   `json:"BannerPath"`
	BannerOutputPath string   `json:"BannerOutputPath"`
	BannerURL        string   `json:"BannerURL"`
	DevelopmentStaff []string `json:"DevelopmentStaff"`
	Key              string   `json:"Key"`
}

// ReadConfig will load the configuration files for API tokens.
func ReadConfig(cfgFile string) error {
	file, err := os.ReadFile(cfgFile)

	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	err = json.Unmarshal(file, &config)

	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	DiscordToken = config.DiscordToken
	DiscordAppID = config.DiscordAppID
	DiscordGuildID = config.DiscordGuildID
	GoogleAPIKey = config.GoogleAPIKey
	AdminUserID = config.AdminUserID
	EIUserID = config.EIUserID
	EIUserIDBasic = config.EIUserIDBasic
	AdminUsers = config.AdminUsers
	TestMode = config.TestMode
	DevBotAppID = config.DevBotAppID
	FeatureFlags = config.FeatureFlags
	EventsURL = config.EventsURL
	RocketsURL = config.RocketsURL
	GistToken = config.GistToken
	GistID = config.GistID
	BannerPath = config.BannerPath
	BannerOutputPath = config.BannerOutputPath
	BannerURL = config.BannerURL
	DevelopmentStaff = config.DevelopmentStaff
	Key = config.Key

	if Key == "" {
		// We need a encryption key for a few things, if it's missing
		// Try to read existing key from ttbb-data/.key.json first
		keyFile := "ttbb-data/.key.json"
		keyData, err := os.ReadFile(keyFile)
		if err == nil {
			// File exists, try to parse it
			var keyStruct struct {
				Key string `json:"Key"`
			}
			if json.Unmarshal(keyData, &keyStruct) == nil && keyStruct.Key != "" {
				Key = keyStruct.Key
				config.Key = Key
				return nil
			}
		}

		// Key file doesn't exist or is invalid, generate new key
		key, err := GenerateKey()
		if err != nil {
			return err
		}
		Key = base64.StdEncoding.EncodeToString(key)
		config.Key = Key

		// Save the new key to ttbb-data/key.json
		keyStruct := struct {
			Key string `json:"Key"`
		}{
			Key: Key,
		}
		keyJSON, err := json.MarshalIndent(keyStruct, "", "  ")
		if err != nil {
			return err
		}

		// Ensure directory exists
		if err := os.MkdirAll("ttbb-data", 0755); err != nil {
			return err
		}

		err = os.WriteFile(keyFile, keyJSON, 0644)
		if err != nil {
			return err
		}
	}
	configFilePath = cfgFile
	return nil
}

var configFilePath string

// UpdateKey updates the active encryption key in memory and persists it to disk.
// It checks both the loaded config file and ttbb-data/.key.json and returns the updated file paths.
func UpdateKey(newKey string) ([]string, error) {
	if newKey == "" {
		return nil, fmt.Errorf("newKey cannot be empty")
	}

	Key = newKey
	if config != nil {
		config.Key = newKey
	}

	var updatedFiles []string
	keyFile := "ttbb-data/.key.json"

	// 1. Update config file if it exists and had Key set
	if configFilePath != "" {
		cfgData, err := os.ReadFile(configFilePath)
		if err == nil {
			var rawMap map[string]interface{}
			if err := json.Unmarshal(cfgData, &rawMap); err == nil {
				if _, hasKey := rawMap["Key"]; hasKey {
					rawMap["Key"] = newKey
					formatted, err := json.MarshalIndent(rawMap, "", "  ")
					if err == nil {
						if err := os.WriteFile(configFilePath, formatted, 0644); err == nil {
							updatedFiles = append(updatedFiles, configFilePath)
						}
					}
				}
			}
		}
	}

	// 2. Update ttbb-data/.key.json if it exists or if no config file was updated
	_, keyFileErr := os.Stat(keyFile)
	if keyFileErr == nil || len(updatedFiles) == 0 {
		keyStruct := struct {
			Key string `json:"Key"`
		}{
			Key: newKey,
		}
		keyJSON, err := json.MarshalIndent(keyStruct, "", "  ")
		if err != nil {
			return updatedFiles, err
		}
		if err := os.MkdirAll("ttbb-data", 0755); err != nil {
			return updatedFiles, err
		}
		if err := os.WriteFile(keyFile, keyJSON, 0644); err != nil {
			return updatedFiles, err
		}
		updatedFiles = append(updatedFiles, keyFile)
	}

	return updatedFiles, nil
}

// IsDevBot returns true if the bot is running in development mode.
func IsDevBot() bool {
	return DiscordAppID == DevBotAppID
}

// GetTestMode returns the current test mode status.
func GetTestMode() bool {
	return TestMode
}
