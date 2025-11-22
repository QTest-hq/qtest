// Package validator provides test validation and quality analysis
package validator

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// CoverageResult holds code coverage analysis results
type CoverageResult struct {
	TotalCoverage       float64                  `json:"total_coverage"`
	TargetFuncCoverage  float64                  `json:"target_func_coverage"`
	TargetFuncCovered   bool                     `json:"target_func_covered"`
	FileCoverage        map[string]float64       `json:"file_coverage"`
	FunctionCoverage    map[string]FuncCoverage  `json:"function_coverage,omitempty"`
	UncoveredLines      []int                    `json:"uncovered_lines,omitempty"`
	CoverageReport      string                   `json:"coverage_report,omitempty"`
}

// FuncCoverage holds coverage for a specific function
type FuncCoverage struct {
	Name         string  `json:"name"`
	Covered      bool    `json:"covered"`
	Coverage     float64 `json:"coverage"`
	TotalLines   int     `json:"total_lines"`
	CoveredLines int     `json:"covered_lines"`
}

// CoverageChecker runs tests with coverage and analyzes results
type CoverageChecker struct {
	workDir        string
	language       string
	targetFile     string
	targetFunction string
}

// NewCoverageChecker creates a new coverage checker
func NewCoverageChecker(workDir, language, targetFile, targetFunction string) *CoverageChecker {
	return &CoverageChecker{
		workDir:        workDir,
		language:       language,
		targetFile:     targetFile,
		targetFunction: targetFunction,
	}
}

// RunWithCoverage executes tests with coverage collection
func (c *CoverageChecker) RunWithCoverage(ctx context.Context, testFile string) (*CoverageResult, error) {
	switch c.language {
	case "go":
		return c.runGoCoverage(ctx, testFile)
	case "python":
		return c.runPythonCoverage(ctx, testFile)
	case "javascript", "typescript":
		return c.runJSCoverage(ctx, testFile)
	default:
		return nil, fmt.Errorf("unsupported language for coverage: %s", c.language)
	}
}

// runGoCoverage runs Go tests with coverage
func (c *CoverageChecker) runGoCoverage(ctx context.Context, testFile string) (*CoverageResult, error) {
	testDir := filepath.Dir(testFile)
	coverFile := filepath.Join(testDir, "coverage.out")

	// Run tests with coverage
	cmd := exec.CommandContext(ctx, "go", "test", "-v", "-coverprofile="+coverFile, "-covermode=count", "./...")
	cmd.Dir = testDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Tests failed but we may still have coverage data
		log.Debug().Err(err).Str("output", string(output)).Msg("tests failed during coverage run")
	}

	result := &CoverageResult{
		FileCoverage:     make(map[string]float64),
		FunctionCoverage: make(map[string]FuncCoverage),
	}

	// Parse coverage output
	cmd = exec.CommandContext(ctx, "go", "tool", "cover", "-func="+coverFile)
	cmd.Dir = testDir

	coverOutput, err := cmd.CombinedOutput()
	if err != nil {
		return result, fmt.Errorf("failed to analyze coverage: %w", err)
	}

	result.CoverageReport = string(coverOutput)
	c.parseGoCoverage(string(coverOutput), result)

	// Check if target function is covered
	if c.targetFunction != "" {
		if fc, ok := result.FunctionCoverage[c.targetFunction]; ok {
			result.TargetFuncCovered = fc.Covered
			result.TargetFuncCoverage = fc.Coverage
		}
	}

	return result, nil
}

// parseGoCoverage parses go tool cover -func output
func (c *CoverageChecker) parseGoCoverage(output string, result *CoverageResult) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	funcPattern := regexp.MustCompile(`^(.+):(\d+):\s+(\w+)\s+(\d+\.?\d*)%`)
	totalPattern := regexp.MustCompile(`^total:\s+\(statements\)\s+(\d+\.?\d*)%`)

	for scanner.Scan() {
		line := scanner.Text()

		// Check for total coverage
		if matches := totalPattern.FindStringSubmatch(line); len(matches) > 1 {
			if pct, err := strconv.ParseFloat(matches[1], 64); err == nil {
				result.TotalCoverage = pct
			}
			continue
		}

		// Check for function coverage
		if matches := funcPattern.FindStringSubmatch(line); len(matches) > 4 {
			fileName := matches[1]
			funcName := matches[3]
			pct, _ := strconv.ParseFloat(matches[4], 64)

			result.FunctionCoverage[funcName] = FuncCoverage{
				Name:     funcName,
				Covered:  pct > 0,
				Coverage: pct,
			}

			// Track file coverage (use highest function coverage per file)
			if existing, ok := result.FileCoverage[fileName]; !ok || pct > existing {
				result.FileCoverage[fileName] = pct
			}
		}
	}
}

// runPythonCoverage runs Python tests with coverage
func (c *CoverageChecker) runPythonCoverage(ctx context.Context, testFile string) (*CoverageResult, error) {
	testDir := filepath.Dir(testFile)

	// Run pytest with coverage
	cmd := exec.CommandContext(ctx, "pytest", testFile, "--cov=.", "--cov-report=json", "-v")
	cmd.Dir = testDir

	output, _ := cmd.CombinedOutput()

	result := &CoverageResult{
		FileCoverage:     make(map[string]float64),
		FunctionCoverage: make(map[string]FuncCoverage),
		CoverageReport:   string(output),
	}

	// Try to parse coverage.json if it exists
	coverageJSON := filepath.Join(testDir, "coverage.json")
	c.parsePythonCoverageJSON(coverageJSON, result)

	return result, nil
}

// parsePythonCoverageJSON parses Python coverage.json output
func (c *CoverageChecker) parsePythonCoverageJSON(jsonFile string, result *CoverageResult) {
	// Read and parse coverage.json
	// Structure: {"totals": {"percent_covered": 85.0}, "files": {...}}
	type PythonCoverage struct {
		Totals struct {
			PercentCovered float64 `json:"percent_covered"`
		} `json:"totals"`
		Files map[string]struct {
			Summary struct {
				PercentCovered float64 `json:"percent_covered"`
			} `json:"summary"`
			MissingLines []int `json:"missing_lines"`
		} `json:"files"`
	}

	cmd := exec.Command("cat", jsonFile)
	output, err := cmd.Output()
	if err != nil {
		return
	}

	var cov PythonCoverage
	if err := json.Unmarshal(output, &cov); err != nil {
		return
	}

	result.TotalCoverage = cov.Totals.PercentCovered

	for fileName, fileData := range cov.Files {
		result.FileCoverage[fileName] = fileData.Summary.PercentCovered

		// Check if this is the target file
		if c.targetFile != "" && strings.HasSuffix(fileName, c.targetFile) {
			result.TargetFuncCoverage = fileData.Summary.PercentCovered
			result.TargetFuncCovered = fileData.Summary.PercentCovered > 0
			result.UncoveredLines = fileData.MissingLines
		}
	}
}

// runJSCoverage runs JavaScript tests with coverage
func (c *CoverageChecker) runJSCoverage(ctx context.Context, testFile string) (*CoverageResult, error) {
	testDir := filepath.Dir(testFile)

	// Run jest with coverage
	cmd := exec.CommandContext(ctx, "npx", "jest", testFile, "--coverage", "--coverageReporters=json-summary")
	cmd.Dir = testDir

	output, _ := cmd.CombinedOutput()

	result := &CoverageResult{
		FileCoverage:     make(map[string]float64),
		FunctionCoverage: make(map[string]FuncCoverage),
		CoverageReport:   string(output),
	}

	// Try to parse coverage-summary.json
	coverageJSON := filepath.Join(testDir, "coverage", "coverage-summary.json")
	c.parseJSCoverageJSON(coverageJSON, result)

	return result, nil
}

// parseJSCoverageJSON parses Jest coverage-summary.json
func (c *CoverageChecker) parseJSCoverageJSON(jsonFile string, result *CoverageResult) {
	// Structure: {"total": {"lines": {"pct": 85}}, "/path/file.js": {...}}
	type JSCoverage map[string]struct {
		Lines struct {
			Pct float64 `json:"pct"`
		} `json:"lines"`
		Functions struct {
			Pct float64 `json:"pct"`
		} `json:"functions"`
	}

	cmd := exec.Command("cat", jsonFile)
	output, err := cmd.Output()
	if err != nil {
		return
	}

	var cov JSCoverage
	if err := json.Unmarshal(output, &cov); err != nil {
		return
	}

	if total, ok := cov["total"]; ok {
		result.TotalCoverage = total.Lines.Pct
	}

	for fileName, fileData := range cov {
		if fileName == "total" {
			continue
		}
		result.FileCoverage[fileName] = fileData.Lines.Pct

		if c.targetFile != "" && strings.HasSuffix(fileName, c.targetFile) {
			result.TargetFuncCoverage = fileData.Lines.Pct
			result.TargetFuncCovered = fileData.Lines.Pct > 0
		}
	}
}

// ValidateCoverage checks if coverage meets thresholds
func (c *CoverageChecker) ValidateCoverage(result *CoverageResult, minTotalCoverage, minTargetCoverage float64) (bool, []string) {
	var failures []string

	// Check total coverage
	if result.TotalCoverage < minTotalCoverage {
		failures = append(failures, fmt.Sprintf(
			"Total coverage %.1f%% is below minimum %.1f%%",
			result.TotalCoverage, minTotalCoverage,
		))
	}

	// Check target function coverage
	if c.targetFunction != "" {
		if !result.TargetFuncCovered {
			failures = append(failures, fmt.Sprintf(
				"Target function '%s' is not covered by tests",
				c.targetFunction,
			))
		} else if result.TargetFuncCoverage < minTargetCoverage {
			failures = append(failures, fmt.Sprintf(
				"Target function coverage %.1f%% is below minimum %.1f%%",
				result.TargetFuncCoverage, minTargetCoverage,
			))
		}
	}

	return len(failures) == 0, failures
}

// CoverageConfig holds coverage validation configuration
type CoverageConfig struct {
	MinTotalCoverage    float64       `json:"min_total_coverage"`
	MinTargetCoverage   float64       `json:"min_target_coverage"`
	Timeout             time.Duration `json:"timeout"`
	SkipOnError         bool          `json:"skip_on_error"`
}

// DefaultCoverageConfig returns default coverage thresholds
func DefaultCoverageConfig() CoverageConfig {
	return CoverageConfig{
		MinTotalCoverage:  50.0,  // At least 50% overall
		MinTargetCoverage: 80.0,  // At least 80% of target function
		Timeout:           5 * time.Minute,
		SkipOnError:       true,  // Don't fail if coverage tool errors
	}
}
