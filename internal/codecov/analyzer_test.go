package codecov

import (
	"testing"

	"github.com/QTest-hq/qtest/pkg/model"
)

func TestNewAnalyzer(t *testing.T) {
	report := &CoverageReport{
		Language:   "go",
		Percentage: 80.0,
	}

	analyzer := NewAnalyzer(report, nil)

	if analyzer == nil {
		t.Fatal("NewAnalyzer() returned nil")
	}
	if analyzer.report != report {
		t.Error("report reference mismatch")
	}
	if analyzer.sysModel != nil {
		t.Error("sysModel should be nil")
	}
}

func TestNewAnalyzer_WithModel(t *testing.T) {
	report := &CoverageReport{Language: "go"}
	sysModel := &model.SystemModel{
		Functions: []model.Function{
			{ID: "fn1", Name: "TestFunc"},
		},
	}

	analyzer := NewAnalyzer(report, sysModel)

	if analyzer.sysModel != sysModel {
		t.Error("sysModel reference mismatch")
	}
}

func TestCoverageGap_Fields(t *testing.T) {
	gap := CoverageGap{
		File:       "main.go",
		StartLine:  10,
		EndLine:    20,
		Type:       "function",
		Name:       "TestFunc",
		Priority:   "high",
		Reason:     "Low coverage",
		TargetID:   "fn1",
		Complexity: 5,
	}

	if gap.File != "main.go" {
		t.Errorf("File = %s, want main.go", gap.File)
	}
	if gap.StartLine != 10 {
		t.Errorf("StartLine = %d, want 10", gap.StartLine)
	}
	if gap.EndLine != 20 {
		t.Errorf("EndLine = %d, want 20", gap.EndLine)
	}
	if gap.Type != "function" {
		t.Errorf("Type = %s, want function", gap.Type)
	}
	if gap.Name != "TestFunc" {
		t.Errorf("Name = %s, want TestFunc", gap.Name)
	}
	if gap.Priority != "high" {
		t.Errorf("Priority = %s, want high", gap.Priority)
	}
	if gap.Reason != "Low coverage" {
		t.Errorf("Reason = %s, want Low coverage", gap.Reason)
	}
	if gap.TargetID != "fn1" {
		t.Errorf("TargetID = %s, want fn1", gap.TargetID)
	}
	if gap.Complexity != 5 {
		t.Errorf("Complexity = %d, want 5", gap.Complexity)
	}
}

func TestAnalysisResult_Fields(t *testing.T) {
	result := AnalysisResult{
		TotalCoverage:   75.0,
		TargetCoverage:  80.0,
		Gaps:            []CoverageGap{{File: "main.go"}},
		CriticalGaps:    2,
		SuggestedTests:  5,
		EstimatedEffort: "medium",
	}

	if result.TotalCoverage != 75.0 {
		t.Errorf("TotalCoverage = %f, want 75.0", result.TotalCoverage)
	}
	if result.TargetCoverage != 80.0 {
		t.Errorf("TargetCoverage = %f, want 80.0", result.TargetCoverage)
	}
	if len(result.Gaps) != 1 {
		t.Errorf("len(Gaps) = %d, want 1", len(result.Gaps))
	}
	if result.CriticalGaps != 2 {
		t.Errorf("CriticalGaps = %d, want 2", result.CriticalGaps)
	}
	if result.SuggestedTests != 5 {
		t.Errorf("SuggestedTests = %d, want 5", result.SuggestedTests)
	}
	if result.EstimatedEffort != "medium" {
		t.Errorf("EstimatedEffort = %s, want medium", result.EstimatedEffort)
	}
}

func TestAnalyze_Empty(t *testing.T) {
	report := &CoverageReport{
		Percentage: 100.0,
		Files:      []FileCoverage{},
		Uncovered:  []UncoveredItem{},
	}

	analyzer := NewAnalyzer(report, nil)
	result := analyzer.Analyze(80.0)

	if result.TotalCoverage != 100.0 {
		t.Errorf("TotalCoverage = %f, want 100.0", result.TotalCoverage)
	}
	if result.TargetCoverage != 80.0 {
		t.Errorf("TargetCoverage = %f, want 80.0", result.TargetCoverage)
	}
	if len(result.Gaps) != 0 {
		t.Errorf("len(Gaps) = %d, want 0", len(result.Gaps))
	}
}

func TestAnalyze_WithUncoveredLines(t *testing.T) {
	report := &CoverageReport{
		Percentage: 70.0,
		Files: []FileCoverage{
			{
				Path:           "main.go",
				Percentage:     70.0,
				UncoveredLines: []int{10, 11, 12, 13, 14, 15}, // 6 consecutive lines
			},
		},
		Uncovered: []UncoveredItem{},
	}

	analyzer := NewAnalyzer(report, nil)
	result := analyzer.Analyze(80.0)

	if result.TotalCoverage != 70.0 {
		t.Errorf("TotalCoverage = %f, want 70.0", result.TotalCoverage)
	}

	// Should find a block of uncovered lines
	hasBlock := false
	for _, gap := range result.Gaps {
		if gap.Type == "block" {
			hasBlock = true
			break
		}
	}
	if !hasBlock {
		t.Error("Should find a block of uncovered lines")
	}
}

func TestAnalyze_WithFunctions(t *testing.T) {
	report := &CoverageReport{
		Percentage: 60.0,
		Files: []FileCoverage{
			{
				Path:           "main.go",
				Percentage:     60.0,
				UncoveredLines: []int{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
			},
		},
	}

	sysModel := &model.SystemModel{
		Functions: []model.Function{
			{
				ID:         "fn1",
				Name:       "TestFunc",
				File:       "main.go",
				StartLine:  10,
				EndLine:    20,
				Complexity: 5,
			},
		},
	}

	analyzer := NewAnalyzer(report, sysModel)
	result := analyzer.Analyze(80.0)

	// Should find the uncovered function
	hasFunctionGap := false
	for _, gap := range result.Gaps {
		if gap.Type == "function" && gap.Name == "TestFunc" {
			hasFunctionGap = true
			break
		}
	}
	if !hasFunctionGap {
		t.Error("Should find uncovered function gap")
	}
}

func TestAnalyze_WithEndpoints(t *testing.T) {
	report := &CoverageReport{
		Percentage: 60.0,
		Files: []FileCoverage{
			{
				Path:           "api.go",
				Percentage:     60.0,
				UncoveredLines: []int{50},
			},
		},
	}

	sysModel := &model.SystemModel{
		Endpoints: []model.Endpoint{
			{
				ID:     "ep1",
				Method: "GET",
				Path:   "/users",
				File:   "api.go",
				Line:   50,
			},
		},
	}

	analyzer := NewAnalyzer(report, sysModel)
	result := analyzer.Analyze(80.0)

	// Should find the uncovered endpoint
	hasEndpointGap := false
	for _, gap := range result.Gaps {
		if gap.Type == "endpoint" {
			hasEndpointGap = true
			if gap.Priority != "critical" {
				t.Errorf("Endpoint gap priority = %s, want critical", gap.Priority)
			}
			break
		}
	}
	if !hasEndpointGap {
		t.Error("Should find uncovered endpoint gap")
	}
}

func TestPriorityValue(t *testing.T) {
	tests := []struct {
		priority string
		want     int
	}{
		{"critical", 4},
		{"high", 3},
		{"medium", 2},
		{"low", 1},
		{"unknown", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.priority, func(t *testing.T) {
			got := priorityValue(tt.priority)
			if got != tt.want {
				t.Errorf("priorityValue(%s) = %d, want %d", tt.priority, got, tt.want)
			}
		})
	}
}

func TestEstimateEffort(t *testing.T) {
	tests := []struct {
		gapCount int
		want     string
	}{
		{0, "small"},
		{3, "small"},
		{5, "small"},
		{6, "medium"},
		{15, "medium"},
		{16, "large"},
		{30, "large"},
		{31, "extensive"},
		{100, "extensive"},
	}

	for _, tt := range tests {
		got := estimateEffort(tt.gapCount)
		if got != tt.want {
			t.Errorf("estimateEffort(%d) = %s, want %s", tt.gapCount, got, tt.want)
		}
	}
}

func TestPrioritizeGaps_Endpoint(t *testing.T) {
	gaps := []CoverageGap{
		{Type: "endpoint", Name: "GET /users"},
	}

	analyzer := NewAnalyzer(&CoverageReport{}, nil)
	analyzer.prioritizeGaps(gaps)

	if gaps[0].Priority != "critical" {
		t.Errorf("Endpoint priority = %s, want critical", gaps[0].Priority)
	}
}

func TestPrioritizeGaps_Function_HighComplexity(t *testing.T) {
	gaps := []CoverageGap{
		{Type: "function", Name: "ComplexFunc", Complexity: 15},
	}

	analyzer := NewAnalyzer(&CoverageReport{}, nil)
	analyzer.prioritizeGaps(gaps)

	if gaps[0].Priority != "critical" {
		t.Errorf("High complexity function priority = %s, want critical", gaps[0].Priority)
	}
}

func TestPrioritizeGaps_Function_MediumComplexity(t *testing.T) {
	gaps := []CoverageGap{
		{Type: "function", Name: "MediumFunc", Complexity: 7},
	}

	analyzer := NewAnalyzer(&CoverageReport{}, nil)
	analyzer.prioritizeGaps(gaps)

	if gaps[0].Priority != "high" {
		t.Errorf("Medium complexity function priority = %s, want high", gaps[0].Priority)
	}
}

func TestPrioritizeGaps_Function_LowComplexity(t *testing.T) {
	gaps := []CoverageGap{
		{Type: "function", Name: "SimpleFunc", Complexity: 3},
	}

	analyzer := NewAnalyzer(&CoverageReport{}, nil)
	analyzer.prioritizeGaps(gaps)

	if gaps[0].Priority != "medium" {
		t.Errorf("Low complexity function priority = %s, want medium", gaps[0].Priority)
	}
}

func TestPrioritizeGaps_Function_Exported(t *testing.T) {
	gaps := []CoverageGap{
		{Type: "function", Name: "ExportedFunc", Complexity: 3, TargetID: "fn1"},
	}

	sysModel := &model.SystemModel{
		Functions: []model.Function{
			{ID: "fn1", Name: "ExportedFunc", Exported: true},
		},
	}

	analyzer := NewAnalyzer(&CoverageReport{}, sysModel)
	analyzer.prioritizeGaps(gaps)

	if gaps[0].Priority != "high" {
		t.Errorf("Exported function priority = %s, want high", gaps[0].Priority)
	}
}

func TestPrioritizeGaps_Block_Large(t *testing.T) {
	gaps := []CoverageGap{
		{Type: "block", StartLine: 1, EndLine: 25}, // 24 lines
	}

	analyzer := NewAnalyzer(&CoverageReport{}, nil)
	analyzer.prioritizeGaps(gaps)

	if gaps[0].Priority != "high" {
		t.Errorf("Large block priority = %s, want high", gaps[0].Priority)
	}
}

func TestPrioritizeGaps_Block_Medium(t *testing.T) {
	gaps := []CoverageGap{
		{Type: "block", StartLine: 1, EndLine: 15}, // 14 lines
	}

	analyzer := NewAnalyzer(&CoverageReport{}, nil)
	analyzer.prioritizeGaps(gaps)

	if gaps[0].Priority != "medium" {
		t.Errorf("Medium block priority = %s, want medium", gaps[0].Priority)
	}
}

func TestPrioritizeGaps_Block_Small(t *testing.T) {
	gaps := []CoverageGap{
		{Type: "block", StartLine: 1, EndLine: 5}, // 4 lines
	}

	analyzer := NewAnalyzer(&CoverageReport{}, nil)
	analyzer.prioritizeGaps(gaps)

	if gaps[0].Priority != "low" {
		t.Errorf("Small block priority = %s, want low", gaps[0].Priority)
	}
}

func TestGenerateTestIntents_Endpoint(t *testing.T) {
	analyzer := NewAnalyzer(&CoverageReport{}, nil)

	gaps := []CoverageGap{
		{
			Type:     "endpoint",
			Name:     "GET /users",
			TargetID: "ep1",
			Priority: "critical",
			Reason:   "Not covered",
		},
	}

	intents := analyzer.GenerateTestIntents(gaps)

	if len(intents) != 1 {
		t.Fatalf("len(intents) = %d, want 1", len(intents))
	}

	intent := intents[0]
	if intent.Level != model.LevelAPI {
		t.Errorf("Level = %s, want %s", intent.Level, model.LevelAPI)
	}
	if intent.TargetKind != "endpoint" {
		t.Errorf("TargetKind = %s, want endpoint", intent.TargetKind)
	}
	if intent.TargetID != "ep1" {
		t.Errorf("TargetID = %s, want ep1", intent.TargetID)
	}
	if intent.Priority != "critical" {
		t.Errorf("Priority = %s, want critical", intent.Priority)
	}
}

func TestGenerateTestIntents_Function(t *testing.T) {
	analyzer := NewAnalyzer(&CoverageReport{}, nil)

	gaps := []CoverageGap{
		{
			Type:     "function",
			Name:     "TestFunc",
			TargetID: "fn1",
			Priority: "high",
			Reason:   "Low coverage",
		},
	}

	intents := analyzer.GenerateTestIntents(gaps)

	if len(intents) != 1 {
		t.Fatalf("len(intents) = %d, want 1", len(intents))
	}

	intent := intents[0]
	if intent.Level != model.LevelUnit {
		t.Errorf("Level = %s, want %s", intent.Level, model.LevelUnit)
	}
	if intent.TargetKind != "function" {
		t.Errorf("TargetKind = %s, want function", intent.TargetKind)
	}
}

func TestGenerateTestIntents_Block(t *testing.T) {
	analyzer := NewAnalyzer(&CoverageReport{}, nil)

	gaps := []CoverageGap{
		{
			Type:      "block",
			File:      "main.go",
			StartLine: 10,
			EndLine:   20,
			Priority:  "low",
			Reason:    "Block not covered",
		},
	}

	intents := analyzer.GenerateTestIntents(gaps)

	if len(intents) != 1 {
		t.Fatalf("len(intents) = %d, want 1", len(intents))
	}

	intent := intents[0]
	if intent.Level != model.LevelUnit {
		t.Errorf("Level = %s, want %s", intent.Level, model.LevelUnit)
	}
	if intent.TargetKind != "block" {
		t.Errorf("TargetKind = %s, want block", intent.TargetKind)
	}
}

func TestFindUncoveredFunctions_NilModel(t *testing.T) {
	report := &CoverageReport{
		Files: []FileCoverage{
			{Path: "main.go", UncoveredLines: []int{10, 11, 12}},
		},
	}

	analyzer := NewAnalyzer(report, nil)
	gaps := analyzer.findUncoveredFunctions()

	if len(gaps) != 0 {
		t.Errorf("len(gaps) = %d, want 0 (no model)", len(gaps))
	}
}

func TestFindUncoveredEndpoints_NilModel(t *testing.T) {
	report := &CoverageReport{
		Files: []FileCoverage{
			{Path: "api.go", UncoveredLines: []int{50}},
		},
	}

	analyzer := NewAnalyzer(report, nil)
	gaps := analyzer.findUncoveredEndpoints()

	if len(gaps) != 0 {
		t.Errorf("len(gaps) = %d, want 0 (no model)", len(gaps))
	}
}

func TestFindUncoveredLines_NoUncovered(t *testing.T) {
	report := &CoverageReport{
		Files: []FileCoverage{
			{Path: "main.go", UncoveredLines: []int{}},
		},
	}

	analyzer := NewAnalyzer(report, nil)
	gaps := analyzer.findUncoveredLines()

	if len(gaps) != 0 {
		t.Errorf("len(gaps) = %d, want 0", len(gaps))
	}
}

func TestFindUncoveredLines_SmallGaps(t *testing.T) {
	report := &CoverageReport{
		Files: []FileCoverage{
			{Path: "main.go", UncoveredLines: []int{10, 11}}, // Only 2 consecutive, < 3
		},
	}

	analyzer := NewAnalyzer(report, nil)
	gaps := analyzer.findUncoveredLines()

	// Small gaps (< 3 lines) should be filtered out
	if len(gaps) != 0 {
		t.Errorf("len(gaps) = %d, want 0 (gap too small)", len(gaps))
	}
}

func TestFindUncoveredLines_MultipleGaps(t *testing.T) {
	report := &CoverageReport{
		Files: []FileCoverage{
			{
				Path:           "main.go",
				UncoveredLines: []int{10, 11, 12, 13, 50, 51, 52, 53, 54}, // Two gaps: 10-13 and 50-54
			},
		},
	}

	analyzer := NewAnalyzer(report, nil)
	gaps := analyzer.findUncoveredLines()

	if len(gaps) != 2 {
		t.Errorf("len(gaps) = %d, want 2", len(gaps))
	}
}
