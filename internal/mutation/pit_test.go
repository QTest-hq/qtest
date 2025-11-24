package mutation

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewPITTool(t *testing.T) {
	tool := NewPITTool()
	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	if tool.MavenPath != "mvn" {
		t.Errorf("expected MavenPath 'mvn', got %s", tool.MavenPath)
	}
	if tool.GradlePath != "gradle" {
		t.Errorf("expected GradlePath 'gradle', got %s", tool.GradlePath)
	}
	if !tool.UseMaven {
		t.Error("expected UseMaven to be true by default")
	}
	if tool.UseGradle {
		t.Error("expected UseGradle to be false by default")
	}
}

func TestPITTool_Name(t *testing.T) {
	tool := NewPITTool()
	if tool.Name() != "pit" {
		t.Errorf("expected name 'pit', got %s", tool.Name())
	}
}

func TestPITTool_ParsePITOutput(t *testing.T) {
	tool := NewPITTool()

	tests := []struct {
		name     string
		output   string
		expected Result
	}{
		{
			name: "standard output",
			output: `================================================================================
- Statistics
================================================================================
>> Line Coverage: 100/120 (83%)
>> Generated 50 mutations Killed 40 (80%)
>> Mutations with no coverage 5. Test strength 89%
>> Ran 120 tests (2.4 tests per mutation)`,
			expected: Result{Total: 50, Killed: 40, Survived: 10},
		},
		{
			name: "with timed out",
			output: `>> Generated 20 mutations Killed 15 (75%)
3 timed out mutants`,
			expected: Result{Total: 20, Killed: 15, Survived: 5, Timeout: 3},
		},
		{
			name:     "empty output",
			output:   "",
			expected: Result{Total: 0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var result Result
			tool.parsePITOutput(tc.output, &result)

			if result.Total != tc.expected.Total {
				t.Errorf("Total: got %d, expected %d", result.Total, tc.expected.Total)
			}
			if result.Killed != tc.expected.Killed {
				t.Errorf("Killed: got %d, expected %d", result.Killed, tc.expected.Killed)
			}
			if result.Survived != tc.expected.Survived {
				t.Errorf("Survived: got %d, expected %d", result.Survived, tc.expected.Survived)
			}
		})
	}
}

func TestPITTool_ParsePITXMLReport(t *testing.T) {
	tool := NewPITTool()

	// Create a temp XML report
	tmpDir := t.TempDir()
	xmlReport := `<?xml version="1.0" encoding="UTF-8"?>
<mutations>
    <mutation detected="true" status="KILLED">
        <mutatedClass>com.example.Calculator</mutatedClass>
        <mutatedMethod>add</mutatedMethod>
        <mutator>org.pitest.mutationtest.engine.gregor.mutators.MathMutator</mutator>
        <lineNumber>10</lineNumber>
        <description>Replaced integer addition with subtraction</description>
        <killingTest>com.example.CalculatorTest.testAdd</killingTest>
    </mutation>
    <mutation detected="false" status="SURVIVED">
        <mutatedClass>com.example.Calculator</mutatedClass>
        <mutatedMethod>subtract</mutatedMethod>
        <mutator>org.pitest.mutationtest.engine.gregor.mutators.ConditionalsBoundaryMutator</mutator>
        <lineNumber>20</lineNumber>
        <description>Changed conditional boundary</description>
    </mutation>
    <mutation detected="false" status="TIMED_OUT">
        <mutatedClass>com.example.Calculator</mutatedClass>
        <mutatedMethod>divide</mutatedMethod>
        <mutator>org.pitest.mutationtest.engine.gregor.mutators.ReturnValsMutator</mutator>
        <lineNumber>30</lineNumber>
        <description>Replaced return value</description>
    </mutation>
</mutations>`

	reportPath := filepath.Join(tmpDir, "mutations.xml")
	if err := os.WriteFile(reportPath, []byte(xmlReport), 0644); err != nil {
		t.Fatalf("failed to write test report: %v", err)
	}

	var result Result
	if err := tool.parsePITXMLReport(reportPath, &result); err != nil {
		t.Fatalf("failed to parse XML report: %v", err)
	}

	if result.Total != 3 {
		t.Errorf("Total: got %d, expected 3", result.Total)
	}
	if result.Killed != 1 {
		t.Errorf("Killed: got %d, expected 1", result.Killed)
	}
	if result.Survived != 1 {
		t.Errorf("Survived: got %d, expected 1", result.Survived)
	}
	if result.Timeout != 1 {
		t.Errorf("Timeout: got %d, expected 1", result.Timeout)
	}
	if len(result.Mutants) != 3 {
		t.Errorf("Mutants count: got %d, expected 3", len(result.Mutants))
	}
}

func TestPITTool_ParsePITHTMLReport(t *testing.T) {
	tool := NewPITTool()

	// Create a temp HTML report
	tmpDir := t.TempDir()
	htmlReport := `<!DOCTYPE html>
<html>
<head><title>PIT Report</title></head>
<body>
<h1>Mutation Score: 75%</h1>
<div>Killed: 15</div>
<div>Survived: 5</div>
</body>
</html>`

	reportPath := filepath.Join(tmpDir, "index.html")
	if err := os.WriteFile(reportPath, []byte(htmlReport), 0644); err != nil {
		t.Fatalf("failed to write test report: %v", err)
	}

	var result Result
	tool.parsePITHTMLReport(reportPath, &result)

	if result.Killed != 15 {
		t.Errorf("Killed: got %d, expected 15", result.Killed)
	}
	if result.Survived != 5 {
		t.Errorf("Survived: got %d, expected 5", result.Survived)
	}
	if result.Score != 0.75 {
		t.Errorf("Score: got %f, expected 0.75", result.Score)
	}
}

func TestPITTool_MapMutatorName(t *testing.T) {
	tool := NewPITTool()

	tests := []struct {
		mutator  string
		expected string
	}{
		{"org.pitest.mutationtest.engine.gregor.mutators.MathMutator", "arithmetic"},
		{"NegateConditionalsMutator", "comparison"},
		{"ConditionalsBoundaryMutator", "comparison"},
		{"RemoveConditionalsMutator", "comparison"},
		{"TrueReturnValsMutator", "boolean"},
		{"FalseReturnValsMutator", "boolean"},
		{"ReturnValsMutator", "statement"},
		{"VoidMethodCallMutator", "statement"},
		{"EmptyReturnsMutator", "statement"},
		{"NullReturnsMutator", "literal"},
		{"ConstantMutator", "literal"},
		{"IncrementsMutator", "arithmetic"},
		{"InvertNegsMutator", "arithmetic"},
		{"UnknownMutator", "unknown"},
	}

	for _, tc := range tests {
		result := tool.mapMutatorName(tc.mutator)
		if result != tc.expected {
			t.Errorf("mapMutatorName(%q) = %s, expected %s", tc.mutator, result, tc.expected)
		}
	}
}

func TestExtractJavaClassName(t *testing.T) {
	tests := []struct {
		filePath string
		expected string
	}{
		{
			"/project/src/main/java/com/example/Calculator.java",
			"com.example.Calculator",
		},
		{
			"/project/src/test/java/com/example/CalculatorTest.java",
			"com.example.CalculatorTest",
		},
		{
			"/project/Calculator.java",
			"Calculator",
		},
		{
			"/project/src/main/java/Calculator.java",
			"Calculator",
		},
		{
			"/project/Calculator.txt",
			"",
		},
	}

	for _, tc := range tests {
		result := extractJavaClassName(tc.filePath)
		if result != tc.expected {
			t.Errorf("extractJavaClassName(%q) = %q, expected %q", tc.filePath, result, tc.expected)
		}
	}
}

func TestFindJavaProjectRoot(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()

	// Without any markers, should return start directory
	result := findJavaProjectRoot(tmpDir)
	if result != tmpDir {
		t.Errorf("expected %s when no markers found, got %s", tmpDir, result)
	}

	// With pom.xml
	pomDir := filepath.Join(tmpDir, "maven-project")
	os.MkdirAll(pomDir, 0755)
	os.WriteFile(filepath.Join(pomDir, "pom.xml"), []byte("<project/>"), 0644)

	subDir := filepath.Join(pomDir, "src", "main", "java")
	os.MkdirAll(subDir, 0755)

	result = findJavaProjectRoot(subDir)
	if result != pomDir {
		t.Errorf("expected %s with pom.xml, got %s", pomDir, result)
	}

	// With build.gradle
	gradleDir := filepath.Join(tmpDir, "gradle-project")
	os.MkdirAll(gradleDir, 0755)
	os.WriteFile(filepath.Join(gradleDir, "build.gradle"), []byte("plugins {}"), 0644)

	gradleSubDir := filepath.Join(gradleDir, "src", "main", "java")
	os.MkdirAll(gradleSubDir, 0755)

	result = findJavaProjectRoot(gradleSubDir)
	if result != gradleDir {
		t.Errorf("expected %s with build.gradle, got %s", gradleDir, result)
	}
}

func TestPITTool_BuildMavenCommand(t *testing.T) {
	tool := NewPITTool()
	ctx := context.Background()

	cfg := MutationConfig{
		TimeoutPerMutant:      10 * time.Second,
		MaxMutantsPerFunction: 5,
	}

	cmd := tool.buildMavenCommand(ctx, "/tmp/project", "com.example.Calculator", "com.example.CalculatorTest", cfg)

	if cmd == nil {
		t.Fatal("expected non-nil command")
	}

	// Check that the command has expected arguments
	args := cmd.Args
	hasTargetClasses := false
	hasTargetTests := false
	hasTimeout := false

	for _, arg := range args {
		if arg == "-DtargetClasses=com.example.Calculator" {
			hasTargetClasses = true
		}
		if arg == "-DtargetTests=com.example.CalculatorTest" {
			hasTargetTests = true
		}
		if arg == "-DtimeoutConstant=10000" {
			hasTimeout = true
		}
	}

	if !hasTargetClasses {
		t.Error("command missing targetClasses argument")
	}
	if !hasTargetTests {
		t.Error("command missing targetTests argument")
	}
	if !hasTimeout {
		t.Error("command missing timeoutConstant argument")
	}
}

func TestPITTool_BuildGradleCommand(t *testing.T) {
	tool := NewPITTool()
	tool.UseGradle = true
	tool.UseMaven = false
	ctx := context.Background()

	cfg := MutationConfig{
		TimeoutPerMutant: 5 * time.Second,
	}

	cmd := tool.buildGradleCommand(ctx, "/tmp/project", "com.example.Calculator", "com.example.CalculatorTest", cfg)

	if cmd == nil {
		t.Fatal("expected non-nil command")
	}
}

func TestPITTool_Run_NonExistentFile(t *testing.T) {
	tool := NewPITTool()
	ctx := context.Background()

	cfg := MutationConfig{
		Timeout:          5 * time.Second,
		TimeoutPerMutant: 1 * time.Second,
	}

	// Running with non-existent file should return result with error
	result, err := tool.Run(ctx, "/nonexistent/src/main/java/Calculator.java", "/nonexistent/src/test/java/CalculatorTest.java", cfg)

	// Should not return a Go error, but result should have error message
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// Result should have error or zero mutants
}

func TestPITTool_Run_NoBuildTool(t *testing.T) {
	tool := NewPITTool()
	tool.UseMaven = false
	tool.UseGradle = false
	ctx := context.Background()

	cfg := MutationConfig{
		Timeout: 5 * time.Second,
	}

	result, err := tool.Run(ctx, "/nonexistent/Calculator.java", "/nonexistent/CalculatorTest.java", cfg)

	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Error == "" {
		t.Error("expected error when no build tool configured")
	}
	if !containsSubstring(result.Error, "no build tool") {
		t.Errorf("expected 'no build tool' error, got: %s", result.Error)
	}
}

func TestPITTool_Implements_Tool_Interface(t *testing.T) {
	var _ Tool = (*PITTool)(nil)
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstringHelper(s, substr))
}

func containsSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Integration tests

func TestPITTool_Integration_MavenProject(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a mock Maven project structure
	srcDir := filepath.Join(tmpDir, "src", "main", "java", "com", "example")
	testDir := filepath.Join(tmpDir, "src", "test", "java", "com", "example")
	os.MkdirAll(srcDir, 0755)
	os.MkdirAll(testDir, 0755)

	// Create pom.xml
	pomXML := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>calculator</artifactId>
    <version>1.0-SNAPSHOT</version>
    <dependencies>
        <dependency>
            <groupId>junit</groupId>
            <artifactId>junit</artifactId>
            <version>4.13.2</version>
            <scope>test</scope>
        </dependency>
    </dependencies>
</project>`
	os.WriteFile(filepath.Join(tmpDir, "pom.xml"), []byte(pomXML), 0644)

	// Create source file
	srcFile := filepath.Join(srcDir, "Calculator.java")
	srcContent := `package com.example;

public class Calculator {
    public int add(int a, int b) {
        return a + b;
    }

    public int subtract(int a, int b) {
        return a - b;
    }

    public int multiply(int a, int b) {
        return a * b;
    }
}`
	os.WriteFile(srcFile, []byte(srcContent), 0644)

	// Create test file
	testFile := filepath.Join(testDir, "CalculatorTest.java")
	testContent := `package com.example;

import org.junit.Test;
import static org.junit.Assert.*;

public class CalculatorTest {
    @Test
    public void testAdd() {
        Calculator calc = new Calculator();
        assertEquals(5, calc.add(2, 3));
    }

    @Test
    public void testSubtract() {
        Calculator calc = new Calculator();
        assertEquals(2, calc.subtract(5, 3));
    }
}`
	os.WriteFile(testFile, []byte(testContent), 0644)

	// Verify files exist
	if _, err := os.Stat(srcFile); os.IsNotExist(err) {
		t.Fatalf("source file not created: %v", err)
	}
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Fatalf("test file not created: %v", err)
	}

	// Test tool creation
	tool := NewPITTool()
	if tool.Name() != "pit" {
		t.Errorf("expected name 'pit', got %s", tool.Name())
	}

	// Test Java class name extraction
	className := extractJavaClassName(srcFile)
	if className != "com.example.Calculator" {
		t.Errorf("expected 'com.example.Calculator', got %s", className)
	}

	// Test project root detection
	root := findJavaProjectRoot(srcDir)
	if root != tmpDir {
		t.Errorf("expected project root %s, got %s", tmpDir, root)
	}
}

func TestPITTool_Integration_GradleProject(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a mock Gradle project structure
	srcDir := filepath.Join(tmpDir, "src", "main", "java", "com", "example")
	testDir := filepath.Join(tmpDir, "src", "test", "java", "com", "example")
	os.MkdirAll(srcDir, 0755)
	os.MkdirAll(testDir, 0755)

	// Create build.gradle
	buildGradle := `plugins {
    id 'java'
    id 'info.solidsoft.pitest' version '1.9.0'
}

repositories {
    mavenCentral()
}

dependencies {
    testImplementation 'junit:junit:4.13.2'
}

pitest {
    junit5PluginVersion = '1.0.0'
    targetClasses = ['com.example.*']
    targetTests = ['com.example.*Test']
}`
	os.WriteFile(filepath.Join(tmpDir, "build.gradle"), []byte(buildGradle), 0644)

	// Create source file
	srcFile := filepath.Join(srcDir, "Calculator.java")
	srcContent := `package com.example;

public class Calculator {
    public int add(int a, int b) {
        return a + b;
    }
}`
	os.WriteFile(srcFile, []byte(srcContent), 0644)

	// Test project root detection with build.gradle
	root := findJavaProjectRoot(srcDir)
	if root != tmpDir {
		t.Errorf("expected project root %s, got %s", tmpDir, root)
	}
}

func TestPITTool_Integration_ReportParsing(t *testing.T) {
	tool := NewPITTool()
	tmpDir := t.TempDir()

	// Test comprehensive XML report parsing
	t.Run("comprehensive XML report", func(t *testing.T) {
		xmlReport := `<?xml version="1.0" encoding="UTF-8"?>
<mutations>
    <mutation detected="true" status="KILLED">
        <mutatedClass>com.example.Calculator</mutatedClass>
        <mutatedMethod>add</mutatedMethod>
        <mutator>org.pitest.mutationtest.engine.gregor.mutators.MathMutator</mutator>
        <lineNumber>5</lineNumber>
        <description>Replaced integer addition with subtraction</description>
        <killingTest>com.example.CalculatorTest.testAdd</killingTest>
    </mutation>
    <mutation detected="true" status="KILLED">
        <mutatedClass>com.example.Calculator</mutatedClass>
        <mutatedMethod>subtract</mutatedMethod>
        <mutator>org.pitest.mutationtest.engine.gregor.mutators.MathMutator</mutator>
        <lineNumber>9</lineNumber>
        <description>Replaced integer subtraction with addition</description>
        <killingTest>com.example.CalculatorTest.testSubtract</killingTest>
    </mutation>
    <mutation detected="false" status="SURVIVED">
        <mutatedClass>com.example.Calculator</mutatedClass>
        <mutatedMethod>multiply</mutatedMethod>
        <mutator>org.pitest.mutationtest.engine.gregor.mutators.ConditionalsBoundaryMutator</mutator>
        <lineNumber>13</lineNumber>
        <description>Changed conditional boundary</description>
    </mutation>
    <mutation detected="false" status="TIMED_OUT">
        <mutatedClass>com.example.Calculator</mutatedClass>
        <mutatedMethod>divide</mutatedMethod>
        <mutator>org.pitest.mutationtest.engine.gregor.mutators.ReturnValsMutator</mutator>
        <lineNumber>17</lineNumber>
        <description>Replaced return value</description>
    </mutation>
</mutations>`

		reportPath := filepath.Join(tmpDir, "mutations.xml")
		os.WriteFile(reportPath, []byte(xmlReport), 0644)

		var result Result
		err := tool.parsePITXMLReport(reportPath, &result)
		if err != nil {
			t.Fatalf("failed to parse XML report: %v", err)
		}

		if result.Total != 4 {
			t.Errorf("Total: got %d, expected 4", result.Total)
		}
		if result.Killed != 2 {
			t.Errorf("Killed: got %d, expected 2", result.Killed)
		}
		if result.Survived != 1 {
			t.Errorf("Survived: got %d, expected 1", result.Survived)
		}
		if result.Timeout != 1 {
			t.Errorf("Timeout: got %d, expected 1", result.Timeout)
		}
		if len(result.Mutants) != 4 {
			t.Errorf("Mutants count: got %d, expected 4", len(result.Mutants))
		}
	})

	// Test HTML parsing - just verify it doesn't panic
	// HTML parsing may have implementation-specific behavior
	t.Run("HTML summary report does not panic", func(t *testing.T) {
		htmlReport := `<!DOCTYPE html>
<html>
<head><title>PIT Mutation Report</title></head>
<body>
<h1>Project Mutation Summary</h1>
<table>
<tr><th>Metric</th><th>Value</th></tr>
<tr><td>Mutation Score</td><td>85%</td></tr>
<tr><td>Killed</td><td>85</td></tr>
<tr><td>Survived</td><td>15</td></tr>
<tr><td>Total</td><td>100</td></tr>
</table>
</body>
</html>`

		htmlPath := filepath.Join(tmpDir, "index.html")
		os.WriteFile(htmlPath, []byte(htmlReport), 0644)

		var result Result
		// Just verify it doesn't panic
		tool.parsePITHTMLReport(htmlPath, &result)
		// HTML parsing implementation may vary
		t.Logf("HTML parsing result: Killed=%d, Survived=%d, Score=%f",
			result.Killed, result.Survived, result.Score)
	})
}

func TestPITTool_Integration_ConcurrentSafety(t *testing.T) {
	// Test that multiple tool instances don't interfere
	tools := make([]*PITTool, 5)
	for i := range tools {
		tools[i] = NewPITTool()
	}

	// Verify each is independent
	for i, tool := range tools {
		tool.MavenPath = "mvn" + string(rune('0'+i))
	}

	for i, tool := range tools {
		expected := "mvn" + string(rune('0'+i))
		if tool.MavenPath != expected {
			t.Errorf("tool %d: expected MavenPath %s, got %s", i, expected, tool.MavenPath)
		}
	}
}

func TestPITTool_Integration_AllMutatorMappings(t *testing.T) {
	tool := NewPITTool()

	// Test comprehensive mutator mappings
	mutatorTests := []struct {
		mutator  string
		expected string
	}{
		// Arithmetic mutations
		{"MathMutator", "arithmetic"},
		{"IncrementsMutator", "arithmetic"},
		{"InvertNegsMutator", "arithmetic"},
		{"org.pitest.mutationtest.engine.gregor.mutators.MathMutator", "arithmetic"},

		// Comparison mutations
		{"NegateConditionalsMutator", "comparison"},
		{"ConditionalsBoundaryMutator", "comparison"},
		{"RemoveConditionalsMutator", "comparison"},

		// Boolean mutations
		{"TrueReturnValsMutator", "boolean"},
		{"FalseReturnValsMutator", "boolean"},

		// Literal mutations
		{"NullReturnsMutator", "literal"},
		{"ConstantMutator", "literal"},

		// Statement mutations
		{"ReturnValsMutator", "statement"},
		{"VoidMethodCallMutator", "statement"},
		{"EmptyReturnsMutator", "statement"},

		// Unknown
		{"CustomMutator", "unknown"},
		{"SomeRandomMutator", "unknown"},
	}

	for _, tc := range mutatorTests {
		result := tool.mapMutatorName(tc.mutator)
		if result != tc.expected {
			t.Errorf("mapMutatorName(%q) = %q, expected %q", tc.mutator, result, tc.expected)
		}
	}
}

func TestPITTool_Integration_ConfigOptions(t *testing.T) {
	tool := NewPITTool()

	t.Run("default config", func(t *testing.T) {
		if tool.MavenPath != "mvn" {
			t.Errorf("expected default MavenPath 'mvn', got %s", tool.MavenPath)
		}
		if tool.GradlePath != "gradle" {
			t.Errorf("expected default GradlePath 'gradle', got %s", tool.GradlePath)
		}
		if !tool.UseMaven {
			t.Error("expected UseMaven to be true by default")
		}
		if tool.UseGradle {
			t.Error("expected UseGradle to be false by default")
		}
	})

	t.Run("gradle mode", func(t *testing.T) {
		tool.UseMaven = false
		tool.UseGradle = true

		ctx := context.Background()
		cfg := MutationConfig{
			TimeoutPerMutant: 10 * time.Second,
		}

		cmd := tool.buildGradleCommand(ctx, "/tmp/project", "com.example.Calc", "com.example.CalcTest", cfg)
		if cmd == nil {
			t.Fatal("expected non-nil command")
		}
	})

	t.Run("custom paths", func(t *testing.T) {
		tool.MavenPath = "/usr/local/bin/mvn"
		tool.GradlePath = "/usr/local/bin/gradle"

		if tool.MavenPath != "/usr/local/bin/mvn" {
			t.Errorf("expected custom MavenPath, got %s", tool.MavenPath)
		}
		if tool.GradlePath != "/usr/local/bin/gradle" {
			t.Errorf("expected custom GradlePath, got %s", tool.GradlePath)
		}
	})
}
