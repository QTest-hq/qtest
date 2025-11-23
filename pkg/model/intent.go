package model

// TestLevel represents the test pyramid level
type TestLevel string

const (
	LevelUnit TestLevel = "unit"
	LevelAPI  TestLevel = "api"
	LevelE2E  TestLevel = "e2e"
)

// TestIntent represents "what to test at which level"
// This is the output of planning, before LLM generation
type TestIntent struct {
	ID         string    `json:"id"`
	Level      TestLevel `json:"level"`       // unit/api/e2e
	TargetKind string    `json:"target_kind"` // "function" | "endpoint"
	TargetID   string    `json:"target_id"`   // refers into SystemModel
	Priority   string    `json:"priority"`    // "high" | "medium" | "low"
	Reason     string    `json:"reason"`      // why this test is needed

	// Token estimates for this intent (populated by EstimateTokensForIntent)
	EstimatedInputTokens  int `json:"estimated_input_tokens,omitempty"`
	EstimatedOutputTokens int `json:"estimated_output_tokens,omitempty"`
}

// TestPlan is a collection of test intents with metadata
type TestPlan struct {
	ModelID    string       `json:"model_id"` // ID of the SystemModel this plan is for
	Repository string       `json:"repository"`
	TotalTests int          `json:"total_tests"`
	UnitTests  int          `json:"unit_tests"`
	APITests   int          `json:"api_tests"`
	E2ETests   int          `json:"e2e_tests"`
	Intents    []TestIntent `json:"intents"`

	// Aggregated token estimates (populated by EstimatePlanTokens)
	TotalEstimatedInputTokens  int `json:"total_estimated_input_tokens,omitempty"`
	TotalEstimatedOutputTokens int `json:"total_estimated_output_tokens,omitempty"`
}

// Stats returns test plan statistics
func (p *TestPlan) Stats() map[string]int {
	return map[string]int{
		"total":  p.TotalTests,
		"unit":   p.UnitTests,
		"api":    p.APITests,
		"e2e":    p.E2ETests,
		"high":   p.countByPriority("high"),
		"medium": p.countByPriority("medium"),
		"low":    p.countByPriority("low"),
	}
}

func (p *TestPlan) countByPriority(priority string) int {
	count := 0
	for _, i := range p.Intents {
		if i.Priority == priority {
			count++
		}
	}
	return count
}

// PopulateTokenEstimates calculates and fills in token estimates for all intents
// and aggregates them into the plan totals
func (p *TestPlan) PopulateTokenEstimates(model *SystemModel) {
	p.TotalEstimatedInputTokens = 0
	p.TotalEstimatedOutputTokens = 0

	for i := range p.Intents {
		estimate := EstimateTokensForIntent(&p.Intents[i], model)
		p.Intents[i].EstimatedInputTokens = estimate.InputTokens
		p.Intents[i].EstimatedOutputTokens = estimate.OutputTokens
		p.TotalEstimatedInputTokens += estimate.InputTokens
		p.TotalEstimatedOutputTokens += estimate.OutputTokens
	}
}

// TotalEstimatedTokens returns the sum of input and output token estimates
func (p *TestPlan) TotalEstimatedTokens() int {
	return p.TotalEstimatedInputTokens + p.TotalEstimatedOutputTokens
}
