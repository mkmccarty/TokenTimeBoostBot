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

	startMsg := AMQPContractStartMessage{
		Event:           "contract_start",
		ContractID:      "contract-1",
		CoopID:          "coop-1",
		StartTime:       time.Now(),
		CoopSize:        5,
		DeliveryTarget:  1e15,
		MinutesPerToken: 60,
	}

	data, err = json.Marshal(startMsg)
	if err != nil {
		t.Fatalf("Failed to marshal AMQPContractStartMessage: %v", err)
	}

	var parsedStart AMQPContractStartMessage
	if err := json.Unmarshal(data, &parsedStart); err != nil {
		t.Fatalf("Failed to unmarshal AMQPContractStartMessage: %v", err)
	}

	if parsedStart.Event != "contract_start" || parsedStart.CoopSize != 5 || parsedStart.DeliveryTarget != 1e15 || parsedStart.MinutesPerToken != 60 {
		t.Errorf("Unexpected contract start message fields. Got: %+v", parsedStart)
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

func TestAMQPContractUpdateAndBoosterChange(t *testing.T) {
	updateMsg := AMQPContractUpdateMessage{
		Event:           "contract_update",
		ContractID:      "contract-1",
		CoopID:          "coop-1",
		ChannelID:       "channel-1",
		StartTime:       time.Now(),
		CoopSize:        10,
		DeliveryTarget:  1e15,
		GenerousGifts:   "None",
		MinutesPerToken: 60,
	}

	data, err := json.Marshal(updateMsg)
	if err != nil {
		t.Fatalf("Failed to marshal AMQPContractUpdateMessage: %v", err)
	}

	var parsedUpdate AMQPContractUpdateMessage
	if err := json.Unmarshal(data, &parsedUpdate); err != nil {
		t.Fatalf("Failed to unmarshal AMQPContractUpdateMessage: %v", err)
	}

	if parsedUpdate.Event != "contract_update" || parsedUpdate.CoopSize != 10 {
		t.Errorf("Unexpected contract update message fields. Got: %+v", parsedUpdate)
	}

	joinMsg := AMQPBoosterChangeMessage{
		Event:      "booster_join",
		ContractID: "contract-1",
		CoopID:     "coop-1",
		UserID:     "user-1",
		Nick:       "FarmerBob",
		Time:       time.Now(),
	}

	data, err = json.Marshal(joinMsg)
	if err != nil {
		t.Fatalf("Failed to marshal AMQPBoosterChangeMessage: %v", err)
	}

	var parsedJoin AMQPBoosterChangeMessage
	if err := json.Unmarshal(data, &parsedJoin); err != nil {
		t.Fatalf("Failed to unmarshal AMQPBoosterChangeMessage: %v", err)
	}

	if parsedJoin.Event != "booster_join" || parsedJoin.Nick != "FarmerBob" {
		t.Errorf("Unexpected booster change message fields. Got: %+v", parsedJoin)
	}
}

func TestNonTokenReactionTracking(t *testing.T) {
	contract := &Contract{
		ContractID: "test-contract",
		CoopID:     "test-coop",
		Boosters: map[string]*Booster{
			"user1": {
				UserID: "user1",
				Nick:   "FarmerUser1",
			},
		},
		TokenLog: []ei.TokenUnitLog{
			{
				Time:       time.Now().Add(-5 * time.Minute),
				Quantity:   2,
				FromUserID: "user1",
				FromNick:   "FarmerUser1",
				ToUserID:   "user2",
				ToNick:     "FarmerUser2",
			},
		},
	}

	booster := contract.Boosters["user1"]
	booster.NonTokenReactionTimes = append(booster.NonTokenReactionTimes, time.Now())

	if len(booster.NonTokenReactionTimes) != 1 {
		t.Errorf("Expected 1 non-token reaction time, got %d", len(booster.NonTokenReactionTimes))
	}
}


