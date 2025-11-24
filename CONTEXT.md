# QTest Development Context Document

**Date:** 2025-11-23
**Purpose:** Resume development from current state

---

## Project Overview

QTest is an AI-powered test generation platform that:
1. Parses source code using tree-sitter (Go, Python, TypeScript, JavaScript)
2. Builds a Universal System Model (language-agnostic IR)
3. Detects API endpoints using framework-specific supplements
4. Plans tests using TestIntent → TestSpec pipeline
5. Sends code to LLM (Ollama) for test spec generation
6. Emits language-specific test code using adapters/emitters
7. Supports coverage-guided incremental test generation

---

## Architecture

```
cmd/cli/                     # CLI entry point (cobra)
  main.go                    # Core commands
  coverage.go                # Coverage commands
  contract.go                # Contract testing commands
  datagen.go                 # Data generation commands
  validate.go                # Validation commands
  workspace.go               # Workspace management
  e2e.go                     # E2E testing commands

internal/
  adapters/                  # DSL → Test code adapters
    go_adapter.go            # Go test generation
    python_adapter.go        # pytest generation
    jest_adapter.go          # Jest generation

  codecov/                   # Real code coverage
    collector.go             # Collect from go/pytest/jest
    analyzer.go              # Gap analysis & prioritization

  config/config.go           # Configuration management

  contract/                  # Contract testing
    contract.go              # Contract types & validation
    testgen.go               # Contract test generation

  datagen/                   # Test data generation
    generator.go             # Field-aware data generator
    schema.go                # Schema-based generation

  e2e/                       # E2E test generation
    playwright.go            # Playwright test generator
    runner.go                # Test runner with result parsing
    llm_enhancer.go          # LLM-powered test enhancement

  emitter/                   # TestSpec → Test code emitters
    supertest.go             # JavaScript API tests
    pytest.go                # Python API tests
    go_http.go               # Go API tests

  flow/                      # User flow detection
    discovery.go             # LLM-based flow discovery
    types.go                 # Flow types and structures

  generator/                 # Legacy DSL generator
    generator.go             # LLM orchestration
    converter.go             # YAML → DSL conversion

  llm/                       # LLM integration
    router.go                # Tier-based routing
    ollama.go                # Ollama client
    prompts.go               # System prompts

  parser/                    # Tree-sitter parsing
    parser.go                # Multi-language parser
    languages.go             # Language detection

  specgen/                   # Test specification generator
    generator.go             # TestIntent → TestSpec via LLM

  supplements/               # Framework endpoint detectors
    express.go               # Express.js (Node.js)
    fastapi.go               # FastAPI (Python)
    gin.go                   # Gin (Go)
    springboot.go            # Spring Boot (Java)
    django.go                # Django REST (Python)

  validator/                 # Test validation
    validator.go             # Run tests, parse errors
    fixer.go                 # LLM-powered auto-fix

  workspace/                 # Workspace management
    workspace.go             # State management
    runner_v2.go             # SystemModel pipeline
    coverage_runner.go       # Coverage-guided generation

pkg/
  dsl/types.go               # DSL type definitions
  model/                     # Universal System Model
    model.go                 # SystemModel, Function, Endpoint
    intent.go                # TestIntent, TestPlan
    spec.go                  # TestSpec, TestSpecSet
    planner.go               # Test planning logic
    adapter.go               # Parser → Model adapter
```

---

## Current Capabilities

### CLI Commands

| Command | Description |
|---------|-------------|
| `qtest parse -f FILE` | Parse source file, show functions |
| `qtest generate-file -f FILE --write` | Generate tests for single file |
| `qtest workspace init URL` | Initialize workspace from repo |
| `qtest workspace run` | Run incremental test generation |
| `qtest coverage collect` | Collect code coverage |
| `qtest coverage analyze` | Analyze coverage gaps |
| `qtest coverage generate` | Coverage-guided test generation |
| `qtest contract generate` | Generate API contracts |
| `qtest contract validate` | Validate API against contracts |
| `qtest datagen generate` | Generate test data |
| `qtest validate run` | Run and validate tests |
| `qtest validate fix` | Auto-fix failing tests |
| `qtest e2e discover` | Auto-discover user flows using LLM |
| `qtest e2e generate` | Generate Playwright tests from flows |
| `qtest e2e run` | Run E2E tests |
| `qtest e2e list` | List flow specifications |

### Framework Supplements

| Framework | Language | File |
|-----------|----------|------|
| Express | JavaScript | supplements/express.go |
| FastAPI | Python | supplements/fastapi.go |
| Gin | Go | supplements/gin.go |
| Spring Boot | Java | supplements/springboot.go |
| Django REST | Python | supplements/django.go |

### Test Emitters

| Emitter | Framework | Language |
|---------|-----------|----------|
| supertest | supertest | JavaScript |
| pytest | pytest | Python |
| go-http | net/http | Go |

### LLM Configuration

- **Tier 1** (fast): qwen2.5-coder:7b
- **Tier 2** (balanced): deepseek-coder-v2:16b
- **Tier 3** (thorough): deepseek-coder-v2:16b
- Requires: `ollama serve` running

---

## Key Workflows

### 1. Single File Test Generation
```bash
./bin/qtest generate-file -f mycode.go -t 2 -m 5 --write
```

### 2. Workspace-Based Generation
```bash
./bin/qtest workspace init https://github.com/user/repo
./bin/qtest workspace run myrepo
```

### 3. Coverage-Guided Generation
```bash
./bin/qtest coverage generate -d . -t 80 -i 5
```

### 4. Contract Testing
```bash
./bin/qtest contract generate -m model.json -o contracts.json
./bin/qtest contract validate -c contracts.json --url http://localhost:3000
```

### 5. E2E Test Generation
```bash
# Discover user flows (requires playwright sidecar)
./bin/qtest e2e discover -u https://example.com -o flows.yaml

# Generate Playwright tests from flows
./bin/qtest e2e generate -f flows.yaml -o tests/ --enhance

# Run E2E tests
./bin/qtest e2e run -d tests/ --browser chromium
```

---

## Quick Start

```bash
# 1. Ensure Ollama is running
ollama serve

# 2. Build the binary
cd /home/satish/QTest
go build -o ./bin/qtest ./cmd/cli/

# 3. Generate tests for a file
./bin/qtest generate-file -f examples/math.go -t 1 -m 2 --write

# 4. Run generated tests
cd examples && go test -v
```

---

## Recent Changes (2025-11-24)

1. **E2E Test Generation CLI** - `qtest e2e` commands for E2E test workflow
   - `qtest e2e discover` - Auto-discover user flows using LLM (requires playwright sidecar)
   - `qtest e2e generate` - Generate Playwright tests from flow specifications
   - `qtest e2e run` - Run E2E tests with result parsing and reporting
   - `qtest e2e list` - List available flow specifications
2. **E2E Package** - internal/e2e/ with playwright.go, runner.go, llm_enhancer.go (32 tests)
3. **Flow Package** - internal/flow/ with discovery.go for LLM-based flow detection

### Previous Changes (2025-11-23)

1. **CLI Auth Commands** - `qtest auth login/logout/status` for API key authentication
   - Credentials stored in `~/.qtest/credentials.json` with 0600 permissions
   - Supports env var `QTEST_API_KEY` and `QTEST_API_URL`
   - 10 tests in internal/cliauth/credentials_test.go
2. **CI Workflow Generator** - `qtest ci` commands for GitHub Actions, GitLab CI, CircleCI
   - Auto-detects language (Go, Python, JS/TS, Java), framework, and services
   - Commands: `qtest ci generate`, `qtest ci detect`, `qtest ci preview`, `qtest ci list`
   - 15 tests in internal/ci/generator_test.go
3. **Enterprise Tests** - Unit tests for admin API (7 tests), rate limiter (11 tests), jobs hooks (9 tests), webhook events (10 tests)
4. **Rate Limiting Complete** - Memory + Redis storage backends with middleware
5. **Test Count ~155** - Comprehensive test coverage across all components

### Previous Changes (2025-11-22)
- GitHub OAuth with session management (27 tests)
- LLM usage tracking with budget limits
- Worker system fully implemented (6 workers)
- API tests (93 tests)
- Frontend initialization (Next.js 16)

### Earlier Changes (2025-11-21)
- Coverage-Guided Generation
- Spring Boot, Django REST Supplements
- Contract Testing & Test Data Generator
- Test Validation with LLM auto-fix

---

## Repository Structure

```
/home/satish/QTest/
├── bin/qtest              # Built binary
├── cmd/cli/               # CLI commands
├── docs/                  # Documentation
│   └── tracker.md         # Implementation tracker
├── examples/              # Example files
├── internal/              # Core implementation
│   ├── adapters/          # Test adapters
│   ├── codecov/           # Coverage collection
│   ├── contract/          # Contract testing
│   ├── datagen/           # Data generation
│   ├── emitter/           # Test emitters
│   ├── generator/         # DSL generator
│   ├── llm/               # LLM integration
│   ├── parser/            # Tree-sitter parsing
│   ├── specgen/           # Spec generation
│   ├── supplements/       # Framework supplements
│   ├── validator/         # Test validation
│   └── workspace/         # Workspace management
├── pkg/                   # Public packages
│   ├── dsl/               # DSL types
│   └── model/             # System model
├── go.mod
├── go.sum
├── CLAUDE.md              # Claude Code instructions
└── CONTEXT.md             # This file
```

---

## Webhooks

QTest supports webhooks for CI/CD integration and event notifications.

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/organizations/{orgID}/webhooks` | Create webhook |
| GET | `/api/v1/organizations/{orgID}/webhooks` | List webhooks |
| GET | `/api/v1/organizations/{orgID}/webhooks/{id}` | Get webhook |
| PATCH | `/api/v1/organizations/{orgID}/webhooks/{id}` | Update webhook |
| DELETE | `/api/v1/organizations/{orgID}/webhooks/{id}` | Delete webhook |
| GET | `/api/v1/organizations/{orgID}/webhooks/{id}/deliveries` | List deliveries |
| POST | `/api/v1/organizations/{orgID}/webhooks/{id}/test` | Send test webhook |

### Event Types

| Event | Description |
|-------|-------------|
| `job.completed` | Job finished successfully |
| `job.failed` | Job failed |
| `run.started` | Test generation run started |
| `run.completed` | Test generation run completed |
| `tests.generated` | Tests were generated |
| `tests.validated` | Tests were validated |
| `mutation.completed` | Mutation testing completed |

### Features

- **HMAC-SHA256 Signatures** - Verify webhook authenticity via `X-QTest-Signature` header
- **Exponential Backoff** - Automatic retries: 2s → 4s → 8s → 16s → 32s (up to 5 retries)
- **Background Dispatcher** - Reliable async delivery processing
- **HTTPS-Only** - Webhooks require HTTPS endpoints
- **Custom Headers** - Add custom headers to webhook requests

### Webhook Payload Format

```json
{
  "id": "evt_abc12345",
  "type": "job.completed",
  "created_at": "2025-11-23T12:00:00Z",
  "organization_id": "uuid",
  "data": {
    "job_id": "uuid",
    "job_type": "test_generation",
    "repository_id": "uuid",
    "status": "completed",
    "duration_ms": 5000
  }
}
```

### Signature Verification

```go
// Verify webhook signature
timestamp := r.Header.Get("X-QTest-Timestamp")
signature := r.Header.Get("X-QTest-Signature")
signatureData := timestamp + "." + string(payload)
expected := "sha256=" + hex.EncodeToString(hmac.New(sha256.New, []byte(secret)).Sum([]byte(signatureData)))
valid := hmac.Equal([]byte(signature), []byte(expected))
```

---

## Redis Rate Limiting

The rate limiter supports Redis for distributed rate limiting in production:

```go
// Configure with Redis backend
cfg := &ratelimit.Config{
    StorageBackend: "redis",
    RedisURL:       "redis://localhost:6379",
    RequestsPerMinute: 60,
}
```

## Usage Tracking

Track API usage with daily/monthly aggregated statistics:

| Endpoint | Description |
|----------|-------------|
| `GET /api/v1/organizations/{orgID}/usage/summary` | Usage summary (today/week/month) |
| `GET /api/v1/organizations/{orgID}/usage/daily` | Daily statistics |
| `GET /api/v1/organizations/{orgID}/usage/monthly` | Monthly statistics |
| `GET /api/v1/organizations/{orgID}/usage/recent` | Recent API calls |
| `GET /api/v1/organizations/{orgID}/usage/endpoints` | Stats by endpoint |

## Admin Endpoints

System-wide admin endpoints (require API key with `admin` scope):

| Endpoint | Description |
|----------|-------------|
| `GET /api/v1/admin/stats` | System-wide statistics |
| `GET /api/v1/admin/organizations` | List all organizations |
| `GET /api/v1/admin/users` | List all users |
| `GET /api/v1/admin/users/{id}` | Get user details |
| `PATCH /api/v1/admin/users/{id}` | Update user (activate/deactivate) |
| `GET /api/v1/admin/jobs` | List all jobs |
| `POST /api/v1/admin/jobs/{id}/cancel` | Cancel a job |
| `GET /api/v1/admin/audit-logs` | System audit logs |

## Webhook Event Integration

Webhooks are automatically triggered on job lifecycle events:

- **job.completed** - Triggered when a job completes successfully
- **job.failed** - Triggered when a job fails (after all retries exhausted)

To enable webhook events on job completion, set the event hook on the job repository:
```go
handler := webhook.NewJobEventHandler(webhookService, store)
jobRepo.SetEventHook(handler.HandleEvent)
```

---

## Remaining Gaps (Priority)

All P1 tasks completed!

### Completed (Previously Gaps)
- ✅ GitHub PR Integration - internal/github/pr.go
- ✅ JUnit Emitter - emitter/junit.go
- ✅ LLM Cache/Budget - internal/llm/cache.go, usage.go
- ✅ Rate Limiting - internal/api/ratelimit/
- ✅ Database Integration Tests - internal/db/*_integration_test.go (fixed import cycle, added testcontainers-go)
- ✅ GitHub App Auth - internal/github/app.go (JWT generation, installation tokens, 15 tests)
- ✅ OpenAPI Documentation - internal/api/docs.go (/docs endpoints for Swagger UI, ReDoc)
