package ei

var colleggtibleELR = 1.0
var colleggtibleShip = 1.0
var colleggtibleHab = 1.0
var colleggtiblesIHR = 1.0

// GetColleggtibleValues will return the current values of the 3 collectibles
func GetColleggtibleValues() (float64, float64, float64) {
	return colleggtibleELR, colleggtibleShip, colleggtibleHab
}

// GetColleggtibleIHR will return the current value of the ELR collectible
func GetColleggtibleIHR() float64 {
	return colleggtiblesIHR
}

// SetColleggtibleValues will set the values of the 3 collectibles based on CustomEggMap
func SetColleggtibleValues() {
	colELR := 1.0
	colShip := 1.0
	colHab := 1.0
	colIHR := 1.0

	for _, eggValue := range CustomEggMap {
		switch eggValue.Dimension {
		case GameModifier_EGG_LAYING_RATE:
			colELR *= eggValue.DimensionValue[len(eggValue.DimensionValue)-1]
		case GameModifier_SHIPPING_CAPACITY:
			colShip *= eggValue.DimensionValue[len(eggValue.DimensionValue)-1]
		case GameModifier_HAB_CAPACITY:
			colHab *= eggValue.DimensionValue[len(eggValue.DimensionValue)-1]
		case GameModifier_INTERNAL_HATCHERY_RATE:
			colIHR *= eggValue.DimensionValue[len(eggValue.DimensionValue)-1]
		}
	}

	colleggtibleELR = colELR
	colleggtibleShip = colShip
	colleggtibleHab = colHab
	colleggtiblesIHR = colIHR
}

// GetColleggtibleBuffs calculates the total buffs from colleggtibles
func GetColleggtibleBuffs(contracts *MyContracts) DimensionBuffs {
	colELR := 1.0
	colSR := 1.0
	colIHR := 1.0
	colHab := 1.0
	colEarnings := 1.0
	colAway := 1.0

	eggCounts := make(map[string]float64)

	for _, c := range contracts.GetArchive() {
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
		customEgg := CustomEggMap[eggName]
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

		switch customEgg.Dimension {
		case GameModifier_EGG_LAYING_RATE:
			colELR *= customEgg.DimensionValue[tier]
		case GameModifier_SHIPPING_CAPACITY:
			colSR *= customEgg.DimensionValue[tier]
		case GameModifier_HAB_CAPACITY:
			colHab *= customEgg.DimensionValue[tier]
		case GameModifier_INTERNAL_HATCHERY_RATE:
			colIHR *= customEgg.DimensionValue[tier]
		case GameModifier_EARNINGS:
			colEarnings *= customEgg.DimensionValue[tier]
		case GameModifier_AWAY_EARNINGS:
			colAway *= customEgg.DimensionValue[tier]
		default:
		}
	}

	return DimensionBuffs{
		ELR:          colELR,
		SR:           colSR,
		IHR:          colIHR,
		Hab:          colHab,
		Earnings:     colEarnings,
		AwayEarnings: colAway,
	}
}
