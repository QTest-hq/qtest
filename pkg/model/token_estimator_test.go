package model

import (
	"testing"
)

func TestEstimateCodeTokens(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected int
	}{
		{
			name:     "empty code",
			code:     "",
			expected: 0,
		},
		{
			name:     "short function",
			code:     "func Add(a, b int) int { return a + b }",
			expected: 9, // 39 chars / 4 = 9
		},
		{
			name:     "medium function",
			code:     "func Calculate(x, y, z int) int {\n\tresult := x + y\n\tresult *= z\n\treturn result\n}",
			expected: 20, // ~80 chars / 4 = 20
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateCodeTokens(tt.code)
			if got != tt.expected {
				t.Errorf("estimateCodeTokens() = %d, want %d (code len: %d)", got, tt.expected, len(tt.code))
			}
		})
	}
}

func TestEstimateOutputTokens(t *testing.T) {
	tests := []struct {
		name       string
		codeTokens int
		level      TestLevel
		complexity int
		wantMin    int
		wantMax    int
	}{
		{
			name:       "small unit test",
			codeTokens: 50,
			level:      LevelUnit,
			complexity: 2,
			wantMin:    MinOutputTokens, // 125 < min, so should be min
			wantMax:    MinOutputTokens,
		},
		{
			name:       "medium unit test",
			codeTokens: 200,
			level:      LevelUnit,
			complexity: 5,
			wantMin:    400, // 200 * 2.5 = 500
			wantMax:    600,
		},
		{
			name:       "complex unit test",
			codeTokens: 300,
			level:      LevelUnit,
			complexity: 15, // high complexity
			wantMin:    750,
			wantMax:    1000, // 300 * 2.5 * 1.2 = 900
		},
		{
			name:       "api test higher multiplier",
			codeTokens: 200,
			level:      LevelAPI,
			complexity: 5,
			wantMin:    500, // 200 * 3.0 = 600
			wantMax:    700,
		},
		{
			name:       "e2e test highest multiplier",
			codeTokens: 200,
			level:      LevelE2E,
			complexity: 5,
			wantMin:    700, // 200 * 4.0 = 800
			wantMax:    900,
		},
		{
			name:       "capped at max",
			codeTokens: 2000,
			level:      LevelE2E,
			complexity: 20,
			wantMin:    MaxOutputTokens,
			wantMax:    MaxOutputTokens,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateOutputTokens(tt.codeTokens, tt.level, tt.complexity)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("estimateOutputTokens(%d, %s, %d) = %d, want between %d and %d",
					tt.codeTokens, tt.level, tt.complexity, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestEstimateTokens(t *testing.T) {
	fn := &Function{
		Name:       "Add",
		Body:       "func Add(a, b int) int {\n\treturn a + b\n}",
		LOC:        3,
		Complexity: 1,
	}

	estimate := EstimateTokens(fn, LevelUnit, 0)

	// Verify structure
	if estimate.InputTokens <= 0 {
		t.Error("InputTokens should be positive")
	}
	if estimate.OutputTokens <= 0 {
		t.Error("OutputTokens should be positive")
	}
	if estimate.TotalTokens != estimate.InputTokens+estimate.OutputTokens {
		t.Errorf("TotalTokens = %d, want %d", estimate.TotalTokens, estimate.InputTokens+estimate.OutputTokens)
	}

	// Verify components
	if estimate.Components.SystemPrompt != SystemPromptTokens {
		t.Errorf("Components.SystemPrompt = %d, want %d", estimate.Components.SystemPrompt, SystemPromptTokens)
	}
	if estimate.Components.UserPrompt != UserPromptBaseTokens {
		t.Errorf("Components.UserPrompt = %d, want %d", estimate.Components.UserPrompt, UserPromptBaseTokens)
	}
	if estimate.Components.FunctionCode <= 0 {
		t.Error("Components.FunctionCode should be positive")
	}
}

func TestEstimateTokens_LOCFallback(t *testing.T) {
	// Function with LOC but no Body
	fn := &Function{
		Name:       "Process",
		Body:       "", // No body
		LOC:        50,
		Complexity: 5,
	}

	estimate := EstimateTokens(fn, LevelUnit, 0)

	// Should use LOC fallback: 50 * 40 / 4 = 500 tokens
	expectedCodeTokens := (50 * 40) / CharsPerToken
	if estimate.Components.FunctionCode != expectedCodeTokens {
		t.Errorf("Components.FunctionCode = %d, want %d (LOC fallback)", estimate.Components.FunctionCode, expectedCodeTokens)
	}
}

func TestEstimateTokens_WithContext(t *testing.T) {
	fn := &Function{
		Name:       "Add",
		Body:       "func Add(a, b int) int { return a + b }",
		LOC:        1,
		Complexity: 1,
	}

	// Without context
	noContext := EstimateTokens(fn, LevelUnit, 0)

	// With context
	withContext := EstimateTokens(fn, LevelUnit, 400) // 400 chars of context

	// With context should have more input tokens
	if withContext.InputTokens <= noContext.InputTokens {
		t.Errorf("InputTokens with context (%d) should be > without context (%d)",
			withContext.InputTokens, noContext.InputTokens)
	}

	// Difference should be approximately contextSize/CharsPerToken
	expectedDiff := 400 / CharsPerToken
	actualDiff := withContext.InputTokens - noContext.InputTokens
	if actualDiff != expectedDiff {
		t.Errorf("Context token difference = %d, want %d", actualDiff, expectedDiff)
	}
}

func TestEstimateTokensForIntent_Function(t *testing.T) {
	model := &SystemModel{
		Functions: []Function{
			{
				ID:         "pkg:file.go:1:Add",
				Name:       "Add",
				Body:       "func Add(a, b int) int { return a + b }",
				LOC:        1,
				Complexity: 1,
			},
		},
	}

	intent := &TestIntent{
		ID:         "intent:unit:pkg:file.go:1:Add",
		Level:      LevelUnit,
		TargetKind: "function",
		TargetID:   "pkg:file.go:1:Add",
	}

	estimate := EstimateTokensForIntent(intent, model)

	if estimate.InputTokens <= 0 {
		t.Error("InputTokens should be positive")
	}
	if estimate.OutputTokens <= 0 {
		t.Error("OutputTokens should be positive")
	}
}

func TestEstimateTokensForIntent_Endpoint(t *testing.T) {
	model := &SystemModel{
		Endpoints: []Endpoint{
			{
				ID:      "ep:get:/users",
				Method:  "GET",
				Path:    "/users",
				Handler: "pkg:file.go:10:GetUsers",
			},
		},
		Functions: []Function{
			{
				ID:         "pkg:file.go:10:GetUsers",
				Name:       "GetUsers",
				Body:       "func GetUsers(c *gin.Context) { c.JSON(200, users) }",
				LOC:        5,
				Complexity: 3,
			},
		},
	}

	intent := &TestIntent{
		ID:         "intent:api:ep:get:/users",
		Level:      LevelAPI,
		TargetKind: "endpoint",
		TargetID:   "ep:get:/users",
	}

	estimate := EstimateTokensForIntent(intent, model)

	if estimate.InputTokens <= 0 {
		t.Error("InputTokens should be positive")
	}
	// API tests should have higher output multiplier
	if estimate.OutputTokens < MinOutputTokens {
		t.Errorf("OutputTokens = %d, should be at least %d", estimate.OutputTokens, MinOutputTokens)
	}
}

func TestEstimateTokensForIntent_UnknownTarget(t *testing.T) {
	model := &SystemModel{}

	intent := &TestIntent{
		ID:         "intent:unit:unknown",
		Level:      LevelUnit,
		TargetKind: "function",
		TargetID:   "nonexistent",
	}

	estimate := EstimateTokensForIntent(intent, model)

	// Should return default estimate
	if estimate.InputTokens <= 0 {
		t.Error("InputTokens should be positive even for unknown target")
	}
	if estimate.OutputTokens != MinOutputTokens {
		t.Errorf("OutputTokens = %d, want %d for unknown target", estimate.OutputTokens, MinOutputTokens)
	}
}

func TestEstimatePlanTokens(t *testing.T) {
	model := &SystemModel{
		Functions: []Function{
			{
				ID:         "pkg:file.go:1:Add",
				Name:       "Add",
				Body:       "func Add(a, b int) int { return a + b }",
				LOC:        1,
				Complexity: 1,
			},
			{
				ID:         "pkg:file.go:5:Sub",
				Name:       "Sub",
				Body:       "func Sub(a, b int) int { return a - b }",
				LOC:        1,
				Complexity: 1,
			},
		},
	}

	plan := &TestPlan{
		Intents: []TestIntent{
			{
				ID:         "intent:unit:Add",
				Level:      LevelUnit,
				TargetKind: "function",
				TargetID:   "pkg:file.go:1:Add",
			},
			{
				ID:         "intent:unit:Sub",
				Level:      LevelUnit,
				TargetKind: "function",
				TargetID:   "pkg:file.go:5:Sub",
			},
		},
	}

	planEstimate := EstimatePlanTokens(plan, model)

	// Should have totals
	if planEstimate.TotalInputTokens <= 0 {
		t.Error("TotalInputTokens should be positive")
	}
	if planEstimate.TotalOutputTokens <= 0 {
		t.Error("TotalOutputTokens should be positive")
	}
	if planEstimate.TotalTokens != planEstimate.TotalInputTokens+planEstimate.TotalOutputTokens {
		t.Error("TotalTokens should equal sum of input and output")
	}

	// Should have individual estimates
	if len(planEstimate.IntentEstimates) != 2 {
		t.Errorf("IntentEstimates length = %d, want 2", len(planEstimate.IntentEstimates))
	}

	// Verify aggregation is correct
	var sumInput, sumOutput int
	for _, ie := range planEstimate.IntentEstimates {
		sumInput += ie.Estimate.InputTokens
		sumOutput += ie.Estimate.OutputTokens
	}
	if sumInput != planEstimate.TotalInputTokens {
		t.Errorf("Sum of input tokens (%d) != TotalInputTokens (%d)", sumInput, planEstimate.TotalInputTokens)
	}
	if sumOutput != planEstimate.TotalOutputTokens {
		t.Errorf("Sum of output tokens (%d) != TotalOutputTokens (%d)", sumOutput, planEstimate.TotalOutputTokens)
	}
}

func TestEstimateCost(t *testing.T) {
	estimate := TokenEstimate{
		InputTokens:  1000,
		OutputTokens: 500,
		TotalTokens:  1500,
	}

	// Test with GPT-4o pricing ($5/1M input, $15/1M output)
	cost := EstimateCost(estimate, GPT4oInputPrice, GPT4oOutputPrice)

	expectedInputCost := float64(1000) / 1_000_000 * GPT4oInputPrice   // $0.005
	expectedOutputCost := float64(500) / 1_000_000 * GPT4oOutputPrice  // $0.0075
	expectedTotal := expectedInputCost + expectedOutputCost

	if cost.InputCost != expectedInputCost {
		t.Errorf("InputCost = %f, want %f", cost.InputCost, expectedInputCost)
	}
	if cost.OutputCost != expectedOutputCost {
		t.Errorf("OutputCost = %f, want %f", cost.OutputCost, expectedOutputCost)
	}
	if cost.TotalCost != expectedTotal {
		t.Errorf("TotalCost = %f, want %f", cost.TotalCost, expectedTotal)
	}
	if cost.Currency != "USD" {
		t.Errorf("Currency = %s, want USD", cost.Currency)
	}
}

func TestEstimateCost_Ollama(t *testing.T) {
	estimate := TokenEstimate{
		InputTokens:  10000,
		OutputTokens: 5000,
		TotalTokens:  15000,
	}

	// Ollama is free
	cost := EstimateCost(estimate, OllamaInputPrice, OllamaOutputPrice)

	if cost.TotalCost != 0 {
		t.Errorf("Ollama cost should be 0, got %f", cost.TotalCost)
	}
}

func TestEstimatePlanCost(t *testing.T) {
	planEstimate := PlanTokenEstimate{
		TotalInputTokens:  100000,
		TotalOutputTokens: 50000,
		TotalTokens:       150000,
	}

	cost := EstimatePlanCost(planEstimate, GPT4oMiniInputPrice, GPT4oMiniOutputPrice)

	// 100K input * $0.15/1M = $0.015
	// 50K output * $0.60/1M = $0.03
	expectedTotal := 0.015 + 0.03
	tolerance := 0.0001

	if cost.TotalCost < expectedTotal-tolerance || cost.TotalCost > expectedTotal+tolerance {
		t.Errorf("TotalCost = %f, want approximately %f", cost.TotalCost, expectedTotal)
	}
}

func TestTestPlan_PopulateTokenEstimates(t *testing.T) {
	model := &SystemModel{
		Functions: []Function{
			{
				ID:         "pkg:file.go:1:Add",
				Name:       "Add",
				Body:       "func Add(a, b int) int { return a + b }",
				LOC:        1,
				Complexity: 1,
			},
		},
	}

	plan := &TestPlan{
		Intents: []TestIntent{
			{
				ID:         "intent:unit:Add",
				Level:      LevelUnit,
				TargetKind: "function",
				TargetID:   "pkg:file.go:1:Add",
			},
		},
	}

	// Initially no estimates
	if plan.TotalEstimatedInputTokens != 0 {
		t.Error("TotalEstimatedInputTokens should be 0 initially")
	}

	plan.PopulateTokenEstimates(model)

	// After population
	if plan.TotalEstimatedInputTokens <= 0 {
		t.Error("TotalEstimatedInputTokens should be positive after PopulateTokenEstimates")
	}
	if plan.TotalEstimatedOutputTokens <= 0 {
		t.Error("TotalEstimatedOutputTokens should be positive after PopulateTokenEstimates")
	}
	if plan.Intents[0].EstimatedInputTokens <= 0 {
		t.Error("Intent.EstimatedInputTokens should be populated")
	}
	if plan.Intents[0].EstimatedOutputTokens <= 0 {
		t.Error("Intent.EstimatedOutputTokens should be populated")
	}

	// TotalEstimatedTokens method
	total := plan.TotalEstimatedTokens()
	if total != plan.TotalEstimatedInputTokens+plan.TotalEstimatedOutputTokens {
		t.Errorf("TotalEstimatedTokens() = %d, want %d", total, plan.TotalEstimatedInputTokens+plan.TotalEstimatedOutputTokens)
	}
}
