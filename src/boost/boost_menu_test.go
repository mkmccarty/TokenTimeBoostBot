package boost

import (
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
	if player.Item6 != "05" {
		t.Fatalf("monocle slot = %q, want %q", player.Item6, "05")
	}
	if player.Item7 != "00" {
		t.Fatalf("IHR deflector slot = %q, want %q", player.Item7, "00")
	}
	if player.Item8 != "00" {
		t.Fatalf("IHR SIAB slot = %q, want %q", player.Item8, "00")
	}
}
