package model

import (
	"context"

	"github.com/QTest-hq/qtest/internal/parser"
)

// ConvertParsedFile converts internal parser output to model adapter format
func ConvertParsedFile(pf *parser.ParsedFile) *ParsedFile {
	if pf == nil {
		return nil
	}

	// Convert functions
	functions := make([]ParserFunction, len(pf.Functions))
	for i, fn := range pf.Functions {
		params := make([]ParserParameter, len(fn.Parameters))
		for j, p := range fn.Parameters {
			params[j] = ParserParameter{
				Name:     p.Name,
				Type:     p.Type,
				Default:  p.Default,
				Optional: p.Optional,
			}
		}

		functions[i] = ParserFunction{
			ID:         fn.ID,
			Name:       fn.Name,
			StartLine:  fn.StartLine,
			EndLine:    fn.EndLine,
			Parameters: params,
			ReturnType: fn.ReturnType,
			Body:       fn.Body,
			Comments:   fn.Comments,
			Exported:   fn.Exported,
			Async:      fn.Async,
			Class:      fn.Class,
		}
	}

	// Convert classes
	classes := make([]ParserClass, len(pf.Classes))
	for i, cls := range pf.Classes {
		methods := make([]ParserFunction, len(cls.Methods))
		for j, m := range cls.Methods {
			params := make([]ParserParameter, len(m.Parameters))
			for k, p := range m.Parameters {
				params[k] = ParserParameter{
					Name:     p.Name,
					Type:     p.Type,
					Default:  p.Default,
					Optional: p.Optional,
				}
			}
			methods[j] = ParserFunction{
				Name:       m.Name,
				StartLine:  m.StartLine,
				EndLine:    m.EndLine,
				Parameters: params,
				Exported:   m.Exported,
				Async:      m.Async,
				Body:       m.Body,
			}
		}

		props := make([]ParserProperty, len(cls.Properties))
		for j, p := range cls.Properties {
			props[j] = ParserProperty{
				Name:     p.Name,
				Type:     p.Type,
				Exported: p.Exported,
			}
		}

		classes[i] = ParserClass{
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

	// Convert imports
	imports := make([]ParserImport, len(pf.Imports))
	for i, imp := range pf.Imports {
		imports[i] = ParserImport{
			Module: imp.Module,
			Names:  imp.Names,
			Alias:  imp.Alias,
		}
	}

	return &ParsedFile{
		Path:      pf.Path,
		Language:  string(pf.Language),
		Functions: functions,
		Classes:   classes,
		Imports:   imports,
	}
}

// BuildSystemModelFromParser builds a SystemModel by parsing files in a directory
func BuildSystemModelFromParser(ctx context.Context, p *parser.Parser, workspacePath, repoName, branch, commitSHA string) (*SystemModel, error) {
	adapter := NewParserAdapter(repoName, branch, commitSHA)

	// Use the parser's directory walking capability
	files, err := p.ParseDirectory(ctx, workspacePath)
	if err != nil {
		return nil, err
	}

	// Add each parsed file to the adapter
	for _, pf := range files {
		converted := ConvertParsedFile(pf)
		adapter.AddFile(converted)
	}

	return adapter.Build()
}
