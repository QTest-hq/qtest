package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// MemberRole represents a member's role in an organization
type MemberRole string

const (
	RoleOwner  MemberRole = "owner"
	RoleAdmin  MemberRole = "admin"
	RoleMember MemberRole = "member"
	RoleViewer MemberRole = "viewer"
)

// Organization represents a tenant organization
type Organization struct {
	ID          uuid.UUID       `json:"id"`
	Name        string          `json:"name"`
	Slug        string          `json:"slug"`
	Description *string         `json:"description,omitempty"`
	OwnerID     uuid.UUID       `json:"owner_id"`
	GitHubOrgID *int64          `json:"github_org_id,omitempty"`
	Settings    json.RawMessage `json:"settings"`
	IsPersonal  bool            `json:"is_personal"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// OrganizationMember represents a user's membership in an organization
type OrganizationMember struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	UserID         uuid.UUID  `json:"user_id"`
	Role           MemberRole `json:"role"`
	InvitedBy      *uuid.UUID `json:"invited_by,omitempty"`
	JoinedAt       time.Time  `json:"joined_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

// OrganizationWithRole combines org info with user's role
type OrganizationWithRole struct {
	Organization
	Role MemberRole `json:"role"`
}

// CreateOrganization creates a new organization
func (s *Store) CreateOrganization(ctx context.Context, org *Organization) error {
	org.ID = uuid.New()
	if org.Settings == nil {
		org.Settings = json.RawMessage(`{}`)
	}
	org.CreatedAt = time.Now()
	org.UpdatedAt = time.Now()

	_, err := s.pool.Exec(ctx, `
		INSERT INTO organizations (id, name, slug, description, owner_id, github_org_id, settings, is_personal, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, org.ID, org.Name, org.Slug, org.Description, org.OwnerID, org.GitHubOrgID, org.Settings, org.IsPersonal, org.CreatedAt, org.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create organization: %w", err)
	}

	// Add owner as member
	_, err = s.pool.Exec(ctx, `
		INSERT INTO organization_members (id, organization_id, user_id, role, joined_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, uuid.New(), org.ID, org.OwnerID, RoleOwner, time.Now(), time.Now())

	if err != nil {
		return fmt.Errorf("failed to add owner as member: %w", err)
	}

	return nil
}

// GetOrganizationByID retrieves an organization by ID
func (s *Store) GetOrganizationByID(ctx context.Context, id uuid.UUID) (*Organization, error) {
	org := &Organization{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, slug, description, owner_id, github_org_id, settings, is_personal, created_at, updated_at
		FROM organizations
		WHERE id = $1
	`, id).Scan(&org.ID, &org.Name, &org.Slug, &org.Description, &org.OwnerID, &org.GitHubOrgID, &org.Settings, &org.IsPersonal, &org.CreatedAt, &org.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	return org, nil
}

// GetOrganizationBySlug retrieves an organization by slug
func (s *Store) GetOrganizationBySlug(ctx context.Context, slug string) (*Organization, error) {
	org := &Organization{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, slug, description, owner_id, github_org_id, settings, is_personal, created_at, updated_at
		FROM organizations
		WHERE slug = $1
	`, slug).Scan(&org.ID, &org.Name, &org.Slug, &org.Description, &org.OwnerID, &org.GitHubOrgID, &org.Settings, &org.IsPersonal, &org.CreatedAt, &org.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get organization by slug: %w", err)
	}

	return org, nil
}

// GetPersonalOrganization retrieves a user's personal organization
func (s *Store) GetPersonalOrganization(ctx context.Context, userID uuid.UUID) (*Organization, error) {
	org := &Organization{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, slug, description, owner_id, github_org_id, settings, is_personal, created_at, updated_at
		FROM organizations
		WHERE owner_id = $1 AND is_personal = true
	`, userID).Scan(&org.ID, &org.Name, &org.Slug, &org.Description, &org.OwnerID, &org.GitHubOrgID, &org.Settings, &org.IsPersonal, &org.CreatedAt, &org.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get personal organization: %w", err)
	}

	return org, nil
}

// ListUserOrganizations returns all organizations a user belongs to
func (s *Store) ListUserOrganizations(ctx context.Context, userID uuid.UUID) ([]OrganizationWithRole, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT o.id, o.name, o.slug, o.description, o.owner_id, o.github_org_id, o.settings, o.is_personal, o.created_at, o.updated_at, m.role
		FROM organizations o
		JOIN organization_members m ON o.id = m.organization_id
		WHERE m.user_id = $1
		ORDER BY o.is_personal DESC, o.name
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list organizations: %w", err)
	}
	defer rows.Close()

	var orgs []OrganizationWithRole
	for rows.Next() {
		var org OrganizationWithRole
		err := rows.Scan(&org.ID, &org.Name, &org.Slug, &org.Description, &org.OwnerID, &org.GitHubOrgID, &org.Settings, &org.IsPersonal, &org.CreatedAt, &org.UpdatedAt, &org.Role)
		if err != nil {
			return nil, fmt.Errorf("failed to scan organization: %w", err)
		}
		orgs = append(orgs, org)
	}

	return orgs, nil
}

// UpdateOrganization updates an organization
func (s *Store) UpdateOrganization(ctx context.Context, org *Organization) error {
	org.UpdatedAt = time.Now()

	_, err := s.pool.Exec(ctx, `
		UPDATE organizations
		SET name = $2, description = $3, settings = $4, updated_at = $5
		WHERE id = $1
	`, org.ID, org.Name, org.Description, org.Settings, org.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update organization: %w", err)
	}

	return nil
}

// DeleteOrganization deletes an organization (cascade deletes members)
func (s *Store) DeleteOrganization(ctx context.Context, id uuid.UUID) error {
	// Prevent deleting personal orgs
	org, err := s.GetOrganizationByID(ctx, id)
	if err != nil {
		return err
	}
	if org != nil && org.IsPersonal {
		return fmt.Errorf("cannot delete personal organization")
	}

	_, err = s.pool.Exec(ctx, `DELETE FROM organizations WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete organization: %w", err)
	}

	return nil
}

// AddOrganizationMember adds a user to an organization
func (s *Store) AddOrganizationMember(ctx context.Context, orgID, userID uuid.UUID, role MemberRole, invitedBy *uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO organization_members (id, organization_id, user_id, role, invited_by, joined_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (organization_id, user_id) DO UPDATE SET role = $4
	`, uuid.New(), orgID, userID, role, invitedBy, time.Now(), time.Now())

	if err != nil {
		return fmt.Errorf("failed to add member: %w", err)
	}

	return nil
}

// RemoveOrganizationMember removes a user from an organization
func (s *Store) RemoveOrganizationMember(ctx context.Context, orgID, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM organization_members
		WHERE organization_id = $1 AND user_id = $2
	`, orgID, userID)

	if err != nil {
		return fmt.Errorf("failed to remove member: %w", err)
	}

	return nil
}

// UpdateMemberRole updates a member's role
func (s *Store) UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role MemberRole) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE organization_members
		SET role = $3
		WHERE organization_id = $1 AND user_id = $2
	`, orgID, userID, role)

	if err != nil {
		return fmt.Errorf("failed to update member role: %w", err)
	}

	return nil
}

// GetMemberRole gets a user's role in an organization
func (s *Store) GetMemberRole(ctx context.Context, orgID, userID uuid.UUID) (MemberRole, error) {
	var role MemberRole
	err := s.pool.QueryRow(ctx, `
		SELECT role FROM organization_members
		WHERE organization_id = $1 AND user_id = $2
	`, orgID, userID).Scan(&role)

	if err == pgx.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get member role: %w", err)
	}

	return role, nil
}

// ListOrganizationMembers lists all members of an organization
func (s *Store) ListOrganizationMembers(ctx context.Context, orgID uuid.UUID) ([]OrganizationMemberWithUser, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT m.id, m.organization_id, m.user_id, m.role, m.invited_by, m.joined_at, m.created_at,
		       u.github_login, u.name, u.avatar_url
		FROM organization_members m
		JOIN users u ON m.user_id = u.id
		WHERE m.organization_id = $1
		ORDER BY m.role, u.github_login
	`, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list members: %w", err)
	}
	defer rows.Close()

	var members []OrganizationMemberWithUser
	for rows.Next() {
		var m OrganizationMemberWithUser
		err := rows.Scan(&m.ID, &m.OrganizationID, &m.UserID, &m.Role, &m.InvitedBy, &m.JoinedAt, &m.CreatedAt,
			&m.GitHubLogin, &m.Name, &m.AvatarURL)
		if err != nil {
			return nil, fmt.Errorf("failed to scan member: %w", err)
		}
		members = append(members, m)
	}

	return members, nil
}

// OrganizationMemberWithUser includes user details
type OrganizationMemberWithUser struct {
	OrganizationMember
	GitHubLogin string  `json:"github_login"`
	Name        *string `json:"name,omitempty"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
}

// IsMember checks if a user is a member of an organization
func (s *Store) IsMember(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
	role, err := s.GetMemberRole(ctx, orgID, userID)
	if err != nil {
		return false, err
	}
	return role != "", nil
}

// CanManageOrg checks if a user can manage an organization (owner or admin)
func (s *Store) CanManageOrg(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
	role, err := s.GetMemberRole(ctx, orgID, userID)
	if err != nil {
		return false, err
	}
	return role == RoleOwner || role == RoleAdmin, nil
}
