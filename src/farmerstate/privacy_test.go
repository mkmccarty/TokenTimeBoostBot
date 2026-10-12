package farmerstate

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
)

func TestGetSlashPrivacyCommand(t *testing.T) {
	cmd := GetSlashPrivacyCommand("privacy")
	if cmd.Name != "privacy" {
		t.Errorf("cmd.Name = %q; want privacy", cmd.Name)
	}

	var foundAlt bool
	for _, opt := range cmd.Options {
		if sOpt, ok := opt.(dc.StringOption); ok && sOpt.Name == "alt" {
			foundAlt = true
			if sOpt.Required {
				t.Errorf("alt option should be optional (Required=false)")
			}
			if !sOpt.Autocomplete {
				t.Errorf("alt option should have Autocomplete=true")
			}
		}
	}
	if !foundAlt {
		t.Fatalf("alt option not found in /privacy command options")
	}
}

func TestHandlePrivacyAutocomplete(t *testing.T) {
	mainUser := "100000000000000001"
	alt1 := "100000000000000002"
	alt2 := "100000000000000003"

	SetMiscSettingString(alt1, "AltController", mainUser)
	SetMiscSettingString(alt1, "ei_ign", "AlphaFarmer")

	_ = AddActAsLink(mainUser, alt2)
	SetMiscSettingString(alt2, "ei_ign", "BetaFarmer")

	// Autocomplete for "alt" option with empty search string
	acEmpty := dctest.AutocompleteEventWithUser("privacy", "alt", "", mainUser)
	HandlePrivacyAutocomplete(acEmpty)
	if len(acEmpty.LastChoices) < 2 {
		t.Fatalf("expected at least 2 choices, got %d", len(acEmpty.LastChoices))
	}

	// Autocomplete with filter "alpha"
	acFilter := dctest.AutocompleteEventWithUser("privacy", "alt", "alpha", mainUser)
	HandlePrivacyAutocomplete(acFilter)
	if len(acFilter.LastChoices) != 1 || acFilter.LastChoices[0].Value != alt1 {
		t.Errorf("expected 1 choice for 'alpha' matching %s, got %+v", alt1, acFilter.LastChoices)
	}

	// Autocomplete with non-matching option name
	acOther := dctest.AutocompleteEventWithUser("privacy", "other", "", mainUser)
	HandlePrivacyAutocomplete(acOther)
	if len(acOther.LastChoices) != 0 {
		t.Errorf("expected 0 choices for non-alt option, got %d", len(acOther.LastChoices))
	}

	// Autocomplete for user with no alts
	acNoAlts := dctest.AutocompleteEventWithUser("privacy", "alt", "", "100000000000000099")
	HandlePrivacyAutocomplete(acNoAlts)
	if len(acNoAlts.LastChoices) != 0 {
		t.Errorf("expected 0 choices for user with no alts, got %d", len(acNoAlts.LastChoices))
	}

	// Clean up
	PurgeAltLinks(mainUser, alt1)
	PurgeAltLinks(mainUser, alt2)
}

func TestResolveLinkedAlt(t *testing.T) {
	mainUser := "100000000000000010"
	alt1 := "100000000000000011"
	alt2 := "test-named-alt"

	SetMiscSettingString(alt1, "AltController", mainUser)
	SetMiscSettingString(alt1, "ei_ign", "TargetIGN")

	_ = AddActAsLink(mainUser, alt2)

	// Direct ID match
	resolved, ok := resolveLinkedAlt(mainUser, alt1)
	if !ok || resolved != alt1 {
		t.Errorf("resolveLinkedAlt by ID failed: got %q, %v", resolved, ok)
	}

	// Discord mention match
	resolved, ok = resolveLinkedAlt(mainUser, "<@"+alt1+">")
	if !ok || resolved != alt1 {
		t.Errorf("resolveLinkedAlt by mention failed: got %q, %v", resolved, ok)
	}

	// IGN match
	resolved, ok = resolveLinkedAlt(mainUser, "TargetIGN")
	if !ok || resolved != alt1 {
		t.Errorf("resolveLinkedAlt by IGN failed: got %q, %v", resolved, ok)
	}

	// Named alt match
	resolved, ok = resolveLinkedAlt(mainUser, alt2)
	if !ok || resolved != alt2 {
		t.Errorf("resolveLinkedAlt for named alt failed: got %q, %v", resolved, ok)
	}

	// Unlinked account
	_, ok = resolveLinkedAlt(mainUser, "100000000000000099")
	if ok {
		t.Errorf("resolveLinkedAlt for unlinked user should fail")
	}

	// Clean up
	PurgeAltLinks(mainUser, alt1)
	PurgeAltLinks(mainUser, alt2)
}

func commandEventFromPayload(payload string) *dc.CommandEvent {
	event, err := dc.NewCommandEventFromPayload([]byte(payload))
	if err != nil {
		panic(fmt.Sprintf("building command event: %v", err))
	}
	return event
}

func TestHandlePrivacy_UnlinkedAlt(t *testing.T) {
	mainUser := "100000000000000020"

	payload := fmt.Sprintf(`{
		"id": "1", "application_id": "2", "type": 2, "token": "test-token", "version": 1,
		"channel": {"id": "3", "type": 0},
		"user": {"id": %q, "username": "tester", "discriminator": "0"},
		"data": {"id": "5", "name": "privacy", "type": 1, "options": [
			{"name": "alt", "type": 3, "value": "100000000000000099"}
		]}
	}`, mainUser)

	event := commandEventFromPayload(payload)
	HandlePrivacy(event)

	if event.LastFollowup == nil {
		t.Fatalf("expected a followup message")
	}
	if !strings.Contains(event.LastFollowup.Content, "not linked to you as an alternate account") {
		t.Errorf("expected unlinked account warning, got: %s", event.LastFollowup.Content)
	}
}

func TestHandlePrivacy_AltView(t *testing.T) {
	mainUser := "100000000000000030"
	altUser := "100000000000000031"

	SetMiscSettingString(altUser, "AltController", mainUser)

	payload := fmt.Sprintf(`{
		"id": "1", "application_id": "2", "type": 2, "token": "test-token", "version": 1,
		"channel": {"id": "3", "type": 0},
		"user": {"id": %q, "username": "tester", "discriminator": "0"},
		"data": {"id": "5", "name": "privacy", "type": 1, "options": [
			{"name": "alt", "type": 3, "value": %q}
		]}
	}`, mainUser, altUser)

	event := commandEventFromPayload(payload)
	HandlePrivacy(event)

	if event.LastFollowup == nil {
		t.Fatalf("expected a followup message")
	}
	if !strings.Contains(event.LastFollowup.Content, "Privacy information for alternate account") {
		t.Errorf("expected alternate account header, got: %s", event.LastFollowup.Content)
	}

	PurgeAltLinks(mainUser, altUser)
}

func TestHandlePrivacy_AltGetSettingsData(t *testing.T) {
	mainUser := "100000000000000040"
	altUser := "100000000000000041"

	SetMiscSettingString(altUser, "AltController", mainUser)
	SetEggIncName(altUser, "AltFarmerData")

	payload := fmt.Sprintf(`{
		"id": "1", "application_id": "2", "type": 2, "token": "test-token", "version": 1,
		"channel": {"id": "3", "type": 0},
		"user": {"id": %q, "username": "tester", "discriminator": "0"},
		"data": {"id": "5", "name": "privacy", "type": 1, "options": [
			{"name": "alt", "type": 3, "value": %q},
			{"name": "get-settings-data", "type": 5, "value": true}
		]}
	}`, mainUser, altUser)

	event := commandEventFromPayload(payload)
	HandlePrivacy(event)

	if event.LastFollowup == nil {
		t.Fatalf("expected a followup message")
	}
	if len(event.LastFollowup.Files) != 1 {
		t.Fatalf("expected 1 file attached, got %d", len(event.LastFollowup.Files))
	}
	expectedFilename := "boostbot-data-" + altUser + ".json"
	if event.LastFollowup.Files[0].Name != expectedFilename {
		t.Errorf("filename = %q, want %q", event.LastFollowup.Files[0].Name, expectedFilename)
	}

	PurgeAltLinks(mainUser, altUser)
}

func TestHandlePrivacy_AltRemoveData_Unconfirmed(t *testing.T) {
	mainUser := "100000000000000050"
	altUser := "100000000000000051"

	SetMiscSettingString(altUser, "AltController", mainUser)

	payload := fmt.Sprintf(`{
		"id": "1", "application_id": "2", "type": 2, "token": "test-token", "version": 1,
		"channel": {"id": "3", "type": 0},
		"user": {"id": %q, "username": "tester", "discriminator": "0"},
		"data": {"id": "5", "name": "privacy", "type": 1, "options": [
			{"name": "alt", "type": 3, "value": %q},
			{"name": "remove-data", "type": 5, "value": true}
		]}
	}`, mainUser, altUser)

	event := commandEventFromPayload(payload)
	HandlePrivacy(event)

	if event.LastFollowup == nil {
		t.Fatalf("expected a followup message")
	}
	if !strings.Contains(event.LastFollowup.Content, "You have not confirmed your data removal") {
		t.Errorf("expected confirmation warning, got: %s", event.LastFollowup.Content)
	}

	// Alt should still be linked
	if GetMiscSettingString(altUser, "AltController") != mainUser {
		t.Errorf("AltController should still be set")
	}

	PurgeAltLinks(mainUser, altUser)
}

func TestHandlePrivacy_AltRemoveData_Confirmed(t *testing.T) {
	mainUser := "100000000000000060"
	altUser := "100000000000000061"

	// Set up alt data and links in both AltController and act_as_links
	SetMiscSettingString(altUser, "AltController", mainUser)
	SetEggIncName(altUser, "PurgeAltFarmer")
	_ = AddActAsLink(mainUser, altUser)
	_ = SetActAsSwitch(mainUser, "test-channel", altUser, time.Now().Add(1*time.Hour))

	if !IsActAsLinked(mainUser, altUser) {
		t.Fatalf("setup failed: altUser should be linked in act_as_links")
	}
	if GetMiscSettingString(altUser, "AltController") != mainUser {
		t.Fatalf("setup failed: altUser should have AltController set")
	}

	payload := fmt.Sprintf(`{
		"id": "1", "application_id": "2", "type": 2, "token": "test-token", "version": 1,
		"channel": {"id": "3", "type": 0},
		"user": {"id": %q, "username": "tester", "discriminator": "0"},
		"data": {"id": "5", "name": "privacy", "type": 1, "options": [
			{"name": "alt", "type": 3, "value": %q},
			{"name": "remove-data", "type": 5, "value": true},
			{"name": "confirm-request", "type": 5, "value": true}
		]}
	}`, mainUser, altUser)

	event := commandEventFromPayload(payload)
	HandlePrivacy(event)

	if event.LastFollowup == nil {
		t.Fatalf("expected a followup message")
	}
	if !strings.Contains(event.LastFollowup.Content, "Alternate account settings data and alt links have been removed") {
		t.Errorf("expected purge confirmation message, got: %s", event.LastFollowup.Content)
	}

	// Verify alt links and switches are purged
	if IsActAsLinked(mainUser, altUser) {
		t.Errorf("IsActAsLinked should be false after purge")
	}
	if GetMiscSettingString(altUser, "AltController") != "" {
		t.Errorf("AltController should be empty after purge, got %q", GetMiscSettingString(altUser, "AltController"))
	}
	if _, _, active := GetActAsSwitch(mainUser, "test-channel"); active {
		t.Errorf("active act-as switch should be removed after purge")
	}

	// Verify alt is no longer in GetLinkedAltIDs for mainUser
	linked := GetLinkedAltIDs(mainUser)
	for _, id := range linked {
		if id == altUser {
			t.Errorf("altUser should no longer be returned by GetLinkedAltIDs")
		}
	}
}

func TestPurgeAltLinks(t *testing.T) {
	mainUser := "100000000000000070"
	altUser := "100000000000000071"

	SetMiscSettingString(altUser, "AltController", mainUser)
	_ = AddActAsLink(mainUser, altUser)
	_ = SetActAsSwitch(mainUser, "chan-1", altUser, time.Now().Add(1*time.Hour))
	StorePendingActAsRegistration(mainUser, altUser, "12345678", 5*time.Minute)

	PurgeAltLinks(mainUser, altUser)

	if IsActAsLinked(mainUser, altUser) {
		t.Errorf("IsActAsLinked should be false")
	}
	if GetMiscSettingString(altUser, "AltController") != "" {
		t.Errorf("AltController should be empty")
	}
	if _, ok := GetPendingActAsRegistration(mainUser); ok {
		t.Errorf("pending registration should be cleared")
	}
}

func TestHandlePrivacy_MainUserPurgesLinkedAlts(t *testing.T) {
	mainUser := "100000000000000080"
	alt1 := "100000000000000081"
	alt2 := "100000000000000082"

	// Set up main and alts with data
	SetEggIncName(mainUser, "MainFarmer")
	SetMiscSettingString(mainUser, "ei_ign", "MainIGN")

	SetEggIncName(alt1, "AltOneFarmer")
	SetMiscSettingString(alt1, "ei_ign", "AltOneIGN")
	SetMiscSettingString(alt1, "AltController", mainUser)

	SetEggIncName(alt2, "AltTwoFarmer")
	SetMiscSettingString(alt2, "ei_ign", "AltTwoIGN")
	_ = AddActAsLink(mainUser, alt2)
	_ = SetActAsSwitch(mainUser, "chan-main", alt2, time.Now().Add(1*time.Hour))

	// Populate leaderboard stats and opt-ins for main and alt
	_ = UpsertLeaderboardStat("cxp", mainUser, "MainIGN", "2026-10-10", 100, sql.NullString{})
	_ = UpsertLeaderboardStat("cxp", alt1, "AltOneIGN", "2026-10-10", 50, sql.NullString{})
	_ = queries.UpsertLeaderboardOptIn(ctx, UpsertLeaderboardOptInParams{
		GuildID: "guild-1",
		UserID:  mainUser,
		LbType:  "cxp",
	})
	_ = queries.UpsertLeaderboardOptIn(ctx, UpsertLeaderboardOptInParams{
		GuildID: "guild-1",
		UserID:  alt1,
		LbType:  "cxp",
	})

	// Ensure alts are linked
	linkedBefore := GetLinkedAltIDs(mainUser)
	if len(linkedBefore) != 2 {
		t.Fatalf("expected 2 linked alts before purge, got %d", len(linkedBefore))
	}

	// Main user runs /privacy with remove-data and confirm-request (no alt param)
	payload := fmt.Sprintf(`{
		"id": "1", "application_id": "2", "type": 2, "token": "test-token", "version": 1,
		"channel": {"id": "3", "type": 0},
		"user": {"id": %q, "username": "tester", "discriminator": "0"},
		"data": {"id": "5", "name": "privacy", "type": 1, "options": [
			{"name": "remove-data", "type": 5, "value": true},
			{"name": "confirm-request", "type": 5, "value": true}
		]}
	}`, mainUser)

	event := commandEventFromPayload(payload)
	HandlePrivacy(event)

	if event.LastFollowup == nil {
		t.Fatalf("expected a followup message")
	}
	if !strings.Contains(event.LastFollowup.Content, "all linked alternate accounts' data have been removed") {
		t.Errorf("expected mention of linked alts removal in: %s", event.LastFollowup.Content)
	}

	// Verify main data is purged
	if GetEggIncName(mainUser) != "" {
		t.Errorf("main EggIncName should be cleared, got %q", GetEggIncName(mainUser))
	}
	if GetMiscSettingString(mainUser, "ei_ign") != "" {
		t.Errorf("main ei_ign should be cleared, got %q", GetMiscSettingString(mainUser, "ei_ign"))
	}

	// Verify alt1 data and links are purged
	if GetEggIncName(alt1) != "" {
		t.Errorf("alt1 EggIncName should be cleared, got %q", GetEggIncName(alt1))
	}
	if GetMiscSettingString(alt1, "AltController") != "" {
		t.Errorf("alt1 AltController should be empty, got %q", GetMiscSettingString(alt1, "AltController"))
	}

	// Verify alt2 data and links are purged
	if GetEggIncName(alt2) != "" {
		t.Errorf("alt2 EggIncName should be cleared, got %q", GetEggIncName(alt2))
	}
	if IsActAsLinked(mainUser, alt2) {
		t.Errorf("act-as link between main and alt2 should be deleted")
	}

	// Verify no alts remain linked to main
	linkedAfter := GetLinkedAltIDs(mainUser)
	if len(linkedAfter) != 0 {
		t.Errorf("expected 0 linked alts after main purge, got %d", len(linkedAfter))
	}

	// Verify leaderboard stats and opt-ins are deleted
	mainStats, _ := queries.GetStatsForPlayer(ctx, mainUser)
	if len(mainStats) != 0 {
		t.Errorf("main leaderboard stats should be deleted, got %d", len(mainStats))
	}
	altStats, _ := queries.GetStatsForPlayer(ctx, alt1)
	if len(altStats) != 0 {
		t.Errorf("alt1 leaderboard stats should be deleted, got %d", len(altStats))
	}
	mainOptIns, _ := queries.GetLeaderboardOptInsForUser(ctx, mainUser)
	if len(mainOptIns) != 0 {
		t.Errorf("main leaderboard optins should be deleted, got %d", len(mainOptIns))
	}
	altOptIns, _ := queries.GetLeaderboardOptInsForUser(ctx, alt1)
	if len(altOptIns) != 0 {
		t.Errorf("alt1 leaderboard optins should be deleted, got %d", len(altOptIns))
	}
}
