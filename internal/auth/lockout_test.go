package auth

import (
	"testing"
	"time"
)

func TestDefaultLockoutConfig(t *testing.T) {
	cfg := DefaultLockoutConfig()

	if cfg.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d, want 5", cfg.MaxAttempts)
	}
	if cfg.LockoutDuration != 15*time.Minute {
		t.Errorf("LockoutDuration = %v, want 15m", cfg.LockoutDuration)
	}
	if cfg.WindowDuration != 15*time.Minute {
		t.Errorf("WindowDuration = %v, want 15m", cfg.WindowDuration)
	}
}

func TestNewLockoutService(t *testing.T) {
	cfg := LockoutConfig{
		MaxAttempts:     10,
		LockoutDuration: 30 * time.Minute,
		WindowDuration:  10 * time.Minute,
	}

	svc := NewLockoutService(nil, cfg)

	if svc.maxAttempts != 10 {
		t.Errorf("maxAttempts = %d, want 10", svc.maxAttempts)
	}
	if svc.lockoutDuration != 30*time.Minute {
		t.Errorf("lockoutDuration = %v, want 30m", svc.lockoutDuration)
	}
	if svc.windowDuration != 10*time.Minute {
		t.Errorf("windowDuration = %v, want 10m", svc.windowDuration)
	}
}

func TestLockoutStatus(t *testing.T) {
	status := LockoutStatus{
		IsLocked:          true,
		FailedAttempts:    5,
		MaxAttempts:       5,
		RemainingLockout:  10 * time.Minute,
		AttemptsRemaining: 0,
	}

	if !status.IsLocked {
		t.Error("IsLocked should be true")
	}
	if status.FailedAttempts != 5 {
		t.Errorf("FailedAttempts = %d, want 5", status.FailedAttempts)
	}
	if status.AttemptsRemaining != 0 {
		t.Errorf("AttemptsRemaining = %d, want 0", status.AttemptsRemaining)
	}
}

func TestIdentifierTypes(t *testing.T) {
	tests := []struct {
		typ  IdentifierType
		want string
	}{
		{IdentifierTypeIP, "ip"},
		{IdentifierTypeUserID, "user_id"},
		{IdentifierTypeGitHubID, "github_id"},
	}

	for _, tt := range tests {
		if string(tt.typ) != tt.want {
			t.Errorf("IdentifierType %v = %q, want %q", tt.typ, string(tt.typ), tt.want)
		}
	}
}

// Note: Full integration tests for RecordAttempt, IsLocked, GetFailedAttempts, etc.
// require a database connection. These should be run as part of integration tests
// with a test database setup.
//
// The tests above verify:
// - Configuration defaults and initialization
// - Type definitions and constants
// - Status struct construction
//
// For database-dependent tests, use:
//   go test -tags=integration ./internal/auth/... -run TestLockout
