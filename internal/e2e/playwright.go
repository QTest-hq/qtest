package e2e

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/QTest-hq/qtest/internal/flow"
)

// PlaywrightGenerator generates Playwright test code.
type PlaywrightGenerator struct {
	config *GenerationConfig
}

// NewPlaywrightGenerator creates a new Playwright generator.
func NewPlaywrightGenerator(config *GenerationConfig) *PlaywrightGenerator {
	if config == nil {
		config = DefaultGenerationConfig()
	}
	return &PlaywrightGenerator{config: config}
}

// Generate generates Playwright tests from an E2E test spec.
func (g *PlaywrightGenerator) Generate(spec *E2ETestSpec) (*GenerationResult, error) {
	result := &GenerationResult{}

	if len(spec.TestCases) == 0 {
		result.Warnings = append(result.Warnings, "No test cases to generate")
		return result, nil
	}

	// Generate main test file
	testCode := g.generateTestFile(spec)
	fileName := g.generateFileName(spec.Name)

	result.Files = append(result.Files, GeneratedFile{
		Path:      fmt.Sprintf("%s/%s", g.config.OutputDir, fileName),
		Content:   testCode,
		Language:  string(g.config.Language),
		TestCount: len(spec.TestCases),
	})
	result.TestCount = len(spec.TestCases)

	// Count total steps
	for _, tc := range spec.TestCases {
		result.StepCount += len(tc.Steps)
	}

	// Generate helpers if configured
	if g.config.GenerateHelpers {
		helpers := g.generateHelpers(spec)
		if helpers != "" {
			result.Files = append(result.Files, GeneratedFile{
				Path:     fmt.Sprintf("%s/helpers.%s", g.config.OutputDir, g.getFileExtension()),
				Content:  helpers,
				Language: string(g.config.Language),
			})
		}
	}

	return result, nil
}

func (g *PlaywrightGenerator) generateTestFile(spec *E2ETestSpec) string {
	var sb strings.Builder

	// Imports
	sb.WriteString(g.generateImports())
	sb.WriteString("\n\n")

	// Test suite
	sb.WriteString(fmt.Sprintf("test.describe('%s', () => {\n", escapeString(spec.Name)))

	// Setup
	if spec.Setup != nil {
		sb.WriteString(g.generateSetup(spec.Setup))
	}

	// Test cases
	for _, tc := range spec.TestCases {
		sb.WriteString(g.generateTestCase(&tc, spec.BaseURL))
		sb.WriteString("\n")
	}

	// Teardown
	if spec.Teardown != nil {
		sb.WriteString(g.generateTeardown(spec.Teardown))
	}

	sb.WriteString("});\n")

	return sb.String()
}

func (g *PlaywrightGenerator) generateImports() string {
	if g.config.Language == LanguageTypeScript {
		return `import { test, expect, Page } from '@playwright/test';`
	}
	return `const { test, expect } = require('@playwright/test');`
}

func (g *PlaywrightGenerator) generateSetup(setup *TestSetup) string {
	var sb strings.Builder

	if setup.BeforeAll != "" {
		sb.WriteString("  test.beforeAll(async () => {\n")
		sb.WriteString(fmt.Sprintf("    %s\n", setup.BeforeAll))
		sb.WriteString("  });\n\n")
	}

	if len(setup.Actions) > 0 {
		sb.WriteString("  test.beforeEach(async ({ page }) => {\n")
		for _, action := range setup.Actions {
			sb.WriteString(fmt.Sprintf("    %s\n", g.generateAction(&action)))
		}
		sb.WriteString("  });\n\n")
	}

	return sb.String()
}

func (g *PlaywrightGenerator) generateTeardown(teardown *TestTeardown) string {
	var sb strings.Builder

	if len(teardown.Actions) > 0 {
		sb.WriteString("  test.afterEach(async ({ page }) => {\n")
		for _, action := range teardown.Actions {
			sb.WriteString(fmt.Sprintf("    %s\n", g.generateAction(&action)))
		}
		sb.WriteString("  });\n\n")
	}

	if teardown.AfterAll != "" {
		sb.WriteString("  test.afterAll(async () => {\n")
		sb.WriteString(fmt.Sprintf("    %s\n", teardown.AfterAll))
		sb.WriteString("  });\n\n")
	}

	return sb.String()
}

func (g *PlaywrightGenerator) generateTestCase(tc *TestCase, baseURL string) string {
	var sb strings.Builder

	// Test function
	testFn := "test"
	if tc.Skip {
		testFn = "test.skip"
	} else if tc.Only {
		testFn = "test.only"
	}

	// Tags as annotations
	if len(tc.Tags) > 0 {
		for _, tag := range tc.Tags {
			sb.WriteString(fmt.Sprintf("  // @%s\n", tag))
		}
	}

	sb.WriteString(fmt.Sprintf("  %s('%s', async ({ page }) => {\n", testFn, escapeString(tc.Name)))

	// Timeout
	if tc.Timeout > 0 {
		sb.WriteString(fmt.Sprintf("    test.setTimeout(%d);\n", tc.Timeout.Milliseconds()))
	}

	// Description as comment
	if tc.Description != "" && g.config.IncludeComments {
		sb.WriteString(fmt.Sprintf("    // %s\n", tc.Description))
	}

	// Steps
	for _, step := range tc.Steps {
		sb.WriteString(g.generateStep(&step, baseURL))
	}

	// Final assertions
	for _, assertion := range tc.Expected {
		sb.WriteString(fmt.Sprintf("    %s\n", g.generateAssertion(&assertion)))
	}

	sb.WriteString("  });\n")

	return sb.String()
}

func (g *PlaywrightGenerator) generateStep(step *TestStep, baseURL string) string {
	var sb strings.Builder

	// Step name as comment
	if step.Name != "" && g.config.IncludeComments {
		sb.WriteString(fmt.Sprintf("    // Step %d: %s\n", step.Order, step.Name))
	}

	// Wait before action if specified
	if step.Wait != nil {
		sb.WriteString(fmt.Sprintf("    %s\n", g.generateWait(step.Wait)))
	}

	// Main action
	action := g.generateAction(&step.Action)
	if step.Action.Type == flow.ActionTypeNavigate && baseURL != "" && !strings.HasPrefix(step.Action.URL, "http") {
		action = fmt.Sprintf("await page.goto('%s%s');", baseURL, step.Action.URL)
	}
	sb.WriteString(fmt.Sprintf("    %s\n", action))

	// Assertions after action
	for _, assertion := range step.Assertions {
		sb.WriteString(fmt.Sprintf("    %s\n", g.generateAssertion(&assertion)))
	}

	// Screenshot if requested
	if step.Screenshot {
		sb.WriteString(fmt.Sprintf("    await page.screenshot({ path: 'screenshots/step-%d.png' });\n", step.Order))
	}

	return sb.String()
}

func (g *PlaywrightGenerator) generateAction(action *TestAction) string {
	switch action.Type {
	case flow.ActionTypeClick:
		return fmt.Sprintf("await page.click('%s');", escapeSelector(action.Selector))

	case flow.ActionTypeFill:
		return fmt.Sprintf("await page.fill('%s', '%s');", escapeSelector(action.Selector), escapeString(action.Value))

	case flow.ActionTypeSelect:
		return fmt.Sprintf("await page.selectOption('%s', '%s');", escapeSelector(action.Selector), escapeString(action.Value))

	case flow.ActionTypeCheck:
		return fmt.Sprintf("await page.check('%s');", escapeSelector(action.Selector))

	case flow.ActionTypeHover:
		return fmt.Sprintf("await page.hover('%s');", escapeSelector(action.Selector))

	case flow.ActionTypeNavigate:
		return fmt.Sprintf("await page.goto('%s');", escapeString(action.URL))

	case flow.ActionTypeKeypress:
		if len(action.Modifiers) > 0 {
			key := strings.Join(action.Modifiers, "+") + "+" + action.Key
			return fmt.Sprintf("await page.keyboard.press('%s');", key)
		}
		return fmt.Sprintf("await page.keyboard.press('%s');", action.Key)

	case flow.ActionTypeWait:
		return fmt.Sprintf("await page.waitForSelector('%s');", escapeSelector(action.Selector))

	case flow.ActionTypeScroll:
		if action.Selector != "" {
			return fmt.Sprintf("await page.locator('%s').scrollIntoViewIfNeeded();", escapeSelector(action.Selector))
		}
		return "await page.evaluate(() => window.scrollBy(0, 300));"

	default:
		return fmt.Sprintf("// Unknown action type: %s", action.Type)
	}
}

func (g *PlaywrightGenerator) generateAssertion(assertion *Assertion) string {
	switch assertion.Type {
	case AssertVisible:
		return fmt.Sprintf("await expect(page.locator('%s')).toBeVisible();", escapeSelector(assertion.Selector))

	case AssertHidden:
		return fmt.Sprintf("await expect(page.locator('%s')).toBeHidden();", escapeSelector(assertion.Selector))

	case AssertText:
		return fmt.Sprintf("await expect(page.locator('%s')).toHaveText('%s');", escapeSelector(assertion.Selector), escapeString(fmt.Sprintf("%v", assertion.Expected)))

	case AssertValue:
		return fmt.Sprintf("await expect(page.locator('%s')).toHaveValue('%s');", escapeSelector(assertion.Selector), escapeString(fmt.Sprintf("%v", assertion.Expected)))

	case AssertEnabled:
		return fmt.Sprintf("await expect(page.locator('%s')).toBeEnabled();", escapeSelector(assertion.Selector))

	case AssertDisabled:
		return fmt.Sprintf("await expect(page.locator('%s')).toBeDisabled();", escapeSelector(assertion.Selector))

	case AssertURL:
		return fmt.Sprintf("await expect(page).toHaveURL('%s');", escapeString(fmt.Sprintf("%v", assertion.Expected)))

	case AssertTitle:
		return fmt.Sprintf("await expect(page).toHaveTitle('%s');", escapeString(fmt.Sprintf("%v", assertion.Expected)))

	case AssertCount:
		return fmt.Sprintf("await expect(page.locator('%s')).toHaveCount(%v);", escapeSelector(assertion.Selector), assertion.Expected)

	case AssertContains:
		return fmt.Sprintf("await expect(page.locator('%s')).toContainText('%s');", escapeSelector(assertion.Selector), escapeString(fmt.Sprintf("%v", assertion.Expected)))

	default:
		return fmt.Sprintf("// Unknown assertion type: %s", assertion.Type)
	}
}

func (g *PlaywrightGenerator) generateWait(wait *WaitConfig) string {
	switch wait.Type {
	case "selector":
		state := wait.State
		if state == "" {
			state = "visible"
		}
		timeout := wait.Timeout
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		return fmt.Sprintf("await page.waitForSelector('%s', { state: '%s', timeout: %d });",
			escapeSelector(wait.Selector), state, timeout.Milliseconds())

	case "timeout":
		return fmt.Sprintf("await page.waitForTimeout(%d);", wait.Timeout.Milliseconds())

	case "networkidle":
		return "await page.waitForLoadState('networkidle');"

	case "load":
		return "await page.waitForLoadState('load');"

	default:
		return fmt.Sprintf("await page.waitForTimeout(%d);", wait.Timeout.Milliseconds())
	}
}

func (g *PlaywrightGenerator) generateHelpers(spec *E2ETestSpec) string {
	var sb strings.Builder

	if g.config.Language == LanguageTypeScript {
		sb.WriteString(`import { Page } from '@playwright/test';

// Helper function to fill a form
export async function fillForm(page: Page, fields: { [key: string]: string }) {
  for (const [selector, value] of Object.entries(fields)) {
    await page.fill(selector, value);
  }
}

// Helper function to click and wait for navigation
export async function clickAndNavigate(page: Page, selector: string) {
  await Promise.all([
    page.waitForNavigation(),
    page.click(selector),
  ]);
}

// Helper function to take a screenshot with timestamp
export async function screenshotWithTimestamp(page: Page, name: string) {
  const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
  await page.screenshot({ path: ` + "`screenshots/${name}-${timestamp}.png`" + ` });
}

// Helper function to wait for network idle
export async function waitForNetworkIdle(page: Page) {
  await page.waitForLoadState('networkidle');
}
`)
	} else {
		sb.WriteString(`// Helper function to fill a form
async function fillForm(page, fields) {
  for (const [selector, value] of Object.entries(fields)) {
    await page.fill(selector, value);
  }
}

// Helper function to click and wait for navigation
async function clickAndNavigate(page, selector) {
  await Promise.all([
    page.waitForNavigation(),
    page.click(selector),
  ]);
}

module.exports = { fillForm, clickAndNavigate };
`)
	}

	return sb.String()
}

func (g *PlaywrightGenerator) generateFileName(name string) string {
	// Convert name to kebab-case
	re := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	kebab := re.ReplaceAllString(strings.ToLower(name), "-")
	kebab = strings.Trim(kebab, "-")

	return fmt.Sprintf("%s.spec.%s", kebab, g.getFileExtension())
}

func (g *PlaywrightGenerator) getFileExtension() string {
	if g.config.Language == LanguageTypeScript {
		return "ts"
	}
	return "js"
}

// FlowToSpec converts a flow to an E2E test spec.
func FlowToSpec(f *flow.Flow, credentials map[string]string) *E2ETestSpec {
	spec := &E2ETestSpec{
		ID:        uuid.New().String(),
		Name:      f.Name,
		Description: f.Description,
		BaseURL:   f.StartURL,
		TestCases: make([]TestCase, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create a test case from the flow
	tc := TestCase{
		ID:   uuid.New().String(),
		Name: f.Name,
		Tags: []string{string(f.Type)},
	}

	// Convert flow steps to test steps
	for _, step := range f.Steps {
		testStep := TestStep{
			ID:    uuid.New().String(),
			Name:  step.Name,
			Order: step.Order,
			Action: TestAction{
				Type:     step.Action.Type,
				Selector: "",
				Value:    step.Action.Value,
				URL:      step.Action.URL,
				Key:      step.Action.Key,
				Modifiers: step.Action.Modifiers,
			},
		}

		if step.Action.Selector != nil {
			testStep.Action.Selector = step.Action.Selector.Primary
		}

		// Handle credential substitution
		if credentials != nil && testStep.Action.Value != "" {
			for key, value := range credentials {
				placeholder := fmt.Sprintf("${%s}", key)
				testStep.Action.Value = strings.ReplaceAll(testStep.Action.Value, placeholder, value)
			}
		}

		// Convert flow assertions to test assertions
		for _, assertion := range step.Assertions {
			selector := ""
			if assertion.Selector != nil {
				selector = assertion.Selector.Primary
			}
			testStep.Assertions = append(testStep.Assertions, Assertion{
				Type:     mapAssertionType(assertion.Type),
				Selector: selector,
				Expected: assertion.Expected,
			})
		}

		tc.Steps = append(tc.Steps, testStep)
	}

	spec.TestCases = append(spec.TestCases, tc)

	return spec
}

func mapAssertionType(flowType string) AssertionType {
	switch flowType {
	case "visible":
		return AssertVisible
	case "hidden":
		return AssertHidden
	case "text":
		return AssertText
	case "value":
		return AssertValue
	case "enabled":
		return AssertEnabled
	case "disabled":
		return AssertDisabled
	case "url":
		return AssertURL
	case "title":
		return AssertTitle
	case "count":
		return AssertCount
	case "contains":
		return AssertContains
	default:
		return AssertVisible
	}
}

// Helper functions

func escapeString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	return s
}

func escapeSelector(s string) string {
	// Escape single quotes in selectors
	return strings.ReplaceAll(s, "'", "\\'")
}
