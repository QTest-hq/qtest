package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/QTest-hq/qtest/internal/mutation"
	"github.com/spf13/cobra"
)

func mutationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mutation",
		Short: "Run mutation testing to evaluate test quality",
		Long: `Mutation testing evaluates test suite quality by introducing small changes (mutations)
to the source code and checking if tests detect them.

A high mutation score indicates tests are effective at catching bugs.

Quality thresholds:
  - Good:       >= 70% mutations killed
  - Acceptable: 50-70% mutations killed
  - Poor:       < 50% mutations killed`,
	}

	cmd.AddCommand(mutationRunCmd())
	cmd.AddCommand(mutationReportCmd())

	return cmd
}

func mutationRunCmd() *cobra.Command {
	var (
		sourceFile string
		testFile   string
		mode       string
		timeout    int
		maxMutants int
		outputFile string
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run mutation testing on a source file",
		Long: `Run mutation testing on a source file using its test file.

Examples:
  qtest mutation run -s calculator.go -t calculator_test.go
  qtest mutation run -s ./pkg/math/math.go -t ./pkg/math/math_test.go --mode thorough
  qtest mutation run -s main.go -t main_test.go -o report.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// Validate source file
			sourceAbs, err := validateFilePath(sourceFile)
			if err != nil {
				return fmt.Errorf("invalid source file: %w", err)
			}

			// Auto-detect test file if not provided
			if testFile == "" {
				testFile = deriveTestPath(sourceAbs)
				if testFile == "" {
					return fmt.Errorf("could not auto-detect test file, please specify with -t")
				}
			}

			// Validate test file
			testAbs, err := validateFilePath(testFile)
			if err != nil {
				return fmt.Errorf("invalid test file: %w", err)
			}

			fmt.Printf("🧬 Mutation Testing\n")
			fmt.Printf("==================\n")
			fmt.Printf("Source: %s\n", sourceAbs)
			fmt.Printf("Test:   %s\n", testAbs)
			fmt.Printf("Mode:   %s\n\n", mode)

			// Configure mutation testing
			var cfg mutation.MutationConfig
			switch mode {
			case "thorough":
				cfg = mutation.ThoroughConfig()
			default:
				cfg = mutation.DefaultConfig()
			}

			if maxMutants > 0 {
				cfg.MaxMutantsPerFunction = maxMutants
			}

			// Create runner with available tools
			runner := mutation.NewRunner(
				mutation.NewGoMutestingTool(),
				mutation.NewSimpleMutationTool(),
			)

			// Check available tools
			tools := runner.GetAvailableTools(ctx)
			if len(tools) == 0 {
				return fmt.Errorf("no mutation testing tools available")
			}
			fmt.Printf("Using: %s\n\n", tools[0].Name())

			// Run mutation testing
			fmt.Println("Running mutation testing...")
			result, err := runner.Run(ctx, sourceAbs, testAbs, cfg)
			if err != nil {
				return fmt.Errorf("mutation testing failed: %w", err)
			}

			// Display results
			displayMutationResult(result)

			// Save report if requested
			if outputFile != "" {
				if err := saveMutationReport(result, outputFile); err != nil {
					return fmt.Errorf("failed to save report: %w", err)
				}
				fmt.Printf("\n📄 Report saved: %s\n", outputFile)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&sourceFile, "source", "s", "", "Source file to mutate")
	cmd.Flags().StringVarP(&testFile, "test", "t", "", "Test file (auto-detected if not specified)")
	cmd.Flags().StringVarP(&mode, "mode", "m", "fast", "Mode: fast or thorough")
	cmd.Flags().IntVar(&timeout, "timeout", 120, "Timeout in seconds")
	cmd.Flags().IntVar(&maxMutants, "max", 0, "Maximum mutants per function (0=use default)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Save report to file")
	cmd.MarkFlagRequired("source")

	return cmd
}

func mutationReportCmd() *cobra.Command {
	var (
		reportFile string
		format     string
	)

	cmd := &cobra.Command{
		Use:   "report",
		Short: "View a mutation testing report",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate report file
			reportAbs, err := validateFilePath(reportFile)
			if err != nil {
				return fmt.Errorf("invalid report file: %w", err)
			}

			// Load report
			data, err := os.ReadFile(reportAbs)
			if err != nil {
				return fmt.Errorf("failed to read report: %w", err)
			}

			var result mutation.Result
			if err := json.Unmarshal(data, &result); err != nil {
				return fmt.Errorf("failed to parse report: %w", err)
			}

			if format == "json" {
				// Pretty print JSON
				pretty, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(pretty))
			} else {
				displayMutationResult(&result)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&reportFile, "file", "f", "", "Report file to view")
	cmd.Flags().StringVar(&format, "format", "text", "Output format: text or json")
	cmd.MarkFlagRequired("file")

	return cmd
}

// displayMutationResult displays mutation testing results
func displayMutationResult(result *mutation.Result) {
	fmt.Printf("\n📊 Results\n")
	fmt.Printf("==========\n")
	fmt.Printf("Total Mutants:    %d\n", result.Total)
	fmt.Printf("Killed:           %d\n", result.Killed)
	fmt.Printf("Survived:         %d\n", result.Survived)
	fmt.Printf("Timeout:          %d\n", result.Timeout)
	fmt.Printf("Duration:         %s\n", result.Duration)
	fmt.Printf("\n")

	// Score with color indicator
	scoreIcon := "🔴"
	quality := result.Quality()
	switch quality {
	case "good":
		scoreIcon = "🟢"
	case "acceptable":
		scoreIcon = "🟡"
	}
	fmt.Printf("Mutation Score:   %.1f%% %s (%s)\n", result.Score*100, scoreIcon, quality)

	// Show surviving mutants (potential test improvements)
	if result.Survived > 0 && len(result.Mutants) > 0 {
		fmt.Printf("\n⚠️  Surviving Mutants (tests did not catch):\n")
		shown := 0
		for _, m := range result.Mutants {
			if m.Status == mutation.StatusSurvived {
				fmt.Printf("   Line %d: %s\n", m.Line, m.Description)
				shown++
				if shown >= 5 {
					remaining := result.Survived - shown
					if remaining > 0 {
						fmt.Printf("   ... and %d more\n", remaining)
					}
					break
				}
			}
		}
	}

	// Recommendations
	fmt.Printf("\n💡 Recommendations:\n")
	if quality == "poor" {
		fmt.Println("   - Add more assertions to test edge cases")
		fmt.Println("   - Test boundary conditions (0, 1, -1, max values)")
		fmt.Println("   - Add tests for error handling paths")
	} else if quality == "acceptable" {
		fmt.Println("   - Consider adding tests for surviving mutants")
		fmt.Println("   - Review conditional logic test coverage")
	} else {
		fmt.Println("   - Test suite has good mutation coverage!")
		fmt.Println("   - Consider maintaining this quality as code evolves")
	}
}

// saveMutationReport saves mutation result to a file
func saveMutationReport(result *mutation.Result, path string) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// deriveTestPath attempts to find the test file for a source file
func deriveTestPath(sourcePath string) string {
	dir := filepath.Dir(sourcePath)
	base := filepath.Base(sourcePath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	var testName string
	switch ext {
	case ".go":
		testName = name + "_test.go"
	case ".py":
		testName = "test_" + name + ".py"
	case ".ts":
		testName = name + ".test.ts"
	case ".js":
		testName = name + ".test.js"
	default:
		return ""
	}

	testPath := filepath.Join(dir, testName)
	if _, err := os.Stat(testPath); err == nil {
		return testPath
	}

	return ""
}
