package parser

import (
	sitter "github.com/smacker/go-tree-sitter"
)

// ExtractBranches extracts all branch statements from a function body node
func ExtractBranches(node *sitter.Node, lang Language, source []byte) []Branch {
	if node == nil {
		return nil
	}

	var branches []Branch

	switch lang {
	case LanguageGo:
		branches = extractGoBranches(node, source)
	case LanguageJavaScript, LanguageTypeScript:
		branches = extractJSBranches(node, source)
	case LanguagePython:
		branches = extractPythonBranches(node, source)
	case LanguageJava:
		branches = extractJavaBranches(node, source)
	}

	return branches
}

// extractGoBranches extracts branches from Go code
func extractGoBranches(node *sitter.Node, source []byte) []Branch {
	var branches []Branch
	walkTree(node, func(n *sitter.Node) bool {
		branch := parseGoBranchNode(n, source)
		if branch != nil {
			branches = append(branches, *branch)
		}
		return true
	})
	return branches
}

// parseGoBranchNode parses a Go node into a Branch if applicable
func parseGoBranchNode(node *sitter.Node, source []byte) *Branch {
	nodeType := node.Type()

	switch nodeType {
	case "if_statement":
		condition := ""
		if condNode := node.ChildByFieldName("condition"); condNode != nil {
			condition = condNode.Content(source)
		}
		return &Branch{
			Type:      BranchIf,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: condition,
			IsLoop:    false,
		}

	case "expression_switch_statement", "type_switch_statement":
		condition := ""
		if valueNode := node.ChildByFieldName("value"); valueNode != nil {
			condition = valueNode.Content(source)
		}
		return &Branch{
			Type:      BranchSwitch,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: condition,
			IsLoop:    false,
		}

	case "for_statement":
		condition := ""
		// Go for loops can have various forms
		if condNode := node.ChildByFieldName("condition"); condNode != nil {
			condition = condNode.Content(source)
		}
		return &Branch{
			Type:      BranchFor,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: condition,
			IsLoop:    true,
		}

	case "select_statement":
		return &Branch{
			Type:      BranchSwitch, // select is similar to switch
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: "select",
			IsLoop:    false,
		}
	}

	return nil
}

// extractJSBranches extracts branches from JavaScript/TypeScript code
func extractJSBranches(node *sitter.Node, source []byte) []Branch {
	var branches []Branch
	walkTree(node, func(n *sitter.Node) bool {
		branch := parseJSBranchNode(n, source)
		if branch != nil {
			branches = append(branches, *branch)
		}
		return true
	})
	return branches
}

// parseJSBranchNode parses a JS/TS node into a Branch if applicable
func parseJSBranchNode(node *sitter.Node, source []byte) *Branch {
	nodeType := node.Type()

	switch nodeType {
	case "if_statement":
		condition := ""
		if condNode := node.ChildByFieldName("condition"); condNode != nil {
			condition = condNode.Content(source)
		}
		return &Branch{
			Type:      BranchIf,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: condition,
			IsLoop:    false,
		}

	case "switch_statement":
		condition := ""
		if valueNode := node.ChildByFieldName("value"); valueNode != nil {
			condition = valueNode.Content(source)
		}
		return &Branch{
			Type:      BranchSwitch,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: condition,
			IsLoop:    false,
		}

	case "try_statement":
		return &Branch{
			Type:      BranchTry,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: "",
			IsLoop:    false,
		}

	case "catch_clause":
		parameter := ""
		if paramNode := node.ChildByFieldName("parameter"); paramNode != nil {
			parameter = paramNode.Content(source)
		}
		return &Branch{
			Type:      BranchCatch,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: parameter,
			IsLoop:    false,
		}

	case "for_statement", "for_in_statement":
		condition := ""
		// Try to get the full for header
		if node.ChildCount() > 0 {
			// Get the part between "for" and the body
			for i := 0; i < int(node.ChildCount()); i++ {
				child := node.Child(i)
				if child.Type() == "parenthesized_expression" {
					condition = child.Content(source)
					break
				}
			}
		}
		return &Branch{
			Type:      BranchFor,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: condition,
			IsLoop:    true,
		}

	case "while_statement":
		condition := ""
		if condNode := node.ChildByFieldName("condition"); condNode != nil {
			condition = condNode.Content(source)
		}
		return &Branch{
			Type:      BranchWhile,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: condition,
			IsLoop:    true,
		}

	case "do_statement":
		condition := ""
		if condNode := node.ChildByFieldName("condition"); condNode != nil {
			condition = condNode.Content(source)
		}
		return &Branch{
			Type:      BranchDoWhile,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: condition,
			IsLoop:    true,
		}

	case "ternary_expression":
		condition := ""
		if condNode := node.ChildByFieldName("condition"); condNode != nil {
			condition = condNode.Content(source)
		}
		return &Branch{
			Type:      BranchIf, // Ternary is essentially an if
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: condition,
			IsLoop:    false,
		}
	}

	return nil
}

// extractPythonBranches extracts branches from Python code
func extractPythonBranches(node *sitter.Node, source []byte) []Branch {
	var branches []Branch
	walkTree(node, func(n *sitter.Node) bool {
		branch := parsePythonBranchNode(n, source)
		if branch != nil {
			branches = append(branches, *branch)
		}
		return true
	})
	return branches
}

// parsePythonBranchNode parses a Python node into a Branch if applicable
func parsePythonBranchNode(node *sitter.Node, source []byte) *Branch {
	nodeType := node.Type()

	switch nodeType {
	case "if_statement":
		condition := ""
		if condNode := node.ChildByFieldName("condition"); condNode != nil {
			condition = condNode.Content(source)
		}
		return &Branch{
			Type:      BranchIf,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: condition,
			IsLoop:    false,
		}

	case "elif_clause":
		condition := ""
		if condNode := node.ChildByFieldName("condition"); condNode != nil {
			condition = condNode.Content(source)
		}
		return &Branch{
			Type:      BranchElseIf,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: condition,
			IsLoop:    false,
		}

	case "try_statement":
		return &Branch{
			Type:      BranchTry,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: "",
			IsLoop:    false,
		}

	case "except_clause":
		exceptionType := ""
		// Get the exception type if specified
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "identifier" || child.Type() == "tuple" {
				exceptionType = child.Content(source)
				break
			}
		}
		return &Branch{
			Type:      BranchCatch,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: exceptionType,
			IsLoop:    false,
		}

	case "for_statement":
		// Python for: "for item in iterable"
		left := ""
		right := ""
		if leftNode := node.ChildByFieldName("left"); leftNode != nil {
			left = leftNode.Content(source)
		}
		if rightNode := node.ChildByFieldName("right"); rightNode != nil {
			right = rightNode.Content(source)
		}
		condition := left + " in " + right
		return &Branch{
			Type:      BranchFor,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: condition,
			IsLoop:    true,
		}

	case "while_statement":
		condition := ""
		if condNode := node.ChildByFieldName("condition"); condNode != nil {
			condition = condNode.Content(source)
		}
		return &Branch{
			Type:      BranchWhile,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: condition,
			IsLoop:    true,
		}

	case "match_statement":
		subject := ""
		if subjectNode := node.ChildByFieldName("subject"); subjectNode != nil {
			subject = subjectNode.Content(source)
		}
		return &Branch{
			Type:      BranchSwitch, // match is Python's switch
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: subject,
			IsLoop:    false,
		}

	case "conditional_expression":
		// Python ternary: value if condition else other
		condition := ""
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "if" {
				// Next sibling is the condition
				if i+1 < int(node.ChildCount()) {
					condition = node.Child(i + 1).Content(source)
				}
				break
			}
		}
		return &Branch{
			Type:      BranchIf,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: condition,
			IsLoop:    false,
		}
	}

	return nil
}

// extractJavaBranches extracts branches from Java code
func extractJavaBranches(node *sitter.Node, source []byte) []Branch {
	var branches []Branch
	walkTree(node, func(n *sitter.Node) bool {
		branch := parseJavaBranchNode(n, source)
		if branch != nil {
			branches = append(branches, *branch)
		}
		return true
	})
	return branches
}

// parseJavaBranchNode parses a Java node into a Branch if applicable
func parseJavaBranchNode(node *sitter.Node, source []byte) *Branch {
	nodeType := node.Type()

	switch nodeType {
	case "if_statement":
		condition := ""
		if condNode := node.ChildByFieldName("condition"); condNode != nil {
			condition = condNode.Content(source)
		}
		return &Branch{
			Type:      BranchIf,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: condition,
			IsLoop:    false,
		}

	case "switch_expression", "switch_statement":
		condition := ""
		if condNode := node.ChildByFieldName("condition"); condNode != nil {
			condition = condNode.Content(source)
		}
		return &Branch{
			Type:      BranchSwitch,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: condition,
			IsLoop:    false,
		}

	case "try_statement":
		return &Branch{
			Type:      BranchTry,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: "",
			IsLoop:    false,
		}

	case "catch_clause":
		parameter := ""
		if paramNode := node.ChildByFieldName("parameter"); paramNode != nil {
			parameter = paramNode.Content(source)
		}
		return &Branch{
			Type:      BranchCatch,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: parameter,
			IsLoop:    false,
		}

	case "for_statement":
		condition := ""
		if condNode := node.ChildByFieldName("condition"); condNode != nil {
			condition = condNode.Content(source)
		}
		return &Branch{
			Type:      BranchFor,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: condition,
			IsLoop:    true,
		}

	case "enhanced_for_statement":
		// Java for-each: "for (Type item : collection)"
		varType := ""
		varName := ""
		collection := ""
		if typeNode := node.ChildByFieldName("type"); typeNode != nil {
			varType = typeNode.Content(source)
		}
		if nameNode := node.ChildByFieldName("name"); nameNode != nil {
			varName = nameNode.Content(source)
		}
		if valueNode := node.ChildByFieldName("value"); valueNode != nil {
			collection = valueNode.Content(source)
		}
		condition := varType + " " + varName + " : " + collection
		return &Branch{
			Type:      BranchFor,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: condition,
			IsLoop:    true,
		}

	case "while_statement":
		condition := ""
		if condNode := node.ChildByFieldName("condition"); condNode != nil {
			condition = condNode.Content(source)
		}
		return &Branch{
			Type:      BranchWhile,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: condition,
			IsLoop:    true,
		}

	case "do_statement":
		condition := ""
		if condNode := node.ChildByFieldName("condition"); condNode != nil {
			condition = condNode.Content(source)
		}
		return &Branch{
			Type:      BranchDoWhile,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: condition,
			IsLoop:    true,
		}

	case "ternary_expression":
		condition := ""
		if condNode := node.ChildByFieldName("condition"); condNode != nil {
			condition = condNode.Content(source)
		}
		return &Branch{
			Type:      BranchIf,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Condition: condition,
			IsLoop:    false,
		}
	}

	return nil
}

// walkTree walks the AST tree and calls the callback for each node
func walkTree(node *sitter.Node, callback func(*sitter.Node) bool) {
	if node == nil {
		return
	}

	if !callback(node) {
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		walkTree(node.Child(i), callback)
	}
}

// ComputeCyclomaticComplexity calculates cyclomatic complexity from branches
// Cyclomatic complexity = number of decision points + 1
func ComputeCyclomaticComplexity(branches []Branch) int {
	if len(branches) == 0 {
		return 1
	}

	// Count decision points (excluding try blocks and loops that don't add paths)
	decisionPoints := 0
	for _, b := range branches {
		switch b.Type {
		case BranchIf, BranchElseIf, BranchSwitch, BranchCase, BranchCatch:
			decisionPoints++
		case BranchFor, BranchWhile, BranchDoWhile:
			decisionPoints++ // Loops add one path
		case BranchTry:
			// Try itself doesn't add complexity, catch clauses do
		}
	}

	return decisionPoints + 1
}
