package boost

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

func TestParseCsEstimateParams_StickySrMode(t *testing.T) {
	testUserID := "999000111222333444"

	makePayload := func(optionsJSON string) string {
		return fmt.Sprintf(`{
			"id": "123456789012345678",
			"application_id": "123456789012345678",
			"type": 2,
			"token": "test-token",
			"version": 1,
			"channel": {"id": "123456789012345678", "type": 0},
			"guild_id": "123456789012345678",
			"member": {
				"user": {"id": "%s", "username": "testuser", "discriminator": "0"},
				"roles": [],
				"joined_at": "2026-01-01T00:00:00Z"
			},
			"data": {
				"id": "1",
				"name": "cs-estimate",
				"type": 1,
				"options": %s
			}
		}`, testUserID, optionsJSON)
	}

	// 1. Initial state: neither flag set in farmerstate, neither passed in options
	defaultEvent, err := dc.NewCommandEventFromPayload([]byte(makePayload(`[]`)))
	if err != nil {
		t.Fatalf("failed to create default event: %v", err)
	}
	p := parseCsEstimateParams(defaultEvent)
	if p.srMode {
		t.Errorf("expected srMode default to be false, got true")
	}

	// 2. Pass sr-mode = true
	setTrueEvent, err := dc.NewCommandEventFromPayload([]byte(makePayload(`[{"name":"sr-mode","type":5,"value":true}]`)))
	if err != nil {
		t.Fatalf("failed to create setTrue event: %v", err)
	}
	p = parseCsEstimateParams(setTrueEvent)
	if !p.srMode {
		t.Errorf("expected srMode to be true after setting it true")
	}
	if !farmerstate.GetMiscSettingFlag(testUserID, "sr-mode") {
		t.Errorf("expected farmerstate sr-mode flag to be true")
	}

	// 3. Omit sr-mode option; should remain sticky true
	omitEvent, err := dc.NewCommandEventFromPayload([]byte(makePayload(`[]`)))
	if err != nil {
		t.Fatalf("failed to create omit event: %v", err)
	}
	p = parseCsEstimateParams(omitEvent)
	if !p.srMode {
		t.Errorf("expected srMode to be sticky true when omitted")
	}

	// 4. Pass sr-mode = false
	setFalseEvent, err := dc.NewCommandEventFromPayload([]byte(makePayload(`[{"name":"sr-mode","type":5,"value":false}]`)))
	if err != nil {
		t.Fatalf("failed to create setFalse event: %v", err)
	}
	p = parseCsEstimateParams(setFalseEvent)
	if p.srMode {
		t.Errorf("expected srMode to be false after setting it false")
	}
	if farmerstate.GetMiscSettingFlag(testUserID, "sr-mode") {
		t.Errorf("expected farmerstate sr-mode flag to be false")
	}

	// 5. Omit sr-mode option again; should remain sticky false
	omitEvent2, err := dc.NewCommandEventFromPayload([]byte(makePayload(`[]`)))
	if err != nil {
		t.Fatalf("failed to create omit event 2: %v", err)
	}
	p = parseCsEstimateParams(omitEvent2)
	if p.srMode {
		t.Errorf("expected srMode to be sticky false when omitted")
	}
}

func TestGetCompletionPingMessage(t *testing.T) {
	c := &Contract{
		ContractHash: "test-cs-estimate-hash",
		Name:         "Costume Party",
		CoopID:       "party-coop",
		Location: []*LocationData{
			{
				ChannelID:   "chan-1",
				GuildID:     "guild-1",
				RoleMention: "@PartyRole",
			},
		},
		Boosters: map[string]*Booster{
			"userA": {
				UserID:  "userA",
				Nick:    "PartyLeader",
				Mention: "<@userA>",
			},
		},
	}

	// 1. Future estimated completion
	c.EstimatedEndTime = time.Now().Add(3 * time.Hour)
	futureMsg := getCompletionPingMessage(c, "userA")

	if !strings.HasPrefix(futureMsg, "The contract **Costume Party** `party-coop` will complete on") {
		t.Errorf("expected future message to start with contract completion text, got: %s", futureMsg)
	}
	expectedFooter := "-# @PartyRole Ping requested by <@userA>"
	if !strings.HasSuffix(futureMsg, expectedFooter) {
		t.Errorf("expected message to end with %q, got: %s", expectedFooter, futureMsg)
	}
	if strings.HasPrefix(futureMsg, "@PartyRole") {
		t.Errorf("expected message NOT to start with role mention, got: %s", futureMsg)
	}

	// 2. Already ended contract
	c.EstimatedEndTime = time.Now().Add(-1 * time.Hour)
	endedMsg := getCompletionPingMessage(c, "userA")

	if !strings.HasPrefix(endedMsg, "The contract **Costume Party** `party-coop` has ended! Please check-in to finalize scores.") {
		t.Errorf("expected ended message to start with contract ended text, got: %s", endedMsg)
	}
	if !strings.HasSuffix(endedMsg, expectedFooter) {
		t.Errorf("expected ended message to end with %q, got: %s", expectedFooter, endedMsg)
	}
	if strings.HasPrefix(endedMsg, "@PartyRole") {
		t.Errorf("expected ended message NOT to start with role mention, got: %s", endedMsg)
	}
}

func TestShouldShowContractSiloWatch(t *testing.T) {
	origResearches := ei.EggIncResearchesMap
	ei.EggIncResearchesMap = map[string]ei.EggResearches{
		"silo_capacity": {
			ID:             "silo_capacity",
			EffectType:     "additive",
			LevelsCompound: "additive",
			PerLevel:       6,
		},
	}
	defer func() { ei.EggIncResearchesMap = origResearches }()

	u32 := func(v uint32) *uint32 { return &v }
	f64 := func(v float64) *float64 { return &v }
	bptr := func(v bool) *bool { return &v }
	sptr := func(v string) *string { return &v }

	maxSiloCap := []*ei.Backup_ResearchItem{{Id: sptr("silo_capacity"), Level: u32(20)}}

	makeStatus := func(contributors []*ei.ContractCoopStatusResponse_ContributionInfo, secRem float64, allGoals bool) *ei.ContractCoopStatusResponse {
		return &ei.ContractCoopStatusResponse{
			Contributors:     contributors,
			SecondsRemaining: f64(secRem),
			AllGoalsAchieved: bptr(allGoals),
		}
	}

	pro10 := &ei.ContractCoopStatusResponse_ContributionInfo{
		UserName: sptr("ProFarmer"),
		FarmInfo: &ei.PlayerFarmInfo{
			PermitLevel:  u32(ei.ProPermitLevel),
			SilosOwned:   u32(10),
			EpicResearch: maxSiloCap, // 30h
		},
	}
	std2 := &ei.ContractCoopStatusResponse_ContributionInfo{
		UserName: sptr("StdFarmer"),
		FarmInfo: &ei.PlayerFarmInfo{
			PermitLevel:  u32(ei.StandardPermitLevel),
			SilosOwned:   u32(2),
			EpicResearch: maxSiloCap, // 6h
		},
	}
	pro2 := &ei.ContractCoopStatusResponse_ContributionInfo{
		UserName: sptr("ProLowSilos"),
		FarmInfo: &ei.PlayerFarmInfo{
			PermitLevel:  u32(ei.ProPermitLevel),
			SilosOwned:   u32(2),
			EpicResearch: maxSiloCap, // 6h
		},
	}

	// 1. Standard permit farmer in short contract -> true
	if !shouldShowContractSiloWatch(makeStatus([]*ei.ContractCoopStatusResponse_ContributionInfo{pro10, std2}, 3600, false), 4*3600, 86400) {
		t.Errorf("expected true when a Standard Permit farmer is in the contract")
	}

	// 2. All Pro permit with 30h silos in 12h contract -> false
	if shouldShowContractSiloWatch(makeStatus([]*ei.ContractCoopStatusResponse_ContributionInfo{pro10}, 3600, false), 12*3600, 86400) {
		t.Errorf("expected false when all Pro permit silos exceed contract duration")
	}

	// 3. All Pro permit, but contract duration (36h) exceeds 30h silos -> true
	if !shouldShowContractSiloWatch(makeStatus([]*ei.ContractCoopStatusResponse_ContributionInfo{pro10}, 3600, false), 36*3600, 86400) {
		t.Errorf("expected true when contract duration exceeds player silo away time")
	}

	// 4. Pro permit player who only purchased 2 silos (6h) in 12h contract -> true
	if !shouldShowContractSiloWatch(makeStatus([]*ei.ContractCoopStatusResponse_ContributionInfo{pro10, pro2}, 3600, false), 12*3600, 86400) {
		t.Errorf("expected true when contract duration exceeds a pro player's purchased silo time")
	}

	// 5. Completed contract -> false even with Standard Permit farmer
	if shouldShowContractSiloWatch(makeStatus([]*ei.ContractCoopStatusResponse_ContributionInfo{std2}, 3600, true), 12*3600, 86400) {
		t.Errorf("expected false for completed contract")
	}
}

func TestBuildCsEstimateActionRow(t *testing.T) {
	row, ok := buildCsEstimateActionRow("hash1", "c-id", "coop1", true)
	if !ok {
		t.Fatalf("expected action row")
	}
	ar := row.(dc.ActionRow)
	if len(ar.Components) != 3 {
		t.Fatalf("expected 3 buttons, got %d", len(ar.Components))
	}
	siloBtn := ar.Components[1].(dc.Button)
	if siloBtn.Label != "Silo Watch" || siloBtn.CustomID != "watch-csilos-toggle#c-id#coop1" {
		t.Errorf("unexpected silo watch button: %+v", siloBtn)
	}

	rowNoHash, ok := buildCsEstimateActionRow("", "c-id", "coop1", true)
	if !ok {
		t.Fatalf("expected action row with Silo Watch even without contract hash")
	}
	if len(rowNoHash.(dc.ActionRow).Components) != 1 {
		t.Fatalf("expected 1 button without hash, got %d", len(rowNoHash.(dc.ActionRow).Components))
	}

	if _, ok := buildCsEstimateActionRow("", "c-id", "coop1", false); ok {
		t.Errorf("expected no action row when no hash and showSiloWatch is false")
	}
}
