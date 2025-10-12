package ei

import (
	"encoding/json"
	"log"
	"math"
	"os"
	"slices"
)

// EggResearches holds the egg researches data
type EggResearches struct {
	SerialID       int       `json:"serial_id"`
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Type           string    `json:"type"`
	Tier           int       `json:"tier"`
	Categories     string    `json:"categories"`
	Description    string    `json:"description"`
	EffectType     string    `json:"effect_type"`
	Levels         int       `json:"levels"`
	PerLevel       float64   `json:"per_level"`
	LevelsCompound string    `json:"levels_compound"`
	Prices         []float64 `json:"prices"`
}

// EggIncResearches holds all the egg researches
var EggIncResearches []EggResearches

// EggIncResearchesMap maps research ID to EggResearches
var EggIncResearchesMap map[string]EggResearches

// LoadResearchData loads research data from a JSON file
func LoadResearchData(filename string) {

	var EggIncResearchesLoaded []EggResearches
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			// Handle the error appropriately, e.g., logging or taking corrective actions
			log.Printf("Failed to close: %v", err)
		}
	}()
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&EggIncResearchesLoaded)
	if err != nil {
		log.Print(err)
		return
	}

	// We need to modify a few values to make it easier to auto calculate things
	EggIncResearches = EggIncResearchesLoaded
	EggIncResearchesMap = make(map[string]EggResearches)
	for _, research := range EggIncResearches {
		EggIncResearchesMap[research.ID] = research
	}
}

// GetResearchGeneric multiplies the per-level effect for each research ID in ids found in commonResearch.
// It uses EggIncResearchesMap to look up per-level values.
func GetResearchGeneric(research []*Backup_ResearchItem, ids []string, baseValue float64) float64 {
	result := baseValue
	//additiveResult := 0.0
	for _, cr := range research {
		id := cr.GetId()
		if slices.Contains(ids, id) {
			researchData, ok := EggIncResearchesMap[id]
			if !ok {
				continue
			}
			switch researchData.EffectType {
			case "additive":
				switch researchData.LevelsCompound {
				case "additive":
					result += researchData.PerLevel * float64(cr.GetLevel())
				case "multiplicative":
					// I don't think this case exists in the game, but just in case
					result += math.Pow(researchData.PerLevel, float64(cr.GetLevel()))
				}
			case "multiplicative":
				switch researchData.LevelsCompound {
				case "additive":
					result *= (1 + researchData.PerLevel*float64(cr.GetLevel()))
				case "multiplicative":
					result *= math.Pow(researchData.PerLevel, float64(cr.GetLevel()))
				}
			}
		}
	}
	return result
}

// GetFarmEggValue returns the current egg value for the farm
func GetFarmEggValue(commonResearch []*Backup_ResearchItem) float64 {
	baseEggValue := 1.0

	ids := []string{
		"nutritional_sup",
		"padded_packaging",
		"bigger_eggs",
		"usde_prime",
		"superfeed",
		"improved_genetics",
		"shell_fortification",
		"even_bigger_eggs",
		"genetic_purification",
		"graviton_coating",
		"chrystal_shells",
		"telepathic_will",
		"atomic_purification",
		"multi_layering",
		"eggsistor",
		"matter_reconfig",
		"timeline_splicing",
	}
	return GetResearchGeneric(commonResearch, ids, baseEggValue)
}

// GetSiloMinutes calculates the total silo minutes from epic research and silos owned
func GetSiloMinutes(farmInfo *Backup_Simulation, epicResearch []*Backup_ResearchItem) uint32 {
	baseSiloMinutes := 60.0

	ids := []string{
		"silo_capacity",
	}

	silosOwned := farmInfo.GetSilosOwned()
	result := uint32(GetResearchGeneric(epicResearch, ids, baseSiloMinutes) * float64(silosOwned))
	return result
}

// GetCommonResearchHoverOnlyMultiplier calculates the hover vehicle shipping multiplier from common research
func GetCommonResearchHoverOnlyMultiplier(commonResearch []*Backup_ResearchItem) float64 {
	ids := []string{
		"hover_upgrades",
	}
	return GetResearchGeneric(commonResearch, ids, 1.0)
}

// GetCommonResearchHyperloopOnlyMultiplier calculates the hyperloop vehicle shipping multiplier from common research
func GetCommonResearchHyperloopOnlyMultiplier(commonResearch []*Backup_ResearchItem) float64 {
	ids := []string{
		"hyper_portalling",
	}
	return GetResearchGeneric(commonResearch, ids, 1.0)
}

// GetResearchInternalHatchery calculates the internal hatchery rate multiplier from common research
func GetResearchInternalHatchery(commonResearch []*Backup_ResearchItem) (float64, float64, float64) {
	internalHatcheryOnline := 1.0
	internalHatcheryOffline := 1.0
	internalHatcheryAdditive := 1.0

	idsIHR := []string{
		"internal_hatchery1",
		"internal_hatchery2",
		"internal_hatchery3",
		"internal_hatchery4",
		"internal_hatchery5",
		"neural_linking",
	}
	internalHatcheryAdditive = GetResearchGeneric(commonResearch, idsIHR, internalHatcheryAdditive)
	idsOnline := []string{
		"epic_internal_incubators",
	}
	internalHatcheryOnline = GetResearchGeneric(commonResearch, idsOnline, internalHatcheryOnline)

	idsOffline := []string{
		"int_hatch_calm",
	}
	internalHatcheryOffline = GetResearchGeneric(commonResearch, idsOffline, internalHatcheryOffline)

	return internalHatcheryOnline, internalHatcheryOffline, internalHatcheryAdditive
}

// GetEpicResearchLayRate calculates the egg laying rate multiplier from epic research
func GetEpicResearchLayRate(epicResearch []*Backup_ResearchItem) float64 {
	userLayRate := 1.0

	ids := []string{
		"epic_egg_laying",
	}
	result := GetResearchGeneric(epicResearch, ids, userLayRate)
	return result
}

// GetEpicResearchShippingRate calculates the shipping rate multiplier from epic research
func GetEpicResearchShippingRate(epicResearch []*Backup_ResearchItem) float64 {
	universalShippingMultiplier := 1.0

	ids := []string{
		"transportation_lobbyist",
	}
	result := GetResearchGeneric(epicResearch, ids, universalShippingMultiplier)
	return result
}

// GetCommonResearchLayRate calculates the egg laying rate multiplier from common research
func GetCommonResearchLayRate(commonResearch []*Backup_ResearchItem) float64 {
	userLayRate := 1.0
	ids := []string{
		"comfy_nests",
		"hen_house_ac",
		"improved_genetics",
		"time_compress",
		"timeline_diversion",
		"relativity_optimization",
	}
	// First do the generic ones
	userLayRate = GetResearchGeneric(commonResearch, ids, userLayRate)

	return userLayRate
}

// GetCommonResearchShippingRate calculates the shipping rate multiplier from common research
func GetCommonResearchShippingRate(commonResearch []*Backup_ResearchItem) float64 {
	universalShippingMultiplier := 1.0

	ids := []string{
		"leafsprings",
		"lightweight_boxes",
		"driver_training",
		"super_alloy",
		"quantum_storage",
		"dark_containment",
		"neural_net_refine",
	}
	universalShippingMultiplier = GetResearchGeneric(commonResearch, ids, universalShippingMultiplier)

	return universalShippingMultiplier
}

// GetCommonResearchHabCapacity calculates the universal hab capacity multiplier from common research
func GetCommonResearchHabCapacity(commonResearch []*Backup_ResearchItem) float64 {
	universalHabCapacity := 1.0

	ids := []string{
		"hab_capacity1",
		"microlux",
		"grav_plating",
	}
	universalHabCapacity = GetResearchGeneric(commonResearch, ids, universalHabCapacity)

	return universalHabCapacity
}

// GetCommonResearchPortalHabCapacity calculates the portal hab capacity multiplier from common research
func GetCommonResearchPortalHabCapacity(commonResearch []*Backup_ResearchItem) float64 {
	portalHabCapacity := 1.0

	ids := []string{
		"wormhole_dampening",
	}
	return GetResearchGeneric(commonResearch, ids, portalHabCapacity)
}

// GetFleetSize calculates the vehicle fleet size from common research
func GetFleetSize(commonResearch []*Backup_ResearchItem) uint32 {
	fleetSize := 1.0

	ids := []string{
		"vehicle_reliablity",
		"excoskeletons",
		"traffic_management",
		"egg_loading_bots",
		"autonomous_vehicles",
	}
	return uint32(GetResearchGeneric(commonResearch, ids, fleetSize))

}

// GetTrainLength calculates the hyperloop train length from common research
func GetTrainLength(commonResearch []*Backup_ResearchItem) uint32 {
	trainLength := 1.0

	ids := []string{
		"micro_coupling",
	}

	return uint32(GetResearchGeneric(commonResearch, ids, trainLength))
}
