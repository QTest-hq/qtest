package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertToDSL_SimpleTestList(t *testing.T) {
	yaml := `
- name: "Add positive numbers"
  setup:
    a: 2
    b: 3
  action: "Add(a, b)"
  assertions:
    result: 5
- name: "Add negative numbers"
  setup:
    a: -1
    b: -2
  action: "Add(a, b)"
  assertions:
    result: -3
`
	dsl, err := ConvertToDSL(yaml, "Add", "math.go", "go")
	require.NoError(t, err)
	assert.Equal(t, "Test_Add", dsl.Name)
	assert.Equal(t, "1.0", dsl.Version)
	assert.Len(t, dsl.Steps, 2)

	// Check first step
	assert.Equal(t, "Add positive numbers", dsl.Steps[0].Description)
	assert.Equal(t, "Add", dsl.Steps[0].Action.Target)
	// Args should be resolved from setup
	assert.Contains(t, dsl.Steps[0].Action.Args, 2)
	assert.Contains(t, dsl.Steps[0].Action.Args, 3)
	assert.Equal(t, 5, dsl.Steps[0].Expected.Value)
}

func TestConvertToDSL_SingleTest(t *testing.T) {
	// Note: When YAML has name: at root, it matches fullDSL format check first
	// This tests the actual current behavior
	yaml := `
name: "Multiply numbers"
setup:
  x: 4
  y: 5
action: "Multiply(x, y)"
assertions:
  result: 20
`
	dsl, err := ConvertToDSL(yaml, "Multiply", "math.go", "go")
	require.NoError(t, err)
	// fullDSL format is detected due to name: field at root
	assert.Equal(t, "Multiply numbers", dsl.Name)
	// Steps are empty in fullDSL format without steps: field
	assert.Len(t, dsl.Steps, 0)
}

func TestConvertToDSL_SingleTestAsList(t *testing.T) {
	// Single test wrapped as list to ensure proper conversion
	yaml := `
- name: "Multiply numbers"
  setup:
    x: 4
    y: 5
  action: "Multiply(x, y)"
  assertions:
    result: 20
`
	dsl, err := ConvertToDSL(yaml, "Multiply", "math.go", "go")
	require.NoError(t, err)
	assert.Equal(t, "Test_Multiply", dsl.Name)
	assert.Len(t, dsl.Steps, 1)
	assert.Equal(t, "Multiply numbers", dsl.Steps[0].Description)
	assert.Equal(t, 20, dsl.Steps[0].Expected.Value)
}

func TestConvertToDSL_WrapperFormat(t *testing.T) {
	yaml := `
tests:
  - name: "Test one"
    setup:
      val: 10
    action: "Process(val)"
    assertions:
      result: 100
  - name: "Test two"
    setup:
      val: 20
    action: "Process(val)"
    assertions:
      result: 400
`
	dsl, err := ConvertToDSL(yaml, "Process", "process.go", "go")
	require.NoError(t, err)
	assert.Len(t, dsl.Steps, 2)
	assert.Equal(t, "Test one", dsl.Steps[0].Description)
	assert.Equal(t, "Test two", dsl.Steps[1].Description)
}

func TestConvertToDSL_DirectArgsInAction(t *testing.T) {
	yaml := `
- name: "Direct args test"
  action: "Add(10, 20)"
  assertions:
    result: 30
`
	dsl, err := ConvertToDSL(yaml, "Add", "math.go", "go")
	require.NoError(t, err)
	assert.Len(t, dsl.Steps, 1)
	// Args should be parsed from action string
	assert.Len(t, dsl.Steps[0].Action.Args, 2)
	assert.Equal(t, "10", dsl.Steps[0].Action.Args[0])
	assert.Equal(t, "20", dsl.Steps[0].Action.Args[1])
}

func TestConvertToDSL_MapStyleAction(t *testing.T) {
	yaml := `
- name: "Map style action"
  action:
    function: "Add"
    args: [5, 6]
  assertions:
    result: 11
`
	dsl, err := ConvertToDSL(yaml, "Add", "math.go", "go")
	require.NoError(t, err)
	assert.Len(t, dsl.Steps, 1)
	assert.Len(t, dsl.Steps[0].Action.Args, 2)
}

func TestConvertToDSL_InvalidYAML(t *testing.T) {
	yaml := `this is not valid yaml: [[[`
	_, err := ConvertToDSL(yaml, "Func", "file.go", "go")
	assert.Error(t, err)
}

func TestConvertToDSL_EmptyYAML(t *testing.T) {
	yaml := ``
	_, err := ConvertToDSL(yaml, "Func", "file.go", "go")
	assert.Error(t, err)
}

func TestResolveArgs_VariablePatterns(t *testing.T) {
	tests := []struct {
		name     string
		args     []interface{}
		setup    map[string]interface{}
		expected []interface{}
	}{
		{
			name:     "bare variable",
			args:     []interface{}{"a", "b"},
			setup:    map[string]interface{}{"a": 1, "b": 2},
			expected: []interface{}{1, 2},
		},
		{
			name:     "dollar variable",
			args:     []interface{}{"$a", "$b"},
			setup:    map[string]interface{}{"a": 10, "b": 20},
			expected: []interface{}{10, 20},
		},
		{
			name:     "braced variable",
			args:     []interface{}{"${x}", "${y}"},
			setup:    map[string]interface{}{"x": 100, "y": 200},
			expected: []interface{}{100, 200},
		},
		{
			name:     "mixed patterns",
			args:     []interface{}{"a", "$b", "${c}"},
			setup:    map[string]interface{}{"a": 1, "b": 2, "c": 3},
			expected: []interface{}{1, 2, 3},
		},
		{
			name:     "unresolved variable",
			args:     []interface{}{"unknown"},
			setup:    map[string]interface{}{"a": 1},
			expected: []interface{}{"unknown"},
		},
		{
			name:     "literal values",
			args:     []interface{}{42, "hello", true},
			setup:    map[string]interface{}{},
			expected: []interface{}{42, "hello", true},
		},
		{
			name:     "empty setup",
			args:     []interface{}{"a", "b"},
			setup:    nil,
			expected: []interface{}{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveArgs(tt.args, tt.setup)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseActionArgs(t *testing.T) {
	tests := []struct {
		name     string
		action   string
		expected []interface{}
	}{
		{
			name:     "simple args",
			action:   "Add(1, 2)",
			expected: []interface{}{"1", "2"},
		},
		{
			name:     "variable args",
			action:   "Add(a, b)",
			expected: []interface{}{"a", "b"},
		},
		{
			name:     "no args",
			action:   "GetValue()",
			expected: nil,
		},
		{
			name:     "no parentheses",
			action:   "JustAName",
			expected: nil,
		},
		{
			name:     "single arg",
			action:   "Process(x)",
			expected: []interface{}{"x"},
		},
		{
			name:     "spaced args",
			action:   "Add( 1 , 2 )",
			expected: []interface{}{"1", "2"},
		},
		{
			name:     "negative numbers",
			action:   "Add(-5, -10)",
			expected: []interface{}{"-5", "-10"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseActionArgs(tt.action)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseAction_StringFormat(t *testing.T) {
	result := parseAction("Add(1, 2)", "Add")
	assert.Equal(t, []interface{}{"1", "2"}, result)
}

func TestParseAction_MapFormat(t *testing.T) {
	action := map[string]interface{}{
		"function": "Add",
		"args":     []interface{}{1, 2},
	}
	result := parseAction(action, "Add")
	assert.Equal(t, []interface{}{1, 2}, result)
}

func TestParseAction_MapArgsFormat(t *testing.T) {
	action := map[string]interface{}{
		"function": "Add",
		"args": map[string]interface{}{
			"a": 1,
			"b": 2,
		},
	}
	result := parseAction(action, "Add")
	assert.Len(t, result, 2)
}

func TestParseAction_Nil(t *testing.T) {
	result := parseAction(nil, "Func")
	assert.Nil(t, result)
}

func TestParseAssertions_MapFormat(t *testing.T) {
	assertions := map[string]interface{}{
		"result": 42,
		"error":  nil,
	}
	expected := parseAssertions(assertions)
	assert.Equal(t, 42, expected.Value)
	assert.Equal(t, 42, expected.Properties["result"])
}

func TestParseAssertions_ListFormat(t *testing.T) {
	assertions := []interface{}{
		map[string]interface{}{"result": 10},
		map[string]interface{}{"error": "none"},
	}
	expected := parseAssertions(assertions)
	assert.Equal(t, 10, expected.Value)
	assert.Equal(t, "none", expected.Properties["error"])
}

func TestParseAssertions_DirectValue(t *testing.T) {
	expected := parseAssertions(42)
	assert.Equal(t, 42, expected.Value)
}

func TestParseAssertions_Nil(t *testing.T) {
	expected := parseAssertions(nil)
	assert.Nil(t, expected)
}

func TestConvertToDSL_TargetInfo(t *testing.T) {
	yaml := `
- name: "Test"
  action: "Func()"
  assertions:
    result: 1
`
	dsl, err := ConvertToDSL(yaml, "MyFunc", "/path/to/file.go", "go")
	require.NoError(t, err)
	assert.Equal(t, "/path/to/file.go", dsl.Target.File)
	assert.Equal(t, "MyFunc", dsl.Target.Function)
}

func TestConvertToDSL_StepIDs(t *testing.T) {
	yaml := `
- name: "First"
  action: "F()"
  assertions: {result: 1}
- name: "Second"
  action: "F()"
  assertions: {result: 2}
- name: "Third"
  action: "F()"
  assertions: {result: 3}
`
	dsl, err := ConvertToDSL(yaml, "F", "f.go", "go")
	require.NoError(t, err)
	assert.Equal(t, "step_1", dsl.Steps[0].ID)
	assert.Equal(t, "step_2", dsl.Steps[1].ID)
	assert.Equal(t, "step_3", dsl.Steps[2].ID)
}
