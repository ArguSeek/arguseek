package content

import "time"

// PDF Processing Configuration Constants
const (
	// Page count thresholds for different processing strategies
	SmallPDFPageThreshold = 3  // PDFs with ≤3 pages get full extraction
	LargePDFPageThreshold = 20 // PDFs with >20 pages are skipped to avoid timeout

	// Timeout configurations
	PDFProcessTimeout   = 25 * time.Second // Overall PDF processing timeout
	ChunkProcessTimeout = 10 * time.Second // Per-chunk processing timeout

	// Token limits for PDF processing
	DefaultMaxTokens      = 30000 // Default token limit for large PDFs
	ChunkSummarizationTokens = 1000  // Token limit for chunk summarization

	// PDF size thresholds
	LargePDFSizeThreshold = 500000 // 500KB threshold for large PDF detection

	// Chunk size configuration
	PagesPerChunk = 3 // Number of pages per chunk for big PDF processing
	
	// PDF Client Constants (duplicated from agent package for consistency)
	PDFClientDefaultMaxTokens = 30000  // Default token limit for large PDFs
	PDFClientLargeSizeThreshold = 500000 // 500KB threshold for large PDF detection
)

// PDF Processing Prompts
const (
	SmallPDFPrompt = "Convert this PDF to clean markdown. Preserve all text, structure, headers, lists, tables, and code blocks."

	LargePDFPrompt = `This is a large document. Create a comprehensive structured summary in markdown format, staying within 25,000 tokens.

REQUIRED STRUCTURE:
# Document Title
**Authors:** [if available]

## Executive Summary
[2-3 paragraph overview]

## Key Sections
### [Section 1 Name]
- Main points
- Key findings

### [Section 2 Name]  
- Main points
- Key findings

[Continue for all major sections]

## Important Details
- Critical data, statistics, or formulas
- Key methodologies or approaches
- Notable conclusions

## References & Citations
[If present in original]

CONSTRAINTS:
- Prioritize the most important content
- Use bullet points for dense information
- Preserve technical terms and concepts
- Stay under 25,000 tokens total`

	ChunkSummarizationPrompt = `Convert this PDF section to concise markdown summary:

## Requirements
- Extract key points and important information
- Use markdown formatting (headers, bullets, tables)
- Include critical data, findings, conclusions
- Preserve technical terms and concepts
- Keep focused and comprehensive
- Maximum 1000 tokens output

Output only the markdown summary.`
)