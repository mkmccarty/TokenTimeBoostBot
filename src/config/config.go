package config

import (
	"encoding/json"
	"fmt"
	"os"
)

var (
	// DiscordToken holds the API Token for discord.
	DiscordToken   string
	DiscordAppID   string
	DiscordGuildID string
	OpenAIKey      string
	GoogleAPIKey   string
	AdminUserID    string
	EIUserID       string
	EIUserIDBasic  string
	AdminUsers     []string
	TestMode       bool
	DevBotAppID    string
	FeatureFlags   []string
	EventsURL      string
	RocketsURL     string
	config         *configStruct
)

type configStruct struct {
	DiscordToken   string   `json:"DiscordToken"`
	DiscordAppID   string   `json:"DiscordAppID"`
	DiscordGuildID string   `json:"DiscordGuildID"`
	OpenAIKey      string   `json:"OpenAIKey"`
	GoogleAPIKey   string   `json:"GoogleAPIKey"`
	AdminUserID    string   `json:"AdminUserId"`
	EIUserIDBasic  string   `json:"EIUserIDBasic"`
	EIUserID       string   `json:"EIUserId"`
	AdminUsers     []string `json:"AdminUsers"`
	TestMode       bool     `json:"TestMode"`
	DevBotAppID    string   `json:"DevBotAppID"`
	FeatureFlags   []string `json:"FeatureFlags"`
	RocketsURL     string   `json:"RocketsURL"`
	EventsURL      string   `json:"EventsURL"`
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
