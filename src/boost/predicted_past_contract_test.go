package boost

import (
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

func TestPredictedPastContractLifecycle(t *testing.T) {
	client := newTestClient()
	pastContractID := "test-past-contract-xyz"
	guildID := "guild-past-1"
	channelID := "chan-past-1"
	userID := "userA"

	// Setup mock data in ei.EggIncContractsAll and ei.EggIncContracts
	ei.EggIncContractsMutex.Lock()
	if ei.EggIncContractsAll == nil {
		ei.EggIncContractsAll = make(map[string]ei.EggIncContract)
	}
	// Past contract in history (not in ei.EggIncContracts)
	ei.EggIncContractsAll[pastContractID] = ei.EggIncContract{
		ID:              pastContractID,
		Name:            "Past Super Contract",
		Description:     "A historic contract from the past",
		Predicted:       false,
		MaxCoopSize:     5,
		EggName:         "Superfood",
		Egg:             int32(ei.Egg_SUPERFOOD),
		LengthInSeconds: 86400,
		MinutesPerToken: 60,
		ValidFrom:       time.Now().Add(-60 * 24 * time.Hour),
		ValidUntil:      time.Now().Add(-53 * 24 * time.Hour),
	}
	// An active contract in ei.EggIncContracts
	activeContract := ei.EggIncContract{
		ID:              "active-current-1",
		Name:            "Active Current Contract",
		Description:     "A currently active contract",
		Predicted:       false,
		MaxCoopSize:     10,
		EggName:         "Quantum",
		LengthInSeconds: 172800,
		MinutesPerToken: 30,
		ValidFrom:       time.Now().Add(-2 * 24 * time.Hour),
		ValidUntil:      time.Now().Add(5 * 24 * time.Hour),
	}
	ei.EggIncContracts = []ei.EggIncContract{activeContract}
	ei.EggIncContractsAll[activeContract.ID] = activeContract
	ei.EggIncContractsMutex.Unlock()

	defer func() {
		ei.EggIncContractsMutex.Lock()
		delete(ei.EggIncContractsAll, pastContractID)
		delete(ei.EggIncContractsAll, activeContract.ID)
		ei.EggIncContracts = nil
		ei.EggIncContractsMutex.Unlock()
	}()

	// 1. Verify isPastContract
	if !isPastContract(pastContractID) {
		t.Fatalf("expected isPastContract(%q) to be true", pastContractID)
	}
	if isPastContract(activeContract.ID) {
		t.Fatalf("expected isPastContract(%q) to be false for active contract", activeContract.ID)
	}

	// 2. Create the contract with the past contract ID
	contract, err := CreateContract(client, pastContractID, "tbd", ContractPlaystyleACOCooperative, 0, ContractOrderSignup, guildID, channelID, []string{userID}, userID, time.Time{}, time.Now())
	if err != nil {
		t.Fatalf("CreateContract failed: %v", err)
	}
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract.ContractHash)
		ContractsMutex.Unlock()
	}()

	// 3. Verify it is created as a single predicted contract
	if !contract.PredictionSignup {
		t.Errorf("expected contract.PredictionSignup to be true, got false")
	}
	if !contract.WasPredictedContract {
		t.Errorf("expected contract.WasPredictedContract to be true, got false")
	}
	if len(contract.PredictionsList) != 1 || contract.PredictionsList[0] != pastContractID {
		t.Errorf("expected contract.PredictionsList to be [%s], got %v", pastContractID, contract.PredictionsList)
	}
	if len(contract.PredictionInfo) != 1 {
		t.Fatalf("expected 1 PredictionInfo, got %d", len(contract.PredictionInfo))
	}
	if contract.PredictionInfo[0].ContractID != pastContractID || contract.PredictionInfo[0].Name != "Past Super Contract" {
		t.Errorf("unexpected PredictionInfo: %+v", contract.PredictionInfo[0])
	}
	if contract.CoopSize != 100 {
		t.Errorf("expected CoopSize 100 for prediction signup, got %d", contract.CoopSize)
	}

	// 4. Verify initial creator has availability set for this single predicted contract
	booster := contract.Boosters[userID]
	if booster == nil {
		t.Fatalf("expected booster for %s to exist", userID)
	}
	if len(booster.Availability.Contract) != 1 || booster.Availability.Contract[0] != pastContractID {
		t.Errorf("expected booster availability to include %s, got %v", pastContractID, booster.Availability.Contract)
	}

	// 5. Verify DrawBoostList shows "# Contract Interest List" and the single predicted contract
	components := DrawBoostList(contract)
	var fullText strings.Builder
	for _, comp := range components {
		if textComp, ok := comp.(dc.TextDisplay); ok {
			fullText.WriteString(textComp.Content)
		}
	}
	output := fullText.String()
	if !strings.Contains(output, "# Contract Interest List") {
		t.Errorf("expected DrawBoostList to include '# Contract Interest List', got: %s", output)
	}
	if !strings.Contains(output, "Past Super Contract") {
		t.Errorf("expected DrawBoostList to include 'Past Super Contract', got: %s", output)
	}

	// 6. Verify thread name has 🔮 leading during prediction signup
	predThreadName := generateThreadName(contract)
	if !strings.HasPrefix(predThreadName, "🔮 ") {
		t.Errorf("expected predicted past contract thread name to start with '🔮 ', got: %s", predThreadName)
	}
	client.WithGuild(guildID, "Test Guild").
		WithThread(channelID, guildID, "parent-1", predThreadName)

	// 7. Add more boosters to exceed the original past contract's MaxCoopSize (5)
	extraUsers := []string{"userB", "userC", "userD", "userE", "userF", "userG"}
	for _, u := range extraUsers {
		_, err := AddFarmerToContract(client, contract, guildID, channelID, u, ContractOrderSignup, false, false)
		if err != nil {
			t.Fatalf("failed to add booster %s: %v", u, err)
		}
	}
	if len(contract.Boosters) != 7 {
		t.Fatalf("expected 7 boosters in prediction signup, got %d", len(contract.Boosters))
	}

	// 8. Simulate contract arrival: periodical arrives with live contract matching pastContractID
	liveArrival := ei.EggIncContract{
		ID:              pastContractID,
		Name:            "Past Super Contract Rerun",
		Description:     "Rerun of the historic contract",
		Predicted:       false,
		MaxCoopSize:     5, // New live coop size is 5
		EggName:         "Superfood",
		Egg:             int32(ei.Egg_SUPERFOOD),
		LengthInSeconds: 90000,
		MinutesPerToken: 45,
		ValidFrom:       time.Now(),
		ValidUntil:      time.Now().Add(7 * 24 * time.Hour),
	}

	// Update ei.EggIncContracts to include liveArrival (as periodicals does)
	ei.EggIncContractsMutex.Lock()
	ei.EggIncContracts = append(ei.EggIncContracts, liveArrival)
	ei.EggIncContractsAll[pastContractID] = liveArrival
	ei.EggIncContractsMutex.Unlock()

	updatedCount := UpdatePredictedSignupContracts(client, []ei.EggIncContract{liveArrival})
	if updatedCount != 1 {
		t.Fatalf("expected UpdatePredictedSignupContracts to update 1 contract, got %d", updatedCount)
	}

	// 9. Verify all fields were updated
	if contract.PredictionSignup {
		t.Errorf("expected PredictionSignup to be false after contract arrival")
	}
	if !contract.WasPredictedContract {
		t.Errorf("expected WasPredictedContract to remain true")
	}
	if contract.CoopSize != 5 {
		t.Errorf("expected CoopSize to update to live MaxCoopSize 5, got %d", contract.CoopSize)
	}
	if contract.LengthInSeconds != 90000 {
		t.Errorf("expected LengthInSeconds to update to 90000, got %d", contract.LengthInSeconds)
	}
	if contract.MinutesPerToken != 45 {
		t.Errorf("expected MinutesPerToken to update to 45, got %d", contract.MinutesPerToken)
	}
	if contract.Name != "Past Super Contract Rerun" {
		t.Errorf("expected Name to update to live name, got %s", contract.Name)
	}

	// 10. Verify thread name dropped 🔮 upon contract arrival
	liveThreadName := generateThreadName(contract)
	if strings.Contains(liveThreadName, "🔮") {
		t.Errorf("expected live contract thread name to drop '🔮', got: %s", liveThreadName)
	}
	edits := client.CallsTo("EditChannel")
	if len(edits) == 0 {
		t.Errorf("expected EditChannel to be called to rename thread upon contract arrival")
	} else {
		newName := edits[len(edits)-1].Args[1]
		if strings.Contains(newName, "🔮") {
			t.Errorf("expected EditChannel thread name to drop '🔮', got: %s", newName)
		}
	}

	// 11. Verify overflow boosters were moved to waitlist
	if len(contract.Order) > 5 {
		t.Errorf("expected contract.Order to be trimmed to 5, got %d", len(contract.Order))
	}
	if len(contract.WaitlistBoosters) == 0 {
		t.Errorf("expected overflow boosters to be moved to waitlist")
	}
}

func TestPredictedPastContractAutocomplete(t *testing.T) {
	pastContractID := "test-past-auto-xyz"
	activeContractID := "active-auto-abc"

	ei.EggIncContractsMutex.Lock()
	if ei.EggIncContractsAll == nil {
		ei.EggIncContractsAll = make(map[string]ei.EggIncContract)
	}
	ei.EggIncContractsAll[pastContractID] = ei.EggIncContract{
		ID:        pastContractID,
		Name:      "Past Galaxy Explorer",
		Predicted: false,
		ValidFrom: time.Now().Add(-100 * 24 * time.Hour),
	}
	activeContract := ei.EggIncContract{
		ID:        activeContractID,
		Name:      "Active Galaxy Quest",
		Predicted: false,
		ValidFrom: time.Now().Add(-1 * 24 * time.Hour),
	}
	ei.EggIncContracts = []ei.EggIncContract{activeContract}
	ei.EggIncContractsAll[activeContractID] = activeContract
	ei.EggIncContractsMutex.Unlock()

	defer func() {
		ei.EggIncContractsMutex.Lock()
		delete(ei.EggIncContractsAll, pastContractID)
		delete(ei.EggIncContractsAll, activeContractID)
		ei.EggIncContracts = nil
		ei.EggIncContractsMutex.Unlock()
	}()

	// Case 1: searchString == "" on /contract command
	// Should only show active contracts, not past contracts
	e1 := dctest.AutocompleteEvent("contract", "contract-id", "")
	HandleContractAutoComplete(e1)
	if len(e1.LastChoices) != 1 || e1.LastChoices[0].Value != activeContractID {
		t.Errorf("expected 1 choice with %s, got: %+v", activeContractID, e1.LastChoices)
	}

	// Case 2: searchString == "galaxy" on /contract command
	// Should show both active and past contracts matching "galaxy"
	e2 := dctest.AutocompleteEvent("contract", "contract-id", "galaxy")
	HandleContractAutoComplete(e2)
	if len(e2.LastChoices) != 2 {
		t.Fatalf("expected 2 choices, got: %d (%+v)", len(e2.LastChoices), e2.LastChoices)
	}
	foundPast := false
	foundActive := false
	for _, choice := range e2.LastChoices {
		if choice.Value == pastContractID {
			foundPast = true
		}
		if choice.Value == activeContractID {
			foundActive = true
		}
	}
	if !foundPast || !foundActive {
		t.Errorf("expected both past (%t) and active (%t) choices, got: %+v", foundPast, foundActive, e2.LastChoices)
	}

	// Case 3: searchString == "explorer" (matches only past contract)
	e3 := dctest.AutocompleteEvent("contract", "contract-id", "explorer")
	HandleContractAutoComplete(e3)
	if len(e3.LastChoices) != 1 || e3.LastChoices[0].Value != pastContractID {
		t.Errorf("expected 1 choice with %s, got: %+v", pastContractID, e3.LastChoices)
	}
}

func TestPredictedPastContractThreadNaming(t *testing.T) {
	pastContractID := "test-past-thread-name-1"

	ei.EggIncContractsMutex.Lock()
	if ei.EggIncContractsAll == nil {
		ei.EggIncContractsAll = make(map[string]ei.EggIncContract)
	}
	ei.EggIncContractsAll[pastContractID] = ei.EggIncContract{
		ID:          pastContractID,
		Name:        "Historic Egg Mission",
		Description: "Historic Mission",
		Predicted:   false,
		MaxCoopSize: 5,
	}
	ei.EggIncContractsMutex.Unlock()

	defer func() {
		ei.EggIncContractsMutex.Lock()
		delete(ei.EggIncContractsAll, pastContractID)
		ei.EggIncContractsMutex.Unlock()
	}()

	c := &Contract{
		ContractHash:         "test-hash-thread-naming",
		ContractID:           pastContractID,
		CoopID:               "predicted",
		CoopSize:             100,
		Name:                 "Historic Egg Mission",
		PredictionSignup:     true,
		WasPredictedContract: true,
		PlayStyle:            ContractPlaystyleACOCooperative,
		State:                ContractStateSignup,
		Boosters: map[string]*Booster{
			"u1": {UserID: "u1"},
		},
		Location: []*LocationData{
			{GuildID: "guild1", ChannelID: "chan1"},
		},
	}

	// 1. Prediction state: Must start with 🔮
	name := generateThreadName(c)
	if !strings.HasPrefix(name, "🔮 ") {
		t.Errorf("expected thread name to start with '🔮 ', got %q", name)
	}
	if !strings.Contains(name, "🟩 ") {
		t.Errorf("expected thread name to contain playstyle icon '🟩 ', got %q", name)
	}

	// 2. Playstyle unset: Must still start with 🔮
	c.PlayStyle = ContractPlaystyleUnset
	nameUnset := generateThreadName(c)
	if !strings.HasPrefix(nameUnset, "🔮 ") {
		t.Errorf("expected unset playstyle thread name to start with '🔮 ', got %q", nameUnset)
	}

	// 3. Once contract arrives: PredictionSignup becomes false, 🔮 is dropped
	c.PlayStyle = ContractPlaystyleACOCooperative
	c.PredictionSignup = false
	c.CoopSize = 5
	nameArrived := generateThreadName(c)
	if strings.Contains(nameArrived, "🔮") {
		t.Errorf("expected thread name to drop '🔮' after contract arrival, got %q", nameArrived)
	}
	if !strings.HasPrefix(nameArrived, "🟩 ") {
		t.Errorf("expected arrived thread name to start with '🟩 ', got %q", nameArrived)
	}

	// 4. Regular (non-past) prediction contract: Should NOT have 🔮
	regularPred := &Contract{
		ContractHash:         "test-hash-reg-pred",
		ContractID:           "monday-2026-10-05",
		CoopID:               "predicted",
		CoopSize:             100,
		Name:                 "Monday Seasonal",
		PredictionSignup:     true,
		WasPredictedContract: true,
		PlayStyle:            ContractPlaystyleACOCooperative,
		State:                ContractStateSignup,
		Boosters: map[string]*Booster{
			"u1": {UserID: "u1"},
		},
		Location: []*LocationData{
			{GuildID: "guild1", ChannelID: "chan1"},
		},
	}
	regName := generateThreadName(regularPred)
	if strings.Contains(regName, "🔮") {
		t.Errorf("regular prediction contract should NOT have 🔮, got %q", regName)
	}
}

func TestPredictedContract_DisableStartContractButton(t *testing.T) {
	contract := &Contract{
		ContractHash:         "test-pred-start-btn",
		ContractID:           "test-pred-contract",
		CoopID:               "validcoop", // non-TBD coopID
		CoopSize:             100,
		Name:                 "Prediction Contract",
		PredictionSignup:     true,
		WasPredictedContract: true,
		State:                ContractStateSignup,
		CreatorID:            []string{"userA"},
		Boosters: map[string]*Booster{
			"userA": {UserID: "userA"},
		},
		Location: []*LocationData{
			{GuildID: "guild1", ChannelID: "chan1"},
		},
	}

	// Helper to find the start button
	findStartButton := func(comps []dc.LayoutComponent) *dc.Button {
		for _, row := range comps {
			if actionRow, ok := row.(dc.ActionRow); ok {
				for _, comp := range actionRow.Components {
					if btn, ok := comp.(dc.Button); ok && btn.CustomID == "fd_signupStart" {
						return &btn
					}
				}
			}
		}
		return nil
	}

	// 1. In prediction signup state: start button must be disabled
	_, comps := GetSignupComponents(contract)
	startBtn := findStartButton(comps)
	if startBtn == nil {
		t.Fatalf("start button not found in signup components")
	}
	if !startBtn.Disabled {
		t.Errorf("expected start button to be disabled for predicted contract")
	}

	// 2. StartContractBoosting should fail if called while PredictionSignup is true
	ContractsMutex.Lock()
	Contracts[contract.ContractHash] = contract
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract.ContractHash)
		ContractsMutex.Unlock()
	}()

	client := dctest.New()
	err := StartContractBoosting(client, "guild1", "chan1", "userA")
	if err == nil {
		t.Errorf("expected StartContractBoosting to return error for predicted contract, got nil")
	}

	// 3. Once contract is no longer in prediction signup (arrived): start button is enabled
	contract.PredictionSignup = false
	contract.CoopSize = 5
	_, compsLive := GetSignupComponents(contract)
	startBtnLive := findStartButton(compsLive)
	if startBtnLive == nil {
		t.Fatalf("start button not found in live signup components")
	}
	if startBtnLive.Disabled {
		t.Errorf("expected start button to be enabled once contract arrived and is no longer predicted")
	}
}

func TestPredictedPastContract_ArchiveCleanupAfterThreeWeeks(t *testing.T) {
	pastContractID := "test-past-archive-contract"
	guildID := "guild-arch-1"
	threadID := "thread-arch-1"

	// Mock past contract in ei.EggIncContractsAll with historical ValidUntil in the past
	ei.EggIncContractsMutex.Lock()
	if ei.EggIncContractsAll == nil {
		ei.EggIncContractsAll = make(map[string]ei.EggIncContract)
	}
	ei.EggIncContractsAll[pastContractID] = ei.EggIncContract{
		ID:          pastContractID,
		Name:        "Old Contract From Years Ago",
		Description: "Historic",
		Predicted:   false,
		MaxCoopSize: 5,
		ValidFrom:   time.Now().Add(-365 * 24 * time.Hour),
		ValidUntil:  time.Now().Add(-350 * 24 * time.Hour), // Expired a year ago
	}
	ei.EggIncContractsMutex.Unlock()

	defer func() {
		ei.EggIncContractsMutex.Lock()
		delete(ei.EggIncContractsAll, pastContractID)
		ei.EggIncContractsMutex.Unlock()
	}()

	client := dctest.New().
		WithGuild(guildID, "Test Guild").
		WithThread(threadID, guildID, "parent-chan", "🔮 Old Contract Signup")

	// Case 1: Predicted past contract created 10 days ago (< 3 weeks)
	// Should NOT be archived, even though historical ValidUntil is long expired.
	contractRecent := &Contract{
		ContractHash:         "test-hash-recent-pred",
		ContractID:           pastContractID,
		CoopID:               "predicted",
		CoopSize:             100,
		Name:                 "Old Contract From Years Ago",
		PredictionSignup:     true,
		WasPredictedContract: true,
		State:                ContractStateSignup,
		StartTime:            time.Now().Add(-10 * 24 * time.Hour),
		Location: []*LocationData{
			{GuildID: guildID, ChannelID: threadID},
		},
	}

	ContractsMutex.Lock()
	Contracts[contractRecent.ContractHash] = contractRecent
	ContractsMutex.Unlock()

	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contractRecent.ContractHash)
		ContractsMutex.Unlock()
	}()

	ArchiveContracts(client)

	ContractsMutex.RLock()
	c1 := Contracts[contractRecent.ContractHash]
	ContractsMutex.RUnlock()

	if c1 == nil || c1.State == ContractStateArchive {
		t.Errorf("expected predicted past contract under 3 weeks old to NOT be archived")
	}

	// Case 2: Predicted past contract created 22 days ago (> 3 weeks)
	// Should be archived because it has exceeded 3 weeks without actual contract showing up.
	contractOld := &Contract{
		ContractHash:         "test-hash-old-pred",
		ContractID:           pastContractID,
		CoopID:               "predicted",
		CoopSize:             100,
		Name:                 "Old Contract From Years Ago",
		PredictionSignup:     true,
		WasPredictedContract: true,
		State:                ContractStateSignup,
		StartTime:            time.Now().Add(-22 * 24 * time.Hour),
		Location: []*LocationData{
			{GuildID: guildID, ChannelID: threadID},
		},
	}

	ContractsMutex.Lock()
	Contracts[contractOld.ContractHash] = contractOld
	ContractsMutex.Unlock()

	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contractOld.ContractHash)
		ContractsMutex.Unlock()
	}()
	ArchiveContracts(client)

	ContractsMutex.RLock()
	c2 := Contracts[contractOld.ContractHash]
	ContractsMutex.RUnlock()

	if c2 != nil {
		t.Errorf("expected predicted past contract over 3 weeks old to be archived and removed from Contracts, got state: %v, hash: %s", c2.State, c2.ContractHash)
	}
}
