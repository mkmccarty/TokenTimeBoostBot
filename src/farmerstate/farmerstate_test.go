package farmerstate

import (
	"database/sql"
	"os"
	"slices"
	"testing"
)

func TestMain(m *testing.M) {
	db, _ := sql.Open("sqlite", ":memory:")
	_, _ = db.ExecContext(ctx, ddl)
	queries = New(db)
	farmerstate = make(map[string]*Farmer)
	os.Exit(m.Run())
}

func TestSetEggIncName(t *testing.T) {
	SetEggIncName("TestUser", "TestEggIncName")

	if GetEggIncName("TestUser") != farmerstate["TestUser"].EggIncName {
		t.Errorf("SetEggIncName() = %v, want %v", GetEggIncName("TestUser"), farmerstate["TestUser"].EggIncName)
	}
}

func TestGetEggIncName(t *testing.T) {
	SetEggIncName("TestUser", "TestEggIncName")

	if GetEggIncName("TestUser") != farmerstate["TestUser"].EggIncName {
		t.Errorf("GetEggIncName() = %v, want %v", GetEggIncName("TestUser"), farmerstate["TestUser"].EggIncName)
	}
}

func TestSetTokens(t *testing.T) {
	SetTokens("TestUser", 5)

	if GetTokens("TestUser") != farmerstate["TestUser"].Tokens {
		t.Errorf("SetTokens() = %v, want %v", GetTokens("TestUser"), farmerstate["TestUser"].Tokens)
	}
}

func TestGetPing(t *testing.T) {
	SetPing("TestUser", true)

	if GetPing("TestUser") != farmerstate["TestUser"].Ping {
		t.Errorf("SetTokens() = %v, want %v", GetPing("TestUser"), farmerstate["TestUser"].Ping)
	}

}

func TestGetEiIgnsByMiscString(t *testing.T) {
	SetMiscSettingString("user1", "guildID", "guild-123")
	SetMiscSettingString("user1", "ei_ign", "FarmerAlpha")
	SetMiscSettingString("user2", "guildID", "guild-123")
	SetMiscSettingString("user2", "ei_ign", "FarmerBeta")
	SetMiscSettingString("user3", "guildID", "guild-999")
	SetMiscSettingString("user3", "ei_ign", "FarmerGamma")

	got := GetEiIgnsByMiscString("guildID", "guild-123")

	if len(got) != 2 {
		t.Fatalf("GetEiIgnsByMiscString() returned %d results, want 2: %v", len(got), got)
	}
	for _, alias := range []string{"FarmerAlpha", "FarmerBeta"} {
		if !slices.Contains(got, alias) {
			t.Errorf("GetEiIgnsByMiscString() missing %q in %v", alias, got)
		}
	}
}

func TestGuildMembership(t *testing.T) {
	SetMiscSettingString("gm_user1", "ei_ign", "GuildFarmerA")
	SetMiscSettingString("gm_user2", "ei_ign", "GuildFarmerB")
	SetMiscSettingString("gm_user3", "ei_ign", "GuildFarmerC")

	// Add memberships
	if !AddGuildMembership("gm_user1", "g-abc") {
		t.Error("AddGuildMembership: expected true on first add")
	}
	AddGuildMembership("gm_user2", "g-abc")
	AddGuildMembership("gm_user3", "g-xyz")

	// Duplicate should return false
	if AddGuildMembership("gm_user1", "g-abc") {
		t.Error("AddGuildMembership: expected false on duplicate")
	}

	// GetGuildMembers
	members := GetGuildMembers("g-abc")
	if len(members) != 2 {
		t.Fatalf("GetGuildMembers() = %v, want 2 members", members)
	}

	// GetUserGuilds — user1 in g-abc, add them to a second guild
	AddGuildMembership("gm_user1", "g-xyz")
	guilds := GetUserGuilds("gm_user1")
	if len(guilds) != 2 {
		t.Fatalf("GetUserGuilds() = %v, want 2 guilds", guilds)
	}

	// GetEiIgnsByGuild
	aliases := GetEiIgnsByGuild("g-abc")
	if len(aliases) != 2 {
		t.Fatalf("GetEiIgnsByGuild() = %v, want 2 aliases", aliases)
	}
	for _, alias := range []string{"GuildFarmerA", "GuildFarmerB"} {
		if !slices.Contains(aliases, alias) {
			t.Errorf("GetEiIgnsByGuild() missing %q in %v", alias, aliases)
		}
	}

	// RemoveGuildMembership
	RemoveGuildMembership("gm_user1", "g-abc")
	members = GetGuildMembers("g-abc")
	if len(members) != 1 || members[0] != "gm_user2" {
		t.Errorf("after remove, GetGuildMembers() = %v, want [gm_user2]", members)
	}
}
