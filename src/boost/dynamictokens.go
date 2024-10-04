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
		HABS        int
		OFFLINE_IHR int
		Name        string
		ELR         float64
	}

	var dt DynamicTokenData

	dt.HABS = 4
	dt.OFFLINE_IHR = 3

	// Chickens per minute
	ihrBase := 7440 // chickens/min/hab
	fourHabsOffline := ihrBase * dt.HABS * dt.OFFLINE_IHR

	// 1000x + number of 10x free boosts * multiplier
	var TokenBoost [9]int64
	TokenBoost[0] := 0
	TokenBoost[1] := 0
	TokenBoost[2] := 0
	TokenBoost[3] := 0
	TokenBoost[4] := fourHabsOffline * (1000 + 4*10)
	TokenBoost[5] := fourHabsOffline * (1000 + 3*10) * 2
	TokenBoost[6] := fourHabsOffline * (1000 + 2*10) * 4
	TokenBoost[7] := fourHabsOffline * (1000 + 1*10) * 6
	TokenBoost[8] := fourHabsOffline * (1000 + 0*10) * 10

	// Assume: T4L Chalice, T4L mono, 6 Life stones
	IHRMultiplier := 1.4 * 1.3 * math.Pow(1.04, 6.0)
	MaxHab := 14175000000.0
	var BoostTimeMinutes [9]time.Duration
	// Loop from 0 through 9
	for i := 0; i < 10; i++ {
		if TokenBoost[i] == 0 {
			continue
		}
		BoostTimeMinutes[i] := time.Duration(MaxHab/float64(TokenBoost[i])*IHRMultiplier) * time.Minute
	}

}
