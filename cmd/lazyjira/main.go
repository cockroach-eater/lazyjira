package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/pkg/config"
	"github.com/textfuel/lazyjira/pkg/jira"
	"github.com/textfuel/lazyjira/pkg/tui"
)

// version is set at build time via ldflags.
var version = "dev"

func main() {
	// Subcommands.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "auth":
			runAuth(os.Args[2:])
			return
		case "logout":
			if err := config.ClearCredentials(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Credentials cleared.")
			return
		case "--version", "version":
			fmt.Printf("lazyjira %s\n", version)
			return
		}
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	dryRun := flag.Bool("dry-run", false, "Log API requests without making write operations")
	logFile := flag.String("log", "", "Log API requests to file")
	demo := flag.Bool("demo", false, "Run with demo data (no Jira account needed)")
	flag.Parse()

	cfg, _ := config.Load()

	var client jira.ClientInterface
	var authMethod tui.AuthMethod

	if *demo {
		var cleanup func()
		var err error
		client, authMethod, cleanup, err = startDemo(cfg)
		if err != nil {
			return fmt.Errorf("demo: %w", err)
		}
		if cleanup != nil {
			defer cleanup()
		}
	} else {
		var err error
		var jiraClient *jira.Client
		jiraClient, authMethod, err = resolveClient(cfg)
		if err != nil {
			return err
		}

		if *dryRun {
			jiraClient.SetDryRun(true)
			if *logFile == "" {
				*logFile = "lazyjira.log"
			}
		}

		if *logFile != "" {
			f, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				return fmt.Errorf("opening log file: %w", err)
			}
			defer func() { _ = f.Close() }()
			jiraClient.SetLogger(f)
		}

		client = jiraClient
	}

	tui.Version = version
	app := tui.NewAppWithAuth(cfg, client, authMethod)

	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

// resolveClient finds credentials from: saved auth.json > env vars > interactive wizard.
func resolveClient(cfg *config.Config) (*jira.Client, tui.AuthMethod, error) {
	// 1. Saved credentials.
	creds, err := config.LoadCredentials()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
	if creds != nil && creds.Host != "" && creds.Email != "" && creds.Token != "" {
		// Populate config so TUI has host/email info.
		cfg.Jira.Host = creds.Host
		cfg.Jira.Email = creds.Email
		return jira.NewClient(creds.Host, creds.Email, creds.Token), tui.AuthSaved, nil
	}

	// 2. Environment variables.
	if cfg.Jira.Host != "" && cfg.Jira.Email != "" && cfg.Jira.Token != "" {
		return jira.NewClient(cfg.Jira.Host, cfg.Jira.Email, cfg.Jira.Token), tui.AuthEnv, nil
	}

	// 3. Interactive wizard.
	fmt.Println()
	fmt.Println("  Welcome to lazyjira! Let's set up your Jira connection.")
	fmt.Println()
	client, err := runSetupWizard()
	return client, tui.AuthWizard, err
}

// runSetupWizard interactively collects Jira credentials.
func runSetupWizard() (*jira.Client, error) {
	reader := bufio.NewReader(os.Stdin)

	// Host.
	fmt.Println("  \033[1mJira Host\033[0m")
	fmt.Println("  Your Jira Cloud URL, e.g. https://yourcompany.atlassian.net")
	fmt.Println()
	fmt.Print("  Host: ")
	host, _ := reader.ReadString('\n')
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, errors.New("host is required")
	}
	host = strings.TrimRight(host, "/")
	if !strings.HasPrefix(host, "http") {
		host = "https://" + host
	}

	fmt.Println()

	// Email.
	fmt.Println("  \033[1mEmail\033[0m")
	fmt.Println("  Your Atlassian account email address")
	fmt.Println()
	fmt.Print("  Email: ")
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, errors.New("email is required")
	}

	fmt.Println()

	// API Token.
	fmt.Println("  \033[1mAPI Token\033[0m")
	fmt.Println("  Create one at: \033[4mhttps://id.atlassian.com/manage-profile/security/api-tokens\033[0m")
	fmt.Println("  Click 'Create API token', give it a name, and paste it here")
	fmt.Println()
	fmt.Print("  Token: ")
	token, _ := reader.ReadString('\n')
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("API token is required")
	}

	fmt.Println()
	fmt.Println("  Verifying connection...")

	// Test the credentials.
	client := jira.NewClient(host, email, token)
	if err := testConnection(client); err != nil {
		fmt.Printf("\n  \033[31m✗ Connection failed: %v\033[0m\n\n", err)
		fmt.Println("  Please check your credentials and try again.")
		fmt.Println("  Run 'lazyjira auth' to retry.")
		return nil, errors.New("connection test failed")
	}

	fmt.Println("  \033[32m✓ Connected successfully!\033[0m")
	fmt.Println()

	// Save.
	creds := &config.Credentials{
		Host:  host,
		Email: email,
		Token: token,
	}
	if err := config.SaveCredentials(creds); err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: could not save credentials: %v\n", err)
		fmt.Fprintf(os.Stderr, "  Saved to: %s\n", config.AuthPath())
	} else {
		fmt.Printf("  Credentials saved to %s\n", config.AuthPath())
	}
	fmt.Println()

	return client, nil
}

// testConnection verifies credentials by fetching the current user.
func testConnection(client *jira.Client) error {
	// Quick test: GET /myself
	httpClient := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", strings.TrimRight(client.BaseURL(), "/")+"/myself", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", client.AuthHeader())
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach %s: %w", client.BaseURL(), err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return fmt.Errorf("auth failed (HTTP %d) — check email and API token", resp.StatusCode)
	}
	return nil
}

// runAuth handles 'lazyjira auth' — re-runs the setup wizard.
func runAuth(args []string) {
	authFlags := flag.NewFlagSet("auth", flag.ExitOnError)
	_ = authFlags.Parse(args)

	_, err := runSetupWizard()
	if err != nil {
		os.Exit(1)
	}
}
