package boost

import (
	"fmt"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

// Make sure to use the testing package
func TestDetermineDynamicTokens(t *testing.T) {

	// Create a contract with 10 farmers
	c := new(Contract)
	c.Style = ContractFlagDynamicTokens
	c.Boosters = make(map[string]*Booster)
	c.StartTime = time.Now()
	c.CoopSize = 10
	c.BoostPosition = 2

	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("Farmer%d", i)

		b := new(Booster)
		b.Name = name
		b.UserID = name
		c.Boosters[name] = b
		// Add this user to the boost order
		c.Order = append(c.Order, name)
	}

	// Mark first booster as boosted
	c.Boosters[c.Order[0]].BoostState = BoostStateBoosted
	c.Boosters[c.Order[0]].StartTime = c.StartTime.Add(time.Minute * 10)
	c.Boosters[c.Order[0]].EndTime = c.Boosters[c.Order[0]].StartTime.Add(time.Minute * 10)

	c.Boosters[c.Order[1]].BoostState = BoostStateBoosted
	c.Boosters[c.Order[1]].StartTime = c.Boosters[c.Order[0]].EndTime // After previous booster finishes
	c.Boosters[c.Order[1]].EndTime = c.Boosters[c.Order[1]].StartTime.Add(time.Minute * 10)

	// Create a dummy token log to fake how many tokens we've received
	for i := 0; i < 20; i++ {
		c.TokenLog = append(c.TokenLog, ei.TokenUnitLog{})
	}

	determineDynamicTokens(c)
}

/*

func TestCreateDynamicTokenData(t *testing.T) {
	testCases := []struct {
		name        string
		te          int64
		shouldBeNil bool
	}{
		{"TE_50", 50, false},
		{"TE_0", 0, false},
		{"TE_70", 70, false},
		{"TE_111", 111, false},
		{"TE_141", 141, false},
		{"TE_200", 200, false},
		{"TE_LessThanOne", 0, false},
		{"TE_NegativeOne", -1, false},
		{"TE_Over490", 491, false},
		{"TE_HighValue", 500, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dt := createDynamicTokenData(tc.te)

			if (dt == nil) != tc.shouldBeNil {
				t.Errorf("createDynamicTokenData(%d) returned nil=%v, expected nil=%v", tc.te, dt == nil, tc.shouldBeNil)
				return
			}

			if dt == nil {
				return
			}

			// Validate basic fields are set
			if dt.TE != tc.te {
				t.Errorf("createDynamicTokenData(%d).TE = %d, want %d", tc.te, dt.TE, tc.te)
			}

			// Validate reasonable values
			if dt.HabNumber != 4 {
				t.Errorf("createDynamicTokenData(%d).HabNumber = %d, want 4", tc.te, dt.HabNumber)
			}

			if dt.OfflineIHR != 3 {
				t.Errorf("createDynamicTokenData(%d).OfflineIHR = %d, want 3", tc.te, dt.OfflineIHR)
			}

			if dt.ColleggtibleIHR <= 0 {
				t.Errorf("createDynamicTokenData(%d).ColleggtibleIHR = %f, want > 0", tc.te, dt.ColleggtibleIHR)
			}

			if dt.IhrBase <= 0 {
				t.Errorf("createDynamicTokenData(%d).IhrBase = %d, want > 0", tc.te, dt.IhrBase)
			}

			if dt.FourHabsOffline <= 0 {
				t.Errorf("createDynamicTokenData(%d).FourHabsOffline = %d, want > 0", tc.te, dt.FourHabsOffline)
			}

			if dt.MaxHab <= 0 {
				t.Errorf("createDynamicTokenData(%d).MaxHab = %f, want > 0", tc.te, dt.MaxHab)
			}

			if dt.ChickenRunHab <= 0 || dt.ChickenRunHab >= dt.MaxHab {
				t.Errorf("createDynamicTokenData(%d).ChickenRunHab = %f, want between 0 and MaxHab(%f)", tc.te, dt.ChickenRunHab, dt.MaxHab)
			}

			if dt.IHRMultiplier <= 0 {
				t.Errorf("createDynamicTokenData(%d).IHRMultiplier = %f, want > 0", tc.te, dt.IHRMultiplier)
			}

			// Validate boost times are populated and increasing
			for i := 0; i < len(dt.TokenBoost); i++ {
				if dt.TokenBoost[i] <= 0 {
					t.Errorf("createDynamicTokenData(%d).TokenBoost[%d] = %f, want > 0", tc.te, i, dt.TokenBoost[i])
				}

				if dt.BoostTimeMinutes[i] <= 0 {
					t.Errorf("createDynamicTokenData(%d).BoostTimeMinutes[%d] = %v, want > 0", tc.te, i, dt.BoostTimeMinutes[i])
				}

				if dt.ChickenRunTimeMinutes[i] <= 0 {
					t.Errorf("createDynamicTokenData(%d).ChickenRunTimeMinutes[%d] = %v, want > 0", tc.te, i, dt.ChickenRunTimeMinutes[i])
				}

				// Chicken run time should be less than boost time (since chicken run hab is less)
				if dt.ChickenRunTimeMinutes[i] >= dt.BoostTimeMinutes[i] {
					t.Errorf("createDynamicTokenData(%d).ChickenRunTimeMinutes[%d]=%v should be less than BoostTimeMinutes[%d]=%v", tc.te, i, dt.ChickenRunTimeMinutes[i], i, dt.BoostTimeMinutes[i])
				}

				// Boost times should increase with more tokens
				if i > 0 && dt.BoostTimeMinutes[i] <= dt.BoostTimeMinutes[i-1] {
					t.Errorf("createDynamicTokenData(%d).BoostTimeMinutes[%d]=%v should be greater than BoostTimeMinutes[%d]=%v", tc.te, i, dt.BoostTimeMinutes[i], i-1, dt.BoostTimeMinutes[i-1])
				}
			}
		})
	}
}
*/

func TestDynamicTokens(t *testing.T) {

	dt := createDynamicTokenData(50)

	// Initially assign 6 token boosts to everyone,
	// In reverse order start calculating using 8 token boosts
	// stop when the 120 minute delivered eggs amount is less than the previous amount
	result := calculateDynamicTokens(dt, 4, 0.39, 0)

	// result should be 4 integers with the the last one being an 8
	if len(result) != 4 {
		t.Errorf("calculateDynamicTokens() = %v, want %v", len(result), 4)
	}

	if result[0] != 6 {
		t.Errorf("calculateDynamicTokens() = %v, want %v", result[0], 6)
	}

	if result[1] != 6 {
		t.Errorf("calculateDynamicTokens() = %v, want %v", result[1], 6)
	}

	if result[2] != 6 {

		t.Errorf("calculateDynamicTokens() = %v, want %v", result[2], 6)
	}

	if result[3] != 8 {

		t.Errorf("calculateDynamicTokens() = %v, want %v", result[3], 8)
	}

}
