package farmerstate

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

// SetFarmerBackupDetails updates the player's saved game name ("ei_ign"), Truth Eggs ("TE"),
// and Ultra status from their Egg Inc backup. It returns the updated game name, whether the name changed,
// the updated Truth Egg count, and whether the Truth Egg count changed.
func SetFarmerBackupDetails(userID string, backup *ei.Backup) (newIGN string, ignChanged bool, newTE uint32, teChanged bool) {
	oldIGN := GetMiscSettingString(userID, "ei_ign")
	oldTEStr := GetMiscSettingString(userID, "TE")

	if backup == nil {
		var te uint32
		if v, err := strconv.ParseUint(oldTEStr, 10, 32); err == nil {
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
		newTE = ei.GetCurrentTruthEggs(backup)
		if oldTEStr != fmt.Sprintf("%d", newTE) {
			teChanged = true
		}
		SetMiscSettingString(userID, "TE", fmt.Sprintf("%d", newTE))
	} else {
		if v, err := strconv.ParseUint(oldTEStr, 10, 32); err == nil {
			newTE = uint32(v)
		}
	}

	return newIGN, ignChanged, newTE, teChanged
}
