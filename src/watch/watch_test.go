package watch

import (
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
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

func TestWatchActAs(t *testing.T) {
	mainUser := "100000000000000071"
	altUser := "100000000000000072"
	stranger := "100000000000000079"
	channelID := "3"

	// Clean up any old state
	farmerstate.DeleteUserWatches(mainUser)
	farmerstate.DeleteUserWatches(altUser)
	_ = farmerstate.ClearActAsSwitch(mainUser, channelID)
	_ = farmerstate.RemoveActAsLink(mainUser, altUser)

	// Link accounts and switch to altUser in channelID
	_ = farmerstate.AddActAsLink(mainUser, altUser)
	_ = farmerstate.SetActAsSwitch(mainUser, channelID, altUser, time.Now().Add(2*time.Hour))

	// 1. /watch contract while acting as altUser
	contractEvent := dctest.SubcommandEventWithUser("watch", "contract", mainUser,
		dctest.StringOption{Name: "contract-id", Value: "test-c-999"},
	)
	HandleWatch(contractEvent)

	// Verify watch was added to altUser, not mainUser
	watchesAlt := farmerstate.GetWatchesForUser(altUser)
	if len(watchesAlt) != 1 || watchesAlt[0].TargetID != "test-c-999" {
		t.Fatalf("expected 1 watch for altUser test-c-999, got %+v", watchesAlt)
	}
	watchesMain := farmerstate.GetWatchesForUser(mainUser)
	if len(watchesMain) != 0 {
		t.Errorf("expected 0 watches for mainUser, got %+v", watchesMain)
	}
	if len(contractEvent.Followups) == 0 || !strings.Contains(contractEvent.Followups[len(contractEvent.Followups)-1].Content, altUser) {
		t.Errorf("expected act-as notice for altUser in response, got: %+v", contractEvent.Followups)
	}

	// 2. /watch colleggtible while acting as altUser
	collEvent := dctest.SubcommandEventWithUser("watch", "colleggtible", mainUser,
		dctest.StringOption{Name: "colleggtible-id", Value: "test-coll-888"},
	)
	HandleWatch(collEvent)

	watchesAlt = farmerstate.GetWatchesForUser(altUser)
	if len(watchesAlt) != 2 {
		t.Fatalf("expected 2 watches for altUser, got %d", len(watchesAlt))
	}

	// 3. /watch status while acting as altUser
	statusEvent := dctest.SubcommandEventWithUser("watch", "status", mainUser)
	HandleWatch(statusEvent)

	if len(statusEvent.Followups) == 0 {
		t.Fatalf("expected followups from /watch status")
	}
	lastStatusMsg := statusEvent.Followups[len(statusEvent.Followups)-1].Content
	if !strings.Contains(lastStatusMsg, altUser) {
		t.Errorf("expected status page header to mention altUser, got: %s", lastStatusMsg)
	}

	// 4. Test button interactions: mainUser interacting with altUser's status page
	sortButtonEvent := dctest.ComponentButtonEventWithUser(
		"watch-toggle-sort#"+altUser+"#0",
		mainUser,
	)
	HandleToggleSort(sortButtonEvent)
	if sortButtonEvent.LastResponse != nil && strings.Contains(sortButtonEvent.LastResponse.Content, "only interact") {
		t.Errorf("expected linked mainUser to be authorized to click toggle sort, but was rejected")
	}

	// 5. Test button interaction by an unauthorized stranger
	unauthButtonEvent := dctest.ComponentButtonEventWithUser(
		"watch-toggle-sort#"+altUser+"#0",
		stranger,
	)
	HandleToggleSort(unauthButtonEvent)
	if unauthButtonEvent.LastResponse == nil || !strings.Contains(unauthButtonEvent.LastResponse.Content, "only interact") {
		t.Errorf("expected unauthorized user to be rejected, got: %+v", unauthButtonEvent.LastResponse)
	}

	// Clean up
	farmerstate.DeleteUserWatches(mainUser)
	farmerstate.DeleteUserWatches(altUser)
	_ = farmerstate.ClearActAsSwitch(mainUser, channelID)
	_ = farmerstate.RemoveActAsLink(mainUser, altUser)
}
