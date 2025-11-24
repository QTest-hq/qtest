package adapters

import (
	"strings"
	"testing"

	"github.com/QTest-hq/qtest/pkg/dsl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGoAdapter(t *testing.T) {
	adapter := NewGoAdapter()
	assert.NotNil(t, adapter)
}

func TestGoAdapter_Framework(t *testing.T) {
	adapter := NewGoAdapter()
	assert.Equal(t, FrameworkGoTest, adapter.Framework())
}

func TestGoAdapter_FileExtension(t *testing.T) {
	adapter := NewGoAdapter()
	assert.Equal(t, ".go", adapter.FileExtension())
}

func TestGoAdapter_TestFileSuffix(t *testing.T) {
	adapter := NewGoAdapter()
	assert.Equal(t, "_test", adapter.TestFileSuffix())
}

func TestGoAdapter_Generate_SimpleTest(t *testing.T) {
	adapter := NewGoAdapter()
	testDSL := &dsl.TestDSL{
		Name:    "Test_Add",
		Version: "1.0",
		Target: dsl.TestTarget{
			File:     "examples/math.go",
			Function: "Add",
		},
		Steps: []dsl.TestStep{
			{
				ID:          "step_1",
				Description: "Add two numbers",
				Action: dsl.StepAction{
					Type:   dsl.ActionCall,
					Target: "Add",
					Args:   []interface{}{2, 3},
				},
				Expected: &dsl.Expected{
					Value: 5,
				},
			},
		},
	}

	code, err := adapter.Generate(testDSL)
	require.NoError(t, err)
	assert.Contains(t, code, "package examples")
	// Name "Test_Add" becomes "TestAdd" via toGoFunctionName, then template adds "Test" prefix
	assert.Contains(t, code, "func TestTestAdd(t *testing.T)")
	assert.Contains(t, code, "Add(2, 3)")
	assert.Contains(t, code, "Add two numbers")
}

func TestGoAdapter_Generate_MultipleSteps(t *testing.T) {
	adapter := NewGoAdapter()
	testDSL := &dsl.TestDSL{
		Name: "Test_Math",
		Target: dsl.TestTarget{
			File:     "math.go",
			Function: "Math",
		},
		Steps: []dsl.TestStep{
			{
				ID:          "step_1",
				Description: "First test",
				Action: dsl.StepAction{
					Type:   dsl.ActionCall,
					Target: "Add",
					Args:   []interface{}{1, 2},
				},
			},
			{
				ID:          "step_2",
				Description: "Second test",
				Action: dsl.StepAction{
					Type:   dsl.ActionCall,
					Target: "Sub",
					Args:   []interface{}{5, 3},
				},
			},
		},
	}

	code, err := adapter.Generate(testDSL)
	require.NoError(t, err)
	assert.Contains(t, code, "First test")
	assert.Contains(t, code, "Second test")
	assert.Contains(t, code, "Add(1, 2)")
	assert.Contains(t, code, "Sub(5, 3)")
}

func TestGoAdapter_Generate_HTTPAction(t *testing.T) {
	adapter := NewGoAdapter()
	testDSL := &dsl.TestDSL{
		Name: "Test_API",
		Target: dsl.TestTarget{
			File:     "api.go",
			Function: "Handler",
		},
		Steps: []dsl.TestStep{
			{
				ID:          "step_1",
				Description: "GET endpoint",
				Action: dsl.StepAction{
					Type:   dsl.ActionHTTP,
					Target: "/api/users",
					Method: "GET",
				},
			},
		},
	}

	code, err := adapter.Generate(testDSL)
	require.NoError(t, err)
	assert.Contains(t, code, `httptest.NewRequest("GET", "/api/users"`)
}

func TestGoAdapter_Generate_WithAssertions(t *testing.T) {
	adapter := NewGoAdapter()
	testDSL := &dsl.TestDSL{
		Name: "Test_Func",
		Target: dsl.TestTarget{
			File:     "func.go",
			Function: "Func",
		},
		Steps: []dsl.TestStep{
			{
				ID:          "step_1",
				Description: "Test with assertion",
				Action: dsl.StepAction{
					Type:   dsl.ActionCall,
					Target: "Func",
					Args:   []interface{}{},
				},
				Expected: &dsl.Expected{
					Value: 42,
				},
			},
		},
	}

	code, err := adapter.Generate(testDSL)
	require.NoError(t, err)
	assert.Contains(t, code, "if result != 42")
	assert.Contains(t, code, `t.Errorf("expected %v, got %v"`)
}

func TestToGoFunctionName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"simple", "add", "Add"},
		{"underscore", "add_numbers", "AddNumbers"},
		{"hyphen", "add-numbers", "AddNumbers"},
		{"space", "add numbers", "AddNumbers"},
		{"mixed", "add_numbers-fast test", "AddNumbersFastTest"},
		{"already_pascal", "AddNumbers", "AddNumbers"},
		{"single_char", "a", "A"},
		{"dot_separator", "math.Add", "MathAdd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toGoFunctionName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractPackageName(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"simple_path", "examples/math.go", "examples"},
		{"nested_path", "internal/adapters/go.go", "adapters"},
		{"root_file", "main.go", "main"},
		{"deep_path", "a/b/c/d/file.go", "d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPackageName(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatGoArg(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"nil", nil, "nil"},
		{"int", 42, "42"},
		{"int64", int64(100), "100"},
		{"float", 3.14, "3.14"},
		{"bool_true", true, "true"},
		{"bool_false", false, "false"},
		{"string_number", "123", "123"},
		{"string_float", "3.14", "3.14"},
		{"string_bool_true", "true", "true"},
		{"string_bool_false", "false", "false"},
		{"string_literal", "hello", `"hello"`},
		{"empty_slice", []interface{}{}, "[]interface{}{}"},
		{"slice_with_values", []interface{}{1, 2}, "[]interface{}{1, 2}"},
		{"empty_map", map[string]interface{}{}, "map[string]interface{}{}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatGoArg(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsUnresolvedVariable(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"${var}", true},
		{"$var", true},
		{"*ptr", true},
		{"&ref", true},
		{"normalValue", false},
		{"123", false},
		{"hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isUnresolvedVariable(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetDefaultForVariable(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"pointer", "*ptr", "nil"},
		{"reference", "&ref", "nil"},
		{"string_hint_name", "${userName}", `""`},
		{"string_hint_path", "$filePath", `""`},
		{"string_hint_url", "${apiUrl}", `""`},
		{"string_hint_message", "${errorMessage}", `""`},
		{"bool_hint_is", "${isEnabled}", "false"},
		// Note: "hasValue" contains "value" which is a string hint (checked first)
		{"bool_hint_has_with_value", "${hasValue}", `""`},
		// Pure boolean variable without string hints
		{"bool_hint_has_pure", "${hasData}", "false"},
		{"bool_hint_can", "${canEdit}", "false"},
		{"bool_hint_flag", "${debugFlag}", "false"},
		// Note: "userList" contains "is" which triggers bool check
		{"slice_hint_list_with_is", "${userList}", "false"},
		// Pure list variable
		{"slice_hint_items", "${menuItems}", "nil"},
		{"numeric_default", "${count}", "0"},
		{"unknown", "${xyz}", "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getDefaultForVariable(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateGoAssertions(t *testing.T) {
	t.Run("value_assertion", func(t *testing.T) {
		step := dsl.TestStep{
			Expected: &dsl.Expected{
				Value: 42,
			},
		}
		assertions := generateGoAssertions(step)
		assert.Len(t, assertions, 1)
		assert.Contains(t, assertions[0], "if result != 42")
	})

	t.Run("error_assertion", func(t *testing.T) {
		step := dsl.TestStep{
			Expected: &dsl.Expected{
				Error: &dsl.ExpectedError{
					Message: "some error",
				},
			},
		}
		assertions := generateGoAssertions(step)
		assert.Len(t, assertions, 1)
		assert.Contains(t, assertions[0], "expected error")
	})

	t.Run("contains_assertion", func(t *testing.T) {
		step := dsl.TestStep{
			Expected: &dsl.Expected{
				Contains: "hello",
			},
		}
		assertions := generateGoAssertions(step)
		assert.Len(t, assertions, 1)
		assert.Contains(t, assertions[0], "strings.Contains")
	})

	t.Run("nil_expected", func(t *testing.T) {
		step := dsl.TestStep{
			Expected: nil,
		}
		assertions := generateGoAssertions(step)
		assert.Len(t, assertions, 0)
	})
}

func TestGenerateGoStepAction(t *testing.T) {
	t.Run("call_action", func(t *testing.T) {
		step := dsl.TestStep{
			Action: dsl.StepAction{
				Type:   dsl.ActionCall,
				Target: "Add",
				Args:   []interface{}{1, 2},
			},
		}
		result := generateGoStepAction(step)
		assert.Equal(t, "Add(1, 2)", result)
	})

	t.Run("call_no_args", func(t *testing.T) {
		step := dsl.TestStep{
			Action: dsl.StepAction{
				Type:   dsl.ActionCall,
				Target: "GetValue",
				Args:   []interface{}{},
			},
		}
		result := generateGoStepAction(step)
		assert.Equal(t, "GetValue()", result)
	})

	t.Run("http_get", func(t *testing.T) {
		step := dsl.TestStep{
			Action: dsl.StepAction{
				Type:   dsl.ActionHTTP,
				Target: "/api/users",
				Method: "GET",
			},
		}
		result := generateGoStepAction(step)
		assert.Contains(t, result, `"GET"`)
		assert.Contains(t, result, `"/api/users"`)
	})

	t.Run("http_default_method", func(t *testing.T) {
		step := dsl.TestStep{
			Action: dsl.StepAction{
				Type:   dsl.ActionHTTP,
				Target: "/api/data",
			},
		}
		result := generateGoStepAction(step)
		assert.Contains(t, result, `"GET"`)
	})

	t.Run("unknown_action", func(t *testing.T) {
		step := dsl.TestStep{
			Action: dsl.StepAction{
				Type:   "custom",
				Target: "something",
			},
		}
		result := generateGoStepAction(step)
		assert.Contains(t, result, "//")
		assert.Contains(t, result, "custom")
	})
}

func TestGenerateGoAction(t *testing.T) {
	tests := []struct {
		name     string
		action   dsl.Action
		contains string
	}{
		{"db_setup", dsl.Action{Type: "db_setup"}, "Setup database"},
		{"mock", dsl.Action{Type: "mock"}, "Mock setup"},
		{"custom", dsl.Action{Type: "custom_action"}, "custom_action"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateGoAction(tt.action)
			assert.Contains(t, result, tt.contains)
		})
	}
}

func TestGoAdapter_Generate_EmptySteps(t *testing.T) {
	adapter := NewGoAdapter()
	testDSL := &dsl.TestDSL{
		Name: "Test_Empty",
		Target: dsl.TestTarget{
			File:     "empty.go",
			Function: "Empty",
		},
		Steps: []dsl.TestStep{},
	}

	code, err := adapter.Generate(testDSL)
	require.NoError(t, err)
	// "Test_Empty" -> "TestEmpty" then template adds "Test" prefix
	assert.Contains(t, code, "func TestTestEmpty(t *testing.T)")
}

func TestGoAdapter_Generate_StringArgs(t *testing.T) {
	adapter := NewGoAdapter()
	testDSL := &dsl.TestDSL{
		Name: "Test_Greet",
		Target: dsl.TestTarget{
			File:     "greet.go",
			Function: "Greet",
		},
		Steps: []dsl.TestStep{
			{
				ID:          "step_1",
				Description: "Greet user",
				Action: dsl.StepAction{
					Type:   dsl.ActionCall,
					Target: "Greet",
					Args:   []interface{}{"Alice"},
				},
			},
		},
	}

	code, err := adapter.Generate(testDSL)
	require.NoError(t, err)
	assert.Contains(t, code, `Greet("Alice")`)
}

func TestFormatGoArg_UnresolvedVariables(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"dollar_var", "$count", "0"},
		{"braced_var", "${total}", "0"},
		{"string_var", "${message}", `""`},
		{"bool_var", "${isActive}", "false"},
		{"pointer_var", "*data", "nil"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatGoArg(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGoAdapter_Generate_FallbackFunctionName(t *testing.T) {
	adapter := NewGoAdapter()
	testDSL := &dsl.TestDSL{
		Name: "", // Empty name
		Target: dsl.TestTarget{
			File:     "calc.go",
			Function: "Calculate",
		},
		Steps: []dsl.TestStep{},
	}

	code, err := adapter.Generate(testDSL)
	require.NoError(t, err)
	assert.Contains(t, code, "func TestCalculate(t *testing.T)")
}

func TestGoAdapter_Generate_OutputFormat(t *testing.T) {
	adapter := NewGoAdapter()
	testDSL := &dsl.TestDSL{
		Name: "Test_Format",
		Target: dsl.TestTarget{
			File:     "format.go",
			Function: "Format",
		},
		Steps: []dsl.TestStep{},
	}

	code, err := adapter.Generate(testDSL)
	require.NoError(t, err)

	// Check structure
	assert.True(t, strings.HasPrefix(code, "package"))
	assert.Contains(t, code, `import (`)
	assert.Contains(t, code, `"testing"`)
	assert.Contains(t, code, "var result interface{}")
	assert.Contains(t, code, "_ = result")
}

// Mock Generation Tests

func TestGenerateGoMock_InterfaceMock(t *testing.T) {
	action := dsl.Action{
		Type: "mock",
		Params: map[string]interface{}{
			"type":   "interface",
			"target": "UserService",
		},
	}
	result := generateGoMock(action)
	assert.Contains(t, result, "mockUserService")
	assert.Contains(t, result, "MockUserService")
}

func TestGenerateGoMock_InterfaceWithReturn(t *testing.T) {
	action := dsl.Action{
		Type: "mock",
		Params: map[string]interface{}{
			"type":   "interface",
			"target": "Cache",
			"return": "cached_value",
		},
	}
	result := generateGoMock(action)
	assert.Contains(t, result, "mockCache")
	assert.Contains(t, result, "ReturnValue")
}

func TestGenerateGoMock_FunctionMock(t *testing.T) {
	action := dsl.Action{
		Type: "mock",
		Params: map[string]interface{}{
			"type":   "function",
			"target": "GetTime",
		},
	}
	result := generateGoMock(action)
	assert.Contains(t, result, "origGetTime")
	assert.Contains(t, result, "defer func()")
}

func TestGenerateGoMock_FunctionWithReturn(t *testing.T) {
	action := dsl.Action{
		Type: "mock",
		Params: map[string]interface{}{
			"type":   "function",
			"target": "FetchData",
			"return": 42,
		},
	}
	result := generateGoMock(action)
	assert.Contains(t, result, "origFetchData")
	assert.Contains(t, result, "return 42")
}

func TestGenerateGoMock_HTTPClient(t *testing.T) {
	action := dsl.Action{
		Type: "mock",
		Params: map[string]interface{}{
			"type": "http_client",
		},
	}
	result := generateGoMock(action)
	assert.Contains(t, result, "httptest.NewServer")
	assert.Contains(t, result, "server.Close()")
	assert.Contains(t, result, "server.Client()")
}

func TestGenerateGoMock_TimeMock(t *testing.T) {
	action := dsl.Action{
		Type: "mock",
		Params: map[string]interface{}{
			"type": "time",
		},
	}
	result := generateGoMock(action)
	assert.Contains(t, result, "time.Now()")
}

func TestGenerateGoMock_TimeMockWithReturn(t *testing.T) {
	action := dsl.Action{
		Type: "mock",
		Params: map[string]interface{}{
			"type":   "time",
			"return": "2024-01-01",
		},
	}
	result := generateGoMock(action)
	assert.Contains(t, result, "Mock time")
}

func TestGenerateGoMock_DefaultWithTarget(t *testing.T) {
	action := dsl.Action{
		Type: "mock",
		Params: map[string]interface{}{
			"target": "MyService",
		},
	}
	result := generateGoMock(action)
	assert.Contains(t, result, "Mock setup for MyService")
	assert.Contains(t, result, "MockMyService")
}

func TestGenerateGoMock_DefaultNoTarget(t *testing.T) {
	action := dsl.Action{
		Type:   "mock",
		Params: map[string]interface{}{},
	}
	result := generateGoMock(action)
	assert.Contains(t, result, "Mock setup")
}

func TestGenerateGoHTTPMock_Default(t *testing.T) {
	action := dsl.Action{
		Type:   "http_mock",
		Params: map[string]interface{}{},
	}
	result := generateGoHTTPMock(action)
	assert.Contains(t, result, "mockServer := httptest.NewServer")
	assert.Contains(t, result, `r.Method == "GET"`)
	assert.Contains(t, result, `r.URL.Path == "/"`)
	assert.Contains(t, result, "w.WriteHeader(200)")
}

func TestGenerateGoHTTPMock_Custom(t *testing.T) {
	action := dsl.Action{
		Type: "http_mock",
		Params: map[string]interface{}{
			"method":   "POST",
			"path":     "/api/users",
			"status":   201,
			"response": `{"id": 1}`,
		},
	}
	result := generateGoHTTPMock(action)
	assert.Contains(t, result, `r.Method == "POST"`)
	assert.Contains(t, result, `r.URL.Path == "/api/users"`)
	assert.Contains(t, result, "w.WriteHeader(201)")
	assert.Contains(t, result, "id")
}

func TestGenerateGoAction_HTTPMock(t *testing.T) {
	action := dsl.Action{
		Type: "http_mock",
		Params: map[string]interface{}{
			"method": "DELETE",
			"path":   "/api/item/1",
			"status": 204,
		},
	}
	result := generateGoAction(action)
	assert.Contains(t, result, "httptest.NewServer")
	assert.Contains(t, result, "DELETE")
}

func TestGenerateGoMock_InterfaceNoTarget(t *testing.T) {
	action := dsl.Action{
		Type: "mock",
		Params: map[string]interface{}{
			"type": "interface",
		},
	}
	result := generateGoMock(action)
	assert.Contains(t, result, "MockInterface")
}

func TestGenerateGoMock_FunctionNoTarget(t *testing.T) {
	action := dsl.Action{
		Type: "mock",
		Params: map[string]interface{}{
			"type": "function",
		},
	}
	result := generateGoMock(action)
	assert.Contains(t, result, "origFunc")
	assert.Contains(t, result, "targetFunc")
}

// Edge case tests for lifecycle hooks
func TestGoAdapter_Generate_WithLifecycleHooks(t *testing.T) {
	adapter := NewGoAdapter()
	testDSL := &dsl.TestDSL{
		Name: "test_with_lifecycle",
		Target: dsl.TestTarget{
			File:     "service.go",
			Function: "ProcessData",
		},
		Lifecycle: &dsl.Lifecycle{
			BeforeEach: []dsl.Action{
				{
					Type:   "setup",
					Params: map[string]interface{}{"target": "db.Connect()"},
				},
			},
			AfterEach: []dsl.Action{
				{
					Type:   "teardown",
					Params: map[string]interface{}{"target": "db.Close()"},
				},
			},
		},
		Steps: []dsl.TestStep{
			{
				Description: "Process data",
				Action: dsl.StepAction{
					Type:   dsl.ActionCall,
					Target: "ProcessData",
					Args:   []interface{}{"input"},
				},
			},
		},
	}

	code, err := adapter.Generate(testDSL)
	require.NoError(t, err)
	assert.Contains(t, code, "Setup")
}

// Edge case: error assertions
func TestGoAdapter_Generate_ErrorAssertion(t *testing.T) {
	adapter := NewGoAdapter()
	testDSL := &dsl.TestDSL{
		Name: "test_error_handling",
		Target: dsl.TestTarget{
			File:     "errors.go",
			Function: "ValidateInput",
		},
		Steps: []dsl.TestStep{
			{
				Description: "Invalid input returns error",
				Action: dsl.StepAction{
					Type:   dsl.ActionCall,
					Target: "ValidateInput",
					Args:   []interface{}{""},
				},
				Expected: &dsl.Expected{
					Error: &dsl.ExpectedError{
						Type: "ValidationError",
					},
				},
			},
		},
	}

	code, err := adapter.Generate(testDSL)
	require.NoError(t, err)
	assert.Contains(t, code, "expected error")
}

// Edge case: contains assertion
func TestGoAdapter_Generate_ContainsAssertion(t *testing.T) {
	adapter := NewGoAdapter()
	testDSL := &dsl.TestDSL{
		Name: "test_string_contains",
		Target: dsl.TestTarget{
			File:     "strings.go",
			Function: "BuildMessage",
		},
		Steps: []dsl.TestStep{
			{
				Description: "Message contains greeting",
				Action: dsl.StepAction{
					Type:   dsl.ActionCall,
					Target: "BuildMessage",
					Args:   []interface{}{"World"},
				},
				Expected: &dsl.Expected{
					Contains: "Hello",
				},
			},
		},
	}

	code, err := adapter.Generate(testDSL)
	require.NoError(t, err)
	assert.Contains(t, code, "strings.Contains")
	assert.Contains(t, code, "Hello")
}

// Edge case: empty test name uses function name
func TestGoAdapter_Generate_EmptyTestName(t *testing.T) {
	adapter := NewGoAdapter()
	testDSL := &dsl.TestDSL{
		Name: "",
		Target: dsl.TestTarget{
			File:     "math.go",
			Function: "Add",
		},
		Steps: []dsl.TestStep{
			{
				Description: "Add numbers",
				Action: dsl.StepAction{
					Type:   dsl.ActionCall,
					Target: "Add",
					Args:   []interface{}{1, 2},
				},
			},
		},
	}

	code, err := adapter.Generate(testDSL)
	require.NoError(t, err)
	assert.Contains(t, code, "func TestAdd")
}

// Edge case: nested map arguments
func TestFormatGoArg_NestedMap(t *testing.T) {
	nested := map[string]interface{}{
		"outer": map[string]interface{}{
			"inner": "value",
		},
	}
	result := formatGoArg(nested)
	// Should handle nested maps gracefully
	assert.NotEmpty(t, result)
}

// Edge case: nil expected value
func TestGenerateGoAssertions_NilExpected(t *testing.T) {
	step := dsl.TestStep{
		Expected: nil,
	}
	assertions := generateGoAssertions(step)
	assert.Empty(t, assertions)
}

// Edge case: empty expected value
func TestGenerateGoAssertions_EmptyExpected(t *testing.T) {
	step := dsl.TestStep{
		Expected: &dsl.Expected{},
	}
	assertions := generateGoAssertions(step)
	assert.Empty(t, assertions)
}

// Edge case: special characters in function name
// toGoFunctionName splits on space, underscore, hyphen, dot only
func TestToGoFunctionName_SpecialChars(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"test@user", "Test@user"},      // @ not a delimiter, remains in output
		{"test#123", "Test#123"},        // # not a delimiter
		{"test_with_underscores", "TestWithUnderscores"},
		{"test-with-hyphens", "TestWithHyphens"},
		{"test with  multiple   spaces", "TestWithMultipleSpaces"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := toGoFunctionName(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// Edge case: HTTP action with body
func TestGenerateGoStepAction_HTTPWithBody(t *testing.T) {
	step := dsl.TestStep{
		Action: dsl.StepAction{
			Type:   dsl.ActionHTTP,
			Target: "/api/users",
			Method: "POST",
		},
	}
	code := generateGoStepAction(step)
	assert.Contains(t, code, "httptest.NewRequest")
	assert.Contains(t, code, "POST")
}
