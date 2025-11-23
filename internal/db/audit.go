package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AuditLog represents an audit log entry
type AuditLog struct {
	ID             uuid.UUID        `json:"id"`
	OrganizationID *uuid.UUID       `json:"organization_id,omitempty"`
	UserID         *uuid.UUID       `json:"user_id,omitempty"`
	Action         string           `json:"action"`
	ResourceType   string           `json:"resource_type"`
	ResourceID     *uuid.UUID       `json:"resource_id,omitempty"`
	Details        *json.RawMessage `json:"details,omitempty"`
	IPAddress      *string          `json:"ip_address,omitempty"`
	UserAgent      *string          `json:"user_agent,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
}

// AuditAction represents the type of action being logged
type AuditAction string

const (
	// Authentication actions
	AuditActionLogin  AuditAction = "login"
	AuditActionLogout AuditAction = "logout"

	// Organization actions
	AuditActionOrgCreate       AuditAction = "org.create"
	AuditActionOrgUpdate       AuditAction = "org.update"
	AuditActionOrgDelete       AuditAction = "org.delete"
	AuditActionMemberAdd       AuditAction = "member.add"
	AuditActionMemberRemove    AuditAction = "member.remove"
	AuditActionMemberRoleChange AuditAction = "member.role_change"

	// Repository actions
	AuditActionRepoCreate AuditAction = "repo.create"
	AuditActionRepoDelete AuditAction = "repo.delete"

	// API key actions
	AuditActionAPIKeyCreate AuditAction = "api_key.create"
	AuditActionAPIKeyRevoke AuditAction = "api_key.revoke"

	// Test generation actions
	AuditActionRunCreate AuditAction = "run.create"
	AuditActionTestAccept AuditAction = "test.accept"
	AuditActionTestReject AuditAction = "test.reject"
)

// AuditResourceType represents the type of resource being audited
type AuditResourceType string

const (
	ResourceTypeUser         AuditResourceType = "user"
	ResourceTypeOrganization AuditResourceType = "organization"
	ResourceTypeMember       AuditResourceType = "member"
	ResourceTypeRepository   AuditResourceType = "repository"
	ResourceTypeAPIKey       AuditResourceType = "api_key"
	ResourceTypeRun          AuditResourceType = "run"
	ResourceTypeTest         AuditResourceType = "test"
)

// CreateAuditLog creates a new audit log entry
func (s *Store) CreateAuditLog(ctx context.Context, log *AuditLog) error {
	log.ID = uuid.New()
	log.CreatedAt = time.Now()

	_, err := s.pool.Exec(ctx, `
		INSERT INTO audit_logs (id, organization_id, user_id, action, resource_type, resource_id, details, ip_address, user_agent, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, log.ID, log.OrganizationID, log.UserID, log.Action, log.ResourceType, log.ResourceID, log.Details, log.IPAddress, log.UserAgent, log.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	return nil
}

// LogAuditEvent is a convenience method to log an audit event
func (s *Store) LogAuditEvent(ctx context.Context, orgID, userID *uuid.UUID, action AuditAction, resourceType AuditResourceType, resourceID *uuid.UUID, details interface{}, ipAddress, userAgent string) error {
	var detailsJSON *json.RawMessage
	if details != nil {
		data, err := json.Marshal(details)
		if err != nil {
			return fmt.Errorf("failed to marshal details: %w", err)
		}
		raw := json.RawMessage(data)
		detailsJSON = &raw
	}

	var ip, ua *string
	if ipAddress != "" {
		ip = &ipAddress
	}
	if userAgent != "" {
		ua = &userAgent
	}

	log := &AuditLog{
		OrganizationID: orgID,
		UserID:         userID,
		Action:         string(action),
		ResourceType:   string(resourceType),
		ResourceID:     resourceID,
		Details:        detailsJSON,
		IPAddress:      ip,
		UserAgent:      ua,
	}

	return s.CreateAuditLog(ctx, log)
}

// ListAuditLogs lists audit logs for an organization
func (s *Store) ListAuditLogs(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]AuditLog, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, organization_id, user_id, action, resource_type, resource_id, details, ip_address, user_agent, created_at
		FROM audit_logs
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit logs: %w", err)
	}
	defer rows.Close()

	logs := make([]AuditLog, 0)
	for rows.Next() {
		var log AuditLog
		err := rows.Scan(&log.ID, &log.OrganizationID, &log.UserID, &log.Action,
			&log.ResourceType, &log.ResourceID, &log.Details, &log.IPAddress,
			&log.UserAgent, &log.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit log: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// ListAuditLogsByUser lists audit logs for a specific user
func (s *Store) ListAuditLogsByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]AuditLog, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, organization_id, user_id, action, resource_type, resource_id, details, ip_address, user_agent, created_at
		FROM audit_logs
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit logs: %w", err)
	}
	defer rows.Close()

	logs := make([]AuditLog, 0)
	for rows.Next() {
		var log AuditLog
		err := rows.Scan(&log.ID, &log.OrganizationID, &log.UserID, &log.Action,
			&log.ResourceType, &log.ResourceID, &log.Details, &log.IPAddress,
			&log.UserAgent, &log.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit log: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// ListAuditLogsByResource lists audit logs for a specific resource
func (s *Store) ListAuditLogsByResource(ctx context.Context, resourceType AuditResourceType, resourceID uuid.UUID, limit, offset int) ([]AuditLog, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, organization_id, user_id, action, resource_type, resource_id, details, ip_address, user_agent, created_at
		FROM audit_logs
		WHERE resource_type = $1 AND resource_id = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`, string(resourceType), resourceID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit logs: %w", err)
	}
	defer rows.Close()

	logs := make([]AuditLog, 0)
	for rows.Next() {
		var log AuditLog
		err := rows.Scan(&log.ID, &log.OrganizationID, &log.UserID, &log.Action,
			&log.ResourceType, &log.ResourceID, &log.Details, &log.IPAddress,
			&log.UserAgent, &log.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit log: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}
