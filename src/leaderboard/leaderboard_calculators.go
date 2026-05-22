package leaderboard

import (
	"fmt"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

// ─── shipVirtueIndex maps LB key → MissionInfo_Spaceship ordinal ─────────────
// ship ordinals match ei.MissionInfo_Spaceship_value entries 0-10.
var shipVirtueIndex = map[string]int{
	LBShipChicken1:           0,
	LBShipChicken9:           1,
	LBShipChickenHeavy:       2,
	LBShipBCR:                3,
	LBShipMilleniumChicken:   4,
	LBShipCorellihenCorvette: 5,
	LBShipGaleggtica:         6,
	LBShipDefihent:           7,
	LBShipVoyegger:           8,
	LBShipHenerprise:         9,
	LBShipAtreggies:          10,
}

var shipStdIndex = map[string]int{
	LBShipStdChicken1:           0,
	LBShipStdChicken9:           1,
	LBShipStdChickenHeavy:       2,
	LBShipStdBCR:                3,
	LBShipStdMilleniumChicken:   4,
	LBShipStdCorellihenCorvette: 5,
	LBShipStdGaleggtica:         6,
	LBShipStdDefihent:           7,
	LBShipStdVoyegger:           8,
	LBShipStdHenerprise:         9,
	LBShipStdAtreggies:          10,
}

// virtueEggIndex maps LB key → index in virtue.GetEggsDelivered() / GetEovEarned()

// No longer needed: virtueEggIndex for individual virtues (removed)

// ─── CollectionResult carries the outputs of RunCalculators ──────────────────

// RunCalculators evaluates all opted-in leaderboard types for a single player
// from their first-contact backup.
//
// archive is the contract archive result (used only for SourceContractArchive types).
// Pass nil for archive if only SourceFirstContact types are being evaluated.
//
// snapDate is the ISO date string "YYYY-MM-DD" for this collection run.
// priorCXPTotal is the total CXP from the previous collection, used for delta calculation.
func RunCalculators(
	userID string,
	backup *ei.Backup,
	archive []*ei.LocalContract,
	optedIn []string,
	snapDate string,
	priorCXPTotal float64,
) []LBEntry {
	if backup == nil && archive == nil {
		return nil
	}

	gameName := ""
	if backup != nil {
		gameName = backup.GetUserName()
		if game := backup.GetGame(); game != nil && game.GetPermitLevel() != 1 {
			gameName += " (SP)"
		}
	}

	// Build a set for fast opt-in lookup.
	optSet := make(map[string]struct{}, len(optedIn))
	for _, k := range optedIn {
		optSet[k] = struct{}{}
	}
	isOptedIn := func(key string) bool {
		_, ok := optSet[key]
		return ok
	}

	var entries []LBEntry

	// ── Helper: append if player opted in ────────────────────────────────────
	emit := func(e LBEntry) {
		if isOptedIn(e.LBType) {
			entries = append(entries, e)
		}
	}

	// ── Virtue data ───────────────────────────────────────────────────────────
	var virtue *ei.Backup_Virtue
	if backup != nil {
		virtue = backup.GetVirtue()
	}

	// Virtue shifts
	pendingTE := 0.0
	if virtue != nil {
		claimedTE := ei.GetCurrentTruthEggs(backup)
		totalTEWithUnclaimed := claimedTE
		deliveries := virtue.GetEggsDelivered()
		earned := virtue.GetEovEarned()
		for i := 0; i < len(deliveries); i++ {
			earnedTE := uint32(0)
			if i < len(earned) {
				earnedTE = earned[i]
			}
			pendingTE += float64(ei.PendingTruthEggs(deliveries[i], earnedTE))
			totalTEWithUnclaimed += ei.PendingTruthEggs(deliveries[i], earnedTE)
		}

		emit(LBEntry{
			LBType:   LBVirtueShifts,
			Player:   userID,
			GameName: gameName,
			SnapDate: snapDate,
			Value:    float64(virtue.GetShiftCount()),
		})

		// Total Truth Eggs should use currently credited/claimed TE, not pending.
		emit(LBEntry{
			LBType:   LBTETotal,
			Player:   userID,
			GameName: gameName,
			SnapDate: snapDate,
			Value:    float64(claimedTE),
		})

		maxCTEResult := ei.CalculateMaxClothedTEWithSlotHint(backup, 2*(int(backup.GetGame().GetPermitLevel())+1))
		maxCTE := maxCTEResult.ClothedTE

		emit(LBEntry{
			LBType:   LBCTETotal,
			Player:   userID,
			GameName: gameName,
			SnapDate: snapDate,
			Value:    maxCTE + pendingTE,
			Details:  fmt.Sprintf("actual:%f", maxCTE),
		})
	}

	// Truth Eggs & raw eggs per virtue egg
	if virtue != nil {
		deliveries := virtue.GetEggsDelivered()
		// Individual virtue eggs
		eggKeys := []string{
			LBEggsCuriosity,
			LBEggsIntegrity,
			LBEggsHumility,
			LBEggsResilience,
			LBEggsKindness,
		}
		for i := 0; i < 5 && i < len(deliveries); i++ {
			teEarned := ei.CountTruthEggTiersPassed(deliveries[i])
			emit(LBEntry{
				LBType:   eggKeys[i],
				Player:   userID,
				GameName: gameName,
				SnapDate: snapDate,
				Value:    deliveries[i],
				Details:  fmt.Sprintf("%d TE", teEarned),
			})
		}

		// Combined sum for all virtue eggs
		totalEggs := 0.0
		totalTEAtLevel := 0
		for i := 0; i < 5 && i < len(deliveries); i++ {
			delivered := deliveries[i]
			totalEggs += delivered
			totalTEAtLevel += int(ei.CountTruthEggTiersPassed(delivered))
		}
		emit(LBEntry{
			LBType:   LBVirtueEggsSum,
			Player:   userID,
			GameName: gameName,
			SnapDate: snapDate,
			Value:    totalEggs,
			Details:  fmt.Sprintf("te:%d", totalTEAtLevel),
		})
	}

	// ── Prestige and Drone stats ──────────────────────────────────────────────
	if backup != nil {
		game := backup.GetGame()
		stats := backup.GetStats()

		if game != nil {
			emit(LBEntry{LBType: LBSoulEggs, Player: userID, GameName: gameName, SnapDate: snapDate, Value: game.GetSoulEggsD()})
			emit(LBEntry{LBType: LBProphecyEggs, Player: userID, GameName: gameName, SnapDate: snapDate, Value: float64(game.GetEggsOfProphecy())})

			// For Earnings Bonus, we need total Truth Eggs (EoV)
			totalTE := 0.0
			if virtue != nil {
				deliveries := virtue.GetEggsDelivered()
				for i := 0; i < 5 && i < len(deliveries); i++ {
					totalTE += float64(ei.CountTruthEggTiersPassed(deliveries[i]))
				}
			}
			nakedEB := ei.GetEarningsBonus(backup, totalTE)
			dressedEB := ei.GetDressedEarningsBonus(backup, totalTE)
			emit(LBEntry{
				LBType:   LBEarningsBonus,
				Player:   userID,
				GameName: gameName,
				SnapDate: snapDate,
				Value:    nakedEB,
				Details:  fmt.Sprintf("dressed:%.6f", dressedEB),
			})
		}

		if stats != nil {
			emit(LBEntry{LBType: LBDrones, Player: userID, GameName: gameName, SnapDate: snapDate, Value: float64(stats.GetDroneTakedowns())})
			emit(LBEntry{LBType: LBEliteDrones, Player: userID, GameName: gameName, SnapDate: snapDate, Value: float64(stats.GetDroneTakedownsElite())})
			emit(LBEntry{LBType: LBPrestiges, Player: userID, GameName: gameName, SnapDate: snapDate, Value: float64(stats.GetNumPrestiges())})
		}

		{
			totalCXP := 0.0
			if backup.GetContracts() != nil && backup.GetContracts().GetLastCpi() != nil {
				totalCXP = backup.GetContracts().GetLastCpi().GetTotalCxp()
			}
			if archive != nil {
				archiveTotalCXP := 0.0
				for _, lc := range archive {
					eval := lc.GetEvaluation()
					if eval != nil {
						archiveTotalCXP += eval.GetCxp()
					}
				}
				if archiveTotalCXP > totalCXP {
					totalCXP = archiveTotalCXP
				}
			}

			if totalCXP > 0 {
				emit(LBEntry{
					LBType:   LBContractExp,
					Player:   userID,
					GameName: gameName,
					SnapDate: snapDate,
					Value:    totalCXP,
				})
			}
		}

		// ── Soul Mirrors ──────────────────────────────────────────────────────
		if game != nil {
			blue, purple, orange := 0, 0, 0
			for _, b := range game.GetBoosts() {
				switch b.GetBoostId() {
				case "soul_mirror_blue":
					blue = int(b.GetCount())
				case "soul_mirror_purple":
					purple = int(b.GetCount())
				case "soul_mirror_orange":
					orange = int(b.GetCount())
				}
			}

			pts := float64(blue + purple*2 + orange*3)
			emit(LBEntry{LBType: LBSoulMirrors, Player: userID, GameName: gameName, Value: pts, Details: fmt.Sprintf("(%d, %d, %d)", blue, purple, orange)})
		}
	}

	// ── Mission launch counts ─────────────────────────────────────────────────
	if backup != nil {
		db := backup.GetArtifactsDb()
		if db != nil {
			allMissions := append(db.GetMissionInfos(), db.GetMissionArchive()...)

			// Count by ship for VIRTUE missions
			virtueCount := make(map[int]int, 11)
			stdCount := make(map[int]int, 11)
			for _, m := range allMissions {
				ship := int(m.GetShip())
				if m.GetType() == ei.MissionInfo_VIRTUE {
					virtueCount[ship]++
				} else {
					stdCount[ship]++
				}
			}

			for key, shipIdx := range shipVirtueIndex {
				emit(LBEntry{
					LBType:   key,
					Player:   userID,
					GameName: gameName,
					SnapDate: snapDate,
					Value:    float64(virtueCount[shipIdx]),
				})
			}
			for key, shipIdx := range shipStdIndex {
				emit(LBEntry{
					LBType:   key,
					Player:   userID,
					GameName: gameName,
					SnapDate: snapDate,
					Value:    float64(stdCount[shipIdx]),
				})
			}
		}
	}

	// ── CXP weekly delta (contract archive) ───────────────────────────────────
	if isOptedIn(LBCXPWeeklyDelta) && archive != nil {
		totalCXP := 0.0
		for _, lc := range archive {
			eval := lc.GetEvaluation()
			if eval != nil {
				totalCXP += eval.GetCxp()
			}
		}

		delta := totalCXP // first run: delta = total
		if priorCXPTotal > 0 {
			delta = totalCXP - priorCXPTotal
		}
		emit(LBEntry{
			LBType:   LBCXPWeeklyDelta,
			Player:   userID,
			GameName: gameName,
			SnapDate: snapDate,
			Value:    delta,
			Details:  fmt.Sprintf("total:%.0f", totalCXP),
		})
	}

	return entries
}
