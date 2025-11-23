package maintenance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/QTest-hq/qtest/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockGenerator implements TestGenerator for testing
type MockGenerator struct {
	specs []model.TestSpec
	err   error
}

func (m *MockGenerator) GenerateForFunction(fn model.Function) ([]model.TestSpec, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.specs, nil
}

func TestNewRegenerator(t *testing.T) {
	config := DefaultRegeneratorConfig()
	r := NewRegenerator(config, nil)
	assert.NotNil(t, r)
}

func TestDefaultRegeneratorConfig(t *testing.T) {
	config := DefaultRegeneratorConfig()
	assert.True(t, config.BackupBeforeRegenerate)
	assert.Equal(t, ".qtest/backups", config.BackupDir)
	assert.False(t, config.DryRun)
}

func TestRegenerateTests_SkipsNonMatchingJobs(t *testing.T) {
	r := NewRegenerator(DefaultRegeneratorConfig(), nil)

	jobs := []MaintenanceJob{
		{Type: JobTypeRemove, TargetID: "fn1"},
		{Type: JobTypeUpdate, TargetID: "fn2"},
	}

	results := r.RegenerateTests(jobs, nil)
	assert.Empty(t, results)
}

func TestGetTestFilePath_Go(t *testing.T) {
	config := DefaultRegeneratorConfig()
	r := NewRegenerator(config, nil)

	fn := model.Function{
		File: "/path/to/calculator.go",
		Name: "Add",
	}

	path := r.getTestFilePath(fn)
	assert.Equal(t, "/path/to/calculator_test.go", path)
}

func TestGetTestFilePath_Python(t *testing.T) {
	config := DefaultRegeneratorConfig()
	r := NewRegenerator(config, nil)

	fn := model.Function{
		File: "/path/to/calculator.py",
		Name: "add",
	}

	path := r.getTestFilePath(fn)
	assert.Equal(t, "/path/to/test_calculator.py", path)
}

func TestGetTestFilePath_Java(t *testing.T) {
	config := DefaultRegeneratorConfig()
	r := NewRegenerator(config, nil)

	fn := model.Function{
		File: "/path/to/Calculator.java",
		Name: "add",
	}

	path := r.getTestFilePath(fn)
	assert.Equal(t, "/path/to/CalculatorTest.java", path)
}

func TestGetTestFilePath_JavaScript(t *testing.T) {
	config := DefaultRegeneratorConfig()
	r := NewRegenerator(config, nil)

	fn := model.Function{
		File: "/path/to/calculator.js",
		Name: "add",
	}

	path := r.getTestFilePath(fn)
	assert.Equal(t, "/path/to/calculator.test.js", path)
}

func TestGetTestFilePath_WithOutputDir(t *testing.T) {
	config := DefaultRegeneratorConfig()
	config.OutputDir = "/custom/tests"
	r := NewRegenerator(config, nil)

	fn := model.Function{
		File: "/path/to/calculator.go",
		Name: "Add",
	}

	path := r.getTestFilePath(fn)
	assert.Equal(t, "/custom/tests/calculator_test.go", path)
}

func TestGenerateTestCode_Go(t *testing.T) {
	r := NewRegenerator(DefaultRegeneratorConfig(), nil)

	fn := model.Function{
		File: "calc.go",
		Name: "Add",
	}
	specs := []model.TestSpec{
		{Description: "adds positive numbers"},
		{Description: "adds negative numbers"},
	}

	code := r.generateTestCode(fn, specs)

	assert.Contains(t, code, "func TestAdd_1")
	assert.Contains(t, code, "func TestAdd_2")
	assert.Contains(t, code, "adds positive numbers")
	assert.Contains(t, code, "adds negative numbers")
}

func TestGenerateTestCode_Python(t *testing.T) {
	r := NewRegenerator(DefaultRegeneratorConfig(), nil)

	fn := model.Function{
		File: "calc.py",
		Name: "add",
	}
	specs := []model.TestSpec{
		{Description: "adds positive numbers"},
	}

	code := r.generateTestCode(fn, specs)

	assert.Contains(t, code, "def test_add_1")
	assert.Contains(t, code, "adds positive numbers")
}

func TestGenerateTestCode_Java(t *testing.T) {
	r := NewRegenerator(DefaultRegeneratorConfig(), nil)

	fn := model.Function{
		File: "Calculator.java",
		Name: "add",
	}
	specs := []model.TestSpec{
		{Description: "adds positive numbers"},
	}

	code := r.generateTestCode(fn, specs)

	assert.Contains(t, code, "@Test")
	assert.Contains(t, code, "void testadd1")
	assert.Contains(t, code, "adds positive numbers")
}

func TestCreateNewTest_NoGenerator(t *testing.T) {
	config := DefaultRegeneratorConfig()
	config.DryRun = true
	r := NewRegenerator(config, nil)

	job := MaintenanceJob{
		Type:      JobTypeCreate,
		TargetID:  "fn1",
		NewEntity: model.Function{Name: "Add", File: "calc.go"},
	}

	result := r.createNewTest(job)

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "no test generator")
}

func TestCreateNewTest_InvalidEntity(t *testing.T) {
	r := NewRegenerator(DefaultRegeneratorConfig(), &MockGenerator{})

	job := MaintenanceJob{
		Type:      JobTypeCreate,
		TargetID:  "fn1",
		NewEntity: "not a function",
	}

	result := r.createNewTest(job)

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "not contain a valid function")
}

func TestCreateNewTest_EmptySpecs(t *testing.T) {
	generator := &MockGenerator{specs: []model.TestSpec{}}
	r := NewRegenerator(DefaultRegeneratorConfig(), generator)

	job := MaintenanceJob{
		Type:      JobTypeCreate,
		TargetID:  "fn1",
		NewEntity: model.Function{Name: "Add", File: "calc.go"},
	}

	result := r.createNewTest(job)

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "no test specs")
}

func TestCreateNewTest_DryRun(t *testing.T) {
	generator := &MockGenerator{
		specs: []model.TestSpec{
			{Description: "test case 1"},
		},
	}
	config := DefaultRegeneratorConfig()
	config.DryRun = true
	r := NewRegenerator(config, generator)

	job := MaintenanceJob{
		Type:      JobTypeCreate,
		TargetID:  "fn1",
		NewEntity: model.Function{Name: "Add", File: "/tmp/calc.go"},
	}

	result := r.createNewTest(job)

	assert.True(t, result.Success)
	assert.Equal(t, 1, result.TestsCreated)
	// File should not be created in dry run
	_, err := os.Stat("/tmp/calc_test.go")
	assert.True(t, os.IsNotExist(err))
}

func TestCreateNewTest_ActualWrite(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "calc.go")

	generator := &MockGenerator{
		specs: []model.TestSpec{
			{Description: "test adding numbers"},
		},
	}
	config := DefaultRegeneratorConfig()
	config.BackupBeforeRegenerate = false
	r := NewRegenerator(config, generator)

	job := MaintenanceJob{
		Type:      JobTypeCreate,
		TargetID:  "fn1",
		NewEntity: model.Function{Name: "Add", File: srcFile},
	}

	result := r.createNewTest(job)

	assert.True(t, result.Success)
	assert.Equal(t, 1, result.TestsCreated)

	// Check file was created
	testFile := filepath.Join(tmpDir, "calc_test.go")
	content, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "TestAdd")
	assert.Contains(t, string(content), "test adding numbers")
}

func TestRegenerateTest_NoGenerator(t *testing.T) {
	config := DefaultRegeneratorConfig()
	config.DryRun = true
	r := NewRegenerator(config, nil)

	job := MaintenanceJob{
		Type:      JobTypeRegenerate,
		TargetID:  "fn1",
		NewEntity: model.Function{Name: "Add", File: "calc.go"},
	}

	result := r.regenerateTest(job, nil)

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "no test generator")
}

func TestRegenerateTest_WithExistingTests(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "calc_test.go")
	err := os.WriteFile(testFile, []byte("existing test content"), 0644)
	require.NoError(t, err)

	generator := &MockGenerator{
		specs: []model.TestSpec{
			{Description: "regenerated test"},
		},
	}
	config := DefaultRegeneratorConfig()
	config.BackupDir = filepath.Join(tmpDir, "backups")
	r := NewRegenerator(config, generator)

	job := MaintenanceJob{
		Type:      JobTypeRegenerate,
		TargetID:  "fn1",
		NewEntity: model.Function{Name: "Add", File: filepath.Join(tmpDir, "calc.go")},
	}

	testMapping := map[string][]TestInfo{
		"fn1": {{File: testFile, FunctionName: "TestAdd", StartLine: 1, EndLine: 5}},
	}

	result := r.regenerateTest(job, testMapping)

	assert.True(t, result.Success)
	assert.Equal(t, testFile, result.TestFile)
	assert.Equal(t, 1, result.TestsCreated)

	// Check backup was created
	backupFile := filepath.Join(config.BackupDir, "calc_test.go.bak")
	_, err = os.Stat(backupFile)
	assert.NoError(t, err)
}

func TestProcessUpdateJob_NoExistingTests(t *testing.T) {
	r := NewRegenerator(DefaultRegeneratorConfig(), nil)

	job := MaintenanceJob{
		Type:     JobTypeUpdate,
		TargetID: "fn1",
		Reason:   "body changed",
	}

	result := r.ProcessUpdateJob(job, map[string][]TestInfo{})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "no existing tests")
}

func TestProcessUpdateJob_WithExistingTests(t *testing.T) {
	r := NewRegenerator(DefaultRegeneratorConfig(), nil)

	job := MaintenanceJob{
		Type:     JobTypeUpdate,
		TargetID: "fn1",
		Reason:   "body changed",
	}

	testMapping := map[string][]TestInfo{
		"fn1": {{File: "/path/to/test.go", FunctionName: "TestAdd"}},
	}

	result := r.ProcessUpdateJob(job, testMapping)

	assert.True(t, result.Success)
	assert.Equal(t, "/path/to/test.go", result.TestFile)
}

func TestRegenerateTests_Integration(t *testing.T) {
	tmpDir := t.TempDir()

	generator := &MockGenerator{
		specs: []model.TestSpec{
			{Description: "test case 1"},
			{Description: "test case 2"},
		},
	}
	config := DefaultRegeneratorConfig()
	config.BackupBeforeRegenerate = false
	r := NewRegenerator(config, generator)

	jobs := []MaintenanceJob{
		{
			Type:      JobTypeCreate,
			TargetID:  "fn1",
			Reason:    "new function",
			NewEntity: model.Function{Name: "Add", File: filepath.Join(tmpDir, "calc.go")},
		},
		{
			Type:      JobTypeRegenerate,
			TargetID:  "fn2",
			Reason:    "signature changed",
			NewEntity: model.Function{Name: "Sub", File: filepath.Join(tmpDir, "calc.go")},
		},
		{
			Type:     JobTypeRemove, // Should be skipped
			TargetID: "fn3",
		},
	}

	results := r.RegenerateTests(jobs, nil)

	assert.Len(t, results, 2)
	assert.True(t, results[0].Success)
	assert.True(t, results[1].Success)
}
