package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"arguseek/internal/agent"
)

func main() {
	log.Println("=== ArguSeek Debug Server Starting ===")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Load API keys from environment variables
	googleAPIKey := os.Getenv("GOOGLE_API_KEY")
	log.Printf("GOOGLE_API_KEY present: %v", googleAPIKey != "")

	googleCSEID := os.Getenv("GOOGLE_CSE_ID")
	log.Printf("GOOGLE_CSE_ID present: %v", googleCSEID != "")

	geminiAPIKey := os.Getenv("GEMINI_API_KEY")
	if geminiAPIKey == "" {
		geminiAPIKey = googleAPIKey
	}
	log.Printf("GEMINI_API_KEY present: %v", geminiAPIKey != "")

	// Try to create search agent
	var searchAgent *agent.SearchAgent
	var agentErr error

	if googleAPIKey != "" && googleCSEID != "" {
		log.Println("Creating search agent...")
		searchAgent, agentErr = agent.NewSearchAgent(agent.Config{
			GoogleAPIKey:       googleAPIKey,
			GoogleCSEID:        googleCSEID,
			GeminiAPIKey:       geminiAPIKey,
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
		fmt.Fprintf(w, `{"status":"healthy","service":"arguseek-debug"}`)
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
				"gemini_api_key_present": geminiAPIKey != "",
			},
			"agent": map[string]interface{}{
				"created": searchAgent != nil,
				"error":   fmt.Sprintf("%v", agentErr),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(debug)
	})

	http.HandleFunc("/test-search", func(w http.ResponseWriter, r *http.Request) {
		if searchAgent == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, `{"error":"Search agent not initialized"}`)
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
		json.NewEncoder(w).Encode(response)
	})

	log.Printf("Starting debug server on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
