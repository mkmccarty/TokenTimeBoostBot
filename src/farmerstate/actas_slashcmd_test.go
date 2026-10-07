package farmerstate

import (
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
)

func TestGetSlashActAsCommand(t *testing.T) {
	cmd := GetSlashActAsCommand("act-as")
	if cmd.Name != "act-as" {
		t.Errorf("cmd.Name = %q; want act-as", cmd.Name)
	}
	if len(cmd.Options) != 4 {
		t.Fatalf("expected 4 subcommands, got %d", len(cmd.Options))
	}

	subs := make(map[string]dc.SubCommand)
	for _, opt := range cmd.Options {
		sub, ok := opt.(dc.SubCommand)
		if !ok {
			t.Fatalf("expected SubCommand, got %T", opt)
		}
		subs[sub.Name] = sub
	}

	// Verify register
	reg, ok := subs["register"]
	if !ok {
		t.Fatalf("missing register subcommand")
	}
	if len(reg.Options) != 1 {
		t.Errorf("expected 1 option for register, got %d", len(reg.Options))
	}
	if userOpt, ok := reg.Options[0].(dc.UserOption); !ok || userOpt.Name != "user" || !userOpt.Required {
		t.Errorf("unexpected register option: %+v", reg.Options[0])
	}

	// Verify switch
	sw, ok := subs["switch"]
	if !ok {
		t.Fatalf("missing switch subcommand")
	}
	if len(sw.Options) != 2 {
		t.Fatalf("expected 2 options for switch, got %d", len(sw.Options))
	}
	if accOpt, ok := sw.Options[0].(dc.StringOption); !ok || accOpt.Name != "account" || !accOpt.Autocomplete || !accOpt.Required {
		t.Errorf("unexpected switch account option: %+v", sw.Options[0])
	}
	if durOpt, ok := sw.Options[1].(dc.StringOption); !ok || durOpt.Name != "duration" || !durOpt.Required || len(durOpt.Choices) == 0 {
		t.Errorf("unexpected switch duration option: %+v", sw.Options[1])
	}

	// Verify revoke
	rev, ok := subs["revoke"]
	if !ok {
		t.Fatalf("missing revoke subcommand")
	}
	if len(rev.Options) != 1 {
		t.Fatalf("expected 1 option for revoke, got %d", len(rev.Options))
	}
	if accOpt, ok := rev.Options[0].(dc.StringOption); !ok || accOpt.Name != "account" || !accOpt.Autocomplete || accOpt.Required {
		t.Errorf("unexpected revoke account option: %+v", rev.Options[0])
	}

	// Verify status
	stat, ok := subs["status"]
	if !ok {
		t.Fatalf("missing status subcommand")
	}
	if len(stat.Options) != 0 {
		t.Errorf("expected 0 options for status, got %d", len(stat.Options))
	}
}

func TestHandleActAsRegisterModalSubmit(t *testing.T) {
	mainUser := "100000000000000001"
	altUser := "100000000000000002"
	code := "87654321"

	StorePendingActAsRegistration(mainUser, altUser, code, 5*time.Minute)

	// Wrong code
	wrongModal := dctest.ModalSubmitEventWithUser("m_actas_reg#"+altUser, mainUser, "code", "00000000")
	HandleActAsRegisterModalSubmit(nil, wrongModal)
	if IsActAsLinked(mainUser, altUser) {
		t.Errorf("expected not linked with wrong code")
	}

	// Correct code with mock client to verify success DM
	fakeClient := &dctest.FakeClient{}
	correctModal := dctest.ModalSubmitEventWithUser("m_actas_reg#"+altUser, mainUser, "code", code)
	HandleActAsRegisterModalSubmit(fakeClient, correctModal)
	if !IsActAsLinked(mainUser, altUser) {
		t.Errorf("expected linked with correct code")
	}

	// Verify linked account received success DM
	if len(fakeClient.SentMessages) == 0 {
		t.Errorf("expected success DM sent to linked account")
	} else {
		lastMsg := fakeClient.SentMessages[len(fakeClient.SentMessages)-1]
		if !strings.Contains(lastMsg.Content, "Account Link Successful") {
			t.Errorf("expected success DM title in message: %s", lastMsg.Content)
		}
		if !strings.Contains(lastMsg.Content, "/act-as revoke") {
			t.Errorf("expected revoke instructions in success DM: %s", lastMsg.Content)
		}
	}

	// Pending should be cleared now
	_, ok := GetPendingActAsRegistration(mainUser)
	if ok {
		t.Errorf("expected pending registration to be cleared")
	}

	// Expired / missing session
	missingModal := dctest.ModalSubmitEventWithUser("m_actas_reg#"+altUser, mainUser, "code", code)
	HandleActAsRegisterModalSubmit(nil, missingModal)
}

func TestHandleActAsSwitchAndRevoke(t *testing.T) {
	mainUser := "100000000000000003"
	altUser := "100000000000000004"

	// Link accounts first
	_ = AddActAsLink(mainUser, altUser)

	// Switch to unlinked account should be rejected
	unlinkedEvent := dctest.SubcommandEventWithUser("act-as", "switch", mainUser,
		dctest.StringOption{Name: "account", Value: "100000000000000099"},
		dctest.StringOption{Name: "duration", Value: "1h"},
	)
	HandleActAsCommand(nil, unlinkedEvent)
	if got := GetEffectiveUserID(mainUser, "3"); got != mainUser {
		t.Errorf("switch to unlinked account should not succeed, got %s", got)
	}

	// Switch to linked account
	validSwitchEvent := dctest.SubcommandEventWithUser("act-as", "switch", mainUser,
		dctest.StringOption{Name: "account", Value: altUser},
		dctest.StringOption{Name: "duration", Value: "2h"},
	)
	HandleActAsCommand(nil, validSwitchEvent)
	if got := GetEffectiveUserID(mainUser, "3"); got != altUser {
		t.Errorf("switch to linked account should succeed, got %s, want %s", got, altUser)
	}

	// Switch with forever duration
	foreverSwitchEvent := dctest.SubcommandEventWithUser("act-as", "switch", mainUser,
		dctest.StringOption{Name: "account", Value: altUser},
		dctest.StringOption{Name: "duration", Value: "forever"},
	)
	HandleActAsCommand(nil, foreverSwitchEvent)
	if got := GetEffectiveUserID(mainUser, "3"); got != altUser {
		t.Errorf("switch to linked account forever should succeed, got %s, want %s", got, altUser)
	}
	_, foreverExp, _ := GetActAsSwitch(mainUser, "3")
	if !IsActAsForever(foreverExp) {
		t.Errorf("expected forever expiry for switch, got %v", foreverExp)
	}

	// Switch back to main using "main"
	resetEvent := dctest.SubcommandEventWithUser("act-as", "switch", mainUser,
		dctest.StringOption{Name: "account", Value: "main"},
		dctest.StringOption{Name: "duration", Value: "0m"},
	)
	HandleActAsCommand(nil, resetEvent)
	if got := GetEffectiveUserID(mainUser, "3"); got != mainUser {
		t.Errorf("switch back to main should succeed, got %s, want %s", got, mainUser)
	}

	// Switch again then revoke
	_ = SetActAsSwitch(mainUser, "3", altUser, time.Now().Add(1*time.Hour))
	revokeEvent := dctest.SubcommandEventWithUser("act-as", "revoke", mainUser,
		dctest.StringOption{Name: "account", Value: altUser},
	)
	HandleActAsCommand(nil, revokeEvent)
	if IsActAsLinked(mainUser, altUser) {
		t.Errorf("link should be revoked")
	}
	if got := GetEffectiveUserID(mainUser, "3"); got != mainUser {
		t.Errorf("revoking should clear active switch, got %s, want %s", got, mainUser)
	}
}

func TestHandleActAsAutocomplete(t *testing.T) {
	mainUser := "100000000000000005"
	alt1 := "100000000000000006"
	alt2 := "100000000000000007"

	_ = AddActAsLink(mainUser, alt1)
	_ = AddActAsLink(mainUser, alt2)
	SetMiscSettingString(alt1, "ei_ign", "AlphaFarmer")
	SetMiscSettingString(alt2, "ei_ign", "BetaFarmer")

	// Autocomplete switch options
	acEvent := dctest.SubcommandAutocompleteEvent("act-as", "switch", "account", "", mainUser)
	HandleActAsAutocomplete(acEvent)
	if len(acEvent.LastChoices) != 2 {
		t.Fatalf("expected 2 choices, got %d", len(acEvent.LastChoices))
	}

	// Filter with query
	acFilter := dctest.SubcommandAutocompleteEvent("act-as", "switch", "account", "beta", mainUser)
	HandleActAsAutocomplete(acFilter)
	if len(acFilter.LastChoices) != 1 || acFilter.LastChoices[0].Value != alt2 {
		t.Errorf("expected 1 choice for beta, got %+v", acFilter.LastChoices)
	}

	// Autocomplete revoke options
	acRevoke := dctest.SubcommandAutocompleteEvent("act-as", "revoke", "account", "", mainUser)
	HandleActAsAutocomplete(acRevoke)
	// Because there are > 1 links, should include "All Linked Accounts" + 2 alts = 3 choices
	if len(acRevoke.LastChoices) != 3 {
		t.Errorf("expected 3 choices for revoke (all + 2 alts), got %d", len(acRevoke.LastChoices))
	}
	if acRevoke.LastChoices[0].Value != "all" {
		t.Errorf("first choice should be all, got %s", acRevoke.LastChoices[0].Value)
	}

	// Clean up
	_ = DeleteAllActAsLinks(mainUser)
}

func TestHandleActAsRegisterSelfRejection(t *testing.T) {
	// Attempting to register self should be rejected
	payload := `{
		"id": "1",
		"application_id": "2",
		"type": 2,
		"token": "tok",
		"version": 1,
		"channel": {"id": "3", "type": 0},
		"user": {"id": "100000000000000008", "username": "tester", "discriminator": "0"},
		"data": {
			"id": "5",
			"name": "act-as",
			"type": 1,
			"options": [{
				"name": "register",
				"type": 1,
				"options": [{"name": "user", "type": 6, "value": "100000000000000008"}]
			}],
			"resolved": {
				"users": {"100000000000000008": {"id": "100000000000000008", "username": "tester", "discriminator": "0"}}
			}
		}
	}`

	event, err := dc.NewCommandEventFromPayload([]byte(payload))
	if err != nil {
		t.Fatalf("building command event: %v", err)
	}

	fakeClient := &dctest.FakeClient{}
	HandleActAsCommand(fakeClient, event)

	// Verify no pending registration was stored
	if _, ok := GetPendingActAsRegistration("100000000000000008"); ok {
		t.Errorf("should not store pending registration for self")
	}
}

func TestHandleActAsRegisterSuccess(t *testing.T) {
	mainUser := "100000000000000009"
	altUser := "100000000000000010"

	payload := `{
		"id": "1",
		"application_id": "2",
		"type": 2,
		"token": "tok",
		"version": 1,
		"channel": {"id": "3", "type": 0},
		"user": {"id": "` + mainUser + `", "username": "mainTester", "discriminator": "0"},
		"data": {
			"id": "5",
			"name": "act-as",
			"type": 1,
			"options": [{
				"name": "register",
				"type": 1,
				"options": [{"name": "user", "type": 6, "value": "` + altUser + `"}]
			}],
			"resolved": {
				"users": {"` + altUser + `": {"id": "` + altUser + `", "username": "altTester", "discriminator": "0"}}
			}
		}
	}`

	event, err := dc.NewCommandEventFromPayload([]byte(payload))
	if err != nil {
		t.Fatalf("building command event: %v", err)
	}

	fakeClient := &dctest.FakeClient{}
	HandleActAsCommand(fakeClient, event)

	// Check that a pending registration was stored with an 8-digit code
	reg, ok := GetPendingActAsRegistration(mainUser)
	if !ok {
		t.Fatalf("expected pending registration to be stored")
	}
	if reg.AltUserID != altUser {
		t.Errorf("got alt user %s, want %s", reg.AltUserID, altUser)
	}
	if len(reg.Code) != 8 {
		t.Errorf("got code %q, want 8 digits", reg.Code)
	}

	// Check that fakeClient was used to create user channel and send DM
	if len(fakeClient.SentMessages) == 0 {
		t.Fatalf("expected a DM message to be sent")
	}
	lastMsg := fakeClient.SentMessages[len(fakeClient.SentMessages)-1]
	if !strings.Contains(lastMsg.Content, reg.Code) {
		t.Errorf("DM message does not contain code: %s", lastMsg.Content)
	}
	if !strings.Contains(lastMsg.Content, "WARNING") {
		t.Errorf("DM message does not contain warning: %s", lastMsg.Content)
	}
	if !strings.Contains(lastMsg.Content, "/act-as revoke") {
		t.Errorf("DM message does not contain revoke instructions: %s", lastMsg.Content)
	}

	ClearPendingActAsRegistration(mainUser)
}

func TestHandleActAsRevokeFromLinkedAccount(t *testing.T) {
	mainUser := "100000000000000011"
	altUser := "100000000000000012"

	// 1. Link accounts
	_ = AddActAsLink(mainUser, altUser)
	if !IsActAsLinked(mainUser, altUser) {
		t.Fatalf("expected accounts to be linked")
	}

	// 2. Main switches to alt in channel 3
	_ = SetActAsSwitch(mainUser, "3", altUser, time.Now().Add(2*time.Hour))
	if got := GetEffectiveUserID(mainUser, "3"); got != altUser {
		t.Fatalf("expected mainUser to be acting as altUser, got %s", got)
	}

	// 3. Autocomplete on revoke from altUser perspective should return mainUser
	acRevoke := dctest.SubcommandAutocompleteEvent("act-as", "revoke", "account", "", altUser)
	HandleActAsAutocomplete(acRevoke)
	if len(acRevoke.LastChoices) != 1 {
		t.Fatalf("expected 1 choice for revoke from alt, got %d", len(acRevoke.LastChoices))
	}
	if acRevoke.LastChoices[0].Value != mainUser {
		t.Errorf("expected choice value to be mainUser (%s), got %s", mainUser, acRevoke.LastChoices[0].Value)
	}

	// 4. AltUser executes /act-as revoke without specifying account option
	revokeEvent := dctest.SubcommandEventWithUser("act-as", "revoke", altUser)
	HandleActAsCommand(nil, revokeEvent)

	// Verify link is removed in both directions
	if IsActAsLinked(mainUser, altUser) {
		t.Errorf("link should be revoked")
	}
	if IsActAsLinkedEither(mainUser, altUser) {
		t.Errorf("link should be revoked in either direction")
	}

	// Verify mainUser's active switch was cleared
	if got := GetEffectiveUserID(mainUser, "3"); got != mainUser {
		t.Errorf("revoking from altUser should clear mainUser active switch, got %s, want %s", got, mainUser)
	}

	// 5. Test revoking with explicit account argument from altUser
	_ = AddActAsLink(mainUser, altUser)
	_ = SetActAsSwitch(mainUser, "3", altUser, time.Now().Add(2*time.Hour))
	if got := GetEffectiveUserID(mainUser, "3"); got != altUser {
		t.Fatalf("expected mainUser to be acting as altUser, got %s", got)
	}

	revokeExplicit := dctest.SubcommandEventWithUser("act-as", "revoke", altUser,
		dctest.StringOption{Name: "account", Value: mainUser},
	)
	HandleActAsCommand(nil, revokeExplicit)

	if IsActAsLinked(mainUser, altUser) {
		t.Errorf("link should be revoked with explicit account")
	}
	if got := GetEffectiveUserID(mainUser, "3"); got != mainUser {
		t.Errorf("revoking explicit should clear switch, got %s, want %s", got, mainUser)
	}
}

func TestHandleActAsStatus(t *testing.T) {
	user1 := "100000000000000025"
	user2 := "100000000000000026"
	channelID := "3"

	// 1. Unswitched state (acting as oneself)
	_ = ClearActAsSwitch(user1, channelID)
	statusEvent := dctest.SubcommandEventWithUser("act-as", "status", user1)
	HandleActAsCommand(nil, statusEvent)

	if statusEvent.LastResponse == nil {
		t.Fatalf("expected response from /act-as status")
	}
	if !statusEvent.LastResponse.Ephemeral {
		t.Errorf("expected ephemeral response")
	}
	if !strings.Contains(statusEvent.LastResponse.Content, "yourself") || !strings.Contains(statusEvent.LastResponse.Content, user1) {
		t.Errorf("expected yourself status message, got: %s", statusEvent.LastResponse.Content)
	}

	// Link accounts for switch tests
	_ = AddActAsLink(user1, user2)

	// 2. Active timed switch
	exp := time.Now().Add(2 * time.Hour)
	_ = SetActAsSwitch(user1, channelID, user2, exp)

	statusEvent2 := dctest.SubcommandEventWithUser("act-as", "status", user1)
	HandleActAsCommand(nil, statusEvent2)

	if statusEvent2.LastResponse == nil {
		t.Fatalf("expected response from /act-as status")
	}
	if !statusEvent2.LastResponse.Ephemeral {
		t.Errorf("expected ephemeral response")
	}
	if !strings.Contains(statusEvent2.LastResponse.Content, user2) || !strings.Contains(statusEvent2.LastResponse.Content, "expire") {
		t.Errorf("expected timed switch status message with alt user, got: %s", statusEvent2.LastResponse.Content)
	}

	// 3. Active forever switch
	_ = SetActAsSwitch(user1, channelID, user2, ActAsForeverExpiry)

	statusEvent3 := dctest.SubcommandEventWithUser("act-as", "status", user1)
	HandleActAsCommand(nil, statusEvent3)

	if statusEvent3.LastResponse == nil {
		t.Fatalf("expected response from /act-as status")
	}
	if !statusEvent3.LastResponse.Ephemeral {
		t.Errorf("expected ephemeral response")
	}
	if !strings.Contains(statusEvent3.LastResponse.Content, user2) || !strings.Contains(statusEvent3.LastResponse.Content, "forever") {
		t.Errorf("expected forever switch status message with alt user, got: %s", statusEvent3.LastResponse.Content)
	}

	// Clean up
	_ = ClearActAsSwitch(user1, channelID)
	_ = RemoveActAsLink(user1, user2)
}
