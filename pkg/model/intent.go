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
