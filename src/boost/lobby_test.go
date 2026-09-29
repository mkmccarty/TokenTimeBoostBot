package boost

import (
	"strings"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

func TestBoosterDisplayName(t *testing.T) {
	tests := []struct {
		name      string
		mention   string
		nick      string
		discordID string
		want      string
	}{
		{
			name:      "mention with <@",
			mention:   "<@123456789>",
			nick:      "SomeNick",
			discordID: "123456789",
			want:      "<@123456789>",
		},
		{
			name:      "nick without mention",
			mention:   "",
			nick:      "FarmerJohn",
			discordID: "123456789",
			want:      "`FarmerJohn`",
		},
		{
			name:      "plain text mention",
			mention:   "PlainMention",
			nick:      "",
			discordID: "123456789",
			want:      "`PlainMention`",
		},
		{
			name:      "empty mention and nick fallbacks to discordID",
			mention:   "",
			nick:      "",
			discordID: "123456789",
			want:      "<@123456789>",
		},
		{
			name:      "all empty",
			mention:   "",
			nick:      "",
			discordID: "",
			want:      "`Unknown`",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := boosterDisplayName(tc.mention, tc.nick, tc.discordID)
			if got != tc.want {
				t.Errorf("boosterDisplayName(%q, %q, %q) = %q, want %q", tc.mention, tc.nick, tc.discordID, got, tc.want)
			}
		})
	}
}

func TestBuildLobbyMismatchSection(t *testing.T) {
	contributors := []*ei.ContractCoopStatusResponse_ContributionInfo{
		{
			UserId:   protoRef("user-1"),
			UserName: protoRef("Alice"),
		},
		{
			UserId:   protoRef("user-2"),
			UserName: protoRef("Bob"),
		},
	}

	contract := &Contract{
		ContractID: "test-contract",
		CoopID:     "test-coop",
		Boosters:   make(map[string]*Booster),
	}
	contract.Boosters["1001"] = &Booster{
		UserID:  "1001",
		Nick:    "Alice",
		Mention: "<@1001>",
	}
	contract.Boosters["1002"] = &Booster{
		UserID:  "1002",
		Nick:    "",
		Mention: "",
	}

	mismatch := buildLobbyMismatchSection(contributors, contract)
	if strings.Contains(mismatch, "- ``") {
		t.Errorf("mismatch contains empty code block: %q", mismatch)
	}
	if strings.Contains(mismatch, "Alice") {
		t.Errorf("expected Alice to be cleanly matched without mismatch, got %q", mismatch)
	}
	if !strings.Contains(mismatch, "<@1002>") {
		t.Errorf("expected booster 1002 to be displayed as <@1002>, got %q", mismatch)
	}
	if !strings.Contains(mismatch, "Bob") {
		t.Errorf("expected contributor Bob in mismatch, got %q", mismatch)
	}
}

func TestBuildLobbyMismatchSection_ExactDiscordMatch(t *testing.T) {
	// Tests exact case-insensitive match against Discord Username/Nick/GlobalName
	contributors := []*ei.ContractCoopStatusResponse_ContributionInfo{
		{
			UserId:   protoRef("dummy-user-1"),
			UserName: protoRef("kannabie"),
		},
	}

	contract := &Contract{
		ContractID: "test-contract",
		CoopID:     "test-coop",
		Boosters:   make(map[string]*Booster),
	}
	contract.Boosters["dummy-discord-1"] = &Booster{
		UserID:     "dummy-discord-1",
		Nick:       "kannabie",
		UserName:   "kannabie",
		GlobalName: "kannabie",
		Mention:    "<@dummy-discord-1>",
	}

	mismatch := buildLobbyMismatchSection(contributors, contract)
	if mismatch != "" {
		t.Errorf("expected no mismatch for identical name, got %q", mismatch)
	}
}

func TestBuildLobbyMismatchSection_SubstringMatch(t *testing.T) {
	// Tests unverified substring matching when name is not an exact match
	contributors := []*ei.ContractCoopStatusResponse_ContributionInfo{
		{
			UserId:   protoRef("dummy-user-1"),
			UserName: protoRef("FarmerDan_Alt"),
		},
	}

	contract := &Contract{
		ContractID: "test-contract",
		CoopID:     "test-coop",
		Boosters:   make(map[string]*Booster),
	}
	contract.Boosters["dummy-discord-1"] = &Booster{
		UserID:     "dummy-discord-1",
		Nick:       "FarmerDan",
		UserName:   "farmerdan",
		GlobalName: "FarmerDan",
		Mention:    "<@dummy-discord-1>",
	}

	mismatch := buildLobbyMismatchSection(contributors, contract)
	if !strings.Contains(mismatch, "Possible matches (unverified):") {
		t.Errorf("expected unverified possible match for substring, got %q", mismatch)
	}
	if !strings.Contains(mismatch, "FarmerDan_Alt") || !strings.Contains(mismatch, "<@dummy-discord-1>") {
		t.Errorf("expected substring match between FarmerDan_Alt and booster, got %q", mismatch)
	}
}

func protoRef(s string) *string {
	return &s
}

func TestLobbyButtons(t *testing.T) {
	row := lobbyButtons("contract-1", "coop-1")
	if len(row.Components) != 3 {
		t.Fatalf("expected 3 buttons, got %d", len(row.Components))
	}

	refreshBtn, ok := row.Components[0].(dc.Button)
	if !ok || refreshBtn.CustomID != "lobby#refresh#contract-1#coop-1" || refreshBtn.Label != "Refresh" {
		t.Errorf("unexpected refresh button: %+v", row.Components[0])
	}

	pingBtn, ok := row.Components[1].(dc.Button)
	if !ok || pingBtn.CustomID != "lobby#ping#contract-1#coop-1" || pingBtn.Label != "Ping" || pingBtn.Style != dc.ButtonPrimary {
		t.Errorf("unexpected ping button: %+v", row.Components[1])
	}

	closeBtn, ok := row.Components[2].(dc.Button)
	if !ok || closeBtn.CustomID != "lobby#close#contract-1#coop-1" || closeBtn.Label != "Close" || closeBtn.Style != dc.ButtonDanger {
		t.Errorf("unexpected close button: %+v", row.Components[2])
	}
}

func TestHandleLobbyButtons_Ping_NoContract(t *testing.T) {
	client := &dctest.FakeClient{}
	ev := dctest.ComponentButtonEvent("lobby#ping#no-contract#no-coop")

	// Should not panic, should acknowledge without sending SendMessage
	HandleLobbyButtons(client, ev)

	if client.Called("SendMessage") {
		t.Errorf("expected SendMessage not to be called when no contract exists")
	}
}

func TestHandleLobbyButtons_Ping_Unauthorized(t *testing.T) {
	contractKey := "auth-contract-auth-coop"
	contract := &Contract{
		ContractID: "auth-contract",
		CoopID:     "auth-coop",
		Location: []*LocationData{
			{ChannelID: "3"},
		},
		Order:    []string{"user99"},
		Boosters: make(map[string]*Booster),
	}
	contract.Boosters["user99"] = &Booster{
		UserID:  "user99",
		Nick:    "OnlyMember",
		Mention: "<@user99>",
	}

	ContractsMutex.Lock()
	Contracts[contractKey] = contract
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contractKey)
		ContractsMutex.Unlock()
	}()

	client := &dctest.FakeClient{}
	ev := dctest.ComponentButtonEvent("lobby#ping#auth-contract#auth-coop")

	HandleLobbyButtons(client, ev)

	if client.Called("SendMessage") {
		t.Errorf("expected unauthorized user not to trigger SendMessage")
	}
}

func TestHandleLobbyButtons_Ping_AllInCoop(t *testing.T) {
	contractKey := "all-in-contract-all-in-coop"
	contract := &Contract{
		ContractID: "all-in-contract",
		CoopID:     "all-in-coop",
		Location: []*LocationData{
			{ChannelID: "3"},
		},
		Order:    []string{"4"},
		Boosters: make(map[string]*Booster),
	}
	contract.Boosters["4"] = &Booster{
		UserID:  "4",
		Nick:    "ActiveMember",
		Mention: "<@4>",
	}

	ContractsMutex.Lock()
	Contracts[contractKey] = contract
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contractKey)
		ContractsMutex.Unlock()
	}()

	origGetCoopStatus := getLobbyCoopStatus
	getLobbyCoopStatus = func(cID, cpID, eID string) (*ei.ContractCoopStatusResponse, error) {
		return &ei.ContractCoopStatusResponse{
			ResponseStatus: ei.ContractCoopStatusResponse_NO_ERROR.Enum(),
			CoopIdentifier: protoRef("all-in-coop"),
			Contributors: []*ei.ContractCoopStatusResponse_ContributionInfo{
				{
					UserId:   protoRef("user-4"),
					UserName: protoRef("ActiveMember"),
				},
			},
		}, nil
	}
	defer func() {
		getLobbyCoopStatus = origGetCoopStatus
	}()

	client := &dctest.FakeClient{}
	ev := dctest.ComponentButtonEvent("lobby#ping#all-in-contract#all-in-coop")

	HandleLobbyButtons(client, ev)

	if client.Called("SendMessage") {
		t.Errorf("expected SendMessage not to be called when all players are already in coop")
	}
}

func TestHandleLobbyButtons_Ping_SuccessAndAlt(t *testing.T) {
	contractKey := "test-contract-test-coop"
	contract := &Contract{
		ContractID: "test-contract",
		CoopID:     "test-coop",
		Name:       "Super Ultra Contract",
		Location: []*LocationData{
			{ChannelID: "3"},
		},
		Order:    []string{"4", "1002", "1003"},
		Boosters: make(map[string]*Booster),
	}
	contract.Boosters["4"] = &Booster{
		UserID:  "4",
		Nick:    "CallerMain",
		Mention: "<@4>",
	}
	contract.Boosters["1002"] = &Booster{
		UserID:  "1002",
		Nick:    "MissingPlayer",
		Mention: "<@1002>",
	}
	contract.Boosters["1003"] = &Booster{
		UserID:        "1003",
		Nick:          "CallerAlt",
		Mention:       "<@1003>",
		AltController: "4",
		IsAlt:         true,
	}

	ContractsMutex.Lock()
	Contracts[contractKey] = contract
	ContractsMutex.Unlock()
	defer func() {
		ContractsMutex.Lock()
		delete(Contracts, contractKey)
		ContractsMutex.Unlock()
	}()

	origGetCoopStatus := getLobbyCoopStatus
	getLobbyCoopStatus = func(cID, cpID, eID string) (*ei.ContractCoopStatusResponse, error) {
		return &ei.ContractCoopStatusResponse{
			ResponseStatus: ei.ContractCoopStatusResponse_NO_ERROR.Enum(),
			CoopIdentifier: protoRef("test-coop"),
			Contributors: []*ei.ContractCoopStatusResponse_ContributionInfo{
				{
					UserId:   protoRef("user-4"),
					UserName: protoRef("CallerMain"),
				},
			},
		}, nil
	}
	defer func() {
		getLobbyCoopStatus = origGetCoopStatus
	}()

	client := &dctest.FakeClient{}
	ev := dctest.ComponentButtonEvent("lobby#ping#test-contract#test-coop")

	HandleLobbyButtons(client, ev)

	if !client.Called("SendMessage") {
		t.Fatalf("expected SendMessage to be called for missing players")
	}

	msgContent := client.Calls[0].Args[1]
	if !strings.Contains(msgContent, "<@1002>") {
		t.Errorf("expected missing player mention <@1002>, got: %s", msgContent)
	}
	if !strings.Contains(msgContent, "<@4> (CallerAlt)") {
		t.Errorf("expected alt controller mention <@4> (CallerAlt), got: %s", msgContent)
	}
	if !strings.Contains(msgContent, "Super Ultra Contract") {
		t.Errorf("expected contract name in content, got: %s", msgContent)
	}
	if !strings.Contains(msgContent, "test-coop") {
		t.Errorf("expected coop code in content, got: %s", msgContent)
	}
	if !strings.Contains(msgContent, "Ping requested by <@4>") {
		t.Errorf("expected requester mention in content, got: %s", msgContent)
	}

	// Immediate second click should hit cooldown
	HandleLobbyButtons(client, ev)
	if len(client.Calls) != 1 {
		t.Errorf("expected cooldown to prevent second SendMessage, calls: %d", len(client.Calls))
	}
}
