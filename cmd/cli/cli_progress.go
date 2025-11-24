package main

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/QTest-hq/qtest/internal/progress"
)

// CLIProgress provides enhanced progress output for CLI commands
type CLIProgress struct {
	mu       sync.Mutex
	pipeline *progress.Pipeline
	bar      *progress.Bar
	spinner  *progress.Spinner
	verbose  bool
	quiet    bool
}

// NewCLIProgress creates a new CLI progress tracker
func NewCLIProgress(verbose, quiet bool) *CLIProgress {
	return &CLIProgress{
		verbose: verbose,
		quiet:   quiet,
	}
}

// StartPipeline initializes a multi-phase pipeline
func (p *CLIProgress) StartPipeline(phases ...string) {
	if p.quiet {
		return
	}
	p.mu.Lock()
	p.pipeline = progress.NewPipeline(phases...)
	p.pipeline.SetVerbose(p.verbose)
	p.mu.Unlock()
}

// StartPhase starts a phase in the pipeline
func (p *CLIProgress) StartPhase(name string, total int) {
	if p.quiet {
		return
	}
	p.mu.Lock()
	if p.pipeline != nil {
		p.pipeline.StartPhase(name, total)
	}
	p.mu.Unlock()
}

// UpdatePhase updates the current phase progress
func (p *CLIProgress) UpdatePhase(current int, message string) {
	if p.quiet {
		return
	}
	p.mu.Lock()
	if p.pipeline != nil {
		p.pipeline.UpdatePhase(current, message)
	}
	p.mu.Unlock()
}

// CompletePhase completes the current phase
func (p *CLIProgress) CompletePhase(message string) {
	if p.quiet {
		return
	}
	p.mu.Lock()
	if p.pipeline != nil {
		p.pipeline.CompletePhase(message)
	}
	p.mu.Unlock()
}

// Summary returns the pipeline summary
func (p *CLIProgress) Summary() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pipeline != nil {
		return p.pipeline.Summary()
	}
	return ""
}

// StartSpinner starts an animated spinner
func (p *CLIProgress) StartSpinner(message string) {
	if p.quiet {
		return
	}
	p.mu.Lock()
	p.spinner = progress.NewSpinner(message)
	p.spinner.Start()
	p.mu.Unlock()
}

// UpdateSpinner updates the spinner message
func (p *CLIProgress) UpdateSpinner(message string) {
	if p.quiet {
		return
	}
	p.mu.Lock()
	if p.spinner != nil {
		p.spinner.UpdateMessage(message)
	}
	p.mu.Unlock()
}

// StopSpinnerSuccess stops the spinner with success
func (p *CLIProgress) StopSpinnerSuccess(message string) {
	if p.quiet {
		return
	}
	p.mu.Lock()
	if p.spinner != nil {
		p.spinner.Success(message)
		p.spinner = nil
	}
	p.mu.Unlock()
}

// StopSpinnerFail stops the spinner with failure
func (p *CLIProgress) StopSpinnerFail(message string) {
	if p.quiet {
		return
	}
	p.mu.Lock()
	if p.spinner != nil {
		p.spinner.Fail(message)
		p.spinner = nil
	}
	p.mu.Unlock()
}

// StartBar starts a progress bar
func (p *CLIProgress) StartBar(total int, message string) {
	if p.quiet {
		return
	}
	p.mu.Lock()
	p.bar = progress.NewBar(total, message)
	p.mu.Unlock()
}

// UpdateBar updates the progress bar
func (p *CLIProgress) UpdateBar(current int) {
	if p.quiet {
		return
	}
	p.mu.Lock()
	if p.bar != nil {
		p.bar.Update(current)
	}
	p.mu.Unlock()
}

// IncrementBar increments the progress bar
func (p *CLIProgress) IncrementBar() {
	if p.quiet {
		return
	}
	p.mu.Lock()
	if p.bar != nil {
		p.bar.Increment()
	}
	p.mu.Unlock()
}

// SetBarMessage updates the bar message
func (p *CLIProgress) SetBarMessage(message string) {
	if p.quiet {
		return
	}
	p.mu.Lock()
	if p.bar != nil {
		p.bar.SetMessage(message)
	}
	p.mu.Unlock()
}

// DoneBar completes the progress bar
func (p *CLIProgress) DoneBar() {
	if p.quiet {
		return
	}
	p.mu.Lock()
	if p.bar != nil {
		p.bar.Done()
		p.bar = nil
	}
	p.mu.Unlock()
}

// Print prints a message if not quiet
func (p *CLIProgress) Print(format string, args ...interface{}) {
	if p.quiet {
		return
	}
	fmt.Printf(format, args...)
}

// Println prints a line if not quiet
func (p *CLIProgress) Println(args ...interface{}) {
	if p.quiet {
		return
	}
	fmt.Println(args...)
}

// PrintVerbose prints only in verbose mode
func (p *CLIProgress) PrintVerbose(format string, args ...interface{}) {
	if !p.verbose || p.quiet {
		return
	}
	fmt.Printf(format, args...)
}

// Success prints a success message
func (p *CLIProgress) Success(message string) {
	if p.quiet {
		return
	}
	fmt.Printf("✓ %s\n", message)
}

// Warning prints a warning message
func (p *CLIProgress) Warning(message string) {
	fmt.Fprintf(os.Stderr, "⚠️  %s\n", message)
}

// Error prints an error message
func (p *CLIProgress) Error(message string) {
	fmt.Fprintf(os.Stderr, "✗ %s\n", message)
}

// Table creates a formatted table
func (p *CLIProgress) Table(headers ...string) *progress.Table {
	return progress.NewTable(headers...)
}

// PrintTable renders and prints a table
func (p *CLIProgress) PrintTable(tbl *progress.Table) {
	if p.quiet {
		return
	}
	fmt.Print(tbl.Render())
}

// Separator prints a separator line
func (p *CLIProgress) Separator(char string, width int) {
	if p.quiet {
		return
	}
	fmt.Println(strings.Repeat(char, width))
}

// ProgressCallback returns a callback function compatible with workspace runners
func (p *CLIProgress) ProgressCallback() func(phase string, current, total int, message string) {
	return func(phase string, current, total int, message string) {
		if p.quiet {
			return
		}

		p.mu.Lock()
		defer p.mu.Unlock()

		// If we have a progress bar, update it
		if p.bar != nil {
			p.bar.SetMessage(fmt.Sprintf("[%s] %s", phase, message))
			if total > 0 && current <= total {
				p.bar.Update(current)
			}
			return
		}

		// Otherwise use carriage return for inline update
		if total > 0 {
			fmt.Printf("\r\033[K[%s] %d/%d %s", phase, current, total, message)
		} else {
			fmt.Printf("\r\033[K[%s] %s", phase, message)
		}
	}
}

// CompleteCallback returns a callback function for completion events
func (p *CLIProgress) CompleteCallback() func(testFile string, count int) {
	return func(testFile string, count int) {
		if p.quiet {
			return
		}
		// Print on new line to not overwrite progress
		fmt.Printf("\n✓ Written: %s (%d tests)\n", testFile, count)
	}
}

// WithSpinner runs a function with a spinner
func (p *CLIProgress) WithSpinner(message string, fn func() error) error {
	if p.quiet {
		return fn()
	}

	p.StartSpinner(message)
	err := fn()
	if err != nil {
		p.StopSpinnerFail(fmt.Sprintf("%s: %v", message, err))
	} else {
		p.StopSpinnerSuccess(message)
	}
	return err
}
