package dashboard

import (
	"strings"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
)

func TestDashboardActiveContractsCarpetLink(t *testing.T) {
	client := dctest.New()
	userID := "100000000000000099"

	oldContracts := boost.Contracts
	defer func() {
		boost.Contracts = oldContracts
	}()

	testContract := &boost.Contract{
		ContractID: "test-contract-abc",
		CoopID:     "my-test-coop",
		Name:       "Super Food Contract",
		Boosters: map[string]*boost.Booster{
			userID: {UserID: userID, Nick: "Tester"},
		},
		Order: []string{userID},
	}
	boost.Contracts = map[string]*boost.Contract{
		"test-contract-abc": testContract,
	}

	components := drawDashboard(client, userID, false)

	found := false
	expected := "**Super Food Contract / [my-test-coop](https://eicoop-carpet.netlify.app/test-contract-abc/my-test-coop)**"

	for _, comp := range components {
		if container, ok := comp.(dc.Container); ok {
			for _, sub := range container.Components {
				if td, ok := sub.(dc.TextDisplay); ok {
					if strings.Contains(td.Content, expected) {
						found = true
						break
					}
				}
			}
		}
	}

	if !found {
		t.Errorf("expected dashboard active contracts to contain %q, but was not found in components", expected)
	}
}
