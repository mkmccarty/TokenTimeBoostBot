package farmerstate

import (
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

func TestSetFarmerBackupDetails(t *testing.T) {
	userID := "userA-backup-sync"

	// Initially empty
	SetMiscSettingString(userID, "ei_ign", "OldName")
	SetMiscSettingString(userID, "TE", "5")

	maker := ei.NewBackupMaker("EI1234567890123456", "NewFarmerName")
	backup := maker.GetBackup()
	backup.Virtue = &ei.Backup_Virtue{
		EovEarned:     []uint32{2, 2, 2, 2, 2},
		EggsDelivered: []float64{1e18, 1e18, 1e18, 1e18, 1e18},
	}

	newIGN, ignChanged, newTE, teChanged := SetFarmerBackupDetails(userID, backup)
	if newIGN != "NewFarmerName" || !ignChanged {
		t.Errorf("expected newIGN='NewFarmerName' and ignChanged=true, got '%s', %t", newIGN, ignChanged)
	}
	if newTE != 10 || !teChanged {
		t.Errorf("expected newTE=10 and teChanged=true, got %d, %t", newTE, teChanged)
	}

	if got := GetMiscSettingString(userID, "ei_ign"); got != "NewFarmerName" {
		t.Errorf("expected saved ei_ign='NewFarmerName', got '%s'", got)
	}
	if got := GetMiscSettingString(userID, "TE"); got != "10" {
		t.Errorf("expected saved TE='10', got '%s'", got)
	}

	// Calling again with the same backup should report no changes
	newIGN2, ignChanged2, newTE2, teChanged2 := SetFarmerBackupDetails(userID, backup)
	if newIGN2 != "NewFarmerName" || ignChanged2 {
		t.Errorf("expected newIGN='NewFarmerName' and ignChanged=false, got '%s', %t", newIGN2, ignChanged2)
	}
	if newTE2 != 10 || teChanged2 {
		t.Errorf("expected newTE=10 and teChanged=false, got %d, %t", newTE2, teChanged2)
	}

	// Calling with nil backup should return current values with no changes
	newIGN3, ignChanged3, newTE3, teChanged3 := SetFarmerBackupDetails(userID, nil)
	if newIGN3 != "NewFarmerName" || ignChanged3 {
		t.Errorf("expected newIGN='NewFarmerName' and ignChanged=false for nil backup, got '%s', %t", newIGN3, ignChanged3)
	}
	if newTE3 != 10 || teChanged3 {
		t.Errorf("expected newTE=10 and teChanged=false for nil backup, got %d, %t", newTE3, teChanged3)
	}
}
