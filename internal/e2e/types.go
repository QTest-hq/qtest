package e2e

import (
	"time"

	"github.com/QTest-hq/qtest/internal/api"
	"github.com/QTest-hq/qtest/internal/flow"
)

// TestFramework represents supported test frameworks.
type TestFramework string

const (
	FrameworkPlaywright TestFramework = "playwright"
	FrameworkCypress    TestFramework = "cypress"
	FrameworkPuppeteer  TestFramework = "puppeteer"
)

// TestLanguage represents supported programming languages.
type TestLanguage string

const (
	LanguageTypeScript TestLanguage = "typescript"
	LanguageJavaScript TestLanguage = "javascript"
	LanguagePython     TestLanguage = "python"
)

// AssertionType represents types of assertions.
type AssertionType string

const (
	AssertVisible     AssertionType = "visible"
	AssertHidden      AssertionType = "hidden"
	AssertText        AssertionType = "text"
	AssertValue       AssertionType = "value"
	AssertEnabled     AssertionType = "enabled"
	AssertDisabled    AssertionType = "disabled"
	AssertURL         AssertionType = "url"
	AssertTitle       AssertionType = "title"
	AssertCount       AssertionType = "count"
	AssertContains    AssertionType = "contains"
	AssertStatusCode  AssertionType = "status_code"
	AssertResponseBody AssertionType = "response_body"
)

// E2ETestSpec represents a complete E2E test specification.
type E2ETestSpec struct {
	ID          string            `json:"id" yaml:"id"`
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
	BaseURL     string            `json:"baseUrl" yaml:"baseUrl"`
	Setup       *TestSetup        `json:"setup,omitempty" yaml:"setup,omitempty"`
	Teardown    *TestTeardown     `json:"teardown,omitempty" yaml:"teardown,omitempty"`
	TestCases   []TestCase        `json:"testCases" yaml:"testCases"`
	Config      *TestConfig       `json:"config,omitempty" yaml:"config,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"createdAt" yaml:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt" yaml:"updatedAt"`
}

// TestSetup represents test setup actions.
type TestSetup struct {
	Actions   []TestAction `json:"actions,omitempty" yaml:"actions,omitempty"`
	Fixtures  []Fixture    `json:"fixtures,omitempty" yaml:"fixtures,omitempty"`
	BeforeAll string       `json:"beforeAll,omitempty" yaml:"beforeAll,omitempty"`
}

// TestTeardown represents test teardown actions.
type TestTeardown struct {
	Actions  []TestAction `json:"actions,omitempty" yaml:"actions,omitempty"`
	AfterAll string       `json:"afterAll,omitempty" yaml:"afterAll,omitempty"`
}

// Fixture represents test data fixtures.
type Fixture struct {
	Name  string      `json:"name" yaml:"name"`
	Type  string      `json:"type" yaml:"type"`
	Value interface{} `json:"value" yaml:"value"`
}

// TestCase represents a single test case.
type TestCase struct {
	ID          string            `json:"id" yaml:"id"`
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
	Steps       []TestStep        `json:"steps" yaml:"steps"`
	Expected    []Assertion       `json:"expected,omitempty" yaml:"expected,omitempty"`
	Timeout     time.Duration     `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Retries     int               `json:"retries,omitempty" yaml:"retries,omitempty"`
	Skip        bool              `json:"skip,omitempty" yaml:"skip,omitempty"`
	Only        bool              `json:"only,omitempty" yaml:"only,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// TestStep represents a single step in a test case.
type TestStep struct {
	ID          string       `json:"id" yaml:"id"`
	Name        string       `json:"name,omitempty" yaml:"name,omitempty"`
	Action      TestAction   `json:"action" yaml:"action"`
	Assertions  []Assertion  `json:"assertions,omitempty" yaml:"assertions,omitempty"`
	Wait        *WaitConfig  `json:"wait,omitempty" yaml:"wait,omitempty"`
	Screenshot  bool         `json:"screenshot,omitempty" yaml:"screenshot,omitempty"`
	Order       int          `json:"order" yaml:"order"`
}

// TestAction represents an action to perform in a test.
type TestAction struct {
	Type      flow.ActionType   `json:"type" yaml:"type"`
	Selector  string            `json:"selector,omitempty" yaml:"selector,omitempty"`
	Value     string            `json:"value,omitempty" yaml:"value,omitempty"`
	URL       string            `json:"url,omitempty" yaml:"url,omitempty"`
	Key       string            `json:"key,omitempty" yaml:"key,omitempty"`
	Modifiers []string          `json:"modifiers,omitempty" yaml:"modifiers,omitempty"`
	Options   map[string]interface{} `json:"options,omitempty" yaml:"options,omitempty"`
}

// Assertion represents an assertion to verify.
type Assertion struct {
	Type     AssertionType `json:"type" yaml:"type"`
	Selector string        `json:"selector,omitempty" yaml:"selector,omitempty"`
	Expected interface{}   `json:"expected" yaml:"expected"`
	Message  string        `json:"message,omitempty" yaml:"message,omitempty"`
	Timeout  time.Duration `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

// WaitConfig represents wait configuration.
type WaitConfig struct {
	Type     string        `json:"type" yaml:"type"` // selector, timeout, networkidle
	Selector string        `json:"selector,omitempty" yaml:"selector,omitempty"`
	State    string        `json:"state,omitempty" yaml:"state,omitempty"` // visible, hidden, attached
	Timeout  time.Duration `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

// TestConfig represents test configuration.
type TestConfig struct {
	Framework     TestFramework `json:"framework" yaml:"framework"`
	Language      TestLanguage  `json:"language" yaml:"language"`
	Headless      bool          `json:"headless" yaml:"headless"`
	SlowMo        int           `json:"slowMo,omitempty" yaml:"slowMo,omitempty"`
	Timeout       time.Duration `json:"timeout" yaml:"timeout"`
	Retries       int           `json:"retries,omitempty" yaml:"retries,omitempty"`
	Parallel      int           `json:"parallel,omitempty" yaml:"parallel,omitempty"`
	Screenshots   string        `json:"screenshots,omitempty" yaml:"screenshots,omitempty"` // on, off, only-on-failure
	Video         string        `json:"video,omitempty" yaml:"video,omitempty"`
	Trace         string        `json:"trace,omitempty" yaml:"trace,omitempty"`
	ViewportWidth int           `json:"viewportWidth,omitempty" yaml:"viewportWidth,omitempty"`
	ViewportHeight int          `json:"viewportHeight,omitempty" yaml:"viewportHeight,omitempty"`
}

// DefaultTestConfig returns default test configuration.
func DefaultTestConfig() *TestConfig {
	return &TestConfig{
		Framework:      FrameworkPlaywright,
		Language:       LanguageTypeScript,
		Headless:       true,
		Timeout:        30 * time.Second,
		Retries:        2,
		Screenshots:    "only-on-failure",
		ViewportWidth:  1280,
		ViewportHeight: 720,
	}
}

// GenerationConfig configures test generation behavior.
type GenerationConfig struct {
	Framework           TestFramework `json:"framework" yaml:"framework"`
	Language            TestLanguage  `json:"language" yaml:"language"`
	OutputDir           string        `json:"outputDir" yaml:"outputDir"`
	GroupByFeature      bool          `json:"groupByFeature" yaml:"groupByFeature"`
	IncludeComments     bool          `json:"includeComments" yaml:"includeComments"`
	GenerateHelpers     bool          `json:"generateHelpers" yaml:"generateHelpers"`
	GenerateFixtures    bool          `json:"generateFixtures" yaml:"generateFixtures"`
	GeneratePageObjects bool          `json:"generatePageObjects" yaml:"generatePageObjects"`
	MaxStepsPerTest     int           `json:"maxStepsPerTest" yaml:"maxStepsPerTest"`
}

// DefaultGenerationConfig returns default generation configuration.
func DefaultGenerationConfig() *GenerationConfig {
	return &GenerationConfig{
		Framework:           FrameworkPlaywright,
		Language:            LanguageTypeScript,
		OutputDir:           "tests",
		GroupByFeature:      true,
		IncludeComments:     true,
		GenerateHelpers:     true,
		GenerateFixtures:    true,
		GeneratePageObjects: false,
		MaxStepsPerTest:     20,
	}
}

// GenerationResult represents the result of test generation.
type GenerationResult struct {
	Files       []GeneratedFile `json:"files"`
	TestCount   int             `json:"testCount"`
	StepCount   int             `json:"stepCount"`
	Warnings    []string        `json:"warnings,omitempty"`
	Errors      []string        `json:"errors,omitempty"`
}

// GeneratedFile represents a generated test file.
type GeneratedFile struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Language string `json:"language"`
	TestCount int   `json:"testCount"`
}

// FlowToTestInput represents input for converting a flow to a test.
type FlowToTestInput struct {
	Flow        *flow.Flow
	Credentials map[string]string
	Assertions  []Assertion
}

// APIToTestInput represents input for converting API endpoints to tests.
type APIToTestInput struct {
	Endpoint    *api.Endpoint
	BaseURL     string
	Auth        *api.AuthRequirement
	TestData    map[string]interface{}
}
