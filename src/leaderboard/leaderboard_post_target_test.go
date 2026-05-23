package leaderboard

import "testing"

func TestTargetMemberSet_GroupAndSingle(t *testing.T) {
	groupSet := targetMemberSet("group_misc")
	if len(groupSet) == 0 {
		t.Fatalf("expected non-empty target set for group")
	}
	if _, ok := groupSet[LBSoulMirrors]; !ok {
		t.Fatalf("expected group_misc target set to include %s", LBSoulMirrors)
	}

	singleSet := targetMemberSet(LBSoulMirrors)
	if len(singleSet) != 1 {
		t.Fatalf("expected single target set size 1, got %d", len(singleSet))
	}
	if _, ok := singleSet[LBSoulMirrors]; !ok {
		t.Fatalf("expected single target set to include %s", LBSoulMirrors)
	}
}

func TestIntersectsTarget(t *testing.T) {
	members := []string{LBDrones, LBEliteDrones, LBSoulMirrors}

	if !intersectsTarget(members, nil) {
		t.Fatalf("expected nil target set to match all members")
	}

	target := map[string]struct{}{LBSoulMirrors: {}}
	if !intersectsTarget(members, target) {
		t.Fatalf("expected target set to intersect members")
	}

	noMatch := map[string]struct{}{LBContractExp: {}}
	if intersectsTarget(members, noMatch) {
		t.Fatalf("expected target set with no overlap to not intersect")
	}
}

func TestShouldProcessMember(t *testing.T) {
	if !shouldProcessMember(LBSoulMirrors, nil) {
		t.Fatalf("expected nil target set to process all members")
	}

	target := map[string]struct{}{LBSoulMirrors: {}}
	if !shouldProcessMember(LBSoulMirrors, target) {
		t.Fatalf("expected matching member to be processed")
	}
	if shouldProcessMember(LBContractExp, target) {
		t.Fatalf("did not expect non-target member to be processed")
	}
}
