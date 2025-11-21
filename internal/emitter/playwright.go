package emitter

import (
	"fmt"
	"strings"

	"github.com/QTest-hq/qtest/pkg/model"
)

// PlaywrightEmitter generates Playwright tests for E2E testing
type PlaywrightEmitter struct{}

func (e *PlaywrightEmitter) Name() string          { return "playwright" }
func (e *PlaywrightEmitter) Language() string      { return "typescript" }
func (e *PlaywrightEmitter) Framework() string     { return "playwright" }
func (e *PlaywrightEmitter) FileExtension() string { return ".spec.ts" }

// Emit generates a complete Playwright test file from multiple specs
func (e *PlaywrightEmitter) Emit(specs []model.TestSpec) (string, error) {
	var sb strings.Builder

	// File header with imports
	sb.WriteString(`import { test, expect } from '@playwright/test';

`)

	// Group specs by target for describe blocks
	groups := e.groupByTarget(specs)

	for groupName, groupSpecs := range groups {
		sb.WriteString(fmt.Sprintf("test.describe('%s', () => {\n", groupName))

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
func (e *PlaywrightEmitter) EmitSingle(spec model.TestSpec) (string, error) {
	return e.emitTest(spec)
}

func (e *PlaywrightEmitter) emitTest(spec model.TestSpec) (string, error) {
	var sb strings.Builder

	// Test function
	testName := e.generateTestName(spec)
	sb.WriteString(fmt.Sprintf("  test('%s', async ({ page }) => {\n", testName))

	// Handle E2E actions from spec inputs
	if spec.Level == model.LevelE2E {
		e.emitE2EActions(&sb, spec)
	} else {
		// Fallback for non-E2E specs - treat as page navigation test
		if spec.Path != "" {
			sb.WriteString(fmt.Sprintf("    await page.goto('%s');\n", spec.Path))
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
func (e *PlaywrightEmitter) emitE2EActions(sb *strings.Builder, spec model.TestSpec) {
	// Check for URL/navigation
	if spec.Path != "" {
		sb.WriteString(fmt.Sprintf("    await page.goto('%s');\n", spec.Path))
	}

	// Process actions from inputs
	if spec.Inputs != nil {
		// Navigate action
		if url, ok := spec.Inputs["url"].(string); ok {
			sb.WriteString(fmt.Sprintf("    await page.goto('%s');\n", url))
		}

		// Click action
		if selector, ok := spec.Inputs["click"].(string); ok {
			sb.WriteString(fmt.Sprintf("    await page.click('%s');\n", e.formatSelector(selector)))
		}

		// Fill/type action
		if fills, ok := spec.Inputs["fill"].(map[string]interface{}); ok {
			for selector, value := range fills {
				sb.WriteString(fmt.Sprintf("    await page.fill('%s', '%v');\n", e.formatSelector(selector), value))
			}
		}

		// Type action (for keyboard input)
		if typeData, ok := spec.Inputs["type"].(map[string]interface{}); ok {
			if selector, ok := typeData["selector"].(string); ok {
				if text, ok := typeData["text"].(string); ok {
					sb.WriteString(fmt.Sprintf("    await page.type('%s', '%s');\n", e.formatSelector(selector), text))
				}
			}
		}

		// Wait action
		if waitFor, ok := spec.Inputs["wait"].(string); ok {
			sb.WriteString(fmt.Sprintf("    await page.waitForSelector('%s');\n", e.formatSelector(waitFor)))
		}
		if waitMs, ok := spec.Inputs["waitMs"].(float64); ok {
			sb.WriteString(fmt.Sprintf("    await page.waitForTimeout(%d);\n", int(waitMs)))
		}

		// Screenshot action
		if screenshot, ok := spec.Inputs["screenshot"].(string); ok {
			sb.WriteString(fmt.Sprintf("    await page.screenshot({ path: '%s' });\n", screenshot))
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

// emitStep emits a single E2E step
func (e *PlaywrightEmitter) emitStep(sb *strings.Builder, step map[string]interface{}) {
	action, _ := step["action"].(string)
	selector, _ := step["selector"].(string)
	value, _ := step["value"].(string)

	switch action {
	case "navigate", "goto":
		if url, ok := step["url"].(string); ok {
			sb.WriteString(fmt.Sprintf("    await page.goto('%s');\n", url))
		}
	case "click":
		sb.WriteString(fmt.Sprintf("    await page.click('%s');\n", e.formatSelector(selector)))
	case "fill":
		sb.WriteString(fmt.Sprintf("    await page.fill('%s', '%s');\n", e.formatSelector(selector), value))
	case "type":
		sb.WriteString(fmt.Sprintf("    await page.type('%s', '%s');\n", e.formatSelector(selector), value))
	case "press":
		if key, ok := step["key"].(string); ok {
			sb.WriteString(fmt.Sprintf("    await page.press('%s', '%s');\n", e.formatSelector(selector), key))
		}
	case "select":
		sb.WriteString(fmt.Sprintf("    await page.selectOption('%s', '%s');\n", e.formatSelector(selector), value))
	case "check":
		sb.WriteString(fmt.Sprintf("    await page.check('%s');\n", e.formatSelector(selector)))
	case "uncheck":
		sb.WriteString(fmt.Sprintf("    await page.uncheck('%s');\n", e.formatSelector(selector)))
	case "hover":
		sb.WriteString(fmt.Sprintf("    await page.hover('%s');\n", e.formatSelector(selector)))
	case "wait":
		if timeout, ok := step["timeout"].(float64); ok {
			sb.WriteString(fmt.Sprintf("    await page.waitForTimeout(%d);\n", int(timeout)))
		} else if selector != "" {
			sb.WriteString(fmt.Sprintf("    await page.waitForSelector('%s');\n", e.formatSelector(selector)))
		}
	case "screenshot":
		path := value
		if path == "" {
			path = "screenshot.png"
		}
		sb.WriteString(fmt.Sprintf("    await page.screenshot({ path: '%s' });\n", path))
	case "scroll":
		if selector != "" {
			sb.WriteString(fmt.Sprintf("    await page.locator('%s').scrollIntoViewIfNeeded();\n", e.formatSelector(selector)))
		}
	}
}

func (e *PlaywrightEmitter) emitAssertion(a model.Assertion) string {
	switch a.Kind {
	case "visible":
		return fmt.Sprintf("    await expect(page.locator('%s')).toBeVisible();\n", e.formatSelector(a.Actual))

	case "hidden":
		return fmt.Sprintf("    await expect(page.locator('%s')).toBeHidden();\n", e.formatSelector(a.Actual))

	case "text", "equality":
		if a.Actual == "title" {
			return fmt.Sprintf("    await expect(page).toHaveTitle('%v');\n", a.Expected)
		}
		if a.Actual == "url" {
			return fmt.Sprintf("    await expect(page).toHaveURL('%v');\n", a.Expected)
		}
		return fmt.Sprintf("    await expect(page.locator('%s')).toHaveText('%v');\n", e.formatSelector(a.Actual), a.Expected)

	case "contains":
		if a.Actual == "url" {
			return fmt.Sprintf("    await expect(page).toHaveURL(/%v/);\n", a.Expected)
		}
		return fmt.Sprintf("    await expect(page.locator('%s')).toContainText('%v');\n", e.formatSelector(a.Actual), a.Expected)

	case "value":
		return fmt.Sprintf("    await expect(page.locator('%s')).toHaveValue('%v');\n", e.formatSelector(a.Actual), a.Expected)

	case "count":
		return fmt.Sprintf("    await expect(page.locator('%s')).toHaveCount(%v);\n", e.formatSelector(a.Actual), a.Expected)

	case "enabled":
		return fmt.Sprintf("    await expect(page.locator('%s')).toBeEnabled();\n", e.formatSelector(a.Actual))

	case "disabled":
		return fmt.Sprintf("    await expect(page.locator('%s')).toBeDisabled();\n", e.formatSelector(a.Actual))

	case "checked":
		return fmt.Sprintf("    await expect(page.locator('%s')).toBeChecked();\n", e.formatSelector(a.Actual))

	case "not_null", "exists":
		return fmt.Sprintf("    await expect(page.locator('%s')).toBeAttached();\n", e.formatSelector(a.Actual))

	case "status_code":
		// For page response status
		return fmt.Sprintf("    // Note: Check response status in beforeEach or via route interception\n")

	default:
		return fmt.Sprintf("    // Unknown assertion kind: %s\n", a.Kind)
	}
}

// formatSelector converts a selector string to Playwright format
func (e *PlaywrightEmitter) formatSelector(selector string) string {
	// Already formatted selectors
	if strings.HasPrefix(selector, "[") || strings.HasPrefix(selector, ".") ||
		strings.HasPrefix(selector, "#") || strings.HasPrefix(selector, "//") {
		return selector
	}

	// data-testid shorthand: @testid -> [data-testid="testid"]
	if strings.HasPrefix(selector, "@") {
		return fmt.Sprintf("[data-testid=\"%s\"]", strings.TrimPrefix(selector, "@"))
	}

	// text selector shorthand: text:Submit -> text=Submit
	if strings.HasPrefix(selector, "text:") {
		return "text=" + strings.TrimPrefix(selector, "text:")
	}

	// role selector shorthand: role:button -> role=button
	if strings.HasPrefix(selector, "role:") {
		return "role=" + strings.TrimPrefix(selector, "role:")
	}

	return selector
}

func (e *PlaywrightEmitter) generateTestName(spec model.TestSpec) string {
	if spec.Description != "" {
		return spec.Description
	}
	return "should work correctly"
}

func (e *PlaywrightEmitter) groupByTarget(specs []model.TestSpec) map[string][]model.TestSpec {
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
