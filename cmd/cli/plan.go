package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/QTest-hq/qtest/internal/config"
	"github.com/QTest-hq/qtest/internal/llm"
	"github.com/QTest-hq/qtest/internal/specgen"
	"github.com/QTest-hq/qtest/pkg/model"
	"github.com/spf13/cobra"
)

func planCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Generate test plans and specifications",
	}

	cmd.AddCommand(planGenerateCmd())
	cmd.AddCommand(planShowCmd())

	return cmd
}

func planGenerateCmd() *cobra.Command {
	var (
		modelFile  string
		outputFile string
		maxTests   int
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a test plan from a system model",
		Long: `Analyzes a system model and generates prioritized test intents.
This is a heuristic-based planning step that doesn't require LLM.

Test intents are prioritized based on:
- API endpoints (always high priority)
- Function risk scores (complexity, centrality)
- Export status (public functions first)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load system model
			data, err := os.ReadFile(modelFile)
			if err != nil {
				return fmt.Errorf("failed to read model: %w", err)
			}

			var sysModel model.SystemModel
			if err := json.Unmarshal(data, &sysModel); err != nil {
				return fmt.Errorf("failed to parse model: %w", err)
			}

			fmt.Printf("📊 Loaded model: %s\n", sysModel.Repository)
			fmt.Printf("   Functions: %d, Endpoints: %d\n\n", len(sysModel.Functions), len(sysModel.Endpoints))

			// Create planner
			cfg := model.DefaultPlannerConfig()
			if maxTests > 0 {
				cfg.MaxIntents = maxTests
			}
			planner := model.NewPlanner(cfg)

			// Generate plan
			fmt.Println("🎯 Generating test plan...")
			plan, err := planner.Plan(&sysModel)
			if err != nil {
				return fmt.Errorf("planning failed: %w", err)
			}

			// Print summary
			stats := plan.Stats()
			fmt.Println()
			fmt.Println("📋 Test Plan Summary")
			fmt.Println(strings.Repeat("─", 40))
			fmt.Printf("   Total intents: %d\n", stats["total"])
			fmt.Printf("   API tests:     %d\n", stats["api"])
			fmt.Printf("   Unit tests:    %d\n", stats["unit"])
			fmt.Printf("   E2E tests:     %d\n", stats["e2e"])
			fmt.Println()
			fmt.Printf("   High priority: %d\n", stats["high"])
			fmt.Printf("   Medium:        %d\n", stats["medium"])
			fmt.Printf("   Low:           %d\n", stats["low"])

			// Show sample intents
			fmt.Println()
			fmt.Println("🔍 Sample intents:")
			maxShow := 10
			if len(plan.Intents) < maxShow {
				maxShow = len(plan.Intents)
			}
			for i := 0; i < maxShow; i++ {
				intent := plan.Intents[i]
				fmt.Printf("   [%s] %s - %s\n", intent.Level, intent.Priority, intent.Reason)
			}
			if len(plan.Intents) > maxShow {
				fmt.Printf("   ... and %d more\n", len(plan.Intents)-maxShow)
			}

			// Save to file if requested
			if outputFile != "" {
				data, err := json.MarshalIndent(plan, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal plan: %w", err)
				}
				if err := os.WriteFile(outputFile, data, 0644); err != nil {
					return fmt.Errorf("failed to write plan: %w", err)
				}
				fmt.Printf("\n💾 Plan saved to: %s\n", outputFile)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&modelFile, "model", "m", "", "System model JSON file (required)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file for plan JSON")
	cmd.Flags().IntVar(&maxTests, "max", 0, "Maximum number of test intents")
	cmd.MarkFlagRequired("model")

	return cmd
}

func planShowCmd() *cobra.Command {
	var planFile string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a saved test plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(planFile)
			if err != nil {
				return fmt.Errorf("failed to read plan: %w", err)
			}

			var plan model.TestPlan
			if err := json.Unmarshal(data, &plan); err != nil {
				return fmt.Errorf("failed to parse plan: %w", err)
			}

			fmt.Printf("📋 Test Plan: %s\n\n", plan.Repository)

			stats := plan.Stats()
			fmt.Println("Summary:")
			fmt.Printf("   Total: %d tests\n", stats["total"])
			fmt.Printf("   API:   %d\n", stats["api"])
			fmt.Printf("   Unit:  %d\n", stats["unit"])
			fmt.Printf("   E2E:   %d\n", stats["e2e"])

			fmt.Println("\nAll intents:")
			for _, intent := range plan.Intents {
				fmt.Printf("   [%s][%s] %s\n", intent.Level, intent.Priority, intent.Reason)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&planFile, "file", "f", "", "Plan JSON file (required)")
	cmd.MarkFlagRequired("file")

	return cmd
}

func generateSpecsCmd() *cobra.Command {
	var (
		modelFile  string
		planFile   string
		outputFile string
		tier       string
		maxSpecs   int
	)

	cmd := &cobra.Command{
		Use:   "generate-specs",
		Short: "Generate test specifications from a plan using LLM",
		Long: `Takes a test plan and system model, then uses LLM to generate
detailed test specifications for each intent.

This requires an LLM (Ollama or Anthropic) to be available.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// Load system model
			modelData, err := os.ReadFile(modelFile)
			if err != nil {
				return fmt.Errorf("failed to read model: %w", err)
			}
			var sysModel model.SystemModel
			if err := json.Unmarshal(modelData, &sysModel); err != nil {
				return fmt.Errorf("failed to parse model: %w", err)
			}

			// Load plan
			planData, err := os.ReadFile(planFile)
			if err != nil {
				return fmt.Errorf("failed to read plan: %w", err)
			}
			var plan model.TestPlan
			if err := json.Unmarshal(planData, &plan); err != nil {
				return fmt.Errorf("failed to parse plan: %w", err)
			}

			fmt.Printf("📊 Model: %s\n", sysModel.Repository)
			fmt.Printf("📋 Plan: %d intents\n\n", len(plan.Intents))

			// Load config and create LLM router
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			router, err := llm.NewRouter(cfg)
			if err != nil {
				return fmt.Errorf("failed to create LLM router: %w", err)
			}

			if err := router.HealthCheck(); err != nil {
				return fmt.Errorf("LLM not available: %w\nMake sure Ollama is running", err)
			}

			// Parse tier
			tierNum, _ := strconv.Atoi(tier)
			llmTier := llm.Tier(tierNum)
			if llmTier < 1 || llmTier > 3 {
				llmTier = llm.Tier1
			}

			// Create spec generator
			gen := specgen.NewGenerator(router, llmTier)

			// Apply max limit
			intents := plan.Intents
			if maxSpecs > 0 && len(intents) > maxSpecs {
				intents = intents[:maxSpecs]
			}

			// Generate specs
			fmt.Printf("🔄 Generating %d test specifications...\n\n", len(intents))

			specSet := &model.TestSpecSet{
				ModelID:    plan.ModelID,
				Repository: plan.Repository,
				Specs:      make([]model.TestSpec, 0),
			}

			for i, intent := range intents {
				fmt.Printf("[%d/%d] %s %s...", i+1, len(intents), intent.Level, intent.Reason)

				spec, err := gen.GenerateSpec(ctx, intent, &sysModel)
				if err != nil {
					fmt.Printf(" ✗ %v\n", err)
					continue
				}

				specSet.Specs = append(specSet.Specs, *spec)
				fmt.Printf(" ✓\n")
			}

			// Print summary
			stats := specSet.Stats()
			fmt.Println()
			fmt.Println("📝 Generated Specifications")
			fmt.Println(strings.Repeat("─", 40))
			fmt.Printf("   Total: %d specs\n", stats["total"])
			fmt.Printf("   API:   %d\n", stats["api"])
			fmt.Printf("   Unit:  %d\n", stats["unit"])

			// Save to file
			if outputFile != "" {
				data, err := json.MarshalIndent(specSet, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal specs: %w", err)
				}
				if err := os.WriteFile(outputFile, data, 0644); err != nil {
					return fmt.Errorf("failed to write specs: %w", err)
				}
				fmt.Printf("\n💾 Specs saved to: %s\n", outputFile)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&modelFile, "model", "m", "", "System model JSON file (required)")
	cmd.Flags().StringVarP(&planFile, "plan", "p", "", "Test plan JSON file (required)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file for specs JSON")
	cmd.Flags().StringVarP(&tier, "tier", "t", "1", "LLM tier (1=fast, 2=balanced, 3=thorough)")
	cmd.Flags().IntVar(&maxSpecs, "max", 0, "Maximum number of specs to generate")
	cmd.MarkFlagRequired("model")
	cmd.MarkFlagRequired("plan")

	return cmd
}
