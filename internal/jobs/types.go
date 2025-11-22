// Package jobs defines job types and payloads for async processing
package jobs

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// JobType represents the type of async job
type JobType string

const (
	JobTypeIngestion   JobType = "ingestion"
	JobTypeModeling    JobType = "modeling"
	JobTypePlanning    JobType = "planning"
	JobTypeGeneration  JobType = "generation"
	JobTypeValidation  JobType = "validation"
	JobTypeMutation    JobType = "mutation"
	JobTypeIntegration JobType = "integration"
)

// JobStatus represents the current state of a job
type JobStatus string

const (
	StatusPending   JobStatus = "pending"
	StatusRunning   JobStatus = "running"
	StatusCompleted JobStatus = "completed"
	StatusFailed    JobStatus = "failed"
	StatusRetrying  JobStatus = "retrying"
	StatusCancelled JobStatus = "cancelled"
)

// Job represents an async job in the system
type Job struct {
	ID              uuid.UUID       `json:"id" db:"id"`
	Type            JobType         `json:"type" db:"type"`
	Status          JobStatus       `json:"status" db:"status"`
	Priority        int             `json:"priority" db:"priority"`
	RepositoryID    *uuid.UUID      `json:"repository_id,omitempty" db:"repository_id"`
	GenerationRunID *uuid.UUID      `json:"generation_run_id,omitempty" db:"generation_run_id"`
	ParentJobID     *uuid.UUID      `json:"parent_job_id,omitempty" db:"parent_job_id"`
	Payload         json.RawMessage  `json:"payload" db:"payload"`
	Result          *json.RawMessage `json:"result,omitempty" db:"result"`
	ErrorMessage    *string          `json:"error_message,omitempty" db:"error_message"`
	ErrorDetails    *json.RawMessage `json:"error_details,omitempty" db:"error_details"`
	RetryCount      int             `json:"retry_count" db:"retry_count"`
	MaxRetries      int             `json:"max_retries" db:"max_retries"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at" db:"updated_at"`
	StartedAt       *time.Time      `json:"started_at,omitempty" db:"started_at"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty" db:"completed_at"`
	LockedUntil     *time.Time      `json:"locked_until,omitempty" db:"locked_until"`
	WorkerID        *string         `json:"worker_id,omitempty" db:"worker_id"`
}

// IngestionPayload is the payload for ingestion jobs
type IngestionPayload struct {
	RepositoryURL string `json:"repository_url"`
	Branch        string `json:"branch,omitempty"`
	CommitHash    string `json:"commit_hash,omitempty"`
	WorkspacePath string `json:"workspace_path,omitempty"`
	// Pipeline options (propagated through chain)
	MaxTests    int  `json:"max_tests,omitempty"`
	LLMTier     int  `json:"llm_tier,omitempty"`
	RunMutation bool `json:"run_mutation,omitempty"`
	CreatePR    bool `json:"create_pr,omitempty"`
}

// ModelingPayload is the payload for modeling jobs
type ModelingPayload struct {
	RepositoryID  uuid.UUID `json:"repository_id"`
	WorkspacePath string    `json:"workspace_path"`
	IncludePaths  []string  `json:"include_paths,omitempty"`
	ExcludePaths  []string  `json:"exclude_paths,omitempty"`
	// Pipeline options (propagated through chain)
	MaxTests    int  `json:"max_tests,omitempty"`
	LLMTier     int  `json:"llm_tier,omitempty"`
	RunMutation bool `json:"run_mutation,omitempty"`
	CreatePR    bool `json:"create_pr,omitempty"`
}

// PlanningPayload is the payload for planning jobs
type PlanningPayload struct {
	RepositoryID uuid.UUID `json:"repository_id"`
	ModelID      uuid.UUID `json:"model_id"`
	MaxTests     int       `json:"max_tests,omitempty"`
	TestLevels   []string  `json:"test_levels,omitempty"` // "unit", "api", "e2e"
	// Pipeline options (propagated through chain)
	LLMTier     int  `json:"llm_tier,omitempty"`
	RunMutation bool `json:"run_mutation,omitempty"`
	CreatePR    bool `json:"create_pr,omitempty"`
}

// GenerationPayload is the payload for generation jobs
type GenerationPayload struct {
	RepositoryID    uuid.UUID `json:"repository_id"`
	GenerationRunID uuid.UUID `json:"generation_run_id"`
	PlanID          uuid.UUID `json:"plan_id"`
	IntentIDs       []string  `json:"intent_ids,omitempty"` // Specific intents to generate
	LLMTier         int       `json:"llm_tier,omitempty"`   // 1=fast, 2=balanced, 3=thorough
	RunMutation     bool      `json:"run_mutation"`         // Whether to run mutation testing
	CreatePR        bool      `json:"create_pr"`            // Whether to create a PR at the end
}

// MutationPayload is the payload for mutation testing jobs
type MutationPayload struct {
	RepositoryID    uuid.UUID `json:"repository_id"`
	GenerationRunID uuid.UUID `json:"generation_run_id"`
	TestFilePath    string    `json:"test_file_path"`
	SourceFilePath  string    `json:"source_file_path"`
}

// ValidationPayload is the payload for validation jobs
type ValidationPayload struct {
	RepositoryID    uuid.UUID `json:"repository_id"`
	GenerationRunID uuid.UUID `json:"generation_run_id"`
	TestIDs         []string  `json:"test_ids"`         // IDs of GeneratedTest records to validate
	TestFilePaths   []string  `json:"test_file_paths"`  // Paths to test files
	WorkspacePath   string    `json:"workspace_path"`   // Repository workspace path
	Language        string    `json:"language"`         // Programming language
	AutoFix         bool      `json:"auto_fix"`         // Whether to attempt auto-fixing
	MaxFixAttempts  int       `json:"max_fix_attempts"` // Max LLM fix attempts (default 3)
	// Pipeline continuation
	RunMutation bool `json:"run_mutation"` // Whether to run mutation testing after validation
	CreatePR    bool `json:"create_pr"`    // Whether to create a PR at the end
}

// IntegrationPayload is the payload for integration jobs
type IntegrationPayload struct {
	RepositoryID    uuid.UUID `json:"repository_id"`
	GenerationRunID uuid.UUID `json:"generation_run_id"`
	TestFilePaths   []string  `json:"test_file_paths"`
	TargetBranch    string    `json:"target_branch,omitempty"`
	CreatePR        bool      `json:"create_pr"`
}

// IngestionResult is the result of an ingestion job
type IngestionResult struct {
	RepositoryID  uuid.UUID `json:"repository_id"`
	WorkspacePath string    `json:"workspace_path"`
	FileCount     int       `json:"file_count"`
	Language      string    `json:"language"`
	Framework     string    `json:"framework,omitempty"`
}

// ModelingResult is the result of a modeling job
type ModelingResult struct {
	ModelID       uuid.UUID `json:"model_id"`
	FileCount     int       `json:"file_count"`
	FunctionCount int       `json:"function_count"`
	EndpointCount int       `json:"endpoint_count"`
}

// PlanningResult is the result of a planning job
type PlanningResult struct {
	PlanID     uuid.UUID `json:"plan_id"`
	TotalTests int       `json:"total_tests"`
	UnitTests  int       `json:"unit_tests"`
	APITests   int       `json:"api_tests"`
	E2ETests   int       `json:"e2e_tests"`
}

// GenerationResult is the result of a generation job
type GenerationResult struct {
	TestsGenerated int      `json:"tests_generated"`
	TestFilePaths  []string `json:"test_file_paths"`
	FailedIntents  []string `json:"failed_intents,omitempty"`
}

// MutationResult is the result of a mutation testing job
type MutationResult struct {
	MutantsTotal   int     `json:"mutants_total"`
	MutantsKilled  int     `json:"mutants_killed"`
	MutantsLived   int     `json:"mutants_lived"`
	MutationScore  float64 `json:"mutation_score"`
	ReportFilePath string  `json:"report_file_path,omitempty"`
}

// ValidationResult is the result of a validation job
type ValidationResult struct {
	TotalTests     int                 `json:"total_tests"`
	PassedTests    int                 `json:"passed_tests"`
	FailedTests    int                 `json:"failed_tests"`
	FixedTests     int                 `json:"fixed_tests"`
	ValidationTime time.Duration       `json:"validation_time"`
	Results        []TestValidationRes `json:"results"`
}

// TestValidationRes holds validation result for a single test
type TestValidationRes struct {
	TestID        string `json:"test_id"`
	TestFile      string `json:"test_file"`
	Status        string `json:"status"` // "validated", "compile_error", "test_failure", "fixed"
	Output        string `json:"output,omitempty"`
	ErrorMessage  string `json:"error_message,omitempty"`
	FixAttempts   int    `json:"fix_attempts"`
	ValidationMs  int64  `json:"validation_ms"`
}

// IntegrationResult is the result of an integration job
type IntegrationResult struct {
	FilesIntegrated int    `json:"files_integrated"`
	PRNumber        int    `json:"pr_number,omitempty"`
	PRURL           string `json:"pr_url,omitempty"`
	BranchName      string `json:"branch_name,omitempty"`
}

// NewJob creates a new job with defaults
func NewJob(jobType JobType, payload interface{}) (*Job, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &Job{
		ID:         uuid.New(),
		Type:       jobType,
		Status:     StatusPending,
		Priority:   0,
		Payload:    payloadBytes,
		RetryCount: 0,
		MaxRetries: 3,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

// SetPayload marshals and sets the payload
func (j *Job) SetPayload(payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	j.Payload = data
	return nil
}

// GetPayload unmarshals the payload into the provided struct
func (j *Job) GetPayload(v interface{}) error {
	return json.Unmarshal(j.Payload, v)
}

// SetResult marshals and sets the result
func (j *Job) SetResult(result interface{}) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	raw := json.RawMessage(data)
	j.Result = &raw
	return nil
}

// GetResult unmarshals the result into the provided struct
func (j *Job) GetResult(v interface{}) error {
	if j.Result == nil {
		return nil
	}
	return json.Unmarshal(*j.Result, v)
}

// CanRetry returns true if the job can be retried
func (j *Job) CanRetry() bool {
	return j.RetryCount < j.MaxRetries
}

// JobMessage is the message sent via NATS for job notifications
type JobMessage struct {
	JobID    uuid.UUID `json:"job_id"`
	Type     JobType   `json:"type"`
	Priority int       `json:"priority"`
}

// Encode serializes the job message to JSON
func (m *JobMessage) Encode() ([]byte, error) {
	return json.Marshal(m)
}

// DecodeJobMessage deserializes a job message from JSON
func DecodeJobMessage(data []byte) (*JobMessage, error) {
	var m JobMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
