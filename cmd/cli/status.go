package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/QTest-hq/qtest/internal/cliauth"
	"github.com/QTest-hq/qtest/internal/workspace"
	"github.com/spf13/cobra"
)

// version is set at build time
var statusVersion = "dev"

func statusCmd() *cobra.Command {
	var (
		verbose bool
		json    bool
	)

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show QTest system status",
		Long: `Display comprehensive status information about the QTest CLI and its dependencies.

Shows:
- CLI version and configuration
- Authentication status
- API server connectivity
- Ollama (LLM) status
- Workspace summary

Examples:
  qtest status              # Show basic status
  qtest status -v           # Show verbose status with details`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if json {
				return printJSONStatus(verbose)
			}
			return printStatus(verbose)
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show verbose status")
	cmd.Flags().BoolVar(&json, "json", false, "Output as JSON")

	return cmd
}

func printStatus(verbose bool) error {
	fmt.Println("QTest Status")
	fmt.Println("============")
	fmt.Println()

	// CLI Info
	fmt.Println("CLI")
	fmt.Println("---")
	fmt.Printf("  Version:     %s\n", version)
	fmt.Printf("  Go:          %s\n", runtime.Version())
	fmt.Printf("  Platform:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println()

	// Authentication
	printAuthStatus(verbose)
	fmt.Println()

	// API Server
	printAPIStatus(verbose)
	fmt.Println()

	// Ollama (LLM)
	printOllamaStatus(verbose)
	fmt.Println()

	// Workspaces
	printWorkspaceStatus(verbose)

	return nil
}

func printAuthStatus(verbose bool) {
	fmt.Println("Authentication")
	fmt.Println("--------------")

	// Check environment variable first
	envKey := os.Getenv("QTEST_API_KEY")
	if envKey != "" {
		fmt.Printf("  Status:      CONFIGURED (via QTEST_API_KEY)\n")
		if verbose && len(envKey) > 10 {
			fmt.Printf("  API Key:     %s...%s\n", envKey[:6], envKey[len(envKey)-4:])
		}
		return
	}

	// Check credentials file
	mgr, err := cliauth.NewCredentialsManager()
	if err != nil {
		fmt.Printf("  Status:      ERROR (%v)\n", err)
		return
	}

	creds, err := mgr.Load()
	if err != nil {
		if err == cliauth.ErrNoCredentials {
			fmt.Printf("  Status:      NOT CONFIGURED\n")
			fmt.Printf("  Hint:        Run 'qtest auth login' to authenticate\n")
		} else {
			fmt.Printf("  Status:      ERROR (%v)\n", err)
		}
		return
	}

	if creds.IsExpired() {
		fmt.Printf("  Status:      EXPIRED\n")
		fmt.Printf("  Hint:        Run 'qtest auth login' to re-authenticate\n")
		return
	}

	fmt.Printf("  Status:      OK\n")
	if creds.APIServer != "" {
		fmt.Printf("  Server:      %s\n", creds.APIServer)
	}
	if verbose {
		if creds.Username != "" {
			fmt.Printf("  User:        %s\n", creds.Username)
		}
		if !creds.CreatedAt.IsZero() {
			fmt.Printf("  Since:       %s\n", creds.CreatedAt.Format("2006-01-02 15:04"))
		}
	}
}

func printAPIStatus(verbose bool) {
	fmt.Println("API Server")
	fmt.Println("----------")

	// Determine server URL
	serverURL := cliauth.DefaultAPIServer()

	// Try to load from credentials
	mgr, err := cliauth.NewCredentialsManager()
	if err == nil {
		creds, err := mgr.Load()
		if err == nil && creds.APIServer != "" {
			serverURL = creds.APIServer
		}
	}

	fmt.Printf("  URL:         %s\n", serverURL)

	// Check health endpoint
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(serverURL + "/health")
	if err != nil {
		fmt.Printf("  Status:      UNREACHABLE\n")
		if verbose {
			fmt.Printf("  Error:       %v\n", err)
		}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Printf("  Status:      OK\n")
	} else {
		fmt.Printf("  Status:      UNHEALTHY (%d)\n", resp.StatusCode)
	}
}

func printOllamaStatus(verbose bool) {
	fmt.Println("Ollama (LLM)")
	fmt.Println("------------")

	ollamaURL := os.Getenv("OLLAMA_HOST")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}

	fmt.Printf("  URL:         %s\n", ollamaURL)

	// Check Ollama API
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(ollamaURL + "/api/tags")
	if err != nil {
		fmt.Printf("  Status:      NOT RUNNING\n")
		fmt.Printf("  Hint:        Run 'ollama serve' to start\n")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Printf("  Status:      OK\n")
		if verbose {
			// Could parse and show available models here
			fmt.Printf("  Models:      Available (use 'ollama list' to see)\n")
		}
	} else {
		fmt.Printf("  Status:      ERROR (%d)\n", resp.StatusCode)
	}
}

func printWorkspaceStatus(verbose bool) {
	fmt.Println("Workspaces")
	fmt.Println("----------")

	workspaces, err := workspace.ListWorkspaces(nil)
	if err != nil {
		fmt.Printf("  Status:      ERROR (%v)\n", err)
		return
	}

	if len(workspaces) == 0 {
		fmt.Printf("  Count:       0\n")
		fmt.Printf("  Hint:        Run 'qtest workspace init <url>' to create one\n")
		return
	}

	fmt.Printf("  Count:       %d\n", len(workspaces))

	// Count by phase
	phaseCounts := make(map[string]int)
	for _, ws := range workspaces {
		phase := string(ws.State.Phase)
		if phase == "" {
			phase = "idle"
		}
		phaseCounts[phase]++
	}

	for phase, count := range phaseCounts {
		fmt.Printf("  %s: %d\n", phase, count)
	}

	if verbose && len(workspaces) > 0 {
		fmt.Println()
		fmt.Println("  Recent workspaces:")
		limit := 5
		if len(workspaces) < limit {
			limit = len(workspaces)
		}
		for i := 0; i < limit; i++ {
			ws := workspaces[i]
			progress := ""
			if ws.State.TotalTargets > 0 {
				progress = fmt.Sprintf(" (%d/%d)", ws.State.Completed, ws.State.TotalTargets)
			}
			phase := string(ws.State.Phase)
			if phase == "" {
				phase = "idle"
			}
			fmt.Printf("    - %s: %s%s\n", ws.ID, phase, progress)
		}
	}
}

func printJSONStatus(verbose bool) error {
	// Basic JSON output - could be expanded
	fmt.Println("{")
	fmt.Printf("  \"version\": \"%s\",\n", version)
	fmt.Printf("  \"go_version\": \"%s\",\n", runtime.Version())
	fmt.Printf("  \"platform\": \"%s/%s\",\n", runtime.GOOS, runtime.GOARCH)

	// Auth
	authStatus := "not_configured"
	if os.Getenv("QTEST_API_KEY") != "" {
		authStatus = "configured"
	} else {
		mgr, err := cliauth.NewCredentialsManager()
		if err == nil {
			if creds, err := mgr.Load(); err == nil && !creds.IsExpired() {
				authStatus = "configured"
			}
		}
	}
	fmt.Printf("  \"auth_status\": \"%s\",\n", authStatus)

	// API
	apiStatus := "unreachable"
	serverURL := cliauth.DefaultAPIServer()
	client := &http.Client{Timeout: 5 * time.Second}
	if resp, err := client.Get(serverURL + "/health"); err == nil {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			apiStatus = "ok"
		}
	}
	fmt.Printf("  \"api_status\": \"%s\",\n", apiStatus)
	fmt.Printf("  \"api_url\": \"%s\",\n", serverURL)

	// Ollama
	ollamaStatus := "not_running"
	ollamaURL := os.Getenv("OLLAMA_HOST")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}
	if resp, err := client.Get(ollamaURL + "/api/tags"); err == nil {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			ollamaStatus = "ok"
		}
	}
	fmt.Printf("  \"ollama_status\": \"%s\",\n", ollamaStatus)
	fmt.Printf("  \"ollama_url\": \"%s\",\n", ollamaURL)

	// Workspaces
	workspaceCount := 0
	if workspaces, err := workspace.ListWorkspaces(nil); err == nil {
		workspaceCount = len(workspaces)
	}
	fmt.Printf("  \"workspace_count\": %d\n", workspaceCount)

	fmt.Println("}")
	return nil
}
