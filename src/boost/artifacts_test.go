package boost

import (
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

func TestArtifactCommandClearsManualIHROverride(t *testing.T) {
	client := newTestClient()
	userID := "123456789012345678"

	// Set manual IHR override
	farmerstate.SetMiscSettingString(userID, "IHR", "99")
	farmerstate.SetMiscSettingString(userID, "TE", "100")
	farmerstate.SetMiscSettingString(userID, "chalice", "T4L")

	if got := farmerstate.GetMiscSettingString(userID, "IHR"); got != "99" {
		t.Fatalf("expected initial manual IHR '99', got '%s'", got)
	}

	// Create a mock contract with this user
	contractID := "contract-art-test"
	guildID := "987654321098765432"
	channelID := "555555555555555555"
	contract, err := CreateContract(client, contractID, "coop-art-test", ContractPlaystyleChill, 10, ContractOrderFair, guildID, channelID, []string{userID}, userID, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("CreateContract failed: %v", err)
	}
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract.ContractHash)
		ContractsMutex.Unlock()
	}()

	// Set booster's initial rate to manual 99
	contract.mutex.Lock()
	if b, ok := contract.Boosters[userID]; ok {
		b.IHRRate = 99.0
	}
	contract.mutex.Unlock()

	payload := `{
		"id": "100", "application_id": "200", "type": 2, "token": "tok", "version": 1,
		"channel": {"id": "` + channelID + `", "type": 0},
		"guild_id": "` + guildID + `",
		"member": {"user": {"id": "` + userID + `", "username": "tester", "discriminator": "0"}, "roles": [], "joined_at": "2026-01-01T00:00:00Z"},
		"data": {"id": "1", "name": "artifact", "type": 1, "options": []}
	}`
	cmdEvent, err := dc.NewCommandEventFromPayload([]byte(payload))
	if err != nil {
		t.Fatalf("NewCommandEventFromPayload failed: %v", err)
	}

	HandleArtifactCommand(client, cmdEvent)

	// Verify manual IHR override was cleared in farmerstate
	if got := farmerstate.GetMiscSettingString(userID, "IHR"); got != "" {
		t.Errorf("expected manual IHR to be empty, got '%s'", got)
	}

	// Verify contract booster IHRRate was recalculated from known artifact values (> 99.0)
	contract.mutex.Lock()
	booster := contract.Boosters[userID]
	ihrRate := booster.IHRRate
	contract.mutex.Unlock()

	expectedRate, _ := CalculateIHRRateFromDB(userID)
	if ihrRate != expectedRate || ihrRate == 99.0 {
		t.Errorf("expected booster.IHRRate = %f, got %f", expectedRate, ihrRate)
	}
}

func TestPopulateFarmerBackupDetails(t *testing.T) {
	userID := "userA-te-test"

	farmerstate.SetMiscSettingString(userID, "ei_ign", "OldIgn")
	farmerstate.SetMiscSettingString(userID, "TE", "5")
	farmerstate.SetMiscSettingString(userID, "chalice", "T4L")

	maker := ei.NewBackupMaker("EI1234567890123456", "UpdatedIgn")
	backup := maker.GetBackup()
	backup.Virtue = &ei.Backup_Virtue{
		EovEarned:     []uint32{5, 5, 5, 5, 5},
		EggsDelivered: []float64{1e18, 1e18, 1e18, 1e18, 1e18},
	}

	newIGN, ignChanged, te, teChanged := farmerstate.SetFarmerBackupDetails(userID, backup)
	if newIGN != "UpdatedIgn" || !ignChanged {
		t.Errorf("expected newIGN='UpdatedIgn' and ignChanged=true, got '%s', %t", newIGN, ignChanged)
	}
	if te != 25 || !teChanged {
		t.Fatalf("expected te = 25 and teChanged=true, got %d, %t", te, teChanged)
	}

	savedIGN := farmerstate.GetMiscSettingString(userID, "ei_ign")
	if savedIGN != "UpdatedIgn" {
		t.Errorf("expected saved ei_ign = 'UpdatedIgn', got '%s'", savedIGN)
	}

	savedTE := farmerstate.GetMiscSettingString(userID, "TE")
	if savedTE != "25" {
		t.Errorf("expected saved TE = '25', got '%s'", savedTE)
	}

	ihrRate, logStr := CalculateIHRRateFromDB(userID)
	if ihrRate <= DefaultLeggyIHR {
		t.Errorf("expected calculated IHR rate to exceed DefaultLeggyIHR, got %f (%s)", ihrRate, logStr)
	}

	// Calling again with the same backup should not flag as changed
	newIGN2, ignChanged2, te2, teChanged2 := farmerstate.SetFarmerBackupDetails(userID, backup)
	if newIGN2 != "UpdatedIgn" || ignChanged2 || te2 != 25 || teChanged2 {
		t.Errorf("expected newIGN='UpdatedIgn', ignChanged=false, te=25, teChanged=false; got newIGN='%s', ignChanged=%t, te=%d, teChanged=%t",
			newIGN2, ignChanged2, te2, teChanged2)
	}
}

func TestArtifactCommandSyncsBoosterTE(t *testing.T) {
	client := newTestClient()
	userID := "userA-sync-te"

	farmerstate.SetMiscSettingString(userID, "TE", "42")

	contractID := "contract-sync-te"
	guildID := "987654321098765432"
	channelID := "555555555555555556"
	contract, err := CreateContract(client, contractID, "coop-sync-te", ContractPlaystyleChill, 10, ContractOrderFair, guildID, channelID, []string{userID}, userID, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("CreateContract failed: %v", err)
	}
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract.ContractHash)
		ContractsMutex.Unlock()
	}()

	contract.mutex.Lock()
	if b, ok := contract.Boosters[userID]; ok {
		b.TECount = 0
	}
	contract.mutex.Unlock()

	updateFarmerInContracts(client, userID, "artifacts", 0)

	contract.mutex.Lock()
	booster := contract.Boosters[userID]
	teCount := booster.TECount
	contract.mutex.Unlock()

	if teCount != 42 {
		t.Errorf("expected booster.TECount = 42, got %d", teCount)
	}
}
