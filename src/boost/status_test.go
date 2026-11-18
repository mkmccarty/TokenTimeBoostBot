package boost

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

func TestGetSlashStatusCommand(t *testing.T) {
	cmd := GetSlashStatusCommand("status")
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}
	if cmd.Name != "status" {
		t.Errorf("Name = %q, want %q", cmd.Name, "status")
	}
}

func TestHandleStatusCommand_EphemeralAndContent(t *testing.T) {
	guildID := "900000000000000301"
	channelID := "900000000000000302"
	mainID := "900000000000000310"
	actAsAltID := "900000000000000311"
	linkedDiscordAltID := "900000000000000312"
	guestAlt := "GuestStatusAlt"

	farmerstate.SetMiscSettingString(mainID, "ei_ign", "MainFarmer")
	farmerstate.SetMiscSettingString(mainID, "encrypted_ei_id", "dummy-encrypted")
	farmerstate.SetMiscSettingString(linkedDiscordAltID, "AltController", mainID)
	farmerstate.SetMiscSettingString(linkedDiscordAltID, "ei_ign", "AltDiscordFarmer")
	farmerstate.SetMiscSettingString(guestAlt, "AltController", mainID)
	farmerstate.SetMiscSettingString(actAsAltID, "ei_ign", "ActAsFarmer")

	_ = farmerstate.AddActAsLink(mainID, actAsAltID)
	_ = farmerstate.SetActAsSwitch(mainID, channelID, actAsAltID, time.Now().Add(1*time.Hour))
	defer func() {
		farmerstate.SetMiscSettingString(mainID, "ei_ign", "")
		farmerstate.SetMiscSettingString(mainID, "encrypted_ei_id", "")
		farmerstate.SetMiscSettingString(linkedDiscordAltID, "AltController", "")
		farmerstate.SetMiscSettingString(linkedDiscordAltID, "ei_ign", "")
		farmerstate.SetMiscSettingString(guestAlt, "AltController", "")
		farmerstate.SetMiscSettingString(actAsAltID, "ei_ign", "")
		_ = farmerstate.ClearActAsSwitch(mainID, channelID)
		_ = farmerstate.RemoveActAsLink(mainID, actAsAltID)
	}()

	contract := &Contract{
		ContractHash: "test-hash-status-cmd",
		ContractID:   "status-contract",
		CoopID:       "status-coop",
		State:        ContractStateFastrun,
		Location: []*LocationData{{
			GuildID:        guildID,
			ChannelID:      channelID,
			ChannelMention: "<#" + channelID + ">",
			ListMsgID:      "900000000000000399",
		}},
		Boosters: map[string]*Booster{
			mainID:     {UserID: mainID, Name: "MainFarmer"},
			actAsAltID: {UserID: actAsAltID, Name: "ActAsFarmer"},
		},
		WaitlistBoosters: []string{guestAlt},
	}
	ContractsMutex.Lock()
	Contracts[contract.ContractHash] = contract
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract.ContractHash)
		ContractsMutex.Unlock()
	}()

	payload := fmt.Sprintf(`{
		"id": "103",
		"application_id": "200",
		"type": 2,
		"token": "tok",
		"version": 1,
		"guild_id": %q,
		"channel": {"id": %q, "type": 0},
		"member": {"user": {"id": %q, "username": "main", "discriminator": "0"}},
		"user": {"id": %q, "username": "main", "discriminator": "0"},
		"data": {"id": "1", "name": "status", "type": 1}
	}`, guildID, channelID, mainID, mainID)
	e, err := dc.NewCommandEventFromPayload([]byte(payload))
	if err != nil {
		t.Fatalf("failed to create command event: %v", err)
	}

	HandleStatusCommand(e)
	resp := e.LastResponse
	if resp == nil {
		t.Fatal("expected non-nil LastResponse")
	}
	if !resp.Ephemeral {
		t.Errorf("expected ephemeral response, got Ephemeral=%v", resp.Ephemeral)
	}

	checks := []string{
		"MainFarmer",
		"EI ID:** Registered",
		"<@" + actAsAltID + ">",
		"<@" + linkedDiscordAltID + ">",
		"`" + guestAlt + "`",
		"<#" + channelID + "> (Fastrun)",
		"[status-contract / status-coop](https://discord.com/channels/" + guildID + "/" + channelID + "/900000000000000399)",
		"`" + guestAlt + "` (waitlist)",
	}
	for _, want := range checks {
		if !strings.Contains(resp.Content, want) {
			t.Errorf("status output missing %q\nfull output:\n%s", want, resp.Content)
		}
	}
	if strings.Contains(resp.Content, "[<#") {
		t.Errorf("status output should not wrap channel mention in markdown link:\n%s", resp.Content)
	}
}
