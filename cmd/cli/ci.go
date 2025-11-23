package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/QTest-hq/qtest/internal/ci"
	"github.com/spf13/cobra"
)

func ciCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ci",
		Short: "Generate CI/CD workflow configurations",
		Long: `Generate CI/CD workflow configurations for various platforms.

Supported platforms:
  - github-actions  GitHub Actions (.github/workflows/ci.yml)
  - gitlab-ci       GitLab CI (.gitlab-ci.yml)
  - circleci        CircleCI (.circleci/config.yml)

Supported languages:
  - go              Go (golang)
  - python          Python
  - javascript      JavaScript
  - typescript      TypeScript
  - java            Java

Examples:
  qtest ci generate                           # Auto-detect and generate GitHub Actions
  qtest ci generate -p gitlab-ci              # Generate GitLab CI config
  qtest ci generate -l go -p github-actions   # Generate for Go + GitHub Actions
  qtest ci detect                             # Show detected project config
  qtest ci preview                            # Preview workflow without writing`,
	}

	cmd.AddCommand(ciGenerateCmd())
	cmd.AddCommand(ciDetectCmd())
	cmd.AddCommand(ciPreviewCmd())
	cmd.AddCommand(ciListCmd())

	return cmd
}

func ciGenerateCmd() *cobra.Command {
	var (
		platform        string
		language        string
		projectDir      string
		outputPath      string
		coverageEnabled bool
		threshold       float64
		qtestEnabled    bool
		qtestTier       int
		qtestMaxTests   int
		services        []string
		branches        []string
		force           bool
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate CI workflow configuration",
		Long: `Generate a CI/CD workflow configuration file for your project.

The command will auto-detect your project's language and framework,
then generate an appropriate workflow configuration.

Examples:
  qtest ci generate                              # Auto-detect everything
  qtest ci generate -p github-actions            # Generate GitHub Actions
  qtest ci generate -p gitlab-ci -l python       # GitLab CI for Python
  qtest ci generate --qtest --qtest-tier 2       # Include QTest generation step
  qtest ci generate --services postgres,redis    # Add service containers`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate and normalize directory
			if projectDir == "." {
				var err error
				projectDir, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to get working directory: %w", err)
				}
			}

			absDir, err := validateDirPath(projectDir)
			if err != nil {
				return err
			}
			projectDir = absDir

			// Detect or use config
			var cfg *ci.Config
			if language == "" {
				// Auto-detect
				cfg, err = ci.DetectConfig(projectDir)
				if err != nil {
					return fmt.Errorf("failed to detect project config: %w", err)
				}
				fmt.Printf("Detected: %s project", cfg.Language)
				if cfg.Framework != "" {
					fmt.Printf(" (framework: %s)", cfg.Framework)
				}
				fmt.Println()
			} else {
				// Use specified language
				cfg = ci.DefaultConfig(ci.Language(language))
			}

			// Apply overrides
			cfg.CoverageEnabled = coverageEnabled
			cfg.CoverageThreshold = threshold
			cfg.QTestEnabled = qtestEnabled
			cfg.QTestTier = qtestTier
			cfg.QTestMaxTests = qtestMaxTests

			if len(services) > 0 {
				cfg.Services = services
			}
			if len(branches) > 0 {
				cfg.Branches = branches
			}

			// Default platform
			if platform == "" {
				platform = "github-actions"
			}

			// Create generator
			gen := ci.NewGenerator(cfg)

			// Check if file already exists
			var expectedPath string
			switch ci.Platform(platform) {
			case ci.PlatformGitHubActions:
				expectedPath = projectDir + "/.github/workflows/ci.yml"
			case ci.PlatformGitLabCI:
				expectedPath = projectDir + "/.gitlab-ci.yml"
			case ci.PlatformCircleCI:
				expectedPath = projectDir + "/.circleci/config.yml"
			}

			if !force {
				if _, err := os.Stat(expectedPath); err == nil {
					return fmt.Errorf("workflow file already exists: %s\nUse --force to overwrite", expectedPath)
				}
			}

			// Generate and write
			outputFile := outputPath
			if outputFile == "" {
				outputFile, err = gen.WriteWorkflow(ci.Platform(platform), projectDir)
			} else {
				content, err := gen.GenerateWorkflow(ci.Platform(platform))
				if err != nil {
					return err
				}
				err = os.WriteFile(outputFile, []byte(content), 0644)
				if err != nil {
					return fmt.Errorf("failed to write workflow: %w", err)
				}
			}

			if err != nil {
				return fmt.Errorf("failed to generate workflow: %w", err)
			}

			fmt.Printf("Generated %s workflow: %s\n", platform, outputFile)
			fmt.Println()
			fmt.Printf("Configuration:\n")
			fmt.Printf("  Language:     %s\n", cfg.Language)
			fmt.Printf("  Framework:    %s\n", cfg.Framework)
			fmt.Printf("  Coverage:     %v (threshold: %.0f%%)\n", cfg.CoverageEnabled, cfg.CoverageThreshold)
			fmt.Printf("  QTest:        %v\n", cfg.QTestEnabled)
			if len(cfg.Services) > 0 {
				fmt.Printf("  Services:     %s\n", strings.Join(cfg.Services, ", "))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&platform, "platform", "p", "", "CI platform (github-actions, gitlab-ci, circleci)")
	cmd.Flags().StringVarP(&language, "language", "l", "", "Language (go, python, javascript, typescript, java)")
	cmd.Flags().StringVarP(&projectDir, "dir", "d", ".", "Project directory")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path (default: platform-specific)")
	cmd.Flags().BoolVar(&coverageEnabled, "coverage", true, "Enable coverage collection")
	cmd.Flags().Float64VarP(&threshold, "threshold", "t", 80.0, "Coverage threshold percentage")
	cmd.Flags().BoolVar(&qtestEnabled, "qtest", false, "Include QTest generation step")
	cmd.Flags().IntVar(&qtestTier, "qtest-tier", 2, "QTest LLM tier (1-3)")
	cmd.Flags().IntVar(&qtestMaxTests, "qtest-max", 10, "QTest max tests per run")
	cmd.Flags().StringSliceVar(&services, "services", nil, "Services to include (postgres, redis, mysql, mongo)")
	cmd.Flags().StringSliceVar(&branches, "branches", []string{"main", "master"}, "Branches to run CI on")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing workflow file")

	return cmd
}

func ciDetectCmd() *cobra.Command {
	var (
		projectDir string
		jsonOut    bool
	)

	cmd := &cobra.Command{
		Use:   "detect",
		Short: "Detect project configuration",
		Long: `Analyze a project directory and detect its language, framework, and suggested CI configuration.

Examples:
  qtest ci detect                    # Detect current directory
  qtest ci detect -d ./myproject     # Detect specific directory
  qtest ci detect --json             # Output as JSON`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate directory
			if projectDir == "." {
				var err error
				projectDir, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to get working directory: %w", err)
				}
			}

			absDir, err := validateDirPath(projectDir)
			if err != nil {
				return err
			}

			// Detect config
			cfg, err := ci.DetectConfig(absDir)
			if err != nil {
				return fmt.Errorf("failed to detect config: %w", err)
			}

			if jsonOut {
				data, _ := json.MarshalIndent(cfg, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			// Display results
			fmt.Printf("Project Detection Results\n")
			fmt.Printf("=========================\n\n")

			fmt.Printf("Project:    %s\n", cfg.ProjectName)
			fmt.Printf("Language:   %s\n", cfg.Language)
			if cfg.Framework != "" {
				fmt.Printf("Framework:  %s\n", cfg.Framework)
			}

			fmt.Printf("\nBuild Configuration:\n")
			fmt.Printf("  Build:    %s\n", cfg.BuildCommand)
			fmt.Printf("  Test:     %s\n", cfg.TestCommand)
			fmt.Printf("  Lint:     %s\n", cfg.LintCommand)

			fmt.Printf("\nVersion Settings:\n")
			switch cfg.Language {
			case ci.LanguageGo:
				fmt.Printf("  Go:       %s\n", cfg.GoVersion)
			case ci.LanguagePython:
				fmt.Printf("  Python:   %s\n", cfg.PythonVersion)
			case ci.LanguageJavaScript, ci.LanguageTypeScript:
				fmt.Printf("  Node.js:  %s\n", cfg.NodeVersion)
			case ci.LanguageJava:
				fmt.Printf("  Java:     %s\n", cfg.JavaVersion)
			}

			if len(cfg.Services) > 0 {
				fmt.Printf("\nDetected Services:\n")
				for _, svc := range cfg.Services {
					fmt.Printf("  - %s\n", svc)
				}
			}

			fmt.Printf("\nCoverage Settings:\n")
			fmt.Printf("  Enabled:   %v\n", cfg.CoverageEnabled)
			fmt.Printf("  Threshold: %.0f%%\n", cfg.CoverageThreshold)

			return nil
		},
	}

	cmd.Flags().StringVarP(&projectDir, "dir", "d", ".", "Project directory to analyze")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")

	return cmd
}

func ciPreviewCmd() *cobra.Command {
	var (
		platform   string
		language   string
		projectDir string
	)

	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Preview generated workflow without writing",
		Long: `Preview the CI workflow that would be generated without writing to disk.

Examples:
  qtest ci preview                           # Preview GitHub Actions workflow
  qtest ci preview -p gitlab-ci              # Preview GitLab CI config
  qtest ci preview -p circleci -l python     # Preview CircleCI for Python`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate directory
			if projectDir == "." {
				var err error
				projectDir, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to get working directory: %w", err)
				}
			}

			absDir, err := validateDirPath(projectDir)
			if err != nil {
				return err
			}

			// Detect or use config
			var cfg *ci.Config
			if language == "" {
				cfg, err = ci.DetectConfig(absDir)
				if err != nil {
					return fmt.Errorf("failed to detect config: %w", err)
				}
			} else {
				cfg = ci.DefaultConfig(ci.Language(language))
			}

			// Default platform
			if platform == "" {
				platform = "github-actions"
			}

			// Generate preview
			gen := ci.NewGenerator(cfg)
			content, err := gen.GenerateWorkflow(ci.Platform(platform))
			if err != nil {
				return fmt.Errorf("failed to generate workflow: %w", err)
			}

			// Display with header
			fmt.Printf("# Preview: %s workflow for %s project\n", platform, cfg.Language)
			fmt.Printf("# Framework: %s\n", cfg.Framework)
			fmt.Printf("# =====================================\n\n")
			fmt.Print(content)

			return nil
		},
	}

	cmd.Flags().StringVarP(&platform, "platform", "p", "", "CI platform (github-actions, gitlab-ci, circleci)")
	cmd.Flags().StringVarP(&language, "language", "l", "", "Language (go, python, javascript, typescript, java)")
	cmd.Flags().StringVarP(&projectDir, "dir", "d", ".", "Project directory")

	return cmd
}

func ciListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List supported platforms and languages",
		Long:  `Display all supported CI platforms and programming languages.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Supported CI Platforms:\n")
			fmt.Printf("=======================\n")
			for _, p := range ci.SupportedPlatforms() {
				description := ""
				switch p {
				case ci.PlatformGitHubActions:
					description = "GitHub Actions (.github/workflows/ci.yml)"
				case ci.PlatformGitLabCI:
					description = "GitLab CI (.gitlab-ci.yml)"
				case ci.PlatformCircleCI:
					description = "CircleCI (.circleci/config.yml)"
				}
				fmt.Printf("  %-16s %s\n", p, description)
			}

			fmt.Printf("\nSupported Languages:\n")
			fmt.Printf("====================\n")
			for _, l := range ci.SupportedLanguages() {
				description := ""
				switch l {
				case ci.LanguageGo:
					description = "Go (golang) - go test, golangci-lint"
				case ci.LanguagePython:
					description = "Python - pytest, ruff"
				case ci.LanguageJavaScript:
					description = "JavaScript - jest, npm test"
				case ci.LanguageTypeScript:
					description = "TypeScript - jest, npm test"
				case ci.LanguageJava:
					description = "Java - JUnit, Maven"
				}
				fmt.Printf("  %-12s %s\n", l, description)
			}

			fmt.Printf("\nSupported Services:\n")
			fmt.Printf("===================\n")
			fmt.Printf("  postgres     PostgreSQL database\n")
			fmt.Printf("  redis        Redis cache\n")
			fmt.Printf("  mysql        MySQL database\n")
			fmt.Printf("  mongo        MongoDB database\n")
		},
	}

	return cmd
}
