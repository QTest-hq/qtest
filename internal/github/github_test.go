package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// PRService Tests
// =============================================================================

func TestNewPRService(t *testing.T) {
	token := "test-token"
	svc := NewPRService(token)

	if svc == nil {
		t.Fatal("NewPRService returned nil")
	}

	if svc.token != token {
		t.Errorf("token = %s, want %s", svc.token, token)
	}

	if svc.baseURL != "https://api.github.com" {
		t.Errorf("baseURL = %s, want https://api.github.com", svc.baseURL)
	}

	if svc.client == nil {
		t.Error("client should not be nil")
	}
}

func TestPRService_SetHeaders(t *testing.T) {
	svc := NewPRService("test-token")
	req, _ := http.NewRequest("GET", "https://api.github.com/test", nil)

	svc.setHeaders(req)

	tests := []struct {
		header string
		want   string
	}{
		{"Accept", "application/vnd.github+json"},
		{"Authorization", "Bearer test-token"},
		{"X-GitHub-Api-Version", "2022-11-28"},
		{"Content-Type", "application/json"},
	}

	for _, tt := range tests {
		got := req.Header.Get(tt.header)
		if got != tt.want {
			t.Errorf("Header %s = %s, want %s", tt.header, got, tt.want)
		}
	}
}

func TestBase64Encode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"a", "YQ=="},
		{"ab", "YWI="},
		{"abc", "YWJj"},
		{"Hello, World!", "SGVsbG8sIFdvcmxkIQ=="},
		{"test content\nwith newlines", "dGVzdCBjb250ZW50CndpdGggbmV3bGluZXM="},
	}

	for _, tt := range tests {
		got := base64Encode([]byte(tt.input))
		if got != tt.want {
			t.Errorf("base64Encode(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestEncodeBase64(t *testing.T) {
	// encodeBase64 is a wrapper around base64Encode
	input := "test string"
	got := encodeBase64(input)
	expected := base64Encode([]byte(input))

	if got != expected {
		t.Errorf("encodeBase64(%q) = %q, want %q", input, got, expected)
	}
}

func TestGeneratePRBody(t *testing.T) {
	tmpl := PRTemplate{
		TestCount:     10,
		CoverageDelta: 5.5,
		Files:         []string{"test_users.py", "test_items.py"},
		Framework:     "pytest",
		Language:      "Python",
	}

	body := GeneratePRBody(tmpl)

	// Verify key elements are present
	assertions := []struct {
		name    string
		contain string
	}{
		{"summary", "## Summary"},
		{"test count", "**10 generated tests**"},
		{"coverage", "+5.5%"},
		{"files section", "## Test Files"},
		{"file1", "`test_users.py`"},
		{"file2", "`test_items.py`"},
		{"language", "**Language**: Python"},
		{"framework", "**Framework**: pytest"},
		{"checklist", "## Checklist"},
		{"qtest link", "QTest"},
	}

	for _, a := range assertions {
		if !strings.Contains(body, a.contain) {
			t.Errorf("PR body should contain %s (%q)", a.name, a.contain)
		}
	}
}

func TestGeneratePRBody_NoCoverage(t *testing.T) {
	tmpl := PRTemplate{
		TestCount:     5,
		CoverageDelta: 0, // No coverage improvement
		Files:         []string{"test.go"},
		Framework:     "go-test",
		Language:      "Go",
	}

	body := GeneratePRBody(tmpl)

	// Should not contain coverage line when delta is 0
	if strings.Contains(body, "coverage improvement") {
		t.Error("PR body should not contain coverage when delta is 0")
	}
}

// Test CreateBranch with mock server
func TestPRService_CreateBranch(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{"success", 201, false},
		{"branch exists", 422, false}, // 422 means branch exists, which is OK
		{"server error", 500, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					t.Errorf("expected POST, got %s", r.Method)
				}
				if !strings.Contains(r.URL.Path, "/git/refs") {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			svc := NewPRService("test-token")
			svc.baseURL = server.URL

			err := svc.CreateBranch(context.Background(), BranchRequest{
				Owner:  "owner",
				Repo:   "repo",
				Branch: "feature-branch",
				SHA:    "abc123",
			})

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// Test CreatePR with mock server
func TestPRService_CreatePR(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}

			w.WriteHeader(201)
			json.NewEncoder(w).Encode(PRResponse{
				ID:      123,
				Number:  42,
				HTMLURL: "https://github.com/owner/repo/pull/42",
				State:   "open",
				Title:   "Test PR",
			})
		}))
		defer server.Close()

		svc := NewPRService("test-token")
		svc.baseURL = server.URL

		pr, err := svc.CreatePR(context.Background(), PRRequest{
			Owner: "owner",
			Repo:  "repo",
			Title: "Test PR",
			Body:  "Test body",
			Head:  "feature",
			Base:  "main",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if pr.Number != 42 {
			t.Errorf("PR number = %d, want 42", pr.Number)
		}
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
			w.Write([]byte("Internal Server Error"))
		}))
		defer server.Close()

		svc := NewPRService("test-token")
		svc.baseURL = server.URL

		_, err := svc.CreatePR(context.Background(), PRRequest{
			Owner: "owner",
			Repo:  "repo",
			Title: "Test PR",
			Head:  "feature",
			Base:  "main",
		})

		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

// Test FindPR with mock server
func TestPRService_FindPR(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				t.Errorf("expected GET, got %s", r.Method)
			}

			w.WriteHeader(200)
			json.NewEncoder(w).Encode([]PRResponse{
				{Number: 42, HTMLURL: "https://github.com/owner/repo/pull/42"},
			})
		}))
		defer server.Close()

		svc := NewPRService("test-token")
		svc.baseURL = server.URL

		pr, err := svc.FindPR(context.Background(), "owner", "repo", "feature", "main")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if pr == nil {
			t.Fatal("expected PR, got nil")
		}

		if pr.Number != 42 {
			t.Errorf("PR number = %d, want 42", pr.Number)
		}
	})

	t.Run("not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			json.NewEncoder(w).Encode([]PRResponse{}) // Empty array
		}))
		defer server.Close()

		svc := NewPRService("test-token")
		svc.baseURL = server.URL

		pr, err := svc.FindPR(context.Background(), "owner", "repo", "feature", "main")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if pr != nil {
			t.Errorf("expected nil PR, got %v", pr)
		}
	})
}

// Test GetDefaultBranch
func TestPRService_GetDefaultBranch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]string{
				"default_branch": "develop",
			})
		}))
		defer server.Close()

		svc := NewPRService("test-token")
		svc.baseURL = server.URL

		branch, err := svc.GetDefaultBranch(context.Background(), "owner", "repo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if branch != "develop" {
			t.Errorf("branch = %s, want develop", branch)
		}
	})

	t.Run("fallback to main", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(404) // Repo not found
		}))
		defer server.Close()

		svc := NewPRService("test-token")
		svc.baseURL = server.URL

		branch, err := svc.GetDefaultBranch(context.Background(), "owner", "repo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if branch != "main" {
			t.Errorf("branch = %s, want main (fallback)", branch)
		}
	})
}

// Test GetLatestCommitSHA
func TestPRService_GetLatestCommitSHA(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"object": map[string]string{
					"sha": "abc123def456",
				},
			})
		}))
		defer server.Close()

		svc := NewPRService("test-token")
		svc.baseURL = server.URL

		sha, err := svc.GetLatestCommitSHA(context.Background(), "owner", "repo", "main")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if sha != "abc123def456" {
			t.Errorf("sha = %s, want abc123def456", sha)
		}
	})

	t.Run("branch not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(404)
		}))
		defer server.Close()

		svc := NewPRService("test-token")
		svc.baseURL = server.URL

		_, err := svc.GetLatestCommitSHA(context.Background(), "owner", "repo", "nonexistent")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

// Test CommitFile
func TestPRService_CommitFile(t *testing.T) {
	t.Run("new file", func(t *testing.T) {
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			if r.Method == "GET" {
				// File doesn't exist
				w.WriteHeader(404)
				return
			}
			if r.Method == "PUT" {
				w.WriteHeader(201)
				return
			}
		}))
		defer server.Close()

		svc := NewPRService("test-token")
		svc.baseURL = server.URL

		err := svc.CommitFile(context.Background(), "owner", "repo", "feature",
			"test/file.go", "package test", "Add test file")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("update existing file", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				// File exists, return SHA
				w.WriteHeader(200)
				json.NewEncoder(w).Encode(map[string]string{
					"sha": "existing-sha",
				})
				return
			}
			if r.Method == "PUT" {
				w.WriteHeader(200)
				return
			}
		}))
		defer server.Close()

		svc := NewPRService("test-token")
		svc.baseURL = server.URL

		err := svc.CommitFile(context.Background(), "owner", "repo", "feature",
			"test/file.go", "package test // updated", "Update test file")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// =============================================================================
// RepoService Tests
// =============================================================================

func TestNewRepoService(t *testing.T) {
	svc := NewRepoService("/tmp/repos", "test-token")

	if svc == nil {
		t.Fatal("NewRepoService returned nil")
	}

	if svc.baseDir != "/tmp/repos" {
		t.Errorf("baseDir = %s, want /tmp/repos", svc.baseDir)
	}

	if svc.token != "test-token" {
		t.Errorf("token = %s, want test-token", svc.token)
	}
}

func TestParseRepoURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantOwner string
		wantName  string
		wantErr   bool
	}{
		{
			name:      "https URL",
			url:       "https://github.com/owner/repo",
			wantOwner: "owner",
			wantName:  "repo",
			wantErr:   false,
		},
		{
			name:      "https URL with .git",
			url:       "https://github.com/owner/repo.git",
			wantOwner: "owner",
			wantName:  "repo",
			wantErr:   false,
		},
		{
			name:      "SSH URL",
			url:       "git@github.com:owner/repo.git",
			wantOwner: "owner",
			wantName:  "repo",
			wantErr:   false,
		},
		{
			name:      "SSH URL without .git",
			url:       "git@github.com:owner/repo",
			wantOwner: "owner",
			wantName:  "repo",
			wantErr:   false,
		},
		{
			name:    "non-github URL",
			url:     "https://gitlab.com/owner/repo",
			wantErr: true,
		},
		{
			name:    "invalid URL",
			url:     "not-a-url",
			wantErr: true,
		},
		{
			name:    "missing repo in path",
			url:     "https://github.com/owner",
			wantErr: true,
		},
		{
			name:    "invalid SSH format",
			url:     "git@github.com/owner/repo",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ParseRepoURL(tt.url)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if info.Owner != tt.wantOwner {
				t.Errorf("Owner = %s, want %s", info.Owner, tt.wantOwner)
			}

			if info.Name != tt.wantName {
				t.Errorf("Name = %s, want %s", info.Name, tt.wantName)
			}

			// Verify CloneURL is always HTTPS
			if !strings.HasPrefix(info.CloneURL, "https://") {
				t.Errorf("CloneURL should be HTTPS, got %s", info.CloneURL)
			}
		})
	}
}

func TestGetFiles(t *testing.T) {
	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "github-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	createTestFile(t, tmpDir, "main.go", "package main")
	createTestFile(t, tmpDir, "utils.go", "package main")
	createTestFile(t, tmpDir, "README.md", "# Test")
	createTestFile(t, tmpDir, "src/handler.go", "package src")
	createTestFile(t, tmpDir, "src/handler_test.go", "package src")

	// Create directories that should be skipped
	os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755)
	createTestFile(t, tmpDir, ".git/config", "git config")
	os.MkdirAll(filepath.Join(tmpDir, "node_modules/pkg"), 0755)
	createTestFile(t, tmpDir, "node_modules/pkg/index.js", "module.exports = {}")

	tests := []struct {
		name     string
		patterns []string
		want     []string
	}{
		{
			name:     "go files",
			patterns: []string{"*.go"},
			want:     []string{"main.go", "utils.go"},
		},
		{
			name:     "markdown files",
			patterns: []string{"*.md"},
			want:     []string{"README.md"},
		},
		{
			name:     "multiple patterns",
			patterns: []string{"*.go", "*.md"},
			want:     []string{"main.go", "utils.go", "README.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := GetFiles(tmpDir, tt.patterns)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check that expected files are found
			for _, want := range tt.want {
				found := false
				for _, f := range files {
					if filepath.Base(f) == want || f == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected file %s not found in %v", want, files)
				}
			}

			// Check that hidden dirs and node_modules are skipped
			for _, f := range files {
				if strings.Contains(f, ".git") {
					t.Errorf("should skip .git directory, found %s", f)
				}
				if strings.Contains(f, "node_modules") {
					t.Errorf("should skip node_modules, found %s", f)
				}
			}
		})
	}
}

func TestDetectLanguages(t *testing.T) {
	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "github-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	createTestFile(t, tmpDir, "main.go", "package main")
	createTestFile(t, tmpDir, "utils.go", "package main")
	createTestFile(t, tmpDir, "main_test.go", "package main") // Should be skipped (_test.)
	createTestFile(t, tmpDir, "app.py", "import os")
	createTestFile(t, tmpDir, "app.test.py", "import pytest") // Should be skipped (.test.)
	createTestFile(t, tmpDir, "index.js", "console.log('hello')")
	createTestFile(t, tmpDir, "component.tsx", "export default function() {}")
	createTestFile(t, tmpDir, "Main.java", "public class Main {}")

	languages, err := DetectLanguages(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		lang string
		want int
	}{
		{"go", 2},     // main.go and utils.go (not main_test.go)
		{"python", 1}, // app.py (not app.test.py)
		{"javascript", 1},
		{"typescript", 1},
		{"java", 1},
	}

	for _, tt := range tests {
		got := languages[tt.lang]
		if got != tt.want {
			t.Errorf("languages[%s] = %d, want %d", tt.lang, got, tt.want)
		}
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

func createTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file %s: %v", name, err)
	}
}

// =============================================================================
// PRRequest and PRResponse Tests
// =============================================================================

func TestPRRequest_Fields(t *testing.T) {
	req := PRRequest{
		Owner:      "owner",
		Repo:       "repo",
		Title:      "Test PR",
		Body:       "Description",
		Head:       "feature-branch",
		Base:       "main",
		Draft:      true,
		Maintainer: true,
	}

	if req.Owner != "owner" {
		t.Errorf("Owner = %s, want owner", req.Owner)
	}
	if req.Draft != true {
		t.Error("Draft should be true")
	}
	if req.Maintainer != true {
		t.Error("Maintainer should be true")
	}
}

func TestPRResponse_JSON(t *testing.T) {
	jsonData := `{
		"id": 123,
		"number": 42,
		"html_url": "https://github.com/owner/repo/pull/42",
		"state": "open",
		"title": "Test PR",
		"created_at": "2024-01-01T00:00:00Z"
	}`

	var resp PRResponse
	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp.ID != 123 {
		t.Errorf("ID = %d, want 123", resp.ID)
	}
	if resp.Number != 42 {
		t.Errorf("Number = %d, want 42", resp.Number)
	}
	if resp.State != "open" {
		t.Errorf("State = %s, want open", resp.State)
	}
}

func TestBranchRequest_Fields(t *testing.T) {
	req := BranchRequest{
		Owner:  "owner",
		Repo:   "repo",
		Branch: "feature-branch",
		SHA:    "abc123",
	}

	if req.Branch != "feature-branch" {
		t.Errorf("Branch = %s, want feature-branch", req.Branch)
	}
	if req.SHA != "abc123" {
		t.Errorf("SHA = %s, want abc123", req.SHA)
	}
}

func TestRepoInfo_Fields(t *testing.T) {
	info := RepoInfo{
		Owner:    "owner",
		Name:     "repo",
		URL:      "https://github.com/owner/repo",
		CloneURL: "https://github.com/owner/repo.git",
		Branch:   "main",
	}

	if info.Owner != "owner" {
		t.Errorf("Owner = %s, want owner", info.Owner)
	}
	if info.Branch != "main" {
		t.Errorf("Branch = %s, want main", info.Branch)
	}
}

func TestCloneResult_Fields(t *testing.T) {
	result := CloneResult{
		Path:      "/tmp/repos/owner/repo",
		CommitSHA: "abc123def456",
		Branch:    "main",
	}

	if result.Path != "/tmp/repos/owner/repo" {
		t.Errorf("Path = %s, want /tmp/repos/owner/repo", result.Path)
	}
	if result.CommitSHA != "abc123def456" {
		t.Errorf("CommitSHA = %s, want abc123def456", result.CommitSHA)
	}
}

func TestPRTemplate_Fields(t *testing.T) {
	tmpl := PRTemplate{
		TestCount:     10,
		CoverageDelta: 5.5,
		Files:         []string{"test1.go", "test2.go"},
		Framework:     "go-test",
		Language:      "Go",
	}

	if tmpl.TestCount != 10 {
		t.Errorf("TestCount = %d, want 10", tmpl.TestCount)
	}
	if len(tmpl.Files) != 2 {
		t.Errorf("len(Files) = %d, want 2", len(tmpl.Files))
	}
}

// =============================================================================
// GeneratePRBody Edge Case Tests
// =============================================================================

func TestGeneratePRBody_ZeroCoverage(t *testing.T) {
	tmpl := PRTemplate{
		TestCount:     5,
		CoverageDelta: 0,
		Files:         []string{"test.go"},
		Framework:     "go-test",
		Language:      "Go",
	}

	body := GeneratePRBody(tmpl)

	// Should not contain coverage line when delta is 0
	if strings.Contains(body, "coverage improvement") {
		t.Error("Should not include coverage line when delta is 0")
	}
	if !strings.Contains(body, "5 generated tests") {
		t.Error("Should include test count")
	}
}

func TestGeneratePRBody_NegativeCoverage(t *testing.T) {
	tmpl := PRTemplate{
		TestCount:     3,
		CoverageDelta: -2.5,
		Files:         []string{"test.go"},
		Framework:     "pytest",
		Language:      "Python",
	}

	body := GeneratePRBody(tmpl)

	// Negative coverage should not be shown (only positive)
	if strings.Contains(body, "coverage improvement") {
		t.Error("Should not show negative coverage as improvement")
	}
}

func TestGeneratePRBody_EmptyFiles(t *testing.T) {
	tmpl := PRTemplate{
		TestCount:     0,
		CoverageDelta: 0,
		Files:         []string{},
		Framework:     "jest",
		Language:      "JavaScript",
	}

	body := GeneratePRBody(tmpl)

	if !strings.Contains(body, "## Test Files") {
		t.Error("Should include Test Files section")
	}
	if !strings.Contains(body, "0 generated tests") {
		t.Error("Should show 0 tests")
	}
}

func TestGeneratePRBody_ManyFiles(t *testing.T) {
	files := make([]string, 10)
	for i := 0; i < 10; i++ {
		files[i] = fmt.Sprintf("test_%d.go", i)
	}

	tmpl := PRTemplate{
		TestCount:     100,
		CoverageDelta: 15.5,
		Files:         files,
		Framework:     "go-test",
		Language:      "Go",
	}

	body := GeneratePRBody(tmpl)

	// Should contain all files
	for _, f := range files {
		if !strings.Contains(body, f) {
			t.Errorf("Should include file %s", f)
		}
	}
}

func TestGeneratePRBody_AllSections(t *testing.T) {
	tmpl := PRTemplate{
		TestCount:     10,
		CoverageDelta: 5.0,
		Files:         []string{"test.go"},
		Framework:     "go-test",
		Language:      "Go",
	}

	body := GeneratePRBody(tmpl)

	// Check all sections are present
	sections := []string{
		"## Summary",
		"## Test Files",
		"## Details",
		"## Checklist",
		"QTest",
	}

	for _, section := range sections {
		if !strings.Contains(body, section) {
			t.Errorf("Should include section: %s", section)
		}
	}
}

// =============================================================================
// base64Encode Edge Case Tests
// =============================================================================

func TestBase64Encode_SimpleString(t *testing.T) {
	result := base64Encode([]byte("hello"))
	expected := "aGVsbG8="

	if result != expected {
		t.Errorf("base64Encode(hello) = %s, want %s", result, expected)
	}
}

func TestBase64Encode_EmptyString(t *testing.T) {
	result := base64Encode([]byte(""))

	if result != "" {
		t.Errorf("base64Encode('') = %s, want empty string", result)
	}
}

func TestBase64Encode_SingleChar(t *testing.T) {
	result := base64Encode([]byte("a"))
	expected := "YQ=="

	if result != expected {
		t.Errorf("base64Encode(a) = %s, want %s", result, expected)
	}
}

func TestBase64Encode_TwoChars(t *testing.T) {
	result := base64Encode([]byte("ab"))
	expected := "YWI="

	if result != expected {
		t.Errorf("base64Encode(ab) = %s, want %s", result, expected)
	}
}

func TestBase64Encode_ThreeChars(t *testing.T) {
	result := base64Encode([]byte("abc"))
	expected := "YWJj"

	if result != expected {
		t.Errorf("base64Encode(abc) = %s, want %s", result, expected)
	}
}

func TestBase64Encode_SpecialChars(t *testing.T) {
	// Test with special characters
	result := base64Encode([]byte("hello\nworld"))

	if result == "" {
		t.Error("Should encode string with newlines")
	}
}

func TestBase64Encode_BinaryData(t *testing.T) {
	// Test with binary data
	data := []byte{0, 1, 2, 255, 254, 253}
	result := base64Encode(data)

	if result == "" {
		t.Error("Should encode binary data")
	}
}

func TestEncodeBase64_String(t *testing.T) {
	result := encodeBase64("test")
	expected := "dGVzdA=="

	if result != expected {
		t.Errorf("encodeBase64(test) = %s, want %s", result, expected)
	}
}

// =============================================================================
// ParseRepoURL Edge Case Tests
// =============================================================================

func TestParseRepoURL_TrailingSlash(t *testing.T) {
	info, err := ParseRepoURL("https://github.com/owner/repo/")
	if err != nil {
		t.Fatalf("ParseRepoURL error: %v", err)
	}

	if info.Owner != "owner" {
		t.Errorf("Owner = %s, want owner", info.Owner)
	}
	if info.Name != "repo" {
		t.Errorf("Name = %s, want repo", info.Name)
	}
}

func TestParseRepoURL_GitExtension(t *testing.T) {
	info, err := ParseRepoURL("https://github.com/owner/repo.git")
	if err != nil {
		t.Fatalf("ParseRepoURL error: %v", err)
	}

	if info.Name != "repo" {
		t.Errorf("Name = %s, want repo (without .git)", info.Name)
	}
}

func TestParseRepoURL_CaseSensitivity(t *testing.T) {
	info, err := ParseRepoURL("https://github.com/Owner/Repo")
	if err != nil {
		t.Fatalf("ParseRepoURL error: %v", err)
	}

	// Should preserve case
	if info.Owner != "Owner" {
		t.Errorf("Owner = %s, want Owner (case preserved)", info.Owner)
	}
	if info.Name != "Repo" {
		t.Errorf("Name = %s, want Repo (case preserved)", info.Name)
	}
}

func TestParseRepoURL_EmptyOwner(t *testing.T) {
	_, err := ParseRepoURL("https://github.com//repo")
	if err == nil {
		t.Error("Should return error for empty owner")
	}
}

func TestParseRepoURL_EmptyRepo(t *testing.T) {
	_, err := ParseRepoURL("https://github.com/owner/")
	if err == nil {
		t.Error("Should return error for empty repo")
	}
}

func TestParseRepoURL_TooManySegments(t *testing.T) {
	// URL with extra path segments should still work
	info, err := ParseRepoURL("https://github.com/owner/repo/tree/main")
	if err != nil {
		t.Fatalf("ParseRepoURL error: %v", err)
	}

	if info.Owner != "owner" {
		t.Errorf("Owner = %s, want owner", info.Owner)
	}
}
