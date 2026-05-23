package leaderboard

import (
	"strings"
	"testing"
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
		if _, ok := groupKeys[c.Value.(string)]; !ok {
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
