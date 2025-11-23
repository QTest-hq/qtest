package maintenance

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPRCreator(t *testing.T) {
	config := DefaultPRCreatorConfig()
	creator := NewPRCreator(config)
	assert.NotNil(t, creator)
}

func TestDefaultPRCreatorConfig(t *testing.T) {
	config := DefaultPRCreatorConfig()
	assert.Equal(t, "https://api.github.com", config.BaseURL)
	assert.False(t, config.DryRun)
	assert.Equal(t, "qtest/maintenance", config.BranchPrefix)
	assert.False(t, config.AutoMerge)
	assert.Contains(t, config.Labels, "qtest")
	assert.Contains(t, config.Labels, "automated")
}

func TestGenerateBranchName(t *testing.T) {
	config := DefaultPRCreatorConfig()
	creator := NewPRCreator(config)

	req := PRRequest{
		Owner: "testowner",
		Repo:  "testrepo",
	}

	branch := creator.generateBranchName(req)

	assert.True(t, len(branch) > 0)
	assert.Contains(t, branch, "qtest/maintenance/")
}

func TestGeneratePRBody_BasicFiles(t *testing.T) {
	creator := NewPRCreator(DefaultPRCreatorConfig())

	req := PRRequest{
		Files: []FileChange{
			{Path: "test_add.go", Action: "create"},
			{Path: "test_sub.go", Action: "update"},
			{Path: "test_old.go", Action: "delete"},
		},
	}

	body := creator.generatePRBody(req)

	assert.Contains(t, body, "Test Maintenance Update")
	assert.Contains(t, body, "**Tests Created:** 1")
	assert.Contains(t, body, "**Tests Updated:** 1")
	assert.Contains(t, body, "**Tests Removed:** 1")
	assert.Contains(t, body, "test_add.go")
	assert.Contains(t, body, "test_sub.go")
	assert.Contains(t, body, "test_old.go")
	assert.Contains(t, body, "✨") // Create icon
	assert.Contains(t, body, "📝") // Update icon
	assert.Contains(t, body, "🗑️") // Delete icon
}

func TestGeneratePRBody_WithResults(t *testing.T) {
	creator := NewPRCreator(DefaultPRCreatorConfig())

	req := PRRequest{
		Files: []FileChange{
			{Path: "test.go", Action: "create"},
		},
		Results: []PRResultItem{
			{Type: "creation", File: "test.go", Success: true, Message: "Created tests"},
			{Type: "removal", File: "old.go", Success: false, Message: "File not found"},
		},
	}

	body := creator.generatePRBody(req)

	assert.Contains(t, body, "Maintenance Results")
	assert.Contains(t, body, "creation")
	assert.Contains(t, body, "removal")
	assert.Contains(t, body, "✅")
	assert.Contains(t, body, "❌")
}

func TestCreateMaintenancePR_NoFiles(t *testing.T) {
	config := DefaultPRCreatorConfig()
	config.DryRun = true
	creator := NewPRCreator(config)

	req := PRRequest{
		Owner:      "owner",
		Repo:       "repo",
		BaseBranch: "main",
		Files:      []FileChange{},
	}

	_, err := creator.CreateMaintenancePR(context.Background(), req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no files")
}

func TestCreateMaintenancePR_DryRun(t *testing.T) {
	config := DefaultPRCreatorConfig()
	config.DryRun = true
	creator := NewPRCreator(config)

	req := PRRequest{
		Owner:      "testowner",
		Repo:       "testrepo",
		BaseBranch: "main",
		Title:      "Test PR",
		Files: []FileChange{
			{Path: "test.go", Content: "package test", Action: "create"},
		},
	}

	resp, err := creator.CreateMaintenancePR(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.DryRun)
	assert.Equal(t, "dry_run", resp.State)
	assert.Equal(t, "Test PR", resp.Title)
	assert.Contains(t, resp.HTMLURL, "testowner")
	assert.Contains(t, resp.HTMLURL, "testrepo")
}

func TestBase64Encode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"a", "YQ=="},
		{"ab", "YWI="},
		{"abc", "YWJj"},
		{"Hello", "SGVsbG8="},
		{"Hello World", "SGVsbG8gV29ybGQ="},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := base64Encode(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateMaintenancePRFromResults_NoChanges(t *testing.T) {
	config := DefaultPRCreatorConfig()
	config.DryRun = true
	creator := NewPRCreator(config)

	// Empty results
	removalResults := []RemovalResult{}
	regenResults := []RegenerationResult{}

	_, err := creator.CreateMaintenancePRFromResults(
		context.Background(),
		"owner", "repo", "main",
		removalResults, regenResults,
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no changes")
}

func TestCreateMaintenancePRFromResults_WithRemovals(t *testing.T) {
	config := DefaultPRCreatorConfig()
	config.DryRun = true
	creator := NewPRCreator(config)

	removalResults := []RemovalResult{
		{TargetID: "fn1", TestFile: "test_calc.go", Removed: true},
		{TargetID: "fn2", TestFile: "test_math.go", Removed: false, Error: "not found"},
	}
	regenResults := []RegenerationResult{}

	resp, err := creator.CreateMaintenancePRFromResults(
		context.Background(),
		"owner", "repo", "main",
		removalResults, regenResults,
	)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.DryRun)
}

func TestCreateMaintenancePRFromResults_WithRegenerations(t *testing.T) {
	config := DefaultPRCreatorConfig()
	config.DryRun = true
	creator := NewPRCreator(config)

	removalResults := []RemovalResult{}
	regenResults := []RegenerationResult{
		{TargetID: "fn1", TestFile: "test_calc.go", Success: true, TestsCreated: 3},
		{TargetID: "fn2", TestFile: "test_math.go", Success: false, Error: "generator failed"},
	}

	resp, err := creator.CreateMaintenancePRFromResults(
		context.Background(),
		"owner", "repo", "main",
		removalResults, regenResults,
	)

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestCreateMaintenancePRFromResults_Mixed(t *testing.T) {
	config := DefaultPRCreatorConfig()
	config.DryRun = true
	creator := NewPRCreator(config)

	removalResults := []RemovalResult{
		{TargetID: "fn-old", TestFile: "test_old.go", Removed: true},
	}
	regenResults := []RegenerationResult{
		{TargetID: "fn-new", TestFile: "test_new.go", Success: true, TestsCreated: 2},
	}

	resp, err := creator.CreateMaintenancePRFromResults(
		context.Background(),
		"owner", "repo", "main",
		removalResults, regenResults,
	)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Contains(t, resp.Title, "maintenance update")
}

func TestPRRequest_Structure(t *testing.T) {
	req := PRRequest{
		Owner:      "testowner",
		Repo:       "testrepo",
		BaseBranch: "main",
		Title:      "Test PR",
		Body:       "PR body",
		Files: []FileChange{
			{Path: "test.go", Content: "content", Action: "create"},
		},
		Results: []PRResultItem{
			{Type: "test", File: "test.go", Success: true, Message: "ok"},
		},
	}

	assert.Equal(t, "testowner", req.Owner)
	assert.Equal(t, "testrepo", req.Repo)
	assert.Equal(t, 1, len(req.Files))
	assert.Equal(t, 1, len(req.Results))
}

func TestPRResponse_Structure(t *testing.T) {
	resp := PRResponse{
		Number:    123,
		URL:       "https://api.github.com/repos/o/r/pulls/123",
		HTMLURL:   "https://github.com/o/r/pull/123",
		State:     "open",
		Title:     "Test PR",
		CreatedAt: time.Now(),
		Branch:    "feature/test",
		DryRun:    false,
	}

	assert.Equal(t, 123, resp.Number)
	assert.Equal(t, "open", resp.State)
	assert.False(t, resp.DryRun)
}

func TestFileChange_Actions(t *testing.T) {
	tests := []struct {
		action   string
		expected string
	}{
		{"create", "create"},
		{"update", "update"},
		{"delete", "delete"},
	}

	for _, tt := range tests {
		fc := FileChange{
			Path:    "test.go",
			Content: "content",
			Action:  tt.action,
		}
		assert.Equal(t, tt.expected, fc.Action)
	}
}

func TestGeneratePRBody_OnlyCreated(t *testing.T) {
	creator := NewPRCreator(DefaultPRCreatorConfig())

	req := PRRequest{
		Files: []FileChange{
			{Path: "test1.go", Action: "create"},
			{Path: "test2.go", Action: "create"},
		},
	}

	body := creator.generatePRBody(req)

	assert.Contains(t, body, "**Tests Created:** 2")
	assert.Contains(t, body, "**Tests Updated:** 0")
	assert.Contains(t, body, "**Tests Removed:** 0")
}

func TestGeneratePRBody_Footer(t *testing.T) {
	creator := NewPRCreator(DefaultPRCreatorConfig())

	req := PRRequest{
		Files: []FileChange{
			{Path: "test.go", Action: "create"},
		},
	}

	body := creator.generatePRBody(req)

	assert.Contains(t, body, "Generated by")
	assert.Contains(t, body, "QTest")
}
