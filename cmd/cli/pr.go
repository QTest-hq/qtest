package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/QTest-hq/qtest/internal/github"
	"github.com/spf13/cobra"
)

func prCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pr",
		Short: "Create pull requests with generated tests",
		Long:  `Create GitHub pull requests containing generated test files`,
	}

	cmd.AddCommand(prCreateCmd())

	return cmd
}

func prCreateCmd() *cobra.Command {
	var (
		owner     string
		repo      string
		branch    string
		base      string
		title     string
		testFiles []string
		testDir   string
		draft     bool
		token     string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a PR with test files",
		Long:  `Create a GitHub pull request containing the specified test files`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// Get token from flag or environment
			if token == "" {
				token = os.Getenv("GITHUB_TOKEN")
			}
			if token == "" {
				return fmt.Errorf("GitHub token required. Set GITHUB_TOKEN env var or use --token flag")
			}

			// Validate owner and repo
			if owner == "" || repo == "" {
				// Try to detect from git remote
				detected, err := detectGitHubRepo()
				if err != nil {
					return fmt.Errorf("could not detect repo, please specify --owner and --repo")
				}
				owner = detected.Owner
				repo = detected.Name
			}

			fmt.Printf("Creating PR for %s/%s\n", owner, repo)
			fmt.Printf("Branch: %s -> %s\n\n", branch, base)

			// Create PR service
			prService := github.NewPRService(token)

			// Get default branch if not specified
			if base == "" {
				defaultBranch, err := prService.GetDefaultBranch(ctx, owner, repo)
				if err != nil {
					base = "main"
				} else {
					base = defaultBranch
				}
			}

			// Get latest commit SHA from base branch
			baseSHA, err := prService.GetLatestCommitSHA(ctx, owner, repo, base)
			if err != nil {
				return fmt.Errorf("failed to get base branch SHA: %w", err)
			}

			fmt.Printf("Base branch %s at %s\n", base, baseSHA[:8])

			// Create test branch
			fmt.Printf("Creating branch %s...\n", branch)
			err = prService.CreateBranch(ctx, github.BranchRequest{
				Owner:  owner,
				Repo:   repo,
				Branch: branch,
				SHA:    baseSHA,
			})
			if err != nil {
				return fmt.Errorf("failed to create branch: %w", err)
			}

			// Collect test files
			var filesToCommit []string
			if testDir != "" {
				// Find all test files in directory
				err := filepath.Walk(testDir, func(path string, info os.FileInfo, err error) error {
					if err != nil || info.IsDir() {
						return err
					}
					if isTestFile(path) {
						filesToCommit = append(filesToCommit, path)
					}
					return nil
				})
				if err != nil {
					return fmt.Errorf("failed to walk test directory: %w", err)
				}
			}
			filesToCommit = append(filesToCommit, testFiles...)

			if len(filesToCommit) == 0 {
				return fmt.Errorf("no test files to commit")
			}

			fmt.Printf("Committing %d test files...\n", len(filesToCommit))

			// Commit each file
			var committedFiles []string
			for _, file := range filesToCommit {
				content, err := os.ReadFile(file)
				if err != nil {
					fmt.Printf("  Warning: could not read %s: %v\n", file, err)
					continue
				}

				// Determine repo-relative path
				repoPath := filepath.Base(file)
				if testDir != "" {
					relPath, _ := filepath.Rel(testDir, file)
					repoPath = filepath.Join("tests", relPath)
				}

				fmt.Printf("  Committing %s...\n", repoPath)

				err = prService.CommitFile(ctx, owner, repo, branch, repoPath, string(content),
					fmt.Sprintf("Add generated test: %s", filepath.Base(file)))
				if err != nil {
					fmt.Printf("  Warning: could not commit %s: %v\n", file, err)
					continue
				}

				committedFiles = append(committedFiles, repoPath)
			}

			if len(committedFiles) == 0 {
				return fmt.Errorf("no files were committed")
			}

			// Generate PR title if not specified
			if title == "" {
				title = fmt.Sprintf("Add %d generated tests", len(committedFiles))
			}

			// Generate PR body
			body := github.GeneratePRBody(github.PRTemplate{
				TestCount: len(committedFiles),
				Files:     committedFiles,
				Framework: detectTestFramework(committedFiles),
				Language:  detectTestLanguage(committedFiles),
			})

			// Create PR
			fmt.Printf("\nCreating pull request...\n")
			pr, err := prService.CreatePR(ctx, github.PRRequest{
				Owner:      owner,
				Repo:       repo,
				Title:      title,
				Body:       body,
				Head:       branch,
				Base:       base,
				Draft:      draft,
				Maintainer: true,
			})
			if err != nil {
				return fmt.Errorf("failed to create PR: %w", err)
			}

			fmt.Printf("\n✅ Pull request created!\n")
			fmt.Printf("   PR #%d: %s\n", pr.Number, pr.Title)
			fmt.Printf("   URL: %s\n", pr.HTMLURL)

			return nil
		},
	}

	cmd.Flags().StringVar(&owner, "owner", "", "GitHub repository owner")
	cmd.Flags().StringVar(&repo, "repo", "", "GitHub repository name")
	cmd.Flags().StringVarP(&branch, "branch", "b", "qtest/generated-tests", "Branch name for tests")
	cmd.Flags().StringVar(&base, "base", "", "Base branch (default: repo default)")
	cmd.Flags().StringVarP(&title, "title", "t", "", "PR title")
	cmd.Flags().StringSliceVarP(&testFiles, "files", "f", nil, "Test files to include")
	cmd.Flags().StringVarP(&testDir, "dir", "d", "", "Directory containing test files")
	cmd.Flags().BoolVar(&draft, "draft", false, "Create as draft PR")
	cmd.Flags().StringVar(&token, "token", "", "GitHub token (or set GITHUB_TOKEN)")

	return cmd
}

// detectGitHubRepo tries to detect the GitHub repo from git remote
func detectGitHubRepo() (*github.RepoInfo, error) {
	// Read git config
	gitConfig, err := os.ReadFile(".git/config")
	if err != nil {
		return nil, err
	}

	// Find remote URL
	lines := strings.Split(string(gitConfig), "\n")
	for i, line := range lines {
		if strings.Contains(line, "[remote \"origin\"]") {
			for j := i + 1; j < len(lines); j++ {
				if strings.Contains(lines[j], "url = ") {
					url := strings.TrimSpace(strings.TrimPrefix(lines[j], "\turl = "))
					return github.ParseRepoURL(url)
				}
				if strings.HasPrefix(lines[j], "[") {
					break
				}
			}
		}
	}

	return nil, fmt.Errorf("could not find origin remote")
}

// isTestFile checks if a file is a test file
func isTestFile(path string) bool {
	base := filepath.Base(path)
	return strings.Contains(base, "_test.") ||
		strings.Contains(base, ".test.") ||
		strings.Contains(base, ".spec.") ||
		strings.HasPrefix(base, "test_") ||
		strings.HasSuffix(base, "Test.java")
}

// detectTestFramework detects the test framework from file extensions
func detectTestFramework(files []string) string {
	for _, f := range files {
		ext := filepath.Ext(f)
		switch ext {
		case ".go":
			return "Go testing"
		case ".py":
			return "pytest"
		case ".js", ".ts":
			if strings.Contains(f, ".spec.") {
				return "Jest"
			}
			return "supertest"
		case ".java":
			return "JUnit 5"
		}
	}
	return "unknown"
}

// detectTestLanguage detects the language from file extensions
func detectTestLanguage(files []string) string {
	for _, f := range files {
		ext := filepath.Ext(f)
		switch ext {
		case ".go":
			return "Go"
		case ".py":
			return "Python"
		case ".js":
			return "JavaScript"
		case ".ts":
			return "TypeScript"
		case ".java":
			return "Java"
		}
	}
	return "unknown"
}
