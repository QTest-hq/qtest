// Package flow provides user flow detection and recording for E2E test generation.
package flow

import (
	"time"
)

// FlowType represents the type of user flow.
type FlowType string

const (
	FlowTypeLogin       FlowType = "login"
	FlowTypeRegistration FlowType = "registration"
	FlowTypeCheckout    FlowType = "checkout"
	FlowTypeSearch      FlowType = "search"
	FlowTypeFormSubmit  FlowType = "form_submit"
	FlowTypeNavigation  FlowType = "navigation"
	FlowTypeCRUD        FlowType = "crud"
	FlowTypeCustom      FlowType = "custom"
)

// ActionType represents the type of user action.
type ActionType string

const (
	ActionTypeClick      ActionType = "click"
	ActionTypeFill       ActionType = "fill"
	ActionTypeSelect     ActionType = "select"
	ActionTypeCheck      ActionType = "check"
	ActionTypeUncheck    ActionType = "uncheck"
	ActionTypeHover      ActionType = "hover"
	ActionTypeScroll     ActionType = "scroll"
	ActionTypeKeypress   ActionType = "keypress"
	ActionTypeNavigate   ActionType = "navigate"
	ActionTypeWait       ActionType = "wait"
	ActionTypeAssert     ActionType = "assert"
	ActionTypeScreenshot ActionType = "screenshot"
)

// SelectorStrategy defines how to locate an element.
type SelectorStrategy string

const (
	SelectorTestID    SelectorStrategy = "test-id"
	SelectorDataCy    SelectorStrategy = "data-cy"
	SelectorID        SelectorStrategy = "id"
	SelectorCSS       SelectorStrategy = "css"
	SelectorXPath     SelectorStrategy = "xpath"
	SelectorText      SelectorStrategy = "text"
	SelectorRole      SelectorStrategy = "role"
	SelectorPlaceholder SelectorStrategy = "placeholder"
	SelectorLabel     SelectorStrategy = "label"
)

// Selector represents an element selector with fallback strategies.
type Selector struct {
	Primary    string           `json:"primary"`
	Strategy   SelectorStrategy `json:"strategy"`
	Fallbacks  []Selector       `json:"fallbacks,omitempty"`
	Confidence float64          `json:"confidence"`
}

// Action represents a single user action in a flow.
type Action struct {
	ID          string            `json:"id"`
	Type        ActionType        `json:"type"`
	Selector    *Selector         `json:"selector,omitempty"`
	Value       string            `json:"value,omitempty"`
	URL         string            `json:"url,omitempty"`
	Key         string            `json:"key,omitempty"`
	Modifiers   []string          `json:"modifiers,omitempty"`
	WaitFor     string            `json:"wait_for,omitempty"`
	Timeout     time.Duration     `json:"timeout,omitempty"`
	Screenshot  bool              `json:"screenshot,omitempty"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
}

// Assertion represents a validation step in a flow.
type Assertion struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"` // visible, text, value, url, count, attribute
	Selector    *Selector         `json:"selector,omitempty"`
	Expected    interface{}       `json:"expected"`
	Attribute   string            `json:"attribute,omitempty"`
	Negated     bool              `json:"negated,omitempty"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// NetworkRequest represents an API call made during a flow.
type NetworkRequest struct {
	ID           string            `json:"id"`
	Method       string            `json:"method"`
	URL          string            `json:"url"`
	Headers      map[string]string `json:"headers,omitempty"`
	Body         string            `json:"body,omitempty"`
	StatusCode   int               `json:"status_code,omitempty"`
	ResponseBody string            `json:"response_body,omitempty"`
	Duration     time.Duration     `json:"duration,omitempty"`
	Timestamp    time.Time         `json:"timestamp"`
}

// FormField represents a field in a detected form.
type FormField struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"` // text, password, email, number, select, checkbox, radio, textarea
	Label       string   `json:"label,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Required    bool     `json:"required"`
	Options     []string `json:"options,omitempty"` // For select/radio
	Selector    Selector `json:"selector"`
	Validation  string   `json:"validation,omitempty"` // Detected validation pattern
}

// Form represents a detected form on a page.
type Form struct {
	ID       string      `json:"id"`
	Action   string      `json:"action,omitempty"`
	Method   string      `json:"method,omitempty"`
	Fields   []FormField `json:"fields"`
	SubmitButton *Selector `json:"submit_button,omitempty"`
	FormType string      `json:"form_type,omitempty"` // login, registration, search, contact, checkout
	Selector Selector    `json:"selector"`
}

// PageState represents the state of a page at a point in time.
type PageState struct {
	URL         string            `json:"url"`
	Title       string            `json:"title"`
	Forms       []Form            `json:"forms,omitempty"`
	Links       []string          `json:"links,omitempty"`
	Buttons     []Selector        `json:"buttons,omitempty"`
	Inputs      []Selector        `json:"inputs,omitempty"`
	Screenshot  []byte            `json:"screenshot,omitempty"`
	DOMSnapshot string            `json:"dom_snapshot,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CapturedAt  time.Time         `json:"captured_at"`
}

// Step represents a single step in a flow (action + optional assertions).
type Step struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	Action        Action           `json:"action"`
	Assertions    []Assertion      `json:"assertions,omitempty"`
	NetworkCalls  []NetworkRequest `json:"network_calls,omitempty"`
	PageStateBefore *PageState     `json:"page_state_before,omitempty"`
	PageStateAfter  *PageState     `json:"page_state_after,omitempty"`
	Duration      time.Duration    `json:"duration,omitempty"`
	Order         int              `json:"order"`
}

// Flow represents a complete user flow.
type Flow struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Type        FlowType          `json:"type"`
	StartURL    string            `json:"start_url"`
	Steps       []Step            `json:"steps"`
	Tags        []string          `json:"tags,omitempty"`
	Priority    int               `json:"priority,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// FlowHint represents a user-provided hint for flow discovery.
type FlowHint struct {
	Type        FlowType          `json:"type"`
	StartURL    string            `json:"start_url,omitempty"`
	Description string            `json:"description,omitempty"`
	Credentials map[string]string `json:"credentials,omitempty"` // For login flows
	Steps       []HintStep        `json:"steps,omitempty"`
	Selectors   map[string]string `json:"selectors,omitempty"` // Named selectors
}

// HintStep represents a hint for a single step in a flow.
type HintStep struct {
	Action      string `json:"action"` // click, fill, select, etc.
	Target      string `json:"target"` // Selector or named reference
	Value       string `json:"value,omitempty"`
	Description string `json:"description,omitempty"`
}

// FlowConfig contains configuration for flow detection and recording.
type FlowConfig struct {
	// Recording options
	RecordScreenshots  bool          `json:"record_screenshots"`
	RecordNetwork      bool          `json:"record_network"`
	RecordDOMSnapshots bool          `json:"record_dom_snapshots"`
	ActionDelay        time.Duration `json:"action_delay"`

	// Detection options
	DetectLoginForms   bool `json:"detect_login_forms"`
	DetectSearchForms  bool `json:"detect_search_forms"`
	DetectCheckoutFlow bool `json:"detect_checkout_flow"`

	// Selector preferences (in order of priority)
	SelectorPreferences []SelectorStrategy `json:"selector_preferences"`

	// Timeouts
	PageLoadTimeout    time.Duration `json:"page_load_timeout"`
	ActionTimeout      time.Duration `json:"action_timeout"`
	NetworkIdleTimeout time.Duration `json:"network_idle_timeout"`

	// LLM options
	UseLLMDiscovery    bool   `json:"use_llm_discovery"`
	LLMModel           string `json:"llm_model,omitempty"`
}

// DefaultFlowConfig returns a default flow configuration.
func DefaultFlowConfig() *FlowConfig {
	return &FlowConfig{
		RecordScreenshots:  false,
		RecordNetwork:      true,
		RecordDOMSnapshots: false,
		ActionDelay:        100 * time.Millisecond,

		DetectLoginForms:   true,
		DetectSearchForms:  true,
		DetectCheckoutFlow: true,

		SelectorPreferences: []SelectorStrategy{
			SelectorTestID,
			SelectorDataCy,
			SelectorID,
			SelectorRole,
			SelectorPlaceholder,
			SelectorCSS,
		},

		PageLoadTimeout:    30 * time.Second,
		ActionTimeout:      10 * time.Second,
		NetworkIdleTimeout: 5 * time.Second,

		UseLLMDiscovery: false,
	}
}

// FlowSet represents a collection of related flows.
type FlowSet struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	BaseURL     string    `json:"base_url"`
	Flows       []Flow    `json:"flows"`
	Hints       []FlowHint `json:"hints,omitempty"`
	Config      *FlowConfig `json:"config,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DetectionResult represents the result of automatic flow detection.
type DetectionResult struct {
	Forms        []Form    `json:"forms"`
	Flows        []Flow    `json:"flows"`
	Suggestions  []FlowHint `json:"suggestions,omitempty"`
	PageStates   []PageState `json:"page_states,omitempty"`
	Errors       []string  `json:"errors,omitempty"`
	DetectedAt   time.Time `json:"detected_at"`
}
