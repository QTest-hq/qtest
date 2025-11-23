package adapters

import (
	"strings"
	"testing"

	"github.com/QTest-hq/qtest/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJUnitSpecAdapter_Framework(t *testing.T) {
	adapter := NewJUnitSpecAdapter()
	assert.Equal(t, FrameworkJUnit, adapter.Framework())
}

func TestJUnitSpecAdapter_FileExtension(t *testing.T) {
	adapter := NewJUnitSpecAdapter()
	assert.Equal(t, ".java", adapter.FileExtension())
}

func TestJUnitSpecAdapter_TestFileSuffix(t *testing.T) {
	adapter := NewJUnitSpecAdapter()
	assert.Equal(t, "Test", adapter.TestFileSuffix())
}

func TestJUnitSpecAdapter_GenerateFromSpecs_Basic(t *testing.T) {
	adapter := NewJUnitSpecAdapter()

	specs := []model.TestSpec{
		{
			FunctionName: "add",
			Description:  "adds two positive numbers",
			Inputs: map[string]interface{}{
				"a": 1,
				"b": 2,
			},
			ArgOrder:   []string{"a", "b"},
			InputTypes: map[string]string{"a": "int", "b": "int"},
			Assertions: []model.Assertion{
				{Kind: "equals", Expected: 3},
			},
		},
	}

	code, err := adapter.GenerateFromSpecs(specs, "Calculator.java")
	require.NoError(t, err)

	// Check generated code contains expected elements
	assert.Contains(t, code, "class CalculatorTest")
	assert.Contains(t, code, "@Test")
	assert.Contains(t, code, "@DisplayName")
	assert.Contains(t, code, "assertEquals(3, result)")
	assert.Contains(t, code, "int a = 1;")
	assert.Contains(t, code, "int b = 2;")
}

func TestJUnitSpecAdapter_GenerateFromSpecs_MultipleTests(t *testing.T) {
	adapter := NewJUnitSpecAdapter()

	specs := []model.TestSpec{
		{
			FunctionName: "multiply",
			Description:  "multiplies positive numbers",
			Inputs:       map[string]interface{}{"x": 3, "y": 4},
			ArgOrder:     []string{"x", "y"},
			Assertions:   []model.Assertion{{Kind: "equals", Expected: 12}},
		},
		{
			FunctionName: "multiply",
			Description:  "multiplies with zero",
			Inputs:       map[string]interface{}{"x": 5, "y": 0},
			ArgOrder:     []string{"x", "y"},
			Assertions:   []model.Assertion{{Kind: "equals", Expected: 0}},
		},
	}

	code, err := adapter.GenerateFromSpecs(specs, "Math.java")
	require.NoError(t, err)

	// Should have two test methods
	assert.Equal(t, 2, strings.Count(code, "@Test"))
	assert.Contains(t, code, "multipliesPositiveNumbers")
	assert.Contains(t, code, "multipliesWithZero")
}

func TestJUnitSpecAdapter_GenerateFromSpecs_StringTypes(t *testing.T) {
	adapter := NewJUnitSpecAdapter()

	specs := []model.TestSpec{
		{
			FunctionName: "greet",
			Description:  "greets with name",
			Inputs:       map[string]interface{}{"name": "Alice"},
			InputTypes:   map[string]string{"name": "String"},
			ArgOrder:     []string{"name"},
			Assertions:   []model.Assertion{{Kind: "equals", Expected: "Hello, Alice!"}},
		},
	}

	code, err := adapter.GenerateFromSpecs(specs, "Greeter.java")
	require.NoError(t, err)

	assert.Contains(t, code, `String name = "Alice";`)
	assert.Contains(t, code, `assertEquals("Hello, Alice!", result)`)
}

func TestJUnitSpecAdapter_GenerateFromSpecs_BooleanTypes(t *testing.T) {
	adapter := NewJUnitSpecAdapter()

	specs := []model.TestSpec{
		{
			FunctionName: "isValid",
			Description:  "returns true for valid input",
			Inputs:       map[string]interface{}{"flag": true},
			InputTypes:   map[string]string{"flag": "boolean"},
			ArgOrder:     []string{"flag"},
			Assertions:   []model.Assertion{{Kind: "truthy"}},
		},
	}

	code, err := adapter.GenerateFromSpecs(specs, "Validator.java")
	require.NoError(t, err)

	assert.Contains(t, code, "boolean flag = true;")
	assert.Contains(t, code, "assertTrue(result)")
}

func TestJUnitSpecAdapter_GenerateFromSpecs_AssertionTypes(t *testing.T) {
	adapter := NewJUnitSpecAdapter()

	specs := []model.TestSpec{
		{
			FunctionName: "process",
			Description:  "test various assertions",
			Inputs:       map[string]interface{}{},
			Assertions: []model.Assertion{
				{Kind: "not_null"},
				{Kind: "not_equals", Expected: 0},
				{Kind: "greater_than", Expected: 5},
				{Kind: "less_than", Expected: 100},
			},
		},
	}

	code, err := adapter.GenerateFromSpecs(specs, "Processor.java")
	require.NoError(t, err)

	assert.Contains(t, code, "assertNotNull(result)")
	assert.Contains(t, code, "assertNotEquals(0, result)")
	assert.Contains(t, code, "assertTrue(result > 5)")
	assert.Contains(t, code, "assertTrue(result < 100)")
}

func TestJUnitSpecAdapter_GenerateFromSpecs_NoSpecs(t *testing.T) {
	adapter := NewJUnitSpecAdapter()

	_, err := adapter.GenerateFromSpecs([]model.TestSpec{}, "Test.java")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no test specs provided")
}

func TestToJavaMethodName(t *testing.T) {
	tests := []struct {
		description string
		index       int
		expected    string
	}{
		{"adds two numbers", 0, "addsTwoNumbers"},
		{"returns null for empty input", 1, "returnsNullForEmptyInput"},
		{"handles_special_chars", 2, "handlesSpecialChars"},
		{"", 0, "testCase1"},
		{"A", 0, "a"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			result := toJavaMethodName(tt.description, tt.index)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractJavaClassName(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"Calculator.java", "Calculator"},
		{"/path/to/MyClass.java", "MyClass"},
		{"src/main/java/com/example/Service.java", "Service"},
		{"", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := extractJavaClassName(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractJavaPackageName(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"src/main/java/com/example/Calculator.java", "com.example"},
		{"src/test/java/com/example/tests/MyTest.java", "com.example.tests"},
		{"Calculator.java", "tests"},
		{"", "tests"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := extractJavaPackageName(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatJavaValue(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		typeHint string
		expected string
	}{
		{"int", 42, "int", "42"},
		{"string", "hello", "String", "\"hello\""},
		{"bool true", true, "boolean", "true"},
		{"bool false", false, "boolean", "false"},
		{"nil", nil, "", "null"},
		{"float", 3.14, "double", "3.14"},
		{"string with quotes", `say "hi"`, "String", `"say \"hi\""`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatJavaValue(tt.value, tt.typeHint)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInferJavaType(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{"int", 42, "int"},
		{"string", "hello", "String"},
		{"bool", true, "boolean"},
		{"float", 3.14, "double"},
		{"nil", nil, "Object"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferJavaType(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}
