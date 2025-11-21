# QTest Architecture

## Overview

QTest is a distributed system designed for high-throughput test generation. The architecture follows a pipeline pattern where repositories flow through ingestion, modeling, planning, generation, validation, and integration stages.

## System Context

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              EXTERNAL SYSTEMS                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐            │
│   │  GitHub  │    │ Target   │    │ Cloud LLM│    │ Mutation │            │
│   │   API    │    │ Websites │    │   APIs   │    │  Tools   │            │
│   └────┬─────┘    └────┬─────┘    └────┬─────┘    └────┬─────┘            │
│        │               │               │               │                   │
└────────┼───────────────┼───────────────┼───────────────┼───────────────────┘
         │               │               │ (optional)    │
         ▼               ▼               ▼               ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                               QTEST PLATFORM                                 │
│                                                                              │
│  ┌───────────────────────────────────────────────────────────────────────┐ │
│  │                           API GATEWAY (Go)                             │ │
│  │                    Rate Limiting • Auth • Routing                      │ │
│  └───────────────────────────────────────────────────────────────────────┘ │
│                                    │                                        │
│         ┌──────────────────────────┼──────────────────────────┐            │
│         ▼                          ▼                          ▼            │
│  ┌─────────────┐          ┌─────────────┐          ┌─────────────┐        │
│  │   Web UI    │          │   CLI       │          │  GitHub App │        │
│  │  (Next.js)  │          │   (Go)      │          │    (Go)     │        │
│  └─────────────┘          └─────────────┘          └─────────────┘        │
│                                                                              │
│  ┌───────────────────────────────────────────────────────────────────────┐ │
│  │                        WORKER POOL (NATS)                              │ │
│  │  Ingestion • Modeling • Generation • Mutation • Integration Workers    │ │
│  └───────────────────────────────────────────────────────────────────────┘ │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                     LLM ROUTER SERVICE (Internal)                       ││
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                     ││
│  │  │   Ollama    │  │  Anthropic  │  │   OpenAI    │  ← Pluggable        ││
│  │  │   (Local)   │  │   (Cloud)   │  │   (Cloud)   │    Providers        ││
│  │  │  DEFAULT    │  │  Optional   │  │  Optional   │                     ││
│  │  └─────────────┘  └─────────────┘  └─────────────┘                     ││
│  │  Task Routing • Tier Selection • Budget Management • Caching            ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
│  ┌─────────────────────────────┐  ┌─────────────────────────────┐         │
│  │      PostgreSQL             │  │         Redis               │         │
│  │  System Models • Results    │  │  Queue • Cache • Sessions   │         │
│  └─────────────────────────────┘  └─────────────────────────────┘         │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Key Architecture Decision: LLM Router is Internal**

The LLM Router Service is an **internal component** (not external) that:
- Defaults to **Ollama** for local GPU inference (4080, etc.)
- Can be configured to use Claude/OpenAI APIs when needed
- Routes requests based on task type (`unit_test`, `api_test`, `critic`, etc.)
- Manages tiered model selection (cheap vs expensive)
- Handles caching, budget enforcement, and fallbacks

## Core Components

### 1. Ingestion Layer

Responsible for fetching and preparing source material for analysis.

```
┌─────────────────────────────────────────────────────────────────┐
│                     INGESTION LAYER                              │
│                                                                  │
│  ┌──────────────────┐  ┌──────────────────┐  ┌───────────────┐ │
│  │   RepoCloner     │  │ PlaywrightCrawler│  │  AuthManager  │ │
│  │                  │  │                  │  │               │ │
│  │ • Clone repos    │  │ • Crawl pages    │  │ • OAuth flow  │ │
│  │ • Fetch branches │  │ • Record network │  │ • Token store │ │
│  │ • Detect lang    │  │ • DOM snapshots  │  │ • Key mgmt    │ │
│  │ • File tree      │  │ • Flow detection │  │               │ │
│  └────────┬─────────┘  └────────┬─────────┘  └───────┬───────┘ │
│           │                     │                    │          │
│           └─────────────────────┼────────────────────┘          │
│                                 ▼                               │
│                    ┌────────────────────────┐                   │
│                    │   RawSourceBundle      │                   │
│                    │   • Files[]            │                   │
│                    │   • NetworkLogs[]      │                   │
│                    │   • DOMSnapshots[]     │                   │
│                    │   • Metadata           │                   │
│                    └────────────────────────┘                   │
└─────────────────────────────────────────────────────────────────┘
```

**Components:**

| Component | Responsibility | Technology |
|-----------|---------------|------------|
| RepoCloner | Clone GitHub repos, detect languages | go-git, GitHub API |
| PlaywrightCrawler | Crawl websites, capture flows | Playwright (Node.js sidecar) |
| AuthManager | Handle OAuth, API keys, credentials | Vault/encrypted storage |

### 2. Modeling Engine

Transforms raw source into a Universal System Model.

```
┌─────────────────────────────────────────────────────────────────┐
│                     MODELING ENGINE                              │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                   PARSER REGISTRY                         │  │
│  │                                                           │  │
│  │  ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌──────────┐ │  │
│  │  │Tree-sitter│ │ ts-morph  │ │  Python   │ │JavaParser│ │  │
│  │  │(Universal)│ │  (TS/JS)  │ │    ast    │ │  (Java)  │ │  │
│  │  └─────┬─────┘ └─────┬─────┘ └─────┬─────┘ └────┬─────┘ │  │
│  │        │             │             │            │        │  │
│  │        └─────────────┴──────┬──────┴────────────┘        │  │
│  │                             ▼                            │  │
│  │              ┌─────────────────────────┐                 │  │
│  │              │   Unified AST Adapter   │                 │  │
│  │              └───────────┬─────────────┘                 │  │
│  └──────────────────────────┼───────────────────────────────┘  │
│                             ▼                                   │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                  MODEL BUILDERS                           │  │
│  │                                                           │  │
│  │  ┌────────────┐ ┌────────────┐ ┌────────────┐           │  │
│  │  │  Function  │ │  Endpoint  │ │   Entity   │           │  │
│  │  │  Extractor │ │  Extractor │ │  Extractor │           │  │
│  │  └──────┬─────┘ └──────┬─────┘ └──────┬─────┘           │  │
│  │         │              │              │                  │  │
│  │         └──────────────┼──────────────┘                  │  │
│  │                        ▼                                 │  │
│  │         ┌─────────────────────────────┐                  │  │
│  │         │    Dependency Resolver      │                  │  │
│  │         │    (Build call graph)       │                  │  │
│  │         └─────────────┬───────────────┘                  │  │
│  └───────────────────────┼──────────────────────────────────┘  │
│                          ▼                                      │
│               ┌─────────────────────┐                           │
│               │  UNIVERSAL SYSTEM   │                           │
│               │       MODEL         │                           │
│               │                     │                           │
│               │ • Endpoints[]       │                           │
│               │ • Functions[]       │                           │
│               │ • Entities[]        │                           │
│               │ • Dependencies{}    │                           │
│               │ • Flows[]           │                           │
│               └─────────────────────┘                           │
└─────────────────────────────────────────────────────────────────┘
```

**What the System Model captures:**
- Entry points (functions, endpoints, CLI commands, UI pages)
- Dependencies (imports, service calls, DB access)
- Data flow (inputs, outputs, side effects)
- Behavioral clues (branches, guards, error cases)

### 3. Test Planner

Decides what to test and how based on risk analysis.

```
┌─────────────────────────────────────────────────────────────────┐
│                      TEST PLANNER                                │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐│
│  │                    RISK ANALYZER                            ││
│  │                                                             ││
│  │  Inputs:                                                    ││
│  │  • System Model                                             ││
│  │  • Change frequency (git history)                           ││
│  │  • Complexity metrics                                       ││
│  │  • Domain keywords (auth, payment, etc.)                    ││
│  │                                                             ││
│  │  Outputs:                                                   ││
│  │  • Risk score per function/endpoint                         ││
│  │  • Critical path identification                             ││
│  └────────────────────────────────────────────────────────────┘│
│                             │                                    │
│                             ▼                                    │
│  ┌────────────────────────────────────────────────────────────┐│
│  │                 PYRAMID DISTRIBUTOR                         ││
│  │                                                             ││
│  │  Rules:                                                     ││
│  │  • Pure functions → Unit tests                              ││
│  │  • Service methods with deps → Integration tests            ││
│  │  • HTTP endpoints → API tests                               ││
│  │  • User flows → E2E tests                                   ││
│  │                                                             ││
│  │  Output: TestPlan                                           ││
│  │  • targets: [{ target, testType, priority }]                ││
│  └────────────────────────────────────────────────────────────┘│
│                             │                                    │
│                             ▼                                    │
│                    ┌─────────────────┐                          │
│                    │    TestPlan     │                          │
│                    │                 │                          │
│                    │ • UnitTargets[] │                          │
│                    │ • APITargets[]  │                          │
│                    │ • E2ETargets[]  │                          │
│                    │ • Priorities{}  │                          │
│                    └─────────────────┘                          │
└─────────────────────────────────────────────────────────────────┘
```

### 4. Test Generator

Converts test targets into Test DSL using the LLM Router Service.

```
┌─────────────────────────────────────────────────────────────────┐
│                     TEST GENERATOR                               │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐│
│  │                  CONTEXT BUILDER                            ││
│  │                                                             ││
│  │  For each target, assemble:                                 ││
│  │  • Function signature                                       ││
│  │  • Branch conditions                                        ││
│  │  • Dependency signatures                                    ││
│  │  • Sample usage (from call sites)                           ││
│  │  • Domain hints                                             ││
│  └────────────────────────────────────────────────────────────┘│
│                             │                                    │
│                             ▼                                    │
│  ┌────────────────────────────────────────────────────────────┐│
│  │              LLM ROUTER SERVICE (POST /llm/generate)        ││
│  │                                                             ││
│  │  Request:                                                   ││
│  │  • task_type: unit_test | api_test | e2e_flow | critic      ││
│  │  • model_tier: TIER1 | TIER2 | TIER3 | AUTO                 ││
│  │  • context: { function_meta, branches, ... }                ││
│  │                                                             ││
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        ││
│  │  │   Tier 1    │  │   Tier 2    │  │   Tier 3    │        ││
│  │  │  Qwen 7B    │  │  Qwen 32B   │  │ DeepSeek 70B│        ││
│  │  │  (Ollama)   │  │  (Ollama)   │  │ (Ollama)    │        ││
│  │  │             │  │             │  │             │        ││
│  │  │ Boilerplate │  │ Test logic  │  │ Critic pass │        ││
│  │  │ Summaries   │  │ Assertions  │  │ Complex     │        ││
│  │  └─────────────┘  └─────────────┘  └─────────────┘        ││
│  │                                                             ││
│  │  Fallback: Claude/OpenAI APIs if local unavailable          ││
│  │  Budget Manager: per-repo limits, caching, tier routing     ││
│  └────────────────────────────────────────────────────────────┘│
│                             │                                    │
│                             ▼                                    │
│  ┌────────────────────────────────────────────────────────────┐│
│  │                   DSL OUTPUT                                ││
│  │                                                             ││
│  │  TestDSL[]                                                  ││
│  │  • id, type, level, description                             ││
│  │  • target (endpoint/function/flow)                          ││
│  │  • setup, input, expect                                     ││
│  │  • lifecycle, isolation, resources                          ││
│  └────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

**LLM Router Service API:**

```
POST /llm/generate
Content-Type: application/json

{
  "task_type": "unit_test",
  "model_tier": "auto",
  "context": {
    "function_code": "...",
    "function_meta": { ... },
    "target_branches": [ ... ]
  },
  "max_tokens": 2000,
  "temperature": 0.3
}

Response:
{
  "content": "...(generated test DSL)...",
  "provider": "ollama",
  "model": "qwen2.5:32b",
  "tier": "tier2",
  "input_tokens": 1500,
  "output_tokens": 800,
  "latency_ms": 2340,
  "cached": false
}
```

### 5. Framework Adapters

Convert Test DSL to framework-specific code.

```
┌─────────────────────────────────────────────────────────────────┐
│                   FRAMEWORK ADAPTERS                             │
│                                                                  │
│  Input: TestDSL[]                                               │
│                                                                  │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌───────────┐ │
│  │    Jest     │ │   Pytest    │ │   JUnit     │ │ Playwright│ │
│  │   Adapter   │ │   Adapter   │ │   Adapter   │ │  Adapter  │ │
│  │             │ │             │ │             │ │           │ │
│  │ DSL → .ts   │ │ DSL → .py   │ │ DSL → .java │ │ DSL → .ts │ │
│  │             │ │             │ │             │ │           │ │
│  │ Templates:  │ │ Templates:  │ │ Templates:  │ │Templates: │ │
│  │ • describe  │ │ • def test_ │ │ • @Test     │ │ • test()  │ │
│  │ • it/test   │ │ • fixtures  │ │ • @Before   │ │ • page.   │ │
│  │ • expect    │ │ • assert    │ │ • assert    │ │ • expect  │ │
│  └──────┬──────┘ └──────┬──────┘ └──────┬──────┘ └─────┬─────┘ │
│         │               │               │              │        │
│         └───────────────┴───────┬───────┴──────────────┘        │
│                                 ▼                               │
│                    ┌────────────────────────┐                   │
│                    │   Generated Test Files │                   │
│                    │   (ready to execute)   │                   │
│                    └────────────────────────┘                   │
└─────────────────────────────────────────────────────────────────┘
```

### 6. Quality Gates

Validates generated tests before shipping.

```
┌─────────────────────────────────────────────────────────────────┐
│                     QUALITY GATES                                │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐│
│  │                  GATE 1: COMPILATION                        ││
│  │                                                             ││
│  │  • Run tsc/eslint for TS                                    ││
│  │  • Run mypy for Python                                      ││
│  │  • Run javac for Java                                       ││
│  │                                                             ││
│  │  Failure → Retry with error context OR discard              ││
│  └────────────────────────────────────────────────────────────┘│
│                             │                                    │
│                             ▼                                    │
│  ┌────────────────────────────────────────────────────────────┐│
│  │                  GATE 2: RUNTIME                            ││
│  │                                                             ││
│  │  • Execute test against original code                       ││
│  │  • Must pass consistently (3 runs)                          ││
│  │                                                             ││
│  │  Failure → Mark as "test is wrong" OR flaky                 ││
│  └────────────────────────────────────────────────────────────┘│
│                             │                                    │
│                             ▼                                    │
│  ┌────────────────────────────────────────────────────────────┐│
│  │                  GATE 3: MUTATION                           ││
│  │                                                             ││
│  │  • Generate 3-5 mutants per target function                 ││
│  │  • Run test against mutants                                 ││
│  │  • Test must fail ≥1 mutant to pass gate                    ││
│  │                                                             ││
│  │  Failure → Strengthen assertions OR discard                 ││
│  └────────────────────────────────────────────────────────────┘│
│                             │                                    │
│                             ▼                                    │
│                    ┌────────────────────────┐                   │
│                    │   VALIDATED TESTS      │                   │
│                    │   (mutation-proven)    │                   │
│                    └────────────────────────┘                   │
└─────────────────────────────────────────────────────────────────┘
```

### 7. Integration Layer

Delivers tests to user's repository.

```
┌─────────────────────────────────────────────────────────────────┐
│                   INTEGRATION LAYER                              │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐│
│  │                   GITHUB SERVICE                            ││
│  │                                                             ││
│  │  • Create branch                                            ││
│  │  • Commit generated tests                                   ││
│  │  • Open PR with summary                                     ││
│  │  • Generate CI workflow file                                ││
│  │                                                             ││
│  │  PR Description includes:                                   ││
│  │  • Tests generated count                                    ││
│  │  • Coverage delta                                           ││
│  │  • Mutation score                                           ││
│  │  • Quality breakdown                                        ││
│  └────────────────────────────────────────────────────────────┘│
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐│
│  │                   CI GENERATOR                              ││
│  │                                                             ││
│  │  Outputs:                                                   ││
│  │  • .github/workflows/qtest.yml                              ││
│  │  • Test commands per framework                              ││
│  │  • Coverage collection                                      ││
│  └────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

## Data Flow Sequence

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                        COMPLETE DATA FLOW                                     │
└──────────────────────────────────────────────────────────────────────────────┘

User Request (repo URL)
        │
        ▼
┌───────────────┐
│   API/CLI     │──────────────────────────────────────────┐
└───────┬───────┘                                          │
        │ Enqueue job                                      │
        ▼                                                  │
┌───────────────┐                                          │
│  NATS Queue   │                                          │
└───────┬───────┘                                          │
        │                                                  │
        ▼                                                  │
┌───────────────┐     ┌─────────────┐                     │
│  Ingestion    │────▶│ PostgreSQL  │  Store repo meta    │
│    Worker     │     │             │                     │
└───────┬───────┘     └─────────────┘                     │
        │ RawSourceBundle                                  │
        ▼                                                  │
┌───────────────┐     ┌─────────────┐                     │
│   Modeling    │────▶│ PostgreSQL  │  Store SystemModel  │
│    Worker     │     │             │                     │
└───────┬───────┘     └─────────────┘                     │
        │ SystemModel                                      │
        ▼                                                  │
┌───────────────┐                                          │
│   Planning    │                                          │
│    Worker     │                                          │
└───────┬───────┘                                          │
        │ TestPlan                                         │
        ▼                                                  │
┌───────────────┐     ┌─────────────┐                     │
│  Generation   │────▶│  LLM APIs   │  Tiered calls       │
│    Worker     │     │             │                     │
│               │◀────│  Redis      │  Cache responses    │
└───────┬───────┘     └─────────────┘                     │
        │ TestDSL[]                                        │
        ▼                                                  │
┌───────────────┐                                          │
│   Adapter     │                                          │
│    Worker     │                                          │
└───────┬───────┘                                          │
        │ TestFiles[]                                      │
        ▼                                                  │
┌───────────────┐     ┌─────────────┐                     │
│   Mutation    │────▶│  Sandboxed  │  Run tests + mutants│
│    Worker     │     │  Container  │                     │
└───────┬───────┘     └─────────────┘                     │
        │ ValidatedTests[]                                 │
        ▼                                                  │
┌───────────────┐     ┌─────────────┐                     │
│  Integration  │────▶│  GitHub     │  Create PR          │
│    Worker     │     │  API        │                     │
└───────┬───────┘     └─────────────┘                     │
        │                                                  │
        ▼                                                  │
┌───────────────┐                                          │
│   Notify      │◀─────────────────────────────────────────┘
│   User        │     Webhook / WS / Email
└───────────────┘
```

## Worker Architecture

All workers are stateless, containerized Go processes communicating via NATS.

```
┌─────────────────────────────────────────────────────────────────┐
│                      WORKER POOL                                 │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                     NATS JetStream                          ││
│  │                                                             ││
│  │  Streams:                                                   ││
│  │  • jobs.ingestion                                           ││
│  │  • jobs.modeling                                            ││
│  │  • jobs.planning                                            ││
│  │  • jobs.generation                                          ││
│  │  • jobs.mutation                                            ││
│  │  • jobs.integration                                         ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
│  Worker Scaling:                                                │
│  • Ingestion: 2-4 replicas (I/O bound)                          │
│  • Modeling: 4-8 replicas (CPU bound)                           │
│  • Generation: 4-8 replicas (LLM bound)                         │
│  • Mutation: 8-16 replicas (CPU intensive)                      │
│  • Integration: 2-4 replicas (I/O bound)                        │
│                                                                  │
│  Each worker:                                                   │
│  • Pulls from queue                                             │
│  • Processes job                                                │
│  • Writes results to DB                                         │
│  • Pushes next job to downstream queue                          │
│  • Reports metrics to Prometheus                                │
└─────────────────────────────────────────────────────────────────┘
```

## Database Schema (High-Level)

```
┌─────────────────────────────────────────────────────────────────┐
│                      POSTGRESQL SCHEMA                           │
│                                                                  │
│  repositories                                                   │
│  ├── id (uuid)                                                  │
│  ├── github_url                                                 │
│  ├── languages[]                                                │
│  ├── last_analyzed_at                                           │
│  └── created_at                                                 │
│                                                                  │
│  system_models                                                  │
│  ├── id (uuid)                                                  │
│  ├── repository_id (fk)                                         │
│  ├── model_json (jsonb)                                         │
│  ├── version                                                    │
│  └── created_at                                                 │
│                                                                  │
│  generation_runs                                                │
│  ├── id (uuid)                                                  │
│  ├── repository_id (fk)                                         │
│  ├── system_model_id (fk)                                       │
│  ├── status (pending/running/completed/failed)                  │
│  ├── tests_generated                                            │
│  ├── tests_validated                                            │
│  ├── mutation_score                                             │
│  ├── llm_tokens_used                                            │
│  └── created_at                                                 │
│                                                                  │
│  test_results                                                   │
│  ├── id (uuid)                                                  │
│  ├── generation_run_id (fk)                                     │
│  ├── test_dsl (jsonb)                                           │
│  ├── generated_code                                             │
│  ├── status (generated/validated/rejected)                      │
│  ├── rejection_reason                                           │
│  ├── mutation_kills                                             │
│  └── created_at                                                 │
│                                                                  │
│  pull_requests                                                  │
│  ├── id (uuid)                                                  │
│  ├── generation_run_id (fk)                                     │
│  ├── github_pr_number                                           │
│  ├── status (open/merged/closed)                                │
│  └── created_at                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Security Considerations

1. **Code Isolation**: All test execution runs in ephemeral containers
2. **Credential Storage**: OAuth tokens and API keys in encrypted Vault
3. **Network Isolation**: Workers cannot access customer code outside of job context
4. **Audit Logging**: All actions logged for compliance
5. **Rate Limiting**: Per-user and per-repo limits on API gateway

## Observability Stack

```
┌─────────────────────────────────────────────────────────────────┐
│                    OBSERVABILITY                                 │
│                                                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│  │ Prometheus  │  │    Loki     │  │   Jaeger    │             │
│  │  (Metrics)  │  │   (Logs)    │  │  (Traces)   │             │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘             │
│         │                │                │                     │
│         └────────────────┼────────────────┘                     │
│                          ▼                                      │
│                   ┌─────────────┐                               │
│                   │   Grafana   │                               │
│                   │ Dashboards  │                               │
│                   └─────────────┘                               │
│                                                                  │
│  Key Metrics:                                                   │
│  • Jobs processed per minute                                    │
│  • Generation success rate                                      │
│  • Mutation validation rate                                     │
│  • LLM token usage per repo                                     │
│  • Queue depth per stage                                        │
│  • P95 latency per worker type                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Deployment Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    AWS DEPLOYMENT                                │
│                                                                  │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │                       VPC                                  │ │
│  │                                                            │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐    │ │
│  │  │   EKS/ECS    │  │     RDS      │  │ ElastiCache  │    │ │
│  │  │   Workers    │  │  PostgreSQL  │  │    Redis     │    │ │
│  │  └──────────────┘  └──────────────┘  └──────────────┘    │ │
│  │                                                            │ │
│  │  ┌──────────────┐  ┌──────────────┐                      │ │
│  │  │    NATS      │  │     S3       │                      │ │
│  │  │  JetStream   │  │  (artifacts) │                      │ │
│  │  └──────────────┘  └──────────────┘                      │ │
│  │                                                            │ │
│  └───────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐                            │
│  │   CloudFront │  │     ALB      │                            │
│  │   (Static)   │  │   (API)      │                            │
│  └──────────────┘  └──────────────┘                            │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Next Steps

See [Tracker](tracker.md) for detailed implementation tasks.
