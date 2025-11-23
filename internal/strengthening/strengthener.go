// Package strengthening provides test strengthening based on mutation results.
// It identifies weak tests (those with surviving mutants) and generates
// additional test cases to improve mutation scores.
package strengthening

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/QTest-hq/qtest/internal/llm"
	"github.com/QTest-hq/qtest/internal/mutation"
)

// StrengthenConfig configures the test strengthener
type StrengthenConfig struct {
	// MaxAttempts is the maximum number of strengthening attempts
	MaxAttempts int
	// MinScoreImprovement is the minimum score improvement to continue
	MinScoreImprovement float64
	// TargetScore is the target mutation score to achieve
	TargetScore float64
	// DryRun if true, doesn't write files
	DryRun bool
	// LLMTier for test generation
	LLMTier llm.Tier
}

// DefaultStrengthenConfig returns sensible defaults
func DefaultStrengthenConfig() StrengthenConfig {
	return StrengthenConfig{
		MaxAttempts:         2,
		MinScoreImprovement: 0.10, // 10% improvement required
		TargetScore:         0.80, // 80% mutation score target
		DryRun:              false,
		LLMTier:             llm.Tier2,
	}
}

// SurvivingMutant represents a mutant that survived testing
type SurvivingMutant struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Line        int    `json:"line"`
	Original    string `json:"original,omitempty"`
	Mutated     string `json:"mutated,omitempty"`
}

// AnalysisResult contains the analysis of surviving mutants
type AnalysisResult struct {
	SourceFile        string            `json:"source_file"`
	TestFile          string            `json:"test_file"`
	TotalMutants      int               `json:"total_mutants"`
	KilledMutants     int               `json:"killed_mutants"`
	SurvivingMutants  []SurvivingMutant `json:"surviving_mutants"`
	MutationScore     float64           `json:"mutation_score"`
	WeakAreas         []WeakArea        `json:"weak_areas"`
	Recommendations   []string          `json:"recommendations"`
	NeedsStrengthening bool             `json:"needs_strengthening"`
}

// WeakArea represents an area of code with poor test coverage
type WeakArea struct {
	StartLine   int      `json:"start_line"`
	EndLine     int      `json:"end_line"`
	MutantTypes []string `json:"mutant_types"`
	Count       int      `json:"count"`
	Severity    string   `json:"severity"` // "critical", "high", "medium", "low"
}

// StrengthenResult contains the result of a strengthening attempt
type StrengthenResult struct {
	SourceFile       string        `json:"source_file"`
	TestFile         string        `json:"test_file"`
	Attempt          int           `json:"attempt"`
	PreviousScore    float64       `json:"previous_score"`
	NewScore         float64       `json:"new_score"`
	Improvement      float64       `json:"improvement"`
	TestsAdded       int           `json:"tests_added"`
	NewTestCode      string        `json:"new_test_code,omitempty"`
	Success          bool          `json:"success"`
	ReachedTarget    bool          `json:"reached_target"`
	Error            string        `json:"error,omitempty"`
	Duration         time.Duration `json:"duration"`
}

// Strengthener analyzes surviving mutants and strengthens tests
type Strengthener struct {
	config    StrengthenConfig
	llmRouter *llm.Router
	analyzer  *MutantAnalyzer
}

// NewStrengthener creates a new test strengthener
func NewStrengthener(config StrengthenConfig, router *llm.Router) *Strengthener {
	return &Strengthener{
		config:    config,
		llmRouter: router,
		analyzer:  NewMutantAnalyzer(),
	}
}

// MutantAnalyzer analyzes mutation results to identify weak tests
type MutantAnalyzer struct{}

// NewMutantAnalyzer creates a new mutant analyzer
func NewMutantAnalyzer() *MutantAnalyzer {
	return &MutantAnalyzer{}
}

// Analyze analyzes mutation results and returns surviving mutant analysis
func (a *MutantAnalyzer) Analyze(result *mutation.Result) *AnalysisResult {
	analysis := &AnalysisResult{
		SourceFile:    result.SourceFile,
		TestFile:      result.TestFile,
		TotalMutants:  result.Total,
		KilledMutants: result.Killed,
		MutationScore: result.Score,
	}

	// Collect surviving mutants
	for _, m := range result.Mutants {
		if m.Status == mutation.StatusSurvived {
			analysis.SurvivingMutants = append(analysis.SurvivingMutants, SurvivingMutant{
				ID:          m.ID,
				Type:        m.Type,
				Description: m.Description,
				Line:        m.Line,
				Original:    m.Original,
				Mutated:     m.Mutated,
			})
		}
	}

	// Identify weak areas (clusters of surviving mutants)
	analysis.WeakAreas = a.identifyWeakAreas(analysis.SurvivingMutants)

	// Generate recommendations
	analysis.Recommendations = a.generateRecommendations(analysis)

	// Determine if strengthening is needed
	analysis.NeedsStrengthening = len(analysis.SurvivingMutants) > 0 && result.Score < 0.80

	return analysis
}

// identifyWeakAreas clusters surviving mutants into weak areas
func (a *MutantAnalyzer) identifyWeakAreas(mutants []SurvivingMutant) []WeakArea {
	if len(mutants) == 0 {
		return nil
	}

	// Group mutants by line proximity (within 5 lines = same area)
	areas := make(map[int]*WeakArea)
	const proximityThreshold = 5

	for _, m := range mutants {
		// Find existing area or create new one
		var foundArea *WeakArea
		for baseLine, area := range areas {
			if abs(m.Line-baseLine) <= proximityThreshold {
				foundArea = area
				break
			}
		}

		if foundArea == nil {
			areas[m.Line] = &WeakArea{
				StartLine:   m.Line,
				EndLine:     m.Line,
				MutantTypes: []string{m.Type},
				Count:       1,
			}
		} else {
			if m.Line < foundArea.StartLine {
				foundArea.StartLine = m.Line
			}
			if m.Line > foundArea.EndLine {
				foundArea.EndLine = m.Line
			}
			foundArea.Count++
			// Add unique mutant type
			found := false
			for _, t := range foundArea.MutantTypes {
				if t == m.Type {
					found = true
					break
				}
			}
			if !found {
				foundArea.MutantTypes = append(foundArea.MutantTypes, m.Type)
			}
		}
	}

	// Convert to slice and assign severity
	result := make([]WeakArea, 0, len(areas))
	for _, area := range areas {
		area.Severity = a.calculateSeverity(area)
		result = append(result, *area)
	}

	return result
}

// calculateSeverity determines the severity of a weak area
func (a *MutantAnalyzer) calculateSeverity(area *WeakArea) string {
	// Multiple surviving mutants in same area = higher severity
	if area.Count >= 5 {
		return "critical"
	}
	if area.Count >= 3 {
		return "high"
	}
	if area.Count >= 2 {
		return "medium"
	}
	return "low"
}

// generateRecommendations creates recommendations based on surviving mutants
func (a *MutantAnalyzer) generateRecommendations(analysis *AnalysisResult) []string {
	var recommendations []string

	// Analyze by mutant type
	typeCounts := make(map[string]int)
	for _, m := range analysis.SurvivingMutants {
		typeCounts[m.Type]++
	}

	for mtype, count := range typeCounts {
		switch mtype {
		case "arithmetic":
			recommendations = append(recommendations,
				fmt.Sprintf("Add %d tests for boundary values and arithmetic operations", count))
		case "comparison":
			recommendations = append(recommendations,
				fmt.Sprintf("Add %d tests for edge cases in comparison operations (==, !=, <, >, <=, >=)", count))
		case "boolean":
			recommendations = append(recommendations,
				fmt.Sprintf("Add %d tests to cover both true/false branches", count))
		case "return":
			recommendations = append(recommendations,
				fmt.Sprintf("Add %d tests to verify return values under different conditions", count))
		case "statement":
			recommendations = append(recommendations,
				fmt.Sprintf("Add %d tests to ensure all statements are reachable and necessary", count))
		case "branch":
			recommendations = append(recommendations,
				fmt.Sprintf("Add %d tests to cover untested branches", count))
		default:
			recommendations = append(recommendations,
				fmt.Sprintf("Add %d tests targeting %s mutations", count, mtype))
		}
	}

	// Add general recommendations based on score
	if analysis.MutationScore < 0.50 {
		recommendations = append(recommendations,
			"Consider comprehensive review of test coverage - score is below acceptable threshold")
	} else if analysis.MutationScore < 0.70 {
		recommendations = append(recommendations,
			"Focus on edge cases and boundary conditions to improve test effectiveness")
	}

	return recommendations
}

// Strengthen attempts to improve test quality by generating additional tests
func (s *Strengthener) Strengthen(ctx context.Context, sourceFile, testFile string, mutationResult *mutation.Result) (*StrengthenResult, error) {
	start := time.Now()

	result := &StrengthenResult{
		SourceFile:    sourceFile,
		TestFile:      testFile,
		PreviousScore: mutationResult.Score,
	}

	// Check if strengthening is needed
	if mutationResult.Score >= s.config.TargetScore {
		result.Success = true
		result.ReachedTarget = true
		result.NewScore = mutationResult.Score
		result.Duration = time.Since(start)
		return result, nil
	}

	// Analyze surviving mutants
	analysis := s.analyzer.Analyze(mutationResult)
	if !analysis.NeedsStrengthening {
		result.Success = true
		result.NewScore = mutationResult.Score
		result.Duration = time.Since(start)
		return result, nil
	}

	// Generate strengthening prompt
	prompt := s.buildStrengtheningPrompt(sourceFile, testFile, analysis)

	// Generate additional tests using LLM
	if s.llmRouter == nil {
		result.Error = "LLM router not configured"
		result.Duration = time.Since(start)
		return result, fmt.Errorf("LLM router not configured")
	}

	llmReq := &llm.Request{
		Tier:   s.config.LLMTier,
		System: "You are a test quality expert that generates test cases to improve mutation scores.",
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
		MaxTokens:   2048,
		Temperature: 0.3,
	}

	resp, err := s.llmRouter.Complete(ctx, llmReq)
	if err != nil {
		result.Error = fmt.Sprintf("LLM completion failed: %v", err)
		result.Duration = time.Since(start)
		return result, err
	}

	result.NewTestCode = resp.Content
	result.TestsAdded = countNewTests(resp.Content)
	result.Attempt = 1
	result.Duration = time.Since(start)

	// In a real implementation, we would:
	// 1. Append new tests to test file
	// 2. Run mutation testing again
	// 3. Compare scores
	// For now, we return the generated code

	result.Success = true
	return result, nil
}

// StrengthenLoop runs multiple strengthening attempts until target is reached or limits hit
func (s *Strengthener) StrengthenLoop(ctx context.Context, sourceFile, testFile string, runner *mutation.Runner) ([]StrengthenResult, error) {
	var results []StrengthenResult

	currentScore := 0.0
	cfg := mutation.DefaultConfig()

	for attempt := 1; attempt <= s.config.MaxAttempts; attempt++ {
		// Run mutation testing
		mutResult, err := runner.Run(ctx, sourceFile, testFile, cfg)
		if err != nil {
			return results, fmt.Errorf("mutation testing failed on attempt %d: %w", attempt, err)
		}

		// Check if we've reached target
		if mutResult.Score >= s.config.TargetScore {
			results = append(results, StrengthenResult{
				SourceFile:    sourceFile,
				TestFile:      testFile,
				Attempt:       attempt,
				PreviousScore: currentScore,
				NewScore:      mutResult.Score,
				Improvement:   mutResult.Score - currentScore,
				Success:       true,
				ReachedTarget: true,
			})
			break
		}

		// Check if improvement is sufficient to continue
		improvement := mutResult.Score - currentScore
		if attempt > 1 && improvement < s.config.MinScoreImprovement {
			results = append(results, StrengthenResult{
				SourceFile:    sourceFile,
				TestFile:      testFile,
				Attempt:       attempt,
				PreviousScore: currentScore,
				NewScore:      mutResult.Score,
				Improvement:   improvement,
				Success:       false,
				Error:         fmt.Sprintf("insufficient improvement: %.2f%% (min: %.2f%%)", improvement*100, s.config.MinScoreImprovement*100),
			})
			break
		}

		// Attempt strengthening
		strengthenResult, err := s.Strengthen(ctx, sourceFile, testFile, mutResult)
		if err != nil {
			results = append(results, StrengthenResult{
				SourceFile:    sourceFile,
				TestFile:      testFile,
				Attempt:       attempt,
				PreviousScore: currentScore,
				NewScore:      mutResult.Score,
				Success:       false,
				Error:         err.Error(),
			})
			break
		}

		strengthenResult.Attempt = attempt
		strengthenResult.PreviousScore = currentScore
		results = append(results, *strengthenResult)

		currentScore = mutResult.Score
	}

	return results, nil
}

// buildStrengtheningPrompt creates an LLM prompt for test strengthening
func (s *Strengthener) buildStrengtheningPrompt(sourceFile, testFile string, analysis *AnalysisResult) string {
	var sb strings.Builder

	sb.WriteString("You are a test quality expert. Analyze the following mutation testing results and generate additional test cases to kill the surviving mutants.\n\n")

	sb.WriteString(fmt.Sprintf("Source file: %s\n", sourceFile))
	sb.WriteString(fmt.Sprintf("Test file: %s\n", testFile))
	sb.WriteString(fmt.Sprintf("Current mutation score: %.1f%%\n\n", analysis.MutationScore*100))

	sb.WriteString("## Surviving Mutants\n\n")
	for _, m := range analysis.SurvivingMutants {
		sb.WriteString(fmt.Sprintf("- Line %d: [%s] %s\n", m.Line, m.Type, m.Description))
		if m.Original != "" && m.Mutated != "" {
			sb.WriteString(fmt.Sprintf("  Original: %s\n", m.Original))
			sb.WriteString(fmt.Sprintf("  Mutated:  %s\n", m.Mutated))
		}
	}

	sb.WriteString("\n## Weak Areas\n\n")
	for _, area := range analysis.WeakAreas {
		sb.WriteString(fmt.Sprintf("- Lines %d-%d: %d surviving mutants (%s severity)\n",
			area.StartLine, area.EndLine, area.Count, area.Severity))
		sb.WriteString(fmt.Sprintf("  Mutation types: %s\n", strings.Join(area.MutantTypes, ", ")))
	}

	sb.WriteString("\n## Task\n\n")
	sb.WriteString("Generate additional test cases that would detect these mutations. Focus on:\n")
	for _, rec := range analysis.Recommendations {
		sb.WriteString(fmt.Sprintf("- %s\n", rec))
	}

	sb.WriteString("\nReturn ONLY the new test function code (no imports or package declaration).\n")
	sb.WriteString("Use the same test framework as the existing tests.\n")

	return sb.String()
}

// countNewTests counts the number of test functions in generated code
func countNewTests(code string) int {
	// Simple heuristic: count "func Test" occurrences
	count := 0
	for _, line := range strings.Split(code, "\n") {
		if strings.Contains(line, "func Test") || strings.Contains(line, "def test_") {
			count++
		}
	}
	return count
}

// abs returns absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
