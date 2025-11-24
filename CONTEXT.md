# QTest Development Context Document

**Date:** 2025-11-24
**Status:** 76% Complete (157/206 tasks)
**Phase:** Post-MVP, Pre-Enterprise
**Ready For:** CLI Beta Launch

---

## Executive Summary

QTest is a **production-ready AI-powered test generation platform** with a comprehensive backend, complete E2E testing system, and functional web dashboard. The platform successfully generates unit, API, and E2E tests across 5 programming languages and 6 web frameworks.

**What Works:**
- ✅ CLI tool (15+ commands, 66 tests)
- ✅ API server (50+ endpoints, 130+ tests)
- ✅ 10 operational workers (NATS JetStream)
- ✅ Test generation (5 languages, 6 frameworks)
- ✅ Mutation testing (4 tools integrated)
- ✅ E2E testing (100% complete, 137 tests)
- ✅ GitHub integration (OAuth + App + PR creation)
- ✅ Coverage tracking (3 languages)
- ✅ Web dashboard (12 pages, needs polish)

**What's Missing:**
- ⚠️ Frontend polish (40% complete, needs 2-4 weeks)
- ⚠️ Observability (no Prometheus/Grafana)
- ⚠️ IaC (Docker Compose exists, no K8s/Helm)
- 🔴 Enterprise features (SSO, audit logging, 9% complete)

---

## Quick Start

### Prerequisites
- Go 1.21+
- Node.js 18+
- PostgreSQL 14+
- Redis 7+
- NATS 2.10+
- Ollama (or Claude/OpenAI API key)

### Local Development

```bash
# 1. Start dependencies
docker-compose up -d postgres redis nats

# 2. Run migrations
psql $DATABASE_URL < migrations/*.sql

# 3. Start Ollama (if using local LLM)
ollama serve &
ollama pull qwen2.5-coder:7b

# 4. Start API server
go run ./cmd/api

# 5. Start worker pool
go run ./cmd/worker

# 6. Start frontend (optional)
cd web && npm install && npm run dev

# 7. Use CLI
go build -o ./bin/qtest ./cmd/cli
./bin/qtest --help
```

### Test the System

```bash
# Generate tests for a single file
./bin/qtest generate-file -f examples/math.go -t 1 -m 5 --write

# Initialize a workspace
./bin/qtest workspace init https://github.com/your/repo

# Run coverage-guided generation
./bin/qtest coverage run --workspace myrepo

# Check system status
./bin/qtest status
```

---

## Architecture

### High-Level Flow

```
User (CLI/API/Web)
        ↓
    API Server (Chi, 50+ endpoints)
        ↓
    Job Queue (NATS JetStream)
        ↓
  Worker Pool (10 worker types)
   ↙    ↓    ↓    ↓     ↘
Parser LLM  Validator  Mutation  GitHub
  ↓     ↓     ↓        ↓         ↓
Database ← ← ← ← ← ← ← ← ← ←
```

### Component Map

```
QTest/
├── cmd/
│   ├── api/          # REST API server
│   ├── worker/       # Background worker pool
│   └── cli/          # CLI tool (15+ commands)
│
├── internal/
│   ├── adapters/     # DSL → Test code (Go/Jest/Pytest/JUnit)
│   ├── api/          # API handlers (28 files)
│   ├── auth/         # GitHub OAuth + sessions
│   ├── ci/           # CI workflow generator
│   ├── codecov/      # Coverage collection & analysis
│   ├── contract/     # Contract testing
│   ├── crawler/      # Website crawler (Playwright)
│   ├── datagen/      # Test data generator
│   ├── db/           # Database layer (23 tables)
│   ├── depgraph/     # Dependency graph builder
│   ├── differ/       # Code diff & drift detection
│   ├── e2e/          # E2E test generation (17 files, 137 tests)
│   ├── emitter/      # API/E2E test emitters (7 frameworks)
│   ├── flakiness/    # Flakiness tracking
│   ├── flow/         # User flow detection
│   ├── generator/    # LLM test generation (119 tests)
│   ├── github/       # GitHub API integration
│   ├── jobs/         # Job queue system
│   ├── llm/          # LLM router (175 tests)
│   ├── maintenance/  # Continuous maintenance (77 tests)
│   ├── mutation/     # Mutation testing (62 tests)
│   ├── parser/       # Tree-sitter parser (5 languages)
│   ├── planner/      # Test planner (14 tests)
│   ├── sidecar/      # Playwright gRPC sidecar
│   ├── strengthening/# Test strengthening (22 tests)
│   ├── supplements/  # Framework detection (6 frameworks)
│   ├── validator/    # Test validation (21 tests)
│   ├── webhook/      # Webhook system
│   └── worker/       # 10 worker implementations
│
├── web/
│   └── src/
│       ├── app/      # 12 Next.js pages
│       ├── components/
│       ├── lib/      # API client (548 lines)
│       └── __tests__/# 2 test files (19 tests)
│
├── sidecar/
│   └── playwright/   # TypeScript gRPC service
│
├── migrations/       # 9 database migrations
└── docs/             # 8 documentation files
```

---

## Current State

### Completed Features

**Phase 1: MVP (92% - 90/98 tasks)**
- ✅ All P0 tasks complete
- ✅ Parser (5 languages: Go, Python, JS, TS, Java)
- ✅ Framework detection (6: Express, FastAPI, Gin, Spring Boot, Django, NestJS)
- ✅ Test generation (unit + API + E2E)
- ✅ LLM integration (Ollama + Claude + OpenAI)
- ✅ Mutation testing (Stryker + go-mutesting + mutmut + PIT)
- ✅ GitHub integration (OAuth + App + PR creation)
- ✅ Worker system (10 workers operational)
- ✅ CLI tool (15+ commands)
- ✅ API server (50+ endpoints)
- ✅ Database (23 tables, 9 migrations)

**Phase 2: E2E Testing (100% - 38/38 tasks)**
- ✅ Playwright sidecar (gRPC, 20+ methods)
- ✅ Website crawler (depth/page limits, robots.txt)
- ✅ Flow detection (login, forms, user flows)
- ✅ Network parser (API endpoint inference)
- ✅ Endpoint merger (traffic + code)
- ✅ DSL generator (flow → DSL → test code)
- ✅ Playwright/Cypress emitters
- ✅ E2E validation (multi-run flakiness detection)
- ✅ Screenshot comparison (visual regression)

**Phase 3: Quality (74% - 26/35 tasks)**
- ✅ Coverage tracking (Go, Python, Jest)
- ✅ Mutation testing (4 tools)
- ✅ Test strengthening (LLM-based)
- ✅ Drift detection (code diff → test impact)
- ✅ Maintenance scheduler
- ✅ Flakiness tracker
- ✅ Contract testing
- ✅ Test data generation
- ✅ Test validation
- ⚠️ Quarantine feature (implemented, needs auto-quarantine)
- ⚠️ Flakiness fix suggestions (needs LLM integration)

**Phase 4: Enterprise (9% - 3/35 tasks)**
- ✅ Organizations (DB + API)
- ✅ RBAC (owner/admin/member/viewer)
- ✅ Multi-tenancy (organization-scoped resources)
- ⚠️ Team dashboard (backend ready, frontend 80% done)
- 🔴 SSO (SAML/OIDC) - not started
- 🔴 Audit logging - DB ready, API not started
- 🔴 Data export (GDPR) - not started
- 🔴 Self-hosted deployment - Docker Compose exists, no K8s/Helm

### Test Coverage

**Backend (Go):**
- Test files: 120+
- Test functions: 2,276
- Test lines: 56,438
- Coverage: ~90%

**Frontend (TypeScript):**
- Test files: 2
- Test functions: 19
- Coverage: ~10%

**Total Tests by Module:**
- LLM: 175 tests
- Generator: 119 tests
- E2E: 137 tests
- API: 130 tests
- GitHub: 110+ tests
- Mutation: 62 tests
- Maintenance: 77 tests
- Parser: 80+ tests
- Adapters: 60+ tests
- CLI: 66 tests

---

## Data Flow

### Code → Test Generation

```
1. Repository Ingestion (IngestionWorker)
   - Clone repo (git depth=1)
   - Detect language (file extensions + content)
   - Create Repository record
   ↓

2. System Modeling (ModelingWorker)
   - Parse all files (tree-sitter)
   - Extract functions, classes, branches, calls
   - Build dependency graph
   - Detect framework
   - Extract API endpoints
   - Calculate risk scores
   - Persist SystemModel (JSON)
   ↓

3. Test Planning (PlanningWorker)
   - Classify targets (unit/API/E2E)
   - Rank by priority (complexity, risk)
   - Distribute test pyramid (70/20/10)
   - Estimate tokens & cost
   - Create TestPlan
   ↓

4. Test Generation (GenerationWorker)
   - Build context (code, branches, dependencies)
   - Route to LLM (tier 1/2/3)
   - Parse YAML → DSL
   - Convert DSL → framework code
   - Write test file
   ↓

5. Validation (ValidationWorker)
   - Check compilation (tsc/mypy/javac/go build)
   - Run tests in Docker sandbox
   - Parse test output
   - Retry on failure
   ↓

6. Mutation Testing (MutationWorker) - Optional
   - Generate mutants
   - Run tests against mutants
   - Calculate mutation score
   - If score < threshold → strengthen
   ↓

7. Integration (IntegrationWorker)
   - Create GitHub branch
   - Commit tests
   - Generate PR body (summary, coverage, mutations)
   - Create pull request
```

### Website → E2E Test Generation

```
1. E2E Discovery (E2EDiscoveryWorker)
   - Crawl pages (Playwright)
   - Capture network traffic
   - Take DOM snapshots
   - Detect flows (login, checkout)
   ↓

2. E2E Generation (E2EGenerateWorker)
   - Parse network logs → API endpoints
   - Infer request/response schemas
   - Merge with code endpoints
   - Convert flows → DSL
   - Emit Playwright/Cypress tests
   ↓

3. E2E Validation (E2ERunWorker)
   - Run tests multiple times (flakiness)
   - Take screenshots
   - Compare screenshots (pixel diff)
   - Generate stability report
```

---

## API Endpoints

**Health:**
- `GET /health` - Health check
- `GET /ready` - Readiness check

**Auth:**
- `GET /auth/login` - GitHub OAuth initiation
- `GET /auth/callback` - OAuth callback
- `POST /auth/logout` - Logout
- `GET /api/v1/auth/me` - Current user
- `POST /api/v1/auth/refresh` - Refresh session
- `GET /api/v1/auth/repos` - User's GitHub repos

**Repositories:**
- `GET /api/v1/repositories` - List repos
- `POST /api/v1/repositories` - Create repo
- `GET /api/v1/repositories/:id` - Get repo
- `PATCH /api/v1/repositories/:id` - Update repo
- `DELETE /api/v1/repositories/:id` - Delete repo

**Jobs:**
- `GET /api/v1/jobs` - List jobs
- `POST /api/v1/jobs` - Create job
- `GET /api/v1/jobs/:id` - Get job
- `POST /api/v1/jobs/:id/cancel` - Cancel job
- `POST /api/v1/jobs/:id/retry` - Retry job
- `POST /api/v1/jobs/pipeline` - Start full pipeline

**Tests:**
- `GET /api/v1/tests` - List tests
- `GET /api/v1/tests/:id` - Get test
- `PUT /api/v1/tests/:id/accept` - Accept test
- `PUT /api/v1/tests/:id/reject` - Reject test

**Coverage:**
- `POST /api/v1/coverage/snapshots` - Create snapshot
- `GET /api/v1/coverage/summary` - Get summary
- `GET /api/v1/coverage/snapshots` - List snapshots
- `GET /api/v1/coverage/repos/:id/trend` - Get trend

**Mutation:**
- `GET /api/v1/mutation` - List mutation runs
- `POST /api/v1/mutation` - Create mutation run
- `GET /api/v1/mutation/:id` - Get mutation run

**Organizations:**
- `GET /api/v1/organizations` - List orgs
- `POST /api/v1/organizations` - Create org
- `GET /api/v1/organizations/:id` - Get org
- `PATCH /api/v1/organizations/:id` - Update org
- `DELETE /api/v1/organizations/:id` - Delete org
- `GET /api/v1/organizations/:id/members` - List members
- `POST /api/v1/organizations/:id/members` - Add member
- `PATCH /api/v1/organizations/:id/members/:uid` - Update member
- `DELETE /api/v1/organizations/:id/members/:uid` - Remove member

**API Keys:**
- `GET /api/v1/api-keys` - List API keys
- `POST /api/v1/api-keys` - Create API key
- `DELETE /api/v1/api-keys/:id` - Revoke API key

**Webhooks:**
- `GET /api/v1/webhooks` - List webhooks
- `POST /api/v1/webhooks` - Create webhook
- `GET /api/v1/webhooks/:id` - Get webhook
- `PATCH /api/v1/webhooks/:id` - Update webhook
- `DELETE /api/v1/webhooks/:id` - Delete webhook
- `GET /api/v1/webhooks/:id/deliveries` - List deliveries

**Admin:**
- `GET /api/v1/admin/users` - List users
- `GET /api/v1/admin/stats` - System stats
- `GET /api/v1/admin/health` - Detailed health

**Documentation:**
- `GET /docs` - Swagger UI
- `GET /docs/redoc` - ReDoc
- `GET /openapi.yaml` - OpenAPI spec

---

## Language & Framework Support

**Languages (5):**
| Language | Parser | Unit Tests | API Tests | Mutation | Status |
|----------|--------|------------|-----------|----------|--------|
| Go | ✅ | ✅ (go test) | ✅ (net/http) | ✅ (go-mutesting) | Complete |
| Python | ✅ | ✅ (pytest) | ✅ (pytest + requests) | ✅ (mutmut) | Complete |
| JavaScript | ✅ | ✅ (Jest) | ✅ (Supertest) | ✅ (Stryker) | Complete |
| TypeScript | ✅ | ✅ (Jest) | ✅ (Supertest) | ✅ (Stryker) | Complete |
| Java | ✅ | ✅ (JUnit 5) | ✅ (MockMvc) | ✅ (PIT) | Complete |

**Frameworks (6):**
| Framework | Language | Routes | Params | Body | Middleware | Status |
|-----------|----------|--------|--------|------|------------|--------|
| Express | Node.js | ✅ | ✅ | ✅ | ✅ | Complete |
| FastAPI | Python | ✅ | ✅ | ✅ | ✅ | Complete |
| Gin | Go | ✅ | ✅ | ✅ | ✅ | Complete |
| Spring Boot | Java | ✅ | ✅ | ✅ | ✅ | Complete |
| Django REST | Python | ✅ | ✅ | ✅ | ✅ | Complete |
| NestJS | TypeScript | ✅ | ✅ | ✅ | ✅ | Complete |

**E2E Frameworks (2):**
- Playwright (TypeScript)
- Cypress (JavaScript)

---

## Configuration

### Environment Variables

**Database:**
```bash
DATABASE_URL=postgresql://user:pass@localhost:5432/qtest
```

**Redis:**
```bash
REDIS_URL=redis://localhost:6379
```

**NATS:**
```bash
NATS_URL=nats://localhost:4222
```

**GitHub OAuth:**
```bash
GITHUB_CLIENT_ID=your_client_id
GITHUB_CLIENT_SECRET=your_client_secret
GITHUB_CALLBACK_URL=http://localhost:8080/auth/callback
```

**GitHub App:**
```bash
GITHUB_APP_ID=your_app_id
GITHUB_APP_PRIVATE_KEY_PATH=/path/to/private-key.pem
```

**LLM (choose one):**
```bash
# Ollama (local)
OLLAMA_URL=http://localhost:11434

# OR Claude
ANTHROPIC_API_KEY=sk-ant-...

# OR OpenAI
OPENAI_API_KEY=sk-...
```

**Server:**
```bash
PORT=8080
HOST=0.0.0.0
```

---

## Known Limitations

**Current Limitations:**
1. **No observability** - No Prometheus metrics, Grafana dashboards, or traces
2. **Limited frontend tests** - Only 19 tests, needs 80%+ coverage
3. **No IaC** - Docker Compose exists but no Kubernetes/Helm
4. **Sequential LLM requests** - Could be parallelized for faster generation
5. **Polling for job status** - Should use WebSockets for real-time updates
6. **No self-healing** - Cannot auto-fix flaky tests or adjust selectors

**Scalability Concerns:**
- Database N+1 queries in job listing (needs optimization)
- No connection pool tuning for high load
- No API response caching
- No CDN for frontend assets

**Security Gaps:**
- No IP-based rate limiting
- No DDoS protection
- Audit logging schema exists but no API
- No automated vulnerability scanning

---

## Next Steps

### Before Launch (1-2 weeks)

**Priority 1: Observability**
- Add Prometheus exporters to API and workers
- Create Grafana dashboards (API metrics, worker metrics, queue depth)
- Set up alerts (error rate, latency, queue backup)
- Add correlation IDs to logs

**Priority 2: Frontend Polish**
- Add comprehensive tests (target 80% coverage)
- Implement skeleton loaders
- Add toast notifications
- Standardize loading/error states
- Add WebSocket for real-time job updates

**Priority 3: Deployment**
- Create Kubernetes manifests (API, worker, DB, Redis, NATS)
- Write Helm chart with configurable values
- Add Terraform for AWS/GCP infrastructure
- Document deployment process
- Test full stack deployment

### Post-Launch (1-3 months)

**Phase 1: Enterprise Features (8-12 weeks)**
- Implement SSO (SAML/OIDC)
- Build audit logging API + UI
- Add data export (GDPR compliance)
- Create admin dashboard
- Add usage analytics UI

**Phase 2: Performance (2-4 weeks)**
- Optimize database queries
- Implement query caching
- Add API response caching
- Parallel test execution
- Batch LLM requests

**Phase 3: Documentation (1-2 weeks)**
- User guides (getting started, CLI, Web UI)
- API documentation (beyond OpenAPI)
- Deployment guides (Docker, K8s, AWS)
- Architecture deep-dive
- Troubleshooting guide

---

## Development Workflow

### Adding a New Language

1. Add tree-sitter grammar to `internal/parser/parser.go`
2. Implement parser extraction (functions, classes, etc.)
3. Create unit test adapter in `internal/adapters/`
4. Create API test emitter in `internal/emitter/`
5. Add mutation testing tool integration in `internal/mutation/`
6. Write tests (parser + adapter + emitter + mutation)
7. Update documentation

### Adding a New Framework

1. Create supplement in `internal/supplements/`
2. Implement route detection
3. Extract path parameters
4. Infer request/response schemas
5. Detect middleware
6. Write tests (analyze + detect + middleware)
7. Update documentation

### Adding a New Worker

1. Define job type in `internal/jobs/types.go`
2. Implement worker in `internal/worker/workers.go`
3. Add job payload struct
4. Implement job processing logic
5. Add retry logic
6. Write tests (success, failure, retry)
7. Update worker pool configuration

---

## Testing Strategy

### Unit Tests
- Test all business logic in isolation
- Mock external dependencies (DB, LLM, GitHub)
- Use table-driven tests for edge cases
- Target 90%+ coverage

### Integration Tests
- Test full worker flows (ingestion → generation → integration)
- Use test database (Docker)
- Use real NATS (Docker)
- Test error handling and retries

### E2E Tests
- Test full CLI workflows
- Test full API workflows
- Test frontend (Jest + React Testing Library)
- Test real LLM integration (staging only)

### Performance Tests
- Load test API (k6 or wrk)
- Load test workers (NATS load generator)
- Database query benchmarks
- LLM throughput tests

---

## Troubleshooting

### Worker Not Processing Jobs

```bash
# Check NATS connection
./bin/qtest status

# Check worker logs
docker logs qtest-worker

# Check job queue depth
# (needs admin API - not implemented)
```

### LLM Requests Failing

```bash
# Check Ollama connection
curl http://localhost:11434/api/tags

# Check Claude API key
curl -H "x-api-key: $ANTHROPIC_API_KEY" https://api.anthropic.com/v1/messages

# Check OpenAI API key
curl -H "Authorization: Bearer $OPENAI_API_KEY" https://api.openai.com/v1/models
```

### Database Connection Issues

```bash
# Check PostgreSQL
psql $DATABASE_URL -c "SELECT 1"

# Check migrations
psql $DATABASE_URL -c "SELECT * FROM schema_migrations"

# Run migrations
psql $DATABASE_URL < migrations/*.sql
```

### GitHub OAuth Not Working

```bash
# Check callback URL
echo $GITHUB_CALLBACK_URL

# Check client credentials
# (should be set in GitHub App settings)

# Test OAuth flow manually
open "https://github.com/login/oauth/authorize?client_id=$GITHUB_CLIENT_ID&scope=repo"
```

---

## File References

**Parser & Modeling:**
- Parser: `internal/parser/parser.go`
- System Model: `pkg/model/builder.go`
- Dependency Graph: `internal/depgraph/`

**Test Generation:**
- Generator: `internal/generator/generator.go`
- Adapters: `internal/adapters/*.go`
- Emitters: `internal/emitter/*.go`

**Workers:**
- All workers: `internal/worker/workers.go`
- Job types: `internal/jobs/types.go`

**API:**
- Server: `internal/api/server.go`
- Routes: `internal/api/*.go`

**Database:**
- Schema: `migrations/*.sql`
- Queries: `internal/db/*.go`

**Documentation:**
- Architecture: `docs/architecture.md`
- Tracker: `docs/tracker.md`
- Review: `docs/REVIEW.md` (comprehensive analysis)
- PRD: `docs/prd.md`

---

**Last Updated:** 2025-11-24
**Status:** Production-ready backend, functional frontend, needs observability and IaC
**Next Milestone:** Beta launch (CLI-first) with observability stack
**ETA:** 1-2 weeks for beta, 1 month for public web launch

For the latest progress, see `docs/tracker.md`.
For comprehensive analysis, see `docs/REVIEW.md`.
