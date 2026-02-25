package boost

import (
	"reflect"
	"testing"
)

func TestMoveOverflowBoostersToWaitlist_OrderPriority(t *testing.T) {
	contract := &Contract{
		CoopSize:         3,
		Order:            []string{"u1", "u2", "u3", "u4", "u5"},
		Boosters:         map[string]*Booster{"u1": {UserID: "u1"}, "u2": {UserID: "u2"}, "u3": {UserID: "u3"}, "u4": {UserID: "u4"}, "u5": {UserID: "u5"}},
		WaitlistBoosters: []string{"w1"},
	}

	moveOverflowBoostersToWaitlist(contract)

	wantOrder := []string{"u1", "u2", "u3"}
	if !reflect.DeepEqual(contract.Order, wantOrder) {
		t.Fatalf("order = %v, want %v", contract.Order, wantOrder)
	}

	if contract.Boosters["u4"] != nil || contract.Boosters["u5"] != nil {
		t.Fatalf("expected overflow boosters removed from contract.Boosters")
	}

	wantWaitlist := []string{"u4", "u5", "w1"}
	if !reflect.DeepEqual(contract.WaitlistBoosters, wantWaitlist) {
		t.Fatalf("waitlist = %v, want %v", contract.WaitlistBoosters, wantWaitlist)
	}

	if contract.RegisteredNum != 3 {
		t.Fatalf("registeredNum = %d, want 3", contract.RegisteredNum)
	}

	if contract.OrderRevision != 1 {
		t.Fatalf("orderRevision = %d, want 1", contract.OrderRevision)
	}
}

func TestMoveOverflowBoostersToWaitlist_CleansAltRelationships(t *testing.T) {
	contract := &Contract{
		CoopSize: 1,
		Order:    []string{"main", "alt"},
		Boosters: map[string]*Booster{
			"main": {UserID: "main", Alts: []string{"alt"}, AltsIcons: []string{"😀"}},
			"alt":  {UserID: "alt", AltController: "main"},
		},
	}

	moveOverflowBoostersToWaitlist(contract)

	if len(contract.Boosters["main"].Alts) != 0 {
		t.Fatalf("main.Alts not cleared: %v", contract.Boosters["main"].Alts)
	}
	if len(contract.Boosters["main"].AltsIcons) != 0 {
		t.Fatalf("main.AltsIcons not cleared: %v", contract.Boosters["main"].AltsIcons)
	}
	if contract.Boosters["alt"] != nil {
		t.Fatalf("alt should be removed from boosters")
	}

	wantWaitlist := []string{"alt"}
	if !reflect.DeepEqual(contract.WaitlistBoosters, wantWaitlist) {
		t.Fatalf("waitlist = %v, want %v", contract.WaitlistBoosters, wantWaitlist)
	}
}
