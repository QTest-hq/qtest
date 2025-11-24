package generator

import (
	"testing"
	"time"

	"github.com/QTest-hq/qtest/internal/llm"
	"github.com/QTest-hq/qtest/internal/parser"
	"github.com/QTest-hq/qtest/pkg/dsl"
)

func TestNewGenerator(t *testing.T) {
	// NewGenerator with nil router (acceptable for unit testing)
	gen := NewGenerator(nil)
	if gen == nil {
		t.Fatal("NewGenerator returned nil")
	}
	if gen.parser == nil {
		t.Error("parser should be initialized")
	}
	// llmRouter can be nil
}

func TestGenerateOptions_Fields(t *testing.T) {
	opts := GenerateOptions{
		Tier:       llm.Tier2,
		TestType:   dsl.TestTypeUnit,
		Framework:  "testing",
		MaxTests:   10,
		TargetFile: "main.go",
	}

	if opts.Tier != llm.Tier2 {
		t.Errorf("Tier = %d, want 2", opts.Tier)
	}
	if opts.TestType != dsl.TestTypeUnit {
		t.Errorf("TestType = %s, want unit", opts.TestType)
	}
	if opts.Framework != "testing" {
		t.Errorf("Framework = %s, want testing", opts.Framework)
	}
	if opts.MaxTests != 10 {
		t.Errorf("MaxTests = %d, want 10", opts.MaxTests)
	}
	if opts.TargetFile != "main.go" {
		t.Errorf("TargetFile = %s, want main.go", opts.TargetFile)
	}
}

func TestGeneratedTest_Fields(t *testing.T) {
	fn := &parser.Function{Name: "TestFunc"}
	testDSL := &dsl.TestDSL{Name: "Test_TestFunc"}

	gt := GeneratedTest{
		DSL:      testDSL,
		RawYAML:  "name: test",
		Function: fn,
		FileName: "test.go",
	}

	if gt.DSL != testDSL {
		t.Error("DSL not set correctly")
	}
	if gt.RawYAML != "name: test" {
		t.Errorf("RawYAML = %s, want 'name: test'", gt.RawYAML)
	}
	if gt.Function != fn {
		t.Error("Function not set correctly")
	}
	if gt.FileName != "test.go" {
		t.Errorf("FileName = %s, want test.go", gt.FileName)
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "single line",
			content: "hello",
			want:    []string{"hello"},
		},
		{
			name:    "multiple lines",
			content: "line1\nline2\nline3",
			want:    []string{"line1", "line2", "line3"},
		},
		{
			name:    "trailing newline",
			content: "line1\nline2\n",
			want:    []string{"line1", "line2"},
		},
		{
			name:    "empty string",
			content: "",
			want:    []string{},
		},
		{
			name:    "only newlines",
			content: "\n\n",
			want:    []string{"", ""},
		},
		{
			name:    "code with indentation",
			content: "func main() {\n\tfmt.Println(\"hello\")\n}",
			want:    []string{"func main() {", "\tfmt.Println(\"hello\")", "}"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitLines(tt.content)
			if len(got) != len(tt.want) {
				t.Errorf("len(splitLines) = %d, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitLines[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtractLines(t *testing.T) {
	lines := []string{"line1", "line2", "line3", "line4", "line5"}

	tests := []struct {
		name      string
		startLine int
		endLine   int
		want      string
	}{
		{
			name:      "extract middle",
			startLine: 2,
			endLine:   4,
			want:      "line2\nline3\nline4\n",
		},
		{
			name:      "extract all",
			startLine: 1,
			endLine:   5,
			want:      "line1\nline2\nline3\nline4\nline5\n",
		},
		{
			name:      "extract single line",
			startLine: 3,
			endLine:   3,
			want:      "line3\n",
		},
		{
			name:      "start before 1",
			startLine: 0,
			endLine:   2,
			want:      "line1\nline2\n",
		},
		{
			name:      "end beyond length",
			startLine: 4,
			endLine:   10,
			want:      "line4\nline5\n",
		},
		{
			name:      "both out of bounds",
			startLine: -5,
			endLine:   100,
			want:      "line1\nline2\nline3\nline4\nline5\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractLines(lines, tt.startLine, tt.endLine)
			if got != tt.want {
				t.Errorf("extractLines(%d, %d) = %q, want %q", tt.startLine, tt.endLine, got, tt.want)
			}
		})
	}
}

func TestExtractLines_EmptySlice(t *testing.T) {
	got := extractLines([]string{}, 1, 5)
	if got != "" {
		t.Errorf("extractLines(empty, 1, 5) = %q, want empty string", got)
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{1, 2, 1},
		{5, 3, 3},
		{0, 0, 0},
		{-1, 1, -1},
		{100, 100, 100},
	}

	for _, tt := range tests {
		got := min(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("min(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestBuildContext(t *testing.T) {
	gen := NewGenerator(nil)

	t.Run("with related functions", func(t *testing.T) {
		file := &parser.ParsedFile{
			Functions: []parser.Function{
				{Name: "Add"},
				{Name: "Subtract"},
				{Name: "Multiply"},
			},
		}
		targetFn := &parser.Function{Name: "Add"}

		ctx := gen.buildContext(file, targetFn)

		if ctx == "" {
			t.Error("buildContext should return non-empty string")
		}
		if !contains(ctx, "Subtract") {
			t.Error("context should contain Subtract")
		}
		if !contains(ctx, "Multiply") {
			t.Error("context should contain Multiply")
		}
		if contains(ctx, "Add") && !contains(ctx, "Related") {
			t.Error("context should not contain target function name except in description")
		}
	})

	t.Run("no related functions", func(t *testing.T) {
		file := &parser.ParsedFile{
			Functions: []parser.Function{
				{Name: "OnlyFunc"},
			},
		}
		targetFn := &parser.Function{Name: "OnlyFunc"}

		ctx := gen.buildContext(file, targetFn)

		if ctx != "" {
			t.Errorf("buildContext should return empty string when no related functions, got %q", ctx)
		}
	})

	t.Run("empty file", func(t *testing.T) {
		file := &parser.ParsedFile{
			Functions: []parser.Function{},
		}
		targetFn := &parser.Function{Name: "Func"}

		ctx := gen.buildContext(file, targetFn)

		if ctx != "" {
			t.Errorf("buildContext should return empty string for empty file, got %q", ctx)
		}
	})
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestGenerateOptions_Defaults(t *testing.T) {
	// Zero value should be usable
	opts := GenerateOptions{}

	if opts.Tier != 0 {
		t.Errorf("default Tier = %d, want 0", opts.Tier)
	}
	if opts.MaxTests != 0 {
		t.Errorf("default MaxTests = %d, want 0", opts.MaxTests)
	}
	if opts.Framework != "" {
		t.Errorf("default Framework = %s, want empty", opts.Framework)
	}
}

func TestGeneratedTest_NilFields(t *testing.T) {
	gt := GeneratedTest{}

	if gt.DSL != nil {
		t.Error("default DSL should be nil")
	}
	if gt.Function != nil {
		t.Error("default Function should be nil")
	}
	if gt.RawYAML != "" {
		t.Error("default RawYAML should be empty")
	}
	if gt.FileName != "" {
		t.Error("default FileName should be empty")
	}
}

func TestSplitLines_WindowsLineEndings(t *testing.T) {
	// Windows line endings (\r\n) - current implementation only handles \n
	content := "line1\r\nline2\r\n"
	lines := splitLines(content)

	// Current implementation doesn't strip \r
	// This test documents current behavior
	if len(lines) < 2 {
		t.Errorf("should have at least 2 lines, got %d", len(lines))
	}
}

func TestExtractLines_SingleLineFile(t *testing.T) {
	lines := []string{"only line"}

	got := extractLines(lines, 1, 1)
	want := "only line\n"

	if got != want {
		t.Errorf("extractLines = %q, want %q", got, want)
	}
}

// Batch Generation Tests

func TestBatchOptions_Fields(t *testing.T) {
	progressCalled := false
	opts := BatchOptions{
		GenerateOptions: GenerateOptions{
			Tier:     llm.Tier1,
			TestType: dsl.TestTypeUnit,
			MaxTests: 10,
		},
		Concurrency: 4,
		Files:       []string{"file1.go", "file2.go"},
		OnProgress: func(completed, total int, current string) {
			progressCalled = true
		},
	}

	if opts.Concurrency != 4 {
		t.Errorf("Concurrency = %d, want 4", opts.Concurrency)
	}
	if len(opts.Files) != 2 {
		t.Errorf("Files count = %d, want 2", len(opts.Files))
	}
	if opts.MaxTests != 10 {
		t.Errorf("MaxTests = %d, want 10", opts.MaxTests)
	}
	if opts.Tier != llm.Tier1 {
		t.Errorf("Tier = %d, want 1", opts.Tier)
	}

	// Test progress callback
	if opts.OnProgress != nil {
		opts.OnProgress(1, 2, "test")
		if !progressCalled {
			t.Error("OnProgress callback should have been called")
		}
	}
}

func TestBatchResult_Fields(t *testing.T) {
	result := BatchResult{
		Tests:      make([]GeneratedTest, 2),
		Errors:     make([]BatchError, 1),
		TotalFiles: 3,
		TotalFuncs: 10,
		Generated:  2,
		Failed:     1,
	}

	if result.TotalFiles != 3 {
		t.Errorf("TotalFiles = %d, want 3", result.TotalFiles)
	}
	if result.TotalFuncs != 10 {
		t.Errorf("TotalFuncs = %d, want 10", result.TotalFuncs)
	}
	if result.Generated != 2 {
		t.Errorf("Generated = %d, want 2", result.Generated)
	}
	if result.Failed != 1 {
		t.Errorf("Failed = %d, want 1", result.Failed)
	}
	if len(result.Tests) != 2 {
		t.Errorf("len(Tests) = %d, want 2", len(result.Tests))
	}
	if len(result.Errors) != 1 {
		t.Errorf("len(Errors) = %d, want 1", len(result.Errors))
	}
}

func TestBatchError_Fields(t *testing.T) {
	testErr := BatchError{
		File:     "test.go",
		Function: "TestFunc",
		Error:    nil,
	}

	if testErr.File != "test.go" {
		t.Errorf("File = %s, want test.go", testErr.File)
	}
	if testErr.Function != "TestFunc" {
		t.Errorf("Function = %s, want TestFunc", testErr.Function)
	}
}

func TestBatchOptions_DefaultConcurrency(t *testing.T) {
	opts := BatchOptions{
		GenerateOptions: GenerateOptions{},
		Concurrency:     0, // Should default to 4
		Files:           []string{"file.go"},
	}

	// The actual default is applied in GenerateBatch
	if opts.Concurrency != 0 {
		t.Errorf("initial Concurrency = %d, want 0", opts.Concurrency)
	}
}

func TestBatchResult_Empty(t *testing.T) {
	result := BatchResult{}

	if result.TotalFiles != 0 {
		t.Error("default TotalFiles should be 0")
	}
	if result.Generated != 0 {
		t.Error("default Generated should be 0")
	}
	if result.Failed != 0 {
		t.Error("default Failed should be 0")
	}
	if result.Tests != nil {
		t.Error("default Tests should be nil")
	}
	if result.Errors != nil {
		t.Error("default Errors should be nil")
	}
}

func TestGenerateBatch_NoFiles(t *testing.T) {
	gen := NewGenerator(nil)

	opts := BatchOptions{
		GenerateOptions: GenerateOptions{},
		Files:           []string{}, // Empty files list
	}

	_, err := gen.GenerateBatch(nil, opts)
	if err == nil {
		t.Error("expected error for empty files list")
	}
}

func TestBatchOptions_WithProgress(t *testing.T) {
	var progressUpdates []struct {
		completed int
		total     int
		current   string
	}

	opts := BatchOptions{
		GenerateOptions: GenerateOptions{},
		Files:           []string{"file.go"},
		OnProgress: func(completed, total int, current string) {
			progressUpdates = append(progressUpdates, struct {
				completed int
				total     int
				current   string
			}{completed, total, current})
		},
	}

	// Simulate progress callback
	opts.OnProgress(1, 5, "Func1")
	opts.OnProgress(2, 5, "Func2")

	if len(progressUpdates) != 2 {
		t.Errorf("expected 2 progress updates, got %d", len(progressUpdates))
	}
	if progressUpdates[0].completed != 1 {
		t.Errorf("first update completed = %d, want 1", progressUpdates[0].completed)
	}
	if progressUpdates[1].current != "Func2" {
		t.Errorf("second update current = %s, want Func2", progressUpdates[1].current)
	}
}

func TestBatchResult_SuccessRate(t *testing.T) {
	tests := []struct {
		name      string
		generated int
		failed    int
		want      float64
	}{
		{"all success", 10, 0, 100.0},
		{"half success", 5, 5, 50.0},
		{"no success", 0, 10, 0.0},
		{"zero total", 0, 0, 0.0},
		{"75% success", 3, 1, 75.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BatchResult{
				Generated: tt.generated,
				Failed:    tt.failed,
			}
			got := result.SuccessRate()
			if got != tt.want {
				t.Errorf("SuccessRate() = %.2f, want %.2f", got, tt.want)
			}
		})
	}
}

func TestBatchResult_Summary(t *testing.T) {
	result := BatchResult{
		TotalFiles: 5,
		TotalFuncs: 20,
		Generated:  15,
		Failed:     3,
		Skipped:    2,
		Retried:    4,
	}

	summary := result.Summary()

	// Check that summary contains expected information
	// Format: "Generated %d/%d tests (%.1f%% success) in %v"
	if !containsHelper(summary, "15") {
		t.Error("summary should contain generated count")
	}
	// Total is Generated+Failed (15+3=18)
	if !containsHelper(summary, "18") {
		t.Error("summary should contain total (generated+failed)")
	}
	if !containsHelper(summary, "success") {
		t.Error("summary should contain 'success'")
	}
}

func TestBatchResult_WithTiming(t *testing.T) {
	now := time.Now()
	result := BatchResult{
		StartTime: now,
		EndTime:   now.Add(5 * time.Second),
		Duration:  5 * time.Second,
	}

	if result.Duration != 5*time.Second {
		t.Errorf("Duration = %v, want 5s", result.Duration)
	}
	if result.StartTime.IsZero() {
		t.Error("StartTime should not be zero")
	}
	if result.EndTime.Before(result.StartTime) {
		t.Error("EndTime should be after StartTime")
	}
}

func TestBatchOptions_RetrySettings(t *testing.T) {
	opts := BatchOptions{
		GenerateOptions: GenerateOptions{},
		Files:           []string{"file.go"},
		MaxRetries:      3,
		RetryDelay:      2 * time.Second,
	}

	if opts.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", opts.MaxRetries)
	}
	if opts.RetryDelay != 2*time.Second {
		t.Errorf("RetryDelay = %v, want 2s", opts.RetryDelay)
	}
}

func TestBatchOptions_Filtering(t *testing.T) {
	filterCalled := false
	opts := BatchOptions{
		GenerateOptions: GenerateOptions{},
		Files:           []string{"file.go"},
		SkipPrivate:     true,
		FuncFilter: func(name string) bool {
			filterCalled = true
			return name != "skipMe"
		},
	}

	if !opts.SkipPrivate {
		t.Error("SkipPrivate should be true")
	}

	// Test the filter function
	if opts.FuncFilter != nil {
		result := opts.FuncFilter("testFunc")
		if !filterCalled {
			t.Error("FuncFilter should have been called")
		}
		if !result {
			t.Error("FuncFilter should return true for 'testFunc'")
		}

		result = opts.FuncFilter("skipMe")
		if result {
			t.Error("FuncFilter should return false for 'skipMe'")
		}
	}
}

func TestBatchError_RetryFields(t *testing.T) {
	testErr := BatchError{
		File:      "test.go",
		Function:  "TestFunc",
		Error:     nil,
		Retries:   2,
		Retryable: true,
	}

	if testErr.Retries != 2 {
		t.Errorf("Retries = %d, want 2", testErr.Retries)
	}
	if !testErr.Retryable {
		t.Error("Retryable should be true")
	}
}

func TestBatchResult_SkippedAndRetried(t *testing.T) {
	result := BatchResult{
		Generated:  10,
		Failed:     2,
		Skipped:    3,
		Retried:    5,
		TotalFuncs: 15,
	}

	if result.Skipped != 3 {
		t.Errorf("Skipped = %d, want 3", result.Skipped)
	}
	if result.Retried != 5 {
		t.Errorf("Retried = %d, want 5", result.Retried)
	}

	// Verify counts add up correctly
	// Generated + Failed + Skipped should equal TotalFuncs
	total := result.Generated + result.Failed + result.Skipped
	if total != result.TotalFuncs {
		t.Errorf("Generated(%d) + Failed(%d) + Skipped(%d) = %d, want TotalFuncs(%d)",
			result.Generated, result.Failed, result.Skipped, total, result.TotalFuncs)
	}
}
