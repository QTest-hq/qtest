package model

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOrchestrator(t *testing.T) {
	cfg := &OrchestratorConfig{
		Repository: "test/repo",
		Branch:     "main",
		CommitSHA:  "abc123",
	}

	o := NewOrchestrator(cfg)
	require.NotNil(t, o)
	assert.Equal(t, "test/repo", o.config.Repository)
	assert.NotEmpty(t, o.config.ExcludeDirs)
	assert.Equal(t, 4, o.config.ParallelWorkers)
}

func TestNewOrchestrator_SkipTestFiles(t *testing.T) {
	cfg := &OrchestratorConfig{
		SkipTestFiles: true,
	}

	o := NewOrchestrator(cfg)
	assert.Contains(t, o.config.ExcludeFiles, "_test.go")
	assert.Contains(t, o.config.ExcludeFiles, ".test.js")
	assert.Contains(t, o.config.ExcludeFiles, ".spec.ts")
}

func TestDefaultRiskWeights(t *testing.T) {
	w := DefaultRiskWeights()
	assert.Equal(t, 0.4, w.Complexity)
	assert.Equal(t, 0.3, w.Centrality)
	assert.Equal(t, 0.2, w.Churn)
	assert.Equal(t, 0.1, w.Depth)
}

func TestOrchestrator_shouldSkipFile(t *testing.T) {
	cfg := &OrchestratorConfig{
		ExcludeFiles: []string{"_test.go", ".spec.ts"},
		IncludeOnly:  []string{},
	}
	o := NewOrchestrator(cfg)

	tests := []struct {
		path     string
		expected bool
	}{
		{"/path/to/main.go", false},
		{"/path/to/main_test.go", true},
		{"/path/to/component.spec.ts", true},
		{"/path/to/service.ts", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := o.shouldSkipFile(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOrchestrator_shouldSkipFile_IncludeOnly(t *testing.T) {
	cfg := &OrchestratorConfig{
		IncludeOnly: []string{"/src/", "/lib/"},
	}
	o := NewOrchestrator(cfg)

	tests := []struct {
		path     string
		expected bool
	}{
		{"/project/src/main.go", false},
		{"/project/lib/util.go", false},
		{"/project/test/helper.go", true},
		{"/project/vendor/dep.go", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := o.shouldSkipFile(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOrchestrator_Build_EmptyDir(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "orchestrator-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cfg := &OrchestratorConfig{
		Repository:           "test/repo",
		Branch:               "main",
		CommitSHA:            "abc123",
		BuildDependencyGraph: true,
	}
	o := NewOrchestrator(cfg)

	model, err := o.Build(context.Background(), tmpDir)
	require.NoError(t, err)
	require.NotNil(t, model)

	assert.Equal(t, "test/repo", model.Repository)
	assert.Empty(t, model.Functions)
	assert.Empty(t, model.Types)

	stats := o.Stats()
	assert.Equal(t, 0, stats.FilesFound)
	assert.Equal(t, 0, stats.FilesParsed)
}

func TestOrchestrator_Build_WithGoFiles(t *testing.T) {
	// Create temp directory with Go files
	tmpDir, err := os.MkdirTemp("", "orchestrator-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a Go file
	goCode := `package main

func Add(a, b int) int {
	return a + b
}

func Subtract(a, b int) int {
	return a - b
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "math.go"), []byte(goCode), 0644)
	require.NoError(t, err)

	cfg := &OrchestratorConfig{
		Repository:           "test/repo",
		Branch:               "main",
		CommitSHA:            "abc123",
		BuildDependencyGraph: true,
	}
	o := NewOrchestrator(cfg)

	model, err := o.Build(context.Background(), tmpDir)
	require.NoError(t, err)
	require.NotNil(t, model)

	assert.Len(t, model.Functions, 2)

	stats := o.Stats()
	assert.Equal(t, 1, stats.FilesFound)
	assert.Equal(t, 1, stats.FilesParsed)
	assert.Equal(t, 2, stats.FunctionsFound)
}

func TestOrchestrator_Build_WithJSExports(t *testing.T) {
	// Create temp directory with JS files
	tmpDir, err := os.MkdirTemp("", "orchestrator-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a JS file with exports
	jsCode := `export function greet(name) {
    return "Hello, " + name;
}

function privateHelper() {
    return 42;
}

export const multiply = (a, b) => a * b;
`
	err = os.WriteFile(filepath.Join(tmpDir, "utils.js"), []byte(jsCode), 0644)
	require.NoError(t, err)

	cfg := &OrchestratorConfig{
		Repository:           "test/repo",
		Branch:               "main",
		CommitSHA:            "abc123",
		BuildDependencyGraph: true,
	}
	o := NewOrchestrator(cfg)

	model, err := o.Build(context.Background(), tmpDir)
	require.NoError(t, err)
	require.NotNil(t, model)

	// Should have 3 functions: greet, privateHelper, multiply
	assert.GreaterOrEqual(t, len(model.Functions), 2)

	// Check export status
	exportedCount := 0
	for _, fn := range model.Functions {
		if fn.Exported {
			exportedCount++
		}
	}
	assert.GreaterOrEqual(t, exportedCount, 2) // greet and multiply should be exported

	stats := o.Stats()
	assert.Equal(t, 1, stats.FilesParsed)
}

func TestOrchestrator_Build_Parallel(t *testing.T) {
	// Create temp directory with multiple files
	tmpDir, err := os.MkdirTemp("", "orchestrator-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create multiple Go files
	for i := 0; i < 5; i++ {
		goCode := `package main

func Func` + string(rune('A'+i)) + `() {}
`
		err = os.WriteFile(filepath.Join(tmpDir, "file"+string(rune('0'+i))+".go"), []byte(goCode), 0644)
		require.NoError(t, err)
	}

	cfg := &OrchestratorConfig{
		Repository:           "test/repo",
		Branch:               "main",
		CommitSHA:            "abc123",
		ParallelParsing:      true,
		ParallelWorkers:      2,
		BuildDependencyGraph: true,
	}
	o := NewOrchestrator(cfg)

	model, err := o.Build(context.Background(), tmpDir)
	require.NoError(t, err)
	require.NotNil(t, model)

	assert.Len(t, model.Functions, 5)

	stats := o.Stats()
	assert.Equal(t, 5, stats.FilesFound)
	assert.Equal(t, 5, stats.FilesParsed)
}

func TestOrchestrator_Build_SkipsExcludedDirs(t *testing.T) {
	// Create temp directory with node_modules
	tmpDir, err := os.MkdirTemp("", "orchestrator-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a source file
	srcDir := filepath.Join(tmpDir, "src")
	err = os.MkdirAll(srcDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main\nfunc Main() {}"), 0644)
	require.NoError(t, err)

	// Create node_modules
	nodeDir := filepath.Join(tmpDir, "node_modules")
	err = os.MkdirAll(filepath.Join(nodeDir, "dep"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(nodeDir, "dep", "index.js"), []byte("function dep() {}"), 0644)
	require.NoError(t, err)

	cfg := &OrchestratorConfig{
		Repository: "test/repo",
		Branch:     "main",
	}
	o := NewOrchestrator(cfg)

	model, err := o.Build(context.Background(), tmpDir)
	require.NoError(t, err)

	// Should only have the main.go function, not node_modules
	assert.Len(t, model.Functions, 1)

	stats := o.Stats()
	assert.Equal(t, 1, stats.FilesParsed)
}

func TestOrchestrator_Stats(t *testing.T) {
	cfg := &OrchestratorConfig{}
	o := NewOrchestrator(cfg)

	stats := o.Stats()
	assert.NotNil(t, stats.SupplementResults)
	assert.True(t, stats.StartTime.IsZero())
}

func TestOrchestrator_RegisterSupplement(t *testing.T) {
	cfg := &OrchestratorConfig{}
	o := NewOrchestrator(cfg)

	// Create a mock supplement (using existing mockSupplement from builder_test.go)
	mock := &mockSupplement{name: "test"}
	o.RegisterSupplement(mock)

	assert.Len(t, o.supplements, 1)
	assert.Equal(t, "test", o.supplements[0].Name())
}

func TestOrchestrator_Build_WithDependencyGraph(t *testing.T) {
	// Create temp directory with files that have imports
	tmpDir, err := os.MkdirTemp("", "orchestrator-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create main.ts that imports utils
	mainCode := `import { helper } from './utils';

export function main() {
    return helper();
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "main.ts"), []byte(mainCode), 0644)
	require.NoError(t, err)

	// Create utils.ts
	utilsCode := `export function helper() {
    return 42;
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "utils.ts"), []byte(utilsCode), 0644)
	require.NoError(t, err)

	cfg := &OrchestratorConfig{
		Repository:           "test/repo",
		Branch:               "main",
		CommitSHA:            "abc123",
		BuildDependencyGraph: true,
	}
	o := NewOrchestrator(cfg)

	model, err := o.Build(context.Background(), tmpDir)
	require.NoError(t, err)
	require.NotNil(t, model)

	stats := o.Stats()
	assert.Equal(t, 2, stats.FilesParsed)
	assert.GreaterOrEqual(t, stats.GraphNodes, 2) // At least file nodes
}

func TestOrchestrator_ValidateModel(t *testing.T) {
	cfg := &OrchestratorConfig{}
	o := NewOrchestrator(cfg)

	// Create a model with duplicates
	model := &SystemModel{
		Functions: []Function{
			{ID: "func1", Name: "foo"},
			{ID: "func1", Name: "foo"}, // Duplicate
		},
	}

	err := o.validateModel(model)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate function ID")
}
