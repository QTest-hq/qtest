# QTest Development Context Document

**Date:** 2025-11-21
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

  emitter/                   # TestSpec → Test code emitters
    supertest.go             # JavaScript API tests
    pytest.go                # Python API tests
    go_http.go               # Go API tests

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

## Recent Changes (2025-11-21)

1. **Coverage-Guided Generation** - Iterative test generation to reach coverage targets
2. **Spring Boot Supplement** - Java REST endpoint detection
3. **Django REST Supplement** - Python DRF endpoint detection
4. **Contract Testing** - API contract generation and validation
5. **Test Data Generator** - Field-aware and schema-based data generation
6. **Test Validation** - Run tests with LLM-powered auto-fix

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

## Remaining Gaps (Priority)

1. **GitHub PR Integration** - Auto-create PRs with generated tests
2. **Java Test Emitter (JUnit)** - Spring Boot detection exists, no emitter
3. **LLM Cache/Budget** - Cost control for LLM calls
4. **Worker System** - Async job processing
5. **Web Dashboard** - Visual interface
