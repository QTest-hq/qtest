package datagen

import (
	"encoding/json"
	"testing"
)

func TestNewSchemaGenerator(t *testing.T) {
	sg := NewSchemaGenerator()

	if sg == nil {
		t.Fatal("NewSchemaGenerator() returned nil")
	}
	if sg.gen == nil {
		t.Error("gen should not be nil")
	}
}

func TestSchema_Fields(t *testing.T) {
	minLen := 5
	maxLen := 100
	minItems := 1
	maxItems := 10
	min := 0.0
	max := 100.0

	schema := Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"name": {Type: "string"},
			"age":  {Type: "integer"},
		},
		Items:     &Schema{Type: "string"},
		Required:  []string{"name"},
		Format:    "email",
		Enum:      []interface{}{"a", "b"},
		Minimum:   &min,
		Maximum:   &max,
		MinLength: &minLen,
		MaxLength: &maxLen,
		MinItems:  &minItems,
		MaxItems:  &maxItems,
		Example:   "example",
		Default:   "default",
	}

	if schema.Type != "object" {
		t.Errorf("Type = %s, want object", schema.Type)
	}
	if len(schema.Properties) != 2 {
		t.Errorf("len(Properties) = %d, want 2", len(schema.Properties))
	}
	if schema.Items == nil {
		t.Error("Items should not be nil")
	}
	if len(schema.Required) != 1 {
		t.Errorf("len(Required) = %d, want 1", len(schema.Required))
	}
	if schema.Format != "email" {
		t.Errorf("Format = %s, want email", schema.Format)
	}
	if len(schema.Enum) != 2 {
		t.Errorf("len(Enum) = %d, want 2", len(schema.Enum))
	}
	if *schema.MinLength != 5 {
		t.Errorf("MinLength = %d, want 5", *schema.MinLength)
	}
}

func TestGenerateFromSchema_Example(t *testing.T) {
	sg := NewSchemaGenerator()

	schema := &Schema{
		Type:    "string",
		Example: "test-example",
	}

	result := sg.GenerateFromSchema(schema, "test")

	if result != "test-example" {
		t.Errorf("Should use Example value, got %v", result)
	}
}

func TestGenerateFromSchema_Default(t *testing.T) {
	sg := NewSchemaGenerator()

	schema := &Schema{
		Type:    "string",
		Default: "default-value",
	}

	result := sg.GenerateFromSchema(schema, "test")

	if result != "default-value" {
		t.Errorf("Should use Default value, got %v", result)
	}
}

func TestGenerateFromSchema_Enum(t *testing.T) {
	sg := NewSchemaGenerator()

	schema := &Schema{
		Type: "string",
		Enum: []interface{}{"active", "inactive"},
	}

	result := sg.GenerateFromSchema(schema, "status")

	valid := false
	for _, e := range schema.Enum {
		if result == e {
			valid = true
			break
		}
	}
	if !valid {
		t.Errorf("Should return value from Enum, got %v", result)
	}
}

func TestGenerateFromSchema_Object(t *testing.T) {
	sg := NewSchemaGenerator()

	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"name":  {Type: "string"},
			"email": {Type: "string", Format: "email"},
		},
	}

	result := sg.GenerateFromSchema(schema, "user")

	obj, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Should return map for object type")
	}
	if _, exists := obj["name"]; !exists {
		t.Error("Object should have 'name' property")
	}
	if _, exists := obj["email"]; !exists {
		t.Error("Object should have 'email' property")
	}
}

func TestGenerateFromSchema_Array(t *testing.T) {
	sg := NewSchemaGenerator()

	schema := &Schema{
		Type:  "array",
		Items: &Schema{Type: "string"},
	}

	result := sg.GenerateFromSchema(schema, "items")

	arr, ok := result.([]interface{})
	if !ok {
		t.Fatal("Should return slice for array type")
	}
	if len(arr) < 1 {
		t.Error("Array should have at least 1 item")
	}
}

func TestGenerateFromSchema_Array_MinMaxItems(t *testing.T) {
	sg := NewSchemaGenerator()

	min := 5
	max := 5
	schema := &Schema{
		Type:     "array",
		Items:    &Schema{Type: "string"},
		MinItems: &min,
		MaxItems: &max,
	}

	result := sg.GenerateFromSchema(schema, "items")

	arr, ok := result.([]interface{})
	if !ok {
		t.Fatal("Should return slice")
	}
	if len(arr) != 5 {
		t.Errorf("len(arr) = %d, want 5", len(arr))
	}
}

func TestGenerateFromSchema_Array_NoItems(t *testing.T) {
	sg := NewSchemaGenerator()

	schema := &Schema{
		Type: "array",
	}

	result := sg.GenerateFromSchema(schema, "items")

	arr, ok := result.([]interface{})
	if !ok {
		t.Fatal("Should return slice")
	}
	// Should generate default word items when no Items schema
	if len(arr) < 1 {
		t.Error("Array should have items")
	}
}

func TestGenerateFromSchema_String(t *testing.T) {
	sg := NewSchemaGenerator()

	schema := &Schema{Type: "string"}

	result := sg.GenerateFromSchema(schema, "test")

	_, ok := result.(string)
	if !ok {
		t.Error("Should return string")
	}
}

func TestGenerateFromSchema_String_Formats(t *testing.T) {
	sg := NewSchemaGenerator()

	tests := []struct {
		format   string
		contains string
	}{
		{"email", "@"},
		{"date", "-"},
		{"date-time", "T"},
		{"uri", "https://"},
		{"url", "https://"},
		{"uuid", "-"},
		{"phone", "+"},
		{"password", ""},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			schema := &Schema{Type: "string", Format: tt.format}
			result := sg.GenerateFromSchema(schema, "test")

			str, ok := result.(string)
			if !ok {
				t.Fatal("Should return string")
			}
			if tt.contains != "" && !containsStr(str, tt.contains) {
				t.Errorf("Format %s result %s should contain %s", tt.format, str, tt.contains)
			}
		})
	}
}

func TestGenerateFromSchema_Integer(t *testing.T) {
	sg := NewSchemaGenerator()

	schema := &Schema{Type: "integer"}

	result := sg.GenerateFromSchema(schema, "count")

	_, ok := result.(int)
	if !ok {
		t.Errorf("Should return int, got %T", result)
	}
}

func TestGenerateFromSchema_Number(t *testing.T) {
	sg := NewSchemaGenerator()

	schema := &Schema{Type: "number"}

	result := sg.GenerateFromSchema(schema, "price")

	_, ok := result.(float64)
	if !ok {
		t.Errorf("Should return float64, got %T", result)
	}
}

func TestGenerateFromSchema_Number_MinMax(t *testing.T) {
	sg := NewSchemaGenerator()

	min := 10.0
	max := 20.0
	schema := &Schema{
		Type:    "number",
		Minimum: &min,
		Maximum: &max,
	}

	for i := 0; i < 100; i++ {
		result := sg.GenerateFromSchema(schema, "value")
		val := result.(float64)
		if val < 10 || val > 20 {
			t.Errorf("Value %f out of range [10, 20]", val)
		}
	}
}

func TestGenerateFromSchema_Boolean(t *testing.T) {
	sg := NewSchemaGenerator()

	schema := &Schema{Type: "boolean"}

	result := sg.GenerateFromSchema(schema, "active")

	_, ok := result.(bool)
	if !ok {
		t.Error("Should return bool")
	}
}

func TestGenerateFromJSON_Valid(t *testing.T) {
	sg := NewSchemaGenerator()

	schemaJSON := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer"}
		}
	}`

	result, err := sg.GenerateFromJSON(schemaJSON)
	if err != nil {
		t.Fatalf("GenerateFromJSON() error = %v", err)
	}

	obj, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Should return object")
	}
	if _, exists := obj["name"]; !exists {
		t.Error("Should have name property")
	}
	if _, exists := obj["age"]; !exists {
		t.Error("Should have age property")
	}
}

func TestGenerateFromJSON_Invalid(t *testing.T) {
	sg := NewSchemaGenerator()

	_, err := sg.GenerateFromJSON("invalid json")
	if err == nil {
		t.Error("Should return error for invalid JSON")
	}
}

func TestInferSchemaFromSample(t *testing.T) {
	sg := NewSchemaGenerator()

	sample := map[string]interface{}{
		"name":   "John",
		"age":    30.0,
		"active": true,
		"tags":   []interface{}{"a", "b"},
		"address": map[string]interface{}{
			"city": "NYC",
		},
	}

	schema := sg.InferSchemaFromSample(sample)

	if schema.Type != "object" {
		t.Errorf("Type = %s, want object", schema.Type)
	}
	if schema.Properties["name"].Type != "string" {
		t.Error("name should be string type")
	}
	if schema.Properties["age"].Type != "integer" {
		t.Errorf("age should be integer type, got %s", schema.Properties["age"].Type)
	}
	if schema.Properties["active"].Type != "boolean" {
		t.Error("active should be boolean type")
	}
	if schema.Properties["tags"].Type != "array" {
		t.Error("tags should be array type")
	}
	if schema.Properties["address"].Type != "object" {
		t.Error("address should be object type")
	}
}

func TestInferType_String(t *testing.T) {
	sg := NewSchemaGenerator()

	schema := sg.inferType("test", "hello")

	if schema.Type != "string" {
		t.Errorf("Type = %s, want string", schema.Type)
	}
}

func TestInferType_Integer(t *testing.T) {
	sg := NewSchemaGenerator()

	schema := sg.inferType("count", 42)

	if schema.Type != "integer" {
		t.Errorf("Type = %s, want integer", schema.Type)
	}
}

func TestInferType_Float(t *testing.T) {
	sg := NewSchemaGenerator()

	schema := sg.inferType("price", 42.5)

	if schema.Type != "number" {
		t.Errorf("Type = %s, want number", schema.Type)
	}
}

func TestInferType_Boolean(t *testing.T) {
	sg := NewSchemaGenerator()

	schema := sg.inferType("active", true)

	if schema.Type != "boolean" {
		t.Errorf("Type = %s, want boolean", schema.Type)
	}
}

func TestInferType_Array(t *testing.T) {
	sg := NewSchemaGenerator()

	schema := sg.inferType("items", []interface{}{"a", "b"})

	if schema.Type != "array" {
		t.Errorf("Type = %s, want array", schema.Type)
	}
	if schema.Items.Type != "string" {
		t.Error("Items type should be string")
	}
}

func TestInferType_EmptyArray(t *testing.T) {
	sg := NewSchemaGenerator()

	schema := sg.inferType("items", []interface{}{})

	if schema.Type != "array" {
		t.Errorf("Type = %s, want array", schema.Type)
	}
}

func TestInferType_Object(t *testing.T) {
	sg := NewSchemaGenerator()

	schema := sg.inferType("user", map[string]interface{}{"name": "John"})

	if schema.Type != "object" {
		t.Errorf("Type = %s, want object", schema.Type)
	}
}

func TestInferType_Unknown(t *testing.T) {
	sg := NewSchemaGenerator()

	// nil or unknown types default to string
	schema := sg.inferType("test", nil)

	if schema.Type != "string" {
		t.Errorf("Type = %s, want string for nil", schema.Type)
	}
}

func TestInferStringSchema_Email(t *testing.T) {
	sg := NewSchemaGenerator()

	schema := sg.inferStringSchema("userEmail", "test@example.com")

	if schema.Format != "email" {
		t.Errorf("Format = %s, want email", schema.Format)
	}
}

func TestInferStringSchema_Date(t *testing.T) {
	sg := NewSchemaGenerator()

	schema := sg.inferStringSchema("createdDate", "2024-01-01")

	if schema.Format != "date-time" {
		t.Errorf("Format = %s, want date-time", schema.Format)
	}
}

func TestInferStringSchema_URL(t *testing.T) {
	sg := NewSchemaGenerator()

	schema := sg.inferStringSchema("profileUrl", "https://example.com")

	if schema.Format != "uri" {
		t.Errorf("Format = %s, want uri", schema.Format)
	}
}

func TestInferStringSchema_Phone(t *testing.T) {
	sg := NewSchemaGenerator()

	schema := sg.inferStringSchema("phone", "+1-555-1234")

	if schema.Format != "phone" {
		t.Errorf("Format = %s, want phone", schema.Format)
	}
}

func TestInferStringSchema_Password(t *testing.T) {
	sg := NewSchemaGenerator()

	schema := sg.inferStringSchema("password", "secret123")

	if schema.Format != "password" {
		t.Errorf("Format = %s, want password", schema.Format)
	}
}

func TestTestDataSet_Fields(t *testing.T) {
	ds := TestDataSet{
		Valid:    []interface{}{"a", "b"},
		Invalid:  []interface{}{nil, 123},
		Edge:     []interface{}{"", " "},
		Boundary: []interface{}{0, 100},
	}

	if len(ds.Valid) != 2 {
		t.Errorf("len(Valid) = %d, want 2", len(ds.Valid))
	}
	if len(ds.Invalid) != 2 {
		t.Errorf("len(Invalid) = %d, want 2", len(ds.Invalid))
	}
	if len(ds.Edge) != 2 {
		t.Errorf("len(Edge) = %d, want 2", len(ds.Edge))
	}
	if len(ds.Boundary) != 2 {
		t.Errorf("len(Boundary) = %d, want 2", len(ds.Boundary))
	}
}

func TestGenerateTestDataSet_String(t *testing.T) {
	sg := NewSchemaGenerator()

	schema := &Schema{Type: "string"}
	ds := sg.GenerateTestDataSet(schema, "name", 3)

	if len(ds.Valid) != 3 {
		t.Errorf("len(Valid) = %d, want 3", len(ds.Valid))
	}
	if len(ds.Edge) < 3 {
		t.Error("Should have edge cases for string")
	}
	if len(ds.Invalid) < 2 {
		t.Error("Should have invalid cases for string")
	}
}

func TestGenerateTestDataSet_Integer(t *testing.T) {
	sg := NewSchemaGenerator()

	min := 10.0
	max := 100.0
	schema := &Schema{
		Type:    "integer",
		Minimum: &min,
		Maximum: &max,
	}
	ds := sg.GenerateTestDataSet(schema, "count", 5)

	if len(ds.Valid) != 5 {
		t.Errorf("len(Valid) = %d, want 5", len(ds.Valid))
	}
	// Should have boundary values
	if len(ds.Boundary) < 4 {
		t.Errorf("Should have boundary values, got %d", len(ds.Boundary))
	}
}

func TestGenerateTestDataSet_Array(t *testing.T) {
	sg := NewSchemaGenerator()

	schema := &Schema{
		Type:  "array",
		Items: &Schema{Type: "string"},
	}
	ds := sg.GenerateTestDataSet(schema, "items", 2)

	if len(ds.Valid) != 2 {
		t.Errorf("len(Valid) = %d, want 2", len(ds.Valid))
	}
	// Edge should include empty array
	hasEmpty := false
	for _, e := range ds.Edge {
		if arr, ok := e.([]interface{}); ok && len(arr) == 0 {
			hasEmpty = true
			break
		}
	}
	if !hasEmpty {
		t.Error("Edge cases should include empty array")
	}
}

func TestGenerateTestDataSet_Object(t *testing.T) {
	sg := NewSchemaGenerator()

	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"name": {Type: "string"},
		},
	}
	ds := sg.GenerateTestDataSet(schema, "user", 2)

	if len(ds.Valid) != 2 {
		t.Errorf("len(Valid) = %d, want 2", len(ds.Valid))
	}
	// Edge should include empty object
	hasEmpty := false
	for _, e := range ds.Edge {
		if obj, ok := e.(map[string]interface{}); ok && len(obj) == 0 {
			hasEmpty = true
			break
		}
	}
	if !hasEmpty {
		t.Error("Edge cases should include empty object")
	}
}

func TestSchemaJSONMarshal(t *testing.T) {
	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"name": {Type: "string"},
		},
		Required: []string{"name"},
	}

	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var unmarshaled Schema
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if unmarshaled.Type != "object" {
		t.Errorf("Type = %s, want object", unmarshaled.Type)
	}
}

// Helper
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (substr == "" || s != "" && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
