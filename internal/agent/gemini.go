package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Model enum for type safety
type GeminiModel string

const (
	GeminiFlash25     GeminiModel = "gemini-2.5-flash"      // Default for text
	GeminiFlash25Lite GeminiModel = "gemini-2.5-flash-lite" // For PDFs
)

type GeminiClient struct {
	apiKey     string
	httpClient *http.Client
}

type geminiRequest struct {
	Contents         []geminiContent   `json:"contents"`
	GenerationConfig *generationConfig `json:"generationConfig,omitempty"`
}

type generationConfig struct {
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text *string `json:"text,omitempty"`
}

type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
}

type geminiCandidate struct {
	Content geminiContent `json:"content"`
}

func NewGeminiClient(apiKey string) *GeminiClient {
	// Create a custom transport that removes the project header
	transport := &headerStripTransport{
		Base: http.DefaultTransport,
	}

	return &GeminiClient{
		apiKey: strings.TrimSpace(apiKey),
		httpClient: &http.Client{
			Timeout:   90 * time.Second,
			Transport: transport,
		},
	}
}

// headerStripTransport removes the X-Goog-User-Project header
type headerStripTransport struct {
	Base http.RoundTripper
}

func (t *headerStripTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid modifying the original
	newReq := req.Clone(req.Context())
	// Remove the project header that Cloud Run adds
	newReq.Header.Del("X-Goog-User-Project")
	return t.Base.RoundTrip(newReq)
}

func (c *GeminiClient) Complete(ctx context.Context, prompt string, model GeminiModel) (string, error) {
	return c.CompleteWithTemp(ctx, prompt, model, nil)
}

func (c *GeminiClient) CompleteWithTemp(ctx context.Context, prompt string, model GeminiModel, temperature *float64) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("Gemini API key is empty")
	}
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, c.apiKey)

	text := prompt
	req := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: &text},
				},
			},
		},
	}

	// Add temperature if specified
	if temperature != nil {
		req.GenerationConfig = &generationConfig{
			Temperature: temperature,
		}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// Log the actual error for debugging
		if resp.StatusCode == 429 {
			return "", fmt.Errorf("Gemini API returned status %d: %s (key prefix: %s...)", resp.StatusCode, string(body), c.apiKey[:10])
		}
		return "", fmt.Errorf("Gemini API returned status %d: %s", resp.StatusCode, string(body))
	}

	var geminiResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from Gemini")
	}

	return *geminiResp.Candidates[0].Content.Parts[0].Text, nil
}
