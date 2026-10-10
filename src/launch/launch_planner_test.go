package launch

import (
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

func TestGetSlashLaunchPlannerCommand(t *testing.T) {
	cmd := GetSlashLaunchPlannerCommand("launch-planner")
	if cmd == nil {
		t.Fatal("Expected command to not be nil")
	}
	if cmd.Name != "launch-planner" {
		t.Errorf("Expected cmd.Name to be 'launch-planner', got '%s'", cmd.Name)
	}
	if len(cmd.Options) < 4 {
		t.Errorf("Expected at least 4 options, got %d", len(cmd.Options))
	}

	hasGoal := false
	hasDubcap := false
	hasAlt := false
	hasTarget := false

	for _, opt := range cmd.Options {
		switch o := opt.(type) {
		case dc.StringOption:
			if o.Name == "goal" {
				hasGoal = true
				if len(o.Choices) != 4 {
					t.Errorf("Expected 4 goal choices, got %d", len(o.Choices))
				}
			}
			if o.Name == "alt" && o.Autocomplete {
				hasAlt = true
			}
		case dc.BoolOption:
			if o.Name == "dubcap" {
				hasDubcap = true
			}
		case dc.IntOption:
			if o.Name == "target-launches" {
				hasTarget = true
			}
		}
	}

	if !hasGoal {
		t.Error("Missing 'goal' option")
	}
	if !hasDubcap {
		t.Error("Missing 'dubcap' option")
	}
	if !hasAlt {
		t.Error("Missing 'alt' autocomplete option")
	}
	if !hasTarget {
		t.Error("Missing 'target-launches' option")
	}
}

func TestBuildPlannerDialog_NormalFarm(t *testing.T) {
	dummyEI := "EI1234567890123456"
	dummyUser := "userA"
	dummyName := "TestFarmer"

	backupMaker := ei.NewBackupMaker(dummyEI, dummyName)
	backup := backupMaker.GetBackup()

	// Setup normal farm on Dark Matter
	homeFarmType := ei.FarmType_HOME
	eggType := ei.Egg_DARK_MATTER
	backup.Farms = []*ei.Backup_Simulation{{
		EggType:  &eggType,
		FarmType: &homeFarmType,
	}}

	// Setup tank fuels
	tankFuels := make([]float64, 25)
	tankFuels[8] = 10e12  // 10T Tachyon
	tankFuels[10] = 30e12 // 30T Dilithium
	tankFuels[13] = 30e12 // 30T Antimatter
	tankFuels[14] = 30e12 // 30T Dark Matter
	if backup.Artifacts != nil {
		backup.Artifacts.TankFuels = tankFuels
		lvl := uint32(7)
		backup.Artifacts.TankLevel = &lvl
	}

	// Add an active mission: Atreggies Henliner Extended (ship 10, dur 2)
	now := float64(time.Now().Unix())
	ship := ei.MissionInfo_ATREGGIES
	dur := ei.MissionInfo_EPIC
	status := ei.MissionInfo_EXPLORING
	level := uint32(8)
	durSec := 86400.0 * 2.5
	startTime := now - 3600

	activeMission := &ei.MissionInfo{
		Ship:             &ship,
		DurationType:     &dur,
		Status:           &status,
		Level:            &level,
		DurationSeconds:  &durSec,
		StartTimeDerived: &startTime,
	}

	virtueMissionType := ei.MissionInfo_VIRTUE
	virtueMission := &ei.MissionInfo{
		Ship:             &ship,
		DurationType:     &dur,
		Status:           &status,
		Level:            &level,
		DurationSeconds:  &durSec,
		StartTimeDerived: &startTime,
		Type:             &virtueMissionType,
	}

	if backup.ArtifactsDb == nil {
		backup.ArtifactsDb = &ei.ArtifactsDB{}
	}
	backup.ArtifactsDb.MissionInfos = []*ei.MissionInfo{activeMission, virtueMission}

	farmerstate.SetMiscSettingString(dummyUser, "launch_planner_goal", GoalArtifactHunt)
	farmerstate.SetMiscSettingFlag(dummyUser, "launch_planner_dubcap", true)

	msg := buildPlannerDialog(dummyUser, backup, "")
	if len(msg.Components) == 0 {
		t.Fatal("Expected dialog to contain layout components")
	}

	container, ok := msg.Components[0].(dc.Container)
	if !ok {
		t.Fatal("Expected top level component to be a dc.Container")
	}

	foundActive := false
	foundFuel := false
	foundDubcap := false

	for _, sub := range container.Components {
		if textDisp, ok := sub.(dc.TextDisplay); ok {
			content := textDisp.Content
			if strings.Contains(content, "Returning Rockets & Ship Quality") {
				foundActive = true
				if !strings.Contains(content, "Atreggies Henliner") {
					t.Error("Expected active rocket section to show Atreggies Henliner")
				}
				if !strings.Contains(content, "Extended") {
					t.Error("Expected active rocket section to show Extended duration")
				}
				if !strings.Contains(content, "Quality") {
					t.Error("Expected active rocket section to show Quality")
				}
				// Should only have 1 active rocket shown (the standard one, NOT the virtue one)
				if strings.Count(content, "Atreggies Henliner") != 1 {
					t.Error("Expected virtue rocket to NOT be displayed when on home farm")
				}
			}
			if strings.Contains(content, "Fuel Tank Advisor") {
				foundFuel = true
			}
			if strings.Contains(content, "Sunday Double Capacity Event") {
				foundDubcap = true
			}
		}
	}

	if !foundActive {
		t.Error("Expected active rocket section in dialog")
	}
	if !foundFuel {
		t.Error("Expected fuel advisor section in dialog")
	}
	if !foundDubcap {
		t.Error("Expected dubcap section when dubcap is enabled")
	}
}

func TestBuildPlannerDialog_VirtueFarm(t *testing.T) {
	dummyEI := "EI1234567890123456"
	dummyUser := "userA"
	dummyName := "VirtueFarmer"

	backupMaker := ei.NewBackupMaker(dummyEI, dummyName)
	backup := backupMaker.GetBackup()

	// Virtue tank fuels (last 5 are virtue eggs)
	tankFuels := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 50e12, 10e12, 5e12, 40e12, 75e12}
	tankLimits := []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}

	backupMaker.SetVirtueAFX(1.0, true, false, tankFuels, tankLimits, 10, 50000)

	homeFarmType := ei.FarmType_HOME
	curiosity := ei.Egg_CURIOSITY
	backup.Farms = []*ei.Backup_Simulation{{
		EggType:  &curiosity,
		FarmType: &homeFarmType,
	}}

	// Setup active missions: 1 standard mission and 1 virtue mission
	now := float64(time.Now().Unix())
	shipAtreggies := ei.MissionInfo_ATREGGIES
	shipHen := ei.MissionInfo_HENERPRISE
	durEpic := ei.MissionInfo_EPIC
	status := ei.MissionInfo_EXPLORING
	durSec := 86400.0 * 2.0
	startTime := now - 1800
	standardType := ei.MissionInfo_STANDARD
	virtueType := ei.MissionInfo_VIRTUE

	stdMission := &ei.MissionInfo{
		Ship:             &shipAtreggies,
		DurationType:     &durEpic,
		Status:           &status,
		DurationSeconds:  &durSec,
		StartTimeDerived: &startTime,
		Type:             &standardType,
	}

	virtueMission := &ei.MissionInfo{
		Ship:             &shipHen,
		DurationType:     &durEpic,
		Status:           &status,
		DurationSeconds:  &durSec,
		StartTimeDerived: &startTime,
		Type:             &virtueType,
	}

	if backup.ArtifactsDb == nil {
		backup.ArtifactsDb = &ei.ArtifactsDB{}
	}
	backup.ArtifactsDb.MissionInfos = []*ei.MissionInfo{stdMission, virtueMission}

	farmerstate.SetMiscSettingString(dummyUser, "launch_planner_goal", GoalArtifactHunt)
	farmerstate.SetMiscSettingString(dummyUser, "launch_planner_target", "9")

	msg := buildPlannerDialog(dummyUser, backup, "")
	container := msg.Components[0].(dc.Container)

	foundVirtueNotice := false
	foundActiveSec := false
	for _, sub := range container.Components {
		if textDisp, ok := sub.(dc.TextDisplay); ok {
			content := textDisp.Content
			if strings.Contains(content, "Returning Virtue Rockets") {
				foundActiveSec = true
				if !strings.Contains(content, "Henerprise") {
					t.Error("Expected Virtue Henerprise to be displayed in active missions")
				}
				if strings.Contains(content, "Atreggies Henliner") {
					t.Error("Non-virtue rocket should NOT be displayed when on a Virtue egg")
				}
			}
			if strings.Contains(content, "Path of Virtue Fuel Planning") {
				foundVirtueNotice = true
				if !strings.Contains(content, "Curiosity") {
					t.Error("Expected notice indicating farmer is on Egg of Curiosity")
				}
				if !strings.Contains(content, "occur on this egg") {
					t.Error("Expected notice that launches cannot happen on Curiosity")
				}
				if !strings.Contains(content, "Humility ignored") {
					t.Error("Expected note that Humility fuel is ignored")
				}
				if !strings.Contains(content, "Calculated Launch Capacity") {
					t.Error("Expected Calculated Launch Capacity section in virtue planning")
				}
				if !strings.Contains(content, "Eggs to Visit") {
					t.Error("Expected itinerary of eggs to visit")
				}
				if !strings.Contains(content, "Current Farm (Curiosity)") {
					t.Error("Expected current farm action for Curiosity")
				}
				if !strings.Contains(content, "Tank Waste & Cleanup Recommendations") {
					t.Error("Expected tank waste recommendations for Integrity and Humility")
				}
				if !strings.Contains(content, "Integrity") || !strings.Contains(content, "Humility") {
					t.Error("Expected Integrity and Humility to be identified as useless tank fuel")
				}
			}
		}
	}

	if !foundActiveSec {
		t.Error("Expected returning virtue rockets section")
	}
	if !foundVirtueNotice {
		t.Error("Expected virtue fuel planning section")
	}

	// Case 2: On Humility egg
	humility := ei.Egg_HUMILITY
	backup.Farms = []*ei.Backup_Simulation{{
		EggType:  &humility,
		FarmType: &homeFarmType,
	}}

	msgHumility := buildPlannerDialog(dummyUser, backup, "")
	containerHumility := msgHumility.Components[0].(dc.Container)

	foundHumilityActive := false
	for _, sub := range containerHumility.Components {
		if textDisp, ok := sub.(dc.TextDisplay); ok {
			if strings.Contains(textDisp.Content, "Humility") && strings.Contains(textDisp.Content, "Rocket launches are") {
				foundHumilityActive = true
			}
		}
	}

	if !foundHumilityActive {
		t.Error("Expected notice that rocket launches are active on Humility egg")
	}

	// Case 3: Test GoalFuelEfficiency ("Most Artifacts / Used Fuel")
	farmerstate.SetMiscSettingString(dummyUser, "launch_planner_goal", GoalFuelEfficiency)
	msgEff := buildPlannerDialog(dummyUser, backup, "")
	containerEff := msgEff.Components[0].(dc.Container)

	foundEffGoal := false
	for _, sub := range containerEff.Components {
		if textDisp, ok := sub.(dc.TextDisplay); ok {
			content := textDisp.Content
			if strings.Contains(content, "Extended Henerprise") && strings.Contains(content, "Most Artifacts / Used Fuel") {
				foundEffGoal = true
				if !strings.Contains(content, "2.35x less tank fuel") {
					t.Error("Expected fuel efficiency tip in virtue planning section")
				}
			}
		}
	}

	if !foundEffGoal {
		t.Error("Expected GoalFuelEfficiency to target Extended Henerprise with fuel efficiency explanation")
	}
}

func TestEnlightenmentRunAdvisor(t *testing.T) {
	dummyEI := "EI1234567890123456"
	dummyUser := "userA"
	dummyName := "EnlightenmentFarmer"

	backupMaker := ei.NewBackupMaker(dummyEI, dummyName)
	backup := backupMaker.GetBackup()

	homeFarmType := ei.FarmType_HOME
	enl := ei.Egg_ENLIGHTENMENT
	backup.Farms = []*ei.Backup_Simulation{{
		EggType:  &enl,
		FarmType: &homeFarmType,
	}}

	farmerstate.SetMiscSettingString(dummyUser, "launch_planner_goal", GoalEnlightenment)

	msg := buildPlannerDialog(dummyUser, backup, "")
	container := msg.Components[0].(dc.Container)

	foundEnlightenment := false
	for _, sub := range container.Components {
		if textDisp, ok := sub.(dc.TextDisplay); ok {
			if strings.Contains(textDisp.Content, "Maximize Enlightenment Egg Run") {
				foundEnlightenment = true
				if !strings.Contains(textDisp.Content, "Optimal Tank Fill Distribution") {
					t.Error("Expected optimal tank fill distribution in Enlightenment advisor")
				}
			}
		}
	}

	if !foundEnlightenment {
		t.Error("Expected enlightenment run advisor section")
	}
}

func TestAllStarsClubShipSelection(t *testing.T) {
	dummyEI := "EI1234567890123456"
	dummyName := "ASCUser"

	backupMaker := ei.NewBackupMaker(dummyEI, dummyName)
	backup := backupMaker.GetBackup()

	// Add mission history
	now := float64(time.Now().Unix())
	ship := ei.MissionInfo_ATREGGIES
	dur := ei.MissionInfo_SHORT
	status := ei.MissionInfo_COMPLETE
	durSec := 1000.0
	startTime := now - 5000

	mi := &ei.MissionInfo{
		Ship:             &ship,
		DurationType:     &dur,
		Status:           &status,
		DurationSeconds:  &durSec,
		StartTimeDerived: &startTime,
	}

	if backup.ArtifactsDb == nil {
		backup.ArtifactsDb = &ei.ArtifactsDB{}
	}
	backup.ArtifactsDb.MissionArchive = []*ei.MissionInfo{mi}

	shipName, selectedShip, selectedDur := findNextAllStarsShip(backup, false)
	if selectedDur != ei.MissionInfo_SHORT {
		t.Errorf("Expected Short duration for fastest LP, got %v", selectedDur)
	}
	if !strings.Contains(shipName, "Short") {
		t.Errorf("Expected shipName to include Short, got %s", shipName)
	}
	_ = selectedShip
}

func TestSundayEventTiming(t *testing.T) {
	start, end := getNextSundayEvent()
	if end.Sub(start) != 48*time.Hour {
		t.Errorf("Expected double capacity event to last 48 hours, got %v", end.Sub(start))
	}
	if start.Weekday() != time.Sunday {
		t.Errorf("Expected start to be on Sunday, got %v", start.Weekday())
	}
}

func TestFuelTankRoundingWarning_Virtue(t *testing.T) {
	dummyEI := "EI1234567890123456"
	dummyUser := "userA"
	dummyName := "RoundingVirtueFarmer"

	backupMaker := ei.NewBackupMaker(dummyEI, dummyName)
	backup := backupMaker.GetBackup()

	// 500T tank: for 3 Extended Henliners, Kindness needed is exactly 225.0T (3 * 75T)
	// Set Kindness to exactly 225.0T with 0 buffer
	tankFuels := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 50e12, 0, 0, 40e12, 225e12}
	tankLimits := []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	backupMaker.SetVirtueAFX(1.0, true, false, tankFuels, tankLimits, 10, 50000)

	homeFarmType := ei.FarmType_HOME
	kindness := ei.Egg_KINDNESS
	backup.Farms = []*ei.Backup_Simulation{{
		EggType:  &kindness,
		FarmType: &homeFarmType,
	}}

	farmerstate.SetMiscSettingString(dummyUser, "launch_planner_goal", GoalArtifactHunt)
	msg := buildPlannerDialog(dummyUser, backup, "")
	container := msg.Components[0].(dc.Container)

	foundRoundingWarning := false
	foundRoundingRiskTag := false
	for _, sub := range container.Components {
		if textDisp, ok := sub.(dc.TextDisplay); ok {
			content := textDisp.Content
			if strings.Contains(content, "Fuel Tank Rounding Warning") {
				foundRoundingWarning = true
			}
			if strings.Contains(content, "Exact! (Rounding Risk)") {
				foundRoundingRiskTag = true
			}
		}
	}

	if !foundRoundingWarning {
		t.Error("Expected Fuel Tank Rounding Warning when egg fuel is exactly at the required amount")
	}
	if !foundRoundingRiskTag {
		t.Error("Expected 'Exact! (Rounding Risk)' tag in fuel row when exactly at required amount")
	}
}

func TestFuelTankRoundingWarning_Normal(t *testing.T) {
	dummyEI := "EI1234567890123456"
	dummyUser := "userA"
	dummyName := "RoundingNormalFarmer"

	backupMaker := ei.NewBackupMaker(dummyEI, dummyName)
	backup := backupMaker.GetBackup()

	homeFarmType := ei.FarmType_HOME
	eggType := ei.Egg_DARK_MATTER
	backup.Farms = []*ei.Backup_Simulation{{
		EggType:  &eggType,
		FarmType: &homeFarmType,
	}}

	// Setup tank fuels where Dilithium is exactly at cutoff for 3 Extended Henliners (3 * 6T = 18T)
	tankFuels := make([]float64, 25)
	tankFuels[8] = 20e12  // 20T Tachyon (needed 6T for 3 ships, plenty of buffer)
	tankFuels[10] = 18e12 // 18T Dilithium (exactly 3 * 6T = 18T, exact cutoff)
	tankFuels[13] = 30e12 // 30T Antimatter
	tankFuels[14] = 30e12 // 30T Dark Matter
	if backup.Artifacts != nil {
		backup.Artifacts.TankFuels = tankFuels
		lvl := uint32(7)
		backup.Artifacts.TankLevel = &lvl
	}

	farmerstate.SetMiscSettingString(dummyUser, "launch_planner_goal", GoalArtifactHunt)
	msg := buildPlannerDialog(dummyUser, backup, "")
	container := msg.Components[0].(dc.Container)

	foundRoundingWarning := false
	for _, sub := range container.Components {
		if textDisp, ok := sub.(dc.TextDisplay); ok {
			content := textDisp.Content
			if strings.Contains(content, "Fuel Tank Rounding Warning") && strings.Contains(content, "Dilithium") {
				foundRoundingWarning = true
			}
		}
	}

	if !foundRoundingWarning {
		t.Error("Expected Fuel Tank Rounding Warning for Dilithium when exactly at 3-launch cutoff on normal farm")
	}
}

func TestBuildPlannerDialog_TargetArtifactEmoji(t *testing.T) {
	dummyEI := "EI1234567890123456"
	dummyUser := "userA"
	dummyName := "TestFarmer"

	backupMaker := ei.NewBackupMaker(dummyEI, dummyName)
	backup := backupMaker.GetBackup()

	homeFarmType := ei.FarmType_HOME
	eggType := ei.Egg_DARK_MATTER
	backup.Farms = []*ei.Backup_Simulation{{
		EggType:  &eggType,
		FarmType: &homeFarmType,
	}}

	// Populate mock EmoteMap
	if ei.EmoteMap == nil {
		ei.EmoteMap = make(map[string]ei.Emotes)
	}
	ei.EmoteMap["chalice_t4c"] = ei.Emotes{Name: "chalice_t4c", ID: "11223344"}
	ei.EmoteMap["st_t4"] = ei.Emotes{Name: "st_t4", ID: "55667788"}

	ship := ei.MissionInfo_ATREGGIES
	dur := ei.MissionInfo_EPIC
	status := ei.MissionInfo_EXPLORING
	level := uint32(8)
	durSec := 86400.0 * 2.5
	startTime := float64(time.Now().Unix()) - 3600
	chaliceTarget := ei.ArtifactSpec_THE_CHALICE

	mission := &ei.MissionInfo{
		Ship:             &ship,
		DurationType:     &dur,
		Status:           &status,
		Level:            &level,
		DurationSeconds:  &durSec,
		StartTimeDerived: &startTime,
		TargetArtifact:   &chaliceTarget,
	}

	backup.ArtifactsDb = &ei.ArtifactsDB{
		MissionInfos: []*ei.MissionInfo{mission},
	}

	msg := buildPlannerDialog(dummyUser, backup, "")
	container := msg.Components[0].(dc.Container)

	foundTargetWithEmoji := false
	for _, sub := range container.Components {
		if textDisp, ok := sub.(dc.TextDisplay); ok {
			content := textDisp.Content
			if strings.Contains(content, "Returning Rockets & Ship Quality") {
				if strings.Contains(content, "<:chalice_t4c:11223344>") && !strings.Contains(content, "The Chalice") {
					foundTargetWithEmoji = true
				}
			}
		}
	}

	if !foundTargetWithEmoji {
		t.Error("Expected active rocket section to show T4C Chalice emoji <:chalice_t4c:11223344> without the artifact name")
	}

	// Test stone target
	stoneTarget := ei.ArtifactSpec_TACHYON_STONE
	mission.TargetArtifact = &stoneTarget

	msgStone := buildPlannerDialog(dummyUser, backup, "")
	containerStone := msgStone.Components[0].(dc.Container)

	foundStoneWithEmoji := false
	for _, sub := range containerStone.Components {
		if textDisp, ok := sub.(dc.TextDisplay); ok {
			content := textDisp.Content
			if strings.Contains(content, "Returning Rockets & Ship Quality") {
				if strings.Contains(content, "<:st_t4:55667788>") && !strings.Contains(content, "Tachyon Stone") {
					foundStoneWithEmoji = true
				}
			}
		}
	}

	if !foundStoneWithEmoji {
		t.Error("Expected active rocket section to show T4 Tachyon Stone emoji <:st_t4:55667788> without the stone name")
	}

	// Test ingredient target (Tau Ceti Geode)
	ei.EmoteMap["afx_tau_ceti_geode_3"] = ei.Emotes{Name: "afx_tau_ceti_geode_3", ID: "33445566"}
	geodeTarget := ei.ArtifactSpec_TAU_CETI_GEODE
	mission.TargetArtifact = &geodeTarget

	msgGeode := buildPlannerDialog(dummyUser, backup, "")
	containerGeode := msgGeode.Components[0].(dc.Container)

	foundGeodeWithEmoji := false
	for _, sub := range containerGeode.Components {
		if textDisp, ok := sub.(dc.TextDisplay); ok {
			content := textDisp.Content
			if strings.Contains(content, "Returning Rockets & Ship Quality") {
				if strings.Contains(content, "<:afx_tau_ceti_geode_3:33445566>") && !strings.Contains(content, "Tau Ceti Geode") {
					foundGeodeWithEmoji = true
				}
			}
		}
	}

	if !foundGeodeWithEmoji {
		t.Error("Expected active rocket section to show Tau Ceti Geode emoji <:afx_tau_ceti_geode_3:33445566> without the ingredient name")
	}
}
