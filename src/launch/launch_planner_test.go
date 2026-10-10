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
	hasExactFuel := false

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
			if o.Name == "exact-fuel" {
				hasExactFuel = true
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
	if hasExactFuel {
		t.Error("Unexpected 'exact-fuel' option; exact fuel is toggled via sticky button only")
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

	foundDubcapOnBtn := false
	foundDubcapMenu := false
	for _, sub := range container.Components {
		if row, ok := sub.(dc.ActionRow); ok {
			for _, comp := range row.Components {
				if btn, ok := comp.(dc.Button); ok && btn.Label == "Dubcap: On" {
					foundDubcapOnBtn = true
				}
				if sm, ok := comp.(dc.SelectMenu); ok && strings.Contains(sm.CustomID, "dubcap") {
					foundDubcapMenu = true
				}
			}
		}
	}
	if !foundDubcapOnBtn {
		t.Error("Expected 'Dubcap: On' button when dubcap is enabled")
	}
	if foundDubcapMenu {
		t.Error("Did not expect dubcap SelectMenu; it should be a toggle button")
	}

	// Toggle dubcap off
	farmerstate.SetMiscSettingFlag(dummyUser, "launch_planner_dubcap", false)
	msgOff := buildPlannerDialog(dummyUser, backup, "")
	containerOff := msgOff.Components[0].(dc.Container)
	foundDubcapOffBtn := false
	for _, sub := range containerOff.Components {
		if row, ok := sub.(dc.ActionRow); ok {
			for _, comp := range row.Components {
				if btn, ok := comp.(dc.Button); ok && btn.Label == "Dubcap: Off" {
					foundDubcapOffBtn = true
				}
			}
		}
	}
	if !foundDubcapOffBtn {
		t.Error("Expected 'Dubcap: Off' button when dubcap is disabled")
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

	shipName, selectedShip, selectedDur := findNextAllStarsShip(backup, nil)
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

func TestExactFuelOptionAndFormatting(t *testing.T) {
	dummyEI := "EI1234567890123456"
	dummyUser := "userExactFuel"
	dummyName := "TestFarmer"

	backupMaker := ei.NewBackupMaker(dummyEI, dummyName)
	backup := backupMaker.GetBackup()

	// Setup virtue farm on Kindness
	homeFarmType := ei.FarmType_HOME
	eggType := ei.Egg_KINDNESS
	backup.Farms = []*ei.Backup_Simulation{{
		EggType:  &eggType,
		FarmType: &homeFarmType,
	}}

	// Setup Virtue AFX tank fuels (last 5 are virtue eggs)
	// Curiosity (20), Integrity (21), Humility (22), Resilience (23), Kindness (24)
	tankFuels := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 265420000000, 0, 0, 313560000000.5, 225100000000000.12}
	tankLimits := []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}

	backupMaker.SetVirtueAFX(1.0, true, false, tankFuels, tankLimits, 10, 50000)

	lvl := uint32(7) // 500T tank
	if backup.Artifacts != nil {
		backup.Artifacts.TankLevel = &lvl
	}

	farmerstate.SetMiscSettingString(dummyUser, "launch_planner_goal", GoalArtifactHunt)

	// Case 1: exactFuel = false (default)
	farmerstate.SetMiscSettingFlag(dummyUser, "launch_planner_exact_fuel", false)
	msgDefault := buildPlannerDialog(dummyUser, backup, "")
	containerDefault := msgDefault.Components[0].(dc.Container)

	fuelContentDefault := ""
	for _, sub := range containerDefault.Components {
		if textDisp, ok := sub.(dc.TextDisplay); ok {
			if strings.Contains(textDisp.Content, "Path of Virtue Fuel Planning") {
				fuelContentDefault = textDisp.Content
			}
		}
	}
	// With exactFuel = false, formats as standard 1-decimal EI units: 265.4B, 225.1T
	if !strings.Contains(fuelContentDefault, "265.4B") || !strings.Contains(fuelContentDefault, "225.1T") {
		t.Errorf("Expected 1 decimal formatting when exact-fuel is off, got:\n%s", fuelContentDefault)
	}

	// Verify button label
	foundExactOffBtn := false
	for _, sub := range containerDefault.Components {
		if row, ok := sub.(dc.ActionRow); ok {
			for _, comp := range row.Components {
				if btn, ok := comp.(dc.Button); ok {
					if btn.Label == "Exact Fuel: Off" {
						foundExactOffBtn = true
					}
				}
			}
		}
	}
	if !foundExactOffBtn {
		t.Error("Expected to find 'Exact Fuel: Off' button")
	}

	// Case 2: exactFuel = true
	farmerstate.SetMiscSettingFlag(dummyUser, "launch_planner_exact_fuel", true)
	msgExact := buildPlannerDialog(dummyUser, backup, "")
	containerExact := msgExact.Components[0].(dc.Container)

	fuelContentExact := ""
	for _, sub := range containerExact.Components {
		if textDisp, ok := sub.(dc.TextDisplay); ok {
			if strings.Contains(textDisp.Content, "Path of Virtue Fuel Planning") {
				fuelContentExact = textDisp.Content
			}
		}
	}
	// With exactFuel = true, should show all digits with commas up to 2 decimal places:
	// e.g. 225,100,000,000,000.12, 265,420,000,000, 313,560,000,000.5
	if !strings.Contains(fuelContentExact, "225,100,000,000,000.12") {
		t.Errorf("Expected '225,100,000,000,000.12' when exact-fuel is on, got:\n%s", fuelContentExact)
	}
	if !strings.Contains(fuelContentExact, "265,420,000,000") {
		t.Errorf("Expected '265,420,000,000' when exact-fuel is on, got:\n%s", fuelContentExact)
	}
	if !strings.Contains(fuelContentExact, "313,560,000,000.5") {
		t.Errorf("Expected '313,560,000,000.5' when exact-fuel is on, got:\n%s", fuelContentExact)
	}
	// Total needed should remain standard (e.g. 225T, not 225,000,000,000,000)
	if !strings.Contains(fuelContentExact, "/ 225T") {
		t.Errorf("Expected total needed to remain standard '/ 225T', got:\n%s", fuelContentExact)
	}

	// Verify button label is now "Exact Fuel: On"
	foundExactOnBtn := false
	for _, sub := range containerExact.Components {
		if row, ok := sub.(dc.ActionRow); ok {
			for _, comp := range row.Components {
				if btn, ok := comp.(dc.Button); ok {
					if btn.Label == "Exact Fuel: On" {
						foundExactOnBtn = true
					}
				}
			}
		}
	}
	if !foundExactOnBtn {
		t.Error("Expected to find 'Exact Fuel: On' button")
	}
}

func TestVirtueShipPoolSelection(t *testing.T) {
	dummyEI := "EI1234567890123456"
	dummyUser := "userPool"
	dummyName := "TestFarmerPool"

	backupMaker := ei.NewBackupMaker(dummyEI, dummyName)
	backup := backupMaker.GetBackup()

	// Setup virtue farm on Kindness
	homeFarmType := ei.FarmType_HOME
	eggType := ei.Egg_KINDNESS
	backup.Farms = []*ei.Backup_Simulation{{
		EggType:  &eggType,
		FarmType: &homeFarmType,
	}}

	// Setup Virtue AFX tank fuels (last 5 are virtue eggs)
	tankFuels := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 50e12, 0, 0, 40e12, 50e12}
	tankLimits := []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}

	backupMaker.SetVirtueAFX(1.0, true, false, tankFuels, tankLimits, 10, 50000)

	lvl := uint32(7) // 500T tank
	if backup.Artifacts != nil {
		backup.Artifacts.TankLevel = &lvl
	}

	// Case 1: By default, menu is closed; toggle button exists with "Select Ships"
	msgDefault := buildPlannerDialog(dummyUser, backup, "")
	containerDefault := msgDefault.Components[0].(dc.Container)

	var foundShipMenu *dc.SelectMenu
	var foundToggleBtn *dc.Button
	for _, sub := range containerDefault.Components {
		if row, ok := sub.(dc.ActionRow); ok {
			for _, comp := range row.Components {
				if sm, ok := comp.(dc.SelectMenu); ok && strings.Contains(sm.CustomID, "virtue_ships") {
					foundShipMenu = &sm
				}
				if btn, ok := comp.(dc.Button); ok && strings.Contains(btn.CustomID, "toggle_virtue_ships") {
					foundToggleBtn = &btn
				}
			}
		}
	}

	if foundShipMenu != nil {
		t.Error("Expected virtue_ships menu to be hidden by default")
	}
	if foundToggleBtn == nil {
		t.Fatal("Expected to find toggle_virtue_ships button in ActionRow")
	}
	if foundToggleBtn.Label != "Select Ships" {
		t.Errorf("Expected button label 'Select Ships', got '%s'", foundToggleBtn.Label)
	}

	fuelContentDefault := ""
	for _, sub := range containerDefault.Components {
		if textDisp, ok := sub.(dc.TextDisplay); ok {
			if strings.Contains(textDisp.Content, "Path of Virtue Fuel Planning") {
				fuelContentDefault = textDisp.Content
			}
		}
	}
	if !strings.Contains(fuelContentDefault, "**Ship Pool**: All") {
		t.Errorf("Expected fuel content to list '**Ship Pool**: All' when all ships available, got:\n%s", fuelContentDefault)
	}

	// Case 1b: Toggle menu OPEN
	farmerstate.SetMiscSettingFlag(dummyUser, "launch_planner_show_virtue_ships", true)
	msgOpen := buildPlannerDialog(dummyUser, backup, "")
	containerOpen := msgOpen.Components[0].(dc.Container)

	foundShipMenu = nil
	foundToggleBtn = nil
	for _, sub := range containerOpen.Components {
		if row, ok := sub.(dc.ActionRow); ok {
			for _, comp := range row.Components {
				if sm, ok := comp.(dc.SelectMenu); ok && strings.Contains(sm.CustomID, "virtue_ships") {
					foundShipMenu = &sm
				}
				if btn, ok := comp.(dc.Button); ok && strings.Contains(btn.CustomID, "toggle_virtue_ships") {
					foundToggleBtn = &btn
				}
			}
		}
	}

	if foundShipMenu == nil {
		t.Fatal("Expected to find virtue_ships multi-select menu when toggled open")
	}
	if foundShipMenu.MinValues == nil || *foundShipMenu.MinValues != 1 {
		t.Errorf("Expected MinValues to be 1, got %v", foundShipMenu.MinValues)
	}
	if foundShipMenu.MaxValues < 11 {
		t.Errorf("Expected MaxValues to be at least 11, got %d", foundShipMenu.MaxValues)
	}
	if foundToggleBtn == nil || foundToggleBtn.Label != "Hide Ships" {
		t.Errorf("Expected button label 'Hide Ships' when open, got %v", foundToggleBtn)
	}

	// Close menu for subsequent tests
	farmerstate.SetMiscSettingFlag(dummyUser, "launch_planner_show_virtue_ships", false)

	// Case 2: Custom pool selection [Defihent (7), Voyegger (8)]
	// Artifact Hunt should select best ship in pool: Voyegger
	farmerstate.SetMiscSettingString(dummyUser, "launch_planner_virtue_ships", "7,8")
	farmerstate.SetMiscSettingString(dummyUser, "launch_planner_goal", GoalArtifactHunt)

	msgCustom := buildPlannerDialog(dummyUser, backup, "")
	containerCustom := msgCustom.Components[0].(dc.Container)

	fuelContent := ""
	for _, sub := range containerCustom.Components {
		if textDisp, ok := sub.(dc.TextDisplay); ok {
			if strings.Contains(textDisp.Content, "Path of Virtue Fuel Planning") {
				fuelContent = textDisp.Content
			}
		}
	}

	if !strings.Contains(fuelContent, "Extended Voyegger") {
		t.Errorf("Expected Artifact Hunt with pool [7,8] to target Extended Voyegger, got:\n%s", fuelContent)
	}
	if !strings.Contains(fuelContent, "**Ship Pool**: Defihent, Voyegger") {
		t.Errorf("Expected fuel content to list selected Ship Pool, got:\n%s", fuelContent)
	}

	// Case 3: Fuel Efficiency with pool [Defihent (7), Voyegger (8)]
	// Should select second-best / lower fuel ship: Defihent
	farmerstate.SetMiscSettingString(dummyUser, "launch_planner_goal", GoalFuelEfficiency)
	msgEff := buildPlannerDialog(dummyUser, backup, "")
	containerEff := msgEff.Components[0].(dc.Container)

	fuelContentEff := ""
	for _, sub := range containerEff.Components {
		if textDisp, ok := sub.(dc.TextDisplay); ok {
			if strings.Contains(textDisp.Content, "Path of Virtue Fuel Planning") {
				fuelContentEff = textDisp.Content
			}
		}
	}

	if !strings.Contains(fuelContentEff, "Extended Defihent") {
		t.Errorf("Expected Fuel Efficiency with pool [7,8] to target Extended Defihent, got:\n%s", fuelContentEff)
	}

	// Case 4: ASC with pool [Defihent (7), Voyegger (8)]
	// Should select Short of one of the ships in pool
	farmerstate.SetMiscSettingString(dummyUser, "launch_planner_goal", GoalAllStars)
	msgASC := buildPlannerDialog(dummyUser, backup, "")
	containerASC := msgASC.Components[0].(dc.Container)

	fuelContentASC := ""
	for _, sub := range containerASC.Components {
		if textDisp, ok := sub.(dc.TextDisplay); ok {
			if strings.Contains(textDisp.Content, "Path of Virtue Fuel Planning") {
				fuelContentASC = textDisp.Content
			}
		}
	}

	if !strings.Contains(fuelContentASC, "Short Defihent") && !strings.Contains(fuelContentASC, "Short Voyegger") {
		t.Errorf("Expected ASC with pool [7,8] to target a ship in the pool, got:\n%s", fuelContentASC)
	}
}

func TestAllStarsClubOptionOmittedWhenNotNeeded(t *testing.T) {
	dummyEI := "EI1234567890123456"
	dummyUser := "userMaxStars"
	dummyName := "MaxStarsFarmer"

	backupMaker := ei.NewBackupMaker(dummyEI, dummyName)
	backup := backupMaker.GetBackup()

	// Max out all 11 ships in MissionArchive
	var missions []*ei.MissionInfo
	dur := ei.MissionInfo_SHORT
	status := ei.MissionInfo_ARCHIVED
	for s := 0; s < 11; s++ {
		ship := ei.MissionInfo_Spaceship(s)
		numMissions := int(ei.ShipMaxLaunchPoints[s]) + 1
		for i := 0; i < numMissions; i++ {
			missions = append(missions, &ei.MissionInfo{
				Ship:         &ship,
				DurationType: &dur,
				Status:       &status,
			})
		}
	}
	if backup.ArtifactsDb == nil {
		backup.ArtifactsDb = &ei.ArtifactsDB{}
	}
	backup.ArtifactsDb.MissionArchive = missions

	// Test Normal Farm: should NOT show All Stars Club in goalMenu options
	eggNormal := ei.Egg_DARK_MATTER
	homeFarmType := ei.FarmType_HOME
	backup.Farms = []*ei.Backup_Simulation{{
		EggType:  &eggNormal,
		FarmType: &homeFarmType,
	}}
	msgNormal := buildPlannerDialog(dummyUser, backup, "")
	containerNormal := msgNormal.Components[0].(dc.Container)

	for _, sub := range containerNormal.Components {
		if row, ok := sub.(dc.ActionRow); ok {
			for _, comp := range row.Components {
				if sm, ok := comp.(dc.SelectMenu); ok && strings.Contains(sm.CustomID, "launch_planner#goal#") {
					for _, opt := range sm.Options {
						if opt.Value == GoalAllStars {
							t.Error("Did not expect 'All Stars Club' option when all ships are maxed on Normal farm")
						}
					}
				}
			}
		}
	}

	// Test Virtue Farm: should NOT show All Stars Club in goalMenu options
	curiosity := ei.Egg_CURIOSITY
	backup.Farms = []*ei.Backup_Simulation{{
		EggType:  &curiosity,
		FarmType: &homeFarmType,
	}}
	tankFuels := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 50e12, 10e12, 5e12, 40e12, 75e12}
	tankLimits := []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	backupMaker.SetVirtueAFX(1.0, true, false, tankFuels, tankLimits, 10, 50000)

	msgVirtue := buildPlannerDialog(dummyUser, backup, "")
	containerVirtue := msgVirtue.Components[0].(dc.Container)

	for _, sub := range containerVirtue.Components {
		if row, ok := sub.(dc.ActionRow); ok {
			for _, comp := range row.Components {
				if sm, ok := comp.(dc.SelectMenu); ok && strings.Contains(sm.CustomID, "launch_planner#goal#") {
					for _, opt := range sm.Options {
						if opt.Value == GoalAllStars {
							t.Error("Did not expect 'All Stars Club' option when all ships are maxed on Virtue farm")
						}
					}
				}
			}
		}
	}
}

func TestSundayDubcapAdvisorWiggleRoom(t *testing.T) {
	if ei.EggIncResearchesMap == nil {
		ei.EggIncResearchesMap = make(map[string]ei.EggResearches)
	}
	ei.EggIncResearchesMap["afx_mission_time"] = ei.EggResearches{
		ID:             "afx_mission_time",
		EffectType:     "multiplicative",
		PerLevel:       -0.01,
		LevelsCompound: "additive",
	}

	backupMaker := ei.NewBackupMaker("EI1234567890123456", "testUser")
	backup := backupMaker.GetBackup()

	var b strings.Builder
	renderSundayDubcapAdvisor(&b, nil, backup, ei.Egg_UNKNOWN, nil)
	out := b.String()

	if !strings.Contains(out, "Double ExLiner window") {
		t.Errorf("Expected Dubcap advisor to include Double ExLiner window, got:\n%s", out)
	}
	if !strings.Contains(out, "wiggle room after start") {
		t.Errorf("Expected Dubcap advisor to include wiggle room, got:\n%s", out)
	}

	// Test with active mission landing during dubcap
	dubcapStart, dubcapEnd := getNextSundayEvent()
	// Mission lands 2 hours after Dubcap starts
	missionReturn := dubcapStart.Add(2 * time.Hour)
	durSec := float64(138240)
	startSec := float64(missionReturn.Unix()) - durSec
	ship := ei.MissionInfo_ATREGGIES
	dur := ei.MissionInfo_EPIC
	activeM := []*ei.MissionInfo{
		{
			Ship:             &ship,
			DurationType:     &dur,
			StartTimeDerived: &startSec,
			DurationSeconds:  &durSec,
		},
	}

	b.Reset()
	renderSundayDubcapAdvisor(&b, activeM, backup, ei.Egg_UNKNOWN, nil)
	outActive := b.String()

	if dubcapEnd.Sub(missionReturn) > 38*time.Hour+24*time.Minute {
		if !strings.Contains(outActive, "Amazing timing") {
			t.Errorf("Expected 'Amazing timing' recommendation for mission landing early in Dubcap, got:\n%s", outActive)
		}
		if !strings.Contains(outActive, "buffer before cutoff") {
			t.Errorf("Expected 'buffer before cutoff' in output, got:\n%s", outActive)
		}
	}

	// Test virtue farm filler suppression:
	// Mission lands 4 hours before Dubcap starts
	landingBefore := dubcapStart.Add(-4 * time.Hour)
	startSecBefore := float64(landingBefore.Unix()) - durSec
	shipChicken := ei.MissionInfo_CHICKEN_ONE
	durShort := ei.MissionInfo_SHORT
	durShortSec := float64(1200)
	activeBefore := []*ei.MissionInfo{
		{
			Ship:             &shipChicken,
			DurationType:     &durShort,
			StartTimeDerived: &startSecBefore,
			DurationSeconds:  &durShortSec,
		},
	}

	// Case A: On Curiosity farm (Virtue, not Humility) -> No filler recommendation
	b.Reset()
	renderSundayDubcapAdvisor(&b, activeBefore, backup, ei.Egg_CURIOSITY, nil)
	outCuriosity := b.String()
	if strings.Contains(outCuriosity, "filler mission next") {
		t.Errorf("Did not expect filler mission recommendation when on non-Humility virtue farm, got:\n%s", outCuriosity)
	}
	if !strings.Contains(outCuriosity, "shift to Humility for Dubcap") {
		t.Errorf("Expected 'shift to Humility for Dubcap' advice, got:\n%s", outCuriosity)
	}

	// Case B: On Humility farm (Virtue, is Humility) -> Filler recommendation is allowed
	b.Reset()
	renderSundayDubcapAdvisor(&b, activeBefore, backup, ei.Egg_HUMILITY, nil)
	outHumility := b.String()
	if !strings.Contains(outHumility, "filler mission next") {
		t.Errorf("Expected filler mission recommendation when already on Humility farm, got:\n%s", outHumility)
	}

	// Test Case C: Rocket already returned in past (e.g. 2 months ago)
	pastReturn := time.Now().Add(-60 * 24 * time.Hour)
	pastStartSec := float64(pastReturn.Unix()) - durSec
	activeLanded := []*ei.MissionInfo{
		{
			Ship:             &ship,
			DurationType:     &dur,
			StartTimeDerived: &pastStartSec,
			DurationSeconds:  &durSec,
		},
	}
	// Zero out tank fuels to test missing fuel advice
	backup.Virtue.Afx.TankFuels = make([]float64, 25)
	b.Reset()
	renderSundayDubcapAdvisor(&b, activeLanded, backup, ei.Egg_CURIOSITY, nil)
	outLanded := b.String()
	if !strings.Contains(outLanded, "🟢 **Landed & Ready to Collect!**") {
		t.Errorf("Expected landed rocket status for past return time, got:\n%s", outLanded)
	}
	if strings.Contains(outLanded, "Lands ") && strings.Contains(outLanded, "before Dubcap") {
		t.Errorf("Did not expect 'Lands ... before Dubcap' for already landed mission, got:\n%s", outLanded)
	}
	// Missing fuel check: backup has 0 virtue fuels, so it must tell user to bank fuel
	if !strings.Contains(outLanded, "Bank required fuel") {
		t.Errorf("Expected 'Bank required fuel' recommendation when virtue fuel is missing, got:\n%s", outLanded)
	}

	// Test Case D: Rocket already returned with sufficient virtue tank fuel
	// Add virtue fuel to backup
	vAfx := &ei.Backup_Artifacts{
		TankLevel: new(uint32),
		// Curiosity, Integrity, Humility, Resilience, Kindness
		TankFuels: []float64{100e12, 0, 0, 100e12, 100e12},
	}
	*vAfx.TankLevel = 5
	backup.Virtue = &ei.Backup_Virtue{
		Afx: vAfx,
	}
	b.Reset()
	renderSundayDubcapAdvisor(&b, activeLanded, backup, ei.Egg_CURIOSITY, nil)
	outFueled := b.String()
	if !strings.Contains(outFueled, "Tank fuel ready!") {
		t.Errorf("Expected 'Tank fuel ready!' recommendation when sufficient virtue fuel is banked, got:\n%s", outFueled)
	}
}
