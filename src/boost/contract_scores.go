package boost

import (
	"math"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

/*
	func calculateBuffTimeValue(contractDurationSeconds float64, buffHistory []*ei.CoopBuffState) (float64, float64) {
		totalBuffTime := 0.0
				earnings := int(math.Round(a.GetEarnings()*100 - 100))
				eggRate := int(math.Round(a.GetEggLayingRate()*100 - 100))
				serverTimestamp := int64(a.GetServerTimestamp()) // When it was equipped
				if coopStatus.GetSecondsSinceAllGoalsAchieved() > 0 {
					serverTimestamp -= int64(coopStatus.GetSecondsSinceAllGoalsAchieved())
				} else {
					serverTimestamp += calcSecondsRemaining
				}
				serverTimestamp = int64(contractDurationSeconds) - serverTimestamp
				BuffTimeValues = append(BuffTimeValues, BuffTimeValue{name, earnings, 0.0075 * float64(earnings), eggRate, 0.0075 * float64(eggRate) * 10.0, serverTimestamp, 0, 0, 0, 0})
		B := min(buffTimeValue/contractDurationSeconds, 2.0)
		return buffTimeValue, B
	}
*/

// GetTargetBuffTimeValue returns the target buff time value for a contract.
// Rules:
//   - If cxpVersion == 1: target = durationSec * 1.875.
//   - Else: target = durationSec * 2.0.
func GetTargetBuffTimeValue(cxpVersion int, durationSec float64) float64 {
	coef := 2.0
	if cxpVersion == 1 {
		// Sept 22, 2025 and newer contracts don't have buff time value requirements
		coef = 1.875
	}
	return durationSec * coef
}

// GetTargetChickenRun returns the target chicken runs for a contract.
// Rules:
//   - If cxpVersion == 1: target = N - 1.
//   - Else: target = min(20, N * (lengthSec/86400) / 2).
func GetTargetChickenRun(cxpVersion, coopSize int, lengthSec float64) float64 {
	if cxpVersion == 1 {
		// Sept 22, 2025 and newer contracts have N-1 CR requirements
		return float64(coopSize - 1)
	}
	lengthDays := lengthSec / 86400.0
	return math.Min(20.0, float64(coopSize)*(lengthDays)/2.0)
}

// GetTargetTval will return the target tval for the contract based on the contract duration and minutes per token
func GetTargetTval(cxpVersion int, contractMinutes float64, minutesPerToken float64) float64 {
	if cxpVersion == 1 {
		// Sept 22, 2025 and newer contracts don't have token value requirements
		return 0.0
	}
	BTA := math.Floor(contractMinutes / minutesPerToken)
	targetTval := 3.0
	if BTA > 42.0 {
		targetTval = 0.07 * BTA
	}

	return targetTval
}

func calculateBuffTimeValue(cxpVersion int, segmentDurationSeconds float64, deflPercent int, siabPercent int) float64 {
	earnings := float64(siabPercent) * 0.0075
	eggRate := float64(deflPercent) * 0.075
	if cxpVersion == 1 {
		/*
			Sept 22, 2025 and newer contracts don't have buff time value requirements
			T1C (SIAB, Defl.): 0.150*, 0.625*
			T2C (SIAB, Defl.): 0.225*, 1.000*
			T3C (SIAB, Defl.): 0.375*, 1.500*
			T3R (SIAB, Defl.): 0.375*, 1.500*
			T4C (SIAB, Defl.): 0.375 , 1.500*
			T4R (SIAB, Defl.): 0.375 , 1.500*
			T4E (SIAB, Defl.): 0.375*, 1.500*
			T4L (SIAB, Defl.): 0.375 , 1.500*
		*/
		switch siabPercent {
		case 0:
			earnings = 0.0
		case 20: // T1C
			earnings = 0.150
		case 30: // T2C
			earnings = 0.225
		default: // T3C, T3R, T4C, T4R, T4E, T4L
			earnings = 0.375
		}

		switch deflPercent {
		case 0: // No Deflector
			eggRate = 0.0
		case 5: // T1C
			eggRate = 0.625
		case 8: // T2C
			eggRate = 1.000
		default: // T3C, T3R, T4C, T4R, T4E, T4L
			eggRate = 1.500
		}
	}

	buffTimeValue := (segmentDurationSeconds * earnings) + (segmentDurationSeconds * eggRate)
	return buffTimeValue
}

func calculateTeamworkB(buffTimeValue float64, contractDurationSeconds float64) float64 {
	return min(2, buffTimeValue/contractDurationSeconds)
}

func calculateChickenRunTeamwork(cxpVersion int, coopSize int, durationInDays int, runs int) float64 {
	fCR := max(12.0/(float64(coopSize*durationInDays)), 0.3)
	CR := min(fCR*float64(runs), 6.0)
	if cxpVersion == 1 {
		CR = min(float64(runs)/min(float64(coopSize-1), 20.0), 1.0)
	}
	return CR
}

func calculateTokenTeamwork(contractDurationSeconds float64, minutesPerToken int, tokenValueSent float64, tokenValueReceived float64) float64 {
	BTA := math.Floor(contractDurationSeconds / (float64(minutesPerToken) * 60))
	T := 0.0

	if BTA <= 42.0 {
		T = ((2.0 / 3.0) * min(tokenValueSent, 3.0)) + ((8.0 / 3.0) * min(max(tokenValueSent-tokenValueReceived, 0.0), 3.0))
	} else {
		T = (200.0/(7.0*BTA))*min(tokenValueSent, 0.07*BTA) + (800.0 / (7.0 * BTA) * min(max(tokenValueSent-tokenValueReceived, 0.0), 0.07*BTA))
	}

	//T := 2.0 * (min(V, tokenValueSent) + 4*min(V, max(0.0, tokenValueSent-tokenValueReceived))) / V
	return T
}

func calculateContractScore(cxpversion int, grade int, coopSize int, targetGoal float64, contribution float64, contractLength int, contractDurationSeconds float64, B float64, CR float64, T float64) int64 {
	basePoints := 1.0
	durationPoints := 1.0 / 259200.0
	score := basePoints + durationPoints*float64(contractLength)

	gradeMultiplier := ei.GradeMultiplier[ei.Contract_PlayerGrade_name[int32(grade)]]
	score *= gradeMultiplier

	completionFactor := 1.0
	score *= completionFactor

	ratio := contribution / (targetGoal / float64(coopSize))
	contributionFactor := 0.0
	if ratio <= 2.5 {
		contributionFactor = 1 + 3*math.Pow(ratio, 0.15)
	} else {
		contributionFactor = 0.02221*min(ratio, 12.5) + 4.386486
	}
	score *= contributionFactor

	completionTimeBonus := 1.0 + 4.0*math.Pow((1.0-float64(contractDurationSeconds)/float64(contractLength)), 3)
	score *= completionTimeBonus

	teamworkScore := getPredictedTeamwork(cxpversion, B, CR, T)
	teamworkBonus := 1.0 + 0.19*teamworkScore

	score *= teamworkBonus
	score *= float64(187.5)

	return int64(math.Ceil(score))
}

func getPredictedTeamwork(cxpVersion int, B float64, CR float64, T float64) float64 {
	if cxpVersion == 1 {
		// Sept 22, 2025 and newer contracts don't have teamwork
		return (5.0 / 19.0 * (B + CR))
	}
	return (5.0*B + CR + T) / 19.0
}

// getContractScoreEstimate will return the estimated score for the contract
// based on the given parameters
func getContractScoreEstimate(c ei.EggIncContract, grade ei.Contract_PlayerGrade, fastest bool, durationMod float64, fairShare float64, siabPercent float64, siabMinutes int, deflPercent float64, deflMinutesReduction int, chickenRuns int, sentTokens float64, receivedTokens float64) int64 {
	// Sept 22, 2025 and newer contracts don't have buffs

	contractDuration := c.EstimatedDurationLower
	if !fastest {
		contractDuration = c.EstimatedDuration
	}
	contractDuration = time.Duration(float64(contractDuration.Seconds()*durationMod)) * time.Second

	siabDuration := (time.Duration(siabMinutes) * time.Minute).Seconds()
	deflectorDuration := (contractDuration - time.Duration(deflMinutesReduction)*time.Minute).Seconds()
	buffTimeValue := calculateBuffTimeValue(c.SeasonalScoring, siabDuration, 0, int(siabPercent))
	buffTimeValue += calculateBuffTimeValue(c.SeasonalScoring, deflectorDuration, int(deflPercent), 0)

	//buffTimeValue := calculateBuffTimeValue(c.SeasonalScoring, contractDuration.Seconds(), int(siabPercent), int(deflPercent))
	B := calculateTeamworkB(buffTimeValue, contractDuration.Seconds())

	CR := calculateChickenRunTeamwork(c.SeasonalScoring, c.MaxCoopSize, c.LengthInDays, chickenRuns)
	T := calculateTokenTeamwork(contractDuration.Seconds(), c.MinutesPerToken, sentTokens, receivedTokens)
	score := calculateContractScore(c.SeasonalScoring, int(ei.Contract_GRADE_AAA),
		c.MaxCoopSize,
		c.Grade[grade].TargetAmount[len(c.Grade[grade].TargetAmount)-1],
		c.TargetAmount[len(c.TargetAmount)-1]/float64(c.MaxCoopSize)*fairShare,
		c.Grade[grade].LengthInSeconds,
		contractDuration.Seconds(),
		B, CR, T)

	return score
}
