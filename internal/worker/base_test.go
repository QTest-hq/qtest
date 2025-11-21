package worker

import (
	"strings"
	"testing"
	"time"

	"github.com/QTest-hq/qtest/internal/config"
	"github.com/QTest-hq/qtest/internal/jobs"
)

func TestNewBaseWorker(t *testing.T) {
	cfg := &config.Config{}

	base := NewBaseWorker(BaseWorkerConfig{
		Config:  cfg,
		JobType: jobs.JobTypeIngestion,
	})

	if base == nil {
		t.Fatal("base worker should not be nil")
	}

	if base.jobType != jobs.JobTypeIngestion {
		t.Errorf("jobType = %s, want ingestion", base.jobType)
	}

	// Should generate worker ID
	if base.workerID == "" {
		t.Error("workerID should not be empty")
	}

	if !strings.HasPrefix(base.workerID, "ingestion-") {
		t.Errorf("workerID should start with 'ingestion-', got %s", base.workerID)
	}
}

func TestNewBaseWorker_WithWorkerID(t *testing.T) {
	cfg := &config.Config{}

	base := NewBaseWorker(BaseWorkerConfig{
		Config:   cfg,
		WorkerID: "custom-worker-id",
		JobType:  jobs.JobTypeGeneration,
	})

	if base.workerID != "custom-worker-id" {
		t.Errorf("workerID = %s, want custom-worker-id", base.workerID)
	}
}

func TestBaseWorker_WorkerID(t *testing.T) {
	base := NewBaseWorker(BaseWorkerConfig{
		WorkerID: "test-worker",
		JobType:  jobs.JobTypeModeling,
	})

	if base.WorkerID() != "test-worker" {
		t.Errorf("WorkerID() = %s, want test-worker", base.WorkerID())
	}
}

func TestBaseWorker_JobType(t *testing.T) {
	base := NewBaseWorker(BaseWorkerConfig{
		JobType: jobs.JobTypePlanning,
	})

	if base.JobType() != jobs.JobTypePlanning {
		t.Errorf("JobType() = %s, want planning", base.JobType())
	}
}

func TestBaseWorker_SetPollPeriod(t *testing.T) {
	base := NewBaseWorker(BaseWorkerConfig{
		JobType: jobs.JobTypeIngestion,
	})

	// Default poll period
	if base.pollPeriod != 5*time.Second {
		t.Errorf("default pollPeriod = %v, want 5s", base.pollPeriod)
	}

	// Set custom poll period
	base.SetPollPeriod(10 * time.Second)

	if base.pollPeriod != 10*time.Second {
		t.Errorf("pollPeriod = %v, want 10s", base.pollPeriod)
	}
}

func TestBaseWorker_SetLockTime(t *testing.T) {
	base := NewBaseWorker(BaseWorkerConfig{
		JobType: jobs.JobTypeIngestion,
	})

	// Default lock time
	if base.lockTime != 5*time.Minute {
		t.Errorf("default lockTime = %v, want 5m", base.lockTime)
	}

	// Set custom lock time
	base.SetLockTime(10 * time.Minute)

	if base.lockTime != 10*time.Minute {
		t.Errorf("lockTime = %v, want 10m", base.lockTime)
	}
}

func TestBaseWorker_Repository(t *testing.T) {
	base := NewBaseWorker(BaseWorkerConfig{
		JobType: jobs.JobTypeIngestion,
	})

	// Without repository, should be nil
	if base.Repository() != nil {
		t.Error("Repository() should be nil without repo")
	}
}

func TestBaseWorker_Pipeline(t *testing.T) {
	base := NewBaseWorker(BaseWorkerConfig{
		JobType: jobs.JobTypeIngestion,
	})

	// Without pipeline, should be nil
	if base.Pipeline() != nil {
		t.Error("Pipeline() should be nil without pipeline")
	}
}

func TestBaseWorker_AllJobTypes(t *testing.T) {
	jobTypes := []jobs.JobType{
		jobs.JobTypeIngestion,
		jobs.JobTypeModeling,
		jobs.JobTypePlanning,
		jobs.JobTypeGeneration,
		jobs.JobTypeMutation,
		jobs.JobTypeIntegration,
	}

	for _, jt := range jobTypes {
		t.Run(string(jt), func(t *testing.T) {
			base := NewBaseWorker(BaseWorkerConfig{
				JobType: jt,
			})

			if base.JobType() != jt {
				t.Errorf("JobType() = %s, want %s", base.JobType(), jt)
			}

			// Worker ID should contain job type
			if !strings.Contains(base.WorkerID(), string(jt)) {
				t.Errorf("WorkerID() should contain %s, got %s", jt, base.WorkerID())
			}
		})
	}
}

func TestBaseWorkerConfig_Defaults(t *testing.T) {
	cfg := BaseWorkerConfig{
		JobType: jobs.JobTypeIngestion,
	}

	base := NewBaseWorker(cfg)

	// Check defaults are applied
	if base.pollPeriod != 5*time.Second {
		t.Errorf("default pollPeriod = %v, want 5s", base.pollPeriod)
	}
	if base.lockTime != 5*time.Minute {
		t.Errorf("default lockTime = %v, want 5m", base.lockTime)
	}
	if base.cfg != nil {
		t.Error("cfg should be nil when not provided")
	}
	if base.repo != nil {
		t.Error("repo should be nil when not provided")
	}
	if base.nats != nil {
		t.Error("nats should be nil when not provided")
	}
	if base.pipeline != nil {
		t.Error("pipeline should be nil when not provided")
	}
}
