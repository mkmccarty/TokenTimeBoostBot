package boost

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

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

// AMQPContractStartMessage represents details of a contract when started.
type AMQPContractStartMessage struct {
	Event          string    `json:"event"`
	ContractID     string    `json:"contract_id"`
	CoopID         string    `json:"coop_id"`
	StartTime      time.Time `json:"start_time"`
	CoopSize       int       `json:"coop_size"`
	DeliveryTarget float64   `json:"delivery_target"`
	GenerousGifts  string    `json:"generous_gifts"`
	GGMultiplier   float64   `json:"gg_multiplier,omitempty"`
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
