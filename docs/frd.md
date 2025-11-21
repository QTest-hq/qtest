# Functional Requirements Document (FRD)

## 1. Overview

This document defines the functional requirements for QTest, an AI-powered test generation and maintenance platform. It specifies what the system must do, not how it should be implemented.

## 2. System Inputs

### 2.1 Primary Inputs

| Input Type | Description | Required Fields |
|------------|-------------|-----------------|
| GitHub Repository URL | Public or private repo | `url`, optional `branch`, optional `auth_token` |
| Website URL | Any accessible website | `url`, optional `auth_credentials` |
| Configuration | User preferences | `target_frameworks`, `test_types[]`, `exclusions[]` |

### 2.2 Authentication Inputs

| Auth Type | Use Case | Required |
|-----------|----------|----------|
| GitHub OAuth | Access private repos | For private repos |
| GitHub PAT | CLI/API access | Alternative to OAuth |
| Website Credentials | Authenticated website testing | For gated sites |
| API Keys | Third-party service access | As needed |

## 3. System Outputs

### 3.1 Generated Artifacts

| Artifact | Format | Description |
|----------|--------|-------------|
| Unit Tests | Framework-specific code | Tests for individual functions |
| Integration Tests | Framework-specific code | Tests for service interactions |
| API Tests | Framework-specific code | Tests for HTTP endpoints |
| E2E Tests | Playwright code | Tests for user flows |
| Test Plan | JSON + Markdown | Structured test strategy |
| CI Pipeline | YAML | GitHub Actions workflow |
| Coverage Report | JSON + HTML | Code coverage metrics |
| Quality Report | JSON + Markdown | Mutation scores, flakiness data |

### 3.2 Delivery Methods

| Method | Description |
|--------|-------------|
| GitHub PR | Pull request with generated tests |
| ZIP Download | Downloadable archive |
| Direct Commit | Commit to specified branch |
| API Response | JSON payload with test content |

## 4. Functional Requirements

### 4.1 Repo Ingestion (FR-ING)

#### FR-ING-001: Repository Cloning
- **SHALL** clone public GitHub repositories without authentication
- **SHALL** clone private repositories with valid OAuth token or PAT
- **SHALL** support specifying a target branch (default: default branch)
- **SHALL** support specifying a commit SHA for reproducibility
- **SHALL** timeout after 5 minutes for clone operations
- **SHALL** reject repositories larger than 500MB (configurable)

#### FR-ING-002: Language Detection
- **SHALL** detect primary programming language(s) in repository
- **SHALL** support: TypeScript, JavaScript, Python, Java, Go
- **SHALL** identify framework(s) in use (Express, FastAPI, Spring, etc.)
- **SHALL** detect test frameworks already present (Jest, Pytest, JUnit)
- **SHALL** detect package managers (npm, pip, maven, go mod)

#### FR-ING-003: File Filtering
- **SHALL** respect `.gitignore` patterns
- **SHALL** exclude `node_modules`, `vendor`, `venv`, `.git` directories
- **SHALL** support custom exclusion patterns via configuration
- **SHALL** identify source files vs test files vs config files

### 4.2 Website Ingestion (FR-WEB)

#### FR-WEB-001: Page Crawling
- **SHALL** crawl website starting from provided URL
- **SHALL** discover linked pages within same domain
- **SHALL** capture DOM snapshots of each page
- **SHALL** limit crawl depth (configurable, default: 3)
- **SHALL** limit total pages crawled (configurable, default: 100)
- **SHALL** respect `robots.txt` directives

#### FR-WEB-002: Network Capture
- **SHALL** intercept all network requests during crawling
- **SHALL** capture request method, URL, headers, body
- **SHALL** capture response status, headers, body
- **SHALL** identify API endpoints from XHR/Fetch calls
- **SHALL** capture WebSocket connections and messages

#### FR-WEB-003: Flow Detection
- **SHALL** identify user authentication flows
- **SHALL** identify form submission flows
- **SHALL** identify multi-step workflows (checkout, signup)
- **SHALL** record user action sequences (clicks, fills, navigations)
- **SHALL** support manual flow hints via configuration

#### FR-WEB-004: Authenticated Crawling
- **SHALL** support cookie-based authentication
- **SHALL** support Bearer token authentication
- **SHALL** support form-based login (username/password)
- **SHALL** maintain session across crawl operations

### 4.3 System Modeling (FR-MOD)

#### FR-MOD-001: AST Extraction
- **SHALL** parse source files using Tree-sitter (universal)
- **SHALL** use native parsers for deep analysis when available
- **SHALL** extract function signatures (name, params, return type)
- **SHALL** extract class definitions and methods
- **SHALL** extract exported symbols (ES modules, CommonJS)
- **SHALL** handle parsing errors gracefully (skip malformed files)

#### FR-MOD-002: Endpoint Detection
- **SHALL** detect HTTP route definitions (Express, FastAPI, Spring, etc.)
- **SHALL** extract route method (GET, POST, PUT, DELETE, etc.)
- **SHALL** extract route path with parameters
- **SHALL** identify request body schema when available
- **SHALL** identify response schema when available
- **SHALL** detect middleware/guards applied to routes

#### FR-MOD-003: Dependency Analysis
- **SHALL** build import/require graph between modules
- **SHALL** identify service dependencies (class injections)
- **SHALL** identify external dependencies (npm packages, pip packages)
- **SHALL** identify database access patterns (ORM calls, raw queries)
- **SHALL** identify external API calls (fetch, axios, requests)

#### FR-MOD-004: Behavioral Analysis
- **SHALL** identify branching conditions (if/else, switch)
- **SHALL** identify error handling patterns (try/catch, error returns)
- **SHALL** identify validation logic (guards, validators)
- **SHALL** identify loops and iterations
- **SHALL** extract domain keywords from identifiers and strings

#### FR-MOD-005: System Model Output
- **SHALL** produce a Universal System Model in JSON format
- **SHALL** include all endpoints with metadata
- **SHALL** include all functions with metadata
- **SHALL** include all entities (classes, types, interfaces)
- **SHALL** include dependency graph
- **SHALL** include detected flows (from website crawling)
- **SHALL** version the model for drift detection

### 4.4 Test Planning (FR-PLN)

#### FR-PLN-001: Risk Assessment
- **SHALL** assign risk scores to functions/endpoints
- **SHALL** consider complexity (cyclomatic, cognitive)
- **SHALL** consider domain keywords (auth, payment, security)
- **SHALL** consider change frequency (git history)
- **SHALL** consider dependency count (highly depended = higher risk)

#### FR-PLN-002: Pyramid Distribution
- **SHALL** classify targets into test pyramid levels:
  - Unit: Pure functions, utilities, helpers
  - Integration: Services with dependencies
  - API: HTTP endpoints
  - E2E: User flows, critical paths
- **SHALL** prioritize based on risk score
- **SHALL** respect budget constraints (max tests per level)

#### FR-PLN-003: Test Plan Output
- **SHALL** produce structured TestPlan document
- **SHALL** include target list with assigned test type
- **SHALL** include priority ordering
- **SHALL** include estimated token budget for generation
- **SHALL** be exportable as JSON and Markdown

### 4.5 Test Generation (FR-GEN)

#### FR-GEN-001: Context Assembly
- **SHALL** assemble context for each test target:
  - Function/endpoint signature
  - Implementation code
  - Dependency signatures
  - Branch conditions
  - Sample usage from call sites
  - Domain hints

#### FR-GEN-002: LLM Invocation
- **SHALL** use tiered LLM strategy (cheap → mid → frontier)
- **SHALL** enforce per-repo token budgets
- **SHALL** cache LLM responses for identical contexts
- **SHALL** support fallback to smaller models on budget exhaustion
- **SHALL** track token usage per generation run

#### FR-GEN-003: Test DSL Generation
- **SHALL** generate tests in intermediate DSL format
- **SHALL** include all required DSL fields (see Test DSL Spec)
- **SHALL** generate multiple test cases per target (happy path, edge cases, errors)
- **SHALL** ensure generated DSL is syntactically valid

#### FR-GEN-004: Framework Adaptation
- **SHALL** convert Test DSL to framework-specific code:
  - Jest/Vitest for TypeScript/JavaScript
  - Pytest for Python
  - JUnit for Java
  - Go test for Go
  - Playwright for E2E
- **SHALL** generate syntactically correct code
- **SHALL** include proper imports and setup
- **SHALL** follow framework conventions and best practices

### 4.6 Quality Validation (FR-QA)

#### FR-QA-001: Compilation Gate
- **SHALL** verify generated tests compile without errors
- **SHALL** run appropriate linter/type-checker
- **SHALL** retry with error context on first failure
- **SHALL** discard tests that fail after retry
- **SHALL** record failure reason for analytics

#### FR-QA-002: Runtime Gate
- **SHALL** execute tests against original code
- **SHALL** run each test 3 times to detect flakiness
- **SHALL** tests must pass consistently to proceed
- **SHALL** mark intermittent failures as "flaky"
- **SHALL** record failure details for debugging

#### FR-QA-003: Mutation Gate
- **SHALL** generate 3-5 mutants per target function
- **SHALL** run test against each mutant
- **SHALL** test must kill ≥1 mutant to pass gate
- **SHALL** support mutation types:
  - Comparison operators (>, <, >=, <=, ==, !=)
  - Boolean negation
  - Return value changes
  - Branch removal
  - Arithmetic operators
- **SHALL** time-box mutation testing (configurable)
- **SHALL** sample mutants for large targets

#### FR-QA-004: Assertion Strengthening
- **SHALL** invoke critic model for weak tests
- **SHALL** suggest additional assertions
- **SHALL** re-run mutation gate after strengthening
- **SHALL** limit strengthening iterations (max 2)

### 4.7 Maintenance Engine (FR-MNT)

#### FR-MNT-001: Drift Detection
- **SHALL** compare current System Model with previous version
- **SHALL** identify added functions/endpoints
- **SHALL** identify removed functions/endpoints
- **SHALL** identify modified signatures
- **SHALL** identify tests that reference changed code

#### FR-MNT-002: Test Updates
- **SHALL** regenerate tests for modified code
- **SHALL** remove tests for deleted code
- **SHALL** generate tests for new code
- **SHALL** preserve passing tests that still work
- **SHALL** create PR with test updates

#### FR-MNT-003: Flakiness Management
- **SHALL** track test pass/fail history over time
- **SHALL** calculate flakiness score per test
- **SHALL** flag tests exceeding flakiness threshold
- **SHALL** suggest fixes for flaky tests
- **SHALL** allow quarantine of persistently flaky tests

### 4.8 CI Integration (FR-CI)

#### FR-CI-001: Pipeline Generation
- **SHALL** generate GitHub Actions workflow file
- **SHALL** include appropriate test commands per framework
- **SHALL** include coverage collection
- **SHALL** configure parallel test execution where appropriate
- **SHALL** support custom workflow triggers (push, PR, schedule)

#### FR-CI-002: GitHub App
- **SHALL** install as GitHub App to repositories
- **SHALL** trigger on push/PR events (configurable)
- **SHALL** post PR comments with generation results
- **SHALL** update PR checks with test status
- **SHALL** support manual trigger via comment command

#### FR-CI-003: Reporting
- **SHALL** generate coverage reports (line, branch, function)
- **SHALL** generate mutation score reports
- **SHALL** track metrics over time
- **SHALL** expose metrics via API
- **SHALL** display metrics in web dashboard

### 4.9 API Requirements (FR-API)

#### FR-API-001: REST Endpoints
- **SHALL** expose `/repos` - CRUD for repository configurations
- **SHALL** expose `/runs` - trigger and monitor generation runs
- **SHALL** expose `/tests` - retrieve generated tests
- **SHALL** expose `/reports` - retrieve reports and metrics
- **SHALL** expose `/webhooks` - GitHub webhook receiver

#### FR-API-002: Authentication
- **SHALL** support OAuth 2.0 for user authentication
- **SHALL** support API keys for programmatic access
- **SHALL** enforce rate limits per user/organization

#### FR-API-003: Webhooks
- **SHALL** send webhooks on run completion
- **SHALL** send webhooks on PR creation
- **SHALL** support configurable webhook URLs

### 4.10 Web Dashboard (FR-WEB)

#### FR-WEB-001: Repository Management
- **SHALL** display list of connected repositories
- **SHALL** allow adding new repositories
- **SHALL** allow configuring repository settings
- **SHALL** show repository analysis status

#### FR-WEB-002: Run Management
- **SHALL** display generation run history
- **SHALL** show real-time progress during runs
- **SHALL** allow triggering manual runs
- **SHALL** show detailed run results

#### FR-WEB-003: Analytics
- **SHALL** display test coverage trends
- **SHALL** display mutation score trends
- **SHALL** display flakiness metrics
- **SHALL** display LLM usage and costs

## 5. Non-Functional Requirements

### 5.1 Performance

| Metric | Requirement |
|--------|-------------|
| Repo clone | < 5 minutes for repos up to 500MB |
| System modeling | < 10 minutes for 10k files |
| Test generation | < 1 minute per test target |
| PR creation | < 30 seconds after generation complete |
| API response time | P95 < 500ms for read operations |

### 5.2 Scalability

| Metric | Requirement |
|--------|-------------|
| Concurrent repos | Support 100 concurrent generation runs |
| Worker scaling | Auto-scale workers based on queue depth |
| Storage | Support repos with up to 100k files |

### 5.3 Reliability

| Metric | Requirement |
|--------|-------------|
| Uptime | 99.9% availability |
| Data durability | 99.99% (backed by PostgreSQL) |
| Job completion | Automatic retry on transient failures |

### 5.4 Security

| Requirement | Description |
|-------------|-------------|
| Code isolation | All execution in ephemeral containers |
| Credential encryption | All secrets encrypted at rest |
| Audit logging | All actions logged |
| Network isolation | Workers cannot access external systems |
| RBAC | Role-based access control for teams |

## 6. Constraints

### 6.1 Technology Constraints
- Backend must be implemented in Go
- Frontend must use Next.js/React
- Must use PostgreSQL for primary storage
- Must use NATS for job queuing

### 6.2 Business Constraints
- Must support self-serve usage
- Must track LLM costs per user
- Must support multi-tenant deployment

## 7. Assumptions

1. Users have valid GitHub accounts
2. Target repositories use standard project structures
3. LLM APIs (Claude, OpenAI) remain available and performant
4. Mutation testing tools are available for supported languages

## 8. Dependencies

| External System | Purpose | Criticality |
|-----------------|---------|-------------|
| GitHub API | Repo access, PR creation | Critical |
| Claude API | Test generation | Critical |
| OpenAI API | Fallback generation | High |
| Stryker | JS/TS mutation testing | High |
| Playwright | E2E test execution | High |

## 9. Acceptance Criteria

### 9.1 MVP Acceptance
- [ ] Can clone and analyze JS/TS repositories
- [ ] Generates valid Jest unit tests
- [ ] Generates valid Supertest API tests
- [ ] Tests pass compilation and runtime gates
- [ ] Creates PR with generated tests
- [ ] Generates basic GitHub Actions workflow

### 9.2 Phase 2 Acceptance
- [ ] Can crawl websites and capture flows
- [ ] Generates valid Playwright E2E tests
- [ ] E2E tests execute successfully
- [ ] API inference from network traffic works

### 9.3 Phase 3 Acceptance
- [ ] Drift detection identifies code changes
- [ ] Test updates generated for changed code
- [ ] Mutation testing validates test quality
- [ ] Flakiness detection and reporting works

## 10. Glossary

| Term | Definition |
|------|------------|
| System Model | JSON representation of codebase structure |
| Test DSL | Intermediate format for test definitions |
| Mutation Testing | Technique to validate test effectiveness |
| Test Pyramid | Strategy balancing unit/integration/E2E tests |
| Drift Detection | Identifying changes between model versions |
