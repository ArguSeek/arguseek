package content

import (
	"bytes"
	"fmt"
	"log"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// DefaultPDFChunker implements the PDFChunker interface
type DefaultPDFChunker struct{}

// NewPDFChunker creates a new PDF chunker
func NewPDFChunker() PDFChunker {
	return &DefaultPDFChunker{}
}

// GetPageCount returns the number of pages in the PDF
func (c *DefaultPDFChunker) GetPageCount(data []byte) (pageCount int, err error) {
	// Add panic recovery for pdfcpu library calls
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PDF processing panic recovered: %v", r)
			err = fmt.Errorf("PDF processing failed: %v", r)
		}
	}()

	// Basic validation of PDF data
	if len(data) < 4 {
		return 0, fmt.Errorf("invalid PDF: data too short")
	}
	
	// Check for PDF header
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		return 0, fmt.Errorf("invalid PDF: missing PDF header")
	}

	reader := bytes.NewReader(data)
	conf := model.NewDefaultConfiguration()
	ctx, err := api.ReadValidateAndOptimize(reader, conf)
	if err != nil {
		return 0, fmt.Errorf("failed to read PDF: document may be corrupted or unsupported: %w", err)
	}
	if ctx == nil {
		return 0, fmt.Errorf("failed to process PDF: document appears to be corrupted")
	}
	
	return ctx.PageCount, nil
}

// CreatePageChunks splits PDF into chunks of specified page size
func (c *DefaultPDFChunker) CreatePageChunks(data []byte, pageCount int) ([][]byte, error) {
	var chunks [][]byte

	for i := 1; i <= pageCount; i += PagesPerChunk {
		endPage := i + PagesPerChunk - 1
		if endPage > pageCount {
			endPage = pageCount
		}

		chunk, err := c.ExtractPageRange(data, i, endPage)
		if err != nil {
			return nil, fmt.Errorf("failed to extract pages %d-%d: %w", i, endPage, err)
		}

		chunks = append(chunks, chunk)
		log.Printf("PDF chunker: Created chunk for pages %d-%d", i, endPage)
	}

	return chunks, nil
}

// ExtractPageRange extracts a range of pages from PDF data
func (c *DefaultPDFChunker) ExtractPageRange(data []byte, startPage, endPage int) (extractedData []byte, err error) {
	// Add panic recovery for pdfcpu library calls
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PDF page extraction panic recovered: %v", r)
			err = fmt.Errorf("PDF extraction failed: %v", r)
		}
	}()

	reader := bytes.NewReader(data)
	var output bytes.Buffer

	conf := model.NewDefaultConfiguration()

	// Create page selection for the range
	pageSelection := []string{fmt.Sprintf("%d-%d", startPage, endPage)}

	err = api.Trim(reader, &output, pageSelection, conf)
	if err != nil {
		return nil, fmt.Errorf("failed to trim PDF pages %d-%d: document may be corrupted or unsupported: %w", startPage, endPage, err)
	}

	return output.Bytes(), nil
}