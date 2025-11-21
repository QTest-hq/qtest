package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var (
	apiURL     string
	jsonOutput bool
)

// jobCmd returns the job parent command
func jobCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "job",
		Aliases: []string{"jobs"},
		Short:   "Manage async jobs",
		Long:    "Submit, list, and manage async test generation jobs via the API server.",
	}

	cmd.PersistentFlags().StringVar(&apiURL, "api-url", "http://localhost:8080", "API server URL")
	cmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	cmd.AddCommand(jobSubmitCmd())
	cmd.AddCommand(jobListCmd())
	cmd.AddCommand(jobStatusCmd())
	cmd.AddCommand(jobCancelCmd())
	cmd.AddCommand(jobRetryCmd())

	return cmd
}

// jobSubmitCmd creates a new job or starts a pipeline
func jobSubmitCmd() *cobra.Command {
	var (
		repoURL    string
		branch     string
		maxTests   int
		llmTier    int
		createPR   bool
		jobType    string
	)

	cmd := &cobra.Command{
		Use:   "submit",
		Short: "Submit a new job or start a pipeline",
		Long: `Submit a new test generation job.

Examples:
  # Start full pipeline for a repository
  qtest job submit --repo https://github.com/user/repo

  # With options
  qtest job submit --repo https://github.com/user/repo --max-tests 50 --tier 2

  # Submit specific job type
  qtest job submit --type generation --repo https://github.com/user/repo`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if repoURL == "" {
				return fmt.Errorf("--repo is required")
			}

			var endpoint string
			var payload interface{}

			if jobType != "" {
				// Submit specific job type
				endpoint = "/api/v1/jobs"
				payload = map[string]interface{}{
					"type": jobType,
					"payload": map[string]interface{}{
						"repository_url": repoURL,
						"branch":         branch,
					},
				}
			} else {
				// Start full pipeline
				endpoint = "/api/v1/jobs/pipeline"
				payload = map[string]interface{}{
					"repository_url": repoURL,
					"branch":         branch,
					"max_tests":      maxTests,
					"llm_tier":       llmTier,
					"create_pr":      createPR,
				}
			}

			resp, err := postJSON(apiURL+endpoint, payload)
			if err != nil {
				return err
			}

			if jsonOutput {
				fmt.Println(string(resp))
				return nil
			}

			var job jobResponse
			if err := json.Unmarshal(resp, &job); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			fmt.Printf("Job submitted successfully!\n")
			fmt.Printf("  ID:     %s\n", job.ID)
			fmt.Printf("  Type:   %s\n", job.Type)
			fmt.Printf("  Status: %s\n", job.Status)
			fmt.Printf("\nCheck status with: qtest job status %s\n", job.ID)

			return nil
		},
	}

	cmd.Flags().StringVar(&repoURL, "repo", "", "Repository URL (required)")
	cmd.Flags().StringVar(&branch, "branch", "", "Git branch")
	cmd.Flags().IntVar(&maxTests, "max-tests", 0, "Maximum tests to generate")
	cmd.Flags().IntVar(&llmTier, "tier", 1, "LLM tier (1=fast, 2=balanced, 3=thorough)")
	cmd.Flags().BoolVar(&createPR, "create-pr", false, "Create PR when done")
	cmd.Flags().StringVar(&jobType, "type", "", "Specific job type (ingestion, modeling, etc.)")

	return cmd
}

// jobListCmd lists jobs
func jobListCmd() *cobra.Command {
	var (
		status   string
		jobType  string
		limit    int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List jobs",
		Long: `List jobs with optional filters.

Examples:
  qtest job list
  qtest job list --status running
  qtest job list --type generation --limit 10`,
		RunE: func(cmd *cobra.Command, args []string) error {
			endpoint := "/api/v1/jobs"
			params := []string{}

			if status != "" {
				params = append(params, "status="+status)
			}
			if jobType != "" {
				params = append(params, "type="+jobType)
			}
			if limit > 0 {
				params = append(params, fmt.Sprintf("limit=%d", limit))
			}

			if len(params) > 0 {
				endpoint += "?" + strings.Join(params, "&")
			}

			resp, err := getJSON(apiURL + endpoint)
			if err != nil {
				return err
			}

			if jsonOutput {
				fmt.Println(string(resp))
				return nil
			}

			var jobs []jobResponse
			if err := json.Unmarshal(resp, &jobs); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			if len(jobs) == 0 {
				fmt.Println("No jobs found.")
				return nil
			}

			printJobTable(jobs)
			return nil
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Filter by status (pending, running, completed, failed)")
	cmd.Flags().StringVar(&jobType, "type", "", "Filter by type")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum results")

	return cmd
}

// jobStatusCmd gets job status
func jobStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <job-id>",
		Short: "Get job status",
		Long: `Get detailed status of a job including child jobs.

Examples:
  qtest job status 550e8400-e29b-41d4-a716-446655440000`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := args[0]
			endpoint := fmt.Sprintf("/api/v1/jobs/%s", jobID)

			resp, err := getJSON(apiURL + endpoint)
			if err != nil {
				return err
			}

			if jsonOutput {
				fmt.Println(string(resp))
				return nil
			}

			var status jobStatusResponse
			if err := json.Unmarshal(resp, &status); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			printJobDetail(status.Job)

			if len(status.Children) > 0 {
				fmt.Println("\nChild Jobs:")
				printJobTable(status.Children)
			}

			return nil
		},
	}

	return cmd
}

// jobCancelCmd cancels a job
func jobCancelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel <job-id>",
		Short: "Cancel a pending job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := args[0]
			endpoint := fmt.Sprintf("/api/v1/jobs/%s/cancel", jobID)

			resp, err := postJSON(apiURL+endpoint, nil)
			if err != nil {
				return err
			}

			if jsonOutput {
				fmt.Println(string(resp))
				return nil
			}

			fmt.Printf("Job %s cancelled.\n", jobID)
			return nil
		},
	}

	return cmd
}

// jobRetryCmd retries a failed job
func jobRetryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "retry <job-id>",
		Short: "Retry a failed job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := args[0]
			endpoint := fmt.Sprintf("/api/v1/jobs/%s/retry", jobID)

			resp, err := postJSON(apiURL+endpoint, nil)
			if err != nil {
				return err
			}

			if jsonOutput {
				fmt.Println(string(resp))
				return nil
			}

			var job jobResponse
			if err := json.Unmarshal(resp, &job); err != nil {
				fmt.Printf("Job %s queued for retry.\n", jobID)
				return nil
			}

			fmt.Printf("Job %s queued for retry.\n", jobID)
			fmt.Printf("  Status: %s\n", job.Status)
			return nil
		},
	}

	return cmd
}

// Response types
type jobResponse struct {
	ID              string  `json:"id"`
	Type            string  `json:"type"`
	Status          string  `json:"status"`
	Priority        int     `json:"priority"`
	RepositoryID    *string `json:"repository_id,omitempty"`
	ErrorMessage    *string `json:"error_message,omitempty"`
	RetryCount      int     `json:"retry_count"`
	MaxRetries      int     `json:"max_retries"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
	StartedAt       *string `json:"started_at,omitempty"`
	CompletedAt     *string `json:"completed_at,omitempty"`
	WorkerID        *string `json:"worker_id,omitempty"`
}

type jobStatusResponse struct {
	Job      *jobResponse  `json:"job"`
	Children []jobResponse `json:"children,omitempty"`
}

// HTTP helpers
func getJSON(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp map[string]string
		if json.Unmarshal(body, &errResp) == nil {
			if msg, ok := errResp["error"]; ok {
				return nil, fmt.Errorf("API error: %s", msg)
			}
		}
		return nil, fmt.Errorf("API error: %s", resp.Status)
	}

	return body, nil
}

func postJSON(url string, data interface{}) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	var body io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to encode request: %w", err)
		}
		body = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp map[string]string
		if json.Unmarshal(respBody, &errResp) == nil {
			if msg, ok := errResp["error"]; ok {
				return nil, fmt.Errorf("API error: %s", msg)
			}
		}
		return nil, fmt.Errorf("API error: %s", resp.Status)
	}

	return respBody, nil
}

// Output helpers
func printJobTable(jobs []jobResponse) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTYPE\tSTATUS\tCREATED\tWORKER")

	for _, j := range jobs {
		created := formatTime(j.CreatedAt)
		worker := "-"
		if j.WorkerID != nil {
			worker = *j.WorkerID
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			truncateJobID(j.ID, 8), j.Type, j.Status, created, truncateJobID(worker, 12))
	}
	w.Flush()
}

func printJobDetail(j *jobResponse) {
	fmt.Printf("Job: %s\n", j.ID)
	fmt.Printf("  Type:       %s\n", j.Type)
	fmt.Printf("  Status:     %s\n", j.Status)
	fmt.Printf("  Priority:   %d\n", j.Priority)
	fmt.Printf("  Retries:    %d/%d\n", j.RetryCount, j.MaxRetries)
	fmt.Printf("  Created:    %s\n", j.CreatedAt)

	if j.StartedAt != nil {
		fmt.Printf("  Started:    %s\n", *j.StartedAt)
	}
	if j.CompletedAt != nil {
		fmt.Printf("  Completed:  %s\n", *j.CompletedAt)
	}
	if j.WorkerID != nil {
		fmt.Printf("  Worker:     %s\n", *j.WorkerID)
	}
	if j.ErrorMessage != nil {
		fmt.Printf("  Error:      %s\n", *j.ErrorMessage)
	}
}

func formatTime(t string) string {
	parsed, err := time.Parse("2006-01-02T15:04:05Z", t)
	if err != nil {
		return t
	}
	return parsed.Format("Jan 02 15:04")
}

func truncateJobID(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
