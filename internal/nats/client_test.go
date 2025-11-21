package nats

import (
	"testing"
	"time"
)

func TestStreamConfig_Fields(t *testing.T) {
	cfg := StreamConfig{
		Name:        "test-stream",
		Subjects:    []string{"test.>"},
		MaxMsgs:     10000,
		MaxBytes:    1024 * 1024,
		MaxAge:      24 * time.Hour,
		Replicas:    3,
		Description: "Test stream",
	}

	if cfg.Name != "test-stream" {
		t.Errorf("Name = %s, want test-stream", cfg.Name)
	}
	if len(cfg.Subjects) != 1 || cfg.Subjects[0] != "test.>" {
		t.Errorf("Subjects = %v, want [test.>]", cfg.Subjects)
	}
	if cfg.MaxMsgs != 10000 {
		t.Errorf("MaxMsgs = %d, want 10000", cfg.MaxMsgs)
	}
	if cfg.MaxBytes != 1024*1024 {
		t.Errorf("MaxBytes = %d, want %d", cfg.MaxBytes, 1024*1024)
	}
	if cfg.MaxAge != 24*time.Hour {
		t.Errorf("MaxAge = %v, want 24h", cfg.MaxAge)
	}
	if cfg.Replicas != 3 {
		t.Errorf("Replicas = %d, want 3", cfg.Replicas)
	}
	if cfg.Description != "Test stream" {
		t.Errorf("Description = %s, want 'Test stream'", cfg.Description)
	}
}

func TestStreamConfig_Defaults(t *testing.T) {
	cfg := StreamConfig{}

	if cfg.Name != "" {
		t.Errorf("default Name = %s, want empty", cfg.Name)
	}
	if cfg.Subjects != nil {
		t.Error("default Subjects should be nil")
	}
	if cfg.MaxMsgs != 0 {
		t.Errorf("default MaxMsgs = %d, want 0", cfg.MaxMsgs)
	}
	if cfg.Replicas != 0 {
		t.Errorf("default Replicas = %d, want 0", cfg.Replicas)
	}
}

func TestClient_NilState(t *testing.T) {
	// Test client with nil connections
	client := &Client{}

	if client.IsConnected() {
		t.Error("IsConnected() should return false for nil connection")
	}

	if client.JetStream() != nil {
		t.Error("JetStream() should return nil")
	}

	if client.Conn() != nil {
		t.Error("Conn() should return nil")
	}

	// HealthCheck should return error
	err := client.HealthCheck()
	if err == nil {
		t.Error("HealthCheck() should return error for nil connection")
	}
}

func TestClient_CloseIdempotent(t *testing.T) {
	client := &Client{}

	// Close should be safe to call multiple times
	client.Close()
	client.Close()
	client.Close()

	// Should be marked as closed
	if !client.closed {
		t.Error("client should be marked as closed")
	}
}

func TestClient_URL(t *testing.T) {
	client := &Client{
		url: "nats://localhost:4222",
	}

	if client.url != "nats://localhost:4222" {
		t.Errorf("url = %s, want nats://localhost:4222", client.url)
	}
}

func TestNewClient_InvalidURL(t *testing.T) {
	// NewClient with invalid URL should fail
	// Using an unreachable URL to test error handling
	_, err := NewClient("nats://invalid-host-that-does-not-exist:4222")

	// Should return an error (connection refused or similar)
	if err == nil {
		t.Error("NewClient() should return error for invalid URL")
	}
}

func TestClient_CreateStream_NotConnected(t *testing.T) {
	client := &Client{}

	_, err := client.CreateStream(nil, StreamConfig{Name: "test"})
	if err == nil {
		t.Error("CreateStream() should return error when not connected")
	}
}

func TestClient_CreateConsumer_NotConnected(t *testing.T) {
	client := &Client{}

	_, err := client.CreateConsumer(nil, "stream", "consumer", "subject")
	if err == nil {
		t.Error("CreateConsumer() should return error when not connected")
	}
}

func TestClient_Publish_NotConnected(t *testing.T) {
	client := &Client{}

	_, err := client.Publish(nil, "subject", []byte("data"))
	if err == nil {
		t.Error("Publish() should return error when not connected")
	}
}

func TestClient_SetupStreams_NotConnected(t *testing.T) {
	client := &Client{}

	err := client.SetupStreams(nil)
	if err == nil {
		t.Error("SetupStreams() should return error when not connected")
	}
}

func TestDefaultStreamConfig_Values(t *testing.T) {
	cfg := DefaultStreamConfig()

	if cfg.Name != StreamJobs {
		t.Errorf("Name = %s, want %s", cfg.Name, StreamJobs)
	}

	if len(cfg.Subjects) != 1 {
		t.Errorf("len(Subjects) = %d, want 1", len(cfg.Subjects))
	}

	if cfg.Subjects[0] != SubjectJobsAll {
		t.Errorf("Subjects[0] = %s, want %s", cfg.Subjects[0], SubjectJobsAll)
	}

	if cfg.MaxMsgs != 100000 {
		t.Errorf("MaxMsgs = %d, want 100000", cfg.MaxMsgs)
	}

	if cfg.MaxBytes != 1024*1024*500 {
		t.Errorf("MaxBytes = %d, want %d", cfg.MaxBytes, 1024*1024*500)
	}

	if cfg.MaxAge != 7*24*time.Hour {
		t.Errorf("MaxAge = %v, want 7 days", cfg.MaxAge)
	}

	if cfg.Replicas != 1 {
		t.Errorf("Replicas = %d, want 1", cfg.Replicas)
	}

	if cfg.Description == "" {
		t.Error("Description should not be empty")
	}
}

func TestSubjectConstants(t *testing.T) {
	// Verify all subject constants have the correct prefix
	subjects := map[string]string{
		"ingestion":   SubjectJobIngestion,
		"modeling":    SubjectJobModeling,
		"planning":    SubjectJobPlanning,
		"generation":  SubjectJobGeneration,
		"mutation":    SubjectJobMutation,
		"integration": SubjectJobIntegration,
	}

	for name, subject := range subjects {
		expected := "jobs." + name
		if subject != expected {
			t.Errorf("Subject%s = %s, want %s", name, subject, expected)
		}
	}
}

func TestConsumerConstants(t *testing.T) {
	// Verify all consumer constants have the correct suffix
	consumers := map[string]string{
		"ingestion":   ConsumerIngestion,
		"modeling":    ConsumerModeling,
		"planning":    ConsumerPlanning,
		"generation":  ConsumerGeneration,
		"mutation":    ConsumerMutation,
		"integration": ConsumerIntegration,
	}

	for name, consumer := range consumers {
		expected := name + "-worker"
		if consumer != expected {
			t.Errorf("Consumer%s = %s, want %s", name, consumer, expected)
		}
	}
}
