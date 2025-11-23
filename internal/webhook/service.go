package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/QTest-hq/qtest/internal/db"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Service handles webhook delivery
type Service struct {
	store      *db.Store
	httpClient *http.Client
}

// NewService creates a new webhook service
func NewService(store *db.Store) *Service {
	return &Service{
		store: store,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// TriggerEvent triggers a webhook event for an organization
func (s *Service) TriggerEvent(ctx context.Context, orgID uuid.UUID, eventType string, eventID uuid.UUID, data interface{}) error {
	// Create the event
	event, err := NewEvent(eventType, orgID, data)
	if err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}

	// Get all active webhooks for this event type
	webhooks, err := s.store.GetActiveWebhooksForEvent(ctx, orgID, eventType)
	if err != nil {
		return fmt.Errorf("failed to get webhooks: %w", err)
	}

	if len(webhooks) == 0 {
		log.Debug().
			Str("event_type", eventType).
			Str("org_id", orgID.String()).
			Msg("no webhooks registered for event")
		return nil
	}

	// Marshal the event payload
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Create delivery records for each webhook
	for _, webhook := range webhooks {
		delivery := &db.WebhookDelivery{
			ID:           uuid.New(),
			WebhookID:    webhook.ID,
			EventType:    eventType,
			EventID:      eventID,
			Payload:      payload,
			Status:       db.DeliveryStatusPending,
			AttemptCount: 0,
			CreatedAt:    time.Now(),
		}

		if err := s.store.CreateWebhookDelivery(ctx, delivery); err != nil {
			log.Error().Err(err).
				Str("webhook_id", webhook.ID.String()).
				Msg("failed to create delivery record")
			continue
		}

		// Update last triggered timestamp
		s.store.UpdateWebhookLastTriggered(ctx, webhook.ID)

		log.Info().
			Str("webhook_id", webhook.ID.String()).
			Str("event_type", eventType).
			Str("delivery_id", delivery.ID.String()).
			Msg("webhook delivery queued")
	}

	return nil
}

// DeliverWebhook attempts to deliver a webhook
func (s *Service) DeliverWebhook(ctx context.Context, delivery *db.WebhookDelivery, webhook *db.Webhook) error {
	startTime := time.Now()

	// Prepare request
	req, err := http.NewRequestWithContext(ctx, "POST", webhook.URL, bytes.NewReader(delivery.Payload))
	if err != nil {
		return s.handleDeliveryError(ctx, delivery, webhook, err)
	}

	// Set standard headers
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := SignPayload(delivery.Payload, webhook.Secret, timestamp)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "QTest-Webhooks/1.0")
	req.Header.Set("X-QTest-Signature", signature)
	req.Header.Set("X-QTest-Timestamp", timestamp)
	req.Header.Set("X-QTest-Event", delivery.EventType)
	req.Header.Set("X-QTest-Delivery", delivery.ID.String())

	// Add custom headers from webhook config
	if webhook.Headers != nil {
		var customHeaders map[string]string
		if err := json.Unmarshal(*webhook.Headers, &customHeaders); err == nil {
			for k, v := range customHeaders {
				req.Header.Set(k, v)
			}
		}
	}

	// Store request headers
	reqHeaders, _ := json.Marshal(req.Header)
	reqHeadersRaw := json.RawMessage(reqHeaders)
	delivery.RequestHeaders = &reqHeadersRaw

	// Make the request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return s.handleDeliveryError(ctx, delivery, webhook, err)
	}
	defer resp.Body.Close()

	// Read response
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024)) // Limit to 64KB
	bodyStr := string(body)
	delivery.ResponseBody = &bodyStr
	delivery.ResponseStatus = &resp.StatusCode

	respHeaders, _ := json.Marshal(resp.Header)
	respHeadersRaw := json.RawMessage(respHeaders)
	delivery.ResponseHeaders = &respHeadersRaw

	// Calculate duration
	durationMs := int(time.Since(startTime).Milliseconds())
	delivery.DurationMs = &durationMs

	// Check if successful (2xx status)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		delivery.Status = db.DeliveryStatusSuccess
		now := time.Now()
		delivery.DeliveredAt = &now
		delivery.AttemptCount++

		if err := s.store.UpdateWebhookDelivery(ctx, delivery); err != nil {
			log.Error().Err(err).Msg("failed to update successful delivery")
		}

		log.Info().
			Str("delivery_id", delivery.ID.String()).
			Int("status", resp.StatusCode).
			Int("duration_ms", durationMs).
			Msg("webhook delivered successfully")

		return nil
	}

	// Non-2xx status is treated as an error
	return s.handleDeliveryError(ctx, delivery, webhook,
		fmt.Errorf("webhook returned status %d", resp.StatusCode))
}

// handleDeliveryError handles a failed delivery attempt
func (s *Service) handleDeliveryError(ctx context.Context, delivery *db.WebhookDelivery, webhook *db.Webhook, err error) error {
	delivery.AttemptCount++
	errMsg := err.Error()
	delivery.ErrorMessage = &errMsg

	if delivery.AttemptCount >= webhook.MaxRetries {
		// Max retries reached - mark as failed
		delivery.Status = db.DeliveryStatusFailed
		log.Warn().
			Str("delivery_id", delivery.ID.String()).
			Int("attempts", delivery.AttemptCount).
			Str("error", errMsg).
			Msg("webhook delivery failed permanently")
	} else {
		// Schedule retry with exponential backoff
		delivery.Status = db.DeliveryStatusRetrying
		backoff := CalculateBackoff(delivery.AttemptCount)
		nextRetry := time.Now().Add(backoff)
		delivery.NextRetryAt = &nextRetry

		log.Info().
			Str("delivery_id", delivery.ID.String()).
			Int("attempt", delivery.AttemptCount).
			Time("next_retry", nextRetry).
			Str("error", errMsg).
			Msg("webhook delivery will be retried")
	}

	if updateErr := s.store.UpdateWebhookDelivery(ctx, delivery); updateErr != nil {
		log.Error().Err(updateErr).Msg("failed to update failed delivery")
	}

	return err
}

// ProcessPendingDeliveries processes pending webhook deliveries
func (s *Service) ProcessPendingDeliveries(ctx context.Context, limit int) (int, error) {
	deliveries, err := s.store.GetPendingDeliveries(ctx, limit)
	if err != nil {
		return 0, err
	}

	processed := 0
	for _, delivery := range deliveries {
		// Get the webhook
		webhook, err := s.store.GetWebhookByID(ctx, delivery.WebhookID)
		if err != nil || webhook == nil {
			log.Error().Err(err).
				Str("webhook_id", delivery.WebhookID.String()).
				Msg("webhook not found for delivery")
			continue
		}

		// Attempt delivery
		if err := s.DeliverWebhook(ctx, &delivery, webhook); err != nil {
			log.Debug().Err(err).
				Str("delivery_id", delivery.ID.String()).
				Msg("webhook delivery attempt failed")
		}
		processed++
	}

	return processed, nil
}

// SignPayload creates an HMAC-SHA256 signature
func SignPayload(payload []byte, secret, timestamp string) string {
	// Signature covers timestamp + payload
	signatureData := timestamp + "." + string(payload)

	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(signatureData))
	signature := hex.EncodeToString(h.Sum(nil))

	return "sha256=" + signature
}

// VerifySignature verifies a webhook signature
func VerifySignature(payload []byte, secret, timestamp, signature string) bool {
	expected := SignPayload(payload, secret, timestamp)
	return hmac.Equal([]byte(signature), []byte(expected))
}

// CalculateBackoff calculates exponential backoff with jitter
func CalculateBackoff(attemptCount int) time.Duration {
	// Base: 2 seconds, exponential: 2^attempt
	// Attempt 1: ~2s, 2: ~4s, 3: ~8s, 4: ~16s, 5: ~32s
	baseSeconds := 2.0
	backoff := math.Pow(baseSeconds, float64(attemptCount))

	// Add jitter (0-25% of backoff)
	jitter := rand.Float64() * 0.25 * backoff

	// Cap at 1 hour
	total := time.Duration((backoff+jitter)*1000) * time.Millisecond
	maxBackoff := time.Hour
	if total > maxBackoff {
		total = maxBackoff
	}

	return total
}

// SendTestEvent sends a test webhook to verify configuration
func (s *Service) SendTestEvent(ctx context.Context, webhook *db.Webhook) error {
	testData := map[string]interface{}{
		"message": "This is a test webhook from QTest",
		"webhook": map[string]interface{}{
			"id":   webhook.ID.String(),
			"name": webhook.Name,
		},
	}

	event, err := NewEvent("test", webhook.OrganizationID, testData)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	delivery := &db.WebhookDelivery{
		ID:           uuid.New(),
		WebhookID:    webhook.ID,
		EventType:    "test",
		EventID:      uuid.New(),
		Payload:      payload,
		Status:       db.DeliveryStatusPending,
		AttemptCount: 0,
		CreatedAt:    time.Now(),
	}

	if err := s.store.CreateWebhookDelivery(ctx, delivery); err != nil {
		return err
	}

	return s.DeliverWebhook(ctx, delivery, webhook)
}
