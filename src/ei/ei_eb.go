package ei

import (
	"math"
)

const baseSoulEggBonus = 0.1
const baseProphecyEggBonus = 0.05

// GetEarningsBonus calculates the earnings bonus from soul eggs, prophecy eggs, and epic research
func GetEarningsBonus(backup *Backup, eov float64) float64 {
	game := backup.GetGame()
	prophecyEggsCount := game.GetEggsOfProphecy()
	soulEggsCount := game.GetSoulEggsD()
	soulBonus := baseSoulEggBonus
	prophecyBonus := baseProphecyEggBonus

	if game != nil {
		for _, r := range game.EpicResearch {
			if r.GetId() == "soul_eggs" {
				soulBonus += float64(r.GetLevel()) * 0.01
			} else if r.GetId() == "prophecy_bonus" {
				prophecyBonus += float64(r.GetLevel()) * 0.01
			}
		}
	}

	eb := soulEggsCount * soulBonus * math.Pow(1+prophecyBonus, float64(prophecyEggsCount))

	return eb * (math.Pow(1.01, eov)) * 100
}
