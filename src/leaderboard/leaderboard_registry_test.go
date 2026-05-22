package leaderboard

import "testing"

func TestVirtueTESumIsAliasedToVirtueEggsSum(t *testing.T) {
	if _, ok := LBDefByKey(LBVirtueTEEarnedSum); !ok {
		t.Fatalf("expected legacy key %s to resolve via alias", LBVirtueTEEarnedSum)
	}

	if got := resolveAlias(LBVirtueTEEarnedSum); got != LBVirtueEggsSum {
		t.Fatalf("expected %s to alias to %s, got %s", LBVirtueTEEarnedSum, LBVirtueEggsSum, got)
	}
}

func TestAllLeaderboardsExcludesVirtueTESum(t *testing.T) {
	for _, def := range AllLeaderboards {
		if def.Key == LBVirtueTEEarnedSum {
			t.Fatalf("did not expect merged key %s in AllLeaderboards", LBVirtueTEEarnedSum)
		}
	}
}

func TestExpandConfigKeyExpandsGroupMembership(t *testing.T) {
	got := ExpandConfigKey("group_misc")
	want := map[string]struct{}{
		LBDrones:      {},
		LBEliteDrones: {},
		LBSoulMirrors: {},
	}

	if len(got) != len(want) {
		t.Fatalf("expected %d keys for group_misc, got %d: %v", len(want), len(got), got)
	}

	for _, key := range got {
		if _, ok := want[key]; !ok {
			t.Fatalf("unexpected key %q in group_misc expansion: %v", key, got)
		}
		delete(want, key)
	}

	if len(want) != 0 {
		t.Fatalf("missing keys from group_misc expansion: %v", want)
	}
}

func TestPlayerOptInMatchingUsesExpandedGroupMembership(t *testing.T) {
	if !optInTypesInclude([]string{"group_misc"}, LBSoulMirrors) {
		t.Fatalf("expected group_misc opt-in to include %s", LBSoulMirrors)
	}

	if optInTypesInclude([]string{"group_misc"}, LBContractExp) {
		t.Fatalf("did not expect group_misc opt-in to include %s", LBContractExp)
	}
}

func optInTypesInclude(rawTypes []string, lbType string) bool {
	targetKey := resolveAlias(lbType)
	for _, rawType := range rawTypes {
		for _, optType := range ExpandConfigKey(rawType) {
			if resolveAlias(optType) == targetKey {
				return true
			}
		}
	}
	return false
}
