package model

// Assertion represents a single test assertion
type Assertion struct {
	Kind     string      `json:"kind" yaml:"kind"`         // "equality", "contains", "not_null", "status_code", "expression"
	Actual   string      `json:"actual" yaml:"actual"`     // "result", "status", "body.id", "response.data[0].name"
	Expected interface{} `json:"expected" yaml:"expected"` // expected value
}

// TestSpec represents a complete test specification
// This is the canonical DSL that adapters consume
type TestSpec struct {
	ID          string    `json:"id" yaml:"id"`
	Level       TestLevel `json:"level" yaml:"level"`
	TargetKind  string    `json:"target_kind" yaml:"target_kind"` // "function" | "endpoint"
	TargetID    string    `json:"target_id" yaml:"target_id"`
	Description string    `json:"description" yaml:"description"`

	// For function tests
	FunctionName string                 `json:"function_name,omitempty" yaml:"function_name,omitempty"`
	Inputs       map[string]interface{} `json:"inputs,omitempty" yaml:"inputs,omitempty"` // function args

	// For API tests
	Method      string                 `json:"method,omitempty" yaml:"method,omitempty"`           // GET, POST, etc.
	Path        string                 `json:"path,omitempty" yaml:"path,omitempty"`               // /users/:id
	PathParams  map[string]interface{} `json:"path_params,omitempty" yaml:"path_params,omitempty"` // {id: 1}
	QueryParams map[string]interface{} `json:"query_params,omitempty" yaml:"query_params,omitempty"`
	Headers     map[string]string      `json:"headers,omitempty" yaml:"headers,omitempty"`
	Body        interface{}            `json:"body,omitempty" yaml:"body,omitempty"` // request body

	// Expected outcomes
	Expected   map[string]interface{} `json:"expected,omitempty" yaml:"expected,omitempty"` // status, body, etc.
	Assertions []Assertion            `json:"assertions" yaml:"assertions"`

	// Metadata
	Tags     []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Priority string   `json:"priority,omitempty" yaml:"priority,omitempty"`
}

// TestSpecSet is a collection of test specs
type TestSpecSet struct {
	ModelID    string     `json:"model_id"`
	Repository string     `json:"repository"`
	Language   string     `json:"language"`  // target language for code generation
	Framework  string     `json:"framework"` // test framework (jest, pytest, go)
	Specs      []TestSpec `json:"specs"`
}

// Stats returns spec set statistics
func (s *TestSpecSet) Stats() map[string]int {
	unit, api, e2e := 0, 0, 0
	for _, spec := range s.Specs {
		switch spec.Level {
		case LevelUnit:
			unit++
		case LevelAPI:
			api++
		case LevelE2E:
			e2e++
		}
	}
	return map[string]int{
		"total": len(s.Specs),
		"unit":  unit,
		"api":   api,
		"e2e":   e2e,
	}
}

// FilterByLevel returns specs of a specific level
func (s *TestSpecSet) FilterByLevel(level TestLevel) []TestSpec {
	var filtered []TestSpec
	for _, spec := range s.Specs {
		if spec.Level == level {
			filtered = append(filtered, spec)
		}
	}
	return filtered
}

// GetByID returns a spec by ID
func (s *TestSpecSet) GetByID(id string) *TestSpec {
	for i := range s.Specs {
		if s.Specs[i].ID == id {
			return &s.Specs[i]
		}
	}
	return nil
}
