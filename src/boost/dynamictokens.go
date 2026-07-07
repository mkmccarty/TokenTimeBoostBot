package boost

import (
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
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
	for i := range len(dt.TokenBoost) {
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

func getMonocleMultiplier(artifact *ei.CompleteArtifact) float64 {
	spec := artifact.GetSpec()
	if spec == nil || spec.GetName() != ei.ArtifactSpec_DILITHIUM_MONOCLE {
		return 1.0
	}
	level := spec.GetLevel()
	rarity := spec.GetRarity()
	switch level {
	case ei.ArtifactSpec_INFERIOR:
		return 1.05
	case ei.ArtifactSpec_LESSER:
		return 1.10
	case ei.ArtifactSpec_NORMAL:
		return 1.15
	case ei.ArtifactSpec_GREATER:
		switch rarity {
		case ei.ArtifactSpec_COMMON:
			return 1.20
		case ei.ArtifactSpec_RARE:
			return 1.22
		case ei.ArtifactSpec_EPIC:
			return 1.25
		case ei.ArtifactSpec_LEGENDARY:
			return 1.30
		}
	}
	return 1.0
}

func findContributor(contract *Contract, booster *Booster, coopStatus *ei.ContractCoopStatusResponse) *ei.ContractCoopStatusResponse_ContributionInfo {
	if coopStatus == nil {
		return nil
	}
	for _, contributor := range coopStatus.GetContributors() {
		coopName := contributor.GetUserName()
		if coopName == "" {
			coopName = contributor.GetUserId()
		}
		if coopName == "" {
			continue
		}
		// Exact database lookup by name to see if it matches booster
		if discordID, err := farmerstate.GetDiscordUserIDFromEiIgn(coopName); err == nil && discordID == booster.UserID {
			return contributor
		}
		// Fallbacks
		if strings.EqualFold(coopName, booster.Nick) || strings.EqualFold(coopName, booster.Name) {
			return contributor
		}
	}
	return nil
}

func createCorrectedDynamicTokenData(TE int64, contributor *ei.ContractCoopStatusResponse_ContributionInfo) *DynamicTokenData {
	dt := createDynamicTokenData(TE)
	if contributor == nil || contributor.GetFarmInfo() == nil {
		return dt
	}
	farmInfo := contributor.GetFarmInfo()
	buffs := ei.GetArtifactBuffs(farmInfo.EquippedArtifacts)
	_, _, _, colleggtiblesIHR := ei.GetColleggtibleValues()

	dummyGame := &ei.Backup_Game{EpicResearch: farmInfo.EpicResearch}
	_, _, _, actualOfflineRate := ei.GetInternalHatcheryFromBackup(farmInfo.CommonResearch, dummyGame, buffs.IHR*colleggtiblesIHR, uint32(farmInfo.GetEggsOfProphecy()))

	actualMonocle := 1.0
	for _, art := range farmInfo.EquippedArtifacts {
		if art.GetSpec().GetName() == ei.ArtifactSpec_DILITHIUM_MONOCLE {
			actualMonocle = getMonocleMultiplier(art)
			break
		}
	}

	actualMaxHab := 0.0
	for _, cap := range farmInfo.HabCapacity {
		actualMaxHab += float64(cap)
	}
	if actualMaxHab == 0 {
		_, _, colleggtibleHab, _ := ei.GetColleggtibleValues()
		actualMaxHab = 14_175_000_000.0 * colleggtibleHab * buffs.Hab
	}

	dt.IhrBase = int64(actualOfflineRate / 4 / float64(dt.OfflineIHR))
	dt.FourHabsOffline = int64(actualOfflineRate)
	dt.IHRMultiplier = 1.0 // already factored into FourHabsOffline
	dt.MaxHab = actualMaxHab
	dt.ChickenRunHab = actualMaxHab * 0.70

	for i := range len(dt.TokenBoost) {
		mult := calcBoostMulti(float64(i))
		dt.TokenBoost[i] = mult * actualMonocle
		ihr := float64(dt.TokenBoost[i]) * float64(dt.FourHabsOffline) // per minute
		dt.BoostTimeMinutes[i] = max(1.0, float64(dt.MaxHab)/ihr)
		dt.ChickenRunTimeMinutes[i] = max(1.0, float64(dt.ChickenRunHab)/ihr)
	}

	return dt
}

// setEstimatedBoostTimings calculates and sets the estimated boost timings on a booster
func setEstimatedBoostTimings(contract *Contract, booster *Booster, coopStatus *ei.ContractCoopStatusResponse) {
	contributor := findContributor(contract, booster, coopStatus)
	var dt *DynamicTokenData
	if contributor != nil {
		dt = createCorrectedDynamicTokenData(int64(booster.TECount), contributor)
	} else {
		dt = createDynamicTokenData(int64(booster.TECount))
	}

	if dt != nil {
		slopTime := 20 * time.Second
		wiggleRoom := time.Duration(slopTime)
		boostDuration, chickenRunDuration := getBoostTimeSeconds(dt, booster.TokensWanted)
		bonusStep := 220 * time.Second
		extraBoost := time.Duration(boostDuration/bonusStep) * slopTime
		totalBoostDuration := boostDuration + extraBoost
		booster.EstDurationOfBoost = totalBoostDuration

		baseTime := time.Now()
		if !booster.StartTime.IsZero() {
			baseTime = booster.StartTime
		}
		booster.EstEndOfBoost = baseTime.Add(totalBoostDuration).Add(wiggleRoom)
		booster.EstRequestChickenRuns = baseTime.Add(chickenRunDuration).Add(wiggleRoom)
	}
}

// UpdateAllEstimatedBoostTimings updates estimated timings for all boosters in the contract
func UpdateAllEstimatedBoostTimings(contract *Contract, coopStatus *ei.ContractCoopStatusResponse) {
	for _, b := range contract.Boosters {
		setEstimatedBoostTimings(contract, b, coopStatus)
	}
}

// GetLiveCSScoreEstimate calculates and returns the projected contract score for a booster
func GetLiveCSScoreEstimate(contract *Contract, booster *Booster) string {
	if contract.LatestCoopStatus == nil {
		return ""
	}
	coopStatus := contract.LatestCoopStatus
	if coopStatus.GetResponseStatus() != ei.ContractCoopStatusResponse_NO_ERROR {
		return ""
	}
	contributor := findContributor(contract, booster, coopStatus)
	if contributor == nil {
		return ""
	}

	secondsRemaining := float64(coopStatus.GetSecondsRemaining())
	nowTime := time.Now()
	eiContract, ok := ei.GetEggIncContract(contract.ContractID)
	if !ok {
		return ""
	}
	grade := int(coopStatus.GetGrade())
	if grade < 0 || grade >= len(eiContract.Grade) {
		return ""
	}

	startTime := nowTime.Add(time.Duration(secondsRemaining) * time.Second).Add(-time.Duration(eiContract.Grade[grade].LengthInSeconds) * time.Second)

	var totalContributions float64
	var contributionRatePerSecond float64
	for _, c := range coopStatus.GetContributors() {
		totalContributions += c.GetContributionAmount()
		totalContributions += -(c.GetContributionRate() * c.GetFarmInfo().GetTimestamp())
		contributionRatePerSecond += c.GetContributionRate()
	}

	targetGoal := eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1]
	calcSecondsRemaining := secondsRemaining
	if contributionRatePerSecond > 0 {
		calcSecondsRemaining = (targetGoal - totalContributions) / contributionRatePerSecond
		if calcSecondsRemaining < 0 {
			calcSecondsRemaining = 0
		}
	}
	endTime := nowTime.Add(time.Duration(calcSecondsRemaining) * time.Second)
	contractDurationSeconds := endTime.Sub(startTime).Seconds()

	var projectedContribution float64
	if booster.BoostState == BoostStateBoosted {
		projectedContribution = contributor.GetContributionAmount() + contributor.GetContributionRate()*calcSecondsRemaining
	} else {
		dt := createCorrectedDynamicTokenData(int64(booster.TECount), contributor)

		var chaliceQuality, monocleQuality string
		var gussetQuality, compassQuality, metronomeQuality string
		for _, art := range booster.ArtifactSet.Artifacts {
			switch art.Type {
			case "Chalice":
				chaliceQuality = art.Quality
			case "Monocle":
				monocleQuality = art.Quality
			case "Gusset":
				gussetQuality = art.Quality
			case "Compass":
				compassQuality = art.Quality
			case "Metronome":
				metronomeQuality = art.Quality
			}
		}

		chaliceMap := map[string]float64{
			"T1C": 1.05,
			"T2C": 1.10, "T2E": 1.15,
			"T3C": 1.20, "T3R": 1.23, "T3E": 1.25,
			"T4C": 1.30, "T4E": 1.35, "T4L": 1.40,
		}
		monocleMap := map[string]float64{
			"T1C": 1.05,
			"T2C": 1.10,
			"T3C": 1.15,
			"T4C": 1.20, "T4R": 1.22, "T4E": 1.25, "T4L": 1.30,
		}
		gussetMap := map[string]float64{
			"T1C": 1.05,
			"T2C": 1.10, "T2E": 1.12,
			"T3C": 1.15, "T3R": 1.16,
			"T4C": 1.20, "T4E": 1.22, "T4L": 1.25,
		}
		compassMap := map[string]float64{
			"T1C": 1.05,
			"T2C": 1.10,
			"T3C": 1.20, "T3R": 1.22,
			"T4C": 1.30, "T4R": 1.35, "T4E": 1.40, "T4L": 1.50,
		}
		metronomeMap := map[string]float64{
			"T1C": 1.05,
			"T2C": 1.10, "T2R": 1.12,
			"T3C": 1.15, "T3R": 1.17, "T3E": 1.20,
			"T4C": 1.25, "T4R": 1.27, "T4E": 1.30, "T4L": 1.35,
		}

		farmInfo := contributor.GetFarmInfo()

		hasChalice := false
		hasMonocle := false
		if farmInfo != nil {
			for _, art := range farmInfo.EquippedArtifacts {
				if art.GetSpec().GetName() == ei.ArtifactSpec_THE_CHALICE {
					hasChalice = true
				}
				if art.GetSpec().GetName() == ei.ArtifactSpec_DILITHIUM_MONOCLE {
					hasMonocle = true
				}
			}
		}

		actualIHRMult := 1.0
		if !hasChalice && chaliceQuality != "" {
			if m, ok := chaliceMap[chaliceQuality]; ok {
				actualIHRMult *= m
			}
		}
		actualMonocleMult := 1.0
		if !hasMonocle && monocleQuality != "" {
			if m, ok := monocleMap[monocleQuality]; ok {
				actualMonocleMult = m
			}
		}

		if actualIHRMult > 1.0 || actualMonocleMult > 1.0 {
			buffs := ei.GetArtifactBuffs(farmInfo.EquippedArtifacts)
			_, _, _, colleggtiblesIHR := ei.GetColleggtibleValues()
			dummyGame := &ei.Backup_Game{EpicResearch: farmInfo.EpicResearch}
			_, _, _, baseOfflineRate := ei.GetInternalHatcheryFromBackup(farmInfo.CommonResearch, dummyGame, buffs.IHR*colleggtiblesIHR, uint32(farmInfo.GetEggsOfProphecy()))

			if !hasChalice {
				baseOfflineRate *= actualIHRMult
			}
			dt.FourHabsOffline = int64(baseOfflineRate)

			monocleToUse := actualMonocleMult
			if hasMonocle {
				for _, art := range farmInfo.EquippedArtifacts {
					if art.GetSpec().GetName() == ei.ArtifactSpec_DILITHIUM_MONOCLE {
						monocleToUse = getMonocleMultiplier(art)
						break
					}
				}
			}
			for i := range len(dt.TokenBoost) {
				mult := calcBoostMulti(float64(i))
				dt.TokenBoost[i] = mult * monocleToUse
				ihr := float64(dt.TokenBoost[i]) * float64(dt.FourHabsOffline)
				dt.BoostTimeMinutes[i] = max(1.0, float64(dt.MaxHab)/ihr)
				dt.ChickenRunTimeMinutes[i] = max(1.0, float64(dt.ChickenRunHab)/ihr)
			}
		}

		hasMetronome := false
		hasCompass := false
		hasGusset := false
		if farmInfo != nil {
			for _, art := range farmInfo.EquippedArtifacts {
				if art.GetSpec().GetName() == ei.ArtifactSpec_QUANTUM_METRONOME {
					hasMetronome = true
				}
				if art.GetSpec().GetName() == ei.ArtifactSpec_INTERSTELLAR_COMPASS {
					hasCompass = true
				}
				if art.GetSpec().GetName() == ei.ArtifactSpec_ORNATE_GUSSET {
					hasGusset = true
				}
			}
		}

		pp := contributor.GetProductionParams()
		layingRatePerSecondPerChicken := pp.GetElr() / 3600
		shippingRatePerSecond := pp.GetSr() / 3600

		if !hasMetronome && metronomeQuality != "" {
			if m, ok := metronomeMap[metronomeQuality]; ok {
				layingRatePerSecondPerChicken *= m
			}
		}
		if !hasCompass && compassQuality != "" {
			if m, ok := compassMap[compassQuality]; ok {
				shippingRatePerSecond *= m
			}
		}
		if !hasGusset && gussetQuality != "" {
			if m, ok := gussetMap[gussetQuality]; ok {
				dt.MaxHab *= m
				dt.ChickenRunHab = dt.MaxHab * 0.70
			}
		}

		tokens := booster.TokensWanted
		boostTimeMinutes := dt.BoostTimeMinutes[tokens]

		currentPop := 0.0
		if farmInfo != nil {
			for _, pop := range farmInfo.HabPopulation {
				currentPop += float64(pop)
			}
		}

		boostedRate := math.Min(dt.MaxHab*layingRatePerSecondPerChicken, shippingRatePerSecond)
		remMinutes := calcSecondsRemaining / 60
		if remMinutes <= boostTimeMinutes {
			avgPop := (currentPop + dt.MaxHab) / 2
			layingRate := math.Min(avgPop*layingRatePerSecondPerChicken, shippingRatePerSecond)
			projectedContribution = contributor.GetContributionAmount() + layingRate*calcSecondsRemaining
		} else {
			contribDuringBoost := math.Min((currentPop+dt.MaxHab)/2*layingRatePerSecondPerChicken, shippingRatePerSecond) * boostTimeMinutes * 60
			contribAfterBoost := boostedRate * (remMinutes - boostTimeMinutes) * 60
			projectedContribution = contributor.GetContributionAmount() + contribDuringBoost + contribAfterBoost
		}
	}

	durationDays := max(1, int(math.Ceil(float64(eiContract.Grade[grade].LengthInSeconds)/86400.0)))
	CR := calculateChickenRunTeamwork(
		eiContract.SeasonalScoring,
		eiContract.MaxCoopSize,
		durationDays,
		eiContract.ChickenRuns,
	)
	T := 0.0

	type buffTimeVal struct {
		timeEquiped     float64
		durationEquiped float64
		eggRate         int
		earnings        int
	}
	var buffsList []buffTimeVal
	for _, a := range contributor.GetBuffHistory() {
		earnings := int(math.Round(a.GetEarnings()*100 - 100))
		eggRate := int(math.Round(a.GetEggLayingRate()*100 - 100))
		equipTimestamp := contractDurationSeconds - (a.GetServerTimestamp() + calcSecondsRemaining)
		buffsList = append(buffsList, buffTimeVal{
			timeEquiped: equipTimestamp,
			eggRate:     eggRate,
			earnings:    earnings,
		})
	}

	var buffTimeValue float64
	for i, b := range buffsList {
		var dur float64
		if i == len(buffsList)-1 {
			dur = contractDurationSeconds - b.timeEquiped
		} else {
			dur = buffsList[i+1].timeEquiped - b.timeEquiped
		}
		segmentBuffTimeValue := calculateBuffTimeValue(int(eiContract.SeasonalScoring), dur, b.eggRate, b.earnings)
		buffTimeValue += segmentBuffTimeValue
	}
	B := calculateTeamworkB(buffTimeValue, contractDurationSeconds)

	score, _ := calculateSRScores(
		eiContract.SeasonalScoring,
		grade,
		eiContract.MaxCoopSize,
		eiContract.Grade[grade].LengthInSeconds,
		targetGoal,
		contractDurationSeconds,
		CR,
		T,
		projectedContribution,
		B,
	)

	return fmt.Sprintf(" CS:%d", score)
}
