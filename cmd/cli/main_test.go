package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QTest-hq/qtest/internal/parser"
)

func TestIsSupportedExt(t *testing.T) {
	tests := []struct {
		ext  string
		want bool
	}{
		{".go", true},
		{".py", true},
		{".js", true},
		{".jsx", true},
		{".ts", true},
		{".tsx", true},
		{".java", true},
		{".rb", false},
		{".php", false},
		{".c", false},
		{".cpp", false},
		{".rs", false},
		{"", false},
		{".txt", false},
		{".md", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			got := isSupportedExt(tt.ext)
			if got != tt.want {
				t.Errorf("isSupportedExt(%s) = %v, want %v", tt.ext, got, tt.want)
			}
		})
	}
}

func TestMaskConnectionString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "postgres URL with password",
			input: "postgres://user:secretpass@localhost:5432/db",
			want:  "postgres://user:****@localhost:5432/db",
		},
		{
			name:  "postgres URL with query params",
			input: "postgres://admin:secret123@host:5432/mydb?sslmode=disable",
			want:  "postgres://admin:****@host:5432/mydb?sslmode=disable",
		},
		{
			name:  "URL without password",
			input: "redis://localhost:6379",
			want:  "redis://localhost:6379",
		},
		{
			name:  "URL with user but no password",
			input: "nats://user@localhost:4222",
			want:  "nats://user@localhost:4222",
		},
		{
			name:  "simple string no URL format",
			input: "localhost:5432",
			want:  "localhost:5432",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskConnectionString(tt.input)
			if got != tt.want {
				t.Errorf("maskConnectionString(%s) = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateFilePath_Empty(t *testing.T) {
	_, err := validateFilePath("")
	if err == nil {
		t.Error("validateFilePath('') should return error")
	}
}

func TestValidateFilePath_NonExistent(t *testing.T) {
	_, err := validateFilePath("/nonexistent/path/to/file.go")
	if err == nil {
		t.Error("validateFilePath with non-existent file should return error")
	}
}

func TestValidateDirPath_Empty(t *testing.T) {
	_, err := validateDirPath("")
	if err == nil {
		t.Error("validateDirPath('') should return error")
	}
}

func TestValidateDirPath_NonExistent(t *testing.T) {
	_, err := validateDirPath("/nonexistent/directory/path")
	if err == nil {
		t.Error("validateDirPath with non-existent directory should return error")
	}
}

func TestValidateDirPath_CurrentDir(t *testing.T) {
	// Current directory should be valid
	path, err := validateDirPath(".")
	if err != nil {
		t.Errorf("validateDirPath('.') should not error: %v", err)
	}
	if path == "" {
		t.Error("validateDirPath('.') should return non-empty path")
	}
}

func TestGetMethodIcon(t *testing.T) {
	tests := []struct {
		method string
		want   string
	}{
		{"GET", "🔵"},
		{"POST", "🟢"},
		{"PUT", "🟡"},
		{"PATCH", "🟡"},
		{"DELETE", "🔴"},
		{"OPTIONS", "⚪"},
		{"", "⚪"},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			if got := getMethodIcon(tt.method); got != tt.want {
				t.Errorf("getMethodIcon(%s) = %s, want %s", tt.method, got, tt.want)
			}
		})
	}
}

func TestGetPriorityIcon(t *testing.T) {
	tests := []struct {
		priority int
		want     string
	}{
		{100, "🔴"},
		{90, "🔴"},
		{80, "🟠"},
		{70, "🟠"},
		{60, "🟡"},
		{50, "🟡"},
		{40, "🟢"},
		{0, "🟢"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("priority_%d", tt.priority), func(t *testing.T) {
			if got := getPriorityIcon(tt.priority); got != tt.want {
				t.Errorf("getPriorityIcon(%d) = %s, want %s", tt.priority, got, tt.want)
			}
		})
	}
}

func TestFormatTargetKind(t *testing.T) {
	tests := []struct {
		kind string
		want string
	}{
		{"endpoint", "API"},
		{"function", "FN"},
		{"method", "MTH"},
		{"class", "CLS"},
		{"unknown", "unknown"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			if got := formatTargetKind(tt.kind); got != tt.want {
				t.Errorf("formatTargetKind(%s) = %s, want %s", tt.kind, got, tt.want)
			}
		})
	}
}

func TestValidateFilePath_ValidFile(t *testing.T) {
	// Create a temp file
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.go")
	if err := os.WriteFile(testFile, []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	path, err := validateFilePath(testFile)
	if err != nil {
		t.Errorf("validateFilePath() should succeed for valid file: %v", err)
	}
	if path == "" {
		t.Error("validateFilePath() should return non-empty path")
	}
}

func TestValidateFilePath_IsDirectory(t *testing.T) {
	dir := t.TempDir()
	_, err := validateFilePath(dir)
	if err == nil {
		t.Error("validateFilePath() should fail for directory")
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Errorf("error should mention 'directory', got: %v", err)
	}
}

func TestValidateDirPath_ValidDir(t *testing.T) {
	dir := t.TempDir()
	path, err := validateDirPath(dir)
	if err != nil {
		t.Errorf("validateDirPath() should succeed for valid dir: %v", err)
	}
	if path == "" {
		t.Error("validateDirPath() should return non-empty path")
	}
}

func TestValidateDirPath_IsFile(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := validateDirPath(testFile)
	if err == nil {
		t.Error("validateDirPath() should fail for file")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("error should mention 'not a directory', got: %v", err)
	}
}

func TestToModelFile(t *testing.T) {
	pf := &parser.ParsedFile{
		Path:     "/test/file.go",
		Language: parser.LanguageGo,
		Functions: []parser.Function{
			{
				ID:         "func1",
				Name:       "TestFunc",
				StartLine:  10,
				EndLine:    20,
				Parameters: []parser.Parameter{{Name: "a", Type: "int"}},
				ReturnType: "string",
				Body:       "return \"test\"",
				Comments:   "// Test function",
				Exported:   true,
				Async:      false,
				Class:      "",
			},
		},
	}

	result := toModelFile(pf)

	if result.Path != pf.Path {
		t.Errorf("Path mismatch: got %s, want %s", result.Path, pf.Path)
	}
	if result.Language != string(pf.Language) {
		t.Errorf("Language mismatch: got %s, want %s", result.Language, pf.Language)
	}
	if len(result.Functions) != 1 {
		t.Fatalf("Expected 1 function, got %d", len(result.Functions))
	}

	fn := result.Functions[0]
	if fn.Name != "TestFunc" {
		t.Errorf("Function name mismatch: got %s, want TestFunc", fn.Name)
	}
	if fn.StartLine != 10 {
		t.Errorf("StartLine mismatch: got %d, want 10", fn.StartLine)
	}
	if fn.Exported != true {
		t.Error("Exported should be true")
	}
	if len(fn.Parameters) != 1 {
		t.Fatalf("Expected 1 parameter, got %d", len(fn.Parameters))
	}
	if fn.Parameters[0].Name != "a" {
		t.Errorf("Parameter name mismatch: got %s, want a", fn.Parameters[0].Name)
	}
}

func TestToModelFile_EmptyFunctions(t *testing.T) {
	pf := &parser.ParsedFile{
		Path:      "/test/empty.go",
		Language:  parser.LanguageGo,
		Functions: []parser.Function{},
	}

	result := toModelFile(pf)

	if result.Path != pf.Path {
		t.Errorf("Path mismatch: got %s, want %s", result.Path, pf.Path)
	}
	if len(result.Functions) != 0 {
		t.Errorf("Expected 0 functions, got %d", len(result.Functions))
	}
}

func TestToModelFile_MultipleParameters(t *testing.T) {
	pf := &parser.ParsedFile{
		Path:     "/test/multi.go",
		Language: parser.LanguageGo,
		Functions: []parser.Function{
			{
				Name: "MultiParam",
				Parameters: []parser.Parameter{
					{Name: "a", Type: "int", Optional: false},
					{Name: "b", Type: "string", Default: "\"default\"", Optional: true},
					{Name: "c", Type: "bool"},
				},
			},
		},
	}

	result := toModelFile(pf)
	params := result.Functions[0].Parameters

	if len(params) != 3 {
		t.Fatalf("Expected 3 parameters, got %d", len(params))
	}
	if params[1].Default != "\"default\"" {
		t.Errorf("Default mismatch: got %s", params[1].Default)
	}
	if params[1].Optional != true {
		t.Error("Second param should be optional")
	}
}

func TestGenerateCmd(t *testing.T) {
	cmd := generateCmd()

	if cmd.Use != "generate" {
		t.Errorf("expected Use 'generate', got '%s'", cmd.Use)
	}

	// Check flags exist
	repoFlag := cmd.Flags().Lookup("repo")
	if repoFlag == nil {
		t.Error("expected --repo flag")
	}

	tierFlag := cmd.Flags().Lookup("tier")
	if tierFlag == nil {
		t.Error("expected --tier flag")
	}
	if tierFlag.DefValue != "2" {
		t.Errorf("expected tier default '2', got '%s'", tierFlag.DefValue)
	}

	maxFlag := cmd.Flags().Lookup("max")
	if maxFlag == nil {
		t.Error("expected --max flag")
	}

	dryRunFlag := cmd.Flags().Lookup("dry-run")
	if dryRunFlag == nil {
		t.Error("expected --dry-run flag")
	}

	validateFlag := cmd.Flags().Lookup("validate")
	if validateFlag == nil {
		t.Error("expected --validate flag")
	}

	mutationFlag := cmd.Flags().Lookup("mutation")
	if mutationFlag == nil {
		t.Error("expected --mutation flag")
	}
}

func TestGenerateFileCmd(t *testing.T) {
	cmd := generateFileCmd()

	if cmd.Use != "generate-file" {
		t.Errorf("expected Use 'generate-file', got '%s'", cmd.Use)
	}

	// Check flags exist
	fileFlag := cmd.Flags().Lookup("file")
	if fileFlag == nil {
		t.Error("expected --file flag")
	}

	outputFlag := cmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Error("expected --output flag")
	}

	tierFlag := cmd.Flags().Lookup("tier")
	if tierFlag == nil {
		t.Error("expected --tier flag")
	}

	writeFlag := cmd.Flags().Lookup("write")
	if writeFlag == nil {
		t.Error("expected --write flag")
	}

	irspecFlag := cmd.Flags().Lookup("irspec")
	if irspecFlag == nil {
		t.Error("expected --irspec flag")
	}
}

func TestAnalyzeCmd(t *testing.T) {
	cmd := analyzeCmd()

	if cmd.Use != "analyze" {
		t.Errorf("expected Use 'analyze', got '%s'", cmd.Use)
	}

	// Check flags
	pathFlag := cmd.Flags().Lookup("path")
	if pathFlag == nil {
		t.Error("expected --path flag")
	}
	if pathFlag.DefValue != "." {
		t.Errorf("expected path default '.', got '%s'", pathFlag.DefValue)
	}

	outputFlag := cmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Error("expected --output flag")
	}

	verboseFlag := cmd.Flags().Lookup("verbose")
	if verboseFlag == nil {
		t.Error("expected --verbose flag")
	}

	jsonFlag := cmd.Flags().Lookup("json")
	if jsonFlag == nil {
		t.Error("expected --json flag")
	}

	coverageFlag := cmd.Flags().Lookup("coverage")
	if coverageFlag == nil {
		t.Error("expected --coverage flag")
	}

	allFlag := cmd.Flags().Lookup("all")
	if allFlag == nil {
		t.Error("expected --all flag")
	}
}

func TestParseCmd(t *testing.T) {
	cmd := parseCmd()

	if cmd.Use != "parse" {
		t.Errorf("expected Use 'parse', got '%s'", cmd.Use)
	}

	fileFlag := cmd.Flags().Lookup("file")
	if fileFlag == nil {
		t.Error("expected --file flag")
	}
}

func TestConfigCmd(t *testing.T) {
	cmd := configCmd()

	if cmd.Use != "config" {
		t.Errorf("expected Use 'config', got '%s'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}
}

func TestMaskConnectionString_Complex(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "URL with port only",
			input: "http://localhost:8080",
			want:  "http://localhost:8080",
		},
		{
			name:  "mysql URL",
			input: "mysql://root:secret@127.0.0.1:3306/mydb",
			want:  "mysql://root:****@127.0.0.1:3306/mydb",
		},
		{
			name:  "simple postgres URL",
			input: "postgres://dbuser:mypassword@db.host.com:5432/database",
			want:  "postgres://dbuser:****@db.host.com:5432/database",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskConnectionString(tt.input)
			if got != tt.want {
				t.Errorf("maskConnectionString(%s) = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}
