package model

// IRSpec is the Universal Intermediate Representation Specification
// This is a language-agnostic test specification that LLMs output in JSON format.
// Framework adapters (Go, Jest, pytest, etc.) translate this to specific test code.
//
// Design principles:
// 1. Language-agnostic: No language-specific types or constructs
// 2. Given-When-Then: Universal pattern understood by all test frameworks
// 3. Type-hinted values: Include type information for proper code generation
// 4. Serializable: Clean JSON/YAML representation for LLM output

// IRTestSuite represents a collection of test cases for a function
type IRTestSuite struct {
	// FunctionName is the target function being tested
	FunctionName string `json:"function_name"`

	// Description provides context about what's being tested
	Description string `json:"description,omitempty"`

	// Tests is the list of individual test cases
	Tests []IRTestCase `json:"tests"`
}

// IRTestCase represents a single test case in Given-When-Then format
type IRTestCase struct {
	// Name is a descriptive name for the test case (snake_case preferred)
	Name string `json:"name"`

	// Description explains what behavior is being tested
	Description string `json:"description,omitempty"`

	// Given contains the preconditions/setup (variable assignments)
	Given []IRVariable `json:"given"`

	// When describes the action being tested
	When IRAction `json:"when"`

	// Then contains the expected outcomes/assertions
	Then []IRAssertion `json:"then"`

	// Tags for categorization (e.g., "edge_case", "error", "happy_path")
	Tags []string `json:"tags,omitempty"`
}

// IRVariable represents a typed variable for test setup
type IRVariable struct {
	// Name is the variable identifier
	Name string `json:"name"`

	// Value is the variable value (any JSON-compatible value)
	Value interface{} `json:"value"`

	// Type hints the language-agnostic type
	// Supported: "int", "float", "string", "bool", "null",
	//            "array", "object", "function" (for callbacks/mocks)
	Type string `json:"type"`
}

// IRAction represents the function call being tested
type IRAction struct {
	// Call is the function invocation pattern
	// References variables from Given section using $name syntax
	// Example: "Add($a, $b)" or "user.Save()"
	Call string `json:"call"`

	// Args lists the argument variable names in order
	// Example: ["a", "b"] for Add(a, b)
	Args []string `json:"args,omitempty"`
}

// IRAssertion represents an expected outcome
type IRAssertion struct {
	// Type is the assertion kind
	// Supported: "equals", "not_equals", "contains", "not_contains",
	//            "greater_than", "less_than", "throws", "truthy", "falsy",
	//            "nil", "not_nil", "length", "type_is"
	Type string `json:"type"`

	// Actual is what we're checking (usually "result" or an expression)
	// Special values: "result" (function return), "error" (exception)
	Actual string `json:"actual"`

	// Expected is the expected value (for equality-type assertions)
	Expected interface{} `json:"expected,omitempty"`

	// Message is an optional custom error message
	Message string `json:"message,omitempty"`
}

// IRSpecJSONSchema returns the JSON schema for prompting LLMs
// This is included in the system prompt to guide structured output
const IRSpecJSONSchema = `{
  "type": "object",
  "required": ["function_name", "tests"],
  "properties": {
    "function_name": {
      "type": "string",
      "description": "Name of the function being tested"
    },
    "description": {
      "type": "string",
      "description": "Brief description of what is being tested"
    },
    "tests": {
      "type": "array",
      "minItems": 1,
      "maxItems": 6,
      "items": {
        "type": "object",
        "required": ["name", "given", "when", "then"],
        "properties": {
          "name": {
            "type": "string",
            "description": "Test case name in snake_case (e.g., 'add_positive_numbers')"
          },
          "description": {
            "type": "string",
            "description": "What behavior this test verifies"
          },
          "given": {
            "type": "array",
            "description": "Setup variables with name, value, and type",
            "items": {
              "type": "object",
              "required": ["name", "value", "type"],
              "properties": {
                "name": {"type": "string"},
                "value": {},
                "type": {"type": "string", "enum": ["int", "float", "string", "bool", "null", "array", "object"]}
              }
            }
          },
          "when": {
            "type": "object",
            "required": ["call", "args"],
            "properties": {
              "call": {
                "type": "string",
                "description": "Function call pattern (e.g., 'Add($a, $b)')"
              },
              "args": {
                "type": "array",
                "items": {"type": "string"},
                "description": "Variable names to pass as arguments"
              }
            }
          },
          "then": {
            "type": "array",
            "minItems": 1,
            "items": {
              "type": "object",
              "required": ["type", "actual"],
              "properties": {
                "type": {
                  "type": "string",
                  "enum": ["equals", "not_equals", "contains", "greater_than", "less_than", "throws", "truthy", "falsy", "nil", "not_nil"]
                },
                "actual": {
                  "type": "string",
                  "description": "What to check (usually 'result' for function return value)"
                },
                "expected": {
                  "description": "Expected value for comparison"
                }
              }
            }
          },
          "tags": {
            "type": "array",
            "items": {"type": "string"}
          }
        }
      }
    }
  }
}`

// IRSpecExample provides an example for few-shot prompting
const IRSpecExample = `{
  "function_name": "Add",
  "description": "Tests for the Add function that sums two integers",
  "tests": [
    {
      "name": "add_positive_numbers",
      "description": "Adding two positive numbers returns their sum",
      "given": [
        {"name": "a", "value": 5, "type": "int"},
        {"name": "b", "value": 3, "type": "int"}
      ],
      "when": {
        "call": "Add($a, $b)",
        "args": ["a", "b"]
      },
      "then": [
        {"type": "equals", "actual": "result", "expected": 8}
      ],
      "tags": ["happy_path"]
    },
    {
      "name": "add_negative_numbers",
      "description": "Adding two negative numbers returns negative sum",
      "given": [
        {"name": "a", "value": -2, "type": "int"},
        {"name": "b", "value": -4, "type": "int"}
      ],
      "when": {
        "call": "Add($a, $b)",
        "args": ["a", "b"]
      },
      "then": [
        {"type": "equals", "actual": "result", "expected": -6}
      ],
      "tags": ["edge_case"]
    },
    {
      "name": "add_with_zero",
      "description": "Adding zero returns the other number unchanged",
      "given": [
        {"name": "a", "value": 0, "type": "int"},
        {"name": "b", "value": 7, "type": "int"}
      ],
      "when": {
        "call": "Add($a, $b)",
        "args": ["a", "b"]
      },
      "then": [
        {"type": "equals", "actual": "result", "expected": 7}
      ],
      "tags": ["boundary"]
    }
  ]
}`
