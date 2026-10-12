package leaderboard

import (
	"strings"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

func TestBuildAutocompleteChoices_DefaultShowsGroups(t *testing.T) {
	choices := buildAutocompleteChoices("", false)
	if len(choices) == 0 {
		t.Fatalf("expected group choices for empty autocomplete")
	}

	groupKeys := make(map[string]struct{}, len(AllGroups))
	for _, g := range AllGroups {
		groupKeys[g.Key] = struct{}{}
	}

	for _, c := range choices {
		if _, ok := groupKeys[c.Value]; !ok {
			t.Fatalf("expected only group values for empty autocomplete, got %q", c.Value)
		}
	}
}

func TestBuildAutocompleteChoices_AdminSearchIncludesGroupedIndividual(t *testing.T) {
	choices := buildAutocompleteChoices("soul_mirrors", false)

	found := false
	for _, c := range choices {
		if c.Value == LBSoulMirrors {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("expected admin autocomplete search to include %s", LBSoulMirrors)
	}
}

func TestLeaderboardChoiceNameIncludesGroupTags(t *testing.T) {
	def, ok := LBDefByKey(LBSoulMirrors)
	if !ok {
		t.Fatalf("expected leaderboard definition for %s", LBSoulMirrors)
	}

	name := leaderboardChoiceName(def)
	if !strings.Contains(name, "(MISC)") {
		t.Fatalf("expected group tag in choice name, got %q", name)
	}
}

func TestAdminLBPruneCommandDefinition(t *testing.T) {
	cmd := GetSlashAdminLBCommand("admin-lb")
	foundPrune := false
	for _, opt := range cmd.Options {
		if sub, ok := opt.(dc.SubCommand); ok && sub.Name == "prune" {
			foundPrune = true
			if len(sub.Options) != 1 {
				t.Fatalf("expected 1 option in prune subcommand, got %d", len(sub.Options))
			}
			if bOpt, ok := sub.Options[0].(dc.BoolOption); !ok || bOpt.Name != "redraw" {
				t.Fatalf("expected redraw bool option in prune subcommand, got %v", sub.Options[0])
			}
			break
		}
	}
	if !foundPrune {
		t.Fatalf("expected prune subcommand in /admin-lb command definition")
	}
}
