package boost

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

func TestParseCsEstimateParams_StickySrMode(t *testing.T) {
	testUserID := "999000111222333444"

	makePayload := func(optionsJSON string) string {
		return fmt.Sprintf(`{
			"id": "123456789012345678",
			"application_id": "123456789012345678",
			"type": 2,
			"token": "test-token",
			"version": 1,
			"channel": {"id": "123456789012345678", "type": 0},
			"guild_id": "123456789012345678",
			"member": {
				"user": {"id": "%s", "username": "testuser", "discriminator": "0"},
				"roles": [],
				"joined_at": "2026-01-01T00:00:00Z"
			},
			"data": {
				"id": "1",
				"name": "cs-estimate",
				"type": 1,
				"options": %s
			}
		}`, testUserID, optionsJSON)
	}

	// 1. Initial state: neither flag set in farmerstate, neither passed in options
	defaultEvent, err := dc.NewCommandEventFromPayload([]byte(makePayload(`[]`)))
	if err != nil {
		t.Fatalf("failed to create default event: %v", err)
	}
	p := parseCsEstimateParams(defaultEvent)
	if p.srMode {
		t.Errorf("expected srMode default to be false, got true")
	}

	// 2. Pass sr-mode = true
	setTrueEvent, err := dc.NewCommandEventFromPayload([]byte(makePayload(`[{"name":"sr-mode","type":5,"value":true}]`)))
	if err != nil {
		t.Fatalf("failed to create setTrue event: %v", err)
	}
	p = parseCsEstimateParams(setTrueEvent)
	if !p.srMode {
		t.Errorf("expected srMode to be true after setting it true")
	}
	if !farmerstate.GetMiscSettingFlag(testUserID, "sr-mode") {
		t.Errorf("expected farmerstate sr-mode flag to be true")
	}

	// 3. Omit sr-mode option; should remain sticky true
	omitEvent, err := dc.NewCommandEventFromPayload([]byte(makePayload(`[]`)))
	if err != nil {
		t.Fatalf("failed to create omit event: %v", err)
	}
	p = parseCsEstimateParams(omitEvent)
	if !p.srMode {
		t.Errorf("expected srMode to be sticky true when omitted")
	}

	// 4. Pass sr-mode = false
	setFalseEvent, err := dc.NewCommandEventFromPayload([]byte(makePayload(`[{"name":"sr-mode","type":5,"value":false}]`)))
	if err != nil {
		t.Fatalf("failed to create setFalse event: %v", err)
	}
	p = parseCsEstimateParams(setFalseEvent)
	if p.srMode {
		t.Errorf("expected srMode to be false after setting it false")
	}
	if farmerstate.GetMiscSettingFlag(testUserID, "sr-mode") {
		t.Errorf("expected farmerstate sr-mode flag to be false")
	}

	// 5. Omit sr-mode option again; should remain sticky false
	omitEvent2, err := dc.NewCommandEventFromPayload([]byte(makePayload(`[]`)))
	if err != nil {
		t.Fatalf("failed to create omit event 2: %v", err)
	}
	p = parseCsEstimateParams(omitEvent2)
	if p.srMode {
		t.Errorf("expected srMode to be sticky false when omitted")
	}
}

func TestGetCompletionPingMessage(t *testing.T) {
	c := &Contract{
		ContractHash: "test-cs-estimate-hash",
		Name:         "Costume Party",
		CoopID:       "party-coop",
		Location: []*LocationData{
			{
				ChannelID:   "chan-1",
				GuildID:     "guild-1",
				RoleMention: "@PartyRole",
			},
		},
		Boosters: map[string]*Booster{
			"userA": {
				UserID:  "userA",
				Nick:    "PartyLeader",
				Mention: "<@userA>",
			},
		},
	}

	// 1. Future estimated completion
	c.EstimatedEndTime = time.Now().Add(3 * time.Hour)
	futureMsg := getCompletionPingMessage(c, "userA")

	if !strings.HasPrefix(futureMsg, "The contract **Costume Party** `party-coop` will complete on") {
		t.Errorf("expected future message to start with contract completion text, got: %s", futureMsg)
	}
	expectedFooter := "-# @PartyRole Ping requested by <@userA>"
	if !strings.HasSuffix(futureMsg, expectedFooter) {
		t.Errorf("expected message to end with %q, got: %s", expectedFooter, futureMsg)
	}
	if strings.HasPrefix(futureMsg, "@PartyRole") {
		t.Errorf("expected message NOT to start with role mention, got: %s", futureMsg)
	}

	// 2. Already ended contract
	c.EstimatedEndTime = time.Now().Add(-1 * time.Hour)
	endedMsg := getCompletionPingMessage(c, "userA")

	if !strings.HasPrefix(endedMsg, "The contract **Costume Party** `party-coop` has ended! Please check-in to finalize scores.") {
		t.Errorf("expected ended message to start with contract ended text, got: %s", endedMsg)
	}
	if !strings.HasSuffix(endedMsg, expectedFooter) {
		t.Errorf("expected ended message to end with %q, got: %s", expectedFooter, endedMsg)
	}
	if strings.HasPrefix(endedMsg, "@PartyRole") {
		t.Errorf("expected ended message NOT to start with role mention, got: %s", endedMsg)
	}
}
