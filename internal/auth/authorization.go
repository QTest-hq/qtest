// Package auth provides authorization functionality for resource access control
package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

var (
	ErrUnauthorized     = errors.New("unauthorized access")
	ErrForbidden        = errors.New("forbidden: insufficient permissions")
	ErrResourceNotFound = errors.New("resource not found")
)

// ResourceType represents the type of resource being accessed
type ResourceType string

const (
	ResourceTypeRepository ResourceType = "repository"
	ResourceTypeJob        ResourceType = "job"
	ResourceTypeTest       ResourceType = "test"
	ResourceTypeWebhook    ResourceType = "webhook"
	ResourceTypeAPIKey     ResourceType = "api_key"
	ResourceTypeUser       ResourceType = "user"
)

// Action represents the type of action being performed
type Action string

const (
	ActionRead   Action = "read"
	ActionWrite  Action = "write"
	ActionDelete Action = "delete"
	ActionAdmin  Action = "admin"
)

// Role represents an organization member's role
type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"
)

// AuthzRequest represents an authorization request
type AuthzRequest struct {
	UserID         uuid.UUID
	OrganizationID uuid.UUID
	ResourceType   ResourceType
	ResourceID     uuid.UUID
	Action         Action
}

// Permissions represents the permissions for a role
type Permissions struct {
	CanRead   bool
	CanWrite  bool
	CanDelete bool
	CanAdmin  bool
}

// rolePermissions maps roles to their permissions
var rolePermissions = map[Role]Permissions{
	RoleOwner:  {CanRead: true, CanWrite: true, CanDelete: true, CanAdmin: true},
	RoleAdmin:  {CanRead: true, CanWrite: true, CanDelete: true, CanAdmin: false},
	RoleMember: {CanRead: true, CanWrite: true, CanDelete: false, CanAdmin: false},
	RoleViewer: {CanRead: true, CanWrite: false, CanDelete: false, CanAdmin: false},
}

// Can checks if the permissions allow the specified action
func (p Permissions) Can(action Action) bool {
	switch action {
	case ActionRead:
		return p.CanRead
	case ActionWrite:
		return p.CanWrite
	case ActionDelete:
		return p.CanDelete
	case ActionAdmin:
		return p.CanAdmin
	default:
		return false
	}
}

// AuthorizationService handles resource-level authorization
type AuthorizationService struct {
	pool *pgxpool.Pool
}

// NewAuthorizationService creates a new authorization service
func NewAuthorizationService(pool *pgxpool.Pool) *AuthorizationService {
	return &AuthorizationService{pool: pool}
}

// Can checks if a user can perform an action on a resource
func (s *AuthorizationService) Can(ctx context.Context, req AuthzRequest) (bool, error) {
	// 1. Get user's role in the organization
	role, err := s.getUserRole(ctx, req.UserID, req.OrganizationID)
	if err != nil {
		return false, err
	}

	// 2. Check if role allows the action
	perms, ok := rolePermissions[role]
	if !ok {
		return false, nil
	}

	// 3. Check basic permission
	if !perms.Can(req.Action) {
		return false, nil
	}

	// 4. Verify resource belongs to organization (if resource ID provided)
	if req.ResourceID != uuid.Nil {
		belongs, err := s.resourceBelongsToOrg(ctx, req.ResourceType, req.ResourceID, req.OrganizationID)
		if err != nil {
			return false, err
		}
		if !belongs {
			return false, nil
		}
	}

	return true, nil
}

// Require is like Can but returns an error if authorization fails
func (s *AuthorizationService) Require(ctx context.Context, req AuthzRequest) error {
	allowed, err := s.Can(ctx, req)
	if err != nil {
		log.Error().Err(err).
			Str("user_id", req.UserID.String()).
			Str("org_id", req.OrganizationID.String()).
			Str("resource_type", string(req.ResourceType)).
			Str("action", string(req.Action)).
			Msg("authorization check failed")
		return ErrUnauthorized
	}

	if !allowed {
		log.Warn().
			Str("user_id", req.UserID.String()).
			Str("org_id", req.OrganizationID.String()).
			Str("resource_type", string(req.ResourceType)).
			Str("action", string(req.Action)).
			Msg("authorization denied")
		return ErrForbidden
	}

	return nil
}

// getUserRole gets a user's role in an organization
func (s *AuthorizationService) getUserRole(ctx context.Context, userID, orgID uuid.UUID) (Role, error) {
	var role string
	err := s.pool.QueryRow(ctx, `
		SELECT role FROM organization_members
		WHERE organization_id = $1 AND user_id = $2
	`, orgID, userID).Scan(&role)

	if err != nil {
		return "", err
	}

	return Role(role), nil
}

// resourceBelongsToOrg checks if a resource belongs to an organization
func (s *AuthorizationService) resourceBelongsToOrg(ctx context.Context, resourceType ResourceType, resourceID, orgID uuid.UUID) (bool, error) {
	var query string
	switch resourceType {
	case ResourceTypeRepository:
		query = `SELECT EXISTS(SELECT 1 FROM repositories WHERE id = $1 AND organization_id = $2)`
	case ResourceTypeJob:
		query = `SELECT EXISTS(SELECT 1 FROM jobs WHERE id = $1 AND organization_id = $2)`
	case ResourceTypeTest:
		query = `SELECT EXISTS(SELECT 1 FROM generated_tests t JOIN generation_runs r ON t.run_id = r.id WHERE t.id = $1 AND r.organization_id = $2)`
	case ResourceTypeWebhook:
		query = `SELECT EXISTS(SELECT 1 FROM webhooks WHERE id = $1 AND organization_id = $2)`
	case ResourceTypeAPIKey:
		query = `SELECT EXISTS(SELECT 1 FROM api_keys WHERE id = $1 AND organization_id = $2)`
	default:
		return false, nil
	}

	var exists bool
	err := s.pool.QueryRow(ctx, query, resourceID, orgID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// GetUserOrganizations returns all organizations a user belongs to
func (s *AuthorizationService) GetUserOrganizations(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT organization_id FROM organization_members WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgs []uuid.UUID
	for rows.Next() {
		var orgID uuid.UUID
		if err := rows.Scan(&orgID); err != nil {
			return nil, err
		}
		orgs = append(orgs, orgID)
	}

	return orgs, rows.Err()
}

// HasOrgAccess checks if a user has any access to an organization
func (s *AuthorizationService) HasOrgAccess(ctx context.Context, userID, orgID uuid.UUID) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM organization_members WHERE organization_id = $1 AND user_id = $2)
	`, orgID, userID).Scan(&exists)
	return exists, err
}

// GetUserRoleInOrg returns the user's role in an organization (public wrapper)
func (s *AuthorizationService) GetUserRoleInOrg(ctx context.Context, userID, orgID uuid.UUID) (string, error) {
	role, err := s.getUserRole(ctx, userID, orgID)
	return string(role), err
}
