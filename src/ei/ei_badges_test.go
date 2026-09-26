package ei

import (
	"strings"
	"testing"
)

func TestBadgeNAHAndNAHLegacy(t *testing.T) {
	// 19 eggs; index 18 is Enlightenment.
	// NAHBaseThreshold = 19845000000.
	// Default hab modifier fallback is 1.05.
	// nahCurrentThreshold = 19845000000 * 1.05 = 20837250000.

	tests := []struct {
		name          string
		habSize       uint64
		expectedBadge BadgeType
		expectNone    bool
	}{
		{
			name:          "Below NAH Base Threshold",
			habSize:       10000000000,
			expectNone:    true,
		},
		{
			name:          "Legacy NAH",
			habSize:       19845000000,
			expectedBadge: BadgeNAHLegacy,
		},
		{
			name:          "Between Legacy and Current Threshold",
			habSize:       20000000000,
			expectedBadge: BadgeNAHLegacy,
		},
		{
			name:          "Current NAH",
			habSize:       20837250000,
			expectedBadge: BadgeNAH,
		},
		{
			name:          "Well Above Current NAH",
			habSize:       25000000000,
			expectedBadge: BadgeNAH,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			farmsize := make([]uint64, 19)
			farmsize[18] = tc.habSize

			backup := &Backup{
				Game: &Backup_Game{
					MaxFarmSizeReached: farmsize,
				},
			}

			badges := GetBadges(backup)
			if tc.expectNone {
				for _, b := range badges {
					if b == BadgeNAH || b == BadgeNAHLegacy {
						t.Errorf("Expected no NAH badge, but got %s", b)
					}
				}
			} else {
				found := false
				for _, b := range badges {
					if b == tc.expectedBadge {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected badge %s, but got badges %v", tc.expectedBadge, badges)
				}
			}
		})
	}
}

func TestBadgeFED(t *testing.T) {
	// fedCurrentThreshold = 14175000000 * 1.05 = 14883750000
	// nahCurrentThreshold = 19845000000 * 1.05 = 20837250000

	t.Run("FED Achieved", func(t *testing.T) {
		farmsize := make([]uint64, 19)
		for i := 0; i < 18; i++ {
			farmsize[i] = 15000000000 // > 14883750000
		}
		farmsize[18] = 21000000000 // > 20837250000

		backup := &Backup{
			Game: &Backup_Game{
				MaxFarmSizeReached: farmsize,
			},
		}

		badges := GetBadges(backup)
		hasFED := false
		hasNAH := false
		for _, b := range badges {
			if b == BadgeFED {
				hasFED = true
			}
			if b == BadgeNAH {
				hasNAH = true
			}
		}
		if !hasFED {
			t.Errorf("Expected FED badge to be awarded")
		}
		if !hasNAH {
			t.Errorf("Expected NAH badge to also be awarded with FED")
		}
	})

	t.Run("FED Missing one egg", func(t *testing.T) {
		farmsize := make([]uint64, 19)
		for i := 0; i < 18; i++ {
			farmsize[i] = 15000000000
		}
		farmsize[5] = 10000000000 // Below threshold for egg 6
		farmsize[18] = 21000000000

		backup := &Backup{
			Game: &Backup_Game{
				MaxFarmSizeReached: farmsize,
			},
		}

		badges := GetBadges(backup)
		for _, b := range badges {
			if b == BadgeFED {
				t.Errorf("Did not expect FED badge when an egg is below threshold")
			}
		}
	})
}

func TestBadgeALCandZLC(t *testing.T) {
	t.Run("ALC with 22 distinct legendaries", func(t *testing.T) {
		var items []*ArtifactInventoryItem
		names := []ArtifactSpec_Name{
			ArtifactSpec_LUNAR_TOTEM,
			ArtifactSpec_NEODYMIUM_MEDALLION,
			ArtifactSpec_BEAK_OF_MIDAS,
			ArtifactSpec_LIGHT_OF_EGGENDIL,
			ArtifactSpec_DEMETERS_NECKLACE,
			ArtifactSpec_VIAL_MARTIAN_DUST,
			ArtifactSpec_ORNATE_GUSSET,
			ArtifactSpec_THE_CHALICE,
			ArtifactSpec_BOOK_OF_BASAN,
			ArtifactSpec_PHOENIX_FEATHER,
			ArtifactSpec_TUNGSTEN_ANKH, // level 2 (NORMAL/T3)
			ArtifactSpec_TUNGSTEN_ANKH, // level 3 (GREATER/T4)
			ArtifactSpec_AURELIAN_BROOCH,
			ArtifactSpec_CARVED_RAINSTICK,
			ArtifactSpec_PUZZLE_CUBE,
			ArtifactSpec_QUANTUM_METRONOME,
			ArtifactSpec_SHIP_IN_A_BOTTLE,
			ArtifactSpec_TACHYON_DEFLECTOR,
			ArtifactSpec_INTERSTELLAR_COMPASS,
			ArtifactSpec_DILITHIUM_MONOCLE,
			ArtifactSpec_TITANIUM_ACTUATOR,
			ArtifactSpec_MERCURYS_LENS,
		}

		for i, name := range names {
			lvl := ArtifactSpec_GREATER
			if name == ArtifactSpec_TUNGSTEN_ANKH && i == 10 {
				lvl = ArtifactSpec_NORMAL
			}
			rarity := ArtifactSpec_LEGENDARY
			qty := float64(1)
			items = append(items, &ArtifactInventoryItem{
				Quantity: &qty,
				Artifact: &CompleteArtifact{
					Spec: &ArtifactSpec{
						Name:   &name,
						Level:  &lvl,
						Rarity: &rarity,
					},
				},
			})
		}

		backup := &Backup{
			ArtifactsDb: &ArtifactsDB{
				InventoryItems: items,
			},
		}

		badges := GetBadges(backup)
		hasALC := false
		for _, b := range badges {
			if b == BadgeALC {
				hasALC = true
			}
		}
		if !hasALC {
			t.Errorf("Expected ALC badge with 22 distinct legendaries, got %v", badges)
		}
	})

	t.Run("ZLC variants", func(t *testing.T) {
		// Player with 0 legendaries and 10 missions (not 100 exhens) -> ZLC
		statusArchived := MissionInfo_ARCHIVED
		statusComplete := MissionInfo_COMPLETE
		shipHenerprise := MissionInfo_HENERPRISE
		durationEpic := MissionInfo_EPIC
		qty := float64(1)
		nameCommon := ArtifactSpec_LUNAR_TOTEM
		lvlCommon := ArtifactSpec_INFERIOR
		rarityCommon := ArtifactSpec_COMMON

		backupZLC := &Backup{
			ArtifactsDb: &ArtifactsDB{
				InventoryItems: []*ArtifactInventoryItem{
					{
						Quantity: &qty,
						Artifact: &CompleteArtifact{
							Spec: &ArtifactSpec{
								Name:   &nameCommon,
								Level:  &lvlCommon,
								Rarity: &rarityCommon,
							},
						},
					},
				},
			},
		}

		badgesZLC := GetBadges(backupZLC)
		if len(badgesZLC) != 1 || badgesZLC[0] != BadgeZLC {
			t.Errorf("Expected ZLC, got %v", badgesZLC)
		}

		// Player with 100 completed ExHens but Henerprise stars < 7 -> ZLC100
		var exHens []*MissionInfo
		for i := 0; i < 100; i++ {
			exHens = append(exHens, &MissionInfo{
				Ship:         &shipHenerprise,
				DurationType: &durationEpic,
				Status:       &statusArchived,
			})
		}
		// 100 ExHens gives 100 * 1.8 = 180 points. Level 7 needs 335.
		backupZLC100 := &Backup{
			ArtifactsDb: &ArtifactsDB{
				InventoryItems: []*ArtifactInventoryItem{
					{
						Quantity: &qty,
						Artifact: &CompleteArtifact{
							Spec: &ArtifactSpec{
								Name:   &nameCommon,
								Level:  &lvlCommon,
								Rarity: &rarityCommon,
							},
						},
					},
				},
				MissionArchive: exHens,
			},
		}

		badgesZLC100 := GetBadges(backupZLC100)
		hasZLC100 := false
		for _, b := range badgesZLC100 {
			if b == BadgeZLC100 {
				hasZLC100 = true
			}
		}
		if !hasZLC100 {
			t.Errorf("Expected ZLC100, got %v", badgesZLC100)
		}

		// Player with 200 completed ExHens -> 200 * 1.8 = 360 points >= 335 -> ZLC7STAR
		var exHens200 []*MissionInfo
		for i := 0; i < 200; i++ {
			exHens200 = append(exHens200, &MissionInfo{
				Ship:         &shipHenerprise,
				DurationType: &durationEpic,
				Status:       &statusComplete,
			})
		}
		backupZLC7 := &Backup{
			ArtifactsDb: &ArtifactsDB{
				InventoryItems: []*ArtifactInventoryItem{
					{
						Quantity: &qty,
						Artifact: &CompleteArtifact{
							Spec: &ArtifactSpec{
								Name:   &nameCommon,
								Level:  &lvlCommon,
								Rarity: &rarityCommon,
							},
						},
					},
				},
				MissionArchive: exHens200,
			},
		}

		badgesZLC7 := GetBadges(backupZLC7)
		hasZLC7 := false
		for _, b := range badgesZLC7 {
			if b == BadgeZLC7Star {
				hasZLC7 = true
			}
		}
		if !hasZLC7 {
			t.Errorf("Expected ZLC7STAR, got %v", badgesZLC7)
		}
	})
}

func TestBadgeASC(t *testing.T) {
	// Build missions to max out all ships 0..10
	statusArchived := MissionInfo_ARCHIVED
	durationEpic := MissionInfo_EPIC

	var allMissions []*MissionInfo
	// Launch points for Epic is 1.8
	for shipIdx := 1; shipIdx <= 10; shipIdx++ {
		ship := MissionInfo_Spaceship(shipIdx)
		reqLP := ShipMaxLaunchPoints[shipIdx]
		count := int(reqLP/1.8) + 1
		for i := 0; i < count; i++ {
			allMissions = append(allMissions, &MissionInfo{
				Ship:         &ship,
				DurationType: &durationEpic,
				Status:       &statusArchived,
			})
		}
	}

	backup := &Backup{
		ArtifactsDb: &ArtifactsDB{
			MissionArchive: allMissions,
		},
	}

	badges := GetBadges(backup)
	hasASC := false
	for _, b := range badges {
		if b == BadgeASC {
			hasASC = true
		}
	}
	if !hasASC {
		t.Errorf("Expected ASC badge when all ships are maxed, got %v", badges)
	}
}

func TestBadgeSFC(t *testing.T) {
	statusArchived := MissionInfo_ARCHIVED
	durationEpic := MissionInfo_EPIC

	buildMissions := func(chickfiantLP, voyeggerLP, henerpriseLP float64) []*MissionInfo {
		var missions []*MissionInfo
		ships := []struct {
			ship MissionInfo_Spaceship
			lp   float64
		}{
			{MissionInfo_CHICKFIANT, chickfiantLP},
			{MissionInfo_VOYEGGER, voyeggerLP},
			{MissionInfo_HENERPRISE, henerpriseLP},
		}
		for _, s := range ships {
			count := int(s.lp/1.8) + 1
			for i := 0; i < count; i++ {
				ship := s.ship
				missions = append(missions, &MissionInfo{
					Ship:         &ship,
					DurationType: &durationEpic,
					Status:       &statusArchived,
				})
			}
		}
		return missions
	}

	t.Run("SFC Achieved", func(t *testing.T) {
		backup := &Backup{
			ArtifactsDb: &ArtifactsDB{
				MissionArchive: buildMissions(ShipMaxLaunchPoints[7], ShipMaxLaunchPoints[8], ShipMaxLaunchPoints[9]),
			},
		}
		badges := GetBadges(backup)
		hasSFC := false
		for _, b := range badges {
			if b == BadgeSFC {
				hasSFC = true
			}
		}
		if !hasSFC {
			t.Errorf("Expected SFC badge when Defihent, Voyegger, and Henerprise are maxed, got %v", badges)
		}
	})

	t.Run("SFC Incomplete", func(t *testing.T) {
		// Henerprise not maxed
		backup := &Backup{
			ArtifactsDb: &ArtifactsDB{
				MissionArchive: buildMissions(ShipMaxLaunchPoints[7], ShipMaxLaunchPoints[8], 100.0),
			},
		}
		badges := GetBadges(backup)
		for _, b := range badges {
			if b == BadgeSFC {
				t.Errorf("Did not expect SFC badge when Henerprise is not maxed, got %v", badges)
			}
		}
	})
}

func TestBadgeGoodJob(t *testing.T) {
	xpPass := CraftingLevel30XP + 100.0
	backupPass := &Backup{
		Artifacts: &Backup_Artifacts{
			CraftingXp: &xpPass,
		},
	}

	badges := GetBadges(backupPass)
	hasGJ := false
	for _, b := range badges {
		if b == BadgeGoodJob {
			hasGJ = true
		}
	}
	if !hasGJ {
		t.Errorf("Expected GOOD-JOB badge with crafting XP >= level 30, got %v", badges)
	}

	xpFail := 100000.0
	backupFail := &Backup{
		Artifacts: &Backup_Artifacts{
			CraftingXp: &xpFail,
		},
	}
	badgesFail := GetBadges(backupFail)
	for _, b := range badgesFail {
		if b == BadgeGoodJob {
			t.Errorf("Did not expect GOOD-JOB badge with low crafting XP")
		}
	}
}

func TestGetBadgeMarkdownRow(t *testing.T) {
	if EmoteMap == nil {
		EmoteMap = make(map[string]Emotes)
	}
	EmoteMap["badge_nah"] = Emotes{Name: "badge_nah", ID: "111"}
	EmoteMap["badge_fed"] = Emotes{Name: "badge_fed", ID: "222"}

	farmsize := make([]uint64, 19)
	for i := 0; i < 18; i++ {
		farmsize[i] = 15000000000
	}
	farmsize[18] = 21000000000

	backup := &Backup{
		Game: &Backup_Game{
			MaxFarmSizeReached: farmsize,
		},
	}

	row := GetBadgeMarkdownRow(backup)
	expected := "<:badge_nah:111> <:badge_fed:222>"
	if !strings.Contains(row, "<:badge_nah:111>") || !strings.Contains(row, "<:badge_fed:222>") {
		t.Errorf("Expected %q in row, got %q", expected, row)
	}
}
