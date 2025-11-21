# Product Requirements Document (PRD)

## 1. Executive Summary

QTest is an AI-powered platform that automatically generates and maintains comprehensive test suites for any GitHub repository or website. By combining static code analysis, runtime observation, and AI-driven test generation, QTest delivers mutation-validated tests across the entire test pyramid—unit, integration, API, and E2E.

## 2. Problem Statement

### 2.1 The Testing Gap

Software teams face a persistent challenge: **test coverage is expensive to build and even more expensive to maintain**.

**Pain Points:**

1. **Manual test writing is slow** - Engineers spend 20-40% of development time writing and maintaining tests
2. **AI-generated code lacks verification** - The rise of AI coding assistants creates code faster than teams can test it
3. **Test maintenance is neglected** - Tests become stale, flaky, or obsolete as code evolves
4. **E2E tests are brittle** - UI tests break frequently, creating distrust in CI pipelines
5. **Coverage != Quality** - High coverage doesn't mean tests catch bugs; most tests assert current behavior without meaningful validation

### 2.2 Market Gap

Existing solutions address pieces of the problem:
- **Codium/Diffblue**: Generate unit tests but don't maintain them
- **Testim/Mabl**: Focus on E2E only, no code-level testing
- **Copilot**: Suggests tests inline but no systematic coverage

**No product offers**: unified test pyramid generation + continuous maintenance + mutation-validated quality.

## 3. Vision

> QTest becomes the universal verification layer for all software—automatically understanding systems, generating meaningful tests, and maintaining them as code evolves.

## 4. Target Users

### 4.1 Primary Personas

#### Engineering Teams (5-100 engineers)
- **Pain**: Struggling to maintain test coverage during rapid development
- **Need**: Automated test generation that integrates into existing workflows
- **Value**: Reduced manual testing effort, increased confidence in releases

#### QA Engineers
- **Pain**: Writing and maintaining E2E tests is tedious and brittle
- **Need**: Self-healing tests that adapt to UI changes
- **Value**: Focus on exploratory testing instead of script maintenance

#### DevOps Teams
- **Pain**: CI pipelines fail due to flaky tests, causing deployment delays
- **Need**: Reliable test suites that run consistently
- **Value**: Predictable pipelines, faster deployments

#### AI Agent Developers
- **Pain**: AI-generated code needs verification before deployment
- **Need**: Automatic test generation for AI outputs
- **Value**: Trust in AI-generated code

### 4.2 Secondary Personas

#### Startups Building Fast
- **Need**: Bootstrap test coverage quickly without dedicated QA
- **Value**: Ship with confidence from day one

#### Legacy Codebase Owners
- **Need**: Add tests to untested code before refactoring
- **Value**: Safe modernization path

#### Open Source Maintainers
- **Need**: Ensure contributions don't break existing functionality
- **Value**: Automated verification of PRs

## 5. Use Cases

### UC-1: New Codebase Onboarding
**Actor**: Engineering Lead
**Scenario**: Team inherits a codebase with minimal tests
**Flow**:
1. Connect QTest to repository
2. QTest analyzes codebase and builds system model
3. QTest generates initial test suite (unit + API)
4. PR opened with 50+ meaningful tests
5. Team reviews and merges

**Success Metric**: 50%+ function coverage in first run

### UC-2: Pre-Release Test Generation
**Actor**: Release Manager
**Scenario**: Sprint complete, need confidence before release
**Flow**:
1. Trigger QTest run on release branch
2. QTest identifies changed code since last release
3. Generates tests for new/modified functions
4. Runs mutation testing to validate quality
5. PR with targeted tests for changes

**Success Metric**: All new code has mutation-validated tests

### UC-3: AI Code Verification
**Actor**: Developer using AI assistant
**Scenario**: AI generated a new feature module
**Flow**:
1. Developer points QTest at AI-generated code
2. QTest generates comprehensive tests
3. Tests reveal edge cases AI missed
4. Developer fixes issues before merge

**Success Metric**: Bugs caught before human review

### UC-4: E2E Flow Coverage
**Actor**: QA Engineer
**Scenario**: New feature requires E2E testing
**Flow**:
1. Point QTest at staging URL
2. QTest crawls and discovers user flows
3. Generates Playwright tests for critical paths
4. Tests added to CI pipeline

**Success Metric**: Critical flows have automated coverage

### UC-5: Continuous Test Maintenance
**Actor**: Tech Lead
**Scenario**: Ongoing development breaks existing tests
**Flow**:
1. QTest monitors repository for changes
2. On each PR, compares system model
3. Automatically updates affected tests
4. Opens PR with test maintenance

**Success Metric**: Zero stale tests after 3 months

## 6. Core Features

### 6.1 Feature: Multi-Language Repo Analysis

**Description**: Analyze repositories in TypeScript, JavaScript, Python, Java, and Go to build a comprehensive system model.

**Capabilities**:
- Detect languages and frameworks automatically
- Parse AST using Tree-sitter + native parsers
- Extract functions, classes, endpoints, dependencies
- Build unified system model across all languages

**User Value**: Works with any modern tech stack without configuration

### 6.2 Feature: Website Crawler + Flow Builder

**Description**: Crawl websites using Playwright to discover pages, capture network traffic, and identify user flows.

**Capabilities**:
- Automated page discovery
- Network request/response capture
- Login flow detection
- Multi-step flow recording
- API inference from XHR calls

**User Value**: E2E tests without manual recording

### 6.3 Feature: AI Test Generation

**Description**: Use tiered LLM strategy to generate tests from system model.

**Capabilities**:
- Generate unit tests for functions
- Generate integration tests for services
- Generate API tests for endpoints
- Generate E2E tests for flows
- Adaptive test case selection (happy path, edge cases, errors)

**User Value**: Meaningful tests, not just coverage padding

### 6.4 Feature: Mutation-Based Quality Gate

**Description**: Validate generated tests actually catch bugs using mutation testing.

**Capabilities**:
- Generate code mutations (flip operators, remove branches)
- Run tests against mutants
- Reject tests that don't kill mutants
- Strengthen weak tests automatically

**User Value**: Every shipped test is proven to catch real bugs

### 6.5 Feature: Continuous Maintenance

**Description**: Keep test suite fresh as code evolves.

**Capabilities**:
- Detect code changes via system model diff
- Update tests for modified code
- Remove tests for deleted code
- Generate tests for new code
- Track and fix flaky tests

**User Value**: Tests stay relevant without manual effort

### 6.6 Feature: GitHub Native Integration

**Description**: Seamless integration into GitHub workflow.

**Capabilities**:
- GitHub App installation
- Automatic PR creation
- PR comment summaries
- CI workflow generation
- Webhook-based triggers

**User Value**: Works where developers already work

### 6.7 Feature: Coverage & Quality Dashboard

**Description**: Visualize test health and trends.

**Capabilities**:
- Coverage trends (line, branch, function)
- Mutation score tracking
- Flakiness metrics
- LLM usage and cost tracking
- Per-module drill-down

**User Value**: Data-driven testing decisions

## 7. Feature Prioritization

### Phase 1: MVP (Must Have)
| Feature | Priority | Rationale |
|---------|----------|-----------|
| JS/TS Repo Analysis | P0 | Most common stack |
| Unit Test Generation | P0 | Foundation of pyramid |
| API Test Generation | P0 | High-value, low-effort |
| Jest Adapter | P0 | Default JS test framework |
| GitHub PR Integration | P0 | Core delivery mechanism |
| Basic CI Generation | P0 | Enables immediate value |

### Phase 2: E2E + Website (Should Have)
| Feature | Priority | Rationale |
|---------|----------|-----------|
| Playwright Crawler | P1 | Enables E2E |
| E2E Test Generation | P1 | Complete pyramid |
| Flow Detection | P1 | Automated discovery |
| Network → API Inference | P1 | Runtime analysis |

### Phase 3: Quality + Maintenance (Should Have)
| Feature | Priority | Rationale |
|---------|----------|-----------|
| Mutation Testing | P1 | Quality validation |
| Drift Detection | P1 | Maintenance automation |
| Test Updates | P1 | Continuous value |
| Flakiness Detection | P1 | Reliability |

### Phase 4: Scale + Enterprise (Nice to Have)
| Feature | Priority | Rationale |
|---------|----------|-----------|
| Python Support | P2 | Expand market |
| Java Support | P2 | Enterprise market |
| Team Dashboard | P2 | Team collaboration |
| SSO/RBAC | P2 | Enterprise requirement |

## 8. Success Metrics

### 8.1 Product Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Tests Generated per Run | 20+ meaningful tests | Avg per generation run |
| Coverage Increase | +30% on first run | Before/after comparison |
| Mutation Score | >0.70 for validated tests | Mutants killed / total |
| Test Acceptance Rate | >80% merged | PRs merged / PRs created |
| Flaky Test Rate | <5% | Flaky / total tests |

### 8.2 Business Metrics

| Metric | Target | Timeframe |
|--------|--------|-----------|
| Repos Connected | 1,000 | 6 months |
| Monthly Active Repos | 500 | 6 months |
| Paid Conversions | 100 teams | 6 months |
| Net Promoter Score | >40 | Ongoing |
| Monthly Recurring Revenue | $50k | 12 months |

### 8.3 Engagement Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| D7 Retention | >50% | Users active 7 days after signup |
| Weekly Active Users | 60% of signups | WAU / Total users |
| Runs per Repo per Week | 2+ | Avg across active repos |
| Time to First PR | <10 minutes | From signup to first generated PR |

## 9. User Experience

### 9.1 Onboarding Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                      ONBOARDING FLOW                             │
│                                                                  │
│  1. Sign Up                                                      │
│     └── GitHub OAuth                                             │
│                                                                  │
│  2. Connect Repository                                           │
│     └── Select from list OR paste URL                            │
│                                                                  │
│  3. Initial Analysis (background)                                │
│     └── Show progress: Cloning → Analyzing → Planning            │
│                                                                  │
│  4. Review Test Plan                                             │
│     └── Show what will be generated                              │
│     └── Allow customization (exclude files, change frameworks)   │
│                                                                  │
│  5. Generate Tests                                               │
│     └── Real-time progress                                       │
│     └── Show tests as they're generated                          │
│                                                                  │
│  6. PR Created                                                   │
│     └── Link to GitHub PR                                        │
│     └── Summary of results                                       │
└─────────────────────────────────────────────────────────────────┘
```

### 9.2 Core Workflows

**Workflow A: First Run**
1. Connect repo via GitHub App
2. Review auto-detected settings
3. Click "Generate Tests"
4. View real-time generation progress
5. Receive PR with tests

**Workflow B: Continuous Maintenance**
1. Enable "Auto-maintain" for repo
2. System triggers on each push
3. Drift detected, tests updated
4. PR opened automatically
5. Developer reviews and merges

**Workflow C: Manual Trigger**
1. Navigate to repo in dashboard
2. Click "Run Analysis"
3. Optionally specify branch/commit
4. View generation progress
5. Download or PR delivery

### 9.3 CLI Experience

```bash
# Install
npm install -g qtest-cli

# Authenticate
qtest auth login

# Generate tests for current directory
qtest generate .

# Generate tests for GitHub repo
qtest generate https://github.com/user/repo

# Generate E2E tests for website
qtest generate --e2e https://example.com

# Check status
qtest status

# View reports
qtest report --coverage
qtest report --mutations
```

## 10. Competitive Analysis

| Feature | QTest | Codium | Diffblue | Testim | Mabl |
|---------|-------|--------|----------|--------|------|
| Unit Tests | ✓ | ✓ | ✓ | ✗ | ✗ |
| API Tests | ✓ | Partial | ✗ | ✗ | ✗ |
| E2E Tests | ✓ | ✗ | ✗ | ✓ | ✓ |
| Multi-language | ✓ | ✓ | Java only | N/A | N/A |
| Mutation Testing | ✓ | ✗ | ✓ | ✗ | ✗ |
| Auto-maintenance | ✓ | ✗ | ✗ | Partial | Partial |
| GitHub Integration | ✓ | ✓ | ✗ | ✗ | ✗ |
| Website Crawling | ✓ | ✗ | ✗ | ✓ | ✓ |

**QTest Differentiators**:
1. **Full pyramid** - Only solution covering unit through E2E
2. **Mutation validation** - Only solution proving test quality
3. **Continuous maintenance** - Auto-updates, not just initial generation
4. **Unified model** - Same system model powers all test types

## 11. Monetization

### 11.1 Pricing Tiers

| Tier | Price | Includes |
|------|-------|----------|
| **Free** | $0 | 1 public repo, 100 tests/month |
| **Pro** | $49/month | 5 repos, 1,000 tests/month, private repos |
| **Team** | $199/month | 20 repos, 5,000 tests/month, team dashboard |
| **Enterprise** | Custom | Unlimited, SSO, SLA, dedicated support |

### 11.2 Usage-Based Add-ons

| Add-on | Price |
|--------|-------|
| Additional repos | $10/repo/month |
| Additional tests | $0.05/test |
| E2E test generation | $0.10/test |
| Priority generation | 2x base cost |

### 11.3 Revenue Projections

| Month | Free Users | Paid Users | MRR |
|-------|------------|------------|-----|
| 3 | 500 | 20 | $2k |
| 6 | 2,000 | 100 | $10k |
| 12 | 10,000 | 500 | $50k |

## 12. Risks & Mitigations

### Risk 1: LLM Quality Variability
**Risk**: Generated tests may be low quality or incorrect
**Mitigation**: Mutation testing as quality gate; only ship proven tests
**Residual Risk**: Low (mutation testing is definitive)

### Risk 2: Multi-Language Complexity
**Risk**: Supporting multiple languages increases scope
**Mitigation**: Unified DSL reduces per-language work; prioritize JS/TS first
**Residual Risk**: Medium (some languages harder than others)

### Risk 3: E2E Test Flakiness
**Risk**: Generated E2E tests may be unreliable
**Mitigation**: Self-healing selectors; flakiness tracking; quarantine
**Residual Risk**: Medium (E2E flakiness is industry-wide problem)

### Risk 4: Market Competition
**Risk**: Well-funded competitors may catch up
**Mitigation**: Full pyramid + maintenance is hard to replicate; move fast
**Residual Risk**: Medium

### Risk 5: LLM Cost Scaling
**Risk**: LLM costs may make large repos unprofitable
**Mitigation**: Tiered model strategy; caching; budget caps
**Residual Risk**: Low (costs are predictable and controllable)

## 13. Go-To-Market Strategy

### 13.1 Launch Phases

**Phase 1: Private Beta (Weeks 1-4)**
- Invite 50 design partners
- Focus on JS/TS repos
- Collect feedback, iterate

**Phase 2: Public Beta (Weeks 5-8)**
- Open signups with waitlist
- Twitter/LinkedIn launch
- Product Hunt launch

**Phase 3: General Availability (Week 9+)**
- Remove waitlist
- Enable paid tiers
- GitHub Marketplace listing

### 13.2 Marketing Channels

| Channel | Strategy |
|---------|----------|
| Twitter/X | Thread launches, demo videos |
| LinkedIn | Engineering leadership content |
| Product Hunt | Coordinated launch day |
| GitHub Marketplace | App listing |
| Dev.to/Hashnode | Technical deep-dives |
| Hacker News | Show HN post |

### 13.3 Content Strategy

- "How We Built QTest" technical blog series
- Video demos showing real repos
- Open source examples with generated tests
- Comparison posts vs alternatives

## 14. Timeline

| Phase | Duration | Deliverables |
|-------|----------|--------------|
| Phase 1: MVP | 8-12 weeks | JS/TS analysis, unit+API tests, GitHub PR |
| Phase 2: E2E | 6-8 weeks | Playwright crawler, E2E generation |
| Phase 3: Quality | 6-8 weeks | Mutation testing, drift detection, maintenance |
| Phase 4: Scale | 8-12 weeks | Python/Java, teams, enterprise |

**Total to Full Product**: 28-40 weeks (~7-10 months)

## 15. Open Questions

1. **Pricing sensitivity**: Will developers pay $49/month? Need validation.
2. **E2E scope**: How much flow complexity to support in v1?
3. **Self-host demand**: Enterprise may want on-prem; prioritize?
4. **IDE integration**: VS Code extension valuable? When to build?

## 16. Appendix

### A. Glossary

See [FRD Glossary](frd.md#glossary)

### B. Related Documents

- [Architecture](architecture.md)
- [FRD](frd.md)
- [Test DSL Spec](test-dsl-spec.md)
- [Tracker](tracker.md)
