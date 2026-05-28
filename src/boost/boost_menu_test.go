package boost

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

func mustSandboxArtifact(t *testing.T, key string) ei.Artifact {
	t.Helper()

	artifact := ei.GetArtifactByKey(key)
	if artifact != nil {
		return *artifact
	}
	t.Fatalf("artifact %q not found", key)
	return ei.Artifact{}
}

func TestSandboxPlayersFromContractKeepsDeflectorsSeparate(t *testing.T) {
	contract := &Contract{
		Order: []string{"u1"},
		Boosters: map[string]*Booster{
			"u1": {
				UserID:   "u1",
				Nick:     "Alpha",
				Register: time.Unix(1, 0),
				ArtifactSet: ArtifactSet{Artifacts: []ei.Artifact{
					mustSandboxArtifact(t, "D-T4L"),
					mustSandboxArtifact(t, "ID-T4L"),
					mustSandboxArtifact(t, "CH-T4L"),
					mustSandboxArtifact(t, "SIAB-T4L"),
				}},
			},
		},
	}

	players := sandboxPlayersFromContract(contract)
	if len(players) != 1 {
		t.Fatalf("expected 1 sandbox player, got %d", len(players))
	}

	player := players[0]
	if player.Item4 != "00" {
		t.Fatalf("delivery deflector slot = %q, want %q", player.Item4, "00")
	}
	if player.Item5 != "00" {
		t.Fatalf("chalice slot = %q, want %q", player.Item5, "00")
	}
	if player.Item6 != "00" {
		t.Fatalf("monocle slot = %q, want %q", player.Item6, "00")
	}
	if player.Item7 != "00" {
		t.Fatalf("IHR deflector slot = %q, want %q", player.Item7, "00")
	}
	if player.Item8 != "00" {
		t.Fatalf("IHR SIAB slot = %q, want %q", player.Item8, "00")
	}
}

func TestSandboxArtifactLabelsFromBooster(t *testing.T) {
	tests := []struct {
		name     string
		booster  *Booster
		expected []string
	}{
		{
			name:     "nil booster",
			booster:  nil,
			expected: nil,
		},
		{
			name: "empty artifacts",
			booster: &Booster{
				ArtifactSet: ArtifactSet{Artifacts: []ei.Artifact{}},
			},
			expected: []string{"T4L Chalice", "T4L Monocle", "3 Slot", "T4L IHR Defl."},
		},
		{
			name: "all specific artifacts present",
			booster: &Booster{
				ArtifactSet: ArtifactSet{
					Artifacts: []ei.Artifact{
						{Type: "Chalice", Quality: "T4E"},
						{Type: "Monocle", Quality: "T3C"},
						{Type: "SIAB", Quality: "T4R"},
						{Type: "IHR Deflector", Quality: "T4C"},
						{Type: "Metronome", Quality: "T4L"},
						{Type: "Compass", Quality: "T4L"},
						{Type: "Gusset", Quality: "T2E"},
						{Type: "Deflector", Quality: "T4E"},
					},
				},
			},
			expected: []string{
				"T4E Chalice", "T3C Monocle", "T4R SIAB", "T4C IHR Defl.",
				"T4L Metro", "T4L Comp", "T2E Gusset", "T4E Defl.",
			},
		},
		{
			name: "fallback IHR deflector uses standard deflector quality",
			booster: &Booster{
				ArtifactSet: ArtifactSet{
					Artifacts: []ei.Artifact{
						{Type: "Deflector", Quality: "T4E"},
					},
				},
			},
			expected: []string{
				"T4E Defl.", "T4L Chalice", "T4L Monocle", "3 Slot", "T4E IHR Defl.",
			},
		},
		{
			name: "stone slots handling with NONE quality",
			booster: &Booster{
				ArtifactSet: ArtifactSet{
					Artifacts: []ei.Artifact{
						{Type: "Unknown", Quality: "NONE", Stones: 3},
						{Type: "Unknown", Quality: "", Stones: 2},
						{Type: "Unknown", Quality: "T4L", Stones: 2},
					},
				},
			},
			expected: []string{
				"3 Slot", "2 Slot", "2 Slot",
				"T4L Chalice", "T4L Monocle", "3 Slot", "T4L IHR Defl.",
			},
		},
		{
			name: "SIAB via Bottle type",
			booster: &Booster{
				ArtifactSet: ArtifactSet{
					Artifacts: []ei.Artifact{
						{Type: "Ship In A Bottle", Quality: "T4L"},
					},
				},
			},
			expected: []string{
				"T4L SIAB", "T4L Chalice", "T4L Monocle", "T4L IHR Defl.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sandboxArtifactLabelsFromBooster(tt.booster)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("sandboxArtifactLabelsFromBooster() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDrawBoostListCompactRange(t *testing.T) {
	// Create a contract with 35 players, state Waiting (active).
	// We want to test earlyList (>10 players) and lateList (>10 players) compaction.
	contract := &Contract{
		ContractHash: "test-hash",
		ContractID:   "test-contract",
		CoopID:       "test-coop",
		State:        ContractStateWaiting,
		Style:        ContractStyleFastrun,
		CreatorID:    []string{"creator-id"},
		Order:        make([]string, 35),
		Boosters:     make(map[string]*Booster),
		Location:     []*LocationData{{GuildID: "guild1", ChannelID: "channel1"}},
	}

	for i := 0; i < 35; i++ {
		userID := fmt.Sprintf("user%d", i)
		contract.Order[i] = userID
		contract.Boosters[userID] = &Booster{
			UserID:       userID,
			Mention:      fmt.Sprintf("<@%d>", i),
			TokensWanted: 6,
			BoostState:   BoostStateUnboosted,
		}
	}

	// Set current booster index to 20.
	// This will make start = 20 - 6 = 14 (early list elements: 0 to 14, count = 14 > 10).
	// and end = 20 + 4 = 24 (late list elements: 24 to 35, count = 11 > 10).
	contract.CurrentBoosterUserID = "user20"
	contract.BoostPosition = 20

	// Also make sure we have some boosted/token time states if needed, but buildCompactRange just formats them.
	// We will call DrawBoostList.
	components := DrawBoostList(nil, contract)
	if len(components) == 0 {
		t.Fatalf("expected components, got none")
	}

	// We expect the earlyList and lateList text displays to contain compaction indicators.
	foundEarlyCompaction := false
	foundLateCompaction := false

	for _, comp := range components {
		if textDisplay, ok := comp.(*discordgo.TextDisplay); ok {
			content := textDisplay.Content
			if reflect.TypeOf(comp).String() == "*discordgo.TextDisplay" {
				if strings.Contains(content, "... (11 more) ...,") {
					foundEarlyCompaction = true
				}
				if strings.Contains(content, ", ... (8 more) ...") {
					// 35 - 24 = 11 total elements in lateList.
					// Compaction keeps first 3, so middleCount = 11 - 3 = 8.
					foundLateCompaction = true
				}
			}
		}
	}

	if !foundEarlyCompaction {
		t.Errorf("expected early list compaction '... (11 more) ...' not found in components")
	}
	if !foundLateCompaction {
		t.Errorf("expected late list compaction '... (8 more) ...' not found in components")
	}
}
