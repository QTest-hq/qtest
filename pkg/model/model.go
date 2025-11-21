// Package model defines the Universal System Model - a language-agnostic
// intermediate representation of any codebase. This is the common IR that
// Tree-sitter output and framework supplements feed into, and that LLM
// uses to generate the test pyramid.
package model

import "time"

// SystemModel is the universal intermediate representation of a codebase.
// It's language-agnostic and represents everything needed to generate
// a complete test pyramid.
type SystemModel struct {
	// Metadata
	ID          string    `json:"id"`
	Repository  string    `json:"repository"`
	Branch      string    `json:"branch"`
	CommitSHA   string    `json:"commit_sha"`
	CreatedAt   time.Time `json:"created_at"`

	// Structure
	Modules     []Module     `json:"modules"`      // Packages, namespaces, folders
	Functions   []Function   `json:"functions"`    // All callable units
	Types       []TypeDef    `json:"types"`        // Structs, classes, interfaces
	Endpoints   []Endpoint   `json:"endpoints"`    // HTTP routes (from supplements)
	Events      []Event      `json:"events"`       // Message handlers, webhooks

	// Analysis
	CallGraph    []CallEdge            `json:"call_graph"`    // Function dependencies
	RiskScores   map[string]RiskScore  `json:"risk_scores"`   // Per-function risk
	TestTargets  []TestTarget          `json:"test_targets"`  // Prioritized test targets

	// Languages detected
	Languages    []string `json:"languages"`
}

// Module represents a logical grouping (package, namespace, folder)
type Module struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Path     string   `json:"path"`      // File system path
	Language string   `json:"language"`
	Files    []string `json:"files"`     // File paths in this module
}

// Function represents any callable unit (function, method, lambda)
type Function struct {
	ID          string      `json:"id"`           // Unique: module:file:line:name
	Name        string      `json:"name"`
	Module      string      `json:"module"`       // Parent module ID
	File        string      `json:"file"`
	StartLine   int         `json:"start_line"`
	EndLine     int         `json:"end_line"`

	// Signature
	Parameters  []Parameter `json:"parameters"`
	Returns     []Parameter `json:"returns"`      // Return types

	// Context
	Class       string      `json:"class,omitempty"`       // If it's a method
	Receiver    string      `json:"receiver,omitempty"`    // Go-style receiver
	Decorators  []string    `json:"decorators,omitempty"`  // Python decorators, Java annotations

	// Characteristics
	Exported    bool        `json:"exported"`
	Async       bool        `json:"async"`
	Pure        bool        `json:"pure"`         // No side effects (estimated)

	// Source
	Body        string      `json:"body,omitempty"`        // Full source code
	DocComment  string      `json:"doc_comment,omitempty"`

	// Analysis
	Complexity  int         `json:"complexity"`   // Cyclomatic complexity
	LOC         int         `json:"loc"`          // Lines of code
}

// Parameter represents a function parameter or return value
type Parameter struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Optional bool   `json:"optional"`
	Default  string `json:"default,omitempty"`
}

// TypeDef represents a type definition (struct, class, interface, enum)
type TypeDef struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Kind      TypeKind   `json:"kind"`        // struct, class, interface, enum
	Module    string     `json:"module"`
	File      string     `json:"file"`
	Line      int        `json:"line"`

	Fields    []Field    `json:"fields,omitempty"`
	Methods   []string   `json:"methods,omitempty"`    // Function IDs
	Extends   string     `json:"extends,omitempty"`    // Parent type
	Implements []string  `json:"implements,omitempty"` // Interfaces

	Exported  bool       `json:"exported"`
}

// TypeKind represents the kind of type definition
type TypeKind string

const (
	TypeKindStruct    TypeKind = "struct"
	TypeKindClass     TypeKind = "class"
	TypeKindInterface TypeKind = "interface"
	TypeKindEnum      TypeKind = "enum"
	TypeKindAlias     TypeKind = "alias"
)

// Field represents a field in a type
type Field struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Exported bool   `json:"exported"`
	Tags     string `json:"tags,omitempty"` // Go struct tags, Java annotations
}

// Endpoint represents an HTTP API endpoint
type Endpoint struct {
	ID          string   `json:"id"`
	Method      string   `json:"method"`       // GET, POST, PUT, DELETE, etc.
	Path        string   `json:"path"`         // Route path with params
	Handler     string   `json:"handler"`      // Function ID that handles this
	File        string   `json:"file"`
	Line        int      `json:"line"`

	// Route info
	PathParams  []string `json:"path_params,omitempty"`  // e.g., :id, {userId}
	QueryParams []string `json:"query_params,omitempty"`

	// Request/Response
	RequestBody  string  `json:"request_body,omitempty"`  // Type name
	ResponseBody string  `json:"response_body,omitempty"` // Type name

	// Framework
	Framework   string   `json:"framework"`    // express, fastapi, gin, etc.
	Middleware  []string `json:"middleware,omitempty"`
}

// Event represents an event handler (message queue, webhook, etc.)
type Event struct {
	ID       string `json:"id"`
	Name     string `json:"name"`       // Event/topic name
	Kind     string `json:"kind"`       // queue, webhook, cron, etc.
	Handler  string `json:"handler"`    // Function ID
	File     string `json:"file"`
	Line     int    `json:"line"`
}

// CallEdge represents a function call relationship
type CallEdge struct {
	Caller string `json:"caller"` // Function ID
	Callee string `json:"callee"` // Function ID
	File   string `json:"file"`
	Line   int    `json:"line"`
}

// RiskScore represents the risk assessment for a function
type RiskScore struct {
	FunctionID  string  `json:"function_id"`
	Score       float64 `json:"score"`       // 0.0 - 1.0
	Complexity  float64 `json:"complexity"`  // Component scores
	Centrality  float64 `json:"centrality"`  // How many things depend on it
	Churn       float64 `json:"churn"`       // How often it changes
	HasTests    bool    `json:"has_tests"`   // Existing test coverage
}

// TestTarget represents a prioritized item to generate tests for
type TestTarget struct {
	ID          string     `json:"id"`
	Kind        TargetKind `json:"kind"`        // unit, integration, api, e2e
	FunctionID  string     `json:"function_id,omitempty"`
	EndpointID  string     `json:"endpoint_id,omitempty"`
	Priority    int        `json:"priority"`    // 1 = highest
	RiskScore   float64    `json:"risk_score"`
	Reason      string     `json:"reason"`      // Why this was prioritized
}

// TargetKind represents what kind of test to generate
type TargetKind string

const (
	TargetKindUnit        TargetKind = "unit"
	TargetKindIntegration TargetKind = "integration"
	TargetKindAPI         TargetKind = "api"
	TargetKindE2E         TargetKind = "e2e"
)

// Stats returns statistics about the system model
func (m *SystemModel) Stats() map[string]int {
	return map[string]int{
		"modules":     len(m.Modules),
		"functions":   len(m.Functions),
		"types":       len(m.Types),
		"endpoints":   len(m.Endpoints),
		"events":      len(m.Events),
		"test_targets": len(m.TestTargets),
	}
}

// GetFunction returns a function by ID
func (m *SystemModel) GetFunction(id string) *Function {
	for i := range m.Functions {
		if m.Functions[i].ID == id {
			return &m.Functions[i]
		}
	}
	return nil
}

// GetEndpoint returns an endpoint by ID
func (m *SystemModel) GetEndpoint(id string) *Endpoint {
	for i := range m.Endpoints {
		if m.Endpoints[i].ID == id {
			return &m.Endpoints[i]
		}
	}
	return nil
}

// GetExportedFunctions returns all exported functions
func (m *SystemModel) GetExportedFunctions() []Function {
	var exported []Function
	for _, fn := range m.Functions {
		if fn.Exported {
			exported = append(exported, fn)
		}
	}
	return exported
}

// GetFunctionsByModule returns functions in a specific module
func (m *SystemModel) GetFunctionsByModule(moduleID string) []Function {
	var funcs []Function
	for _, fn := range m.Functions {
		if fn.Module == moduleID {
			funcs = append(funcs, fn)
		}
	}
	return funcs
}
