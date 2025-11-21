package worker

import (
	"testing"

	"github.com/QTest-hq/qtest/internal/config"
	"github.com/QTest-hq/qtest/internal/jobs"
)

func TestIngestionWorker_Name(t *testing.T) {
	base := NewBaseWorker(BaseWorkerConfig{
		JobType: jobs.JobTypeIngestion,
	})
	worker := NewIngestionWorker(base, nil)

	if worker.Name() != "ingestion" {
		t.Errorf("Name() = %s, want ingestion", worker.Name())
	}
}

func TestModelingWorker_Name(t *testing.T) {
	base := NewBaseWorker(BaseWorkerConfig{
		JobType: jobs.JobTypeModeling,
	})
	worker := NewModelingWorker(base, nil)

	if worker.Name() != "modeling" {
		t.Errorf("Name() = %s, want modeling", worker.Name())
	}
}

func TestPlanningWorker_Name(t *testing.T) {
	base := NewBaseWorker(BaseWorkerConfig{
		JobType: jobs.JobTypePlanning,
	})
	worker := NewPlanningWorker(base, nil)

	if worker.Name() != "planning" {
		t.Errorf("Name() = %s, want planning", worker.Name())
	}
}

func TestGenerationWorker_Name(t *testing.T) {
	cfg := &config.Config{}
	base := NewBaseWorker(BaseWorkerConfig{
		Config:  cfg,
		JobType: jobs.JobTypeGeneration,
	})
	worker := NewGenerationWorker(base, cfg, nil, nil)

	if worker.Name() != "generation" {
		t.Errorf("Name() = %s, want generation", worker.Name())
	}
}

func TestMutationWorker_Name(t *testing.T) {
	base := NewBaseWorker(BaseWorkerConfig{
		JobType: jobs.JobTypeMutation,
	})
	worker := NewMutationWorker(base, nil, nil)

	if worker.Name() != "mutation" {
		t.Errorf("Name() = %s, want mutation", worker.Name())
	}
}

func TestIntegrationWorker_Name(t *testing.T) {
	base := NewBaseWorker(BaseWorkerConfig{
		JobType: jobs.JobTypeIntegration,
	})
	worker := NewIntegrationWorker(base, nil)

	if worker.Name() != "integration" {
		t.Errorf("Name() = %s, want integration", worker.Name())
	}
}

func TestWorker_Interface(t *testing.T) {
	// Verify all workers implement the Worker interface
	cfg := &config.Config{}

	workers := []Worker{
		NewIngestionWorker(NewBaseWorker(BaseWorkerConfig{JobType: jobs.JobTypeIngestion}), nil),
		NewModelingWorker(NewBaseWorker(BaseWorkerConfig{JobType: jobs.JobTypeModeling}), nil),
		NewPlanningWorker(NewBaseWorker(BaseWorkerConfig{JobType: jobs.JobTypePlanning}), nil),
		NewGenerationWorker(NewBaseWorker(BaseWorkerConfig{Config: cfg, JobType: jobs.JobTypeGeneration}), cfg, nil, nil),
		NewMutationWorker(NewBaseWorker(BaseWorkerConfig{JobType: jobs.JobTypeMutation}), nil, nil),
		NewIntegrationWorker(NewBaseWorker(BaseWorkerConfig{JobType: jobs.JobTypeIntegration}), nil),
	}

	expectedNames := []string{"ingestion", "modeling", "planning", "generation", "mutation", "integration"}

	for i, w := range workers {
		if w.Name() != expectedNames[i] {
			t.Errorf("worker[%d].Name() = %s, want %s", i, w.Name(), expectedNames[i])
		}
	}
}

func TestWorker_BaseWorkerEmbedding(t *testing.T) {
	base := NewBaseWorker(BaseWorkerConfig{
		WorkerID: "test-ingestion-1",
		JobType:  jobs.JobTypeIngestion,
	})
	worker := NewIngestionWorker(base, nil)

	// Should have access to base worker methods
	if worker.WorkerID() != "test-ingestion-1" {
		t.Errorf("WorkerID() = %s, want test-ingestion-1", worker.WorkerID())
	}

	if worker.JobType() != jobs.JobTypeIngestion {
		t.Errorf("JobType() = %s, want ingestion", worker.JobType())
	}
}

func TestIngestionWorker_PayloadParsing(t *testing.T) {
	// Test that IngestionPayload can be properly parsed
	payload := jobs.IngestionPayload{
		RepositoryURL: "https://github.com/test/repo",
		Branch:        "main",
		CommitHash:    "abc123",
	}

	job, err := jobs.NewJob(jobs.JobTypeIngestion, payload)
	if err != nil {
		t.Fatalf("NewJob failed: %v", err)
	}

	var parsed jobs.IngestionPayload
	if err := job.GetPayload(&parsed); err != nil {
		t.Fatalf("GetPayload failed: %v", err)
	}

	if parsed.RepositoryURL != payload.RepositoryURL {
		t.Errorf("RepositoryURL mismatch")
	}
	if parsed.Branch != payload.Branch {
		t.Errorf("Branch mismatch")
	}
}

func TestModelingWorker_PayloadParsing(t *testing.T) {
	payload := jobs.ModelingPayload{
		WorkspacePath: "/tmp/workspace",
		IncludePaths:  []string{"src/"},
		ExcludePaths:  []string{"vendor/"},
	}

	job, err := jobs.NewJob(jobs.JobTypeModeling, payload)
	if err != nil {
		t.Fatalf("NewJob failed: %v", err)
	}

	var parsed jobs.ModelingPayload
	if err := job.GetPayload(&parsed); err != nil {
		t.Fatalf("GetPayload failed: %v", err)
	}

	if parsed.WorkspacePath != payload.WorkspacePath {
		t.Errorf("WorkspacePath mismatch")
	}
	if len(parsed.IncludePaths) != 1 {
		t.Errorf("len(IncludePaths) = %d, want 1", len(parsed.IncludePaths))
	}
}

func TestPlanningWorker_PayloadParsing(t *testing.T) {
	payload := jobs.PlanningPayload{
		MaxTests:   100,
		TestLevels: []string{"unit", "api"},
	}

	job, err := jobs.NewJob(jobs.JobTypePlanning, payload)
	if err != nil {
		t.Fatalf("NewJob failed: %v", err)
	}

	var parsed jobs.PlanningPayload
	if err := job.GetPayload(&parsed); err != nil {
		t.Fatalf("GetPayload failed: %v", err)
	}

	if parsed.MaxTests != 100 {
		t.Errorf("MaxTests = %d, want 100", parsed.MaxTests)
	}
}

func TestGenerationWorker_PayloadParsing(t *testing.T) {
	payload := jobs.GenerationPayload{
		LLMTier:   2,
		IntentIDs: []string{"intent-1", "intent-2"},
	}

	job, err := jobs.NewJob(jobs.JobTypeGeneration, payload)
	if err != nil {
		t.Fatalf("NewJob failed: %v", err)
	}

	var parsed jobs.GenerationPayload
	if err := job.GetPayload(&parsed); err != nil {
		t.Fatalf("GetPayload failed: %v", err)
	}

	if parsed.LLMTier != 2 {
		t.Errorf("LLMTier = %d, want 2", parsed.LLMTier)
	}
	if len(parsed.IntentIDs) != 2 {
		t.Errorf("len(IntentIDs) = %d, want 2", len(parsed.IntentIDs))
	}
}

func TestMutationWorker_PayloadParsing(t *testing.T) {
	payload := jobs.MutationPayload{
		TestFilePath:   "foo_test.go",
		SourceFilePath: "foo.go",
	}

	job, err := jobs.NewJob(jobs.JobTypeMutation, payload)
	if err != nil {
		t.Fatalf("NewJob failed: %v", err)
	}

	var parsed jobs.MutationPayload
	if err := job.GetPayload(&parsed); err != nil {
		t.Fatalf("GetPayload failed: %v", err)
	}

	if parsed.TestFilePath != "foo_test.go" {
		t.Errorf("TestFilePath = %s, want foo_test.go", parsed.TestFilePath)
	}
}

func TestIntegrationWorker_PayloadParsing(t *testing.T) {
	payload := jobs.IntegrationPayload{
		TestFilePaths: []string{"test1.go", "test2.go"},
		CreatePR:      true,
		TargetBranch:  "main",
	}

	job, err := jobs.NewJob(jobs.JobTypeIntegration, payload)
	if err != nil {
		t.Fatalf("NewJob failed: %v", err)
	}

	var parsed jobs.IntegrationPayload
	if err := job.GetPayload(&parsed); err != nil {
		t.Fatalf("GetPayload failed: %v", err)
	}

	if !parsed.CreatePR {
		t.Error("CreatePR should be true")
	}
	if parsed.TargetBranch != "main" {
		t.Errorf("TargetBranch = %s, want main", parsed.TargetBranch)
	}
}

// Test helper functions
func TestExtractRepoInfo(t *testing.T) {
	tests := []struct {
		url       string
		wantName  string
		wantOwner string
	}{
		{
			url:       "https://github.com/user/myrepo.git",
			wantName:  "myrepo",
			wantOwner: "user",
		},
		{
			url:       "https://github.com/user/myrepo",
			wantName:  "myrepo",
			wantOwner: "user",
		},
		{
			url:       "git@github.com:user/myrepo.git",
			wantName:  "myrepo",
			wantOwner: "user",
		},
		{
			url:       "git@github.com:user/myrepo",
			wantName:  "myrepo",
			wantOwner: "user",
		},
		{
			url:       "https://gitlab.com/group/project.git",
			wantName:  "project",
			wantOwner: "group",
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			name, owner := extractRepoInfo(tt.url)
			if name != tt.wantName {
				t.Errorf("extractRepoInfo(%s) name = %s, want %s", tt.url, name, tt.wantName)
			}
			if owner != tt.wantOwner {
				t.Errorf("extractRepoInfo(%s) owner = %s, want %s", tt.url, owner, tt.wantOwner)
			}
		})
	}
}

func TestIsHTTPHandler(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"handleRequest", true},
		{"HandleUser", true},
		{"userHandler", true},
		{"UserHandler", true},
		{"serveHTTP", true},
		{"ServeHTTP", true},
		{"apiEndpoint", true},
		{"getUser", false},
		{"processData", false},
		{"main", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isHTTPHandler(tt.name); got != tt.want {
				t.Errorf("isHTTPHandler(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestDetectFramework(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"foo_test.go", "go"},
		{"test_foo.py", "pytest"},
		{"foo.test.ts", "jest"},
		{"foo.test.js", "jest"},
		{"foo.spec.ts", "jest"},
		{"foo.txt", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := detectFramework(tt.path); got != tt.want {
				t.Errorf("detectFramework(%s) = %s, want %s", tt.path, got, tt.want)
			}
		})
	}
}
