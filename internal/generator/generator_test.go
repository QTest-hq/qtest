package generator

import (
	"testing"

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
