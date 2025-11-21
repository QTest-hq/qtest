package nats

import (
	"testing"
)

func TestSubjectForJobType(t *testing.T) {
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

func TestConsumerForJobType(t *testing.T) {
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

func TestDefaultStreamConfig(t *testing.T) {
	cfg := DefaultStreamConfig()

	if cfg.Name != StreamJobs {
		t.Errorf("Name = %s, want %s", cfg.Name, StreamJobs)
	}
	if len(cfg.Subjects) != 1 || cfg.Subjects[0] != SubjectJobsAll {
		t.Errorf("Subjects = %v, want [%s]", cfg.Subjects, SubjectJobsAll)
	}
	if cfg.MaxMsgs != 100000 {
		t.Errorf("MaxMsgs = %d, want 100000", cfg.MaxMsgs)
	}
	if cfg.Replicas != 1 {
		t.Errorf("Replicas = %d, want 1", cfg.Replicas)
	}
}

func TestConstants(t *testing.T) {
	// Verify constant values are set correctly
	if StreamJobs != "QTEST_JOBS" {
		t.Errorf("StreamJobs = %s, want QTEST_JOBS", StreamJobs)
	}
	if SubjectJobsAll != "jobs.>" {
		t.Errorf("SubjectJobsAll = %s, want jobs.>", SubjectJobsAll)
	}

	// Verify subject patterns
	subjects := []string{
		SubjectJobIngestion,
		SubjectJobModeling,
		SubjectJobPlanning,
		SubjectJobGeneration,
		SubjectJobMutation,
		SubjectJobIntegration,
	}
	for _, s := range subjects {
		if len(s) < 5 || s[:5] != "jobs." {
			t.Errorf("subject %s should start with 'jobs.'", s)
		}
	}
}
