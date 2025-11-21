package parser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
)

// Parser parses source code files using tree-sitter
type Parser struct {
	goParser *sitter.Parser
	pyParser *sitter.Parser
	jsParser *sitter.Parser
}

// NewParser creates a new parser with all language support
func NewParser() *Parser {
	goParser := sitter.NewParser()
	goParser.SetLanguage(golang.GetLanguage())

	pyParser := sitter.NewParser()
	pyParser.SetLanguage(python.GetLanguage())

	jsParser := sitter.NewParser()
	jsParser.SetLanguage(javascript.GetLanguage())

	return &Parser{
		goParser: goParser,
		pyParser: pyParser,
		jsParser: jsParser,
	}
}

// ParseFile parses a single file
func (p *Parser) ParseFile(ctx context.Context, filePath string) (*ParsedFile, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	lang := DetectLanguage(filePath)
	if lang == LanguageUnknown {
		return nil, fmt.Errorf("unsupported language for file: %s", filePath)
	}

	return p.ParseContent(ctx, filePath, string(content), lang)
}

// ParseContent parses source code content
func (p *Parser) ParseContent(ctx context.Context, filePath, content string, lang Language) (*ParsedFile, error) {
	var parser *sitter.Parser
	switch lang {
	case LanguageGo:
		parser = p.goParser
	case LanguagePython:
		parser = p.pyParser
	case LanguageJavaScript, LanguageTypeScript:
		parser = p.jsParser // Use JS parser for TS as well (basic support)
	default:
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}

	tree, err := parser.ParseCtx(ctx, nil, []byte(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse: %w", err)
	}
	defer tree.Close()

	parsed := &ParsedFile{
		Path:      filePath,
		Language:  lang,
		Functions: make([]Function, 0),
		Classes:   make([]Class, 0),
		Imports:   make([]Import, 0),
	}

	// Extract functions and classes based on language
	switch lang {
	case LanguageGo:
		p.extractGoFunctions(tree.RootNode(), []byte(content), parsed)
	case LanguagePython:
		p.extractPythonFunctions(tree.RootNode(), []byte(content), parsed)
	case LanguageJavaScript, LanguageTypeScript:
		p.extractJSFunctions(tree.RootNode(), []byte(content), parsed)
	}

	return parsed, nil
}

// extractGoFunctions extracts functions from Go source
func (p *Parser) extractGoFunctions(node *sitter.Node, source []byte, parsed *ParsedFile) {
	cursor := sitter.NewTreeCursor(node)
	defer cursor.Close()

	p.walkTree(cursor, source, func(n *sitter.Node) {
		switch n.Type() {
		case "function_declaration":
			fn := p.parseGoFunction(n, source)
			if fn != nil {
				fn.ID = fmt.Sprintf("%s:%d:%s", parsed.Path, fn.StartLine, fn.Name)
				parsed.Functions = append(parsed.Functions, *fn)
			}
		case "method_declaration":
			fn := p.parseGoMethod(n, source)
			if fn != nil {
				fn.ID = fmt.Sprintf("%s:%d:%s", parsed.Path, fn.StartLine, fn.Name)
				parsed.Functions = append(parsed.Functions, *fn)
			}
		}
	})
}

func (p *Parser) parseGoFunction(node *sitter.Node, source []byte) *Function {
	fn := &Function{
		StartLine:  int(node.StartPoint().Row) + 1,
		EndLine:    int(node.EndPoint().Row) + 1,
		Parameters: make([]Parameter, 0),
	}

	// Extract function name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			fn.Name = child.Content(source)
			fn.Exported = strings.ToUpper(fn.Name[:1]) == fn.Name[:1]
		} else if child.Type() == "parameter_list" {
			fn.Parameters = p.parseGoParameters(child, source)
		} else if child.Type() == "block" {
			fn.Body = child.Content(source)
		}
	}

	return fn
}

func (p *Parser) parseGoMethod(node *sitter.Node, source []byte) *Function {
	fn := p.parseGoFunction(node, source)
	if fn == nil {
		return nil
	}

	// Extract receiver type
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "parameter_list" && i == 0 {
			// This is the receiver
			for j := 0; j < int(child.ChildCount()); j++ {
				param := child.Child(j)
				if param.Type() == "parameter_declaration" {
					typeNode := param.ChildByFieldName("type")
					if typeNode != nil {
						fn.Class = strings.TrimPrefix(typeNode.Content(source), "*")
					}
				}
			}
		}
	}

	return fn
}

func (p *Parser) parseGoParameters(node *sitter.Node, source []byte) []Parameter {
	params := make([]Parameter, 0)

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "parameter_declaration" {
			var param Parameter
			nameNode := child.ChildByFieldName("name")
			typeNode := child.ChildByFieldName("type")

			if nameNode != nil {
				param.Name = nameNode.Content(source)
			}
			if typeNode != nil {
				param.Type = typeNode.Content(source)
			}

			if param.Name != "" {
				params = append(params, param)
			}
		}
	}

	return params
}

// extractPythonFunctions extracts functions from Python source
func (p *Parser) extractPythonFunctions(node *sitter.Node, source []byte, parsed *ParsedFile) {
	cursor := sitter.NewTreeCursor(node)
	defer cursor.Close()

	p.walkTree(cursor, source, func(n *sitter.Node) {
		if n.Type() == "function_definition" {
			fn := p.parsePythonFunction(n, source)
			if fn != nil {
				fn.ID = fmt.Sprintf("%s:%d:%s", parsed.Path, fn.StartLine, fn.Name)
				parsed.Functions = append(parsed.Functions, *fn)
			}
		} else if n.Type() == "class_definition" {
			cls := p.parsePythonClass(n, source, parsed.Path)
			if cls != nil {
				parsed.Classes = append(parsed.Classes, *cls)
			}
		}
	})
}

func (p *Parser) parsePythonFunction(node *sitter.Node, source []byte) *Function {
	fn := &Function{
		StartLine:  int(node.StartPoint().Row) + 1,
		EndLine:    int(node.EndPoint().Row) + 1,
		Parameters: make([]Parameter, 0),
	}

	// Get function name
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		fn.Name = nameNode.Content(source)
		fn.Exported = !strings.HasPrefix(fn.Name, "_")
	}

	// Get parameters
	paramsNode := node.ChildByFieldName("parameters")
	if paramsNode != nil {
		fn.Parameters = p.parsePythonParameters(paramsNode, source)
	}

	// Check if async
	for i := 0; i < int(node.ChildCount()); i++ {
		if node.Child(i).Type() == "async" {
			fn.Async = true
			break
		}
	}

	return fn
}

func (p *Parser) parsePythonParameters(node *sitter.Node, source []byte) []Parameter {
	params := make([]Parameter, 0)

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			name := child.Content(source)
			if name != "self" && name != "cls" {
				params = append(params, Parameter{Name: name})
			}
		} else if child.Type() == "typed_parameter" || child.Type() == "default_parameter" {
			var param Parameter
			for j := 0; j < int(child.ChildCount()); j++ {
				subChild := child.Child(j)
				if subChild.Type() == "identifier" {
					param.Name = subChild.Content(source)
				} else if subChild.Type() == "type" {
					param.Type = subChild.Content(source)
				}
			}
			if param.Name != "" && param.Name != "self" && param.Name != "cls" {
				params = append(params, param)
			}
		}
	}

	return params
}

func (p *Parser) parsePythonClass(node *sitter.Node, source []byte, filePath string) *Class {
	cls := &Class{
		StartLine: int(node.StartPoint().Row) + 1,
		EndLine:   int(node.EndPoint().Row) + 1,
		Methods:   make([]Function, 0),
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		cls.Name = nameNode.Content(source)
		cls.Exported = !strings.HasPrefix(cls.Name, "_")
		cls.ID = fmt.Sprintf("%s:%d:%s", filePath, cls.StartLine, cls.Name)
	}

	// Extract methods from class body
	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		for i := 0; i < int(bodyNode.ChildCount()); i++ {
			child := bodyNode.Child(i)
			if child.Type() == "function_definition" {
				fn := p.parsePythonFunction(child, source)
				if fn != nil {
					fn.Class = cls.Name
					fn.ID = fmt.Sprintf("%s:%d:%s.%s", filePath, fn.StartLine, cls.Name, fn.Name)
					cls.Methods = append(cls.Methods, *fn)
				}
			}
		}
	}

	return cls
}

// extractJSFunctions extracts functions from JavaScript/TypeScript source
func (p *Parser) extractJSFunctions(node *sitter.Node, source []byte, parsed *ParsedFile) {
	cursor := sitter.NewTreeCursor(node)
	defer cursor.Close()

	p.walkTree(cursor, source, func(n *sitter.Node) {
		switch n.Type() {
		case "function_declaration":
			fn := p.parseJSFunction(n, source)
			if fn != nil {
				fn.ID = fmt.Sprintf("%s:%d:%s", parsed.Path, fn.StartLine, fn.Name)
				parsed.Functions = append(parsed.Functions, *fn)
			}
		case "arrow_function", "function":
			// These might be assigned to variables
			parent := n.Parent()
			if parent != nil && parent.Type() == "variable_declarator" {
				fn := p.parseJSArrowFunction(n, parent, source)
				if fn != nil {
					fn.ID = fmt.Sprintf("%s:%d:%s", parsed.Path, fn.StartLine, fn.Name)
					parsed.Functions = append(parsed.Functions, *fn)
				}
			}
		case "method_definition":
			fn := p.parseJSMethod(n, source)
			if fn != nil {
				fn.ID = fmt.Sprintf("%s:%d:%s", parsed.Path, fn.StartLine, fn.Name)
				parsed.Functions = append(parsed.Functions, *fn)
			}
		}
	})
}

func (p *Parser) parseJSFunction(node *sitter.Node, source []byte) *Function {
	fn := &Function{
		StartLine:  int(node.StartPoint().Row) + 1,
		EndLine:    int(node.EndPoint().Row) + 1,
		Parameters: make([]Parameter, 0),
		Exported:   true, // Check export status separately
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		fn.Name = nameNode.Content(source)
	}

	paramsNode := node.ChildByFieldName("parameters")
	if paramsNode != nil {
		fn.Parameters = p.parseJSParameters(paramsNode, source)
	}

	return fn
}

func (p *Parser) parseJSArrowFunction(node, parent *sitter.Node, source []byte) *Function {
	fn := &Function{
		StartLine:  int(node.StartPoint().Row) + 1,
		EndLine:    int(node.EndPoint().Row) + 1,
		Parameters: make([]Parameter, 0),
	}

	// Get name from parent variable declarator
	nameNode := parent.ChildByFieldName("name")
	if nameNode != nil {
		fn.Name = nameNode.Content(source)
	}

	// Get parameters
	paramsNode := node.ChildByFieldName("parameters")
	if paramsNode != nil {
		fn.Parameters = p.parseJSParameters(paramsNode, source)
	}

	return fn
}

func (p *Parser) parseJSMethod(node *sitter.Node, source []byte) *Function {
	fn := &Function{
		StartLine:  int(node.StartPoint().Row) + 1,
		EndLine:    int(node.EndPoint().Row) + 1,
		Parameters: make([]Parameter, 0),
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		fn.Name = nameNode.Content(source)
	}

	paramsNode := node.ChildByFieldName("parameters")
	if paramsNode != nil {
		fn.Parameters = p.parseJSParameters(paramsNode, source)
	}

	return fn
}

func (p *Parser) parseJSParameters(node *sitter.Node, source []byte) []Parameter {
	params := make([]Parameter, 0)

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			params = append(params, Parameter{Name: child.Content(source)})
		} else if child.Type() == "required_parameter" || child.Type() == "optional_parameter" {
			var param Parameter
			patternNode := child.ChildByFieldName("pattern")
			if patternNode != nil {
				param.Name = patternNode.Content(source)
			}
			typeNode := child.ChildByFieldName("type")
			if typeNode != nil {
				param.Type = typeNode.Content(source)
			}
			if param.Name != "" {
				params = append(params, param)
			}
		}
	}

	return params
}

// walkTree walks the tree and calls fn for each node
func (p *Parser) walkTree(cursor *sitter.TreeCursor, source []byte, fn func(*sitter.Node)) {
	for {
		fn(cursor.CurrentNode())

		if cursor.GoToFirstChild() {
			continue
		}

		for {
			if cursor.GoToNextSibling() {
				break
			}
			if !cursor.GoToParent() {
				return
			}
		}
	}
}

// DetectLanguage detects language from file extension
func DetectLanguage(path string) Language {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return LanguageGo
	case ".py":
		return LanguagePython
	case ".js", ".jsx", ".mjs":
		return LanguageJavaScript
	case ".ts", ".tsx":
		return LanguageTypeScript
	case ".java":
		return LanguageJava
	default:
		return LanguageUnknown
	}
}
