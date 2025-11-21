package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/QTest-hq/qtest/internal/codecov"
	"github.com/QTest-hq/qtest/internal/config"
	"github.com/QTest-hq/qtest/internal/llm"
	"github.com/QTest-hq/qtest/internal/workspace"
	"github.com/QTest-hq/qtest/pkg/model"
	"github.com/spf13/cobra"
)

func coverageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Collect and analyze code coverage",
		Long:  `Collect code coverage from test runs and identify gaps for test generation`,
	}

	cmd.AddCommand(coverageCollectCmd())
	cmd.AddCommand(coverageAnalyzeCmd())
	cmd.AddCommand(coverageGapsCmd())
	cmd.AddCommand(coverageGenerateCmd())
	cmd.AddCommand(coverageReportCmd())
	cmd.AddCommand(coverageCICmd())

	return cmd
}

func coverageCollectCmd() *cobra.Command {
	var (
		workDir    string
		language   string
		outputFile string
		jsonOut    bool
		htmlOut    string
	)

	cmd := &cobra.Command{
		Use:   "collect",
		Short: "Collect code coverage by running tests",
		Long: `Run tests with coverage instrumentation and display results.

Examples:
  qtest coverage collect                           # Collect coverage for current directory
  qtest coverage collect -d ./myproject            # Collect for specific directory
  qtest coverage collect -o coverage.json          # Save JSON report
  qtest coverage collect --json                    # Output as JSON to stdout
  qtest coverage collect --html ./reports          # Generate HTML report`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Auto-detect language if not specified
			if language == "" {
				language = detectProjectLanguage(workDir)
			}

			if !jsonOut {
				fmt.Printf("Collecting coverage for %s project...\n", language)
				fmt.Printf("Working directory: %s\n\n", workDir)
			}

			collector := codecov.NewCollector(workDir, language)

			report, err := collector.Collect(context.Background())
			if err != nil {
				return fmt.Errorf("failed to collect coverage: %w", err)
			}

			// JSON output mode
			if jsonOut {
				data, _ := json.MarshalIndent(report, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			// Display summary
			displayCoverageReport(report)

			// Save JSON report if output specified
			if outputFile != "" {
				if err := collector.SaveReport(report, outputFile); err != nil {
					return fmt.Errorf("failed to save report: %w", err)
				}
				fmt.Printf("\n📄 Report saved to: %s\n", outputFile)
			}

			// Generate HTML if requested
			if htmlOut != "" {
				htmlPath := filepath.Join(htmlOut, "coverage-report.html")
				if err := generateCoverageHTML(report, htmlPath); err != nil {
					return fmt.Errorf("failed to generate HTML report: %w", err)
				}
				fmt.Printf("📄 HTML report generated: %s\n", htmlPath)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&workDir, "dir", "d", ".", "Working directory")
	cmd.Flags().StringVarP(&language, "language", "l", "", "Language (auto-detected if not specified)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file for JSON coverage report")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON to stdout")
	cmd.Flags().StringVar(&htmlOut, "html", "", "Output directory for HTML report")

	return cmd
}

func coverageAnalyzeCmd() *cobra.Command {
	var (
		reportFile string
		modelFile  string
		target     float64
	)

	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze coverage report and identify gaps",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load coverage report
			report, err := codecov.LoadReport(reportFile)
			if err != nil {
				return fmt.Errorf("failed to load coverage report: %w", err)
			}

			// Load system model if provided
			var sysModel *model.SystemModel
			if modelFile != "" {
				data, err := os.ReadFile(modelFile)
				if err != nil {
					return fmt.Errorf("failed to read model: %w", err)
				}
				sysModel = &model.SystemModel{}
				if err := json.Unmarshal(data, sysModel); err != nil {
					return fmt.Errorf("failed to parse model: %w", err)
				}
			}

			// Analyze
			analyzer := codecov.NewAnalyzer(report, sysModel)
			result := analyzer.Analyze(target)

			// Display results
			fmt.Printf("📊 Coverage Analysis\n")
			fmt.Printf("====================\n\n")

			fmt.Printf("Current Coverage: %.1f%%\n", result.TotalCoverage)
			fmt.Printf("Target Coverage:  %.1f%%\n", result.TargetCoverage)

			if result.TotalCoverage >= result.TargetCoverage {
				fmt.Printf("✅ Coverage target met!\n\n")
			} else {
				gap := result.TargetCoverage - result.TotalCoverage
				fmt.Printf("❌ %.1f%% below target\n\n", gap)
			}

			fmt.Printf("Coverage Gaps: %d\n", len(result.Gaps))
			fmt.Printf("Critical Gaps: %d\n", result.CriticalGaps)
			fmt.Printf("Suggested Tests: %d\n", result.SuggestedTests)
			fmt.Printf("Estimated Effort: %s\n\n", result.EstimatedEffort)

			// Show top gaps
			fmt.Printf("Top Priority Gaps:\n")
			maxShow := 10
			if len(result.Gaps) < maxShow {
				maxShow = len(result.Gaps)
			}

			for i := 0; i < maxShow; i++ {
				gap := result.Gaps[i]
				priorityIcon := "⬜"
				switch gap.Priority {
				case "critical":
					priorityIcon = "🔴"
				case "high":
					priorityIcon = "🟠"
				case "medium":
					priorityIcon = "🟡"
				case "low":
					priorityIcon = "🟢"
				}

				name := gap.Name
				if name == "" {
					name = fmt.Sprintf("%s:%d-%d", filepath.Base(gap.File), gap.StartLine, gap.EndLine)
				}
				fmt.Printf("  %s [%s] %s - %s\n", priorityIcon, gap.Type, name, gap.Reason)
			}

			if len(result.Gaps) > maxShow {
				fmt.Printf("  ... and %d more\n", len(result.Gaps)-maxShow)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&reportFile, "report", "r", "codecov.json", "Coverage report file")
	cmd.Flags().StringVarP(&modelFile, "model", "m", "", "System model file (optional)")
	cmd.Flags().Float64VarP(&target, "target", "t", 80.0, "Target coverage percentage")

	return cmd
}

func coverageGenerateCmd() *cobra.Command {
	var (
		workDir        string
		targetCoverage float64
		maxIterations  int
		maxTests       int
		tier           string
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate tests to improve code coverage",
		Long:  `Run iterative test generation to reach target coverage`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// Validate directory
			if workDir == "." {
				var err error
				workDir, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to get working directory: %w", err)
				}
			}

			// Auto-detect language
			language := detectProjectLanguage(workDir)

			fmt.Printf("Coverage-Guided Test Generation\n")
			fmt.Printf("================================\n")
			fmt.Printf("Directory: %s\n", workDir)
			fmt.Printf("Language:  %s\n", language)
			fmt.Printf("Target:    %.1f%%\n", targetCoverage)
			fmt.Printf("Max Iter:  %d\n\n", maxIterations)

			// Load config
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create LLM router
			router, err := llm.NewRouter(cfg)
			if err != nil {
				return fmt.Errorf("failed to create LLM router: %w", err)
			}

			// Check LLM health
			if err := router.HealthCheck(); err != nil {
				return fmt.Errorf("LLM not available: %w\nMake sure Ollama is running: ollama serve", err)
			}

			// Parse tier
			tierNum := 2
			fmt.Sscanf(tier, "%d", &tierNum)
			llmTier := llm.Tier(tierNum)
			if llmTier < 1 || llmTier > 3 {
				llmTier = llm.Tier2
			}

			// Create workspace for coverage generation
			ws := &workspace.Workspace{
				Name:     filepath.Base(workDir),
				RepoPath: workDir,
				Language: language,
				State:    &workspace.WorkspaceState{},
			}

			// Create coverage runner
			runnerCfg := &workspace.CoverageRunConfig{
				Tier:           llmTier,
				TargetCoverage: targetCoverage,
				MaxIterations:  maxIterations,
				MaxTestsPerRun: maxTests,
				TestDir:        "tests",
				RunTests:       true,
				FocusCritical:  true,
			}

			runner := workspace.NewCoverageRunner(ws, router, runnerCfg)

			// Set up progress callback
			runner.OnProgress = func(phase string, current, total int, message string) {
				fmt.Printf("[%d/%d] %s\n", current, total, message)
			}

			runner.OnComplete = func(testFile string, testsCount int) {
				fmt.Printf("Generated %d tests: %s\n", testsCount, testFile)
			}

			runner.OnCoverage = func(before, after float64) {
				diff := after - before
				icon := "📈"
				if diff <= 0 {
					icon = "📉"
				}
				fmt.Printf("%s Coverage: %.1f%% -> %.1f%% (%+.1f%%)\n", icon, before, after, diff)
			}

			// Run coverage-guided generation
			if err := runner.Run(ctx); err != nil {
				return fmt.Errorf("coverage generation failed: %w", err)
			}

			// Show final report
			report := runner.GetCoverageReport()
			if report != nil {
				fmt.Printf("\n📊 Final Coverage: %.1f%%\n", report.Percentage)
				if report.Percentage >= targetCoverage {
					fmt.Printf("✅ Target coverage reached!\n")
				} else {
					fmt.Printf("⚠️ Target coverage not reached (%.1f%% remaining)\n", targetCoverage-report.Percentage)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&workDir, "dir", "d", ".", "Working directory")
	cmd.Flags().Float64VarP(&targetCoverage, "target", "t", 80.0, "Target coverage percentage")
	cmd.Flags().IntVarP(&maxIterations, "iterations", "i", 5, "Maximum iterations")
	cmd.Flags().IntVarP(&maxTests, "max", "m", 10, "Maximum tests per iteration")
	cmd.Flags().StringVar(&tier, "tier", "2", "LLM tier (1=fast, 2=balanced, 3=thorough)")

	return cmd
}

func coverageGapsCmd() *cobra.Command {
	var (
		reportFile string
		modelFile  string
		outputFile string
		format     string
	)

	cmd := &cobra.Command{
		Use:   "gaps",
		Short: "Generate test intents for coverage gaps",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load coverage report
			report, err := codecov.LoadReport(reportFile)
			if err != nil {
				return fmt.Errorf("failed to load coverage report: %w", err)
			}

			// Load system model
			var sysModel *model.SystemModel
			if modelFile != "" {
				data, err := os.ReadFile(modelFile)
				if err != nil {
					return fmt.Errorf("failed to read model: %w", err)
				}
				sysModel = &model.SystemModel{}
				if err := json.Unmarshal(data, sysModel); err != nil {
					return fmt.Errorf("failed to parse model: %w", err)
				}
			}

			// Analyze and generate intents
			analyzer := codecov.NewAnalyzer(report, sysModel)
			result := analyzer.Analyze(80.0)
			intents := analyzer.GenerateTestIntents(result.Gaps)

			fmt.Printf("Generated %d test intents for coverage gaps\n\n", len(intents))

			// Output
			if format == "json" || outputFile != "" {
				plan := &model.TestPlan{
					ModelID: "coverage-gaps",
					Intents: intents,
				}

				data, _ := json.MarshalIndent(plan, "", "  ")

				if outputFile != "" {
					if err := os.WriteFile(outputFile, data, 0644); err != nil {
						return fmt.Errorf("failed to write output: %w", err)
					}
					fmt.Printf("Test plan saved to: %s\n", outputFile)
				} else {
					fmt.Println(string(data))
				}
			} else {
				// Display as list
				for i, intent := range intents {
					fmt.Printf("%d. [%s] %s - %s\n", i+1, intent.Level, intent.TargetKind, intent.Reason)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&reportFile, "report", "r", "codecov.json", "Coverage report file")
	cmd.Flags().StringVarP(&modelFile, "model", "m", "", "System model file (optional)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file for test plan")
	cmd.Flags().StringVarP(&format, "format", "f", "text", "Output format (text, json)")

	return cmd
}

// detectProjectLanguage auto-detects project language
func detectProjectLanguage(dir string) string {
	// Check for language-specific files
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return "go"
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		return "javascript"
	}
	if _, err := os.Stat(filepath.Join(dir, "requirements.txt")); err == nil {
		return "python"
	}
	if _, err := os.Stat(filepath.Join(dir, "pyproject.toml")); err == nil {
		return "python"
	}
	if _, err := os.Stat(filepath.Join(dir, "setup.py")); err == nil {
		return "python"
	}

	// Default
	return "go"
}

func coverageReportCmd() *cobra.Command {
	var (
		reportFile string
		format     string
		outputDir  string
	)

	cmd := &cobra.Command{
		Use:   "report",
		Short: "View or export a coverage report",
		Long: `View an existing JSON coverage report or generate reports in different formats.

Examples:
  qtest coverage report -r coverage.json                  # View report as text
  qtest coverage report -r coverage.json --format json    # View as formatted JSON
  qtest coverage report -r coverage.json --format html -o ./reports  # Generate HTML`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate report file
			reportAbs, err := validateFilePath(reportFile)
			if err != nil {
				return fmt.Errorf("invalid report file: %w", err)
			}

			// Load report
			report, err := codecov.LoadReport(reportAbs)
			if err != nil {
				return fmt.Errorf("failed to load report: %w", err)
			}

			switch format {
			case "json":
				// Pretty print JSON
				data, _ := json.MarshalIndent(report, "", "  ")
				fmt.Println(string(data))

			case "html":
				// Generate HTML report
				if outputDir == "" {
					outputDir = "."
				}
				htmlPath := filepath.Join(outputDir, "coverage-report.html")
				if err := generateCoverageHTML(report, htmlPath); err != nil {
					return fmt.Errorf("failed to generate HTML report: %w", err)
				}
				fmt.Printf("📄 HTML report generated: %s\n", htmlPath)

			default:
				// Text format
				displayCoverageReport(report)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&reportFile, "report", "r", "coverage.json", "Coverage report file")
	cmd.Flags().StringVar(&format, "format", "text", "Output format: text, json, or html")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "", "Output directory for HTML reports")

	return cmd
}

func coverageCICmd() *cobra.Command {
	var (
		workDir   string
		language  string
		threshold float64
		quiet     bool
		jsonOut   bool
	)

	cmd := &cobra.Command{
		Use:   "ci",
		Short: "CI-friendly coverage check with threshold enforcement",
		Long: `Run coverage collection and check against a threshold.

Returns exit code 1 if coverage is below the threshold.
Useful for CI pipelines to enforce minimum coverage.

Examples:
  qtest coverage ci -t 80                    # Fail if coverage < 80%
  qtest coverage ci -t 70 --quiet            # Quiet mode for scripts
  qtest coverage ci -t 80 --json             # Output as JSON`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Auto-detect language if not specified
			if language == "" {
				language = detectProjectLanguage(workDir)
			}

			if !quiet {
				fmt.Printf("Running coverage check (threshold: %.1f%%)...\n\n", threshold)
			}

			collector := codecov.NewCollector(workDir, language)

			report, err := collector.Collect(context.Background())
			if err != nil {
				return fmt.Errorf("failed to collect coverage: %w", err)
			}

			// Determine pass/fail
			passed := report.Percentage >= threshold

			if jsonOut {
				// JSON output for CI parsing
				result := map[string]interface{}{
					"coverage":   report.Percentage,
					"threshold":  threshold,
					"passed":     passed,
					"total":      report.TotalLines,
					"covered":    report.CoveredLines,
					"files":      len(report.Files),
					"language":   language,
					"timestamp":  report.Timestamp,
				}
				data, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(data))
			} else if !quiet {
				// Human-readable output
				displayCoverageReport(report)
				fmt.Println()

				if passed {
					fmt.Printf("✅ Coverage %.1f%% meets threshold %.1f%%\n", report.Percentage, threshold)
				} else {
					fmt.Printf("❌ Coverage %.1f%% is below threshold %.1f%%\n", report.Percentage, threshold)
				}
			}

			// Exit with error if below threshold
			if !passed {
				return fmt.Errorf("coverage %.1f%% is below threshold %.1f%%", report.Percentage, threshold)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&workDir, "dir", "d", ".", "Working directory")
	cmd.Flags().StringVarP(&language, "language", "l", "", "Language (auto-detected if not specified)")
	cmd.Flags().Float64VarP(&threshold, "threshold", "t", 80.0, "Minimum coverage threshold")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Quiet mode (only exit code)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")

	return cmd
}

// displayCoverageReport displays a coverage report in text format
func displayCoverageReport(report *codecov.CoverageReport) {
	fmt.Printf("📊 Coverage Report\n")
	fmt.Printf("==================\n")
	fmt.Printf("Total Lines:   %d\n", report.TotalLines)
	fmt.Printf("Covered Lines: %d\n", report.CoveredLines)
	fmt.Printf("Coverage:      %.1f%%\n\n", report.Percentage)

	if len(report.Files) > 0 {
		fmt.Printf("Files (%d):\n", len(report.Files))
		for _, f := range report.Files {
			status := "✅"
			if f.Percentage < 50 {
				status = "❌"
			} else if f.Percentage < 80 {
				status = "⚠️"
			}
			fmt.Printf("  %s %s: %.1f%% (%d/%d lines)\n",
				status, filepath.Base(f.Path), f.Percentage, f.CoveredLines, f.TotalLines)
		}
	}
}

// generateCoverageHTML generates an HTML coverage report
func generateCoverageHTML(report *codecov.CoverageReport, outputPath string) error {
	// Ensure output directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Determine quality class
	qualityClass := "poor"
	qualityText := "Poor"
	if report.Percentage >= 80 {
		qualityClass = "good"
		qualityText = "Good"
	} else if report.Percentage >= 50 {
		qualityClass = "acceptable"
		qualityText = "Acceptable"
	}

	// Build files table
	var filesHTML string
	for _, f := range report.Files {
		statusClass := "status-poor"
		statusIcon := "✗"
		if f.Percentage >= 80 {
			statusClass = "status-good"
			statusIcon = "✓"
		} else if f.Percentage >= 50 {
			statusClass = "status-acceptable"
			statusIcon = "~"
		}
		filesHTML += fmt.Sprintf(`
        <tr class="%s">
            <td>%s</td>
            <td>%s</td>
            <td>%.1f%%</td>
            <td>%d</td>
            <td>%d</td>
        </tr>`, statusClass, statusIcon, f.Path, f.Percentage, f.CoveredLines, f.TotalLines)
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Coverage Report</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 40px; background: #f5f5f5; }
        .container { max-width: 1000px; margin: 0 auto; background: white; padding: 30px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        h1 { color: #333; border-bottom: 2px solid #eee; padding-bottom: 10px; }
        .summary { display: flex; gap: 20px; margin: 20px 0; }
        .stat { background: #f8f9fa; padding: 20px; border-radius: 8px; text-align: center; flex: 1; }
        .stat-value { font-size: 2em; font-weight: bold; color: #333; }
        .stat-label { color: #666; margin-top: 5px; }
        .quality-good { color: #28a745; }
        .quality-acceptable { color: #ffc107; }
        .quality-poor { color: #dc3545; }
        table { width: 100%%; border-collapse: collapse; margin-top: 20px; }
        th { background: #f8f9fa; padding: 12px; text-align: left; border-bottom: 2px solid #dee2e6; }
        td { padding: 10px 12px; border-bottom: 1px solid #eee; }
        .status-good td:first-child { color: #28a745; }
        .status-acceptable td:first-child { color: #ffc107; }
        .status-poor td:first-child { color: #dc3545; }
        .timestamp { color: #999; font-size: 0.9em; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>📊 Coverage Report</h1>

        <div class="summary">
            <div class="stat">
                <div class="stat-value quality-%s">%.1f%%</div>
                <div class="stat-label">Coverage (%s)</div>
            </div>
            <div class="stat">
                <div class="stat-value">%d</div>
                <div class="stat-label">Total Lines</div>
            </div>
            <div class="stat">
                <div class="stat-value">%d</div>
                <div class="stat-label">Covered Lines</div>
            </div>
            <div class="stat">
                <div class="stat-value">%d</div>
                <div class="stat-label">Files</div>
            </div>
        </div>

        <h2>Files</h2>
        <table>
            <thead>
                <tr>
                    <th>Status</th>
                    <th>File</th>
                    <th>Coverage</th>
                    <th>Covered</th>
                    <th>Total</th>
                </tr>
            </thead>
            <tbody>
                %s
            </tbody>
        </table>

        <p class="timestamp">Generated: %s</p>
    </div>
</body>
</html>`,
		qualityClass, report.Percentage, qualityText,
		report.TotalLines, report.CoveredLines, len(report.Files),
		filesHTML, report.Timestamp.Format("2006-01-02 15:04:05"))

	return os.WriteFile(outputPath, []byte(html), 0644)
}
