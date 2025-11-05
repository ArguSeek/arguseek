package content

import "context"

// PDFProcessor defines the interface for processing PDF documents
type PDFProcessor interface {
	ProcessPDF(ctx context.Context, data []byte, prompt string) (string, error)
	ProcessPDFWithLimit(ctx context.Context, data []byte, prompt string, maxTokens int) (string, error)
}

// PDFChunker defines the interface for splitting PDFs into chunks
type PDFChunker interface {
	GetPageCount(data []byte) (int, error)
	CreatePageChunks(data []byte, pageCount int) ([][]byte, error)
	ExtractPageRange(data []byte, startPage, endPage int) ([]byte, error)
}