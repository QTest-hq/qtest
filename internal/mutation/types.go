// Package mutation provides mutation testing functionality
package mutation

import "time"

// MutationConfig configures mutation testing execution
type MutationConfig struct {
	// MaxMutantsPerFunction limits mutants generated per function
	MaxMutantsPerFunction int `json:"max_mutants_per_function"`

	// Timeout is the maximum time for mutation testing
	Timeout time.Duration `json:"timeout"`

	// TimeoutPerMutant is the timeout for running each mutant
	TimeoutPerMutant time.Duration `json:"timeout_per_mutant"`

	// Mode is the execution mode: "fast", "thorough", or "off"
	Mode string `json:"mode"`
}

// DefaultConfig returns default mutation testing configuration
func DefaultConfig() MutationConfig {
	return MutationConfig{
		MaxMutantsPerFunction: 5,
		Timeout:               2 * time.Minute,
		TimeoutPerMutant:      5 * time.Second,
		Mode:                  "fast",
	}
}

// ThoroughConfig returns configuration for thorough mutation testing
func ThoroughConfig() MutationConfig {
	return MutationConfig{
		MaxMutantsPerFunction: 10,
		Timeout:               10 * time.Minute,
		TimeoutPerMutant:      10 * time.Second,
		Mode:                  "thorough",
	}
}

// Result holds the results of mutation testing
type Result struct {
	// SourceFile is the file that was mutated
	SourceFile string `json:"source_file"`

	// TestFile is the test file used to kill mutants
	TestFile string `json:"test_file"`

	// Total is the total number of mutants generated
	Total int `json:"total"`

	// Killed is the number of mutants killed by tests
	Killed int `json:"killed"`

	// Survived is the number of mutants that survived
	Survived int `json:"survived"`

	// Timeout is the number of mutants that timed out
	Timeout int `json:"timeout"`

	// Score is the mutation score (killed / total)
	Score float64 `json:"score"`

	// Duration is how long mutation testing took
	Duration time.Duration `json:"duration"`

	// Mutants contains details about each mutant
	Mutants []Mutant `json:"mutants,omitempty"`

	// Error contains any error message
	Error string `json:"error,omitempty"`
}

// Mutant represents a single code mutation
type Mutant struct {
	// ID is a unique identifier for the mutant
	ID string `json:"id"`

	// Type is the mutation type (e.g., "arithmetic", "comparison", "boolean")
	Type string `json:"type"`

	// Description describes what was mutated
	Description string `json:"description"`

	// Line is the line number where the mutation occurred
	Line int `json:"line"`

	// Status is the result: "killed", "survived", "timeout", "error"
	Status string `json:"status"`

	// Original is the original code
	Original string `json:"original,omitempty"`

	// Mutated is the mutated code
	Mutated string `json:"mutated,omitempty"`
}

// MutantStatus constants
const (
	StatusKilled   = "killed"
	StatusSurvived = "survived"
	StatusTimeout  = "timeout"
	StatusError    = "error"
)

// QualityThreshold constants based on mutation-strategy.md
const (
	ThresholdGood       = 0.70 // >= 70% is good
	ThresholdAcceptable = 0.50 // 50-70% is acceptable
	// < 50% is poor
)

// Quality returns the quality assessment based on the score
func (r *Result) Quality() string {
	if r.Score >= ThresholdGood {
		return "good"
	}
	if r.Score >= ThresholdAcceptable {
		return "acceptable"
	}
	return "poor"
}

// HasMutants returns true if any mutants were generated
func (r *Result) HasMutants() bool {
	return r.Total > 0
}
