package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/QTest-hq/qtest/internal/config"
	"github.com/QTest-hq/qtest/internal/generator"
	"github.com/QTest-hq/qtest/internal/llm"
	"github.com/QTest-hq/qtest/internal/parser"
	"github.com/QTest-hq/qtest/pkg/dsl"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

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

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generateCmd() *cobra.Command {
	var (
		repoURL   string
		outputDir string
		framework string
		tier      string
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate tests for a repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Generating tests for %s\n", repoURL)
			fmt.Printf("Output: %s, Framework: %s, Tier: %s\n", outputDir, framework, tier)
			// TODO: Implement full repo test generation
			return fmt.Errorf("full repo generation not yet implemented, use 'generate-file' for single file")
		},
	}

	cmd.Flags().StringVarP(&repoURL, "repo", "r", "", "GitHub repository URL")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "./tests", "Output directory for generated tests")
	cmd.Flags().StringVarP(&framework, "framework", "f", "auto", "Test framework (jest, pytest, go, auto)")
	cmd.Flags().StringVarP(&tier, "tier", "t", "2", "LLM tier (1=fast, 2=balanced, 3=thorough)")
	cmd.MarkFlagRequired("repo")

	return cmd
}

func generateFileCmd() *cobra.Command {
	var (
		filePath  string
		outputDir string
		tier      string
		maxTests  int
	)

	cmd := &cobra.Command{
		Use:   "generate-file",
		Short: "Generate tests for a single source file",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

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

			// Output results
			fmt.Printf("\n✅ Generated %d tests:\n\n", len(tests))
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
	cmd.Flags().StringVarP(&outputDir, "output", "o", "./tests", "Output directory for generated tests")
	cmd.Flags().StringVarP(&tier, "tier", "t", "2", "LLM tier (1=fast, 2=balanced, 3=thorough)")
	cmd.Flags().IntVarP(&maxTests, "max", "m", 5, "Maximum number of tests to generate")
	cmd.MarkFlagRequired("file")

	return cmd
}

func analyzeCmd() *cobra.Command {
	var repoPath string

	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze a repository and build system model",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Analyzing repository at %s\n", repoPath)
			// TODO: Implement analysis
			return nil
		},
	}

	cmd.Flags().StringVarP(&repoPath, "path", "p", ".", "Path to repository")

	return cmd
}

func parseCmd() *cobra.Command {
	var filePath string

	cmd := &cobra.Command{
		Use:   "parse",
		Short: "Parse a source file and show extracted functions",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

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
