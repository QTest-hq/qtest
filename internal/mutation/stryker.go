package mutation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// StrykerTool implements mutation testing using Stryker for JS/TS
type StrykerTool struct {
	// BinaryPath is the path to stryker binary (default: npx stryker)
	BinaryPath string
	// UseNpx uses npx to run stryker
	UseNpx bool
}

// NewStrykerTool creates a new Stryker mutation testing tool
func NewStrykerTool() *StrykerTool {
	return &StrykerTool{
		BinaryPath: "stryker",
		UseNpx:     true, // Default to using npx
	}
}

// Name returns the tool name
func (t *StrykerTool) Name() string {
	return "stryker"
}

// IsAvailable checks if Stryker is installed and available
func (t *StrykerTool) IsAvailable(ctx context.Context) bool {
	// Check if npx/node is available
	if t.UseNpx {
		cmd := exec.CommandContext(ctx, "npx", "--version")
		if err := cmd.Run(); err != nil {
			return false
		}
	}

	// Check if stryker is available (either globally or via npx)
	var cmd *exec.Cmd
	if t.UseNpx {
		cmd = exec.CommandContext(ctx, "npx", "@stryker-mutator/core", "--version")
	} else {
		cmd = exec.CommandContext(ctx, t.BinaryPath, "--version")
	}
	err := cmd.Run()
	return err == nil
}

// Run executes Stryker mutation testing on the source file
func (t *StrykerTool) Run(ctx context.Context, sourceFile, testFile string, cfg MutationConfig) (*Result, error) {
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

	// Get the project directory (where package.json should be)
	projectDir := findProjectRoot(filepath.Dir(sourceFile))

	// Create a temporary stryker config for this specific file
	strykerConfig := t.createStrykerConfig(sourceFile, testFile, cfg)

	configPath := filepath.Join(projectDir, ".stryker-tmp.json")
	if err := os.WriteFile(configPath, strykerConfig, 0644); err != nil {
		result.Error = fmt.Sprintf("failed to write stryker config: %v", err)
		return result, nil
	}
	defer os.Remove(configPath)

	// Build stryker command
	var cmd *exec.Cmd
	if t.UseNpx {
		cmd = exec.CommandContext(ctx, "npx", "@stryker-mutator/core", "run", "--configFile", configPath)
	} else {
		cmd = exec.CommandContext(ctx, t.BinaryPath, "run", "--configFile", configPath)
	}
	cmd.Dir = projectDir

	log.Debug().
		Str("source", sourceFile).
		Str("test", testFile).
		Str("dir", projectDir).
		Msg("running stryker mutation testing")

	// Run stryker
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	result.Duration = time.Since(start)

	if ctx.Err() == context.DeadlineExceeded {
		result.Error = "mutation testing timed out"
		return result, nil
	}

	// Try to parse JSON report
	reportPath := filepath.Join(projectDir, "reports", "mutation", "mutation.json")
	if _, statErr := os.Stat(reportPath); statErr == nil {
		if parseErr := t.parseStrykerReport(reportPath, result); parseErr != nil {
			log.Warn().Err(parseErr).Msg("failed to parse stryker report, falling back to output parsing")
			t.parseStrykerOutput(outputStr, result)
		}
	} else {
		t.parseStrykerOutput(outputStr, result)
	}

	// Clean up report directory
	os.RemoveAll(filepath.Join(projectDir, "reports"))

	// If we couldn't parse any results and there was an error, report it
	if result.Total == 0 && err != nil {
		result.Error = fmt.Sprintf("stryker failed: %v\nOutput: %s", err, truncateOutput(outputStr, 500))
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
		Msg("stryker mutation testing complete")

	return result, nil
}

// createStrykerConfig creates a temporary Stryker configuration for the specific files
func (t *StrykerTool) createStrykerConfig(sourceFile, testFile string, cfg MutationConfig) []byte {
	// Determine test runner and mutators based on file extension
	testRunner := "jest"
	if strings.HasSuffix(testFile, ".spec.ts") || strings.HasSuffix(testFile, ".test.ts") {
		testRunner = "jest"
	} else if strings.HasSuffix(testFile, "_test.js") || strings.HasSuffix(testFile, ".test.js") {
		testRunner = "jest"
	}

	// Get relative paths
	srcRel := filepath.Base(sourceFile)
	testRel := filepath.Base(testFile)

	config := map[string]interface{}{
		"$schema":     "https://raw.githubusercontent.com/stryker-mutator/stryker/master/packages/core/schema/stryker-schema.json",
		"packageManager": "npm",
		"testRunner":  testRunner,
		"coverageAnalysis": "perTest",
		"mutate": []string{
			srcRel,
		},
		"reporters": []string{"json", "clear-text"},
		"logLevel": "warn",
		"timeoutMS": int(cfg.TimeoutPerMutant.Milliseconds()),
		"maxConcurrentTestRunners": 2,
	}

	// Set max mutants if configured
	if cfg.MaxMutantsPerFunction > 0 {
		config["maxMutantsPerFile"] = cfg.MaxMutantsPerFunction * 5 // Estimate 5 functions per file
	}

	// Limit to specific test file
	config["testFilter"] = []string{testRel}

	data, _ := json.MarshalIndent(config, "", "  ")
	return data
}

// StrykerReport represents the Stryker JSON report format
type StrykerReport struct {
	SchemaVersion string `json:"schemaVersion"`
	Thresholds    struct {
		High  int `json:"high"`
		Low   int `json:"low"`
		Break int `json:"break"`
	} `json:"thresholds"`
	Files map[string]StrykerFileResult `json:"files"`
}

// StrykerFileResult contains mutation results for a single file
type StrykerFileResult struct {
	Language string `json:"language"`
	Source   string `json:"source"`
	Mutants  []StrykerMutant `json:"mutants"`
}

// StrykerMutant represents a single mutant in Stryker output
type StrykerMutant struct {
	ID          string `json:"id"`
	MutatorName string `json:"mutatorName"`
	Replacement string `json:"replacement"`
	Status      string `json:"status"` // "Killed", "Survived", "Timeout", "NoCoverage"
	Location    struct {
		Start struct {
			Line   int `json:"line"`
			Column int `json:"column"`
		} `json:"start"`
		End struct {
			Line   int `json:"line"`
			Column int `json:"column"`
		} `json:"end"`
	} `json:"location"`
	Description string `json:"description"`
}

// parseStrykerReport parses the JSON mutation report
func (t *StrykerTool) parseStrykerReport(reportPath string, result *Result) error {
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return err
	}

	var report StrykerReport
	if err := json.Unmarshal(data, &report); err != nil {
		return err
	}

	for _, fileResult := range report.Files {
		for _, m := range fileResult.Mutants {
			mutant := Mutant{
				ID:          m.ID,
				Type:        t.mapMutatorName(m.MutatorName),
				Description: m.Description,
				Line:        m.Location.Start.Line,
				Original:    "",
				Mutated:     m.Replacement,
			}

			switch m.Status {
			case "Killed":
				mutant.Status = StatusKilled
				result.Killed++
			case "Survived", "NoCoverage":
				mutant.Status = StatusSurvived
				result.Survived++
			case "Timeout":
				mutant.Status = StatusTimeout
				result.Timeout++
			default:
				mutant.Status = StatusError
			}

			result.Mutants = append(result.Mutants, mutant)
			result.Total++
		}
	}

	return nil
}

// parseStrykerOutput parses Stryker CLI output when JSON report isn't available
func (t *StrykerTool) parseStrykerOutput(output string, result *Result) {
	/*
		Stryker output format:
		Mutation testing  [====================] 100% (elapsed: 10s, remaining: ~0s) 20/20 tested (10 survived, 0 timed out)

		Or:
		All tests...
		Killed:   15
		Survived: 5
		Timeout:  0
		Runtime:  X seconds
	*/

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Try to parse summary line
		if strings.Contains(line, "Killed:") {
			fmt.Sscanf(line, "Killed: %d", &result.Killed)
		}
		if strings.Contains(line, "Survived:") {
			fmt.Sscanf(line, "Survived: %d", &result.Survived)
		}
		if strings.Contains(line, "Timeout:") || strings.Contains(line, "TimedOut:") {
			fmt.Sscanf(line, "Timeout: %d", &result.Timeout)
		}

		// Try to parse progress line
		if strings.Contains(line, "tested") && strings.Contains(line, "survived") {
			// Format: Mutation testing ... 20/20 tested (10 survived, 0 timed out)
			// Find the start of the "N/N tested" pattern
			testedIdx := strings.Index(line, "tested")
			if testedIdx > 0 {
				// Look backwards to find where the numbers start
				progressPart := line[:testedIdx]
				// Find last space before "tested" to get "20/20 "
				lastSpace := strings.LastIndex(strings.TrimRight(progressPart, " "), " ")
				if lastSpace >= 0 {
					progressPart = line[lastSpace+1:]
				}
				var total, survived, timeout int
				n, _ := fmt.Sscanf(progressPart, "%d/%d tested (%d survived, %d timed out)",
					&total, &total, &survived, &timeout)
				if n >= 3 {
					result.Total = total
					result.Survived = survived
					result.Timeout = timeout
					result.Killed = total - survived - timeout
				}
			}
		}
	}

	// Calculate total if not found
	if result.Total == 0 {
		result.Total = result.Killed + result.Survived + result.Timeout
	}
}

// mapMutatorName maps Stryker mutator names to our generic types
func (t *StrykerTool) mapMutatorName(name string) string {
	switch name {
	case "ArithmeticOperator", "UnaryOperator":
		return "arithmetic"
	case "EqualityOperator", "RelationalOperator":
		return "comparison"
	case "LogicalOperator", "BooleanLiteral":
		return "boolean"
	case "BlockStatement", "ConditionalExpression":
		return "branch"
	case "StringLiteral", "ArrayDeclaration":
		return "literal"
	case "MethodExpression", "ObjectLiteral":
		return "statement"
	default:
		return "unknown"
	}
}

// findProjectRoot walks up directories to find package.json
func findProjectRoot(start string) string {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root, return original directory
			return start
		}
		dir = parent
	}
}

// truncateOutput truncates output string to maxLen
func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... (truncated)"
}
