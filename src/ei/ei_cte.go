package ei

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// MaxClothedTEResult contains max clothed TE and the recommended artifact set.
type MaxClothedTEResult struct {
	ClothedTE float64
	Artifacts []*CompleteArtifact
}

var truthEggBreakpointsCTE = buildTruthEggBreakpoints()

func buildTruthEggBreakpoints() []float64 {
	breakpoints := []float64{
		5e7,
		1e9,
		1e10,
		7e10,
		5e11,
		2e12,
		7e12,
		2e13,
		6e13,
		1.5e14,
		5e14,
		1.5e15,
		4e15,
		1e16,
		2.5e16,
		5e16,
	}

	for m := 17; m <= 98; m++ {
		am := 1e17 + float64(m-17)*5e16 + float64((m-17)*(m-18)/2)*1e16
		breakpoints = append(breakpoints, am)
	}

	return breakpoints
}

// MultiplierToTE converts an earnings multiplier into Truth Egg equivalent.
func MultiplierToTE(multiplier float64) float64 {
	if multiplier <= 0 {
		return math.Inf(-1)
	}
	return math.Log(multiplier) / math.Log(1.1)
}

// CTEFromArtifacts calculates the Clothed TE contribution from artifacts only.
func CTEFromArtifacts(artifacts []*CompleteArtifact) float64 {
	artifactBuffs := GetArtifactBuffs(artifacts)
	researchDiscountEffect := 1 / artifactBuffs.ResearchDiscount
	totalMultiplier := artifactBuffs.Earnings * artifactBuffs.AwayEarnings * researchDiscountEffect
	return MultiplierToTE(totalMultiplier)
}

// CTEFromColleggtibles calculates the Clothed TE contribution from colleggtibles only.
func CTEFromColleggtibles(modifiers DimensionBuffs) float64 {
	maxEarningsModifier := maxColleggtibleModifier(GameModifier_EARNINGS)
	maxAwayEarningsModifier := maxColleggtibleModifier(GameModifier_AWAY_EARNINGS)
	maxResearchCostModifier := maxColleggtibleModifier(GameModifier_RESEARCH_COST)

	earningsPenalty := modifiers.Earnings / maxEarningsModifier
	awayEarningsPenalty := modifiers.AwayEarnings / maxAwayEarningsModifier
	researchCostPenalty := maxResearchCostModifier / modifiers.ResearchDiscount

	totalPenalty := earningsPenalty * awayEarningsPenalty * researchCostPenalty
	return MultiplierToTE(totalPenalty)
}

// CTEFromColleggtiblesFromBackup calculates colleggtible-only CTE using backup contracts.
func CTEFromColleggtiblesFromBackup(backup *Backup) float64 {
	if backup == nil {
		return 0
	}
	currentModifiers := GetColleggtibleBuffs(backup.GetContracts())
	return CTEFromColleggtibles(currentModifiers)
}

// CalculateClothedTE calculates total player CTE from current backup state.
func CalculateClothedTE(backup *Backup) float64 {
	return CalculateClothedTEWithArtifacts(backup, GetActiveVirtueArtifacts(backup))
}

// CalculateMaxClothedTE calculates the maximum possible clothed TE from virtue inventory.
func CalculateMaxClothedTE(backup *Backup) MaxClothedTEResult {
	return CalculateMaxClothedTEWithSlotHint(backup, 0)
}

// CalculateMaxClothedTEWithSlotHint calculates max clothed TE using a host slot hint (e.g. equipped count).
// Pass 0 to use fallback permit-based slot count.
func CalculateMaxClothedTEWithSlotHint(backup *Backup, slotHint int) MaxClothedTEResult {
	bestArtifacts := GetMaxClothedTEArtifactsWithSlotHint(backup, slotHint)
	return MaxClothedTEResult{
		ClothedTE: CalculateClothedTEWithArtifacts(backup, bestArtifacts),
		Artifacts: bestArtifacts,
	}
}

// GetMaxClothedTEArtifacts returns the best artifact loadout for clothed TE.
func GetMaxClothedTEArtifacts(backup *Backup) []*CompleteArtifact {
	return GetMaxClothedTEArtifactsWithSlotHint(backup, 0)
}

// GetMaxClothedTEArtifactsWithSlotHint returns best artifact loadout using slot hint when provided.
func GetMaxClothedTEArtifactsWithSlotHint(backup *Backup, slotHint int) []*CompleteArtifact {
	virtueDB := getVirtueArtifactDB(backup)
	if virtueDB == nil {
		return nil
	}

	slotCount := resolveCTEArtifactSlotCount(backup, slotHint)

	hostCandidates, stonePool := collectCTECandidates(virtueDB.GetInventoryItems())
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

func resolveCTEArtifactSlotCount(backup *Backup, slotHint int) int {
	if slotHint > 0 {
		if slotHint < 2 {
			return 2
		}
		if slotHint > 4 {
			return 4
		}
		return slotHint
	}

	if game := backup.GetGame(); game != nil && game.GetPermitLevel() != 1 {
		return 2
	}
	return 4
}

// CalculateClothedTEWithArtifacts calculates total player CTE with an explicit artifact setup.
func CalculateClothedTEWithArtifacts(backup *Backup, artifacts []*CompleteArtifact) float64 {
	if backup == nil {
		return 0
	}

	truthEggs := float64(GetCurrentTruthEggs(backup))
	game := backup.GetGame()
	if game == nil {
		return truthEggs
	}

	currentModifiers := GetColleggtibleBuffs(backup.GetContracts())
	artifactBuffs := GetArtifactBuffs(artifacts)

	maxEarningsModifier := maxColleggtibleModifier(GameModifier_EARNINGS)
	maxAwayEarningsModifier := maxColleggtibleModifier(GameModifier_AWAY_EARNINGS)
	maxResearchCostModifier := maxColleggtibleModifier(GameModifier_RESEARCH_COST)

	epicResearchMult := GetResearchDiscount(game.GetEpicResearch())
	maxEpicResearchMult := 0.5

	permitMult := 1.0
	if game.GetPermitLevel() != 1 {
		permitMult = 0.5
	}

	earningsEffect := artifactBuffs.Earnings * artifactBuffs.AwayEarnings

	currentResearchPriceMult := artifactBuffs.ResearchDiscount * epicResearchMult * currentModifiers.ResearchDiscount
	researchDiscountEffect := 1 / currentResearchPriceMult
	maxResearchDiscountEffect := 1 / (maxEpicResearchMult * maxResearchCostModifier)

	earningsPenalty := currentModifiers.Earnings / maxEarningsModifier
	awayEarningsPenalty := currentModifiers.AwayEarnings / maxAwayEarningsModifier
	researchCostPenalty := researchDiscountEffect / maxResearchDiscountEffect

	totalMultiplier := earningsEffect * permitMult * earningsPenalty * awayEarningsPenalty * researchCostPenalty
	multiplierAsTE := MultiplierToTE(totalMultiplier)

	return truthEggs + multiplierAsTE
}

// GetActiveVirtueArtifacts returns currently equipped virtue artifacts from backup.
func GetActiveVirtueArtifacts(backup *Backup) []*CompleteArtifact {
	virtueDB := getVirtueArtifactDB(backup)
	if virtueDB == nil {
		return nil
	}

	return activeArtifactsFromSet(virtueDB.GetInventoryItems(), virtueDB.GetActiveArtifacts())
}

// GetCurrentTruthEggs returns currently credited Truth Eggs derived from virtue progress.
func GetCurrentTruthEggs(backup *Backup) uint32 {
	if backup == nil || backup.GetVirtue() == nil {
		return 0
	}

	virtue := backup.GetVirtue()
	earned := virtue.GetEovEarned()
	delivered := virtue.GetEggsDelivered()

	n := len(delivered)
	if len(earned) < n {
		n = len(earned)
	}

	var total uint32
	for i := 0; i < n; i++ {
		tiersPassed := countTruthEggTiersPassed(delivered[i])
		if tiersPassed < earned[i] {
			total += tiersPassed
		} else {
			total += earned[i]
		}
	}

	return total
}

// CountTruthEggTiersPassed returns the number of Truth Egg tiers passed for delivered value.
func CountTruthEggTiersPassed(delivered float64) uint32 {
	return countTruthEggTiersPassed(delivered)
}

// PendingTruthEggs returns pending Truth Eggs for a delivered value against earned Truth Eggs.
func PendingTruthEggs(delivered float64, earnedTE uint32) uint32 {
	tiersPassed := countTruthEggTiersPassed(delivered)
	if tiersPassed <= earnedTE {
		return 0
	}
	return tiersPassed - earnedTE
}

// NextTruthEggThreshold returns the next Truth Egg threshold for a delivered value.
// If all tiers are passed, it returns +Inf.
func NextTruthEggThreshold(delivered float64, eov uint32) float64 {
	tiersPassed := countTruthEggTiersPassed(delivered)
	if tiersPassed != 0 && tiersPassed < eov {
		tiersPassed = eov
	}
	if int(tiersPassed) >= len(truthEggBreakpointsCTE) {
		return math.Inf(1)
	}
	return truthEggBreakpointsCTE[tiersPassed]
}

// TruthEggThresholdByIndex returns the threshold value for a 1-based Truth Egg target.
// If targetTE is out of range, it returns +Inf.
func TruthEggThresholdByIndex(targetTE uint32) float64 {
	if targetTE == 0 || int(targetTE) > len(truthEggBreakpointsCTE) {
		return math.Inf(1)
	}
	return truthEggBreakpointsCTE[targetTE-1]
}

func countTruthEggTiersPassed(delivered float64) uint32 {
	i := 0
	for i < len(truthEggBreakpointsCTE) && delivered >= truthEggBreakpointsCTE[i] {
		i++
	}
	return uint32(i)
}

func maxColleggtibleModifier(dimension GameModifier_GameDimension) float64 {
	multiplier := 1.0
	for _, egg := range CustomEggMap {
		if egg == nil || egg.Dimension != dimension || len(egg.DimensionValue) == 0 {
			continue
		}
		multiplier *= egg.DimensionValue[len(egg.DimensionValue)-1]
	}
	if multiplier <= 0 {
		return 1.0
	}
	return multiplier
}

func activeArtifactsFromSet(inventory []*ArtifactInventoryItem, activeSet *ArtifactsDB_ActiveArtifactSet) []*CompleteArtifact {
	if activeSet == nil || len(inventory) == 0 {
		return nil
	}

	slots := activeSet.GetSlots()
	if len(slots) == 0 {
		return nil
	}

	itemsByID := make(map[uint64]*CompleteArtifact, len(inventory))
	for _, item := range inventory {
		if item == nil || item.GetArtifact() == nil {
			continue
		}
		itemsByID[item.GetItemId()] = item.GetArtifact()
	}

	artifacts := make([]*CompleteArtifact, 0, len(slots))
	for _, slot := range slots {
		if slot == nil || !slot.GetOccupied() {
			continue
		}
		if artifact := itemsByID[slot.GetItemId()]; artifact != nil {
			artifacts = append(artifacts, artifact)
		}
	}

	return artifacts
}

func getVirtueArtifactDB(backup *Backup) *ArtifactsDB_VirtueDB {
	if backup == nil {
		return nil
	}
	artifactsDB := backup.GetArtifactsDb()
	if artifactsDB == nil {
		return nil
	}
	return artifactsDB.GetVirtueAfxDb()
}

type cteHostCandidate struct {
	artifact       *CompleteArtifact
	hostType       ArtifactSpec_Name
	baseMultiplier float64
	slots          int
}

type cteStoneCandidate struct {
	spec       *ArtifactSpec
	multiplier float64
}

func collectCTECandidates(inventory []*ArtifactInventoryItem) ([]cteHostCandidate, []cteStoneCandidate) {
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
			if isCTERelevantStone(spec.GetName()) {
				qty := int(item.GetQuantity())
				if qty < 1 {
					qty = 1
				}
				for i := 0; i < qty; i++ {
					stones = append(stones, cteStoneCandidate{spec: spec, multiplier: cteStoneMultiplier(spec)})
				}
			}
			continue
		}

		for _, st := range artifact.GetStones() {
			if st == nil || !isCTERelevantStone(st.GetName()) {
				continue
			}
			stones = append(stones, cteStoneCandidate{spec: st, multiplier: cteStoneMultiplier(st)})
		}

		slots, err := GetStones(spec.GetName(), spec.GetLevel(), spec.GetRarity())
		if err != nil {
			slots = len(artifact.GetStones())
		}
		if slots < 0 {
			slots = 0
		}

		base := cteArtifactMultiplierWithoutStones(spec)
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

	sort.SliceStable(stones, func(i, j int) bool {
		return stones[i].multiplier > stones[j].multiplier
	})

	return hosts, stones
}

func cteHostPotential(host cteHostCandidate) float64 {
	return host.baseMultiplier * math.Pow(1.4, float64(host.slots))
}

func bestStoneProducts(stones []cteStoneCandidate) ([]float64, [][]*ArtifactSpec) {
	products := make([]float64, len(stones)+1)
	products[0] = 1.0
	picks := make([][]*ArtifactSpec, len(stones)+1)
	picks[0] = []*ArtifactSpec{}

	for i := 1; i <= len(stones); i++ {
		products[i] = products[i-1] * stones[i-1].multiplier
		current := make([]*ArtifactSpec, 0, i)
		current = append(current, picks[i-1]...)
		current = append(current, stones[i-1].spec)
		picks[i] = current
	}

	return products, picks
}

func bestCTEHostCombo(hosts []cteHostCandidate, maxHosts int, stoneProducts []float64) []cteHostCandidate {
	bestScore := 1.0
	bestHosts := []cteHostCandidate{}
	usedTypes := make(map[ArtifactSpec_Name]bool)
	current := make([]cteHostCandidate, 0, maxHosts)

	var dfs func(index int)
	dfs = func(index int) {
		if len(current) > 0 {
			base := 1.0
			totalSlots := 0
			for _, host := range current {
				base *= host.baseMultiplier
				totalSlots += host.slots
			}
			if totalSlots < 0 {
				totalSlots = 0
			}
			if totalSlots >= len(stoneProducts) {
				totalSlots = len(stoneProducts) - 1
			}
			score := base * stoneProducts[totalSlots]
			if score > bestScore {
				bestScore = score
				bestHosts = append([]cteHostCandidate(nil), current...)
			}
		}

		if index >= len(hosts) || len(current) == maxHosts {
			return
		}

		dfs(index + 1)

		host := hosts[index]
		if usedTypes[host.hostType] {
			return
		}
		usedTypes[host.hostType] = true
		current = append(current, host)
		dfs(index + 1)
		current = current[:len(current)-1]
		usedTypes[host.hostType] = false
	}

	dfs(0)
	return bestHosts
}

func buildReslottedArtifacts(hosts []cteHostCandidate, stones []*ArtifactSpec) []*CompleteArtifact {
	result := make([]*CompleteArtifact, 0, len(hosts))
	stoneIdx := 0

	for _, host := range hosts {
		if host.artifact == nil || host.artifact.GetSpec() == nil {
			continue
		}

		assigned := make([]*ArtifactSpec, 0, host.slots)
		for i := 0; i < host.slots && stoneIdx < len(stones); i++ {
			assigned = append(assigned, stones[stoneIdx])
			stoneIdx++
		}

		result = append(result, &CompleteArtifact{
			Spec:   host.artifact.GetSpec(),
			Stones: assigned,
		})
	}

	return result
}

func cteArtifactMultiplierWithoutStones(spec *ArtifactSpec) float64 {
	if spec == nil {
		return 1.0
	}
	buffs := GetArtifactBuffs([]*CompleteArtifact{{Spec: spec, Stones: nil}})
	return buffs.Earnings * buffs.AwayEarnings * (1 / buffs.ResearchDiscount)
}

func cteStoneMultiplier(spec *ArtifactSpec) float64 {
	if spec == nil {
		return 1.0
	}
	if spec.GetLevel() < 0 || int(spec.GetLevel()) > 2 {
		return 1.0
	}
	idx := spec.GetLevel()
	switch spec.GetName() {
	case ArtifactSpec_LUNAR_STONE:
		levels := []float64{1.2, 1.3, 1.4}
		return levels[idx]
	case ArtifactSpec_SHELL_STONE:
		levels := []float64{1.05, 1.08, 1.1}
		return levels[idx]
	default:
		return 1.0
	}
}

func isCTERelevantStone(name ArtifactSpec_Name) bool {
	return name == ArtifactSpec_LUNAR_STONE || name == ArtifactSpec_SHELL_STONE
}

func isStoneType(name ArtifactSpec_Name) bool {
	switch name {
	case ArtifactSpec_TACHYON_STONE,
		ArtifactSpec_DILITHIUM_STONE,
		ArtifactSpec_SHELL_STONE,
		ArtifactSpec_LUNAR_STONE,
		ArtifactSpec_SOUL_STONE,
		ArtifactSpec_PROPHECY_STONE,
		ArtifactSpec_QUANTUM_STONE,
		ArtifactSpec_TERRA_STONE,
		ArtifactSpec_LIFE_STONE,
		ArtifactSpec_CLARITY_STONE,
		ArtifactSpec_TACHYON_STONE_FRAGMENT,
		ArtifactSpec_DILITHIUM_STONE_FRAGMENT,
		ArtifactSpec_SHELL_STONE_FRAGMENT,
		ArtifactSpec_LUNAR_STONE_FRAGMENT,
		ArtifactSpec_SOUL_STONE_FRAGMENT,
		ArtifactSpec_PROPHECY_STONE_FRAGMENT,
		ArtifactSpec_QUANTUM_STONE_FRAGMENT,
		ArtifactSpec_TERRA_STONE_FRAGMENT,
		ArtifactSpec_LIFE_STONE_FRAGMENT,
		ArtifactSpec_CLARITY_STONE_FRAGMENT:
		return true
	default:
		return false
	}
}

// DescribeArtifactSetWithStones returns human-readable labels for artifacts and their slotted stones.
func DescribeArtifactSetWithStones(artifacts []*CompleteArtifact) []string {
	descriptions := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		if artifact == nil || artifact.GetSpec() == nil {
			continue
		}

		host := formatArtifactSpecLabel(artifact.GetSpec(), true)
		stones := artifact.GetStones()
		if len(stones) == 0 {
			descriptions = append(descriptions, host)
			continue
		}

		stoneLabels := make([]string, 0, len(stones))
		for _, stone := range stones {
			if stone == nil {
				continue
			}
			stoneLabels = append(stoneLabels, formatArtifactSpecLabel(stone, false))
		}

		if len(stoneLabels) == 0 {
			descriptions = append(descriptions, host)
		} else {
			descriptions = append(descriptions, fmt.Sprintf("%s [%s]", host, strings.Join(stoneLabels, ", ")))
		}
	}

	return descriptions
}

func formatArtifactSpecLabel(spec *ArtifactSpec, includeRarity bool) string {
	if spec == nil {
		return "unknown"
	}

	name := spec.GetName().String()
	if short, ok := ShortArtifactName[int32(spec.GetName())]; ok && short != "" {
		name = short
	}

	level := fmt.Sprintf("L%d", spec.GetLevel())
	if int(spec.GetLevel()) >= 0 && int(spec.GetLevel()) < len(ArtifactLevels) {
		level = ArtifactLevels[spec.GetLevel()]
	}

	if !includeRarity {
		return fmt.Sprintf("%s-%s", name, level)
	}

	rarity := ""
	if int(spec.GetRarity()) >= 0 && int(spec.GetRarity()) < len(ArtifactRarity) {
		rarity = ArtifactRarity[spec.GetRarity()]
	}

	if rarity == "" {
		return fmt.Sprintf("%s-%s", name, level)
	}
	return fmt.Sprintf("%s-%s%s", name, level, rarity)
}
