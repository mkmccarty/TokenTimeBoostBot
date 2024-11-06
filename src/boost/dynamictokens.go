package boost

import (
	"log"
	"math"
	"time"
)

// DynamicTokenData is a struct that holds the data needed to calculate dynamic tokens
type DynamicTokenData struct {
	TokenTimer int

	HabNumber          int64
	OfflineIHR         int64
	Name               string
	ELR                float64
	TokenBoost         [5]int64
	BoostTimeMinutes   [5]float64
	IhrBase            int64
	FourHabsOffline    int64
	MaxHab             float64
	IHRMultiplier      float64
	ColleggtibleEaster float64
}

// Returns an slice of integers for all contract players with needed token counts
func calculateDynamicTokens(dt *DynamicTokenData, count int, tpm float64, heldTokens int) []int {
	var retSlice []int

	log.Print("Calculating dynamic tokens for ", dt.Name, " for ", count, " players")

	log.Print("TPM: ", tpm)
	log.Print("Held Tokens: ", heldTokens)

	// Want the highest eggs at 120 minutes
	//eggsAt120 := boostedELR * (120 - timeToFinishBoostInMin) / 60

	// Data needed...
	// Time of start of contract
	// For Boosted - time of player boosts and tokens spent.
	// For Unboosted -  zero boost time and count of tokens on hand

	// 1. Get fixed values from those that already boosted
	// 2. Calculate the estimates using mix of token boosts

	// Add 6, 6, 6, 8 to retSlice
	retSlice = append(retSlice, 6, 6, 6, 8)

	// For every booster
	return retSlice
}

// createDynamicTokenData creates all the common underlying data for dynamic tokens
func createDynamicTokenData() *DynamicTokenData {
	dt := new(DynamicTokenData)

	dt.HabNumber = 4
	dt.OfflineIHR = 3

	// Chickens per minute
	// Assumption is that the player has completed Epic and Common Research
	dt.IhrBase = 7440 // chickens/min/hab
	dt.FourHabsOffline = dt.IhrBase * dt.HabNumber * dt.OfflineIHR

	// 1000x + number of 10x free boosts * multiplier
	dt.TokenBoost[0] = dt.FourHabsOffline * (1000 + 4*10)
	dt.TokenBoost[1] = dt.FourHabsOffline * (1000 + 3*10) * 2
	dt.TokenBoost[2] = dt.FourHabsOffline * (1000 + 2*10) * 4
	dt.TokenBoost[3] = dt.FourHabsOffline * (1000 + 1*10) * 6
	dt.TokenBoost[4] = dt.FourHabsOffline * (1000 + 0*10) * 10

	// Assume: T4L Chalice, T4L mono, 6 Life stones

	dt.ColleggtibleEaster = 1.05
	dt.IHRMultiplier = 1.4 * 1.25 * math.Pow(1.04, 11.0) * dt.ColleggtibleEaster
	dt.MaxHab = 14175000000.0
	// Create boost times for 4 through 9 tokens
	for i := 0; i < len(dt.TokenBoost); i++ {
		dt.BoostTimeMinutes[i] = dt.MaxHab / (float64(dt.TokenBoost[i]) * dt.IHRMultiplier)
	}
	return dt
}

func determineDynamicTokens(c *Contract) {

	if c == nil {
		return
	}

	if c.Style&ContractFlagDynamicTokens == 0 {
		return
	}
	// Determine the dynamic tokens
	// For everyone in the contract
	// 1. Determine boosted ELR
	// 2. Determine start boost time ane end boost time based on tokens IHR
	// 3. Determine how much fully boosted minutes at ELR (120 - minutes to boost)
	dt := createDynamicTokenData()

	// Initially assign 6 token boosts to everyone,
	// In reverse order start calculating using 8 token boosts
	// stop when the 120 minute delivered eggs amount is less than the previous amount

	tpm := float64(len(c.TokenLog)) / time.Since(c.StartTime).Minutes()

	calculateDynamicTokens(dt, 4, tpm, 0)

}
