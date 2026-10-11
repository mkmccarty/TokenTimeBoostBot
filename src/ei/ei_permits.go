package ei

// Standard and Pro Permit limits in Egg, Inc.
const (
	StandardPermitLevel = uint32(0)
	ProPermitLevel      = uint32(1)

	StandardPermitMaxArtifactSlots = 2
	ProPermitMaxArtifactSlots      = 4

	StandardPermitMaxActiveBoosts = 2
	ProPermitMaxActiveBoosts      = 5

	StandardPermitMaxSilos = 2
	ProPermitMaxSilos      = 10

	StandardPermitMaxFuelTankLevel = 0
	ProPermitMaxFuelTankLevel      = 7
)

// PermitLimits defines limits and capabilities for a permit type in Egg, Inc.
type PermitLimits struct {
	PermitLevel         uint32  `json:"permit_level"`
	Name                string  `json:"name"`
	MaxArtifactSlots    int     `json:"max_artifact_slots"`
	MaxActiveBoosts     int     `json:"max_active_boosts"`
	MaxSilos            int     `json:"max_silos"`
	MaxFuelTankLevel    int     `json:"max_fuel_tank_level"`
	MaxFuelTankCapacity float64 `json:"max_fuel_tank_capacity"`
}

// StandardPermitLimits contains the limits for Standard Permit players.
var StandardPermitLimits = PermitLimits{
	PermitLevel:         StandardPermitLevel,
	Name:                "Standard Permit",
	MaxArtifactSlots:    StandardPermitMaxArtifactSlots,
	MaxActiveBoosts:     StandardPermitMaxActiveBoosts,
	MaxSilos:            StandardPermitMaxSilos,
	MaxFuelTankLevel:    StandardPermitMaxFuelTankLevel,
	MaxFuelTankCapacity: FuelTankCapacities[StandardPermitMaxFuelTankLevel], // 2B
}

// ProPermitLimits contains the limits for Pro Permit players.
var ProPermitLimits = PermitLimits{
	PermitLevel:         ProPermitLevel,
	Name:                "Pro Permit",
	MaxArtifactSlots:    ProPermitMaxArtifactSlots,
	MaxActiveBoosts:     ProPermitMaxActiveBoosts,
	MaxSilos:            ProPermitMaxSilos,
	MaxFuelTankLevel:    ProPermitMaxFuelTankLevel,
	MaxFuelTankCapacity: FuelTankCapacities[ProPermitMaxFuelTankLevel], // 500T
}

// HasProPermit returns true if the player has the Pro Permit (permit level == 1).
func HasProPermit(backup *Backup) bool {
	if backup == nil || backup.GetGame() == nil {
		return false
	}
	return backup.GetGame().GetPermitLevel() == ProPermitLevel
}

// GetPermitLimits returns the PermitLimits applicable to the player backup.
func GetPermitLimits(backup *Backup) PermitLimits {
	if HasProPermit(backup) {
		return ProPermitLimits
	}
	return StandardPermitLimits
}

// GetPermitLimitsByLevel returns the PermitLimits for a given permit level.
func GetPermitLimitsByLevel(permitLevel uint32) PermitLimits {
	if permitLevel == ProPermitLevel {
		return ProPermitLimits
	}
	return StandardPermitLimits
}

// GetMaxEquippedArtifacts returns the maximum number of equipped artifact slots (2 for Standard, 4 for Pro).
func GetMaxEquippedArtifacts(backup *Backup) int {
	return GetPermitLimits(backup).MaxArtifactSlots
}

// GetMaxActiveBoosts returns the maximum number of active boost slots (2 for Standard, 5 for Pro).
func GetMaxActiveBoosts(backup *Backup) int {
	return GetPermitLimits(backup).MaxActiveBoosts
}

// GetMaxSilos returns the maximum number of silos allowed (2 for Standard, 10 for Pro).
func GetMaxSilos(backup *Backup) int {
	return GetPermitLimits(backup).MaxSilos
}

// GetMaxSilosByPermitLevel returns the maximum number of silos for a given permit level.
func GetMaxSilosByPermitLevel(permitLevel uint32) int {
	return GetPermitLimitsByLevel(permitLevel).MaxSilos
}

// GetEffectiveSilos clamps silosOwned to the maximum silos allowed by the player's permit.
func GetEffectiveSilos(backup *Backup, silosOwned uint32) uint32 {
	maxSilos := uint32(GetMaxSilos(backup))
	if silosOwned > maxSilos {
		return maxSilos
	}
	return silosOwned
}

// GetPlayerFuelTankLevel returns the effective fuel tank level for a player from their backup.
// Players with a Standard Permit (permit level != 1) are limited to level 0.
func GetPlayerFuelTankLevel(backup *Backup) int {
	if backup == nil {
		return 0
	}
	// Players with a Standard Permit are limited to a level 0 fuel tank.
	if backup.GetGame() != nil && backup.GetGame().GetPermitLevel() != ProPermitLevel {
		return 0
	}
	tankLevel := 0
	if backup.GetArtifacts() != nil {
		tankLevel = int(backup.GetArtifacts().GetTankLevel())
	}
	if tankLevel < 0 {
		return 0
	}
	maxLevel := ProPermitMaxFuelTankLevel
	if !HasProPermit(backup) && backup.GetGame() != nil {
		maxLevel = StandardPermitMaxFuelTankLevel
	}
	if tankLevel > maxLevel {
		return maxLevel
	}
	return tankLevel
}

// GetPlayerFuelTankCapacity returns the effective fuel tank capacity for a player.
// Players with a Standard Permit (permit level != 1) are limited to a level 0 tank (2 billion eggs).
func GetPlayerFuelTankCapacity(backup *Backup) float64 {
	level := GetPlayerFuelTankLevel(backup)
	return GetFuelTankCapacity(level)
}
