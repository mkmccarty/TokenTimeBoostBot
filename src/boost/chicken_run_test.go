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
	comps, _ := buildCRMessageComponents(c, "@CoopRole")
	if len(comps) == 0 {
		t.Fatalf("expected components, got none")
	}
	headerText, ok := comps[0].(dc.TextDisplay)
	if !ok {
		t.Fatalf("expected first component to be TextDisplay")
	}
	if !slices.Contains([]rune(headerText.Content), rune('-')) || !strings.Contains(headerText.Content, "💡 Use the Boost Menu to indicate you aren't holding runs.") {
		t.Errorf("expected header to contain helper tip when CRNoticeCount < 2, got: %s", headerText.Content)
	}

	// After 2 displays (CRNoticeCount = 2): should NOT contain helper tip
	c.CRNoticeCount = 2
	comps2, _ := buildCRMessageComponents(c, "@CoopRole")
	headerText2, ok := comps2[0].(dc.TextDisplay)
	if !ok {
		t.Fatalf("expected first component to be TextDisplay")
	}
	if strings.Contains(headerText2.Content, "Use the Boost Menu to indicate you aren't holding runs.") {
		t.Errorf("expected header NOT to contain helper tip when CRNoticeCount >= 2, got: %s", headerText2.Content)
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

	comps, allowedMentions := buildCRMessageComponents(c, "")
	if len(comps) != 1 {
		t.Fatalf("expected 1 container component when complete, got %d", len(comps))
	}
	if len(allowedMentions) != 0 {
		t.Errorf("expected no allowed mentions when all complete, got %v", allowedMentions)
	}
	container, ok := comps[0].(dc.Container)
	if !ok {
		t.Fatalf("expected component to be Container, got %T", comps[0])
	}
	if len(container.Components) != 1 {
		t.Fatalf("expected 1 text display inside container, got %d", len(container.Components))
	}
	textDisplay, ok := container.Components[0].(dc.TextDisplay)
	if !ok {
		t.Fatalf("expected inner component to be TextDisplay, got %T", container.Components[0])
	}
	if textDisplay.Content == "" || !strings.Contains(textDisplay.Content, "Player1") {
		t.Errorf("expected completion message with Player1, got %q", textDisplay.Content)
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
