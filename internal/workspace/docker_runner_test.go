package workspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultDockerConfig(t *testing.T) {
	cfg := DefaultDockerConfig()

	if cfg.Enabled {
		t.Error("expected Enabled to be false by default")
	}

	if cfg.Timeout != 5*time.Minute {
		t.Errorf("expected timeout to be 5 minutes, got %v", cfg.Timeout)
	}

	if cfg.MemoryLimit != 512*1024*1024 {
		t.Errorf("expected memory limit to be 512MB, got %d", cfg.MemoryLimit)
	}

	if cfg.CPULimit != 1.0 {
		t.Errorf("expected CPU limit to be 1.0, got %f", cfg.CPULimit)
	}

	if !cfg.NetworkDisabled {
		t.Error("expected network to be disabled by default for security")
	}

	if cfg.WorkDir != "/workspace" {
		t.Errorf("expected work dir to be /workspace, got %s", cfg.WorkDir)
	}

	// Check all language images are configured
	languages := []string{"go", "python", "javascript", "typescript", "java"}
	for _, lang := range languages {
		if _, ok := cfg.Images[lang]; !ok {
			t.Errorf("expected image for language %s to be configured", lang)
		}
	}
}

func TestNewDockerRunner(t *testing.T) {
	t.Run("with nil config", func(t *testing.T) {
		runner := NewDockerRunner(nil)
		if runner == nil {
			t.Fatal("expected runner to be created with nil config")
		}
		if runner.config == nil {
			t.Error("expected config to be set to default")
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		cfg := &DockerConfig{
			Enabled: true,
			Timeout: 10 * time.Minute,
			Images: map[string]string{
				"go": "golang:1.22-alpine",
			},
		}
		runner := NewDockerRunner(cfg)
		if runner.config.Timeout != 10*time.Minute {
			t.Error("expected custom timeout to be preserved")
		}
	})
}

func TestDockerRunner_GetImageForLanguage(t *testing.T) {
	runner := NewDockerRunner(nil)

	tests := []struct {
		language string
		expected string
		ok       bool
	}{
		{"go", "golang:1.21-alpine", true},
		{"python", "python:3.11-alpine", true},
		{"javascript", "node:20-alpine", true},
		{"typescript", "node:20-alpine", true},
		{"java", "openjdk:17-alpine", true},
		{"rust", "", false},
		{"ruby", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			image, ok := runner.GetImageForLanguage(tt.language)
			if ok != tt.ok {
				t.Errorf("expected ok=%v, got %v", tt.ok, ok)
			}
			if ok && image != tt.expected {
				t.Errorf("expected image %s, got %s", tt.expected, image)
			}
		})
	}
}

func TestDockerRunner_SetImage(t *testing.T) {
	runner := NewDockerRunner(nil)

	runner.SetImage("rust", "rust:1.75-alpine")

	image, ok := runner.GetImageForLanguage("rust")
	if !ok {
		t.Error("expected rust image to be set")
	}
	if image != "rust:1.75-alpine" {
		t.Errorf("expected rust:1.75-alpine, got %s", image)
	}
}

func TestDockerRunner_buildTestCommand(t *testing.T) {
	runner := NewDockerRunner(nil)

	tests := []struct {
		language string
		testFile string
		repoPath string
		contains string
	}{
		{"go", "/repo/pkg/math/math_test.go", "/repo", "go test"},
		{"python", "/repo/tests/test_math.py", "/repo", "pytest"},
		{"javascript", "/repo/src/math.test.js", "/repo", "jest"},
		{"typescript", "/repo/src/math.test.ts", "/repo", "jest"},
		{"java", "/repo/src/test/MathTest.java", "/repo", "mvn test"},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			cmd := runner.buildTestCommand(tt.language, tt.testFile, tt.repoPath)
			if cmd == "" {
				t.Error("expected non-empty command")
			}
			if !containsSubstring(cmd, tt.contains) {
				t.Errorf("expected command to contain %q, got %q", tt.contains, cmd)
			}
		})
	}
}

func TestDockerRunner_buildDockerArgs(t *testing.T) {
	runner := NewDockerRunner(nil)

	args := runner.buildDockerArgs("golang:1.21-alpine", "/repo", "go test -v ./...")

	// Check essential args are present
	expectedArgs := []string{
		"run",
		"--rm",
		"-v",
		"-w",
		"--network", "none",
		"--security-opt", "no-new-privileges:true",
		"--cap-drop", "ALL",
	}

	for _, expected := range expectedArgs {
		found := false
		for _, arg := range args {
			if arg == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected args to contain %q", expected)
		}
	}
}

func TestDockerRunner_IsAvailable(t *testing.T) {
	runner := NewDockerRunner(nil)

	// Just test that the method doesn't panic
	// The result depends on whether Docker is installed
	available := runner.IsAvailable()
	t.Logf("Docker available: %v", available)
}

func TestDockerRunner_Config(t *testing.T) {
	cfg := &DockerConfig{
		Timeout: 3 * time.Minute,
	}
	runner := NewDockerRunner(cfg)

	got := runner.Config()
	if got.Timeout != cfg.Timeout {
		t.Errorf("expected Config() to return same config")
	}
}

func TestDockerRunner_buildBatchTestCommand(t *testing.T) {
	runner := NewDockerRunner(nil)

	t.Run("go batch", func(t *testing.T) {
		files := []string{
			"/repo/pkg/math/add_test.go",
			"/repo/pkg/math/sub_test.go",
			"/repo/pkg/str/format_test.go",
		}
		cmd := runner.buildBatchTestCommand("go", files, "/repo")
		if cmd == "" {
			t.Error("expected non-empty batch command for Go")
		}
		if !containsSubstring(cmd, "go test") {
			t.Error("expected Go batch command to contain 'go test'")
		}
	})

	t.Run("python batch", func(t *testing.T) {
		files := []string{
			"/repo/tests/test_math.py",
			"/repo/tests/test_str.py",
		}
		cmd := runner.buildBatchTestCommand("python", files, "/repo")
		if cmd == "" {
			t.Error("expected non-empty batch command for Python")
		}
		if !containsSubstring(cmd, "pytest") {
			t.Error("expected Python batch command to contain 'pytest'")
		}
	})
}

func TestExtractJavaTestClass(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/repo/src/test/java/MathTest.java", "MathTest"},
		{"/repo/MathServiceTest.java", "MathServiceTest"},
		{"SimpleTest.java", "SimpleTest"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractJavaTestClass(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// Integration test - only runs if Docker is available
func TestDockerRunner_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Check if Docker is available
	if err := exec.Command("docker", "version").Run(); err != nil {
		t.Skip("Docker not available, skipping integration test")
	}

	// Create a temp directory with a simple Go test
	tmpDir, err := os.MkdirTemp("", "docker-runner-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a simple Go module
	goMod := `module testmod

go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a simple Go file
	goCode := `package testmod

func Add(a, b int) int {
	return a + b
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "math.go"), []byte(goCode), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a simple test
	testCode := `package testmod

import "testing"

func TestAdd(t *testing.T) {
	result := Add(2, 3)
	if result != 5 {
		t.Errorf("expected 5, got %d", result)
	}
}
`
	testFile := filepath.Join(tmpDir, "math_test.go")
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatal(err)
	}

	// Run the test in Docker
	cfg := DefaultDockerConfig()
	cfg.Enabled = true
	cfg.NetworkDisabled = false // Need network to download deps
	cfg.Timeout = 2 * time.Minute

	runner := NewDockerRunner(cfg)

	ctx := context.Background()
	result := runner.RunTest(ctx, tmpDir, testFile, "go")

	t.Logf("Docker execution result: exit=%d, duration=%v", result.ExitCode, result.Duration)
	t.Logf("Stdout: %s", result.Stdout)
	t.Logf("Stderr: %s", result.Stderr)

	if result.Error != nil {
		t.Logf("Error: %v", result.Error)
	}

	// For CI environments where Docker might not work properly
	if result.Error != nil && containsSubstring(result.Error.Error(), "permission denied") {
		t.Skip("Docker permission denied, skipping integration test")
	}
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
