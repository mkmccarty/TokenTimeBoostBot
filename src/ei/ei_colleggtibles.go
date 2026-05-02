package ei

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

	// Look in active and archived contracts for custom eggs
	for _, c := range append(contracts.GetArchive(), contracts.GetContracts()...) {
		egg := c.GetContract().GetCustomEggId()
		if egg == "" {
			continue
		}
		farmSize := c.GetMaxFarmSizeReached()
		value := eggCounts[egg]
		if farmSize > value {
			eggCounts[egg] = farmSize
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
