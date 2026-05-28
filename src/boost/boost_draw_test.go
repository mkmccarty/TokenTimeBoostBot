package boost

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestDrawBoostListOutputScenarios(t *testing.T) {
	scenarios := []struct {
		name         string
		totalPlayers int
		currentIdx   int
	}{
		{
			name:         "Compact - 15 Players (Current in middle)",
			totalPlayers: 15,
			currentIdx:   7,
		},
		{
			name:         "Compact - 30 Players (Current near beginning)",
			totalPlayers: 30,
			currentIdx:   2,
		},
		{
			name:         "Compact - 30 Players (Current near end)",
			totalPlayers: 30,
			currentIdx:   28,
		},
		{
			name:         "Large Contract - 60 Players (Current in middle)",
			totalPlayers: 60,
			currentIdx:   30,
		},
	}

	for _, tc := range scenarios {
		t.Run(tc.name, func(t *testing.T) {
			contract := &Contract{
				ContractHash: "test-hash",
				ContractID:   "test-contract",
				CoopID:       "test-coop",
				State:        ContractStateWaiting,
				Style:        ContractStyleFastrun,
				CreatorID:    []string{"creator-id"},
				Order:        make([]string, tc.totalPlayers),
				Boosters:     make(map[string]*Booster),
				Location:     []*LocationData{{GuildID: "guild1", ChannelID: "channel1"}},
			}

			for i := 0; i < tc.totalPlayers; i++ {
				userID := fmt.Sprintf("user%02d", i)
				contract.Order[i] = userID

				// Determine boost state based on position relative to currentIdx
				boostState := BoostStateUnboosted
				if i < tc.currentIdx {
					boostState = BoostStateBoosted
				} else if i == tc.currentIdx {
					boostState = BoostStateTokenTime
				}

				contract.Boosters[userID] = &Booster{
					UserID:       userID,
					Mention:      fmt.Sprintf("<@%s>", userID),
					Name:         fmt.Sprintf("Farmer%02d", i),
					TokensWanted: 6,
					BoostState:   boostState,
				}
			}

			contract.CurrentBoosterUserID = contract.Order[tc.currentIdx]
			contract.BoostPosition = tc.currentIdx

			components := DrawBoostList(nil, contract)

			var outputBuilder strings.Builder
			fmt.Fprintf(&outputBuilder, "\n=== Scenario: %s ===\n", tc.name)
			fmt.Fprintf(&outputBuilder, "Total Players: %d | Current Active Booster Index: %d (%s)\n", tc.totalPlayers, tc.currentIdx, contract.CurrentBoosterUserID)
			outputBuilder.WriteString("--------------------------------------------------------------------------------\n")
			for _, comp := range components {
				if textDisplay, ok := comp.(*discordgo.TextDisplay); ok {
					outputBuilder.WriteString(textDisplay.Content)
				}
			}
			outputBuilder.WriteString("\n--------------------------------------------------------------------------------\n")

			t.Log(outputBuilder.String())
		})
	}
}
