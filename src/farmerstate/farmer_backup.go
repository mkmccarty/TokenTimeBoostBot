package farmerstate

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

// SetFarmerBackupDetails updates the player's saved game name ("ei_ign"), Truth Eggs ("TE"),
// Ultra status, and Permit Level from their Egg Inc backup. It returns the updated game name, whether the name changed,
// the updated Truth Egg count, and whether the Truth Egg count changed.
func SetFarmerBackupDetails(userID string, backup *ei.Backup) (newIGN string, ignChanged bool, newTE uint32, teChanged bool) {
	oldIGN := GetMiscSettingString(userID, "ei_ign")
	oldTEStr := GetMiscSettingString(userID, "TE")

	if backup == nil {
		var te uint32
		if v, err := strconv.Atoi(oldTEStr); err == nil && v >= 0 && v <= ei.MaxTruthEggs {
			te = uint32(v)
		}
		return oldIGN, false, te, false
	}

	// Update Ultra status
	isUltra := false
	if sub := backup.GetSubInfo(); sub != nil {
		isUltra = (sub.GetStatus() == ei.UserSubscriptionInfo_ACTIVE)
	}
	SetUltra(userID, isUltra)

	// Update game name (ei_ign)
	newIGN = strings.TrimSpace(backup.GetUserName())
	if newIGN != "" && newIGN != oldIGN {
		SetMiscSettingString(userID, "ei_ign", newIGN)
		ignChanged = true
	} else if newIGN == "" {
		newIGN = oldIGN
	}

	// Update Truth Eggs (TE)
	if backup.GetVirtue() != nil || oldTEStr == "" {
		newTE = min(ei.GetCurrentTruthEggs(backup), ei.MaxTruthEggs)
		if oldTEStr != fmt.Sprintf("%d", newTE) {
			teChanged = true
		}
		SetMiscSettingString(userID, "TE", fmt.Sprintf("%d", newTE))
	} else {
		if v, err := strconv.Atoi(oldTEStr); err == nil && v >= 0 && v <= ei.MaxTruthEggs {
			newTE = uint32(v)
		}
	}

	// Update Permit Level
	if game := backup.GetGame(); game != nil {
		SetPermitLevel(userID, game.GetPermitLevel())
	}

	return newIGN, ignChanged, newTE, teChanged
}
