package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/QTest-hq/qtest/internal/datagen"
	"github.com/spf13/cobra"
)

func datagenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "datagen",
		Short: "Generate test data",
		Long:  `Generate realistic test data for your tests`,
	}

	cmd.AddCommand(datagenSampleCmd())
	cmd.AddCommand(datagenSchemaCmd())
	cmd.AddCommand(datagenFieldCmd())

	return cmd
}

func datagenSampleCmd() *cobra.Command {
	var count int

	cmd := &cobra.Command{
		Use:   "sample",
		Short: "Generate sample data for common types",
		RunE: func(cmd *cobra.Command, args []string) error {
			gen := datagen.NewDataGenerator()

			fmt.Println("Sample Test Data:")
			fmt.Println("=================")

			for i := 0; i < count; i++ {
				if count > 1 {
					fmt.Printf("\n--- Sample %d ---\n", i+1)
				}

				data := map[string]interface{}{
					"id":         gen.UUID(),
					"email":      gen.Email(),
					"firstName":  gen.FirstName(),
					"lastName":   gen.LastName(),
					"username":   gen.Username(),
					"phone":      gen.Phone(),
					"street":     gen.Street(),
					"city":       gen.City(),
					"state":      gen.State(),
					"country":    gen.Country(),
					"zipCode":    gen.ZipCode(),
					"company":    gen.Company(),
					"jobTitle":   gen.JobTitle(),
					"price":      gen.Price(),
					"createdAt":  gen.DateTime(),
					"isActive":   gen.Bool(),
					"age":        gen.Age(),
					"rating":     gen.Float(1, 5),
				}

				jsonData, _ := json.MarshalIndent(data, "", "  ")
				fmt.Println(string(jsonData))
			}

			return nil
		},
	}

	cmd.Flags().IntVarP(&count, "count", "n", 1, "Number of samples to generate")

	return cmd
}

func datagenSchemaCmd() *cobra.Command {
	var (
		schemaFile string
		count      int
		outputFile string
	)

	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Generate data from JSON schema",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Read schema
			data, err := os.ReadFile(schemaFile)
			if err != nil {
				return fmt.Errorf("failed to read schema: %w", err)
			}

			gen := datagen.NewSchemaGenerator()

			var schema datagen.Schema
			if err := json.Unmarshal(data, &schema); err != nil {
				return fmt.Errorf("failed to parse schema: %w", err)
			}

			var results []interface{}
			for i := 0; i < count; i++ {
				result := gen.GenerateFromSchema(&schema, "")
				results = append(results, result)
			}

			var output []byte
			if count == 1 {
				output, _ = json.MarshalIndent(results[0], "", "  ")
			} else {
				output, _ = json.MarshalIndent(results, "", "  ")
			}

			if outputFile != "" {
				if err := os.WriteFile(outputFile, output, 0644); err != nil {
					return fmt.Errorf("failed to write output: %w", err)
				}
				fmt.Printf("Generated %d sample(s) to: %s\n", count, outputFile)
			} else {
				fmt.Println(string(output))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&schemaFile, "schema", "s", "", "JSON schema file (required)")
	cmd.Flags().IntVarP(&count, "count", "n", 1, "Number of samples to generate")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file")
	cmd.MarkFlagRequired("schema")

	return cmd
}

func datagenFieldCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "field <field-name> [type]",
		Short: "Generate data for a specific field",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fieldName := args[0]
			typeName := "string"
			if len(args) > 1 {
				typeName = args[1]
			}

			gen := datagen.NewDataGenerator()
			value := gen.GenerateForType(typeName, fieldName)

			// Pretty print based on type
			switch v := value.(type) {
			case string:
				fmt.Printf("%s: \"%s\"\n", fieldName, v)
			default:
				jsonVal, _ := json.Marshal(v)
				fmt.Printf("%s: %s\n", fieldName, string(jsonVal))
			}

			return nil
		},
	}

	return cmd
}
