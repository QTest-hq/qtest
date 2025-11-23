package maintenance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRemover(t *testing.T) {
	config := DefaultRemoverConfig()
	r := NewRemover(config)
	assert.NotNil(t, r)
}

func TestDefaultRemoverConfig(t *testing.T) {
	config := DefaultRemoverConfig()
	assert.False(t, config.DryRun)
	assert.False(t, config.CommentOutInsteadOfDelete)
	assert.True(t, config.BackupBeforeRemoval)
	assert.Equal(t, ".qtest/backups", config.BackupDir)
}

func TestRemoveTests_NoTestMapping(t *testing.T) {
	r := NewRemover(DefaultRemoverConfig())

	jobs := []MaintenanceJob{
		{Type: JobTypeRemove, TargetID: "fn1"},
	}

	results := r.RemoveTests(jobs, map[string][]TestInfo{})
	require.Len(t, results, 1)
	assert.False(t, results[0].Removed)
	assert.Contains(t, results[0].Error, "no tests found")
}

func TestRemoveTests_SkipsNonRemoveJobs(t *testing.T) {
	r := NewRemover(DefaultRemoverConfig())

	jobs := []MaintenanceJob{
		{Type: JobTypeCreate, TargetID: "fn1"},
		{Type: JobTypeUpdate, TargetID: "fn2"},
	}

	testMapping := map[string][]TestInfo{
		"fn1": {{File: "/tmp/test.go", FunctionName: "TestAdd"}},
	}

	results := r.RemoveTests(jobs, testMapping)
	assert.Empty(t, results)
}

func TestRemoveLines(t *testing.T) {
	r := NewRemover(DefaultRemoverConfig())

	lines := []string{
		"line1",
		"line2",
		"line3",
		"line4",
		"line5",
	}

	// Remove lines 1-2 (0-indexed: 1 and 2)
	result := r.removeLines(lines, 1, 2)
	resultLines := strings.Split(result, "\n")

	assert.Len(t, resultLines, 3)
	assert.Equal(t, "line1", resultLines[0])
	assert.Equal(t, "line4", resultLines[1])
	assert.Equal(t, "line5", resultLines[2])
}

func TestRemoveLines_WithPrecedingBlankLines(t *testing.T) {
	r := NewRemover(DefaultRemoverConfig())

	lines := []string{
		"package main",
		"",
		"@Test",
		"func TestAdd(t *testing.T) {",
		"    // test",
		"}",
	}

	// Remove function with decorator
	result := r.removeLines(lines, 3, 5)
	resultLines := strings.Split(result, "\n")

	// Should also remove the blank line and @Test decorator
	assert.Equal(t, 1, len(resultLines))
	assert.Equal(t, "package main", resultLines[0])
}

func TestCommentOutLines(t *testing.T) {
	r := NewRemover(DefaultRemoverConfig())

	lines := []string{
		"line1",
		"func TestAdd(t *testing.T) {",
		"    // test",
		"}",
		"line5",
	}

	result := r.commentOutLines(lines, 1, 3)

	// Check that the commented lines contain expected markers
	assert.Contains(t, result, "REMOVED BY QTEST")
	assert.Contains(t, result, "// func TestAdd")
	assert.Contains(t, result, "//     // test")
	assert.Contains(t, result, "// }")
}

func TestFindTestFunction_Go(t *testing.T) {
	r := NewRemover(DefaultRemoverConfig())

	lines := []string{
		"package main",
		"",
		"func TestAdd(t *testing.T) {",
		"    result := Add(1, 2)",
		"    assert.Equal(t, 3, result)",
		"}",
		"",
		"func TestSubtract(t *testing.T) {",
		"    result := Subtract(5, 3)",
		"    assert.Equal(t, 2, result)",
		"}",
	}

	start, end := r.findTestFunction(lines, "TestAdd")
	assert.Equal(t, 2, start)
	assert.Equal(t, 5, end)

	start, end = r.findTestFunction(lines, "TestSubtract")
	assert.Equal(t, 7, start)
	assert.Equal(t, 10, end)
}

func TestFindTestFunction_Python(t *testing.T) {
	r := NewRemover(DefaultRemoverConfig())

	lines := []string{
		"import pytest",
		"",
		"def test_add():",
		"    result = add(1, 2)",
		"    assert result == 3",
		"",
		"def test_subtract():",
		"    result = subtract(5, 3)",
		"    assert result == 2",
	}

	start, end := r.findTestFunction(lines, "test_add")
	assert.Equal(t, 2, start)
	// End is 5 because the indentation-based detection stops at the blank line
	assert.Equal(t, 5, end)
}

func TestFindTestFunction_NotFound(t *testing.T) {
	r := NewRemover(DefaultRemoverConfig())

	lines := []string{
		"package main",
		"func TestAdd(t *testing.T) {}",
	}

	start, end := r.findTestFunction(lines, "TestNonExistent")
	assert.Equal(t, -1, start)
	assert.Equal(t, -1, end)
}

func TestGetIndentation(t *testing.T) {
	r := NewRemover(DefaultRemoverConfig())

	tests := []struct {
		line     string
		expected int
	}{
		{"no indent", 0},
		{"  two spaces", 2},
		{"    four spaces", 4},
		{"\ttab", 4},
		{"\t\ttwo tabs", 8},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			result := r.getIndentation(tt.line)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsTestFileEmpty_Go(t *testing.T) {
	r := NewRemover(DefaultRemoverConfig())

	hasTests := `package main

func TestAdd(t *testing.T) {
    // test
}
`
	assert.False(t, r.isTestFileEmpty(hasTests))

	noTests := `package main

// Just a comment
func main() {}
`
	assert.True(t, r.isTestFileEmpty(noTests))
}

func TestIsTestFileEmpty_Python(t *testing.T) {
	r := NewRemover(DefaultRemoverConfig())

	hasTests := `import pytest

def test_add():
    assert add(1, 2) == 3
`
	assert.False(t, r.isTestFileEmpty(hasTests))

	noTests := `# Just a comment
def main():
    pass
`
	assert.True(t, r.isTestFileEmpty(noTests))
}

func TestIsTestFileEmpty_JUnit(t *testing.T) {
	r := NewRemover(DefaultRemoverConfig())

	hasTests := `public class Test {
    @Test
    void testAdd() {}
}
`
	assert.False(t, r.isTestFileEmpty(hasTests))

	noTests := `public class NotATest {
    void main() {}
}
`
	assert.True(t, r.isTestFileEmpty(noTests))
}

func TestRemoveTestFunction_ByLineNumbers(t *testing.T) {
	r := NewRemover(DefaultRemoverConfig())

	content := `package main

func TestAdd(t *testing.T) {
    result := Add(1, 2)
}

func TestSub(t *testing.T) {
    result := Sub(3, 1)
}
`
	test := TestInfo{
		FunctionName: "TestAdd",
		StartLine:    3,
		EndLine:      5,
	}

	newContent, removed := r.removeTestFunction(content, test, false)
	assert.True(t, removed)
	assert.NotContains(t, newContent, "TestAdd")
	assert.Contains(t, newContent, "TestSub")
}

func TestRemoveTestFunction_CommentOut(t *testing.T) {
	r := NewRemover(DefaultRemoverConfig())

	content := `package main

func TestAdd(t *testing.T) {
    result := Add(1, 2)
}
`
	test := TestInfo{
		FunctionName: "TestAdd",
		StartLine:    3,
		EndLine:      5,
	}

	newContent, removed := r.removeTestFunction(content, test, true)
	assert.True(t, removed)
	assert.Contains(t, newContent, "REMOVED BY QTEST")
	assert.Contains(t, newContent, "// func TestAdd")
}

func TestRemoveTestFile_DryRun(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_file.go")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	config := DefaultRemoverConfig()
	config.DryRun = true
	config.BackupBeforeRemoval = false
	r := NewRemover(config)

	result := r.RemoveTestFile(testFile)

	assert.True(t, result.Removed)
	// File should still exist because it's a dry run
	_, err = os.Stat(testFile)
	assert.NoError(t, err)
}

func TestRemoveTestFile_ActualRemoval(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_file.go")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	config := DefaultRemoverConfig()
	config.BackupBeforeRemoval = false
	r := NewRemover(config)

	result := r.RemoveTestFile(testFile)

	assert.True(t, result.Removed)
	assert.Equal(t, "file", result.Method)
	// File should be gone
	_, err = os.Stat(testFile)
	assert.True(t, os.IsNotExist(err))
}

func TestRemoveTestFile_NonExistent(t *testing.T) {
	r := NewRemover(DefaultRemoverConfig())

	result := r.RemoveTestFile("/nonexistent/path/test.go")

	assert.False(t, result.Removed)
	assert.Contains(t, result.Error, "does not exist")
}

func TestBackupFile(t *testing.T) {
	tmpDir := t.TempDir()

	config := DefaultRemoverConfig()
	config.BackupDir = filepath.Join(tmpDir, "backups")
	r := NewRemover(config)

	content := []byte("test content")
	err := r.backupFile("/some/path/test.go", content)
	require.NoError(t, err)

	// Check backup was created
	backupPath := filepath.Join(config.BackupDir, "test.go.bak")
	backupContent, err := os.ReadFile(backupPath)
	require.NoError(t, err)
	assert.Equal(t, content, backupContent)
}

func TestRemoveTests_Integration(t *testing.T) {
	// Create a temp test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "calculator_test.go")
	testContent := `package calculator

import "testing"

func TestAdd(t *testing.T) {
    result := Add(1, 2)
    if result != 3 {
        t.Error("expected 3")
    }
}

func TestSubtract(t *testing.T) {
    result := Subtract(5, 3)
    if result != 2 {
        t.Error("expected 2")
    }
}
`
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	config := DefaultRemoverConfig()
	config.BackupBeforeRemoval = false
	r := NewRemover(config)

	jobs := []MaintenanceJob{
		{Type: JobTypeRemove, TargetID: "fn-add"},
	}

	testMapping := map[string][]TestInfo{
		"fn-add": {{
			File:         testFile,
			FunctionName: "TestAdd",
			StartLine:    5,
			EndLine:      10,
		}},
	}

	results := r.RemoveTests(jobs, testMapping)

	require.Len(t, results, 1)
	assert.True(t, results[0].Removed)

	// Read the file and verify TestAdd is gone but TestSubtract remains
	newContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.NotContains(t, string(newContent), "TestAdd")
	assert.Contains(t, string(newContent), "TestSubtract")
}
