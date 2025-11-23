package workspace

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// DockerRunner provides sandboxed test execution in Docker containers
type DockerRunner struct {
	config *DockerConfig
}

// DockerConfig holds Docker runner configuration
type DockerConfig struct {
	// Enabled indicates if Docker execution should be used
	Enabled bool

	// Timeout for container execution
	Timeout time.Duration

	// MemoryLimit in bytes (e.g., 512MB = 512 * 1024 * 1024)
	MemoryLimit int64

	// CPULimit as number of CPUs (e.g., 1.0 = 1 CPU)
	CPULimit float64

	// NetworkDisabled disables network access in containers
	NetworkDisabled bool

	// Images maps language to Docker image
	Images map[string]string

	// WorkDir inside the container
	WorkDir string
}

// DefaultDockerConfig returns sensible defaults for Docker execution
func DefaultDockerConfig() *DockerConfig {
	return &DockerConfig{
		Enabled:         false,
		Timeout:         5 * time.Minute,
		MemoryLimit:     512 * 1024 * 1024, // 512MB
		CPULimit:        1.0,
		NetworkDisabled: true, // No network by default for security
		WorkDir:         "/workspace",
		Images: map[string]string{
			"go":         "golang:1.21-alpine",
			"python":     "python:3.11-alpine",
			"javascript": "node:20-alpine",
			"typescript": "node:20-alpine",
			"java":       "openjdk:17-alpine",
		},
	}
}

// NewDockerRunner creates a new Docker runner
func NewDockerRunner(config *DockerConfig) *DockerRunner {
	if config == nil {
		config = DefaultDockerConfig()
	}
	return &DockerRunner{config: config}
}

// IsAvailable checks if Docker is available on the system
func (d *DockerRunner) IsAvailable() bool {
	cmd := exec.Command("docker", "version")
	return cmd.Run() == nil
}

// DockerExecResult holds the result of a Docker execution
type DockerExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
	Error    error
}

// RunTest executes a test in a Docker container
func (d *DockerRunner) RunTest(ctx context.Context, repoPath, testFile, language string) *DockerExecResult {
	startTime := time.Now()
	result := &DockerExecResult{}

	// Get image for language
	image, ok := d.config.Images[language]
	if !ok {
		result.Error = fmt.Errorf("no Docker image configured for language: %s", language)
		return result
	}

	// Build test command based on language
	testCmd := d.buildTestCommand(language, testFile, repoPath)
	if testCmd == "" {
		result.Error = fmt.Errorf("unable to build test command for language: %s", language)
		return result
	}

	// Build Docker run command
	dockerArgs := d.buildDockerArgs(image, repoPath, testCmd)

	log.Debug().
		Str("image", image).
		Str("testFile", testFile).
		Strs("args", dockerArgs).
		Msg("running test in Docker container")

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	// Execute Docker command
	cmd := exec.CommandContext(execCtx, "docker", dockerArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result.Duration = time.Since(startTime)
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else if execCtx.Err() == context.DeadlineExceeded {
			result.Error = fmt.Errorf("test execution timed out after %v", d.config.Timeout)
			result.ExitCode = -1
		} else {
			result.Error = err
			result.ExitCode = -1
		}
	}

	return result
}

// buildDockerArgs constructs the Docker run command arguments
func (d *DockerRunner) buildDockerArgs(image, repoPath, testCmd string) []string {
	args := []string{
		"run",
		"--rm",
		"-v", fmt.Sprintf("%s:%s:ro", repoPath, d.config.WorkDir),
		"-w", d.config.WorkDir,
	}

	// Memory limit
	if d.config.MemoryLimit > 0 {
		args = append(args, "--memory", fmt.Sprintf("%d", d.config.MemoryLimit))
	}

	// CPU limit
	if d.config.CPULimit > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%.2f", d.config.CPULimit))
	}

	// Network isolation
	if d.config.NetworkDisabled {
		args = append(args, "--network", "none")
	}

	// Security options
	args = append(args,
		"--security-opt", "no-new-privileges:true",
		"--cap-drop", "ALL",
	)

	// Image and command
	args = append(args, image, "sh", "-c", testCmd)

	return args
}

// buildTestCommand constructs the test command for a specific language
func (d *DockerRunner) buildTestCommand(language, testFile, repoPath string) string {
	// Get relative path within workspace
	relPath, err := filepath.Rel(repoPath, testFile)
	if err != nil {
		relPath = testFile
	}

	switch language {
	case "go":
		// For Go, we need to run go test on the package
		dir := filepath.Dir(relPath)
		return fmt.Sprintf("cd %s && go test -v -json ./%s", d.config.WorkDir, dir)

	case "python":
		return fmt.Sprintf("cd %s && python -m pytest -v %s", d.config.WorkDir, relPath)

	case "javascript", "typescript":
		// Check for Jest configuration
		return fmt.Sprintf("cd %s && npx jest --verbose %s", d.config.WorkDir, relPath)

	case "java":
		// For Java/Maven projects
		return fmt.Sprintf("cd %s && mvn test -Dtest=%s", d.config.WorkDir, extractJavaTestClass(testFile))

	default:
		return ""
	}
}

// extractJavaTestClass extracts the Java test class name from a file path
func extractJavaTestClass(testFile string) string {
	base := filepath.Base(testFile)
	return strings.TrimSuffix(base, ".java")
}

// RunBatchTests executes multiple tests in a single container (more efficient)
func (d *DockerRunner) RunBatchTests(ctx context.Context, repoPath, language string, testFiles []string) *DockerExecResult {
	startTime := time.Now()
	result := &DockerExecResult{}

	if len(testFiles) == 0 {
		result.Error = fmt.Errorf("no test files provided")
		return result
	}

	image, ok := d.config.Images[language]
	if !ok {
		result.Error = fmt.Errorf("no Docker image configured for language: %s", language)
		return result
	}

	// Build batch test command
	testCmd := d.buildBatchTestCommand(language, testFiles, repoPath)
	if testCmd == "" {
		result.Error = fmt.Errorf("unable to build batch test command for language: %s", language)
		return result
	}

	dockerArgs := d.buildDockerArgs(image, repoPath, testCmd)

	log.Debug().
		Str("image", image).
		Int("testCount", len(testFiles)).
		Msg("running batch tests in Docker container")

	execCtx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "docker", dockerArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result.Duration = time.Since(startTime)
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else if execCtx.Err() == context.DeadlineExceeded {
			result.Error = fmt.Errorf("batch test execution timed out after %v", d.config.Timeout)
			result.ExitCode = -1
		} else {
			result.Error = err
			result.ExitCode = -1
		}
	}

	return result
}

// buildBatchTestCommand constructs a command to run multiple tests
func (d *DockerRunner) buildBatchTestCommand(language string, testFiles []string, repoPath string) string {
	switch language {
	case "go":
		// Go can test entire packages
		packages := make(map[string]bool)
		for _, tf := range testFiles {
			relPath, _ := filepath.Rel(repoPath, tf)
			dir := filepath.Dir(relPath)
			packages["./"+dir] = true
		}
		pkgList := make([]string, 0, len(packages))
		for pkg := range packages {
			pkgList = append(pkgList, pkg)
		}
		return fmt.Sprintf("cd %s && go test -v -json %s", d.config.WorkDir, strings.Join(pkgList, " "))

	case "python":
		relPaths := make([]string, len(testFiles))
		for i, tf := range testFiles {
			relPaths[i], _ = filepath.Rel(repoPath, tf)
		}
		return fmt.Sprintf("cd %s && python -m pytest -v %s", d.config.WorkDir, strings.Join(relPaths, " "))

	case "javascript", "typescript":
		relPaths := make([]string, len(testFiles))
		for i, tf := range testFiles {
			relPaths[i], _ = filepath.Rel(repoPath, tf)
		}
		return fmt.Sprintf("cd %s && npx jest --verbose %s", d.config.WorkDir, strings.Join(relPaths, " "))

	default:
		return ""
	}
}

// PullImage ensures the Docker image is available locally
func (d *DockerRunner) PullImage(ctx context.Context, language string) error {
	image, ok := d.config.Images[language]
	if !ok {
		return fmt.Errorf("no Docker image configured for language: %s", language)
	}

	log.Info().Str("image", image).Msg("pulling Docker image")

	cmd := exec.CommandContext(ctx, "docker", "pull", image)
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// EnsureImages pulls all configured images
func (d *DockerRunner) EnsureImages(ctx context.Context) error {
	for lang := range d.config.Images {
		if err := d.PullImage(ctx, lang); err != nil {
			log.Warn().Err(err).Str("language", lang).Msg("failed to pull image")
		}
	}
	return nil
}

// CleanupContainers removes any stale containers from previous runs
func (d *DockerRunner) CleanupContainers(ctx context.Context) error {
	// List containers with qtest label
	cmd := exec.CommandContext(ctx, "docker", "ps", "-a", "-q", "--filter", "label=qtest=true")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	containers := strings.Fields(string(output))
	if len(containers) == 0 {
		return nil
	}

	// Remove containers
	args := append([]string{"rm", "-f"}, containers...)
	cmd = exec.CommandContext(ctx, "docker", args...)
	return cmd.Run()
}

// GetImageForLanguage returns the configured image for a language
func (d *DockerRunner) GetImageForLanguage(language string) (string, bool) {
	image, ok := d.config.Images[language]
	return image, ok
}

// SetImage sets the Docker image for a specific language
func (d *DockerRunner) SetImage(language, image string) {
	d.config.Images[language] = image
}

// Config returns the Docker configuration
func (d *DockerRunner) Config() *DockerConfig {
	return d.config
}
