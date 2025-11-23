package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// CountOrganizations returns the total number of organizations
func (s *Store) CountOrganizations(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM organizations").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count organizations: %w", err)
	}
	return count, nil
}

// CountUsers returns the total number of users
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}

// CountRepositories returns the total number of repositories
func (s *Store) CountRepositories(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM repositories").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count repositories: %w", err)
	}
	return count, nil
}

// CountJobs returns total and active job counts
func (s *Store) CountJobs(ctx context.Context) (total int, active int, err error) {
	err = s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM jobs").Scan(&total)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to count jobs: %w", err)
	}

	err = s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM jobs WHERE status IN ('pending', 'running')").Scan(&active)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to count active jobs: %w", err)
	}

	return total, active, nil
}

// CountTests returns the total number of generated tests
func (s *Store) CountTests(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM generated_tests").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count tests: %w", err)
	}
	return count, nil
}

// ListAllOrganizations lists all organizations (admin only)
func (s *Store) ListAllOrganizations(ctx context.Context, limit, offset int) ([]Organization, error) {
	query := `
		SELECT id, name, slug, description, owner_id, github_org_id, settings, is_personal, created_at, updated_at
		FROM organizations
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := s.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list organizations: %w", err)
	}
	defer rows.Close()

	var orgs []Organization
	for rows.Next() {
		var o Organization
		err := rows.Scan(&o.ID, &o.Name, &o.Slug, &o.Description, &o.OwnerID, &o.GitHubOrgID, &o.Settings, &o.IsPersonal, &o.CreatedAt, &o.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan organization: %w", err)
		}
		orgs = append(orgs, o)
	}

	return orgs, nil
}

// ListAllUsers lists all users (admin only)
func (s *Store) ListAllUsers(ctx context.Context, limit, offset int) ([]User, error) {
	query := `
		SELECT id, github_id, github_login, email, name, avatar_url, is_active, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := s.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		err := rows.Scan(&u.ID, &u.GitHubID, &u.GitHubLogin, &u.Email, &u.Name, &u.AvatarURL, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, u)
	}

	return users, nil
}

// AdminUpdateUserStatus updates user active status (admin only)
func (s *Store) AdminUpdateUserStatus(ctx context.Context, userID uuid.UUID, isActive *bool) error {
	query := `
		UPDATE users SET
			is_active = COALESCE($2, is_active),
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`

	_, err := s.pool.Exec(ctx, query, userID, isActive)
	if err != nil {
		return fmt.Errorf("failed to update user status: %w", err)
	}

	return nil
}

// ListAllJobs lists all jobs (admin only)
func (s *Store) ListAllJobs(ctx context.Context, status string, limit, offset int) ([]map[string]interface{}, error) {
	query := `
		SELECT j.id, j.organization_id, j.type, j.status, j.priority,
		       j.created_at, j.started_at, j.completed_at,
		       o.name as org_name
		FROM jobs j
		LEFT JOIN organizations o ON j.organization_id = o.id
		WHERE ($1 = '' OR j.status = $1)
		ORDER BY j.created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := s.pool.Query(ctx, query, status, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []map[string]interface{}
	for rows.Next() {
		var j struct {
			ID          uuid.UUID
			OrgID       *uuid.UUID
			Type        string
			Status      string
			Priority    int
			CreatedAt   interface{}
			StartedAt   interface{}
			CompletedAt interface{}
			OrgName     *string
		}

		err := rows.Scan(&j.ID, &j.OrgID, &j.Type, &j.Status, &j.Priority,
			&j.CreatedAt, &j.StartedAt, &j.CompletedAt, &j.OrgName)
		if err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}

		job := map[string]interface{}{
			"id":           j.ID,
			"org_id":       j.OrgID,
			"type":         j.Type,
			"status":       j.Status,
			"priority":     j.Priority,
			"created_at":   j.CreatedAt,
			"started_at":   j.StartedAt,
			"completed_at": j.CompletedAt,
			"org_name":     j.OrgName,
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// CancelJob cancels a job
func (s *Store) CancelJob(ctx context.Context, jobID uuid.UUID) error {
	query := `
		UPDATE jobs SET
			status = 'cancelled',
			completed_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND status IN ('pending', 'running')
	`

	_, err := s.pool.Exec(ctx, query, jobID)
	if err != nil {
		return fmt.Errorf("failed to cancel job: %w", err)
	}

	return nil
}

// ListAllAuditLogs lists all audit logs (admin only)
func (s *Store) ListAllAuditLogs(ctx context.Context, limit, offset int) ([]AuditLog, error) {
	query := `
		SELECT id, organization_id, user_id, action, resource_type, resource_id,
		       details, ip_address, user_agent, created_at
		FROM audit_logs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := s.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit logs: %w", err)
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		var l AuditLog
		err := rows.Scan(&l.ID, &l.OrganizationID, &l.UserID, &l.Action, &l.ResourceType, &l.ResourceID,
			&l.Details, &l.IPAddress, &l.UserAgent, &l.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit log: %w", err)
		}
		logs = append(logs, l)
	}

	return logs, nil
}
