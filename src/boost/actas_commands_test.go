package boost

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

func makeTestCommandPayload(name, userID, optionsJSON string) string {
	return fmt.Sprintf(`{
		"id": "123456789012345678",
		"application_id": "123456789012345678",
		"type": 2,
		"token": "test-token",
		"version": 1,
		"channel": {"id": "123456789012345678", "type": 0},
		"guild_id": "123456789012345678",
		"member": {
			"user": {"id": "%s", "username": "testuser", "discriminator": "0"},
			"roles": [],
			"joined_at": "2026-01-01T00:00:00Z"
		},
		"data": {
			"id": "1",
			"name": "%s",
			"type": 1,
			"options": %s
		}
	}`, userID, name, optionsJSON)
}

func TestActAs_StonesEffectiveUser(t *testing.T) {
	mainID := "100000000000000010"
	altID := "100000000000000020"

	const testChanID = "123456789012345678"
	_ = farmerstate.AddActAsLink(mainID, altID)
	_ = farmerstate.SetActAsSwitch(mainID, testChanID, altID, time.Now().Add(1*time.Hour))
	defer func() {
		_ = farmerstate.ClearActAsSwitch(mainID, testChanID)
		_ = farmerstate.RemoveActAsLink(mainID, altID)
	}()

	// Alt has distinct stone-details setting
	farmerstate.SetMiscSettingFlag(mainID, "stone-details", false)
	farmerstate.SetMiscSettingFlag(altID, "stone-details", true)

	effectiveUser := farmerstate.GetEffectiveUserID(mainID, testChanID)
	if effectiveUser != altID {
		t.Fatalf("expected effective user %s, got %s", altID, effectiveUser)
	}

	details := farmerstate.GetMiscSettingFlag(effectiveUser, "stone-details")
	if !details {
		t.Errorf("expected effective user to have stone-details=true from alt")
	}
}

func TestActAs_TeamworkEffectiveUser(t *testing.T) {
	mainID := "100000000000000011"
	altID := "100000000000000021"

	const testChanID = "123456789012345678"
	_ = farmerstate.AddActAsLink(mainID, altID)
	_ = farmerstate.SetActAsSwitch(mainID, testChanID, altID, time.Now().Add(1*time.Hour))
	defer func() {
		_ = farmerstate.ClearActAsSwitch(mainID, testChanID)
		_ = farmerstate.RemoveActAsLink(mainID, altID)
	}()

	farmerstate.SetMiscSettingString(mainID, "EggIncRawName", "MainFarmer")
	farmerstate.SetMiscSettingString(altID, "EggIncRawName", "AltFarmer")

	effectiveUser := farmerstate.GetEffectiveUserID(mainID, testChanID)
	ign := farmerstate.GetMiscSettingString(effectiveUser, "EggIncRawName")
	if ign != "AltFarmer" {
		t.Errorf("expected effective teamwork IGN to be AltFarmer, got %s", ign)
	}
}

func TestActAs_CsEstimateParams(t *testing.T) {
	mainID := "100000000000000012"
	altID := "100000000000000022"

	const testChanID = "123456789012345678"
	_ = farmerstate.AddActAsLink(mainID, altID)
	_ = farmerstate.SetActAsSwitch(mainID, testChanID, altID, time.Now().Add(1*time.Hour))
	defer func() {
		_ = farmerstate.ClearActAsSwitch(mainID, testChanID)
		_ = farmerstate.RemoveActAsLink(mainID, altID)
	}()

	farmerstate.SetMiscSettingFlag(mainID, "sr-mode", false)
	farmerstate.SetMiscSettingFlag(altID, "sr-mode", false)

	// User mainID runs cs-estimate setting sr-mode=true
	payload := makeTestCommandPayload("cs-estimate", mainID, `[{"name":"sr-mode","type":5,"value":true}]`)
	ev, err := dc.NewCommandEventFromPayload([]byte(payload))
	if err != nil {
		t.Fatalf("failed to create command event: %v", err)
	}

	params := parseCsEstimateParams(ev)
	if !params.srMode {
		t.Errorf("expected srMode=true in parsed params")
	}

	// Flag should be saved to altID, not mainID
	if farmerstate.GetMiscSettingFlag(mainID, "sr-mode") {
		t.Errorf("mainID sr-mode should not have been set")
	}
	if !farmerstate.GetMiscSettingFlag(altID, "sr-mode") {
		t.Errorf("altID sr-mode should have been set to true")
	}
}

func TestActAs_ContractReportEffectiveUser(t *testing.T) {
	mainID := "100000000000000013"
	altID := "100000000000000023"

	const testChanID = "123456789012345678"
	_ = farmerstate.AddActAsLink(mainID, altID)
	_ = farmerstate.SetActAsSwitch(mainID, testChanID, altID, time.Now().Add(1*time.Hour))
	defer func() {
		_ = farmerstate.ClearActAsSwitch(mainID, testChanID)
		_ = farmerstate.RemoveActAsLink(mainID, altID)
	}()

	farmerstate.SetMiscSettingString(mainID, "encrypted_ei_id", "main_enc_id")
	farmerstate.SetMiscSettingString(altID, "encrypted_ei_id", "alt_enc_id")

	effectiveUser := farmerstate.GetEffectiveUserID(mainID, testChanID)
	eiID := farmerstate.GetMiscSettingString(effectiveUser, "encrypted_ei_id")
	if eiID != "alt_enc_id" {
		t.Errorf("expected effective eiID to be alt_enc_id, got %s", eiID)
	}
}

func TestActAs_ContractReportEggIDModalSubmit(t *testing.T) {
	if config.Key == "" {
		config.Key = base64.StdEncoding.EncodeToString(make([]byte, 32))
	}

	mainID := "100000000000000014"
	altID := "100000000000000024"

	_ = farmerstate.AddActAsLink(mainID, altID)
	_ = farmerstate.SetActAsSwitch(mainID, "3", altID, time.Now().Add(1*time.Hour))
	defer func() {
		_ = farmerstate.ClearActAsSwitch(mainID, "3")
		_ = farmerstate.RemoveActAsLink(mainID, altID)
	}()

	payload := fmt.Sprintf(`{
		"id": "1",
		"application_id": "2",
		"type": 5,
		"token": "test-token",
		"version": 1,
		"channel": {"id": "3", "type": 0},
		"user": {"id": %q, "username": "tester", "discriminator": "0"},
		"data": {
			"custom_id": "m_eggid#contract-report",
			"components": [
				{"type": 18, "label": "Egg Inc ID", "component": {"type": 4, "custom_id": "egginc-id", "value": "EI1234567890123456"}},
				{"type": 18, "label": "Confirm", "component": {"type": 4, "custom_id": "confirm", "value": "save"}}
			]
		}
	}`, mainID)

	ev, err := dc.NewModalEventFromPayload([]byte(payload))
	if err != nil {
		t.Fatalf("failed to create modal event: %v", err)
	}

	HandleEggIDModalSubmit(ev)

	// Encrypted ID should have been saved to altID, not mainID
	if farmerstate.GetMiscSettingString(mainID, "encrypted_ei_id") != "" {
		t.Errorf("mainID should not have received encrypted_ei_id")
	}
	if farmerstate.GetMiscSettingString(altID, "encrypted_ei_id") == "" {
		t.Errorf("altID should have received encrypted_ei_id")
	}
}

func TestActAs_RerunEffectiveUser(t *testing.T) {
	mainID := "100000000000000015"
	altID := "100000000000000025"

	const testChanID = "123456789012345678"
	_ = farmerstate.AddActAsLink(mainID, altID)
	_ = farmerstate.SetActAsSwitch(mainID, testChanID, altID, time.Now().Add(1*time.Hour))
	defer func() {
		_ = farmerstate.ClearActAsSwitch(mainID, testChanID)
		_ = farmerstate.RemoveActAsLink(mainID, altID)
	}()

	farmerstate.SetMiscSettingString(mainID, "encrypted_ei_id", "main_enc_id")
	farmerstate.SetMiscSettingString(altID, "encrypted_ei_id", "alt_enc_id")
	farmerstate.SetMiscSettingString(mainID, "rerunMobileFriendly", "false")
	farmerstate.SetMiscSettingString(altID, "rerunMobileFriendly", "true")

	effectiveUser := farmerstate.GetEffectiveUserID(mainID, testChanID)
	if effectiveUser != altID {
		t.Fatalf("expected effective user %s, got %s", altID, effectiveUser)
	}

	eiID := farmerstate.GetMiscSettingString(effectiveUser, "encrypted_ei_id")
	if eiID != "alt_enc_id" {
		t.Errorf("expected effective eiID to be alt_enc_id, got %s", eiID)
	}

	mobile := farmerstate.GetMiscSettingString(effectiveUser, "rerunMobileFriendly")
	if mobile != "true" {
		t.Errorf("expected effective rerunMobileFriendly to be true, got %s", mobile)
	}
}

func TestActAs_RerunEggIDModalSubmit(t *testing.T) {
	if config.Key == "" {
		config.Key = base64.StdEncoding.EncodeToString(make([]byte, 32))
	}

	mainID := "100000000000000016"
	altID := "100000000000000026"

	_ = farmerstate.AddActAsLink(mainID, altID)
	_ = farmerstate.SetActAsSwitch(mainID, "3", altID, time.Now().Add(1*time.Hour))
	defer func() {
		_ = farmerstate.ClearActAsSwitch(mainID, "3")
		_ = farmerstate.RemoveActAsLink(mainID, altID)
	}()

	payload := fmt.Sprintf(`{
		"id": "1",
		"application_id": "2",
		"type": 5,
		"token": "test-token",
		"version": 1,
		"channel": {"id": "3", "type": 0},
		"user": {"id": %q, "username": "tester", "discriminator": "0"},
		"data": {
			"custom_id": "m_eggid#replay",
			"components": [
				{"type": 18, "label": "Egg Inc ID", "component": {"type": 4, "custom_id": "egginc-id", "value": "EI1234567890123456"}},
				{"type": 18, "label": "Confirm", "component": {"type": 4, "custom_id": "confirm", "value": "save"}}
			]
		}
	}`, mainID)

	ev, err := dc.NewModalEventFromPayload([]byte(payload))
	if err != nil {
		t.Fatalf("failed to create modal event: %v", err)
	}

	HandleEggIDModalSubmit(ev)

	// Encrypted ID should have been saved to altID, not mainID
	if farmerstate.GetMiscSettingString(mainID, "encrypted_ei_id") != "" {
		t.Errorf("mainID should not have received encrypted_ei_id")
	}
	if farmerstate.GetMiscSettingString(altID, "encrypted_ei_id") == "" {
		t.Errorf("altID should have received encrypted_ei_id")
	}
}

func TestActAs_RerunChartActionAuthorization(t *testing.T) {
	mainID := "100000000000000017"
	altID := "100000000000000027"

	_ = farmerstate.AddActAsLink(mainID, altID)
	_ = farmerstate.SetActAsSwitch(mainID, "3", altID, time.Now().Add(1*time.Hour))
	defer func() {
		_ = farmerstate.ClearActAsSwitch(mainID, "3")
		_ = farmerstate.RemoveActAsLink(mainID, altID)
	}()

	uuidStr := "test-chart-uuid-123"
	chartSessionsMutex.Lock()
	chartSessions[uuidStr] = &chartSession{
		uuidStr:   uuidStr,
		userID:    altID,
		expiresAt: time.Now().Add(5 * time.Minute),
	}
	chartSessionsMutex.Unlock()

	defer func() {
		chartSessionsMutex.Lock()
		delete(chartSessions, uuidStr)
		chartSessionsMutex.Unlock()
	}()

	// mainID clicks finish on a session owned by altID
	payload := fmt.Sprintf(`{
		"id": "1",
		"application_id": "2",
		"type": 3,
		"token": "test-token",
		"version": 1,
		"channel": {"id": "3", "type": 0},
		"user": {"id": %q, "username": "tester", "discriminator": "0"},
		"data": {
			"custom_id": "fd_rerun_chart#finish#%s",
			"component_type": 2
		}
	}`, mainID, uuidStr)

	ev, err := dc.NewComponentEventFromPayload([]byte(payload))
	if err != nil {
		t.Fatalf("failed to create component event: %v", err)
	}

	HandleChartReactions(ev)

	// Session should be deleted by finish action because mainID was authorized via act-as
	chartSessionsMutex.Lock()
	_, exists := chartSessions[uuidStr]
	chartSessionsMutex.Unlock()

	if exists {
		t.Errorf("expected chart session to be finished/deleted by act-as user")
	}
}

func TestActAs_SignupJoinAndLeave(t *testing.T) {
	mainID := "100000000000000030"
	altID := "100000000000000040"
	guildID := "200000000000000001"
	channelID := "300000000000000001"

	client := dctest.New().
		WithGuild(guildID, "Test Guild").
		WithChannel(channelID, guildID, "contract-channel").
		WithUser(mainID, "mainUser", "Main User").
		WithUser(altID, "altUser", "Alt User")

	_ = farmerstate.AddActAsLink(mainID, altID)
	_ = farmerstate.SetActAsSwitch(mainID, channelID, altID, time.Now().Add(1*time.Hour))
	defer func() {
		_ = farmerstate.ClearActAsSwitch(mainID, channelID)
		_ = farmerstate.RemoveActAsLink(mainID, altID)
	}()

	contract := &Contract{
		ContractHash: "test-hash-actas-join",
		ContractID:   "test-contract",
		CoopID:       "test-coop",
		CoopSize:     5,
		State:        ContractStateSignup,
		BoostOrder:   ContractOrderSignup,
		CreatorID:    []string{"creator-1"},
		Order:        []string{},
		Boosters:     map[string]*Booster{},
		Location: []*LocationData{
			{GuildID: guildID, ChannelID: channelID, ListMsgID: "msg-1"},
		},
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
		"id": "1",
		"application_id": "2",
		"type": 3,
		"token": "test-token",
		"version": 1,
		"channel": {"id": %q, "type": 0},
		"guild_id": %q,
		"user": {"id": %q, "username": "mainUser", "discriminator": "0"},
		"data": {
			"custom_id": "fd_signupFarmer#%s",
			"component_type": 2
		}
	}`, channelID, guildID, mainID, contract.ContractHash)
	ev, err := dc.NewComponentEventFromPayload([]byte(payload))
	if err != nil {
		t.Fatalf("failed to create component event: %v", err)
	}

	joinContract(client, ev, false)

	if !UserInContract(contract, altID) {
		t.Errorf("expected altID %s to be joined into contract", altID)
	}
	if UserInContract(contract, mainID) {
		t.Errorf("mainID %s should NOT have been joined into contract", mainID)
	}

	// Now leave
	leavePayload := fmt.Sprintf(`{
		"id": "2",
		"application_id": "2",
		"type": 3,
		"token": "test-token",
		"version": 1,
		"channel": {"id": %q, "type": 0},
		"guild_id": %q,
		"user": {"id": %q, "username": "mainUser", "discriminator": "0"},
		"data": {
			"custom_id": "fd_signupLeave#%s",
			"component_type": 2
		}
	}`, channelID, guildID, mainID, contract.ContractHash)
	leaveEv, err := dc.NewComponentEventFromPayload([]byte(leavePayload))
	if err != nil {
		t.Fatalf("failed to create leave event: %v", err)
	}

	HandleSignupLeave(client, leaveEv)

	if UserInContract(contract, altID) {
		t.Errorf("expected altID %s to be removed from contract after leave", altID)
	}
}

func TestActAs_BoostAndUnboostCommands(t *testing.T) {
	mainID := "100000000000000031"
	altID := "100000000000000041"
	guildID := "200000000000000002"
	channelID := "300000000000000002"

	client := dctest.New().
		WithGuild(guildID, "Test Guild").
		WithChannel(channelID, guildID, "contract-channel").
		WithUser(mainID, "mainUser", "Main User").
		WithUser(altID, "altUser", "Alt User")

	_ = farmerstate.AddActAsLink(mainID, altID)
	_ = farmerstate.SetActAsSwitch(mainID, channelID, altID, time.Now().Add(1*time.Hour))
	defer func() {
		_ = farmerstate.ClearActAsSwitch(mainID, channelID)
		_ = farmerstate.RemoveActAsLink(mainID, altID)
	}()

	contract := &Contract{
		ContractHash:         "test-hash-actas-boost",
		ContractID:           "test-contract",
		CoopID:               "test-coop",
		CoopSize:             3,
		State:                ContractStateFastrun,
		Style:                ContractFlagFastrun,
		BoostOrder:           ContractOrderSignup,
		CreatorID:            []string{altID},
		CurrentBoosterUserID: altID,
		BoostPosition:        0,
		Order:                []string{altID, "other-user"},
		Boosters: map[string]*Booster{
			altID:        {UserID: altID, Name: "Alt User", Nick: "altUser", Mention: "<@" + altID + ">", BoostState: BoostStateTokenTime},
			"other-user": {UserID: "other-user", Name: "Other", Nick: "other", Mention: "<@other-user>", BoostState: BoostStateUnboosted},
		},
		Location: []*LocationData{
			{GuildID: guildID, ChannelID: channelID, ListMsgID: "msg-1"},
		},
	}
	ContractsMutex.Lock()
	Contracts[contract.ContractHash] = contract
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract.ContractHash)
		ContractsMutex.Unlock()
	}()

	// mainID calls /boost
	boostPayload := fmt.Sprintf(`{
		"id": "1",
		"application_id": "2",
		"type": 2,
		"token": "test-token",
		"version": 1,
		"channel": {"id": %q, "type": 0},
		"guild_id": %q,
		"member": {
			"user": {"id": %q, "username": "mainUser", "discriminator": "0"},
			"roles": [],
			"joined_at": "2026-01-01T00:00:00Z"
		},
		"data": {
			"id": "1",
			"name": "boost",
			"type": 1,
			"options": []
		}
	}`, channelID, guildID, mainID)
	boostEv, err := dc.NewCommandEventFromPayload([]byte(boostPayload))
	if err != nil {
		t.Fatalf("failed to create boost event: %v", err)
	}

	HandleBoostCommand(client, boostEv)

	if contract.Boosters[altID].BoostState != BoostStateBoosted {
		t.Errorf("expected altID to be BoostStateBoosted, got %v", contract.Boosters[altID].BoostState)
	}

	// mainID calls /unboost with farmer empty
	unboostPayload := fmt.Sprintf(`{
		"id": "2",
		"application_id": "2",
		"type": 2,
		"token": "test-token",
		"version": 1,
		"channel": {"id": %q, "type": 0},
		"guild_id": %q,
		"member": {
			"user": {"id": %q, "username": "mainUser", "discriminator": "0"},
			"roles": [],
			"joined_at": "2026-01-01T00:00:00Z"
		},
		"data": {
			"id": "2",
			"name": "unboost",
			"type": 1,
			"options": []
		}
	}`, channelID, guildID, mainID)
	unboostEv, err := dc.NewCommandEventFromPayload([]byte(unboostPayload))
	if err != nil {
		t.Fatalf("failed to create unboost event: %v", err)
	}

	HandleUnboostCommand(client, unboostEv)

	if contract.Boosters[altID].BoostState == BoostStateBoosted {
		t.Errorf("expected altID to be unboosted")
	}
}

func TestActAs_ButtonReactionsAndTokens(t *testing.T) {
	mainID := "100000000000000032"
	altID := "100000000000000042"
	receiverID := "100000000000000052"
	guildID := "200000000000000003"
	channelID := "300000000000000003"

	client := dctest.New().
		WithGuild(guildID, "Test Guild").
		WithChannel(channelID, guildID, "contract-channel").
		WithUser(mainID, "mainUser", "Main User").
		WithUser(altID, "altUser", "Alt User").
		WithUser(receiverID, "receiverUser", "Receiver User")

	_ = farmerstate.AddActAsLink(mainID, altID)
	_ = farmerstate.SetActAsSwitch(mainID, channelID, altID, time.Now().Add(1*time.Hour))
	defer func() {
		_ = farmerstate.ClearActAsSwitch(mainID, channelID)
		_ = farmerstate.RemoveActAsLink(mainID, altID)
	}()

	contract := &Contract{
		ContractHash:         "test-hash-actas-reactions",
		ContractID:           "test-contract",
		CoopID:               "test-coop",
		CoopSize:             3,
		State:                ContractStateFastrun,
		Style:                ContractFlagFastrun,
		BoostOrder:           ContractOrderSignup,
		CreatorID:            []string{altID},
		CurrentBoosterUserID: receiverID,
		BoostPosition:        1,
		Order:                []string{altID, receiverID},
		Boosters: map[string]*Booster{
			altID:      {UserID: altID, Name: "Alt User", Nick: "altUser", Mention: "<@" + altID + ">", BoostState: BoostStateUnboosted, TokensWanted: 8},
			receiverID: {UserID: receiverID, Name: "Receiver", Nick: "receiver", Mention: "<@" + receiverID + ">", BoostState: BoostStateTokenTime, TokensWanted: 8},
		},
		Location: []*LocationData{
			{GuildID: guildID, ChannelID: channelID, ListMsgID: "msg-1"},
		},
	}
	ContractsMutex.Lock()
	Contracts[contract.ContractHash] = contract
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract.ContractHash)
		ContractsMutex.Unlock()
	}()

	// mainID sends token button reaction
	tokenPayload := fmt.Sprintf(`{
		"id": "1",
		"application_id": "2",
		"type": 3,
		"token": "test-token",
		"version": 1,
		"channel": {"id": %q, "type": 0},
		"guild_id": %q,
		"user": {"id": %q, "username": "mainUser", "discriminator": "0"},
		"data": {
			"custom_id": "rc_#token#%s",
			"component_type": 2
		}
	}`, channelID, guildID, mainID, contract.ContractHash)
	tokenEv, err := dc.NewComponentEventFromPayload([]byte(tokenPayload))
	if err != nil {
		t.Fatalf("failed to create token event: %v", err)
	}

	HandleContractReactions(client, tokenEv)

	contract.mutex.Lock()
	logLen := len(contract.TokenLog)
	var lastLogFrom string
	if logLen > 0 {
		lastLogFrom = contract.TokenLog[logLen-1].FromUserID
	}
	contract.mutex.Unlock()

	if lastLogFrom != altID {
		t.Errorf("expected token log to be from altID %s, got %s", altID, lastLogFrom)
	}

	// AddBoostTokens from mainID adjusts altID's token count
	addTokenPayload := fmt.Sprintf(`{
		"id": "2",
		"application_id": "2",
		"type": 3,
		"token": "test-token",
		"version": 1,
		"channel": {"id": %q, "type": 0},
		"guild_id": %q,
		"user": {"id": %q, "username": "mainUser", "discriminator": "0"},
		"data": {
			"custom_id": "fd_tokensPlus#%s",
			"component_type": 2
		}
	}`, channelID, guildID, mainID, contract.ContractHash)
	addTokenEv, err := dc.NewComponentEventFromPayload([]byte(addTokenPayload))
	if err != nil {
		t.Fatalf("failed to create add token event: %v", err)
	}

	wantedTokens, _, err := AddBoostTokens(client, addTokenEv, 0, 2)
	if err != nil {
		t.Fatalf("unexpected error in AddBoostTokens: %v", err)
	}
	if wantedTokens != 10 {
		t.Errorf("expected wantedTokens=10, got %d", wantedTokens)
	}
	if contract.Boosters[altID].TokensWanted != 10 {
		t.Errorf("expected altID TokensWanted=10, got %d", contract.Boosters[altID].TokensWanted)
	}
}

func TestActAs_ArtifactTargetResolution(t *testing.T) {
	mainID := "100000000000000033"
	altID := "100000000000000043"

	const testChanID = "123456789012345678"
	_ = farmerstate.AddActAsLink(mainID, altID)
	_ = farmerstate.SetActAsSwitch(mainID, testChanID, altID, time.Now().Add(1*time.Hour))
	defer func() {
		_ = farmerstate.ClearActAsSwitch(mainID, testChanID)
		_ = farmerstate.RemoveActAsLink(mainID, altID)
	}()

	// Without "alternate" option, resolves to effective ID (altID)
	payload := makeTestCommandPayload("artifacts", mainID, `[]`)
	ev, err := dc.NewCommandEventFromPayload([]byte(payload))
	if err != nil {
		t.Fatalf("failed to create command event: %v", err)
	}

	reqID, targetID, invalid := resolveArtifactTargetUserID(ev)
	if invalid {
		t.Fatalf("expected valid target")
	}
	if reqID != mainID {
		t.Errorf("expected requesterID=%s, got %s", mainID, reqID)
	}
	if targetID != altID {
		t.Errorf("expected targetID=%s, got %s", altID, targetID)
	}
}

func TestActAs_ContractSettingsEphemeralSwitchMemory(t *testing.T) {
	parentChan := "300000000000000010"
	threadChan := "300000000000000011"
	ephemeralMsgID := "400000000000000010"

	mainID := "100000000000000060"
	alt1 := "100000000000000061"
	alt2 := "100000000000000062"

	_ = farmerstate.AddActAsLink(mainID, alt1)
	_ = farmerstate.AddActAsLink(mainID, alt2)
	defer func() {
		_ = farmerstate.ClearAllActAsSwitches(mainID)
		_ = farmerstate.RemoveActAsLink(mainID, alt1)
		_ = farmerstate.RemoveActAsLink(mainID, alt2)
		farmerstate.ClearEphemeralMessageSwitch(ephemeralMsgID)
	}()

	// 1. User starts in parent channel switched to alt1
	_ = farmerstate.SetActAsSwitch(mainID, parentChan, alt1, time.Now().Add(2*time.Hour))
	if got := farmerstate.GetEffectiveUserID(mainID, parentChan); got != alt1 {
		t.Fatalf("expected mainID in parentChan to be switched to alt1, got %s", got)
	}

	// 2. Thread is created: switch moves from parentChan to threadChan
	err := farmerstate.MoveActAsSwitch(mainID, parentChan, threadChan)
	if err != nil {
		t.Fatalf("failed to move switch: %v", err)
	}
	if got := farmerstate.GetEffectiveUserID(mainID, parentChan); got != mainID {
		t.Errorf("expected switch to be removed from parentChan after move, got %s", got)
	}
	if got := farmerstate.GetEffectiveUserID(mainID, threadChan); got != alt1 {
		t.Errorf("expected switch to be active in threadChan for alt1, got %s", got)
	}

	// 3. Ephemeral settings message in parentChan is registered to remember it was launched under alt1
	farmerstate.RegisterEphemeralMessageSwitch(ephemeralMsgID, mainID, alt1)

	// 4. In threadChan, user now switches to alt2
	_ = farmerstate.SetActAsSwitch(mainID, threadChan, alt2, time.Now().Add(2*time.Hour))
	if got := farmerstate.GetEffectiveUserID(mainID, threadChan); got != alt2 {
		t.Fatalf("expected switch in threadChan to be alt2, got %s", got)
	}

	// 5. Ephemeral settings response from parentChan still resolves to alt1 via message ID
	if got := farmerstate.GetEffectiveUserID(mainID, ephemeralMsgID, parentChan); got != alt1 {
		t.Errorf("ephemeral message ID should remember alt1 switch, got %s, want %s", got, alt1)
	}

	// 6. Test HandleContractSettingsReactions using ephemeral message ID
	guildID := "200000000000000009"
	client := dctest.New().
		WithGuild(guildID, "Test Guild").
		WithChannel(threadChan, guildID, "contract-thread").
		WithUser(mainID, "mainUser", "Main User").
		WithUser(alt1, "alt1User", "Alt 1 User").
		WithUser(alt2, "alt2User", "Alt 2 User")

	contract := &Contract{
		ContractHash: "test-hash-settings-switch",
		ContractID:   "test-contract",
		CoopID:       "test-coop",
		CoopSize:     3,
		State:        ContractStateSignup,
		Style:        ContractFlagBanker,
		BoostOrder:   ContractOrderSignup,
		CreatorID:    []string{alt1, mainID},
		Order:        []string{alt1, alt2},
		Boosters: map[string]*Booster{
			alt1: {UserID: alt1, Name: "Alt1", Nick: "alt1", Mention: "<@" + alt1 + ">", BoostState: BoostStateUnboosted},
			alt2: {UserID: alt2, Name: "Alt2", Nick: "alt2", Mention: "<@" + alt2 + ">", BoostState: BoostStateUnboosted},
		},
		Banker: BankerInfo{
			BoostingSinkUserID: "",
		},
		Location: []*LocationData{
			{GuildID: guildID, ChannelID: threadChan, ListMsgID: "msg-1"},
		},
	}
	ContractsMutex.Lock()
	Contracts[contract.ContractHash] = contract
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contract.ContractHash)
		ContractsMutex.Unlock()
	}()

	// mainID clicks Boost Sink button on ephemeral message (message ID = ephemeralMsgID)
	sinkPayload := fmt.Sprintf(`{
		"id": "1",
		"application_id": "2",
		"type": 3,
		"token": "test-token",
		"version": 1,
		"channel": {"id": %q, "type": 0},
		"guild_id": %q,
		"message": {"id": %q},
		"user": {"id": %q, "username": "mainUser", "discriminator": "0"},
		"data": {
			"custom_id": "cs_#boostsink#%s",
			"component_type": 2
		}
	}`, parentChan, guildID, ephemeralMsgID, mainID, contract.ContractHash)
	sinkEv, err := dc.NewComponentEventFromPayload([]byte(sinkPayload))
	if err != nil {
		t.Fatalf("failed to create component event: %v", err)
	}

	HandleContractSettingsReactions(client, sinkEv)

	// Since the ephemeral message knows it was launched under alt1, BoostingSinkUserID should be alt1
	if contract.Banker.BoostingSinkUserID != alt1 {
		t.Errorf("expected BoostingSinkUserID to be %s from original switch, got %s", alt1, contract.Banker.BoostingSinkUserID)
	}
}

func TestActAs_ContractCommandExtendsDurationToForever(t *testing.T) {
	parentChan := "300000000000000088"
	guildID := "200000000000000088"
	mainID := "100000000000000088"
	altID := "100000000000000089"

	client := dctest.New().
		WithGuild(guildID, "Test Guild").
		WithChannel(parentChan, guildID, "announcements").
		WithUser(mainID, "mainUser", "Main User").
		WithUser(altID, "altUser", "Alt User").
		WithPermissions(config.DiscordAppID, parentChan, dc.Permissions(1<<35))

	_ = farmerstate.AddActAsLink(mainID, altID)
	defer func() {
		_ = farmerstate.ClearAllActAsSwitches(mainID)
		_ = farmerstate.RemoveActAsLink(mainID, altID)
	}()

	// User switches to altID in parentChan with 1 hour duration
	_ = farmerstate.SetActAsSwitch(mainID, parentChan, altID, time.Now().Add(1*time.Hour))
	_, expBefore, _ := farmerstate.GetActAsSwitch(mainID, parentChan)
	if farmerstate.IsActAsForever(expBefore) {
		t.Fatalf("expected non-forever switch in parentChan initially")
	}

	payload := fmt.Sprintf(`{
		"id": "1",
		"application_id": "2",
		"type": 2,
		"token": "test-token",
		"version": 1,
		"channel": {"id": %q, "type": 0},
		"guild_id": %q,
		"member": {
			"user": {"id": %q, "username": "mainUser", "discriminator": "0"},
			"roles": [],
			"joined_at": "2026-01-01T00:00:00Z"
		},
		"data": {
			"id": "1",
			"name": "contract",
			"type": 1,
			"options": [
				{"name": "contract-id", "type": 3, "value": "test-c"},
				{"name": "coop-id", "type": 3, "value": "test-coop"},
				{"name": "coop-size", "type": 4, "value": 5},
				{"name": "make-thread", "type": 5, "value": true}
			]
		}
	}`, parentChan, guildID, mainID)

	ev, err := dc.NewCommandEventFromPayload([]byte(payload))
	if err != nil {
		t.Fatalf("failed to create command event: %v", err)
	}

	HandleContractCommand(client, ev)

	// In parent channel, switch should be moved away
	if got := farmerstate.GetEffectiveUserID(mainID, parentChan); got != mainID {
		t.Errorf("parent channel should have switch removed, got %s", got)
	}

	// Find the spawned thread in client.Channels
	var threadChanID string
	for chID, ch := range client.Channels {
		if ch.IsThread && ch.ParentID == parentChan {
			threadChanID = chID
			break
		}
	}
	if threadChanID == "" {
		t.Fatalf("expected spawned thread channel to be created")
	}

	// Verify the spawned thread has active switch for altID and duration is forever
	altGot, expAfter, active := farmerstate.GetActAsSwitch(mainID, threadChanID)
	if !active || altGot != altID {
		t.Fatalf("expected thread %s to have active switch for %s, got active=%v alt=%s", threadChanID, altID, active, altGot)
	}
	if !farmerstate.IsActAsForever(expAfter) {
		t.Errorf("expected thread switch duration to be forever, got %v", expAfter)
	}

	// Verify additional followup in original channel notifying user they were switched back to main profile
	foundNotice := false
	for _, f := range ev.Followups {
		if strings.Contains(f.Content, "main profile") && strings.Contains(f.Content, threadChanID) {
			foundNotice = true
			if !f.Ephemeral {
				t.Errorf("expected followup notice to be ephemeral")
			}
			break
		}
	}
	if !foundNotice {
		t.Errorf("expected additional followup notifying user of switch back to main profile, got followups: %+v", ev.Followups)
	}

	// Clean up created contract
	ContractsMutex.Lock()
	for h, c := range Contracts {
		if c.CoopID == "test-coop" {
			delete(Contracts, h)
		}
	}
	ContractsMutex.Unlock()
}
