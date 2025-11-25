package content

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
)

// DefaultPDFHandler handles PDF processing using different strategies based on size
type DefaultPDFHandler struct {
	pdfProcessor PDFProcessor
	chunker      PDFChunker
}

// NewPDFHandler creates a new PDF handler with the specified dependencies
func NewPDFHandler(pdfProcessor PDFProcessor, chunker PDFChunker) *DefaultPDFHandler {
	return &DefaultPDFHandler{
		pdfProcessor: pdfProcessor,
		chunker:      chunker,
	}
}

// ProcessPDF processes a PDF using the appropriate strategy based on page count
func (h *DefaultPDFHandler) ProcessPDF(ctx context.Context, data []byte, options ProcessingOptions) (*ProcessedContent, error) {
	if h.pdfProcessor == nil {
		return nil, fmt.Errorf("PDF processing requires PDF processor")
	}

	log.Printf("PDF processing started: data size %d bytes", len(data))

	// Get page count
	pageCount, err := h.chunker.GetPageCount(data)
	if err != nil {
		log.Printf("PDF processing failed to get page count: %v", err)
		return nil, fmt.Errorf("failed to get page count: %w", err)
	}

	log.Printf("PDF page count: %d", pageCount)

	// Choose processing strategy based on page count
	if pageCount <= SmallPDFPageThreshold {
		log.Printf("Processing as small PDF (≤%d pages)", SmallPDFPageThreshold)
		return h.processSmallPDF(ctx, data, options)
	} else if pageCount > LargePDFPageThreshold {
		log.Printf("PDF too large (%d pages), skipping to avoid timeout", pageCount)
		return &ProcessedContent{
			Markdown: fmt.Sprintf("PDF document with %d pages (too large to process in real-time)", pageCount),
			Text:     fmt.Sprintf("PDF document with %d pages", pageCount),
		}, nil
	} else {
		log.Printf("Processing as medium PDF (>%d pages) with chunked processing", SmallPDFPageThreshold)
		return h.processMediumPDFWithTimeout(ctx, data, pageCount, options)
	}
}

// buildPDFPrompt creates a query-focused prompt based on the LookingFor parameter
func (h *DefaultPDFHandler) buildPDFPrompt(lookingFor string, isChunk bool) string {
	if lookingFor == "" {
		// Return existing prompts for backward compatibility
		if isChunk {
			return ChunkSummarizationPrompt
		}
		return SmallPDFPrompt
	}

	// Query-focused prompts
	if isChunk {
		return fmt.Sprintf(`You are extracting specific information from a PDF chunk.
LOOKING FOR: %s

Instructions:
1. Focus on information related to: %s
2. Extract ONLY relevant content
3. If the chunk doesn't contain relevant information, state "This section doesn't contain information about %s"
4. Present findings in clean markdown

Content follows...`, lookingFor, lookingFor, lookingFor)
	}

	return fmt.Sprintf(`You are extracting specific information from a PDF document.
LOOKING FOR: %s

Extraction approach:
1. Scan the document for information about: %s
2. Extract ONLY content directly related to what the user is looking for
3. If not found, clearly state: "This PDF doesn't contain information about %s"
4. Note any related information that IS available
5. Convert to clean markdown preserving relevant structure

Focus exclusively on the requested information.`, lookingFor, lookingFor, lookingFor)
}

// processSmallPDF handles PDFs with ≤3 pages using full extraction
func (h *DefaultPDFHandler) processSmallPDF(ctx context.Context, data []byte, options ProcessingOptions) (*ProcessedContent, error) {
	log.Printf("Processing small PDF with full extraction")

	// Build query-focused prompt based on options
	prompt := h.buildPDFPrompt(options.LookingFor, false)

	result, err := h.pdfProcessor.ProcessPDF(ctx, data, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to process small PDF: %w", err)
	}

	return &ProcessedContent{
		Markdown: result,
		Text:     result,
	}, nil
}

// processMediumPDFWithTimeout handles PDFs with 4-20 pages using chunked summarization with timeout protection
func (h *DefaultPDFHandler) processMediumPDFWithTimeout(ctx context.Context, data []byte, pageCount int, options ProcessingOptions) (*ProcessedContent, error) {
	log.Printf("Processing medium PDF with chunked summarization and timeout protection")

	// Create a timeout context for PDF processing
	processingCtx, cancel := context.WithTimeout(ctx, PDFProcessTimeout)
	defer cancel()

	// Split into chunks
	chunks, err := h.chunker.CreatePageChunks(data, pageCount)
	if err != nil {
		return nil, fmt.Errorf("failed to create page chunks: %w", err)
	}

	log.Printf("Created %d chunks for summarization", len(chunks))

	// Process each chunk with summarization and timeout
	summaries := h.processChunksWithTimeout(processingCtx, chunks, options)

	// Combine summaries (may include partial results if some chunks timed out)
	finalMarkdown := strings.Join(summaries, "\n\n")

	return &ProcessedContent{
		Markdown: finalMarkdown,
		Text:     finalMarkdown,
	}, nil
}

// processChunksWithTimeout processes chunks with timeout protection
func (h *DefaultPDFHandler) processChunksWithTimeout(ctx context.Context, chunks [][]byte, options ProcessingOptions) []string {
	log.Printf("Chunk processor: Starting to process %d chunks with timeout protection", len(chunks))
	results := make([]string, len(chunks))

	var wg sync.WaitGroup

	for i, chunk := range chunks {
		wg.Add(1)
		go func(index int, chunkData []byte) {
			defer wg.Done()

			// Check context before starting
			select {
			case <-ctx.Done():
				log.Printf("Chunk %d: Skipped due to timeout", index+1)
				results[index] = fmt.Sprintf("<!-- Chunk %d skipped due to timeout -->", index+1)
				return
			default:
			}

			log.Printf("Chunk %d: Starting processing (size: %d bytes)", index+1, len(chunkData))

			// Create per-chunk timeout
			chunkCtx, cancel := context.WithTimeout(ctx, ChunkProcessTimeout)
			defer cancel()

			// Build query-focused prompt for chunk processing
			chunkPrompt := h.buildPDFPrompt(options.LookingFor, true)

			result, err := h.pdfProcessor.ProcessPDFWithLimit(chunkCtx, chunkData, chunkPrompt, ChunkSummarizationTokens)
			if err != nil {
				if chunkCtx.Err() == context.DeadlineExceeded {
					log.Printf("Chunk %d: Timed out after %v", index+1, ChunkProcessTimeout)
					results[index] = fmt.Sprintf("<!-- Chunk %d timed out -->", index+1)
				} else {
					log.Printf("Chunk %d: Error summarizing: %v", index+1, err)
					results[index] = fmt.Sprintf("<!-- Error summarizing chunk %d: %v -->", index+1, err)
				}
			} else {
				log.Printf("Chunk %d: Completed successfully (output: %d chars)", index+1, len(result))
				results[index] = result
			}
		}(i, chunk)
	}

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("Chunk processor: All chunks completed")
	case <-ctx.Done():
		log.Printf("Chunk processor: Processing timed out, returning partial results")
	}

	return results
}
