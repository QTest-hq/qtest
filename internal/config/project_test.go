package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultProjectConfig(t *testing.T) {
	cfg := DefaultProjectConfig()

	if cfg == nil {
		t.Fatal("DefaultProjectConfig() returned nil")
	}

	if cfg.Version != "1.0" {
		t.Errorf("Version = %s, want 1.0", cfg.Version)
	}

	// Check generation defaults
	if cfg.Generation.Tier != 2 {
		t.Errorf("Generation.Tier = %d, want 2", cfg.Generation.Tier)
	}
	if cfg.Generation.Style != "standard" {
		t.Errorf("Generation.Style = %s, want standard", cfg.Generation.Style)
	}
	if cfg.Generation.MaxTestsPerFunction != 5 {
		t.Errorf("Generation.MaxTestsPerFunction = %d, want 5", cfg.Generation.MaxTestsPerFunction)
	}
	if !cfg.Generation.EdgeCases {
		t.Error("Generation.EdgeCases should be true")
	}
	if !cfg.Generation.ErrorPaths {
		t.Error("Generation.ErrorPaths should be true")
	}

	// Check include patterns
	if len(cfg.Include) != 4 {
		t.Errorf("len(Include) = %d, want 4", len(cfg.Include))
	}

	// Check exclude patterns
	if len(cfg.Exclude) < 4 {
		t.Errorf("len(Exclude) = %d, want at least 4", len(cfg.Exclude))
	}

	// Check coverage threshold
	if cfg.Coverage.Threshold != 80.0 {
		t.Errorf("Coverage.Threshold = %f, want 80.0", cfg.Coverage.Threshold)
	}
}

func TestProjectConfig_Fields(t *testing.T) {
	cfg := &ProjectConfig{
		Version:  "2.0",
		Language: "go",
		Include:  []string{"src/**/*.go"},
		Exclude:  []string{"vendor/**"},
	}

	if cfg.Version != "2.0" {
		t.Errorf("Version = %s, want 2.0", cfg.Version)
	}
	if cfg.Language != "go" {
		t.Errorf("Language = %s, want go", cfg.Language)
	}
	if len(cfg.Include) != 1 {
		t.Errorf("len(Include) = %d, want 1", len(cfg.Include))
	}
	if len(cfg.Exclude) != 1 {
		t.Errorf("len(Exclude) = %d, want 1", len(cfg.Exclude))
	}
}

func TestGenerationConfig_Fields(t *testing.T) {
	gen := GenerationConfig{
		Tier:                3,
		Style:               "table-driven",
		MaxTestsPerFunction: 10,
		EdgeCases:           true,
		ErrorPaths:          false,
	}

	if gen.Tier != 3 {
		t.Errorf("Tier = %d, want 3", gen.Tier)
	}
	if gen.Style != "table-driven" {
		t.Errorf("Style = %s, want table-driven", gen.Style)
	}
	if gen.MaxTestsPerFunction != 10 {
		t.Errorf("MaxTestsPerFunction = %d, want 10", gen.MaxTestsPerFunction)
	}
	if !gen.EdgeCases {
		t.Error("EdgeCases should be true")
	}
	if gen.ErrorPaths {
		t.Error("ErrorPaths should be false")
	}
}

func TestFrameworkConfig_Fields(t *testing.T) {
	fw := FrameworkConfig{
		Name:           "jest",
		TestFileSuffix: ".spec",
		TestDir:        "__tests__",
	}

	if fw.Name != "jest" {
		t.Errorf("Name = %s, want jest", fw.Name)
	}
	if fw.TestFileSuffix != ".spec" {
		t.Errorf("TestFileSuffix = %s, want .spec", fw.TestFileSuffix)
	}
	if fw.TestDir != "__tests__" {
		t.Errorf("TestDir = %s, want __tests__", fw.TestDir)
	}
}

func TestCoverageConfig_Fields(t *testing.T) {
	cov := CoverageConfig{
		Threshold: 90.5,
		Exclude:   []string{"generated/**", "mocks/**"},
	}

	if cov.Threshold != 90.5 {
		t.Errorf("Threshold = %f, want 90.5", cov.Threshold)
	}
	if len(cov.Exclude) != 2 {
		t.Errorf("len(Exclude) = %d, want 2", len(cov.Exclude))
	}
}

func TestProjectConfig_Merge(t *testing.T) {
	base := DefaultProjectConfig()

	override := &ProjectConfig{
		Language: "python",
		Generation: GenerationConfig{
			Tier:                1,
			Style:               "bdd",
			MaxTestsPerFunction: 3,
		},
		Include: []string{"src/**/*.py"},
		Framework: FrameworkConfig{
			Name:    "pytest",
			TestDir: "tests",
		},
		Coverage: CoverageConfig{
			Threshold: 95.0,
		},
	}

	base.Merge(override)

	if base.Language != "python" {
		t.Errorf("Language = %s, want python", base.Language)
	}
	if base.Generation.Tier != 1 {
		t.Errorf("Generation.Tier = %d, want 1", base.Generation.Tier)
	}
	if base.Generation.Style != "bdd" {
		t.Errorf("Generation.Style = %s, want bdd", base.Generation.Style)
	}
	if base.Generation.MaxTestsPerFunction != 3 {
		t.Errorf("Generation.MaxTestsPerFunction = %d, want 3", base.Generation.MaxTestsPerFunction)
	}
	if len(base.Include) != 1 || base.Include[0] != "src/**/*.py" {
		t.Errorf("Include = %v, want [src/**/*.py]", base.Include)
	}
	if base.Framework.Name != "pytest" {
		t.Errorf("Framework.Name = %s, want pytest", base.Framework.Name)
	}
	if base.Framework.TestDir != "tests" {
		t.Errorf("Framework.TestDir = %s, want tests", base.Framework.TestDir)
	}
	if base.Coverage.Threshold != 95.0 {
		t.Errorf("Coverage.Threshold = %f, want 95.0", base.Coverage.Threshold)
	}
}

func TestProjectConfig_Merge_NilOverride(t *testing.T) {
	base := DefaultProjectConfig()
	originalVersion := base.Version

	base.Merge(nil)

	// Should not change anything
	if base.Version != originalVersion {
		t.Error("Merge(nil) should not change config")
	}
}

func TestProjectConfig_Merge_PartialOverride(t *testing.T) {
	base := DefaultProjectConfig()
	originalStyle := base.Generation.Style
	originalExclude := len(base.Exclude)

	// Only override tier
	override := &ProjectConfig{
		Generation: GenerationConfig{
			Tier: 3,
		},
	}

	base.Merge(override)

	// Tier should change
	if base.Generation.Tier != 3 {
		t.Errorf("Generation.Tier = %d, want 3", base.Generation.Tier)
	}

	// Style should remain unchanged
	if base.Generation.Style != originalStyle {
		t.Errorf("Generation.Style = %s, want %s", base.Generation.Style, originalStyle)
	}

	// Exclude should remain unchanged
	if len(base.Exclude) != originalExclude {
		t.Errorf("len(Exclude) = %d, want %d", len(base.Exclude), originalExclude)
	}
}

func TestLoadProjectConfig_NoFile(t *testing.T) {
	// Use temp directory with no config file
	tmpDir := t.TempDir()

	cfg, err := LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error = %v", err)
	}

	// Should return defaults
	if cfg.Version != "1.0" {
		t.Errorf("Version = %s, want 1.0", cfg.Version)
	}
}

func TestLoadProjectConfig_YamlFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".qtest.yaml")

	yamlContent := `
version: "2.0"
language: typescript
generation:
  tier: 1
  style: bdd
include:
  - "src/**/*.ts"
coverage:
  threshold: 85.0
`

	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error = %v", err)
	}

	if cfg.Version != "2.0" {
		t.Errorf("Version = %s, want 2.0", cfg.Version)
	}
	if cfg.Language != "typescript" {
		t.Errorf("Language = %s, want typescript", cfg.Language)
	}
	if cfg.Generation.Tier != 1 {
		t.Errorf("Generation.Tier = %d, want 1", cfg.Generation.Tier)
	}
	if cfg.Generation.Style != "bdd" {
		t.Errorf("Generation.Style = %s, want bdd", cfg.Generation.Style)
	}
	if cfg.Coverage.Threshold != 85.0 {
		t.Errorf("Coverage.Threshold = %f, want 85.0", cfg.Coverage.Threshold)
	}
}

func TestLoadProjectConfig_YmlFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".qtest.yml")

	yamlContent := `
version: "1.5"
language: python
`

	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error = %v", err)
	}

	if cfg.Version != "1.5" {
		t.Errorf("Version = %s, want 1.5", cfg.Version)
	}
	if cfg.Language != "python" {
		t.Errorf("Language = %s, want python", cfg.Language)
	}
}

func TestSaveProjectConfig(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &ProjectConfig{
		Version:  "1.0",
		Language: "go",
		Generation: GenerationConfig{
			Tier:  2,
			Style: "table-driven",
		},
	}

	if err := SaveProjectConfig(tmpDir, cfg); err != nil {
		t.Fatalf("SaveProjectConfig() error = %v", err)
	}

	// Verify file was created
	configPath := filepath.Join(tmpDir, ".qtest.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// Load it back
	loaded, err := LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error = %v", err)
	}

	if loaded.Version != cfg.Version {
		t.Errorf("Version = %s, want %s", loaded.Version, cfg.Version)
	}
	if loaded.Language != cfg.Language {
		t.Errorf("Language = %s, want %s", loaded.Language, cfg.Language)
	}
	if loaded.Generation.Tier != cfg.Generation.Tier {
		t.Errorf("Generation.Tier = %d, want %d", loaded.Generation.Tier, cfg.Generation.Tier)
	}
}

func TestLoadProjectConfig_InvalidYaml(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".qtest.yaml")

	invalidYaml := `
version: [invalid yaml
generation:
  - this is wrong
`

	if err := os.WriteFile(configPath, []byte(invalidYaml), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := LoadProjectConfig(tmpDir)
	if err == nil {
		t.Error("LoadProjectConfig() should return error for invalid YAML")
	}
}

func TestGenerationConfig_Defaults(t *testing.T) {
	gen := GenerationConfig{}

	if gen.Tier != 0 {
		t.Errorf("default Tier = %d, want 0", gen.Tier)
	}
	if gen.Style != "" {
		t.Errorf("default Style = %s, want empty", gen.Style)
	}
	if gen.MaxTestsPerFunction != 0 {
		t.Errorf("default MaxTestsPerFunction = %d, want 0", gen.MaxTestsPerFunction)
	}
	if gen.EdgeCases {
		t.Error("default EdgeCases should be false")
	}
	if gen.ErrorPaths {
		t.Error("default ErrorPaths should be false")
	}
}

func TestFrameworkConfig_Defaults(t *testing.T) {
	fw := FrameworkConfig{}

	if fw.Name != "" {
		t.Errorf("default Name = %s, want empty", fw.Name)
	}
	if fw.TestFileSuffix != "" {
		t.Errorf("default TestFileSuffix = %s, want empty", fw.TestFileSuffix)
	}
	if fw.TestDir != "" {
		t.Errorf("default TestDir = %s, want empty", fw.TestDir)
	}
}

func TestCoverageConfig_Defaults(t *testing.T) {
	cov := CoverageConfig{}

	if cov.Threshold != 0 {
		t.Errorf("default Threshold = %f, want 0", cov.Threshold)
	}
	if cov.Exclude != nil {
		t.Error("default Exclude should be nil")
	}
}
