# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

QTest is an AI-powered test generation platform. It parses source code using Tree-sitter, sends functions to LLMs (Ollama by default) for test specification generation, and converts the YAML output to language-specific test code.

## Build & Development Commands

```bash
# Build CLI
go build -o ./bin/qtest ./cmd/cli/

# Build all (api, worker, cli)
make build

# Run tests
go test ./...                           # All tests
go test -v ./internal/generator/...     # Single package
go test -v -run TestConvertToDSL ./internal/generator/...  # Single test

# Lint
make lint

# Generate tests using QTest itself (requires: ollama serve)
./bin/qtest generate-file -f <source-file> -t 1 -m 5 --write

# Parse a file to see extracted functions
./bin/qtest parse -f <source-file>
```

## Architecture

### Universal System Model (The Core IR)

QTest uses a **Universal System Model** as its language-agnostic intermediate representation:

```
Source Files → Tree-sitter Parser → System Model ← Framework Supplements
                                         ↓
                                   Test Targets
                                         ↓
                                  LLM Generation
                                         ↓
                              Framework Adapters
                                         ↓
                                   Test Code
```

**Key packages:**
- `pkg/model/` - Universal System Model schema and builder
- `internal/supplements/` - Framework-specific endpoint detectors (Express, FastAPI, Gin)

**Full Test Generation Pipeline:**
```bash
# 1. Build system model (parse code, detect endpoints)
./bin/qtest model build -d <directory> -o model.json

# 2. Generate test plan (prioritize what to test)
./bin/qtest plan generate -m model.json -o plan.json

# 3. Generate test specs via LLM
./bin/qtest generate-specs -m model.json -p plan.json -o specs.json -t 1

# 4. Emit test code from specs
./bin/qtest emit-tests -s specs.json -o ./tests --emitter supertest  # Jest
./bin/qtest emit-tests -s specs.json -o ./tests --emitter pytest     # pytest
./bin/qtest emit-tests -s specs.json -o ./tests --emitter go-http    # Go
```

### Pipeline Flow
```
Source Files → Tree-sitter → SystemModel → Planner → TestIntents → LLM → TestSpecs → Adapters → Test Code
```

### Key Components

**Universal System Model** (`pkg/model/`):
- `model.go` - Schema for modules, functions, endpoints, types, test targets
- `builder.go` - Builds model from parsed files, runs supplements, computes risk scores
- `adapter.go` - Bridges parser output to model builder
- `intent.go` - TestIntent (what to test) and TestPlan types
- `spec.go` - TestSpec (detailed test specification) with assertions
- `planner.go` - Generates prioritized TestIntents from SystemModel

**Spec Generator** (`internal/specgen/`):
- `generator.go` - Uses LLM to convert TestIntent → TestSpec

**Test Emitters** (`internal/emitter/`):
- `supertest.go` - Jest + Supertest for Express/Node.js APIs
- `pytest.go` - pytest + httpx for FastAPI/Python APIs
- `go_http.go` - Go net/http testing

**Framework Supplements** (`internal/supplements/`):
- `express.go` - Detects Express.js routes (app.get, router.post, etc.)
- `fastapi.go` - Detects FastAPI routes (@app.get decorators)
- `gin.go` - Detects Gin routes (r.GET, router.POST)
- `registry.go` - Auto-detects which supplements to run

**Generator Pipeline** (`internal/generator/`):
- `generator.go` - Orchestrates LLM calls, builds context from parsed functions
- `converter.go` - Converts LLM YAML output to internal DSL. Handles multiple YAML formats (`assertions:`, `assert:`, `expected:`, `expect: "result == X"`)

**LLM Layer** (`internal/llm/`):
- `router.go` - Tier-based routing with retry logic and exponential backoff
- `ollama.go` / `anthropic.go` - Provider clients
- `prompts.go` - System prompts for test generation

**Framework Adapters** (`internal/adapters/`):
- `go_adapter.go` - Generates Go test code from DSL
- Uses `var result interface{}` pattern to avoid redeclaration errors
- `formatGoArg()` handles type conversion and unresolved variable defaults

**Parser** (`internal/parser/`):
- Tree-sitter based parsing for Go, Python, JavaScript, TypeScript
- Extracts functions, methods, classes, parameters

### DSL Format
LLM returns YAML that converter transforms to `pkg/dsl/types.go` structures:
```yaml
- name: "Test case name"
  setup: {a: 1, b: 2}
  action: "FunctionName(a, b)"
  assertions: {result: 3}
```

### LLM Tiers
- **Tier 1** (fast): qwen2.5-coder:7b - use for most generation
- **Tier 2** (balanced): deepseek-coder-v2:16b
- **Tier 3** (thorough): claude-3-opus - complex reasoning

## Current Implementation Status

**Working (Phase 1):**
- Universal System Model with framework supplements
- API endpoint detection (Express, FastAPI, Gin)
- CLI parsing and test generation for Go files
- LLM integration with Ollama (local) and Anthropic
- Go test adapter with assertions
- Workspace management
- Risk scoring and test target prioritization
- Mutation testing with HTML/JSON/text reports
- Pipeline integration with `run_mutation` flag

**Not yet implemented:**
- API test generation (from detected endpoints)
- E2E test generation (Playwright)

## Jobs API & Worker System

Async job processing via NATS JetStream with REST API:

```bash
# Start test generation pipeline
curl -X POST http://localhost:8080/api/v1/jobs/pipeline \
  -d '{"repository_url": "https://github.com/user/repo", "max_tests": 50}'

# List jobs
curl "http://localhost:8080/api/v1/jobs?status=running"

# Get job with children
curl http://localhost:8080/api/v1/jobs/{id}

# Cancel/retry
curl -X POST http://localhost:8080/api/v1/jobs/{id}/cancel
curl -X POST http://localhost:8080/api/v1/jobs/{id}/retry
```

**Job Pipeline:** `ingestion → modeling → planning → generation → [mutation] → integration`

**Worker types:** Run with `WORKER_TYPE=all` (default) or specific: `ingestion`, `modeling`, `planning`, `generation`, `mutation`, `integration`

## Mutation Testing

QTest includes mutation testing to evaluate test quality. Mutation testing introduces small changes (mutations) to source code and verifies tests can detect them.

### CLI Commands

```bash
# Run mutation testing on a single file
./bin/qtest mutation run -s calculator.go -t calculator_test.go

# Thorough mode with more mutants
./bin/qtest mutation run -s calculator.go --mode thorough

# Save report (format detected by extension)
./bin/qtest mutation run -s calculator.go -o report.json
./bin/qtest mutation run -s calculator.go -o report.html

# View/convert existing reports
./bin/qtest mutation report -f report.json                   # Text view
./bin/qtest mutation report -f report.json --format html -o ./reports  # Generate HTML
```

### Quality Thresholds

- **Good (70%+):** Tests effectively detect mutations
- **Acceptable (50-70%):** Room for improvement
- **Poor (<50%):** Tests need significant enhancement

### Pipeline Integration

Enable mutation testing in the pipeline by setting `run_mutation: true`:

```bash
curl -X POST http://localhost:8080/api/v1/jobs/pipeline \
  -d '{"repository_url": "https://github.com/user/repo", "run_mutation": true}'
```

### API Endpoints

```bash
# Start mutation testing
curl -X POST http://localhost:8080/api/v1/mutation \
  -d '{"source_file_path": "calc.go", "test_file_path": "calc_test.go"}'

# Get mutation run results
curl http://localhost:8080/api/v1/mutation/{id}

# List mutation runs for a repository
curl http://localhost:8080/api/v1/repos/{repo_id}/mutation
```

### Report Formats

- **JSON:** Machine-readable, includes all mutant details
- **HTML:** Visual report with score visualization and mutant breakdown
- **Text:** Terminal-friendly summary with surviving mutant highlights

## CI Workflow Generation

QTest can generate CI/CD workflow configurations for multiple platforms.

### CLI Commands

```bash
# Auto-detect project and generate workflow
./bin/qtest ci generate

# Generate for specific platform
./bin/qtest ci generate -p github-actions    # GitHub Actions (default)
./bin/qtest ci generate -p gitlab-ci         # GitLab CI
./bin/qtest ci generate -p circleci          # CircleCI

# Specify language explicitly
./bin/qtest ci generate -l python -p gitlab-ci

# Include QTest in CI pipeline
./bin/qtest ci generate --qtest --qtest-tier 2

# Include service containers
./bin/qtest ci generate --services postgres,redis

# Preview without writing
./bin/qtest ci preview -p github-actions

# Detect project configuration
./bin/qtest ci detect

# List supported platforms and languages
./bin/qtest ci list
```

### Supported Platforms

| Platform | Output File |
|----------|-------------|
| `github-actions` | `.github/workflows/ci.yml` |
| `gitlab-ci` | `.gitlab-ci.yml` |
| `circleci` | `.circleci/config.yml` |

### Supported Languages

| Language | Build | Test | Lint |
|----------|-------|------|------|
| `go` | `go build ./...` | `go test -v -race -coverprofile=coverage.out ./...` | `golangci-lint run` |
| `python` | `pip install -r requirements.txt` | `pytest --cov=. --cov-report=xml -v` | `ruff check .` |
| `javascript` | `npm ci` | `npm test -- --coverage` | `npm run lint` |
| `typescript` | `npm ci` | `npm test -- --coverage` | `npm run lint` |
| `java` | `mvn compile` | `mvn test` | `mvn checkstyle:check` |

### Auto-Detection

The generator auto-detects:
- **Language**: From `go.mod`, `package.json`, `requirements.txt`, `pom.xml`
- **Framework**: Express, FastAPI, Flask, Gin, Spring Boot, etc.
- **Services**: From `docker-compose.yml` (postgres, redis, mysql, mongo)
- **Test framework**: Jest, pytest, Go test, JUnit

### Key Files

- `internal/ci/generator.go` - Workflow generation logic
- `cmd/cli/ci.go` - CLI commands
- `internal/ci/generator_test.go` - 15 unit tests

## CLI Authentication

The CLI supports API key authentication for accessing QTest API services.

### CLI Commands

```bash
# Login with API key (interactive prompt)
./bin/qtest auth login

# Login with API key directly
./bin/qtest auth login --token qtest_xxxxxxxxxxxx

# Login without server validation
./bin/qtest auth login --token qtest_xxxxxxxxxxxx --validate=false

# Show current auth status
./bin/qtest auth status

# Validate credentials with server
./bin/qtest auth status --check

# Remove stored credentials
./bin/qtest auth logout
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `QTEST_API_KEY` | API key for authentication (takes precedence over stored credentials) |
| `QTEST_API_URL` | API server URL (default: `http://localhost:8080`) |

### Credential Storage

- Credentials stored in `~/.qtest/credentials.json`
- File permissions: `0600` (owner read/write only)
- Contains: API key, server URL, user info (if validated)

### Key Files

- `internal/cliauth/credentials.go` - Credential management
- `cmd/cli/auth.go` - CLI commands (login, logout, status)
- `internal/cliauth/credentials_test.go` - 10 unit tests

## Key Files When Debugging Test Generation

1. `internal/llm/prompts.go` - What we ask the LLM
2. `internal/generator/converter.go` - YAML parsing, variable resolution
3. `internal/adapters/go_adapter.go` - Code generation, assertion rendering
4. `cmd/cli/main.go:writeTestFiles()` - How tests get combined and written
