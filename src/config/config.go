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
	// OpenAIKey holds the API key for OpenAI.
	OpenAIKey string
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
	OpenAIKey        string   `json:"OpenAIKey"`
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
	fmt.Println("Reading from config file...")

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
	OpenAIKey = config.OpenAIKey
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
		// We'll create one and update the config file.

		key, err := GenerateKey()
		if err != nil {
			return err
		}
		Key = base64.StdEncoding.EncodeToString(key)

		// Want to read in the config file and update it with the new key
		config.Key = Key
		// Read the original file as a map to preserve unmapped fields
		var configMap map[string]interface{}
		err = json.Unmarshal(file, &configMap)
		if err != nil {
			return err
		}

		// Update only the Key field in the map
		configMap["Key"] = Key

		newFile, err := json.MarshalIndent(configMap, "", "  ")
		if err != nil {
			return err
		}
		err = os.WriteFile(cfgFile, newFile, 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

// IsDevBot returns true if the bot is running in development mode.
func IsDevBot() bool {
	return DiscordAppID == DevBotAppID
}

// GetTestMode returns the current test mode status.
func GetTestMode() bool {
	return TestMode
}
