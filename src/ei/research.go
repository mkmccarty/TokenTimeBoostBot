package ei

import "math"

// Vehicles

type vehicleType struct {
	ID           uint32
	Name         string
	BaseCapacity float64 // Unupgraded shipping capacity per second.
}

var vehicleTypes = map[uint32]vehicleType{
	0: {
		ID:           0,
		Name:         "Trike",
		BaseCapacity: 5e3,
	},
	1: {
		ID:           1,
		Name:         "Transit Van",
		BaseCapacity: 15e3,
	},
	2: {
		ID:           2,
		Name:         "Pickup",
		BaseCapacity: 50e3,
	},
	3: {
		ID:           3,
		Name:         "10 Foot",
		BaseCapacity: 100e3,
	},
	4: {
		ID:           4,
		Name:         "24 Foot",
		BaseCapacity: 250e3,
	},
	5: {
		ID:           5,
		Name:         "Semi",
		BaseCapacity: 500e3,
	},
	6: {
		ID:           6,
		Name:         "Double Semi",
		BaseCapacity: 1e6,
	},
	7: {
		ID:           7,
		Name:         "Future Semi",
		BaseCapacity: 5e6,
	},
	8: {
		ID:           8,
		Name:         "Mega Semi",
		BaseCapacity: 15e6,
	},
	9: {
		ID:           9,
		Name:         "Hover Semi",
		BaseCapacity: 30e6,
	},
	10: {
		ID:           10,
		Name:         "Quantum Transporter",
		BaseCapacity: 50e6,
	},
	11: {
		ID:           11,
		Name:         "Hyperloop Train",
		BaseCapacity: 50e6,
	},
}

func isHoverVehicle(vehicle vehicleType) bool {
	return vehicle.ID >= 9
}

func isHyperloop(vehicle vehicleType) bool {
	return vehicle.ID == 11
}

// GetVehiclesShippingCapacity calculates the total shipping capacity of the user's vehicles
func GetVehiclesShippingCapacity(vehicles []uint32, trainLength []uint32, univMult float64, hoverOnlyMult float64, hyperOnlyMult float64) (float64, string) {
	userShippingCap := 0.0
	shippingNote := ""
	fullyUpgraded := true

	for i, v := range vehicles {
		if v == 0 {
			continue
		}
		vehicleType := vehicleTypes[v]
		capacity := vehicleType.BaseCapacity
		if vehicleType.ID != 11 && trainLength[i] != 10 {
			fullyUpgraded = false
		}
		if isHoverVehicle(vehicleType) {
			capacity *= hoverOnlyMult
		}
		capacity *= univMult
		if isHyperloop(vehicleType) {
			capacity *= hyperOnlyMult
			if trainLength[i] > 0 {
				lengthOfOneTrain := trainLength[i]
				capacity *= float64(lengthOfOneTrain)
			}
		}

		userShippingCap += capacity

	}
	if !fullyUpgraded {
		shippingNote = "Vehicles not fully upgraded"
	}

	return userShippingCap, shippingNote
}

// Hab structure for hab Data
type Hab struct {
	ID           int
	Name         string
	IconPath     string
	BaseCapacity float64
}

// Habs is a list of all habs in the game
var Habs = []Hab{
	{
		ID:           0,
		Name:         "Coop",
		IconPath:     "egginc/ei_hab_icon_coop.png",
		BaseCapacity: 250,
	},
	{
		ID:           1,
		Name:         "Shack",
		IconPath:     "egginc/ei_hab_icon_shack.png",
		BaseCapacity: 500,
	},
	{
		ID:           2,
		Name:         "Super Shack",
		IconPath:     "egginc/ei_hab_icon_super_shack.png",
		BaseCapacity: 1e3,
	},
	{
		ID:           3,
		Name:         "Short House",
		IconPath:     "egginc/ei_hab_icon_short_house.png",
		BaseCapacity: 2e3,
	},
	{
		ID:           4,
		Name:         "The Standard",
		IconPath:     "egginc/ei_hab_icon_the_standard.png",
		BaseCapacity: 5e3,
	},
	{
		ID:           5,
		Name:         "Long House",
		IconPath:     "egginc/ei_hab_icon_long_house.png",
		BaseCapacity: 1e4,
	},
	{
		ID:           6,
		Name:         "Double Decker",
		IconPath:     "egginc/ei_hab_icon_double_decker.png",
		BaseCapacity: 2e4,
	},
	{
		ID:           7,
		Name:         "Warehouse",
		IconPath:     "egginc/ei_hab_icon_warehouse.png",
		BaseCapacity: 5e4,
	},
	{
		ID:           8,
		Name:         "Center",
		IconPath:     "egginc/ei_hab_icon_center.png",
		BaseCapacity: 1e5,
	},
	{
		ID:           9,
		Name:         "Bunker",
		IconPath:     "egginc/ei_hab_icon_bunker.png",
		BaseCapacity: 2e5,
	},
	{
		ID:           10,
		Name:         "Eggkea",
		IconPath:     "egginc/ei_hab_icon_eggkea.png",
		BaseCapacity: 5e5,
	},
	{
		ID:           11,
		Name:         "HAB 1000",
		IconPath:     "egginc/ei_hab_icon_hab1k.png",
		BaseCapacity: 1e6,
	},
	{
		ID:           12,
		Name:         "Hangar",
		IconPath:     "egginc/ei_hab_icon_hanger.png",
		BaseCapacity: 2e6,
	},
	{
		ID:           13,
		Name:         "Tower",
		IconPath:     "egginc/ei_hab_icon_tower.png",
		BaseCapacity: 5e6,
	},
	{
		ID:           14,
		Name:         "HAB 10,000",
		IconPath:     "egginc/ei_hab_icon_hab10k.png",
		BaseCapacity: 1e7,
	},
	{
		ID:           15,
		Name:         "Eggtopia",
		IconPath:     "egginc/ei_hab_icon_eggtopia.png",
		BaseCapacity: 2.5e7,
	},
	{
		ID:           16,
		Name:         "Monolith",
		IconPath:     "egginc/ei_hab_icon_monolith.png",
		BaseCapacity: 5e7,
	},
	{
		ID:           17,
		Name:         "Planet Portal",
		IconPath:     "egginc/ei_hab_icon_portal.png",
		BaseCapacity: 1e8,
	},
	{
		ID:           18,
		Name:         "Chicken Universe",
		IconPath:     "egginc/ei_hab_icon_chicken_universe.png",
		BaseCapacity: 6e8,
	},
}

// IsPortalHab returns true if the hab is a portal hab
func IsPortalHab(hab Hab) bool {
	return hab.ID >= 17
}

// GetEpicResearchLayRate calculates the egg laying rate multiplier from epic research
func GetEpicResearchLayRate(epicResearch []*Backup_ResearchItem) float64 {
	userLayRate := 1.0
	for _, er := range epicResearch {
		switch er.GetId() {
		case "epic_egg_laying": // 20
			userLayRate *= (1 + 0.05*float64(er.GetLevel())) // Epic Egg Laying 5%
		}
	}
	return userLayRate
}

// GetEpicResearchShippingRate calculates the shipping rate multiplier from epic research
func GetEpicResearchShippingRate(epicResearch []*Backup_ResearchItem) float64 {
	universalShippingMultiplier := 1.0
	for _, er := range epicResearch {
		switch er.GetId() {
		case "transportation_lobbyist": // 30
			universalShippingMultiplier *= (1 + 0.05*float64(er.GetLevel())) // Transportation Lobbyist 5%
		}
	}
	return universalShippingMultiplier
}

// GetCommonResearchLayRate calculates the egg laying rate multiplier from common research
func GetCommonResearchLayRate(commonResearch []*Backup_ResearchItem) float64 {
	userLayRate := 1.0
	for _, cr := range commonResearch {
		switch cr.GetId() {
		case "comfy_nests": // 50
			userLayRate *= (1 + 0.1*float64(cr.GetLevel())) // Comfortable Nests 10%
		case "hen_house_ac": // 50
			userLayRate *= (1 + 0.05*float64(cr.GetLevel())) // Hen House Expansion 5%
		case "improved_genetics": // 30
			userLayRate *= (1 + 0.15*float64(cr.GetLevel())) // Internal Hatcheries 15%
		case "time_compress": // 20
			userLayRate *= (1 + 0.1*float64(cr.GetLevel())) // Time Compression 10%
		case "timeline_diversion": // 50
			userLayRate *= (1 + 0.02*float64(cr.GetLevel())) // Timeline Diversion 2%
		case "relativity_optimization": // 10
			userLayRate *= (1 + 0.1*float64(cr.GetLevel())) // Relativity Optimization 10%
		}
	}
	return userLayRate
}

// GetCommonResearchShippingRate calculates the shipping rate multiplier from common research
func GetCommonResearchShippingRate(commonResearch []*Backup_ResearchItem) float64 {
	universalShippingMultiplier := 1.0
	for _, cr := range commonResearch {
		switch cr.GetId() {
		case "leafsprings": // 30
			universalShippingMultiplier *= (1 + 0.05*float64(cr.GetLevel())) // Leafsprings 5%
		case "lightweight_boxes": // 40
			universalShippingMultiplier *= (1 + 0.1*float64(cr.GetLevel())) // Lightweight Boxes 10%
		case "driver_training": // 30
			universalShippingMultiplier *= (1 + 0.05*float64(cr.GetLevel())) // Driver Training 5%
		case "super_alloy": // 50
			universalShippingMultiplier *= (1 + 0.05*float64(cr.GetLevel())) // Super Alloy 5%
		case "quantum_storage": // 20
			universalShippingMultiplier *= (1 + 0.05*float64(cr.GetLevel())) // Quantum Storage 5%
		case "dark_containment": // 25
			universalShippingMultiplier *= (1 + 0.05*float64(cr.GetLevel())) // Dark Containment 5%
		case "neural_net_refine": // 25
			universalShippingMultiplier *= (1 + 0.05*float64(cr.GetLevel())) // Neural Net Refine 5%
		}
	}
	return universalShippingMultiplier
}

// GetCommonResearchHoverOnlyMultiplier calculates the hover vehicle shipping multiplier from common research
func GetCommonResearchHoverOnlyMultiplier(commonResearch []*Backup_ResearchItem) float64 {
	hoverOnlyMultiplier := 1.0
	for _, cr := range commonResearch {
		if cr.GetId() == "hover_upgrades" {
			hoverOnlyMultiplier *= (1 + 0.05*float64(cr.GetLevel())) // Hover Upgrades 5%
		}
	}
	return hoverOnlyMultiplier
}

// GetCommonResearchHyperloopOnlyMultiplier calculates the hyperloop vehicle shipping multiplier from common research
func GetCommonResearchHyperloopOnlyMultiplier(commonResearch []*Backup_ResearchItem) float64 {
	hyperloopOnlyMultiplier := 1.0
	for _, cr := range commonResearch {
		if cr.GetId() == "hyper_portalling" {
			hyperloopOnlyMultiplier *= (1 + 0.05*float64(cr.GetLevel())) // Hyper Portalling 5%
		}
	}
	return hyperloopOnlyMultiplier
}

// GetCommonResearchHabCapacity calculates the universal hab capacity multiplier from common research
func GetCommonResearchHabCapacity(commonResearch []*Backup_ResearchItem) float64 {
	universalHabCapacity := 1.0
	for _, cr := range commonResearch {
		switch cr.GetId() {
		case "hab_capacity1": // 8
			universalHabCapacity *= (1.0 + 0.05*float64(cr.GetLevel())) // Hab Capacity 5%
		case "microlux": // 10
			universalHabCapacity *= (1.0 + 0.05*float64(cr.GetLevel())) // Microlux 5%
		case "grav_plating": // 25
			universalHabCapacity *= (1.0 + 0.02*float64(cr.GetLevel())) // Grav Plating 2%
		}
	}
	return universalHabCapacity
}

// GetCommonResearchPortalHabCapacity calculates the portal hab capacity multiplier from common research
func GetCommonResearchPortalHabCapacity(commonResearch []*Backup_ResearchItem) float64 {
	portalHabCapacity := 1.0
	for _, cr := range commonResearch {
		if cr.GetId() == "wormhole_dampening" {
			portalHabCapacity *= (1.0 + 0.02*float64(cr.GetLevel())) // Wormhole Dampening 2%
		}
	}
	return portalHabCapacity
}

// GetEggLayingRate calculates the egg laying rate multiplier
func GetEggLayingRate(farmInfo *PlayerFarmInfo) float64 {
	userLayRate := 1 / 30.0 // 1 chicken per 30 seconds

	userLayRate *= GetCommonResearchLayRate(farmInfo.GetCommonResearch())
	userLayRate *= GetEpicResearchLayRate(farmInfo.GetEpicResearch())

	universalHabCapacity := GetCommonResearchHabCapacity(farmInfo.GetCommonResearch())
	portalHabCapacity := GetCommonResearchPortalHabCapacity(farmInfo.GetCommonResearch())

	//userLayRate *= 3600 // convert to hr rate
	habPopulation := 0.0
	for _, hab := range farmInfo.GetHabPopulation() {
		habPopulation += float64(hab)
	}
	habCapacity := 0.0
	for _, hab := range farmInfo.GetHabCapacity() {
		habCapacity += float64(hab)
	}

	baseHab := 0.0
	for _, hab := range farmInfo.GetHabs() {
		// Values 1->18 for each of these
		value := 0.0
		if hab != 19 {
			value = float64(Habs[hab].BaseCapacity)
			if IsPortalHab(Habs[hab]) {
				value *= portalHabCapacity
			}
			value *= universalHabCapacity
		}
		baseHab += value
	}

	//userLayRate *= 3600 // convert to hr rate
	baseLayingRate := userLayRate * baseHab * 3600.0
	//as.baseLayingRate = userLayRate * min(habPopulation, as.baseHab) * 3600.0 / 1e15

	return baseLayingRate * colleggtibleELR * colleggtibleHab
}

// GetEggLayingRateFromBackup calculates the egg laying rate multiplier
func GetEggLayingRateFromBackup(farmInfo *Backup_Simulation, game *Backup_Game) float64 {
	userLayRate := 1 / 30.0 // 1 chicken per 30 seconds

	userLayRate *= GetCommonResearchLayRate(farmInfo.GetCommonResearch())
	userLayRate *= GetEpicResearchLayRate(game.GetEpicResearch())

	universalHabCapacity := GetCommonResearchHabCapacity(farmInfo.GetCommonResearch())
	portalHabCapacity := GetCommonResearchPortalHabCapacity(farmInfo.GetCommonResearch())

	//userLayRate *= 3600 // convert to hr rate
	habPopulation := 0.0
	for _, hab := range farmInfo.GetHabPopulation() {
		habPopulation += float64(hab)
	}

	baseHab := 0.0
	for _, hab := range farmInfo.GetHabs() {
		// Values 1->18 for each of these
		value := 0.0
		if hab != 19 {
			value = float64(Habs[hab].BaseCapacity)
			if IsPortalHab(Habs[hab]) {
				value *= portalHabCapacity
			}
			value *= universalHabCapacity
		}
		baseHab += value
	}

	//userLayRate *= 3600 // convert to hr rate
	baseLayingRate := userLayRate * baseHab * 3600.0
	//as.baseLayingRate = userLayRate * min(habPopulation, as.baseHab) * 3600.0 / 1e15

	return baseLayingRate * colleggtibleELR * colleggtibleHab
}

// GetShippingRate calculates the shipping rate multiplier
func GetShippingRate(farmInfo *PlayerFarmInfo) float64 {
	universalShippingMultiplier := 1.0

	universalShippingMultiplier *= GetCommonResearchShippingRate(farmInfo.GetCommonResearch())
	universalShippingMultiplier *= GetEpicResearchShippingRate(farmInfo.GetEpicResearch())

	hoverOnlyMultiplier := GetCommonResearchHoverOnlyMultiplier(farmInfo.GetCommonResearch())
	hyperloopOnlyMultiplier := GetCommonResearchHyperloopOnlyMultiplier(farmInfo.GetCommonResearch())

	userShippingRate, _ := GetVehiclesShippingCapacity(farmInfo.GetVehicles(), farmInfo.GetTrainLength(), universalShippingMultiplier, hoverOnlyMultiplier, hyperloopOnlyMultiplier)

	return userShippingRate * colleggtibleShip * 60
}

func GetShippingRateFromBackup(farmInfo *Backup_Simulation, game *Backup_Game) float64 {
	universalShippingMultiplier := 1.0

	universalShippingMultiplier *= GetCommonResearchShippingRate(farmInfo.GetCommonResearch())
	universalShippingMultiplier *= GetEpicResearchShippingRate(game.GetEpicResearch())

	hoverOnlyMultiplier := GetCommonResearchHoverOnlyMultiplier(farmInfo.GetCommonResearch())
	hyperloopOnlyMultiplier := GetCommonResearchHyperloopOnlyMultiplier(farmInfo.GetCommonResearch())

	userShippingRate, _ := GetVehiclesShippingCapacity(farmInfo.GetVehicles(), farmInfo.GetTrainLength(), universalShippingMultiplier, hoverOnlyMultiplier, hyperloopOnlyMultiplier)

	return userShippingRate * colleggtibleShip * 60
}

// GetResearchInternalHatchery calculates the internal hatchery rate multiplier from common research
func GetResearchInternalHatchery(commonResearch []*Backup_ResearchItem) (float64, float64, float64) {
	internalHatcheryOnline := 1.0
	internalHatcheryOffline := 1.0
	internalHatcheryAdditive := 1.0
	for _, cr := range commonResearch {
		switch cr.GetId() {
		case "internal_hatchery1":
			internalHatcheryAdditive += (2 * float64(cr.GetLevel())) // Internal Hatcheries 2%
		case "internal_hatchery2":
			internalHatcheryAdditive += (5 * float64(cr.GetLevel())) // Internal Hatchery Upgrades 5%
		case "internal_hatchery3":
			internalHatcheryAdditive += (10 * float64(cr.GetLevel())) // Internal Hatchery Expansion 10%
		case "internal_hatchery4":
			internalHatcheryAdditive += (25 * float64(cr.GetLevel())) // Internal Hatchery Expansion 25%
		case "internal_hatchery5":
			internalHatcheryAdditive += (5 * float64(cr.GetLevel())) // Machine Learning Incubators 5%
		case "neural_linking":
			internalHatcheryAdditive += (50 * float64(cr.GetLevel())) // Neural Linking 50%
		case "epic_internal_incubators":
			internalHatcheryOnline += (1.0 + 0.05*float64(cr.GetLevel())) // Epic Internal Incubators 5%
		case "int_hatch_calm":
			internalHatcheryOffline += (1.0 + 0.1*float64(cr.GetLevel())) // Epic Internal Incubators 10%
		}
	}
	return internalHatcheryOnline, internalHatcheryOffline, internalHatcheryAdditive
}

// GetInternalHatcheryFromBackup calculates the internal hatchery rate multiplier
func GetInternalHatcheryFromBackup(commonResearch []*Backup_ResearchItem, game *Backup_Game, artifacts []*ArtifactInventoryItem, modifier float64, truthEggs uint32) (float64, float64, float64, float64) {

	baseRate := 0.0
	artifactsMultiplier := 1.0

	_, _, hatcheryAdditive := GetResearchInternalHatchery(commonResearch)
	onlineMultiplier, offlineMultiplier, _ := GetResearchInternalHatchery(game.GetEpicResearch())

	baseRate += hatcheryAdditive

	truthEggBonus := math.Pow(1.1, float64(truthEggs)) // 10% per truth egg

	// artifactsMultiplier = internalHatcheryRateMultiplier(artifacts);

	// With max internal hatchery sharing, four internal hatcheries are constantly
	// at work even if not all habs are bought;
	onlineRatePerHab := baseRate * onlineMultiplier * artifactsMultiplier * modifier * truthEggBonus * colleggtiblesIHR
	onlineRate := 4 * onlineRatePerHab
	offlineRatePerHab := onlineRatePerHab * offlineMultiplier * colleggtiblesIHR
	offlineRate := onlineRate * offlineMultiplier

	return onlineRatePerHab, onlineRate, offlineRatePerHab, offlineRate
}
