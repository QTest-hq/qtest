package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/QTest-hq/qtest/internal/emitter"
	"github.com/QTest-hq/qtest/pkg/model"
	"github.com/spf13/cobra"
)

func emitTestsCmd() *cobra.Command {
	var (
		specsFile   string
		outputDir   string
		emitterName string
		language    string
	)

	cmd := &cobra.Command{
		Use:   "emit-tests",
		Short: "Generate test code from test specifications",
		Long: `Converts test specifications (JSON) to runnable test code.

Supported emitters:
  - supertest: Jest + Supertest for Express/Node.js
  - pytest: pytest + httpx for FastAPI/Python
  - go-http: Go net/http testing

Example:
  qtest emit-tests -s specs.json -o ./tests --emitter supertest`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load specs
			data, err := os.ReadFile(specsFile)
			if err != nil {
				return fmt.Errorf("failed to read specs: %w", err)
			}

			var specSet model.TestSpecSet
			if err := json.Unmarshal(data, &specSet); err != nil {
				return fmt.Errorf("failed to parse specs: %w", err)
			}

			fmt.Printf("📝 Loaded %d test specifications\n", len(specSet.Specs))

			// Get emitter
			registry := emitter.NewRegistry()

			var em emitter.Emitter
			if emitterName != "" {
				em, err = registry.Get(emitterName)
				if err != nil {
					return fmt.Errorf("emitter not found: %s\nAvailable: %v", emitterName, registry.List())
				}
			} else if language != "" {
				em, err = registry.GetForLanguage(language)
				if err != nil {
					return fmt.Errorf("no emitter for language: %s", language)
				}
			} else {
				// Auto-detect from specs
				em, _ = registry.Get("supertest") // Default
			}

			fmt.Printf("🔧 Using emitter: %s (%s)\n\n", em.Name(), em.Framework())

			// Group specs by level
			apiSpecs := specSet.FilterByLevel(model.LevelAPI)
			unitSpecs := specSet.FilterByLevel(model.LevelUnit)

			// Create output directory
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}

			filesWritten := 0

			// Emit API tests
			if len(apiSpecs) > 0 {
				code, err := em.Emit(apiSpecs)
				if err != nil {
					return fmt.Errorf("failed to emit API tests: %w", err)
				}

				filename := "api" + em.FileExtension()
				filepath := filepath.Join(outputDir, filename)

				if err := os.WriteFile(filepath, []byte(code), 0644); err != nil {
					return fmt.Errorf("failed to write %s: %w", filepath, err)
				}

				fmt.Printf("✅ Written: %s (%d API tests)\n", filepath, len(apiSpecs))
				filesWritten++
			}

			// Emit unit tests (if we have a unit emitter - for now just note them)
			if len(unitSpecs) > 0 {
				fmt.Printf("ℹ️  Skipped %d unit tests (unit emitter not yet implemented)\n", len(unitSpecs))
			}

			// Summary
			fmt.Println()
			fmt.Println(strings.Repeat("─", 40))
			fmt.Printf("📦 Generated %d test file(s) in %s\n", filesWritten, outputDir)
			fmt.Printf("   Framework: %s\n", em.Framework())

			return nil
		},
	}

	cmd.Flags().StringVarP(&specsFile, "specs", "s", "", "Test specifications JSON file (required)")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "./tests", "Output directory for test files")
	cmd.Flags().StringVarP(&emitterName, "emitter", "e", "", "Emitter name (supertest, pytest, go-http)")
	cmd.Flags().StringVarP(&language, "language", "l", "", "Target language (javascript, python, go)")
	cmd.MarkFlagRequired("specs")

	return cmd
}
