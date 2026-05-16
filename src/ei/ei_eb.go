package ei

import (
	"math"
	"sort"
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

// GetDressedEarningsBonus calculates the optimal earnings bonus from soul eggs, prophecy eggs, epic research, and artifacts.
func GetDressedEarningsBonus(backup *Backup, eov float64) float64 {
	game := backup.GetGame()
	if game == nil {
		return 0
	}
	prophecyEggsCount := game.GetEggsOfProphecy()
	soulEggsCount := game.GetSoulEggsD()
	soulBonus := baseSoulEggBonus
	prophecyBonus := baseProphecyEggBonus

	for _, r := range game.EpicResearch {
		if r.GetId() == "soul_eggs" {
			soulBonus += float64(r.GetLevel()) * 0.01
		} else if r.GetId() == "prophecy_bonus" {
			prophecyBonus += float64(r.GetLevel()) * 0.01
		}
	}

	// Artifacts
	adb := backup.GetArtifactsDb()
	if adb == nil {
		return GetEarningsBonus(backup, eov)
	}
	inventory := adb.GetInventoryItems()

	// Find best BoB
	bestBoB := findBestArtifact(inventory, ArtifactSpec_BOOK_OF_BASAN)
	bobBonus := 0.0
	if bestBoB != nil {
		levels := []float64{0.0025, 0.005, 0.0075, 0.01}
		if int(bestBoB.GetArtifact().GetSpec().GetLevel()) < len(levels) {
			bobBonus = levels[bestBoB.GetArtifact().GetSpec().GetLevel()]
		}
	}

	// Find top 4 non-virtue artifacts with most slots
	type artifactSlot struct {
		slots int
		name  ArtifactSpec_Name
	}
	var nonVirtueArtifacts []artifactSlot
	for _, item := range inventory {
		spec := item.GetArtifact().GetSpec()
		if spec == nil || isStoneType(spec.GetName()) {
			continue
		}
		// Check if virtue
		if _, isVirtue := ArtifactTypeNameVirtue[int32(spec.GetName())]; isVirtue {
			continue
		}

		slots, _ := GetStones(spec.GetName(), spec.GetLevel(), spec.GetRarity())
		nonVirtueArtifacts = append(nonVirtueArtifacts, artifactSlot{slots: slots, name: spec.GetName()})
	}

	// Sort by slots desc
	sort.Slice(nonVirtueArtifacts, func(i, j int) bool {
		return nonVirtueArtifacts[i].slots > nonVirtueArtifacts[j].slots
	})

	totalSlots := 0
	equippedCount := 0
	bobEquipped := false
	for _, a := range nonVirtueArtifacts {
		if equippedCount >= 4 {
			break
		}
		if a.name == ArtifactSpec_BOOK_OF_BASAN {
			bobEquipped = true
		}
		totalSlots += a.slots
		equippedCount++
	}

	// If BoB wasn't in top 4 (unlikely, but possible), we should consider it
	if !bobEquipped && bestBoB != nil {
		// Just for safety, add BoB slots if we have room or it's better than the 4th
		bobSlots, _ := GetStones(ArtifactSpec_BOOK_OF_BASAN, bestBoB.GetArtifact().GetSpec().GetLevel(), bestBoB.GetArtifact().GetSpec().GetRarity())
		if equippedCount < 4 {
			totalSlots += bobSlots
		} else {
			// Replace 4th if BoB is better? BoB itself adds bobBonus regardless of slots.
			// But the slots are also valuable.
		}
	}

	// Calculate stone bonuses
	var pStones []float64
	var sStones []float64
	for _, item := range inventory {
		spec := item.GetArtifact().GetSpec()
		if spec == nil {
			continue
		}
		qty := int(item.GetQuantity())
		if spec.GetName() == ArtifactSpec_PROPHECY_STONE {
			levels := []float64{0.0005, 0.001, 0.0015} // T2-T4 (T1 is fragment)
			bonus := 0.0
			if int(spec.GetLevel()) < len(levels) {
				bonus = levels[spec.GetLevel()]
			}
			for i := 0; i < qty; i++ {
				pStones = append(pStones, bonus)
			}
		} else if spec.GetName() == ArtifactSpec_SOUL_STONE {
			levels := []float64{0.05, 0.10, 0.25} // T2-T4
			bonus := 0.0
			if int(spec.GetLevel()) < len(levels) {
				bonus = levels[spec.GetLevel()]
			}
			for i := 0; i < qty; i++ {
				sStones = append(sStones, bonus)
			}
		}
	}

	sort.Slice(pStones, func(i, j int) bool { return pStones[i] > pStones[j] })
	sort.Slice(sStones, func(i, j int) bool { return sStones[i] > sStones[j] })

	// Greedily fill slots
	currentSoulBonus := soulBonus
	currentProphecyBonus := prophecyBonus + bobBonus
	pIdx, sIdx := 0, 0

	for i := 0; i < totalSlots; i++ {
		pBonus := 0.0
		if pIdx < len(pStones) {
			pBonus = pStones[pIdx]
		}
		sBonus := 0.0
		if sIdx < len(sStones) {
			sBonus = sStones[sIdx]
		}

		if pBonus == 0 && sBonus == 0 {
			break
		}

		// Try adding P stone
		ebP := soulEggsCount * currentSoulBonus * math.Pow(1+currentProphecyBonus+pBonus, float64(prophecyEggsCount))
		// Try adding S stone
		ebS := soulEggsCount * (currentSoulBonus * (1 + sBonus)) * math.Pow(1+currentProphecyBonus, float64(prophecyEggsCount))

		if ebP > ebS && pBonus > 0 {
			currentProphecyBonus += pBonus
			pIdx++
		} else if sBonus > 0 {
			currentSoulBonus *= (1 + sBonus)
			sIdx++
		} else {
			// fallback
			currentProphecyBonus += pBonus
			pIdx++
		}
	}

	eb := soulEggsCount * currentSoulBonus * math.Pow(1+currentProphecyBonus, float64(prophecyEggsCount))
	return eb * (math.Pow(1.01, eov)) * 100
}
