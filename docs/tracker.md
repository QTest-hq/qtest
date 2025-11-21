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
| P1-001 | Initialize Go module | 🟢 | P0 | - | `github.com/QTest-hq/qtest` |
| P1-002 | Set up project directory structure | 🟢 | P0 | P1-001 | cmd/, internal/, pkg/ complete |
| P1-003 | Configure linting (golangci-lint) | 🟢 | P0 | P1-001 | .golangci.yml exists |
| P1-004 | Set up Makefile | 🟢 | P0 | P1-001 | build, test, lint, run targets |
| P1-005 | Create Docker Compose for local dev | 🟢 | P0 | - | postgres, redis, nats configured |
| P1-006 | Set up GitHub Actions CI | 🟢 | P1 | P1-003 | .github/workflows/ci.yml |
| P1-007 | Configure environment variables | 🟢 | P0 | P1-001 | config/config.go with env loading |
| P1-008 | Set up logging (zerolog) | 🟢 | P0 | P1-001 | Structured logging throughout |

### 1.2 Database Layer

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-010 | Design database schema | 🟢 | P0 | - | migrations/init.sql - 6+ tables |
| P1-011 | Set up sqlc for type-safe queries | 🟢 | P0 | P1-001 | sqlc.yaml configured |
| P1-012 | Write migration files | 🟢 | P0 | P1-010 | migrations/ directory |
| P1-013 | Implement repositories table CRUD | 🟢 | P0 | P1-011, P1-012 | internal/db/store.go |
| P1-014 | Implement system_models table CRUD | 🟢 | P0 | P1-011, P1-012 | internal/db/store.go |
| P1-015 | Implement generation_runs table CRUD | 🟢 | P0 | P1-011, P1-012 | internal/db/store.go |
| P1-016 | Implement test_results table CRUD | 🟢 | P0 | P1-011, P1-012 | internal/db/store.go |
| P1-017 | Set up connection pooling (pgx) | 🟢 | P1 | P1-011 | min=5, max=25 in db.go |
| P1-018 | Write database integration tests | 🔴 | P1 | P1-013-016 | testcontainers needed |

### 1.3 Repository Ingestion

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-020 | Implement GitHub OAuth flow | 🔴 | P0 | - | OAuth2 for private repos |
| P1-021 | Implement repository cloner (go-git) | 🟢 | P0 | - | internal/github/repo.go |
| P1-022 | Add private repo clone support | 🟡 | P0 | P1-020, P1-021 | Token support exists, OAuth missing |
| P1-023 | Implement language detection | 🟢 | P0 | P1-021 | internal/parser/languages.go |
| P1-024 | Implement framework detection | 🔴 | P1 | P1-023 | Express, FastAPI, Spring missing |
| P1-025 | Build file tree extraction | 🟢 | P0 | P1-021 | workspace/targets.go |
| P1-026 | Implement clone timeout handling | 🟡 | P1 | P1-021 | Basic timeout, needs hardening |
| P1-027 | Add repo size validation | 🔴 | P1 | P1-021 | Not implemented |
| P1-028 | Write ingestion unit tests | 🔴 | P1 | P1-021-027 | No tests |

### 1.4 AST Parsing (Tree-sitter)

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-030 | Set up tree-sitter Go bindings | 🟢 | P0 | - | smacker/go-tree-sitter in go.mod |
| P1-031 | Add TypeScript grammar | 🟢 | P0 | P1-030 | tree-sitter-typescript |
| P1-032 | Add JavaScript grammar | 🟢 | P0 | P1-030 | tree-sitter-javascript |
| P1-033 | Implement function extractor (TS/JS) | 🟢 | P0 | P1-031, P1-032 | internal/parser/parser.go |
| P1-034 | Implement class extractor (TS/JS) | 🟢 | P0 | P1-031, P1-032 | internal/parser/parser.go |
| P1-035 | Implement export extractor (TS/JS) | 🟡 | P0 | P1-031, P1-032 | Basic, needs improvement |
| P1-036 | Implement branch extractor | 🔴 | P1 | P1-033 | if/else, switch not extracted |
| P1-037 | Implement call site extractor | 🔴 | P1 | P1-033 | Function calls not tracked |
| P1-038 | Build unified AST adapter | 🟢 | P0 | P1-033-037 | ParsedFile, Function, Class types |
| P1-039 | Write parser unit tests | 🔴 | P1 | P1-033-038 | No tests |

### 1.5 Endpoint Detection (Framework Supplements)

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-040 | Implement Express route detector | 🟢 | P0 | P1-033 | supplements/express.go |
| P1-041 | Implement FastAPI route detector | 🟢 | P1 | P1-033 | supplements/fastapi.go |
| P1-042 | Implement Gin route detector | 🟢 | P1 | P1-034 | supplements/gin.go |
| P1-042a | Implement Spring Boot detector | 🟢 | P1 | P1-034 | supplements/springboot.go |
| P1-042b | Implement Django REST detector | 🟢 | P1 | P1-034 | supplements/django.go |
| P1-043 | Extract route parameters | 🟢 | P0 | P1-040 | Path params in all supplements |
| P1-044 | Extract request body schema | 🟡 | P1 | P1-040 | Basic support |
| P1-045 | Extract middleware chain | 🔴 | P1 | P1-040 | Not implemented |
| P1-046 | Write endpoint detection tests | 🔴 | P1 | P1-040-045 | |

### 1.6 System Model Builder

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-050 | Define SystemModel Go struct | 🟢 | P0 | - | internal/parser/types.go |
| P1-051 | Implement model builder orchestrator | 🟡 | P0 | P1-038 | Basic in workspace/ |
| P1-052 | Build dependency graph | 🟡 | P0 | P1-037 | Imports tracked, graph incomplete |
| P1-053 | Calculate complexity metrics | 🔴 | P1 | P1-036 | Not implemented |
| P1-054 | Calculate risk scores | 🔴 | P1 | P1-052, P1-053 | Not implemented |
| P1-055 | Serialize model to JSON | 🟢 | P0 | P1-050 | JSON marshaling works |
| P1-056 | Write model builder tests | 🔴 | P1 | P1-051-055 | No tests |

### 1.7 Test Planner

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-060 | Define TestPlan Go struct | 🟢 | P0 | - | pkg/model/intent.go - TestPlan, TestIntent |
| P1-061 | Implement target classifier | 🟢 | P0 | P1-050 | pkg/model/planner.go - classifyFunction |
| P1-062 | Implement priority ranker | 🟢 | P0 | P1-054 | pkg/model/planner.go - priorityScore |
| P1-063 | Implement pyramid distributor | 🟢 | P0 | P1-061 | pkg/model/planner.go - balance levels |
| P1-064 | Generate test case suggestions | 🟢 | P1 | P1-061 | TestIntent with Reason |
| P1-065 | Calculate token estimates | 🔴 | P1 | P1-064 | Not implemented |
| P1-066 | Write test planner tests | 🔴 | P1 | P1-061-065 | |

### 1.8 LLM Integration

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-070 | Define LLMClient interface | 🟢 | P0 | - | internal/llm/types.go |
| P1-071 | Implement Anthropic Claude client | 🟢 | P0 | P1-070 | internal/llm/anthropic.go |
| P1-072 | Implement OpenAI client | 🔴 | P1 | P1-070 | Not implemented |
| P1-073 | Implement tiered model router | 🟢 | P0 | P1-071 | internal/llm/router.go - Tier1/2/3 |
| P1-074 | Implement request cache (Redis) | 🔴 | P0 | P1-071 | Redis configured, not integrated |
| P1-075 | Implement budget manager | 🔴 | P0 | P1-071 | Not implemented |
| P1-076 | Implement usage tracker | 🔴 | P0 | P1-071 | Not implemented |
| P1-077 | Implement fallback logic | 🟢 | P1 | P1-071, P1-072 | router.go with retry + backoff |
| P1-078 | Write LLM integration tests | 🔴 | P1 | P1-071-077 | No tests |

### 1.9 Test DSL Generator

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-080 | Define TestDSL Go struct | 🟢 | P0 | - | pkg/dsl/types.go + pkg/model/spec.go |
| P1-081 | Implement context builder | 🟢 | P0 | P1-050 | generator/generator.go buildContext |
| P1-082 | Design generation prompts | 🟢 | P0 | - | internal/llm/prompts.go |
| P1-083 | Implement unit test DSL generator | 🟢 | P0 | P1-081, P1-082 | generator.go + converter.go |
| P1-084 | Implement API test DSL generator | 🟢 | P0 | P1-081, P1-082 | specgen/ + emitter/ |
| P1-085 | Implement DSL validator | 🟢 | P0 | P1-080 | internal/validator/ |
| P1-086 | Implement batch generation | 🟡 | P1 | P1-083 | workspace/runner_v2.go |
| P1-087 | Write DSL generator tests | 🔴 | P1 | P1-083-086 | No tests |

### 1.10 Framework Adapters & Emitters

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-090 | Define Adapter interface | 🟢 | P0 | - | internal/adapters/adapter.go |
| P1-091 | Implement Jest adapter | 🟢 | P0 | P1-090 | jest_adapter.go |
| P1-091a | Implement Supertest emitter | 🟢 | P0 | P1-090 | emitter/supertest.go |
| P1-091b | Implement Pytest emitter | 🟢 | P0 | P1-090 | emitter/pytest.go |
| P1-091c | Implement Go-HTTP emitter | 🟢 | P0 | P1-090 | emitter/go_http.go |
| P1-092 | Implement Go test adapter | 🟢 | P0 | P1-090 | go_adapter.go |
| P1-093 | Design adapter templates | 🟢 | P0 | P1-091 | Go templates in adapters |
| P1-094 | Handle imports generation | 🟢 | P0 | P1-091 | Auto imports in templates |
| P1-095 | Handle mock generation | 🔴 | P1 | P1-091 | Not implemented |
| P1-096 | Write adapter unit tests | 🟡 | P1 | P1-091-095 | Basic tests exist |

### 1.11 Quality Gates (Basic)

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-100 | Implement compilation checker | 🟢 | P0 | P1-091 | workspace/validator.go |
| P1-101 | Implement runtime validator | 🟢 | P0 | P1-091 | workspace/runner.go |
| P1-102 | Design sandboxed execution | 🟡 | P0 | - | Basic, no Docker isolation |
| P1-103 | Implement test runner service | 🟢 | P0 | P1-102 | workspace/runner.go |
| P1-104 | Handle test failures | 🟡 | P0 | P1-103 | Basic retry, needs improvement |
| P1-105 | Record gate results | 🟢 | P0 | P1-016 | db/store.go |
| P1-106 | Write quality gate tests | 🔴 | P1 | P1-100-105 | No tests |

### 1.12 GitHub Integration

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-110 | Create GitHub App configuration | 🔴 | P0 | - | Not started |
| P1-111 | Implement GitHub App auth | 🔴 | P0 | P1-110 | Token only, no App auth |
| P1-112 | Implement branch creation | 🔴 | P0 | P1-111 | Not implemented |
| P1-113 | Implement file commit | 🔴 | P0 | P1-111 | Not implemented |
| P1-114 | Implement PR creation | 🔴 | P0 | P1-112, P1-113 | Not implemented |
| P1-115 | Design PR template | 🔴 | P1 | - | Not started |
| P1-116 | Implement webhook receiver | 🔴 | P1 | P1-111 | Not implemented |
| P1-117 | Write GitHub integration tests | 🔴 | P1 | P1-111-116 | No tests |

### 1.13 CI Pipeline Generator

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-120 | Design workflow template | 🟢 | P0 | - | .github/workflows/ci.yml exists |
| P1-121 | Implement workflow generator | 🔴 | P0 | P1-120 | Manual only, no generation |
| P1-122 | Add coverage collection | 🟢 | P1 | P1-121 | workspace/coverage.go |
| P1-123 | Write CI generator tests | 🔴 | P1 | P1-121-122 | No tests |

### 1.14 Worker System

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-130 | Set up NATS JetStream | 🟢 | P0 | - | docker-compose + config |
| P1-131 | Define job schemas | 🔴 | P0 | - | **NOT STARTED** |
| P1-132 | Implement base worker | 🟡 | P0 | P1-130 | worker/pool.go skeleton |
| P1-133 | Implement ingestion worker | 🔴 | P0 | P1-132, P1-021 | Stub only |
| P1-134 | Implement modeling worker | 🔴 | P0 | P1-132, P1-051 | Stub only |
| P1-135 | Implement planning worker | 🔴 | P0 | P1-132, P1-061 | Stub only |
| P1-136 | Implement generation worker | 🔴 | P0 | P1-132, P1-083 | Stub only |
| P1-137 | Implement integration worker | 🔴 | P0 | P1-132, P1-114 | Stub only |
| P1-138 | Add job retry logic | 🔴 | P1 | P1-132 | Not implemented |
| P1-139 | Write worker tests | 🔴 | P1 | P1-133-138 | No tests |

### 1.15 API Server

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-140 | Set up Chi router | 🟢 | P0 | - | internal/api/server.go |
| P1-141 | Implement health endpoint | 🟢 | P0 | P1-140 | /health, /ready |
| P1-142 | Implement repos endpoints | 🟡 | P0 | P1-140, P1-013 | Routes exist, handlers TODO |
| P1-143 | Implement runs endpoints | 🟡 | P0 | P1-140, P1-015 | Routes exist, handlers TODO |
| P1-144 | Implement auth middleware | 🔴 | P0 | P1-020 | Not wired |
| P1-145 | Implement rate limiting | 🔴 | P1 | P1-140 | Not implemented |
| P1-146 | Add OpenAPI documentation | 🔴 | P1 | P1-142-143 | Not implemented |
| P1-147 | Write API tests | 🔴 | P1 | P1-142-145 | No tests |

### 1.16 CLI Tool

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P1-150 | Set up cobra CLI framework | 🟢 | P0 | - | cmd/cli/main.go |
| P1-151 | Implement `qtest auth login` | 🔴 | P0 | P1-150 | Not implemented |
| P1-152 | Implement `qtest generate` | 🟢 | P0 | P1-150 | generate-file works |
| P1-153 | Implement `qtest status` | 🔴 | P1 | P1-150 | Not implemented |
| P1-154 | Add progress output | 🟡 | P1 | P1-152 | Basic, no real-time |
| P1-155 | Write CLI tests | 🔴 | P1 | P1-151-154 | No tests |

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
| P3-050 | Implement coverage collector | 🟢 | P0 | - | codecov/collector.go - Go, Python, JS |
| P3-050a | Implement coverage analyzer | 🟢 | P0 | P3-050 | codecov/analyzer.go - gap detection |
| P3-050b | Implement coverage-guided gen | 🟢 | P0 | P3-050a | workspace/coverage_runner.go |
| P3-051 | Store coverage snapshots | 🔴 | P0 | P3-050 | In database |
| P3-052 | Calculate coverage delta | 🟢 | P0 | P3-051 | Before/after in runner |
| P3-053 | Implement mutation score reporter | 🔴 | P0 | P3-002 | |
| P3-054 | Generate markdown reports | 🔴 | P1 | P3-050-053 | For PR comments |
| P3-055 | Write reporting tests | 🔴 | P1 | P3-050-054 | |

### 3.7 Contract Testing (NEW)

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P3-060 | Define Contract types | 🟢 | P0 | - | contract/contract.go |
| P3-061 | Implement contract generator | 🟢 | P0 | P3-060 | From SystemModel endpoints |
| P3-062 | Implement contract validator | 🟢 | P0 | P3-060 | Validate API responses |
| P3-063 | Generate contract tests | 🟢 | P0 | P3-061 | Jest, pytest, Go tests |
| P3-064 | CLI commands for contracts | 🟢 | P0 | P3-060 | contract generate/validate |

### 3.8 Test Data Generation (NEW)

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P3-070 | Implement field-aware generator | 🟢 | P0 | - | datagen/generator.go |
| P3-071 | Implement schema-based generator | 🟢 | P0 | P3-070 | datagen/schema.go |
| P3-072 | Generate edge case data | 🟢 | P0 | P3-071 | Valid, invalid, boundary |
| P3-073 | CLI commands for datagen | 🟢 | P0 | P3-070 | datagen generate |

### 3.9 Test Validation (NEW)

| ID | Task | Status | Priority | Dependencies | Notes |
|----|------|--------|----------|--------------|-------|
| P3-080 | Implement test runner | 🟢 | P0 | - | validator/validator.go |
| P3-081 | Parse test errors | 🟢 | P0 | P3-080 | Jest, pytest, go test |
| P3-082 | LLM-powered auto-fix | 🟢 | P0 | P3-081 | validator/fixer.go |
| P3-083 | CLI validate command | 🟢 | P0 | P3-080 | validate run/fix |

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

### Progress Tracking (Updated 2025-11-21)

| Phase | 🟢 Completed | 🟡 In Progress | 🔴 Not Started | % Done |
|-------|-------------|----------------|----------------|--------|
| Phase 1 | 62 | 8 | 33 | **60%** |
| Phase 2 | 0 | 0 | 35 | 0% |
| Phase 3 | 21 | 0 | 24 | **47%** |
| Phase 4 | 0 | 0 | 35 | 0% |
| **Total** | **83** | **8** | **127** | **38%** |

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
| 2025-11-21 | **Major audit**: Updated all Phase 1 tasks to reflect actual implementation. 49% of Phase 1 complete. |
| 2025-11-21 | **Feature update**: Added Contract Testing (3.7), Test Data Gen (3.8), Validation (3.9). Updated supplements (Express, FastAPI, Gin, Spring Boot, Django). Added coverage-guided generation. Overall 38% complete. |

---

## Critical Gaps for MVP (P0 Not Started)

These P0 tasks block MVP completion:

| ID | Task | Category | Blocking |
|----|------|----------|----------|
| P1-020 | GitHub OAuth flow | Ingestion | Private repo support |
| ~~P1-040-046~~ | ~~Endpoint Detection~~ | ~~Parsing~~ | ✅ Done - 5 frameworks |
| ~~P1-060-066~~ | ~~Test Planner~~ | ~~Planning~~ | ✅ Done |
| P1-074-076 | LLM Cache/Budget | LLM | Cost control |
| ~~P1-084~~ | ~~API test DSL generator~~ | ~~Generator~~ | ✅ Done |
| ~~P1-092~~ | ~~Supertest adapter~~ | ~~Adapters~~ | ✅ Done - 3 emitters |
| P1-110-114 | GitHub Integration | Integration | PR creation |
| P1-131-137 | Worker Implementation | Workers | Async processing |
| P1-144 | Auth middleware | API | Security |

---

## Notes

- Update this document as tasks progress
- Move blocked tasks to deferred if not resolvable
- Add new tasks as discovered during implementation
- Weekly review to update status and priorities
- **Focus on P0 gaps before starting Phase 2**
