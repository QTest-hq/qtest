package adapters

import (
	"strings"
	"testing"

	"github.com/QTest-hq/qtest/pkg/dsl"
)

func TestNewJestAdapter(t *testing.T) {
	adapter := NewJestAdapter()
	if adapter == nil {
		t.Fatal("NewJestAdapter returned nil")
	}
}

func TestJestAdapter_Framework(t *testing.T) {
	adapter := NewJestAdapter()
	if adapter.Framework() != FrameworkJest {
		t.Errorf("Framework() = %s, want jest", adapter.Framework())
	}
}

func TestJestAdapter_FileExtension(t *testing.T) {
	adapter := NewJestAdapter()
	if adapter.FileExtension() != ".ts" {
		t.Errorf("FileExtension() = %s, want .ts", adapter.FileExtension())
	}
}

func TestJestAdapter_TestFileSuffix(t *testing.T) {
	adapter := NewJestAdapter()
	if adapter.TestFileSuffix() != ".test" {
		t.Errorf("TestFileSuffix() = %s, want .test", adapter.TestFileSuffix())
	}
}

func TestJestAdapter_Generate_SimpleTest(t *testing.T) {
	adapter := NewJestAdapter()
	testDSL := &dsl.TestDSL{
		Name: "adds two numbers",
		Target: dsl.TestTarget{
			File:     "math.ts",
			Function: "add",
		},
		Steps: []dsl.TestStep{
			{
				ID:          "step_1",
				Description: "Add two numbers",
				Action: dsl.StepAction{
					Type:   dsl.ActionCall,
					Target: "add",
					Args:   []interface{}{2, 3},
				},
				Expected: &dsl.Expected{
					Value: 5,
				},
			},
		},
	}

	code, err := adapter.Generate(testDSL)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if !strings.Contains(code, "describe('add'") {
		t.Error("code should contain describe block")
	}
	if !strings.Contains(code, "adds two numbers") {
		t.Error("code should contain test name")
	}
	if !strings.Contains(code, "add(2, 3)") {
		t.Error("code should contain function call")
	}
	if !strings.Contains(code, "expect(result).toBe(5)") {
		t.Error("code should contain assertion")
	}
}

func TestJestAdapter_Generate_AsyncTest(t *testing.T) {
	adapter := NewJestAdapter()
	testDSL := &dsl.TestDSL{
		Name: "fetches data",
		Target: dsl.TestTarget{
			Function: "fetchData",
		},
		Steps: []dsl.TestStep{
			{
				Description: "HTTP call",
				Action: dsl.StepAction{
					Type:   dsl.ActionHTTP,
					Target: "/api/users",
					Method: "GET",
				},
			},
		},
	}

	code, err := adapter.Generate(testDSL)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if !strings.Contains(code, "async") {
		t.Error("code should contain async")
	}
	if !strings.Contains(code, "await fetch") {
		t.Error("code should contain await fetch")
	}
}

func TestJestAdapter_Generate_WithLifecycle(t *testing.T) {
	adapter := NewJestAdapter()
	testDSL := &dsl.TestDSL{
		Name: "test",
		Target: dsl.TestTarget{
			Function: "func",
		},
		Lifecycle: &dsl.Lifecycle{
			BeforeEach: []dsl.Action{
				{Type: "db_setup"},
			},
			AfterEach: []dsl.Action{
				{Type: "cleanup"},
			},
		},
		Steps: []dsl.TestStep{},
	}

	code, err := adapter.Generate(testDSL)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if !strings.Contains(code, "beforeEach") {
		t.Error("code should contain beforeEach")
	}
	if !strings.Contains(code, "afterEach") {
		t.Error("code should contain afterEach")
	}
}

func TestHasAsyncSteps(t *testing.T) {
	t.Run("no async steps", func(t *testing.T) {
		testDSL := &dsl.TestDSL{
			Steps: []dsl.TestStep{
				{Action: dsl.StepAction{Type: dsl.ActionCall}},
			},
		}
		if hasAsyncSteps(testDSL) {
			t.Error("should return false for call action")
		}
	})

	t.Run("http step", func(t *testing.T) {
		testDSL := &dsl.TestDSL{
			Steps: []dsl.TestStep{
				{Action: dsl.StepAction{Type: dsl.ActionHTTP}},
			},
		}
		if !hasAsyncSteps(testDSL) {
			t.Error("should return true for HTTP action")
		}
	})

	t.Run("wait step", func(t *testing.T) {
		testDSL := &dsl.TestDSL{
			Steps: []dsl.TestStep{
				{Action: dsl.StepAction{Type: dsl.ActionWait}},
			},
		}
		if !hasAsyncSteps(testDSL) {
			t.Error("should return true for wait action")
		}
	})

	t.Run("empty steps", func(t *testing.T) {
		testDSL := &dsl.TestDSL{Steps: []dsl.TestStep{}}
		if hasAsyncSteps(testDSL) {
			t.Error("should return false for empty steps")
		}
	})
}

func TestFormatJSArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []interface{}
		expected string
	}{
		{"empty", []interface{}{}, ""},
		{"int", []interface{}{42}, "42"},
		{"string", []interface{}{"hello"}, "'hello'"},
		{"multiple", []interface{}{1, "a", true}, "1, 'a', true"},
		{"nil", nil, ""},
		{"float", []interface{}{3.14}, "3.14"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatJSArgs(tt.args)
			if result != tt.expected {
				t.Errorf("formatJSArgs() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGenerateJestAssertions(t *testing.T) {
	t.Run("value assertion int", func(t *testing.T) {
		expected := &dsl.Expected{Value: 42}
		result := generateJestAssertions(expected)
		if !strings.Contains(result, "expect(result).toBe(42)") {
			t.Errorf("assertion should check value, got %q", result)
		}
	})

	t.Run("value assertion string", func(t *testing.T) {
		expected := &dsl.Expected{Value: "hello"}
		result := generateJestAssertions(expected)
		if !strings.Contains(result, "expect(result).toBe('hello')") {
			t.Errorf("assertion should check string value, got %q", result)
		}
	})

	t.Run("type assertion", func(t *testing.T) {
		expected := &dsl.Expected{Type: "number"}
		result := generateJestAssertions(expected)
		if !strings.Contains(result, "expect(typeof result).toBe('number')") {
			t.Errorf("assertion should check type, got %q", result)
		}
	})

	t.Run("contains assertion", func(t *testing.T) {
		expected := &dsl.Expected{Contains: "hello"}
		result := generateJestAssertions(expected)
		if !strings.Contains(result, "expect(result).toContain('hello')") {
			t.Errorf("assertion should check contains, got %q", result)
		}
	})

	t.Run("error assertion", func(t *testing.T) {
		expected := &dsl.Expected{Error: &dsl.ExpectedError{}}
		result := generateJestAssertions(expected)
		if !strings.Contains(result, "expect(() => result).toThrow()") {
			t.Errorf("assertion should check error, got %q", result)
		}
	})

	t.Run("empty expected", func(t *testing.T) {
		// Empty expected (all nil fields) should return empty string
		expected := &dsl.Expected{}
		result := generateJestAssertions(expected)
		if result != "" {
			t.Errorf("empty expected should return empty string, got %q", result)
		}
	})
}

func TestGenerateJestAction(t *testing.T) {
	t.Run("mock action with target", func(t *testing.T) {
		action := dsl.Action{
			Type:   "mock",
			Params: map[string]interface{}{"target": "./module"},
		}
		result := generateJestAction(action)
		if !strings.Contains(result, "jest.mock('./module')") {
			t.Errorf("should generate mock, got %q", result)
		}
	})

	t.Run("mock action without target", func(t *testing.T) {
		action := dsl.Action{Type: "mock", Params: map[string]interface{}{}}
		result := generateJestAction(action)
		if !strings.Contains(result, "mock setup") {
			t.Errorf("should generate comment, got %q", result)
		}
	})

	t.Run("db_setup action", func(t *testing.T) {
		action := dsl.Action{Type: "db_setup"}
		result := generateJestAction(action)
		if !strings.Contains(result, "database setup") {
			t.Errorf("should generate db comment, got %q", result)
		}
	})

	t.Run("unknown action", func(t *testing.T) {
		action := dsl.Action{Type: "custom"}
		result := generateJestAction(action)
		if !strings.Contains(result, "custom") {
			t.Errorf("should include action type, got %q", result)
		}
	})
}

func TestGenerateJestStepCode(t *testing.T) {
	t.Run("call action with expected", func(t *testing.T) {
		step := dsl.TestStep{
			Action: dsl.StepAction{
				Type:   dsl.ActionCall,
				Target: "add",
				Args:   []interface{}{1, 2},
			},
			Expected: &dsl.Expected{Value: 3},
		}
		code := generateJestStepCode(step)
		if !strings.Contains(code, "const result = add(1, 2)") {
			t.Errorf("code should assign result, got %q", code)
		}
	})

	t.Run("call action without expected", func(t *testing.T) {
		step := dsl.TestStep{
			Action: dsl.StepAction{
				Type:   dsl.ActionCall,
				Target: "func",
				Args:   []interface{}{},
			},
		}
		code := generateJestStepCode(step)
		if strings.Contains(code, "const result") {
			t.Error("code should not assign result when no expected")
		}
	})

	t.Run("http action", func(t *testing.T) {
		step := dsl.TestStep{
			Action: dsl.StepAction{
				Type:   dsl.ActionHTTP,
				Target: "/api/data",
				Method: "POST",
			},
		}
		code := generateJestStepCode(step)
		if !strings.Contains(code, "await fetch('/api/data'") {
			t.Errorf("code should use fetch, got %q", code)
		}
		if !strings.Contains(code, "method: 'POST'") {
			t.Errorf("code should specify method, got %q", code)
		}
	})

	t.Run("http action default method", func(t *testing.T) {
		step := dsl.TestStep{
			Action: dsl.StepAction{
				Type:   dsl.ActionHTTP,
				Target: "/api/data",
			},
		}
		code := generateJestStepCode(step)
		if !strings.Contains(code, "method: 'GET'") {
			t.Errorf("code should default to GET, got %q", code)
		}
	})

	t.Run("unknown action", func(t *testing.T) {
		step := dsl.TestStep{
			Action: dsl.StepAction{
				Type:   "custom",
				Target: "something",
			},
		}
		code := generateJestStepCode(step)
		if !strings.Contains(code, "// custom") {
			t.Errorf("code should comment unknown action, got %q", code)
		}
	})
}

func TestJestAdapter_Generate_EmptySteps(t *testing.T) {
	adapter := NewJestAdapter()
	testDSL := &dsl.TestDSL{
		Name: "empty test",
		Target: dsl.TestTarget{
			Function: "func",
		},
		Steps: []dsl.TestStep{},
	}

	code, err := adapter.Generate(testDSL)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if !strings.Contains(code, "describe('func'") {
		t.Error("code should contain describe block")
	}
}

func TestJestAdapter_Generate_StringArgs(t *testing.T) {
	adapter := NewJestAdapter()
	testDSL := &dsl.TestDSL{
		Name: "greets user",
		Target: dsl.TestTarget{
			File:     "greet.ts",
			Function: "greet",
		},
		Steps: []dsl.TestStep{
			{
				Description: "Greet user",
				Action: dsl.StepAction{
					Type:   dsl.ActionCall,
					Target: "greet",
					Args:   []interface{}{"Alice"},
				},
			},
		},
	}

	code, err := adapter.Generate(testDSL)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if !strings.Contains(code, "greet('Alice')") {
		t.Error("code should contain string argument")
	}
}
