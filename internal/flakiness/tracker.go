// Package flakiness provides test flakiness detection and tracking.
// It monitors test results over time to identify flaky tests and
// calculate flakiness scores.
package flakiness

import (
	"sync"
	"time"
)

// TrackerConfig configures the flakiness tracker
type TrackerConfig struct {
	// WindowSize is the number of recent runs to consider
	WindowSize int
	// MinRuns is the minimum runs needed to calculate score
	MinRuns int
	// FlakyThreshold is the flakiness score threshold (0-1)
	FlakyThreshold float64
	// HighFlakyThreshold marks tests as highly flaky
	HighFlakyThreshold float64
}

// DefaultTrackerConfig returns sensible defaults
func DefaultTrackerConfig() TrackerConfig {
	return TrackerConfig{
		WindowSize:         10,
		MinRuns:            3,
		FlakyThreshold:     0.10, // 10% failure rate = flaky
		HighFlakyThreshold: 0.30, // 30% failure rate = highly flaky
	}
}

// RunResult represents the result of a single test run
type RunResult struct {
	TestID    string    `json:"test_id"`
	TestName  string    `json:"test_name"`
	Passed    bool      `json:"passed"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time `json:"timestamp"`
	Error     string    `json:"error,omitempty"`
	Attempt   int       `json:"attempt"` // Which attempt (for retries)
}

// TestHistory tracks the history of a single test
type TestHistory struct {
	TestID      string      `json:"test_id"`
	TestName    string      `json:"test_name"`
	Runs        []RunResult `json:"runs"`
	TotalRuns   int         `json:"total_runs"`
	PassedRuns  int         `json:"passed_runs"`
	FailedRuns  int         `json:"failed_runs"`
	FirstSeen   time.Time   `json:"first_seen"`
	LastSeen    time.Time   `json:"last_seen"`
	Transitions int         `json:"transitions"` // Pass→Fail or Fail→Pass count
}

// FlakinessScore contains the calculated flakiness metrics
type FlakinessScore struct {
	TestID          string  `json:"test_id"`
	TestName        string  `json:"test_name"`
	Score           float64 `json:"score"`           // 0-1, higher = more flaky
	Classification  string  `json:"classification"`  // "stable", "flaky", "highly_flaky"
	FailureRate     float64 `json:"failure_rate"`    // Simple fail/total ratio
	TransitionRate  float64 `json:"transition_rate"` // How often state changes
	RecentFailures  int     `json:"recent_failures"` // Failures in recent window
	TotalRuns       int     `json:"total_runs"`
	Recommendation  string  `json:"recommendation"`
}

// Tracker tracks test flakiness across runs
type Tracker struct {
	config   TrackerConfig
	mu       sync.RWMutex
	history  map[string]*TestHistory
}

// NewTracker creates a new flakiness tracker
func NewTracker(config TrackerConfig) *Tracker {
	return &Tracker{
		config:  config,
		history: make(map[string]*TestHistory),
	}
}

// RecordRun records a test run result
func (t *Tracker) RecordRun(result RunResult) {
	t.mu.Lock()
	defer t.mu.Unlock()

	h, exists := t.history[result.TestID]
	if !exists {
		h = &TestHistory{
			TestID:    result.TestID,
			TestName:  result.TestName,
			FirstSeen: result.Timestamp,
		}
		t.history[result.TestID] = h
	}

	// Track transitions (pass→fail or fail→pass)
	if len(h.Runs) > 0 {
		lastPassed := h.Runs[len(h.Runs)-1].Passed
		if lastPassed != result.Passed {
			h.Transitions++
		}
	}

	// Add run to history
	h.Runs = append(h.Runs, result)
	h.TotalRuns++
	h.LastSeen = result.Timestamp

	if result.Passed {
		h.PassedRuns++
	} else {
		h.FailedRuns++
	}

	// Trim to window size
	if len(h.Runs) > t.config.WindowSize*2 {
		h.Runs = h.Runs[len(h.Runs)-t.config.WindowSize*2:]
	}
}

// RecordBatchRuns records multiple run results
func (t *Tracker) RecordBatchRuns(results []RunResult) {
	for _, r := range results {
		t.RecordRun(r)
	}
}

// GetHistory returns the history for a specific test
func (t *Tracker) GetHistory(testID string) *TestHistory {
	t.mu.RLock()
	defer t.mu.RUnlock()

	h, exists := t.history[testID]
	if !exists {
		return nil
	}

	// Return a copy
	copy := *h
	copy.Runs = make([]RunResult, len(h.Runs))
	for i, r := range h.Runs {
		copy.Runs[i] = r
	}
	return &copy
}

// CalculateScore calculates the flakiness score for a test
func (t *Tracker) CalculateScore(testID string) *FlakinessScore {
	t.mu.RLock()
	defer t.mu.RUnlock()

	h, exists := t.history[testID]
	if !exists {
		return nil
	}

	return t.calculateScoreFromHistory(h)
}

// calculateScoreFromHistory calculates flakiness metrics from history
func (t *Tracker) calculateScoreFromHistory(h *TestHistory) *FlakinessScore {
	score := &FlakinessScore{
		TestID:    h.TestID,
		TestName:  h.TestName,
		TotalRuns: h.TotalRuns,
	}

	if h.TotalRuns < t.config.MinRuns {
		score.Classification = "insufficient_data"
		score.Recommendation = "Need more test runs to determine flakiness"
		return score
	}

	// Calculate failure rate
	score.FailureRate = float64(h.FailedRuns) / float64(h.TotalRuns)

	// Calculate transition rate (normalized by total runs - 1)
	if h.TotalRuns > 1 {
		score.TransitionRate = float64(h.Transitions) / float64(h.TotalRuns-1)
	}

	// Count recent failures (within window)
	recentRuns := h.Runs
	if len(recentRuns) > t.config.WindowSize {
		recentRuns = recentRuns[len(recentRuns)-t.config.WindowSize:]
	}
	for _, r := range recentRuns {
		if !r.Passed {
			score.RecentFailures++
		}
	}

	// Calculate combined flakiness score
	// Weight: 40% failure rate, 40% transition rate, 20% recent failures
	recentFailureRate := float64(score.RecentFailures) / float64(len(recentRuns))
	score.Score = (score.FailureRate * 0.4) + (score.TransitionRate * 0.4) + (recentFailureRate * 0.2)

	// Classify based on thresholds
	if score.Score >= t.config.HighFlakyThreshold {
		score.Classification = "highly_flaky"
		score.Recommendation = "Consider quarantining this test and investigating root cause"
	} else if score.Score >= t.config.FlakyThreshold {
		score.Classification = "flaky"
		score.Recommendation = "Investigate intermittent failures; may need test isolation"
	} else {
		score.Classification = "stable"
		score.Recommendation = "Test is reliable"
	}

	return score
}

// GetAllScores calculates flakiness scores for all tracked tests
func (t *Tracker) GetAllScores() []*FlakinessScore {
	t.mu.RLock()
	defer t.mu.RUnlock()

	scores := make([]*FlakinessScore, 0, len(t.history))
	for _, h := range t.history {
		scores = append(scores, t.calculateScoreFromHistory(h))
	}
	return scores
}

// GetFlakyTests returns tests classified as flaky or highly flaky
func (t *Tracker) GetFlakyTests() []*FlakinessScore {
	scores := t.GetAllScores()
	var flaky []*FlakinessScore
	for _, s := range scores {
		if s.Classification == "flaky" || s.Classification == "highly_flaky" {
			flaky = append(flaky, s)
		}
	}
	return flaky
}

// GetHighlyFlakyTests returns only highly flaky tests
func (t *Tracker) GetHighlyFlakyTests() []*FlakinessScore {
	scores := t.GetAllScores()
	var highlyFlaky []*FlakinessScore
	for _, s := range scores {
		if s.Classification == "highly_flaky" {
			highlyFlaky = append(highlyFlaky, s)
		}
	}
	return highlyFlaky
}

// Summary provides an overview of test flakiness
type Summary struct {
	TotalTests      int     `json:"total_tests"`
	StableTests     int     `json:"stable_tests"`
	FlakyTests      int     `json:"flaky_tests"`
	HighlyFlakyTests int    `json:"highly_flaky_tests"`
	InsufficientData int    `json:"insufficient_data"`
	AverageScore    float64 `json:"average_score"`
	WorstTests      []*FlakinessScore `json:"worst_tests"`
}

// GetSummary returns a summary of all tracked tests
func (t *Tracker) GetSummary() *Summary {
	scores := t.GetAllScores()

	summary := &Summary{
		TotalTests: len(scores),
	}

	var totalScore float64
	scoredCount := 0

	for _, s := range scores {
		switch s.Classification {
		case "stable":
			summary.StableTests++
			totalScore += s.Score
			scoredCount++
		case "flaky":
			summary.FlakyTests++
			totalScore += s.Score
			scoredCount++
		case "highly_flaky":
			summary.HighlyFlakyTests++
			totalScore += s.Score
			scoredCount++
		case "insufficient_data":
			summary.InsufficientData++
		}
	}

	if scoredCount > 0 {
		summary.AverageScore = totalScore / float64(scoredCount)
	}

	// Get top 5 worst tests
	worst := t.GetFlakyTests()
	if len(worst) > 5 {
		// Sort by score descending
		for i := 0; i < len(worst)-1; i++ {
			for j := i + 1; j < len(worst); j++ {
				if worst[j].Score > worst[i].Score {
					worst[i], worst[j] = worst[j], worst[i]
				}
			}
		}
		worst = worst[:5]
	}
	summary.WorstTests = worst

	return summary
}

// ClearHistory removes all tracked history
func (t *Tracker) ClearHistory() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.history = make(map[string]*TestHistory)
}

// ClearTest removes history for a specific test
func (t *Tracker) ClearTest(testID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.history, testID)
}

// IsFlaky returns true if the test is classified as flaky
func (t *Tracker) IsFlaky(testID string) bool {
	score := t.CalculateScore(testID)
	if score == nil {
		return false
	}
	return score.Classification == "flaky" || score.Classification == "highly_flaky"
}

// ShouldQuarantine returns true if the test should be quarantined
func (t *Tracker) ShouldQuarantine(testID string) bool {
	score := t.CalculateScore(testID)
	if score == nil {
		return false
	}
	return score.Classification == "highly_flaky"
}
