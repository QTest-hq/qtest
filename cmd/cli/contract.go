package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/QTest-hq/qtest/internal/contract"
	"github.com/QTest-hq/qtest/pkg/model"
	"github.com/spf13/cobra"
)

func contractCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contract",
		Short: "API contract testing",
		Long:  `Generate and validate API contracts from your codebase`,
	}

	cmd.AddCommand(contractGenerateCmd())
	cmd.AddCommand(contractTestsCmd())
	cmd.AddCommand(contractValidateCmd())

	return cmd
}

func contractGenerateCmd() *cobra.Command {
	var (
		modelFile  string
		outputFile string
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate API contract from system model",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load system model
			data, err := os.ReadFile(modelFile)
			if err != nil {
				return fmt.Errorf("failed to read model file: %w", err)
			}

			var sysModel model.SystemModel
			if err := json.Unmarshal(data, &sysModel); err != nil {
				return fmt.Errorf("failed to parse model: %w", err)
			}

			// Generate contract
			gen := contract.NewContractGenerator()
			apiContract := gen.GenerateFromModel(&sysModel)

			// Output
			contractJSON, err := json.MarshalIndent(apiContract, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal contract: %w", err)
			}

			if outputFile != "" {
				if err := os.WriteFile(outputFile, contractJSON, 0644); err != nil {
					return fmt.Errorf("failed to write contract: %w", err)
				}
				fmt.Printf("Contract written to: %s\n", outputFile)
				fmt.Printf("Endpoints: %d\n", len(apiContract.Endpoints))
			} else {
				fmt.Println(string(contractJSON))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&modelFile, "model", "m", "model.json", "System model file")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file (stdout if not specified)")

	return cmd
}

func contractTestsCmd() *cobra.Command {
	var (
		contractFile string
		outputDir    string
		language     string
	)

	cmd := &cobra.Command{
		Use:   "tests",
		Short: "Generate contract test code",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load contract
			data, err := os.ReadFile(contractFile)
			if err != nil {
				return fmt.Errorf("failed to read contract file: %w", err)
			}

			var apiContract contract.Contract
			if err := json.Unmarshal(data, &apiContract); err != nil {
				return fmt.Errorf("failed to parse contract: %w", err)
			}

			// Generate tests
			gen := contract.NewContractTestGenerator()
			testCode := gen.GenerateTests(&apiContract, language)

			// Determine output file
			var outputFile string
			switch language {
			case "javascript", "typescript":
				outputFile = filepath.Join(outputDir, "contract.test.js")
			case "python":
				outputFile = filepath.Join(outputDir, "test_contract.py")
			case "go":
				outputFile = filepath.Join(outputDir, "contract_test.go")
			}

			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return fmt.Errorf("failed to create output dir: %w", err)
			}

			if err := os.WriteFile(outputFile, []byte(testCode), 0644); err != nil {
				return fmt.Errorf("failed to write tests: %w", err)
			}

			fmt.Printf("Contract tests written to: %s\n", outputFile)
			fmt.Printf("Endpoints covered: %d\n", len(apiContract.Endpoints))

			return nil
		},
	}

	cmd.Flags().StringVarP(&contractFile, "contract", "c", "contract.json", "Contract file")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "./tests", "Output directory")
	cmd.Flags().StringVarP(&language, "language", "l", "javascript", "Target language (javascript, python, go)")

	return cmd
}

func contractValidateCmd() *cobra.Command {
	var (
		contractFile string
		baseURL      string
	)

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate API against contract (coming soon)",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Contract validation against live API coming soon!")
			fmt.Printf("Contract: %s\n", contractFile)
			fmt.Printf("Base URL: %s\n", baseURL)
			return nil
		},
	}

	cmd.Flags().StringVarP(&contractFile, "contract", "c", "contract.json", "Contract file")
	cmd.Flags().StringVarP(&baseURL, "url", "u", "http://localhost:3000", "Base URL for API")

	return cmd
}
