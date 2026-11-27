package boost

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

func newTokenEditSlashEvent(t *testing.T, guildID, channelID, userID string) *dc.CommandEvent {
	t.Helper()
	payload := fmt.Sprintf(`{
		"id": "100",
		"application_id": "200",
		"type": 2,
		"token": "tok",
		"version": 1,
		"guild_id": %q,
		"channel": {"id": %q, "type": 0},
		"member": {"user": {"id": %q, "username": "caller", "discriminator": "0"}},
		"user": {"id": %q, "username": "caller", "discriminator": "0"},
		"data": {"id": "1", "name": "token-edit", "type": 1, "options": []}
	}`, guildID, channelID, userID, userID)

	e, err := dc.NewCommandEventFromPayload([]byte(payload))
	if err != nil {
		t.Fatalf("NewCommandEventFromPayload failed: %v", err)
	}
	return e
}

func newTokenEditComponentEvent(t *testing.T, guildID, channelID, userID, customID string, values []string) *dc.ComponentEvent {
	t.Helper()
	componentType := 2 // Button
	valuesJSON := ""
	if values != nil {
		componentType = 3 // String Select
		quoted := make([]string, len(values))
		for i, v := range values {
			quoted[i] = fmt.Sprintf("%q", v)
		}
		valuesJSON = fmt.Sprintf(`, "values": [%s]`, strings.Join(quoted, ", "))
	}

	payload := fmt.Sprintf(`{
		"id": "101",
		"application_id": "200",
		"type": 3,
		"token": "tok",
		"version": 1,
		"guild_id": %q,
		"channel": {"id": %q, "type": 0},
		"message": {"id": "300", "flags": 64},
		"member": {"user": {"id": %q, "username": "caller", "discriminator": "0"}},
		"user": {"id": %q, "username": "caller", "discriminator": "0"},
		"data": {"component_type": %d, "custom_id": %q%s}
	}`, guildID, channelID, userID, userID, componentType, customID, valuesJSON)

	e, err := dc.NewComponentEventFromPayload([]byte(payload))
	if err != nil {
		t.Fatalf("NewComponentEventFromPayload failed: %v", err)
	}
	return e
}

func findTokenEditSessionForUser(userID, contractHash string) *tokenEditSession {
	tokenEditSessionsMutex.Lock()
	defer tokenEditSessionsMutex.Unlock()
	for _, s := range tokenEditSessions {
		if s.userID == userID && s.contractHash == contractHash {
			return s
		}
	}
	return nil
}

func TestGetSlashTokenEditCommand_NoOptions(t *testing.T) {
	cmd := GetSlashTokenEditCommand("token-edit")
	if cmd == nil {
		t.Fatal("expected command definition, got nil")
	}
	if cmd.Name != "token-edit" {
		t.Errorf("got command name %q, want %q", cmd.Name, "token-edit")
	}
	if len(cmd.Options) != 0 {
		t.Errorf("expected 0 options for interactive dialog command, got %d", len(cmd.Options))
	}
}

func TestTokenEditDialog_MoveQuantityDeleteUndoAndSave(t *testing.T) {
	guildID := "9001"
	channelID := "8010"
	client := dctest.New().WithGuild(guildID, "Test Guild")
	client.WithChannel(channelID, guildID, "contract-channel")

	now := time.Now()
	serial1 := "00000000-0000-0000-0000-000000000001"
	serial2 := "00000000-0000-0000-0000-000000000002"
	serialConcurrent := "00000000-0000-0000-0000-000000000099"

	contract := &Contract{
		ContractHash:      "token-edit-dialog-hash-1",
		ContractID:        "test-contract",
		CoopID:            "test-coop",
		CreatorID:         []string{"1001"},
		Location:          []*LocationData{{GuildID: guildID, ChannelID: channelID}},
		Order:             []string{"1001", "1002", "1003"},
		StartTime:         now.Add(-20 * time.Minute),
		EstimatedDuration: time.Hour,
		Boosters: map[string]*Booster{
			"1001": {UserID: "1001", Nick: "Alice"},
			"1002": {UserID: "1002", Nick: "Bob"},
			"1003": {UserID: "1003", Nick: "Carol"},
		},
		TokenLog: []ei.TokenUnitLog{
			{
				Serial:     serial1,
				FromUserID: "1002",
				FromNick:   "Bob",
				ToUserID:   "1001",
				ToNick:     "Alice",
				Quantity:   1,
				Time:       now.Add(-5 * time.Minute),
			},
			{
				Serial:     serial2,
				FromUserID: "1002",
				FromNick:   "Bob",
				ToUserID:   "1003",
				ToNick:     "Carol",
				Quantity:   2,
				Time:       now.Add(-2 * time.Minute),
			},
		},
	}

	ContractsMutex.Lock()
	Contracts[contract.ContractHash] = contract
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract.ContractHash)
		ContractsMutex.Unlock()
		clearTokenEditSessionsForUserContract("1002", contract.ContractHash)
	}()

	// Non-coordinator participant Bob starts /token-edit.
	HandleTokenEditInteraction(client, newTokenEditSlashEvent(t, guildID, channelID, "1002"))
	session := findTokenEditSessionForUser("1002", contract.ContractHash)
	if session == nil {
		t.Fatal("expected token edit session to be created for participant")
	}
	if session.isCoordinator || session.showAll {
		t.Fatalf("expected non-coordinator session with showAll=false, got isCoord=%v showAll=%v", session.isCoordinator, session.showAll)
	}

	// Select serial1.
	HandleTokenEditReactions(client, newTokenEditComponentEvent(t, guildID, channelID, "1002",
		fmt.Sprintf("%s#%s#select", tokenEditHandlerPrefix, session.uuidStr), []string{serial1}))
	if session.selectedKey != serial1 {
		t.Fatalf("expected selectedKey=%q, got %q", serial1, session.selectedKey)
	}

	// Attempt move to nonexistent receiver -> should not panic or modify.
	HandleTokenEditReactions(client, newTokenEditComponentEvent(t, guildID, channelID, "1002",
		fmt.Sprintf("%s#%s#move", tokenEditHandlerPrefix, session.uuidStr), []string{"9999"}))
	if session.entriesByKey[serial1].token.ToUserID != "1001" {
		t.Fatalf("expected receiver unchanged after invalid move, got %q", session.entriesByKey[serial1].token.ToUserID)
	}

	// Move serial1 to Carol ("1003").
	HandleTokenEditReactions(client, newTokenEditComponentEvent(t, guildID, channelID, "1002",
		fmt.Sprintf("%s#%s#move", tokenEditHandlerPrefix, session.uuidStr), []string{"1003"}))
	if session.entriesByKey[serial1].token.ToUserID != "1003" || session.entriesByKey[serial1].token.ToNick != "Carol" {
		t.Fatalf("expected staged move to Carol, got %q (%q)", session.entriesByKey[serial1].token.ToNick, session.entriesByKey[serial1].token.ToUserID)
	}

	// Adjust quantity +1 -> 2, +1 -> 3, then -1 -> 2, then undo -> 3.
	HandleTokenEditReactions(client, newTokenEditComponentEvent(t, guildID, channelID, "1002",
		fmt.Sprintf("%s#%s#qty#1", tokenEditHandlerPrefix, session.uuidStr), nil))
	HandleTokenEditReactions(client, newTokenEditComponentEvent(t, guildID, channelID, "1002",
		fmt.Sprintf("%s#%s#qty#1", tokenEditHandlerPrefix, session.uuidStr), nil))
	if session.entriesByKey[serial1].token.Quantity != 3 {
		t.Fatalf("expected quantity 3, got %d", session.entriesByKey[serial1].token.Quantity)
	}
	HandleTokenEditReactions(client, newTokenEditComponentEvent(t, guildID, channelID, "1002",
		fmt.Sprintf("%s#%s#qty#-1", tokenEditHandlerPrefix, session.uuidStr), nil))
	if session.entriesByKey[serial1].token.Quantity != 2 {
		t.Fatalf("expected quantity 2, got %d", session.entriesByKey[serial1].token.Quantity)
	}
	HandleTokenEditReactions(client, newTokenEditComponentEvent(t, guildID, channelID, "1002",
		fmt.Sprintf("%s#%s#undo", tokenEditHandlerPrefix, session.uuidStr), nil))
	if session.entriesByKey[serial1].token.Quantity != 3 {
		t.Fatalf("expected quantity restored to 3 after undo, got %d", session.entriesByKey[serial1].token.Quantity)
	}

	// Go back, select serial2, and mark it for deletion.
	HandleTokenEditReactions(client, newTokenEditComponentEvent(t, guildID, channelID, "1002",
		fmt.Sprintf("%s#%s#back", tokenEditHandlerPrefix, session.uuidStr), nil))
	HandleTokenEditReactions(client, newTokenEditComponentEvent(t, guildID, channelID, "1002",
		fmt.Sprintf("%s#%s#select", tokenEditHandlerPrefix, session.uuidStr), []string{serial2}))
	HandleTokenEditReactions(client, newTokenEditComponentEvent(t, guildID, channelID, "1002",
		fmt.Sprintf("%s#%s#delete", tokenEditHandlerPrefix, session.uuidStr), nil))
	if !session.entriesByKey[serial2].deleted {
		t.Fatal("expected serial2 to be marked deleted")
	}

	// Simulate a new token arriving concurrently while the dialog is open.
	contract.mutex.Lock()
	contract.TokenLog = append(contract.TokenLog, ei.TokenUnitLog{
		Serial:     serialConcurrent,
		FromUserID: "1001",
		FromNick:   "Alice",
		ToUserID:   "1002",
		ToNick:     "Bob",
		Quantity:   1,
		Time:       now,
	})
	contract.mutex.Unlock()

	// Save & Close.
	HandleTokenEditReactions(client, newTokenEditComponentEvent(t, guildID, channelID, "1002",
		fmt.Sprintf("%s#%s#save", tokenEditHandlerPrefix, session.uuidStr), nil))

	if findTokenEditSessionForUser("1002", contract.ContractHash) != nil {
		t.Fatal("expected session to be cleaned up after save")
	}

	if len(contract.TokenLog) != 2 {
		t.Fatalf("expected 2 tokens after save (serial1 updated, serial2 deleted, concurrent preserved), got %d", len(contract.TokenLog))
	}
	if contract.TokenLog[0].Serial != serial1 || contract.TokenLog[0].ToUserID != "1003" || contract.TokenLog[0].Quantity != 3 {
		t.Errorf("unexpected updated serial1: %+v", contract.TokenLog[0])
	}
	if contract.TokenLog[1].Serial != serialConcurrent {
		t.Errorf("expected concurrent token %q to be preserved, got %q", serialConcurrent, contract.TokenLog[1].Serial)
	}
}

func TestTokenEditDialog_CoordinatorCanViewAndEditAllTokens(t *testing.T) {
	guildID := "9001"
	channelID := "8011"
	client := dctest.New().WithGuild(guildID, "Test Guild")
	client.WithChannel(channelID, guildID, "contract-channel")

	now := time.Now()
	serialOther := "00000000-0000-0000-0000-000000000010"

	contract := &Contract{
		ContractHash:      "token-edit-dialog-hash-coord",
		ContractID:        "test-contract",
		CoopID:            "test-coop",
		CreatorID:         []string{"1001"},
		Location:          []*LocationData{{GuildID: guildID, ChannelID: channelID}},
		Order:             []string{"1001", "1002", "1003"},
		StartTime:         now.Add(-15 * time.Minute),
		EstimatedDuration: time.Hour,
		Boosters: map[string]*Booster{
			"1001": {UserID: "1001", Nick: "Coord"},
			"1002": {UserID: "1002", Nick: "Bob"},
			"1003": {UserID: "1003", Nick: "Carol"},
		},
		TokenLog: []ei.TokenUnitLog{
			{
				Serial:     serialOther,
				FromUserID: "1002",
				FromNick:   "Bob",
				ToUserID:   "1003",
				ToNick:     "Carol",
				Quantity:   1,
				Time:       now.Add(-3 * time.Minute),
			},
		},
	}

	ContractsMutex.Lock()
	Contracts[contract.ContractHash] = contract
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract.ContractHash)
		ContractsMutex.Unlock()
		clearTokenEditSessionsForUserContract("1001", contract.ContractHash)
	}()

	// Coordinator has sent 0 tokens, so session should start with showAll=true.
	HandleTokenEditInteraction(client, newTokenEditSlashEvent(t, guildID, channelID, "1001"))
	session := findTokenEditSessionForUser("1001", contract.ContractHash)
	if session == nil {
		t.Fatal("expected coordinator session to be created")
	}
	if !session.isCoordinator || !session.showAll {
		t.Fatalf("expected coordinator session with showAll=true, got isCoord=%v showAll=%v", session.isCoordinator, session.showAll)
	}

	// Toggle scope to My Tokens, then back to All Tokens.
	HandleTokenEditReactions(client, newTokenEditComponentEvent(t, guildID, channelID, "1001",
		fmt.Sprintf("%s#%s#scope", tokenEditHandlerPrefix, session.uuidStr), nil))
	if session.showAll {
		t.Fatal("expected showAll=false after scope toggle")
	}
	HandleTokenEditReactions(client, newTokenEditComponentEvent(t, guildID, channelID, "1001",
		fmt.Sprintf("%s#%s#scope", tokenEditHandlerPrefix, session.uuidStr), nil))
	if !session.showAll {
		t.Fatal("expected showAll=true after second scope toggle")
	}

	// Coordinator selects Bob's token and moves it to Coord ("1001"), then saves.
	HandleTokenEditReactions(client, newTokenEditComponentEvent(t, guildID, channelID, "1001",
		fmt.Sprintf("%s#%s#select", tokenEditHandlerPrefix, session.uuidStr), []string{serialOther}))
	HandleTokenEditReactions(client, newTokenEditComponentEvent(t, guildID, channelID, "1001",
		fmt.Sprintf("%s#%s#move", tokenEditHandlerPrefix, session.uuidStr), []string{"1001"}))
	HandleTokenEditReactions(client, newTokenEditComponentEvent(t, guildID, channelID, "1001",
		fmt.Sprintf("%s#%s#save", tokenEditHandlerPrefix, session.uuidStr), nil))

	if contract.TokenLog[0].ToUserID != "1001" || contract.TokenLog[0].ToNick != "Coord" {
		t.Fatalf("expected coordinator edit to move Bob's token to Coord, got %+v", contract.TokenLog[0])
	}
}

func TestTokenEditDialog_NonCallerRejectedAndCancelDiscards(t *testing.T) {
	guildID := "9001"
	channelID := "8012"
	client := dctest.New().WithGuild(guildID, "Test Guild")
	client.WithChannel(channelID, guildID, "contract-channel")

	now := time.Now()
	serial := "00000000-0000-0000-0000-000000000020"

	contract := &Contract{
		ContractHash:      "token-edit-dialog-hash-cancel",
		ContractID:        "test-contract",
		CoopID:            "test-coop",
		CreatorID:         []string{"1001"},
		Location:          []*LocationData{{GuildID: guildID, ChannelID: channelID}},
		Order:             []string{"1001", "1002"},
		StartTime:         now.Add(-10 * time.Minute),
		EstimatedDuration: time.Hour,
		Boosters: map[string]*Booster{
			"1001": {UserID: "1001", Nick: "Alice"},
			"1002": {UserID: "1002", Nick: "Bob"},
		},
		TokenLog: []ei.TokenUnitLog{
			{
				Serial:     serial,
				FromUserID: "1001",
				FromNick:   "Alice",
				ToUserID:   "1002",
				ToNick:     "Bob",
				Quantity:   2,
				Time:       now.Add(-1 * time.Minute),
			},
		},
	}

	ContractsMutex.Lock()
	Contracts[contract.ContractHash] = contract
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract.ContractHash)
		ContractsMutex.Unlock()
		clearTokenEditSessionsForUserContract("1001", contract.ContractHash)
	}()

	HandleTokenEditInteraction(client, newTokenEditSlashEvent(t, guildID, channelID, "1001"))
	session := findTokenEditSessionForUser("1001", contract.ContractHash)
	if session == nil {
		t.Fatal("expected session to be created")
	}

	// Different user tries to select a token in Alice's session -> ignored.
	HandleTokenEditReactions(client, newTokenEditComponentEvent(t, guildID, channelID, "1002",
		fmt.Sprintf("%s#%s#select", tokenEditHandlerPrefix, session.uuidStr), []string{serial}))
	if session.selectedKey != "" {
		t.Fatalf("expected non-caller interaction to be ignored, got selectedKey=%q", session.selectedKey)
	}

	// Alice stages a delete then cancels.
	HandleTokenEditReactions(client, newTokenEditComponentEvent(t, guildID, channelID, "1001",
		fmt.Sprintf("%s#%s#select", tokenEditHandlerPrefix, session.uuidStr), []string{serial}))
	HandleTokenEditReactions(client, newTokenEditComponentEvent(t, guildID, channelID, "1001",
		fmt.Sprintf("%s#%s#delete", tokenEditHandlerPrefix, session.uuidStr), nil))
	HandleTokenEditReactions(client, newTokenEditComponentEvent(t, guildID, channelID, "1001",
		fmt.Sprintf("%s#%s#cancel", tokenEditHandlerPrefix, session.uuidStr), nil))

	if findTokenEditSessionForUser("1001", contract.ContractHash) != nil {
		t.Fatal("expected session to be removed after cancel")
	}
	if len(contract.TokenLog) != 1 || contract.TokenLog[0].Quantity != 2 {
		t.Fatalf("expected contract.TokenLog unchanged after cancel, got %+v", contract.TokenLog)
	}
}
