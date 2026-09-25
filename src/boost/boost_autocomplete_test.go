package boost

import (
	"strings"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

func TestBoostOrderAutoComplete(t *testing.T) {
	testUser := "test_ac_user_123"

	// Setup user custom order template
	SaveUserCustomOrder(testUser, CustomBoostOrderTemplate{
		Name:      "Alpha Deflector",
		Lines:     []string{"<DEFL", ">TE"},
		CreatorID: testUser,
	})
	SaveUserCustomOrder(testUser, CustomBoostOrderTemplate{
		Name:      "Beta IHR",
		Lines:     []string{"<IHR[10%]", "RANDOM"},
		CreatorID: testUser,
	})

	// Test 1: typing "user" brings up user defined boost orders
	choicesUser := getBoostOrderAutoCompleteChoices(testUser, "user")
	if len(choicesUser) == 0 {
		t.Fatal("expected choices when searching for 'user', got 0")
	}
	foundAlpha := false
	foundBeta := false
	for _, c := range choicesUser {
		if strings.Contains(c.Name, "Alpha Deflector") && c.Value == "custom_u:Alpha Deflector" {
			foundAlpha = true
		}
		if strings.Contains(c.Name, "Beta IHR") && c.Value == "custom_u:Beta IHR" {
			foundBeta = true
		}
	}
	if !foundAlpha || !foundBeta {
		t.Errorf("expected Alpha Deflector and Beta IHR in 'user' choices, got %+v", choicesUser)
	}

	// Test 2: typing "custom" brings up user defined boost orders
	choicesCustom := getBoostOrderAutoCompleteChoices(testUser, "custom")
	if len(choicesCustom) == 0 {
		t.Fatal("expected choices when searching for 'custom', got 0")
	}
	foundAlphaInCustom := false
	foundStandardCustom := false
	for _, c := range choicesCustom {
		if strings.Contains(c.Name, "Alpha Deflector") {
			foundAlphaInCustom = true
		}
		if c.Name == "Custom Ordering" {
			foundStandardCustom = true
		}
	}
	if !foundAlphaInCustom {
		t.Errorf("expected user defined order in 'custom' choices, got %+v", choicesCustom)
	}
	if !foundStandardCustom {
		t.Errorf("expected standard 'Custom Ordering' in 'custom' choices, got %+v", choicesCustom)
	}

	// Test 3: typing "Alpha" brings up the specific user defined boost order
	choicesAlpha := getBoostOrderAutoCompleteChoices(testUser, "Alpha")
	if len(choicesAlpha) == 0 || !strings.Contains(choicesAlpha[0].Name, "Alpha Deflector") {
		t.Errorf("expected Alpha Deflector when searching for 'Alpha', got %+v", choicesAlpha)
	}

	// Clean up
	DeleteUserCustomOrder(testUser, "Alpha Deflector")
	DeleteUserCustomOrder(testUser, "Beta IHR")
	farmerstate.SetMiscSettingString(testUser, "saved_custom_orders", "")
}
