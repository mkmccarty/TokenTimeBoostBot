package boost

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestBoostOrderUnselected(t *testing.T) {
	original := []string{"u1", "u2", "u3", "u4"}
	selected := []string{"u2", "u4"}
	got := boostOrderUnselected(original, selected)
	want := []string{"u1", "u3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected unselected list: got=%v want=%v", got, want)
	}
}

func TestBoostOrderVisiblePage(t *testing.T) {
	values := make([]string, 0, boostOrderPageSize+4)
	for i := 1; i <= boostOrderPageSize+4; i++ {
		values = append(values, "u"+strconv.Itoa(i))
	}

	page0 := boostOrderVisiblePage(values, 0)
	if len(page0) != boostOrderPageSize {
		t.Fatalf("expected first page to have %d items, got %d", boostOrderPageSize, len(page0))
	}
	page1 := boostOrderVisiblePage(values, 1)
	if len(page1) != 4 {
		t.Fatalf("expected second page to have 4 items, got %d", len(page1))
	}
}

func TestApplyBoostOrderSelectionActiveContractSetsNextBooster(t *testing.T) {
	contract := &Contract{
		State:                ContractStateFastrun,
		Order:                []string{"u1", "u2", "u3"},
		CurrentBoosterUserID: "u3",
		BoostPosition:        2,
		Boosters: map[string]*Booster{
			"u1": {BoostState: BoostStateBoosted},
			"u2": {BoostState: BoostStateUnboosted},
			"u3": {BoostState: BoostStateBoosted},
		},
	}

	applyBoostOrderSelection(contract, []string{"u3", "u1", "u2"}, true)

	if !reflect.DeepEqual(contract.Order, []string{"u3", "u1", "u2"}) {
		t.Fatalf("unexpected order: %v", contract.Order)
	}
	if contract.CurrentBoosterUserID != "u2" {
		t.Fatalf("expected current booster u2, got %q", contract.CurrentBoosterUserID)
	}
	if contract.OrderRevision != 1 {
		t.Fatalf("expected order revision 1, got %d", contract.OrderRevision)
	}
}

func TestApplyBoostOrderSelectionSignupDoesNotChangeCurrent(t *testing.T) {
	contract := &Contract{
		State:                ContractStateSignup,
		Order:                []string{"u1", "u2"},
		CurrentBoosterUserID: "u2",
		BoostPosition:        1,
		Boosters: map[string]*Booster{
			"u1": {BoostState: BoostStateUnboosted},
			"u2": {BoostState: BoostStateUnboosted},
		},
	}

	applyBoostOrderSelection(contract, []string{"u2", "u1"}, true)

	if contract.CurrentBoosterUserID != "u2" {
		t.Fatalf("expected current booster to remain unchanged in signup, got %q", contract.CurrentBoosterUserID)
	}
}

func TestBoostOrderButtonLabelIncludesTE(t *testing.T) {
	contract := &Contract{
		State:      ContractStateFastrun,
		BoostOrder: ContractOrderSignup,
		Boosters: map[string]*Booster{
			"u1": {Nick: "VeryLongNickNameForTesting", TECount: 123},
		},
	}

	label := boostOrderButtonLabel(contract, "u1")
	if !strings.Contains(label, "(TE:123)") {
		t.Fatalf("expected TE suffix in label, got %q", label)
	}
}

func TestBoostOrderButtonLabelHidesZeroTE(t *testing.T) {
	contract := &Contract{
		State:      ContractStateFastrun,
		BoostOrder: ContractOrderSignup,
		Boosters: map[string]*Booster{
			"u1": {Nick: "Alpha", TECount: 0},
		},
	}

	label := boostOrderButtonLabel(contract, "u1")
	if strings.Contains(label, "(TE:0)") {
		t.Fatalf("did not expect zero TE suffix in label, got %q", label)
	}
}

func TestBoostOrderButtonLabelHidesUnknownTE(t *testing.T) {
	contract := &Contract{
		State:      ContractStateFastrun,
		BoostOrder: ContractOrderSignup,
		Boosters: map[string]*Booster{
			"u1": {Nick: "Alpha", TECount: -1},
		},
	}

	label := boostOrderButtonLabel(contract, "u1")
	if strings.Contains(label, "(TE:") {
		t.Fatalf("did not expect unknown TE suffix in label, got %q", label)
	}
}

func TestBoostOrderButtonLabelIncludesELR(t *testing.T) {
	contract := &Contract{
		State:      ContractStateFastrun,
		BoostOrder: ContractOrderELR,
		Boosters: map[string]*Booster{
			"u1": {Nick: "Alpha", ArtifactSet: ArtifactSet{LayRate: 1.2345}},
		},
	}

	label := boostOrderButtonLabel(contract, "u1")
	if !strings.Contains(label, "(ELR:1.23)") {
		t.Fatalf("expected ELR suffix in label, got %q", label)
	}
}

func TestClearBoostOrderSessionsForUserContract(t *testing.T) {
	boostOrderSessions = map[string]*boostOrderSession{
		"keep-other-contract": {
			xid:          "keep-other-contract",
			userID:       "u1",
			contractHash: "c2",
			expiresAt:    time.Now().Add(time.Minute),
		},
		"remove-this": {
			xid:          "remove-this",
			userID:       "u1",
			contractHash: "c1",
			expiresAt:    time.Now().Add(time.Minute),
		},
		"keep-other-user": {
			xid:          "keep-other-user",
			userID:       "u2",
			contractHash: "c1",
			expiresAt:    time.Now().Add(time.Minute),
		},
	}

	clearBoostOrderSessionsForUserContract("u1", "c1")

	if _, ok := boostOrderSessions["remove-this"]; ok {
		t.Fatalf("expected matching session to be removed")
	}
	if _, ok := boostOrderSessions["keep-other-contract"]; !ok {
		t.Fatalf("expected different-contract session to be kept")
	}
	if _, ok := boostOrderSessions["keep-other-user"]; !ok {
		t.Fatalf("expected different-user session to be kept")
	}
}

func TestBoostOrderSeededSelectionStartedContract(t *testing.T) {
	contract := &Contract{
		State: ContractStateFastrun,
		Order: []string{"u1", "u2", "u3", "u4"},
		Boosters: map[string]*Booster{
			"u1": {BoostState: BoostStateBoosted},
			"u2": {BoostState: BoostStateUnboosted},
			"u3": {BoostState: BoostStateBoosted},
			"u4": {BoostState: BoostStateTokenTime},
		},
	}

	got := boostOrderSeededSelection(contract)
	want := []string{"u1", "u3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected seeded selection: got=%v want=%v", got, want)
	}
}

func TestBoostOrderSeededSelectionSignupContract(t *testing.T) {
	contract := &Contract{
		State: ContractStateSignup,
		Order: []string{"u1", "u2"},
		Boosters: map[string]*Booster{
			"u1": {BoostState: BoostStateBoosted},
			"u2": {BoostState: BoostStateBoosted},
		},
	}

	got := boostOrderSeededSelection(contract)
	if len(got) != 0 {
		t.Fatalf("expected no seeded users in signup state, got=%v", got)
	}
}

func TestBoostOrderExclude(t *testing.T) {
	values := []string{"u1", "u2", "u3", "u4"}
	excludes := []string{"u1", "u3"}

	got := boostOrderExclude(values, excludes)
	want := []string{"u2", "u4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected filtered values: got=%v want=%v", got, want)
	}
}

func TestBoostOrderHasReorderTargetsFalseWhenAllBoosted(t *testing.T) {
	contract := &Contract{
		State: ContractStateFastrun,
		Order: []string{"u1", "u2"},
		Boosters: map[string]*Booster{
			"u1": {BoostState: BoostStateBoosted},
			"u2": {BoostState: BoostStateBoosted},
		},
	}

	if boostOrderHasReorderTargets(contract) {
		t.Fatalf("expected no reorder targets when everyone is boosted")
	}
}

func TestBoostOrderHasReorderTargetsTrueWhenAnyUnboosted(t *testing.T) {
	contract := &Contract{
		State: ContractStateFastrun,
		Order: []string{"u1", "u2"},
		Boosters: map[string]*Booster{
			"u1": {BoostState: BoostStateBoosted},
			"u2": {BoostState: BoostStateUnboosted},
		},
	}

	if !boostOrderHasReorderTargets(contract) {
		t.Fatalf("expected reorder targets when at least one player is unboosted")
	}
}

func TestBoostOrderUndoRemovesPreviousFillStep(t *testing.T) {
	session := &boostOrderSession{
		selected:  []string{"u1", "u4", "u2", "u3", "u5"},
		undoSteps: []int{1, 3},
	}

	removedIDs, removed := boostOrderUndoLastStep(session)
	if removed != 3 {
		t.Fatalf("expected undo to remove fill step of 3, got %d", removed)
	}
	wantSelected := []string{"u1", "u4"}
	if !reflect.DeepEqual(session.selected, wantSelected) {
		t.Fatalf("unexpected selected after undo: got=%v want=%v", session.selected, wantSelected)
	}
	wantRemovedIDs := []string{"u2", "u3", "u5"}
	if !reflect.DeepEqual(removedIDs, wantRemovedIDs) {
		t.Fatalf("unexpected removed IDs: got=%v want=%v", removedIDs, wantRemovedIDs)
	}
	wantSteps := []int{1}
	if !reflect.DeepEqual(session.undoSteps, wantSteps) {
		t.Fatalf("unexpected undo steps after undo: got=%v want=%v", session.undoSteps, wantSteps)
	}
}

func TestBoostOrderUndoReverseMode(t *testing.T) {
	session := &boostOrderSession{
		selected:    []string{"u1", "u2", "u3", "u4"},
		undoSteps:   []int{1, -2},
		bottomCount: 2,
	}

	removedIDs, removed := boostOrderUndoLastStep(session)
	if removed != 2 {
		t.Fatalf("expected undo to remove 2 items, got %d", removed)
	}
	if session.bottomCount != 0 {
		t.Fatalf("expected bottomCount to be 0, got %d", session.bottomCount)
	}
	wantSelected := []string{"u1", "u2"}
	if !reflect.DeepEqual(session.selected, wantSelected) {
		t.Fatalf("unexpected selected after undo: got=%v want=%v", session.selected, wantSelected)
	}
	wantRemovedIDs := []string{"u3", "u4"}
	if !reflect.DeepEqual(removedIDs, wantRemovedIDs) {
		t.Fatalf("unexpected removed IDs: got=%v want=%v", removedIDs, wantRemovedIDs)
	}
	wantSteps := []int{1}
	if !reflect.DeepEqual(session.undoSteps, wantSteps) {
		t.Fatalf("unexpected undo steps after undo: got=%v want=%v", session.undoSteps, wantSteps)
	}
}

func TestBoostOrderCommandPath(t *testing.T) {
	if got := boostOrderCommandPath(""); got != "/boost-order" {
		t.Fatalf("expected default command path '/boost-order', got %q", got)
	}
	if got := boostOrderCommandPath("catalyst"); got != "/catalyst" {
		t.Fatalf("expected alias command path '/catalyst', got %q", got)
	}
}

func TestApplyBoostOrderSelectionKeepCurrentBooster(t *testing.T) {
	// Test that when changeCurrentBooster is false, the current booster is kept in new position
	contract := &Contract{
		State:                ContractStateFastrun,
		Order:                []string{"u1", "u2", "u3"},
		CurrentBoosterUserID: "u2",
		BoostPosition:        1,
		Boosters: map[string]*Booster{
			"u1": {BoostState: BoostStateBoosted},
			"u2": {BoostState: BoostStateUnboosted},
			"u3": {BoostState: BoostStateUnboosted},
		},
	}

	applyBoostOrderSelection(contract, []string{"u3", "u2", "u1"}, false)

	if contract.CurrentBoosterUserID != "u2" {
		t.Fatalf("expected current booster to remain u2 when changeCurrentBooster=false, got %q", contract.CurrentBoosterUserID)
	}
	if !reflect.DeepEqual(contract.Order, []string{"u3", "u2", "u1"}) {
		t.Fatalf("expected order [u3 u2 u1], got %v", contract.Order)
	}
}

func TestApplyBoostOrderSelectionKeepCurrentBoosterRemovedFallbackToFirstUnboosted(t *testing.T) {
	// Test that when current booster is removed and changeCurrentBooster=false, fallback to first unboosted
	contract := &Contract{
		State:                ContractStateFastrun,
		Order:                []string{"u1", "u2", "u3"},
		CurrentBoosterUserID: "u2",
		BoostPosition:        1,
		Boosters: map[string]*Booster{
			"u1": {BoostState: BoostStateBoosted},
			"u2": {BoostState: BoostStateUnboosted},
			"u3": {BoostState: BoostStateUnboosted},
		},
	}

	// Removed u2 from boosters but it's in the new order for test setup; we'll use u1, u3
	applyBoostOrderSelection(contract, []string{"u1", "u3"}, false)

	// u2 was removed (not in new order), so should fallback to first unboosted which is u3
	if contract.CurrentBoosterUserID != "u3" {
		t.Fatalf("expected current booster to fallback to u3 after u2 removed, got %q", contract.CurrentBoosterUserID)
	}
}

func TestApplyBoostOrderSelectionKeepCurrentBoosterRemovedAllBoosted(t *testing.T) {
	// Test that when current booster is removed and all others are boosted, currentBooster clears
	contract := &Contract{
		State:                ContractStateFastrun,
		Order:                []string{"u1", "u2", "u3"},
		CurrentBoosterUserID: "u2",
		BoostPosition:        1,
		Boosters: map[string]*Booster{
			"u1": {BoostState: BoostStateBoosted},
			"u2": {BoostState: BoostStateUnboosted},
			"u3": {BoostState: BoostStateBoosted},
		},
	}

	applyBoostOrderSelection(contract, []string{"u1", "u3"}, false)

	// u2 removed, both u1 and u3 are boosted, so should clear current booster
	if contract.CurrentBoosterUserID != "" {
		t.Fatalf("expected current booster to clear when all remaining are boosted, got %q", contract.CurrentBoosterUserID)
	}
}

func TestApplyBoostOrderSelectionUpdatesStartTimeWhenCurrentChanges(t *testing.T) {
	oldStart := time.Now().Add(-10 * time.Minute)
	contract := &Contract{
		State:                ContractStateFastrun,
		Order:                []string{"u1", "u2"},
		CurrentBoosterUserID: "u1",
		BoostPosition:        0,
		Boosters: map[string]*Booster{
			"u1": {BoostState: BoostStateTokenTime, StartTime: oldStart},
			"u2": {BoostState: BoostStateUnboosted, StartTime: oldStart},
		},
	}

	applyBoostOrderSelection(contract, []string{"u2", "u1"}, true)

	if contract.CurrentBoosterUserID != "u2" {
		t.Fatalf("expected current booster to be u2, got %q", contract.CurrentBoosterUserID)
	}
	if !contract.Boosters["u2"].StartTime.After(oldStart) {
		t.Fatalf("expected u2 StartTime to be updated when selected as new current booster")
	}
}

func TestApplyBoostOrderSelectionDoesNotUpdateStartTimeWhenCurrentUnchanged(t *testing.T) {
	oldStart := time.Now().Add(-10 * time.Minute)
	contract := &Contract{
		State:                ContractStateFastrun,
		Order:                []string{"u1", "u2"},
		CurrentBoosterUserID: "u1",
		BoostPosition:        0,
		Boosters: map[string]*Booster{
			"u1": {BoostState: BoostStateTokenTime, StartTime: oldStart},
			"u2": {BoostState: BoostStateUnboosted, StartTime: oldStart},
		},
	}

	applyBoostOrderSelection(contract, []string{"u1", "u2"}, false)

	if contract.CurrentBoosterUserID != "u1" {
		t.Fatalf("expected current booster to remain u1, got %q", contract.CurrentBoosterUserID)
	}
	if !contract.Boosters["u1"].StartTime.Equal(oldStart) {
		t.Fatalf("expected u1 StartTime to remain unchanged when current booster does not change")
	}
}
