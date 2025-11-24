package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/QTest-hq/qtest/internal/config"
	"github.com/QTest-hq/qtest/internal/e2e"
	"github.com/QTest-hq/qtest/internal/flow"
	"github.com/QTest-hq/qtest/internal/llm"
	"github.com/QTest-hq/qtest/internal/sidecar/playwright"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func e2eCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "e2e",
		Short: "E2E test generation and execution",
		Long:  `Generate, discover, and run end-to-end tests using Playwright`,
	}

	cmd.AddCommand(e2eDiscoverCmd())
	cmd.AddCommand(e2eGenerateCmd())
	cmd.AddCommand(e2eRunCmd())
	cmd.AddCommand(e2eListCmd())

	return cmd
}

func e2eDiscoverCmd() *cobra.Command {
	var (
		url           string
		outputFile    string
		maxPages      int
		playwrightURL string
	)

	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Auto-discover user flows using LLM",
		Long: `Analyze a website and discover user flows (login, registration, checkout, etc.) using LLM.

Requires a Playwright sidecar service running. Start it with:
  docker run -p 3000:3000 qtest/playwright-sidecar`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if url == "" {
				return fmt.Errorf("--url is required")
			}

			fmt.Printf("Discovering flows for: %s\n", url)
			fmt.Printf("Max pages to explore: %d\n\n", maxPages)

			// Load config and create LLM router
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			router, err := llm.NewRouter(cfg)
			if err != nil {
				return fmt.Errorf("failed to create LLM router: %w", err)
			}

			// Create playwright client
			pwClient, err := playwright.NewClient(playwrightURL)
			if err != nil {
				return fmt.Errorf("failed to connect to playwright sidecar at %s: %w\nMake sure the playwright sidecar is running", playwrightURL, err)
			}

			// Create flow discovery
			discovery := flow.NewDiscovery(pwClient, router, nil)

			// Discover flows by exploring the site
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			flows, err := discovery.ExploreAndDiscover(ctx, url, maxPages)
			if err != nil {
				return fmt.Errorf("flow discovery failed: %w", err)
			}

			fmt.Printf("Discovered %d flows:\n\n", len(flows))
			for i, f := range flows {
				fmt.Printf("%d. %s (%s)\n", i+1, f.Name, f.Type)
				fmt.Printf("   Description: %s\n", f.Description)
				fmt.Printf("   Steps: %d\n\n", len(f.Steps))
			}

			// Save to file if specified
			if outputFile != "" {
				data, err := yaml.Marshal(flows)
				if err != nil {
					return fmt.Errorf("failed to marshal flows: %w", err)
				}
				if err := os.WriteFile(outputFile, data, 0644); err != nil {
					return fmt.Errorf("failed to write output: %w", err)
				}
				fmt.Printf("Flows saved to: %s\n", outputFile)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&url, "url", "u", "", "Website URL to analyze (required)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file for discovered flows (YAML)")
	cmd.Flags().IntVar(&maxPages, "max-pages", 5, "Maximum number of pages to explore")
	cmd.Flags().StringVar(&playwrightURL, "playwright-url", "http://localhost:3000", "Playwright sidecar URL")

	return cmd
}

func e2eGenerateCmd() *cobra.Command {
	var (
		flowFile   string
		outputDir  string
		framework  string
		language   string
		baseURL    string
		enhance    bool
		tier       int
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate E2E tests from flow specification",
		Long:  `Generate Playwright/Cypress tests from a flow YAML specification`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flowFile == "" {
				return fmt.Errorf("--flow is required")
			}

			fmt.Printf("Generating E2E tests from: %s\n", flowFile)
			fmt.Printf("Framework: %s\n", framework)
			fmt.Printf("Language: %s\n\n", language)

			// Read flow file
			data, err := os.ReadFile(flowFile)
			if err != nil {
				return fmt.Errorf("failed to read flow file: %w", err)
			}

			// Parse flows
			var flows []flow.Flow
			if err := yaml.Unmarshal(data, &flows); err != nil {
				// Try single flow
				var singleFlow flow.Flow
				if err := yaml.Unmarshal(data, &singleFlow); err != nil {
					return fmt.Errorf("failed to parse flow file: %w", err)
				}
				flows = append(flows, singleFlow)
			}

			// Create generation config
			genConfig := &e2e.GenerationConfig{
				Framework:       e2e.TestFramework(framework),
				Language:        e2e.TestLanguage(language),
				OutputDir:       outputDir,
				IncludeComments: true,
				GenerateHelpers: true,
			}

			generator := e2e.NewPlaywrightGenerator(genConfig)

			// Generate tests for each flow
			var totalTests int
			for _, f := range flows {
				// Set base URL if provided
				if baseURL != "" {
					f.StartURL = baseURL
				}

				// Convert flow to E2E spec
				spec := e2e.FlowToSpec(&f, nil)

				// Enhance with LLM if requested
				if enhance {
					cfg, err := config.Load()
					if err == nil {
						router, err := llm.NewRouter(cfg)
						if err == nil {
							enhancer := e2e.NewLLMEnhancer(router, nil)
							enhanced, err := enhancer.Enhance(context.Background(), spec)
							if err == nil {
								spec = enhanced.EnhancedSpec
								// Add suggested tests
								spec.TestCases = append(spec.TestCases, enhanced.SuggestedTests...)
							}
						}
					}
				}

				// Generate tests
				result, err := generator.Generate(spec)
				if err != nil {
					fmt.Printf("Warning: Failed to generate tests for %s: %v\n", f.Name, err)
					continue
				}

				// Write generated files
				for _, file := range result.Files {
					filePath := file.Path
					if !filepath.IsAbs(filePath) {
						filePath = filepath.Join(outputDir, filepath.Base(file.Path))
					}

					dir := filepath.Dir(filePath)
					if err := os.MkdirAll(dir, 0755); err != nil {
						return fmt.Errorf("failed to create directory: %w", err)
					}

					if err := os.WriteFile(filePath, []byte(file.Content), 0644); err != nil {
						return fmt.Errorf("failed to write file: %w", err)
					}

					fmt.Printf("Generated: %s (%d tests)\n", filePath, file.TestCount)
				}

				totalTests += result.TestCount
			}

			fmt.Printf("\nTotal: %d tests generated from %d flows\n", totalTests, len(flows))

			return nil
		},
	}

	cmd.Flags().StringVarP(&flowFile, "flow", "f", "", "Flow specification file (YAML, required)")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "tests", "Output directory for generated tests")
	cmd.Flags().StringVar(&framework, "framework", "playwright", "Test framework (playwright, cypress)")
	cmd.Flags().StringVarP(&language, "language", "l", "typescript", "Output language (typescript, javascript)")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "Override base URL for tests")
	cmd.Flags().BoolVar(&enhance, "enhance", false, "Enhance tests with LLM suggestions")
	cmd.Flags().IntVarP(&tier, "tier", "t", 2, "LLM tier for enhancement")

	return cmd
}

func e2eRunCmd() *cobra.Command {
	var (
		testDir    string
		pattern    string
		browser    string
		headless   bool
		workers    int
		retries    int
		baseURL    string
		outputDir  string
		format     string
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run E2E tests using Playwright",
		Long:  `Execute E2E tests and display results`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Running E2E tests\n")
			fmt.Printf("Directory: %s\n", testDir)
			fmt.Printf("Browser: %s\n", browser)
			fmt.Printf("Headless: %v\n\n", headless)

			// Create runner config
			runnerConfig := &e2e.RunnerConfig{
				WorkDir:   testDir,
				TestDir:   testDir,
				OutputDir: outputDir,
				Headless:  headless,
				Timeout:   10 * time.Minute,
				Retries:   retries,
				Workers:   workers,
				Reporter:  "json",
				Browser:   browser,
				BaseURL:   baseURL,
			}

			runner := e2e.NewTestRunner(runnerConfig)

			// Run tests
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cancel()

			result, err := runner.RunTests(ctx, pattern)
			if err != nil {
				return fmt.Errorf("test execution failed: %w", err)
			}

			// Display results
			fmt.Printf("\n=== Test Results ===\n\n")
			fmt.Printf("Total:   %d\n", result.TotalTests)
			fmt.Printf("Passed:  %d\n", result.Passed)
			fmt.Printf("Failed:  %d\n", result.Failed)
			fmt.Printf("Skipped: %d\n", result.Skipped)
			fmt.Printf("Duration: %s\n\n", result.Duration)

			// Show failures
			if result.Failed > 0 {
				fmt.Println("Failed tests:")
				for _, test := range result.Tests {
					if test.Status == "failed" {
						fmt.Printf("  - %s\n", test.Name)
						if test.Error != "" {
							fmt.Printf("    Error: %s\n", test.Error)
						}
					}
				}
				fmt.Println()
			}

			// Generate report if requested
			if format != "" {
				report, err := runner.GenerateReport(result, format)
				if err != nil {
					fmt.Printf("Warning: Failed to generate report: %v\n", err)
				} else {
					reportFile := filepath.Join(outputDir, fmt.Sprintf("report.%s", format))
					if format == "html" {
						reportFile = filepath.Join(outputDir, "report.html")
					} else if format == "markdown" {
						reportFile = filepath.Join(outputDir, "report.md")
					}

					if err := os.MkdirAll(outputDir, 0755); err == nil {
						if err := os.WriteFile(reportFile, []byte(report), 0644); err == nil {
							fmt.Printf("Report saved to: %s\n", reportFile)
						}
					}
				}
			}

			// Exit with error if tests failed
			if !result.Success {
				os.Exit(1)
			}

			fmt.Println("All tests passed!")
			return nil
		},
	}

	cmd.Flags().StringVarP(&testDir, "dir", "d", "tests", "Test directory")
	cmd.Flags().StringVarP(&pattern, "pattern", "p", "", "Test pattern (file or grep pattern)")
	cmd.Flags().StringVar(&browser, "browser", "chromium", "Browser (chromium, firefox, webkit)")
	cmd.Flags().BoolVar(&headless, "headless", true, "Run in headless mode")
	cmd.Flags().IntVar(&workers, "workers", 4, "Number of parallel workers")
	cmd.Flags().IntVar(&retries, "retries", 2, "Number of retries for flaky tests")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "Base URL for tests")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "test-results", "Output directory for results")
	cmd.Flags().StringVar(&format, "format", "markdown", "Report format (json, html, markdown)")

	return cmd
}

func e2eListCmd() *cobra.Command {
	var (
		flowDir string
		format  string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available flow specifications",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Find flow files
			pattern := filepath.Join(flowDir, "*.yaml")
			files, err := filepath.Glob(pattern)
			if err != nil {
				return fmt.Errorf("failed to list flows: %w", err)
			}

			// Also check for .yml files
			ymlFiles, _ := filepath.Glob(filepath.Join(flowDir, "*.yml"))
			files = append(files, ymlFiles...)

			if len(files) == 0 {
				fmt.Println("No flow files found in", flowDir)
				return nil
			}

			fmt.Printf("Found %d flow file(s):\n\n", len(files))

			for _, file := range files {
				data, err := os.ReadFile(file)
				if err != nil {
					continue
				}

				var flows []flow.Flow
				if err := yaml.Unmarshal(data, &flows); err != nil {
					// Try single flow
					var singleFlow flow.Flow
					if err := yaml.Unmarshal(data, &singleFlow); err != nil {
						continue
					}
					flows = append(flows, singleFlow)
				}

				if format == "json" {
					jsonData, _ := json.MarshalIndent(map[string]interface{}{
						"file":  file,
						"flows": len(flows),
					}, "", "  ")
					fmt.Println(string(jsonData))
				} else {
					fmt.Printf("File: %s\n", file)
					for _, f := range flows {
						fmt.Printf("  - %s (%s): %d steps\n", f.Name, f.Type, len(f.Steps))
					}
					fmt.Println()
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&flowDir, "dir", "d", "flows", "Directory containing flow files")
	cmd.Flags().StringVar(&format, "format", "text", "Output format (text, json)")

	return cmd
}
