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

func TestDynamicTokens(t *testing.T) {

	dt := createDynamicTokenData()

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
