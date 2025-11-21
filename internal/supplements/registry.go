// Package supplements provides framework-specific analyzers that detect
// patterns like API routes and add them to the Universal System Model.
//
// Supplements are the "plugin" layer that makes QTest work with any framework.
// Tree-sitter provides the universal parsing, supplements provide the semantics.
package supplements

import (
	"github.com/QTest-hq/qtest/pkg/model"
)

// Registry holds all available supplements
type Registry struct {
	supplements []model.Supplement
}

// NewRegistry creates a new supplement registry with all built-in supplements
func NewRegistry() *Registry {
	r := &Registry{
		supplements: make([]model.Supplement, 0),
	}

	// Register all built-in supplements
	r.Register(&ExpressSupplement{})
	r.Register(&FastAPISupplement{})
	r.Register(&GinSupplement{})
	r.Register(&SpringBootSupplement{})
	r.Register(&DjangoSupplement{})
	// Future: r.Register(&NestJSSupplement{})

	return r
}

// Register adds a supplement to the registry
func (r *Registry) Register(s model.Supplement) {
	r.supplements = append(r.supplements, s)
}

// GetAll returns all registered supplements
func (r *Registry) GetAll() []model.Supplement {
	return r.supplements
}

// Detect returns supplements that should run on the given files
func (r *Registry) Detect(files []string) []model.Supplement {
	var applicable []model.Supplement
	for _, s := range r.supplements {
		if s.Detect(files) {
			applicable = append(applicable, s)
		}
	}
	return applicable
}
