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

	// Validate AdminUsers loaded
	if len(config.AdminUsers) != 2 {
		t.Errorf("AdminUsers length = %d, want %d", len(config.AdminUsers), 2)
	}
	// Validate GuildAdminUsers loaded in internal struct
	if config.GuildAdminUsers == nil {
		t.Fatalf("GuildAdminUsers not loaded into internal config struct")
	}
	if _, ok := config.GuildAdminUsers["guild-A"]; !ok {
		t.Fatalf("GuildAdminUsers missing key guild-A")
	}
	if len(config.GuildAdminUsers["guild-A"]) != 2 {
		t.Errorf("GuildAdminUsers[guild-A] length = %d, want %d", len(config.GuildAdminUsers["guild-A"]), 2)
	}

	// Validate exported variables were set as well
	if GuildAdminUsers == nil {
		t.Fatalf("GuildAdminUsers exported var not set")
	}
	if _, ok := GuildAdminUsers["guild-B"]; !ok {
		t.Fatalf("GuildAdminUsers missing key guild-B in exported var")
	}
	if len(GuildAdminUsers["guild-B"]) != 1 || GuildAdminUsers["guild-B"][0] != "adminB" {
		t.Errorf("GuildAdminUsers[guild-B] = %v, want [adminB]", GuildAdminUsers["guild-B"])
	}
}
