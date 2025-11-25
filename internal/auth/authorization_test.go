package auth

import (
	"testing"
)

func TestRolePermissions(t *testing.T) {
	tests := []struct {
		role      Role
		action    Action
		canDo     bool
	}{
		// Owner can do everything
		{RoleOwner, ActionRead, true},
		{RoleOwner, ActionWrite, true},
		{RoleOwner, ActionDelete, true},
		{RoleOwner, ActionAdmin, true},

		// Admin can do everything except admin
		{RoleAdmin, ActionRead, true},
		{RoleAdmin, ActionWrite, true},
		{RoleAdmin, ActionDelete, true},
		{RoleAdmin, ActionAdmin, false},

		// Member can read and write but not delete or admin
		{RoleMember, ActionRead, true},
		{RoleMember, ActionWrite, true},
		{RoleMember, ActionDelete, false},
		{RoleMember, ActionAdmin, false},

		// Viewer can only read
		{RoleViewer, ActionRead, true},
		{RoleViewer, ActionWrite, false},
		{RoleViewer, ActionDelete, false},
		{RoleViewer, ActionAdmin, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.role)+"_"+string(tt.action), func(t *testing.T) {
			perms := rolePermissions[tt.role]
			if perms.Can(tt.action) != tt.canDo {
				t.Errorf("role %s, action %s: got %v, want %v", tt.role, tt.action, perms.Can(tt.action), tt.canDo)
			}
		})
	}
}

func TestPermissions_Can_InvalidAction(t *testing.T) {
	perms := Permissions{CanRead: true, CanWrite: true, CanDelete: true, CanAdmin: true}
	if perms.Can(Action("invalid")) {
		t.Error("Can() should return false for invalid action")
	}
}

func TestResourceTypes(t *testing.T) {
	types := []struct {
		typ  ResourceType
		want string
	}{
		{ResourceTypeRepository, "repository"},
		{ResourceTypeJob, "job"},
		{ResourceTypeTest, "test"},
		{ResourceTypeWebhook, "webhook"},
		{ResourceTypeAPIKey, "api_key"},
		{ResourceTypeUser, "user"},
	}

	for _, tt := range types {
		if string(tt.typ) != tt.want {
			t.Errorf("ResourceType %v = %q, want %q", tt.typ, string(tt.typ), tt.want)
		}
	}
}

func TestActions(t *testing.T) {
	actions := []struct {
		action Action
		want   string
	}{
		{ActionRead, "read"},
		{ActionWrite, "write"},
		{ActionDelete, "delete"},
		{ActionAdmin, "admin"},
	}

	for _, tt := range actions {
		if string(tt.action) != tt.want {
			t.Errorf("Action %v = %q, want %q", tt.action, string(tt.action), tt.want)
		}
	}
}

func TestRoles(t *testing.T) {
	roles := []struct {
		role Role
		want string
	}{
		{RoleOwner, "owner"},
		{RoleAdmin, "admin"},
		{RoleMember, "member"},
		{RoleViewer, "viewer"},
	}

	for _, tt := range roles {
		if string(tt.role) != tt.want {
			t.Errorf("Role %v = %q, want %q", tt.role, string(tt.role), tt.want)
		}
	}
}

func TestNewAuthorizationService(t *testing.T) {
	svc := NewAuthorizationService(nil)
	if svc == nil {
		t.Error("NewAuthorizationService() should not return nil")
	}
}

func TestAuthzErrors(t *testing.T) {
	if ErrUnauthorized.Error() != "unauthorized access" {
		t.Errorf("ErrUnauthorized = %q", ErrUnauthorized.Error())
	}
	if ErrForbidden.Error() != "forbidden: insufficient permissions" {
		t.Errorf("ErrForbidden = %q", ErrForbidden.Error())
	}
	if ErrResourceNotFound.Error() != "resource not found" {
		t.Errorf("ErrResourceNotFound = %q", ErrResourceNotFound.Error())
	}
}

func TestPermissionsStruct(t *testing.T) {
	// Test explicit construction
	p := Permissions{
		CanRead:   true,
		CanWrite:  false,
		CanDelete: false,
		CanAdmin:  false,
	}

	if !p.CanRead {
		t.Error("CanRead should be true")
	}
	if p.CanWrite {
		t.Error("CanWrite should be false")
	}
	if p.CanDelete {
		t.Error("CanDelete should be false")
	}
	if p.CanAdmin {
		t.Error("CanAdmin should be false")
	}
}

// Note: Full integration tests for Can, Require, getUserRole, resourceBelongsToOrg,
// GetUserOrganizations, HasOrgAccess, etc. require a database connection.
// These should be run as part of integration tests with a test database setup.
//
// For database-dependent tests, use:
//   go test -tags=integration ./internal/auth/... -run TestAuthz
