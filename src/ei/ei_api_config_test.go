package ei

import (
	"strings"
	"testing"
)

func TestDiffCustomHatcheries_DetectsNewHatchery(t *testing.T) {
	oldCfg := []byte(`{
		"dlc_catalog": {
			"shells": [
				{
					"identifier": "ei_hatchery_custom_cegatoregg",
					"primary_piece": {
						"dlc": {
							"url": "https://example.com/gatoregg.rpoz",
							"checksum": "abc123"
						}
					},
					"seconds_until_available": -1000.0
				}
			]
		}
	}`)

	newCfg := []byte(`{
		"dlc_catalog": {
			"shells": [
				{
					"identifier": "ei_hatchery_custom_cegatoregg",
					"primary_piece": {
						"dlc": {
							"url": "https://example.com/gatoregg.rpoz",
							"checksum": "abc123"
						}
					},
					"seconds_until_available": -2000.0
				},
				{
					"identifier": "ei_hatchery_custom_cecopper",
					"primary_piece": {
						"dlc": {
							"url": "https://example.com/copper.rpoz",
							"checksum": "def456"
						}
					},
					"seconds_until_available": -500.0
				}
			]
		}
	}`)

	hasNew, diff, err := diffCustomHatcheries(oldCfg, newCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasNew {
		t.Fatalf("expected hasNew=true, got false")
	}
	if !strings.Contains(diff, "+ ei_hatchery_custom_cecopper") {
		t.Errorf("expected diff to contain new copper hatchery, got:\n%s", diff)
	}
	if strings.Contains(diff, "cegatoregg") {
		t.Errorf("expected unchanged gatoregg hatchery to be omitted from diff, got:\n%s", diff)
	}
}

func TestDiffCustomHatcheries_IgnoresVolatileAndNonCustomChanges(t *testing.T) {
	oldCfg := []byte(`{
		"dlc_catalog": {
			"shells": [
				{
					"identifier": "ei_hatchery_custom_cegatoregg",
					"primary_piece": {
						"dlc": {
							"url": "https://example.com/gatoregg.rpoz",
							"checksum": "abc123"
						}
					},
					"seconds_until_available": -1000.0,
					"popularity": 10
				},
				{
					"identifier": "ei_silo_0_large_leggo",
					"primary_piece": {
						"dlc": {
							"url": "https://example.com/silo_v1.rpoz",
							"checksum": "111"
						}
					}
				}
			]
		}
	}`)

	newCfg := []byte(`{
		"dlc_catalog": {
			"shells": [
				{
					"identifier": "ei_hatchery_custom_cegatoregg",
					"primary_piece": {
						"dlc": {
							"url": "https://example.com/gatoregg.rpoz",
							"checksum": "abc123"
						}
					},
					"seconds_until_available": -9999.0,
					"popularity": 42
				},
				{
					"identifier": "ei_silo_0_large_leggo",
					"primary_piece": {
						"dlc": {
							"url": "https://example.com/silo_v2.rpoz",
							"checksum": "222"
						}
					}
				}
			]
		}
	}`)

	hasNew, diff, err := diffCustomHatcheries(oldCfg, newCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasNew || diff != "" {
		t.Fatalf("expected no custom hatchery change, got hasNew=%v diff=%q", hasNew, diff)
	}
}

func TestDiffCustomHatcheries_DetectsModifiedHatchery(t *testing.T) {
	oldCfg := []byte(`{
		"dlc_catalog": {
			"shells": [
				{
					"identifier": "ei_hatchery_custom_cecopper",
					"primary_piece": {
						"dlc": {
							"url": "https://example.com/copper_v1.rpoz",
							"checksum": "oldsum"
						}
					}
				}
			]
		}
	}`)

	newCfg := []byte(`{
		"dlc_catalog": {
			"shells": [
				{
					"identifier": "ei_hatchery_custom_cecopper",
					"primary_piece": {
						"dlc": {
							"url": "https://example.com/copper_v2.rpoz",
							"checksum": "newsum"
						}
					}
				}
			]
		}
	}`)

	hasNew, diff, err := diffCustomHatcheries(oldCfg, newCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasNew {
		t.Fatalf("expected hasNew=true for modified hatchery, got false")
	}
	if !strings.Contains(diff, "oldsum") || !strings.Contains(diff, "newsum") {
		t.Errorf("expected diff to contain old and new checksums, got:\n%s", diff)
	}
}
