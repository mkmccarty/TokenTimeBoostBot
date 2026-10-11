package ei

// FuelTankLevel defines properties for an Egg, Inc. Fuel Tank level.
// Reference: https://egg-inc.fandom.com/wiki/Fuel_Tank
type FuelTankLevel struct {
	Level             int     `json:"level"`
	Capacity          float64 `json:"capacity"`
	MaxOutputPerMin   float64 `json:"max_output_per_min"`
	UpgradeCostGE     uint64  `json:"upgrade_cost_ge"`
	TotalCumulativeGE uint64  `json:"total_cumulative_ge"`
	RequiresProPermit bool    `json:"requires_pro_permit"`
}

// FuelTankLevels lists the 8 Fuel Tank levels (0 through 7).
// Reference: https://egg-inc.fandom.com/wiki/Fuel_Tank
var FuelTankLevels = []FuelTankLevel{
	{Level: 0, Capacity: 2e9, MaxOutputPerMin: 300e6, UpgradeCostGE: 0, TotalCumulativeGE: 0, RequiresProPermit: false},
	{Level: 1, Capacity: 200e9, MaxOutputPerMin: 12e9, UpgradeCostGE: 50_000, TotalCumulativeGE: 50_000, RequiresProPermit: true},
	{Level: 2, Capacity: 10e12, MaxOutputPerMin: 180e9, UpgradeCostGE: 250_000, TotalCumulativeGE: 300_000, RequiresProPermit: true},
	{Level: 3, Capacity: 100e12, MaxOutputPerMin: 3e12, UpgradeCostGE: 1_000_000, TotalCumulativeGE: 1_300_000, RequiresProPermit: true},
	{Level: 4, Capacity: 200e12, MaxOutputPerMin: 4.5e12, UpgradeCostGE: 2_000_000, TotalCumulativeGE: 3_300_000, RequiresProPermit: true},
	{Level: 5, Capacity: 300e12, MaxOutputPerMin: 6e12, UpgradeCostGE: 3_000_000, TotalCumulativeGE: 6_300_000, RequiresProPermit: true},
	{Level: 6, Capacity: 400e12, MaxOutputPerMin: 7.5e12, UpgradeCostGE: 4_000_000, TotalCumulativeGE: 10_300_000, RequiresProPermit: true},
	{Level: 7, Capacity: 500e12, MaxOutputPerMin: 9e12, UpgradeCostGE: 5_000_000, TotalCumulativeGE: 15_300_000, RequiresProPermit: true},
}

// FuelTankCapacities contains the capacities in eggs for each level (0 through 7).
var FuelTankCapacities = []float64{
	2e9,
	200e9,
	10e12,
	100e12,
	200e12,
	300e12,
	400e12,
	500e12,
}

// GetFuelTankCapacities returns a slice of capacities for all tank levels.
func GetFuelTankCapacities() []float64 {
	caps := make([]float64, len(FuelTankCapacities))
	copy(caps, FuelTankCapacities)
	return caps
}

// GetFuelTankCapacity returns the capacity for a given tank level, clamped to the valid range.
func GetFuelTankCapacity(level int) float64 {
	if level < 0 {
		return FuelTankCapacities[0]
	}
	if level >= len(FuelTankCapacities) {
		return FuelTankCapacities[len(FuelTankCapacities)-1]
	}
	return FuelTankCapacities[level]
}

// GetFuelTankLevelInfo returns the FuelTankLevel struct for a given level, clamped to the valid range.
func GetFuelTankLevelInfo(level int) FuelTankLevel {
	if level < 0 {
		return FuelTankLevels[0]
	}
	if level >= len(FuelTankLevels) {
		return FuelTankLevels[len(FuelTankLevels)-1]
	}
	return FuelTankLevels[level]
}
