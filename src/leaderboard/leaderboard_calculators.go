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
var virtueEggIndex = map[string]int{
	LBTECuriosity:    0,
	LBTEIntegrity:    1,
	LBTEHumility:     2,
	LBTEResilience:   3,
	LBTEKindness:     4,
	LBEggsCuriosity:  0,
	LBEggsIntegrity:  1,
	LBEggsHumility:   2,
	LBEggsResilience: 3,
	LBEggsKindness:   4,
}

// ─── CollectionResult carries the outputs of RunCalculators ──────────────────

// RunCalculators evaluates all opted-in leaderboard types for a single player
// from their first-contact backup.
//
// archive is the contract archive result (used only for SourceContractArchive types).
// Pass nil for archive if only SourceFirstContact types are being evaluated.
//
// snapDate is the ISO date string "YYYY-MM-DD" for this collection run.
func RunCalculators(
	userID string,
	backup *ei.Backup,
	archive []*ei.LocalContract,
	optedIn []string,
	snapDate string,
) []LBEntry {
	if backup == nil && archive == nil {
		return nil
	}

	gameName := ""
	if backup != nil {
		gameName = backup.GetUserName()
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
	if virtue != nil {
		emit(LBEntry{
			LBType:   LBVirtueShifts,
			Player:   userID,
			GameName: gameName,
			SnapDate: snapDate,
			Value:    float64(virtue.GetShiftCount()),
		})
	}

	// Truth Eggs & raw eggs per virtue egg
	if virtue != nil {
		deliveries := virtue.GetEggsDelivered()
		for key, idx := range virtueEggIndex {
			if idx >= len(deliveries) {
				continue
			}
			delivered := deliveries[idx]

			switch key {
			case LBTECuriosity, LBTEIntegrity, LBTEHumility, LBTEResilience, LBTEKindness:
				// Integer TE count
				te := float64(ei.CountTruthEggTiersPassed(delivered))
				emit(LBEntry{LBType: key, Player: userID, GameName: gameName, SnapDate: snapDate, Value: te})

			case LBEggsCuriosity, LBEggsIntegrity, LBEggsHumility, LBEggsResilience, LBEggsKindness:
				// Raw delivered eggs; detail = TE count
				te := ei.CountTruthEggTiersPassed(delivered)
				detail := fmt.Sprintf("%d TE", te)
				emit(LBEntry{LBType: key, Player: userID, GameName: gameName, SnapDate: snapDate, Value: delivered, Details: detail})
			}
		}

		// Total TE across all 5 eggs
		if isOptedIn(LBTETotal) {
			total := 0
			for i := 0; i < 5 && i < len(deliveries); i++ {
				total += int(ei.CountTruthEggTiersPassed(deliveries[i]))
			}
			emit(LBEntry{LBType: LBTETotal, Player: userID, GameName: gameName, SnapDate: snapDate, Value: float64(total)})
		}
	}

	// ── Prestige and Drone stats ──────────────────────────────────────────────
	if backup != nil {
		game := backup.GetGame()
		stats := backup.GetStats()

		if game != nil {
			emit(LBEntry{LBType: LBSoulEggs, Player: userID, GameName: gameName, SnapDate: snapDate, Value: game.GetSoulEggsD()})

			// For Earnings Bonus, we need total Truth Eggs (EoV)
			totalTE := 0.0
			if virtue != nil {
				deliveries := virtue.GetEggsDelivered()
				for i := 0; i < 5 && i < len(deliveries); i++ {
					totalTE += float64(ei.CountTruthEggTiersPassed(deliveries[i]))
				}
			}
			emit(LBEntry{LBType: LBEarningsBonus, Player: userID, GameName: gameName, SnapDate: snapDate, Value: ei.GetEarningsBonus(backup, totalTE)})
		}

		if stats != nil {
			emit(LBEntry{LBType: LBDrones, Player: userID, GameName: gameName, SnapDate: snapDate, Value: float64(stats.GetDroneTakedowns())})
			emit(LBEntry{LBType: LBEliteDrones, Player: userID, GameName: gameName, SnapDate: snapDate, Value: float64(stats.GetDroneTakedownsElite())})
			emit(LBEntry{LBType: LBPrestiges, Player: userID, GameName: gameName, SnapDate: snapDate, Value: float64(stats.GetNumPrestiges())})
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
			totalScore := float64(blue*1 + purple*2 + orange*3)
			if totalScore > 0 {
				detail := fmt.Sprintf("%d, %d, %d", blue, purple, orange)
				emit(LBEntry{LBType: LBSoulMirrors, Player: userID, GameName: gameName, SnapDate: snapDate, Value: totalScore, Details: detail})
			}
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

		// Look up previous week's stored total (kept in the details field).
		prior := GetPriorStatForPlayer(LBCXPWeeklyDelta, userID)
		delta := totalCXP // first run: delta = total
		if prior != nil {
			// Prior details stores the raw CXP total at that snapshot.
			var priorTotal float64
			if _, err := fmt.Sscanf(prior.Details, "total:%f", &priorTotal); err == nil {
				delta = totalCXP - priorTotal
			}
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
