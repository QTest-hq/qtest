// Package nats provides NATS JetStream client for job queue management
package nats

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog/log"
)

// StreamConfig defines configuration for a JetStream stream
type StreamConfig struct {
	Name        string
	Subjects    []string
	MaxMsgs     int64
	MaxBytes    int64
	MaxAge      time.Duration
	Replicas    int
	Description string
}

// Client wraps NATS connection and JetStream context
type Client struct {
	nc     *nats.Conn
	js     jetstream.JetStream
	url    string
	mu     sync.RWMutex
	closed bool
}

// NewClient creates a new NATS client with the given URL
func NewClient(url string) (*Client, error) {
	c := &Client{url: url}
	if err := c.connect(); err != nil {
		return nil, err
	}
	return c, nil
}

// connect establishes connection to NATS server
func (c *Client) connect() error {
	opts := []nats.Option{
		nats.Name("qtest-worker"),
		nats.ReconnectWait(2 * time.Second),
		nats.MaxReconnects(-1), // Infinite reconnects
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Info().Str("url", nc.ConnectedUrl()).Msg("reconnected to NATS")
		}),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				log.Warn().Err(err).Msg("disconnected from NATS")
			}
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			log.Error().Err(err).Msg("NATS error")
		}),
	}

	nc, err := nats.Connect(c.url, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return fmt.Errorf("failed to create JetStream context: %w", err)
	}

	c.nc = nc
	c.js = js

	log.Info().Str("url", c.url).Msg("connected to NATS JetStream")
	return nil
}

// JetStream returns the JetStream context
func (c *Client) JetStream() jetstream.JetStream {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.js
}

// Conn returns the underlying NATS connection
func (c *Client) Conn() *nats.Conn {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nc
}

// CreateStream creates or updates a JetStream stream
func (c *Client) CreateStream(ctx context.Context, cfg StreamConfig) (jetstream.Stream, error) {
	c.mu.RLock()
	js := c.js
	c.mu.RUnlock()

	if js == nil {
		return nil, fmt.Errorf("not connected to NATS")
	}

	streamCfg := jetstream.StreamConfig{
		Name:        cfg.Name,
		Subjects:    cfg.Subjects,
		MaxMsgs:     cfg.MaxMsgs,
		MaxBytes:    cfg.MaxBytes,
		MaxAge:      cfg.MaxAge,
		Replicas:    cfg.Replicas,
		Description: cfg.Description,
		Storage:     jetstream.FileStorage,
		Retention:   jetstream.WorkQueuePolicy, // Each message delivered once
		Discard:     jetstream.DiscardOld,
	}

	// Set defaults
	if streamCfg.MaxMsgs == 0 {
		streamCfg.MaxMsgs = 100000
	}
	if streamCfg.MaxBytes == 0 {
		streamCfg.MaxBytes = 1024 * 1024 * 100 // 100MB
	}
	if streamCfg.MaxAge == 0 {
		streamCfg.MaxAge = 7 * 24 * time.Hour // 7 days
	}
	if streamCfg.Replicas == 0 {
		streamCfg.Replicas = 1
	}

	stream, err := js.CreateOrUpdateStream(ctx, streamCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream %s: %w", cfg.Name, err)
	}

	log.Debug().Str("stream", cfg.Name).Strs("subjects", cfg.Subjects).Msg("stream ready")
	return stream, nil
}

// CreateConsumer creates a durable consumer for a stream
func (c *Client) CreateConsumer(ctx context.Context, streamName, consumerName string, filterSubject string) (jetstream.Consumer, error) {
	c.mu.RLock()
	js := c.js
	c.mu.RUnlock()

	if js == nil {
		return nil, fmt.Errorf("not connected to NATS")
	}

	consumerCfg := jetstream.ConsumerConfig{
		Name:          consumerName,
		Durable:       consumerName,
		FilterSubject: filterSubject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       5 * time.Minute, // Time to process before redelivery
		MaxDeliver:    5,               // Max redelivery attempts
		MaxAckPending: 100,             // Max unacked messages
	}

	consumer, err := js.CreateOrUpdateConsumer(ctx, streamName, consumerCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer %s: %w", consumerName, err)
	}

	log.Debug().
		Str("stream", streamName).
		Str("consumer", consumerName).
		Str("filter", filterSubject).
		Msg("consumer ready")

	return consumer, nil
}

// Publish publishes a message to a subject
func (c *Client) Publish(ctx context.Context, subject string, data []byte) (*jetstream.PubAck, error) {
	c.mu.RLock()
	js := c.js
	c.mu.RUnlock()

	if js == nil {
		return nil, fmt.Errorf("not connected to NATS")
	}

	ack, err := js.Publish(ctx, subject, data)
	if err != nil {
		return nil, fmt.Errorf("failed to publish to %s: %w", subject, err)
	}

	return ack, nil
}

// IsConnected returns true if connected to NATS
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.nc == nil {
		return false
	}
	return c.nc.IsConnected()
}

// Close closes the NATS connection
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}

	c.closed = true
	if c.nc != nil {
		c.nc.Close()
		log.Info().Msg("NATS connection closed")
	}
}

// HealthCheck verifies NATS connectivity
func (c *Client) HealthCheck() error {
	c.mu.RLock()
	nc := c.nc
	c.mu.RUnlock()

	if nc == nil || !nc.IsConnected() {
		return fmt.Errorf("not connected to NATS")
	}
	return nil
}
