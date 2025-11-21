package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/QTest-hq/qtest/internal/parser"
	"github.com/QTest-hq/qtest/internal/supplements"
	"github.com/QTest-hq/qtest/pkg/model"
	"github.com/spf13/cobra"
)

func modelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "model",
		Short: "Build and analyze Universal System Model",
	}

	cmd.AddCommand(modelBuildCmd())
	cmd.AddCommand(modelShowCmd())

	return cmd
}

func modelBuildCmd() *cobra.Command {
	var (
		dirPath    string
		outputFile string
		verbose    bool
	)

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build a system model from a directory",
		Long: `Scans a directory, parses all source files using Tree-sitter,
runs framework supplements (Express, FastAPI, Gin, etc.) to detect endpoints,
and outputs the Universal System Model.

This is the foundation for generating the complete test pyramid.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// Validate directory path
			validPath, err := validateDirPath(dirPath)
			if err != nil {
				return fmt.Errorf("invalid directory: %w", err)
			}

			fmt.Printf("🔍 Scanning directory: %s\n\n", validPath)

			// Create parser adapter
			repoName := filepath.Base(validPath)
			adapter := model.NewParserAdapter(repoName, "main", "")

			// Register all supplements
			registry := supplements.NewRegistry()
			for _, supp := range registry.GetAll() {
				adapter.RegisterSupplement(supp)
			}

			// Create tree-sitter parser
			p := parser.NewParser()

			// Walk directory and parse files
			fileCount := 0
			err = filepath.Walk(validPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}

				// Skip hidden and vendor directories
				if info.IsDir() {
					name := info.Name()
					if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "__pycache__" {
						return filepath.SkipDir
					}
					return nil
				}

				// Only parse supported source files
				ext := strings.ToLower(filepath.Ext(path))
				if !isSupportedSourceFile(ext) {
					return nil
				}

				// Parse file
				parsed, err := p.ParseFile(ctx, path)
				if err != nil {
					if verbose {
						fmt.Printf("  ⚠️  Skip %s: %v\n", path, err)
					}
					return nil
				}

				// Convert to model format
				pf := convertParsedFile(parsed)
				adapter.AddFile(pf)
				fileCount++

				if verbose {
					fmt.Printf("  ✓ %s (%d functions)\n", path, len(parsed.Functions))
				}

				return nil
			})

			if err != nil {
				return fmt.Errorf("failed to scan directory: %w", err)
			}

			fmt.Printf("📄 Parsed %d source files\n", fileCount)

			// Build model (runs supplements)
			fmt.Println("🔌 Running framework supplements...")
			sysModel, err := adapter.Build()
			if err != nil {
				return fmt.Errorf("failed to build model: %w", err)
			}

			// Print summary
			stats := sysModel.Stats()
			fmt.Println()
			fmt.Println("📊 System Model Summary")
			fmt.Println(strings.Repeat("─", 40))
			fmt.Printf("   Modules:      %d\n", stats["modules"])
			fmt.Printf("   Functions:    %d\n", stats["functions"])
			fmt.Printf("   Types:        %d\n", stats["types"])
			fmt.Printf("   Endpoints:    %d\n", stats["endpoints"])
			fmt.Printf("   Test Targets: %d\n", stats["test_targets"])
			fmt.Printf("   Languages:    %s\n", strings.Join(sysModel.Languages, ", "))

			// Show detected endpoints
			if len(sysModel.Endpoints) > 0 {
				fmt.Println()
				fmt.Println("🌐 Detected API Endpoints:")
				for _, ep := range sysModel.Endpoints {
					fmt.Printf("   %s %s → %s (%s)\n", ep.Method, ep.Path, ep.Handler, ep.Framework)
				}
			}

			// Show test targets
			if len(sysModel.TestTargets) > 0 {
				fmt.Println()
				fmt.Println("🎯 Top Test Targets:")
				maxShow := 10
				if len(sysModel.TestTargets) < maxShow {
					maxShow = len(sysModel.TestTargets)
				}
				for i := 0; i < maxShow; i++ {
					t := sysModel.TestTargets[i]
					fmt.Printf("   %d. [%s] %s (risk: %.2f)\n", t.Priority, t.Kind, t.Reason, t.RiskScore)
				}
				if len(sysModel.TestTargets) > maxShow {
					fmt.Printf("   ... and %d more\n", len(sysModel.TestTargets)-maxShow)
				}
			}

			// Output to file if requested
			if outputFile != "" {
				data, err := json.MarshalIndent(sysModel, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal model: %w", err)
				}
				if err := os.WriteFile(outputFile, data, 0644); err != nil {
					return fmt.Errorf("failed to write output: %w", err)
				}
				fmt.Printf("\n💾 Model saved to: %s\n", outputFile)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&dirPath, "dir", "d", ".", "Directory to scan")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file for JSON model")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show verbose output")

	return cmd
}

func modelShowCmd() *cobra.Command {
	var modelFile string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a saved system model",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(modelFile)
			if err != nil {
				return fmt.Errorf("failed to read model file: %w", err)
			}

			var sysModel model.SystemModel
			if err := json.Unmarshal(data, &sysModel); err != nil {
				return fmt.Errorf("failed to parse model: %w", err)
			}

			// Pretty print the model
			stats := sysModel.Stats()
			fmt.Printf("📊 System Model: %s\n", sysModel.Repository)
			fmt.Printf("   Branch: %s\n", sysModel.Branch)
			fmt.Printf("   Created: %s\n\n", sysModel.CreatedAt.Format("2006-01-02 15:04:05"))

			fmt.Println("Stats:")
			fmt.Printf("   Modules:   %d\n", stats["modules"])
			fmt.Printf("   Functions: %d\n", stats["functions"])
			fmt.Printf("   Types:     %d\n", stats["types"])
			fmt.Printf("   Endpoints: %d\n", stats["endpoints"])

			return nil
		},
	}

	cmd.Flags().StringVarP(&modelFile, "file", "f", "", "Model JSON file")
	cmd.MarkFlagRequired("file")

	return cmd
}

// isSupportedSourceFile checks if a file extension is supported
func isSupportedSourceFile(ext string) bool {
	switch ext {
	case ".go", ".py", ".js", ".jsx", ".ts", ".tsx", ".java":
		return true
	default:
		return false
	}
}

// convertParsedFile converts parser.ParsedFile to model.ParsedFile
func convertParsedFile(pf *parser.ParsedFile) *model.ParsedFile {
	result := &model.ParsedFile{
		Path:     pf.Path,
		Language: string(pf.Language),
	}

	// Convert functions
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

	// Convert classes
	for _, cls := range pf.Classes {
		methods := make([]model.ParserFunction, len(cls.Methods))
		for i, m := range cls.Methods {
			params := make([]model.ParserParameter, len(m.Parameters))
			for j, p := range m.Parameters {
				params[j] = model.ParserParameter{
					Name:     p.Name,
					Type:     p.Type,
					Default:  p.Default,
					Optional: p.Optional,
				}
			}
			methods[i] = model.ParserFunction{
				Name:       m.Name,
				StartLine:  m.StartLine,
				EndLine:    m.EndLine,
				Parameters: params,
				Exported:   m.Exported,
				Async:      m.Async,
				Body:       m.Body,
			}
		}

		props := make([]model.ParserProperty, len(cls.Properties))
		for i, p := range cls.Properties {
			props[i] = model.ParserProperty{
				Name:     p.Name,
				Type:     p.Type,
				Exported: p.Exported,
			}
		}

		result.Classes = append(result.Classes, model.ParserClass{
			ID:         cls.ID,
			Name:       cls.Name,
			StartLine:  cls.StartLine,
			EndLine:    cls.EndLine,
			Methods:    methods,
			Properties: props,
			Comments:   cls.Comments,
			Exported:   cls.Exported,
			Extends:    cls.Extends,
			Implements: cls.Implements,
		})
	}

	return result
}
