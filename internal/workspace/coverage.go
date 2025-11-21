package workspace

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

// CoverageCollector collects code coverage from test runs
type CoverageCollector struct {
	ws        *Workspace
	artifacts *ArtifactManager
}

// NewCoverageCollector creates a new coverage collector
func NewCoverageCollector(ws *Workspace) *CoverageCollector {
	return &CoverageCollector{
		ws:        ws,
		artifacts: NewArtifactManager(ws),
	}
}

// CollectAll runs all tests with coverage and returns results
func (c *CoverageCollector) CollectAll(ctx context.Context) (*CoverageReport, error) {
	var files []FileCoverage

	// Determine language and collect coverage accordingly
	switch c.ws.Language {
	case "go":
		goFiles, err := c.collectGoCoverage(ctx)
		if err != nil {
			return nil, fmt.Errorf("go coverage failed: %w", err)
		}
		files = goFiles
	case "python":
		pyFiles, err := c.collectPythonCoverage(ctx)
		if err != nil {
			return nil, fmt.Errorf("python coverage failed: %w", err)
		}
		files = pyFiles
	case "javascript", "typescript":
		jsFiles, err := c.collectJestCoverage(ctx)
		if err != nil {
			return nil, fmt.Errorf("jest coverage failed: %w", err)
		}
		files = jsFiles
	default:
		return nil, fmt.Errorf("unsupported language for coverage: %s", c.ws.Language)
	}

	// Generate coverage report artifact
	report, err := c.artifacts.GenerateCoverageReport(files)
	if err != nil {
		return nil, fmt.Errorf("failed to generate coverage report: %w", err)
	}

	return report, nil
}

// collectGoCoverage runs go test with coverage
func (c *CoverageCollector) collectGoCoverage(ctx context.Context) ([]FileCoverage, error) {
	coverFile := filepath.Join(c.ws.Path(), "artifacts", "coverage.out")

	// Run go test with coverage
	cmd := exec.CommandContext(ctx, "go", "test", "-coverprofile="+coverFile, "-covermode=count", "./...")
	cmd.Dir = c.ws.RepoPath

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Tests might fail but we still want coverage data
		log.Debug().Err(err).Str("stderr", stderr.String()).Msg("go test completed with errors")
	}

	// Parse coverage file
	return c.parseGoCoverage(coverFile)
}

// parseGoCoverage parses Go coverage profile
func (c *CoverageCollector) parseGoCoverage(coverFile string) ([]FileCoverage, error) {
	data, err := os.ReadFile(coverFile)
	if err != nil {
		return nil, err
	}

	// Go coverage format: mode: count
	// file:startLine.startCol,endLine.endCol statements count
	fileStats := make(map[string]*FileCoverage)

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "mode:") {
			continue
		}

		// Parse: file:start,end statements count
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		// Extract file path
		colonIdx := strings.LastIndex(parts[0], ":")
		if colonIdx == -1 {
			continue
		}
		filePath := parts[0][:colonIdx]

		// Make path relative to repo
		if idx := strings.Index(filePath, c.ws.RepoPath); idx != -1 {
			filePath = filePath[idx+len(c.ws.RepoPath)+1:]
		}

		statements, _ := strconv.Atoi(parts[1])
		count, _ := strconv.Atoi(parts[2])

		// Get or create file stats
		fc, exists := fileStats[filePath]
		if !exists {
			fc = &FileCoverage{
				Path:           filePath,
				UncoveredLines: []int{},
			}
			fileStats[filePath] = fc
		}

		fc.TotalLines += statements
		if count > 0 {
			fc.CoveredLines += statements
		}
	}

	// Calculate percentages and build result
	var result []FileCoverage
	for _, fc := range fileStats {
		if fc.TotalLines > 0 {
			fc.CoveragePercent = float64(fc.CoveredLines) / float64(fc.TotalLines) * 100
		}
		result = append(result, *fc)
	}

	return result, nil
}

// collectPythonCoverage runs pytest with coverage
func (c *CoverageCollector) collectPythonCoverage(ctx context.Context) ([]FileCoverage, error) {
	coverFile := filepath.Join(c.ws.Path(), "artifacts", "coverage.json")

	// Run pytest with coverage
	cmd := exec.CommandContext(ctx, "python", "-m", "pytest",
		"--cov=.", "--cov-report=json:"+coverFile)
	cmd.Dir = c.ws.RepoPath

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Debug().Err(err).Str("stderr", stderr.String()).Msg("pytest completed with errors")
	}

	// Parse coverage JSON
	return c.parsePythonCoverage(coverFile)
}

// parsePythonCoverage parses coverage.py JSON output
func (c *CoverageCollector) parsePythonCoverage(coverFile string) ([]FileCoverage, error) {
	data, err := os.ReadFile(coverFile)
	if err != nil {
		return nil, err
	}

	var report struct {
		Files map[string]struct {
			Summary struct {
				CoveredLines   int     `json:"covered_lines"`
				NumStatements  int     `json:"num_statements"`
				PercentCovered float64 `json:"percent_covered"`
				MissingLines   []int   `json:"missing_lines"`
			} `json:"summary"`
		} `json:"files"`
	}

	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}

	var result []FileCoverage
	for path, file := range report.Files {
		// Make path relative
		relPath := path
		if strings.HasPrefix(path, c.ws.RepoPath) {
			relPath = path[len(c.ws.RepoPath)+1:]
		}

		result = append(result, FileCoverage{
			Path:            relPath,
			TotalLines:      file.Summary.NumStatements,
			CoveredLines:    file.Summary.CoveredLines,
			CoveragePercent: file.Summary.PercentCovered,
			UncoveredLines:  file.Summary.MissingLines,
		})
	}

	return result, nil
}

// collectJestCoverage runs jest with coverage
func (c *CoverageCollector) collectJestCoverage(ctx context.Context) ([]FileCoverage, error) {
	coverDir := filepath.Join(c.ws.Path(), "artifacts", "coverage")

	// Run jest with coverage
	cmd := exec.CommandContext(ctx, "npx", "jest",
		"--coverage", "--coverageDirectory="+coverDir, "--coverageReporters=json")
	cmd.Dir = c.ws.RepoPath

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Debug().Err(err).Str("stderr", stderr.String()).Msg("jest completed with errors")
	}

	// Parse coverage JSON
	coverFile := filepath.Join(coverDir, "coverage-final.json")
	return c.parseJestCoverage(coverFile)
}

// parseJestCoverage parses Jest/Istanbul coverage JSON
func (c *CoverageCollector) parseJestCoverage(coverFile string) ([]FileCoverage, error) {
	data, err := os.ReadFile(coverFile)
	if err != nil {
		return nil, err
	}

	// Istanbul format: map of file paths to coverage data
	var report map[string]struct {
		S map[string]int `json:"s"` // Statement counts
		F map[string]int `json:"f"` // Function counts
		B map[string][]int `json:"b"` // Branch counts
	}

	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}

	var result []FileCoverage
	for path, file := range report {
		// Make path relative
		relPath := path
		if strings.HasPrefix(path, c.ws.RepoPath) {
			relPath = path[len(c.ws.RepoPath)+1:]
		}

		totalStatements := len(file.S)
		coveredStatements := 0
		for _, count := range file.S {
			if count > 0 {
				coveredStatements++
			}
		}

		var coveragePercent float64
		if totalStatements > 0 {
			coveragePercent = float64(coveredStatements) / float64(totalStatements) * 100
		}

		result = append(result, FileCoverage{
			Path:            relPath,
			TotalLines:      totalStatements,
			CoveredLines:    coveredStatements,
			CoveragePercent: coveragePercent,
			UncoveredLines:  []int{}, // Would need line mapping from statementMap
		})
	}

	return result, nil
}

// CollectForFile collects coverage for a specific test file
func (c *CoverageCollector) CollectForFile(ctx context.Context, testFile string) (*FileCoverage, error) {
	ext := filepath.Ext(testFile)

	switch ext {
	case ".go":
		return c.collectGoFileCoverage(ctx, testFile)
	case ".py":
		return c.collectPythonFileCoverage(ctx, testFile)
	case ".js", ".ts":
		return c.collectJestFileCoverage(ctx, testFile)
	default:
		return nil, fmt.Errorf("unsupported test file type: %s", ext)
	}
}

// collectGoFileCoverage collects coverage for a single Go test file
func (c *CoverageCollector) collectGoFileCoverage(ctx context.Context, testFile string) (*FileCoverage, error) {
	dir := filepath.Dir(testFile)
	relDir, _ := filepath.Rel(c.ws.RepoPath, dir)
	pkg := "./" + relDir

	coverFile := filepath.Join(c.ws.Path(), "artifacts", "coverage_single.out")

	cmd := exec.CommandContext(ctx, "go", "test", "-coverprofile="+coverFile, pkg)
	cmd.Dir = c.ws.RepoPath

	if err := cmd.Run(); err != nil {
		log.Debug().Err(err).Msg("go test for single file completed with errors")
	}

	files, err := c.parseGoCoverage(coverFile)
	if err != nil {
		return nil, err
	}

	// Aggregate all files into one result
	result := &FileCoverage{
		Path:           testFile,
		UncoveredLines: []int{},
	}

	for _, f := range files {
		result.TotalLines += f.TotalLines
		result.CoveredLines += f.CoveredLines
	}

	if result.TotalLines > 0 {
		result.CoveragePercent = float64(result.CoveredLines) / float64(result.TotalLines) * 100
	}

	return result, nil
}

// collectPythonFileCoverage collects coverage for a single Python test file
func (c *CoverageCollector) collectPythonFileCoverage(ctx context.Context, testFile string) (*FileCoverage, error) {
	relPath, _ := filepath.Rel(c.ws.RepoPath, testFile)

	cmd := exec.CommandContext(ctx, "python", "-m", "pytest", "--cov=.", relPath)
	cmd.Dir = c.ws.RepoPath

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		log.Debug().Err(err).Msg("pytest for single file completed with errors")
	}

	// Parse coverage from stdout (simple format)
	return c.parseCoverageOutput(stdout.String(), testFile)
}

// collectJestFileCoverage collects coverage for a single Jest test file
func (c *CoverageCollector) collectJestFileCoverage(ctx context.Context, testFile string) (*FileCoverage, error) {
	relPath, _ := filepath.Rel(c.ws.RepoPath, testFile)

	cmd := exec.CommandContext(ctx, "npx", "jest", "--coverage", relPath)
	cmd.Dir = c.ws.RepoPath

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		log.Debug().Err(err).Msg("jest for single file completed with errors")
	}

	return c.parseCoverageOutput(stdout.String(), testFile)
}

// parseCoverageOutput parses coverage from text output (fallback)
func (c *CoverageCollector) parseCoverageOutput(output, testFile string) (*FileCoverage, error) {
	result := &FileCoverage{
		Path:           testFile,
		UncoveredLines: []int{},
	}

	// Look for common coverage patterns
	// Pattern: "Coverage: 85.5%" or "85.5% coverage"
	re := regexp.MustCompile(`(\d+\.?\d*)%`)
	matches := re.FindStringSubmatch(output)
	if len(matches) > 1 {
		result.CoveragePercent, _ = strconv.ParseFloat(matches[1], 64)
	}

	// Pattern: "Statements: 100/120"
	re = regexp.MustCompile(`(\d+)/(\d+)`)
	matches = re.FindStringSubmatch(output)
	if len(matches) > 2 {
		result.CoveredLines, _ = strconv.Atoi(matches[1])
		result.TotalLines, _ = strconv.Atoi(matches[2])
	}

	return result, nil
}

// CoverageSummaryLine returns a one-line summary of coverage
func CoverageSummaryLine(report *CoverageReport) string {
	return fmt.Sprintf("Coverage: %.1f%% (%d/%d lines)",
		report.Summary.CoveragePercent,
		report.Summary.CoveredLines,
		report.Summary.TotalLines)
}
