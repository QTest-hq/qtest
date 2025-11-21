package nats

import (
	"testing"
	"time"
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

// =============================================================================
// SubjectForJobType Edge Cases
// =============================================================================

func TestSubjectForJobType_EmptyString(t *testing.T) {
	result := SubjectForJobType("")
	if result != "" {
		t.Errorf("SubjectForJobType('') = %s, want empty string", result)
	}
}

func TestSubjectForJobType_MixedCase(t *testing.T) {
	// Function is case-sensitive
	result := SubjectForJobType("INGESTION")
	if result != "" {
		t.Errorf("SubjectForJobType('INGESTION') = %s, want empty string (case-sensitive)", result)
	}
}

func TestSubjectForJobType_WithSpaces(t *testing.T) {
	result := SubjectForJobType(" ingestion ")
	if result != "" {
		t.Errorf("SubjectForJobType(' ingestion ') = %s, want empty string", result)
	}
}

func TestSubjectForJobType_PartialMatch(t *testing.T) {
	result := SubjectForJobType("ingest")
	if result != "" {
		t.Errorf("SubjectForJobType('ingest') = %s, want empty string", result)
	}
}

// =============================================================================
// ConsumerForJobType Edge Cases
// =============================================================================

func TestConsumerForJobType_EmptyString(t *testing.T) {
	result := ConsumerForJobType("")
	if result != "" {
		t.Errorf("ConsumerForJobType('') = %s, want empty string", result)
	}
}

func TestConsumerForJobType_MixedCase(t *testing.T) {
	result := ConsumerForJobType("GENERATION")
	if result != "" {
		t.Errorf("ConsumerForJobType('GENERATION') = %s, want empty string (case-sensitive)", result)
	}
}

func TestConsumerForJobType_WithSpaces(t *testing.T) {
	result := ConsumerForJobType(" generation ")
	if result != "" {
		t.Errorf("ConsumerForJobType(' generation ') = %s, want empty string", result)
	}
}

func TestConsumerForJobType_SimilarName(t *testing.T) {
	result := ConsumerForJobType("generate")
	if result != "" {
		t.Errorf("ConsumerForJobType('generate') = %s, want empty string", result)
	}
}

// =============================================================================
// DefaultStreamConfig Tests
// =============================================================================

func TestDefaultStreamConfig_Description(t *testing.T) {
	cfg := DefaultStreamConfig()
	if cfg.Description == "" {
		t.Error("DefaultStreamConfig().Description should not be empty")
	}
	if cfg.Description != "QTest job processing stream" {
		t.Errorf("Description = %s, want 'QTest job processing stream'", cfg.Description)
	}
}

func TestDefaultStreamConfig_MaxBytes(t *testing.T) {
	cfg := DefaultStreamConfig()
	expected := int64(1024 * 1024 * 500) // 500MB
	if cfg.MaxBytes != expected {
		t.Errorf("MaxBytes = %d, want %d (500MB)", cfg.MaxBytes, expected)
	}
}

func TestDefaultStreamConfig_MaxAge(t *testing.T) {
	cfg := DefaultStreamConfig()
	expected := 7 * 24 * time.Hour
	if cfg.MaxAge != expected {
		t.Errorf("MaxAge = %v, want %v (7 days)", cfg.MaxAge, expected)
	}
}
