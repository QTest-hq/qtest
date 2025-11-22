package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/QTest-hq/qtest/internal/adapters"
	"github.com/QTest-hq/qtest/internal/codecov"
	"github.com/QTest-hq/qtest/internal/config"
	"github.com/QTest-hq/qtest/internal/generator"
	"github.com/QTest-hq/qtest/internal/llm"
	"github.com/QTest-hq/qtest/internal/mutation"
	"github.com/QTest-hq/qtest/internal/parser"
	"github.com/QTest-hq/qtest/internal/supplements"
	"github.com/QTest-hq/qtest/internal/workspace"
	"github.com/QTest-hq/qtest/pkg/dsl"
	"github.com/QTest-hq/qtest/pkg/model"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// validateFilePath validates and normalizes a file path for security and correctness
func validateFilePath(path string) (string, error) {
	if path == "" {
		return "", errors.New("file path cannot be empty")
	}

	// Clean the path to remove . and .. components
	cleanPath := filepath.Clean(path)

	// Convert to absolute path
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("invalid file path: %w", err)
	}

	// Check if file exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file does not exist: %s", absPath)
		}
		return "", fmt.Errorf("cannot access file: %w", err)
	}

	// Ensure it's a file, not a directory
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file: %s", absPath)
	}

	// Check if file is readable
	file, err := os.Open(absPath)
	if err != nil {
		return "", fmt.Errorf("cannot read file: %w", err)
	}
	file.Close()

	return absPath, nil
}

// validateDirPath validates and normalizes a directory path
func validateDirPath(path string) (string, error) {
	if path == "" {
		return "", errors.New("directory path cannot be empty")
	}

	// Clean the path
	cleanPath := filepath.Clean(path)

	// Convert to absolute path
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("invalid directory path: %w", err)
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("directory does not exist: %s", absPath)
		}
		return "", fmt.Errorf("cannot access directory: %w", err)
	}

	// Ensure it's a directory
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", absPath)
	}

	return absPath, nil
}

var version = "dev"

func main() {
	// Setup logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	rootCmd := &cobra.Command{
		Use:     "qtest",
		Short:   "QTest - AI-powered test generation",
		Long:    `QTest generates comprehensive test suites for your codebase using AI.`,
		Version: version,
	}

	// Add subcommands
	rootCmd.AddCommand(generateCmd())
	rootCmd.AddCommand(generateFileCmd())
	rootCmd.AddCommand(analyzeCmd())
	rootCmd.AddCommand(parseCmd())
	rootCmd.AddCommand(workspaceCmd())
	rootCmd.AddCommand(modelCmd())
	rootCmd.AddCommand(planCmd())
	rootCmd.AddCommand(generateSpecsCmd())
	rootCmd.AddCommand(emitTestsCmd())
	rootCmd.AddCommand(validateCmd())
	rootCmd.AddCommand(contractCmd())
	rootCmd.AddCommand(datagenCmd())
	rootCmd.AddCommand(coverageCmd())
	rootCmd.AddCommand(mutationCmd())
	rootCmd.AddCommand(prCmd())
	rootCmd.AddCommand(jobCmd())
	rootCmd.AddCommand(configCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generateCmd() *cobra.Command {
	var (
		repoURL     string
		tier        string
		maxTests    int
		dryRun      bool
		validate    bool
		runMutation bool
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate tests for a repository",
		Long: `Generate tests for an entire repository using the workspace system.

This command will:
1. Clone the repository (if URL provided) or use local path
2. Build a system model to understand the codebase
3. Generate tests using AI (requires Ollama)
4. Optionally validate generated tests
5. Optionally run mutation testing to evaluate test quality

Examples:
  qtest generate -r https://github.com/user/repo
  qtest generate -r ./local/path -t 1 --dry-run
  qtest generate -r ./local/path --mutation`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Determine if local path or remote URL
			isLocal := !strings.HasPrefix(repoURL, "http")

			if isLocal {
				// Validate local path
				validPath, err := validateDirPath(repoURL)
				if err != nil {
					return fmt.Errorf("invalid path: %w", err)
				}
				repoURL = validPath
			}

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

			if err := router.HealthCheck(); err != nil {
				return fmt.Errorf("LLM not available: %w\nMake sure Ollama is running: ollama serve", err)
			}

			// Create temporary workspace
			wsName := "gen-" + strconv.FormatInt(time.Now().Unix(), 36)
			ws, err := workspace.New(wsName, repoURL, nil)
			if err != nil {
				return fmt.Errorf("failed to create workspace: %w", err)
			}

			fmt.Printf("🚀 Generating tests for: %s\n", repoURL)
			fmt.Printf("   Workspace: %s\n\n", ws.ID)

			// Create runner config
			tierNum, _ := strconv.Atoi(tier)
			runCfg := workspace.DefaultRunConfig()
			runCfg.Tier = llm.Tier(tierNum)
			runCfg.DryRun = dryRun
			runCfg.ValidateTests = validate
			runCfg.MaxTests = maxTests

			// Create v2 runner (uses SystemModel pipeline)
			runner := workspace.NewRunnerV2(ws, router, cfg.GitHubToken, runCfg)

			runner.OnProgress = func(phase string, current, total int, message string) {
				if total > 0 {
					fmt.Printf("\r[%s] %d/%d %s", phase, current, total, message)
				} else {
					fmt.Printf("\r[%s] %s", phase, message)
				}
			}

			runner.OnComplete = func(testFile string, count int) {
				fmt.Printf("\n✓ Written: %s (%d tests)\n", testFile, count)
			}

			// Initialize workspace
			fmt.Println("🔍 Analyzing repository...")
			if err := runner.Initialize(ctx); err != nil {
				return fmt.Errorf("initialization failed: %w", err)
			}

			fmt.Printf("\n📊 Found %d test targets\n\n", ws.State.TotalTargets)

			// Run generation
			fmt.Println("⚡ Generating tests...")
			if err := runner.Run(ctx); err != nil {
				return fmt.Errorf("generation failed: %w", err)
			}

			// Summary
			summary := ws.Summary()
			fmt.Println("\n" + strings.Repeat("=", 50))
			fmt.Printf("✅ Generation complete!\n")
			fmt.Printf("   Completed: %d\n", summary["completed"])
			fmt.Printf("   Failed:    %d\n", summary["failed"])
			fmt.Printf("   Output:    %s\n", ws.Path())

			// Run mutation testing if requested
			if runMutation && !dryRun {
				fmt.Println("\n🧬 Running mutation testing...")
				if err := runRepoMutationTesting(ctx, ws.Path()); err != nil {
					fmt.Printf("⚠️  Mutation testing warning: %v\n", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&repoURL, "repo", "r", "", "Repository URL or local path")
	cmd.Flags().StringVarP(&tier, "tier", "t", "2", "LLM tier (1=fast, 2=balanced, 3=thorough)")
	cmd.Flags().IntVarP(&maxTests, "max", "m", 0, "Maximum tests to generate (0=all)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Don't write test files")
	cmd.Flags().BoolVar(&validate, "validate", false, "Run tests after generation")
	cmd.Flags().BoolVar(&runMutation, "mutation", false, "Run mutation testing after generation")
	cmd.MarkFlagRequired("repo")

	return cmd
}

func generateFileCmd() *cobra.Command {
	var (
		filePath    string
		outputDir   string
		tier        string
		maxTests    int
		write       bool
		runMutation bool
	)

	cmd := &cobra.Command{
		Use:   "generate-file",
		Short: "Generate tests for a single source file",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// Validate file path
			validPath, err := validateFilePath(filePath)
			if err != nil {
				return fmt.Errorf("invalid file path: %w", err)
			}
			filePath = validPath

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

			// Create generator
			gen := generator.NewGenerator(router)

			// Parse tier
			tierNum, _ := strconv.Atoi(tier)
			llmTier := llm.Tier(tierNum)
			if llmTier < 1 || llmTier > 3 {
				llmTier = llm.Tier2
			}

			log.Info().
				Str("file", filePath).
				Int("tier", int(llmTier)).
				Msg("generating tests")

			// Generate tests
			tests, err := gen.GenerateForFile(ctx, filePath, generator.GenerateOptions{
				Tier:     llmTier,
				TestType: dsl.TestTypeUnit,
				MaxTests: maxTests,
			})
			if err != nil {
				return fmt.Errorf("failed to generate tests: %w", err)
			}

			fmt.Printf("\n✅ Generated %d tests:\n\n", len(tests))

			// Write test files if requested
			if write {
				if err := writeTestFiles(filePath, tests, outputDir); err != nil {
					return err
				}

				// Run mutation testing if requested
				if runMutation {
					return runMutationTesting(ctx, filePath, outputDir)
				}
				return nil
			}

			// Otherwise just output DSL
			for i, test := range tests {
				fmt.Printf("--- Test %d: %s ---\n", i+1, test.DSL.Name)
				fmt.Printf("Target: %s\n", test.Function.Name)
				fmt.Println(test.RawYAML)
				fmt.Println()
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Source file to generate tests for")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "", "Output directory (default: same as source)")
	cmd.Flags().StringVarP(&tier, "tier", "t", "2", "LLM tier (1=fast, 2=balanced, 3=thorough)")
	cmd.Flags().IntVarP(&maxTests, "max", "m", 5, "Maximum number of tests to generate")
	cmd.Flags().BoolVarP(&write, "write", "w", false, "Write test files to disk")
	cmd.Flags().BoolVar(&runMutation, "mutation", false, "Run mutation testing after generating tests (requires --write)")
	cmd.MarkFlagRequired("file")

	return cmd
}

func analyzeCmd() *cobra.Command {
	var (
		repoPath    string
		outputFile  string
		verbose     bool
		jsonOut     bool
		withCoverage bool
		showAll     bool
	)

	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze a repository and build system model",
		Long: `Analyze a repository to understand its structure and detect testable code.

This command scans the repository, parses all source files, detects frameworks
(Express, FastAPI, Gin, etc.), and builds a Universal System Model.

The output shows:
- File and function counts by language
- Detected API endpoints
- Prioritized test targets
- Code complexity metrics

Examples:
  qtest analyze                        # Analyze current directory
  qtest analyze -p ./my-project        # Analyze specific path
  qtest analyze -p . -o model.json     # Save model to file
  qtest analyze --json                 # Output as JSON
  qtest analyze --coverage             # Include coverage analysis
  qtest analyze --all                  # Show all test targets`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// Validate path
			validPath, err := validateDirPath(repoPath)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}

			fmt.Printf("🔍 Analyzing: %s\n\n", validPath)

			// Use the model package to build the system model
			repoName := filepath.Base(validPath)
			adapter := model.NewParserAdapter(repoName, "main", "")

			// Register framework supplements
			registry := supplements.NewRegistry()
			for _, supp := range registry.GetAll() {
				adapter.RegisterSupplement(supp)
			}

			// Parse all source files
			p := parser.NewParser()
			fileCount := 0
			funcCount := 0

			err = filepath.Walk(validPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}

				// Skip hidden/vendor directories
				if info.IsDir() {
					name := info.Name()
					if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "__pycache__" {
						return filepath.SkipDir
					}
					return nil
				}

				// Only parse supported files
				ext := strings.ToLower(filepath.Ext(path))
				if !isSupportedExt(ext) {
					return nil
				}

				parsed, err := p.ParseFile(ctx, path)
				if err != nil {
					if verbose {
						fmt.Printf("  ⚠️  %s: %v\n", path, err)
					}
					return nil
				}

				// Add to model
				pf := toModelFile(parsed)
				adapter.AddFile(pf)
				fileCount++
				funcCount += len(parsed.Functions)

				if verbose {
					fmt.Printf("  ✓ %s (%d functions)\n", path, len(parsed.Functions))
				}

				return nil
			})

			if err != nil {
				return fmt.Errorf("scan failed: %w", err)
			}

			fmt.Printf("📄 Scanned %d files, %d functions\n", fileCount, funcCount)
			fmt.Println("🔌 Detecting frameworks...")

			// Build the model
			sysModel, err := adapter.Build()
			if err != nil {
				return fmt.Errorf("failed to build model: %w", err)
			}

			// Build stats
			stats := sysModel.Stats()

			// JSON output mode
			if jsonOut {
				result := map[string]interface{}{
					"path":        validPath,
					"languages":   sysModel.Languages,
					"stats":       stats,
					"endpoints":   sysModel.Endpoints,
					"testTargets": sysModel.TestTargets,
					"modules":     len(sysModel.Modules),
				}
				data, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			// Print summary
			fmt.Println()
			fmt.Println("📊 Analysis Summary")
			fmt.Println(strings.Repeat("─", 40))
			fmt.Printf("   Languages:    %s\n", strings.Join(sysModel.Languages, ", "))
			fmt.Printf("   Modules:      %d\n", stats["modules"])
			fmt.Printf("   Functions:    %d\n", stats["functions"])
			fmt.Printf("   Types:        %d\n", stats["types"])
			fmt.Printf("   Endpoints:    %d\n", stats["endpoints"])
			fmt.Printf("   Test Targets: %d\n", stats["test_targets"])

			// Show endpoints with method colors
			if len(sysModel.Endpoints) > 0 {
				fmt.Println()
				fmt.Println("🌐 API Endpoints:")
				for _, ep := range sysModel.Endpoints {
					methodIcon := getMethodIcon(ep.Method)
					fmt.Printf("   %s %-6s %s → %s\n", methodIcon, ep.Method, ep.Path, ep.Handler)
				}
			}

			// Show test targets with priority indicators
			if len(sysModel.TestTargets) > 0 {
				fmt.Println()
				fmt.Println("🎯 Priority Test Targets:")

				maxShow := 10
				if showAll {
					maxShow = len(sysModel.TestTargets)
				} else if len(sysModel.TestTargets) < maxShow {
					maxShow = len(sysModel.TestTargets)
				}

				for i := 0; i < maxShow; i++ {
					t := sysModel.TestTargets[i]
					priorityIcon := getPriorityIcon(t.Priority)
					kindLabel := formatTargetKind(string(t.Kind))
					fmt.Printf("   %s %d. [%s] %s\n", priorityIcon, i+1, kindLabel, t.Reason)
				}

				if !showAll && len(sysModel.TestTargets) > maxShow {
					fmt.Printf("   ... and %d more (use --all to show all)\n", len(sysModel.TestTargets)-maxShow)
				}
			}

			// Coverage analysis if requested
			if withCoverage {
				fmt.Println()
				fmt.Println("📈 Coverage Analysis:")
				lang := detectProjectLanguage(validPath)
				collector := codecov.NewCollector(validPath, lang)
				report, err := collector.Collect(ctx)
				if err != nil {
					fmt.Printf("   ⚠️  Could not collect coverage: %v\n", err)
				} else {
					qualityIcon := "🔴"
					if report.Percentage >= 80 {
						qualityIcon = "🟢"
					} else if report.Percentage >= 50 {
						qualityIcon = "🟡"
					}
					fmt.Printf("   %s Coverage: %.1f%% (%d/%d lines)\n",
						qualityIcon, report.Percentage, report.CoveredLines, report.TotalLines)

					// Show low coverage files
					lowCovFiles := 0
					for _, f := range report.Files {
						if f.Percentage < 50 {
							lowCovFiles++
						}
					}
					if lowCovFiles > 0 {
						fmt.Printf("   ⚠️  %d files with <50%% coverage\n", lowCovFiles)
					}
				}
			}

			// Save to file if requested
			if outputFile != "" {
				data, err := json.MarshalIndent(sysModel, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal: %w", err)
				}
				if err := os.WriteFile(outputFile, data, 0644); err != nil {
					return fmt.Errorf("failed to write: %w", err)
				}
				fmt.Printf("\n💾 Model saved: %s\n", outputFile)
			}

			fmt.Println()
			fmt.Println("Next steps:")
			fmt.Printf("  qtest generate -r %s    # Generate tests\n", validPath)
			if !withCoverage {
				fmt.Printf("  qtest analyze -p %s --coverage  # Analyze with coverage\n", validPath)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&repoPath, "path", "p", ".", "Path to repository")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Save model to JSON file")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show verbose output")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&withCoverage, "coverage", false, "Include coverage analysis")
	cmd.Flags().BoolVar(&showAll, "all", false, "Show all test targets")

	return cmd
}

// getMethodIcon returns an icon for HTTP method
func getMethodIcon(method string) string {
	switch method {
	case "GET":
		return "🔵"
	case "POST":
		return "🟢"
	case "PUT", "PATCH":
		return "🟡"
	case "DELETE":
		return "🔴"
	default:
		return "⚪"
	}
}

// getPriorityIcon returns an icon for priority level
func getPriorityIcon(priority int) string {
	switch {
	case priority >= 90:
		return "🔴"
	case priority >= 70:
		return "🟠"
	case priority >= 50:
		return "🟡"
	default:
		return "🟢"
	}
}

// formatTargetKind formats the target kind for display
func formatTargetKind(kind string) string {
	switch kind {
	case "endpoint":
		return "API"
	case "function":
		return "FN"
	case "method":
		return "MTH"
	case "class":
		return "CLS"
	default:
		return kind
	}
}

// isSupportedExt checks if file extension is supported for parsing
func isSupportedExt(ext string) bool {
	switch ext {
	case ".go", ".py", ".js", ".jsx", ".ts", ".tsx", ".java":
		return true
	}
	return false
}

// toModelFile converts parser.ParsedFile to model.ParsedFile
func toModelFile(pf *parser.ParsedFile) *model.ParsedFile {
	result := &model.ParsedFile{
		Path:     pf.Path,
		Language: string(pf.Language),
	}

	for _, fn := range pf.Functions {
		params := make([]model.ParserParameter, len(fn.Parameters))
		for i, p := range fn.Parameters {
			params[i] = model.ParserParameter{
				Name:     p.Name,
				Type:     p.Type,
				Default:  p.Default,
				Optional: p.Optional,
			}
		}
		result.Functions = append(result.Functions, model.ParserFunction{
			ID:         fn.ID,
			Name:       fn.Name,
			StartLine:  fn.StartLine,
			EndLine:    fn.EndLine,
			Parameters: params,
			ReturnType: fn.ReturnType,
			Body:       fn.Body,
			Comments:   fn.Comments,
			Exported:   fn.Exported,
			Async:      fn.Async,
			Class:      fn.Class,
		})
	}

	return result
}

func parseCmd() *cobra.Command {
	var filePath string

	cmd := &cobra.Command{
		Use:   "parse",
		Short: "Parse a source file and show extracted functions",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// Validate file path
			validPath, err := validateFilePath(filePath)
			if err != nil {
				return fmt.Errorf("invalid file path: %w", err)
			}
			filePath = validPath

			p := parser.NewParser()
			parsed, err := p.ParseFile(ctx, filePath)
			if err != nil {
				return fmt.Errorf("failed to parse file: %w", err)
			}

			fmt.Printf("📄 File: %s\n", parsed.Path)
			fmt.Printf("🔤 Language: %s\n", parsed.Language)
			fmt.Printf("📦 Functions: %d\n", len(parsed.Functions))
			fmt.Printf("🏛️  Classes: %d\n\n", len(parsed.Classes))

			for i, fn := range parsed.Functions {
				exported := "private"
				if fn.Exported {
					exported = "exported"
				}
				fmt.Printf("%d. %s (%s) [lines %d-%d]\n", i+1, fn.Name, exported, fn.StartLine, fn.EndLine)
				if len(fn.Parameters) > 0 {
					fmt.Printf("   Parameters: ")
					for j, p := range fn.Parameters {
						if j > 0 {
							fmt.Print(", ")
						}
						if p.Type != "" {
							fmt.Printf("%s %s", p.Name, p.Type)
						} else {
							fmt.Print(p.Name)
						}
					}
					fmt.Println()
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Source file to parse")
	cmd.MarkFlagRequired("file")

	return cmd
}

// writeTestFiles writes generated tests to disk using the appropriate adapter
func writeTestFiles(sourceFile string, tests []generator.GeneratedTest, outputDir string) error {
	if len(tests) == 0 {
		return nil
	}

	// Get adapter for source language
	lang := parser.DetectLanguage(sourceFile)
	registry := adapters.NewRegistry()
	adapter, err := registry.GetForLanguage(lang)
	if err != nil {
		return fmt.Errorf("no adapter for language %s: %w", lang, err)
	}

	// Determine output directory
	dir := outputDir
	if dir == "" {
		dir = filepath.Dir(sourceFile)
	}

	// Create output directory if needed
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate test file name
	base := filepath.Base(sourceFile)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	testFile := filepath.Join(dir, name+adapter.TestFileSuffix()+adapter.FileExtension())

	var code string

	// Try TestSpec-based generation first for Go (better assertions)
	if lang == parser.LanguageGo {
		var allSpecs []model.TestSpec
		for _, test := range tests {
			if len(test.TestSpecs) > 0 {
				allSpecs = append(allSpecs, test.TestSpecs...)
			}
		}
		if len(allSpecs) > 0 {
			specAdapter := adapters.NewGoSpecAdapter()
			code, err = specAdapter.GenerateFromSpecs(allSpecs, sourceFile)
			if err != nil {
				log.Warn().Err(err).Msg("TestSpec generation failed, falling back to DSL")
				code = ""
			} else {
				log.Info().Int("specs", len(allSpecs)).Msg("generated test from TestSpecs with proper assertions")
			}
		}
	}

	// Fall back to DSL-based generation
	if code == "" {
		// Combine all DSLs into one with all steps
		combinedDSL := &dsl.TestDSL{
			Version: "1.0",
			Name:    name + "_combined",
			Type:    dsl.TestTypeUnit,
			Target: dsl.TestTarget{
				File: sourceFile,
			},
			Steps: make([]dsl.TestStep, 0),
		}

		for _, test := range tests {
			if test.DSL == nil {
				continue
			}
			// Add all steps from each test
			combinedDSL.Steps = append(combinedDSL.Steps, test.DSL.Steps...)
		}

		if len(combinedDSL.Steps) == 0 {
			return fmt.Errorf("no test steps could be generated")
		}

		// Generate combined test code
		code, err = adapter.Generate(combinedDSL)
		if err != nil {
			return fmt.Errorf("failed to generate test code: %w", err)
		}
	}

	// Write to file
	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		return fmt.Errorf("failed to write test file: %w", err)
	}

	fmt.Printf("📝 Written: %s\n", testFile)

	// Count steps for display
	stepCount := 0
	for _, test := range tests {
		if len(test.TestSpecs) > 0 {
			stepCount += len(test.TestSpecs)
		} else if test.DSL != nil {
			stepCount += len(test.DSL.Steps)
		}
	}
	fmt.Printf("   Tests: %d steps\n", stepCount)

	return nil
}

// runMutationTesting runs mutation testing on a source file after test generation
func runMutationTesting(ctx context.Context, sourceFile, outputDir string) error {
	fmt.Println("\n🧬 Running mutation testing...")

	// Determine test file path
	dir := outputDir
	if dir == "" {
		dir = filepath.Dir(sourceFile)
	}

	// Get adapter for test file naming
	lang := parser.DetectLanguage(sourceFile)
	registry := adapters.NewRegistry()
	adapter, err := registry.GetForLanguage(lang)
	if err != nil {
		return fmt.Errorf("unsupported language for mutation testing: %s", lang)
	}

	base := filepath.Base(sourceFile)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	testFile := filepath.Join(dir, name+adapter.TestFileSuffix()+adapter.FileExtension())

	// Check test file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		return fmt.Errorf("test file not found: %s", testFile)
	}

	// Create mutation runner
	runner := mutation.NewRunner(
		mutation.NewGoMutestingTool(),
		mutation.NewSimpleMutationTool(),
	)

	// Run mutation testing
	cfg := mutation.DefaultConfig()
	result, err := runner.Run(ctx, sourceFile, testFile, cfg)
	if err != nil {
		fmt.Printf("⚠️  Mutation testing failed: %v\n", err)
		return nil // Don't fail the command, just report
	}

	// Display results
	fmt.Printf("\n📊 Mutation Score: %.1f%%", result.Score*100)
	quality := result.Quality()
	switch quality {
	case "good":
		fmt.Printf(" 🟢 (good)\n")
	case "acceptable":
		fmt.Printf(" 🟡 (acceptable)\n")
	default:
		fmt.Printf(" 🔴 (poor)\n")
	}

	fmt.Printf("   Mutants: %d total, %d killed, %d survived\n",
		result.Total, result.Killed, result.Survived)

	if result.Survived > 0 {
		fmt.Println("   Tip: Consider adding tests for edge cases to improve mutation score")
	}

	return nil
}

// runRepoMutationTesting runs mutation testing on all source/test pairs in a repository
func runRepoMutationTesting(ctx context.Context, repoPath string) error {
	// Create mutation runner
	runner := mutation.NewRunner(
		mutation.NewGoMutestingTool(),
		mutation.NewSimpleMutationTool(),
	)

	// Find all source files with corresponding test files
	var pairs []struct{ source, test string }

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Skip non-source files and test files
		ext := filepath.Ext(path)
		if ext != ".go" {
			return nil // For now, only support Go
		}

		// Skip test files themselves
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Check if test file exists
		dir := filepath.Dir(path)
		base := filepath.Base(path)
		name := strings.TrimSuffix(base, ext)
		testFile := filepath.Join(dir, name+"_test.go")

		if _, err := os.Stat(testFile); err == nil {
			pairs = append(pairs, struct{ source, test string }{path, testFile})
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to scan repository: %w", err)
	}

	if len(pairs) == 0 {
		fmt.Println("   No source/test pairs found for mutation testing")
		return nil
	}

	fmt.Printf("   Found %d source/test pairs\n\n", len(pairs))

	// Run mutation testing on each pair
	cfg := mutation.DefaultConfig()
	var totalScore float64
	successCount := 0

	for _, pair := range pairs {
		relSource, _ := filepath.Rel(repoPath, pair.source)
		fmt.Printf("   Testing: %s\n", relSource)

		result, err := runner.Run(ctx, pair.source, pair.test, cfg)
		if err != nil {
			fmt.Printf("     ⚠️  Failed: %v\n", err)
			continue
		}

		totalScore += result.Score
		successCount++

		icon := "🔴"
		if result.Quality() == "good" {
			icon = "🟢"
		} else if result.Quality() == "acceptable" {
			icon = "🟡"
		}
		fmt.Printf("     %s Score: %.1f%% (%d/%d killed)\n",
			icon, result.Score*100, result.Killed, result.Total)
	}

	// Summary
	if successCount > 0 {
		avgScore := totalScore / float64(successCount)
		fmt.Printf("\n📊 Average Mutation Score: %.1f%%\n", avgScore*100)
	}

	return nil
}

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show current configuration",
		Long: `Display current QTest configuration settings.

Configuration is loaded from environment variables. Set these to customize behavior:

  PORT              Server port (default: 8080)
  DATABASE_URL      PostgreSQL connection string
  NATS_URL          NATS server URL
  OLLAMA_URL        Ollama server URL (default: http://localhost:11434)
  OLLAMA_TIER1_MODEL   Fast model (default: qwen2.5-coder:7b)
  OLLAMA_TIER2_MODEL   Balanced model (default: deepseek-coder-v2:16b)
  ANTHROPIC_API_KEY    Anthropic API key for Tier 3
  GITHUB_TOKEN         GitHub token for private repos`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			fmt.Println("⚙️  QTest Configuration")
			fmt.Println(strings.Repeat("─", 50))

			// Server
			fmt.Println("\n📡 Server:")
			fmt.Printf("   Port: %d\n", cfg.Port)
			fmt.Printf("   Env:  %s\n", cfg.Env)

			// LLM
			fmt.Println("\n🤖 LLM:")
			fmt.Printf("   Provider: %s\n", cfg.LLM.DefaultProvider)
			fmt.Printf("   Ollama URL: %s\n", cfg.LLM.OllamaURL)
			fmt.Printf("   Tier 1 Model: %s\n", cfg.LLM.OllamaTier1)
			fmt.Printf("   Tier 2 Model: %s\n", cfg.LLM.OllamaTier2)
			if cfg.LLM.AnthropicKey != "" {
				fmt.Printf("   Tier 3 Model: %s (Anthropic)\n", cfg.LLM.AnthropicTier3)
				fmt.Printf("   Anthropic Key: %s...%s\n", cfg.LLM.AnthropicKey[:4], cfg.LLM.AnthropicKey[len(cfg.LLM.AnthropicKey)-4:])
			} else {
				fmt.Println("   Anthropic: not configured")
			}

			// Infrastructure
			fmt.Println("\n🗄️  Infrastructure:")
			fmt.Printf("   Database: %s\n", maskConnectionString(cfg.DatabaseURL))
			fmt.Printf("   NATS: %s\n", cfg.NATSURL)
			fmt.Printf("   Redis: %s\n", cfg.RedisURL)

			// GitHub
			fmt.Println("\n🐙 GitHub:")
			if cfg.GitHubToken != "" {
				fmt.Printf("   Token: %s...%s\n", cfg.GitHubToken[:4], cfg.GitHubToken[len(cfg.GitHubToken)-4:])
			} else {
				fmt.Println("   Token: not configured")
			}

			// Validate
			if err := cfg.Validate(); err != nil {
				fmt.Printf("\n⚠️  Warning: %s\n", err)
			}

			return nil
		},
	}

	return cmd
}

// maskConnectionString masks password in connection strings
func maskConnectionString(connStr string) string {
	// Simple masking for postgres://user:pass@host format
	if strings.Contains(connStr, "@") && strings.Contains(connStr, ":") {
		parts := strings.SplitN(connStr, "://", 2)
		if len(parts) == 2 {
			userHost := strings.SplitN(parts[1], "@", 2)
			if len(userHost) == 2 {
				userPass := strings.SplitN(userHost[0], ":", 2)
				if len(userPass) == 2 {
					return parts[0] + "://" + userPass[0] + ":****@" + userHost[1]
				}
			}
		}
	}
	return connStr
}
