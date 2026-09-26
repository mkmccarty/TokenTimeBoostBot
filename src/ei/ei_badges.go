package ei

import (
	"fmt"
	"strings"
)

// BadgeType represents a recognized player achievement badge.
type BadgeType string

const (
	BadgeNAH       BadgeType = "NAH"
	BadgeNAHLegacy BadgeType = "NAH-LEGACY"
	BadgeFED       BadgeType = "FED"
	BadgeZLC       BadgeType = "ZLC"
	BadgeZLC100    BadgeType = "ZLC100"
	BadgeZLC7Star  BadgeType = "ZLC7STAR"
	BadgeALC       BadgeType = "ALC"
	BadgeASC       BadgeType = "ASC"
	BadgeSFC       BadgeType = "SFC"
	BadgeGoodJob   BadgeType = "GOOD-JOB"
)

// Base thresholds for Nobel Prize in Animal Husbandry and Full Egg Dedication
const (
	NAHBaseThreshold = 19845000000
	FEDBaseThreshold = 14175000000
)

// ShipMaxLaunchPoints holds the cumulative launch points required to reach max level for each ship (0..10).
var ShipMaxLaunchPoints = [11]float64{
	0,    // CHICKEN_ONE (0)
	14,   // CHICKEN_NINE (1)
	45,   // CHICKEN_HEAVY (2)
	85,   // BCR (3)
	125,  // MILLENIUM_CHICKEN (4)
	125,  // CORELLIHEN_CORVETTE (5)
	185,  // GALEGGTICA (6)
	185,  // CHICKFIANT (7)
	255,  // VOYEGGER (8)
	435,  // HENERPRISE (9)
	1420, // ATREGGIES (10)
}

// HenerpriseLevel7LaunchPoints is the launch point threshold required for Henerprise to reach 7 stars.
const HenerpriseLevel7LaunchPoints = 335.0

// CraftingLevel30XP is the minimum cumulative crafting XP required for crafting level 30 (Crafting Legend).
const CraftingLevel30XP = 5070943000.0

var badgeEmojiNames = map[BadgeType][]string{
	BadgeNAH:       {"badge_nah", "nah"},
	BadgeNAHLegacy: {"badge_nah_legacy", "badge_nahlegacy", "nah_legacy", "nahlegacy"},
	BadgeFED:       {"badge_fed", "fed"},
	BadgeZLC:       {"badge_zlc", "zlc"},
	BadgeZLC100:    {"badge_zlc100", "zlc100"},
	BadgeZLC7Star:  {"badge_zlc7star", "zlc7star"},
	BadgeALC:       {"badge_alc", "alc"},
	BadgeASC:       {"badge_asc", "asc"},
	BadgeSFC:       {"badge_sfc", "sfc", "star_fleet_commander"},
	BadgeGoodJob:   {"badge_good_job", "badge_goodjob", "good_job", "goodjob"},
}

// GetBadgeEmojiMarkdown returns the markdown string for the given badge's emoji if available.
func GetBadgeEmojiMarkdown(badge BadgeType) (string, bool) {
	for _, name := range badgeEmojiNames[badge] {
		if md, ok := GetBotEmojiMarkdownIfExists(name); ok {
			return md, true
		}
	}
	return "", false
}

func getMissionLaunchPoints(dt MissionInfo_DurationType) float64 {
	switch dt {
	case MissionInfo_TUTORIAL, MissionInfo_SHORT:
		return 1.0
	case MissionInfo_LONG:
		return 1.4
	case MissionInfo_EPIC:
		return 1.8
	default:
		return 1.0
	}
}

// GetBadges evaluates the player's backup against achievement badge criteria and returns earned badges.
func GetBadges(backup *Backup) []BadgeType {
	if backup == nil {
		return nil
	}

	var badges []BadgeType

	// 1. NAH and NAH-LEGACY
	game := backup.GetGame()
	var farmsize []uint64
	if game != nil {
		farmsize = game.GetMaxFarmSizeReached()
	}

	habMod := GetMaxColleggtibleHabModifier()
	nahCurrentThreshold := float64(NAHBaseThreshold) * habMod
	fedCurrentThreshold := float64(FEDBaseThreshold) * habMod

	hasNAH := false
	hasLegacyNAH := false
	if len(farmsize) > 18 {
		enlightHab := float64(farmsize[18])
		if enlightHab >= nahCurrentThreshold {
			hasNAH = true
		} else if enlightHab >= NAHBaseThreshold {
			hasLegacyNAH = true
		}
	}

	if hasNAH {
		badges = append(badges, BadgeNAH)
	} else if hasLegacyNAH {
		badges = append(badges, BadgeNAHLegacy)
	}

	// 2. FED (Full Egg Dedication)
	// Requires all 19 regular eggs to have reached their respective thresholds:
	// Enlightenment (egg 19, index 18) must meet nahCurrentThreshold,
	// and eggs 1..18 (indices 0..17) must meet fedCurrentThreshold.
	if len(farmsize) >= 19 {
		hasFED := true
		for i := 0; i < 19; i++ {
			threshold := fedCurrentThreshold
			if i == 18 {
				threshold = nahCurrentThreshold
			}
			if float64(farmsize[i]) < threshold {
				hasFED = false
				break
			}
		}
		if hasFED {
			badges = append(badges, BadgeFED)
		}
	}

	// 3. Artifact Club: ZLC / ZLC100 / ZLC7STAR / ALC
	afxDB := backup.GetArtifactsDb()
	if afxDB != nil {
		legendaryCount := uint64(0)
		distinctLegendaries := make(map[string]bool)

		for _, item := range afxDB.GetInventoryItems() {
			if item == nil || item.GetQuantity() == 0 {
				continue
			}
			art := item.GetArtifact()
			if art == nil {
				continue
			}
			spec := art.GetSpec()
			if spec == nil {
				continue
			}
			if spec.GetRarity() == ArtifactSpec_LEGENDARY {
				legendaryCount += uint64(item.GetQuantity())
				key := fmt.Sprintf("%d_%d", spec.GetName(), spec.GetLevel())
				distinctLegendaries[key] = true
			}
		}

		allMissions := append(afxDB.GetMissionArchive(), afxDB.GetMissionInfos()...)

		if legendaryCount == 0 {
			// Check if player has unlocked/engaged with artifacts
			hasArtifactActivity := len(afxDB.GetInventoryItems()) > 0 || len(allMissions) > 0
			if hasArtifactActivity {
				completedExHens := 0
				henerpriseLP := 0.0
				for _, mi := range allMissions {
					if mi == nil {
						continue
					}
					if mi.GetShip() == MissionInfo_HENERPRISE {
						if mi.GetStatus() >= MissionInfo_EXPLORING {
							henerpriseLP += getMissionLaunchPoints(mi.GetDurationType())
						}
						if mi.GetDurationType() == MissionInfo_EPIC {
							st := mi.GetStatus()
							if st == MissionInfo_COMPLETE || st == MissionInfo_ARCHIVED {
								completedExHens++
							}
						}
					}
				}

				if completedExHens >= 100 {
					if henerpriseLP >= HenerpriseLevel7LaunchPoints {
						badges = append(badges, BadgeZLC7Star)
					} else {
						badges = append(badges, BadgeZLC100)
					}
				} else {
					badges = append(badges, BadgeZLC)
				}
			}
		} else if len(distinctLegendaries) >= 22 {
			badges = append(badges, BadgeALC)
		}

		// 4. Ship Club: ASC (All Star Club)
		// Requires all 11 ships (from Chicken One to Atreggies) to have max stars.
		// Also requires Atreggies to have been launched (so allMissions is not empty and includes Atreggies).
		var shipLP [11]float64
		hasAtreggiesLaunch := false
		for _, mi := range allMissions {
			if mi == nil || mi.GetStatus() < MissionInfo_EXPLORING {
				continue
			}
			s := int(mi.GetShip())
			if s >= 0 && s < 11 {
				shipLP[s] += getMissionLaunchPoints(mi.GetDurationType())
				if mi.GetShip() == MissionInfo_ATREGGIES {
					hasAtreggiesLaunch = true
				}
			}
		}

		if hasAtreggiesLaunch {
			allShipsMaxed := true
			for i := 0; i < 11; i++ {
				if shipLP[i] < ShipMaxLaunchPoints[i] {
					allShipsMaxed = false
					break
				}
			}
			if allShipsMaxed {
				badges = append(badges, BadgeASC)
			}
		}

		// Star Fleet Commander (all stars on Defihent, Voyegger, and Henerprise)
		if shipLP[MissionInfo_CHICKFIANT] >= ShipMaxLaunchPoints[MissionInfo_CHICKFIANT] &&
			shipLP[MissionInfo_VOYEGGER] >= ShipMaxLaunchPoints[MissionInfo_VOYEGGER] &&
			shipLP[MissionInfo_HENERPRISE] >= ShipMaxLaunchPoints[MissionInfo_HENERPRISE] {
			badges = append(badges, BadgeSFC)
		}
	}

	// 5. GOOD-JOB (Crafting Level >= 30)
	var craftingXP float64
	if afx := backup.GetArtifacts(); afx != nil {
		craftingXP = afx.GetCraftingXp()
	}
	if craftingXP >= CraftingLevel30XP || GetCraftingLevel(craftingXP) >= 30 {
		badges = append(badges, BadgeGoodJob)
	}

	return badges
}

// GetBadgeMarkdownRow returns the formatted row of badge emojis for the player.
// If the player has no badges or none can be resolved, an empty string is returned.
func GetBadgeMarkdownRow(backup *Backup) string {
	badges := GetBadges(backup)
	if len(badges) == 0 {
		return ""
	}

	var row []string
	for _, b := range badges {
		if md, ok := GetBadgeEmojiMarkdown(b); ok {
			row = append(row, md)
		}
	}
	if len(row) == 0 {
		return ""
	}
	return strings.Join(row, " ")
}
