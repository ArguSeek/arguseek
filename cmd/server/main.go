package main

import (
	"context"
	"fmt"
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
// Security: For production deployments, add authentication externally via:
// - Reverse proxy (nginx, Caddy, Traefik)
// - Cloud Run IAM authentication
// - API Gateway with custom authorizer
// - See PRODUCTION_SECURITY.md for detailed options
func main() {
	ctx := context.Background()

	// Setup structured logging
	logLevel := logging.INFO
	if os.Getenv("DEBUG") == "true" {
		logLevel = logging.DEBUG
	}
	logging.SetLevel(logLevel)

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

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"healthy","service":"arguseek"}`)
	})

	// Setup MCP endpoint without authentication
	// For production: Add auth via reverse proxy or Cloud Run IAM (see PRODUCTION_SECURITY.md)
	mux.HandleFunc("/mcp", handler.HandleRequest)

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
