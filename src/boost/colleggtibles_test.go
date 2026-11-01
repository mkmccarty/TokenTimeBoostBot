package boost

import (
	"bytes"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

func setupTestCustomEggMap() {
	ei.CustomEggMap = make(map[string]*ei.EggIncCustomEgg)
	ei.CustomEggMap["carbon-fiber"] = &ei.EggIncCustomEgg{
		ID:             "carbon-fiber",
		Name:           "Carbon Fiber",
		Dimension:      ei.GameModifier_SHIPPING_CAPACITY,
		DimensionValue: []float64{1.01, 1.02, 1.03, 1.05},
	}
	ei.CustomEggMap["chocolate"] = &ei.EggIncCustomEgg{
		ID:             "chocolate",
		Name:           "Chocolate",
		Dimension:      ei.GameModifier_AWAY_EARNINGS,
		DimensionValue: []float64{1.25, 1.5, 2.0, 3.0},
	}
	ei.CustomEggMap["easter"] = &ei.EggIncCustomEgg{
		ID:             "easter",
		Name:           "Easter",
		Dimension:      ei.GameModifier_INTERNAL_HATCHERY_RATE,
		DimensionValue: []float64{1.01, 1.02, 1.03, 1.05},
	}
	ei.CustomEggMap["firework"] = &ei.EggIncCustomEgg{
		ID:             "firework",
		Name:           "Firework",
		Dimension:      ei.GameModifier_EARNINGS,
		DimensionValue: []float64{1.01, 1.02, 1.03, 1.05},
	}
	ei.CustomEggMap["flame-retardant"] = &ei.EggIncCustomEgg{
		ID:             "flame-retardant",
		Name:           "Flame Retardant",
		Dimension:      ei.GameModifier_HAB_COST,
		DimensionValue: []float64{0.99, 0.95, 0.88, 0.75},
	}
	ei.CustomEggMap["gatoregg"] = &ei.EggIncCustomEgg{
		ID:             "gatoregg",
		Name:           "Gatoregg",
		Dimension:      ei.GameModifier_EGG_LAYING_RATE,
		DimensionValue: []float64{1.01, 1.02, 1.03, 1.05},
	}
	ei.CustomEggMap["ice"] = &ei.EggIncCustomEgg{
		ID:             "ice",
		Name:           "Ice",
		Dimension:      ei.GameModifier_RESEARCH_COST,
		DimensionValue: []float64{0.99, 0.98, 0.965, 0.95},
	}
	ei.CustomEggMap["lithium"] = &ei.EggIncCustomEgg{
		ID:             "lithium",
		Name:           "Lithium",
		Dimension:      ei.GameModifier_VEHICLE_COST,
		DimensionValue: []float64{0.98, 0.96, 0.93, 0.90},
	}
	ei.CustomEggMap["pegg"] = &ei.EggIncCustomEgg{
		ID:             "pegg",
		Name:           "P.E.G.G.",
		Dimension:      ei.GameModifier_HAB_CAPACITY,
		DimensionValue: []float64{1.01, 1.02, 1.03, 1.05},
	}
	ei.CustomEggMap["pumpkin"] = &ei.EggIncCustomEgg{
		ID:             "pumpkin",
		Name:           "Pumpkin",
		Dimension:      ei.GameModifier_SHIPPING_CAPACITY,
		DimensionValue: []float64{1.01, 1.02, 1.03, 1.05},
	}
	ei.CustomEggMap["silicon"] = &ei.EggIncCustomEgg{
		ID:             "silicon",
		Name:           "Silicon",
		Dimension:      ei.GameModifier_EGG_LAYING_RATE,
		DimensionValue: []float64{1.01, 1.02, 1.03, 1.05},
	}
	ei.CustomEggMap["waterballoon"] = &ei.EggIncCustomEgg{
		ID:             "waterballoon",
		Name:           "Waterballoon",
		Dimension:      ei.GameModifier_RESEARCH_COST,
		DimensionValue: []float64{0.99, 0.98, 0.97, 0.95},
	}
	ei.CustomEggMap["wood"] = &ei.EggIncCustomEgg{
		ID:             "wood",
		Name:           "Wood",
		Dimension:      ei.GameModifier_AWAY_EARNINGS,
		DimensionValue: []float64{1.1, 1.25, 1.5, 2.0},
	}
}

func TestGetSlashColleggtiblesCommand(t *testing.T) {
	cmd := GetSlashColleggtiblesCommand("colleggtibles")
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}
	if cmd.Name != "colleggtibles" {
		t.Errorf("expected command name colleggtibles, got %s", cmd.Name)
	}
	if config.IsDevBot() {
		if len(cmd.Options) != 2 {
			t.Fatalf("expected 2 options on dev bot, got %d", len(cmd.Options))
		}
		predictOpt, ok := cmd.Options[1].(dc.BoolOption)
		if !ok || predictOpt.Name != "predict" {
			t.Errorf("expected predict bool option, got %#v", cmd.Options[1])
		}
	} else {
		if len(cmd.Options) != 1 {
			t.Fatalf("expected 1 option, got %d", len(cmd.Options))
		}
	}
	opt, ok := cmd.Options[0].(dc.StringOption)
	if !ok {
		t.Fatalf("expected string option, got %T", cmd.Options[0])
	}
	if opt.Name != "alt" || !opt.Autocomplete {
		t.Errorf("expected alt option with autocomplete, got name=%s, autocomplete=%t", opt.Name, opt.Autocomplete)
	}
}

func TestBuildColleggtiblesComponents_Maxed(t *testing.T) {
	setupTestCustomEggMap()

	dummyID := "EI1234567890123456"
	maker := ei.NewBackupMaker(dummyID, "MaxFarmer")
	for id := range ei.CustomEggMap {
		maker.AddMaxColleggtibleContract(id)
	}

	backup := maker.GetBackup()
	components, files := BuildColleggtiblesComponents(backup, "user-test-max", "https://cdn.discordapp.com/avatars/123/avatar.png", false)

	if len(components) != 1 {
		t.Fatalf("expected 1 layout component, got %d", len(components))
	}

	container, ok := components[0].(dc.Container)
	if !ok {
		t.Fatalf("expected dc.Container, got %T", components[0])
	}
	if container.AccentColor != 0x0080FF {
		t.Errorf("expected AccentColor 0x0080FF, got 0x%06x", container.AccentColor)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file attached, got %d", len(files))
	}
	if files[0].Name != "colleggtibles.png" {
		t.Errorf("expected colleggtibles.png, got %s", files[0].Name)
	}

	// Verify Container has Section (header) and MediaGallery (image)
	hasGallery := false
	hasHeader := false
	for _, sub := range container.Components {
		switch c := sub.(type) {
		case dc.Section:
			for _, tc := range c.Components {
				if strings.Contains(tc.Content, "MaxFarmer's Colleggtibles") {
					hasHeader = true
				}
			}
		case dc.MediaGallery:
			if len(c.Items) > 0 && c.Items[0].URL == "attachment://colleggtibles.png" {
				hasGallery = true
			}
		}
	}

	if !hasHeader {
		t.Error("expected section header with player name")
	}
	if !hasGallery {
		t.Error("expected MediaGallery containing attachment://colleggtibles.png")
	}
}

func TestBuildColleggtiblesComponents_Partial(t *testing.T) {
	setupTestCustomEggMap()

	dummyID := "EI1234567890123456"
	maker := ei.NewBackupMaker(dummyID, "PartialFarmer")
	maker.AddColleggtibleContract("chocolate", 1e10) // 3x Maxed
	maker.AddColleggtibleContract("gatoregg", 1e8)   // Tier 1: +2% (100M)

	gatPredTime := time.Date(2026, time.November, 15, 9, 0, 0, 0, time.UTC)
	silPredTime := time.Date(2026, time.December, 1, 9, 0, 0, 0, time.UTC)
	mockPreds := map[string]time.Time{
		"gatoregg": gatPredTime,
		"silicon":  silPredTime,
	}

	backup := maker.GetBackup()
	components, files := BuildColleggtiblesComponents(backup, "user-test-part", "", false, mockPreds)

	if len(files) != 1 {
		t.Fatalf("expected 1 file attached, got %d", len(files))
	}

	var allText strings.Builder
	for _, sub := range components[0].(dc.Container).Components {
		switch c := sub.(type) {
		case dc.TextDisplay:
			allText.WriteString(c.Content)
			allText.WriteString("\n")
		case dc.Section:
			for _, tc := range c.Components {
				allText.WriteString(tc.Content)
				allText.WriteString("\n")
			}
		}
	}

	combined := allText.String()

	if !strings.Contains(combined, "2/13 Collected") {
		t.Errorf("expected '2/13 Collected', got: %s", combined)
	}
	if !strings.Contains(combined, "1/13 Maxed") {
		t.Errorf("expected '1/13 Maxed', got: %s", combined)
	}
	if !strings.Contains(combined, "11 Missing") {
		t.Errorf("expected '11 Missing', got: %s", combined)
	}
}

func TestRenderColleggtiblesImage(t *testing.T) {
	setupTestCustomEggMap()

	dummyID := "EI1234567890123456"
	maker := ei.NewBackupMaker(dummyID, "ImageFarmer")
	maker.AddColleggtibleContract("chocolate", 1e10) // 3x Maxed
	maker.AddColleggtibleContract("gatoregg", 1e8)   // Tier 1: +2% (100M)

	gatPredTime := time.Date(2026, time.November, 15, 9, 0, 0, 0, time.UTC)
	silPredTime := time.Date(2026, time.December, 1, 9, 0, 0, 0, time.UTC)
	mockPreds := map[string]time.Time{
		"gatoregg": gatPredTime,
		"silicon":  silPredTime,
	}

	backup := maker.GetBackup()
	overview := ei.GetColleggtiblesOverview(backup.GetContracts(), "")

	imgBytes, err := RenderColleggtiblesImage(overview, "ImageFarmer", mockPreds)
	if err != nil {
		t.Fatalf("unexpected error rendering image: %v", err)
	}
	if len(imgBytes) == 0 {
		t.Fatal("expected non-empty image bytes")
	}

	// Verify PNG can be decoded and dimensions are correct
	img, err := png.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		t.Fatalf("failed to decode generated PNG: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != 860 {
		t.Errorf("expected width 860, got %d", bounds.Dx())
	}
	if bounds.Dy() < 600 {
		t.Errorf("expected height >= 600, got %d", bounds.Dy())
	}

	// Also verify maxed player rendering
	maxMaker := ei.NewBackupMaker(dummyID, "MaxFarmer")
	for id := range ei.CustomEggMap {
		maxMaker.AddMaxColleggtibleContract(id)
	}
	maxOverview := ei.GetColleggtiblesOverview(maxMaker.GetBackup().GetContracts(), "")
	maxImgBytes, err := RenderColleggtiblesImage(maxOverview, "MaxFarmer", nil)
	if err != nil {
		t.Fatalf("unexpected error rendering maxed image: %v", err)
	}
	if len(maxImgBytes) == 0 {
		t.Fatal("expected non-empty maxed image bytes")
	}
}

func TestBuildColleggtiblesComponents_FallbackToStored(t *testing.T) {
	setupTestCustomEggMap()

	userID := "user-fallback-test"
	farmerstate.SetMiscSettingString(userID, "collegg", "Chocolate,Wood,Easter")

	components, files := BuildColleggtiblesComponents(nil, userID, "", false)
	if len(components) != 1 {
		t.Fatalf("expected 1 component, got %d", len(components))
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file attached, got %d", len(files))
	}

	var allText strings.Builder
	for _, sub := range components[0].(dc.Container).Components {
		switch c := sub.(type) {
		case dc.TextDisplay:
			allText.WriteString(c.Content)
			allText.WriteString("\n")
		}
	}
	combined := allText.String()

	if !strings.Contains(combined, "3/13 Collected") {
		t.Errorf("expected 3/13 Collected in header, got: %s", combined)
	}
}

func TestRenderColleggtiblesImage_ShowAllPredictions(t *testing.T) {
	setupTestCustomEggMap()

	dummyID := "EI1234567890123456"
	maker := ei.NewBackupMaker(dummyID, "MaxFarmerWithPreds")
	for id := range ei.CustomEggMap {
		maker.AddMaxColleggtibleContract(id)
	}
	maxOverview := ei.GetColleggtiblesOverview(maker.GetBackup().GetContracts(), "")

	gatPredTime := time.Date(2026, time.November, 15, 9, 0, 0, 0, time.UTC)
	mockPreds := map[string]time.Time{
		"gatoregg": gatPredTime,
	}

	// Without showAllPredictions, maxed egg should NOT show predictions
	imgBytesNoPred, err := RenderColleggtiblesImage(maxOverview, "MaxFarmer", mockPreds, false)
	if err != nil {
		t.Fatalf("unexpected error rendering without predictions: %v", err)
	}

	// With showAllPredictions = true, maxed egg SHOULD show predictions
	imgBytesWithPred, err := RenderColleggtiblesImage(maxOverview, "MaxFarmer", mockPreds, true)
	if err != nil {
		t.Fatalf("unexpected error rendering with predictions: %v", err)
	}

	if len(imgBytesWithPred) == 0 || len(imgBytesNoPred) == 0 {
		t.Fatal("expected non-empty image bytes")
	}

	// Decode both and check dimensions
	imgWith, err := png.Decode(bytes.NewReader(imgBytesWithPred))
	if err != nil {
		t.Fatalf("failed to decode image: %v", err)
	}
	if imgWith.Bounds().Dx() != 860 {
		t.Errorf("expected width 860, got %d", imgWith.Bounds().Dx())
	}
}
