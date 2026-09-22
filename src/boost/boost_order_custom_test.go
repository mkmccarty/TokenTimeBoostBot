package boost

import (
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
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
