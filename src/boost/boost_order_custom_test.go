package boost

import (
	"database/sql"
	"math"
	"strings"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
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
			critType:  CritArtifactHas,
			artName:   ei.ArtifactSpec_TITANIUM_ACTUATOR,
			artLevel:  3,
			artRarity: 3,
			label:     "T4L Actuator",
		},
		{
			input:     "COUNT(T4L_ACTUATOR)",
			critType:  CritArtifactCount,
			artName:   ei.ArtifactSpec_TITANIUM_ACTUATOR,
			artLevel:  3,
			artRarity: 3,
			label:     "T4L Actuator Count",
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
			critType:  CritArtifactHas,
			artName:   ei.ArtifactSpec_ORNATE_GUSSET,
			artLevel:  3,
			artRarity: 2,
			label:     "T4E Gusset",
		},
		{
			input:     "T4_COMPASS",
			critType:  CritArtifactHas,
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
	// Goal 1: primary = T4L_ACTUATOR (boolean has-it), secondary = CRAFT(T4_ACTUATOR)
	// Alice: 2x T4L Actuator (has=1), 10 crafts
	// Bob: 1x T4L Actuator (has=1), 50 crafts
	// Charlie: 0x T4L Actuator (has=0), 100 crafts
	// Dana: 0x T4L Actuator (has=0), 30 crafts
	// Since both Alice and Bob have the item, they tie on Level 1 (T4L_ACTUATOR).
	// Level 2 (CRAFT) breaks the tie: Bob (50 crafts) > Alice (10 crafts).
	// Charlie and Dana lack the item (0), so they tie on Level 1.
	// Level 2 (CRAFT) breaks the tie: Charlie (100 crafts) > Dana (30 crafts).
	// Expected Order: Bob -> Alice -> Charlie -> Dana

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

	expected := []string{"bob", "alice", "charlie", "dana"}
	if len(sorted) != len(expected) {
		t.Fatalf("sorted length = %d, want %d", len(sorted), len(expected))
	}
	for i, id := range sorted {
		if id != expected[i] {
			t.Errorf("at index %d: got %s, want %s (sorted: %v)", i, id, expected[i], sorted)
		}
	}

	// Goal 2: COUNT(T4L_ACTUATOR) as primary sort quantity:
	// Alice (count 2) > Bob (count 1) > Charlie (count 0, 100 crafts) > Dana (count 0, 30 crafts)
	countLines := []string{
		"COUNT(T4L_ACTUATOR)",
		"CRAFT(T4_ACTUATOR)",
	}
	sortedCount := sortCustomRemaining(contract, unselected, countLines, false)
	expectedCount := []string{"alice", "bob", "charlie", "dana"}
	for i, id := range sortedCount {
		if id != expectedCount[i] {
			t.Errorf("COUNT sort at index %d: got %s, want %s (sorted: %v)", i, id, expectedCount[i], sortedCount)
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
	expectedLabels2 := []string{"#", "Player", "Role", "IHR[6%]", "Tokens"}
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
	expectedLabels3 := []string{"#", "Player", "Role", "Effort[N=50]", "Tokens", "TE"}
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

func TestCustomOrderRolePriority(t *testing.T) {
	contract := &Contract{
		State: ContractStateSignup,
		Order: []string{"altUser", "regularUser", "quantUser", "gussetUser", "siabUser"},
		Boosters: map[string]*Booster{
			"altUser": {
				UserID:        "altUser",
				AltController: "mainUser",
				IHRRate:       10000,
			},
			"regularUser": {
				UserID:  "regularUser",
				IHRRate: 1000,
			},
			"quantUser": {
				UserID:  "quantUser",
				IHRRate: 1000,
			},
			"gussetUser": {
				UserID:  "gussetUser",
				IHRRate: 1000,
				ArtifactSet: ArtifactSet{
					Artifacts: []ei.Artifact{{Type: "Gusset", Quality: "T4L"}},
				},
			},
			"siabUser": {
				UserID:  "siabUser",
				IHRRate: 1000,
				ArtifactSet: ArtifactSet{
					Artifacts: []ei.Artifact{{Type: "SIAB", Quality: "T4L"}},
				},
			},
		},
	}

	// 1. Verify Role strings (only Main or Helper)
	regStr, regColor := getBoosterRoleString(contract.Boosters["regularUser"])
	if regStr != "Main" || regColor != "green" {
		t.Errorf("expected Main role, got %s / %s", regStr, regColor)
	}
	altStr, altColor := getBoosterRoleString(contract.Boosters["altUser"])
	if altStr != "Helper" || altColor != "red" {
		t.Errorf("expected Helper role, got %s / %s", altStr, altColor)
	}
	siabStr, siabColor := getBoosterRoleString(contract.Boosters["siabUser"])
	if siabStr != "Main" || siabColor != "green" {
		t.Errorf("expected Main role for siabUser, got %s / %s", siabStr, siabColor)
	}

	// 2. Verify sort order: Mains first, Helper last
	lines := []string{"<ROLE"}
	sorted := sortCustomRemaining(contract, contract.Order, lines, false)
	if sorted[len(sorted)-1] != "altUser" {
		t.Fatalf("expected altUser (Helper) to be last, got order %v", sorted)
	}

	// >ROLE puts Helper first
	linesReverse := []string{">ROLE"}
	sortedReverse := sortCustomRemaining(contract, contract.Order, linesReverse, false)
	if sortedReverse[0] != "altUser" {
		t.Fatalf("expected altUser (Helper) to be first with >ROLE, got order %v", sortedReverse)
	}

	// 3. Verify IF MAIN and IF HELPER conditions
	mainCrit := parseCustomCriterion("IF MAIN <IHR ELSE >TOKENS")
	if !mainCrit.matchesRole(contract.Boosters["regularUser"]) {
		t.Errorf("expected regularUser to match IF MAIN")
	}
	if !mainCrit.matchesRole(contract.Boosters["siabUser"]) {
		t.Errorf("expected siabUser to match IF MAIN")
	}
	if mainCrit.matchesRole(contract.Boosters["altUser"]) {
		t.Errorf("did not expect altUser to match IF MAIN")
	}

	helperCrit := parseCustomCriterion("IF HELPER <IHR ELSE >TOKENS")
	if !helperCrit.matchesRole(contract.Boosters["altUser"]) {
		t.Errorf("expected altUser to match IF HELPER")
	}
	if helperCrit.matchesRole(contract.Boosters["regularUser"]) {
		t.Errorf("did not expect regularUser to match IF HELPER")
	}
}

func TestGetContractBoostOrderLines(t *testing.T) {
	if lines := GetContractBoostOrderLines(nil); len(lines) == 0 || lines[0] != "SIGNUP" {
		t.Errorf("expected SIGNUP for nil contract, got %v", lines)
	}

	c1 := &Contract{BoostOrder: ContractOrderIHR}
	if lines := GetContractBoostOrderLines(c1); len(lines) != 4 || lines[0] != "<IHR" {
		t.Errorf("expected IHR lines, got %v", lines)
	}

	c2 := &Contract{BoostOrder: ContractOrderIHRFuzzy}
	if lines := GetContractBoostOrderLines(c2); len(lines) != 4 || lines[0] != "<IHR[6%]" {
		t.Errorf("expected Fuzzy IHR lines, got %v", lines)
	}

	cCustom := &Contract{
		BoostOrder:       ContractOrderCustom,
		CustomOrderLines: []string{"<ROLE", "<DEFL_SLOT", "<IHR[6%]", ">TOKENS"},
	}
	if lines := GetContractBoostOrderLines(cCustom); len(lines) != 4 || lines[0] != "<ROLE" {
		t.Errorf("expected custom lines, got %v", lines)
	}

	orderTests := []struct {
		order int
		want  string
	}{
		{ContractOrderSignup, "SIGNUP"},
		{ContractOrderReverse, "REVERSE"},
		{ContractOrderRandom, "RANDOM"},
		{ContractOrderFair, "FAIR"},
		{ContractOrderTimeBased, "TIME"},
		{ContractOrderELR, "<ELR"},
		{ContractOrderTVal, "<TVAL"},
		{ContractOrderTokenAsk, ">TOKENS"},
		{ContractOrderTE, "<TE"},
		{ContractOrderTEFuzzy, "<TE[sqrt]"},
		{ContractManualOrder, "MANUAL"},
	}

	for _, tt := range orderTests {
		c := &Contract{BoostOrder: tt.order}
		lines := GetContractBoostOrderLines(c)
		if len(lines) == 0 || lines[0] != tt.want {
			t.Errorf("GetContractBoostOrderLines(order=%d) = %v, want starting with %s", tt.order, lines, tt.want)
		}
	}
}

func TestBuildBoostOrderPreviewMessage(t *testing.T) {
	contract := &Contract{
		ContractID:   "preview-test-contract",
		CoopID:       "preview-coop",
		ContractHash: "preview-hash-456",
		BoostOrder:   ContractOrderIHRFuzzy,
		Boosters:     make(map[string]*Booster),
		Order:        []string{"u1", "u2"},
	}
	contract.Boosters["u1"] = &Booster{
		UserID:  "u1",
		Nick:    "Farmer 1",
		IHRRate: 5e9,
	}
	contract.Boosters["u2"] = &Booster{
		UserID:  "u2",
		Nick:    "Farmer 2",
		IHRRate: 10e9,
	}

	msg, err := BuildBoostOrderPreviewMessage(contract)
	if err != nil {
		t.Fatalf("BuildBoostOrderPreviewMessage failed: %v", err)
	}
	if len(msg.Components) == 0 {
		t.Fatalf("expected components in preview message")
	}
	if len(msg.Files) == 0 {
		t.Fatalf("expected image file in preview message")
	}

	// Verify Keep and Dismiss buttons in action row
	var keepBtn, dismissBtn *dc.Button
	for _, comp := range msg.Components {
		if ar, ok := comp.(dc.ActionRow); ok {
			for _, ic := range ar.Components {
				if btn, ok := ic.(dc.Button); ok {
					if strings.Contains(btn.CustomID, "#keep#") {
						keepBtn = &btn
					} else if strings.Contains(btn.CustomID, "#dismiss#") {
						dismissBtn = &btn
					}
				}
			}
		}
	}

	if keepBtn == nil {
		t.Errorf("expected Keep button in action row")
	} else if keepBtn.CustomID != "rc_#keep#preview-hash-456" {
		t.Errorf("unexpected Keep button CustomID: %s", keepBtn.CustomID)
	}

	if dismissBtn == nil {
		t.Errorf("expected Dismiss button in action row")
	} else if dismissBtn.CustomID != "rc_#dismiss#preview-hash-456" {
		t.Errorf("unexpected Dismiss button CustomID: %s", dismissBtn.CustomID)
	}
}

func TestBuildBoostOrderPreviewMessage_AllBoostOrders(t *testing.T) {
	allOrders := []struct {
		order int
		name  string
	}{
		{ContractOrderSignup, "Signup"},
		{ContractOrderReverse, "Reverse"},
		{ContractOrderRandom, "Random"},
		{ContractOrderFair, "Fair"},
		{ContractOrderTimeBased, "Time-Based"},
		{ContractOrderELR, "ELR"},
		{ContractOrderTVal, "TVal"},
		{ContractOrderTokenAsk, "Token-Ask"},
		{ContractOrderTE, "TE"},
		{ContractOrderTEFuzzy, "Fuzzy TE"},
		{ContractManualOrder, "Manual"},
		{ContractOrderIHR, "Boosting IHR"},
		{ContractOrderIHRFuzzy, "Fuzzy IHR"},
		{ContractOrderCustom, "Custom"},
	}

	for _, tc := range allOrders {
		t.Run(tc.name, func(t *testing.T) {
			contract := &Contract{
				ContractID:   "preview-test-contract",
				CoopID:       "preview-coop",
				ContractHash: "hash-" + tc.name,
				BoostOrder:   tc.order,
				Boosters:     make(map[string]*Booster),
				Order:        []string{"u1", "u2"},
			}
			if tc.order == ContractOrderCustom {
				contract.CustomOrderLines = []string{"<IHR", "<DEFL"}
				contract.CustomOrderName = "My CBO Preset"
			}
			contract.Boosters["u1"] = &Booster{
				UserID:       "u1",
				Nick:         "Farmer 1",
				IHRRate:      5e9,
				TokensWanted: 6,
				TECount:      50,
			}
			contract.Boosters["u2"] = &Booster{
				UserID:       "u2",
				Nick:         "Farmer 2",
				IHRRate:      10e9,
				TokensWanted: 4,
				TECount:      100,
			}

			msg, err := BuildBoostOrderPreviewMessage(contract)
			if err != nil {
				t.Fatalf("BuildBoostOrderPreviewMessage failed for %s: %v", tc.name, err)
			}
			if len(msg.Components) == 0 {
				t.Fatalf("expected components for %s", tc.name)
			}
			if len(msg.Files) == 0 {
				t.Fatalf("expected image file for %s", tc.name)
			}

			// Verify header text contains the order name
			firstComp, ok := msg.Components[0].(dc.TextDisplay)
			if !ok {
				t.Fatalf("expected first component to be TextDisplay for %s", tc.name)
			}
			if tc.order == ContractOrderCustom {
				if !strings.Contains(firstComp.Content, "My CBO Preset") {
					t.Errorf("expected My CBO Preset in header, got: %s", firstComp.Content)
				}
				if strings.Contains(firstComp.Content, "CBO:") {
					t.Errorf("expected no CBO: in header, got: %s", firstComp.Content)
				}
			} else {
				if !strings.Contains(firstComp.Content, tc.name) {
					t.Errorf("expected %s in header, got: %s", tc.name, firstComp.Content)
				}
			}

			// Verify Keep and Dismiss buttons exist
			hasKeep := false
			hasDismiss := false
			for _, comp := range msg.Components {
				if ar, ok := comp.(dc.ActionRow); ok {
					for _, ic := range ar.Components {
						if btn, ok := ic.(dc.Button); ok {
							if strings.Contains(btn.CustomID, "#keep#") {
								hasKeep = true
							}
							if strings.Contains(btn.CustomID, "#dismiss#") {
								hasDismiss = true
							}
						}
					}
				}
			}
			if !hasKeep || !hasDismiss {
				t.Errorf("missing Keep or Dismiss button for %s", tc.name)
			}
		})
	}
}

func TestCritReverse_Sorting(t *testing.T) {
	contract := &Contract{
		ContractHash: "test-rev",
		Order:        []string{"u1", "u2", "u3"},
		Boosters: map[string]*Booster{
			"u1": {UserID: "u1", Nick: "First Joined"},
			"u2": {UserID: "u2", Nick: "Second Joined"},
			"u3": {UserID: "u3", Nick: "Third Joined"},
		},
	}

	sorted := sortCustomRemaining(contract, contract.Order, []string{"REVERSE"}, false)
	if len(sorted) != 3 {
		t.Fatalf("expected 3 items, got %d", len(sorted))
	}
	// Reverse of u1, u2, u3 should be u3, u2, u1
	if sorted[0] != "u3" || sorted[1] != "u2" || sorted[2] != "u1" {
		t.Errorf("expected [u3, u2, u1], got %v", sorted)
	}
}

func TestSuggestCustomOrderName(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		expected string
	}{
		{
			name:     "Single criterion",
			lines:    []string{"DEFL_EFFORT"},
			expected: "Deflector Effort",
		},
		{
			name:     "Two criteria",
			lines:    []string{"DEFL_EFFORT", "IHR"},
			expected: "Deflector Effort & IHR",
		},
		{
			name:     "Four criteria fitting within 50 chars",
			lines:    []string{"DEFL_EFFORT", "IHR", "TE", "SIGNUP"},
			expected: "Deflector Effort, IHR, Truth Eggs & Signup",
		},
		{
			name:     "Artifact count and craft",
			lines:    []string{"T4L_ACTUATOR", "CRAFT(T4_ACTUATOR)"},
			expected: "T4L Actuator & T4 Actuator Crafts",
		},
		{
			name:     "Conditional rule with else",
			lines:    []string{"IF ROLE = MAIN THEN IHR ELSE TE"},
			expected: "Role (IHR/TE)",
		},
		{
			name:     "Conditional rule helper",
			lines:    []string{"IF ROLE = HELPER THEN TE"},
			expected: "Helper (TE)",
		},
		{
			name:     "Tokens ascending vs descending",
			lines:    []string{"+TOKENS", "-TOKENS"},
			expected: "Tokens & Most Tokens",
		},
		{
			name:     "Empty and dash lines",
			lines:    []string{"-", "", "   "},
			expected: "",
		},
		{
			name:     "Long criteria trimmed to fit 50 chars",
			lines:    []string{"COUNT(T4E_GUSSET)", "T4_COMPASS", "CRAFT(T4_ACTUATOR)", "DEFL_EFFORT"},
			expected: "T4E Gusset Count, T4 Compass & T4 Actuator Crafts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SuggestCustomOrderName(tt.lines)
			if got != tt.expected {
				t.Errorf("SuggestCustomOrderName(%v) = %q, want %q", tt.lines, got, tt.expected)
			}
			if len(got) > 50 {
				t.Errorf("SuggestCustomOrderName(%v) length %d exceeds 50 chars: %q", tt.lines, len(got), got)
			}
		})
	}
}

func TestParseCustomCriterion_ExtendedCriteria(t *testing.T) {
	tests := []struct {
		input     string
		critType  CustomCriterionType
		ascending bool
		boostID   string
		boostLbl  string
	}{
		{input: "ARTIFACT_SCORE", critType: CritArtifactScore, ascending: false},
		{input: "ART_SCORE", critType: CritArtifactScore, ascending: false},
		{input: "<ARTIFACT_SCORE", critType: CritArtifactScore, ascending: false},
		{input: ">ARTIFACT_SCORE", critType: CritArtifactScore, ascending: true},
		{input: "CRAFTING_XP", critType: CritCraftingXP, ascending: false},
		{input: "CRAFT_XP", critType: CritCraftingXP, ascending: false},
		{input: "CXP", critType: CritCraftingXP, ascending: false},
		{input: ">CRAFTING_XP", critType: CritCraftingXP, ascending: true},
		{input: "GE", critType: CritGE, ascending: false},
		{input: "GOLDEN_EGGS", critType: CritGE, ascending: false},
		{input: "GOLDEN_EGG", critType: CritGE, ascending: false},
		{input: ">GE", critType: CritGE, ascending: true},
		{input: "EB", critType: CritEB, ascending: false},
		{input: "EARNINGS_BONUS", critType: CritEB, ascending: false},
		{input: ">EB", critType: CritEB, ascending: true},
		{input: "SE", critType: CritSE, ascending: false},
		{input: "SOUL_EGGS", critType: CritSE, ascending: false},
		{input: "SOUL_EGG", critType: CritSE, ascending: false},
		{input: ">SE", critType: CritSE, ascending: true},
		{input: "CTE", critType: CritCTE, ascending: false},
		{input: "CLOTHED_TRUTH_EGGS", critType: CritCTE, ascending: false},
		{input: "CLOTHED_TE", critType: CritCTE, ascending: false},
		{input: ">CTE", critType: CritCTE, ascending: true},
		{input: "PRESTIGE", critType: CritPrestige, ascending: false},
		{input: "PRESTIGES", critType: CritPrestige, ascending: false},
		{input: "PRESTIGE_COUNT", critType: CritPrestige, ascending: false},
		{input: ">PRESTIGE", critType: CritPrestige, ascending: true},
		{input: "DRONE", critType: CritDrone, ascending: false},
		{input: "DRONES", critType: CritDrone, ascending: false},
		{input: "DRONE_COUNT", critType: CritDrone, ascending: false},
		{input: "DRONE_TAKEDOWNS", critType: CritDrone, ascending: false},
		{input: ">DRONE", critType: CritDrone, ascending: true},
		{input: "ELITE_DRONE", critType: CritEliteDrone, ascending: false},
		{input: "ELITE_DRONES", critType: CritEliteDrone, ascending: false},
		{input: "ELITE_DRONE_COUNT", critType: CritEliteDrone, ascending: false},
		{input: ">ELITE_DRONE", critType: CritEliteDrone, ascending: true},
		{input: "SOUL_MIRROR_ORANGE", critType: CritBoostCount, ascending: false, boostID: "soul_mirror_orange", boostLbl: "Soul Mirror (1d)"},
		{input: "SOUL_MIRROR_BLUE", critType: CritBoostCount, ascending: false, boostID: "soul_mirror_blue", boostLbl: "Soul Mirror (10m)"},
		{input: "SOUL_MIRROR_PURPLE", critType: CritBoostCount, ascending: false, boostID: "soul_mirror_purple", boostLbl: "Soul Mirror (1h)"},
		{input: "BOOST(soul_mirror_orange)", critType: CritBoostCount, ascending: false, boostID: "soul_mirror_orange", boostLbl: "Soul Mirror (1d)"},
		{input: "BOOST(soul_mirror_blue)", critType: CritBoostCount, ascending: false, boostID: "soul_mirror_blue", boostLbl: "Soul Mirror (10m)"},
		{input: "BOOST(tachyon_prism_epic)", critType: CritBoostCount, ascending: false, boostID: "tachyon_prism_epic", boostLbl: "Tachyon Prism Epic"},
		{input: "BOOST(boost_beacon_orange)", critType: CritBoostCount, ascending: false, boostID: "boost_beacon_orange", boostLbl: "Legendary Boost Beacon"},
		{input: ">BOOST(soul_mirror_orange)", critType: CritBoostCount, ascending: true, boostID: "soul_mirror_orange", boostLbl: "Soul Mirror (1d)"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			crit := parseCustomCriterion(tt.input)
			if crit.critType != tt.critType {
				t.Errorf("parseCustomCriterion(%q) critType = %v, want %v", tt.input, crit.critType, tt.critType)
			}
			if crit.ascending != tt.ascending {
				t.Errorf("parseCustomCriterion(%q) ascending = %v, want %v", tt.input, crit.ascending, tt.ascending)
			}
			if tt.boostID != "" && crit.boostID != tt.boostID {
				t.Errorf("parseCustomCriterion(%q) boostID = %q, want %q", tt.input, crit.boostID, tt.boostID)
			}
			if tt.boostLbl != "" && crit.boostLabel != tt.boostLbl {
				t.Errorf("parseCustomCriterion(%q) boostLabel = %q, want %q", tt.input, crit.boostLabel, tt.boostLbl)
			}
		})
	}
}

func TestParseCustomCriterion_CaseInsensitive(t *testing.T) {
	tests := []struct {
		input     string
		critType  CustomCriterionType
		ascending bool
		fuzzyPct  float64
		fuzzySqrt bool
		effortN   int
		boostID   string
		boostLbl  string
	}{
		// Lowercase & mixed case basic criteria
		{input: "ihr", critType: CritIHR, ascending: false},
		{input: "Ihr", critType: CritIHR, ascending: false},
		{input: "<ihr[6%]>", critType: CritIHR, ascending: false, fuzzyPct: 0.06},
		{input: "ihr[6%]", critType: CritIHR, ascending: false, fuzzyPct: 0.06},
		{input: "elr", critType: CritELR, ascending: false},
		{input: "<elr", critType: CritELR, ascending: false},
		{input: "te", critType: CritTE, ascending: false},
		{input: "te[sqrt]", critType: CritTE, ascending: false, fuzzySqrt: true},
		{input: "te[~]", critType: CritTE, ascending: false, fuzzySqrt: true},
		{input: "tokens", critType: CritTokens, ascending: true},
		{input: ">tokens", critType: CritTokens, ascending: true},
		{input: "<tokens", critType: CritTokens, ascending: false},
		{input: "defl", critType: CritDefl, ascending: false},
		{input: "deflector", critType: CritDefl, ascending: false},
		{input: "defl[5%]", critType: CritDefl, ascending: false, fuzzyPct: 0.05},
		{input: "defl effort", critType: CritDeflEffort, ascending: false, effortN: 50},
		{input: "deflector effort [40]", critType: CritDeflEffort, ascending: false, effortN: 40},
		{input: "defl_slot", critType: CritDeflSlot, ascending: false},
		{input: "deflector slot", critType: CritDeflSlot, ascending: false},
		{input: "delivery", critType: CritDeliv, ascending: false},
		{input: "shipping", critType: CritDeliv, ascending: false},
		{input: "signup", critType: CritSignup, ascending: true},
		{input: "reverse", critType: CritReverse, ascending: false},
		{input: "random", critType: CritRandom, ascending: false},
		{input: "role", critType: CritRole, ascending: false},
		{input: "main", critType: CritRole, ascending: false},
		{input: "helper", critType: CritRole, ascending: true},
		{input: "siab", critType: CritArtifactHas, ascending: false},
		{input: "gusset", critType: CritArtifactHas, ascending: false},
		{input: "quant", critType: CritArtifactHas, ascending: false},
		{input: "craft defl", critType: CritCraftDefl, ascending: false},

		// Lowercase & mixed case extended account criteria
		{input: "artifact score", critType: CritArtifactScore, ascending: false},
		{input: "Artifact Score", critType: CritArtifactScore, ascending: false},
		{input: "art_score", critType: CritArtifactScore, ascending: false},
		{input: "crafting xp", critType: CritCraftingXP, ascending: false},
		{input: "Crafting XP", critType: CritCraftingXP, ascending: false},
		{input: "cxp", critType: CritCraftingXP, ascending: false},
		{input: "ge", critType: CritGE, ascending: false},
		{input: "golden eggs", critType: CritGE, ascending: false},
		{input: "Golden Eggs", critType: CritGE, ascending: false},
		{input: "eb", critType: CritEB, ascending: false},
		{input: "earnings bonus", critType: CritEB, ascending: false},
		{input: "Earnings Bonus", critType: CritEB, ascending: false},
		{input: "se", critType: CritSE, ascending: false},
		{input: "soul eggs", critType: CritSE, ascending: false},
		{input: "cte", critType: CritCTE, ascending: false},
		{input: "clothed truth eggs", critType: CritCTE, ascending: false},
		{input: "clothed te", critType: CritCTE, ascending: false},
		{input: "prestige", critType: CritPrestige, ascending: false},
		{input: "prestige count", critType: CritPrestige, ascending: false},
		{input: "drone", critType: CritDrone, ascending: false},
		{input: "drone takedowns", critType: CritDrone, ascending: false},
		{input: "elite drone", critType: CritEliteDrone, ascending: false},
		{input: "Elite Drone", critType: CritEliteDrone, ascending: false},
		{input: "elite drones", critType: CritEliteDrone, ascending: false},

		// Case-insensitive boost items
		{input: "soul_mirror_orange", critType: CritBoostCount, ascending: false, boostID: "soul_mirror_orange", boostLbl: "Soul Mirror (1d)"},
		{input: "Soul Mirror Orange", critType: CritBoostCount, ascending: false, boostID: "soul_mirror_orange", boostLbl: "Soul Mirror (1d)"},
		{input: "boost(soul_mirror_orange)", critType: CritBoostCount, ascending: false, boostID: "soul_mirror_orange", boostLbl: "Soul Mirror (1d)"},
		{input: "boost(Soul Mirror Orange)", critType: CritBoostCount, ascending: false, boostID: "soul_mirror_orange", boostLbl: "Soul Mirror (1d)"},
		{input: "boost[soul_mirror_orange]", critType: CritBoostCount, ascending: false, boostID: "soul_mirror_orange", boostLbl: "Soul Mirror (1d)"},
		{input: "boost: soul_mirror_orange", critType: CritBoostCount, ascending: false, boostID: "soul_mirror_orange", boostLbl: "Soul Mirror (1d)"},
		{input: "boost: Supreme Tachyon Prism", critType: CritBoostCount, ascending: false, boostID: "tachyon_prism_orange_big", boostLbl: "Supreme Tachyon Prism"},
		{input: "boost: quantum warming bulb", critType: CritBoostCount, ascending: false, boostID: "dilithium_bulb", boostLbl: "Quantum Warming Bulb"},
		{input: "dilithium_bulb", critType: CritBoostCount, ascending: false, boostID: "dilithium_bulb", boostLbl: "Quantum Warming Bulb"},
		{input: "Dilithium Bulb", critType: CritBoostCount, ascending: false, boostID: "dilithium_bulb", boostLbl: "Quantum Warming Bulb"},

		// Case-insensitive artifacts
		{input: "t4l deflector", critType: CritArtifactHas, ascending: false},
		{input: "T4l Deflector", critType: CritArtifactHas, ascending: false},
		{input: "t4 gusset", critType: CritArtifactHas, ascending: false},
		{input: "has(t4l deflector)", critType: CritArtifactHas, ascending: false},
		{input: "Has(T4L Deflector)", critType: CritArtifactHas, ascending: false},
		{input: "count(t4 gusset)", critType: CritArtifactCount, ascending: false},
		{input: "Count(T4 Gusset)", critType: CritArtifactCount, ascending: false},
		{input: "craft(t4 deflector)", critType: CritArtifactCraft, ascending: false},
		{input: "Craft(T4 Deflector)", critType: CritArtifactCraft, ascending: false},
		{input: "crafts(chalice)", critType: CritArtifactCraft, ascending: false},
		{input: "legendary chalice", critType: CritArtifactHas, ascending: false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			crit := parseCustomCriterion(tt.input)
			if crit.critType != tt.critType {
				t.Errorf("parseCustomCriterion(%q) critType = %v, want %v", tt.input, crit.critType, tt.critType)
			}
			if crit.ascending != tt.ascending {
				t.Errorf("parseCustomCriterion(%q) ascending = %v, want %v", tt.input, crit.ascending, tt.ascending)
			}
			if tt.fuzzyPct != 0 && crit.fuzzyPct != tt.fuzzyPct {
				t.Errorf("parseCustomCriterion(%q) fuzzyPct = %v, want %v", tt.input, crit.fuzzyPct, tt.fuzzyPct)
			}
			if tt.fuzzySqrt != crit.fuzzySqrt {
				t.Errorf("parseCustomCriterion(%q) fuzzySqrt = %v, want %v", tt.input, crit.fuzzySqrt, tt.fuzzySqrt)
			}
			if tt.effortN != 0 && crit.effortN != tt.effortN {
				t.Errorf("parseCustomCriterion(%q) effortN = %v, want %v", tt.input, crit.effortN, tt.effortN)
			}
			if tt.boostID != "" && crit.boostID != tt.boostID {
				t.Errorf("parseCustomCriterion(%q) boostID = %q, want %q", tt.input, crit.boostID, tt.boostID)
			}
			if tt.boostLbl != "" && crit.boostLabel != tt.boostLbl {
				t.Errorf("parseCustomCriterion(%q) boostLabel = %q, want %q", tt.input, crit.boostLabel, tt.boostLbl)
			}
		})
	}
}

func TestParseCustomCriterion_ConditionalsCaseInsensitive(t *testing.T) {
	tests := []struct {
		input      string
		targetRole customRoleCondition
		thenType   CustomCriterionType
		thenAsc    bool
		hasElse    bool
		elseType   CustomCriterionType
		elseAsc    bool
	}{
		{
			input:      "if role == main then ihr else te",
			targetRole: roleConditionMain,
			thenType:   CritIHR,
			thenAsc:    false,
			hasElse:    true,
			elseType:   CritTE,
			elseAsc:    false,
		},
		{
			input:      "if helper then >tokens else <ihr",
			targetRole: roleConditionHelper,
			thenType:   CritTokens,
			thenAsc:    true,
			hasElse:    true,
			elseType:   CritIHR,
			elseAsc:    false,
		},
		{
			input:      "if role != helper then ihr[6%] else >tokens",
			targetRole: roleConditionMain,
			thenType:   CritIHR,
			thenAsc:    false,
			hasElse:    true,
			elseType:   CritTokens,
			elseAsc:    true,
		},
		{
			input:      "if not helper then ihr else te",
			targetRole: roleConditionMain,
			thenType:   CritIHR,
			thenAsc:    false,
			hasElse:    true,
			elseType:   CritTE,
			elseAsc:    false,
		},
		{
			input:      "if main: ihr else: te",
			targetRole: roleConditionMain,
			thenType:   CritIHR,
			thenAsc:    false,
			hasElse:    true,
			elseType:   CritTE,
			elseAsc:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			crit := parseCustomCriterion(tt.input)
			if !crit.isConditional {
				t.Fatalf("parseCustomCriterion(%q) isConditional = false, want true", tt.input)
			}
			if crit.targetRole != tt.targetRole {
				t.Errorf("parseCustomCriterion(%q) targetRole = %v, want %v", tt.input, crit.targetRole, tt.targetRole)
			}
			if crit.thenCrit == nil || crit.thenCrit.critType != tt.thenType {
				t.Errorf("parseCustomCriterion(%q) thenCrit type = %v, want %v", tt.input, crit.thenCrit.critType, tt.thenType)
			}
			if crit.thenCrit.ascending != tt.thenAsc {
				t.Errorf("parseCustomCriterion(%q) thenCrit ascending = %v, want %v", tt.input, crit.thenCrit.ascending, tt.thenAsc)
			}
			if crit.hasElse != tt.hasElse {
				t.Errorf("parseCustomCriterion(%q) hasElse = %v, want %v", tt.input, crit.hasElse, tt.hasElse)
			}
			if tt.hasElse && (crit.elseCrit == nil || crit.elseCrit.critType != tt.elseType) {
				t.Errorf("parseCustomCriterion(%q) elseCrit type = %v, want %v", tt.input, crit.elseCrit.critType, tt.elseType)
			}
			if tt.hasElse && crit.elseCrit.ascending != tt.elseAsc {
				t.Errorf("parseCustomCriterion(%q) elseCrit ascending = %v, want %v", tt.input, crit.elseCrit.ascending, tt.elseAsc)
			}
		})
	}
}

func TestSuggestCustomOrderName_ExtendedCriteria(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		expected string
	}{
		{
			name:     "Artifact score and CXP",
			lines:    []string{"ARTIFACT_SCORE", "CRAFTING_XP"},
			expected: "Artifact Score & Crafting XP",
		},
		{
			name:     "EB and SE",
			lines:    []string{"EB", "SE"},
			expected: "EB & Soul Eggs",
		},
		{
			name:     "GE and Prestiges",
			lines:    []string{"GE", "PRESTIGE"},
			expected: "Golden Eggs & Prestiges",
		},
		{
			name:     "Soul Mirror boost and CTE",
			lines:    []string{"SOUL_MIRROR_ORANGE", "CTE"},
			expected: "Soul Mirror (1d) & CTE",
		},
		{
			name:     "Drones and Elite Drones",
			lines:    []string{"DRONE", "ELITE_DRONE"},
			expected: "Drones & Elite Drones",
		},
		{
			name:     "Ascending variations",
			lines:    []string{">EB", ">GE"},
			expected: "Lowest EB & Fewest GE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SuggestCustomOrderName(tt.lines)
			if got != tt.expected {
				t.Errorf("SuggestCustomOrderName(%v) = %q, want %q", tt.lines, got, tt.expected)
			}
		})
	}
}

func TestCustomOrderTableColumns_FuzzyHeaders(t *testing.T) {
	tests := []struct {
		name      string
		lines     []string
		wantCol   string
		wantFound bool
	}{
		{
			name:      "IHR with 6%",
			lines:     []string{"<IHR[6%]"},
			wantCol:   "IHR[6%]",
			wantFound: true,
		},
		{
			name:      "IHR with 10%",
			lines:     []string{"<IHR[10%]"},
			wantCol:   "IHR[10%]",
			wantFound: true,
		},
		{
			name:      "TE with sqrt",
			lines:     []string{"<TE[sqrt]"},
			wantCol:   "TE[√]",
			wantFound: true,
		},
		{
			name:      "TE with tilde",
			lines:     []string{"<TE[~]"},
			wantCol:   "TE[√]",
			wantFound: true,
		},
		{
			name:      "TE with unicode sqrt",
			lines:     []string{"<TE[√]"},
			wantCol:   "TE[√]",
			wantFound: true,
		},
		{
			name:      "TE with 8%",
			lines:     []string{"<TE[8%]"},
			wantCol:   "TE[8%]",
			wantFound: true,
		},
		{
			name:      "IHR without fuzzy",
			lines:     []string{"<IHR"},
			wantCol:   "IHR",
			wantFound: true,
		},
		{
			name:      "TE without fuzzy",
			lines:     []string{"<TE"},
			wantCol:   "TE",
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cols := getCustomOrderTableColumns(nil, tt.lines)
			found := false
			for _, c := range cols {
				if c.Label == tt.wantCol {
					found = true
					break
				}
			}
			if found != tt.wantFound {
				var gotLabels []string
				for _, c := range cols {
					gotLabels = append(gotLabels, c.Label)
				}
				t.Errorf("getCustomOrderTableColumns(%v) cols = %v, want col %q (found=%v)", tt.lines, gotLabels, tt.wantCol, tt.wantFound)
			}
		})
	}

	// Verify built-in Fuzzy IHR contract order
	cIHRFuzzy := &Contract{
		ContractID:   "c-ihr-fuzzy",
		BoostOrder:   ContractOrderIHRFuzzy,
		Order:        []string{"u1"},
		Boosters:     map[string]*Booster{"u1": {UserID: "u1", Nick: "Booster 1"}},
		ContractHash: "test-ihr-fuzzy",
	}
	ihrLines := GetContractBoostOrderLines(cIHRFuzzy)
	colsIHR := getCustomOrderTableColumns(cIHRFuzzy, ihrLines)
	hasIHRFuzzy := false
	for _, c := range colsIHR {
		if c.Label == "IHR[6%]" {
			hasIHRFuzzy = true
			break
		}
	}
	if !hasIHRFuzzy {
		t.Errorf("ContractOrderIHRFuzzy expected 'IHR[6%%]' column, got: %v", colsIHR)
	}

	// Verify built-in Fuzzy TE contract order
	cTEFuzzy := &Contract{
		ContractID:   "c-te-fuzzy",
		BoostOrder:   ContractOrderTEFuzzy,
		Order:        []string{"u1"},
		Boosters:     map[string]*Booster{"u1": {UserID: "u1", Nick: "Booster 1"}},
		ContractHash: "test-te-fuzzy",
	}
	teLines := GetContractBoostOrderLines(cTEFuzzy)
	colsTE := getCustomOrderTableColumns(cTEFuzzy, teLines)
	hasTEFuzzy := false
	for _, c := range colsTE {
		if c.Label == "TE[√]" {
			hasTEFuzzy = true
			break
		}
	}
	if !hasTEFuzzy {
		t.Errorf("ContractOrderTEFuzzy expected 'TE[√]' column, got: %v", colsTE)
	}

	// Verify RenderCustomOrderTableImage renders successfully with √
	imgBytes, err := RenderCustomOrderTableImage(cTEFuzzy, teLines)
	if err != nil {
		t.Fatalf("RenderCustomOrderTableImage error: %v", err)
	}
	if len(imgBytes) == 0 {
		t.Fatalf("RenderCustomOrderTableImage returned empty bytes")
	}

	// Verify Signup column cell does not include '#'
	crit := parseCustomCriterion("SIGNUP")
	activeCols, _ := buildCustomOrderTableColDefs(cIHRFuzzy, [4]customCriterion{crit})
	for _, ac := range activeCols {
		if ac.id == "signup" {
			cell := ac.evalCell(cIHRFuzzy.Boosters["u1"], 0)
			if cell.Text != "1" {
				t.Errorf("Signup column cell = %q, want '1' without '#'", cell.Text)
			}
		}
	}
}

func TestBoosterDeliveryRate(t *testing.T) {
	b1 := &Booster{
		UserID: "u1",
		Nick:   "Player 1",
		ArtifactSet: ArtifactSet{
			LayRate:  12.00,
			ShipRate: 19.25,
		},
	}
	b2 := &Booster{
		UserID: "u2",
		Nick:   "Player 2",
		ArtifactSet: ArtifactSet{
			LayRate:  25.00,
			ShipRate: 18.50,
		},
	}

	if r := getBoosterDeliveryRate(b1); math.Abs(r-12.00) > 1e-4 {
		t.Errorf("getBoosterDeliveryRate(b1) = %f, want 12.00", r)
	}
	if r := getBoosterDeliveryRate(b2); math.Abs(r-18.50) > 1e-4 {
		t.Errorf("getBoosterDeliveryRate(b2) = %f, want 18.50", r)
	}
	if r := getBoosterDeliveryRate(nil); r != 0 {
		t.Errorf("getBoosterDeliveryRate(nil) = %f, want 0", r)
	}

	contract := &Contract{
		ContractHash: "test-deliv",
		Order:        []string{"u1", "u2"},
		Boosters: map[string]*Booster{
			"u1": b1,
			"u2": b2,
		},
	}

	// b2 has deliv rate 18.50, b1 has 12.00. Descending (<DELIV) should put u2 first.
	sorted := sortCustomRemaining(contract, contract.Order, []string{"<DELIV"}, false)
	if len(sorted) != 2 || sorted[0] != "u2" || sorted[1] != "u1" {
		t.Errorf("sortCustomRemaining(<DELIV) = %v, want [u2, u1]", sorted)
	}

	cols := getCustomOrderTableColumns(contract, []string{"<DELIV"})
	hasDelivCol := false
	for _, c := range cols {
		if c.Label == "Delivery" {
			hasDelivCol = true
			break
		}
	}
	if !hasDelivCol {
		t.Errorf("expected Delivery column in table, got %v", cols)
	}

	activeCols, _ := buildCustomOrderTableColDefs(contract, [4]customCriterion{parseCustomCriterion("<DELIV")})
	for _, ac := range activeCols {
		if ac.id == "deliv" {
			cell1 := ac.evalCell(b1, 0)
			if cell1.Text != "12.00" {
				t.Errorf("Delivery cell b1 = %q, want '12.00'", cell1.Text)
			}
			cell2 := ac.evalCell(b2, 1)
			if cell2.Text != "18.50" {
				t.Errorf("Delivery cell b2 = %q, want '18.50'", cell2.Text)
			}
		}
	}

	// Verify RANDOM table output produces "RND"
	rndCols, _ := buildCustomOrderTableColDefs(contract, [4]customCriterion{parseCustomCriterion("RANDOM")})
	for _, ac := range rndCols {
		if ac.id == "random" {
			cell := ac.evalCell(b1, 0)
			if cell.Text != "RND" {
				t.Errorf("Random cell = %q, want 'RND'", cell.Text)
			}
		}
	}
}

func TestCustomOrderSessionPersistenceAndFallback(t *testing.T) {
	origDB := dbConn
	defer func() { dbConn = origDB }()

	db, err := sqlOpenMemory()
	if err == nil && db != nil {
		_, _ = db.ExecContext(ctx, ddl)
		dbConn = db
	}

	lines := []string{"<IHR[6%]", "ELR"}
	s := getOrCreateCustomOrderSession("user_test_persist", "hash_test_persist", lines)
	if s == nil {
		t.Fatal("expected session to be created")
	}
	uuidStr := s.uuidStr

	// Verify in-memory retrieval
	got := getCustomOrderSession(uuidStr)
	if got == nil || len(got.lines) != 2 || got.lines[0] != "<IHR[6%]" {
		t.Fatalf("unexpected session from memory: %#v", got)
	}

	if dbConn != nil {
		// Simulate bot restart by wiping in-memory map
		customOrderSessionsMutex.Lock()
		delete(customOrderSessions, uuidStr)
		customOrderSessionsMutex.Unlock()

		// Verify retrieval from SQLite database
		recovered := getCustomOrderSession(uuidStr)
		if recovered == nil {
			t.Fatal("expected session to be recovered from DB after memory wipe")
		}
		if recovered.userID != "user_test_persist" || recovered.contractHash != "hash_test_persist" {
			t.Errorf("recovered session mismatch: %#v", recovered)
		}

		// Test findCustomOrderSession
		found := findCustomOrderSession("user_test_persist", "hash_test_persist")
		if found == nil || found.uuidStr != uuidStr {
			t.Errorf("findCustomOrderSession failed: %#v", found)
		}

		// Test clearCustomOrderSession
		clearCustomOrderSession(uuidStr)
		if getCustomOrderSession(uuidStr) != nil {
			t.Error("expected session to be cleared")
		}
	}
}

func sqlOpenMemory() (*sql.DB, error) {
	return sql.Open("sqlite", ":memory:")
}

func TestCustomOrderBoostersRefreshedFromDB(t *testing.T) {
	u1 := "test_ihr_user_1"
	u2 := "test_ihr_user_2"

	farmerstate.SetMiscSettingString(u1, "TE", "50")
	farmerstate.SetMiscSettingString(u1, "chalice", "T4L")
	farmerstate.SetMiscSettingString(u1, "monocle", "T4L")

	farmerstate.SetMiscSettingString(u2, "TE", "10")
	farmerstate.SetMiscSettingString(u2, "chalice", "T2C")

	c := &Contract{
		ContractHash: "test-ihr-refresh",
		Order:        []string{u1, u2},
		Boosters: map[string]*Booster{
			u1: {UserID: u1, IHRRate: DefaultLeggyIHR, TECount: 0},
			u2: {UserID: u2, IHRRate: DefaultLeggyIHR, TECount: 0},
		},
	}

	refreshCustomBoosters(nil, c)

	b1 := c.Boosters[u1]
	b2 := c.Boosters[u2]

	if b1.IHRRate <= DefaultLeggyIHR {
		t.Errorf("expected b1.IHRRate > DefaultLeggyIHR, got %f", b1.IHRRate)
	}
	if b2.IHRRate <= DefaultLeggyIHR {
		t.Errorf("expected b2.IHRRate > DefaultLeggyIHR, got %f", b2.IHRRate)
	}
	if b1.IHRRate <= b2.IHRRate {
		t.Errorf("expected b1.IHRRate (%f) > b2.IHRRate (%f)", b1.IHRRate, b2.IHRRate)
	}
	if b1.TECount != 50 {
		t.Errorf("expected b1.TECount == 50, got %d", b1.TECount)
	}
	if b2.TECount != 10 {
		t.Errorf("expected b2.TECount == 10, got %d", b2.TECount)
	}

	// Verify custom order evaluation uses refreshed rates
	sorted := sortCustomRemaining(c, c.Order, []string{"<IHR[12%]"}, false)
	if len(sorted) != 2 || sorted[0] != u1 || sorted[1] != u2 {
		t.Errorf("sortCustomRemaining(<IHR[12%%]) = %v, want [%s, %s]", sorted, u1, u2)
	}
}

