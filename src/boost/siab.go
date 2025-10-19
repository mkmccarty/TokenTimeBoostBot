package boost

import (
	"fmt"
	"time"
)

// ProductionScheduleParams Used to determine the entire coop swap time
type ProductionScheduleParams struct {
	name            string
	targetEggAmount float64
	initialElr      float64
	deltaElr        float64
	alpha           float64
	elapsedTimeSec  float64
	eggsShipped     float64
	startTime       time.Time
	timezone        string
	futureSwapTime  time.Time
}

// ProductionSchedule calculates key timestamps for a production schedule based on egg production.
//
// Created by: James WST DiscordID: @james.wst
// Parameters:
//
//	targetEggAmount: float64 - Target Egg Amount in quadrillion eggs. (q = 10^15)
//	initialElr: float64 - The initial elr from all players in quadrillion eggs per hour. (q/h)
//	deltaElr: float64 - The change in ELR per hour due to one person SiaB->Gusset switching (q/h)
//	alpha: float64 - The fraction of the total contract duration at which the switch occurs. 0 < alpha < 1.
//	elapsedTimeSec: float64 - The elapsed time in seconds since the start of the contract. (s)
//	eggsShipped: float64 - The number of eggs already shipped in quadrillion eggs. (q)
//	startLocal: time.Time - The start time of the contract in local time.
//	tz: string - The timezone of the start time, e.g., "Europe/Berlin".
//
// Returns:
//
//	switchTime, switchTimestamp: (time.Time, int64) - The time and Unix timestamp when the switch occurs.
//	finishTimeWithSwitch, finishTimestampWithSwitch: (time.Time, int64) - The time and Unix timestamp when the contract finishes with the switch.
//	finishTimeWithoutSwitch, finishTimestampWithoutSwitch: (time.Time, int64) - The time and Unix timestamp when the contract finishes without the switch.
//	err: error - An error if any calculation or timezone loading fails.
func ProductionSchedule(
	targetEggAmount float64,
	initialElr float64,
	deltaElr float64,
	alpha float64,
	elapsedTimeSec float64, // Now in seconds
	eggsShipped float64,
	startLocal time.Time,
	tz string,
) (
	switchTime time.Time,
	switchTimestamp int64,
	finishTimeWithSwitch time.Time,
	finishTimestampWithSwitch int64,
	finishTimeWithoutSwitch time.Time,
	finishTimestampWithoutSwitch int64,
	err error,
) {
	// Convert elapsed time from seconds to hours
	elapsedTimeHours := elapsedTimeSec / 3600.0 // hours

	remainingEggAmount := targetEggAmount - eggsShipped // q

	// Load the timezone location
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.Time{}, 0, time.Time{}, 0, time.Time{}, 0, fmt.Errorf("failed to load timezone %s: %w", tz, err)
	}

	// Convert the startLocal time to the specified timezone
	start := startLocal.In(loc)

	// Calculate Total Contract Duration (T) in hours
	// Denominator for total_contract_duration calculation: initialElr + deltaElr * (1 - alpha)
	denominatorDuration := initialElr + deltaElr*(1-alpha)
	if denominatorDuration == 0 {
		return time.Time{}, 0, time.Time{}, 0, time.Time{}, 0, fmt.Errorf("cannot calculate total contract duration: denominator is zero (initialElr + deltaElr*(1 - alpha) = 0)")
	}
	totalContractDuration := (remainingEggAmount + initialElr*elapsedTimeHours) / denominatorDuration // in hours
	//fmt.Printf("Total contract duration: %.2f hours\n", totalContractDuration)

	// Calculate elapsed time at which the switch occurs
	elapsedTimeAtSwitch := alpha * totalContractDuration // in hours

	// Calculate switch_time
	switchTime = start.Add(time.Duration(elapsedTimeAtSwitch * float64(time.Hour)))
	switchTimestamp = switchTime.Unix()

	// Calculate finish_time_with_switch
	finishTimeWithSwitch = start.Add(time.Duration(totalContractDuration * float64(time.Hour)))
	finishTimestampWithSwitch = finishTimeWithSwitch.Unix()

	// Calculate finish_time_without_switch
	// Check for division by zero for remainingEggAmount / initialElr
	if initialElr == 0 {
		return time.Time{}, 0, time.Time{}, 0, time.Time{}, 0, fmt.Errorf("cannot calculate finish_time_without_switch: initial_elr is zero")
	}
	finishTimeWithoutSwitch = start.Add(time.Duration((remainingEggAmount/initialElr + elapsedTimeHours) * float64(time.Hour)))
	finishTimestampWithoutSwitch = finishTimeWithoutSwitch.Unix()

	return switchTime, switchTimestamp, finishTimeWithSwitch, finishTimestampWithSwitch, finishTimeWithoutSwitch, finishTimestampWithoutSwitch, nil
}
