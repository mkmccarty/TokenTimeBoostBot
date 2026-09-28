package boost

import (
	"strings"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

func TestBoosterDisplayName(t *testing.T) {
	tests := []struct {
		name      string
		mention   string
		nick      string
		discordID string
		want      string
	}{
		{
			name:      "mention with <@",
			mention:   "<@123456789>",
			nick:      "SomeNick",
			discordID: "123456789",
			want:      "<@123456789>",
		},
		{
			name:      "nick without mention",
			mention:   "",
			nick:      "FarmerJohn",
			discordID: "123456789",
			want:      "`FarmerJohn`",
		},
		{
			name:      "plain text mention",
			mention:   "PlainMention",
			nick:      "",
			discordID: "123456789",
			want:      "`PlainMention`",
		},
		{
			name:      "empty mention and nick fallbacks to discordID",
			mention:   "",
			nick:      "",
			discordID: "123456789",
			want:      "<@123456789>",
		},
		{
			name:      "all empty",
			mention:   "",
			nick:      "",
			discordID: "",
			want:      "`Unknown`",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := boosterDisplayName(tc.mention, tc.nick, tc.discordID)
			if got != tc.want {
				t.Errorf("boosterDisplayName(%q, %q, %q) = %q, want %q", tc.mention, tc.nick, tc.discordID, got, tc.want)
			}
		})
	}
}

func TestBuildLobbyMismatchSection(t *testing.T) {
	contributors := []*ei.ContractCoopStatusResponse_ContributionInfo{
		{
			UserId:   protoRef("user-1"),
			UserName: protoRef("Alice"),
		},
		{
			UserId:   protoRef("user-2"),
			UserName: protoRef("Bob"),
		},
	}

	contract := &Contract{
		ContractID: "test-contract",
		CoopID:     "test-coop",
		Boosters:   make(map[string]*Booster),
	}
	contract.Boosters["1001"] = &Booster{
		UserID:  "1001",
		Nick:    "Alice",
		Mention: "<@1001>",
	}
	contract.Boosters["1002"] = &Booster{
		UserID:  "1002",
		Nick:    "",
		Mention: "",
	}

	mismatch := buildLobbyMismatchSection(contributors, contract)
	if strings.Contains(mismatch, "- ``") {
		t.Errorf("mismatch contains empty code block: %q", mismatch)
	}
	if strings.Contains(mismatch, "Alice") {
		t.Errorf("expected Alice to be cleanly matched without mismatch, got %q", mismatch)
	}
	if !strings.Contains(mismatch, "<@1002>") {
		t.Errorf("expected booster 1002 to be displayed as <@1002>, got %q", mismatch)
	}
	if !strings.Contains(mismatch, "Bob") {
		t.Errorf("expected contributor Bob in mismatch, got %q", mismatch)
	}
}

func TestBuildLobbyMismatchSection_ExactDiscordMatch(t *testing.T) {
	// Tests exact case-insensitive match against Discord Username/Nick/GlobalName
	contributors := []*ei.ContractCoopStatusResponse_ContributionInfo{
		{
			UserId:   protoRef("dummy-user-1"),
			UserName: protoRef("kannabie"),
		},
	}

	contract := &Contract{
		ContractID: "test-contract",
		CoopID:     "test-coop",
		Boosters:   make(map[string]*Booster),
	}
	contract.Boosters["dummy-discord-1"] = &Booster{
		UserID:     "dummy-discord-1",
		Nick:       "kannabie",
		UserName:   "kannabie",
		GlobalName: "kannabie",
		Mention:    "<@dummy-discord-1>",
	}

	mismatch := buildLobbyMismatchSection(contributors, contract)
	if mismatch != "" {
		t.Errorf("expected no mismatch for identical name, got %q", mismatch)
	}
}

func TestBuildLobbyMismatchSection_SubstringMatch(t *testing.T) {
	// Tests unverified substring matching when name is not an exact match
	contributors := []*ei.ContractCoopStatusResponse_ContributionInfo{
		{
			UserId:   protoRef("dummy-user-1"),
			UserName: protoRef("FarmerDan_Alt"),
		},
	}

	contract := &Contract{
		ContractID: "test-contract",
		CoopID:     "test-coop",
		Boosters:   make(map[string]*Booster),
	}
	contract.Boosters["dummy-discord-1"] = &Booster{
		UserID:     "dummy-discord-1",
		Nick:       "FarmerDan",
		UserName:   "farmerdan",
		GlobalName: "FarmerDan",
		Mention:    "<@dummy-discord-1>",
	}

	mismatch := buildLobbyMismatchSection(contributors, contract)
	if !strings.Contains(mismatch, "Possible matches (unverified):") {
		t.Errorf("expected unverified possible match for substring, got %q", mismatch)
	}
	if !strings.Contains(mismatch, "FarmerDan_Alt") || !strings.Contains(mismatch, "<@dummy-discord-1>") {
		t.Errorf("expected substring match between FarmerDan_Alt and booster, got %q", mismatch)
	}
}

func protoRef(s string) *string {
	return &s
}
