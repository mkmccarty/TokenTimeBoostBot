package boost

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"google.golang.org/protobuf/proto"
)

// Helper function for floating-point comparisons
func almostEqual(a, b float64) bool {
	return math.Abs(a-b) <= 0.05
}

func TestCalculateBoostTime(t *testing.T) {
	tokenRate := 13.0
	sixTokBoostTime := 20.0

	tests := []struct {
		name     string
		te       int
		tokens   int
		expected float64
	}{
		// User's initial test cases
		{"TE 0, 6tok", 0, 6, 97.61},
		{"TE 490, 0tok", 490, 0, 12.21},

		// Additional specific column checks
		{"TE 1, 8tok", 1, 8, 111.92},
		{"TE 10, 3tok", 10, 3, 317.55},
		{"TE 199, 2tok", 199, 2, 104.89},

		// Crossover point: TE 108 (5tok and 4tok are tied)
		{"TE 108, 5tok", 108, 5, 78.26},
		{"TE 108, 4tok", 108, 4, 78.26},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CalculateBoostTime(tc.te, tc.tokens, tokenRate, sixTokBoostTime)
			if !almostEqual(got, tc.expected) {
				t.Errorf("CalculateBoostTime(%d, %d, %.1f, %.1f) = %.2f; want %.2f",
					tc.te, tc.tokens, tokenRate, sixTokBoostTime, got, tc.expected)
			}
		})
	}
}

func TestFindFastestBoost(t *testing.T) {
	tokenRate := 13.0
	sixTokBoostTime := 20.0

	tests := []struct {
		name             string
		te               int
		expectedStrategy string
		expectedTime     float64
	}{
		// User's initial test case
		{"TE 40 (Optimal Strategy Shift to 5tok)", 40, "5tok", 91.08},

		// Range 1: 6tok (TE 0 to 39)
		{"TE 0 (Lower bound 6tok)", 0, "6tok", 97.61},
		{"TE 39 (Upper bound 6tok)", 39, "6tok", 91.30},

		// Range 2: 5tok (TE 40 to 108)
		{"TE 108 (Upper bound 5tok)", 108, "5tok", 78.26},

		// Range 3: 4tok (TE 109 to 289)
		{"TE 109 (Lower bound 4tok)", 109, "4tok", 78.00},
		{"TE 199 (Mid 4tok)", 199, "4tok", 62.62},

		// Range End: 0tok (TE 386 to 490)
		{"TE 490 (Upper bound 0tok)", 490, "0tok", 12.21},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FindFastestBoost(tc.te, tokenRate, sixTokBoostTime)

			if got.Strategy != tc.expectedStrategy {
				t.Errorf("FindFastestBoost(%d, ...).Strategy = %s; want %s",
					tc.te, got.Strategy, tc.expectedStrategy)
			}

			var expectedTokInt int
			_, _ = fmt.Sscanf(tc.expectedStrategy, "%dtok", &expectedTokInt)
			if got.Tokens != expectedTokInt {
				t.Errorf("FindFastestBoost(%d, ...).Tokens = %d; want %d",
					tc.te, got.Tokens, expectedTokInt)
			}

			expectedMult := calcBoostMulti(float64(expectedTokInt))
			if got.Multiplier != expectedMult {
				t.Errorf("FindFastestBoost(%d, ...).Multiplier = %.1f; want %.1f",
					tc.te, got.Multiplier, expectedMult)
			}

			if !almostEqual(got.TotalTime, tc.expectedTime) {
				t.Errorf("FindFastestBoost(%d, ...).TotalTime = %.2f; want %.2f",
					tc.te, got.TotalTime, tc.expectedTime)
			}
		})
	}

	// Tests with full artifact multipliers derived from log data
	// Log 1: TE=149, Collegg=1.05, Chalice=1.40, Monocle=1.30, Stones=1.4802 => artifactMult = 1.05 * 1.40 * 1.30 * 1.4802 = 2.82866
	// Log 2: TE=101, Collegg=1.05, Chalice=1.40, Monocle=1.30, Stones=9 (1.4233) => artifactMult = 1.05 * 1.40 * 1.30 * 1.4233 = 2.71992
	artifactTests := []struct {
		name             string
		te               int
		artifactMult     float64
		expectedTokens   int
		expectedStrategy string
	}{
		{
			name:             "TE 149 with artifacts (Collegg 1.05, T4L Chalice 1.4, T4L Monocle 1.3, 10 Stones 1.4802)",
			te:               149,
			artifactMult:     1.05 * 1.40 * 1.30 * 1.4802, // 2.82866
			expectedTokens:   4,
			expectedStrategy: "4tok",
		},
		{
			name:             "TE 101 with artifacts (Collegg 1.05, T4L Chalice 1.4, T4L Monocle 1.3, 9 Stones 1.4233)",
			te:               101,
			artifactMult:     1.05 * 1.40 * 1.30 * 1.4233, // 2.71992
			expectedTokens:   4,
			expectedStrategy: "4tok",
		},
	}

	for _, tc := range artifactTests {
		t.Run(tc.name, func(t *testing.T) {
			got := FindFastestBoost(tc.te, tokenRate, sixTokBoostTime, tc.artifactMult)
			if got.Tokens != tc.expectedTokens {
				t.Errorf("FindFastestBoost(%d, ..., %.4f).Tokens = %d (%s); want %d (%s)",
					tc.te, tc.artifactMult, got.Tokens, got.Strategy, tc.expectedTokens, tc.expectedStrategy)
			}
		})
	}
}

func TestGetPlayerBoostConfig(t *testing.T) {
	tests := []struct {
		name               string
		te                 float64
		expectedTokens     float64
		expectedMultiplier float64
	}{
		{"Low TE (1.01^0 = 1 <= 2) -> 6 tokens", 0.0, 6.0, 4080.0},
		{"Mid TE (1.01^70 = 2.006 > 2) -> 5 tokens", 70.0, 5.0, 2060.0},
		{"High TE (1.01^140 = 4.028 > 4) -> 4 tokens", 140.0, 4.0, 1040.0},
		{"Very High TE (TE 490 -> 0tok)", 490.0, 0.0, 50.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, mult := GetPlayerBoostConfig(tc.te)
			if tokens != tc.expectedTokens {
				t.Errorf("GetPlayerBoostConfig(%.1f) tokens = %.1f; want %.1f", tc.te, tokens, tc.expectedTokens)
			}
			if mult != tc.expectedMultiplier {
				t.Errorf("GetPlayerBoostConfig(%.1f) multiplier = %.1f; want %.1f", tc.te, mult, tc.expectedMultiplier)
			}
		})
	}
}

func TestGetBoostMultiplierForTokens(t *testing.T) {
	tests := []struct {
		tokens   float64
		expected float64
	}{
		{1.0, 80.0},
		{2.0, 140.0},
		{3.0, 260.0},
		{4.0, 1040.0},
		{5.0, 2060.0},
		{6.0, 4080.0},
		{8.0, 10300.0},
	}

	for _, tc := range tests {
		got := GetBoostMultiplierForTokens(tc.tokens)
		if got != tc.expected {
			t.Errorf("GetBoostMultiplierForTokens(%.1f) = %.1f; want %.1f", tc.tokens, got, tc.expected)
		}
	}
}

func TestGetContractEstimateString(t *testing.T) {
	if ei.EggIncContractsAll == nil {
		ei.EggIncContractsAll = make(map[string]ei.EggIncContract)
	}

	// 1. Non-existent contract
	res := GetContractEstimateString("non-existent-contract-id", false)
	if res != "No contract found in this channel, use the command parameters to pick one." {
		t.Errorf("expected missing contract error message, got: %q", res)
	}

	// 2. Predicted contract or contract without TargetAmount
	ei.EggIncContractsAll["predicted-placeholder"] = ei.EggIncContract{
		ID:          "predicted-placeholder",
		Name:        "Predicted Contract",
		Predicted:   true,
		MaxCoopSize: 10,
	}

	res = GetContractEstimateString("predicted-placeholder", false)
	if res != "Contract estimates are not available for predicted or incomplete contracts." {
		t.Errorf("expected predicted contract error message, got: %q", res)
	}

	resWithOverride := GetContractEstimateString("predicted-placeholder", false, 150.0)
	if resWithOverride != "Contract estimates are not available for predicted or incomplete contracts." {
		t.Errorf("expected predicted contract error message with override, got: %q", resWithOverride)
	}
}

func TestQuantBlitzEstimate(t *testing.T) {
	LoadContractData("../../ttbb-data/ei-contracts.json")
	c, ok := ei.EggIncContractsAll["quant-blitz"]
	if !ok {
		t.Fatalf("quant-blitz not found")
	}

	if c.EstimatedDurationLower >= c.EstimatedDuration {
		t.Errorf("expected lower duration (%v) < upper duration (%v)", c.EstimatedDurationLower, c.EstimatedDuration)
	}
	if c.EstimatedDurationMax >= c.EstimatedDurationLower {
		t.Errorf("expected max duration (%v) < lower duration (%v)", c.EstimatedDurationMax, c.EstimatedDurationLower)
	}
	if c.EstimatedDurationMax > 20*time.Minute {
		t.Errorf("expected max duration (%v) to be under 20m", c.EstimatedDurationMax)
	}

	estStr := GetContractEstimateString("quant-blitz", true)
	t.Logf("estStr:\n%s", estStr)
	if !strings.Contains(estStr, "3.85 fair share") {
		t.Errorf("expected GetContractEstimateString output to contain '3.85 fair share', got:\n%s", estStr)
	}
	if !strings.Contains(estStr, "Leggy Set: **10m**") {
		t.Errorf("expected GetContractEstimateString output to contain 'Leggy Set: **10m**', got:\n%s", estStr)
	}
	if !strings.Contains(estStr, "8🪙 boost") && !strings.Contains(estStr, "8<::> boost") {
		t.Errorf("expected GetContractEstimateString output to contain '8 token boost', got:\n%s", estStr)
	}

	estStrOverride := GetContractEstimateString("quant-blitz", true, 232.0)
	if !strings.Contains(estStrOverride, "3.85 fair share") {
		t.Errorf("expected GetContractEstimateString (TE=232) output to contain '3.85 fair share', got:\n%s", estStrOverride)
	}
	if !strings.Contains(estStrOverride, "Leggy Set: **8m**") {
		t.Errorf("expected GetContractEstimateString (TE=232) output to contain 'Leggy Set: **8m**', got:\n%s", estStrOverride)
	}
}

func TestInspectQuantBlitzAcl(t *testing.T) {
	fname := "../../ttbb-data/pb-completed/quant-blitz-acl.pb"
	protoDataBytes, err := os.ReadFile(fname)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	gzReader, err := gzip.NewReader(bytes.NewReader(protoDataBytes))
	if err != nil {
		t.Fatalf("failed to gzip reader: %v", err)
	}
	decompressedBytes, err := io.ReadAll(gzReader)
	_ = gzReader.Close()
	if err != nil {
		t.Fatalf("failed to read decompressed: %v", err)
	}

	enc := base64.StdEncoding
	decodedAuthBuf := &ei.AuthenticatedMessage{}
	rawDecodedText, err := enc.DecodeString(string(decompressedBytes))
	if err != nil {
		t.Fatalf("failed to b64 decode: %v", err)
	}
	err = proto.Unmarshal(rawDecodedText, decodedAuthBuf)
	if err != nil {
		t.Fatalf("failed to unmarshal auth: %v", err)
	}

	if decodedAuthBuf.GetCompressed() {
		gr, zerr := zlib.NewReader(bytes.NewReader(decodedAuthBuf.Message))
		if zerr != nil {
			t.Fatalf("failed zlib: %v", zerr)
		}
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, gr)
		_ = gr.Close()
		decodedAuthBuf.Message = buf.Bytes()
	}

	status := &ei.ContractCoopStatusResponse{}
	err = proto.Unmarshal(decodedAuthBuf.Message, status)
	if err != nil {
		t.Fatalf("failed to unmarshal coop status: %v", err)
	}

	if status.GetContractIdentifier() != "quant-blitz" {
		t.Errorf("expected contract quant-blitz, got %s", status.GetContractIdentifier())
	}
	if status.GetTotalAmount() < 1.3e14 {
		t.Errorf("expected total amount >= 1.3e14, got %f", status.GetTotalAmount())
	}

	contributors := status.GetContributors()
	if len(contributors) != 3 {
		t.Fatalf("expected 3 contributors, got %d", len(contributors))
	}

	c0 := contributors[0]
	farm := c0.GetFarmInfo()
	var totalPop uint64
	for _, p := range farm.GetHabPopulation() {
		totalPop += p
	}
	var boosts []string
	for _, b := range farm.GetActiveBoosts() {
		boosts = append(boosts, fmt.Sprintf("%s(timeRemaining=%.1f)", b.GetBoostId(), b.GetTimeRemaining()))
	}
	t.Logf("Contributor 0: totalPop=%d, contribRate=%.3e (%.2f q/hr), activeBoosts=%v",
		totalPop, c0.GetContributionRate(), c0.GetContributionRate()*3600/1e15, boosts)

	for i, c := range contributors {

		t.Logf("--- Contributor %d (%s) ---", i, c.GetUserName())
		farm := c.GetFarmInfo()
		var artStr []string
		if farm != nil {
			for _, a := range farm.GetEquippedArtifacts() {
				var stones []string
				for _, s := range a.GetStones() {
					stones = append(stones, fmt.Sprintf("%s(L%d)", s.GetName(), s.GetLevel()))
				}
				artStr = append(artStr, fmt.Sprintf("%s(R%d,stones=%v)", a.GetSpec().GetName(), a.GetSpec().GetRarity(), stones))
			}
		}

		for _, b := range c.GetBuffHistory() {
			t.Logf("  Buff: time=%.1f, defl=%.2f, siab=%.2f",
				b.GetServerTimestamp(), b.GetEggLayingRate(), b.GetEarnings())
		}

	}

	fraction := c0.GetContributionAmount() / status.GetTotalAmount()
	if fraction < 0.95 {
		t.Errorf("expected contributor 0 fraction >= 0.95, got %f", fraction)
	}

	// When goals are achieved, SecondsRemaining_now = SecondsRemaining_completion - SecondsSinceAllGoalsAchieved
	// So SecondsRemaining_completion = SecondsSinceAchieved + SecondsRemaining
	// completionSec = 1800 - (SecondsSinceAchieved + SecondsRemaining)
	completionSec := 1800.0 - (status.GetSecondsSinceAllGoalsAchieved() + status.GetSecondsRemaining())
	t.Logf("Computed completion duration: %.1fs (%.1fm)", completionSec, completionSec/60.0)
	if completionSec < 630 || completionSec > 640 {
		t.Errorf("expected completion around 635s (10.5m), got %f", completionSec)
	}
}

func TestQuantBlitzAclScore(t *testing.T) {
	LoadContractData("../../ttbb-data/ei-contracts.json")
	c := ei.EggIncContractsAll["quant-blitz"]
	_ = getContractDurationEstimate(c, c.TargetAmount[len(c.TargetAmount)-1], float64(c.MaxCoopSize), c.LengthInSeconds,
		c.ModifierSR, c.ModifierELR, c.ModifierHabCap, true, 100)

	estScore := getContractScoreEstimateWithDuration(c, ei.Contract_GRADE_AAA,
		635*time.Second,
		3.85,
		100, 10,
		32, 0,
		2,
		100, 5)

	if estScore < 14000 {
		t.Errorf("expected high score estimate for 10.5m run with fairShare 3.85, got %d", estScore)
	}
}
