# Technology Stack

This document defines the technology choices for QTest with rationale for each decision.

## 1. Overview

| Layer | Technology | Version |
|-------|------------|---------|
| Backend | Go | 1.22+ |
| Frontend | Next.js + React | 14+ |
| Database | PostgreSQL | 16+ |
| Cache | Redis | 7+ |
| Queue | NATS JetStream | 2.10+ |
| Parsing | Tree-sitter | Latest |
| E2E | Playwright | 1.40+ |
| Container | Docker | 24+ |
| Orchestration | Kubernetes (EKS) | 1.28+ |
| CI/CD | GitHub Actions | - |

## 2. Backend: Go

### 2.1 Why Go?

**Chosen over**: Node.js/TypeScript, Python, Rust

| Factor | Go | Node.js | Python | Rust |
|--------|----|---------|---------|----|
| Concurrency | Excellent (goroutines) | Good (async) | Poor (GIL) | Excellent |
| Compilation | Fast, single binary | N/A | N/A | Slow |
| Memory | Low footprint | Medium | High | Low |
| Dev velocity | High | High | High | Medium |
| Devtools ecosystem | Strong | Strong | Medium | Medium |

**Key reasons:**
1. **Goroutines** - Perfect for parallel test generation workers
2. **Single binary** - Simple deployment, no runtime dependencies
3. **Low memory** - Run more workers per node
4. **Strong typing** - Catch errors at compile time
5. **Excellent tooling** - Tree-sitter bindings, GitHub API clients

### 2.2 Go Libraries

| Purpose | Library | Rationale |
|---------|---------|-----------|
| HTTP Router | chi | Lightweight, middleware support |
| Database | pgx | High-performance PostgreSQL driver |
| ORM | sqlc | Type-safe SQL, no reflection |
| Validation | validator | Struct tag validation |
| Config | viper | Multi-source config |
| Logging | zerolog | Structured, zero-allocation |
| Testing | testify | Assertions, mocking |
| Git | go-git | Pure Go git implementation |
| GitHub | go-github | Official GitHub API client |

### 2.3 Project Structure

```
/cmd
  /api         # API server entrypoint
  /worker      # Worker entrypoint
  /cli         # CLI entrypoint
/internal
  /api         # HTTP handlers
  /worker      # Worker implementations
  /parser      # AST parsing
  /model       # System model builder
  /generator   # Test generation
  /adapters    # Framework adapters
  /mutation    # Mutation testing
  /github      # GitHub integration
  /db          # Database layer
  /llm         # LLM client
/pkg
  /dsl         # Public DSL types
  /client      # Go client SDK
```

## 3. Parsing: Tree-sitter

### 3.1 Why Tree-sitter?

**Chosen over**: Language-specific parsers only, LSP, regex-based

**Key reasons:**
1. **Multi-language** - 40+ language grammars available
2. **Incremental** - Fast re-parsing on edits
3. **Error-tolerant** - Produces AST even with syntax errors
4. **Battle-tested** - Used by GitHub, Neovim, Zed

### 3.2 Language Support

| Language | Parser | Deep Analysis |
|----------|--------|---------------|
| TypeScript | tree-sitter-typescript | ts-morph (Node.js sidecar) |
| JavaScript | tree-sitter-javascript | babel (Node.js sidecar) |
| Python | tree-sitter-python | ast module (Python sidecar) |
| Java | tree-sitter-java | JavaParser (Java sidecar) |
| Go | tree-sitter-go | go/ast (native) |

### 3.3 Parser Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    PARSER REGISTRY                          │
│                                                             │
│  ┌─────────────────┐                                       │
│  │  File Detector  │ → Determines language per file         │
│  └────────┬────────┘                                       │
│           │                                                 │
│           ▼                                                 │
│  ┌─────────────────┐                                       │
│  │ Tree-sitter     │ → Universal AST extraction            │
│  │ (Go binding)    │                                       │
│  └────────┬────────┘                                       │
│           │                                                 │
│           ▼                                                 │
│  ┌─────────────────┐                                       │
│  │ Language Plugin │ → Deep analysis via sidecar            │
│  │ (if available)  │                                       │
│  └────────┬────────┘                                       │
│           │                                                 │
│           ▼                                                 │
│  ┌─────────────────┐                                       │
│  │ Unified AST     │ → Common representation               │
│  └─────────────────┘                                       │
└─────────────────────────────────────────────────────────────┘
```

## 4. LLM Integration

### 4.1 Local-First Architecture

QTest defaults to **local LLM inference via Ollama**, making it:
- **Cost-free** for local GPU users (RTX 4080, 4090, etc.)
- **Privacy-preserving** - code never leaves your machine
- **Fast** - no network latency to cloud APIs

Cloud APIs (Claude, OpenAI) are optional fallbacks.

### 4.2 Tiered Model Strategy

| Tier | Local (Ollama) | Cloud (Fallback) | Use Case |
|------|----------------|------------------|----------|
| Tier 1 | Qwen2.5 7B, Llama 3.1 8B | Claude Haiku, GPT-4o-mini | Boilerplate, summaries |
| Tier 2 | Qwen2.5 32B, DeepSeek Coder 33B | Claude Sonnet, GPT-4o | Test logic, assertions |
| Tier 3 | DeepSeek 70B, Llama 3.1 70B | Claude Opus, GPT-4 | Complex reasoning, critics |

**Cost Comparison:**

| Scenario | Local (Ollama) | Cloud APIs |
|----------|----------------|------------|
| 1000 tests generated | $0 (electricity only) | ~$3-15 |
| Per-month usage | $0 | $50-500 |

### 4.3 Provider Priority

1. **Primary**: Ollama (local models - Qwen, DeepSeek, Llama)
2. **Fallback 1**: Anthropic Claude (better at code understanding)
3. **Fallback 2**: OpenAI GPT-4o (broad availability)

### 4.4 Ollama Setup

```bash
# Install Ollama
curl -fsSL https://ollama.com/install.sh | sh

# Pull recommended models
ollama pull qwen2.5:7b      # Tier 1 (fast, 4GB VRAM)
ollama pull qwen2.5:32b     # Tier 2 (balanced, 20GB VRAM)
ollama pull deepseek-coder:33b  # Tier 2 alternative
ollama pull llama3.1:70b    # Tier 3 (requires 48GB+ VRAM)

# Verify
ollama list
```

### 4.5 LLM Router Service

```go
// LLMProvider represents supported backends
type LLMProvider string

const (
    ProviderOllama    LLMProvider = "ollama"    // Default, local
    ProviderAnthropic LLMProvider = "anthropic" // Cloud fallback
    ProviderOpenAI    LLMProvider = "openai"    // Cloud fallback
)

// LLMRouterConfig configures the router
type LLMRouterConfig struct {
    DefaultProvider LLMProvider
    Providers       map[LLMProvider]ProviderConfig
    TierModels      map[LLMTier]map[LLMProvider]string
}

// Default configuration (local-first)
var DefaultConfig = LLMRouterConfig{
    DefaultProvider: ProviderOllama,
    Providers: map[LLMProvider]ProviderConfig{
        ProviderOllama: {
            Enabled: true,
            BaseURL: "http://localhost:11434",
        },
        ProviderAnthropic: {
            Enabled: false, // Enable if API key provided
        },
    },
    TierModels: map[LLMTier]map[LLMProvider]string{
        LLMTier1: {
            ProviderOllama:    "qwen2.5:7b",
            ProviderAnthropic: "claude-3-haiku-20240307",
        },
        LLMTier2: {
            ProviderOllama:    "qwen2.5:32b",
            ProviderAnthropic: "claude-3-5-sonnet-20241022",
        },
        LLMTier3: {
            ProviderOllama:    "llama3.1:70b",
            ProviderAnthropic: "claude-3-opus-20240229",
        },
    },
}
```

## 5. E2E: Playwright

### 5.1 Why Playwright?

**Chosen over**: Puppeteer, Selenium, Cypress

| Factor | Playwright | Puppeteer | Selenium | Cypress |
|--------|------------|-----------|----------|---------|
| Multi-browser | Chrome, FF, Safari | Chrome only | All | Chrome, FF |
| Auto-wait | Yes | No | No | Yes |
| Network intercept | Excellent | Good | Poor | Good |
| API testing | Built-in | No | No | Yes |
| Parallelism | Native | Manual | Manual | Native |

**Key reasons:**
1. **Auto-wait** - Reduces flakiness significantly
2. **Network interception** - Critical for API inference
3. **Codegen** - Can record flows and generate code
4. **Cross-browser** - Test on all major browsers

### 5.2 Playwright Integration

Playwright runs as a **Node.js sidecar** since the Go bindings are immature.

```
┌──────────────────┐      ┌──────────────────┐
│   Go Worker      │      │  Node.js Sidecar │
│                  │      │                  │
│  ┌────────────┐  │ gRPC │  ┌────────────┐  │
│  │ Crawler    │◀─┼──────┼─▶│ Playwright │  │
│  │ Client     │  │      │  │ Server     │  │
│  └────────────┘  │      │  └────────────┘  │
│                  │      │                  │
└──────────────────┘      └──────────────────┘
```

## 6. Mutation Testing

### 6.1 Language-Specific Tools

| Language | Tool | Integration |
|----------|------|-------------|
| TypeScript/JS | Stryker | npm package, CLI |
| Python | mutmut | pip package, CLI |
| Java | PIT (Pitest) | Maven/Gradle plugin |
| Go | go-mutesting | Go tool |

### 6.2 Mutation Service

```
┌─────────────────────────────────────────────────────────────┐
│                  MUTATION SERVICE                            │
│                                                             │
│  ┌─────────────────┐                                       │
│  │ Mutation Runner │                                       │
│  │                 │                                       │
│  │ 1. Receive test + target function                       │
│  │ 2. Generate mutants (3-5 per function)                  │
│  │ 3. Run test against each mutant                         │
│  │ 4. Collect kill/survive results                         │
│  │ 5. Return MutationResult                                │
│  └─────────────────┘                                       │
│                                                             │
│  Execution: Docker container per language                   │
│  Timeout: 30s per mutant (configurable)                    │
│  Parallelism: Up to 4 mutants concurrently                 │
└─────────────────────────────────────────────────────────────┘
```

## 7. Database: PostgreSQL

### 7.1 Why PostgreSQL?

**Chosen over**: MySQL, MongoDB, CockroachDB

**Key reasons:**
1. **JSONB** - Store system models, DSL without schema changes
2. **Reliability** - Battle-tested, excellent tooling
3. **Performance** - Handles complex queries well
4. **Managed options** - AWS RDS, Neon, Supabase

### 7.2 Schema Design Principles

- Use **UUIDs** for primary keys (distributed-friendly)
- Use **JSONB** for flexible nested data (system models, DSL)
- Use **indexes** on frequently queried columns
- Use **partitioning** for large tables (test_results by date)

### 7.3 Connection Pooling

Use **PgBouncer** or built-in pgx pooling:
- Min connections: 5
- Max connections: 50
- Idle timeout: 5 minutes

## 8. Cache & Queue: Redis + NATS

### 8.1 Redis Use Cases

| Use Case | Redis Feature |
|----------|---------------|
| LLM response cache | String with TTL |
| Rate limiting | INCR with expiry |
| Session storage | Hash |
| Distributed locks | SET NX |

### 8.2 Why NATS JetStream?

**Chosen over**: Redis Streams, RabbitMQ, Kafka

| Factor | NATS | Redis Streams | RabbitMQ | Kafka |
|--------|------|---------------|----------|-------|
| Latency | <1ms | <1ms | ~5ms | ~10ms |
| Persistence | JetStream | Yes | Yes | Yes |
| Complexity | Low | Medium | High | High |
| Go client | Excellent | Good | Good | Good |

**Key reasons:**
1. **Simple** - Single binary, easy to operate
2. **Fast** - Sub-millisecond latency
3. **JetStream** - Durable queues with exactly-once delivery
4. **Go-native** - Official Go client is excellent

### 8.3 Queue Design

```
Streams:
├── JOBS.ingestion      # Repo clone jobs
├── JOBS.modeling       # System model jobs
├── JOBS.planning       # Test planning jobs
├── JOBS.generation     # Test generation jobs
├── JOBS.mutation       # Mutation testing jobs
└── JOBS.integration    # PR creation jobs

Consumer Groups:
├── ingestion-workers (3 replicas)
├── modeling-workers (5 replicas)
├── generation-workers (8 replicas)
├── mutation-workers (10 replicas)
└── integration-workers (2 replicas)
```

## 9. Frontend: Next.js

### 9.1 Why Next.js?

**Chosen over**: Create React App, Remix, Vue/Nuxt

**Key reasons:**
1. **App Router** - Server components, streaming
2. **API Routes** - Backend-for-frontend when needed
3. **Vercel** - Easy deployment for preview/staging
4. **Ecosystem** - Large community, good libraries

### 9.2 Frontend Libraries

| Purpose | Library |
|---------|---------|
| UI Components | shadcn/ui |
| Styling | Tailwind CSS |
| State | Zustand |
| Data Fetching | TanStack Query |
| Charts | Recharts |
| Forms | React Hook Form |
| Auth | NextAuth.js |

### 9.3 Key Pages

```
/                      # Landing page
/dashboard             # User dashboard
/repos                 # Repository list
/repos/[id]            # Repository detail
/repos/[id]/runs       # Run history
/repos/[id]/runs/[id]  # Run detail
/repos/[id]/tests      # Generated tests
/repos/[id]/coverage   # Coverage reports
/settings              # User settings
```

## 10. Infrastructure

### 10.1 Cloud Provider: AWS

**Chosen over**: GCP, Azure

**Key reasons:**
1. **Market share** - Most common, familiar to users
2. **EKS** - Managed Kubernetes
3. **RDS** - Managed PostgreSQL
4. **ElastiCache** - Managed Redis

### 10.2 Deployment Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         AWS VPC                              │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                    Public Subnet                     │   │
│  │  ┌─────────────┐  ┌─────────────┐                   │   │
│  │  │     ALB     │  │  CloudFront │                   │   │
│  │  │   (API)     │  │  (Static)   │                   │   │
│  │  └──────┬──────┘  └─────────────┘                   │   │
│  └─────────┼───────────────────────────────────────────┘   │
│            │                                                │
│  ┌─────────┼───────────────────────────────────────────┐   │
│  │         │         Private Subnet                     │   │
│  │         ▼                                            │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐ │   │
│  │  │    EKS      │  │     RDS     │  │ ElastiCache │ │   │
│  │  │  Cluster    │  │  PostgreSQL │  │    Redis    │ │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘ │   │
│  │                                                      │   │
│  │  ┌─────────────┐  ┌─────────────┐                   │   │
│  │  │    NATS     │  │     S3      │                   │   │
│  │  │  JetStream  │  │ (Artifacts) │                   │   │
│  │  └─────────────┘  └─────────────┘                   │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### 10.3 Kubernetes Resources

```yaml
# API Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: qtest-api
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: api
        image: qtest/api:latest
        resources:
          requests:
            cpu: 500m
            memory: 512Mi
          limits:
            cpu: 1000m
            memory: 1Gi

# Worker Deployment (per type)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: qtest-generation-worker
spec:
  replicas: 8  # Scale based on queue depth
  template:
    spec:
      containers:
      - name: worker
        image: qtest/worker:latest
        env:
        - name: WORKER_TYPE
          value: generation
        resources:
          requests:
            cpu: 1000m
            memory: 2Gi
```

### 10.4 Observability

| Component | Tool |
|-----------|------|
| Metrics | Prometheus + Grafana |
| Logs | Loki |
| Traces | Jaeger / OpenTelemetry |
| Alerts | Alertmanager |
| Uptime | Better Uptime / Pingdom |

## 11. Development Environment

### 11.1 Local Setup

```bash
# Prerequisites
- Go 1.22+
- Node.js 20+
- Docker Desktop
- direnv (optional)

# Clone and setup
git clone https://github.com/qtest/qtest
cd qtest
make setup

# Start dependencies
docker-compose up -d

# Run API
make run-api

# Run worker
make run-worker

# Run frontend
cd web && npm run dev
```

### 11.2 Docker Compose (Development)

```yaml
version: '3.8'
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: qtest
      POSTGRES_USER: qtest
      POSTGRES_PASSWORD: qtest
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data

  redis:
    image: redis:7
    ports:
      - "6379:6379"

  nats:
    image: nats:2.10
    command: ["-js"]
    ports:
      - "4222:4222"
      - "8222:8222"

volumes:
  pgdata:
```

### 11.3 CI/CD Pipeline

```yaml
# .github/workflows/ci.yml
name: CI
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: make test
      - run: make lint

  build:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: docker/build-push-action@v5
        with:
          push: ${{ github.ref == 'refs/heads/main' }}
          tags: qtest/api:${{ github.sha }}
```

## 12. Security

### 12.1 Secrets Management

| Secret Type | Storage |
|-------------|---------|
| API keys | AWS Secrets Manager |
| DB credentials | AWS Secrets Manager |
| OAuth tokens | Encrypted in PostgreSQL |
| User sessions | Redis (encrypted) |

### 12.2 Security Measures

1. **Code Isolation**: All test execution in ephemeral containers
2. **Network Isolation**: Workers in private subnet
3. **Encryption**: TLS everywhere, encryption at rest
4. **Auth**: OAuth 2.0, API keys with scopes
5. **Audit**: All actions logged with user context
6. **Scanning**: Trivy for container images, CodeQL for code

## 13. Scalability Considerations

### 13.1 Horizontal Scaling

| Component | Scaling Strategy |
|-----------|------------------|
| API | Pod autoscaler (CPU/memory) |
| Workers | Queue depth-based autoscaler |
| Database | Read replicas for queries |
| Redis | Cluster mode for high throughput |

### 13.2 Performance Targets

| Operation | Target |
|-----------|--------|
| API response (P95) | <500ms |
| Job pickup latency | <100ms |
| Test generation | <60s per test |
| Full repo analysis | <10min for 10k files |

## 14. Cost Estimates (Monthly)

### 14.1 Infrastructure (Small Scale)

| Service | Specs | Cost |
|---------|-------|------|
| EKS | 3 x t3.medium | ~$150 |
| RDS | db.t3.medium | ~$50 |
| ElastiCache | cache.t3.micro | ~$15 |
| S3 | 100GB | ~$3 |
| ALB | 1 | ~$20 |
| **Total** | | **~$240/month** |

### 14.2 LLM Costs (Per 1000 Tests Generated)

| Model | Tokens (est.) | Cost |
|-------|--------------|------|
| Tier 1 (40%) | 400k | $0.10 |
| Tier 2 (50%) | 500k | $1.50 |
| Tier 3 (10%) | 100k | $1.50 |
| **Total** | | **~$3.10/1000 tests** |
