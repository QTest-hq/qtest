package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock handler for testing
type mockEventHandler struct {
	pushCalled   bool
	prCalled     bool
	pingCalled   bool
	pushEvent    *PushEvent
	prEvent      *PullRequestEvent
	pingEvent    *PingEvent
	returnError  error
}

func (m *mockEventHandler) HandlePush(event *PushEvent) error {
	m.pushCalled = true
	m.pushEvent = event
	return m.returnError
}

func (m *mockEventHandler) HandlePullRequest(event *PullRequestEvent) error {
	m.prCalled = true
	m.prEvent = event
	return m.returnError
}

func (m *mockEventHandler) HandlePing(event *PingEvent) error {
	m.pingCalled = true
	m.pingEvent = event
	return m.returnError
}

func TestNewGitHubReceiver(t *testing.T) {
	handler := &mockEventHandler{}
	receiver := NewGitHubReceiver(handler)

	assert.NotNil(t, receiver)
	assert.NotNil(t, receiver.secrets)
	assert.Equal(t, handler, receiver.handler)
}

func TestGitHubReceiver_RegisterSecret(t *testing.T) {
	receiver := NewGitHubReceiver(&mockEventHandler{})

	receiver.RegisterSecret("owner/repo", "secret123")

	assert.Equal(t, "secret123", receiver.secrets["owner/repo"])
}

func TestGitHubReceiver_SetDefaultSecret(t *testing.T) {
	receiver := NewGitHubReceiver(&mockEventHandler{})

	receiver.SetDefaultSecret("default-secret")

	assert.Equal(t, "default-secret", receiver.secrets["*"])
}

func TestGitHubReceiver_GetSecret(t *testing.T) {
	receiver := NewGitHubReceiver(&mockEventHandler{})
	receiver.RegisterSecret("owner/repo", "repo-secret")
	receiver.SetDefaultSecret("default-secret")

	tests := []struct {
		name     string
		repo     string
		expected string
	}{
		{"specific repo", "owner/repo", "repo-secret"},
		{"unknown repo uses default", "other/repo", "default-secret"},
		{"empty repo uses default", "", "default-secret"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := receiver.getSecret(tt.repo)
			assert.Equal(t, tt.expected, secret)
		})
	}
}

func TestGitHubReceiver_VerifySignature(t *testing.T) {
	receiver := NewGitHubReceiver(&mockEventHandler{})
	secret := "test-secret"
	body := []byte(`{"test": "payload"}`)

	// Calculate correct signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	correctSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	tests := []struct {
		name      string
		signature string
		valid     bool
	}{
		{"valid signature", correctSig, true},
		{"invalid signature", "sha256=invalid", false},
		{"missing prefix", hex.EncodeToString(mac.Sum(nil)), false},
		{"empty signature", "", false},
		{"wrong prefix", "sha1=abc123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := receiver.verifySignature(body, tt.signature, secret)
			assert.Equal(t, tt.valid, result)
		})
	}
}

func TestValidateGitHubSignature(t *testing.T) {
	secret := "webhook-secret"
	body := []byte(`{"action": "test"}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	validSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	tests := []struct {
		name      string
		signature string
		wantErr   bool
		errMsg    string
	}{
		{"valid", validSig, false, ""},
		{"empty signature", "", true, "no signature provided"},
		{"wrong prefix", "sha1=abc", true, "invalid signature format"},
		{"invalid hex", "sha256=notvalidhex!", true, "invalid signature encoding"},
		{"wrong signature", "sha256=0000000000000000000000000000000000000000000000000000000000000000", true, "signature mismatch"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGitHubSignature(body, tt.signature, secret)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGitHubReceiver_HandleWebhook_Ping(t *testing.T) {
	handler := &mockEventHandler{}
	receiver := NewGitHubReceiver(handler)

	payload := PingEvent{
		Zen:    "Keep it simple",
		HookID: 12345,
		Repository: GitHubRepository{
			FullName: "owner/repo",
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "ping")
	req.Header.Set("X-GitHub-Delivery", "delivery-123")

	w := httptest.NewRecorder()
	receiver.HandleWebhook(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, handler.pingCalled)
	assert.Equal(t, "Keep it simple", handler.pingEvent.Zen)
	assert.Equal(t, int64(12345), handler.pingEvent.HookID)
}

func TestGitHubReceiver_HandleWebhook_Push(t *testing.T) {
	handler := &mockEventHandler{}
	receiver := NewGitHubReceiver(handler)

	payload := PushEvent{
		Ref:    "refs/heads/main",
		After:  "abc123",
		Before: "def456",
		Repository: GitHubRepository{
			FullName:      "owner/repo",
			DefaultBranch: "main",
		},
		Commits: []PushCommit{
			{
				ID:      "abc123",
				Message: "Test commit",
				Added:   []string{"new.go"},
				Modified: []string{"existing.go"},
			},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-GitHub-Delivery", "delivery-456")

	w := httptest.NewRecorder()
	receiver.HandleWebhook(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, handler.pushCalled)
	assert.Equal(t, "refs/heads/main", handler.pushEvent.Ref)
	assert.Equal(t, "abc123", handler.pushEvent.After)
}

func TestGitHubReceiver_HandleWebhook_PullRequest(t *testing.T) {
	handler := &mockEventHandler{}
	receiver := NewGitHubReceiver(handler)

	payload := PullRequestEvent{
		Action: "opened",
		Number: 42,
		PullRequest: GitHubPR{
			Title: "Test PR",
			State: "open",
			Head: struct {
				Ref string `json:"ref"`
				SHA string `json:"sha"`
			}{Ref: "feature", SHA: "abc123"},
			Base: struct {
				Ref string `json:"ref"`
				SHA string `json:"sha"`
			}{Ref: "main", SHA: "def456"},
		},
		Repository: GitHubRepository{
			FullName: "owner/repo",
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-GitHub-Delivery", "delivery-789")

	w := httptest.NewRecorder()
	receiver.HandleWebhook(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, handler.prCalled)
	assert.Equal(t, "opened", handler.prEvent.Action)
	assert.Equal(t, 42, handler.prEvent.Number)
	assert.Equal(t, "Test PR", handler.prEvent.PullRequest.Title)
}

func TestGitHubReceiver_HandleWebhook_UnknownEvent(t *testing.T) {
	handler := &mockEventHandler{}
	receiver := NewGitHubReceiver(handler)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("X-GitHub-Event", "issues")
	req.Header.Set("X-GitHub-Delivery", "delivery-000")

	w := httptest.NewRecorder()
	receiver.HandleWebhook(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ignored")
	assert.False(t, handler.pushCalled)
	assert.False(t, handler.prCalled)
	assert.False(t, handler.pingCalled)
}

func TestGitHubReceiver_HandleWebhook_MissingEventHeader(t *testing.T) {
	handler := &mockEventHandler{}
	receiver := NewGitHubReceiver(handler)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader([]byte(`{}`)))
	// No X-GitHub-Event header

	w := httptest.NewRecorder()
	receiver.HandleWebhook(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "missing X-GitHub-Event")
}

func TestGitHubReceiver_HandleWebhook_InvalidPayload(t *testing.T) {
	handler := &mockEventHandler{}
	receiver := NewGitHubReceiver(handler)

	tests := []struct {
		name      string
		eventType string
		body      string
	}{
		{"invalid ping", "ping", "{invalid json"},
		{"invalid push", "push", "{invalid json"},
		{"invalid pull_request", "pull_request", "{invalid json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader([]byte(tt.body)))
			req.Header.Set("X-GitHub-Event", tt.eventType)

			w := httptest.NewRecorder()
			receiver.HandleWebhook(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestGitHubReceiver_HandleWebhook_WithSignature(t *testing.T) {
	handler := &mockEventHandler{}
	receiver := NewGitHubReceiver(handler)
	receiver.SetDefaultSecret("test-secret")

	payload := PingEvent{Zen: "Test"}
	body, _ := json.Marshal(payload)

	// Calculate correct signature
	mac := hmac.New(sha256.New, []byte("test-secret"))
	mac.Write(body)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "ping")
	req.Header.Set("X-Hub-Signature-256", signature)

	w := httptest.NewRecorder()
	receiver.HandleWebhook(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, handler.pingCalled)
}

func TestGitHubReceiver_HandleWebhook_InvalidSignature(t *testing.T) {
	handler := &mockEventHandler{}
	receiver := NewGitHubReceiver(handler)
	receiver.SetDefaultSecret("test-secret")

	payload := PingEvent{Zen: "Test"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "ping")
	req.Header.Set("X-Hub-Signature-256", "sha256=invalid")

	w := httptest.NewRecorder()
	receiver.HandleWebhook(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "invalid signature")
	assert.False(t, handler.pingCalled)
}

func TestPushEvent_Branch(t *testing.T) {
	tests := []struct {
		ref      string
		expected string
	}{
		{"refs/heads/main", "main"},
		{"refs/heads/feature/test", "feature/test"},
		{"refs/heads/", ""},
		{"main", "main"},
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			event := &PushEvent{Ref: tt.ref}
			assert.Equal(t, tt.expected, event.Branch())
		})
	}
}

func TestPushEvent_IsDefaultBranch(t *testing.T) {
	tests := []struct {
		name          string
		ref           string
		defaultBranch string
		expected      bool
	}{
		{"main branch", "refs/heads/main", "main", true},
		{"master branch", "refs/heads/master", "master", true},
		{"feature branch", "refs/heads/feature", "main", false},
		{"develop on develop default", "refs/heads/develop", "develop", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &PushEvent{
				Ref: tt.ref,
				Repository: GitHubRepository{
					DefaultBranch: tt.defaultBranch,
				},
			}
			assert.Equal(t, tt.expected, event.IsDefaultBranch())
		})
	}
}

func TestPushEvent_ChangedFiles(t *testing.T) {
	event := &PushEvent{
		Commits: []PushCommit{
			{
				Added:    []string{"new1.go", "new2.go"},
				Modified: []string{"mod1.go"},
			},
			{
				Added:    []string{"new3.go", "new1.go"}, // new1.go is duplicate
				Modified: []string{"mod1.go", "mod2.go"}, // mod1.go is duplicate
			},
		},
	}

	files := event.ChangedFiles()

	assert.Len(t, files, 5) // new1, new2, mod1, new3, mod2 (deduped)
	assert.Contains(t, files, "new1.go")
	assert.Contains(t, files, "new2.go")
	assert.Contains(t, files, "new3.go")
	assert.Contains(t, files, "mod1.go")
	assert.Contains(t, files, "mod2.go")
}

func TestPushEvent_ChangedFiles_Empty(t *testing.T) {
	event := &PushEvent{
		Commits: []PushCommit{},
	}

	files := event.ChangedFiles()
	assert.Empty(t, files)
}

func TestDefaultGitHubEventHandler_HandlePing(t *testing.T) {
	handler := &DefaultGitHubEventHandler{}

	event := &PingEvent{
		Zen:    "Test zen",
		HookID: 123,
		Repository: GitHubRepository{
			FullName: "owner/repo",
		},
	}

	err := handler.HandlePing(event)
	assert.NoError(t, err)
}

func TestDefaultGitHubEventHandler_HandlePush(t *testing.T) {
	t.Run("without callback", func(t *testing.T) {
		handler := &DefaultGitHubEventHandler{}

		event := &PushEvent{
			Ref:   "refs/heads/main",
			After: "abc123",
			Repository: GitHubRepository{
				FullName: "owner/repo",
			},
		}

		err := handler.HandlePush(event)
		assert.NoError(t, err)
	})

	t.Run("with callback", func(t *testing.T) {
		called := false
		handler := &DefaultGitHubEventHandler{
			OnPush: func(e *PushEvent) error {
				called = true
				return nil
			},
		}

		event := &PushEvent{}
		err := handler.HandlePush(event)

		assert.NoError(t, err)
		assert.True(t, called)
	})
}

func TestDefaultGitHubEventHandler_HandlePullRequest(t *testing.T) {
	t.Run("without callback", func(t *testing.T) {
		handler := &DefaultGitHubEventHandler{}

		event := &PullRequestEvent{
			Action: "opened",
			Number: 1,
			PullRequest: GitHubPR{
				Title: "Test PR",
			},
			Repository: GitHubRepository{
				FullName: "owner/repo",
			},
		}

		err := handler.HandlePullRequest(event)
		assert.NoError(t, err)
	})

	t.Run("with callback", func(t *testing.T) {
		called := false
		handler := &DefaultGitHubEventHandler{
			OnPullRequest: func(e *PullRequestEvent) error {
				called = true
				return nil
			},
		}

		event := &PullRequestEvent{}
		err := handler.HandlePullRequest(event)

		assert.NoError(t, err)
		assert.True(t, called)
	})
}

func TestGitHubPR_Fields(t *testing.T) {
	pr := GitHubPR{
		ID:     123,
		Number: 42,
		State:  "open",
		Title:  "Test PR",
		Body:   "PR body",
		Head: struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		}{Ref: "feature", SHA: "abc"},
		Base: struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		}{Ref: "main", SHA: "def"},
		HTMLURL:  "https://github.com/owner/repo/pull/42",
		DiffURL:  "https://github.com/owner/repo/pull/42.diff",
		PatchURL: "https://github.com/owner/repo/pull/42.patch",
		Merged:   false,
		Draft:    false,
	}

	assert.Equal(t, int64(123), pr.ID)
	assert.Equal(t, 42, pr.Number)
	assert.Equal(t, "open", pr.State)
	assert.Equal(t, "Test PR", pr.Title)
	assert.Equal(t, "feature", pr.Head.Ref)
	assert.Equal(t, "main", pr.Base.Ref)
	assert.False(t, pr.Merged)
}

func TestGitHubRepository_Fields(t *testing.T) {
	repo := GitHubRepository{
		ID:            12345,
		NodeID:        "MDEwOlJlcG9zaXRvcnkx",
		Name:          "repo",
		FullName:      "owner/repo",
		Private:       false,
		HTMLURL:       "https://github.com/owner/repo",
		CloneURL:      "https://github.com/owner/repo.git",
		SSHURL:        "git@github.com:owner/repo.git",
		DefaultBranch: "main",
	}

	assert.Equal(t, int64(12345), repo.ID)
	assert.Equal(t, "repo", repo.Name)
	assert.Equal(t, "owner/repo", repo.FullName)
	assert.False(t, repo.Private)
	assert.Equal(t, "main", repo.DefaultBranch)
}

func TestPushCommit_Fields(t *testing.T) {
	commit := PushCommit{
		ID:        "abc123",
		TreeID:    "tree123",
		Distinct:  true,
		Message:   "Test commit",
		Timestamp: "2024-01-01T00:00:00Z",
		URL:       "https://github.com/owner/repo/commit/abc123",
		Author: struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Username string `json:"username"`
		}{Name: "Author", Email: "author@example.com", Username: "author"},
		Committer: struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Username string `json:"username"`
		}{Name: "Committer", Email: "committer@example.com", Username: "committer"},
		Added:    []string{"new.go"},
		Removed:  []string{"old.go"},
		Modified: []string{"changed.go"},
	}

	assert.Equal(t, "abc123", commit.ID)
	assert.True(t, commit.Distinct)
	assert.Equal(t, "Test commit", commit.Message)
	assert.Equal(t, "Author", commit.Author.Name)
	assert.Contains(t, commit.Added, "new.go")
	assert.Contains(t, commit.Removed, "old.go")
	assert.Contains(t, commit.Modified, "changed.go")
}
