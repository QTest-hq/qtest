package adapters

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/QTest-hq/qtest/pkg/model"
)

// JUnitSpecAdapter generates JUnit 5 test code from model.TestSpec
type JUnitSpecAdapter struct{}

func NewJUnitSpecAdapter() *JUnitSpecAdapter {
	return &JUnitSpecAdapter{}
}

func (a *JUnitSpecAdapter) Framework() Framework {
	return FrameworkJUnit
}

func (a *JUnitSpecAdapter) FileExtension() string {
	return ".java"
}

func (a *JUnitSpecAdapter) TestFileSuffix() string {
	return "Test"
}

const junitSpecTemplate = `package {{.Package}};

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.DisplayName;
import static org.junit.jupiter.api.Assertions.*;
{{range .Imports}}
import {{.}};
{{end}}

/**
 * Tests for {{.ClassName}}
 */
class {{.ClassName}}Test {
{{range .Tests}}
    @Test
    @DisplayName("{{.DisplayName}}")
    void {{.MethodName}}() {
        // Arrange
{{if .Setup}}{{.Setup}}{{end}}
        // Act
        {{.Action}}

        // Assert
{{range .Assertions}}        {{.}}
{{end}}    }
{{end}}
}
`

type junitSpecTemplateData struct {
	Package   string
	ClassName string
	Imports   []string
	Tests     []junitSpecTestData
}

type junitSpecTestData struct {
	MethodName  string
	DisplayName string
	Setup       string
	Action      string
	Assertions  []string
}

// GenerateFromSpecs generates JUnit test code from TestSpec slice
func (a *JUnitSpecAdapter) GenerateFromSpecs(specs []model.TestSpec, sourceFile string) (string, error) {
	if len(specs) == 0 {
		return "", fmt.Errorf("no test specs provided")
	}

	// Determine class name from source file
	className := extractJavaClassName(sourceFile)

	data := junitSpecTemplateData{
		Package:   extractJavaPackageName(sourceFile),
		ClassName: className,
		Imports:   []string{},
		Tests:     make([]junitSpecTestData, 0),
	}

	// Build tests from specs
	for i, spec := range specs {
		testData := junitSpecTestData{
			MethodName:  toJavaMethodName(spec.Description, i),
			DisplayName: escapeJavaString(spec.Description),
			Assertions:  make([]string, 0),
		}

		// Generate setup from inputs with type hints
		if len(spec.Inputs) > 0 {
			testData.Setup = a.generateSetup(spec)
		}

		// Generate action (method call)
		testData.Action = a.generateAction(spec, className)

		// Generate assertions from spec.Assertions
		for _, assertion := range spec.Assertions {
			assertCode := a.generateAssertion(assertion)
			if assertCode != "" {
				testData.Assertions = append(testData.Assertions, assertCode)
			}
		}

		// If no assertions were generated, add a placeholder
		if len(testData.Assertions) == 0 {
			testData.Assertions = append(testData.Assertions, "// TODO: Add assertions")
		}

		data.Tests = append(data.Tests, testData)
	}

	// Execute template
	tmpl, err := template.New("junitspec").Parse(junitSpecTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// generateSetup generates setup code from inputs with type hints
func (a *JUnitSpecAdapter) generateSetup(spec model.TestSpec) string {
	var setup strings.Builder

	// Use ArgOrder if available, otherwise sort keys
	var keys []string
	if len(spec.ArgOrder) > 0 {
		keys = spec.ArgOrder
	} else {
		// Separate named args from indexed args
		var namedKeys []string
		var indexedKeys []string
		for key := range spec.Inputs {
			if strings.HasPrefix(key, "arg") {
				indexedKeys = append(indexedKeys, key)
			} else {
				namedKeys = append(namedKeys, key)
			}
		}

		// If we have named args, use those; otherwise use indexed args
		if len(namedKeys) > 0 {
			sort.Strings(namedKeys)
			keys = namedKeys
		} else {
			sort.Slice(indexedKeys, func(i, j int) bool {
				numI, _ := strconv.Atoi(strings.TrimPrefix(indexedKeys[i], "arg"))
				numJ, _ := strconv.Atoi(strings.TrimPrefix(indexedKeys[j], "arg"))
				return numI < numJ
			})
			keys = indexedKeys
		}
	}

	for _, key := range keys {
		value, ok := spec.Inputs[key]
		if !ok {
			continue
		}
		typeHint := "Object"
		if spec.InputTypes != nil && spec.InputTypes[key] != "" {
			typeHint = toJavaType(spec.InputTypes[key])
		} else {
			typeHint = inferJavaType(value)
		}
		setup.WriteString(fmt.Sprintf("        %s %s = %s;\n", typeHint, key, formatJavaValue(value, typeHint)))
	}
	return setup.String()
}

// generateAction generates the method call
func (a *JUnitSpecAdapter) generateAction(spec model.TestSpec, className string) string {
	funcName := spec.FunctionName
	if funcName == "" {
		funcName = spec.TargetID
	}

	// Build args list
	var args []string
	if len(spec.ArgOrder) > 0 {
		args = spec.ArgOrder
	} else {
		// Build args from inputs
		var namedArgs []string
		var indexedArgs []string

		for key := range spec.Inputs {
			if strings.HasPrefix(key, "arg") {
				indexedArgs = append(indexedArgs, key)
			} else {
				namedArgs = append(namedArgs, key)
			}
		}

		// Use named args if available
		if len(namedArgs) > 0 {
			sort.Strings(namedArgs)
			args = namedArgs
		} else if len(indexedArgs) > 0 {
			sort.Slice(indexedArgs, func(i, j int) bool {
				numI, _ := strconv.Atoi(strings.TrimPrefix(indexedArgs[i], "arg"))
				numJ, _ := strconv.Atoi(strings.TrimPrefix(indexedArgs[j], "arg"))
				return numI < numJ
			})
			args = indexedArgs
		}
	}

	// For static methods or when no instance is needed
	return fmt.Sprintf("var result = %s.%s(%s);", className, funcName, strings.Join(args, ", "))
}

// generateAssertion generates JUnit assertion code from model.Assertion
func (a *JUnitSpecAdapter) generateAssertion(assertion model.Assertion) string {
	actual := stripJavaDollarPrefix(assertion.Actual)
	if actual == "" {
		actual = "result"
	}

	switch assertion.Kind {
	case "equality", "equals":
		expected := formatJavaValue(assertion.Expected, "")
		return fmt.Sprintf("assertEquals(%s, %s);", expected, actual)

	case "not_equal", "not_equals":
		expected := formatJavaValue(assertion.Expected, "")
		return fmt.Sprintf("assertNotEquals(%s, %s);", expected, actual)

	case "not_null", "not_nil", "is_not_nil":
		return fmt.Sprintf("assertNotNull(%s);", actual)

	case "null", "nil", "is_nil":
		return fmt.Sprintf("assertNull(%s);", actual)

	case "contains":
		expected := formatJavaValue(assertion.Expected, "String")
		return fmt.Sprintf("assertTrue(%s.contains(%s));", actual, expected)

	case "greater_than":
		expected := formatJavaValue(assertion.Expected, "")
		return fmt.Sprintf("assertTrue(%s > %s);", actual, expected)

	case "less_than":
		expected := formatJavaValue(assertion.Expected, "")
		return fmt.Sprintf("assertTrue(%s < %s);", actual, expected)

	case "truthy":
		return fmt.Sprintf("assertTrue(%s);", actual)

	case "falsy":
		return fmt.Sprintf("assertFalse(%s);", actual)

	case "throws", "error":
		return "// Exception is expected - wrap call in assertThrows()"

	case "type", "type_is":
		expected := assertion.Expected
		return fmt.Sprintf("assertTrue(%s instanceof %s);", actual, expected)

	case "length":
		expected := formatJavaValue(assertion.Expected, "int")
		return fmt.Sprintf("assertEquals(%s, %s.length);", expected, actual)

	default:
		if assertion.Expected != nil {
			expected := formatJavaValue(assertion.Expected, "")
			return fmt.Sprintf("assertEquals(%s, %s);", expected, actual)
		}
		return ""
	}
}

// formatJavaValue formats a value for Java code
func formatJavaValue(val interface{}, typeHint string) string {
	if val == nil {
		return "null"
	}

	switch v := val.(type) {
	case string:
		// Check if it looks like a number
		if typeHint == "int" || typeHint == "Integer" {
			if _, err := fmt.Sscanf(v, "%d", new(int)); err == nil {
				return v
			}
		}
		if typeHint == "double" || typeHint == "Double" || typeHint == "float" || typeHint == "Float" {
			if _, err := fmt.Sscanf(v, "%f", new(float64)); err == nil {
				return v
			}
		}
		if v == "true" || v == "false" {
			return v
		}
		return fmt.Sprintf("\"%s\"", escapeJavaString(v))
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32:
		return fmt.Sprintf("%vf", v)
	case float64:
		if typeHint == "float" || typeHint == "Float" {
			return fmt.Sprintf("%vf", v)
		}
		// Check if it's a whole number
		if float64(int64(v)) == v {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%v", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case []interface{}:
		elements := make([]string, len(v))
		for i, elem := range v {
			elements[i] = formatJavaValue(elem, "")
		}
		return fmt.Sprintf("new Object[]{%s}", strings.Join(elements, ", "))
	case map[string]interface{}:
		return "Map.of(/* TODO: construct map */)"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// inferJavaType infers Java type from a value
func inferJavaType(val interface{}) string {
	if val == nil {
		return "Object"
	}

	switch v := val.(type) {
	case string:
		return "String"
	case int, int8, int16, int32, int64:
		return "int"
	case uint, uint8, uint16, uint32, uint64:
		return "long"
	case float32:
		return "float"
	case float64:
		// Check if it's a whole number
		if float64(int64(v)) == v {
			return "int"
		}
		return "double"
	case bool:
		return "boolean"
	case []interface{}:
		return "Object[]"
	case map[string]interface{}:
		return "Map<String, Object>"
	default:
		return "Object"
	}
}

// toJavaType converts a type hint to Java type
func toJavaType(typeHint string) string {
	switch strings.ToLower(typeHint) {
	case "int", "integer":
		return "int"
	case "long":
		return "long"
	case "float":
		return "float"
	case "double":
		return "double"
	case "string":
		return "String"
	case "bool", "boolean":
		return "boolean"
	case "array":
		return "Object[]"
	case "object":
		return "Object"
	case "null":
		return "Object"
	default:
		return typeHint
	}
}

// toJavaMethodName converts a description to a valid Java method name (camelCase)
func toJavaMethodName(description string, index int) string {
	if description == "" {
		return fmt.Sprintf("testCase%d", index+1)
	}

	// Replace special characters with spaces
	description = strings.ReplaceAll(description, "_", " ")
	description = strings.ReplaceAll(description, "-", " ")
	description = strings.ReplaceAll(description, ".", " ")
	description = strings.ReplaceAll(description, "'", "")
	description = strings.ReplaceAll(description, "\"", "")
	description = strings.ReplaceAll(description, "(", "")
	description = strings.ReplaceAll(description, ")", "")
	description = strings.ReplaceAll(description, ",", " ")

	// Split into words
	words := strings.Fields(description)
	if len(words) == 0 {
		return fmt.Sprintf("testCase%d", index+1)
	}

	// First word lowercase, rest TitleCase
	var result strings.Builder
	for i, word := range words {
		if i == 0 {
			result.WriteString(strings.ToLower(word))
		} else {
			if len(word) > 0 {
				result.WriteString(strings.ToUpper(word[:1]))
				if len(word) > 1 {
					result.WriteString(strings.ToLower(word[1:]))
				}
			}
		}
	}

	return result.String()
}

// extractJavaClassName extracts the class name from a file path
func extractJavaClassName(filePath string) string {
	if filePath == "" {
		return "Unknown"
	}

	// Get just the filename without path
	parts := strings.Split(filePath, "/")
	filename := parts[len(parts)-1]

	// Also handle Windows paths
	parts = strings.Split(filename, "\\")
	filename = parts[len(parts)-1]

	// Remove .java extension
	name := strings.TrimSuffix(filename, ".java")

	return name
}

// extractJavaPackageName extracts a package name from a file path
func extractJavaPackageName(filePath string) string {
	// Default package
	if filePath == "" {
		return "tests"
	}

	// Try to extract from path (e.g., src/main/java/com/example/MyClass.java)
	if strings.Contains(filePath, "src/main/java/") {
		idx := strings.Index(filePath, "src/main/java/")
		path := filePath[idx+len("src/main/java/"):]
		// Remove filename
		parts := strings.Split(path, "/")
		if len(parts) > 1 {
			return strings.Join(parts[:len(parts)-1], ".")
		}
	}

	// Try to extract from test path
	if strings.Contains(filePath, "src/test/java/") {
		idx := strings.Index(filePath, "src/test/java/")
		path := filePath[idx+len("src/test/java/"):]
		parts := strings.Split(path, "/")
		if len(parts) > 1 {
			return strings.Join(parts[:len(parts)-1], ".")
		}
	}

	return "tests"
}

// escapeJavaString escapes special characters in a Java string
func escapeJavaString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

// stripJavaDollarPrefix removes $ prefix from variable references
func stripJavaDollarPrefix(s string) string {
	if strings.HasPrefix(s, "$") {
		return s[1:]
	}
	return s
}
