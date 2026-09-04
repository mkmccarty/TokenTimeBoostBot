package boost

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
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
	headerText, ok := comps[0].(discordgo.TextDisplay)
	if !ok {
		t.Fatalf("expected first component to be TextDisplay")
	}
	if !slices.Contains([]rune(headerText.Content), rune('-')) || !strings.Contains(headerText.Content, "💡 Use the Boost Menu to indicate you aren't holding runs.") {
		t.Errorf("expected header to contain helper tip when CRNoticeCount < 2, got: %s", headerText.Content)
	}

	// After 2 displays (CRNoticeCount = 2): should NOT contain helper tip
	c.CRNoticeCount = 2
	comps2, _ := buildCRMessageComponents(c, "@CoopRole")
	headerText2, ok := comps2[0].(discordgo.TextDisplay)
	if !ok {
		t.Fatalf("expected first component to be TextDisplay")
	}
	if strings.Contains(headerText2.Content, "Use the Boost Menu to indicate you aren't holding runs.") {
		t.Errorf("expected header NOT to contain helper tip when CRNoticeCount >= 2, got: %s", headerText2.Content)
	}
}
