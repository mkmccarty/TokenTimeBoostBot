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
