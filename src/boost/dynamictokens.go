package boost

import (
	"log"
	"math"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

// DynamicTokenData is a struct that holds the data needed to calculate dynamic tokens
type DynamicTokenData struct {
	TokenTimer int

	TE                    int64
	HabNumber             int64
	OfflineIHR            int64
	Name                  string
	ELR                   float64
	TokenBoost            [5]int64
	BoostTimeSeconds      [5]time.Duration
	ChickenRunTimeSeconds [5]time.Duration
	IhrBase               int64
	FourHabsOffline       int64
	MaxHab                float64
	ChickenRunHab         float64
	IHRMultiplier         float64
	ColleggtibleIHR       float64
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

// getBoostTimeSeconds returns the boost time as a time.Duration for a given number of tokens
func getBoostTimeSeconds(dt *DynamicTokenData, tokens int) (time.Duration, time.Duration) {
	if tokens < 4 || tokens > 9 {
		log.Printf("Invalid number of tokens: %d. Must be between 4 and 9.", tokens)
		return 0, 0
	}
	// Check if someone is using a 9th token for a soul mirror boost
	if tokens == 9 {
		tokens = 8
	}
	return dt.BoostTimeSeconds[tokens-4], dt.ChickenRunTimeSeconds[tokens-4]
}

// createDynamicTokenData creates all the common underlying data for dynamic tokens
func createDynamicTokenData() *DynamicTokenData {
	dt := new(DynamicTokenData)

	_, _, colleggtibleHab, colleggtiblesIHR := ei.GetColleggtibleValues()
	dt.ColleggtibleIHR = colleggtiblesIHR
	dt.HabNumber = 4
	dt.OfflineIHR = 3
	dt.TE = 50 // Assuming 50 TE

	// Chickens per minute
	// Assumption is that the player has completed Epic and Common Research
	dt.IhrBase = 7440                                            // chickens/min/hab
	dt.IhrBase = int64(float64(dt.IhrBase) * dt.ColleggtibleIHR) // 5% from Easter Colleggtibles
	dt.FourHabsOffline = dt.IhrBase * dt.HabNumber * dt.OfflineIHR

	// 1000x + number of 10x free boosts * multiplier
	dt.TokenBoost[0] = dt.FourHabsOffline * int64(calcBoostMulti(4))
	dt.TokenBoost[1] = dt.FourHabsOffline * int64(calcBoostMulti(5))
	dt.TokenBoost[2] = dt.FourHabsOffline * int64(calcBoostMulti(6))
	dt.TokenBoost[3] = dt.FourHabsOffline * int64(calcBoostMulti(7))
	dt.TokenBoost[4] = dt.FourHabsOffline * int64(calcBoostMulti(8))

	// Assume: T4L Chalice, T4L mono, 6 Life stones
	chickenRunPercent := 0.70 // Chicken run is 70.0% of normal boost time
	dt.IHRMultiplier = 1.4 * 1.25 * math.Pow(1.04, 11.0) * dt.ColleggtibleIHR * math.Pow(1.01, float64(dt.TE))
	dt.MaxHab = 14175000000.0 * colleggtibleHab
	dt.ChickenRunHab = dt.MaxHab * chickenRunPercent
	// Create boost times for 4 through 9 tokens
	for i := 0; i < len(dt.TokenBoost); i++ {
		dt.BoostTimeSeconds[i] = time.Duration(dt.MaxHab / (float64(dt.TokenBoost[i]) * dt.IHRMultiplier) * 60.0 * float64(time.Second))
		dt.ChickenRunTimeSeconds[i] = time.Duration(dt.ChickenRunHab / (float64(dt.TokenBoost[i]) * dt.IHRMultiplier) * 60.0 * float64(time.Second))
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
