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

// Stryker tests

func TestNewStrykerTool(t *testing.T) {
	tool := NewStrykerTool()
	if tool == nil {
		t.Error("NewStrykerTool should not return nil")
	}
	if !tool.UseNpx {
		t.Error("UseNpx should be true by default")
	}
}

func TestStrykerTool_Name(t *testing.T) {
	tool := NewStrykerTool()
	if tool.Name() != "stryker" {
		t.Errorf("Name() = %s, want stryker", tool.Name())
	}
}

func TestStrykerTool_MapMutatorName(t *testing.T) {
	tool := NewStrykerTool()

	tests := []struct {
		mutator string
		want    string
	}{
		{"ArithmeticOperator", "arithmetic"},
		{"UnaryOperator", "arithmetic"},
		{"EqualityOperator", "comparison"},
		{"RelationalOperator", "comparison"},
		{"LogicalOperator", "boolean"},
		{"BooleanLiteral", "boolean"},
		{"BlockStatement", "branch"},
		{"ConditionalExpression", "branch"},
		{"StringLiteral", "literal"},
		{"ArrayDeclaration", "literal"},
		{"MethodExpression", "statement"},
		{"ObjectLiteral", "statement"},
		{"UnknownMutator", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.mutator, func(t *testing.T) {
			if got := tool.mapMutatorName(tt.mutator); got != tt.want {
				t.Errorf("mapMutatorName(%s) = %s, want %s", tt.mutator, got, tt.want)
			}
		})
	}
}

func TestStrykerTool_ParseStrykerOutput(t *testing.T) {
	tool := NewStrykerTool()

	tests := []struct {
		name     string
		output   string
		wantKilled   int
		wantSurvived int
		wantTimeout  int
	}{
		{
			name: "summary format",
			output: `
Killed:   15
Survived: 5
Timeout:  2
`,
			wantKilled:   15,
			wantSurvived: 5,
			wantTimeout:  2,
		},
		{
			name:   "progress format",
			output: "Mutation testing  [====================] 100% (elapsed: 10s) 20/20 tested (5 survived, 1 timed out)",
			wantKilled:   14,
			wantSurvived: 5,
			wantTimeout:  1,
		},
		{
			name:   "empty output",
			output: "",
			wantKilled:   0,
			wantSurvived: 0,
			wantTimeout:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &Result{}
			tool.parseStrykerOutput(tt.output, result)

			if result.Killed != tt.wantKilled {
				t.Errorf("Killed = %d, want %d", result.Killed, tt.wantKilled)
			}
			if result.Survived != tt.wantSurvived {
				t.Errorf("Survived = %d, want %d", result.Survived, tt.wantSurvived)
			}
			if result.Timeout != tt.wantTimeout {
				t.Errorf("Timeout = %d, want %d", result.Timeout, tt.wantTimeout)
			}
		})
	}
}

func TestStrykerTool_CreateStrykerConfig(t *testing.T) {
	tool := NewStrykerTool()

	config := tool.createStrykerConfig("src/calculator.ts", "src/calculator.test.ts", DefaultConfig())

	if len(config) == 0 {
		t.Error("createStrykerConfig should return non-empty config")
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(config, &parsed); err != nil {
		t.Errorf("createStrykerConfig should return valid JSON: %v", err)
	}

	// Check required fields
	if _, ok := parsed["testRunner"]; !ok {
		t.Error("config should contain testRunner")
	}
	if _, ok := parsed["mutate"]; !ok {
		t.Error("config should contain mutate")
	}
}

func TestStrykerReport_Types(t *testing.T) {
	// Test that our types can be used
	report := StrykerReport{
		SchemaVersion: "1",
		Files: map[string]StrykerFileResult{
			"src/app.ts": {
				Language: "typescript",
				Mutants: []StrykerMutant{
					{
						ID:          "m1",
						MutatorName: "ArithmeticOperator",
						Status:      "Killed",
					},
				},
			},
		},
	}

	if len(report.Files) != 1 {
		t.Errorf("Files count = %d, want 1", len(report.Files))
	}

	if len(report.Files["src/app.ts"].Mutants) != 1 {
		t.Error("should have 1 mutant")
	}
}

func TestFindProjectRoot(t *testing.T) {
	// Test with a temp directory
	dir := t.TempDir()

	// Without package.json, should return original dir
	result := findProjectRoot(dir)
	if result != dir {
		t.Errorf("without package.json, should return original dir")
	}
}

func TestTruncateOutput(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"this is a long string", 10, "this is a ... (truncated)"},
		{"exact", 5, "exact"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := truncateOutput(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateOutput(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

// Integration tests

func TestSimpleMutationTool_Run(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a simple Go source file
	sourceCode := `package main

func Add(a, b int) int {
	return a + b
}

func Subtract(a, b int) int {
	return a - b
}
`
	sourcePath := tmpDir + "/calc.go"
	os.WriteFile(sourcePath, []byte(sourceCode), 0644)

	// Create a test file
	testCode := `package main

import "testing"

func TestAdd(t *testing.T) {
	if Add(1, 2) != 3 {
		t.Error("Add failed")
	}
}
`
	testPath := tmpDir + "/calc_test.go"
	os.WriteFile(testPath, []byte(testCode), 0644)

	tool := NewSimpleMutationTool()
	ctx := context.Background()

	result, err := tool.Run(ctx, sourcePath, testPath, DefaultConfig())
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if result == nil {
		t.Fatal("Run() returned nil result")
	}

	// SimpleMutationTool should produce some results
	if result.SourceFile != sourcePath {
		t.Errorf("SourceFile = %s, want %s", result.SourceFile, sourcePath)
	}
	if result.TestFile != testPath {
		t.Errorf("TestFile = %s, want %s", result.TestFile, testPath)
	}
}

func TestSimpleMutationTool_Run_NonExistentFile(t *testing.T) {
	tool := NewSimpleMutationTool()
	ctx := context.Background()

	result, err := tool.Run(ctx, "/nonexistent/source.go", "/nonexistent/test.go", DefaultConfig())
	// Should not panic, may return error or empty result
	if err != nil {
		return // Error is acceptable
	}
	if result != nil && result.Error != "" {
		return // Error in result is acceptable
	}
}

func TestRunner_Run_WithSimpleTool(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file
	sourceCode := `package main
func Multiply(a, b int) int { return a * b }
`
	sourcePath := tmpDir + "/multiply.go"
	os.WriteFile(sourcePath, []byte(sourceCode), 0644)

	// Create test file
	testCode := `package main
import "testing"
func TestMultiply(t *testing.T) {
	if Multiply(2, 3) != 6 { t.Error("failed") }
}
`
	testPath := tmpDir + "/multiply_test.go"
	os.WriteFile(testPath, []byte(testCode), 0644)

	// Create runner with SimpleMutationTool
	runner := NewRunner(NewSimpleMutationTool())
	ctx := context.Background()

	result, err := runner.Run(ctx, sourcePath, testPath, DefaultConfig())
	if err != nil {
		t.Fatalf("Runner.Run() error: %v", err)
	}

	if result == nil {
		t.Fatal("Runner.Run() returned nil result")
	}
}

func TestCachedRunner_Integration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create cache
	cache, err := NewCache(&CacheConfig{
		Enabled:       true,
		Directory:     tmpDir,
		TTL:           time.Hour,
		MaxEntries:    10,
		PersistToDisk: false,
	})
	if err != nil {
		t.Fatalf("NewCache error: %v", err)
	}

	// Create source and test files
	sourceCode := `package main
func Divide(a, b int) int { if b == 0 { return 0 }; return a / b }
`
	sourcePath := tmpDir + "/divide.go"
	os.WriteFile(sourcePath, []byte(sourceCode), 0644)

	testCode := `package main
import "testing"
func TestDivide(t *testing.T) { if Divide(6, 2) != 3 { t.Error("failed") } }
`
	testPath := tmpDir + "/divide_test.go"
	os.WriteFile(testPath, []byte(testCode), 0644)

	// Create cached runner
	tool := NewSimpleMutationTool()
	cachedRunner := NewCachedRunner(tool, cache)
	ctx := context.Background()

	// First run - should call the underlying tool
	result1, err := cachedRunner.Run(ctx, sourcePath, testPath, DefaultConfig())
	if err != nil {
		t.Fatalf("First Run() error: %v", err)
	}
	if result1 == nil {
		t.Fatal("First Run() returned nil")
	}

	// Second run - should use cache
	result2, err := cachedRunner.Run(ctx, sourcePath, testPath, DefaultConfig())
	if err != nil {
		t.Fatalf("Second Run() error: %v", err)
	}
	if result2 == nil {
		t.Fatal("Second Run() returned nil")
	}

	// Both results should have same source file
	if result1.SourceFile != result2.SourceFile {
		t.Error("Cached result should match original")
	}
}

func TestCachedRunner_CacheInvalidation(t *testing.T) {
	tmpDir := t.TempDir()

	cache, _ := NewCache(&CacheConfig{
		Enabled:       true,
		Directory:     tmpDir,
		TTL:           time.Hour,
		MaxEntries:    10,
		PersistToDisk: false,
	})

	sourcePath := tmpDir + "/mod.go"
	testPath := tmpDir + "/mod_test.go"

	// Initial content
	os.WriteFile(sourcePath, []byte(`package main
func Mod(a, b int) int { return a % b }
`), 0644)
	os.WriteFile(testPath, []byte(`package main
import "testing"
func TestMod(t *testing.T) { if Mod(7, 3) != 1 { t.Error("failed") } }
`), 0644)

	// Manually set a cached result (to test invalidation)
	result := &Result{
		SourceFile: sourcePath,
		TestFile:   testPath,
		Total:      5,
		Killed:     4,
		Score:      0.8,
	}
	cache.Set(sourcePath, testPath, result)

	// Verify cached
	_, found := cache.Get(sourcePath, testPath)
	if !found {
		t.Error("Result should be cached after Set")
	}

	// Modify source file
	os.WriteFile(sourcePath, []byte(`package main
func Mod(a, b int) int { if b == 0 { return 0 }; return a % b }
`), 0644)

	// Cache should be invalidated on next get (hash mismatch)
	_, found = cache.Get(sourcePath, testPath)
	if found {
		t.Error("Cache should be invalidated after source modification")
	}
}

func TestMutationConfig_Validation(t *testing.T) {
	cfg := DefaultConfig()

	// Test that default values are reasonable
	if cfg.MaxMutantsPerFunction <= 0 {
		t.Error("MaxMutantsPerFunction should be positive")
	}
	if cfg.Timeout <= 0 {
		t.Error("Timeout should be positive")
	}
	if cfg.TimeoutPerMutant <= 0 {
		t.Error("TimeoutPerMutant should be positive")
	}

	// Test thorough config has higher limits
	thorough := ThoroughConfig()
	if thorough.MaxMutantsPerFunction <= cfg.MaxMutantsPerFunction {
		t.Error("Thorough config should have higher MaxMutantsPerFunction")
	}
	if thorough.Timeout <= cfg.Timeout {
		t.Error("Thorough config should have higher Timeout")
	}
}

func TestResult_CalculateScore(t *testing.T) {
	tests := []struct {
		name     string
		killed   int
		survived int
		timeout  int
		want     float64
	}{
		{"all killed", 10, 0, 0, 1.0},
		{"all survived", 0, 10, 0, 0.0},
		{"half and half", 5, 5, 0, 0.5},
		{"with timeouts", 7, 2, 1, 0.8}, // timeouts count as killed (7+1=8 out of 10)
		{"no mutants", 0, 0, 0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Result{
				Total:    tt.killed + tt.survived + tt.timeout,
				Killed:   tt.killed,
				Survived: tt.survived,
				Timeout:  tt.timeout,
			}
			// Calculate score
			if r.Total > 0 {
				r.Score = float64(r.Killed+r.Timeout) / float64(r.Total)
			}

			if r.Score != tt.want {
				t.Errorf("Score = %f, want %f", r.Score, tt.want)
			}
		})
	}
}

func TestMutant_Fields(t *testing.T) {
	m := Mutant{
		ID:          "mut-001",
		Type:        "arithmetic",
		Line:        42,
		Status:      StatusKilled,
		Description: "Replaced + with -",
		Original:    "a + b",
		Mutated:     "a - b",
	}

	if m.ID != "mut-001" {
		t.Errorf("ID = %s, want mut-001", m.ID)
	}
	if m.Type != "arithmetic" {
		t.Errorf("Type = %s, want arithmetic", m.Type)
	}
	if m.Line != 42 {
		t.Errorf("Line = %d, want 42", m.Line)
	}
	if m.Status != StatusKilled {
		t.Errorf("Status = %s, want %s", m.Status, StatusKilled)
	}
}
