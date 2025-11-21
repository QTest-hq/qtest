package model

import (
	"fmt"
	"sort"
)

// PlannerConfig configures the test planner
type PlannerConfig struct {
	// Risk thresholds for prioritization
	HighRiskThreshold   float64 // Above this = high priority (default: 0.7)
	MediumRiskThreshold float64 // Above this = medium priority (default: 0.4)

	// Test distribution targets (percentage)
	UnitTestRatio float64 // Target ratio of unit tests (default: 0.7)
	APITestRatio  float64 // Target ratio of API tests (default: 0.2)
	E2ETestRatio  float64 // Target ratio of E2E tests (default: 0.1)

	// Limits
	MaxIntents int // Maximum intents to generate (0 = unlimited)
}

// DefaultPlannerConfig returns default planner configuration
func DefaultPlannerConfig() PlannerConfig {
	return PlannerConfig{
		HighRiskThreshold:   0.7,
		MediumRiskThreshold: 0.4,
		UnitTestRatio:       0.7,
		APITestRatio:        0.2,
		E2ETestRatio:        0.1,
		MaxIntents:          0,
	}
}

// Planner generates test intents from a system model
type Planner struct {
	config PlannerConfig
}

// NewPlanner creates a new test planner
func NewPlanner(config PlannerConfig) *Planner {
	return &Planner{config: config}
}

// Plan generates a test plan from a system model
func (p *Planner) Plan(model *SystemModel) (*TestPlan, error) {
	plan := &TestPlan{
		ModelID:    model.ID,
		Repository: model.Repository,
		Intents:    make([]TestIntent, 0),
	}

	// 1. Generate API test intents for all endpoints (highest priority)
	for _, ep := range model.Endpoints {
		intent := TestIntent{
			ID:         fmt.Sprintf("intent:api:%s", ep.ID),
			Level:      LevelAPI,
			TargetKind: "endpoint",
			TargetID:   ep.ID,
			Priority:   "high", // API endpoints are always high priority
			Reason:     fmt.Sprintf("API endpoint: %s %s", ep.Method, ep.Path),
		}
		plan.Intents = append(plan.Intents, intent)
		plan.APITests++
	}

	// 2. Generate unit test intents for exported functions
	type scoredFunction struct {
		fn    Function
		score float64
	}

	var scoredFuncs []scoredFunction
	for _, fn := range model.Functions {
		if !fn.Exported {
			continue
		}
		score := 0.0
		if rs, ok := model.RiskScores[fn.ID]; ok {
			score = rs.Score
		}
		scoredFuncs = append(scoredFuncs, scoredFunction{fn: fn, score: score})
	}

	// Sort by risk score (highest first)
	sort.Slice(scoredFuncs, func(i, j int) bool {
		return scoredFuncs[i].score > scoredFuncs[j].score
	})

	// Generate intents for functions
	for _, sf := range scoredFuncs {
		priority := "low"
		if sf.score >= p.config.HighRiskThreshold {
			priority = "high"
		} else if sf.score >= p.config.MediumRiskThreshold {
			priority = "medium"
		}

		// Skip functions that are likely endpoint handlers (already covered by API tests)
		isHandler := false
		for _, ep := range model.Endpoints {
			if ep.Handler == sf.fn.Name {
				isHandler = true
				break
			}
		}
		if isHandler {
			continue
		}

		reason := fmt.Sprintf("Exported function (risk: %.2f)", sf.score)
		if sf.fn.Class != "" {
			reason = fmt.Sprintf("Method %s.%s (risk: %.2f)", sf.fn.Class, sf.fn.Name, sf.score)
		}

		intent := TestIntent{
			ID:         fmt.Sprintf("intent:unit:%s", sf.fn.ID),
			Level:      LevelUnit,
			TargetKind: "function",
			TargetID:   sf.fn.ID,
			Priority:   priority,
			Reason:     reason,
		}
		plan.Intents = append(plan.Intents, intent)
		plan.UnitTests++
	}

	// Apply max intents limit if set
	if p.config.MaxIntents > 0 && len(plan.Intents) > p.config.MaxIntents {
		plan.Intents = plan.Intents[:p.config.MaxIntents]
		// Recount
		plan.UnitTests = 0
		plan.APITests = 0
		plan.E2ETests = 0
		for _, i := range plan.Intents {
			switch i.Level {
			case LevelUnit:
				plan.UnitTests++
			case LevelAPI:
				plan.APITests++
			case LevelE2E:
				plan.E2ETests++
			}
		}
	}

	plan.TotalTests = len(plan.Intents)

	return plan, nil
}

// PlanWithPyramid generates a test plan following the test pyramid distribution
func (p *Planner) PlanWithPyramid(model *SystemModel, targetTotal int) (*TestPlan, error) {
	// Calculate target counts for each level
	targetAPI := int(float64(targetTotal) * p.config.APITestRatio)
	targetE2E := int(float64(targetTotal) * p.config.E2ETestRatio)
	targetUnit := targetTotal - targetAPI - targetE2E

	plan := &TestPlan{
		ModelID:    model.ID,
		Repository: model.Repository,
		Intents:    make([]TestIntent, 0),
	}

	// Add API tests (up to target)
	apiCount := 0
	for _, ep := range model.Endpoints {
		if apiCount >= targetAPI {
			break
		}
		intent := TestIntent{
			ID:         fmt.Sprintf("intent:api:%s", ep.ID),
			Level:      LevelAPI,
			TargetKind: "endpoint",
			TargetID:   ep.ID,
			Priority:   "high",
			Reason:     fmt.Sprintf("API endpoint: %s %s", ep.Method, ep.Path),
		}
		plan.Intents = append(plan.Intents, intent)
		apiCount++
	}
	plan.APITests = apiCount

	// Add unit tests (up to target)
	unitCount := 0
	for _, fn := range model.Functions {
		if unitCount >= targetUnit {
			break
		}
		if !fn.Exported {
			continue
		}

		// Skip handlers
		isHandler := false
		for _, ep := range model.Endpoints {
			if ep.Handler == fn.Name {
				isHandler = true
				break
			}
		}
		if isHandler {
			continue
		}

		score := 0.0
		if rs, ok := model.RiskScores[fn.ID]; ok {
			score = rs.Score
		}

		priority := "low"
		if score >= p.config.HighRiskThreshold {
			priority = "high"
		} else if score >= p.config.MediumRiskThreshold {
			priority = "medium"
		}

		intent := TestIntent{
			ID:         fmt.Sprintf("intent:unit:%s", fn.ID),
			Level:      LevelUnit,
			TargetKind: "function",
			TargetID:   fn.ID,
			Priority:   priority,
			Reason:     fmt.Sprintf("Exported function (risk: %.2f)", score),
		}
		plan.Intents = append(plan.Intents, intent)
		unitCount++
	}
	plan.UnitTests = unitCount

	// E2E tests would be added here when we have flow detection
	plan.E2ETests = 0

	plan.TotalTests = len(plan.Intents)

	return plan, nil
}
