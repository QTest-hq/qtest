package model

import (
	"testing"
)

// =============================================================================
// TestIntent Tests
// =============================================================================

func TestTestIntent_Fields(t *testing.T) {
	intent := TestIntent{
		ID:         "intent-001",
		Level:      LevelUnit,
		TargetKind: "function",
		TargetID:   "fn:GetUser",
		Priority:   "high",
		Reason:     "High complexity function without tests",
	}

	if intent.ID != "intent-001" {
		t.Errorf("ID = %s, want intent-001", intent.ID)
	}
	if intent.Level != LevelUnit {
		t.Errorf("Level = %s, want unit", intent.Level)
	}
	if intent.TargetKind != "function" {
		t.Errorf("TargetKind = %s, want function", intent.TargetKind)
	}
	if intent.Priority != "high" {
		t.Errorf("Priority = %s, want high", intent.Priority)
	}
}

func TestTestIntent_Levels(t *testing.T) {
	tests := []struct {
		name  string
		level TestLevel
	}{
		{"unit", LevelUnit},
		{"api", LevelAPI},
		{"e2e", LevelE2E},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intent := TestIntent{Level: tt.level}
			if intent.Level != tt.level {
				t.Errorf("Level = %s, want %s", intent.Level, tt.level)
			}
		})
	}
}

// =============================================================================
// TestPlan Tests
// =============================================================================

func TestTestPlan_Stats(t *testing.T) {
	plan := &TestPlan{
		ModelID:    "model-123",
		Repository: "test/repo",
		TotalTests: 10,
		UnitTests:  6,
		APITests:   3,
		E2ETests:   1,
		Intents: []TestIntent{
			{ID: "1", Priority: "high"},
			{ID: "2", Priority: "high"},
			{ID: "3", Priority: "medium"},
			{ID: "4", Priority: "medium"},
			{ID: "5", Priority: "medium"},
			{ID: "6", Priority: "low"},
		},
	}

	stats := plan.Stats()

	tests := []struct {
		key  string
		want int
	}{
		{"total", 10},
		{"unit", 6},
		{"api", 3},
		{"e2e", 1},
		{"high", 2},
		{"medium", 3},
		{"low", 1},
	}

	for _, tt := range tests {
		got := stats[tt.key]
		if got != tt.want {
			t.Errorf("Stats()[%s] = %d, want %d", tt.key, got, tt.want)
		}
	}
}

func TestTestPlan_CountByPriority(t *testing.T) {
	plan := &TestPlan{
		Intents: []TestIntent{
			{Priority: "high"},
			{Priority: "high"},
			{Priority: "medium"},
			{Priority: "low"},
			{Priority: "low"},
			{Priority: "low"},
		},
	}

	// Access the private method through Stats
	stats := plan.Stats()

	if stats["high"] != 2 {
		t.Errorf("count(high) = %d, want 2", stats["high"])
	}
	if stats["medium"] != 1 {
		t.Errorf("count(medium) = %d, want 1", stats["medium"])
	}
	if stats["low"] != 3 {
		t.Errorf("count(low) = %d, want 3", stats["low"])
	}
}

func TestTestPlan_Empty(t *testing.T) {
	plan := &TestPlan{}

	stats := plan.Stats()

	if stats["total"] != 0 {
		t.Errorf("empty plan Stats()[total] = %d, want 0", stats["total"])
	}
	if stats["high"] != 0 {
		t.Errorf("empty plan Stats()[high] = %d, want 0", stats["high"])
	}
}

func TestTestPlan_Fields(t *testing.T) {
	plan := &TestPlan{
		ModelID:    "model-abc",
		Repository: "github.com/test/repo",
		TotalTests: 100,
		UnitTests:  70,
		APITests:   25,
		E2ETests:   5,
	}

	if plan.ModelID != "model-abc" {
		t.Errorf("ModelID = %s, want model-abc", plan.ModelID)
	}
	if plan.Repository != "github.com/test/repo" {
		t.Errorf("Repository = %s, want github.com/test/repo", plan.Repository)
	}
	if plan.TotalTests != 100 {
		t.Errorf("TotalTests = %d, want 100", plan.TotalTests)
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestTestIntent_ToTestSpec(t *testing.T) {
	// This tests the conceptual flow from intent to spec
	intent := TestIntent{
		ID:         "intent-001",
		Level:      LevelUnit,
		TargetKind: "function",
		TargetID:   "fn:GetUser",
		Priority:   "high",
		Reason:     "Critical function",
	}

	// In real code, intents would be converted to specs
	// This just validates the data consistency
	if intent.TargetKind != "function" && intent.TargetKind != "endpoint" {
		t.Errorf("TargetKind should be 'function' or 'endpoint', got %s", intent.TargetKind)
	}

	validPriorities := map[string]bool{"high": true, "medium": true, "low": true}
	if !validPriorities[intent.Priority] {
		t.Errorf("Priority should be high/medium/low, got %s", intent.Priority)
	}
}
