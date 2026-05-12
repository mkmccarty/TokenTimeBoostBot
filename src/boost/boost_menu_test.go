package boost

import (
	"reflect"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

func mustSandboxArtifact(t *testing.T, key string) ei.Artifact {
	t.Helper()

	artifact := ei.GetArtifactByKey(key)
	if artifact == nil {
		t.Fatalf("artifact %q not found", key)
	}

	return *artifact
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
