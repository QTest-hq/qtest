# Data Schemas

This document defines the core data structures used throughout QTest. All schemas are defined as Go structs with JSON tags for serialization.

## 1. Universal System Model

The System Model is the central representation of a codebase or website.

### 1.1 SystemModel

```go
// SystemModel represents a complete analysis of a repository or website
type SystemModel struct {
    ID           string            `json:"id"`
    Version      int               `json:"version"`
    RepositoryID string            `json:"repository_id"`
    CreatedAt    time.Time         `json:"created_at"`

    // Source metadata
    Source       SourceInfo        `json:"source"`

    // Extracted components
    Endpoints    []Endpoint        `json:"endpoints"`
    Functions    []Function        `json:"functions"`
    Classes      []Class           `json:"classes"`
    Entities     []Entity          `json:"entities"`

    // Relationships
    Dependencies DependencyGraph   `json:"dependencies"`
    Flows        []Flow            `json:"flows"`

    // Analysis metadata
    Languages    []LanguageInfo    `json:"languages"`
    Frameworks   []FrameworkInfo   `json:"frameworks"`
    Metrics      ModelMetrics      `json:"metrics"`
}

// SourceInfo describes the source of the system model
type SourceInfo struct {
    Type       SourceType `json:"type"`        // "repository" | "website"
    URL        string     `json:"url"`
    Branch     string     `json:"branch,omitempty"`
    CommitSHA  string     `json:"commit_sha,omitempty"`
    AnalyzedAt time.Time  `json:"analyzed_at"`
}

type SourceType string

const (
    SourceTypeRepository SourceType = "repository"
    SourceTypeWebsite    SourceType = "website"
)

// ModelMetrics contains aggregate metrics about the model
type ModelMetrics struct {
    TotalFiles       int     `json:"total_files"`
    TotalFunctions   int     `json:"total_functions"`
    TotalEndpoints   int     `json:"total_endpoints"`
    TotalClasses     int     `json:"total_classes"`
    TotalLinesOfCode int     `json:"total_lines_of_code"`
    CyclomaticAvg    float64 `json:"cyclomatic_avg"`
}
```

### 1.2 Function

```go
// Function represents a standalone function or module-level callable
type Function struct {
    ID          string       `json:"id"`
    Name        string       `json:"name"`
    Module      string       `json:"module"`       // File path
    StartLine   int          `json:"start_line"`
    EndLine     int          `json:"end_line"`

    // Signature
    Parameters  []Parameter  `json:"parameters"`
    ReturnType  TypeInfo     `json:"return_type"`
    Async       bool         `json:"async"`
    Exported    bool         `json:"exported"`

    // Behavioral analysis
    Branches    []Branch     `json:"branches"`
    Calls       []CallSite   `json:"calls"`
    SideEffects []SideEffect `json:"side_effects"`

    // Metadata
    JSDoc       string       `json:"jsdoc,omitempty"`
    Complexity  int          `json:"complexity"`
    RiskScore   float64      `json:"risk_score"`
}

// Parameter represents a function parameter
type Parameter struct {
    Name         string   `json:"name"`
    Type         TypeInfo `json:"type"`
    Optional     bool     `json:"optional"`
    DefaultValue string   `json:"default_value,omitempty"`
}

// TypeInfo represents type information
type TypeInfo struct {
    Name       string     `json:"name"`
    Kind       TypeKind   `json:"kind"`
    Nullable   bool       `json:"nullable"`
    Generic    []TypeInfo `json:"generic,omitempty"`
    Properties []Property `json:"properties,omitempty"` // For object types
}

type TypeKind string

const (
    TypeKindPrimitive TypeKind = "primitive"
    TypeKindArray     TypeKind = "array"
    TypeKindObject    TypeKind = "object"
    TypeKindUnion     TypeKind = "union"
    TypeKindFunction  TypeKind = "function"
    TypeKindUnknown   TypeKind = "unknown"
)

// Property represents an object property
type Property struct {
    Name     string   `json:"name"`
    Type     TypeInfo `json:"type"`
    Optional bool     `json:"optional"`
}

// Branch represents a branching condition in code
type Branch struct {
    Type      BranchType `json:"type"`
    Condition string     `json:"condition"`
    Line      int        `json:"line"`
}

type BranchType string

const (
    BranchTypeIf     BranchType = "if"
    BranchTypeElse   BranchType = "else"
    BranchTypeSwitch BranchType = "switch"
    BranchTypeTernary BranchType = "ternary"
)

// CallSite represents a function call within code
type CallSite struct {
    Target   string `json:"target"`    // Called function/method
    Line     int    `json:"line"`
    IsAsync  bool   `json:"is_async"`
    IsAwait  bool   `json:"is_await"`
}

// SideEffect represents a side effect in code
type SideEffect struct {
    Type    SideEffectType `json:"type"`
    Target  string         `json:"target"`
    Details string         `json:"details,omitempty"`
}

type SideEffectType string

const (
    SideEffectDB        SideEffectType = "database"
    SideEffectNetwork   SideEffectType = "network"
    SideEffectFileSystem SideEffectType = "filesystem"
    SideEffectConsole   SideEffectType = "console"
    SideEffectState     SideEffectType = "state"
)
```

### 1.3 Endpoint

```go
// Endpoint represents an HTTP endpoint
type Endpoint struct {
    ID          string       `json:"id"`
    Method      HTTPMethod   `json:"method"`
    Path        string       `json:"path"`

    // Location
    Module      string       `json:"module"`
    Handler     string       `json:"handler"`
    StartLine   int          `json:"start_line"`

    // Schema
    Parameters  []RouteParam `json:"parameters"`
    RequestBody *SchemaInfo  `json:"request_body,omitempty"`
    Responses   []Response   `json:"responses"`

    // Middleware
    Middleware  []string     `json:"middleware"`
    Auth        *AuthInfo    `json:"auth,omitempty"`

    // Metadata
    Description string       `json:"description,omitempty"`
    Tags        []string     `json:"tags,omitempty"`
    RiskScore   float64      `json:"risk_score"`
}

type HTTPMethod string

const (
    HTTPMethodGET     HTTPMethod = "GET"
    HTTPMethodPOST    HTTPMethod = "POST"
    HTTPMethodPUT     HTTPMethod = "PUT"
    HTTPMethodPATCH   HTTPMethod = "PATCH"
    HTTPMethodDELETE  HTTPMethod = "DELETE"
    HTTPMethodHEAD    HTTPMethod = "HEAD"
    HTTPMethodOPTIONS HTTPMethod = "OPTIONS"
)

// RouteParam represents a route parameter
type RouteParam struct {
    Name     string     `json:"name"`
    In       ParamIn    `json:"in"` // "path" | "query" | "header"
    Type     TypeInfo   `json:"type"`
    Required bool       `json:"required"`
}

type ParamIn string

const (
    ParamInPath   ParamIn = "path"
    ParamInQuery  ParamIn = "query"
    ParamInHeader ParamIn = "header"
)

// SchemaInfo represents a request/response schema
type SchemaInfo struct {
    ContentType string     `json:"content_type"`
    Schema      TypeInfo   `json:"schema"`
    Example     any        `json:"example,omitempty"`
}

// Response represents an endpoint response
type Response struct {
    Status      int        `json:"status"`
    Description string     `json:"description,omitempty"`
    Schema      *SchemaInfo `json:"schema,omitempty"`
}

// AuthInfo represents authentication requirements
type AuthInfo struct {
    Type     AuthType `json:"type"`
    Required bool     `json:"required"`
    Scopes   []string `json:"scopes,omitempty"`
}

type AuthType string

const (
    AuthTypeBearer  AuthType = "bearer"
    AuthTypeBasic   AuthType = "basic"
    AuthTypeAPIKey  AuthType = "api_key"
    AuthTypeCookie  AuthType = "cookie"
    AuthTypeOAuth2  AuthType = "oauth2"
)
```

### 1.4 Class

```go
// Class represents a class or interface definition
type Class struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Module      string    `json:"module"`
    StartLine   int       `json:"start_line"`
    EndLine     int       `json:"end_line"`

    // Structure
    Kind        ClassKind `json:"kind"`
    Extends     string    `json:"extends,omitempty"`
    Implements  []string  `json:"implements,omitempty"`

    // Members
    Properties  []ClassProperty `json:"properties"`
    Methods     []Method        `json:"methods"`
    Constructor *Method         `json:"constructor,omitempty"`

    // Metadata
    Abstract    bool      `json:"abstract"`
    Exported    bool      `json:"exported"`
    Decorators  []string  `json:"decorators,omitempty"`
}

type ClassKind string

const (
    ClassKindClass     ClassKind = "class"
    ClassKindInterface ClassKind = "interface"
    ClassKindType      ClassKind = "type"
    ClassKindEnum      ClassKind = "enum"
)

// ClassProperty represents a class property
type ClassProperty struct {
    Name       string   `json:"name"`
    Type       TypeInfo `json:"type"`
    Visibility string   `json:"visibility"` // "public" | "private" | "protected"
    Static     bool     `json:"static"`
    Readonly   bool     `json:"readonly"`
}

// Method represents a class method
type Method struct {
    ID          string      `json:"id"`
    Name        string      `json:"name"`
    Visibility  string      `json:"visibility"`
    Static      bool        `json:"static"`
    Async       bool        `json:"async"`

    Parameters  []Parameter `json:"parameters"`
    ReturnType  TypeInfo    `json:"return_type"`

    Branches    []Branch    `json:"branches"`
    Calls       []CallSite  `json:"calls"`
    SideEffects []SideEffect `json:"side_effects"`

    StartLine   int         `json:"start_line"`
    EndLine     int         `json:"end_line"`
    Complexity  int         `json:"complexity"`
}
```

### 1.5 Flow (E2E)

```go
// Flow represents a user flow discovered from website crawling
type Flow struct {
    ID          string      `json:"id"`
    Name        string      `json:"name"`
    Description string      `json:"description"`
    StartURL    string      `json:"start_url"`

    // Steps in the flow
    Steps       []FlowStep  `json:"steps"`

    // Network activity during flow
    APIcalls    []APICall   `json:"api_calls"`

    // Metadata
    Duration    int         `json:"duration_ms"`
    Confidence  float64     `json:"confidence"` // How confident we are in flow detection
}

// FlowStep represents a single step in a user flow
type FlowStep struct {
    Order       int           `json:"order"`
    Action      ActionType    `json:"action"`
    Selector    *Selector     `json:"selector,omitempty"`
    Value       string        `json:"value,omitempty"`
    URL         string        `json:"url,omitempty"`
    Screenshot  string        `json:"screenshot,omitempty"` // Base64 or path
    DOMSnapshot string        `json:"dom_snapshot,omitempty"`
}

type ActionType string

const (
    ActionTypeGoto   ActionType = "goto"
    ActionTypeClick  ActionType = "click"
    ActionTypeFill   ActionType = "fill"
    ActionTypeSelect ActionType = "select"
    ActionTypeWait   ActionType = "wait"
    ActionTypeScroll ActionType = "scroll"
)

// Selector represents an element selector
type Selector struct {
    CSS    string `json:"css,omitempty"`
    XPath  string `json:"xpath,omitempty"`
    TestID string `json:"test_id,omitempty"`
    Text   string `json:"text,omitempty"`
}

// APICall represents an API call captured during flow
type APICall struct {
    Method   HTTPMethod        `json:"method"`
    URL      string            `json:"url"`
    Headers  map[string]string `json:"headers"`
    Body     string            `json:"body,omitempty"`
    Response APIResponse       `json:"response"`
    Duration int               `json:"duration_ms"`
}

// APIResponse represents a captured API response
type APIResponse struct {
    Status  int               `json:"status"`
    Headers map[string]string `json:"headers"`
    Body    string            `json:"body,omitempty"`
}
```

### 1.6 Dependencies

```go
// DependencyGraph represents the dependency relationships
type DependencyGraph struct {
    // Module-level imports
    Imports map[string][]ImportInfo `json:"imports"`

    // Service dependencies (DI)
    Services map[string][]ServiceDep `json:"services"`

    // External packages
    Packages []PackageInfo `json:"packages"`
}

// ImportInfo represents a module import
type ImportInfo struct {
    Source   string   `json:"source"`   // Import path
    Symbols  []string `json:"symbols"`  // Imported symbols
    Default  string   `json:"default,omitempty"` // Default import name
    Line     int      `json:"line"`
}

// ServiceDep represents a service dependency
type ServiceDep struct {
    Provider string `json:"provider"` // Class/function providing service
    Consumer string `json:"consumer"` // Class/function consuming service
    Type     string `json:"type"`     // Service type/interface
}

// PackageInfo represents an external package
type PackageInfo struct {
    Name    string `json:"name"`
    Version string `json:"version"`
    Dev     bool   `json:"dev"`
}
```

## 2. Test Planning

### 2.1 TestPlan

```go
// TestPlan represents a complete test plan for a repository
type TestPlan struct {
    ID            string        `json:"id"`
    SystemModelID string        `json:"system_model_id"`
    CreatedAt     time.Time     `json:"created_at"`

    // Targets grouped by test type
    UnitTargets   []TestTarget  `json:"unit_targets"`
    IntegTargets  []TestTarget  `json:"integration_targets"`
    APITargets    []TestTarget  `json:"api_targets"`
    E2ETargets    []TestTarget  `json:"e2e_targets"`

    // Configuration
    Config        PlanConfig    `json:"config"`

    // Estimates
    Estimates     PlanEstimates `json:"estimates"`
}

// TestTarget represents a target for test generation
type TestTarget struct {
    ID           string        `json:"id"`
    Kind         TargetKind    `json:"kind"` // "function" | "method" | "endpoint" | "flow"
    Reference    string        `json:"reference"` // Reference to system model entity
    Priority     int           `json:"priority"`  // 1-10, higher = more important
    RiskScore    float64       `json:"risk_score"`

    // Suggested test cases
    SuggestedCases []TestCase  `json:"suggested_cases"`
}

type TargetKind string

const (
    TargetKindFunction TargetKind = "function"
    TargetKindMethod   TargetKind = "method"
    TargetKindEndpoint TargetKind = "endpoint"
    TargetKindFlow     TargetKind = "flow"
)

// TestCase represents a suggested test case
type TestCase struct {
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Type        CaseType `json:"type"` // "happy_path" | "edge_case" | "error_case"
    Inputs      any      `json:"inputs,omitempty"`
    Expected    any      `json:"expected,omitempty"`
}

type CaseType string

const (
    CaseTypeHappyPath CaseType = "happy_path"
    CaseTypeEdgeCase  CaseType = "edge_case"
    CaseTypeErrorCase CaseType = "error_case"
)

// PlanConfig contains planning configuration
type PlanConfig struct {
    MaxUnitTests     int      `json:"max_unit_tests"`
    MaxIntegTests    int      `json:"max_integration_tests"`
    MaxAPITests      int      `json:"max_api_tests"`
    MaxE2ETests      int      `json:"max_e2e_tests"`
    ExcludePatterns  []string `json:"exclude_patterns"`
    FocusModules     []string `json:"focus_modules,omitempty"`
    Framework        string   `json:"framework"`
}

// PlanEstimates contains estimated resources
type PlanEstimates struct {
    TotalTests      int `json:"total_tests"`
    EstimatedTokens int `json:"estimated_tokens"`
    EstimatedTimeMs int `json:"estimated_time_ms"`
}
```

## 3. Generation Run

### 3.1 GenerationRun

```go
// GenerationRun represents a single test generation execution
type GenerationRun struct {
    ID            string         `json:"id"`
    RepositoryID  string         `json:"repository_id"`
    SystemModelID string         `json:"system_model_id"`
    TestPlanID    string         `json:"test_plan_id"`

    // Status
    Status        RunStatus      `json:"status"`
    StartedAt     time.Time      `json:"started_at"`
    CompletedAt   *time.Time     `json:"completed_at,omitempty"`

    // Progress
    Progress      RunProgress    `json:"progress"`

    // Results
    Results       []TestResult   `json:"results"`

    // Metrics
    Metrics       RunMetrics     `json:"metrics"`

    // Configuration
    MutationConfig MutationConfig `json:"mutation_config"`
    LLMConfig      LLMConfig      `json:"llm_config"`

    // Errors
    Errors        []RunError     `json:"errors,omitempty"`
}

// MutationConfig controls mutation testing behavior for a run
type MutationConfig struct {
    Mode              MutationMode `json:"mode"`               // off, sampled, thorough
    MutantsPerTarget  int          `json:"mutants_per_target"` // e.g., 3-5
    MaxRuntimeSeconds int          `json:"max_runtime_seconds"`
    MutatedEntities   []string     `json:"mutated_entities"`   // function IDs to mutate (incremental)
}

type MutationMode string

const (
    MutationModeOff      MutationMode = "off"
    MutationModeSampled  MutationMode = "sampled"  // Default: 3-5 mutants, 2 min budget
    MutationModeThorough MutationMode = "thorough" // Nightly: 10+ mutants, 15 min budget
)

// LLMConfig controls LLM behavior for a run
type LLMConfig struct {
    PreferredProvider LLMProvider `json:"preferred_provider"`
    MaxTokenBudget    int         `json:"max_token_budget"`
    AllowTier3        bool        `json:"allow_tier3"` // Allow expensive models
}

type RunStatus string

const (
    RunStatusPending    RunStatus = "pending"
    RunStatusIngesting  RunStatus = "ingesting"
    RunStatusModeling   RunStatus = "modeling"
    RunStatusPlanning   RunStatus = "planning"
    RunStatusGenerating RunStatus = "generating"
    RunStatusValidating RunStatus = "validating"
    RunStatusCompleted  RunStatus = "completed"
    RunStatusFailed     RunStatus = "failed"
)

// RunProgress tracks generation progress
type RunProgress struct {
    Stage           string  `json:"stage"`
    CurrentTarget   string  `json:"current_target,omitempty"`
    TargetsTotal    int     `json:"targets_total"`
    TargetsComplete int     `json:"targets_complete"`
    Percentage      float64 `json:"percentage"`
}

// RunMetrics contains metrics for a generation run
type RunMetrics struct {
    // Test counts
    TestsGenerated    int `json:"tests_generated"`
    TestsValidated    int `json:"tests_validated"`
    TestsRejected     int `json:"tests_rejected"`

    // Quality
    MutationScore     float64 `json:"mutation_score"`
    CoverageDelta     float64 `json:"coverage_delta"`

    // Resources
    TokensUsed        int `json:"tokens_used"`
    DurationMs        int `json:"duration_ms"`

    // Breakdown by type
    UnitTestsGenerated int `json:"unit_tests_generated"`
    APITestsGenerated  int `json:"api_tests_generated"`
    E2ETestsGenerated  int `json:"e2e_tests_generated"`
}

// RunError represents an error during generation
type RunError struct {
    Stage   string    `json:"stage"`
    Target  string    `json:"target,omitempty"`
    Message string    `json:"message"`
    Code    string    `json:"code"`
    Time    time.Time `json:"time"`
}
```

### 3.2 TestResult

```go
// TestResult represents a generated test and its validation status
type TestResult struct {
    ID            string           `json:"id"`
    RunID         string           `json:"run_id"`
    TargetID      string           `json:"target_id"`

    // Generated content
    DSL           TestDSL          `json:"dsl"`
    GeneratedCode string           `json:"generated_code"`
    FilePath      string           `json:"file_path"`

    // Validation status
    Status        TestResultStatus `json:"status"`

    // Gate results
    CompileGate   *GateResult      `json:"compile_gate,omitempty"`
    RuntimeGate   *GateResult      `json:"runtime_gate,omitempty"`
    MutationGate  *MutationResult  `json:"mutation_gate,omitempty"`

    // Rejection info (formalized enum)
    RejectionReason RejectionReason `json:"rejection_reason,omitempty"`

    // Timing
    GeneratedAt   time.Time        `json:"generated_at"`
    ValidatedAt   *time.Time       `json:"validated_at,omitempty"`
}

type TestResultStatus string

const (
    TestResultStatusGenerated TestResultStatus = "generated"
    TestResultStatusValidated TestResultStatus = "validated"
    TestResultStatusRejected  TestResultStatus = "rejected"
    TestResultStatusFlaky     TestResultStatus = "flaky"
)

// RejectionReason provides detailed categorization of why a test was rejected
type RejectionReason string

const (
    RejectionNone             RejectionReason = ""                    // Not rejected
    RejectionCompileError     RejectionReason = "COMPILE_ERROR"       // Failed to compile/typecheck
    RejectionRuntimeFail      RejectionReason = "RUNTIME_FAIL_ORIGINAL" // Failed when run against original code
    RejectionMutationWeak     RejectionReason = "MUTATION_WEAK"       // Passed but killed 0 mutants
    RejectionFlaky            RejectionReason = "FLAKY"               // Inconsistent pass/fail
    RejectionTimeout          RejectionReason = "TIMEOUT"             // Exceeded time budget
    RejectionCriticRejection  RejectionReason = "CRITIC_REJECTION"    // LLM critic determined test is low quality
)

// GenerationRunSummary provides a high-level view of run results for UX
type GenerationRunSummary struct {
    RunID               string                    `json:"run_id"`
    TestsRequested      int                       `json:"tests_requested"`
    TestsGenerated      int                       `json:"tests_generated"`
    TestsAccepted       int                       `json:"tests_accepted"`
    TestsRejectedByGate map[RejectionReason]int   `json:"tests_rejected_by_gate"`
    MutationScore       float64                   `json:"mutation_score"`
    CoverageDelta       float64                   `json:"coverage_delta"`

    // For PR description / UI display
    SummaryText         string                    `json:"summary_text"`
}

// GateResult represents result of a quality gate
type GateResult struct {
    Passed  bool      `json:"passed"`
    Message string    `json:"message,omitempty"`
    Details any       `json:"details,omitempty"`
    Time    time.Time `json:"time"`
}

// TestDSL represents the DSL structure (matches test-dsl-spec.md)
type TestDSL struct {
    Version   string      `json:"version"`
    Test      TestMeta    `json:"test"`
    Target    Target      `json:"target"`
    Setup     []SetupStep `json:"setup,omitempty"`
    Teardown  []string    `json:"teardown,omitempty"`
    Input     any         `json:"input,omitempty"`
    Expect    any         `json:"expect"`
    Steps     []Step      `json:"steps,omitempty"` // For E2E
    Lifecycle *Lifecycle  `json:"lifecycle,omitempty"`
    Resources *Resources  `json:"resources,omitempty"`
    Isolation *Isolation  `json:"isolation,omitempty"`
}

// TestMeta contains test metadata
type TestMeta struct {
    ID          string `json:"id"`
    Type        string `json:"type"`
    Level       string `json:"level"`
    Description string `json:"description"`
}

// Target identifies what is being tested
type Target struct {
    Kind    string `json:"kind"`
    Module  string `json:"module,omitempty"`
    Name    string `json:"name,omitempty"`
    Class   string `json:"class,omitempty"`
    Method  string `json:"method,omitempty"`
    Path    string `json:"path,omitempty"`
    Async   bool   `json:"async,omitempty"`
}

// Lifecycle defines test lifecycle hooks
type Lifecycle struct {
    Scope    string   `json:"scope"`
    Setup    []string `json:"setup,omitempty"`
    Teardown []string `json:"teardown,omitempty"`
}

// Resources defines required resources
type Resources struct {
    DB       *DBResource       `json:"db,omitempty"`
    Cache    *CacheResource    `json:"cache,omitempty"`
    Services []ServiceResource `json:"services,omitempty"`
}

type DBResource struct {
    Type       string `json:"type"`
    Mode       string `json:"mode"`
    Migrations bool   `json:"migrations"`
}

type CacheResource struct {
    Type string `json:"type"`
    Mode string `json:"mode"`
}

type ServiceResource struct {
    Name  string `json:"name"`
    Type  string `json:"type"`
    Image string `json:"image,omitempty"`
    Port  int    `json:"port,omitempty"`
}

// Isolation defines test isolation settings
type Isolation struct {
    Level        string `json:"level"`
    ParallelSafe bool   `json:"parallel_safe"`
}
```

## 4. Mutation Testing

### 4.1 MutationResult

```go
// MutationResult represents mutation testing results for a test
type MutationResult struct {
    TestID        string    `json:"test_id"`
    TargetID      string    `json:"target_id"`

    // Overall result
    Passed        bool      `json:"passed"`
    Score         float64   `json:"score"` // Mutants killed / total

    // Mutant details
    MutantsTotal  int       `json:"mutants_total"`
    MutantsKilled int       `json:"mutants_killed"`
    MutantsSurvived int     `json:"mutants_survived"`
    MutantsTimeout  int     `json:"mutants_timeout"`

    // Individual mutants
    Mutants       []Mutant  `json:"mutants"`

    // Timing
    DurationMs    int       `json:"duration_ms"`
    CompletedAt   time.Time `json:"completed_at"`
}

// Mutant represents a single code mutation
type Mutant struct {
    ID          string      `json:"id"`
    Type        MutantType  `json:"type"`
    Location    Location    `json:"location"`
    Original    string      `json:"original"`
    Mutated     string      `json:"mutated"`
    Status      MutantStatus `json:"status"`
}

type MutantType string

const (
    MutantTypeComparisonOperator  MutantType = "comparison_operator"
    MutantTypeArithmeticOperator  MutantType = "arithmetic_operator"
    MutantTypeBooleanNegation     MutantType = "boolean_negation"
    MutantTypeReturnValue         MutantType = "return_value"
    MutantTypeBranchRemoval       MutantType = "branch_removal"
    MutantTypeMethodCallRemoval   MutantType = "method_call_removal"
)

type MutantStatus string

const (
    MutantStatusKilled   MutantStatus = "killed"
    MutantStatusSurvived MutantStatus = "survived"
    MutantStatusTimeout  MutantStatus = "timeout"
    MutantStatusError    MutantStatus = "error"
)

// Location represents a code location
type Location struct {
    File   string `json:"file"`
    Line   int    `json:"line"`
    Column int    `json:"column"`
}
```

## 5. Repository & Integration

### 5.1 Repository

```go
// Repository represents a connected repository
type Repository struct {
    ID            string          `json:"id"`
    UserID        string          `json:"user_id"`

    // GitHub info
    GitHubID      int64           `json:"github_id"`
    FullName      string          `json:"full_name"` // "owner/repo"
    URL           string          `json:"url"`
    DefaultBranch string          `json:"default_branch"`
    Private       bool            `json:"private"`

    // Configuration
    Config        RepoConfig      `json:"config"`

    // Status
    Status        RepoStatus      `json:"status"`
    LastAnalyzed  *time.Time      `json:"last_analyzed,omitempty"`

    // Timestamps
    ConnectedAt   time.Time       `json:"connected_at"`
    UpdatedAt     time.Time       `json:"updated_at"`
}

// RepoConfig contains repository configuration
type RepoConfig struct {
    AutoMaintain     bool     `json:"auto_maintain"`
    TriggerOnPush    bool     `json:"trigger_on_push"`
    TriggerOnPR      bool     `json:"trigger_on_pr"`
    TargetBranch     string   `json:"target_branch"`
    TestFramework    string   `json:"test_framework"`
    ExcludePatterns  []string `json:"exclude_patterns"`
    TokenBudget      int      `json:"token_budget"`
}

type RepoStatus string

const (
    RepoStatusActive    RepoStatus = "active"
    RepoStatusInactive  RepoStatus = "inactive"
    RepoStatusAnalyzing RepoStatus = "analyzing"
    RepoStatusError     RepoStatus = "error"
)
```

### 5.2 PullRequest

```go
// PullRequest represents a generated PR
type PullRequest struct {
    ID            string       `json:"id"`
    RunID         string       `json:"run_id"`
    RepositoryID  string       `json:"repository_id"`

    // GitHub info
    GitHubNumber  int          `json:"github_number"`
    GitHubURL     string       `json:"github_url"`
    Branch        string       `json:"branch"`

    // Content summary
    TestsAdded    int          `json:"tests_added"`
    FilesChanged  int          `json:"files_changed"`

    // Status
    Status        PRStatus     `json:"status"`
    MergedAt      *time.Time   `json:"merged_at,omitempty"`
    ClosedAt      *time.Time   `json:"closed_at,omitempty"`

    // Timestamps
    CreatedAt     time.Time    `json:"created_at"`
    UpdatedAt     time.Time    `json:"updated_at"`
}

type PRStatus string

const (
    PRStatusOpen   PRStatus = "open"
    PRStatusMerged PRStatus = "merged"
    PRStatusClosed PRStatus = "closed"
)
```

## 6. LLM Router Service

The LLM Router is a provider-agnostic internal service that routes requests to the appropriate model based on task type and budget.

### 6.1 LLM Router Types

```go
// LLMProvider represents supported LLM backends
type LLMProvider string

const (
    LLMProviderOllama    LLMProvider = "ollama"    // Local models (Qwen, DeepSeek, Llama)
    LLMProviderAnthropic LLMProvider = "anthropic" // Claude API
    LLMProviderOpenAI    LLMProvider = "openai"    // GPT API
)

// LLMTaskType categorizes the type of LLM task for routing decisions
type LLMTaskType string

const (
    LLMTaskUnitTest     LLMTaskType = "unit_test"     // Generate unit test DSL
    LLMTaskAPITest      LLMTaskType = "api_test"      // Generate API test DSL
    LLMTaskE2EFlow      LLMTaskType = "e2e_flow"      // Generate E2E test DSL
    LLMTaskCritic       LLMTaskType = "critic"        // Review/strengthen test
    LLMTaskPlanner      LLMTaskType = "planner"       // Plan test strategy
    LLMTaskSummarize    LLMTaskType = "summarize"     // Summarize code/docs
    LLMTaskExtract      LLMTaskType = "extract"       // Extract metadata
)

// LLMRequest is the unified request to the LLM Router Service
type LLMRequest struct {
    TaskType    LLMTaskType       `json:"task_type"`
    ModelTier   LLMTier           `json:"model_tier"`   // TIER1, TIER2, TIER3, or AUTO
    Provider    LLMProvider       `json:"provider"`     // Preferred provider (optional)
    Context     LLMContext        `json:"context"`      // Task-specific context
    MaxTokens   int               `json:"max_tokens"`   // Max output tokens
    Temperature float64           `json:"temperature"`  // 0.0-1.0
}

// LLMContext holds the context for generation
type LLMContext struct {
    // For code-related tasks
    FunctionCode   string     `json:"function_code,omitempty"`
    FunctionMeta   *Function  `json:"function_meta,omitempty"`
    EndpointMeta   *Endpoint  `json:"endpoint_meta,omitempty"`

    // For test tasks
    ExistingTests  []string   `json:"existing_tests,omitempty"`
    TargetBranches []Branch   `json:"target_branches,omitempty"`

    // For critic tasks
    TestToReview   string     `json:"test_to_review,omitempty"`
    MutantsSurvived []Mutant  `json:"mutants_survived,omitempty"`

    // Additional context
    SystemPrompt   string     `json:"system_prompt,omitempty"`
    UserPrompt     string     `json:"user_prompt,omitempty"`
}

// LLMResponse is the response from the LLM Router Service
type LLMResponse struct {
    Content      string      `json:"content"`
    Provider     LLMProvider `json:"provider"`      // Which provider was used
    Model        string      `json:"model"`         // Actual model name
    Tier         LLMTier     `json:"tier"`
    InputTokens  int         `json:"input_tokens"`
    OutputTokens int         `json:"output_tokens"`
    LatencyMs    int         `json:"latency_ms"`
    Cached       bool        `json:"cached"`        // Was response from cache?
}

// LLMRouterConfig configures the LLM Router Service
type LLMRouterConfig struct {
    // Provider configurations
    Providers map[LLMProvider]ProviderConfig `json:"providers"`

    // Routing rules
    DefaultProvider LLMProvider            `json:"default_provider"` // ollama by default
    TierRouting     map[LLMTier][]LLMProvider `json:"tier_routing"`  // Tier -> providers to try

    // Budget limits
    GlobalTokenBudget int `json:"global_token_budget"` // Per day
    Tier3TokenBudget  int `json:"tier3_token_budget"`  // Limit expensive calls
}

// ProviderConfig configures a single LLM provider
type ProviderConfig struct {
    Enabled     bool              `json:"enabled"`
    BaseURL     string            `json:"base_url"`      // e.g., "http://localhost:11434" for Ollama
    APIKey      string            `json:"api_key,omitempty"`
    Models      map[LLMTier]string `json:"models"`       // Tier -> model name
    RateLimit   int               `json:"rate_limit"`    // Requests per minute
    TimeoutSecs int               `json:"timeout_secs"`
}
```

### 6.2 LLMUsage

```go
// LLMUsage tracks LLM API usage
type LLMUsage struct {
    ID           string    `json:"id"`
    RunID        string    `json:"run_id"`
    UserID       string    `json:"user_id"`

    // Model info
    Provider     LLMProvider `json:"provider"` // ollama, anthropic, openai
    Model        string      `json:"model"`    // "qwen2.5:32b", "claude-3-sonnet", etc.
    Tier         LLMTier     `json:"tier"`
    TaskType     LLMTaskType `json:"task_type"`

    // Usage
    InputTokens  int       `json:"input_tokens"`
    OutputTokens int       `json:"output_tokens"`
    TotalTokens  int       `json:"total_tokens"`

    // Cost (in cents) - 0 for local models
    CostCents    int       `json:"cost_cents"`

    // Timing
    LatencyMs    int       `json:"latency_ms"`
    Cached       bool      `json:"cached"`
    CreatedAt    time.Time `json:"created_at"`
}

type LLMTier string

const (
    LLMTierAuto LLMTier = "auto"  // Let router decide
    LLMTier1    LLMTier = "tier1" // Cheap/fast (Qwen 7B, Haiku, GPT-4o-mini)
    LLMTier2    LLMTier = "tier2" // Mid (Qwen 32B, Sonnet, GPT-4o)
    LLMTier3    LLMTier = "tier3" // Frontier (DeepSeek 70B, Opus, GPT-4)
)
```

## 7. Database Tables Summary

```sql
-- Core entities
CREATE TABLE repositories (...);
CREATE TABLE system_models (...);
CREATE TABLE generation_runs (...);
CREATE TABLE test_results (...);
CREATE TABLE pull_requests (...);

-- Analytics & tracking
CREATE TABLE llm_usage (...);
CREATE TABLE mutation_results (...);
CREATE TABLE coverage_snapshots (...);

-- User & auth
CREATE TABLE users (...);
CREATE TABLE organizations (...);
CREATE TABLE api_keys (...);

-- Indexes for common queries
CREATE INDEX idx_runs_repo ON generation_runs(repository_id);
CREATE INDEX idx_runs_status ON generation_runs(status);
CREATE INDEX idx_results_run ON test_results(run_id);
CREATE INDEX idx_usage_user ON llm_usage(user_id);
```

## 8. JSON Examples

### 8.1 System Model Example

```json
{
  "id": "sm_abc123",
  "version": 1,
  "repository_id": "repo_xyz",
  "source": {
    "type": "repository",
    "url": "https://github.com/example/app",
    "branch": "main",
    "commit_sha": "a1b2c3d4"
  },
  "endpoints": [
    {
      "id": "ep_001",
      "method": "POST",
      "path": "/api/users",
      "handler": "UserController.create",
      "request_body": {
        "content_type": "application/json",
        "schema": {
          "name": "object",
          "properties": [
            { "name": "name", "type": { "name": "string" } },
            { "name": "email", "type": { "name": "string" } }
          ]
        }
      }
    }
  ],
  "functions": [
    {
      "id": "fn_001",
      "name": "calculateTotal",
      "module": "src/utils/pricing.ts",
      "parameters": [
        { "name": "items", "type": { "name": "CartItem[]" } },
        { "name": "discount", "type": { "name": "number" } }
      ],
      "return_type": { "name": "number" },
      "branches": [
        { "type": "if", "condition": "discount > 0", "line": 15 },
        { "type": "if", "condition": "items.length === 0", "line": 18 }
      ]
    }
  ]
}
```

### 8.2 Generation Run Example

```json
{
  "id": "run_abc123",
  "repository_id": "repo_xyz",
  "status": "completed",
  "progress": {
    "stage": "completed",
    "targets_total": 50,
    "targets_complete": 50,
    "percentage": 100
  },
  "metrics": {
    "tests_generated": 120,
    "tests_validated": 95,
    "tests_rejected": 25,
    "mutation_score": 0.78,
    "tokens_used": 150000,
    "duration_ms": 180000
  }
}
```
