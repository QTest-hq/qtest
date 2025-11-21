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

**Not yet implemented:**
- API test generation (from detected endpoints)
- E2E test generation (Playwright)
- Mutation testing validation

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

**Job Pipeline:** `ingestion → modeling → planning → generation → integration`

**Worker types:** Run with `WORKER_TYPE=all` (default) or specific: `ingestion`, `modeling`, `planning`, `generation`, `mutation`, `integration`

## Key Files When Debugging Test Generation

1. `internal/llm/prompts.go` - What we ask the LLM
2. `internal/generator/converter.go` - YAML parsing, variable resolution
3. `internal/adapters/go_adapter.go` - Code generation, assertion rendering
4. `cmd/cli/main.go:writeTestFiles()` - How tests get combined and written
