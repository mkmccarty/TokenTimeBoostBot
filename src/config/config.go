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
	config         *configStruct
)

type configStruct struct {
	DiscordToken   string `json:"DiscordToken"`
	DiscordAppID   string `json:"DiscordAppID"`
	DiscordGuildID string `json:"DiscordGuildID"`
	OpenAIKey      string `json:"OpenAIKey"`
	GoogleAPIKey   string `json:"GoogleAPIKey"`
	AdminUserID    string `json:"AdminUserId"`
}

// ReadConfig will load the configuration files for API tokens.
func ReadConfig() error {
	fmt.Println("Reading from config file...")

	file, err := os.ReadFile("./.config.json")

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
	DiscordGuildID = "" //config.DiscordGuildID
	OpenAIKey = config.OpenAIKey
	GoogleAPIKey = config.GoogleAPIKey
	AdminUserID = config.AdminUserID

	return nil
}
