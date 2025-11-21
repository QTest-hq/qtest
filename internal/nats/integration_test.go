//go:build integration
// +build integration

package nats

import (
	"context"
	"testing"
	"time"

	"github.com/QTest-hq/qtest/internal/testutil"
)

func TestIntegration_NewClient(t *testing.T) {
	testNATS := testutil.RequireNATS(t)

	client, err := NewClient(testNATS.URL)
	if err != nil {
		t.Skipf("skipping test: could not connect to NATS: %v", err)
	}
	defer client.Close()

	if !client.IsConnected() {
		t.Error("client should be connected")
	}

	if client.Conn() == nil {
		t.Error("Conn() should not be nil")
	}

	if client.JetStream() == nil {
		t.Error("JetStream() should not be nil")
	}
}

func TestIntegration_HealthCheck(t *testing.T) {
	testNATS := testutil.RequireNATS(t)

	client, err := NewClient(testNATS.URL)
	if err != nil {
		t.Skipf("skipping test: could not connect to NATS: %v", err)
	}
	defer client.Close()

	if err := client.HealthCheck(); err != nil {
		t.Errorf("HealthCheck() error: %v", err)
	}
}

func TestIntegration_CreateStream(t *testing.T) {
	testNATS := testutil.RequireNATS(t)

	client, err := NewClient(testNATS.URL)
	if err != nil {
		t.Skipf("skipping test: could not connect to NATS: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := StreamConfig{
		Name:        "TEST_INTEGRATION_STREAM",
		Subjects:    []string{"test.integration.>"},
		MaxMsgs:     1000,
		MaxBytes:    1024 * 1024, // 1MB
		MaxAge:      1 * time.Hour,
		Description: "Integration test stream",
	}

	stream, err := client.CreateStream(ctx, cfg)
	if err != nil {
		t.Fatalf("CreateStream() error: %v", err)
	}

	if stream == nil {
		t.Fatal("CreateStream() returned nil stream")
	}

	// Cleanup: delete stream
	js := client.JetStream()
	if js != nil {
		js.DeleteStream(ctx, cfg.Name)
	}
}

func TestIntegration_CreateStreamWithDefaults(t *testing.T) {
	testNATS := testutil.RequireNATS(t)

	client, err := NewClient(testNATS.URL)
	if err != nil {
		t.Skipf("skipping test: could not connect to NATS: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Minimal config - should use defaults
	cfg := StreamConfig{
		Name:     "TEST_DEFAULTS_STREAM",
		Subjects: []string{"test.defaults.>"},
	}

	stream, err := client.CreateStream(ctx, cfg)
	if err != nil {
		t.Fatalf("CreateStream() error: %v", err)
	}

	if stream == nil {
		t.Fatal("CreateStream() returned nil stream")
	}

	// Cleanup
	js := client.JetStream()
	if js != nil {
		js.DeleteStream(ctx, cfg.Name)
	}
}

func TestIntegration_CreateConsumer(t *testing.T) {
	testNATS := testutil.RequireNATS(t)

	client, err := NewClient(testNATS.URL)
	if err != nil {
		t.Skipf("skipping test: could not connect to NATS: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First create a stream
	streamCfg := StreamConfig{
		Name:     "TEST_CONSUMER_STREAM",
		Subjects: []string{"test.consumer.>"},
	}

	_, err = client.CreateStream(ctx, streamCfg)
	if err != nil {
		t.Fatalf("CreateStream() error: %v", err)
	}

	// Create consumer
	consumer, err := client.CreateConsumer(ctx, "TEST_CONSUMER_STREAM", "test-consumer", "test.consumer.jobs")
	if err != nil {
		t.Fatalf("CreateConsumer() error: %v", err)
	}

	if consumer == nil {
		t.Fatal("CreateConsumer() returned nil consumer")
	}

	// Cleanup
	js := client.JetStream()
	if js != nil {
		js.DeleteStream(ctx, streamCfg.Name)
	}
}

func TestIntegration_PublishAndReceive(t *testing.T) {
	testNATS := testutil.RequireNATS(t)

	client, err := NewClient(testNATS.URL)
	if err != nil {
		t.Skipf("skipping test: could not connect to NATS: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create stream
	streamCfg := StreamConfig{
		Name:     "TEST_PUBSUB_STREAM",
		Subjects: []string{"test.pubsub.>"},
	}

	_, err = client.CreateStream(ctx, streamCfg)
	if err != nil {
		t.Fatalf("CreateStream() error: %v", err)
	}

	// Publish message
	testData := []byte(`{"job_id": "test-123", "type": "test"}`)
	ack, err := client.Publish(ctx, "test.pubsub.jobs", testData)
	if err != nil {
		t.Fatalf("Publish() error: %v", err)
	}

	if ack == nil {
		t.Fatal("Publish() returned nil ack")
	}

	if ack.Stream != "TEST_PUBSUB_STREAM" {
		t.Errorf("ack.Stream = %s, want TEST_PUBSUB_STREAM", ack.Stream)
	}

	// Cleanup
	js := client.JetStream()
	if js != nil {
		js.DeleteStream(ctx, streamCfg.Name)
	}
}

func TestIntegration_Close(t *testing.T) {
	testNATS := testutil.RequireNATS(t)

	client, err := NewClient(testNATS.URL)
	if err != nil {
		t.Skipf("skipping test: could not connect to NATS: %v", err)
	}

	if !client.IsConnected() {
		t.Error("client should be connected before close")
	}

	client.Close()

	// After close, IsConnected should return false
	if client.IsConnected() {
		t.Error("client should not be connected after close")
	}

	// Second close should be safe (idempotent)
	client.Close()
}

func TestIntegration_HealthCheckAfterClose(t *testing.T) {
	testNATS := testutil.RequireNATS(t)

	client, err := NewClient(testNATS.URL)
	if err != nil {
		t.Skipf("skipping test: could not connect to NATS: %v", err)
	}

	client.Close()

	// HealthCheck should return error after close
	if err := client.HealthCheck(); err == nil {
		t.Error("HealthCheck() should return error after close")
	}
}

func TestIntegration_PublishNotConnected(t *testing.T) {
	testNATS := testutil.RequireNATS(t)

	client, err := NewClient(testNATS.URL)
	if err != nil {
		t.Skipf("skipping test: could not connect to NATS: %v", err)
	}

	client.Close()

	ctx := context.Background()
	_, err = client.Publish(ctx, "test.subject", []byte("data"))
	if err == nil {
		t.Error("Publish() should return error when not connected")
	}
}

func TestIntegration_CreateStreamNotConnected(t *testing.T) {
	testNATS := testutil.RequireNATS(t)

	client, err := NewClient(testNATS.URL)
	if err != nil {
		t.Skipf("skipping test: could not connect to NATS: %v", err)
	}

	client.Close()

	ctx := context.Background()
	cfg := StreamConfig{
		Name:     "TEST_STREAM",
		Subjects: []string{"test.>"},
	}

	_, err = client.CreateStream(ctx, cfg)
	if err == nil {
		t.Error("CreateStream() should return error when not connected")
	}
}

func TestIntegration_CreateConsumerNotConnected(t *testing.T) {
	testNATS := testutil.RequireNATS(t)

	client, err := NewClient(testNATS.URL)
	if err != nil {
		t.Skipf("skipping test: could not connect to NATS: %v", err)
	}

	client.Close()

	ctx := context.Background()
	_, err = client.CreateConsumer(ctx, "stream", "consumer", "subject")
	if err == nil {
		t.Error("CreateConsumer() should return error when not connected")
	}
}

func TestIntegration_SetupStreams(t *testing.T) {
	testNATS := testutil.RequireNATS(t)

	client, err := NewClient(testNATS.URL)
	if err != nil {
		t.Skipf("skipping test: could not connect to NATS: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// SetupStreams should create the default job stream
	err = client.SetupStreams(ctx)
	if err != nil {
		t.Fatalf("SetupStreams() error: %v", err)
	}

	// Verify stream was created
	js := client.JetStream()
	if js == nil {
		t.Fatal("JetStream() returned nil")
	}

	// Check if the stream exists
	stream, err := js.Stream(ctx, StreamJobs)
	if err != nil {
		t.Fatalf("failed to get stream: %v", err)
	}

	if stream == nil {
		t.Error("stream should exist after SetupStreams")
	}

	// Cleanup
	js.DeleteStream(ctx, StreamJobs)
}

func TestIntegration_SubjectForJobType_AllTypes(t *testing.T) {
	tests := []struct {
		jobType string
		want    string
	}{
		{"ingestion", SubjectJobIngestion},
		{"modeling", SubjectJobModeling},
		{"planning", SubjectJobPlanning},
		{"generation", SubjectJobGeneration},
		{"mutation", SubjectJobMutation},
		{"integration", SubjectJobIntegration},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.jobType, func(t *testing.T) {
			got := SubjectForJobType(tt.jobType)
			if got != tt.want {
				t.Errorf("SubjectForJobType(%s) = %s, want %s", tt.jobType, got, tt.want)
			}
		})
	}
}

func TestIntegration_ConsumerForJobType_AllTypes(t *testing.T) {
	tests := []struct {
		jobType string
		want    string
	}{
		{"ingestion", ConsumerIngestion},
		{"modeling", ConsumerModeling},
		{"planning", ConsumerPlanning},
		{"generation", ConsumerGeneration},
		{"mutation", ConsumerMutation},
		{"integration", ConsumerIntegration},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.jobType, func(t *testing.T) {
			got := ConsumerForJobType(tt.jobType)
			if got != tt.want {
				t.Errorf("ConsumerForJobType(%s) = %s, want %s", tt.jobType, got, tt.want)
			}
		})
	}
}
