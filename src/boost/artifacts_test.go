package boost

import (
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
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
