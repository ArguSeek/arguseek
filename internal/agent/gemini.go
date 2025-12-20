package agent

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/genai"
)

// Model enum for type safety
type GeminiModel string

const (
	GeminiFlash25     GeminiModel = "gemini-2.5-flash"      // Default for text
	GeminiFlash25Lite GeminiModel = "gemini-2.5-flash-lite" // For PDFs
)

// GeminiConfig holds configuration for Gemini client
type GeminiConfig struct {
	UseVertexAI bool   // true = Vertex AI, false = Gemini API
	APIKey      string // For Gemini API backend
	Project     string // For Vertex AI backend
	Location    string // For Vertex AI backend (default: us-central1)
}

// GeminiConfigFromEnv creates a GeminiConfig from environment variables.
// If GOOGLE_CLOUD_PROJECT is set, uses Vertex AI backend (better quotas, IAM auth).
// Otherwise, uses Gemini API with GEMINI_API_KEY or the provided fallbackAPIKey.
func GeminiConfigFromEnv(fallbackAPIKey string) GeminiConfig {
	if project := os.Getenv("GOOGLE_CLOUD_PROJECT"); project != "" {
		location := os.Getenv("GOOGLE_CLOUD_LOCATION")
		if location == "" {
			location = "us-central1"
		}
		return GeminiConfig{
			UseVertexAI: true,
			Project:     project,
			Location:    location,
		}
	}

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = fallbackAPIKey
	}
	return GeminiConfig{
		UseVertexAI: false,
		APIKey:      apiKey,
	}
}

// GeminiClient wraps the Google Gen AI SDK client
type GeminiClient struct {
	client *genai.Client
}

// NewGeminiClient creates a client using the unified Google Gen AI SDK
func NewGeminiClient(ctx context.Context, cfg GeminiConfig) (*GeminiClient, error) {
	var clientCfg *genai.ClientConfig

	if cfg.UseVertexAI {
		// Vertex AI backend - uses Application Default Credentials
		if cfg.Project == "" {
			return nil, fmt.Errorf("project is required for Vertex AI backend")
		}
		location := cfg.Location
		if location == "" {
			location = "us-central1"
		}
		clientCfg = &genai.ClientConfig{
			Project:  cfg.Project,
			Location: location,
			Backend:  genai.BackendVertexAI,
		}
	} else {
		// Gemini API backend - uses API key
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("API key is required for Gemini API backend")
		}
		clientCfg = &genai.ClientConfig{
			APIKey:  cfg.APIKey,
			Backend: genai.BackendGeminiAPI,
		}
	}

	client, err := genai.NewClient(ctx, clientCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	return &GeminiClient{client: client}, nil
}

// Complete generates text from a prompt using the specified model
func (c *GeminiClient) Complete(ctx context.Context, prompt string, model GeminiModel) (string, error) {
	return c.CompleteWithTemp(ctx, prompt, model, nil)
}

// CompleteWithTemp generates text with optional temperature control
func (c *GeminiClient) CompleteWithTemp(ctx context.Context, prompt string, model GeminiModel, temperature *float64) (string, error) {
	var config *genai.GenerateContentConfig
	if temperature != nil {
		temp32 := float32(*temperature)
		config = &genai.GenerateContentConfig{
			Temperature: &temp32,
		}
	}

	result, err := c.client.Models.GenerateContent(ctx,
		string(model),
		genai.Text(prompt),
		config,
	)
	if err != nil {
		return "", fmt.Errorf("generate content failed: %w", err)
	}

	return result.Text(), nil
}
