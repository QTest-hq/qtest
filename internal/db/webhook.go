package db

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lib/pq"
)

// Webhook represents a webhook configuration
type Webhook struct {
	ID              uuid.UUID        `json:"id"`
	OrganizationID  uuid.UUID        `json:"organization_id"`
	CreatedBy       uuid.UUID        `json:"created_by"`
	Name            string           `json:"name"`
	URL             string           `json:"url"`
	Secret          string           `json:"-"` // Never expose in API responses
	Events          []string         `json:"events"`
	IsActive        bool             `json:"is_active"`
	MaxRetries      int              `json:"max_retries"`
	TimeoutSeconds  int              `json:"timeout_seconds"`
	Description     *string          `json:"description,omitempty"`
	Headers         *json.RawMessage `json:"headers,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	LastTriggeredAt *time.Time       `json:"last_triggered_at,omitempty"`
}

// WebhookWithSecret is returned when creating a new webhook
type WebhookWithSecret struct {
	Webhook
	Secret string `json:"secret"` // Only returned on creation
}

// WebhookDelivery represents a webhook delivery attempt
type WebhookDelivery struct {
	ID              uuid.UUID        `json:"id"`
	WebhookID       uuid.UUID        `json:"webhook_id"`
	EventType       string           `json:"event_type"`
	EventID         uuid.UUID        `json:"event_id"`
	Payload         json.RawMessage  `json:"payload"`
	RequestHeaders  *json.RawMessage `json:"request_headers,omitempty"`
	ResponseStatus  *int             `json:"response_status,omitempty"`
	ResponseBody    *string          `json:"response_body,omitempty"`
	ResponseHeaders *json.RawMessage `json:"response_headers,omitempty"`
	Status          string           `json:"status"`
	AttemptCount    int              `json:"attempt_count"`
	NextRetryAt     *time.Time       `json:"next_retry_at,omitempty"`
	ErrorMessage    *string          `json:"error_message,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	DeliveredAt     *time.Time       `json:"delivered_at,omitempty"`
	DurationMs      *int             `json:"duration_ms,omitempty"`
}

// WebhookDeliveryStatus constants
const (
	DeliveryStatusPending  = "pending"
	DeliveryStatusSuccess  = "success"
	DeliveryStatusFailed   = "failed"
	DeliveryStatusRetrying = "retrying"
)

// GenerateWebhookSecret generates a random secret for webhook signing
func GenerateWebhookSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return "whsec_" + hex.EncodeToString(b), nil
}

// CreateWebhook creates a new webhook
func (s *Store) CreateWebhook(ctx context.Context, orgID, userID uuid.UUID, name, url string, events []string, description *string, headers *json.RawMessage) (*WebhookWithSecret, error) {
	secret, err := GenerateWebhookSecret()
	if err != nil {
		return nil, err
	}

	webhook := &WebhookWithSecret{
		Webhook: Webhook{
			ID:             uuid.New(),
			OrganizationID: orgID,
			CreatedBy:      userID,
			Name:           name,
			URL:            url,
			Events:         events,
			IsActive:       true,
			MaxRetries:     5,
			TimeoutSeconds: 30,
			Description:    description,
			Headers:        headers,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		Secret: secret,
	}

	query := `
		INSERT INTO webhooks (id, organization_id, created_by, name, url, secret, events, is_active, max_retries, timeout_seconds, description, headers, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err = s.pool.Exec(ctx, query,
		webhook.ID, webhook.OrganizationID, webhook.CreatedBy,
		webhook.Name, webhook.URL, secret,
		pq.Array(webhook.Events), webhook.IsActive,
		webhook.MaxRetries, webhook.TimeoutSeconds,
		webhook.Description, webhook.Headers,
		webhook.CreatedAt, webhook.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook: %w", err)
	}

	return webhook, nil
}

// GetWebhookByID retrieves a webhook by ID
func (s *Store) GetWebhookByID(ctx context.Context, id uuid.UUID) (*Webhook, error) {
	query := `
		SELECT id, organization_id, created_by, name, url, secret, events, is_active,
		       max_retries, timeout_seconds, description, headers,
		       created_at, updated_at, last_triggered_at
		FROM webhooks
		WHERE id = $1
	`

	webhook := &Webhook{}
	var events []string
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&webhook.ID, &webhook.OrganizationID, &webhook.CreatedBy,
		&webhook.Name, &webhook.URL, &webhook.Secret,
		&events, &webhook.IsActive,
		&webhook.MaxRetries, &webhook.TimeoutSeconds,
		&webhook.Description, &webhook.Headers,
		&webhook.CreatedAt, &webhook.UpdatedAt, &webhook.LastTriggeredAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get webhook: %w", err)
	}

	webhook.Events = events
	return webhook, nil
}

// ListWebhooksByOrg lists all webhooks for an organization
func (s *Store) ListWebhooksByOrg(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]Webhook, error) {
	query := `
		SELECT id, organization_id, created_by, name, url, events, is_active,
		       max_retries, timeout_seconds, description, headers,
		       created_at, updated_at, last_triggered_at
		FROM webhooks
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := s.pool.Query(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list webhooks: %w", err)
	}
	defer rows.Close()

	var webhooks []Webhook
	for rows.Next() {
		var w Webhook
		var events []string
		err := rows.Scan(
			&w.ID, &w.OrganizationID, &w.CreatedBy,
			&w.Name, &w.URL, &events, &w.IsActive,
			&w.MaxRetries, &w.TimeoutSeconds,
			&w.Description, &w.Headers,
			&w.CreatedAt, &w.UpdatedAt, &w.LastTriggeredAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan webhook: %w", err)
		}
		w.Events = events
		webhooks = append(webhooks, w)
	}

	return webhooks, nil
}

// UpdateWebhook updates a webhook
func (s *Store) UpdateWebhook(ctx context.Context, id uuid.UUID, name, url *string, events []string, isActive *bool, description *string, headers *json.RawMessage) error {
	query := `
		UPDATE webhooks SET
			name = COALESCE($2, name),
			url = COALESCE($3, url),
			events = COALESCE($4, events),
			is_active = COALESCE($5, is_active),
			description = COALESCE($6, description),
			headers = COALESCE($7, headers),
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`

	var eventsArr interface{}
	if events != nil {
		eventsArr = pq.Array(events)
	}

	_, err := s.pool.Exec(ctx, query, id, name, url, eventsArr, isActive, description, headers)
	if err != nil {
		return fmt.Errorf("failed to update webhook: %w", err)
	}

	return nil
}

// DeleteWebhook deletes a webhook
func (s *Store) DeleteWebhook(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM webhooks WHERE id = $1`
	_, err := s.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete webhook: %w", err)
	}
	return nil
}

// GetActiveWebhooksForEvent retrieves all active webhooks for an org that subscribe to an event type
func (s *Store) GetActiveWebhooksForEvent(ctx context.Context, orgID uuid.UUID, eventType string) ([]Webhook, error) {
	query := `
		SELECT id, organization_id, created_by, name, url, secret, events, is_active,
		       max_retries, timeout_seconds, description, headers,
		       created_at, updated_at, last_triggered_at
		FROM webhooks
		WHERE organization_id = $1
		  AND is_active = true
		  AND $2 = ANY(events)
	`

	rows, err := s.pool.Query(ctx, query, orgID, eventType)
	if err != nil {
		return nil, fmt.Errorf("failed to get webhooks for event: %w", err)
	}
	defer rows.Close()

	var webhooks []Webhook
	for rows.Next() {
		var w Webhook
		var events []string
		err := rows.Scan(
			&w.ID, &w.OrganizationID, &w.CreatedBy,
			&w.Name, &w.URL, &w.Secret, &events, &w.IsActive,
			&w.MaxRetries, &w.TimeoutSeconds,
			&w.Description, &w.Headers,
			&w.CreatedAt, &w.UpdatedAt, &w.LastTriggeredAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan webhook: %w", err)
		}
		w.Events = events
		webhooks = append(webhooks, w)
	}

	return webhooks, nil
}

// UpdateWebhookLastTriggered updates the last_triggered_at timestamp
func (s *Store) UpdateWebhookLastTriggered(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE webhooks SET last_triggered_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err := s.pool.Exec(ctx, query, id)
	return err
}

// CreateWebhookDelivery creates a new webhook delivery record
func (s *Store) CreateWebhookDelivery(ctx context.Context, delivery *WebhookDelivery) error {
	query := `
		INSERT INTO webhook_deliveries (id, webhook_id, event_type, event_id, payload, status, attempt_count, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	if delivery.ID == uuid.Nil {
		delivery.ID = uuid.New()
	}
	if delivery.CreatedAt.IsZero() {
		delivery.CreatedAt = time.Now()
	}

	_, err := s.pool.Exec(ctx, query,
		delivery.ID, delivery.WebhookID, delivery.EventType, delivery.EventID,
		delivery.Payload, delivery.Status, delivery.AttemptCount, delivery.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create webhook delivery: %w", err)
	}

	return nil
}

// UpdateWebhookDelivery updates a webhook delivery record
func (s *Store) UpdateWebhookDelivery(ctx context.Context, delivery *WebhookDelivery) error {
	query := `
		UPDATE webhook_deliveries SET
			request_headers = $2,
			response_status = $3,
			response_body = $4,
			response_headers = $5,
			status = $6,
			attempt_count = $7,
			next_retry_at = $8,
			error_message = $9,
			delivered_at = $10,
			duration_ms = $11
		WHERE id = $1
	`

	_, err := s.pool.Exec(ctx, query,
		delivery.ID,
		delivery.RequestHeaders, delivery.ResponseStatus,
		delivery.ResponseBody, delivery.ResponseHeaders,
		delivery.Status, delivery.AttemptCount,
		delivery.NextRetryAt, delivery.ErrorMessage,
		delivery.DeliveredAt, delivery.DurationMs,
	)
	if err != nil {
		return fmt.Errorf("failed to update webhook delivery: %w", err)
	}

	return nil
}

// GetPendingDeliveries retrieves pending webhook deliveries ready for processing
func (s *Store) GetPendingDeliveries(ctx context.Context, limit int) ([]WebhookDelivery, error) {
	query := `
		SELECT d.id, d.webhook_id, d.event_type, d.event_id, d.payload,
		       d.request_headers, d.response_status, d.response_body, d.response_headers,
		       d.status, d.attempt_count, d.next_retry_at, d.error_message,
		       d.created_at, d.delivered_at, d.duration_ms
		FROM webhook_deliveries d
		JOIN webhooks w ON d.webhook_id = w.id
		WHERE w.is_active = true
		  AND d.status IN ('pending', 'retrying')
		  AND (d.next_retry_at IS NULL OR d.next_retry_at <= CURRENT_TIMESTAMP)
		ORDER BY d.created_at ASC
		LIMIT $1
	`

	rows, err := s.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending deliveries: %w", err)
	}
	defer rows.Close()

	var deliveries []WebhookDelivery
	for rows.Next() {
		var d WebhookDelivery
		err := rows.Scan(
			&d.ID, &d.WebhookID, &d.EventType, &d.EventID, &d.Payload,
			&d.RequestHeaders, &d.ResponseStatus, &d.ResponseBody, &d.ResponseHeaders,
			&d.Status, &d.AttemptCount, &d.NextRetryAt, &d.ErrorMessage,
			&d.CreatedAt, &d.DeliveredAt, &d.DurationMs,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan delivery: %w", err)
		}
		deliveries = append(deliveries, d)
	}

	return deliveries, nil
}

// ListDeliveriesByWebhook lists deliveries for a specific webhook
func (s *Store) ListDeliveriesByWebhook(ctx context.Context, webhookID uuid.UUID, limit, offset int) ([]WebhookDelivery, error) {
	query := `
		SELECT id, webhook_id, event_type, event_id, payload,
		       request_headers, response_status, response_body, response_headers,
		       status, attempt_count, next_retry_at, error_message,
		       created_at, delivered_at, duration_ms
		FROM webhook_deliveries
		WHERE webhook_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := s.pool.Query(ctx, query, webhookID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list deliveries: %w", err)
	}
	defer rows.Close()

	var deliveries []WebhookDelivery
	for rows.Next() {
		var d WebhookDelivery
		err := rows.Scan(
			&d.ID, &d.WebhookID, &d.EventType, &d.EventID, &d.Payload,
			&d.RequestHeaders, &d.ResponseStatus, &d.ResponseBody, &d.ResponseHeaders,
			&d.Status, &d.AttemptCount, &d.NextRetryAt, &d.ErrorMessage,
			&d.CreatedAt, &d.DeliveredAt, &d.DurationMs,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan delivery: %w", err)
		}
		deliveries = append(deliveries, d)
	}

	return deliveries, nil
}

// GetDeliveryByID retrieves a specific delivery
func (s *Store) GetDeliveryByID(ctx context.Context, id uuid.UUID) (*WebhookDelivery, error) {
	query := `
		SELECT id, webhook_id, event_type, event_id, payload,
		       request_headers, response_status, response_body, response_headers,
		       status, attempt_count, next_retry_at, error_message,
		       created_at, delivered_at, duration_ms
		FROM webhook_deliveries
		WHERE id = $1
	`

	d := &WebhookDelivery{}
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&d.ID, &d.WebhookID, &d.EventType, &d.EventID, &d.Payload,
		&d.RequestHeaders, &d.ResponseStatus, &d.ResponseBody, &d.ResponseHeaders,
		&d.Status, &d.AttemptCount, &d.NextRetryAt, &d.ErrorMessage,
		&d.CreatedAt, &d.DeliveredAt, &d.DurationMs,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get delivery: %w", err)
	}

	return d, nil
}
