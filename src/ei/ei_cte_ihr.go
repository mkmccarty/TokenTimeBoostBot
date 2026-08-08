package ei

import (
	"math"
)

// MaxClothedIHRResult contains max clothed IHR rate and the recommended artifact set.
type MaxClothedIHRResult struct {
	ClothedIHR float64
	Artifacts  []*CompleteArtifact
}

// CalculateClothedIHR calculates total clothed IHR from current backup state.
func CalculateClothedIHR(backup *Backup) float64 {
	return CalculateClothedIHRWithArtifacts(backup, GetActiveVirtueArtifacts(backup))
}

// CalculateClothedIHRFromBackup calculates total clothed IHR from current backup state.
func CalculateClothedIHRFromBackup(backup *Backup) float64 {
	return CalculateClothedIHRWithArtifacts(backup, GetActiveVirtueArtifacts(backup))
}

// CalculateMaxClothedIHR calculates the maximum possible clothed IHR rate from virtue inventory.
func CalculateMaxClothedIHR(backup *Backup) MaxClothedIHRResult {
	return CalculateMaxClothedIHRWithSlotHint(backup, 0)
}

// CalculateMaxClothedIHRWithSlotHint calculates max clothed IHR using a host slot hint (e.g. equipped count).
func CalculateMaxClothedIHRWithSlotHint(backup *Backup, slotHint int) MaxClothedIHRResult {
	bestArtifacts := GetMaxClothedIHRArtifactsWithSlotHint(backup, slotHint)
	return MaxClothedIHRResult{
		ClothedIHR: CalculateClothedIHRWithArtifacts(backup, bestArtifacts),
		Artifacts:  bestArtifacts,
	}
}

// GetMaxClothedIHRArtifacts returns the best artifact loadout for clothed IHR.
func GetMaxClothedIHRArtifacts(backup *Backup) []*CompleteArtifact {
	return GetMaxClothedIHRArtifactsWithSlotHint(backup, 0)
}

// GetMaxClothedIHRArtifactsWithSlotHint returns best artifact loadout using slot hint when provided.
func GetMaxClothedIHRArtifactsWithSlotHint(backup *Backup, slotHint int) []*CompleteArtifact {
	if backup == nil || backup.GetArtifactsDb() == nil {
		return nil
	}
	inventory := backup.GetArtifactsDb().GetInventoryItems()

	slotCount := resolveCTEArtifactSlotCount(backup, slotHint)

	hostCandidates, stonePool := collectCTEIHRCandidates(inventory)
	if len(hostCandidates) == 0 {
		return nil
	}

	stoneProduct, stonesBySlots := bestStoneProducts(stonePool)
	bestHosts := bestCTEHostCombo(hostCandidates, slotCount, stoneProduct)
	if len(bestHosts) == 0 {
		return nil
	}

	totalSlots := 0
	for _, host := range bestHosts {
		totalSlots += host.slots
	}
	chosenStones := []*ArtifactSpec{}
	if totalSlots >= 0 && totalSlots < len(stonesBySlots) {
		chosenStones = stonesBySlots[totalSlots]
	} else if len(stonesBySlots) > 0 {
		chosenStones = stonesBySlots[len(stonesBySlots)-1]
	}

	return buildReslottedArtifacts(bestHosts, chosenStones)
}

// CalculateClothedIHRWithArtifacts calculates total player IHR with an explicit artifact setup.
func CalculateClothedIHRWithArtifacts(backup *Backup, artifacts []*CompleteArtifact) float64 {
	if backup == nil {
		return 0
	}
	baseOnlineRatePerHab := 7440.0

	var allEov uint32
	virtue := backup.GetVirtue()
	if virtue != nil {
		earned := virtue.GetEovEarned()
		delivered := virtue.GetEggsDelivered()
		n := len(delivered)
		if len(earned) < n {
			n = len(earned)
		}
		for i := 0; i < n; i++ {
			tiersPassed := countTruthEggTiersPassed(delivered[i])
			eov := earned[i]
			eovPending := uint32(0)
			if tiersPassed > eov {
				eovPending = tiersPassed - eov
			}
			eovEarned := tiersPassed
			if eovEarned > eovPending {
				allEov += (eovEarned - eovPending)
			}
		}
	}

	teMultiplier := math.Pow(1.01, float64(allEov))

	colMultiplier := 1.0
	currentModifiers := GetColleggtibleBuffs(backup.GetContracts())
	if currentModifiers.IHR > 0 {
		colMultiplier = currentModifiers.IHR
	}

	artifactBuffs := GetArtifactBuffs(artifacts)
	chaliceMultiplier := artifactBuffs.IHR

	monocleMultiplier := 1.0
	monocleMap := map[string]float64{
		"T1C": 1.05,
		"T2C": 1.10,
		"T3C": 1.15,
		"T4C": 1.20, "T4E": 1.25, "T4L": 1.30,
	}
	for _, art := range artifacts {
		if art != nil && art.GetSpec() != nil && art.GetSpec().GetName() == ArtifactSpec_DILITHIUM_MONOCLE {
			spec := art.GetSpec()
			strType := ArtifactLevels[spec.GetLevel()] + ArtifactRarity[spec.GetRarity()]
			if mult, ok := monocleMap[strType]; ok {
				monocleMultiplier = mult
			}
		}
	}

	lifeStoneMultiplier := 1.0
	for _, art := range artifacts {
		if art != nil {
			for _, st := range art.GetStones() {
				if st != nil && st.GetName() == ArtifactSpec_LIFE_STONE {
					idx := st.GetLevel()
					if idx >= 0 && idx <= 2 {
						levels := []float64{1.02, 1.04, 1.05}
						lifeStoneMultiplier *= levels[idx]
					}
				}
			}
		}
	}

	return baseOnlineRatePerHab * teMultiplier * colMultiplier * chaliceMultiplier * monocleMultiplier * lifeStoneMultiplier
}

func collectCTEIHRCandidates(inventory []*ArtifactInventoryItem) ([]cteHostCandidate, []cteStoneCandidate) {
	hosts := make([]cteHostCandidate, 0, len(inventory))
	bestHostByType := make(map[ArtifactSpec_Name]cteHostCandidate)
	stones := make([]cteStoneCandidate, 0)

	for _, item := range inventory {
		if item == nil || item.GetArtifact() == nil {
			continue
		}

		artifact := item.GetArtifact()
		spec := artifact.GetSpec()
		if spec == nil {
			continue
		}

		if isStoneType(spec.GetName()) {
			if spec.GetName() == ArtifactSpec_LIFE_STONE {
				qty := int(item.GetQuantity())
				if qty < 1 {
					qty = 1
				}
				for i := 0; i < qty; i++ {
					stones = append(stones, cteStoneCandidate{spec: spec, multiplier: lifeStoneMultiplierValue(spec)})
				}
			}
			continue
		}

		for _, st := range artifact.GetStones() {
			if st == nil || st.GetName() != ArtifactSpec_LIFE_STONE {
				continue
			}
			stones = append(stones, cteStoneCandidate{spec: st, multiplier: lifeStoneMultiplierValue(st)})
		}

		slots, err := GetStones(spec.GetName(), spec.GetLevel(), spec.GetRarity())
		if err != nil {
			slots = len(artifact.GetStones())
		}
		if slots < 0 {
			slots = 0
		}

		base := cteIHRArtifactMultiplierWithoutStones(spec)
		if base <= 1.0 && slots == 0 {
			continue
		}

		candidate := cteHostCandidate{
			artifact:       artifact,
			hostType:       spec.GetName(),
			baseMultiplier: base,
			slots:          slots,
		}

		best, exists := bestHostByType[candidate.hostType]
		if !exists || cteHostPotential(candidate) > cteHostPotential(best) {
			bestHostByType[candidate.hostType] = candidate
		}
	}

	for _, candidate := range bestHostByType {
		hosts = append(hosts, candidate)
	}

	return hosts, stones
}

func cteIHRArtifactMultiplierWithoutStones(spec *ArtifactSpec) float64 {
	if spec == nil {
		return 1.0
	}
	buffs := GetArtifactBuffs([]*CompleteArtifact{{Spec: spec, Stones: nil}})
	mult := buffs.IHR
	if spec.GetName() == ArtifactSpec_DILITHIUM_MONOCLE {
		monocleMap := map[string]float64{
			"T1C": 1.05,
			"T2C": 1.10,
			"T3C": 1.15,
			"T4C": 1.20, "T4E": 1.25, "T4L": 1.30,
		}
		strType := ArtifactLevels[spec.GetLevel()] + ArtifactRarity[spec.GetRarity()]
		if m, ok := monocleMap[strType]; ok {
			mult *= m
		}
	}
	return mult
}

func lifeStoneMultiplierValue(spec *ArtifactSpec) float64 {
	if spec == nil {
		return 1.0
	}
	idx := int(spec.GetLevel())
	if idx >= 0 && idx <= 2 {
		levels := []float64{1.02, 1.04, 1.05}
		return levels[idx]
	}
	return 1.0
}
