package ci

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDefaultConfig tests default configuration for each language
func TestDefaultConfig(t *testing.T) {
	tests := []struct {
		language     Language
		wantVersion  string
		wantBuild    string
		wantTest     string
	}{
		{
			language:    LanguageGo,
			wantVersion: "1.21",
			wantBuild:   "go build ./...",
			wantTest:    "go test -v -race -coverprofile=coverage.out ./...",
		},
		{
			language:    LanguagePython,
			wantVersion: "3.11",
			wantBuild:   "pip install -r requirements.txt",
			wantTest:    "pytest --cov=. --cov-report=xml -v",
		},
		{
			language:    LanguageJavaScript,
			wantVersion: "20",
			wantBuild:   "npm ci",
			wantTest:    "npm test -- --coverage",
		},
		{
			language:    LanguageTypeScript,
			wantVersion: "20",
			wantBuild:   "npm ci",
			wantTest:    "npm test -- --coverage",
		},
		{
			language:    LanguageJava,
			wantVersion: "17",
			wantBuild:   "mvn compile",
			wantTest:    "mvn test",
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.language), func(t *testing.T) {
			cfg := DefaultConfig(tt.language)

			if cfg.Language != tt.language {
				t.Errorf("Language = %s, want %s", cfg.Language, tt.language)
			}

			if cfg.BuildCommand != tt.wantBuild {
				t.Errorf("BuildCommand = %s, want %s", cfg.BuildCommand, tt.wantBuild)
			}

			if cfg.TestCommand != tt.wantTest {
				t.Errorf("TestCommand = %s, want %s", cfg.TestCommand, tt.wantTest)
			}

			// Check version based on language
			switch tt.language {
			case LanguageGo:
				if cfg.GoVersion != tt.wantVersion {
					t.Errorf("GoVersion = %s, want %s", cfg.GoVersion, tt.wantVersion)
				}
			case LanguagePython:
				if cfg.PythonVersion != tt.wantVersion {
					t.Errorf("PythonVersion = %s, want %s", cfg.PythonVersion, tt.wantVersion)
				}
			case LanguageJavaScript, LanguageTypeScript:
				if cfg.NodeVersion != tt.wantVersion {
					t.Errorf("NodeVersion = %s, want %s", cfg.NodeVersion, tt.wantVersion)
				}
			case LanguageJava:
				if cfg.JavaVersion != tt.wantVersion {
					t.Errorf("JavaVersion = %s, want %s", cfg.JavaVersion, tt.wantVersion)
				}
			}

			// Common checks
			if !cfg.CoverageEnabled {
				t.Error("expected CoverageEnabled to be true")
			}
			if cfg.CoverageThreshold != 80.0 {
				t.Errorf("CoverageThreshold = %.1f, want 80.0", cfg.CoverageThreshold)
			}
		})
	}
}

// TestNewGenerator tests generator creation
func TestNewGenerator(t *testing.T) {
	t.Run("nil config uses default", func(t *testing.T) {
		gen := NewGenerator(nil)
		if gen == nil {
			t.Fatal("expected generator to be created")
		}
		if gen.config == nil {
			t.Error("expected config to be set")
		}
		// Default should be Go
		if gen.config.Language != LanguageGo {
			t.Errorf("Language = %s, want %s", gen.config.Language, LanguageGo)
		}
	})

	t.Run("uses provided config", func(t *testing.T) {
		cfg := &Config{
			Language:     LanguagePython,
			BuildCommand: "custom build",
		}
		gen := NewGenerator(cfg)
		if gen.config.Language != LanguagePython {
			t.Errorf("Language = %s, want %s", gen.config.Language, LanguagePython)
		}
		if gen.config.BuildCommand != "custom build" {
			t.Errorf("BuildCommand = %s, want custom build", gen.config.BuildCommand)
		}
	})
}

// TestGenerateWorkflow_GitHubActions tests GitHub Actions workflow generation
func TestGenerateWorkflow_GitHubActions(t *testing.T) {
	tests := []struct {
		name           string
		language       Language
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:     "Go workflow",
			language: LanguageGo,
			wantContains: []string{
				"name: CI",
				"runs-on: ubuntu-latest",
				"actions/checkout@v4",
				"actions/setup-go@v5",
				"go-version:",
				"go build",
				"go test",
			},
		},
		{
			name:     "Python workflow",
			language: LanguagePython,
			wantContains: []string{
				"actions/setup-python@v5",
				"python-version:",
				"pip install",
				"pytest",
			},
		},
		{
			name:     "JavaScript workflow",
			language: LanguageJavaScript,
			wantContains: []string{
				"actions/setup-node@v4",
				"node-version:",
				"npm ci",
				"npm test",
			},
		},
		{
			name:     "Java workflow",
			language: LanguageJava,
			wantContains: []string{
				"actions/setup-java@v4",
				"java-version:",
				"mvn compile",
				"mvn test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig(tt.language)
			gen := NewGenerator(cfg)

			content, err := gen.GenerateWorkflow(PlatformGitHubActions)
			if err != nil {
				t.Fatalf("GenerateWorkflow failed: %v", err)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("workflow should contain %q", want)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(content, notWant) {
					t.Errorf("workflow should not contain %q", notWant)
				}
			}
		})
	}
}

// TestGenerateWorkflow_GitLabCI tests GitLab CI workflow generation
func TestGenerateWorkflow_GitLabCI(t *testing.T) {
	cfg := DefaultConfig(LanguageGo)
	gen := NewGenerator(cfg)

	content, err := gen.GenerateWorkflow(PlatformGitLabCI)
	if err != nil {
		t.Fatalf("GenerateWorkflow failed: %v", err)
	}

	wantContains := []string{
		"stages:",
		"- build",
		"- test",
		"image: golang:",
		"go build",
		"go test",
	}

	for _, want := range wantContains {
		if !strings.Contains(content, want) {
			t.Errorf("workflow should contain %q", want)
		}
	}
}

// TestGenerateWorkflow_CircleCI tests CircleCI workflow generation
func TestGenerateWorkflow_CircleCI(t *testing.T) {
	cfg := DefaultConfig(LanguageGo)
	gen := NewGenerator(cfg)

	content, err := gen.GenerateWorkflow(PlatformCircleCI)
	if err != nil {
		t.Fatalf("GenerateWorkflow failed: %v", err)
	}

	wantContains := []string{
		"version: 2.1",
		"orbs:",
		"jobs:",
		"workflows:",
		"build-and-test",
	}

	for _, want := range wantContains {
		if !strings.Contains(content, want) {
			t.Errorf("workflow should contain %q", want)
		}
	}
}

// TestGenerateWorkflow_UnsupportedPlatform tests error for unsupported platform
func TestGenerateWorkflow_UnsupportedPlatform(t *testing.T) {
	cfg := DefaultConfig(LanguageGo)
	gen := NewGenerator(cfg)

	_, err := gen.GenerateWorkflow(Platform("unsupported"))
	if err == nil {
		t.Error("expected error for unsupported platform")
	}
}

// TestGenerateWorkflow_WithServices tests service containers
func TestGenerateWorkflow_WithServices(t *testing.T) {
	cfg := DefaultConfig(LanguageGo)
	cfg.Services = []string{"postgres", "redis"}
	gen := NewGenerator(cfg)

	content, err := gen.GenerateWorkflow(PlatformGitHubActions)
	if err != nil {
		t.Fatalf("GenerateWorkflow failed: %v", err)
	}

	wantContains := []string{
		"services:",
		"postgres:",
		"image: postgres:latest",
		"POSTGRES_PASSWORD:",
		"redis:",
	}

	for _, want := range wantContains {
		if !strings.Contains(content, want) {
			t.Errorf("workflow should contain %q", want)
		}
	}
}

// TestGenerateWorkflow_WithQTest tests QTest integration
func TestGenerateWorkflow_WithQTest(t *testing.T) {
	cfg := DefaultConfig(LanguageGo)
	cfg.QTestEnabled = true
	cfg.QTestTier = 2
	cfg.QTestMaxTests = 10
	gen := NewGenerator(cfg)

	content, err := gen.GenerateWorkflow(PlatformGitHubActions)
	if err != nil {
		t.Fatalf("GenerateWorkflow failed: %v", err)
	}

	wantContains := []string{
		"Generate tests with QTest",
		"qtest generate-file",
		"-t 2",
		"-m 10",
	}

	for _, want := range wantContains {
		if !strings.Contains(content, want) {
			t.Errorf("workflow should contain %q", want)
		}
	}
}

// TestWriteWorkflow tests writing workflow to file
func TestWriteWorkflow(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ci-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig(LanguageGo)
	gen := NewGenerator(cfg)

	t.Run("GitHub Actions", func(t *testing.T) {
		outputPath, err := gen.WriteWorkflow(PlatformGitHubActions, tmpDir)
		if err != nil {
			t.Fatalf("WriteWorkflow failed: %v", err)
		}

		expectedPath := filepath.Join(tmpDir, ".github", "workflows", "ci.yml")
		if outputPath != expectedPath {
			t.Errorf("outputPath = %s, want %s", outputPath, expectedPath)
		}

		// Verify file exists
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			t.Error("expected workflow file to exist")
		}
	})

	t.Run("GitLab CI", func(t *testing.T) {
		outputPath, err := gen.WriteWorkflow(PlatformGitLabCI, tmpDir)
		if err != nil {
			t.Fatalf("WriteWorkflow failed: %v", err)
		}

		expectedPath := filepath.Join(tmpDir, ".gitlab-ci.yml")
		if outputPath != expectedPath {
			t.Errorf("outputPath = %s, want %s", outputPath, expectedPath)
		}

		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			t.Error("expected workflow file to exist")
		}
	})

	t.Run("CircleCI", func(t *testing.T) {
		outputPath, err := gen.WriteWorkflow(PlatformCircleCI, tmpDir)
		if err != nil {
			t.Fatalf("WriteWorkflow failed: %v", err)
		}

		expectedPath := filepath.Join(tmpDir, ".circleci", "config.yml")
		if outputPath != expectedPath {
			t.Errorf("outputPath = %s, want %s", outputPath, expectedPath)
		}

		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			t.Error("expected workflow file to exist")
		}
	})
}

// TestDetectConfig tests project detection
func TestDetectConfig(t *testing.T) {
	t.Run("Go project", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ci-go-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// Create go.mod
		goMod := filepath.Join(tmpDir, "go.mod")
		os.WriteFile(goMod, []byte("module test\n\ngo 1.21"), 0644)

		cfg, err := DetectConfig(tmpDir)
		if err != nil {
			t.Fatalf("DetectConfig failed: %v", err)
		}

		if cfg.Language != LanguageGo {
			t.Errorf("Language = %s, want %s", cfg.Language, LanguageGo)
		}
	})

	t.Run("Python project", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ci-py-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// Create requirements.txt
		reqTxt := filepath.Join(tmpDir, "requirements.txt")
		os.WriteFile(reqTxt, []byte("flask==2.0\n"), 0644)

		cfg, err := DetectConfig(tmpDir)
		if err != nil {
			t.Fatalf("DetectConfig failed: %v", err)
		}

		if cfg.Language != LanguagePython {
			t.Errorf("Language = %s, want %s", cfg.Language, LanguagePython)
		}
		if cfg.Framework != "flask" {
			t.Errorf("Framework = %s, want flask", cfg.Framework)
		}
	})

	t.Run("JavaScript project", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ci-js-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// Create package.json
		pkgJson := filepath.Join(tmpDir, "package.json")
		os.WriteFile(pkgJson, []byte(`{"dependencies": {"express": "4.0"}}`), 0644)

		cfg, err := DetectConfig(tmpDir)
		if err != nil {
			t.Fatalf("DetectConfig failed: %v", err)
		}

		if cfg.Language != LanguageJavaScript {
			t.Errorf("Language = %s, want %s", cfg.Language, LanguageJavaScript)
		}
		if cfg.Framework != "express" {
			t.Errorf("Framework = %s, want express", cfg.Framework)
		}
	})

	t.Run("TypeScript project", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ci-ts-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// Create package.json and tsconfig.json
		pkgJson := filepath.Join(tmpDir, "package.json")
		os.WriteFile(pkgJson, []byte(`{"dependencies": {}}`), 0644)
		tsConfig := filepath.Join(tmpDir, "tsconfig.json")
		os.WriteFile(tsConfig, []byte(`{}`), 0644)

		cfg, err := DetectConfig(tmpDir)
		if err != nil {
			t.Fatalf("DetectConfig failed: %v", err)
		}

		if cfg.Language != LanguageTypeScript {
			t.Errorf("Language = %s, want %s", cfg.Language, LanguageTypeScript)
		}
	})

	t.Run("Java project", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ci-java-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// Create pom.xml
		pomXml := filepath.Join(tmpDir, "pom.xml")
		os.WriteFile(pomXml, []byte(`<project><groupId>test</groupId></project>`), 0644)

		cfg, err := DetectConfig(tmpDir)
		if err != nil {
			t.Fatalf("DetectConfig failed: %v", err)
		}

		if cfg.Language != LanguageJava {
			t.Errorf("Language = %s, want %s", cfg.Language, LanguageJava)
		}
	})
}

// TestDetectServices tests service detection from docker-compose
func TestDetectServices(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ci-svc-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create go.mod and docker-compose.yml
	goMod := filepath.Join(tmpDir, "go.mod")
	os.WriteFile(goMod, []byte("module test"), 0644)

	compose := filepath.Join(tmpDir, "docker-compose.yml")
	os.WriteFile(compose, []byte(`
services:
  postgres:
    image: postgres:15
  redis:
    image: redis:7
`), 0644)

	cfg, err := DetectConfig(tmpDir)
	if err != nil {
		t.Fatalf("DetectConfig failed: %v", err)
	}

	if len(cfg.Services) != 2 {
		t.Errorf("expected 2 services, got %d", len(cfg.Services))
	}

	hasPostgres := false
	hasRedis := false
	for _, svc := range cfg.Services {
		if svc == "postgres" {
			hasPostgres = true
		}
		if svc == "redis" {
			hasRedis = true
		}
	}

	if !hasPostgres {
		t.Error("expected postgres service to be detected")
	}
	if !hasRedis {
		t.Error("expected redis service to be detected")
	}
}

// TestSupportedPlatforms tests supported platforms list
func TestSupportedPlatforms(t *testing.T) {
	platforms := SupportedPlatforms()

	if len(platforms) != 3 {
		t.Errorf("expected 3 platforms, got %d", len(platforms))
	}

	expected := map[Platform]bool{
		PlatformGitHubActions: true,
		PlatformGitLabCI:      true,
		PlatformCircleCI:      true,
	}

	for _, p := range platforms {
		if !expected[p] {
			t.Errorf("unexpected platform: %s", p)
		}
	}
}

// TestSupportedLanguages tests supported languages list
func TestSupportedLanguages(t *testing.T) {
	languages := SupportedLanguages()

	if len(languages) != 5 {
		t.Errorf("expected 5 languages, got %d", len(languages))
	}

	expected := map[Language]bool{
		LanguageGo:         true,
		LanguagePython:     true,
		LanguageJavaScript: true,
		LanguageTypeScript: true,
		LanguageJava:       true,
	}

	for _, l := range languages {
		if !expected[l] {
			t.Errorf("unexpected language: %s", l)
		}
	}
}

// TestConfig_CustomBranches tests custom branch configuration
func TestConfig_CustomBranches(t *testing.T) {
	cfg := DefaultConfig(LanguageGo)
	cfg.Branches = []string{"develop", "release/*"}
	gen := NewGenerator(cfg)

	content, err := gen.GenerateWorkflow(PlatformGitHubActions)
	if err != nil {
		t.Fatalf("GenerateWorkflow failed: %v", err)
	}

	if !strings.Contains(content, "develop") {
		t.Error("workflow should contain custom branch 'develop'")
	}
	if !strings.Contains(content, "release/*") {
		t.Error("workflow should contain custom branch 'release/*'")
	}
}
