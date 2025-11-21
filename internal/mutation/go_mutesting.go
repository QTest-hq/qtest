package mutation

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// GoMutestingTool implements mutation testing using go-mutesting
type GoMutestingTool struct {
	// BinaryPath is the path to go-mutesting binary (default: go-mutesting in PATH)
	BinaryPath string
}

// NewGoMutestingTool creates a new go-mutesting tool
func NewGoMutestingTool() *GoMutestingTool {
	return &GoMutestingTool{
		BinaryPath: "go-mutesting",
	}
}

// Name returns the tool name
func (t *GoMutestingTool) Name() string {
	return "go-mutesting"
}

// IsAvailable checks if go-mutesting is installed
func (t *GoMutestingTool) IsAvailable(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, t.BinaryPath, "--help")
	err := cmd.Run()
	return err == nil
}

// Run executes mutation testing on the source file
func (t *GoMutestingTool) Run(ctx context.Context, sourceFile, testFile string, cfg MutationConfig) (*Result, error) {
	start := time.Now()

	result := &Result{
		SourceFile: sourceFile,
		TestFile:   testFile,
	}

	// Create context with timeout
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}

	// Get the package directory
	pkgDir := filepath.Dir(sourceFile)

	// Build go-mutesting command
	// go-mutesting runs on packages, not individual files
	args := []string{
		"--verbose",
	}

	// Add timeout per mutant if specified
	if cfg.TimeoutPerMutant > 0 {
		args = append(args, "--exec-timeout", cfg.TimeoutPerMutant.String())
	}

	// Target the specific file
	args = append(args, sourceFile)

	log.Debug().
		Str("binary", t.BinaryPath).
		Strs("args", args).
		Str("dir", pkgDir).
		Msg("running go-mutesting")

	cmd := exec.CommandContext(ctx, t.BinaryPath, args...)
	cmd.Dir = pkgDir

	// Capture output
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	result.Duration = time.Since(start)

	if ctx.Err() == context.DeadlineExceeded {
		result.Error = "mutation testing timed out"
		return result, nil
	}

	// Parse the output even if there's an error (go-mutesting returns non-zero on surviving mutants)
	parseGoMutestingOutput(outputStr, result)

	// If we couldn't parse any results and there was an error, report it
	if result.Total == 0 && err != nil {
		result.Error = fmt.Sprintf("go-mutesting failed: %v\nOutput: %s", err, outputStr)
	}

	// Calculate score
	if result.Total > 0 {
		result.Score = float64(result.Killed) / float64(result.Total)
	}

	log.Info().
		Str("source", sourceFile).
		Int("total", result.Total).
		Int("killed", result.Killed).
		Int("survived", result.Survived).
		Float64("score", result.Score).
		Dur("duration", result.Duration).
		Msg("mutation testing complete")

	return result, nil
}

// parseGoMutestingOutput parses go-mutesting verbose output
func parseGoMutestingOutput(output string, result *Result) {
	/*
		go-mutesting output format (verbose):
		PASS: path/to/file.go:42: Replaced != with ==
		FAIL: path/to/file.go:55: Replaced + with -

		Summary:
		x mutants passed testing
		y mutants did not pass testing
	*/

	scanner := bufio.NewScanner(strings.NewReader(output))

	// Patterns for parsing
	passPattern := regexp.MustCompile(`^PASS:\s+(.+?):(\d+):\s+(.+)$`)
	failPattern := regexp.MustCompile(`^FAIL:\s+(.+?):(\d+):\s+(.+)$`)
	skipPattern := regexp.MustCompile(`^SKIP:\s+(.+?):(\d+):\s+(.+)$`)

	for scanner.Scan() {
		line := scanner.Text()

		// Parse individual mutant results
		if matches := passPattern.FindStringSubmatch(line); matches != nil {
			// PASS means test killed the mutant
			lineNum, _ := strconv.Atoi(matches[2])
			result.Mutants = append(result.Mutants, Mutant{
				ID:          fmt.Sprintf("mutant-%d", len(result.Mutants)+1),
				Description: matches[3],
				Line:        lineNum,
				Status:      StatusKilled,
				Type:        inferMutationType(matches[3]),
			})
			result.Killed++
			result.Total++
			continue
		}

		if matches := failPattern.FindStringSubmatch(line); matches != nil {
			// FAIL means mutant survived
			lineNum, _ := strconv.Atoi(matches[2])
			result.Mutants = append(result.Mutants, Mutant{
				ID:          fmt.Sprintf("mutant-%d", len(result.Mutants)+1),
				Description: matches[3],
				Line:        lineNum,
				Status:      StatusSurvived,
				Type:        inferMutationType(matches[3]),
			})
			result.Survived++
			result.Total++
			continue
		}

		if matches := skipPattern.FindStringSubmatch(line); matches != nil {
			// SKIP means mutant timed out or error
			lineNum, _ := strconv.Atoi(matches[2])
			result.Mutants = append(result.Mutants, Mutant{
				ID:          fmt.Sprintf("mutant-%d", len(result.Mutants)+1),
				Description: matches[3],
				Line:        lineNum,
				Status:      StatusTimeout,
				Type:        inferMutationType(matches[3]),
			})
			result.Timeout++
			result.Total++
			continue
		}
	}

	// If no mutants parsed but output contains summary, try to extract counts
	if result.Total == 0 {
		parseSummary(output, result)
	}
}

// parseSummary extracts mutation counts from summary section
func parseSummary(output string, result *Result) {
	// Look for patterns like "X mutants passed" and "Y mutants did not pass"
	passedPattern := regexp.MustCompile(`(\d+)\s+mutants?\s+passed`)
	failedPattern := regexp.MustCompile(`(\d+)\s+mutants?\s+did\s+not\s+pass`)

	if matches := passedPattern.FindStringSubmatch(output); matches != nil {
		result.Killed, _ = strconv.Atoi(matches[1])
	}
	if matches := failedPattern.FindStringSubmatch(output); matches != nil {
		result.Survived, _ = strconv.Atoi(matches[1])
	}

	result.Total = result.Killed + result.Survived + result.Timeout
}

// inferMutationType infers the mutation type from the description
func inferMutationType(description string) string {
	desc := strings.ToLower(description)

	switch {
	case strings.Contains(desc, "replaced") && (strings.Contains(desc, "+") || strings.Contains(desc, "-") ||
		strings.Contains(desc, "*") || strings.Contains(desc, "/")):
		return "arithmetic"
	case strings.Contains(desc, "replaced") && (strings.Contains(desc, "==") || strings.Contains(desc, "!=") ||
		strings.Contains(desc, "<") || strings.Contains(desc, ">")):
		return "comparison"
	case strings.Contains(desc, "replaced") && (strings.Contains(desc, "&&") || strings.Contains(desc, "||") ||
		strings.Contains(desc, "true") || strings.Contains(desc, "false")):
		return "boolean"
	case strings.Contains(desc, "return"):
		return "return"
	case strings.Contains(desc, "removed"):
		return "statement"
	case strings.Contains(desc, "branch"):
		return "branch"
	default:
		return "unknown"
	}
}

// SimpleMutationTool is a simple mutation testing implementation that doesn't require external tools
// It performs basic mutations and runs tests to check if they detect the changes
type SimpleMutationTool struct{}

// NewSimpleMutationTool creates a new simple mutation tool
func NewSimpleMutationTool() *SimpleMutationTool {
	return &SimpleMutationTool{}
}

// Name returns the tool name
func (t *SimpleMutationTool) Name() string {
	return "simple"
}

// IsAvailable always returns true as this tool has no external dependencies
func (t *SimpleMutationTool) IsAvailable(ctx context.Context) bool {
	return true
}

// Run performs simple mutation testing by running tests multiple times
// This is a fallback when go-mutesting is not available
func (t *SimpleMutationTool) Run(ctx context.Context, sourceFile, testFile string, cfg MutationConfig) (*Result, error) {
	start := time.Now()

	result := &Result{
		SourceFile: sourceFile,
		TestFile:   testFile,
	}

	// Get package directory
	pkgDir := filepath.Dir(sourceFile)

	// Run tests normally first to ensure they pass
	cmd := exec.CommandContext(ctx, "go", "test", "-v", "-count=1", "./...")
	cmd.Dir = pkgDir

	output, err := cmd.CombinedOutput()
	result.Duration = time.Since(start)

	if err != nil {
		// Tests don't pass, can't do mutation testing
		result.Error = fmt.Sprintf("tests must pass before mutation testing: %s", string(output))
		return result, nil
	}

	// Since we can't easily mutate code without go-mutesting,
	// return a result indicating the tool isn't fully functional
	result.Error = "simple mutation tool: go-mutesting not available, tests pass but mutation score unknown"
	result.Total = 0
	result.Killed = 0
	result.Survived = 0
	result.Score = 0

	return result, nil
}
