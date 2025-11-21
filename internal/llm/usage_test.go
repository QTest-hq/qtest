package llm

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewUsageTracker(t *testing.T) {
	tracker := NewUsageTracker(UsageTrackerConfig{
		MaxRecords: 100,
	})

	if tracker == nil {
		t.Fatal("Expected non-nil tracker")
	}

	if tracker.maxRecords != 100 {
		t.Errorf("MaxRecords = %d, want 100", tracker.maxRecords)
	}
}

func TestNewUsageTracker_Defaults(t *testing.T) {
	tracker := NewUsageTracker(UsageTrackerConfig{})

	if tracker.maxRecords != 1000 {
		t.Errorf("Default MaxRecords = %d, want 1000", tracker.maxRecords)
	}
}

func TestUsageTracker_Record(t *testing.T) {
	tracker := NewUsageTracker(UsageTrackerConfig{
		MaxRecords: 10,
	})

	record := UsageRecord{
		Provider:     ProviderOllama,
		Model:        "qwen2.5-coder:7b",
		Tier:         Tier1,
		InputTokens:  100,
		OutputTokens: 50,
	}

	tracker.Record(record)

	stats := tracker.GetStats()
	if stats.TotalTokens != 150 {
		t.Errorf("TotalTokens = %d, want 150", stats.TotalTokens)
	}
}

func TestUsageTracker_RecentRecords(t *testing.T) {
	tracker := NewUsageTracker(UsageTrackerConfig{
		MaxRecords: 10,
	})

	// Record 5 usage events
	for i := 0; i < 5; i++ {
		tracker.Record(UsageRecord{
			Provider:     ProviderOllama,
			Model:        "test-model",
			InputTokens:  100,
			OutputTokens: 50,
		})
	}

	records := tracker.RecentRecords(10)
	if len(records) != 5 {
		t.Errorf("RecentRecords returned %d records, want 5", len(records))
	}
}

func TestUsageTracker_CheckBudget_NoLimit(t *testing.T) {
	tracker := NewUsageTracker(UsageTrackerConfig{})

	// No limits set, should always pass
	err := tracker.CheckBudget(1000000)
	if err != nil {
		t.Errorf("CheckBudget should pass with no limits, got: %v", err)
	}
}

func TestUsageTracker_CheckBudget_HourlyLimit(t *testing.T) {
	tracker := NewUsageTracker(UsageTrackerConfig{
		Budget: BudgetConfig{
			HourlyTokenLimit: 1000,
		},
	})

	// First request should pass
	err := tracker.CheckBudget(500)
	if err != nil {
		t.Errorf("First request should pass: %v", err)
	}

	// Record some usage
	tracker.Record(UsageRecord{
		Provider:     ProviderOllama,
		InputTokens:  800,
		OutputTokens: 200,
	})

	// Should exceed limit now
	err = tracker.CheckBudget(100)
	if err != ErrBudgetExceeded {
		t.Errorf("Expected ErrBudgetExceeded, got: %v", err)
	}
}

func TestUsageTracker_CheckBudget_DailyLimit(t *testing.T) {
	tracker := NewUsageTracker(UsageTrackerConfig{
		Budget: BudgetConfig{
			DailyTokenLimit: 5000,
		},
	})

	// Record usage up to limit
	tracker.Record(UsageRecord{
		Provider:     ProviderOllama,
		InputTokens:  3000,
		OutputTokens: 2000,
	})

	// Should fail
	err := tracker.CheckBudget(100)
	if err != ErrBudgetExceeded {
		t.Errorf("Expected ErrBudgetExceeded, got: %v", err)
	}
}

func TestUsageTracker_CheckBudget_MonthlyBudget(t *testing.T) {
	tracker := NewUsageTracker(UsageTrackerConfig{
		Budget: BudgetConfig{
			MonthlyBudgetUSD: 10.0,
		},
	})

	// Record expensive usage (Anthropic)
	for i := 0; i < 100; i++ {
		tracker.Record(UsageRecord{
			Provider:     ProviderAnthropic,
			Model:        "claude-3-5-sonnet-20241022",
			InputTokens:  5000,
			OutputTokens: 1000,
		})
	}

	// Should exceed budget
	err := tracker.CheckBudget(100)
	if err != ErrBudgetExceeded {
		t.Errorf("Expected ErrBudgetExceeded, got: %v", err)
	}
}

func TestUsageTracker_CheckBudget_RateLimit(t *testing.T) {
	tracker := NewUsageTracker(UsageTrackerConfig{
		Budget: BudgetConfig{
			RequestsPerMinute: 5,
		},
	})

	// Make requests up to limit
	for i := 0; i < 5; i++ {
		tracker.IncrementRequests()
	}

	// Should be rate limited
	err := tracker.CheckBudget(100)
	if err != ErrRateLimited {
		t.Errorf("Expected ErrRateLimited, got: %v", err)
	}
}

func TestUsageTracker_GetBudgetStatus(t *testing.T) {
	tracker := NewUsageTracker(UsageTrackerConfig{
		Budget: BudgetConfig{
			HourlyTokenLimit: 10000,
			DailyTokenLimit:  100000,
			MonthlyBudgetUSD: 100.0,
		},
	})

	// Record some usage
	tracker.Record(UsageRecord{
		Provider:     ProviderOllama,
		InputTokens:  500,
		OutputTokens: 500,
	})

	status := tracker.GetBudgetStatus()

	if status.HourlyTokensUsed != 1000 {
		t.Errorf("HourlyTokensUsed = %d, want 1000", status.HourlyTokensUsed)
	}
	if status.HourlyTokensLimit != 10000 {
		t.Errorf("HourlyTokensLimit = %d, want 10000", status.HourlyTokensLimit)
	}
	if status.HourlyPercentUsed != 10.0 {
		t.Errorf("HourlyPercentUsed = %f, want 10.0", status.HourlyPercentUsed)
	}
}

func TestUsageTracker_EstimateCost(t *testing.T) {
	tracker := NewUsageTracker(UsageTrackerConfig{})

	// Ollama should be free
	ollamaRecord := UsageRecord{
		Provider:    ProviderOllama,
		TotalTokens: 1000,
	}
	ollamaCost := tracker.estimateCost(ollamaRecord)
	if ollamaCost != 0 {
		t.Errorf("Ollama cost = %f, want 0", ollamaCost)
	}

	// Anthropic should have cost
	anthropicRecord := UsageRecord{
		Provider:    ProviderAnthropic,
		Model:       "claude-3-5-sonnet-20241022",
		TotalTokens: 1000,
	}
	anthropicCost := tracker.estimateCost(anthropicRecord)
	if anthropicCost <= 0 {
		t.Errorf("Anthropic cost should be > 0, got %f", anthropicCost)
	}
}

func TestUsageRecord_Fields(t *testing.T) {
	record := UsageRecord{
		ID:           uuid.New(),
		Timestamp:    time.Now(),
		Provider:     ProviderAnthropic,
		Model:        "claude-3-5-sonnet-20241022",
		Tier:         Tier3,
		InputTokens:  100,
		OutputTokens: 200,
		TotalTokens:  300,
		Cost:         0.05,
		Duration:     1500.0,
	}

	if record.TotalTokens != 300 {
		t.Error("TotalTokens mismatch")
	}
	if record.Tier != Tier3 {
		t.Error("Tier mismatch")
	}
}

func TestBudgetConfig_Fields(t *testing.T) {
	cfg := BudgetConfig{
		HourlyTokenLimit:  50000,
		DailyTokenLimit:   500000,
		MonthlyBudgetUSD:  100.0,
		RequestsPerMinute: 60,
	}

	if cfg.HourlyTokenLimit != 50000 {
		t.Error("HourlyTokenLimit mismatch")
	}
	if cfg.MonthlyBudgetUSD != 100.0 {
		t.Error("MonthlyBudgetUSD mismatch")
	}
}

func TestUsageStats_Fields(t *testing.T) {
	stats := UsageStats{
		TotalRequests:   100,
		TotalTokens:     50000,
		EstimatedCost:   5.50,
		AvgTokensPerReq: 500.0,
		Period:          "day",
	}

	if stats.TotalRequests != 100 {
		t.Error("TotalRequests mismatch")
	}
	if stats.Period != "day" {
		t.Error("Period mismatch")
	}
}

func TestBudgetStatus_Percentages(t *testing.T) {
	status := BudgetStatus{
		HourlyTokensUsed:   5000,
		HourlyTokensLimit:  10000,
		HourlyPercentUsed:  50.0,
		DailyTokensUsed:    25000,
		DailyTokensLimit:   100000,
		DailyPercentUsed:   25.0,
		MonthlySpentUSD:    25.0,
		MonthlyBudgetUSD:   100.0,
		MonthlyPercentUsed: 25.0,
	}

	if status.HourlyPercentUsed != 50.0 {
		t.Error("HourlyPercentUsed mismatch")
	}
	if status.MonthlyPercentUsed != 25.0 {
		t.Error("MonthlyPercentUsed mismatch")
	}
}

func TestUsageTracker_ExportJSON(t *testing.T) {
	tracker := NewUsageTracker(UsageTrackerConfig{MaxRecords: 10})

	tracker.Record(UsageRecord{
		Provider:     ProviderOllama,
		Model:        "test-model",
		InputTokens:  100,
		OutputTokens: 50,
	})

	data, err := tracker.ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected non-empty JSON")
	}
}

func TestTrackedRouter_Creation(t *testing.T) {
	// This would need a mock router in practice
	tracker := NewUsageTracker(UsageTrackerConfig{})
	trackedRouter := NewTrackedRouter(nil, tracker)

	if trackedRouter.tracker != tracker {
		t.Error("Tracker not set correctly")
	}
}

func TestTrackedRouter_GetTracker(t *testing.T) {
	tracker := NewUsageTracker(UsageTrackerConfig{})
	trackedRouter := NewTrackedRouter(nil, tracker)

	retrieved := trackedRouter.GetTracker()
	if retrieved != tracker {
		t.Error("GetTracker returned wrong tracker")
	}
}

func TestDefaultCostPer1K(t *testing.T) {
	costs := defaultCostPer1K()

	// Ollama should be free
	if costs[ProviderOllama]["default"] != 0 {
		t.Error("Ollama should be free")
	}

	// Anthropic should have costs
	if costs[ProviderAnthropic]["claude-3-5-sonnet-20241022"] <= 0 {
		t.Error("Anthropic should have costs")
	}

	// OpenAI should have costs
	if costs[ProviderOpenAI]["gpt-4"] <= 0 {
		t.Error("OpenAI should have costs")
	}
}
