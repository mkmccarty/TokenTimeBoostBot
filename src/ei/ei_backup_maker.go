package ei

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// BackupMaker is a builder for creating test Backup objects.
type BackupMaker struct {
	backup      *Backup
	nextItemID  uint64
	currentFarm *Backup_Simulation
}

// NewBackupMaker creates a new BackupMaker with a minimal backup structure.
func NewBackupMaker(eiUserID, userName string) *BackupMaker {
	now := float64(time.Now().Unix())

	lastFueledShip := MissionInfo_Spaceship(10)
	tankFuelsArray := make([]float64, 25)
	tankLimitsArray := make([]float64, 25)
	for i := range tankLimitsArray {
		tankLimitsArray[i] = 1.0
	}

	periodEnd := float64(time.Now().AddDate(0, 1, 0).Unix())
	subHistory := make([]*UserSubscriptionInfo_HistoryEntry, 0)
	subStart := time.Now().AddDate(0, -6, 0)

	subHistory = append(subHistory, &UserSubscriptionInfo_HistoryEntry{
		Timestamp: float64p(float64(subStart.Unix())),
		Message:   stringp("Subscription verified and updated at request of client."),
	})
	subHistory = append(subHistory, &UserSubscriptionInfo_HistoryEntry{
		Timestamp: float64p(float64(subStart.Unix()) + 13.0),
		Message:   stringp("SUBSCRIBED"),
	})

	for i := 1; i <= 6; i++ {
		renewTime := subStart.AddDate(0, i, 0)
		if renewTime.After(time.Now()) {
			break
		}
		subHistory = append(subHistory, &UserSubscriptionInfo_HistoryEntry{
			Timestamp: float64p(float64(renewTime.Unix())),
			Message:   stringp("DID_RENEW"),
		})
	}

	backup := &Backup{
		EiUserId:   &eiUserID,
		UserName:   &userName,
		ApproxTime: &now,
		Version:    uint32p(DefaultClientVersion),
		Game: &Backup_Game{
			CurrentFarm:               uint32p(0),
			MaxEggReached:             Egg_KINDNESS.Enum(),
			GoldenEggsEarned:          uint64p(1_000_000_000),
			GoldenEggsSpent:           uint64p(500_000),
			UncliamedGoldenEggs:       uint64p(0), // Typo inherited directly from Egg Inc. Protobufs
			SoulEggsD:                 float64p(4.0e+23),
			UnclaimedSoulEggsD:        float64p(0),
			EggsOfProphecy:            uint64p(232),
			UnclaimedEggsOfProphecy:   uint64p(0),
			ShellScriptsEarned:        uint64p(0),
			ShellScriptsSpent:         uint64p(0),
			UnclaimedShellScripts:     uint64p(0),
			PrestigeCashEarned:        float64p(1.5741864631873203e+66),
			PrestigeSoulBoostCash:     float64p(0),
			LifetimeCashEarned:        float64p(6.968358395984488e+93),
			PiggyBank:                 uint64p(500_000_000),
			PiggyFullAlertShown:       boolp(true),
			PermitLevel:               uint32p(1),
			HyperloopStation:          boolp(true),
			NextDailyGiftTime:         float64p(1776643200.0005379),
			LastDailyGiftCollectedDay: uint32p(9605),
			NumDailyGiftsCollected:    uint32p(2558),
		},
		Artifacts: &Backup_Artifacts{
			FlowPercentageArtifacts: float64p(1.0),
			FuelingEnabled:          boolp(true),
			TankFillingEnabled:      boolp(false),
			TankLevel:               uint32p(7),
			TankFuels:               tankFuelsArray,
			TankLimits:              tankLimitsArray,
			LastFueledShip:          &lastFueledShip,
			InventoryScore:          float64p(285240.2899999999),
			CraftingXp:              float64p(6659529454),
			Enabled:                 boolp(true),
			IntroShown:              boolp(true),
		},
		Virtue: &Backup_Virtue{
			Afx: &Backup_Artifacts{},
		},
		ArtifactsDb: &ArtifactsDB{
			ItemSequence: new(uint64),
		},
		SubInfo: &UserSubscriptionInfo{
			SubscriptionLevel:     userSubscriptionInfoLevelp(UserSubscriptionInfo_PRO),
			NextSubscriptionLevel: userSubscriptionInfoLevelp(UserSubscriptionInfo_PRO),
			Platform:              platformp(Platform_IOS),
			PeriodEnd:             &periodEnd,
			Status:                userSubscriptionInfoStatusp(UserSubscriptionInfo_ACTIVE),
			StoreStatus:           stringp("1"),
			AutoRenew:             boolp(true),
			Sandbox:               boolp(false),
			History:               subHistory,
		},
	}

	// Initialize home farm to Virtue Farm
	homeFarm := &Backup_Simulation{
		FarmType:                    FarmType_HOME.Enum(),
		EggType:                     Egg_CURIOSITY.Enum(),
		ContractId:                  stringp(""),
		CashEarned:                  float64p(2.292897726703627e+37),
		CashSpent:                   float64p(1.24e+36),
		UnclaimedCash:               float64p(0),
		LastStepTime:                float64p(1776634308.0993152),
		NumChickens:                 uint64p(9684360000),
		NumChickensUnsettled:        uint64p(0),
		NumChickensRunning:          uint64p(0),
		EggsLaid:                    float64p(2420370378219646000),
		EggsShipped:                 float64p(2419740504446384000),
		EggsPaidFor:                 float64p(2419740504446384000),
		SilosOwned:                  uint32p(8),
		HatcheryPopulation:          float64p(500),
		LastCashBoostTime:           float64p(0),
		TimeCheatsDetected:          uint32p(0),
		BoostTokensReceived:         uint32p(0),
		BoostTokensSpent:            uint32p(0),
		BoostTokensGiven:            uint32p(0),
		UnclaimedBoostTokens:        uint32p(0),
		GametimeUntilNextBoostToken: float64p(3600),
		TotalStepTime:               float64p(6352188.731107798),
	}

	// Initialize common research for the home farm
	defaultCommonResearch := []struct {
		id    string
		level uint32
	}{
		{"comfy_nests", 50},
		{"nutritional_sup", 40},
		{"better_incubators", 15},
		{"excitable_chickens", 25},
		{"hab_capacity1", 8},
		{"internal_hatchery1", 10},
		{"padded_packaging", 30},
		{"hatchery_expansion", 10},
		{"bigger_eggs", 1},
		{"internal_hatchery2", 10},
		{"leafsprings", 30},
		{"vehicle_reliablity", 2},
		{"rooster_booster", 25},
		{"coordinated_clucking", 50},
		{"hatchery_rebuild1", 1},
		{"usde_prime", 1},
		{"hen_house_ac", 50},
		{"superfeed", 35},
		{"microlux", 10},
		{"compact_incubators", 10},
		{"lightweight_boxes", 40},
		{"excoskeletons", 2},
		{"internal_hatchery3", 15},
		{"improved_genetics", 30},
		{"traffic_management", 2},
		{"motivational_clucking", 50},
		{"driver_training", 30},
		{"shell_fortification", 60},
		{"egg_loading_bots", 2},
		{"super_alloy", 50},
		{"even_bigger_eggs", 5},
		{"internal_hatchery4", 30},
		{"quantum_storage", 20},
		{"genetic_purification", 100},
		{"internal_hatchery5", 250},
		{"time_compress", 20},
		{"hover_upgrades", 25},
		{"graviton_coating", 7},
		{"grav_plating", 25},
		{"chrystal_shells", 100},
		{"autonomous_vehicles", 5},
		{"neural_linking", 30},
		{"telepathic_will", 50},
		{"enlightened_chickens", 150},
		{"dark_containment", 25},
		{"atomic_purification", 50},
		{"multi_layering", 3},
		{"timeline_diversion", 50},
		{"wormhole_dampening", 25},
		{"eggsistor", 100},
		{"micro_coupling", 5},
		{"neural_net_refine", 25},
		{"matter_reconfig", 500},
		{"timeline_splicing", 1},
		{"hyper_portalling", 25},
		{"relativity_optimization", 10},
	}

	homeFarm.CommonResearch = make([]*Backup_ResearchItem, 0, len(defaultCommonResearch))
	for _, cr := range defaultCommonResearch {
		idCopy := cr.id
		levelCopy := cr.level
		homeFarm.CommonResearch = append(homeFarm.CommonResearch, &Backup_ResearchItem{
			Id:    &idCopy,
			Level: &levelCopy,
		})
	}

	backup.Farms = append(backup.Farms, homeFarm)
	backup.Sim = homeFarm // Sim is the currently active farm

	maker := &BackupMaker{
		backup:      backup,
		nextItemID:  1,
		currentFarm: homeFarm,
	}

	eovEarned := []uint32{20, 20, 21, 20, 20}
	eggsDelivered := []float64{671228376732604700, 281084978380496960, 1861466584249923000, 321860104553812600, 766522163167315600}
	maker.SetVirtueData(36, 1, eovEarned, eggsDelivered, 11215366.125829924)

	tankFuels := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 77804200160814.05, 247867287321.0276, 1759725104048.5757, 65000000000001.2, 75480247097414}
	tankLimits := []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0.36, 0.02, 0.45, 0.29, 0.35}
	maker.SetVirtueAFX(1.0, true, true, tankFuels, tankLimits, 9, 48448.569999999956)

	maker.SetAllHabitatsTo(18) // Chicken Universe
	maker.SetAllVehiclesTo(11) // Hyperloop Train
	trains := make([]uint32, 17)
	for i := range trains {
		trains[i] = 10 // Max train length
	}
	maker.currentFarm.TrainLength = trains

	defaultEpicResearch := []struct {
		id    string
		level uint32
	}{
		{"hold_to_hatch", 15},
		{"epic_hatchery", 20},
		{"epic_internal_incubators", 20},
		{"video_doubler_time", 12},
		{"epic_clucking", 20},
		{"epic_multiplier", 100},
		{"cheaper_contractors", 10},
		{"bust_unions", 10},
		{"cheaper_research", 10},
		{"epic_silo_quality", 40},
		{"silo_capacity", 20},
		{"int_hatch_sharing", 10},
		{"int_hatch_calm", 20},
		{"accounting_tricks", 20},
		{"hold_to_research", 20},
		{"soul_eggs", 140},
		{"prestige_bonus", 20},
		{"drone_rewards", 20},
		{"epic_egg_laying", 20},
		{"transportation_lobbyist", 30},
		{"warp_shift", 16},
		{"prophecy_bonus", 5},
		{"afx_mission_time", 60},
		{"afx_mission_capacity", 10},
	}

	backup.Game.EpicResearch = make([]*Backup_ResearchItem, 0, len(defaultEpicResearch))
	for _, er := range defaultEpicResearch {
		idCopy := er.id
		levelCopy := er.level
		backup.Game.EpicResearch = append(backup.Game.EpicResearch, &Backup_ResearchItem{
			Id:    &idCopy,
			Level: &levelCopy,
		})
	}

	var artifacts = []ArtifactSpec_Name{
		ArtifactSpec_LUNAR_TOTEM, ArtifactSpec_NEODYMIUM_MEDALLION, ArtifactSpec_BEAK_OF_MIDAS,
		ArtifactSpec_LIGHT_OF_EGGENDIL, ArtifactSpec_DEMETERS_NECKLACE, ArtifactSpec_VIAL_MARTIAN_DUST,
		ArtifactSpec_ORNATE_GUSSET, ArtifactSpec_THE_CHALICE, ArtifactSpec_BOOK_OF_BASAN,
		ArtifactSpec_PHOENIX_FEATHER, ArtifactSpec_TUNGSTEN_ANKH, ArtifactSpec_AURELIAN_BROOCH,
		ArtifactSpec_CARVED_RAINSTICK, ArtifactSpec_PUZZLE_CUBE, ArtifactSpec_QUANTUM_METRONOME,
		ArtifactSpec_SHIP_IN_A_BOTTLE, ArtifactSpec_TACHYON_DEFLECTOR, ArtifactSpec_INTERSTELLAR_COMPASS,
		ArtifactSpec_DILITHIUM_MONOCLE, ArtifactSpec_TITANIUM_ACTUATOR, ArtifactSpec_MERCURYS_LENS,
	}
	var stones = []ArtifactSpec_Name{
		ArtifactSpec_TACHYON_STONE, ArtifactSpec_DILITHIUM_STONE, ArtifactSpec_SHELL_STONE,
		ArtifactSpec_LUNAR_STONE, ArtifactSpec_SOUL_STONE, ArtifactSpec_PROPHECY_STONE,
		ArtifactSpec_QUANTUM_STONE, ArtifactSpec_TERRA_STONE, ArtifactSpec_LIFE_STONE,
		ArtifactSpec_CLARITY_STONE,
	}

	for _, art := range artifacts {
		maker.AddArtifact(art, ArtifactSpec_GREATER, ArtifactSpec_LEGENDARY, 1, false)
		maker.AddArtifact(art, ArtifactSpec_GREATER, ArtifactSpec_LEGENDARY, 1, true)
	}
	for _, stone := range stones {
		maker.AddArtifact(stone, ArtifactSpec_GREATER, ArtifactSpec_COMMON, 12, false)
		maker.AddArtifact(stone, ArtifactSpec_GREATER, ArtifactSpec_COMMON, 12, true)
	}

	return maker
}

// GetBackup returns the built Backup object.
func (b *BackupMaker) GetBackup() *Backup {
	return b.backup
}

// SetVirtueData populates the primary Virtue message fields in the backup.
func (b *BackupMaker) SetVirtueData(shiftCount, resets uint32, eovEarned []uint32, eggsDelivered []float64, pastSimTime float64) {
	if b.backup.Virtue == nil {
		b.backup.Virtue = &Backup_Virtue{}
	}
	b.backup.Virtue.ShiftCount = &shiftCount
	b.backup.Virtue.Resets = &resets
	b.backup.Virtue.EovEarned = eovEarned
	b.backup.Virtue.EggsDelivered = eggsDelivered
	b.backup.Virtue.PastSimTime = &pastSimTime
}

// SetVirtueAFX populates the AFX (ArtifactsDB) subset of the Virtue message in the backup.
func (b *BackupMaker) SetVirtueAFX(flowPercentage float64, fuelingEnabled, tankFillingEnabled bool, tankFuels, tankLimits []float64, lastFueledShip uint32, inventoryScore float64) {
	if b.backup.Virtue == nil {
		b.backup.Virtue = &Backup_Virtue{}
	}
	ship := MissionInfo_Spaceship(lastFueledShip)
	b.backup.Virtue.Afx = &Backup_Artifacts{
		FlowPercentageArtifacts: &flowPercentage,
		FuelingEnabled:          &fuelingEnabled,
		TankFillingEnabled:      &tankFillingEnabled,
		TankFuels:               tankFuels,
		TankLimits:              tankLimits,
		LastFueledShip:          &ship,
		InventoryScore:          &inventoryScore,
	}
}

// --- Farm Setup ---

// SetAllResearchThroughTier sets all common research up to a given tier to its max level.
func (b *BackupMaker) SetAllResearchThroughTier(tier uint32) *BackupMaker {
	for _, researchData := range EggIncResearches {
		if researchData.Tier > 0 && uint32(researchData.Tier) <= tier {
			maxLevel := uint32(researchData.Levels)
			for _, cr := range b.currentFarm.CommonResearch {
				if cr.GetId() == researchData.ID {
					cr.Level = &maxLevel
					break
				}
			}
		}
	}
	return b
}

// setResearchTypeToLevel is a helper to set a category of research to a specific level.
func (b *BackupMaker) setResearchTypeToLevel(level uint32, isType func(string) bool) *BackupMaker {
	for _, researchData := range EggIncResearches {
		if isType(researchData.ID) {
			maxLevel := uint32(researchData.Levels)
			actualLevel := uint32(math.Min(float64(level), float64(maxLevel)))
			for _, cr := range b.currentFarm.CommonResearch {
				if cr.GetId() == researchData.ID {
					cr.Level = &actualLevel
					break
				}
			}
		}
	}
	return b
}

// SetEggValueResearch sets all egg value research to a specific level.
func (b *BackupMaker) SetEggValueResearch(level uint32) *BackupMaker {
	return b.setResearchTypeToLevel(level, isEggValue)
}

// SetShippingRateResearch sets all shipping rate research to a specific level.
func (b *BackupMaker) SetShippingRateResearch(level uint32) *BackupMaker {
	return b.setResearchTypeToLevel(level, isShippingRate)
}

// SetLayRateResearch sets all egg laying rate research to a specific level.
func (b *BackupMaker) SetLayRateResearch(level uint32) *BackupMaker {
	return b.setResearchTypeToLevel(level, isLayRate)
}

// SetHabCapacityResearch sets all hab capacity research to a specific level.
func (b *BackupMaker) SetHabCapacityResearch(level uint32) *BackupMaker {
	return b.setResearchTypeToLevel(level, isHabCapacity)
}

// SetVehicleResearch sets all vehicle research to a specific level.
func (b *BackupMaker) SetVehicleResearch(level uint32) *BackupMaker {
	return b.setResearchTypeToLevel(level, isVehicleResearch)
}

// SetAllVehiclesTo sets all vehicle types to a specific level.
func (b *BackupMaker) SetAllVehiclesTo(level uint32) *BackupMaker {
	vehicles := make([]uint32, 17)
	for i := range vehicles {
		vehicles[i] = level
	}
	b.currentFarm.Vehicles = vehicles
	return b
}

// SetAllHabitatsTo sets all habitat types to a specific level.
func (b *BackupMaker) SetAllHabitatsTo(level uint32) *BackupMaker {
	habs := make([]uint32, 4)
	for i := range habs {
		habs[i] = level
	}
	b.currentFarm.Habs = habs
	return b
}

// --- Artifacts ---

func (b *BackupMaker) getInventory(virtue bool) *[]*ArtifactInventoryItem {
	if virtue {
		if b.backup.ArtifactsDb.VirtueAfxDb == nil {
			b.backup.ArtifactsDb.VirtueAfxDb = &ArtifactsDB_VirtueDB{}
		}
		return &b.backup.ArtifactsDb.VirtueAfxDb.InventoryItems
	}
	return &b.backup.ArtifactsDb.InventoryItems
}

// AddArtifact adds an artifact to the inventory. Returns the ItemID.
func (b *BackupMaker) AddArtifact(name ArtifactSpec_Name, level ArtifactSpec_Level, rarity ArtifactSpec_Rarity, quantity uint64, virtue bool) (uint64, *BackupMaker) {
	inventory := b.getInventory(virtue)

	itemID := b.nextItemID
	b.nextItemID++
	*b.backup.ArtifactsDb.ItemSequence = b.nextItemID

	item := &ArtifactInventoryItem{
		ItemId:   &itemID,
		Quantity: float64p(float64(quantity)),
		Artifact: &CompleteArtifact{
			Spec: &ArtifactSpec{
				Name:   name.Enum(),
				Level:  level.Enum(),
				Rarity: rarity.Enum(),
				Egg:    Egg_UNKNOWN.Enum(),
			},
		},
	}

	*inventory = append(*inventory, item)
	return itemID, b
}

// AddStone adds a stone to the inventory. Returns the ItemID.
func (b *BackupMaker) AddStone(name ArtifactSpec_Name, level ArtifactSpec_Level, quantity uint64, virtue bool) (uint64, *BackupMaker) {
	return b.AddArtifact(name, level, ArtifactSpec_COMMON, quantity, virtue)
}

// AddMetronome adds a metronome to the inventory. Returns the ItemID.
func (b *BackupMaker) AddMetronome(level ArtifactSpec_Level, rarity ArtifactSpec_Rarity, quantity uint64, virtue bool) (uint64, *BackupMaker) {
	return b.AddArtifact(ArtifactSpec_QUANTUM_METRONOME, level, rarity, quantity, virtue)
}

// AssignStonesToArtifact slots stones into an artifact.
// It finds the artifact by its itemID and the stones by their itemIDs,
// then moves the stones from the inventory into the artifact's slots.
func (b *BackupMaker) AssignStonesToArtifact(artifactItemID uint64, stoneItemIDs []uint64, virtue bool) (*BackupMaker, error) {
	inventory := b.getInventory(virtue)

	var targetArtifact *ArtifactInventoryItem
	for _, item := range *inventory {
		if item.GetItemId() == artifactItemID {
			targetArtifact = item
			break
		}
	}

	if targetArtifact == nil {
		return b, fmt.Errorf("artifact with itemID %d not found", artifactItemID)
	}

	stonesToSlot := make(map[uint64]bool)
	for _, id := range stoneItemIDs {
		stonesToSlot[id] = true
	}

	newInventory := []*ArtifactInventoryItem{}
	for _, item := range *inventory {
		if stonesToSlot[item.GetItemId()] {
			nameStr := item.GetArtifact().GetSpec().GetName().String()
			if !strings.HasSuffix(nameStr, "_STONE") {
				return b, fmt.Errorf("item with ID %d is not a stone", item.GetItemId())
			}
			targetArtifact.Artifact.Stones = append(targetArtifact.Artifact.Stones, item.GetArtifact().GetSpec())

			newQuantity := item.GetQuantity() - 1
			if newQuantity > 0 {
				item.Quantity = float64p(newQuantity)
				newInventory = append(newInventory, item)
			}
		} else {
			newInventory = append(newInventory, item)
		}
	}

	*inventory = newInventory

	return b, nil
}

// --- Game and Stats Setup ---

// SetEggTotal sets the total eggs laid for a specific egg type.
func (b *BackupMaker) SetEggTotal(egg Egg, total float64) *BackupMaker {
	if b.backup.Stats == nil {
		b.backup.Stats = &Backup_Stats{}
	}
	eggInt := int(egg)
	if len(b.backup.Stats.EggTotals) <= eggInt {
		newTotals := make([]float64, eggInt+1)
		copy(newTotals, b.backup.Stats.EggTotals)
		b.backup.Stats.EggTotals = newTotals
	}
	b.backup.Stats.EggTotals[eggInt] = total
	return b
}

// SetAllEggTotals sets the total eggs laid for all egg types.
func (b *BackupMaker) SetAllEggTotals(total float64) *BackupMaker {
	if b.backup.Stats == nil {
		b.backup.Stats = &Backup_Stats{}
	}
	totals := make([]float64, 32) // Typical size covering all eggs
	for i := range totals {
		totals[i] = total
	}
	b.backup.Stats.EggTotals = totals
	return b
}

// SetEggsOfProphecy sets the number of prophecy eggs.
func (b *BackupMaker) SetEggsOfProphecy(count uint64) *BackupMaker {
	b.backup.Game.EggsOfProphecy = &count
	return b
}

// SetSoulEggsD sets the number of soul eggs (as a float64/double).
func (b *BackupMaker) SetSoulEggsD(count float64) *BackupMaker {
	b.backup.Game.SoulEggsD = &count
	return b
}

// SetPermitLevel sets the pro permit level (0 for standard, 1 for pro).
func (b *BackupMaker) SetPermitLevel(level uint32) *BackupMaker {
	b.backup.Game.PermitLevel = &level
	return b
}

// --- Contracts and Colleggtibles ---

// AddColleggtibleContract adds a past contract to the archive to simulate earning a Colleggtible.
// farmSize determines the tier (e.g., 1e10 for the highest tier).
func (b *BackupMaker) AddColleggtibleContract(customEggID string, farmSize float64) *BackupMaker {
	if b.backup.Contracts == nil {
		b.backup.Contracts = &MyContracts{}
	}
	b.backup.Contracts.Archive = append(b.backup.Contracts.Archive, &LocalContract{
		Contract: &Contract{
			Egg:         Egg_CUSTOM_EGG.Enum(),
			CustomEggId: stringp(customEggID),
		},
		MaxFarmSizeReached: float64p(farmSize),
	})
	return b
}

// AddMaxColleggtibleContract adds a past contract for a custom egg with enough farm size (10 billion) for the maximum tier.
func (b *BackupMaker) AddMaxColleggtibleContract(customEggID string) *BackupMaker {
	return b.AddColleggtibleContract(customEggID, 1e10)
}

// --- Pointer Helpers ---

func platformp(p Platform) *Platform {
	return &p
}

func float64p(f float64) *float64 {
	return &f
}

func uint32p(i uint32) *uint32 {
	return &i
}

func stringp(s string) *string {
	return &s
}

func boolp(b bool) *bool {
	return &b
}

func uint64p(i uint64) *uint64 {
	return &i
}

func userSubscriptionInfoLevelp(level UserSubscriptionInfo_Level) *UserSubscriptionInfo_Level {
	return &level
}

func userSubscriptionInfoStatusp(status UserSubscriptionInfo_Status) *UserSubscriptionInfo_Status {
	return &status
}
