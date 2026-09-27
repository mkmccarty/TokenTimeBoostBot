package ei

import (
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
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

	maxSlots := 4
	if game.GetPermitLevel() != 1 {
		maxSlots = 2
	}

	// Group non-BoB artifacts by family and find maximum slots for each family.
	// In Egg Inc, duplicate artifacts of the same family cannot be equipped simultaneously.
	type nonBobCandidate struct {
		name  ArtifactSpec_Name
		spec  *ArtifactSpec
		slots int
	}
	nonBobMaxSlots := make(map[ArtifactSpec_Name]nonBobCandidate)
	type bobCandidate struct {
		spec  *ArtifactSpec
		bonus float64
		slots int
	}
	var bobCandidates []bobCandidate

	for _, item := range inventory {
		spec := item.GetArtifact().GetSpec()
		if spec == nil || isStoneType(spec.GetName()) {
			continue
		}

		if spec.GetName() == ArtifactSpec_BOOK_OF_BASAN {
			bonus := GetBookOfBasanBonus(spec.GetLevel(), spec.GetRarity())
			bobSlots := GetBookOfBasanSlots(spec.GetLevel(), spec.GetRarity())
			bobCandidates = append(bobCandidates, bobCandidate{spec: spec, bonus: bonus, slots: bobSlots})
		} else {
			slots, _ := GetStones(spec.GetName(), spec.GetLevel(), spec.GetRarity())
			if slots <= 0 && item.GetArtifact() != nil {
				slots = len(item.GetArtifact().GetStones())
			}
			if slots <= 0 {
				switch spec.GetRarity() {
				case ArtifactSpec_LEGENDARY:
					slots = 3
				case ArtifactSpec_EPIC:
					slots = 2
				case ArtifactSpec_RARE:
					slots = 1
				}
			}
			if existing, ok := nonBobMaxSlots[spec.GetName()]; !ok || slots > existing.slots {
				nonBobMaxSlots[spec.GetName()] = nonBobCandidate{name: spec.GetName(), spec: spec, slots: slots}
			}
		}
	}

	var nonBobList []nonBobCandidate
	for _, c := range nonBobMaxSlots {
		nonBobList = append(nonBobList, c)
	}
	sort.Slice(nonBobList, func(i, j int) bool {
		return nonBobList[i].slots > nonBobList[j].slots
	})

	// Calculate stone bonuses (both loose in inventory and socketed in artifacts)
	// For Earnings Bonus, only Prophecy stones are considered.
	var pStones []float64

	addProphecyStone := func(spec *ArtifactSpec) {
		if spec == nil || spec.GetName() != ArtifactSpec_PROPHECY_STONE {
			return
		}
		// Prophecy Stones:
		// Fragment (T1): not socketable (0%)
		// Regular (T2, level 0): +0.05% (+0.0005)
		// Eggsquisite (T3, level 1): +0.10% (+0.0010)
		// Radiant (T4, level 2): +0.15% (+0.0015)
		levels := []float64{0.0005, 0.0010, 0.0015}
		lvl := int(spec.GetLevel())
		if lvl >= len(levels) {
			lvl = len(levels) - 1
		}
		if lvl >= 0 {
			pStones = append(pStones, levels[lvl])
		}
	}

	for _, item := range inventory {
		art := item.GetArtifact()
		if art == nil {
			continue
		}
		spec := art.GetSpec()
		qty := int(item.GetQuantity())
		if qty <= 0 {
			qty = 1
		}
		if spec != nil && spec.GetName() == ArtifactSpec_PROPHECY_STONE {
			for i := 0; i < qty; i++ {
				addProphecyStone(spec)
			}
		}
		for _, st := range art.GetStones() {
			if st != nil && st.GetName() == ArtifactSpec_PROPHECY_STONE {
				for i := 0; i < qty; i++ {
					addProphecyStone(st)
				}
			}
		}
	}

	sort.Slice(pStones, func(i, j int) bool { return pStones[i] > pStones[j] })

	fmtFmt := map[string]any{"decimals": 3, "trim": true}

	nonBobSlotCount := maxSlots - 1
	if nonBobSlotCount < 0 {
		nonBobSlotCount = 0
	}

	// 1 BoB slot + nonBobSlotCount remaining slots (3 for pro permit, 1 for standard permit).
	// If the player has no Book of Basan, the BoB slot is considered empty.
	noBobSlots := 0
	var noBobChosen []nonBobCandidate
	for i := 0; i < nonBobSlotCount && i < len(nonBobList); i++ {
		noBobSlots += nonBobList[i].slots
		noBobChosen = append(noBobChosen, nonBobList[i])
	}
	noBobEB, noBobPUsed := calculateOptimalStoneEB(soulEggsCount, prophecyEggsCount, soulBonus, prophecyBonus, noBobSlots, pStones, eov)
	bestEB := noBobEB
	bestIsBoB := false
	var bestBoB bobCandidate
	bestNonBobChosen := noBobChosen
	bestSlots := noBobSlots
	bestPUsed := noBobPUsed

	var candidateLogs []string

	// If player has a Book of Basan, use the best of that (1 slot for BoB + remaining slots for distinct artifacts with most slots)
	if len(bobCandidates) > 0 && maxSlots > 0 {
		bestEB = 0 // Prioritize equipping the best BoB
		for _, bob := range bobCandidates {
			slots := bob.slots + noBobSlots
			eb, pUsed := calculateOptimalStoneEB(soulEggsCount, prophecyEggsCount, soulBonus, prophecyBonus+bob.bonus, slots, pStones, eov)
			bobLabel := formatArtifactNice(bob.spec)
			candidateLogs = append(candidateLogs, fmt.Sprintf("%s (+%.2f%% PE, %d slots) -> %s%%", bobLabel, bob.bonus*100, slots, FormatEIValue(eb, fmtFmt)))
			if eb > bestEB {
				bestEB = eb
				bestIsBoB = true
				bestBoB = bob
				bestNonBobChosen = noBobChosen
				bestSlots = slots
				bestPUsed = pUsed
			}
		}
	}

	// Bot logging for artifact selection (especially for Standard Permit players)
	dressedStr := FormatEIValue(bestEB, fmtFmt)
	nakedEB := GetEarningsBonus(backup, eov)
	nakedStr := FormatEIValue(nakedEB, fmtFmt)
	userName := backup.GetUserName()
	if userName == "" {
		userName = "Unknown"
	}

	var selectedDesc strings.Builder
	if bestIsBoB && bestBoB.spec != nil {
		fmt.Fprintf(&selectedDesc, "%s (+%.2f%% PE)", formatArtifactNice(bestBoB.spec), bestBoB.bonus*100)
		for _, art := range bestNonBobChosen {
			fmt.Fprintf(&selectedDesc, " + %s (%d slots)", formatArtifactNice(art.spec), art.slots)
		}
	} else {
		var arts []string
		for _, art := range bestNonBobChosen {
			arts = append(arts, fmt.Sprintf("%s (%d slots)", formatArtifactNice(art.spec), art.slots))
		}
		if len(arts) > 0 {
			fmt.Fprintf(&selectedDesc, "No BoB (empty slot) + %s", strings.Join(arts, " + "))
		} else {
			selectedDesc.WriteString("None")
		}
	}
	stonesDesc := formatStonesSummary(bestPUsed)
	fmt.Fprintf(&selectedDesc, " | Stones (%d/%d slots): %s", len(bestPUsed), bestSlots, stonesDesc)

	if game.GetPermitLevel() != 1 {
		log.Printf("[EB Standard Permit] %q | Dressed: %s%% (Naked: %s%%) | Selected: %s | Options: [%s], No-BoB: %s%% (%d slots)",
			userName, dressedStr, nakedStr, selectedDesc.String(), strings.Join(candidateLogs, "; "), FormatEIValue(noBobEB, fmtFmt), noBobSlots)
	} else {
		log.Printf("[EB Pro Permit] %q | Dressed: %s%% (Naked: %s%%) | Selected: %s",
			userName, dressedStr, nakedStr, selectedDesc.String())
	}

	return bestEB
}

// formatArtifactNice formats an artifact spec into a concise human-readable string (e.g. "T4C BOOK", "T4L ANKH").
func formatArtifactNice(spec *ArtifactSpec) string {
	if spec == nil {
		return "unknown"
	}
	tier := fmt.Sprintf("T%d", spec.GetLevel()+1)
	rarity := "C"
	switch spec.GetRarity() {
	case ArtifactSpec_RARE:
		rarity = "R"
	case ArtifactSpec_EPIC:
		rarity = "E"
	case ArtifactSpec_LEGENDARY:
		rarity = "L"
	}
	name := spec.GetName().String()
	name = strings.TrimPrefix(name, "ArtifactSpec_")
	if short, ok := ShortArtifactName[int32(spec.GetName())]; ok && short != "" {
		name = strings.TrimSuffix(short, "_")
	}
	return fmt.Sprintf("%s%s %s", tier, rarity, name)
}

// formatStonesSummary formats a list of used prophecy stone bonuses into a readable string.
func formatStonesSummary(pUsed []float64) string {
	counts := make(map[string]int)
	var order []string
	for _, p := range pUsed {
		var desc string
		switch {
		case math.Abs(p-0.0015) < 1e-5:
			desc = "T4 Prophecy (+0.15%)"
		case math.Abs(p-0.0010) < 1e-5:
			desc = "T3 Prophecy (+0.10%)"
		case math.Abs(p-0.0005) < 1e-5:
			desc = "T2 Prophecy (+0.05%)"
		default:
			desc = fmt.Sprintf("Prophecy (+%.4f%%)", p*100)
		}
		if counts[desc] == 0 {
			order = append(order, desc)
		}
		counts[desc]++
	}
	if len(order) == 0 {
		return "none"
	}
	var parts []string
	for _, desc := range order {
		parts = append(parts, fmt.Sprintf("%dx %s", counts[desc], desc))
	}
	return strings.Join(parts, ", ")
}

// calculateOptimalStoneEB places available Prophecy stones into totalSlots and computes the final EB.
func calculateOptimalStoneEB(soulEggsCount float64, prophecyEggsCount uint64, soulBonus, initialProphecyBonus float64, totalSlots int, pStones []float64, eov float64) (float64, []float64) {
	currentProphecyBonus := initialProphecyBonus
	var pUsed []float64

	for i := 0; i < totalSlots && i < len(pStones); i++ {
		currentProphecyBonus += pStones[i]
		pUsed = append(pUsed, pStones[i])
	}

	eb := soulEggsCount * soulBonus * math.Pow(1+currentProphecyBonus, float64(prophecyEggsCount))
	return eb * (math.Pow(1.01, eov)) * 100, pUsed
}

// GetBookOfBasanBonus returns the additive bonus per Prophecy Egg for a given Book of Basan level and rarity.
func GetBookOfBasanBonus(level ArtifactSpec_Level, rarity ArtifactSpec_Rarity) float64 {
	if data != nil && data.ArtifactFamilies != nil {
		for _, f := range data.ArtifactFamilies {
			if f.AfxID == ArtifactSpec_BOOK_OF_BASAN {
				if int(level) < len(f.Tiers) && f.Tiers[level] != nil {
					tier := f.Tiers[level]
					for _, eff := range tier.Effects {
						if eff.AfxRarity == rarity && eff.EffectDelta > 0 {
							return eff.EffectDelta
						}
					}
				}
			}
		}
	}
	// Fallback table for all Book of Basan tiers and rarities:
	switch level {
	case 0: // T1: Regular
		return 0.0025
	case 1: // T2: Collectors
		return 0.005
	case 2: // T3: Fortified
		if rarity == ArtifactSpec_EPIC {
			return 0.008
		}
		return 0.0075
	case 3: // T4: Gilded
		switch rarity {
		case ArtifactSpec_LEGENDARY:
			return 0.012
		case ArtifactSpec_EPIC:
			return 0.011
		default:
			return 0.010
		}
	}
	return 0.0
}

// GetBookOfBasanSlots returns the number of slots for a Book of Basan given level and rarity.
func GetBookOfBasanSlots(level ArtifactSpec_Level, rarity ArtifactSpec_Rarity) int {
	slots, err := GetStones(ArtifactSpec_BOOK_OF_BASAN, level, rarity)
	if err == nil && slots > 0 {
		return slots
	}
	switch level {
	case 2: // T3
		if rarity == ArtifactSpec_EPIC {
			return 1
		}
	case 3: // T4
		switch rarity {
		case ArtifactSpec_LEGENDARY:
			return 2
		case ArtifactSpec_EPIC:
			return 1
		}
	}
	return 0
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
