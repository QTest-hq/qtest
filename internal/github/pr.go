package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// PRService handles GitHub Pull Request operations
type PRService struct {
	token   string
	client  *http.Client
	baseURL string
}

// NewPRService creates a new PR service
func NewPRService(token string) *PRService {
	return &PRService{
		token:   token,
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://api.github.com",
	}
}

// PRRequest represents a pull request creation request
type PRRequest struct {
	Owner      string
	Repo       string
	Title      string
	Body       string
	Head       string // Branch with changes
	Base       string // Target branch (e.g., main)
	Draft      bool
	Maintainer bool // Allow maintainer edits
}

// PRResponse represents a created pull request
type PRResponse struct {
	ID        int    `json:"id"`
	Number    int    `json:"number"`
	HTMLURL   string `json:"html_url"`
	State     string `json:"state"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
}

// BranchRequest represents a branch creation request
type BranchRequest struct {
	Owner  string
	Repo   string
	Branch string
	SHA    string // Base commit SHA
}

// CreateBranch creates a new branch from a commit SHA
func (s *PRService) CreateBranch(ctx context.Context, req BranchRequest) error {
	url := fmt.Sprintf("%s/repos/%s/%s/git/refs", s.baseURL, req.Owner, req.Repo)

	payload := map[string]string{
		"ref": "refs/heads/" + req.Branch,
		"sha": req.SHA,
	}

	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	s.setHeaders(httpReq)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 422 {
		// Branch already exists, that's OK
		return nil
	}

	if resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create branch: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// CreatePR creates a new pull request
func (s *PRService) CreatePR(ctx context.Context, req PRRequest) (*PRResponse, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls", s.baseURL, req.Owner, req.Repo)

	payload := map[string]interface{}{
		"title":                 req.Title,
		"body":                  req.Body,
		"head":                  req.Head,
		"base":                  req.Base,
		"draft":                 req.Draft,
		"maintainer_can_modify": req.Maintainer,
	}

	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	s.setHeaders(httpReq)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create PR: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 201 {
		// Check if PR already exists
		if resp.StatusCode == 422 && strings.Contains(string(respBody), "A pull request already exists") {
			// Try to find existing PR
			existingPR, err := s.FindPR(ctx, req.Owner, req.Repo, req.Head, req.Base)
			if err == nil && existingPR != nil {
				return existingPR, nil
			}
		}
		return nil, fmt.Errorf("failed to create PR: %s - %s", resp.Status, string(respBody))
	}

	var prResp PRResponse
	if err := json.Unmarshal(respBody, &prResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &prResp, nil
}

// FindPR finds an existing PR for the given head and base branches
func (s *PRService) FindPR(ctx context.Context, owner, repo, head, base string) (*PRResponse, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls?head=%s:%s&base=%s&state=open",
		s.baseURL, owner, repo, owner, head, base)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	s.setHeaders(httpReq)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to find PR: %s", resp.Status)
	}

	var prs []PRResponse
	if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil {
		return nil, err
	}

	if len(prs) == 0 {
		return nil, nil
	}

	return &prs[0], nil
}

// CommitFile commits a single file to a branch
func (s *PRService) CommitFile(ctx context.Context, owner, repo, branch, path, content, message string) error {
	// Get the current file SHA if it exists
	existingSHA, _ := s.getFileSHA(ctx, owner, repo, branch, path)

	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s", s.baseURL, owner, repo, path)

	payload := map[string]interface{}{
		"message": message,
		"content": encodeBase64(content),
		"branch":  branch,
	}

	if existingSHA != "" {
		payload["sha"] = existingSHA
	}

	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	s.setHeaders(httpReq)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to commit file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to commit file: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// getFileSHA gets the SHA of an existing file
func (s *PRService) getFileSHA(ctx context.Context, owner, repo, branch, path string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s", s.baseURL, owner, repo, path, branch)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	s.setHeaders(httpReq)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", nil
	}

	var fileInfo struct {
		SHA string `json:"sha"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&fileInfo); err != nil {
		return "", err
	}

	return fileInfo.SHA, nil
}

// GetDefaultBranch gets the default branch of a repository
func (s *PRService) GetDefaultBranch(ctx context.Context, owner, repo string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", s.baseURL, owner, repo)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	s.setHeaders(httpReq)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "main", nil // Default fallback
	}

	var repoInfo struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&repoInfo); err != nil {
		return "main", nil
	}

	return repoInfo.DefaultBranch, nil
}

// GetLatestCommitSHA gets the latest commit SHA for a branch
func (s *PRService) GetLatestCommitSHA(ctx context.Context, owner, repo, branch string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/git/refs/heads/%s", s.baseURL, owner, repo, branch)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	s.setHeaders(httpReq)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to get branch: %s", resp.Status)
	}

	var refInfo struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&refInfo); err != nil {
		return "", err
	}

	return refInfo.Object.SHA, nil
}

func (s *PRService) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")
}

func encodeBase64(content string) string {
	return base64Encode([]byte(content))
}

func base64Encode(data []byte) string {
	const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var result strings.Builder

	for i := 0; i < len(data); i += 3 {
		var n uint32
		remaining := len(data) - i

		n = uint32(data[i]) << 16
		if remaining > 1 {
			n |= uint32(data[i+1]) << 8
		}
		if remaining > 2 {
			n |= uint32(data[i+2])
		}

		result.WriteByte(base64Chars[(n>>18)&0x3F])
		result.WriteByte(base64Chars[(n>>12)&0x3F])

		if remaining > 1 {
			result.WriteByte(base64Chars[(n>>6)&0x3F])
		} else {
			result.WriteByte('=')
		}

		if remaining > 2 {
			result.WriteByte(base64Chars[n&0x3F])
		} else {
			result.WriteByte('=')
		}
	}

	return result.String()
}

// PRTemplate generates a standard PR body for test generation
type PRTemplate struct {
	TestCount     int
	CoverageDelta float64
	Files         []string
	Framework     string
	Language      string
}

// GeneratePRBody generates the PR description body
func GeneratePRBody(tmpl PRTemplate) string {
	var sb strings.Builder

	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("This PR adds **%d generated tests** using QTest.\n\n", tmpl.TestCount))

	if tmpl.CoverageDelta > 0 {
		sb.WriteString(fmt.Sprintf("Estimated coverage improvement: **+%.1f%%**\n\n", tmpl.CoverageDelta))
	}

	sb.WriteString("## Test Files\n\n")
	for _, f := range tmpl.Files {
		sb.WriteString(fmt.Sprintf("- `%s`\n", f))
	}

	sb.WriteString("\n## Details\n\n")
	sb.WriteString(fmt.Sprintf("- **Language**: %s\n", tmpl.Language))
	sb.WriteString(fmt.Sprintf("- **Framework**: %s\n", tmpl.Framework))
	sb.WriteString("- **Generated by**: [QTest](https://github.com/QTest-hq/qtest)\n\n")

	sb.WriteString("## Checklist\n\n")
	sb.WriteString("- [ ] Tests pass locally\n")
	sb.WriteString("- [ ] Coverage meets target\n")
	sb.WriteString("- [ ] No flaky tests\n\n")

	sb.WriteString("---\n")
	sb.WriteString("*This PR was automatically generated by QTest*\n")

	return sb.String()
}
