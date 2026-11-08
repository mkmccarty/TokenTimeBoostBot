package farm

import (
	"encoding/base64"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

func setupDeliveryTestResearches() {
	if ei.EggIncResearchesMap == nil {
		ei.EggIncResearchesMap = make(map[string]ei.EggResearches)
	}
	ensureResearch := func(id, effectType string, perLevel float64, compound string) {
		ei.EggIncResearchesMap[id] = ei.EggResearches{
			ID:             id,
			EffectType:     effectType,
			PerLevel:       perLevel,
			LevelsCompound: compound,
		}
	}
	ensureResearch("comfy_nests", "multiplicative", 0.1, "additive")
	ensureResearch("hen_house_ac", "multiplicative", 0.05, "additive")
	ensureResearch("improved_genetics", "multiplicative", 0.15, "additive")
	ensureResearch("time_compress", "multiplicative", 0.1, "additive")
	ensureResearch("timeline_diversion", "multiplicative", 0.02, "additive")
	ensureResearch("relativity_optimization", "multiplicative", 0.1, "additive")
	ensureResearch("epic_egg_laying", "multiplicative", 0.05, "additive")
	ensureResearch("hab_capacity1", "multiplicative", 0.05, "additive")
	ensureResearch("microlux", "multiplicative", 0.05, "additive")
	ensureResearch("grav_plating", "multiplicative", 0.02, "additive")
	ensureResearch("wormhole_dampening", "multiplicative", 0.02, "additive")
	ensureResearch("leafsprings", "multiplicative", 0.05, "additive")
	ensureResearch("lightweight_boxes", "multiplicative", 0.1, "additive")
	ensureResearch("driver_training", "multiplicative", 0.05, "additive")
	ensureResearch("super_alloy", "multiplicative", 0.05, "additive")
	ensureResearch("quantum_storage", "multiplicative", 0.05, "additive")
	ensureResearch("dark_containment", "multiplicative", 0.05, "additive")
	ensureResearch("neural_net_refine", "multiplicative", 0.05, "additive")
	ensureResearch("hyper_portalling", "multiplicative", 0.05, "additive")
	ensureResearch("transportation_lobbyist", "multiplicative", 0.05, "additive")
	ensureResearch("hover_upgrades", "multiplicative", 0.05, "additive")

	if ei.CustomEggMap == nil {
		ei.CustomEggMap = make(map[string]*ei.EggIncCustomEgg)
	}
	setCustomEgg := func(id string, dim ei.GameModifier_GameDimension) {
		ei.CustomEggMap[id] = &ei.EggIncCustomEgg{
			ID:             id,
			Dimension:      dim,
			DimensionValue: []float64{1.01, 1.02, 1.03, 1.05},
		}
	}
	setCustomEgg("carbon-fiber", ei.GameModifier_SHIPPING_CAPACITY)
	setCustomEgg("pumpkin", ei.GameModifier_SHIPPING_CAPACITY)
	setCustomEgg("silicon", ei.GameModifier_EGG_LAYING_RATE)
	setCustomEgg("gatoregg", ei.GameModifier_EGG_LAYING_RATE)
	setCustomEgg("pegg", ei.GameModifier_HAB_CAPACITY)
}

func TestOptimizeDeliveryCalculation(t *testing.T) {
	setupDeliveryTestResearches()

	tachyonStoneSpec := &ei.ArtifactSpec{
		Name:  ei.ArtifactSpec_TACHYON_STONE.Enum(),
		Level: ei.ArtifactSpec_NORMAL.Enum(),
	}
	quantumStoneSpec := &ei.ArtifactSpec{
		Name:  ei.ArtifactSpec_QUANTUM_STONE.Enum(),
		Level: ei.ArtifactSpec_NORMAL.Enum(),
	}

	equippedMetronome := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_QUANTUM_METRONOME.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(),
			Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
		},
		Stones: []*ei.ArtifactSpec{tachyonStoneSpec, tachyonStoneSpec, tachyonStoneSpec},
	}
	equippedCompass := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_INTERSTELLAR_COMPASS.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(),
			Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
		},
		Stones: []*ei.ArtifactSpec{quantumStoneSpec, quantumStoneSpec},
	}
	equippedGusset := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_ORNATE_GUSSET.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(),
			Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
		},
		Stones: []*ei.ArtifactSpec{tachyonStoneSpec, tachyonStoneSpec, tachyonStoneSpec},
	}
	equippedChalice := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_THE_CHALICE.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(),
			Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
		},
		Stones: []*ei.ArtifactSpec{quantumStoneSpec, quantumStoneSpec, tachyonStoneSpec},
	}

	inventory := []*ei.ArtifactInventoryItem{
		{ItemId: protoUint64(1), Artifact: equippedMetronome, Quantity: protoFloat64(1)},
		{ItemId: protoUint64(2), Artifact: equippedCompass, Quantity: protoFloat64(1)},
		{ItemId: protoUint64(3), Artifact: equippedGusset, Quantity: protoFloat64(1)},
		{ItemId: protoUint64(4), Artifact: equippedChalice, Quantity: protoFloat64(1)},
	}

	activeSlots := []*ei.ActiveArtifactSlot{
		{Occupied: protoBool(true), ItemId: protoUint64(1)},
		{Occupied: protoBool(true), ItemId: protoUint64(2)},
		{Occupied: protoBool(true), ItemId: protoUint64(3)},
		{Occupied: protoBool(true), ItemId: protoUint64(4)},
	}

	commonResearch := []*ei.Backup_ResearchItem{
		{Id: protoStr("comfy_nests"), Level: protoUint32(50)},
		{Id: protoStr("hen_house_ac"), Level: protoUint32(50)},
		{Id: protoStr("improved_genetics"), Level: protoUint32(30)},
		{Id: protoStr("time_compress"), Level: protoUint32(20)},
		{Id: protoStr("timeline_diversion"), Level: protoUint32(36)},
		{Id: protoStr("hab_capacity1"), Level: protoUint32(8)},
		{Id: protoStr("microlux"), Level: protoUint32(10)},
		{Id: protoStr("grav_plating"), Level: protoUint32(25)},
		{Id: protoStr("wormhole_dampening"), Level: protoUint32(8)},
		{Id: protoStr("leafsprings"), Level: protoUint32(30)},
		{Id: protoStr("lightweight_boxes"), Level: protoUint32(40)},
		{Id: protoStr("driver_training"), Level: protoUint32(30)},
		{Id: protoStr("super_alloy"), Level: protoUint32(50)},
		{Id: protoStr("quantum_storage"), Level: protoUint32(20)},
		{Id: protoStr("dark_containment"), Level: protoUint32(23)},
		{Id: protoStr("neural_net_refine"), Level: protoUint32(11)},
		{Id: protoStr("hover_upgrades"), Level: protoUint32(25)},
	}

	epicResearch := []*ei.Backup_ResearchItem{
		{Id: protoStr("epic_egg_laying"), Level: protoUint32(20)},
		{Id: protoStr("transportation_lobbyist"), Level: protoUint32(30)},
	}

	vehicles := make([]uint32, 17)
	trainLength := make([]uint32, 17)
	for i := 0; i < 17; i++ {
		vehicles[i] = 11 // Hyperloop
		trainLength[i] = 8
	}

	habs := []uint32{18, 18, 18, 18}
	habPopulation := []uint64{2877525000, 2877525000, 2877525000, 2877525000}

	farm := &ei.Backup_Simulation{
		EggType:        ei.Egg_KINDNESS.Enum(),
		FarmType:       ei.FarmType_HOME.Enum(),
		Habs:           habs,
		HabPopulation:  habPopulation,
		Vehicles:       vehicles,
		TrainLength:    trainLength,
		CommonResearch: commonResearch,
	}

	contracts := &ei.MyContracts{
		ColleggtibleMaxFarmSizeReached: []*ei.ColleggtibleMaxFarmSize{
			{EggId: protoStr("carbon-fiber"), MaxFarmSizeReached: protoFloat64(14883750000)},
			{EggId: protoStr("pumpkin"), MaxFarmSizeReached: protoFloat64(14883750000)},
			{EggId: protoStr("silicon"), MaxFarmSizeReached: protoFloat64(14883750000)},
			{EggId: protoStr("gatoregg"), MaxFarmSizeReached: protoFloat64(14883750000)},
			{EggId: protoStr("pegg"), MaxFarmSizeReached: protoFloat64(14883750000)},
		},
	}

	backup := ei.Backup{
		UserId:   protoStr("EI1234567890123456"),
		UserName: protoStr("DeliveryTester"),
		Farms:    []*ei.Backup_Simulation{farm},
		Game: &ei.Backup_Game{
			PermitLevel:  protoUint32(1),
			EpicResearch: epicResearch,
		},
		Contracts: contracts,
		ArtifactsDb: &ei.ArtifactsDB{
			VirtueAfxDb: &ei.ArtifactsDB_VirtueDB{
				InventoryItems: inventory,
				ActiveArtifacts: &ei.ArtifactsDB_ActiveArtifactSet{
					Slots: activeSlots,
				},
			},
		},
	}

	t.Logf("contracts.Colleggtibles: %d", len(contracts.GetColleggtibleMaxFarmSizeReached()))
	for _, s := range contracts.GetColleggtibleMaxFarmSizeReached() {
		t.Logf("colleggtible: egg=%s, maxFarmSize=%f", s.GetEggId(), s.GetMaxFarmSizeReached())
	}
	colBuffs := ei.GetColleggtibleBuffs(contracts)
	t.Logf("colBuffs: ELR=%f, SR=%f, Hab=%f", colBuffs.ELR, colBuffs.SR, colBuffs.Hab)

	res := OptimizeDelivery(&backup, farm)
	t.Logf("LayingRate: %e (%s)", res.LayingRate, ei.FormatEIValue(res.LayingRate, map[string]any{"decimals": 2, "trim": true}))
	t.Logf("ShippingRate: %e (%s)", res.ShippingRate, ei.FormatEIValue(res.ShippingRate, map[string]any{"decimals": 2, "trim": true}))
	t.Logf("DeliveryRate: %e (%s)", res.DeliveryRate, ei.FormatEIValue(res.DeliveryRate, map[string]any{"decimals": 2, "trim": true}))
	t.Logf("CurrentLayingRate: %e (%s)", res.CurrentLayingRate, ei.FormatEIValue(res.CurrentLayingRate, map[string]any{"decimals": 2, "trim": true}))
	t.Logf("CurrentShippingRate: %e (%s)", res.CurrentShippingRate, ei.FormatEIValue(res.CurrentShippingRate, map[string]any{"decimals": 2, "trim": true}))
	t.Logf("CurrentDeliveryRate: %e (%s)", res.CurrentDeliveryRate, ei.FormatEIValue(res.CurrentDeliveryRate, map[string]any{"decimals": 2, "trim": true}))
	t.Logf("Delivery diff: %v, current >= opt: %v", res.DeliveryRate-res.CurrentDeliveryRate, res.CurrentDeliveryRate >= res.DeliveryRate)
	t.Logf("IsAlreadyOptimal: %v", res.IsAlreadyOptimal)

	// Laying rate should be around 3.44q/hr (3.44e15), not 28.2S/hr (2.82e25)
	if res.LayingRate > 1e16 || res.LayingRate < 1e15 {
		t.Errorf("LayingRate %e is outside expected ~3.44q/hr range", res.LayingRate)
	}
	// Delivery rate should be around 3.36q/hr
	if res.DeliveryRate > 1e16 || res.DeliveryRate < 1e15 {
		t.Errorf("DeliveryRate %e is outside expected ~3.36q/hr range", res.DeliveryRate)
	}
	if !res.IsAlreadyOptimal {
		t.Errorf("expected setup to be already optimal, got false")
	}

	comps := printDeliverySetup("100000000000000001", &backup, farm, "", "")
	for i, c := range comps {
		switch tc := c.(type) {
		case dc.TextDisplay:
			t.Logf("comp %d (TextDisplay):\n%s", i, tc.Content)
		case dc.Section:
			for j, sc := range tc.Components {
				t.Logf("comp %d.%d (Section):\n%s", i, j, sc.Content)
			}
		}
	}
}

func makeTestEncryptedEID(t *testing.T, plainEID string) string {
	t.Helper()
	if config.Key == "" {
		key, err := config.GenerateKey()
		if err != nil {
			t.Fatalf("failed to generate config key: %v", err)
		}
		config.Key = base64.StdEncoding.EncodeToString(key)
	}
	keyBytes, err := base64.StdEncoding.DecodeString(config.Key)
	if err != nil {
		t.Fatalf("failed to decode config.Key: %v", err)
	}
	enc, err := config.EncryptAndCombine(keyBytes, []byte(plainEID))
	if err != nil {
		t.Fatalf("failed to encrypt test eid: %v", err)
	}
	return base64.StdEncoding.EncodeToString(enc)
}

func TestGetSlashFarmCommand(t *testing.T) {
	cmd := GetSlashFarmCommand("farm")
	if cmd.Name != "farm" {
		t.Errorf("cmd.Name = %q; want farm", cmd.Name)
	}

	if len(cmd.Options) != 3 {
		t.Fatalf("expected 3 subcommands, got %d", len(cmd.Options))
	}

	subMap := make(map[string]dc.SubCommand)
	for _, opt := range cmd.Options {
		sc, ok := opt.(dc.SubCommand)
		if !ok {
			t.Fatalf("expected option to be dc.SubCommand, got %T", opt)
		}
		subMap[sc.Name] = sc
	}

	for _, name := range []string{"delivery", "earnings", "drone-farming"} {
		sc, exists := subMap[name]
		if !exists {
			t.Fatalf("subcommand %q not found", name)
		}

		var hasAlt, hasHelp bool
		for _, o := range sc.Options {
			switch so := o.(type) {
			case dc.StringOption:
				if so.Name == "alt" {
					hasAlt = true
					if !so.Autocomplete {
						t.Errorf("subcommand %q alt option should have Autocomplete=true", name)
					}
				}
			case dc.BoolOption:
				if so.Name == "help" {
					hasHelp = true
				}
			}
		}

		if !hasAlt {
			t.Errorf("subcommand %q missing alt option", name)
		}
		if !hasHelp {
			t.Errorf("subcommand %q missing help option", name)
		}
	}
}

func TestHandleFarmAutocomplete(t *testing.T) {
	testUser := "100000000000000001"
	altUser := "100000000000000002"
	encEID := makeTestEncryptedEID(t, "EI1234567890123456")
	farmerstate.SetMiscSettingString(altUser, "encrypted_ei_id", encEID)
	farmerstate.SetMiscSettingString(altUser, "ei_ign", "AlphaFarmer")
	farmerstate.SetMiscSettingString(altUser, "AltController", testUser)

	event := dctest.SubcommandAutocompleteEvent("farm", "delivery", "alt", "alpha", testUser)
	HandleFarmAutocomplete(event)

	if len(event.LastChoices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(event.LastChoices))
	}
	if event.LastChoices[0].Value != altUser {
		t.Errorf("expected choice value %q, got %q", altUser, event.LastChoices[0].Value)
	}

	// Non-matching query
	eventNone := dctest.SubcommandAutocompleteEvent("farm", "delivery", "alt", "nonexistent", testUser)
	HandleFarmAutocomplete(eventNone)
	if len(eventNone.LastChoices) != 0 {
		t.Errorf("expected 0 choices for nonexistent filter, got %d", len(eventNone.LastChoices))
	}

	// Other option focused
	eventOther := dctest.SubcommandAutocompleteEvent("farm", "delivery", "other", "", testUser)
	HandleFarmAutocomplete(eventOther)
	if len(eventOther.LastChoices) != 0 {
		t.Errorf("expected 0 choices for non-alt option, got %d", len(eventOther.LastChoices))
	}
}

func TestOptimizeEarningsMath(t *testing.T) {
	// Setup test backup with Virtue farm
	// Totem T4L (200x, 3 slots), Necklace T4L (3x, 3 slots), Ankh T4L (2.5x, 3 slots), Gusset T4L (1.25x, 3 slots)
	// 12 T4 Lunar Stones (1.4x each)
	// Base multiplier: 200 * 3 * 2.5 = 1500
	// 1.4^12 = 56.693912
	// Total = 1500 * 56.693912 = 85,040.87 -> rounds to 85,041x

	game := &ei.Backup_Game{
		PermitLevel: protoUint32(1), // Pro permit
	}

	curiosityEgg := ei.Egg_CURIOSITY
	homeFarm := &ei.Backup_Simulation{
		FarmType:      ei.FarmType_HOME.Enum(),
		EggType:       &curiosityEgg,
		NumChickens:   protoUint64(15_000_000_000), // > baseHabCap so gusset required
		HabPopulation: []uint64{3_750_000_000, 3_750_000_000, 3_750_000_000, 3_750_000_000},
		Habs:          []uint32{18, 18, 18, 18}, // Chicken Universe
	}

	// 4 artifacts in inventory
	totem := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_LUNAR_TOTEM.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(), // T4
			Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
		},
	}
	necklace := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_DEMETERS_NECKLACE.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(), // T4
			Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
		},
	}
	ankh := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_TUNGSTEN_ANKH.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(), // T4
			Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
		},
	}
	gusset := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_ORNATE_GUSSET.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(), // T4
			Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
		},
	}

	inventory := []*ei.ArtifactInventoryItem{
		{ItemId: protoUint64(1), Artifact: totem, Quantity: protoFloat64(1)},
		{ItemId: protoUint64(2), Artifact: necklace, Quantity: protoFloat64(1)},
		{ItemId: protoUint64(3), Artifact: ankh, Quantity: protoFloat64(1)},
		{ItemId: protoUint64(4), Artifact: gusset, Quantity: protoFloat64(1)},
		// 12 T4 Lunar stones
		{
			ItemId: protoUint64(10),
			Artifact: &ei.CompleteArtifact{
				Spec: &ei.ArtifactSpec{
					Name:  ei.ArtifactSpec_LUNAR_STONE.Enum(),
					Level: ei.ArtifactSpec_NORMAL.Enum(), // T4 (level 2)
				},
			},
			Quantity: protoFloat64(12),
		},
	}

	backup := &ei.Backup{
		Game:  game,
		Farms: []*ei.Backup_Simulation{homeFarm},
		ArtifactsDb: &ei.ArtifactsDB{
			VirtueAfxDb: &ei.ArtifactsDB_VirtueDB{
				InventoryItems: inventory,
			},
		},
	}

	result := OptimizeEarnings(backup, homeFarm)
	if result.TotalMultiplier < 85040 || result.TotalMultiplier > 85042 {
		t.Errorf("expected TotalMultiplier ~85041, got %f", result.TotalMultiplier)
	}

	if !result.GussetRequired {
		t.Errorf("expected GussetRequired = true")
	}

	if len(result.OptimalArtifacts) != 4 {
		t.Fatalf("expected 4 optimal artifacts, got %d", len(result.OptimalArtifacts))
	}

	if len(result.OptimalStones) != 1 {
		t.Fatalf("expected 1 stone group, got %d", len(result.OptimalStones))
	}
	if result.OptimalStones[0].Count != 12 {
		t.Errorf("expected 12 lunar stones, got %d", result.OptimalStones[0].Count)
	}

	// Verify formatted multiplier
	multStr := formatMultiplier(result.TotalMultiplier)
	if multStr != "85,041" {
		t.Errorf("expected formatted multiplier '85,041', got %q", multStr)
	}
}

func TestOptimizeDeliverySetup(t *testing.T) {
	game := &ei.Backup_Game{
		PermitLevel: protoUint32(1), // Pro permit
	}

	curiosityEgg := ei.Egg_CURIOSITY
	homeFarm := &ei.Backup_Simulation{
		FarmType:      ei.FarmType_HOME.Enum(),
		EggType:       &curiosityEgg,
		NumChickens:   protoUint64(14_000_000_000),
		HabPopulation: []uint64{3_500_000_000, 3_500_000_000, 3_500_000_000, 3_500_000_000},
		Habs:          []uint32{18, 18, 18, 18},
		Vehicles:      []uint32{11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11},
		TrainLength:   []uint32{10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10},
	}

	metronome := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_QUANTUM_METRONOME.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(), // T4
			Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
		},
	}
	compass := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_INTERSTELLAR_COMPASS.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(), // T4
			Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
		},
	}
	gusset := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_ORNATE_GUSSET.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(), // T4
			Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
		},
	}
	ankh := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_TUNGSTEN_ANKH.Enum(),
			Level:  ei.ArtifactSpec_NORMAL.Enum(), // T3
			Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
		},
	}

	inventory := []*ei.ArtifactInventoryItem{
		{ItemId: protoUint64(1), Artifact: metronome, Quantity: protoFloat64(1)},
		{ItemId: protoUint64(2), Artifact: compass, Quantity: protoFloat64(1)},
		{ItemId: protoUint64(3), Artifact: gusset, Quantity: protoFloat64(1)},
		{ItemId: protoUint64(4), Artifact: ankh, Quantity: protoFloat64(1)},
		// 8 Tachyon T4 stones
		{
			ItemId: protoUint64(20),
			Artifact: &ei.CompleteArtifact{
				Spec: &ei.ArtifactSpec{
					Name:  ei.ArtifactSpec_TACHYON_STONE.Enum(),
					Level: ei.ArtifactSpec_NORMAL.Enum(), // T4
				},
			},
			Quantity: protoFloat64(8),
		},
		// 8 Quantum T4 stones
		{
			ItemId: protoUint64(21),
			Artifact: &ei.CompleteArtifact{
				Spec: &ei.ArtifactSpec{
					Name:  ei.ArtifactSpec_QUANTUM_STONE.Enum(),
					Level: ei.ArtifactSpec_NORMAL.Enum(), // T4
				},
			},
			Quantity: protoFloat64(8),
		},
	}

	backup := &ei.Backup{
		Game:  game,
		Farms: []*ei.Backup_Simulation{homeFarm},
		ArtifactsDb: &ei.ArtifactsDB{
			VirtueAfxDb: &ei.ArtifactsDB_VirtueDB{
				InventoryItems: inventory,
			},
		},
	}

	result := OptimizeDelivery(backup, homeFarm)
	if result.DeliveryRate <= 0 {
		t.Errorf("expected positive delivery rate, got %f", result.DeliveryRate)
	}

	if len(result.OptimalArtifacts) != 4 {
		t.Fatalf("expected 4 optimal artifacts, got %d", len(result.OptimalArtifacts))
	}

	// Check that Metronome, Compass, and Gusset are chosen
	hasMetr, hasComp, hasGusset := false, false, false
	for _, a := range result.OptimalArtifacts {
		switch a.GetSpec().GetName() {
		case ei.ArtifactSpec_QUANTUM_METRONOME:
			hasMetr = true
		case ei.ArtifactSpec_INTERSTELLAR_COMPASS:
			hasComp = true
		case ei.ArtifactSpec_ORNATE_GUSSET:
			hasGusset = true
		}
	}
	if !hasMetr || !hasComp || !hasGusset {
		t.Errorf("expected Metronome, Compass, and Gusset in optimal loadout: metr=%v comp=%v gusset=%v", hasMetr, hasComp, hasGusset)
	}

	// Verify that stones were split
	totalStones := 0
	for _, sg := range result.OptimalStones {
		totalStones += sg.Count
	}
	// Metronome (3) + Compass (2) + Gusset (3) + Ankh (3) = 11 slots
	if totalStones != 11 {
		t.Errorf("expected 11 total stones, got %d", totalStones)
	}
}

func TestFormatFarmTypeHeader(t *testing.T) {
	curiosity := ei.Egg_CURIOSITY
	farmVirtue := &ei.Backup_Simulation{
		EggType: &curiosity,
	}
	label, _ := formatFarmTypeHeader(farmVirtue)
	if label != "Virtue (Curiosity)" {
		t.Errorf("expected 'Virtue (Curiosity)', got %q", label)
	}

	universe := ei.Egg_UNIVERSE
	farmHome := &ei.Backup_Simulation{
		EggType: &universe,
	}
	labelHome, _ := formatFarmTypeHeader(farmHome)
	if labelHome != "Home (Universe)" {
		t.Errorf("expected 'Home (Universe)', got %q", labelHome)
	}

	enlightenment := ei.Egg_ENLIGHTENMENT
	farmEnlight := &ei.Backup_Simulation{
		EggType: &enlightenment,
	}
	labelEnlight, _ := formatFarmTypeHeader(farmEnlight)
	if labelEnlight != "Enlightenment" {
		t.Errorf("expected 'Enlightenment', got %q", labelEnlight)
	}
}

func TestFarmHelpText(t *testing.T) {
	help := farmHelpText()
	if !strings.Contains(help, "/farm delivery") {
		t.Errorf("expected help text to mention /farm delivery")
	}
	if !strings.Contains(help, "/farm earnings") {
		t.Errorf("expected help text to mention /farm earnings")
	}
	if !strings.Contains(help, "alt") {
		t.Errorf("expected help text to mention alt option")
	}
}

func TestPrintEarningsSetupVirtue(t *testing.T) {
	curiosityEgg := ei.Egg_CURIOSITY
	farm := &ei.Backup_Simulation{
		FarmType:      ei.FarmType_HOME.Enum(),
		EggType:       &curiosityEgg,
		NumChickens:   protoUint64(10_000_000),
		HabPopulation: []uint64{2_500_000, 2_500_000, 2_500_000, 2_500_000},
		Habs:          []uint32{18, 18, 18, 18},
	}
	totem := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_LUNAR_TOTEM.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(),
			Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
		},
	}
	necklace := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_DEMETERS_NECKLACE.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(),
			Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
		},
	}
	inventory := []*ei.ArtifactInventoryItem{
		{ItemId: protoUint64(1), Artifact: totem, Quantity: protoFloat64(1)},
		{ItemId: protoUint64(2), Artifact: necklace, Quantity: protoFloat64(1)},
		{
			ItemId: protoUint64(3),
			Artifact: &ei.CompleteArtifact{
				Spec: &ei.ArtifactSpec{
					Name:  ei.ArtifactSpec_LUNAR_STONE.Enum(),
					Level: ei.ArtifactSpec_NORMAL.Enum(),
				},
			},
			Quantity: protoFloat64(6),
		},
	}
	backup := &ei.Backup{
		Game:     &ei.Backup_Game{PermitLevel: protoUint32(1)},
		UserName: protoStr("TestFarmer"),
		Farms:    []*ei.Backup_Simulation{farm},
		ArtifactsDb: &ei.ArtifactsDB{
			VirtueAfxDb: &ei.ArtifactsDB_VirtueDB{
				InventoryItems: inventory,
			},
		},
	}

	comps := printEarningsSetup("100000000000000001", backup, farm, "", "")
	if len(comps) < 3 {
		t.Fatalf("expected at least 3 components, got %d", len(comps))
	}

	sec, ok := comps[0].(dc.Section)
	if !ok || len(sec.Components) == 0 {
		t.Fatalf("expected first component to be dc.Section, got %T", comps[0])
	}
	if !strings.Contains(sec.Components[0].Content, "# Optimal Earnings Setup") {
		t.Errorf("header missing '# Optimal Earnings Setup': %s", sec.Components[0].Content)
	}

	artDisp, ok := comps[2].(dc.TextDisplay)
	if !ok {
		t.Fatalf("expected third component to be dc.TextDisplay, got %T", comps[2])
	}
	body := artDisp.Content

	if !strings.Contains(body, "Max CTE") {
		t.Errorf("body missing 'Max CTE': %s", body)
	}
	if !strings.Contains(body, "Currently Equipped:") {
		t.Errorf("body missing 'Currently Equipped:': %s", body)
	}
	if !strings.Contains(body, "Optimal Setup:") {
		t.Errorf("body missing 'Optimal Setup:': %s", body)
	}
	if !strings.Contains(body, "Formula:") {
		t.Errorf("body missing 'Formula:': %s", body)
	}
	// Should not have wonky style numbered artifact list
	if strings.Contains(body, "1. ") {
		t.Errorf("body should not contain wonky numbered artifact list: %s", body)
	}
	if strings.Contains(body, "➜") {
		t.Errorf("body should not contain '+... CTE' transition line: %s", body)
	}
}

func TestPrintDeliverySetup(t *testing.T) {
	curiosityEgg := ei.Egg_CURIOSITY
	farm := &ei.Backup_Simulation{
		FarmType:      ei.FarmType_HOME.Enum(),
		EggType:       &curiosityEgg,
		NumChickens:   protoUint64(10_000_000),
		HabPopulation: []uint64{2_500_000, 2_500_000, 2_500_000, 2_500_000},
		Habs:          []uint32{18, 18, 18, 18},
		Vehicles:      []uint32{11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11},
		TrainLength:   []uint32{10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10},
	}
	metronome := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_QUANTUM_METRONOME.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(),
			Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
		},
	}
	inventory := []*ei.ArtifactInventoryItem{
		{ItemId: protoUint64(1), Artifact: metronome, Quantity: protoFloat64(1)},
	}
	backup := &ei.Backup{
		Game:     &ei.Backup_Game{PermitLevel: protoUint32(1)},
		UserName: protoStr("TestFarmer"),
		Farms:    []*ei.Backup_Simulation{farm},
		ArtifactsDb: &ei.ArtifactsDB{
			VirtueAfxDb: &ei.ArtifactsDB_VirtueDB{
				InventoryItems: inventory,
			},
		},
	}

	comps := printDeliverySetup("100000000000000001", backup, farm, "", "")
	if len(comps) < 3 {
		t.Fatalf("expected at least 3 components, got %d", len(comps))
	}

	sec, ok := comps[0].(dc.Section)
	if !ok || len(sec.Components) == 0 {
		t.Fatalf("expected first component to be dc.Section, got %T", comps[0])
	}
	if !strings.Contains(sec.Components[0].Content, "# Optimal Production Setup") {
		t.Errorf("header missing '# Optimal Production Setup': %s", sec.Components[0].Content)
	}

	artDisp, ok := comps[2].(dc.TextDisplay)
	if !ok {
		t.Fatalf("expected third component to be dc.TextDisplay, got %T", comps[2])
	}
	body := artDisp.Content

	if !strings.Contains(body, "Currently Equipped:") {
		t.Errorf("body missing 'Currently Equipped:': %s", body)
	}
	if !strings.Contains(body, "Optimal Setup:") {
		t.Errorf("body missing 'Optimal Setup:': %s", body)
	}
	if !strings.Contains(body, "Formulas:") {
		t.Errorf("body missing 'Formulas:': %s", body)
	}
	if !strings.Contains(body, "Delivery:") {
		t.Errorf("body missing 'Delivery:': %s", body)
	}
	if strings.Contains(body, "1. ") {
		t.Errorf("body should not contain wonky numbered artifact list: %s", body)
	}
}

func TestPrintDeliveryAndEarningsDetailed(t *testing.T) {
	setupTestCustomEggs()
	if ei.EmoteMap == nil {
		ei.EmoteMap = make(map[string]ei.Emotes)
	}
	ei.EmoteMap["egg_carbonfiber"] = ei.Emotes{Name: "egg_carbonfiber", ID: "1001"}
	ei.EmoteMap["egg_pumpkin"] = ei.Emotes{Name: "egg_pumpkin", ID: "1002"}
	ei.EmoteMap["egg_pegg"] = ei.Emotes{Name: "egg_pegg", ID: "1003"}
	ei.EmoteMap["collegg"] = ei.Emotes{Name: "collegg", ID: "9999"}

	testUser := "100000000000000001"
	farmerstate.SetMiscSettingString(testUser, "collegg", "carbon-fiber,pumpkin,pegg,chocolate,firework")

	curiosityEgg := ei.Egg_CURIOSITY
	farm := &ei.Backup_Simulation{
		FarmType:      ei.FarmType_HOME.Enum(),
		EggType:       &curiosityEgg,
		NumChickens:   protoUint64(10_000_000),
		HabPopulation: []uint64{2_500_000, 2_500_000, 2_500_000, 2_500_000},
		Habs:          []uint32{18, 18, 18, 18},
		Vehicles:      []uint32{11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11},
		TrainLength:   []uint32{10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10},
	}
	metronome := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_QUANTUM_METRONOME.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(),
			Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
		},
		Stones: []*ei.ArtifactSpec{
			{Name: ei.ArtifactSpec_TACHYON_STONE.Enum(), Level: ei.ArtifactSpec_NORMAL.Enum()},
		},
	}
	totem := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_LUNAR_TOTEM.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(),
			Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
		},
		Stones: []*ei.ArtifactSpec{
			{Name: ei.ArtifactSpec_LUNAR_STONE.Enum(), Level: ei.ArtifactSpec_NORMAL.Enum()},
		},
	}

	inventory := []*ei.ArtifactInventoryItem{
		{ItemId: protoUint64(1), Artifact: metronome, Quantity: protoFloat64(1)},
		{ItemId: protoUint64(2), Artifact: totem, Quantity: protoFloat64(1)},
	}

	backup := &ei.Backup{
		Game:     &ei.Backup_Game{PermitLevel: protoUint32(1)},
		UserName: protoStr("TestFarmer"),
		Farms:    []*ei.Backup_Simulation{farm},
		ArtifactsDb: &ei.ArtifactsDB{
			VirtueAfxDb: &ei.ArtifactsDB_VirtueDB{
				InventoryItems: inventory,
				ActiveArtifacts: &ei.ArtifactsDB_ActiveArtifactSet{
					Slots: []*ei.ActiveArtifactSlot{
						{ItemId: protoUint64(1), Occupied: protoBool(true)},
						{ItemId: protoUint64(2), Occupied: protoBool(true)},
					},
				},
			},
		},
	}

	// Test Delivery setup (already optimal)
	delComps := printDeliverySetup(testUser, backup, farm, "", "")
	delBody := delComps[2].(dc.TextDisplay).Content
	if !strings.Contains(delBody, "Currently Equipped:") {
		t.Errorf("expected Currently Equipped in delivery: %s", delBody)
	}
	if !strings.Contains(delBody, "Your current setup is already optimal!") {
		t.Errorf("expected already optimal in delivery: %s", delBody)
	}
	if !strings.Contains(delBody, "Formulas:") {
		t.Errorf("expected Formulas: in delivery: %s", delBody)
	}
	// Check that Laying and Habs are coupled with × in Formula
	if !strings.Contains(delBody, "🐔 Laying:") || !strings.Contains(delBody, "× 🏠 Habs:") {
		t.Errorf("expected Laying and Habs coupled with '×' in delivery formula: %s", delBody)
	}
	if !strings.Contains(delBody, "🚚 Shipping:") {
		t.Errorf("expected '🚚 Shipping:' in delivery formula: %s", delBody)
	}
	// Verify that colleggtibles in the formula display individual egg emojis rather than generic :collegg:
	if !strings.Contains(delBody, "<:egg_carbonfiber:1001> 1.05x") {
		t.Errorf("expected <:egg_carbonfiber:1001> 1.05x in shipping formula: %s", delBody)
	}
	if !strings.Contains(delBody, "<:egg_pumpkin:1002> 1.05x") {
		t.Errorf("expected <:egg_pumpkin:1002> 1.05x in laying formula: %s", delBody)
	}
	if !strings.Contains(delBody, "<:egg_pegg:1003> 1.05x") {
		t.Errorf("expected <:egg_pegg:1003> 1.05x in hab formula: %s", delBody)
	}
	formulaSection := delBody[strings.Index(delBody, "Formulas:"):strings.Index(delBody, "📦 Delivery:")]
	if strings.Contains(formulaSection, "<:collegg:9999>") {
		t.Errorf("expected no generic collegg emoji in delivery formula: %s", formulaSection)
	}
	if !strings.Contains(delBody, "Colleggtibles Contributing to Delivery:") {
		t.Errorf("expected Colleggtibles Contributing to Delivery: %s", delBody)
	}

	// Test Earnings setup (already optimal)
	earnComps := printEarningsSetup(testUser, backup, farm, "", "")
	earnBody := earnComps[2].(dc.TextDisplay).Content
	if !strings.Contains(earnBody, "Currently Equipped:") {
		t.Errorf("expected Currently Equipped in earnings: %s", earnBody)
	}
	if !strings.Contains(earnBody, "Your current setup is already optimal!") {
		t.Errorf("expected already optimal in earnings: %s", earnBody)
	}
	if !strings.Contains(earnBody, "Formula:") {
		t.Errorf("expected Formula in earnings: %s", earnBody)
	}
	if !strings.Contains(earnBody, "Colleggtibles Contributing to Earnings:") {
		t.Errorf("expected Colleggtibles Contributing to Earnings: %s", earnBody)
	}
}

func TestOptimizeEarningsWithActiveEvent(t *testing.T) {
	testUser := "100000000000000001"
	tachyonEgg := ei.Egg_TACHYON
	farm := &ei.Backup_Simulation{
		FarmType:     ei.FarmType_HOME.Enum(),
		EggType:      &tachyonEgg,
		NumChickens:  protoUint64(1000000),
		Habs:         []uint32{18, 18, 18, 18},
		LastStepTime: protoFloat64(1790477235),
	}

	t4Totem := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_LUNAR_TOTEM.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(),
			Rarity: ei.ArtifactSpec_EPIC.Enum(),
		},
	}
	t4Necklace := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_DEMETERS_NECKLACE.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(),
			Rarity: ei.ArtifactSpec_RARE.Enum(),
		},
	}

	backup := &ei.Backup{
		UserId:   protoStr("EI1234567890123456"),
		UserName: protoStr("EarningsTester"),
		Farms:    []*ei.Backup_Simulation{farm},
		Game:     &ei.Backup_Game{PermitLevel: protoUint32(1)},
		ArtifactsDb: &ei.ArtifactsDB{
			InventoryItems: []*ei.ArtifactInventoryItem{
				{ItemId: protoUint64(1), Artifact: t4Totem, Quantity: protoFloat64(1)},
				{ItemId: protoUint64(2), Artifact: t4Necklace, Quantity: protoFloat64(1)},
			},
			ActiveArtifactSets: []*ei.ArtifactsDB_ActiveArtifactSet{
				{
					Slots: []*ei.ActiveArtifactSlot{
						{Occupied: protoBool(true), ItemId: protoUint64(1)},
					},
				},
			},
		},
	}

	// 1. Without active event
	optNoEvent := OptimizeEarnings(backup, farm)
	if optNoEvent.HasActiveEvent {
		t.Errorf("expected no active event when none is configured")
	}
	compsNoEvent := printEarningsSetup(testUser, backup, farm, "", "")
	earnBodyNoEvent := compsNoEvent[2].(dc.TextDisplay).Content
	if strings.Contains(earnBodyNoEvent, "Active Periodicals Event") {
		t.Errorf("expected no active event banner without event")
	}

	// 2. With active 3x earnings event
	now := time.Now().UTC()
	ei.EventMutex.Lock()
	origEggIncEvents := ei.EggIncEvents
	ei.EggIncEvents = []ei.EggEvent{
		{
			ID:             "active-earnings-event",
			EventType:      "earnings-boost",
			Message:        "3x EARNINGS!",
			Multiplier:     3,
			StartTimestamp: float64(now.Add(-1 * time.Hour).Unix()),
			EndTimestamp:   float64(now.Add(5 * time.Hour).Unix()),
			StartTime:      now.Add(-1 * time.Hour),
			EndTime:        now.Add(5 * time.Hour),
		},
	}
	ei.EventMutex.Unlock()
	defer func() {
		ei.EventMutex.Lock()
		ei.EggIncEvents = origEggIncEvents
		ei.EventMutex.Unlock()
	}()

	optWithEvent := OptimizeEarnings(backup, farm)
	if !optWithEvent.HasActiveEvent {
		t.Errorf("expected HasActiveEvent to be true with active earnings event")
	}
	if optWithEvent.EventMultiplier != 3.0 {
		t.Errorf("expected EventMultiplier 3.0, got %f", optWithEvent.EventMultiplier)
	}

	compsWithEvent := printEarningsSetup(testUser, backup, farm, "", "")
	earnBodyWithEvent := compsWithEvent[2].(dc.TextDisplay).Content
	if !strings.Contains(earnBodyWithEvent, "Active Periodicals Event") {
		t.Errorf("expected active periodicals event banner in earnings output")
	}
	if !strings.Contains(earnBodyWithEvent, "3x EARNINGS!") {
		t.Errorf("expected event message '3x EARNINGS!' in earnings output")
	}
	if !strings.Contains(earnBodyWithEvent, "amplifies earnings by **3x**") {
		t.Errorf("expected 'amplifies earnings by **3x**' in earnings output")
	}
	if !strings.Contains(earnBodyWithEvent, "with 3x EARNINGS!") {
		t.Errorf("expected 'with 3x EARNINGS!' on multiplier line")
	}
}

func TestPrintEarningsSetupWithColleggtibles(t *testing.T) {
	setupTestCustomEggs()
	if ei.EmoteMap == nil {
		ei.EmoteMap = make(map[string]ei.Emotes)
	}
	ei.EmoteMap["egg_chocolate"] = ei.Emotes{Name: "egg_chocolate", ID: "1004"}
	ei.EmoteMap["egg_firework"] = ei.Emotes{Name: "egg_firework", ID: "1005"}
	ei.EmoteMap["egg_wood"] = ei.Emotes{Name: "egg_wood", ID: "1006"}
	ei.EmoteMap["totem_T4L"] = ei.Emotes{Name: "totem_T4L", ID: "2001"}
	ei.EmoteMap["neck_T4L"] = ei.Emotes{Name: "neck_T4L", ID: "2002"}

	testUser := "100000000000000002"
	farmerstate.SetMiscSettingString(testUser, "collegg", "chocolate,wood,firework")

	tachyonEgg := ei.Egg_TACHYON
	farm := &ei.Backup_Simulation{
		FarmType:     ei.FarmType_HOME.Enum(),
		EggType:      &tachyonEgg,
		NumChickens:  protoUint64(1000000),
		Habs:         []uint32{18, 18, 18, 18},
		LastStepTime: protoFloat64(1790477235),
	}

	t4Totem := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_LUNAR_TOTEM.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(),
			Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
		},
	}
	t4Necklace := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_DEMETERS_NECKLACE.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(),
			Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
		},
	}

	backup := &ei.Backup{
		UserId:   protoStr("EI1234567890123456"),
		UserName: protoStr("ColleggEarningsTester"),
		Farms:    []*ei.Backup_Simulation{farm},
		Game:     &ei.Backup_Game{PermitLevel: protoUint32(1)},
		ArtifactsDb: &ei.ArtifactsDB{
			InventoryItems: []*ei.ArtifactInventoryItem{
				{ItemId: protoUint64(1), Artifact: t4Totem, Quantity: protoFloat64(1)},
				{ItemId: protoUint64(2), Artifact: t4Necklace, Quantity: protoFloat64(1)},
			},
			ActiveArtifactSets: []*ei.ArtifactsDB_ActiveArtifactSet{
				{
					Slots: []*ei.ActiveArtifactSlot{
						{Occupied: protoBool(true), ItemId: protoUint64(1)},
						{Occupied: protoBool(true), ItemId: protoUint64(2)},
					},
				},
			},
		},
	}

	comps := printEarningsSetup(testUser, backup, farm, "", "")
	earnBody := comps[2].(dc.TextDisplay).Content

	// Check that the formula contains the individual colleggtible egg emojis and their multiplier
	if !strings.Contains(earnBody, "<:egg_firework:1005> 1.05x") {
		t.Errorf("expected <:egg_firework:1005> 1.05x in earnings formula: %s", earnBody)
	}
	if !strings.Contains(earnBody, "<:egg_chocolate:1004> 3.00x") {
		t.Errorf("expected <:egg_chocolate:1004> 3.00x in earnings formula: %s", earnBody)
	}
	if !strings.Contains(earnBody, "<:egg_wood:1006> 2.00x") {
		t.Errorf("expected <:egg_wood:1006> 2.00x in earnings formula: %s", earnBody)
	}

	// Verify that the total multiplier reflects colleggtible buffs
	// Base: totem 200x * necklace 3.0x = 600x. Collegg: 1.05 * 3.0 * 2.0 = 6.30x.
	// 600 * 6.30 = 3,780x
	if !strings.Contains(earnBody, "3,780x") {
		t.Errorf("expected total multiplier '3,780x' reflecting colleggtible boost: %s", earnBody)
	}
}

func setupTestCustomEggs() {
	if ei.CustomEggMap == nil {
		ei.CustomEggMap = make(map[string]*ei.EggIncCustomEgg)
	}
	ei.CustomEggMap["carbon-fiber"] = &ei.EggIncCustomEgg{
		ID:                   "carbon-fiber",
		Name:                 "Carbon Fiber",
		Dimension:            ei.GameModifier_SHIPPING_CAPACITY,
		DimensionValue:       []float64{1.01, 1.02, 1.03, 1.05},
		DimensionValueString: []string{"+1%", "+2%", "+3%", "+5%"},
	}
	ei.CustomEggMap["pumpkin"] = &ei.EggIncCustomEgg{
		ID:                   "pumpkin",
		Name:                 "Pumpkin",
		Dimension:            ei.GameModifier_EGG_LAYING_RATE,
		DimensionValue:       []float64{1.01, 1.02, 1.03, 1.05},
		DimensionValueString: []string{"+1%", "+2%", "+3%", "+5%"},
	}
	ei.CustomEggMap["pegg"] = &ei.EggIncCustomEgg{
		ID:                   "pegg",
		Name:                 "Pegg",
		Dimension:            ei.GameModifier_HAB_CAPACITY,
		DimensionValue:       []float64{1.01, 1.02, 1.03, 1.05},
		DimensionValueString: []string{"+1%", "+2%", "+3%", "+5%"},
	}
	ei.CustomEggMap["chocolate"] = &ei.EggIncCustomEgg{
		ID:                   "chocolate",
		Name:                 "Chocolate",
		Dimension:            ei.GameModifier_AWAY_EARNINGS,
		DimensionValue:       []float64{1.25, 1.5, 2.0, 3.0},
		DimensionValueString: []string{"+25%", "+50%", "+100%", "+200%"},
	}
	ei.CustomEggMap["wood"] = &ei.EggIncCustomEgg{
		ID:                   "wood",
		Name:                 "Wood",
		Dimension:            ei.GameModifier_AWAY_EARNINGS,
		DimensionValue:       []float64{1.1, 1.25, 1.5, 2.0},
		DimensionValueString: []string{"+10%", "+25%", "+50%", "2x"},
	}
	ei.CustomEggMap["firework"] = &ei.EggIncCustomEgg{
		ID:                   "firework",
		Name:                 "Firework",
		Dimension:            ei.GameModifier_EARNINGS,
		DimensionValue:       []float64{1.01, 1.02, 1.03, 1.05},
		DimensionValueString: []string{"+1%", "+2%", "+3%", "+5%"},
	}
}

func protoBool(v bool) *bool {
	return &v
}

func protoStr(v string) *string {
	return &v
}

func protoUint64(v uint64) *uint64 {
	return &v
}

func protoUint32(v uint32) *uint32 {
	return &v
}

func protoFloat64(v float64) *float64 {
	return &v
}

func TestSlashFarmCommandSubcommands(t *testing.T) {
	cmd := GetSlashFarmCommand("farm")
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}
	if len(cmd.Options) != 3 {
		t.Fatalf("expected 3 subcommands, got %d", len(cmd.Options))
	}

	names := make(map[string]bool)
	for _, opt := range cmd.Options {
		sub, ok := opt.(dc.SubCommand)
		if !ok {
			t.Errorf("expected option to be dc.SubCommand, got %T", opt)
			continue
		}
		names[sub.Name] = true
		if len(sub.Options) != 2 {
			t.Errorf("expected 2 options for subcommand %s, got %d", sub.Name, len(sub.Options))
		}
	}

	expectedSubcmds := []string{"delivery", "earnings", "drone-farming"}
	for _, expected := range expectedSubcmds {
		if !names[expected] {
			t.Errorf("missing subcommand %s", expected)
		}
	}
}

func TestOptimizeDroneFarming(t *testing.T) {
	// Create a synthetic backup with T4L Brooch, T4L Medallion, T4L Beak and T4 Clarity stones
	t4lBrooch := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_AURELIAN_BROOCH.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(),
			Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
		},
	}
	t4lMedallion := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_NEODYMIUM_MEDALLION.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(),
			Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
		},
	}
	t4lBeak := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_BEAK_OF_MIDAS.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(),
			Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
		},
	}

	clarityStoneSpec := &ei.ArtifactSpec{
		Name:  ei.ArtifactSpec_CLARITY_STONE.Enum(),
		Level: ei.ArtifactSpec_NORMAL.Enum(), // T4 Clarity (level 2)
	}

	backup := &ei.Backup{
		UserId:   protoStr("EI1234567890123456"),
		UserName: protoStr("TestDroneFarmer"),
		Game: &ei.Backup_Game{
			PermitLevel: protoUint32(1), // Pro permit
		},
		ArtifactsDb: &ei.ArtifactsDB{
			InventoryItems: []*ei.ArtifactInventoryItem{
				{ItemId: protoUint64(1), Artifact: t4lBrooch, Quantity: protoFloat64(1)},
				{ItemId: protoUint64(2), Artifact: t4lMedallion, Quantity: protoFloat64(1)},
				{ItemId: protoUint64(3), Artifact: t4lBeak, Quantity: protoFloat64(1)},
				{
					ItemId: protoUint64(4),
					Artifact: &ei.CompleteArtifact{
						Spec: clarityStoneSpec,
					},
					Quantity: protoFloat64(10), // 10x T4 Clarity stones
				},
			},
		},
	}

	enlightenmentFarm := &ei.Backup_Simulation{
		EggType:      ei.Egg_ENLIGHTENMENT.Enum(),
		LastStepTime: protoFloat64(1700000000),
	}

	opt := OptimizeDroneFarming(backup, enlightenmentFarm)
	if opt == nil {
		t.Fatal("expected non-nil DroneFarmingResult")
	}

	// 3x T4 clarity on T4L Brooch = 1.0 + 2.0*3.0 = 7.0x
	if opt.BroochMultiplier != 7.0 {
		t.Errorf("expected BroochMultiplier 7.0, got %f", opt.BroochMultiplier)
	}
	// 3x T4 clarity on T4L Medallion = 1.0 + 1.3*3.0 = 4.9x
	if math.Abs(opt.MedallionMultiplier-4.9) > 1e-4 {
		t.Errorf("expected MedallionMultiplier 4.9, got %f", opt.MedallionMultiplier)
	}
	// T4L Beak with >=1 clarity = 100% GE chance (1.0)
	if opt.GEDropChance != 1.0 {
		t.Errorf("expected GEDropChance 1.0, got %f", opt.GEDropChance)
	}
	// Total = 7.0 * 4.9 * (1.0 / 0.30) = 114.333...
	expectedTotal := 7.0 * 4.9 * (1.0 / 0.30)
	if math.Abs(opt.TotalMultiplier-expectedTotal) > 0.1 {
		t.Errorf("expected TotalMultiplier ~%.1f, got %f", expectedTotal, opt.TotalMultiplier)
	}

	// Test printDroneFarmingSetup
	comps := printDroneFarmingSetup("100000000000000001", backup, enlightenmentFarm, "", "")
	if len(comps) == 0 {
		t.Fatal("expected components from printDroneFarmingSetup")
	}

	// Test with loadout already equipped
	equippedBrooch := &ei.CompleteArtifact{
		Spec:   t4lBrooch.GetSpec(),
		Stones: []*ei.ArtifactSpec{clarityStoneSpec, clarityStoneSpec, clarityStoneSpec},
	}
	equippedMedallion := &ei.CompleteArtifact{
		Spec:   t4lMedallion.GetSpec(),
		Stones: []*ei.ArtifactSpec{clarityStoneSpec, clarityStoneSpec, clarityStoneSpec},
	}
	equippedBeak := &ei.CompleteArtifact{
		Spec:   t4lBeak.GetSpec(),
		Stones: []*ei.ArtifactSpec{clarityStoneSpec},
	}
	backup.ArtifactsDb.InventoryItems = []*ei.ArtifactInventoryItem{
		{ItemId: protoUint64(1), Artifact: equippedBrooch, Quantity: protoFloat64(1)},
		{ItemId: protoUint64(2), Artifact: equippedMedallion, Quantity: protoFloat64(1)},
		{ItemId: protoUint64(3), Artifact: equippedBeak, Quantity: protoFloat64(1)},
	}
	backup.ArtifactsDb.ActiveArtifactSets = []*ei.ArtifactsDB_ActiveArtifactSet{
		{
			Slots: []*ei.ActiveArtifactSlot{
				{Occupied: protoBool(true), ItemId: protoUint64(1)},
				{Occupied: protoBool(true), ItemId: protoUint64(2)},
				{Occupied: protoBool(true), ItemId: protoUint64(3)},
			},
		},
	}

	optEquipped := OptimizeDroneFarming(backup, enlightenmentFarm)
	if !optEquipped.IsAlreadyOptimal {
		t.Errorf("expected setup to be marked already optimal")
	}

	compsEquipped := printDroneFarmingSetup("100000000000000001", backup, enlightenmentFarm, "", "")
	foundOptimalMsg := false
	for _, c := range compsEquipped {
		if td, ok := c.(dc.TextDisplay); ok {
			if strings.Contains(td.Content, "Your current setup is already optimal!") {
				foundOptimalMsg = true
			}
		}
	}
	if !foundOptimalMsg {
		t.Errorf("expected 'Your current setup is already optimal!' in output")
	}

	// Test when user is active on a non-Enlightenment home farm (e.g. Tachyon)
	tachyonFarm := &ei.Backup_Simulation{
		EggType:      ei.Egg_TACHYON.Enum(),
		LastStepTime: protoFloat64(1700000000),
	}
	optTachyon := OptimizeDroneFarming(backup, tachyonFarm)
	if !optTachyon.IsAlreadyOptimal {
		t.Errorf("expected setup on non-Enlightenment active home farm to be recognized as optimal drone set")
	}
	if optTachyon.CurrentMultiplier != 1.0 {
		t.Errorf("expected current multiplier on non-Enlightenment farm to be 1.0x, got %f", optTachyon.CurrentMultiplier)
	}
	if optTachyon.CurrentMedallionMult != 1.0 {
		t.Errorf("expected equipped medallion on non-Enlightenment farm to be 1.0x, got %f", optTachyon.CurrentMedallionMult)
	}
	if optTachyon.CurrentBroochMult != 1.0 {
		t.Errorf("expected equipped brooch on non-Enlightenment farm to be 1.0x, got %f", optTachyon.CurrentBroochMult)
	}

	compsTachyon := printDroneFarmingSetup("100000000000000001", backup, tachyonFarm, "", "")
	foundTachyonOptimal := false
	foundSwitchEnlightenmentNotice := false
	foundEnlightenmentBasisNotice := false
	foundTachyonHeader := false
	foundTachyonMultiplierFormat := false
	for _, c := range compsTachyon {
		if sec, ok := c.(dc.Section); ok {
			for _, item := range sec.Components {
				if strings.Contains(item.Content, "Farm Type**: <::> Enlightenment *(Current:") {
					foundTachyonHeader = true
				}
			}
		}
		if td, ok := c.(dc.TextDisplay); ok {
			if strings.Contains(td.Content, "Your current setup is already optimal!") {
				foundTachyonOptimal = true
			}
			if strings.Contains(td.Content, "Optimal <::> GE Multiplier") && strings.Contains(td.Content, "(<::> 1.0x)") {
				foundTachyonMultiplierFormat = true
			}
			if strings.Contains(td.Content, "Switch to Enlightenment Egg") {
				foundSwitchEnlightenmentNotice = true
			}
			if strings.Contains(td.Content, "based on being on the **Enlightenment Farm**") {
				foundEnlightenmentBasisNotice = true
			}
		}
	}
	if !foundTachyonHeader {
		t.Errorf("expected header to show Enlightenment with current farm noted")
	}
	if !foundTachyonOptimal {
		t.Errorf("expected setup on Tachyon farm with drone set to say 'Your current setup is already optimal!'")
	}
	if !foundTachyonMultiplierFormat {
		t.Errorf("expected 'Optimal <::> GE Multiplier' with current egg emote in parentheses showing 1.0x for tachyon farm")
	}
	if !foundSwitchEnlightenmentNotice {
		t.Errorf("expected 'Switch to Enlightenment Egg' warning notice when not active on Enlightenment")
	}
	if !foundEnlightenmentBasisNotice {
		t.Errorf("expected statement that drone farming is based on being on the Enlightenment Farm")
	}

	// Test when user is active on a Virtue farm (e.g. Kindness)
	kindnessFarm := &ei.Backup_Simulation{
		EggType:      ei.Egg_KINDNESS.Enum(),
		LastStepTime: protoFloat64(1700000000),
	}
	optKindness := OptimizeDroneFarming(backup, kindnessFarm)
	if optKindness.IsAlreadyOptimal {
		t.Errorf("expected virtue farm without drone loadout to not be already optimal")
	}
	compsKindness := printDroneFarmingSetup("100000000000000001", backup, kindnessFarm, "", "")
	foundKindnessMultiplierFormat := false
	for _, c := range compsKindness {
		if td, ok := c.(dc.TextDisplay); ok {
			if strings.Contains(td.Content, "Optimal <::> GE Multiplier") && strings.Contains(td.Content, "(<::> 1.0x)") {
				foundKindnessMultiplierFormat = true
			}
			if strings.Contains(td.Content, "(Current:") {
				t.Errorf("expected '(Current:' to be replaced by egg emote, but found it in output")
			}
		}
	}
	if !foundKindnessMultiplierFormat {
		t.Errorf("expected 'Optimal <::> GE Multiplier' with kindness egg emote and 1.0x in parentheses for kindness farm")
	}

	// Test with Beak having a secondary non-clarity stone (e.g. Soul stone) and a 4th utility artifact (e.g. Ankh)
	soulStoneSpec := &ei.ArtifactSpec{
		Name:  ei.ArtifactSpec_SOUL_STONE.Enum(),
		Level: ei.ArtifactSpec_NORMAL.Enum(),
	}
	t4lAnkh := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_TUNGSTEN_ANKH.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(),
			Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
		},
		Stones: []*ei.ArtifactSpec{soulStoneSpec, soulStoneSpec, soulStoneSpec},
	}
	beakWithSoulAndClarity := &ei.CompleteArtifact{
		Spec:   t4lBeak.GetSpec(),
		Stones: []*ei.ArtifactSpec{soulStoneSpec, clarityStoneSpec}, // Slot 1: Soul, Slot 2: T4 Clarity
	}
	backup.ArtifactsDb.InventoryItems = []*ei.ArtifactInventoryItem{
		{ItemId: protoUint64(1), Artifact: equippedBrooch, Quantity: protoFloat64(1)},
		{ItemId: protoUint64(2), Artifact: equippedMedallion, Quantity: protoFloat64(1)},
		{ItemId: protoUint64(3), Artifact: beakWithSoulAndClarity, Quantity: protoFloat64(1)},
		{ItemId: protoUint64(4), Artifact: t4lAnkh, Quantity: protoFloat64(1)},
	}
	backup.ArtifactsDb.ActiveArtifactSets = []*ei.ArtifactsDB_ActiveArtifactSet{
		{
			Slots: []*ei.ActiveArtifactSlot{
				{Occupied: protoBool(true), ItemId: protoUint64(4)}, // 4th artifact: Ankh
				{Occupied: protoBool(true), ItemId: protoUint64(3)}, // Beak with Soul + Clarity
				{Occupied: protoBool(true), ItemId: protoUint64(2)}, // Medallion with 3x Clarity
				{Occupied: protoBool(true), ItemId: protoUint64(1)}, // Brooch with 3x Clarity
			},
		},
	}

	optWithVariations := OptimizeDroneFarming(backup, enlightenmentFarm)
	if !optWithVariations.IsAlreadyOptimal {
		t.Errorf("expected setup with Beak stone variation and 4th artifact to be recognized as already optimal")
	}
	if optWithVariations.CurrentGEDropChance != 1.0 {
		t.Errorf("expected 100%% GE drop chance with 1 T4 clarity on Beak, got %f", optWithVariations.CurrentGEDropChance)
	}

	compsVariations := printDroneFarmingSetup("100000000000000001", backup, enlightenmentFarm, "", "")
	foundVarOptimal := false
	foundOptimalSetupBlock := false
	for _, c := range compsVariations {
		if td, ok := c.(dc.TextDisplay); ok {
			if strings.Contains(td.Content, "Your current setup is already optimal!") {
				foundVarOptimal = true
			}
			if strings.Contains(td.Content, "Optimal Setup:") {
				foundOptimalSetupBlock = true
			}
		}
	}
	if !foundVarOptimal {
		t.Errorf("expected 'Your current setup is already optimal!' when Beak has secondary stone")
	}
	if foundOptimalSetupBlock {
		t.Errorf("expected no duplicate 'Optimal Setup:' block when already optimal")
	}

	// When there is no active event, event multiplier must NOT be added
	if optWithVariations.HasActiveEvent {
		t.Errorf("expected no active event when none is running")
	}
	for _, c := range compsVariations {
		if td, ok := c.(dc.TextDisplay); ok {
			if strings.Contains(td.Content, "Drone Event") || strings.Contains(td.Content, "× 4x Event") {
				t.Errorf("expected no event multiplier in output when no active drone event is running")
			}
		}
	}

	// Test with active periodical event in progress
	now := time.Now().UTC()
	ei.EventMutex.Lock()
	origEggIncEvents := ei.EggIncEvents
	ei.EggIncEvents = []ei.EggEvent{
		{
			ID:             "active-drone-event",
			EventType:      "drone-boost",
			Message:        "5x DRONE REWARDS!",
			Multiplier:     5,
			StartTimestamp: float64(now.Add(-1 * time.Hour).Unix()),
			EndTimestamp:   float64(now.Add(5 * time.Hour).Unix()),
			StartTime:      now.Add(-1 * time.Hour),
			EndTime:        now.Add(5 * time.Hour),
		},
	}
	ei.EventMutex.Unlock()
	defer func() {
		ei.EventMutex.Lock()
		ei.EggIncEvents = origEggIncEvents
		ei.EventMutex.Unlock()
	}()

	optActive := OptimizeDroneFarming(backup, kindnessFarm)
	if !optActive.HasActiveEvent {
		t.Errorf("expected HasActiveEvent to be true with active drone event")
	}
	if optActive.EventMultiplier != 5.0 {
		t.Errorf("expected EventMultiplier 5.0, got %f", optActive.EventMultiplier)
	}

	compsActive := printDroneFarmingSetup("100000000000000001", backup, kindnessFarm, "", "")
	foundActiveBanner := false
	for _, c := range compsActive {
		if td, ok := c.(dc.TextDisplay); ok {
			if strings.Contains(td.Content, "5x DRONE REWARDS!") && strings.Contains(td.Content, "Active Periodicals Event") {
				foundActiveBanner = true
			}
		}
	}
	if !foundActiveBanner {
		t.Errorf("expected active periodicals event banner with 5x DRONE REWARDS!")
	}
}

func TestGetStonesSafeWithNilSlots(t *testing.T) {
	// Shell stone has e.Slots == nil in data; calling GetStones must not panic
	slots, err := ei.GetStones(ei.ArtifactSpec_SHELL_STONE, ei.ArtifactSpec_LESSER, ei.ArtifactSpec_COMMON)
	if err != nil {
		t.Logf("GetStones returned error: %v", err)
	}
	if slots != 0 {
		t.Errorf("expected 0 slots for shell stone, got %d", slots)
	}

	spec := &ei.ArtifactSpec{
		Name:   ei.ArtifactSpec_SHELL_STONE.Enum(),
		Level:  ei.ArtifactSpec_LESSER.Enum(),
		Rarity: ei.ArtifactSpec_COMMON.Enum(),
	}
	if s := getArtifactSlotCount(spec); s != 0 {
		t.Errorf("expected 0 slots from getArtifactSlotCount, got %d", s)
	}
}

func TestOptimizeStandardPermitDelivery(t *testing.T) {
	testUser := "100000000000000001"
	tachyonEgg := ei.Egg_TACHYON
	farm := &ei.Backup_Simulation{
		FarmType:      ei.FarmType_HOME.Enum(),
		EggType:       &tachyonEgg,
		NumChickens:   protoUint64(10_000_000),
		HabPopulation: []uint64{2_500_000, 2_500_000, 2_500_000, 2_500_000},
		Habs:          []uint32{18, 18, 18, 18},
		Vehicles:      []uint32{11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11},
		TrainLength:   []uint32{10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10},
	}

	inventory := []*ei.ArtifactInventoryItem{
		{
			ItemId: protoUint64(1),
			Artifact: &ei.CompleteArtifact{
				Spec: &ei.ArtifactSpec{
					Name:   ei.ArtifactSpec_QUANTUM_METRONOME.Enum(),
					Level:  ei.ArtifactSpec_GREATER.Enum(),
					Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
				},
			},
			Quantity: protoFloat64(1),
		},
		{
			ItemId: protoUint64(2),
			Artifact: &ei.CompleteArtifact{
				Spec: &ei.ArtifactSpec{
					Name:   ei.ArtifactSpec_INTERSTELLAR_COMPASS.Enum(),
					Level:  ei.ArtifactSpec_GREATER.Enum(),
					Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
				},
			},
			Quantity: protoFloat64(1),
		},
		{
			ItemId: protoUint64(3),
			Artifact: &ei.CompleteArtifact{
				Spec: &ei.ArtifactSpec{
					Name:   ei.ArtifactSpec_ORNATE_GUSSET.Enum(),
					Level:  ei.ArtifactSpec_GREATER.Enum(),
					Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
				},
			},
			Quantity: protoFloat64(1),
		},
	}

	backup := &ei.Backup{
		UserId:   protoStr("EI1234567890123456"),
		UserName: protoStr("StdPermitDeliveryTester"),
		Farms:    []*ei.Backup_Simulation{farm},
		Game:     &ei.Backup_Game{PermitLevel: protoUint32(0)}, // Standard permit = 2 slots
		ArtifactsDb: &ei.ArtifactsDB{
			InventoryItems: inventory,
		},
	}

	res := OptimizeDelivery(backup, farm)
	if len(res.OptimalArtifacts) > 2 {
		t.Errorf("expected at most 2 optimal artifacts for standard permit, got %d", len(res.OptimalArtifacts))
	}
	if len(res.OptimalArtifacts) != 2 {
		t.Errorf("expected exactly 2 optimal artifacts, got %d", len(res.OptimalArtifacts))
	}

	comps := printDeliverySetup(testUser, backup, farm, "", "")
	body := comps[2].(dc.TextDisplay).Content
	if !strings.Contains(body, "Optimal Setup:") {
		t.Errorf("expected Optimal Setup in delivery: %s", body)
	}
}

func TestOptimizeStandardPermitEarnings(t *testing.T) {
	testUser := "100000000000000001"
	tachyonEgg := ei.Egg_TACHYON
	farm := &ei.Backup_Simulation{
		FarmType:      ei.FarmType_HOME.Enum(),
		EggType:       &tachyonEgg,
		NumChickens:   protoUint64(10_000_000),
		HabPopulation: []uint64{2_500_000, 2_500_000, 2_500_000, 2_500_000},
		Habs:          []uint32{18, 18, 18, 18},
	}

	inventory := []*ei.ArtifactInventoryItem{
		{
			ItemId: protoUint64(1),
			Artifact: &ei.CompleteArtifact{
				Spec: &ei.ArtifactSpec{
					Name:   ei.ArtifactSpec_LUNAR_TOTEM.Enum(),
					Level:  ei.ArtifactSpec_GREATER.Enum(),
					Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
				},
			},
			Quantity: protoFloat64(1),
		},
		{
			ItemId: protoUint64(2),
			Artifact: &ei.CompleteArtifact{
				Spec: &ei.ArtifactSpec{
					Name:   ei.ArtifactSpec_DEMETERS_NECKLACE.Enum(),
					Level:  ei.ArtifactSpec_GREATER.Enum(),
					Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
				},
			},
			Quantity: protoFloat64(1),
		},
		{
			ItemId: protoUint64(3),
			Artifact: &ei.CompleteArtifact{
				Spec: &ei.ArtifactSpec{
					Name:   ei.ArtifactSpec_TUNGSTEN_ANKH.Enum(),
					Level:  ei.ArtifactSpec_GREATER.Enum(),
					Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
				},
			},
			Quantity: protoFloat64(1),
		},
	}

	backup := &ei.Backup{
		UserId:   protoStr("EI1234567890123456"),
		UserName: protoStr("StdPermitEarningsTester"),
		Farms:    []*ei.Backup_Simulation{farm},
		Game:     &ei.Backup_Game{PermitLevel: protoUint32(0)}, // Standard permit = 2 slots
		ArtifactsDb: &ei.ArtifactsDB{
			InventoryItems: inventory,
		},
	}

	res := OptimizeEarnings(backup, farm)
	if len(res.OptimalArtifacts) > 2 {
		t.Errorf("expected at most 2 optimal artifacts for standard permit, got %d", len(res.OptimalArtifacts))
	}
	if len(res.OptimalArtifacts) != 2 {
		t.Errorf("expected exactly 2 optimal artifacts, got %d", len(res.OptimalArtifacts))
	}

	comps := printEarningsSetup(testUser, backup, farm, "", "")
	body := comps[2].(dc.TextDisplay).Content
	if !strings.Contains(body, "Optimal Setup:") {
		t.Errorf("expected Optimal Setup in earnings: %s", body)
	}
}

func TestOptimizeStandardPermitDroneFarming(t *testing.T) {
	testUser := "100000000000000001"
	enlightenmentEgg := ei.Egg_ENLIGHTENMENT
	farm := &ei.Backup_Simulation{
		FarmType: ei.FarmType_HOME.Enum(),
		EggType:  &enlightenmentEgg,
	}

	inventory := []*ei.ArtifactInventoryItem{
		{
			ItemId: protoUint64(1),
			Artifact: &ei.CompleteArtifact{
				Spec: &ei.ArtifactSpec{
					Name:   ei.ArtifactSpec_AURELIAN_BROOCH.Enum(),
					Level:  ei.ArtifactSpec_GREATER.Enum(),
					Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
				},
			},
			Quantity: protoFloat64(1),
		},
		{
			ItemId: protoUint64(2),
			Artifact: &ei.CompleteArtifact{
				Spec: &ei.ArtifactSpec{
					Name:   ei.ArtifactSpec_NEODYMIUM_MEDALLION.Enum(),
					Level:  ei.ArtifactSpec_GREATER.Enum(),
					Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
				},
			},
			Quantity: protoFloat64(1),
		},
		{
			ItemId: protoUint64(3),
			Artifact: &ei.CompleteArtifact{
				Spec: &ei.ArtifactSpec{
					Name:   ei.ArtifactSpec_BEAK_OF_MIDAS.Enum(),
					Level:  ei.ArtifactSpec_GREATER.Enum(),
					Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
				},
			},
			Quantity: protoFloat64(1),
		},
		{
			ItemId: protoUint64(4),
			Artifact: &ei.CompleteArtifact{
				Spec: &ei.ArtifactSpec{
					Name:  ei.ArtifactSpec_CLARITY_STONE.Enum(),
					Level: ei.ArtifactSpec_NORMAL.Enum(), // T4 100%
				},
			},
			Quantity: protoFloat64(10),
		},
	}

	backup := &ei.Backup{
		UserId:   protoStr("EI1234567890123456"),
		UserName: protoStr("StdPermitDroneTester"),
		Farms:    []*ei.Backup_Simulation{farm},
		Game:     &ei.Backup_Game{PermitLevel: protoUint32(0)}, // Standard permit = 2 slots
		ArtifactsDb: &ei.ArtifactsDB{
			InventoryItems: inventory,
		},
	}

	res := OptimizeDroneFarming(backup, farm)
	if len(res.OptimalArtifacts) != 2 {
		t.Fatalf("expected exactly 2 optimal artifacts for standard permit drone farming, got %d", len(res.OptimalArtifacts))
	}

	comps := printDroneFarmingSetup(testUser, backup, farm, "", "")
	body := comps[2].(dc.TextDisplay).Content
	if strings.Contains(body, "4th slot can be used for any utility") {
		t.Errorf("standard permit should not mention 4th slot utility tip: %s", body)
	}
}

func TestOptimizeStandardPermitVirtueEarnings(t *testing.T) {
	testUser := "100000000000000001"
	curiosityEgg := ei.Egg_CURIOSITY
	farm := &ei.Backup_Simulation{
		FarmType:      ei.FarmType_HOME.Enum(),
		EggType:       &curiosityEgg,
		NumChickens:   protoUint64(10_000_000),
		HabPopulation: []uint64{2_500_000, 2_500_000, 2_500_000, 2_500_000},
		Habs:          []uint32{18, 18, 18, 18},
	}

	inventory := []*ei.ArtifactInventoryItem{
		{
			ItemId: protoUint64(1),
			Artifact: &ei.CompleteArtifact{
				Spec: &ei.ArtifactSpec{
					Name:   ei.ArtifactSpec_LUNAR_TOTEM.Enum(),
					Level:  ei.ArtifactSpec_GREATER.Enum(),
					Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
				},
			},
			Quantity: protoFloat64(1),
		},
		{
			ItemId: protoUint64(2),
			Artifact: &ei.CompleteArtifact{
				Spec: &ei.ArtifactSpec{
					Name:   ei.ArtifactSpec_DEMETERS_NECKLACE.Enum(),
					Level:  ei.ArtifactSpec_GREATER.Enum(),
					Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
				},
			},
			Quantity: protoFloat64(1),
		},
		{
			ItemId: protoUint64(3),
			Artifact: &ei.CompleteArtifact{
				Spec: &ei.ArtifactSpec{
					Name:   ei.ArtifactSpec_TUNGSTEN_ANKH.Enum(),
					Level:  ei.ArtifactSpec_GREATER.Enum(),
					Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
				},
			},
			Quantity: protoFloat64(1),
		},
		{
			ItemId: protoUint64(4),
			Artifact: &ei.CompleteArtifact{
				Spec: &ei.ArtifactSpec{
					Name:  ei.ArtifactSpec_LUNAR_STONE.Enum(),
					Level: ei.ArtifactSpec_NORMAL.Enum(),
				},
			},
			Quantity: protoFloat64(12),
		},
	}

	backup := &ei.Backup{
		UserId:   protoStr("EI1234567890123456"),
		UserName: protoStr("StdPermitVirtueTester"),
		Farms:    []*ei.Backup_Simulation{farm},
		Game:     &ei.Backup_Game{PermitLevel: protoUint32(0)}, // Standard permit = 2 slots
		ArtifactsDb: &ei.ArtifactsDB{
			VirtueAfxDb: &ei.ArtifactsDB_VirtueDB{
				InventoryItems: inventory,
			},
		},
	}

	comps := printEarningsSetup(testUser, backup, farm, "", "")
	body := comps[2].(dc.TextDisplay).Content
	if !strings.Contains(body, "Optimal Setup:") {
		t.Errorf("expected Optimal Setup in virtue earnings: %s", body)
	}
}

func TestFormatSingleArtifactBuffs(t *testing.T) {
	t4lTotem := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_LUNAR_TOTEM.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(),
			Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
		},
		Stones: []*ei.ArtifactSpec{
			{Name: ei.ArtifactSpec_LUNAR_STONE.Enum(), Level: ei.ArtifactSpec_NORMAL.Enum()},
			{Name: ei.ArtifactSpec_LUNAR_STONE.Enum(), Level: ei.ArtifactSpec_NORMAL.Enum()},
			{Name: ei.ArtifactSpec_LUNAR_STONE.Enum(), Level: ei.ArtifactSpec_NORMAL.Enum()},
		},
	}
	buffStr := formatSingleArtifactBuffs(t4lTotem, false)
	if !strings.Contains(buffStr, "200x Away") || !strings.Contains(buffStr, "+174% (2.7x) Away") {
		t.Errorf("expected totem with 3 lunar stones to show stone buff and totem multiplier, got: %s", buffStr)
	}

	plainTotem := &ei.CompleteArtifact{
		Spec: &ei.ArtifactSpec{
			Name:   ei.ArtifactSpec_LUNAR_TOTEM.Enum(),
			Level:  ei.ArtifactSpec_GREATER.Enum(),
			Rarity: ei.ArtifactSpec_LEGENDARY.Enum(),
		},
	}
	plainStr := formatSingleArtifactBuffs(plainTotem, false)
	if plainStr != "200x Away" {
		t.Errorf("expected plain totem to show '200x Away', got: %s", plainStr)
	}
}
