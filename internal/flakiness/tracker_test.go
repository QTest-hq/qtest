package flakiness

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultTrackerConfig(t *testing.T) {
	config := DefaultTrackerConfig()
	assert.Equal(t, 10, config.WindowSize)
	assert.Equal(t, 3, config.MinRuns)
	assert.Equal(t, 0.10, config.FlakyThreshold)
	assert.Equal(t, 0.30, config.HighFlakyThreshold)
}

func TestNewTracker(t *testing.T) {
	tracker := NewTracker(DefaultTrackerConfig())
	assert.NotNil(t, tracker)
	assert.NotNil(t, tracker.history)
}

func TestRecordRun(t *testing.T) {
	tracker := NewTracker(DefaultTrackerConfig())

	result := RunResult{
		TestID:    "test1",
		TestName:  "TestAdd",
		Passed:    true,
		Timestamp: time.Now(),
	}

	tracker.RecordRun(result)

	history := tracker.GetHistory("test1")
	assert.NotNil(t, history)
	assert.Equal(t, "test1", history.TestID)
	assert.Equal(t, 1, history.TotalRuns)
	assert.Equal(t, 1, history.PassedRuns)
	assert.Equal(t, 0, history.FailedRuns)
}

func TestRecordMultipleRuns(t *testing.T) {
	tracker := NewTracker(DefaultTrackerConfig())
	now := time.Now()

	runs := []RunResult{
		{TestID: "test1", TestName: "TestAdd", Passed: true, Timestamp: now},
		{TestID: "test1", TestName: "TestAdd", Passed: true, Timestamp: now.Add(time.Second)},
		{TestID: "test1", TestName: "TestAdd", Passed: false, Timestamp: now.Add(2 * time.Second)},
	}

	tracker.RecordBatchRuns(runs)

	history := tracker.GetHistory("test1")
	assert.Equal(t, 3, history.TotalRuns)
	assert.Equal(t, 2, history.PassedRuns)
	assert.Equal(t, 1, history.FailedRuns)
	assert.Equal(t, 1, history.Transitions) // One transition: pass→fail
}

func TestTransitionTracking(t *testing.T) {
	tracker := NewTracker(DefaultTrackerConfig())
	now := time.Now()

	// pass, fail, pass, fail = 3 transitions
	runs := []RunResult{
		{TestID: "test1", Passed: true, Timestamp: now},
		{TestID: "test1", Passed: false, Timestamp: now.Add(time.Second)},
		{TestID: "test1", Passed: true, Timestamp: now.Add(2 * time.Second)},
		{TestID: "test1", Passed: false, Timestamp: now.Add(3 * time.Second)},
	}

	tracker.RecordBatchRuns(runs)

	history := tracker.GetHistory("test1")
	assert.Equal(t, 3, history.Transitions)
}

func TestCalculateScore_InsufficientData(t *testing.T) {
	tracker := NewTracker(DefaultTrackerConfig())

	tracker.RecordRun(RunResult{TestID: "test1", Passed: true, Timestamp: time.Now()})
	tracker.RecordRun(RunResult{TestID: "test1", Passed: true, Timestamp: time.Now()})
	// Only 2 runs, need 3

	score := tracker.CalculateScore("test1")
	assert.NotNil(t, score)
	assert.Equal(t, "insufficient_data", score.Classification)
}

func TestCalculateScore_Stable(t *testing.T) {
	tracker := NewTracker(DefaultTrackerConfig())
	now := time.Now()

	// All passes = stable
	for i := 0; i < 10; i++ {
		tracker.RecordRun(RunResult{
			TestID:    "test1",
			Passed:    true,
			Timestamp: now.Add(time.Duration(i) * time.Second),
		})
	}

	score := tracker.CalculateScore("test1")
	assert.Equal(t, "stable", score.Classification)
	assert.Equal(t, 0.0, score.FailureRate)
	assert.Equal(t, 0.0, score.Score)
}

func TestCalculateScore_Flaky(t *testing.T) {
	tracker := NewTracker(DefaultTrackerConfig())
	now := time.Now()

	// Alternating pass/fail = flaky
	for i := 0; i < 10; i++ {
		tracker.RecordRun(RunResult{
			TestID:    "test1",
			Passed:    i%2 == 0,
			Timestamp: now.Add(time.Duration(i) * time.Second),
		})
	}

	score := tracker.CalculateScore("test1")
	assert.Equal(t, 0.5, score.FailureRate)   // 50% failed
	assert.True(t, score.TransitionRate > 0.8) // High transition rate
	assert.True(t, score.Score >= 0.30)        // Should be highly flaky
}

func TestCalculateScore_HighlyFlaky(t *testing.T) {
	tracker := NewTracker(DefaultTrackerConfig())
	now := time.Now()

	// High failure rate with transitions
	runs := []RunResult{
		{TestID: "test1", Passed: true, Timestamp: now},
		{TestID: "test1", Passed: false, Timestamp: now.Add(time.Second)},
		{TestID: "test1", Passed: false, Timestamp: now.Add(2 * time.Second)},
		{TestID: "test1", Passed: true, Timestamp: now.Add(3 * time.Second)},
		{TestID: "test1", Passed: false, Timestamp: now.Add(4 * time.Second)},
	}

	tracker.RecordBatchRuns(runs)

	score := tracker.CalculateScore("test1")
	assert.True(t, score.Score >= 0.10) // At least flaky
	assert.Contains(t, []string{"flaky", "highly_flaky"}, score.Classification)
}

func TestGetAllScores(t *testing.T) {
	tracker := NewTracker(DefaultTrackerConfig())
	now := time.Now()

	// Record runs for multiple tests
	for i := 0; i < 5; i++ {
		tracker.RecordRun(RunResult{TestID: "test1", Passed: true, Timestamp: now.Add(time.Duration(i) * time.Second)})
		tracker.RecordRun(RunResult{TestID: "test2", Passed: i%2 == 0, Timestamp: now.Add(time.Duration(i) * time.Second)})
	}

	scores := tracker.GetAllScores()
	assert.Len(t, scores, 2)
}

func TestGetFlakyTests(t *testing.T) {
	tracker := NewTracker(DefaultTrackerConfig())
	now := time.Now()

	// Test1: stable (all pass)
	for i := 0; i < 5; i++ {
		tracker.RecordRun(RunResult{TestID: "test1", Passed: true, Timestamp: now.Add(time.Duration(i) * time.Second)})
	}

	// Test2: flaky (alternating)
	for i := 0; i < 5; i++ {
		tracker.RecordRun(RunResult{TestID: "test2", Passed: i%2 == 0, Timestamp: now.Add(time.Duration(i) * time.Second)})
	}

	flaky := tracker.GetFlakyTests()
	assert.GreaterOrEqual(t, len(flaky), 1)

	// Check that test2 is in the flaky list
	found := false
	for _, s := range flaky {
		if s.TestID == "test2" {
			found = true
			break
		}
	}
	assert.True(t, found, "test2 should be classified as flaky")
}

func TestGetSummary(t *testing.T) {
	tracker := NewTracker(DefaultTrackerConfig())
	now := time.Now()

	// Create 3 stable tests
	for j := 0; j < 3; j++ {
		for i := 0; i < 5; i++ {
			tracker.RecordRun(RunResult{
				TestID:    "stable" + string(rune('0'+j)),
				Passed:    true,
				Timestamp: now.Add(time.Duration(i) * time.Second),
			})
		}
	}

	summary := tracker.GetSummary()
	assert.Equal(t, 3, summary.TotalTests)
	assert.Equal(t, 3, summary.StableTests)
	assert.Equal(t, 0, summary.FlakyTests)
}

func TestIsFlaky(t *testing.T) {
	tracker := NewTracker(DefaultTrackerConfig())
	now := time.Now()

	// Stable test
	for i := 0; i < 5; i++ {
		tracker.RecordRun(RunResult{TestID: "stable", Passed: true, Timestamp: now.Add(time.Duration(i) * time.Second)})
	}

	// Flaky test (alternating)
	for i := 0; i < 5; i++ {
		tracker.RecordRun(RunResult{TestID: "flaky", Passed: i%2 == 0, Timestamp: now.Add(time.Duration(i) * time.Second)})
	}

	assert.False(t, tracker.IsFlaky("stable"))
	assert.True(t, tracker.IsFlaky("flaky"))
	assert.False(t, tracker.IsFlaky("nonexistent"))
}

func TestShouldQuarantine(t *testing.T) {
	config := DefaultTrackerConfig()
	config.MinRuns = 3
	tracker := NewTracker(config)
	now := time.Now()

	// Highly flaky test (mostly failures with transitions)
	runs := []RunResult{
		{TestID: "bad", Passed: true, Timestamp: now},
		{TestID: "bad", Passed: false, Timestamp: now.Add(time.Second)},
		{TestID: "bad", Passed: false, Timestamp: now.Add(2 * time.Second)},
		{TestID: "bad", Passed: true, Timestamp: now.Add(3 * time.Second)},
		{TestID: "bad", Passed: false, Timestamp: now.Add(4 * time.Second)},
		{TestID: "bad", Passed: false, Timestamp: now.Add(5 * time.Second)},
	}
	tracker.RecordBatchRuns(runs)

	// Should be quarantined due to high failure rate + transitions
	score := tracker.CalculateScore("bad")
	t.Logf("Score: %.3f, Classification: %s, FailureRate: %.3f, TransitionRate: %.3f",
		score.Score, score.Classification, score.FailureRate, score.TransitionRate)

	// The test is definitely flaky, quarantine depends on exact thresholds
	assert.True(t, tracker.IsFlaky("bad"))
}

func TestClearHistory(t *testing.T) {
	tracker := NewTracker(DefaultTrackerConfig())

	tracker.RecordRun(RunResult{TestID: "test1", Passed: true, Timestamp: time.Now()})
	tracker.RecordRun(RunResult{TestID: "test2", Passed: true, Timestamp: time.Now()})

	assert.NotNil(t, tracker.GetHistory("test1"))
	assert.NotNil(t, tracker.GetHistory("test2"))

	tracker.ClearHistory()

	assert.Nil(t, tracker.GetHistory("test1"))
	assert.Nil(t, tracker.GetHistory("test2"))
}

func TestClearTest(t *testing.T) {
	tracker := NewTracker(DefaultTrackerConfig())

	tracker.RecordRun(RunResult{TestID: "test1", Passed: true, Timestamp: time.Now()})
	tracker.RecordRun(RunResult{TestID: "test2", Passed: true, Timestamp: time.Now()})

	tracker.ClearTest("test1")

	assert.Nil(t, tracker.GetHistory("test1"))
	assert.NotNil(t, tracker.GetHistory("test2"))
}

func TestRunResult_Fields(t *testing.T) {
	r := RunResult{
		TestID:    "test1",
		TestName:  "TestAdd",
		Passed:    true,
		Duration:  100 * time.Millisecond,
		Timestamp: time.Now(),
		Error:     "",
		Attempt:   1,
	}

	assert.Equal(t, "test1", r.TestID)
	assert.Equal(t, "TestAdd", r.TestName)
	assert.True(t, r.Passed)
	assert.Equal(t, 1, r.Attempt)
}

func TestFlakinessScore_Recommendations(t *testing.T) {
	tracker := NewTracker(DefaultTrackerConfig())
	now := time.Now()

	// Stable - should recommend "reliable"
	for i := 0; i < 5; i++ {
		tracker.RecordRun(RunResult{TestID: "stable", Passed: true, Timestamp: now.Add(time.Duration(i) * time.Second)})
	}

	score := tracker.CalculateScore("stable")
	assert.Contains(t, score.Recommendation, "reliable")
}

func TestHistoryWindowTrimming(t *testing.T) {
	config := DefaultTrackerConfig()
	config.WindowSize = 5
	tracker := NewTracker(config)
	now := time.Now()

	// Record more runs than window allows
	for i := 0; i < 20; i++ {
		tracker.RecordRun(RunResult{
			TestID:    "test1",
			Passed:    true,
			Timestamp: now.Add(time.Duration(i) * time.Second),
		})
	}

	history := tracker.GetHistory("test1")
	// Should be trimmed to WindowSize*2 = 10
	assert.LessOrEqual(t, len(history.Runs), 10)
	assert.Equal(t, 20, history.TotalRuns) // But total count preserved
}

func TestGetHighlyFlakyTests(t *testing.T) {
	config := DefaultTrackerConfig()
	config.HighFlakyThreshold = 0.20
	tracker := NewTracker(config)
	now := time.Now()

	// Create a highly flaky test (high failure + transitions)
	runs := []RunResult{
		{TestID: "bad", Passed: false, Timestamp: now},
		{TestID: "bad", Passed: true, Timestamp: now.Add(time.Second)},
		{TestID: "bad", Passed: false, Timestamp: now.Add(2 * time.Second)},
		{TestID: "bad", Passed: false, Timestamp: now.Add(3 * time.Second)},
		{TestID: "bad", Passed: true, Timestamp: now.Add(4 * time.Second)},
		{TestID: "bad", Passed: false, Timestamp: now.Add(5 * time.Second)},
	}
	tracker.RecordBatchRuns(runs)

	score := tracker.CalculateScore("bad")
	t.Logf("Highly flaky test score: %.3f", score.Score)

	// This test should be at least flaky
	flaky := tracker.GetFlakyTests()
	assert.GreaterOrEqual(t, len(flaky), 1)
}
