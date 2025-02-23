package boost

import (
	"math"

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
func calculateChickenRunTeamwork(coopSize int, durationInDays int, runs int) float64 {
	fCR := max(12.0/(float64(coopSize*durationInDays)), 0.3)
	CR := min(fCR*float64(runs), 6.0)
	return CR
}

func calculateTokenTeamwork(contractDurationSeconds float64, minutesPerToken int, tokenValueSent float64, tokenValueReceived float64) float64 {
	BTA := contractDurationSeconds / (float64(minutesPerToken) * 60)
	T := 0.0

	if BTA <= 42.0 {
		T = ((2.0 / 3.0) * min(tokenValueSent, 3.0)) + ((8.0 / 3.0) * min(max(tokenValueSent-tokenValueReceived, 0.0), 3.0))
	} else {
		T = (200.0/(7.0*BTA))*min(tokenValueSent, 0.07*BTA) + (800.0 / (7.0 * BTA) * min(max(tokenValueSent-tokenValueReceived, 0.0), 0.07*BTA))
	}

	//T := 2.0 * (min(V, tokenValueSent) + 4*min(V, max(0.0, tokenValueSent-tokenValueReceived))) / V
	return T
}

func calculateContractScore(grade int, coopSize int, targetGoal float64, contribution float64, contractLength int, contractDurationSeconds float64, B float64, CR float64, T float64) int64 {
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

	teamworkScore := (5.0*B + CR + T) / 19.0
	teamworkBonus := 1.0 + 0.19*teamworkScore
	score *= teamworkBonus
	score *= float64(187.5)

	return int64(math.Ceil(score))
}

func getPredictedTeamwork(B float64, CR float64, T float64) float64 {
	return (5.0*B + CR + T) / 19.0
}
