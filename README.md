# QTest - AI Testing Orchestrator Platform

> Transform any GitHub repository or website URL into a complete, continuously maintained test pyramid using AI-driven system modeling, risk-based planning, and autonomous test generation.

## Vision

The universal **AI-driven verification layer** for all AI-generated and human-written software. QTest automatically understands systems, plans tests, generates tests, fixes flaky tests, updates tests during repo evolution, and provides complete confidence in delivery pipelines.

## What QTest Does

```
GitHub Repo / Website URL
         ↓
    [System Modeling]
         ↓
    [Test Planning]
         ↓
    [Test Generation]
         ↓
Unit + Integration + API + E2E Tests
         ↓
    [Mutation Validation]
         ↓
    [GitHub PR with Tests]
```

## Key Features

- **Multi-language repo analysis** - TypeScript, Python, Java, Go (via Tree-sitter + native parsers)
- **Website crawling** - Playwright-based flow discovery and API inference
- **Full test pyramid** - Unit, Integration, API, and E2E tests from a single input
- **Mutation-validated tests** - Only ship tests that catch real bugs
- **Continuous maintenance** - Drift detection, auto-updates, flakiness removal
- **GitHub native** - PRs with generated tests, CI pipeline generation

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                         INGESTION LAYER                             │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐ │
│  │  Repo Cloner    │    │ Playwright      │    │  Auth Handler   │ │
│  │  (GitHub API)   │    │ Crawler         │    │  (OAuth/Keys)   │ │
│  └────────┬────────┘    └────────┬────────┘    └────────┬────────┘ │
└───────────┼──────────────────────┼──────────────────────┼──────────┘
            │                      │                      │
            ▼                      ▼                      ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         MODELING ENGINE                             │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐ │
│  │  Tree-sitter    │    │  Network Flow   │    │  Framework      │ │
│  │  AST Parser     │    │  Analyzer       │    │  Detectors      │ │
│  └────────┬────────┘    └────────┬────────┘    └────────┬────────┘ │
└───────────┼──────────────────────┼──────────────────────┼──────────┘
            │                      │                      │
            ▼                      ▼                      ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    UNIVERSAL SYSTEM MODEL (JSON)                    │
│  Endpoints • Functions • Classes • Routes • Dependencies • Flows    │
└─────────────────────────────────────────────────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         TEST PLANNER                                │
│  Risk Ranking • Pyramid Distribution • Critical Path Analysis       │
└─────────────────────────────────────────────────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      TEST DSL GENERATOR                             │
│  System Model → Test Intent → Test DSL (YAML)                       │
└─────────────────────────────────────────────────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     FRAMEWORK ADAPTERS                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐           │
│  │  Jest    │  │  Pytest  │  │  JUnit   │  │Playwright│           │
│  │  Adapter │  │  Adapter │  │  Adapter │  │  Adapter │           │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘           │
└─────────────────────────────────────────────────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      QUALITY GATES                                  │
│  Compilation Check → Runtime Validation → Mutation Testing          │
└─────────────────────────────────────────────────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      INTEGRATIONS                                   │
│  GitHub PR • CI Pipeline Generation • Coverage Reports              │
└─────────────────────────────────────────────────────────────────────┘
```

## Project Structure

```
QTest/
├── cmd/                    # Entry points
│   ├── api/               # API server
│   ├── worker/            # Test generation workers
│   └── cli/               # CLI tool
├── internal/              # Private packages
│   ├── parser/            # AST/Tree-sitter parsing
│   ├── model/             # System model builder
│   ├── planner/           # Test planning engine
│   ├── generator/         # Test DSL generator
│   ├── adapters/          # Framework adapters
│   ├── mutation/          # Mutation testing service
│   ├── crawler/           # Playwright crawler
│   └── github/            # GitHub App integration
├── pkg/                   # Public packages
│   └── dsl/               # Test DSL types
├── web/                   # Next.js dashboard
├── docker/                # Container configs
└── docs/                  # Documentation
```

## Documentation

| Document | Description |
|----------|-------------|
| [Architecture](docs/architecture.md) | Full system design and data flow |
| [FRD](docs/frd.md) | Functional Requirements Document |
| [PRD](docs/prd.md) | Product Requirements Document |
| [Test DSL Spec](docs/test-dsl-spec.md) | Test DSL specification |
| [Data Schemas](docs/data-schemas.md) | Go struct definitions |
| [Tech Stack](docs/tech-stack.md) | Technology decisions |
| [Mutation Strategy](docs/mutation-strategy.md) | Mutation testing approach |
| [LLM Cost Management](docs/llm-cost-management.md) | LLM tiering and budgets |
| [Tracker](docs/tracker.md) | Implementation tracker |

## Tech Stack

| Layer | Technology | Rationale |
|-------|------------|-----------|
| Backend | Go | Fast, concurrent, single binary deployment |
| Parsing | Tree-sitter + native parsers | Multi-language support |
| LLM | Tiered (Haiku/Sonnet/Opus) | Cost efficiency |
| Crawling | Playwright | Best E2E engine |
| Mutation | Stryker/Pitest/Mutmut | Language-specific tools |
| Queue | NATS JetStream | High throughput workers |
| Database | PostgreSQL + Redis | Reliable + fast caching |
| Frontend | Next.js + React | Modern dashboard |
| CI | GitHub App + Actions | Native integration |

## Roadmap

### Phase 1: MVP (8-12 weeks)
- Repo ingestion (JS/TS)
- AST extraction via Tree-sitter
- System model v1
- Unit + API test generation
- Jest adapter
- GitHub App PR automation
- Basic CI pipeline generation

### Phase 2: E2E + Website (6-8 weeks)
- Playwright crawler
- Flow discovery
- API inference from network traffic
- E2E test generation
- Unified test plan output

### Phase 3: Maintenance + Full Pyramid (6-8 weeks)
- Drift detection system
- Test update PRs
- Flakiness detection
- Coverage & mutation scoring
- Pyramid planner

### Phase 4: Enterprise (Months 4-6)
- Multi-language support (Python, Java, Go)
- RBAC, SSO, Team dashboards
- Advanced analytics

## Quick Start

```bash
# Clone the repository
git clone https://github.com/your-org/qtest.git
cd qtest

# Build
go build -o ./bin/qtest ./cmd/cli

# Analyze a repository
./bin/qtest analyze -p ./my-project

# Generate tests for a single file
./bin/qtest generate-file -f ./path/to/source.go -t 1 -m 5 --write

# Generate tests for a full repo
./bin/qtest generate -r ./my-project

# Run mutation testing
./bin/qtest mutation run -s calculator.go -t calculator_test.go
```

## CLI Commands

### Analysis & Generation

| Command | Description |
|---------|-------------|
| `qtest analyze -p PATH` | Analyze repository structure and detect test targets |
| `qtest analyze --json` | Output analysis as JSON |
| `qtest analyze --coverage` | Include coverage analysis |
| `qtest generate -r REPO` | Generate tests for entire repository |
| `qtest generate-file -f FILE` | Generate tests for single file |
| `qtest parse -f FILE` | Parse source file and show functions |

### Coverage

| Command | Description |
|---------|-------------|
| `qtest coverage collect` | Run tests and collect coverage |
| `qtest coverage collect --json` | Output coverage as JSON |
| `qtest coverage collect --html DIR` | Generate HTML report |
| `qtest coverage analyze -r FILE` | Analyze coverage gaps |
| `qtest coverage gaps -r FILE` | Generate test intents for gaps |
| `qtest coverage generate` | Generate tests to improve coverage |
| `qtest coverage report -r FILE` | View/export coverage report |
| `qtest coverage ci -t 80` | CI check with threshold enforcement |

### Mutation Testing

| Command | Description |
|---------|-------------|
| `qtest mutation run -s SRC -t TEST` | Run mutation testing |
| `qtest mutation run --mode thorough` | Thorough mutation analysis |
| `qtest mutation report -f FILE` | View mutation report |

### Workspace Management

| Command | Description |
|---------|-------------|
| `qtest workspace init URL` | Initialize workspace from repo |
| `qtest workspace list` | List all workspaces |
| `qtest workspace status NAME` | Show workspace status |
| `qtest workspace run NAME` | Run test generation |

### Configuration

| Command | Description |
|---------|-------------|
| `qtest config` | Show current configuration |
| `qtest validate -f FILE` | Validate generated tests |

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `OLLAMA_URL` | Ollama server URL | `http://localhost:11434` |
| `OLLAMA_TIER1_MODEL` | Fast model | `qwen2.5-coder:7b` |
| `OLLAMA_TIER2_MODEL` | Balanced model | `deepseek-coder-v2:16b` |
| `ANTHROPIC_API_KEY` | Anthropic API key (Tier 3) | - |
| `GITHUB_TOKEN` | GitHub token for private repos | - |

## License

[License TBD]

## Contributing

[Contributing guidelines TBD]
