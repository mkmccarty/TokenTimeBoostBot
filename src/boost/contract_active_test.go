package boost

import (
	"strings"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
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
