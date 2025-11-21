# Implementation Tracker

## Overview

This document tracks all implementation tasks for QTest. Tasks are organized by phase and component.

**Legend:**
- 🔴 Not Started
- 🟡 In Progress
- 🟢 Completed
- ⏸️ Blocked
- 🔵 Deferred

---

## Phase 1: MVP (Weeks 1-12)

### 1.1 Project Setup

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-001 | Initialize Go module | 🔴 | P0 | - | `go mod init github.com/qtest/qtest` |
| P1-002 | Set up project directory structure | 🔴 | P0 | P1-001 | cmd/, internal/, pkg/, web/ |
| P1-003 | Configure linting (golangci-lint) | 🔴 | P0 | P1-001 | .golangci.yml |
| P1-004 | Set up Makefile | 🔴 | P0 | P1-001 | build, test, lint, run targets |
| P1-005 | Create Docker Compose for local dev | 🔴 | P0 | - | postgres, redis, nats |
| P1-006 | Set up GitHub Actions CI | 🔴 | P1 | P1-003 | test, lint, build on PR |
| P1-007 | Configure environment variables | 🔴 | P0 | P1-001 | .env.example, viper config |
| P1-008 | Set up logging (zerolog) | 🔴 | P0 | P1-001 | Structured JSON logging |

### 1.2 Database Layer

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-010 | Design database schema | 🔴 | P0 | - | See data-schemas.md |
| P1-011 | Set up sqlc for type-safe queries | 🔴 | P0 | P1-001 | sqlc.yaml configuration |
| P1-012 | Write migration files | 🔴 | P0 | P1-010 | goose or golang-migrate |
| P1-013 | Implement repositories table CRUD | 🔴 | P0 | P1-011, P1-012 | |
| P1-014 | Implement system_models table CRUD | 🔴 | P0 | P1-011, P1-012 | |
| P1-015 | Implement generation_runs table CRUD | 🔴 | P0 | P1-011, P1-012 | |
| P1-016 | Implement test_results table CRUD | 🔴 | P0 | P1-011, P1-012 | |
| P1-017 | Set up connection pooling (pgx) | 🔴 | P1 | P1-011 | |
| P1-018 | Write database integration tests | 🔴 | P1 | P1-013-016 | testcontainers |

### 1.3 Repository Ingestion

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-020 | Implement GitHub OAuth flow | 🔴 | P0 | - | OAuth2 for private repos |
| P1-021 | Implement repository cloner (go-git) | 🔴 | P0 | - | Clone public repos |
| P1-022 | Add private repo clone support | 🔴 | P0 | P1-020, P1-021 | With auth token |
| P1-023 | Implement language detection | 🔴 | P0 | P1-021 | Detect TS/JS/Python/Go/Java |
| P1-024 | Implement framework detection | 🔴 | P1 | P1-023 | Express, FastAPI, Spring, etc. |
| P1-025 | Build file tree extraction | 🔴 | P0 | P1-021 | Respect .gitignore |
| P1-026 | Implement clone timeout handling | 🔴 | P1 | P1-021 | 5 minute max |
| P1-027 | Add repo size validation | 🔴 | P1 | P1-021 | 500MB max |
| P1-028 | Write ingestion unit tests | 🔴 | P1 | P1-021-027 | |

### 1.4 AST Parsing (Tree-sitter)

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-030 | Set up tree-sitter Go bindings | 🔴 | P0 | - | go-tree-sitter |
| P1-031 | Add TypeScript grammar | 🔴 | P0 | P1-030 | tree-sitter-typescript |
| P1-032 | Add JavaScript grammar | 🔴 | P0 | P1-030 | tree-sitter-javascript |
| P1-033 | Implement function extractor (TS/JS) | 🔴 | P0 | P1-031, P1-032 | Name, params, return type |
| P1-034 | Implement class extractor (TS/JS) | 🔴 | P0 | P1-031, P1-032 | Properties, methods |
| P1-035 | Implement export extractor (TS/JS) | 🔴 | P0 | P1-031, P1-032 | ES modules, CommonJS |
| P1-036 | Implement branch extractor | 🔴 | P1 | P1-033 | if/else, switch, ternary |
| P1-037 | Implement call site extractor | 🔴 | P1 | P1-033 | Function calls |
| P1-038 | Build unified AST adapter | 🔴 | P0 | P1-033-037 | Common representation |
| P1-039 | Write parser unit tests | 🔴 | P1 | P1-033-038 | |

### 1.5 Endpoint Detection (Express/TS)

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-040 | Implement Express route detector | 🔴 | P0 | P1-033 | app.get/post/put/delete |
| P1-041 | Implement Fastify route detector | 🔴 | P1 | P1-033 | |
| P1-042 | Implement NestJS route detector | 🔴 | P1 | P1-034 | Decorators @Get, @Post |
| P1-043 | Extract route parameters | 🔴 | P0 | P1-040 | Path params, query params |
| P1-044 | Extract request body schema | 🔴 | P1 | P1-040 | TypeScript types |
| P1-045 | Extract middleware chain | 🔴 | P1 | P1-040 | Auth, validation |
| P1-046 | Write endpoint detection tests | 🔴 | P1 | P1-040-045 | |

### 1.6 System Model Builder

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-050 | Define SystemModel Go struct | 🔴 | P0 | - | See data-schemas.md |
| P1-051 | Implement model builder orchestrator | 🔴 | P0 | P1-038 | Coordinate extractors |
| P1-052 | Build dependency graph | 🔴 | P0 | P1-037 | Import relationships |
| P1-053 | Calculate complexity metrics | 🔴 | P1 | P1-036 | Cyclomatic complexity |
| P1-054 | Calculate risk scores | 🔴 | P1 | P1-052, P1-053 | Based on complexity, deps |
| P1-055 | Serialize model to JSON | 🔴 | P0 | P1-050 | JSONB for PostgreSQL |
| P1-056 | Write model builder tests | 🔴 | P1 | P1-051-055 | |

### 1.7 Test Planner

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-060 | Define TestPlan Go struct | 🔴 | P0 | - | See data-schemas.md |
| P1-061 | Implement target classifier | 🔴 | P0 | P1-050 | Function → unit, endpoint → API |
| P1-062 | Implement priority ranker | 🔴 | P0 | P1-054 | Based on risk score |
| P1-063 | Implement pyramid distributor | 🔴 | P0 | P1-061 | Balance unit/integration/API |
| P1-064 | Generate test case suggestions | 🔴 | P1 | P1-061 | Happy path, edge cases |
| P1-065 | Calculate token estimates | 🔴 | P1 | P1-064 | For budget management |
| P1-066 | Write test planner tests | 🔴 | P1 | P1-061-065 | |

### 1.8 LLM Integration

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-070 | Define LLMClient interface | 🔴 | P0 | - | Complete, Stream methods |
| P1-071 | Implement Anthropic Claude client | 🔴 | P0 | P1-070 | Haiku, Sonnet, Opus |
| P1-072 | Implement OpenAI client | 🔴 | P1 | P1-070 | GPT-4o, GPT-4o-mini |
| P1-073 | Implement tiered model router | 🔴 | P0 | P1-071 | Route by task type |
| P1-074 | Implement request cache (Redis) | 🔴 | P0 | P1-071 | Hash prompt → response |
| P1-075 | Implement budget manager | 🔴 | P0 | P1-071 | Per-user/repo limits |
| P1-076 | Implement usage tracker | 🔴 | P0 | P1-071 | Log all token usage |
| P1-077 | Implement fallback logic | 🔴 | P1 | P1-071, P1-072 | Provider and tier fallback |
| P1-078 | Write LLM integration tests | 🔴 | P1 | P1-071-077 | Mock responses |

### 1.9 Test DSL Generator

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-080 | Define TestDSL Go struct | 🔴 | P0 | - | See test-dsl-spec.md |
| P1-081 | Implement context builder | 🔴 | P0 | P1-050 | Assemble function context |
| P1-082 | Design generation prompts | 🔴 | P0 | - | Unit, API test prompts |
| P1-083 | Implement unit test DSL generator | 🔴 | P0 | P1-081, P1-082 | LLM → DSL |
| P1-084 | Implement API test DSL generator | 🔴 | P0 | P1-081, P1-082 | LLM → DSL |
| P1-085 | Implement DSL validator | 🔴 | P0 | P1-080 | Schema validation |
| P1-086 | Implement batch generation | 🔴 | P1 | P1-083 | Multiple functions at once |
| P1-087 | Write DSL generator tests | 🔴 | P1 | P1-083-086 | |

### 1.10 Framework Adapters

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-090 | Define Adapter interface | 🔴 | P0 | - | DSL → Code |
| P1-091 | Implement Jest adapter | 🔴 | P0 | P1-090 | For unit tests |
| P1-092 | Implement Supertest adapter | 🔴 | P0 | P1-090 | For API tests |
| P1-093 | Design adapter templates | 🔴 | P0 | P1-091 | Go templates |
| P1-094 | Handle imports generation | 🔴 | P0 | P1-091 | Automatic imports |
| P1-095 | Handle mock generation | 🔴 | P1 | P1-091 | jest.mock() |
| P1-096 | Write adapter unit tests | 🔴 | P1 | P1-091-095 | |

### 1.11 Quality Gates (Basic)

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-100 | Implement compilation checker | 🔴 | P0 | P1-091 | Run tsc/eslint |
| P1-101 | Implement runtime validator | 🔴 | P0 | P1-091 | Run test, check pass |
| P1-102 | Design sandboxed execution | 🔴 | P0 | - | Docker container |
| P1-103 | Implement test runner service | 🔴 | P0 | P1-102 | Execute in sandbox |
| P1-104 | Handle test failures | 🔴 | P0 | P1-103 | Retry logic |
| P1-105 | Record gate results | 🔴 | P0 | P1-016 | Store in database |
| P1-106 | Write quality gate tests | 🔴 | P1 | P1-100-105 | |

### 1.12 GitHub Integration

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-110 | Create GitHub App configuration | 🔴 | P0 | - | Manifest file |
| P1-111 | Implement GitHub App auth | 🔴 | P0 | P1-110 | JWT, installation tokens |
| P1-112 | Implement branch creation | 🔴 | P0 | P1-111 | go-github |
| P1-113 | Implement file commit | 🔴 | P0 | P1-111 | Commit generated tests |
| P1-114 | Implement PR creation | 🔴 | P0 | P1-112, P1-113 | With description |
| P1-115 | Design PR template | 🔴 | P1 | - | Summary, metrics |
| P1-116 | Implement webhook receiver | 🔴 | P1 | P1-111 | Push/PR events |
| P1-117 | Write GitHub integration tests | 🔴 | P1 | P1-111-116 | Mock GitHub API |

### 1.13 CI Pipeline Generator

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-120 | Design workflow template | 🔴 | P0 | - | GitHub Actions YAML |
| P1-121 | Implement workflow generator | 🔴 | P0 | P1-120 | Based on detected framework |
| P1-122 | Add coverage collection | 🔴 | P1 | P1-121 | Jest --coverage |
| P1-123 | Write CI generator tests | 🔴 | P1 | P1-121-122 | |

### 1.14 Worker System

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-130 | Set up NATS JetStream | 🔴 | P0 | - | Docker, Go client |
| P1-131 | Define job schemas | 🔴 | P0 | - | Ingestion, modeling, etc. |
| P1-132 | Implement base worker | 🔴 | P0 | P1-130 | Pull, process, ack pattern |
| P1-133 | Implement ingestion worker | 🔴 | P0 | P1-132, P1-021 | |
| P1-134 | Implement modeling worker | 🔴 | P0 | P1-132, P1-051 | |
| P1-135 | Implement planning worker | 🔴 | P0 | P1-132, P1-061 | |
| P1-136 | Implement generation worker | 🔴 | P0 | P1-132, P1-083 | |
| P1-137 | Implement integration worker | 🔴 | P0 | P1-132, P1-114 | |
| P1-138 | Add job retry logic | 🔴 | P1 | P1-132 | Exponential backoff |
| P1-139 | Write worker tests | 🔴 | P1 | P1-133-138 | |

### 1.15 API Server

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-140 | Set up Chi router | 🔴 | P0 | - | HTTP server |
| P1-141 | Implement health endpoint | 🔴 | P0 | P1-140 | /health |
| P1-142 | Implement repos endpoints | 🔴 | P0 | P1-140, P1-013 | CRUD |
| P1-143 | Implement runs endpoints | 🔴 | P0 | P1-140, P1-015 | Trigger, status |
| P1-144 | Implement auth middleware | 🔴 | P0 | P1-020 | OAuth, API keys |
| P1-145 | Implement rate limiting | 🔴 | P1 | P1-140 | Redis-based |
| P1-146 | Add OpenAPI documentation | 🔴 | P1 | P1-142-143 | Swagger |
| P1-147 | Write API tests | 🔴 | P1 | P1-142-145 | |

### 1.16 CLI Tool

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-150 | Set up cobra CLI framework | 🔴 | P0 | - | |
| P1-151 | Implement `qtest auth login` | 🔴 | P0 | P1-150 | OAuth flow |
| P1-152 | Implement `qtest generate` | 🔴 | P0 | P1-150 | Main command |
| P1-153 | Implement `qtest status` | 🔴 | P1 | P1-150 | Check run status |
| P1-154 | Add progress output | 🔴 | P1 | P1-152 | Real-time updates |
| P1-155 | Write CLI tests | 🔴 | P1 | P1-151-154 | |

---

## Phase 2: E2E + Website (Weeks 13-20)

### 2.1 Playwright Integration

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P2-001 | Design Playwright sidecar service | 🔴 | P0 | - | Node.js gRPC server |
| P2-002 | Implement crawler API | 🔴 | P0 | P2-001 | Start, stop, status |
| P2-003 | Implement page navigation | 🔴 | P0 | P2-001 | goto, click, fill |
| P2-004 | Implement network interception | 🔴 | P0 | P2-001 | Capture XHR/fetch |
| P2-005 | Implement DOM snapshots | 🔴 | P1 | P2-001 | For selector generation |
| P2-006 | Implement screenshot capture | 🔴 | P1 | P2-001 | |
| P2-007 | Write Playwright integration tests | 🔴 | P1 | P2-001-006 | |

### 2.2 Website Crawler

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P2-010 | Implement crawler orchestrator | 🔴 | P0 | P2-002 | Manage crawl session |
| P2-011 | Implement page discovery | 🔴 | P0 | P2-003 | Find links, crawl |
| P2-012 | Implement depth limiting | 🔴 | P0 | P2-011 | Max depth config |
| P2-013 | Implement page limiting | 🔴 | P0 | P2-011 | Max pages config |
| P2-014 | Implement robots.txt handling | 🔴 | P1 | P2-011 | Respect directives |
| P2-015 | Write crawler tests | 🔴 | P1 | P2-010-014 | Mock websites |

### 2.3 Flow Detection

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P2-020 | Design Flow data structure | 🔴 | P0 | - | See data-schemas.md |
| P2-021 | Implement action recorder | 🔴 | P0 | P2-003 | Record user actions |
| P2-022 | Implement login flow detection | 🔴 | P0 | P2-021 | Heuristics for auth |
| P2-023 | Implement form detection | 🔴 | P0 | P2-005 | Identify form fields |
| P2-024 | Implement LLM flow discovery | 🔴 | P1 | P2-021 | Agent explores site |
| P2-025 | Support user-provided hints | 🔴 | P1 | - | YAML flow config |
| P2-026 | Write flow detection tests | 🔴 | P1 | P2-021-025 | |

### 2.4 API Inference

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P2-030 | Implement network log parser | 🔴 | P0 | P2-004 | Parse captured requests |
| P2-031 | Identify API endpoints | 🔴 | P0 | P2-030 | Filter XHR/fetch calls |
| P2-032 | Infer request schemas | 🔴 | P1 | P2-031 | From request bodies |
| P2-033 | Infer response schemas | 🔴 | P1 | P2-031 | From response bodies |
| P2-034 | Merge with code-based endpoints | 🔴 | P1 | P2-031, P1-040 | Unified endpoint list |
| P2-035 | Write API inference tests | 🔴 | P1 | P2-030-034 | |

### 2.5 E2E Test Generation

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P2-040 | Design E2E DSL extensions | 🔴 | P0 | - | Steps, selectors |
| P2-041 | Implement E2E test DSL generator | 🔴 | P0 | P2-020, P2-040 | Flow → DSL |
| P2-042 | Design E2E generation prompts | 🔴 | P0 | - | LLM prompts |
| P2-043 | Implement Playwright adapter | 🔴 | P0 | P1-090 | DSL → Playwright code |
| P2-044 | Handle selector generation | 🔴 | P0 | P2-005 | Prefer test-id, fallback |
| P2-045 | Implement wait strategies | 🔴 | P1 | P2-043 | Auto-wait, explicit waits |
| P2-046 | Write E2E generation tests | 🔴 | P1 | P2-041-045 | |

### 2.6 E2E Validation

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P2-050 | Implement E2E test runner | 🔴 | P0 | P2-043 | Run in browser |
| P2-051 | Implement flakiness detection | 🔴 | P0 | P2-050 | Run 3x, check consistency |
| P2-052 | Implement screenshot comparison | 🔴 | P1 | P2-050 | Visual regression |
| P2-053 | Handle test timeouts | 🔴 | P0 | P2-050 | Graceful timeout |
| P2-054 | Write E2E validation tests | 🔴 | P1 | P2-050-053 | |

---

## Phase 3: Quality + Maintenance (Weeks 21-28)

### 3.1 Mutation Testing Integration

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P3-001 | Design mutation service interface | 🔴 | P0 | - | See mutation-strategy.md |
| P3-002 | Implement Stryker integration | 🔴 | P0 | P3-001 | For TS/JS |
| P3-003 | Implement mutant sampler | 🔴 | P0 | P3-002 | 3-5 mutants per function |
| P3-004 | Implement time budgeting | 🔴 | P0 | P3-002 | Per-function timeout |
| P3-005 | Implement mutation cache | 🔴 | P1 | P3-002 | Cache results |
| P3-006 | Add mutation worker | 🔴 | P0 | P1-132, P3-002 | Process mutation jobs |
| P3-007 | Write mutation integration tests | 🔴 | P1 | P3-002-006 | |

### 3.2 Test Strengthening

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P3-010 | Implement surviving mutant analyzer | 🔴 | P0 | P3-002 | Identify weak tests |
| P3-011 | Design strengthening prompts | 🔴 | P0 | - | LLM prompts |
| P3-012 | Implement strengthening loop | 🔴 | P0 | P3-010, P3-011 | Retry with feedback |
| P3-013 | Add strengthening limits | 🔴 | P0 | P3-012 | Max 2 attempts |
| P3-014 | Write strengthening tests | 🔴 | P1 | P3-010-013 | |

### 3.3 Drift Detection

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P3-020 | Implement system model differ | 🔴 | P0 | P1-050 | Compare versions |
| P3-021 | Identify added entities | 🔴 | P0 | P3-020 | New functions/endpoints |
| P3-022 | Identify removed entities | 🔴 | P0 | P3-020 | Deleted code |
| P3-023 | Identify modified entities | 🔴 | P0 | P3-020 | Signature changes |
| P3-024 | Map tests to changed code | 🔴 | P0 | P3-020 | Find affected tests |
| P3-025 | Write drift detection tests | 🔴 | P1 | P3-020-024 | |

### 3.4 Continuous Maintenance

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P3-030 | Implement maintenance scheduler | 🔴 | P0 | P3-020 | Trigger on push |
| P3-031 | Implement test regenerator | 🔴 | P0 | P3-023, P1-083 | Regen for changes |
| P3-032 | Implement test remover | 🔴 | P0 | P3-022 | Remove obsolete |
| P3-033 | Implement test updater PR | 🔴 | P0 | P1-114, P3-031 | Maintenance PRs |
| P3-034 | Write maintenance tests | 🔴 | P1 | P3-030-033 | |

### 3.5 Flakiness Management

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P3-040 | Implement flakiness tracker | 🔴 | P0 | - | Track pass/fail history |
| P3-041 | Calculate flakiness score | 🔴 | P0 | P3-040 | Rolling average |
| P3-042 | Implement quarantine feature | 🔴 | P1 | P3-041 | Skip flaky tests |
| P3-043 | Suggest flakiness fixes | 🔴 | P1 | P3-041 | LLM analysis |
| P3-044 | Write flakiness tests | 🔴 | P1 | P3-040-043 | |

### 3.6 Coverage & Reporting

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P3-050 | Implement coverage collector | 🔴 | P0 | - | Parse Jest coverage |
| P3-051 | Store coverage snapshots | 🔴 | P0 | P3-050 | In database |
| P3-052 | Calculate coverage delta | 🔴 | P0 | P3-051 | Before/after |
| P3-053 | Implement mutation score reporter | 🔴 | P0 | P3-002 | |
| P3-054 | Generate markdown reports | 🔴 | P1 | P3-050-053 | For PR comments |
| P3-055 | Write reporting tests | 🔴 | P1 | P3-050-054 | |

---

## Phase 4: Scale + Enterprise (Weeks 29-40)

### 4.1 Multi-Language Support

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P4-001 | Add Python grammar (tree-sitter) | 🔴 | P0 | P1-030 | |
| P4-002 | Implement Python function extractor | 🔴 | P0 | P4-001 | |
| P4-003 | Implement FastAPI route detector | 🔴 | P0 | P4-002 | |
| P4-004 | Implement Pytest adapter | 🔴 | P0 | P1-090 | |
| P4-005 | Add mutmut integration | 🔴 | P1 | P3-001 | Python mutation |
| P4-006 | Add Java grammar (tree-sitter) | 🔴 | P0 | P1-030 | |
| P4-007 | Implement Java class extractor | 🔴 | P0 | P4-006 | |
| P4-008 | Implement Spring route detector | 🔴 | P0 | P4-007 | |
| P4-009 | Implement JUnit adapter | 🔴 | P0 | P1-090 | |
| P4-010 | Add PIT integration | 🔴 | P1 | P3-001 | Java mutation |
| P4-011 | Write multi-language tests | 🔴 | P1 | P4-001-010 | |

### 4.2 Web Dashboard

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P4-020 | Set up Next.js project | 🔴 | P0 | - | |
| P4-021 | Implement authentication (NextAuth) | 🔴 | P0 | P4-020 | GitHub OAuth |
| P4-022 | Build repository list page | 🔴 | P0 | P4-020 | |
| P4-023 | Build repository detail page | 🔴 | P0 | P4-020 | |
| P4-024 | Build run history page | 🔴 | P0 | P4-020 | |
| P4-025 | Build run detail page | 🔴 | P0 | P4-020 | Real-time progress |
| P4-026 | Build coverage dashboard | 🔴 | P1 | P4-020 | Charts, trends |
| P4-027 | Build settings page | 🔴 | P1 | P4-020 | |
| P4-028 | Write frontend tests | 🔴 | P1 | P4-022-027 | |

### 4.3 Team Features

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P4-030 | Design organization model | 🔴 | P0 | - | Multi-tenant |
| P4-031 | Implement organization CRUD | 🔴 | P0 | P4-030 | |
| P4-032 | Implement team membership | 🔴 | P0 | P4-031 | Invite, roles |
| P4-033 | Implement RBAC | 🔴 | P0 | P4-032 | Admin, member, viewer |
| P4-034 | Build team dashboard | 🔴 | P1 | P4-020, P4-031 | Aggregate metrics |
| P4-035 | Write team feature tests | 🔴 | P1 | P4-030-034 | |

### 4.4 Enterprise Features

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P4-040 | Implement SSO (SAML/OIDC) | 🔴 | P1 | - | Enterprise auth |
| P4-041 | Implement audit logging | 🔴 | P1 | - | Compliance |
| P4-042 | Implement data export | 🔴 | P1 | - | GDPR compliance |
| P4-043 | Design self-hosted deployment | 🔴 | P2 | - | Docker Compose, Helm |
| P4-044 | Write enterprise feature tests | 🔴 | P1 | P4-040-043 | |

---

## Summary

### Task Count by Phase

| Phase | Total Tasks | P0 | P1 | P2 |
|-------|-------------|----|----|----|
| Phase 1: MVP | 98 | 72 | 26 | 0 |
| Phase 2: E2E | 35 | 22 | 13 | 0 |
| Phase 3: Quality | 35 | 23 | 12 | 0 |
| Phase 4: Scale | 35 | 20 | 14 | 1 |
| **Total** | **203** | **137** | **65** | **1** |

### Progress Tracking

| Phase | Not Started | In Progress | Completed | Blocked |
|-------|-------------|-------------|-----------|---------|
| Phase 1 | 98 | 0 | 0 | 0 |
| Phase 2 | 35 | 0 | 0 | 0 |
| Phase 3 | 35 | 0 | 0 | 0 |
| Phase 4 | 35 | 0 | 0 | 0 |
| **Total** | **203** | **0** | **0** | **0** |

### Critical Path (Must Complete for MVP)

```
P1-001 → P1-005 → P1-010 → P1-021 → P1-030 → P1-033 → P1-040 →
P1-051 → P1-061 → P1-071 → P1-083 → P1-091 → P1-100 → P1-114 →
P1-130 → P1-133 → MVP Complete
```

---

## Update Log

| Date | Changes |
|------|---------|
| Initial | Created tracker with 203 tasks |

---

## Notes

- Update this document as tasks progress
- Move blocked tasks to deferred if not resolvable
- Add new tasks as discovered during implementation
- Weekly review to update status and priorities
