package boost

import (
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

var (
	activeBoostClientMu sync.RWMutex
	activeBoostClient   dc.Client
)

func init() {
	ei.OnCoopStatusCallback = handleCoopStatusForActiveContracts
}

func setActiveBoostClient(client dc.Client) {
	if client == nil {
		return
	}
	activeBoostClientMu.Lock()
	activeBoostClient = client
	activeBoostClientMu.Unlock()
}

func getActiveBoostClient() dc.Client {
	activeBoostClientMu.RLock()
	defer activeBoostClientMu.RUnlock()
	return activeBoostClient
}

// DynamicTokenData is a struct that holds the data needed to calculate dynamic tokens
type DynamicTokenData struct {
	TokenTimer int

	TE                    int64
	HabNumber             int64
	OfflineIHR            int64
	Name                  string
	ELR                   float64
	TokenBoost            [13]float64
	BoostTimeMinutes      [13]float64
	ChickenRunTimeMinutes [13]float64
	IhrBase               int64
	FourHabsOffline       int64
	MaxHab                float64
	ChickenRunHab         float64
	IHRMultiplier         float64
	ColleggtibleIHR       float64
}

// Returns an slice of integers for all contract players with needed token counts
func calculateDynamicTokens(dt *DynamicTokenData, count int, tpm float64, heldTokens int) []int {
	var retSlice []int

	log.Print("Calculating dynamic tokens for ", dt.Name, " for ", count, " players")

	log.Print("TPM: ", tpm)
	log.Print("Held Tokens: ", heldTokens)

	// Want the highest eggs at 120 minutes
	//eggsAt120 := boostedELR * (120 - timeToFinishBoostInMin) / 60

	// Data needed...
	// Time of start of contract
	// For Boosted - time of player boosts and tokens spent.
	// For Unboosted -  zero boost time and count of tokens on hand

	// 1. Get fixed values from those that already boosted
	// 2. Calculate the estimates using mix of token boosts

	// Add 6, 6, 6, 8 to retSlice
	retSlice = append(retSlice, 6, 6, 6, 8)

	// For every booster
	return retSlice
}

// getBoostTimeSeconds returns the boost time as a time.Duration for a given number of tokens
func getBoostTimeSeconds(dt *DynamicTokenData, tokens int) (time.Duration, time.Duration) {

	// This protects the parameters of the next function call
	if tokens < 0 {
		tokens = 0
	} else if tokens > len(dt.BoostTimeMinutes) {
		tokens = len(dt.BoostTimeMinutes) - 1
	}
	return time.Duration(dt.BoostTimeMinutes[tokens] * float64(time.Minute)), time.Duration(dt.ChickenRunTimeMinutes[tokens] * float64(time.Minute))
}

// createDynamicTokenData creates all the common underlying data for dynamic tokens
func createDynamicTokenData(TE int64) *DynamicTokenData {
	dt := new(DynamicTokenData)

	_, _, colleggtibleHab, colleggtiblesIHR := ei.GetColleggtibleValues()
	dt.ColleggtibleIHR = colleggtiblesIHR
	dt.HabNumber = 4
	dt.OfflineIHR = 3
	dt.TE = TE

	// Chickens per minute
	// Assumption is that the player has completed Epic and Common Research
	dt.IhrBase = 7440                                            // chickens/min/hab (without any artifacts)
	dt.IhrBase = int64(float64(dt.IhrBase) * dt.ColleggtibleIHR) // Apply colleggibles (5% from Easter Colleggtibles)
	dt.FourHabsOffline = dt.IhrBase * dt.HabNumber * dt.OfflineIHR

	// Assume: T4L Chalice (1.4), T4L Monocle (1.3), 9 Life stones (IHR stones = 1.04^9 ≈ 1.432)
	// IHR multiplier should NOT reapply colleggibles - those are already in IhrBase
	chickenRunPercent := 0.70 // Chicken run is 70.0% of normal boost time
	chaliceMultiplier := 1.4  // T4L Chalice
	monocleMultiplier := 1.3  // T4L Monocle, only for boosts
	ihrSlots := 9.0           // IHR stone slots
	dt.IHRMultiplier = chaliceMultiplier * math.Pow(1.04, ihrSlots) * math.Pow(1.01, float64(dt.TE))
	dt.MaxHab = 14_175_000_000.0 * colleggtibleHab
	dt.ChickenRunHab = dt.MaxHab * chickenRunPercent
	// Create boost times for 0 through 12 token boosts
	// 14.825K/min/hab (×1.993)
	for i := range len(dt.TokenBoost) {
		mult := calcBoostMulti(float64(i))
		dt.TokenBoost[i] = mult * monocleMultiplier
		ihr := float64(dt.TokenBoost[i]) * dt.IHRMultiplier * float64(dt.FourHabsOffline) // per minute
		// Minimum time is 1 minute due to away time calculation
		dt.BoostTimeMinutes[i] = max(1.0, float64(dt.MaxHab)/ihr)
		dt.ChickenRunTimeMinutes[i] = max(1.0, float64(dt.ChickenRunHab)/ihr)
	}
	return dt
}

func determineDynamicTokens(c *Contract) {

	if c == nil {
		return
	}

	if c.Style&ContractFlagDynamicTokens == 0 {
		return
	}
	// Determine the dynamic tokens
	// For everyone in the contract
	// 1. Determine boosted ELR
	// 2. Determine start boost time ane end boost time based on tokens IHR
	// 3. Determine how much fully boosted minutes at ELR (120 - minutes to boost)
	dt := createDynamicTokenData(50)

	// Initially assign 6 token boosts to everyone,
	// In reverse order start calculating using 8 token boosts
	// stop when the 120 minute delivered eggs amount is less than the previous amount

	tpm := float64(len(c.TokenLog)) / time.Since(c.StartTime).Minutes()

	calculateDynamicTokens(dt, 4, tpm, 0)

}

// BoostSchedule holds the calculated boost schedule for a player's boost run.
type BoostSchedule struct {
	Tokens                int
	BoostIcons            string
	TotalBoostDuration    time.Duration
	CRDuration            time.Duration
	PureCRDuration        time.Duration
	TruckChecks           int
	TruckWaitDuration     time.Duration
	PostCROfflineDuration time.Duration
	CRRequestPopulation   float64
	MaxShippingPopulation float64
	OfflineIHRPerMin      float64
	DeflectorPercent      float64
	DeflectorIsActual     bool
	DeliveryArtifacts     string
	TachyonStones         int
	QuantumStones         int
	LayRate               float64
	ShipRate              float64
	DeliveryRate          float64
	ShowSIABSet           bool
	SIABDeliveryArtifacts string
	SIABTachyonStones     int
	SIABQuantumStones     int
	SIABLayRate           float64
	SIABShipRate          float64
	SIABDeliveryRate      float64
	BoostsVerified        bool
	CurrentPopulation     float64
	PopulationBasis       string
	EstRequestChickenRuns time.Time
	EstEndOfBoost         time.Time
}

type boostItemSpec struct {
	emojiName string
	count     int
	duration  string
}

func boostEmojiMarkdown(name string) string {
	if md, ok := ei.GetBotEmojiMarkdownIfExists(name); ok {
		return md
	}
	return ":" + name + ":"
}

// getExpectedBoostIcons returns the formatted boost icons and their durations for a given token count.
// If the total boost duration exceeds 10 minutes, the 1-hour 1000x Tachyon Prism is selected over the 10-minute one.
func getExpectedBoostIcons(tokens int, boostDuration ...time.Duration) string {
	tach1000Emoji := "b_tach1000x10m"
	tach1000Dur := "10m"
	if len(boostDuration) > 0 && boostDuration[0] > 10*time.Minute {
		tach1000Emoji = "b_tach1000x1h"
		tach1000Dur = "1h"
	}

	var items []boostItemSpec
	switch {
	case tokens <= 0:
		items = []boostItemSpec{
			{emojiName: "b_tach10x4h", count: 5, duration: "4h"},
		}
	case tokens == 1:
		items = []boostItemSpec{
			{emojiName: "b_boost2x30m", count: 1, duration: "30m"},
			{emojiName: "b_tach10x4h", count: 4, duration: "4h"},
		}
	case tokens == 2:
		items = []boostItemSpec{
			{emojiName: "b_tach100x10m", count: 1, duration: "10m"},
			{emojiName: "b_tach10x4h", count: 4, duration: "4h"},
		}
	case tokens == 3:
		items = []boostItemSpec{
			{emojiName: "b_tach100x10m", count: 1, duration: "10m"},
			{emojiName: "b_boost2x30m", count: 1, duration: "30m"},
			{emojiName: "b_tach10x4h", count: 3, duration: "4h"},
		}
	case tokens == 4:
		items = []boostItemSpec{
			{emojiName: tach1000Emoji, count: 1, duration: tach1000Dur},
			{emojiName: "b_tach10x4h", count: 4, duration: "4h"},
		}
	case tokens == 5:
		items = []boostItemSpec{
			{emojiName: tach1000Emoji, count: 1, duration: tach1000Dur},
			{emojiName: "b_boost2x30m", count: 1, duration: "30m"},
			{emojiName: "b_tach10x4h", count: 3, duration: "4h"},
		}
	case tokens == 6:
		items = []boostItemSpec{
			{emojiName: tach1000Emoji, count: 1, duration: tach1000Dur},
			{emojiName: "b_boost2x30m", count: 2, duration: "30m"},
			{emojiName: "b_tach10x4h", count: 2, duration: "4h"},
		}
	case tokens == 7:
		items = []boostItemSpec{
			{emojiName: tach1000Emoji, count: 1, duration: tach1000Dur},
			{emojiName: "b_boost2x30m", count: 3, duration: "30m"},
			{emojiName: "b_tach10x4h", count: 1, duration: "4h"},
		}
	case tokens == 8:
		items = []boostItemSpec{
			{emojiName: tach1000Emoji, count: 1, duration: tach1000Dur},
			{emojiName: "b_boost10x10m", count: 1, duration: "10m"},
			{emojiName: "b_tach10x4h", count: 3, duration: "4h"},
		}
	case tokens == 9:
		items = []boostItemSpec{
			{emojiName: tach1000Emoji, count: 1, duration: tach1000Dur},
			{emojiName: "b_boost10x10m", count: 1, duration: "10m"},
			{emojiName: "b_boost2x30m", count: 1, duration: "30m"},
			{emojiName: "b_tach10x4h", count: 2, duration: "4h"},
		}
	case tokens == 10:
		items = []boostItemSpec{
			{emojiName: tach1000Emoji, count: 1, duration: tach1000Dur},
			{emojiName: "b_boost10x10m", count: 1, duration: "10m"},
			{emojiName: "b_boost2x30m", count: 2, duration: "30m"},
			{emojiName: "b_tach10x4h", count: 1, duration: "4h"},
		}
	case tokens == 11:
		items = []boostItemSpec{
			{emojiName: tach1000Emoji, count: 1, duration: tach1000Dur},
			{emojiName: "b_boost10x10m", count: 1, duration: "10m"},
			{emojiName: "b_boost2x30m", count: 3, duration: "30m"},
		}
	default:
		items = []boostItemSpec{
			{emojiName: tach1000Emoji, count: 1, duration: tach1000Dur},
			{emojiName: "b_boost50x10m", count: 1, duration: "10m"},
			{emojiName: "b_tach10x4h", count: 3, duration: "4h"},
		}
	}

	parts := make([]string, 0, len(items))
	for _, item := range items {
		icon := boostEmojiMarkdown(item.emojiName)
		parts = append(parts, fmt.Sprintf("%s (%s)", strings.Repeat(icon, item.count), item.duration))
	}
	return strings.Join(parts, "  ")
}

// calculateBoostSchedule computes a booster's personalized boost schedule based on their boost tokens,
// known artifacts, TE, and contract parameters.
func calculateBoostSchedule(contract *Contract, booster *Booster, now time.Time) BoostSchedule {
	tokens := 6
	if booster != nil {
		tokens = booster.TokensWanted
	}
	if tokens < 0 {
		tokens = 0
	}

	modSR := 1.0
	modELR := 1.0
	modHab := 1.0
	modIHR := 1.0
	numFarmers := 4.0
	showSIABSet := false

	if contract != nil {
		if contract.CoopSize > 0 {
			numFarmers = float64(contract.CoopSize)
		} else if len(contract.Order) > 0 {
			numFarmers = float64(len(contract.Order))
		}
		if c, ok := ei.EggIncContractsAll[contract.ContractID]; ok {
			if c.ModifierSR > 0 {
				modSR = c.ModifierSR
			}
			if c.ModifierELR > 0 {
				modELR = c.ModifierELR
			}
			if c.ModifierHabCap > 0 {
				modHab = c.ModifierHabCap
			}
			if c.ModifierIHR > 0 {
				modIHR = c.ModifierIHR
			}
			if contract.CoopSize <= 0 && c.MaxCoopSize > 0 {
				numFarmers = float64(c.MaxCoopSize)
			}
			if c.CxpMaxSiab > c.CxpMax {
				showSIABSet = true
			} else {
				gg, ugg, _ := ei.GetGenerousGiftEvent()
				if (gg > 1.0 || ugg > 1.0) && c.CxpMaxSiabGG > c.CxpMaxGG {
					showSIABSet = true
				}
			}
		}
	}

	colELR, colShip, colHab, colIHR := ei.GetColleggtibleValues()

	metronome := DefaultLeggyMetronome
	compass := DefaultLeggyCompass
	gusset := DefaultLeggyGusset
	deflStones := 2
	metrStones := 3
	compStones := 3
	gussStones := 2
	siabStones := 2
	deflQuality := "T4L"
	metrQuality := "T4L"
	compQuality := "T4L"
	gussQuality := "T4L"
	siabQuality := "T4L"
	hasDeliveryArt := false
	hasSIABArt := false

	chalice := DefaultLeggyChalice
	monocle := DefaultLeggyMonocle
	ihrSlots := int(DefaultLeggyIHRSlots)
	hasIHRArt := false

	var artifacts []ei.Artifact
	if booster != nil {
		artifacts = booster.ArtifactSet.Artifacts
		if len(artifacts) == 0 && booster.UserID != "" {
			as := getUserArtifacts(booster.UserID, nil)
			artifacts = as.Artifacts
		}
	}

	hasColleggArt := false
	customColELR := 1.0
	customColShip := 1.0
	customColHab := 1.0
	customColIHR := 1.0

	chaliceMap := map[string]float64{
		"T1C": 1.05,
		"T2C": 1.10, "T2E": 1.15,
		"T3C": 1.20, "T3R": 1.23, "T3E": 1.25,
		"T4C": 1.30, "T4E": 1.35, "T4L": 1.40,
		"NONE": 1.0,
	}
	monocleMap := map[string]float64{
		"T1C": 1.05,
		"T2C": 1.10,
		"T3C": 1.15,
		"T4C": 1.20, "T4E": 1.25, "T4L": 1.30,
		"NONE": 1.0,
	}

	customIHRSlots := 0
	customChalice := 1.0
	customMonocle := 1.0

	for _, a := range artifacts {
		switch a.Type {
		case "Deflector":
			hasDeliveryArt = true
			deflStones = a.Stones
			if a.Quality != "" {
				deflQuality = a.Quality
			}
		case "Metronome":
			hasDeliveryArt = true
			if a.LayBuff > 0 {
				metronome = a.LayBuff
			}
			metrStones = a.Stones
			if a.Quality != "" {
				metrQuality = a.Quality
			}
		case "Compass":
			hasDeliveryArt = true
			if a.ShipBuff > 0 {
				compass = a.ShipBuff
			}
			compStones = a.Stones
			if a.Quality != "" {
				compQuality = a.Quality
			}
		case "Gusset":
			hasDeliveryArt = true
			if a.LayBuff > 0 {
				gusset = a.LayBuff
			}
			gussStones = a.Stones
			if a.Quality != "" {
				gussQuality = a.Quality
			}
		case "Chalice":
			hasIHRArt = true
			if mult, ok := chaliceMap[a.Quality]; ok {
				customChalice = mult
			}
			customIHRSlots += a.Stones
		case "Monocle":
			hasIHRArt = true
			if mult, ok := monocleMap[a.Quality]; ok {
				customMonocle = mult
			}
			customIHRSlots += a.Stones
		case "SIAB":
			hasIHRArt = true
			customIHRSlots += a.Stones
			if strings.HasPrefix(a.Quality, "T") {
				hasSIABArt = true
				siabQuality = a.Quality
				siabStones = a.Stones
			}
		case "IHR Deflector":
			hasIHRArt = true
			customIHRSlots += a.Stones
		case "Collegg":
			if a.Quality != "NONE" {
				hasColleggArt = true
				switch a.Dimension {
				case ei.GameModifier_EGG_LAYING_RATE:
					if a.LayBuff > 0 {
						customColELR *= a.LayBuff
					}
				case ei.GameModifier_HAB_CAPACITY:
					if a.LayBuff > 0 {
						customColHab *= a.LayBuff
					}
				case ei.GameModifier_SHIPPING_CAPACITY:
					if a.ShipBuff > 0 {
						customColShip *= a.ShipBuff
					}
				case ei.GameModifier_INTERNAL_HATCHERY_RATE:
					for _, egg := range ei.CustomEggMap {
						if egg != nil && egg.Dimension == ei.GameModifier_INTERNAL_HATCHERY_RATE && len(egg.DimensionValue) > 0 {
							if len(egg.DimensionValueString) > 0 && egg.DimensionValueString[len(egg.DimensionValueString)-1] == a.Quality {
								customColIHR *= egg.DimensionValue[len(egg.DimensionValue)-1]
								break
							}
						}
					}
				}
			}
		}
	}

	if !hasSIABArt && booster != nil && booster.UserID != "" {
		if savedSIAB := farmerstate.GetMiscSettingString(booster.UserID, "SIAB"); strings.HasPrefix(savedSIAB, "T") {
			if art := ei.GetArtifactByKey("SIAB-" + savedSIAB); art != nil {
				siabQuality = art.Quality
				siabStones = art.Stones
			}
		}
	}

	deliverySlots := DefaultLeggyDeliverySlots
	if hasDeliveryArt {
		deliverySlots = float64(deflStones + metrStones + compStones + gussStones)
	}
	siabDeliverySlots := max(0, int(DefaultLeggyDeliverySlots)-3+siabStones)
	if hasDeliveryArt {
		siabDeliverySlots = max(0, deflStones+metrStones+compStones+siabStones)
	}
	if hasColleggArt {
		colELR = customColELR
		colShip = customColShip
		colHab = customColHab
		if customColIHR > 1.0 {
			colIHR = customColIHR
		}
	}
	if hasIHRArt {
		chalice = customChalice
		monocle = customMonocle
		ihrSlots = customIHRSlots
	}

	// Calculate total deflector bonus on this farmer from other players in the contract.
	// Use known deflector artifacts when available, and DefaultLeggyDeflectorBonus for unknown players.
	otherFarmersCount := int(max(0.0, numFarmers-1.0))
	knownDeflectorPlayers := 0
	totalDeflectorBonus := 0.0
	if contract != nil && otherFarmersCount > 0 {
		boosterID := ""
		if booster != nil {
			boosterID = booster.UserID
		}
		for uid, other := range contract.Boosters {
			if uid == boosterID || other == nil {
				continue
			}
			otherArts := other.ArtifactSet.Artifacts
			if len(otherArts) == 0 && uid != "" {
				otherArts = getUserArtifacts(uid, nil).Artifacts
			}
			for _, a := range otherArts {
				if a.Type == "Deflector" {
					knownDeflectorPlayers++
					if a.DeflBuff > 1.0 {
						totalDeflectorBonus += a.DeflBuff - 1.0
					}
					break
				}
			}
		}
	}
	unknownDeflectorPlayers := max(0, otherFarmersCount-knownDeflectorPlayers)
	totalDeflectorBonus += float64(unknownDeflectorPlayers) * DefaultLeggyDeflectorBonus
	deflectorIsActual := otherFarmersCount == 0 || knownDeflectorPlayers >= otherFarmersCount
	deflectorPercent := math.Round(totalDeflectorBonus * 100.0)

	// Calculate max shipping population using the stone-hunt method from estimate_time.go
	deflectorMultiplier := 1.0 + totalDeflectorBonus
	baseELR := 3.772 * metronome * gusset
	baseShipping := 7.148 * compass
	contractBaseELR := baseELR * modELR * modHab

	intSlots := int(deliverySlots)
	bestTotal := 0.0
	bestTach := 0
	bestLR := contractBaseELR * deflectorMultiplier * colELR * colHab
	bestSR := baseShipping * colShip * modSR
	contractELR := bestLR
	for i := 0; i <= intSlots; i++ {
		stoneLayRate := contractBaseELR * deflectorMultiplier * math.Pow(1.05, float64(i)) * colELR * colHab
		stoneShipRate := baseShipping * math.Pow(1.05, float64(intSlots-i)) * colShip * modSR
		bestMin := min(stoneLayRate, stoneShipRate)
		if bestMin > bestTotal {
			bestTotal = bestMin
			bestTach = i
			bestLR = stoneLayRate
			bestSR = stoneShipRate
			contractELR = stoneLayRate
		} else if bestTotal > 0 {
			break
		}
	}
	bestQuant := max(0, intSlots-bestTach)

	var deliveryArtIcons strings.Builder
	for _, spec := range []struct {
		prefix  string
		quality string
	}{
		{"DEFL_", deflQuality},
		{"METR_", metrQuality},
		{"COMP_", compQuality},
		{"GUSSET_", gussQuality},
	} {
		if spec.quality != "" && spec.quality != "NONE" {
			deliveryArtIcons.WriteString(boostEmojiMarkdown(spec.prefix + spec.quality))
		}
	}

	// Calculate Post-Boost Delivery SIAB Set (SIAB used instead of Gusset)
	siabBaseELR := 3.772 * metronome
	siabContractBaseELR := siabBaseELR * modELR * modHab
	siabBestTotal := 0.0
	siabBestTach := 0
	siabBestLR := siabContractBaseELR * deflectorMultiplier * colELR * colHab
	siabBestSR := baseShipping * colShip * modSR
	for i := 0; i <= siabDeliverySlots; i++ {
		stoneLayRate := siabContractBaseELR * deflectorMultiplier * math.Pow(1.05, float64(i)) * colELR * colHab
		stoneShipRate := baseShipping * math.Pow(1.05, float64(siabDeliverySlots-i)) * colShip * modSR
		bestMin := min(stoneLayRate, stoneShipRate)
		if bestMin > siabBestTotal {
			siabBestTotal = bestMin
			siabBestTach = i
			siabBestLR = stoneLayRate
			siabBestSR = stoneShipRate
		} else if siabBestTotal > 0 {
			break
		}
	}
	siabBestQuant := max(0, siabDeliverySlots-siabBestTach)

	var siabDeliveryArtIcons strings.Builder
	for _, spec := range []struct {
		prefix  string
		quality string
	}{
		{"DEFL_", deflQuality},
		{"METR_", metrQuality},
		{"COMP_", compQuality},
		{"SIAB_", siabQuality},
	} {
		if spec.quality != "" && spec.quality != "NONE" {
			siabDeliveryArtIcons.WriteString(boostEmojiMarkdown(spec.prefix + spec.quality))
		}
	}

	unusedRatioELR := 1.0
	if bestTotal > 0 {
		unusedRatioELR = max(1.0, contractELR/bestTotal)
	}

	maxHabCap := (14_175_000_000.0 / DefaultLeggyGusset) * gusset * colHab * modHab
	maxPop := maxHabCap / unusedRatioELR

	// Determine online IHR per hab per minute (including TE, colleggtibles, chalice, monocle, life stones)
	onlineIHRPerHab := 0.0
	if booster != nil && booster.IHRRate > DefaultLeggyIHR {
		onlineIHRPerHab = booster.IHRRate
	} else if booster != nil && booster.UserID != "" {
		if dbRate, _ := CalculateIHRRateFromDB(booster.UserID); dbRate > DefaultLeggyIHR {
			onlineIHRPerHab = dbRate
		}
	}
	if onlineIHRPerHab <= 0 {
		teCount := 0
		if booster != nil && booster.TECount > 0 {
			teCount = booster.TECount
		}
		onlineIHRPerHab = DefaultLeggyIHR * colIHR * chalice * monocle * math.Pow(1.04, float64(ihrSlots)) * math.Pow(1.01, float64(teCount))
	}

	clampedTokens := min(12, max(0, tokens))
	expectedBoostMulti := calcBoostMulti(float64(clampedTokens))
	boostMulti := expectedBoostMulti
	boostsVerified := false
	if booster != nil && booster.VerifiedBoostMulti >= expectedBoostMulti*0.9 {
		boostMulti = booster.VerifiedBoostMulti
		boostsVerified = true
	}
	// 4 habs * 3x offline (Internal Hatchery Calm) = 12x online per-hab IHR
	offlineIHRPerMin := onlineIHRPerHab * 12.0 * boostMulti * modIHR

	// Calculate CR request population and post-CR offline duration (min 1m offline).
	// When the player requests CRs, they must go offline for at least 1 minute and will
	// gain those offline chickens in addition to the chicken runs from coop mates.
	populationForCR := maxPop * (DefaultLeggyChickenRunPercent / 100.0)
	crPopulation := 0.0
	if numFarmers > 1.0 {
		crPopulation = populationForCR * 0.05 * (numFarmers - 1.0)
	}
	minPostCROfflineChickens := offlineIHRPerMin * 1.0
	totalBoostChickensNeeded := max(0.0, maxPop-crPopulation)

	crRequestPop := min(populationForCR, max(0.0, totalBoostChickensNeeded-minPostCROfflineChickens))
	remainingBoostChickens := max(minPostCROfflineChickens, totalBoostChickensNeeded-crRequestPop)

	postCROfflineMinutes := max(1.0, remainingBoostChickens/offlineIHRPerMin)
	postCROfflineDuration := time.Duration(postCROfflineMinutes * float64(time.Minute))

	pureCRMinutes := max(1.0, crRequestPop/offlineIHRPerMin)
	pureCRDuration := time.Duration(pureCRMinutes * float64(time.Minute))
	crDuration := pureCRDuration
	totalBoostDuration := crDuration + postCROfflineDuration

	currentPop := 0.0
	popBasis := "Estimated starting from 0 (pre-boost habs not synced)"
	if booster != nil && booster.VerifiedPopulation > 0 {
		snapPop := booster.VerifiedPopulation
		snapStr := ei.FormatEIValue(snapPop, map[string]any{"decimals": 2, "trim": true})

		if boostsVerified {
			// Snapshot was taken with active token boosts verified. Advance population by offline growth since snapshot.
			refTime := booster.VerifiedAt
			if refTime.IsZero() {
				refTime = now
			}
			offlineDur := now.Sub(refTime)
			if offlineDur < 0 {
				offlineDur = 0
			}
			offlineGained := (offlineIHRPerMin / float64(time.Minute)) * float64(offlineDur)
			currentPop = min(maxPop, snapPop+offlineGained)
			timeStr := ""
			if !booster.VerifiedAt.IsZero() {
				timeStr = fmt.Sprintf(" from <t:%d:R>", booster.VerifiedAt.Unix())
			}
			if offlineDur >= 5*time.Second && offlineGained >= 100_000 {
				gainedStr := ei.FormatEIValue(offlineGained, map[string]any{"decimals": 2, "trim": true})
				durStr := bottools.FmtDuration(offlineDur.Round(time.Second))
				popBasis = fmt.Sprintf("Snapshot %s%s + %s offline (%s)", snapStr, timeStr, gainedStr, durStr)
			} else {
				popBasis = fmt.Sprintf("Active boost snapshot %s%s", snapStr, timeStr)
			}
		} else {
			// Snapshot was taken before boosts were active (pre-boost habs from prior coop_status query).
			boostStart := booster.EndTime
			if boostStart.IsZero() {
				boostStart = booster.VerifiedAt
			}
			if boostStart.IsZero() {
				boostStart = now
			}
			boostElapsed := now.Sub(boostStart)
			if boostElapsed < 0 {
				boostElapsed = 0
			}
			offlineGained := (offlineIHRPerMin / float64(time.Minute)) * float64(boostElapsed)
			currentPop = min(maxPop, snapPop+offlineGained)
			timeStr := ""
			if !booster.VerifiedAt.IsZero() {
				timeStr = fmt.Sprintf(" from <t:%d:R>", booster.VerifiedAt.Unix())
			}
			if boostElapsed >= 5*time.Second && offlineGained >= 100_000 {
				gainedStr := ei.FormatEIValue(offlineGained, map[string]any{"decimals": 2, "trim": true})
				durStr := bottools.FmtDuration(boostElapsed.Round(time.Second))
				popBasis = fmt.Sprintf("Pre-boost habs %s%s + %s offline (%s)", snapStr, timeStr, gainedStr, durStr)
			} else {
				popBasis = fmt.Sprintf("Pre-boost hab snapshot %s%s", snapStr, timeStr)
			}
		}

		if currentPop < crRequestPop {
			remCRMinutes := (crRequestPop - currentPop) / offlineIHRPerMin
			crDuration = time.Duration(remCRMinutes * float64(time.Minute))
			totalBoostDuration = crDuration + postCROfflineDuration
		} else {
			crDuration = 0
			remChickens := max(0.0, totalBoostChickensNeeded-currentPop)
			postCROfflineMinutes = max(1.0, remChickens/offlineIHRPerMin)
			postCROfflineDuration = time.Duration(postCROfflineMinutes * float64(time.Minute))
			totalBoostDuration = postCROfflineDuration
		}
	}

	boostIcons := getExpectedBoostIcons(tokens, pureCRDuration+postCROfflineDuration)
	if boostsVerified && booster.VerifiedBoostIcons != "" {
		boostIcons = booster.VerifiedBoostIcons
	}

	return BoostSchedule{
		Tokens:                tokens,
		BoostIcons:            boostIcons,
		TotalBoostDuration:    totalBoostDuration,
		CRDuration:            crDuration,
		PureCRDuration:        pureCRDuration,
		PostCROfflineDuration: postCROfflineDuration,
		CRRequestPopulation:   crRequestPop,
		MaxShippingPopulation: maxPop,
		OfflineIHRPerMin:      offlineIHRPerMin,
		DeflectorPercent:      deflectorPercent,
		DeflectorIsActual:     deflectorIsActual,
		DeliveryArtifacts:     deliveryArtIcons.String(),
		TachyonStones:         bestTach,
		QuantumStones:         bestQuant,
		LayRate:               bestLR,
		ShipRate:              bestSR,
		DeliveryRate:          bestTotal,
		ShowSIABSet:           showSIABSet,
		SIABDeliveryArtifacts: siabDeliveryArtIcons.String(),
		SIABTachyonStones:     siabBestTach,
		SIABQuantumStones:     siabBestQuant,
		SIABLayRate:           siabBestLR,
		SIABShipRate:          siabBestSR,
		SIABDeliveryRate:      siabBestTotal,
		BoostsVerified:        boostsVerified,
		CurrentPopulation:     currentPop,
		PopulationBasis:       popBasis,
		EstRequestChickenRuns: now.Add(crDuration),
		EstEndOfBoost:         now.Add(totalBoostDuration),
	}
}

func formatStoneRatePerHour(rate float64) string {
	if rate < 1.0 {
		return fmt.Sprintf("%.2fT/hr", rate*1000.0)
	}
	return fmt.Sprintf("%.3fq/hr", rate)
}

// formatBoostSchedule builds the ephemeral boost schedule message shown when a booster presses 🚀.
func formatBoostSchedule(contract *Contract, booster *Booster, sched BoostSchedule) string {
	tokenIcon := boostEmojiMarkdown("token")
	if contract != nil && contract.TokenStr != "" {
		tokenIcon = contract.TokenStr
	}
	crIcon := boostEmojiMarkdown("icon_chicken_run")
	deflIcon := boostEmojiMarkdown("defl_T4L")
	tachStoneIcon := boostEmojiMarkdown("afx_tachyon_stone_4")
	quantStoneIcon := boostEmojiMarkdown("afx_quantum_stone_4")

	crPopStr := ei.FormatEIValue(sched.CRRequestPopulation, map[string]any{"decimals": 2, "trim": true})
	maxPopStr := ei.FormatEIValue(sched.MaxShippingPopulation, map[string]any{"decimals": 2, "trim": true})
	totalDurStr := bottools.FmtDuration(sched.TotalBoostDuration.Round(time.Second))
	crDurStr := bottools.FmtDuration(sched.CRDuration.Round(time.Second))
	postCRDurStr := bottools.FmtDuration(sched.PostCROfflineDuration.Round(time.Second))

	deflStatus := "estimated"
	if sched.DeflectorIsActual {
		deflStatus = "actual"
	}

	var sb strings.Builder
	boosterTarget := ""
	if booster != nil {
		if booster.Mention != "" {
			boosterTarget = " for " + booster.Mention
		} else if booster.Nick != "" {
			boosterTarget = " for " + booster.Nick
		}
	}
	fmt.Fprintf(&sb, "## 🚀 Boost Schedule%s (%d %s)\n", boosterTarget, sched.Tokens, tokenIcon)
	fmt.Fprintf(&sb, "**Total Boost Time:** `%s` (Done <t:%d:t>)\n", totalDurStr, sched.EstEndOfBoost.Unix())
	if sched.CurrentPopulation > 0 {
		curPopStr := ei.FormatEIValue(sched.CurrentPopulation, map[string]any{"decimals": 2, "trim": true})
		verifiedStr := ""
		if sched.BoostsVerified {
			verifiedStr = " (verified)"
		}
		fmt.Fprintf(&sb, "**Current Population:** `%s`%s — %s\n", curPopStr, verifiedStr, sched.PopulationBasis)
	} else if sched.PopulationBasis != "" {
		fmt.Fprintf(&sb, "**Current Population:** `0` — %s\n", sched.PopulationBasis)
	}
	fmt.Fprintf(&sb, "**Request Chicken Runs %s at:** `%s` chickens (Max shipping pop: `%s`)\n", crIcon, crPopStr, maxPopStr)
	fmt.Fprintf(&sb, "**Deflector Bonus %s:** `%.0f%%` (%s)\n", deflIcon, sched.DeflectorPercent, deflStatus)
	boostStatus := ""
	if sched.BoostsVerified {
		boostStatus = " (verified)"
	} else if booster != nil && (booster.BoostState == BoostStateBoosted || !booster.EndTime.IsZero()) && sched.Tokens > 0 {
		boostStatus = " (assumed — sync pending)"
	}
	fmt.Fprintf(&sb, "**Boosts:** %s%s\n", sched.BoostIcons, boostStatus)
	fmt.Fprintf(&sb, "**Post-Boost Delivery Set:** %s — %s `%d`  %s `%d`\n",
		sched.DeliveryArtifacts, tachStoneIcon, sched.TachyonStones, quantStoneIcon, sched.QuantumStones)
	fmt.Fprintf(&sb, "**Projected Rates:** **Delivery:** `%s` (**LR:** `%s` | **SR:** `%s`)\n",
		formatStoneRatePerHour(sched.DeliveryRate), formatStoneRatePerHour(sched.LayRate), formatStoneRatePerHour(sched.ShipRate))
	if sched.ShowSIABSet {
		fmt.Fprintf(&sb, "**Post-Boost Delivery SIAB Set:** %s — %s `%d`  %s `%d`\n",
			sched.SIABDeliveryArtifacts, tachStoneIcon, sched.SIABTachyonStones, quantStoneIcon, sched.SIABQuantumStones)
		fmt.Fprintf(&sb, "**Projected SIAB Rates:** **Delivery:** `%s` (**LR:** `%s` | **SR:** `%s`)\n",
			formatStoneRatePerHour(sched.SIABDeliveryRate), formatStoneRatePerHour(sched.SIABLayRate), formatStoneRatePerHour(sched.SIABShipRate))
	}
	sb.WriteString("\n**Schedule:**\n")
	if sched.CRDuration > 0 {
		fmt.Fprintf(&sb, "• **Reach CR Population (`%s`):** `%s` — Request CR %s <t:%d:R>\n",
			crPopStr, crDurStr, crIcon, sched.EstRequestChickenRuns.Unix())
	} else {
		fmt.Fprintf(&sb, "• **Reach CR Population (`%s`):** Ready now! — Request CR %s immediately\n",
			crPopStr, crIcon)
	}
	fmt.Fprintf(&sb, "• **Offline to Max Shipping (`%s`):** `%s` (go offline for at least 1m after requesting CRs to receive runs & offline chickens)",
		maxPopStr, postCRDurStr)

	return sb.String()
}

// setEstimatedBoostTimings calculates and sets the estimated boost timings on a booster
func setEstimatedBoostTimings(booster *Booster, contracts ...*Contract) {
	if booster == nil {
		return
	}
	var contract *Contract
	if len(contracts) > 0 {
		contract = contracts[0]
	}
	sched := calculateBoostSchedule(contract, booster, time.Now())
	booster.EstDurationOfBoost = sched.TotalBoostDuration
	booster.EstRequestChickenRuns = sched.EstRequestChickenRuns
	booster.EstEndOfBoost = sched.EstEndOfBoost
}

var fetchCoopStatusForDeflectors = func(contractID, coopID, eeidOverride string) (*ei.ContractCoopStatusResponse, error) {
	if config.EIUserIDBasic == "" && eeidOverride == "" && !strings.HasPrefix(coopID, "!!") {
		return nil, nil
	}
	coopStatus, _, _, err := ei.GetCoopStatusUncached(contractID, coopID, eeidOverride)
	if err != nil {
		coopStatus, _, _, err = ei.GetCoopStatus(contractID, coopID, eeidOverride)
	}
	return coopStatus, err
}

func extractContributorDeflector(contrib *ei.ContractCoopStatusResponse_ContributionInfo) ei.Artifact {
	deflectorBuffMap := map[string]float64{
		"T1C": 1.05,
		"T2C": 1.08,
		"T3C": 1.12, "T3R": 1.13,
		"T4C": 1.15, "T4R": 1.17, "T4E": 1.19, "T4L": 1.20,
	}

	if fi := contrib.GetFarmInfo(); fi != nil {
		for _, artifact := range fi.GetEquippedArtifacts() {
			spec := artifact.GetSpec()
			if spec.GetName() != ei.ArtifactSpec_TACHYON_DEFLECTOR {
				continue
			}
			lvl := int(spec.GetLevel())
			rar := int(spec.GetRarity())
			if lvl >= 0 && lvl < len(ei.ArtifactLevels) && rar >= 0 && rar < len(ei.ArtifactRarity) {
				strType := ei.ArtifactLevels[lvl] + ei.ArtifactRarity[rar]
				if art := ei.GetArtifactByKey("D-" + strType); art != nil {
					return *art
				}
				if buff, ok := deflectorBuffMap[strType]; ok {
					numStones, _ := ei.GetStones(spec.GetName(), spec.GetLevel(), spec.GetRarity())
					return ei.Artifact{
						Type:     "Deflector",
						Quality:  strType,
						ShipBuff: 1.0,
						LayBuff:  1.0,
						DeflBuff: buff,
						Stones:   numStones,
					}
				}
			}
		}
	}

	bestBuff := 1.0
	for _, b := range contrib.GetBuffHistory() {
		if b.GetEggLayingRate() > bestBuff {
			bestBuff = b.GetEggLayingRate()
		}
	}
	if bestBuff > 1.0 {
		pct := int(math.Round((bestBuff - 1.0) * 100.0))
		pctToKey := map[int]string{
			20: "D-T4L",
			19: "D-T4E",
			17: "D-T4R",
			15: "D-T4C",
			13: "D-T3R",
			12: "D-T3C",
		}
		if key, ok := pctToKey[pct]; ok {
			if art := ei.GetArtifactByKey(key); art != nil {
				return *art
			}
		}
		return ei.Artifact{
			Type:     "Deflector",
			Quality:  fmt.Sprintf("%d%%", pct),
			ShipBuff: 1.0,
			LayBuff:  1.0,
			DeflBuff: bestBuff,
		}
	}

	if noneArt := ei.GetArtifactByKey("D-NONE"); noneArt != nil {
		return *noneArt
	}
	return ei.Artifact{
		Type:     "Deflector",
		Quality:  "NONE",
		ShipBuff: 1.0,
		LayBuff:  1.0,
		DeflBuff: 1.0,
	}
}

func setBoosterDeflectorArtifact(b *Booster, deflArt ei.Artifact) {
	if b == nil {
		return
	}
	if deflArt.Quality == "NONE" && (b.BoostState == BoostStateTokenTime || (!b.EstEndOfBoost.IsZero() && time.Now().Before(b.EstEndOfBoost))) {
		return
	}
	if len(b.ArtifactSet.Artifacts) == 0 && b.UserID != "" {
		b.ArtifactSet = getUserArtifacts(b.UserID, nil)
	}
	replaced := false
	for i, a := range b.ArtifactSet.Artifacts {
		if a.Type == "Deflector" {
			b.ArtifactSet.Artifacts[i] = deflArt
			replaced = true
			break
		}
	}
	if !replaced {
		b.ArtifactSet.Artifacts = append(b.ArtifactSet.Artifacts, deflArt)
	}
	b.ArtifactSet = getUserArtifacts(b.UserID, &b.ArtifactSet)
}

func isFastrunOrLeaderboard(contract *Contract) bool {
	if contract == nil {
		return false
	}
	if contract.Style&ContractFlagBanker != 0 || contract.State == ContractStateBanker {
		return false
	}
	return contract.PlayStyle == ContractPlaystyleFastrun || contract.PlayStyle == ContractPlaystyleLeaderboard
}

func resolveBoostClickUserID(contract *Contract, userID, currentBoosterID string) string {
	if contract == nil || currentBoosterID == "" {
		return userID
	}
	if b := contract.Boosters[userID]; b != nil && len(b.Alts) > 0 {
		for _, altID := range b.Alts {
			if altID == currentBoosterID {
				return altID
			}
		}
	}
	return userID
}

// formatInitialBoostSchedule builds the initial ephemeral message shown immediately when a booster presses 🚀.
func formatInitialBoostSchedule(contract *Contract, booster *Booster, sched BoostSchedule) string {
	tokenIcon := boostEmojiMarkdown("token")
	if contract != nil && contract.TokenStr != "" {
		tokenIcon = contract.TokenStr
	}
	crIcon := boostEmojiMarkdown("icon_chicken_run")
	crPopStr := ei.FormatEIValue(sched.CRRequestPopulation, map[string]any{"decimals": 2, "trim": true})
	totalDurStr := bottools.FmtDuration(sched.TotalBoostDuration.Round(time.Second))

	var sb strings.Builder
	boosterTarget := ""
	if booster != nil {
		if booster.Mention != "" {
			boosterTarget = " for " + booster.Mention
		} else if booster.Nick != "" {
			boosterTarget = " for " + booster.Nick
		}
	}
	fmt.Fprintf(&sb, "## 🚀 Boost Schedule%s (%d %s)\n", boosterTarget, sched.Tokens, tokenIcon)
	fmt.Fprintf(&sb, "**Boosts:** %s\n", sched.BoostIcons)
	fmt.Fprintf(&sb, "**Estimated Boost Time:** `%s` (Request CR %s at `%s` chickens)\n", totalDurStr, crIcon, crPopStr)
	if sched.CurrentPopulation > 0 {
		curPopStr := ei.FormatEIValue(sched.CurrentPopulation, map[string]any{"decimals": 2, "trim": true})
		fmt.Fprintf(&sb, "**Population Basis:** Starting from `%s` (%s)\n", curPopStr, sched.PopulationBasis)
	} else {
		sb.WriteString("**Population Basis:** Estimated starting from `0` chickens (pre-boost habs not yet synced)\n")
	}
	sb.WriteString("\n⏳ *Checking coop API in 1m to verify your active boosts, current population, and deflectors for accurate timing...*")
	return sb.String()
}

var boostVerifyDelay = 1 * time.Minute

func scheduleBoostVerification(contract *Contract) {
	if contract == nil {
		return
	}
	delay := boostVerifyDelay
	go func() {
		if delay > 0 {
			time.Sleep(delay)
		}
		updateCoopDeflectorsFromStatus(contract)
	}()
}

// sendBoostScheduleEphemeral sends the active booster's ephemeral boost schedule message and records
// its interaction metadata so it can be updated if a coop_status response arrives during their boost time.
func sendBoostScheduleEphemeral(client dc.Client, e *dc.ComponentEvent, contract *Contract, booster *Booster) {
	if e == nil || contract == nil || booster == nil {
		return
	}
	setActiveBoostClient(client)

	boostStart := booster.EndTime
	if boostStart.IsZero() {
		boostStart = time.Now()
	}
	sched := calculateBoostSchedule(contract, booster, boostStart)
	scheduleMsg := formatInitialBoostSchedule(contract, booster, sched)

	msg, err := e.FollowupMessage(dc.Message{
		Content:   scheduleMsg,
		Ephemeral: true,
	})
	if err != nil {
		return
	}

	msgID := ""
	if msg != nil && msg.ID != "" {
		msgID = msg.ID
	} else if e.Client() == nil && e.Token() != "" {
		msgID = "followup-ephemeral"
	}

	contract.mutex.Lock()
	booster.BoostScheduleAppID = e.ApplicationID()
	booster.BoostScheduleToken = e.Token()
	booster.BoostScheduleMsgID = msgID
	contract.mutex.Unlock()
}

// sendBoostScheduleDM sends a DM to recipientUserID with the boost schedule for booster and a Dismiss button.
func sendBoostScheduleDM(client dc.Client, contract *Contract, booster *Booster, recipientUserID string) {
	if client == nil || contract == nil || booster == nil || recipientUserID == "" {
		return
	}
	setActiveBoostClient(client)

	sched := calculateBoostSchedule(contract, booster, time.Now())
	scheduleMsg := formatBoostSchedule(contract, booster, sched)
	u, dmErr := client.CreateUserChannel(recipientUserID)
	if dmErr != nil || u == nil {
		return
	}
	msg := dc.Message{
		Content:      scheduleMsg,
		ComponentsV1: true,
		Components: []dc.LayoutComponent{
			dc.ActionRow{
				Components: []dc.InteractiveComponent{
					dc.Button{
						Label:    "Dismiss",
						Style:    dc.ButtonSecondary,
						CustomID: "rc_#dismiss#" + contract.ContractHash,
					},
				},
			},
		},
	}
	_, _ = client.SendMessage(u.ID, msg)
}

type pendingScheduleEdit struct {
	appID   string
	token   string
	msgID   string
	content string
}

// updateActiveBoostScheduleMessages updates the ephemeral boost schedule message for any player
// in the contract who is currently within their boost time.
func updateActiveBoostScheduleMessages(client dc.Client, contract *Contract) {
	if contract == nil {
		return
	}
	now := time.Now()
	var edits []pendingScheduleEdit
	var needsRetryVerification bool

	contract.mutex.Lock()
	for _, b := range contract.Boosters {
		if b == nil || b.BoostState != BoostStateBoosted || b.EstEndOfBoost.IsZero() || !now.Before(b.EstEndOfBoost) {
			continue
		}
		boostStart := b.EndTime
		if boostStart.IsZero() {
			boostStart = now
		}
		sched := calculateBoostSchedule(contract, b, boostStart)
		b.EstDurationOfBoost = sched.TotalBoostDuration
		b.EstRequestChickenRuns = sched.EstRequestChickenRuns
		b.EstEndOfBoost = sched.EstEndOfBoost

		if b.BoostScheduleMsgID != "" && b.BoostScheduleToken != "" && b.BoostScheduleAppID != "" {
			edits = append(edits, pendingScheduleEdit{
				appID:   b.BoostScheduleAppID,
				token:   b.BoostScheduleToken,
				msgID:   b.BoostScheduleMsgID,
				content: formatBoostSchedule(contract, b, sched),
			})
			if !sched.BoostsVerified && b.BoostVerifyRetries < 5 {
				b.BoostVerifyRetries++
				needsRetryVerification = true
			}
		}
	}
	contract.mutex.Unlock()

	if needsRetryVerification {
		scheduleBoostVerification(contract)
	}

	if client == nil {
		client = getActiveBoostClient()
	}
	if client == nil {
		return
	}
	for _, edit := range edits {
		if edit.msgID == "@original" {
			if err := client.EditInteractionResponse(edit.appID, edit.token, dc.Message{Content: edit.content}); err != nil {
				log.Printf("updateActiveBoostScheduleMessages: EditInteractionResponse error: %v", err)
			}
			continue
		}
		if err := client.EditFollowupMessage(edit.appID, edit.token, edit.msgID, dc.Message{Content: edit.content}); err != nil {
			log.Printf("updateActiveBoostScheduleMessages: EditFollowupMessage error: %v", err)
		}
	}
}

func handleCoopStatusForActiveContracts(contractID, coopID string, coopStatus *ei.ContractCoopStatusResponse) {
	if coopStatus == nil || coopStatus.GetResponseStatus() != ei.ContractCoopStatusResponse_NO_ERROR {
		return
	}
	var matching []*Contract
	ContractsMutex.RLock()
	for _, c := range Contracts {
		if c != nil && strings.EqualFold(c.ContractID, contractID) && strings.EqualFold(c.CoopID, coopID) {
			matching = append(matching, c)
		}
	}
	ContractsMutex.RUnlock()

	for _, c := range matching {
		applyCoopStatusDeflectors(c, coopStatus)
	}
}

type activeBoostMeta struct {
	emojiName  string
	duration   string
	isTachyon  bool
	isBeacon   bool
	multiplier float64
}

var knownActiveBoostMap = map[string]activeBoostMeta{
	"tachyon_prism_blue":       {emojiName: "b_tach10x10m", duration: "10m", isTachyon: true, multiplier: 10},
	"tachyon_prism_blue_v2":    {emojiName: "b_tach10x10m", duration: "10m", isTachyon: true, multiplier: 10},
	"tachyon_prism_blue_big":   {emojiName: "b_tach10x4h", duration: "4h", isTachyon: true, multiplier: 10},
	"tachyon_prism_purple":     {emojiName: "b_tach100x10m", duration: "10m", isTachyon: true, multiplier: 100},
	"tachyon_prism_purple_v2":  {emojiName: "b_tach100x10m", duration: "10m", isTachyon: true, multiplier: 100},
	"tachyon_prism_purple_big": {emojiName: "b_tach100x2h", duration: "2h", isTachyon: true, multiplier: 100},
	"tachyon_prism_orange":     {emojiName: "b_tach1000x10m", duration: "10m", isTachyon: true, multiplier: 1000},
	"tachyon_prism_orange_big": {emojiName: "b_tach1000x1h", duration: "1h", isTachyon: true, multiplier: 1000},
	"boost_beacon_blue":        {emojiName: "b_boost2x30m", duration: "30m", isBeacon: true, multiplier: 2},
	"boost_beacon_blue_big":    {emojiName: "b_boost5x1h", duration: "1h", isBeacon: true, multiplier: 5},
	"boost_beacon_purple":      {emojiName: "b_boost10x10m", duration: "10m", isBeacon: true, multiplier: 10},
	"boost_beacon_orange":      {emojiName: "b_boost50x10m", duration: "10m", isBeacon: true, multiplier: 50},
}

func getBoostBaseDuration(boostID string) time.Duration {
	switch strings.ToLower(strings.TrimSpace(boostID)) {
	case "tachyon_prism_orange", "tachyon_prism_purple", "tachyon_prism_purple_v2", "tachyon_prism_blue", "tachyon_prism_blue_v2":
		return 10 * time.Minute
	case "tachyon_prism_orange_big":
		return 1 * time.Hour
	case "tachyon_prism_purple_big":
		return 2 * time.Hour
	case "tachyon_prism_blue_big":
		return 4 * time.Hour
	default:
		return 10 * time.Minute
	}
}

func getActivePaidTachyonBoost(activeBoosts []*ei.Backup_ActiveBoost) (*ei.Backup_ActiveBoost, activeBoostMeta, bool) {
	for _, ab := range activeBoosts {
		if ab == nil || ab.TimeRemaining == nil || ab.GetTimeRemaining() <= 0 {
			continue
		}
		rawID := strings.ToLower(strings.TrimSpace(ab.GetBoostId()))
		meta, ok := knownActiveBoostMap[rawID]
		if ok && meta.isTachyon && meta.multiplier >= 100 {
			return ab, meta, true
		}
	}
	return nil, activeBoostMeta{}, false
}

func extractContributorActiveBoosts(activeBoosts []*ei.Backup_ActiveBoost) (float64, string) {
	if len(activeBoosts) == 0 {
		return 0, ""
	}
	var tachSum, beaconSum float64
	var items []boostItemSpec
	indexByEmoji := make(map[string]int)

	for _, ab := range activeBoosts {
		if ab == nil {
			continue
		}
		if ab.TimeRemaining != nil && ab.GetTimeRemaining() <= 0 {
			continue
		}
		rawID := strings.ToLower(strings.TrimSpace(ab.GetBoostId()))
		meta, ok := knownActiveBoostMap[rawID]
		if !ok {
			continue
		}
		mult := meta.multiplier
		if ab.GetReferenceValue() > 0 {
			mult = ab.GetReferenceValue()
		}
		if meta.isTachyon {
			tachSum += mult
		} else if meta.isBeacon {
			beaconSum += mult
		}
		if idx, exists := indexByEmoji[meta.emojiName]; exists {
			items[idx].count++
		} else {
			indexByEmoji[meta.emojiName] = len(items)
			items = append(items, boostItemSpec{
				emojiName: meta.emojiName,
				count:     1,
				duration:  meta.duration,
			})
		}
	}

	if tachSum <= 0 {
		return 0, ""
	}
	if beaconSum <= 0 {
		beaconSum = 1.0
	}
	totalMulti := tachSum * beaconSum

	parts := make([]string, 0, len(items))
	for _, item := range items {
		icon := boostEmojiMarkdown(item.emojiName)
		parts = append(parts, fmt.Sprintf("%s (%s)", strings.Repeat(icon, item.count), item.duration))
	}
	return totalMulti, strings.Join(parts, "  ")
}

func applyContributorBoostAndPop(b *Booster, contrib *ei.ContractCoopStatusResponse_ContributionInfo) {
	if b == nil || contrib == nil {
		return
	}
	pop := 0.0
	if pp := contrib.GetProductionParams(); pp != nil && pp.GetFarmPopulation() > 0 {
		pop = pp.GetFarmPopulation()
	}
	var habSum uint64
	var activeBoosts []*ei.Backup_ActiveBoost
	var rawTimestamp float64
	hasFarmInfo := false
	if fi := contrib.GetFarmInfo(); fi != nil {
		hasFarmInfo = true
		for _, hp := range fi.GetHabPopulation() {
			habSum += hp
		}
		if float64(habSum) > pop {
			pop = float64(habSum)
		}
		activeBoosts = fi.GetActiveBoosts()
		if fi.Timestamp != nil {
			rawTimestamp = fi.GetTimestamp()
		}
	}

	snapshotTime := time.Now()
	if hasFarmInfo {
		offlineSec := 0.0
		if rawTimestamp < 0 {
			offlineSec = -rawTimestamp
		} else if rawTimestamp > 1_000_000_000 {
			offlineSec = max(0, time.Since(time.Unix(int64(rawTimestamp), 0)).Seconds())
		}
		if offlineSec > 0 {
			snapshotTime = time.Now().Add(-time.Duration(offlineSec * float64(time.Second)))
		}
	}

	boostMulti, boostIcons := extractContributorActiveBoosts(activeBoosts)
	if boostMulti > 0 {
		b.VerifiedBoostMulti = boostMulti
		b.VerifiedBoostIcons = boostIcons
	}
	if pop > 0 {
		if b.VerifiedAt.IsZero() || !snapshotTime.Before(b.VerifiedAt) {
			b.VerifiedPopulation = pop
			b.VerifiedAt = snapshotTime
		}
	}
}

// applyCoopStatusDeflectors updates the contract's boosters with deflector artifacts, active boosts, and population discovered in coopStatus.
func applyCoopStatusDeflectors(contract *Contract, coopStatus *ei.ContractCoopStatusResponse) {
	if contract == nil || coopStatus == nil {
		return
	}
	if coopStatus.GetResponseStatus() != ei.ContractCoopStatusResponse_NO_ERROR {
		return
	}
	contributors := coopStatus.GetContributors()
	if len(contributors) == 0 {
		return
	}

	contract.mutex.Lock()

	matchedBoosters := make(map[string]bool, len(contract.Boosters))
	type unmatchedEntry struct {
		contrib *ei.ContractCoopStatusResponse_ContributionInfo
		deflArt ei.Artifact
	}
	var unmatchedContribs []unmatchedEntry

	for _, contrib := range contributors {
		if contrib == nil || contrib.GetUserName() == "[departed]" {
			continue
		}
		deflArt := extractContributorDeflector(contrib)
		rawName := strings.TrimSpace(contrib.GetUserName())
		normName := strings.TrimSpace(ei.NormalizePlayerNameForDisplay(rawName))

		matchedUID := ""
		if rawName != "" {
			if discordID, err := farmerstate.GetDiscordUserIDFromEiIgn(rawName); err == nil && discordID != "" {
				if _, ok := contract.Boosters[discordID]; ok && !matchedBoosters[discordID] {
					matchedUID = discordID
				}
			}
			if matchedUID == "" {
				if discordID, err := farmerstate.GetDiscordUserIDFromEggIncName(rawName); err == nil && discordID != "" {
					if _, ok := contract.Boosters[discordID]; ok && !matchedBoosters[discordID] {
						matchedUID = discordID
					}
				}
			}
		}
		if matchedUID == "" {
			for uid, b := range contract.Boosters {
				if b == nil || matchedBoosters[uid] {
					continue
				}
				ign := strings.TrimSpace(farmerstate.GetMiscSettingString(uid, "ei_ign"))
				if (ign != "" && (strings.EqualFold(ign, rawName) || strings.EqualFold(ign, normName))) ||
					strings.EqualFold(strings.TrimSpace(b.Name), rawName) ||
					strings.EqualFold(strings.TrimSpace(b.Nick), rawName) ||
					strings.EqualFold(strings.TrimSpace(b.UserName), rawName) ||
					strings.EqualFold(strings.TrimSpace(b.Name), normName) ||
					strings.EqualFold(strings.TrimSpace(b.Nick), normName) ||
					strings.EqualFold(uid, rawName) {
					matchedUID = uid
					break
				}
			}
		}

		if matchedUID != "" {
			matchedBoosters[matchedUID] = true
			b := contract.Boosters[matchedUID]
			setBoosterDeflectorArtifact(b, deflArt)
			applyContributorBoostAndPop(b, contrib)
		} else {
			unmatchedContribs = append(unmatchedContribs, unmatchedEntry{contrib: contrib, deflArt: deflArt})
		}
	}

	// Pass 2: If the player isn't known to boost bot and the name isn't found matched up to something
	// in the coop status response, look at the times on the players active boosts and match based on
	// those to someone who spent tokens for one of the tachyon boosts.
	var remainingUnmatchedContribs []unmatchedEntry
	for _, ue := range unmatchedContribs {
		contrib := ue.contrib
		var activeBoosts []*ei.Backup_ActiveBoost
		if fi := contrib.GetFarmInfo(); fi != nil {
			activeBoosts = fi.GetActiveBoosts()
		}
		tachBoost, _, ok := getActivePaidTachyonBoost(activeBoosts)
		if !ok {
			remainingUnmatchedContribs = append(remainingUnmatchedContribs, ue)
			continue
		}

		timeRemaining := time.Duration(tachBoost.GetTimeRemaining() * float64(time.Second))
		offlineSec := 0.0
		if fi := contrib.GetFarmInfo(); fi != nil && fi.Timestamp != nil {
			if fi.GetTimestamp() < 0 {
				offlineSec = -fi.GetTimestamp()
			} else if fi.GetTimestamp() > 1_000_000_000 {
				offlineSec = max(0, time.Since(time.Unix(int64(fi.GetTimestamp()), 0)).Seconds())
			}
		}
		snapshotTime := time.Now().Add(-time.Duration(offlineSec * float64(time.Second)))
		boostEndTime := snapshotTime.Add(timeRemaining)
		baseDur := getBoostBaseDuration(tachBoost.GetBoostId())

		var bestUID string
		var minDiff time.Duration = 1<<63 - 1

		for uid, b := range contract.Boosters {
			if b == nil || matchedBoosters[uid] {
				continue
			}
			if b.BoostState != BoostStateBoosted && b.EndTime.IsZero() {
				continue
			}
			tokensSpent := b.TokensWanted
			if tokensSpent < 2 && b.TokensReceived >= 2 {
				tokensSpent = b.TokensReceived
			}
			if tokensSpent < 2 {
				continue
			}

			boostTurnTime := b.EndTime
			if boostTurnTime.IsZero() {
				boostTurnTime = b.StartTime
			}
			if boostTurnTime.IsZero() {
				continue
			}

			if boostTurnTime.After(boostEndTime) {
				continue
			}

			nominalElapsed := time.Duration(0)
			if timeRemaining <= baseDur {
				nominalElapsed = baseDur - timeRemaining
			}
			nominalStart := snapshotTime.Add(-nominalElapsed)

			diff := boostTurnTime.Sub(nominalStart)
			if diff < 0 {
				diff = -diff
			}

			maxWindow := time.Duration(float64(baseDur) * 2.6)
			if maxWindow < 25*time.Minute {
				maxWindow = 25 * time.Minute
			}
			if diff < maxWindow && diff < minDiff {
				if contrib.BoostTokensSpent != nil && int(contrib.GetBoostTokensSpent()) == tokensSpent {
					diff -= 5 * time.Minute
					if diff < 0 {
						diff = 0
					}
				}
				minDiff = diff
				bestUID = uid
			}
		}

		if bestUID != "" {
			matchedBoosters[bestUID] = true
			b := contract.Boosters[bestUID]
			setBoosterDeflectorArtifact(b, ue.deflArt)
			applyContributorBoostAndPop(b, contrib)
			rawName := strings.TrimSpace(contrib.GetUserName())
			if rawName != "" && farmerstate.GetMiscSettingString(bestUID, "ei_ign") == "" {
				farmerstate.SetMiscSettingString(bestUID, "ei_ign", rawName)
			}
		} else {
			remainingUnmatchedContribs = append(remainingUnmatchedContribs, ue)
		}
	}

	unmatchedDeflectors := make([]ei.Artifact, 0, len(remainingUnmatchedContribs))
	for _, ue := range remainingUnmatchedContribs {
		unmatchedDeflectors = append(unmatchedDeflectors, ue.deflArt)
	}

	// For any remaining unmatched contributors, assign deflectors in boost order to boosters without a matched deflector
	if len(unmatchedDeflectors) > 0 {
		idx := 0
		for _, uid := range contract.Order {
			if idx >= len(unmatchedDeflectors) {
				break
			}
			b := contract.Boosters[uid]
			if b == nil || matchedBoosters[uid] {
				continue
			}
			hasDefl := false
			for _, a := range b.ArtifactSet.Artifacts {
				if a.Type == "Deflector" {
					hasDefl = true
					break
				}
			}
			if !hasDefl {
				matchedBoosters[uid] = true
				setBoosterDeflectorArtifact(b, unmatchedDeflectors[idx])
				idx++
			}
		}
		for _, uid := range contract.Order {
			if idx >= len(unmatchedDeflectors) {
				break
			}
			b := contract.Boosters[uid]
			if b == nil || matchedBoosters[uid] {
				continue
			}
			matchedBoosters[uid] = true
			setBoosterDeflectorArtifact(b, unmatchedDeflectors[idx])
			idx++
		}
	}
	contract.mutex.Unlock()

	updateActiveBoostScheduleMessages(getActiveBoostClient(), contract)
	saveData(contract.ContractHash)
}

// updateCoopDeflectorsFromStatus fetches coop_status for the contract and updates booster deflector artifacts, active boosts, and population.
func updateCoopDeflectorsFromStatus(contract *Contract) {
	if contract == nil || contract.ContractID == "" || contract.CoopID == "" {
		if contract != nil {
			updateActiveBoostScheduleMessages(getActiveBoostClient(), contract)
		}
		return
	}

	eeidOverride := ""
	contract.mutex.Lock()
	for uid := range contract.Boosters {
		if id := farmerstate.GetMiscSettingString(uid, "encrypted_ei_id"); id != "" {
			eeidOverride = id
			break
		}
	}
	if eeidOverride == "" {
		for _, cid := range contract.CreatorID {
			if id := farmerstate.GetMiscSettingString(cid, "encrypted_ei_id"); id != "" {
				eeidOverride = id
				break
			}
		}
	}
	contractID := contract.ContractID
	coopID := contract.CoopID
	contractHash := contract.ContractHash
	contract.mutex.Unlock()

	coopStatus, err := fetchCoopStatusForDeflectors(contractID, coopID, eeidOverride)
	if err != nil || coopStatus == nil {
		if err != nil {
			log.Printf("updateCoopDeflectorsFromStatus: unable to fetch coop_status for %s/%s (falling back to estimated deflectors): %v", contractID, coopID, err)
		}
		updateActiveBoostScheduleMessages(getActiveBoostClient(), contract)
		return
	}
	if coopStatus.GetResponseStatus() != ei.ContractCoopStatusResponse_NO_ERROR {
		log.Printf("updateCoopDeflectorsFromStatus: coop_status returned %s for %s/%s (falling back to estimated deflectors)",
			coopStatus.GetResponseStatus().String(), contractID, coopID)
		updateActiveBoostScheduleMessages(getActiveBoostClient(), contract)
		return
	}

	applyCoopStatusDeflectors(contract, coopStatus)
	if contractHash != "" {
		saveData(contractHash)
	}
}
