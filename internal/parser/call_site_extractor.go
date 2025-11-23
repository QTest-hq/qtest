package parser

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// ExtractCallSites extracts all function calls from a function body node
func ExtractCallSites(node *sitter.Node, lang Language, source []byte) []CallSite {
	if node == nil {
		return nil
	}

	var callSites []CallSite

	switch lang {
	case LanguageGo:
		callSites = extractGoCallSites(node, source)
	case LanguageJavaScript, LanguageTypeScript:
		callSites = extractJSCallSites(node, source)
	case LanguagePython:
		callSites = extractPythonCallSites(node, source)
	case LanguageJava:
		callSites = extractJavaCallSites(node, source)
	}

	return callSites
}

// extractGoCallSites extracts call sites from Go code
func extractGoCallSites(node *sitter.Node, source []byte) []CallSite {
	var callSites []CallSite
	walkTree(node, func(n *sitter.Node) bool {
		if n.Type() == "call_expression" {
			cs := parseGoCallExpression(n, source)
			if cs != nil {
				callSites = append(callSites, *cs)
			}
		}
		return true
	})
	return callSites
}

// parseGoCallExpression parses a Go call_expression into a CallSite
func parseGoCallExpression(node *sitter.Node, source []byte) *CallSite {
	funcNode := node.ChildByFieldName("function")
	if funcNode == nil {
		return nil
	}

	cs := &CallSite{
		Line: int(node.StartPoint().Row) + 1,
	}

	// Count arguments
	if argsNode := node.ChildByFieldName("arguments"); argsNode != nil {
		cs.Arguments = countArguments(argsNode)
	}

	funcContent := funcNode.Content(source)

	// Check if it's a method call (has selector)
	if funcNode.Type() == "selector_expression" {
		operandNode := funcNode.ChildByFieldName("operand")
		fieldNode := funcNode.ChildByFieldName("field")
		if operandNode != nil && fieldNode != nil {
			cs.Receiver = operandNode.Content(source)
			cs.FunctionName = fieldNode.Content(source)
			cs.IsMethod = true

			// Check if receiver is a package name (lowercase, no dots)
			receiver := cs.Receiver
			if !strings.Contains(receiver, ".") && len(receiver) > 0 {
				firstChar := receiver[0]
				if firstChar >= 'a' && firstChar <= 'z' {
					// Likely a package import
					cs.Module = receiver
					cs.IsMethod = false
				}
			}
		}
	} else if funcNode.Type() == "identifier" {
		// Direct function call
		cs.FunctionName = funcContent
		cs.IsMethod = false
	} else {
		// Complex expression (e.g., function returned from another call)
		cs.FunctionName = funcContent
	}

	return cs
}

// extractJSCallSites extracts call sites from JavaScript/TypeScript code
func extractJSCallSites(node *sitter.Node, source []byte) []CallSite {
	var callSites []CallSite
	walkTree(node, func(n *sitter.Node) bool {
		nodeType := n.Type()
		if nodeType == "call_expression" {
			cs := parseJSCallExpression(n, source)
			if cs != nil {
				callSites = append(callSites, *cs)
			}
		} else if nodeType == "await_expression" {
			// Check if awaiting a call
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child.Type() == "call_expression" {
					cs := parseJSCallExpression(child, source)
					if cs != nil {
						cs.IsAsync = true
						callSites = append(callSites, *cs)
					}
				}
			}
		}
		return true
	})
	return callSites
}

// parseJSCallExpression parses a JS call_expression into a CallSite
func parseJSCallExpression(node *sitter.Node, source []byte) *CallSite {
	funcNode := node.ChildByFieldName("function")
	if funcNode == nil {
		return nil
	}

	cs := &CallSite{
		Line: int(node.StartPoint().Row) + 1,
	}

	// Count arguments
	if argsNode := node.ChildByFieldName("arguments"); argsNode != nil {
		cs.Arguments = countArguments(argsNode)
	}

	funcContent := funcNode.Content(source)

	// Check if it's a method call
	if funcNode.Type() == "member_expression" {
		objectNode := funcNode.ChildByFieldName("object")
		propertyNode := funcNode.ChildByFieldName("property")
		if objectNode != nil && propertyNode != nil {
			cs.Receiver = objectNode.Content(source)
			cs.FunctionName = propertyNode.Content(source)
			cs.IsMethod = true

			// Check for module patterns (e.g., fs.readFile, path.join)
			if objectNode.Type() == "identifier" {
				cs.Module = cs.Receiver
			}
		}
	} else if funcNode.Type() == "identifier" {
		cs.FunctionName = funcContent
		cs.IsMethod = false
	} else {
		// Arrow function or other complex expression
		cs.FunctionName = funcContent
	}

	return cs
}

// extractPythonCallSites extracts call sites from Python code
func extractPythonCallSites(node *sitter.Node, source []byte) []CallSite {
	var callSites []CallSite
	walkTree(node, func(n *sitter.Node) bool {
		if n.Type() == "call" {
			cs := parsePythonCall(n, source)
			if cs != nil {
				callSites = append(callSites, *cs)
			}
		}
		return true
	})
	return callSites
}

// parsePythonCall parses a Python call into a CallSite
func parsePythonCall(node *sitter.Node, source []byte) *CallSite {
	funcNode := node.ChildByFieldName("function")
	if funcNode == nil {
		return nil
	}

	cs := &CallSite{
		Line: int(node.StartPoint().Row) + 1,
	}

	// Count arguments
	if argsNode := node.ChildByFieldName("arguments"); argsNode != nil {
		cs.Arguments = countPythonArguments(argsNode)
	}

	funcContent := funcNode.Content(source)

	// Check if it's a method call (attribute access)
	if funcNode.Type() == "attribute" {
		objectNode := funcNode.ChildByFieldName("object")
		attrNode := funcNode.ChildByFieldName("attribute")
		if objectNode != nil && attrNode != nil {
			cs.Receiver = objectNode.Content(source)
			cs.FunctionName = attrNode.Content(source)
			cs.IsMethod = true

			// Check for module patterns
			if objectNode.Type() == "identifier" {
				// Could be module or object - hard to tell without type info
				cs.Module = cs.Receiver
			}
		}
	} else if funcNode.Type() == "identifier" {
		cs.FunctionName = funcContent
		cs.IsMethod = false
	} else {
		cs.FunctionName = funcContent
	}

	// Check for async (await is parent)
	if node.Parent() != nil && node.Parent().Type() == "await" {
		cs.IsAsync = true
	}

	return cs
}

// extractJavaCallSites extracts call sites from Java code
func extractJavaCallSites(node *sitter.Node, source []byte) []CallSite {
	var callSites []CallSite
	walkTree(node, func(n *sitter.Node) bool {
		nodeType := n.Type()
		if nodeType == "method_invocation" {
			cs := parseJavaMethodInvocation(n, source)
			if cs != nil {
				callSites = append(callSites, *cs)
			}
		} else if nodeType == "object_creation_expression" {
			// new ClassName() - constructor call
			cs := parseJavaConstructorCall(n, source)
			if cs != nil {
				callSites = append(callSites, *cs)
			}
		}
		return true
	})
	return callSites
}

// parseJavaMethodInvocation parses a Java method_invocation into a CallSite
func parseJavaMethodInvocation(node *sitter.Node, source []byte) *CallSite {
	cs := &CallSite{
		Line:     int(node.StartPoint().Row) + 1,
		IsMethod: true,
	}

	// Get method name
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		cs.FunctionName = nameNode.Content(source)
	}

	// Get receiver object
	if objectNode := node.ChildByFieldName("object"); objectNode != nil {
		cs.Receiver = objectNode.Content(source)

		// Check if it's a class/static method
		if objectNode.Type() == "identifier" {
			firstChar := cs.Receiver[0]
			if firstChar >= 'A' && firstChar <= 'Z' {
				// Static method call (e.g., Math.abs)
				cs.Module = cs.Receiver
			}
		}
	} else {
		// No receiver - implicit this or static import
		cs.IsMethod = false
	}

	// Count arguments
	if argsNode := node.ChildByFieldName("arguments"); argsNode != nil {
		cs.Arguments = countArguments(argsNode)
	}

	return cs
}

// parseJavaConstructorCall parses a Java new expression into a CallSite
func parseJavaConstructorCall(node *sitter.Node, source []byte) *CallSite {
	cs := &CallSite{
		Line:     int(node.StartPoint().Row) + 1,
		IsMethod: false,
	}

	// Get type being constructed
	if typeNode := node.ChildByFieldName("type"); typeNode != nil {
		typeName := typeNode.Content(source)
		cs.FunctionName = "new " + typeName
		cs.Module = typeName
	}

	// Count arguments
	if argsNode := node.ChildByFieldName("arguments"); argsNode != nil {
		cs.Arguments = countArguments(argsNode)
	}

	return cs
}

// countArguments counts the number of arguments in an argument list node
func countArguments(argsNode *sitter.Node) int {
	count := 0
	for i := 0; i < int(argsNode.ChildCount()); i++ {
		child := argsNode.Child(i)
		childType := child.Type()
		// Skip punctuation (parens, commas)
		if childType != "(" && childType != ")" && childType != "," {
			count++
		}
	}
	return count
}

// countPythonArguments counts arguments in Python, handling kwargs
func countPythonArguments(argsNode *sitter.Node) int {
	count := 0
	for i := 0; i < int(argsNode.ChildCount()); i++ {
		child := argsNode.Child(i)
		childType := child.Type()
		// Count actual arguments
		if childType == "argument" || childType == "keyword_argument" ||
			childType == "expression" || childType == "identifier" ||
			childType == "string" || childType == "integer" ||
			childType == "float" || childType == "true" || childType == "false" ||
			childType == "none" || childType == "list" || childType == "dictionary" ||
			childType == "call" || childType == "attribute" {
			count++
		}
	}
	return count
}
