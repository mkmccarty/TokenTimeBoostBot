package boost

import (
	"encoding/json"
	"testing"

	"github.com/bwmarrin/discordgo"
)

// TestGuildRoleReadsLegacyContractJSON guards the on-disk compatibility of the
// contract blob. LocationData.GuildContractRole used to be a discordgo.Role, and
// contracts saved before that changed are still in SQLite. Those rows must keep
// loading with the role intact.
func TestGuildRoleReadsLegacyContractJSON(t *testing.T) {
	// A LocationData exactly as it was marshalled when the field was a discordgo.Role.
	legacy := []byte(`{
		"GuildID": "111",
		"GuildName": "Test Guild",
		"ChannelID": "222",
		"GuildContractRole": {
			"id": "333",
			"name": "Team Pumpkin",
			"managed": false,
			"mentionable": true,
			"hoist": false,
			"color": 1234,
			"position": 7,
			"permissions": "0",
			"icon": "",
			"unicode_emoji": ""
		},
		"RoleManagedByBot": true,
		"RoleMention": "<@&333>"
	}`)

	var loc LocationData
	if err := json.Unmarshal(legacy, &loc); err != nil {
		t.Fatalf("unmarshal legacy LocationData: %v", err)
	}

	if loc.GuildContractRole.ID != "333" {
		t.Errorf("role ID = %q, want %q", loc.GuildContractRole.ID, "333")
	}
	if loc.GuildContractRole.Name != "Team Pumpkin" {
		t.Errorf("role Name = %q, want %q", loc.GuildContractRole.Name, "Team Pumpkin")
	}
	if loc.RoleManagedByBot != true {
		t.Error("RoleManagedByBot = false, want true")
	}
}

// TestGuildRoleJSONMatchesDiscordShape verifies the stored representation still
// serializes to the same field names the library used, so a contract written by
// the new code can be read by an older build (and vice versa).
func TestGuildRoleJSONMatchesDiscordShape(t *testing.T) {
	stored, err := json.Marshal(GuildRole{ID: "444", Name: "Team Turnip"})
	if err != nil {
		t.Fatalf("marshal GuildRole: %v", err)
	}

	var asDiscordRole discordgo.Role
	if err := json.Unmarshal(stored, &asDiscordRole); err != nil {
		t.Fatalf("unmarshal GuildRole into discordgo.Role: %v", err)
	}

	if asDiscordRole.ID != "444" {
		t.Errorf("discordgo.Role.ID = %q, want %q", asDiscordRole.ID, "444")
	}
	if asDiscordRole.Name != "Team Turnip" {
		t.Errorf("discordgo.Role.Name = %q, want %q", asDiscordRole.Name, "Team Turnip")
	}
}

func TestGuildRoleFromDiscord(t *testing.T) {
	tests := []struct {
		name     string
		input    *discordgo.Role
		wantID   string
		wantName string
	}{
		{name: "nil role", input: nil, wantID: "", wantName: ""},
		{name: "populated role", input: &discordgo.Role{ID: "555", Name: "Team Radish", Color: 99}, wantID: "555", wantName: "Team Radish"},
		{name: "empty role", input: &discordgo.Role{}, wantID: "", wantName: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := guildRoleFromDiscord(tc.input)
			if got.ID != tc.wantID {
				t.Errorf("ID = %q, want %q", got.ID, tc.wantID)
			}
			if got.Name != tc.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tc.wantName)
			}
		})
	}
}

// TestGuildRoleMentionMatchesDiscord pins Mention() to the exact string the
// library produced, including the empty-ID case, so this refactor stays
// behaviour-neutral.
func TestGuildRoleMentionMatchesDiscord(t *testing.T) {
	for _, id := range []string{"666", ""} {
		local := GuildRole{ID: id}
		library := discordgo.Role{ID: id}
		if got, want := local.Mention(), library.Mention(); got != want {
			t.Errorf("Mention() with ID %q = %q, want %q", id, got, want)
		}
	}
}
