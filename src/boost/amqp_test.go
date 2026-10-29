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

func TestFindPreviousCXPScore(t *testing.T) {
	cID := "test-contract"
	coopPast := "past-coop"
	coopCurrent := "current-coop"
	cxpPast1 := 10500.5
	cxpPast2 := 12500.0
	cxpCurrent := 5000.0

	archive := []*ei.LocalContract{
		{
			ContractIdentifier: &cID,
			Evaluation: &ei.ContractEvaluation{
				ContractIdentifier: &cID,
				CoopIdentifier:     &coopPast,
				Cxp:                &cxpPast1,
			},
		},
		{
			ContractIdentifier: &cID,
			Evaluation: &ei.ContractEvaluation{
				ContractIdentifier: &cID,
				CoopIdentifier:     &coopPast,
				Cxp:                &cxpPast2, // higher score
			},
		},
		{
			ContractIdentifier: &cID,
			Evaluation: &ei.ContractEvaluation{
				ContractIdentifier: &cID,
				CoopIdentifier:     &coopCurrent,
				Cxp:                &cxpCurrent,
			},
		},
	}

	// Should skip current coop and pick the highest past score
	score, found := findPreviousCXPScore(archive, "TEST-CONTRACT", "current-coop")
	if !found {
		t.Fatalf("Expected to find previous CXP score")
	}
	if score != cxpPast2 {
		t.Errorf("Expected score %f, got %f", cxpPast2, score)
	}

	// When current coop doesn't match any, all matching are eligible
	score, found = findPreviousCXPScore(archive, "test-contract", "other-coop")
	if !found {
		t.Fatalf("Expected to find previous CXP score")
	}
	if score != cxpPast2 {
		t.Errorf("Expected score %f, got %f", cxpPast2, score)
	}

	// Non-matching contract
	score, found = findPreviousCXPScore(archive, "unmatched-contract", "")
	if found || score != 0 {
		t.Errorf("Expected not found for unmatched contract, got found=%v, score=%f", found, score)
	}

	// Empty archive
	score, found = findPreviousCXPScore(nil, "test-contract", "")
	if found || score != 0 {
		t.Errorf("Expected not found for nil archive, got found=%v, score=%f", found, score)
	}
}

func TestAMQPContractStartMessage_PreviousScoreJSON(t *testing.T) {
	startMsg := AMQPContractStartMessage{
		Event:           "contract_start",
		ContractID:      "test-contract",
		CoopID:          "test-coop",
		PreviousScore:   map[string]float64{"userA": 12345.5, "userB": 9876.0},
		MinutesPerToken: 60,
	}

	data, err := json.Marshal(startMsg)
	if err != nil {
		t.Fatalf("Failed to marshal AMQPContractStartMessage: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Failed to unmarshal JSON into map: %v", err)
	}

	prevScoreRaw, ok := raw["previous_score"]
	if !ok {
		t.Fatalf("Expected 'previous_score' key in JSON output: %s", string(data))
	}
	prevMap, ok := prevScoreRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected 'previous_score' to be map, got %T", prevScoreRaw)
	}
	if prevMap["userA"] != 12345.5 || prevMap["userB"] != 9876.0 {
		t.Errorf("Unexpected previous_score values: %+v", prevMap)
	}

	var parsed AMQPContractStartMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal back to struct: %v", err)
	}
	if len(parsed.PreviousScore) != 2 || parsed.PreviousScore["userA"] != 12345.5 {
		t.Errorf("Struct unmarshal mismatch: %+v", parsed.PreviousScore)
	}
}

func TestBuildAMQPBoostOrder_PopulatesPreviousScore(t *testing.T) {
	contract := &Contract{
		ContractID: "test-c",
		CoopID:     "test-coop",
		BoostOrder: ContractOrderSignup,
		Order:      []string{"userA", "userB"},
		PreviousScore: map[string]float64{
			"userB": 9999.0, // Pre-populated in contract
		},
		Boosters: map[string]*Booster{
			"userA": {
				UserID:        "userA",
				Nick:          "Player A",
				PreviousScore: 12345.0, // Set directly on booster
			},
			"userB": {
				UserID: "userB",
				Nick:   "Player B",
			},
		},
	}

	bo := BuildAMQPBoostOrder(contract)
	if bo == nil {
		t.Fatalf("Expected non-nil AMQPBoostOrder")
	}
	if len(bo.Boosters) != 2 {
		t.Fatalf("Expected 2 boosters, got %d", len(bo.Boosters))
	}

	if bo.Boosters[0].PreviousScore != 12345.0 {
		t.Errorf("Expected booster 0 previous score 12345.0, got %f", bo.Boosters[0].PreviousScore)
	}
	if bo.Boosters[1].PreviousScore != 9999.0 {
		t.Errorf("Expected booster 1 previous score 9999.0 from contract map, got %f", bo.Boosters[1].PreviousScore)
	}

	// Verify JSON serialization includes previous_score
	data, err := json.Marshal(bo.Boosters[0])
	if err != nil {
		t.Fatalf("Failed to marshal booster order entry: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Failed to unmarshal booster order entry json: %v", err)
	}
	if raw["previous_score"] != 12345.0 {
		t.Errorf("Expected previous_score in json, got %+v", raw["previous_score"])
	}
}

func TestGatherContractPreviousScores_SyncsExisting(t *testing.T) {
	contract := &Contract{
		ContractID: "c1",
		CoopID:     "coop1",
		Boosters: map[string]*Booster{
			"userA": {UserID: "userA", Nick: "Player A", PreviousScore: 5000.0},
			"userB": {UserID: "userB", Nick: "Player B"},
		},
		PreviousScore: map[string]float64{
			"userB": 7500.0,
		},
	}

	GatherContractPreviousScores(contract)

	// userA's score from booster should be synced to contract.PreviousScore
	if contract.PreviousScore["userA"] != 5000.0 {
		t.Errorf("Expected contract.PreviousScore[userA] = 5000.0, got %f", contract.PreviousScore["userA"])
	}
	// userB's score from contract.PreviousScore should be synced to booster
	if contract.Boosters["userB"].PreviousScore != 7500.0 {
		t.Errorf("Expected booster userB.PreviousScore = 7500.0, got %f", contract.Boosters["userB"].PreviousScore)
	}
}

func TestFetchBoosterPreviousScore_SyncsExisting(t *testing.T) {
	contract := &Contract{
		ContractID: "c1",
		CoopID:     "coop1",
		Boosters: map[string]*Booster{
			"userA": {UserID: "userA", Nick: "Player A"},
		},
		PreviousScore: map[string]float64{
			"userA": 8888.0,
		},
	}

	FetchBoosterPreviousScore(contract, "userA")

	if contract.Boosters["userA"].PreviousScore != 8888.0 {
		t.Errorf("Expected booster userA.PreviousScore to be synced to 8888.0, got %f", contract.Boosters["userA"].PreviousScore)
	}

	// Conversely, if booster has score but contract map doesn't
	contract2 := &Contract{
		ContractID: "c2",
		CoopID:     "coop2",
		Boosters: map[string]*Booster{
			"userB": {UserID: "userB", Nick: "Player B", PreviousScore: 4444.0},
		},
	}

	FetchBoosterPreviousScore(contract2, "userB")

	if contract2.PreviousScore["userB"] != 4444.0 {
		t.Errorf("Expected contract2.PreviousScore[userB] to be synced to 4444.0, got %f", contract2.PreviousScore["userB"])
	}
}

func TestGetGameDimensionName(t *testing.T) {
	tests := []struct {
		dim  ei.GameModifier_GameDimension
		want string
	}{
		{ei.GameModifier_EARNINGS, "earnings"},
		{ei.GameModifier_AWAY_EARNINGS, "away_earnings"},
		{ei.GameModifier_INTERNAL_HATCHERY_RATE, "internal_hatchery_rate"},
		{ei.GameModifier_EGG_LAYING_RATE, "egg_laying_rate"},
		{ei.GameModifier_SHIPPING_CAPACITY, "shipping_capacity"},
		{ei.GameModifier_HAB_CAPACITY, "hab_capacity"},
		{ei.GameModifier_VEHICLE_COST, "vehicle_cost"},
		{ei.GameModifier_HAB_COST, "hab_cost"},
		{ei.GameModifier_RESEARCH_COST, "research_cost"},
		{ei.GameModifier_INVALID, "unknown"},
		{ei.GameModifier_GameDimension(999), "unknown"},
	}

	for _, tt := range tests {
		got := GetGameDimensionName(tt.dim)
		if got != tt.want {
			t.Errorf("GetGameDimensionName(%v) = %q, want %q", tt.dim, got, tt.want)
		}
	}
}

func TestGetContractModifiers(t *testing.T) {
	// Empty contract ID
	if mods := GetContractModifiers(""); mods != nil {
		t.Errorf("Expected nil modifiers for empty contract ID, got %+v", mods)
	}

	// Non-existent contract ID
	if mods := GetContractModifiers("nonexistent-contract-xyz"); mods != nil {
		t.Errorf("Expected nil modifiers for nonexistent contract ID, got %+v", mods)
	}

	testContractID := "test-contract-modifiers"
	habCapDim := ei.GameModifier_HAB_CAPACITY
	habCapVal := 0.5
	habCapDesc := "50% Hab Space"

	ei.EggIncContractsMutex.Lock()
	if ei.EggIncContractsAll == nil {
		ei.EggIncContractsAll = make(map[string]ei.EggIncContract)
	}
	ei.EggIncContractsAll[testContractID] = ei.EggIncContract{
		ID: testContractID,
		RawModifiers: []*ei.GameModifier{
			{
				Dimension:   &habCapDim,
				Value:       &habCapVal,
				Description: &habCapDesc,
			},
		},
	}
	ei.EggIncContractsMutex.Unlock()

	defer func() {
		ei.EggIncContractsMutex.Lock()
		delete(ei.EggIncContractsAll, testContractID)
		ei.EggIncContractsMutex.Unlock()
	}()

	mods := GetContractModifiers(testContractID)
	if len(mods) != 1 {
		t.Fatalf("Expected 1 modifier, got %d", len(mods))
	}
	if mods[0].Dimension != int(ei.GameModifier_HAB_CAPACITY) {
		t.Errorf("Expected dimension %d, got %d", int(ei.GameModifier_HAB_CAPACITY), mods[0].Dimension)
	}
	if mods[0].DimensionName != "hab_capacity" {
		t.Errorf("Expected dimension_name 'hab_capacity', got %q", mods[0].DimensionName)
	}
	if mods[0].Value != 0.5 {
		t.Errorf("Expected value 0.5, got %f", mods[0].Value)
	}
	if mods[0].Description != "50% Hab Space" {
		t.Errorf("Expected description '50%% Hab Space', got %q", mods[0].Description)
	}
}

func TestAMQPContractModifiersJSON(t *testing.T) {
	// Test with modifiers populated
	startMsg := AMQPContractStartMessage{
		Event:      "contract_start",
		ContractID: "test-c1",
		CoopID:     "coop-1",
		Modifiers: []AMQPContractModifier{
			{
				Dimension:     6,
				DimensionName: "hab_capacity",
				Value:         0.5,
				Description:   "Half Hab Capacity",
			},
		},
	}

	data, err := json.Marshal(startMsg)
	if err != nil {
		t.Fatalf("Failed to marshal AMQPContractStartMessage: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Failed to unmarshal startMsg json: %v", err)
	}

	rawMods, ok := raw["modifiers"].([]interface{})
	if !ok || len(rawMods) != 1 {
		t.Fatalf("Expected modifiers array with 1 item, got: %+v", raw["modifiers"])
	}
	modMap := rawMods[0].(map[string]interface{})
	if modMap["dimension"] != float64(6) {
		t.Errorf("Expected dimension 6, got %+v", modMap["dimension"])
	}
	if modMap["dimension_name"] != "hab_capacity" {
		t.Errorf("Expected dimension_name 'hab_capacity', got %+v", modMap["dimension_name"])
	}
	if modMap["value"] != 0.5 {
		t.Errorf("Expected value 0.5, got %+v", modMap["value"])
	}
	if modMap["description"] != "Half Hab Capacity" {
		t.Errorf("Expected description 'Half Hab Capacity', got %+v", modMap["description"])
	}

	// Test update message with modifiers
	updateMsg := AMQPContractUpdateMessage{
		Event:      "contract_update",
		ContractID: "test-c1",
		CoopID:     "coop-1",
		Modifiers: []AMQPContractModifier{
			{
				Dimension:     4,
				DimensionName: "egg_laying_rate",
				Value:         2.0,
			},
		},
	}

	dataUpdate, err := json.Marshal(updateMsg)
	if err != nil {
		t.Fatalf("Failed to marshal AMQPContractUpdateMessage: %v", err)
	}

	var rawUpdate map[string]interface{}
	if err := json.Unmarshal(dataUpdate, &rawUpdate); err != nil {
		t.Fatalf("Failed to unmarshal updateMsg json: %v", err)
	}
	rawUpdateMods, ok := rawUpdate["modifiers"].([]interface{})
	if !ok || len(rawUpdateMods) != 1 {
		t.Fatalf("Expected modifiers array with 1 item in update message, got: %+v", rawUpdate["modifiers"])
	}

	// Test omit empty when modifiers is nil or empty
	startMsgEmpty := AMQPContractStartMessage{
		Event:      "contract_start",
		ContractID: "test-c2",
		CoopID:     "coop-2",
	}
	dataEmpty, err := json.Marshal(startMsgEmpty)
	if err != nil {
		t.Fatalf("Failed to marshal AMQPContractStartMessage without modifiers: %v", err)
	}
	var rawEmpty map[string]interface{}
	if err := json.Unmarshal(dataEmpty, &rawEmpty); err != nil {
		t.Fatalf("Failed to unmarshal startMsgEmpty json: %v", err)
	}
	if _, exists := rawEmpty["modifiers"]; exists {
		t.Errorf("Expected 'modifiers' to be omitted when empty, but was present: %+v", rawEmpty["modifiers"])
	}
}

func TestGetContractTimeLimitSeconds(t *testing.T) {
	if sec := GetContractTimeLimitSeconds(nil); sec != 0 {
		t.Errorf("Expected 0 for nil contract, got %d", sec)
	}

	cDirect := &Contract{
		ContractID:      "direct-c",
		LengthInSeconds: 86400,
	}
	if sec := GetContractTimeLimitSeconds(cDirect); sec != 86400 {
		t.Errorf("Expected 86400 for contract with direct LengthInSeconds, got %d", sec)
	}

	testCID := "test-timelimit-contract"
	ei.EggIncContractsMutex.Lock()
	if ei.EggIncContractsAll == nil {
		ei.EggIncContractsAll = make(map[string]ei.EggIncContract)
	}
	ei.EggIncContractsAll[testCID] = ei.EggIncContract{
		ID:              testCID,
		LengthInSeconds: 172800,
	}
	ei.EggIncContractsMutex.Unlock()

	defer func() {
		ei.EggIncContractsMutex.Lock()
		delete(ei.EggIncContractsAll, testCID)
		ei.EggIncContractsMutex.Unlock()
	}()

	cFallback := &Contract{
		ContractID: testCID,
	}
	if sec := GetContractTimeLimitSeconds(cFallback); sec != 172800 {
		t.Errorf("Expected 172800 from EggIncContractsAll fallback, got %d", sec)
	}
}

func TestAMQPTimeLimitSecondsJSON(t *testing.T) {
	startMsg := AMQPContractStartMessage{
		Event:            "contract_start",
		ContractID:       "c1",
		CoopID:           "coop1",
		TimeLimitSeconds: 259200,
	}

	data, err := json.Marshal(startMsg)
	if err != nil {
		t.Fatalf("Failed to marshal AMQPContractStartMessage: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Failed to unmarshal startMsg JSON: %v", err)
	}

	if raw["time_limit_seconds"] != float64(259200) {
		t.Errorf("Expected time_limit_seconds to be 259200, got %+v", raw["time_limit_seconds"])
	}

	updateMsg := AMQPContractUpdateMessage{
		Event:            "contract_update",
		ContractID:       "c1",
		CoopID:           "coop1",
		TimeLimitSeconds: 86400,
	}

	dataUpdate, err := json.Marshal(updateMsg)
	if err != nil {
		t.Fatalf("Failed to marshal AMQPContractUpdateMessage: %v", err)
	}

	var rawUpdate map[string]interface{}
	if err := json.Unmarshal(dataUpdate, &rawUpdate); err != nil {
		t.Fatalf("Failed to unmarshal updateMsg JSON: %v", err)
	}

	if rawUpdate["time_limit_seconds"] != float64(86400) {
		t.Errorf("Expected time_limit_seconds in update message to be 86400, got %+v", rawUpdate["time_limit_seconds"])
	}

	// Test omitempty when 0
	emptyMsg := AMQPContractStartMessage{
		Event:      "contract_start",
		ContractID: "c2",
		CoopID:     "coop2",
	}

	dataEmpty, err := json.Marshal(emptyMsg)
	if err != nil {
		t.Fatalf("Failed to marshal emptyMsg: %v", err)
	}

	var rawEmpty map[string]interface{}
	if err := json.Unmarshal(dataEmpty, &rawEmpty); err != nil {
		t.Fatalf("Failed to unmarshal emptyMsg JSON: %v", err)
	}

	if _, exists := rawEmpty["time_limit_seconds"]; exists {
		t.Errorf("Expected time_limit_seconds to be omitted when 0, but found: %+v", rawEmpty["time_limit_seconds"])
	}
}
