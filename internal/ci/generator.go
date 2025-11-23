package ci

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// Platform represents a CI/CD platform
type Platform string

const (
	PlatformGitHubActions Platform = "github-actions"
	PlatformGitLabCI      Platform = "gitlab-ci"
	PlatformCircleCI      Platform = "circleci"
)

// Language represents a programming language
type Language string

const (
	LanguageGo         Language = "go"
	LanguagePython     Language = "python"
	LanguageJavaScript Language = "javascript"
	LanguageTypeScript Language = "typescript"
	LanguageJava       Language = "java"
)

// Config holds CI workflow generation configuration
type Config struct {
	// Project settings
	ProjectName    string   `json:"project_name"`
	Language       Language `json:"language"`
	Framework      string   `json:"framework,omitempty"`

	// Build settings
	BuildCommand   string   `json:"build_command,omitempty"`
	TestCommand    string   `json:"test_command,omitempty"`
	LintCommand    string   `json:"lint_command,omitempty"`

	// Coverage settings
	CoverageEnabled   bool    `json:"coverage_enabled"`
	CoverageThreshold float64 `json:"coverage_threshold"`

	// Test generation settings
	QTestEnabled      bool   `json:"qtest_enabled"`
	QTestTier         int    `json:"qtest_tier"`
	QTestMaxTests     int    `json:"qtest_max_tests"`

	// Platform-specific
	NodeVersion       string `json:"node_version,omitempty"`
	GoVersion         string `json:"go_version,omitempty"`
	PythonVersion     string `json:"python_version,omitempty"`
	JavaVersion       string `json:"java_version,omitempty"`

	// Branches
	Branches          []string `json:"branches,omitempty"`

	// Additional services
	Services          []string `json:"services,omitempty"` // postgres, redis, etc.
}

// DefaultConfig returns default configuration based on language
func DefaultConfig(language Language) *Config {
	cfg := &Config{
		Language:          language,
		CoverageEnabled:   true,
		CoverageThreshold: 80.0,
		QTestEnabled:      false,
		QTestTier:         2,
		QTestMaxTests:     10,
		Branches:          []string{"main", "master"},
	}

	switch language {
	case LanguageGo:
		cfg.GoVersion = "1.21"
		cfg.BuildCommand = "go build ./..."
		cfg.TestCommand = "go test -v -race -coverprofile=coverage.out ./..."
		cfg.LintCommand = "golangci-lint run"
	case LanguagePython:
		cfg.PythonVersion = "3.11"
		cfg.BuildCommand = "pip install -r requirements.txt"
		cfg.TestCommand = "pytest --cov=. --cov-report=xml -v"
		cfg.LintCommand = "ruff check ."
	case LanguageJavaScript, LanguageTypeScript:
		cfg.NodeVersion = "20"
		cfg.BuildCommand = "npm ci"
		cfg.TestCommand = "npm test -- --coverage"
		cfg.LintCommand = "npm run lint"
	case LanguageJava:
		cfg.JavaVersion = "17"
		cfg.BuildCommand = "mvn compile"
		cfg.TestCommand = "mvn test"
		cfg.LintCommand = "mvn checkstyle:check"
	}

	return cfg
}

// Generator generates CI workflow files
type Generator struct {
	config *Config
}

// NewGenerator creates a new CI workflow generator
func NewGenerator(config *Config) *Generator {
	if config == nil {
		config = DefaultConfig(LanguageGo)
	}
	return &Generator{config: config}
}

// GenerateWorkflow generates a CI workflow for the specified platform
func (g *Generator) GenerateWorkflow(platform Platform) (string, error) {
	switch platform {
	case PlatformGitHubActions:
		return g.generateGitHubActions()
	case PlatformGitLabCI:
		return g.generateGitLabCI()
	case PlatformCircleCI:
		return g.generateCircleCI()
	default:
		return "", fmt.Errorf("unsupported platform: %s", platform)
	}
}

// WriteWorkflow writes the CI workflow to the appropriate location
func (g *Generator) WriteWorkflow(platform Platform, projectDir string) (string, error) {
	content, err := g.GenerateWorkflow(platform)
	if err != nil {
		return "", err
	}

	var outputPath string
	switch platform {
	case PlatformGitHubActions:
		outputPath = filepath.Join(projectDir, ".github", "workflows", "ci.yml")
	case PlatformGitLabCI:
		outputPath = filepath.Join(projectDir, ".gitlab-ci.yml")
	case PlatformCircleCI:
		outputPath = filepath.Join(projectDir, ".circleci", "config.yml")
	}

	// Create directory if needed
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write workflow: %w", err)
	}

	return outputPath, nil
}

// generateGitHubActions generates a GitHub Actions workflow
func (g *Generator) generateGitHubActions() (string, error) {
	tmpl := `name: CI

on:
  push:
    branches: [{{.BranchList}}]
  pull_request:
    branches: [{{.BranchList}}]

jobs:
  test:
    runs-on: ubuntu-latest
{{if .Services}}
    services:
{{range .Services}}      {{.}}:
        image: {{.}}:latest
        {{if eq . "postgres"}}env:
          POSTGRES_PASSWORD: postgres
          POSTGRES_DB: test
        ports:
          - 5432:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5{{end}}{{if eq . "redis"}}ports:
          - 6379:6379{{end}}
{{end}}{{end}}
    steps:
      - uses: actions/checkout@v4
{{.SetupStep}}
      - name: Install dependencies
        run: {{.BuildCommand}}

      - name: Lint
        run: {{.LintCommand}}
        continue-on-error: true

      - name: Run tests
        run: {{.TestCommand}}
{{if .CoverageEnabled}}
      - name: Check coverage
        run: |
          {{.CoverageCheck}}{{end}}
{{if .QTestEnabled}}
      - name: Generate tests with QTest
        run: |
          curl -sSL https://qtest.dev/install.sh | bash
          qtest generate-file -d . -t {{.QTestTier}} -m {{.QTestMaxTests}} --write
{{end}}
`

	data := g.prepareTemplateData()

	t, err := template.New("github-actions").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// generateGitLabCI generates a GitLab CI configuration
func (g *Generator) generateGitLabCI() (string, error) {
	tmpl := `stages:
  - build
  - test
  - coverage
{{if .QTestEnabled}}  - generate{{end}}

variables:
{{.Variables}}

{{if .Services}}services:
{{range .Services}}  - {{.}}:latest
{{end}}{{end}}

build:
  stage: build
  {{.ImageLine}}
  script:
    - {{.BuildCommand}}
{{if .CacheConfig}}  cache:
{{.CacheConfig}}{{end}}

lint:
  stage: build
  {{.ImageLine}}
  script:
    - {{.LintCommand}}
  allow_failure: true

test:
  stage: test
  {{.ImageLine}}
  script:
    - {{.TestCommand}}
{{if .CoverageEnabled}}  coverage: '/coverage: \d+\.\d+%%/'
  artifacts:
    reports:
      coverage_report:
        coverage_format: {{.CoverageFormat}}
        path: {{.CoveragePath}}{{end}}
{{if .QTestEnabled}}
generate-tests:
  stage: generate
  {{.ImageLine}}
  script:
    - curl -sSL https://qtest.dev/install.sh | bash
    - qtest generate-file -d . -t {{.QTestTier}} -m {{.QTestMaxTests}} --write
  only:
    - merge_requests
{{end}}
`

	data := g.prepareGitLabData()

	t, err := template.New("gitlab-ci").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// generateCircleCI generates a CircleCI configuration
func (g *Generator) generateCircleCI() (string, error) {
	tmpl := `version: 2.1

orbs:
{{.Orbs}}

jobs:
  build-and-test:
    {{.Executor}}
    steps:
      - checkout
{{.SetupSteps}}
      - run:
          name: Install dependencies
          command: {{.BuildCommand}}
      - run:
          name: Lint
          command: {{.LintCommand}}
      - run:
          name: Run tests
          command: {{.TestCommand}}
{{if .CoverageEnabled}}      - store_artifacts:
          path: {{.CoveragePath}}
          destination: coverage{{end}}
{{if .QTestEnabled}}
  generate-tests:
    {{.Executor}}
    steps:
      - checkout
      - run:
          name: Install QTest
          command: curl -sSL https://qtest.dev/install.sh | bash
      - run:
          name: Generate tests
          command: qtest generate-file -d . -t {{.QTestTier}} -m {{.QTestMaxTests}} --write
{{end}}

workflows:
  main:
    jobs:
      - build-and-test
{{if .QTestEnabled}}      - generate-tests:
          requires:
            - build-and-test
          filters:
            branches:
              ignore: main{{end}}
`

	data := g.prepareCircleCIData()

	t, err := template.New("circleci").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// prepareTemplateData prepares data for GitHub Actions template
func (g *Generator) prepareTemplateData() map[string]interface{} {
	data := map[string]interface{}{
		"BranchList":      strings.Join(g.config.Branches, ", "),
		"BuildCommand":    g.config.BuildCommand,
		"TestCommand":     g.config.TestCommand,
		"LintCommand":     g.config.LintCommand,
		"CoverageEnabled": g.config.CoverageEnabled,
		"QTestEnabled":    g.config.QTestEnabled,
		"QTestTier":       g.config.QTestTier,
		"QTestMaxTests":   g.config.QTestMaxTests,
		"Services":        g.config.Services,
	}

	// Setup step based on language
	switch g.config.Language {
	case LanguageGo:
		data["SetupStep"] = fmt.Sprintf(`
      - uses: actions/setup-go@v5
        with:
          go-version: '%s'`, g.config.GoVersion)
		data["CoverageCheck"] = `go tool cover -func=coverage.out | grep total | awk '{print $3}'`
	case LanguagePython:
		data["SetupStep"] = fmt.Sprintf(`
      - uses: actions/setup-python@v5
        with:
          python-version: '%s'`, g.config.PythonVersion)
		data["CoverageCheck"] = `coverage report --fail-under=` + fmt.Sprintf("%.0f", g.config.CoverageThreshold)
	case LanguageJavaScript, LanguageTypeScript:
		data["SetupStep"] = fmt.Sprintf(`
      - uses: actions/setup-node@v4
        with:
          node-version: '%s'
          cache: 'npm'`, g.config.NodeVersion)
		data["CoverageCheck"] = `# Coverage threshold enforced in jest.config.js`
	case LanguageJava:
		data["SetupStep"] = fmt.Sprintf(`
      - uses: actions/setup-java@v4
        with:
          java-version: '%s'
          distribution: 'temurin'
          cache: 'maven'`, g.config.JavaVersion)
		data["CoverageCheck"] = `# Coverage checked via JaCoCo`
	}

	return data
}

// prepareGitLabData prepares data for GitLab CI template
func (g *Generator) prepareGitLabData() map[string]interface{} {
	data := map[string]interface{}{
		"BuildCommand":    g.config.BuildCommand,
		"TestCommand":     g.config.TestCommand,
		"LintCommand":     g.config.LintCommand,
		"CoverageEnabled": g.config.CoverageEnabled,
		"QTestEnabled":    g.config.QTestEnabled,
		"QTestTier":       g.config.QTestTier,
		"QTestMaxTests":   g.config.QTestMaxTests,
		"Services":        g.config.Services,
	}

	switch g.config.Language {
	case LanguageGo:
		data["ImageLine"] = fmt.Sprintf("image: golang:%s", g.config.GoVersion)
		data["Variables"] = "  GOPROXY: https://proxy.golang.org"
		data["CacheConfig"] = `    paths:
      - /go/pkg/mod/`
		data["CoverageFormat"] = "cobertura"
		data["CoveragePath"] = "coverage.xml"
	case LanguagePython:
		data["ImageLine"] = fmt.Sprintf("image: python:%s", g.config.PythonVersion)
		data["Variables"] = "  PIP_CACHE_DIR: .pip-cache"
		data["CacheConfig"] = `    paths:
      - .pip-cache/`
		data["CoverageFormat"] = "cobertura"
		data["CoveragePath"] = "coverage.xml"
	case LanguageJavaScript, LanguageTypeScript:
		data["ImageLine"] = fmt.Sprintf("image: node:%s", g.config.NodeVersion)
		data["Variables"] = "  npm_config_cache: .npm"
		data["CacheConfig"] = `    paths:
      - .npm/
      - node_modules/`
		data["CoverageFormat"] = "cobertura"
		data["CoveragePath"] = "coverage/cobertura-coverage.xml"
	case LanguageJava:
		data["ImageLine"] = fmt.Sprintf("image: maven:3.9-%s-eclipse-temurin", g.config.JavaVersion)
		data["Variables"] = "  MAVEN_OPTS: -Dmaven.repo.local=.m2/repository"
		data["CacheConfig"] = `    paths:
      - .m2/repository/`
		data["CoverageFormat"] = "jacoco"
		data["CoveragePath"] = "target/site/jacoco/jacoco.xml"
	}

	return data
}

// prepareCircleCIData prepares data for CircleCI template
func (g *Generator) prepareCircleCIData() map[string]interface{} {
	data := map[string]interface{}{
		"BuildCommand":    g.config.BuildCommand,
		"TestCommand":     g.config.TestCommand,
		"LintCommand":     g.config.LintCommand,
		"CoverageEnabled": g.config.CoverageEnabled,
		"QTestEnabled":    g.config.QTestEnabled,
		"QTestTier":       g.config.QTestTier,
		"QTestMaxTests":   g.config.QTestMaxTests,
	}

	switch g.config.Language {
	case LanguageGo:
		data["Orbs"] = "  go: circleci/go@1.7"
		data["Executor"] = fmt.Sprintf(`docker:
      - image: cimg/go:%s`, g.config.GoVersion)
		data["SetupSteps"] = ""
		data["CoveragePath"] = "coverage.out"
	case LanguagePython:
		data["Orbs"] = "  python: circleci/python@2.1"
		data["Executor"] = fmt.Sprintf(`docker:
      - image: cimg/python:%s`, g.config.PythonVersion)
		data["SetupSteps"] = ""
		data["CoveragePath"] = "coverage.xml"
	case LanguageJavaScript, LanguageTypeScript:
		data["Orbs"] = "  node: circleci/node@5.1"
		data["Executor"] = fmt.Sprintf(`docker:
      - image: cimg/node:%s`, g.config.NodeVersion)
		data["SetupSteps"] = `      - node/install-packages`
		data["CoveragePath"] = "coverage"
	case LanguageJava:
		data["Orbs"] = "  maven: circleci/maven@1.4"
		data["Executor"] = fmt.Sprintf(`docker:
      - image: cimg/openjdk:%s.0`, g.config.JavaVersion)
		data["SetupSteps"] = ""
		data["CoveragePath"] = "target/site/jacoco"
	}

	return data
}

// DetectConfig auto-detects project configuration from a directory
func DetectConfig(projectDir string) (*Config, error) {
	// Detect language
	language := detectLanguage(projectDir)
	cfg := DefaultConfig(language)

	// Set project name from directory
	cfg.ProjectName = filepath.Base(projectDir)

	// Detect framework and adjust settings
	detectFramework(projectDir, cfg)

	// Detect services from docker-compose or other config
	detectServices(projectDir, cfg)

	return cfg, nil
}

// detectLanguage detects the primary language of a project
func detectLanguage(dir string) Language {
	// Check for language-specific files
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return LanguageGo
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		// Check for TypeScript
		if _, err := os.Stat(filepath.Join(dir, "tsconfig.json")); err == nil {
			return LanguageTypeScript
		}
		return LanguageJavaScript
	}
	if _, err := os.Stat(filepath.Join(dir, "requirements.txt")); err == nil {
		return LanguagePython
	}
	if _, err := os.Stat(filepath.Join(dir, "pyproject.toml")); err == nil {
		return LanguagePython
	}
	if _, err := os.Stat(filepath.Join(dir, "pom.xml")); err == nil {
		return LanguageJava
	}
	if _, err := os.Stat(filepath.Join(dir, "build.gradle")); err == nil {
		return LanguageJava
	}

	return LanguageGo // Default
}

// detectFramework detects web frameworks and adjusts config
func detectFramework(dir string, cfg *Config) {
	switch cfg.Language {
	case LanguageJavaScript, LanguageTypeScript:
		// Check package.json for framework
		pkgPath := filepath.Join(dir, "package.json")
		if data, err := os.ReadFile(pkgPath); err == nil {
			content := string(data)
			if strings.Contains(content, "\"express\"") {
				cfg.Framework = "express"
			} else if strings.Contains(content, "\"@nestjs/core\"") {
				cfg.Framework = "nestjs"
			} else if strings.Contains(content, "\"next\"") {
				cfg.Framework = "next"
			} else if strings.Contains(content, "\"react\"") {
				cfg.Framework = "react"
			}
			// Check for test framework
			if strings.Contains(content, "\"jest\"") {
				cfg.TestCommand = "npm test -- --coverage --watchAll=false"
			} else if strings.Contains(content, "\"vitest\"") {
				cfg.TestCommand = "npm test -- --coverage"
			}
		}
	case LanguagePython:
		// Check for Django, FastAPI, Flask
		reqPath := filepath.Join(dir, "requirements.txt")
		if data, err := os.ReadFile(reqPath); err == nil {
			content := string(data)
			if strings.Contains(content, "django") {
				cfg.Framework = "django"
				cfg.TestCommand = "python manage.py test --with-coverage"
			} else if strings.Contains(content, "fastapi") {
				cfg.Framework = "fastapi"
			} else if strings.Contains(content, "flask") {
				cfg.Framework = "flask"
			}
		}
	case LanguageGo:
		// Check for web frameworks
		goModPath := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(goModPath); err == nil {
			content := string(data)
			if strings.Contains(content, "github.com/gin-gonic/gin") {
				cfg.Framework = "gin"
			} else if strings.Contains(content, "github.com/gofiber/fiber") {
				cfg.Framework = "fiber"
			} else if strings.Contains(content, "github.com/go-chi/chi") {
				cfg.Framework = "chi"
			}
		}
	case LanguageJava:
		// Check for Spring Boot
		pomPath := filepath.Join(dir, "pom.xml")
		if data, err := os.ReadFile(pomPath); err == nil {
			content := string(data)
			if strings.Contains(content, "spring-boot") {
				cfg.Framework = "spring-boot"
				cfg.TestCommand = "mvn test -Djacoco.skip=false"
			}
		}
	}
}

// detectServices detects required services from docker-compose
func detectServices(dir string, cfg *Config) {
	composePath := filepath.Join(dir, "docker-compose.yml")
	if data, err := os.ReadFile(composePath); err == nil {
		content := string(data)
		if strings.Contains(content, "postgres") {
			cfg.Services = append(cfg.Services, "postgres")
		}
		if strings.Contains(content, "redis") {
			cfg.Services = append(cfg.Services, "redis")
		}
		if strings.Contains(content, "mysql") {
			cfg.Services = append(cfg.Services, "mysql")
		}
		if strings.Contains(content, "mongo") {
			cfg.Services = append(cfg.Services, "mongo")
		}
	}
}

// SupportedPlatforms returns list of supported CI platforms
func SupportedPlatforms() []Platform {
	return []Platform{
		PlatformGitHubActions,
		PlatformGitLabCI,
		PlatformCircleCI,
	}
}

// SupportedLanguages returns list of supported languages
func SupportedLanguages() []Language {
	return []Language{
		LanguageGo,
		LanguagePython,
		LanguageJavaScript,
		LanguageTypeScript,
		LanguageJava,
	}
}
