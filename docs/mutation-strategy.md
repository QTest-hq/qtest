# Mutation Testing Strategy

## 1. Overview

Mutation testing is the **quality gate** that ensures generated tests actually catch bugs. Without it, tests may achieve high coverage while failing to detect real defects.

### 1.1 The Problem with Coverage-Only Metrics

```
Traditional approach:
  Code Coverage: 90% ✓
  Tests Pass: 100% ✓
  Actual Bug Detection: ??? (unknown)

With mutation testing:
  Code Coverage: 90% ✓
  Tests Pass: 100% ✓
  Mutation Score: 0.78 ✓  ← Tests kill 78% of simulated bugs
```

### 1.2 How Mutation Testing Works

```
┌─────────────────────────────────────────────────────────────────┐
│                    MUTATION TESTING FLOW                         │
│                                                                  │
│  1. Original Code                                               │
│     function add(a, b) { return a + b; }                        │
│                                                                  │
│  2. Generate Mutants                                            │
│     Mutant 1: return a - b;  (arithmetic operator)              │
│     Mutant 2: return a * b;  (arithmetic operator)              │
│     Mutant 3: return a;      (return value)                     │
│     Mutant 4: return 0;      (return value)                     │
│                                                                  │
│  3. Run Tests Against Each Mutant                               │
│     Test: expect(add(2, 3)).toBe(5)                             │
│                                                                  │
│     Mutant 1: add(2, 3) = -1 ≠ 5 → KILLED ✓                    │
│     Mutant 2: add(2, 3) = 6  ≠ 5 → KILLED ✓                    │
│     Mutant 3: add(2, 3) = 2  ≠ 5 → KILLED ✓                    │
│     Mutant 4: add(2, 3) = 0  ≠ 5 → KILLED ✓                    │
│                                                                  │
│  4. Calculate Score                                             │
│     Mutation Score = 4/4 = 1.0 (100%)                           │
│     This test is highly effective!                              │
└─────────────────────────────────────────────────────────────────┘
```

## 2. Mutation Types

### 2.1 Supported Mutations

| Category | Mutation | Original | Mutated |
|----------|----------|----------|---------|
| **Arithmetic** | Addition → Subtraction | `a + b` | `a - b` |
| | Subtraction → Addition | `a - b` | `a + b` |
| | Multiplication → Division | `a * b` | `a / b` |
| | Division → Multiplication | `a / b` | `a * b` |
| | Modulo → Multiplication | `a % b` | `a * b` |
| **Comparison** | Greater than → Less than | `a > b` | `a < b` |
| | Greater than → Greater/equal | `a > b` | `a >= b` |
| | Less than → Greater than | `a < b` | `a > b` |
| | Equals → Not equals | `a == b` | `a != b` |
| | Not equals → Equals | `a != b` | `a == b` |
| **Boolean** | True → False | `true` | `false` |
| | False → True | `false` | `true` |
| | AND → OR | `a && b` | `a \|\| b` |
| | OR → AND | `a \|\| b` | `a && b` |
| | Negate condition | `if (x)` | `if (!x)` |
| **Return** | Return value | `return x` | `return null` |
| | Return constant | `return x` | `return 0` |
| | Empty return | `return x` | `return` |
| **Branch** | Remove if body | `if (x) { ... }` | `if (x) { }` |
| | Remove else body | `else { ... }` | `else { }` |
| | Always true | `if (cond)` | `if (true)` |
| | Always false | `if (cond)` | `if (false)` |
| **Method Call** | Remove call | `foo()` | (removed) |
| | Change argument | `foo(x)` | `foo(null)` |
| **String** | Empty string | `"hello"` | `""` |
| | Different string | `"hello"` | `"mutated"` |
| **Array** | Empty array | `[1,2,3]` | `[]` |
| | Remove element | `[1,2,3]` | `[1,2]` |

### 2.2 Mutation Priority

Not all mutations are equally valuable. Prioritize by bug-detection likelihood:

```
HIGH PRIORITY (always include):
  - Comparison operators (>, <, ==, !=)
  - Boolean negation
  - Return value changes

MEDIUM PRIORITY (include if budget allows):
  - Arithmetic operators
  - Branch removal
  - Method call removal

LOW PRIORITY (rarely include):
  - String mutations
  - Array mutations
  - Constant changes
```

## 3. Scalability Strategy

### 3.1 The Challenge

Full mutation testing is O(M × T) where:
- M = number of mutants
- T = time to run test suite

For a large codebase: 10,000 functions × 50 mutants each × 10ms/test = **~83 hours**

### 3.2 Scope Minimization

**Principle**: Only mutate what's relevant to the generated tests.

```
┌─────────────────────────────────────────────────────────────────┐
│                    SCOPE MINIMIZATION                            │
│                                                                  │
│  Full Repo: 10,000 functions                                    │
│                    ↓                                            │
│  This Run: 50 new tests targeting 30 functions                  │
│                    ↓                                            │
│  Mutate Only: Those 30 functions                                │
│                    ↓                                            │
│  Effort: 30 × 5 mutants × 10ms = 1.5 seconds                    │
└─────────────────────────────────────────────────────────────────┘
```

**Implementation**:
```go
func selectMutationTargets(tests []TestResult) []Function {
    targets := make(map[string]Function)

    for _, test := range tests {
        // Get the function/method this test covers
        fn := getTargetFunction(test.TargetID)
        targets[fn.ID] = fn
    }

    return maps.Values(targets)
}
```

### 3.3 Mutant Sampling

**Principle**: Generate a small, representative set of mutants per function.

```
Per-function limits:
  - Maximum 5 mutants per function
  - At least 1 from each category (if applicable)
  - Random selection within category

Selection algorithm:
  1. Identify applicable mutation types for function
  2. Select 1 representative from each category
  3. Fill remaining slots randomly
  4. Cap at 5 total
```

**Implementation**:
```go
func sampleMutants(fn Function, limit int) []Mutant {
    allMutants := generateAllMutants(fn)

    // Group by category
    byCategory := groupByCategory(allMutants)

    selected := []Mutant{}

    // One from each category
    for _, category := range priorityOrder {
        if mutants, ok := byCategory[category]; ok && len(selected) < limit {
            selected = append(selected, mutants[0])
        }
    }

    // Fill remaining slots randomly
    remaining := flattenExcluding(byCategory, selected)
    rand.Shuffle(len(remaining), func(i, j int) {
        remaining[i], remaining[j] = remaining[j], remaining[i]
    })

    for _, m := range remaining {
        if len(selected) >= limit {
            break
        }
        selected = append(selected, m)
    }

    return selected
}
```

### 3.4 Time Budgeting

**Principle**: Hard cap mutation testing time per module/run.

```
Time budget allocation:
  - Per function: 30 seconds max
  - Per mutant: 5 seconds max (then timeout)
  - Per module: 5 minutes max
  - Per run: 10 minutes max

Behavior on timeout:
  - Mark mutant as TIMEOUT (counts as survived)
  - Continue to next mutant
  - Log for analysis
```

**Implementation**:
```go
func runMutationWithBudget(test TestResult, fn Function, budget time.Duration) MutationResult {
    mutants := sampleMutants(fn, 5)
    results := []MutantResult{}

    deadline := time.Now().Add(budget)

    for _, mutant := range mutants {
        if time.Now().After(deadline) {
            // Budget exhausted, mark remaining as timeout
            results = append(results, MutantResult{
                Mutant: mutant,
                Status: MutantStatusTimeout,
            })
            continue
        }

        result := runWithTimeout(test, mutant, 5*time.Second)
        results = append(results, result)
    }

    return calculateResult(results)
}
```

### 3.5 Incremental Mutation

**Principle**: Cache and reuse mutation results across runs.

```
Cache key: hash(function_content + test_content)

On re-run:
  1. Check if function content changed → if not, skip mutation
  2. Check if test content changed → if not, reuse result
  3. Only re-run mutations for changed code/tests
```

**Cache storage**:
```go
type MutationCache struct {
    FunctionHash string    // Hash of function source
    TestHash     string    // Hash of test source
    Result       MutationResult
    CachedAt     time.Time
    ExpiresAt    time.Time // Cache for 7 days
}
```

## 4. Execution Modes

### 4.1 Mode: Fast (Default)

For development and PR workflows.

```yaml
mutation:
  mode: fast
  mutants_per_function: 3
  timeout_per_mutant: 3s
  total_timeout: 2m
  skip_low_priority: true
```

### 4.2 Mode: Thorough

For nightly/CI and quality assurance.

```yaml
mutation:
  mode: thorough
  mutants_per_function: 5
  timeout_per_mutant: 10s
  total_timeout: 10m
  skip_low_priority: false
```

### 4.3 Mode: Off

For cost-sensitive or time-critical scenarios.

```yaml
mutation:
  mode: off
  # Tests are shipped without mutation validation
  # Quality disclaimer added to PR
```

## 5. Quality Thresholds

### 5.1 Per-Test Thresholds

```
PASS: Test kills ≥1 mutant
  → Test is validated, include in output

WEAK: Test kills 0 mutants but compiles/passes
  → Attempt strengthening (1 retry)
  → If still weak, discard

ERROR: Test fails to compile or run
  → Discard immediately
```

### 5.2 Per-Run Thresholds

```
GOOD: Mutation score ≥ 0.70
  → High confidence in generated tests

ACCEPTABLE: Mutation score 0.50-0.70
  → Tests are useful but could be stronger

POOR: Mutation score < 0.50
  → Flag for review, suggest manual tests
```

### 5.3 Reporting Thresholds

```go
type QualityReport struct {
    MutationScore    float64 // 0.0-1.0
    QualityLevel     string  // "good" | "acceptable" | "poor"
    TestsValidated   int
    TestsRejected    int
    MutantsKilled    int
    MutantsSurvived  int
    Recommendations  []string
}

func calculateQuality(results []MutationResult) QualityReport {
    killed := sumKilled(results)
    total := sumTotal(results)
    score := float64(killed) / float64(total)

    var level string
    var recommendations []string

    switch {
    case score >= 0.70:
        level = "good"
    case score >= 0.50:
        level = "acceptable"
        recommendations = append(recommendations,
            "Consider adding more edge case tests")
    default:
        level = "poor"
        recommendations = append(recommendations,
            "Many tests are weak - review assertion coverage",
            "Consider manual tests for complex logic")
    }

    return QualityReport{
        MutationScore:   score,
        QualityLevel:    level,
        TestsValidated:  countValidated(results),
        TestsRejected:   countRejected(results),
        MutantsKilled:   killed,
        MutantsSurvived: total - killed,
        Recommendations: recommendations,
    }
}
```

## 6. Test Strengthening

When a test fails to kill mutants, attempt automatic strengthening.

### 6.1 Strengthening Process

```
┌─────────────────────────────────────────────────────────────────┐
│                    STRENGTHENING FLOW                            │
│                                                                  │
│  1. Identify Surviving Mutants                                  │
│     - Mutant: return a - b (survived)                           │
│     - Test: expect(add(2,2)).toBe(4)                            │
│                                                                  │
│  2. Analyze Why Test Didn't Kill                                │
│     - add(2,2) with mutation = 2-2 = 0 ≠ 4 → Should be killed!  │
│     - Ah, the test uses toBe(4) which is correct for original   │
│     - But mutation with (2,2) still differs from 4              │
│                                                                  │
│  3. Problem: Test inputs happen to work for mutation too        │
│     - add(2,3) → mutation gives 2-3 = -1, test expects 5        │
│     - This would kill the mutant!                               │
│                                                                  │
│  4. LLM Strengthening Prompt                                    │
│     "The following test does not detect this mutation:          │
│      [test code]                                                 │
│      Mutation: a + b → a - b                                    │
│      Add or modify assertions to catch this mutation."          │
│                                                                  │
│  5. Re-run Mutation                                             │
│     - If kills mutant → validated                               │
│     - If still survives → discard test                          │
└─────────────────────────────────────────────────────────────────┘
```

### 6.2 Strengthening Limits

```
Max strengthening attempts: 2
Token budget for strengthening: 500 tokens per test
Timeout: 30 seconds per attempt

If still weak after 2 attempts:
  - Discard test
  - Log surviving mutants for analysis
  - Do NOT retry infinitely
```

### 6.3 LLM Prompt for Strengthening

```
You are improving a test to catch more bugs. The test currently passes
but fails to detect certain code mutations.

## Original Function
```{language}
{function_code}
```

## Current Test
```{language}
{test_code}
```

## Surviving Mutations
The following mutations were NOT detected by the test:
{surviving_mutations}

## Task
Modify the test to detect these mutations. You can:
1. Add new assertions
2. Add new test inputs that expose the mutations
3. Check additional properties of the result

Return only the modified test code.
```

## 7. Language-Specific Tools

### 7.1 JavaScript/TypeScript: Stryker

```bash
# Installation
npm install --save-dev @stryker-mutator/core @stryker-mutator/typescript-checker

# Configuration (stryker.conf.json)
{
  "mutate": ["src/**/*.ts", "!src/**/*.test.ts"],
  "testRunner": "jest",
  "reporters": ["json", "progress"],
  "concurrency": 4,
  "timeoutMS": 5000
}

# Run
npx stryker run
```

### 7.2 Python: mutmut

```bash
# Installation
pip install mutmut

# Run
mutmut run --paths-to-mutate=src/ --tests-dir=tests/

# Results
mutmut results
mutmut show <mutant_id>
```

### 7.3 Java: PIT (Pitest)

```xml
<!-- pom.xml -->
<plugin>
    <groupId>org.pitest</groupId>
    <artifactId>pitest-maven</artifactId>
    <version>1.15.0</version>
    <configuration>
        <targetClasses>
            <param>com.example.*</param>
        </targetClasses>
        <targetTests>
            <param>com.example.*Test</param>
        </targetTests>
    </configuration>
</plugin>
```

```bash
mvn org.pitest:pitest-maven:mutationCoverage
```

### 7.4 Go: go-mutesting

```bash
# Installation
go install github.com/zimmski/go-mutesting/cmd/go-mutesting@latest

# Run
go-mutesting ./...
```

## 8. Integration Architecture

### 8.1 Mutation Worker

```go
type MutationWorker struct {
    tools   map[string]MutationTool // Language -> tool
    cache   MutationCache
    metrics *prometheus.CounterVec
}

type MutationTool interface {
    GenerateMutants(code string, opts MutationOpts) ([]Mutant, error)
    RunMutant(test, mutant string, timeout time.Duration) (MutantResult, error)
}

func (w *MutationWorker) Process(job MutationJob) (MutationResult, error) {
    // 1. Check cache
    if cached := w.cache.Get(job.CacheKey()); cached != nil {
        return *cached, nil
    }

    // 2. Select tool based on language
    tool := w.tools[job.Language]

    // 3. Generate mutants (sampled)
    mutants, err := tool.GenerateMutants(job.TargetCode, MutationOpts{
        Limit: 5,
        Timeout: 30 * time.Second,
    })

    // 4. Run tests against each mutant
    results := make([]MutantResult, len(mutants))
    for i, mutant := range mutants {
        results[i] = tool.RunMutant(job.TestCode, mutant, 5*time.Second)
    }

    // 5. Calculate score
    result := calculateMutationResult(results)

    // 6. Cache result
    w.cache.Set(job.CacheKey(), result, 7*24*time.Hour)

    return result, nil
}
```

### 8.2 Containerized Execution

```yaml
# docker-compose.mutation.yml
services:
  mutation-runner-js:
    image: qtest/mutation-runner:js
    volumes:
      - /tmp/workdir:/workdir
    environment:
      - STRYKER_CONFIG=/config/stryker.json
    tmpfs:
      - /tmp:size=1G
    security_opt:
      - no-new-privileges:true
    read_only: true

  mutation-runner-python:
    image: qtest/mutation-runner:python
    volumes:
      - /tmp/workdir:/workdir
    tmpfs:
      - /tmp:size=1G
```

## 9. Metrics & Monitoring

### 9.1 Key Metrics

```go
var (
    mutationJobsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "qtest_mutation_jobs_total",
            Help: "Total mutation testing jobs",
        },
        []string{"language", "status"},
    )

    mutationScore = prometheus.NewHistogram(
        prometheus.HistogramOpts{
            Name:    "qtest_mutation_score",
            Help:    "Distribution of mutation scores",
            Buckets: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
        },
    )

    mutationDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "qtest_mutation_duration_seconds",
            Help:    "Time spent on mutation testing",
            Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
        },
        []string{"language"},
    )
)
```

### 9.2 Alerting Rules

```yaml
groups:
  - name: mutation
    rules:
      - alert: MutationScoreLow
        expr: avg(qtest_mutation_score) < 0.5
        for: 1h
        labels:
          severity: warning
        annotations:
          summary: "Average mutation score below 50%"

      - alert: MutationTimeoutHigh
        expr: rate(qtest_mutation_jobs_total{status="timeout"}[1h]) > 0.1
        for: 30m
        labels:
          severity: warning
        annotations:
          summary: "High rate of mutation timeouts"
```

## 10. Best Practices

### 10.1 Do's

- ✅ Run mutation testing on every generated test
- ✅ Cache results to avoid redundant computation
- ✅ Sample mutants to keep runtime manageable
- ✅ Use language-specific tools (Stryker, PIT) for best results
- ✅ Set hard timeouts to prevent runaway tests
- ✅ Log surviving mutants for continuous improvement

### 10.2 Don'ts

- ❌ Don't run full mutation on entire codebase
- ❌ Don't skip mutation testing for "simple" functions
- ❌ Don't retry strengthening indefinitely
- ❌ Don't count timeout mutants as killed
- ❌ Don't cache results forever (code changes)
