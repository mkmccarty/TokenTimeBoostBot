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
			case def.Key == LBEggsCuriosity:
				row.Details = "te:77"
			case def.Key == LBVirtueEggsSum:
				row.Details = "te:321"
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
			if def.ValueFmt == "cxp" {
				if expectedDelta != "" && !strings.Contains(line, expectedDelta) {
					t.Fatalf("expected CXP gain %q in row line for %s, got %q", expectedDelta, def.Key, line)
				}
			} else if expectedDelta != "" && strings.Contains(line, expectedDelta) {
				t.Fatalf("did not expect non-CXP gain %q in row line for %s, got %q", expectedDelta, def.Key, line)
			}

			if !strings.HasPrefix(footer, "-# Updated:") {
				t.Fatalf("expected footer timestamp prefix for %s, got %q", def.Key, footer)
			}

			if def.HeaderName != "" && !strings.Contains(colHeader, def.HeaderName) {
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
			} else if def.Key != LBEarningsBonus && !withTEColumn[def.Key] {
				if !strings.Contains(line, "(sample detail)") {
					t.Fatalf("expected generic detail rendering for %s, got %q", def.Key, line)
				}
			}

			//t.Logf("rendered table for %s:\n%s%s%s", def.Key, colHeader, line, footer)
		})
	}
}
