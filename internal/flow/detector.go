package flow

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/QTest-hq/qtest/internal/sidecar/playwright"
)

// Detector detects user flows and forms on web pages.
type Detector struct {
	client *playwright.Client
	config *FlowConfig
}

// NewDetector creates a new flow detector.
func NewDetector(client *playwright.Client, config *FlowConfig) *Detector {
	if config == nil {
		config = DefaultFlowConfig()
	}
	return &Detector{
		client: client,
		config: config,
	}
}

// DetectForms detects all forms on the current page.
func (d *Detector) DetectForms(ctx context.Context, sessionID string) ([]Form, error) {
	// Get page forms from Playwright
	pageForms, err := d.client.GetPageForms(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get page forms: %w", err)
	}

	forms := make([]Form, 0, len(pageForms))
	for i, pf := range pageForms {
		form := Form{
			ID:     fmt.Sprintf("form-%d", i),
			Action: pf.Action,
			Method: pf.Method,
			Fields: make([]FormField, 0, len(pf.Fields)),
			Selector: Selector{
				Primary:    fmt.Sprintf("form:nth-of-type(%d)", i+1),
				Strategy:   SelectorCSS,
				Confidence: 0.7,
			},
		}

		// Convert fields
		for _, field := range pf.Fields {
			ff := FormField{
				Name:        field.Name,
				Type:        field.Type,
				Placeholder: field.Placeholder,
				Required:    field.Required,
				Selector: Selector{
					Primary:    d.buildFieldSelector(field),
					Strategy:   SelectorCSS,
					Confidence: 0.8,
				},
			}
			form.Fields = append(form.Fields, ff)
		}

		// Detect form type
		form.FormType = d.classifyForm(form)

		// Find submit button
		submitSelector := d.findSubmitButton(form)
		if submitSelector != nil {
			form.SubmitButton = submitSelector
		}

		forms = append(forms, form)
	}

	return forms, nil
}

// DetectLoginForm detects login forms on the current page.
func (d *Detector) DetectLoginForm(ctx context.Context, sessionID string) (*Form, error) {
	forms, err := d.DetectForms(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	for _, form := range forms {
		if d.isLoginForm(form) {
			form.FormType = "login"
			return &form, nil
		}
	}

	return nil, nil
}

// DetectLoginFlow attempts to detect and build a login flow.
func (d *Detector) DetectLoginFlow(ctx context.Context, sessionID string, credentials map[string]string) (*Flow, error) {
	// Detect login form
	loginForm, err := d.DetectLoginForm(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if loginForm == nil {
		return nil, fmt.Errorf("no login form detected")
	}

	// Get current URL
	status, err := d.client.GetSessionStatus(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session status: %w", err)
	}

	// Build login flow
	flow := &Flow{
		ID:        uuid.New().String(),
		Name:      "Login Flow",
		Type:      FlowTypeLogin,
		StartURL:  status.CurrentURL,
		Steps:     make([]Step, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	stepOrder := 0

	// Add steps for each credential field
	for _, field := range loginForm.Fields {
		value, hasValue := credentials[field.Name]
		if !hasValue {
			// Try common aliases
			value, hasValue = d.getCredentialValue(field, credentials)
		}

		if hasValue && (field.Type == "text" || field.Type == "email" || field.Type == "password") {
			stepOrder++
			step := Step{
				ID:   uuid.New().String(),
				Name: fmt.Sprintf("Fill %s", field.Name),
				Action: Action{
					ID:          uuid.New().String(),
					Type:        ActionTypeFill,
					Selector:    &field.Selector,
					Value:       value,
					Description: fmt.Sprintf("Enter %s", field.Name),
					Timestamp:   time.Now(),
				},
				Order: stepOrder,
			}
			flow.Steps = append(flow.Steps, step)
		}
	}

	// Add submit step
	if loginForm.SubmitButton != nil {
		stepOrder++
		step := Step{
			ID:   uuid.New().String(),
			Name: "Submit login form",
			Action: Action{
				ID:          uuid.New().String(),
				Type:        ActionTypeClick,
				Selector:    loginForm.SubmitButton,
				Description: "Click login button",
				Timestamp:   time.Now(),
			},
			Order: stepOrder,
		}
		flow.Steps = append(flow.Steps, step)
	}

	return flow, nil
}

// DetectSearchForm detects search forms on the current page.
func (d *Detector) DetectSearchForm(ctx context.Context, sessionID string) (*Form, error) {
	forms, err := d.DetectForms(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	for _, form := range forms {
		if d.isSearchForm(form) {
			form.FormType = "search"
			return &form, nil
		}
	}

	return nil, nil
}

// DetectRegistrationForm detects registration forms.
func (d *Detector) DetectRegistrationForm(ctx context.Context, sessionID string) (*Form, error) {
	forms, err := d.DetectForms(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	for _, form := range forms {
		if d.isRegistrationForm(form) {
			form.FormType = "registration"
			return &form, nil
		}
	}

	return nil, nil
}

// DetectAll performs comprehensive detection on the current page.
func (d *Detector) DetectAll(ctx context.Context, sessionID string) (*DetectionResult, error) {
	result := &DetectionResult{
		DetectedAt: time.Now(),
	}

	// Detect forms
	forms, err := d.DetectForms(ctx, sessionID)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("form detection: %v", err))
	} else {
		result.Forms = forms
	}

	// Capture page state
	status, _ := d.client.GetSessionStatus(ctx, sessionID)
	if status != nil {
		pageState := PageState{
			URL:        status.CurrentURL,
			Title:      status.PageTitle,
			Forms:      forms,
			CapturedAt: time.Now(),
		}
		result.PageStates = append(result.PageStates, pageState)
	}

	// Generate flow suggestions based on detected forms
	for _, form := range forms {
		hint := d.generateFlowHint(form)
		if hint != nil {
			result.Suggestions = append(result.Suggestions, *hint)
		}
	}

	return result, nil
}

// Helper methods

func (d *Detector) buildFieldSelector(field *playwright.FormField) string {
	// Try to build a robust selector
	if field.Name != "" {
		return fmt.Sprintf("input[name='%s'], textarea[name='%s'], select[name='%s']",
			field.Name, field.Name, field.Name)
	}
	if field.Placeholder != "" {
		return fmt.Sprintf("[placeholder='%s']", field.Placeholder)
	}
	return fmt.Sprintf("input[type='%s']", field.Type)
}

func (d *Detector) classifyForm(form Form) string {
	if d.isLoginForm(form) {
		return "login"
	}
	if d.isRegistrationForm(form) {
		return "registration"
	}
	if d.isSearchForm(form) {
		return "search"
	}
	if d.isCheckoutForm(form) {
		return "checkout"
	}
	if d.isContactForm(form) {
		return "contact"
	}
	return "unknown"
}

func (d *Detector) isLoginForm(form Form) bool {
	hasPassword := false
	hasUsername := false

	for _, field := range form.Fields {
		fieldName := strings.ToLower(field.Name)
		fieldType := strings.ToLower(field.Type)
		placeholder := strings.ToLower(field.Placeholder)

		if fieldType == "password" {
			hasPassword = true
		}

		if fieldType == "email" || fieldType == "text" {
			if containsAny(fieldName, []string{"user", "email", "login", "username"}) ||
				containsAny(placeholder, []string{"user", "email", "login", "username"}) {
				hasUsername = true
			}
		}
	}

	// Login forms typically have username/email + password, and few fields
	return hasPassword && hasUsername && len(form.Fields) <= 4
}

func (d *Detector) isRegistrationForm(form Form) bool {
	hasPassword := false
	hasConfirmPassword := false
	hasEmail := false
	fieldCount := len(form.Fields)

	for _, field := range form.Fields {
		fieldName := strings.ToLower(field.Name)
		fieldType := strings.ToLower(field.Type)

		if fieldType == "password" {
			if containsAny(fieldName, []string{"confirm", "repeat", "retype"}) {
				hasConfirmPassword = true
			} else {
				hasPassword = true
			}
		}

		if fieldType == "email" || containsAny(fieldName, []string{"email"}) {
			hasEmail = true
		}
	}

	// Registration forms typically have more fields and confirm password
	return hasPassword && hasEmail && (hasConfirmPassword || fieldCount >= 4)
}

func (d *Detector) isSearchForm(form Form) bool {
	for _, field := range form.Fields {
		fieldName := strings.ToLower(field.Name)
		fieldType := strings.ToLower(field.Type)
		placeholder := strings.ToLower(field.Placeholder)

		if fieldType == "search" {
			return true
		}

		if containsAny(fieldName, []string{"search", "query", "q", "keyword"}) ||
			containsAny(placeholder, []string{"search", "find", "look"}) {
			return true
		}
	}

	// Also check form action
	if containsAny(strings.ToLower(form.Action), []string{"search", "find", "query"}) {
		return true
	}

	return false
}

func (d *Detector) isCheckoutForm(form Form) bool {
	checkoutIndicators := 0

	for _, field := range form.Fields {
		fieldName := strings.ToLower(field.Name)

		if containsAny(fieldName, []string{"card", "credit", "cvv", "expir", "billing", "shipping", "address", "zip", "postal"}) {
			checkoutIndicators++
		}
	}

	return checkoutIndicators >= 2
}

func (d *Detector) isContactForm(form Form) bool {
	hasEmail := false
	hasMessage := false

	for _, field := range form.Fields {
		fieldName := strings.ToLower(field.Name)
		fieldType := strings.ToLower(field.Type)

		if fieldType == "email" || containsAny(fieldName, []string{"email"}) {
			hasEmail = true
		}

		if fieldType == "textarea" || containsAny(fieldName, []string{"message", "comment", "body", "content"}) {
			hasMessage = true
		}
	}

	return hasEmail && hasMessage
}

func (d *Detector) findSubmitButton(form Form) *Selector {
	// Common submit button selectors
	selectors := []string{
		"button[type='submit']",
		"input[type='submit']",
		"button:contains('Login')",
		"button:contains('Sign in')",
		"button:contains('Submit')",
		"button:contains('Register')",
		"button:contains('Search')",
	}

	// Return the first likely selector
	return &Selector{
		Primary:    selectors[0],
		Strategy:   SelectorCSS,
		Confidence: 0.7,
		Fallbacks: []Selector{
			{Primary: selectors[1], Strategy: SelectorCSS, Confidence: 0.6},
		},
	}
}

func (d *Detector) getCredentialValue(field FormField, credentials map[string]string) (string, bool) {
	fieldName := strings.ToLower(field.Name)
	fieldType := strings.ToLower(field.Type)

	// Common credential field mappings
	mappings := map[string][]string{
		"username": {"user", "login", "email", "username"},
		"password": {"pass", "pwd", "password"},
		"email":    {"email", "mail", "e-mail"},
	}

	for credKey, aliases := range mappings {
		if value, exists := credentials[credKey]; exists {
			if containsAny(fieldName, aliases) || fieldType == credKey {
				return value, true
			}
		}
	}

	return "", false
}

func (d *Detector) generateFlowHint(form Form) *FlowHint {
	var flowType FlowType
	var description string

	switch form.FormType {
	case "login":
		flowType = FlowTypeLogin
		description = "Login with credentials"
	case "registration":
		flowType = FlowTypeRegistration
		description = "User registration"
	case "search":
		flowType = FlowTypeSearch
		description = "Search functionality"
	case "checkout":
		flowType = FlowTypeCheckout
		description = "Checkout process"
	default:
		flowType = FlowTypeFormSubmit
		description = "Form submission"
	}

	hint := &FlowHint{
		Type:        flowType,
		Description: description,
		Steps:       make([]HintStep, 0),
	}

	// Add hint steps for each field
	for _, field := range form.Fields {
		step := HintStep{
			Action:      "fill",
			Target:      field.Selector.Primary,
			Description: fmt.Sprintf("Enter %s", field.Name),
		}
		hint.Steps = append(hint.Steps, step)
	}

	// Add submit step
	if form.SubmitButton != nil {
		hint.Steps = append(hint.Steps, HintStep{
			Action:      "click",
			Target:      form.SubmitButton.Primary,
			Description: "Submit form",
		})
	}

	return hint
}

// ScanPages scans multiple pages starting from a URL and detects flows.
func (d *Detector) ScanPages(ctx context.Context, startURL string, maxPages int) (*DetectionResult, error) {
	// Create session
	sessionID, err := d.client.CreateSession(ctx, playwright.SessionConfig{
		BrowserType:    "chromium",
		Headless:       true,
		ViewportWidth:  1920,
		ViewportHeight: 1080,
		TimeoutMs:      int32(d.config.PageLoadTimeout.Milliseconds()),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer d.client.DestroySession(ctx, sessionID)

	result := &DetectionResult{
		DetectedAt: time.Now(),
	}

	visited := make(map[string]bool)
	queue := []string{startURL}

	parsedStart, _ := url.Parse(startURL)
	baseHost := parsedStart.Host

	for len(queue) > 0 && len(visited) < maxPages {
		currentURL := queue[0]
		queue = queue[1:]

		if visited[currentURL] {
			continue
		}
		visited[currentURL] = true

		// Navigate to page
		_, err := d.client.Navigate(ctx, sessionID, currentURL, &playwright.NavigateOptions{
			TimeoutMs: int32(d.config.PageLoadTimeout.Milliseconds()),
			WaitUntil: "networkidle",
		})
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("navigation to %s: %v", currentURL, err))
			continue
		}

		// Detect on this page
		pageResult, err := d.DetectAll(ctx, sessionID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("detection on %s: %v", currentURL, err))
			continue
		}

		// Merge results
		result.Forms = append(result.Forms, pageResult.Forms...)
		result.Suggestions = append(result.Suggestions, pageResult.Suggestions...)
		result.PageStates = append(result.PageStates, pageResult.PageStates...)

		// Get links for further crawling
		links, _ := d.client.GetPageLinks(ctx, sessionID, true)
		for _, link := range links {
			parsedLink, err := url.Parse(link.Href)
			if err != nil {
				continue
			}

			// Only follow same-host links
			if parsedLink.Host == baseHost && !visited[link.Href] {
				queue = append(queue, link.Href)
			}
		}
	}

	return result, nil
}

// Utility functions

func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
