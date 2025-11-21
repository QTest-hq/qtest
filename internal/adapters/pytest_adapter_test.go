package adapters

import (
	"strings"
	"testing"

	"github.com/QTest-hq/qtest/pkg/dsl"
)

func TestNewPytestAdapter(t *testing.T) {
	adapter := NewPytestAdapter()
	if adapter == nil {
		t.Fatal("NewPytestAdapter returned nil")
	}
}

func TestPytestAdapter_Framework(t *testing.T) {
	adapter := NewPytestAdapter()
	if adapter.Framework() != FrameworkPytest {
		t.Errorf("Framework() = %s, want pytest", adapter.Framework())
	}
}

func TestPytestAdapter_FileExtension(t *testing.T) {
	adapter := NewPytestAdapter()
	if adapter.FileExtension() != ".py" {
		t.Errorf("FileExtension() = %s, want .py", adapter.FileExtension())
	}
}

func TestPytestAdapter_TestFileSuffix(t *testing.T) {
	adapter := NewPytestAdapter()
	if adapter.TestFileSuffix() != "_test" {
		t.Errorf("TestFileSuffix() = %s, want _test", adapter.TestFileSuffix())
	}
}

func TestPytestAdapter_Generate_SimpleTest(t *testing.T) {
	adapter := NewPytestAdapter()
	testDSL := &dsl.TestDSL{
		Name: "Test_Add",
		Target: dsl.TestTarget{
			File:     "math.py",
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

	if !strings.Contains(code, "import pytest") {
		t.Error("code should import pytest")
	}
	if !strings.Contains(code, "def test_") {
		t.Error("code should contain test function")
	}
	if !strings.Contains(code, "add(2, 3)") {
		t.Error("code should contain function call")
	}
	if !strings.Contains(code, "assert result == 5") {
		t.Error("code should contain assertion")
	}
}

func TestPytestAdapter_Generate_WithMarkers(t *testing.T) {
	adapter := NewPytestAdapter()

	tests := []struct {
		name     string
		testType dsl.TestType
		marker   string
	}{
		{"integration", dsl.TestTypeIntegration, "integration"},
		{"e2e", dsl.TestTypeE2E, "e2e"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDSL := &dsl.TestDSL{
				Name: "Test",
				Type: tt.testType,
				Target: dsl.TestTarget{
					Function: "func",
				},
				Steps: []dsl.TestStep{},
			}

			code, err := adapter.Generate(testDSL)
			if err != nil {
				t.Fatalf("Generate() error: %v", err)
			}

			if !strings.Contains(code, "@pytest.mark."+tt.marker) {
				t.Errorf("code should contain @pytest.mark.%s", tt.marker)
			}
		})
	}
}

func TestPytestAdapter_Generate_AsyncTest(t *testing.T) {
	adapter := NewPytestAdapter()
	testDSL := &dsl.TestDSL{
		Name: "Test_API",
		Target: dsl.TestTarget{
			Function: "fetch_data",
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

	if !strings.Contains(code, "@pytest.mark.asyncio") {
		t.Error("code should contain asyncio marker")
	}
	if !strings.Contains(code, "async def test_") {
		t.Error("code should contain async function")
	}
}

func TestToPythonFunctionName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"simple", "simple"},
		{"CamelCase", "camelcase"},
		{"with-hyphen", "with_hyphen"},
		{"with space", "with_space"},
		{"with.dot", "with_dot"},
		{"Mixed-Case Name", "mixed_case_name"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toPythonFunctionName(tt.input)
			if result != tt.expected {
				t.Errorf("toPythonFunctionName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatPythonArgs(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatPythonArgs(tt.args)
			if result != tt.expected {
				t.Errorf("formatPythonArgs() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGeneratePytestAssertions(t *testing.T) {
	t.Run("value assertion int", func(t *testing.T) {
		expected := &dsl.Expected{Value: 42}
		result := generatePytestAssertions(expected)
		if !strings.Contains(result, "assert result == 42") {
			t.Errorf("assertion should check value, got %q", result)
		}
	})

	t.Run("value assertion string", func(t *testing.T) {
		expected := &dsl.Expected{Value: "hello"}
		result := generatePytestAssertions(expected)
		if !strings.Contains(result, "assert result == 'hello'") {
			t.Errorf("assertion should check string value, got %q", result)
		}
	})

	t.Run("type assertion", func(t *testing.T) {
		expected := &dsl.Expected{Type: "int"}
		result := generatePytestAssertions(expected)
		if !strings.Contains(result, "isinstance(result, int)") {
			t.Errorf("assertion should check type, got %q", result)
		}
	})

	t.Run("contains assertion", func(t *testing.T) {
		expected := &dsl.Expected{Contains: "hello"}
		result := generatePytestAssertions(expected)
		if !strings.Contains(result, "'hello' in result") {
			t.Errorf("assertion should check contains, got %q", result)
		}
	})

	t.Run("error assertion with type", func(t *testing.T) {
		expected := &dsl.Expected{Error: &dsl.ExpectedError{Type: "ValueError"}}
		result := generatePytestAssertions(expected)
		if !strings.Contains(result, "pytest.raises(ValueError)") {
			t.Errorf("assertion should check error type, got %q", result)
		}
	})

	t.Run("error assertion without type", func(t *testing.T) {
		expected := &dsl.Expected{Error: &dsl.ExpectedError{}}
		result := generatePytestAssertions(expected)
		if !strings.Contains(result, "pytest.raises(Exception)") {
			t.Errorf("assertion should check general exception, got %q", result)
		}
	})

	t.Run("empty expected", func(t *testing.T) {
		// Empty expected (all nil fields) should return empty string
		expected := &dsl.Expected{}
		result := generatePytestAssertions(expected)
		if result != "" {
			t.Errorf("empty expected should return empty string, got %q", result)
		}
	})
}

func TestHasAsyncPythonSteps(t *testing.T) {
	t.Run("no async steps", func(t *testing.T) {
		testDSL := &dsl.TestDSL{
			Steps: []dsl.TestStep{
				{Action: dsl.StepAction{Type: dsl.ActionCall}},
			},
		}
		if hasAsyncPythonSteps(testDSL) {
			t.Error("should return false for call action")
		}
	})

	t.Run("http step", func(t *testing.T) {
		testDSL := &dsl.TestDSL{
			Steps: []dsl.TestStep{
				{Action: dsl.StepAction{Type: dsl.ActionHTTP}},
			},
		}
		if !hasAsyncPythonSteps(testDSL) {
			t.Error("should return true for HTTP action")
		}
	})

	t.Run("wait step", func(t *testing.T) {
		testDSL := &dsl.TestDSL{
			Steps: []dsl.TestStep{
				{Action: dsl.StepAction{Type: dsl.ActionWait}},
			},
		}
		if !hasAsyncPythonSteps(testDSL) {
			t.Error("should return true for wait action")
		}
	})
}

func TestResourceToFixture(t *testing.T) {
	t.Run("database resource", func(t *testing.T) {
		resource := dsl.Resource{Type: dsl.ResourceDatabase, Name: "db"}
		fixture := resourceToFixture(resource)

		if fixture.Name != "database_db" {
			t.Errorf("Name = %s, want database_db", fixture.Name)
		}
		if fixture.YieldValue != "db" {
			t.Errorf("YieldValue = %s, want db", fixture.YieldValue)
		}
	})

	t.Run("cache resource", func(t *testing.T) {
		resource := dsl.Resource{Type: dsl.ResourceCache, Name: "redis"}
		fixture := resourceToFixture(resource)

		if fixture.Name != "cache_redis" {
			t.Errorf("Name = %s, want cache_redis", fixture.Name)
		}
	})
}

func TestGeneratePytestStepCode(t *testing.T) {
	t.Run("call action with expected", func(t *testing.T) {
		step := dsl.TestStep{
			Action: dsl.StepAction{
				Type:   dsl.ActionCall,
				Target: "add",
				Args:   []interface{}{1, 2},
			},
			Expected: &dsl.Expected{Value: 3},
		}
		code := generatePytestStepCode(step)
		if !strings.Contains(code, "result = add(1, 2)") {
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
		code := generatePytestStepCode(step)
		if strings.Contains(code, "result =") {
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
		code := generatePytestStepCode(step)
		if !strings.Contains(code, "await client.post") {
			t.Errorf("code should use async client, got %q", code)
		}
	})

	t.Run("http action default method", func(t *testing.T) {
		step := dsl.TestStep{
			Action: dsl.StepAction{
				Type:   dsl.ActionHTTP,
				Target: "/api/data",
			},
		}
		code := generatePytestStepCode(step)
		if !strings.Contains(code, "client.get") {
			t.Errorf("code should default to GET, got %q", code)
		}
	})
}
