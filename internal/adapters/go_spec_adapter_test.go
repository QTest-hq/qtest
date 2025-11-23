package adapters

import (
	"strings"
	"testing"

	"github.com/QTest-hq/qtest/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGoSpecAdapter(t *testing.T) {
	adapter := NewGoSpecAdapter()
	assert.NotNil(t, adapter)
}

func TestGoSpecAdapter_Framework(t *testing.T) {
	adapter := NewGoSpecAdapter()
	assert.Equal(t, FrameworkGoTest, adapter.Framework())
}

func TestGoSpecAdapter_FileExtension(t *testing.T) {
	adapter := NewGoSpecAdapter()
	assert.Equal(t, ".go", adapter.FileExtension())
}

func TestGoSpecAdapter_TestFileSuffix(t *testing.T) {
	adapter := NewGoSpecAdapter()
	assert.Equal(t, "_test", adapter.TestFileSuffix())
}

func TestGoSpecAdapter_GenerateFromSpecs_Empty(t *testing.T) {
	adapter := NewGoSpecAdapter()
	_, err := adapter.GenerateFromSpecs([]model.TestSpec{}, "test.go")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no test specs provided")
}

func TestGoSpecAdapter_GenerateFromSpecs_Simple(t *testing.T) {
	adapter := NewGoSpecAdapter()
	specs := []model.TestSpec{
		{
			FunctionName: "Add",
			Description:  "adds two numbers",
			Inputs:       map[string]interface{}{"a": 1, "b": 2},
			ArgOrder:     []string{"a", "b"},
			Assertions: []model.Assertion{
				{Kind: "equals", Expected: 3},
			},
		},
	}

	code, err := adapter.GenerateFromSpecs(specs, "math.go")
	require.NoError(t, err)

	assert.Contains(t, code, "package")
	assert.Contains(t, code, "func TestAdd(t *testing.T)")
	assert.Contains(t, code, "t.Run")
	assert.Contains(t, code, "result := Add(a, b)")
}

func TestGoSpecAdapter_GenerateFromSpecs_MultipleSpecs(t *testing.T) {
	adapter := NewGoSpecAdapter()
	specs := []model.TestSpec{
		{
			FunctionName: "Add",
			Description:  "positive numbers",
			Inputs:       map[string]interface{}{"a": 1, "b": 2},
			ArgOrder:     []string{"a", "b"},
			Assertions:   []model.Assertion{{Kind: "equals", Expected: 3}},
		},
		{
			FunctionName: "Add",
			Description:  "negative numbers",
			Inputs:       map[string]interface{}{"a": -1, "b": -2},
			ArgOrder:     []string{"a", "b"},
			Assertions:   []model.Assertion{{Kind: "equals", Expected: -3}},
		},
	}

	code, err := adapter.GenerateFromSpecs(specs, "math.go")
	require.NoError(t, err)

	assert.Contains(t, code, "positive_numbers")
	assert.Contains(t, code, "negative_numbers")
}

func TestGoSpecAdapter_GenerateAssertion_Equality(t *testing.T) {
	adapter := NewGoSpecAdapter()
	code, usesStrings, usesReflect := adapter.generateAssertion(model.Assertion{
		Kind:     "equals",
		Expected: 42,
	})

	assert.Contains(t, code, "result != 42")
	assert.False(t, usesStrings)
	assert.False(t, usesReflect)
}

func TestGoSpecAdapter_GenerateAssertion_NotEqual(t *testing.T) {
	adapter := NewGoSpecAdapter()
	code, _, _ := adapter.generateAssertion(model.Assertion{
		Kind:     "not_equal",
		Expected: 0,
	})

	assert.Contains(t, code, "result == 0")
	assert.Contains(t, code, "expected not")
}

func TestGoSpecAdapter_GenerateAssertion_NotNil(t *testing.T) {
	adapter := NewGoSpecAdapter()
	code, _, _ := adapter.generateAssertion(model.Assertion{
		Kind: "not_nil",
	})

	assert.Contains(t, code, "result == nil")
	assert.Contains(t, code, "expected non-nil")
}

func TestGoSpecAdapter_GenerateAssertion_IsNil(t *testing.T) {
	adapter := NewGoSpecAdapter()
	code, _, _ := adapter.generateAssertion(model.Assertion{
		Kind: "is_nil",
	})

	assert.Contains(t, code, "result != nil")
	assert.Contains(t, code, "expected nil")
}

func TestGoSpecAdapter_GenerateAssertion_Contains(t *testing.T) {
	adapter := NewGoSpecAdapter()
	code, usesStrings, _ := adapter.generateAssertion(model.Assertion{
		Kind:     "contains",
		Expected: "hello",
	})

	assert.Contains(t, code, "strings.Contains")
	assert.True(t, usesStrings)
}

func TestGoSpecAdapter_GenerateAssertion_GreaterThan(t *testing.T) {
	adapter := NewGoSpecAdapter()
	code, _, _ := adapter.generateAssertion(model.Assertion{
		Kind:     "greater_than",
		Expected: 10,
	})

	assert.Contains(t, code, "result <= 10")
}

func TestGoSpecAdapter_GenerateAssertion_LessThan(t *testing.T) {
	adapter := NewGoSpecAdapter()
	code, _, _ := adapter.generateAssertion(model.Assertion{
		Kind:     "less_than",
		Expected: 100,
	})

	assert.Contains(t, code, "result >= 100")
}

func TestGoSpecAdapter_GenerateAssertion_Truthy(t *testing.T) {
	adapter := NewGoSpecAdapter()
	code, _, _ := adapter.generateAssertion(model.Assertion{
		Kind: "truthy",
	})

	assert.Contains(t, code, "!result")
	assert.Contains(t, code, "truthy value")
}

func TestGoSpecAdapter_GenerateAssertion_Falsy(t *testing.T) {
	adapter := NewGoSpecAdapter()
	code, _, _ := adapter.generateAssertion(model.Assertion{
		Kind: "falsy",
	})

	assert.Contains(t, code, "if result")
	assert.Contains(t, code, "falsy value")
}

func TestGoSpecAdapter_GenerateAssertion_Error(t *testing.T) {
	adapter := NewGoSpecAdapter()
	code, _, _ := adapter.generateAssertion(model.Assertion{
		Kind: "error",
	})

	assert.Contains(t, code, "err == nil")
	assert.Contains(t, code, "expected error")
}

func TestGoSpecAdapter_GenerateAssertion_TypeIs(t *testing.T) {
	adapter := NewGoSpecAdapter()
	code, _, usesReflect := adapter.generateAssertion(model.Assertion{
		Kind:     "type_is",
		Expected: "int",
	})

	assert.Contains(t, code, "reflect.TypeOf")
	assert.True(t, usesReflect)
}

func TestGoSpecAdapter_GenerateAssertion_Unknown(t *testing.T) {
	adapter := NewGoSpecAdapter()
	code, _, _ := adapter.generateAssertion(model.Assertion{
		Kind:     "unknown_kind",
		Expected: 5,
	})

	// Should default to equality check
	assert.Contains(t, code, "result != 5")
}

func TestGoSpecAdapter_GenerateAssertion_UnknownNoExpected(t *testing.T) {
	adapter := NewGoSpecAdapter()
	code, _, _ := adapter.generateAssertion(model.Assertion{
		Kind: "unknown_kind",
	})

	assert.Empty(t, code)
}

func TestFormatGoValue_String(t *testing.T) {
	result := formatGoValue("hello")
	assert.Equal(t, `"hello"`, result)
}

func TestFormatGoValue_Int(t *testing.T) {
	result := formatGoValue(42)
	assert.Equal(t, "42", result)
}

func TestFormatGoValue_Float(t *testing.T) {
	result := formatGoValue(3.14)
	assert.Equal(t, "3.14", result)
}

func TestFormatGoValue_Bool(t *testing.T) {
	result := formatGoValue(true)
	assert.Equal(t, "true", result)
}

func TestFormatGoValue_Nil(t *testing.T) {
	result := formatGoValue(nil)
	assert.Equal(t, "nil", result)
}

func TestFormatGoValue_Array(t *testing.T) {
	result := formatGoValue([]interface{}{1, 2, 3})
	assert.Contains(t, result, "[]interface{}")
	assert.Contains(t, result, "1")
	assert.Contains(t, result, "2")
	assert.Contains(t, result, "3")
}

func TestFormatGoValue_Map(t *testing.T) {
	result := formatGoValue(map[string]interface{}{"key": "value"})
	assert.Contains(t, result, "map[string]interface{}")
	assert.Contains(t, result, "key")
}

func TestFormatGoValue_VariableReference(t *testing.T) {
	result := formatGoValue("${myVar}")
	assert.Equal(t, "myVar", result)

	result2 := formatGoValue("$count")
	assert.Equal(t, "count", result2)
}

func TestFormatGoValue_StringNumber(t *testing.T) {
	result := formatGoValue("42")
	assert.Equal(t, "42", result)

	result2 := formatGoValue("3.14")
	assert.Equal(t, "3.14", result2)
}

func TestFormatGoValue_BoolString(t *testing.T) {
	assert.Equal(t, "true", formatGoValue("true"))
	assert.Equal(t, "false", formatGoValue("false"))
}

func TestSanitizeTestName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple test", "simple_test"},
		{"test-with-dashes", "test_with_dashes"},
		{"test.with.dots", "test_with_dots"},
		{"test's quotes", "tests_quotes"},
		{"test (parens)", "test_parens"},
		{"test, comma", "test__comma"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeTestName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEscapeStringForErrorMsg(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{`has "quotes"`, `has \"quotes\"`},
		{`has \backslash`, `has \\backslash`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeStringForErrorMsg(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatGoValueWithType(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		typeHint string
		expected string
	}{
		{"int from float64", float64(42), "int", "42"},
		{"int from int", 42, "int", "42"},
		{"float", 3.14, "float", "3.14"},
		{"string", "hello", "string", `"hello"`},
		{"bool", true, "bool", "true"},
		{"null", nil, "null", "nil"},
		{"array", []interface{}{1, 2}, "array", "[]interface{}{1, 2}"},
		{"object", map[string]interface{}{"a": 1}, "object", `map[string]interface{}{"a": 1}`},
		{"no type hint", "test", "", `"test"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatGoValueWithType(tt.value, tt.typeHint)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGoSpecAdapter_GenerateSetup(t *testing.T) {
	adapter := NewGoSpecAdapter()
	spec := model.TestSpec{
		Inputs:   map[string]interface{}{"x": 10, "y": 20},
		ArgOrder: []string{"x", "y"},
	}

	setup := adapter.generateSetup(spec)
	assert.Contains(t, setup, "x :=")
	assert.Contains(t, setup, "y :=")
}

func TestGoSpecAdapter_GenerateAction(t *testing.T) {
	adapter := NewGoSpecAdapter()
	spec := model.TestSpec{
		FunctionName: "Calculate",
		Inputs:       map[string]interface{}{"a": 1, "b": 2},
		ArgOrder:     []string{"a", "b"},
	}

	action := adapter.generateAction(spec)
	assert.Equal(t, "result := Calculate(a, b)", action)
}

func TestGoSpecAdapter_GenerateAction_IndexedArgs(t *testing.T) {
	adapter := NewGoSpecAdapter()
	spec := model.TestSpec{
		FunctionName: "Process",
		Inputs:       map[string]interface{}{"arg0": 1, "arg1": 2, "arg2": 3},
	}

	action := adapter.generateAction(spec)
	assert.Contains(t, action, "result := Process(")
	assert.Contains(t, action, "arg0")
}

func TestGoSpecAdapter_GenerateFromSpecs_WithImports(t *testing.T) {
	adapter := NewGoSpecAdapter()
	specs := []model.TestSpec{
		{
			FunctionName: "Search",
			Description:  "contains substring",
			Inputs:       map[string]interface{}{"s": "hello world"},
			ArgOrder:     []string{"s"},
			Assertions: []model.Assertion{
				{Kind: "contains", Expected: "hello"},
			},
		},
	}

	code, err := adapter.GenerateFromSpecs(specs, "search.go")
	require.NoError(t, err)

	// Should include strings import
	assert.Contains(t, code, `"strings"`)
}

func TestGoSpecAdapter_GenerateFromSpecs_ReflectImport(t *testing.T) {
	adapter := NewGoSpecAdapter()
	specs := []model.TestSpec{
		{
			FunctionName: "GetType",
			Description:  "returns correct type",
			Inputs:       map[string]interface{}{},
			Assertions: []model.Assertion{
				{Kind: "type_is", Expected: "string"},
			},
		},
	}

	code, err := adapter.GenerateFromSpecs(specs, "types.go")
	require.NoError(t, err)

	assert.Contains(t, code, `"reflect"`)
}

func TestGoSpecAdapter_GenerateFromSpecs_NoAssertions(t *testing.T) {
	adapter := NewGoSpecAdapter()
	specs := []model.TestSpec{
		{
			FunctionName: "DoSomething",
			Description:  "does something",
			Inputs:       map[string]interface{}{},
			Assertions:   []model.Assertion{},
		},
	}

	code, err := adapter.GenerateFromSpecs(specs, "do.go")
	require.NoError(t, err)

	assert.Contains(t, code, "TODO: Add assertions")
}

func TestGoSpecAdapter_GenerateFromSpecs_TargetID(t *testing.T) {
	adapter := NewGoSpecAdapter()
	specs := []model.TestSpec{
		{
			TargetID:    "MyFunction",
			Description: "test case",
			Inputs:      map[string]interface{}{},
			Assertions:  []model.Assertion{{Kind: "equals", Expected: 1}},
		},
	}

	code, err := adapter.GenerateFromSpecs(specs, "my.go")
	require.NoError(t, err)

	assert.Contains(t, code, "TestMyFunction")
}

func TestGoSpecAdapter_GenerateFromSpecs_OutputStructure(t *testing.T) {
	adapter := NewGoSpecAdapter()
	specs := []model.TestSpec{
		{
			FunctionName: "Test",
			Description:  "basic test",
			Inputs:       map[string]interface{}{"x": 1},
			ArgOrder:     []string{"x"},
			Assertions:   []model.Assertion{{Kind: "equals", Expected: 1}},
		},
	}

	code, err := adapter.GenerateFromSpecs(specs, "test.go")
	require.NoError(t, err)

	// Check code structure
	assert.True(t, strings.HasPrefix(code, "package"))
	assert.Contains(t, code, "import")
	assert.Contains(t, code, `"testing"`)
	assert.Contains(t, code, "func Test")
	assert.Contains(t, code, "t.Run")
	assert.Contains(t, code, "// Setup")
	assert.Contains(t, code, "// Act")
	assert.Contains(t, code, "// Assert")
}
