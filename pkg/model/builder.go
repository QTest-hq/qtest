package model

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Builder constructs a SystemModel from parsed files
type Builder struct {
	model       *SystemModel
	supplements []Supplement
}

// Supplement is the interface that framework-specific analyzers implement.
// Supplements detect framework-specific patterns (like Express routes)
// and add them to the SystemModel.
type Supplement interface {
	// Name returns the supplement name (e.g., "express", "fastapi")
	Name() string

	// Detect returns true if this supplement should run on the given files
	Detect(files []string) bool

	// Analyze adds framework-specific information to the model
	Analyze(model *SystemModel) error
}

// NewBuilder creates a new model builder
func NewBuilder(repo, branch, commitSHA string) *Builder {
	return &Builder{
		model: &SystemModel{
			ID:          uuid.New().String(),
			Repository:  repo,
			Branch:      branch,
			CommitSHA:   commitSHA,
			CreatedAt:   time.Now(),
			Modules:     make([]Module, 0),
			Functions:   make([]Function, 0),
			Types:       make([]TypeDef, 0),
			Endpoints:   make([]Endpoint, 0),
			Events:      make([]Event, 0),
			CallGraph:   make([]CallEdge, 0),
			RiskScores:  make(map[string]RiskScore),
			TestTargets: make([]TestTarget, 0),
			Languages:   make([]string, 0),
		},
		supplements: make([]Supplement, 0),
	}
}

// RegisterSupplement adds a framework supplement to the builder
func (b *Builder) RegisterSupplement(s Supplement) {
	b.supplements = append(b.supplements, s)
}

// AddParsedFile adds a parsed file to the model
func (b *Builder) AddParsedFile(path, language string, functions []ParsedFunction, classes []ParsedClass) {
	// Track language
	langFound := false
	for _, l := range b.model.Languages {
		if l == language {
			langFound = true
			break
		}
	}
	if !langFound {
		b.model.Languages = append(b.model.Languages, language)
	}

	// Determine module (directory)
	dir := filepath.Dir(path)
	moduleName := filepath.Base(dir)
	moduleID := fmt.Sprintf("mod:%s", dir)

	// Find or create module
	moduleExists := false
	for i := range b.model.Modules {
		if b.model.Modules[i].ID == moduleID {
			b.model.Modules[i].Files = append(b.model.Modules[i].Files, path)
			moduleExists = true
			break
		}
	}
	if !moduleExists {
		b.model.Modules = append(b.model.Modules, Module{
			ID:       moduleID,
			Name:     moduleName,
			Path:     dir,
			Language: language,
			Files:    []string{path},
		})
	}

	// Add functions
	for _, fn := range functions {
		funcID := fmt.Sprintf("%s:%d:%s", path, fn.StartLine, fn.Name)
		b.model.Functions = append(b.model.Functions, Function{
			ID:         funcID,
			Name:       fn.Name,
			Module:     moduleID,
			File:       path,
			StartLine:  fn.StartLine,
			EndLine:    fn.EndLine,
			Parameters: convertParameters(fn.Parameters),
			Returns:    convertParameters(fn.Returns),
			Class:      fn.Class,
			Decorators: fn.Decorators,
			Exported:   fn.Exported,
			Async:      fn.Async,
			Body:       fn.Body,
			DocComment: fn.DocComment,
			LOC:        fn.EndLine - fn.StartLine + 1,
		})
	}

	// Add classes as types
	for _, cls := range classes {
		typeID := fmt.Sprintf("%s:%d:%s", path, cls.StartLine, cls.Name)
		typeDef := TypeDef{
			ID:         typeID,
			Name:       cls.Name,
			Kind:       TypeKindClass,
			Module:     moduleID,
			File:       path,
			Line:       cls.StartLine,
			Fields:     convertFields(cls.Properties),
			Extends:    cls.Extends,
			Implements: cls.Implements,
			Exported:   cls.Exported,
		}

		// Add method references
		for _, method := range cls.Methods {
			methodID := fmt.Sprintf("%s:%d:%s.%s", path, method.StartLine, cls.Name, method.Name)
			typeDef.Methods = append(typeDef.Methods, methodID)

			// Also add method as a function
			b.model.Functions = append(b.model.Functions, Function{
				ID:         methodID,
				Name:       method.Name,
				Module:     moduleID,
				File:       path,
				StartLine:  method.StartLine,
				EndLine:    method.EndLine,
				Parameters: convertParameters(method.Parameters),
				Returns:    convertParameters(method.Returns),
				Class:      cls.Name,
				Exported:   method.Exported,
				Async:      method.Async,
				Body:       method.Body,
				LOC:        method.EndLine - method.StartLine + 1,
			})
		}

		b.model.Types = append(b.model.Types, typeDef)
	}
}

// Build finalizes the model by running supplements and computing analysis
func (b *Builder) Build() (*SystemModel, error) {
	// Collect all files
	var allFiles []string
	for _, mod := range b.model.Modules {
		allFiles = append(allFiles, mod.Files...)
	}

	// Run applicable supplements
	for _, supp := range b.supplements {
		if supp.Detect(allFiles) {
			if err := supp.Analyze(b.model); err != nil {
				return nil, fmt.Errorf("supplement %s failed: %w", supp.Name(), err)
			}
		}
	}

	// Compute risk scores
	b.computeRiskScores()

	// Generate test targets
	b.generateTestTargets()

	return b.model, nil
}

// computeRiskScores calculates risk scores for all functions
func (b *Builder) computeRiskScores() {
	for _, fn := range b.model.Functions {
		score := RiskScore{
			FunctionID: fn.ID,
		}

		// Complexity component (based on LOC as simple heuristic)
		if fn.LOC > 50 {
			score.Complexity = 0.9
		} else if fn.LOC > 20 {
			score.Complexity = 0.6
		} else if fn.LOC > 10 {
			score.Complexity = 0.3
		} else {
			score.Complexity = 0.1
		}

		// Centrality (how many things call this function)
		callCount := 0
		for _, edge := range b.model.CallGraph {
			if edge.Callee == fn.ID {
				callCount++
			}
		}
		if callCount > 10 {
			score.Centrality = 0.9
		} else if callCount > 5 {
			score.Centrality = 0.6
		} else if callCount > 0 {
			score.Centrality = 0.3
		}

		// Overall score (weighted average)
		score.Score = score.Complexity*0.5 + score.Centrality*0.3 + score.Churn*0.2

		b.model.RiskScores[fn.ID] = score
	}
}

// generateTestTargets creates prioritized test targets
func (b *Builder) generateTestTargets() {
	priority := 1

	// API endpoints get highest priority (integration tests)
	for _, ep := range b.model.Endpoints {
		b.model.TestTargets = append(b.model.TestTargets, TestTarget{
			ID:         fmt.Sprintf("target:api:%s", ep.ID),
			Kind:       TargetKindAPI,
			EndpointID: ep.ID,
			Priority:   priority,
			Reason:     fmt.Sprintf("API endpoint: %s %s", ep.Method, ep.Path),
		})
		priority++
	}

	// High-risk exported functions
	for _, fn := range b.model.Functions {
		if !fn.Exported {
			continue
		}

		score := b.model.RiskScores[fn.ID]
		if score.Score > 0.5 {
			b.model.TestTargets = append(b.model.TestTargets, TestTarget{
				ID:         fmt.Sprintf("target:unit:%s", fn.ID),
				Kind:       TargetKindUnit,
				FunctionID: fn.ID,
				Priority:   priority,
				RiskScore:  score.Score,
				Reason:     fmt.Sprintf("High-risk function (score: %.2f)", score.Score),
			})
			priority++
		}
	}

	// Remaining exported functions
	for _, fn := range b.model.Functions {
		if !fn.Exported {
			continue
		}

		// Skip if already added as high-risk
		alreadyAdded := false
		for _, t := range b.model.TestTargets {
			if t.FunctionID == fn.ID {
				alreadyAdded = true
				break
			}
		}
		if alreadyAdded {
			continue
		}

		score := b.model.RiskScores[fn.ID]
		b.model.TestTargets = append(b.model.TestTargets, TestTarget{
			ID:         fmt.Sprintf("target:unit:%s", fn.ID),
			Kind:       TargetKindUnit,
			FunctionID: fn.ID,
			Priority:   priority,
			RiskScore:  score.Score,
			Reason:     "Exported function",
		})
		priority++
	}
}

// ParsedFunction is a simplified function representation from the parser
type ParsedFunction struct {
	Name       string
	StartLine  int
	EndLine    int
	Parameters []ParsedParam
	Returns    []ParsedParam
	Class      string
	Decorators []string
	Exported   bool
	Async      bool
	Body       string
	DocComment string
}

// ParsedParam is a simplified parameter
type ParsedParam struct {
	Name     string
	Type     string
	Optional bool
	Default  string
}

// ParsedClass is a simplified class representation
type ParsedClass struct {
	Name       string
	StartLine  int
	EndLine    int
	Methods    []ParsedFunction
	Properties []ParsedProperty
	Extends    string
	Implements []string
	Exported   bool
}

// ParsedProperty is a simplified property
type ParsedProperty struct {
	Name     string
	Type     string
	Exported bool
}

// Helper functions

func convertParameters(params []ParsedParam) []Parameter {
	result := make([]Parameter, len(params))
	for i, p := range params {
		result[i] = Parameter{
			Name:     p.Name,
			Type:     p.Type,
			Optional: p.Optional,
			Default:  p.Default,
		}
	}
	return result
}

func convertFields(props []ParsedProperty) []Field {
	result := make([]Field, len(props))
	for i, p := range props {
		result[i] = Field{
			Name:     p.Name,
			Type:     p.Type,
			Exported: p.Exported,
		}
	}
	return result
}

// containsAny checks if any of the patterns exist in any file path
func containsAny(files []string, patterns []string) bool {
	for _, f := range files {
		for _, p := range patterns {
			if strings.Contains(f, p) {
				return true
			}
		}
	}
	return false
}
