package codecov

import (
	"sort"

	"github.com/QTest-hq/qtest/pkg/model"
)

// Analyzer analyzes coverage data and suggests test targets
type Analyzer struct {
	report   *CoverageReport
	sysModel *model.SystemModel
}

// NewAnalyzer creates a coverage analyzer
func NewAnalyzer(report *CoverageReport, sysModel *model.SystemModel) *Analyzer {
	return &Analyzer{
		report:   report,
		sysModel: sysModel,
	}
}

// CoverageGap represents a gap in test coverage
type CoverageGap struct {
	File       string `json:"file"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	Type       string `json:"type"` // "function", "endpoint", "branch"
	Name       string `json:"name"`
	Priority   string `json:"priority"` // "critical", "high", "medium", "low"
	Reason     string `json:"reason"`
	TargetID   string `json:"target_id,omitempty"`
	Complexity int    `json:"complexity,omitempty"`
}

// AnalysisResult holds the coverage analysis results
type AnalysisResult struct {
	TotalCoverage   float64       `json:"total_coverage"`
	TargetCoverage  float64       `json:"target_coverage"` // Coverage goal
	Gaps            []CoverageGap `json:"gaps"`
	CriticalGaps    int           `json:"critical_gaps"`
	SuggestedTests  int           `json:"suggested_tests"`
	EstimatedEffort string        `json:"estimated_effort"`
}

// Analyze performs coverage gap analysis
func (a *Analyzer) Analyze(targetCoverage float64) *AnalysisResult {
	result := &AnalysisResult{
		TotalCoverage:  a.report.Percentage,
		TargetCoverage: targetCoverage,
		Gaps:           make([]CoverageGap, 0),
	}

	// Find uncovered functions
	functionGaps := a.findUncoveredFunctions()
	result.Gaps = append(result.Gaps, functionGaps...)

	// Find uncovered endpoints
	endpointGaps := a.findUncoveredEndpoints()
	result.Gaps = append(result.Gaps, endpointGaps...)

	// Find other uncovered lines
	lineGaps := a.findUncoveredLines()
	result.Gaps = append(result.Gaps, lineGaps...)

	// Prioritize gaps
	a.prioritizeGaps(result.Gaps)

	// Sort by priority
	sort.Slice(result.Gaps, func(i, j int) bool {
		return priorityValue(result.Gaps[i].Priority) > priorityValue(result.Gaps[j].Priority)
	})

	// Count critical gaps
	for _, gap := range result.Gaps {
		if gap.Priority == "critical" || gap.Priority == "high" {
			result.CriticalGaps++
		}
	}

	result.SuggestedTests = len(result.Gaps)
	result.EstimatedEffort = estimateEffort(len(result.Gaps))

	return result
}

// findUncoveredFunctions identifies functions without coverage
func (a *Analyzer) findUncoveredFunctions() []CoverageGap {
	var gaps []CoverageGap

	if a.sysModel == nil {
		return gaps
	}

	// Build map of covered lines per file
	uncoveredLines := make(map[string]map[int]bool)
	for _, file := range a.report.Files {
		uncoveredLines[file.Path] = make(map[int]bool)
		for _, line := range file.UncoveredLines {
			uncoveredLines[file.Path][line] = true
		}
	}

	// Check each function
	for _, fn := range a.sysModel.Functions {
		filePath := fn.File
		if filePath == "" {
			continue
		}

		fileUncovered, ok := uncoveredLines[filePath]
		if !ok {
			continue
		}

		// Count uncovered lines in this function
		uncoveredCount := 0
		totalLines := fn.EndLine - fn.StartLine + 1
		for line := fn.StartLine; line <= fn.EndLine; line++ {
			if fileUncovered[line] {
				uncoveredCount++
			}
		}

		// If significant portion is uncovered, add as gap
		if totalLines > 0 && float64(uncoveredCount)/float64(totalLines) > 0.3 {
			gap := CoverageGap{
				File:       filePath,
				StartLine:  fn.StartLine,
				EndLine:    fn.EndLine,
				Type:       "function",
				Name:       fn.Name,
				TargetID:   fn.ID,
				Complexity: fn.Complexity,
				Reason:     "Function has low test coverage",
			}
			gaps = append(gaps, gap)
		}
	}

	return gaps
}

// findUncoveredEndpoints identifies API endpoints without coverage
func (a *Analyzer) findUncoveredEndpoints() []CoverageGap {
	var gaps []CoverageGap

	if a.sysModel == nil {
		return gaps
	}

	// Build coverage lookup
	uncoveredLines := make(map[string]map[int]bool)
	for _, file := range a.report.Files {
		uncoveredLines[file.Path] = make(map[int]bool)
		for _, line := range file.UncoveredLines {
			uncoveredLines[file.Path][line] = true
		}
	}

	// Check each endpoint
	for _, ep := range a.sysModel.Endpoints {
		filePath := ep.File
		if filePath == "" {
			continue
		}

		fileUncovered, ok := uncoveredLines[filePath]
		if !ok {
			continue
		}

		// Check if endpoint handler line is uncovered
		if fileUncovered[ep.Line] {
			gap := CoverageGap{
				File:      filePath,
				StartLine: ep.Line,
				EndLine:   ep.Line,
				Type:      "endpoint",
				Name:      ep.Method + " " + ep.Path,
				TargetID:  ep.ID,
				Reason:    "API endpoint handler is not covered",
			}
			gaps = append(gaps, gap)
		}
	}

	return gaps
}

// findUncoveredLines finds other uncovered code sections
func (a *Analyzer) findUncoveredLines() []CoverageGap {
	var gaps []CoverageGap

	// Group consecutive uncovered lines
	for _, file := range a.report.Files {
		if len(file.UncoveredLines) == 0 {
			continue
		}

		// Sort lines
		lines := make([]int, len(file.UncoveredLines))
		copy(lines, file.UncoveredLines)
		sort.Ints(lines)

		// Group consecutive lines
		start := lines[0]
		end := lines[0]

		for i := 1; i < len(lines); i++ {
			if lines[i] == end+1 {
				end = lines[i]
			} else {
				// Save group if significant
				if end-start >= 3 {
					gap := CoverageGap{
						File:      file.Path,
						StartLine: start,
						EndLine:   end,
						Type:      "block",
						Reason:    "Code block not covered by tests",
					}
					gaps = append(gaps, gap)
				}
				start = lines[i]
				end = lines[i]
			}
		}

		// Don't forget last group
		if end-start >= 3 {
			gap := CoverageGap{
				File:      file.Path,
				StartLine: start,
				EndLine:   end,
				Type:      "block",
				Reason:    "Code block not covered by tests",
			}
			gaps = append(gaps, gap)
		}
	}

	return gaps
}

// prioritizeGaps assigns priorities to coverage gaps
func (a *Analyzer) prioritizeGaps(gaps []CoverageGap) {
	for i := range gaps {
		gap := &gaps[i]

		switch gap.Type {
		case "endpoint":
			// API endpoints are always high priority
			gap.Priority = "critical"

		case "function":
			// Prioritize based on complexity and exported status
			if gap.Complexity > 10 {
				gap.Priority = "critical"
			} else if gap.Complexity > 5 {
				gap.Priority = "high"
			} else {
				gap.Priority = "medium"
			}

			// Check if it's an exported function (more important)
			if a.sysModel != nil {
				for _, fn := range a.sysModel.Functions {
					if fn.ID == gap.TargetID && fn.Exported {
						if gap.Priority == "medium" {
							gap.Priority = "high"
						}
						break
					}
				}
			}

		case "block":
			// Code blocks are lower priority
			lineCount := gap.EndLine - gap.StartLine
			if lineCount > 20 {
				gap.Priority = "high"
			} else if lineCount > 10 {
				gap.Priority = "medium"
			} else {
				gap.Priority = "low"
			}

		default:
			gap.Priority = "low"
		}
	}
}

// GenerateTestIntents creates test intents for coverage gaps
func (a *Analyzer) GenerateTestIntents(gaps []CoverageGap) []model.TestIntent {
	var intents []model.TestIntent

	for _, gap := range gaps {
		intent := model.TestIntent{
			ID:       "cov:" + gap.TargetID,
			Priority: gap.Priority,
			Reason:   gap.Reason + " (" + gap.Name + ")",
		}

		switch gap.Type {
		case "endpoint":
			intent.Level = model.LevelAPI
			intent.TargetKind = "endpoint"
			intent.TargetID = gap.TargetID

		case "function":
			intent.Level = model.LevelUnit
			intent.TargetKind = "function"
			intent.TargetID = gap.TargetID

		default:
			intent.Level = model.LevelUnit
			intent.TargetKind = "block"
			intent.TargetID = gap.File + ":" + string(rune(gap.StartLine))
		}

		intents = append(intents, intent)
	}

	return intents
}

// Helper functions

func priorityValue(priority string) int {
	switch priority {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func estimateEffort(gapCount int) string {
	if gapCount <= 5 {
		return "small"
	} else if gapCount <= 15 {
		return "medium"
	} else if gapCount <= 30 {
		return "large"
	}
	return "extensive"
}
