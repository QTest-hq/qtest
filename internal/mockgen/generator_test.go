package mockgen

import (
	"strings"
	"testing"

	"github.com/QTest-hq/qtest/pkg/model"
)

func TestNewGenerator(t *testing.T) {
	gen := NewGenerator("mypackage")
	if gen.packageName != "mypackage" {
		t.Errorf("expected packageName 'mypackage', got '%s'", gen.packageName)
	}
}

func TestGenerateFromInterface_Simple(t *testing.T) {
	gen := NewGenerator("test")
	iface := &Interface{
		Name: "UserService",
		Methods: []Method{
			{
				Name:       "GetUser",
				Parameters: []Parameter{{Name: "id", Type: "string"}},
				Returns:    []Parameter{{Type: "*User"}, {Type: "error"}},
			},
			{
				Name:       "CreateUser",
				Parameters: []Parameter{{Name: "user", Type: "*User"}},
				Returns:    []Parameter{{Type: "error"}},
			},
		},
	}

	config := &MockConfig{PackageName: "test_mock"}
	result, err := gen.GenerateFromInterface(iface, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the output contains expected elements
	expectedStrings := []string{
		"package test_mock",
		"type MockUserService struct",
		"getUserFunc func(id string) (*User, error)",
		"createUserFunc func(user *User) error",
		"func NewMockUserService() *MockUserService",
		"func (m *MockUserService) GetUser(id string) (*User, error)",
		"func (m *MockUserService) CreateUser(user *User) error",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(result, expected) {
			t.Errorf("expected output to contain %q, got:\n%s", expected, result)
		}
	}
}

func TestGenerateFromInterface_WithRecording(t *testing.T) {
	gen := NewGenerator("test")
	iface := &Interface{
		Name: "Logger",
		Methods: []Method{
			{
				Name:       "Log",
				Parameters: []Parameter{{Name: "msg", Type: "string"}},
				Returns:    nil,
			},
		},
	}

	config := &MockConfig{
		PackageName:   "logger_mock",
		WithRecording: true,
	}
	result, err := gen.GenerateFromInterface(iface, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify recording functionality
	expectedStrings := []string{
		"calls struct",
		"LogCalls()",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(result, expected) {
			t.Errorf("expected output to contain %q, got:\n%s", expected, result)
		}
	}
}

func TestGenerateFromModel(t *testing.T) {
	gen := NewGenerator("test")

	m := &model.SystemModel{
		Types: []model.TypeDef{
			{
				ID:      "reader",
				Name:    "Reader",
				Kind:    model.TypeKindInterface,
				Methods: []string{"read_method"},
			},
			{
				ID:   "data",
				Name: "Data",
				Kind: model.TypeKindStruct, // Should be skipped
			},
		},
		Functions: []model.Function{
			{
				ID:         "read_method",
				Name:       "Read",
				Parameters: []model.Parameter{{Name: "p", Type: "[]byte"}},
				Returns:    []model.Parameter{{Type: "int"}, {Type: "error"}},
			},
		},
	}

	config := &MockConfig{PackageName: "reader_mock"}
	result, err := gen.GenerateFromModel(m, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify only interface is mocked
	if !strings.Contains(result, "MockReader") {
		t.Error("expected MockReader in output")
	}
	if strings.Contains(result, "MockData") {
		t.Error("Data struct should not be mocked")
	}
}

func TestGenerateFromModel_FilterInterfaces(t *testing.T) {
	gen := NewGenerator("test")

	m := &model.SystemModel{
		Types: []model.TypeDef{
			{
				ID:      "reader",
				Name:    "Reader",
				Kind:    model.TypeKindInterface,
				Methods: []string{},
			},
			{
				ID:      "writer",
				Name:    "Writer",
				Kind:    model.TypeKindInterface,
				Methods: []string{},
			},
		},
	}

	config := &MockConfig{
		PackageName: "io_mock",
		Interfaces:  []string{"Reader"}, // Only mock Reader
	}
	result, err := gen.GenerateFromModel(m, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "MockReader") {
		t.Error("expected MockReader in output")
	}
	if strings.Contains(result, "MockWriter") {
		t.Error("Writer should not be mocked due to filter")
	}
}

func TestGenerateFromModel_NoInterfaces(t *testing.T) {
	gen := NewGenerator("test")

	m := &model.SystemModel{
		Types: []model.TypeDef{
			{Name: "Data", Kind: model.TypeKindStruct},
		},
	}

	config := &MockConfig{}
	_, err := gen.GenerateFromModel(m, config)
	if err == nil {
		t.Error("expected error when no interfaces found")
	}
	if !strings.Contains(err.Error(), "no interfaces found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestUnexport(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"GetUser", "getUser"},
		{"ID", "iD"},
		{"get", "get"},
		{"", ""},
		{"A", "a"},
	}

	for _, tc := range tests {
		result := unexport(tc.input)
		if result != tc.expected {
			t.Errorf("unexport(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestFormatParams(t *testing.T) {
	tests := []struct {
		params   []Parameter
		expected string
	}{
		{nil, ""},
		{[]Parameter{}, ""},
		{[]Parameter{{Name: "id", Type: "string"}}, "id string"},
		{
			[]Parameter{
				{Name: "id", Type: "string"},
				{Name: "name", Type: "string"},
			},
			"id string, name string",
		},
		{[]Parameter{{Type: "string"}}, "string"}, // Anonymous param
	}

	for _, tc := range tests {
		result := formatParams(tc.params)
		if result != tc.expected {
			t.Errorf("formatParams(%v) = %q, want %q", tc.params, result, tc.expected)
		}
	}
}

func TestFormatReturn(t *testing.T) {
	tests := []struct {
		returns  []Parameter
		expected string
	}{
		{nil, ""},
		{[]Parameter{}, ""},
		{[]Parameter{{Type: "error"}}, "error"},
		{
			[]Parameter{{Type: "int"}, {Type: "error"}},
			"(int, error)",
		},
	}

	for _, tc := range tests {
		result := formatReturn(tc.returns)
		if result != tc.expected {
			t.Errorf("formatReturn(%v) = %q, want %q", tc.returns, result, tc.expected)
		}
	}
}

func TestZeroValue(t *testing.T) {
	tests := []struct {
		typeName string
		expected string
	}{
		{"int", "0"},
		{"int64", "0"},
		{"float64", "0"},
		{"bool", "false"},
		{"string", `""`},
		{"error", "nil"},
		{"*User", "nil"},
		{"[]byte", "nil"},
		{"map[string]int", "nil"},
		{"SomeStruct", "nil"},
	}

	for _, tc := range tests {
		result := zeroValue(tc.typeName)
		if result != tc.expected {
			t.Errorf("zeroValue(%q) = %q, want %q", tc.typeName, result, tc.expected)
		}
	}
}

func TestParamNames(t *testing.T) {
	tests := []struct {
		params   []Parameter
		expected string
	}{
		{nil, ""},
		{[]Parameter{}, ""},
		{[]Parameter{{Name: "id", Type: "string"}}, "id"},
		{
			[]Parameter{
				{Name: "id", Type: "string"},
				{Name: "name", Type: "string"},
			},
			"id, name",
		},
		{[]Parameter{{Type: "string"}}, "arg0"}, // Anonymous gets default name
		{
			[]Parameter{
				{Name: "a", Type: "int"},
				{Type: "string"}, // Anonymous
			},
			"a, arg1",
		},
	}

	for _, tc := range tests {
		result := paramNames(tc.params)
		if result != tc.expected {
			t.Errorf("paramNames(%v) = %q, want %q", tc.params, result, tc.expected)
		}
	}
}

func TestGenerateFromInterface_NilConfig(t *testing.T) {
	gen := NewGenerator("test")
	iface := &Interface{
		Name: "Simple",
		Methods: []Method{
			{Name: "Do"},
		},
	}

	result, err := gen.GenerateFromInterface(iface, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should use default package name
	if !strings.Contains(result, "package test_mock") {
		t.Error("expected default package name 'test_mock'")
	}
}
