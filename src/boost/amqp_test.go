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

func TestAMQPContractStartWithBoostOrder(t *testing.T) {
	contract := &Contract{
		ContractID:   "test-contract",
		CoopID:       "test-coop",
		ContractHash: "test-hash",
		BoostOrder:   ContractOrderCustom,
		CustomOrderLines: []string{
			"<ROLE",
			"<IHR[6%]",
			"<DEFL",
			"<DELIV",
		},
		CustomOrderName: "My Custom Order",
		Order:           []string{"userA", "userB"},
		Boosters: map[string]*Booster{
			"userA": {
				UserID:       "userA",
				Nick:         "Farmer Alpha",
				IHRRate:      8.5e9,
				TokensWanted: 6,
				TECount:      100,
				ArtifactSet: ArtifactSet{
					Artifacts: []ei.Artifact{
						{Type: "Deflector", Quality: "T4L"},
					},
					LayRate: 25.0,
				},
			},
			"userB": {
				UserID:       "userB",
				Nick:         "Farmer Beta",
				IHRRate:      8.2e9,
				TokensWanted: 4,
				TECount:      90,
				ArtifactSet: ArtifactSet{
					Artifacts: []ei.Artifact{
						{Type: "Deflector", Quality: "T4E"},
					},
					LayRate: 20.0,
				},
			},
		},
	}

	boostOrder := BuildAMQPBoostOrder(contract)
	if boostOrder == nil {
		t.Fatalf("Expected non-nil AMQPBoostOrder")
	}

	if boostOrder.OrderName != "My Custom Order" {
		t.Errorf("Expected OrderName 'My Custom Order', got %q", boostOrder.OrderName)
	}

	if len(boostOrder.Hierarchy) != 4 {
		t.Errorf("Expected 4 hierarchy items, got %d", len(boostOrder.Hierarchy))
	} else {
		if boostOrder.Hierarchy[1].Rule != "<IHR[6%]" {
			t.Errorf("Expected rule %q, got %q", "<IHR[6%]", boostOrder.Hierarchy[1].Rule)
		}
		if boostOrder.Hierarchy[1].Explanation == "" {
			t.Errorf("Expected fuzzy explanation for hierarchy item 2, got empty")
		}
	}

	if len(boostOrder.Columns) < 2 {
		t.Fatalf("Expected at least 2 columns, got %d", len(boostOrder.Columns))
	}
	if boostOrder.Columns[0] != "#" || boostOrder.Columns[1] != "Player" {
		t.Errorf("Expected first columns '#' and 'Player', got %v", boostOrder.Columns[:2])
	}

	if len(boostOrder.Boosters) != 2 {
		t.Fatalf("Expected 2 boosters in boost order, got %d", len(boostOrder.Boosters))
	}

	for i, b := range boostOrder.Boosters {
		if b.Position != i+1 {
			t.Errorf("Expected position %d, got %d", i+1, b.Position)
		}
		if b.UserID == "" || b.Nick == "" {
			t.Errorf("Expected non-empty UserID and Nick, got %+v", b)
		}
		if len(b.Row) != len(boostOrder.Columns) {
			t.Errorf("Expected row length %d to match columns length %d, got %d", len(boostOrder.Columns), len(boostOrder.Columns), len(b.Row))
		}
		for _, col := range boostOrder.Columns {
			if _, ok := b.Values[col]; !ok {
				t.Errorf("Expected value for column %q in booster values", col)
			}
		}
	}

	startMsg := AMQPContractStartMessage{
		Event:           "contract_start",
		ContractID:      contract.ContractID,
		CoopID:          contract.CoopID,
		ChannelID:       "channel-1",
		StartTime:       time.Now(),
		CoopSize:        10,
		DeliveryTarget:  1e16,
		GenerousGifts:   "Ultra GG",
		GGMultiplier:    3.0,
		MinutesPerToken: 60,
		BoostOrder:      boostOrder,
	}

	data, err := json.Marshal(startMsg)
	if err != nil {
		t.Fatalf("Failed to marshal AMQPContractStartMessage with BoostOrder: %v", err)
	}

	var parsedStart AMQPContractStartMessage
	if err := json.Unmarshal(data, &parsedStart); err != nil {
		t.Fatalf("Failed to unmarshal AMQPContractStartMessage with BoostOrder: %v", err)
	}

	if parsedStart.BoostOrder == nil {
		t.Fatalf("Parsed BoostOrder is nil")
	}
	if parsedStart.BoostOrder.OrderName != "My Custom Order" {
		t.Errorf("Parsed OrderName mismatch: got %q", parsedStart.BoostOrder.OrderName)
	}
	if len(parsedStart.BoostOrder.Boosters) != 2 {
		t.Errorf("Parsed Boosters count mismatch: got %d", len(parsedStart.BoostOrder.Boosters))
	}
}

func TestBuildAMQPBoostOrder_PresetsAndNil(t *testing.T) {
	if BuildAMQPBoostOrder(nil) != nil {
		t.Errorf("Expected nil for nil contract")
	}

	ihrContract := &Contract{
		ContractID: "c1",
		CoopID:     "coop1",
		BoostOrder: ContractOrderIHR,
		Order:      []string{"userA"},
		Boosters: map[string]*Booster{
			"userA": {
				UserID:  "userA",
				Nick:    "PlayerA",
				IHRRate: 8.0e9,
			},
		},
	}

	bo := BuildAMQPBoostOrder(ihrContract)
	if bo == nil {
		t.Fatalf("Expected non-nil AMQPBoostOrder for IHR contract")
	}
	if bo.OrderName != "Boosting IHR" {
		t.Errorf("Expected OrderName 'Boosting IHR', got %q", bo.OrderName)
	}
	if len(bo.Boosters) != 1 {
		t.Fatalf("Expected 1 booster, got %d", len(bo.Boosters))
	}
	if bo.Boosters[0].Nick != "PlayerA" {
		t.Errorf("Expected Nick 'PlayerA', got %q", bo.Boosters[0].Nick)
	}
}

func TestBuildAMQPBoostOrder_StartedContractPreservesOrder(t *testing.T) {
	runningContract := &Contract{
		ContractID: "c-running",
		CoopID:     "coop-running",
		State:      ContractStateBanker,
		BoostOrder: ContractOrderSignup,
		Order:      []string{"userB", "userA"}, // already ordered/swapped
		Boosters: map[string]*Booster{
			"userA": {UserID: "userA", Nick: "PlayerA"},
			"userB": {UserID: "userB", Nick: "PlayerB"},
		},
	}

	bo := BuildAMQPBoostOrder(runningContract)
	if bo == nil {
		t.Fatalf("Expected non-nil AMQPBoostOrder")
	}
	if len(bo.Boosters) != 2 {
		t.Fatalf("Expected 2 boosters, got %d", len(bo.Boosters))
	}
	if bo.Boosters[0].UserID != "userB" || bo.Boosters[1].UserID != "userA" {
		t.Errorf("Expected running contract to preserve Order [userB, userA], got [%s, %s]",
			bo.Boosters[0].UserID, bo.Boosters[1].UserID)
	}
}
