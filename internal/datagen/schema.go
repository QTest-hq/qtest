package datagen

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SchemaGenerator generates data from schemas (OpenAPI, JSON Schema, etc.)
type SchemaGenerator struct {
	gen *DataGenerator
}

// NewSchemaGenerator creates a schema-based generator
func NewSchemaGenerator() *SchemaGenerator {
	return &SchemaGenerator{
		gen: NewDataGenerator(),
	}
}

// Schema represents a simplified JSON schema
type Schema struct {
	Type       string             `json:"type"`
	Properties map[string]*Schema `json:"properties,omitempty"`
	Items      *Schema            `json:"items,omitempty"`
	Required   []string           `json:"required,omitempty"`
	Format     string             `json:"format,omitempty"`
	Enum       []interface{}      `json:"enum,omitempty"`
	Minimum    *float64           `json:"minimum,omitempty"`
	Maximum    *float64           `json:"maximum,omitempty"`
	MinLength  *int               `json:"minLength,omitempty"`
	MaxLength  *int               `json:"maxLength,omitempty"`
	MinItems   *int               `json:"minItems,omitempty"`
	MaxItems   *int               `json:"maxItems,omitempty"`
	Example    interface{}        `json:"example,omitempty"`
	Default    interface{}        `json:"default,omitempty"`
}

// GenerateFromSchema generates data from a JSON schema
func (sg *SchemaGenerator) GenerateFromSchema(schema *Schema, fieldName string) interface{} {
	// Use example if provided
	if schema.Example != nil {
		return schema.Example
	}

	// Use default if provided
	if schema.Default != nil {
		return schema.Default
	}

	// Use enum if provided
	if len(schema.Enum) > 0 {
		return schema.Enum[sg.gen.Int(0, len(schema.Enum)-1)]
	}

	switch schema.Type {
	case "object":
		return sg.generateObject(schema)
	case "array":
		return sg.generateArray(schema, fieldName)
	case "string":
		return sg.generateString(schema, fieldName)
	case "integer", "number":
		return sg.generateNumber(schema, fieldName)
	case "boolean":
		return sg.gen.Bool()
	default:
		return sg.gen.GenerateForType(schema.Type, fieldName)
	}
}

func (sg *SchemaGenerator) generateObject(schema *Schema) map[string]interface{} {
	result := make(map[string]interface{})

	for propName, propSchema := range schema.Properties {
		result[propName] = sg.GenerateFromSchema(propSchema, propName)
	}

	return result
}

func (sg *SchemaGenerator) generateArray(schema *Schema, fieldName string) []interface{} {
	minItems := 1
	maxItems := 3
	if schema.MinItems != nil {
		minItems = *schema.MinItems
	}
	if schema.MaxItems != nil {
		maxItems = *schema.MaxItems
	}

	count := sg.gen.Int(minItems, maxItems)
	result := make([]interface{}, count)

	for i := 0; i < count; i++ {
		if schema.Items != nil {
			result[i] = sg.GenerateFromSchema(schema.Items, fieldName)
		} else {
			result[i] = sg.gen.Word()
		}
	}

	return result
}

func (sg *SchemaGenerator) generateString(schema *Schema, fieldName string) string {
	// Check format first
	switch schema.Format {
	case "email":
		return sg.gen.Email()
	case "date":
		return sg.gen.Date()
	case "date-time":
		return sg.gen.DateTime()
	case "uri", "url":
		return sg.gen.URL()
	case "uuid":
		return sg.gen.UUID()
	case "phone":
		return sg.gen.Phone()
	case "password":
		return sg.gen.Password()
	}

	// Fall back to field name inference
	return sg.gen.GenerateForType("string", fieldName).(string)
}

func (sg *SchemaGenerator) generateNumber(schema *Schema, fieldName string) interface{} {
	min := 0.0
	max := 1000.0
	if schema.Minimum != nil {
		min = *schema.Minimum
	}
	if schema.Maximum != nil {
		max = *schema.Maximum
	}

	if schema.Type == "integer" {
		return sg.gen.Int(int(min), int(max))
	}
	return sg.gen.Float(min, max)
}

// GenerateFromJSON parses JSON schema and generates data
func (sg *SchemaGenerator) GenerateFromJSON(schemaJSON string) (interface{}, error) {
	var schema Schema
	if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
		return nil, fmt.Errorf("invalid schema: %w", err)
	}
	return sg.GenerateFromSchema(&schema, ""), nil
}

// InferSchemaFromSample infers a schema from a sample JSON object
func (sg *SchemaGenerator) InferSchemaFromSample(sample map[string]interface{}) *Schema {
	schema := &Schema{
		Type:       "object",
		Properties: make(map[string]*Schema),
	}

	for key, value := range sample {
		schema.Properties[key] = sg.inferType(key, value)
	}

	return schema
}

func (sg *SchemaGenerator) inferType(fieldName string, value interface{}) *Schema {
	switch v := value.(type) {
	case string:
		return sg.inferStringSchema(fieldName, v)
	case float64:
		if v == float64(int(v)) {
			return &Schema{Type: "integer"}
		}
		return &Schema{Type: "number"}
	case int:
		return &Schema{Type: "integer"}
	case bool:
		return &Schema{Type: "boolean"}
	case []interface{}:
		if len(v) > 0 {
			return &Schema{
				Type:  "array",
				Items: sg.inferType(fieldName, v[0]),
			}
		}
		return &Schema{Type: "array"}
	case map[string]interface{}:
		return sg.InferSchemaFromSample(v)
	default:
		return &Schema{Type: "string"}
	}
}

func (sg *SchemaGenerator) inferStringSchema(fieldName string, value string) *Schema {
	schema := &Schema{Type: "string"}
	fieldLower := strings.ToLower(fieldName)

	// Infer format from field name
	switch {
	case strings.Contains(fieldLower, "email"):
		schema.Format = "email"
	case strings.Contains(fieldLower, "date"):
		schema.Format = "date-time"
	case strings.Contains(fieldLower, "url") || strings.Contains(fieldLower, "link"):
		schema.Format = "uri"
	case strings.Contains(fieldLower, "id") && len(value) > 30:
		schema.Format = "uuid"
	case strings.Contains(fieldLower, "phone"):
		schema.Format = "phone"
	case strings.Contains(fieldLower, "password"):
		schema.Format = "password"
	}

	return schema
}

// TestDataSet generates a set of test data variations
type TestDataSet struct {
	Valid    []interface{} `json:"valid"`    // Valid data samples
	Invalid  []interface{} `json:"invalid"`  // Invalid data for negative tests
	Edge     []interface{} `json:"edge"`     // Edge cases
	Boundary []interface{} `json:"boundary"` // Boundary values
}

// GenerateTestDataSet generates comprehensive test data
func (sg *SchemaGenerator) GenerateTestDataSet(schema *Schema, fieldName string, count int) *TestDataSet {
	dataset := &TestDataSet{
		Valid:    make([]interface{}, count),
		Invalid:  make([]interface{}, 0),
		Edge:     make([]interface{}, 0),
		Boundary: make([]interface{}, 0),
	}

	// Generate valid samples
	for i := 0; i < count; i++ {
		dataset.Valid[i] = sg.GenerateFromSchema(schema, fieldName)
	}

	// Generate edge cases based on type
	switch schema.Type {
	case "string":
		dataset.Edge = append(dataset.Edge, "")         // Empty string
		dataset.Edge = append(dataset.Edge, " ")        // Whitespace
		dataset.Edge = append(dataset.Edge, "   ")      // Multiple spaces
		dataset.Invalid = append(dataset.Invalid, nil)  // Null
		dataset.Invalid = append(dataset.Invalid, 123)  // Wrong type

	case "integer", "number":
		dataset.Edge = append(dataset.Edge, 0)
		dataset.Edge = append(dataset.Edge, -1)
		dataset.Invalid = append(dataset.Invalid, "not a number")
		dataset.Invalid = append(dataset.Invalid, nil)
		if schema.Minimum != nil {
			dataset.Boundary = append(dataset.Boundary, *schema.Minimum)
			dataset.Boundary = append(dataset.Boundary, *schema.Minimum-1) // Below min
		}
		if schema.Maximum != nil {
			dataset.Boundary = append(dataset.Boundary, *schema.Maximum)
			dataset.Boundary = append(dataset.Boundary, *schema.Maximum+1) // Above max
		}

	case "array":
		dataset.Edge = append(dataset.Edge, []interface{}{}) // Empty array
		dataset.Invalid = append(dataset.Invalid, nil)
		dataset.Invalid = append(dataset.Invalid, "not an array")

	case "object":
		dataset.Edge = append(dataset.Edge, map[string]interface{}{}) // Empty object
		dataset.Invalid = append(dataset.Invalid, nil)
		dataset.Invalid = append(dataset.Invalid, []interface{}{}) // Array instead
	}

	return dataset
}
