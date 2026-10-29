package boost

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"
	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	amqpClients   = make(map[string]*AMQPClient)
	amqpClientsMu sync.Mutex
)

// AMQPClient manages the AMQP connection and channel for a single guild.
type AMQPClient struct {
	guildID string
	url     string
	conn    *amqp.Connection
	channel *amqp.Channel
	mu      sync.Mutex
}

// getAMQPClient retrieves or creates an AMQP client for a guild.
func getAMQPClient(guildID string, url string) *AMQPClient {
	amqpClientsMu.Lock()
	defer amqpClientsMu.Unlock()

	client, exists := amqpClients[guildID]
	if !exists || client.url != url {
		if exists {
			client.Close()
		}
		client = &AMQPClient{
			guildID: guildID,
			url:     url,
		}
		amqpClients[guildID] = client
	}
	return client
}

// connect establishes the connection and channel, declaring the "boost-bot" queue.
func (c *AMQPClient) connect() error {
	conn, err := amqp.Dial(c.url)
	if err != nil {
		return err
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return err
	}

	_, err = ch.QueueDeclare(
		"boost-bot", // name
		true,        // durable
		false,       // delete when unused
		false,       // exclusive
		false,       // no-wait
		nil,         // arguments
	)
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}

	c.conn = conn
	c.channel = ch
	return nil
}

// Close closes the AMQP connection and channel.
func (c *AMQPClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.channel != nil {
		_ = c.channel.Close()
		c.channel = nil
	}
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}

// publish sends a message to the "boost-bot" queue.
func (c *AMQPClient) publish(body []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil || c.conn.IsClosed() || c.channel == nil {
		if err := c.connect(); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return c.channel.PublishWithContext(
		ctx,
		"",          // exchange
		"boost-bot", // routing key
		false,       // mandatory
		false,       // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
			Expiration:  "86400000", // 24 hours in milliseconds
		},
	)
}

// AMQPTokenMessage represents the structure of the token unit log sent over AMQP.
type AMQPTokenMessage struct {
	Event      string    `json:"event"`
	ContractID string    `json:"contract_id"`
	CoopID     string    `json:"coop_id"`
	Time       time.Time `json:"time"`
	Quantity   int       `json:"quantity"`
	Value      float64   `json:"value"`
	FromUserID string    `json:"from_user_id"`
	FromNick   string    `json:"from_nick"`
	ToUserID   string    `json:"to_user_id"`
	ToNick     string    `json:"to_nick"`
	Boost      bool      `json:"boost"`
}

// AMQPBoostStatusMessage represents a player's boost status change.
type AMQPBoostStatusMessage struct {
	Event      string    `json:"event"`
	ContractID string    `json:"contract_id"`
	CoopID     string    `json:"coop_id"`
	UserID     string    `json:"user_id"`
	Nick       string    `json:"nick"`
	BoostState string    `json:"boost_state"` // "Unboosted", "TokenTime", "Boosted"
	Time       time.Time `json:"time"`
}

// getBoostStateString maps boost state integers to human readable strings.
func getBoostStateString(state int) string {
	switch state {
	case BoostStateUnboosted:
		return "Unboosted"
	case BoostStateTokenTime:
		return "TokenTime"
	case BoostStateBoosted:
		return "Boosted"
	default:
		return "Unknown"
	}
}

// PublishAMQPTokenLog publishes a token log to the guild's AMQP queue.
func PublishAMQPTokenLog(guildID string, url string, contractID string, coopID string, logEntry AMQPTokenMessage) {
	body, err := json.Marshal(logEntry)
	if err != nil {
		log.Printf("AMQP: error marshaling token log: %v", err)
		return
	}

	client := getAMQPClient(guildID, url)
	go func() {
		if err := client.publish(body); err != nil {
			log.Printf("AMQP: error publishing token log: %v", err)
		}
	}()
}

// PublishAMQPBoostStatus publishes a boost status update to the guild's AMQP queue.
func PublishAMQPBoostStatus(guildID string, url string, contractID string, coopID string, status AMQPBoostStatusMessage) {
	body, err := json.Marshal(status)
	if err != nil {
		log.Printf("AMQP: error marshaling boost status: %v", err)
		return
	}

	client := getAMQPClient(guildID, url)
	go func() {
		if err := client.publish(body); err != nil {
			log.Printf("AMQP: error publishing boost status: %v", err)
		}
	}()
}

// AMQPHierarchyCriterion represents a single line in the boost order hierarchy.
type AMQPHierarchyCriterion struct {
	Index       int    `json:"index"`
	Rule        string `json:"rule"`
	Explanation string `json:"explanation,omitempty"`
}

// AMQPBoosterOrderEntry represents a player's entry in the boost order table.
type AMQPBoosterOrderEntry struct {
	Position      int               `json:"position"`
	UserID        string            `json:"user_id"`
	Nick          string            `json:"nick"`
	PreviousScore float64           `json:"previous_score,omitempty"`
	Values        map[string]string `json:"values"`
	Row           []string          `json:"row"`
}

// AMQPBoostOrder represents the complete boost order structure matching the ⚙️ response.
type AMQPBoostOrder struct {
	OrderName string                   `json:"order_name"`
	Hierarchy []AMQPHierarchyCriterion `json:"hierarchy,omitempty"`
	Columns   []string                 `json:"columns"`
	Boosters  []AMQPBoosterOrderEntry  `json:"boosters"`
}

// AMQPContractModifier represents a game modifier / dimension buff on a contract.
type AMQPContractModifier struct {
	Dimension     int     `json:"dimension"`
	DimensionName string  `json:"dimension_name"`
	Value         float64 `json:"value"`
	Description   string  `json:"description,omitempty"`
}

// AMQPContractStartMessage represents details of a contract when started.
type AMQPContractStartMessage struct {
	Event            string                 `json:"event"`
	ContractID       string                 `json:"contract_id"`
	CoopID           string                 `json:"coop_id"`
	ChannelID        string                 `json:"channel_id"`
	StartTime        time.Time              `json:"start_time"`
	CoopSize         int                    `json:"coop_size"`
	DeliveryTarget   float64                `json:"delivery_target"`
	GenerousGifts    string                 `json:"generous_gifts"`
	GGMultiplier     float64                `json:"gg_multiplier,omitempty"`
	MinutesPerToken  int                    `json:"minutes_per_token"`
	TimeLimitSeconds int                    `json:"time_limit_seconds,omitempty"`
	Modifiers        []AMQPContractModifier `json:"modifiers,omitempty"`
	PreviousScore    map[string]float64     `json:"previous_score,omitempty"`
	BoostOrder       *AMQPBoostOrder        `json:"boost_order,omitempty"`
}

// PublishAMQPContractStart publishes a contract start event to the guild's AMQP queue.
func PublishAMQPContractStart(guildID string, url string, status AMQPContractStartMessage) {
	body, err := json.Marshal(status)
	if err != nil {
		log.Printf("AMQP: error marshaling contract start: %v", err)
		return
	}

	client := getAMQPClient(guildID, url)
	go func() {
		if err := client.publish(body); err != nil {
			log.Printf("AMQP: error publishing contract start: %v", err)
		}
	}()
}

// AMQPNonTokenMessage represents a non-token delivery report.
type AMQPNonTokenMessage struct {
	Event      string    `json:"event"`
	ContractID string    `json:"contract_id"`
	CoopID     string    `json:"coop_id"`
	UserID     string    `json:"user_id"`
	Nick       string    `json:"nick"`
	Time       time.Time `json:"time"`
}

// PublishAMQPNonToken publishes a non-token event to the guild's AMQP queue.
func PublishAMQPNonToken(guildID string, url string, contractID string, coopID string, status AMQPNonTokenMessage) {
	body, err := json.Marshal(status)
	if err != nil {
		log.Printf("AMQP: error marshaling non-token: %v", err)
		return
	}

	client := getAMQPClient(guildID, url)
	go func() {
		if err := client.publish(body); err != nil {
			log.Printf("AMQP: error publishing non-token: %v", err)
		}
	}()
}

// AMQPContractUpdateMessage represents details of a contract when updated/reset.
type AMQPContractUpdateMessage struct {
	Event            string                 `json:"event"`
	ContractID       string                 `json:"contract_id"`
	CoopID           string                 `json:"coop_id"`
	ChannelID        string                 `json:"channel_id"`
	StartTime        time.Time              `json:"start_time"`
	CoopSize         int                    `json:"coop_size"`
	DeliveryTarget   float64                `json:"delivery_target"`
	GenerousGifts    string                 `json:"generous_gifts"`
	GGMultiplier     float64                `json:"gg_multiplier,omitempty"`
	MinutesPerToken  int                    `json:"minutes_per_token"`
	TimeLimitSeconds int                    `json:"time_limit_seconds,omitempty"`
	Modifiers        []AMQPContractModifier `json:"modifiers,omitempty"`
	PreviousScore    map[string]float64     `json:"previous_score,omitempty"`
	BoostOrder       *AMQPBoostOrder        `json:"boost_order,omitempty"`
}

// PublishAMQPContractUpdate publishes a contract update/reset event to the guild's AMQP queue.
func PublishAMQPContractUpdate(guildID string, url string, status AMQPContractUpdateMessage) {
	body, err := json.Marshal(status)
	if err != nil {
		log.Printf("AMQP: error marshaling contract update: %v", err)
		return
	}

	client := getAMQPClient(guildID, url)
	go func() {
		if err := client.publish(body); err != nil {
			log.Printf("AMQP: error publishing contract update: %v", err)
		}
	}()
}

// CheckAndPublishAMQPContractUpdate publishes an update event if contract has AMQP enabled.
func CheckAndPublishAMQPContractUpdate(contract *Contract) {
	if contract == nil || contract.Style&ContractFlagAMQP == 0 || len(contract.Location) == 0 {
		return
	}
	guildID := contract.Location[0].GuildID
	if guildID == "" {
		return
	}
	amqpURL := guildstate.GetGuildSettingString(guildID, "amqp_url")
	if amqpURL == "" {
		return
	}

	var deliveryTarget float64
	if contractInfo, ok := ei.EggIncContractsAll[contract.ContractID]; ok && len(contractInfo.TargetAmount) > 0 {
		deliveryTarget = contractInfo.TargetAmount[len(contractInfo.TargetAmount)-1]
	}
	gg, ugg, _ := ei.GetGenerousGiftEvent()
	generousGifts := "None"
	var ggMultiplier float64
	if ugg > 1.0 {
		generousGifts = "Ultra GG"
		ggMultiplier = ugg
	} else if gg > 1.0 {
		generousGifts = "Common GG"
		ggMultiplier = gg
	}

	mods := contract.Modifiers
	if len(mods) == 0 {
		mods = GetContractModifiers(contract.ContractID)
	}

	PublishAMQPContractUpdate(guildID, amqpURL, AMQPContractUpdateMessage{
		Event:            "contract_update",
		ContractID:       contract.ContractID,
		CoopID:           contract.CoopID,
		ChannelID:        contract.Location[0].ChannelID,
		StartTime:        contract.StartTime,
		CoopSize:         contract.CoopSize,
		DeliveryTarget:   deliveryTarget,
		GenerousGifts:    generousGifts,
		GGMultiplier:     ggMultiplier,
		MinutesPerToken:  contract.MinutesPerToken,
		TimeLimitSeconds: GetContractTimeLimitSeconds(contract),
		Modifiers:        mods,
		PreviousScore:    contract.PreviousScore,
		BoostOrder:       BuildAMQPBoostOrder(contract),
	})
}

// GetGameDimensionName returns the snake_case name of a GameModifier_GameDimension.
func GetGameDimensionName(d ei.GameModifier_GameDimension) string {
	switch d {
	case ei.GameModifier_EARNINGS:
		return "earnings"
	case ei.GameModifier_AWAY_EARNINGS:
		return "away_earnings"
	case ei.GameModifier_INTERNAL_HATCHERY_RATE:
		return "internal_hatchery_rate"
	case ei.GameModifier_EGG_LAYING_RATE:
		return "egg_laying_rate"
	case ei.GameModifier_SHIPPING_CAPACITY:
		return "shipping_capacity"
	case ei.GameModifier_HAB_CAPACITY:
		return "hab_capacity"
	case ei.GameModifier_VEHICLE_COST:
		return "vehicle_cost"
	case ei.GameModifier_HAB_COST:
		return "hab_cost"
	case ei.GameModifier_RESEARCH_COST:
		return "research_cost"
	default:
		return "unknown"
	}
}

// GetContractModifiers returns the active modifiers for a given contract ID.
func GetContractModifiers(contractID string) []AMQPContractModifier {
	if contractID == "" {
		return nil
	}

	ei.EggIncContractsMutex.RLock()
	contractInfo, ok := ei.EggIncContractsAll[contractID]
	ei.EggIncContractsMutex.RUnlock()
	if !ok {
		return nil
	}

	// 1. If RawModifiers are present on contractInfo, use them
	if len(contractInfo.RawModifiers) > 0 {
		var modifiers []AMQPContractModifier
		for _, mod := range contractInfo.RawModifiers {
			if mod == nil {
				continue
			}
			dim := mod.GetDimension()
			val := mod.GetValue()
			if val == 1.0 && dim == ei.GameModifier_INVALID {
				continue
			}
			modifiers = append(modifiers, AMQPContractModifier{
				Dimension:     int(dim),
				DimensionName: GetGameDimensionName(dim),
				Value:         val,
				Description:   mod.GetDescription(),
			})
		}
		if len(modifiers) > 0 {
			return modifiers
		}
	}

	// 2. Check Grade AAA if present
	if len(contractInfo.Grade) > int(ei.Contract_GRADE_AAA) {
		g := contractInfo.Grade[int(ei.Contract_GRADE_AAA)]
		if len(g.Modifiers) > 0 {
			var modifiers []AMQPContractModifier
			for _, mod := range g.Modifiers {
				if mod == nil {
					continue
				}
				dim := mod.GetDimension()
				val := mod.GetValue()
				if val == 1.0 && dim == ei.GameModifier_INVALID {
					continue
				}
				modifiers = append(modifiers, AMQPContractModifier{
					Dimension:     int(dim),
					DimensionName: GetGameDimensionName(dim),
					Value:         val,
					Description:   mod.GetDescription(),
				})
			}
			if len(modifiers) > 0 {
				return modifiers
			}
		}
	}

	// 3. Fallback to active fields on contractInfo (supports loaded contracts and test fixtures)
	var modifiers []AMQPContractModifier
	checkDim := func(dim ei.GameModifier_GameDimension, val float64) {
		if val != 0 && val != 1.0 {
			modifiers = append(modifiers, AMQPContractModifier{
				Dimension:     int(dim),
				DimensionName: GetGameDimensionName(dim),
				Value:         val,
			})
		}
	}

	checkDim(ei.GameModifier_EARNINGS, contractInfo.ModifierEarnings)
	checkDim(ei.GameModifier_AWAY_EARNINGS, contractInfo.ModifierAwayEarnings)
	checkDim(ei.GameModifier_INTERNAL_HATCHERY_RATE, contractInfo.ModifierIHR)
	checkDim(ei.GameModifier_EGG_LAYING_RATE, contractInfo.ModifierELR)
	checkDim(ei.GameModifier_SHIPPING_CAPACITY, contractInfo.ModifierSR)
	checkDim(ei.GameModifier_HAB_CAPACITY, contractInfo.ModifierHabCap)
	checkDim(ei.GameModifier_VEHICLE_COST, contractInfo.ModifierVehicleCost)
	checkDim(ei.GameModifier_HAB_COST, contractInfo.ModifierHabCost)
	checkDim(ei.GameModifier_RESEARCH_COST, contractInfo.ModifierResearchCost)

	return modifiers
}

// GetContractTimeLimitSeconds returns the contract time limit in seconds.
func GetContractTimeLimitSeconds(contract *Contract) int {
	if contract == nil {
		return 0
	}
	if contract.LengthInSeconds > 0 {
		return contract.LengthInSeconds
	}
	ei.EggIncContractsMutex.RLock()
	defer ei.EggIncContractsMutex.RUnlock()
	if cInfo, ok := ei.EggIncContractsAll[contract.ContractID]; ok {
		if cInfo.LengthInSeconds > 0 {
			return cInfo.LengthInSeconds
		}
		if len(cInfo.Grade) > int(ei.Contract_GRADE_AAA) && cInfo.Grade[int(ei.Contract_GRADE_AAA)].LengthInSeconds > 0 {
			return cInfo.Grade[int(ei.Contract_GRADE_AAA)].LengthInSeconds
		}
	}
	return 0
}

// AMQPBoosterChangeMessage represents a join or remove event for a booster in a contract.
type AMQPBoosterChangeMessage struct {
	Event      string    `json:"event"` // "booster_join" or "booster_remove"
	ContractID string    `json:"contract_id"`
	CoopID     string    `json:"coop_id"`
	UserID     string    `json:"user_id"`
	Nick       string    `json:"nick"`
	Time       time.Time `json:"time"`
}

// PublishAMQPBoosterChange publishes a booster join/remove event to the guild's AMQP queue.
func PublishAMQPBoosterChange(guildID string, url string, status AMQPBoosterChangeMessage) {
	body, err := json.Marshal(status)
	if err != nil {
		log.Printf("AMQP: error marshaling booster change: %v", err)
		return
	}

	client := getAMQPClient(guildID, url)
	go func() {
		if err := client.publish(body); err != nil {
			log.Printf("AMQP: error publishing booster change: %v", err)
		}
	}()
}

// CheckAndPublishAMQPBoosterChange publishes a booster join or remove event if contract has AMQP enabled.
func CheckAndPublishAMQPBoosterChange(contract *Contract, userID string, nick string, event string) {
	if contract == nil || contract.Style&ContractFlagAMQP == 0 || len(contract.Location) == 0 {
		return
	}
	guildID := contract.Location[0].GuildID
	if guildID == "" {
		return
	}
	amqpURL := guildstate.GetGuildSettingString(guildID, "amqp_url")
	if amqpURL == "" {
		return
	}

	PublishAMQPBoosterChange(guildID, amqpURL, AMQPBoosterChangeMessage{
		Event:      event,
		ContractID: contract.ContractID,
		CoopID:     contract.CoopID,
		UserID:     userID,
		Nick:       nick,
		Time:       time.Now(),
	})
}

// findPreviousCXPScore extracts the CXP score from an archive for a specific contractID,
// skipping entries matching currentCoopID.
func findPreviousCXPScore(archive []*ei.LocalContract, contractID, currentCoopID string) (float64, bool) {
	var maxScore float64
	var found bool
	for _, lc := range archive {
		lcContractID := lc.GetContractIdentifier()
		if lcContractID == "" && lc.GetContract() != nil {
			lcContractID = lc.GetContract().GetIdentifier()
		} else if lcContractID == "" && lc.GetEvaluation() != nil {
			lcContractID = lc.GetEvaluation().GetContractIdentifier()
		}
		if !strings.EqualFold(lcContractID, contractID) {
			continue
		}
		eval := lc.GetEvaluation()
		if eval == nil {
			continue
		}
		if currentCoopID != "" && strings.EqualFold(eval.GetCoopIdentifier(), currentCoopID) {
			continue
		}
		cxp := eval.GetCxp()
		if !found || cxp > maxScore {
			maxScore = cxp
			found = true
		}
	}
	return maxScore, found
}

// GatherContractPreviousScores fetches the CXP score from the previous time each player ran the contract,
// saving it into contract.PreviousScore and each booster's PreviousScore.
func GatherContractPreviousScores(contract *Contract) {
	if contract == nil {
		return
	}

	contract.mutex.Lock()
	if len(contract.Boosters) == 0 {
		contract.mutex.Unlock()
		return
	}

	if contract.PreviousScore == nil {
		contract.PreviousScore = make(map[string]float64)
	}

	// Sync any scores already present in contract.PreviousScore to Boosters
	for userID, score := range contract.PreviousScore {
		if b, ok := contract.Boosters[userID]; ok && b != nil && b.PreviousScore == 0 {
			b.PreviousScore = score
		}
	}

	type playerJob struct {
		userID   string
		eggIncID string
	}

	var jobs []playerJob
	for userID, b := range contract.Boosters {
		if b != nil && b.PreviousScore > 0 {
			contract.PreviousScore[userID] = b.PreviousScore
			continue
		}
		if contract.PreviousScore[userID] > 0 {
			if b != nil {
				b.PreviousScore = contract.PreviousScore[userID]
			}
			continue
		}

		eiCipher := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
		if eiCipher == "" && !dc.IsSnowflake(userID) {
			if discordID, err := farmerstate.GetDiscordUserIDFromEiIgnExact(userID); err == nil && discordID != "" {
				eiCipher = farmerstate.GetMiscSettingString(discordID, "encrypted_ei_id")
			}
		}
		if eiCipher == "" {
			continue
		}
		eggIncID := ei.DecryptEID(eiCipher)
		if len(eggIncID) != 18 || !strings.HasPrefix(eggIncID, "EI") {
			continue
		}
		jobs = append(jobs, playerJob{
			userID:   userID,
			eggIncID: eggIncID,
		})
	}
	contractID := contract.ContractID
	coopID := contract.CoopID
	contract.mutex.Unlock()

	if len(jobs) == 0 {
		return
	}

	maxParallelJobs := max(min(len(jobs), maxParallel), 1)
	jobsCh := make(chan playerJob, len(jobs))
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for j := range jobsCh {
			if ign := farmerstate.GetMiscSettingString(j.userID, "ei_ign"); ign == "" {
				if backup, _ := ei.GetFirstContactFromAPI(j.eggIncID, j.userID, true); backup != nil {
					farmerstate.SetFarmerBackupDetails(j.userID, backup)
				}
			}

			archive, _ := ei.GetContractArchiveFromAPI(j.eggIncID, j.userID, false, true)
			if score, found := findPreviousCXPScore(archive, contractID, coopID); found {
				contract.mutex.Lock()
				if contract.PreviousScore == nil {
					contract.PreviousScore = make(map[string]float64)
				}
				contract.PreviousScore[j.userID] = score
				if b, ok := contract.Boosters[j.userID]; ok && b != nil {
					b.PreviousScore = score
				}
				contract.mutex.Unlock()
			}
		}
	}

	wg.Add(maxParallelJobs)
	for i := 0; i < maxParallelJobs; i++ {
		go worker()
	}
	for _, j := range jobs {
		jobsCh <- j
	}
	close(jobsCh)
	wg.Wait()
}

// FetchBoosterPreviousScore fetches the previous CXP score for a single player in a goroutine if not already present.
func FetchBoosterPreviousScore(contract *Contract, userID string) {
	if contract == nil || userID == "" {
		return
	}

	contract.mutex.Lock()
	if contract.PreviousScore != nil && contract.PreviousScore[userID] > 0 {
		if b, ok := contract.Boosters[userID]; ok && b != nil && b.PreviousScore == 0 {
			b.PreviousScore = contract.PreviousScore[userID]
		}
		contract.mutex.Unlock()
		return
	}
	if b, ok := contract.Boosters[userID]; ok && b != nil && b.PreviousScore > 0 {
		if contract.PreviousScore == nil {
			contract.PreviousScore = make(map[string]float64)
		}
		contract.PreviousScore[userID] = b.PreviousScore
		contract.mutex.Unlock()
		return
	}
	contractID := contract.ContractID
	coopID := contract.CoopID
	contract.mutex.Unlock()

	go func() {
		eiCipher := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
		if eiCipher == "" && !dc.IsSnowflake(userID) {
			if discordID, err := farmerstate.GetDiscordUserIDFromEiIgnExact(userID); err == nil && discordID != "" {
				eiCipher = farmerstate.GetMiscSettingString(discordID, "encrypted_ei_id")
			}
		}
		if eiCipher == "" {
			return
		}
		eggIncID := ei.DecryptEID(eiCipher)
		if len(eggIncID) != 18 || !strings.HasPrefix(eggIncID, "EI") {
			return
		}

		if ign := farmerstate.GetMiscSettingString(userID, "ei_ign"); ign == "" {
			if backup, _ := ei.GetFirstContactFromAPI(eggIncID, userID, true); backup != nil {
				farmerstate.SetFarmerBackupDetails(userID, backup)
			}
		}

		archive, _ := ei.GetContractArchiveFromAPI(eggIncID, userID, false, true)
		if score, found := findPreviousCXPScore(archive, contractID, coopID); found {
			contract.mutex.Lock()
			if contract.PreviousScore == nil {
				contract.PreviousScore = make(map[string]float64)
			}
			contract.PreviousScore[userID] = score
			if b, ok := contract.Boosters[userID]; ok && b != nil {
				b.PreviousScore = score
			}
			contract.mutex.Unlock()
		}
	}()
}
