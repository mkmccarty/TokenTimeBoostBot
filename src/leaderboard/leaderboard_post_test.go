package leaderboard

import (
	"strings"
	"testing"
)

func TestRenderTable_AllLeaderboards(t *testing.T) {
	prevMap := map[string]float64{
		"p1": 100,
	}

	virtueEggKeys := map[string]bool{
		LBEggsCuriosity:  true,
		LBEggsIntegrity:  true,
		LBEggsHumility:   true,
		LBEggsResilience: true,
		LBEggsKindness:   true,
	}

	withTEColumn := map[string]bool{
		LBEggsCuriosity:  true,
		LBEggsIntegrity:  true,
		LBEggsHumility:   true,
		LBEggsResilience: true,
		LBEggsKindness:   true,
		LBVirtueEggsSum:  true,
	}

	withCTEComparisonColumns := map[string]bool{
		LBCTETotal: true,
	}

	shortVirtueHeaders := map[string]string{
		LBEggsCuriosity:  "Curiosity",
		LBEggsIntegrity:  "Integrity",
		LBEggsHumility:   "Humility",
		LBEggsResilience: "Resilience",
		LBEggsKindness:   "Kindness",
	}

	for _, def := range AllLeaderboards {
		def := def
		t.Run(def.Key, func(t *testing.T) {
			row := LBEntry{
				LBType:   def.Key,
				Player:   "p1",
				GameName: "FarmerNameThatIsWayOverFifteenChars",
				Value:    12345,
			}

			switch {
			case def.Key == LBEarningsBonus:
				row.Details = "dressed:23456"
			case def.Key == LBCXPWeeklyDelta:
				row.Details = "na"
			case def.Key == LBEggsCuriosity:
				row.Details = "te:77"
			case def.Key == LBVirtueEggsSum:
				row.Details = "te:321"
			case def.Key == LBCTETotal:
				row.Details = "actual:12000"
			case virtueEggKeys[def.Key]:
				row.Details = "77 TE"
			default:
				row.Details = "sample detail"
			}

			colHeader, rowLines, footer := renderTable(def, []LBEntry{row}, prevMap, 0)

			if !strings.HasPrefix(colHeader, "```\n") {
				t.Fatalf("expected column header to start code block, got %q", colHeader)
			}
			if !strings.Contains(colHeader, "Rank") || !strings.Contains(colHeader, "Name") {
				t.Fatalf("expected rank/name columns in header for %s, got %q", def.Key, colHeader)
			}
			if len(rowLines) != 1 {
				t.Fatalf("expected 1 row line for %s, got %d", def.Key, len(rowLines))
			}

			line := rowLines[0]
			if !strings.Contains(line, "#1") {
				t.Fatalf("expected rank prefix in row line for %s, got %q", def.Key, line)
			}

			expectedDelta := FormatLBDelta(def.ValueFmt, row.Value-prevMap[row.Player])
			if def.Key == LBCXPWeeklyDelta {
				if !strings.Contains(line, "-") {
					t.Fatalf("expected fallback '-' display for %s when lookback is unavailable, got %q", def.Key, line)
				}
			} else if expectedDelta != "" && strings.Contains(line, expectedDelta) {
				t.Fatalf("did not expect delta %q in row line for %s, got %q", expectedDelta, def.Key, line)
			}

			if !strings.HasPrefix(footer, "-# Updated:") {
				t.Fatalf("expected footer timestamp prefix for %s, got %q", def.Key, footer)
			}

			if def.HeaderName != "" && def.Key != LBSoulMirrors && !strings.Contains(colHeader, def.HeaderName) {
				t.Fatalf("expected header override %q for %s, got %q", def.HeaderName, def.Key, colHeader)
			}

			if withTEColumn[def.Key] {
				if !strings.Contains(colHeader, "TE") {
					t.Fatalf("expected TE column in header for %s, got %q", def.Key, colHeader)
				}
				if def.Key == LBEggsCuriosity && !strings.Contains(line, "| 77 ") {
					t.Fatalf("expected Curiosity TE column value for %s, got %q", def.Key, line)
				}
				if (def.Key == LBEggsIntegrity || def.Key == LBEggsHumility || def.Key == LBEggsResilience || def.Key == LBEggsKindness) && !strings.Contains(line, "| 77 ") {
					t.Fatalf("expected virtue TE column value for %s, got %q", def.Key, line)
				}
				if def.Key == LBVirtueEggsSum && !strings.Contains(line, "| 321 ") {
					t.Fatalf("expected Virtue Sum TE column value for %s, got %q", def.Key, line)
				}
				if strings.Contains(line, "(77 TE)") || strings.Contains(line, "(321 TE)") {
					t.Fatalf("did not expect TE details in parentheses when TE column is present for %s, got %q", def.Key, line)
				}
			}

			if withCTEComparisonColumns[def.Key] {
				if !strings.Contains(colHeader, "Pending CTE") || !strings.Contains(colHeader, "CTE") {
					t.Fatalf("expected CTE comparison columns for %s, got %q", def.Key, colHeader)
				}
				if !strings.Contains(line, "12,345") || !strings.Contains(line, "12,000") {
					t.Fatalf("expected pending and actual CTE values in row for %s, got %q", def.Key, line)
				}
			}

			if def.Key == LBEarningsBonus {
				if !strings.Contains(colHeader, "Nekkid") || !strings.Contains(colHeader, "Dressed") {
					t.Fatalf("expected EB columns for %s, got %q", def.Key, colHeader)
				}
				if strings.Contains(line, "(dressed:") {
					t.Fatalf("expected dressed details not rendered as parentheses for %s, got %q", def.Key, line)
				}
			} else {
				if strings.Contains(colHeader, "Dressed") {
					t.Fatalf("did not expect dressed column for %s, got %q", def.Key, colHeader)
				}
			}

			if virtueEggKeys[def.Key] {
				expectedHeader := shortVirtueHeaders[def.Key]
				if !strings.Contains(colHeader, expectedHeader) {
					t.Fatalf("expected shortened virtue header %q for %s, got %q", expectedHeader, def.Key, colHeader)
				}
				if strings.Contains(colHeader, "Eggs Delivered") {
					t.Fatalf("did not expect long virtue header text for %s, got %q", def.Key, colHeader)
				}
				if !withTEColumn[def.Key] && !strings.Contains(line, "(77 TE)") {
					t.Fatalf("expected TE detail in row for %s, got %q", def.Key, line)
				}
			}

			//t.Logf("rendered table for %s:\n%s%s%s", def.Key, colHeader, line, footer)
		})
	}
}

func TestRenderTable_CompetitionRankingOnTies(t *testing.T) {
	def := LBDef{Key: LBDrones, DisplayName: "Drones", ValueFmt: "int", HigherIsBetter: true}

	rows := []LBEntry{
		{LBType: LBDrones, Player: "p1", GameName: "Alpha", Value: 101},
		{LBType: LBDrones, Player: "p2", GameName: "Bravo", Value: 101},
		{LBType: LBDrones, Player: "p3", GameName: "Charlie", Value: 90},
	}

	_, rowLines, _ := renderTable(def, rows, map[string]float64{}, 0)
	if len(rowLines) != 3 {
		t.Fatalf("expected 3 row lines, got %d", len(rowLines))
	}

	if !strings.Contains(rowLines[0], "#1") {
		t.Fatalf("expected first row rank #1, got %q", rowLines[0])
	}
	if strings.Contains(rowLines[1], "#") {
		t.Fatalf("expected second row rank to be blank due to tie, got %q", rowLines[1])
	}
	if !strings.Contains(rowLines[2], "#3") {
		t.Fatalf("expected third row rank #3 after tie, got %q", rowLines[2])
	}
}

func TestRenderTable_SoulMirrorsDynamicSpacing(t *testing.T) {
	def, ok := LBDefByKey(LBSoulMirrors)
	if !ok {
		t.Fatalf("expected leaderboard definition for %s", LBSoulMirrors)
	}

	rows := []LBEntry{
		{LBType: LBSoulMirrors, Player: "p1", GameName: "Alpha", Value: 123, Details: "(1, 22, 333)"},
		{LBType: LBSoulMirrors, Player: "p2", GameName: "Bravo", Value: 456, Details: "(4444, 5, 66)"},
	}

	colHeader, rowLines, _ := renderTable(def, rows, map[string]float64{}, 0)

	if len(rowLines) != 2 {
		t.Fatalf("expected 2 row lines, got %d", len(rowLines))
	}

	// Dynamic widths should be driven by max observed digits per tier: C=4, E=2, L=3.
	if !strings.Contains(colHeader, "|    C |  E |   L ") {
		t.Fatalf("expected dynamic Soul Mirrors header spacing, got %q", colHeader)
	}
	if strings.Contains(colHeader, "|     C |") || strings.Contains(colHeader, "|     E |") || strings.Contains(colHeader, "|     L |") {
		t.Fatalf("did not expect fixed 5-character Soul Mirrors header spacing, got %q", colHeader)
	}

	if !strings.Contains(rowLines[0], "|    1 | 22 | 333 ") {
		t.Fatalf("expected dynamic C/E/L spacing for first Soul Mirrors row, got %q", rowLines[0])
	}
	if !strings.Contains(rowLines[1], "| 4444 |  5 |  66 ") {
		t.Fatalf("expected dynamic C/E/L spacing for second Soul Mirrors row, got %q", rowLines[1])
	}
	if strings.Contains(rowLines[0], "|     1 |") || strings.Contains(rowLines[1], "|    66 |") {
		t.Fatalf("did not expect fixed 5-character Soul Mirrors row spacing, got %q / %q", rowLines[0], rowLines[1])
	}
}

func TestTruncateString_EmojisAndWidth(t *testing.T) {
	tests := []struct {
		input    string
		max      int
		expected string
	}{
		{"ShortName", 14, "ShortName"},
		{"Exactly14Chars", 14, "Exactly14Chars"},
		{"WayTooLongNameHere", 14, "WayTooLongNam…"},
		{"Emoji🚀Name", 14, "Emoji🚀Name"},
		{"🚀🚀🚀🚀🚀🚀🚀", 14, "🚀🚀🚀🚀🚀🚀🚀"}, // 7 * 2 = 14 width
		{"🚀🚀🚀🚀🚀🚀🚀🚀", 14, "🚀🚀🚀🚀🚀🚀…"}, // 8 * 2 = 16 width, so truncate. "…" is width 1, 6 * 2 = 12, total 13 width <= 14.
		{"🚀🚀🚀🚀🚀🚀🚀A", 14, "🚀🚀🚀🚀🚀🚀…"}, // 7*2 + 1 = 15 width. 6 * 2 = 12 + "…" (1) = 13 <= 14.
		{"Emoji🚀🚀🚀🚀🚀🚀", 14, "Emoji🚀🚀🚀🚀…"}, // 5 + 6*2 = 17 width. "Emoji" (5) + 4*2 (8) + "…" (1) = 14 <= 14.
	}

	for _, tt := range tests {
		result := truncateString(tt.input, tt.max)
		if result != tt.expected {
			t.Errorf("truncateString(%q, %d) = %q; want %q", tt.input, tt.max, result, tt.expected)
		}
	}
}
