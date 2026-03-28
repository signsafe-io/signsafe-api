package queue

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Client wraps a RabbitMQ connection and channel.
type Client struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

// NewClient establishes a connection to RabbitMQ and declares the required queues.
func NewClient(amqpURL string) (*Client, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, fmt.Errorf("queue.NewClient: dial: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("queue.NewClient: channel: %w", err)
	}

	c := &Client{conn: conn, channel: ch}

	// Declare queues with DLQ support.
	queues := []string{"ingestion.jobs", "analysis.jobs"}
	for _, q := range queues {
		if err := c.declareQueue(q); err != nil {
			c.Close()
			return nil, fmt.Errorf("queue.NewClient: declare %s: %w", q, err)
		}
	}

	return c, nil
}

func (c *Client) declareQueue(name string) error {
	dlqName := name + ".dlq"

	// Declare DLQ first.
	_, err := c.channel.QueueDeclare(dlqName, true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("declareQueue %s dlq: %w", dlqName, err)
	}

	// Declare main queue with dead-letter routing.
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

// Publish marshals v as JSON and publishes it to the named queue.
func (c *Client) Publish(ctx context.Context, queue string, v interface{}) error {
	body, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("queue.Publish: marshal: %w", err)
	}

	err = c.channel.PublishWithContext(ctx, "", queue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
	if err != nil {
		return fmt.Errorf("queue.Publish: publish to %s: %w", queue, err)
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
