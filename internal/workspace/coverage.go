package workspace

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/QTest-hq/qtest/internal/codecov"
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
// Delegates to codecov.Collector for actual collection
func (c *CoverageCollector) CollectAll(ctx context.Context) (*CoverageReport, error) {
	// Use codecov.Collector for actual coverage collection
	collector := codecov.NewCollector(c.ws.RepoPath, c.ws.Language)
	codecovReport, err := collector.Collect(ctx)
	if err != nil {
		return nil, fmt.Errorf("coverage collection failed: %w", err)
	}

	// Convert codecov report to workspace format
	files := convertCodecovFiles(codecovReport.Files, c.ws.RepoPath)

	// Generate coverage report artifact
	report, err := c.artifacts.GenerateCoverageReport(files)
	if err != nil {
		return nil, fmt.Errorf("failed to generate coverage report: %w", err)
	}

	return report, nil
}

// convertCodecovFiles converts codecov.FileCoverage to workspace.FileCoverage
func convertCodecovFiles(codecovFiles []codecov.FileCoverage, repoPath string) []FileCoverage {
	files := make([]FileCoverage, 0, len(codecovFiles))
	for _, cf := range codecovFiles {
		// Make path relative to repo if needed
		relPath := cf.Path
		if strings.HasPrefix(cf.Path, repoPath) {
			relPath = cf.Path[len(repoPath)+1:]
		}

		files = append(files, FileCoverage{
			Path:            relPath,
			TotalLines:      cf.TotalLines,
			CoveredLines:    cf.CoveredLines,
			CoveragePercent: cf.Percentage,
			UncoveredLines:  cf.UncoveredLines,
		})
	}
	return files
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

	// Use codecov package to parse coverage file
	codecovFiles, err := codecov.ParseGoCoverageFile(coverFile)
	if err != nil {
		return nil, err
	}

	// Aggregate all files into one result
	result := &FileCoverage{
		Path:           testFile,
		UncoveredLines: []int{},
	}

	for _, f := range codecovFiles {
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
