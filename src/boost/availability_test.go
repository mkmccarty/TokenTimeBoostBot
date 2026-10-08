package boost

import (
	"strings"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

func TestGetAvailabilityComponents_Timeslots(t *testing.T) {
	contract := &Contract{
		ContractHash: "test-hash",
		ContractID:   "test-contract",
		Boosters: map[string]*Booster{
			"userA": {
				UserID: "userA",
				Nick:   "FarmerA",
				Availability: ContractAvailability{
					Timeslots: []string{"09-10", "12-13"},
				},
			},
			"userAll": {
				UserID: "userAll",
				Nick:   "FarmerAll",
				Availability: ContractAvailability{
					Timeslots: []string{"all"},
				},
			},
		},
		Order: []string{"userA", "userAll"},
	}

	components := GetAvailabilityComponents(nil, contract, "userA")
	if len(components) == 0 {
		t.Fatalf("expected non-empty components")
	}

	var foundTimeMenu bool
	var timeOptions []dc.SelectOption
	for _, comp := range components {
		if row, ok := comp.(dc.ActionRow); ok {
			for _, item := range row.Components {
				if menu, ok := item.(dc.SelectMenu); ok && strings.HasPrefix(menu.CustomID, "rc_#predtime#") {
					foundTimeMenu = true
					timeOptions = menu.Options
				}
			}
		}
	}

	if !foundTimeMenu {
		t.Fatalf("expected to find time select menu with customID prefix rc_#predtime#")
	}

	// Should have 18 options: "ALL Times" + +0 through +12 (13 options) + -4 through -1 (4 options)
	if len(timeOptions) != 18 {
		t.Fatalf("expected 18 time options, got %d", len(timeOptions))
	}

	if timeOptions[0].Label != "ALL Times" || timeOptions[0].Value != "all" {
		t.Fatalf("expected first option to be 'ALL Times' (all), got label=%q value=%q", timeOptions[0].Label, timeOptions[0].Value)
	}

	expectedValues := map[string]string{
		"ALL Times": "all",
		"+0":        "00-01",
		"+1":        "01-02",
		"+2":        "02-03",
		"+3":        "03-04",
		"+4":        "04-05",
		"+5":        "05-06",
		"+6":        "06-07",
		"+7":        "07-08",
		"+8":        "08-09",
		"+9":        "09-10",
		"+10":       "10-11",
		"+11":       "11-12",
		"+12":       "12-13",
		"-4":        "20-21",
		"-3":        "21-22",
		"-2":        "22-23",
		"-1":        "23-24",
	}

	for _, opt := range timeOptions {
		expectedVal, ok := expectedValues[opt.Label]
		if !ok {
			t.Errorf("unexpected option label: %s", opt.Label)
			continue
		}
		if opt.Value != expectedVal {
			t.Errorf("for label %s, expected value %s, got %s", opt.Label, expectedVal, opt.Value)
		}
		if opt.Value == "09-10" || opt.Value == "12-13" {
			if !opt.Default {
				t.Errorf("expected option %s to be marked default", opt.Label)
			}
		} else {
			if opt.Default {
				t.Errorf("expected option %s not to be marked default", opt.Label)
			}
		}
	}

	// Check components for userAll
	allComponents := GetAvailabilityComponents(nil, contract, "userAll")
	for _, comp := range allComponents {
		if row, ok := comp.(dc.ActionRow); ok {
			for _, item := range row.Components {
				if menu, ok := item.(dc.SelectMenu); ok && strings.HasPrefix(menu.CustomID, "rc_#predtime#") {
					for _, opt := range menu.Options {
						if opt.Value == "all" && !opt.Default {
							t.Errorf("expected 'all' option to be default for userAll")
						} else if opt.Value != "all" && opt.Default {
							t.Errorf("expected option %s not to be default for userAll", opt.Label)
						}
					}
				}
			}
		}
	}
}

func TestNormalizeTimeslotValues(t *testing.T) {
	// 1. Selecting "all" when previously empty
	res := NormalizeTimeslotValues([]string{"all"}, nil)
	if len(res) != 1 || res[0] != "all" {
		t.Errorf("expected [all], got %v", res)
	}

	// 2. Selecting "all" while having previous specific slots
	res = NormalizeTimeslotValues([]string{"00-01", "all"}, []string{"00-01"})
	if len(res) != 1 || res[0] != "all" {
		t.Errorf("expected [all], got %v", res)
	}

	// 3. User already had "all", then clicked a specific slot
	res = NormalizeTimeslotValues([]string{"all", "02-03"}, []string{"all"})
	if len(res) != 1 || res[0] != "02-03" {
		t.Errorf("expected [02-03], got %v", res)
	}

	// 4. Normal specific selection
	res = NormalizeTimeslotValues([]string{"00-01", "01-02"}, []string{"00-01"})
	if len(res) != 2 || res[0] != "00-01" || res[1] != "01-02" {
		t.Errorf("expected [00-01 01-02], got %v", res)
	}

	// 5. Selecting all 17 slots individually normalizes to ["all"]
	res = NormalizeTimeslotValues(availabilitySortedTimeKeys, []string{"00-01"})
	if len(res) != 1 || res[0] != "all" {
		t.Errorf("expected [all] for full set, got %v", res)
	}
}

func TestFormatTimes(t *testing.T) {
	if got := formatTimes(nil); got != "Not set" {
		t.Errorf("expected 'Not set', got %q", got)
	}
	if got := formatTimes([]string{"all"}); got != "ALL Times" {
		t.Errorf("expected 'ALL Times', got %q", got)
	}
	if got := formatTimes(availabilitySortedTimeKeys); got != "ALL Times" {
		t.Errorf("expected 'ALL Times', got %q", got)
	}
	if got := formatTimes([]string{"00-01", "09-10"}); got != "+0, +9" {
		t.Errorf("expected '+0, +9', got %q", got)
	}
}

func TestDrawBoostList_ExtendedTimeslots(t *testing.T) {
	contract := &Contract{
		ContractHash:     "test-hash",
		ContractID:       "test-contract",
		CreatorID:        []string{"creator-user"},
		Location:         []*LocationData{{GuildID: "guild1", ChannelID: "chan1"}},
		State:            ContractStateSignup,
		PredictionSignup: true,
		PredictionInfo: []PredictionInfo{
			{
				ContractID: "test-contract",
				Name:       "Test Egg Contract",
				EggName:    "superfood",
			},
		},
		Order: []string{"userA", "userB", "userC"},
		Boosters: map[string]*Booster{
			"userA": {
				UserID: "userA",
				Nick:   "FarmerA",
				Availability: ContractAvailability{
					Contract:  []string{"test-contract"},
					Timeslots: []string{"09-10", "12-13"},
				},
			},
			"userB": {
				UserID: "userB",
				Nick:   "FarmerB",
				Availability: ContractAvailability{
					Contract: []string{"test-contract"},
					Timeslots: []string{
						"00-01", "01-02", "02-03", "03-04",
						"04-05", "05-06", "06-07", "07-08",
						"08-09", "09-10", "10-11", "11-12",
						"12-13",
						"20-21", "21-22", "22-23", "23-24",
					},
				},
			},
			"userC": {
				UserID: "userC",
				Nick:   "FarmerC",
				Availability: ContractAvailability{
					Contract:  []string{"test-contract"},
					Timeslots: []string{"all"},
				},
			},
		},
	}

	components := DrawBoostList(contract)
	var output strings.Builder
	for _, comp := range components {
		if textComp, ok := comp.(dc.TextDisplay); ok {
			output.WriteString(textComp.Content)
		}
	}

	rendered := output.String()
	if !strings.Contains(rendered, "+9: 1") {
		t.Errorf("expected output to contain '+9: 1', got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "+12: 1") {
		t.Errorf("expected output to contain '+12: 1', got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Any: 2") {
		t.Errorf("expected output to contain 'Any: 2', got:\n%s", rendered)
	}
}
