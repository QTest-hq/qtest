// Package maintenance provides test maintenance functionality.
// This file implements the test remover for cleaning up obsolete tests.
package maintenance

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// RemovalResult represents the result of removing a test
type RemovalResult struct {
	TargetID     string `json:"target_id"`
	TestFile     string `json:"test_file"`
	TestFunction string `json:"test_function,omitempty"`
	Removed      bool   `json:"removed"`
	Error        string `json:"error,omitempty"`
	Method       string `json:"method"` // "file", "function", "comment"
}

// RemoverConfig configures the test remover behavior
type RemoverConfig struct {
	// DryRun if true, doesn't actually delete files
	DryRun bool
	// CommentOutInsteadOfDelete if true, comments out tests instead of deleting
	CommentOutInsteadOfDelete bool
	// BackupBeforeRemoval if true, creates backup before removal
	BackupBeforeRemoval bool
	// BackupDir is the directory for backups
	BackupDir string
}

// DefaultRemoverConfig returns sensible defaults
func DefaultRemoverConfig() RemoverConfig {
	return RemoverConfig{
		DryRun:                    false,
		CommentOutInsteadOfDelete: false,
		BackupBeforeRemoval:       true,
		BackupDir:                 ".qtest/backups",
	}
}

// Remover handles removal of obsolete tests
type Remover struct {
	config RemoverConfig
}

// NewRemover creates a new test remover
func NewRemover(config RemoverConfig) *Remover {
	return &Remover{config: config}
}

// RemoveTests processes removal jobs and removes associated tests
func (r *Remover) RemoveTests(jobs []MaintenanceJob, testMapping map[string][]TestInfo) []RemovalResult {
	var results []RemovalResult

	for _, job := range jobs {
		if job.Type != JobTypeRemove {
			continue
		}

		// Find tests associated with this target
		tests, ok := testMapping[job.TargetID]
		if !ok || len(tests) == 0 {
			results = append(results, RemovalResult{
				TargetID: job.TargetID,
				Removed:  false,
				Error:    "no tests found for target",
				Method:   "none",
			})
			continue
		}

		// Remove each associated test
		for _, test := range tests {
			result := r.removeTest(job.TargetID, test)
			results = append(results, result)
		}
	}

	return results
}

// TestInfo represents information about a test
type TestInfo struct {
	File         string `json:"file"`
	FunctionName string `json:"function_name"`
	StartLine    int    `json:"start_line"`
	EndLine      int    `json:"end_line"`
}

// removeTest removes a single test
func (r *Remover) removeTest(targetID string, test TestInfo) RemovalResult {
	result := RemovalResult{
		TargetID:     targetID,
		TestFile:     test.File,
		TestFunction: test.FunctionName,
		Method:       "function",
	}

	// Check if file exists
	if _, err := os.Stat(test.File); os.IsNotExist(err) {
		result.Error = "test file does not exist"
		return result
	}

	// Read the file
	content, err := os.ReadFile(test.File)
	if err != nil {
		result.Error = fmt.Sprintf("failed to read file: %v", err)
		return result
	}

	// Backup if configured
	if r.config.BackupBeforeRemoval && !r.config.DryRun {
		if err := r.backupFile(test.File, content); err != nil {
			result.Error = fmt.Sprintf("failed to backup: %v", err)
			return result
		}
	}

	// Try to remove the specific test function
	newContent, removed := r.removeTestFunction(string(content), test, r.config.CommentOutInsteadOfDelete)
	if !removed {
		result.Error = "could not locate test function in file"
		return result
	}

	// Check if file is now empty of tests
	if r.isTestFileEmpty(newContent) {
		result.Method = "file"
		if !r.config.DryRun {
			if err := os.Remove(test.File); err != nil {
				result.Error = fmt.Sprintf("failed to remove file: %v", err)
				return result
			}
		}
		result.Removed = true
		return result
	}

	// Write updated content
	if !r.config.DryRun {
		if err := os.WriteFile(test.File, []byte(newContent), 0644); err != nil {
			result.Error = fmt.Sprintf("failed to write file: %v", err)
			return result
		}
	}

	if r.config.CommentOutInsteadOfDelete {
		result.Method = "comment"
	}
	result.Removed = true
	return result
}

// removeTestFunction removes or comments out a test function from content
func (r *Remover) removeTestFunction(content string, test TestInfo, commentOut bool) (string, bool) {
	lines := strings.Split(content, "\n")

	// If we have line numbers, use them
	if test.StartLine > 0 && test.EndLine > 0 && test.StartLine <= len(lines) && test.EndLine <= len(lines) {
		if commentOut {
			return r.commentOutLines(lines, test.StartLine-1, test.EndLine-1), true
		}
		return r.removeLines(lines, test.StartLine-1, test.EndLine-1), true
	}

	// Otherwise, try to find the function by name
	startLine, endLine := r.findTestFunction(lines, test.FunctionName)
	if startLine == -1 {
		return content, false
	}

	if commentOut {
		return r.commentOutLines(lines, startLine, endLine), true
	}
	return r.removeLines(lines, startLine, endLine), true
}

// findTestFunction finds the start and end lines of a test function
func (r *Remover) findTestFunction(lines []string, funcName string) (int, int) {
	// Patterns for different languages
	patterns := []string{
		fmt.Sprintf(`func\s+%s\s*\(`, regexp.QuoteMeta(funcName)),           // Go
		fmt.Sprintf(`def\s+%s\s*\(`, regexp.QuoteMeta(funcName)),            // Python
		fmt.Sprintf(`(it|test|describe)\s*\(\s*['"]%s`, regexp.QuoteMeta(funcName)), // Jest
		fmt.Sprintf(`void\s+%s\s*\(`, regexp.QuoteMeta(funcName)),           // Java/JUnit
	}

	startLine := -1
	for i, line := range lines {
		for _, pattern := range patterns {
			matched, _ := regexp.MatchString(pattern, line)
			if matched {
				startLine = i
				break
			}
		}
		if startLine != -1 {
			break
		}
	}

	if startLine == -1 {
		return -1, -1
	}

	// Find the end of the function (track braces/indentation)
	endLine := r.findFunctionEnd(lines, startLine)
	return startLine, endLine
}

// findFunctionEnd finds where a function ends
func (r *Remover) findFunctionEnd(lines []string, startLine int) int {
	braceCount := 0
	inFunction := false

	for i := startLine; i < len(lines); i++ {
		line := lines[i]

		// Count braces
		for _, ch := range line {
			if ch == '{' {
				braceCount++
				inFunction = true
			} else if ch == '}' {
				braceCount--
				if inFunction && braceCount == 0 {
					return i
				}
			}
		}
	}

	// If no braces found (Python-style), find by indentation
	if !inFunction && startLine < len(lines) {
		baseIndent := r.getIndentation(lines[startLine])
		for i := startLine + 1; i < len(lines); i++ {
			line := lines[i]
			if strings.TrimSpace(line) == "" {
				continue
			}
			currentIndent := r.getIndentation(line)
			if currentIndent <= baseIndent && strings.TrimSpace(line) != "" {
				return i - 1
			}
		}
		return len(lines) - 1
	}

	return len(lines) - 1
}

// getIndentation returns the indentation level of a line
func (r *Remover) getIndentation(line string) int {
	count := 0
	for _, ch := range line {
		if ch == ' ' {
			count++
		} else if ch == '\t' {
			count += 4
		} else {
			break
		}
	}
	return count
}

// removeLines removes lines from start to end (inclusive)
func (r *Remover) removeLines(lines []string, start, end int) string {
	if start < 0 || end >= len(lines) || start > end {
		return strings.Join(lines, "\n")
	}

	// Also remove any preceding blank lines and decorators
	for start > 0 {
		prevLine := strings.TrimSpace(lines[start-1])
		if prevLine == "" || strings.HasPrefix(prevLine, "@") || strings.HasPrefix(prevLine, "#") {
			start--
		} else {
			break
		}
	}

	newLines := make([]string, 0, len(lines)-(end-start+1))
	newLines = append(newLines, lines[:start]...)
	if end+1 < len(lines) {
		newLines = append(newLines, lines[end+1:]...)
	}

	return strings.Join(newLines, "\n")
}

// commentOutLines comments out lines from start to end
func (r *Remover) commentOutLines(lines []string, start, end int) string {
	if start < 0 || end >= len(lines) || start > end {
		return strings.Join(lines, "\n")
	}

	// Add a marker comment before
	lines[start] = "// REMOVED BY QTEST: Test for deleted function\n// " + lines[start]

	for i := start + 1; i <= end; i++ {
		lines[i] = "// " + lines[i]
	}

	return strings.Join(lines, "\n")
}

// isTestFileEmpty checks if a test file has no remaining tests
func (r *Remover) isTestFileEmpty(content string) bool {
	// Check for common test patterns
	testPatterns := []string{
		`func\s+Test\w+`,     // Go
		`def\s+test_\w+`,     // Python
		`it\s*\(`,            // Jest
		`@Test`,              // JUnit
	}

	for _, pattern := range testPatterns {
		matched, _ := regexp.MatchString(pattern, content)
		if matched {
			return false
		}
	}

	return true
}

// backupFile creates a backup of a file
func (r *Remover) backupFile(filePath string, content []byte) error {
	if r.config.BackupDir == "" {
		return nil
	}

	// Create backup directory if needed
	if err := os.MkdirAll(r.config.BackupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup dir: %w", err)
	}

	// Generate backup filename
	baseName := filepath.Base(filePath)
	backupPath := filepath.Join(r.config.BackupDir, baseName+".bak")

	return os.WriteFile(backupPath, content, 0644)
}

// RemoveTestFile removes an entire test file
func (r *Remover) RemoveTestFile(filePath string) RemovalResult {
	result := RemovalResult{
		TestFile: filePath,
		Method:   "file",
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		result.Error = "file does not exist"
		return result
	}

	// Read and backup
	if r.config.BackupBeforeRemoval && !r.config.DryRun {
		content, err := os.ReadFile(filePath)
		if err != nil {
			result.Error = fmt.Sprintf("failed to read for backup: %v", err)
			return result
		}
		if err := r.backupFile(filePath, content); err != nil {
			result.Error = fmt.Sprintf("failed to backup: %v", err)
			return result
		}
	}

	if !r.config.DryRun {
		if err := os.Remove(filePath); err != nil {
			result.Error = fmt.Sprintf("failed to remove: %v", err)
			return result
		}
	}

	result.Removed = true
	return result
}
