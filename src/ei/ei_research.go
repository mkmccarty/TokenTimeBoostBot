package ei

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"slices"
	"strings"
	"time"
)

// EggCostResearch holds the cost data for a research item
type EggCostResearch struct {
	ID        string
	Name      string
	Level     int
	Price     float64
	BestValue float64
	TimeToBuy time.Duration
}

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
	Gems           []float64 `json:"gems"`
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
func GetSiloMinutes(silosOwned uint32, epicResearch []*Backup_ResearchItem) uint32 {
	baseSiloMinutes := 60.0

	ids := []string{
		"silo_capacity",
	}

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
	internalHatcheryAdditive := 0.0

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
	fleetSize := 4.0

	ids := []string{
		"vehicle_reliablity",
		"excoskeletons",
		"traffic_management",
		"egg_loading_bots",
		"autonomous_vehicles",
	}
	return uint32(GetResearchGeneric(commonResearch, ids, fleetSize))
}

func isVehicleResearch(researchID string) bool {
	ids := []string{
		"vehicle_reliablity",
		"excoskeletons",
		"traffic_management",
		"egg_loading_bots",
		"autonomous_vehicles",
	}

	return slices.Contains(ids, researchID)
}

func isEggValue(researchID string) bool {
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
	return slices.Contains(ids, researchID)
}

func isShippingRate(researchID string) bool {
	ids := []string{
		"hover_upgrades",
		"hyper_portalling",
		"leafsprings",
		"lightweight_boxes",
		"driver_training",
		"super_alloy",
		"quantum_storage",
		"dark_containment",
		"neural_net_refine",
	}

	return slices.Contains(ids, researchID)
}

func isLayRate(researchID string) bool {
	ids := []string{
		"comfy_nests",
		"hen_house_ac",
		"improved_genetics",
		"time_compress",
		"timeline_diversion",
		"relativity_optimization",
	}

	return slices.Contains(ids, researchID)
}

func isHabCapacity(researchID string) bool {
	ids := []string{
		"hab_capacity1",
		"microlux",
		"grav_plating",
		"wormhole_dampening",
	}

	return slices.Contains(ids, researchID)
}

// GetTrainLength calculates the hyperloop train length from common research
func GetTrainLength(commonResearch []*Backup_ResearchItem) uint32 {
	trainLength := 5.0

	ids := []string{
		"micro_coupling",
	}

	return uint32(GetResearchGeneric(commonResearch, ids, trainLength))
}

// GetResearchDiscount calculates the research cost discount from common research
func GetResearchDiscount(commonResearch []*Backup_ResearchItem) float64 {
	researchDiscount := 1.0

	ids := []string{
		"cheaper_research",
	}
	return GetResearchGeneric(commonResearch, ids, researchDiscount)
}

// GatherCommonResearchCosts gathers the next 10 common research items to be purchased based on their gem costs
func GatherCommonResearchCosts(gemsOnHand float64, offlineRateHr float64, epicResearch []*Backup_ResearchItem, commonResearch []*Backup_ResearchItem, collDiscount float64, afxDiscount float64) string {
	epicResearchDiscount := GetResearchDiscount(epicResearch)
	var eggCostResearchs []*EggCostResearch
	var eggValueResearchs []*EggCostResearch
	var vehicleResearchs []*EggCostResearch
	var shippingRateResearchs []*EggCostResearch
	var layRateResearchs []*EggCostResearch
	var habCapacityResearchs []*EggCostResearch

	discounts := epicResearchDiscount * collDiscount * afxDiscount * currentResearchDiscountEvent

	var researchTierThreadholds = []uint32{0, 0, 30, 80, 160, 280, 400, 520, 650, 800, 980, 1185, 1390, 1655}

	totalResearchsCompleted := uint32(0)
	//effectTypeFactor := 1.0
	for i, item := range commonResearch {
		research := EggIncResearches[i]
		levels := uint32(research.Levels)
		totalResearchsCompleted += item.GetLevel()
		if totalResearchsCompleted >= researchTierThreadholds[research.Tier] {
			for level := item.GetLevel(); level < levels; level++ {

				gemprice := research.Gems[level]
				remainingCost := math.Max(0, (gemprice*discounts)-gemsOnHand)
				duration := remainingCost / offlineRateHr
				bestValue := research.PerLevel
				if level != 0 {
					bestValue = (float64(level+1) * research.PerLevel) / (float64(level) * research.PerLevel)
				}

				if duration > 96 {
					// If the duration is more than 4 days, skip this research item
					continue
				}
				researchItem := EggCostResearch{
					ID:        research.ID,
					Name:      research.Name,
					Level:     int(level + 1),
					Price:     gemprice * discounts,
					BestValue: bestValue,
					TimeToBuy: time.Duration(duration * float64(time.Hour)),
				}

				if isVehicleResearch(research.ID) {
					vehicleResearchs = append(vehicleResearchs, &researchItem)
				} else if isEggValue(research.ID) {
					eggValueResearchs = append(eggValueResearchs, &researchItem)
				} else if isShippingRate(research.ID) {
					shippingRateResearchs = append(shippingRateResearchs, &researchItem)
				} else if isLayRate(research.ID) {
					layRateResearchs = append(layRateResearchs, &researchItem)
				} else if isHabCapacity(research.ID) {
					habCapacityResearchs = append(habCapacityResearchs, &researchItem)
				} else {
					eggCostResearchs = append(eggCostResearchs, &researchItem)
				}
			}
		}
	}

	sortByBestValue := func(researches []*EggCostResearch) {
		slices.SortFunc(researches, func(a, b *EggCostResearch) int {
			if a.BestValue > b.BestValue {
				return -1
			} else if a.BestValue < b.BestValue {
				return 1
			}
			return 0
		})
	}

	sortByPrice := func(researches []*EggCostResearch) {
		slices.SortFunc(researches, func(a, b *EggCostResearch) int {
			if a.Price < b.Price {
				return -1
			} else if a.Price > b.Price {
				return 1
			}
			return 0
		})
	}

	sortByBestValue(vehicleResearchs)
	sortByBestValue(eggValueResearchs)
	sortByBestValue(layRateResearchs)
	sortByBestValue(shippingRateResearchs)
	sortByBestValue(habCapacityResearchs)

	// Sort this one by price for just cheap stuff
	sortByPrice(eggCostResearchs)

	// Loop through first 10 of eggValueResearchs so I can print debug info
	/*
		for i := 0; i < 10; i++ {
			if i < len(eggValueResearchs) {
				research := eggValueResearchs[i]
				fmt.Printf("Debug: Egg Value Research %d: %+v\n", i+1, research)
			}
		}
	*/

	var builder strings.Builder
	// Print the next 10 researches to do
	researchCategories := []struct {
		name     string
		research []*EggCostResearch
	}{
		{"Value", eggValueResearchs},
		{"Vehicle", vehicleResearchs},
		{"ELR", layRateResearchs},
		{"SR", shippingRateResearchs},
		{"Hab", habCapacityResearchs},
		{"Cheap", eggCostResearchs},
	}

	for _, category := range researchCategories {
		if len(category.research) > 0 {
			builder.WriteString("-# **" + category.name + ":** ")
			if len(category.research) == 1 {
				research := category.research[0]
				fmt.Fprintf(&builder, "%s: %s\n", research.Name, FormatEIValue(research.Price, map[string]any{"decimals": 3, "trim": true}))
			} else if len(category.research) >= 2 {
				r1 := category.research[0]
				r2 := category.research[1]
				if r1.Name == r2.Name {
					fmt.Fprintf(&builder, "%s: %s, %s\n", r1.Name,
						FormatEIValue(r1.Price, map[string]any{"decimals": 2, "trim": true}),
						FormatEIValue(r2.Price, map[string]any{"decimals": 2, "trim": true}))
				} else {
					fmt.Fprintf(&builder, "%s: %s, %s: %s\n",
						r1.Name, FormatEIValue(r1.Price, map[string]any{"decimals": 2, "trim": true}),
						r2.Name, FormatEIValue(r2.Price, map[string]any{"decimals": 2, "trim": true}))
				}
			}
		}
	}
	header := ""
	if builder.Len() > 0 {
		// If we have any research info I want to add a string to the beginning of this string
		header = "-# **Next Common Research to Purchase:**\n"
	}

	return header + builder.String()
}
