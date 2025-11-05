package content

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	md "github.com/JohannesKaufmann/html-to-markdown"
)

type ContentProcessor interface {
	ProcessHTML(html string, options ProcessingOptions) (ProcessedContent, error)
	ProcessHTMLWithTimeout(html string, options ProcessingOptions, timeout time.Duration) (ProcessedContent, error)
	ProcessPDF(ctx context.Context, data []byte, options ProcessingOptions) (*ProcessedContent, error)
}

type ProcessingOptions struct {
	MaxLength              int
	PreferMainContent      bool
	PreserveCodeBlocks     bool
	PreserveTables         bool
	LookingFor             string // Query-focused extraction hint
}

type ProcessedContent struct {
	Markdown       string
	ExtractedTitle string
	ContentType    string
	ProcessingInfo ProcessingMetadata
	Text           string // For plain text content from PDFs
	Title          string // For title from PDFs
}

type ProcessingMetadata struct {
	ProcessingTimeMS    int64
	UsedFallback        bool
	MainContentFound    bool
	OriginalLength      int
	ProcessedLength     int
	ExtractionStrategy  string
}


type Processor struct {
	extractor    *ContentExtractor
	truncator    *SmartTruncator
	pdfHandler   *DefaultPDFHandler
}

func NewProcessor() *Processor {
	return &Processor{
		extractor: NewContentExtractor(),
		truncator: NewSmartTruncator(),
	}
}

func (p *Processor) SetPDFHandler(handler *DefaultPDFHandler) {
	p.pdfHandler = handler
}

func (p *Processor) ProcessHTML(html string, options ProcessingOptions) (ProcessedContent, error) {
	startTime := time.Now()
	
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return p.fallbackToSimpleProcessing(html, options, startTime)
	}
	
	mainContent, confidence, strategy := p.extractor.ExtractMainContent(doc)
	if confidence < 0.3 && options.PreferMainContent {
		return p.processFullDocument(doc, options, startTime)
	}
	
	markdown, err := p.convertToMarkdown(mainContent, options)
	if err != nil {
		return p.fallbackToSimpleProcessing(html, options, startTime)
	}
	
	truncatedMarkdown := p.truncator.TruncateMarkdown(markdown, options.MaxLength)
	
	return ProcessedContent{
		Markdown:       truncatedMarkdown,
		ExtractedTitle: p.extractTitle(doc),
		ContentType:    p.detectContentType(doc),
		ProcessingInfo: ProcessingMetadata{
			ProcessingTimeMS:   time.Since(startTime).Milliseconds(),
			UsedFallback:       false,
			MainContentFound:   confidence >= 0.3,
			OriginalLength:     len(html),
			ProcessedLength:    len(truncatedMarkdown),
			ExtractionStrategy: strategy,
		},
	}, nil
}

func (p *Processor) ProcessHTMLWithTimeout(html string, options ProcessingOptions, timeout time.Duration) (ProcessedContent, error) {
	resultChan := make(chan ProcessedContent)
	errorChan := make(chan error)
	
	go func() {
		result, err := p.ProcessHTML(html, options)
		if err != nil {
			errorChan <- err
		} else {
			resultChan <- result
		}
	}()
	
	select {
	case result := <-resultChan:
		return result, nil
	case err := <-errorChan:
		return ProcessedContent{}, err
	case <-time.After(timeout):
		return p.fallbackToSimpleProcessing(html, options, time.Now())
	}
}

func (p *Processor) convertToMarkdown(selection *goquery.Selection, options ProcessingOptions) (string, error) {
	html, err := selection.Html()
	if err != nil {
		return "", err
	}
	
	opts := &md.Options{
		EmDelimiter:     "_",
		StrongDelimiter: "**",
		LinkStyle:       "inlined",
	}
	
	if options.PreserveCodeBlocks {
		opts.CodeBlockStyle = "fenced"
		opts.Fence = "```"
	} else {
		opts.CodeBlockStyle = "indented"
	}
	
	converter := md.NewConverter("", true, opts)
	
	if options.PreserveCodeBlocks {
		converter.AddRules(md.Rule{
			Filter: []string{"pre"},
			Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
				lang := ""
				if code := selec.Find("code"); code.Length() > 0 {
					if class, exists := code.Attr("class"); exists {
						parts := strings.Split(class, "-")
						if len(parts) > 1 {
							lang = parts[1]
						}
					}
				}
				result := "\n```" + lang + "\n" + strings.TrimSpace(content) + "\n```\n"
				return &result
			},
		})
	}
	
	return converter.ConvertString(html)
}

func (p *Processor) processFullDocument(doc *goquery.Document, options ProcessingOptions, startTime time.Time) (ProcessedContent, error) {
	body := doc.Find("body")
	if body.Length() == 0 {
		return p.fallbackToSimpleProcessing(doc.Text(), options, startTime)
	}
	
	markdown, err := p.convertToMarkdown(body, options)
	if err != nil {
		return p.fallbackToSimpleProcessing(doc.Text(), options, startTime)
	}
	
	truncatedMarkdown := p.truncator.TruncateMarkdown(markdown, options.MaxLength)
	
	return ProcessedContent{
		Markdown:       truncatedMarkdown,
		ExtractedTitle: p.extractTitle(doc),
		ContentType:    "general",
		ProcessingInfo: ProcessingMetadata{
			ProcessingTimeMS:   time.Since(startTime).Milliseconds(),
			UsedFallback:       false,
			MainContentFound:   false,
			OriginalLength:     len(doc.Text()),
			ProcessedLength:    len(truncatedMarkdown),
			ExtractionStrategy: "full_document",
		},
	}, nil
}

func (p *Processor) fallbackToSimpleProcessing(html string, options ProcessingOptions, startTime time.Time) (ProcessedContent, error) {
	// Try to use goquery for better text extraction even in fallback
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	var text string
	
	if err != nil {
		// If goquery fails, use basic regex-based approach
		text = html
		// Remove script and style tags and their content
		text = regexp.MustCompile(`(?s)<script[^>]*>.*?</script>`).ReplaceAllString(text, " ")
		text = regexp.MustCompile(`(?s)<style[^>]*>.*?</style>`).ReplaceAllString(text, " ")
		// Remove all other HTML tags
		text = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(text, " ")
		// Decode HTML entities
		text = strings.ReplaceAll(text, "&nbsp;", " ")
		text = strings.ReplaceAll(text, "&amp;", "&")
		text = strings.ReplaceAll(text, "&lt;", "<")
		text = strings.ReplaceAll(text, "&gt;", ">")
		text = strings.ReplaceAll(text, "&quot;", "\"")
	} else {
		// Use goquery's text extraction
		text = doc.Text()
	}
	
	// Clean up whitespace
	lines := strings.Split(text, "\n")
	cleanedLines := make([]string, 0, len(lines))
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleanedLines = append(cleanedLines, trimmed)
		}
	}
	
	text = strings.Join(cleanedLines, "\n")
	
	if len(text) > options.MaxLength {
		text = text[:options.MaxLength]
	}
	
	return ProcessedContent{
		Markdown:       text,
		ExtractedTitle: "",
		ContentType:    "fallback",
		ProcessingInfo: ProcessingMetadata{
			ProcessingTimeMS:   time.Since(startTime).Milliseconds(),
			UsedFallback:       true,
			MainContentFound:   false,
			OriginalLength:     len(html),
			ProcessedLength:    len(text),
			ExtractionStrategy: "fallback",
		},
	}, nil
}

func (p *Processor) extractTitle(doc *goquery.Document) string {
	if title := doc.Find("title").First().Text(); title != "" {
		return strings.TrimSpace(title)
	}
	if h1 := doc.Find("h1").First().Text(); h1 != "" {
		return strings.TrimSpace(h1)
	}
	return ""
}

func (p *Processor) detectContentType(doc *goquery.Document) string {
	if doc.Find("article").Length() > 0 {
		return "article"
	}
	if doc.Find("pre code, .highlight, .code-block").Length() > 5 {
		return "documentation"
	}
	return "general"
}

func (p *Processor) ProcessPDF(ctx context.Context, data []byte, options ProcessingOptions) (*ProcessedContent, error) {
	if p.pdfHandler == nil {
		return nil, fmt.Errorf("PDF processing requires PDF handler")
	}
	
	return p.pdfHandler.ProcessPDF(ctx, data, options)
}

