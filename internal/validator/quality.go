// Package validator provides test validation and quality analysis
package validator

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
)

// QualityScore represents the overall quality assessment of a test
type QualityScore struct {
	Score             float64              `json:"score"`              // 0-100
	Grade             string               `json:"grade"`              // A, B, C, D, F
	Passed            bool                 `json:"passed"`             // Meets minimum threshold
	AssertionScore    float64              `json:"assertion_score"`    // 0-100
	CoverageScore     float64              `json:"coverage_score"`     // 0-100
	MutationScore     float64              `json:"mutation_score"`     // 0-100
	StaticScore       float64              `json:"static_score"`       // 0-100
	Issues            []QualityIssue       `json:"issues,omitempty"`
	Breakdown         QualityBreakdown     `json:"breakdown"`
	Recommendation    string               `json:"recommendation,omitempty"`
}

// QualityIssue represents a specific quality problem
type QualityIssue struct {
	Severity    string `json:"severity"`    // critical, warning, info
	Category    string `json:"category"`    // assertion, coverage, mutation, static
	Message     string `json:"message"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// QualityBreakdown shows individual metrics
type QualityBreakdown struct {
	AssertionCount       int     `json:"assertion_count"`
	TrivialAssertions    int     `json:"trivial_assertions"`
	TestsWithAssertions  int     `json:"tests_with_assertions"`
	TotalTests           int     `json:"total_tests"`
	CoveragePercent      float64 `json:"coverage_percent"`
	TargetFuncCovered    bool    `json:"target_func_covered"`
	MutationKillRate     float64 `json:"mutation_kill_rate"`
	MutantsKilled        int     `json:"mutants_killed"`
	MutantsTotal         int     `json:"mutants_total"`
}

// QualityConfig configures quality thresholds
type QualityConfig struct {
	MinScore               float64 `json:"min_score"`               // Minimum overall score to pass (default 60)
	MinAssertions          int     `json:"min_assertions"`          // Minimum assertions per test (default 1)
	MinCoverage            float64 `json:"min_coverage"`            // Minimum coverage % (default 50)
	MinMutationKillRate    float64 `json:"min_mutation_kill_rate"`  // Minimum mutation score (default 50)
	MaxTrivialAssertions   float64 `json:"max_trivial_assertions"`  // Max % of trivial assertions (default 25)
	RequireTargetCoverage  bool    `json:"require_target_coverage"` // Must cover target function

	// Weights for score calculation
	AssertionWeight  float64 `json:"assertion_weight"`
	CoverageWeight   float64 `json:"coverage_weight"`
	MutationWeight   float64 `json:"mutation_weight"`
	StaticWeight     float64 `json:"static_weight"`
}

// DefaultQualityConfig returns sensible defaults
func DefaultQualityConfig() QualityConfig {
	return QualityConfig{
		MinScore:              60.0,
		MinAssertions:         1,
		MinCoverage:           50.0,
		MinMutationKillRate:   50.0,
		MaxTrivialAssertions:  25.0,
		RequireTargetCoverage: true,
		AssertionWeight:       0.20,
		CoverageWeight:        0.20,
		MutationWeight:        0.40,
		StaticWeight:          0.20,
	}
}

// QualityChecker performs comprehensive quality assessment
type QualityChecker struct {
	config         QualityConfig
	language       string
	targetFile     string
	targetFunction string
	workDir        string
}

// NewQualityChecker creates a new quality checker
func NewQualityChecker(config QualityConfig, language, workDir, targetFile, targetFunction string) *QualityChecker {
	return &QualityChecker{
		config:         config,
		language:       language,
		workDir:        workDir,
		targetFile:     targetFile,
		targetFunction: targetFunction,
	}
}

// Assess performs full quality assessment on a test file
func (q *QualityChecker) Assess(ctx context.Context, testFile string, mutationScore float64) (*QualityScore, error) {
	result := &QualityScore{
		Issues: []QualityIssue{},
	}

	// Read test code
	code, err := os.ReadFile(testFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read test file: %w", err)
	}

	// 1. Assertion Analysis
	analyzer := NewAnalyzer(q.language, q.targetFunction)
	assertionAnalysis := analyzer.AnalyzeAssertions(string(code))
	result.AssertionScore = q.scoreAssertions(assertionAnalysis, result)
	result.Breakdown.AssertionCount = assertionAnalysis.TotalAssertions
	result.Breakdown.TrivialAssertions = assertionAnalysis.TrivialAssertions
	result.Breakdown.TotalTests = len(assertionAnalysis.AssertionsByTest)

	testsWithAssertions := 0
	for _, count := range assertionAnalysis.AssertionsByTest {
		if count > 0 {
			testsWithAssertions++
		}
	}
	result.Breakdown.TestsWithAssertions = testsWithAssertions

	// 2. Coverage Analysis (optional - may fail if coverage tools not available)
	covChecker := NewCoverageChecker(q.workDir, q.language, q.targetFile, q.targetFunction)
	covResult, err := covChecker.RunWithCoverage(ctx, testFile)
	if err != nil {
		log.Warn().Err(err).Msg("coverage analysis failed, using default score")
		result.CoverageScore = 50.0 // Default if coverage fails
	} else {
		result.CoverageScore = q.scoreCoverage(covResult, result)
		result.Breakdown.CoveragePercent = covResult.TotalCoverage
		result.Breakdown.TargetFuncCovered = covResult.TargetFuncCovered
	}

	// 3. Mutation Score (passed in from mutation testing phase)
	result.MutationScore = q.scoreMutation(mutationScore, result)
	result.Breakdown.MutationKillRate = mutationScore

	// 4. Static Analysis Score
	result.StaticScore = q.scoreStatic(assertionAnalysis, result)

	// Calculate overall score
	result.Score = q.calculateOverallScore(result)
	result.Grade = q.calculateGrade(result.Score)
	result.Passed = result.Score >= q.config.MinScore
	result.Recommendation = q.generateRecommendation(result)

	log.Info().
		Float64("score", result.Score).
		Str("grade", result.Grade).
		Bool("passed", result.Passed).
		Int("issues", len(result.Issues)).
		Msg("quality assessment complete")

	return result, nil
}

// scoreAssertions calculates assertion quality score
func (q *QualityChecker) scoreAssertions(analysis *AssertionAnalysis, result *QualityScore) float64 {
	score := 100.0

	// Check for empty tests
	emptyTests := 0
	for _, count := range analysis.AssertionsByTest {
		if count == 0 {
			emptyTests++
		}
	}

	if emptyTests > 0 {
		penalty := float64(emptyTests) / float64(len(analysis.AssertionsByTest)) * 50
		score -= penalty
		result.Issues = append(result.Issues, QualityIssue{
			Severity:   "critical",
			Category:   "assertion",
			Message:    fmt.Sprintf("%d test(s) have no assertions", emptyTests),
			Suggestion: "Add meaningful assertions to verify expected behavior",
		})
	}

	// Check assertion count per test
	if analysis.TotalAssertions > 0 && len(analysis.AssertionsByTest) > 0 {
		avgAssertions := float64(analysis.TotalAssertions) / float64(len(analysis.AssertionsByTest))
		if avgAssertions < float64(q.config.MinAssertions) {
			score -= 20
			result.Issues = append(result.Issues, QualityIssue{
				Severity:   "warning",
				Category:   "assertion",
				Message:    fmt.Sprintf("Low assertion density: %.1f per test", avgAssertions),
				Suggestion: "Add more assertions to thoroughly test the function",
			})
		}
	}

	// Penalize trivial assertions
	if analysis.TotalAssertions > 0 {
		trivialPct := float64(analysis.TrivialAssertions) / float64(analysis.TotalAssertions) * 100
		if trivialPct > q.config.MaxTrivialAssertions {
			penalty := (trivialPct - q.config.MaxTrivialAssertions) / 2
			score -= penalty
			result.Issues = append(result.Issues, QualityIssue{
				Severity:   "warning",
				Category:   "assertion",
				Message:    fmt.Sprintf("%.0f%% of assertions are trivial", trivialPct),
				Suggestion: "Replace trivial assertions with meaningful comparisons",
			})
		}
	}

	// Check target function coverage
	if !analysis.TargetFuncCalled && q.targetFunction != "" {
		score -= 30
		result.Issues = append(result.Issues, QualityIssue{
			Severity:   "critical",
			Category:   "assertion",
			Message:    fmt.Sprintf("Target function '%s' is never called", q.targetFunction),
			Suggestion: "Ensure the test actually invokes the function being tested",
		})
	}

	if score < 0 {
		score = 0
	}
	return score
}

// scoreCoverage calculates coverage quality score
func (q *QualityChecker) scoreCoverage(cov *CoverageResult, result *QualityScore) float64 {
	score := 100.0

	// Score based on total coverage
	if cov.TotalCoverage < q.config.MinCoverage {
		penalty := (q.config.MinCoverage - cov.TotalCoverage) / 2
		score -= penalty
		result.Issues = append(result.Issues, QualityIssue{
			Severity:   "warning",
			Category:   "coverage",
			Message:    fmt.Sprintf("Code coverage %.1f%% is below minimum %.1f%%", cov.TotalCoverage, q.config.MinCoverage),
			Suggestion: "Add tests for uncovered code paths",
		})
	}

	// Bonus for high coverage
	if cov.TotalCoverage >= 80 {
		score = 100
	}

	// Critical: target function must be covered
	if q.config.RequireTargetCoverage && !cov.TargetFuncCovered {
		score -= 40
		result.Issues = append(result.Issues, QualityIssue{
			Severity:   "critical",
			Category:   "coverage",
			Message:    "Target function is not covered by tests",
			Suggestion: "Ensure tests execute the target function's code",
		})
	}

	if score < 0 {
		score = 0
	}
	return score
}

// scoreMutation calculates mutation testing score
func (q *QualityChecker) scoreMutation(mutationScore float64, result *QualityScore) float64 {
	score := mutationScore * 100 // Convert 0-1 to 0-100

	if mutationScore < q.config.MinMutationKillRate/100 {
		result.Issues = append(result.Issues, QualityIssue{
			Severity:   "critical",
			Category:   "mutation",
			Message:    fmt.Sprintf("Mutation kill rate %.1f%% is below minimum %.1f%%", mutationScore*100, q.config.MinMutationKillRate),
			Suggestion: "Tests are not detecting code changes - add more specific assertions",
		})
	}

	if mutationScore < 0.3 {
		result.Issues = append(result.Issues, QualityIssue{
			Severity:   "critical",
			Category:   "mutation",
			Message:    "Very low mutation score indicates tests may not be effective",
			Suggestion: "Review test logic - tests should fail when code is broken",
		})
	}

	return score
}

// scoreStatic calculates static analysis score
func (q *QualityChecker) scoreStatic(analysis *AssertionAnalysis, result *QualityScore) float64 {
	score := 100.0

	// Penalize based on issues found
	for _, issue := range analysis.Issues {
		switch issue.Type {
		case IssueNoAssertions:
			score -= 25
		case IssueTrivialAssertion:
			score -= 10
		case IssueConstantComparison:
			score -= 10
		case IssueTautology:
			score -= 15
		case IssueTargetNotCalled:
			score -= 30
		}
	}

	// Check assertion variety
	if len(analysis.AssertionTypes) == 1 && analysis.TotalAssertions > 3 {
		score -= 10
		result.Issues = append(result.Issues, QualityIssue{
			Severity:   "info",
			Category:   "static",
			Message:    "All assertions are of the same type",
			Suggestion: "Consider testing different aspects (errors, edge cases, types)",
		})
	}

	if score < 0 {
		score = 0
	}
	return score
}

// calculateOverallScore combines all scores
func (q *QualityChecker) calculateOverallScore(result *QualityScore) float64 {
	return result.AssertionScore*q.config.AssertionWeight +
		result.CoverageScore*q.config.CoverageWeight +
		result.MutationScore*q.config.MutationWeight +
		result.StaticScore*q.config.StaticWeight
}

// calculateGrade converts score to letter grade
func (q *QualityChecker) calculateGrade(score float64) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

// generateRecommendation creates actionable feedback
func (q *QualityChecker) generateRecommendation(result *QualityScore) string {
	if result.Passed {
		if result.Grade == "A" {
			return "Excellent test quality - ready for integration"
		}
		return "Test meets minimum quality standards"
	}

	// Find most critical issue
	var criticalIssues []string
	for _, issue := range result.Issues {
		if issue.Severity == "critical" {
			criticalIssues = append(criticalIssues, issue.Suggestion)
		}
	}

	if len(criticalIssues) > 0 {
		return "REGENERATE: " + strings.Join(criticalIssues, "; ")
	}

	return "Test quality below threshold - consider regenerating with better prompts"
}

// ShouldRegenerate determines if test should be regenerated
func (q *QualityChecker) ShouldRegenerate(result *QualityScore) (bool, string) {
	if result.Passed {
		return false, ""
	}

	// Collect critical reasons
	var reasons []string
	for _, issue := range result.Issues {
		if issue.Severity == "critical" {
			reasons = append(reasons, issue.Message)
		}
	}

	if len(reasons) == 0 {
		reasons = append(reasons, fmt.Sprintf("Quality score %.1f%% below minimum %.1f%%", result.Score, q.config.MinScore))
	}

	return true, strings.Join(reasons, "; ")
}
