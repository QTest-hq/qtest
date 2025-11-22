package adapters

import (
	"fmt"

	"github.com/QTest-hq/qtest/internal/parser"
)

// Registry manages framework adapters
type Registry struct {
	adapters     map[Framework]Adapter
	specAdapters map[Framework]SpecAdapter
}

// NewRegistry creates a new adapter registry with all adapters
func NewRegistry() *Registry {
	r := &Registry{
		adapters:     make(map[Framework]Adapter),
		specAdapters: make(map[Framework]SpecAdapter),
	}

	// Register DSL-based adapters (legacy)
	r.Register(NewGoAdapter())
	r.Register(NewJestAdapter())
	r.Register(NewPytestAdapter())

	// Register spec-based adapters (IRSpec)
	r.RegisterSpec(NewGoSpecAdapter())
	r.RegisterSpec(NewJestSpecAdapter())
	r.RegisterSpec(NewPytestSpecAdapter())

	return r
}

// Register adds a DSL adapter to the registry
func (r *Registry) Register(adapter Adapter) {
	r.adapters[adapter.Framework()] = adapter
}

// RegisterSpec adds a spec adapter to the registry
func (r *Registry) RegisterSpec(adapter SpecAdapter) {
	r.specAdapters[adapter.Framework()] = adapter
}

// Get returns a DSL adapter by framework
func (r *Registry) Get(framework Framework) (Adapter, error) {
	adapter, ok := r.adapters[framework]
	if !ok {
		return nil, fmt.Errorf("no adapter for framework: %s", framework)
	}
	return adapter, nil
}

// GetSpec returns a spec adapter by framework
func (r *Registry) GetSpec(framework Framework) (SpecAdapter, error) {
	adapter, ok := r.specAdapters[framework]
	if !ok {
		return nil, fmt.Errorf("no spec adapter for framework: %s", framework)
	}
	return adapter, nil
}

// GetForLanguage returns the default DSL adapter for a programming language
func (r *Registry) GetForLanguage(lang parser.Language) (Adapter, error) {
	switch lang {
	case parser.LanguageGo:
		return r.Get(FrameworkGoTest)
	case parser.LanguageJavaScript, parser.LanguageTypeScript:
		return r.Get(FrameworkJest)
	case parser.LanguagePython:
		return r.Get(FrameworkPytest)
	case parser.LanguageJava:
		return r.Get(FrameworkJUnit) // Not implemented yet
	default:
		return nil, fmt.Errorf("no default adapter for language: %s", lang)
	}
}

// GetSpecForLanguage returns the spec adapter for a programming language
func (r *Registry) GetSpecForLanguage(lang parser.Language) (SpecAdapter, error) {
	switch lang {
	case parser.LanguageGo:
		return r.GetSpec(FrameworkGoTest)
	case parser.LanguageJavaScript, parser.LanguageTypeScript:
		return r.GetSpec(FrameworkJest)
	case parser.LanguagePython:
		return r.GetSpec(FrameworkPytest)
	default:
		return nil, fmt.Errorf("no spec adapter for language: %s", lang)
	}
}

// List returns all registered frameworks
func (r *Registry) List() []Framework {
	frameworks := make([]Framework, 0, len(r.adapters))
	for f := range r.adapters {
		frameworks = append(frameworks, f)
	}
	return frameworks
}
