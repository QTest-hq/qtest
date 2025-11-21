# LLM Cost Management

## 1. Overview

LLM costs can quickly become the dominant expense in AI-powered applications. This document defines strategies for controlling costs while maintaining quality.

### 1.1 Cost Challenge

```
Naive approach (all Opus/GPT-4):
  - 1 test generation = ~2000 tokens input + 500 output
  - 100 tests = 250,000 tokens
  - Cost: ~$7.50 per repo per run

With tiered approach:
  - Same 100 tests
  - Cost: ~$0.75-1.50 per repo per run
  - 5-10x cost reduction
```

### 1.2 Pricing Reference (as of Jan 2025)

| Provider | Model | Input $/1M | Output $/1M | Tier |
|----------|-------|------------|-------------|------|
| Anthropic | Claude Haiku | $0.25 | $1.25 | 1 |
| Anthropic | Claude Sonnet | $3.00 | $15.00 | 2 |
| Anthropic | Claude Opus | $15.00 | $75.00 | 3 |
| OpenAI | GPT-4o-mini | $0.15 | $0.60 | 1 |
| OpenAI | GPT-4o | $2.50 | $10.00 | 2 |
| OpenAI | GPT-4 | $30.00 | $60.00 | 3 |
| Groq | Llama 3.1 70B | $0.59 | $0.79 | 1 |

## 2. Tiered Model Strategy

### 2.1 Tier Definitions

```
┌─────────────────────────────────────────────────────────────────┐
│                    MODEL TIERS                                   │
│                                                                  │
│  TIER 1: Fast & Cheap                                           │
│  ├── Models: Claude Haiku, GPT-4o-mini, Llama 3.1 8B            │
│  ├── Cost: $0.15-0.25/1M input tokens                           │
│  ├── Latency: <1s                                               │
│  └── Use for: Boilerplate, summaries, simple transformations    │
│                                                                  │
│  TIER 2: Balanced                                               │
│  ├── Models: Claude Sonnet, GPT-4o, Llama 3.1 70B              │
│  ├── Cost: $2.50-3.00/1M input tokens                           │
│  ├── Latency: 1-3s                                              │
│  └── Use for: Test logic, assertions, moderate reasoning        │
│                                                                  │
│  TIER 3: Premium                                                │
│  ├── Models: Claude Opus, GPT-4                                 │
│  ├── Cost: $15-30/1M input tokens                               │
│  ├── Latency: 3-10s                                             │
│  └── Use for: Complex reasoning, critics, edge cases            │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 Task-to-Tier Mapping

| Task | Tier | Rationale |
|------|------|-----------|
| Code summarization | 1 | Pattern recognition, no reasoning |
| Function signature extraction | 1 | Structured extraction |
| Test case enumeration | 1 | List generation |
| Test DSL boilerplate | 1 | Template filling |
| Test input generation | 2 | Requires domain understanding |
| Assertion generation | 2 | Requires behavior analysis |
| Edge case identification | 2 | Moderate reasoning |
| Complex business logic | 3 | Deep reasoning required |
| Critic/review pass | 3 | Meta-reasoning |
| Debugging failed tests | 3 | Multi-step analysis |

### 2.3 Routing Implementation

```go
type ModelRouter struct {
    tier1 LLMClient // Haiku/GPT-4o-mini
    tier2 LLMClient // Sonnet/GPT-4o
    tier3 LLMClient // Opus/GPT-4
}

type TaskType string

const (
    TaskSummarize       TaskType = "summarize"
    TaskExtract         TaskType = "extract"
    TaskEnumerate       TaskType = "enumerate"
    TaskBoilerplate     TaskType = "boilerplate"
    TaskGenerateInputs  TaskType = "generate_inputs"
    TaskGenerateAsserts TaskType = "generate_assertions"
    TaskEdgeCases       TaskType = "edge_cases"
    TaskComplexLogic    TaskType = "complex_logic"
    TaskCritic          TaskType = "critic"
    TaskDebug           TaskType = "debug"
)

func (r *ModelRouter) Route(task TaskType) LLMClient {
    switch task {
    case TaskSummarize, TaskExtract, TaskEnumerate, TaskBoilerplate:
        return r.tier1
    case TaskGenerateInputs, TaskGenerateAsserts, TaskEdgeCases:
        return r.tier2
    case TaskComplexLogic, TaskCritic, TaskDebug:
        return r.tier3
    default:
        return r.tier2 // Default to middle tier
    }
}

// Dynamic tier escalation based on context
func (r *ModelRouter) RouteWithContext(task TaskType, ctx TaskContext) LLMClient {
    baseTier := r.Route(task)

    // Escalate for critical paths
    if ctx.IsCriticalPath && baseTier == r.tier1 {
        return r.tier2
    }

    // Escalate for complex functions
    if ctx.Complexity > 20 && baseTier != r.tier3 {
        return r.tier3
    }

    // Downgrade for simple functions
    if ctx.Complexity < 5 && baseTier == r.tier2 {
        return r.tier1
    }

    return baseTier
}
```

## 3. Budget Management

### 3.1 Budget Hierarchy

```
Organization Budget
├── Monthly limit: 1,000,000 tokens
│
├── Team A Budget (40%)
│   ├── Repo 1: 200,000 tokens
│   └── Repo 2: 200,000 tokens
│
└── Team B Budget (60%)
    ├── Repo 3: 300,000 tokens
    └── Repo 4: 300,000 tokens

Per-run limits:
├── Default: 50,000 tokens
├── Large repo: 100,000 tokens
└── Enterprise: Configurable
```

### 3.2 Budget Enforcement

```go
type BudgetManager struct {
    store   BudgetStore
    metrics *prometheus.CounterVec
}

type Budget struct {
    UserID       string
    RepoID       string
    MonthlyLimit int64
    CurrentUsage int64
    ResetAt      time.Time
}

type BudgetCheckResult struct {
    Allowed       bool
    Remaining     int64
    ResetAt       time.Time
    SuggestedTier LLMTier
}

func (m *BudgetManager) CheckBudget(userID, repoID string, estimatedTokens int64) BudgetCheckResult {
    budget := m.store.Get(userID, repoID)

    remaining := budget.MonthlyLimit - budget.CurrentUsage

    if remaining < estimatedTokens {
        // Over budget - suggest downgrade
        return BudgetCheckResult{
            Allowed:       remaining > 0,
            Remaining:     remaining,
            ResetAt:       budget.ResetAt,
            SuggestedTier: LLMTier1, // Downgrade to cheap tier
        }
    }

    // Calculate suggested tier based on remaining budget
    percentRemaining := float64(remaining) / float64(budget.MonthlyLimit)
    var suggestedTier LLMTier
    switch {
    case percentRemaining > 0.5:
        suggestedTier = LLMTier3 // Plenty of budget
    case percentRemaining > 0.2:
        suggestedTier = LLMTier2 // Moderate budget
    default:
        suggestedTier = LLMTier1 // Conserve budget
    }

    return BudgetCheckResult{
        Allowed:       true,
        Remaining:     remaining,
        ResetAt:       budget.ResetAt,
        SuggestedTier: suggestedTier,
    }
}

func (m *BudgetManager) RecordUsage(userID, repoID string, tokens int64) error {
    return m.store.IncrementUsage(userID, repoID, tokens)
}
```

### 3.3 Budget Exhaustion Behavior

```
When budget is exhausted:

Option 1: Graceful degradation
  - Continue with Tier 1 only
  - Mark tests as "low-confidence"
  - Skip critic pass
  - Reduce test count

Option 2: Pause and notify
  - Stop generation
  - Notify user
  - Offer to purchase more tokens

Option 3: Best-effort completion
  - Complete current batch only
  - Skip remaining targets
  - Prioritize high-risk functions
```

## 4. Caching Strategy

### 4.1 Cache Layers

```
┌─────────────────────────────────────────────────────────────────┐
│                    CACHING LAYERS                                │
│                                                                  │
│  Layer 1: Request-Level Cache (Redis)                           │
│  ├── Key: hash(model + prompt)                                  │
│  ├── TTL: 1 hour                                                │
│  └── Hit rate: ~20%                                             │
│                                                                  │
│  Layer 2: Semantic Cache (Embeddings)                           │
│  ├── Key: embedding(prompt) with cosine similarity > 0.95       │
│  ├── TTL: 24 hours                                              │
│  └── Hit rate: ~15%                                             │
│                                                                  │
│  Layer 3: Pattern Cache (Templates)                             │
│  ├── Key: function_pattern (e.g., CRUD endpoint)                │
│  ├── TTL: 7 days                                                │
│  └── Hit rate: ~30%                                             │
│                                                                  │
│  Combined effective hit rate: ~50%                              │
│  Cost savings: ~50%                                             │
└─────────────────────────────────────────────────────────────────┘
```

### 4.2 Request Cache Implementation

```go
type LLMCache struct {
    redis   *redis.Client
    ttl     time.Duration
    metrics *CacheMetrics
}

func (c *LLMCache) Get(req CompletionRequest) (*CompletionResponse, bool) {
    key := c.cacheKey(req)

    data, err := c.redis.Get(context.Background(), key).Bytes()
    if err == redis.Nil {
        c.metrics.miss.Inc()
        return nil, false
    }

    var resp CompletionResponse
    if err := json.Unmarshal(data, &resp); err != nil {
        return nil, false
    }

    c.metrics.hit.Inc()
    return &resp, true
}

func (c *LLMCache) Set(req CompletionRequest, resp *CompletionResponse) error {
    key := c.cacheKey(req)
    data, _ := json.Marshal(resp)

    return c.redis.Set(context.Background(), key, data, c.ttl).Err()
}

func (c *LLMCache) cacheKey(req CompletionRequest) string {
    // Include model in key to avoid cross-model cache hits
    h := sha256.New()
    h.Write([]byte(req.Model))
    h.Write([]byte(req.SystemPrompt))
    h.Write([]byte(req.UserPrompt))
    return fmt.Sprintf("llm:cache:%x", h.Sum(nil))
}
```

### 4.3 Pattern Cache

Cache responses for common code patterns:

```go
type PatternCache struct {
    patterns map[PatternType][]TestDSL
}

type PatternType string

const (
    PatternCRUDEndpoint     PatternType = "crud_endpoint"
    PatternAuthMiddleware   PatternType = "auth_middleware"
    PatternValidatorFunc    PatternType = "validator_function"
    PatternEventHandler     PatternType = "event_handler"
    PatternDatabaseRepo     PatternType = "database_repository"
)

func (c *PatternCache) Match(fn Function) (PatternType, float64) {
    // Use simple heuristics to match patterns
    if containsKeywords(fn.Name, []string{"create", "update", "delete", "get", "list"}) &&
       fn.HasAnnotation("@Route") {
        return PatternCRUDEndpoint, 0.9
    }

    if fn.ReturnType.Name == "bool" &&
       containsKeywords(fn.Name, []string{"is", "has", "can", "should", "validate"}) {
        return PatternValidatorFunc, 0.85
    }

    // ... more patterns

    return "", 0.0
}

func (c *PatternCache) GetTemplateTests(pattern PatternType) []TestDSL {
    return c.patterns[pattern]
}
```

## 5. Prompt Optimization

### 5.1 Token Reduction Techniques

```
Original prompt: 2000 tokens
After optimization: 800 tokens (60% reduction)

Techniques:
├── Remove redundant context
├── Use shorter variable names in examples
├── Compress code (remove comments, whitespace)
├── Use schema references instead of full examples
└── Batch similar requests
```

### 5.2 Optimized Prompt Structure

```go
type PromptBuilder struct {
    maxTokens int
}

func (b *PromptBuilder) BuildTestGenerationPrompt(fn Function) string {
    // Start with minimal context
    prompt := fmt.Sprintf(`Generate test for:
%s

Signature: %s
Returns: %s
Branches: %v

Output format: YAML Test DSL (see schema)`,
        compressCode(fn.Source),
        fn.Signature(),
        fn.ReturnType.Name,
        summarizeBranches(fn.Branches),
    )

    // Add context only if budget allows
    if b.maxTokens > 1000 {
        prompt += fmt.Sprintf("\n\nDependencies:\n%s", fn.DependencySummary())
    }

    if b.maxTokens > 1500 {
        prompt += fmt.Sprintf("\n\nUsage examples:\n%s", fn.UsageExamples(3))
    }

    return prompt
}

func compressCode(code string) string {
    // Remove comments
    code = removeComments(code)
    // Normalize whitespace
    code = normalizeWhitespace(code)
    // Truncate if too long
    if len(code) > 2000 {
        code = code[:2000] + "..."
    }
    return code
}
```

### 5.3 Batching Requests

```go
// Instead of 10 separate requests, batch into 1
type BatchRequest struct {
    Functions []Function
}

func (g *Generator) BatchGenerate(fns []Function) ([]TestDSL, error) {
    if len(fns) <= 3 {
        // Batch into single request
        prompt := buildBatchPrompt(fns)
        resp, err := g.llm.Complete(prompt)
        return parseBatchResponse(resp)
    }

    // For larger batches, split into groups of 3
    var results []TestDSL
    for i := 0; i < len(fns); i += 3 {
        batch := fns[i:min(i+3, len(fns))]
        batchResults, err := g.BatchGenerate(batch)
        results = append(results, batchResults...)
    }
    return results, nil
}
```

## 6. Fallback Strategy

### 6.1 Provider Fallback

```go
type FallbackClient struct {
    primary   LLMClient // Claude
    secondary LLMClient // OpenAI
    tertiary  LLMClient // Local Llama
}

func (c *FallbackClient) Complete(req CompletionRequest) (*CompletionResponse, error) {
    // Try primary
    resp, err := c.primary.Complete(req)
    if err == nil {
        return resp, nil
    }

    // Log and try secondary
    log.Warn("Primary LLM failed, falling back", "error", err)

    resp, err = c.secondary.Complete(req)
    if err == nil {
        return resp, nil
    }

    // Log and try tertiary
    log.Warn("Secondary LLM failed, falling back to local", "error", err)

    return c.tertiary.Complete(req)
}
```

### 6.2 Tier Fallback

```go
func (r *ModelRouter) CompleteWithFallback(task TaskType, req CompletionRequest) (*CompletionResponse, error) {
    client := r.Route(task)

    resp, err := client.Complete(req)
    if err == nil {
        return resp, nil
    }

    // If premium tier failed, try lower tier
    if client == r.tier3 {
        log.Info("Tier 3 failed, falling back to Tier 2")
        return r.tier2.Complete(req)
    }

    if client == r.tier2 {
        log.Info("Tier 2 failed, falling back to Tier 1")
        return r.tier1.Complete(req)
    }

    return nil, err
}
```

## 7. Cost Tracking & Analytics

### 7.1 Usage Tracking

```go
type UsageTracker struct {
    db      *sql.DB
    metrics *prometheus.CounterVec
}

type UsageRecord struct {
    ID           string
    UserID       string
    RepoID       string
    RunID        string
    Model        string
    Tier         LLMTier
    InputTokens  int
    OutputTokens int
    CostCents    int
    CreatedAt    time.Time
}

func (t *UsageTracker) Record(record UsageRecord) error {
    // Calculate cost
    record.CostCents = t.calculateCost(record)

    // Store in database
    _, err := t.db.Exec(`
        INSERT INTO llm_usage
        (id, user_id, repo_id, run_id, model, tier, input_tokens, output_tokens, cost_cents, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
    `, record.ID, record.UserID, record.RepoID, record.RunID,
       record.Model, record.Tier, record.InputTokens, record.OutputTokens,
       record.CostCents, record.CreatedAt)

    // Update metrics
    t.metrics.WithLabelValues(record.Model, string(record.Tier)).Add(float64(record.CostCents))

    return err
}

func (t *UsageTracker) GetMonthlyUsage(userID string) (UsageSummary, error) {
    var summary UsageSummary

    err := t.db.QueryRow(`
        SELECT
            SUM(input_tokens) as total_input,
            SUM(output_tokens) as total_output,
            SUM(cost_cents) as total_cost,
            COUNT(*) as request_count
        FROM llm_usage
        WHERE user_id = $1
        AND created_at >= date_trunc('month', CURRENT_DATE)
    `, userID).Scan(&summary.TotalInput, &summary.TotalOutput,
                    &summary.TotalCostCents, &summary.RequestCount)

    return summary, err
}
```

### 7.2 Cost Dashboard Queries

```sql
-- Daily cost breakdown by tier
SELECT
    DATE(created_at) as date,
    tier,
    SUM(cost_cents) / 100.0 as cost_dollars,
    SUM(input_tokens + output_tokens) as total_tokens
FROM llm_usage
WHERE created_at >= CURRENT_DATE - INTERVAL '30 days'
GROUP BY DATE(created_at), tier
ORDER BY date DESC, tier;

-- Top repos by cost
SELECT
    r.full_name,
    SUM(u.cost_cents) / 100.0 as cost_dollars,
    COUNT(*) as request_count
FROM llm_usage u
JOIN repositories r ON u.repo_id = r.id
WHERE u.created_at >= CURRENT_DATE - INTERVAL '30 days'
GROUP BY r.id, r.full_name
ORDER BY cost_dollars DESC
LIMIT 10;

-- Cost per test generated
SELECT
    AVG(cost_per_test) as avg_cost_per_test
FROM (
    SELECT
        gr.id as run_id,
        SUM(u.cost_cents) / NULLIF(gr.tests_generated, 0) as cost_per_test
    FROM generation_runs gr
    JOIN llm_usage u ON gr.id = u.run_id
    WHERE gr.created_at >= CURRENT_DATE - INTERVAL '30 days'
    GROUP BY gr.id, gr.tests_generated
) subq;
```

### 7.3 Prometheus Metrics

```go
var (
    llmRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "qtest_llm_requests_total",
            Help: "Total LLM requests",
        },
        []string{"model", "tier", "status"},
    )

    llmTokensTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "qtest_llm_tokens_total",
            Help: "Total tokens used",
        },
        []string{"model", "tier", "direction"}, // direction: input/output
    )

    llmCostCents = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "qtest_llm_cost_cents_total",
            Help: "Total cost in cents",
        },
        []string{"model", "tier"},
    )

    llmLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "qtest_llm_latency_seconds",
            Help:    "LLM request latency",
            Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
        },
        []string{"model", "tier"},
    )

    cacheHitRatio = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "qtest_llm_cache_hit_ratio",
            Help: "Cache hit ratio",
        },
        []string{"cache_type"},
    )
)
```

## 8. Cost Optimization Checklist

### 8.1 Pre-Launch

- [ ] Implement tiered model routing
- [ ] Set up per-user/repo budgets
- [ ] Enable request-level caching
- [ ] Configure fallback providers
- [ ] Set up cost monitoring dashboards

### 8.2 Ongoing

- [ ] Review tier mapping monthly (optimize based on quality metrics)
- [ ] Analyze cache hit rates (improve if below 40%)
- [ ] Monitor cost per test (target: <$0.05)
- [ ] Identify high-cost repos and optimize prompts
- [ ] Test new cheaper models as they release

### 8.3 Cost Alerts

```yaml
alerts:
  - name: HighLLMCost
    condition: daily_cost > $100
    action: notify_team

  - name: BudgetExceeded
    condition: monthly_cost > budget * 0.8
    action: notify_user, reduce_tier

  - name: AbnormalUsage
    condition: hourly_tokens > 2x avg
    action: investigate, possible_abuse
```

## 9. Enterprise Considerations

### 9.1 Self-Hosted LLM Option

For enterprises that can't send code to external APIs:

```yaml
deployment: self-hosted

llm:
  provider: vllm
  model: llama-3.1-70b
  endpoint: http://internal-llm.company.com
  gpu: 4x A100

costs:
  infrastructure: ~$10k/month (4x A100 instance)
  per_token: $0 (no API costs)
  break_even: ~50M tokens/month
```

### 9.2 Hybrid Approach

```
Tier 1: Local Llama (free, fast)
  └── All boilerplate, summaries, extractions

Tier 2: Claude API (paid, better quality)
  └── Test generation, assertions

Tier 3: Claude API or Local (configurable)
  └── Complex reasoning, critics

Savings: ~70% reduction in API costs
```

## 10. Summary

### 10.1 Key Strategies

1. **Tier appropriately** - Use cheap models for simple tasks
2. **Cache aggressively** - Target 50%+ hit rate
3. **Budget strictly** - Hard limits prevent runaway costs
4. **Monitor continuously** - Catch anomalies early
5. **Optimize prompts** - Fewer tokens = lower cost

### 10.2 Target Metrics

| Metric | Target |
|--------|--------|
| Cost per test | <$0.05 |
| Cache hit rate | >50% |
| Tier 1 usage | >40% of requests |
| Budget overrun | 0% |
| Cost per repo/month | <$50 |
