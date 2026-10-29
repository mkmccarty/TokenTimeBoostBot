package boost

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
)

func TestRanCoopAndBuildChickenRunLists(t *testing.T) {
	c := &Contract{
		ContractHash: "test-hash",
		Order:        []string{"user1", "user2", "user3", "alt1"},
		Boosters: map[string]*Booster{
			"user1": {
				UserID:  "user1",
				Nick:    "Player1",
				Mention: "<@user1>",
				Alts:    []string{"alt1"},
			},
			"alt1": {
				UserID:        "alt1",
				Nick:          "Player1Alt",
				Mention:       "<@alt1>",
				AltController: "user1",
			},
			"user2": {
				UserID:  "user2",
				Nick:    "Player2",
				Mention: "<@user2>",
			},
			"user3": {
				UserID:  "user3",
				Nick:    "Player3",
				Mention: "<@user3>",
			},
		},
	}

	// Before user1 runs coop, check chicken run list for user2
	ar, missing, _ := buildChickenRunLists(c, "user2")
	if len(ar) != 0 {
		t.Errorf("expected 0 already run, got %d", len(ar))
	}
	if len(missing) != 3 { // user1, user3, alt1
		t.Errorf("expected 3 missing, got %d", len(missing))
	}

	// Simulate user1 pressing "Ran Coop"
	reactingUserIDs := append([]string{"user1"}, c.Boosters["user1"].Alts...)
	for _, id := range reactingUserIDs {
		targetBooster := c.Boosters[id]
		if targetBooster == nil {
			continue
		}
		for _, coopMemberID := range c.Order {
			if slices.Contains(reactingUserIDs, coopMemberID) {
				continue
			}
			if !slices.Contains(targetBooster.RanChickensOn, coopMemberID) {
				targetBooster.RanChickensOn = append(targetBooster.RanChickensOn, coopMemberID)
			}
		}
	}

	// Simulate user1 pressing "Ran Coop" again (multiple times)
	for repeat := 0; repeat < 2; repeat++ {
		for _, id := range reactingUserIDs {
			targetBooster := c.Boosters[id]
			if targetBooster == nil {
				continue
			}
			for _, coopMemberID := range c.Order {
				if slices.Contains(reactingUserIDs, coopMemberID) {
					continue
				}
				if !slices.Contains(targetBooster.RanChickensOn, coopMemberID) {
					targetBooster.RanChickensOn = append(targetBooster.RanChickensOn, coopMemberID)
				}
			}
		}
	}

	// Verify lengths to ensure no duplicates after repeated button presses
	if len(c.Boosters["user1"].RanChickensOn) != 2 {
		t.Errorf("expected user1 RanChickensOn length to be 2 with no duplicates, got %d (%v)", len(c.Boosters["user1"].RanChickensOn), c.Boosters["user1"].RanChickensOn)
	}
	if len(c.Boosters["alt1"].RanChickensOn) != 2 {
		t.Errorf("expected alt1 RanChickensOn length to be 2 with no duplicates, got %d (%v)", len(c.Boosters["alt1"].RanChickensOn), c.Boosters["alt1"].RanChickensOn)
	}

	// Verify user1 and alt1 both have user2 and user3 in RanChickensOn
	if !slices.Contains(c.Boosters["user1"].RanChickensOn, "user2") || !slices.Contains(c.Boosters["user1"].RanChickensOn, "user3") {
		t.Errorf("expected user1 to have ran chickens on user2 and user3, got %v", c.Boosters["user1"].RanChickensOn)
	}
	if !slices.Contains(c.Boosters["alt1"].RanChickensOn, "user2") || !slices.Contains(c.Boosters["alt1"].RanChickensOn, "user3") {
		t.Errorf("expected alt1 to have ran chickens on user2 and user3, got %v", c.Boosters["alt1"].RanChickensOn)
	}
	// Verify neither ran on themselves or each other
	if slices.Contains(c.Boosters["user1"].RanChickensOn, "user1") || slices.Contains(c.Boosters["user1"].RanChickensOn, "alt1") {
		t.Errorf("user1 ran chickens list contains self or alt: %v", c.Boosters["user1"].RanChickensOn)
	}

	// Now user2 makes a chicken run request later
	c.Boosters["user2"].RunChickensTime = time.Now()
	ar2, missing2, _ := buildChickenRunLists(c, "user2")

	// user1 and alt1 should now be in alreadyRun!
	if len(ar2) != 2 {
		t.Errorf("expected 2 in alreadyRun (user1 and alt1), got %d: %v", len(ar2), ar2)
	}
	if len(missing2) != 1 { // only user3 is missing
		t.Errorf("expected 1 in missing (user3), got %d: %v", len(missing2), missing2)
	}
}

func TestBuildCRMessageComponentsTipAndButtons(t *testing.T) {
	c := &Contract{
		ContractHash:  "test-hash-2",
		CRNoticeCount: 0,
		Order:         []string{"user1", "user2"},
		Boosters: map[string]*Booster{
			"user1": {
				UserID:          "user1",
				Nick:            "Player1",
				RunChickensTime: time.Now(),
			},
			"user2": {
				UserID: "user2",
				Nick:   "Player2",
			},
		},
	}

	// First display (CRNoticeCount = 0): should contain helper tip
	msg, _ := buildCRMessage(c, "@CoopRole")
	if msg.Content == "" {
		t.Fatalf("expected message Content, got none")
	}
	if !strings.HasPrefix(msg.Content, "**Player1** is requesting chicken runs @CoopRole!") {
		t.Errorf("expected header to start with '**Player1** is requesting chicken runs @CoopRole!', got: %s", msg.Content)
	}
	if !slices.Contains([]rune(msg.Content), rune('-')) || !strings.Contains(msg.Content, "💡 Use the Boost Menu to indicate you aren't holding runs.") {
		t.Errorf("expected header to contain helper tip when CRNoticeCount < 2, got: %s", msg.Content)
	}
	if !msg.ComponentsV1 {
		t.Errorf("expected ComponentsV1 to be true")
	}
	if len(msg.Embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(msg.Embeds))
	}
	if msg.Embeds[0].Title != "Active Chicken Run Requests" {
		t.Errorf("expected embed title 'Active Chicken Run Requests', got %s", msg.Embeds[0].Title)
	}

	// Without role mention: mention should be omitted cleanly
	msgNoRole, _ := buildCRMessage(c, "")
	if !strings.HasPrefix(msgNoRole.Content, "**Player1** is requesting chicken runs!\n") {
		t.Errorf("expected header without role to start with '**Player1** is requesting chicken runs!\\n', got: %s", msgNoRole.Content)
	}

	// After 2 displays (CRNoticeCount = 2): should NOT contain helper tip
	c.CRNoticeCount = 2
	msg2, _ := buildCRMessage(c, "@CoopRole")
	if strings.Contains(msg2.Content, "Use the Boost Menu to indicate you aren't holding runs.") {
		t.Errorf("expected header NOT to contain helper tip when CRNoticeCount >= 2, got: %s", msg2.Content)
	}
}

func TestBuildCRMessageComponentsCompleted(t *testing.T) {
	c := &Contract{
		ContractHash: "test-hash-completed",
		Order:        []string{"user1", "user2"},
		Boosters: map[string]*Booster{
			"user1": {
				UserID:          "user1",
				Nick:            "Player1",
				RunChickensTime: time.Now(),
			},
			"user2": {
				UserID:        "user2",
				Nick:          "Player2",
				RanChickensOn: []string{"user1"},
			},
		},
	}

	msg, allowedMentions := buildCRMessage(c, "")
	if len(allowedMentions) != 0 {
		t.Errorf("expected no allowed mentions when all complete, got %v", allowedMentions)
	}
	if !msg.ComponentsV1 {
		t.Errorf("expected ComponentsV1 to be true")
	}
	if !msg.ClearComponents {
		t.Errorf("expected ClearComponents to be true")
	}
	if len(msg.Embeds) != 1 {
		t.Fatalf("expected 1 embed when complete, got %d", len(msg.Embeds))
	}
	if msg.Embeds[0].Description == "" || !strings.Contains(msg.Embeds[0].Description, "Player1") {
		t.Errorf("expected completion message with Player1, got %q", msg.Embeds[0].Description)
	}
}

func TestBuildCRMessageMultipleRequestersCarriedAlongUntilAllComplete(t *testing.T) {
	c := &Contract{
		ContractHash: "test-hash-multi-requester",
		Order:        []string{"400000000000000001", "400000000000000002", "400000000000000003"},
		PlayStyle:    ContractPlaystyleLeaderboard,
		Boosters: map[string]*Booster{
			"400000000000000001": {
				UserID:          "400000000000000001",
				Nick:            "Player1",
				UserName:        "player1",
				Mention:         "<@400000000000000001>",
				RunChickensTime: time.Now().Add(-2 * time.Minute),
			},
			"400000000000000002": {
				UserID:          "400000000000000002",
				Nick:            "Player2",
				UserName:        "player2",
				Mention:         "<@400000000000000002>",
				RunChickensTime: time.Now().Add(-1 * time.Minute),
			},
			"400000000000000003": {
				UserID:        "400000000000000003",
				Nick:          "Player3",
				UserName:      "player3",
				Mention:       "<@400000000000000003>",
				RanChickensOn: []string{"400000000000000001"},
			},
		},
	}
	// user2 also ran on user1
	c.Boosters["400000000000000002"].RanChickensOn = []string{"400000000000000001"}

	// State: user1 has runs from user2 and user3 -> user1 has 0 missing (complete!).
	// user2 has runs from neither user1 nor user3 -> user2 has 2 missing (incomplete).
	msg, allowedMentions := buildCRMessage(c, "@role")

	// 1. Check that the dialog is NOT collapsed to completion because user2 is still waiting
	if msg.ClearComponents {
		t.Errorf("expected ClearComponents to be false while user2 is still waiting")
	}
	if len(msg.Embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(msg.Embeds))
	}

	// 2. Both Player1 and Player2 must be listed in the embed description
	desc := msg.Embeds[0].Description
	if !strings.Contains(desc, "Player1") {
		t.Errorf("expected completed Player1 to be carried along in embed, got: %s", desc)
	}
	if !strings.Contains(desc, "Player2") {
		t.Errorf("expected incomplete Player2 to be listed in embed, got: %s", desc)
	}
	if !strings.Contains(desc, "0️⃣ **Player1 (2/2)**") {
		t.Errorf("expected Player1 to be listed with 0 missing, got: %s", desc)
	}

	// 3. Only Player2 should have a button, Player1 should not have a button
	if len(msg.Components) == 0 {
		t.Fatalf("expected components for incomplete requester")
	}
	row, ok := msg.Components[0].(dc.ActionRow)
	if !ok {
		t.Fatalf("expected ActionRow, got %T", msg.Components[0])
	}
	if len(row.Components) != 1 {
		t.Fatalf("expected exactly 1 button for Player2, got %d", len(row.Components))
	}
	btn, ok := row.Components[0].(dc.Button)
	if !ok || !strings.Contains(btn.CustomID, "400000000000000002") {
		t.Errorf("expected button for user2, got button customID=%s", btn.CustomID)
	}

	// 4. Header should indicate Player2 is requesting (since user2 is incomplete)
	if !strings.Contains(msg.Content, "Player2") {
		t.Errorf("expected ping header to name Player2 who still needs runs, got: %s", msg.Content)
	}
	if len(allowedMentions) == 0 {
		t.Errorf("expected allowed mentions for user2's missing runners")
	}

	// 5. Now user1 and user3 run on user2, completing all runs for everyone
	c.Boosters["400000000000000001"].RanChickensOn = append(c.Boosters["400000000000000001"].RanChickensOn, "400000000000000002")
	c.Boosters["400000000000000003"].RanChickensOn = append(c.Boosters["400000000000000003"].RanChickensOn, "400000000000000002")

	msgCompleted, mentionsCompleted := buildCRMessage(c, "@role")
	if len(mentionsCompleted) != 0 {
		t.Errorf("expected no allowed mentions when all completed, got %v", mentionsCompleted)
	}
	if !msgCompleted.ClearComponents {
		t.Errorf("expected ClearComponents to be true when all completed")
	}
	if len(msgCompleted.Embeds) != 1 {
		t.Fatalf("expected 1 embed when complete, got %d", len(msgCompleted.Embeds))
	}
	compDesc := msgCompleted.Embeds[0].Description
	if !strings.Contains(compDesc, "Player1") || !strings.Contains(compDesc, "Player2") {
		t.Errorf("expected completion message to include both Player1 and Player2, got: %s", compDesc)
	}
	if !strings.Contains(compDesc, "requests are complete") {
		t.Errorf("expected plural 'requests are complete', got: %s", compDesc)
	}
}

func TestButtonReactionRunChickensSendFailureRollback(t *testing.T) {
	c := &Contract{
		ContractHash: "test-hash-rollback",
		Order:        []string{"user1", "user2"},
		Location: []*LocationData{
			{
				GuildID:   "guild1",
				ChannelID: "channel1",
			},
		},
		CRMessageIDs: make(map[string]string),
		Boosters: map[string]*Booster{
			"user1": {
				UserID:     "user1",
				Nick:       "Player1",
				BoostState: BoostStateBoosted,
			},
			"user2": {
				UserID:     "user2",
				Nick:       "Player2",
				BoostState: BoostStateBoosted,
			},
		},
	}

	client := dctest.New()
	client.SendErr = errors.New("failed to send CR message")

	// Trigger buttonReactionRunChickens
	ok, _ := buttonReactionRunChickens(client, c, "user1")
	if !ok {
		t.Fatalf("expected buttonReactionRunChickens to return true")
	}

	// Wait for goroutine to finish rolling back
	var runTime time.Time
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c.mutex.Lock()
		runTime = c.Boosters["user1"].RunChickensTime
		c.mutex.Unlock()
		if runTime.IsZero() {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if !runTime.IsZero() {
		t.Errorf("expected RunChickensTime to be rolled back to zero on send error, got %v", runTime)
	}
}

func TestChickenRunUserInContractRestriction(t *testing.T) {
	c := &Contract{
		ContractHash: "test-hash-restriction",
		Order:        []string{"1001", "1002"},
		Location: []*LocationData{
			{
				GuildID:   "101",
				ChannelID: "102",
			},
		},
		CreatorID:    []string{"1000"},
		CRMessageIDs: make(map[string]string),
		Boosters: map[string]*Booster{
			"1001": {
				UserID:          "1001",
				Nick:            "Player1",
				UserName:        "player1",
				Mention:         "<@1001>",
				BoostState:      BoostStateBoosted,
				RunChickensTime: time.Now(),
			},
			"1002": {
				UserID:     "1002",
				Nick:       "Player2",
				UserName:   "player2",
				Mention:    "<@1002>",
				BoostState: BoostStateBoosted,
			},
		},
	}

	Contracts[c.ContractHash] = c
	defer delete(Contracts, c.ContractHash)

	client := dctest.New()

	// 1. Outsider (not in contract) clicking RanChicken button
	outsiderButtonEvent := dctest.ComponentButtonEventWithUser("rc_#RanChicken#1001#"+c.ContractHash, "1099")
	HandleContractReactions(client, outsiderButtonEvent)

	// Verify outsider was not able to run chickens on 1001
	if slices.Contains(c.Boosters["1001"].RanChickensOn, "1099") {
		t.Errorf("outsider should not have run chickens")
	}

	// 2. Creator (not in contract) clicking RanChicken button
	creatorButtonEvent := dctest.ComponentButtonEventWithUser("rc_#RanChicken#1001#"+c.ContractHash, "1000")
	HandleContractReactions(client, creatorButtonEvent)
	if slices.Contains(c.Boosters["1001"].RanChickensOn, "1000") {
		t.Errorf("creator not in contract should not have run chickens")
	}

	// 3. Outsider (not in contract) selecting CRPing
	outsiderSelectEvent := dctest.ComponentSelectEventWithUser("rc_#CRPing#"+c.ContractHash, "1099", "1001")
	HandleContractReactions(client, outsiderSelectEvent)
	// Verify client did NOT send any message to channel
	if len(client.SentMessages) != 0 {
		t.Errorf("outsider should not have triggered a CRPing message, got %d messages", len(client.SentMessages))
	}

	// 4. Creator (not in contract) selecting CRPing -> allowed because admin/coordinator can send pings
	creatorSelectEvent := dctest.ComponentSelectEventWithUser("rc_#CRPing#"+c.ContractHash, "1000", "1001")
	HandleContractReactions(client, creatorSelectEvent)
	if len(client.SentMessages) != 1 {
		t.Errorf("creator should have been allowed to trigger CRPing message, got %d messages", len(client.SentMessages))
	}

	// 5. In-contract user 1002 clicking RanChicken button
	user2ButtonEvent := dctest.ComponentButtonEventWithUser("rc_#RanChicken#1001#"+c.ContractHash, "1002")
	HandleContractReactions(client, user2ButtonEvent)
	if !slices.Contains(c.Boosters["1002"].RanChickensOn, "1001") {
		t.Errorf("user2 in contract should have run chickens on 1001")
	}

	// 6. Direct buttonReactionCRPing test with outsider
	client.SentMessages = nil
	buttonReactionCRPing(client, outsiderSelectEvent, c, "1099")
	if len(client.SentMessages) != 0 {
		t.Errorf("direct buttonReactionCRPing should not send message for outsider")
	}

	// 7. Direct buttonReactionCRPing test with creator (reset RanChickensOn so player is remaining)
	c.Boosters["1002"].RanChickensOn = nil
	buttonReactionCRPing(client, creatorSelectEvent, c, "1000")
	if len(client.SentMessages) != 1 {
		t.Errorf("direct buttonReactionCRPing should send message for creator")
	}

	// 8. Direct buttonReactionRanChicken test with outsider
	buttonReactionRanChicken(client, outsiderButtonEvent, c, "1099", "1001")
	if slices.Contains(c.Boosters["1001"].RanChickensOn, "1099") {
		t.Errorf("direct buttonReactionRanChicken should not add outsider to RanChickensOn")
	}

	// 9. Direct buttonReactionRanChicken test with creator not in contract
	buttonReactionRanChicken(client, creatorButtonEvent, c, "1000", "1001")
	if slices.Contains(c.Boosters["1001"].RanChickensOn, "1000") {
		t.Errorf("direct buttonReactionRanChicken should not add creator to RanChickensOn")
	}
}

func TestBumpCRMessagesAndSetChickenRunMessageIDDeadlock(t *testing.T) {
	client := dctest.New()
	c := &Contract{
		ContractHash: "test-cr-bump-hash",
		Location: []*LocationData{
			{
				ChannelID:   "chan-1",
				GuildID:     "guild-1",
				RoleMention: "@role",
			},
		},
		Order: []string{"userA", "userB"},
		Boosters: map[string]*Booster{
			"userA": {
				UserID:          "userA",
				Nick:            "PlayerA",
				Mention:         "<@userA>",
				RunChickensTime: time.Now(),
			},
			"userB": {
				UserID:  "userB",
				Nick:    "PlayerB",
				Mention: "<@userB>",
			},
		},
		CRMessageIDs: make(map[string]string),
	}

	ContractsMutex.Lock()
	Contracts[c.ContractHash] = c
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, c.ContractHash)
		ContractsMutex.Unlock()
	}()

	done := make(chan struct{})
	go func() {
		bumpCRMessages(client, c)
		close(done)
	}()

	select {
	case <-done:
		// success, no deadlock
	case <-time.After(3 * time.Second):
		t.Fatal("bumpCRMessages deadlocked")
	}

	if c.CRMessageIDs["chan-1"] == "" {
		t.Errorf("expected CRMessageIDs for chan-1 to be set, got empty")
	}
}

func TestChickenRunMigrationCleansUpOldMessage(t *testing.T) {
	client := dctest.New()
	c := &Contract{
		ContractHash: "test-cr-migration-hash",
		Location: []*LocationData{
			{
				ChannelID:   "chan-1",
				GuildID:     "guild-1",
				RoleMention: "@role",
			},
		},
		Order: []string{"userA", "userB"},
		Boosters: map[string]*Booster{
			"userA": {
				UserID:     "userA",
				Nick:       "PlayerA",
				Mention:    "<@userA>",
				BoostState: BoostStateBoosted,
			},
			"userB": {
				UserID:     "userB",
				Nick:       "PlayerB",
				Mention:    "<@userB>",
				BoostState: BoostStateBoosted,
			},
		},
		CRMessageIDs: make(map[string]string),
	}

	ContractsMutex.Lock()
	Contracts[c.ContractHash] = c
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, c.ContractHash)
		ContractsMutex.Unlock()
	}()

	// 1. User A requests chicken runs
	ok, _ := buttonReactionRunChickens(client, c, "userA")
	if !ok {
		t.Fatalf("expected buttonReactionRunChickens for userA to succeed")
	}

	// Wait for goroutine to finish posting message 1
	var firstMsgID string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c.mutex.Lock()
		firstMsgID = c.CRMessageIDs["chan-1"]
		c.mutex.Unlock()
		if firstMsgID != "" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if firstMsgID == "" {
		t.Fatalf("expected CR message ID to be set for userA request")
	}

	// 2. User B requests chicken runs (new requestor)
	ok, _ = buttonReactionRunChickens(client, c, "userB")
	if !ok {
		t.Fatalf("expected buttonReactionRunChickens for userB to succeed")
	}

	// Wait for goroutine to update CRMessageIDs to second message
	var secondMsgID string
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c.mutex.Lock()
		secondMsgID = c.CRMessageIDs["chan-1"]
		c.mutex.Unlock()
		if secondMsgID != "" && secondMsgID != firstMsgID {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if secondMsgID == "" || secondMsgID == firstMsgID {
		t.Fatalf("expected CR message ID to change to second message, got %s", secondMsgID)
	}

	// 3. Verify EditMessage was called on firstMsgID to migrate and clean it up
	var editedMsg *dc.Message
	for _, m := range client.EditedMessages {
		if strings.Contains(m.Content, "Chicken Run request moved") && strings.Contains(m.Content, secondMsgID) {
			editedMsg = &m
			break
		}
	}
	if editedMsg == nil {
		t.Fatalf("expected an edit call redirecting to %s, got edits: %+v", secondMsgID, client.EditedMessages)
	}
	if !editedMsg.ClearComponents {
		t.Errorf("expected ClearComponents: true on migrated message")
	}
	if !editedMsg.ClearEmbeds {
		t.Errorf("expected ClearEmbeds: true on migrated message")
	}
}
