package codecov

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

// CoverageReport holds coverage data for a project
type CoverageReport struct {
	Timestamp    time.Time       `json:"timestamp"`
	Language     string          `json:"language"`
	TotalLines   int             `json:"total_lines"`
	CoveredLines int             `json:"covered_lines"`
	Percentage   float64         `json:"percentage"`
	Files        []FileCoverage  `json:"files"`
	Uncovered    []UncoveredItem `json:"uncovered"`
}

// FileCoverage holds coverage data for a single file
type FileCoverage struct {
	Path           string  `json:"path"`
	TotalLines     int     `json:"total_lines"`
	CoveredLines   int     `json:"covered_lines"`
	Percentage     float64 `json:"percentage"`
	UncoveredLines []int   `json:"uncovered_lines"`
}

// UncoveredItem represents an uncovered code section
type UncoveredItem struct {
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Type      string `json:"type"` // "function", "branch", "line"
	Name      string `json:"name,omitempty"`
}

// Collector collects code coverage from test runs
type Collector struct {
	workDir  string
	language string
}

// NewCollector creates a coverage collector
func NewCollector(workDir, language string) *Collector {
	return &Collector{
		workDir:  workDir,
		language: language,
	}
}

// Collect runs tests with coverage and returns a report
func (c *Collector) Collect(ctx context.Context) (*CoverageReport, error) {
	switch c.language {
	case "go":
		return c.collectGoCoverage(ctx)
	case "python":
		return c.collectPythonCoverage(ctx)
	case "javascript", "typescript":
		return c.collectJSCoverage(ctx)
	default:
		return nil, fmt.Errorf("unsupported language for coverage: %s", c.language)
	}
}

// collectGoCoverage runs go test with coverage
func (c *Collector) collectGoCoverage(ctx context.Context) (*CoverageReport, error) {
	coverFile := filepath.Join(c.workDir, "coverage.out")

	// Run go test with coverage
	cmd := exec.CommandContext(ctx, "go", "test", "-coverprofile="+coverFile, "-covermode=count", "./...")
	cmd.Dir = c.workDir
	output, err := cmd.CombinedOutput()

	log.Debug().Str("output", string(output)).Msg("go test coverage output")

	if err != nil {
		// Tests might fail but still produce coverage
		if _, statErr := os.Stat(coverFile); os.IsNotExist(statErr) {
			return nil, fmt.Errorf("coverage collection failed: %w", err)
		}
	}

	// Parse coverage file
	report, err := c.parseGoCoverage(coverFile)
	if err != nil {
		return nil, err
	}

	// Clean up
	os.Remove(coverFile)

	return report, nil
}

// parseGoCoverage parses Go coverage profile
func (c *Collector) parseGoCoverage(coverFile string) (*CoverageReport, error) {
	file, err := os.Open(coverFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	report := &CoverageReport{
		Timestamp: time.Now(),
		Language:  "go",
		Files:     make([]FileCoverage, 0),
		Uncovered: make([]UncoveredItem, 0),
	}

	fileMap := make(map[string]*FileCoverage)
	scanner := bufio.NewScanner(file)

	// Skip mode line
	scanner.Scan()

	// Parse coverage lines: file:startLine.startCol,endLine.endCol numStmt count
	lineRegex := regexp.MustCompile(`^(.+):(\d+)\.\d+,(\d+)\.\d+ (\d+) (\d+)$`)

	for scanner.Scan() {
		line := scanner.Text()
		matches := lineRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		filePath := matches[1]
		startLine, _ := strconv.Atoi(matches[2])
		endLine, _ := strconv.Atoi(matches[3])
		numStmt, _ := strconv.Atoi(matches[4])
		count, _ := strconv.Atoi(matches[5])

		// Get or create file coverage
		fc, ok := fileMap[filePath]
		if !ok {
			fc = &FileCoverage{
				Path:           filePath,
				UncoveredLines: make([]int, 0),
			}
			fileMap[filePath] = fc
		}

		fc.TotalLines += numStmt
		if count > 0 {
			fc.CoveredLines += numStmt
		} else {
			// Track uncovered lines
			for l := startLine; l <= endLine; l++ {
				fc.UncoveredLines = append(fc.UncoveredLines, l)
			}
			report.Uncovered = append(report.Uncovered, UncoveredItem{
				File:      filePath,
				StartLine: startLine,
				EndLine:   endLine,
				Type:      "line",
			})
		}
	}

	// Calculate totals
	for _, fc := range fileMap {
		if fc.TotalLines > 0 {
			fc.Percentage = float64(fc.CoveredLines) / float64(fc.TotalLines) * 100
		}
		report.Files = append(report.Files, *fc)
		report.TotalLines += fc.TotalLines
		report.CoveredLines += fc.CoveredLines
	}

	if report.TotalLines > 0 {
		report.Percentage = float64(report.CoveredLines) / float64(report.TotalLines) * 100
	}

	return report, nil
}

// collectPythonCoverage runs pytest with coverage
func (c *Collector) collectPythonCoverage(ctx context.Context) (*CoverageReport, error) {
	coverFile := filepath.Join(c.workDir, "coverage.json")

	// Run pytest with coverage
	cmd := exec.CommandContext(ctx, "pytest", "--cov=.", "--cov-report=json:"+coverFile, "-q")
	cmd.Dir = c.workDir
	output, err := cmd.CombinedOutput()

	log.Debug().Str("output", string(output)).Msg("pytest coverage output")

	if err != nil {
		if _, statErr := os.Stat(coverFile); os.IsNotExist(statErr) {
			return nil, fmt.Errorf("coverage collection failed: %w", err)
		}
	}

	// Parse coverage JSON
	report, err := c.parsePythonCoverage(coverFile)
	if err != nil {
		return nil, err
	}

	// Clean up
	os.Remove(coverFile)

	return report, nil
}

// PytestCoverageJSON represents pytest-cov JSON output
type PytestCoverageJSON struct {
	Totals struct {
		CoveredLines   int     `json:"covered_lines"`
		NumStatements  int     `json:"num_statements"`
		PercentCovered float64 `json:"percent_covered"`
	} `json:"totals"`
	Files map[string]struct {
		Summary struct {
			CoveredLines   int     `json:"covered_lines"`
			NumStatements  int     `json:"num_statements"`
			PercentCovered float64 `json:"percent_covered"`
		} `json:"summary"`
		MissingLines []int `json:"missing_lines"`
	} `json:"files"`
}

func (c *Collector) parsePythonCoverage(coverFile string) (*CoverageReport, error) {
	data, err := os.ReadFile(coverFile)
	if err != nil {
		return nil, err
	}

	var pyCov PytestCoverageJSON
	if err := json.Unmarshal(data, &pyCov); err != nil {
		return nil, err
	}

	report := &CoverageReport{
		Timestamp:    time.Now(),
		Language:     "python",
		TotalLines:   pyCov.Totals.NumStatements,
		CoveredLines: pyCov.Totals.CoveredLines,
		Percentage:   pyCov.Totals.PercentCovered,
		Files:        make([]FileCoverage, 0),
		Uncovered:    make([]UncoveredItem, 0),
	}

	for filePath, fileCov := range pyCov.Files {
		fc := FileCoverage{
			Path:           filePath,
			TotalLines:     fileCov.Summary.NumStatements,
			CoveredLines:   fileCov.Summary.CoveredLines,
			Percentage:     fileCov.Summary.PercentCovered,
			UncoveredLines: fileCov.MissingLines,
		}
		report.Files = append(report.Files, fc)

		// Create uncovered items for missing lines
		for _, line := range fileCov.MissingLines {
			report.Uncovered = append(report.Uncovered, UncoveredItem{
				File:      filePath,
				StartLine: line,
				EndLine:   line,
				Type:      "line",
			})
		}
	}

	return report, nil
}

// collectJSCoverage runs jest with coverage
func (c *Collector) collectJSCoverage(ctx context.Context) (*CoverageReport, error) {
	coverDir := filepath.Join(c.workDir, "coverage")

	// Run jest with coverage
	cmd := exec.CommandContext(ctx, "npx", "jest", "--coverage", "--coverageReporters=json-summary", "--coverageDirectory="+coverDir)
	cmd.Dir = c.workDir
	output, err := cmd.CombinedOutput()

	log.Debug().Str("output", string(output)).Msg("jest coverage output")

	coverFile := filepath.Join(coverDir, "coverage-summary.json")
	if err != nil {
		if _, statErr := os.Stat(coverFile); os.IsNotExist(statErr) {
			return nil, fmt.Errorf("coverage collection failed: %w", err)
		}
	}

	// Parse coverage JSON
	report, err := c.parseJSCoverage(coverFile)
	if err != nil {
		return nil, err
	}

	// Clean up
	os.RemoveAll(coverDir)

	return report, nil
}

// JestCoverageSummary represents Jest coverage-summary.json
type JestCoverageSummary struct {
	Total map[string]struct {
		Total   int     `json:"total"`
		Covered int     `json:"covered"`
		Skipped int     `json:"skipped"`
		Pct     float64 `json:"pct"`
	} `json:"total"`
}

func (c *Collector) parseJSCoverage(coverFile string) (*CoverageReport, error) {
	data, err := os.ReadFile(coverFile)
	if err != nil {
		return nil, err
	}

	var jsCov map[string]interface{}
	if err := json.Unmarshal(data, &jsCov); err != nil {
		return nil, err
	}

	report := &CoverageReport{
		Timestamp: time.Now(),
		Language:  "javascript",
		Files:     make([]FileCoverage, 0),
		Uncovered: make([]UncoveredItem, 0),
	}

	// Parse total
	if total, ok := jsCov["total"].(map[string]interface{}); ok {
		if lines, ok := total["lines"].(map[string]interface{}); ok {
			report.TotalLines = int(lines["total"].(float64))
			report.CoveredLines = int(lines["covered"].(float64))
			report.Percentage = lines["pct"].(float64)
		}
	}

	// Parse per-file coverage
	for key, val := range jsCov {
		if key == "total" {
			continue
		}

		fileCov, ok := val.(map[string]interface{})
		if !ok {
			continue
		}

		fc := FileCoverage{Path: key}
		if lines, ok := fileCov["lines"].(map[string]interface{}); ok {
			fc.TotalLines = int(lines["total"].(float64))
			fc.CoveredLines = int(lines["covered"].(float64))
			fc.Percentage = lines["pct"].(float64)
		}
		report.Files = append(report.Files, fc)
	}

	return report, nil
}

// GetUncoveredFunctions identifies functions that lack coverage
func (c *Collector) GetUncoveredFunctions(report *CoverageReport, threshold float64) []UncoveredItem {
	var uncovered []UncoveredItem

	for _, file := range report.Files {
		if file.Percentage < threshold {
			// Mark entire file as needing coverage
			uncovered = append(uncovered, UncoveredItem{
				File:      file.Path,
				StartLine: 1,
				EndLine:   file.TotalLines,
				Type:      "file",
				Name:      filepath.Base(file.Path),
			})
		}
	}

	// Also include specific uncovered items
	uncovered = append(uncovered, report.Uncovered...)

	return uncovered
}

// SaveReport saves coverage report to file
func (c *Collector) SaveReport(report *CoverageReport, outputPath string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, data, 0644)
}

// LoadReport loads a saved coverage report
func LoadReport(path string) (*CoverageReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var report CoverageReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}

	return &report, nil
}
