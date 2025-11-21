package worker

import (
	"testing"

	"github.com/QTest-hq/qtest/internal/config"
	"github.com/QTest-hq/qtest/internal/jobs"
)

func TestNewPool_AllWorkers(t *testing.T) {
	cfg := &config.Config{}

	pool, err := NewPool(PoolConfig{
		Config:     cfg,
		WorkerType: "all",
	})

	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}

	if pool == nil {
		t.Fatal("pool should not be nil")
	}

	// Should have 6 workers (one for each job type)
	if len(pool.workers) != 6 {
		t.Errorf("len(workers) = %d, want 6", len(pool.workers))
	}
}

func TestNewPool_SingleWorker(t *testing.T) {
	cfg := &config.Config{}

	tests := []struct {
		workerType string
		wantName   string
	}{
		{"ingestion", "ingestion"},
		{"modeling", "modeling"},
		{"planning", "planning"},
		{"generation", "generation"},
		{"mutation", "mutation"},
		{"integration", "integration"},
	}

	for _, tt := range tests {
		t.Run(tt.workerType, func(t *testing.T) {
			pool, err := NewPool(PoolConfig{
				Config:     cfg,
				WorkerType: tt.workerType,
			})

			if err != nil {
				t.Fatalf("NewPool failed: %v", err)
			}

			if len(pool.workers) != 1 {
				t.Errorf("len(workers) = %d, want 1", len(pool.workers))
			}

			if pool.workers[0].Name() != tt.wantName {
				t.Errorf("worker.Name() = %s, want %s", pool.workers[0].Name(), tt.wantName)
			}
		})
	}
}

func TestNewPool_UnknownWorkerType(t *testing.T) {
	cfg := &config.Config{}

	_, err := NewPool(PoolConfig{
		Config:     cfg,
		WorkerType: "unknown",
	})

	if err == nil {
		t.Error("expected error for unknown worker type")
	}
}

func TestNewPool_NilConfig(t *testing.T) {
	pool, err := NewPool(PoolConfig{
		Config:     nil,
		WorkerType: "ingestion",
	})

	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}

	// Should still create pool even with nil config
	if pool == nil {
		t.Fatal("pool should not be nil")
	}
}

func TestPool_Pipeline(t *testing.T) {
	cfg := &config.Config{}

	pool, _ := NewPool(PoolConfig{
		Config:     cfg,
		WorkerType: "all",
	})

	// Without DB, pipeline should be nil
	if pool.Pipeline() != nil {
		t.Error("Pipeline() should be nil without DB")
	}
}

func TestPool_Repository(t *testing.T) {
	cfg := &config.Config{}

	pool, _ := NewPool(PoolConfig{
		Config:     cfg,
		WorkerType: "all",
	})

	// Without DB, repo should be nil
	if pool.Repository() != nil {
		t.Error("Repository() should be nil without DB")
	}
}

func TestPool_NATS(t *testing.T) {
	cfg := &config.Config{}

	pool, _ := NewPool(PoolConfig{
		Config:     cfg,
		WorkerType: "all",
	})

	// Without NATS client, should be nil
	if pool.NATS() != nil {
		t.Error("NATS() should be nil without NATS client")
	}
}

func TestWorkerType_Constants(t *testing.T) {
	tests := []struct {
		wt   WorkerType
		want string
	}{
		{WorkerIngestion, "ingestion"},
		{WorkerModeling, "modeling"},
		{WorkerPlanning, "planning"},
		{WorkerGeneration, "generation"},
		{WorkerMutation, "mutation"},
		{WorkerIntegration, "integration"},
		{WorkerAll, "all"},
	}

	for _, tt := range tests {
		if string(tt.wt) != tt.want {
			t.Errorf("WorkerType %v = %s, want %s", tt.wt, string(tt.wt), tt.want)
		}
	}
}

func TestPool_AddWorker(t *testing.T) {
	cfg := &config.Config{}

	pool := &Pool{
		cfg:        cfg,
		workerType: WorkerAll,
		workers:    make([]Worker, 0),
	}

	// Test adding each job type
	jobTypes := []jobs.JobType{
		jobs.JobTypeIngestion,
		jobs.JobTypeModeling,
		jobs.JobTypePlanning,
		jobs.JobTypeGeneration,
		jobs.JobTypeMutation,
		jobs.JobTypeIntegration,
	}

	for _, jt := range jobTypes {
		initialLen := len(pool.workers)
		pool.addWorker(jt)

		if len(pool.workers) != initialLen+1 {
			t.Errorf("addWorker(%s) did not add worker", jt)
		}
	}

	if len(pool.workers) != 6 {
		t.Errorf("len(workers) = %d, want 6", len(pool.workers))
	}
}
