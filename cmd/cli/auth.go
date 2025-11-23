package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/QTest-hq/qtest/internal/cliauth"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func authCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
		Long: `Manage authentication for the QTest CLI.

The QTest CLI uses API keys for authentication. You can obtain an API key
from the QTest web interface or use the login command to authenticate.

Examples:
  qtest auth login                    # Login with API key
  qtest auth login --token KEY        # Login with provided key
  qtest auth status                   # Show current auth status
  qtest auth logout                   # Remove stored credentials`,
	}

	cmd.AddCommand(authLoginCmd())
	cmd.AddCommand(authLogoutCmd())
	cmd.AddCommand(authStatusCmd())

	return cmd
}

func authLoginCmd() *cobra.Command {
	var (
		token     string
		apiServer string
		validate  bool
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with QTest API",
		Long: `Authenticate with the QTest API using an API key.

You can provide the API key via:
  1. --token flag
  2. QTEST_API_KEY environment variable
  3. Interactive prompt (secure input)

The credentials are stored in ~/.qtest/credentials.json with restricted permissions.

Examples:
  qtest auth login                              # Interactive login
  qtest auth login --token qtest_xxxx           # Login with token
  qtest auth login --server https://api.qtest.io  # Custom server`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get API key from flag, env, or prompt
			apiKey := token
			if apiKey == "" {
				apiKey = os.Getenv("QTEST_API_KEY")
			}
			if apiKey == "" {
				var err error
				apiKey, err = promptForAPIKey()
				if err != nil {
					return err
				}
			}

			// Validate key format
			if err := cliauth.ValidateAPIKey(apiKey); err != nil {
				return fmt.Errorf("invalid API key: %w", err)
			}

			// Determine server URL
			server := apiServer
			if server == "" {
				server = cliauth.DefaultAPIServer()
			}

			// Create credentials
			creds := &cliauth.Credentials{
				APIKey:    apiKey,
				APIServer: server,
				CreatedAt: time.Now(),
			}

			// Validate against server if requested
			if validate {
				fmt.Println("Validating credentials...")
				client := cliauth.NewAPIClient(server, apiKey)

				// Check server health first
				if err := client.CheckHealth(); err != nil {
					return fmt.Errorf("cannot connect to server: %w", err)
				}

				// Validate credentials
				userInfo, err := client.ValidateCredentials()
				if err != nil {
					return fmt.Errorf("authentication failed: %w", err)
				}

				// Update credentials with user info
				creds.UserID = userInfo.ID
				creds.Username = userInfo.Username
				creds.Email = userInfo.Email
				creds.OrganizationID = userInfo.OrganizationID

				fmt.Printf("Authenticated as: %s (%s)\n", userInfo.Username, userInfo.Email)
			}

			// Save credentials
			mgr, err := cliauth.NewCredentialsManager()
			if err != nil {
				return fmt.Errorf("failed to initialize credentials manager: %w", err)
			}

			if err := mgr.Save(creds); err != nil {
				return fmt.Errorf("failed to save credentials: %w", err)
			}

			fmt.Println("Login successful!")
			fmt.Printf("Credentials saved to ~/.qtest/credentials.json\n")
			fmt.Printf("Server: %s\n", server)

			return nil
		},
	}

	cmd.Flags().StringVarP(&token, "token", "t", "", "API key token")
	cmd.Flags().StringVarP(&apiServer, "server", "s", "", "API server URL (default: http://localhost:8080)")
	cmd.Flags().BoolVar(&validate, "validate", true, "Validate credentials against server")

	return cmd
}

func authLogoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Remove stored credentials",
		Long: `Remove stored credentials from the local machine.

This will delete the credentials file at ~/.qtest/credentials.json.
You will need to login again to use authenticated commands.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := cliauth.NewCredentialsManager()
			if err != nil {
				return fmt.Errorf("failed to initialize credentials manager: %w", err)
			}

			if !mgr.Exists() {
				fmt.Println("No credentials found. Already logged out.")
				return nil
			}

			if err := mgr.Delete(); err != nil {
				return fmt.Errorf("failed to remove credentials: %w", err)
			}

			fmt.Println("Logged out successfully.")
			fmt.Println("Credentials removed from ~/.qtest/credentials.json")
			return nil
		},
	}

	return cmd
}

func authStatusCmd() *cobra.Command {
	var checkServer bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show current authentication status",
		Long: `Display information about the current authentication status.

Shows whether you are logged in, which server you're connected to,
and optionally validates the credentials against the server.

Examples:
  qtest auth status              # Show status
  qtest auth status --check      # Validate with server`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check for environment variable first
			envKey := os.Getenv("QTEST_API_KEY")
			if envKey != "" {
				fmt.Println("Authentication Status")
				fmt.Println("=====================")
				fmt.Println()
				fmt.Println("Source:     Environment variable (QTEST_API_KEY)")
				fmt.Printf("API Key:    %s...%s\n", envKey[:6], envKey[len(envKey)-4:])
				fmt.Printf("Server:     %s\n", cliauth.DefaultAPIServer())

				if checkServer {
					return validateWithServer(cliauth.DefaultAPIServer(), envKey)
				}
				return nil
			}

			// Try loading from credentials file
			mgr, err := cliauth.NewCredentialsManager()
			if err != nil {
				return fmt.Errorf("failed to initialize credentials manager: %w", err)
			}

			creds, err := mgr.Load()
			if err != nil {
				if err == cliauth.ErrNoCredentials {
					fmt.Println("Not logged in.")
					fmt.Println()
					fmt.Println("To authenticate, run:")
					fmt.Println("  qtest auth login")
					fmt.Println()
					fmt.Println("Or set the QTEST_API_KEY environment variable.")
					return nil
				}
				return fmt.Errorf("failed to load credentials: %w", err)
			}

			// Display status
			fmt.Println("Authentication Status")
			fmt.Println("=====================")
			fmt.Println()
			fmt.Println("Source:     Credentials file (~/.qtest/credentials.json)")

			if creds.APIKey != "" {
				masked := creds.APIKey[:6] + "..." + creds.APIKey[len(creds.APIKey)-4:]
				fmt.Printf("API Key:    %s\n", masked)
			}
			if creds.APIServer != "" {
				fmt.Printf("Server:     %s\n", creds.APIServer)
			}
			if creds.Username != "" {
				fmt.Printf("Username:   %s\n", creds.Username)
			}
			if creds.Email != "" {
				fmt.Printf("Email:      %s\n", creds.Email)
			}
			if creds.OrganizationID != "" {
				fmt.Printf("Org ID:     %s\n", creds.OrganizationID)
			}
			if !creds.CreatedAt.IsZero() {
				fmt.Printf("Logged in:  %s\n", creds.CreatedAt.Format(time.RFC3339))
			}

			if creds.IsExpired() {
				fmt.Println()
				fmt.Println("WARNING: Credentials have expired. Please login again.")
				return nil
			}

			if checkServer {
				server := creds.APIServer
				if server == "" {
					server = cliauth.DefaultAPIServer()
				}
				return validateWithServer(server, creds.APIKey)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&checkServer, "check", "c", false, "Validate credentials with server")

	return cmd
}

func promptForAPIKey() (string, error) {
	fmt.Println("Enter your QTest API key.")
	fmt.Println("You can get one from the QTest web interface (Settings > API Keys).")
	fmt.Println()
	fmt.Print("API Key: ")

	// Try to read securely (hidden input)
	if term.IsTerminal(int(syscall.Stdin)) {
		password, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println() // Add newline after hidden input
		if err != nil {
			return "", fmt.Errorf("failed to read input: %w", err)
		}
		return strings.TrimSpace(string(password)), nil
	}

	// Fall back to regular input (for non-interactive use)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	return strings.TrimSpace(input), nil
}

func validateWithServer(server, apiKey string) error {
	fmt.Println()
	fmt.Println("Validating with server...")

	client := cliauth.NewAPIClient(server, apiKey)

	// Check health
	if err := client.CheckHealth(); err != nil {
		fmt.Printf("Server status: UNREACHABLE (%v)\n", err)
		return nil
	}
	fmt.Println("Server status: OK")

	// Validate credentials
	userInfo, err := client.ValidateCredentials()
	if err != nil {
		fmt.Printf("Credentials:   INVALID (%v)\n", err)
		return nil
	}

	fmt.Println("Credentials:   VALID")
	fmt.Printf("User:          %s (%s)\n", userInfo.Username, userInfo.Email)
	if len(userInfo.Scopes) > 0 {
		fmt.Printf("Scopes:        %s\n", strings.Join(userInfo.Scopes, ", "))
	}

	return nil
}
