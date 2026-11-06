package boost

import (
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"
)

// withTestContracts swaps the global contract map for the duration of a test.
func withTestContracts(t *testing.T, contracts map[string]*Contract) {
	t.Helper()
	saved := Contracts
	Contracts = contracts
	t.Cleanup(func() { Contracts = saved })
}

// componentsText flattens the text of the returned components so a test can
// assert on what the user would read.
func componentsText(components []dc.LayoutComponent) string {
	var b strings.Builder
	for _, component := range components {
		container, ok := component.(dc.Container)
		if !ok {
			continue
		}
		for _, sub := range container.Components {
			if text, ok := sub.(dc.TextDisplay); ok {
				b.WriteString(text.Content)
			}
		}
	}
	return b.String()
}

// TestGetCurrentContractsComponentsUsesGuildForThreadLookup pins that the
// active-threads lookup is made against the guild, not the channel. Discord's
// active-threads endpoint is guild-scoped, so passing a channel ID there
// answers "HTTP 404 (10004): Unknown Guild".
func TestGetCurrentContractsComponentsUsesGuildForThreadLookup(t *testing.T) {
	client := dctest.New().
		WithThread("thread-1", "guild-1", "channel-1", "coop thread").
		WithThread("thread-2", "guild-1", "channel-other", "unrelated thread")

	withTestContracts(t, map[string]*Contract{
		"hash-1": {
			ContractID: "contract-1",
			CoopID:     "coop-1",
			Name:       "Contract One",
			CoopSize:   10,
			State:      ContractStateSignup,
			Boosters:   map[string]*Booster{"user-1": {}},
			Location:   []*LocationData{{GuildID: "guild-1", ChannelID: "thread-1"}},
		},
		"hash-2": {
			ContractID: "contract-2",
			CoopID:     "coop-2",
			Name:       "Contract Two",
			CoopSize:   10,
			State:      ContractStateSignup,
			Boosters:   map[string]*Booster{"user-2": {}},
			Location:   []*LocationData{{GuildID: "guild-1", ChannelID: "thread-2"}},
		},
	})

	_, shown := getCurrentContractsComponents(client, "guild-1", "channel-1")
	if !shown {
		t.Fatalf("expected the contract in channel-1 to be shown")
	}

	calls := client.CallsTo("ActiveThreads")
	if len(calls) != 1 {
		t.Fatalf("ActiveThreads called %d times, want 1", len(calls))
	}
	if calls[0].Args[0] != "guild-1" {
		t.Errorf("ActiveThreads called with %q, want the guild ID %q", calls[0].Args[0], "guild-1")
	}
}

// TestGetCurrentContractsComponentsFiltersOtherChannels pins that only threads
// parented to the requested channel count, since the guild-scoped endpoint
// returns every active thread in the guild.
func TestGetCurrentContractsComponentsFiltersOtherChannels(t *testing.T) {
	client := dctest.New().
		WithThread("thread-2", "guild-1", "channel-other", "unrelated thread")

	withTestContracts(t, map[string]*Contract{
		"hash-2": {
			ContractID: "contract-2",
			CoopID:     "coop-2",
			Name:       "Contract Two",
			CoopSize:   10,
			State:      ContractStateSignup,
			Boosters:   map[string]*Booster{"user-2": {}},
			Location:   []*LocationData{{GuildID: "guild-1", ChannelID: "thread-2"}},
		},
	})

	components, shown := getCurrentContractsComponents(client, "guild-1", "channel-1")
	if shown {
		t.Errorf("contract in another channel should not be shown")
	}
	if len(components) == 0 {
		t.Fatal("expected components to be returned")
	}
	if strings.Contains(componentsText(components), "Error retrieving active contracts") {
		t.Errorf("unexpected error component: %s", componentsText(components))
	}
}

func TestGetContractDisplayStartOffsetOnlyInSignupMode(t *testing.T) {
	startTime := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

	signupContract := &Contract{
		ContractID:       "contract-1",
		CoopID:           "coop-signup",
		Name:             "Contract Signup",
		CoopSize:         10,
		State:            ContractStateSignup,
		PlannedStartTime: startTime,
		Boosters:         map[string]*Booster{"user-1": {}},
		Location:         []*LocationData{{GuildID: "guild-1", ChannelID: "thread-1"}},
	}

	activeContract := &Contract{
		ContractID:       "contract-1",
		CoopID:           "coop-active",
		Name:             "Contract Active",
		CoopSize:         10,
		State:            ContractStateFastrun,
		PlannedStartTime: startTime,
		Boosters:         map[string]*Booster{"user-2": {}},
		Location:         []*LocationData{{GuildID: "guild-1", ChannelID: "thread-2"}},
	}

	activeThreadIDs := map[string]bool{
		"thread-1": true,
		"thread-2": true,
	}

	signupDisplay := getContractDisplay(signupContract, []*Contract{signupContract}, activeThreadIDs, 4000)
	if !strings.Contains(signupDisplay.Content, "↳Start:") {
		t.Errorf("expected signup contract to display start offset time, got: %s", signupDisplay.Content)
	}

	activeDisplay := getContractDisplay(activeContract, []*Contract{activeContract}, activeThreadIDs, 4000)
	if strings.Contains(activeDisplay.Content, "↳Start:") {
		t.Errorf("expected non-signup contract not to display start offset time, got: %s", activeDisplay.Content)
	}
}

func TestGetCurrentContractsComponentsSorting(t *testing.T) {
	startTime1 := time.Now().Add(1 * time.Hour)
	startTime2 := time.Now().Add(2 * time.Hour)

	client := dctest.New().
		WithThread("th-boost-unfilled", "guild-1", "channel-1", "boost-unfilled").
		WithThread("th-wait-unfilled", "guild-1", "channel-1", "wait-unfilled").
		WithThread("th-signup-immediate", "guild-1", "channel-1", "signup-immediate").
		WithThread("th-signup-planned-early", "guild-1", "channel-1", "signup-planned-early").
		WithThread("th-signup-planned-late", "guild-1", "channel-1", "signup-planned-late").
		WithThread("th-signup-full", "guild-1", "channel-1", "signup-full").
		WithThread("th-boost-full-b", "guild-1", "channel-1", "boost-full-b").
		WithThread("th-boost-full-a", "guild-1", "channel-1", "boost-full-a").
		WithThread("th-wait-full", "guild-1", "channel-1", "wait-full").
		WithThread("th-complete", "guild-1", "channel-1", "complete")

	withTestContracts(t, map[string]*Contract{
		"c-boost-unfilled": {
			ContractID: "contract-sort",
			CoopID:     "coop-boost-unfilled",
			Name:       "Sort Contract",
			CoopSize:   5,
			State:      ContractStateFastrun,
			Boosters:   map[string]*Booster{"u1": {}, "u2": {}, "u3": {}}, // Group 1: unfilled running boosting
			Location:   []*LocationData{{GuildID: "guild-1", ChannelID: "th-boost-unfilled"}},
		},
		"c-wait-unfilled": {
			ContractID: "contract-sort",
			CoopID:     "coop-wait-unfilled",
			Name:       "Sort Contract",
			CoopSize:   5,
			State:      ContractStateWaiting,
			Boosters:   map[string]*Booster{"u1": {}, "u2": {}}, // Group 1: unfilled running waiting
			Location:   []*LocationData{{GuildID: "guild-1", ChannelID: "th-wait-unfilled"}},
		},
		"c-signup-immediate": {
			ContractID: "contract-sort",
			CoopID:     "coop-signup-immediate",
			Name:       "Sort Contract",
			CoopSize:   5,
			State:      ContractStateSignup,
			Boosters:   map[string]*Booster{"u1": {}}, // Group 2: unfilled signup without start offset
			Location:   []*LocationData{{GuildID: "guild-1", ChannelID: "th-signup-immediate"}},
		},
		"c-signup-planned-early": {
			ContractID:       "contract-sort",
			CoopID:           "coop-signup-planned-early",
			Name:             "Sort Contract",
			CoopSize:         5,
			State:            ContractStateSignup,
			PlannedStartTime: startTime1,
			Boosters:         map[string]*Booster{"u1": {}}, // Group 2: unfilled signup with early start offset
			Location:         []*LocationData{{GuildID: "guild-1", ChannelID: "th-signup-planned-early"}},
		},
		"c-signup-planned-late": {
			ContractID:       "contract-sort",
			CoopID:           "coop-signup-planned-late",
			Name:             "Sort Contract",
			CoopSize:         5,
			State:            ContractStateSignup,
			PlannedStartTime: startTime2,
			Boosters:         map[string]*Booster{"u1": {}}, // Group 2: unfilled signup with later start offset
			Location:         []*LocationData{{GuildID: "guild-1", ChannelID: "th-signup-planned-late"}},
		},
		"c-signup-full": {
			ContractID:       "contract-sort",
			CoopID:           "coop-signup-full",
			Name:             "Sort Contract",
			CoopSize:         2,
			State:            ContractStateSignup,
			PlannedStartTime: startTime1,
			Boosters:         map[string]*Booster{"u1": {}, "u2": {}}, // Group 3: FULL signup
			Location:         []*LocationData{{GuildID: "guild-1", ChannelID: "th-signup-full"}},
		},
		"c-boost-full-b": {
			ContractID: "contract-sort",
			CoopID:     "coop-boost-full-b",
			Name:       "Sort Contract",
			CoopSize:   3,
			State:      ContractStateFastrun,
			Boosters:   map[string]*Booster{"u1": {}, "u2": {}, "u3": {}}, // Group 4: FULL boosting
			Location:   []*LocationData{{GuildID: "guild-1", ChannelID: "th-boost-full-b"}},
		},
		"c-boost-full-a": {
			ContractID: "contract-sort",
			CoopID:     "coop-boost-full-a",
			Name:       "Sort Contract",
			CoopSize:   2,
			State:      ContractStateBanker,
			Boosters:   map[string]*Booster{"u1": {}, "u2": {}}, // Group 4: FULL boosting (banker)
			Location:   []*LocationData{{GuildID: "guild-1", ChannelID: "th-boost-full-a"}},
		},
		"c-wait-full": {
			ContractID: "contract-sort",
			CoopID:     "coop-wait-full",
			Name:       "Sort Contract",
			CoopSize:   2,
			State:      ContractStateWaiting,
			Boosters:   map[string]*Booster{"u1": {}, "u2": {}}, // Group 4: FULL waiting
			Location:   []*LocationData{{GuildID: "guild-1", ChannelID: "th-wait-full"}},
		},
		"c-complete": {
			ContractID: "contract-sort",
			CoopID:     "coop-complete",
			Name:       "Sort Contract",
			CoopSize:   2,
			State:      ContractStateCompleted,
			Boosters:   map[string]*Booster{"u1": {}, "u2": {}}, // Group 4: Completed
			Location:   []*LocationData{{GuildID: "guild-1", ChannelID: "th-complete"}},
		},
	})

	// Enable active-contracts-show-completed so completed coops are included
	guildstate.SetGuildSettingFlag("guild-1", "active-contracts-show-completed", true)
	t.Cleanup(func() {
		guildstate.SetGuildSettingFlag("guild-1", "active-contracts-show-completed", false)
	})

	components, shown := getCurrentContractsComponents(client, "guild-1", "channel-1")
	if !shown {
		t.Fatalf("expected contracts to be shown")
	}

	text := componentsText(components)

	// Expected order:
	// Group 1: Unfilled running contracts (boosting, then waiting, alpha)
	//   coop-boost-unfilled
	//   coop-wait-unfilled
	// Group 2: Unfilled signup contracts (no start offset, early offset, late offset)
	//   coop-signup-immediate
	//   coop-signup-planned-early
	//   coop-signup-planned-late
	// Group 3: FULL signup contracts (sorts after unfilled, before full running)
	//   coop-signup-full
	// Group 4: FULL contracts active down through completed (boosting alpha, waiting, completed)
	//   coop-boost-full-a
	//   coop-boost-full-b
	//   coop-wait-full
	//   coop-complete
	expectedOrder := []string{
		"coop-boost-unfilled",
		"coop-wait-unfilled",
		"coop-signup-immediate",
		"coop-signup-planned-early",
		"coop-signup-planned-late",
		"coop-signup-full",
		"coop-boost-full-a",
		"coop-boost-full-b",
		"coop-wait-full",
		"coop-complete",
	}

	lastIdx := -1
	for _, coopID := range expectedOrder {
		idx := strings.Index(text, coopID)
		if idx == -1 {
			t.Fatalf("expected %q in text, but not found. Full text:\n%s", coopID, text)
		}
		if idx <= lastIdx {
			t.Fatalf("expected %q to appear after previous coop (idx %d <= %d). Full text:\n%s", coopID, idx, lastIdx, text)
		}
		lastIdx = idx
	}
}

func TestGetContractDisplayBudgetStrict(t *testing.T) {
	activeThreadIDs := map[string]bool{
		"thread-1": true,
		"thread-2": true,
		"thread-3": true,
	}

	header := &Contract{
		ContractID: "contract-1",
		CoopID:     "coop-1",
		Name:       "Contract Budget Test",
		CoopSize:   10,
		State:      ContractStateFastrun,
		Boosters:   map[string]*Booster{"u1": {}},
		Location:   []*LocationData{{GuildID: "guild-1", ChannelID: "thread-1"}},
	}

	coops := []*Contract{
		{
			ContractID: "contract-1",
			CoopID:     "coop-long-name-1",
			CoopSize:   10,
			State:      ContractStateFastrun,
			Boosters:   map[string]*Booster{"u1": {}},
			Location:   []*LocationData{{GuildID: "guild-1", ChannelID: "thread-1"}},
		},
		{
			ContractID: "contract-1",
			CoopID:     "coop-long-name-2",
			CoopSize:   10,
			State:      ContractStateFastrun,
			Boosters:   map[string]*Booster{"u1": {}},
			Location:   []*LocationData{{GuildID: "guild-1", ChannelID: "thread-2"}},
		},
		{
			ContractID: "contract-1",
			CoopID:     "coop-long-name-3",
			CoopSize:   10,
			State:      ContractStateFastrun,
			Boosters:   map[string]*Booster{"u1": {}},
			Location:   []*LocationData{{GuildID: "guild-1", ChannelID: "thread-3"}},
		},
	}

	// Calculate header length
	emptyDisplay := getContractDisplay(header, nil, activeThreadIDs, 4000)
	headerLen := len(emptyDisplay.Content)

	// Budget that only allows header plus exactly one coop
	oneCoopDisplay := getContractDisplay(header, coops[:1], activeThreadIDs, 4000)
	exactOneCoopBudget := len(oneCoopDisplay.Content)

	// Provide budget that fits 1 coop, but not 2
	budget := exactOneCoopBudget + 10
	display := getContractDisplay(header, coops, activeThreadIDs, budget)

	if len(display.Content) > budget {
		t.Errorf("display content length %d exceeds budget %d", len(display.Content), budget)
	}
	if !strings.Contains(display.Content, "coop-long-name-1") {
		t.Errorf("expected coop-long-name-1 in display")
	}
	if strings.Contains(display.Content, "coop-long-name-2") {
		t.Errorf("coop-long-name-2 should not have fit in budget")
	}
	if headerLen >= budget {
		t.Fatalf("sanity check: headerLen %d should be less than budget %d", headerLen, budget)
	}

	// Budget that fits 1 coop plus the omitted suffix
	budgetWithSuffix := exactOneCoopBudget + 60
	displayWithSuffix := getContractDisplay(header, coops, activeThreadIDs, budgetWithSuffix)
	if !strings.Contains(displayWithSuffix.Content, "… and 2 more full contracts") {
		t.Errorf("expected omitted contracts suffix in display, got:\n%s", displayWithSuffix.Content)
	}
}

func TestGetCurrentContractsComponentsSeparatorBetweenContracts(t *testing.T) {
	client := dctest.New().
		WithThread("thread-c1", "guild-1", "channel-1", "coop-c1").
		WithThread("thread-c2", "guild-1", "channel-1", "coop-c2")

	withTestContracts(t, map[string]*Contract{
		"c-1": {
			ContractID: "contract-1",
			CoopID:     "coop-1",
			Name:       "Contract 1",
			CoopSize:   5,
			State:      ContractStateSignup,
			Boosters:   map[string]*Booster{"u1": {}},
			Location:   []*LocationData{{GuildID: "guild-1", ChannelID: "thread-c1"}},
		},
		"c-2": {
			ContractID: "contract-2",
			CoopID:     "coop-2",
			Name:       "Contract 2",
			CoopSize:   5,
			State:      ContractStateSignup,
			Boosters:   map[string]*Booster{"u2": {}},
			Location:   []*LocationData{{GuildID: "guild-1", ChannelID: "thread-c2"}},
		},
	})

	components, shown := getCurrentContractsComponents(client, "guild-1", "channel-1")
	if !shown {
		t.Fatalf("expected contracts to be shown")
	}

	container, ok := components[0].(dc.Container)
	if !ok {
		t.Fatalf("expected first component to be Container, got %T", components[0])
	}

	hasSeparator := false
	for _, sub := range container.Components {
		if _, isSep := sub.(dc.Separator); isSep {
			hasSeparator = true
			break
		}
	}

	if !hasSeparator {
		t.Errorf("expected dc.Separator between multiple contracts in container")
	}
}

func TestGetActiveContractIDs(t *testing.T) {
	withTestContracts(t, map[string]*Contract{
		"c-1": {
			ContractID: "contract-active-1",
			State:      ContractStateSignup,
		},
		"c-2": {
			ContractID: "contract-active-2",
			State:      ContractStateFastrun,
		},
		"c-3": {
			ContractID: "contract-active-1", // duplicate contract ID with different coop
			State:      ContractStateWaiting,
		},
		"c-4": {
			ContractID: "contract-archived",
			State:      ContractStateArchive,
		},
		"c-5": {
			ContractID: "",
			State:      ContractStateSignup,
		},
	})

	ids := GetActiveContractIDs()
	if len(ids) != 2 {
		t.Fatalf("expected 2 active contract IDs, got %d: %v", len(ids), ids)
	}

	found1, found2 := false, false
	for _, id := range ids {
		if id == "contract-active-1" {
			found1 = true
		}
		if id == "contract-active-2" {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Errorf("expected contract-active-1 and contract-active-2, got %v", ids)
	}
}
