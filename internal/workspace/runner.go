package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QTest-hq/qtest/internal/adapters"
	"github.com/QTest-hq/qtest/internal/config"
	"github.com/QTest-hq/qtest/internal/github"
	"github.com/QTest-hq/qtest/internal/llm"
	"github.com/QTest-hq/qtest/internal/parser"
	"github.com/QTest-hq/qtest/pkg/dsl"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// Runner orchestrates incremental test generation
type Runner struct {
	ws          *Workspace
	git         *GitManager
	parser      *parser.Parser
	llmRouter   *llm.Router
	adapters    *adapters.Registry
	artifacts   *ArtifactManager
	cfg         *RunConfig
	projectCfg  *config.ProjectConfig
	startTime   time.Time

	// Callbacks for progress reporting
	OnProgress func(current, total int, target *TargetState)
	OnComplete func(target *TargetState, testFile string)
	OnError    func(target *TargetState, err error)
}

// RunConfig holds configuration for a generation run
type RunConfig struct {
	Tier            llm.Tier
	CommitEach      bool     // Commit after each test
	BranchName      string   // Branch for tests
	TestDir         string   // Directory for test files
	DryRun          bool     // Don't write files
	MaxConcurrent   int      // Max parallel generations
	FilePatterns    []string // Files to include
	ValidateTests   bool     // Run tests after generation
	MaxTests        int      // Max tests to generate (0=unlimited)
	CreatePR        bool     // Create a PR after generation
	PRDraft         bool     // Create PR as draft
	PRTitle         string   // Custom PR title
	GitHubOwner     string   // GitHub repo owner
	GitHubRepo      string   // GitHub repo name
}

// DefaultRunConfig returns sensible defaults
func DefaultRunConfig() *RunConfig {
	return &RunConfig{
		Tier:          llm.Tier2,
		CommitEach:    true,
		BranchName:    "qtest/generated-tests",
		TestDir:       "", // Same directory as source
		DryRun:        false,
		MaxConcurrent: 1, // Sequential for now
		FilePatterns:  []string{"*.go", "*.py", "*.ts", "*.js"},
	}
}

// NewRunner creates a new runner
func NewRunner(ws *Workspace, llmRouter *llm.Router, gitToken string, cfg *RunConfig) *Runner {
	if cfg == nil {
		cfg = DefaultRunConfig()
	}

	return &Runner{
		ws:        ws,
		git:       NewGitManager(ws, gitToken),
		parser:    parser.NewParser(),
		llmRouter: llmRouter,
		adapters:  adapters.NewRegistry(),
		artifacts: NewArtifactManager(ws),
		cfg:       cfg,
	}
}

// Initialize prepares the workspace (clone + parse)
func (r *Runner) Initialize(ctx context.Context) error {
	// Clone if not already cloned
	if _, err := os.Stat(r.ws.RepoPath); os.IsNotExist(err) {
		if err := r.git.Clone(ctx); err != nil {
			return fmt.Errorf("clone failed: %w", err)
		}
	}

	// Load project config from repo
	projectCfg, err := config.LoadProjectConfig(r.ws.RepoPath)
	if err != nil {
		log.Warn().Err(err).Msg("failed to load project config, using defaults")
		projectCfg = config.DefaultProjectConfig()
	}
	r.projectCfg = projectCfg

	// Apply project config to run config
	r.applyProjectConfig()

	// Create test branch if configured
	if r.cfg.BranchName != "" && r.ws.Branch == "" {
		if err := r.git.CreateTestBranch(r.cfg.BranchName); err != nil {
			log.Warn().Err(err).Msg("failed to create branch, continuing on current branch")
		}
	}

	// Parse the repository
	if err := r.parse(ctx); err != nil {
		return fmt.Errorf("parse failed: %w", err)
	}

	// Generate test plan artifact
	if _, err := r.artifacts.GenerateTestPlan(); err != nil {
		log.Warn().Err(err).Msg("failed to generate test plan artifact")
	}

	return nil
}

// applyProjectConfig applies project config settings to run config
func (r *Runner) applyProjectConfig() {
	if r.projectCfg == nil {
		return
	}

	// Apply tier if specified in project config
	if r.projectCfg.Generation.Tier != 0 {
		r.cfg.Tier = llm.Tier(r.projectCfg.Generation.Tier)
	}

	// Apply test directory if specified
	if r.projectCfg.Framework.TestDir != "" {
		r.cfg.TestDir = r.projectCfg.Framework.TestDir
	}

	// Apply file patterns
	if len(r.projectCfg.Include) > 0 {
		r.cfg.FilePatterns = r.projectCfg.Include
	}

	// Apply language override
	if r.projectCfg.Language != "" {
		r.ws.Language = r.projectCfg.Language
	}

	log.Debug().
		Int("tier", int(r.cfg.Tier)).
		Str("testDir", r.cfg.TestDir).
		Strs("patterns", r.cfg.FilePatterns).
		Msg("applied project config")
}

// parse discovers all testable functions
func (r *Runner) parse(ctx context.Context) error {
	r.ws.SetPhase(PhaseParsing)
	log.Info().Str("path", r.ws.RepoPath).Msg("parsing repository")

	// Find source files
	var files []string
	for _, pattern := range r.cfg.FilePatterns {
		matches, _ := filepath.Glob(filepath.Join(r.ws.RepoPath, "**", pattern))
		files = append(files, matches...)

		// Also check root level
		rootMatches, _ := filepath.Glob(filepath.Join(r.ws.RepoPath, pattern))
		files = append(files, rootMatches...)
	}

	// Walk directory for more thorough search
	filepath.Walk(r.ws.RepoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Skip common ignore patterns
		if strings.Contains(path, "node_modules") ||
			strings.Contains(path, "vendor") ||
			strings.Contains(path, ".git") ||
			strings.Contains(path, "__pycache__") {
			return nil
		}

		// Skip test files
		base := filepath.Base(path)
		if strings.Contains(base, "_test.") || strings.Contains(base, ".test.") || strings.HasPrefix(base, "test_") {
			return nil
		}

		// Check extension
		ext := filepath.Ext(path)
		for _, pattern := range r.cfg.FilePatterns {
			if strings.HasSuffix(pattern, ext) {
				files = append(files, path)
				break
			}
		}

		return nil
	})

	// Deduplicate
	seen := make(map[string]bool)
	uniqueFiles := make([]string, 0)
	for _, f := range files {
		if !seen[f] {
			seen[f] = true
			uniqueFiles = append(uniqueFiles, f)
		}
	}

	log.Info().Int("files", len(uniqueFiles)).Msg("found source files")

	// Parse each file
	for _, file := range uniqueFiles {
		parsed, err := r.parser.ParseFile(ctx, file)
		if err != nil {
			log.Debug().Err(err).Str("file", file).Msg("skipping file")
			continue
		}

		// Detect language for the workspace
		if r.ws.Language == "" {
			r.ws.Language = string(parsed.Language)
		}

		// Add functions as targets
		r.ws.AddTargets(parsed.Functions, file)

		log.Debug().
			Str("file", file).
			Int("functions", len(parsed.Functions)).
			Msg("parsed file")
	}

	r.ws.SetPhase(PhasePlanning)
	log.Info().Int("targets", r.ws.State.TotalTargets).Msg("found testable targets")

	return r.ws.Save()
}

// Run executes the incremental generation
func (r *Runner) Run(ctx context.Context) error {
	r.ws.SetPhase(PhaseGenerating)
	now := time.Now()
	r.ws.State.StartedAt = &now
	r.startTime = now

	processed := 0
	total := r.ws.State.TotalTargets

	for {
		select {
		case <-ctx.Done():
			r.ws.SetPhase(PhasePaused)
			return r.ws.Save()
		default:
		}

		// Get next target
		target := r.ws.GetNextTarget()
		if target == nil {
			break // All done
		}

		processed++
		log.Info().
			Str("target", target.Name).
			Str("file", target.File).
			Int("progress", processed).
			Int("total", total).
			Msg("generating test")

		// Callback for progress
		if r.OnProgress != nil {
			r.OnProgress(processed, total, target)
		}

		// Mark as running
		r.ws.UpdateTarget(target.ID, StatusRunning, "", nil)

		// Generate test
		testFile, err := r.generateTest(ctx, target)
		if err != nil {
			log.Warn().Err(err).Str("target", target.Name).Msg("generation failed")
			r.ws.UpdateTarget(target.ID, StatusFailed, "", err)
			if r.OnError != nil {
				r.OnError(target, err)
			}
			continue
		}

		// Update target
		r.ws.UpdateTarget(target.ID, StatusCompleted, testFile, nil)

		// Commit if configured
		if r.cfg.CommitEach && testFile != "" && !r.cfg.DryRun {
			commitSHA, err := r.git.CommitTest(testFile, target.Name)
			if err != nil {
				log.Warn().Err(err).Msg("failed to commit test")
			} else {
				target.CommitSHA = commitSHA
			}
		}

		// Callback for completion
		if r.OnComplete != nil {
			r.OnComplete(target, testFile)
		}

		// Save state after each test
		if err := r.ws.Save(); err != nil {
			log.Warn().Err(err).Msg("failed to save workspace state")
		}
	}

	r.ws.SetPhase(PhaseCompleted)

	// Validate tests if configured
	if r.cfg.ValidateTests && !r.cfg.DryRun {
		log.Info().Msg("validating generated tests")
		validator := NewTestValidator(r.ws)
		if _, err := validator.ValidateAll(ctx); err != nil {
			log.Warn().Err(err).Msg("test validation failed")
		}
	}

	// Generate summary artifact
	if _, err := r.artifacts.GenerateSummary(r.startTime); err != nil {
		log.Warn().Err(err).Msg("failed to generate summary artifact")
	}

	return r.ws.Save()
}

// generateTest generates a test for a single target
func (r *Runner) generateTest(ctx context.Context, target *TargetState) (string, error) {
	// Read the source file
	content, err := os.ReadFile(target.File)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Parse to get function details
	lang := parser.DetectLanguage(target.File)
	parsed, err := r.parser.ParseContent(ctx, target.File, string(content), lang)
	if err != nil {
		return "", fmt.Errorf("failed to parse file: %w", err)
	}

	// Find the target function
	var targetFn *parser.Function
	for _, fn := range parsed.Functions {
		if fn.Name == target.Name && fn.StartLine == target.Line {
			targetFn = &fn
			break
		}
	}

	if targetFn == nil {
		return "", fmt.Errorf("function not found: %s", target.Name)
	}

	// Extract function code
	lines := strings.Split(string(content), "\n")
	var funcCode strings.Builder
	for i := targetFn.StartLine - 1; i < targetFn.EndLine && i < len(lines); i++ {
		funcCode.WriteString(lines[i])
		funcCode.WriteString("\n")
	}

	// Create prompt and generate
	prompt := llm.TestGenerationPrompt(
		funcCode.String(),
		targetFn.Name,
		target.File,
		string(lang),
		"",
	)

	resp, err := r.llmRouter.Complete(ctx, &llm.Request{
		Tier:        r.cfg.Tier,
		System:      llm.SystemPromptTestGeneration,
		Messages:    []llm.Message{{Role: "user", Content: prompt}},
		Temperature: 0.3,
		MaxTokens:   2000,
	})
	if err != nil {
		return "", fmt.Errorf("LLM error: %w", err)
	}

	// Parse DSL
	yamlContent := llm.ParseDSLOutput(resp.Content)
	var testDSL dsl.TestDSL
	if err := yaml.Unmarshal([]byte(yamlContent), &testDSL); err != nil {
		return "", fmt.Errorf("invalid DSL: %w", err)
	}

	// Store DSL in target
	dslJSON, _ := yaml.Marshal(testDSL)
	target.DSL = dslJSON

	if r.cfg.DryRun {
		return "", nil
	}

	// Get adapter for language
	adapter, err := r.adapters.GetForLanguage(lang)
	if err != nil {
		// Just save DSL if no adapter
		return r.saveDSL(target, yamlContent)
	}

	// Generate test code
	testCode, err := adapter.Generate(&testDSL)
	if err != nil {
		// Fall back to DSL
		return r.saveDSL(target, yamlContent)
	}

	// Write test file
	testFile := r.getTestFilePath(target.File, adapter)
	if err := os.MkdirAll(filepath.Dir(testFile), 0755); err != nil {
		return "", err
	}

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		return "", err
	}

	return testFile, nil
}

// saveDSL saves just the DSL when no adapter is available
func (r *Runner) saveDSL(target *TargetState, yamlContent string) (string, error) {
	dslFile := r.getTestFilePath(target.File, nil) + ".yaml"
	if err := os.MkdirAll(filepath.Dir(dslFile), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(dslFile, []byte(yamlContent), 0644); err != nil {
		return "", err
	}
	return dslFile, nil
}

// getTestFilePath returns the path for a test file
func (r *Runner) getTestFilePath(sourceFile string, adapter adapters.Adapter) string {
	dir := filepath.Dir(sourceFile)
	base := filepath.Base(sourceFile)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	if r.cfg.TestDir != "" {
		// Use custom test directory
		relDir, _ := filepath.Rel(r.ws.RepoPath, dir)
		dir = filepath.Join(r.ws.RepoPath, r.cfg.TestDir, relDir)
	}

	if adapter != nil {
		return filepath.Join(dir, name+adapter.TestFileSuffix()+adapter.FileExtension())
	}

	// Default: source_test.yaml
	return filepath.Join(dir, name+"_test")
}

// Resume continues a paused run
func (r *Runner) Resume(ctx context.Context) error {
	if r.ws.State.Phase != PhasePaused {
		return fmt.Errorf("workspace not paused")
	}
	return r.Run(ctx)
}

// Pause pauses the current run
func (r *Runner) Pause() {
	r.ws.SetPhase(PhasePaused)
	now := time.Now()
	r.ws.State.PausedAt = &now
	r.ws.Save()
}

// CreatePR creates a GitHub pull request with the generated tests
func (r *Runner) CreatePR(ctx context.Context) (*github.PRResponse, error) {
	if r.git.token == "" {
		return nil, fmt.Errorf("GitHub token required for PR creation")
	}

	// Determine owner/repo
	owner := r.cfg.GitHubOwner
	repo := r.cfg.GitHubRepo

	if owner == "" || repo == "" {
		// Try to parse from repo URL
		repoInfo, err := github.ParseRepoURL(r.ws.RepoURL)
		if err != nil {
			return nil, fmt.Errorf("could not determine GitHub owner/repo: %w", err)
		}
		owner = repoInfo.Owner
		repo = repoInfo.Name
	}

	log.Info().
		Str("owner", owner).
		Str("repo", repo).
		Str("branch", r.cfg.BranchName).
		Msg("creating pull request")

	// Push changes first
	if err := r.git.Push(ctx); err != nil {
		return nil, fmt.Errorf("failed to push changes: %w", err)
	}

	// Create PR service
	prService := github.NewPRService(r.git.token)

	// Get default branch if not set
	base := r.ws.BaseBranch
	if base == "" {
		defaultBranch, err := prService.GetDefaultBranch(ctx, owner, repo)
		if err != nil {
			base = "main"
		} else {
			base = defaultBranch
		}
	}

	// Count completed tests
	completedCount := 0
	var testFiles []string
	for _, target := range r.ws.State.Targets {
		if target.Status == StatusCompleted && target.TestFile != "" {
			completedCount++
			// Get relative path
			relPath, _ := filepath.Rel(r.ws.RepoPath, target.TestFile)
			if relPath != "" {
				testFiles = append(testFiles, relPath)
			}
		}
	}

	if completedCount == 0 {
		return nil, fmt.Errorf("no tests to include in PR")
	}

	// Generate PR title
	title := r.cfg.PRTitle
	if title == "" {
		title = fmt.Sprintf("Add %d generated tests", completedCount)
	}

	// Generate PR body
	body := github.GeneratePRBody(github.PRTemplate{
		TestCount: completedCount,
		Files:     testFiles,
		Framework: r.detectFramework(),
		Language:  r.ws.Language,
	})

	// Create the PR
	pr, err := prService.CreatePR(ctx, github.PRRequest{
		Owner:      owner,
		Repo:       repo,
		Title:      title,
		Body:       body,
		Head:       r.cfg.BranchName,
		Base:       base,
		Draft:      r.cfg.PRDraft,
		Maintainer: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create PR: %w", err)
	}

	log.Info().
		Int("number", pr.Number).
		Str("url", pr.HTMLURL).
		Msg("pull request created")

	return pr, nil
}

// detectFramework returns the detected test framework
func (r *Runner) detectFramework() string {
	switch r.ws.Language {
	case "go":
		return "Go testing"
	case "python":
		return "pytest"
	case "javascript", "typescript":
		return "Jest"
	case "java":
		return "JUnit 5"
	case "ruby":
		return "RSpec"
	default:
		return "unknown"
	}
}

// PRResult holds the result of PR creation
type PRResult struct {
	Number  int
	URL     string
	Title   string
	Created bool
}
