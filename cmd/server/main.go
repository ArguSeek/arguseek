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
)

// ArguSeek MCP Server
//
// This server provides MCP tools for web research without built-in authentication.
// It is designed to be open-by-default for simplicity and flexibility.
//
// By default, the server runs in stdio mode for MCP clients like Claude Code CLI.
// Use the -http flag to run as an HTTP server (for Cloud Run deployments).
//
// Security: For production deployments, add authentication externally via:
// - Reverse proxy (nginx, Caddy, Traefik)
// - Cloud Run IAM authentication
// - API Gateway with custom authorizer
// - See PRODUCTION_SECURITY.md for detailed options
func main() {
	// Parse command-line flags
	httpMode := flag.Bool("http", false, "run as HTTP server instead of stdio mode")
	flag.Parse()

	ctx := context.Background()

	// Setup structured logging
	logLevel := logging.INFO
	if os.Getenv("DEBUG") == "true" {
		logLevel = logging.DEBUG
	}
	logging.SetLevel(logLevel)

	// In stdio mode, redirect logs to stderr (stdout reserved for MCP messages)
	if !*httpMode {
		log.SetOutput(os.Stderr)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Load API keys from environment variables
	googleAPIKey := os.Getenv("GOOGLE_API_KEY")
	if googleAPIKey == "" {
		logging.Fatal(ctx, "GOOGLE_API_KEY environment variable is required")
	}

	googleCSEID := os.Getenv("GOOGLE_CSE_ID")
	if googleCSEID == "" {
		logging.Fatal(ctx, "GOOGLE_CSE_ID environment variable is required")
	}

	geminiAPIKey := os.Getenv("GEMINI_API_KEY")
	if geminiAPIKey == "" {
		geminiAPIKey = googleAPIKey
		logging.Info(ctx, "GEMINI_API_KEY not set, using GOOGLE_API_KEY")
	}

	// Debug logging
	logging.Info(ctx, "API Keys loaded from environment", map[string]interface{}{
		"google_key_prefix": googleAPIKey[:10] + "...",
		"gemini_key_prefix": geminiAPIKey[:10] + "...",
		"keys_are_same":     googleAPIKey == geminiAPIKey,
	})

	searchAgent, err := agent.NewSearchAgent(agent.Config{
		GoogleAPIKey:       googleAPIKey,
		GoogleCSEID:        googleCSEID,
		GeminiAPIKey:       geminiAPIKey,
		MaxResultsPerQuery: 10,
	})
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
		fmt.Fprintf(w, `{"status":"healthy","service":"arguseek"}`)
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
		fmt.Fprintf(w, `{"issuer":%q}`, oauthIssuer)
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
			fmt.Fprintf(w, `{"service":"arguseek","version":"0.3.3","endpoints":["/health","/.well-known/oauth-authorization-server","/mcp"]}`)
			return
		}

		// All other unregistered paths return JSON 404
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"error":"not found","path":%q}`, r.URL.Path)
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
