package jobs

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewPipeline(t *testing.T) {
	// NewPipeline with nil dependencies (acceptable for unit testing)
	pipeline := NewPipeline(nil, nil)
	if pipeline == nil {
		t.Fatal("NewPipeline returned nil")
	}
}

func TestPipelineOptions_Fields(t *testing.T) {
	opts := PipelineOptions{
		Branch:     "main",
		MaxTests:   100,
		LLMTier:    2,
		TestLevels: []string{"unit", "api", "e2e"},
		CreatePR:   true,
	}

	if opts.Branch != "main" {
		t.Errorf("Branch = %s, want main", opts.Branch)
	}
	if opts.MaxTests != 100 {
		t.Errorf("MaxTests = %d, want 100", opts.MaxTests)
	}
	if opts.LLMTier != 2 {
		t.Errorf("LLMTier = %d, want 2", opts.LLMTier)
	}
	if len(opts.TestLevels) != 3 {
		t.Errorf("len(TestLevels) = %d, want 3", len(opts.TestLevels))
	}
	if !opts.CreatePR {
		t.Error("CreatePR should be true")
	}
}

func TestPipelineOptions_Defaults(t *testing.T) {
	opts := PipelineOptions{}

	if opts.Branch != "" {
		t.Errorf("default Branch = %s, want empty", opts.Branch)
	}
	if opts.MaxTests != 0 {
		t.Errorf("default MaxTests = %d, want 0", opts.MaxTests)
	}
	if opts.LLMTier != 0 {
		t.Errorf("default LLMTier = %d, want 0", opts.LLMTier)
	}
	if opts.TestLevels != nil {
		t.Error("default TestLevels should be nil")
	}
	if opts.CreatePR {
		t.Error("default CreatePR should be false")
	}
}

func TestJobStatusReport_Fields(t *testing.T) {
	parentJob := &Job{
		ID:     uuid.New(),
		Type:   JobTypeIngestion,
		Status: StatusCompleted,
	}

	childJobs := []*Job{
		{ID: uuid.New(), Type: JobTypeModeling, Status: StatusRunning},
		{ID: uuid.New(), Type: JobTypePlanning, Status: StatusPending},
	}

	report := JobStatusReport{
		Job:      parentJob,
		Children: childJobs,
	}

	if report.Job != parentJob {
		t.Error("Job should reference parent job")
	}
	if len(report.Children) != 2 {
		t.Errorf("len(Children) = %d, want 2", len(report.Children))
	}
	if report.Children[0].Type != JobTypeModeling {
		t.Errorf("Children[0].Type = %s, want modeling", report.Children[0].Type)
	}
}

func TestJobStatusReport_EmptyChildren(t *testing.T) {
	job := &Job{
		ID:     uuid.New(),
		Type:   JobTypeGeneration,
		Status: StatusPending,
	}

	report := JobStatusReport{
		Job:      job,
		Children: nil,
	}

	if report.Job == nil {
		t.Error("Job should not be nil")
	}
	if report.Children != nil {
		t.Error("Children should be nil")
	}
}

func TestJobStatusReport_Defaults(t *testing.T) {
	report := JobStatusReport{}

	if report.Job != nil {
		t.Error("default Job should be nil")
	}
	if report.Children != nil {
		t.Error("default Children should be nil")
	}
}

func TestPipelineOptions_TestLevels(t *testing.T) {
	tests := []struct {
		name   string
		levels []string
	}{
		{"unit only", []string{"unit"}},
		{"api only", []string{"api"}},
		{"e2e only", []string{"e2e"}},
		{"unit and api", []string{"unit", "api"}},
		{"all levels", []string{"unit", "api", "e2e"}},
		{"empty", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := PipelineOptions{
				TestLevels: tt.levels,
			}
			if len(opts.TestLevels) != len(tt.levels) {
				t.Errorf("len(TestLevels) = %d, want %d", len(opts.TestLevels), len(tt.levels))
			}
		})
	}
}

func TestPipelineOptions_LLMTiers(t *testing.T) {
	tests := []struct {
		name string
		tier int
	}{
		{"fast", 1},
		{"balanced", 2},
		{"thorough", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := PipelineOptions{
				LLMTier: tt.tier,
			}
			if opts.LLMTier != tt.tier {
				t.Errorf("LLMTier = %d, want %d", opts.LLMTier, tt.tier)
			}
		})
	}
}
