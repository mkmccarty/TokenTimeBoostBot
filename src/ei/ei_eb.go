package ei

import (
	"math"
)

const baseSoulEggBonus = 0.1
const baseProphecyEggBonus = 0.05

// GetEarningsBonus calculates the earnings bonus from soul eggs, prophecy eggs, and epic research
func GetEarningsBonus(backup *Backup, eov float64) float64 {
	prophecyEggsCount := backup.GetGame().GetEggsOfProphecy()
	soulEggsCount := backup.GetGame().GetSoulEggsD()
	soulBonus := baseSoulEggBonus
	prophecyBonus := baseProphecyEggBonus

	soulBonus = GetResearchGeneric(backup.GetGame().GetEpicResearch(), []string{"soul_eggs"}, soulBonus)

	prophecyBonus = GetResearchGeneric(backup.GetGame().GetEpicResearch(), []string{"prophecy_bonus"}, prophecyBonus)
	eb := soulEggsCount * soulBonus * math.Pow(1+prophecyBonus, float64(prophecyEggsCount))

	return eb * (math.Pow(1.01, eov)) * 100
}
