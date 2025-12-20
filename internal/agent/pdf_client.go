package agent

import (
	"context"
	"fmt"

	"arguseek/internal/content"

	"google.golang.org/genai"
)

// PDFClient handles PDF processing using the Google Gen AI SDK
type PDFClient struct {
	client *genai.Client
}

// NewPDFClient creates a new PDF client using the shared genai client
func NewPDFClient(client *genai.Client) *PDFClient {
	return &PDFClient{client: client}
}

// ProcessPDF processes a PDF document with the given prompt
func (c *PDFClient) ProcessPDF(ctx context.Context, pdfData []byte, prompt string) (string, error) {
	maxTokens := int32(content.DefaultMaxTokens)
	temperature := float32(0.0)

	// Choose appropriate prompt based on PDF size
	finalPrompt := prompt
	if len(pdfData) > content.LargePDFSizeThreshold {
		finalPrompt = content.LargePDFPrompt
	}

	// Build multimodal content with PDF inline data
	parts := []*genai.Part{
		{
			InlineData: &genai.Blob{
				MIMEType: "application/pdf",
				Data:     pdfData,
			},
		},
		genai.NewPartFromText(finalPrompt),
	}
	contents := []*genai.Content{
		genai.NewContentFromParts(parts, genai.RoleUser),
	}

	config := &genai.GenerateContentConfig{
		MaxOutputTokens: maxTokens,
		Temperature:     &temperature,
	}

	result, err := c.client.Models.GenerateContent(ctx,
		string(GeminiFlash25Lite),
		contents,
		config,
	)
	if err != nil {
		return "", fmt.Errorf("PDF processing failed: %w", err)
	}

	return result.Text(), nil
}

// ProcessPDFWithLimit processes a PDF with a specific token limit
func (c *PDFClient) ProcessPDFWithLimit(ctx context.Context, pdfData []byte, prompt string, maxTokens int) (string, error) {
	temperature := float32(0.0) // Deterministic output for consistent summarization
	maxTokens32 := int32(maxTokens)

	// Build multimodal content with PDF inline data
	parts := []*genai.Part{
		{
			InlineData: &genai.Blob{
				MIMEType: "application/pdf",
				Data:     pdfData,
			},
		},
		genai.NewPartFromText(prompt),
	}
	contents := []*genai.Content{
		genai.NewContentFromParts(parts, genai.RoleUser),
	}

	config := &genai.GenerateContentConfig{
		MaxOutputTokens: maxTokens32,
		Temperature:     &temperature,
	}

	result, err := c.client.Models.GenerateContent(ctx,
		string(GeminiFlash25Lite),
		contents,
		config,
	)
	if err != nil {
		return "", fmt.Errorf("PDF processing failed: %w", err)
	}

	return result.Text(), nil
}
