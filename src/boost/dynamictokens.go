package boost

import (
	"math"
	"time"
)

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
	type DynamicTokenData struct {
		HabNumber        int
		OfflineIHR       int
		Name             string
		ELR              float64
		TokenBoost       [5]int64
		BoostTimeMinutes [5]time.Duration
	}

	var dt DynamicTokenData

	dt.HabNumber = 4
	dt.OfflineIHR = 3

	// Chickens per minute
	ihrBase := 7440 // chickens/min/hab
	fourHabsOffline := int64(ihrBase * dt.HabNumber * dt.OfflineIHR)

	// 1000x + number of 10x free boosts * multiplier
	dt.TokenBoost[0] = fourHabsOffline * (1000 + 4*10)
	dt.TokenBoost[1] = fourHabsOffline * (1000 + 3*10) * 2
	dt.TokenBoost[2] = fourHabsOffline * (1000 + 2*10) * 4
	dt.TokenBoost[3] = fourHabsOffline * (1000 + 1*10) * 6
	dt.TokenBoost[4] = fourHabsOffline * (1000 + 0*10) * 10

	// Assume: T4L Chalice, T4L mono, 6 Life stones
	IHRMultiplier := 1.4 * 1.3 * math.Pow(1.04, 6.0)
	MaxHab := 14175000000.0
	// Loop from 0 through 9
	for i := 0; i < len(dt.TokenBoost); i++ {
		dt.BoostTimeMinutes[i] = time.Duration(MaxHab/float64(dt.TokenBoost[i])*IHRMultiplier) * time.Minute
	}

}
