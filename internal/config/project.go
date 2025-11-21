package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ProjectConfig represents a .qtest.yaml file in a repository
type ProjectConfig struct {
	Version string `yaml:"version"`

	// Language detection override
	Language string `yaml:"language,omitempty"`

	// Test generation settings
	Generation GenerationConfig `yaml:"generation"`

	// File patterns
	Include []string `yaml:"include,omitempty"`
	Exclude []string `yaml:"exclude,omitempty"`

	// Framework preferences
	Framework FrameworkConfig `yaml:"framework,omitempty"`

	// Coverage settings
	Coverage CoverageConfig `yaml:"coverage,omitempty"`
}

// GenerationConfig holds test generation preferences
type GenerationConfig struct {
	// Default tier for generation (1, 2, 3)
	Tier int `yaml:"tier,omitempty"`

	// Test style preferences
	Style string `yaml:"style,omitempty"` // table-driven, standard, bdd

	// Max tests per function
	MaxTestsPerFunction int `yaml:"max_tests_per_function,omitempty"`

	// Whether to generate edge case tests
	EdgeCases bool `yaml:"edge_cases,omitempty"`

	// Whether to generate error path tests
	ErrorPaths bool `yaml:"error_paths,omitempty"`
}

// FrameworkConfig holds framework preferences
type FrameworkConfig struct {
	// Test framework to use (go, jest, pytest, etc.)
	Name string `yaml:"name,omitempty"`

	// Custom test file suffix
	TestFileSuffix string `yaml:"test_file_suffix,omitempty"`

	// Custom test directory
	TestDir string `yaml:"test_dir,omitempty"`
}

// CoverageConfig holds coverage settings
type CoverageConfig struct {
	// Minimum coverage threshold (0-100)
	Threshold float64 `yaml:"threshold,omitempty"`

	// Files to exclude from coverage
	Exclude []string `yaml:"exclude,omitempty"`
}

// DefaultProjectConfig returns sensible defaults
func DefaultProjectConfig() *ProjectConfig {
	return &ProjectConfig{
		Version: "1.0",
		Generation: GenerationConfig{
			Tier:                2,
			Style:               "standard",
			MaxTestsPerFunction: 5,
			EdgeCases:           true,
			ErrorPaths:          true,
		},
		Include: []string{"**/*.go", "**/*.py", "**/*.ts", "**/*.js"},
		Exclude: []string{
			"**/vendor/**",
			"**/node_modules/**",
			"**/*_test.go",
			"**/test_*.py",
			"**/*.test.ts",
			"**/*.test.js",
		},
		Coverage: CoverageConfig{
			Threshold: 80.0,
		},
	}
}

// LoadProjectConfig loads a .qtest.yaml from the given directory
func LoadProjectConfig(repoPath string) (*ProjectConfig, error) {
	configPath := filepath.Join(repoPath, ".qtest.yaml")

	// Check if config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Also try .qtest.yml
		configPath = filepath.Join(repoPath, ".qtest.yml")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return DefaultProjectConfig(), nil
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	cfg := DefaultProjectConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// SaveProjectConfig saves the config to .qtest.yaml
func SaveProjectConfig(repoPath string, cfg *ProjectConfig) error {
	configPath := filepath.Join(repoPath, ".qtest.yaml")

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// Merge applies overrides from another config (e.g., CLI flags)
func (c *ProjectConfig) Merge(other *ProjectConfig) {
	if other == nil {
		return
	}

	if other.Language != "" {
		c.Language = other.Language
	}

	if other.Generation.Tier != 0 {
		c.Generation.Tier = other.Generation.Tier
	}

	if other.Generation.Style != "" {
		c.Generation.Style = other.Generation.Style
	}

	if other.Generation.MaxTestsPerFunction != 0 {
		c.Generation.MaxTestsPerFunction = other.Generation.MaxTestsPerFunction
	}

	if len(other.Include) > 0 {
		c.Include = other.Include
	}

	if len(other.Exclude) > 0 {
		c.Exclude = other.Exclude
	}

	if other.Framework.Name != "" {
		c.Framework.Name = other.Framework.Name
	}

	if other.Framework.TestDir != "" {
		c.Framework.TestDir = other.Framework.TestDir
	}

	if other.Coverage.Threshold != 0 {
		c.Coverage.Threshold = other.Coverage.Threshold
	}
}
