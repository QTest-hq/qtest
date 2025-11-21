package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/QTest-hq/qtest/internal/codecov"
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

	return cmd
}

func coverageCollectCmd() *cobra.Command {
	var (
		workDir    string
		language   string
		outputFile string
	)

	cmd := &cobra.Command{
		Use:   "collect",
		Short: "Collect code coverage by running tests",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Auto-detect language if not specified
			if language == "" {
				language = detectProjectLanguage(workDir)
			}

			fmt.Printf("Collecting coverage for %s project...\n", language)
			fmt.Printf("Working directory: %s\n\n", workDir)

			collector := codecov.NewCollector(workDir, language)

			report, err := collector.Collect(context.Background())
			if err != nil {
				return fmt.Errorf("failed to collect coverage: %w", err)
			}

			// Display summary
			fmt.Printf("📊 Coverage Report\n")
			fmt.Printf("==================\n")
			fmt.Printf("Total Lines:   %d\n", report.TotalLines)
			fmt.Printf("Covered Lines: %d\n", report.CoveredLines)
			fmt.Printf("Coverage:      %.1f%%\n\n", report.Percentage)

			fmt.Printf("Files (%d):\n", len(report.Files))
			for _, f := range report.Files {
				status := "✅"
				if f.Percentage < 50 {
					status = "❌"
				} else if f.Percentage < 80 {
					status = "⚠️"
				}
				fmt.Printf("  %s %s: %.1f%%\n", status, filepath.Base(f.Path), f.Percentage)
			}

			// Save report if output specified
			if outputFile != "" {
				if err := collector.SaveReport(report, outputFile); err != nil {
					return fmt.Errorf("failed to save report: %w", err)
				}
				fmt.Printf("\n📄 Report saved to: %s\n", outputFile)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&workDir, "dir", "d", ".", "Working directory")
	cmd.Flags().StringVarP(&language, "language", "l", "", "Language (auto-detected if not specified)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file for coverage report")

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
