// Package webhook provides webhook handling for QTest
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
)

// GitHubReceiver handles incoming GitHub webhooks
type GitHubReceiver struct {
	secrets map[string]string // repo -> secret mapping
	handler GitHubEventHandler
}

// GitHubEventHandler processes GitHub webhook events
type GitHubEventHandler interface {
	HandlePush(event *PushEvent) error
	HandlePullRequest(event *PullRequestEvent) error
	HandlePing(event *PingEvent) error
}

// NewGitHubReceiver creates a new GitHub webhook receiver
func NewGitHubReceiver(handler GitHubEventHandler) *GitHubReceiver {
	return &GitHubReceiver{
		secrets: make(map[string]string),
		handler: handler,
	}
}

// RegisterSecret registers a webhook secret for a repository
func (r *GitHubReceiver) RegisterSecret(repoFullName, secret string) {
	r.secrets[repoFullName] = secret
}

// SetDefaultSecret sets a default secret for all repositories
func (r *GitHubReceiver) SetDefaultSecret(secret string) {
	r.secrets["*"] = secret
}

// HandleWebhook processes an incoming GitHub webhook request
func (r *GitHubReceiver) HandleWebhook(w http.ResponseWriter, req *http.Request) {
	// Read request body
	body, err := io.ReadAll(req.Body)
	if err != nil {
		log.Error().Err(err).Msg("failed to read webhook body")
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	// Get event type
	eventType := req.Header.Get("X-GitHub-Event")
	if eventType == "" {
		http.Error(w, "missing X-GitHub-Event header", http.StatusBadRequest)
		return
	}

	// Get delivery ID
	deliveryID := req.Header.Get("X-GitHub-Delivery")

	log.Info().
		Str("event", eventType).
		Str("delivery_id", deliveryID).
		Msg("received GitHub webhook")

	// Validate signature if we have a secret
	signature := req.Header.Get("X-Hub-Signature-256")
	if signature != "" {
		// Try to get repo name from payload for specific secret
		var payload struct {
			Repository struct {
				FullName string `json:"full_name"`
			} `json:"repository"`
		}
		json.Unmarshal(body, &payload)

		secret := r.getSecret(payload.Repository.FullName)
		if secret != "" {
			if !r.verifySignature(body, signature, secret) {
				log.Warn().
					Str("repo", payload.Repository.FullName).
					Msg("webhook signature verification failed")
				http.Error(w, "invalid signature", http.StatusUnauthorized)
				return
			}
		}
	}

	// Process event based on type
	var processErr error
	switch eventType {
	case "ping":
		var event PingEvent
		if err := json.Unmarshal(body, &event); err != nil {
			http.Error(w, "invalid ping payload", http.StatusBadRequest)
			return
		}
		processErr = r.handler.HandlePing(&event)

	case "push":
		var event PushEvent
		if err := json.Unmarshal(body, &event); err != nil {
			http.Error(w, "invalid push payload", http.StatusBadRequest)
			return
		}
		processErr = r.handler.HandlePush(&event)

	case "pull_request":
		var event PullRequestEvent
		if err := json.Unmarshal(body, &event); err != nil {
			http.Error(w, "invalid pull_request payload", http.StatusBadRequest)
			return
		}
		processErr = r.handler.HandlePullRequest(&event)

	default:
		log.Debug().Str("event", eventType).Msg("ignoring unhandled event type")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ignored"}`))
		return
	}

	if processErr != nil {
		log.Error().Err(processErr).Str("event", eventType).Msg("failed to process webhook event")
		http.Error(w, "failed to process event", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// getSecret gets the secret for a repository
func (r *GitHubReceiver) getSecret(repoFullName string) string {
	if secret, ok := r.secrets[repoFullName]; ok {
		return secret
	}
	return r.secrets["*"] // default secret
}

// verifySignature verifies the GitHub webhook signature
func (r *GitHubReceiver) verifySignature(body []byte, signature, secret string) bool {
	// GitHub sends "sha256=<hash>"
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}

	expectedMAC, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	actualMAC := mac.Sum(nil)

	return hmac.Equal(expectedMAC, actualMAC)
}

// PingEvent represents a GitHub ping event
type PingEvent struct {
	Zen    string `json:"zen"`
	HookID int64  `json:"hook_id"`
	Hook   struct {
		Type   string   `json:"type"`
		ID     int64    `json:"id"`
		Name   string   `json:"name"`
		Active bool     `json:"active"`
		Events []string `json:"events"`
		Config struct {
			ContentType string `json:"content_type"`
			InsecureSSL string `json:"insecure_ssl"`
			URL         string `json:"url"`
		} `json:"config"`
	} `json:"hook"`
	Repository GitHubRepository `json:"repository"`
	Sender     GitHubUser       `json:"sender"`
}

// PushEvent represents a GitHub push event
type PushEvent struct {
	Ref        string           `json:"ref"`
	Before     string           `json:"before"`
	After      string           `json:"after"`
	Created    bool             `json:"created"`
	Deleted    bool             `json:"deleted"`
	Forced     bool             `json:"forced"`
	BaseRef    *string          `json:"base_ref"`
	Compare    string           `json:"compare"`
	Commits    []PushCommit     `json:"commits"`
	HeadCommit *PushCommit      `json:"head_commit"`
	Repository GitHubRepository `json:"repository"`
	Pusher     GitHubPusher     `json:"pusher"`
	Sender     GitHubUser       `json:"sender"`
}

// PushCommit represents a commit in a push event
type PushCommit struct {
	ID        string   `json:"id"`
	TreeID    string   `json:"tree_id"`
	Distinct  bool     `json:"distinct"`
	Message   string   `json:"message"`
	Timestamp string   `json:"timestamp"`
	URL       string   `json:"url"`
	Author    struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Username string `json:"username"`
	} `json:"author"`
	Committer struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Username string `json:"username"`
	} `json:"committer"`
	Added    []string `json:"added"`
	Removed  []string `json:"removed"`
	Modified []string `json:"modified"`
}

// PullRequestEvent represents a GitHub pull request event
type PullRequestEvent struct {
	Action      string           `json:"action"` // opened, closed, synchronize, etc.
	Number      int              `json:"number"`
	PullRequest GitHubPR         `json:"pull_request"`
	Repository  GitHubRepository `json:"repository"`
	Sender      GitHubUser       `json:"sender"`
}

// GitHubPR represents a GitHub pull request
type GitHubPR struct {
	ID     int64  `json:"id"`
	Number int    `json:"number"`
	State  string `json:"state"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	Head   struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"base"`
	HTMLURL  string `json:"html_url"`
	DiffURL  string `json:"diff_url"`
	PatchURL string `json:"patch_url"`
	Merged   bool   `json:"merged"`
	Draft    bool   `json:"draft"`
}

// GitHubRepository represents a GitHub repository
type GitHubRepository struct {
	ID            int64  `json:"id"`
	NodeID        string `json:"node_id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	HTMLURL       string `json:"html_url"`
	CloneURL      string `json:"clone_url"`
	SSHURL        string `json:"ssh_url"`
	DefaultBranch string `json:"default_branch"`
}

// GitHubUser represents a GitHub user
type GitHubUser struct {
	Login     string `json:"login"`
	ID        int64  `json:"id"`
	NodeID    string `json:"node_id"`
	AvatarURL string `json:"avatar_url"`
	Type      string `json:"type"`
}

// GitHubPusher represents the pusher in a push event
type GitHubPusher struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Branch extracts the branch name from the ref
func (e *PushEvent) Branch() string {
	return strings.TrimPrefix(e.Ref, "refs/heads/")
}

// IsDefaultBranch checks if the push is to the default branch
func (e *PushEvent) IsDefaultBranch() bool {
	return e.Branch() == e.Repository.DefaultBranch
}

// ChangedFiles returns all changed files in the push
func (e *PushEvent) ChangedFiles() []string {
	seen := make(map[string]bool)
	var files []string

	for _, commit := range e.Commits {
		for _, f := range commit.Added {
			if !seen[f] {
				files = append(files, f)
				seen[f] = true
			}
		}
		for _, f := range commit.Modified {
			if !seen[f] {
				files = append(files, f)
				seen[f] = true
			}
		}
	}

	return files
}

// DefaultGitHubEventHandler provides default handling for GitHub events
type DefaultGitHubEventHandler struct {
	OnPush        func(*PushEvent) error
	OnPullRequest func(*PullRequestEvent) error
}

func (h *DefaultGitHubEventHandler) HandlePing(event *PingEvent) error {
	log.Info().
		Str("zen", event.Zen).
		Int64("hook_id", event.HookID).
		Str("repo", event.Repository.FullName).
		Msg("received ping from GitHub")
	return nil
}

func (h *DefaultGitHubEventHandler) HandlePush(event *PushEvent) error {
	log.Info().
		Str("repo", event.Repository.FullName).
		Str("branch", event.Branch()).
		Str("after", event.After).
		Int("commits", len(event.Commits)).
		Msg("received push event")

	if h.OnPush != nil {
		return h.OnPush(event)
	}
	return nil
}

func (h *DefaultGitHubEventHandler) HandlePullRequest(event *PullRequestEvent) error {
	log.Info().
		Str("repo", event.Repository.FullName).
		Str("action", event.Action).
		Int("number", event.Number).
		Str("title", event.PullRequest.Title).
		Msg("received pull_request event")

	if h.OnPullRequest != nil {
		return h.OnPullRequest(event)
	}
	return nil
}

// ValidateGitHubSignature validates a GitHub webhook signature
func ValidateGitHubSignature(body []byte, signature, secret string) error {
	if signature == "" {
		return fmt.Errorf("no signature provided")
	}
	if !strings.HasPrefix(signature, "sha256=") {
		return fmt.Errorf("invalid signature format")
	}

	expectedMAC, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	actualMAC := mac.Sum(nil)

	if !hmac.Equal(expectedMAC, actualMAC) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}
