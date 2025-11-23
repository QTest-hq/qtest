package strengthening

import (
	"testing"

	"github.com/QTest-hq/qtest/internal/mutation"
	"github.com/stretchr/testify/assert"
)

func TestDefaultStrengthenConfig(t *testing.T) {
	config := DefaultStrengthenConfig()
	assert.Equal(t, 2, config.MaxAttempts)
	assert.Equal(t, 0.10, config.MinScoreImprovement)
	assert.Equal(t, 0.80, config.TargetScore)
	assert.False(t, config.DryRun)
}

func TestNewStrengthener(t *testing.T) {
	config := DefaultStrengthenConfig()
	s := NewStrengthener(config, nil)
	assert.NotNil(t, s)
	assert.NotNil(t, s.analyzer)
}

func TestNewMutantAnalyzer(t *testing.T) {
	analyzer := NewMutantAnalyzer()
	assert.NotNil(t, analyzer)
}

func TestAnalyze_NoSurvivors(t *testing.T) {
	analyzer := NewMutantAnalyzer()

	result := &mutation.Result{
		SourceFile: "test.go",
		TestFile:   "test_test.go",
		Total:      5,
		Killed:     5,
		Survived:   0,
		Score:      1.0,
		Mutants: []mutation.Mutant{
			{ID: "m1", Status: mutation.StatusKilled, Type: "arithmetic", Line: 10},
			{ID: "m2", Status: mutation.StatusKilled, Type: "comparison", Line: 15},
		},
	}

	analysis := analyzer.Analyze(result)

	assert.Equal(t, "test.go", analysis.SourceFile)
	assert.Equal(t, 5, analysis.TotalMutants)
	assert.Equal(t, 5, analysis.KilledMutants)
	assert.Equal(t, 1.0, analysis.MutationScore)
	assert.Empty(t, analysis.SurvivingMutants)
	assert.False(t, analysis.NeedsStrengthening)
}

func TestAnalyze_WithSurvivors(t *testing.T) {
	analyzer := NewMutantAnalyzer()

	result := &mutation.Result{
		SourceFile: "calc.go",
		TestFile:   "calc_test.go",
		Total:      10,
		Killed:     6,
		Survived:   4,
		Score:      0.6,
		Mutants: []mutation.Mutant{
			{ID: "m1", Status: mutation.StatusKilled, Type: "arithmetic", Line: 10},
			{ID: "m2", Status: mutation.StatusSurvived, Type: "arithmetic", Line: 12, Description: "Replaced + with -"},
			{ID: "m3", Status: mutation.StatusSurvived, Type: "comparison", Line: 20, Description: "Replaced == with !="},
			{ID: "m4", Status: mutation.StatusSurvived, Type: "boolean", Line: 25, Description: "Replaced true with false"},
			{ID: "m5", Status: mutation.StatusSurvived, Type: "return", Line: 30, Description: "Changed return value"},
		},
	}

	analysis := analyzer.Analyze(result)

	assert.Equal(t, 10, analysis.TotalMutants)
	assert.Equal(t, 6, analysis.KilledMutants)
	assert.Len(t, analysis.SurvivingMutants, 4)
	assert.True(t, analysis.NeedsStrengthening)
	assert.NotEmpty(t, analysis.Recommendations)
}

func TestAnalyze_HighScore_NoStrengtheningNeeded(t *testing.T) {
	analyzer := NewMutantAnalyzer()

	result := &mutation.Result{
		SourceFile: "test.go",
		TestFile:   "test_test.go",
		Total:      10,
		Killed:     9,
		Survived:   1,
		Score:      0.9, // Above 0.80 threshold
		Mutants: []mutation.Mutant{
			{ID: "m1", Status: mutation.StatusSurvived, Type: "arithmetic", Line: 10},
		},
	}

	analysis := analyzer.Analyze(result)

	assert.Len(t, analysis.SurvivingMutants, 1)
	assert.False(t, analysis.NeedsStrengthening) // Score is high enough
}

func TestIdentifyWeakAreas_SingleMutant(t *testing.T) {
	analyzer := NewMutantAnalyzer()

	mutants := []SurvivingMutant{
		{ID: "m1", Type: "arithmetic", Line: 10},
	}

	areas := analyzer.identifyWeakAreas(mutants)

	assert.Len(t, areas, 1)
	assert.Equal(t, 10, areas[0].StartLine)
	assert.Equal(t, 10, areas[0].EndLine)
	assert.Equal(t, 1, areas[0].Count)
	assert.Equal(t, "low", areas[0].Severity)
}

func TestIdentifyWeakAreas_ClusteredMutants(t *testing.T) {
	analyzer := NewMutantAnalyzer()

	mutants := []SurvivingMutant{
		{ID: "m1", Type: "arithmetic", Line: 10},
		{ID: "m2", Type: "comparison", Line: 12},
		{ID: "m3", Type: "boolean", Line: 14},
	}

	areas := analyzer.identifyWeakAreas(mutants)

	// All within 5 lines of each other, should be one area
	assert.Len(t, areas, 1)
	assert.Equal(t, 10, areas[0].StartLine)
	assert.Equal(t, 14, areas[0].EndLine)
	assert.Equal(t, 3, areas[0].Count)
	assert.Equal(t, "high", areas[0].Severity) // 3 mutants = high
	assert.Len(t, areas[0].MutantTypes, 3)
}

func TestIdentifyWeakAreas_SeparateAreas(t *testing.T) {
	analyzer := NewMutantAnalyzer()

	mutants := []SurvivingMutant{
		{ID: "m1", Type: "arithmetic", Line: 10},
		{ID: "m2", Type: "comparison", Line: 100}, // Far away
	}

	areas := analyzer.identifyWeakAreas(mutants)

	assert.Len(t, areas, 2)
}

func TestCalculateSeverity(t *testing.T) {
	analyzer := NewMutantAnalyzer()

	tests := []struct {
		count    int
		expected string
	}{
		{1, "low"},
		{2, "medium"},
		{3, "high"},
		{4, "high"},
		{5, "critical"},
		{10, "critical"},
	}

	for _, tt := range tests {
		area := &WeakArea{Count: tt.count}
		severity := analyzer.calculateSeverity(area)
		assert.Equal(t, tt.expected, severity)
	}
}

func TestGenerateRecommendations_ArithmeticMutants(t *testing.T) {
	analyzer := NewMutantAnalyzer()

	analysis := &AnalysisResult{
		MutationScore: 0.6,
		SurvivingMutants: []SurvivingMutant{
			{Type: "arithmetic", Line: 10},
			{Type: "arithmetic", Line: 15},
		},
	}

	recommendations := analyzer.generateRecommendations(analysis)

	found := false
	for _, rec := range recommendations {
		if contains(rec, "arithmetic") || contains(rec, "boundary") {
			found = true
			break
		}
	}
	assert.True(t, found, "should recommend tests for arithmetic mutations")
}

func TestGenerateRecommendations_LowScore(t *testing.T) {
	analyzer := NewMutantAnalyzer()

	analysis := &AnalysisResult{
		MutationScore:    0.4,
		SurvivingMutants: []SurvivingMutant{},
	}

	recommendations := analyzer.generateRecommendations(analysis)

	found := false
	for _, rec := range recommendations {
		if contains(rec, "below") || contains(rec, "threshold") {
			found = true
			break
		}
	}
	assert.True(t, found, "should warn about low score")
}

func TestBuildStrengtheningPrompt(t *testing.T) {
	config := DefaultStrengthenConfig()
	s := NewStrengthener(config, nil)

	analysis := &AnalysisResult{
		SourceFile:    "calc.go",
		TestFile:      "calc_test.go",
		MutationScore: 0.6,
		SurvivingMutants: []SurvivingMutant{
			{ID: "m1", Type: "arithmetic", Line: 10, Description: "Replaced + with -"},
		},
		WeakAreas: []WeakArea{
			{StartLine: 10, EndLine: 12, Count: 1, Severity: "low", MutantTypes: []string{"arithmetic"}},
		},
		Recommendations: []string{"Add tests for arithmetic operations"},
	}

	prompt := s.buildStrengtheningPrompt("calc.go", "calc_test.go", analysis)

	assert.Contains(t, prompt, "calc.go")
	assert.Contains(t, prompt, "calc_test.go")
	assert.Contains(t, prompt, "60.0%") // mutation score
	assert.Contains(t, prompt, "arithmetic")
	assert.Contains(t, prompt, "Line 10")
	assert.Contains(t, prompt, "Replaced + with -")
}

func TestCountNewTests_Go(t *testing.T) {
	code := `
func TestAdd_1(t *testing.T) {
    result := Add(1, 2)
    assert.Equal(t, 3, result)
}

func TestAdd_2(t *testing.T) {
    result := Add(-1, 1)
    assert.Equal(t, 0, result)
}
`
	count := countNewTests(code)
	assert.Equal(t, 2, count)
}

func TestCountNewTests_Python(t *testing.T) {
	code := `
def test_add_positive():
    assert add(1, 2) == 3

def test_add_negative():
    assert add(-1, 1) == 0
`
	count := countNewTests(code)
	assert.Equal(t, 2, count)
}

func TestCountNewTests_Empty(t *testing.T) {
	count := countNewTests("")
	assert.Equal(t, 0, count)
}

func TestAbs(t *testing.T) {
	assert.Equal(t, 5, abs(5))
	assert.Equal(t, 5, abs(-5))
	assert.Equal(t, 0, abs(0))
}

func TestStrengthen_TargetAlreadyReached(t *testing.T) {
	config := DefaultStrengthenConfig()
	s := NewStrengthener(config, nil)

	mutResult := &mutation.Result{
		SourceFile: "test.go",
		TestFile:   "test_test.go",
		Score:      0.85, // Above target of 0.80
	}

	result, err := s.Strengthen(nil, "test.go", "test_test.go", mutResult)

	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.True(t, result.ReachedTarget)
}

func TestStrengthen_NoLLMRouter(t *testing.T) {
	config := DefaultStrengthenConfig()
	s := NewStrengthener(config, nil) // No LLM router

	mutResult := &mutation.Result{
		SourceFile: "test.go",
		TestFile:   "test_test.go",
		Score:      0.5,
		Mutants: []mutation.Mutant{
			{Status: mutation.StatusSurvived, Type: "arithmetic", Line: 10},
		},
	}

	result, err := s.Strengthen(nil, "test.go", "test_test.go", mutResult)

	assert.Error(t, err)
	assert.Contains(t, result.Error, "LLM router not configured")
}

func TestSurvivingMutant_Fields(t *testing.T) {
	m := SurvivingMutant{
		ID:          "m1",
		Type:        "arithmetic",
		Description: "Replaced + with -",
		Line:        42,
		Original:    "a + b",
		Mutated:     "a - b",
	}

	assert.Equal(t, "m1", m.ID)
	assert.Equal(t, "arithmetic", m.Type)
	assert.Equal(t, 42, m.Line)
}

func TestWeakArea_Fields(t *testing.T) {
	area := WeakArea{
		StartLine:   10,
		EndLine:     20,
		MutantTypes: []string{"arithmetic", "comparison"},
		Count:       5,
		Severity:    "critical",
	}

	assert.Equal(t, 10, area.StartLine)
	assert.Equal(t, 20, area.EndLine)
	assert.Len(t, area.MutantTypes, 2)
	assert.Equal(t, 5, area.Count)
	assert.Equal(t, "critical", area.Severity)
}

func TestStrengthenResult_Fields(t *testing.T) {
	result := StrengthenResult{
		SourceFile:    "test.go",
		TestFile:      "test_test.go",
		Attempt:       1,
		PreviousScore: 0.5,
		NewScore:      0.7,
		Improvement:   0.2,
		TestsAdded:    3,
		Success:       true,
	}

	assert.Equal(t, "test.go", result.SourceFile)
	assert.Equal(t, 1, result.Attempt)
	assert.Equal(t, 0.2, result.Improvement)
	assert.Equal(t, 3, result.TestsAdded)
	assert.True(t, result.Success)
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
