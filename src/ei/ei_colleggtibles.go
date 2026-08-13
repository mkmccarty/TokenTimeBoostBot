package ei

import (
	"sort"
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
