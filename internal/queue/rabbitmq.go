package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Client wraps a RabbitMQ connection and channel with automatic reconnection.
type Client struct {
	amqpURL string
	mu      sync.Mutex
	conn    *amqp.Connection
	channel *amqp.Channel
}

// NewClient establishes a connection to RabbitMQ and declares the required queues.
func NewClient(amqpURL string) (*Client, error) {
	c := &Client{amqpURL: amqpURL}
	if err := c.connect(); err != nil {
		return nil, fmt.Errorf("queue.NewClient: %w", err)
	}
	return c, nil
}

// connect (re)establishes the AMQP connection and channel, then declares queues.
// Caller must hold c.mu or be in a single-goroutine context (e.g. NewClient).
func (c *Client) connect() error {
	conn, err := amqp.Dial(c.amqpURL)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("channel: %w", err)
	}

	c.conn = conn
	c.channel = ch

	queues := []string{"ingestion.jobs", "analysis.jobs"}
	for _, q := range queues {
		if err := c.declareQueue(q); err != nil {
			c.conn.Close()
			return fmt.Errorf("declare %s: %w", q, err)
		}
	}
	return nil
}

func (c *Client) declareQueue(name string) error {
	dlqName := name + ".dlq"

	_, err := c.channel.QueueDeclare(dlqName, true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("declareQueue %s dlq: %w", dlqName, err)
	}

	args := amqp.Table{
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": dlqName,
	}
	_, err = c.channel.QueueDeclare(name, true, false, false, false, args)
	if err != nil {
		return fmt.Errorf("declareQueue %s: %w", name, err)
	}
	return nil
}

// ensureConnected reconnects if the connection or channel is no longer open.
func (c *Client) ensureConnected() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil && !c.conn.IsClosed() && c.channel != nil {
		return nil
	}

	slog.Warn("rabbitmq connection lost, reconnecting")
	if err := c.connect(); err != nil {
		return fmt.Errorf("queue.reconnect: %w", err)
	}
	slog.Info("rabbitmq reconnected")
	return nil
}

// Publish marshals v as JSON and publishes it to the named queue.
// If the channel is closed it reconnects once and retries.
func (c *Client) Publish(ctx context.Context, queue string, v interface{}) error {
	body, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("queue.Publish: marshal: %w", err)
	}

	if err := c.ensureConnected(); err != nil {
		return fmt.Errorf("queue.Publish: %w", err)
	}

	c.mu.Lock()
	err = c.channel.PublishWithContext(ctx, "", queue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
	c.mu.Unlock()

	if err != nil {
		// Channel may have been closed mid-publish; reconnect and retry once.
		slog.Warn("rabbitmq publish failed, reconnecting and retrying", "queue", queue, "error", err)
		if reconnErr := c.ensureConnected(); reconnErr != nil {
			return fmt.Errorf("queue.Publish: retry reconnect: %w", reconnErr)
		}

		c.mu.Lock()
		err = c.channel.PublishWithContext(ctx, "", queue, false, false, amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		})
		c.mu.Unlock()

		if err != nil {
			return fmt.Errorf("queue.Publish: publish to %s: %w", queue, err)
		}
	}
	return nil
}

// Ping checks that the RabbitMQ connection and channel are open.
func (c *Client) Ping() error {
	if c.conn == nil || c.conn.IsClosed() {
		return fmt.Errorf("queue.Ping: connection is closed")
	}
	if c.channel == nil {
		return fmt.Errorf("queue.Ping: channel is nil")
	}
	return nil
}

// Close cleans up channel and connection.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.channel != nil {
		c.channel.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}

// IngestionMessage is the payload sent to ingestion.jobs.
type IngestionMessage struct {
	ContractID string `json:"contractId"`
	JobID      string `json:"jobId"`
	FilePath   string `json:"filePath"`
}

// AnalysisMessage is the payload sent to analysis.jobs.
type AnalysisMessage struct {
	ContractID string `json:"contractId"`
	AnalysisID string `json:"analysisId"`
}
