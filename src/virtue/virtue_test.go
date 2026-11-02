package virtue

import (
	"encoding/base64"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"google.golang.org/protobuf/proto"
)

func TestGetSlashVirtueCommand(t *testing.T) {
	cmd := GetSlashVirtueCommand("virtue")
	if cmd.Name != "virtue" {
		t.Errorf("cmd.Name = %q; want virtue", cmd.Name)
	}
	if len(cmd.Options) != 6 {
		t.Fatalf("expected 6 options, got %d", len(cmd.Options))
	}

	expectedOptions := map[string]string{
		"simulate-shift":           "int",
		"simulate-shift-target-te": "int",
		"help":                     "bool",
		"reset":                    "bool",
		"compact":                  "bool",
		"alt":                      "string",
	}

	for _, opt := range cmd.Options {
		switch o := opt.(type) {
		case dc.IntOption:
			if expectedOptions[o.Name] != "int" {
				t.Errorf("unexpected int option: %s", o.Name)
			}
			if o.Name == "simulate-shift" && len(o.Choices) != 5 {
				t.Errorf("expected 5 choices for simulate-shift, got %d", len(o.Choices))
			}
			if o.Name == "simulate-shift-target-te" {
				if o.MinValue == nil || *o.MinValue != 1 {
					t.Errorf("expected MinValue 1, got %v", o.MinValue)
				}
				if o.MaxValue == nil || *o.MaxValue != 98 {
					t.Errorf("expected MaxValue 98, got %v", o.MaxValue)
				}
			}
		case dc.BoolOption:
			if expectedOptions[o.Name] != "bool" {
				t.Errorf("unexpected bool option: %s", o.Name)
			}
		case dc.StringOption:
			if expectedOptions[o.Name] != "string" {
				t.Errorf("unexpected string option: %s", o.Name)
			}
			if o.Name == "alt" && !o.Autocomplete {
				t.Errorf("expected alt option to have Autocomplete=true")
			}
		default:
			t.Errorf("unexpected option type for %v", opt)
		}
	}

	if len(cmd.IntegrationTypes) != 2 || cmd.IntegrationTypes[0] != dc.IntegrationGuildInstall || cmd.IntegrationTypes[1] != dc.IntegrationUserInstall {
		t.Errorf("expected IntegrationTypes to have GuildInstall and UserInstall, got %v", cmd.IntegrationTypes)
	}
	if len(cmd.Contexts) != 3 {
		t.Errorf("expected 3 Contexts, got %v", cmd.Contexts)
	}
}

func TestGetHabIconStrings(t *testing.T) {
	// Provide a mock GetBotEmojiMarkdown function
	getBotEmojiMarkdown := func(name string) string {
		return fmt.Sprintf("<:%s:>", name)
	}

	testCases := []struct {
		name             string
		habs             []uint32
		expectedHabArt   string
		expectedHabArray []string
	}{
		{
			name:             "Single low-tier hab",
			habs:             []uint32{0}, // Coop
			expectedHabArt:   "<:hab1:>",
			expectedHabArray: []string{"<:hab1:>"},
		},
		{
			name:             "Multiple low-tier habs",
			habs:             []uint32{0, 1, 2},
			expectedHabArt:   "<:hab3:>",
			expectedHabArray: []string{"<:hab1:>", "<:hab2:>", "<:hab3:>"},
		},
		{
			name:             "Includes high-tier habs",
			habs:             []uint32{0, 8, 17}, // Coop, Bunker, Planet Portal
			expectedHabArt:   "<:hab18:>",
			expectedHabArray: []string{"<:hab1:>", "<:hab9:>", "<:hab18:>"},
		},
		{
			name:             "Highest hab is Chicken Universe",
			habs:             []uint32{18}, // Chicken Universe
			expectedHabArt:   "<:hab19:>",  // highestHab is not updated because 18 is not < 19
			expectedHabArray: []string{"<:hab19:>"},
		},
		{
			name:             "Mixed with highest hab",
			habs:             []uint32{4, 18, 17},
			expectedHabArt:   "<:hab19:>",
			expectedHabArray: []string{"<:hab5:>", "<:hab19:>", "<:hab18:>"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			habArt, habArray := getHabIconStrings(tc.habs, getBotEmojiMarkdown)
			if habArt != tc.expectedHabArt {
				t.Errorf("getHabIconStrings() habArt = %v, want %v", habArt, tc.expectedHabArt)
			}
			if !slices.Equal(habArray, tc.expectedHabArray) {
				t.Errorf("getHabIconStrings() habArray = %v, want %v", habArray, tc.expectedHabArray)
			}
		})
	}
}

func TestVirtueData_Integration(t *testing.T) {
	maker := ei.NewBackupMaker("EI123456", "TestUser")

	// Extracted from firstcontact JSON fixture
	eovEarned := []uint32{20, 20, 21, 20, 20}
	eggsDelivered := []float64{671228376732604700, 281084978380496960, 1861466584249923000, 321860104553812600, 766522163167315600}
	tankFuels := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 77804200160814.05, 247867287321.0276, 1759725104048.5757, 65000000000001.2, 75480247097414}

	backup := maker.GetBackup()
	virtue := backup.GetVirtue()

	if virtue == nil {
		t.Fatalf("Expected Virtue data to be initialized")
	}

	// Evaluate standard function calls that would be made in virtue.go
	if virtue.GetShiftCount() != 36 {
		t.Errorf("Expected ShiftCount 36, got %d", virtue.GetShiftCount())
	}
	if virtue.GetResets() != 1 {
		t.Errorf("Expected Resets 1, got %d", virtue.GetResets())
	}
	if virtue.GetPastSimTime() != 11215366.125829924 {
		t.Errorf("Expected PastSimTime 11215366.125829924, got %f", virtue.GetPastSimTime())
	}

	// Verify array populations
	if !slices.Equal(virtue.GetEovEarned(), eovEarned) {
		t.Errorf("EoV earned did not match expected values")
	}
	if !slices.Equal(virtue.GetEggsDelivered(), eggsDelivered) {
		t.Errorf("Eggs delivered did not match expected values")
	}

	// Evaluate AFX components inside Virtue
	afx := virtue.GetAfx()
	if afx == nil {
		t.Fatalf("Expected Virtue AFX to be initialized")
	}
	if afx.GetInventoryScore() != 48448.569999999956 {
		t.Errorf("Expected Inventory Score 48448.569999999956, got %f", afx.GetInventoryScore())
	}
	if !slices.Equal(afx.GetTankFuels(), tankFuels) {
		t.Errorf("Tank fuels did not match expected values")
	}
}

// Mock ei.GetBotEmojiMarkdown
func TestGetVehicleIconStrings(t *testing.T) {
	// Provide a mock GetBotEmojiMarkdown function
	getBotEmojiMarkdown := func(name string) string {
		return fmt.Sprintf("<:%s:>", name)
	}

	testCases := []struct {
		name                 string
		vehicles             []uint32
		trainLength          []uint32
		expectedVehicleArt   string
		expectedVehicleArray string
	}{
		{"Single vehicle", []uint32{0}, []uint32{0}, "<:veh0:>", "<:veh0:>"},
		{"Multiple unique vehicles", []uint32{0, 5, 11}, []uint32{1, 1, 1}, "<:veh11:>", "<:veh0:><:veh5:><:veh11:>"},
		{"Multiple same vehicles", []uint32{5, 5, 5}, []uint32{1, 1, 1}, "<:veh5:>", "<:veh5:>x3"},
		{"Mixed unique and multiple", []uint32{0, 5, 5, 11, 11, 11}, []uint32{1, 1, 1, 2, 3, 4}, "<:veh11:>", "<:veh0:><:veh5:>x2<:veh11:><:tl:><:tl:><:veh11:><:tl:><:tl:><:tl:><:veh11:><:tl:><:tl:><:tl:><:tl:>"},
		{"Maximum Vehicles", []uint32{11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11}, []uint32{10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10}, "<:veh11:>", "<:veh11:><:tl:><:tl:><:tl:><:tl:><:tl:><:tl:><:tl:><:tl:><:tl:><:tl:>x17"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vehicleArt, vehicleArray := getVehicleIconStrings(tc.vehicles, tc.trainLength, getBotEmojiMarkdown)
			if vehicleArt != tc.expectedVehicleArt {
				t.Errorf("getVehicleIconStrings() vehicleArt = %v, want %v", vehicleArt, tc.expectedVehicleArt)
			}
			if vehicleArray != tc.expectedVehicleArray {
				t.Errorf("getVehicleIconStrings() vehicleArray = %v, want %v", vehicleArray, tc.expectedVehicleArray)
			}
		})
	}
}

func TestPrintVirtue_AvatarAccessory(t *testing.T) {
	maker := ei.NewBackupMaker("EI123456", "TestUser")
	backup := maker.GetBackup()
	avatarURL := "https://cdn.discordapp.com/avatars/123/avatar.png"

	comps := printVirtue("user-test", backup, ei.Egg(-1), 0, true, avatarURL)
	if len(comps) == 0 {
		t.Fatalf("expected components, got empty")
	}

	section, ok := comps[0].(dc.Section)
	if !ok {
		t.Fatalf("expected first component to be dc.Section, got %T", comps[0])
	}

	thumb, ok := section.Accessory.(dc.Thumbnail)
	if !ok {
		t.Fatalf("expected section accessory to be dc.Thumbnail, got %T", section.Accessory)
	}
	if thumb.URL != avatarURL {
		t.Errorf("expected thumbnail URL %q, got %q", avatarURL, thumb.URL)
	}

	// Test fallback when avatar URL is empty and egg_truth emoji is configured
	origEmoteMap := ei.EmoteMap
	defer func() {
		ei.EmoteMap = origEmoteMap
	}()
	ei.EmoteMap = map[string]ei.Emotes{
		"egg_truth": {
			Name: "egg_truth",
			ID:   "987654321012345678",
		},
	}

	compsDynamic := printVirtue("user-test", backup, ei.Egg(-1), 0, true, "")
	if len(compsDynamic) == 0 {
		t.Fatalf("expected components on dynamic fallback, got empty")
	}
	sectionDynamic, ok := compsDynamic[0].(dc.Section)
	if !ok {
		t.Fatalf("expected first component to be dc.Section on dynamic fallback, got %T", compsDynamic[0])
	}
	thumbDynamic, ok := sectionDynamic.Accessory.(dc.Thumbnail)
	if !ok {
		t.Fatalf("expected section accessory to be dc.Thumbnail on dynamic fallback, got %T", sectionDynamic.Accessory)
	}
	wantDynamic := "https://cdn.discordapp.com/emojis/987654321012345678.webp?size=128"
	if thumbDynamic.URL != wantDynamic {
		t.Errorf("expected dynamic thumbnail URL %q, got %q", wantDynamic, thumbDynamic.URL)
	}

	// Test fallback when egg_truth emoji is not found
	ei.EmoteMap = map[string]ei.Emotes{}
	compsFallback := printVirtue("user-test", backup, ei.Egg(-1), 0, true, "")
	if len(compsFallback) == 0 {
		t.Fatalf("expected components on fallback, got empty")
	}
	sectionFallback, ok := compsFallback[0].(dc.Section)
	if !ok {
		t.Fatalf("expected first component to be dc.Section on fallback, got %T", compsFallback[0])
	}
	thumbFallback, ok := sectionFallback.Accessory.(dc.Thumbnail)
	if !ok {
		t.Fatalf("expected section accessory to be dc.Thumbnail on fallback, got %T", sectionFallback.Accessory)
	}
	wantFallback := "https://cdn.discordapp.com/emojis/1418022084205875210.webp?size=128"
	if thumbFallback.URL != wantFallback {
		t.Errorf("expected static fallback thumbnail URL %q, got %q", wantFallback, thumbFallback.URL)
	}
}

func TestPrintVirtue_ActAsFooter(t *testing.T) {
	maker := ei.NewBackupMaker("EI1234567890123456", "TestUser")
	backup := maker.GetBackup()

	// 1. Without ActAs notice
	compsWithout := printVirtue("user-test", backup, ei.Egg(-1), 0, true, "", "")
	if len(compsWithout) == 0 {
		t.Fatalf("expected components, got empty")
	}
	lastCompWithout, ok := compsWithout[len(compsWithout)-1].(dc.TextDisplay)
	if !ok {
		t.Fatalf("expected last component to be dc.TextDisplay, got %T", compsWithout[len(compsWithout)-1])
	}
	if strings.Contains(lastCompWithout.Content, "via /act-as") {
		t.Errorf("expected no act-as footer when actAsNotice is empty, got: %s", lastCompWithout.Content)
	}

	// 2. With ActAs notice
	actAsNotice := "Acting as <@EI1234567890123456> via /act-as (expires in 15 minutes)"
	compsWith := printVirtue("user-test", backup, ei.Egg(-1), 0, true, "", actAsNotice)
	if len(compsWith) == 0 {
		t.Fatalf("expected components, got empty")
	}
	lastCompWith, ok := compsWith[len(compsWith)-1].(dc.TextDisplay)
	if !ok {
		t.Fatalf("expected last component to be dc.TextDisplay, got %T", compsWith[len(compsWith)-1])
	}
	expectedFooterLine := "-# " + actAsNotice
	if !strings.Contains(lastCompWith.Content, expectedFooterLine) {
		t.Errorf("expected act-as footer line %q, got: %s", expectedFooterLine, lastCompWith.Content)
	}
}

func TestDefaultVirtueAvatarURL_Dynamic(t *testing.T) {
	origEmoteMap := ei.EmoteMap
	defer func() {
		ei.EmoteMap = origEmoteMap
	}()

	// 1. When egg_truth emoji is present, URL is dynamically constructed from its ID
	ei.EmoteMap = map[string]ei.Emotes{
		"egg_truth": {
			Name: "egg_truth",
			ID:   "112233445566778899",
		},
	}
	got := defaultVirtueAvatarURL()
	want := "https://cdn.discordapp.com/emojis/112233445566778899.webp?size=128"
	if got != want {
		t.Errorf("defaultVirtueAvatarURL() = %q; want %q", got, want)
	}

	// 2. When egg_truth emoji is missing, falls back to the static default
	ei.EmoteMap = map[string]ei.Emotes{}
	gotMissing := defaultVirtueAvatarURL()
	wantMissing := "https://cdn.discordapp.com/emojis/1418022084205875210.webp?size=128"
	if gotMissing != wantMissing {
		t.Errorf("defaultVirtueAvatarURL() on missing emoji = %q; want %q", gotMissing, wantMissing)
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

func TestHandleVirtueAutocomplete(t *testing.T) {
	parentUser := "4" // dctest.AutocompleteEvent uses user ID "4"
	altA := "alt-alpha-virtue"
	altB := "alt-beta-virtue"
	dummyEID1 := "EI4444444444444444"
	dummyEID2 := "EI5555555555555555"

	enc1 := makeTestEncryptedEID(t, dummyEID1)
	enc2 := makeTestEncryptedEID(t, dummyEID2)

	farmerstate.SetMiscSettingString(altA, "AltController", parentUser)
	farmerstate.SetMiscSettingString(altA, "encrypted_ei_id", enc1)
	farmerstate.SetMiscSettingString(altA, "ei_ign", "AlphaFarm")

	farmerstate.SetMiscSettingString(altB, "AltController", parentUser)
	farmerstate.SetMiscSettingString(altB, "encrypted_ei_id", enc2)
	farmerstate.SetMiscSettingString(altB, "ei_ign", "BetaFarm")

	// Autocomplete for focused option "alt" with empty search
	eventAll := dctest.AutocompleteEvent("virtue", "alt", "")
	HandleVirtueAutocomplete(eventAll)

	choices := eventAll.LastChoices
	if len(choices) != 2 {
		t.Fatalf("expected 2 choices, got %d", len(choices))
	}

	// Autocomplete with filter "alpha"
	eventFilter := dctest.AutocompleteEvent("virtue", "alt", "alpha")
	HandleVirtueAutocomplete(eventFilter)
	filteredChoices := eventFilter.LastChoices
	if len(filteredChoices) != 1 {
		t.Fatalf("expected 1 choice for 'alpha', got %d", len(filteredChoices))
	}
	if filteredChoices[0].Value != altA {
		t.Errorf("expected choice value %q, got %q", altA, filteredChoices[0].Value)
	}

	// Autocomplete for user with no alts
	farmerstate.SetMiscSettingString(altA, "AltController", "other-user")
	farmerstate.SetMiscSettingString(altB, "AltController", "other-user")
	eventEmpty := dctest.AutocompleteEvent("virtue", "alt", "")
	HandleVirtueAutocomplete(eventEmpty)
	if len(eventEmpty.LastChoices) != 0 {
		t.Errorf("expected 0 choices for user without alts, got %d", len(eventEmpty.LastChoices))
	}
}

func TestHandleVirtueActAs(t *testing.T) {
	callerID := "4"
	altUser := "100000000000000088"
	dummyEID := "EI7777777777777777"

	enc := makeTestEncryptedEID(t, dummyEID)
	farmerstate.SetMiscSettingString("sub-virtue-alt-1", "AltController", altUser)
	farmerstate.SetMiscSettingString("sub-virtue-alt-1", "encrypted_ei_id", enc)
	farmerstate.SetMiscSettingString("sub-virtue-alt-1", "ei_ign", "VirtueAltFarm")

	// Initially caller has no alts
	eventBefore := dctest.AutocompleteEvent("virtue", "alt", "")
	HandleVirtueAutocomplete(eventBefore)
	if len(eventBefore.LastChoices) != 0 {
		t.Errorf("expected 0 choices before act-as switch, got %d", len(eventBefore.LastChoices))
	}

	// Link and switch
	_ = farmerstate.AddActAsLink(callerID, altUser)
	_ = farmerstate.SetActAsSwitch(callerID, "3", altUser, time.Now().Add(1*time.Hour))

	// When acting as altUser, autocomplete should suggest altUser's alts
	eventDuring := dctest.AutocompleteEvent("virtue", "alt", "")
	HandleVirtueAutocomplete(eventDuring)
	if len(eventDuring.LastChoices) != 1 || eventDuring.LastChoices[0].Value != "sub-virtue-alt-1" {
		t.Errorf("expected 1 choice for 'sub-virtue-alt-1' while acting as %s, got %+v", altUser, eventDuring.LastChoices)
	}

	// Clean up
	_ = farmerstate.ClearActAsSwitch(callerID, "3")
	_ = farmerstate.RemoveActAsLink(callerID, altUser)
}

func TestFindAltDiscordID_ActAsLink(t *testing.T) {
	mainUser := "111111111111111111"
	altDiscordUser := "222222222222222222"
	altNamed := "chipmunk-alt"

	farmerstate.SetMiscSettingString(altNamed, "AltController", mainUser)
	_ = farmerstate.AddActAsLink(mainUser, altDiscordUser)
	defer func() {
		_ = farmerstate.RemoveActAsLink(mainUser, altDiscordUser)
	}()

	// With AltController set, FindAltDiscordID should resolve the linked Discord account from /act-as
	got := farmerstate.FindAltDiscordID(altNamed)
	if got != altDiscordUser {
		t.Errorf("FindAltDiscordID(%q) = %q; want %q", altNamed, got, altDiscordUser)
	}

	// Passing caller ID should also resolve it even if AltController wasn't set
	farmerstate.SetMiscSettingString("orphan-alt", "AltController", "")
	gotCaller := farmerstate.FindAltDiscordID("orphan-alt", mainUser)
	if gotCaller != altDiscordUser {
		t.Errorf("FindAltDiscordID(orphan-alt, %q) = %q; want %q", mainUser, gotCaller, altDiscordUser)
	}
}

func TestPrintVirtue_ShiftSimulationUsesActualDeliveryRate(t *testing.T) {
	for _, path := range []string{"ttbb-data/ei-researches.json", "../../ttbb-data/ei-researches.json"} {
		if _, err := os.Stat(path); err == nil {
			ei.LoadResearchData(path)
			break
		}
	}
	if len(ei.EggIncResearchesMap) == 0 {
		ei.EggIncResearchesMap = make(map[string]ei.EggResearches)
	}
	ensureResearch := func(id, effectType string, perLevel float64, compound string) {
		if _, ok := ei.EggIncResearchesMap[id]; !ok {
			ei.EggIncResearchesMap[id] = ei.EggResearches{
				ID:             id,
				EffectType:     effectType,
				PerLevel:       perLevel,
				LevelsCompound: compound,
			}
		}
	}
	ensureResearch("internal_hatchery1", "additive", 2, "additive")
	ensureResearch("internal_hatchery2", "additive", 5, "additive")
	ensureResearch("internal_hatchery3", "additive", 10, "additive")
	ensureResearch("internal_hatchery4", "additive", 25, "additive")
	ensureResearch("internal_hatchery5", "additive", 5, "additive")
	ensureResearch("epic_internal_incubators", "multiplicative", 0.05, "additive")
	ensureResearch("int_hatch_calm", "multiplicative", 0.1, "additive")
	ensureResearch("comfy_nests", "multiplicative", 0.1, "additive")
	ensureResearch("hen_house_ac", "multiplicative", 0.05, "additive")
	ensureResearch("improved_genetics", "multiplicative", 0.15, "additive")
	ensureResearch("time_compress", "multiplicative", 0.1, "additive")
	ensureResearch("timeline_diversion", "multiplicative", 0.02, "additive")
	ensureResearch("relativity_optimization", "multiplicative", 0.1, "additive")
	ensureResearch("epic_egg_laying", "multiplicative", 0.05, "additive")
	ensureResearch("leafsprings", "multiplicative", 0.05, "additive")
	ensureResearch("lightweight_boxes", "multiplicative", 0.1, "additive")
	ensureResearch("driver_training", "multiplicative", 0.05, "additive")
	ensureResearch("super_alloy", "multiplicative", 0.05, "additive")
	ensureResearch("quantum_storage", "multiplicative", 0.05, "additive")
	ensureResearch("dark_containment", "multiplicative", 0.05, "additive")
	ensureResearch("neural_net_refine", "multiplicative", 0.05, "additive")
	ensureResearch("hyper_portalling", "multiplicative", 0.05, "additive")
	ensureResearch("transportation_lobbyist", "multiplicative", 0.05, "additive")

	maker := ei.NewBackupMaker("EI1234567890123456", "TestUser")
	backup := maker.GetBackup()
	farm := backup.GetFarms()[0]
	now := float64(time.Now().Unix())
	farm.LastStepTime = &now
	farm.HabPopulation = []uint64{2421090000, 2421090000, 2421090000, 2421090000}
	farm.Habs = []uint32{18, 18, 18, 18}
	farm.CommonResearch = append(farm.CommonResearch,
		&ei.Backup_ResearchItem{Id: proto.String("internal_hatchery1"), Level: proto.Uint32(10)},
		&ei.Backup_ResearchItem{Id: proto.String("internal_hatchery2"), Level: proto.Uint32(10)},
		&ei.Backup_ResearchItem{Id: proto.String("internal_hatchery3"), Level: proto.Uint32(10)},
		&ei.Backup_ResearchItem{Id: proto.String("internal_hatchery4"), Level: proto.Uint32(10)},
		&ei.Backup_ResearchItem{Id: proto.String("internal_hatchery5"), Level: proto.Uint32(10)},
	)
	backup.Game.EpicResearch = append(backup.Game.EpicResearch,
		&ei.Backup_ResearchItem{Id: proto.String("epic_internal_incubators"), Level: proto.Uint32(20)},
		&ei.Backup_ResearchItem{Id: proto.String("int_hatch_calm"), Level: proto.Uint32(20)},
	)
	farm.Vehicles = []uint32{11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11}
	farm.TrainLength = []uint32{10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10}
	backup.Virtue.Afx.TankFillingEnabled = proto.Bool(false)

	shippingRate := ei.GetShippingRateFromBackup(farm, backup.GetGame())

	// 1. Immediate shift (targetTE = 0)
	comps1 := printVirtue("user-test", backup, ei.Egg_INTEGRITY, 0, false, "")
	if len(comps1) == 0 {
		t.Fatalf("expected components, got empty")
	}

	// 2. Shift after targetTE on current egg (targetTE = 25)
	comps2 := printVirtue("user-test", backup, ei.Egg_INTEGRITY, 25, false, "")
	if len(comps2) == 0 {
		t.Fatalf("expected components, got empty")
	}

	sec1, ok1 := comps1[0].(dc.Section)
	sec2, ok2 := comps2[0].(dc.Section)
	if !ok1 || !ok2 || len(sec1.Components) == 0 || len(sec2.Components) == 0 {
		t.Fatalf("expected Section components")
	}

	content1 := sec1.Components[0].Content
	content2 := sec2.Components[0].Content

	extractTimestamps := func(text string) []int64 {
		var ts []int64
		for _, line := range strings.Split(text, "\n") {
			if idx := strings.Index(line, "<t:"); idx != -1 {
				var val int64
				if n, _ := fmt.Sscanf(line[idx:], "<t:%d:", &val); n == 1 {
					ts = append(ts, val)
				}
			}
		}
		return ts
	}

	ts1 := extractTimestamps(content1)
	if len(ts1) < 2 {
		t.Fatalf("expected at least 2 timestamps in content1, got %v", ts1)
	}
	diff1 := ts1[1] - ts1[0]
	minExpectedDiff := int64(20 * 3600) // At least 20 hours
	if diff1 < minExpectedDiff {
		t.Errorf("simulated shift (targetTE=0) between milestone 21 and 22 took %d s; want >= %d s (delivery rate, not max shipping rate)", diff1, minExpectedDiff)
	}

	ts2 := extractTimestamps(content2)
	// content2 has shift time first, then milestone 21, then milestone 22
	if len(ts2) < 3 {
		t.Fatalf("expected at least 3 timestamps in content2, got %v", ts2)
	}
	diff2 := ts2[2] - ts2[1]
	if diff2 < minExpectedDiff {
		t.Errorf("simulated shift (targetTE=25) between milestone 21 and 22 took %d s; want >= %d s (delivery rate, not max shipping rate)", diff2, minExpectedDiff)
	}

	// The interval between milestone 21 and 22 should be identical for both simulations (within 1s for integer timestamp rounding)
	diffDelta := diff1 - diff2
	if diffDelta < -1 || diffDelta > 1 {
		t.Errorf("expected interval between milestones 21 and 22 to match between targetTE=0 (%d s) and targetTE=25 (%d s)", diff1, diff2)
	}

	// Also ensure duration was not simply 90q / shippingRate
	timeAtMaxShipping := int64((90e15 / shippingRate) * 3600)
	if diff1 == timeAtMaxShipping {
		t.Errorf("simulated shift incorrectly used max shipping rate (%d s)", timeAtMaxShipping)
	}
}
