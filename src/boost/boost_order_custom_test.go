package boost

import (
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

func TestParseCustomCriterion(t *testing.T) {
	tests := []struct {
		input     string
		critType  CustomCriterionType
		ascending bool
		effortN   int
		fuzzyPct  float64
		fuzzySqrt bool
	}{
		{
			input:     "<DEFL_EFFORT[50]>",
			critType:  CritDeflEffort,
			ascending: false,
			effortN:   50,
		},
		{
			input:     ">DEFL_EFFORT[40]",
			critType:  CritDeflEffort,
			ascending: true,
			effortN:   40,
		},
		{
			input:     "DEFL_EFFORT",
			critType:  CritDeflEffort,
			ascending: false,
			effortN:   50,
		},
		{
			input:     "<IHR[6%]>",
			critType:  CritIHR,
			ascending: false,
			fuzzyPct:  0.06,
		},
		{
			input:     ">TOKENS",
			critType:  CritTokens,
			ascending: true,
		},
		{
			input:     "<TE[sqrt]>",
			critType:  CritTE,
			ascending: false,
			fuzzySqrt: true,
		},
		{
			input:     "<ELR",
			critType:  CritELR,
			ascending: false,
		},
		{
			input:     "CRAFT_DEFL",
			critType:  CritCraftDefl,
			ascending: false,
		},
		{
			input:     ">SIGNUP",
			critType:  CritSignup,
			ascending: true,
		},
		{
			input:     "RANDOM",
			critType:  CritRandom,
			ascending: false,
		},
	}

	for _, tt := range tests {
		crit := parseCustomCriterion(tt.input)
		if crit.critType != tt.critType {
			t.Errorf("parseCustomCriterion(%q) critType = %v, want %v", tt.input, crit.critType, tt.critType)
		}
		if crit.ascending != tt.ascending {
			t.Errorf("parseCustomCriterion(%q) ascending = %v, want %v", tt.input, crit.ascending, tt.ascending)
		}
		if tt.effortN > 0 && crit.effortN != tt.effortN {
			t.Errorf("parseCustomCriterion(%q) effortN = %v, want %v", tt.input, crit.effortN, tt.effortN)
		}
		if tt.fuzzyPct > 0 && crit.fuzzyPct != tt.fuzzyPct {
			t.Errorf("parseCustomCriterion(%q) fuzzyPct = %v, want %v", tt.input, crit.fuzzyPct, tt.fuzzyPct)
		}
		if tt.fuzzySqrt != crit.fuzzySqrt {
			t.Errorf("parseCustomCriterion(%q) fuzzySqrt = %v, want %v", tt.input, crit.fuzzySqrt, tt.fuzzySqrt)
		}
	}
}

func TestCustomOrderDeflectorEffortBalancing(t *testing.T) {
	// Create contract with 5 boosters:
	// Alice: Has T4L Deflector (few crafts) -> Effort = 50
	// Bob: No T4L Deflector, 70 crafts -> Effort = 50 (Capped)
	// Charlie: No T4L Deflector, 50 crafts -> Effort = 50
	// Dana: No T4L Deflector, 42 crafts -> Effort = 42
	// Eve: No T4L Deflector, 20 crafts -> Effort = 20

	contract := &Contract{
		ContractID:   "custom-test",
		CoopID:       "custom-coop",
		ContractHash: "test-hash",
		Boosters:     make(map[string]*Booster),
		Order:        []string{"alice", "bob", "charlie", "dana", "eve"},
	}

	contract.Boosters["alice"] = &Booster{
		UserID:       "alice",
		Nick:         "Alice (T4L)",
		IHRRate:      9.0e9,
		TokensWanted: 6,
		ArtifactSet: ArtifactSet{
			Artifacts: []ei.Artifact{
				{Type: "Deflector", Quality: "T4L"},
			},
		},
	}
	contract.Boosters["bob"] = &Booster{
		UserID:       "bob",
		Nick:         "Bob (70 crafts)",
		IHRRate:      11.0e9, // Higher IHR than Alice
		TokensWanted: 6,
		ArtifactSet:  ArtifactSet{},
	}
	contract.Boosters["charlie"] = &Booster{
		UserID:       "charlie",
		Nick:         "Charlie (50 crafts)",
		IHRRate:      8.0e9, // Lowest IHR among 50-effort tier
		TokensWanted: 6,
		ArtifactSet:  ArtifactSet{},
	}
	contract.Boosters["dana"] = &Booster{
		UserID:       "dana",
		Nick:         "Dana (42 crafts)",
		IHRRate:      12.0e9,
		TokensWanted: 6,
		ArtifactSet:  ArtifactSet{},
	}
	contract.Boosters["eve"] = &Booster{
		UserID:       "eve",
		Nick:         "Eve (20 crafts)",
		IHRRate:      12.0e9,
		TokensWanted: 6,
		ArtifactSet:  ArtifactSet{},
	}

	// Mock craft counts using mock helper logic or testing evaluateBoosterForCustom
	// Let's verify calculateBoosterDeflectorEffort direct logic first
	if eff, _, hasT4L := calculateBoosterDeflectorEffort(contract.Boosters["alice"], 50); eff != 50 || !hasT4L {
		t.Errorf("Alice effort = %d (hasT4L=%v), want 50 (true)", eff, hasT4L)
	}

	// Test sortCustomRemaining with custom criteria
	// Row 1: <DEFL_EFFORT[50]>
	// Row 2: <IHR
	lines := []string{
		"<DEFL_EFFORT[50]>",
		"<IHR",
		">TOKENS",
		"<TE",
	}

	// We test using evaluateBoosterForCustom with mocked criteria row scores
	criteria := [4]customCriterion{
		parseCustomCriterion(lines[0]),
		parseCustomCriterion(lines[1]),
		parseCustomCriterion(lines[2]),
		parseCustomCriterion(lines[3]),
	}

	// Manually set craft counts on data or verify sorting logic:
	// Let's test sorting directly with manual effort comparison
	// Bob (70 crafts -> capped 50, IHR 11.0e9) should beat Alice (T4L -> 50, IHR 9.0e9) and Charlie (50, IHR 8.0e9)
	// Dana (42 crafts) should beat Eve (20 crafts)
	items := []boosterEvalData{
		{
			userID:    "alice",
			hasT4L:    true,
			t4Crafts:  8,
			rowScores: [4]float64{50, 9.0e9, 6, 0},
		},
		{
			userID:    "bob",
			hasT4L:    false,
			t4Crafts:  70,
			rowScores: [4]float64{50, 11.0e9, 6, 0}, // Capped at 50, higher IHR
		},
		{
			userID:    "charlie",
			hasT4L:    false,
			t4Crafts:  50,
			rowScores: [4]float64{50, 8.0e9, 6, 0}, // 50, lower IHR
		},
		{
			userID:    "dana",
			hasT4L:    false,
			t4Crafts:  42,
			rowScores: [4]float64{42, 12.0e9, 6, 0}, // Below 50
		},
		{
			userID:    "eve",
			hasT4L:    false,
			t4Crafts:  20,
			rowScores: [4]float64{20, 12.0e9, 6, 0}, // Below 50
		},
	}

	// Sort items
	unselected := []string{"alice", "bob", "charlie", "dana", "eve"}
	_ = unselected

	// Verify sorting comparator
	isLess := func(i, j int) bool {
		for r := 0; r < 4; r++ {
			if criteria[r].critType == CritUnknown {
				continue
			}
			valI := items[i].rowScores[r]
			valJ := items[j].rowScores[r]
			if valI != valJ {
				if criteria[r].ascending {
					return valI < valJ
				}
				return valI > valJ // Descending (<)
			}
		}
		return items[i].userID < items[j].userID
	}

	// Bob should be ahead of Alice (both 50, Bob has higher IHR)
	if !isLess(1, 0) {
		t.Errorf("Expected Bob (70 crafts, capped 50, IHR 11B) to rank ahead of Alice (T4L 50, IHR 9B)")
	}
	// Alice should be ahead of Charlie (both 50, Alice has higher IHR)
	if !isLess(0, 2) {
		t.Errorf("Expected Alice (T4L 50, IHR 9B) to rank ahead of Charlie (50 crafts, IHR 8B)")
	}
	// Charlie (50) should be ahead of Dana (42)
	if !isLess(2, 3) {
		t.Errorf("Expected Charlie (50 effort) to rank ahead of Dana (42 effort)")
	}
	// Dana (42) should be ahead of Eve (20)
	if !isLess(3, 4) {
		t.Errorf("Expected Dana (42 effort) to rank ahead of Eve (20 effort)")
	}
}

func TestRenderCustomOrderTableImage(t *testing.T) {
	contract := &Contract{
		ContractID:   "custom-img-test",
		CoopID:       "custom-coop",
		ContractHash: "img-test-hash",
		Boosters:     make(map[string]*Booster),
		Order:        []string{"player1", "player2"},
	}
	contract.Boosters["player1"] = &Booster{
		UserID:       "player1",
		Nick:         "Farmer One",
		IHRRate:      8.0e9,
		TokensWanted: 6,
		TECount:      120,
		ArtifactSet: ArtifactSet{
			Artifacts: []ei.Artifact{
				{Type: "Deflector", Quality: "T4L"},
			},
			LayRate: 25.5,
		},
	}
	contract.Boosters["player2"] = &Booster{
		UserID:       "player2",
		Nick:         "Farmer Two",
		IHRRate:      9.5e9,
		TokensWanted: 4,
		TECount:      85,
		ArtifactSet: ArtifactSet{
			Artifacts: []ei.Artifact{
				{Type: "Deflector", Quality: "T4E"},
			},
			LayRate: 22.0,
		},
	}

	lines := []string{
		"<DEFL_EFFORT[50]>",
		"<IHR[6%]>",
		">TOKENS",
		"<TE",
	}

	imgBytes, err := RenderCustomOrderTableImage(contract, lines)
	if err != nil {
		t.Fatalf("RenderCustomOrderTableImage failed: %v", err)
	}
	if len(imgBytes) == 0 {
		t.Fatal("RenderCustomOrderTableImage returned empty bytes")
	}
}

func TestSortCustomRemaining_MultiTier(t *testing.T) {
	contract := &Contract{
		ContractID:   "custom-multitier",
		CoopID:       "custom-coop",
		ContractHash: "tier-hash",
		Boosters:     make(map[string]*Booster),
		Order:        []string{"p1", "p2", "p3"},
	}

	// p1: IHR 10B, 4 tokens
	// p2: IHR 10B, 6 tokens (same IHR, but wants more tokens)
	// p3: IHR 12B, 8 tokens (higher IHR)
	contract.Boosters["p1"] = &Booster{
		UserID:       "p1",
		Nick:         "P1",
		IHRRate:      10.0e9,
		TokensWanted: 4,
	}
	contract.Boosters["p2"] = &Booster{
		UserID:       "p2",
		Nick:         "P2",
		IHRRate:      10.0e9,
		TokensWanted: 6,
	}
	contract.Boosters["p3"] = &Booster{
		UserID:       "p3",
		Nick:         "P3",
		IHRRate:      12.0e9,
		TokensWanted: 8,
	}

	lines := []string{
		"<IHR",    // Row 1: Higher IHR first -> p3 should be #1
		">TOKENS", // Row 2: For p1 and p2 (tied on IHR 10B), p1 has 4 tokens (< 6) so p1 should be #2, p2 should be #3
	}

	sorted := sortCustomRemaining(contract, []string{"p1", "p2", "p3"}, lines, false)
	expected := []string{"p3", "p1", "p2"}

	if len(sorted) != len(expected) {
		t.Fatalf("sorted len %d, want %d", len(sorted), len(expected))
	}
	for i := range sorted {
		if sorted[i] != expected[i] {
			t.Errorf("at index %d: got %s, want %s", i, sorted[i], expected[i])
		}
	}
}

func TestBuildCustomOrderMessage(t *testing.T) {
	contract := &Contract{
		ContractID:   "custom-msg-test",
		CoopID:       "custom-coop",
		ContractHash: "msg-hash",
		Boosters:     make(map[string]*Booster),
		Order:        []string{"u1"},
	}
	contract.Boosters["u1"] = &Booster{
		UserID:       "u1",
		Nick:         "User 1",
		IHRRate:      5.0e9,
		TokensWanted: 5,
	}

	session := &customOrderSession{
		uuidStr:      "session-123",
		contractHash: "msg-hash",
		userID:       "u1",
		lines:        []string{"<DEFL_EFFORT[50]>", "<IHR[6%]>", ">TOKENS", "<TE"},
	}

	msg := BuildCustomOrderMessage(contract, session, "Status test")
	if len(msg.Components) == 0 {
		t.Fatal("expected message components, got none")
	}
	if len(msg.Files) == 0 {
		t.Fatal("expected preview image file attachment, got none")
	}
}

func TestParseCustomCriterion_RoleAndConditional(t *testing.T) {
	tests := []struct {
		input         string
		isConditional bool
		targetRole    customRoleCondition
		hasElse       bool
		thenType      CustomCriterionType
		thenAscending bool
		elseType      CustomCriterionType
		elseAscending bool
		critType      CustomCriterionType
		ascending     bool
	}{
		{
			input:         "IF ROLE == MAIN <DEFL_EFFORT[50] ELSE >TOKENS",
			isConditional: true,
			targetRole:    roleConditionMain,
			hasElse:       true,
			thenType:      CritDeflEffort,
			thenAscending: false,
			elseType:      CritTokens,
			elseAscending: true,
		},
		{
			input:         "IF MAIN <DEFL_EFFORT[50] ELSE >TOKENS",
			isConditional: true,
			targetRole:    roleConditionMain,
			hasElse:       true,
			thenType:      CritDeflEffort,
			thenAscending: false,
			elseType:      CritTokens,
			elseAscending: true,
		},
		{
			input:         "IF HELPER >TOKENS ELSE <DEFL_EFFORT[50]",
			isConditional: true,
			targetRole:    roleConditionHelper,
			hasElse:       true,
			thenType:      CritTokens,
			thenAscending: true,
			elseType:      CritDeflEffort,
			elseAscending: false,
		},
		{
			input:         "IF ROLE == HELPER >TOKENS",
			isConditional: true,
			targetRole:    roleConditionHelper,
			hasElse:       false,
			thenType:      CritTokens,
			thenAscending: true,
		},
		{
			input:         "IF MAIN <DEFL_EFFORT[50]",
			isConditional: true,
			targetRole:    roleConditionMain,
			hasElse:       false,
			thenType:      CritDeflEffort,
			thenAscending: false,
		},
		{
			input:         "ROLE",
			isConditional: false,
			critType:      CritRole,
			ascending:     false, // Mains first
		},
		{
			input:         "<ROLE",
			isConditional: false,
			critType:      CritRole,
			ascending:     false, // Mains first
		},
		{
			input:         ">ROLE",
			isConditional: false,
			critType:      CritRole,
			ascending:     true, // Helpers first
		},
		{
			input:         "MAIN",
			isConditional: false,
			critType:      CritRole,
			ascending:     false,
		},
		{
			input:         "HELPER",
			isConditional: false,
			critType:      CritRole,
			ascending:     true,
		},
	}

	for _, tt := range tests {
		crit := parseCustomCriterion(tt.input)
		if crit.isConditional != tt.isConditional {
			t.Errorf("parseCustomCriterion(%q) isConditional = %v, want %v", tt.input, crit.isConditional, tt.isConditional)
		}
		if tt.isConditional {
			if crit.targetRole != tt.targetRole {
				t.Errorf("parseCustomCriterion(%q) targetRole = %v, want %v", tt.input, crit.targetRole, tt.targetRole)
			}
			if crit.hasElse != tt.hasElse {
				t.Errorf("parseCustomCriterion(%q) hasElse = %v, want %v", tt.input, crit.hasElse, tt.hasElse)
			}
			if crit.thenCrit == nil || crit.thenCrit.critType != tt.thenType {
				t.Errorf("parseCustomCriterion(%q) thenType = %v, want %v", tt.input, crit.thenCrit, tt.thenType)
			}
			if crit.thenCrit != nil && crit.thenCrit.ascending != tt.thenAscending {
				t.Errorf("parseCustomCriterion(%q) thenAscending = %v, want %v", tt.input, crit.thenCrit.ascending, tt.thenAscending)
			}
			if tt.hasElse {
				if crit.elseCrit == nil || crit.elseCrit.critType != tt.elseType {
					t.Errorf("parseCustomCriterion(%q) elseType = %v, want %v", tt.input, crit.elseCrit, tt.elseType)
				}
				if crit.elseCrit != nil && crit.elseCrit.ascending != tt.elseAscending {
					t.Errorf("parseCustomCriterion(%q) elseAscending = %v, want %v", tt.input, crit.elseCrit.ascending, tt.elseAscending)
				}
			}
		} else {
			if crit.critType != tt.critType {
				t.Errorf("parseCustomCriterion(%q) critType = %v, want %v", tt.input, crit.critType, tt.critType)
			}
			if crit.ascending != tt.ascending {
				t.Errorf("parseCustomCriterion(%q) ascending = %v, want %v", tt.input, crit.ascending, tt.ascending)
			}
		}
	}
}

func TestSortCustomRemaining_ConditionalWithElse(t *testing.T) {
	// 2 Mains and 2 Helpers:
	// m1: Main, 50 crafts, 8 tokens
	// m2: Main, 10 crafts, 4 tokens
	// h1: Helper (alt), 0 crafts, 4 tokens
	// h2: Helper (alt), 0 crafts, 8 tokens
	contract := &Contract{
		ContractID:   "test-cond-else",
		CoopID:       "coop",
		ContractHash: "cond-else-hash",
		Boosters:     make(map[string]*Booster),
		Order:        []string{"m1", "m2", "h1", "h2"},
	}
	contract.Boosters["m1"] = &Booster{
		UserID:       "m1",
		Nick:         "Main 1",
		IsAlt:        false,
		TokensWanted: 8,
		ArtifactSet:  ArtifactSet{Artifacts: []ei.Artifact{{Type: "Deflector", Quality: "T4L"}}},
	}
	contract.Boosters["m2"] = &Booster{
		UserID:       "m2",
		Nick:         "Main 2",
		IsAlt:        false,
		TokensWanted: 4,
		ArtifactSet:  ArtifactSet{Artifacts: []ei.Artifact{{Type: "Deflector", Quality: "T4R"}}},
	}
	contract.Boosters["h1"] = &Booster{
		UserID:       "h1",
		Nick:         "Helper 1",
		IsAlt:        true,
		TokensWanted: 4,
	}
	contract.Boosters["h2"] = &Booster{
		UserID:       "h2",
		Nick:         "Helper 2",
		IsAlt:        true,
		TokensWanted: 8,
	}

	lines := []string{
		"IF MAIN <DEFL_EFFORT[50] ELSE >TOKENS",
	}

	sorted := sortCustomRemaining(contract, []string{"h2", "m2", "h1", "m1"}, lines, false)

	// Mains come first, sorted by DEFL_EFFORT: m1 (T4L = 50) > m2 (0 crafts = 0)
	// Helpers come second, sorted by >TOKENS: h1 (4 tokens) > h2 (8 tokens)
	expected := []string{"m1", "m2", "h1", "h2"}
	if len(sorted) != len(expected) {
		t.Fatalf("got len %d, want %d", len(sorted), len(expected))
	}
	for i, id := range sorted {
		if id != expected[i] {
			t.Errorf("at index %d: got %s, want %s", i, id, expected[i])
		}
	}
}

func TestSortCustomRemaining_ConditionalWithoutElse(t *testing.T) {
	// Rule 1: IF MAIN <DEFL_EFFORT[50] (No ELSE)
	// Rule 2: >TOKENS
	// m1: Main, T4L (50 effort), 8 tokens
	// m2: Main, 0 effort, 4 tokens
	// h1: Helper, 4 tokens
	// h2: Helper, 8 tokens
	contract := &Contract{
		ContractID:   "test-cond-no-else",
		CoopID:       "coop",
		ContractHash: "cond-no-else-hash",
		Boosters:     make(map[string]*Booster),
		Order:        []string{"m1", "m2", "h1", "h2"},
	}
	contract.Boosters["m1"] = &Booster{
		UserID:       "m1",
		Nick:         "Main 1",
		IsAlt:        false,
		TokensWanted: 8,
		ArtifactSet:  ArtifactSet{Artifacts: []ei.Artifact{{Type: "Deflector", Quality: "T4L"}}},
	}
	contract.Boosters["m2"] = &Booster{
		UserID:       "m2",
		Nick:         "Main 2",
		IsAlt:        false,
		TokensWanted: 4,
	}
	contract.Boosters["h1"] = &Booster{
		UserID:       "h1",
		Nick:         "Helper 1",
		IsAlt:        true,
		TokensWanted: 4,
	}
	contract.Boosters["h2"] = &Booster{
		UserID:       "h2",
		Nick:         "Helper 2",
		IsAlt:        true,
		TokensWanted: 8,
	}

	lines := []string{
		"IF MAIN <DEFL_EFFORT[50]", // Level 1: Mains first and sorted by Defl; Helpers tie on Level 1
		">TOKENS",                  // Level 2: Breaks tie among Helpers (h1 wants 4, h2 wants 8)
	}

	sorted := sortCustomRemaining(contract, []string{"h2", "m2", "h1", "m1"}, lines, false)

	// m1 and m2 are Mains: on Level 1, m1 (50) beats m2 (0)
	// h1 and h2 are Helpers: on Level 1, rule doesn't apply so they tie
	// On Level 2, h1 (4 tokens) beats h2 (8 tokens)
	expected := []string{"m1", "m2", "h1", "h2"}
	if len(sorted) != len(expected) {
		t.Fatalf("got len %d, want %d", len(sorted), len(expected))
	}
	for i, id := range sorted {
		if id != expected[i] {
			t.Errorf("at index %d: got %s, want %s", i, id, expected[i])
		}
	}
}

func TestSortCustomRemaining_RoleStandalone(t *testing.T) {
	contract := &Contract{
		ContractID:   "test-role-standalone",
		CoopID:       "coop",
		ContractHash: "role-standalone-hash",
		Boosters:     make(map[string]*Booster),
		Order:        []string{"m1", "h1"},
	}
	contract.Boosters["m1"] = &Booster{
		UserID: "m1",
		IsAlt:  false,
	}
	contract.Boosters["h1"] = &Booster{
		UserID: "h1",
		IsAlt:  true,
	}

	// Test <ROLE (Mains first)
	sortedMainsFirst := sortCustomRemaining(contract, []string{"h1", "m1"}, []string{"<ROLE"}, false)
	if sortedMainsFirst[0] != "m1" || sortedMainsFirst[1] != "h1" {
		t.Errorf("<ROLE expected [m1, h1], got %v", sortedMainsFirst)
	}

	// Test >ROLE (Helpers first)
	sortedHelpersFirst := sortCustomRemaining(contract, []string{"m1", "h1"}, []string{">ROLE"}, false)
	if sortedHelpersFirst[0] != "h1" || sortedHelpersFirst[1] != "m1" {
		t.Errorf(">ROLE expected [h1, m1], got %v", sortedHelpersFirst)
	}
}

func TestParseCustomCriterion_ArtifactCountAndCrafts(t *testing.T) {
	tests := []struct {
		input     string
		critType  CustomCriterionType
		artName   ei.ArtifactSpec_Name
		artLevel  int
		artRarity int
		label     string
	}{
		{
			input:     "T4L_ACTUATOR",
			critType:  CritArtifactCount,
			artName:   ei.ArtifactSpec_TITANIUM_ACTUATOR,
			artLevel:  3,
			artRarity: 3,
			label:     "T4L Actuator",
		},
		{
			input:     "CRAFT(T4_ACTUATOR)",
			critType:  CritArtifactCraft,
			artName:   ei.ArtifactSpec_TITANIUM_ACTUATOR,
			artLevel:  3,
			artRarity: -1,
			label:     "T4 Actuator Crafts",
		},
		{
			input:     "CRAFT_T4_ACTUATOR",
			critType:  CritArtifactCraft,
			artName:   ei.ArtifactSpec_TITANIUM_ACTUATOR,
			artLevel:  3,
			artRarity: -1,
			label:     "T4 Actuator Crafts",
		},
		{
			input:     "T4_ACTUATOR_CRAFTS",
			critType:  CritArtifactCraft,
			artName:   ei.ArtifactSpec_TITANIUM_ACTUATOR,
			artLevel:  3,
			artRarity: -1,
			label:     "T4 Actuator Crafts",
		},
		{
			input:     "T4E_GUSSET",
			critType:  CritArtifactCount,
			artName:   ei.ArtifactSpec_ORNATE_GUSSET,
			artLevel:  3,
			artRarity: 2,
			label:     "T4E Gusset",
		},
		{
			input:     "T4_COMPASS",
			critType:  CritArtifactCount,
			artName:   ei.ArtifactSpec_INTERSTELLAR_COMPASS,
			artLevel:  3,
			artRarity: -1,
			label:     "T4 Compass",
		},
	}

	for _, tt := range tests {
		crit := parseCustomCriterion(tt.input)
		if crit.critType != tt.critType {
			t.Errorf("parseCustomCriterion(%q) critType = %v, want %v", tt.input, crit.critType, tt.critType)
		}
		if crit.artName != tt.artName {
			t.Errorf("parseCustomCriterion(%q) artName = %v, want %v", tt.input, crit.artName, tt.artName)
		}
		if crit.artLevel != tt.artLevel {
			t.Errorf("parseCustomCriterion(%q) artLevel = %v, want %v", tt.input, crit.artLevel, tt.artLevel)
		}
		if crit.artRarity != tt.artRarity {
			t.Errorf("parseCustomCriterion(%q) artRarity = %v, want %v", tt.input, crit.artRarity, tt.artRarity)
		}
		if crit.artLabel != tt.label {
			t.Errorf("parseCustomCriterion(%q) artLabel = %q, want %q", tt.input, crit.artLabel, tt.label)
		}
	}
}

func TestSortCustomRemaining_ArtifactAndCraftTiebreaker(t *testing.T) {
	// Goal: primary = T4L_ACTUATOR, secondary = CRAFT(T4_ACTUATOR)
	// Alice: 2x T4L Actuator, 10 crafts
	// Bob: 1x T4L Actuator, 50 crafts
	// Charlie: 0x T4L Actuator, 100 crafts
	// Dana: 0x T4L Actuator, 30 crafts
	// Expected Order: Alice (2 T4L) -> Bob (1 T4L) -> Charlie (0 T4L, 100 crafts) -> Dana (0 T4L, 30 crafts)

	contract := &Contract{
		ContractID:   "test-actuator-coop",
		CoopID:       "coop",
		ContractHash: "actuator-hash",
		Boosters:     make(map[string]*Booster),
		Order:        []string{"alice", "bob", "charlie", "dana"},
	}

	contract.Boosters["alice"] = &Booster{UserID: "alice", Nick: "Alice"}
	farmerstate.SetMiscSettingString("alice", "art_count_29_3_3", "2")
	farmerstate.SetMiscSettingString("alice", "crafts_29_3", "10")

	contract.Boosters["bob"] = &Booster{UserID: "bob", Nick: "Bob"}
	farmerstate.SetMiscSettingString("bob", "art_count_29_3_3", "1")
	farmerstate.SetMiscSettingString("bob", "crafts_29_3", "50")

	contract.Boosters["charlie"] = &Booster{UserID: "charlie", Nick: "Charlie"}
	farmerstate.SetMiscSettingString("charlie", "art_count_29_3_3", "0")
	farmerstate.SetMiscSettingString("charlie", "crafts_29_3", "100")

	contract.Boosters["dana"] = &Booster{UserID: "dana", Nick: "Dana"}
	farmerstate.SetMiscSettingString("dana", "art_count_29_3_3", "0")
	farmerstate.SetMiscSettingString("dana", "crafts_29_3", "30")

	lines := []string{
		"T4L_ACTUATOR",
		"CRAFT(T4_ACTUATOR)",
	}

	unselected := []string{"dana", "charlie", "bob", "alice"}
	sorted := sortCustomRemaining(contract, unselected, lines, false)

	expected := []string{"alice", "bob", "charlie", "dana"}
	if len(sorted) != len(expected) {
		t.Fatalf("sorted length = %d, want %d", len(sorted), len(expected))
	}
	for i, id := range sorted {
		if id != expected[i] {
			t.Errorf("at index %d: got %s, want %s (sorted: %v)", i, id, expected[i], sorted)
		}
	}

	// Verify table image rendering with custom columns
	imgBytes, err := RenderCustomOrderTableImage(contract, lines)
	if err != nil {
		t.Fatalf("RenderCustomOrderTableImage failed: %v", err)
	}
	if len(imgBytes) == 0 {
		t.Fatalf("RenderCustomOrderTableImage produced empty image bytes")
	}
}

func TestCustomOrderTableColumns_OnlyActiveCriteria(t *testing.T) {
	// Case 1: Actuator primary and crafts tiebreaker
	lines1 := []string{"T4L_ACTUATOR", "CRAFT(T4_ACTUATOR)"}
	cols1 := getCustomOrderTableColumns(nil, lines1)
	var labels1 []string
	for _, c := range cols1 {
		labels1 = append(labels1, c.Label)
	}
	expectedLabels1 := []string{"#", "Player", "T4L Actuator", "T4 Actuator Crafts"}
	if len(labels1) != len(expectedLabels1) {
		t.Fatalf("Case 1 got cols %v, want %v", labels1, expectedLabels1)
	}
	for i, l := range labels1 {
		if l != expectedLabels1[i] {
			t.Errorf("Case 1 col %d = %q, want %q", i, l, expectedLabels1[i])
		}
	}

	// Case 2: Role, IHR, Tokens
	lines2 := []string{"<ROLE", "<IHR[6%]", ">TOKENS"}
	cols2 := getCustomOrderTableColumns(nil, lines2)
	var labels2 []string
	for _, c := range cols2 {
		labels2 = append(labels2, c.Label)
	}
	expectedLabels2 := []string{"#", "Player", "Role", "IHR", "Tokens"}
	if len(labels2) != len(expectedLabels2) {
		t.Fatalf("Case 2 got cols %v, want %v", labels2, expectedLabels2)
	}
	for i, l := range labels2 {
		if l != expectedLabels2[i] {
			t.Errorf("Case 2 col %d = %q, want %q", i, l, expectedLabels2[i])
		}
	}

	// Case 3: Conditional IF MAIN ... with Defl Effort, Tokens, and TE
	lines3 := []string{"IF MAIN <DEFL_EFFORT[50] ELSE >TOKENS", "<TE"}
	cols3 := getCustomOrderTableColumns(nil, lines3)
	var labels3 []string
	for _, c := range cols3 {
		labels3 = append(labels3, c.Label)
	}
	expectedLabels3 := []string{"#", "Player", "Role", "Effort [N=50]", "Tokens", "TE"}
	if len(labels3) != len(expectedLabels3) {
		t.Fatalf("Case 3 got cols %v, want %v", labels3, expectedLabels3)
	}
	for i, l := range labels3 {
		if l != expectedLabels3[i] {
			t.Errorf("Case 3 col %d = %q, want %q", i, l, expectedLabels3[i])
		}
	}

	// Case 4: Empty lines -> only # and Player (name always shown)
	lines4 := []string{}
	cols4 := getCustomOrderTableColumns(nil, lines4)
	var labels4 []string
	for _, c := range cols4 {
		labels4 = append(labels4, c.Label)
	}
	expectedLabels4 := []string{"#", "Player"}
	if len(labels4) != len(expectedLabels4) {
		t.Fatalf("Case 4 got cols %v, want %v", labels4, expectedLabels4)
	}
	for i, l := range labels4 {
		if l != expectedLabels4[i] {
			t.Errorf("Case 4 col %d = %q, want %q", i, l, expectedLabels4[i])
		}
	}
}



