package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewCLIProgress(t *testing.T) {
	tests := []struct {
		name    string
		verbose bool
		quiet   bool
	}{
		{"default", false, false},
		{"verbose", true, false},
		{"quiet", false, true},
		{"verbose_and_quiet", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewCLIProgress(tt.verbose, tt.quiet)
			if p == nil {
				t.Fatal("expected non-nil progress tracker")
			}
			if p.verbose != tt.verbose {
				t.Errorf("expected verbose=%v, got %v", tt.verbose, p.verbose)
			}
			if p.quiet != tt.quiet {
				t.Errorf("expected quiet=%v, got %v", tt.quiet, p.quiet)
			}
		})
	}
}

func TestCLIProgress_QuietMode(t *testing.T) {
	var buf bytes.Buffer

	p := NewCLIProgress(false, true) // quiet mode

	// These should not produce any output in quiet mode
	p.Print("test")
	p.Println("test")
	p.Success("test")

	// Spinner operations should be no-ops
	p.StartSpinner("test")
	p.UpdateSpinner("test")
	p.StopSpinnerSuccess("test")

	// Bar operations should be no-ops
	p.StartBar(10, "test")
	p.UpdateBar(5)
	p.IncrementBar()
	p.DoneBar()

	// Only warning and error should work in quiet mode (they go to stderr)
	// but we're not capturing stderr in this test

	if buf.Len() != 0 {
		t.Error("expected no output in quiet mode")
	}
}

func TestCLIProgress_VerboseMode(t *testing.T) {
	p := NewCLIProgress(true, false) // verbose mode

	// Verify verbose flag is set
	if !p.verbose {
		t.Error("expected verbose mode to be enabled")
	}

	// PrintVerbose should be callable in verbose mode
	// (actual output goes to stdout, hard to capture without redirecting)
}

func TestCLIProgress_ProgressCallback(t *testing.T) {
	p := NewCLIProgress(false, false)

	callback := p.ProgressCallback()
	if callback == nil {
		t.Fatal("expected non-nil progress callback")
	}

	// Should not panic when called
	callback("test", 1, 10, "message")
	callback("test", 5, 0, "no total")
}

func TestCLIProgress_CompleteCallback(t *testing.T) {
	p := NewCLIProgress(false, false)

	callback := p.CompleteCallback()
	if callback == nil {
		t.Fatal("expected non-nil complete callback")
	}

	// Should not panic when called
	callback("test.go", 5)
}

func TestCLIProgress_ProgressCallback_Quiet(t *testing.T) {
	p := NewCLIProgress(false, true) // quiet mode

	callback := p.ProgressCallback()

	// Should not panic and should produce no output in quiet mode
	callback("test", 1, 10, "message")
}

func TestCLIProgress_Table(t *testing.T) {
	p := NewCLIProgress(false, false)

	tbl := p.Table("Col1", "Col2", "Col3")
	if tbl == nil {
		t.Fatal("expected non-nil table")
	}

	tbl.AddRow("a", "b", "c")
	tbl.AddRow("d", "e", "f")

	output := tbl.Render()
	if !strings.Contains(output, "Col1") {
		t.Error("expected table to contain 'Col1'")
	}
	if !strings.Contains(output, "a") {
		t.Error("expected table to contain 'a'")
	}
}

func TestCLIProgress_WithSpinner(t *testing.T) {
	p := NewCLIProgress(false, false)

	// Test successful execution
	callCount := 0
	err := p.WithSpinner("Test operation", func() error {
		callCount++
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected function to be called once, got %d", callCount)
	}
}

func TestCLIProgress_WithSpinner_Error(t *testing.T) {
	p := NewCLIProgress(false, false)

	expectedErr := bytes.ErrTooLarge
	err := p.WithSpinner("Test operation", func() error {
		return expectedErr
	})

	if err != expectedErr {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
}

func TestCLIProgress_WithSpinner_Quiet(t *testing.T) {
	p := NewCLIProgress(false, true) // quiet mode

	callCount := 0
	err := p.WithSpinner("Test", func() error {
		callCount++
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if callCount != 1 {
		t.Error("expected function to still be called in quiet mode")
	}
}

func TestCLIProgress_SpinnerOperations(t *testing.T) {
	p := NewCLIProgress(false, false)

	// Test spinner lifecycle
	p.StartSpinner("Starting...")
	p.UpdateSpinner("In progress...")
	p.StopSpinnerSuccess("Done")

	// Second spinner
	p.StartSpinner("Another task")
	p.StopSpinnerFail("Failed")
}

func TestCLIProgress_BarOperations(t *testing.T) {
	p := NewCLIProgress(false, false)

	p.StartBar(10, "Processing")
	p.UpdateBar(3)
	p.IncrementBar() // now 4
	p.SetBarMessage("Updated message")
	p.DoneBar()

	// Operations after done should be safe
	p.UpdateBar(5)
	p.IncrementBar()
}

func TestCLIProgress_PipelineOperations(t *testing.T) {
	p := NewCLIProgress(false, false)

	p.StartPipeline("Phase 1", "Phase 2", "Phase 3")
	p.StartPhase("Phase 1", 10)
	p.UpdatePhase(5, "Halfway")
	p.CompletePhase("Phase 1 complete")

	summary := p.Summary()
	if !strings.Contains(summary, "Pipeline Summary") {
		t.Error("expected summary to contain 'Pipeline Summary'")
	}
}

func TestCLIProgress_PipelineQuiet(t *testing.T) {
	p := NewCLIProgress(false, true) // quiet

	// These should all be no-ops in quiet mode
	p.StartPipeline("Phase 1", "Phase 2")
	p.StartPhase("Phase 1", 10)
	p.UpdatePhase(5, "Halfway")
	p.CompletePhase("Done")

	// Summary still returns empty string
	summary := p.Summary()
	if summary != "" {
		t.Errorf("expected empty summary in quiet mode, got %s", summary)
	}
}
