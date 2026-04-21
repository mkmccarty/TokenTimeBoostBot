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
	backup := &Backup{
		EiUserId:   &eiUserID,
		UserName:   &userName,
		ApproxTime: &now,
		Game: &Backup_Game{
			GoldenEggsEarned: new(uint64),
			EggsOfProphecy:   new(uint64),
		},
		ArtifactsDb: &ArtifactsDB{
			ItemSequence: new(uint64),
		},
		Virtue: &Backup_Virtue{
			Afx: &Backup_Artifacts{},
		},
	}

	// Initialize home farm
	homeFarm := &Backup_Simulation{
		FarmType: FarmType_HOME.Enum(),
		EggType:  Egg_EDIBLE.Enum(),
	}

	// Initialize common research for the home farm
	homeFarm.CommonResearch = make([]*Backup_ResearchItem, len(EggIncResearches))
	for i, researchData := range EggIncResearches {
		id := researchData.ID
		homeFarm.CommonResearch[i] = &Backup_ResearchItem{
			Id:    &id,
			Level: uint32p(0),
		}
	}

	backup.Farms = append(backup.Farms, homeFarm)
	backup.Sim = homeFarm // Sim is the currently active farm

	maker := &BackupMaker{
		backup:      backup,
		nextItemID:  1,
		currentFarm: homeFarm,
	}

	maker.SetAllHabitatsTo(18) // Chicken Universe
	maker.SetAllVehiclesTo(11) // Hyperloop Train
	trains := make([]uint32, 17)
	for i := range trains {
		trains[i] = 10 // Max train length
	}
	maker.currentFarm.TrainLength = trains

	maker.SetAllResearchThroughTier(100)

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
	for i, researchData := range EggIncResearches {
		if researchData.Tier > 0 && uint32(researchData.Tier) <= tier {
			maxLevel := uint32(researchData.Levels)
			b.currentFarm.CommonResearch[i].Level = &maxLevel
		}
	}
	return b
}

// setResearchTypeToLevel is a helper to set a category of research to a specific level.
func (b *BackupMaker) setResearchTypeToLevel(level uint32, isType func(string) bool) *BackupMaker {
	for i, researchData := range EggIncResearches {
		if isType(researchData.ID) {
			maxLevel := uint32(researchData.Levels)
			actualLevel := uint32(math.Min(float64(level), float64(maxLevel)))
			b.currentFarm.CommonResearch[i].Level = &actualLevel
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

func float64p(f float64) *float64 {
	return &f
}

func uint32p(i uint32) *uint32 {
	return &i
}

func stringp(s string) *string {
	return &s
}
