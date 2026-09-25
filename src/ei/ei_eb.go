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

// FarmerRole represents a player's rank role based on Earnings Bonus.
type FarmerRole struct {
	OOM   int
	Name  string
	Color string
}

// FarmerRoles contains the Discord farmer roles ordered by order of magnitude (OoM).
var FarmerRoles = []FarmerRole{
	{OOM: 0, Name: "Farmer", Color: "#d43500"},
	{OOM: 1, Name: "Farmer II", Color: "#d14400"},
	{OOM: 2, Name: "Farmer III", Color: "#cd5500"},
	{OOM: 3, Name: "Kilofarmer", Color: "#ca6800"},
	{OOM: 4, Name: "Kilofarmer II", Color: "#c77a00"},
	{OOM: 5, Name: "Kilofarmer III", Color: "#c58a00"},
	{OOM: 6, Name: "Megafarmer", Color: "#c49400"},
	{OOM: 7, Name: "Megafarmer II", Color: "#c39f00"},
	{OOM: 8, Name: "Megafarmer III", Color: "#c3a900"},
	{OOM: 9, Name: "Gigafarmer", Color: "#c2b100"},
	{OOM: 10, Name: "Gigafarmer II", Color: "#c2ba00"},
	{OOM: 11, Name: "Gigafarmer III", Color: "#c2c200"},
	{OOM: 12, Name: "Terafarmer", Color: "#aec300"},
	{OOM: 13, Name: "Terafarmer II", Color: "#99c400"},
	{OOM: 14, Name: "Terafarmer III", Color: "#85c600"},
	{OOM: 15, Name: "Petafarmer", Color: "#51ce00"},
	{OOM: 16, Name: "Petafarmer II", Color: "#16dc00"},
	{OOM: 17, Name: "Petafarmer III", Color: "#00ec2e"},
	{OOM: 18, Name: "Exafarmer", Color: "#00fa68"},
	{OOM: 19, Name: "Exafarmer II", Color: "#0afc9c"},
	{OOM: 20, Name: "Exafarmer III", Color: "#1cf7ca"},
	{OOM: 21, Name: "Zettafarmer", Color: "#2af3eb"},
	{OOM: 22, Name: "Zettafarmer II", Color: "#35d9f0"},
	{OOM: 23, Name: "Zettafarmer III", Color: "#40bced"},
	{OOM: 24, Name: "Yottafarmer", Color: "#46a8eb"},
	{OOM: 25, Name: "Yottafarmer II", Color: "#4a9aea"},
	{OOM: 26, Name: "Yottafarmer III", Color: "#4e8dea"},
	{OOM: 27, Name: "Xennafarmer", Color: "#527ce9"},
	{OOM: 28, Name: "Xennafarmer II", Color: "#5463e8"},
	{OOM: 29, Name: "Xennafarmer III", Color: "#6155e8"},
	{OOM: 30, Name: "Weccafarmer", Color: "#7952e9"},
	{OOM: 31, Name: "Weccafarmer II", Color: "#8b4fe9"},
	{OOM: 32, Name: "Weccafarmer III", Color: "#9d4aeb"},
	{OOM: 33, Name: "Vendafarmer", Color: "#b343ec"},
	{OOM: 34, Name: "Vendafarmer II", Color: "#d636ef"},
	{OOM: 35, Name: "Vendafarmer III", Color: "#f327e5"},
	{OOM: 36, Name: "Uadafarmer", Color: "#f915ba"},
	{OOM: 37, Name: "Uadafarmer II", Color: "#fc0a9c"},
	{OOM: 38, Name: "Uadafarmer III", Color: "#ff007d"},
	{OOM: 39, Name: "Treidafarmer", Color: "#f7005d"},
	{OOM: 40, Name: "Treidafarmer II", Color: "#f61fd2"},
	{OOM: 41, Name: "Treidafarmer III", Color: "#9c4aea"},
	{OOM: 42, Name: "Quadafarmer", Color: "#5559e8"},
	{OOM: 43, Name: "Quadafarmer II", Color: "#4a9deb"},
	{OOM: 44, Name: "Quadafarmer III", Color: "#2df0f2"},
	{OOM: 45, Name: "Pendafarmer", Color: "#00f759"},
	{OOM: 46, Name: "Pendafarmer II", Color: "#7ec700"},
	{OOM: 47, Name: "Pendafarmer III", Color: "#c2bf00"},
	{OOM: 48, Name: "Exedafarmer", Color: "#c3a000"},
	{OOM: 49, Name: "Exedafarmer II", Color: "#c87200"},
	{OOM: 50, Name: "Exedafarmer III", Color: "#d43500"},
	{OOM: 51, Name: "Infinifarmer", Color: "#546e7a"},
}

// EarningBonusToFarmerRole calculates the farmer role from an earning bonus multiplier ratio (i.e. ebPercent / 100).
func EarningBonusToFarmerRole(earningBonusRatio float64) FarmerRole {
	if earningBonusRatio <= 0 || math.IsNaN(earningBonusRatio) {
		return FarmerRoles[0]
	}
	soulPower := math.Log10(earningBonusRatio)
	oom := int(math.Floor(math.Max(soulPower, 0)))
	if oom >= len(FarmerRoles) {
		return FarmerRole{
			OOM:   oom,
			Name:  FarmerRoles[len(FarmerRoles)-1].Name,
			Color: FarmerRoles[len(FarmerRoles)-1].Color,
		}
	}
	return FarmerRoles[oom]
}

// EarningBonusPercentToFarmerRole calculates the farmer role from an earning bonus percentage.
func EarningBonusPercentToFarmerRole(ebPercent float64) FarmerRole {
	return EarningBonusToFarmerRole(ebPercent / 100.0)
}
