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
	TokenBoost            [13]float64
	BoostTimeMinutes      [13]float64
	ChickenRunTimeMinutes [13]float64
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

	// This protects the parameters of the next function call
	if tokens < 0 {
		tokens = 0
	} else if tokens > len(dt.BoostTimeMinutes) {
		tokens = len(dt.BoostTimeMinutes) - 1
	}
	return time.Duration(dt.BoostTimeMinutes[tokens] * float64(time.Minute)), time.Duration(dt.ChickenRunTimeMinutes[tokens] * float64(time.Minute))
}

// createDynamicTokenData creates all the common underlying data for dynamic tokens
func createDynamicTokenData(TE int64) *DynamicTokenData {
	dt := new(DynamicTokenData)

	_, _, colleggtibleHab, colleggtiblesIHR := ei.GetColleggtibleValues()
	dt.ColleggtibleIHR = colleggtiblesIHR
	dt.HabNumber = 4
	dt.OfflineIHR = 3
	dt.TE = TE

	// Chickens per minute
	// Assumption is that the player has completed Epic and Common Research
	dt.IhrBase = 7440                                            // chickens/min/hab (without any artifacts)
	dt.IhrBase = int64(float64(dt.IhrBase) * dt.ColleggtibleIHR) // Apply colleggibles (5% from Easter Colleggtibles)
	dt.FourHabsOffline = dt.IhrBase * dt.HabNumber * dt.OfflineIHR

	// Assume: T4L Chalice (1.4), T4L Monocle (1.3), 9 Life stones (IHR stones = 1.04^9 ≈ 1.432)
	// IHR multiplier should NOT reapply colleggibles - those are already in IhrBase
	chickenRunPercent := 0.70 // Chicken run is 70.0% of normal boost time
	chaliceMultiplier := 1.4  // T4L Chalice
	monocleMultiplier := 1.3  // T4L Monocle, only for boosts
	ihrSlots := 9.0           // IHR stone slots
	dt.IHRMultiplier = chaliceMultiplier * math.Pow(1.04, ihrSlots) * math.Pow(1.01, float64(dt.TE))
	dt.MaxHab = 14_175_000_000.0 * colleggtibleHab
	dt.ChickenRunHab = dt.MaxHab * chickenRunPercent
	// Create boost times for 0 through 12 token boosts
	// 14.825K/min/hab (×1.993)
	for i := 0; i < len(dt.TokenBoost); i++ {
		mult := calcBoostMulti(float64(i))
		dt.TokenBoost[i] = mult * monocleMultiplier
		ihr := float64(dt.TokenBoost[i]) * dt.IHRMultiplier * float64(dt.FourHabsOffline) // per minute
		// Minimum time is 1 minute due to away time calculation
		dt.BoostTimeMinutes[i] = max(1.0, float64(dt.MaxHab)/ihr)
		dt.ChickenRunTimeMinutes[i] = max(1.0, float64(dt.ChickenRunHab)/ihr)
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
	dt := createDynamicTokenData(50)

	// Initially assign 6 token boosts to everyone,
	// In reverse order start calculating using 8 token boosts
	// stop when the 120 minute delivered eggs amount is less than the previous amount

	tpm := float64(len(c.TokenLog)) / time.Since(c.StartTime).Minutes()

	calculateDynamicTokens(dt, 4, tpm, 0)

}

// setEstimatedBoostTimings calculates and sets the estimated boost timings on a booster
func setEstimatedBoostTimings(booster *Booster) {
	// These fields are for the dynamic token assignment, per user based on TE
	dt := createDynamicTokenData(int64(booster.TECount))

	if dt != nil {
		wiggleRoom := time.Duration(20 * time.Second) // Add 20 seconds of slop
		boostDuration, chickenRunDuration := getBoostTimeSeconds(dt, booster.TokensWanted)
		bonusStep := 220 * time.Second   // 3m40s per step
		bonusPerStep := 20 * time.Second // add 20s for each step
		extraBoost := time.Duration(boostDuration/bonusStep) * bonusPerStep
		totalBoostDuration := boostDuration + extraBoost
		booster.EstDurationOfBoost = totalBoostDuration
		booster.EstEndOfBoost = time.Now().Add(totalBoostDuration).Add(wiggleRoom)
		booster.EstRequestChickenRuns = time.Now().Add(chickenRunDuration).Add(wiggleRoom)
	}
}
