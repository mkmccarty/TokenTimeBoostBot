package watch

import (
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

func TestParseEventWatchTarget(t *testing.T) {
	tests := []struct {
		targetID  string
		wantType  string
		wantUltra bool
		wantRep   bool
	}{
		{"boost-duration:false:false", "boost-duration", false, false},
		{"earnings-boost:true:true", "earnings-boost", true, true},
		{"prestige-boost:true:true", "prestige-boost", true, true},
		{"vehicle-sale", "vehicle-sale", false, false},
		{"hab-sale:true", "hab-sale", true, false},
	}

	for _, tt := range tests {
		gotType, gotUltra, gotRep := parseEventWatchTarget(tt.targetID)
		if gotType != tt.wantType || gotUltra != tt.wantUltra || gotRep != tt.wantRep {
			t.Errorf("parseEventWatchTarget(%q) = (%q, %t, %t), want (%q, %t, %t)",
				tt.targetID, gotType, gotUltra, gotRep, tt.wantType, tt.wantUltra, tt.wantRep)
		}
	}
}

func TestMarkEventNotified(t *testing.T) {
	userID := "test-user-mark-notified"
	eventID := "double-prestige-event-id-999"

	// Clear out any old test data from farmerstate
	farmerstate.SetMiscSettingString(userID, "notified_events", "")

	// 1. Should notify first time
	if !markEventNotified(userID, eventID) {
		t.Errorf("Expected markEventNotified to return true for first notification of %s", eventID)
	}

	// 2. Should NOT notify second time
	if markEventNotified(userID, eventID) {
		t.Errorf("Expected markEventNotified to return false for second notification of %s", eventID)
	}

	// 3. Different event ID should notify
	if !markEventNotified(userID, "another-event-id") {
		t.Errorf("Expected markEventNotified to return true for new event ID")
	}
}
