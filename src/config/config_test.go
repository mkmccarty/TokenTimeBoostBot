package config

import (
	"testing"
)

// Write a test for ReadConfig
// Make sure to use the testing package

func TestReadConfig(t *testing.T) {
	err := ReadConfig("cfg_test.json")
	if err != nil {
		t.Errorf("ReadConfig() error = %v", err)
		return
	}

	// Replace 'ExpectedValue' with the actual expected value
	if config.DiscordToken != "discord_token" {
		t.Errorf("ReadConfig() = %v, want %v", config.DiscordToken, "discord_token")
	}
	if config.DiscordAppID != "discord_app_id" {
		t.Errorf("ReadConfig() = %v, want %v", config.DiscordAppID, "discord_app_id")
	}
	if config.DiscordGuildID != "discord_guild_id" {
		t.Errorf("ReadConfig() = %v, want %v", config.DiscordGuildID, "discord_guild_id")
	}
}
