package ei

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	colleggtibleReleaseDates = make(map[string]time.Time)
	firstContractDates       = make(map[string]time.Time)
	colleggtibleDatesMu      sync.RWMutex
)

// SetColleggtibleReleaseDate sets the first available date of a colleggtible egg after July 1, 2024.
func SetColleggtibleReleaseDate(eggID string, date time.Time) {
	colleggtibleDatesMu.Lock()
	if _, ok := firstContractDates[eggID]; !ok {
		firstContractDates[eggID] = date
	}
	colleggtibleDatesMu.Unlock()

	july1 := time.Date(2024, time.July, 1, 0, 0, 0, 0, time.UTC)
	if !date.After(july1) {
		return
	}
	colleggtibleDatesMu.Lock()
	defer colleggtibleDatesMu.Unlock()
	if _, ok := colleggtibleReleaseDates[eggID]; !ok {
		colleggtibleReleaseDates[eggID] = date
	}
}

// GetColleggtibleDimensionBuffsAt returns the DimensionBuffs available at the given date.
func GetColleggtibleDimensionBuffsAt(date time.Time) DimensionBuffs {
	buffs := newDimensionBuffsIdentity()

	colleggtibleDatesMu.RLock()
	defer colleggtibleDatesMu.RUnlock()

	for eggName, releaseDate := range colleggtibleReleaseDates {
		if !releaseDate.After(date) {
			customEgg, ok := CustomEggMap[eggName]
			if !ok {
				continue
			}
			value, ok := getDimensionValueForTier(customEgg.DimensionValue, len(customEgg.DimensionValue)-1)
			if !ok {
				continue
			}
			applyDimensionBuff(&buffs, customEgg.Dimension, value)
		}
	}

	return buffs
}

var colleggtibleBuffs = newDimensionBuffsIdentity()

// GetColleggtibleValues will return the current values of the 3 collectibles
func GetColleggtibleValues() (float64, float64, float64, float64) {
	return colleggtibleBuffs.ELR, colleggtibleBuffs.SR, colleggtibleBuffs.Hab, colleggtibleBuffs.IHR
}

// GetColleggtibleIHR will return the current value of the ELR collectible
func GetColleggtibleIHR() float64 {
	return colleggtibleBuffs.IHR
}

// GetColleggtibleDimensionBuffs returns all currently saved colleggtible buffs.
func GetColleggtibleDimensionBuffs() DimensionBuffs {
	return colleggtibleBuffs
}

// GetMaxColleggtibleHabModifier calculates the maximum theoretical Hab Capacity multiplier across all colleggtibles.
func GetMaxColleggtibleHabModifier() float64 {
	colleggtibleDatesMu.RLock()
	defer colleggtibleDatesMu.RUnlock()

	mod := 1.0
	for _, egg := range CustomEggMap {
		if egg != nil && egg.Dimension == GameModifier_HAB_CAPACITY && len(egg.DimensionValue) > 0 {
			mod *= egg.DimensionValue[len(egg.DimensionValue)-1]
		}
	}
	if mod == 1.0 {
		return 1.05
	}
	return mod
}

func applyDimensionBuff(buffs *DimensionBuffs, dimension GameModifier_GameDimension, value float64) {
	switch dimension {
	case GameModifier_EGG_LAYING_RATE:
		buffs.ELR *= value
	case GameModifier_SHIPPING_CAPACITY:
		buffs.SR *= value
	case GameModifier_HAB_CAPACITY:
		buffs.Hab *= value
	case GameModifier_INTERNAL_HATCHERY_RATE:
		buffs.IHR *= value
	case GameModifier_EARNINGS:
		buffs.Earnings *= value
	case GameModifier_AWAY_EARNINGS:
		buffs.AwayEarnings *= value
	case GameModifier_VEHICLE_COST:
		buffs.VehicleCost *= value
	case GameModifier_HAB_COST:
		buffs.HabCost *= value
	case GameModifier_RESEARCH_COST:
		buffs.ResearchDiscount *= value
	default:
	}
}

func getDimensionValueForTier(values []float64, tier int) (float64, bool) {
	if len(values) == 0 {
		return 0, false
	}
	if tier < 0 {
		tier = 0
	}
	if tier >= len(values) {
		tier = len(values) - 1
	}
	return values[tier], true
}

// SetColleggtibleValues will set the values of the 3 collectibles based on CustomEggMap
func SetColleggtibleValues() {
	buffs := newDimensionBuffsIdentity()

	for _, eggValue := range CustomEggMap {
		if eggValue == nil {
			continue
		}
		value, ok := getDimensionValueForTier(eggValue.DimensionValue, len(eggValue.DimensionValue)-1)
		if !ok {
			continue
		}
		applyDimensionBuff(&buffs, eggValue.Dimension, value)
	}

	colleggtibleBuffs = buffs
}

// GetColleggtibleBuffs calculates the total buffs from colleggtibles
func GetColleggtibleBuffs(contracts *MyContracts) DimensionBuffs {
	buffs := newDimensionBuffsIdentity()
	if contracts == nil {
		return buffs
	}

	eggCounts := make(map[string]float64)

	colleggtibleSizes := contracts.GetColleggtibleMaxFarmSizeReached()
	if len(colleggtibleSizes) > 0 {
		for _, s := range colleggtibleSizes {
			if s.GetEggId() != "" {
				eggCounts[s.GetEggId()] = s.GetMaxFarmSizeReached()
			}
		}
	} else {
		// Look in active and archived contracts for custom eggs
		for _, c := range append(contracts.GetArchive(), contracts.GetContracts()...) {
			contractID := c.GetContractIdentifier()
			if contractID == "" && c.GetContract() != nil {
				contractID = c.GetContract().GetIdentifier()
			} else if contractID == "" && c.GetEvaluation() != nil {
				contractID = c.GetEvaluation().GetContractIdentifier()
			}

			egg := ""
			if c.GetContract() != nil {
				egg = c.GetContract().GetCustomEggId()
			} else if contractID != "" {
				if contractInfo, ok := GetEggIncContract(contractID); ok {
					if contractInfo.Egg == int32(Egg_CUSTOM_EGG) {
						egg = contractInfo.EggName
					}
				}
			}
			if egg == "" {
				continue
			}
			farmSize := c.GetMaxFarmSizeReached()
			value := eggCounts[egg]
			if farmSize > value {
				eggCounts[egg] = farmSize
			}
		}
	}

	for eggName, eggValue := range eggCounts {
		if eggValue == 0 {
			continue
		}
		customEgg, ok := CustomEggMap[eggName]
		if !ok {
			continue
		}
		tier := 0
		if eggValue >= 1e10 {
			tier = 3
		} else if eggValue >= 1e9 {
			tier = 2
		} else if eggValue >= 1e8 {
			tier = 1
		} else if eggValue >= 1e7 {
			tier = 0
		} else {
			continue
		}

		value, ok := getDimensionValueForTier(customEgg.DimensionValue, tier)
		if !ok {
			continue
		}
		applyDimensionBuff(&buffs, customEgg.Dimension, value)
	}

	return buffs
}

// GetColleggtibleBuffsFromInfo calculates the total buffs from a PlayerColleggtibleInfo object
func GetColleggtibleBuffsFromInfo(info *PlayerColleggtibleInfo) DimensionBuffs {
	buffs := newDimensionBuffsIdentity()
	if info == nil {
		return buffs
	}
	for _, buff := range info.GetBuffs() {
		applyDimensionBuff(&buffs, buff.GetDimension(), buff.GetValue())
	}
	return buffs
}

type NewColleggtibleInfo struct {
	ID        string
	Name      string
	Dimension GameModifier_GameDimension
	MaxVal    float64
}

// GetNewDeliveryColleggtibles returns info on delivery-related (ELR, SR, Hab, IHR) colleggtibles introduced after the runTime.
// This includes colleggtibles whose first contract started after runTime,
// as well as colleggtibles that have never been in a contract (first contract isn't run).
func GetNewDeliveryColleggtibles(runTime time.Time) []NewColleggtibleInfo {
	colleggtibleDatesMu.RLock()
	defer colleggtibleDatesMu.RUnlock()

	var list []NewColleggtibleInfo
	for eggID, egg := range CustomEggMap {
		if egg == nil {
			continue
		}
		isDeliveryEgg := egg.Dimension == GameModifier_EGG_LAYING_RATE ||
			egg.Dimension == GameModifier_SHIPPING_CAPACITY ||
			egg.Dimension == GameModifier_HAB_CAPACITY ||
			egg.Dimension == GameModifier_INTERNAL_HATCHERY_RATE

		if !isDeliveryEgg {
			continue
		}
		fcDate, hasFc := firstContractDates[eggID]
		if !hasFc || fcDate.After(runTime) {
			name := egg.Name
			if name == "" {
				name = egg.ID
			}
			var maxVal float64
			if len(egg.DimensionValue) > 0 {
				maxVal = egg.DimensionValue[len(egg.DimensionValue)-1]
			}
			list = append(list, NewColleggtibleInfo{
				ID:        eggID,
				Name:      name,
				Dimension: egg.Dimension,
				MaxVal:    maxVal,
			})
		}
	}
	// Sort by Name to keep order deterministic
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})
	return list
}

// FormatBuffNumber formats a float into a clean decimal string, trimmed of trailing zeroes.
func FormatBuffNumber(val float64) string {
	rounded := math.Round(val*10000) / 10000
	s := strconv.FormatFloat(rounded, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

// FormatSingleColleggtibleValue formats an individual colleggtible tier's buff value.
func FormatSingleColleggtibleValue(dim GameModifier_GameDimension, val float64) string {
	switch dim {
	case GameModifier_AWAY_EARNINGS:
		if val >= 2.0 {
			return fmt.Sprintf("%sx", FormatBuffNumber(val))
		}
		if val > 1.0 {
			return fmt.Sprintf("+%s%%", FormatBuffNumber((val-1.0)*100))
		}
		return "1x"
	case GameModifier_HAB_COST, GameModifier_VEHICLE_COST, GameModifier_RESEARCH_COST:
		if val < 1.0 {
			return fmt.Sprintf("-%s%%", FormatBuffNumber((1.0-val)*100))
		}
		return "0%"
	default:
		if val > 1.0 {
			return fmt.Sprintf("+%s%%", FormatBuffNumber((val-1.0)*100))
		}
		return "+0%"
	}
}

// FormatDimensionBuff formats a combined dimension multiplier into a display string.
func FormatDimensionBuff(dim GameModifier_GameDimension, mult float64) string {
	switch dim {
	case GameModifier_AWAY_EARNINGS:
		if mult >= 2.0 {
			return fmt.Sprintf("%sx", FormatBuffNumber(mult))
		}
		if mult > 1.0 {
			return fmt.Sprintf("+%s%%", FormatBuffNumber((mult-1.0)*100))
		}
		return "1x"
	case GameModifier_HAB_COST, GameModifier_VEHICLE_COST, GameModifier_RESEARCH_COST:
		if mult < 1.0 {
			return fmt.Sprintf("-%s%%", FormatBuffNumber((1.0-mult)*100))
		}
		return "0%"
	default:
		if mult > 1.0 {
			return fmt.Sprintf("+%s%%", FormatBuffNumber((mult-1.0)*100))
		}
		return "+0%"
	}
}

// PlayerColleggtibleStatus represents a player's status for a single colleggtible egg.
type PlayerColleggtibleStatus struct {
	EggID              string
	Name               string
	Dimension          GameModifier_GameDimension
	DimensionName      string
	FarmSize           float64
	Tier               int // -1 = uncollected, 0 = 10M, 1 = 100M, 2 = 1B, 3 = 10B
	MaxTier            int
	CurrentValue       float64
	MaxValue           float64
	CurrentValueString string
	MaxValueString     string
	IsMaxed            bool
}

// DimensionBuffSummary represents the total buffs and colleggtibles in one game dimension.
type DimensionBuffSummary struct {
	Dimension     GameModifier_GameDimension
	DimensionName string
	CurrentMult   float64
	MaxMult       float64
	CurrentString string
	MaxString     string
	IsMaxed       bool
	Colleggtibles []PlayerColleggtibleStatus
}

// ColleggtiblesOverview summarizes all colleggtibles and dimensions for a player.
type ColleggtiblesOverview struct {
	TotalCount     int
	CollectedCount int
	MaxedCount     int
	Dimensions     []DimensionBuffSummary
	Buffs          DimensionBuffs
}

// GetColleggtiblesOverview aggregates colleggtible progress and dimension buffs for a player.
func GetColleggtiblesOverview(contracts *MyContracts, storedCollegg string) ColleggtiblesOverview {
	overview := ColleggtiblesOverview{
		Buffs: newDimensionBuffsIdentity(),
	}

	eggCounts := make(map[string]float64)

	if contracts != nil {
		colleggtibleSizes := contracts.GetColleggtibleMaxFarmSizeReached()
		for _, s := range colleggtibleSizes {
			if s.GetEggId() != "" {
				eggCounts[s.GetEggId()] = s.GetMaxFarmSizeReached()
			}
		}
		for _, c := range append(contracts.GetArchive(), contracts.GetContracts()...) {
			contractID := c.GetContractIdentifier()
			if contractID == "" && c.GetContract() != nil {
				contractID = c.GetContract().GetIdentifier()
			} else if contractID == "" && c.GetEvaluation() != nil {
				contractID = c.GetEvaluation().GetContractIdentifier()
			}

			egg := ""
			if c.GetContract() != nil {
				egg = c.GetContract().GetCustomEggId()
			} else if contractID != "" {
				if contractInfo, ok := GetEggIncContract(contractID); ok {
					if contractInfo.Egg == int32(Egg_CUSTOM_EGG) {
						egg = contractInfo.EggName
					}
				}
			}
			if egg == "" {
				continue
			}
			farmSize := c.GetMaxFarmSizeReached()
			if farmSize > eggCounts[egg] {
				eggCounts[egg] = farmSize
			}
		}
	}

	// Fallback to stored colleggtible names if contracts did not populate any
	if len(eggCounts) == 0 && strings.TrimSpace(storedCollegg) != "" {
		for _, raw := range strings.Split(storedCollegg, ",") {
			name := strings.TrimSpace(raw)
			if name == "" {
				continue
			}
			for _, egg := range CustomEggMap {
				if egg == nil {
					continue
				}
				if strings.EqualFold(egg.Name, name) || strings.EqualFold(egg.ID, name) {
					eggCounts[egg.ID] = 1e10
				}
			}
		}
	}

	// Order dimensions: Coop & Delivery first, then Farm & Economy
	dimensionOrder := []GameModifier_GameDimension{
		GameModifier_EGG_LAYING_RATE,
		GameModifier_SHIPPING_CAPACITY,
		GameModifier_HAB_CAPACITY,
		GameModifier_INTERNAL_HATCHERY_RATE,
		GameModifier_EARNINGS,
		GameModifier_AWAY_EARNINGS,
		GameModifier_RESEARCH_COST,
		GameModifier_HAB_COST,
		GameModifier_VEHICLE_COST,
	}

	dimEggs := make(map[GameModifier_GameDimension][]*EggIncCustomEgg)
	for _, egg := range CustomEggMap {
		if egg == nil || egg.ID == "" {
			continue
		}
		dimEggs[egg.Dimension] = append(dimEggs[egg.Dimension], egg)
	}

	var orderedDims []GameModifier_GameDimension
	seenDims := make(map[GameModifier_GameDimension]bool)
	for _, d := range dimensionOrder {
		if len(dimEggs[d]) > 0 {
			orderedDims = append(orderedDims, d)
			seenDims[d] = true
		}
	}
	for d := range dimEggs {
		if !seenDims[d] {
			orderedDims = append(orderedDims, d)
		}
	}

	for _, d := range orderedDims {
		eggs := dimEggs[d]
		sort.Slice(eggs, func(i, j int) bool {
			return eggs[i].Name < eggs[j].Name
		})

		currentMult := 1.0
		maxMult := 1.0
		var statusList []PlayerColleggtibleStatus

		for _, egg := range eggs {
			farmSize := eggCounts[egg.ID]
			if farmSize == 0 {
				for k, v := range eggCounts {
					if strings.EqualFold(k, egg.ID) || strings.EqualFold(k, egg.Name) {
						if v > farmSize {
							farmSize = v
						}
					}
				}
			}

			tier := -1
			if farmSize >= 1e10 {
				tier = 3
			} else if farmSize >= 1e9 {
				tier = 2
			} else if farmSize >= 1e8 {
				tier = 1
			} else if farmSize >= 1e7 {
				tier = 0
			}

			maxTier := len(egg.DimensionValue) - 1
			if maxTier < 0 {
				maxTier = 0
			}
			if tier > maxTier {
				tier = maxTier
			}

			currentVal := 1.0
			currentValStr := ""
			if tier >= 0 && tier < len(egg.DimensionValue) {
				currentVal = egg.DimensionValue[tier]
				currentValStr = FormatSingleColleggtibleValue(egg.Dimension, currentVal)
			}

			maxVal := 1.0
			maxValStr := ""
			if len(egg.DimensionValue) > 0 {
				maxVal = egg.DimensionValue[maxTier]
				maxValStr = FormatSingleColleggtibleValue(egg.Dimension, maxVal)
			}

			isMaxed := (tier == maxTier && maxTier >= 0 && tier >= 0)

			overview.TotalCount++
			if tier >= 0 {
				overview.CollectedCount++
			}
			if isMaxed {
				overview.MaxedCount++
			}

			statusList = append(statusList, PlayerColleggtibleStatus{
				EggID:              egg.ID,
				Name:               egg.Name,
				Dimension:          egg.Dimension,
				DimensionName:      GetGameDimensionString(egg.Dimension),
				FarmSize:           farmSize,
				Tier:               tier,
				MaxTier:            maxTier,
				CurrentValue:       currentVal,
				MaxValue:           maxVal,
				CurrentValueString: currentValStr,
				MaxValueString:     maxValStr,
				IsMaxed:            isMaxed,
			})

			currentMult *= currentVal
			maxMult *= maxVal
		}

		applyDimensionBuff(&overview.Buffs, d, currentMult)

		dimSummary := DimensionBuffSummary{
			Dimension:     d,
			DimensionName: GetGameDimensionString(d),
			CurrentMult:   currentMult,
			MaxMult:       maxMult,
			CurrentString: FormatDimensionBuff(d, currentMult),
			MaxString:     FormatDimensionBuff(d, maxMult),
			IsMaxed:       math.Abs(currentMult-maxMult) < 1e-6,
			Colleggtibles: statusList,
		}

		overview.Dimensions = append(overview.Dimensions, dimSummary)
	}

	return overview
}
