package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// APIUsage represents a single API request tracking record
type APIUsage struct {
	ID                uuid.UUID  `json:"id"`
	OrganizationID    *uuid.UUID `json:"organization_id,omitempty"`
	UserID            *uuid.UUID `json:"user_id,omitempty"`
	APIKeyID          *uuid.UUID `json:"api_key_id,omitempty"`
	Endpoint          string     `json:"endpoint"`
	Method            string     `json:"method"`
	StatusCode        int        `json:"status_code"`
	ResponseTimeMs    int        `json:"response_time_ms"`
	RequestSizeBytes  int        `json:"request_size_bytes"`
	ResponseSizeBytes int        `json:"response_size_bytes"`
	IPAddress         *string    `json:"ip_address,omitempty"`
	UserAgent         *string    `json:"user_agent,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

// UsageStatsDaily represents daily aggregated usage statistics
type UsageStatsDaily struct {
	ID                  uuid.UUID `json:"id"`
	OrganizationID      uuid.UUID `json:"organization_id"`
	Date                time.Time `json:"date"`
	TotalRequests       int       `json:"total_requests"`
	SuccessfulRequests  int       `json:"successful_requests"`
	FailedRequests      int       `json:"failed_requests"`
	JobsCreated         int       `json:"jobs_created"`
	JobsCompleted       int       `json:"jobs_completed"`
	JobsFailed          int       `json:"jobs_failed"`
	TestsGenerated      int       `json:"tests_generated"`
	TestsValidated      int       `json:"tests_validated"`
	TestsAccepted       int       `json:"tests_accepted"`
	TotalResponseTimeMs int64     `json:"total_response_time_ms"`
	TotalRequestBytes   int64     `json:"total_request_bytes"`
	TotalResponseBytes  int64     `json:"total_response_bytes"`
	UniqueUsers         int       `json:"unique_users"`
	UniqueAPIKeys       int       `json:"unique_api_keys"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// UsageStatsMonthly represents monthly aggregated usage statistics
type UsageStatsMonthly struct {
	ID                  uuid.UUID `json:"id"`
	OrganizationID      uuid.UUID `json:"organization_id"`
	Year                int       `json:"year"`
	Month               int       `json:"month"`
	TotalRequests       int       `json:"total_requests"`
	SuccessfulRequests  int       `json:"successful_requests"`
	FailedRequests      int       `json:"failed_requests"`
	JobsCreated         int       `json:"jobs_created"`
	JobsCompleted       int       `json:"jobs_completed"`
	JobsFailed          int       `json:"jobs_failed"`
	TestsGenerated      int       `json:"tests_generated"`
	TestsValidated      int       `json:"tests_validated"`
	TestsAccepted       int       `json:"tests_accepted"`
	AvgResponseTimeMs   int       `json:"avg_response_time_ms"`
	PeakDailyRequests   int       `json:"peak_daily_requests"`
	PeakConcurrentJobs  int       `json:"peak_concurrent_jobs"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// UsageSummary provides a summary of usage for an organization
type UsageSummary struct {
	OrganizationID     uuid.UUID `json:"organization_id"`
	Period             string    `json:"period"` // "today", "week", "month"
	TotalRequests      int       `json:"total_requests"`
	SuccessfulRequests int       `json:"successful_requests"`
	FailedRequests     int       `json:"failed_requests"`
	AvgResponseTimeMs  int       `json:"avg_response_time_ms"`
	JobsCreated        int       `json:"jobs_created"`
	JobsCompleted      int       `json:"jobs_completed"`
	TestsGenerated     int       `json:"tests_generated"`
}

// RecordAPIUsage records a single API request
func (s *Store) RecordAPIUsage(ctx context.Context, usage *APIUsage) error {
	if usage.ID == uuid.Nil {
		usage.ID = uuid.New()
	}
	if usage.CreatedAt.IsZero() {
		usage.CreatedAt = time.Now()
	}

	query := `
		INSERT INTO api_usage (id, organization_id, user_id, api_key_id, endpoint, method,
		                       status_code, response_time_ms, request_size_bytes, response_size_bytes,
		                       ip_address, user_agent, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	_, err := s.pool.Exec(ctx, query,
		usage.ID, usage.OrganizationID, usage.UserID, usage.APIKeyID,
		usage.Endpoint, usage.Method, usage.StatusCode, usage.ResponseTimeMs,
		usage.RequestSizeBytes, usage.ResponseSizeBytes,
		usage.IPAddress, usage.UserAgent, usage.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to record api usage: %w", err)
	}

	return nil
}

// GetUsageSummary gets usage summary for an organization
func (s *Store) GetUsageSummary(ctx context.Context, orgID uuid.UUID, period string) (*UsageSummary, error) {
	var startTime time.Time
	now := time.Now()

	switch period {
	case "today":
		startTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "week":
		startTime = now.AddDate(0, 0, -7)
	case "month":
		startTime = now.AddDate(0, -1, 0)
	default:
		startTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		period = "today"
	}

	query := `
		SELECT
			COUNT(*) as total_requests,
			COUNT(*) FILTER (WHERE status_code >= 200 AND status_code < 400) as successful_requests,
			COUNT(*) FILTER (WHERE status_code >= 400) as failed_requests,
			COALESCE(AVG(response_time_ms)::INTEGER, 0) as avg_response_time_ms
		FROM api_usage
		WHERE organization_id = $1 AND created_at >= $2
	`

	summary := &UsageSummary{
		OrganizationID: orgID,
		Period:         period,
	}

	err := s.pool.QueryRow(ctx, query, orgID, startTime).Scan(
		&summary.TotalRequests,
		&summary.SuccessfulRequests,
		&summary.FailedRequests,
		&summary.AvgResponseTimeMs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage summary: %w", err)
	}

	// Get job stats from jobs table if it exists
	jobQuery := `
		SELECT
			COUNT(*) as jobs_created,
			COUNT(*) FILTER (WHERE status = 'completed') as jobs_completed
		FROM jobs
		WHERE organization_id = $1 AND created_at >= $2
	`
	_ = s.pool.QueryRow(ctx, jobQuery, orgID, startTime).Scan(
		&summary.JobsCreated,
		&summary.JobsCompleted,
	)

	return summary, nil
}

// GetDailyStats gets daily usage stats for an organization
func (s *Store) GetDailyStats(ctx context.Context, orgID uuid.UUID, startDate, endDate time.Time) ([]UsageStatsDaily, error) {
	query := `
		SELECT id, organization_id, date, total_requests, successful_requests, failed_requests,
		       jobs_created, jobs_completed, jobs_failed, tests_generated, tests_validated, tests_accepted,
		       total_response_time_ms, total_request_bytes, total_response_bytes,
		       unique_users, unique_api_keys, created_at, updated_at
		FROM usage_stats_daily
		WHERE organization_id = $1 AND date >= $2 AND date <= $3
		ORDER BY date DESC
	`

	rows, err := s.pool.Query(ctx, query, orgID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily stats: %w", err)
	}
	defer rows.Close()

	var stats []UsageStatsDaily
	for rows.Next() {
		var s UsageStatsDaily
		err := rows.Scan(
			&s.ID, &s.OrganizationID, &s.Date,
			&s.TotalRequests, &s.SuccessfulRequests, &s.FailedRequests,
			&s.JobsCreated, &s.JobsCompleted, &s.JobsFailed,
			&s.TestsGenerated, &s.TestsValidated, &s.TestsAccepted,
			&s.TotalResponseTimeMs, &s.TotalRequestBytes, &s.TotalResponseBytes,
			&s.UniqueUsers, &s.UniqueAPIKeys, &s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan daily stats: %w", err)
		}
		stats = append(stats, s)
	}

	return stats, nil
}

// GetMonthlyStats gets monthly usage stats for an organization
func (s *Store) GetMonthlyStats(ctx context.Context, orgID uuid.UUID, year int) ([]UsageStatsMonthly, error) {
	query := `
		SELECT id, organization_id, year, month, total_requests, successful_requests, failed_requests,
		       jobs_created, jobs_completed, jobs_failed, tests_generated, tests_validated, tests_accepted,
		       avg_response_time_ms, peak_daily_requests, peak_concurrent_jobs, created_at, updated_at
		FROM usage_stats_monthly
		WHERE organization_id = $1 AND year = $2
		ORDER BY month DESC
	`

	rows, err := s.pool.Query(ctx, query, orgID, year)
	if err != nil {
		return nil, fmt.Errorf("failed to get monthly stats: %w", err)
	}
	defer rows.Close()

	var stats []UsageStatsMonthly
	for rows.Next() {
		var s UsageStatsMonthly
		err := rows.Scan(
			&s.ID, &s.OrganizationID, &s.Year, &s.Month,
			&s.TotalRequests, &s.SuccessfulRequests, &s.FailedRequests,
			&s.JobsCreated, &s.JobsCompleted, &s.JobsFailed,
			&s.TestsGenerated, &s.TestsValidated, &s.TestsAccepted,
			&s.AvgResponseTimeMs, &s.PeakDailyRequests, &s.PeakConcurrentJobs,
			&s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan monthly stats: %w", err)
		}
		stats = append(stats, s)
	}

	return stats, nil
}

// UpdateDailyStats upserts daily stats for an organization
func (s *Store) UpdateDailyStats(ctx context.Context, stats *UsageStatsDaily) error {
	query := `
		INSERT INTO usage_stats_daily (
			id, organization_id, date, total_requests, successful_requests, failed_requests,
			jobs_created, jobs_completed, jobs_failed, tests_generated, tests_validated, tests_accepted,
			total_response_time_ms, total_request_bytes, total_response_bytes,
			unique_users, unique_api_keys
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (organization_id, date) DO UPDATE SET
			total_requests = usage_stats_daily.total_requests + EXCLUDED.total_requests,
			successful_requests = usage_stats_daily.successful_requests + EXCLUDED.successful_requests,
			failed_requests = usage_stats_daily.failed_requests + EXCLUDED.failed_requests,
			jobs_created = usage_stats_daily.jobs_created + EXCLUDED.jobs_created,
			jobs_completed = usage_stats_daily.jobs_completed + EXCLUDED.jobs_completed,
			jobs_failed = usage_stats_daily.jobs_failed + EXCLUDED.jobs_failed,
			tests_generated = usage_stats_daily.tests_generated + EXCLUDED.tests_generated,
			tests_validated = usage_stats_daily.tests_validated + EXCLUDED.tests_validated,
			tests_accepted = usage_stats_daily.tests_accepted + EXCLUDED.tests_accepted,
			total_response_time_ms = usage_stats_daily.total_response_time_ms + EXCLUDED.total_response_time_ms,
			total_request_bytes = usage_stats_daily.total_request_bytes + EXCLUDED.total_request_bytes,
			total_response_bytes = usage_stats_daily.total_response_bytes + EXCLUDED.total_response_bytes
	`

	if stats.ID == uuid.Nil {
		stats.ID = uuid.New()
	}

	_, err := s.pool.Exec(ctx, query,
		stats.ID, stats.OrganizationID, stats.Date,
		stats.TotalRequests, stats.SuccessfulRequests, stats.FailedRequests,
		stats.JobsCreated, stats.JobsCompleted, stats.JobsFailed,
		stats.TestsGenerated, stats.TestsValidated, stats.TestsAccepted,
		stats.TotalResponseTimeMs, stats.TotalRequestBytes, stats.TotalResponseBytes,
		stats.UniqueUsers, stats.UniqueAPIKeys,
	)
	if err != nil {
		return fmt.Errorf("failed to update daily stats: %w", err)
	}

	return nil
}

// GetRecentAPIUsage gets recent API usage for an organization
func (s *Store) GetRecentAPIUsage(ctx context.Context, orgID uuid.UUID, limit int) ([]APIUsage, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, organization_id, user_id, api_key_id, endpoint, method,
		       status_code, response_time_ms, request_size_bytes, response_size_bytes,
		       ip_address, user_agent, created_at
		FROM api_usage
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := s.pool.Query(ctx, query, orgID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent api usage: %w", err)
	}
	defer rows.Close()

	var usage []APIUsage
	for rows.Next() {
		var u APIUsage
		err := rows.Scan(
			&u.ID, &u.OrganizationID, &u.UserID, &u.APIKeyID,
			&u.Endpoint, &u.Method, &u.StatusCode, &u.ResponseTimeMs,
			&u.RequestSizeBytes, &u.ResponseSizeBytes,
			&u.IPAddress, &u.UserAgent, &u.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan api usage: %w", err)
		}
		usage = append(usage, u)
	}

	return usage, nil
}

// GetEndpointStats gets stats grouped by endpoint
func (s *Store) GetEndpointStats(ctx context.Context, orgID uuid.UUID, startTime time.Time) ([]map[string]interface{}, error) {
	query := `
		SELECT endpoint, method,
		       COUNT(*) as request_count,
		       AVG(response_time_ms)::INTEGER as avg_response_time,
		       COUNT(*) FILTER (WHERE status_code >= 400) as error_count
		FROM api_usage
		WHERE organization_id = $1 AND created_at >= $2
		GROUP BY endpoint, method
		ORDER BY request_count DESC
		LIMIT 20
	`

	rows, err := s.pool.Query(ctx, query, orgID, startTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get endpoint stats: %w", err)
	}
	defer rows.Close()

	var stats []map[string]interface{}
	for rows.Next() {
		var endpoint, method string
		var requestCount, avgResponseTime, errorCount int

		err := rows.Scan(&endpoint, &method, &requestCount, &avgResponseTime, &errorCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan endpoint stats: %w", err)
		}

		stats = append(stats, map[string]interface{}{
			"endpoint":          endpoint,
			"method":            method,
			"request_count":     requestCount,
			"avg_response_time": avgResponseTime,
			"error_count":       errorCount,
		})
	}

	return stats, nil
}
