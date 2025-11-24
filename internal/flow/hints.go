package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/QTest-hq/qtest/internal/sidecar/playwright"
	"gopkg.in/yaml.v3"
)

// HintLoader loads flow hints from files.
type HintLoader struct {
	basePath string
}

// NewHintLoader creates a new hint loader.
func NewHintLoader(basePath string) *HintLoader {
	return &HintLoader{basePath: basePath}
}

// LoadHints loads hints from a YAML or JSON file.
func (l *HintLoader) LoadHints(filename string) ([]FlowHint, error) {
	path := filepath.Join(l.basePath, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read hints file: %w", err)
	}

	var hints []FlowHint

	// Try YAML first (also handles JSON)
	if err := yaml.Unmarshal(data, &hints); err != nil {
		// Try JSON
		if err := json.Unmarshal(data, &hints); err != nil {
			return nil, fmt.Errorf("failed to parse hints: %w", err)
		}
	}

	return hints, nil
}

// LoadHintsFromString loads hints from a YAML/JSON string.
func (l *HintLoader) LoadHintsFromString(content string) ([]FlowHint, error) {
	var hints []FlowHint

	if err := yaml.Unmarshal([]byte(content), &hints); err != nil {
		if err := json.Unmarshal([]byte(content), &hints); err != nil {
			return nil, fmt.Errorf("failed to parse hints: %w", err)
		}
	}

	return hints, nil
}

// SaveHints saves hints to a YAML file.
func (l *HintLoader) SaveHints(filename string, hints []FlowHint) error {
	path := filepath.Join(l.basePath, filename)

	data, err := yaml.Marshal(hints)
	if err != nil {
		return fmt.Errorf("failed to marshal hints: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write hints file: %w", err)
	}

	return nil
}

// HintExecutor executes flows based on user-provided hints.
type HintExecutor struct {
	client *playwright.Client
	config *FlowConfig
}

// NewHintExecutor creates a new hint executor.
func NewHintExecutor(client *playwright.Client, config *FlowConfig) *HintExecutor {
	if config == nil {
		config = DefaultFlowConfig()
	}
	return &HintExecutor{
		client: client,
		config: config,
	}
}

// ExecuteHint executes a flow hint and returns the recorded flow.
func (e *HintExecutor) ExecuteHint(ctx context.Context, hint FlowHint) (*Flow, error) {
	// Create session
	sessionID, err := e.client.CreateSession(ctx, playwright.SessionConfig{
		BrowserType:    "chromium",
		Headless:       true,
		ViewportWidth:  1920,
		ViewportHeight: 1080,
		TimeoutMs:      int32(e.config.PageLoadTimeout.Milliseconds()),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer e.client.DestroySession(ctx, sessionID)

	// Start network capture
	if e.config.RecordNetwork {
		e.client.StartNetworkCapture(ctx, sessionID, playwright.NetworkCaptureConfig{
			CaptureRequestBody:  true,
			CaptureResponseBody: true,
		})
	}

	// Navigate to start URL if provided
	if hint.StartURL != "" {
		_, err = e.client.Navigate(ctx, sessionID, hint.StartURL, &playwright.NavigateOptions{
			TimeoutMs: int32(e.config.PageLoadTimeout.Milliseconds()),
			WaitUntil: "networkidle",
		})
		if err != nil {
			return nil, fmt.Errorf("failed to navigate to start URL: %w", err)
		}
	}

	// Build flow from hint
	flow := &Flow{
		ID:          uuid.New().String(),
		Name:        hint.Description,
		Type:        hint.Type,
		StartURL:    hint.StartURL,
		Steps:       make([]Step, 0, len(hint.Steps)),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Execute each step
	for i, hintStep := range hint.Steps {
		// Resolve selector
		selector := hintStep.Target
		if namedSelector, ok := hint.Selectors[selector]; ok {
			selector = namedSelector
		}

		// Resolve value (handle credential references)
		value := hintStep.Value
		if hint.Credentials != nil {
			value = e.resolveValue(value, hint.Credentials)
		}

		step, err := e.executeStep(ctx, sessionID, hintStep, selector, value, i+1)
		if err != nil {
			return flow, fmt.Errorf("step %d failed: %w", i+1, err)
		}

		flow.Steps = append(flow.Steps, *step)
	}

	// Stop network capture
	if e.config.RecordNetwork {
		e.client.StopNetworkCapture(ctx, sessionID)
	}

	flow.UpdatedAt = time.Now()
	return flow, nil
}

// ExecuteAllHints executes multiple hints and returns all flows.
func (e *HintExecutor) ExecuteAllHints(ctx context.Context, hints []FlowHint) ([]*Flow, []error) {
	flows := make([]*Flow, 0, len(hints))
	errors := make([]error, 0)

	for _, hint := range hints {
		flow, err := e.ExecuteHint(ctx, hint)
		if err != nil {
			errors = append(errors, err)
		}
		if flow != nil {
			flows = append(flows, flow)
		}
	}

	return flows, errors
}

func (e *HintExecutor) executeStep(ctx context.Context, sessionID string, hintStep HintStep, selector, value string, order int) (*Step, error) {
	startTime := time.Now()

	var err error
	actionType := mapActionType(hintStep.Action)
	timeoutMs := int32(e.config.ActionTimeout.Milliseconds())

	switch actionType {
	case ActionTypeClick:
		err = e.client.Click(ctx, sessionID, selector, timeoutMs)

	case ActionTypeFill:
		err = e.client.Fill(ctx, sessionID, selector, value, timeoutMs)

	case ActionTypeSelect:
		// Select uses fill with the value
		err = e.client.Fill(ctx, sessionID, selector, value, timeoutMs)

	case ActionTypeCheck:
		// Check uses click
		err = e.client.Click(ctx, sessionID, selector, timeoutMs)

	case ActionTypeHover:
		// Hover not directly supported, use wait
		err = e.client.WaitForSelector(ctx, sessionID, selector, timeoutMs, "visible")

	case ActionTypeKeypress:
		// Keypress not directly supported, use fill
		err = e.client.Fill(ctx, sessionID, selector, value, timeoutMs)

	case ActionTypeNavigate:
		_, err = e.client.Navigate(ctx, sessionID, value, &playwright.NavigateOptions{
			TimeoutMs: int32(e.config.PageLoadTimeout.Milliseconds()),
			WaitUntil: "networkidle",
		})

	case ActionTypeWait:
		err = e.client.WaitForSelector(ctx, sessionID, selector, timeoutMs, "visible")

	default:
		err = fmt.Errorf("unsupported action type: %s", hintStep.Action)
	}

	if err != nil {
		return nil, err
	}

	duration := time.Since(startTime)

	// Wait for action delay
	time.Sleep(e.config.ActionDelay)

	// Build selector object
	var sel *Selector
	if selector != "" {
		sel = &Selector{
			Primary:    selector,
			Strategy:   SelectorCSS,
			Confidence: 0.9, // High confidence for user-provided hints
		}
	}

	step := &Step{
		ID:   uuid.New().String(),
		Name: hintStep.Description,
		Action: Action{
			ID:          uuid.New().String(),
			Type:        actionType,
			Selector:    sel,
			Value:       value,
			Description: hintStep.Description,
			Timestamp:   startTime,
		},
		Duration: duration,
		Order:    order,
	}

	return step, nil
}

func (e *HintExecutor) resolveValue(value string, credentials map[string]string) string {
	// Check if value is a credential reference like ${username} or ${password}
	if len(value) > 3 && value[0] == '$' && value[1] == '{' && value[len(value)-1] == '}' {
		key := value[2 : len(value)-1]
		if credValue, ok := credentials[key]; ok {
			return credValue
		}
	}
	return value
}

// ValidateHint validates a flow hint for correctness.
func ValidateHint(hint FlowHint) []string {
	var errors []string

	if hint.Type == "" {
		errors = append(errors, "flow type is required")
	}

	if len(hint.Steps) == 0 {
		errors = append(errors, "at least one step is required")
	}

	for i, step := range hint.Steps {
		if step.Action == "" {
			errors = append(errors, fmt.Sprintf("step %d: action is required", i+1))
		}

		// Validate action-specific requirements
		actionType := mapActionType(step.Action)
		switch actionType {
		case ActionTypeFill, ActionTypeSelect:
			if step.Target == "" {
				errors = append(errors, fmt.Sprintf("step %d: target selector is required for %s", i+1, step.Action))
			}
		case ActionTypeClick, ActionTypeCheck, ActionTypeHover:
			if step.Target == "" {
				errors = append(errors, fmt.Sprintf("step %d: target selector is required for %s", i+1, step.Action))
			}
		case ActionTypeNavigate:
			if step.Value == "" {
				errors = append(errors, fmt.Sprintf("step %d: URL value is required for navigate", i+1))
			}
		}
	}

	return errors
}

// MergeHints merges multiple hint sets, deduplicating by description.
func MergeHints(hintSets ...[]FlowHint) []FlowHint {
	seen := make(map[string]bool)
	merged := make([]FlowHint, 0)

	for _, hints := range hintSets {
		for _, hint := range hints {
			key := fmt.Sprintf("%s-%s", hint.Type, hint.Description)
			if !seen[key] {
				seen[key] = true
				merged = append(merged, hint)
			}
		}
	}

	return merged
}

// GenerateHintTemplate generates a template hint file for common flows.
func GenerateHintTemplate() []FlowHint {
	return []FlowHint{
		{
			Type:        FlowTypeLogin,
			StartURL:    "https://example.com/login",
			Description: "User login flow",
			Credentials: map[string]string{
				"username": "testuser@example.com",
				"password": "testpassword123",
			},
			Steps: []HintStep{
				{Action: "fill", Target: "input[name='email']", Value: "${username}", Description: "Enter email"},
				{Action: "fill", Target: "input[name='password']", Value: "${password}", Description: "Enter password"},
				{Action: "click", Target: "button[type='submit']", Description: "Click login button"},
			},
			Selectors: map[string]string{
				"emailInput":    "input[name='email'], input[type='email']",
				"passwordInput": "input[name='password'], input[type='password']",
				"submitButton":  "button[type='submit'], input[type='submit']",
			},
		},
		{
			Type:        FlowTypeSearch,
			StartURL:    "https://example.com",
			Description: "Search functionality",
			Steps: []HintStep{
				{Action: "fill", Target: "input[type='search']", Value: "test query", Description: "Enter search term"},
				{Action: "click", Target: "button[type='submit']", Description: "Submit search"},
			},
		},
		{
			Type:        FlowTypeNavigation,
			Description: "Navigate to key pages",
			Steps: []HintStep{
				{Action: "navigate", Value: "https://example.com/about", Description: "Go to About page"},
				{Action: "navigate", Value: "https://example.com/contact", Description: "Go to Contact page"},
			},
		},
	}
}

// ExportHintsToYAML exports hints to a YAML string.
func ExportHintsToYAML(hints []FlowHint) (string, error) {
	data, err := yaml.Marshal(hints)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ExportHintsToJSON exports hints to a JSON string.
func ExportHintsToJSON(hints []FlowHint) (string, error) {
	data, err := json.MarshalIndent(hints, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
