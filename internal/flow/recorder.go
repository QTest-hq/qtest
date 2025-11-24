package flow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/QTest-hq/qtest/internal/sidecar/playwright"
)

// Recorder records user actions to build flows.
type Recorder struct {
	client    *playwright.Client
	sessionID string
	config    *FlowConfig
	flow      *Flow
	mu        sync.Mutex
	recording bool
	stepOrder int
}

// NewRecorder creates a new action recorder.
func NewRecorder(client *playwright.Client, config *FlowConfig) *Recorder {
	if config == nil {
		config = DefaultFlowConfig()
	}
	return &Recorder{
		client: client,
		config: config,
	}
}

// StartRecording begins recording a new flow.
func (r *Recorder) StartRecording(ctx context.Context, name string, flowType FlowType, startURL string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.recording {
		return fmt.Errorf("already recording a flow")
	}

	// Create browser session
	sessionID, err := r.client.CreateSession(ctx, playwright.SessionConfig{
		BrowserType:    "chromium",
		Headless:       false, // Non-headless for recording
		ViewportWidth:  1920,
		ViewportHeight: 1080,
		UserAgent:      "QTest-Recorder/1.0",
		TimeoutMs:      int32(r.config.PageLoadTimeout.Milliseconds()),
	})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	r.sessionID = sessionID

	// Start network capture if configured
	if r.config.RecordNetwork {
		r.client.StartNetworkCapture(ctx, sessionID, playwright.NetworkCaptureConfig{
			CaptureRequestBody:  true,
			CaptureResponseBody: true,
		})
	}

	// Navigate to start URL
	_, err = r.client.Navigate(ctx, sessionID, startURL, &playwright.NavigateOptions{
		TimeoutMs: int32(r.config.PageLoadTimeout.Milliseconds()),
		WaitUntil: "networkidle",
	})
	if err != nil {
		r.client.DestroySession(ctx, sessionID)
		return fmt.Errorf("failed to navigate to start URL: %w", err)
	}

	// Initialize flow
	r.flow = &Flow{
		ID:        uuid.New().String(),
		Name:      name,
		Type:      flowType,
		StartURL:  startURL,
		Steps:     make([]Step, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	r.stepOrder = 0
	r.recording = true

	return nil
}

// StopRecording stops recording and returns the completed flow.
func (r *Recorder) StopRecording(ctx context.Context) (*Flow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.recording {
		return nil, fmt.Errorf("not recording")
	}

	// Stop network capture
	if r.config.RecordNetwork {
		r.client.StopNetworkCapture(ctx, r.sessionID)
	}

	// Destroy session
	r.client.DestroySession(ctx, r.sessionID)

	r.recording = false
	r.flow.UpdatedAt = time.Now()

	return r.flow, nil
}

// RecordClick records a click action.
func (r *Recorder) RecordClick(ctx context.Context, selector string, description string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.recording {
		return fmt.Errorf("not recording")
	}

	// Capture page state before
	var pageStateBefore *PageState
	if r.config.RecordDOMSnapshots {
		pageStateBefore = r.capturePageState(ctx)
	}

	startTime := time.Now()

	// Perform the click
	err := r.client.Click(ctx, r.sessionID, selector, int32(r.config.ActionTimeout.Milliseconds()))
	if err != nil {
		return fmt.Errorf("click failed: %w", err)
	}

	duration := time.Since(startTime)

	// Wait for action delay
	time.Sleep(r.config.ActionDelay)

	// Capture page state after
	var pageStateAfter *PageState
	if r.config.RecordDOMSnapshots {
		pageStateAfter = r.capturePageState(ctx)
	}

	// Capture network calls
	var networkCalls []NetworkRequest
	if r.config.RecordNetwork {
		networkCalls = r.captureNetworkCalls(ctx)
	}

	// Build selector
	sel := r.buildSelector(selector)

	// Create step
	r.stepOrder++
	step := Step{
		ID:   uuid.New().String(),
		Name: fmt.Sprintf("Click: %s", description),
		Action: Action{
			ID:          uuid.New().String(),
			Type:        ActionTypeClick,
			Selector:    &sel,
			Description: description,
			Timestamp:   startTime,
		},
		NetworkCalls:    networkCalls,
		PageStateBefore: pageStateBefore,
		PageStateAfter:  pageStateAfter,
		Duration:        duration,
		Order:           r.stepOrder,
	}

	r.flow.Steps = append(r.flow.Steps, step)
	r.flow.UpdatedAt = time.Now()

	return nil
}

// RecordFill records a fill action.
func (r *Recorder) RecordFill(ctx context.Context, selector string, value string, description string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.recording {
		return fmt.Errorf("not recording")
	}

	startTime := time.Now()

	// Perform the fill
	err := r.client.Fill(ctx, r.sessionID, selector, value, int32(r.config.ActionTimeout.Milliseconds()))
	if err != nil {
		return fmt.Errorf("fill failed: %w", err)
	}

	duration := time.Since(startTime)
	time.Sleep(r.config.ActionDelay)

	// Build selector
	sel := r.buildSelector(selector)

	// Create step
	r.stepOrder++
	step := Step{
		ID:   uuid.New().String(),
		Name: fmt.Sprintf("Fill: %s", description),
		Action: Action{
			ID:          uuid.New().String(),
			Type:        ActionTypeFill,
			Selector:    &sel,
			Value:       value,
			Description: description,
			Timestamp:   startTime,
		},
		Duration: duration,
		Order:    r.stepOrder,
	}

	r.flow.Steps = append(r.flow.Steps, step)
	r.flow.UpdatedAt = time.Now()

	return nil
}

// RecordSelect records a select action.
func (r *Recorder) RecordSelect(ctx context.Context, selector string, value string, description string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.recording {
		return fmt.Errorf("not recording")
	}

	startTime := time.Now()

	// Perform the select (using fill for now as select is not directly supported)
	err := r.client.Fill(ctx, r.sessionID, selector, value, int32(r.config.ActionTimeout.Milliseconds()))
	if err != nil {
		return fmt.Errorf("select failed: %w", err)
	}

	duration := time.Since(startTime)
	time.Sleep(r.config.ActionDelay)

	sel := r.buildSelector(selector)

	r.stepOrder++
	step := Step{
		ID:   uuid.New().String(),
		Name: fmt.Sprintf("Select: %s", description),
		Action: Action{
			ID:          uuid.New().String(),
			Type:        ActionTypeSelect,
			Selector:    &sel,
			Value:       value,
			Description: description,
			Timestamp:   startTime,
		},
		Duration: duration,
		Order:    r.stepOrder,
	}

	r.flow.Steps = append(r.flow.Steps, step)
	r.flow.UpdatedAt = time.Now()

	return nil
}

// RecordNavigate records a navigation action.
func (r *Recorder) RecordNavigate(ctx context.Context, url string, description string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.recording {
		return fmt.Errorf("not recording")
	}

	startTime := time.Now()

	// Perform navigation
	_, err := r.client.Navigate(ctx, r.sessionID, url, &playwright.NavigateOptions{
		TimeoutMs: int32(r.config.PageLoadTimeout.Milliseconds()),
		WaitUntil: "networkidle",
	})
	if err != nil {
		return fmt.Errorf("navigation failed: %w", err)
	}

	duration := time.Since(startTime)

	// Capture network calls
	var networkCalls []NetworkRequest
	if r.config.RecordNetwork {
		networkCalls = r.captureNetworkCalls(ctx)
	}

	r.stepOrder++
	step := Step{
		ID:   uuid.New().String(),
		Name: fmt.Sprintf("Navigate: %s", description),
		Action: Action{
			ID:          uuid.New().String(),
			Type:        ActionTypeNavigate,
			URL:         url,
			Description: description,
			Timestamp:   startTime,
		},
		NetworkCalls: networkCalls,
		Duration:     duration,
		Order:        r.stepOrder,
	}

	r.flow.Steps = append(r.flow.Steps, step)
	r.flow.UpdatedAt = time.Now()

	return nil
}

// RecordKeypress records a keypress action.
func (r *Recorder) RecordKeypress(ctx context.Context, selector string, key string, modifiers []string, description string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.recording {
		return fmt.Errorf("not recording")
	}

	startTime := time.Now()

	// Perform the keypress (using fill for now as press is not directly supported)
	err := r.client.Fill(ctx, r.sessionID, selector, key, int32(r.config.ActionTimeout.Milliseconds()))
	if err != nil {
		return fmt.Errorf("keypress failed: %w", err)
	}

	duration := time.Since(startTime)
	time.Sleep(r.config.ActionDelay)

	sel := r.buildSelector(selector)

	r.stepOrder++
	step := Step{
		ID:   uuid.New().String(),
		Name: fmt.Sprintf("Press: %s", description),
		Action: Action{
			ID:          uuid.New().String(),
			Type:        ActionTypeKeypress,
			Selector:    &sel,
			Key:         key,
			Modifiers:   modifiers,
			Description: description,
			Timestamp:   startTime,
		},
		Duration: duration,
		Order:    r.stepOrder,
	}

	r.flow.Steps = append(r.flow.Steps, step)
	r.flow.UpdatedAt = time.Now()

	return nil
}

// AddAssertion adds an assertion to the last step.
func (r *Recorder) AddAssertion(assertion Assertion) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.recording {
		return fmt.Errorf("not recording")
	}

	if len(r.flow.Steps) == 0 {
		return fmt.Errorf("no steps to add assertion to")
	}

	assertion.ID = uuid.New().String()
	lastIdx := len(r.flow.Steps) - 1
	r.flow.Steps[lastIdx].Assertions = append(r.flow.Steps[lastIdx].Assertions, assertion)

	return nil
}

// TakeScreenshot takes a screenshot and optionally adds it to the last step.
func (r *Recorder) TakeScreenshot(ctx context.Context) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.recording {
		return nil, fmt.Errorf("not recording")
	}

	screenshot, err := r.client.TakeScreenshot(ctx, r.sessionID, false, "png")
	if err != nil {
		return nil, fmt.Errorf("failed to take screenshot: %w", err)
	}

	return screenshot, nil
}

// GetCurrentFlow returns the current flow being recorded.
func (r *Recorder) GetCurrentFlow() *Flow {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.flow
}

// IsRecording returns whether recording is in progress.
func (r *Recorder) IsRecording() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.recording
}

// Helper functions

func (r *Recorder) buildSelector(selector string) Selector {
	// Determine selector strategy based on format
	strategy := r.detectSelectorStrategy(selector)

	return Selector{
		Primary:    selector,
		Strategy:   strategy,
		Confidence: 0.8, // Default confidence
	}
}

func (r *Recorder) detectSelectorStrategy(selector string) SelectorStrategy {
	if len(selector) == 0 {
		return SelectorCSS
	}

	// Check for common patterns
	switch {
	case selector[0] == '#':
		return SelectorID
	case selector[0] == '[' && containsAttr(selector, "data-testid"):
		return SelectorTestID
	case selector[0] == '[' && containsAttr(selector, "data-cy"):
		return SelectorDataCy
	case selector[0:2] == "//":
		return SelectorXPath
	case containsAttr(selector, "role="):
		return SelectorRole
	case containsAttr(selector, "placeholder="):
		return SelectorPlaceholder
	default:
		return SelectorCSS
	}
}

func containsAttr(selector, attr string) bool {
	return len(selector) >= len(attr) && findSubstring(selector, attr)
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func (r *Recorder) capturePageState(ctx context.Context) *PageState {
	status, err := r.client.GetSessionStatus(ctx, r.sessionID)
	if err != nil {
		return nil
	}

	state := &PageState{
		URL:        status.CurrentURL,
		Title:      status.PageTitle,
		CapturedAt: time.Now(),
	}

	// Optionally capture screenshot
	if r.config.RecordScreenshots {
		screenshot, _ := r.client.TakeScreenshot(ctx, r.sessionID, false, "png")
		state.Screenshot = screenshot
	}

	// Optionally capture DOM snapshot
	if r.config.RecordDOMSnapshots {
		_, html, _ := r.client.GetDOMSnapshot(ctx, r.sessionID, "body", 10)
		state.DOMSnapshot = html
	}

	return state
}

func (r *Recorder) captureNetworkCalls(ctx context.Context) []NetworkRequest {
	captured, err := r.client.GetCapturedRequests(ctx, r.sessionID, true)
	if err != nil {
		return nil
	}

	requests := make([]NetworkRequest, 0, len(captured))
	for _, req := range captured {
		netReq := NetworkRequest{
			ID:        uuid.New().String(),
			Method:    req.Method,
			URL:       req.URL,
			Timestamp: time.Now(),
		}
		if req.Response != nil {
			netReq.StatusCode = int(req.Response.StatusCode)
			netReq.ResponseBody = req.Response.Body
		}
		requests = append(requests, netReq)
	}

	return requests
}
