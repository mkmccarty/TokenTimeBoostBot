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
}

func TestUpdateKey(t *testing.T) {
	newKeyBytes, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	newKeyB64 := string(newKeyBytes) // placeholder or base64
	files, err := UpdateKey(newKeyB64)
	if err != nil {
		t.Fatalf("UpdateKey failed: %v", err)
	}
	if Key != newKeyB64 {
		t.Errorf("Key = %q, want %q", Key, newKeyB64)
	}
	if len(files) == 0 {
		t.Errorf("expected at least one file updated, got 0")
	}
}

func TestEncryptionCycle(t *testing.T) {
	key1, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	key2, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	plain := []byte("SensitiveEggIncID-123456789")
	enc1, err := EncryptAndCombine(key1, plain)
	if err != nil {
		t.Fatalf("EncryptAndCombine failed: %v", err)
	}

	dec1, err := DecryptCombined(key1, enc1)
	if err != nil {
		t.Fatalf("DecryptCombined key1 failed: %v", err)
	}
	if string(dec1) != string(plain) {
		t.Fatalf("decrypted = %q, want %q", string(dec1), string(plain))
	}

	// Re-encrypt with key2
	enc2, err := EncryptAndCombine(key2, dec1)
	if err != nil {
		t.Fatalf("EncryptAndCombine key2 failed: %v", err)
	}

	dec2, err := DecryptCombined(key2, enc2)
	if err != nil {
		t.Fatalf("DecryptCombined key2 failed: %v", err)
	}
	if string(dec2) != string(plain) {
		t.Fatalf("decrypted = %q, want %q", string(dec2), string(plain))
	}
}
