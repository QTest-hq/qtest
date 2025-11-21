package mutation

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxMutantsPerFunction != 5 {
		t.Errorf("MaxMutantsPerFunction = %d, want 5", cfg.MaxMutantsPerFunction)
	}
	if cfg.Timeout != 2*time.Minute {
		t.Errorf("Timeout = %v, want 2m", cfg.Timeout)
	}
	if cfg.Mode != "fast" {
		t.Errorf("Mode = %s, want fast", cfg.Mode)
	}
}

func TestThoroughConfig(t *testing.T) {
	cfg := ThoroughConfig()

	if cfg.MaxMutantsPerFunction != 10 {
		t.Errorf("MaxMutantsPerFunction = %d, want 10", cfg.MaxMutantsPerFunction)
	}
	if cfg.Timeout != 10*time.Minute {
		t.Errorf("Timeout = %v, want 10m", cfg.Timeout)
	}
	if cfg.Mode != "thorough" {
		t.Errorf("Mode = %s, want thorough", cfg.Mode)
	}
}

func TestResult_Quality(t *testing.T) {
	tests := []struct {
		name  string
		score float64
		want  string
	}{
		{"good score", 0.85, "good"},
		{"threshold good", 0.70, "good"},
		{"acceptable score", 0.60, "acceptable"},
		{"threshold acceptable", 0.50, "acceptable"},
		{"poor score", 0.30, "poor"},
		{"zero score", 0.0, "poor"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Result{Score: tt.score}
			if got := r.Quality(); got != tt.want {
				t.Errorf("Quality() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestResult_HasMutants(t *testing.T) {
	tests := []struct {
		name  string
		total int
		want  bool
	}{
		{"has mutants", 10, true},
		{"no mutants", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Result{Total: tt.total}
			if got := r.HasMutants(); got != tt.want {
				t.Errorf("HasMutants() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInferMutationType(t *testing.T) {
	tests := []struct {
		desc string
		want string
	}{
		{"Replaced + with -", "arithmetic"},
		{"Replaced - with +", "arithmetic"},
		{"Replaced * with /", "arithmetic"},
		{"Replaced == with !=", "comparison"},
		{"Replaced < with >", "comparison"},
		{"Replaced && with ||", "boolean"},
		{"Replaced true with false", "boolean"},
		{"return 0 instead of 1", "return"},
		{"removed function call", "statement"},
		{"branch condition changed", "branch"},
		{"something else entirely", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			if got := inferMutationType(tt.desc); got != tt.want {
				t.Errorf("inferMutationType(%s) = %s, want %s", tt.desc, got, tt.want)
			}
		})
	}
}

func TestGoMutestingTool_Name(t *testing.T) {
	tool := NewGoMutestingTool()
	if tool.Name() != "go-mutesting" {
		t.Errorf("Name() = %s, want go-mutesting", tool.Name())
	}
}

func TestSimpleMutationTool_Name(t *testing.T) {
	tool := NewSimpleMutationTool()
	if tool.Name() != "simple" {
		t.Errorf("Name() = %s, want simple", tool.Name())
	}
}

func TestSimpleMutationTool_IsAvailable(t *testing.T) {
	tool := NewSimpleMutationTool()
	if !tool.IsAvailable(context.Background()) {
		t.Error("SimpleMutationTool should always be available")
	}
}

func TestNewRunner(t *testing.T) {
	tool1 := NewSimpleMutationTool()
	tool2 := NewGoMutestingTool()

	runner := NewRunner(tool1, tool2)
	if len(runner.tools) != 2 {
		t.Errorf("len(tools) = %d, want 2", len(runner.tools))
	}
}

func TestRunner_AddTool(t *testing.T) {
	runner := NewRunner()
	runner.AddTool(NewSimpleMutationTool())

	if len(runner.tools) != 1 {
		t.Errorf("len(tools) = %d, want 1", len(runner.tools))
	}
}

func TestRunner_GetAvailableTools(t *testing.T) {
	runner := NewRunner(NewSimpleMutationTool())

	available := runner.GetAvailableTools(context.Background())
	if len(available) == 0 {
		t.Error("should have at least one available tool")
	}
}

func TestRunner_Run_NoTools(t *testing.T) {
	runner := NewRunner()

	_, err := runner.Run(context.Background(), "source.go", "source_test.go", DefaultConfig())
	if err == nil {
		t.Error("expected error when no tools configured")
	}
}

func TestParseGoMutestingOutput(t *testing.T) {
	output := `PASS: foo.go:10: Replaced + with -
PASS: foo.go:20: Replaced == with !=
FAIL: foo.go:30: Replaced && with ||
SKIP: foo.go:40: Timeout`

	result := &Result{}
	parseGoMutestingOutput(output, result)

	if result.Total != 4 {
		t.Errorf("Total = %d, want 4", result.Total)
	}
	if result.Killed != 2 {
		t.Errorf("Killed = %d, want 2", result.Killed)
	}
	if result.Survived != 1 {
		t.Errorf("Survived = %d, want 1", result.Survived)
	}
	if result.Timeout != 1 {
		t.Errorf("Timeout = %d, want 1", result.Timeout)
	}
	if len(result.Mutants) != 4 {
		t.Errorf("len(Mutants) = %d, want 4", len(result.Mutants))
	}
}

func TestParseSummary(t *testing.T) {
	output := `Some output
10 mutants passed testing
5 mutants did not pass testing`

	result := &Result{}
	parseSummary(output, result)

	if result.Killed != 10 {
		t.Errorf("Killed = %d, want 10", result.Killed)
	}
	if result.Survived != 5 {
		t.Errorf("Survived = %d, want 5", result.Survived)
	}
}

func TestMutant_Statuses(t *testing.T) {
	if StatusKilled != "killed" {
		t.Errorf("StatusKilled = %s, want killed", StatusKilled)
	}
	if StatusSurvived != "survived" {
		t.Errorf("StatusSurvived = %s, want survived", StatusSurvived)
	}
	if StatusTimeout != "timeout" {
		t.Errorf("StatusTimeout = %s, want timeout", StatusTimeout)
	}
	if StatusError != "error" {
		t.Errorf("StatusError = %s, want error", StatusError)
	}
}

func TestThresholds(t *testing.T) {
	if ThresholdGood != 0.70 {
		t.Errorf("ThresholdGood = %f, want 0.70", ThresholdGood)
	}
	if ThresholdAcceptable != 0.50 {
		t.Errorf("ThresholdAcceptable = %f, want 0.50", ThresholdAcceptable)
	}
}

func TestNewReporter(t *testing.T) {
	reporter := NewReporter("/tmp/reports")
	if reporter == nil {
		t.Error("NewReporter should not return nil")
	}
	if reporter.outputDir != "/tmp/reports" {
		t.Errorf("outputDir = %s, want /tmp/reports", reporter.outputDir)
	}
}

func TestReporter_GenerateHTMLReport(t *testing.T) {
	dir := t.TempDir()
	reporter := NewReporter(dir)

	result := &Result{
		SourceFile: "calculator.go",
		TestFile:   "calculator_test.go",
		Total:      10,
		Killed:     7,
		Survived:   2,
		Timeout:    1,
		Score:      0.70,
		Duration:   5 * time.Second,
		Mutants: []Mutant{
			{ID: "1", Type: "arithmetic", Line: 10, Status: StatusKilled, Description: "Replaced + with -"},
			{ID: "2", Type: "comparison", Line: 15, Status: StatusSurvived, Description: "Replaced == with !="},
		},
	}

	path, err := reporter.GenerateReport(result, FormatHTML)
	if err != nil {
		t.Fatalf("GenerateReport() error: %v", err)
	}
	if path == "" {
		t.Error("GenerateReport() returned empty path")
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("report file does not exist: %s", path)
	}

	// Read and verify content
	content, _ := os.ReadFile(path)
	if !bytes.Contains(content, []byte("calculator.go")) {
		t.Error("HTML report should contain source file name")
	}
	if !bytes.Contains(content, []byte("70.0%")) {
		t.Error("HTML report should contain score")
	}
	if !bytes.Contains(content, []byte("good")) {
		t.Error("HTML report should contain quality")
	}
}

func TestReporter_GenerateJSONReport(t *testing.T) {
	dir := t.TempDir()
	reporter := NewReporter(dir)

	result := &Result{
		SourceFile: "service.go",
		TestFile:   "service_test.go",
		Total:      5,
		Killed:     4,
		Survived:   1,
		Score:      0.80,
	}

	path, err := reporter.GenerateReport(result, FormatJSON)
	if err != nil {
		t.Fatalf("GenerateReport() error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("report file does not exist: %s", path)
	}

	// Verify JSON content
	content, _ := os.ReadFile(path)
	var parsed Result
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("failed to parse JSON report: %v", err)
	}
	if parsed.SourceFile != "service.go" {
		t.Errorf("SourceFile = %s, want service.go", parsed.SourceFile)
	}
	if parsed.Score != 0.80 {
		t.Errorf("Score = %f, want 0.80", parsed.Score)
	}
}

func TestReporter_GenerateTextReport(t *testing.T) {
	dir := t.TempDir()
	reporter := NewReporter(dir)

	result := &Result{
		SourceFile: "handler.go",
		TestFile:   "handler_test.go",
		Total:      8,
		Killed:     6,
		Survived:   1,
		Timeout:    1,
		Score:      0.75,
		Duration:   3 * time.Second,
		Mutants: []Mutant{
			{ID: "1", Type: "boolean", Line: 20, Status: StatusKilled, Description: "Replaced true with false"},
		},
	}

	path, err := reporter.GenerateReport(result, FormatText)
	if err != nil {
		t.Fatalf("GenerateReport() error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("report file does not exist: %s", path)
	}

	// Verify text content
	content, _ := os.ReadFile(path)
	if !bytes.Contains(content, []byte("MUTATION TESTING REPORT")) {
		t.Error("text report should contain header")
	}
	if !bytes.Contains(content, []byte("handler.go")) {
		t.Error("text report should contain source file")
	}
	if !bytes.Contains(content, []byte("75.0%")) {
		t.Error("text report should contain score")
	}
}

func TestReporter_UnsupportedFormat(t *testing.T) {
	reporter := NewReporter(t.TempDir())

	_, err := reporter.GenerateReport(&Result{}, "invalid")
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

func TestQualityClass(t *testing.T) {
	tests := []struct {
		quality string
		want    string
	}{
		{"good", "quality-good"},
		{"acceptable", "quality-acceptable"},
		{"poor", "quality-poor"},
		{"unknown", "quality-poor"},
	}

	for _, tt := range tests {
		t.Run(tt.quality, func(t *testing.T) {
			if got := qualityClass(tt.quality); got != tt.want {
				t.Errorf("qualityClass(%s) = %s, want %s", tt.quality, got, tt.want)
			}
		})
	}
}

func TestStatusIcon(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{StatusKilled, "✓"},
		{StatusSurvived, "✗"},
		{StatusTimeout, "⏱"},
		{StatusError, "?"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := statusIcon(tt.status); got != tt.want {
				t.Errorf("statusIcon(%s) = %s, want %s", tt.status, got, tt.want)
			}
		})
	}
}

func TestStatusClass(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{StatusKilled, "status-killed"},
		{StatusSurvived, "status-survived"},
		{StatusTimeout, "status-timeout"},
		{StatusError, "status-error"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := statusClass(tt.status); got != tt.want {
				t.Errorf("statusClass(%s) = %s, want %s", tt.status, got, tt.want)
			}
		})
	}
}
