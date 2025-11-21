package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/QTest-hq/qtest/internal/config"
	"github.com/QTest-hq/qtest/internal/llm"
	"github.com/QTest-hq/qtest/internal/validator"
	"github.com/spf13/cobra"
)

func validateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate and fix generated tests",
		Long:  `Run generated tests and optionally auto-fix failures using LLM`,
	}

	cmd.AddCommand(validateRunCmd())
	cmd.AddCommand(validateFixCmd())

	return cmd
}

func validateRunCmd() *cobra.Command {
	var (
		language string
	)

	cmd := &cobra.Command{
		Use:   "run <test-file>",
		Short: "Run tests and show results",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			testFile := args[0]

			// Auto-detect language
			if language == "" {
				language = detectLanguage(testFile)
			}

			workDir := filepath.Dir(testFile)
			v := validator.NewValidator(workDir, language)

			fmt.Printf("Running tests: %s\n", testFile)
			fmt.Printf("Language: %s\n\n", language)

			result, err := v.RunTests(context.Background(), testFile)
			if err != nil {
				return fmt.Errorf("failed to run tests: %w", err)
			}

			// Display results
			if result.Passed {
				fmt.Println("✅ All tests passed!")
			} else {
				fmt.Printf("❌ Tests failed (%d errors)\n\n", len(result.Errors))
				for i, e := range result.Errors {
					fmt.Printf("Error %d: %s\n", i+1, e.TestName)
					if e.Message != "" {
						fmt.Printf("  Message: %s\n", e.Message)
					}
					if e.Expected != "" {
						fmt.Printf("  Expected: %s\n", e.Expected)
					}
					if e.Actual != "" {
						fmt.Printf("  Actual: %s\n", e.Actual)
					}
					fmt.Println()
				}
			}

			fmt.Printf("Duration: %s\n", result.Duration)

			if !result.Passed {
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&language, "language", "l", "", "Language (auto-detected if not specified)")

	return cmd
}

func validateFixCmd() *cobra.Command {
	var (
		language   string
		tier       int
		maxRetries int
	)

	cmd := &cobra.Command{
		Use:   "fix <test-file>",
		Short: "Run tests and auto-fix failures using LLM",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			testFile := args[0]

			// Auto-detect language
			if language == "" {
				language = detectLanguage(testFile)
			}

			workDir := filepath.Dir(testFile)

			// Load config and create LLM router
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			router, err := llm.NewRouter(cfg)
			if err != nil {
				return fmt.Errorf("failed to create LLM router: %w", err)
			}

			// Create validator
			v := validator.NewValidator(workDir, language)

			fmt.Printf("Running tests: %s\n", testFile)
			fmt.Printf("Language: %s\n\n", language)

			// First run
			result, err := v.RunTests(context.Background(), testFile)
			if err != nil {
				return fmt.Errorf("failed to run tests: %w", err)
			}

			if result.Passed {
				fmt.Println("✅ All tests passed! No fixes needed.")
				return nil
			}

			fmt.Printf("❌ Found %d failing tests\n", len(result.Errors))
			fmt.Printf("🔧 Attempting to fix with LLM (tier %d, max %d retries)...\n\n", tier, maxRetries)

			// Create fixer and attempt fix
			fixer := validator.NewFixer(router, llm.Tier(tier))

			fixResult, err := fixer.FixTest(context.Background(), testFile, result, v)
			if err != nil {
				return fmt.Errorf("fix failed: %w", err)
			}

			if fixResult.Fixed {
				fmt.Printf("✅ Tests fixed after %d attempt(s)!\n", fixResult.Attempts)
				fmt.Printf("Explanation: %s\n", fixResult.Explanation)
			} else {
				fmt.Printf("❌ Could not fix tests after %d attempts\n", fixResult.Attempts)
				fmt.Println("Manual intervention required.")
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&language, "language", "l", "", "Language (auto-detected if not specified)")
	cmd.Flags().IntVarP(&tier, "tier", "t", 2, "LLM tier (1=fast, 2=balanced, 3=thorough)")
	cmd.Flags().IntVar(&maxRetries, "retries", 3, "Maximum fix attempts")

	return cmd
}

func detectLanguage(filename string) string {
	ext := filepath.Ext(filename)
	switch ext {
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".py":
		return "python"
	case ".go":
		return "go"
	default:
		return "javascript"
	}
}
