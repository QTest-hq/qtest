package dsl

// TestDSL represents a test in the intermediate DSL format
type TestDSL struct {
	Version     string            `json:"version" yaml:"version"`
	ID          string            `json:"id" yaml:"id"`
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	Type        TestType          `json:"type" yaml:"type"`
	Target      TestTarget        `json:"target" yaml:"target"`
	Lifecycle   *Lifecycle        `json:"lifecycle,omitempty" yaml:"lifecycle,omitempty"`
	Resources   []Resource        `json:"resources,omitempty" yaml:"resources,omitempty"`
	Isolation   *Isolation        `json:"isolation,omitempty" yaml:"isolation,omitempty"`
	Steps       []TestStep        `json:"steps" yaml:"steps"`
	Metadata    map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// TestType represents the type of test
type TestType string

const (
	TestTypeUnit        TestType = "unit"
	TestTypeIntegration TestType = "integration"
	TestTypeAPI         TestType = "api"
	TestTypeE2E         TestType = "e2e"
)

// TestTarget identifies what the test is testing
type TestTarget struct {
	File     string   `json:"file" yaml:"file"`
	Function string   `json:"function,omitempty" yaml:"function,omitempty"`
	Class    string   `json:"class,omitempty" yaml:"class,omitempty"`
	Method   string   `json:"method,omitempty" yaml:"method,omitempty"`
	Endpoint string   `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Tags     []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// Lifecycle defines setup and teardown behavior
type Lifecycle struct {
	Scope      LifecycleScope `json:"scope" yaml:"scope"`
	BeforeAll  []Action       `json:"before_all,omitempty" yaml:"before_all,omitempty"`
	BeforeEach []Action       `json:"before_each,omitempty" yaml:"before_each,omitempty"`
	AfterEach  []Action       `json:"after_each,omitempty" yaml:"after_each,omitempty"`
	AfterAll   []Action       `json:"after_all,omitempty" yaml:"after_all,omitempty"`
}

// LifecycleScope defines the scope of lifecycle hooks
type LifecycleScope string

const (
	ScopeTest  LifecycleScope = "test"
	ScopeSuite LifecycleScope = "suite"
	ScopeFile  LifecycleScope = "file"
)

// Action represents a setup/teardown action
type Action struct {
	Type   string                 `json:"type" yaml:"type"`
	Params map[string]interface{} `json:"params,omitempty" yaml:"params,omitempty"`
}

// Resource declares external dependencies
type Resource struct {
	Type   ResourceType           `json:"type" yaml:"type"`
	Name   string                 `json:"name" yaml:"name"`
	Config map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
}

// ResourceType represents types of resources
type ResourceType string

const (
	ResourceDatabase ResourceType = "database"
	ResourceCache    ResourceType = "cache"
	ResourceQueue    ResourceType = "queue"
	ResourceService  ResourceType = "service"
	ResourceFile     ResourceType = "file"
)

// Isolation defines test isolation requirements
type Isolation struct {
	Level    IsolationLevel `json:"level" yaml:"level"`
	Parallel bool           `json:"parallel" yaml:"parallel"`
	Timeout  string         `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

// IsolationLevel defines how isolated the test should be
type IsolationLevel string

const (
	IsolationNone        IsolationLevel = "none"
	IsolationTransaction IsolationLevel = "transaction"
	IsolationContainer   IsolationLevel = "container"
	IsolationProcess     IsolationLevel = "process"
)

// TestStep represents a single step in a test
type TestStep struct {
	ID          string                 `json:"id" yaml:"id"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Action      StepAction             `json:"action" yaml:"action"`
	Input       map[string]interface{} `json:"input,omitempty" yaml:"input,omitempty"`
	Expected    *Expected              `json:"expected,omitempty" yaml:"expected,omitempty"`
	Store       map[string]string      `json:"store,omitempty" yaml:"store,omitempty"`
}

// StepAction defines what the step does
type StepAction struct {
	Type   ActionType             `json:"type" yaml:"type"`
	Target string                 `json:"target,omitempty" yaml:"target,omitempty"`
	Method string                 `json:"method,omitempty" yaml:"method,omitempty"`
	Args   []interface{}          `json:"args,omitempty" yaml:"args,omitempty"`
	Params map[string]interface{} `json:"params,omitempty" yaml:"params,omitempty"`
}

// ActionType represents the type of action in a step
type ActionType string

const (
	ActionCall       ActionType = "call"        // Call a function
	ActionHTTP       ActionType = "http"        // Make HTTP request
	ActionAssert     ActionType = "assert"      // Assert condition
	ActionSetup      ActionType = "setup"       // Setup action
	ActionTeardown   ActionType = "teardown"    // Teardown action
	ActionNavigate   ActionType = "navigate"    // E2E: navigate to URL
	ActionClick      ActionType = "click"       // E2E: click element
	ActionType_      ActionType = "type"        // E2E: type text
	ActionWait       ActionType = "wait"        // E2E: wait for condition
	ActionScreenshot ActionType = "screenshot"  // E2E: take screenshot
)

// Expected defines the expected outcome of a step
type Expected struct {
	Value      interface{}            `json:"value,omitempty" yaml:"value,omitempty"`
	Type       string                 `json:"type,omitempty" yaml:"type,omitempty"`
	Contains   interface{}            `json:"contains,omitempty" yaml:"contains,omitempty"`
	Matches    string                 `json:"matches,omitempty" yaml:"matches,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty" yaml:"properties,omitempty"`
	Error      *ExpectedError         `json:"error,omitempty" yaml:"error,omitempty"`
}

// ExpectedError defines expected error conditions
type ExpectedError struct {
	Type    string `json:"type,omitempty" yaml:"type,omitempty"`
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
}
