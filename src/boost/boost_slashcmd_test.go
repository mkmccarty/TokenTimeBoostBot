package boost

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

func TestParseFarmerInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []ParsedFarmer
	}{
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
		{
			name:  "single guest",
			input: "Bob",
			want:  []ParsedFarmer{{Guest: "Bob"}},
		},
		{
			name:  "single mention",
			input: "<@123456789>",
			want:  []ParsedFarmer{{Mention: "<@123456789>"}},
		},
		{
			name:  "mixed multiple",
			input: "Alice, <@987654321>, Charlie",
			want: []ParsedFarmer{
				{Guest: "Alice"},
				{Mention: "<@987654321>"},
				{Guest: "Charlie"},
			},
		},
		{
			name:  "with extra spaces and empty elements",
			input: "  Dave  ,, <@111222333> , ",
			want: []ParsedFarmer{
				{Guest: "Dave"},
				{Mention: "<@111222333>"},
			},
		},
		{
			name:  "multiple mentions no spaces",
			input: "<@987654321><@123456789> <@111222333>,<@444555666>",
			want: []ParsedFarmer{
				{Mention: "<@987654321>"},
				{Mention: "<@123456789>"},
				{Mention: "<@111222333>"},
				{Mention: "<@444555666>"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseFarmerInput(tt.input); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseFarmerInput() = %v, want %v", got, tt.want)
			}
		})
	}
}

func newTokenEditCommandEvent(t *testing.T, guildID, channelID, userID string, action, tokenID int, newReceiver string, hasQuantity bool, newQuantity int) *dc.CommandEvent {
	t.Helper()
	opts := []string{
		fmt.Sprintf(`{"name": "action", "type": 4, "value": %d}`, action),
		fmt.Sprintf(`{"name": "list", "type": 3, "value": "test-coop"}`),
		fmt.Sprintf(`{"name": "id", "type": 4, "value": %d}`, tokenID),
	}
	if newReceiver != "" {
		opts = append(opts, fmt.Sprintf(`{"name": "new-receiver", "type": 3, "value": %q}`, newReceiver))
	}
	if hasQuantity {
		opts = append(opts, fmt.Sprintf(`{"name": "new-quantity", "type": 4, "value": %d}`, newQuantity))
	}

	payload := fmt.Sprintf(`{
		"id": "100",
		"application_id": "200",
		"type": 2,
		"token": "tok",
		"version": 1,
		"guild_id": %q,
		"channel": {"id": %q, "type": 0},
		"member": {"user": {"id": %q, "username": "caller", "discriminator": "0"}},
		"user": {"id": %q, "username": "caller", "discriminator": "0"},
		"data": {"id": "1", "name": "token-edit", "type": 1, "options": [%s]}
	}`, guildID, channelID, userID, userID, strings.Join(opts, ", "))

	e, err := dc.NewCommandEventFromPayload([]byte(payload))
	if err != nil {
		t.Fatalf("NewCommandEventFromPayload failed: %v", err)
	}
	return e
}

func TestHandleTokenEditCommand_Guards(t *testing.T) {
	guildID := "9001"
	channelID := "8002"
	client := dctest.New().WithGuild(guildID, "Test Guild")
	client.WithChannel(channelID, guildID, "contract-channel")

	serial := "00000000-0000-0000-0000-00000000002a"
	tokenID := int(tokenSerialToInt(serial))

	contract := &Contract{
		ContractHash:      "token-edit-test-hash",
		ContractID:        "test-contract",
		CoopID:            "test-coop",
		CreatorID:         []string{"1001"},
		Location:          []*LocationData{{GuildID: guildID, ChannelID: channelID}},
		Order:             []string{"1001", "1002"},
		StartTime:         time.Now().Add(-10 * time.Minute),
		EstimatedDuration: time.Hour,
		Boosters: map[string]*Booster{
			"1001": {UserID: "1001", Nick: "Alice"},
			"1002": {UserID: "1002", Nick: "Bob"},
		},
		TokenLog: []ei.TokenUnitLog{
			{
				Serial:     serial,
				FromUserID: "1001",
				FromNick:   "Alice",
				ToUserID:   "1002",
				ToNick:     "Bob",
				Quantity:   1,
				Time:       time.Now(),
			},
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

	t.Run("move without new-receiver does not panic", func(t *testing.T) {
		e := newTokenEditCommandEvent(t, guildID, channelID, "1001", 0, tokenID, "", false, 0)
		got := HandleTokenEditCommand(client, e)
		if got != "New receiver is required to move a token." {
			t.Errorf("got %q, want %q", got, "New receiver is required to move a token.")
		}
		if contract.TokenLog[0].ToUserID != "1002" {
			t.Errorf("expected ToUserID to remain 1002, got %q", contract.TokenLog[0].ToUserID)
		}
	})

	t.Run("move with unknown new-receiver does not panic", func(t *testing.T) {
		e := newTokenEditCommandEvent(t, guildID, channelID, "1001", 0, tokenID, "9999", false, 0)
		got := HandleTokenEditCommand(client, e)
		if got != "New receiver not found in contract." {
			t.Errorf("got %q, want %q", got, "New receiver not found in contract.")
		}
	})

	t.Run("move with valid new-receiver succeeds", func(t *testing.T) {
		e := newTokenEditCommandEvent(t, guildID, channelID, "1001", 0, tokenID, "1001", false, 0)
		got := HandleTokenEditCommand(client, e)
		if got != "Token moved to Alice" {
			t.Errorf("got %q, want %q", got, "Token moved to Alice")
		}
		if contract.TokenLog[0].ToUserID != "1001" || contract.TokenLog[0].ToNick != "Alice" {
			t.Errorf("expected token moved to Alice (1001), got %q (%q)", contract.TokenLog[0].ToNick, contract.TokenLog[0].ToUserID)
		}
	})

	t.Run("modify count without quantity returns error", func(t *testing.T) {
		e := newTokenEditCommandEvent(t, guildID, channelID, "1001", 2, tokenID, "", false, 0)
		got := HandleTokenEditCommand(client, e)
		if got != "New quantity must be at least 1." {
			t.Errorf("got %q, want %q", got, "New quantity must be at least 1.")
		}
		if contract.TokenLog[0].Quantity != 1 {
			t.Errorf("expected Quantity to remain 1, got %d", contract.TokenLog[0].Quantity)
		}
	})
}
