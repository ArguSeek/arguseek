package content

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestNewContentExtractor(t *testing.T) {
	ce := NewContentExtractor()
	if ce == nil {
		t.Fatal("expected content extractor to be non-nil")
	}
	if len(ce.strategies) != 4 {
		t.Errorf("expected 4 strategies, got %d", len(ce.strategies))
	}
}

func TestMainElementStrategy(t *testing.T) {
	tests := []struct {
		name          string
		html          string
		expectFound   bool
		minConfidence float64
	}{
		{
			name:          "main element",
			html:          `<html><body><main><p>Main content</p></main></body></html>`,
			expectFound:   true,
			minConfidence: 0.9,
		},
		{
			name:          "role main",
			html:          `<html><body><div role="main"><p>Main content</p></div></body></html>`,
			expectFound:   true,
			minConfidence: 0.85,
		},
		{
			name:        "no main element",
			html:        `<html><body><div><p>Content</p></div></body></html>`,
			expectFound: false,
		},
	}

	strategy := &MainElementStrategy{}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(test.html))
			if err != nil {
				t.Fatalf("failed to parse HTML: %v", err)
			}

			selection, confidence := strategy.Extract(doc)

			if test.expectFound {
				if selection == nil || selection.Length() == 0 {
					t.Error("expected to find main element")
				}
				if confidence < test.minConfidence {
					t.Errorf("expected confidence >= %f, got %f", test.minConfidence, confidence)
				}
			} else {
				if confidence > 0 {
					t.Errorf("expected zero confidence, got %f", confidence)
				}
			}
		})
	}
}

func TestArticleStrategy(t *testing.T) {
	tests := []struct {
		name          string
		html          string
		expectFound   bool
		minConfidence float64
	}{
		{
			name:          "article with content",
			html:          `<html><body><article>` + strings.Repeat("Long article content. ", 50) + `</article></body></html>`,
			expectFound:   true,
			minConfidence: 0.8,
		},
		{
			name:        "article too short",
			html:        `<html><body><article>Short</article></body></html>`,
			expectFound: false,
		},
		{
			name:        "no article",
			html:        `<html><body><div>Content</div></body></html>`,
			expectFound: false,
		},
	}

	strategy := &ArticleStrategy{}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(test.html))
			if err != nil {
				t.Fatalf("failed to parse HTML: %v", err)
			}

			selection, confidence := strategy.Extract(doc)

			if test.expectFound {
				if selection == nil || selection.Length() == 0 {
					t.Error("expected to find article")
				}
				if confidence < test.minConfidence {
					t.Errorf("expected confidence >= %f, got %f", test.minConfidence, confidence)
				}
			} else {
				if confidence > 0 {
					t.Errorf("expected zero confidence, got %f", confidence)
				}
			}
		})
	}
}

func TestSemanticStrategy(t *testing.T) {
	tests := []struct {
		name        string
		html        string
		expectFound bool
	}{
		{
			name:        "content id",
			html:        `<html><body><div id="content">` + strings.Repeat("Content here. ", 50) + `</div></body></html>`,
			expectFound: true,
		},
		{
			name:        "content class",
			html:        `<html><body><div class="main-content">` + strings.Repeat("Content here. ", 50) + `</div></body></html>`,
			expectFound: true,
		},
		{
			name:        "markdown-body class",
			html:        `<html><body><div class="markdown-body">` + strings.Repeat("Markdown content. ", 50) + `</div></body></html>`,
			expectFound: true,
		},
		{
			name:        "no semantic markers",
			html:        `<html><body><div>Content</div></body></html>`,
			expectFound: false,
		},
	}

	strategy := &SemanticStrategy{}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(test.html))
			if err != nil {
				t.Fatalf("failed to parse HTML: %v", err)
			}

			_, confidence := strategy.Extract(doc)

			if test.expectFound {
				if confidence <= 0 {
					t.Error("expected to find content with semantic markers")
				}
			} else {
				if confidence > 0 {
					t.Errorf("expected zero confidence, got %f", confidence)
				}
			}
		})
	}
}

func TestHeuristicStrategy(t *testing.T) {
	tests := []struct {
		name          string
		html          string
		expectFound   bool
		minConfidence float64
	}{
		{
			name: "content-rich div",
			html: `<html><body><div class="post-content">` +
				strings.Repeat("<p>This is a paragraph with content. </p>", 20) +
				`</div></body></html>`,
			expectFound:   true,
			minConfidence: 0.3,
		},
		{
			name: "navigation heavy",
			html: `<html><body><div class="navigation">` +
				strings.Repeat("<a href='#'>Link</a> ", 100) +
				`</div></body></html>`,
			expectFound: false,
		},
		{
			name: "sidebar content",
			html: `<html><body><div class="sidebar">` +
				strings.Repeat("Sidebar content. ", 50) +
				`</div></body></html>`,
			expectFound: false,
		},
	}

	strategy := &HeuristicStrategy{}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(test.html))
			if err != nil {
				t.Fatalf("failed to parse HTML: %v", err)
			}

			selection, confidence := strategy.Extract(doc)

			if test.expectFound {
				if selection == nil || selection.Length() == 0 {
					t.Error("expected to find content")
				}
				if confidence < test.minConfidence {
					t.Errorf("expected confidence >= %f, got %f", test.minConfidence, confidence)
				}
			} else {
				if confidence > 0.3 {
					t.Errorf("expected low confidence, got %f", confidence)
				}
			}
		})
	}
}

func TestExtractMainContent(t *testing.T) {
	ce := NewContentExtractor()

	tests := []struct {
		name             string
		html             string
		expectedStrategy string
		minConfidence    float64
	}{
		{
			name: "main element wins",
			html: `<html><body>
				<article>Article content</article>
				<main>Main content that should be selected</main>
			</body></html>`,
			expectedStrategy: "main_element",
			minConfidence:    0.9,
		},
		{
			name: "article fallback",
			html: `<html><body>
				<article>` + strings.Repeat("Long article content. ", 50) + `</article>
			</body></html>`,
			expectedStrategy: "article",
			minConfidence:    0.8,
		},
		{
			name: "semantic content",
			html: `<html><body>
				<div id="content">` + strings.Repeat("Main content here. ", 50) + `</div>
			</body></html>`,
			expectedStrategy: "semantic",
			minConfidence:    0.6,
		},
		{
			name:             "fallback to body",
			html:             `<html><body>Just some text</body></html>`,
			expectedStrategy: "fallback_body",
			minConfidence:    0.1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(test.html))
			if err != nil {
				t.Fatalf("failed to parse HTML: %v", err)
			}

			selection, confidence, strategy := ce.ExtractMainContent(doc)

			if selection == nil || selection.Length() == 0 {
				t.Error("expected to extract some content")
			}

			if strategy != test.expectedStrategy {
				t.Errorf("expected strategy '%s', got '%s'", test.expectedStrategy, strategy)
			}

			if confidence < test.minConfidence {
				t.Errorf("expected confidence >= %f, got %f", test.minConfidence, confidence)
			}
		})
	}
}
