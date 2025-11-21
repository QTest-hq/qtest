package emitter

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QTest-hq/qtest/pkg/model"
)

// CypressEmitter generates Cypress tests for E2E testing
type CypressEmitter struct{}

func (e *CypressEmitter) Name() string         { return "cypress" }
func (e *CypressEmitter) Language() string     { return "javascript" }
func (e *CypressEmitter) Framework() string    { return "cypress" }
func (e *CypressEmitter) FileExtension() string { return ".cy.js" }

// Emit generates a complete Cypress test file from multiple specs
func (e *CypressEmitter) Emit(specs []model.TestSpec) (string, error) {
	var sb strings.Builder

	// File header
	sb.WriteString(`/// <reference types="cypress" />

`)

	// Group specs by target for describe blocks
	groups := e.groupByTarget(specs)

	for groupName, groupSpecs := range groups {
		sb.WriteString(fmt.Sprintf("describe('%s', () => {\n", groupName))

		for _, spec := range groupSpecs {
			testCode, err := e.emitTest(spec)
			if err != nil {
				continue
			}
			sb.WriteString(testCode)
			sb.WriteString("\n")
		}

		sb.WriteString("});\n\n")
	}

	return sb.String(), nil
}

// EmitSingle generates test code for a single spec
func (e *CypressEmitter) EmitSingle(spec model.TestSpec) (string, error) {
	return e.emitTest(spec)
}

func (e *CypressEmitter) emitTest(spec model.TestSpec) (string, error) {
	var sb strings.Builder

	// Test function
	testName := e.generateTestName(spec)
	sb.WriteString(fmt.Sprintf("  it('%s', () => {\n", testName))

	// Handle E2E actions from spec inputs
	if spec.Level == model.LevelE2E {
		e.emitE2EActions(&sb, spec)
	} else {
		// Fallback for non-E2E specs - treat as page navigation test
		if spec.Path != "" {
			sb.WriteString(fmt.Sprintf("    cy.visit('%s');\n", spec.Path))
		}
	}

	// Add assertions
	for _, assertion := range spec.Assertions {
		sb.WriteString(e.emitAssertion(assertion))
	}

	sb.WriteString("  });\n")

	return sb.String(), nil
}

// emitE2EActions emits E2E-specific actions
func (e *CypressEmitter) emitE2EActions(sb *strings.Builder, spec model.TestSpec) {
	// Check for URL/navigation
	if spec.Path != "" {
		sb.WriteString(fmt.Sprintf("    cy.visit('%s');\n", spec.Path))
	}

	// Process actions from inputs
	if spec.Inputs != nil {
		// Navigate action
		if url, ok := spec.Inputs["url"].(string); ok {
			sb.WriteString(fmt.Sprintf("    cy.visit('%s');\n", url))
		}

		// Click action
		if selector, ok := spec.Inputs["click"].(string); ok {
			sb.WriteString(fmt.Sprintf("    cy.get('%s').click();\n", e.formatSelector(selector)))
		}

		// Fill/type action
		if fills, ok := spec.Inputs["fill"].(map[string]interface{}); ok {
			for selector, value := range fills {
				sb.WriteString(fmt.Sprintf("    cy.get('%s').clear().type('%v');\n", e.formatSelector(selector), value))
			}
		}

		// Type action
		if typeData, ok := spec.Inputs["type"].(map[string]interface{}); ok {
			if selector, ok := typeData["selector"].(string); ok {
				if text, ok := typeData["text"].(string); ok {
					sb.WriteString(fmt.Sprintf("    cy.get('%s').type('%s');\n", e.formatSelector(selector), text))
				}
			}
		}

		// Wait action
		if waitFor, ok := spec.Inputs["wait"].(string); ok {
			sb.WriteString(fmt.Sprintf("    cy.get('%s').should('exist');\n", e.formatSelector(waitFor)))
		}
		if waitMs, ok := spec.Inputs["waitMs"].(float64); ok {
			sb.WriteString(fmt.Sprintf("    cy.wait(%d);\n", int(waitMs)))
		}

		// Screenshot action
		if screenshot, ok := spec.Inputs["screenshot"].(string); ok {
			sb.WriteString(fmt.Sprintf("    cy.screenshot('%s');\n", strings.TrimSuffix(screenshot, ".png")))
		}

		// Intercept API calls
		if intercepts, ok := spec.Inputs["intercept"].([]interface{}); ok {
			for _, intercept := range intercepts {
				if interceptMap, ok := intercept.(map[string]interface{}); ok {
					e.emitIntercept(sb, interceptMap)
				}
			}
		}

		// Custom steps
		if steps, ok := spec.Inputs["steps"].([]interface{}); ok {
			for _, step := range steps {
				if stepMap, ok := step.(map[string]interface{}); ok {
					e.emitStep(sb, stepMap)
				}
			}
		}
	}
}

// emitIntercept emits a cy.intercept call for API mocking
func (e *CypressEmitter) emitIntercept(sb *strings.Builder, intercept map[string]interface{}) {
	method, _ := intercept["method"].(string)
	url, _ := intercept["url"].(string)
	alias, _ := intercept["alias"].(string)
	response := intercept["response"]

	if method == "" {
		method = "GET"
	}

	if response != nil {
		respJSON, _ := json.Marshal(response)
		sb.WriteString(fmt.Sprintf("    cy.intercept('%s', '%s', %s)", method, url, string(respJSON)))
	} else {
		sb.WriteString(fmt.Sprintf("    cy.intercept('%s', '%s')", method, url))
	}

	if alias != "" {
		sb.WriteString(fmt.Sprintf(".as('%s')", alias))
	}
	sb.WriteString(";\n")
}

// emitStep emits a single E2E step
func (e *CypressEmitter) emitStep(sb *strings.Builder, step map[string]interface{}) {
	action, _ := step["action"].(string)
	selector, _ := step["selector"].(string)
	value, _ := step["value"].(string)

	switch action {
	case "navigate", "goto", "visit":
		if url, ok := step["url"].(string); ok {
			sb.WriteString(fmt.Sprintf("    cy.visit('%s');\n", url))
		}
	case "click":
		sb.WriteString(fmt.Sprintf("    cy.get('%s').click();\n", e.formatSelector(selector)))
	case "dblclick":
		sb.WriteString(fmt.Sprintf("    cy.get('%s').dblclick();\n", e.formatSelector(selector)))
	case "rightclick":
		sb.WriteString(fmt.Sprintf("    cy.get('%s').rightclick();\n", e.formatSelector(selector)))
	case "fill", "type":
		if clear, ok := step["clear"].(bool); ok && clear {
			sb.WriteString(fmt.Sprintf("    cy.get('%s').clear().type('%s');\n", e.formatSelector(selector), value))
		} else {
			sb.WriteString(fmt.Sprintf("    cy.get('%s').type('%s');\n", e.formatSelector(selector), value))
		}
	case "clear":
		sb.WriteString(fmt.Sprintf("    cy.get('%s').clear();\n", e.formatSelector(selector)))
	case "select":
		sb.WriteString(fmt.Sprintf("    cy.get('%s').select('%s');\n", e.formatSelector(selector), value))
	case "check":
		sb.WriteString(fmt.Sprintf("    cy.get('%s').check();\n", e.formatSelector(selector)))
	case "uncheck":
		sb.WriteString(fmt.Sprintf("    cy.get('%s').uncheck();\n", e.formatSelector(selector)))
	case "focus":
		sb.WriteString(fmt.Sprintf("    cy.get('%s').focus();\n", e.formatSelector(selector)))
	case "blur":
		sb.WriteString(fmt.Sprintf("    cy.get('%s').blur();\n", e.formatSelector(selector)))
	case "hover", "trigger:mouseover":
		sb.WriteString(fmt.Sprintf("    cy.get('%s').trigger('mouseover');\n", e.formatSelector(selector)))
	case "scroll", "scrollIntoView":
		sb.WriteString(fmt.Sprintf("    cy.get('%s').scrollIntoView();\n", e.formatSelector(selector)))
	case "scrollTo":
		position := value
		if position == "" {
			position = "bottom"
		}
		sb.WriteString(fmt.Sprintf("    cy.scrollTo('%s');\n", position))
	case "wait":
		if timeout, ok := step["timeout"].(float64); ok {
			sb.WriteString(fmt.Sprintf("    cy.wait(%d);\n", int(timeout)))
		} else if alias, ok := step["alias"].(string); ok {
			sb.WriteString(fmt.Sprintf("    cy.wait('@%s');\n", alias))
		} else if selector != "" {
			sb.WriteString(fmt.Sprintf("    cy.get('%s').should('exist');\n", e.formatSelector(selector)))
		}
	case "screenshot":
		name := value
		if name == "" {
			name = "screenshot"
		}
		sb.WriteString(fmt.Sprintf("    cy.screenshot('%s');\n", name))
	case "submit":
		sb.WriteString(fmt.Sprintf("    cy.get('%s').submit();\n", e.formatSelector(selector)))
	}
}

func (e *CypressEmitter) emitAssertion(a model.Assertion) string {
	switch a.Kind {
	case "visible":
		return fmt.Sprintf("    cy.get('%s').should('be.visible');\n", e.formatSelector(a.Actual))

	case "hidden":
		return fmt.Sprintf("    cy.get('%s').should('not.be.visible');\n", e.formatSelector(a.Actual))

	case "text", "equality":
		if a.Actual == "title" {
			return fmt.Sprintf("    cy.title().should('eq', '%v');\n", a.Expected)
		}
		if a.Actual == "url" {
			return fmt.Sprintf("    cy.url().should('eq', '%v');\n", a.Expected)
		}
		return fmt.Sprintf("    cy.get('%s').should('have.text', '%v');\n", e.formatSelector(a.Actual), a.Expected)

	case "contains":
		if a.Actual == "url" {
			return fmt.Sprintf("    cy.url().should('include', '%v');\n", a.Expected)
		}
		if a.Actual == "body" {
			return fmt.Sprintf("    cy.contains('%v').should('exist');\n", a.Expected)
		}
		return fmt.Sprintf("    cy.get('%s').should('contain', '%v');\n", e.formatSelector(a.Actual), a.Expected)

	case "value":
		return fmt.Sprintf("    cy.get('%s').should('have.value', '%v');\n", e.formatSelector(a.Actual), a.Expected)

	case "count", "length":
		return fmt.Sprintf("    cy.get('%s').should('have.length', %v);\n", e.formatSelector(a.Actual), a.Expected)

	case "enabled":
		return fmt.Sprintf("    cy.get('%s').should('be.enabled');\n", e.formatSelector(a.Actual))

	case "disabled":
		return fmt.Sprintf("    cy.get('%s').should('be.disabled');\n", e.formatSelector(a.Actual))

	case "checked":
		return fmt.Sprintf("    cy.get('%s').should('be.checked');\n", e.formatSelector(a.Actual))

	case "unchecked":
		return fmt.Sprintf("    cy.get('%s').should('not.be.checked');\n", e.formatSelector(a.Actual))

	case "not_null", "exists":
		return fmt.Sprintf("    cy.get('%s').should('exist');\n", e.formatSelector(a.Actual))

	case "not_exists":
		return fmt.Sprintf("    cy.get('%s').should('not.exist');\n", e.formatSelector(a.Actual))

	case "have_class":
		return fmt.Sprintf("    cy.get('%s').should('have.class', '%v');\n", e.formatSelector(a.Actual), a.Expected)

	case "have_attr":
		if attrMap, ok := a.Expected.(map[string]interface{}); ok {
			for attr, val := range attrMap {
				return fmt.Sprintf("    cy.get('%s').should('have.attr', '%s', '%v');\n", e.formatSelector(a.Actual), attr, val)
			}
		}
		return fmt.Sprintf("    cy.get('%s').should('have.attr', '%v');\n", e.formatSelector(a.Actual), a.Expected)

	case "status_code":
		return fmt.Sprintf("    // Note: Use cy.intercept() to check response status codes\n")

	default:
		return fmt.Sprintf("    // Unknown assertion kind: %s\n", a.Kind)
	}
}

// formatSelector converts a selector string to Cypress format
func (e *CypressEmitter) formatSelector(selector string) string {
	// Already formatted selectors
	if strings.HasPrefix(selector, "[") || strings.HasPrefix(selector, ".") ||
		strings.HasPrefix(selector, "#") {
		return selector
	}

	// data-testid shorthand: @testid -> [data-testid="testid"]
	if strings.HasPrefix(selector, "@") {
		return fmt.Sprintf("[data-testid=\"%s\"]", strings.TrimPrefix(selector, "@"))
	}

	// data-cy shorthand: $testid -> [data-cy="testid"]
	if strings.HasPrefix(selector, "$") {
		return fmt.Sprintf("[data-cy=\"%s\"]", strings.TrimPrefix(selector, "$"))
	}

	return selector
}

func (e *CypressEmitter) generateTestName(spec model.TestSpec) string {
	if spec.Description != "" {
		return spec.Description
	}
	return "should work correctly"
}

func (e *CypressEmitter) groupByTarget(specs []model.TestSpec) map[string][]model.TestSpec {
	groups := make(map[string][]model.TestSpec)

	for _, spec := range specs {
		groupName := "E2E Tests"
		if spec.Path != "" {
			// Extract page/route name
			parts := strings.Split(strings.TrimPrefix(spec.Path, "/"), "/")
			if len(parts) > 0 && parts[0] != "" {
				groupName = parts[0] + " page"
			}
		}
		if spec.TargetID != "" {
			groupName = spec.TargetID
		}

		groups[groupName] = append(groups[groupName], spec)
	}

	return groups
}
