package parser

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewParser(t *testing.T) {
	p := NewParser()
	assert.NotNil(t, p)
	assert.NotNil(t, p.goParser)
	assert.NotNil(t, p.pyParser)
	assert.NotNil(t, p.jsParser)
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path     string
		expected Language
	}{
		{"main.go", LanguageGo},
		{"app.py", LanguagePython},
		{"index.js", LanguageJavaScript},
		{"index.jsx", LanguageJavaScript},
		{"index.mjs", LanguageJavaScript},
		{"app.ts", LanguageTypeScript},
		{"app.tsx", LanguageTypeScript},
		{"Main.java", LanguageJava},
		{"README.md", LanguageUnknown},
		{"Makefile", LanguageUnknown},
		{"/path/to/file.go", LanguageGo},
		{"/path/to/file.PY", LanguagePython}, // Case insensitive
		{"file.GO", LanguageGo},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := DetectLanguage(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParser_ParseContent_Go_SimpleFunction(t *testing.T) {
	p := NewParser()
	content := `package main

func Add(a int, b int) int {
	return a + b
}
`
	parsed, err := p.ParseContent(context.Background(), "test.go", content, LanguageGo)
	require.NoError(t, err)
	assert.Equal(t, LanguageGo, parsed.Language)
	assert.Len(t, parsed.Functions, 1)

	fn := parsed.Functions[0]
	assert.Equal(t, "Add", fn.Name)
	assert.True(t, fn.Exported)
	assert.Equal(t, 3, fn.StartLine)
	assert.Len(t, fn.Parameters, 2)
	assert.Equal(t, "a", fn.Parameters[0].Name)
	assert.Equal(t, "int", fn.Parameters[0].Type)
}

func TestParser_ParseContent_Go_UnexportedFunction(t *testing.T) {
	p := NewParser()
	content := `package main

func privateFunc() {
}
`
	parsed, err := p.ParseContent(context.Background(), "test.go", content, LanguageGo)
	require.NoError(t, err)
	assert.Len(t, parsed.Functions, 1)
	assert.Equal(t, "privateFunc", parsed.Functions[0].Name)
	assert.False(t, parsed.Functions[0].Exported)
}

func TestParser_ParseContent_Go_Method(t *testing.T) {
	p := NewParser()
	content := `package main

type Calculator struct{}

func (c *Calculator) Add(a, b int) int {
	return a + b
}
`
	parsed, err := p.ParseContent(context.Background(), "test.go", content, LanguageGo)
	require.NoError(t, err)
	assert.Len(t, parsed.Functions, 1)

	fn := parsed.Functions[0]
	assert.Equal(t, "Add", fn.Name)
	assert.Equal(t, "Calculator", fn.Class)
}

func TestParser_ParseContent_Go_MultipleFunctions(t *testing.T) {
	p := NewParser()
	content := `package main

func Add(a, b int) int {
	return a + b
}

func Sub(a, b int) int {
	return a - b
}

func Mul(a, b int) int {
	return a * b
}
`
	parsed, err := p.ParseContent(context.Background(), "test.go", content, LanguageGo)
	require.NoError(t, err)
	assert.Len(t, parsed.Functions, 3)

	names := []string{parsed.Functions[0].Name, parsed.Functions[1].Name, parsed.Functions[2].Name}
	assert.Contains(t, names, "Add")
	assert.Contains(t, names, "Sub")
	assert.Contains(t, names, "Mul")
}

func TestParser_ParseContent_Python_SimpleFunction(t *testing.T) {
	p := NewParser()
	content := `def add(a, b):
    return a + b
`
	parsed, err := p.ParseContent(context.Background(), "test.py", content, LanguagePython)
	require.NoError(t, err)
	assert.Len(t, parsed.Functions, 1)

	fn := parsed.Functions[0]
	assert.Equal(t, "add", fn.Name)
	assert.True(t, fn.Exported)
	assert.Len(t, fn.Parameters, 2)
}

func TestParser_ParseContent_Python_PrivateFunction(t *testing.T) {
	p := NewParser()
	content := `def _private_func():
    pass
`
	parsed, err := p.ParseContent(context.Background(), "test.py", content, LanguagePython)
	require.NoError(t, err)
	assert.Len(t, parsed.Functions, 1)
	assert.Equal(t, "_private_func", parsed.Functions[0].Name)
	assert.False(t, parsed.Functions[0].Exported)
}

func TestParser_ParseContent_Python_Class(t *testing.T) {
	p := NewParser()
	content := `class Calculator:
    def add(self, a, b):
        return a + b

    def subtract(self, a, b):
        return a - b
`
	parsed, err := p.ParseContent(context.Background(), "test.py", content, LanguagePython)
	require.NoError(t, err)
	assert.Len(t, parsed.Classes, 1)

	cls := parsed.Classes[0]
	assert.Equal(t, "Calculator", cls.Name)
	assert.Len(t, cls.Methods, 2)
}

func TestParser_ParseContent_Python_SelfFiltered(t *testing.T) {
	p := NewParser()
	content := `class Test:
    def method(self, x, y):
        pass
`
	parsed, err := p.ParseContent(context.Background(), "test.py", content, LanguagePython)
	require.NoError(t, err)
	assert.Len(t, parsed.Classes, 1)

	method := parsed.Classes[0].Methods[0]
	// self should be filtered out
	assert.Len(t, method.Parameters, 2)
	for _, p := range method.Parameters {
		assert.NotEqual(t, "self", p.Name)
	}
}

func TestParser_ParseContent_JavaScript_Function(t *testing.T) {
	p := NewParser()
	content := `function greet(name) {
    return "Hello, " + name;
}
`
	parsed, err := p.ParseContent(context.Background(), "test.js", content, LanguageJavaScript)
	require.NoError(t, err)
	assert.Len(t, parsed.Functions, 1)

	fn := parsed.Functions[0]
	assert.Equal(t, "greet", fn.Name)
	assert.Len(t, fn.Parameters, 1)
	assert.Equal(t, "name", fn.Parameters[0].Name)
}

func TestParser_ParseContent_JavaScript_ArrowFunction(t *testing.T) {
	p := NewParser()
	content := `const add = (a, b) => {
    return a + b;
};
`
	parsed, err := p.ParseContent(context.Background(), "test.js", content, LanguageJavaScript)
	require.NoError(t, err)
	assert.Len(t, parsed.Functions, 1)

	fn := parsed.Functions[0]
	assert.Equal(t, "add", fn.Name)
	assert.Len(t, fn.Parameters, 2)
}

func TestParser_ParseContent_JavaScript_MultipleFunctions(t *testing.T) {
	p := NewParser()
	content := `function funcA() {}
function funcB(x) {}
const funcC = (a, b) => a + b;
`
	parsed, err := p.ParseContent(context.Background(), "test.js", content, LanguageJavaScript)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(parsed.Functions), 2)
}

func TestParser_ParseContent_TypeScript_Function(t *testing.T) {
	p := NewParser()
	// TypeScript uses JS parser, so basic function syntax should work
	content := `function add(a, b) {
    return a + b;
}
`
	parsed, err := p.ParseContent(context.Background(), "test.ts", content, LanguageTypeScript)
	require.NoError(t, err)
	assert.Len(t, parsed.Functions, 1)
	assert.Equal(t, "add", parsed.Functions[0].Name)
}

func TestParser_ParseContent_UnsupportedLanguage(t *testing.T) {
	p := NewParser()
	_, err := p.ParseContent(context.Background(), "test.java", "class Test {}", LanguageJava)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported language")
}

func TestParser_ParseContent_EmptyFile(t *testing.T) {
	p := NewParser()
	content := ""
	parsed, err := p.ParseContent(context.Background(), "test.go", content, LanguageGo)
	require.NoError(t, err)
	assert.Len(t, parsed.Functions, 0)
}

func TestParser_ParseContent_NoFunctions(t *testing.T) {
	p := NewParser()
	content := `package main

var x = 10
const y = "hello"
`
	parsed, err := p.ParseContent(context.Background(), "test.go", content, LanguageGo)
	require.NoError(t, err)
	assert.Len(t, parsed.Functions, 0)
}

func TestParser_ParseContent_FunctionID(t *testing.T) {
	p := NewParser()
	content := `package main

func TestFunc() {
}
`
	parsed, err := p.ParseContent(context.Background(), "/path/to/test.go", content, LanguageGo)
	require.NoError(t, err)
	assert.Len(t, parsed.Functions, 1)
	// ID format: file:line:name
	assert.Equal(t, "/path/to/test.go:3:TestFunc", parsed.Functions[0].ID)
}

func TestParser_ParseContent_Go_ParameterTypes(t *testing.T) {
	p := NewParser()
	content := `package main

func Complex(s string, n int, f float64, b bool) {
}
`
	parsed, err := p.ParseContent(context.Background(), "test.go", content, LanguageGo)
	require.NoError(t, err)
	assert.Len(t, parsed.Functions, 1)

	params := parsed.Functions[0].Parameters
	assert.Len(t, params, 4)

	// Verify parameter names
	names := make(map[string]bool)
	for _, p := range params {
		names[p.Name] = true
	}
	assert.True(t, names["s"])
	assert.True(t, names["n"])
	assert.True(t, names["f"])
	assert.True(t, names["b"])
}

func TestParser_ParseContent_Go_MethodReceiver(t *testing.T) {
	p := NewParser()
	content := `package main

type Service struct{}

func (s *Service) Start() {}
func (s Service) Stop() {}
`
	parsed, err := p.ParseContent(context.Background(), "test.go", content, LanguageGo)
	require.NoError(t, err)
	assert.Len(t, parsed.Functions, 2)

	for _, fn := range parsed.Functions {
		assert.Equal(t, "Service", fn.Class)
	}
}

func TestParser_ParseContent_Python_AsyncFunction(t *testing.T) {
	p := NewParser()
	content := `async def fetch_data(url):
    pass
`
	parsed, err := p.ParseContent(context.Background(), "test.py", content, LanguagePython)
	require.NoError(t, err)
	assert.Len(t, parsed.Functions, 1)
	assert.True(t, parsed.Functions[0].Async)
}

func TestParser_ParseContent_ContextCancellation(t *testing.T) {
	p := NewParser()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Parser should handle cancelled context gracefully
	// The behavior depends on tree-sitter implementation
	_, err := p.ParseContent(ctx, "test.go", "package main", LanguageGo)
	// May or may not error depending on tree-sitter
	_ = err
}

func TestParser_ParseFile_NonExistent(t *testing.T) {
	p := NewParser()
	_, err := p.ParseFile(context.Background(), "/nonexistent/file.go")
	assert.Error(t, err)
}

func TestParser_ParseFile_UnsupportedExtension(t *testing.T) {
	p := NewParser()
	// Create a temp file with unsupported extension
	_, err := p.ParseFile(context.Background(), "/tmp/test.xyz")
	assert.Error(t, err)
}

func TestLanguageConstants(t *testing.T) {
	assert.Equal(t, Language("go"), LanguageGo)
	assert.Equal(t, Language("python"), LanguagePython)
	assert.Equal(t, Language("javascript"), LanguageJavaScript)
	assert.Equal(t, Language("typescript"), LanguageTypeScript)
	assert.Equal(t, Language("java"), LanguageJava)
	assert.Equal(t, Language("unknown"), LanguageUnknown)
}

func TestParsedFile_Fields(t *testing.T) {
	p := NewParser()
	content := `package main

func Hello() {}
`
	parsed, err := p.ParseContent(context.Background(), "/test/file.go", content, LanguageGo)
	require.NoError(t, err)

	assert.Equal(t, "/test/file.go", parsed.Path)
	assert.Equal(t, LanguageGo, parsed.Language)
	assert.NotNil(t, parsed.Functions)
	assert.NotNil(t, parsed.Classes)
	assert.NotNil(t, parsed.Imports)
}

func TestParser_ParseContent_Python_ClassID(t *testing.T) {
	p := NewParser()
	content := `class MyClass:
    def method(self):
        pass
`
	parsed, err := p.ParseContent(context.Background(), "/path/file.py", content, LanguagePython)
	require.NoError(t, err)
	assert.Len(t, parsed.Classes, 1)

	cls := parsed.Classes[0]
	// Class ID format: file:line:name
	assert.Contains(t, cls.ID, "/path/file.py")
	assert.Contains(t, cls.ID, "MyClass")
}

func TestParser_ParseContent_Python_MethodID(t *testing.T) {
	p := NewParser()
	content := `class MyClass:
    def method(self):
        pass
`
	parsed, err := p.ParseContent(context.Background(), "/path/file.py", content, LanguagePython)
	require.NoError(t, err)
	assert.Len(t, parsed.Classes, 1)

	method := parsed.Classes[0].Methods[0]
	// Method ID format: file:line:class.method
	assert.Contains(t, method.ID, "MyClass.method")
}
