package llm

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

var (
	// ErrBudgetExceeded indicates the budget limit has been reached
	ErrBudgetExceeded = errors.New("LLM budget exceeded")
	// ErrRateLimited indicates too many requests
	ErrRateLimited = errors.New("rate limit exceeded")
)

// UsageRecord represents a single LLM usage event
type UsageRecord struct {
	ID           uuid.UUID  `json:"id"`
	Timestamp    time.Time  `json:"timestamp"`
	Provider     Provider   `json:"provider"`
	Model        string     `json:"model"`
	Tier         Tier       `json:"tier"`
	InputTokens  int        `json:"input_tokens"`
	OutputTokens int        `json:"output_tokens"`
	TotalTokens  int        `json:"total_tokens"`
	Cost         float64    `json:"cost"` // Estimated cost in USD
	UserID       *uuid.UUID `json:"user_id,omitempty"`
	RequestID    string     `json:"request_id,omitempty"`
	Duration     float64    `json:"duration_ms"`
}

// UsageStats provides aggregate usage statistics
type UsageStats struct {
	TotalRequests   int64   `json:"total_requests"`
	TotalTokens     int64   `json:"total_tokens"`
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	EstimatedCost   float64 `json:"estimated_cost_usd"`
	AvgTokensPerReq float64 `json:"avg_tokens_per_request"`
	Period          string  `json:"period"` // "hour", "day", "month"
}

// BudgetConfig configures budget limits
type BudgetConfig struct {
	// HourlyTokenLimit limits tokens per hour (0 = unlimited)
	HourlyTokenLimit int64
	// DailyTokenLimit limits tokens per day (0 = unlimited)
	DailyTokenLimit int64
	// MonthlyBudgetUSD limits monthly spend in USD (0 = unlimited)
	MonthlyBudgetUSD float64
	// RequestsPerMinute limits request rate (0 = unlimited)
	RequestsPerMinute int
}

// UsageTracker tracks LLM usage and enforces budgets
type UsageTracker struct {
	mu sync.RWMutex

	// Configuration
	budget BudgetConfig

	// Counters
	hourlyTokens  int64
	dailyTokens   int64
	monthlyTokens int64
	monthlyCost   float64

	// Rate limiting
	requestsThisMinute int32
	lastMinuteReset    time.Time

	// History (rolling window)
	records     []UsageRecord
	maxRecords  int
	recordIndex int

	// Cost estimation per 1K tokens (configurable)
	costPer1K map[Provider]map[string]float64
}

// UsageTrackerConfig configures the usage tracker
type UsageTrackerConfig struct {
	Budget     BudgetConfig
	MaxRecords int // Max records to keep in memory
}

// NewUsageTracker creates a new usage tracker
func NewUsageTracker(cfg UsageTrackerConfig) *UsageTracker {
	if cfg.MaxRecords == 0 {
		cfg.MaxRecords = 1000
	}

	t := &UsageTracker{
		budget:          cfg.Budget,
		records:         make([]UsageRecord, cfg.MaxRecords),
		maxRecords:      cfg.MaxRecords,
		lastMinuteReset: time.Now(),
		costPer1K:       defaultCostPer1K(),
	}

	// Start background cleanup
	go t.periodicReset()

	return t
}

// defaultCostPer1K returns default cost estimates per 1K tokens
// These are estimates and should be updated based on actual pricing
func defaultCostPer1K() map[Provider]map[string]float64 {
	return map[Provider]map[string]float64{
		ProviderOllama: {
			"default": 0.0, // Local models are free
		},
		ProviderAnthropic: {
			"claude-3-haiku-20240307":    0.00025 + 0.00125, // input + output avg
			"claude-3-5-sonnet-20241022": 0.003 + 0.015,
			"claude-3-opus-20240229":     0.015 + 0.075,
			"default":                    0.005,
		},
		ProviderOpenAI: {
			"gpt-4":         0.03 + 0.06,
			"gpt-4-turbo":   0.01 + 0.03,
			"gpt-3.5-turbo": 0.0005 + 0.0015,
			"default":       0.01,
		},
	}
}

// Record records a usage event
func (t *UsageTracker) Record(record UsageRecord) {
	record.ID = uuid.New()
	record.Timestamp = time.Now()
	record.TotalTokens = record.InputTokens + record.OutputTokens
	record.Cost = t.estimateCost(record)

	t.mu.Lock()
	defer t.mu.Unlock()

	// Update counters
	atomic.AddInt64(&t.hourlyTokens, int64(record.TotalTokens))
	atomic.AddInt64(&t.dailyTokens, int64(record.TotalTokens))
	atomic.AddInt64(&t.monthlyTokens, int64(record.TotalTokens))
	t.monthlyCost += record.Cost

	// Store record
	t.records[t.recordIndex] = record
	t.recordIndex = (t.recordIndex + 1) % t.maxRecords

	log.Debug().
		Str("provider", string(record.Provider)).
		Str("model", record.Model).
		Int("input_tokens", record.InputTokens).
		Int("output_tokens", record.OutputTokens).
		Float64("cost", record.Cost).
		Msg("recorded LLM usage")
}

// CheckBudget checks if the request is within budget
func (t *UsageTracker) CheckBudget(estimatedTokens int) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Check hourly limit
	if t.budget.HourlyTokenLimit > 0 {
		if atomic.LoadInt64(&t.hourlyTokens)+int64(estimatedTokens) > t.budget.HourlyTokenLimit {
			log.Warn().
				Int64("current", atomic.LoadInt64(&t.hourlyTokens)).
				Int64("limit", t.budget.HourlyTokenLimit).
				Msg("hourly token limit would be exceeded")
			return ErrBudgetExceeded
		}
	}

	// Check daily limit
	if t.budget.DailyTokenLimit > 0 {
		if atomic.LoadInt64(&t.dailyTokens)+int64(estimatedTokens) > t.budget.DailyTokenLimit {
			log.Warn().
				Int64("current", atomic.LoadInt64(&t.dailyTokens)).
				Int64("limit", t.budget.DailyTokenLimit).
				Msg("daily token limit would be exceeded")
			return ErrBudgetExceeded
		}
	}

	// Check monthly budget
	if t.budget.MonthlyBudgetUSD > 0 {
		if t.monthlyCost >= t.budget.MonthlyBudgetUSD {
			log.Warn().
				Float64("current", t.monthlyCost).
				Float64("limit", t.budget.MonthlyBudgetUSD).
				Msg("monthly budget exceeded")
			return ErrBudgetExceeded
		}
	}

	// Check rate limit
	if t.budget.RequestsPerMinute > 0 {
		t.updateRateLimit()
		if int(atomic.LoadInt32(&t.requestsThisMinute)) >= t.budget.RequestsPerMinute {
			log.Warn().
				Int32("current", atomic.LoadInt32(&t.requestsThisMinute)).
				Int("limit", t.budget.RequestsPerMinute).
				Msg("rate limit would be exceeded")
			return ErrRateLimited
		}
	}

	return nil
}

// IncrementRequests increments the request counter
func (t *UsageTracker) IncrementRequests() {
	atomic.AddInt32(&t.requestsThisMinute, 1)
}

// GetStats returns usage statistics
func (t *UsageTracker) GetStats() UsageStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Count actual records
	var totalRequests int64
	for i := range t.records {
		if t.records[i].ID != uuid.Nil {
			totalRequests++
		}
	}

	monthlyTokens := atomic.LoadInt64(&t.monthlyTokens)

	var avgTokens float64
	if totalRequests > 0 {
		avgTokens = float64(monthlyTokens) / float64(totalRequests)
	}

	return UsageStats{
		TotalRequests:   totalRequests,
		TotalTokens:     monthlyTokens,
		InputTokens:     0, // Would need to track separately
		OutputTokens:    0, // Would need to track separately
		EstimatedCost:   t.monthlyCost,
		AvgTokensPerReq: avgTokens,
		Period:          "month",
	}
}

// GetHourlyStats returns hourly statistics
func (t *UsageTracker) GetHourlyStats() UsageStats {
	hourlyTokens := atomic.LoadInt64(&t.hourlyTokens)
	return UsageStats{
		TotalTokens:   hourlyTokens,
		EstimatedCost: 0, // Would need to track separately
		Period:        "hour",
	}
}

// GetDailyStats returns daily statistics
func (t *UsageTracker) GetDailyStats() UsageStats {
	dailyTokens := atomic.LoadInt64(&t.dailyTokens)
	return UsageStats{
		TotalTokens:   dailyTokens,
		EstimatedCost: 0, // Would need to track separately
		Period:        "day",
	}
}

// GetBudgetStatus returns current budget status
func (t *UsageTracker) GetBudgetStatus() BudgetStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()

	status := BudgetStatus{
		HourlyTokensUsed:  atomic.LoadInt64(&t.hourlyTokens),
		HourlyTokensLimit: t.budget.HourlyTokenLimit,
		DailyTokensUsed:   atomic.LoadInt64(&t.dailyTokens),
		DailyTokensLimit:  t.budget.DailyTokenLimit,
		MonthlySpentUSD:   t.monthlyCost,
		MonthlyBudgetUSD:  t.budget.MonthlyBudgetUSD,
	}

	// Calculate percentages
	if t.budget.HourlyTokenLimit > 0 {
		status.HourlyPercentUsed = float64(status.HourlyTokensUsed) / float64(t.budget.HourlyTokenLimit) * 100
	}
	if t.budget.DailyTokenLimit > 0 {
		status.DailyPercentUsed = float64(status.DailyTokensUsed) / float64(t.budget.DailyTokenLimit) * 100
	}
	if t.budget.MonthlyBudgetUSD > 0 {
		status.MonthlyPercentUsed = status.MonthlySpentUSD / t.budget.MonthlyBudgetUSD * 100
	}

	return status
}

// BudgetStatus represents current budget status
type BudgetStatus struct {
	HourlyTokensUsed   int64   `json:"hourly_tokens_used"`
	HourlyTokensLimit  int64   `json:"hourly_tokens_limit"`
	HourlyPercentUsed  float64 `json:"hourly_percent_used"`
	DailyTokensUsed    int64   `json:"daily_tokens_used"`
	DailyTokensLimit   int64   `json:"daily_tokens_limit"`
	DailyPercentUsed   float64 `json:"daily_percent_used"`
	MonthlySpentUSD    float64 `json:"monthly_spent_usd"`
	MonthlyBudgetUSD   float64 `json:"monthly_budget_usd"`
	MonthlyPercentUsed float64 `json:"monthly_percent_used"`
}

// RecentRecords returns recent usage records
func (t *UsageTracker) RecentRecords(limit int) []UsageRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if limit > t.maxRecords {
		limit = t.maxRecords
	}

	result := make([]UsageRecord, 0, limit)

	// Start from most recent
	idx := (t.recordIndex - 1 + t.maxRecords) % t.maxRecords
	for i := 0; i < limit && i < t.maxRecords; i++ {
		if t.records[idx].ID != uuid.Nil {
			result = append(result, t.records[idx])
		}
		idx = (idx - 1 + t.maxRecords) % t.maxRecords
	}

	return result
}

// estimateCost estimates the cost of a usage record
func (t *UsageTracker) estimateCost(record UsageRecord) float64 {
	providerCosts, ok := t.costPer1K[record.Provider]
	if !ok {
		return 0
	}

	costPer1K, ok := providerCosts[record.Model]
	if !ok {
		costPer1K = providerCosts["default"]
	}

	return float64(record.TotalTokens) / 1000.0 * costPer1K
}

// updateRateLimit resets the rate limit counter if needed
func (t *UsageTracker) updateRateLimit() {
	now := time.Now()
	if now.Sub(t.lastMinuteReset) >= time.Minute {
		atomic.StoreInt32(&t.requestsThisMinute, 0)
		t.lastMinuteReset = now
	}
}

// periodicReset resets counters periodically
func (t *UsageTracker) periodicReset() {
	hourTicker := time.NewTicker(time.Hour)
	dayTicker := time.NewTicker(24 * time.Hour)

	for {
		select {
		case <-hourTicker.C:
			atomic.StoreInt64(&t.hourlyTokens, 0)
			log.Debug().Msg("reset hourly token counter")
		case <-dayTicker.C:
			atomic.StoreInt64(&t.dailyTokens, 0)
			log.Debug().Msg("reset daily token counter")
		}
	}
}

// ExportJSON exports usage records as JSON
func (t *UsageTracker) ExportJSON() ([]byte, error) {
	records := t.RecentRecords(t.maxRecords)
	return json.Marshal(records)
}

// TrackedRouter wraps a Router with usage tracking
type TrackedRouter struct {
	*Router
	tracker *UsageTracker
}

// NewTrackedRouter creates a router with usage tracking
func NewTrackedRouter(router *Router, tracker *UsageTracker) *TrackedRouter {
	return &TrackedRouter{
		Router:  router,
		tracker: tracker,
	}
}

// Complete sends a completion request with usage tracking
func (r *TrackedRouter) Complete(ctx context.Context, req *Request) (*Response, error) {
	// Estimate tokens from messages
	var totalChars int
	for _, msg := range req.Messages {
		totalChars += len(msg.Content)
	}
	totalChars += len(req.System)
	estimatedTokens := totalChars / 4 // Rough estimate: 4 chars per token

	// Check budget before request
	if err := r.tracker.CheckBudget(estimatedTokens); err != nil {
		return nil, err
	}

	r.tracker.IncrementRequests()

	start := time.Now()
	resp, err := r.Router.Complete(ctx, req)
	duration := time.Since(start)

	if err == nil && resp != nil {
		// Record usage
		r.tracker.Record(UsageRecord{
			Provider:     resp.Provider,
			Model:        resp.Model,
			Tier:         req.Tier,
			InputTokens:  resp.InputTokens,
			OutputTokens: resp.OutputTokens,
			Duration:     float64(duration.Milliseconds()),
		})
	}

	return resp, err
}

// GetTracker returns the usage tracker
func (r *TrackedRouter) GetTracker() *UsageTracker {
	return r.tracker
}
