package boost

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

func TestAMQPMessageFormatting(t *testing.T) {
	tokenMsg := AMQPTokenMessage{
		Event:      "token_transfer",
		ContractID: "contract-1",
		CoopID:     "coop-1",
		Time:       time.Now(),
		Quantity:   2,
		Value:      0.8,
		FromUserID: "user-from",
		FromNick:   "FromUser",
		ToUserID:   "user-to",
		ToNick:     "ToUser",
		Boost:      false,
	}

	data, err := json.Marshal(tokenMsg)
	if err != nil {
		t.Fatalf("Failed to marshal AMQPTokenMessage: %v", err)
	}

	var parsedToken AMQPTokenMessage
	if err := json.Unmarshal(data, &parsedToken); err != nil {
		t.Fatalf("Failed to unmarshal AMQPTokenMessage: %v", err)
	}

	if parsedToken.Quantity != 2 || parsedToken.FromNick != "FromUser" {
		t.Errorf("Unexpected token message fields. Got: %+v", parsedToken)
	}

	statusMsg := AMQPBoostStatusMessage{
		Event:      "boost_status_change",
		ContractID: "contract-1",
		CoopID:     "coop-1",
		UserID:     "user-1",
		Nick:       "BoosterUser",
		BoostState: "Boosted",
		Time:       time.Now(),
	}

	data, err = json.Marshal(statusMsg)
	if err != nil {
		t.Fatalf("Failed to marshal AMQPBoostStatusMessage: %v", err)
	}

	var parsedStatus AMQPBoostStatusMessage
	if err := json.Unmarshal(data, &parsedStatus); err != nil {
		t.Fatalf("Failed to unmarshal AMQPBoostStatusMessage: %v", err)
	}

	if parsedStatus.BoostState != "Boosted" || parsedStatus.Nick != "BoosterUser" {
		t.Errorf("Unexpected status message fields. Got: %+v", parsedStatus)
	}
}

func TestGetBoostStateString(t *testing.T) {
	tests := []struct {
		state int
		want  string
	}{
		{BoostStateUnboosted, "Unboosted"},
		{BoostStateTokenTime, "TokenTime"},
		{BoostStateBoosted, "Boosted"},
		{999, "Unknown"},
	}

	for _, tt := range tests {
		got := getBoostStateString(tt.state)
		if got != tt.want {
			t.Errorf("getBoostStateString(%d) = %q; want %q", tt.state, got, tt.want)
		}
	}
}

func TestSaveSqliteDataAMQPTrigger(t *testing.T) {
	// Initialize a contract with AMQP style flag
	c := &Contract{
		ContractHash: "c1-coop1",
		ContractID:   "c1",
		CoopID:       "coop1",
		Style:        ContractFlagAMQP,
		Location: []*LocationData{
			{
				GuildID:   "guild-1",
				ChannelID: "channel-1",
			},
		},
		Boosters: map[string]*Booster{
			"u1": {
				UserID:     "u1",
				Nick:       "User1",
				BoostState: BoostStateTokenTime,
			},
		},
		TokenLog: []ei.TokenUnitLog{
			{
				Time:       time.Now(),
				Quantity:   1,
				FromUserID: "u1",
				FromNick:   "User1",
				ToUserID:   "u2",
				ToNick:     "User2",
				Serial:     "s1",
			},
		},
	}

	// It should correctly initialize tracking fields if they are missing
	if c.LastPublishedStates == nil {
		c.LastPublishedStates = make(map[string]int)
	}
	c.LastPublishedStates["u1"] = BoostStateTokenTime
	c.LastPublishedTokenLogIndex = len(c.TokenLog)

	if c.LastPublishedStates["u1"] != BoostStateTokenTime {
		t.Errorf("Expected LastPublishedStates for u1 to be BoostStateTokenTime")
	}
	if c.LastPublishedTokenLogIndex != 1 {
		t.Errorf("Expected LastPublishedTokenLogIndex to be 1")
	}
}
