package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"arguseek/internal/agent"
	"arguseek/internal/logging"
	"arguseek/internal/mcp"
	"arguseek/internal/version"
)

// ArguSeek MCP Server
//
// This server provides MCP tools for web research without built-in authentication.
// It is designed to be open-by-default for simplicity and flexibility.
//
// By default, the server runs in stdio mode for MCP clients like Claude Code CLI.
// Use the -http flag to run as an HTTP server (for Cloud Run deployments).
//
// CLI commands:
// - arguseek research "<query>" [--previous "<previous query>"]
// - arguseek fetch <url> [--looking-for "<extraction hint>"]
//
// Security: For production deployments, add authentication externally via:
// - Reverse proxy (nginx, Caddy, Traefik)
// - Cloud Run IAM authentication
// - API Gateway with custom authorizer
// - See PRODUCTION_SECURITY.md for detailed options
func main() {
	// Check for subcommands before flag parsing
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "research":
			runResearchCLI(os.Args[2:])
			return
		case "fetch":
			runFetchCLI(os.Args[2:])
			return
		}
	}

	// Parse command-line flags
	httpMode := flag.Bool("http", false, "run as HTTP server instead of stdio mode")
	versionFlag := flag.Bool("version", false, "print version information and exit")
	flag.Parse()

	// Handle version flag
	if *versionFlag {
		fmt.Println(version.Version)
		os.Exit(0)
	}

	ctx := context.Background()

	// Setup structured logging
	logLevel := logging.INFO
	if os.Getenv("DEBUG") == "true" {
		logLevel = logging.DEBUG
	}
	logging.SetLevel(logLevel)

	// Configure log output: always use stderr
	// - Stdio mode: stdout reserved for MCP messages
	// - HTTP mode: explicit stderr for structured logging
	log.SetOutput(os.Stderr)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Create search agent from environment
	searchAgent, err := initSearchAgent(ctx)
	if err != nil {
		logging.Fatal(ctx, "Failed to create search agent", map[string]interface{}{
			"error": err.Error(),
		})
	}

	handler := mcp.NewHandler(searchAgent)

	// Mode selection: stdio (default) or HTTP
	if !*httpMode {
		// Stdio mode: read from stdin, write to stdout
		logging.Info(ctx, "Starting server in stdio mode")
		if err := handler.ServeStdio(ctx); err != nil {
			logging.Fatal(ctx, "Stdio server error", map[string]interface{}{
				"error": err.Error(),
			})
		}
		return
	}

	// HTTP mode: start HTTP server
	logging.Info(ctx, "Starting server in HTTP mode", map[string]interface{}{
		"port": port,
	})

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"status":"healthy","service":"arguseek"}`)
	})

	// OAuth discovery endpoint - signals no authentication required
	// This allows Claude Code CLI and other OAuth-aware clients to connect directly via HTTP transport
	// The minimal response indicates this server does not implement OAuth authentication
	oauthIssuer := os.Getenv("OAUTH_ISSUER")
	if oauthIssuer == "" {
		// Default to localhost for local development
		oauthIssuer = "http://localhost:8080"
	}

	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		// Per RFC 8414: OAuth metadata MUST be queried using HTTP GET
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Header().Set("Allow", "GET")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Minimal OAuth metadata - issuer field signals no OAuth flow required
		// Use configured issuer URL (from OAUTH_ISSUER env var) for stable identity
		_, _ = fmt.Fprintf(w, `{"issuer":%q}`, oauthIssuer)
	})

	// Setup MCP endpoint without authentication
	// For production: Add auth via reverse proxy or Cloud Run IAM (see PRODUCTION_SECURITY.md)
	mux.HandleFunc("/mcp", handler.HandleRequest)

	// Catch-all handler - returns JSON responses for all unregistered paths
	// Prevents JSON parse errors in MCP clients expecting JSON 404 responses
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Root path returns service info for discovery
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"service":"arguseek","version":"%s","endpoints":["/health","/.well-known/oauth-authorization-server","/mcp"]}`, version.Version)
			return
		}

		// All other unregistered paths return JSON 404
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, `{"error":"not found","path":%q}`, r.URL.Path)
	})

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  120 * time.Second, // Increased for PDF processing
		WriteTimeout: 120 * time.Second, // Increased for PDF processing
		IdleTimeout:  120 * time.Second,
	}

	done := make(chan bool, 1)
	quit := make(chan os.Signal, 1)

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		logging.Info(ctx, "Server is shutting down...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logging.Fatal(ctx, "Could not gracefully shutdown the server", map[string]interface{}{
				"error": err.Error(),
			})
		}
		close(done)
	}()

	logging.Info(ctx, "Server is ready to handle requests", map[string]interface{}{
		"port": port,
	})

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logging.Fatal(ctx, "Could not listen on server", map[string]interface{}{
			"port":  port,
			"error": err.Error(),
		})
	}

	<-done
	logging.Info(ctx, "Server stopped")
}

// initSearchAgent creates a configured SearchAgent from environment variables.
func initSearchAgent(ctx context.Context) (*agent.SearchAgent, error) {
	googleAPIKey := os.Getenv("GOOGLE_API_KEY")
	if googleAPIKey == "" {
		return nil, fmt.Errorf("GOOGLE_API_KEY environment variable is required")
	}

	googleCSEID := os.Getenv("GOOGLE_CSE_ID")
	if googleCSEID == "" {
		return nil, fmt.Errorf("GOOGLE_CSE_ID environment variable is required")
	}

	geminiCfg := agent.GeminiConfigFromEnv(googleAPIKey)
	if geminiCfg.UseVertexAI {
		logging.Info(ctx, "Using Vertex AI backend", map[string]interface{}{
			"project":  geminiCfg.Project,
			"location": geminiCfg.Location,
		})
	} else {
		logging.Info(ctx, "Using Gemini API backend", nil)
	}

	return agent.NewSearchAgent(ctx, agent.Config{
		GoogleAPIKey:       googleAPIKey,
		GoogleCSEID:        googleCSEID,
		GeminiConfig:       geminiCfg,
		MaxResultsPerQuery: 10,
	})
}

// runResearchCLI handles the "arguseek research" subcommand.
func runResearchCLI(args []string) {
	fs := flag.NewFlagSet("research", flag.ExitOnError)
	previous := fs.String("previous", "", "previous query for context chaining")
	depthFlag := fs.String("depth", "normal", "research depth: fast, normal, deep")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: arguseek research <query> [--previous <previous query>] [--depth fast|normal|deep]\n\n")
		fmt.Fprintf(os.Stderr, "Perform iterative web research using Google Search and Gemini.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: query argument is required")
		fs.Usage()
		os.Exit(1)
	}

	query := fs.Arg(0)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Suppress logs for CLI mode - only output results
	log.SetOutput(os.Stderr)

	searchAgent, err := initSearchAgent(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var previousQuery *string
	if *previous != "" {
		previousQuery = previous
	}

	depth := agent.ParseDepth(*depthFlag)
	result, err := searchAgent.ResearchIteratively(ctx, query, previousQuery, depth)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(result)
}

// runFetchCLI handles the "arguseek fetch" subcommand.
func runFetchCLI(args []string) {
	fs := flag.NewFlagSet("fetch", flag.ExitOnError)
	lookingFor := fs.String("looking-for", "", "what information to extract from the page")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: arguseek fetch <url> [--looking-for <extraction hint>]\n\n")
		fmt.Fprintf(os.Stderr, "Fetch and extract content from a webpage URL.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: url argument is required")
		fs.Usage()
		os.Exit(1)
	}

	url := fs.Arg(0)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Suppress logs for CLI mode - only output results
	log.SetOutput(os.Stderr)

	searchAgent, err := initSearchAgent(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	result, err := searchAgent.FetchURL(ctx, url, *lookingFor)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(result)
}
