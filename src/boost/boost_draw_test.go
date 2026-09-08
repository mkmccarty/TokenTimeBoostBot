package boost

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
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

			components := DrawBoostList(contract)

			var outputBuilder strings.Builder
			fmt.Fprintf(&outputBuilder, "\n=== Scenario: %s ===\n", tc.name)
			fmt.Fprintf(&outputBuilder, "Total Players: %d | Current Active Booster Index: %d (%s)\n", tc.totalPlayers, tc.currentIdx, contract.CurrentBoosterUserID)
			outputBuilder.WriteString("--------------------------------------------------------------------------------\n")
			for _, comp := range components {
				if textDisplay, ok := comp.(dc.TextDisplay); ok {
					outputBuilder.WriteString(textDisplay.Content)
				}
			}
			outputBuilder.WriteString("\n--------------------------------------------------------------------------------\n")

			t.Log(outputBuilder.String())
		})
	}
}

func TestHasRenderableBannerURL(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want bool
	}{
		{name: "absolute https", url: "https://example.com/banners/contract-b.png", want: true},
		{name: "absolute http", url: "http://example.com/banners/contract-b.png", want: true},
		{name: "empty", url: "", want: false},
		{name: "bare filename from unset BannerURL config", url: "farmers-market-2026-b.png", want: false},
		{name: "relative path", url: "/banners/contract-b.png", want: false},
		{name: "scheme without host", url: "https:///contract-b.png", want: false},
		{name: "attachment scheme", url: "attachment://contract-b.png", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasRenderableBannerURL(tc.url); got != tc.want {
				t.Errorf("hasRenderableBannerURL(%q) = %v, want %v", tc.url, got, tc.want)
			}
		})
	}
}

// A contract whose banner URL is not absolute must still draw. Discord rejects
// the entire message when a media gallery item carries an unusable URL, which
// used to leave the boost list unposted whenever the BannerURL config field was
// missing.
func TestDrawBoostListSkipsUnusableBanner(t *testing.T) {
	newContract := func(bannerURL string) *Contract {
		return &Contract{
			ContractHash: "banner-test-hash",
			ContractID:   "banner-test-contract",
			CoopID:       "banner-test-coop",
			Description:  "Banner Test Contract",
			State:        ContractStateSignup,
			Style:        ContractStyleFastrun,
			CreatorID:    []string{"creator-id"},
			BannerURL:    bannerURL,
			Order:        []string{},
			Boosters:     map[string]*Booster{},
			Location:     []*LocationData{{GuildID: "guild1", ChannelID: "channel1"}},
		}
	}

	hasGallery := func(components []dc.LayoutComponent) bool {
		for _, comp := range components {
			if _, ok := comp.(dc.MediaGallery); ok {
				return true
			}
		}
		return false
	}

	if hasGallery(DrawBoostList(newContract("banner-test-contract-b.png"))) {
		t.Error("DrawBoostList included a media gallery for a banner URL that is not absolute")
	}

	if !hasGallery(DrawBoostList(newContract("https://example.com/banners/banner-test-contract-b.png"))) {
		t.Error("DrawBoostList dropped the media gallery for a usable banner URL")
	}
}
