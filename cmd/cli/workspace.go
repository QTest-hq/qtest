package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/QTest-hq/qtest/internal/config"
	"github.com/QTest-hq/qtest/internal/llm"
	"github.com/QTest-hq/qtest/internal/workspace"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func workspaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "workspace",
		Aliases: []string{"ws"},
		Short:   "Manage workspaces for test generation",
	}

	cmd.AddCommand(workspaceInitCmd())
	cmd.AddCommand(workspaceListCmd())
	cmd.AddCommand(workspaceStatusCmd())
	cmd.AddCommand(workspaceRunCmd())
	cmd.AddCommand(workspaceRunV2Cmd()) // New pipeline
	cmd.AddCommand(workspaceResumeCmd())
	cmd.AddCommand(workspaceValidateCmd())
	cmd.AddCommand(workspaceCoverageCmd())

	return cmd
}

func workspaceInitCmd() *cobra.Command {
	var (
		name   string
		branch string
	)

	cmd := &cobra.Command{
		Use:   "init <repo-url>",
		Short: "Initialize a new workspace from a repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoURL := args[0]

			if name == "" {
				// Extract name from URL
				name = extractRepoName(repoURL)
			}

			log.Info().Str("repo", repoURL).Str("name", name).Msg("initializing workspace")

			// Create workspace
			ws, err := workspace.New(name, repoURL, nil)
			if err != nil {
				return fmt.Errorf("failed to create workspace: %w", err)
			}

			fmt.Printf("Workspace created: %s\n", ws.ID)
			fmt.Printf("Path: %s\n", ws.Path())
			fmt.Printf("\nNext steps:\n")
			fmt.Printf("  qtest workspace run %s    # Start generating tests\n", ws.ID)

			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Workspace name")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch for generated tests")

	return cmd
}

func workspaceListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all workspaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			workspaces, err := workspace.ListWorkspaces(nil)
			if err != nil {
				return err
			}

			if len(workspaces) == 0 {
				fmt.Println("No workspaces found.")
				fmt.Println("Create one with: qtest workspace init <repo-url>")
				return nil
			}

			fmt.Printf("%-10s %-20s %-15s %s\n", "ID", "NAME", "STATUS", "PROGRESS")
			fmt.Println("---------------------------------------------------------------")

			for _, ws := range workspaces {
				progress := fmt.Sprintf("%d/%d", ws.State.Completed, ws.State.TotalTargets)
				fmt.Printf("%-10s %-20s %-15s %s\n",
					ws.ID,
					truncate(ws.Name, 18),
					ws.State.Phase,
					progress,
				)
			}

			return nil
		},
	}
}

func workspaceStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <workspace-id>",
		Short: "Show workspace status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := workspace.LoadByID(args[0], nil)
			if err != nil {
				return fmt.Errorf("workspace not found: %w", err)
			}

			summary := ws.Summary()

			fmt.Printf("Workspace: %s (%s)\n", ws.Name, ws.ID)
			fmt.Printf("Repository: %s\n", ws.RepoURL)
			fmt.Printf("Branch: %s\n", ws.Branch)
			fmt.Printf("Language: %s\n", ws.Language)
			fmt.Printf("\n")
			fmt.Printf("Phase: %s\n", summary["phase"])
			fmt.Printf("Progress: %s\n", summary["progress"])
			fmt.Printf("\n")
			fmt.Printf("  Total targets:  %d\n", summary["total"])
			fmt.Printf("  Completed:      %d\n", summary["completed"])
			fmt.Printf("  Failed:         %d\n", summary["failed"])
			fmt.Printf("  Pending:        %d\n", summary["pending"])

			return nil
		},
	}
}

func workspaceRunCmd() *cobra.Command {
	var (
		tier       int
		commitEach bool
		dryRun     bool
		validate   bool
		coverage   bool
	)

	cmd := &cobra.Command{
		Use:   "run <workspace-id>",
		Short: "Run test generation for a workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load workspace
			ws, err := workspace.LoadByID(args[0], nil)
			if err != nil {
				return fmt.Errorf("workspace not found: %w", err)
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
				return fmt.Errorf("LLM not available: %w\nMake sure Ollama is running", err)
			}

			// Create runner
			runCfg := workspace.DefaultRunConfig()
			runCfg.Tier = llm.Tier(tier)
			runCfg.CommitEach = commitEach
			runCfg.DryRun = dryRun
			runCfg.ValidateTests = validate

			runner := workspace.NewRunner(ws, router, cfg.GitHubToken, runCfg)

			// Setup progress callbacks
			runner.OnProgress = func(current, total int, target *workspace.TargetState) {
				fmt.Printf("\r[%d/%d] Generating test for %s...    ",
					current, total, target.Name)
			}

			runner.OnComplete = func(target *workspace.TargetState, testFile string) {
				fmt.Printf("\r[%d/%d] ✓ %s -> %s\n",
					ws.State.Completed+1, ws.State.TotalTargets,
					target.Name, testFile)
			}

			runner.OnError = func(target *workspace.TargetState, err error) {
				fmt.Printf("\r[%d/%d] ✗ %s: %s\n",
					ws.State.Completed+ws.State.Failed+1, ws.State.TotalTargets,
					target.Name, err)
			}

			// Setup graceful shutdown
			ctx, cancel := context.WithCancel(context.Background())
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				<-sigCh
				fmt.Println("\n\nPausing... (Ctrl+C again to force quit)")
				runner.Pause()
				cancel()
			}()

			// Initialize if needed
			if ws.State.Phase == workspace.PhaseInit {
				fmt.Println("Initializing workspace...")
				if err := runner.Initialize(ctx); err != nil {
					return fmt.Errorf("initialization failed: %w", err)
				}
				fmt.Printf("Found %d testable functions\n\n", ws.State.TotalTargets)
			}

			// Run generation
			fmt.Println("Starting test generation...")
			fmt.Println("Press Ctrl+C to pause")

			if err := runner.Run(ctx); err != nil {
				if ctx.Err() != nil {
					fmt.Println("\nWorkspace paused. Resume with:")
					fmt.Printf("  qtest workspace resume %s\n", ws.ID)
					return nil
				}
				return err
			}

			// Summary
			fmt.Println("\n" + repeatStr("=", 50))
			summary := ws.Summary()
			fmt.Printf("Generation complete!\n")
			fmt.Printf("  Completed: %d\n", summary["completed"])
			fmt.Printf("  Failed:    %d\n", summary["failed"])

			// Collect coverage if requested
			if coverage && !dryRun && ws.Language != "" {
				fmt.Println("\nCollecting coverage...")
				collector := workspace.NewCoverageCollector(ws)
				report, err := collector.CollectAll(ctx)
				if err != nil {
					log.Warn().Err(err).Msg("coverage collection failed")
				} else {
					fmt.Printf("  Coverage: %.1f%% (%d/%d lines)\n",
						report.Summary.CoveragePercent,
						report.Summary.CoveredLines,
						report.Summary.TotalLines)
				}
			}

			return nil
		},
	}

	cmd.Flags().IntVarP(&tier, "tier", "t", 2, "LLM tier (1=fast, 2=balanced, 3=thorough)")
	cmd.Flags().BoolVar(&commitEach, "commit", true, "Commit after each test")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Don't write test files")
	cmd.Flags().BoolVar(&validate, "validate", false, "Run tests after generation to verify they pass")
	cmd.Flags().BoolVar(&coverage, "coverage", false, "Collect code coverage after generation")

	return cmd
}

func workspaceRunV2Cmd() *cobra.Command {
	var (
		tier       int
		commitEach bool
		dryRun     bool
		maxTests   int
	)

	cmd := &cobra.Command{
		Use:   "run-v2 <workspace-id>",
		Short: "Run test generation with new SystemModel pipeline",
		Long: `Runs the new SystemModel-based test generation pipeline:

1. Builds Universal System Model from repository
2. Detects API endpoints (Express, FastAPI, Gin)
3. Generates prioritized test plan
4. Uses LLM to create test specifications
5. Emits test code (supertest, pytest, go-http)

This is the recommended command for generating complete test suites.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load workspace
			ws, err := workspace.LoadByID(args[0], nil)
			if err != nil {
				return fmt.Errorf("workspace not found: %w", err)
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
				return fmt.Errorf("LLM not available: %w\nMake sure Ollama is running", err)
			}

			// Create runner config
			runCfg := workspace.DefaultRunConfig()
			runCfg.Tier = llm.Tier(tier)
			runCfg.CommitEach = commitEach
			runCfg.DryRun = dryRun
			runCfg.MaxTests = maxTests

			// Create v2 runner
			runner := workspace.NewRunnerV2(ws, router, cfg.GitHubToken, runCfg)

			// Setup progress callback
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

			// Setup graceful shutdown
			ctx, cancel := context.WithCancel(context.Background())
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				<-sigCh
				fmt.Println("\n\nPausing... (progress saved)")
				runner.Pause()
				cancel()
			}()

			// Initialize if needed
			if ws.State.Phase == workspace.PhaseInit || ws.State.Phase == "" {
				fmt.Println("🔍 Initializing workspace (building model, planning tests)...")
				if err := runner.Initialize(ctx); err != nil {
					return fmt.Errorf("initialization failed: %w", err)
				}
				fmt.Println()
			}

			// Run generation
			fmt.Printf("\n🚀 Starting test generation (%d targets)...\n", ws.State.TotalTargets)
			fmt.Println("Press Ctrl+C to pause")

			if err := runner.Run(ctx); err != nil {
				if ctx.Err() != nil {
					fmt.Println("\nWorkspace paused. Resume with:")
					fmt.Printf("  qtest workspace run-v2 %s\n", ws.ID)
					return nil
				}
				return err
			}

			// Summary
			fmt.Println("\n" + repeatStr("=", 50))
			summary := ws.Summary()
			fmt.Printf("Generation complete!\n")
			fmt.Printf("  Completed: %d\n", summary["completed"])
			fmt.Printf("  Failed:    %d\n", summary["failed"])
			fmt.Printf("  Artifacts: %s/artifacts/\n", ws.Path())

			return nil
		},
	}

	cmd.Flags().IntVarP(&tier, "tier", "t", 1, "LLM tier (1=fast, 2=balanced, 3=thorough)")
	cmd.Flags().BoolVar(&commitEach, "commit", true, "Commit after each batch")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Don't write test files")
	cmd.Flags().IntVar(&maxTests, "max", 0, "Maximum tests to generate (0=all)")

	return cmd
}

func workspaceResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume <workspace-id>",
		Short: "Resume a paused workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Delegate to run command
			return workspaceRunCmd().RunE(cmd, args)
		},
	}
}

func workspaceValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <workspace-id>",
		Short: "Run and validate generated tests",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Load workspace
			ws, err := workspace.LoadByID(args[0], nil)
			if err != nil {
				return fmt.Errorf("workspace not found: %w", err)
			}

			// Check if there are tests to validate
			testsToValidate := 0
			for _, target := range ws.State.Targets {
				if target.TestFile != "" && target.Status == workspace.StatusCompleted {
					testsToValidate++
				}
			}

			if testsToValidate == 0 {
				fmt.Println("No tests to validate.")
				fmt.Println("Generate tests first with: qtest workspace run", ws.ID)
				return nil
			}

			fmt.Printf("Validating %d generated tests...\n\n", testsToValidate)

			// Create validator and run
			validator := workspace.NewTestValidator(ws)
			results, err := validator.ValidateAll(ctx)
			if err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}

			// Print results
			for _, r := range results {
				status := "✓"
				if !r.Passed {
					status = "✗"
				}
				fmt.Printf("%s %s (%dms)\n", status, r.Target, r.Duration.Milliseconds())
				if !r.Passed && r.Error != "" {
					fmt.Printf("  Error: %s\n", r.Error)
				}
			}

			// Print summary
			summary := workspace.Summarize(results)
			fmt.Println()
			fmt.Println(repeatStr("=", 50))
			fmt.Printf("Validation complete!\n")
			fmt.Printf("  Total:   %d\n", summary.Total)
			fmt.Printf("  Passed:  %d\n", summary.Passed)
			fmt.Printf("  Failed:  %d\n", summary.Failed)
			fmt.Printf("  Pass Rate: %.1f%%\n", summary.PassRate)

			if len(summary.FailedTests) > 0 {
				fmt.Println("\nFailed tests:")
				for _, f := range summary.FailedTests {
					fmt.Printf("  - %s\n", f)
				}
			}

			return nil
		},
	}
}

func workspaceCoverageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "coverage <workspace-id>",
		Short: "Collect code coverage from generated tests",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Load workspace
			ws, err := workspace.LoadByID(args[0], nil)
			if err != nil {
				return fmt.Errorf("workspace not found: %w", err)
			}

			if ws.Language == "" {
				return fmt.Errorf("workspace language not detected, run tests first")
			}

			fmt.Printf("Collecting coverage for %s project...\n\n", ws.Language)

			// Create collector and run
			collector := workspace.NewCoverageCollector(ws)
			report, err := collector.CollectAll(ctx)
			if err != nil {
				return fmt.Errorf("coverage collection failed: %w", err)
			}

			// Print results
			fmt.Printf("%-50s %10s %10s %8s\n", "FILE", "COVERED", "TOTAL", "PCT")
			fmt.Println(repeatStr("-", 80))

			for _, f := range report.Files {
				pct := fmt.Sprintf("%.1f%%", f.CoveragePercent)
				name := f.Path
				if len(name) > 48 {
					name = "..." + name[len(name)-45:]
				}
				fmt.Printf("%-50s %10d %10d %8s\n", name, f.CoveredLines, f.TotalLines, pct)
			}

			// Print summary
			fmt.Println(repeatStr("-", 80))
			fmt.Printf("%-50s %10d %10d %8.1f%%\n",
				"TOTAL",
				report.Summary.CoveredLines,
				report.Summary.TotalLines,
				report.Summary.CoveragePercent)

			fmt.Printf("\nCoverage report saved to: %s/artifacts/coverage.json\n", ws.Path())

			return nil
		},
	}
}

// Helper functions

func extractRepoName(url string) string {
	// Simple extraction from URL
	parts := []string{}
	for _, sep := range []string{"/", ":"} {
		for _, p := range parts {
			parts = append(parts, splitString(p, sep)...)
		}
		if len(parts) == 0 {
			parts = splitString(url, sep)
		}
	}

	if len(parts) > 0 {
		name := parts[len(parts)-1]
		name = trimSuffix(name, ".git")
		return name
	}
	return "unnamed"
}

func splitString(s, sep string) []string {
	result := []string{}
	for _, p := range split(s, sep) {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func split(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSuffix(s, suffix string) string {
	if len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix {
		return s[:len(s)-len(suffix)]
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n-2] + ".."
	}
	return s
}

// repeatStr returns a string of n copies of s
func repeatStr(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
