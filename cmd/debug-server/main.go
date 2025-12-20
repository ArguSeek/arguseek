package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"arguseek/internal/agent"
)

func main() {
	log.Println("=== ArguSeek Debug Server Starting ===")
	ctx := context.Background()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Load API keys from environment variables
	googleAPIKey := os.Getenv("GOOGLE_API_KEY")
	log.Printf("GOOGLE_API_KEY present: %v", googleAPIKey != "")

	googleCSEID := os.Getenv("GOOGLE_CSE_ID")
	log.Printf("GOOGLE_CSE_ID present: %v", googleCSEID != "")

	// Configure Gemini backend
	geminiCfg := agent.GeminiConfigFromEnv(googleAPIKey)
	if geminiCfg.UseVertexAI {
		log.Printf("Using Vertex AI backend (project: %s, location: %s)", geminiCfg.Project, geminiCfg.Location)
	} else {
		log.Printf("Using Gemini API backend (key present: %v)", geminiCfg.APIKey != "")
	}

	// Try to create search agent
	var searchAgent *agent.SearchAgent
	var agentErr error

	if googleAPIKey != "" && googleCSEID != "" {
		log.Println("Creating search agent...")
		searchAgent, agentErr = agent.NewSearchAgent(ctx, agent.Config{
			GoogleAPIKey:       googleAPIKey,
			GoogleCSEID:        googleCSEID,
			GeminiConfig:       geminiCfg,
			MaxResultsPerQuery: 10,
		})
		if agentErr != nil {
			log.Printf("ERROR: Failed to create search agent: %v", agentErr)
		} else {
			log.Println("Search agent created successfully")
		}
	} else {
		log.Println("WARNING: Missing API credentials, search agent not created")
	}

	// Set up HTTP endpoints
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"status":"healthy","service":"arguseek-debug"}`)
	})

	http.HandleFunc("/debug", func(w http.ResponseWriter, r *http.Request) {
		debug := map[string]interface{}{
			"env": map[string]interface{}{
				"PORT": port,
			},
			"credentials": map[string]interface{}{
				"google_api_key_present": googleAPIKey != "",
				"google_api_key_length":  len(googleAPIKey),
				"google_cse_id_present":  googleCSEID != "",
				"google_cse_id_length":   len(googleCSEID),
			},
			"gemini": map[string]interface{}{
				"use_vertex_ai": geminiCfg.UseVertexAI,
				"project":       geminiCfg.Project,
				"location":      geminiCfg.Location,
				"api_key_set":   geminiCfg.APIKey != "",
			},
			"agent": map[string]interface{}{
				"created": searchAgent != nil,
				"error":   fmt.Sprintf("%v", agentErr),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(debug); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	})

	http.HandleFunc("/test-search", func(w http.ResponseWriter, r *http.Request) {
		if searchAgent == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintf(w, `{"error":"Search agent not initialized"}`)
			return
		}

		query := r.URL.Query().Get("q")
		if query == "" {
			query = "test"
		}

		log.Printf("Testing search with query: %s", query)
		result, err := searchAgent.ResearchIteratively(r.Context(), query, nil)

		response := map[string]interface{}{
			"query":  query,
			"result": result,
			"error":  fmt.Sprintf("%v", err),
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	})

	log.Printf("Starting debug server on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
