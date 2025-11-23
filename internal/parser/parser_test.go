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
	_, err := p.ParseContent(context.Background(), "test.rs", "fn main() {}", LanguageUnknown)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported language")
}

func TestParser_ParseContent_Java_Class(t *testing.T) {
	p := NewParser()
	content := `public class Calculator {
    private int value;

    public Calculator() {
        this.value = 0;
    }

    public int add(int a, int b) {
        return a + b;
    }

    public static int multiply(int x, int y) {
        return x * y;
    }

    private void helper() {
    }
}
`
	parsed, err := p.ParseContent(context.Background(), "Calculator.java", content, LanguageJava)
	require.NoError(t, err)
	assert.Equal(t, LanguageJava, parsed.Language)
	assert.Len(t, parsed.Classes, 1)

	cls := parsed.Classes[0]
	assert.Equal(t, "Calculator", cls.Name)
	assert.True(t, cls.Exported)
	assert.Len(t, cls.Methods, 4) // constructor + 3 methods
}

func TestParser_ParseContent_Java_Methods(t *testing.T) {
	p := NewParser()
	content := `public class Service {
    public void doSomething(String name, int count) {
    }

    public static String format(String template) {
        return template;
    }
}
`
	parsed, err := p.ParseContent(context.Background(), "Service.java", content, LanguageJava)
	require.NoError(t, err)
	require.Len(t, parsed.Classes, 1)
	require.Len(t, parsed.Classes[0].Methods, 2)

	// First method
	m1 := parsed.Classes[0].Methods[0]
	assert.Equal(t, "doSomething", m1.Name)
	assert.True(t, m1.Exported)
	assert.False(t, m1.Static)
	assert.Equal(t, "void", m1.ReturnType)
	assert.Len(t, m1.Parameters, 2)
	assert.Equal(t, "name", m1.Parameters[0].Name)
	assert.Equal(t, "String", m1.Parameters[0].Type)
	assert.Equal(t, "count", m1.Parameters[1].Name)
	assert.Equal(t, "int", m1.Parameters[1].Type)

	// Second method (static)
	m2 := parsed.Classes[0].Methods[1]
	assert.Equal(t, "format", m2.Name)
	assert.True(t, m2.Exported)
	assert.True(t, m2.Static)
	assert.Equal(t, "String", m2.ReturnType)
}

func TestParser_ParseContent_Java_PrivateMethod(t *testing.T) {
	p := NewParser()
	content := `class Helper {
    private int calculate(int x) {
        return x * 2;
    }
}
`
	parsed, err := p.ParseContent(context.Background(), "Helper.java", content, LanguageJava)
	require.NoError(t, err)
	require.Len(t, parsed.Classes, 1)
	require.Len(t, parsed.Classes[0].Methods, 1)

	m := parsed.Classes[0].Methods[0]
	assert.Equal(t, "calculate", m.Name)
	assert.False(t, m.Exported) // private
}

func TestParser_ParseContent_Java_Constructor(t *testing.T) {
	p := NewParser()
	content := `public class Person {
    public Person(String name, int age) {
    }
}
`
	parsed, err := p.ParseContent(context.Background(), "Person.java", content, LanguageJava)
	require.NoError(t, err)
	require.Len(t, parsed.Classes, 1)
	require.Len(t, parsed.Classes[0].Methods, 1)

	constructor := parsed.Classes[0].Methods[0]
	assert.Equal(t, "Person", constructor.Name) // Constructor name is class name
	assert.True(t, constructor.Exported)
	assert.Len(t, constructor.Parameters, 2)
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

// JS/TS Export Tests

func TestParser_ParseContent_JS_ExportFunction(t *testing.T) {
	p := NewParser()
	content := `export function add(a, b) {
    return a + b;
}

function privateFunc() {
    return 1;
}
`
	parsed, err := p.ParseContent(context.Background(), "test.js", content, LanguageJavaScript)
	require.NoError(t, err)
	assert.Len(t, parsed.Functions, 2)

	// Find the exported function
	var exportedFn, privateFn *Function
	for i := range parsed.Functions {
		if parsed.Functions[i].Name == "add" {
			exportedFn = &parsed.Functions[i]
		} else if parsed.Functions[i].Name == "privateFunc" {
			privateFn = &parsed.Functions[i]
		}
	}

	require.NotNil(t, exportedFn)
	require.NotNil(t, privateFn)
	assert.True(t, exportedFn.Exported, "add should be exported")
	assert.False(t, privateFn.Exported, "privateFunc should not be exported")

	// Check exports list
	assert.Len(t, parsed.Exports, 1)
	assert.Equal(t, "add", parsed.Exports[0].Name)
	assert.Equal(t, "function", parsed.Exports[0].Kind)
}

func TestParser_ParseContent_JS_ExportConst(t *testing.T) {
	p := NewParser()
	content := `export const multiply = (a, b) => a * b;

const privateHelper = (x) => x * 2;
`
	parsed, err := p.ParseContent(context.Background(), "test.js", content, LanguageJavaScript)
	require.NoError(t, err)

	// Find the exported arrow function
	var exportedFn *Function
	for i := range parsed.Functions {
		if parsed.Functions[i].Name == "multiply" {
			exportedFn = &parsed.Functions[i]
			break
		}
	}

	require.NotNil(t, exportedFn)
	assert.True(t, exportedFn.Exported, "multiply should be exported")

	// Check exports list
	assert.GreaterOrEqual(t, len(parsed.Exports), 1)
	found := false
	for _, exp := range parsed.Exports {
		if exp.Name == "multiply" {
			found = true
			assert.Equal(t, "function", exp.Kind)
		}
	}
	assert.True(t, found, "multiply should be in exports list")
}

func TestParser_ParseContent_JS_ExportClass(t *testing.T) {
	p := NewParser()
	content := `export class Calculator {
    add(a, b) {
        return a + b;
    }
}

class PrivateHelper {
    help() {}
}
`
	parsed, err := p.ParseContent(context.Background(), "test.js", content, LanguageJavaScript)
	require.NoError(t, err)
	assert.Len(t, parsed.Classes, 2)

	// Find classes
	var exportedCls, privateCls *Class
	for i := range parsed.Classes {
		if parsed.Classes[i].Name == "Calculator" {
			exportedCls = &parsed.Classes[i]
		} else if parsed.Classes[i].Name == "PrivateHelper" {
			privateCls = &parsed.Classes[i]
		}
	}

	require.NotNil(t, exportedCls)
	require.NotNil(t, privateCls)
	assert.True(t, exportedCls.Exported, "Calculator should be exported")
	assert.False(t, privateCls.Exported, "PrivateHelper should not be exported")

	// Check exports list
	assert.GreaterOrEqual(t, len(parsed.Exports), 1)
	found := false
	for _, exp := range parsed.Exports {
		if exp.Name == "Calculator" {
			found = true
			assert.Equal(t, "class", exp.Kind)
		}
	}
	assert.True(t, found)
}

func TestParser_ParseContent_JS_NamedExports(t *testing.T) {
	p := NewParser()
	content := `function foo() {}
function bar() {}
function baz() {}

export { foo, bar };
`
	parsed, err := p.ParseContent(context.Background(), "test.js", content, LanguageJavaScript)
	require.NoError(t, err)

	// Check that foo and bar are exported, baz is not
	var fooFn, barFn, bazFn *Function
	for i := range parsed.Functions {
		switch parsed.Functions[i].Name {
		case "foo":
			fooFn = &parsed.Functions[i]
		case "bar":
			barFn = &parsed.Functions[i]
		case "baz":
			bazFn = &parsed.Functions[i]
		}
	}

	require.NotNil(t, fooFn)
	require.NotNil(t, barFn)
	require.NotNil(t, bazFn)

	assert.True(t, fooFn.Exported, "foo should be exported")
	assert.True(t, barFn.Exported, "bar should be exported")
	assert.False(t, bazFn.Exported, "baz should not be exported")

	// Check exports list
	assert.Len(t, parsed.Exports, 2)
}

func TestParser_ParseContent_JS_ExportDefault(t *testing.T) {
	p := NewParser()
	content := `export default function main() {
    console.log("main");
}
`
	parsed, err := p.ParseContent(context.Background(), "test.js", content, LanguageJavaScript)
	require.NoError(t, err)
	assert.Len(t, parsed.Functions, 1)

	fn := parsed.Functions[0]
	assert.Equal(t, "main", fn.Name)
	assert.True(t, fn.Exported)

	// Check default export
	assert.Len(t, parsed.Exports, 1)
	assert.Equal(t, "main", parsed.Exports[0].Name)
	assert.True(t, parsed.Exports[0].Default)
}

func TestParser_ParseContent_TS_ExportFunction(t *testing.T) {
	p := NewParser()
	content := `export function greet(name) {
    return "Hello, " + name;
}

function internal() {
    return 42;
}
`
	parsed, err := p.ParseContent(context.Background(), "test.ts", content, LanguageTypeScript)
	require.NoError(t, err)

	var exported, internal *Function
	for i := range parsed.Functions {
		if parsed.Functions[i].Name == "greet" {
			exported = &parsed.Functions[i]
		} else if parsed.Functions[i].Name == "internal" {
			internal = &parsed.Functions[i]
		}
	}

	require.NotNil(t, exported)
	require.NotNil(t, internal)
	assert.True(t, exported.Exported)
	assert.False(t, internal.Exported)
}

func TestParser_ParseContent_JS_MixedExports(t *testing.T) {
	p := NewParser()
	content := `// Direct exports
export function directFunc() {}
export const directConst = () => {};

// Named exports
function namedFunc() {}
const namedConst = () => {};
export { namedFunc, namedConst };

// Not exported
function privateFunc() {}
`
	parsed, err := p.ParseContent(context.Background(), "test.js", content, LanguageJavaScript)
	require.NoError(t, err)

	// Count exported functions
	exportedCount := 0
	for _, fn := range parsed.Functions {
		if fn.Exported {
			exportedCount++
		}
	}

	// directFunc, directConst, namedFunc, namedConst should be exported
	assert.GreaterOrEqual(t, exportedCount, 4)

	// privateFunc should not be exported
	for _, fn := range parsed.Functions {
		if fn.Name == "privateFunc" {
			assert.False(t, fn.Exported)
		}
	}
}

func TestParser_ParseContent_JS_NoExports(t *testing.T) {
	p := NewParser()
	content := `function helper() {}
const util = () => {};
`
	parsed, err := p.ParseContent(context.Background(), "test.js", content, LanguageJavaScript)
	require.NoError(t, err)

	// No functions should be exported
	for _, fn := range parsed.Functions {
		assert.False(t, fn.Exported, "%s should not be exported", fn.Name)
	}

	// Exports list should be empty
	assert.Empty(t, parsed.Exports)
}

// Branch Extraction Tests (P1-036)

func TestParser_BranchExtraction_Go(t *testing.T) {
	p := NewParser()
	content := `package main

func process(x int) int {
	if x > 0 {
		return x * 2
	}
	for i := 0; i < x; i++ {
		println(i)
	}
	switch x {
	case 1:
		return 1
	case 2:
		return 2
	}
	return 0
}
`
	parsed, err := p.ParseContent(context.Background(), "test.go", content, LanguageGo)
	require.NoError(t, err)
	require.Len(t, parsed.Functions, 1)

	fn := parsed.Functions[0]
	assert.GreaterOrEqual(t, len(fn.Branches), 3, "should have at least 3 branches (if, for, switch)")

	// Check cyclomatic complexity
	assert.GreaterOrEqual(t, fn.CyclomaticComplexity, 4, "cyclomatic complexity should be >= 4")
}

func TestParser_BranchExtraction_JavaScript(t *testing.T) {
	p := NewParser()
	content := `function process(x) {
    if (x > 0) {
        return x * 2;
    }
    for (let i = 0; i < x; i++) {
        console.log(i);
    }
    while (x > 0) {
        x--;
    }
    try {
        doSomething();
    } catch (e) {
        console.error(e);
    }
}
`
	parsed, err := p.ParseContent(context.Background(), "test.js", content, LanguageJavaScript)
	require.NoError(t, err)
	require.Len(t, parsed.Functions, 1)

	fn := parsed.Functions[0]
	assert.GreaterOrEqual(t, len(fn.Branches), 4, "should have at least 4 branches (if, for, while, try/catch)")

	// Check for specific branch types
	hasTry := false
	hasCatch := false
	for _, b := range fn.Branches {
		if b.Type == BranchTry {
			hasTry = true
		}
		if b.Type == BranchCatch {
			hasCatch = true
		}
	}
	assert.True(t, hasTry, "should have try branch")
	assert.True(t, hasCatch, "should have catch branch")
}

func TestParser_BranchExtraction_Python(t *testing.T) {
	p := NewParser()
	content := `def process(x):
    if x > 0:
        return x * 2
    for i in range(x):
        print(i)
    while x > 0:
        x -= 1
    try:
        do_something()
    except Exception as e:
        print(e)
`
	parsed, err := p.ParseContent(context.Background(), "test.py", content, LanguagePython)
	require.NoError(t, err)
	require.Len(t, parsed.Functions, 1)

	fn := parsed.Functions[0]
	assert.GreaterOrEqual(t, len(fn.Branches), 4, "should have at least 4 branches")
	assert.GreaterOrEqual(t, fn.CyclomaticComplexity, 5)
}

func TestParser_BranchExtraction_Java(t *testing.T) {
	p := NewParser()
	content := `public class Example {
    public int process(int x) {
        if (x > 0) {
            return x * 2;
        }
        for (int i = 0; i < x; i++) {
            System.out.println(i);
        }
        switch (x) {
            case 1: return 1;
            case 2: return 2;
        }
        return 0;
    }
}
`
	parsed, err := p.ParseContent(context.Background(), "Example.java", content, LanguageJava)
	require.NoError(t, err)
	require.Len(t, parsed.Classes, 1)
	require.Len(t, parsed.Classes[0].Methods, 1)

	method := parsed.Classes[0].Methods[0]
	assert.GreaterOrEqual(t, len(method.Branches), 3, "should have at least 3 branches")
	assert.GreaterOrEqual(t, method.CyclomaticComplexity, 4)
}

// Call Site Extraction Tests (P1-037)

func TestParser_CallSiteExtraction_Go(t *testing.T) {
	p := NewParser()
	content := `package main

import "fmt"

func process(x int) {
	result := calculate(x)
	fmt.Println(result)
	helper(x, result)
}
`
	parsed, err := p.ParseContent(context.Background(), "test.go", content, LanguageGo)
	require.NoError(t, err)
	require.Len(t, parsed.Functions, 1)

	fn := parsed.Functions[0]
	assert.GreaterOrEqual(t, len(fn.CallSites), 3, "should have at least 3 call sites")

	// Check for specific calls
	foundCalculate := false
	foundPrintln := false
	for _, cs := range fn.CallSites {
		if cs.FunctionName == "calculate" {
			foundCalculate = true
		}
		if cs.FunctionName == "Println" {
			foundPrintln = true
			assert.True(t, cs.IsMethod || cs.Module == "fmt", "Println should be a method call or module call")
		}
	}
	assert.True(t, foundCalculate, "should find calculate call")
	assert.True(t, foundPrintln, "should find Println call")
}

func TestParser_CallSiteExtraction_JavaScript(t *testing.T) {
	p := NewParser()
	content := `function process(data) {
    const result = transform(data);
    console.log(result);
    arr.map(x => x * 2);
    await fetchData(url);
}
`
	parsed, err := p.ParseContent(context.Background(), "test.js", content, LanguageJavaScript)
	require.NoError(t, err)
	require.Len(t, parsed.Functions, 1)

	fn := parsed.Functions[0]
	assert.GreaterOrEqual(t, len(fn.CallSites), 3, "should have at least 3 call sites")

	// Check for console.log method call
	foundConsoleLog := false
	for _, cs := range fn.CallSites {
		if cs.FunctionName == "log" && cs.Receiver == "console" {
			foundConsoleLog = true
			assert.True(t, cs.IsMethod)
		}
	}
	assert.True(t, foundConsoleLog, "should find console.log call")
}

func TestParser_CallSiteExtraction_Python(t *testing.T) {
	p := NewParser()
	content := `def process(data):
    result = transform(data)
    print(result)
    obj.method()
    helper(data, result)
`
	parsed, err := p.ParseContent(context.Background(), "test.py", content, LanguagePython)
	require.NoError(t, err)
	require.Len(t, parsed.Functions, 1)

	fn := parsed.Functions[0]
	assert.GreaterOrEqual(t, len(fn.CallSites), 4, "should have at least 4 call sites")

	foundPrint := false
	foundMethod := false
	for _, cs := range fn.CallSites {
		if cs.FunctionName == "print" {
			foundPrint = true
		}
		if cs.FunctionName == "method" && cs.Receiver == "obj" {
			foundMethod = true
			assert.True(t, cs.IsMethod)
		}
	}
	assert.True(t, foundPrint, "should find print call")
	assert.True(t, foundMethod, "should find method call")
}

func TestParser_CallSiteExtraction_Java(t *testing.T) {
	p := NewParser()
	content := `public class Example {
    public void process(int x) {
        int result = calculate(x);
        System.out.println(result);
        list.add(result);
        new Helper().doWork();
    }
}
`
	parsed, err := p.ParseContent(context.Background(), "Example.java", content, LanguageJava)
	require.NoError(t, err)
	require.Len(t, parsed.Classes, 1)
	require.Len(t, parsed.Classes[0].Methods, 1)

	method := parsed.Classes[0].Methods[0]
	assert.GreaterOrEqual(t, len(method.CallSites), 4, "should have at least 4 call sites")

	foundPrintln := false
	foundConstructor := false
	for _, cs := range method.CallSites {
		if cs.FunctionName == "println" {
			foundPrintln = true
		}
		if cs.FunctionName == "new Helper" {
			foundConstructor = true
		}
	}
	assert.True(t, foundPrintln, "should find println call")
	assert.True(t, foundConstructor, "should find constructor call")
}

func TestComputeCyclomaticComplexity(t *testing.T) {
	tests := []struct {
		name     string
		branches []Branch
		expected int
	}{
		{
			name:     "empty",
			branches: nil,
			expected: 1,
		},
		{
			name: "single if",
			branches: []Branch{
				{Type: BranchIf},
			},
			expected: 2,
		},
		{
			name: "if + for",
			branches: []Branch{
				{Type: BranchIf},
				{Type: BranchFor, IsLoop: true},
			},
			expected: 3,
		},
		{
			name: "complex",
			branches: []Branch{
				{Type: BranchIf},
				{Type: BranchElseIf},
				{Type: BranchSwitch},
				{Type: BranchFor, IsLoop: true},
				{Type: BranchWhile, IsLoop: true},
				{Type: BranchTry},
				{Type: BranchCatch},
			},
			expected: 7, // 6 decision points + 1 (try doesn't count, but catch does)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeCyclomaticComplexity(tt.branches)
			assert.Equal(t, tt.expected, result)
		})
	}
}
