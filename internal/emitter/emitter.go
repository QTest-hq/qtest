// Package emitter converts TestSpecs to runnable test code
package emitter

import (
	"fmt"

	"github.com/QTest-hq/qtest/pkg/model"
)

// Emitter converts test specs to test code for a specific framework
type Emitter interface {
	// Name returns the emitter name (e.g., "supertest", "pytest", "go")
	Name() string

	// Language returns the target language
	Language() string

	// Framework returns the test framework name
	Framework() string

	// FileExtension returns the test file extension (e.g., ".test.js", "_test.go")
	FileExtension() string

	// Emit generates test code from a set of specs
	Emit(specs []model.TestSpec) (string, error)

	// EmitSingle generates test code for a single spec
	EmitSingle(spec model.TestSpec) (string, error)
}

// Registry holds all available emitters
type Registry struct {
	emitters map[string]Emitter
}

// NewRegistry creates a new emitter registry with all built-in emitters
func NewRegistry() *Registry {
	r := &Registry{
		emitters: make(map[string]Emitter),
	}

	// Register built-in emitters
	r.Register(&SupertestEmitter{})
	r.Register(&GoHTTPEmitter{})
	r.Register(&PytestEmitter{})
	r.Register(&JUnitEmitter{})
	r.Register(&RSpecEmitter{})

	// E2E test emitters
	r.Register(&PlaywrightEmitter{})
	r.Register(&CypressEmitter{})

	return r
}

// Register adds an emitter to the registry
func (r *Registry) Register(e Emitter) {
	r.emitters[e.Name()] = e
}

// Get returns an emitter by name
func (r *Registry) Get(name string) (Emitter, error) {
	e, ok := r.emitters[name]
	if !ok {
		return nil, fmt.Errorf("emitter not found: %s", name)
	}
	return e, nil
}

// GetForLanguage returns the default emitter for a language
func (r *Registry) GetForLanguage(lang string) (Emitter, error) {
	for _, e := range r.emitters {
		if e.Language() == lang {
			return e, nil
		}
	}
	return nil, fmt.Errorf("no emitter for language: %s", lang)
}

// List returns all registered emitter names
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.emitters))
	for name := range r.emitters {
		names = append(names, name)
	}
	return names
}
