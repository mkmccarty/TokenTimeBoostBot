package boost

import (
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

func TestCalculateTokenValueCoopLog_UnescapesMarkdownInCodeBlock(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	contract := &Contract{
		StartTime: start,
		TokenLog: []ei.TokenUnitLog{
			{
				Time:       start.Add(10 * time.Minute),
				FromUserID: "userA",
				FromNick:   `lil\_lashes`,
				ToUserID:   "userB",
				ToNick:     `Player \(sh\)`,
				Quantity:   2,
			},
			{
				Time:       start.Add(20 * time.Minute),
				FromUserID: "userC",
				FromNick:   "back\\`tick",
				ToUserID:   "userA",
				ToNick:     `lil\_lashes`,
				Quantity:   1,
			},
		},
	}

	got := calculateTokenValueCoopLog(contract, 2*time.Hour)

	if strings.Contains(got, `\`) {
		t.Fatalf("expected no markdown escape backslashes in code-block summary, got:\n%s", got)
	}
	if !strings.Contains(got, "lil_lashes") {
		t.Errorf("expected unescaped name %q in output, got:\n%s", "lil_lashes", got)
	}
	if !strings.Contains(got, "Player (sh)") {
		t.Errorf("expected unescaped name %q in output, got:\n%s", "Player (sh)", got)
	}
	if !strings.Contains(got, "back'tick") {
		t.Errorf("expected backtick sanitized to single quote in output, got:\n%s", got)
	}
	for _, line := range strings.Split(strings.TrimSpace(got), "\n") {
		if strings.Count(line, "`") != 2 {
			t.Errorf("expected exactly 2 enclosing backticks per line, got %d in %q", strings.Count(line, "`"), line)
		}
	}
}
