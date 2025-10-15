package ei

import (
	"log"
	"math"
	"time"
)

// GetVehiclesShippingCapacity calculates the total shipping capacity of the user's vehicles
func GetVehiclesShippingCapacity(vehicles []uint32, trainLength []uint32, univMult float64, hoverOnlyMult float64, hyperOnlyMult float64) (float64, string) {
	userShippingCap := 0.0
	shippingNote := ""
	fullyUpgraded := true

	for i, v := range vehicles {
		vehicleType := vehicleTypes[v]
		capacity := vehicleType.BaseCapacity
		if vehicleType.ID != 11 && trainLength[i] != 10 {
			fullyUpgraded = false
		}
		if isHoverVehicle(vehicleType) {
			capacity *= hoverOnlyMult
		}
		capacity *= univMult
		if isHyperloop(vehicleType) {
			capacity *= hyperOnlyMult
			if trainLength[i] > 0 {
				lengthOfOneTrain := trainLength[i]
				capacity *= float64(lengthOfOneTrain)
			}
		}

		userShippingCap += capacity

	}
	if !fullyUpgraded {
		shippingNote = "Vehicles not fully upgraded"
	}

	return userShippingCap, shippingNote
}

// GetEggLayingRate calculates the egg laying rate multiplier
func GetEggLayingRate(farmInfo *PlayerFarmInfo) float64 {
	userLayRate := 1 / 30.0 // 1 chicken per 30 seconds

	userLayRate *= GetCommonResearchLayRate(farmInfo.GetCommonResearch())
	userLayRate *= GetEpicResearchLayRate(farmInfo.GetEpicResearch())

	universalHabCapacity := GetCommonResearchHabCapacity(farmInfo.GetCommonResearch())
	portalHabCapacity := GetCommonResearchPortalHabCapacity(farmInfo.GetCommonResearch())

	//userLayRate *= 3600 // convert to hr rate
	habPopulation := 0.0
	for _, hab := range farmInfo.GetHabPopulation() {
		habPopulation += float64(hab)
	}
	habCapacity := 0.0
	for _, hab := range farmInfo.GetHabCapacity() {
		habCapacity += float64(hab)
	}

	baseHab := 0.0
	for _, hab := range farmInfo.GetHabs() {
		// Values 1->18 for each of these
		value := 0.0
		if hab != 19 {
			value = float64(Habs[hab].BaseCapacity)
			if IsPortalHab(Habs[hab]) {
				value *= portalHabCapacity
			}
			value *= universalHabCapacity
		}
		baseHab += value
	}

	//userLayRate *= 3600 // convert to hr rate
	baseLayingRate := userLayRate * baseHab * 3600.0
	//as.baseLayingRate = userLayRate * min(habPopulation, as.baseHab) * 3600.0 / 1e15

	return baseLayingRate
}

// GetEggLayingRateFromBackup calculates the egg laying rate multiplier
func GetEggLayingRateFromBackup(farmInfo *Backup_Simulation, game *Backup_Game) (float64, float64, float64) {
	userLayRate := 1 / 30.0 // 1 chicken per 30 seconds

	userLayRate *= GetCommonResearchLayRate(farmInfo.GetCommonResearch())
	userLayRate *= GetEpicResearchLayRate(game.GetEpicResearch())

	universalHabCapacity := GetCommonResearchHabCapacity(farmInfo.GetCommonResearch())
	portalHabCapacity := GetCommonResearchPortalHabCapacity(farmInfo.GetCommonResearch())

	habPopulation := 0.0
	for _, hab := range farmInfo.GetHabPopulation() {
		habPopulation += float64(hab)
	}

	habCapacity := 0.0
	for _, hab := range farmInfo.GetHabs() {
		// Values 1->18 for each of these
		value := 0.0
		if hab != 19 {
			value = float64(Habs[hab].BaseCapacity)
			if IsPortalHab(Habs[hab]) {
				value *= portalHabCapacity
			}
			value *= universalHabCapacity
		}
		habCapacity += value
	}

	//userLayRate *= 3600 // convert to hr rate
	baseLayingRate := userLayRate * habPopulation * 3600.0
	//as.baseLayingRate = userLayRate * min(habPopulation, as.baseHab) * 3600.0 / 1e15

	return baseLayingRate, habPopulation, habCapacity
}

// GetShippingRate calculates the shipping rate multiplier
func GetShippingRate(farmInfo *PlayerFarmInfo) float64 {
	universalShippingMultiplier := 1.0

	universalShippingMultiplier *= GetCommonResearchShippingRate(farmInfo.GetCommonResearch())
	universalShippingMultiplier *= GetEpicResearchShippingRate(farmInfo.GetEpicResearch())

	hoverOnlyMultiplier := GetCommonResearchHoverOnlyMultiplier(farmInfo.GetCommonResearch())
	hyperloopOnlyMultiplier := GetCommonResearchHyperloopOnlyMultiplier(farmInfo.GetCommonResearch())

	userShippingRate, _ := GetVehiclesShippingCapacity(farmInfo.GetVehicles(), farmInfo.GetTrainLength(), universalShippingMultiplier, hoverOnlyMultiplier, hyperloopOnlyMultiplier)

	return userShippingRate * 60
}

// GetShippingRateFromBackup calculates the shipping rate multiplier
func GetShippingRateFromBackup(farmInfo *Backup_Simulation, game *Backup_Game) float64 {
	universalShippingMultiplier := 1.0

	universalShippingMultiplier *= GetCommonResearchShippingRate(farmInfo.GetCommonResearch())
	universalShippingMultiplier *= GetEpicResearchShippingRate(game.GetEpicResearch())

	hoverOnlyMultiplier := GetCommonResearchHoverOnlyMultiplier(farmInfo.GetCommonResearch())
	hyperloopOnlyMultiplier := GetCommonResearchHyperloopOnlyMultiplier(farmInfo.GetCommonResearch())

	userShippingRate, _ := GetVehiclesShippingCapacity(farmInfo.GetVehicles(), farmInfo.GetTrainLength(), universalShippingMultiplier, hoverOnlyMultiplier, hyperloopOnlyMultiplier)

	return userShippingRate * 60
}

// GetInternalHatcheryFromBackup calculates the internal hatchery rate multiplier
func GetInternalHatcheryFromBackup(commonResearch []*Backup_ResearchItem, game *Backup_Game, modifier float64, truthEggs uint32) (float64, float64, float64, float64) {

	baseRate := 0.0

	_, _, hatcheryAdditive := GetResearchInternalHatchery(commonResearch)
	onlineMultiplier, offlineMultiplier, _ := GetResearchInternalHatchery(game.GetEpicResearch())

	baseRate += hatcheryAdditive

	truthEggBonus := math.Pow(1.1, float64(truthEggs)) // 10% per truth egg

	// With max internal hatchery sharing, four internal hatcheries are constantly
	// at work even if not all habs are bought;
	onlineRatePerHab := baseRate * onlineMultiplier * modifier * truthEggBonus
	onlineRate := 4 * onlineRatePerHab
	offlineRatePerHab := onlineRatePerHab * offlineMultiplier
	offlineRate := onlineRate * offlineMultiplier

	return onlineRatePerHab, onlineRate, offlineRatePerHab, offlineRate
}

// TimeForLinearGrowth calculates the time required to reach a target population
// with a constant, absolute growth rate.
//
// Arguments:
//
//	initialPopulation: The starting population.
//	targetPopulation: The desired population.
//	growthRate: The fixed number of people added per unit of time.
//
// Returns:
//
//	The time in units consistent with the growth rate (e.g., minutes).
func TimeForLinearGrowth(initialPopulation, targetPopulation, growthRate float64) float64 {
	// Panic if inputs are invalid to prevent non-sensical calculations.
	if growthRate <= 0 {
		panic("Growth rate must be a positive number.")
	}

	// If the target is not larger than the initial population, no time has passed.
	if targetPopulation <= initialPopulation {
		return 0
	}

	// Calculate the difference between the target and initial populations.
	populationDifference := targetPopulation - initialPopulation

	// Divide the difference by the growth rate to get the time.
	time := populationDifference / growthRate
	return time
}

// TimeToDeliverEggs simulates population growth and egg production to find the time
// required to deliver a certain number of eggs.
//
// Arguments:
//
//	initialPop: The starting chicken population.
//	maxPop: The maximum carrying capacity of the population.
//	growthRatePerMinute: The growth rate of the population per minute.
//	layingRatePerHour: The number of eggs laid per chicken per hour.
//	shippingRatePerHour: The constant number of eggs shipped per hour.
//	targetEggs: The total number of eggs to deliver.
//
// Returns:
//
//	The time in seconds, or -1 if the target is unreachable.
func TimeToDeliverEggs(initialPop, maxPop, growthRatePerMinute, layingRatePerHour, shippingRatePerHour, targetEggs float64) float64 {
	// Validate inputs
	if initialPop <= 0 || maxPop <= 0 || growthRatePerMinute <= 0 || layingRatePerHour <= 0 || targetEggs <= 0 {
		return -1.0 // Return -1 for invalid inputs
	}

	// Convert rates to be consistent with the 5-minute time step
	timeStepMinutes := 1.0
	layingRatePerStep := (layingRatePerHour / 60) * timeStepMinutes
	shippingRatePerStep := (shippingRatePerHour / 60) * timeStepMinutes
	growthRatePerStep := growthRatePerMinute * timeStepMinutes

	// Check if the shipping rate is sustainable
	//if layingRatePerHour*maxPop < shippingRatePerHour {
	//	fmt.Println("Warning: Shipping rate is higher than the maximum possible laying rate. The target may never be reached.")
	//}

	totalTimeMinutes := 0.0
	totalEggsDelivered := 0.0
	currentPop := initialPop

	// Loop until the target number of eggs is delivered
	for totalEggsDelivered < targetEggs {
		// Calculate the number of eggs laid in this time step
		deliveryRate := math.Min(layingRatePerStep, shippingRatePerStep)
		eggsToDeliverThisStep := deliveryRate //* currentPop

		// Calculate the eggs delivered in this time step (limited by the eggs available)
		//eggsToDeliverThisStep := math.Min(eggsLaidInStep, shippingRatePerStep)

		// Update total eggs delivered
		totalEggsDelivered += eggsToDeliverThisStep

		// Update the population for the next time step using the logistical growth formula.
		if currentPop <= maxPop {
			oldPop := currentPop
			currentPop += growthRatePerStep
			if currentPop > maxPop {
				currentPop = maxPop
			}
			// want % of pop increase so we can increase the delivery rate
			popIncrease := currentPop - oldPop
			layingRatePerStep *= (1 + popIncrease/oldPop)
		}

		// Increment time
		totalTimeMinutes += timeStepMinutes

		// Safety break to prevent infinite loops if the target is unreachable.
		if totalTimeMinutes > 525_600 { // 1 year in minutes
			return -1.0
		}
	}

	return totalTimeMinutes * 60.0
}

// GetFarmEarningRates calculates the farm earning rates
func GetFarmEarningRates(backup *Backup, deliveryRate float64, artBuffs DimensionBuffs, colBuffs DimensionBuffs, eov uint32) (float64, float64) {
	eggValue := GetFarmEggValue(backup.GetFarms()[0].GetCommonResearch())
	earningBonus := math.Pow(1.1, float64(eov)) // 10% per egg of virtue
	currentMultiplier := backup.GetGame().GetCurrentMultiplier()
	expiration := time.Unix(int64(backup.GetGame().GetCurrentMultiplierExpiration()), 0)
	eventMultipler := currentEarningsEvent
	// Maybe Ultra Check
	if currentMultiplier != 1.0 && expiration.Before(time.Now()) {
		eventMultipler *= currentEarningsEventUltra
	}

	log.Printf("eggValue: %v\n", eggValue)
	log.Printf("deliveryRate: %v\n", deliveryRate)
	log.Printf("earningBonus: %v\n", earningBonus)
	log.Printf("artBuffs.Earnings: %v\n", artBuffs.Earnings)
	log.Printf("colBuffs.Earnings: %v\n", colBuffs.Earnings)
	log.Printf("currentMultiplier: %v\n", currentMultiplier)
	log.Printf("eventMultipler: %v\n", eventMultipler)
	onlineBaseline := eggValue * deliveryRate * earningBonus * artBuffs.Earnings * colBuffs.Earnings * currentMultiplier * eventMultipler

	permitLevel := backup.Game.GetPermitLevel()
	permitMultiplier := 1.0
	if permitLevel != 1 {
		permitMultiplier = 0.5
	}
	log.Printf("onlineBaseline: %v\n", onlineBaseline)
	log.Printf("permitMultiplier: %v\n", permitMultiplier)
	log.Printf("artBuffs.AwayEarnings: %v\n", artBuffs.AwayEarnings)
	log.Printf("colBuffs.AwayEarnings: %v\n", colBuffs.AwayEarnings)
	offline := onlineBaseline *
		permitMultiplier *
		artBuffs.AwayEarnings *
		colBuffs.AwayEarnings
	log.Printf("offline: %v\n", offline)

	return onlineBaseline, offline
}
