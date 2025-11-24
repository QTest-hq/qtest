package worker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/QTest-hq/qtest/internal/config"
	"github.com/QTest-hq/qtest/internal/flow"
	"github.com/QTest-hq/qtest/internal/jobs"
)

// TestE2EDiscoveryWorker_Name tests the Name method
func TestE2EDiscoveryWorker_Name(t *testing.T) {
	cfg := &config.Config{}
	baseCfg := BaseWorkerConfig{
		Config:  cfg,
		JobType: jobs.JobTypeE2EDiscovery,
	}
	base := NewBaseWorker(baseCfg)
	worker := NewE2EDiscoveryWorker(base, cfg, nil)

	if got := worker.Name(); got != "e2e_discovery" {
		t.Errorf("Name() = %s, want e2e_discovery", got)
	}
}

// TestE2EGenerateWorker_Name tests the Name method
func TestE2EGenerateWorker_Name(t *testing.T) {
	cfg := &config.Config{}
	baseCfg := BaseWorkerConfig{
		Config:  cfg,
		JobType: jobs.JobTypeE2EGenerate,
	}
	base := NewBaseWorker(baseCfg)
	worker := NewE2EGenerateWorker(base, cfg, nil)

	if got := worker.Name(); got != "e2e_generation" {
		t.Errorf("Name() = %s, want e2e_generation", got)
	}
}

// TestE2ERunWorker_Name tests the Name method
func TestE2ERunWorker_Name(t *testing.T) {
	cfg := &config.Config{}
	baseCfg := BaseWorkerConfig{
		Config:  cfg,
		JobType: jobs.JobTypeE2ERun,
	}
	base := NewBaseWorker(baseCfg)
	worker := NewE2ERunWorker(base, cfg)

	if got := worker.Name(); got != "e2e_run" {
		t.Errorf("Name() = %s, want e2e_run", got)
	}
}

// TestNewE2EDiscoveryWorker tests worker creation
func TestNewE2EDiscoveryWorker(t *testing.T) {
	cfg := &config.Config{}
	baseCfg := BaseWorkerConfig{
		Config:  cfg,
		JobType: jobs.JobTypeE2EDiscovery,
	}
	base := NewBaseWorker(baseCfg)

	worker := NewE2EDiscoveryWorker(base, cfg, nil)

	if worker == nil {
		t.Fatal("NewE2EDiscoveryWorker returned nil")
	}
	if worker.cfg != cfg {
		t.Error("config not set correctly")
	}
	if worker.BaseWorker != base {
		t.Error("base worker not set correctly")
	}
}

// TestNewE2EGenerateWorker tests worker creation
func TestNewE2EGenerateWorker(t *testing.T) {
	cfg := &config.Config{}
	baseCfg := BaseWorkerConfig{
		Config:  cfg,
		JobType: jobs.JobTypeE2EGenerate,
	}
	base := NewBaseWorker(baseCfg)

	worker := NewE2EGenerateWorker(base, cfg, nil)

	if worker == nil {
		t.Fatal("NewE2EGenerateWorker returned nil")
	}
	if worker.cfg != cfg {
		t.Error("config not set correctly")
	}
}

// TestNewE2ERunWorker tests worker creation
func TestNewE2ERunWorker(t *testing.T) {
	cfg := &config.Config{}
	baseCfg := BaseWorkerConfig{
		Config:  cfg,
		JobType: jobs.JobTypeE2ERun,
	}
	base := NewBaseWorker(baseCfg)

	worker := NewE2ERunWorker(base, cfg)

	if worker == nil {
		t.Fatal("NewE2ERunWorker returned nil")
	}
	if worker.cfg != cfg {
		t.Error("config not set correctly")
	}
}

// TestE2EDiscoveryPayload_Parsing tests payload parsing
func TestE2EDiscoveryPayload_Parsing(t *testing.T) {
	payload := jobs.E2EDiscoveryPayload{
		URL:           "https://example.com",
		MaxPages:      10,
		PlaywrightURL: "http://localhost:3000",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	var parsed jobs.E2EDiscoveryPayload
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if parsed.URL != payload.URL {
		t.Errorf("URL = %s, want %s", parsed.URL, payload.URL)
	}
	if parsed.MaxPages != payload.MaxPages {
		t.Errorf("MaxPages = %d, want %d", parsed.MaxPages, payload.MaxPages)
	}
	if parsed.PlaywrightURL != payload.PlaywrightURL {
		t.Errorf("PlaywrightURL = %s, want %s", parsed.PlaywrightURL, payload.PlaywrightURL)
	}
}

// TestE2EGeneratePayload_Parsing tests payload parsing
func TestE2EGeneratePayload_Parsing(t *testing.T) {
	flows := []flow.Flow{{Name: "test flow", Type: "authentication"}}
	flowsJSON, _ := json.Marshal(flows)

	payload := jobs.E2EGeneratePayload{
		FlowID:    "flow-123",
		Flows:     flowsJSON,
		Framework: "playwright",
		Language:  "typescript",
		BaseURL:   "https://example.com",
		Enhance:   true,
		OutputDir: "/tmp/tests",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	var parsed jobs.E2EGeneratePayload
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if parsed.FlowID != payload.FlowID {
		t.Errorf("FlowID = %s, want %s", parsed.FlowID, payload.FlowID)
	}
	if parsed.Framework != payload.Framework {
		t.Errorf("Framework = %s, want %s", parsed.Framework, payload.Framework)
	}
	if parsed.Language != payload.Language {
		t.Errorf("Language = %s, want %s", parsed.Language, payload.Language)
	}
	if parsed.Enhance != payload.Enhance {
		t.Errorf("Enhance = %v, want %v", parsed.Enhance, payload.Enhance)
	}
}

// TestE2ERunPayload_Parsing tests payload parsing
func TestE2ERunPayload_Parsing(t *testing.T) {
	payload := jobs.E2ERunPayload{
		TestDir:  "./tests",
		Pattern:  "*.spec.ts",
		Browser:  "chromium",
		Headless: true,
		Workers:  4,
		Retries:  2,
		BaseURL:  "https://example.com",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	var parsed jobs.E2ERunPayload
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if parsed.TestDir != payload.TestDir {
		t.Errorf("TestDir = %s, want %s", parsed.TestDir, payload.TestDir)
	}
	if parsed.Pattern != payload.Pattern {
		t.Errorf("Pattern = %s, want %s", parsed.Pattern, payload.Pattern)
	}
	if parsed.Browser != payload.Browser {
		t.Errorf("Browser = %s, want %s", parsed.Browser, payload.Browser)
	}
	if parsed.Headless != payload.Headless {
		t.Errorf("Headless = %v, want %v", parsed.Headless, payload.Headless)
	}
	if parsed.Workers != payload.Workers {
		t.Errorf("Workers = %d, want %d", parsed.Workers, payload.Workers)
	}
}

// TestE2EDiscoveryResult_Serialization tests result serialization
func TestE2EDiscoveryResult_Serialization(t *testing.T) {
	flows := []flow.Flow{{Name: "login", Type: "authentication"}}
	flowsJSON, _ := json.Marshal(flows)

	result := jobs.E2EDiscoveryResult{
		FlowsDiscovered: 1,
		Flows:           flowsJSON,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	var parsed jobs.E2EDiscoveryResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if parsed.FlowsDiscovered != result.FlowsDiscovered {
		t.Errorf("FlowsDiscovered = %d, want %d", parsed.FlowsDiscovered, result.FlowsDiscovered)
	}
}

// TestE2EGenerateResult_Serialization tests result serialization
func TestE2EGenerateResult_Serialization(t *testing.T) {
	result := jobs.E2EGenerateResult{
		TestsGenerated: 5,
		StepsCount:     20,
		OutputDir:      "/tmp/tests",
		Files:          []string{"login.spec.ts", "checkout.spec.ts"},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	var parsed jobs.E2EGenerateResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if parsed.TestsGenerated != result.TestsGenerated {
		t.Errorf("TestsGenerated = %d, want %d", parsed.TestsGenerated, result.TestsGenerated)
	}
	if parsed.StepsCount != result.StepsCount {
		t.Errorf("StepsCount = %d, want %d", parsed.StepsCount, result.StepsCount)
	}
	if len(parsed.Files) != len(result.Files) {
		t.Errorf("Files length = %d, want %d", len(parsed.Files), len(result.Files))
	}
}

// TestE2ERunResult_Serialization tests result serialization
func TestE2ERunResult_Serialization(t *testing.T) {
	result := jobs.E2ERunResult{
		TotalTests: 10,
		Passed:     8,
		Failed:     1,
		Skipped:    1,
		Duration:   "5m30s",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	var parsed jobs.E2ERunResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if parsed.TotalTests != result.TotalTests {
		t.Errorf("TotalTests = %d, want %d", parsed.TotalTests, result.TotalTests)
	}
	if parsed.Passed != result.Passed {
		t.Errorf("Passed = %d, want %d", parsed.Passed, result.Passed)
	}
	if parsed.Failed != result.Failed {
		t.Errorf("Failed = %d, want %d", parsed.Failed, result.Failed)
	}
	if parsed.Duration != result.Duration {
		t.Errorf("Duration = %s, want %s", parsed.Duration, result.Duration)
	}
}

// TestE2ERunPayload_Defaults tests default value handling
func TestE2ERunPayload_Defaults(t *testing.T) {
	// Test that empty values are handled correctly
	payload := jobs.E2ERunPayload{
		TestDir: "./tests",
	}

	// Verify defaults are zero values
	if payload.Browser != "" {
		t.Errorf("Browser should be empty, got %s", payload.Browser)
	}
	if payload.Headless != false {
		t.Error("Headless should default to false")
	}
	if payload.Workers != 0 {
		t.Errorf("Workers should be 0, got %d", payload.Workers)
	}
	if payload.Retries != 0 {
		t.Errorf("Retries should be 0, got %d", payload.Retries)
	}
}

// TestE2EJobTypes tests job type constants
func TestE2EJobTypes(t *testing.T) {
	tests := []struct {
		jobType jobs.JobType
		want    string
	}{
		{jobs.JobTypeE2EDiscovery, "e2e_discovery"},
		{jobs.JobTypeE2EGenerate, "e2e_generation"},
		{jobs.JobTypeE2ERun, "e2e_run"},
	}

	for _, tt := range tests {
		if string(tt.jobType) != tt.want {
			t.Errorf("JobType %v = %s, want %s", tt.jobType, string(tt.jobType), tt.want)
		}
	}
}

// TestE2EGenerateWorker_OutputDirectory tests output directory creation
func TestE2EGenerateWorker_OutputDirectory(t *testing.T) {
	tmpBase := filepath.Join(os.TempDir(), "qtest-e2e-test")
	defer os.RemoveAll(tmpBase)

	// Create nested directory structure
	outputDir := filepath.Join(tmpBase, "subdir", "tests")

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output directory: %v", err)
	}

	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		t.Error("output directory was not created")
	}
}

// TestE2EWorkerTypes_InPool tests that E2E workers are registered in pool
func TestE2EWorkerTypes_InPool(t *testing.T) {
	cfg := &config.Config{}

	// Test that E2E worker types are recognized
	workerTypes := []string{
		"e2e_discovery",
		"e2e_generation",
		"e2e_run",
	}

	for _, wt := range workerTypes {
		pool, err := NewPool(PoolConfig{
			Config:     cfg,
			WorkerType: wt,
		})
		if err != nil {
			t.Errorf("NewPool(%s) failed: %v", wt, err)
			continue
		}
		if len(pool.workers) != 1 {
			t.Errorf("NewPool(%s): len(workers) = %d, want 1", wt, len(pool.workers))
		}
	}
}

// TestE2EFlowConversion tests flow type serialization
func TestE2EFlowConversion(t *testing.T) {
	// Create a basic flow without complex steps
	f := flow.Flow{
		Name:        "Login Flow",
		Type:        "authentication",
		Description: "User login flow",
		StartURL:    "https://example.com/login",
	}

	// Serialize to JSON
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("failed to marshal flow: %v", err)
	}

	// Deserialize back
	var parsed flow.Flow
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal flow: %v", err)
	}

	if parsed.Name != f.Name {
		t.Errorf("Name = %s, want %s", parsed.Name, f.Name)
	}
	if parsed.StartURL != f.StartURL {
		t.Errorf("StartURL = %s, want %s", parsed.StartURL, f.StartURL)
	}
}
