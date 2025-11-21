package model

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ParserAdapter converts parser output to the model builder format.
// This bridges the gap between Tree-sitter parsed output and our Universal System Model.
type ParserAdapter struct {
	builder *Builder
}

// NewParserAdapter creates a new adapter
func NewParserAdapter(repo, branch, commitSHA string) *ParserAdapter {
	return &ParserAdapter{
		builder: NewBuilder(repo, branch, commitSHA),
	}
}

// RegisterSupplement adds a supplement to the underlying builder
func (a *ParserAdapter) RegisterSupplement(s Supplement) {
	a.builder.RegisterSupplement(s)
}

// ParsedFile represents the output from the Tree-sitter parser.
// This mirrors the structure in internal/parser/types.go
type ParsedFile struct {
	Path      string
	Language  string
	Functions []ParserFunction
	Classes   []ParserClass
	Imports   []ParserImport
}

// ParserFunction mirrors parser.Function
type ParserFunction struct {
	ID         string
	Name       string
	StartLine  int
	EndLine    int
	Parameters []ParserParameter
	ReturnType string
	Body       string
	Comments   string
	Exported   bool
	Async      bool
	Class      string
}

// ParserClass mirrors parser.Class
type ParserClass struct {
	ID         string
	Name       string
	StartLine  int
	EndLine    int
	Methods    []ParserFunction
	Properties []ParserProperty
	Comments   string
	Exported   bool
	Extends    string
	Implements []string
}

// ParserParameter mirrors parser.Parameter
type ParserParameter struct {
	Name     string
	Type     string
	Default  string
	Optional bool
}

// ParserProperty mirrors parser.Property
type ParserProperty struct {
	Name     string
	Type     string
	Exported bool
}

// ParserImport mirrors parser.Import
type ParserImport struct {
	Module string
	Names  []string
	Alias  string
}

// AddFile adds a parsed file to the model
func (a *ParserAdapter) AddFile(pf *ParsedFile) {
	// Convert parser functions to builder format
	functions := make([]ParsedFunction, len(pf.Functions))
	for i, fn := range pf.Functions {
		params := make([]ParsedParam, len(fn.Parameters))
		for j, p := range fn.Parameters {
			params[j] = ParsedParam{
				Name:     p.Name,
				Type:     p.Type,
				Optional: p.Optional,
				Default:  p.Default,
			}
		}

		// Parse return type into returns slice
		var returns []ParsedParam
		if fn.ReturnType != "" {
			returns = append(returns, ParsedParam{Type: fn.ReturnType})
		}

		functions[i] = ParsedFunction{
			Name:       fn.Name,
			StartLine:  fn.StartLine,
			EndLine:    fn.EndLine,
			Parameters: params,
			Returns:    returns,
			Class:      fn.Class,
			Exported:   fn.Exported,
			Async:      fn.Async,
			Body:       fn.Body,
			DocComment: fn.Comments,
		}
	}

	// Convert parser classes to builder format
	classes := make([]ParsedClass, len(pf.Classes))
	for i, cls := range pf.Classes {
		methods := make([]ParsedFunction, len(cls.Methods))
		for j, m := range cls.Methods {
			params := make([]ParsedParam, len(m.Parameters))
			for k, p := range m.Parameters {
				params[k] = ParsedParam{
					Name:     p.Name,
					Type:     p.Type,
					Optional: p.Optional,
					Default:  p.Default,
				}
			}
			methods[j] = ParsedFunction{
				Name:       m.Name,
				StartLine:  m.StartLine,
				EndLine:    m.EndLine,
				Parameters: params,
				Exported:   m.Exported,
				Async:      m.Async,
				Body:       m.Body,
			}
		}

		props := make([]ParsedProperty, len(cls.Properties))
		for j, p := range cls.Properties {
			props[j] = ParsedProperty{
				Name:     p.Name,
				Type:     p.Type,
				Exported: p.Exported,
			}
		}

		classes[i] = ParsedClass{
			Name:       cls.Name,
			StartLine:  cls.StartLine,
			EndLine:    cls.EndLine,
			Methods:    methods,
			Properties: props,
			Extends:    cls.Extends,
			Implements: cls.Implements,
			Exported:   cls.Exported,
		}
	}

	a.builder.AddParsedFile(pf.Path, pf.Language, functions, classes)
}

// Build finalizes and returns the system model
func (a *ParserAdapter) Build() (*SystemModel, error) {
	return a.builder.Build()
}

// BuildFromDirectory scans a directory and builds a system model.
// This is a convenience method for common use cases.
func BuildFromDirectory(ctx context.Context, dir string, parser FileParser, supplements []Supplement) (*SystemModel, error) {
	// Determine repo info (simplified - real impl would use git)
	repoName := filepath.Base(dir)

	adapter := NewParserAdapter(repoName, "main", "")

	// Register supplements
	for _, s := range supplements {
		adapter.RegisterSupplement(s)
	}

	// Walk directory and parse files
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip hidden directories and common non-source directories
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only parse source files
		ext := strings.ToLower(filepath.Ext(path))
		if !isSourceFile(ext) {
			return nil
		}

		// Parse file
		parsed, err := parser.ParseFile(ctx, path)
		if err != nil {
			return nil // Skip unparseable files
		}

		adapter.AddFile(parsed)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return adapter.Build()
}

// FileParser interface for parsing source files
type FileParser interface {
	ParseFile(ctx context.Context, path string) (*ParsedFile, error)
}

// isSourceFile checks if a file extension is a supported source file
func isSourceFile(ext string) bool {
	switch ext {
	case ".go", ".py", ".js", ".jsx", ".ts", ".tsx", ".java":
		return true
	default:
		return false
	}
}
