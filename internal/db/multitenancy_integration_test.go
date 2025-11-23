//go:build integration

package db

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/QTest-hq/qtest/internal/testutil"
)

func setupTestStore(t *testing.T) *Store {
	t.Helper()
	testDB := testutil.RequireDB(t)
	db := &DB{pool: testDB.Pool}
	return NewStore(db)
}

// User Tests

func TestIntegration_CreateUser(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	user := &User{
		GitHubID:    12345,
		GitHubLogin: "testuser",
	}

	err := store.CreateUser(ctx, user)
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	if user.ID == uuid.Nil {
		t.Error("user ID should be set")
	}
	if !user.IsActive {
		t.Error("user should be active by default")
	}
	if user.CreatedAt.IsZero() {
		t.Error("created_at should be set")
	}
}

func TestIntegration_GetUserByID(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	user := &User{GitHubID: 12346, GitHubLogin: "testuser2"}
	store.CreateUser(ctx, user)

	retrieved, err := store.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}

	if retrieved.ID != user.ID {
		t.Errorf("ID = %v, want %v", retrieved.ID, user.ID)
	}
	if retrieved.GitHubLogin != user.GitHubLogin {
		t.Errorf("GitHubLogin = %s, want %s", retrieved.GitHubLogin, user.GitHubLogin)
	}
}

func TestIntegration_GetUserByGitHubID(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	user := &User{GitHubID: 12347, GitHubLogin: "testuser3"}
	store.CreateUser(ctx, user)

	retrieved, err := store.GetUserByGitHubID(ctx, 12347)
	if err != nil {
		t.Fatalf("GetUserByGitHubID() error = %v", err)
	}

	if retrieved.ID != user.ID {
		t.Errorf("ID = %v, want %v", retrieved.ID, user.ID)
	}
}

func TestIntegration_UpsertUserFromGitHub(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	// Create new user
	user, err := store.UpsertUserFromGitHub(ctx, 99999, "newuser", "new@test.com", "New User", "https://avatar.url")
	if err != nil {
		t.Fatalf("UpsertUserFromGitHub() create error = %v", err)
	}

	if user.GitHubLogin != "newuser" {
		t.Errorf("GitHubLogin = %s, want newuser", user.GitHubLogin)
	}

	// Update existing user
	updated, err := store.UpsertUserFromGitHub(ctx, 99999, "updateduser", "updated@test.com", "Updated User", "")
	if err != nil {
		t.Fatalf("UpsertUserFromGitHub() update error = %v", err)
	}

	if updated.ID != user.ID {
		t.Error("should update same user, not create new")
	}
	if updated.GitHubLogin != "updateduser" {
		t.Errorf("GitHubLogin = %s, want updateduser", updated.GitHubLogin)
	}
}

func TestIntegration_DeactivateUser(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	user := &User{GitHubID: 12348, GitHubLogin: "testuser4"}
	store.CreateUser(ctx, user)

	err := store.DeactivateUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("DeactivateUser() error = %v", err)
	}

	retrieved, _ := store.GetUserByID(ctx, user.ID)
	if retrieved.IsActive {
		t.Error("user should be deactivated")
	}
}

// Organization Tests

func TestIntegration_CreateOrganization(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	user := &User{GitHubID: 20001, GitHubLogin: "orgowner"}
	store.CreateUser(ctx, user)

	org := &Organization{
		Name:    "Test Org",
		Slug:    "test-org",
		OwnerID: user.ID,
	}

	err := store.CreateOrganization(ctx, org)
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}

	if org.ID == uuid.Nil {
		t.Error("org ID should be set")
	}

	// Check owner is automatically added as member
	role, err := store.GetMemberRole(ctx, org.ID, user.ID)
	if err != nil {
		t.Fatalf("GetMemberRole() error = %v", err)
	}
	if role != RoleOwner {
		t.Errorf("owner role = %s, want owner", role)
	}
}

func TestIntegration_GetOrganizationByID(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	user := &User{GitHubID: 20002, GitHubLogin: "orgowner2"}
	store.CreateUser(ctx, user)

	org := &Organization{Name: "Test Org 2", Slug: "test-org-2", OwnerID: user.ID}
	store.CreateOrganization(ctx, org)

	retrieved, err := store.GetOrganizationByID(ctx, org.ID)
	if err != nil {
		t.Fatalf("GetOrganizationByID() error = %v", err)
	}

	if retrieved.Slug != "test-org-2" {
		t.Errorf("Slug = %s, want test-org-2", retrieved.Slug)
	}
}

func TestIntegration_GetOrganizationBySlug(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	user := &User{GitHubID: 20003, GitHubLogin: "orgowner3"}
	store.CreateUser(ctx, user)

	org := &Organization{Name: "Slug Test", Slug: "slug-test", OwnerID: user.ID}
	store.CreateOrganization(ctx, org)

	retrieved, err := store.GetOrganizationBySlug(ctx, "slug-test")
	if err != nil {
		t.Fatalf("GetOrganizationBySlug() error = %v", err)
	}

	if retrieved.ID != org.ID {
		t.Errorf("ID = %v, want %v", retrieved.ID, org.ID)
	}
}

func TestIntegration_GetPersonalOrganization(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	user := &User{GitHubID: 20004, GitHubLogin: "personalorguser"}
	store.CreateUser(ctx, user)

	// Create personal org
	org := &Organization{
		Name:       "Personal",
		Slug:       "personal-org-user",
		OwnerID:    user.ID,
		IsPersonal: true,
	}
	store.CreateOrganization(ctx, org)

	retrieved, err := store.GetPersonalOrganization(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetPersonalOrganization() error = %v", err)
	}

	if !retrieved.IsPersonal {
		t.Error("should be personal org")
	}
}

func TestIntegration_ListUserOrganizations(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	user := &User{GitHubID: 20005, GitHubLogin: "multiorguser"}
	store.CreateUser(ctx, user)

	// Create 2 orgs
	for i := 1; i <= 2; i++ {
		org := &Organization{
			Name:    "Multi Org",
			Slug:    "multi-org-" + string(rune('a'+i)),
			OwnerID: user.ID,
		}
		store.CreateOrganization(ctx, org)
	}

	orgs, err := store.ListUserOrganizations(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListUserOrganizations() error = %v", err)
	}

	if len(orgs) != 2 {
		t.Errorf("len(orgs) = %d, want 2", len(orgs))
	}

	// Check roles are included
	for _, org := range orgs {
		if org.Role != RoleOwner {
			t.Errorf("role = %s, want owner", org.Role)
		}
	}
}

func TestIntegration_DeleteOrganization(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	user := &User{GitHubID: 20006, GitHubLogin: "deleteorguser"}
	store.CreateUser(ctx, user)

	org := &Organization{Name: "To Delete", Slug: "to-delete", OwnerID: user.ID}
	store.CreateOrganization(ctx, org)

	err := store.DeleteOrganization(ctx, org.ID)
	if err != nil {
		t.Fatalf("DeleteOrganization() error = %v", err)
	}

	retrieved, _ := store.GetOrganizationByID(ctx, org.ID)
	if retrieved != nil {
		t.Error("org should be deleted")
	}
}

func TestIntegration_DeleteOrganization_PersonalFails(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	user := &User{GitHubID: 20007, GitHubLogin: "deletepersonaluser"}
	store.CreateUser(ctx, user)

	org := &Organization{
		Name:       "Personal",
		Slug:       "delete-personal",
		OwnerID:    user.ID,
		IsPersonal: true,
	}
	store.CreateOrganization(ctx, org)

	err := store.DeleteOrganization(ctx, org.ID)
	if err == nil {
		t.Error("should not allow deleting personal org")
	}
}

// Member Tests

func TestIntegration_AddOrganizationMember(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	owner := &User{GitHubID: 30001, GitHubLogin: "memberowner"}
	store.CreateUser(ctx, owner)

	member := &User{GitHubID: 30002, GitHubLogin: "newmember"}
	store.CreateUser(ctx, member)

	org := &Organization{Name: "Member Test", Slug: "member-test", OwnerID: owner.ID}
	store.CreateOrganization(ctx, org)

	err := store.AddOrganizationMember(ctx, org.ID, member.ID, RoleMember, &owner.ID)
	if err != nil {
		t.Fatalf("AddOrganizationMember() error = %v", err)
	}

	role, _ := store.GetMemberRole(ctx, org.ID, member.ID)
	if role != RoleMember {
		t.Errorf("role = %s, want member", role)
	}
}

func TestIntegration_UpdateMemberRole(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	owner := &User{GitHubID: 30003, GitHubLogin: "roleowner"}
	store.CreateUser(ctx, owner)

	member := &User{GitHubID: 30004, GitHubLogin: "rolemember"}
	store.CreateUser(ctx, member)

	org := &Organization{Name: "Role Test", Slug: "role-test", OwnerID: owner.ID}
	store.CreateOrganization(ctx, org)
	store.AddOrganizationMember(ctx, org.ID, member.ID, RoleMember, nil)

	err := store.UpdateMemberRole(ctx, org.ID, member.ID, RoleAdmin)
	if err != nil {
		t.Fatalf("UpdateMemberRole() error = %v", err)
	}

	role, _ := store.GetMemberRole(ctx, org.ID, member.ID)
	if role != RoleAdmin {
		t.Errorf("role = %s, want admin", role)
	}
}

func TestIntegration_RemoveOrganizationMember(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	owner := &User{GitHubID: 30005, GitHubLogin: "removeowner"}
	store.CreateUser(ctx, owner)

	member := &User{GitHubID: 30006, GitHubLogin: "toremove"}
	store.CreateUser(ctx, member)

	org := &Organization{Name: "Remove Test", Slug: "remove-test", OwnerID: owner.ID}
	store.CreateOrganization(ctx, org)
	store.AddOrganizationMember(ctx, org.ID, member.ID, RoleMember, nil)

	err := store.RemoveOrganizationMember(ctx, org.ID, member.ID)
	if err != nil {
		t.Fatalf("RemoveOrganizationMember() error = %v", err)
	}

	isMember, _ := store.IsMember(ctx, org.ID, member.ID)
	if isMember {
		t.Error("member should be removed")
	}
}

func TestIntegration_IsMember(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	owner := &User{GitHubID: 30007, GitHubLogin: "ismemberowner"}
	store.CreateUser(ctx, owner)

	nonMember := &User{GitHubID: 30008, GitHubLogin: "nonmember"}
	store.CreateUser(ctx, nonMember)

	org := &Organization{Name: "IsMember Test", Slug: "ismember-test", OwnerID: owner.ID}
	store.CreateOrganization(ctx, org)

	isMember, _ := store.IsMember(ctx, org.ID, owner.ID)
	if !isMember {
		t.Error("owner should be member")
	}

	isNotMember, _ := store.IsMember(ctx, org.ID, nonMember.ID)
	if isNotMember {
		t.Error("non-member should not be member")
	}
}

func TestIntegration_CanManageOrg(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	owner := &User{GitHubID: 30009, GitHubLogin: "manageowner"}
	store.CreateUser(ctx, owner)

	admin := &User{GitHubID: 30010, GitHubLogin: "manageadmin"}
	store.CreateUser(ctx, admin)

	member := &User{GitHubID: 30011, GitHubLogin: "managemember"}
	store.CreateUser(ctx, member)

	org := &Organization{Name: "Manage Test", Slug: "manage-test", OwnerID: owner.ID}
	store.CreateOrganization(ctx, org)
	store.AddOrganizationMember(ctx, org.ID, admin.ID, RoleAdmin, nil)
	store.AddOrganizationMember(ctx, org.ID, member.ID, RoleMember, nil)

	canOwner, _ := store.CanManageOrg(ctx, org.ID, owner.ID)
	if !canOwner {
		t.Error("owner should be able to manage")
	}

	canAdmin, _ := store.CanManageOrg(ctx, org.ID, admin.ID)
	if !canAdmin {
		t.Error("admin should be able to manage")
	}

	canMember, _ := store.CanManageOrg(ctx, org.ID, member.ID)
	if canMember {
		t.Error("member should not be able to manage")
	}
}

// API Key Tests

func TestIntegration_CreateAPIKey(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	user := &User{GitHubID: 40001, GitHubLogin: "apikeyuser"}
	store.CreateUser(ctx, user)

	org := &Organization{Name: "APIKey Org", Slug: "apikey-org", OwnerID: user.ID}
	store.CreateOrganization(ctx, org)

	key, err := store.CreateAPIKey(ctx, org.ID, user.ID, "Test Key", []string{"repos:read", "jobs:write"}, nil)
	if err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}

	if key.Secret == "" {
		t.Error("secret should be returned on creation")
	}
	if key.KeyPrefix == "" {
		t.Error("key prefix should be set")
	}
	if len(key.Scopes) != 2 {
		t.Errorf("len(scopes) = %d, want 2", len(key.Scopes))
	}
}

func TestIntegration_ValidateAPIKey(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	user := &User{GitHubID: 40002, GitHubLogin: "validatekeyuser"}
	store.CreateUser(ctx, user)

	org := &Organization{Name: "Validate Key Org", Slug: "validate-key-org", OwnerID: user.ID}
	store.CreateOrganization(ctx, org)

	key, _ := store.CreateAPIKey(ctx, org.ID, user.ID, "Validate Key", []string{"repos:read"}, nil)

	validated, err := store.ValidateAPIKey(ctx, key.Secret)
	if err != nil {
		t.Fatalf("ValidateAPIKey() error = %v", err)
	}

	if validated.ID != key.ID {
		t.Errorf("validated ID = %v, want %v", validated.ID, key.ID)
	}
}

func TestIntegration_ValidateAPIKey_Expired(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	user := &User{GitHubID: 40003, GitHubLogin: "expiredkeyuser"}
	store.CreateUser(ctx, user)

	org := &Organization{Name: "Expired Key Org", Slug: "expired-key-org", OwnerID: user.ID}
	store.CreateOrganization(ctx, org)

	past := time.Now().Add(-time.Hour)
	key, _ := store.CreateAPIKey(ctx, org.ID, user.ID, "Expired Key", []string{"repos:read"}, &past)

	validated, err := store.ValidateAPIKey(ctx, key.Secret)
	if err != nil {
		t.Fatalf("ValidateAPIKey() error = %v", err)
	}
	if validated != nil {
		t.Error("expired key should not validate")
	}
}

func TestIntegration_ValidateAPIKey_Revoked(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	user := &User{GitHubID: 40004, GitHubLogin: "revokedkeyuser"}
	store.CreateUser(ctx, user)

	org := &Organization{Name: "Revoked Key Org", Slug: "revoked-key-org", OwnerID: user.ID}
	store.CreateOrganization(ctx, org)

	key, _ := store.CreateAPIKey(ctx, org.ID, user.ID, "Revoked Key", []string{"repos:read"}, nil)
	store.RevokeAPIKey(ctx, key.ID, user.ID)

	validated, err := store.ValidateAPIKey(ctx, key.Secret)
	if err != nil {
		t.Fatalf("ValidateAPIKey() error = %v", err)
	}
	if validated != nil {
		t.Error("revoked key should not validate")
	}
}

func TestIntegration_ListAPIKeysByOrg(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	user := &User{GitHubID: 40005, GitHubLogin: "listkeyuser"}
	store.CreateUser(ctx, user)

	org := &Organization{Name: "List Key Org", Slug: "list-key-org", OwnerID: user.ID}
	store.CreateOrganization(ctx, org)

	store.CreateAPIKey(ctx, org.ID, user.ID, "Key 1", []string{"repos:read"}, nil)
	store.CreateAPIKey(ctx, org.ID, user.ID, "Key 2", []string{"jobs:write"}, nil)

	keys, err := store.ListAPIKeysByOrg(ctx, org.ID)
	if err != nil {
		t.Fatalf("ListAPIKeysByOrg() error = %v", err)
	}

	if len(keys) != 2 {
		t.Errorf("len(keys) = %d, want 2", len(keys))
	}
}

func TestIntegration_RevokeAPIKey(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	user := &User{GitHubID: 40006, GitHubLogin: "revokekeyuser"}
	store.CreateUser(ctx, user)

	org := &Organization{Name: "Revoke Org", Slug: "revoke-org", OwnerID: user.ID}
	store.CreateOrganization(ctx, org)

	key, _ := store.CreateAPIKey(ctx, org.ID, user.ID, "To Revoke", []string{"repos:read"}, nil)

	err := store.RevokeAPIKey(ctx, key.ID, user.ID)
	if err != nil {
		t.Fatalf("RevokeAPIKey() error = %v", err)
	}

	retrieved, _ := store.GetAPIKeyByID(ctx, key.ID)
	if retrieved.RevokedAt == nil {
		t.Error("key should be revoked")
	}
}

// Audit Log Tests

func TestIntegration_CreateAuditLog(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	user := &User{GitHubID: 50001, GitHubLogin: "audituser"}
	store.CreateUser(ctx, user)

	org := &Organization{Name: "Audit Org", Slug: "audit-org", OwnerID: user.ID}
	store.CreateOrganization(ctx, org)

	auditLog := &AuditLog{
		OrganizationID: &org.ID,
		UserID:         &user.ID,
		Action:         "api_key.create",
		ResourceType:   "api_key",
	}
	err := store.CreateAuditLog(ctx, auditLog)
	if err != nil {
		t.Fatalf("CreateAuditLog() error = %v", err)
	}
}

func TestIntegration_ListAuditLogs(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	user := &User{GitHubID: 50002, GitHubLogin: "listaudituser"}
	store.CreateUser(ctx, user)

	org := &Organization{Name: "List Audit Org", Slug: "list-audit-org", OwnerID: user.ID}
	store.CreateOrganization(ctx, org)

	// Create some audit logs
	store.CreateAuditLog(ctx, &AuditLog{OrganizationID: &org.ID, UserID: &user.ID, Action: "action1", ResourceType: "resource"})
	store.CreateAuditLog(ctx, &AuditLog{OrganizationID: &org.ID, UserID: &user.ID, Action: "action2", ResourceType: "resource"})

	logs, err := store.ListAuditLogs(ctx, org.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListAuditLogs() error = %v", err)
	}

	if len(logs) < 2 {
		t.Errorf("len(logs) = %d, want >= 2", len(logs))
	}
}

func TestIntegration_ListAuditLogsByUser(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	user := &User{GitHubID: 50003, GitHubLogin: "useraudituser"}
	store.CreateUser(ctx, user)

	org := &Organization{Name: "User Audit Org", Slug: "user-audit-org", OwnerID: user.ID}
	store.CreateOrganization(ctx, org)

	store.CreateAuditLog(ctx, &AuditLog{OrganizationID: &org.ID, UserID: &user.ID, Action: "user_action", ResourceType: "user"})

	logs, err := store.ListAuditLogsByUser(ctx, user.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListAuditLogsByUser() error = %v", err)
	}

	if len(logs) < 1 {
		t.Errorf("len(logs) = %d, want >= 1", len(logs))
	}
}
