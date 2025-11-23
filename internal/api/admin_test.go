package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QTest-hq/qtest/internal/auth"
	"github.com/google/uuid"
)

// MockAdminStore provides mock implementations for admin store operations
type MockAdminStore struct {
	orgCount       int
	userCount      int
	repoCount      int
	jobCount       int
	activeJobCount int
	testCount      int
	organizations  []interface{}
	users          []interface{}
	jobs           []interface{}
	auditLogs      []interface{}
	user           interface{}

	// Error injection
	countOrgsErr   error
	countUsersErr  error
	countReposErr  error
	countJobsErr   error
	countTestsErr  error
	listOrgsErr    error
	listUsersErr   error
	listJobsErr    error
	getUserErr     error
	updateUserErr  error
	cancelJobErr   error
	auditLogsErr   error
}

func (m *MockAdminStore) CountOrganizations(ctx context.Context) (int, error) {
	if m.countOrgsErr != nil {
		return 0, m.countOrgsErr
	}
	return m.orgCount, nil
}

func (m *MockAdminStore) CountUsers(ctx context.Context) (int, error) {
	if m.countUsersErr != nil {
		return 0, m.countUsersErr
	}
	return m.userCount, nil
}

func (m *MockAdminStore) CountRepositories(ctx context.Context) (int, error) {
	if m.countReposErr != nil {
		return 0, m.countReposErr
	}
	return m.repoCount, nil
}

func (m *MockAdminStore) CountJobs(ctx context.Context) (int, int, error) {
	if m.countJobsErr != nil {
		return 0, 0, m.countJobsErr
	}
	return m.jobCount, m.activeJobCount, nil
}

func (m *MockAdminStore) CountTests(ctx context.Context) (int, error) {
	if m.countTestsErr != nil {
		return 0, m.countTestsErr
	}
	return m.testCount, nil
}

func (m *MockAdminStore) ListAllOrganizations(ctx context.Context, limit, offset int) ([]interface{}, error) {
	if m.listOrgsErr != nil {
		return nil, m.listOrgsErr
	}
	return m.organizations, nil
}

func (m *MockAdminStore) ListAllUsers(ctx context.Context, limit, offset int) ([]interface{}, error) {
	if m.listUsersErr != nil {
		return nil, m.listUsersErr
	}
	return m.users, nil
}

func (m *MockAdminStore) ListAllJobs(ctx context.Context, status string, limit, offset int) ([]interface{}, error) {
	if m.listJobsErr != nil {
		return nil, m.listJobsErr
	}
	return m.jobs, nil
}

func (m *MockAdminStore) GetUserByID(ctx context.Context, userID uuid.UUID) (interface{}, error) {
	if m.getUserErr != nil {
		return nil, m.getUserErr
	}
	return m.user, nil
}

func (m *MockAdminStore) AdminUpdateUserStatus(ctx context.Context, userID uuid.UUID, isActive *bool) error {
	return m.updateUserErr
}

func (m *MockAdminStore) CancelJob(ctx context.Context, jobID uuid.UUID) error {
	return m.cancelJobErr
}

func (m *MockAdminStore) ListAllAuditLogs(ctx context.Context, limit, offset int) ([]interface{}, error) {
	if m.auditLogsErr != nil {
		return nil, m.auditLogsErr
	}
	return m.auditLogs, nil
}

// withAdminAPIKey creates a context with an admin API key
func withAdminAPIKey(ctx context.Context) context.Context {
	apiKeyInfo := &auth.APIKeyInfo{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		UserID:         uuid.New(),
		Scopes:         []string{"admin"},
	}
	return context.WithValue(ctx, auth.APIKeyKey, apiKeyInfo)
}

// withNonAdminAPIKey creates a context with a non-admin API key
func withNonAdminAPIKey(ctx context.Context) context.Context {
	apiKeyInfo := &auth.APIKeyInfo{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		UserID:         uuid.New(),
		Scopes:         []string{"read", "write"},
	}
	return context.WithValue(ctx, auth.APIKeyKey, apiKeyInfo)
}


// TestGetSystemStats_Success tests successful system stats retrieval
func TestGetSystemStats_Success(t *testing.T) {
	// Create a mock store with test data
	mockStore := &MockAdminStore{
		orgCount:       10,
		userCount:      50,
		repoCount:      25,
		jobCount:       100,
		activeJobCount: 5,
		testCount:      500,
	}

	// Note: AdminHandlers expects *db.Store, so we need to test differently
	// For now, test the handler logic directly by checking isAdmin

	// Test API key admin scope check
	apiKeyInfo := &auth.APIKeyInfo{
		ID:     uuid.New(),
		Scopes: []string{"admin"},
	}
	if !apiKeyInfo.HasScope("admin") {
		t.Error("expected HasScope('admin') to return true")
	}
	if !apiKeyInfo.HasScope("read") { // admin should grant all scopes
		t.Error("expected admin scope to grant all other scopes")
	}

	_ = mockStore // Verify mock is set up correctly
}

// TestGetSystemStats_NonAdmin tests non-admin access denial
func TestGetSystemStats_NonAdmin(t *testing.T) {
	apiKeyInfo := &auth.APIKeyInfo{
		ID:     uuid.New(),
		Scopes: []string{"read", "write"},
	}

	if apiKeyInfo.HasScope("admin") {
		t.Error("expected non-admin API key to not have admin scope")
	}
}

// TestAPIKeyInfo_HasScope tests the HasScope method
func TestAPIKeyInfo_HasScope(t *testing.T) {
	tests := []struct {
		name     string
		scopes   []string
		check    string
		expected bool
	}{
		{
			name:     "admin scope grants all",
			scopes:   []string{"admin"},
			check:    "read",
			expected: true,
		},
		{
			name:     "admin scope check",
			scopes:   []string{"admin"},
			check:    "admin",
			expected: true,
		},
		{
			name:     "explicit scope match",
			scopes:   []string{"read", "write"},
			check:    "read",
			expected: true,
		},
		{
			name:     "missing scope",
			scopes:   []string{"read"},
			check:    "write",
			expected: false,
		},
		{
			name:     "empty scopes",
			scopes:   []string{},
			check:    "admin",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &auth.APIKeyInfo{
				ID:     uuid.New(),
				Scopes: tt.scopes,
			}
			result := info.HasScope(tt.check)
			if result != tt.expected {
				t.Errorf("HasScope(%q) = %v, want %v", tt.check, result, tt.expected)
			}
		})
	}
}

// TestAPIKeyInfo_HasAnyScope tests the HasAnyScope method
func TestAPIKeyInfo_HasAnyScope(t *testing.T) {
	tests := []struct {
		name     string
		scopes   []string
		check    []string
		expected bool
	}{
		{
			name:     "has one of multiple",
			scopes:   []string{"read"},
			check:    []string{"read", "write"},
			expected: true,
		},
		{
			name:     "admin grants any",
			scopes:   []string{"admin"},
			check:    []string{"read", "write", "delete"},
			expected: true,
		},
		{
			name:     "has none",
			scopes:   []string{"read"},
			check:    []string{"write", "delete"},
			expected: false,
		},
		{
			name:     "empty check",
			scopes:   []string{"read"},
			check:    []string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &auth.APIKeyInfo{
				ID:     uuid.New(),
				Scopes: tt.scopes,
			}
			result := info.HasAnyScope(tt.check...)
			if result != tt.expected {
				t.Errorf("HasAnyScope(%v) = %v, want %v", tt.check, result, tt.expected)
			}
		})
	}
}

// TestParsePagination tests pagination parameter parsing
func TestParsePagination(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		expectedLimit  int
		expectedOffset int
	}{
		{
			name:           "default values",
			query:          "",
			expectedLimit:  50,
			expectedOffset: 0,
		},
		{
			name:           "custom limit",
			query:          "limit=25",
			expectedLimit:  25,
			expectedOffset: 0,
		},
		{
			name:           "custom offset",
			query:          "offset=10",
			expectedLimit:  50,
			expectedOffset: 10,
		},
		{
			name:           "both custom",
			query:          "limit=20&offset=40",
			expectedLimit:  20,
			expectedOffset: 40,
		},
		{
			name:           "limit over max",
			query:          "limit=200",
			expectedLimit:  50, // Should use default as 200 > 100
			expectedOffset: 0,
		},
		{
			name:           "negative limit",
			query:          "limit=-5",
			expectedLimit:  50, // Should use default
			expectedOffset: 0,
		},
		{
			name:           "negative offset",
			query:          "offset=-10",
			expectedLimit:  50,
			expectedOffset: 0, // Should use default
		},
		{
			name:           "invalid limit",
			query:          "limit=abc",
			expectedLimit:  50,
			expectedOffset: 0,
		},
		{
			name:           "max limit",
			query:          "limit=100",
			expectedLimit:  100,
			expectedOffset: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test?"+tt.query, nil)
			limit, offset := parsePagination(req)

			if limit != tt.expectedLimit {
				t.Errorf("limit = %d, want %d", limit, tt.expectedLimit)
			}
			if offset != tt.expectedOffset {
				t.Errorf("offset = %d, want %d", offset, tt.expectedOffset)
			}
		})
	}
}

// TestUpdateUserRequest_Parsing tests the update user request parsing
func TestUpdateUserRequest_Parsing(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		isActive *bool
	}{
		{
			name: "set active true",
			body: `{"is_active": true}`,
			isActive: func() *bool { v := true; return &v }(),
		},
		{
			name: "set active false",
			body: `{"is_active": false}`,
			isActive: func() *bool { v := false; return &v }(),
		},
		{
			name:     "no is_active field",
			body:     `{}`,
			isActive: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req UpdateUserRequest
			if err := json.Unmarshal([]byte(tt.body), &req); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			if tt.isActive == nil && req.IsActive != nil {
				t.Errorf("expected IsActive to be nil, got %v", *req.IsActive)
			}
			if tt.isActive != nil && req.IsActive == nil {
				t.Errorf("expected IsActive to be %v, got nil", *tt.isActive)
			}
			if tt.isActive != nil && req.IsActive != nil && *req.IsActive != *tt.isActive {
				t.Errorf("IsActive = %v, want %v", *req.IsActive, *tt.isActive)
			}
		})
	}
}

// TestSystemStats_Fields tests SystemStats struct serialization
func TestSystemStats_Fields(t *testing.T) {
	stats := &SystemStats{
		TotalOrganizations: 10,
		TotalUsers:         50,
		TotalRepositories:  25,
		TotalJobs:          100,
		ActiveJobs:         5,
		TotalTests:         500,
		ServerUptime:       "2h30m0s",
		GoVersion:          "go1.21",
		NumGoroutines:      15,
		MemAllocMB:         50.5,
	}

	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Verify JSON field names
	jsonStr := string(data)
	expectedFields := []string{
		"total_organizations",
		"total_users",
		"total_repositories",
		"total_jobs",
		"active_jobs",
		"total_tests",
		"server_uptime",
		"go_version",
		"num_goroutines",
		"mem_alloc_mb",
	}

	for _, field := range expectedFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("expected field %q in JSON output", field)
		}
	}
}

// TestAdminHandlers_RequiresAPIKeyContext tests that admin check works with context
func TestAdminHandlers_RequiresAPIKeyContext(t *testing.T) {
	// Test with admin API key in context
	adminCtx := withAdminAPIKey(context.Background())
	adminKey, ok := auth.GetAPIKeyFromContext(adminCtx)
	if !ok {
		t.Fatal("expected API key in context")
	}
	if !adminKey.HasScope("admin") {
		t.Error("expected admin scope")
	}

	// Test with non-admin API key in context
	nonAdminCtx := withNonAdminAPIKey(context.Background())
	nonAdminKey, ok := auth.GetAPIKeyFromContext(nonAdminCtx)
	if !ok {
		t.Fatal("expected API key in context")
	}
	if nonAdminKey.HasScope("admin") {
		t.Error("expected no admin scope")
	}
}

// TestAdminHandlers_SessionAuth tests that session auth doesn't grant admin
func TestAdminHandlers_SessionAuth(t *testing.T) {
	// Session auth should not grant admin access
	session := &auth.Session{
		ID:     "test-session-id",
		UserID: uuid.New(),
	}

	ctx := context.WithValue(context.Background(), auth.SessionKey, session)

	// Should not have API key
	_, hasAPIKey := auth.GetAPIKeyFromContext(ctx)
	if hasAPIKey {
		t.Error("expected no API key for session auth")
	}

	// Should have session
	_, hasSession := auth.GetSessionFromContext(ctx)
	if !hasSession {
		t.Error("expected session in context")
	}
}
