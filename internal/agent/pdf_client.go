package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	
	"arguseek/internal/content"
)

// PDF-specific gemini types
type geminiInlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type pdfGeminiPart struct {
	Text       *string            `json:"text,omitempty"`
	InlineData *geminiInlineData  `json:"inline_data,omitempty"`
}

type pdfGeminiContent struct {
	Parts []pdfGeminiPart `json:"parts"`
}

type pdfGeminiRequest struct {
	Contents         []pdfGeminiContent `json:"contents"`
	GenerationConfig *generationConfig  `json:"generationConfig,omitempty"`
}

// PDFClient handles PDF processing using Gemini API
type PDFClient struct {
	apiKey     string
	httpClient *http.Client
}

// NewPDFClient creates a new PDF client with the specified API key
func NewPDFClient(apiKey string) *PDFClient {
	return &PDFClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // Increased timeout for PDF processing
			Transport: &headerStripTransport{
				Base: http.DefaultTransport,
			},
		},
	}
}

// ProcessPDF processes a PDF document with the given prompt
func (c *PDFClient) ProcessPDF(ctx context.Context, pdfData []byte, prompt string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("Gemini API key is empty")
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		GeminiFlash25Lite, c.apiKey)

	encodedPDF := base64.StdEncoding.EncodeToString(pdfData)

	// Smart token management for different PDF sizes
	maxTokens := content.DefaultMaxTokens
	temperature := 0.0 // Deterministic output for PDF processing
	pdfSizeKB := len(pdfData) / 1024
	isLargePDF := len(pdfData) > content.LargePDFSizeThreshold

	// Choose appropriate prompt based on PDF size
	var finalPrompt string
	if isLargePDF {
		finalPrompt = content.LargePDFPrompt
	} else {
		finalPrompt = prompt // Use original prompt for small PDFs
	}

	req := pdfGeminiRequest{
		Contents: []pdfGeminiContent{
			{
				Parts: []pdfGeminiPart{
					{
						InlineData: &geminiInlineData{
							MimeType: "application/pdf",
							Data:     encodedPDF,
						},
					},
					{Text: &finalPrompt},
				},
			},
		},
		GenerationConfig: &generationConfig{
			MaxOutputTokens: &maxTokens,
			Temperature:     &temperature,
		},
	}

	// Debug logging
	strategy := "full extraction"
	if isLargePDF {
		strategy = "summarization"
	}
	fmt.Printf("PDF size: %dKB, using %s strategy\n", pdfSizeKB, strategy)
	fmt.Printf("Using maxOutputTokens: %d\n", maxTokens)

	return c.sendRequest(ctx, url, req)
}

// ProcessPDFWithLimit processes a PDF with a specific token limit
func (c *PDFClient) ProcessPDFWithLimit(ctx context.Context, pdfData []byte, prompt string, maxTokens int) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("Gemini API key is empty")
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		GeminiFlash25Lite, c.apiKey)

	encodedPDF := base64.StdEncoding.EncodeToString(pdfData)
	temperature := 0.0 // Deterministic output for consistent summarization

	req := pdfGeminiRequest{
		Contents: []pdfGeminiContent{
			{
				Parts: []pdfGeminiPart{
					{
						InlineData: &geminiInlineData{
							MimeType: "application/pdf",
							Data:     encodedPDF,
						},
					},
					{Text: &prompt},
				},
			},
		},
		GenerationConfig: &generationConfig{
			MaxOutputTokens: &maxTokens,
			Temperature:     &temperature,
		},
	}

	result, err := c.sendRequest(ctx, url, req)
	if err != nil {
		return "", err
	}

	// Debug logging for limited response
	fmt.Printf("Limited response length: %d characters (max %d tokens)\n", len(result), maxTokens)
	fmt.Printf("Estimated output tokens: ~%d\n", len(result)/4)

	return result, nil
}

// sendRequest sends a request to the Gemini API and processes the response
func (c *PDFClient) sendRequest(ctx context.Context, url string, req pdfGeminiRequest) (string, error) {
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
		respBody, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == 429 {
			return "", fmt.Errorf("Gemini API returned status %d: %s (key prefix: %s...)", resp.StatusCode, string(respBody), c.apiKey[:10])
		}
		return "", fmt.Errorf("Gemini API returned status %d: %s", resp.StatusCode, string(respBody))
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

