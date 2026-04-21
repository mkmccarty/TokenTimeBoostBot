package boost

import (
	"fmt"
	"slices"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

func TestGetHabIconStrings(t *testing.T) {
	// Provide a mock GetBotEmojiMarkdown function
	getBotEmojiMarkdown := func(name string) string {
		return fmt.Sprintf("<:%s:>", name)
	}

	testCases := []struct {
		name             string
		habs             []uint32
		expectedHabArt   string
		expectedHabArray []string
	}{
		{
			name:             "Single low-tier hab",
			habs:             []uint32{0}, // Coop
			expectedHabArt:   "<:hab1:>",
			expectedHabArray: []string{"<:hab1:>"},
		},
		{
			name:             "Multiple low-tier habs",
			habs:             []uint32{0, 1, 2},
			expectedHabArt:   "<:hab3:>",
			expectedHabArray: []string{"<:hab1:>", "<:hab2:>", "<:hab3:>"},
		},
		{
			name:             "Includes high-tier habs",
			habs:             []uint32{0, 8, 17}, // Coop, Bunker, Planet Portal
			expectedHabArt:   "<:hab18:>",
			expectedHabArray: []string{"<:hab1:>", "<:hab9:>", "<:hab18:>"},
		},
		{
			name:             "Highest hab is Chicken Universe",
			habs:             []uint32{18}, // Chicken Universe
			expectedHabArt:   "<:hab19:>",  // highestHab is not updated because 18 is not < 19
			expectedHabArray: []string{"<:hab19:>"},
		},
		{
			name:             "Mixed with highest hab",
			habs:             []uint32{4, 18, 17},
			expectedHabArt:   "<:hab19:>",
			expectedHabArray: []string{"<:hab5:>", "<:hab19:>", "<:hab18:>"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			habArt, habArray := getHabIconStrings(tc.habs, getBotEmojiMarkdown)
			if habArt != tc.expectedHabArt {
				t.Errorf("getHabIconStrings() habArt = %v, want %v", habArt, tc.expectedHabArt)
			}
			if !slices.Equal(habArray, tc.expectedHabArray) {
				t.Errorf("getHabIconStrings() habArray = %v, want %v", habArray, tc.expectedHabArray)
			}
		})
	}
}

func TestVirtueData_Integration(t *testing.T) {
	maker := ei.NewBackupMaker("EI123456", "TestUser")

	// Extracted from firstcontact JSON fixture
	eovEarned := []uint32{20, 20, 21, 20, 20}
	eggsDelivered := []float64{671228376732604700, 281084978380496960, 1861466584249923000, 321860104553812600, 766522163167315600}
	tankFuels := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 77804200160814.05, 247867287321.0276, 1759725104048.5757, 65000000000001.2, 75480247097414}
	tankLimits := []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0.36, 0.02, 0.45, 0.29, 0.35}

	// Create missing data for the backup maker
	maker.SetVirtueData(36, 1, eovEarned, eggsDelivered, 11215366.125829924)
	maker.SetVirtueAFX(1.0, true, true, tankFuels, tankLimits, 9, 48448.569999999956)

	backup := maker.GetBackup()
	virtue := backup.GetVirtue()

	if virtue == nil {
		t.Fatalf("Expected Virtue data to be initialized")
	}

	// Evaluate standard function calls that would be made in virtue.go
	if virtue.GetShiftCount() != 36 {
		t.Errorf("Expected ShiftCount 36, got %d", virtue.GetShiftCount())
	}
	if virtue.GetResets() != 1 {
		t.Errorf("Expected Resets 1, got %d", virtue.GetResets())
	}
	if virtue.GetPastSimTime() != 11215366.125829924 {
		t.Errorf("Expected PastSimTime 11215366.125829924, got %f", virtue.GetPastSimTime())
	}

	// Verify array populations
	if !slices.Equal(virtue.GetEovEarned(), eovEarned) {
		t.Errorf("EoV earned did not match expected values")
	}
	if !slices.Equal(virtue.GetEggsDelivered(), eggsDelivered) {
		t.Errorf("Eggs delivered did not match expected values")
	}

	// Evaluate AFX components inside Virtue
	afx := virtue.GetAfx()
	if afx == nil {
		t.Fatalf("Expected Virtue AFX to be initialized")
	}
	if afx.GetInventoryScore() != 48448.569999999956 {
		t.Errorf("Expected Inventory Score 48448.569999999956, got %f", afx.GetInventoryScore())
	}
	if !slices.Equal(afx.GetTankFuels(), tankFuels) {
		t.Errorf("Tank fuels did not match expected values")
	}
}

// Mock ei.GetBotEmojiMarkdown
func TestGetVehicleIconStrings(t *testing.T) {
	// Provide a mock GetBotEmojiMarkdown function
	getBotEmojiMarkdown := func(name string) string {
		return fmt.Sprintf("<:%s:>", name)
	}

	testCases := []struct {
		name                 string
		vehicles             []uint32
		trainLength          []uint32
		expectedVehicleArt   string
		expectedVehicleArray string
	}{
		{"Single vehicle", []uint32{0}, []uint32{0}, "<:veh0:>", "<:veh0:>"},
		{"Multiple unique vehicles", []uint32{0, 5, 11}, []uint32{1, 1, 1}, "<:veh11:>", "<:veh0:><:veh5:><:veh11:>"},
		{"Multiple same vehicles", []uint32{5, 5, 5}, []uint32{1, 1, 1}, "<:veh5:>", "<:veh5:>x3"},
		{"Mixed unique and multiple", []uint32{0, 5, 5, 11, 11, 11}, []uint32{1, 1, 1, 2, 3, 4}, "<:veh11:>", "<:veh0:><:veh5:>x2<:veh11:><:tl:><:tl:><:veh11:><:tl:><:tl:><:tl:><:veh11:><:tl:><:tl:><:tl:><:tl:>"},
		{"Maximum Vehicles", []uint32{11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11}, []uint32{10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10}, "<:veh11:>", "<:veh11:><:tl:><:tl:><:tl:><:tl:><:tl:><:tl:><:tl:><:tl:><:tl:><:tl:>x17"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vehicleArt, vehicleArray := getVehicleIconStrings(tc.vehicles, tc.trainLength, getBotEmojiMarkdown)
			if vehicleArt != tc.expectedVehicleArt {
				t.Errorf("getVehicleIconStrings() vehicleArt = %v, want %v", vehicleArt, tc.expectedVehicleArt)
			}
			if vehicleArray != tc.expectedVehicleArray {
				t.Errorf("getVehicleIconStrings() vehicleArray = %v, want %v", vehicleArray, tc.expectedVehicleArray)
			}
		})
	}
}
